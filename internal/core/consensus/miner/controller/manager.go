// Package controller 实现矿工控制器服务
//
// 🎯 **控制器服务模块**
//
// 本包实现 MinerController 接口，提供矿工公共接口的具体实现：
// - 继承并实现 consensus.MinerService 接口
// - 作为对外服务的统一入口
// - 管理挖矿的启动、停止和状态查询
package controller

import (
	"context"
	"sync"
	"sync/atomic"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// MinerControllerService 矿工控制器服务实现
type MinerControllerService struct {
	// 基础依赖
	logger   log.Logger     // 日志记录器
	eventBus event.EventBus // 事件总线

	// 链查询服务（用于检查链是否已初始化/就绪）
	chainQuery persistence.ChainQuery

	// 内部服务依赖（通过interfaces进行交互，避免重复造轮子）
	orchestratorService interfaces.MiningOrchestrator // 挖矿编排器
	stateManagerService interfaces.MinerStateManager  // 状态管理器
	powHandlerService   interfaces.PoWComputeHandler  // PoW计算服务
	minerConfig         *consensusconfig.MinerConfig  // 矿工配置（用于PoW参数）
	quorumChecker       quorum.Checker                // v2：挖矿稳定性门闸检查器（开关阶段）

	// 控制状态字段
	isRunning        atomic.Bool        // 挖矿运行状态（原子操作保证线程安全）
	minerAddress     []byte             // 矿工地址（需要保护）
	miningLoopCancel context.CancelFunc // 挖矿循环取消函数
	mineOnceMode     bool               // 🔧 单次挖矿模式标志（true=挖完一个区块后自动停止）

	// 并发控制
	mu sync.RWMutex   // 保护共享状态
	wg sync.WaitGroup // 等待挖矿循环退出
}

// NewMinerControllerService 创建矿工控制器服务实例
func NewMinerControllerService(
	logger log.Logger,
	eventBus event.EventBus,
	chainQuery persistence.ChainQuery,
	orchestratorService interfaces.MiningOrchestrator,
	stateManagerService interfaces.MinerStateManager,
	powHandlerService interfaces.PoWComputeHandler,
	minerConfig *consensusconfig.MinerConfig,
	quorumChecker quorum.Checker,
) interfaces.MinerController {
	return &MinerControllerService{
		logger:              logger,
		eventBus:            eventBus,
		chainQuery:          chainQuery,
		orchestratorService: orchestratorService,
		stateManagerService: stateManagerService,
		powHandlerService:   powHandlerService,
		minerConfig:         minerConfig,
		quorumChecker:       quorumChecker,
	}
}

// 编译时确保 MinerControllerService 实现了 MinerController 接口
var _ interfaces.MinerController = (*MinerControllerService)(nil)

// StartMining 启动挖矿服务（薄委托实现）
func (s *MinerControllerService) StartMining(ctx context.Context, minerAddress []byte) error {
	return s.startMining(ctx, minerAddress)
}

// StartMiningOnce 启动单次挖矿服务（薄委托实现）
//
// 🎯 **单次挖矿模式**
// 挖掘一个区块后自动停止，适用于测试和手动控制场景
func (s *MinerControllerService) StartMiningOnce(ctx context.Context, minerAddress []byte) error {
	return s.startMiningOnce(ctx, minerAddress)
}

// StopMining 停止挖矿服务（薄委托实现）
func (s *MinerControllerService) StopMining(ctx context.Context) error {
	return s.stopMining(ctx)
}

// GetMiningStatus 获取挖矿状态（薄委托实现）
func (s *MinerControllerService) GetMiningStatus(ctx context.Context) (bool, []byte, error) {
	return s.getMiningStatus(ctx)
}
