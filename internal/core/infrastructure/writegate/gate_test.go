package writegate

import (
	"context"
	"sync"
	"testing"

	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// TestRecoveryModeBasic 测试 Recovery Mode 基本功能
func TestRecoveryModeBasic(t *testing.T) {
	gate := New()

	// 1. 启用 recovery mode
	token, err := gate.EnableRecoveryMode("test-repair")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}
	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	// 2. 检查状态
	if !gate.IsRecoveryMode() {
		t.Error("Expected IsRecoveryMode() to return true")
	}
	if gate.RecoveryPurpose() != "test-repair" {
		t.Errorf("Expected purpose 'test-repair', got '%s'", gate.RecoveryPurpose())
	}

	// 3. 禁用 recovery mode
	if err := gate.DisableRecoveryMode(token); err != nil {
		t.Fatalf("DisableRecoveryMode failed: %v", err)
	}

	// 4. 检查状态已清除
	if gate.IsRecoveryMode() {
		t.Error("Expected IsRecoveryMode() to return false after disable")
	}
	if gate.RecoveryPurpose() != "" {
		t.Error("Expected empty purpose after disable")
	}
}

// TestRecoveryModeBypassReadOnly 测试 Recovery Token 绕过只读模式
func TestRecoveryModeBypassReadOnly(t *testing.T) {
	gate := New()

	// 1. 进入只读模式
	gate.EnterReadOnly("test corruption")
	if !gate.IsReadOnly() {
		t.Fatal("Expected to be in read-only mode")
	}

	// 2. 启用 recovery mode
	token, err := gate.EnableRecoveryMode("self-repair")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}
	if token == "" {
		t.Fatal("Expected non-empty token")
	}
	defer gate.DisableRecoveryMode(token)

	// 3. 携带 recovery token 的 context
	ctx := wgif.WithWriteToken(context.Background(), token)

	// 4. 写操作应成功（即使在只读模式下）
	err = gate.AssertWriteAllowed(ctx, "test-write")
	if err != nil {
		t.Errorf("AssertWriteAllowed with recovery token should succeed even in read-only mode, got error: %v", err)
	}

	// 5. 没有 token 的写操作应失败
	ctxNoToken := context.Background()
	err = gate.AssertWriteAllowed(ctxNoToken, "test-write")
	if err == nil {
		t.Error("AssertWriteAllowed without token should fail in read-only mode")
	}
	if gate.IsReadOnly() && err != nil {
		// Expected error message should contain "read-only"
		// (简化版本，不检查具体错误消息)
	}
}

// TestRecoveryModeTokenMismatch 测试 Token 不匹配
func TestRecoveryModeTokenMismatch(t *testing.T) {
	gate := New()

	// 1. 启用 recovery mode
	token, err := gate.EnableRecoveryMode("test")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}

	// 2. 使用错误的 token 尝试禁用
	err = gate.DisableRecoveryMode("wrong-token")
	if err == nil {
		t.Error("DisableRecoveryMode with wrong token should fail")
	}

	// 3. 使用正确的 token 禁用
	if err := gate.DisableRecoveryMode(token); err != nil {
		t.Fatalf("DisableRecoveryMode with correct token failed: %v", err)
	}
}

// TestRecoveryModeConcurrent 测试并发安全
func TestRecoveryModeConcurrent(t *testing.T) {
	gate := New()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	var successToken string

	// 10 个 goroutine 同时尝试启用 recovery mode
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			token, err := gate.EnableRecoveryMode("concurrent-test")
			if err == nil {
				mu.Lock()
				successCount++
				if successToken == "" {
					successToken = token
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 应该只有一个 goroutine 成功
	if successCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to succeed, got %d", successCount)
	}

	// 清理：禁用 recovery mode
	if successToken != "" {
		_ = gate.DisableRecoveryMode(successToken)
	}
}

// TestRecoveryModeDoubleDisable 测试双重禁用是幂等的
func TestRecoveryModeDoubleDisable(t *testing.T) {
	gate := New()

	// 1. 启用 recovery mode
	token, err := gate.EnableRecoveryMode("test")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}

	// 2. 第一次禁用
	if err := gate.DisableRecoveryMode(token); err != nil {
		t.Fatalf("First DisableRecoveryMode failed: %v", err)
	}

	// 3. 第二次禁用应该是幂等的（不报错）
	if err := gate.DisableRecoveryMode(token); err != nil {
		t.Errorf("Second DisableRecoveryMode should be idempotent, got error: %v", err)
	}
}

// TestRecoveryModePriorityOverWriteFence 测试 Recovery Token 优先级高于 WriteFence
func TestRecoveryModePriorityOverWriteFence(t *testing.T) {
	gate := New()

	// 1. 启用 write fence
	fenceToken, err := gate.EnableWriteFence("reorg")
	if err != nil {
		t.Fatalf("EnableWriteFence failed: %v", err)
	}
	defer gate.DisableWriteFence(fenceToken)

	// 2. 启用 recovery mode（即使在 write fence 下也应该成功）
	recoveryToken, err := gate.EnableRecoveryMode("recovery")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}
	defer gate.DisableRecoveryMode(recoveryToken)

	// 3. 携带 recovery token 的 context 应该能通过 write fence
	ctxRecovery := wgif.WithWriteToken(context.Background(), recoveryToken)
	if err := gate.AssertWriteAllowed(ctxRecovery, "test-write"); err != nil {
		t.Errorf("AssertWriteAllowed with recovery token should bypass write fence, got error: %v", err)
	}

	// 4. 没有 token 的 context 应该被 write fence 阻止
	ctxNoToken := context.Background()
	if err := gate.AssertWriteAllowed(ctxNoToken, "test-write"); err == nil {
		t.Error("AssertWriteAllowed without token should be blocked by write fence")
	}

	// 5. 只有 fence token 的 context 应该能通过
	ctxFence := wgif.WithWriteToken(context.Background(), fenceToken)
	if err := gate.AssertWriteAllowed(ctxFence, "test-write"); err != nil {
		t.Errorf("AssertWriteAllowed with fence token should succeed, got error: %v", err)
	}
}

// TestReadOnlyDoesNotClearRecoveryToken 测试进入只读模式不清空 recovery token
func TestReadOnlyDoesNotClearRecoveryToken(t *testing.T) {
	gate := New()

	// 1. 启用 recovery mode
	recoveryToken, err := gate.EnableRecoveryMode("recovery")
	if err != nil {
		t.Fatalf("EnableRecoveryMode failed: %v", err)
	}
	defer gate.DisableRecoveryMode(recoveryToken)

	// 2. 进入只读模式
	gate.EnterReadOnly("test corruption")

	// 3. Recovery mode 应该仍然活跃
	if !gate.IsRecoveryMode() {
		t.Error("Expected IsRecoveryMode() to remain true after entering read-only")
	}

	// 4. 携带 recovery token 的写操作应该仍然成功
	ctx := wgif.WithWriteToken(context.Background(), recoveryToken)
	if err := gate.AssertWriteAllowed(ctx, "test-write"); err != nil {
		t.Errorf("AssertWriteAllowed with recovery token should still succeed after entering read-only, got error: %v", err)
	}
}

// TestRecoveryModeAlreadyEnabled 测试重复启用 recovery mode
func TestRecoveryModeAlreadyEnabled(t *testing.T) {
	gate := New()

	// 1. 第一次启用
	token1, err := gate.EnableRecoveryMode("first")
	if err != nil {
		t.Fatalf("First EnableRecoveryMode failed: %v", err)
	}
	defer gate.DisableRecoveryMode(token1)

	// 2. 第二次启用应该失败
	_, err = gate.EnableRecoveryMode("second")
	if err == nil {
		t.Error("Second EnableRecoveryMode should fail when recovery mode is already enabled")
	}
}
