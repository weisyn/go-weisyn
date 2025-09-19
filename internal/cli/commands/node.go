package commands

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// NodeCommands 节点管理命令处理器
type NodeCommands struct {
	logger    log.Logger
	apiClient *client.Client
	ui        ui.Components
}

// NewNodeCommands 创建节点命令处理器
func NewNodeCommands(
	logger log.Logger,
	apiClient *client.Client,
	ui ui.Components,
) *NodeCommands {
	return &NodeCommands{
		logger:    logger,
		apiClient: apiClient,
		ui:        ui,
	}
}

// ShowNodeMenu 显示节点管理菜单 - 统一子菜单入口
func (n *NodeCommands) ShowNodeMenu(ctx context.Context) error {
	for {
		// 统一页面头部显示
		ui.ShowPageHeader()

		pterm.DefaultSection.Println("🌐 节点管理")
		pterm.Println()

		// 显示菜单选项
		options := []string{
			"查看节点状态",
			"查看连接的节点",
			"网络连接统计",
			"节点诊断信息",
			"返回主菜单",
		}

		selectedIndex, err := n.ui.ShowMenu("请选择节点操作:", options)
		if err != nil {
			n.logger.Errorf("菜单选择失败: %v", err)
			n.ui.ShowError(fmt.Sprintf("菜单操作失败: %v", err))
			n.waitForContinue()
			continue
		}

		switch selectedIndex {
		case 0: // 查看节点状态
			if err := n.ShowStatus(ctx); err != nil {
				n.logger.Errorf("查看节点状态失败: %v", err)
				n.ui.ShowError(fmt.Sprintf("查看节点状态失败: %v", err))
				n.waitForContinue()
			}
		case 1: // 查看连接的节点
			if err := n.ShowPeers(ctx); err != nil {
				n.logger.Errorf("查看连接节点失败: %v", err)
				n.ui.ShowError(fmt.Sprintf("查看连接节点失败: %v", err))
				n.waitForContinue()
			}
		case 2: // 网络连接统计
			if err := n.ShowNetworkStatus(ctx); err != nil {
				n.logger.Errorf("查看网络状态失败: %v", err)
				n.ui.ShowError(fmt.Sprintf("查看网络状态失败: %v", err))
				n.waitForContinue()
			}
		case 3: // 节点诊断信息
			if err := n.ShowSyncStatus(ctx); err != nil {
				n.logger.Errorf("查看同步状态失败: %v", err)
				n.ui.ShowError(fmt.Sprintf("查看同步状态失败: %v", err))
				n.waitForContinue()
			}
		case 4: // 返回主菜单
			return nil
		default:
			n.ui.ShowWarning("无效的选择，请重新选择")
			n.waitForContinue()
			continue
		}
	}
}

// ShowStatus 显示节点状态
func (n *NodeCommands) ShowStatus(ctx context.Context) error {
	// 统一页面头部显示
	ui.ShowPageHeader()

	pterm.DefaultSection.Println("节点状态")

	// 显示加载进度
	progress := ui.StartSpinner("正在获取节点信息...")

	// 查询节点信息
	nodeInfo, err := n.apiClient.GetNodeInfo(ctx)
	if err != nil {
		progress.Stop()
		n.ui.ShowError(fmt.Sprintf("查询失败 - 无法获取节点信息: %v", err))
		n.waitForContinue()
		return nil
	}

	// 查询共识参与状态
	progress.UpdateMessage("正在获取共识参与状态...")
	miningStatus, err := n.apiClient.GetMiningStatus(ctx)
	progress.Stop()

	if err != nil {
		n.ui.ShowError(fmt.Sprintf("查询失败 - 无法获取共识参与状态: %v", err))
		n.waitForContinue()
		return nil
	}

	// 显示综合状态
	n.ui.ShowNodeStatus(nodeInfo, miningStatus)
	n.waitForContinue()
	return nil
}

// ShowPeers 显示连接的节点列表 - 使用分屏模式
func (n *NodeCommands) ShowPeers(ctx context.Context) error {
	// 加载页：仅显示加载进度
	progress := ui.StartSpinner("正在检查网络连接状态...")

	// 查询连接的节点列表
	peers, err := n.apiClient.GetNodePeers(ctx)
	progress.Stop()

	// 结果页：切换到结果显示页面
	ui.SwitchToResultPage("🌐 连接节点列表")

	if err != nil {
		// 使用标准化网络错误状态
		ui.ShowNetworkErrorState("获取节点连接信息", err.Error())
		n.ui.ShowInfo("💡 提示：这可能是因为 /node/peers 端点尚未实现")
		n.waitForContinue()
		return nil
	}

	if len(peers) == 0 {
		// 使用标准化空状态
		ui.ShowDataNotFoundState("连接节点", "节点管理菜单")
		n.waitForContinue()
		return nil
	}

	// 显示节点连接统计
	pterm.DefaultBox.WithTitle("📊 连接统计").Println(
		fmt.Sprintf("总连接数: %d\n", len(peers)) +
			fmt.Sprintf("入站连接: %d\n", n.countInboundPeers(peers)) +
			fmt.Sprintf("出站连接: %d", n.countOutboundPeers(peers)),
	)

	pterm.Println()

	// 创建节点连接表格
	peerData := [][]string{
		{"节点ID", "地址", "方向", "协议", "延迟", "最后连接时间"},
	}

	for _, peer := range peers {
		peerData = append(peerData, []string{
			n.truncateString(peer.ID, 12),
			n.truncateString(peer.Address, 25),
			peer.Direction,
			peer.Protocol,
			peer.GetLatencyFormatted(),
			peer.LastSeen.Format("15:04:05"),
		})
	}

	// 显示表格
	pterm.DefaultTable.
		WithHasHeader().
		WithData(peerData).
		Render()

	n.waitForContinue()
	return nil
}

// countInboundPeers 统计入站连接数
func (n *NodeCommands) countInboundPeers(peers []client.PeerInfo) int {
	count := 0
	for _, peer := range peers {
		if peer.Direction == "inbound" {
			count++
		}
	}
	return count
}

// countOutboundPeers 统计出站连接数
func (n *NodeCommands) countOutboundPeers(peers []client.PeerInfo) int {
	count := 0
	for _, peer := range peers {
		if peer.Direction == "outbound" {
			count++
		}
	}
	return count
}

// truncateString 截断字符串
func (n *NodeCommands) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ShowNetworkStatus 显示网络状态信息页
func (n *NodeCommands) ShowNetworkStatus(ctx context.Context) error {
	// 统一页面头部显示
	ui.ShowPageHeader()

	pterm.DefaultSection.Println("网络状态信息")

	// 显示网络状态说明
	pterm.DefaultBox.WithTitle("📡 网络状态概览").Println(
		"网络状态监控功能说明:\n\n" +
			"• 📊 连接统计: 可通过「连接节点列表」查看\n" +
			"• 🌐 网络拓扑: 基于P2P发现协议动态组网\n" +
			"• ⚡ 通信协议: 使用libp2p进行节点间通信\n" +
			"• 🔄 数据同步: 通过区块链服务自动同步\n\n" +
			"💡 详细的网络监控数据可通过以下方式获取:\n" +
			"   - API接口: GET /node/info 查看节点信息\n" +
			"   - API接口: GET /node/peers 查看连接列表\n" +
			"   - 日志文件: 查看P2P连接和同步日志",
	)

	n.ui.ShowInfo("提示：网络状态实时监控正在规划中，当前可通过API接口获取相关数据")
	n.waitForContinue()
	return nil
}

// ShowSyncStatus 显示同步状态信息页
func (n *NodeCommands) ShowSyncStatus(ctx context.Context) error {
	// 统一页面头部显示
	ui.ShowPageHeader()

	pterm.DefaultSection.Println("区块链同步状态")

	// 获取当前区块高度作为同步状态参考
	progress := ui.StartSpinner("正在检查同步状态...")

	// 从节点信息获取基本状态
	nodeInfo, nodeErr := n.apiClient.GetNodeInfo(ctx)
	progress.Stop()

	if nodeErr != nil {
		n.ui.ShowError(fmt.Sprintf("获取节点信息失败: %v", nodeErr))
	} else {
		// 显示同步状态信息
		pterm.DefaultBox.WithTitle("🔄 区块链同步状态").Println(
			fmt.Sprintf("节点ID: %s\n", nodeInfo.NodeID) +
				fmt.Sprintf("连接节点数: %d\n", nodeInfo.GetPeerCount()) +
				fmt.Sprintf("协议支持: %v\n\n", nodeInfo.SupportedProtocols) +
				"💡 同步机制说明:\n" +
				"• 📦 区块同步: 自动从对等节点获取最新区块\n" +
				"• 🔗 交易同步: 实时接收和验证新交易\n" +
				"• ⚡ 状态同步: 维护最新的区块链状态\n" +
				"• 🌐 网络发现: 持续发现和连接新节点\n\n" +
				"📊 详细同步数据可通过以下方式查看:\n" +
				"   - 区块信息菜单: 查看当前区块高度\n" +
				"   - API接口: GET /blocks/latest 获取最新区块\n" +
				"   - 日志监控: 观察区块同步进度",
		)
	}

	// 显示同步状态说明
	pterm.DefaultBox.WithTitle("ℹ️  同步状态监控").Println(
		"当前同步状态基于以下指标判断:\n\n" +
			"• ✅ 节点连接正常 - 有对等节点连接\n" +
			"• ✅ 区块高度更新 - 持续接收新区块\n" +
			"• ✅ 交易处理正常 - 能够处理和验证交易\n\n" +
			"⚠️  如发现同步异常，请检查:\n" +
			"   - 网络连接是否稳定\n" +
			"   - 是否有足够的对等节点\n" +
			"   - 存储空间是否充足",
	)

	n.ui.ShowInfo("提示：高级同步监控和诊断功能正在开发中")
	n.waitForContinue()
	return nil
}

// waitForContinue 等待用户按任意键继续
func (n *NodeCommands) waitForContinue() {
	pterm.Println()
	ui.ShowStandardWaitPrompt("continue")
}
