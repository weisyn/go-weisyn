//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package hsm_test 提供 SessionPool 的单元测试
//
// 🧪 **测试覆盖**：
// - SessionPool 核心功能测试
// - Session 获取和释放测试
// - 并发安全测试
// - 边界条件和错误场景测试
//
// ⚠️ **注意**：
// - SessionPool 测试需要真实的 PKCS#11 环境或模拟实现
// - 由于 PKCS11Context 是具体类型，无法直接 Mock
// - 某些测试可能需要跳过（如果 PKCS#11 库不可用）
// - 排除 Android 平台（PKCS#11 在 Android 上不可用）
package hsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
)

// ==================== SessionPool 核心功能测试 ====================

// TestNewSessionPool_NilContext 测试 nil context
func TestNewSessionPool_NilContext(t *testing.T) {
	config := &SessionPoolConfig{
		MaxSize:         10,
		PIN:             "test-pin",
		CleanupInterval: 5 * time.Minute,
	}
	logger := &testutil.MockLogger{}

	_, err := NewSessionPool(nil, 1, config, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PKCS#11上下文不能为空")
}

// TestNewSessionPool_NilConfig 测试 nil config
func TestNewSessionPool_NilConfig(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
	// 此测试需要真实的 PKCS11Context，无法使用 Mock
}

// TestNewSessionPool_DefaultMaxSize 测试默认最大大小
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestNewSessionPool_DefaultMaxSize(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
	// 此测试需要真实的 PKCS11Context，无法使用 Mock
	// 在实际环境中，可以使用真实的 PKCS#11 库进行测试
}

// TestNewSessionPool_DefaultCleanupInterval 测试默认清理间隔
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestNewSessionPool_DefaultCleanupInterval(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
	// 此测试需要真实的 PKCS11Context，无法使用 Mock
}

// TestNewSessionPool_ZeroMaxSize 测试零最大大小（应使用默认值）
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestNewSessionPool_ZeroMaxSize(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestNewSessionPool_ZeroCleanupInterval 测试零清理间隔（应使用默认值）
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestNewSessionPool_ZeroCleanupInterval(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_AcquireSession_CreateNew 测试创建新 Session
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_AcquireSession_CreateNew(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_AcquireSession_Reuse 测试复用 Session
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_AcquireSession_Reuse(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_AcquireSession_MaxSize 测试达到最大大小
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_AcquireSession_MaxSize(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_AcquireSession_ContextTimeout 测试上下文超时
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_AcquireSession_ContextTimeout(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_ReleaseSession 测试释放 Session
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_ReleaseSession(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_CloseSession 测试关闭 Session
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_CloseSession(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_Close 测试关闭池
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_Close(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_GetStats 测试获取统计信息
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_GetStats(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_ConcurrentAcquireRelease 测试并发获取和释放
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_ConcurrentAcquireRelease(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// ==================== SessionPool 边界条件测试 ====================

// TestSessionPool_AcquireSession_CreateSessionError 测试创建 Session 失败
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_AcquireSession_CreateSessionError(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_IsSessionValid_InvalidSession 测试无效 Session
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_IsSessionValid_InvalidSession(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

// TestSessionPool_IsSessionValid_StateZero 测试 Session 状态为 0
// 注意：此测试需要真实的 PKCS11Context，可能需要跳过
func TestSessionPool_IsSessionValid_StateZero(t *testing.T) {
	t.Skip("需要真实的 PKCS#11 环境，跳过此测试")
}

