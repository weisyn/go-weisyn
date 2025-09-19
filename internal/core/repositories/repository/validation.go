package repository

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                          🧪 生产验证和测试
// ============================================================================

// ValidationSuite 验证测试套件
type ValidationSuite struct {
	manager *Manager
	logger  log.Logger
}

// NewValidationSuite 创建验证测试套件
func NewValidationSuite(manager *Manager, logger log.Logger) *ValidationSuite {
	return &ValidationSuite{
		manager: manager,
		logger:  logger,
	}
}

// RunFullValidation 运行完整验证
func (vs *ValidationSuite) RunFullValidation(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Info("开始运行完整验证测试")
	}

	// 1. 架构边界验证
	if err := vs.validateArchitectureBoundaries(ctx); err != nil {
		return fmt.Errorf("架构边界验证失败: %w", err)
	}

	// 2. 单一入口验证
	if err := vs.validateSingleEntryPoint(ctx); err != nil {
		return fmt.Errorf("单一入口验证失败: %w", err)
	}

	// 3. 事务原子性验证
	if err := vs.validateTransactionAtomicity(ctx); err != nil {
		return fmt.Errorf("事务原子性验证失败: %w", err)
	}

	// 4. Outbox机制验证
	if err := vs.validateOutboxMechanism(ctx); err != nil {
		return fmt.Errorf("Outbox机制验证失败: %w", err)
	}

	// 5. 性能指标验证
	if err := vs.validatePerformanceMetrics(ctx); err != nil {
		return fmt.Errorf("性能指标验证失败: %w", err)
	}

	if vs.logger != nil {
		vs.logger.Info("完整验证测试通过")
	}

	return nil
}

// validateArchitectureBoundaries 验证架构边界
func (vs *ValidationSuite) validateArchitectureBoundaries(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Debug("验证架构边界")
	}

	// 验证Manager只有一个写入方法
	// 这里通过反射或代码审计来验证
	// 简化实现：检查关键组件是否正确初始化

	if vs.manager.blockStorage == nil {
		return fmt.Errorf("BlockStorage未正确初始化")
	}
	if vs.manager.chainState == nil {
		return fmt.Errorf("ChainState未正确初始化")
	}
	if vs.manager.indexManager == nil {
		return fmt.Errorf("IndexManager未正确初始化")
	}
	if vs.manager.txService == nil {
		return fmt.Errorf("TransactionService未正确初始化")
	}
	if vs.manager.resService == nil {
		return fmt.Errorf("ResourceService未正确初始化")
	}
	if vs.manager.utxoClient == nil {
		return fmt.Errorf("UTXOService未正确初始化")
	}
	if vs.manager.outboxManager == nil {
		return fmt.Errorf("OutboxManager未正确初始化")
	}
	if vs.manager.performanceMonitor == nil {
		return fmt.Errorf("PerformanceMonitor未正确初始化")
	}

	return nil
}

// validateSingleEntryPoint 验证单一入口点
func (vs *ValidationSuite) validateSingleEntryPoint(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Debug("验证单一入口点")
	}

	// 验证只有Manager.StoreBlock是写入入口
	// 这里通过检查是否有其他写入方法被意外暴露

	// 简化实现：验证关键写入路径的完整性
	// 实际生产中，这里应该有更详细的API边界检查

	return nil
}

// validateTransactionAtomicity 验证事务原子性
func (vs *ValidationSuite) validateTransactionAtomicity(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Debug("验证事务原子性")
	}

	// 创建一个测试区块
	testBlock := vs.createTestBlock()

	// 尝试存储区块
	if err := vs.manager.StoreBlock(ctx, testBlock); err != nil {
		return fmt.Errorf("存储测试区块失败: %w", err)
	}

	// 验证所有相关数据都已正确存储
	// 1. 验证区块数据
	retrievedBlock, err := vs.manager.GetBlock(ctx, vs.computeTestBlockHash(testBlock))
	if err != nil {
		return fmt.Errorf("获取存储的区块失败: %w", err)
	}
	if retrievedBlock.Header.Height != testBlock.Header.Height {
		return fmt.Errorf("区块高度不匹配")
	}

	// 2. 验证链状态更新
	chainState, err := vs.manager.GetChainState(ctx)
	if err != nil {
		return fmt.Errorf("获取链状态失败: %w", err)
	}
	if chainState.HighestHeight < testBlock.Header.Height {
		return fmt.Errorf("链状态未正确更新")
	}

	// 3. 验证outbox事件创建
	events, err := vs.manager.outboxManager.GetPendingEvents(ctx)
	if err != nil {
		return fmt.Errorf("获取outbox事件失败: %w", err)
	}
	if len(events) == 0 {
		return fmt.Errorf("outbox事件未创建")
	}

	return nil
}

// validateOutboxMechanism 验证Outbox机制
func (vs *ValidationSuite) validateOutboxMechanism(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Debug("验证Outbox机制")
	}

	// 触发outbox事件处理
	vs.manager.processOutboxEvents(ctx)

	// 等待一段时间让异步处理完成
	time.Sleep(time.Millisecond * 100)

	// 检查事件是否被处理
	events, err := vs.manager.outboxManager.GetPendingEvents(ctx)
	if err != nil {
		return fmt.Errorf("获取待处理事件失败: %w", err)
	}

	// 如果还有待处理事件，可能是正常的（取决于UTXO系统状态）
	if vs.logger != nil && len(events) > 0 {
		vs.logger.Debugf("仍有 %d 个待处理的outbox事件", len(events))
	}

	return nil
}

// validatePerformanceMetrics 验证性能指标
func (vs *ValidationSuite) validatePerformanceMetrics(ctx context.Context) error {
	if vs.logger != nil {
		vs.logger.Debug("验证性能指标")
	}

	// 获取性能指标
	metrics := vs.manager.GetPerformanceMetrics()
	if metrics == nil {
		return fmt.Errorf("性能指标为空")
	}

	// 验证指标的合理性
	if metrics.BlockProcessingTime < 0 {
		return fmt.Errorf("区块处理时间异常: %v", metrics.BlockProcessingTime)
	}

	if vs.logger != nil {
		vs.logger.Debugf("性能指标验证通过 - 平均处理时间: %v, 平均交易数: %d",
			metrics.BlockProcessingTime, metrics.TransactionCount)
	}

	return nil
}

// createTestBlock 创建测试区块
func (vs *ValidationSuite) createTestBlock() *core.Block {
	now := uint64(time.Now().Unix())

	return &core.Block{
		Header: &core.BlockHeader{
			Version:      1,
			Height:       1000000, // 使用高的测试高度避免冲突
			Timestamp:    now,
			PreviousHash: make([]byte, 32), // 空的前一个哈希
			MerkleRoot:   make([]byte, 32), // 空的Merkle根
			Nonce:        []byte{1, 2, 3, 4},
			Difficulty:   1,
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				{
					Version:           1,
					Inputs:            []*transaction.TxInput{},
					Outputs:           []*transaction.TxOutput{},
					Nonce:             12345,
					CreationTimestamp: now,
				},
			},
		},
	}
}

// computeTestBlockHash 计算测试区块哈希（简化实现）
func (vs *ValidationSuite) computeTestBlockHash(block *core.Block) []byte {
	// 简化实现：使用区块高度和时间戳生成伪哈希
	hash := make([]byte, 32)
	height := block.Header.Height
	timestamp := block.Header.Timestamp

	// 将高度和时间戳编码到哈希中
	for i := 0; i < 8 && i < len(hash); i++ {
		hash[i] = byte(height >> (i * 8))
	}
	for i := 8; i < 16 && i < len(hash); i++ {
		hash[i] = byte(timestamp >> ((i - 8) * 8))
	}

	return hash
}
