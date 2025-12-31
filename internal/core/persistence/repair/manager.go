package repair

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
	"google.golang.org/protobuf/proto"
)

// Manager 是 persistence 模块的内部子组件：负责对“存储/索引类损坏”做在线自愈调度。
//
// 设计约束（对齐 _dev 架构）：
// - 不作为 core 一级组件对外暴露
// - 不单独提供 fx module；由 persistence 模块内部在构建时注册订阅
type Manager struct {
	store           storeiface.BadgerStore
	fileStore       storeiface.FileStore
	bus             eventiface.EventBus
	logger          logiface.Logger
	blockHashClient core.BlockHashServiceClient
	txHashClient    transactionpb.TransactionHashServiceClient

	// 并发控制
	sem chan struct{}

	// 去抖/限流（按 key/hash）
	mu          sync.Mutex
	lastAttempt map[string]time.Time

	// 配置（先固定默认，后续走 config-knobs）
	throttle time.Duration
	window   uint64
	enabled  bool
}

func NewManager(
	store storeiface.BadgerStore,
	fileStore storeiface.FileStore,
	blockHashClient core.BlockHashServiceClient,
	txHashClient transactionpb.TransactionHashServiceClient,
	bus eventiface.EventBus,
	logger logiface.Logger,
	opts Options,
) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("store 不能为空")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("fileStore 不能为空")
	}
	// bus 可选：没有事件总线则不启用自愈
	if opts.MaxConcurrency <= 0 {
		opts.MaxConcurrency = 2
	}
	if opts.ThrottleSeconds <= 0 {
		opts.ThrottleSeconds = 60
	}
	if opts.HashIndexWindow <= 0 {
		opts.HashIndexWindow = 5000
	}
	m := &Manager{
		store:           store,
		fileStore:       fileStore,
		blockHashClient: blockHashClient,
		txHashClient:    txHashClient,
		bus:             bus,
		logger:          logger,
		sem:             make(chan struct{}, opts.MaxConcurrency),
		lastAttempt:     make(map[string]time.Time),
		throttle:        time.Duration(opts.ThrottleSeconds) * time.Second,
		window:          uint64(opts.HashIndexWindow),
		enabled:         opts.Enabled,
	}
	return m, nil
}

// RegisterSubscriptions 在 persistence 模块内部调用，用于订阅 corruption.detected。
func (m *Manager) RegisterSubscriptions(ctx context.Context) {
	if m == nil || m.bus == nil || !m.enabled {
		return
	}
	_ = m.bus.Subscribe(eventiface.EventTypeCorruptionDetected, func(evCtx context.Context, data interface{}) error {
		evt, ok := data.(types.CorruptionEventData)
		if !ok {
			if p, ok2 := data.(*types.CorruptionEventData); ok2 && p != nil {
				evt = *p
				ok = true
			}
		}
		if !ok {
			return nil
		}
		go m.handle(evCtx, evt)
		return nil
	})
}

func (m *Manager) handle(ctx context.Context, evt types.CorruptionEventData) {
	if m == nil || m.bus == nil {
		return
	}
	if evt.ErrClass == "" {
		evt.ErrClass = corruptutil.ClassifyErr(fmt.Errorf("%s", evt.Error))
	}

	switch evt.ErrClass {
	case "index_corrupt_hash_height":
		m.repairHashToHeight(ctx, evt)
	case "index_corrupt_height_index":
		m.repairHeightIndex(ctx, evt)
	case "tip_inconsistent":
		m.repairTipConsistency(ctx, evt)
	case "tx_index_corrupt":
		m.repairTxIndex(ctx, evt)
	// 🆕 索引路径损坏：索引中存储了非法路径（如 ../blocks/...），需要重建索引
	case "index_path_corrupt":
		m.repairHeightIndex(ctx, evt)
	// 🆕 区块文件缺失：尝试重建索引（如果文件实际存在但索引路径错误，可以修复）
	case "block_file_missing":
		m.repairHeightIndex(ctx, evt)
	default:
		// 未实现的修复类型：先不做动作（后续扩展）
		if m.logger != nil {
			m.logger.Debugf("🔧 未处理的损坏类型: class=%s height=%v key=%s", evt.ErrClass, evt.Height, evt.Key)
		}
	}
}

func (m *Manager) repairTxIndex(ctx context.Context, evt types.CorruptionEventData) {
	if evt.Hash == "" {
		return
	}
	txHashHex := evt.Hash
	txKey := ""
	if evt.Key != "" {
		txKey = evt.Key
	} else {
		txKey = "indices:tx:" + txHashHex
	}

	// 去抖/限流
	now := time.Now()
	m.mu.Lock()
	if last, ok := m.lastAttempt[txKey]; ok && now.Sub(last) < m.throttle {
		m.mu.Unlock()
		m.publishRepairResult(true, "rebuild_tx_index", txKey, txHashHex, nil, "skipped(throttled)", nil, true)
		return
	}
	m.lastAttempt[txKey] = now
	m.mu.Unlock()

	// 并发控制
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	default:
		m.publishRepairResult(true, "rebuild_tx_index", txKey, txHashHex, nil, "skipped(concurrency_limit)", nil, true)
		return
	}

	if m.txHashClient == nil || m.blockHashClient == nil {
		m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, nil, "missing deps", fmt.Errorf("txHashClient/blockHashClient required"), false)
		return
	}

	txHashBytes, err := hex.DecodeString(txHashHex)
	if err != nil {
		m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, nil, "invalid tx hash hex", err, false)
		return
	}

	// 读取 tipHeight（如果 tip 损坏，tip_inconsistent 会先修复；这里失败则直接退出）
	tipData, err := m.store.Get(ctx, []byte("state:chain:tip"))
	if err != nil || len(tipData) < 8 {
		if err == nil {
			err = fmt.Errorf("tip data invalid (len=%d)", len(tipData))
		}
		m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, nil, "read tip failed", err, false)
		return
	}
	tipHeight := bytesToUint64(tipData[:8])

	var start uint64
	if tipHeight > m.window {
		start = tipHeight - m.window
	} else {
		start = 0
	}

	// S2：扫描 BadgerDB 区块数据：blocks:data:height:{height}
	for h := tipHeight; ; h-- {
		// blocks/ 设计：区块文件路径可由高度确定（与 writer/block.go 对齐）
		seg := (h / 1000) * 1000
		blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, h)
		b, e := m.fileStore.Load(ctx, blockFilePath)
		if e != nil || len(b) == 0 {
			if h == start || h == 0 {
				break
			}
			continue
		}

		blk := &core.Block{}
		if proto.Unmarshal(b, blk) != nil || blk.Body == nil || len(blk.Body.Transactions) == 0 {
			if h == start || h == 0 {
				break
			}
			continue
		}

		for i, txp := range blk.Body.Transactions {
			resp, he := m.txHashClient.ComputeHash(ctx, &transactionpb.ComputeHashRequest{Transaction: txp})
			if he != nil || resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
				continue
			}
			if string(resp.Hash) == string(txHashBytes) {
				// 命中：计算 block hash 并写回 indices:tx
				bhResp, bhe := m.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
				if bhe != nil || bhResp == nil || !bhResp.IsValid || len(bhResp.Hash) == 0 {
					m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, &h, "compute block hash failed", bhe, false)
					return
				}

				indexValue := make([]byte, 44)
				copy(indexValue[0:8], uint64ToBytes(h))
				copy(indexValue[8:40], bhResp.Hash)
				putUint32(indexValue[40:44], uint32(i))

				if err := m.store.Set(ctx, []byte(fmt.Sprintf("indices:tx:%x", txHashBytes)), indexValue); err != nil {
					m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, &h, "write tx index failed", err, false)
					return
				}
				m.publishRepairResult(true, "rebuild_tx_index", fmt.Sprintf("indices:tx:%x", txHashBytes), txHashHex, &h, "repair success", nil, false)
				return
			}
		}

		if h == start || h == 0 {
			break
		}
	}

	m.publishRepairResult(false, "rebuild_tx_index", txKey, txHashHex, nil, "not found in scan window", fmt.Errorf("tx not found in window"), false)
}

func putUint32(b []byte, v uint32) {
	_ = b[3]
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func (m *Manager) repairTipConsistency(ctx context.Context, evt types.CorruptionEventData) {
	tipKey := "state:chain:tip"

	// 去抖/限流
	now := time.Now()
	m.mu.Lock()
	if last, ok := m.lastAttempt[tipKey]; ok && now.Sub(last) < m.throttle {
		m.mu.Unlock()
		m.publishRepairResult(true, "rebuild_tip", tipKey, "", nil, "skipped(throttled)", nil, true)
		return
	}
	m.lastAttempt[tipKey] = now
	m.mu.Unlock()

	// 并发控制
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	default:
		m.publishRepairResult(true, "rebuild_tip", tipKey, "", nil, "skipped(concurrency_limit)", nil, true)
		return
	}

	// 从 indices:height:* 找到最大高度，重建 tip = height(8) + hash(32)
	const prefix = "indices:height:"
	entries, err := m.store.PrefixScan(ctx, []byte(prefix))
	if err != nil {
		m.publishRepairResult(false, "rebuild_tip", tipKey, "", nil, "prefix scan failed", err, false)
		return
	}
	var maxH uint64
	var maxHash []byte
	found := false

	for k, v := range entries {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		hStr := strings.TrimPrefix(k, prefix)
		h, e := strconv.ParseUint(hStr, 10, 64)
		if e != nil {
			continue
		}
		if len(v) < 32 {
			continue
		}
		if !found || h > maxH {
			maxH = h
			maxHash = append([]byte(nil), v[:32]...)
			found = true
		}
	}

	if !found || len(maxHash) != 32 {
		m.publishRepairResult(false, "rebuild_tip", tipKey, "", nil, "no valid height index found", fmt.Errorf("no valid indices:height entries"), false)
		return
	}

	tipValue := append(uint64ToBytes(maxH), maxHash...)
	if err := m.store.Set(ctx, []byte(tipKey), tipValue); err != nil {
		m.publishRepairResult(false, "rebuild_tip", tipKey, fmt.Sprintf("%x", maxHash), &maxH, "write tip failed", err, false)
		return
	}

	// 顺手修复对应的 indices:hash（保证 hash->height 一致）
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", maxHash))
	_ = m.store.Set(ctx, hashKey, uint64ToBytes(maxH))

	m.publishRepairResult(true, "rebuild_tip", tipKey, fmt.Sprintf("%x", maxHash), &maxH, "repair success", nil, false)
}

func (m *Manager) repairHeightIndex(ctx context.Context, evt types.CorruptionEventData) {
	if evt.Height == nil {
		return
	}
	h := *evt.Height
	key := fmt.Sprintf("indices:height:%d", h)

	// 去抖/限流
	now := time.Now()
	m.mu.Lock()
	if last, ok := m.lastAttempt[key]; ok && now.Sub(last) < m.throttle {
		m.mu.Unlock()
		m.publishRepairResult(true, "rebuild_height_index", key, "", &h, "skipped(throttled)", nil, true)
		return
	}
	m.lastAttempt[key] = now
	m.mu.Unlock()

	// 并发控制
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	default:
		m.publishRepairResult(true, "rebuild_height_index", key, "", &h, "skipped(concurrency_limit)", nil, true)
		return
	}

	// 1) 从 blocks/ 文件读取区块数据（按高度可确定路径）
	seg := (h / 1000) * 1000
		blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, h)
	blockBytes, err := m.fileStore.Load(ctx, blockFilePath)
	if err != nil || len(blockBytes) == 0 {
		if err == nil {
			err = fmt.Errorf("empty block bytes")
		}
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "read block file failed", err, false)
		return
	}

	// 2) 反序列化区块
	blk := &core.Block{}
	if err := proto.Unmarshal(blockBytes, blk); err != nil {
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "unmarshal block failed", err, false)
		return
	}
	if blk.Header == nil || blk.Header.Height != h {
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "height mismatch in file", fmt.Errorf("file height=%d expected=%d", safeHeight(blk), h), false)
		return
	}

	// 3) 计算区块哈希（复用 blockHashClient）
	if m.blockHashClient == nil {
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "blockHashClient missing", fmt.Errorf("blockHashClient is nil"), false)
		return
	}
	resp, err := m.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
	if err != nil || resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
		if err == nil {
			err = fmt.Errorf("invalid hash response")
		}
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "compute block hash failed", err, false)
		return
	}
	hash := resp.Hash

	// 4) 写回 indices:height（blockHash(32)+filePathLen(1)+filePath+fileSize(8)）
	filePathBytes := []byte(blockFilePath)
	indexValue := make([]byte, 32+1+len(filePathBytes)+8)
	copy(indexValue[0:32], hash)
	indexValue[32] = byte(len(filePathBytes))
	copy(indexValue[33:33+len(filePathBytes)], filePathBytes)
	copy(indexValue[33+len(filePathBytes):41+len(filePathBytes)], uint64ToBytes(uint64(len(blockBytes))))

	if err := m.store.Set(ctx, []byte(key), indexValue); err != nil {
		m.publishRepairResult(false, "rebuild_height_index", key, "", &h, "write indices:height failed", err, false)
		return
	}

	// 5) 同步更新 indices:hash（hash -> height）
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", hash))
	if err := m.store.Set(ctx, hashKey, uint64ToBytes(h)); err != nil {
		m.publishRepairResult(false, "rebuild_height_index", key, fmt.Sprintf("%x", hash), &h, "write indices:hash failed", err, false)
		return
	}

	m.publishRepairResult(true, "rebuild_height_index", key, fmt.Sprintf("%x", hash), &h, "repair success", nil, false)
}

func safeHeight(b *core.Block) uint64 {
	if b == nil || b.Header == nil {
		return 0
	}
	return b.Header.Height
}

func (m *Manager) repairHashToHeight(ctx context.Context, evt types.CorruptionEventData) {
	hashHex := evt.Hash
	if hashHex == "" {
		return
	}

	key := "hash:" + hashHex
	if evt.Key != "" {
		key = evt.Key
	}

	// 去抖/限流
	now := time.Now()
	m.mu.Lock()
	if last, ok := m.lastAttempt[key]; ok && now.Sub(last) < m.throttle {
		m.mu.Unlock()
		m.publishRepairResult(true, "rebuild_hash_index", evt.Key, hashHex, evt.Height, "skipped(throttled)", nil, true)
		return
	}
	m.lastAttempt[key] = now
	m.mu.Unlock()

	// 并发控制
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	default:
		m.publishRepairResult(true, "rebuild_hash_index", evt.Key, hashHex, evt.Height, "skipped(concurrency_limit)", nil, true)
		return
	}

	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		m.publishRepairResult(false, "rebuild_hash_index", evt.Key, hashHex, evt.Height, "invalid hash hex", err, false)
		return
	}

	height, err := RepairHashToHeightIndex(ctx, m.store, m.logger, hashBytes, m.window)
	if err != nil {
		m.publishRepairResult(false, "rebuild_hash_index", evt.Key, hashHex, evt.Height, "repair failed", err, false)
		return
	}

	m.publishRepairResult(true, "rebuild_hash_index", evt.Key, hashHex, &height, "repair success", nil, false)
}

func (m *Manager) publishRepairResult(success bool, action string, targetKey, targetHash string, targetHeight *uint64, details string, err error, skipped bool) {
	if m == nil || m.bus == nil {
		return
	}

	result := "success"
	evtType := eventiface.EventTypeCorruptionRepaired
	errStr := ""

	if skipped {
		result = "skipped"
		evtType = eventiface.EventTypeCorruptionRepaired
	} else if !success {
		result = "failed"
		evtType = eventiface.EventTypeCorruptionRepairFailed
		if err != nil {
			errStr = err.Error()
		}
	}

	data := types.CorruptionRepairEventData{
		Component:    types.CorruptionComponentPersistence,
		Phase:        types.CorruptionPhaseReadIndex,
		TargetKey:    targetKey,
		TargetHash:   targetHash,
		TargetHeight: targetHeight,
		Action:       action,
		Result:       result,
		Details:      details,
		Error:        errStr,
		At:           types.RFC3339Time(time.Now()),
	}
	m.bus.Publish(evtType, context.Background(), data)
}
