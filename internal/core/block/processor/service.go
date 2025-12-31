// Package processor 实现区块处理服务
//
// 🎯 **BlockProcessor 服务实现**
//
// 本包实现了区块处理服务，负责处理验证通过的区块。
// 采用原子性处理策略，确保状态一致性。
//
// 💡 **核心职责**：
// - 处理验证通过的区块
// - 执行区块中的所有交易
// - 更新UTXO状态
// - 清理交易池
// - 发布区块处理完成事件
package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	eventIntegration "github.com/weisyn/v1/internal/core/block/integration/event"
	"github.com/weisyn/v1/internal/core/block/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// Service 区块处理服务
//
// 🎯 **设计理念**：
// - 原子性：所有操作在事务中完成
// - 一致性：确保状态的严格一致性
// - 并发控制：同一时间只处理一个区块
// - 事件驱动：完成后发布事件通知
//
// 📦 **服务职责**：
// - ProcessBlock: 处理区块
// - GetProcessorMetrics: 获取处理性能指标
type Service struct {
	// ==================== 依赖注入 ====================

	// dataWriter 统一数据写入服务（统一写入入口）
	// ⚠️ 重要：所有数据写入都通过 DataWriter 完成
	dataWriter persistence.DataWriter

	// txProcessor 交易处理器（执行交易）
	txProcessor tx.TxProcessor

	// utxoWriter UTXO写入服务（用于业务逻辑：引用计数管理和状态根更新）
	// ✅ **架构修复**：
	// - 引用计数管理和状态根更新是业务逻辑，应该在业务层（BlockProcessor）处理
	// - Persistence 只负责持久化操作，不处理业务逻辑
	// - utxoWriter 用于业务逻辑操作，不用于持久化
	utxoWriter eutxo.UTXOWriter

	// utxoQuery UTXO查询服务（用于计算状态根）
	// ✅ **架构修复**：
	// - 状态根计算需要在 UTXO 变更后执行，属于业务逻辑
	// - BlockProcessor 通过 utxoQuery 计算状态根，然后通过 utxoWriter 更新
	utxoQuery persistence.UTXOQuery

	// mempool 交易池（清理已处理交易）
	mempool mempool.TxPool

	// hasher 哈希服务（用于其他哈希计算）
	hasher crypto.HashManager

	// blockHashClient 区块哈希服务客户端（用于计算区块哈希）
	blockHashClient core.BlockHashServiceClient

	// txHashClient 交易哈希服务客户端（用于计算交易哈希）
	txHashClient transaction.TransactionHashServiceClient

	// zkProofService ZK证明服务（用于验证StateOutput的ZK证明）
	// ✅ **用途**：验证区块中StateOutput的ZK证明有效性
	zkProofService ispc.ZKProofService

	// eventBus 事件总线（发布事件）
	eventBus event.EventBus

	// logger 日志记录器（可选）
	logger log.Logger

	// writeGate 全局写门闸（可选，用于只读模式和 REORG 写控制）
	writeGate wgif.WriteGate

	// ==================== 延迟注入 ====================

	// validator 验证器（延迟注入，避免循环依赖）
	validator interfaces.InternalBlockValidator

	// ==================== 并发控制 ====================

	// mu 互斥锁
	mu sync.Mutex

	// processing 是否正在处理
	processing bool

	// ==================== 指标收集 ====================

	// metrics 处理服务指标
	metrics *interfaces.ProcessorMetrics

	// metricsMu 指标读写锁
	metricsMu sync.Mutex

	// ==================== 状态管理 ====================

	// isHealthy 健康状态
	isHealthy bool

	// lastError 最后错误
	lastError error

	// ==================== 环境标识 ====================
	//
	// isDevOrTest 标记当前是否处于开发/测试环境（由上层通过配置注入）
	// - 在生产环境中，某些依赖缺失（如 zkProofService / utxoQuery）将视为致命错误
	// - 在开发/测试环境中，允许降级为“warn + 跳过验证”
	isDevOrTest bool
}

func (s *Service) publishCorruptionDetected(ctx context.Context, phase types.CorruptionPhase, severity types.CorruptionSeverity, height *uint64, hashHex string, key string, err error) {
	if s == nil || s.eventBus == nil || err == nil {
		return
	}
	data := types.CorruptionEventData{
		Component: types.CorruptionComponentUTXO,
		Phase:     phase,
		Severity:  severity,
		Height:    height,
		Hash:      hashHex,
		Key:       key,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        types.RFC3339Time(time.Now()),
	}
	s.eventBus.Publish(event.EventTypeCorruptionDetected, ctx, data)
}

// NewService 创建区块处理服务
//
// 🔧 **初始化流程**：
// 1. 验证必需依赖
// 2. 初始化指标
// 3. 设置默认配置
//
// 参数：
//   - dataWriter: 统一数据写入服务（必需，统一写入入口）
//   - txProcessor: 交易处理器（必需）
//   - utxoWriter: UTXO写入服务（可选，用于业务逻辑：引用计数管理和状态根更新）
//   - utxoQuery: UTXO查询服务（可选，用于计算状态根）
//   - mempool: 交易池（必需）
//   - hasher: 哈希服务（必需）
//   - blockHashClient: 区块哈希服务客户端（必需）
//   - txHashClient: 交易哈希服务客户端（必需）
//   - zkProofService: ZK证明服务（可选，用于验证StateOutput的ZK证明）
//   - eventBus: 事件总线（可选）
//   - logger: 日志记录器（可选）
//   - writeGate: 全局写门闸（可选，用于只读模式和 REORG 写控制）
//
// 返回：
//   - interfaces.InternalBlockProcessor: 区块处理服务实例
//   - error: 创建错误
func NewService(
	dataWriter persistence.DataWriter,
	txProcessor tx.TxProcessor,
	utxoWriter eutxo.UTXOWriter,
	utxoQuery persistence.UTXOQuery,
	mempool mempool.TxPool,
	hasher crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	txHashClient transaction.TransactionHashServiceClient,
	zkProofService ispc.ZKProofService,
	eventBus event.EventBus,
	logger log.Logger,
	writeGate wgif.WriteGate,
) (interfaces.InternalBlockProcessor, error) {
	// 验证必需依赖
	if dataWriter == nil {
		return nil, fmt.Errorf("dataWriter 不能为空")
	}
	if txProcessor == nil {
		return nil, fmt.Errorf("txProcessor 不能为空")
	}
	if mempool == nil {
		return nil, fmt.Errorf("mempool 不能为空")
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

	// 生产环境安全约束：
	// - 目前 Block 模块并未直接注入 config.Provider，因此无法在此精确区分 prod/dev。
	// - 为避免误将关键验证依赖当作“可选”，这里采取保守策略：zkProofService 和 utxoQuery 缺失仅记录警告，
	//   但在交易验证路径中会使用严格的失败策略（参见 verifyStateOutput / verifyReferenceUTXO），
	//   即：显式依赖缺失或验证出错时，区块处理失败，不会静默放行。
	// - 若未来需要基于配置区分环境，可通过额外参数注入 isDevOrTest 标志，并在此处强制要求依赖非空。

	// 创建服务实例
	s := &Service{
		dataWriter:      dataWriter,
		txProcessor:     txProcessor,
		utxoWriter:      utxoWriter, // ✅ 用于业务逻辑：引用计数管理和状态根更新
		utxoQuery:       utxoQuery,  // ✅ 用于计算状态根
		mempool:         mempool,
		hasher:          hasher,
		blockHashClient: blockHashClient,
		txHashClient:    txHashClient,
		zkProofService:  zkProofService, // ✅ 用于验证StateOutput的ZK证明
		eventBus:        eventBus,
		logger:          logger,
		writeGate:       writeGate, // 可选，用于只读模式和 REORG 写控制
		metrics:         &interfaces.ProcessorMetrics{},
		isHealthy:       true,
	}

	if logger != nil {
		logger.Info("✅ BlockProcessor 服务初始化成功（已迁移到 DataWriter）")
	}

	return s, nil
}

// ProcessBlock 处理区块
//
// 🎯 **处理流程（对外语义“原子”）**：
// 1. 并发控制检查
// 2. 区块结构和基本字段校验
// 3. 区块级验证（调用 Validator）
// 4. 业务级交易验证（ZK / 资源生命周期 / 引用UTXO 等）
// 5. 通过 DataWriter 在单一事务中持久化区块及其 UTXO / 索引 / 链状态
// 6. 在持久化成功后，按需更新引用计数和状态根、清理交易池
// 7. 发布 BlockProcessed 事件并记录指标
//
// 参数：
//   - ctx: 上下文
//   - block: 待处理区块
//
// 返回：
//   - error: 处理错误
func (s *Service) ProcessBlock(ctx context.Context, block *core.Block) error {
	// 0. WriteGate 检查（只读模式/写围栏保护）
	if s.writeGate != nil {
		if err := s.writeGate.AssertWriteAllowed(ctx, "block.ProcessBlock"); err != nil {
			return fmt.Errorf("写操作被阻止: %w", err)
		}
	}

	// 1. 并发控制
	s.mu.Lock()
	if s.processing {
		s.mu.Unlock()
		return fmt.Errorf("正在处理其他区块，请稍后再试")
	}
	s.processing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.processing = false
		s.mu.Unlock()
	}()

	startTime := time.Now()
	defer func() {
		s.recordProcess(time.Since(startTime))
	}()

	// 检查区块是否为 nil
	if block == nil {
		return s.recordProcessError(fmt.Errorf("区块不能为空"))
	}

	// 检查区块头和区块体是否为 nil
	if block.Header == nil || block.Body == nil {
		return s.recordProcessError(fmt.Errorf("区块头或区块体不能为空"))
	}

	if s.logger != nil {
		s.logger.Infof("开始处理区块，高度: %d",
			block.Header.Height)
	}

	// 2. 验证区块（如果有验证器）
	if s.validator != nil {
		valid, err := s.validator.ValidateBlock(ctx, block)
		if err != nil || !valid {
			return s.recordProcessError(fmt.Errorf("区块验证失败: %w", err))
		}
	}

	// 3. 处理区块（详细实现在 execute.go）
	if err := s.executeBlock(ctx, block); err != nil {
		return s.recordProcessError(err)
	}

	// 4. 发布事件（如果有事件总线）
	if s.eventBus != nil {
		// 计算区块哈希
		blockHash, err := s.calculateBlockHash(ctx, block.Header)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("计算区块哈希失败，跳过事件发布: %v", err)
			}
		} else {
			if err := eventIntegration.PublishBlockProcessedEvent(ctx, s.eventBus, s.logger, block, blockHash); err != nil {
				// 事件发布失败不影响区块处理，只记录警告
				if s.logger != nil {
					s.logger.Warnf("发布BlockProcessed事件失败: %v", err)
				}
			}
		}
	}

	// 5. 记录成功
	s.recordProcessSuccess(block)

	if s.logger != nil {
		s.logger.Infof("✅ 区块处理完成，高度: %d, 交易数: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	return nil
}

// ==================== 内部管理方法 ====================

// GetProcessorMetrics 获取处理服务指标
func (s *Service) GetProcessorMetrics(ctx context.Context) (*interfaces.ProcessorMetrics, error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	// 更新处理状态
	s.mu.Lock()
	s.metrics.IsProcessing = s.processing
	s.mu.Unlock()

	// 更新健康状态
	s.metrics.IsHealthy = s.isHealthy
	if s.lastError != nil {
		s.metrics.ErrorMessage = s.lastError.Error()
	}

	return s.metrics, nil
}

// SetValidator 设置验证器（延迟注入）
func (s *Service) SetValidator(validator interfaces.InternalBlockValidator) {
	s.validator = validator

	if s.logger != nil {
		s.logger.Info("🔗 Validator 已注入到 Processor")
	}
}

// ==================== 辅助方法 ====================

// recordProcess 记录处理指标
func (s *Service) recordProcess(duration time.Duration) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.LastProcessTime = time.Now().Unix()

	// 更新平均处理耗时（滑动平均）
	alpha := 0.1
	newTime := duration.Seconds()
	if s.metrics.AvgProcessTime == 0 {
		s.metrics.AvgProcessTime = newTime
	} else {
		s.metrics.AvgProcessTime = alpha*newTime + (1-alpha)*s.metrics.AvgProcessTime
	}

	// 更新最大处理耗时
	if newTime > s.metrics.MaxProcessTime {
		s.metrics.MaxProcessTime = newTime
	}
}

// recordProcessSuccess 记录处理成功
func (s *Service) recordProcessSuccess(block *core.Block) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.BlocksProcessed++
	s.metrics.SuccessCount++

	// 检查区块和区块体是否为 nil
	if block != nil && block.Body != nil {
		s.metrics.TransactionsExecuted += uint64(len(block.Body.Transactions))
	}

	if block != nil && block.Header != nil {
		s.metrics.LastBlockHeight = block.Header.Height

		// 计算区块哈希并保存到指标
		if s.blockHashClient != nil {
			// 使用 context.Background() 因为这是指标更新，不需要取消
			if blockHash, err := s.calculateBlockHash(context.Background(), block.Header); err == nil {
				s.metrics.LastBlockHash = blockHash
			}
		}
	}

	s.isHealthy = true
}

// recordProcessError 记录处理错误
func (s *Service) recordProcessError(err error) error {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.FailureCount++
	s.isHealthy = false
	s.lastError = err

	return err
}

// calculateBlockHash 计算区块哈希
func (s *Service) calculateBlockHash(ctx context.Context, header *core.BlockHeader) ([]byte, error) {
	if header == nil {
		return nil, fmt.Errorf("区块头为空")
	}
	if s.blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 未初始化")
	}

	// 构建区块（只有Header，Body可以为空）
	block := &core.Block{
		Header: header,
	}

	// 使用 gRPC 服务计算区块哈希
	req := &core.ComputeBlockHashRequest{
		Block: block,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("区块结构无效")
	}

	return resp.Hash, nil
}

// 编译时检查接口实现
var _ interfaces.InternalBlockProcessor = (*Service)(nil)
