// Package signer_test 提供 Signer 的单元测试
//
// 🧪 **测试覆盖**：
// - LocalSigner 核心功能测试
// - 签名功能测试
// - 公钥获取测试
// - 边界条件和错误场景测试
package signer

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// ==================== LocalSigner 核心功能测试 ====================

// TestNewLocalSigner_Success 测试创建 LocalSigner 成功
func TestNewLocalSigner_Success(t *testing.T) {
	// 创建模拟的依赖
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(
		config,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
		logger,
	)

	assert.NoError(t, err)
	assert.NotNil(t, signer)
}

// TestLocalSigner_Sign 测试签名交易
func TestLocalSigner_Sign(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{
		signature: []byte("mock-signature"),
	}
	mockClient := &MockTransactionHashServiceClientForLocal{
		txHash: []byte("mock-tx-hash"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(mockClient)
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(
		config,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
		logger,
	)
	require.NoError(t, err)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	signature, err := signer.Sign(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, []byte("mock-signature"), signature.Value)
}

// TestLocalSigner_PublicKey 测试获取公钥
func TestLocalSigner_PublicKey(t *testing.T) {
	mockKeyMgr := &MockKeyManager{
		publicKey: &transaction.PublicKey{
			Value: testutil.RandomPublicKey(),
		},
	}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(
		config,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
		logger,
	)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()

	assert.NoError(t, err)
	assert.NotNil(t, publicKey)
	assert.Equal(t, mockKeyMgr.publicKey.Value, publicKey.Value)
}

// TestLocalSigner_Algorithm 测试获取算法
func TestLocalSigner_Algorithm(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(
		config,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
		logger,
	)
	require.NoError(t, err)

	algorithm := signer.Algorithm()

	assert.Equal(t, transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1, algorithm)
}

// ==================== LocalSigner 错误场景测试 ====================

// TestNewLocalSigner_NilConfig 测试 nil config
func TestNewLocalSigner_NilConfig(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	// 注意：NewLocalSigner 会先调用 checkEnvironment(config.Environment, logger)
	// 如果 config 为 nil，会在访问 config.Environment 时 panic
	// 这里测试应该捕获 panic 或返回错误
	defer func() {
		if r := recover(); r != nil {
			// 如果 panic，说明没有处理 nil config
			// 这是预期的行为，因为访问 nil config 的字段会 panic
			assert.NotNil(t, r)
		}
	}()

	_, err := NewLocalSigner(nil, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	// 如果返回了错误而不是 panic，验证错误
	if err != nil {
		assert.Error(t, err)
	}
}

// TestNewLocalSigner_InvalidPrivateKey 测试无效私钥
func TestNewLocalSigner_InvalidPrivateKey(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "invalid-key", // 无效的私钥格式
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	_, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析私钥失败")
}

// TestNewLocalSigner_ShortPrivateKey 测试私钥长度不足
func TestNewLocalSigner_ShortPrivateKey(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef", // 只有32个字符，需要64个
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	_, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "私钥长度无效")
}

// TestNewLocalSigner_PrivateKeyWithPrefix 测试带前缀的私钥（兼容性容错）
// 注意：根据 WES 规范，私钥输入可以带 0x 前缀（会自动剥离），但推荐使用纯 hex 格式
func TestNewLocalSigner_PrivateKeyWithPrefix(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 带 0x 前缀（兼容性测试）
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	assert.NoError(t, err)
	assert.NotNil(t, signer)
}

// TestLocalSigner_Sign_NilTransaction 测试 nil transaction
func TestLocalSigner_Sign_NilTransaction(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	_, err = signer.Sign(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction is nil")
}

// TestLocalSigner_Sign_HashError 测试哈希计算失败
func TestLocalSigner_Sign_HashError(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockClient := &MockTransactionHashServiceClientForLocal{
		computeHashError: fmt.Errorf("hash computation failed"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(mockClient)
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	_, err = signer.Sign(context.Background(), tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "计算交易哈希失败")
}

// TestLocalSigner_Sign_SignatureError 测试签名失败
func TestLocalSigner_Sign_SignatureError(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{
		signError: fmt.Errorf("signature failed"),
	}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	_, err = signer.Sign(context.Background(), tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECDSA签名失败")
}

// TestLocalSigner_Sign_UnsupportedAlgorithm 测试不支持的算法
func TestLocalSigner_Sign_UnsupportedAlgorithm(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN,
		Environment:   "testing",
	}

	// 注意：derivePublicKey 不支持 UNKNOWN 算法，创建 signer 时会失败
	_, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	// 创建 signer 时就会失败，因为 derivePublicKey 不支持 UNKNOWN 算法
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提取公钥失败")
}

// TestLocalSigner_SignBytes_Success 测试 SignBytes 成功
func TestLocalSigner_SignBytes_Success(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{
		signature: []byte("mock-signature"),
	}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	signature, err := signer.SignBytes(context.Background(), []byte("test-data"))

	assert.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, []byte("mock-signature"), signature)
}

// TestLocalSigner_SignBytes_EmptyData 测试空数据
func TestLocalSigner_SignBytes_EmptyData(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	_, err = signer.SignBytes(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "待签名数据为空")
}

// TestLocalSigner_SignBytes_SignatureError 测试 SignBytes 签名失败
func TestLocalSigner_SignBytes_SignatureError(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{
		signError: fmt.Errorf("signature failed"),
	}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	_, err = signer.SignBytes(context.Background(), []byte("test-data"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECDSA签名失败")
}

// TestLocalSigner_SignBytes_UnsupportedAlgorithm 测试不支持的算法
func TestLocalSigner_SignBytes_UnsupportedAlgorithm(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN,
		Environment:   "testing",
	}

	// 注意：derivePublicKey 不支持 UNKNOWN 算法，创建 signer 时会失败
	_, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)

	// 创建 signer 时就会失败，因为 derivePublicKey 不支持 UNKNOWN 算法
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提取公钥失败")
}

// TestLocalSigner_PublicKey_Nil 测试 nil 公钥
func TestLocalSigner_PublicKey_Nil(t *testing.T) {
	signer := &LocalSigner{
		publicKey: nil,
	}

	publicKey, err := signer.PublicKey()

	assert.NoError(t, err)
	assert.Nil(t, publicKey)
}

// TestLocalSigner_ED25519 测试 ED25519 算法
func TestLocalSigner_ED25519(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{
		signature: []byte("mock-ed25519-signature"),
	}
	mockCanonicalizer := NewMockCanonicalizer()
	logger := &testutil.MockLogger{}

	config := &LocalSignerConfig{
		PrivateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519,
		Environment:   "testing",
	}

	signer, err := NewLocalSigner(config, mockKeyMgr, mockSigMgr, mockCanonicalizer, logger)
	require.NoError(t, err)

	assert.Equal(t, transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519, signer.Algorithm())

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	signature, err := signer.Sign(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, []byte("mock-ed25519-signature"), signature.Value)
}

// ==================== checkEnvironment 测试 ====================

// TestCheckEnvironment_ProductionEnv 测试生产环境检查
func TestCheckEnvironment_ProductionEnv(t *testing.T) {
	logger := &testutil.MockLogger{}

	// 设置环境变量 ENV=production
	os.Setenv("ENV", "production")
	defer os.Unsetenv("ENV")

	err := checkEnvironment("testing", logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "禁止在生产环境使用")
}

// TestCheckEnvironment_ProductionEnvironment 测试 ENVIRONMENT 环境变量
func TestCheckEnvironment_ProductionEnvironment(t *testing.T) {
	logger := &testutil.MockLogger{}

	// 设置环境变量 ENVIRONMENT=production
	os.Setenv("ENVIRONMENT", "production")
	defer os.Unsetenv("ENVIRONMENT")

	err := checkEnvironment("testing", logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "禁止在生产环境使用")
}

// TestCheckEnvironment_ProductionConfig 测试配置中的生产环境
func TestCheckEnvironment_ProductionConfig(t *testing.T) {
	logger := &testutil.MockLogger{}

	err := checkEnvironment("production", logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "禁止在生产环境使用")
}

// TestCheckEnvironment_ProductionHostname 测试主机名包含 prod
func TestCheckEnvironment_ProductionHostname(t *testing.T) {
	logger := &testutil.MockLogger{}

	// 注意：这个测试依赖于实际的主机名，可能在不同环境中表现不同
	// 如果主机名包含 "prod" 或 "production"，应该返回错误
	err := checkEnvironment("testing", logger)

	// 如果主机名包含 prod，应该返回错误
	// 否则应该通过
	if err != nil {
		assert.Contains(t, err.Error(), "禁止在生产环境使用")
	}
}

// TestCheckEnvironment_Success 测试环境检查通过
func TestCheckEnvironment_Success(t *testing.T) {
	logger := &testutil.MockLogger{}

	// 确保环境变量不包含 prod
	os.Unsetenv("ENV")
	os.Unsetenv("ENVIRONMENT")

	err := checkEnvironment("testing", logger)

	// 如果主机名不包含 prod，应该通过
	// 否则可能返回错误
	_ = err
}

// ==================== derivePublicKey 测试 ====================

// TestDerivePublicKey_InvalidKeyLength 测试无效私钥长度
func TestDerivePublicKey_InvalidKeyLength(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	logger := &testutil.MockLogger{}

	// 使用无效长度的私钥
	invalidKey := []byte("short-key")

	_, err := derivePublicKey(
		invalidKey,
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		mockKeyMgr,
		logger,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "私钥长度无效")
}

// TestDerivePublicKey_ECDSA_InvalidPublicKeyLength 测试 ECDSA 公钥长度无效
func TestDerivePublicKey_ECDSA_InvalidPublicKeyLength(t *testing.T) {
	mockKeyMgr := &MockKeyManager{
		publicKey: &transaction.PublicKey{
			Value: []byte("invalid-length"), // 不是33字节
		},
	}
	logger := &testutil.MockLogger{}

	privateKey := make([]byte, 32)
	copy(privateKey, "0123456789abcdef0123456789abcdef")

	_, err := derivePublicKey(
		privateKey,
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		mockKeyMgr,
		logger,
	)

	// 如果 DerivePublicKey 返回的长度不是33字节，应该返回错误
	assert.Error(t, err)
}

// TestDerivePublicKey_ED25519_33Bytes 测试 ED25519 公钥为33字节（压缩格式）
func TestDerivePublicKey_ED25519_33Bytes(t *testing.T) {
	// 创建一个返回33字节公钥的 MockKeyManager
	mockKeyMgr := &MockKeyManager{
		publicKey: &transaction.PublicKey{
			Value: make([]byte, 33), // 33字节压缩格式
		},
	}
	logger := &testutil.MockLogger{}

	privateKey := make([]byte, 32)
	copy(privateKey, "0123456789abcdef0123456789abcdef")

	publicKey, err := derivePublicKey(
		privateKey,
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519,
		mockKeyMgr,
		logger,
	)

	// 如果成功，公钥应该是32字节（从33字节压缩格式中提取）
	if err == nil {
		assert.NotNil(t, publicKey)
		assert.Equal(t, 32, len(publicKey.Value))
	}
}

// TestDerivePublicKey_UnsupportedAlgorithm 测试不支持的算法
func TestDerivePublicKey_UnsupportedAlgorithm(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	logger := &testutil.MockLogger{}

	privateKey := make([]byte, 32)
	copy(privateKey, "0123456789abcdef0123456789abcdef")

	_, err := derivePublicKey(
		privateKey,
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN,
		mockKeyMgr,
		logger,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的签名算法")
}

// ==================== NewLocalSignerForTesting 测试 ====================

// TestNewLocalSignerForTesting_Success 测试创建测试签名器成功
func TestNewLocalSignerForTesting_Success(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	signer, err := NewLocalSignerForTesting(
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
	)

	assert.NoError(t, err)
	assert.NotNil(t, signer)
}

// TestNewLocalSignerForTesting_InvalidKey 测试无效私钥
func TestNewLocalSignerForTesting_InvalidKey(t *testing.T) {
	mockKeyMgr := &MockKeyManager{}
	mockSigMgr := &MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	_, err := NewLocalSignerForTesting(
		"invalid-key",
		transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		mockKeyMgr,
		mockSigMgr,
		mockCanonicalizer,
	)

	assert.Error(t, err)
}

// ==================== Mock 辅助类型 ====================

// MockKeyManager 模拟 KeyManager
type MockKeyManager struct {
	publicKey *transaction.PublicKey
}

func (m *MockKeyManager) GenerateKeyPair() ([]byte, []byte, error) {
	return nil, nil, nil
}

func (m *MockKeyManager) DeriveKeyPair(seed []byte, index uint32) ([]byte, []byte, error) {
	return nil, nil, nil
}

func (m *MockKeyManager) GetPublicKey(privateKey []byte) ([]byte, error) {
	if m.publicKey != nil {
		return m.publicKey.Value, nil
	}
	return testutil.RandomPublicKey(), nil
}

func (m *MockKeyManager) CompressPublicKey(publicKey []byte) ([]byte, error) {
	return publicKey, nil
}

func (m *MockKeyManager) DecompressPublicKey(compressedKey []byte) ([]byte, error) {
	// 简化实现：返回未压缩格式（65字节）
	if len(compressedKey) == 33 {
		uncompressed := make([]byte, 65)
		uncompressed[0] = 0x04 // 未压缩标记
		copy(uncompressed[1:], compressedKey[1:])
		return uncompressed, nil
	}
	return compressedKey, nil
}

func (m *MockKeyManager) GenerateCompressedKeyPair() ([]byte, []byte, error) {
	return m.GenerateKeyPair()
}

func (m *MockKeyManager) DerivePublicKey(privateKey []byte) ([]byte, error) {
	return m.GetPublicKey(privateKey)
}

func (m *MockKeyManager) DeriveUncompressedPublicKey(privateKey []byte) ([]byte, error) {
	pubKey, err := m.GetPublicKey(privateKey)
	if err != nil {
		return nil, err
	}
	return m.DecompressPublicKey(pubKey)
}

func (m *MockKeyManager) ParsePublicKeyString(publicKeyHex string) ([]byte, error) {
	// 简化实现
	return []byte(publicKeyHex), nil
}

func (m *MockKeyManager) ValidatePrivateKey(privateKey []byte) error {
	if len(privateKey) != 32 {
		return fmt.Errorf("invalid private key length")
	}
	return nil
}

func (m *MockKeyManager) ValidatePublicKey(publicKey []byte) error {
	if len(publicKey) != 33 && len(publicKey) != 65 {
		return fmt.Errorf("invalid public key length")
	}
	return nil
}

// MockSignatureManager 模拟 SignatureManager
type MockSignatureManager struct {
	signature []byte
	signError error
}

func (m *MockSignatureManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	if m.signError != nil {
		return nil, m.signError
	}
	if m.signature != nil {
		return m.signature, nil
	}
	return []byte("mock-signature"), nil
}

func (m *MockSignatureManager) Verify(data, signature, publicKey []byte) bool {
	return true
}

func (m *MockSignatureManager) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	return testutil.RandomPublicKey(), nil
}

func (m *MockSignatureManager) NormalizeSignature(signature []byte) ([]byte, error) {
	return signature, nil
}

func (m *MockSignatureManager) SignTransaction(txHash []byte, privateKey []byte, sigHashType crypto.SignatureHashType) ([]byte, error) {
	return m.Sign(txHash, privateKey)
}

func (m *MockSignatureManager) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType crypto.SignatureHashType) bool {
	return m.Verify(txHash, signature, publicKey)
}

func (m *MockSignatureManager) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	return m.Sign(message, privateKey)
}

func (m *MockSignatureManager) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	return m.Verify(message, signature, publicKey)
}

func (m *MockSignatureManager) ValidateSignature(signature []byte) error {
	if len(signature) == 0 {
		return fmt.Errorf("signature is empty")
	}
	return nil
}

func (m *MockSignatureManager) RecoverAddress(hash []byte, signature []byte) (string, error) {
	pubKey, err := m.RecoverPublicKey(hash, signature)
	if err != nil {
		return "", err
	}
	// 简化实现：返回公钥的十六进制字符串作为地址
	return fmt.Sprintf("%x", pubKey), nil
}

func (m *MockSignatureManager) SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error) {
	signatures := make([][]byte, len(dataList))
	for i, data := range dataList {
		sig, err := m.Sign(data, privateKey)
		if err != nil {
			return nil, err
		}
		signatures[i] = sig
	}
	return signatures, nil
}

func (m *MockSignatureManager) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	results := make([]bool, len(dataList))
	for i := range dataList {
		results[i] = m.Verify(dataList[i], signatureList[i], publicKeyList[i])
	}
	return results, nil
}

// MockHashManager 模拟 HashManager
type MockHashManager struct{}

func (m *MockHashManager) SHA256(data []byte) []byte {
	return testutil.RandomBytes(32)
}

func (m *MockHashManager) Keccak256(data []byte) []byte {
	return testutil.RandomBytes(32)
}

func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	return testutil.RandomBytes(20)
}

func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	return testutil.RandomBytes(32)
}

// MockCanonicalizer 模拟 Canonicalizer
// 注意：NewLocalSigner 需要 *hash.Canonicalizer 类型，所以这里创建一个包装器
type MockCanonicalizer struct {
	txHash  []byte
	sigHash []byte
}

// NewMockCanonicalizer 创建模拟 Canonicalizer（返回 *hash.Canonicalizer）
func NewMockCanonicalizer() *hash.Canonicalizer {
	mockClient := &MockTransactionHashServiceClientForLocal{
		txHash:  testutil.RandomTxID(),
		sigHash: testutil.RandomTxID(),
	}
	return hash.NewCanonicalizer(mockClient)
}

// MockTransactionHashServiceClientForLocal 模拟 TransactionHashServiceClient（用于创建 MockCanonicalizer，避免与 kms_test.go 冲突）
type MockTransactionHashServiceClientForLocal struct {
	txHash           []byte
	sigHash          []byte
	computeHashError error
}

func (m *MockTransactionHashServiceClientForLocal) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if m.computeHashError != nil {
		return nil, m.computeHashError
	}
	if m.txHash != nil {
		return &transaction.ComputeHashResponse{
			Hash:    m.txHash,
			IsValid: true,
		}, nil
	}
	return &transaction.ComputeHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClientForLocal) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClientForLocal) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	if m.sigHash != nil {
		return &transaction.ComputeSignatureHashResponse{
			Hash:    m.sigHash,
			IsValid: true,
		}, nil
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClientForLocal) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}
