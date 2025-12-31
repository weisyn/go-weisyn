package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// WazeroRuntime 基于wazero的WASM运行时
//
// 🎯 **核心职责**：封装wazero WebAssembly运行时，提供合约编译、实例化和执行能力
//
// 基于 github.com/tetratelabs/wazero v1.9.0 实现，
// 提供高性能的WASM合约执行环境。
//
// 📋 **设计特点**：
// - 线程安全：支持并发编译和执行
// - 编译缓存：已编译模块的内存缓存，提升性能
// - 资源隔离：每个实例独立的内存和状态
// - 宿主函数集成：支持动态注册区块链宿主函数
//
// 🔗 **依赖关系**：
// - wazero：WebAssembly运行时引擎
// - log.Logger：日志记录
// - storage.MemoryStore：编译缓存存储
type WazeroRuntime struct {
	logger log.Logger

	// wazero运行时实例
	runtime wazero.Runtime

	// 编译缓存（使用统一存储接口）
	cache storage.MemoryStore // 替代 sync.Map，使用统一存储接口

	// 进程内编译模块缓存（真实可复用对象，避免重复编译）
	compiledCache sync.Map // map[string]wazero.CompiledModule

	// 宿主函数注册状态
	hostFunctionsRegistered bool
	hostMutex               sync.RWMutex

	// 运行时配置
	config *WazeroConfig
}

// 确保WazeroRuntime实现interfaces.WASMRuntime接口
var _ interfaces.WASMRuntime = (*WazeroRuntime)(nil)

// WazeroConfig wazero运行时配置
//
// 控制wazero运行时的行为和性能特性
type WazeroConfig struct {
	// 编译模式：true使用编译器模式（高性能），false使用解释器模式（兼容性）
	UseCompiler bool

	// 执行超时（秒），0表示不限制
	ExecutionTimeoutSeconds int

	// 最大内存页数（每页64KB）
	MaxMemoryPages int

	// 最大栈深度
	MaxStackDepth int

	// 是否启用WASI支持
	EnableWASI bool

	// 编译缓存大小（已编译模块的最大缓存数量）
	CompileCacheSize int
}

// NewWazeroRuntime 创建wazero运行时
//
// 🎯 **构造器模式**：使用指定配置和缓存创建运行时实例
//
// 📋 **参数说明**：
//   - logger: 日志服务
//   - config: 运行时配置
//   - cache: 编译缓存存储（可为nil，表示不使用缓存）
func NewWazeroRuntime(logger log.Logger, config *WazeroConfig, cache storage.MemoryStore) *WazeroRuntime {
	// 使用提供的配置，如果为nil则使用默认配置
	if config == nil {
		config = &WazeroConfig{
			UseCompiler:             true, // 优先使用编译器模式
			ExecutionTimeoutSeconds: 30,   // 默认30秒超时
			MaxMemoryPages:          1024, // 默认64MB内存限制
			MaxStackDepth:           1000, // 默认栈深度
			EnableWASI:              true, // ✅ 启用WASI支持（Go编译的WASM需要）
			CompileCacheSize:        100,  // 缓存100个编译模块
		}
	}

	// 创建wazero运行时
	ctx := context.Background()
	var wasmRuntime wazero.Runtime
	if config.UseCompiler {
		wasmRuntime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(wazero.NewCompilationCache()))
	} else {
		wasmRuntime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
	}

	// 🎯 **关键修复**：实例化WASI模块
	//
	// 问题：Go编译的WASM（GOOS=wasip1）依赖WASI接口
	// 现象：module[wasi_snapshot_preview1] not instantiated
	// 解决：在创建运行时后立即实例化WASI模块
	//
	// 📋 WASI提供的系统接口：
	//   - fd_write：文件/标准输出写入
	//   - fd_read：文件/标准输入读取
	//   - environ_get：环境变量访问
	//   - clock_time_get：时间获取
	//   - random_get：随机数生成
	//   等等...
	//
	// ⚠️ 注意：
	//   - WASI模块必须在合约模块实例化之前实例化
	//   - 使用 wasi_snapshot_preview1.Instantiate() 而不是手动注册函数
	//   - 这是wazero推荐的标准做法
	if config.EnableWASI {
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, wasmRuntime); err != nil {
			// WASI实例化失败是严重错误，应该panic或返回错误
			// 但为了向后兼容，这里只记录警告
			if logger != nil {
				logger.Errorf("WASI模块实例化失败: %v", err)
			}
		} else {
			if logger != nil {
				logger.Debug("WASI模块实例化成功（wasi_snapshot_preview1）")
			}
		}
	}

	return &WazeroRuntime{
		logger:                  logger,
		runtime:                 wasmRuntime,
		cache:                   cache, // 集成外部缓存存储
		config:                  config,
		hostFunctionsRegistered: false,
	}
}

// CompileContract 编译WASM合约
//
// 🎯 **核心编译流程**：
//  1. 检查编译缓存，避免重复编译
//  2. 使用wazero编译WASM字节码
//  3. 缓存编译结果，提升后续性能
//  4. 返回可实例化的编译模块
//
// 📋 **参数说明**：
//   - ctx: 调用上下文，用于超时控制
//   - wasmBytes: WASM字节码
//
// 🔧 **返回值**：
//   - *types.CompiledContract: 编译后的合约模块
//   - error: 编译过程中的错误
func (r *WazeroRuntime) CompileContract(ctx context.Context, wasmBytes []byte) (*types.CompiledContract, error) {
	if r.logger != nil {
		r.logger.Debug("开始编译WASM合约")
	}

	// 1. 计算字节码哈希作为缓存键
	cacheKey := r.getCompileCacheKey(wasmBytes)

	// 1.1 进程内缓存命中：直接返回已编译模块
	if v, ok := r.compiledCache.Load(cacheKey); ok {
		if cm, ok := v.(wazero.CompiledModule); ok {
			return &types.CompiledContract{
				Hash:       r.calculateHash(wasmBytes),
				Module:     cm,
				CompiledAt: time.Now().Unix(),
			}, nil
		}
		// 类型异常，清理
		r.compiledCache.Delete(cacheKey)
	}

	// 2. 检查编译缓存（使用统一存储接口）
	// ⚠️ **缓存限制说明**：
	// - wazero.CompiledModule 无法序列化，无法使用需要序列化的通用缓存接口
	// - 当前实现：跳过缓存检查，每次都重新编译（性能可接受，因为编译相对快速）
	// - 未来优化方向：
	//   1. 使用内存缓存（sync.Map）缓存 CompiledModule（适合单进程场景）
	//   2. 使用 wazero 的 WithCompilationCache 配置（wazero内置缓存机制）
	//   3. 缓存 WASM 字节码哈希，避免重复编译相同合约（当前已有基础实现）
	// 这里不再使用“空字符串占位符”：
	// - r.cache 仅保存“可验证的 marker”，用于跨组件/跨实例的正确命中判定；
	// - 真正可复用的 CompiledModule 保存在 r.compiledCache（进程内）。
	if r.cache != nil {
		if cachedBytes, exists, err := r.cache.Get(ctx, cacheKey); err == nil && exists && len(cachedBytes) > 0 {
			var marker compileCacheMarker
			if err := json.Unmarshal(cachedBytes, &marker); err == nil && marker.IsValidFor(r, wasmBytes) {
				// marker 有效，但当前进程尚无 CompiledModule：继续走编译并回填进程内缓存
			}
		}
	}

	// 3. 编译WASM模块
	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("wazero编译失败: %w", err)
	}

	// 4. 打印导入清单（用于调试和验证）
	// 🎯 **关键诊断信息**：显示 WASM 模块实际导入的所有函数
	// 这有助于确认合约需要哪些宿主函数
	if r.logger != nil {
		r.logger.Debug("==================== WASM 模块导入清单 ====================")
		importedFunctions := compiled.ImportedFunctions()
		if len(importedFunctions) == 0 {
			r.logger.Debug("  （无导入函数）")
		} else {
			for _, def := range importedFunctions {
				moduleName, funcName, _ := def.Import()
				r.logger.Debugf("  [%s] %s", moduleName, funcName)
			}
		}
		r.logger.Debugf("==================== 共 %d 个导入函数 ====================", len(importedFunctions))
	}

	// 5. 构造编译合约对象
	// 注意：导出/导入函数在实例化后通过 api.Module.ExportedFunction(name) 按需查询
	// 这符合 wazero 的标准用法，无需在编译阶段预收集函数清单
	compiledContract := &types.CompiledContract{
		Hash:       r.calculateHash(wasmBytes),
		Module:     compiled, // 存储wazero.CompiledModule
		CompiledAt: time.Now().Unix(),
	}

	// 5.1 写入进程内缓存
	r.compiledCache.Store(cacheKey, compiled)

	// 6. 缓存存储（当前实现限制）
	// ⚠️ **缓存限制说明**：
	// - wazero.CompiledModule 无法序列化，无法使用需要序列化的通用缓存接口
	// - 当前实现：存储“可验证 marker”（包含 hash/编译参数/版本），避免错误命中与静默降级
	// - 未来优化方向：
	//   1. 使用内存缓存（sync.Map）缓存 CompiledModule（适合单进程场景）
	//   2. 使用 wazero 的 WithCompilationCache 配置（wazero内置缓存机制）
	//   3. 在 EngineManager 层面实现 CompiledModule 的内存缓存
	if r.cache != nil {
		marker := newCompileCacheMarker(r, wasmBytes)
		if b, err := json.Marshal(marker); err == nil {
			_ = r.cache.Set(ctx, cacheKey, b, time.Hour)
		}
	}

	if r.logger != nil {
		r.logger.Debug("WASM合约编译成功")
	}

	return compiledContract, nil
}

// compileCacheMarker 是存储层的“可验证缓存条目”（不承载 CompiledModule 本体）。
type compileCacheMarker struct {
	Version    int    `json:"version"`
	WasmSHA256 string `json:"wasm_sha256"`
	UseCompiler bool  `json:"use_compiler"`
	EnableWASI  bool  `json:"enable_wasi"`
	MaxMemoryPages int `json:"max_memory_pages"`
	MaxStackDepth  int `json:"max_stack_depth"`
	CreatedAt int64  `json:"created_at"`
}

func newCompileCacheMarker(r *WazeroRuntime, wasmBytes []byte) compileCacheMarker {
	h := sha256.Sum256(wasmBytes)
	m := compileCacheMarker{
		Version:     1,
		WasmSHA256:  fmt.Sprintf("%x", h),
		CreatedAt:   time.Now().Unix(),
	}
	if r != nil && r.config != nil {
		m.UseCompiler = r.config.UseCompiler
		m.EnableWASI = r.config.EnableWASI
		m.MaxMemoryPages = r.config.MaxMemoryPages
		m.MaxStackDepth = r.config.MaxStackDepth
	}
	return m
}

func (m compileCacheMarker) IsValidFor(r *WazeroRuntime, wasmBytes []byte) bool {
	if m.Version != 1 {
		return false
	}
	h := sha256.Sum256(wasmBytes)
	if m.WasmSHA256 != fmt.Sprintf("%x", h) {
		return false
	}
	if r == nil || r.config == nil {
		return true
	}
	return m.UseCompiler == r.config.UseCompiler &&
		m.EnableWASI == r.config.EnableWASI &&
		m.MaxMemoryPages == r.config.MaxMemoryPages &&
		m.MaxStackDepth == r.config.MaxStackDepth
}

// CreateInstance 创建合约实例
//
// 🎯 **实例化流程**：
//  1. 基于已编译模块创建实例
//  2. 绑定已注册的宿主函数
//  3. 初始化实例内存和状态
//  4. 验证导出函数的存在性
//
// 📋 **参数说明**：
//   - ctx: 调用上下文
//   - compiled: 已编译的合约模块
//
// 🔧 **返回值**：
//   - *types.WASMInstance: 创建的合约实例
//   - error: 实例化过程中的错误
func (r *WazeroRuntime) CreateInstance(ctx context.Context, compiled *types.CompiledContract) (*types.WASMInstance, error) {
	// 1. 类型断言获取wazero编译模块
	wazeroCompiled, ok := compiled.Module.(wazero.CompiledModule)
	if !ok {
		return nil, fmt.Errorf("无效的编译模块类型")
	}

	// 2. 创建模块配置
	moduleConfig := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("contract_%x", compiled.Hash[:8])). // 使用哈希前8字节作为模块名
		WithStartFunctions()                                     // 自动调用start函数

	// 3. 实例化模块
	apiModule, err := r.runtime.InstantiateModule(ctx, wazeroCompiled, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("wazero实例化失败: %w", err)
	}

	// 4. 获取内存引用
	memory := apiModule.Memory()

	// 5. 生成实例ID
	instanceID := fmt.Sprintf("instance_%x_%d", compiled.Hash[:8], time.Now().UnixNano())

	// 6. 构造WASM实例对象
	wasmInstance := &types.WASMInstance{
		ID:        instanceID,
		Hash:      compiled.Hash,
		Instance:  apiModule, // 存储api.Module
		Memory:    memory,    // 存储api.Memory
		CreatedAt: time.Now().Unix(),
		Status:    types.WASMInstanceStatusCreated,
	}

	if r.logger != nil {
		r.logger.Debug("WASM合约实例创建成功")
	}

	return wasmInstance, nil
}

// ExecuteFunction 执行合约函数
//
// 🎯 **函数执行流程**：
//  1. 验证函数存在性和签名匹配
//  2. 准备函数参数（类型转换）
//  3. 调用wazero执行函数
//  4. 处理执行结果和错误
//  5. 返回原生uint64结果数组
//
// 📋 **参数说明**：
//   - ctx: 调用上下文，用于超时控制
//   - instance: WASM合约实例
//   - functionName: 要执行的函数名
//   - params: 函数参数数组（wazero原生uint64格式）
//
// 🔧 **返回值**：
//   - []uint64: 函数执行结果（wazero原生格式）
//   - error: 执行过程中的错误
//
// 📋 **参数类型说明**：
// - i32/i64: 直接使用uint64传递
// - f32: 使用api.EncodeF32编码为uint64
// - f64: 使用api.EncodeF64编码为uint64
// - 字符串: 通过内存指针+长度传递
func (r *WazeroRuntime) ExecuteFunction(ctx context.Context, instance *types.WASMInstance, functionName string, params []uint64) ([]uint64, error) {
	// 1. 类型断言获取api.Module
	apiModule, ok := instance.Instance.(api.Module)
	if !ok {
		return nil, fmt.Errorf("无效的实例类型")
	}

	// 2. 获取导出函数
	exportedFunc := apiModule.ExportedFunction(functionName)
	if exportedFunc == nil {
		return nil, fmt.Errorf("函数 '%s' 未找到", functionName)
	}

	// 3. 设置执行超时
	executionCtx := ctx
	if r.config.ExecutionTimeoutSeconds > 0 {
		var cancel context.CancelFunc
		executionCtx, cancel = context.WithTimeout(ctx, time.Duration(r.config.ExecutionTimeoutSeconds)*time.Second)
		defer cancel()
	}

	// 4. 更新实例状态
	instance.Status = types.WASMInstanceStatusRunning

	// 5. 执行函数调用
	results, err := exportedFunc.Call(executionCtx, params...)
	if err != nil {
		instance.Status = types.WASMInstanceStatusFailed
		return nil, fmt.Errorf("函数执行失败: %w", err)
	}

	// 6. 更新实例状态
	instance.Status = types.WASMInstanceStatusFinished

	if r.logger != nil {
		r.logger.Debug("WASM函数执行成功")
	}

	return results, nil
}

// DestroyInstance 销毁合约实例
//
// 🎯 **资源清理**：
// 清理实例占用的内存和资源，防止内存泄漏
func (r *WazeroRuntime) DestroyInstance(ctx context.Context, instance *types.WASMInstance) error {
	// 1. 类型断言获取api.Module
	apiModule, ok := instance.Instance.(api.Module)
	if !ok {
		// 如果类型断言失败，可能实例已经被销毁或无效，直接返回成功
		instance.Status = types.WASMInstanceStatusDestroyed
		return nil
	}

	// 2. 关闭模块实例，释放资源
	err := apiModule.Close(ctx)
	if err != nil {
		return fmt.Errorf("销毁实例失败: %w", err)
	}

	// 3. 清理实例数据
	instance.Instance = nil
	instance.Memory = nil
	instance.Status = types.WASMInstanceStatusDestroyed

	if r.logger != nil {
		r.logger.Debug("WASM实例销毁成功")
	}

	return nil
}

// RegisterHostFunctions 注册宿主函数（完整实现）
//
// 🎯 **核心业务功能**：将Go函数注册为WASM可调用的宿主函数
//
// 📋 **功能说明**：
//   - 使用wazero的HostModuleBuilder进行注册
//   - 支持多种函数签名和参数类型
//   - 避免重复注册的状态管理
//   - 提供区块链环境所需的系统调用能力
//
// 🎯 **关键修复**：正确识别并导出绑定器返回的函数签名
//
//	绑定器返回的函数签名为 func(context.Context, WASMModule, ...) ...
//	需要包装成 api.GoModuleFunc 或 api.GoFunc 以供 wazero 使用
func (r *WazeroRuntime) RegisterHostFunctions(functions map[string]interface{}) error {
	r.hostMutex.Lock()
	defer r.hostMutex.Unlock()

	// ⚠️ **关键设计**：env模块只能实例化一次
	// wazero不允许重复实例化同名模块，所以第二次调用会报错：
	// "module[env] has already been instantiated"
	//
	// ✅ **解决方案**：
	// 1. 宿主函数只注册一次（env模块只实例化一次）
	// 2. 所有宿主函数从ctx参数动态提取ExecutionContext（不闭包捕获）
	// 3. 这样第二次调用时会跳过注册，但使用新的ExecutionContext
	if r.hostFunctionsRegistered {
		// env模块已注册，跳过
		// 宿主函数会从ctx动态获取新的ExecutionContext
		return nil
	}

	if len(functions) == 0 {
		return nil // 无函数需要注册
	}

	// 创建宿主模块构建器
	builder := r.runtime.NewHostModuleBuilder("env")

	// 注册标准宿主函数
	// 🎯 **关键修复**：binding.go 中的函数现在直接使用 api.Module
	// wazero 的 WithFunc 可以自动处理这些函数
	registeredCount := 0
	for name, fn := range functions {
		builder.NewFunctionBuilder().
			WithFunc(fn).
			Export(name)

		registeredCount++
		if r.logger != nil {
			r.logger.Debugf("注册宿主函数: %s", name)
		}
	}

	if registeredCount == 0 {
		if r.logger != nil {
			r.logger.Warn("没有宿主函数被注册")
		}
		return nil // 如果没有函数需要注册，不实例化模块
	}

	// 实例化宿主模块
	_, err := builder.Instantiate(context.Background())
	if err != nil {
		return fmt.Errorf("宿主模块实例化失败: %w", err)
	}

	r.hostFunctionsRegistered = true

	if r.logger != nil {
		r.logger.Debugf("宿主函数注册成功（共%d个函数）", len(functions))
	}

	return nil
}

// Close 关闭运行时，释放所有相关资源
//
// 📋 **功能说明**：
//   - 关闭wazero运行时
//   - 清理编译缓存
//   - 释放所有占用的资源
func (r *WazeroRuntime) Close() error {
	if r.runtime != nil {
		return r.runtime.Close(context.Background())
	}
	return nil
}

// getCompileCacheKey 生成编译缓存键
//
// 基于WASM字节码内容生成唯一的缓存键
func (r *WazeroRuntime) getCompileCacheKey(wasmBytes []byte) string {
	// 使用SHA-256计算字节码哈希
	hash := sha256.Sum256(wasmBytes)
	return fmt.Sprintf("wasm_%x", hash)
}

// calculateHash 计算WASM字节码的哈希值
func (r *WazeroRuntime) calculateHash(wasmBytes []byte) []byte {
	hash := sha256.Sum256(wasmBytes)
	return hash[:]
}

// 注意：宿主函数相关代码已简化移除
// 专注于核心WASM执行路径：编译 -> 实例化 -> 调用导出函数
//
// 🎯 **关键修复**：
// - 宿主函数签名直接使用 api.Module，无需适配器
// - binding.go 中的所有函数都已修改为使用 api.Module 参数
// - wazero 的 WithFunc 可以直接处理这些函数

// 以下功能计划在后续版本实现：
// - validateFunctionSignature: 函数签名验证（可通过ExportedFunction()动态检查）
// - setupExecutionContext: 执行上下文配置（可通过context.WithTimeout等标准方式实现）
// - GetCompiledModuleFromCache: 编译缓存管理（已有sync.Map实现基础缓存）
// - CreateInstancePool: 实例池优化（可根据性能需求决定是否实现）
