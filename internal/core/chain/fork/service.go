// Package fork 实现分叉处理服务
//
// 🔄 **分叉处理服务 (Fork Handler Service)**
//
// 本包实现了区块链分叉的检测和处理功能，负责：
// - 分叉检测
// - 链权重比较
// - 链切换决策
// - 分叉指标收集
//
// 🏗️ **设计特点**：
// - 依赖 QueryService 获取链状态
// - 使用事件驱动通信
// - 提供完整的指标收集
// - 支持延迟依赖注入
//
// ⚠️ **注意**：这是从 blockchain/fork 重构的简化版本
package fork

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/chain/fork/reorg"
	"github.com/weisyn/v1/internal/core/chain/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	mempoolif "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// ============================================================================
//                              依赖接口定义
// ============================================================================

// 注意：以下接口使用公共接口定义，符合 code-organization.md 规范
// - BlockProcessor: 使用 pkg/interfaces/block.BlockProcessor
// - UTXOSnapshot: 使用 pkg/interfaces/eutxo.UTXOSnapshot
//
// 这些接口不应该在实现文件中重新定义，而应该直接使用公共接口或内部接口

// ============================================================================
//                              服务结构定义
// ============================================================================

// Service 分叉处理服务实现
//
// 🎯 **职责**：
// - 实现 InternalForkHandler 接口
// - 检测区块链分叉
// - 处理分叉区块
// - 执行链重组
// - 收集分叉指标
//
// 🔧 **并发安全**：
// - 使用互斥锁保护分叉处理
// - 确保同一时间只处理一个分叉
type Service struct {
	// 依赖
	queryService    persistence.QueryService
	hasher          crypto.HashManager
	blockHashClient core.BlockHashServiceClient
	txHashClient    txpb.TransactionHashServiceClient
	configProvider  config.Provider // 🔧 配置提供者，用于获取分叉处理相关配置（例如最大分叉深度）
	logger          log.Logger
	eventBus        eventiface.EventBus // 可选：发布 corruption.detected（reorg相关）
	store           storage.BadgerStore // ✅ 用于 reorg 的状态清理（UTXO/索引/链尖）

	// 延迟注入的依赖
	// 使用公共接口，符合 public-interface-design.md 和 code-organization.md 规范
	blockProcessor block.BlockProcessor
	utxoSnapshot   eutxo.UTXOSnapshot
	dataWriter     persistence.DataWriter
	txPool         mempoolif.TxPool

	// 状态（需要并发保护）
	mu                sync.Mutex
	isProcessingFork  bool
	currentForkHeight uint64

	// 只读模式状态
	writeGate writegate.WriteGate

	// 指标（需要并发保护）
	metrics   *interfaces.ForkMetrics
	metricsMu sync.RWMutex
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewService 创建分叉处理服务
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数：
//   - queryService: 查询服务（必需）
//   - hasher: 哈希管理器（必需）
//   - blockHashClient: 区块哈希服务客户端（必需）
//   - configProvider: 配置提供者（可选，用于获取默认难度值）
//   - logger: 日志服务（可选）
//
// 返回：
//   - interfaces.InternalForkHandler: 内部分叉处理接口
//   - error: 创建错误
//
// 设计说明：
// - 验证必需依赖
// - 初始化内部状态
// - BlockProcessor 和 UTXOSnapshot 通过 SetXXX 方法延迟注入
func NewService(
	queryService persistence.QueryService,
	hasher crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	txHashClient txpb.TransactionHashServiceClient,
	store storage.BadgerStore,
	configProvider config.Provider,
	eventBus eventiface.EventBus,
	logger log.Logger,
) (interfaces.InternalForkHandler, error) {
	if queryService == nil {
		return nil, fmt.Errorf("queryService 不能为空")
	}
	if hasher == nil {
		return nil, fmt.Errorf("hasher 不能为空")
	}
	if blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 不能为空")
	}
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}

	service := &Service{
		queryService:    queryService,
		hasher:          hasher,
		blockHashClient: blockHashClient,
		txHashClient:    txHashClient,
		store:           store,
		configProvider:  configProvider,
		eventBus:        eventBus,
		logger:          logger,
		writeGate:       writegate.Default(),
		metrics: &interfaces.ForkMetrics{
			TotalForks:    0,
			ResolvedForks: 0,
			PendingForks:  0,
		},
	}

	if logger != nil {
		logger.Info("✅ ForkHandler 服务已创建")
	}

	return service, nil
}

func (s *Service) publishCorruptionDetected(ctx context.Context, phase types.CorruptionPhase, severity types.CorruptionSeverity, height *uint64, hashHex string, key string, err error) {
	if s == nil || s.eventBus == nil || err == nil {
		return
	}
	data := types.CorruptionEventData{
		Component: types.CorruptionComponentFork,
		Phase:     phase,
		Severity:  severity,
		Height:    height,
		Hash:      hashHex,
		Key:       key,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        types.RFC3339Time(time.Now()),
	}
	// 事件总线约定：args[0]=ctx, args[1]=data
	s.eventBus.Publish(eventiface.EventTypeCorruptionDetected, ctx, data)
}

// ============================================================================
//                              延迟依赖注入
// ============================================================================

// SetBlockProcessor 设置区块处理器（延迟注入）
//
// 用于解决循环依赖问题
// 使用公共接口 BlockProcessor，符合 code-organization.md 规范
func (s *Service) SetBlockProcessor(processor block.BlockProcessor) {
	s.blockProcessor = processor
	if s.logger != nil {
		s.logger.Info("🔗 BlockProcessor 已注入到 ForkHandler")
	}
}

// SetUTXOSnapshot 设置UTXO快照服务（延迟注入）
//
// 用于解决循环依赖问题
// 使用公共接口 eutxo.UTXOSnapshot，符合 code-organization.md 规范
func (s *Service) SetUTXOSnapshot(snapshot eutxo.UTXOSnapshot) {
	s.utxoSnapshot = snapshot
	if s.logger != nil {
		s.logger.Info("🔗 UTXOSnapshot 已注入到 ForkHandler")
	}
}

// SetDataWriter 设置数据写入服务（延迟注入）
//
// 用于分叉处理时删除原主链的交易索引
// 使用公共接口 persistence.DataWriter，符合 code-organization.md 规范
func (s *Service) SetDataWriter(writer persistence.DataWriter) {
	s.dataWriter = writer
	if s.logger != nil {
		s.logger.Info("🔗 DataWriter 已注入到 ForkHandler")
	}
}

// SetTxPool 设置交易池（延迟注入）
//
// 用于 reorg 后回收（detached）区块中的交易，并回注到 mempool，形成生产级闭环。
func (s *Service) SetTxPool(pool mempoolif.TxPool) {
	s.txPool = pool
	if s.logger != nil {
		s.logger.Info("🔗 TxPool 已注入到 ForkHandler")
	}
}

// ============================================================================
//                              链状态回滚（REORG核心）
// ============================================================================

// RollbackToHeight 回滚链状态到指定高度
//
// 🎯 **功能**：
// - 删除 height+1 及以后的所有区块数据（Badger + FileStore）
// - 删除对应的交易索引、区块哈希索引
// - 更新链尖状态到 height
//
// ⚠️ **注意**：
// - 此方法会修改链状态，必须在事务中调用或确保原子性
// - 调用前应创建UTXO快照用于失败恢复
// - 回滚后链尖高度会变为 height
//
// 参数：
//   - ctx: 操作上下文
//   - height: 目标高度（回滚到此高度）
//
// 返回：
//   - error: 回滚失败的错误
func (s *Service) RollbackToHeight(ctx context.Context, height uint64) error {
	if s.store == nil {
		return fmt.Errorf("BadgerStore 未注入，无法执行回滚操作")
	}

	if s.queryService == nil {
		return fmt.Errorf("QueryService 未注入，无法获取当前链状态")
	}

	// 1. 获取当前链高度
	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}
	currentHeight := chainInfo.Height

	// 2. 验证回滚高度
	if height > currentHeight {
		return fmt.Errorf("回滚高度(%d)不能大于当前高度(%d)", height, currentHeight)
	}

	if height == currentHeight {
		// 无需回滚
		if s.logger != nil {
			s.logger.Infof("回滚高度等于当前高度，无需操作: height=%d", height)
		}
		return nil
	}

	if s.logger != nil {
		s.logger.Warnf("🔁 开始回滚链状态: 从高度%d回滚到高度%d（将删除%d个区块）",
			currentHeight, height, currentHeight-height)
	}

	// 3) 预收集需要删除的索引键（避免在 Badger 事务中调用 QueryService 造成不可预期的嵌套读/缓存问题）
	//
	// 说明：
	// - 当前区块存储采用 blocks/ 文件落盘，Badger 仅保存索引：
	//   - indices:height:{height} -> {blockHash(32)+pathLen(1)+path+size(8)}
	//   - indices:hash:{hash} -> height(8)
	// - 因此回滚必须删除 indices:*，而不是旧的 block:data/block:hash 键。
	type delPlan struct {
		heightKeys [][]byte
		hashKeys   [][]byte
		txKeys     [][]byte
		tipValue   []byte
	}
	plan := &delPlan{
		heightKeys: make([][]byte, 0, currentHeight-height),
		hashKeys:   make([][]byte, 0, currentHeight-height),
		txKeys:     make([][]byte, 0, (currentHeight-height)*2),
	}

	// 3.1 收集 height+1..currentHeight 的区块索引与交易索引
	if s.txHashClient == nil {
		return fmt.Errorf("txHashClient 未注入，无法删除交易索引")
	}
	for h := height + 1; h <= currentHeight; h++ {
		// 读取高度索引，提取 blockHash（用于删除 indices:hash）
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		indexData, ierr := s.store.Get(ctx, heightKey)
		if ierr != nil {
			return fmt.Errorf("回滚时读取高度索引失败 height=%d: %w", h, ierr)
		}
		if len(indexData) < 32 {
			return fmt.Errorf("回滚时高度索引数据无效 height=%d len=%d", h, len(indexData))
		}
		blockHash := indexData[:32]
		hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))

		plan.heightKeys = append(plan.heightKeys, heightKey)
		plan.hashKeys = append(plan.hashKeys, hashKey)

		// 读取区块以提取交易（用于删除 indices:tx）
		blk, berr := s.queryService.GetBlockByHeight(ctx, h)
		if berr != nil {
			return fmt.Errorf("回滚时获取区块失败 height=%d: %w", h, berr)
		}
		if blk == nil || blk.Header == nil {
			return fmt.Errorf("回滚时区块不存在或区块头为空 height=%d", h)
		}
		if blk.Body != nil && len(blk.Body.Transactions) > 0 {
			for i, txProto := range blk.Body.Transactions {
				txResp, err := s.txHashClient.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: txProto})
				if err != nil {
					return fmt.Errorf("回滚时计算交易哈希失败 height=%d tx_index=%d: %w", h, i, err)
				}
				if txResp == nil || !txResp.IsValid || len(txResp.Hash) != 32 {
					return fmt.Errorf("回滚时交易哈希无效 height=%d tx_index=%d valid=%v hash_len=%d",
						h, i, txResp != nil && txResp.IsValid, func() int {
							if txResp == nil {
								return 0
							}
							return len(txResp.Hash)
						}(),
					)
				}
				plan.txKeys = append(plan.txKeys, []byte(fmt.Sprintf("indices:tx:%x", txResp.Hash)))
			}
		}
	}

	// 3.2 生成回滚后链尖 tipValue（使用高度索引中存储的 blockHash，避免与索引 hash 不一致）
	targetHeightKey := []byte(fmt.Sprintf("indices:height:%d", height))
	targetIndex, terr := s.store.Get(ctx, targetHeightKey)
	if terr != nil {
		return fmt.Errorf("回滚时读取目标高度索引失败 height=%d: %w", height, terr)
	}
	if len(targetIndex) < 32 {
		return fmt.Errorf("回滚时目标高度索引数据无效 height=%d len=%d", height, len(targetIndex))
	}
	targetHash := targetIndex[:32]
	tipValue := make([]byte, 40)
	tipValue[0] = byte(height >> 56)
	tipValue[1] = byte(height >> 48)
	tipValue[2] = byte(height >> 40)
	tipValue[3] = byte(height >> 32)
	tipValue[4] = byte(height >> 24)
	tipValue[5] = byte(height >> 16)
	tipValue[6] = byte(height >> 8)
	tipValue[7] = byte(height)
	copy(tipValue[8:], targetHash)
	plan.tipValue = tipValue

	// 4. 在事务中执行回滚删除与链尖更新
	err = s.store.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		for _, k := range plan.heightKeys {
			if err := tx.Delete(k); err != nil {
				if s.logger != nil {
					s.logger.Warnf("删除区块高度索引失败 key=%s: %v（继续）", string(k), err)
				}
			}
		}
		for _, k := range plan.hashKeys {
			if err := tx.Delete(k); err != nil {
				if s.logger != nil {
					s.logger.Warnf("删除区块哈希索引失败 key=%s: %v（继续）", string(k), err)
				}
			}
		}
		for _, k := range plan.txKeys {
			if err := tx.Delete(k); err != nil {
				return fmt.Errorf("回滚时删除交易索引失败 key=%s: %w", string(k), err)
			}
		}

		// 5. 删除资源索引（P0-2: 补充资源索引回滚清理）
		//
		// 说明：
		// - 普通 REORG (forkHeight > 0) 必须清理资源索引，确保与 UTXO 状态一致
		// - 资源索引包括：资源实例、资源代码、UTXO-资源映射、计数器、所有者索引、历史索引
		resourcePrefixes := []string{
			"indices:resource-instance:",
			"indices:resource-code:",
			"resource:utxo-instance:",
			"resource:counters-instance:",
			"index:resource:owner-instance:",
			"indices:utxo:history:",
		}

		for _, prefix := range resourcePrefixes {
			// 使用前缀删除，清理所有相关资源索引
			if err := s.deleteByPrefixInTx(tx, []byte(prefix)); err != nil {
				if s.logger != nil {
					s.logger.Warnf("删除资源索引失败 prefix=%s: %v（继续）", prefix, err)
				}
				// 资源索引清理失败不应阻断回滚，但需记录警告
			}
		}

		// 更新链尖：height(8 bytes) + hash(32 bytes)
		tipKey := []byte("state:chain:tip")
		if err := tx.Set(tipKey, plan.tipValue); err != nil {
			return fmt.Errorf("更新链尖失败: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("回滚事务执行失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ 链状态回滚完成: 新高度=%d", height)
	}

	return nil
}

// BuildIndexRollbackPlan 构建“索引回滚删除计划”（事务外预收集，事务内执行）。
//
// 设计约束（严格）：
// - 不允许在 Badger 事务内做 QueryService/PrefixScan（避免嵌套读与不可预期副作用）
// - 所有需要删除的键必须在事务外预收集为确定性列表
func (s *Service) BuildIndexRollbackPlan(ctx context.Context, targetHeight uint64) (*reorg.IndexRollbackPlan, error) {
	if s == nil || s.store == nil || s.queryService == nil || s.txHashClient == nil {
		return nil, fmt.Errorf("依赖未注入（store/queryService/txHashClient）")
	}
	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取链信息失败: %w", err)
	}
	currentHeight := chainInfo.Height
	if targetHeight > currentHeight {
		return nil, fmt.Errorf("targetHeight(%d) > currentHeight(%d)", targetHeight, currentHeight)
	}
	if targetHeight == currentHeight {
		return &reorg.IndexRollbackPlan{TargetHeight: targetHeight}, nil
	}

	plan := &reorg.IndexRollbackPlan{
		TargetHeight: targetHeight,
		HeightKeys:   make([][]byte, 0, currentHeight-targetHeight),
		HashKeys:     make([][]byte, 0, currentHeight-targetHeight),
		TxKeys:       make([][]byte, 0, (currentHeight-targetHeight)*2),
		ResourceKeys: make([][]byte, 0, 1024),
	}

	// 1) 收集 height+1..currentHeight 的 height/hash/tx 索引
	for h := targetHeight + 1; h <= currentHeight; h++ {
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		indexData, ierr := s.store.Get(ctx, heightKey)
		if ierr != nil {
			return nil, fmt.Errorf("读取高度索引失败 height=%d: %w", h, ierr)
		}
		if len(indexData) < 32 {
			return nil, fmt.Errorf("高度索引数据无效 height=%d len=%d", h, len(indexData))
		}
		blockHash := indexData[:32]
		hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
		plan.HeightKeys = append(plan.HeightKeys, heightKey)
		plan.HashKeys = append(plan.HashKeys, hashKey)

		blk, berr := s.queryService.GetBlockByHeight(ctx, h)
		if berr != nil {
			return nil, fmt.Errorf("获取区块失败 height=%d: %w", h, berr)
		}
		if blk == nil || blk.Header == nil {
			return nil, fmt.Errorf("区块缺失或头为空 height=%d", h)
		}
		if blk.Body != nil && len(blk.Body.Transactions) > 0 {
			for i, txProto := range blk.Body.Transactions {
				txResp, err := s.txHashClient.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: txProto})
				if err != nil {
					return nil, fmt.Errorf("计算交易哈希失败 height=%d tx_index=%d: %w", h, i, err)
				}
				if txResp == nil || !txResp.IsValid || len(txResp.Hash) != 32 {
					return nil, fmt.Errorf("交易哈希无效 height=%d tx_index=%d", h, i)
				}
				plan.TxKeys = append(plan.TxKeys, []byte(fmt.Sprintf("indices:tx:%x", txResp.Hash)))
			}
		}
	}

	// 2) 预收集资源/历史索引（严格：通过 PrefixScan 预收集为确定性 key 列表）
	resourcePrefixes := [][]byte{
		[]byte("indices:resource-instance:"),
		[]byte("indices:resource-code:"),
		[]byte("resource:utxo-instance:"),
		[]byte("resource:counters-instance:"),
		[]byte("index:resource:owner-instance:"),
		[]byte("indices:utxo:history:"),
	}
	for _, prefix := range resourcePrefixes {
		m, err := s.store.PrefixScan(ctx, prefix)
		if err != nil {
			return nil, fmt.Errorf("PrefixScan 失败 prefix=%s: %w", string(prefix), err)
		}
		for k := range m {
			plan.ResourceKeys = append(plan.ResourceKeys, []byte(k))
		}
	}

	// 3) 计算回滚后 tipValue（height(8)+hash(32)）
	targetHeightKey := []byte(fmt.Sprintf("indices:height:%d", targetHeight))
	targetIndex, terr := s.store.Get(ctx, targetHeightKey)
	if terr != nil {
		return nil, fmt.Errorf("读取目标高度索引失败 height=%d: %w", targetHeight, terr)
	}
	if len(targetIndex) < 32 {
		return nil, fmt.Errorf("目标高度索引数据无效 height=%d len=%d", targetHeight, len(targetIndex))
	}
	targetHash := targetIndex[:32]
	tipValue := make([]byte, 40)
	tipValue[0] = byte(targetHeight >> 56)
	tipValue[1] = byte(targetHeight >> 48)
	tipValue[2] = byte(targetHeight >> 40)
	tipValue[3] = byte(targetHeight >> 32)
	tipValue[4] = byte(targetHeight >> 24)
	tipValue[5] = byte(targetHeight >> 16)
	tipValue[6] = byte(targetHeight >> 8)
	tipValue[7] = byte(targetHeight)
	copy(tipValue[8:], targetHash)
	plan.TipValue = tipValue

	return plan, nil
}

// ApplyIndexRollbackPlanInTx 在 BadgerTransaction 内原子执行“索引回滚删除计划”。
func (s *Service) ApplyIndexRollbackPlanInTx(tx storage.BadgerTransaction, plan *reorg.IndexRollbackPlan) error {
	if tx == nil || plan == nil {
		return fmt.Errorf("tx/plan 不能为空")
	}
	for _, k := range plan.HeightKeys {
		_ = tx.Delete(k)
	}
	for _, k := range plan.HashKeys {
		_ = tx.Delete(k)
	}
	for _, k := range plan.TxKeys {
		if err := tx.Delete(k); err != nil {
			return fmt.Errorf("删除交易索引失败 key=%s: %w", string(k), err)
		}
	}
	for _, k := range plan.ResourceKeys {
		_ = tx.Delete(k)
	}
	if len(plan.TipValue) > 0 {
		if err := tx.Set([]byte("state:chain:tip"), plan.TipValue); err != nil {
			return fmt.Errorf("更新链尖失败: %w", err)
		}
	}
	return nil
}

// RollbackIndicesToHeight 使用“预收集计划 + 事务内执行”的方式回滚索引到目标高度。
// 注意：此方法只处理索引与 tip，不处理 UTXO（UTXO 回滚由 SnapshotManager 负责）。
func (s *Service) RollbackIndicesToHeight(ctx context.Context, targetHeight uint64) error {
	plan, err := s.BuildIndexRollbackPlan(ctx, targetHeight)
	if err != nil {
		return err
	}
	if plan == nil {
		return fmt.Errorf("index rollback plan 为空")
	}
	return s.store.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return s.ApplyIndexRollbackPlanInTx(tx, plan)
	})
}

// ============================================================================
//                              接口实现
// ============================================================================

// HandleFork 处理分叉区块
//
// 🎯 **ForkHandler 接口实现**
//
// 处理检测到的分叉区块，包括：
// 1. 验证分叉区块
// 2. 比较链权重
// 3. 决定是否切换链
// 4. 执行重组（如需要）
//
// 参数：
//   - ctx: 操作上下文
//   - forkBlock: 分叉区块
//
// 返回：
//   - error: 处理失败的错误
func (s *Service) HandleFork(ctx context.Context, forkBlock *core.Block) error {
	// 检查分叉区块是否为 nil
	if forkBlock == nil {
		return fmt.Errorf("分叉区块不能为空")
	}

	// 检查区块头是否为 nil
	if forkBlock.Header == nil {
		return fmt.Errorf("分叉区块头不能为空")
	}

	if s.logger != nil {
		s.logger.Infof("处理分叉区块: 高度=%d",
			forkBlock.Header.Height)
	}

	// 委托给专门的处理逻辑
	return s.handleFork(ctx, forkBlock)
}

// HandleForkWithExternalBlocks 用外部下载的分叉段执行自动 reorg（sync 场景）。
//
// 说明：
// - forkHeight 由 SyncHelloV2 的 locator 匹配得到（共同祖先高度）
// - forkTip/forkBlocksByHeight 由同步模块从对端下载得到
// - 本方法会复用 ForkHandler 的链权重决策与 reorg 执行逻辑
func (s *Service) HandleForkWithExternalBlocks(ctx context.Context, forkHeight uint64, forkTip *core.Block, forkBlocksByHeight map[uint64]*core.Block) error {
	if forkTip == nil || forkTip.Header == nil {
		return fmt.Errorf("forkTip 不能为空")
	}
	if s.isProcessing() {
		return fmt.Errorf("正在处理另一个分叉，请稍后重试")
	}

	// 记录处理状态：使用 forkTip 高度
	s.setProcessing(true, forkTip.Header.Height)
	defer s.setProcessing(false, 0)

	s.incrementMetric("total_forks")

	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}
	currentHeight := chainInfo.Height
	if currentHeight < forkHeight {
		return fmt.Errorf("本地主链高度(%d)小于 forkHeight(%d)", currentHeight, forkHeight)
	}
	if forkTip.Header.Height <= forkHeight {
		return fmt.Errorf("forkTip.Height(%d) 必须大于 forkHeight(%d)", forkTip.Header.Height, forkHeight)
	}

	// 分叉深度限制：按主链回滚深度计算
	forkDepth := uint32(currentHeight - forkHeight)
	// ✅ sync 自动 reorg 的深度限制必须与共识矿工的 max_fork_depth 解耦：
	// - consensus.miner.max_fork_depth：更偏向“在线共识/挖矿/广播”的安全门闸（默认较小）
	// - blockchain.sync.advanced.auto_reorg_max_depth：专用于“同步 + 自动重组”的上限（默认较大）
	//
	// 否则会出现：sync 允许重组，但 fork 模块又按矿工阈值拒绝，导致“检测到分叉但无法自愈”。
	maxForkDepth := uint32(s.getMaxExternalForkDepth())
	if forkDepth > maxForkDepth {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 外部分叉深度 %d 超过阈值 %d（blockchain.sync.advanced.auto_reorg_max_depth），拒绝处理",
				forkDepth, maxForkDepth)
		}
		return fmt.Errorf("分叉深度过大: %d > %d（受 blockchain.sync.advanced.auto_reorg_max_depth 限制）", forkDepth, maxForkDepth)
	}

	// provider：优先使用外部 blocks
	provider := func(height uint64) (*core.Block, bool) {
		if height == forkTip.Header.Height {
			return forkTip, true
		}
		if forkBlocksByHeight == nil {
			return nil, false
		}
		blk, ok := forkBlocksByHeight[height]
		return blk, ok
	}

	// 主链权重（从共同祖先到主链 tip）
	mainChainWeight, err := s.calculateChainWeightWithProvider(ctx, forkHeight, currentHeight, nil)
	if err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		return fmt.Errorf("计算主链权重失败: %w", err)
	}

	// 分叉链权重（从共同祖先到 forkTip）
	forkChainWeight, err := s.calculateChainWeightWithProvider(ctx, forkHeight, forkTip.Header.Height, provider)
	if err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		return fmt.Errorf("计算分叉链权重失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("链权重比较(外部分叉): 主链=%s, 分叉链=%s", mainChainWeight.String(), forkChainWeight.String())
	}

	shouldSwitch := s.shouldSwitchChain(mainChainWeight, forkChainWeight)
	if !shouldSwitch {
		if s.logger != nil {
			s.logger.Info("✅ 主链权重更大，保持主链不变（外部分叉）")
		}
		s.incrementMetric("resolved_forks")
		return nil
	}

	if s.logger != nil {
		s.logger.Warnf("⚠️ 外部分叉链权重更大，准备切换主链: fork_height=%d new_tip=%d", forkHeight, forkTip.Header.Height)
	}

	if err := s.switchChainWithProvider(ctx, forkTip, forkHeight, provider); err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		return fmt.Errorf("链切换失败(外部分叉): %w", err)
	}

	s.incrementMetric("resolved_forks")
	s.incrementMetric("total_reorgs")
	s.updateReorgDepth(forkDepth)

	return nil
}

// GetActiveChain 获取活跃链信息
//
// 🎯 **ForkHandler 接口实现**
//
// 返回当前活跃的主链信息
//
// 返回：
//   - *types.ChainInfo: 链信息
//   - error: 查询失败的错误
func (s *Service) GetActiveChain(ctx context.Context) (*types.ChainInfo, error) {
	// 通过 QueryService 查询
	return s.queryService.GetChainInfo(ctx)
}

// DetectFork 检测分叉
//
// 🎯 **InternalForkHandler 接口实现**
//
// 检测给定区块是否造成分叉
//
// 返回：
//   - isFork: 是否是分叉
//   - forkHeight: 分叉点高度
//   - error: 检测错误
func (s *Service) DetectFork(ctx context.Context, block *core.Block) (bool, uint64, error) {
	// 委托给检测逻辑
	return s.detectFork(ctx, block)
}

// GetForkMetrics 获取分叉指标
//
// 🎯 **InternalForkHandler 接口实现**
//
// 返回分叉处理的统计指标
//
// 返回：
//   - *interfaces.ForkMetrics: 分叉指标
//   - error: 获取失败的错误（通常不会失败）
func (s *Service) GetForkMetrics(ctx context.Context) (*interfaces.ForkMetrics, error) {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	// 返回指标副本
	metricsCopy := *s.metrics
	metricsCopy.IsProcessing = s.isProcessingFork
	metricsCopy.CurrentForkHeight = s.currentForkHeight

	return &metricsCopy, nil
}

// CalculateChainWeight 计算链权重
//
// 🎯 **InternalForkHandler 接口实现**
//
// 计算指定高度范围内的链权重
//
// 参数：
//   - fromHeight: 起始高度
//   - toHeight: 结束高度
//
// 返回：
//   - *types.ChainWeight: 链权重
//   - error: 计算错误
func (s *Service) CalculateChainWeight(ctx context.Context, fromHeight, toHeight uint64) (*types.ChainWeight, error) {
	// 委托给权重计算逻辑
	return s.calculateChainWeight(ctx, fromHeight, toHeight)
}

func (s *Service) restoreSnapshotWithHeightCheck(ctx context.Context, snapshot *types.UTXOSnapshotData) error {
	if snapshot == nil {
		return fmt.Errorf("快照数据不能为空")
	}

	// ✅ 不再提供“跳过检查直接恢复”的向后兼容分支
	//   分叉处理属于高危操作，如果依赖未正确注入，应当立即失败而不是“尽力而为”
	if s.queryService == nil {
		return fmt.Errorf("链查询服务未注入，无法在恢复快照前进行高度一致性检查")
	}

	if s.utxoSnapshot == nil {
		return fmt.Errorf("UTXOSnapshot 未注入，无法恢复快照")
	}

	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("恢复快照前获取链信息失败: %w", err)
	}

	currentHeight := chainInfo.Height
	if currentHeight < snapshot.Height {
		if s.logger != nil {
			s.logger.Errorf("❌ 恢复快照被拒绝：当前链高度=%d 小于快照高度=%d。"+
				"这通常表示试图在错误的链状态下应用快照，请检查调用路径和快照来源。",
				currentHeight, snapshot.Height)
		}
		return fmt.Errorf("当前链高度(%d)小于快照高度(%d)，拒绝恢复", currentHeight, snapshot.Height)
	}

	if s.logger != nil {
		s.logger.Infof("🔁 正在恢复快照: snapshot_height=%d, current_chain_height=%d, snapshot_id=%s",
			snapshot.Height, currentHeight, snapshot.SnapshotID)
	}

	return s.utxoSnapshot.RestoreSnapshotAtomic(ctx, snapshot)
}

// getMaxForkDepth 获取最大允许分叉深度（优先从共识配置读取）
//
// 配置来源：
//   - consensus.miner.max_fork_depth
//   - 默认值：100（见 internal/config/consensus/defaults.go）
func (s *Service) getMaxForkDepth() uint64 {
	const defaultMaxForkDepth uint64 = 100

	if s.configProvider == nil {
		return defaultMaxForkDepth
	}

	consensusCfg := s.configProvider.GetConsensus()
	if consensusCfg == nil {
		return defaultMaxForkDepth
	}

	if consensusCfg.Miner.MaxForkDepth == 0 {
		return defaultMaxForkDepth
	}

	return consensusCfg.Miner.MaxForkDepth
}

// getMaxExternalForkDepth 获取“同步/外部分叉段自动 reorg”允许的最大回滚深度。
//
// 设计原则：
// - sync 自动 reorg 的窗口应当以 sync 配置为准，而不是复用 miner 的门闸参数
// - 允许较深的重组（甚至从 genesis）以解决长期分区/历史分叉，但仍需一个可配置上限防 DoS
//
// 配置来源（优先级从高到低）：
//   - blockchain.sync.advanced.auto_reorg_max_depth（默认 1000）
//   - （兜底）默认 1000
func (s *Service) getMaxExternalForkDepth() uint64 {
	const defaultAutoReorgMaxDepth uint64 = 1000

	if s == nil || s.configProvider == nil {
		return defaultAutoReorgMaxDepth
	}
	bc := s.configProvider.GetBlockchain()
	if bc == nil {
		return defaultAutoReorgMaxDepth
	}
	if bc.Sync.Advanced.AutoReorgMaxDepth > 0 {
		return uint64(bc.Sync.Advanced.AutoReorgMaxDepth)
	}
	return defaultAutoReorgMaxDepth
}

// getMaxForkBacktrack 获取查找分叉点时允许的最大回溯层数
//
// 默认策略：
//   - 复用共识配置中的 MaxForkDepth 作为回溯上限
//   - 避免出现“分叉深度阈值”和“回溯阈值”不一致导致的行为差异
func (s *Service) getMaxForkBacktrack() int {
	// ✅ 解耦：fork 检测“回溯寻找共同祖先”的上限，不应复用 miner 的 max_fork_depth。
	// 原因：
	// - miner.max_fork_depth 偏向“在线共识/挖矿门闸”的保护参数（默认较小）
	// - fork 检测需要在“深历史分叉/长时间网络分区”情况下仍能定位共同祖先，默认应更大
	//
	// 这里复用 sync 的 auto_reorg_max_depth 作为回溯上限（默认 1000），保证语义一致：
	// - sync 自动 reorg 允许多深，就至少应该能回溯定位到共同祖先
	maxDepth := s.getMaxExternalForkDepth()
	// 防止溢出，回退到安全默认值
	if maxDepth == 0 || maxDepth > 1_000_000 {
		return 1000
	}
	return int(maxDepth)
}

// ============================================================================
//                              编译时检查
// ============================================================================

// 确保 Service 实现了 InternalForkHandler 接口
var _ interfaces.InternalForkHandler = (*Service)(nil)

// ============================================================================
//                              辅助方法
// ============================================================================

// isProcessing 检查是否正在处理分叉
func (s *Service) isProcessing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isProcessingFork
}

// setProcessing 设置处理状态
func (s *Service) setProcessing(processing bool, forkHeight uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isProcessingFork = processing
	s.currentForkHeight = forkHeight
}

// incrementMetric 增加指标计数
func (s *Service) incrementMetric(metricName string) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	switch metricName {
	case "total_forks":
		s.metrics.TotalForks++
	case "resolved_forks":
		s.metrics.ResolvedForks++
	case "pending_forks":
		s.metrics.PendingForks++
	case "total_reorgs":
		s.metrics.TotalReorgs++
	}
}

// updateReorgDepth 更新重组深度统计
func (s *Service) updateReorgDepth(depth uint32) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	if depth > s.metrics.MaxReorgDepth {
		s.metrics.MaxReorgDepth = depth
	}

	// 更新平均重组深度（简单平均）
	if s.metrics.TotalReorgs > 0 {
		totalDepth := s.metrics.AvgReorgDepth * float64(s.metrics.TotalReorgs-1)
		s.metrics.AvgReorgDepth = (totalDepth + float64(depth)) / float64(s.metrics.TotalReorgs)
	} else {
		s.metrics.AvgReorgDepth = float64(depth)
	}
}

// deleteByPrefixInTx 在事务中批量删除指定前缀的键
//
// 说明：
// - 与 deleteByPrefix 不同，此方法在已有事务中执行删除
// - 用于 RollbackToHeight 等需要原子性操作的场景
//
// 参数：
//   - tx: Badger 事务对象
//   - prefix: 键前缀
//
// 返回：
//   - error: 删除失败的错误
func (s *Service) deleteByPrefixInTx(tx storage.BadgerTransaction, prefix []byte) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("badger store 未注入")
	}
	if tx == nil {
		return fmt.Errorf("transaction 不能为空")
	}

	// 使用 PrefixScan 查找所有匹配的键
	// 注意：PrefixScan 需要 context，但在事务中我们使用空 context
	ctx := context.Background()
	m, err := s.store.PrefixScan(ctx, prefix)
	if err != nil {
		return fmt.Errorf("前缀扫描失败: %w", err)
	}

	if len(m) == 0 {
		// 没有匹配的键，直接返回
		return nil
	}

	// 在事务中逐个删除
	deletedCount := 0
	for k := range m {
		if err := tx.Delete([]byte(k)); err != nil {
			return fmt.Errorf("删除键失败 key=%s: %w", k, err)
		}
		deletedCount++
	}

	if s.logger != nil {
		s.logger.Debugf("事务中删除前缀键: prefix=%s, count=%d", string(prefix), deletedCount)
	}

	return nil
}
