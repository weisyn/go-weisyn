package zkproof

import (
	"fmt"
	"sync"
	"time"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// gnark ZK库
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// CircuitManager 电路管理器
//
// 🎯 **专门职责**：负责管理和提供各种类型的ZK电路
// 🏗️ **设计原则**：电路的创建、缓存、版本管理
type CircuitManager struct {
	logger        log.Logger
	config        *ZKProofManagerConfig
	circuits      map[string]frontend.Circuit // 电路缓存
	circuitsMutex sync.RWMutex                // 读写锁保护

	// P1: 电路版本管理
	versionManager *CircuitVersionManager

	// Trusted setup 缓存（proving/verifying key & 已编译电路）
	setupCache map[string]*trustedSetupEntry
	setupMutex sync.RWMutex
}

type trustedSetupEntry struct {
	compiled     constraint.ConstraintSystem
	provingKey   groth16.ProvingKey
	verifyingKey groth16.VerifyingKey
}

// NewCircuitManager 创建电路管理器
func NewCircuitManager(
	logger log.Logger,
	config *ZKProofManagerConfig,
) *CircuitManager {
	return &CircuitManager{
		logger:         logger,
		config:         config,
		circuits:       make(map[string]frontend.Circuit),
		versionManager: NewCircuitVersionManager(logger), // P1: 初始化版本管理器
		setupCache:     make(map[string]*trustedSetupEntry),
	}
}

// GetCircuit 获取电路
func (cm *CircuitManager) GetCircuit(circuitID string, version uint32) (frontend.Circuit, error) {
	circuitKey := fmt.Sprintf("%s.v%d", circuitID, version)

	// 先尝试从缓存获取
	cm.circuitsMutex.RLock()
	if circuit, exists := cm.circuits[circuitKey]; exists {
		cm.circuitsMutex.RUnlock()
		return circuit, nil
	}
	cm.circuitsMutex.RUnlock()

	// 缓存中不存在，创建新电路
	circuit, err := cm.createCircuit(circuitID, version)
	if err != nil {
		return nil, err
	}

	// 加入缓存
	cm.circuitsMutex.Lock()
	cm.circuits[circuitKey] = circuit
	cm.circuitsMutex.Unlock()

	cm.logger.Debugf("电路创建并缓存成功: %s", circuitKey)

	// P1: 注册电路版本信息（如果版本管理器可用）
	if cm.versionManager != nil {
		// 尝试分析约束数量（可能需要编译电路，这里简化处理）
		versionInfo := &CircuitVersionInfo{
			CircuitID:         circuitID,
			Version:           version,
			CreatedAt:         time.Now(),
			ConstraintCount:   0, // 需要实际编译后才能获取
			OptimizationLevel: "basic",
			HashFunction:      "sha256", // 默认使用SHA-256
			Notes:             fmt.Sprintf("电路版本 %d", version),
		}
		cm.versionManager.RegisterCircuitVersion(versionInfo)
	}

	return circuit, nil
}

// LoadCircuit 预加载电路
func (cm *CircuitManager) LoadCircuit(circuitID string, version uint32) error {
	_, err := cm.GetCircuit(circuitID, version)
	return err
}

// IsCircuitLoaded 检查电路是否已加载
func (cm *CircuitManager) IsCircuitLoaded(circuitID string) bool {
	// 检查是否有任何版本的该电路已加载
	cm.circuitsMutex.RLock()
	defer cm.circuitsMutex.RUnlock()

	for key := range cm.circuits {
		if len(key) > len(circuitID) && key[:len(circuitID)] == circuitID {
			return true
		}
	}
	return false
}

// createCircuit 创建具体的电路实例
//
// ⚠️ **注意**：对于Merkle Tree电路，需要使用工厂函数创建，因为需要指定路径深度。
// 如果 circuitID 是 "merkle_path"、"batch_merkle_path" 或 "incremental_update"，
// 需要额外的参数（路径深度），这些电路应该通过工厂函数直接创建，而不是通过此方法。
func (cm *CircuitManager) createCircuit(circuitID string, version uint32) (frontend.Circuit, error) {
	switch circuitID {
	case "contract_execution":
		return cm.createContractExecutionCircuit(version)
	case "aimodel_inference":
		return cm.createAIModelInferenceCircuit(version)
	case "merkle_path", "batch_merkle_path", "incremental_update":
		// ⚠️ **注意**：Merkle Tree电路需要路径深度参数，不能通过此方法创建
		// 应该使用 circuits.NewMerklePathCircuit()、circuits.NewBatchMerklePathCircuit()
		// 或 circuits.NewIncrementalUpdateCircuit() 工厂函数
		return nil, fmt.Errorf("Merkle Tree电路需要通过工厂函数创建，需要指定路径深度参数")
	default:
		return nil, fmt.Errorf("不支持的电路ID: %s", circuitID)
	}
}

// createContractExecutionCircuit 创建合约执行电路
func (cm *CircuitManager) createContractExecutionCircuit(version uint32) (frontend.Circuit, error) {
	switch version {
	case 1:
		return &ContractExecutionCircuit{}, nil
	default:
		return nil, fmt.Errorf("不支持的合约执行电路版本: %d", version)
	}
}

// createAIModelInferenceCircuit 创建AI模型推理电路
func (cm *CircuitManager) createAIModelInferenceCircuit(version uint32) (frontend.Circuit, error) {
	switch version {
	case 1:
		return &AIModelInferenceCircuit{}, nil
	default:
		return nil, fmt.Errorf("不支持的AI模型推理电路版本: %d", version)
	}
}

// GetCircuitVersionInfo 获取电路版本信息
func (cm *CircuitManager) GetCircuitVersionInfo(circuitID string, version uint32) (*CircuitVersionInfo, bool) {
	if cm.versionManager == nil {
		return nil, false
	}
	return cm.versionManager.GetCircuitVersionInfo(circuitID, version)
}

// GetOptimizationReport 获取电路优化报告
func (cm *CircuitManager) GetOptimizationReport(circuitID string, version uint32) (*CircuitOptimizationReport, bool) {
	if cm.versionManager == nil {
		return nil, false
	}
	return cm.versionManager.GetOptimizationReport(circuitID, version)
}

// ListCircuitVersions 列出所有电路版本
func (cm *CircuitManager) ListCircuitVersions(circuitID string) []*CircuitVersionInfo {
	if cm.versionManager == nil {
		return nil
	}
	return cm.versionManager.ListCircuitVersions(circuitID)
}

// GetTrustedSetup 返回指定电路的可信设置（编译电路、ProvingKey、VerifyingKey）
func (cm *CircuitManager) GetTrustedSetup(circuitID string, version uint32) (constraint.ConstraintSystem, groth16.ProvingKey, groth16.VerifyingKey, error) {
	curveID, err := cm.resolveCurveID()
	if err != nil {
		return nil, nil, nil, err
	}

	cacheKey := fmt.Sprintf("%s.v%d:%s", circuitID, version, curveID.String())

	cm.setupMutex.RLock()
	if entry, exists := cm.setupCache[cacheKey]; exists {
		cm.setupMutex.RUnlock()
		return entry.compiled, entry.provingKey, entry.verifyingKey, nil
	}
	cm.setupMutex.RUnlock()

	circuit, err := cm.GetCircuit(circuitID, version)
	if err != nil {
		return nil, nil, nil, err
	}

	compiledCircuit, err := frontend.Compile(curveID.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("编译电路失败: %w", err)
	}

	provingKey, verifyingKey, err := groth16.Setup(compiledCircuit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("生成可信设置失败: %w", err)
	}

	cm.setupMutex.Lock()
	cm.setupCache[cacheKey] = &trustedSetupEntry{
		compiled:     compiledCircuit,
		provingKey:   provingKey,
		verifyingKey: verifyingKey,
	}
	cm.setupMutex.Unlock()

	return compiledCircuit, provingKey, verifyingKey, nil
}

func (cm *CircuitManager) resolveCurveID() (ecc.ID, error) {
	if cm.config == nil || cm.config.DefaultCurve == "" {
		return ecc.BN254, nil
	}

	switch cm.config.DefaultCurve {
	case "bn254":
		return ecc.BN254, nil
	case "bls12-381":
		return ecc.BLS12_381, nil
	case "bls12-377":
		return ecc.BLS12_377, nil
	case "bw6-761":
		return ecc.BW6_761, nil
	default:
		return 0, fmt.Errorf("不支持的椭圆曲线: %s", cm.config.DefaultCurve)
	}
}
