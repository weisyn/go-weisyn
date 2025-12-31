package engines

import (
	"context"
	"fmt"
	"sync"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ============================================================================
// 引擎注册表实现（可扩展性增强）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现EngineRegistry接口，提供引擎注册和查找机制。
//
// 🏗️ **实现策略**：
// - 使用map存储引擎实例（key为EngineType）
// - 使用sync.RWMutex保护并发访问
// - 提供线程安全的注册、注销、查找操作
//
// ============================================================================

// Registry 引擎注册表实现
type Registry struct {
	engines map[ispcInterfaces.EngineType]ispcInterfaces.Engine
	mutex   sync.RWMutex
}

// NewRegistry 创建引擎注册表
func NewRegistry() *Registry {
	return &Registry{
		engines: make(map[ispcInterfaces.EngineType]ispcInterfaces.Engine),
	}
}

// Register 注册引擎
func (r *Registry) Register(engine ispcInterfaces.Engine) error {
	if engine == nil {
		return fmt.Errorf("engine cannot be nil")
	}

	metadata := engine.GetMetadata()
	if metadata.Type == "" {
		return fmt.Errorf("engine type cannot be empty")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 检查是否已存在
	if _, exists := r.engines[metadata.Type]; exists {
		return fmt.Errorf("engine type %s already registered", metadata.Type)
	}

	r.engines[metadata.Type] = engine
	return nil
}

// Unregister 注销引擎
func (r *Registry) Unregister(engineType ispcInterfaces.EngineType) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.engines[engineType]; !exists {
		return fmt.Errorf("engine type %s not found", engineType)
	}

	delete(r.engines, engineType)
	return nil
}

// Get 获取指定类型的引擎
func (r *Registry) Get(engineType ispcInterfaces.EngineType) (ispcInterfaces.Engine, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	engine, exists := r.engines[engineType]
	return engine, exists
}

// List 列出所有已注册的引擎
func (r *Registry) List() []ispcInterfaces.EngineMetadata {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]ispcInterfaces.EngineMetadata, 0, len(r.engines))
	for _, engine := range r.engines {
		result = append(result, engine.GetMetadata())
	}

	return result
}

// Has 检查指定类型的引擎是否已注册
func (r *Registry) Has(engineType ispcInterfaces.EngineType) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.engines[engineType]
	return exists
}

// ============================================================================
// WASM引擎适配器（向后兼容）
// ============================================================================

// WASMEngineAdapter WASM引擎适配器
//
// 🎯 **设计目的**：
// 将InternalWASMEngine适配到统一的Engine接口，实现向后兼容。
type WASMEngineAdapter struct {
	engine ispcInterfaces.InternalWASMEngine
}

// NewWASMEngineAdapter 创建WASM引擎适配器
func NewWASMEngineAdapter(engine ispcInterfaces.InternalWASMEngine) *WASMEngineAdapter {
	return &WASMEngineAdapter{
		engine: engine,
	}
}

// GetMetadata 获取引擎元数据
func (a *WASMEngineAdapter) GetMetadata() ispcInterfaces.EngineMetadata {
	return ispcInterfaces.EngineMetadata{
		Type:        ispcInterfaces.EngineTypeWASM,
		Name:        "WASM Engine",
		Version:     "1.0.0",
		Description: "WebAssembly合约执行引擎",
		Capabilities: []string{"execution", "hostabi", "debugging"},
	}
}

// Execute 执行WASM合约
func (a *WASMEngineAdapter) Execute(
	ctx context.Context,
	resourceHash []byte,
	method string,
	params interface{},
) (interface{}, error) {
	paramsTyped, ok := params.([]uint64)
	if !ok {
		return nil, fmt.Errorf("invalid params type for WASM engine, expected []uint64")
	}

	result, err := a.engine.CallFunction(ctx, resourceHash, method, paramsTyped)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Close 关闭引擎
func (a *WASMEngineAdapter) Close() error {
	return a.engine.Close()
}

// ============================================================================
// ONNX引擎适配器（向后兼容）
// ============================================================================

// ONNXEngineAdapter ONNX引擎适配器
//
// 🎯 **设计目的**：
// 将InternalONNXEngine适配到统一的Engine接口，实现向后兼容。
type ONNXEngineAdapter struct {
	engine ispcInterfaces.InternalONNXEngine
}

// NewONNXEngineAdapter 创建ONNX引擎适配器
func NewONNXEngineAdapter(engine ispcInterfaces.InternalONNXEngine) *ONNXEngineAdapter {
	return &ONNXEngineAdapter{
		engine: engine,
	}
}

// GetMetadata 获取引擎元数据
func (a *ONNXEngineAdapter) GetMetadata() ispcInterfaces.EngineMetadata {
	return ispcInterfaces.EngineMetadata{
		Type:        ispcInterfaces.EngineTypeONNX,
		Name:        "ONNX Engine",
		Version:     "1.0.0",
		Description: "ONNX模型推理引擎",
		Capabilities: []string{"inference", "tensor", "model_cache"},
	}
}

// Execute 执行ONNX模型推理
func (a *ONNXEngineAdapter) Execute(
	ctx context.Context,
	resourceHash []byte,
	method string,
	params interface{},
) (interface{}, error) {
	// 支持两种输入格式：[][]float64（向后兼容）或[]TensorInput
	var tensorInputs []ispcInterfaces.TensorInput
	switch v := params.(type) {
	case [][]float64:
		// 转换为TensorInput格式（float32类型）
		tensorInputs = make([]ispcInterfaces.TensorInput, len(v))
		for i, data := range v {
			tensorInputs[i] = ispcInterfaces.TensorInput{
				Data:     data,
				DataType: "float32",
				// Shape为空，将从模型元数据获取
			}
		}
	case []ispcInterfaces.TensorInput:
		tensorInputs = v
	default:
		return nil, fmt.Errorf("invalid params type for ONNX engine, expected [][]float64 or []TensorInput")
	}

	result, err := a.engine.CallModel(ctx, resourceHash, tensorInputs)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Close 关闭引擎
func (a *ONNXEngineAdapter) Close() error {
	return a.engine.Shutdown()
}

