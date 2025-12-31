// Package writegate 提供全局写门闸接口，用于控制系统级写操作。
//
// WriteGate 是 L2 基础设施层组件，提供横切关注点的写控制能力：
//   - 只读模式（ReadOnly）：系统级故障保护，禁止所有写操作
//   - 写围栏（WriteFence）：REORG 专用的受控写窗口，只允许持有 token 的写操作
//
// 设计原则：
//   - 接口抽象：使用方依赖接口，不依赖具体实现
//   - 全局单例：通过 Default() 获取全局实例
//   - 可测试性：支持 Mock 实现用于单元测试
//   - 多实例支持：测试场景可创建独立实例
package writegate

import "context"

// WriteGate 全局写门闸接口
//
// WriteGate 提供三种写控制机制：
//  1. ReadOnly 模式：用于不可恢复的系统级故障，完全禁止所有写操作
//  2. WriteFence 模式：用于 REORG 等需要受控写入的场景，只允许持有有效 token 的写操作
//  3. RecoveryMode 模式：用于系统自动修复，允许在只读模式下执行受控的恢复操作
//
// 优先级规则：
//  RecoveryToken > ReadOnly > WriteFenceToken > Normal
//
// 使用示例：
//
//	// 检查写操作是否允许
//	if err := writegate.Default().AssertWriteAllowed(ctx, "myOperation"); err != nil {
//	    return err
//	}
//	// 执行写操作...
//
//	// REORG 中使用写围栏
//	token, err := writegate.Default().EnableWriteFence("reorg")
//	if err != nil {
//	    return err
//	}
//	defer writegate.Default().DisableWriteFence(token)
//	ctx = writegate.WithWriteToken(ctx, token)
//	// 执行受控写操作...
//
//	// 自省修复中使用恢复模式
//	token, err := writegate.Default().EnableRecoveryMode("self-introspection-rebuild")
//	if err != nil {
//	    return err
//	}
//	defer writegate.Default().DisableRecoveryMode(token)
//	ctx = writegate.WithWriteToken(ctx, token)
//	// 执行恢复写操作（即使在只读模式下也允许）...
type WriteGate interface {
	// EnterReadOnly 进入只读模式，禁止所有写操作
	//
	// 只读模式用于系统级故障保护，当进入只读模式后：
	//   - 所有写操作调用 AssertWriteAllowed 都会失败
	//   - 写围栏（WriteFence）会被自动清除
	//   - 必须调用 ExitReadOnly() 才能恢复写操作
	//
	// 参数：
	//   - reason: 进入只读模式的原因（用于日志和错误消息）
	EnterReadOnly(reason string)

	// ExitReadOnly 退出只读模式，恢复正常写操作
	ExitReadOnly()

	// IsReadOnly 检查当前是否处于只读模式
	IsReadOnly() bool

	// ReadOnlyReason 返回进入只读模式的原因
	//
	// 如果当前不在只读模式，返回空字符串
	ReadOnlyReason() string

	// EnableWriteFence 开启写围栏，只允许持有 token 的写操作
	//
	// 写围栏用于需要受控写入的场景（如 REORG），开启后：
	//   - 生成一个唯一的 token
	//   - 只有通过 WithWriteToken(ctx, token) 携带该 token 的 context 才能通过 AssertWriteAllowed
	//   - 其他所有写操作都会被阻止
	//
	// 参数：
	//   - purpose: 写围栏的用途（用于日志和错误消息）
	//
	// 返回：
	//   - token: 写操作通行证，需要通过 WithWriteToken 绑定到 context
	//   - err: 如果当前处于只读模式，返回错误
	//
	// 注意：必须在使用完成后调用 DisableWriteFence(token) 关闭写围栏
	EnableWriteFence(purpose string) (token string, err error)

	// DisableWriteFence 关闭写围栏，恢复正常写操作
	//
	// 参数：
	//   - token: EnableWriteFence 返回的 token，必须匹配
	//
	// 返回：
	//   - err: 如果 token 不匹配，返回错误
	DisableWriteFence(token string) error

	// EnableRecoveryMode 开启恢复模式，允许在只读模式下执行受控的恢复操作
	//
	// 恢复模式用于系统自动修复场景（如自省重建），开启后：
	//   - 生成一个唯一的 recovery token
	//   - 即使在只读模式下，携带该 token 的写操作也允许通过
	//   - Recovery token 优先级高于 ReadOnly
	//
	// 参数：
	//   - purpose: 恢复操作的用途（用于日志和错误消息）
	//
	// 返回：
	//   - token: 恢复操作通行证，需要通过 WithWriteToken 绑定到 context
	//   - err: 如果已经有 recovery token 活跃，返回错误
	//
	// 注意：
	//   - 必须在使用完成后调用 DisableRecoveryMode(token) 关闭恢复模式
	//   - 同时只能有一个 recovery token 活跃
	//   - Recovery token 不受只读模式限制，需要谨慎使用
	EnableRecoveryMode(purpose string) (token string, err error)

	// DisableRecoveryMode 关闭恢复模式
	//
	// 参数：
	//   - token: EnableRecoveryMode 返回的 token，必须匹配
	//
	// 返回：
	//   - err: 如果 token 不匹配，返回错误
	DisableRecoveryMode(token string) error

	// IsRecoveryMode 检查当前是否处于恢复模式
	IsRecoveryMode() bool

	// RecoveryPurpose 返回恢复模式的用途
	//
	// 如果当前不在恢复模式，返回空字符串
	RecoveryPurpose() string

	// AssertWriteAllowed 校验写操作是否允许
	//
	// 根据当前的写控制状态，检查是否允许执行写操作：
	//   - RecoveryMode 模式：只允许携带有效 recovery token 的写操作（优先级最高，可绕过只读）
	//   - ReadOnly 模式：拒绝所有写操作（除了持有 recovery token 的）
	//   - WriteFence 模式：只允许携带有效 fence token 的写操作
	//   - 正常模式：允许所有写操作
	//
	// 参数：
	//   - ctx: 上下文，可能通过 WithWriteToken 携带 token
	//   - operation: 操作名称（用于日志和错误消息）
	//
	// 返回：
	//   - err: 如果写操作被阻止，返回错误；否则返回 nil
	//
	// 使用方式：
	//   在执行任何写操作前，必须先调用此方法检查是否允许写入
	AssertWriteAllowed(ctx context.Context, operation string) error
}

