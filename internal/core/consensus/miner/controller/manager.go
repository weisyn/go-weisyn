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
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MinerControllerService 矿工控制器服务实现
type MinerControllerService struct {
	// 基础依赖
	logger   log.Logger     // 日志记录器
	eventBus event.EventBus // 事件总线

	// 内部服务依赖（通过interfaces进行交互，避免重复造轮子）
	orchestratorService interfaces.MiningOrchestrator // 挖矿编排器
	stateManagerService interfaces.MinerStateManager  // 状态管理器
	powHandlerService   interfaces.PoWComputeHandler  // PoW计算服务
	minerConfig         *consensusconfig.MinerConfig  // 矿工配置（用于PoW参数）

	// 控制状态字段
	isRunning        atomic.Bool        // 挖矿运行状态（原子操作保证线程安全）
	minerAddress     []byte             // 矿工地址（需要保护）
	miningLoopCancel context.CancelFunc // 挖矿循环取消函数

	// 并发控制
	mu sync.RWMutex   // 保护共享状态
	wg sync.WaitGroup // 等待挖矿循环退出
}

// NewMinerControllerService 创建矿工控制器服务实例
func NewMinerControllerService(
	logger log.Logger,
	eventBus event.EventBus,
	orchestratorService interfaces.MiningOrchestrator,
	stateManagerService interfaces.MinerStateManager,
	powHandlerService interfaces.PoWComputeHandler,
	minerConfig *consensusconfig.MinerConfig,
) interfaces.MinerController {
	return &MinerControllerService{
		logger:              logger,
		eventBus:            eventBus,
		orchestratorService: orchestratorService,
		stateManagerService: stateManagerService,
		powHandlerService:   powHandlerService,
		minerConfig:         minerConfig,
	}
}

// 编译时确保 MinerControllerService 实现了 MinerController 接口
var _ interfaces.MinerController = (*MinerControllerService)(nil)

// StartMining 启动挖矿服务（薄委托实现）
func (s *MinerControllerService) StartMining(ctx context.Context, minerAddress []byte) error {
	return s.startMining(ctx, minerAddress)
}

// StopMining 停止挖矿服务（薄委托实现）
func (s *MinerControllerService) StopMining(ctx context.Context) error {
	return s.stopMining(ctx)
}

// GetMiningStatus 获取挖矿状态（薄委托实现）
func (s *MinerControllerService) GetMiningStatus(ctx context.Context) (bool, []byte, error) {
	return s.getMiningStatus(ctx)
}
