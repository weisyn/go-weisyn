// Package validator 实现区块验证服务
//
// 🎯 **BlockValidator 服务实现**
//
// 本包实现了区块验证服务，负责验证区块的有效性。
// 采用多层验证策略：结构 → 共识 → 交易。
//
// 💡 **核心职责**：
// - 验证区块结构
// - 验证共识规则
// - 验证交易有效性
// - 提供验证性能指标
package validator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/block/interfaces"
	"github.com/weisyn/v1/internal/core/block/merkle"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	pkgtypes "github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Service 区块验证服务
//
// 🎯 **设计理念**：
// - 多层验证：结构 → 共识 → 交易
// - 快速失败：第一个错误时立即返回
// - 无状态：不修改任何状态，只读验证
//
// 📦 **服务职责**：
// - ValidateBlock: 验证区块有效性
// - ValidateStructure: 验证区块结构（内部方法）
// - ValidateConsensus: 验证共识规则（内部方法）
// - GetValidatorMetrics: 获取验证性能指标
type Service struct {
	// ==================== 依赖注入 ====================

	// queryService 查询服务（读取链状态）
	queryService persistence.QueryService

	// hasher 哈希服务（用于Merkle树计算和其他哈希操作）
	hasher crypto.HashManager

	// blockHashClient 区块哈希服务客户端（用于计算区块哈希）
	blockHashClient core.BlockHashServiceClient

	// txHashClient 交易哈希服务客户端（用于计算交易哈希）
	txHashClient transaction.TransactionHashServiceClient

	// txVerifier 交易验证器（用于验证交易有效性，P3-7）
	txVerifier tx.TxVerifier

	// configProvider 配置提供者（必需，用于 v2 共识强校验：难度/时间戳）
	configProvider config.Provider

	// logger 日志记录器（可选）
	logger log.Logger

	// eventBus 事件总线（可选，用于发布 corruption.detected 事件）
	eventBus eventiface.EventBus

	// ==================== 指标收集 ====================

	// metrics 验证服务指标
	metrics *interfaces.ValidatorMetrics

	// metricsMu 指标读写锁
	metricsMu sync.Mutex

	// ==================== 状态管理 ====================

	// isHealthy 健康状态
	isHealthy bool

	// lastError 最后错误
	lastError error
}

// NewService 创建区块验证服务
//
// 🔧 **初始化流程**：
// 1. 验证必需依赖
// 2. 初始化指标
// 3. 设置默认配置
//
// 参数：
//   - queryService: 查询服务（必需）
//   - hasher: 哈希服务（必需，用于Merkle树计算）
//   - blockHashClient: 区块哈希服务客户端（必需）
//   - txHashClient: 交易哈希服务客户端（必需）
//   - txVerifier: 交易验证器（可选，用于验证交易有效性）
//   - logger: 日志记录器（可选）
//
// 返回：
//   - interfaces.InternalBlockValidator: 区块验证服务实例
//   - error: 创建错误
func NewService(
	queryService persistence.QueryService,
	hasher crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	txHashClient transaction.TransactionHashServiceClient,
	txVerifier tx.TxVerifier,
	configProvider config.Provider,
	eventBus eventiface.EventBus,
	logger log.Logger,
) (interfaces.InternalBlockValidator, error) {
	// 验证必需依赖
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
	if configProvider == nil {
		return nil, fmt.Errorf("configProvider 不能为空")
	}

	// 创建服务实例
	s := &Service{
		queryService:   queryService,
		hasher:         hasher,
		blockHashClient: blockHashClient,
		txHashClient:   txHashClient,
		txVerifier:     txVerifier,
		configProvider: configProvider,
		eventBus:       eventBus,
		logger:         logger,
		metrics:        &interfaces.ValidatorMetrics{},
		isHealthy:      true,
	}

	if logger != nil {
		logger.Info("✅ BlockValidator 服务初始化成功")
	}

	return s, nil
}

func (s *Service) publishCorruptionDetected(phase pkgtypes.CorruptionPhase, severity pkgtypes.CorruptionSeverity, height *uint64, hashHex string, key string, err error) {
	if s.eventBus == nil || err == nil {
		return
	}
	data := pkgtypes.CorruptionEventData{
		Component: pkgtypes.CorruptionComponentValidator,
		Phase:     phase,
		Severity:  severity,
		Height:    height,
		Hash:      hashHex,
		Key:       key,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        pkgtypes.RFC3339Time(time.Now()),
	}
	s.eventBus.Publish(eventiface.EventTypeCorruptionDetected, context.Background(), data)
}

// ValidateBlock 验证区块有效性
//
// 🎯 **多层验证流程**：
// 1. 基础验证（nil检查、空区块检查）
// 2. 结构验证（ValidateStructure）
// 3. 共识验证（ValidateConsensus）
// 4. 交易验证
// 5. 链连接性验证
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - bool: 验证结果（true=有效，false=无效）
//   - error: 验证错误（nil表示有效）
func (s *Service) ValidateBlock(ctx context.Context, block *core.Block) (bool, error) {
	startTime := time.Now()
	defer func() {
		s.recordValidation(time.Since(startTime))
	}()

	if block != nil && block.Header != nil && s.logger != nil {
		s.logger.Debugf("开始验证区块，高度: %d",
			block.Header.Height)
	}

	// 1. 基础验证
	if block == nil || block.Header == nil || block.Body == nil {
		return false, s.recordValidationError("structure", fmt.Errorf("区块或区块头/区块体为空"))
	}

	// 2. 结构验证
	if err := s.ValidateStructure(ctx, block); err != nil {
		return false, s.recordValidationError("structure", err)
	}

	// 3. 共识验证
	if err := s.ValidateConsensus(ctx, block); err != nil {
		return false, s.recordValidationError("consensus", err)
	}

	// 4. 交易验证（P3-7：完整的交易验证逻辑）
	if err := s.validateTransactions(ctx, block); err != nil {
		return false, s.recordValidationError("transaction", err)
	}

	// 5. 链连接性验证（P3-8：验证父区块存在性和高度连续性）
	if err := s.validateChainConnectivity(ctx, block); err != nil {
		return false, s.recordValidationError("chain", err)
	}

	// 验证通过
	s.recordValidationSuccess()

	if s.logger != nil {
		s.logger.Infof("✅ 区块验证通过，高度: %d",
			block.Header.Height)
	}

	return true, nil
}

// ValidateStructure 验证区块结构（内部方法）
//
// 🎯 **结构验证**：
// - 区块头格式
// - 区块体格式
// - 字段完整性
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) ValidateStructure(ctx context.Context, block *core.Block) error {
	// 详细实现在 structure.go
	return s.validateStructure(ctx, block)
}

// ValidateConsensus 验证共识规则（内部方法）
//
// 🎯 **共识验证**：
// - POW验证
// - 难度验证
// - 时间戳验证
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) ValidateConsensus(ctx context.Context, block *core.Block) error {
	// 详细实现在 consensus.go
	return s.validateConsensus(ctx, block)
}

// ==================== 内部管理方法 ====================

// GetValidatorMetrics 获取验证服务指标
func (s *Service) GetValidatorMetrics(ctx context.Context) (*interfaces.ValidatorMetrics, error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	// 更新健康状态
	s.metrics.IsHealthy = s.isHealthy
	if s.lastError != nil {
		s.metrics.ErrorMessage = s.lastError.Error()
	}

	return s.metrics, nil
}

// ==================== 辅助方法 ====================

// recordValidation 记录验证指标
func (s *Service) recordValidation(duration time.Duration) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.BlocksValidated++
	s.metrics.LastValidateTime = time.Now().Unix()

	// 更新平均验证耗时（滑动平均）
	alpha := 0.1
	newTime := duration.Seconds()
	if s.metrics.AvgValidateTime == 0 {
		s.metrics.AvgValidateTime = newTime
	} else {
		s.metrics.AvgValidateTime = alpha*newTime + (1-alpha)*s.metrics.AvgValidateTime
	}

	// 更新最大验证耗时
	if newTime > s.metrics.MaxValidateTime {
		s.metrics.MaxValidateTime = newTime
	}
}

// recordValidationSuccess 记录验证成功
func (s *Service) recordValidationSuccess() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.ValidationsPassed++
	s.isHealthy = true
}

// recordValidationError 记录验证错误
func (s *Service) recordValidationError(errorType string, err error) error {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.ValidationsFailed++
	s.lastError = err

	switch errorType {
	case "structure":
		s.metrics.StructureErrors++
	case "consensus":
		s.metrics.ConsensusErrors++
	case "transaction":
		s.metrics.TransactionErrors++
	case "chain":
		s.metrics.ChainErrors++
	}

	return err
}

// validateChainConnectivity 验证链连接性（P3-8：父区块验证）
//
// 🎯 **链连接性验证**：
// 1. 验证父区块存在性（通过 PreviousHash 查询）
// 2. 验证高度连续性（父区块高度 = 当前高度 - 1）
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) validateChainConnectivity(ctx context.Context, block *core.Block) error {
	// 创世区块跳过父区块验证
	if block.Header.Height == 0 {
		if s.logger != nil {
			s.logger.Debug("创世区块，跳过父区块验证")
		}
		return nil
	}

	// 1. 验证父区块哈希非空
	if len(block.Header.PreviousHash) == 0 {
		return fmt.Errorf("父区块哈希为空（高度=%d）", block.Header.Height)
	}

	// 2. 获取父区块
	parentBlock, err := s.queryService.GetBlockByHash(ctx, block.Header.PreviousHash)
	if err != nil {
		// 生产自运行：给出“验证阶段”的腐化上下文（父块缺失/索引损坏/读取失败等）
		parentHeight := block.Header.Height - 1
		hashHex := fmt.Sprintf("%x", block.Header.PreviousHash)
		s.publishCorruptionDetected(pkgtypes.CorruptionPhaseValidate, pkgtypes.CorruptionSeverityCritical, &parentHeight, hashHex, "", err)

		hashPrefix := block.Header.PreviousHash
		if len(hashPrefix) > 8 {
			hashPrefix = hashPrefix[:8]
		}
		return fmt.Errorf("父区块不存在（高度=%d，父哈希=%x）: %w",
			block.Header.Height, hashPrefix, err)
	}

	if parentBlock == nil || parentBlock.Header == nil {
		return fmt.Errorf("父区块数据无效（高度=%d）", block.Header.Height)
	}

	// 3. 验证高度连续性
	expectedParentHeight := block.Header.Height - 1
	if parentBlock.Header.Height != expectedParentHeight {
		return fmt.Errorf("高度不连续: 当前高度=%d，父区块高度=%d，期望=%d",
			block.Header.Height, parentBlock.Header.Height, expectedParentHeight)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 链连接性验证通过: 高度=%d，父高度=%d", block.Header.Height, parentBlock.Header.Height)
	}

	return nil
}

// validateTransactions 验证区块中的交易（P3-7：完整的交易验证逻辑）
//
// 🎯 **交易验证检查项**：
// 1. 交易列表非空检查
// 2. Coinbase 交易位置检查（必须在首位）
// 3. 交易哈希重复检查（确保区块中无重复交易）
// 4. 每笔交易的有效性验证（使用 TxVerifier，Coinbase 交易跳过）
// 5. Merkle 根验证（确保交易列表的 Merkle 根与区块头中的一致）
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) validateTransactions(ctx context.Context, block *core.Block) error {
	transactions := block.Body.Transactions

	// 1. 交易列表非空检查
	if len(transactions) == 0 {
		return fmt.Errorf("区块交易列表为空")
	}

	// 2. Coinbase 交易位置检查（必须在首位）
	if len(transactions[0].Inputs) != 0 {
		return fmt.Errorf("首个交易应该是Coinbase交易（没有输入）")
	}

	// 3. 交易哈希重复检查（确保区块中无重复交易）
	txHashes := make(map[string]int)
	for i, tx := range transactions {
		// 使用 gRPC 服务计算交易哈希
		req := &transaction.ComputeHashRequest{
			Transaction: tx,
		}
		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return fmt.Errorf("计算交易%d哈希失败: %w", i, err)
		}

		if !resp.IsValid {
			return fmt.Errorf("交易%d结构无效", i)
		}

		txHash := resp.Hash
		// 检查重复
		txHashStr := string(txHash)
		if dupIndex, exists := txHashes[txHashStr]; exists {
			return fmt.Errorf("交易重复: 交易%d与交易%d具有相同的哈希 %x", i, dupIndex, txHash[:min(8, len(txHash))])
		}
		txHashes[txHashStr] = i
	}

	// 4. 每笔交易的有效性验证（使用 TxVerifier，Coinbase 交易跳过）
	if s.txVerifier != nil {
		for i, tx := range transactions {
			// Coinbase 交易跳过验证（Coinbase 交易没有输入，不需要验证 UTXO）
			if i == 0 && len(tx.Inputs) == 0 {
				if s.logger != nil {
					s.logger.Debug("Coinbase 交易跳过验证")
				}
				continue
			}

			// 验证交易
			if err := s.txVerifier.Verify(ctx, tx); err != nil {
				txHashPrefix := ""
				// 使用 gRPC 服务计算交易哈希（用于错误信息）
				req := &transaction.ComputeHashRequest{
					Transaction: tx,
				}
				if resp, hashErr := s.txHashClient.ComputeHash(ctx, req); hashErr == nil && resp.IsValid {
					if len(resp.Hash) >= 8 {
						txHashPrefix = fmt.Sprintf("（哈希=%x", resp.Hash[:8])
					}
				}
				return fmt.Errorf("交易%d验证失败%s）: %w", i, txHashPrefix, err)
			}
		}

		if s.logger != nil {
			s.logger.Debugf("✅ 交易验证通过: 共%d笔交易", len(transactions))
		}
	} else {
		if s.logger != nil {
			s.logger.Debug("⚠️ TxVerifier 未注入，跳过交易有效性验证")
		}
	}

	// 5. Merkle 根验证（使用统一的交易哈希服务，与 BlockBuilder/PoWHandler 保持一致）
	calculatedMerkleRoot, err := s.calculateMerkleRootFromTransactions(ctx, transactions)
	if err != nil {
		return fmt.Errorf("计算Merkle根失败: %w", err)
	}

	// 比较计算出的 Merkle 根与区块头中的 Merkle 根
	if len(calculatedMerkleRoot) != len(block.Header.MerkleRoot) {
		return fmt.Errorf("Merkle根长度不一致: 计算=%d，区块头=%d",
			len(calculatedMerkleRoot), len(block.Header.MerkleRoot))
	}

	for i := range calculatedMerkleRoot {
		if calculatedMerkleRoot[i] != block.Header.MerkleRoot[i] {
			return fmt.Errorf("Merkle根不匹配: 计算=%x，区块头=%x",
				calculatedMerkleRoot[:min(8, len(calculatedMerkleRoot))],
				block.Header.MerkleRoot[:min(8, len(block.Header.MerkleRoot))])
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ Merkle根验证通过: %x", calculatedMerkleRoot[:min(8, len(calculatedMerkleRoot))])
	}

	return nil
}

// calculateMerkleRootFromTransactions 从交易列表计算 Merkle 根
// 🔧 使用统一的交易哈希服务，与 BlockBuilder/PoWHandler 保持一致
// ⚠️ 使用调用方传入的 ctx，以便在高负载或大区块场景下能响应超时/取消。
func (s *Service) calculateMerkleRootFromTransactions(ctx context.Context, transactions []*transaction.Transaction) ([]byte, error) {
	if len(transactions) == 0 {
		// 空交易列表返回全零Merkle根
		return make([]byte, 32), nil
	}

	// 使用统一的交易哈希服务计算交易哈希
	transactionHashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		req := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}

		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("计算交易[%d]哈希失败: %w", i, err)
		}

		if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
			return nil, fmt.Errorf("交易[%d]哈希无效", i)
		}

		transactionHashes[i] = resp.Hash
	}

	// 从交易哈希构建 Merkle 树
	return s.buildMerkleTreeFromHashes(transactionHashes)
}

// buildMerkleTreeFromHashes 从交易哈希列表构建Merkle树
// 🔧 与 BlockBuilder/PoWHandler 保持完全一致的算法
func (s *Service) buildMerkleTreeFromHashes(hashes [][]byte) ([]byte, error) {
	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 基础情况：2个节点配对后返回
	if len(hashes) == 2 {
		combined := append(hashes[0], hashes[1]...)
		hasherAdapter := merkle.NewHashManagerAdapter(s.hasher)
		parentHash, err := hasherAdapter.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}
		return parentHash, nil
	}

	// 计算下一层节点
	nextLevel := make([][]byte, 0, len(hashes)/2)
	hasherAdapter := merkle.NewHashManagerAdapter(s.hasher)
	for i := 0; i < len(hashes); i += 2 {
		// 连接两个子节点的哈希
		combined := append(hashes[i], hashes[i+1]...)

		// 计算父节点哈希
		parentHash, err := hasherAdapter.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return s.buildMerkleTreeFromHashes(nextLevel)
}

// 编译时检查接口实现
var _ interfaces.InternalBlockValidator = (*Service)(nil)
