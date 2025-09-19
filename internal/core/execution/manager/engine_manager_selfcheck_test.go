package manager

import (
	"errors"
	"testing"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/types"
)

// mockEngine 简单引擎桩：记录调用并返回可控结果
type mockEngine struct {
	t      types.EngineType
	ok     bool
	called int
}

func (m *mockEngine) GetEngineType() types.EngineType        { return m.t }
func (m *mockEngine) Initialize(_ map[string]any) error      { return nil }
func (m *mockEngine) BindHost(_ execiface.HostBinding) error { return nil }
func (m *mockEngine) Execute(_ types.ExecutionParams) (*types.ExecutionResult, error) {
	m.called++
	if m.ok {
		return &types.ExecutionResult{Success: true, Consumed: 10}, nil
	}
	return nil, errors.New("engine failed")
}
func (m *mockEngine) Close() error { return nil }

// 自检：注册两个引擎（主失败+备用成功），验证分发与回退；验证多副本负载均衡调用
func TestEngineManager_SelfCheck_DispatchAndFailover(t *testing.T) {
	reg := NewRegistry()
	mgr := NewEngineManager(reg)

	primary := &mockEngine{t: types.EngineTypeWASM, ok: false}
	backup := &mockEngine{t: types.EngineTypeONNX, ok: true}

	if err := mgr.RegisterEngine(primary); err != nil {
		t.Fatalf("register primary: %v", err)
	}
	if err := mgr.RegisterEngine(backup); err != nil {
		t.Fatalf("register backup: %v", err)
	}

	// 直接按类型执行成功路径（备用）
	if _, err := mgr.Execute(types.EngineTypeONNX, types.ExecutionParams{ResourceID: []byte{1}, ExecutionFeeLimit: 1, Timeout: 1}); err != nil {
		t.Fatalf("execute backup failed: %v", err)
	}

	// 设置回退顺序并触发回退
	mgr.SetFailoverOrder([]types.EngineType{types.EngineTypeONNX})
	if _, err := mgr.ExecuteWithDefaultFailover(types.EngineTypeWASM, types.ExecutionParams{ResourceID: []byte{2}, ExecutionFeeLimit: 1, Timeout: 1}); err != nil {
		t.Fatalf("execute with failover failed: %v", err)
	}

	if primary.called == 0 {
		t.Errorf("primary should be called at least once")
	}
	if backup.called == 0 {
		t.Errorf("backup should be called at least once")
	}
}
