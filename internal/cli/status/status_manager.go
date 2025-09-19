package status

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/internal/cli/client"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// SystemStatus 系统状态信息
type SystemStatus struct {
	// 基本信息
	Version     string `json:"version"`
	NodeID      string `json:"node_id"`
	Environment string `json:"environment"`

	// 网络状态
	ConnectedPeers int    `json:"connected_peers"`
	NetworkDelay   string `json:"network_delay"`

	// 区块链状态
	BlockHeight   uint64 `json:"block_height"`
	ChainID       uint64 `json:"chain_id"`
	LastBlockTime string `json:"last_block_time"`

	// 挖矿状态
	IsMining    bool   `json:"is_mining"`
	HashRate    string `json:"hash_rate"`
	MinedBlocks int    `json:"mined_blocks"`

	// 更新时间
	LastUpdated time.Time `json:"last_updated"`
}

// StatusManager 状态管理器
type StatusManager struct {
	logger         log.Logger
	chainService   blockchainintf.ChainService
	minerService   consensusintf.MinerService
	configProvider config.Provider
	apiClient      *client.Client

	status *SystemStatus
	mutex  sync.RWMutex

	// 更新控制
	updateInterval time.Duration
	stopChan       chan struct{}
	isRunning      bool
}

// NewStatusManager 创建状态管理器
func NewStatusManager(
	logger log.Logger,
	chainService blockchainintf.ChainService,
	minerService consensusintf.MinerService, // 可选依赖
	configProvider config.Provider,
	apiClient *client.Client,
) *StatusManager {
	return &StatusManager{
		logger:         logger,
		chainService:   chainService,
		minerService:   minerService,
		configProvider: configProvider,
		apiClient:      apiClient,
		status:         &SystemStatus{LastUpdated: time.Now()},
		updateInterval: 2 * time.Second, // 2秒更新一次，实现动态刷新
		stopChan:       make(chan struct{}),
	}
}

// Start 启动状态管理器
func (sm *StatusManager) Start(ctx context.Context) error {
	if sm.isRunning {
		return fmt.Errorf("status manager is already running")
	}

	sm.isRunning = true

	// 立即更新一次状态
	sm.updateStatus(ctx)

	// 启动定时更新协程
	go sm.updateLoop(ctx)

	sm.logger.Info("状态管理器已启动")
	return nil
}

// Stop 停止状态管理器
func (sm *StatusManager) Stop() {
	if !sm.isRunning {
		return
	}

	close(sm.stopChan)
	sm.isRunning = false
	sm.logger.Info("状态管理器已停止")
}

// GetStatus 获取当前状态
func (sm *StatusManager) GetStatus() *SystemStatus {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// 返回副本，避免并发问题
	statusCopy := *sm.status
	return &statusCopy
}

// updateLoop 状态更新循环
func (sm *StatusManager) updateLoop(ctx context.Context) {
	ticker := time.NewTicker(sm.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.updateStatus(ctx)
		case <-sm.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// updateStatus 更新状态信息
func (sm *StatusManager) updateStatus(ctx context.Context) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 更新基本信息 - 版本显示
	if sm.configProvider != nil {
		sm.status.Version = version.GetDisplayVersion(nil)

		// 从区块链配置获取网络类型作为环境信息
		if blockchainConfig := sm.configProvider.GetBlockchain(); blockchainConfig != nil {
			sm.status.Environment = blockchainConfig.NetworkType
		} else {
			sm.status.Environment = "N/A"
		}
	} else {
		sm.status.Version = "N/A"
		sm.status.Environment = "N/A"
		sm.logger.Info("配置提供者不可用，使用默认值")
	}

	// 获取节点信息 - 从API获取真实节点ID
	if sm.apiClient != nil {
		if nodeInfo, err := sm.apiClient.GetNodeInfo(ctx); err == nil {
			// 显示完整节点ID（详细状态页面会显示完整的）
			sm.status.NodeID = nodeInfo.NodeID
		} else {
			sm.status.NodeID = "N/A"
			sm.logger.Info(fmt.Sprintf("获取节点信息失败: %v", err))
		}
	} else {
		sm.status.NodeID = "N/A"
		sm.logger.Info("API客户端不可用，无法获取节点信息")
	}

	// 更新区块链状态 - 使用真实接口
	if sm.chainService != nil {
		if chainInfo, err := sm.chainService.GetChainInfo(ctx); err == nil {
			sm.status.BlockHeight = chainInfo.Height
			sm.status.LastBlockTime = time.Unix(chainInfo.LastBlockTime, 0).Format("15:04:05")
		} else {
			sm.logger.Info(fmt.Sprintf("获取链信息失败: %v", err))
		}
	} else {
		sm.status.BlockHeight = 0
		sm.status.LastBlockTime = "N/A"
		sm.logger.Info("区块链服务不可用")
	}

	// 从配置获取ChainID
	if sm.configProvider != nil {
		if blockchainConfig := sm.configProvider.GetBlockchain(); blockchainConfig != nil {
			sm.status.ChainID = blockchainConfig.ChainID
		}
	} else {
		sm.status.ChainID = 0
		sm.logger.Info("配置提供者不可用")
	}

	// 更新网络状态 - 优先从peer列表获取真实连接数，备用方案使用节点信息
	if sm.apiClient != nil {
		if peers, err := sm.apiClient.GetNodePeers(ctx); err == nil {
			sm.status.ConnectedPeers = len(peers)
		} else {
			// 备用方案：从节点信息获取连接数
			if nodeInfo, err := sm.apiClient.GetNodeInfo(ctx); err == nil {
				sm.status.ConnectedPeers = nodeInfo.GetPeerCount()
			} else {
				sm.status.ConnectedPeers = 0
			}
			sm.logger.Info(fmt.Sprintf("获取peer列表失败，使用备用方案: %v", err))
		}
	} else {
		sm.status.ConnectedPeers = 0
		sm.logger.Info("API客户端不可用，无法获取连接数")
	}
	sm.status.NetworkDelay = "N/A" // 未实现ping/RTT时显示N/A

	// 更新挖矿状态 - 从真实接口获取（minerService为可选依赖）
	if sm.minerService != nil {
		if isRunning, _, err := sm.minerService.GetMiningStatus(ctx); err == nil {
			sm.status.IsMining = isRunning
		} else {
			sm.status.IsMining = false
			sm.logger.Info(fmt.Sprintf("获取挖矿状态失败: %v", err))
		}
	} else {
		sm.status.IsMining = false
		sm.logger.Info("挖矿服务不可用")
	}

	// 根据项目约束，不显示哈希率（公共接口不暴露指标）
	sm.status.HashRate = "N/A"
	sm.status.MinedBlocks = 0 // 同样不显示挖矿区块数

	sm.status.LastUpdated = time.Now()
}

// GetConfigInfo 获取配置信息（用于只读查看）
func (sm *StatusManager) GetConfigInfo() map[string]interface{} {
	configInfo := make(map[string]interface{})

	if sm.configProvider == nil {
		configInfo["error"] = "配置提供者不可用"
		return configInfo
	}

	// 区块链配置
	if blockchain := sm.configProvider.GetBlockchain(); blockchain != nil {
		configInfo["blockchain"] = map[string]interface{}{
			"chain_id":     blockchain.ChainID,
			"network_type": blockchain.NetworkType,
		}
	}

	// API配置
	if api := sm.configProvider.GetAPI(); api != nil {
		configInfo["api"] = map[string]interface{}{
			"http_host": api.HTTP.Host,
			"http_port": api.HTTP.Port,
		}
	}

	// 节点配置
	if node := sm.configProvider.GetNode(); node != nil {
		nodeConfig := make(map[string]interface{})

		// 主机配置 - 监听地址
		if len(node.Host.ListenAddresses) > 0 {
			nodeConfig["listen_addresses"] = node.Host.ListenAddresses
		}

		// 连接管理配置
		nodeConfig["min_peers"] = node.Connectivity.MinPeers
		nodeConfig["max_peers"] = node.Connectivity.MaxPeers

		// 节点发现配置
		if node.Discovery.MDNS.Enabled {
			nodeConfig["mdns_enabled"] = true
			nodeConfig["mdns_service"] = node.Discovery.MDNS.ServiceName
		}

		if node.Discovery.DHT.Enabled {
			nodeConfig["dht_enabled"] = true
			nodeConfig["dht_mode"] = node.Discovery.DHT.Mode
		}

		configInfo["node"] = nodeConfig
	}

	return configInfo
}

// RenderStatusBar 渲染状态栏
func (sm *StatusManager) RenderStatusBar() string {
	status := sm.GetStatus()

	// 截断节点ID显示，只显示前8位和后4位
	nodeIDDisplay := truncateNodeID(status.NodeID)

	// 创建简洁的状态栏
	statusItems := []string{
		fmt.Sprintf("WES %s", status.Version),
		fmt.Sprintf("节点: %s", nodeIDDisplay),
		fmt.Sprintf("区块: %d", status.BlockHeight),
		fmt.Sprintf("连接: %d", status.ConnectedPeers),
	}

	// 挖矿状态 - 不显示哈希率（遵循项目约束）
	miningStatus := "已停止"
	if status.IsMining {
		miningStatus = "运行中"
	}
	statusItems = append(statusItems, fmt.Sprintf("挖矿: %s", miningStatus))

	// 使用简单的分隔符连接
	return fmt.Sprintf("━━━ %s ━━━",
		pterm.Gray(strings.Join(statusItems, " | ")))
}

// truncateNodeID 截断节点ID显示
func truncateNodeID(nodeID string) string {
	if nodeID == "" || nodeID == "N/A" {
		return nodeID
	}

	// 如果节点ID长度超过20个字符，截断显示
	if len(nodeID) > 20 {
		// 显示前8位...后4位格式
		return fmt.Sprintf("%s...%s", nodeID[:8], nodeID[len(nodeID)-4:])
	}

	return nodeID
}

// RenderDetailedStatus 渲染详细状态（用于状态页面）
func (sm *StatusManager) RenderDetailedStatus() {
	status := sm.GetStatus()

	pterm.DefaultSection.Println("系统状态")

	// 系统信息
	systemData := [][]string{
		{"版本", status.Version},
		{"环境", status.Environment},
		{"节点ID", status.NodeID},
		{"更新时间", status.LastUpdated.Format("15:04:05")},
	}

	// 区块链信息
	blockchainData := [][]string{
		{"链ID", fmt.Sprintf("%d", status.ChainID)},
		{"区块高度", fmt.Sprintf("%d", status.BlockHeight)},
		{"最后出块", status.LastBlockTime},
	}

	// 网络信息
	networkData := [][]string{
		{"连接节点", fmt.Sprintf("%d", status.ConnectedPeers)},
		{"网络延迟", status.NetworkDelay},
	}

	// 挖矿信息 - 遵循项目约束，不显示指标
	miningData := [][]string{
		{"挖矿状态", func() string {
			if status.IsMining {
				return "运行中"
			}
			return "已停止"
		}()},
		{"算力", "N/A"},   // 根据项目约束，公共接口不暴露指标
		{"挖矿区块", "N/A"}, // 同样不显示
	}

	// 显示表格
	pterm.Println("系统信息:")
	pterm.DefaultTable.WithHasHeader(false).WithData(systemData).Render()
	pterm.Println()

	pterm.Println("区块链状态:")
	pterm.DefaultTable.WithHasHeader(false).WithData(blockchainData).Render()
	pterm.Println()

	pterm.Println("网络状态:")
	pterm.DefaultTable.WithHasHeader(false).WithData(networkData).Render()
	pterm.Println()

	pterm.Println("挖矿状态:")
	pterm.DefaultTable.WithHasHeader(false).WithData(miningData).Render()
}
