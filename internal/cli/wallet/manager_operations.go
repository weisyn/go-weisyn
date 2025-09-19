package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// ListWallets 列出所有钱包
func (w *walletManager) ListWallets(ctx context.Context) ([]*WalletInfo, error) {
	storage, err := w.loadStorage()
	if err != nil {
		if os.IsNotExist(err) {
			return []*WalletInfo{}, nil
		}
		return nil, fmt.Errorf("加载钱包存储失败: %v", err)
	}

	wallets := make([]*WalletInfo, 0, len(storage.Wallets))
	for _, encrypted := range storage.Wallets {
		wallet := &WalletInfo{
			ID:           encrypted.ID,
			Name:         encrypted.Name,
			Address:      encrypted.Address,
			CreatedAt:    time.Unix(encrypted.CreatedAt, 0),
			UpdatedAt:    time.Unix(encrypted.UpdatedAt, 0),
			IsDefault:    encrypted.IsDefault,
			IsUnlocked:   w.isWalletUnlockedInMemory(encrypted.ID),
			KeystorePath: w.getStorageFilePath(),
		}
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}

// GetWallet 获取指定钱包信息
func (w *walletManager) GetWallet(ctx context.Context, walletID string) (*WalletInfo, error) {
	storage, err := w.loadStorage()
	if err != nil {
		return nil, fmt.Errorf("加载钱包存储失败: %v", err)
	}

	for _, encrypted := range storage.Wallets {
		if encrypted.ID == walletID {
			wallet := &WalletInfo{
				ID:           encrypted.ID,
				Name:         encrypted.Name,
				Address:      encrypted.Address,
				CreatedAt:    time.Unix(encrypted.CreatedAt, 0),
				UpdatedAt:    time.Unix(encrypted.UpdatedAt, 0),
				IsDefault:    encrypted.IsDefault,
				IsUnlocked:   w.isWalletUnlockedInMemory(encrypted.ID),
				KeystorePath: w.getStorageFilePath(),
			}
			return wallet, nil
		}
	}

	return nil, fmt.Errorf("钱包不存在: %s", walletID)
}

// DeleteWallet 删除钱包
func (w *walletManager) DeleteWallet(ctx context.Context, walletID string) error {
	w.logger.Info(fmt.Sprintf("删除钱包: id=%s", walletID))

	storage, err := w.loadStorage()
	if err != nil {
		return fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找并删除钱包
	found := false
	newWallets := make([]*EncryptedWallet, 0, len(storage.Wallets))
	var deletedWallet *EncryptedWallet

	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			found = true
			deletedWallet = wallet

			// 从内存中清除解锁状态
			delete(w.unlockedWallets, walletID)
		} else {
			newWallets = append(newWallets, wallet)
		}
	}

	if !found {
		return fmt.Errorf("钱包不存在: %s", walletID)
	}

	// 如果删除的是默认钱包，需要重新设置默认钱包
	if deletedWallet.IsDefault && len(newWallets) > 0 {
		newWallets[0].IsDefault = true
		newWallets[0].UpdatedAt = time.Now().Unix()
	}

	// 更新存储
	storage.Wallets = newWallets
	storage.UpdatedAt = time.Now().Unix()

	if err := w.saveStorage(storage); err != nil {
		return fmt.Errorf("保存钱包存储失败: %v", err)
	}

	w.logger.Info(fmt.Sprintf("钱包删除成功: id=%s", walletID))
	return nil
}

// UnlockWallet 解锁钱包
func (w *walletManager) UnlockWallet(ctx context.Context, walletID, password string) error {
	w.logger.Info(fmt.Sprintf("解锁钱包: id=%s", walletID))

	// 检查是否已经解锁
	if w.isWalletUnlockedInMemory(walletID) {
		return nil // 已经解锁
	}

	storage, err := w.loadStorage()
	if err != nil {
		return fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找钱包
	var encryptedWallet *EncryptedWallet
	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			encryptedWallet = wallet
			break
		}
	}

	if encryptedWallet == nil {
		return fmt.Errorf("钱包不存在: %s", walletID)
	}

	// 解密私钥
	privateKey, err := w.decryptPrivateKey(encryptedWallet.EncryptedKey, encryptedWallet.Salt, password)
	if err != nil {
		return fmt.Errorf("密码错误或解密失败: %v", err)
	}

	// 存储到内存中
	now := time.Now()
	w.unlockedWallets[walletID] = &unlockedWalletData{
		PrivateKey:   privateKey,
		UnlockedAt:   now,
		LastAccessAt: now,
	}

	w.logger.Info(fmt.Sprintf("钱包解锁成功: id=%s", walletID))
	return nil
}

// LockWallet 锁定钱包
func (w *walletManager) LockWallet(ctx context.Context, walletID string) error {
	w.logger.Info(fmt.Sprintf("锁定钱包: id=%s", walletID))

	// 从内存中删除解锁数据
	delete(w.unlockedWallets, walletID)

	w.logger.Info(fmt.Sprintf("钱包锁定成功: id=%s", walletID))
	return nil
}

// IsWalletUnlocked 检查钱包是否已解锁
func (w *walletManager) IsWalletUnlocked(ctx context.Context, walletID string) (bool, error) {
	return w.isWalletUnlockedInMemory(walletID), nil
}

// ChangePassword 修改钱包密码
func (w *walletManager) ChangePassword(ctx context.Context, walletID, oldPassword, newPassword string) error {
	w.logger.Info(fmt.Sprintf("修改钱包密码: id=%s", walletID))

	storage, err := w.loadStorage()
	if err != nil {
		return fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找钱包
	var encryptedWallet *EncryptedWallet
	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			encryptedWallet = wallet
			break
		}
	}

	if encryptedWallet == nil {
		return fmt.Errorf("钱包不存在: %s", walletID)
	}

	// 先用旧密码解密私钥
	privateKey, err := w.decryptPrivateKey(encryptedWallet.EncryptedKey, encryptedWallet.Salt, oldPassword)
	if err != nil {
		return fmt.Errorf("旧密码错误: %v", err)
	}

	// 生成新盐值和用新密码加密
	newSalt := w.generateSalt()
	newEncryptedKey, err := w.encryptPrivateKey(privateKey, newSalt, newPassword)
	if err != nil {
		return fmt.Errorf("重新加密私钥失败: %v", err)
	}

	// 更新钱包记录
	encryptedWallet.EncryptedKey = newEncryptedKey
	encryptedWallet.Salt = newSalt
	encryptedWallet.UpdatedAt = time.Now().Unix()

	// 保存到文件
	if err := w.saveStorage(storage); err != nil {
		return fmt.Errorf("保存钱包存储失败: %v", err)
	}

	// 如果钱包当前已解锁，更新内存中的数据
	if unlockedData, exists := w.unlockedWallets[walletID]; exists {
		unlockedData.LastAccessAt = time.Now()
	}

	w.logger.Info(fmt.Sprintf("钱包密码修改成功: id=%s", walletID))
	return nil
}

// SetDefaultWallet 设置默认钱包
func (w *walletManager) SetDefaultWallet(ctx context.Context, walletID string) error {
	w.logger.Info(fmt.Sprintf("设置默认钱包: id=%s", walletID))

	storage, err := w.loadStorage()
	if err != nil {
		return fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找目标钱包并清除其他默认标记
	found := false
	now := time.Now().Unix()

	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			wallet.IsDefault = true
			wallet.UpdatedAt = now
			found = true
		} else if wallet.IsDefault {
			wallet.IsDefault = false
			wallet.UpdatedAt = now
		}
	}

	if !found {
		return fmt.Errorf("钱包不存在: %s", walletID)
	}

	// 保存更新
	storage.UpdatedAt = now
	if err := w.saveStorage(storage); err != nil {
		return fmt.Errorf("保存钱包存储失败: %v", err)
	}

	w.logger.Info(fmt.Sprintf("默认钱包设置成功: id=%s", walletID))
	return nil
}

// GetDefaultWallet 获取默认钱包
func (w *walletManager) GetDefaultWallet(ctx context.Context) (*WalletInfo, error) {
	storage, err := w.loadStorage()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("没有钱包")
		}
		return nil, fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找默认钱包
	for _, encrypted := range storage.Wallets {
		if encrypted.IsDefault {
			wallet := &WalletInfo{
				ID:           encrypted.ID,
				Name:         encrypted.Name,
				Address:      encrypted.Address,
				CreatedAt:    time.Unix(encrypted.CreatedAt, 0),
				UpdatedAt:    time.Unix(encrypted.UpdatedAt, 0),
				IsDefault:    encrypted.IsDefault,
				IsUnlocked:   w.isWalletUnlockedInMemory(encrypted.ID),
				KeystorePath: w.getStorageFilePath(),
			}
			return wallet, nil
		}
	}

	// 如果没有默认钱包但有钱包，返回第一个
	if len(storage.Wallets) > 0 {
		encrypted := storage.Wallets[0]
		wallet := &WalletInfo{
			ID:           encrypted.ID,
			Name:         encrypted.Name,
			Address:      encrypted.Address,
			CreatedAt:    time.Unix(encrypted.CreatedAt, 0),
			UpdatedAt:    time.Unix(encrypted.UpdatedAt, 0),
			IsDefault:    false,
			IsUnlocked:   w.isWalletUnlockedInMemory(encrypted.ID),
			KeystorePath: w.getStorageFilePath(),
		}
		return wallet, nil
	}

	return nil, fmt.Errorf("没有可用的钱包")
}

// ValidatePassword 验证钱包密码
func (w *walletManager) ValidatePassword(ctx context.Context, walletID, password string) (bool, error) {
	storage, err := w.loadStorage()
	if err != nil {
		return false, fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找钱包
	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			// 尝试解密私钥验证密码
			_, err := w.decryptPrivateKey(wallet.EncryptedKey, wallet.Salt, password)
			return err == nil, nil
		}
	}

	return false, fmt.Errorf("钱包不存在: %s", walletID)
}

// ValidatePrivateKey 验证私钥格式
func (w *walletManager) ValidatePrivateKey(privateKey string) (bool, string, error) {
	// 简化版验证：检查是否为64字符的十六进制字符串
	if len(privateKey) != 64 {
		return false, "", fmt.Errorf("私钥长度不正确，应为64个字符")
	}

	// 检查是否为有效的十六进制
	for _, char := range privateKey {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false, "", fmt.Errorf("私钥包含非十六进制字符")
		}
	}

	// 根据私钥生成地址（简化版本）
	keyBytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		fmt.Sscanf(privateKey[i*2:i*2+2], "%02x", &keyBytes[i])
	}

	hash := sha256.Sum256(keyBytes)
	address := "Cf" + hex.EncodeToString(hash[:16])

	return true, address, nil
}

// isWalletUnlockedInMemory 检查钱包是否在内存中已解锁
func (w *walletManager) isWalletUnlockedInMemory(walletID string) bool {
	_, exists := w.unlockedWallets[walletID]
	return exists
}

// CleanupExpiredSessions 清理过期会话（应定期调用）
func (w *walletManager) CleanupExpiredSessions() {
	now := time.Now()
	expiredDuration := 30 * time.Minute // 30分钟超时

	for walletID, data := range w.unlockedWallets {
		if now.Sub(data.LastAccessAt) > expiredDuration {
			delete(w.unlockedWallets, walletID)
			w.logger.Info(fmt.Sprintf("清理过期会话: wallet_id=%s", walletID))
		}
	}
}

// GetPrivateKey 获取钱包私钥（需要密码验证）
func (w *walletManager) GetPrivateKey(ctx context.Context, walletID, password string) ([]byte, error) {
	// 加载钱包存储
	storage, err := w.loadStorage()
	if err != nil {
		return nil, fmt.Errorf("加载钱包存储失败: %v", err)
	}

	// 查找钱包
	var targetWallet *EncryptedWallet
	for _, wallet := range storage.Wallets {
		if wallet.ID == walletID {
			targetWallet = wallet
			break
		}
	}

	if targetWallet == nil {
		return nil, fmt.Errorf("钱包不存在")
	}

	// 解密私钥
	privateKey, err := w.decryptPrivateKey(targetWallet.EncryptedKey, targetWallet.Salt, password)
	if err != nil {
		return nil, fmt.Errorf("密码错误或私钥解密失败: %v", err)
	}

	// 转换私钥为字节数组
	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return nil, fmt.Errorf("私钥格式转换失败: %v", err)
	}

	return privateKeyBytes, nil
}
