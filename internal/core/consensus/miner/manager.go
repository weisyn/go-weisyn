// Package miner 提供矿工管理服务的实现
//
// 🎯 **矿工管理器**
//
// 本文件实现矿工服务管理器，作为各个业务模块的协调中心：
// - **架构角色**：薄管理器，委托具体业务实现给专业模块
// - **接口实现**：统一实现 consensus.MinerService 公共接口
// - **模块协调**：协调 controller/、orchestrator/、pow_handler/ 等业务模块
// - **依赖注入**：作为各模块的依赖注入入口，管理全局依赖
package miner

import (
	"context"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	eventintegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/internal/core/consensus/miner/controller"
	"github.com/weisyn/v1/internal/core/consensus/miner/event_handler"
	"github.com/weisyn/v1/internal/core/consensus/miner/height_gate"
	"github.com/weisyn/v1/internal/core/consensus/miner/orchestrator"
	"github.com/weisyn/v1/internal/core/consensus/miner/pow_handler"
	"github.com/weisyn/v1/internal/core/consensus/miner/state_manager"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventIf "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// Manager 矿工管理器
//
// 🎯 **职责定位**：提供完整的矿工管理服务
//
// 遵循 block/transaction 模式的完整依赖注入架构
type Manager struct {
	// ========== 核心依赖 ==========
	logger           log.Logger                        // 日志记录器
	eventBus         eventIf.EventBus                  // 事件总线
	consensusOptions *consensusconfig.ConsensusOptions // 共识配置选项

	// 注意：事件处理现在直接使用eventBus，不再使用自定义EventCoordinator

	// ========== 业务模块实例 ==========
	controllerService   interfaces.MinerController    // 控制器服务
	orchestratorService interfaces.MiningOrchestrator // 编排器服务
	powHandlerService   interfaces.PoWComputeHandler  // PoW计算服务
	heightGateService   interfaces.HeightGateManager  // 高度门闸服务
	stateManagerService interfaces.MinerStateManager  // 状态管理服务
	eventHandlerService interfaces.MinerEventHandler  // 事件处理服务
}

// NewManager 创建矿工管理器实例
//
// 🎯 **薄管理器设计**：只保留必要依赖，委托具体功能给业务模块
func NewManager(
	// ========== 基础依赖 ==========
	logger log.Logger,
	eventBus eventIf.EventBus,
	consensusOptions *consensusconfig.ConsensusOptions,

	// ========== 业务服务依赖（传递给子模块） ==========
	blockService blockchain.BlockService,
	chainService blockchain.ChainService,
	syncService blockchain.SystemSyncService,
	cacheStore storage.MemoryStore,
	networkService netiface.Network,

	// ========== 加密服务依赖（传递给子模块） ==========
	powEngine crypto.POWEngine,
	hashManager crypto.HashManager,
	merkleTreeManager crypto.MerkleTreeManager,

	// ========== 聚合器依赖（用于区块提交） ==========
	aggregatorController interfaces.AggregatorController,

	// ========== 合规依赖（可选） ==========
	compliancePolicy complianceIfaces.Policy,

) consensus.MinerService {
	// 1. 创建所有业务模块服务（遵循 transaction 模式）
	powHandlerService := pow_handler.NewPoWComputeService(logger, powEngine, hashManager, merkleTreeManager)
	// 应用启动阶段初始化 PoW 引擎（幂等）
	if powHandlerService != nil {
		params := types.MiningParameters{
			MiningTimeout:   consensusOptions.Miner.MiningTimeout,
			LoopInterval:    consensusOptions.Miner.LoopInterval,
			MaxTransactions: int(consensusOptions.Miner.MaxTransactions),
			MinTransactions: int(consensusOptions.Miner.MinTransactions),
			TxSelectionMode: consensusOptions.Miner.TxSelectionMode,
		}
		if err := powHandlerService.StartPoWEngine(context.Background(), params); err != nil {
			if logger != nil {
				logger.Warnf("PoW 引擎启动失败（将按需在 StartMining 重试）：%v", err)
			}
		} else if logger != nil {
			logger.Info("PoW 引擎已在应用启动阶段初始化")
		}
	}
	heightGateService := height_gate.NewHeightGateService(logger, consensusOptions.Miner.MaxForkDepth)
	stateManagerService := state_manager.NewMinerStateService(logger)

	// 2. 创建编排器服务，注入完整依赖（包括聚合器接口和合规策略）
	orchestratorService := orchestrator.NewMiningOrchestratorService(
		logger,
		blockService,
		chainService,
		cacheStore,
		powHandlerService,
		heightGateService,
		stateManagerService,
		syncService,
		networkService,
		aggregatorController, // 聚合器控制器依赖
		&consensusOptions.Miner,
		compliancePolicy, // 合规策略依赖（可选）
	)

	// 3. 创建控制器服务，注入所有必要依赖（遵循内部接口交互原则）
	controllerService := controller.NewMinerControllerService(
		logger,
		eventBus,
		orchestratorService,
		stateManagerService,
		powHandlerService,
		&consensusOptions.Miner,
	)

	// 4. 创建事件处理服务，用于处理系统事件（如分叉事件）
	eventHandlerService := event_handler.NewMinerEventHandlerService(
		logger,
		controllerService,
		stateManagerService,
	)

	// 5. 创建Manager实例（薄管理器：只保留必要依赖）
	manager := &Manager{
		// 基础依赖
		logger:           logger,
		eventBus:         eventBus,
		consensusOptions: consensusOptions,

		// 业务模块服务依赖
		controllerService:   controllerService,
		orchestratorService: orchestratorService,
		powHandlerService:   powHandlerService,
		heightGateService:   heightGateService,
		stateManagerService: stateManagerService,
		eventHandlerService: eventHandlerService,
	}

	// 6. 使用标准事件订阅集成（遵循integration/event架构）
	if err := eventintegration.RegisterEventSubscriptions(
		eventBus,
		nil,                 // 不需要aggregator订阅
		eventHandlerService, // 使用miner事件处理器
		logger,
	); err != nil {
		logger.Errorf("注册事件订阅失败: %v", err)
		// 不阻断构造，允许系统继续运行
	}

	return manager
}

// ==================== consensus.MinerService 接口实现（薄实现） ====================

// StartMining 启动挖矿服务
func (m *Manager) StartMining(ctx context.Context, minerAddress []byte) error {
	return m.controllerService.StartMining(ctx, minerAddress) // 委托给业务模块
}

// StopMining 停止挖矿服务
func (m *Manager) StopMining(ctx context.Context) error {
	return m.controllerService.StopMining(ctx) // 委托给业务模块
}

// GetMiningStatus 获取挖矿状态
func (m *Manager) GetMiningStatus(ctx context.Context) (bool, []byte, error) {
	return m.controllerService.GetMiningStatus(ctx) // 委托给业务模块
}

// ==================== 注意：事件处理已重构 ====================
//
// 原有的事件处理方法已被移除，现在使用标准化的事件集成模式：
//
// 1. 事件订阅：通过 RegisterEventSubscriptions 统一注册
// 2. 事件处理：由 event_handler 子模块的 MinerEventHandlerService 处理
// 3. 接口统一：实现 MinerEventSubscriber 接口，使用标准签名
// 4. 类型安全：使用类型化的事件数据结构，避免 interface{} 类型转换
//
// 这种模式提供了更好的：
// - 类型安全性
// - 测试能力
// - 代码组织
// - 错误处理
// - 架构一致性

// 注意：所有旧的事件处理方法已被移除，现在使用标准的事件集成架构

// ==================== MinerEventHandler接口实现 ====================

// HandleForkDetected 处理分叉检测事件
//
// 🔀 **委托模式**：
// 委托给专门的事件处理服务处理分叉检测事件
func (m *Manager) HandleForkDetected(ctx context.Context, eventData *types.ForkDetectedEventData) error {
	if m.eventHandlerService == nil {
		if m.logger != nil {
			m.logger.Warn("[MinerManager] 事件处理服务未初始化，跳过分叉检测事件处理")
		}
		return nil
	}

	return m.eventHandlerService.HandleForkDetected(ctx, eventData)
}

// HandleForkProcessing 处理分叉处理中事件
//
// 🔄 **委托模式**：
// 委托给专门的事件处理服务处理分叉处理进度事件
func (m *Manager) HandleForkProcessing(ctx context.Context, eventData *types.ForkProcessingEventData) error {
	if m.eventHandlerService == nil {
		if m.logger != nil {
			m.logger.Warn("[MinerManager] 事件处理服务未初始化，跳过分叉处理中事件处理")
		}
		return nil
	}

	return m.eventHandlerService.HandleForkProcessing(ctx, eventData)
}

// HandleForkCompleted 处理分叉完成事件
//
// ✅ **委托模式**：
// 委托给专门的事件处理服务处理分叉完成事件
func (m *Manager) HandleForkCompleted(ctx context.Context, eventData *types.ForkCompletedEventData) error {
	if m.eventHandlerService == nil {
		if m.logger != nil {
			m.logger.Warn("[MinerManager] 事件处理服务未初始化，跳过分叉完成事件处理")
		}
		return nil
	}

	return m.eventHandlerService.HandleForkCompleted(ctx, eventData)
}

// ==================== 编译时接口检查 ====================

// 确保Manager实现了MinerEventHandler接口
var _ interfaces.MinerEventHandler = (*Manager)(nil)
