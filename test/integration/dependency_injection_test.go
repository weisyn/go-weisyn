package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	// 导入所有模块

	"github.com/weisyn/v1/internal/api"
	"github.com/weisyn/v1/internal/config"
	"github.com/weisyn/v1/internal/core/blockchain"
	"github.com/weisyn/v1/internal/core/compliance"
	"github.com/weisyn/v1/internal/core/consensus"
	"github.com/weisyn/v1/internal/core/engines/onnx"
	"github.com/weisyn/v1/internal/core/engines/wasm"
	"github.com/weisyn/v1/internal/core/execution"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto"
	"github.com/weisyn/v1/internal/core/infrastructure/event"
	kademlia "github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	"github.com/weisyn/v1/internal/core/infrastructure/log"
	"github.com/weisyn/v1/internal/core/infrastructure/node"
	"github.com/weisyn/v1/internal/core/infrastructure/storage"
	"github.com/weisyn/v1/internal/core/mempool"
	"github.com/weisyn/v1/internal/core/network"
	"github.com/weisyn/v1/internal/core/repositories"

	// 接口导入用于验证
	blockchainIface "github.com/weisyn/v1/pkg/interfaces/blockchain"
	consensusIface "github.com/weisyn/v1/pkg/interfaces/consensus"
	cryptoIface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventIface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	kademliaIface "github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	logIface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storageIface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	networkIface "github.com/weisyn/v1/pkg/interfaces/network"
	repositoryIface "github.com/weisyn/v1/pkg/interfaces/repository"
)

// DependencyTestTarget 依赖注入测试目标结构
type DependencyTestTarget struct {
	fx.In

	// 测试关键的命名依赖是否正确注入
	EnhancedEventBus       *event.EnhancedEventBus           `name:"enhanced_eventbus"`
	EventBus               eventIface.EventBus               `name:"eventbus"`
	RoutingTableManager    kademliaIface.RoutingTableManager `name:"routing_table_manager"`
	DistanceCalculator     kademliaIface.DistanceCalculator  `name:"distance_calculator"`
	ConsensusMinderService consensusIface.MinerService       `name:"consensus_miner_service"`

	// 测试必需的存储组件
	BadgerStore storageIface.BadgerStore
	FileStore   storageIface.FileStore

	// 测试其他关键组件
	Logger            logIface.Logger
	HashManager       cryptoIface.HashManager
	RepositoryManager repositoryIface.RepositoryManager
	UTXOManager       repositoryIface.UTXOManager
	ChainService      blockchainIface.ChainService
	NetworkService    networkIface.Network `name:"network_service"`
}

// TestDependencyInjectionIntegrity 测试依赖注入完整性
func TestDependencyInjectionIntegrity(t *testing.T) {
	// 创建测试应用
	app := fxtest.New(t,
		// 配置模块
		config.Module(),

		// 基础设施模块
		log.Module(),
		crypto.Module(),
		storage.Module(),
		event.Module(),
		node.Module(),
		kademlia.Module(),

		// 网络和数据层
		network.Module(),
		repositories.Module(),
		compliance.Module(),

		// 内存池
		mempool.Module(),

		// 执行引擎
		wasm.Module(),
		onnx.Module(),
		execution.Module(),

		// 业务层
		blockchain.Module(),
		consensus.Module(),

		// API层
		api.Module(),

		// 验证依赖注入
		fx.Invoke(func(target DependencyTestTarget) {
			// 测试增强事件总线命名注入
			assert.NotNil(t, target.EnhancedEventBus, "增强事件总线应该被正确注入")
			assert.NotNil(t, target.EventBus, "基础事件总线应该被正确注入")

			// 测试Kademlia组件命名注入
			assert.NotNil(t, target.RoutingTableManager, "路由表管理器应该被正确注入")
			assert.NotNil(t, target.DistanceCalculator, "距离计算器应该被正确注入")

			// 测试共识矿工服务命名注入
			assert.NotNil(t, target.ConsensusMinderService, "共识矿工服务应该被正确注入")

			// 测试存储组件（必需）
			assert.NotNil(t, target.BadgerStore, "BadgerDB存储应该被正确注入")
			assert.NotNil(t, target.FileStore, "文件存储应该被正确注入")

			// 测试其他关键组件
			assert.NotNil(t, target.Logger, "Logger应该被正确注入")
			assert.NotNil(t, target.HashManager, "哈希管理器应该被正确注入")
			assert.NotNil(t, target.RepositoryManager, "仓储管理器应该被正确注入")
			assert.NotNil(t, target.UTXOManager, "UTXO管理器应该被正确注入")
			assert.NotNil(t, target.ChainService, "链服务应该被正确注入")
			assert.NotNil(t, target.NetworkService, "网络服务应该被正确注入")
		}),

		// 启用详细日志以便调试
		fx.Logger(fxtest.NewTestPrinter(t)),
	)

	// 启动应用
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := app.Start(ctx)
	require.NoError(t, err, "应用应该能够成功启动")

	// 停止应用
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	err = app.Stop(stopCtx)
	require.NoError(t, err, "应用应该能够成功停止")
}

// TestOptionalLoggerSafety 测试可选Logger的安全性
func TestOptionalLoggerSafety(t *testing.T) {
	// 创建不包含log模块的应用来测试可选Logger处理
	app := fxtest.New(t,
		// 只包含基础配置
		config.Module(),

		// 测试crypto和network模块在没有Logger时的行为
		crypto.Module(),
		network.Module(),

		// 验证模块可以在没有Logger时正常工作
		fx.Invoke(func(hashManager cryptoIface.HashManager) {
			assert.NotNil(t, hashManager, "即使没有Logger，crypto模块也应该能正常工作")
		}),

		// 禁用日志以测试no-op logger
		fx.NopLogger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := app.Start(ctx)
	require.NoError(t, err, "应用应该能够在没有Logger的情况下启动")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = app.Stop(stopCtx)
	require.NoError(t, err, "应用应该能够在没有Logger的情况下停止")
}

// TestNamingConsistency 测试命名一致性
func TestNamingConsistency(t *testing.T) {
	type NamingTestTarget struct {
		fx.In

		// 测试所有命名依赖都使用snake_case风格
		EnhancedEventBus       *event.EnhancedEventBus           `name:"enhanced_eventbus"`
		EventBus               eventIface.EventBus               `name:"eventbus"`
		DomainRegistry         *event.DomainRegistry             `name:"domain_registry"`
		EventRouter            *event.EventRouter                `name:"event_router"`
		EventValidator         event.EventValidator              `name:"event_validator"`
		EventCoordinator       event.EventCoordinator            `name:"event_coordinator"`
		RoutingTableManager    kademliaIface.RoutingTableManager `name:"routing_table_manager"`
		DistanceCalculator     kademliaIface.DistanceCalculator  `name:"distance_calculator"`
		PeerSelector           kademliaIface.PeerSelector        `name:"peer_selector"`
		NetworkService         networkIface.Network              `name:"network_service"`
		ConsensusMinderService consensusIface.MinerService       `name:"consensus_miner_service"`
	}

	app := fxtest.New(t,
		config.Module(),
		log.Module(),
		crypto.Module(),
		storage.Module(),
		event.Module(),
		node.Module(),
		kademlia.Module(),
		network.Module(),
		repositories.Module(),
		consensus.Module(),

		fx.Invoke(func(target NamingTestTarget) {
			// 验证所有命名依赖都能正确注入
			assert.NotNil(t, target.EnhancedEventBus, "enhanced_eventbus命名依赖应该正确")
			assert.NotNil(t, target.EventBus, "eventbus命名依赖应该正确")
			assert.NotNil(t, target.DomainRegistry, "domain_registry命名依赖应该正确")
			assert.NotNil(t, target.EventRouter, "event_router命名依赖应该正确")
			assert.NotNil(t, target.EventValidator, "event_validator命名依赖应该正确")
			assert.NotNil(t, target.EventCoordinator, "event_coordinator命名依赖应该正确")
			assert.NotNil(t, target.RoutingTableManager, "routing_table_manager命名依赖应该正确")
			assert.NotNil(t, target.DistanceCalculator, "distance_calculator命名依赖应该正确")
			assert.NotNil(t, target.PeerSelector, "peer_selector命名依赖应该正确")
			assert.NotNil(t, target.NetworkService, "network_service命名依赖应该正确")
			assert.NotNil(t, target.ConsensusMinderService, "consensus_miner_service命名依赖应该正确")
		}),

		fx.Logger(fxtest.NewTestPrinter(t)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := app.Start(ctx)
	require.NoError(t, err, "命名一致性测试应该通过")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = app.Stop(stopCtx)
	require.NoError(t, err, "应用应该能够正常停止")
}
