package host

import (
	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
)

// Binding 宿主接口绑定器
//
// # 功能说明：
// - 将标准宿主接口包装为HostBinding，提供统一的接口访问
// - 作为执行引擎和宿主能力之间的桥接层
// - 隐藏底层实现细节，仅暴露标准化的接口契约
//
// # 设计目标：
// - 接口统一：提供一致的宿主能力访问方式
// - 解耦设计：执行引擎无需了解具体的Provider实现
// - 简单包装：最小化的包装层，不增加额外复杂性
// - 类型安全：编译时确保接口契约的正确性
//
// # 使用场景：
// - 执行引擎获取宿主接口的标准入口
// - 测试环境中mock宿主能力的注入点
// - 不同执行引擎之间的接口标准化
type Binding struct {
	// std 标准宿主接口实例
	// 包含所有宿主能力的统一访问接口
	// 通常由Registry.BuildStandardInterface()创建
	std execiface.HostStandardInterface
}

// NewBinding 创建宿主接口绑定器
//
// 参数：
//   - std: 标准宿主接口实例
//
// 返回值：
//   - *Binding: 新创建的绑定器实例
//
// 使用示例：
//
//	registry := NewRegistry()
//	// ... 注册各种Provider
//	standardInterface := registry.BuildStandardInterface()
//	binding := NewBinding(standardInterface)
//
// 设计考虑：
//   - 简单包装，无额外逻辑或状态
//   - 直接传递，保持接口语义不变
func NewBinding(std execiface.HostStandardInterface) *Binding {
	return &Binding{std: std}
}

// Standard 获取标准宿主接口
//
// 返回包装的标准宿主接口实例，供执行引擎调用
//
// 返回值：
//   - execiface.HostStandardInterface: 标准宿主接口
//
// 用途：
//   - 执行引擎获取宿主能力的主要入口
//   - 提供类型安全的接口访问
//   - 保持接口契约的一致性
func (b *Binding) Standard() execiface.HostStandardInterface {
	return b.std
}
