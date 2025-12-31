package writegate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// gateImpl 是 WriteGate 接口的默认实现
//
// 实现了三种写控制机制：
//  1. ReadOnly 模式：完全禁止所有写操作
//  2. WriteFence 模式：只允许持有有效 token 的写操作
//  3. RecoveryMode 模式：允许在只读模式下执行受控的恢复操作
//
// 优先级规则：RecoveryToken > ReadOnly > WriteFenceToken > Normal
//
// 线程安全：使用 RWMutex 保护内部状态
type gateImpl struct {
	mu sync.RWMutex

	// ReadOnly 模式相关字段
	readOnly   bool
	reason     string
	readOnlyAt time.Time

	// WriteFence 模式相关字段
	fenceEnabled bool
	fenceToken   string
	fencePurpose string
	fenceAt      time.Time

	// Recovery 模式相关字段
	recoveryEnabled bool
	recoveryToken   string
	recoveryPurpose string
	recoveryAt      time.Time
}

// 编译时检查：确保 gateImpl 实现了 WriteGate 接口
var _ wgif.WriteGate = (*gateImpl)(nil)

// New 创建一个新的 WriteGate 实例
//
// 主要用于测试场景，生产环境应使用 writegate.Default() 获取全局实例
func New() wgif.WriteGate {
	return &gateImpl{}
}

// EnterReadOnly 进入只读模式
func (g *gateImpl) EnterReadOnly(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.readOnly = true
	g.reason = reason
	g.readOnlyAt = time.Now()
	// 进入只读后，清空写 fence，避免"绕过只读"的 token 存在
	// 注意：不清空 recovery token，因为 recovery mode 优先级高于只读模式
	g.fenceEnabled = false
	g.fenceToken = ""
	g.fencePurpose = ""
	g.fenceAt = time.Time{}
}

// ExitReadOnly 退出只读模式
func (g *gateImpl) ExitReadOnly() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.readOnly = false
	g.reason = ""
	g.readOnlyAt = time.Time{}
}

// IsReadOnly 检查是否处于只读模式
func (g *gateImpl) IsReadOnly() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.readOnly
}

// ReadOnlyReason 返回只读模式的原因
func (g *gateImpl) ReadOnlyReason() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.reason
}

// EnableWriteFence 开启写围栏
func (g *gateImpl) EnableWriteFence(purpose string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.readOnly {
		return "", fmt.Errorf("node is read-only: %s", g.reason)
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	g.fenceEnabled = true
	g.fenceToken = token
	g.fencePurpose = purpose
	g.fenceAt = time.Now()
	return token, nil
}

// DisableWriteFence 关闭写围栏
func (g *gateImpl) DisableWriteFence(token string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.fenceEnabled {
		return nil
	}
	if g.fenceToken != token {
		return fmt.Errorf("write fence token mismatch")
	}
	g.fenceEnabled = false
	g.fenceToken = ""
	g.fencePurpose = ""
	g.fenceAt = time.Time{}
	return nil
}

// EnableRecoveryMode 开启恢复模式
func (g *gateImpl) EnableRecoveryMode(purpose string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 检查是否已有 recovery token 活跃
	if g.recoveryEnabled {
		return "", fmt.Errorf("recovery mode already enabled: %s", g.recoveryPurpose)
	}

	token, err := randomToken()
	if err != nil {
		return "", err
	}

	g.recoveryEnabled = true
	g.recoveryToken = token
	g.recoveryPurpose = purpose
	g.recoveryAt = time.Now()

	return token, nil
}

// DisableRecoveryMode 关闭恢复模式
func (g *gateImpl) DisableRecoveryMode(token string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.recoveryEnabled {
		return nil
	}

	if g.recoveryToken != token {
		return fmt.Errorf("recovery token mismatch")
	}

	g.recoveryEnabled = false
	g.recoveryToken = ""
	g.recoveryPurpose = ""
	g.recoveryAt = time.Time{}

	return nil
}

// IsRecoveryMode 检查当前是否处于恢复模式
func (g *gateImpl) IsRecoveryMode() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.recoveryEnabled
}

// RecoveryPurpose 返回恢复模式的用途
func (g *gateImpl) RecoveryPurpose() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.recoveryPurpose
}

// AssertWriteAllowed 校验写操作是否允许
func (g *gateImpl) AssertWriteAllowed(ctx context.Context, op string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 1. 检查 Recovery Token（优先级最高）
	// Recovery token 可以绕过只读模式，用于系统自动修复
	if g.recoveryEnabled {
		token := wgif.TokenFromContext(ctx)
		if token != "" && token == g.recoveryToken {
			// ✅ 持有有效 recovery token，允许写入（即使在只读模式下）
			return nil
		}
	}

	// 2. 检查 ReadOnly 模式
	if g.readOnly {
		return fmt.Errorf("write blocked (read-only): op=%s reason=%s", op, g.reason)
	}

	// 3. 检查 WriteFence
	if g.fenceEnabled {
		token := wgif.TokenFromContext(ctx)
		if token == "" || token != g.fenceToken {
			return fmt.Errorf("write blocked (write-fence): op=%s purpose=%s", op, g.fencePurpose)
		}
	}

	return nil
}

// randomToken 生成随机 token
func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

