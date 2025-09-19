package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// WalletStatusChecker 钱包状态检查器接口
type WalletStatusChecker interface {
	// HasWallets 检查是否存在钱包
	HasWallets(ctx context.Context) (bool, error)

	// GetWalletCount 获取钱包数量
	GetWalletCount(ctx context.Context) (int, error)

	// IsWalletUnlocked 检查钱包是否已解锁
	IsWalletUnlocked(ctx context.Context, walletID string) (bool, error)

	// GetDefaultWallet 获取默认钱包ID
	GetDefaultWallet(ctx context.Context) (string, error)

	// GetAvailableWallets 获取所有可用钱包
	GetAvailableWallets(ctx context.Context) ([]*WalletInfo, error)

	// CreateWallet 创建新钱包
	CreateWallet(ctx context.Context, name, address string) (*WalletInfo, error)

	// UnlockWallet 解锁钱包
	UnlockWallet(ctx context.Context, walletID, password string) error

	// LockWallet 锁定钱包
	LockWallet(ctx context.Context, walletID string) error
}

// walletStatusChecker 钱包状态检查器实现
type walletStatusChecker struct {
	logger    log.Logger
	configDir string
}

// WalletInfo 钱包信息
type WalletInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	CreatedAt  int64  `json:"created_at"`
	IsDefault  bool   `json:"is_default"`
	IsUnlocked bool   `json:"is_unlocked"`
}

// NewWalletStatusChecker 创建钱包状态检查器
func NewWalletStatusChecker(logger log.Logger) WalletStatusChecker {
	configDir := getConfigDir()

	return &walletStatusChecker{
		logger:    logger,
		configDir: configDir,
	}
}

// HasWallets 检查是否存在钱包
func (w *walletStatusChecker) HasWallets(ctx context.Context) (bool, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		return false, fmt.Errorf("加载钱包信息失败: %v", err)
	}

	return len(wallets) > 0, nil
}

// GetWalletCount 获取钱包数量
func (w *walletStatusChecker) GetWalletCount(ctx context.Context) (int, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		return 0, fmt.Errorf("加载钱包信息失败: %v", err)
	}

	return len(wallets), nil
}

// IsWalletUnlocked 检查钱包是否已解锁
func (w *walletStatusChecker) IsWalletUnlocked(ctx context.Context, walletID string) (bool, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		return false, fmt.Errorf("加载钱包信息失败: %v", err)
	}

	for _, wallet := range wallets {
		if wallet.ID == walletID {
			return wallet.IsUnlocked, nil
		}
	}

	return false, fmt.Errorf("钱包不存在: %s", walletID)
}

// GetDefaultWallet 获取默认钱包ID
func (w *walletStatusChecker) GetDefaultWallet(ctx context.Context) (string, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		return "", fmt.Errorf("加载钱包信息失败: %v", err)
	}

	if len(wallets) == 0 {
		return "", fmt.Errorf("没有可用的钱包")
	}

	// 查找默认钱包
	for _, wallet := range wallets {
		if wallet.IsDefault {
			return wallet.ID, nil
		}
	}

	// 如果没有设置默认钱包，返回第一个钱包
	return wallets[0].ID, nil
}

// GetAvailableWallets 获取所有可用钱包
func (w *walletStatusChecker) GetAvailableWallets(ctx context.Context) ([]*WalletInfo, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		return nil, fmt.Errorf("加载钱包信息失败: %v", err)
	}

	return wallets, nil
}

// CreateWallet 创建新钱包（简化版本）
func (w *walletStatusChecker) CreateWallet(ctx context.Context, name, address string) (*WalletInfo, error) {
	wallets, err := w.loadWallets()
	if err != nil {
		// 如果加载失败，可能是第一次使用，创建空列表
		wallets = []*WalletInfo{}
	}

	// 生成钱包ID
	walletID := generateWalletID(name)

	// 检查是否是第一个钱包
	isDefault := len(wallets) == 0

	// 创建钱包信息
	wallet := &WalletInfo{
		ID:         walletID,
		Name:       name,
		Address:    address,
		CreatedAt:  getCurrentTimestamp(),
		IsDefault:  isDefault,
		IsUnlocked: false,
	}

	// 添加到钱包列表
	wallets = append(wallets, wallet)

	// 保存钱包信息
	if err := w.saveWallets(wallets); err != nil {
		return nil, fmt.Errorf("保存钱包信息失败: %v", err)
	}

	w.logger.Info(fmt.Sprintf("钱包创建成功: id=%s, name=%s, address=%s, isDefault=%v",
		walletID, name, address, isDefault))

	return wallet, nil
}

// UnlockWallet 解锁钱包
func (w *walletStatusChecker) UnlockWallet(ctx context.Context, walletID, password string) error {
	wallets, err := w.loadWallets()
	if err != nil {
		return fmt.Errorf("加载钱包信息失败: %v", err)
	}

	// 查找钱包
	for i, wallet := range wallets {
		if wallet.ID == walletID {
			// 这里应该验证密码，简化版本直接设置为解锁
			// TODO: 实际实现应该验证密码和解密私钥
			wallets[i].IsUnlocked = true

			// 保存更新后的钱包状态
			if err := w.saveWallets(wallets); err != nil {
				return fmt.Errorf("保存钱包状态失败: %v", err)
			}

			w.logger.Info(fmt.Sprintf("钱包解锁成功: wallet_id=%s", walletID))
			return nil
		}
	}

	return fmt.Errorf("钱包不存在: %s", walletID)
}

// LockWallet 锁定钱包
func (w *walletStatusChecker) LockWallet(ctx context.Context, walletID string) error {
	wallets, err := w.loadWallets()
	if err != nil {
		return fmt.Errorf("加载钱包信息失败: %v", err)
	}

	// 查找钱包
	for i, wallet := range wallets {
		if wallet.ID == walletID {
			wallets[i].IsUnlocked = false

			// 保存更新后的钱包状态
			if err := w.saveWallets(wallets); err != nil {
				return fmt.Errorf("保存钱包状态失败: %v", err)
			}

			w.logger.Info(fmt.Sprintf("钱包已锁定: wallet_id=%s", walletID))
			return nil
		}
	}

	return fmt.Errorf("钱包不存在: %s", walletID)
}

// loadWallets 从文件加载钱包信息
func (w *walletStatusChecker) loadWallets() ([]*WalletInfo, error) {
	walletsFile := filepath.Join(w.configDir, "wallets.json")

	if _, err := os.Stat(walletsFile); os.IsNotExist(err) {
		return []*WalletInfo{}, nil
	}

	data, err := os.ReadFile(walletsFile)
	if err != nil {
		return nil, fmt.Errorf("读取钱包文件失败: %v", err)
	}

	var wallets []*WalletInfo
	if err := json.Unmarshal(data, &wallets); err != nil {
		return nil, fmt.Errorf("解析钱包文件失败: %v", err)
	}

	// 按创建时间排序
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].CreatedAt < wallets[j].CreatedAt
	})

	return wallets, nil
}

// saveWallets 保存钱包信息到文件
func (w *walletStatusChecker) saveWallets(wallets []*WalletInfo) error {
	// 确保配置目录存在
	if err := os.MkdirAll(w.configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	walletsFile := filepath.Join(w.configDir, "wallets.json")

	data, err := json.MarshalIndent(wallets, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化钱包信息失败: %v", err)
	}

	if err := os.WriteFile(walletsFile, data, 0600); err != nil {
		return fmt.Errorf("写入钱包文件失败: %v", err)
	}

	return nil
}

// generateWalletID 生成钱包ID
func generateWalletID(name string) string {
	// 简化版本：使用名称和时间戳生成ID
	timestamp := getCurrentTimestamp()
	cleanName := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	return fmt.Sprintf("%s_%d", cleanName, timestamp%10000)
}
