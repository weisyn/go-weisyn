// Package block 实现区块查询服务
package block

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
	"google.golang.org/protobuf/proto"
)

// Service 区块查询服务
type Service struct {
	storage   storage.BadgerStore
	fileStore storage.FileStore
	logger    log.Logger
	eventBus  eventiface.EventBus // 可选：用于发布corruption/repaired事件

	// blockCache 简单的按高度缓存区块，减少重复读盘
	blockCache *blockCache

	// 自愈：避免同一个 hash 的索引损坏反复触发昂贵扫描
	repairMu          sync.Mutex
	lastRepairAttempt map[string]time.Time // key: hex(hash)

	// 自愈配置（对齐 blockchain.sync.advanced.*）
	repairEnabled         bool
	repairThrottle        time.Duration
	repairHashIndexWindow uint64
}

// NewService 创建区块查询服务
func NewService(storage storage.BadgerStore, fileStore storage.FileStore, configProvider config.Provider, eventBus eventiface.EventBus, logger log.Logger) (interfaces.InternalBlockQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("fileStore 不能为空")
	}

	s := &Service{
		storage:               storage,
		fileStore:             fileStore,
		eventBus:              eventBus,
		logger:                logger,
		blockCache:            newBlockCache(1000), // 默认缓存最近 1000 个区块
		lastRepairAttempt:     make(map[string]time.Time),
		repairEnabled:         true,
		repairThrottle:        10 * time.Second, // 🔧 从60秒缩短到10秒，加快索引修复响应
		repairHashIndexWindow: 5000,
	}

	// 从配置注入自愈 knobs（不影响共识，只影响在线修复行为）
	if configProvider != nil && configProvider.GetBlockchain() != nil {
		adv := configProvider.GetBlockchain().Sync.Advanced
		s.repairEnabled = adv.RepairEnabled
		if adv.RepairThrottleSeconds > 0 {
			s.repairThrottle = time.Duration(adv.RepairThrottleSeconds) * time.Second
		}
		if adv.RepairHashIndexWindow > 0 {
			s.repairHashIndexWindow = uint64(adv.RepairHashIndexWindow)
		}
	}

	if logger != nil {
		logger.Info("✅ BlockQuery 服务已创建（blocks/: 区块数据从文件读取，Badger 存索引）")
	}

	return s, nil
}

func (s *Service) publishCorruptionDetected(phase types.CorruptionPhase, severity types.CorruptionSeverity, height *uint64, hashHex string, key string, err error) {
	if s.eventBus == nil || err == nil {
		return
	}
	data := types.CorruptionEventData{
		Component: types.CorruptionComponentPersistence,
		Phase:     phase,
		Severity:  severity,
		Height:    height,
		Hash:      hashHex,
		Key:       key,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        types.RFC3339Time(time.Now()),
	}
	s.eventBus.Publish(eventiface.EventTypeCorruptionDetected, context.Background(), data)
}

// publishGenesisIndexCorruption 发布创世区块索引损坏事件
//
// 🆕 **查询时自愈触发**：当GetBlockByHeight(0)失败时，自动触发修复
//
// 参数：
//   - ctx: 上下文
//   - err: 错误信息
func (s *Service) publishGenesisIndexCorruption(ctx context.Context, err error) {
	if s.eventBus == nil {
		return
	}
	
	height := uint64(0)
	evt := types.CorruptionEventData{
		Component: types.CorruptionComponentPersistence,
		Phase:     types.CorruptionPhaseReadIndex,
		Severity:  types.CorruptionSeverityCritical,
		Height:    &height,
		Key:       "indices:height:0",
		Error:     err.Error(),
		ErrClass:  "genesis_index_corrupt", // 特殊分类
		At:        types.RFC3339Time(time.Now()),
	}
	
	if s.logger != nil {
		s.logger.Warnf("🩹 检测到创世区块索引损坏，发布修复事件: err=%v", err)
	}
	
	s.eventBus.Publish(eventiface.EventTypeCorruptionDetected, ctx, evt)
}

func (s *Service) publishRepairResult(success bool, action string, targetKey, targetHash string, targetHeight *uint64, details string, err error) {
	if s.eventBus == nil {
		return
	}
	result := "success"
	evtType := eventiface.EventTypeCorruptionRepaired
	errStr := ""
	if !success {
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
	s.eventBus.Publish(evtType, context.Background(), data)
}

// uint64ToBytes 将 uint64 编码为 8 字节（大端）。
func uint64ToBytes(v uint64) []byte {
	return []byte{
		byte(v >> 56),
		byte(v >> 48),
		byte(v >> 40),
		byte(v >> 32),
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
}

// tryRepairHashIndex 尝试修复 indices:hash:<blockHash> 的映射（hash->height）。
//
// 设计目标（生产化自运行）：
// - 该索引损坏会导致 “GetBlockByHash → 无法得到高度 → 无法加载父块 → 验证/同步/分叉归口全部失效”
// - 修复应在错误发生时自动触发，而不是等待人工
// - 修复必须是“轻量/有界/可重试/可观测”
//
// 修复策略（有界扫描）：
// - 从 state:chain:tip 取 tipHeight
// - 向下扫描最近 window 个高度，从 indices:height:<h> 读出 blockHash(32 bytes)
// - 若匹配目标 hash，则写回 indices:hash:<hash> = height(8 bytes)
func (s *Service) tryRepairHashIndex(ctx context.Context, blockHash []byte) error {
	if len(blockHash) == 0 {
		return fmt.Errorf("empty blockHash")
	}
	if s != nil && !s.repairEnabled {
		return fmt.Errorf("repair disabled (hash=%x)", blockHash)
	}

	// 去重与限流：同一 hash 默认10秒内只尝试一次（可通过配置调整）
	// 🔧 优化：从60秒缩短到10秒，在保持限流保护的同时加快修复响应
	key := fmt.Sprintf("%x", blockHash)
	s.repairMu.Lock()
	throttle := 10 * time.Second // 默认10秒
	if s != nil && s.repairThrottle > 0 {
		throttle = s.repairThrottle
	}
	if last, ok := s.lastRepairAttempt[key]; ok && time.Since(last) < throttle {
		s.repairMu.Unlock()
		return fmt.Errorf("repair throttled (hash=%s throttle=%s)", key[:8], throttle)
	}
	s.lastRepairAttempt[key] = time.Now()
	s.repairMu.Unlock()

	// 读取链尖
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityWarning, nil, "", string(tipKey), err)
		return fmt.Errorf("repair: read tip failed: %w", err)
	}
	if len(tipData) < 8 {
		return fmt.Errorf("repair: tip data invalid (len=%d)", len(tipData))
	}
	tipHeight := bytesToUint64(tipData[:8])

	// 有界扫描窗口（来自配置 blockchain.sync.advanced.repair_hash_index_window）
	window := uint64(5000)
	if s != nil && s.repairHashIndexWindow > 0 {
		window = s.repairHashIndexWindow
	}
	var start uint64
	if tipHeight > window {
		start = tipHeight - window
	} else {
		start = 0
	}

	if s.logger != nil {
		s.logger.Warnf("🩹 auto_repair: detected corrupted hash->height index, scanning window [%d..%d] hash=%s",
			start, tipHeight, key[:12]+"...")
	}

	for h := tipHeight; ; h-- {
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		indexData, e := s.storage.Get(ctx, heightKey)
		if e == nil && len(indexData) >= 32 {
			// 新/旧格式都以 32 bytes blockHash 开头
			if string(indexData[:32]) == string(blockHash) {
				hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
				if err := s.storage.Set(ctx, hashKey, uint64ToBytes(h)); err != nil {
					s.publishRepairResult(false, "rebuild_hash_index", string(hashKey), key, &h, "write indices:hash failed", err)
					return fmt.Errorf("repair: write hash index failed: %w", err)
				}
				if s.logger != nil {
					s.logger.Warnf("✅ auto_repair: hash index repaired: hash=%s height=%d", key[:12]+"...", h)
				}
				s.publishRepairResult(true, "rebuild_hash_index", string(hashKey), key, &h, "repaired by scanning indices:height window", nil)
				return nil
			}
		}

		if h == start {
			break
		}
		if h == 0 {
			break
		}
	}

	return fmt.Errorf("repair: target hash not found in window (tip=%d window=%d)", tipHeight, window)
}

// tryRepairHashIndexFast 是一个“快速探测式”修复：
//
// 背景：
// - BadgerStore.Get 在 key 不存在时会返回 (nil, nil)（即 len==0 且 err==nil）
// - 对于同步/验证的热路径，hash->height 索引缺失并不一定意味着数据损坏；
//   更常见的是“索引尚未构建/迁移未覆盖/历史版本未写入 indices:hash”
//
// 策略：
// - 仅扫描 tipHeight 附近的最近 maxProbe 个高度（默认 256），命中则立即补写 indices:hash
// - 不做全窗口扫描，不占用 tryRepairHashIndex 的去重节流额度
func (s *Service) tryRepairHashIndexFast(ctx context.Context, blockHash []byte, maxProbe uint64) (bool, error) {
	if len(blockHash) == 0 {
		return false, fmt.Errorf("empty blockHash")
	}
	if s != nil && !s.repairEnabled {
		return false, fmt.Errorf("repair disabled (hash=%x)", blockHash)
	}
	if maxProbe == 0 {
		maxProbe = 256
	}

	// 读取链尖
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		return false, fmt.Errorf("repair_fast: read tip failed: %w", err)
	}
	if len(tipData) < 8 {
		return false, fmt.Errorf("repair_fast: tip data invalid (len=%d)", len(tipData))
	}
	tipHeight := bytesToUint64(tipData[:8])

	// 计算探测起点：最近 maxProbe 个高度
	var start uint64
	if tipHeight > maxProbe {
		start = tipHeight - maxProbe
	} else {
		start = 0
	}

	for h := tipHeight; ; h-- {
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		indexData, e := s.storage.Get(ctx, heightKey)
		if e == nil && len(indexData) >= 32 {
			// 新/旧格式都以 32 bytes blockHash 开头
			if string(indexData[:32]) == string(blockHash) {
				hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
				if err := s.storage.Set(ctx, hashKey, uint64ToBytes(h)); err != nil {
					return false, fmt.Errorf("repair_fast: write hash index failed: %w", err)
				}
				return true, nil
			}
		}

		if h == start {
			break
		}
		if h == 0 {
			break
		}
	}

	// 未命中并不是错误：可能该 hash 不在 tip 附近，需要走重扫描/或本地本就没有该块。
	return false, nil
}

// GetBlockByHeight 按高度获取区块
func (s *Service) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	// 优先从内存缓存获取，减少重复读盘
	if s.blockCache != nil {
		if cached, ok := s.blockCache.Get(height); ok && cached != nil {
			if s.logger != nil {
				s.logger.Debugf("命中区块缓存: height=%d", height)
			}
			return cached, nil
		}
	}

	// blocks/ 设计：区块数据在文件系统，Badger 仅存索引（height->hash+path+size）
	heightKey := []byte(fmt.Sprintf("indices:height:%d", height))
	indexData, err := s.storage.Get(ctx, heightKey)
	if err != nil {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityWarning, &h, "", string(heightKey), err)
		
		// 🆕 特殊处理高度0：触发创世区块索引修复
		if height == 0 {
			s.publishGenesisIndexCorruption(ctx, err)
		}
		
		return nil, fmt.Errorf("获取区块高度索引失败: %w", err)
	}
	// indexData 格式：blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)
	if len(indexData) < 32+1+8 {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityCritical, &h, "", string(heightKey),
			fmt.Errorf("invalid indices:height format len=%d", len(indexData)))
		
		// 🆕 特殊处理高度0：触发创世区块索引修复
		if height == 0 {
			s.publishGenesisIndexCorruption(ctx, fmt.Errorf("索引数据长度不足: len=%d", len(indexData)))
		}
		
		return nil, fmt.Errorf("区块高度索引数据格式错误: len=%d", len(indexData))
	}
	pathLen := int(indexData[32])
	if pathLen <= 0 || len(indexData) < 33+pathLen+8 {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityCritical, &h, "", string(heightKey),
			fmt.Errorf("invalid indices:height pathLen=%d len=%d", pathLen, len(indexData)))
		return nil, fmt.Errorf("区块高度索引数据格式错误: pathLen=%d len=%d", pathLen, len(indexData))
	}
	filePath := string(indexData[33 : 33+pathLen])
	fileSizeBytes := indexData[33+pathLen : 41+pathLen]
	expectedSize := bytesToUint64(fileSizeBytes)

	blockData, err := s.fileStore.Load(ctx, filePath)
	if err != nil || len(blockData) == 0 {
		h := height
		originalErr := err
		if err == nil {
			originalErr = fmt.Errorf("empty file data")
		}

		// 🆕 路径重试机制：如果索引中的路径无效（如 ../blocks/...），尝试用标准路径重试
		// 标准路径格式：blocks/{heightSegment:010d}/{height:010d}.bin
		errClass := corruptutil.ClassifyErr(originalErr)
		if errClass == "index_path_corrupt" || errClass == "block_file_missing" {
			seg := (height / 1000) * 1000
			standardPath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, height)

			// 仅当索引路径与标准路径不同时才重试
			if standardPath != filePath {
				if s.logger != nil {
					s.logger.Warnf("🔧 索引路径异常，尝试标准路径重试: height=%d indexPath=%s standardPath=%s err=%v",
						height, filePath, standardPath, originalErr)
				}

				retryData, retryErr := s.fileStore.Load(ctx, standardPath)
				if retryErr == nil && len(retryData) > 0 {
					// 标准路径加载成功！使用重试数据
					blockData = retryData
					err = nil
					originalPath := filePath
					filePath = standardPath // 更新路径用于后续日志

					if s.logger != nil {
						s.logger.Infof("✅ 标准路径重试成功: height=%d path=%s", height, standardPath)
					}

					// 🆕 立即同步修复索引（直接写入正确路径到索引）
					// 这比异步事件更可靠，因为异步事件有节流机制
					go s.repairHeightIndexPath(ctx, height, indexData, standardPath, uint64(len(retryData)), originalPath)
				}
			}
		}

		// 如果重试后仍然失败，返回原始错误
		if err != nil || len(blockData) == 0 {
			s.publishCorruptionDetected(types.CorruptionPhaseReadBlock, types.CorruptionSeverityWarning, &h, "", filePath, originalErr)
			return nil, fmt.Errorf("读取区块文件失败: %w", originalErr)
		}
	}
	if expectedSize > 0 && uint64(len(blockData)) != expectedSize {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseValidate, types.CorruptionSeverityWarning, &h, "", filePath,
			fmt.Errorf("block file size mismatch expected=%d got=%d", expectedSize, len(blockData)))
		// size mismatch 不一定致命：仍尝试解码，避免误杀（但会在日志/事件中暴露）
	}

	// 反序列化区块
	block := &core.Block{}
	if err := proto.Unmarshal(blockData, block); err != nil {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseReadBlock, types.CorruptionSeverityCritical, &h, "", filePath, err)
		return nil, fmt.Errorf("反序列化区块失败: %w", err)
	}
	if block.Header == nil || block.Header.Height != height {
		h := height
		s.publishCorruptionDetected(types.CorruptionPhaseValidate, types.CorruptionSeverityCritical, &h, "", filePath,
			fmt.Errorf("block height mismatch: expected=%d got=%v", height, func() interface{} {
				if block.Header == nil {
					return nil
				}
				return block.Header.Height
			}()))
		return nil, fmt.Errorf("区块数据高度不匹配: expected=%d got=%v", height, func() interface{} {
			if block.Header == nil {
				return nil
			}
			return block.Header.Height
		}())
	}

	// 写入内存缓存
	if s.blockCache != nil {
		s.blockCache.Put(height, block)
	}

	return block, nil
}

// GetBlockByHash 按哈希获取区块
func (s *Service) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) {
	// 1. 根据哈希获取区块高度
	// 键格式：indices:hash:{hash}
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
	heightBytes, err := s.storage.Get(ctx, hashKey)
	if err != nil {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityWarning, nil, fmt.Sprintf("%x", blockHash), string(hashKey), err)
		return nil, fmt.Errorf("获取区块高度失败: %w", err)
	}

	if len(heightBytes) != 8 {
		// BadgerStore.Get：key 不存在时 (nil, nil)，因此 got_len=0 往往是“索引缺失”而非“索引损坏”。
		// 这里将 len==0 视为 Warning，避免把“缺索引”过度上升为 Critical。
		severity := types.CorruptionSeverityCritical
		if len(heightBytes) == 0 {
			severity = types.CorruptionSeverityWarning
		}
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, severity, nil, fmt.Sprintf("%x", blockHash), string(hashKey),
			fmt.Errorf("区块高度数据格式错误：长度应为8字节 (key=%s hash=%x got_len=%d)", string(hashKey), blockHash, len(heightBytes)))

		// ✅ 快速自愈：先做 tip 附近小窗口探测（避免每次都触发昂贵扫描或被节流）
		if ok, fastErr := s.tryRepairHashIndexFast(ctx, blockHash, 256); fastErr == nil && ok {
			heightBytes, err = s.storage.Get(ctx, hashKey)
			if err != nil {
				return nil, fmt.Errorf("获取区块高度失败(快速修复后重试): %w", err)
			}
			if len(heightBytes) == 8 {
				height := bytesToUint64(heightBytes)
				return s.GetBlockByHeight(ctx, height)
			}
			// fast 修复未能得到有效值：继续走重扫描修复
		}

		// ✅ 生产化自愈：索引损坏/缺失时尝试重扫描修复一次，再重试读取
		if repairErr := s.tryRepairHashIndex(ctx, blockHash); repairErr == nil {
			heightBytes, err = s.storage.Get(ctx, hashKey)
			if err != nil {
				return nil, fmt.Errorf("获取区块高度失败(修复后重试): %w", err)
			}
			if len(heightBytes) != 8 {
				return nil, fmt.Errorf("区块高度数据格式错误：长度应为8字节 (key=%s hash=%x got_len=%d repair=applied_but_still_invalid)",
					string(hashKey), blockHash, len(heightBytes))
			}
		} else {
			if s.logger != nil {
				s.logger.Warnf("🩹 auto_repair failed: hash=%x err=%v", blockHash, repairErr)
			}
			s.publishRepairResult(false, "rebuild_hash_index", string(hashKey), fmt.Sprintf("%x", blockHash), nil, "scan window failed", repairErr)
			return nil, fmt.Errorf("区块高度数据格式错误：长度应为8字节 (key=%s hash=%x got_len=%d)；auto_repair_failed=%v",
				string(hashKey), blockHash, len(heightBytes), repairErr)
		}
	}

	height := bytesToUint64(heightBytes)

	// 2. 根据高度获取区块（复用 GetBlockByHeight）
	return s.GetBlockByHeight(ctx, height)
}

// GetBlockHeader 获取区块头
func (s *Service) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	// 获取完整区块
	block, err := s.GetBlockByHash(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	// 返回区块头
	return block.Header, nil
}

// GetBlockRange 获取区块范围
func (s *Service) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	// 验证参数
	if startHeight > endHeight {
		return nil, fmt.Errorf("起始高度不能大于结束高度")
	}

	// 获取区块列表
	blocks := make([]*core.Block, 0, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		block, err := s.GetBlockByHeight(ctx, height)
		if err != nil {
			return nil, fmt.Errorf("获取高度 %d 的区块失败: %w", height, err)
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetHighestBlock 获取最高区块信息
func (s *Service) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	// 获取链尖状态（遵循 data-architecture.md 规范）
	// 键格式：state:chain:tip
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityWarning, nil, "", string(tipKey), err)
		return 0, nil, fmt.Errorf("获取链尖状态失败: %w", err)
	}

	if len(tipData) < 40 {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityCritical, nil, "", string(tipKey), fmt.Errorf("链尖数据格式错误"))
		return 0, nil, fmt.Errorf("链尖数据格式错误")
	}

	height = bytesToUint64(tipData[:8])
	blockHash = tipData[8:40]

	return height, blockHash, nil
}

// bytesToUint64 将字节数组转换为uint64
func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// repairHeightIndexPath 立即修复索引中的错误路径
//
// 🎯 **同步修复策略**：
// 当检测到索引路径损坏（如 ../blocks/...）并通过标准路径成功加载区块后，
// 立即修复索引，而不是依赖异步事件（异步事件有节流机制，大量损坏时效率低）。
//
// 参数：
//   - ctx: 上下文
//   - height: 区块高度
//   - oldIndexData: 原始索引数据（包含 blockHash）
//   - correctPath: 正确的路径
//   - fileSize: 文件大小
//   - originalPath: 原始错误路径（用于日志）
func (s *Service) repairHeightIndexPath(ctx context.Context, height uint64, oldIndexData []byte, correctPath string, fileSize uint64, originalPath string) {
	if s.storage == nil {
		return
	}

	// 从原始索引数据中提取 blockHash（前32字节）
	if len(oldIndexData) < 32 {
		if s.logger != nil {
			s.logger.Warnf("🔧 索引修复失败: height=%d 原始索引数据不足 (len=%d)", height, len(oldIndexData))
		}
		return
	}
	blockHash := oldIndexData[:32]

	// 构建新的索引值：blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)
	pathBytes := []byte(correctPath)
	newIndexValue := make([]byte, 32+1+len(pathBytes)+8)
	copy(newIndexValue[0:32], blockHash)
	newIndexValue[32] = byte(len(pathBytes))
	copy(newIndexValue[33:33+len(pathBytes)], pathBytes)
	copy(newIndexValue[33+len(pathBytes):41+len(pathBytes)], uint64ToBytes(fileSize))

	// 写入修复后的索引
	heightKey := fmt.Sprintf("indices:height:%d", height)
	if err := s.storage.Set(ctx, []byte(heightKey), newIndexValue); err != nil {
		if s.logger != nil {
			s.logger.Warnf("🔧 索引修复写入失败: height=%d err=%v", height, err)
		}
		return
	}

	if s.logger != nil {
		s.logger.Infof("🔧 索引路径已修复: height=%d oldPath=%s newPath=%s", height, originalPath, correctPath)
	}
}

// 编译时检查接口实现
var _ interfaces.InternalBlockQuery = (*Service)(nil)
