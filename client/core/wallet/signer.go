// Package wallet provides wallet signing functionality for client operations.
package wallet

import (
	"crypto/ecdsa"
	"time"
)

// Signer 签名器接口 - 统一的签名抽象
// 支持多种签名方式：Keystore/助记词/硬件钱包
type Signer interface {
	// Sign 签名交易
	// tx: 待签名的交易数据
	// fromAddr: 签名地址(用于选择私钥)
	// 返回: 签名后的交易数据
	Sign(tx []byte, fromAddr string) ([]byte, error)

	// SignHash 签名哈希值(用于消息签名)
	SignHash(hash []byte, fromAddr string) ([]byte, error)

	// GetAddress 获取地址
	// derivationPath: 派生路径(HD钱包),如 m/44'/WES'/0'/0/0
	// 空字符串表示使用默认地址
	GetAddress(derivationPath string) (string, error)

	// ListAddresses 列出所有管理的地址
	ListAddresses() ([]string, error)

	// Unlock 解锁签名器(如需密码)
	// password: 解锁密码
	// duration: 解锁时长,0表示永久解锁(直到调用Lock)
	Unlock(password string, duration time.Duration) error

	// Lock 锁定签名器
	Lock()

	// IsLocked 检查是否已锁定
	IsLocked() bool

	// Type 返回签名器类型(keystore/mnemonic/hardware)
	Type() SignerType
}

// HDSigner HD钱包签名器扩展接口
// 支持 BIP32/BIP44 密钥派生的签名器应实现此接口
type HDSigner interface {
	Signer

	// DeriveAddress 派生新地址
	// path: BIP44 派生路径，如 m/44'/8888'/0'/0/0
	DeriveAddress(path string) (string, error)

	// DeriveMultipleAddresses 批量派生地址
	// account: 账户索引
	// startIndex: 起始地址索引
	// count: 派生数量
	DeriveMultipleAddresses(account, startIndex, count uint32) ([]string, error)

	// GetDerivationPath 获取当前默认派生路径
	GetDerivationPath() string

	// GetDerivedPaths 获取所有已派生的路径
	GetDerivedPaths() []string

	// GetPathForAddress 获取地址对应的派生路径
	GetPathForAddress(address string) (string, bool)

	// ExportExtendedPublicKey 导出扩展公钥（用于观察钱包）
	// account: 账户索引
	ExportExtendedPublicKey(account uint32) (string, error)
}

// SignerType 签名器类型
type SignerType string

const (
	SignerTypeKeystore SignerType = "keystore" // 加密Keystore文件
	SignerTypeMnemonic SignerType = "mnemonic" // BIP39助记词
	SignerTypeHardware SignerType = "hardware" // 硬件钱包(预留)
	SignerTypeExternal SignerType = "external" // 外部签名器(预留)
)

// Account 账户信息
type Account struct {
	Address        string    `json:"address"`
	DerivationPath string    `json:"derivation_path,omitempty"` // HD钱包派生路径
	CreatedAt      time.Time `json:"created_at"`
	Label          string    `json:"label,omitempty"` // 用户标签
}

// AccountService 账户服务接口（已弃用：使用 AccountManager 结构体代替）
type AccountService interface {
	// NewAccount 创建新账户
	// password: 加密密码
	// label: 账户标签(可选)
	NewAccount(password string, label string) (*Account, error)

	// ImportAccount 导入账户
	// privateKey: 私钥(十六进制)
	// password: 加密密码
	// label: 账户标签(可选)
	ImportAccount(privateKey string, password string, label string) (*Account, error)

	// ExportAccount 导出账户私钥
	// address: 账户地址
	// password: 解密密码
	ExportAccount(address string, password string) (string, error)

	// DeleteAccount 删除账户
	// address: 账户地址
	// password: 验证密码
	DeleteAccount(address string, password string) error

	// GetAccount 获取账户信息
	GetAccount(address string) (*Account, error)

	// ListAccounts 列出所有账户
	ListAccounts() ([]*Account, error)

	// GetSigner 获取账户的签名器
	// address: 账户地址
	// 返回的签名器需要调用Unlock才能签名
	GetSigner(address string) (Signer, error)
}

// KeyDerivation HD钱包密钥派生接口
type KeyDerivation interface {
	// DeriveKey 派生子密钥
	// path: 派生路径,如 m/44'/WES'/0'/0/0
	DeriveKey(path string) (*ecdsa.PrivateKey, error)

	// DeriveAddress 派生地址
	// path: 派生路径
	DeriveAddress(path string) (string, error)

	// GetMasterKey 获取主密钥
	GetMasterKey() (*ecdsa.PrivateKey, error)
}
