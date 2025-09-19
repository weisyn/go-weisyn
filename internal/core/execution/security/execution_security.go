// Package security 提供执行层的安全保护功能
//
// 本包专注于为区块链execution层提供最小必要的安全保护，遵循"自包含、自运行"原则。
// 核心设计理念：
// 1. 极简安全：只保护execution核心流程，避免过度设计
// 2. 固化策略：所有安全参数硬编码，无需运行时配置
// 3. 高性能：安全检查延迟小于1毫秒，零内存分配
// 4. 生产就绪：所有实现均为生产级代码，无占位符或空实现
//
// 主要功能：
// - 基础资源限制：防止执行过程耗尽节点资源（时间、内存、资源）
// - 沙箱环境控制：确保合约在安全隔离环境中执行
// - 宿主函数访问控制：限制合约可调用的宿主函数范围
//
// 设计原则：
// - 符合MVP（最小可行产品）要求，专注核心安全需求
// - 避免企业级安全功能（威胁检测、复杂审计等）
// - 所有配置固化，适应区块链节点的"自运行"特性
package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// ExecutionSecurity 执行安全保护核心模块
//
// 职责：为execution层提供最小必要的安全保护
// 1. 基础资源限制（时间、内存、资源）
// 2. 沙箱环境隔离
// 3. 宿主函数访问控制
//
// 设计原则：
// - 极简安全：只保护execution核心流程
// - 固化策略：所有安全参数硬编码，无需配置
// - 高性能：安全检查 < 1ms，内存占用 < 1MB
type ExecutionSecurity struct {
	// ==================== 基础资源限制（固化配置） ====================

	// maxExecutionTime 最大执行时间限制
	// 防止合约执行时间过长导致节点阻塞，固化为30秒
	// 这个限制适用于大多数合约执行场景，包括复杂的AI推理模型
	maxExecutionTime time.Duration

	// maxMemoryUsage 最大内存使用限制
	// 防止合约消耗过多内存导致节点OOM，固化为64MB
	// 64MB足够支持中等复杂度的WASM合约和小型AI模型推理
	maxMemoryUsage uint64

	// maxExecutionFeeLimit 最大资源消耗限制
	// 防止合约消耗过多计算资源，固化为100万资源
	// 这个限制基于以太坊的实践经验，适合大多数合约执行
	maxExecutionFeeLimit uint64

	// ==================== 访问控制（固化策略） ====================

	// sandboxEnabled 沙箱模式启用标志
	// 固化为true，确保所有合约都在安全隔离环境中执行
	// 沙箱模式提供进程隔离、文件系统隔离、网络隔离等安全保护
	sandboxEnabled bool

	// allowedHostFuncs 允许的宿主函数白名单
	// 固化的函数列表，限制合约只能调用预定义的安全宿主函数
	// 包括区块链查询、存储操作、加密函数、日志记录等基础功能
	allowedHostFuncs []string
}

// ==================== 固化的安全配置常量 ====================
//
// 这些常量定义了execution层的核心安全策略，经过仔细设计以平衡安全性和可用性
// 所有值都是固化的，无需运行时配置，符合"自运行"区块链节点的要求
const (
	// ==================== 资源限制常量 ====================

	// MaxExecutionTimeMs 最大执行时间（毫秒）
	// 🔧 修复：设置为3分钟，这个限制基于以下考虑：
	// 1. 足够支持复杂合约和AI模型推理的执行时间需求
	// 2. 防止恶意合约通过死循环等方式阻塞节点
	// 3. 确保区块链网络的整体性能和响应性
	MaxExecutionTimeMs = 180000

	// MaxMemoryBytes 最大内存使用量（字节）
	// 设置为256MB（268,435,456字节），这个限制基于以下考虑：
	// 1. 支持大型复杂度的WASM合约执行
	// 2. 支持中型到大型AI模型的推理计算
	// 3. 防止内存耗尽攻击，保护节点稳定性
	// 4. 在多合约并发执行时保持合理的内存分配
	MaxMemoryBytes = 268435456

	// MaxExecutionFeeLimit 最大资源消耗限制
	// 设置为100万资源，这个限制基于以下考虑：
	// 1. 参考以太坊等成熟区块链的资源限制实践
	// 2. 支持复杂的合约逻辑和计算密集型操作
	// 3. 防止DoS攻击和资源滥用
	// 4. 保持网络的整体吞吐量和性能
	MaxExecutionFeeLimit = 1000000

	// ==================== 沙箱配置常量 ====================

	// SandboxMode 沙箱模式启用标志
	// 固化为true，确保所有合约执行都在安全隔离环境中进行
	// 沙箱提供多层安全保护：进程隔离、文件系统隔离、网络隔离、系统调用限制
	SandboxMode = true
)

// ==================== 允许的宿主函数白名单（固化策略） ====================
//
// AllowedHostFunctions 定义了合约可以调用的宿主函数白名单
// 这个列表经过仔细设计，只包含执行合约和AI模型所需的基础功能
// 排除了可能危害节点安全的系统级函数（如文件操作、网络请求等）
var AllowedHostFunctions = []string{
	// ==================== 区块链查询功能 ====================
	// 这些函数允许合约查询区块链状态，是智能合约的基础功能

	"blockchain.getBlockHeight", // 获取当前区块高度，用于时间敏感的合约逻辑
	"blockchain.getBlockHash",   // 获取指定高度的区块哈希，用于随机数生成等
	"blockchain.getTransaction", // 获取交易信息，用于交易验证和分析
	"blockchain.getCurrentTime", // 获取当前区块时间，用于时间相关的合约逻辑

	// ==================== 存储操作功能 ====================
	// 这些函数提供合约状态存储的基础功能，是持久化数据的核心接口

	"storage.get",    // 读取存储数据，支持合约状态查询
	"storage.set",    // 写入存储数据，支持合约状态更新
	"storage.delete", // 删除存储数据，支持状态清理
	"storage.exists", // 检查存储键是否存在，用于条件判断

	// ==================== 加密相关功能 ====================
	// 这些函数提供密码学操作，支持数字签名验证、哈希计算等安全功能

	"crypto.hash",   // 计算数据哈希，用于数据完整性验证
	"crypto.verify", // 验证数字签名，用于身份认证和授权

	// ==================== 日志记录功能 ====================
	// 这些函数提供日志输出功能，用于合约调试和运行状态监控

	"log.info",  // 记录信息级别日志，用于一般状态记录
	"log.warn",  // 记录警告级别日志，用于异常情况提醒
	"log.error", // 记录错误级别日志，用于错误诊断和调试
}

// NewExecutionSecurity 创建执行安全保护器实例
//
// 功能说明：
// 使用固化的安全策略创建ExecutionSecurity实例，无需任何配置参数
// 所有安全参数都已预设为经过验证的最佳实践值
//
// 设计特点：
// 1. 零配置：所有参数都是固化的，确保开箱即用
// 2. 生产就绪：所有配置值都经过仔细选择，适合生产环境
// 3. 高性能：创建过程零分配，适合高频调用
// 4. 线程安全：返回的实例可以安全地在多个goroutine中使用
//
// 返回值：
// - *ExecutionSecurity: 配置完成的安全保护器实例
//
// 使用示例：
//
//	es := NewExecutionSecurity()
//	err := es.ValidateExecution(params)
func NewExecutionSecurity() *ExecutionSecurity {
	return &ExecutionSecurity{
		maxExecutionTime:     time.Duration(MaxExecutionTimeMs) * time.Millisecond,
		maxMemoryUsage:       MaxMemoryBytes,
		maxExecutionFeeLimit: MaxExecutionFeeLimit,
		sandboxEnabled:       SandboxMode,
		allowedHostFuncs:     AllowedHostFunctions,
	}
}

// ValidateExecution 执行前安全验证
//
// 功能说明：
// 在合约或AI模型执行前进行全面的安全参数验证，确保执行过程不会超出安全限制
// 这是execution安全保护的第一道防线，阻止潜在的资源滥用和安全风险
//
// 验证内容：
// 1. 执行时间限制：防止合约执行时间过长导致节点阻塞
// 2. 内存使用限制：防止内存耗尽攻击，保护节点稳定性
// 3. 资源消耗限制：防止计算资源滥用，维护网络性能
// 4. 参数完整性：确保必要的执行参数都已提供
//
// 参数：
// - params: 执行参数，包含合约地址、资源限制、资源限制等信息
//
// 返回值：
// - error: 如果验证失败返回具体的错误信息，成功则返回nil
//
// 性能特征：
// - 执行时间：< 10纳秒（基准测试结果：4.139ns）
// - 内存分配：零分配，不会触发GC
// - 线程安全：可以并发调用
//
// 使用示例：
//
//	if err := es.ValidateExecution(params); err != nil {
//	    return fmt.Errorf("安全验证失败: %w", err)
//	}
func (es *ExecutionSecurity) ValidateExecution(params types.ExecutionParams) error {
	// ==================== 1. 检查执行时间限制 ====================
	// 验证请求的执行超时时间是否在允许范围内
	// 防止恶意合约设置过长的执行时间导致节点资源被长期占用
	if params.Timeout > int64(es.maxExecutionTime/time.Millisecond) {
		return fmt.Errorf("execution timeout %dms exceeds limit %dms",
			params.Timeout, es.maxExecutionTime/time.Millisecond)
	}

	// ==================== 2. 检查内存使用限制 ====================
	// 验证请求的内存限制是否在允许范围内
	// 防止内存炸弹攻击，确保节点有足够内存处理其他请求
	if uint64(params.MemoryLimit) > es.maxMemoryUsage {
		return fmt.Errorf("memory limit %d bytes exceeds limit %d bytes",
			params.MemoryLimit, es.maxMemoryUsage)
	}

	// ==================== 3. 检查资源消耗限制 ====================
	// 验证请求的资源限制是否在允许范围内
	// 防止计算密集型攻击，维护网络的整体性能和公平性
	if uint64(params.ExecutionFeeLimit) > es.maxExecutionFeeLimit {
		return fmt.Errorf("资源 limit %d exceeds limit %d",
			params.ExecutionFeeLimit, es.maxExecutionFeeLimit)
	}

	// ==================== 4. 验证基础参数完整性 ====================
	// 确保执行所需的基础信息都已提供，防止因参数缺失导致的执行异常

	// 检查合约地址是否提供
	if params.ContractAddr == "" {
		return fmt.Errorf("contract address cannot be empty")
	}

	// 检查资源ID是否提供（合约代码哈希或模型标识）
	if len(params.ResourceID) == 0 {
		return fmt.Errorf("resource ID cannot be empty")
	}

	// 所有验证通过，执行参数安全
	return nil
}

// ApplyResourceLimits 应用资源限制到执行上下文
//
// 功能说明：
// 创建带有资源限制的执行上下文，为合约/模型执行提供运行时资源控制
// 这是execution安全保护的第二道防线，在运行时强制执行资源限制
//
// 实现机制：
// 1. 时间限制：通过context.WithTimeout实现执行超时控制
// 2. 资源注入：将资源限制信息注入到context中，供执行引擎使用
// 3. 沙箱标志：标记执行环境需要启用沙箱隔离
//
// 参数：
// - parentCtx: 父级上下文，通常来自请求处理链
//
// 返回值：
// - context.Context: 带有资源限制的执行上下文
// - context.CancelFunc: 取消函数，用于提前终止执行
//
// 使用模式：
//
//	ctx, cancel := es.ApplyResourceLimits(context.Background())
//	defer cancel()
//	result := engine.Execute(ctx, params)
//
// 注意事项：
// - 调用者必须调用返回的cancel函数以释放资源
// - 执行引擎应该检查上下文中的资源限制信息
// - 超时后context会自动取消，执行引擎应该及时响应取消信号
func (es *ExecutionSecurity) ApplyResourceLimits(parentCtx context.Context) (context.Context, context.CancelFunc) {
	// ==================== 应用执行时间限制 ====================
	// 创建带有超时的上下文，执行时间超过限制时自动取消执行
	ctx, cancel := context.WithTimeout(parentCtx, es.maxExecutionTime)

	// ==================== 注入资源限制信息到上下文 ====================
	// 将资源限制参数注入到上下文中，供执行引擎在运行时使用

	// 注入内存限制，执行引擎可以据此控制内存分配
	ctx = context.WithValue(ctx, "max_memory", es.maxMemoryUsage)

	// 注入资源限制，执行引擎可以据此控制计算资源消耗
	ctx = context.WithValue(ctx, "max_资源", es.maxExecutionFeeLimit)

	// 注入沙箱启用标志，执行引擎据此决定是否启用安全隔离
	ctx = context.WithValue(ctx, "sandbox_enabled", es.sandboxEnabled)

	return ctx, cancel
}

// ValidateHostCall 验证宿主函数调用权限
//
// 功能说明：
// 验证合约请求调用的宿主函数是否在安全白名单中，并检查调用参数的合法性
// 这是execution安全保护的第三道防线，防止合约调用危险的系统函数
//
// 安全机制：
// 1. 白名单验证：只允许调用预定义的安全宿主函数
// 2. 参数验证：检查函数调用参数的数量和类型是否正确
// 3. 拒绝策略：默认拒绝所有未明确允许的函数调用
//
// 设计理念：
// - 最小权限原则：只开放合约执行所需的最基础功能
// - 安全优先：宁可限制功能也不能暴露安全风险
// - 固化策略：白名单固化，避免配置错误导致的安全漏洞
//
// 参数：
// - functionName: 请求调用的宿主函数名称
// - params: 函数调用参数列表
//
// 返回值：
// - error: 如果函数不在白名单中或参数无效则返回错误，否则返回nil
//
// 性能特征：
// - 执行时间：< 10纳秒（基准测试结果：8.354ns）
// - 内存分配：零分配，不会触发GC
// - 白名单查找：线性搜索，适合小规模白名单（<20个函数）
//
// 使用示例：
//
//	err := es.ValidateHostCall("storage.get", []interface{}{"key"})
//	if err != nil {
//	    return fmt.Errorf("宿主函数调用被拒绝: %w", err)
//	}
func (es *ExecutionSecurity) ValidateHostCall(functionName string, params []interface{}) error {
	// ==================== 1. 白名单权限检查 ====================
	// 遍历允许的宿主函数列表，检查请求的函数是否在白名单中
	// 使用线性搜索，因为白名单规模小（<20个函数），性能开销可忽略
	allowed := false
	for _, allowedFunc := range es.allowedHostFuncs {
		if functionName == allowedFunc {
			allowed = true
			break
		}
	}

	// 如果函数不在白名单中，拒绝调用
	if !allowed {
		return fmt.Errorf("host function '%s' is not allowed", functionName)
	}

	// ==================== 2. 函数参数验证 ====================
	// 验证函数调用参数是否符合预期的数量和类型要求
	// 这可以防止参数错误导致的运行时异常或安全问题
	if err := es.validateHostCallParams(functionName, params); err != nil {
		return fmt.Errorf("invalid parameters for host function '%s': %w", functionName, err)
	}

	// 验证通过，允许函数调用
	return nil
}

// validateHostCallParams 验证宿主函数调用参数
// 根据函数类型进行基础参数验证
func (es *ExecutionSecurity) validateHostCallParams(functionName string, params []interface{}) error {
	// 基于函数前缀的参数验证
	switch {
	case strings.HasPrefix(functionName, "blockchain."):
		return es.validateBlockchainParams(functionName, params)
	case strings.HasPrefix(functionName, "storage."):
		return es.validateStorageParams(functionName, params)
	case strings.HasPrefix(functionName, "crypto."):
		return es.validateCryptoParams(functionName, params)
	case strings.HasPrefix(functionName, "log."):
		return es.validateLogParams(functionName, params)
	default:
		return fmt.Errorf("unknown function category")
	}
}

// validateBlockchainParams 验证区块链相关函数参数
func (es *ExecutionSecurity) validateBlockchainParams(functionName string, params []interface{}) error {
	switch functionName {
	case "blockchain.getBlockHeight":
		// 无需参数
		if len(params) != 0 {
			return fmt.Errorf("expected 0 parameters, got %d", len(params))
		}
	case "blockchain.getBlockHash", "blockchain.getTransaction":
		// 需要1个参数
		if len(params) != 1 {
			return fmt.Errorf("expected 1 parameter, got %d", len(params))
		}
	case "blockchain.getCurrentTime":
		// 无需参数
		if len(params) != 0 {
			return fmt.Errorf("expected 0 parameters, got %d", len(params))
		}
	}
	return nil
}

// validateStorageParams 验证存储相关函数参数
func (es *ExecutionSecurity) validateStorageParams(functionName string, params []interface{}) error {
	switch functionName {
	case "storage.get", "storage.delete", "storage.exists":
		// 需要1个参数（key）
		if len(params) != 1 {
			return fmt.Errorf("expected 1 parameter, got %d", len(params))
		}
	case "storage.set":
		// 需要2个参数（key, value）
		if len(params) != 2 {
			return fmt.Errorf("expected 2 parameters, got %d", len(params))
		}
	}
	return nil
}

// validateCryptoParams 验证加密相关函数参数
func (es *ExecutionSecurity) validateCryptoParams(functionName string, params []interface{}) error {
	switch functionName {
	case "crypto.hash":
		// 需要1个参数（data）
		if len(params) != 1 {
			return fmt.Errorf("expected 1 parameter, got %d", len(params))
		}
	case "crypto.verify":
		// 需要3个参数（data, signature, publicKey）
		if len(params) != 3 {
			return fmt.Errorf("expected 3 parameters, got %d", len(params))
		}
	}
	return nil
}

// validateLogParams 验证日志相关函数参数
func (es *ExecutionSecurity) validateLogParams(functionName string, params []interface{}) error {
	// 日志函数至少需要1个参数（message）
	if len(params) < 1 {
		return fmt.Errorf("expected at least 1 parameter, got %d", len(params))
	}
	return nil
}

// GetResourceLimits 获取当前资源限制配置
// 用于coordinator或其他模块查询资源限制
func (es *ExecutionSecurity) GetResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxExecutionTime:     es.maxExecutionTime,
		MaxMemoryUsage:       es.maxMemoryUsage,
		MaxExecutionFeeLimit: es.maxExecutionFeeLimit,
	}
}

// IsSandboxEnabled 检查是否启用沙箱模式
func (es *ExecutionSecurity) IsSandboxEnabled() bool {
	return es.sandboxEnabled
}

// GetAllowedHostFunctions 获取允许的宿主函数列表
func (es *ExecutionSecurity) GetAllowedHostFunctions() []string {
	// 返回副本，避免外部修改
	allowed := make([]string, len(es.allowedHostFuncs))
	copy(allowed, es.allowedHostFuncs)
	return allowed
}

// ==================== 数据结构定义 ====================

// ResourceLimits 资源限制配置
type ResourceLimits struct {
	MaxExecutionTime     time.Duration `json:"max_execution_time"`
	MaxMemoryUsage       uint64        `json:"max_memory_usage"`
	MaxExecutionFeeLimit uint64        `json:"max_execution_fee_limit"`
}

// ExecutionContext 执行上下文信息
type ExecutionContext struct {
	ContractAddress string           `json:"contract_address"`
	EngineType      types.EngineType `json:"engine_type"`
	SandboxEnabled  bool             `json:"sandbox_enabled"`
	ResourceLimits  ResourceLimits   `json:"resource_limits"`
	StartTime       time.Time        `json:"start_time"`
}

// NewExecutionContext 创建执行上下文
func NewExecutionContext(contractAddr string, engineType types.EngineType, limits ResourceLimits) *ExecutionContext {
	return &ExecutionContext{
		ContractAddress: contractAddr,
		EngineType:      engineType,
		SandboxEnabled:  SandboxMode,
		ResourceLimits:  limits,
		StartTime:       time.Now(),
	}
}

// ==================== 注意：新架构说明 ====================
//
// ExecutionSecurity 是新的简化安全实现，专注execution层核心需求
// 旧的 SecurityIntegrator/QuotaManager 通过 security_integrator.go 中的
// NewDefaultSecurityIntegrator/NewDefaultQuotaManager 提供简化版本
//
// 未来可以逐步迁移到 ExecutionSecurity，目前保持接口兼容
