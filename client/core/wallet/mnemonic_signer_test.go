package wallet

import (
	"crypto/ecdsa"
	"testing"
	"time"
)

// mockAddressManager 用于测试的 mock AddressManager
type mockAddressManager struct{}

func (m *mockAddressManager) PrivateKeyToAddress(privateKey []byte) (string, error) {
	// 简单的 mock 实现：返回私钥前8字节的十六进制作为地址
	if len(privateKey) < 8 {
		return "", nil
	}
	addr := "CU"
	for i := 0; i < 8; i++ {
		addr += string("0123456789abcdef"[privateKey[i]>>4])
		addr += string("0123456789abcdef"[privateKey[i]&0x0f])
	}
	return addr, nil
}

func TestNewMnemonicSigner(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	tests := []struct {
		name    string
		config  MnemonicSignerConfig
		wantErr bool
	}{
		{
			"valid config",
			MnemonicSignerConfig{
				Mnemonic:       mnemonic,
				AddressManager: addrMgr,
			},
			false,
		},
		{
			"empty mnemonic",
			MnemonicSignerConfig{
				Mnemonic:       "",
				AddressManager: addrMgr,
			},
			true,
		},
		{
			"nil address manager",
			MnemonicSignerConfig{
				Mnemonic:       mnemonic,
				AddressManager: nil,
			},
			true,
		},
		{
			"invalid mnemonic",
			MnemonicSignerConfig{
				Mnemonic:       "invalid mnemonic words",
				AddressManager: addrMgr,
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMnemonicSigner(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMnemonicSigner() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewMnemonicSignerFromNew(t *testing.T) {
	addrMgr := &mockAddressManager{}

	signer, mnemonic, err := NewMnemonicSignerFromNew(Mnemonic12Words, "", addrMgr)
	if err != nil {
		t.Fatalf("NewMnemonicSignerFromNew() error = %v", err)
	}

	if signer == nil {
		t.Fatal("NewMnemonicSignerFromNew() returned nil signer")
	}

	if mnemonic == "" {
		t.Error("NewMnemonicSignerFromNew() returned empty mnemonic")
	}

	// 验证助记词
	mm := NewMnemonicManager()
	if !mm.ValidateMnemonic(mnemonic) {
		t.Error("NewMnemonicSignerFromNew() returned invalid mnemonic")
	}
}

func TestMnemonicSigner_UnlockLock(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	// 初始状态应该是锁定的
	if !signer.IsLocked() {
		t.Error("signer should be locked initially")
	}

	// 解锁
	if err := signer.Unlock("", 0); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if signer.IsLocked() {
		t.Error("signer should be unlocked after Unlock()")
	}

	// 锁定
	signer.Lock()
	if !signer.IsLocked() {
		t.Error("signer should be locked after Lock()")
	}
}

func TestMnemonicSigner_UnlockWithDuration(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	// 解锁 100ms
	if err := signer.Unlock("", 100*time.Millisecond); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if signer.IsLocked() {
		t.Error("signer should be unlocked immediately after Unlock()")
	}

	// 等待自动锁定
	time.Sleep(150 * time.Millisecond)

	if !signer.IsLocked() {
		t.Error("signer should be automatically locked after duration")
	}
}

func TestMnemonicSigner_DeriveKey(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	// 未解锁时应该失败
	_, err := signer.DeriveKey(WESDefaultPath())
	if err == nil {
		t.Error("DeriveKey() should fail when locked")
	}

	// 解锁
	signer.Unlock("", 0)

	// 派生密钥
	key, err := signer.DeriveKey(WESDefaultPath())
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	if key == nil {
		t.Fatal("DeriveKey() returned nil key")
	}

	// 验证是有效的 ECDSA 私钥
	if key.D == nil || key.D.Sign() == 0 {
		t.Error("DeriveKey() returned invalid private key")
	}
}

func TestMnemonicSigner_DeriveAddress(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	// 解锁
	signer.Unlock("", 0)

	// 派生地址
	addr, err := signer.DeriveAddress(WESDefaultPath())
	if err != nil {
		t.Fatalf("DeriveAddress() error = %v", err)
	}

	if addr == "" {
		t.Error("DeriveAddress() returned empty address")
	}

	// 相同路径应该返回相同地址
	addr2, _ := signer.DeriveAddress(WESDefaultPath())
	if addr != addr2 {
		t.Error("DeriveAddress() should return same address for same path")
	}

	// 不同路径应该返回不同地址
	addr3, _ := signer.DeriveAddress("m/44'/8888'/0'/0/1")
	if addr == addr3 {
		t.Error("DeriveAddress() should return different address for different path")
	}
}

func TestMnemonicSigner_GetAddress(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 空路径应该使用默认路径
	addr1, err := signer.GetAddress("")
	if err != nil {
		t.Fatalf("GetAddress('') error = %v", err)
	}

	addr2, _ := signer.GetAddress(WESDefaultPath())
	if addr1 != addr2 {
		t.Error("GetAddress('') should return same as GetAddress(defaultPath)")
	}
}

func TestMnemonicSigner_ListAddresses(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 派生一些地址
	signer.DeriveAddress(WESDefaultPath())
	signer.DeriveAddress("m/44'/8888'/0'/0/1")

	addrs, err := signer.ListAddresses()
	if err != nil {
		t.Fatalf("ListAddresses() error = %v", err)
	}

	if len(addrs) < 2 {
		t.Errorf("ListAddresses() returned %d addresses, want at least 2", len(addrs))
	}
}

func TestMnemonicSigner_Type(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	if signer.Type() != SignerTypeMnemonic {
		t.Errorf("Type() = %v, want %v", signer.Type(), SignerTypeMnemonic)
	}
}

func TestMnemonicSigner_SignHash(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 获取地址
	addr, _ := signer.GetAddress("")

	// 签名
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}

	sig, err := signer.SignHash(hash, addr)
	if err != nil {
		t.Fatalf("SignHash() error = %v", err)
	}

	if len(sig) != 64 {
		t.Errorf("SignHash() signature length = %d, want 64", len(sig))
	}
}

func TestMnemonicSigner_Sign(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 获取地址
	addr, _ := signer.GetAddress("")

	// 签名交易数据
	tx := []byte("test transaction data")

	sig, err := signer.Sign(tx, addr)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if len(sig) != 64 {
		t.Errorf("Sign() signature length = %d, want 64", len(sig))
	}

	// 使用未知地址应该失败
	_, err = signer.Sign(tx, "unknown_address")
	if err == nil {
		t.Error("Sign() should fail with unknown address")
	}
}

func TestMnemonicSigner_DeriveMultipleAddresses(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 派生 5 个地址
	addrs, err := signer.DeriveMultipleAddresses(0, 0, 5)
	if err != nil {
		t.Fatalf("DeriveMultipleAddresses() error = %v", err)
	}

	if len(addrs) != 5 {
		t.Errorf("DeriveMultipleAddresses() returned %d addresses, want 5", len(addrs))
	}

	// 检查地址唯一性
	seen := make(map[string]bool)
	for _, addr := range addrs {
		if seen[addr] {
			t.Error("DeriveMultipleAddresses() returned duplicate addresses")
		}
		seen[addr] = true
	}
}

func TestMnemonicSigner_GetDerivationPath(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	if signer.GetDerivationPath() != WESDefaultPath() {
		t.Errorf("GetDerivationPath() = %s, want %s", signer.GetDerivationPath(), WESDefaultPath())
	}
}

func TestMnemonicSigner_GetPathForAddress(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	// 派生地址
	path := "m/44'/8888'/0'/0/5"
	addr, _ := signer.DeriveAddress(path)

	// 获取路径
	gotPath, ok := signer.GetPathForAddress(addr)
	if !ok {
		t.Fatal("GetPathForAddress() returned false")
	}

	if gotPath != path {
		t.Errorf("GetPathForAddress() = %s, want %s", gotPath, path)
	}

	// 未知地址
	_, ok = signer.GetPathForAddress("unknown")
	if ok {
		t.Error("GetPathForAddress() should return false for unknown address")
	}
}

func TestMnemonicSigner_GetMasterKey(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	// 未解锁时应该失败
	_, err := signer.GetMasterKey()
	if err == nil {
		t.Error("GetMasterKey() should fail when locked")
	}

	signer.Unlock("", 0)

	masterKey, err := signer.GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error = %v", err)
	}

	if masterKey == nil {
		t.Fatal("GetMasterKey() returned nil")
	}

	// 验证是有效的私钥
	var _ *ecdsa.PrivateKey = masterKey // 编译时类型检查
}

func TestMnemonicSigner_VerifySignature(t *testing.T) {
	mm := NewMnemonicManager()
	mnemonic, _ := mm.GenerateMnemonic(Mnemonic12Words)
	addrMgr := &mockAddressManager{}

	signer, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer.Unlock("", 0)

	addr, _ := signer.GetAddress("")

	// 签名
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	sig, _ := signer.SignHash(hash, addr)

	// 验证
	valid, err := signer.VerifySignature(hash, sig, addr)
	if err != nil {
		t.Fatalf("VerifySignature() error = %v", err)
	}

	if !valid {
		t.Error("VerifySignature() = false, want true")
	}

	// 使用错误的哈希验证应该失败
	wrongHash := make([]byte, 32)
	valid, _ = signer.VerifySignature(wrongHash, sig, addr)
	if valid {
		t.Error("VerifySignature() should return false for wrong hash")
	}
}

func TestMnemonicSigner_Deterministic(t *testing.T) {
	// 使用固定的测试助记词
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	addrMgr := &mockAddressManager{}

	// 创建两个使用相同助记词的签名器
	signer1, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer2, _ := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		AddressManager: addrMgr,
	})

	signer1.Unlock("", 0)
	signer2.Unlock("", 0)

	// 相同路径应该派生出相同的地址
	addr1, _ := signer1.GetAddress("")
	addr2, _ := signer2.GetAddress("")

	if addr1 != addr2 {
		t.Error("Same mnemonic should derive same address")
	}

	// 相同路径应该派生出相同的密钥
	key1, _ := signer1.DeriveKey(WESDefaultPath())
	key2, _ := signer2.DeriveKey(WESDefaultPath())

	if key1.D.Cmp(key2.D) != 0 {
		t.Error("Same mnemonic should derive same key")
	}
}

