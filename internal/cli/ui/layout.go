package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/status"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// LogEntry 日志条目
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

// LogBuffer 专用日志缓冲区，避免干扰主界面
type LogBuffer struct {
	entries []LogEntry
	maxSize int
	mutex   sync.RWMutex
}

// NewLogBuffer 创建日志缓冲区
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// AddEntry 添加日志条目
func (l *LogBuffer) AddEntry(level, message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[1:] // 保持固定大小
	}
}

// GetRecentEntries 获取最近的日志条目
func (l *LogBuffer) GetRecentEntries(count int) []string {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	start := len(l.entries) - count
	if start < 0 {
		start = 0
	}

	var lines []string
	for i := start; i < len(l.entries); i++ {
		entry := l.entries[i]
		line := fmt.Sprintf("[%s] %s %s",
			entry.Time.Format("15:04:05"),
			getLevelIcon(entry.Level),
			entry.Message)
		lines = append(lines, line)
	}
	return lines
}

// DashboardLayout 基于表格的仪表盘布局
type DashboardLayout struct {
	logger         log.Logger
	logBuffer      *LogBuffer
	apiClient      *client.Client
	chainService   blockchainintf.ChainService   // 🔗 链状态服务
	accountService blockchainintf.AccountService // 📊 账户服务
	minerService   consensusintf.MinerService    // ⛏️ 挖矿服务
	configProvider config.Provider               // ⚙️ 配置提供者
	statusManager  *status.StatusManager         // 📊 状态管理器

	// 状态数据
	currentMenu  int
	menuItems    []MenuItem
	nodeInfo     *client.NodeInfo
	miningStatus *client.MiningStatus
	balanceInfo  *client.BalanceInfo

	// 控制标志
	isRunning      bool
	updateInterval time.Duration
	mutex          sync.RWMutex
}

// MenuItem 菜单项
type MenuItem struct {
	Icon        string
	Title       string
	Description string
	IsSelected  bool
}

// NewDashboardLayout 创建新的仪表盘布局
func NewDashboardLayout(
	logger log.Logger,
	apiClient *client.Client,
	chainService blockchainintf.ChainService,
	accountService blockchainintf.AccountService,
	minerService consensusintf.MinerService,
	configProvider config.Provider,
	statusManager *status.StatusManager,
) *DashboardLayout {
	return &DashboardLayout{
		logger:         logger,
		logBuffer:      NewLogBuffer(50), // 保存最近50条日志
		apiClient:      apiClient,
		chainService:   chainService,
		accountService: accountService,
		minerService:   minerService,
		configProvider: configProvider,
		statusManager:  statusManager,
		updateInterval: 1 * time.Second,
		menuItems: []MenuItem{
			{Icon: "💰", Title: "账户管理", Description: "查看余额、创建账户、导入账户"},
			{Icon: "🔄", Title: "转账操作", Description: "简单转账、批量转账、时间锁转账"},
			{Icon: "📊", Title: "区块信息", Description: "查看区块、交易状态、链信息"},
			{Icon: "⛏️", Title: "挖矿控制", Description: "启动挖矿、停止挖矿、查看状态"},
			{Icon: "🌐", Title: "节点管理", Description: "节点信息、对等节点、网络状态"},
			{Icon: "📈", Title: "实时监控", Description: "系统监控、性能统计、日志查看"},
			{Icon: "⚙️", Title: "系统设置", Description: "查看当前配置信息（只读）"},
			{Icon: "🚪", Title: "退出程序", Description: ""},
		},
	}
}

// Start 启动仪表盘布局
func (d *DashboardLayout) Start(ctx context.Context) error {
	d.isRunning = true

	// 初始选中第一个菜单项
	d.menuItems[0].IsSelected = true

	// 进行初始数据更新和渲染
	d.updateData()
	d.render()

	// 启动定时刷新协程，实现动态数据更新
	go d.startUpdateLoop(ctx)

	return nil
}

// Stop 停止仪表盘布局
func (d *DashboardLayout) Stop() {
	d.isRunning = false
}

// startUpdateLoop 启动定时更新循环
func (d *DashboardLayout) startUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(d.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !d.isRunning {
				return
			}
			d.updateData()
			d.render()
		case <-ctx.Done():
			return
		}
	}
}

// ManualUpdate 手动更新数据和界面（供外部调用）
func (d *DashboardLayout) ManualUpdate() {
	if !d.isRunning {
		return
	}
	d.updateData()
	d.render()
}

// updateData 更新数据 - 使用StatusManager获取真实状态
func (d *DashboardLayout) updateData() {
	ctx := context.Background()

	// 🚀 从StatusManager获取系统状态
	if d.statusManager != nil {
		systemStatus := d.statusManager.GetStatus()
		if systemStatus != nil {
			// 使用真实的系统状态更新NodeInfo
			d.nodeInfo = &client.NodeInfo{
				NodeID:      systemStatus.NodeID,
				Version:     systemStatus.Version,
				BlockHeight: systemStatus.BlockHeight,
				PeerCount:   systemStatus.ConnectedPeers,
				Uptime:      0, // 可以根据需要从其他地方获取
			}

			// 使用真实的挖矿状态
			d.miningStatus = &client.MiningStatus{
				IsMining:    systemStatus.IsMining,
				IsActive:    systemStatus.IsMining,
				HashRate:    0, // 根据项目约束，不显示算力
				BlocksMined: 0, // 同样不显示挖矿区块数
				Difficulty:  "N/A",
			}

			if d.logger != nil {
				d.logger.Debugf("✅ 获取到系统状态: 高度=%d, 节点=%s, 挖矿=%t",
					systemStatus.BlockHeight, systemStatus.NodeID, systemStatus.IsMining)
			}
		}
	}

	// 备用：如果StatusManager不可用，从链服务直接获取
	if d.nodeInfo == nil {
		if chainInfo, err := d.chainService.GetChainInfo(ctx); err == nil {
			d.nodeInfo = &client.NodeInfo{
				NodeID:      "N/A",
				Version:     version.GetDisplayVersion(d.configProvider),
				BlockHeight: chainInfo.Height,
				PeerCount:   0,
				Uptime:      0,
			}
		} else {
			// 完全失败时使用默认值
			d.nodeInfo = &client.NodeInfo{
				NodeID:      "未连接",
				Version:     version.GetDisplayVersion(d.configProvider),
				BlockHeight: 0,
				PeerCount:   0,
				Uptime:      0,
			}
		}
	}

	// 备用：如果挖矿状态未设置
	if d.miningStatus == nil {
		if isRunning, _, err := d.minerService.GetMiningStatus(ctx); err == nil {
			d.miningStatus = &client.MiningStatus{
				IsMining:    isRunning,
				IsActive:    isRunning,
				HashRate:    0, // 不显示算力
				BlocksMined: 0, // 不显示挖矿区块数
				Difficulty:  "N/A",
			}
		} else {
			d.miningStatus = &client.MiningStatus{
				IsMining:    false,
				IsActive:    false,
				HashRate:    0,
				BlocksMined: 0,
				Difficulty:  "N/A",
			}
		}
	}

	// 📊 余额信息：不再显示硬编码的默认地址余额，提示用户在账户菜单查看
	// 避免误导用户以为这是他们的实际余额
	d.balanceInfo = &client.BalanceInfo{
		Address: struct {
			RawHash string `json:"raw_hash"`
		}{RawHash: "请在「💰账户管理」菜单中查看真实余额"},
		TokenID:   nil,
		Available: 0,
		Total:     0,
	}
}

// renderOnce 渲染一次界面（静态显示）
func (d *DashboardLayout) renderOnce() {
	d.updateData()
	d.render()
}

// render 渲染整个布局
func (d *DashboardLayout) render() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 创建主布局表格数据
	tableData := [][]string{
		// 第1行: 标题栏
		{d.getHeaderContent()},

		// 第2行: 主要内容区域 (左侧菜单 | 右侧内容)
		{d.getMainContent()},

		// 第3行: 日志区域
		{d.getLogContent()},
	}

	// 创建表格并渲染
	table := pterm.DefaultTable.
		WithHasHeader(false).
		WithBoxed(true).
		WithData(tableData)

	// 完全清屏后再渲染，避免叠加
	pterm.Print("\033[2J\033[H") // 完全清屏并移动光标到左上角
	table.Render()
}

// getHeaderContent 获取标题栏内容
func (d *DashboardLayout) getHeaderContent() string {
	nodeID := "未连接"
	peerCount := 0
	version := version.GetDisplayVersion(nil)
	environment := "N/A"

	// 从StatusManager获取真实状态
	if d.statusManager != nil {
		systemStatus := d.statusManager.GetStatus()
		if systemStatus != nil {
			nodeID = truncateString(systemStatus.NodeID, 15)
			peerCount = systemStatus.ConnectedPeers
			version = systemStatus.Version
			environment = systemStatus.Environment
		}
	} else if d.nodeInfo != nil {
		// 备用数据源
		nodeID = truncateString(d.nodeInfo.NodeID, 15)
		peerCount = d.nodeInfo.PeerCount
		version = d.nodeInfo.Version
	}

	return fmt.Sprintf("             🌟 WES %s | %s | 节点: %s | ⚡ 已连接%d个节点",
		version, environment, nodeID, peerCount)
}

// getMainContent 获取主要内容区域
func (d *DashboardLayout) getMainContent() string {
	// 创建左右分栏的内容
	leftContent := d.getMenuContent()
	rightContent := d.getContentAreaAndStatus()

	// 使用两列子表格
	subTable := pterm.DefaultTable.
		WithHasHeader(false).
		WithBoxed(false).
		WithData([][]string{
			{leftContent, rightContent},
		})

	// Srender返回两个值，只取第一个
	content, _ := subTable.Srender()
	return content
}

// getMenuContent 获取左侧菜单内容
func (d *DashboardLayout) getMenuContent() string {
	lines := []string{
		"   🎯 功能菜单",
		"",
	}

	for _, item := range d.menuItems {
		prefix := "  "
		if item.IsSelected {
			prefix = "► "
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", prefix, item.Icon, item.Title))
	}

	// 添加一些空行来增加高度
	for i := 0; i < 5; i++ {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// getContentAreaAndStatus 获取右侧内容区域和状态信息
func (d *DashboardLayout) getContentAreaAndStatus() string {
	// 上半部分: 主操作区域
	mainArea := []string{
		"                📊 主操作区域",
		"",
		"        [当前选中功能的具体内容]",
		"",
		"        • 表格数据",
		"        • 表单输入",
		"        • 操作按钮",
		"        • 状态显示",
		"",
	}

	// 下半部分: 快速状态信息
	statusInfo := d.getQuickStatusInfo()

	allLines := append(mainArea, statusInfo...)
	return strings.Join(allLines, "\n")
}

// getQuickStatusInfo 获取快速状态信息
func (d *DashboardLayout) getQuickStatusInfo() []string {
	if d.nodeInfo == nil || d.miningStatus == nil || d.balanceInfo == nil {
		return []string{"", "              🔍 正在加载状态信息..."}
	}

	// 获取网络延迟状态
	networkDelay := "N/A" // 默认显示N/A，未实现ping/RTT时
	if d.statusManager != nil {
		systemStatus := d.statusManager.GetStatus()
		if systemStatus != nil {
			networkDelay = systemStatus.NetworkDelay
		}
	}

	return []string{
		"              🔍 快速状态栏",
		"",
		fmt.Sprintf("   ⛏️ 挖矿: %s    💰 余额: 查看账户菜单    🌐 节点: %d",
			d.getMiningStatusText(), d.nodeInfo.PeerCount),
		fmt.Sprintf("   📊 区块: %d      ⚡ 算力: N/A        🕐 延迟: %s",
			d.nodeInfo.BlockHeight, networkDelay),
	}
}

// getLogContent 获取日志区域内容
func (d *DashboardLayout) getLogContent() string {
	entries := d.logBuffer.GetRecentEntries(3) // 只显示最近3条

	if len(entries) == 0 {
		return "  📜 系统日志 (最近消息)                                                        \n  暂无日志消息..."
	}

	lines := []string{"  📜 系统日志 (最近消息)"}
	for _, entry := range entries {
		lines = append(lines, "  "+entry)
	}

	return strings.Join(lines, "\n")
}

// getMiningStatusText 获取挖矿状态文本
func (d *DashboardLayout) getMiningStatusText() string {
	if d.miningStatus != nil && d.miningStatus.IsActive {
		return "活跃"
	}
	return "停止"
}

// AddLogEntry 添加日志条目（公共接口）
func (d *DashboardLayout) AddLogEntry(level, message string) {
	d.logBuffer.AddEntry(level, message)
}

// SetSelectedMenu 设置选中的菜单项
func (d *DashboardLayout) SetSelectedMenu(index int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if index >= 0 && index < len(d.menuItems) {
		// 清除所有选中状态
		for i := range d.menuItems {
			d.menuItems[i].IsSelected = false
		}
		// 设置新的选中项
		d.menuItems[index].IsSelected = true
		d.currentMenu = index
	}
}

// GetSelectedMenu 获取当前选中的菜单索引
func (d *DashboardLayout) GetSelectedMenu() int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.currentMenu
}

// 辅助函数
func getLevelIcon(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR":
		return "❌"
	case "WARN", "WARNING":
		return "⚠️"
	case "INFO":
		return "ℹ️"
	case "DEBUG":
		return "🔧"
	default:
		return "📝"
	}
}
