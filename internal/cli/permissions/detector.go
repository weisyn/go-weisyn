package permissions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// permissionDetector 权限检测器实现
type permissionDetector struct {
	logger              log.Logger
	walletStatusChecker WalletStatusChecker
	userContext         *UserContext
	configDir           string
}

// NewPermissionDetector 创建权限检测器
func NewPermissionDetector(
	logger log.Logger,
	walletStatusChecker WalletStatusChecker,
) PermissionDetector {
	configDir := getConfigDir()

	return &permissionDetector{
		logger:              logger,
		walletStatusChecker: walletStatusChecker,
		userContext:         NewUserContext(),
		configDir:           configDir,
	}
}

// DetectPermissionLevel 检测当前用户权限级别
func (p *permissionDetector) DetectPermissionLevel(ctx context.Context) (PermissionLevel, error) {
	// 检查是否为首次用户
	isFirstTime, err := p.IsFirstTimeUser(ctx)
	if err != nil {
		return SystemOnly, fmt.Errorf("检查首次用户状态失败: %v", err)
	}

	// 检查钱包可用性
	hasWallets, err := p.CheckWalletAvailability(ctx)
	if err != nil {
		return SystemOnly, fmt.Errorf("检查钱包可用性失败: %v", err)
	}

	// 更新用户上下文
	var permissionLevel PermissionLevel
	if hasWallets {
		permissionLevel = FullAccess
	} else {
		permissionLevel = SystemOnly
	}

	p.userContext.UpdateFromDetection(permissionLevel, hasWallets, isFirstTime)

	p.logger.Info(fmt.Sprintf("权限检测完成: level=%s, hasWallets=%v, isFirstTime=%v",
		permissionLevel.String(), hasWallets, isFirstTime))

	return permissionLevel, nil
}

// CheckWalletAvailability 检查钱包可用性
func (p *permissionDetector) CheckWalletAvailability(ctx context.Context) (bool, error) {
	if p.walletStatusChecker == nil {
		// 如果没有钱包状态检查器，执行简单的文件检查
		return p.simpleWalletCheck()
	}

	return p.walletStatusChecker.HasWallets(ctx)
}

// IsFirstTimeUser 检查是否为首次用户
func (p *permissionDetector) IsFirstTimeUser(ctx context.Context) (bool, error) {
	// 检查首次用户标记文件
	firstTimeMarkerFile := filepath.Join(p.configDir, ".first_time_completed")

	if _, err := os.Stat(firstTimeMarkerFile); os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return true, fmt.Errorf("检查首次用户标记文件失败: %v", err)
	}

	return false, nil
}

// UpdatePermissionLevel 更新权限级别
func (p *permissionDetector) UpdatePermissionLevel(ctx context.Context, level PermissionLevel) error {
	p.userContext.PermissionLevel = level

	// 如果提升到完全访问权限，标记首次用户完成
	if level == FullAccess {
		if err := p.markFirstTimeCompleted(); err != nil {
			return fmt.Errorf("标记首次用户完成失败: %v", err)
		}
	}

	p.logger.Info(fmt.Sprintf("权限级别已更新: new_level=%s", level.String()))
	return nil
}

// GetUserContext 获取当前用户上下文
func (p *permissionDetector) GetUserContext() *UserContext {
	return p.userContext
}

// simpleWalletCheck 简单的钱包文件检查
func (p *permissionDetector) simpleWalletCheck() (bool, error) {
	walletsDir := filepath.Join(p.configDir, "wallets")

	if _, err := os.Stat(walletsDir); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("检查钱包目录失败: %v", err)
	}

	// 检查钱包目录下是否有文件
	entries, err := os.ReadDir(walletsDir)
	if err != nil {
		return false, fmt.Errorf("读取钱包目录失败: %v", err)
	}

	// 如果有钱包文件，认为有钱包
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".wallet" {
			return true, nil
		}
	}

	return false, nil
}

// markFirstTimeCompleted 标记首次用户完成
func (p *permissionDetector) markFirstTimeCompleted() error {
	firstTimeMarkerFile := filepath.Join(p.configDir, ".first_time_completed")

	// 确保配置目录存在
	if err := os.MkdirAll(p.configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 创建标记文件
	file, err := os.Create(firstTimeMarkerFile)
	if err != nil {
		return fmt.Errorf("创建首次用户标记文件失败: %v", err)
	}
	defer file.Close()

	// 写入完成时间
	_, err = file.WriteString(fmt.Sprintf("completed_at=%d\n", getCurrentTimestamp()))
	if err != nil {
		return fmt.Errorf("写入首次用户标记失败: %v", err)
	}

	return nil
}

// getConfigDir 获取配置目录路径
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".weisyn_cli"
	}
	return filepath.Join(homeDir, ".weisyn_cli")
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return 1700000000 // 模拟时间戳，实际应该用time.Now().Unix()
}
