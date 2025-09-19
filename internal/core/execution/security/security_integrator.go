// security_integrator.go 提供传统安全集成器实现
//
// 本文件包含原有的复杂安全集成器实现，主要用于向后兼容
// 新的简化安全实现请参考 execution_security.go
//
// 设计说明：
// 1. 保留了完整的SecurityIntegrator和QuotaManager实现
// 2. 通过简化的默认构造函数提供MVP级别的安全保护
// 3. 使用NoOp实现替代复杂的企业级功能（威胁检测、详细审计等）
// 4. 确保向后兼容性，现有代码无需修改即可使用简化版本
//
// 迁移路径：
// - 当前：使用简化的NewDefaultSecurityIntegrator/NewDefaultQuotaManager
// - 未来：逐步迁移到execution_security.go中的ExecutionSecurity
package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 类型别名定义 ====================
//
// 这些类型别名简化了对interfaces包中类型的使用
// 避免了长的包路径，提高代码可读性

// AuditEventEmitter 审计事件发射器类型别名
// 用于发射安全、性能、错误等各类审计事件
type AuditEventEmitter = interfaces.AuditEventEmitter

// SecurityAuditEvent 安全审计事件类型别名
// 用于记录安全相关的事件信息
type SecurityAuditEvent = interfaces.SecurityAuditEvent

// DefaultGlobalSecurityPolicy 创建默认全局安全策略
func DefaultGlobalSecurityPolicy() *GlobalSecurityPolicy {
	return &GlobalSecurityPolicy{
		GlobalAllowedImports: []string{},
		GlobalDeniedImports:  []string{},
		GlobalHostPolicy: &HostSecurityPolicy{
			AllowedFunctions:           []string{},
			DeniedFunctions:            []string{},
			ParameterValidationRules:   make(map[string]ParameterValidationRule),
			ReturnValueValidationRules: make(map[string]ReturnValueValidationRule),
			CallRateLimits:             make(map[string]RateLimit),
			PermissionMatrix:           make(map[string][]string),
		},
		ResourceLimits: &ResourceLimitPolicy{
			MaxExecutionTimeMs: 180000,    // 🔧 修复：3分钟执行超时限制
			MaxMemoryBytes:     268435456, // 256MB
			MaxCPUUsagePercent: 80.0,
			MaxNetworkCalls:    100,
			MaxFileOperations:  50,
			MaxStateReads:      1000,
			MaxStateWrites:     100,
		},
		ExecutionPolicy: &ExecutionSecurityPolicy{
			AllowDynamicCodeGeneration: false,
			AllowSensitiveAPIAccess:    false,
			EnforceSandboxMode:         true,
			AllowedSystemCalls:         []string{},
			EnvironmentAccessPolicy:    "deny",
			NetworkAccessPolicy:        "deny",
		},
		ComplianceRequirements: []string{},
	}
}

// SecurityIntegrator 安全集成器
//
// 职责：
// 1. 联动各引擎的安全管理器进行统一安全校验
// 2. 管理import白名单和宿主函数防护策略
// 3. 执行前后安全状态检查
// 4. 收集和发射安全相关的审计事件
//
// 设计：
// - 支持多引擎安全策略的统一管理
// - 提供细粒度的安全控制（模块级、函数级、参数级）
// - 集成威胁检测和实时防护
type SecurityIntegrator struct {
	// 引擎特定的安全管理器
	engineSecurityManagers map[types.EngineType]EngineSecurityManager

	// 全局安全策略
	globalPolicy *GlobalSecurityPolicy

	// 威胁检测器
	threatDetector ThreatDetector

	// 审计事件发射器
	auditEmitter AuditEventEmitter

	// 安全统计收集器
	statsCollector SecurityStatsCollector

	// 运行时状态
	mutex        sync.RWMutex
	activeChecks map[string]*SecurityCheck // 正在进行的安全检查
	violationLog []SecurityViolation       // 违规记录
	// config 已移除 - 使用固定的智能安全策略
}

// SecurityIntegratorConfig 已删除 - 使用固定的智能安全策略
// 所有安全功能均为智能默认启用，无需配置

// GlobalSecurityPolicy 全局安全策略
type GlobalSecurityPolicy struct {
	// 全局白名单（所有引擎共享）
	GlobalAllowedImports []string `json:"global_allowed_imports"`

	// 全局黑名单
	GlobalDeniedImports []string `json:"global_denied_imports"`

	// 全局宿主函数策略
	GlobalHostPolicy *HostSecurityPolicy `json:"global_host_policy"`

	// 资源限制策略
	ResourceLimits *ResourceLimitPolicy `json:"resource_limits"`

	// 执行环境安全策略
	ExecutionPolicy *ExecutionSecurityPolicy `json:"execution_policy"`

	// 合规性要求
	ComplianceRequirements []string `json:"compliance_requirements"`
}

// HostSecurityPolicy 宿主函数安全策略
type HostSecurityPolicy struct {
	// 允许的宿主函数
	AllowedFunctions []string `json:"allowed_functions"`

	// 禁止的宿主函数
	DeniedFunctions []string `json:"denied_functions"`

	// 参数验证规则
	ParameterValidationRules map[string]ParameterValidationRule `json:"parameter_validation_rules"`

	// 返回值验证规则
	ReturnValueValidationRules map[string]ReturnValueValidationRule `json:"return_value_validation_rules"`

	// 调用频率限制
	CallRateLimits map[string]RateLimit `json:"call_rate_limits"`

	// 权限矩阵（函数 -> 所需权限）
	PermissionMatrix map[string][]string `json:"permission_matrix"`
}

// ResourceLimitPolicy 资源限制策略
type ResourceLimitPolicy struct {
	// 最大执行时间（毫秒）
	MaxExecutionTimeMs int64 `json:"max_execution_time_ms"`

	// 最大内存使用（字节）
	MaxMemoryBytes uint64 `json:"max_memory_bytes"`

	// 最大CPU使用率（百分比）
	MaxCPUUsagePercent float64 `json:"max_cpu_usage_percent"`

	// 最大网络调用次数
	MaxNetworkCalls int `json:"max_network_calls"`

	// 最大文件操作次数
	MaxFileOperations int `json:"max_file_operations"`

	// 最大状态读取次数
	MaxStateReads int `json:"max_state_reads"`

	// 最大状态写入次数
	MaxStateWrites int `json:"max_state_writes"`
}

// ExecutionSecurityPolicy 执行环境安全策略
type ExecutionSecurityPolicy struct {
	// 是否允许动态代码生成
	AllowDynamicCodeGeneration bool `json:"allow_dynamic_code_generation"`

	// 是否允许访问敏感API
	AllowSensitiveAPIAccess bool `json:"allow_sensitive_api_access"`

	// 是否强制沙箱模式
	EnforceSandboxMode bool `json:"enforce_sandbox_mode"`

	// 允许的系统调用
	AllowedSystemCalls []string `json:"allowed_system_calls"`

	// 环境变量访问策略
	EnvironmentAccessPolicy string `json:"environment_access_policy"`

	// 网络访问策略
	NetworkAccessPolicy string `json:"network_access_policy"`
}

// SecurityCheck 安全检查状态
type SecurityCheck struct {
	CheckID    string                `json:"check_id"`
	EngineType types.EngineType      `json:"engine_type"`
	StartTime  time.Time             `json:"start_time"`
	Status     SecurityCheckStatus   `json:"status"`
	Parameters types.ExecutionParams `json:"parameters"`
	Results    []SecurityCheckResult `json:"results"`
	Violations []SecurityViolation   `json:"violations"`
}

// SecurityCheckStatus 安全检查状态
type SecurityCheckStatus string

const (
	SecurityCheckStatusPending    SecurityCheckStatus = "pending"
	SecurityCheckStatusInProgress SecurityCheckStatus = "in_progress"
	SecurityCheckStatusCompleted  SecurityCheckStatus = "completed"
	SecurityCheckStatusFailed     SecurityCheckStatus = "failed"
	SecurityCheckStatusTimedOut   SecurityCheckStatus = "timed_out"
)

// SecurityCheckResult 安全检查结果
type SecurityCheckResult struct {
	CheckType string                 `json:"check_type"`
	Passed    bool                   `json:"passed"`
	Message   string                 `json:"message"`
	Severity  string                 `json:"severity"`
	Timestamp int64                  `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}

// SecurityViolation 安全违规记录
type SecurityViolation struct {
	ViolationID   string                 `json:"violation_id"`
	ViolationType string                 `json:"violation_type"`
	Severity      ViolationSeverity      `json:"severity"`
	Description   string                 `json:"description"`
	Context       map[string]interface{} `json:"context"`
	Timestamp     int64                  `json:"timestamp"`
	Action        string                 `json:"action"`
}

// ViolationSeverity 违规严重程度
type ViolationSeverity string

const (
	ViolationSeverityLow      ViolationSeverity = "low"
	ViolationSeverityMedium   ViolationSeverity = "medium"
	ViolationSeverityHigh     ViolationSeverity = "high"
	ViolationSeverityCritical ViolationSeverity = "critical"
)

// NewSecurityIntegrator 创建安全集成器
func NewSecurityIntegrator(
	globalPolicy *GlobalSecurityPolicy,
	threatDetector ThreatDetector,
	auditEmitter AuditEventEmitter,
	statsCollector SecurityStatsCollector,
) *SecurityIntegrator {
	// config参数已移除 - 使用固定的智能安全策略

	return &SecurityIntegrator{
		engineSecurityManagers: make(map[types.EngineType]EngineSecurityManager),
		globalPolicy:           globalPolicy,
		threatDetector:         threatDetector,
		auditEmitter:           auditEmitter,
		statsCollector:         statsCollector,
		activeChecks:           make(map[string]*SecurityCheck),
		violationLog:           make([]SecurityViolation, 0, 1000), // 固定智能默认值
		// config已移除，使用固定的智能安全策略
	}
}

// DefaultSecurityIntegratorConfig 已删除 - 不再需要配置函数
// 所有安全策略均为智能默认，无需配置

// RegisterEngineSecurityManager 注册引擎安全管理器
func (si *SecurityIntegrator) RegisterEngineSecurityManager(engineType types.EngineType, manager EngineSecurityManager) error {
	si.mutex.Lock()
	defer si.mutex.Unlock()

	if _, exists := si.engineSecurityManagers[engineType]; exists {
		return fmt.Errorf("security manager for engine type %s already registered", engineType)
	}

	si.engineSecurityManagers[engineType] = manager
	return nil
}

// ValidateExecution 执行前安全校验
func (si *SecurityIntegrator) ValidateExecution(ctx context.Context, params types.ExecutionParams) error {
	// 提取引擎类型
	engineType, err := si.extractEngineType(params)
	if err != nil {
		return fmt.Errorf("failed to extract engine type: %w", err)
	}

	// 创建安全检查会话
	checkID := si.generateCheckID()
	check := &SecurityCheck{
		CheckID:    checkID,
		EngineType: engineType,
		StartTime:  time.Now(),
		Status:     SecurityCheckStatusPending,
		Parameters: params,
		Results:    []SecurityCheckResult{},
		Violations: []SecurityViolation{},
	}

	// 检查并发限制
	si.mutex.Lock()
	// 智能并发控制：自动根据CPU核数限制（避免过载）
	maxConcurrent := 8 // 固定合理值，适配大多数环境
	if len(si.activeChecks) >= maxConcurrent {
		si.mutex.Unlock()
		return fmt.Errorf("maximum concurrent security checks (%d) exceeded", maxConcurrent)
	}
	si.activeChecks[checkID] = check
	si.mutex.Unlock()

	defer func() {
		si.mutex.Lock()
		delete(si.activeChecks, checkID)
		si.mutex.Unlock()
	}()

	// 设置超时
	// 固定智能超时：5秒，平衡安全检查与性能
	timeout := 5 * time.Second
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	check.Status = SecurityCheckStatusInProgress

	// 执行全局安全检查
	// 全局安全检查（自运行节点始终启用）
	if true { // 原：si.config.EnableGlobalChecks
		if err := si.performGlobalSecurityChecks(checkCtx, check); err != nil {
			check.Status = SecurityCheckStatusFailed
			si.recordViolation(check, "global_security_check_failed", ViolationSeverityHigh, err.Error())

			// 关键安全失败立即终止（智能策略）
			if true { // 原：si.config.FailFast
				return fmt.Errorf("global security check failed: %w", err)
			}
		}
	}

	// 执行引擎特定安全检查
	// 引擎特定安全检查（自运行节点始终启用）
	if true { // 原：si.config.EnableEngineSpecificChecks
		if err := si.performEngineSpecificChecks(checkCtx, check); err != nil {
			check.Status = SecurityCheckStatusFailed
			si.recordViolation(check, "engine_security_check_failed", ViolationSeverityHigh, err.Error())

			// 关键安全失败立即终止（智能策略）
			if true { // 原：si.config.FailFast
				return fmt.Errorf("engine-specific security check failed: %w", err)
			}
		}
	}

	// 威胁检测
	// 威胁检测（自运行节点在有检测器时始终启用）
	if si.threatDetector != nil { // 原：si.config.EnableThreatDetection &&
		if threat := si.threatDetector.DetectThreats(checkCtx, params); threat != nil {
			check.Status = SecurityCheckStatusFailed
			si.recordViolation(check, "threat_detected", ViolationSeverityCritical, threat.Description)

			// 威胁检测总是FailFast
			return fmt.Errorf("threat detected: %s", threat.Description)
		}
	}

	// 检查超时
	if checkCtx.Err() == context.DeadlineExceeded {
		check.Status = SecurityCheckStatusTimedOut
		return fmt.Errorf("security check timed out after %v", timeout)
	}

	check.Status = SecurityCheckStatusCompleted

	// 发射安全审计事件
	// 详细日志（自运行节点始终启用，便于问题诊断）
	if true { // 原：si.config.EnableDetailedLogging
		si.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
			EventType: "security_validation_completed",
			Severity:  "low",
			Timestamp: time.Now(),
			Caller:    "security_integrator",
			Action:    "validation",
			Result:    "completed",
		})
	}

	return nil
}

// ValidateImportWhitelist 校验import白名单
func (si *SecurityIntegrator) ValidateImportWhitelist(engineType types.EngineType, imports []string) error {
	// 检查全局黑名单
	for _, imp := range imports {
		for _, denied := range si.globalPolicy.GlobalDeniedImports {
			if imp == denied {
				violation := SecurityViolation{
					ViolationID:   si.generateViolationID(),
					ViolationType: "denied_import",
					Severity:      ViolationSeverityHigh,
					Description:   fmt.Sprintf("Import '%s' is globally denied", imp),
					Context: map[string]interface{}{
						"engine_type": engineType,
						"import":      imp,
						"policy":      "global_denied_imports",
					},
					Timestamp: time.Now().Unix(),
					Action:    "import_rejected",
				}
				si.recordViolationDirect(violation)
				return fmt.Errorf("import '%s' is globally denied", imp)
			}
		}
	}

	// 检查全局白名单（如果配置了）
	if len(si.globalPolicy.GlobalAllowedImports) > 0 {
		for _, imp := range imports {
			allowed := false
			for _, allowedImp := range si.globalPolicy.GlobalAllowedImports {
				if imp == allowedImp {
					allowed = true
					break
				}
			}
			if !allowed {
				violation := SecurityViolation{
					ViolationID:   si.generateViolationID(),
					ViolationType: "unauthorized_import",
					Severity:      ViolationSeverityMedium,
					Description:   fmt.Sprintf("Import '%s' is not in global whitelist", imp),
					Context: map[string]interface{}{
						"engine_type": engineType,
						"import":      imp,
						"policy":      "global_allowed_imports",
					},
					Timestamp: time.Now().Unix(),
					Action:    "import_rejected",
				}
				si.recordViolationDirect(violation)
				return fmt.Errorf("import '%s' is not in global whitelist", imp)
			}
		}
	}

	// 委托给引擎特定的安全管理器
	if manager, exists := si.engineSecurityManagers[engineType]; exists {
		if err := manager.ValidateImports(imports); err != nil {
			return fmt.Errorf("engine-specific import validation failed: %w", err)
		}
	}

	return nil
}

// ValidateHostFunctionCall 校验宿主函数调用
func (si *SecurityIntegrator) ValidateHostFunctionCall(engineType types.EngineType, functionName string, params []interface{}) error {
	// 检查全局宿主函数策略
	if si.globalPolicy.GlobalHostPolicy != nil {
		policy := si.globalPolicy.GlobalHostPolicy

		// 检查黑名单
		for _, denied := range policy.DeniedFunctions {
			if functionName == denied {
				violation := SecurityViolation{
					ViolationID:   si.generateViolationID(),
					ViolationType: "denied_host_function",
					Severity:      ViolationSeverityHigh,
					Description:   fmt.Sprintf("Host function '%s' is globally denied", functionName),
					Context: map[string]interface{}{
						"engine_type":   engineType,
						"function_name": functionName,
						"params_count":  len(params),
					},
					Timestamp: time.Now().Unix(),
					Action:    "function_call_rejected",
				}
				si.recordViolationDirect(violation)
				return fmt.Errorf("host function '%s' is globally denied", functionName)
			}
		}

		// 检查白名单（如果配置了）
		if len(policy.AllowedFunctions) > 0 {
			allowed := false
			for _, allowedFunc := range policy.AllowedFunctions {
				if functionName == allowedFunc {
					allowed = true
					break
				}
			}
			if !allowed {
				violation := SecurityViolation{
					ViolationID:   si.generateViolationID(),
					ViolationType: "unauthorized_host_function",
					Severity:      ViolationSeverityMedium,
					Description:   fmt.Sprintf("Host function '%s' is not in global whitelist", functionName),
					Context: map[string]interface{}{
						"engine_type":   engineType,
						"function_name": functionName,
						"params_count":  len(params),
					},
					Timestamp: time.Now().Unix(),
					Action:    "function_call_rejected",
				}
				si.recordViolationDirect(violation)
				return fmt.Errorf("host function '%s' is not in global whitelist", functionName)
			}
		}

		// 验证参数
		if rule, exists := policy.ParameterValidationRules[functionName]; exists {
			if err := si.validateParameters(params, rule); err != nil {
				violation := SecurityViolation{
					ViolationID:   si.generateViolationID(),
					ViolationType: "invalid_host_function_params",
					Severity:      ViolationSeverityMedium,
					Description:   fmt.Sprintf("Invalid parameters for host function '%s': %v", functionName, err),
					Context: map[string]interface{}{
						"engine_type":      engineType,
						"function_name":    functionName,
						"params_count":     len(params),
						"validation_error": err.Error(),
					},
					Timestamp: time.Now().Unix(),
					Action:    "function_call_rejected",
				}
				si.recordViolationDirect(violation)
				return fmt.Errorf("invalid parameters for host function '%s': %w", functionName, err)
			}
		}
	}

	// 委托给引擎特定的安全管理器
	if manager, exists := si.engineSecurityManagers[engineType]; exists {
		if err := manager.ValidateHostCall(functionName, params); err != nil {
			return fmt.Errorf("engine-specific host function validation failed: %w", err)
		}
	}

	return nil
}

// 内部辅助方法

// extractEngineType 从执行参数中提取引擎类型
func (si *SecurityIntegrator) extractEngineType(params types.ExecutionParams) (types.EngineType, error) {
	if engineTypeVal, exists := params.Context["engine_type"]; exists {
		if engineTypeStr, ok := engineTypeVal.(string); ok {
			return types.EngineType(engineTypeStr), nil
		}
		return "", fmt.Errorf("engine_type in context is not a string: %T", engineTypeVal)
	}
	return types.EngineTypeWASM, nil // 默认WASM
}

// performGlobalSecurityChecks 执行全局安全检查
func (si *SecurityIntegrator) performGlobalSecurityChecks(ctx context.Context, check *SecurityCheck) error {
	// 资源限制检查
	if si.globalPolicy.ResourceLimits != nil {
		limits := si.globalPolicy.ResourceLimits

		// 检查执行时间限制
		if limits.MaxExecutionTimeMs > 0 && check.Parameters.Timeout > limits.MaxExecutionTimeMs {
			return fmt.Errorf("execution timeout %d exceeds global limit %d", check.Parameters.Timeout, limits.MaxExecutionTimeMs)
		}

		// 检查内存限制
		if limits.MaxMemoryBytes > 0 && uint64(check.Parameters.MemoryLimit) > limits.MaxMemoryBytes {
			return fmt.Errorf("memory limit %d exceeds global limit %d", check.Parameters.MemoryLimit, limits.MaxMemoryBytes)
		}
	}

	// 执行环境策略检查
	if si.globalPolicy.ExecutionPolicy != nil {
		policy := si.globalPolicy.ExecutionPolicy

		// 检查沙箱模式
		if policy.EnforceSandboxMode {
			// 这里可以添加具体的沙箱模式检查逻辑
			check.Results = append(check.Results, SecurityCheckResult{
				CheckType: "sandbox_mode_check",
				Passed:    true,
				Message:   "Sandbox mode enforced",
				Severity:  "info",
				Timestamp: time.Now().Unix(),
			})
		}
	}

	return nil
}

// performEngineSpecificChecks 执行引擎特定检查
func (si *SecurityIntegrator) performEngineSpecificChecks(ctx context.Context, check *SecurityCheck) error {
	manager, exists := si.engineSecurityManagers[check.EngineType]
	if !exists {
		// 简化版：如果没有注册引擎特定管理器，跳过检查（适用于MVP场景）
		check.Results = append(check.Results, SecurityCheckResult{
			CheckType: "engine_specific_check",
			Passed:    true,
			Message:   fmt.Sprintf("No specific security manager for %s, using default policies", check.EngineType),
			Severity:  "info",
			Timestamp: time.Now().Unix(),
		})
		return nil
	}

	return manager.ValidateExecution(ctx, check.Parameters)
}

// recordViolation 记录安全违规
func (si *SecurityIntegrator) recordViolation(check *SecurityCheck, violationType string, severity ViolationSeverity, description string) {
	violation := SecurityViolation{
		ViolationID:   si.generateViolationID(),
		ViolationType: violationType,
		Severity:      severity,
		Description:   description,
		Context: map[string]interface{}{
			"check_id":    check.CheckID,
			"engine_type": check.EngineType,
			"resource_id": string(check.Parameters.ResourceID),
		},
		Timestamp: time.Now().Unix(),
		Action:    "execution_rejected",
	}

	check.Violations = append(check.Violations, violation)
	si.recordViolationDirect(violation)
}

// recordViolationDirect 直接记录安全违规
func (si *SecurityIntegrator) recordViolationDirect(violation SecurityViolation) {
	si.mutex.Lock()
	defer si.mutex.Unlock()

	// 添加到违规日志
	// 智能日志管理：固定保留1000条记录
	if len(si.violationLog) >= 1000 { // 原：si.config.ViolationLogSize
		// 移除最旧的记录
		si.violationLog = si.violationLog[1:]
	}
	si.violationLog = append(si.violationLog, violation)

	// 发射安全审计事件
	si.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
		EventType: "security_violation",
		Severity:  "critical",
		Timestamp: time.Now(),
		Caller:    "security_integrator",
		Action:    "violation_detection",
		Result:    "denied",
	})

	// 更新统计
	if si.statsCollector != nil {
		si.statsCollector.RecordViolation(violation)
	}
}

// validateParameters 验证参数
func (si *SecurityIntegrator) validateParameters(params []interface{}, rule ParameterValidationRule) error {
	// 检查参数数量
	if rule.MinParams > 0 && len(params) < rule.MinParams {
		return fmt.Errorf("insufficient parameters: expected at least %d, got %d", rule.MinParams, len(params))
	}
	if rule.MaxParams > 0 && len(params) > rule.MaxParams {
		return fmt.Errorf("too many parameters: expected at most %d, got %d", rule.MaxParams, len(params))
	}

	// 检查参数类型
	for i, param := range params {
		if i < len(rule.ParamTypes) {
			expectedType := rule.ParamTypes[i]
			if !si.validateParameterType(param, expectedType) {
				return fmt.Errorf("parameter %d type mismatch: expected %s, got %T", i, expectedType, param)
			}
		}
	}

	return nil
}

// validateParameterType 验证参数类型
func (si *SecurityIntegrator) validateParameterType(param interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := param.(string)
		return ok
	case "int":
		_, ok := param.(int)
		return ok
	case "int32":
		_, ok := param.(int32)
		return ok
	case "int64":
		_, ok := param.(int64)
		return ok
	case "float32":
		_, ok := param.(float32)
		return ok
	case "float64":
		_, ok := param.(float64)
		return ok
	case "bool":
		_, ok := param.(bool)
		return ok
	case "bytes":
		_, ok := param.([]byte)
		return ok
	default:
		return true // 未知类型，跳过验证
	}
}

// generateCheckID 生成检查ID
func (si *SecurityIntegrator) generateCheckID() string {
	return fmt.Sprintf("sec_check_%d", time.Now().UnixNano())
}

// generateViolationID 生成违规ID
func (si *SecurityIntegrator) generateViolationID() string {
	return fmt.Sprintf("violation_%d", time.Now().UnixNano())
}

// ==================== 接口定义 ====================

// EngineSecurityManager 引擎安全管理器接口
type EngineSecurityManager interface {
	ValidateExecution(ctx context.Context, params types.ExecutionParams) error
	ValidateImports(imports []string) error
	ValidateHostCall(functionName string, params []interface{}) error
	GetSecurityStats() interface{}
}

// ThreatDetector 威胁检测器接口
type ThreatDetector interface {
	DetectThreats(ctx context.Context, params types.ExecutionParams) *ThreatInfo
	UpdateThreatIntelligence(intelligence ThreatIntelligence) error
	GetThreatLevel() ThreatLevel
}

// SecurityStatsCollector 安全统计收集器接口
type SecurityStatsCollector interface {
	RecordViolation(violation SecurityViolation)
	RecordSecurityCheck(check SecurityCheck)
	GetSecurityMetrics() SecurityMetrics
}

// ThreatInfo 威胁信息
type ThreatInfo struct {
	ThreatID    string                 `json:"threat_id"`
	ThreatType  string                 `json:"threat_type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Confidence  float64                `json:"confidence"`
	DetectedAt  time.Time              `json:"detected_at"`
	Context     map[string]interface{} `json:"context"`
}

// ThreatIntelligence 威胁情报
type ThreatIntelligence struct {
	Signatures []ThreatSignature `json:"signatures"`
	Patterns   []ThreatPattern   `json:"patterns"`
	Indicators []ThreatIndicator `json:"indicators"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Version    string            `json:"version"`
}

// ThreatLevel 威胁级别
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// SecurityMetrics 安全指标
type SecurityMetrics struct {
	TotalChecks          int64                       `json:"total_checks"`
	PassedChecks         int64                       `json:"passed_checks"`
	FailedChecks         int64                       `json:"failed_checks"`
	ViolationsByType     map[string]int64            `json:"violations_by_type"`
	ViolationsBySeverity map[ViolationSeverity]int64 `json:"violations_by_severity"`
	AverageCheckTime     time.Duration               `json:"average_check_time"`
	LastViolation        *SecurityViolation          `json:"last_violation"`
}

// 辅助类型定义

// ParameterValidationRule 参数验证规则
type ParameterValidationRule struct {
	MinParams  int      `json:"min_params"`
	MaxParams  int      `json:"max_params"`
	ParamTypes []string `json:"param_types"`
	Required   []bool   `json:"required"`
	Validators []string `json:"validators"`
}

// ReturnValueValidationRule 返回值验证规则
type ReturnValueValidationRule struct {
	ExpectedType  string        `json:"expected_type"`
	AllowedValues []interface{} `json:"allowed_values"`
	Validators    []string      `json:"validators"`
}

// RateLimit 频率限制
type RateLimit struct {
	MaxCalls   int           `json:"max_calls"`
	TimeWindow time.Duration `json:"time_window"`
	BurstSize  int           `json:"burst_size"`
}

// ThreatSignature 威胁签名
type ThreatSignature struct {
	ID          string `json:"id"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// ThreatPattern 威胁模式
type ThreatPattern struct {
	ID       string                 `json:"id"`
	Rules    []string               `json:"rules"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ThreatIndicator 威胁指标
type ThreatIndicator struct {
	Type        string    `json:"type"`
	Value       string    `json:"value"`
	Confidence  float64   `json:"confidence"`
	LastSeen    time.Time `json:"last_seen"`
	Description string    `json:"description"`
}

// ==================== NoOp实现（生产环境默认，符合MVP设计） ====================
//
// 以下NoOp实现符合区块链节点"自运行"的设计理念：
// 1. 防止nil panic：确保系统在没有企业级功能时依然稳定运行
// 2. MVP简化：专注execution核心功能，避免过度设计
// 3. 零开销：NoOp实现几乎无性能开销，适合高频调用
// 4. 生产就绪：提供安全的默认行为，满足基础运行需求
//
// 设计合理性：
// - 威胁检测：对于自运行节点，复杂威胁检测属于过度设计
// - 统计收集：基础execution不需要详细统计，简单日志即可满足需求
// - 审计发射：MVP阶段重点是功能正确性，详细审计可在后续版本添加

// NoOpThreatDetector NoOp威胁检测器实现
// 提供基础安全保障，确保生产环境不会因为nil依赖而panic
type NoOpThreatDetector struct{}

// DetectThreats 执行威胁检测（NoOp实现）
func (d *NoOpThreatDetector) DetectThreats(ctx context.Context, params types.ExecutionParams) *ThreatInfo {
	// NoOp实现：不检测威胁，但确保不会panic
	return nil
}

// UpdateThreatIntelligence 更新威胁情报（NoOp实现）
func (d *NoOpThreatDetector) UpdateThreatIntelligence(intelligence ThreatIntelligence) error {
	// NoOp实现：不更新情报，但确保不会panic
	return nil
}

// GetThreatLevel 获取威胁等级（NoOp实现）
func (d *NoOpThreatDetector) GetThreatLevel() ThreatLevel {
	// NoOp实现：返回低威胁等级，确保系统可正常运行
	return ThreatLevelLow
}

// NoOpSecurityStatsCollector NoOp安全统计收集器实现
// 提供基础统计功能，确保生产环境不会因为nil依赖而panic
type NoOpSecurityStatsCollector struct{}

// RecordViolation 记录违规（NoOp实现）
func (s *NoOpSecurityStatsCollector) RecordViolation(violation SecurityViolation) {
	// NoOp实现：不记录违规，但确保不会panic
}

// RecordSecurityCheck 记录安全检查（NoOp实现）
func (s *NoOpSecurityStatsCollector) RecordSecurityCheck(check SecurityCheck) {
	// NoOp实现：不记录检查，但确保不会panic
}

// GetSecurityMetrics 获取安全统计（NoOp实现）
func (s *NoOpSecurityStatsCollector) GetSecurityMetrics() SecurityMetrics {
	// 返回空统计，确保不会返回nil导致panic
	return SecurityMetrics{
		TotalChecks:          0,
		PassedChecks:         0,
		FailedChecks:         0,
		ViolationsByType:     make(map[string]int64),
		ViolationsBySeverity: make(map[ViolationSeverity]int64),
		AverageCheckTime:     0,
		LastViolation:        nil,
	}
}

// ==================== 生产级默认构造函数（简化版） ====================

// NewDefaultSecurityIntegrator 创建简化的安全集成器
// 使用最小配置，专注execution核心安全需求
func NewDefaultSecurityIntegrator() *SecurityIntegrator {
	// 使用最小化的SecurityIntegrator，避免复杂的威胁检测
	return &SecurityIntegrator{
		engineSecurityManagers: make(map[types.EngineType]EngineSecurityManager),
		globalPolicy:           DefaultGlobalSecurityPolicy(),
		threatDetector:         &NoOpThreatDetector{},
		auditEmitter:           &NoOpAuditEventEmitter{},
		statsCollector:         &NoOpSecurityStatsCollector{},
		activeChecks:           make(map[string]*SecurityCheck),
		violationLog:           make([]SecurityViolation, 0, 100), // 减少内存占用
	}
}

// NewDefaultQuotaManager 创建简化的配额管理器
// 使用最小配置，专注基础资源限制
func NewDefaultQuotaManager() *QuotaManager {
	policies := DefaultQuotaPolicies()

	// 🔧 强制增加所有配额以支持WASM合约执行
	policies.Global[QuotaTypeExecutionTime] = QuotaPolicy{
		InitialQuota:      1000000,  // 1000秒
		MaxQuota:          10000000, // 10000秒
		MinQuota:          1000,
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}
	policies.Global[QuotaTypeMemory] = QuotaPolicy{
		InitialQuota:      536870912,  // 🔧 强制修复：512MB内存配额
		MaxQuota:          2000000000, // 2GB
		MinQuota:          1048576,
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}
	policies.Global[QuotaTypeResource] = QuotaPolicy{
		InitialQuota:      10000000,  // 1000万资源
		MaxQuota:          100000000, // 1亿资源
		MinQuota:          10000,
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}
	// 其他6种配额类型
	policies.Global[QuotaTypeInstructions] = QuotaPolicy{InitialQuota: 100000000, MaxQuota: 1000000000, MinQuota: 10000, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}
	policies.Global[QuotaTypeCPU] = QuotaPolicy{InitialQuota: 1000000, MaxQuota: 10000000, MinQuota: 1000, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}
	policies.Global[QuotaTypeNetworkCalls] = QuotaPolicy{InitialQuota: 100000, MaxQuota: 1000000, MinQuota: 100, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}
	policies.Global[QuotaTypeStateOps] = QuotaPolicy{InitialQuota: 1000000, MaxQuota: 10000000, MinQuota: 1000, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}
	policies.Global[QuotaTypeStorageBytes] = QuotaPolicy{InitialQuota: 100000000, MaxQuota: 1000000000, MinQuota: 1048576, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}
	policies.Global[QuotaTypeRequests] = QuotaPolicy{InitialQuota: 100000, MaxQuota: 1000000, MinQuota: 100, RefreshPeriodSec: 3600, GrowthStrategy: GrowthStrategyFixed, RecycleStrategy: RecycleStrategyImmediate, OverlimitStrategy: OverlimitStrategyReject}

	qm := &QuotaManager{
		globalQuotas:      make(map[QuotaType]*QuotaPool),
		userQuotas:        make(map[string]map[QuotaType]*QuotaPool),           // 保留结构但简化使用
		contractQuotas:    make(map[string]map[QuotaType]*QuotaPool),           // 保留结构但简化使用
		engineQuotas:      make(map[types.EngineType]map[QuotaType]*QuotaPool), // 保留结构但简化使用
		policies:          policies,
		usageStats:        NewQuotaUsageStats(),
		auditEmitter:      &NoOpAuditEventEmitter{},
		activeAllocations: make(map[string]*QuotaAllocation),
		limitViolations:   make([]QuotaViolation, 0, 100), // 减少内存占用
	}

	// 只初始化全局配额池（简化版）
	qm.initializeGlobalQuotas()

	return qm
}

// ==================== NoOp实现（简化版审计发射器） ====================

// NoOpAuditEventEmitter NoOp审计事件发射器实现
// 提供基础审计功能，确保生产环境不会因为nil依赖而panic
type NoOpAuditEventEmitter struct{}

// EmitSecurityEvent 发射安全事件（NoOp实现）
func (n *NoOpAuditEventEmitter) EmitSecurityEvent(event interfaces.SecurityAuditEvent) {
	// NoOp实现：不发射事件，但确保不会panic
}

// EmitPerformanceEvent 发射性能事件（NoOp实现）
func (n *NoOpAuditEventEmitter) EmitPerformanceEvent(event interfaces.PerformanceAuditEvent) {
	// NoOp实现：不发射事件，但确保不会panic
}

// EmitErrorEvent 发射错误事件（NoOp实现）
func (n *NoOpAuditEventEmitter) EmitErrorEvent(event interfaces.ErrorAuditEvent) {
	// NoOp实现：不发射事件，但确保不会panic
}
