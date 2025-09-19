// Package permissions 提供双层权限控制系统
package permissions

import (
	"context"
	"fmt"
)

// PermissionLevel 权限级别枚举
type PermissionLevel int

const (
	// UNKNOWN 未知权限级别
	UNKNOWN PermissionLevel = iota
	// SystemOnly 仅系统级功能，无钱包访问权限
	SystemOnly
	// FullAccess 完全访问权限，包括用户级功能
	FullAccess
)

// String 返回权限级别的字符串表示
func (p PermissionLevel) String() string {
	switch p {
	case UNKNOWN:
		return "Unknown"
	case SystemOnly:
		return "SystemOnly"
	case FullAccess:
		return "FullAccess"
	default:
		return "Undefined"
	}
}

// IsSystemLevelAllowed 检查是否允许系统级操作
func (p PermissionLevel) IsSystemLevelAllowed() bool {
	return p >= SystemOnly
}

// IsUserLevelAllowed 检查是否允许用户级操作
func (p PermissionLevel) IsUserLevelAllowed() bool {
	return p >= FullAccess
}

// PermissionDetector 权限检测器接口
type PermissionDetector interface {
	// DetectPermissionLevel 检测当前用户权限级别
	DetectPermissionLevel(ctx context.Context) (PermissionLevel, error)

	// CheckWalletAvailability 检查钱包可用性
	CheckWalletAvailability(ctx context.Context) (bool, error)

	// IsFirstTimeUser 检查是否为首次用户
	IsFirstTimeUser(ctx context.Context) (bool, error)

	// UpdatePermissionLevel 更新权限级别（钱包创建后）
	UpdatePermissionLevel(ctx context.Context, level PermissionLevel) error
}

// UserContext 用户上下文信息
type UserContext struct {
	PermissionLevel  PermissionLevel
	HasWallets       bool
	IsFirstTimeUser  bool
	CurrentWallet    string
	IsWalletUnlocked bool
}

// NewUserContext 创建用户上下文
func NewUserContext() *UserContext {
	return &UserContext{
		PermissionLevel:  UNKNOWN,
		HasWallets:       false,
		IsFirstTimeUser:  true,
		CurrentWallet:    "",
		IsWalletUnlocked: false,
	}
}

// UpdateFromDetection 从权限检测结果更新上下文
func (uc *UserContext) UpdateFromDetection(
	permissionLevel PermissionLevel,
	hasWallets bool,
	isFirstTime bool,
) {
	uc.PermissionLevel = permissionLevel
	uc.HasWallets = hasWallets
	uc.IsFirstTimeUser = isFirstTime
}

// SetCurrentWallet 设置当前钱包
func (uc *UserContext) SetCurrentWallet(walletID string, isUnlocked bool) {
	uc.CurrentWallet = walletID
	uc.IsWalletUnlocked = isUnlocked

	// 如果有解锁的钱包，提升权限级别
	if isUnlocked {
		uc.PermissionLevel = FullAccess
	}
}

// GetDisplayStatus 获取用户状态显示文本
func (uc *UserContext) GetDisplayStatus() string {
	if uc.IsFirstTimeUser {
		return "🆕 首次用户 - 建议先创建钱包"
	}

	switch uc.PermissionLevel {
	case UNKNOWN:
		return "❓ 权限检测中..."
	case SystemOnly:
		if uc.HasWallets {
			return "🔒 钱包已锁定 - 仅系统级功能可用"
		}
		return "📋 系统级功能 - 无钱包访问"
	case FullAccess:
		return fmt.Sprintf("✅ 完全访问 - 钱包: %s", uc.CurrentWallet)
	default:
		return "❓ 未知状态"
	}
}

// CanExecuteUserLevel 检查是否可以执行用户级操作
func (uc *UserContext) CanExecuteUserLevel() bool {
	return uc.PermissionLevel.IsUserLevelAllowed()
}

// CanExecuteSystemLevel 检查是否可以执行系统级操作
func (uc *UserContext) CanExecuteSystemLevel() bool {
	return uc.PermissionLevel.IsSystemLevelAllowed()
}
