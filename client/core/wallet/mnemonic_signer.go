// Package wallet provides wallet functionality for WES blockchain.
package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

// MnemonicSigner 助记词签名器实现
// 实现 Signer 和 KeyDerivation 接口
type MnemonicSigner struct {
	mnemonic      string                           // 助记词（加密存储时清除）
	passphrase    string                           // BIP39 密码
	masterKey     *hdkeychain.ExtendedKey          // 主密钥
	derivedKeys   map[string]*ecdsa.PrivateKey     // 已派生的密钥缓存 (path -> key)
	derivedAddrs  map[string]string                // 已派生的地址缓存 (path -> address)
	addressToPath map[string]string                // 地址到路径的反向映射
	defaultPath   *DerivationPath                  // 默认派生路径
	addressMgr    AddressManager                   // 地址管理器
	mu            sync.RWMutex
	locked        bool
	unlockUntil   time.Time
}

// MnemonicSignerConfig 助记词签名器配置
type MnemonicSignerConfig struct {
	Mnemonic      string          // 助记词
	Passphrase    string          // BIP39 密码（可选）
	DefaultPath   *DerivationPath // 默认派生路径（可选，默认为 m/44'/8888'/0'/0/0）
	AddressManager AddressManager // 地址管理器（必需）
}

// NewMnemonicSigner 创建助记词签名器
func NewMnemonicSigner(config MnemonicSignerConfig) (*MnemonicSigner, error) {
	if config.Mnemonic == "" {
		return nil, errors.New("mnemonic is required")
	}
	if config.AddressManager == nil {
		return nil, errors.New("address manager is required")
	}

	// 验证助记词
	mm := NewMnemonicManager()
	if !mm.ValidateMnemonic(config.Mnemonic) {
		return nil, errors.New("invalid mnemonic")
	}

	// 设置默认路径
	defaultPath := config.DefaultPath
	if defaultPath == nil {
		defaultPath = DefaultDerivationPath()
	}

	signer := &MnemonicSigner{
		mnemonic:      config.Mnemonic,
		passphrase:    config.Passphrase,
		derivedKeys:   make(map[string]*ecdsa.PrivateKey),
		derivedAddrs:  make(map[string]string),
		addressToPath: make(map[string]string),
		defaultPath:   defaultPath,
		addressMgr:    config.AddressManager,
		locked:        true, // 默认锁定
	}

	return signer, nil
}

// NewMnemonicSignerFromNew 创建新钱包（生成新助记词）
func NewMnemonicSignerFromNew(strength MnemonicStrength, passphrase string, addressMgr AddressManager) (*MnemonicSigner, string, error) {
	if addressMgr == nil {
		return nil, "", errors.New("address manager is required")
	}

	mm := NewMnemonicManager()
	mnemonic, err := mm.GenerateMnemonic(strength)
	if err != nil {
		return nil, "", fmt.Errorf("generate mnemonic: %w", err)
	}

	signer, err := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		Passphrase:     passphrase,
		AddressManager: addressMgr,
	})
	if err != nil {
		return nil, "", err
	}

	return signer, mnemonic, nil
}

// initMasterKey 初始化主密钥
func (s *MnemonicSigner) initMasterKey() error {
	if s.masterKey != nil {
		return nil
	}

	// 从助记词生成种子
	mm := NewMnemonicManager()
	seed, err := mm.MnemonicToSeed(s.mnemonic, s.passphrase)
	if err != nil {
		return fmt.Errorf("mnemonic to seed: %w", err)
	}

	// 从种子生成主密钥
	// 使用 Bitcoin mainnet 参数（实际上只用于 HD 派生，不影响地址格式）
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return fmt.Errorf("create master key: %w", err)
	}

	s.masterKey = masterKey
	return nil
}

// Sign 签名交易
func (s *MnemonicSigner) Sign(tx []byte, fromAddr string) ([]byte, error) {
	if s.IsLocked() {
		return nil, errors.New("signer is locked")
	}

	// 查找地址对应的派生路径
	s.mu.RLock()
	path, ok := s.addressToPath[fromAddr]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("address not found: %s", fromAddr)
	}

	// 获取私钥
	privateKey, err := s.DeriveKey(path)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// 计算交易哈希
	hash := sha256.Sum256(tx)

	// ECDSA 签名
	r, sigS, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("ecdsa sign: %w", err)
	}

	// 序列化签名: r || s (64字节)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := sigS.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// SignHash 签名哈希值
func (s *MnemonicSigner) SignHash(hash []byte, fromAddr string) ([]byte, error) {
	if s.IsLocked() {
		return nil, errors.New("signer is locked")
	}

	// 查找地址对应的派生路径
	s.mu.RLock()
	path, ok := s.addressToPath[fromAddr]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("address not found: %s", fromAddr)
	}

	// 获取私钥
	privateKey, err := s.DeriveKey(path)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// ECDSA 签名
	r, sigS, err := ecdsa.Sign(rand.Reader, privateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("ecdsa sign: %w", err)
	}

	// 序列化签名: r || s (64字节)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := sigS.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// GetAddress 获取地址
func (s *MnemonicSigner) GetAddress(derivationPath string) (string, error) {
	if derivationPath == "" {
		derivationPath = s.defaultPath.String()
	}

	// 先检查缓存
	s.mu.RLock()
	if addr, ok := s.derivedAddrs[derivationPath]; ok {
		s.mu.RUnlock()
		return addr, nil
	}
	s.mu.RUnlock()

	// 派生新地址
	return s.DeriveAddress(derivationPath)
}

// ListAddresses 列出所有已派生的地址
func (s *MnemonicSigner) ListAddresses() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]string, 0, len(s.derivedAddrs))
	for _, addr := range s.derivedAddrs {
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// Unlock 解锁签名器
func (s *MnemonicSigner) Unlock(password string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 初始化主密钥
	if err := s.initMasterKey(); err != nil {
		return err
	}

	// 派生默认地址
	if err := s.deriveDefaultAddressUnlocked(); err != nil {
		return fmt.Errorf("derive default address: %w", err)
	}

	s.locked = false

	// 设置解锁时长
	if duration > 0 {
		s.unlockUntil = time.Now().Add(duration)

		// 启动自动锁定
		go func() {
			time.Sleep(duration)
			s.Lock()
		}()
	} else {
		s.unlockUntil = time.Time{}
	}

	return nil
}

// Lock 锁定签名器
func (s *MnemonicSigner) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清除内存中的敏感数据
	for path, key := range s.derivedKeys {
		if key != nil && key.D != nil {
			key.D.SetInt64(0)
		}
		delete(s.derivedKeys, path)
	}

	s.locked = true
	s.unlockUntil = time.Time{}
}

// IsLocked 检查是否锁定
func (s *MnemonicSigner) IsLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.locked {
		return true
	}

	// 检查是否过期
	if !s.unlockUntil.IsZero() && time.Now().After(s.unlockUntil) {
		return true
	}

	return false
}

// Type 返回签名器类型
func (s *MnemonicSigner) Type() SignerType {
	return SignerTypeMnemonic
}

// DeriveKey 派生子密钥（实现 KeyDerivation 接口）
func (s *MnemonicSigner) DeriveKey(path string) (*ecdsa.PrivateKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查缓存
	if key, ok := s.derivedKeys[path]; ok {
		return key, nil
	}

	if s.masterKey == nil {
		return nil, errors.New("master key not initialized, call Unlock first")
	}

	// 解析路径
	dp, err := ParseDerivationPath(path)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}

	// 派生子密钥
	childKey := s.masterKey
	for _, index := range dp.ToUint32Array() {
		childKey, err = childKey.Derive(index)
		if err != nil {
			return nil, fmt.Errorf("derive key: %w", err)
		}
	}

	// 获取私钥
	privKey, err := childKey.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("get private key: %w", err)
	}

	// 转换为 ecdsa.PrivateKey
	ecdsaKey := privKey.ToECDSA()

	// 缓存
	s.derivedKeys[path] = ecdsaKey

	return ecdsaKey, nil
}

// DeriveAddress 派生地址（实现 KeyDerivation 接口）
func (s *MnemonicSigner) DeriveAddress(path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.deriveAddressUnlocked(path)
}

// deriveAddressUnlocked 内部方法：派生地址（调用者需持有锁）
func (s *MnemonicSigner) deriveAddressUnlocked(path string) (string, error) {
	// 检查缓存
	if addr, ok := s.derivedAddrs[path]; ok {
		return addr, nil
	}

	if s.masterKey == nil {
		return "", errors.New("master key not initialized, call Unlock first")
	}

	// 解析路径
	dp, err := ParseDerivationPath(path)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}

	// 派生子密钥
	childKey := s.masterKey
	for _, index := range dp.ToUint32Array() {
		childKey, err = childKey.Derive(index)
		if err != nil {
			return "", fmt.Errorf("derive key: %w", err)
		}
	}

	// 获取私钥
	privKey, err := childKey.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("get private key: %w", err)
	}

	// 使用 AddressManager 生成地址
	privateKeyBytes := privKey.Serialize()
	address, err := s.addressMgr.PrivateKeyToAddress(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("generate address: %w", err)
	}

	// 缓存
	s.derivedAddrs[path] = address
	s.addressToPath[address] = path
	s.derivedKeys[path] = privKey.ToECDSA()

	return address, nil
}

// deriveDefaultAddressUnlocked 派生默认地址（调用者需持有锁）
func (s *MnemonicSigner) deriveDefaultAddressUnlocked() error {
	_, err := s.deriveAddressUnlocked(s.defaultPath.String())
	return err
}

// GetMasterKey 获取主密钥（实现 KeyDerivation 接口）
func (s *MnemonicSigner) GetMasterKey() (*ecdsa.PrivateKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.masterKey == nil {
		return nil, errors.New("master key not initialized")
	}

	privKey, err := s.masterKey.ECPrivKey()
	if err != nil {
		return nil, err
	}

	return privKey.ToECDSA(), nil
}

// GetDerivationPath 获取默认派生路径
func (s *MnemonicSigner) GetDerivationPath() string {
	return s.defaultPath.String()
}

// DeriveMultipleAddresses 批量派生地址
func (s *MnemonicSigner) DeriveMultipleAddresses(account, startIndex, count uint32) ([]string, error) {
	if s.IsLocked() {
		return nil, errors.New("signer is locked")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	addrs := make([]string, 0, count)
	generator := NewHDPathGenerator(account)

	for i := uint32(0); i < count; i++ {
		path := generator.GenerateReceivePath(startIndex + i)
		addr, err := s.deriveAddressUnlocked(path.String())
		if err != nil {
			return nil, fmt.Errorf("derive address at index %d: %w", startIndex+i, err)
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// GetDerivedPaths 获取所有已派生的路径
func (s *MnemonicSigner) GetDerivedPaths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paths := make([]string, 0, len(s.derivedAddrs))
	for path := range s.derivedAddrs {
		paths = append(paths, path)
	}
	return paths
}

// GetAddressForPath 获取指定路径的地址
func (s *MnemonicSigner) GetAddressForPath(path string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addr, ok := s.derivedAddrs[path]
	return addr, ok
}

// GetPathForAddress 获取地址对应的路径
func (s *MnemonicSigner) GetPathForAddress(address string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, ok := s.addressToPath[address]
	return path, ok
}

// ExportExtendedPublicKey 导出扩展公钥（用于观察钱包）
// 导出指定账户的扩展公钥，可用于生成接收地址而无需私钥
func (s *MnemonicSigner) ExportExtendedPublicKey(account uint32) (string, error) {
	if s.IsLocked() {
		return "", errors.New("signer is locked")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	// 派生到账户层级: m/44'/8888'/account'
	path := []uint32{
		BIP44Purpose + HardenedOffset,
		WESCoinType + HardenedOffset,
		account + HardenedOffset,
	}

	key := s.masterKey
	var err error
	for _, index := range path {
		key, err = key.Derive(index)
		if err != nil {
			return "", fmt.Errorf("derive: %w", err)
		}
	}

	// 获取公钥版本
	pubKey, err := key.Neuter()
	if err != nil {
		return "", fmt.Errorf("neuter: %w", err)
	}

	return pubKey.String(), nil
}

// VerifySignature 验证签名
func (s *MnemonicSigner) VerifySignature(hash, signature []byte, address string) (bool, error) {
	s.mu.RLock()
	path, ok := s.addressToPath[address]
	s.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("address not found: %s", address)
	}

	// 获取私钥（用于获取公钥）
	privateKey, err := s.DeriveKey(path)
	if err != nil {
		return false, fmt.Errorf("derive key: %w", err)
	}

	// 解析签名
	if len(signature) != 64 {
		return false, errors.New("invalid signature length")
	}

	r := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:])

	// 验证签名
	return ecdsa.Verify(&privateKey.PublicKey, hash, r, sigS), nil
}

// GetPrivateKeyBytes 获取指定地址的私钥字节（用于导出）
// 警告：此方法会暴露私钥，仅在必要时使用
func (s *MnemonicSigner) GetPrivateKeyBytes(address string) ([]byte, error) {
	if s.IsLocked() {
		return nil, errors.New("signer is locked")
	}

	s.mu.RLock()
	path, ok := s.addressToPath[address]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("address not found: %s", address)
	}

	privateKey, err := s.DeriveKey(path)
	if err != nil {
		return nil, err
	}

	// 使用 btcec 进行序列化（确保是 32 字节）
	btcecPrivKey, _ := btcec.PrivKeyFromBytes(privateKey.D.Bytes())
	return btcecPrivKey.Serialize(), nil
}

// 确保实现了接口
var _ Signer = (*MnemonicSigner)(nil)
var _ KeyDerivation = (*MnemonicSigner)(nil)

