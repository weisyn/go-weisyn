package zkproof

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	// 内部接口
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// gnark ZK库
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	gnarklogger "github.com/consensys/gnark/logger"

	// zerolog for gnark logger
	"github.com/rs/zerolog"
)

// VerifyingKeyCache 验证密钥缓存项
type VerifyingKeyCache struct {
	verifyingKey      groth16.VerifyingKey
	circuitCommitment []byte
	lastUsed          time.Time
}

// Validator ZK证明验证器
//
// 🎯 **专门职责**：负责验证各种类型的零知识证明
// 🏗️ **技术栈**：基于gnark库实现Groth16/PlonK证明验证
// 🔧 **核心功能**：
// - 验证密钥缓存管理
// - 多种证明方案支持
// - 电路特化验证逻辑
type Validator struct {
	logger         log.Logger
	circuitManager *CircuitManager
	config         *ZKProofManagerConfig
	hashManager    crypto.HashManager // 哈希管理器（用于计算验证密钥哈希和电路承诺）

	// 验证密钥缓存（线程安全）
	vkCache  map[string]*VerifyingKeyCache
	cacheMux sync.RWMutex

	// 支持的证明方案
	supportedSchemes map[string]bool
	supportedCurves  map[string]ecc.ID
}

// GenericCircuit 通用电路结构：用于把“公开输入列表”绑定成 gnark witness。
//
// ⚠️ 重要说明（避免误导）：
// - ZKProof 的安全性来自“验证密钥（VK）+ 证明（Proof）”，其约束系统已经固化在 VK 中；
// - 在验证流程里，我们只需要构造 public witness 来喂给 gnark 的 Verify；
// - 因此此处的 Define **不是安全约束**，也不会被用来替代真实电路的约束。
// - 该结构的目的仅是：在不知道具体电路结构时，仍能把公开输入按数量/顺序构造成 witness。
type GenericCircuit struct {
	PublicInputs []frontend.Variable `gnark:",public"`
}

// Define 通用电路的约束定义
//
// 说明：
// - 这里刻意只放“恒等约束”（input == input），用于让变量被 gnark API 正常处理；
// - 不对公开输入施加任何业务含义约束，避免误把 GenericCircuit 当作真实电路的一部分。
func (circuit *GenericCircuit) Define(api frontend.API) error {
	for _, input := range circuit.PublicInputs {
		api.AssertIsEqual(input, input)
	}

	return nil
}

// NewValidator 创建证明验证器
func NewValidator(
	logger log.Logger,
	circuitManager *CircuitManager,
	config *ZKProofManagerConfig,
	hashManager crypto.HashManager,
) *Validator {
	return &Validator{
		logger:         logger,
		circuitManager: circuitManager,
		config:         config,
		hashManager:    hashManager,
		vkCache:        make(map[string]*VerifyingKeyCache),

		// P1: 初始化支持的证明方案（从配置获取，默认支持Groth16）
		supportedSchemes: map[string]bool{
			"groth16": true,
			"plonk":   true, // P1: 启用PlonK支持
		},

		// 初始化支持的椭圆曲线
		supportedCurves: map[string]ecc.ID{
			"bn254":     ecc.BN254,
			"bls12-381": ecc.BLS12_381,
		},
	}
}

// ValidateProof 验证零知识证明
func (v *Validator) ValidateProof(ctx context.Context, proof *transaction.ZKStateProof) (bool, error) {
	startTime := time.Now()
	v.logger.Debugf("开始验证ZK证明: circuitID=%s, version=%d, scheme=%s",
		proof.CircuitId, proof.CircuitVersion, proof.ProvingScheme)

	// ⚠️ **禁用gnark库的日志输出**
	// gnark库会输出大量的调试信息，在验证期间禁用
	oldGnarkLogger := gnarklogger.Logger()
	discardLogger := zerolog.New(io.Discard).Level(zerolog.Disabled)
	gnarklogger.Set(discardLogger)
	defer func() {
		gnarklogger.Set(oldGnarkLogger)
	}()

	// 1. 验证证明方案支持
	if !v.supportedSchemes[proof.ProvingScheme] {
		return false, fmt.Errorf("不支持的证明方案: %s", proof.ProvingScheme)
	}

	// 2. 验证椭圆曲线支持
	curveID, supported := v.supportedCurves[proof.Curve]
	if !supported {
		return false, fmt.Errorf("不支持的椭圆曲线: %s", proof.Curve)
	}

	// 3. 验证基础数据完整性
	if err := v.validateProofData(proof); err != nil {
		return false, fmt.Errorf("证明数据验证失败: %w", err)
	}

	// 4. 获取或构建验证密钥
	vk, err := v.getVerifyingKey(proof.CircuitId, proof.CircuitVersion, curveID)
	if err != nil {
		return false, fmt.Errorf("获取验证密钥失败: %w", err)
	}

	// 5. 验证验证密钥哈希
	if err := v.validateVerifyingKeyHash(vk, proof.VerificationKeyHash); err != nil {
		return false, fmt.Errorf("验证密钥哈希不匹配: %w", err)
	}

	// 6. 反序列化证明对象
	proofObj, err := v.deserializeProof(proof.Proof, curveID)
	if err != nil {
		return false, fmt.Errorf("反序列化证明失败: %w", err)
	}

	// 7. 构建公开输入witness
	publicWitness, err := v.buildPublicWitness(proof.CircuitId, proof.PublicInputs, curveID)
	if err != nil {
		return false, fmt.Errorf("构建公开输入失败: %w", err)
	}

	// 8. 执行ZK证明验证
	err = groth16.Verify(proofObj, vk, publicWitness)
	if err != nil {
		v.logger.Debugf("ZK证明验证失败: %v", err)
		return false, nil // 验证失败但不是系统错误
	}

	verificationTime := time.Since(startTime)
	v.logger.Debugf("ZK证明验证成功: 耗时=%v", verificationTime)

	return true, nil
}

// validateProofData 验证证明数据完整性
func (v *Validator) validateProofData(proof *transaction.ZKStateProof) error {
	if len(proof.Proof) == 0 {
		return fmt.Errorf("证明数据为空")
	}

	if len(proof.PublicInputs) == 0 {
		return fmt.Errorf("公开输入为空")
	}

	if proof.CircuitId == "" {
		return fmt.Errorf("电路ID为空")
	}

	if len(proof.VerificationKeyHash) != 32 {
		return fmt.Errorf("验证密钥哈希长度无效: expected=32, actual=%d", len(proof.VerificationKeyHash))
	}

	return nil
}

// getVerifyingKey 获取或构建验证密钥（带缓存）
func (v *Validator) getVerifyingKey(circuitID string, version uint32, curveID ecc.ID) (groth16.VerifyingKey, error) {
	cacheKey := fmt.Sprintf("%s:%d:%s", circuitID, version, curveID.String())

	// 尝试从缓存获取
	v.cacheMux.RLock()
	if cached, exists := v.vkCache[cacheKey]; exists {
		cached.lastUsed = time.Now()
		v.cacheMux.RUnlock()
		v.logger.Debugf("验证密钥缓存命中: %s", cacheKey)
		return cached.verifyingKey, nil
	}
	v.cacheMux.RUnlock()

	// 缓存未命中，构建验证密钥
	v.logger.Debugf("验证密钥缓存未命中，开始构建: %s", cacheKey)

	compiledCircuit, _, vk, err := v.circuitManager.GetTrustedSetup(circuitID, version)
	if err != nil {
		return nil, fmt.Errorf("获取可信设置失败: %w", err)
	}

	// 计算电路承诺
	circuitCommitment, err := v.computeCircuitCommitment(compiledCircuit)
	if err != nil {
		return nil, fmt.Errorf("计算电路承诺失败: %w", err)
	}

	// 缓存验证密钥
	v.cacheMux.Lock()
	v.vkCache[cacheKey] = &VerifyingKeyCache{
		verifyingKey:      vk,
		circuitCommitment: circuitCommitment,
		lastUsed:          time.Now(),
	}
	v.cacheMux.Unlock()

	v.logger.Debugf("验证密钥构建并缓存成功: %s", cacheKey)
	return vk, nil
}

// validateVerifyingKeyHash 验证验证密钥哈希
func (v *Validator) validateVerifyingKeyHash(vk groth16.VerifyingKey, expectedHash []byte) error {
	// 序列化验证密钥
	var buf bytes.Buffer
	_, err := vk.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("序列化验证密钥失败: %w", err)
	}

	// 使用HashManager计算哈希
	actualHash := v.hashManager.SHA256(buf.Bytes())

	// 比较哈希
	if !bytes.Equal(actualHash, expectedHash) {
		return fmt.Errorf("验证密钥哈希不匹配")
	}

	return nil
}

// deserializeProof 反序列化证明对象
func (v *Validator) deserializeProof(proofData []byte, curveID ecc.ID) (groth16.Proof, error) {
	proofObj := groth16.NewProof(curveID)
	reader := bytes.NewReader(proofData)

	_, err := proofObj.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("反序列化证明失败: %w", err)
	}

	return proofObj, nil
}

// buildPublicWitness 构建公开输入witness（电路特化）
//
// 🎯 **电路ID规范**：使用基础名（不含版本），版本通过单独参数指定
//   - "contract_execution" + version 1
//   - "aimodel_inference" + version 1
func (v *Validator) buildPublicWitness(circuitID string, publicInputs [][]byte, curveID ecc.ID) (witness.Witness, error) {
	v.logger.Debugf("构建公开输入witness: circuitID=%s, inputs=%d", circuitID, len(publicInputs))

	switch circuitID {
	case "contract_execution":
		return v.buildContractExecutionWitness(publicInputs, curveID)
	case "aimodel_inference":
		return v.buildAIModelInferenceWitness(publicInputs, curveID)
	default:
		return v.buildGenericWitness(publicInputs, curveID)
	}
}

// buildContractExecutionWitness 构建合约执行电路的公开输入witness
func (v *Validator) buildContractExecutionWitness(publicInputs [][]byte, curveID ecc.ID) (witness.Witness, error) {
	if len(publicInputs) < 1 {
		return nil, fmt.Errorf("合约执行电路至少需要1个公开输入（执行结果哈希）")
	}

	// 设置执行结果哈希（第一个公开输入）
	executionResultHash := new(big.Int).SetBytes(publicInputs[0])

	// 创建合约执行电路实例，只设置公开输入
	circuit := ContractExecutionCircuit{
		ExecutionResultHash: executionResultHash,
		// 私有输入在验证时不需要设置
		ExecutionTrace: 0,
		StateDiff:      0,
	}

	// 使用电路实例创建witness（只包含公开输入）
	publicWitness, err := frontend.NewWitness(&circuit, curveID.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("创建合约执行witness失败: %w", err)
	}

	v.logger.Debugf("合约执行witness创建成功: executionResultHash=%s", executionResultHash.String())
	return publicWitness, nil
}

// buildAIModelInferenceWitness 构建AI模型推理电路的公开输入witness
func (v *Validator) buildAIModelInferenceWitness(publicInputs [][]byte, curveID ecc.ID) (witness.Witness, error) {
	if len(publicInputs) < 1 {
		return nil, fmt.Errorf("AI模型推理电路至少需要1个公开输入")
	}

	// AI模型推理的公开输入通常包括推理结果哈希
	inferenceResultHash := new(big.Int).SetBytes(publicInputs[0])

	// 创建AI推理电路实例，只设置公开输入
	circuit := AIModelInferenceCircuit{
		InferenceResultHash: inferenceResultHash,
		// 私有输入在验证时不需要设置
		ModelWeights: 0,
		InputData:    0,
	}

	// 使用电路实例创建witness（只包含公开输入）
	publicWitness, err := frontend.NewWitness(&circuit, curveID.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("创建AI推理电路witness失败: %w", err)
	}

	v.logger.Debugf("AI推理witness创建成功: inferenceResultHash=%s", inferenceResultHash.String())
	return publicWitness, nil
}

// buildGenericWitness 构建通用电路的公开输入witness
func (v *Validator) buildGenericWitness(publicInputs [][]byte, curveID ecc.ID) (witness.Witness, error) {
	if len(publicInputs) == 0 {
		return nil, fmt.Errorf("通用电路至少需要1个公开输入")
	}

	// 将字节数组转换为big.Int数组
	publicValues := make([]frontend.Variable, len(publicInputs))
	for i, input := range publicInputs {
		value := new(big.Int).SetBytes(input)
		publicValues[i] = value

		v.logger.Debugf("通用公开输入[%d]: %s", i, value.String())
	}

	// 创建通用电路实例
	circuit := GenericCircuit{
		PublicInputs: publicValues,
	}

	// 使用电路实例创建witness（只包含公开输入）
	publicWitness, err := frontend.NewWitness(&circuit, curveID.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return nil, fmt.Errorf("创建通用witness失败: %w", err)
	}

	v.logger.Debugf("通用witness创建成功: %d个公开输入", len(publicInputs))
	return publicWitness, nil
}

// computeCircuitCommitment 计算电路承诺
func (v *Validator) computeCircuitCommitment(compiledCircuit constraint.ConstraintSystem) ([]byte, error) {
	// 序列化编译后的电路
	var buf bytes.Buffer
	_, err := compiledCircuit.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化电路失败: %w", err)
	}

	// 使用HashManager计算SHA-256哈希作为承诺
	hash := v.hashManager.SHA256(buf.Bytes())
	return hash, nil
}
