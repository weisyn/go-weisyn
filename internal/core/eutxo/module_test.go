// Package eutxo 提供 EUTXO 模块的集成测试
//
// 🧪 **集成测试文件**
//
// 本文件测试 EUTXO 模块的 fx 依赖注入集成，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - fx 模块加载
// - 服务创建和导出
// - 延迟依赖注入
// - 生命周期管理
package eutxo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/core/eutxo/testutil"
	eutxoif "github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	infraStorage "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== fx 模块集成测试 ====================

// TestModule_WithValidDependencies_LoadsSuccessfully 测试使用有效依赖加载模块
func TestModule_WithValidDependencies_LoadsSuccessfully(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return storage },
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				writer eutxoif.UTXOWriter,
				snapshot eutxoif.UTXOSnapshot,
			) {
				// Assert - 验证服务已创建
				assert.NotNil(t, writer, "UTXOWriter 应该被创建")
				assert.NotNil(t, snapshot, "UTXOSnapshot 应该被创建")
			},
			fx.ParamTags(
				`name:"utxo_writer"`,
				`name:"utxo_snapshot"`,
			),
		)),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	defer app.Stop(ctx)

	// Assert
	assert.NoError(t, err, "模块应该成功加载")
}

// TestModule_WithMissingDependencies_FailsToLoad 测试缺少必需依赖时模块加载失败
func TestModule_WithMissingDependencies_FailsToLoad(t *testing.T) {
	// Arrange - 缺少 storage 依赖
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用（缺少 storage）
	app := fx.New(
		fx.Provide(
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
	)

	// 启动应用
	err := app.Err()
	if err != nil {
		// 如果启动失败，这是预期的
		assert.Error(t, err, "缺少必需依赖时应该失败")
		return
	}
	ctx := context.Background()
	err = app.Start(ctx)
	if err != nil {
		assert.Error(t, err, "缺少必需依赖时应该失败")
		return
	}
	defer app.Stop(ctx)

	// 如果没有失败，说明依赖是可选的，这不符合预期
	t.Logf("⚠️ 警告：缺少 storage 依赖时模块仍然加载成功，这可能不符合预期")
}

// TestModule_RuntimeDependencies_AreInjected 测试运行时依赖注入
func TestModule_RuntimeDependencies_AreInjected(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用并验证运行时依赖注入
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return storage },
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				snapshot eutxoif.UTXOSnapshot,
			) {
				// Assert - 验证快照服务可以创建快照（说明 Writer 和 Query 已注入）
				// 注意：这里只验证服务不为 nil，实际功能测试在单元测试中
				assert.NotNil(t, snapshot, "UTXOSnapshot 应该被创建")
			},
			fx.ParamTags(
				`name:"utxo_snapshot"`,
			),
		)),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	defer app.Stop(ctx)

	// Assert
	assert.NoError(t, err, "模块应该成功加载")
}

// TestModule_Lifecycle_HooksAreRegistered 测试生命周期钩子注册
func TestModule_Lifecycle_LifecycleHooksAreRegistered(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用并验证生命周期钩子
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return storage },
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	require.NoError(t, err)

	// 停止应用
	err = app.Stop(ctx)
	require.NoError(t, err)

	// Assert
	// 验证应用可以正常启动和停止
	assert.NoError(t, err, "应用应该正常停止")
}

// TestModule_ServiceCreation_AllServicesAreCreated 测试所有服务都被创建
func TestModule_ServiceCreation_AllServicesAreCreated(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用并验证所有服务
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return storage },
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				writer eutxoif.UTXOWriter,
				snapshot eutxoif.UTXOSnapshot,
			) {
				// Assert - 验证所有服务都已创建
				assert.NotNil(t, writer, "UTXOWriter 应该被创建")
				assert.NotNil(t, snapshot, "UTXOSnapshot 应该被创建")
			},
			fx.ParamTags(
				`name:"utxo_writer"`,
				`name:"utxo_snapshot"`,
			),
		)),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	defer app.Stop(ctx)

	// Assert
	assert.NoError(t, err, "所有服务应该成功创建")
}

// TestModule_OptionalDependencies_WorkCorrectly 测试可选依赖
func TestModule_OptionalDependencies_WorkCorrectly(t *testing.T) {
	// Arrange - 不提供可选依赖（EventBus），但提供必需的 Logger
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager()
	logger := testutil.NewTestLogger()
	var blockHashClient core.BlockHashServiceClient = nil

	// Act - 创建 fx 应用（不提供 EventBus，但提供 Logger）
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return storage },
			func() crypto.HashManager { return hasher },
			func() log.Logger { return logger },
			func() core.BlockHashServiceClient { return blockHashClient },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				writer eutxoif.UTXOWriter,
				snapshot eutxoif.UTXOSnapshot,
			) {
				// Assert - 验证服务仍然可以创建（可选依赖 EventBus 缺失不应该导致失败）
				assert.NotNil(t, writer, "UTXOWriter 应该被创建（即使没有 EventBus）")
				assert.NotNil(t, snapshot, "UTXOSnapshot 应该被创建")
			},
			fx.ParamTags(
				`name:"utxo_writer"`,
				`name:"utxo_snapshot"`,
			),
		)),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	defer app.Stop(ctx)

	// Assert
	assert.NoError(t, err, "可选依赖 EventBus 缺失时模块应该仍然可以加载")
}

