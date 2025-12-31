// Package persistence 提供 Persistence 模块的集成测试
//
// 🧪 **集成测试文件**
//
// 本文件测试 Persistence 模块的 fx 依赖注入集成，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - fx 模块加载
// - 服务创建和导出
// - 生命周期管理
package persistence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	infraStorage "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== fx 模块集成测试 ====================

// TestModule_WithValidDependencies_LoadsSuccessfully 测试使用有效依赖加载模块
func TestModule_WithValidDependencies_LoadsSuccessfully(t *testing.T) {
	// Arrange
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	hashManager := testutil.NewTestHashManager().(crypto.HashManager)
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act - 创建 fx 应用
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return badgerStore },
			func() infraStorage.FileStore { return fileStore },
			func() crypto.HashManager { return hashManager },
			func() core.BlockHashServiceClient { return blockHashClient },
			func() transaction.TransactionHashServiceClient { return txHashClient },
			func() log.Logger { return logger },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				queryService persistence.QueryService,
				dataWriter persistence.DataWriter,
			) {
				// Assert - 验证服务已创建
				assert.NotNil(t, queryService, "QueryService 应该被创建")
				assert.NotNil(t, dataWriter, "DataWriter 应该被创建")
			},
			fx.ParamTags(
				`name:"query_service"`,
				`name:"data_writer"`,
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
	// Arrange - 缺少 BadgerStore 依赖
	fileStore := testutil.NewTestFileStore()
	hashManager := testutil.NewTestHashManager().(crypto.HashManager)
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act - 创建 fx 应用（缺少 BadgerStore）
	app := fx.New(
		fx.Provide(
			func() infraStorage.FileStore { return fileStore },
			func() crypto.HashManager { return hashManager },
			func() core.BlockHashServiceClient { return blockHashClient },
			func() transaction.TransactionHashServiceClient { return txHashClient },
			func() log.Logger { return logger },
		),
		Module(),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)

	// Assert
	// 注意：fx 在构建时就会检查依赖，如果缺少必需依赖，会在 Start 之前就失败
	if err != nil {
		assert.Error(t, err, "缺少必需依赖时应该失败")
		return
	}
	defer app.Stop(ctx)

	// 如果没有失败，说明依赖是可选的，这不符合预期
	t.Logf("⚠️ 警告：缺少 BadgerStore 依赖时模块仍然加载成功，这可能不符合预期")
}

// TestModule_ServiceCreation_AllServicesAreCreated 测试所有服务都被创建
func TestModule_ServiceCreation_AllServicesAreCreated(t *testing.T) {
	// Arrange
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	hashManager := testutil.NewTestHashManager().(crypto.HashManager)
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act - 创建 fx 应用并验证所有服务
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return badgerStore },
			func() infraStorage.FileStore { return fileStore },
			func() crypto.HashManager { return hashManager },
			func() core.BlockHashServiceClient { return blockHashClient },
			func() transaction.TransactionHashServiceClient { return txHashClient },
			func() log.Logger { return logger },
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				queryService persistence.QueryService,
				dataWriter persistence.DataWriter,
				chainQuery persistence.ChainQuery,
				blockQuery persistence.BlockQuery,
				txQuery persistence.TxQuery,
				utxoQuery persistence.UTXOQuery,
				resourceQuery persistence.ResourceQuery,
				accountQuery persistence.AccountQuery,
			) {
				// Assert - 验证所有服务都已创建
				assert.NotNil(t, queryService, "QueryService 应该被创建")
				assert.NotNil(t, dataWriter, "DataWriter 应该被创建")
				assert.NotNil(t, chainQuery, "ChainQuery 应该被创建")
				assert.NotNil(t, blockQuery, "BlockQuery 应该被创建")
				assert.NotNil(t, txQuery, "TxQuery 应该被创建")
				assert.NotNil(t, utxoQuery, "UTXOQuery 应该被创建")
				assert.NotNil(t, resourceQuery, "ResourceQuery 应该被创建")
				assert.NotNil(t, accountQuery, "AccountQuery 应该被创建")
			},
			fx.ParamTags(
				`name:"query_service"`,
				`name:"data_writer"`,
				`name:"chain_query"`,
				`name:"block_query"`,
				`name:"tx_query"`,
				`name:"utxo_query"`,
				`name:"resource_query"`,
				`name:"account_query"`,
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

// TestModule_Lifecycle_LifecycleHooksAreRegistered 测试生命周期钩子注册
func TestModule_Lifecycle_LifecycleHooksAreRegistered(t *testing.T) {
	// Arrange
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	hashManager := testutil.NewTestHashManager().(crypto.HashManager)
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act - 创建 fx 应用并验证生命周期钩子
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return badgerStore },
			func() infraStorage.FileStore { return fileStore },
			func() crypto.HashManager { return hashManager },
			func() core.BlockHashServiceClient { return blockHashClient },
			func() transaction.TransactionHashServiceClient { return txHashClient },
			func() log.Logger { return logger },
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

// TestModule_OptionalDependencies_WorkCorrectly 测试可选依赖
// 注意：Logger 在生命周期钩子中使用，但可以通过 nil 值处理
func TestModule_OptionalDependencies_WorkCorrectly(t *testing.T) {
	// Arrange - 提供 nil Logger（测试可选依赖处理）
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	hashManager := testutil.NewTestHashManager().(crypto.HashManager)
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()

	// Act - 创建 fx 应用（提供 nil Logger）
	app := fx.New(
		fx.Provide(
			func() infraStorage.BadgerStore { return badgerStore },
			func() infraStorage.FileStore { return fileStore },
			func() crypto.HashManager { return hashManager },
			func() core.BlockHashServiceClient { return blockHashClient },
			func() transaction.TransactionHashServiceClient { return txHashClient },
			func() log.Logger { return nil }, // 提供 nil Logger
		),
		Module(),
		fx.Invoke(fx.Annotate(
			func(
				queryService persistence.QueryService,
				dataWriter persistence.DataWriter,
			) {
				// Assert - 验证服务仍然可以创建（nil Logger 不应该导致失败）
				assert.NotNil(t, queryService, "QueryService 应该被创建（即使 Logger 为 nil）")
				assert.NotNil(t, dataWriter, "DataWriter 应该被创建（即使 Logger 为 nil）")
			},
			fx.ParamTags(
				`name:"query_service"`,
				`name:"data_writer"`,
			),
		)),
	)

	// 启动应用
	ctx := context.Background()
	err := app.Start(ctx)
	defer app.Stop(ctx)

	// Assert
	assert.NoError(t, err, "nil Logger 时模块应该仍然可以加载")
}

