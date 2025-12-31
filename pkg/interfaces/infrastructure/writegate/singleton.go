package writegate

import (
	"context"
	"flag"
	"sync"
)

var (
	// defaultGate 全局默认 WriteGate 实例
	defaultGate WriteGate
	// mu 保护 defaultGate 的并发访问
	mu sync.RWMutex

	// fallbackGate 用于测试/工具场景的兜底实现（允许所有写）
	// 目的：避免单测因未显式导入实现包（internal/core/infrastructure/writegate）而 panic。
	fallbackGate WriteGate = allowAllGate{}
)

// Default 返回全局默认 WriteGate 实例
//
// 这是获取 WriteGate 的标准方式，适用于生产环境和大多数测试场景。
// 实现层会在 init() 中调用 SetDefault() 注册默认实例。
//
// 返回：
//   - WriteGate: 全局默认实例
//
// Panic：
//   - 如果没有实现层注册默认实例，会 panic
//
// 使用示例：
//
//	if err := writegate.Default().AssertWriteAllowed(ctx, "myOperation"); err != nil {
//	    return err
//	}
func Default() WriteGate {
	mu.RLock()
	defer mu.RUnlock()
	if defaultGate == nil {
		// 仅在 go test 环境下启用兜底，生产环境仍保持 fail-fast。
		if flag.Lookup("test.v") != nil {
			return fallbackGate
		}
		panic("writegate: no default WriteGate implementation registered")
	}
	return defaultGate
}

// SetDefault 设置全局默认 WriteGate 实例
//
// 此函数由实现层在 init() 中调用，用于注册默认实例。
// 应用代码不应直接调用此函数，除非在测试中需要 Mock WriteGate。
//
// 参数：
//   - gate: 要设置为默认实例的 WriteGate 实现
//
// 测试中使用示例：
//
//	// 保存原实例
//	oldGate := writegate.Default()
//	defer writegate.SetDefault(oldGate)
//
//	// 设置 Mock 实例
//	mockGate := &MockWriteGate{}
//	writegate.SetDefault(mockGate)
//
//	// 执行测试...
func SetDefault(gate WriteGate) {
	mu.Lock()
	defer mu.Unlock()
	defaultGate = gate
}

// allowAllGate 是 WriteGate 的最简实现：永远允许写入。
// 仅用于测试兜底，不应作为生产策略依赖。
type allowAllGate struct{}

func (allowAllGate) EnterReadOnly(string)                              {}
func (allowAllGate) ExitReadOnly()                                     {}
func (allowAllGate) IsReadOnly() bool                                   { return false }
func (allowAllGate) ReadOnlyReason() string                             { return "" }
func (allowAllGate) EnableWriteFence(string) (string, error)            { return "noop", nil }
func (allowAllGate) DisableWriteFence(string) error                     { return nil }
func (allowAllGate) EnableRecoveryMode(string) (string, error)          { return "noop", nil }
func (allowAllGate) DisableRecoveryMode(string) error                   { return nil }
func (allowAllGate) IsRecoveryMode() bool                               { return false }
func (allowAllGate) RecoveryPurpose() string                            { return "" }
func (allowAllGate) AssertWriteAllowed(context.Context, string) error   { return nil }

