// Package tx provides transaction context interfaces.
package tx

import "context"

// ================================================================================================
// 🔑 Context Keys（上下文键）
// ================================================================================================

// verifierEnvironmentKey 是用于在context中存储VerifierEnvironment的key
//
// 💡 **设计理念**：
// 使用自定义类型作为context key，避免与其他包的key冲突。
// 这是Go官方推荐的最佳实践。
//
// 📝 **使用方式**：
//
//	// 存储环境到context
//	ctx = WithVerifierEnvironment(ctx, env)
//
//	// 从context提取环境
//	env, ok := GetVerifierEnvironment(ctx)
type verifierEnvironmentKey struct{}

// WithVerifierEnvironment 将VerifierEnvironment存储到context中
//
// 🎯 **用途**：Verifier Kernel在调用插件前将环境信息注入context
//
// 参数：
//   - ctx: 父context
//   - env: 验证环境
//
// 返回：
//   - context.Context: 包含环境信息的新context
//
// 📝 **使用示例**：
//
//	// 在Verifier Kernel中
//	ctx = tx.WithVerifierEnvironment(ctx, env)
//	err := plugin.Verify(ctx, transaction)
func WithVerifierEnvironment(ctx context.Context, env VerifierEnvironment) context.Context {
	return context.WithValue(ctx, verifierEnvironmentKey{}, env)
}

// GetVerifierEnvironment 从context中提取VerifierEnvironment
//
// 🎯 **用途**：验证插件从context获取环境信息
//
// 参数：
//   - ctx: context对象
//
// 返回：
//   - VerifierEnvironment: 验证环境（如果存在）
//   - bool: 是否成功提取（false表示context中不包含环境）
//
// 📝 **使用示例**：
//
//	// 在验证插件中
//	env, ok := tx.GetVerifierEnvironment(ctx)
//	if !ok || env == nil {
//	    return fmt.Errorf("验证环境未提供")
//	}
//	currentHeight := env.GetBlockHeight()
func GetVerifierEnvironment(ctx context.Context) (VerifierEnvironment, bool) {
	env, ok := ctx.Value(verifierEnvironmentKey{}).(VerifierEnvironment)
	return env, ok
}

// MustGetVerifierEnvironment 从context中提取VerifierEnvironment（不存在则panic）
//
// 🎯 **用途**：在确信环境一定存在的场景下使用，简化错误处理
//
// ⚠️ **注意**：仅在测试或确信环境已注入的场景下使用
//
// 参数：
//   - ctx: context对象
//
// 返回：
//   - VerifierEnvironment: 验证环境
//
// Panics：
//   - 如果context中不包含VerifierEnvironment
//
// 📝 **使用示例**：
//
//	// 在测试或确信环境存在的场景
//	env := tx.MustGetVerifierEnvironment(ctx)
//	currentHeight := env.GetBlockHeight()
func MustGetVerifierEnvironment(ctx context.Context) VerifierEnvironment {
	env, ok := GetVerifierEnvironment(ctx)
	if !ok || env == nil {
		panic("VerifierEnvironment not found in context")
	}
	return env
}

