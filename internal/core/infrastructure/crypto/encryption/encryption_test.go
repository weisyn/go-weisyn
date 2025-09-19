package encryption

import (
	"bytes"
	"testing"

	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
)

func TestEncryptDecrypt(t *testing.T) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)
	keyManager := key.NewKeyManager()

	// 生成测试用的密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("无法生成密钥对: %v", err)
	}

	testCases := []struct {
		name      string
		data      []byte
		expectErr bool
	}{
		{
			name:      "普通数据",
			data:      []byte("这是一段需要加密的测试数据"),
			expectErr: false,
		},
		{
			name:      "空数据",
			data:      []byte{},
			expectErr: true, // ECIES不支持加密空数据
		},
		{
			name:      "二进制数据",
			data:      []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 使用公钥加密
			encrypted, err := encryptionService.Encrypt(tc.data, publicKey)
			if tc.expectErr {
				if err == nil {
					t.Errorf("期望加密错误但没有得到错误")
				}
				return
			}

			if err != nil {
				t.Errorf("加密失败: %v", err)
				return
			}

			// 验证密文不等于明文
			if bytes.Equal(encrypted, tc.data) && len(tc.data) > 0 {
				t.Errorf("密文不应该等于明文")
			}

			// 使用私钥解密
			decrypted, err := encryptionService.Decrypt(encrypted, privateKey)
			if err != nil {
				t.Errorf("解密失败: %v", err)
				return
			}

			// 验证解密后的数据等于原始数据
			if !bytes.Equal(decrypted, tc.data) {
				t.Errorf("解密后的数据与原始数据不匹配")
			}
		})
	}

	// 测试错误情况
	t.Run("无效公钥", func(t *testing.T) {
		_, err := encryptionService.Encrypt([]byte("测试"), []byte("无效公钥"))
		if err == nil {
			t.Errorf("使用无效公钥应该返回错误")
		}
	})

	t.Run("无效私钥", func(t *testing.T) {
		validData := []byte("测试数据")
		encrypted, _ := encryptionService.Encrypt(validData, publicKey)

		_, err := encryptionService.Decrypt(encrypted, []byte("无效私钥"))
		if err == nil {
			t.Errorf("使用无效私钥应该返回错误")
		}
	})
}

func TestEncryptDecryptWithPassword(t *testing.T) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)

	testCases := []struct {
		name      string
		data      []byte
		password  string
		expectErr bool
	}{
		{
			name:      "普通数据和密码",
			data:      []byte("这是一段需要加密的测试数据"),
			password:  "test_password_123",
			expectErr: false,
		},
		{
			name:      "空数据",
			data:      []byte{},
			password:  "test_password_123",
			expectErr: false,
		},
		{
			name:      "二进制数据",
			data:      []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			password:  "test_password_123",
			expectErr: false,
		},
		{
			name:      "空密码",
			data:      []byte("这是一段需要加密的测试数据"),
			password:  "",
			expectErr: false,
		},
		{
			name:      "中文密码",
			data:      []byte("这是一段需要加密的测试数据"),
			password:  "测试密码123",
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 使用密码加密
			encrypted, err := encryptionService.EncryptWithPassword(tc.data, tc.password)
			if tc.expectErr {
				if err == nil {
					t.Errorf("期望加密错误但没有得到错误")
				}
				return
			}

			if err != nil {
				t.Errorf("加密失败: %v", err)
				return
			}

			// 验证密文不等于明文
			if bytes.Equal(encrypted, tc.data) && len(tc.data) > 0 {
				t.Errorf("密文不应该等于明文")
			}

			// 使用密码解密
			decrypted, err := encryptionService.DecryptWithPassword(encrypted, tc.password)
			if err != nil {
				t.Errorf("解密失败: %v", err)
				return
			}

			// 验证解密后的数据等于原始数据
			if !bytes.Equal(decrypted, tc.data) {
				t.Errorf("解密后的数据与原始数据不匹配")
			}

			// 使用错误密码尝试解密
			_, err = encryptionService.DecryptWithPassword(encrypted, tc.password+"wrong")
			if err == nil {
				t.Errorf("使用错误密码应该返回错误")
			}
		})
	}

	// 测试无效密文
	t.Run("无效密文", func(t *testing.T) {
		invalidCiphertext := []byte("too_short")
		_, err := encryptionService.DecryptWithPassword(invalidCiphertext, "password")
		if err == nil {
			t.Errorf("解密无效密文应该返回错误")
		}

		if err != ErrInvalidCiphertext {
			t.Errorf("期望错误 %v, 但得到 %v", ErrInvalidCiphertext, err)
		}
	})
}

func TestDeriveKey(t *testing.T) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)

	// 测试相同密码和盐生成相同密钥
	password := "test_password"
	salt := []byte("test_salt_123456")

	key1, err := encryptionService.deriveKey(password, salt)
	if err != nil {
		t.Errorf("密钥派生失败: %v", err)
	}

	key2, err := encryptionService.deriveKey(password, salt)
	if err != nil {
		t.Errorf("密钥派生失败: %v", err)
	}

	// 相同输入应该产生相同密钥
	if !bytes.Equal(key1, key2) {
		t.Errorf("相同密码和盐应该生成相同的密钥")
	}

	// 测试不同密码生成不同密钥
	key3, err := encryptionService.deriveKey("different_password", salt)
	if err != nil {
		t.Errorf("密钥派生失败: %v", err)
	}

	if bytes.Equal(key1, key3) {
		t.Errorf("不同密码应该生成不同的密钥")
	}

	// 测试不同盐生成不同密钥
	differentSalt := []byte("different_salt_")
	key4, err := encryptionService.deriveKey(password, differentSalt)
	if err != nil {
		t.Errorf("密钥派生失败: %v", err)
	}

	if bytes.Equal(key1, key4) {
		t.Errorf("不同盐应该生成不同的密钥")
	}
}

// 基准测试
func BenchmarkEncrypt(b *testing.B) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)
	keyManager := key.NewKeyManager()

	_, publicKey, _ := keyManager.GenerateKeyPair()
	data := []byte("这是一段需要加密的基准测试数据这是一段需要加密的基准测试数据")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encryptionService.Encrypt(data, publicKey)
	}
}

func BenchmarkEncryptWithPassword(b *testing.B) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)
	data := []byte("这是一段需要加密的基准测试数据这是一段需要加密的基准测试数据")
	password := "benchmark_password_123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encryptionService.EncryptWithPassword(data, password)
	}
}

func BenchmarkDeriveKey(b *testing.B) {
	hashService := hash.NewHashService()
	encryptionService := NewEncryptionService(hashService)
	password := "benchmark_password"
	salt := []byte("benchmark_salt_123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encryptionService.deriveKey(password, salt)
	}
}
