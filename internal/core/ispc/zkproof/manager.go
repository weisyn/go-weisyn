package zkproof

import (
	"context"
	"fmt"

	// 公共接口依赖
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 内部接口
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Manager 零知识证明管理器
//
// 🎯 **设计理念**：薄实现，专注依赖注入和接口协调
// 🏗️ **架构原则**：符合WES三层架构，Manager只做依赖管理，业务逻辑委托给子组件
type Manager struct {
	// ==================== 密码学服务 ====================
	hashManager      crypto.HashManager      // 哈希计算服务
	signatureManager crypto.SignatureManager // 签名服务

	// ==================== 基础设施服务 ====================
	logger         log.Logger      // 日志服务
	configProvider config.Provider // 配置提供者

	// ==================== 专门的子组件（真实实现） ====================
	prover         *Prover         // ZK证明生成器
	validator      *Validator      // ZK证明验证器
	circuitManager *CircuitManager // 电路管理器
	
	// P0: 证明生成可靠性增强器
	reliabilityEnforcer *ProofReliabilityEnforcer
	
	// P1: 证明方案注册表
	schemeRegistry *ProvingSchemeRegistry

	// ==================== 预留扩展接口 ====================
	circuitCache interface{} // 电路缓存（待扩展）
	metrics      interface{} // 指标收集服务（待扩展）

	// ==================== 配置参数 ====================
	config *ZKProofManagerConfig
}

// ZKProofManagerConfig ZK证明管理器配置
type ZKProofManagerConfig struct {
	// 证明方案配置
	DefaultProvingScheme string // 默认证明方案 (groth16, plonk)
	DefaultCurve         string // 默认椭圆曲线 (bn254, bls12-381)

	// 性能配置
	MaxConcurrentProofs int  // 最大并发证明数
	ProofTimeoutSeconds int  // 证明超时时间
	CircuitCacheSize    int  // 电路缓存大小
	EnableParallelSetup bool // 启用并行可信设置

	// 存储配置
	TrustedSetupPath     string // 可信设置路径
	ValidateSetupOnStart bool   // 启动时验证可信设置
}

// NewManager 创建零知识证明管理器
//
// 🎯 **依赖注入模式**：通过构造函数注入所有依赖
// 🏗️ **初始化顺序**：基础服务 → 配置 → 子组件 → 组装Manager
func NewManager(
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	logger log.Logger,
	configProvider config.Provider,
) *Manager {

	// 创建默认配置
	config := &ZKProofManagerConfig{
		// 证明方案配置
		DefaultProvingScheme: "groth16", // 使用Groth16作为默认方案
		DefaultCurve:         "bn254",   // 使用BN254曲线

		// 性能配置
		MaxConcurrentProofs: 4,    // 最大4个并发证明
		ProofTimeoutSeconds: 300,  // 5分钟超时
		CircuitCacheSize:    100,  // 缓存100个电路
		EnableParallelSetup: true, // 启用并行可信设置

		// 存储配置
		TrustedSetupPath:     "/var/zkproof/trusted_setup", // 可信设置路径
		ValidateSetupOnStart: true,                         // 启动时验证可信设置
	}

	// 创建专门的子组件
	circuitManager := NewCircuitManager(logger, config)
	prover := NewProver(logger, hashManager, circuitManager, config)
	validator := NewValidator(logger, circuitManager, config, hashManager)
	
	// P0: 创建证明生成可靠性增强器
	reliabilityEnforcer := NewProofReliabilityEnforcer(logger, prover, validator, nil)

	return &Manager{
		// 密码学服务
		hashManager:      hashManager,
		signatureManager: signatureManager,

		// 基础设施服务
		logger:         logger,
		configProvider: configProvider,

	// 专门的子组件
	prover:         prover,
	validator:      validator,
	circuitManager: circuitManager,
	
	// P0: 证明生成可靠性增强器
	reliabilityEnforcer: reliabilityEnforcer,
	
	// P1: 证明方案注册表
	schemeRegistry: NewProvingSchemeRegistry(logger),

	// 占位：未来扩展
	circuitCache: nil,
	metrics:      nil,

	// 配置参数
	config: config,
}
}

// ==================== ZKProofManager接口实现（薄实现） ====================

// GenerateProof 生成零知识证明（委托给Prover子组件）
func (m *Manager) GenerateProof(ctx context.Context, input *interfaces.ZKProofInput) (*interfaces.ZKProofResult, error) {
	return m.prover.GenerateProof(ctx, input)
}

// GenerateStateProof 生成状态证明（委托给Prover子组件）
func (m *Manager) GenerateStateProof(ctx context.Context, input *interfaces.ZKProofInput) (*transaction.ZKStateProof, error) {
	// P0: 使用可靠性增强器（带重试和验证自检）
	if m.reliabilityEnforcer != nil {
		return m.reliabilityEnforcer.GenerateStateProofWithRetry(ctx, input)
	}
	
	// 回退到直接调用Prover（兼容性）
	return m.prover.GenerateStateProof(ctx, input)
}

// GetSchemeRegistry 获取证明方案注册表
func (m *Manager) GetSchemeRegistry() *ProvingSchemeRegistry {
	return m.schemeRegistry
}

// GetScheme 获取指定的证明方案
func (m *Manager) GetScheme(schemeName string) (ProvingScheme, error) {
	if m.schemeRegistry == nil {
		return nil, fmt.Errorf("证明方案注册表未初始化")
	}
	return m.schemeRegistry.GetScheme(schemeName)
}

// ListSupportedSchemes 列出所有支持的证明方案
func (m *Manager) ListSupportedSchemes() []string {
	if m.schemeRegistry == nil {
		return []string{}
	}
	return m.schemeRegistry.ListSchemes()
}

// IsSchemeSupported 检查证明方案是否支持
func (m *Manager) IsSchemeSupported(schemeName string) bool {
	if m.schemeRegistry == nil {
		return false
	}
	return m.schemeRegistry.IsSchemeSupported(schemeName)
}

// GetDefaultProvingScheme 获取默认证明方案
//
// 📋 **返回值**：
//   - string: 默认证明方案名称（如 "groth16"）
func (m *Manager) GetDefaultProvingScheme() string {
	if m.config == nil {
		return "groth16" // 默认值
	}
	return m.config.DefaultProvingScheme
}

// GetDefaultCurve 获取默认椭圆曲线
//
// 📋 **返回值**：
//   - string: 默认椭圆曲线名称（如 "bn254"）
func (m *Manager) GetDefaultCurve() string {
	if m.config == nil {
		return "bn254" // 默认值
	}
	return m.config.DefaultCurve
}

// ==================== P0: 证明生成可靠性增强方法 ====================

// GenerateProofWithRetry 带重试机制的证明生成
func (m *Manager) GenerateProofWithRetry(ctx context.Context, input *interfaces.ZKProofInput) (*interfaces.ZKProofResult, error) {
	if m.reliabilityEnforcer == nil {
		return nil, fmt.Errorf("可靠性增强器未初始化")
	}
	return m.reliabilityEnforcer.GenerateProofWithRetry(ctx, input)
}

// GetErrorLogs 获取错误日志（用于故障排查）
func (m *Manager) GetErrorLogs(limit int) []ProofGenerationErrorLog {
	if m.reliabilityEnforcer == nil {
		return nil
	}
	return m.reliabilityEnforcer.GetErrorLogs(limit)
}

// GetErrorStats 获取错误统计信息
func (m *Manager) GetErrorStats() map[string]interface{} {
	if m.reliabilityEnforcer == nil {
		return map[string]interface{}{
			"error": "可靠性增强器未初始化",
		}
	}
	return m.reliabilityEnforcer.GetErrorStats()
}

// ClearErrorLogs 清空错误日志
func (m *Manager) ClearErrorLogs() {
	if m.reliabilityEnforcer != nil {
		m.reliabilityEnforcer.ClearErrorLogs()
	}
}

// LoadCircuit 加载证明电路（委托给CircuitManager子组件）
func (m *Manager) LoadCircuit(circuitID string, circuitVersion uint32) error {
	return m.circuitManager.LoadCircuit(circuitID, circuitVersion)
}

// IsCircuitLoaded 检查电路是否已加载（委托给CircuitManager子组件）
func (m *Manager) IsCircuitLoaded(circuitID string) bool {
	return m.circuitManager.IsCircuitLoaded(circuitID)
}
