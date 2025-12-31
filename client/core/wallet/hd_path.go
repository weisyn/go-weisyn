// Package wallet provides wallet functionality for WES blockchain.
package wallet

import (
	"fmt"
	"strconv"
	"strings"
)

// BIP44 相关常量
const (
	// WESCoinType WES 链的 BIP44 Coin Type (待 SLIP-0044 注册)
	// 参考: https://github.com/satoshilabs/slips/blob/master/slip-0044.md
	WESCoinType uint32 = 8888

	// BIP44Purpose BIP44 标准的 purpose 值
	BIP44Purpose uint32 = 44

	// HardenedOffset 硬化派生偏移量
	HardenedOffset uint32 = 0x80000000

	// DefaultAccount 默认账户索引
	DefaultAccount uint32 = 0

	// ExternalChain 外部链（用于接收地址）
	ExternalChain uint32 = 0

	// InternalChain 内部链（用于找零地址）
	InternalChain uint32 = 1

	// DefaultAddressIndex 默认地址索引
	DefaultAddressIndex uint32 = 0
)

// DerivationPath BIP32/BIP44 派生路径
type DerivationPath struct {
	Purpose      uint32 `json:"purpose"`       // 目的（通常为 44'）
	CoinType     uint32 `json:"coin_type"`     // 币种类型
	Account      uint32 `json:"account"`       // 账户
	Change       uint32 `json:"change"`        // 变化链（0=外部，1=内部）
	AddressIndex uint32 `json:"address_index"` // 地址索引
}

// DefaultDerivationPath 返回 WES 默认派生路径
// m/44'/8888'/0'/0/0
func DefaultDerivationPath() *DerivationPath {
	return &DerivationPath{
		Purpose:      BIP44Purpose,
		CoinType:     WESCoinType,
		Account:      DefaultAccount,
		Change:       ExternalChain,
		AddressIndex: DefaultAddressIndex,
	}
}

// NewDerivationPath 创建新的派生路径
func NewDerivationPath(account, change, addressIndex uint32) *DerivationPath {
	return &DerivationPath{
		Purpose:      BIP44Purpose,
		CoinType:     WESCoinType,
		Account:      account,
		Change:       change,
		AddressIndex: addressIndex,
	}
}

// ParseDerivationPath 解析派生路径字符串
// 支持格式: m/44'/8888'/0'/0/0 或 44'/8888'/0'/0/0
func ParseDerivationPath(path string) (*DerivationPath, error) {
	// 移除开头的 "m/" 或 "M/"
	path = strings.TrimPrefix(path, "m/")
	path = strings.TrimPrefix(path, "M/")

	// 分割路径组件
	parts := strings.Split(path, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid derivation path: expected 5 components, got %d", len(parts))
	}

	dp := &DerivationPath{}
	var err error

	// 解析 purpose
	dp.Purpose, err = parsePathComponent(parts[0], true)
	if err != nil {
		return nil, fmt.Errorf("invalid purpose: %w", err)
	}
	if dp.Purpose != BIP44Purpose {
		return nil, fmt.Errorf("invalid purpose: expected %d (BIP44), got %d", BIP44Purpose, dp.Purpose)
	}

	// 解析 coin type
	dp.CoinType, err = parsePathComponent(parts[1], true)
	if err != nil {
		return nil, fmt.Errorf("invalid coin type: %w", err)
	}

	// 解析 account
	dp.Account, err = parsePathComponent(parts[2], true)
	if err != nil {
		return nil, fmt.Errorf("invalid account: %w", err)
	}

	// 解析 change
	dp.Change, err = parsePathComponent(parts[3], false)
	if err != nil {
		return nil, fmt.Errorf("invalid change: %w", err)
	}
	if dp.Change > 1 {
		return nil, fmt.Errorf("invalid change: expected 0 or 1, got %d", dp.Change)
	}

	// 解析 address index
	dp.AddressIndex, err = parsePathComponent(parts[4], false)
	if err != nil {
		return nil, fmt.Errorf("invalid address index: %w", err)
	}

	return dp, nil
}

// parsePathComponent 解析路径组件
// requireHardened: 是否要求硬化派生
func parsePathComponent(component string, requireHardened bool) (uint32, error) {
	isHardened := strings.HasSuffix(component, "'") || strings.HasSuffix(component, "H") || strings.HasSuffix(component, "h")

	if requireHardened && !isHardened {
		return 0, fmt.Errorf("hardened derivation required for %s", component)
	}

	// 移除硬化标记
	component = strings.TrimSuffix(component, "'")
	component = strings.TrimSuffix(component, "H")
	component = strings.TrimSuffix(component, "h")

	// 解析数字
	value, err := strconv.ParseUint(component, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", component)
	}

	return uint32(value), nil
}

// String 返回路径字符串表示
func (dp *DerivationPath) String() string {
	return fmt.Sprintf("m/%d'/%d'/%d'/%d/%d",
		dp.Purpose,
		dp.CoinType,
		dp.Account,
		dp.Change,
		dp.AddressIndex,
	)
}

// ToUint32Array 转换为 uint32 数组（用于 hdkeychain）
// 返回包含硬化标记的完整路径
func (dp *DerivationPath) ToUint32Array() []uint32 {
	return []uint32{
		dp.Purpose + HardenedOffset,   // 硬化
		dp.CoinType + HardenedOffset,  // 硬化
		dp.Account + HardenedOffset,   // 硬化
		dp.Change,                     // 非硬化
		dp.AddressIndex,               // 非硬化
	}
}

// WithAccount 返回使用指定账户的新路径
func (dp *DerivationPath) WithAccount(account uint32) *DerivationPath {
	newPath := *dp
	newPath.Account = account
	return &newPath
}

// WithChange 返回使用指定变化链的新路径
func (dp *DerivationPath) WithChange(change uint32) *DerivationPath {
	newPath := *dp
	newPath.Change = change
	return &newPath
}

// WithAddressIndex 返回使用指定地址索引的新路径
func (dp *DerivationPath) WithAddressIndex(index uint32) *DerivationPath {
	newPath := *dp
	newPath.AddressIndex = index
	return &newPath
}

// NextAddress 返回下一个地址的路径
func (dp *DerivationPath) NextAddress() *DerivationPath {
	return dp.WithAddressIndex(dp.AddressIndex + 1)
}

// IsExternal 是否为外部链（接收地址）
func (dp *DerivationPath) IsExternal() bool {
	return dp.Change == ExternalChain
}

// IsInternal 是否为内部链（找零地址）
func (dp *DerivationPath) IsInternal() bool {
	return dp.Change == InternalChain
}

// IsWESPath 是否为有效的 WES 路径
func (dp *DerivationPath) IsWESPath() bool {
	return dp.Purpose == BIP44Purpose && dp.CoinType == WESCoinType
}

// Validate 验证路径是否有效
func (dp *DerivationPath) Validate() error {
	if dp.Purpose != BIP44Purpose {
		return fmt.Errorf("invalid purpose: expected %d, got %d", BIP44Purpose, dp.Purpose)
	}
	if dp.CoinType != WESCoinType {
		return fmt.Errorf("invalid coin type: expected %d (WES), got %d", WESCoinType, dp.CoinType)
	}
	if dp.Change > 1 {
		return fmt.Errorf("invalid change: expected 0 or 1, got %d", dp.Change)
	}
	return nil
}

// HDPathGenerator HD 路径生成器
type HDPathGenerator struct {
	baseAccount uint32
}

// NewHDPathGenerator 创建新的 HD 路径生成器
func NewHDPathGenerator(account uint32) *HDPathGenerator {
	return &HDPathGenerator{
		baseAccount: account,
	}
}

// GenerateReceivePath 生成接收地址路径
func (g *HDPathGenerator) GenerateReceivePath(index uint32) *DerivationPath {
	return NewDerivationPath(g.baseAccount, ExternalChain, index)
}

// GenerateChangePath 生成找零地址路径
func (g *HDPathGenerator) GenerateChangePath(index uint32) *DerivationPath {
	return NewDerivationPath(g.baseAccount, InternalChain, index)
}

// GeneratePaths 批量生成路径
func (g *HDPathGenerator) GeneratePaths(change uint32, startIndex, count uint32) []*DerivationPath {
	paths := make([]*DerivationPath, count)
	for i := uint32(0); i < count; i++ {
		paths[i] = NewDerivationPath(g.baseAccount, change, startIndex+i)
	}
	return paths
}

// WESDefaultPath 返回 WES 默认路径字符串
// m/44'/8888'/0'/0/0
func WESDefaultPath() string {
	return DefaultDerivationPath().String()
}

// WESPathForAccount 返回指定账户的路径字符串
func WESPathForAccount(account uint32) string {
	return NewDerivationPath(account, ExternalChain, DefaultAddressIndex).String()
}

// WESPathForIndex 返回指定地址索引的路径字符串
func WESPathForIndex(account, index uint32) string {
	return NewDerivationPath(account, ExternalChain, index).String()
}

