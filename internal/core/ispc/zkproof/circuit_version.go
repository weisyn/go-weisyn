package zkproof

import (
	"fmt"
	"sync"
	"time"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// gnark ZK库
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// ============================================================================
// 电路版本管理和优化工具
// ============================================================================
//
// 🎯 **目的**：
//   - 管理电路的版本信息
//   - 提供电路优化建议
//   - 统计电路约束数量
//
// 📋 **设计原则**：
//   - 版本管理：支持电路版本追踪和比较
//   - 优化建议：提供电路优化建议
//   - 约束统计：统计电路约束数量用于性能分析
//
// ============================================================================

// CircuitVersionInfo 电路版本信息
type CircuitVersionInfo struct {
	CircuitID         string    // 电路ID
	Version           uint32    // 版本号
	CreatedAt         time.Time // 创建时间
	ConstraintCount   int       // 约束数量
	OptimizationLevel string    // 优化级别（basic, optimized, advanced）
	HashFunction      string    // 使用的哈希函数（sha256, poseidon等）
	Notes             string    // 版本说明
}

// CircuitOptimizationReport 电路优化报告
type CircuitOptimizationReport struct {
	CircuitID        string   // 电路ID
	Version          uint32   // 版本号
	ConstraintCount  int      // 当前约束数量
	Optimizations    []string // 优化建议列表
	EstimatedSavings int      // 预计可节省的约束数量
}

// CircuitVersionManager 电路版本管理器
type CircuitVersionManager struct {
	logger log.Logger

	// 版本信息存储
	versionInfo  map[string]*CircuitVersionInfo
	versionMutex sync.RWMutex

	// 优化报告存储
	optimizationReports map[string]*CircuitOptimizationReport
	reportMutex         sync.RWMutex
}

// NewCircuitVersionManager 创建电路版本管理器
func NewCircuitVersionManager(logger log.Logger) *CircuitVersionManager {
	return &CircuitVersionManager{
		logger:              logger,
		versionInfo:         make(map[string]*CircuitVersionInfo),
		optimizationReports: make(map[string]*CircuitOptimizationReport),
	}
}

// RegisterCircuitVersion 注册电路版本信息
func (cvm *CircuitVersionManager) RegisterCircuitVersion(info *CircuitVersionInfo) {
	if info == nil {
		return
	}

	versionKey := fmt.Sprintf("%s.v%d", info.CircuitID, info.Version)

	cvm.versionMutex.Lock()
	cvm.versionInfo[versionKey] = info
	cvm.versionMutex.Unlock()

	if cvm.logger != nil {
		cvm.logger.Debugf("注册电路版本: %s, 约束数量=%d", versionKey, info.ConstraintCount)
	}
}

// GetCircuitVersionInfo 获取电路版本信息
func (cvm *CircuitVersionManager) GetCircuitVersionInfo(circuitID string, version uint32) (*CircuitVersionInfo, bool) {
	versionKey := fmt.Sprintf("%s.v%d", circuitID, version)

	cvm.versionMutex.RLock()
	defer cvm.versionMutex.RUnlock()

	info, exists := cvm.versionInfo[versionKey]
	return info, exists
}

// ListCircuitVersions 列出所有电路版本
func (cvm *CircuitVersionManager) ListCircuitVersions(circuitID string) []*CircuitVersionInfo {
	cvm.versionMutex.RLock()
	defer cvm.versionMutex.RUnlock()

	var versions []*CircuitVersionInfo
	for key, info := range cvm.versionInfo {
		if len(key) >= len(circuitID) && key[:len(circuitID)] == circuitID {
			versions = append(versions, info)
		}
	}

	return versions
}

// AnalyzeCircuitConstraints 分析电路约束数量
//
// 📋 **参数**：
//   - circuit: 电路实例
//
// 🔧 **返回值**：
//   - constraintCount: 约束数量
//   - error: 分析过程中的错误
//
// ⚠️ **注意**：
//   - 使用BN254曲线作为默认曲线进行分析
//   - 实际约束数量可能因曲线而异
//   - 如果电路类型不支持，将返回错误
func (cvm *CircuitVersionManager) AnalyzeCircuitConstraints(circuit frontend.Circuit) (int, error) {
	if circuit == nil {
		return 0, fmt.Errorf("电路不能为nil")
	}

	// 编译电路以获取约束数量
	// 使用BN254曲线作为默认曲线进行分析（实际应该从配置获取）
	compiledCircuit, err := frontend.Compile(
		ecc.BN254.ScalarField(), // 使用BN254的标量域
		r1cs.NewBuilder,         // 使用R1CS构建器（Groth16）
		circuit,
	)
	if err != nil {
		return 0, fmt.Errorf("编译电路失败: %w", err)
	}

	// 获取约束数量
	// frontend.Compile 使用 r1cs.NewBuilder 时，返回的类型已经是 constraint.ConstraintSystem
	return compiledCircuit.GetNbConstraints(), nil
}

// GenerateOptimizationReport 生成电路优化报告
//
// 📋 **参数**：
//   - circuitID: 电路ID
//   - version: 版本号
//   - constraintCount: 约束数量
//
// 🔧 **返回值**：
//   - *CircuitOptimizationReport: 优化报告
func (cvm *CircuitVersionManager) GenerateOptimizationReport(circuitID string, version uint32, constraintCount int) *CircuitOptimizationReport {
	report := &CircuitOptimizationReport{
		CircuitID:        circuitID,
		Version:          version,
		ConstraintCount:  constraintCount,
		Optimizations:    []string{},
		EstimatedSavings: 0,
	}

	// 生成优化建议
	optimizations := cvm.generateOptimizationSuggestions(constraintCount)
	report.Optimizations = optimizations

	// 估算可节省的约束数量
	report.EstimatedSavings = cvm.estimateConstraintSavings(constraintCount, optimizations)

	// 存储报告
	reportKey := fmt.Sprintf("%s.v%d", circuitID, version)
	cvm.reportMutex.Lock()
	cvm.optimizationReports[reportKey] = report
	cvm.reportMutex.Unlock()

	return report
}

// generateOptimizationSuggestions 生成优化建议
func (cvm *CircuitVersionManager) generateOptimizationSuggestions(constraintCount int) []string {
	var suggestions []string

	// 基于约束数量的优化建议
	if constraintCount > 10000 {
		suggestions = append(suggestions, "考虑使用PlonK证明方案（适合大型电路）")
		suggestions = append(suggestions, "考虑使用Poseidon哈希替代SHA-256（可减少30-50%约束）")
		suggestions = append(suggestions, "考虑电路分解（将大电路拆分为多个小电路）")
	} else if constraintCount > 1000 {
		suggestions = append(suggestions, "考虑使用Poseidon哈希替代SHA-256（可减少20-40%约束）")
		suggestions = append(suggestions, "优化电路结构，减少不必要的约束")
	} else {
		suggestions = append(suggestions, "电路规模较小，Groth16方案适合")
		suggestions = append(suggestions, "可考虑使用Poseidon哈希优化（可选）")
	}

	// 通用优化建议
	suggestions = append(suggestions, "使用预计算值减少约束")
	suggestions = append(suggestions, "优化循环展开策略")
	suggestions = append(suggestions, "使用查找表（Lookup Table）优化复杂运算")

	return suggestions
}

// estimateConstraintSavings 估算可节省的约束数量
func (cvm *CircuitVersionManager) estimateConstraintSavings(constraintCount int, optimizations []string) int {
	savings := 0

	for _, opt := range optimizations {
		if cvm.containsOptimization(opt, "Poseidon") {
			// Poseidon哈希可节省20-50%的约束
			savings += int(float64(constraintCount) * 0.3) // 估算30%
		}
		if cvm.containsOptimization(opt, "预计算") {
			// 预计算可节省5-15%的约束
			savings += int(float64(constraintCount) * 0.1) // 估算10%
		}
		if cvm.containsOptimization(opt, "查找表") {
			// 查找表可节省10-30%的约束
			savings += int(float64(constraintCount) * 0.2) // 估算20%
		}
	}

	// 避免重复计算，取最大值
	if savings > constraintCount/2 {
		savings = constraintCount / 2
	}

	return savings
}

// GetOptimizationReport 获取优化报告
func (cvm *CircuitVersionManager) GetOptimizationReport(circuitID string, version uint32) (*CircuitOptimizationReport, bool) {
	reportKey := fmt.Sprintf("%s.v%d", circuitID, version)

	cvm.reportMutex.RLock()
	defer cvm.reportMutex.RUnlock()

	report, exists := cvm.optimizationReports[reportKey]
	return report, exists
}

// containsOptimization 检查优化建议中是否包含特定关键词
func (cvm *CircuitVersionManager) containsOptimization(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			cvm.containsInMiddle(s, substr))))
}

func (cvm *CircuitVersionManager) containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
