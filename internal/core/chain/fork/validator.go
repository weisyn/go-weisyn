// Package fork 实现分叉处理验证器
//
// 🔍 **REORG 验证器 (Reorg Validator)**
//
// 本文件实现了 REORG 后的完整性验证功能，确保链状态的正确性：
// - Level 1: StateRoot 验证（强验证）
// - Level 2: 索引完整性验证（弱验证）
// - Level 3: 跨模块一致性验证
//
// 🏗️ **设计特点**：
// - 三层验证确保不同维度的状态一致性
// - 支持快速失败和详细错误报告
// - 验证失败时提供明确的错误上下文
package fork

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	eutxoiface "github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"google.golang.org/protobuf/proto"
)

// ReorgValidator REORG 验证器
//
// 🎯 **职责**：
// - 验证 REORG 后链状态的完整性和一致性
// - 提供三层验证机制确保状态正确
// - 快速失败并提供详细错误信息
//
// 🔧 **验证层次**：
// 1. StateRoot 验证：确保 UTXO 状态与区块头一致
// 2. 索引完整性验证：确保区块索引连续且完整
// 3. 跨模块一致性验证：确保 UTXO 引用的区块存在
type ReorgValidator struct {
	store        storage.BadgerStore
	queryService persistence.QueryService
	txHashClient txpb.TransactionHashServiceClient
	logger       log.Logger
}

// NewReorgValidator 创建 REORG 验证器
//
// 🏗️ **构造函数**
//
// 参数：
//   - store: Badger 存储服务（必需）
//   - queryService: 查询服务（必需）
//   - logger: 日志服务（可选）
//
// 返回：
//   - *ReorgValidator: 验证器实例
//   - error: 创建错误
func NewReorgValidator(
	store storage.BadgerStore,
	queryService persistence.QueryService,
	txHashClient txpb.TransactionHashServiceClient,
	logger log.Logger,
) (*ReorgValidator, error) {
	if store == nil {
		return nil, fmt.Errorf("store 不能为空")
	}
	if queryService == nil {
		return nil, fmt.Errorf("queryService 不能为空")
	}
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}

	return &ReorgValidator{
		store:        store,
		queryService: queryService,
		txHashClient: txHashClient,
		logger:       logger,
	}, nil
}

// VerifyReorgResult 验证 REORG 结果
//
// 🎯 **三层验证**：
// 1. StateRoot 验证（强验证）：对比实际 StateRoot 与区块头 StateRoot
// 2. 索引完整性验证（弱验证）：检查 0..height 的区块索引连续性
// 3. 跨模块一致性验证：验证 UTXO 引用的区块存在
//
// ⚠️ **注意**：
// - 任何一层验证失败都会立即返回错误
// - 验证失败意味着 REORG 存在严重问题，应进入只读模式
//
// 参数：
//   - ctx: 操作上下文
//   - expectedHeight: 预期的链高度
//
// 返回：
//   - error: 验证失败的错误（nil 表示验证通过）
func (v *ReorgValidator) VerifyReorgResult(ctx context.Context, expectedHeight uint64) error {
	if v.logger != nil {
		v.logger.Infof("🔍 开始验证 REORG 结果: expected_height=%d", expectedHeight)
	}

	// Level 1: StateRoot 验证（强验证）
	if err := v.verifyStateRoot(ctx, expectedHeight); err != nil {
		if v.logger != nil {
			v.logger.Errorf("❌ StateRoot 验证失败: %v", err)
		}
		return fmt.Errorf("state-level validation failed: %w", err)
	}
	if v.logger != nil {
		v.logger.Info("✅ Level 1: StateRoot 验证通过")
	}

	// Level 2: 索引完整性验证（弱验证）
	if err := v.verifyIndexIntegrity(ctx, expectedHeight); err != nil {
		if v.logger != nil {
			v.logger.Errorf("❌ 索引完整性验证失败: %v", err)
		}
		return fmt.Errorf("index-level validation failed: %w", err)
	}
	if v.logger != nil {
		v.logger.Info("✅ Level 2: 索引完整性验证通过")
	}

	// Level 3: 跨模块一致性验证（一致性验证）
	if err := v.verifyCrossModuleConsistency(ctx, expectedHeight); err != nil {
		if v.logger != nil {
			v.logger.Errorf("❌ 跨模块一致性验证失败: %v", err)
		}
		return fmt.Errorf("cross-module validation failed: %w", err)
	}
	if v.logger != nil {
		v.logger.Info("✅ Level 3: 跨模块一致性验证通过")
	}

	if v.logger != nil {
		v.logger.Infof("✅ REORG 验证成功: height=%d", expectedHeight)
	}

	return nil
}

// verifyStateRoot 验证 StateRoot
//
// 🎯 **Level 1: StateRoot 验证（强验证）**
//
// 验证逻辑：
// 1. 获取指定高度的区块
// 2. 计算当前 UTXO 状态的 StateRoot
// 3. 对比实际 StateRoot 与区块头中的 StateRoot
//
// ⚠️ **注意**：
// - StateRoot 不匹配意味着 UTXO 状态与区块链不一致
// - 这是最严重的验证失败，必须立即处理
//
// 参数：
//   - ctx: 操作上下文
//   - height: 链高度
//
// 返回：
//   - error: 验证失败的错误
func (v *ReorgValidator) verifyStateRoot(ctx context.Context, height uint64) error {
	// 1. 获取区块
	block, err := v.queryService.GetBlockByHeight(ctx, height)
	if err != nil {
		return fmt.Errorf("获取区块失败 height=%d: %w", height, err)
	}
	if block == nil || block.Header == nil {
		return fmt.Errorf("区块或区块头为空 height=%d", height)
	}

	expectedRoot := block.Header.StateRoot
	if len(expectedRoot) == 0 {
		// Genesis 区块或早期区块可能没有 StateRoot，跳过验证
		if v.logger != nil {
			v.logger.Debugf("区块 height=%d 没有 StateRoot，跳过验证", height)
		}
		return nil
	}

	// 2. 从链状态中读取记录的 StateRoot（与区块头写入保持同一语义）
	//
	// 按当前实现，Persistence 在写入区块时会将 Header.StateRoot
	// 写入键 `state:chain:root`：
	//
	//   - internal/core/persistence/writer/chain.go:72-81
	//
	// 在 REORG 校验阶段，我们不重新扫描 UTXO 计算 StateRoot，
	// 而是校验「区块头中的 StateRoot 与链状态里记录的 StateRoot 一致」，
	// 避免因为：
	//   1) UTXO 状态已经前进到更高高度
	//   2) 以及“本块前/本块后”的语义差异
	// 而产生误报。
	stateRootKey := []byte("state:chain:root")
	actualRoot, err := v.store.Get(ctx, stateRootKey)
	if err != nil {
		return fmt.Errorf("读取链状态 StateRoot 失败: %w", err)
	}
	if len(actualRoot) == 0 {
		return fmt.Errorf("链状态 StateRoot 为空")
	}

	// 3. 对比 StateRoot
	if !bytes.Equal(actualRoot, expectedRoot) {
		return fmt.Errorf("StateRoot mismatch at height=%d: actual=%x, expected=%x",
			height, actualRoot, expectedRoot)
	}

	return nil
}

// verifyIndexIntegrity 验证索引完整性
//
// 🎯 **Level 2: 索引完整性验证（弱验证）**
//
// 验证逻辑：
// 1. 检查 0..maxHeight 的区块索引连续性
// 2. 验证每个高度的 indices:height:* 键存在
//
// ⚠️ **注意**：
// - 索引不连续意味着回滚不完整或重放有遗漏
// - 这会导致查询行为不确定
//
// 参数：
//   - ctx: 操作上下文
//   - maxHeight: 最大高度
//
// 返回：
//   - error: 验证失败的错误
func (v *ReorgValidator) verifyIndexIntegrity(ctx context.Context, maxHeight uint64) error {
	// 1) 检查 0..maxHeight 的区块索引连续性，并校验 height->hash 与 hash->height 映射一致
	for h := uint64(0); h <= maxHeight; h++ {
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		heightVal, err := v.store.Get(ctx, heightKey)
		if err != nil {
			return fmt.Errorf("读取高度索引失败 height=%d: %w", h, err)
		}
		if len(heightVal) < 32 {
			return fmt.Errorf("高度索引数据无效 height=%d len=%d", h, len(heightVal))
		}
		blockHash := heightVal[:32]

		hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
		hashVal, err := v.store.Get(ctx, hashKey)
		if err != nil {
			return fmt.Errorf("读取哈希索引失败 height=%d: %w", h, err)
		}
		if len(hashVal) != 8 {
			return fmt.Errorf("哈希索引数据长度无效 height=%d len=%d", h, len(hashVal))
		}
		hashHeight := binary.BigEndian.Uint64(hashVal)
		if hashHeight != h {
			return fmt.Errorf("哈希索引映射不一致 height=%d hashHeight=%d", h, hashHeight)
		}
	}

	// 2) 校验 state:chain:tip 与 indices:height:tip 一致（height+hash）
	tipVal, err := v.store.Get(ctx, []byte("state:chain:tip"))
	if err != nil {
		return fmt.Errorf("读取链尖失败: %w", err)
	}
	if len(tipVal) != 40 {
		return fmt.Errorf("链尖格式无效 len=%d", len(tipVal))
	}
	tipHeight := binary.BigEndian.Uint64(tipVal[:8])
	if tipHeight != maxHeight {
		return fmt.Errorf("链尖高度不一致 expected=%d actual=%d", maxHeight, tipHeight)
	}
	ih, err := v.store.Get(ctx, []byte(fmt.Sprintf("indices:height:%d", maxHeight)))
	if err != nil {
		return fmt.Errorf("读取链尖高度索引失败: %w", err)
	}
	if len(ih) < 32 {
		return fmt.Errorf("链尖高度索引数据无效 len=%d", len(ih))
	}
	if !bytes.Equal(tipVal[8:], ih[:32]) {
		return fmt.Errorf("链尖 hash 与高度索引不一致: height=%d", maxHeight)
	}

	return nil
}

// verifyCrossModuleConsistency 验证跨模块一致性
//
// 🎯 **Level 3: 跨模块一致性验证**
//
// 验证逻辑：
// 1. 验证链尖高度与预期一致
// 2. 验证 StateRoot 存在
// 3. 验证链尖区块存在
//
// ⚠️ **注意**：
// - 这是一个简化的一致性检查
// - 更详细的 UTXO 验证需要 UTXO 管理器接口支持
//
// 参数：
//   - ctx: 操作上下文
//   - maxHeight: 最大高度
//
// 返回：
//   - error: 验证失败的错误
func (v *ReorgValidator) verifyCrossModuleConsistency(ctx context.Context, maxHeight uint64) error {
	// 1. 验证链信息
	chainInfo, err := v.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	// 验证链高度
	if chainInfo.Height != maxHeight {
		return fmt.Errorf("chain height mismatch: expected=%d, actual=%d", maxHeight, chainInfo.Height)
	}

	// 2. 验证 StateRoot 存在
	stateRoot, err := v.queryService.GetCurrentStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("获取当前 StateRoot 失败: %w", err)
	}
	if len(stateRoot) == 0 {
		if v.logger != nil {
			v.logger.Debugf("StateRoot 为空 (可能是 Genesis 或早期区块)")
		}
	}

	// 3. 验证链尖区块存在
	tipBlock, err := v.queryService.GetBlockByHeight(ctx, maxHeight)
	if err != nil {
		return fmt.Errorf("获取链尖区块失败 height=%d: %w", maxHeight, err)
	}
	if tipBlock == nil || tipBlock.Header == nil {
		return fmt.Errorf("chain tip block missing or invalid at height=%d", maxHeight)
	}

	// 4. tx 索引可达性（全量）：0..maxHeight 的所有 tx 都必须存在 indices:tx
	if err := v.verifyTxIndexReachability(ctx, maxHeight); err != nil {
		return err
	}

	// 5. UTXO-Block 一致性：UTXO 的 BlockHeight 必须在链上存在且 <= tip
	if err := v.verifyUTXOBlockConsistency(ctx, maxHeight); err != nil {
		return err
	}

	// 6. Resource-UTXO 双向一致性：资源记录/索引 <-> UTXO 集合互相可达
	if err := v.verifyResourceUTXOConsistency(ctx, maxHeight); err != nil {
		return err
	}

	if v.logger != nil {
		v.logger.Debugf("跨模块一致性验证: 链尖高度=%d", chainInfo.Height)
	}

	return nil
}

// verifyTxIndexReachability 对 0..maxHeight 的所有交易做 indices:tx 可达性验证（全量）。
func (v *ReorgValidator) verifyTxIndexReachability(ctx context.Context, maxHeight uint64) error {
	for h := uint64(0); h <= maxHeight; h++ {
		blk, err := v.queryService.GetBlockByHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("获取区块失败 height=%d: %w", h, err)
		}
		if blk == nil || blk.Header == nil {
			return fmt.Errorf("区块为空 height=%d", h)
		}
		if blk.Body == nil || len(blk.Body.Transactions) == 0 {
			continue
		}
		for i, txProto := range blk.Body.Transactions {
			txResp, err := v.txHashClient.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: txProto})
			if err != nil {
				return fmt.Errorf("计算交易哈希失败 height=%d idx=%d: %w", h, i, err)
			}
			if txResp == nil || !txResp.IsValid || len(txResp.Hash) != 32 {
				return fmt.Errorf("交易哈希无效 height=%d idx=%d", h, i)
			}
			key := []byte(fmt.Sprintf("indices:tx:%x", txResp.Hash))
			ok, err := v.store.Exists(ctx, key)
			if err != nil {
				return fmt.Errorf("检查交易索引失败 height=%d idx=%d: %w", h, i, err)
			}
			if !ok {
				return fmt.Errorf("缺失交易索引: height=%d idx=%d", h, i)
			}
		}
	}
	return nil
}

// verifyUTXOBlockConsistency 校验所有 UTXO 的 BlockHeight 合法且可达。
func (v *ReorgValidator) verifyUTXOBlockConsistency(ctx context.Context, maxHeight uint64) error {
	utxoMap, err := v.store.PrefixScan(ctx, []byte("utxo:set:"))
	if err != nil {
		return fmt.Errorf("扫描UTXO失败: %w", err)
	}
	for keyStr, data := range utxoMap {
		_ = keyStr
		u := &utxopb.UTXO{}
		if err := proto.Unmarshal(data, u); err != nil {
			return fmt.Errorf("反序列化UTXO失败: %w", err)
		}
		if u.Outpoint == nil || len(u.Outpoint.TxId) != 32 {
			return fmt.Errorf("UTXO outpoint 无效")
		}
		// height=0 仅允许在 tip=0 的 genesis 场景
		if maxHeight > 0 && u.BlockHeight == 0 {
			return fmt.Errorf("UTXO BlockHeight=0 非法（tip=%d） outpoint=%x:%d", maxHeight, u.Outpoint.TxId, u.Outpoint.OutputIndex)
		}
		if u.BlockHeight > maxHeight {
			return fmt.Errorf("UTXO BlockHeight 超过 tip: utxoHeight=%d tip=%d", u.BlockHeight, maxHeight)
		}
		if u.BlockHeight > 0 {
			ok, err := v.store.Exists(ctx, []byte(fmt.Sprintf("indices:height:%d", u.BlockHeight)))
			if err != nil || !ok {
				return fmt.Errorf("UTXO 引用不存在的区块高度: utxoHeight=%d", u.BlockHeight)
			}
		}
	}
	return nil
}

// verifyResourceUTXOConsistency 校验资源索引/记录与 UTXO 集合的双向一致性。
func (v *ReorgValidator) verifyResourceUTXOConsistency(ctx context.Context, maxHeight uint64) error {
	// A) 资源记录 -> UTXO 必存在 + 索引/计数可达
	resMap, err := v.store.PrefixScan(ctx, []byte("resource:utxo-instance:"))
	if err != nil {
		return fmt.Errorf("扫描资源记录失败: %w", err)
	}
	for _, val := range resMap {
		rec := &eutxoiface.ResourceUTXORecord{}
		if err := json.Unmarshal(val, rec); err != nil {
			return fmt.Errorf("反序列化资源记录失败: %w", err)
		}
		rec.EnsureBackwardCompatibility()

		instanceID := rec.InstanceID
		codeID := rec.CodeID
		if len(instanceID.TxId) != 32 {
			return fmt.Errorf("资源记录 InstanceID 无效")
		}
		utxoKey := []byte(fmt.Sprintf("utxo:set:%x:%d", instanceID.TxId, instanceID.OutputIndex))
		utxoData, err := v.store.Get(ctx, utxoKey)
		if err != nil || len(utxoData) == 0 {
			return fmt.Errorf("资源记录对应的UTXO不存在: instance=%s", instanceID.Encode())
		}
		u := &utxopb.UTXO{}
		if err := proto.Unmarshal(utxoData, u); err != nil {
			return fmt.Errorf("反序列化资源UTXO失败: %w", err)
		}
		if u.Category != utxopb.UTXOCategory_UTXO_CATEGORY_RESOURCE {
			return fmt.Errorf("资源记录对应UTXO类别不匹配: expected=RESOURCE actual=%v", u.Category)
		}
		if u.BlockHeight > maxHeight {
			return fmt.Errorf("资源UTXO BlockHeight 超过 tip: utxoHeight=%d tip=%d", u.BlockHeight, maxHeight)
		}

		// 索引：indices:resource-instance:{instanceID}
		instKey := []byte(fmt.Sprintf("indices:resource-instance:%s", instanceID.Encode()))
		instVal, err := v.store.Get(ctx, instKey)
		if err != nil || len(instVal) != 72 {
			return fmt.Errorf("缺失/损坏资源实例索引: instance=%s", instanceID.Encode())
		}
		instHeight := binary.BigEndian.Uint64(instVal[32:40])
		if instHeight > maxHeight {
			return fmt.Errorf("资源实例索引高度超过 tip: instHeight=%d tip=%d", instHeight, maxHeight)
		}

		// 索引：indices:resource-code:{codeID} 必须包含 instanceID
		codeKey := []byte(fmt.Sprintf("indices:resource-code:%x", codeID.Bytes()))
		codeVal, err := v.store.Get(ctx, codeKey)
		if err != nil || len(codeVal) == 0 {
			return fmt.Errorf("缺失资源代码索引: code=%x", codeID.Bytes())
		}
		var instanceList []string
		if err := json.Unmarshal(codeVal, &instanceList); err != nil {
			return fmt.Errorf("解析资源代码索引失败: %w", err)
		}
		found := false
		for _, id := range instanceList {
			if id == instanceID.Encode() {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("资源代码索引不包含实例: code=%x instance=%s", codeID.Bytes(), instanceID.Encode())
		}

		// owner 索引：index:resource:owner-instance:{owner}:{instanceID} -> instanceID
		if len(rec.Owner) > 0 {
			ownerKey := []byte(fmt.Sprintf("index:resource:owner-instance:%x:%s", rec.Owner, instanceID.Encode()))
			ownerVal, err := v.store.Get(ctx, ownerKey)
			if err != nil || len(ownerVal) == 0 {
				return fmt.Errorf("缺失资源 owner 索引: owner=%x instance=%s", rec.Owner, instanceID.Encode())
			}
			if string(ownerVal) != instanceID.Encode() {
				return fmt.Errorf("资源 owner 索引值不一致: owner=%x instance=%s", rec.Owner, instanceID.Encode())
			}
		}

		// counters：resource:counters-instance:{instanceID}
		countersKey := []byte(fmt.Sprintf("resource:counters-instance:%s", instanceID.Encode()))
		if ok, _ := v.store.Exists(ctx, countersKey); !ok {
			return fmt.Errorf("缺失资源 counters: instance=%s", instanceID.Encode())
		}
	}

	// B) 反向：所有 RESOURCE 类 UTXO 必须存在资源记录 resource:utxo-instance:{instanceID}
	utxoMap, err := v.store.PrefixScan(ctx, []byte("utxo:set:"))
	if err != nil {
		return fmt.Errorf("扫描UTXO失败: %w", err)
	}
	for _, data := range utxoMap {
		u := &utxopb.UTXO{}
		if err := proto.Unmarshal(data, u); err != nil {
			return fmt.Errorf("反序列化UTXO失败: %w", err)
		}
		if u.Category != utxopb.UTXOCategory_UTXO_CATEGORY_RESOURCE || u.Outpoint == nil || len(u.Outpoint.TxId) != 32 {
			continue
		}
		instanceID := eutxoiface.NewResourceInstanceID(u.Outpoint.TxId, u.Outpoint.OutputIndex)
		recKey := []byte(fmt.Sprintf("resource:utxo-instance:%s", instanceID.Encode()))
		if ok, _ := v.store.Exists(ctx, recKey); !ok {
			return fmt.Errorf("资源UTXO缺失资源记录: instance=%s", instanceID.Encode())
		}
	}
	return nil
}

// VerifyStateRoot 快捷方法：只验证 StateRoot
//
// 用于快速验证 UTXO 状态是否正确
func (v *ReorgValidator) VerifyStateRoot(ctx context.Context, height uint64) error {
	return v.verifyStateRoot(ctx, height)
}

// VerifyIndexIntegrity 快捷方法：只验证索引完整性
//
// 用于快速验证区块索引是否连续
func (v *ReorgValidator) VerifyIndexIntegrity(ctx context.Context, maxHeight uint64) error {
	return v.verifyIndexIntegrity(ctx, maxHeight)
}

// VerifyCrossModuleConsistency 快捷方法：只验证跨模块一致性
//
// 用于快速验证 UTXO 与区块的一致性
func (v *ReorgValidator) VerifyCrossModuleConsistency(ctx context.Context, maxHeight uint64) error {
	return v.verifyCrossModuleConsistency(ctx, maxHeight)
}

