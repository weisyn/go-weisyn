// Package processor 实现交易处理器服务
//
// 🎯 **交易处理器核心实现**
//
// 本包实现 Processor 接口，提供交易处理的统一入口，并整合网络和事件能力：
// - 核心交易处理（验证 + 提交）
// - 网络交易接收（P2P 网络集成）
// - 事件订阅监听（交易状态跟踪）
//
// 设计理念：
// - 薄协调层：不实现具体逻辑，只做组件协调
// - 依赖注入：通过组合模式整合子模块能力
// - 接口委托：将网络和事件能力委托给专门的 handler
package processor

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/tx/processor/event_handler"
	"github.com/weisyn/v1/internal/core/tx/processor/network_handler"
	"github.com/weisyn/v1/internal/core/tx/verifier"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// Service 交易处理器服务实现
//
// 🎯 **核心职责**：
// - 对外统一入口：提供 SubmitTx、GetTxStatus 等公共接口
// - 验证交易：调用 Verifier 进行三阶段验证
// - 网络交易接收：委托给 NetworkHandler 处理
// - 事件状态跟踪：委托给 EventHandler 处理
//
// 🏗️ **设计原则**：
// - 薄协调层：只做组件协调，不实现具体逻辑
// - 组合优于继承：通过组合模式整合子模块能力
// - 依赖注入：所有依赖通过构造函数注入
type Service struct {
	verifier       Verifier                        // 交易验证器（P1 新增）
	txPool         mempool.TxPool                  // 交易池服务
	logger         log.Logger                      // 日志服务
	networkHandler *network_handler.NetworkHandler // 网络协议处理器
	eventHandler   *event_handler.EventHandler     // 事件订阅处理器

	configProvider config.Provider          // 配置提供者（用于获取链ID等）
	utxoQuery      persistence.UTXOQuery    // UTXO 查询服务（用于环境注入）
	queryService   persistence.QueryService // 统一查询服务（用于获取当前高度等）
}

// Verifier 交易验证器接口（内部使用）
//
// 注意：这里定义一个简化的接口，避免循环依赖
type Verifier interface {
	Verify(ctx context.Context, tx *transaction.Transaction) error
	VerifyWithContext(ctx context.Context, tx *transaction.Transaction, validationCtx interface{}) error
}

// NewService 创建交易处理器服务实例
//
// 参数:
//
//	verifier: 交易验证器（P1 新增）
//	txPool: 交易池服务
//	chainStateReader: 链状态读取器（P1.5 新增）
//	logger: 日志服务
//
// 返回:
//
//	*Service: 交易处理器服务实例
func NewService(
	verifier Verifier,
	txPool mempool.TxPool,
	configProvider config.Provider,
	utxoQuery persistence.UTXOQuery,
	queryService persistence.QueryService,
	logger log.Logger,
) *Service {
	// 创建子模块
	networkHandler := network_handler.NewNetworkHandler(txPool, logger)
	eventHandler := event_handler.NewEventHandler(logger, nil) // EventBus 后续设置

	return &Service{
		verifier:       verifier,
		txPool:         txPool,
		logger:         logger,
		networkHandler: networkHandler,
		eventHandler:   eventHandler,
		configProvider: configProvider,
		utxoQuery:      utxoQuery,
		queryService:   queryService,
	}
}

// ============================================================================
//                           核心交易处理接口实现
// ============================================================================

// SubmitTx 提交交易到系统（由上层传入环境，TX 不感知链状态）
//
// 🎯 **实现 tx.TxProcessor.SubmitTx 接口**
//
// 处理流程：
// 1. 由上层系统（blockchain/调度器）负责环境注入
// 2. 使用 Verifier 验证交易（AuthZ + Conservation + Condition）
// 3. 验证通过后提交到池（TxPool 内部自动广播）
// 4. 返回 SubmittedTx
//
// 参数：
//   - ctx: 上下文对象
//   - signedTx: 已签名的交易
//
// 返回：
//   - *types.SubmittedTx: 已提交的交易
//   - error: 处理过程中的错误
func (s *Service) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	if s.logger != nil {
		s.logger.Infof("[TxProcessor] 📥 提交交易")
	}

	// 1. 构造验证环境（近似当前区块视图，用于 TimeLock/HeightLock/Nonce 等条件校验）
	env, err := s.buildVerifierEnvironment(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[TxProcessor] ❌ 构造验证环境失败: %v", err)
		}
		return nil, err
	}

	// 2. 使用带环境的验证接口进行交易验证
	if err := s.verifier.VerifyWithContext(ctx, signedTx.Tx, env); err != nil {
		if s.logger != nil {
			s.logger.Errorf("[TxProcessor] ❌ 交易验证失败: %v", err)
		}
		return nil, err
	}

	if s.logger != nil {
		s.logger.Infof("[TxProcessor] ✅ 交易验证通过")
	}

	// 3. 提交到池（TxPool 内部会自动广播）
	txHash, err := s.txPool.SubmitTx(signedTx.Tx)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[TxProcessor] ❌ 交易提交失败: %v", err)
		}
		return nil, err
	}

	if s.logger != nil {
		s.logger.Infof("[TxProcessor] ✅ 交易提交成功: txHash=%x", txHash[:8])
	}

	// 4. 返回 SubmittedTx
	return &types.SubmittedTx{
		TxHash:      txHash,
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

// buildVerifierEnvironment 构造用于验证的环境视图
//
// 设计原则：
//   - 使用当前链配置和查询服务，构造一个尽量接近“当前区块视图”的环境；
//   - 该环境用于 TxPool 提交阶段的预验证，最终安全性仍由区块验证时的真实环境保证。
func (s *Service) buildVerifierEnvironment(ctx context.Context) (tx.VerifierEnvironment, error) {
	if s.configProvider == nil {
		return nil, fmt.Errorf("config provider is nil")
	}
	if s.utxoQuery == nil {
		return nil, fmt.Errorf("utxo query is nil")
	}
	if s.queryService == nil {
		return nil, fmt.Errorf("query service is nil")
	}

	// 获取链配置（主要用于 ChainID）
	blockchainCfg := s.configProvider.GetBlockchain()
	if blockchainCfg == nil {
		return nil, fmt.Errorf("blockchain config is nil")
	}

	// 当前链高度（本地视角）
	currentHeight, err := s.queryService.GetCurrentHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取当前链高度失败: %w", err)
	}

	// 近似当前区块时间：这里只能使用本地时间作为近似值
	currentTime := uint64(time.Now().Unix())

	// 将 ChainID(uint64) 编码为 []byte，供插件使用
	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, blockchainCfg.ChainID)

	envCfg := &verifier.VerifierEnvironmentConfig{
		BlockHeight:  currentHeight,
		BlockTime:    currentTime,
		ChainID:      chainIDBytes,
		UTXOQuery:    s.utxoQuery,
		QueryService: s.queryService,
	}

	return verifier.NewStaticVerifierEnvironment(envCfg), nil
}

// GetTxStatus 获取交易状态
//
// 🎯 **实现 tx.TxProcessor.GetTxStatus 接口**
//
// 查询流程：
// 1. 从 TxPool 查询交易
// 2. 如果存在，返回状态信息
// 3. 如果不存在，返回 NotFound
//
// 参数：
//   - ctx: 上下文对象
//   - txHash: 交易哈希
//
// 返回：
//   - *types.TxBroadcastState: 交易广播状态
//   - error: 查询过程中的错误
func (s *Service) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	if s.logger != nil {
		s.logger.Debugf("[TxProcessor] 🔍 查询交易状态: txHash=%x", txHash[:8])
	}

	// 从 TxPool 查询交易
	_, err := s.txPool.GetTx(txHash)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("[TxProcessor] 交易不存在: txHash=%x, error=%v", txHash[:8], err)
		}
		return nil, err
	}

	// 交易存在，返回状态（在池中待处理）
	// 💡 **增强建议**：当前简化实现仅返回基础状态。
	// 理想情况下应从以下来源获取更详细的状态信息：
	// 1. TxPool: 添加 GetTxMetadata(txHash) 接口，返回进池时间、广播状态
	// 2. EventBus: 订阅交易生命周期事件（已广播、已确认、已拒绝）
	// 3. P2P层: 获取广播进度（已发送到多少节点、收到多少确认）
	// 当前仅返回"已提交到本地"状态，满足基本需求。
	now := time.Now()
	return &types.TxBroadcastState{
		TxHash:      txHash,
		Status:      types.BroadcastStatusLocalSubmitted,
		SubmittedAt: now,
	}, nil
}

// ============================================================================
//                           网络协议接口实现（委托）
// ============================================================================

// HandleTransactionAnnounce 处理交易公告（委托给 NetworkHandler）
//
// 🎯 **实现 TxAnnounceRouter.HandleTransactionAnnounce 接口**
func (s *Service) HandleTransactionAnnounce(ctx context.Context, from peer.ID, topic string, data []byte) error {
	return s.networkHandler.HandleTransactionAnnounce(ctx, from, topic, data)
}

// HandleTransactionDirect 处理交易直连传播（委托给 NetworkHandler）
//
// 🎯 **实现 TxProtocolRouter.HandleTransactionDirect 接口**
func (s *Service) HandleTransactionDirect(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	return s.networkHandler.HandleTransactionDirect(ctx, from, reqBytes)
}

// ============================================================================
//                           事件订阅接口实现（委托）
// ============================================================================

// HandleTransactionReceived 处理交易接收事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleTransactionReceived 接口**
func (s *Service) HandleTransactionReceived(eventData *types.TransactionReceivedEventData) error {
	return s.eventHandler.HandleTransactionReceived(eventData)
}

// HandleTransactionValidated 处理交易验证事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleTransactionValidated 接口**
func (s *Service) HandleTransactionValidated(eventData *types.TransactionValidatedEventData) error {
	return s.eventHandler.HandleTransactionValidated(eventData)
}

// HandleTransactionExecuted 处理交易执行事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleTransactionExecuted 接口**
func (s *Service) HandleTransactionExecuted(eventData *types.TransactionExecutedEventData) error {
	return s.eventHandler.HandleTransactionExecuted(eventData)
}

// HandleTransactionFailed 处理交易失败事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleTransactionFailed 接口**
func (s *Service) HandleTransactionFailed(eventData *types.TransactionFailedEventData) error {
	return s.eventHandler.HandleTransactionFailed(eventData)
}

// HandleTransactionConfirmed 处理交易确认事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleTransactionConfirmed 接口**
func (s *Service) HandleTransactionConfirmed(eventData *types.TransactionConfirmedEventData) error {
	return s.eventHandler.HandleTransactionConfirmed(eventData)
}

// HandleMempoolTransactionAdded 处理交易添加到内存池事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleMempoolTransactionAdded 接口**
func (s *Service) HandleMempoolTransactionAdded(eventData *types.TransactionReceivedEventData) error {
	return s.eventHandler.HandleMempoolTransactionAdded(eventData)
}

// HandleMempoolTransactionRemoved 处理内存池交易移除事件（委托给 EventHandler）
//
// 🎯 **实现 TransactionEventSubscriber.HandleMempoolTransactionRemoved 接口**
func (s *Service) HandleMempoolTransactionRemoved(eventData *types.TransactionRemovedEventData) error {
	return s.eventHandler.HandleMempoolTransactionRemoved(eventData)
}

// ============================================================================
//                              辅助方法
// ============================================================================

// GetTransactionStats 获取交易处理统计信息
//
// 返回 EventHandler 维护的统计数据
func (s *Service) GetTransactionStats() map[string]interface{} {
	return s.eventHandler.GetTransactionStats()
}
