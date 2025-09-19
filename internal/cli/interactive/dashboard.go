package interactive

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/status"
	"github.com/weisyn/v1/internal/cli/ui"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// Protobuf导入
	blockpb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Dashboard 实时仪表盘
type Dashboard struct {
	logger         log.Logger
	apiClient      *client.Client
	uiComponents   ui.Components
	layout         *ui.DashboardLayout
	chainService   blockchainintf.ChainService   // 🔗 链状态服务
	accountService blockchainintf.AccountService // 📊 账户服务
	minerService   consensusintf.MinerService    // ⛏️ 挖矿服务
	configProvider config.Provider               // ⚙️ 配置提供者
	statusManager  *status.StatusManager         // 📊 状态管理器
	isRunning      bool
}

// NewDashboard 创建新的仪表盘
func NewDashboard(
	logger log.Logger,
	apiClient *client.Client,
	uiComponents ui.Components,
	chainService blockchainintf.ChainService,
	accountService blockchainintf.AccountService,
	minerService consensusintf.MinerService,
	configProvider config.Provider,
	statusManager *status.StatusManager,
) *Dashboard {
	return &Dashboard{
		logger:         logger,
		apiClient:      apiClient,
		uiComponents:   uiComponents,
		layout:         ui.NewDashboardLayout(logger, apiClient, chainService, accountService, minerService, configProvider, statusManager),
		chainService:   chainService,
		accountService: accountService,
		minerService:   minerService,
		configProvider: configProvider,
		statusManager:  statusManager,
		isRunning:      false,
	}
}

// Start 启动仪表盘
func (d *Dashboard) Start(ctx context.Context) error {
	d.isRunning = true

	// 启动新的表格布局仪表盘
	if err := d.layout.Start(ctx); err != nil {
		d.logger.Errorf("启动仪表盘布局失败: %v", err)
		return err
	}

	// 显示初始状态
	if err := d.showInitialStatus(ctx); err != nil {
		d.logger.Errorf("显示初始状态失败: %v", err)
		// 不返回错误，继续运行
	}

	return nil
}

// Stop 停止仪表盘
func (d *Dashboard) Stop() {
	d.isRunning = false
	if d.layout != nil {
		d.layout.Stop()
	}
}

// AddLogEntry 添加日志条目到仪表盘
func (d *Dashboard) AddLogEntry(level, message string) {
	if d.layout != nil {
		d.layout.AddLogEntry(level, message)
	}
}

// SetSelectedMenu 设置选中的菜单项
func (d *Dashboard) SetSelectedMenu(index int) {
	if d.layout != nil {
		d.layout.SetSelectedMenu(index)
	}
}

// GetSelectedMenu 获取当前选中的菜单索引
func (d *Dashboard) GetSelectedMenu() int {
	if d.layout != nil {
		return d.layout.GetSelectedMenu()
	}
	return 0
}

// showInitialStatus 显示初始状态信息
func (d *Dashboard) showInitialStatus(ctx context.Context) error {
	pterm.DefaultSection.Println("系统状态检查")

	// 创建进度条
	progress := pterm.DefaultProgressbar.WithTotal(4).WithTitle("正在加载系统信息...")
	progress, _ = progress.Start()

	var nodeInfo *client.NodeInfo
	var miningStatus *client.MiningStatus
	var blockInfo *client.BlockInfo

	// 🚀 使用真实API和服务调用获取数据

	// 获取节点信息 - 使用API调用
	progress.UpdateTitle("获取节点信息...")
	if info, err := d.apiClient.GetNodeInfo(ctx); err == nil {
		d.logger.Infof("✅ 获取到节点信息: ID=%s", info.NodeID)
		nodeInfo = info
	} else {
		d.logger.Warnf("❌ 获取节点信息失败: %v", err)
	}
	progress.Increment()

	// 获取挖矿状态 - 使用挖矿服务
	progress.UpdateTitle("获取挖矿状态...")
	if isRunning, minerAddr, err := d.minerService.GetMiningStatus(ctx); err == nil {
		d.logger.Infof("✅ 获取到挖矿状态: 运行=%t", isRunning)
		miningStatus = &client.MiningStatus{
			IsMining:     isRunning,
			MinerAddress: string(minerAddr),
			IsActive:     isRunning,
			HashRate:     0,     // 不显示算力指标（遵循项目约束）
			BlocksMined:  0,     // 不显示挖矿区块数
			Difficulty:   "N/A", // 难度信息需要从其他源获取
			Uptime:       0,
		}
	} else {
		d.logger.Warnf("❌ 获取挖矿状态失败: %v", err)
	}
	progress.Increment()

	// 🚀 获取最新区块 - 使用API调用
	progress.UpdateTitle("获取区块信息...")
	if latestBlock, err := d.apiClient.GetLatestBlock(ctx); err == nil {
		d.logger.Infof("✅ 获取到最新区块: 高度=%d", latestBlock.GetHeight())
		blockInfo = latestBlock
	} else {
		d.logger.Warnf("❌ 获取最新区块失败，使用链信息作为备用: %v", err)
		// 备用方案：从链服务获取基本信息
		if chainInfo, chainErr := d.chainService.GetChainInfo(ctx); chainErr == nil {
			blockInfo = &client.BlockInfo{
				Block: &blockpb.Block{
					Header: &blockpb.BlockHeader{
						ChainId:   d.configProvider.GetBlockchain().ChainID, // 使用配置的链ID
						Version:   1,
						Height:    chainInfo.Height,
						Timestamp: uint64(chainInfo.LastBlockTime),
					},
					Body: &blockpb.BlockBody{
						Transactions: []*transaction.Transaction{}, // 空交易列表
					},
				},
			}
		} else {
			d.logger.Errorf("获取区块信息失败: API=%v, 链服务=%v", err, chainErr)
		}
	}
	progress.Increment()

	// 完成加载
	progress.UpdateTitle("加载完成!")
	progress.Increment()

	time.Sleep(500 * time.Millisecond) // 让用户看到完成状态
	progress.Stop()

	// 显示状态概览
	d.showStatusOverview(nodeInfo, miningStatus, blockInfo)

	return nil
}

// showStatusOverview 显示状态概览
func (d *Dashboard) showStatusOverview(nodeInfo *client.NodeInfo, miningStatus *client.MiningStatus, blockInfo *client.BlockInfo) {
	pterm.Println() // 空行
	pterm.DefaultSection.Println("节点概览")

	// 创建状态面板
	var panels pterm.Panels

	// 节点信息面板
	if nodeInfo != nil {
		nodePanel := fmt.Sprintf(
			"🌐 节点状态: %s\n"+
				"📊 区块高度: %s\n"+
				"🔗 连接节点: %s\n"+
				"⏱️ 运行时间: %s\n"+
				"📱 版本: %s",
			pterm.Green("运行中"),
			pterm.Yellow(fmt.Sprintf("%d", nodeInfo.BlockHeight)),
			pterm.Blue(fmt.Sprintf("%d", nodeInfo.PeerCount)),
			formatUptime(nodeInfo.Uptime),
			nodeInfo.Version,
		)
		panels = append(panels, []pterm.Panel{{Data: nodePanel}})
	}

	// 挖矿信息面板
	if miningStatus != nil {
		miningPanel := fmt.Sprintf(
			"⛏️ 挖矿状态: %s\n"+
				"📈 算力: %s\n"+
				"🏆 已挖区块: %s\n"+
				"🎯 当前难度: %s\n"+
				"⏰ 运行时长: %s",
			getMiningStatusText(miningStatus.IsActive),
			pterm.Gray("N/A"),       // 根据项目约束，不显示算力指标
			pterm.Gray("N/A"),       // 同样不显示挖矿区块数
			miningStatus.Difficulty, // 显示完整难度值
			formatUptime(miningStatus.Uptime),
		)
		panels = append(panels, []pterm.Panel{{Data: miningPanel}})
	}

	// 区块信息面板
	if blockInfo != nil {
		blockPanel := fmt.Sprintf(
			"🧱 最新区块: %s\n"+
				"📏 区块高度: %s\n"+
				"📝 交易数量: %s\n"+
				"👤 出块者: %s\n"+
				"🕐 时间: %s",
			"N/A", // Hash需要单独计算 - 暂不可用
			pterm.Yellow(fmt.Sprintf("%d", blockInfo.GetHeight())),
			pterm.Blue(fmt.Sprintf("%d", blockInfo.GetTxCount())),
			"N/A", // Miner信息不在Block结构中 - 暂不可用
			blockInfo.GetFormattedTime(),
		)
		panels = append(panels, []pterm.Panel{{Data: blockPanel}})
	}

	// 显示面板
	if len(panels) > 0 {
		pterm.DefaultPanel.WithPanels(panels).Render()
	}

	// 显示分隔线
	pterm.Println()
	pterm.Println(strings.Repeat("─", 50))
}

// backgroundUpdate 后台更新（可选功能）
func (d *Dashboard) backgroundUpdate(ctx context.Context) {
	// 这里可以实现后台定时更新
	// 比如每30秒更新一次状态信息
	// 但要注意不要干扰用户的交互
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !d.isRunning {
				return
			}
			// 这里可以添加后台更新逻辑
			// 但需要谨慎处理，避免干扰用户操作
		}
	}
}

// ShowLiveStatus 显示实时状态（用于监控模式）
func (d *Dashboard) ShowLiveStatus(ctx context.Context) error {
	pterm.Print("\033[2J\033[H")
	pterm.DefaultHeader.WithFullWidth().Println("实时监控模式 - 按 Ctrl+C 退出")

	// 创建实时更新器
	liveArea, _ := pterm.DefaultArea.WithFullscreen().Start()
	defer liveArea.Stop()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// 获取最新状态
			nodeInfo, _ := d.apiClient.GetNodeInfo(ctx)
			miningStatus, _ := d.apiClient.GetMiningStatus(ctx)
			blockInfo, _ := d.apiClient.GetLatestBlock(ctx)

			// 构建状态显示内容
			content := d.buildLiveStatusContent(nodeInfo, miningStatus, blockInfo)

			// 更新显示
			liveArea.Update(content)
		}
	}
}

// buildLiveStatusContent 构建实时状态内容
func (d *Dashboard) buildLiveStatusContent(nodeInfo *client.NodeInfo, miningStatus *client.MiningStatus, blockInfo *client.BlockInfo) string {
	var content string

	// 标题
	content += pterm.DefaultHeader.WithFullWidth().Sprint("🚀 WES 节点实时监控")
	content += "\n\n"

	// 时间戳
	content += pterm.Gray(fmt.Sprintf("更新时间: %s", time.Now().Format("2006-01-02 15:04:05")))
	content += "\n\n"

	// 节点状态
	if nodeInfo != nil {
		content += pterm.DefaultSection.Sprint("节点状态")
		content += fmt.Sprintf("区块高度: %s | 连接节点: %s | 运行时间: %s\n",
			pterm.Yellow(fmt.Sprintf("%d", nodeInfo.BlockHeight)),
			pterm.Blue(fmt.Sprintf("%d", nodeInfo.PeerCount)),
			formatUptime(nodeInfo.Uptime),
		)
		content += "\n"
	}

	// 挖矿状态
	if miningStatus != nil {
		content += pterm.DefaultSection.Sprint("挖矿状态")
		content += fmt.Sprintf("状态: %s | 算力: %s | 已挖区块: %s\n",
			getMiningStatusText(miningStatus.IsActive),
			pterm.Gray("N/A"), // 根据项目约束，不显示算力指标
			pterm.Gray("N/A"), // 同样不显示挖矿区块数
		)
		content += "\n"
	}

	// 最新区块
	if blockInfo != nil {
		content += pterm.DefaultSection.Sprint("最新区块")
		content += fmt.Sprintf("高度: %s | 哈希: %s | 交易: %s | 时间: %s\n",
			pterm.Yellow(fmt.Sprintf("%d", blockInfo.GetHeight())),
			"N/A", // Hash需要单独计算 - 暂不可用
			pterm.Blue(fmt.Sprintf("%d", blockInfo.GetTxCount())),
			blockInfo.GetFormattedTime(),
		)
	}

	return content
}

// 辅助函数

// formatUptime 格式化运行时间
func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%d分", seconds/60)
	} else if seconds < 86400 {
		return fmt.Sprintf("%d小时", seconds/3600)
	} else {
		return fmt.Sprintf("%d天", seconds/86400)
	}
}

// getMiningStatusText 获取挖矿状态文本
func getMiningStatusText(isActive bool) string {
	if isActive {
		return pterm.Green("🟢 运行中")
	}
	return pterm.Red("🔴 已停止")
}

// truncateHash 截断哈希显示
func truncateHash(hash string, maxLen int) string {
	if len(hash) <= maxLen {
		return hash
	}
	return hash[:maxLen-3] + "..."
}
