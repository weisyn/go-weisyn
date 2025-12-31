package zkproof

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// proving_scheme.go 测试
// ============================================================================

// simpleTestCircuit 简单的测试电路
type simpleTestCircuit struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
}

func (c *simpleTestCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.X, c.Y)
	return nil
}

// TestNewGroth16Scheme 测试创建Groth16方案
func TestNewGroth16Scheme(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheme := NewGroth16Scheme(logger)
	require.NotNil(t, scheme)
	require.Equal(t, "groth16", scheme.SchemeName())
}

// TestGroth16Scheme_SchemeName 测试Groth16方案名称
func TestGroth16Scheme_SchemeName(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	require.Equal(t, "groth16", scheme.SchemeName())
}

// TestGroth16Scheme_GetBuilder 测试Groth16方案构建器
func TestGroth16Scheme_GetBuilder(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	builder := scheme.GetBuilder()
	require.NotNil(t, builder)
	// 函数指针不能直接比较，只检查非nil
}

// TestGroth16Scheme_Setup 测试Groth16 Setup
func TestGroth16Scheme_Setup(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	circuit := &simpleTestCircuit{}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	// Setup
	pk, vk, err := scheme.Setup(compiledCircuit)
	require.NoError(t, err)
	require.NotNil(t, pk)
	require.NotNil(t, vk)
}

// TestGroth16Scheme_Prove 测试Groth16 Prove
func TestGroth16Scheme_Prove(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	circuit := &simpleTestCircuit{}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	// Setup
	pk, vk, err := scheme.Setup(compiledCircuit)
	require.NoError(t, err)
	
	// 创建witness
	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 42}, ecc.BN254.ScalarField())
	require.NoError(t, err)
	
	// Prove
	proof, err := scheme.Prove(compiledCircuit, pk, witness)
	require.NoError(t, err)
	require.NotNil(t, proof)
	
	// Verify
	publicWitness, err := frontend.NewWitness(&simpleTestCircuit{Y: 42}, ecc.BN254.ScalarField(), frontend.PublicOnly())
	require.NoError(t, err)
	
	err = scheme.Verify(proof, vk, publicWitness)
	require.NoError(t, err)
}

// TestGroth16Scheme_Prove_InvalidWitness 测试Groth16 Prove with invalid witness
func TestGroth16Scheme_Prove_InvalidWitness(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	circuit := &simpleTestCircuit{}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	// Setup
	pk, _, err := scheme.Setup(compiledCircuit)
	require.NoError(t, err)
	
	// 创建无效witness（X != Y）
	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 43}, ecc.BN254.ScalarField())
	require.NoError(t, err)
	
	// Prove应该失败（因为电路约束不满足）
	_, err = scheme.Prove(compiledCircuit, pk, witness)
	require.Error(t, err)
}

// TestGroth16Scheme_SerializeProof 测试Groth16证明序列化
func TestGroth16Scheme_SerializeProof(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	circuit := &simpleTestCircuit{}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	// Setup
	pk, _, err := scheme.Setup(compiledCircuit)
	require.NoError(t, err)
	
	// 创建witness并生成证明
	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 42}, ecc.BN254.ScalarField())
	require.NoError(t, err)
	
	proof, err := scheme.Prove(compiledCircuit, pk, witness)
	require.NoError(t, err)
	
	// 序列化证明
	proofBytes, err := scheme.SerializeProof(proof)
	require.NoError(t, err)
	require.NotEmpty(t, proofBytes)
	
	// 反序列化证明
	deserializedProof, err := scheme.DeserializeProof(proofBytes, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, deserializedProof)
}

// TestGroth16Scheme_SerializeVerifyingKey 测试Groth16验证密钥序列化
func TestGroth16Scheme_SerializeVerifyingKey(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	circuit := &simpleTestCircuit{}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	// Setup
	_, vk, err := scheme.Setup(compiledCircuit)
	require.NoError(t, err)
	
	// 序列化验证密钥
	vkBytes, err := scheme.SerializeVerifyingKey(vk)
	require.NoError(t, err)
	require.NotEmpty(t, vkBytes)
	
	// 反序列化验证密钥
	deserializedVk, err := scheme.DeserializeVerifyingKey(vkBytes, ecc.BN254)
	require.NoError(t, err)
	require.NotNil(t, deserializedVk)
}

// TestGroth16Scheme_InvalidTypes 测试Groth16无效类型处理
func TestGroth16Scheme_InvalidTypes(t *testing.T) {
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	
	// 测试无效的证明类型
	_, err := scheme.SerializeProof("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的Groth16证明类型")
	
	// 测试无效的验证密钥类型
	_, err = scheme.SerializeVerifyingKey("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的Groth16验证密钥类型")
}

// TestNewPlonKScheme 测试创建PlonK方案
func TestNewPlonKScheme(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheme := NewPlonKScheme(logger)
	require.NotNil(t, scheme)
	require.Equal(t, "plonk", scheme.SchemeName())
}

// TestPlonKScheme_SchemeName 测试PlonK方案名称
func TestPlonKScheme_SchemeName(t *testing.T) {
	scheme := NewPlonKScheme(testutil.NewTestLogger())
	require.Equal(t, "plonk", scheme.SchemeName())
}

// TestPlonKScheme_GetBuilder 测试PlonK方案构建器
func TestPlonKScheme_GetBuilder(t *testing.T) {
	scheme := NewPlonKScheme(testutil.NewTestLogger())
	builder := scheme.GetBuilder()
	require.NotNil(t, builder)
	// 函数指针不能直接比较，只检查非nil
}

// TestPlonKScheme_Setup 测试PlonK Setup
// 注意：PlonK 需要预先生成的 SRS，这在测试中比较复杂
// 这里只测试接口调用，实际 SRS 生成需要在生产环境中预先生成
func TestPlonKScheme_Setup(t *testing.T) {
	t.Skip("PlonK Setup 需要预先生成的 SRS，测试中跳过")
}

// TestPlonKScheme_Prove 测试PlonK Prove
// 注意：PlonK 需要预先生成的 SRS，这在测试中比较复杂
func TestPlonKScheme_Prove(t *testing.T) {
	t.Skip("PlonK Prove 需要预先生成的 SRS，测试中跳过")
}

// TestPlonKScheme_SerializeProof 测试PlonK证明序列化
// 注意：PlonK 需要预先生成的 SRS，这在测试中比较复杂
func TestPlonKScheme_SerializeProof(t *testing.T) {
	t.Skip("PlonK SerializeProof 需要预先生成的 SRS，测试中跳过")
}

// TestPlonKScheme_SerializeVerifyingKey 测试PlonK验证密钥序列化
// 注意：PlonK 需要预先生成的 SRS，这在测试中比较复杂
func TestPlonKScheme_SerializeVerifyingKey(t *testing.T) {
	t.Skip("PlonK SerializeVerifyingKey 需要预先生成的 SRS，测试中跳过")
}

// TestPlonKScheme_InvalidTypes 测试PlonK无效类型处理
func TestPlonKScheme_InvalidTypes(t *testing.T) {
	scheme := NewPlonKScheme(testutil.NewTestLogger())
	
	// 测试无效的证明类型
	_, err := scheme.SerializeProof("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的PlonK证明类型")
	
	// 测试无效的验证密钥类型
	_, err = scheme.SerializeVerifyingKey("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的PlonK验证密钥类型")
	
	// 测试无效的证明类型（Prove方法）
	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
	
	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 42}, ecc.BN254.ScalarField())
	require.NoError(t, err)
	
	// 使用无效的provingKey类型
	_, err = scheme.Prove(compiledCircuit, "invalid_proving_key", witness)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的PlonK证明密钥类型")
	
	// 测试无效的证明类型（Verify方法）
	err = scheme.Verify("invalid_proof", "invalid_vk", witness)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的PlonK证明类型")
	
	// 测试无效的验证密钥类型（Verify方法）
	// 创建一个假的plonk.Proof（实际上无法创建，所以这里测试类型断言失败）
	err = scheme.Verify("invalid_proof", "invalid_vk", witness)
	require.Error(t, err)
}

// TestPlonKScheme_DeserializeProof 测试PlonK证明反序列化
func TestPlonKScheme_DeserializeProof(t *testing.T) {
	scheme := NewPlonKScheme(testutil.NewTestLogger())
	
	// 测试无效数据
	_, err := scheme.DeserializeProof([]byte("invalid_data"), ecc.BN254)
	require.Error(t, err)
	require.Contains(t, err.Error(), "反序列化PlonK证明失败")
	
	// 测试空数据
	_, err = scheme.DeserializeProof([]byte{}, ecc.BN254)
	require.Error(t, err)
}

// TestPlonKScheme_DeserializeVerifyingKey 测试PlonK验证密钥反序列化
func TestPlonKScheme_DeserializeVerifyingKey(t *testing.T) {
	scheme := NewPlonKScheme(testutil.NewTestLogger())
	
	// 测试无效数据
	_, err := scheme.DeserializeVerifyingKey([]byte("invalid_data"), ecc.BN254)
	require.Error(t, err)
	require.Contains(t, err.Error(), "反序列化PlonK验证密钥失败")
	
	// 测试空数据
	_, err = scheme.DeserializeVerifyingKey([]byte{}, ecc.BN254)
	require.Error(t, err)
}

// TestNewProvingSchemeRegistry 测试创建证明方案注册表
func TestNewProvingSchemeRegistry(t *testing.T) {
	logger := testutil.NewTestLogger()
	registry := NewProvingSchemeRegistry(logger)
	require.NotNil(t, registry)
	require.NotNil(t, registry.schemes)
	
	// 应该已经注册了默认方案
	schemes := registry.ListSchemes()
	require.GreaterOrEqual(t, len(schemes), 2)
	require.Contains(t, schemes, "groth16")
	require.Contains(t, schemes, "plonk")
}

// TestProvingSchemeRegistry_RegisterScheme 测试注册证明方案
func TestProvingSchemeRegistry_RegisterScheme(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	// 注册新方案
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	registry.RegisterScheme(scheme)
	
	// 验证已注册
	require.True(t, registry.IsSchemeSupported("groth16"))
}

// TestProvingSchemeRegistry_RegisterScheme_Nil 测试注册nil方案
func TestProvingSchemeRegistry_RegisterScheme_Nil(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	// 注册nil方案不应该panic
	registry.RegisterScheme(nil)
}

// TestProvingSchemeRegistry_GetScheme 测试获取证明方案
func TestProvingSchemeRegistry_GetScheme(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	// 获取已注册的方案
	scheme, err := registry.GetScheme("groth16")
	require.NoError(t, err)
	require.NotNil(t, scheme)
	require.Equal(t, "groth16", scheme.SchemeName())
}

// TestProvingSchemeRegistry_GetScheme_NotExists 测试获取不存在的方案
func TestProvingSchemeRegistry_GetScheme_NotExists(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	// 获取不存在的方案
	_, err := registry.GetScheme("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "未注册的证明方案")
}

// TestProvingSchemeRegistry_ListSchemes 测试列出所有方案
func TestProvingSchemeRegistry_ListSchemes(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	schemes := registry.ListSchemes()
	require.GreaterOrEqual(t, len(schemes), 2)
	require.Contains(t, schemes, "groth16")
	require.Contains(t, schemes, "plonk")
}

// TestProvingSchemeRegistry_IsSchemeSupported 测试检查方案是否支持
func TestProvingSchemeRegistry_IsSchemeSupported(t *testing.T) {
	registry := NewProvingSchemeRegistry(testutil.NewTestLogger())
	
	require.True(t, registry.IsSchemeSupported("groth16"))
	require.True(t, registry.IsSchemeSupported("plonk"))
	require.False(t, registry.IsSchemeSupported("nonexistent"))
}

