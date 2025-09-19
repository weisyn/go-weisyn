// periodic_sync.go - 定时同步机制实现
// 负责基于时间的自动同步触发，确保长时间无新区块时能够及时发现网络更新
package sync

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
)

// PeriodicSyncScheduler 定时同步调度器
type PeriodicSyncScheduler struct {
	chainService   blockchain.ChainService
	blockService   blockchain.BlockService
	routingManager kademlia.RoutingTableManager
	networkService network.Network
	host           node.Host
	configProvider config.Provider
	logger         log.Logger

	ticker          *time.Ticker
	stopChan        chan struct{}
	lastBlockHeight uint64
	lastBlockTime   time.Time
	isRunning       bool
}

// NewPeriodicSyncScheduler 创建定时同步调度器
func NewPeriodicSyncScheduler(
	chainService blockchain.ChainService,
	blockService blockchain.BlockService,
	routingManager kademlia.RoutingTableManager,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) *PeriodicSyncScheduler {
	return &PeriodicSyncScheduler{
		chainService:   chainService,
		blockService:   blockService,
		routingManager: routingManager,
		networkService: networkService,
		host:           host,
		configProvider: configProvider,
		logger:         logger,
		stopChan:       make(chan struct{}),
	}
}

// Start 启动定时同步调度器
func (p *PeriodicSyncScheduler) Start(ctx context.Context) error {
	if p.isRunning {
		return nil
	}

	// 从配置获取定时同步间隔
	var syncInterval time.Duration = 10 * time.Minute // 默认10分钟
	blockchainConfig := p.configProvider.GetBlockchain()
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.TimeCheckIntervalMins > 0 {
		syncInterval = time.Duration(blockchainConfig.Sync.Advanced.TimeCheckIntervalMins) * time.Minute
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
	chainInfo, err := p.chainService.GetChainInfo(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warnf("定时同步检查-获取链状态失败: %v", err)
		}
		return
	}

	currentHeight := chainInfo.Height
	currentTime := time.Now()

	// 2. 检查是否长时间没有新区块
	var blockStaleThreshold time.Duration = 15 * time.Minute // 默认15分钟
	blockchainConfig := p.configProvider.GetBlockchain()
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.TimeCheckThresholdMins > 0 {
		blockStaleThreshold = time.Duration(blockchainConfig.Sync.Advanced.TimeCheckThresholdMins) * time.Minute
	}

	// 初始化状态记录
	if p.lastBlockHeight == 0 {
		p.lastBlockHeight = currentHeight
		p.lastBlockTime = currentTime
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
		p.lastBlockHeight = currentHeight
		p.lastBlockTime = currentTime
		if p.logger != nil {
			p.logger.Debugf("定时同步检查-高度更新: %d → %d", p.lastBlockHeight, currentHeight)
		}
	} else if currentTime.Sub(p.lastBlockTime) > blockStaleThreshold {
		// 长时间没有新区块，触发同步
		needsSync = true
		reason = "长时间无新区块"
	}

	if needsSync {
		if p.logger != nil {
			p.logger.Infof("⏰ 定时同步触发: %s (上次区块时间: %v前)",
				reason, currentTime.Sub(p.lastBlockTime))
		}

		// 清理过期的节点同步缓存
		cleanupExpiredPeerRecords(24 * time.Hour)

		// 执行同步检查
		err := triggerSyncImpl(ctx, p.chainService, p.blockService, p.routingManager,
			p.networkService, p.host, p.configProvider, p.logger)
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
