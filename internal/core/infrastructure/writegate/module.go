// Package writegate 提供全局写门闸功能
package writegate

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// ModuleInput 定义 WriteGate 模块的输入依赖
type ModuleInput struct {
	fx.In

	Logger *zap.Logger `optional:"true"` // 日志记录器（可选）
}

// ModuleOutput 定义 WriteGate 模块的输出服务
type ModuleOutput struct {
	fx.Out

	// WriteGate 全局写门闸实例（实际由 singleton.go init() 注册）
	WriteGate wgif.WriteGate
}

// Module 返回 WriteGate 模块的 fx.Option
//
// 提供：
//   - WriteGate: 全局写门闸服务
//
// 依赖：
//   - *zap.Logger: 日志记录器（可选，用于记录模块加载信息）
//
// 注意：
//   - WriteGate 实际由 singleton.go 的 init() 注册到全局单例
//   - 本模块主要作用是确保 writegate 包被加载，触发 init() 执行
//   - 同时提供 Fx 依赖注入支持，供需要显式注入的模块使用
func Module() fx.Option {
	return fx.Module("writegate",
		// 提供 WriteGate 实例
		fx.Provide(ProvideWriteGate),
		// 注册生命周期（虽然 WriteGate 无需启停，但遵循架构规范）
		fx.Invoke(RegisterLifecycle),
	)
}

// ProvideWriteGate 提供 WriteGate 实例
//
// 此函数确保：
//   1. writegate 包被加载，触发 singleton.go 的 init()
//   2. 返回全局单例供 Fx 依赖注入
func ProvideWriteGate(input ModuleInput) ModuleOutput {
	// 此时 singleton.go 的 init() 已执行，defaultGate 已注册
	gate := wgif.Default()

	if input.Logger != nil {
		input.Logger.Info("✅ WriteGate 模块已加载（全局单例已注册）",
			zap.String("module", "writegate"))
	}

	return ModuleOutput{
		WriteGate: gate,
	}
}

// RegisterLifecycle 注册 WriteGate 的生命周期管理
//
// WriteGate 是无状态的全局单例，无需启停逻辑，
// 但保留此函数以遵循基础设施模块架构规范
func RegisterLifecycle(
	lifecycle fx.Lifecycle,
	logger *zap.Logger,
) {
	// WriteGate 无需生命周期管理
	// 但如果未来需要添加（如统计信息输出），可在此扩展
	_ = lifecycle
	_ = logger
}

