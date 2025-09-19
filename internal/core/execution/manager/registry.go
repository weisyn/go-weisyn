package manager

import (
	"fmt"
	"sort"
	"sync"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	types "github.com/weisyn/v1/pkg/types"
)

// Registry 引擎注册表
//
// # 核心功能：
// - 维护引擎类型到EngineAdapter的一对一映射
// - 确保每种引擎类型只有一个主适配器
// - 提供并发安全的注册、查询、注销操作
//
// # 设计目标：
// - 并发安全：读写锁保护，支持高并发查询
// - 域唯一：每种引擎类型只能注册一次
// - 可枚举：支持列出所有已注册引擎类型
// - 高性能：O(1)查询复杂度，微秒级延迟
//
// # 使用场景：
// - 系统启动时注册各种执行引擎
// - 执行时快速查找适配器
// - 动态管理引擎生命周期
// - 系统状态查询和诊断
type Registry struct {
	// mu 读写锁
	// 保护engines映射的并发访问安全
	// 读写锁优化读多写少的访问模式
	mu sync.RWMutex

	// engines 引擎类型到适配器的映射
	// 键：引擎类型（如EngineTypeWASM、EngineTypeONNX）
	// 值：对应的引擎适配器实现
	// 设计：每种类型只能有一个主适配器
	engines map[types.EngineType]execiface.EngineAdapter
}

// NewRegistry 创建引擎注册表
//
// 返回初始化完成的空注册表实例
//
// 返回值：
//   - *Registry: 新创建的注册表实例
//
// 初始状态：
//   - 内部映射为空，无已注册引擎
//   - 读写锁已初始化，可安全并发使用
//
// 使用示例：
//
//	reg := NewRegistry()
//	reg.Register(wasmAdapter)
//	reg.Register(onnxAdapter)
func NewRegistry() *Registry {
	return &Registry{engines: make(map[types.EngineType]execiface.EngineAdapter)}
}

// Register 注册引擎适配器（同类型唯一）
func (r *Registry) Register(adapter execiface.EngineAdapter) error {
	if adapter == nil {
		return fmt.Errorf("engine adapter is nil")
	}
	t := adapter.GetEngineType()
	if t == "" {
		return fmt.Errorf("engine type is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.engines[t]; exists {
		return fmt.Errorf("engine already registered: %s", t)
	}
	r.engines[t] = adapter
	return nil
}

// Unregister 取消注册引擎
func (r *Registry) Unregister(t types.EngineType) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.engines[t]; !exists {
		return false
	}
	delete(r.engines, t)
	return true
}

// Get 按类型获取引擎
func (r *Registry) Get(t types.EngineType) (execiface.EngineAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ad, ok := r.engines[t]
	return ad, ok
}

// List 列出所有已注册引擎类型
func (r *Registry) List() []types.EngineType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]types.EngineType, 0, len(r.engines))
	for k := range r.engines {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}
