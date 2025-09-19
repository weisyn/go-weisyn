package permissions

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Manager 权限管理器，整合所有权限相关功能
type Manager struct {
	logger        log.Logger
	detector      PermissionDetector
	walletChecker WalletStatusChecker
	userContext   *UserContext
}

// NewManager 创建权限管理器
func NewManager(logger log.Logger) *Manager {
	walletChecker := NewWalletStatusChecker(logger)
	detector := NewPermissionDetector(logger, walletChecker)

	return &Manager{
		logger:        logger,
		detector:      detector,
		walletChecker: walletChecker,
		userContext:   NewUserContext(),
	}
}

// Initialize 初始化权限系统
func (m *Manager) Initialize(ctx context.Context) error {
	m.logger.Info("初始化权限系统...")

	// 检测用户权限级别
	permissionLevel, err := m.detector.DetectPermissionLevel(ctx)
	if err != nil {
		return fmt.Errorf("权限检测失败: %v", err)
	}

	// 检查钱包可用性
	hasWallets, err := m.detector.CheckWalletAvailability(ctx)
	if err != nil {
		return fmt.Errorf("钱包可用性检查失败: %v", err)
	}

	// 检查是否为首次用户
	isFirstTime, err := m.detector.IsFirstTimeUser(ctx)
	if err != nil {
		return fmt.Errorf("首次用户检查失败: %v", err)
	}

	// 更新用户上下文
	m.userContext.UpdateFromDetection(permissionLevel, hasWallets, isFirstTime)

	m.logger.Info(fmt.Sprintf("权限系统初始化完成: level=%s, hasWallets=%v, isFirstTime=%v",
		permissionLevel.String(), hasWallets, isFirstTime))

	return nil
}

// GetUserContext 获取用户上下文
func (m *Manager) GetUserContext() *UserContext {
	return m.userContext
}

// CanExecuteSystemLevel 检查是否可以执行系统级操作
func (m *Manager) CanExecuteSystemLevel() bool {
	return m.userContext.CanExecuteSystemLevel()
}

// CanExecuteUserLevel 检查是否可以执行用户级操作
func (m *Manager) CanExecuteUserLevel() bool {
	return m.userContext.CanExecuteUserLevel()
}

// RequireUserLevel 要求用户级权限，如果不满足返回错误
func (m *Manager) RequireUserLevel() error {
	if !m.CanExecuteUserLevel() {
		return fmt.Errorf("此操作需要用户级权限，请先创建并解锁钱包")
	}
	return nil
}

// RequireSystemLevel 要求系统级权限
func (m *Manager) RequireSystemLevel() error {
	if !m.CanExecuteSystemLevel() {
		return fmt.Errorf("此操作需要系统级权限")
	}
	return nil
}

// CreateWallet 创建钱包并更新权限
func (m *Manager) CreateWallet(ctx context.Context, name, address string) error {
	// 使用钱包检查器创建钱包
	wallet, err := m.walletChecker.(*walletStatusChecker).CreateWallet(ctx, name, address)
	if err != nil {
		return fmt.Errorf("创建钱包失败: %v", err)
	}

	// 更新用户上下文
	m.userContext.HasWallets = true
	m.userContext.PermissionLevel = FullAccess
	m.userContext.SetCurrentWallet(wallet.ID, false)

	// 更新权限检测器
	if err := m.detector.UpdatePermissionLevel(ctx, FullAccess); err != nil {
		m.logger.Info(fmt.Sprintf("更新权限级别失败: %v", err))
	}

	m.logger.Info(fmt.Sprintf("钱包创建完成，权限已提升: name=%s, id=%s", name, wallet.ID))

	return nil
}

// UnlockWallet 解锁钱包并更新上下文
func (m *Manager) UnlockWallet(ctx context.Context, walletID, password string) error {
	// 解锁钱包
	if err := m.walletChecker.(*walletStatusChecker).UnlockWallet(ctx, walletID, password); err != nil {
		return fmt.Errorf("解锁钱包失败: %v", err)
	}

	// 更新用户上下文
	m.userContext.SetCurrentWallet(walletID, true)
	m.userContext.PermissionLevel = FullAccess

	m.logger.Info(fmt.Sprintf("钱包解锁成功: wallet_id=%s", walletID))
	return nil
}

// LockWallet 锁定钱包并更新上下文
func (m *Manager) LockWallet(ctx context.Context, walletID string) error {
	// 锁定钱包
	if err := m.walletChecker.(*walletStatusChecker).LockWallet(ctx, walletID); err != nil {
		return fmt.Errorf("锁定钱包失败: %v", err)
	}

	// 更新用户上下文
	if m.userContext.CurrentWallet == walletID {
		m.userContext.SetCurrentWallet("", false)
		// 如果还有其他钱包，保持SystemOnly权限，否则降级
		if m.userContext.HasWallets {
			m.userContext.PermissionLevel = SystemOnly
		}
	}

	m.logger.Info(fmt.Sprintf("钱包已锁定: wallet_id=%s", walletID))
	return nil
}

// GetAvailableWallets 获取可用钱包列表
func (m *Manager) GetAvailableWallets(ctx context.Context) ([]WalletInfo, error) {
	wallets, err := m.walletChecker.(*walletStatusChecker).GetAvailableWallets(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为非指针类型
	result := make([]WalletInfo, len(wallets))
	for i, wallet := range wallets {
		result[i] = *wallet
	}

	return result, nil
}

// RefreshPermissions 刷新权限状态
func (m *Manager) RefreshPermissions(ctx context.Context) error {
	return m.Initialize(ctx)
}

// GetPermissionLevel 获取当前权限级别
func (m *Manager) GetPermissionLevel() PermissionLevel {
	return m.userContext.PermissionLevel
}

// GetStatusDisplay 获取权限状态显示文本
func (m *Manager) GetStatusDisplay() string {
	return m.userContext.GetDisplayStatus()
}
