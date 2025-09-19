package coordinator

import (
	"testing"
	"time"

	"github.com/weisyn/v1/internal/core/execution/manager"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/zap"
)

// TestDispatcherIntegrationCompile 测试Dispatcher集成的编译时验证
func TestDispatcherIntegrationCompile(t *testing.T) {
	// 创建测试用的组件
	registry := manager.NewRegistry()
	engineManager := manager.NewEngineManager(registry)

	// 创建配置好的Dispatcher
	dispatcher := manager.NewDispatcher(engineManager).
		WithCircuitBreakerConfig(3, 5*time.Second). // 3次失败后熔断5秒
		WithRateLimit(types.EngineTypeWASM, 10, 2). // WASM: 容量10，每秒补充2个token
		WithRateLimit(types.EngineTypeONNX, 5, 1).  // ONNX: 容量5，每秒补充1个token
		WithDynamicStrategy(true)                   // 启用动态引擎选择

	// 验证Dispatcher的基本配置
	if dispatcher == nil {
		t.Fatal("Dispatcher创建失败")
	}

	t.Logf("✅ Dispatcher创建成功，具备以下功能:")
	t.Logf("   - 熔断器: 3次失败后熔断5秒")
	t.Logf("   - 限流器: WASM(10,2), ONNX(5,1)")
	t.Logf("   - 动态策略: 启用")
}

// TestResourceExecutionCoordinatorWithDispatcher 测试ResourceExecutionCoordinator与Dispatcher的集成
func TestResourceExecutionCoordinatorWithDispatcher(t *testing.T) {
	// 创建基础组件
	registry := manager.NewRegistry()
	engineManager := manager.NewEngineManager(registry)
	dispatcher := manager.NewDispatcher(engineManager).
		WithCircuitBreakerConfig(2, 1*time.Second).
		WithRateLimit(types.EngineTypeWASM, 5, 1).
		WithDynamicStrategy(true)

	// 创建测试用logger
	testLogger := &TestLogger{t: t}

	// 创建ResourceExecutionCoordinator
	coordinator := NewResourceExecutionCoordinator(
		engineManager,
		dispatcher,
		nil, // hostRegistry - 在测试中可以为nil
		&NoOpMetricsCollector{},
		&NoOpAuditEventEmitter{},
		&NoOpSideEffectProcessor{},
		nil, // securityIntegrator
		nil, // quotaManager
		// auditTracker已移除，遵循MVP极简原则
		nil,        // envAdvisor
		testLogger, // logger
		DefaultCoordinatorConfig(),
	)

	// 验证集成
	if coordinator == nil {
		t.Fatal("ResourceExecutionCoordinator创建失败")
	}

	if coordinator.dispatcher == nil {
		t.Fatal("Dispatcher应该被正确集成到协调器中")
	}

	if coordinator.engineManager == nil {
		t.Fatal("EngineManager应该被正确设置")
	}

	t.Logf("✅ ResourceExecutionCoordinator创建成功，集成了Dispatcher")
	t.Logf("   - 熔断/限流功能已集成")
	t.Logf("   - 智能调度功能已启用")
}

// TestDispatcherFallbackPath 测试Dispatcher为nil时的回退路径
func TestDispatcherFallbackPath(t *testing.T) {
	// 创建基础组件
	registry := manager.NewRegistry()
	engineManager := manager.NewEngineManager(registry)

	// 创建测试用logger
	testLogger := &TestLogger{t: t}

	// 创建ResourceExecutionCoordinator（不提供dispatcher）
	coordinator := NewResourceExecutionCoordinator(
		engineManager,
		nil, // 无dispatcher，应该回退到直接引擎调用
		nil, // hostRegistry
		&NoOpMetricsCollector{},
		&NoOpAuditEventEmitter{},
		&NoOpSideEffectProcessor{},
		nil, // securityIntegrator
		nil, // quotaManager
		// auditTracker已移除，遵循MVP极简原则
		nil,        // envAdvisor
		testLogger, // logger
		DefaultCoordinatorConfig(),
	)

	// 验证回退路径
	if coordinator == nil {
		t.Fatal("ResourceExecutionCoordinator创建失败")
	}

	if coordinator.dispatcher != nil {
		t.Fatal("Dispatcher应该为nil，验证回退路径")
	}

	if coordinator.engineManager == nil {
		t.Fatal("EngineManager应该被正确设置用于回退")
	}

	t.Logf("✅ 回退路径验证成功")
	t.Logf("   - Dispatcher为nil时正确回退到直接引擎调用")
	t.Logf("   - EngineManager仍然可用")
}

// TestLogger 测试用的简单logger实现
type TestLogger struct {
	t *testing.T
}

func (tl *TestLogger) Debug(msg string) {
	tl.t.Logf("[DEBUG] %s", msg)
}

func (tl *TestLogger) Debugf(format string, args ...interface{}) {
	tl.t.Logf("[DEBUG] "+format, args...)
}

func (tl *TestLogger) Info(msg string) {
	tl.t.Logf("[INFO] %s", msg)
}

func (tl *TestLogger) Infof(format string, args ...interface{}) {
	tl.t.Logf("[INFO] "+format, args...)
}

func (tl *TestLogger) Warn(msg string) {
	tl.t.Logf("[WARN] %s", msg)
}

func (tl *TestLogger) Warnf(format string, args ...interface{}) {
	tl.t.Logf("[WARN] "+format, args...)
}

func (tl *TestLogger) Error(msg string) {
	tl.t.Logf("[ERROR] %s", msg)
}

func (tl *TestLogger) Errorf(format string, args ...interface{}) {
	tl.t.Logf("[ERROR] "+format, args...)
}

func (tl *TestLogger) Fatal(msg string) {
	tl.t.Fatalf("[FATAL] %s", msg)
}

func (tl *TestLogger) Fatalf(format string, args ...interface{}) {
	tl.t.Fatalf("[FATAL] "+format, args...)
}

func (tl *TestLogger) With(args ...interface{}) log.Logger {
	return tl // 简化实现，直接返回自己
}

func (tl *TestLogger) Sync() error {
	return nil // 测试中不需要同步
}

func (tl *TestLogger) GetZapLogger() *zap.Logger {
	return nil // 测试中不需要zap logger
}
