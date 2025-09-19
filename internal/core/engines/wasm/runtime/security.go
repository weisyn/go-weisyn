package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/engines/wasm/engine"
	types "github.com/weisyn/v1/pkg/types"
)

// CtxKey 上下文键类型
type CtxKey string

// KeyMemUsed 运行期内存使用上下文键（字节）
var KeyMemUsed CtxKey = "wasm_mem_used_bytes"

// SecurityManager WASM运行时安全管理器
// 负责模块验证、执行限制、威胁检测等安全功能
type SecurityManager struct {
	mutex           sync.RWMutex
	config          *SecurityConfig
	validationRules []ValidationRule
	limitCheckers   []LimitChecker
	threatDetector  *ThreatDetector
	securityEvents  chan *SecurityEvent
	stats           *SecurityStats
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableValidation      bool          `json:"enable_validation"`
	EnableThreatDetection bool          `json:"enable_threat_detection"`
	MaxExecutionTime      time.Duration `json:"max_execution_time"`
	MaxMemoryUsage        uint64        `json:"max_memory_usage"`
	MaxInstructionCount   uint64        `json:"max_instruction_count"`
	AllowedImports        []string      `json:"allowed_imports"`
	DeniedInstructions    []string      `json:"denied_instructions"`
	StrictMode            bool          `json:"strict_mode"`

	// 宿主调用防护
	AllowedHostFunctions []string                    `json:"allowed_host_functions"`
	DeniedHostFunctions  []string                    `json:"denied_host_functions"`
	HostParamSchemas     map[string][]ParamSchema    `json:"host_param_schemas"`
	EnforceIdempotency   bool                        `json:"enforce_idempotency"`
	IdempotentFunctions  []string                    `json:"idempotent_functions"`
	EnforceSideEffects   bool                        `json:"enforce_side_effects"`
	SideEffectPolicies   map[string]SideEffectPolicy `json:"side_effect_policies"`
}

// ValidationRule 验证规则
type ValidationRule struct {
	Name        string
	Description string
	Validator   func(module *engine.CompiledModule) error
	Enabled     bool
	Severity    int
}

// LimitChecker 限制检查器
type LimitChecker struct {
	Name          string
	Description   string
	Check         func(ctx context.Context) error
	PreExecution  bool // 执行前检查
	PostExecution bool // 执行后检查
}

// ThreatDetector 威胁检测器
type ThreatDetector struct {
	patterns       []ThreatPattern
	alertThreshold float64
	activeScans    map[string]*ScanSession
	mutex          sync.RWMutex
}

// ThreatPattern 威胁模式
type ThreatPattern struct {
	ID          string
	Name        string
	Description string
	Indicators  []string
	Severity    int
	Action      string
}

// ScanSession 扫描会话
type ScanSession struct {
	SessionID   string
	StartTime   time.Time
	ModuleHash  string
	ThreatScore float64
	Alerts      []string
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Severity  int                    `json:"severity"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// SecurityStats 安全统计
type SecurityStats struct {
	TotalValidations  uint64    `json:"total_validations"`
	FailedValidations uint64    `json:"failed_validations"`
	ThreatsDetected   uint64    `json:"threats_detected"`
	ThreatsBlocked    uint64    `json:"threats_blocked"`
	LimitViolations   uint64    `json:"limit_violations"`
	LastValidation    time.Time `json:"last_validation"`

	// 宿主调用统计
	HostCallsValidated uint64    `json:"host_calls_validated"`
	HostCallViolations uint64    `json:"host_call_violations"`
	LastHostViolation  time.Time `json:"last_host_violation"`
}

// ParamSchema 宿主参数Schema
type ParamSchema struct {
	Type     types.ValueType
	Required bool
	MaxSize  uint32 // 对于bytes/string
}

// SideEffectPolicy 副作用策略
type SideEffectPolicy struct {
	Allowed bool
	Notes   string
}

// ValidateHostCall 宿主调用防护：白名单/黑名单/参数Schema/幂等与副作用
func (sm *SecurityManager) ValidateHostCall(function string, args []interface{}, isWrite bool) error {
	sm.mutex.RLock()
	cfg := sm.config
	sm.mutex.RUnlock()

	// 白名单/黑名单
	if len(cfg.AllowedHostFunctions) > 0 && !contains(cfg.AllowedHostFunctions, function) {
		sm.stats.HostCallViolations++
		sm.stats.LastHostViolation = time.Now()
		sm.recordSecurityEvent("host_call_denied", 8, fmt.Sprintf("function %s not in allowlist", function))
		return fmt.Errorf("host function not allowed: %s", function)
	}
	if contains(cfg.DeniedHostFunctions, function) {
		sm.stats.HostCallViolations++
		sm.stats.LastHostViolation = time.Now()
		sm.recordSecurityEvent("host_call_blacklist", 9, fmt.Sprintf("function %s in denylist", function))
		return fmt.Errorf("host function denied: %s", function)
	}

	// 参数Schema校验
	if schema, ok := cfg.HostParamSchemas[function]; ok {
		if err := validateArgsBySchema(schema, args); err != nil {
			sm.stats.HostCallViolations++
			sm.stats.LastHostViolation = time.Now()
			sm.recordSecurityEvent("host_param_schema_violation", 7, fmt.Sprintf("%s: %v", function, err))
			return err
		}
	}

	// 幂等与副作用策略
	if isWrite {
		if cfg.EnforceIdempotency && !contains(cfg.IdempotentFunctions, function) {
			sm.stats.HostCallViolations++
			sm.stats.LastHostViolation = time.Now()
			sm.recordSecurityEvent("host_idempotency_violation", 8, fmt.Sprintf("%s not idempotent", function))
			return fmt.Errorf("non-idempotent host write: %s", function)
		}
		if cfg.EnforceSideEffects {
			if pol, ok := cfg.SideEffectPolicies[function]; ok && !pol.Allowed {
				sm.stats.HostCallViolations++
				sm.stats.LastHostViolation = time.Now()
				sm.recordSecurityEvent("host_side_effect_violation", 8, fmt.Sprintf("%s: %s", function, pol.Notes))
				return fmt.Errorf("host side-effect not allowed: %s", function)
			}
		}
	}

	sm.stats.HostCallsValidated++
	return nil
}

func validateArgsBySchema(schema []ParamSchema, args []interface{}) error {
	if len(args) < countRequired(schema) {
		return fmt.Errorf("missing required arguments")
	}
	for i := range schema {
		if i >= len(args) {
			if schema[i].Required {
				return fmt.Errorf("arg[%d] required", i)
			}
			break
		}
		if err := checkTypeSize(schema[i], args[i]); err != nil {
			return fmt.Errorf("arg[%d] invalid: %w", i, err)
		}
	}
	return nil
}

func checkTypeSize(s ParamSchema, v interface{}) error {
	switch s.Type {
	case types.ValueTypeI32:
		if _, ok := v.(int32); !ok {
			return fmt.Errorf("expect i32")
		}
	case types.ValueTypeI64:
		if _, ok := v.(int64); !ok {
			return fmt.Errorf("expect i64")
		}
	case types.ValueTypeF32:
		if _, ok := v.(float32); !ok {
			return fmt.Errorf("expect f32")
		}
	case types.ValueTypeF64:
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("expect f64")
		}
	default:
		switch x := v.(type) {
		case string:
			if s.MaxSize > 0 && uint32(len(x)) > s.MaxSize {
				return fmt.Errorf("string too large")
			}
		case []byte:
			if s.MaxSize > 0 && uint32(len(x)) > s.MaxSize {
				return fmt.Errorf("bytes too large")
			}
		}
	}
	return nil
}

func countRequired(schema []ParamSchema) int {
	n := 0
	for i := range schema {
		if schema[i].Required {
			n++
		}
	}
	return n
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

var (
	ErrModuleValidationFailed = errors.New("module validation failed")
	ErrExecutionLimitExceeded = errors.New("execution limit exceeded")
	ErrSecurityThreatDetected = errors.New("security threat detected")
	ErrInvalidSecurityConfig  = errors.New("invalid security config")
)

// NewSecurityManager 创建安全管理器
func NewSecurityManager(config *SecurityConfig) (*SecurityManager, error) {
	if config == nil {
		config = getDefaultSecurityConfig()
	}

	sm := &SecurityManager{
		config:          config,
		validationRules: getDefaultValidationRules(config),
		limitCheckers:   getDefaultLimitCheckers(config),
		threatDetector: &ThreatDetector{
			patterns:       getDefaultThreatPatterns(),
			alertThreshold: 0.7,
			activeScans:    make(map[string]*ScanSession),
		},
		securityEvents: make(chan *SecurityEvent, 100),
		stats:          &SecurityStats{},
	}

	// 启动事件处理器
	go sm.processSecurityEvents()

	return sm, nil
}

// ValidateModuleSecurity 校验模块安全属性
func ValidateModuleSecurity(module interface{}) error {
	if module == nil {
		return ErrModuleValidationFailed
	}

	// 类型断言检查
	compiledModule, ok := module.(*engine.CompiledModule)
	if !ok {
		return fmt.Errorf("invalid module type: expected *engine.CompiledModule")
	}

	sm, err := NewSecurityManager(nil)
	if err != nil {
		return fmt.Errorf("failed to create security manager: %w", err)
	}

	return sm.ValidateModule(compiledModule)
}

// EnforceExecutionLimits 执行前限制检查
func EnforceExecutionLimits(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	sm, err := NewSecurityManager(nil)
	if err != nil {
		return fmt.Errorf("failed to create security manager: %w", err)
	}

	return sm.CheckExecutionLimits(ctx)
}

// EnforcePostExecutionLimits 执行后限制检查
func EnforcePostExecutionLimits(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}
	sm, err := NewSecurityManager(nil)
	if err != nil {
		return fmt.Errorf("failed to create security manager: %w", err)
	}
	return sm.CheckPostExecutionLimits(ctx)
}

// ValidateModule 验证模块安全性
func (sm *SecurityManager) ValidateModule(module *engine.CompiledModule) error {
	if !sm.config.EnableValidation {
		return nil
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.stats.TotalValidations++
	sm.stats.LastValidation = time.Now()

	// 执行所有验证规则
	for _, rule := range sm.validationRules {
		if !rule.Enabled {
			continue
		}

		if err := rule.Validator(module); err != nil {
			sm.stats.FailedValidations++
			sm.recordSecurityEvent("validation_failed", rule.Severity,
				fmt.Sprintf("Module validation failed for rule %s: %v", rule.Name, err))

			if sm.config.StrictMode {
				return fmt.Errorf("validation rule %s failed: %w", rule.Name, err)
			}
		}
	}

	return nil
}

// CheckExecutionLimits 检查执行限制
func (sm *SecurityManager) CheckExecutionLimits(ctx context.Context) error {
	// 执行前检查
	for _, checker := range sm.limitCheckers {
		if checker.PreExecution {
			if err := checker.Check(ctx); err != nil {
				sm.stats.LimitViolations++
				sm.recordSecurityEvent("limit_violation", 8,
					fmt.Sprintf("Execution limit check failed: %s - %v", checker.Name, err))
				return fmt.Errorf("limit check %s failed: %w", checker.Name, err)
			}
		}
	}
	return nil
}

// CheckPostExecutionLimits 执行后限制检查
func (sm *SecurityManager) CheckPostExecutionLimits(ctx context.Context) error {
	for _, checker := range sm.limitCheckers {
		if checker.PostExecution {
			if err := checker.Check(ctx); err != nil {
				sm.stats.LimitViolations++
				sm.recordSecurityEvent("post_limit_violation", 8,
					fmt.Sprintf("Post execution limit failed: %s - %v", checker.Name, err))
				return fmt.Errorf("post limit check %s failed: %w", checker.Name, err)
			}
		}
	}
	return nil
}

// DetectThreats 检测威胁
func (sm *SecurityManager) DetectThreats(moduleHash string, scanData map[string]interface{}) (*ScanSession, error) {
	if !sm.config.EnableThreatDetection {
		return nil, nil
	}

	sm.threatDetector.mutex.Lock()
	defer sm.threatDetector.mutex.Unlock()

	sessionID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	session := &ScanSession{
		SessionID:   sessionID,
		StartTime:   time.Now(),
		ModuleHash:  moduleHash,
		ThreatScore: 0.0,
		Alerts:      make([]string, 0),
	}

	// 运行威胁检测模式
	for _, pattern := range sm.threatDetector.patterns {
		score := sm.evaluateThreatPattern(pattern, scanData)
		session.ThreatScore += score

		if score > sm.threatDetector.alertThreshold {
			session.Alerts = append(session.Alerts, pattern.Name)
			sm.stats.ThreatsDetected++

			sm.recordSecurityEvent("threat_detected", pattern.Severity,
				fmt.Sprintf("Threat pattern detected: %s (score: %.2f)", pattern.Name, score))
		}
	}

	sm.threatDetector.activeScans[sessionID] = session

	if session.ThreatScore > sm.threatDetector.alertThreshold {
		sm.stats.ThreatsBlocked++
		return session, ErrSecurityThreatDetected
	}

	return session, nil
}

// GetSecurityStats 获取安全统计
func (sm *SecurityManager) GetSecurityStats() *SecurityStats {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stats := *sm.stats // 复制统计数据
	return &stats
}

// UpdateConfig 更新安全配置
func (sm *SecurityManager) UpdateConfig(config *SecurityConfig) error {
	if config == nil {
		return ErrInvalidSecurityConfig
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.config = config
	return nil
}

// AddValidationRule 添加验证规则
func (sm *SecurityManager) AddValidationRule(rule ValidationRule) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.validationRules = append(sm.validationRules, rule)
}

// AddLimitChecker 添加限制检查器
func (sm *SecurityManager) AddLimitChecker(checker LimitChecker) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.limitCheckers = append(sm.limitCheckers, checker)
}

// Close 关闭安全管理器
func (sm *SecurityManager) Close() {
	close(sm.securityEvents)
}

// 内部方法

// recordSecurityEvent 记录安全事件
func (sm *SecurityManager) recordSecurityEvent(eventType string, severity int, message string) {
	event := &SecurityEvent{
		ID:        fmt.Sprintf("%s_%d", eventType, time.Now().UnixNano()),
		Type:      eventType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	select {
	case sm.securityEvents <- event:
	default:
		// 事件队列满，丢弃事件
	}
}

// processSecurityEvents 处理安全事件
func (sm *SecurityManager) processSecurityEvents() {
	for event := range sm.securityEvents {
		// 记录到日志或发送告警
		if event.Severity >= 8 {
			// 高严重性事件，可以触发告警
			_ = event // 暂时不做处理
		}
	}
}

// evaluateThreatPattern 评估威胁模式
func (sm *SecurityManager) evaluateThreatPattern(pattern ThreatPattern, scanData map[string]interface{}) float64 {
	score := 0.0

	// 简单的威胁评分算法
	for _, indicator := range pattern.Indicators {
		if value, exists := scanData[indicator]; exists {
			switch v := value.(type) {
			case bool:
				if v {
					score += 0.2
				}
			case float64:
				score += v * 0.1
			case int:
				score += float64(v) * 0.1
			}
		}
	}

	// 根据严重程度调整分数
	return score * float64(pattern.Severity) / 10.0
}

// 默认配置和规则

// getDefaultSecurityConfig 获取默认安全配置
func getDefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		EnableValidation:      true,
		EnableThreatDetection: true,
		MaxExecutionTime:      30 * time.Second,
		MaxMemoryUsage:        64 * 1024 * 1024, // 64MB
		MaxInstructionCount:   1000000,
		AllowedImports:        []string{"env"},
		DeniedInstructions:    []string{"unreachable"},
		StrictMode:            true,

		// 宿主防护默认
		AllowedHostFunctions: []string{},
		DeniedHostFunctions:  []string{},
		HostParamSchemas:     map[string][]ParamSchema{},
		EnforceIdempotency:   false,
		IdempotentFunctions:  []string{},
		EnforceSideEffects:   true,
		SideEffectPolicies:   map[string]SideEffectPolicy{},
	}
}

// getDefaultValidationRules 获取默认验证规则
func getDefaultValidationRules(config *SecurityConfig) []ValidationRule {
	return []ValidationRule{
		{
			Name:        "module_structure",
			Description: "验证模块基本结构",
			Validator: func(module *engine.CompiledModule) error {
				if module == nil {
					return errors.New("module is nil")
				}
				return nil
			},
			Enabled:  true,
			Severity: 9,
		},
		{
			Name:        "import_validation",
			Description: "验证导入函数安全性",
			Validator: func(module *engine.CompiledModule) error {
				// 校验导入模块是否在允许清单内（简化：使用 CompiledModule.Imports）
				if len(config.AllowedImports) == 0 {
					return nil
				}
				allowed := make(map[string]struct{}, len(config.AllowedImports))
				for _, m := range config.AllowedImports {
					allowed[m] = struct{}{}
				}
				for _, imp := range module.Imports {
					if _, ok := allowed[imp]; !ok {
						return fmt.Errorf("import '%s' not allowed", imp)
					}
				}
				return nil
			},
			Enabled:  true,
			Severity: 7,
		},
		{
			Name:        "memory_limits",
			Description: "验证内存使用限制（声明阶段）",
			Validator: func(module *engine.CompiledModule) error {
				// 此处无法获取运行期内存，留作占位（运行期由limitChecker负责）
				return nil
			},
			Enabled:  true,
			Severity: 5,
		},
	}
}

// getDefaultLimitCheckers 获取默认限制检查器
func getDefaultLimitCheckers(config *SecurityConfig) []LimitChecker {
	return []LimitChecker{
		{
			Name:        "execution_time",
			Description: "检查执行时间限制",
			Check: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if ok && time.Until(deadline) < 0 {
					return errors.New("execution timeout")
				}
				return nil
			},
			PreExecution:  true,
			PostExecution: false,
		},
		{
			Name:        "memory_usage",
			Description: "检查运行期内存使用上限",
			Check: func(ctx context.Context) error {
				if config.MaxMemoryUsage == 0 {
					return nil
				}
				if v := ctx.Value(KeyMemUsed); v != nil {
					switch used := v.(type) {
					case uint64:
						if used > config.MaxMemoryUsage {
							return fmt.Errorf("memory used %d > limit %d", used, config.MaxMemoryUsage)
						}
					case uint32:
						if uint64(used) > config.MaxMemoryUsage {
							return fmt.Errorf("memory used %d > limit %d", used, config.MaxMemoryUsage)
						}
					}
				}
				return nil
			},
			PreExecution:  false,
			PostExecution: true,
		},
	}
}

// getDefaultThreatPatterns 获取默认威胁模式
func getDefaultThreatPatterns() []ThreatPattern {
	return []ThreatPattern{
		{
			ID:          "infinite_loop",
			Name:        "无限循环检测",
			Description: "检测可能的无限循环攻击",
			Indicators:  []string{"high_cpu_usage", "long_execution_time"},
			Severity:    8,
			Action:      "terminate",
		},
		{
			ID:          "memory_bomb",
			Name:        "内存炸弹检测",
			Description: "检测快速内存增长攻击",
			Indicators:  []string{"rapid_memory_growth", "large_allocations"},
			Severity:    9,
			Action:      "terminate",
		},
		{
			ID:          "malicious_imports",
			Name:        "恶意导入检测",
			Description: "检测可疑的导入函数",
			Indicators:  []string{"unauthorized_imports", "suspicious_functions"},
			Severity:    7,
			Action:      "alert",
		},
	}
}
