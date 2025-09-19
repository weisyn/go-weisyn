package wasm

import (
	"context"
	"errors"
	"fmt"
	"time"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	types "github.com/weisyn/v1/pkg/types"

	compilerpkg "github.com/weisyn/v1/internal/core/engines/wasm/compiler"
	enginepkg "github.com/weisyn/v1/internal/core/engines/wasm/engine"
	runtimepkg "github.com/weisyn/v1/internal/core/engines/wasm/runtime"
)

// Config 为适配器层的运行配置
// - DefaultExecutionFeeLimit：默认 资源 限制
// - InstancePoolSize：实例池容量
// - ModuleCacheCapacity：编译缓存容量（由具体缓存实现处理）
// - MaxLinearMemoryPages：线性内存上限（页）
// - ExecutionTimeoutMillis：执行超时时间（毫秒）
type Config struct {
	DefaultExecutionFeeLimit uint64
	InstancePoolSize         int
	ModuleCacheCapacity      int
	MaxLinearMemoryPages     uint32
	ExecutionTimeoutMillis   int
}

var (
	errHostNotBound      = errors.New("host binding not set")
	errInstantiateFailed = errors.New("instantiate module failed")
)

// MetricsCollector 统一指标收集器接口
// 用于将WASM运行时指标集成到execution层监控系统
type MetricsCollector interface {
	RecordExecutionStart(engineType types.EngineType, resourceID []byte)
	RecordExecutionComplete(engineType types.EngineType, duration time.Duration, success bool)
	RecordResourceConsumption(engineType types.EngineType, consumed uint64)
	RecordMemoryUsage(engineType types.EngineType, used uint32)
	RecordError(errorType types.ExecutionErrorType, message string)
}

// Adapter 实现 EngineAdapter，用于对接底层引擎封装、运行时与编译器
// 仅依赖公共抽象，不引入区块链实现
type Adapter struct {
	vm        *enginepkg.VM
	cache     compilerpkg.CompiledModuleCache
	validator compilerpkg.Validator
	optimizer compilerpkg.Optimizer
	pool      runtimepkg.InstancePool
	binding   execiface.HostBinding
	cfg       *Config

	// 统一指标收集器 - 与execution层监控系统集成
	metricsCollector MetricsCollector

	// 日志记录器
	logger log.Logger
}

// NewAdapter 创建 WASM 引擎适配器
// 依赖由上层注入：底层 VM、编译缓存、验证器、优化器、实例池、配置和指标收集器
func NewAdapter(
	vm *enginepkg.VM,
	cache compilerpkg.CompiledModuleCache,
	validator compilerpkg.Validator,
	optimizer compilerpkg.Optimizer,
	pool runtimepkg.InstancePool,
	cfg *Config,
	metricsCollector MetricsCollector,
	logger log.Logger,
) execiface.EngineAdapter {
	// 设置默认配置
	defaultCfg := &Config{
		DefaultExecutionFeeLimit: 5_000_000,
		InstancePoolSize:         32,
		ModuleCacheCapacity:      1024,
		MaxLinearMemoryPages:     2048,
		ExecutionTimeoutMillis:   30_000,
	}
	if cfg != nil {
		if cfg.DefaultExecutionFeeLimit != 0 {
			defaultCfg.DefaultExecutionFeeLimit = cfg.DefaultExecutionFeeLimit
		}
		if cfg.InstancePoolSize != 0 {
			defaultCfg.InstancePoolSize = cfg.InstancePoolSize
		}
		if cfg.ModuleCacheCapacity != 0 {
			defaultCfg.ModuleCacheCapacity = cfg.ModuleCacheCapacity
		}
		if cfg.MaxLinearMemoryPages != 0 {
			defaultCfg.MaxLinearMemoryPages = cfg.MaxLinearMemoryPages
		}
		if cfg.ExecutionTimeoutMillis != 0 {
			defaultCfg.ExecutionTimeoutMillis = cfg.ExecutionTimeoutMillis
		}
	}

	return &Adapter{
		vm:               vm,
		cache:            cache,
		validator:        validator,
		optimizer:        optimizer,
		pool:             pool,
		cfg:              defaultCfg,
		metricsCollector: metricsCollector,
		logger:           logger,
	}
}

// NewAdapterWithDefaults 创建带有默认依赖的WASM引擎适配器
// 用于模块装配，自动创建所需的底层组件
func NewAdapterWithDefaults(metricsCollector MetricsCollector, logger log.Logger) execiface.EngineAdapter {
	// 创建默认VM（需要context）
	ctx := context.Background()
	vm, err := enginepkg.NewVM(ctx)
	if err != nil {
		// 对于模块装配，使用panic是合理的，因为这表示严重的配置错误
		panic(fmt.Sprintf("failed to create WASM VM: %v", err))
	}

	// 创建默认缓存
	cache := compilerpkg.NewWASMModuleCache(1024, 10*1024*1024) // 1024个条目，10MB限制

	// 创建默认验证器
	validator := compilerpkg.NewBasicValidator()

	// 创建默认优化器（使用基础优化器）
	optimizer := compilerpkg.NewBasicOptimizer(1) // 基础优化级别

	// 创建默认实例池（修正参数类型）
	var pool runtimepkg.InstancePool = runtimepkg.NewWASMInstancePool(
		32,            // maxPoolSize
		5*time.Minute, // maxIdleTime
		1*time.Minute, // cleanupInterval
	)

	// 创建默认配置
	cfg := &Config{
		DefaultExecutionFeeLimit: 5_000_000,
		InstancePoolSize:         32,
		ModuleCacheCapacity:      1024,
		MaxLinearMemoryPages:     2048,
		ExecutionTimeoutMillis:   30_000,
	}

	return NewAdapter(vm, cache, validator, optimizer, pool, cfg, metricsCollector, logger)
}

// GetEngineType 返回引擎类型标识
func (a *Adapter) GetEngineType() types.EngineType { return types.EngineTypeWASM }

// Initialize 完成引擎初始化工作（配置校验、运行时预热）
func (a *Adapter) Initialize(config map[string]any) error {
	// 合并可选动态配置
	if config != nil {
		if v, ok := config["DefaultExecutionFeeLimit"].(uint64); ok && v != 0 {
			a.cfg.DefaultExecutionFeeLimit = v
		}
		if v, ok := config["InstancePoolSize"].(int); ok && v != 0 {
			a.cfg.InstancePoolSize = v
		}
		if v, ok := config["ModuleCacheCapacity"].(int); ok && v != 0 {
			a.cfg.ModuleCacheCapacity = v
		}
		if v, ok := config["MaxLinearMemoryPages"].(uint32); ok && v != 0 {
			a.cfg.MaxLinearMemoryPages = v
		}
		if v, ok := config["ExecutionTimeoutMillis"].(int); ok && v != 0 {
			a.cfg.ExecutionTimeoutMillis = v
		}
	}
	// 运行时预热与健康检查（留待底层实现完善）
	return nil
}

// BindHost 绑定宿主标准接口
func (a *Adapter) BindHost(binding execiface.HostBinding) error {
	if binding == nil || binding.Standard() == nil {
		return fmt.Errorf("invalid host binding")
	}
	a.binding = binding
	return nil
}

// Execute 主执行流程：
// 1) 构造内部上下文
// 2) 获取或编译模块（含安全校验）
// 3) 实例化模块
// 4) 参数编码/内存绑定并调用导出函数
// 5) 执行后限制校验并归档结果
func (a *Adapter) Execute(params types.ExecutionParams) (*types.ExecutionResult, error) {
	// 记录执行开始（如果有统一监控系统）
	if a.metricsCollector != nil {
		a.metricsCollector.RecordExecutionStart(types.EngineTypeWASM, params.ResourceID)
	}

	// 记录执行开始时间
	startTime := time.Now()

	if a.binding == nil || a.binding.Standard() == nil {
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("host_binding"), errHostNotBound, params)
		return nil, errHostNotBound
	}
	if a.vm == nil {
		err := fmt.Errorf("vm not initialized")
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("vm_error"), err, params)
		return nil, err
	}

	// 构造内部上下文 + 超时
	to := a.cfg.ExecutionTimeoutMillis
	if params.Timeout > 0 {
		to = int(params.Timeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(to)*time.Millisecond)
	defer cancel()

	// 执行前限制检查（基于deadline）
	if err := runtimepkg.EnforceExecutionLimits(ctx); err != nil {
		finalErr := fmt.Errorf("pre-exec limits: %w", err)
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("resource_limit"), finalErr, params)
		return nil, finalErr
	}

	internalCtx, err := enginepkg.FromExternalContext(params)
	if err != nil {
		finalErr := fmt.Errorf("build internal context: %w", err)
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("context_error"), finalErr, params)
		return nil, finalErr
	}
	if internalCtx.ExecutionFeeLimit == 0 {
		internalCtx.ExecutionFeeLimit = a.cfg.DefaultExecutionFeeLimit
	}

	// 获取或编译模块
	compiled, err := enginepkg.GetOrCompileModule(
		ctx,
		a.cache,
		params.Payload, // 此处按需要选择字节码来源；迁移实现时对齐资源加载
		a.validator,
		a.optimizer,
		a.vm,
	)
	if err != nil {
		finalErr := fmt.Errorf("compile module: %w", err)
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("compilation"), finalErr, params)
		return nil, finalErr
	}

	// 模块安全校验（包含导入白名单）
	if err := runtimepkg.ValidateModuleSecurity(compiled); err != nil {
		finalErr := fmt.Errorf("module security: %w", err)
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("security"), finalErr, params)
		return nil, finalErr
	}

	// 实例化模块
	if a.logger != nil {
		a.logger.Debugf("🔧 开始实例化WASM模块: MemoryLimitPages=%d", a.cfg.MaxLinearMemoryPages)
	}

	inst, err := a.vm.Instantiate(ctx, compiled, enginepkg.ModuleConfig{
		MemoryLimitPages: a.cfg.MaxLinearMemoryPages,
	})

	if a.logger != nil {
		if err != nil || inst == nil {
			a.logger.Errorf("❌ WASM模块实例化失败: %v", err)
		} else {
			a.logger.Debugf("✅ WASM模块实例化成功")
		}
	}

	if err != nil || inst == nil {
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("instantiation"), errInstantiateFailed, params)
		return nil, errInstantiateFailed
	}
	defer func() { _ = inst.Close(ctx) }()

	// 参数编码与内存绑定（优先使用 params.Context["args"]）
	var argsSlice []interface{}
	if params.Context != nil {
		if v, ok := params.Context["args"].([]interface{}); ok {
			argsSlice = v
		}
	}
	if len(argsSlice) > 0 {
		execCtx2, err := enginepkg.BuildContext(params)
		if err != nil {
			return nil, fmt.Errorf("build context: %w", err)
		}
		if err := execCtx2.MapArgs(argsSlice); err != nil {
			return nil, fmt.Errorf("encode args: %w", err)
		}
		if err := execCtx2.BindArgumentsToInstance(inst, argsSlice); err != nil {
			return nil, fmt.Errorf("bind args: %w", err)
		}
		u64args := make([]any, len(execCtx2.EncodedArgs))
		for i := range execCtx2.EncodedArgs {
			u64args[i] = execCtx2.EncodedArgs[i]
		}
		if _, callErr := inst.Call(ctx, params.Entry, u64args); callErr != nil {
			finalErr := fmt.Errorf("invoke '%s': %w", params.Entry, callErr)
			a.recordExecutionFailure(startTime, types.ExecutionErrorType("execution"), finalErr, params)
			return nil, finalErr
		}
	} else {
		// 无参数调用
		// 添加调试日志
		if a.logger != nil {
			a.logger.Debugf("🔧 开始调用WASM函数: %s", params.Entry)
		}

		callResult, callErr := inst.Call(ctx, params.Entry, nil)

		if a.logger != nil {
			if callErr != nil {
				a.logger.Errorf("❌ WASM函数调用失败: %s, error: %v", params.Entry, callErr)
			} else {
				a.logger.Debugf("✅ WASM函数调用成功: %s, result: %v", params.Entry, callResult)
			}
		}

		if callErr != nil {
			finalErr := fmt.Errorf("invoke '%s': %w", params.Entry, callErr)
			a.recordExecutionFailure(startTime, types.ExecutionErrorType("execution"), finalErr, params)
			return nil, finalErr
		}
	}

	// 执行后限制检查：内存使用
	ctxPost := context.WithValue(ctx, runtimepkg.KeyMemUsed, uint64(inst.MemUsed()))
	if err := runtimepkg.EnforcePostExecutionLimits(ctxPost); err != nil {
		finalErr := fmt.Errorf("post-exec limits: %w", err)
		a.recordExecutionFailure(startTime, types.ExecutionErrorType("resource_limit"), finalErr, params)
		return nil, finalErr
	}

	// 构造统一结果（后续按真实返回与宿主回传数据补足 ReturnData/Metadata）
	result := &types.ExecutionResult{
		Success:    true,
		ReturnData: nil,
		Consumed:   0,
		Metadata:   map[string]any{"engine": "wasm"},
	}

	// 记录执行完成指标（成功）
	duration := time.Since(startTime)
	if a.metricsCollector != nil {
		a.metricsCollector.RecordExecutionComplete(types.EngineTypeWASM, duration, true)
		a.metricsCollector.RecordResourceConsumption(types.EngineTypeWASM, result.Consumed)
		if inst != nil {
			a.metricsCollector.RecordMemoryUsage(types.EngineTypeWASM, uint32(inst.MemUsed()))
		}
	}

	// ❌ **已删除：WASM运行时本地监控调用**
	// 删除原因：ObserveSuccess方法已删除，符合"避免暴露无意义运行状态"原则
	// 统一监控系统上方已记录，无需重复记录

	return result, nil
}

// recordExecutionFailure 记录执行失败的指标
func (a *Adapter) recordExecutionFailure(startTime time.Time, errorType types.ExecutionErrorType, err error, params types.ExecutionParams) {
	duration := time.Since(startTime)

	// 记录到统一监控系统
	if a.metricsCollector != nil {
		a.metricsCollector.RecordExecutionComplete(types.EngineTypeWASM, duration, false)
		a.metricsCollector.RecordError(errorType, err.Error())
	}

	// ❌ **已删除：WASM运行时本地监控调用**
	// 删除原因：ObserveFailure方法已删除，符合"避免暴露无意义运行状态"原则
	// 统一监控系统上方已记录，无需重复记录
}

// Close 释放引擎资源
func (a *Adapter) Close() error {
	if a.vm == nil {
		return nil
	}
	return a.vm.Close(context.Background())
}
