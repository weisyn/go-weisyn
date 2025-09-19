package commands

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/ui"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	repositoryintf "github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"
)

// BlockchainCommands 区块链查询命令处理器
type BlockchainCommands struct {
	logger            log.Logger
	apiClient         *client.Client
	ui                ui.Components
	chainService      blockchainintf.ChainService      // 🔗 链状态服务
	blockService      blockchainintf.BlockService      // 📦 区块服务
	repositoryManager repositoryintf.RepositoryManager // 📊 数据仓储服务
}

// NewBlockchainCommands 创建区块链命令处理器
func NewBlockchainCommands(
	logger log.Logger,
	apiClient *client.Client,
	ui ui.Components,
	chainService blockchainintf.ChainService,
	blockService blockchainintf.BlockService,
	repositoryManager repositoryintf.RepositoryManager,
) *BlockchainCommands {
	return &BlockchainCommands{
		logger:            logger,
		apiClient:         apiClient,
		ui:                ui,
		chainService:      chainService,
		blockService:      blockService,
		repositoryManager: repositoryManager,
	}
}

// ShowLatestBlocks 显示最新区块 - 使用分屏模式
func (b *BlockchainCommands) ShowLatestBlocks(ctx context.Context) error {
	// 加载页：仅显示加载进度
	progress := ui.StartSpinner("正在检查区块链状态...")

	// 🚀 步骤1: 先从ChainService获取链状态信息（包含最新区块高度）
	chainInfo, err := b.chainService.GetChainInfo(ctx)
	if err != nil {
		progress.Stop()
		// 结果页：切换到错误显示页面
		ui.SwitchToResultPage("📊 最新区块信息")
		ui.ShowNetworkErrorState("获取链状态信息", err.Error())
		b.waitForContinue()
		return nil
	}

	// 🚀 步骤2: 使用RepositoryManager按高度获取具体区块数据
	coreBlock, err := b.repositoryManager.GetBlockByHeight(ctx, chainInfo.Height)
	if err != nil {
		progress.Stop()
		// 结果页：切换到错误显示页面
		ui.SwitchToResultPage("📊 最新区块信息")
		ui.ShowNetworkErrorState(fmt.Sprintf("获取高度 %d 的区块", chainInfo.Height), err.Error())
		b.waitForContinue()
		return nil
	}

	progress.Stop()

	// 结果页：切换到结果显示页面
	ui.SwitchToResultPage("📊 最新区块信息")

	// 🚀 步骤3: 转换为BlockInfo格式用于显示
	blockInfo := client.NewBlockInfoFromProto(coreBlock)

	// 显示区块信息
	b.showBlockInfo(blockInfo)
	b.waitForContinue()
	return nil
}

// ShowBlockByHeight 根据高度查询区块信息页
func (b *BlockchainCommands) ShowBlockByHeight(ctx context.Context) error {
	pterm.DefaultSection.Println("按高度查询区块")

	pterm.DefaultBox.WithTitle("🔍 区块查询说明").Println(
		"按高度查询区块功能:\n\n" +
			"• 📊 区块详情: 根据区块高度获取完整区块信息\n" +
			"• 💸 交易列表: 查看区块中包含的所有交易\n" +
			"• ⏰ 时间信息: 区块创建时间和确认信息\n" +
			"• 🔗 哈希验证: 区块哈希和前一区块关联\n\n" +
			"📋 当前可用的查询方式:\n" +
			"   - API接口: GET /blocks/{height}\n" +
			"   - 区块浏览器: 通过区块链浏览器查看\n" +
			"   - 日志记录: 查看节点处理的区块日志\n\n" +
			"💡 提示: CLI图形化区块查询功能正在开发中",
	)

	b.waitForContinue()
	return nil
}

// ShowTransaction 显示交易详情信息页
func (b *BlockchainCommands) ShowTransaction(ctx context.Context) error {
	pterm.DefaultSection.Println("交易详情查询")

	pterm.DefaultBox.WithTitle("🔍 交易查询说明").Println(
		"交易详情查询功能:\n\n" +
			"• 💸 基本信息: 发送方、接收方、金额、手续费\n" +
			"• 📊 确认状态: 交易确认数和包含区块信息\n" +
			"• 🔗 UTXO追踪: 输入输出的UTXO引用关系\n" +
			"• ⏰ 时间记录: 交易创建、确认时间戳\n" +
			"• 🔐 签名验证: 交易签名和验证状态\n\n" +
			"📋 交易查询方式:\n" +
			"   - API接口: GET /transactions/{txhash}\n" +
			"   - 交易哈希: 通过完整交易哈希查询\n" +
			"   - 账户记录: 查看特定地址的交易历史\n\n" +
			"💡 提示: CLI交互式交易查询功能正在开发中",
	)

	b.waitForContinue()
	return nil
}

// ShowChainInfo 显示链状态信息页 - 基于真实接口实现
func (b *BlockchainCommands) ShowChainInfo(ctx context.Context) error {
	// 统一页面头部显示
	ui.ShowPageHeader()

	pterm.DefaultSection.Println("📊 区块链状态信息")
	pterm.Println()

	// 显示加载进度
	progress := ui.StartSpinner("正在获取链状态信息...")

	// 🔗 步骤1：获取链基础信息
	chainInfo, err := b.chainService.GetChainInfo(ctx)
	if err != nil {
		progress.Stop()
		b.ui.ShowError(ui.StandardErrorFormat("获取链状态", "链服务调用失败", err))
		b.waitForContinue()
		return nil
	}

	// 🔗 步骤2：检查系统就绪状态
	isReady, err := b.chainService.IsReady(ctx)
	if err != nil {
		progress.Stop()
		b.ui.ShowError(ui.StandardErrorFormat("检查系统状态", "系统就绪状态检查失败", err))
		b.waitForContinue()
		return nil
	}

	// 🔗 步骤3：检查数据新鲜度
	dataFresh, err := b.chainService.IsDataFresh(ctx)
	if err != nil {
		progress.Stop()
		b.ui.ShowError(ui.StandardErrorFormat("检查数据新鲜度", "数据新鲜度检查失败", err))
		b.waitForContinue()
		return nil
	}

	// 🔗 步骤4：获取节点模式
	nodeMode, err := b.chainService.GetNodeMode(ctx)
	if err != nil {
		progress.Stop()
		b.ui.ShowError(ui.StandardErrorFormat("获取节点模式", "节点模式查询失败", err))
		b.waitForContinue()
		return nil
	}

	progress.Stop()

	// 💎 显示链状态信息（基于真实数据）
	b.showRealChainStatus(chainInfo, isReady, dataFresh, nodeMode)

	b.waitForContinue()
	return nil
}

// showRealChainStatus 显示真实的链状态信息
func (b *BlockchainCommands) showRealChainStatus(chainInfo *types.ChainInfo, isReady, dataFresh bool, nodeMode types.NodeMode) {
	// 链基础状态
	chainData := [][]string{
		{"当前区块高度", fmt.Sprintf("%d", chainInfo.Height)},
		{"最佳区块哈希", fmt.Sprintf("%.16s...", chainInfo.BestBlockHash)},
		{"链状态", b.formatChainStatus(chainInfo.Status)},
		{"节点模式", string(nodeMode)},
		{"系统就绪", b.formatReadyStatus(isReady)},
		{"数据新鲜度", b.formatDataFreshness(dataFresh)},
	}

	pterm.DefaultBox.WithTitle("🔗 区块链状态").WithTitleTopCenter().Println("")
	pterm.DefaultTable.
		WithHasHeader().
		WithData(append([][]string{{"状态项", "当前值"}}, chainData...)).
		Render()
	pterm.Println()

	// 节点能力说明
	pterm.DefaultBox.WithTitle("📋 本节点信息说明").Println(
		"✅ 真实节点状态:\n\n" +
			"• 🔗 区块高度: 本节点当前同步的区块高度\n" +
			"• 🔐 区块哈希: 当前最佳区块的哈希值\n" +
			"• 🔄 同步状态: 节点是否与网络保持同步\n" +
			"• ⚙️ 节点模式: 全节点/轻节点运行模式\n" +
			"• 🟢 系统就绪: 所有组件是否正常运行\n" +
			"• ⚡ 数据新鲜: 节点数据是否为最新状态\n\n" +
			"💡 说明: 这些是去中心化节点的真实状态，无需中心化统计",
	)
}

// formatChainStatus 格式化链状态
func (b *BlockchainCommands) formatChainStatus(status string) string {
	switch status {
	case "normal":
		return "🟢 正常运行"
	case "syncing":
		return "🟡 同步中"
	case "fork_processing":
		return "🔄 处理分叉"
	case "error":
		return "🔴 系统错误"
	case "maintenance":
		return "🔧 维护状态"
	default:
		return "❓ 未知状态"
	}
}

// formatReadyStatus 格式化就绪状态
func (b *BlockchainCommands) formatReadyStatus(isReady bool) string {
	if isReady {
		return "✅ 系统就绪"
	}
	return "⚠️ 启动中"
}

// formatDataFreshness 格式化数据新鲜度
func (b *BlockchainCommands) formatDataFreshness(dataFresh bool) string {
	if dataFresh {
		return "⚡ 数据最新"
	}
	return "🔄 更新中"
}

// showBlockInfo 显示区块详细信息
func (b *BlockchainCommands) showBlockInfo(block *client.BlockInfo) {
	// 创建区块信息表格 - 显示完整哈希值
	blockData := [][]string{
		{"链ID", fmt.Sprintf("%d", block.GetChainID())},
		{"版本", fmt.Sprintf("%d", block.GetVersion())},
		{"区块高度", fmt.Sprintf("%d", block.GetHeight())},
		{"前一区块哈希", block.GetPreviousHashHex()},
		{"时间戳", block.GetFormattedTime()},
		{"交易数量", fmt.Sprintf("%d", block.GetTxCount())},
		{"难度", fmt.Sprintf("%d", block.GetDifficulty())},
		{"随机数", block.GetNonceHex()},
		{"Merkle根", block.GetMerkleRootHex()},
	}

	pterm.DefaultTable.
		WithHasHeader().
		WithData(append([][]string{{"属性", "值"}}, blockData...)).
		Render()
}

// truncateHash 截断哈希显示 (已弃用 - 现在显示完整哈希值)
// 保留函数以避免编译错误，但不再使用于重要标识符
func truncateHash(hash string, maxLen int) string {
	if len(hash) <= maxLen {
		return hash
	}
	return hash[:maxLen-3] + "..."
}

// ShowBlockchainMenu 显示区块链菜单
func (b *BlockchainCommands) ShowBlockchainMenu(ctx context.Context) error {
	options := []string{
		"查看最新区块",
		"按高度查询区块",
		"查询交易信息",
		"链状态信息",
		"返回主菜单",
	}

	selectedIndex, err := b.ui.ShowMenu("区块信息", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return b.ShowLatestBlocks(ctx)
	case 1:
		return b.ShowBlockByHeight(ctx)
	case 2:
		return b.ShowTransaction(ctx)
	case 3:
		return b.ShowChainInfo(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// waitForContinue 等待用户按任意键继续
func (b *BlockchainCommands) waitForContinue() {
	pterm.Println()
	ui.ShowStandardWaitPrompt("continue")
}
