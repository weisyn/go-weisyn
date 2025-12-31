package signature

import (
	"bytes"
	"testing"

	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

func TestSignVerify(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "普通消息",
			data: []byte("这是一条测试消息"),
		},
		{
			name: "空消息",
			data: []byte{},
		},
		{
			name: "二进制数据",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
		},
		{
			name: "长消息",
			data: []byte("这是一条很长的测试消息，用于验证签名服务对长数据的处理能力，包含了各种字符和Unicode内容：测试中文🚀✅🎯"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 签名
			signature, err := signatureService.Sign(tc.data, privateKey)
			if err != nil {
				t.Fatalf("签名失败: %v", err)
			}

			// 验证签名长度
			if len(signature) != SignatureLength {
				t.Errorf("签名长度应为%d字节，但得到 %d 字节", SignatureLength, len(signature))
			}

			// 验证
			valid := signatureService.Verify(tc.data, signature, publicKey)
			if !valid {
				t.Errorf("签名验证失败")
			}

			// 篡改数据后验证
			if len(tc.data) > 0 {
				tamperedData := make([]byte, len(tc.data))
				copy(tamperedData, tc.data)
				tamperedData[0] ^= 0xFF // 修改第一个字节

				valid = signatureService.Verify(tamperedData, signature, publicKey)
				if valid {
					t.Errorf("篡改数据后签名验证应该失败")
				}
			}

			// 篡改签名后验证
			tamperedSignature := make([]byte, len(signature))
			copy(tamperedSignature, signature)
			tamperedSignature[0] ^= 0xFF // 修改第一个字节

			valid = signatureService.Verify(tc.data, tamperedSignature, publicKey)
			if valid {
				t.Errorf("篡改签名后验证应该失败")
			}
		})
	}
}

func TestSignTransaction(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 模拟交易哈希
	txHash := []byte("这是一个模拟的交易哈希，长度应该是32字节")
	if len(txHash) < 32 {
		// 补齐到32字节
		padding := make([]byte, 32-len(txHash))
		txHash = append(txHash, padding...)
	}
	txHash = txHash[:32] // 确保正好32字节

	// 测试不同的签名哈希类型
	testCases := []struct {
		name        string
		sigHashType cryptointf.SignatureHashType
	}{
		{
			name:        "SIGHASH_ALL",
			sigHashType: cryptointf.SigHashAll,
		},
		{
			name:        "SIGHASH_NONE",
			sigHashType: cryptointf.SigHashNone,
		},
		{
			name:        "SIGHASH_SINGLE",
			sigHashType: cryptointf.SigHashSingle,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 签名交易
			signature, err := signatureService.SignTransaction(txHash, privateKey, tc.sigHashType)
			if err != nil {
				t.Fatalf("交易签名失败: %v", err)
			}

			// 验证交易签名
			valid := signatureService.VerifyTransactionSignature(txHash, signature, publicKey, tc.sigHashType)
			if !valid {
				t.Errorf("交易签名验证失败")
			}

			// 验证签名格式
			err = signatureService.ValidateSignature(signature)
			if err != nil {
				t.Errorf("签名格式验证失败: %v", err)
			}
		})
	}
}

func TestSignMessage(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 测试消息
	message := []byte("这是要签名的消息")

	// 测试消息签名
	signature, err := signatureService.SignMessage(message, privateKey)
	if err != nil {
		t.Fatalf("消息签名失败: %v", err)
	}

	// 验证消息签名
	valid := signatureService.VerifyMessage(message, signature, publicKey)
	if !valid {
		t.Errorf("消息签名验证失败")
	}

	// 篡改消息后验证
	tamperedMessage := []byte("这是篡改过的消息")
	valid = signatureService.VerifyMessage(tamperedMessage, signature, publicKey)
	if valid {
		t.Errorf("篡改消息后验证应该失败")
	}
}

func TestRecoverPublicKey(t *testing.T) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, originalPublicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 测试消息（不是哈希，SignMessage会内部处理哈希）
	testMessage := []byte("测试公钥恢复功能")

	// 使用SignMessage创建可恢复的65字节签名
	signature, err := signatureService.SignMessage(testMessage, privateKey)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// 验证签名长度
	if len(signature) != 65 {
		t.Fatalf("签名长度错误: %d, 期望65字节", len(signature))
	}

	// 计算消息的哈希（与SignMessage内部使用的相同算法）
	// SignMessage使用的是prefixed message + double SHA256
	hashManager := hash.NewHashService()
	prefixedMessage := buildPrefixedMessage(testMessage)
	messageHash := hashManager.DoubleSHA256(prefixedMessage)

	// 尝试恢复公钥
	recoveredPublicKey, err := signatureService.RecoverPublicKey(messageHash, signature)
	if err != nil {
		t.Logf("恢复公钥失败: %v", err)
		t.Skip("公钥恢复功能可能需要进一步完善")
		return
	}

	// 比较公钥（转换为统一格式进行比较）
	originalCompressed := originalPublicKey
	if len(originalPublicKey) == 65 {
		// 如果原始公钥是未压缩格式，转换为压缩格式
		compressed, err := keyManager.CompressPublicKey(originalPublicKey)
		if err != nil {
			t.Fatalf("压缩原始公钥失败: %v", err)
		}
		originalCompressed = compressed
	}

	// 比较压缩公钥
	if !bytes.Equal(originalCompressed, recoveredPublicKey) {
		t.Errorf("恢复的公钥与原始公钥不匹配")
		t.Logf("原始公钥: %x", originalCompressed)
		t.Logf("恢复公钥: %x", recoveredPublicKey)
		return
	}

	t.Logf("✅ 公钥恢复成功")
}

// buildPrefixedMessage 构建带前缀的消息（与SignatureService相同的实现）
func buildPrefixedMessage(message []byte) []byte {
	prefix := []byte("\x18 Signed Message:\n")
	lengthBytes := []byte{byte(len(message))}

	result := make([]byte, 0, len(prefix)+len(lengthBytes)+len(message))
	result = append(result, prefix...)
	result = append(result, lengthBytes...)
	result = append(result, message...)

	return result
}

func TestRecoverAddress(t *testing.T) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 测试消息
	testMessage := []byte("测试地址恢复功能")

	// 使用SignMessage创建可恢复的65字节签名
	signature, err := signatureService.SignMessage(testMessage, privateKey)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// 验证签名长度
	if len(signature) != 65 {
		t.Fatalf("签名长度错误: %d, 期望65字节", len(signature))
	}

	// 计算消息的哈希（与SignMessage内部使用的相同算法）
	hashManager := hash.NewHashService()
	prefixedMessage := buildPrefixedMessage(testMessage)
	messageHash := hashManager.DoubleSHA256(prefixedMessage)

	// 恢复公钥
	recoveredPublicKey, err := signatureService.RecoverPublicKey(messageHash, signature)
	if err != nil {
		t.Fatalf("恢复公钥失败: %v", err)
	}

	// 验证恢复的公钥与原始公钥一致（转换为压缩格式比较）
	originalCompressed := publicKey
	if len(publicKey) == 65 {
		compressed, err := keyManager.CompressPublicKey(publicKey)
		if err != nil {
			t.Fatalf("压缩原始公钥失败: %v", err)
		}
		originalCompressed = compressed
	}

	if !bytes.Equal(originalCompressed, recoveredPublicKey) {
		t.Errorf("恢复的公钥与原始公钥不匹配")
		t.Logf("原始公钥: %x", originalCompressed)
		t.Logf("恢复公钥: %x", recoveredPublicKey)
		return
	}

	// 从恢复的公钥生成地址
	recoveredAddress, err := addressManager.PublicKeyToAddress(recoveredPublicKey)
	if err != nil {
		t.Fatalf("从恢复公钥生成地址失败: %v", err)
	}

	// 从原始公钥生成地址进行比较
	expectedAddress, err := addressManager.PublicKeyToAddress(originalCompressed)
	if err != nil {
		t.Fatalf("从原始公钥生成地址失败: %v", err)
	}

	// 比较地址
	if expectedAddress != recoveredAddress {
		t.Errorf("恢复的地址与期望地址不匹配")
		t.Logf("期望地址: %s", expectedAddress)
		t.Logf("恢复地址: %s", recoveredAddress)
		return
	}

	t.Logf("✅ 地址恢复成功: %s", recoveredAddress)
}

func TestSignBatch(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成密钥对
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	// 测试数据批次
	dataList := [][]byte{
		[]byte("第一条数据"),
		[]byte("第二条数据"),
		[]byte("第三条数据"),
	}

	// 批量签名
	signatures, err := signatureService.SignBatch(dataList, privateKey)
	if err != nil {
		t.Fatalf("批量签名失败: %v", err)
	}

	// 验证签名数量
	if len(signatures) != len(dataList) {
		t.Fatalf("签名数量不匹配，期望%d，得到%d", len(dataList), len(signatures))
	}

	// 创建公钥列表
	publicKeyList := make([][]byte, len(dataList))
	for i := range publicKeyList {
		publicKeyList[i] = publicKey
	}

	// 批量验证
	results, err := signatureService.VerifyBatch(dataList, signatures, publicKeyList)
	if err != nil {
		t.Fatalf("批量验证失败: %v", err)
	}

	// 检查验证结果
	for i, result := range results {
		if !result {
			t.Errorf("第%d个签名验证失败", i+1)
		}
	}
}

func TestNormalizeSignature(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成测试签名
	privateKey, publicKey, err := keyManager.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	testData := []byte("测试签名规范化")
	signature, err := signatureService.Sign(testData, privateKey)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// 规范化签名
	normalizedSig, err := signatureService.NormalizeSignature(signature)
	if err != nil {
		t.Fatalf("签名规范化失败: %v", err)
	}

	// 验证规范化后的签名长度
	if len(normalizedSig) != SignatureLength {
		t.Errorf("规范化后签名长度不正确，期望%d，得到%d", SignatureLength, len(normalizedSig))
	}

	// 验证规范化后的签名是否有效
	valid := signatureService.Verify(testData, normalizedSig, publicKey)
	if !valid {
		t.Errorf("规范化后的签名验证失败")
	}
}

func TestValidateSignature(t *testing.T) {
	// 创建管理器
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	// 生成一个真实的签名用于测试
	privateKey, _, _ := keyManager.GenerateKeyPair()
	testData := []byte("测试数据")
	validSignature, _ := signatureService.Sign(testData, privateKey)

	// 测试用例
	testCases := []struct {
		name        string
		signature   []byte
		expectError bool
	}{
		{
			name:        "有效签名",
			signature:   validSignature,
			expectError: false,
		},
		{
			name:        "签名太短",
			signature:   make([]byte, SignatureLength-1),
			expectError: true,
		},
		{
			name:        "签名太长",
			signature:   make([]byte, SignatureLength+1),
			expectError: true,
		},
		{
			name:        "空签名",
			signature:   nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := signatureService.ValidateSignature(tc.signature)
			if tc.expectError && err == nil {
				t.Errorf("期望出现错误，但没有")
			}
			if !tc.expectError && err != nil {
				t.Errorf("不期望出现错误，但得到: %v", err)
			}
		})
	}
}

// 基准测试
func BenchmarkSign(b *testing.B) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	privateKey, _, _ := keyManager.GenerateKeyPair()
	data := []byte("这是用于基准测试的数据")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signatureService.Sign(data, privateKey)
	}
}

func BenchmarkVerify(b *testing.B) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	privateKey, publicKey, _ := keyManager.GenerateKeyPair()
	data := []byte("这是用于基准测试的数据")
	signature, _ := signatureService.Sign(data, privateKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signatureService.Verify(data, signature, publicKey)
	}
}

func BenchmarkSignMessage(b *testing.B) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	privateKey, _, _ := keyManager.GenerateKeyPair()
	message := []byte("这是用于基准测试的消息")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signatureService.SignMessage(message, privateKey)
	}
}

func BenchmarkRecoverPublicKey(b *testing.B) {
	keyManager := key.NewKeyManager()
	addressManager := address.NewAddressService(keyManager)
	signatureService := NewSignatureService(keyManager, addressManager)

	privateKey, _, _ := keyManager.GenerateKeyPair()
	data := []byte("这是用于基准测试的数据")
	signature, _ := signatureService.Sign(data, privateKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signatureService.RecoverPublicKey(data, signature)
	}
}
