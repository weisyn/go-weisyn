// Package flows 提供可复用的交互流程
package flows

import (
	"context"
	"time"
)

// ============================================================================
// Account Flow Ports（端口接口）
//
// 这些接口定义了账户流程需要的后端服务能力，解耦UI交互与具体实现。
// 客户端可以通过 transport（JSON-RPC/REST）或 mock 实现这些接口。
// ============================================================================

// AccountService 账户服务端口接口
//
// 功能：
//   - 提供账户余额查询能力
//   - 支持主币和代币余额查询
//
// 实现方式：
//   - 通过 JSON-RPC/REST 客户端调用节点 API
//   - Mock 实现用于测试
type AccountService interface {
	// GetBalance 获取账户余额
	//
	// 参数：
	//   - ctx: 上下文
	//   - address: 账户地址（Base58Check编码）
	//
	// 返回：
	//   - balance: 主币余额（最小单位）
	//   - tokenBalances: 代币余额列表
	//   - error: 错误信息
	GetBalance(ctx context.Context, address string) (balance uint64, tokenBalances []TokenBalance, err error)
}

// ContractBalanceService 合约代币余额服务
//
// 功能：
//   - 根据配置的合约列表查询指定地址的代币余额
type ContractBalanceService interface {
	FetchBalances(ctx context.Context, ownerAddress string, specs []ContractTokenSpec) ([]TokenBalance, error)
}

// WalletService 钱包服务端口接口
//
// 功能：
//   - 提供本地钱包管理能力
//   - 支持创建、导入、列表、删除、解锁等操作
//   - 私钥加密存储，密码验证
//
// 实现方式：
//   - 本地keystore文件管理
//   - 使用 Argon2/PBKDF2 加密私钥
type WalletService interface {
	// ListWallets 列出所有钱包
	ListWallets(ctx context.Context) ([]WalletInfo, error)

	// CreateWallet 创建新钱包
	//
	// 参数：
	//   - name: 钱包名称
	//   - password: 钱包密码
	//
	// 返回：
	//   - WalletInfo: 钱包信息（包含地址）
	//   - error: 错误信息
	CreateWallet(ctx context.Context, name, password string) (*WalletInfo, error)

	// ImportWallet 导入已有钱包
	//
	// 参数：
	//   - name: 钱包名称
	//   - privateKey: 私钥（十六进制字符串）
	//   - password: 钱包密码
	//
	// 返回：
	//   - WalletInfo: 钱包信息
	//   - error: 错误信息
	ImportWallet(ctx context.Context, name, privateKey, password string) (*WalletInfo, error)

	// DeleteWallet 删除钱包
	//
	// 参数：
	//   - name: 钱包名称
	//
	// 返回：
	//   - error: 错误信息
	DeleteWallet(ctx context.Context, name string) error

	// UnlockWallet 解锁钱包
	//
	// 参数：
	//   - name: 钱包名称
	//   - password: 钱包密码
	//
	// 返回：
	//   - error: 错误信息
	UnlockWallet(ctx context.Context, name, password string) error

	// SetDefaultWallet 设置默认钱包
	//
	// 参数：
	//   - name: 钱包名称
	//
	// 返回：
	//   - error: 错误信息
	SetDefaultWallet(ctx context.Context, name string) error

	// ExportPrivateKey 导出私钥
	//
	// 参数：
	//   - name: 钱包名称
	//   - password: 钱包密码
	//
	// 返回：
	//   - privateKey: 私钥（十六进制字符串）
	//   - error: 错误信息
	ExportPrivateKey(ctx context.Context, name, password string) (string, error)

	// ChangePassword 修改钱包密码
	//
	// 参数：
	//   - name: 钱包名称
	//   - oldPassword: 旧密码
	//   - newPassword: 新密码
	//
	// 返回：
	//   - error: 错误信息
	ChangePassword(ctx context.Context, name, oldPassword, newPassword string) error

	// ValidatePassword 验证密码
	//
	// 参数：
	//   - name: 钱包名称
	//   - password: 密码
	//
	// 返回：
	//   - bool: 密码是否正确
	//   - error: 错误信息
	ValidatePassword(ctx context.Context, name, password string) (bool, error)
}

// AddressValidator 地址验证器端口接口
//
// 功能：
//   - 验证地址格式有效性
//   - 支持 Base58Check 编码地址
type AddressValidator interface {
	// ValidateAddress 验证地址格式
	//
	// 参数：
	//   - address: 地址字符串
	//
	// 返回：
	//   - bool: 地址是否有效
	//   - error: 错误信息
	ValidateAddress(address string) (bool, error)
}

// ============================================================================
// Data Transfer Objects (DTOs)
// ============================================================================

// WalletInfo 钱包信息
type WalletInfo struct {
	ID        string    // 钱包唯一标识
	Name      string    // 钱包名称
	Address   string    // 钱包地址
	IsDefault bool      // 是否为默认钱包
	IsLocked  bool      // 是否锁定
	CreatedAt time.Time // 创建时间
	Mnemonic  string    // 助记词（仅在创建时返回，之后不会再显示）
}

// TokenBalance 代币余额
type TokenBalance struct {
	TokenID   string // 代币ID
	TokenName string // 代币名称
	Amount    uint64 // 代币数量
}

// ContractTokenSpec 需要查询的合约代币定义
type ContractTokenSpec struct {
	Label       string // 展示名称
	ContentHash string // 合约内容哈希（64位十六进制）
	TokenID     string // 代币标识（可选）
}

// BalanceInfo 余额信息（用于UI展示）
type BalanceInfo struct {
	Address          string         // 地址
	Balance          uint64         // 主币余额（最小单位）
	BalanceFormatted string         // 格式化后的余额字符串
	TokenBalances    []TokenBalance // 代币余额列表
}

// CreateWalletResult 创建钱包结果
type CreateWalletResult struct {
	WalletName string // 钱包名称
	Address    string // 钱包地址
	Success    bool   // 是否成功
	Message    string // 消息
}

// ImportWalletResult 导入钱包结果
type ImportWalletResult struct {
	WalletName string // 钱包名称
	Address    string // 钱包地址
	Success    bool   // 是否成功
	Message    string // 消息
}

// ExportPrivateKeyResult 导出私钥结果
type ExportPrivateKeyResult struct {
	WalletName string // 钱包名称
	Address    string // 钱包地址
	PrivateKey string // 私钥（十六进制字符串）
	Warning    string // 安全警告
}
