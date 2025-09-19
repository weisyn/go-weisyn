package engine

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	types "github.com/weisyn/v1/pkg/types"
)

// CompiledModule 表示编译后的WASM模块
// 说明：
// - 封装 wazero.CompiledModule 并附加元数据
// - 支持缓存标识与创建时间戳
// - 为模块复用与生命周期管理提供基础
type CompiledModule struct {
	module  wazero.CompiledModule
	hash    string
	created int64
	// Imports 记录模块导入（简化：默认["env"]，后续可根据wazero查询）
	Imports []string
}

// ModuleConfig 实例化配置
// - 控制线性内存上限、导入/环境绑定等参数
// - 对接 wazero.ModuleConfig 的封装
type ModuleConfig struct {
	// 线性内存页数上限（64KiB/页）。例如：2^16页≈4GiB
	MemoryLimitPages uint32
	// 最大表格大小（可选）
	MaxTableSize uint32
	// 特性列表（可选，扩展用）
	Features []string
}

// Instance 表示已实例化的模块实例
// 说明：
// - 封装 api.Module 并附加执行状态
// - 提供 资源 计量与内存使用统计
// - 支持执行元数据记录
type Instance struct {
	module       api.Module
	ctx          context.Context
	ResourceUsed uint64
	memUsed      uint32
	metadata     map[string]any
}

// ExecutionContext 执行上下文
// - 从外部执行参数转换而来的内部上下文
// - 包含 资源 限制、内存限制、超时等约束
type ExecutionContext struct {
	ExecutionFeeLimit uint64
	MemoryLimit       uint32
	Timeout           int64
	Caller            string
	ContractAddr      string
	InitParams        []byte
}

// VM 封装底层运行时生命周期与通用操作
// - 负责创建/编译/实例化模块
// - 为适配器与运行时层提供统一入口
// - 基于 wazero 实现高性能 WASM 执行
type VM struct {
	runtime wazero.Runtime
	ctx     context.Context
}

var (
	ErrModuleNotFound      = errors.New("module not found")
	ErrInvalidBytecode     = errors.New("invalid wasm bytecode")
	ErrInstantiationFailed = errors.New("module instantiation failed")
	ErrFunctionNotFound    = errors.New("exported function not found")
)

// NewVM 创建 VM
// 参数：上下文，用于运行时生命周期管理
// 返回：VM句柄或错误
func NewVM(ctx context.Context) (*VM, error) {
	// 创建 wazero 运行时配置
	runtimeConfig := wazero.NewRuntimeConfig()

	// 创建运行时实例
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// 注册 WASI 标准库（可选）
	_, err := wasi_snapshot_preview1.Instantiate(ctx, runtime)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	return &VM{
		runtime: runtime,
		ctx:     ctx,
	}, nil
}

// Compile 编译 WASM 字节码为可复用的 CompiledModule
// 参数：
//   - ctx: 上下文，用于取消与超时
//   - wasmBytes: 原始WASM字节码
//
// 返回：
//   - *CompiledModule: 编译后的模块句柄
//   - error: 失败错误
func (v *VM) Compile(ctx context.Context, wasmBytes []byte) (*CompiledModule, error) {
	if v.runtime == nil {
		return nil, errors.New("runtime not initialized")
	}

	if len(wasmBytes) == 0 {
		return nil, ErrInvalidBytecode
	}

	// 计算字节码哈希
	hash := fmt.Sprintf("%x", sha256.Sum256(wasmBytes))

	// 编译模块
	compiled, err := v.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile module failed: %w", err)
	}

	return &CompiledModule{
		module:  compiled,
		hash:    hash,
		created: time.Now().Unix(),
		Imports: []string{"env"},
	}, nil
}

// Instantiate 实例化已编译模块，返回可调用实例
// 参数：
//   - ctx: 上下文
//   - mod: 编译模块句柄
//   - cfg: 实例化配置（内存等）
//
// 返回：
//   - *Instance: 可调用的模块实例
//   - error: 失败错误
func (v *VM) Instantiate(ctx context.Context, mod *CompiledModule, cfg ModuleConfig) (*Instance, error) {
	if v.runtime == nil {
		return nil, errors.New("runtime not initialized")
	}

	if mod == nil || mod.module == nil {
		return nil, errors.New("invalid compiled module")
	}

	// 创建模块配置
	moduleConfig := wazero.NewModuleConfig()

	// 设置内存限制（wazero API 调整）
	// 注意：根据实际 wazero 版本选择正确的 API
	// v1.0+ 通常使用 WithName + 手动限制，或其他 API
	// 此处暂时跳过，后续根据实际 API 文档调整
	_ = cfg.MemoryLimitPages

	// 实例化模块
	instance, err := v.runtime.InstantiateModule(ctx, mod.module, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("instantiate module failed: %w", err)
	}

	return &Instance{
		module:       instance,
		ctx:          ctx,
		ResourceUsed: 0,
		memUsed:      0,
		metadata:     make(map[string]any),
	}, nil
}

// Call 调用模块实例的导出函数
func (inst *Instance) Call(ctx context.Context, function string, params []any) ([]any, error) {
	if inst.module == nil {
		return nil, errors.New("module instance not initialized")
	}

	// 获取导出函数
	fn := inst.module.ExportedFunction(function)
	if fn == nil {
		return nil, fmt.Errorf("function '%s' not found", function)
	}

	// 转换参数为 uint64
	var args []uint64
	for _, param := range params {
		switch v := param.(type) {
		case uint64:
			args = append(args, v)
		case int64:
			args = append(args, uint64(v))
		case uint32:
			args = append(args, uint64(v))
		case int32:
			args = append(args, uint64(v))
		case int:
			args = append(args, uint64(v))
		default:
			return nil, fmt.Errorf("unsupported parameter type: %T", v)
		}
	}

	// 调用函数
	results, err := fn.Call(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("function call failed: %w", err)
	}

	// 转换返回值
	var returnValues []any
	for _, result := range results {
		returnValues = append(returnValues, result)
	}

	return returnValues, nil
}

// Memory 返回实例的线性内存
func (inst *Instance) Memory() api.Memory {
	if inst.module == nil {
		return nil
	}
	return inst.module.Memory()
}

// AllocateAndWriteBytes 在线性内存中分配一段空间并写入数据，返回指针与长度
// 策略：使用简单的堆指针（heap_cursor）进行顺序分配，不扩容
func (inst *Instance) AllocateAndWriteBytes(data []byte) (uint32, uint32, error) {
	mem := inst.Memory()
	if mem == nil {
		return 0, 0, fmt.Errorf("memory not available")
	}

	memSize := mem.Size()
	dataLen := uint32(len(data))
	if dataLen == 0 {
		return 0, 0, nil
	}
	if dataLen > memSize {
		return 0, 0, fmt.Errorf("insufficient memory: data=%d, mem=%d", dataLen, memSize)
	}

	// 取得堆指针，首次从内存中部开始，减少与静态数据碰撞概率
	var cursor uint32
	if v, ok := inst.metadata["heap_cursor"].(uint32); ok && v > 0 {
		cursor = v
	} else {
		cursor = memSize / 2
	}

	if cursor+dataLen > memSize {
		return 0, 0, fmt.Errorf("out of memory: need=%d, cursor=%d, mem=%d", dataLen, cursor, memSize)
	}

	if ok := mem.Write(cursor, data); !ok {
		return 0, 0, fmt.Errorf("memory write failed at offset=%d", cursor)
	}

	// 更新堆指针与统计
	inst.metadata["heap_cursor"] = cursor + dataLen
	if cursor+dataLen > inst.memUsed {
		inst.memUsed = cursor + dataLen
	}
	return cursor, dataLen, nil
}

// MemUsed 返回实例已使用的线性内存字节数（近似，以最近写入位置为准）
func (inst *Instance) MemUsed() uint32 { return inst.memUsed }

// Close 关闭模块实例
func (inst *Instance) Close(ctx context.Context) error {
	if inst.module == nil {
		return nil
	}
	return inst.module.Close(ctx)
}

// Close 关闭 VM 并释放运行时资源
func (v *VM) Close(ctx context.Context) error {
	if v.runtime == nil {
		return nil
	}
	return v.runtime.Close(ctx)
}

// GetOrCompileModule 获取或编译模块（供适配器调用）
func GetOrCompileModule(ctx context.Context, cache interface{}, bytecode []byte, validator interface{}, optimizer interface{}, vm *VM) (*CompiledModule, error) {
	// 计算字节码哈希以检查缓存
	_ = fmt.Sprintf("%x", sha256.Sum256(bytecode))

	// TODO: 实现缓存查询逻辑
	// if cache != nil {
	//     if cached := cache.Get(hash); cached != nil {
	//         return cached.(*CompiledModule), nil
	//     }
	// }

	// TODO: 实现验证逻辑
	// if validator != nil {
	//     if err := validator.Validate(bytecode); err != nil {
	//         return nil, fmt.Errorf("validation failed: %w", err)
	//     }
	// }

	// TODO: 实现优化逻辑
	// if optimizer != nil {
	//     bytecode = optimizer.Optimize(bytecode)
	// }

	// 编译模块
	compiled, err := vm.Compile(ctx, bytecode)
	if err != nil {
		return nil, err
	}

	// TODO: 缓存编译结果
	// if cache != nil {
	//     cache.Set(hash, compiled)
	// }

	return compiled, nil
}

// FromExternalContext 从外部参数构造内部上下文
func FromExternalContext(params types.ExecutionParams) (*ExecutionContext, error) {
	if params.Entry == "" {
		return nil, errors.New("entry function not specified")
	}

	return &ExecutionContext{
		ExecutionFeeLimit: params.ExecutionFeeLimit,
		MemoryLimit:       params.MemoryLimit,
		Timeout:           params.Timeout,
		Caller:            params.Caller,
		ContractAddr:      params.ContractAddr,
		InitParams:        params.Payload,
	}, nil
}
