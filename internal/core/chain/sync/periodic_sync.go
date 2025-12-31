// periodic_sync.go - 定时同步机制实现
// 负责基于时间的自动同步触发，确保长时间无新区块时能够及时发现网络更新
package sync

import (
	"context"
	"time"

	chaininterfaces "github.com/weisyn/v1/internal/core/chain/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// PeriodicSyncScheduler 定时同步调度器
type PeriodicSyncScheduler struct {
	chainQuery      persistence.ChainQuery
	queryService    persistence.QueryService
	blockValidator  block.BlockValidator
	blockProcessor  block.BlockProcessor
	routingManager  kademlia.RoutingTableManager
	networkService  network.Network
	p2pService      p2pi.Service
	configProvider  config.Provider
	logger          log.Logger
	eventBus        eventiface.EventBus
	blockHashClient core.BlockHashServiceClient
	forkHandler     chaininterfaces.InternalForkHandler

	// ✅ P1修复：临时存储服务（用于分页补齐时处理乱序区块）
	tempStore storage.TempStore

	// 节点运行时状态（用于更新同步状态）
	runtimeState p2pi.RuntimeState

	ticker          *time.Ticker
	stopChan        chan struct{}
	lastBlockHeight uint64
	lastBlockTime   time.Time
	isRunning       bool
}

func unixTimeFromBlockHeaderTimestamp(ts uint64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	// 兼容：秒级/毫秒级时间戳
	// - 秒级：~1e9
	// - 毫秒级：~1e12
	if ts > 1_000_000_000_000 {
		return time.Unix(0, int64(ts)*int64(time.Millisecond))
	}
	return time.Unix(int64(ts), 0)
}

// NewPeriodicSyncScheduler 创建定时同步调度器
//
// 🎯 **适配新的依赖注入架构**：
// - chainQuery: 使用persistence.ChainQuery替代ChainService（读操作）
// - blockValidator: 使用block.BlockValidator替代BlockService.ValidateBlock
// - blockProcessor: 使用block.BlockProcessor替代BlockService.ProcessBlock
//
// ⚠️ **同步状态管理**：
// - 同步状态不再持久化，查询时实时计算
// - 通过 runtimeState 实时更新节点同步状态
func NewPeriodicSyncScheduler(
	chainQuery persistence.ChainQuery,
	queryService persistence.QueryService,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	routingManager kademlia.RoutingTableManager,
	networkService network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	tempStore storage.TempStore,
	runtimeState p2pi.RuntimeState,
	blockHashClient core.BlockHashServiceClient,
	forkHandler chaininterfaces.InternalForkHandler,
	logger log.Logger,
	eventBus eventiface.EventBus,
) *PeriodicSyncScheduler {
	return &PeriodicSyncScheduler{
		chainQuery:      chainQuery,
		queryService:    queryService,
		blockValidator:  blockValidator,
		blockProcessor:  blockProcessor,
		routingManager:  routingManager,
		networkService:  networkService,
		p2pService:      p2pService,
		configProvider:  configProvider,
		logger:          logger,
		eventBus:        eventBus,
		blockHashClient: blockHashClient,
		forkHandler:     forkHandler,
		tempStore:       tempStore,
		runtimeState:    runtimeState,
		stopChan:        make(chan struct{}),
	}
}

// Start 启动定时同步调度器
func (p *PeriodicSyncScheduler) Start(ctx context.Context) error {
	if p.isRunning {
		return nil
	}

	// 从配置读取开关：允许禁用“时间探针触发”逻辑
	if p.configProvider != nil {
		if bc := p.configProvider.GetBlockchain(); bc != nil {
			if !bc.Sync.Advanced.TimeCheckEnabled {
				if p.logger != nil {
					p.logger.Info("⏰ 定时同步调度器未启动：time_check_enabled=false")
				}
				return nil
			}
		}
	}

	// 从配置获取定时同步间隔
	var syncInterval time.Duration = 10 * time.Minute // 默认10分钟
	blockchainConfig := p.configProvider.GetBlockchain()
	if blockchainConfig != nil {
		if blockchainConfig.Sync.Advanced.TimeCheckIntervalMins > 0 {
		syncInterval = time.Duration(blockchainConfig.Sync.Advanced.TimeCheckIntervalMins) * time.Minute
		} else if blockchainConfig.Block.BlockTimeTarget > 0 {
			// 若未配置 interval mins，则默认按出块目标时间的 1/2 做探针频率（有上限）
			syncInterval = time.Duration(blockchainConfig.Block.BlockTimeTarget) * time.Second / 2
			if syncInterval < 5*time.Second {
				syncInterval = 5 * time.Second
			}
			if syncInterval > 1*time.Minute {
				syncInterval = 1 * time.Minute
			}
		}
	}

	p.ticker = time.NewTicker(syncInterval)
	p.isRunning = true

	if p.logger != nil {
		p.logger.Infof("✅ 定时同步调度器已启动，检查间隔: %v", syncInterval)
	}

	go p.scheduledSyncLoop(ctx)
	return nil
}

// Stop 停止定时同步调度器
func (p *PeriodicSyncScheduler) Stop() {
	if !p.isRunning {
		return
	}

	close(p.stopChan)
	if p.ticker != nil {
		p.ticker.Stop()
	}
	p.isRunning = false

	if p.logger != nil {
		p.logger.Info("🛑 定时同步调度器已停止")
	}
}

// scheduledSyncLoop 定时同步循环
func (p *PeriodicSyncScheduler) scheduledSyncLoop(ctx context.Context) {
	for {
		select {
		case <-p.stopChan:
			return
		case <-ctx.Done():
			return
		case <-p.ticker.C:
			p.performScheduledSyncCheck(ctx)
		}
	}
}

// performScheduledSyncCheck 执行定时同步检查
func (p *PeriodicSyncScheduler) performScheduledSyncCheck(ctx context.Context) {
	if p.logger != nil {
		p.logger.Debug("⏰ 执行定时同步检查")
	}

	// 1. 获取当前链状态
	chainInfo, err := p.chainQuery.GetChainInfo(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warnf("定时同步检查-获取链状态失败: %v", err)
		}
		return
	}

	// 检查链信息是否为 nil
	if chainInfo == nil {
		if p.logger != nil {
			p.logger.Warnf("定时同步检查-链信息为空")
		}
		return
	}

	currentHeight := chainInfo.Height
	currentTime := time.Now()

	// 1.1 读取“最新区块的时间戳”（优先用区块头 timestamp，而不是本地 wall clock 变化）
	latestBlockTime := currentTime
	if currentHeight > 0 && p.queryService != nil {
		if blk, err := p.queryService.GetBlockByHeight(ctx, currentHeight); err == nil && blk != nil && blk.Header != nil {
			if ts := unixTimeFromBlockHeaderTimestamp(blk.Header.Timestamp); !ts.IsZero() {
				latestBlockTime = ts
			}
		}
	}

	// 查询网络高度（用于更新 RuntimeState）
	var networkHeight uint64 = currentHeight // 默认使用本地高度
	if p.runtimeState != nil {
		// 尝试查询网络高度（简化实现：使用本地高度作为默认值）
		// 注意：完整的网络高度查询逻辑在 checkSyncImpl 中，这里仅做基本更新
		// 获取同步滞后阈值（使用默认值，配置中暂无此字段）
		var syncLagThreshold uint64 = 10 // 默认10个区块

		// 更新 RuntimeState（使用本地高度作为网络高度的保守估计）
		// 注意：这里不进行完整的网络高度查询，以避免在定时检查中增加网络开销
		// 完整的网络高度查询和状态更新在 checkSyncImpl 中进行
		isSyncing := p.runtimeState.GetSyncStatus() == p2pi.SyncStatusSyncing
		p.runtimeState.UpdateSyncStatusFromSyncService(
			currentHeight,
			networkHeight, // 使用本地高度作为保守估计
			syncLagThreshold,
			isSyncing,
		)
	}

	// 2. 检查是否长时间没有新区块
	var blockStaleThreshold time.Duration = 15 * time.Minute // 默认15分钟
	blockchainConfig := p.configProvider.GetBlockchain()
	if blockchainConfig != nil {
		if blockchainConfig.Sync.Advanced.TimeCheckThresholdMins > 0 {
		blockStaleThreshold = time.Duration(blockchainConfig.Sync.Advanced.TimeCheckThresholdMins) * time.Minute
		} else if blockchainConfig.Block.BlockTimeTarget > 0 {
			// 若未显式配置 threshold mins，则按“出块目标时间 * 3 + 网络延迟buffer”派生。
			blockStaleThreshold = time.Duration(blockchainConfig.Block.BlockTimeTarget) * time.Second * 3
			if blockStaleThreshold < 30*time.Second {
				blockStaleThreshold = 30 * time.Second
			}
			if blockchainConfig.Sync.Advanced.NetworkLatencyBuffer > 0 {
				blockStaleThreshold += blockchainConfig.Sync.Advanced.NetworkLatencyBuffer
			}
		}
	}

	// 初始化状态记录
	if p.lastBlockHeight == 0 {
		p.lastBlockHeight = currentHeight
		p.lastBlockTime = latestBlockTime
		if p.logger != nil {
			p.logger.Debugf("定时同步检查-初始化状态记录: height=%d", currentHeight)
		}
		return
	}

	// 3. 判断是否需要触发同步
	needsSync := false
	reason := ""

	if currentHeight > p.lastBlockHeight {
		// 高度增加了，更新记录
		prev := p.lastBlockHeight
		p.lastBlockHeight = currentHeight
		p.lastBlockTime = latestBlockTime
		if p.logger != nil {
			p.logger.Debugf("定时同步检查-高度更新: %d → %d", prev, currentHeight)
		}
	} else if currentTime.Sub(p.lastBlockTime) > blockStaleThreshold {
		// 长时间没有新区块，触发同步
		needsSync = true
		reason = "长时间无新区块"
	}

	if needsSync {
		// 已在同步中则不重复触发（避免对外表现为“失败/堆积”）
		if hasActiveSyncTask() {
			if p.logger != nil {
				p.logger.Debugf("⏰ 定时同步跳过：已有同步任务进行中（reason=%s）", reason)
			}
			return
		}

		// ✅ 轻量探针：先做 hello/高度采样判断是否真的需要 full sync。
		// 设计意图：
		// - “长时间无新区块”可能是：网络确实没有出块、网络延迟、订阅丢包/抖动；
		// - full sync 成本较高（会进入 hello + blocks + range），而 probe 只做 hello/高度采样；
		// - 先 probe 再决定是否 full sync，既保证及时性又避免无谓网络开销。
		decision, _ := probeSyncImpl(
			ctx,
			p.chainQuery,
			p.queryService,
			p.routingManager,
			p.networkService,
			p.p2pService,
			p.configProvider,
			p.blockHashClient,
			p.logger,
		)

		if p.logger != nil {
			p.logger.Infof("🧪 定时同步探针结果: need_full_sync=%t reason=%s local=%d network_tip=%d fork=%t sampled=%d hello_ok=%d hint=%s",
				decision.ShouldFullSync,
				decision.Reason,
				currentHeight,
				decision.NetworkTip,
				decision.ForkDetected,
				decision.SampledPeers,
				decision.HelloSuccess,
				func() string {
					if decision.HintPeer == "" {
						return ""
					}
					s := decision.HintPeer.String()
					if len(s) > 12 {
						return s[:12] + "..."
					}
					return s
				}(),
			)
		}

		if !decision.ShouldFullSync {
			// 关键：探针确认“无需 full sync”时，避免每个 tick 都重复触发“长时间无新区块”的 full sync。
			// 这里将 lastBlockTime 视为“已通过探针确认网络状态”的时间点，从而延后下一次触发。
			p.lastBlockTime = currentTime
			return
		}

		if p.logger != nil {
			p.logger.Infof("⏰ 定时同步触发: %s (上次区块时间: %v前)",
				reason, currentTime.Sub(p.lastBlockTime))
		}

		// 清理过期的节点同步缓存
		cleanupExpiredPeerRecords(24 * time.Hour)

		// 执行同步检查（携带 tempStore，确保分页补齐路径启用乱序区块临时存储能力）
		syncCtx := ctx
		if decision.HintPeer != "" {
			syncCtx = ContextWithPeerHint(syncCtx, decision.HintPeer)
		}
		err := triggerSyncImpl(syncCtx, p.chainQuery, p.queryService, p.blockValidator, p.blockProcessor,
			p.routingManager, p.networkService, p.p2pService, p.configProvider, p.tempStore, p.blockHashClient, p.forkHandler, p.logger, p.eventBus, nil)
		if err != nil {
			if p.logger != nil {
				p.logger.Warnf("定时同步执行失败: %v", err)
			}
		}
	} else {
		if p.logger != nil {
			p.logger.Debug("⏰ 定时同步检查完成，无需同步")
		}
	}
}
