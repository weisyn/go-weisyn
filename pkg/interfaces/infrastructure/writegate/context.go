package writegate

import "context"

// ctxKey 用于在 context 中存储写入 token 的私有 key 类型
type ctxKey struct{}

// WithWriteToken 将写入 token 绑定到 context
//
// 用于写围栏（WriteFence）场景，将 EnableWriteFence 返回的 token 绑定到 context，
// 使得该 context 可以通过 AssertWriteAllowed 的检查。
//
// 参数：
//   - ctx: 父 context
//   - token: EnableWriteFence 返回的写操作通行证
//
// 返回：
//   - 携带 token 的新 context
//
// 使用示例：
//
//	token, err := writegate.Default().EnableWriteFence("reorg")
//	if err != nil {
//	    return err
//	}
//	defer writegate.Default().DisableWriteFence(token)
//	ctx = writegate.WithWriteToken(ctx, token)
//	// 使用 ctx 进行受控写操作...
func WithWriteToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKey{}, token)
}

// TokenFromContext 从 context 中读取写入 token
//
// 如果 context 中不存在 token，返回空字符串。
//
// 参数：
//   - ctx: 可能携带 token 的 context
//
// 返回：
//   - token: 如果存在则返回 token，否则返回空字符串
//
// 注意：此函数通常由 WriteGate 实现内部使用，应用代码一般不需要直接调用
func TokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

