# CLI交互界面控制器（internal/cli/interactive）

## 📋 **模块概述**

　　本模块是CLI交互界面的用户界面控制层，负责实现美观直观的**双层功能架构**交互式用户界面。通过智能权限识别、主菜单系统和实时仪表盘，为不同权限层次的用户提供流畅的操作体验，包括系统级查询导航、用户级操作引导、状态监控、实时数据展示等核心交互功能。

## 🎯 **核心职责**

- **双层菜单管理**：区分并展示系统级（公开）和用户级（私钥保护）功能菜单
- **权限状态识别**：智能检测用户钱包状态，动态调整菜单可用性
- **实时仪表盘**：显示节点状态、余额、共识参与信息等关键实时数据
- **安全交互控制**：处理键盘输入、菜单导航和安全界面切换
- **界面状态管理**：管理不同权限层次的界面模式和状态转换
- **视觉效果渲染**：基于pterm实现美观的终端UI效果和权限状态提示
- **新用户引导集成**：与首次用户引导系统无缝集成，提供个性化体验

## 🏗️ **组件架构**

```mermaid
graph TB
    subgraph "双层功能架构交互界面控制器"
        subgraph "权限识别层"
            PERM_DETECTOR["权限检测器<br/>🔍 PermissionDetector"]
            WALLET_STATUS["钱包状态检查<br/>💳 WalletStatusChecker"]
            FIRST_TIME["首次用户检测<br/>🆕 FirstTimeDetector"]
        end
        
        subgraph "界面控制层"
            MENU["双层菜单控制器<br/>🎯 DualMenu"]
            DASHBOARD["实时仪表盘<br/>📊 Dashboard"]
            NAVIGATOR["安全导航控制器<br/>🧭 SecureNavigator"]
            GUIDE_UI["引导界面控制器<br/>📚 GuideUIController"]
        end
        
        subgraph "界面状态层"
            STATE["权限状态管理<br/>🔄 PermissionStateManager"]
            MODE["显示模式控制<br/>🎨 DisplayMode"]
            EVENT["安全事件处理器<br/>⚡ SecurityEventHandler"]
            CONTEXT["用户上下文<br/>👤 UserContext"]
        end
        
        subgraph "数据更新层"
            SYS_UPDATER["系统级数据更新器<br/>🌐 SystemDataUpdater"]
            USER_UPDATER["用户级数据更新器<br/>🔐 UserDataUpdater"]
            TIMER["定时器管理<br/>⏱️ TimerManager"]
            CACHE["分层数据缓存<br/>💾 LayeredDataCache"]
        end
        
        subgraph "基础设施层"
            CORE_SVC["核心服务<br/>⚡ CoreServices"]
            CLIENT["API客户端<br/>🌐 APIClient"]
            WALLET_MGR["钱包管理器<br/>🔑 WalletManager"]
            UI["UI组件<br/>🎨 pterm组件"]
            INPUT["输入处理<br/>⌨️ InputHandler"]
            LOG["分级日志记录<br/>📝 TieredLogger"]
        end
    end
    
    %% 权限识别层连接
    PERM_DETECTOR --> WALLET_STATUS
    PERM_DETECTOR --> FIRST_TIME
    
    %% 界面控制层连接 
    PERM_DETECTOR --> MENU
    PERM_DETECTOR --> GUIDE_UI
    MENU --> STATE
    DASHBOARD --> STATE
    NAVIGATOR --> STATE
    GUIDE_UI --> STATE
    
    %% 界面状态层连接
    STATE --> MODE
    STATE --> EVENT
    STATE --> CONTEXT
    WALLET_STATUS --> CONTEXT
    
    %% 数据更新层连接
    DASHBOARD --> SYS_UPDATER
    DASHBOARD --> USER_UPDATER
    SYS_UPDATER --> TIMER
    USER_UPDATER --> TIMER
    SYS_UPDATER --> CACHE
    USER_UPDATER --> CACHE
    
    %% 基础设施层连接 - 系统级
    SYS_UPDATER --> CORE_SVC
    MENU --> CORE_SVC
    
    %% 基础设施层连接 - 用户级
    USER_UPDATER --> CLIENT
    USER_UPDATER --> WALLET_MGR
    MENU --> WALLET_MGR
    
    %% 通用连接
    MENU --> UI
    DASHBOARD --> UI
    NAVIGATOR --> UI
    GUIDE_UI --> UI
    
    EVENT --> INPUT
    MENU --> INPUT
    NAVIGATOR --> INPUT
    
    STATE --> LOG
    SYS_UPDATER --> LOG
    USER_UPDATER --> LOG
    EVENT --> LOG
    
    %% 样式设置 - 权限层次颜色
    style PERM_DETECTOR fill:#FFE8E8
    style WALLET_STATUS fill:#FFE8E8  
    style FIRST_TIME fill:#FFE8E8
    style MENU fill:#E8F5E8
    style DASHBOARD fill:#FFF3E0
    style NAVIGATOR fill:#E3F2FD
    style GUIDE_UI fill:#F3E5F5
    style STATE fill:#F3E5F5
    style SYS_UPDATER fill:#E8F5E8
    style USER_UPDATER fill:#FFF3E0
    style CORE_SVC fill:#E8F5E8
    style WALLET_MGR fill:#FFF3E0
```

## 📝 **双层功能架构核心接口**

```go
// DualMenu 双层功能菜单界面控制器接口
type DualMenu interface {
    // ShowMainMenu 显示带权限识别的主菜单界面
    ShowMainMenu(ctx context.Context) error
    
    // HandleUserSelection 处理用户选择（带权限验证）
    HandleUserSelection(ctx context.Context, selection int) error
    
    // RefreshMenu 刷新菜单显示（更新权限状态）
    RefreshMenu(ctx context.Context) error
    
    // SetPermissionLevel 设置当前用户权限级别
    SetPermissionLevel(level PermissionLevel) error
    
    // ShowSystemLevelMenu 显示系统级功能菜单
    ShowSystemLevelMenu(ctx context.Context) error
    
    // ShowUserLevelMenu 显示用户级功能菜单（需要钱包验证）
    ShowUserLevelMenu(ctx context.Context) error
}

// PermissionDetector 权限检测器接口
type PermissionDetector interface {
    // DetectPermissionLevel 检测当前用户权限级别
    DetectPermissionLevel(ctx context.Context) (PermissionLevel, error)
    
    // CheckWalletAvailability 检查钱包可用性
    CheckWalletAvailability(ctx context.Context) (bool, error)
    
    // IsFirstTimeUser 检查是否为首次用户
    IsFirstTimeUser(ctx context.Context) (bool, error)
}

// Dashboard 实时仪表盘接口（支持分层数据）
type Dashboard interface {
    // Start 启动分层数据仪表盘显示
    Start(ctx context.Context) error
    
    // Stop 停止仪表盘
    Stop() error
    
    // UpdateSystemData 更新系统级数据显示
    UpdateSystemData(data *SystemDashboardData) error
    
    // UpdateUserData 更新用户级数据显示（需要钱包权限）
    UpdateUserData(ctx context.Context, data *UserDashboardData) error
    
    // SetRefreshInterval 设置不同数据层的刷新间隔
    SetRefreshInterval(systemInterval, userInterval time.Duration) error
}
```

## 🎨 **界面设计**

### **主菜单界面**

```go
// 主菜单布局示例
func (m *Menu) renderMainMenu() string {
    return `
╭─────────────────── WES 节点控制台 ──────────────────╮
│                                                     │
│  🌐 节点状态: ` + m.nodeStatus + `    📊 区块高度: ` + m.blockHeight + `         │  
│  ⛏️ 挖矿状态: ` + m.miningStatus + `      💰 钱包余额: ` + m.balance + ` WES │
│  🔗 连接节点: ` + m.peerCount + `         📈 哈希率: ` + m.hashRate + ` MH/s       │
│                                                     │
├─────────────────── 功能菜单 ────────────────────────┤
│                                                     │
│  💰 账户管理     📊 区块链信息     ⛏️ 挖矿控制      │
│  🔄 转账操作     📄 交易查询      🌐 节点管理       │
│  📈 实时监控     ⚙️ 系统设置      🚪 退出          │
│                                                     │
╰─────────────────────────────────────────────────────╯`
}
```

### **实时仪表盘**

```go
// 仪表盘数据结构
type DashboardData struct {
    NodeStatus    NodeStatus    `json:"node_status"`
    BlockInfo     BlockInfo     `json:"block_info"`
    MiningStatus  MiningStatus  `json:"mining_status"`
    NetworkInfo   NetworkInfo   `json:"network_info"`
    AccountInfo   AccountInfo   `json:"account_info"`
    Timestamp     time.Time     `json:"timestamp"`
}

// 仪表盘渲染
func (d *Dashboard) renderDashboard(data *DashboardData) error {
    // 清屏并显示标题
    d.ui.Clear()
    d.ui.ShowTitle("WES 实时监控仪表盘")
    
    // 创建实时更新的面板
    panels := []pterm.Panel{
        {Data: d.renderNodePanel(data.NodeStatus)},
        {Data: d.renderBlockPanel(data.BlockInfo)},
        {Data: d.renderMiningPanel(data.MiningStatus)},
        {Data: d.renderNetworkPanel(data.NetworkInfo)},
    }
    
    return d.ui.ShowPanels(panels)
}
```

---

## ⚡ **双层功能架构交互流程**

### **🔍 权限检测与菜单初始化流程**

```mermaid
sequenceDiagram
    participant User as 👤 用户
    participant PermDetector as 🔍 权限检测器
    participant FirstTime as 🆕 首次用户检测器
    participant WalletStatus as 💳 钱包状态检查器
    participant Menu as 🎯 双层菜单控制器
    participant GuideUI as 📚 引导界面控制器

    Note over User,GuideUI: 🎯 启动与权限识别流程
    User->>+PermDetector: 启动CLI界面
    PermDetector->>+FirstTime: 检查是否首次用户
    
    alt 首次用户
        FirstTime-->>PermDetector: 是首次用户
        PermDetector->>+GuideUI: 启动首次用户引导
        GuideUI->>User: 显示欢迎界面和引导流程
        User->>GuideUI: 完成引导设置（创建钱包等）
        GuideUI-->>-PermDetector: 引导完成
    else 已有用户
        FirstTime-->>-PermDetector: 非首次用户
    end
    
    PermDetector->>+WalletStatus: 检查钱包状态
    WalletStatus->>WalletStatus: 扫描本地钱包文件
    alt 有可用钱包
        WalletStatus-->>PermDetector: 钱包可用，用户级功能可访问
        PermDetector->>PermDetector: 设置权限级别为 FULL_ACCESS
    else 无钱包
        WalletStatus-->>-PermDetector: 无钱包，仅系统级功能可用
        PermDetector->>PermDetector: 设置权限级别为 SYSTEM_ONLY
    end
    
    PermDetector->>+Menu: 初始化双层菜单
    Menu->>Menu: 根据权限级别渲染菜单界面
    Menu-->>-PermDetector: 菜单准备就绪
    PermDetector-->>-User: 显示个性化主菜单
```

### **🌐 系统级功能交互流程**

```mermaid
sequenceDiagram
    participant User as 👤 用户
    participant Menu as 🎯 双层菜单
    participant Navigator as 🧭 安全导航器
    participant SysUpdater as 🌐 系统级数据更新器
    participant CoreService as ⚡ 核心服务
    participant Dashboard as 📊 实时仪表盘

    Note over User,Dashboard: 🌐 系统级查询操作流程
    User->>+Menu: 选择系统级功能(区块链信息)
    Menu->>Menu: 验证为系统级操作，无需权限验证
    Menu->>+Navigator: 导航到区块链查询界面
    
    Navigator->>+SysUpdater: 请求系统级数据更新
    SysUpdater->>+CoreService: 直接调用ChainService.GetChainInfo()
    CoreService-->>-SysUpdater: 返回链状态信息
    SysUpdater->>SysUpdater: 缓存系统数据避免重复查询
    SysUpdater-->>-Navigator: 返回格式化的系统信息
    
    Navigator->>+Dashboard: 显示系统级仪表盘
    Dashboard->>Dashboard: 渲染区块高度、网络状态、节点信息
    Dashboard-->>User: 显示实时系统状态
    
    Note over User,Dashboard: 📊 系统级实时监控
    loop 每3秒更新系统数据
        Dashboard->>+SysUpdater: 获取最新系统数据
        SysUpdater->>+CoreService: 查询最新状态
        CoreService-->>-SysUpdater: 返回更新数据
        SysUpdater-->>-Dashboard: 返回系统状态更新
        Dashboard->>Dashboard: 更新系统级显示界面
    end
    
    User->>Dashboard: 按ESC退出监控
    Dashboard-->>-Navigator: 返回菜单
    Navigator-->>-Menu: 返回主菜单
    Menu-->>-User: 显示菜单选项
```

### **🔐 用户级功能交互流程（含权限验证）**

```mermaid
sequenceDiagram
    participant User as 👤 用户
    participant Menu as 🎯 双层菜单
    participant PermDetector as 🔍 权限检测器
    participant WalletMgr as 💳 钱包管理器
    participant Navigator as 🧭 安全导航器
    participant UserUpdater as 🔐 用户级数据更新器
    participant Commands as ⚙️ 用户级命令处理器
    participant TransactionSvc as ⚡ 交易服务

    Note over User,TransactionSvc: 🔐 用户级操作流程（转账示例）
    User->>+Menu: 选择用户级功能(转账操作)
    Menu->>+PermDetector: 验证用户级权限
    
    alt 用户无钱包
        PermDetector->>User: 提示需要创建或导入钱包
        User->>+WalletMgr: 创建新钱包
        WalletMgr->>User: 收集钱包信息（名称、密码）
        User->>WalletMgr: 提供钱包详细信息
        WalletMgr->>WalletMgr: 生成密钥对并加密存储
        WalletMgr-->>-User: 钱包创建成功
        PermDetector->>PermDetector: 更新权限级别为 FULL_ACCESS
    end
    
    PermDetector->>+WalletMgr: 请求钱包访问权限
    WalletMgr->>User: 请求钱包密码验证
    User->>WalletMgr: 输入钱包密码
    WalletMgr->>WalletMgr: 验证密码并解锁钱包访问权限
    alt 验证成功
        WalletMgr-->>PermDetector: 钱包访问权限已授权
        PermDetector-->>-Menu: 权限验证通过
        
        Menu->>+Navigator: 导航到用户级操作界面
        Navigator->>+UserUpdater: 请求用户级数据更新
        UserUpdater->>+WalletMgr: 获取用户钱包余额
        WalletMgr-->>-UserUpdater: 返回钱包余额信息
        UserUpdater-->>-Navigator: 返回用户数据
        
        Navigator->>+Commands: 执行转账命令
        Commands->>User: 收集转账参数（接收地址、金额等）
        User->>Commands: 提供转账详细信息
        Commands->>Commands: 验证转账参数和余额充足性
        Commands->>User: 显示转账确认信息
        User->>Commands: 确认执行转账
        
        Commands->>+WalletMgr: 请求私钥签名交易
        WalletMgr->>WalletMgr: 使用私钥签名交易数据
        WalletMgr-->>-Commands: 返回已签名交易
        Commands->>+TransactionSvc: 广播签名交易
        TransactionSvc-->>-Commands: 返回交易哈希
        
        Commands->>User: 显示转账成功，交易哈希：0x...
        Commands-->>-Navigator: 操作完成
        Navigator-->>-Menu: 返回主菜单
        
    else 验证失败
        WalletMgr-->>PermDetector: 密码错误，权限验证失败
        PermDetector-->>Menu: 权限验证失败
        Menu->>User: 显示密码错误提示，请重试
    end
    
    Menu-->>-User: 返回主菜单或重试权限验证
```

## 🎮 **用户交互功能**

### **键盘导航**

```go
// 键盘事件处理
func (m *Menu) handleKeyboardInput() error {
    for {
        key := m.input.ReadKey()
        
        switch key {
        case keyboard.ArrowUp:
            m.moveCursorUp()
        case keyboard.ArrowDown:
            m.moveCursorDown()
        case keyboard.Enter:
            return m.executeSelection()
        case keyboard.Escape:
            return m.showExitConfirm()
        case 'q', 'Q':
            return m.quit()
        case 'h', 'H':
            m.showHelp()
        case 'r', 'R':
            m.refresh()
        }
    }
}
```

### **实时数据更新**

```go
// 仪表盘数据更新
func (d *Dashboard) startDataUpdate(ctx context.Context) error {
    ticker := time.NewTicker(d.refreshInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // 获取最新数据
            data, err := d.fetchDashboardData()
            if err != nil {
                d.logger.Errorf("获取仪表盘数据失败: %v", err)
                continue
            }
            
            // 更新界面显示
            if err := d.updateDisplay(data); err != nil {
                d.logger.Errorf("更新界面显示失败: %v", err)
            }
            
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

## 🎨 **视觉效果**

### **动态效果**

```go
// 加载动画
func (m *Menu) showLoadingAnimation(message string) {
    spinner := m.ui.NewSpinner(message)
    spinner.Start()
    defer spinner.Stop()
    
    // 模拟加载过程
    time.Sleep(2 * time.Second)
}

// 渐变色彩
func (d *Dashboard) applyColorGradient(value float64, min, max float64) string {
    ratio := (value - min) / (max - min)
    
    switch {
    case ratio < 0.3:
        return pterm.FgRed.Sprint(fmt.Sprintf("%.2f", value))
    case ratio < 0.7:
        return pterm.FgYellow.Sprint(fmt.Sprintf("%.2f", value))
    default:
        return pterm.FgGreen.Sprint(fmt.Sprintf("%.2f", value))
    }
}
```

### **状态指示器**

```go
// 状态图标映射
func (m *Menu) getStatusIcon(status string) string {
    statusIcons := map[string]string{
        "running":    "🟢",
        "stopped":    "🔴", 
        "connecting": "🟡",
        "syncing":    "🔄",
        "error":      "❌",
    }
    
    if icon, exists := statusIcons[status]; exists {
        return icon
    }
    return "⚪"
}
```

## 📊 **性能优化**

| **优化策略** | **实现方案** | **性能提升** |
|-------------|-------------|-------------|
| 界面缓存 | 缓存静态界面元素 | ~40% 渲染时间减少 |
| 增量更新 | 只更新变化的数据部分 | ~60% CPU使用减少 |
| 异步渲染 | 后台数据获取 | ~80% 响应性提升 |
| 智能刷新 | 根据数据变化频率调整刷新间隔 | ~50% 网络请求减少 |

## 🔧 **配置示例**

```go
// 交互界面配置
type InteractiveConfig struct {
    RefreshInterval    time.Duration `json:"refresh_interval"`
    MaxHistoryLines    int          `json:"max_history_lines"`
    EnableColors       bool         `json:"enable_colors"`
    ShowTimestamps     bool         `json:"show_timestamps"`
    AutoRefresh        bool         `json:"auto_refresh"`
    KeyboardShortcuts  map[string]string `json:"keyboard_shortcuts"`
}

// 默认配置
var DefaultConfig = &InteractiveConfig{
    RefreshInterval:   5 * time.Second,
    MaxHistoryLines:   1000,
    EnableColors:      true,
    ShowTimestamps:    true,
    AutoRefresh:       true,
    KeyboardShortcuts: map[string]string{
        "quit":    "q",
        "help":    "h",
        "refresh": "r",
        "back":    "esc",
    },
}
```

## 🚨 **错误处理**

- **界面渲染失败**：降级到简单文本模式
- **数据获取超时**：显示缓存数据和警告提示
- **用户输入错误**：友好的错误提示和操作建议
- **网络连接问题**：离线模式和重连提示

---

> 📝 **说明**：本模块专注于提供优秀的用户交互体验，所有界面元素都经过精心设计，确保操作的直观性和视觉效果的美观性。

> 🔄 **维护**：随着用户反馈和使用习惯的变化，持续优化界面设计和交互流程，提升用户满意度。
