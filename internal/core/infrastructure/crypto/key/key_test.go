package key

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

func TestGenerateKeyPair(t *testing.T) {
	km := NewKeyManager()

	privateKey, publicKey, err := km.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair失败: %v", err)
	}

	// 验证私钥长度
	if len(privateKey) != 32 {
		t.Errorf("私钥长度 = %d, 期望 32", len(privateKey))
	}

	// 验证公钥长度（修正：GenerateKeyPair返回33字节压缩公钥）
	if len(publicKey) != 33 {
		t.Errorf("公钥长度 = %d, 期望 33（压缩格式）", len(publicKey))
	}

	// 验证派生公钥
	derivedPublicKey, err := km.DerivePublicKey(privateKey)
	if err != nil {
		t.Fatalf("DerivePublicKey失败: %v", err)
	}

	if !bytes.Equal(publicKey, derivedPublicKey) {
		t.Errorf("派生的公钥与生成的公钥不匹配")
	}
}

func TestGenerateKeyPairWithContext(t *testing.T) {
	km := NewKeyManager()

	// 测试正常情况
	ctx := context.Background()
	privateKey, publicKey, err := km.GenerateKeyPairWithContext(ctx)
	if err != nil {
		t.Fatalf("GenerateKeyPairWithContext失败: %v", err)
	}

	if len(privateKey) != 32 || len(publicKey) != 64 {
		t.Errorf("生成的密钥对长度不正确")
	}

	// 测试取消的上下文
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, _, err = km.GenerateKeyPairWithContext(cancelCtx)
	if err != ErrOperationCancelled {
		t.Errorf("预期操作取消错误，得到: %v", err)
	}

	// 测试超时上下文
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // 确保超时

	_, _, err = km.GenerateKeyPairWithContext(timeoutCtx)
	if err == nil {
		t.Errorf("预期操作超时错误，但没有得到错误")
	}
}

func TestDerivePublicKey(t *testing.T) {
	km := NewKeyManager()

	testCases := []struct {
		name          string
		privateKey    []byte
		expectError   bool
		expectedError error
	}{
		{"有效私钥", make([]byte, 32), false, nil},
		{"空私钥", []byte{}, true, ErrInvalidPrivateKey},
		{"长度错误", make([]byte, 31), true, ErrInvalidPrivateKey},
	}

	// 生成有效私钥
	realPrivKey, _, err := km.GenerateKeyPair()
	if err != nil {
		t.Fatalf("无法生成测试密钥对: %v", err)
	}

	// 替换第一个测试用例中的私钥
	testCases[0].privateKey = realPrivKey

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			publicKey, err := km.DerivePublicKey(tc.privateKey)

			if tc.expectError {
				if err == nil {
					t.Errorf("预期错误但没有得到错误")
				} else if err != tc.expectedError {
					t.Errorf("预期错误 %v, 得到 %v", tc.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("DerivePublicKey失败: %v", err)
				}

				if len(publicKey) != 33 {
					t.Errorf("公钥长度 = %d, 期望 33（压缩格式）", len(publicKey))
				}
			}
		})
	}
}

func TestPrivateKeyToECDSA(t *testing.T) {
	km := NewKeyManager()

	// 生成有效私钥
	privateKey, _, err := km.GenerateKeyPair()
	if err != nil {
		t.Fatalf("无法生成测试密钥对: %v", err)
	}

	// 测试有效私钥
	ecdsaKey, err := km.PrivateKeyToECDSA(privateKey)
	if err != nil {
		t.Errorf("PrivateKeyToECDSA失败: %v", err)
	}

	if ecdsaKey == nil {
		t.Errorf("返回的ECDSA私钥为nil")
	} else if ecdsaKey.Curve == nil {
		t.Errorf("ECDSA私钥曲线为nil")
	}

	// 测试无效私钥
	_, err = km.PrivateKeyToECDSA([]byte{})
	if err != ErrInvalidPrivateKey {
		t.Errorf("预期私钥无效错误，得到: %v", err)
	}
}

func TestPublicKeyToECDSA(t *testing.T) {
	km := NewKeyManager()

	// 生成有效公钥
	_, publicKey, err := km.GenerateKeyPair()
	if err != nil {
		t.Fatalf("无法生成测试密钥对: %v", err)
	}

	// 测试有效公钥
	ecdsaKey, err := km.PublicKeyToECDSA(publicKey)
	if err != nil {
		t.Errorf("PublicKeyToECDSA失败: %v", err)
	}

	if ecdsaKey == nil {
		t.Errorf("返回的ECDSA公钥为nil")
	} else if ecdsaKey.Curve == nil {
		t.Errorf("ECDSA公钥曲线为nil")
	}

	// 测试曲线上的点
	if !ecdsaKey.Curve.IsOnCurve(ecdsaKey.X, ecdsaKey.Y) {
		t.Errorf("公钥不在曲线上")
	}

	// 测试无效公钥
	_, err = km.PublicKeyToECDSA([]byte{})
	if err == nil {
		t.Errorf("预期公钥无效错误，但没有得到错误")
	}
}

func TestSecureWipe(t *testing.T) {
	// 测试数据擦除
	sensitiveData := []byte{1, 2, 3, 4, 5}
	SecureWipe(sensitiveData)

	// 验证所有字节都被擦除为0
	for i, b := range sensitiveData {
		if b != 0 {
			t.Errorf("位置 %d 的字节未被擦除, 得到 %d", i, b)
		}
	}
}

func TestKeyPools(t *testing.T) {
	km := NewKeyManager()

	// 测试私钥池
	privateKey := km.privateKeyPool.Get()
	if len(privateKey) != 32 {
		t.Errorf("从池获取的私钥长度 = %d, 期望 32", len(privateKey))
	}

	// 填充一些数据
	for i := range privateKey {
		privateKey[i] = byte(i)
	}

	// 归还到池
	km.ReleasePrivateKey(privateKey)

	// 再次获取，验证已被清零
	anotherPrivateKey := km.privateKeyPool.Get()
	for i, b := range anotherPrivateKey {
		if b != 0 {
			t.Errorf("归还的私钥未被清零，位置 %d 的值为 %d", i, b)
		}
	}

	// 测试公钥池
	publicKey := km.publicKeyPool.Get()
	if len(publicKey) != 64 {
		t.Errorf("从池获取的公钥长度 = %d, 期望 64", len(publicKey))
	}

	// 归还到池
	km.ReleasePublicKey(publicKey)
}

// 测试私钥和公钥的ECDSA互操作性
func TestECDSAInteroperability(t *testing.T) {
	km := NewKeyManager()

	// 生成ECDSA密钥对
	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), cryptorand.Reader)
	if err != nil {
		t.Fatalf("ECDSA密钥生成失败: %v", err)
	}

	// 转换为字节格式
	privateKeyBytes := make([]byte, 32)
	copy(privateKeyBytes[32-len(privateKey.D.Bytes()):], privateKey.D.Bytes())

	// 从字节转回ECDSA
	recoveredPrivateKey, err := km.PrivateKeyToECDSA(privateKeyBytes)
	if err != nil {
		t.Fatalf("PrivateKeyToECDSA失败: %v", err)
	}

	// 验证两个ECDSA私钥是否匹配
	if privateKey.D.Cmp(recoveredPrivateKey.D) != 0 {
		t.Errorf("恢复的ECDSA私钥与原始私钥不匹配")
	}
}

// 基准测试

func BenchmarkGenerateKeyPair(b *testing.B) {
	km := NewKeyManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		privateKey, publicKey, _ := km.GenerateKeyPair()
		// 在生产环境中应该释放这些密钥
		km.ReleasePrivateKey(privateKey)
		km.ReleasePublicKey(publicKey)
	}
}

func BenchmarkDerivePublicKey(b *testing.B) {
	km := NewKeyManager()
	privateKey, _, _ := km.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		publicKey, _ := km.DerivePublicKey(privateKey)
		km.ReleasePublicKey(publicKey)
	}
}

func BenchmarkPrivateKeyToECDSA(b *testing.B) {
	km := NewKeyManager()
	privateKey, _, _ := km.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		km.PrivateKeyToECDSA(privateKey)
	}
}

func BenchmarkPublicKeyToECDSA(b *testing.B) {
	km := NewKeyManager()
	_, publicKey, _ := km.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		km.PublicKeyToECDSA(publicKey)
	}
}

func BenchmarkKeyPoolGet(b *testing.B) {
	pool := NewPrivateKeyPool()

	// 预热池
	keys := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		keys[i] = pool.Get()
	}
	for i := 0; i < 100; i++ {
		pool.Put(keys[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := pool.Get()
		pool.Put(key)
	}
}
