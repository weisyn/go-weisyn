package zkproof

import (
	"bytes"
	"context"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/stretchr/testify/require"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// validator.go 测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestNewValidator 测试创建验证器
func TestNewValidator(t *testing.T) {
	logger := testutil.NewTestLogger()
	circuitManager := NewCircuitManager(logger, &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	})
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	hashManager := testutil.NewTestHashManager()

	validator := NewValidator(logger, circuitManager, config, hashManager)
	require.NotNil(t, validator)
	require.NotNil(t, validator.vkCache)
	require.True(t, validator.supportedSchemes["groth16"])
	require.True(t, validator.supportedSchemes["plonk"])
	require.Equal(t, ecc.BN254, validator.supportedCurves["bn254"])
}

// TestValidator_validateProofData 测试证明数据验证
func TestValidator_validateProofData(t *testing.T) {
	validator := createTestValidator(t)

	// 测试空证明数据
	proof := &transaction.ZKStateProof{
		Proof:               []byte{},
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "test_circuit",
		VerificationKeyHash: make([]byte, 32),
	}
	err := validator.validateProofData(proof)
	require.Error(t, err)
	require.Contains(t, err.Error(), "证明数据为空")

	// 测试空公开输入
	proof = &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{},
		CircuitId:           "test_circuit",
		VerificationKeyHash: make([]byte, 32),
	}
	err = validator.validateProofData(proof)
	require.Error(t, err)
	require.Contains(t, err.Error(), "公开输入为空")

	// 测试空电路ID
	proof = &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "",
		VerificationKeyHash: make([]byte, 32),
	}
	err = validator.validateProofData(proof)
	require.Error(t, err)
	require.Contains(t, err.Error(), "电路ID为空")

	// 测试无效的验证密钥哈希长度
	proof = &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "test_circuit",
		VerificationKeyHash: make([]byte, 16), // 无效长度
	}
	err = validator.validateProofData(proof)
	require.Error(t, err)
	require.Contains(t, err.Error(), "验证密钥哈希长度无效")

	// 测试有效数据
	proof = &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "test_circuit",
		VerificationKeyHash: make([]byte, 32),
	}
	err = validator.validateProofData(proof)
	require.NoError(t, err)
}

// TestValidator_getVerifyingKey 测试获取验证密钥（带缓存）
func TestValidator_getVerifyingKey(t *testing.T) {
	validator := createTestValidator(t)

	// 第一次获取（缓存未命中）
	vk1, err := validator.getVerifyingKey("contract_execution", 1, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, vk1)

	// 第二次获取（缓存命中）
	vk2, err := validator.getVerifyingKey("contract_execution", 1, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, vk2)
	// 应该是同一个实例（从缓存获取）
}

// TestValidator_getVerifyingKey_InvalidCircuit 测试获取无效电路的验证密钥
func TestValidator_getVerifyingKey_InvalidCircuit(t *testing.T) {
	validator := createTestValidator(t)

	_, err := validator.getVerifyingKey("nonexistent_circuit", 1, ecc.BN254)
	require.Error(t, err)
	require.Contains(t, err.Error(), "获取可信设置失败")
}

// TestValidator_validateVerifyingKeyHash 测试验证密钥哈希验证
func TestValidator_validateVerifyingKeyHash(t *testing.T) {
	validator := createTestValidator(t)

	// 创建测试电路和验证密钥
	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	_, vk, err := groth16.Setup(compiledCircuit)
	require.NoError(t, err)

	// 计算正确的哈希
	var buf bytes.Buffer
	_, err = vk.WriteTo(&buf)
	require.NoError(t, err)
	correctHash := validator.hashManager.SHA256(buf.Bytes())

	// 测试正确的哈希
	err = validator.validateVerifyingKeyHash(vk, correctHash)
	require.NoError(t, err)

	// 测试错误的哈希
	wrongHash := make([]byte, 32)
	copy(wrongHash, correctHash)
	wrongHash[0] ^= 0xFF // 修改第一个字节
	err = validator.validateVerifyingKeyHash(vk, wrongHash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "验证密钥哈希不匹配")
}

// TestValidator_deserializeProof 测试反序列化证明
func TestValidator_deserializeProof(t *testing.T) {
	validator := createTestValidator(t)

	// 创建测试电路和证明
	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	pk, _, err := groth16.Setup(compiledCircuit)
	require.NoError(t, err)

	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 42}, ecc.BN254.ScalarField())
	require.NoError(t, err)

	proof, err := groth16.Prove(compiledCircuit, pk, witness)
	require.NoError(t, err)

	// 序列化证明
	var buf bytes.Buffer
	_, err = proof.WriteTo(&buf)
	require.NoError(t, err)
	proofData := buf.Bytes()

	// 反序列化证明
	deserializedProof, err := validator.deserializeProof(proofData, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, deserializedProof)
}

// TestValidator_buildPublicWitness_ContractExecution 测试构建合约执行电路的公开输入witness
func TestValidator_buildPublicWitness_ContractExecution(t *testing.T) {
	validator := createTestValidator(t)

	publicInputs := [][]byte{[]byte("test_hash")}
	witness, err := validator.buildPublicWitness("contract_execution", publicInputs, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestValidator_buildPublicWitness_AIModelInference 测试构建AI模型推理电路的公开输入witness
func TestValidator_buildPublicWitness_AIModelInference(t *testing.T) {
	validator := createTestValidator(t)

	publicInputs := [][]byte{[]byte("test_hash")}
	witness, err := validator.buildPublicWitness("aimodel_inference", publicInputs, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestValidator_buildPublicWitness_Generic 测试构建通用电路的公开输入witness
func TestValidator_buildPublicWitness_Generic(t *testing.T) {
	validator := createTestValidator(t)

	publicInputs := [][]byte{[]byte("input1"), []byte("input2")}
	witness, err := validator.buildPublicWitness("generic_circuit", publicInputs, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestValidator_buildPublicWitness_EmptyInputs 测试构建空输入的witness
func TestValidator_buildPublicWitness_EmptyInputs(t *testing.T) {
	validator := createTestValidator(t)

	_, err := validator.buildPublicWitness("generic_circuit", [][]byte{}, ecc.BN254)
	require.Error(t, err)
	require.Contains(t, err.Error(), "至少需要1个公开输入")
}

// TestValidator_computeCircuitCommitment 测试计算电路承诺
func TestValidator_computeCircuitCommitment(t *testing.T) {
	validator := createTestValidator(t)

	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	commitment, err := validator.computeCircuitCommitment(compiledCircuit)
	require.NoError(t, err)
	require.NotNil(t, commitment)
	require.Equal(t, 32, len(commitment)) // SHA-256 哈希长度
}

// TestValidator_ValidateProof_UnsupportedScheme 测试不支持的证明方案
func TestValidator_ValidateProof_UnsupportedScheme(t *testing.T) {
	validator := createTestValidator(t)

	proof := &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "contract_execution",
		CircuitVersion:      1,
		ProvingScheme:       "unsupported_scheme",
		Curve:               "bn254",
		VerificationKeyHash: make([]byte, 32),
	}

	valid, err := validator.ValidateProof(context.Background(), proof)
	require.Error(t, err)
	require.False(t, valid)
	require.Contains(t, err.Error(), "不支持的证明方案")
}

// TestValidator_ValidateProof_UnsupportedCurve 测试不支持的椭圆曲线
func TestValidator_ValidateProof_UnsupportedCurve(t *testing.T) {
	validator := createTestValidator(t)

	proof := &transaction.ZKStateProof{
		Proof:               []byte("test"),
		PublicInputs:        [][]byte{[]byte("test")},
		CircuitId:           "contract_execution",
		CircuitVersion:      1,
		ProvingScheme:       "groth16",
		Curve:               "unsupported_curve",
		VerificationKeyHash: make([]byte, 32),
	}

	valid, err := validator.ValidateProof(context.Background(), proof)
	require.Error(t, err)
	require.False(t, valid)
	require.Contains(t, err.Error(), "不支持的椭圆曲线")
}

// createTestValidator 创建测试用的验证器
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func createTestValidator(t *testing.T) *Validator {
	logger := testutil.NewTestLogger()
	circuitManager := NewCircuitManager(logger, &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	})
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	hashManager := testutil.NewTestHashManager()

	return NewValidator(logger, circuitManager, config, hashManager)
}
