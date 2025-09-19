// Package orchestrator 实现挖矿编排器服务
//
// 🎯 **编排器服务模块**
//
// 本包实现 MiningOrchestrator 接口，提供挖矿流程的编排和控制：
// - 协调整个挖矿流程的执行
// - 管理候选区块创建和PoW计算
// - 处理区块发送和确认等待
package orchestrator

import (
	"context"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// MiningOrchestratorService 挖矿编排器服务实现
type MiningOrchestratorService struct {
	logger               log.Logger                      // 日志记录器
	cacheStore           storage.MemoryStore             // 内存缓存服务
	blockService         blockchain.BlockService         // 区块服务
	chainService         blockchain.ChainService         // 链服务（用于状态查询）
	syncService          blockchain.SystemSyncService    // 同步服务
	powHandlerService    interfaces.PoWComputeHandler    // PoW计算服务
	heightGateService    interfaces.HeightGateManager    // 高度门闸服务
	stateManagerService  interfaces.MinerStateManager    // 状态管理服务
	aggregatorController interfaces.AggregatorController // 聚合器控制器（用于区块提交）
	minerConfig          *consensusconfig.MinerConfig    // Miner配置（用于超时和间隔设置）
	compliancePolicy     complianceIfaces.Policy         // 合规策略服务（可选）
}

// NewMiningOrchestratorService 创建挖矿编排器服务实例
func NewMiningOrchestratorService(
	logger log.Logger,
	blockService blockchain.BlockService,
	chainService blockchain.ChainService,
	cacheStore storage.MemoryStore,
	powHandlerService interfaces.PoWComputeHandler,
	heightGateService interfaces.HeightGateManager,
	stateManagerService interfaces.MinerStateManager,
	syncService blockchain.SystemSyncService,
	networkService netiface.Network,
	aggregatorController interfaces.AggregatorController, // 聚合器控制器接口
	minerConfig *consensusconfig.MinerConfig,
	compliancePolicy complianceIfaces.Policy, // 合规策略服务（可选）
) interfaces.MiningOrchestrator {
	return &MiningOrchestratorService{
		logger:               logger,
		cacheStore:           cacheStore,
		blockService:         blockService,
		chainService:         chainService,
		syncService:          syncService,
		powHandlerService:    powHandlerService,
		heightGateService:    heightGateService,
		stateManagerService:  stateManagerService,
		aggregatorController: aggregatorController, // 聚合器控制器接口
		minerConfig:          minerConfig,
		compliancePolicy:     compliancePolicy, // 合规策略服务
	}
}

// 编译时确保 MiningOrchestratorService 实现了 MiningOrchestrator 接口
var _ interfaces.MiningOrchestrator = (*MiningOrchestratorService)(nil)

// ExecuteMiningRound 执行一轮挖矿
// 实现薄封装原则：仅进行接口方法委托，具体业务逻辑在 execute_mining_round.go 中实现
func (s *MiningOrchestratorService) ExecuteMiningRound(ctx context.Context) error {
	return s.executeMiningRound(ctx)
}
