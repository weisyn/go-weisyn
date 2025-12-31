# UX Flows - 可复用交互流程

提供高级交互流程，封装完整的用户体验，用于账户管理、转账等复杂操作。

## 📋 设计理念

**Flows** 基于六边形架构（端口与适配器模式），将 UI 交互与后端实现解耦：

- **UI 交互**：通过 `ui.Components` 接口提供统一的用户体验
- **后端服务**：通过端口接口（Ports）定义能力需求
- **实现无关**：后端可以是 JSON-RPC、REST、Mock，甚至本地服务

```
┌─────────────────┐
│   Cobra CLI     │ ← 命令层
└────────┬────────┘
         │
┌────────▼────────┐
│   UX Flows      │ ← 交互流程层
│                 │
│ • AccountFlow   │
│ • TransferFlow  │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼──────┐
│  UI   │ │ Ports   │ ← 端口接口层
│ Comp. │ │ (服务)  │
└───────┘ └────┬────┘
               │
      ┌────────┴────────┐
      │                 │
┌─────▼──────┐  ┌──────▼───────┐
│  Transport │  │ Local Wallet │ ← 实现层
│ (JSON-RPC) │  │   (Keystore) │
└────────────┘  └──────────────┘
```

## 🚀 快速开始

### 1. 创建 AccountFlow 实例

```go
package main

import (
    "context"
    
    "github.com/weisyn/v1/client/pkg/ux/flows"
    "github.com/weisyn/v1/client/pkg/ux/ui"
)

func main() {
    // 1. 创建 UI 组件
    uiComponents := ui.NewComponents(ui.NoopLogger())
    
    // 2. 创建后端服务实现（示例：Mock）
    accountService := NewMockAccountService()
    walletService := NewMockWalletService()
    addressValidator := NewMockAddressValidator()
    
    // 3. 创建 AccountFlow
    accountFlow := flows.NewAccountFlow(
        uiComponents,
        accountService,
        walletService,
        addressValidator,
    )
    
    // 4. 使用交互流程
    ctx := context.Background()
    
    // 显示余额（交互式）
    err := accountFlow.ShowBalance(ctx)
    if err != nil {
        panic(err)
    }
    
    // 创建钱包（交互式）
    result, err := accountFlow.CreateWallet(ctx)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("钱包创建成功：%s\n", result.Address)
}
```

### 2. 创建 TransferFlow 实例

```go
// 1. 创建 UI 组件
uiComponents := ui.NewComponents(ui.NoopLogger())

// 2. 创建后端服务实现
transferService := NewMockTransferService()
walletService := NewMockWalletService()
addressValidator := NewMockAddressValidator()

// 3. 创建 TransferFlow
transferFlow := flows.NewTransferFlow(
    uiComponents,
    transferService,
    walletService,
    addressValidator,
)

// 4. 使用交互流程
ctx := context.Background()

// 执行单笔转账（交互式）
result, err := transferFlow.ExecuteTransfer(ctx)
if err != nil {
    panic(err)
}

fmt.Printf("转账成功：%s\n", result.TxHash)
```

## 📦 AccountFlow 功能列表

### 交互式流程

| 方法 | 说明 | 交互 |
|------|------|------|
| `ShowBalance(ctx)` | 查询余额 | 输入地址 → 显示余额 |
| `ShowWalletList(ctx)` | 显示钱包列表 | 显示表格 |
| `CreateWallet(ctx)` | 创建钱包 | 输入名称/密码 → 显示结果 |
| `ImportWallet(ctx)` | 导入钱包 | 输入名称/私钥/密码 → 显示结果 |
| `DeleteWallet(ctx)` | 删除钱包 | 选择钱包 → 确认 → 删除 |
| `ExportPrivateKey(ctx)` | 导出私钥 | 选择钱包 → 输入密码 → 显示私钥（含警告） |
| `ChangePassword(ctx)` | 修改密码 | 选择钱包 → 输入旧密码 → 输入新密码 |

### 编程式调用

| 方法 | 说明 | 场景 |
|------|------|------|
| `GetBalanceByAddress(ctx, address)` | 获取指定地址余额 | 命令行参数传入地址 |

## 📦 TransferFlow 功能列表

### 交互式流程

| 方法 | 说明 | 交互 |
|------|------|------|
| `ExecuteTransfer(ctx)` | 单笔转账 | 选择钱包 → 输入地址/金额 → 确认 → 执行 |
| `ExecuteBatchTransfer(ctx)` | 批量转账 | 选择钱包 → 输入多个地址/金额 → 确认 → 执行 |
| `ExecuteTimeLockTransfer(ctx)` | 时间锁转账 | 选择钱包 → 输入地址/金额/锁定时间 → 确认 → 执行 |
| `EstimateFee(ctx)` | 估算手续费 | 输入发送方/接收方/金额 → 显示估算结果 |

## 🔌 端口接口（Ports）

Flows 通过端口接口定义对后端服务的需求，实现解耦。

### AccountService 接口

```go
type AccountService interface {
    // GetBalance 获取账户余额
    GetBalance(ctx context.Context, address string) (balance uint64, tokenBalances []TokenBalance, err error)
}
```

### WalletService 接口

```go
type WalletService interface {
    ListWallets(ctx context.Context) ([]WalletInfo, error)
    CreateWallet(ctx context.Context, name, password string) (*WalletInfo, error)
    ImportWallet(ctx context.Context, name, privateKey, password string) (*WalletInfo, error)
    DeleteWallet(ctx context.Context, name string) error
    UnlockWallet(ctx context.Context, name, password string) error
    SetDefaultWallet(ctx context.Context, name string) error
    ExportPrivateKey(ctx context.Context, name, password string) (string, error)
    ChangePassword(ctx context.Context, name, oldPassword, newPassword string) error
    ValidatePassword(ctx context.Context, name, password string) (bool, error)
}
```

### TransferService 接口

```go
type TransferService interface {
    Transfer(ctx context.Context, req *TransferRequest) (txHash string, err error)
    BatchTransfer(ctx context.Context, req *BatchTransferRequest) (txHash string, err error)
    TimeLockTransfer(ctx context.Context, req *TimeLockTransferRequest) (txHash string, err error)
    EstimateFee(ctx context.Context, from, to string, amount uint64) (fee uint64, err error)
}
```

### AddressValidator 接口

```go
type AddressValidator interface {
    ValidateAddress(address string) (bool, error)
}
```

## 🛠️ 实现端口接口

### 示例：Mock 实现

```go
// MockAccountService 模拟账户服务
type MockAccountService struct{}

func (m *MockAccountService) GetBalance(ctx context.Context, address string) (uint64, []flows.TokenBalance, error) {
    // 模拟返回余额
    return 100_000_000_000_000_000_000, []flows.TokenBalance{}, nil // 100 WES
}

// MockWalletService 模拟钱包服务
type MockWalletService struct {
    wallets map[string]*flows.WalletInfo
}

func (m *MockWalletService) CreateWallet(ctx context.Context, name, password string) (*flows.WalletInfo, error) {
    wallet := &flows.WalletInfo{
        ID:        generateID(),
        Name:      name,
        Address:   "weisyn1" + generateRandomAddress(),
        IsDefault: len(m.wallets) == 0,
        IsLocked:  false,
        CreatedAt: time.Now(),
    }
    m.wallets[wallet.ID] = wallet
    return wallet, nil
}

// ... 实现其他方法
```

### 示例：JSON-RPC 实现

```go
// JSONRPCAccountService 通过 JSON-RPC 实现账户服务
type JSONRPCAccountService struct {
    client *transport.JSONRPCClient
}

func (j *JSONRPCAccountService) GetBalance(ctx context.Context, address string) (uint64, []flows.TokenBalance, error) {
    var result struct {
        Balance uint64 `json:"balance"`
    }
    
    err := j.client.Call(ctx, "account_getBalance", []interface{}{address}, &result)
    if err != nil {
        return 0, nil, err
    }
    
    return result.Balance, []flows.TokenBalance{}, nil
}
```

## 📝 在 Cobra 命令中使用

### 示例：账户余额命令

```go
package cmd

import (
    "context"
    
    "github.com/spf13/cobra"
    "github.com/weisyn/v1/client/pkg/ux/flows"
    "github.com/weisyn/v1/client/pkg/ux/ui"
)

var balanceCmd = &cobra.Command{
    Use:   "balance [address]",
    Short: "查询账户余额",
    Long:  `查询指定地址的账户余额，支持主币和代币`,
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx := context.Background()
        
        // 1. 创建 UI 组件
        uiComponents := ui.NewComponents(ui.NoopLogger())
        
        // 2. 创建后端服务（假设已有全局实例）
        accountService := getAccountService()
        walletService := getWalletService()
        addressValidator := getAddressValidator()
        
        // 3. 创建 AccountFlow
        accountFlow := flows.NewAccountFlow(
            uiComponents,
            accountService,
            walletService,
            addressValidator,
        )
        
        // 4. 执行流程
        if len(args) > 0 {
            // 命令行参数传入地址（编程式调用）
            balanceInfo, err := accountFlow.GetBalanceByAddress(ctx, args[0])
            if err != nil {
                return err
            }
            
            // 展示结果
            uiComponents.ShowBalanceInfo(balanceInfo.Address, convertToFloat(balanceInfo.BalanceFormatted), "WES")
        } else {
            // 交互式输入
            err := accountFlow.ShowBalance(ctx)
            if err != nil {
                return err
            }
        }
        
        return nil
    },
}

func init() {
    rootCmd.AddCommand(balanceCmd)
}
```

### 示例：转账命令

```go
var transferCmd = &cobra.Command{
    Use:   "transfer",
    Short: "执行转账操作",
    Long:  `执行单笔转账、批量转账或时间锁转账`,
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx := context.Background()
        
        // 1. 创建 UI 组件
        uiComponents := ui.NewComponents(ui.NoopLogger())
        
        // 2. 创建后端服务
        transferService := getTransferService()
        walletService := getWalletService()
        addressValidator := getAddressValidator()
        
        // 3. 创建 TransferFlow
        transferFlow := flows.NewTransferFlow(
            uiComponents,
            transferService,
            walletService,
            addressValidator,
        )
        
        // 4. 显示转账类型菜单
        options := []string{
            "单笔转账",
            "批量转账",
            "时间锁转账",
            "估算手续费",
        }
        
        selectedIndex, err := uiComponents.ShowMenu("选择转账类型", options)
        if err != nil {
            return err
        }
        
        // 5. 根据选择执行相应流程
        switch selectedIndex {
        case 0:
            _, err = transferFlow.ExecuteTransfer(ctx)
        case 1:
            _, err = transferFlow.ExecuteBatchTransfer(ctx)
        case 2:
            _, err = transferFlow.ExecuteTimeLockTransfer(ctx)
        case 3:
            _, err = transferFlow.EstimateFee(ctx)
        }
        
        return err
    },
}

func init() {
    rootCmd.AddCommand(transferCmd)
}
```

## 🎯 最佳实践

### 1. 错误处理

Flows 内部已包含错误提示（通过 UI 组件），但仍会返回错误供调用方记录日志或进一步处理：

```go
result, err := accountFlow.CreateWallet(ctx)
if err != nil {
    // Flow 已通过 UI 显示错误信息
    // 调用方可记录日志或返回错误码
    logger.Error("创建钱包失败", err)
    return err
}
```

### 2. 非 TTY 环境处理

Flows 通过 `ui.Components` 自动适配非 TTY 环境。对于需要交互的流程，建议提供编程式调用版本：

```go
// 交互式（TTY）
err := accountFlow.ShowBalance(ctx)

// 编程式（非 TTY）
balanceInfo, err := accountFlow.GetBalanceByAddress(ctx, address)
```

### 3. 测试

使用 Mock 实现进行单元测试：

```go
func TestAccountFlow_CreateWallet(t *testing.T) {
    // 1. 创建 Mock 服务
    mockUI := NewMockUI()
    mockWalletService := NewMockWalletService()
    
    // 2. 设置预期输入
    mockUI.SetInputs([]string{
        "test-wallet",  // 钱包名称
        "password123",  // 密码
        "password123",  // 确认密码
    })
    
    // 3. 创建 Flow
    accountFlow := flows.NewAccountFlow(
        mockUI,
        NewMockAccountService(),
        mockWalletService,
        NewMockAddressValidator(),
    )
    
    // 4. 执行测试
    result, err := accountFlow.CreateWallet(context.Background())
    
    // 5. 验证结果
    assert.NoError(t, err)
    assert.Equal(t, "test-wallet", result.WalletName)
    assert.NotEmpty(t, result.Address)
}
```

## 🔗 相关链接

- [UI 组件文档](../ui/README.md)
- [Transport 文档](../../core/transport/README.md)
- [CLI 架构文档](../../../../_docs/architecture/CLI_ARCHITECTURE_SPECIFICATION.md)

