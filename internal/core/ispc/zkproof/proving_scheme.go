package zkproof

import (
	"bytes"
	"fmt"
	"sync"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// gnark ZK库
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/frontend/cs/scs"
)

// ============================================================================
// 证明方案抽象和可扩展性增强
// ============================================================================
//
// 🎯 **目的**：
//   - 抽象证明方案接口，支持多种证明方案
//   - 支持Groth16和PlonK两种主流方案
//   - 实现证明方案切换机制
//
// 📋 **设计原则**：
//   - 方案抽象：定义统一的证明方案接口
//   - 可扩展性：易于添加新的证明方案
//   - 配置驱动：通过配置选择证明方案
//
// ============================================================================

// ProvingScheme 证明方案接口
//
// 🎯 **抽象接口**：定义统一的证明方案操作
type ProvingScheme interface {
	// SchemeName 返回方案名称
	SchemeName() string

	// Setup 生成可信设置（proving key和verifying key）
	Setup(compiledCircuit constraint.ConstraintSystem) (ProvingKey, VerifyingKey, error)

	// Prove 生成证明
	Prove(compiledCircuit constraint.ConstraintSystem, provingKey ProvingKey, witness witness.Witness) (Proof, error)

	// Verify 验证证明
	Verify(proof Proof, verifyingKey VerifyingKey, publicWitness witness.Witness) error

	// SerializeProof 序列化证明
	SerializeProof(proof Proof) ([]byte, error)

	// DeserializeProof 反序列化证明
	DeserializeProof(data []byte, curveID ecc.ID) (Proof, error)

	// SerializeVerifyingKey 序列化验证密钥
	SerializeVerifyingKey(vk VerifyingKey) ([]byte, error)

	// DeserializeVerifyingKey 反序列化验证密钥
	DeserializeVerifyingKey(data []byte, curveID ecc.ID) (VerifyingKey, error)

	// GetBuilder 获取电路构建器
	GetBuilder() frontend.NewBuilder
}

// Proof 证明接口（类型擦除）
type Proof interface{}

// ProvingKey 证明密钥接口（类型擦除）
type ProvingKey interface{}

// VerifyingKey 验证密钥接口（类型擦除）
type VerifyingKey interface{}

// Groth16Scheme Groth16证明方案实现
type Groth16Scheme struct {
	logger log.Logger
}

// NewGroth16Scheme 创建Groth16证明方案
func NewGroth16Scheme(logger log.Logger) *Groth16Scheme {
	return &Groth16Scheme{
		logger: logger,
	}
}

// SchemeName 返回方案名称
func (s *Groth16Scheme) SchemeName() string {
	return "groth16"
}

// GetBuilder 获取电路构建器
func (s *Groth16Scheme) GetBuilder() frontend.NewBuilder {
	return r1cs.NewBuilder
}

// Setup 生成可信设置
func (s *Groth16Scheme) Setup(compiledCircuit constraint.ConstraintSystem) (ProvingKey, VerifyingKey, error) {
	// groth16.Setup 接受实现了 constraint.ConstraintSystem 接口的类型
	// frontend.Compile 返回的类型实现了该接口，可以直接调用
	pk, vk, err := groth16.Setup(compiledCircuit)
	if err != nil {
		return nil, nil, fmt.Errorf("Groth16 Setup失败: %w", err)
	}
	return pk, vk, nil
}

// Prove 生成证明
func (s *Groth16Scheme) Prove(compiledCircuit constraint.ConstraintSystem, provingKey ProvingKey, witness witness.Witness) (Proof, error) {
	// 类型断言：确保 provingKey 是 groth16.ProvingKey 类型
	groth16Pk, ok := provingKey.(groth16.ProvingKey)
	if !ok {
		return nil, fmt.Errorf("无效的Groth16证明密钥类型")
	}

	// groth16.Prove 接受实现了 constraint.ConstraintSystem 接口的类型
	proof, err := groth16.Prove(compiledCircuit, groth16Pk, witness)
	if err != nil {
		return nil, fmt.Errorf("Groth16 Prove失败: %w", err)
	}
	return proof, nil
}

// Verify 验证证明
func (s *Groth16Scheme) Verify(proof Proof, verifyingKey VerifyingKey, publicWitness witness.Witness) error {
	groth16Proof, ok := proof.(groth16.Proof)
	if !ok {
		return fmt.Errorf("无效的Groth16证明类型")
	}

	vk, ok := verifyingKey.(groth16.VerifyingKey)
	if !ok {
		return fmt.Errorf("无效的Groth16验证密钥类型")
	}

	return groth16.Verify(groth16Proof, vk, publicWitness)
}

// SerializeProof 序列化证明
func (s *Groth16Scheme) SerializeProof(proof Proof) ([]byte, error) {
	groth16Proof, ok := proof.(groth16.Proof)
	if !ok {
		return nil, fmt.Errorf("无效的Groth16证明类型")
	}

	var buf bytes.Buffer
	_, err := groth16Proof.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化Groth16证明失败: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeProof 反序列化证明
func (s *Groth16Scheme) DeserializeProof(data []byte, curveID ecc.ID) (Proof, error) {
	proof := groth16.NewProof(curveID)
	reader := bytes.NewReader(data)

	_, err := proof.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("反序列化Groth16证明失败: %w", err)
	}
	return proof, nil
}

// SerializeVerifyingKey 序列化验证密钥
func (s *Groth16Scheme) SerializeVerifyingKey(vk VerifyingKey) ([]byte, error) {
	groth16Vk, ok := vk.(groth16.VerifyingKey)
	if !ok {
		return nil, fmt.Errorf("无效的Groth16验证密钥类型")
	}

	var buf bytes.Buffer
	_, err := groth16Vk.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化Groth16验证密钥失败: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeVerifyingKey 反序列化验证密钥
func (s *Groth16Scheme) DeserializeVerifyingKey(data []byte, curveID ecc.ID) (VerifyingKey, error) {
	vk := groth16.NewVerifyingKey(curveID)
	reader := bytes.NewReader(data)

	_, err := vk.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("反序列化Groth16验证密钥失败: %w", err)
	}
	return vk, nil
}

// PlonKScheme PlonK证明方案实现
type PlonKScheme struct {
	logger log.Logger
}

// NewPlonKScheme 创建PlonK证明方案
func NewPlonKScheme(logger log.Logger) *PlonKScheme {
	return &PlonKScheme{
		logger: logger,
	}
}

// SchemeName 返回方案名称
func (s *PlonKScheme) SchemeName() string {
	return "plonk"
}

// GetBuilder 获取电路构建器
func (s *PlonKScheme) GetBuilder() frontend.NewBuilder {
	return scs.NewBuilder
}

// Setup 生成可信设置
func (s *PlonKScheme) Setup(compiledCircuit constraint.ConstraintSystem) (ProvingKey, VerifyingKey, error) {
	// PlonK 需要 SRS (Structured Reference String) 参数
	// 我们需要根据电路的约束数量生成 SRS
	// 使用默认曲线 BN254（实际应该从配置获取）
	curveID := ecc.BN254

	// 生成 SRS（在实际应用中，SRS 应该预先生成并缓存）
	// kzg.NewSRS 创建一个空的 SRS，plonk.Setup 会根据电路约束数量自动调整
	srs := kzg.NewSRS(curveID)

	// 调用 plonk.Setup，需要两个 SRS 参数（通常使用同一个 SRS）
	pk, vk, err := plonk.Setup(compiledCircuit, srs, srs)
	if err != nil {
		return nil, nil, fmt.Errorf("PlonK Setup失败: %w", err)
	}
	return pk, vk, nil
}

// Prove 生成证明
func (s *PlonKScheme) Prove(compiledCircuit constraint.ConstraintSystem, provingKey ProvingKey, witness witness.Witness) (Proof, error) {
	// 类型断言：确保 provingKey 是 plonk.ProvingKey 类型
	plonkPk, ok := provingKey.(plonk.ProvingKey)
	if !ok {
		return nil, fmt.Errorf("无效的PlonK证明密钥类型")
	}

	// plonk.Prove 接受实现了 constraint.ConstraintSystem 接口的类型
	proof, err := plonk.Prove(compiledCircuit, plonkPk, witness)
	if err != nil {
		return nil, fmt.Errorf("PlonK Prove失败: %w", err)
	}
	return proof, nil
}

// Verify 验证证明
func (s *PlonKScheme) Verify(proof Proof, verifyingKey VerifyingKey, publicWitness witness.Witness) error {
	plonkProof, ok := proof.(plonk.Proof)
	if !ok {
		return fmt.Errorf("无效的PlonK证明类型")
	}

	vk, ok := verifyingKey.(plonk.VerifyingKey)
	if !ok {
		return fmt.Errorf("无效的PlonK验证密钥类型")
	}

	return plonk.Verify(plonkProof, vk, publicWitness)
}

// SerializeProof 序列化证明
func (s *PlonKScheme) SerializeProof(proof Proof) ([]byte, error) {
	plonkProof, ok := proof.(plonk.Proof)
	if !ok {
		return nil, fmt.Errorf("无效的PlonK证明类型")
	}

	var buf bytes.Buffer
	_, err := plonkProof.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化PlonK证明失败: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeProof 反序列化证明
func (s *PlonKScheme) DeserializeProof(data []byte, curveID ecc.ID) (Proof, error) {
	proof := plonk.NewProof(curveID)
	reader := bytes.NewReader(data)

	_, err := proof.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("反序列化PlonK证明失败: %w", err)
	}
	return proof, nil
}

// SerializeVerifyingKey 序列化验证密钥
func (s *PlonKScheme) SerializeVerifyingKey(vk VerifyingKey) ([]byte, error) {
	plonkVk, ok := vk.(plonk.VerifyingKey)
	if !ok {
		return nil, fmt.Errorf("无效的PlonK验证密钥类型")
	}

	var buf bytes.Buffer
	_, err := plonkVk.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化PlonK验证密钥失败: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeVerifyingKey 反序列化验证密钥
func (s *PlonKScheme) DeserializeVerifyingKey(data []byte, curveID ecc.ID) (VerifyingKey, error) {
	vk := plonk.NewVerifyingKey(curveID)
	reader := bytes.NewReader(data)

	_, err := vk.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("反序列化PlonK验证密钥失败: %w", err)
	}
	return vk, nil
}

// ProvingSchemeRegistry 证明方案注册表
type ProvingSchemeRegistry struct {
	logger  log.Logger
	schemes map[string]ProvingScheme
	mutex   sync.RWMutex
}

// NewProvingSchemeRegistry 创建证明方案注册表
func NewProvingSchemeRegistry(logger log.Logger) *ProvingSchemeRegistry {
	registry := &ProvingSchemeRegistry{
		logger:  logger,
		schemes: make(map[string]ProvingScheme),
	}

	// 注册默认方案
	registry.RegisterScheme(NewGroth16Scheme(logger))
	registry.RegisterScheme(NewPlonKScheme(logger))

	return registry
}

// RegisterScheme 注册证明方案
func (r *ProvingSchemeRegistry) RegisterScheme(scheme ProvingScheme) {
	if scheme == nil {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	schemeName := scheme.SchemeName()
	r.schemes[schemeName] = scheme

	if r.logger != nil {
		r.logger.Debugf("注册证明方案: %s", schemeName)
	}
}

// GetScheme 获取证明方案
func (r *ProvingSchemeRegistry) GetScheme(schemeName string) (ProvingScheme, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	scheme, exists := r.schemes[schemeName]
	if !exists {
		return nil, fmt.Errorf("未注册的证明方案: %s", schemeName)
	}

	return scheme, nil
}

// ListSchemes 列出所有注册的方案
func (r *ProvingSchemeRegistry) ListSchemes() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	schemes := make([]string, 0, len(r.schemes))
	for name := range r.schemes {
		schemes = append(schemes, name)
	}

	return schemes
}

// IsSchemeSupported 检查方案是否支持
func (r *ProvingSchemeRegistry) IsSchemeSupported(schemeName string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.schemes[schemeName]
	return exists
}
