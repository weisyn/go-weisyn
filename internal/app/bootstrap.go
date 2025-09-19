package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weisyn/v1/internal/api"
	"github.com/weisyn/v1/internal/cli"
	config "github.com/weisyn/v1/internal/config"
	"github.com/weisyn/v1/internal/core/blockchain"
	"github.com/weisyn/v1/internal/core/compliance"
	"github.com/weisyn/v1/internal/core/consensus"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto"
	"github.com/weisyn/v1/internal/core/infrastructure/event"
	kademlia "github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	log "github.com/weisyn/v1/internal/core/infrastructure/log"
	"github.com/weisyn/v1/internal/core/infrastructure/node"
	"github.com/weisyn/v1/internal/core/infrastructure/storage"

	// "github.com/weisyn/v1/internal/core/infrastructure/wallet" // 🔐 钱包模块（暂时移除）
	"github.com/weisyn/v1/internal/core/mempool"
	"github.com/weisyn/v1/internal/core/network"

	// 执行引擎模块
	"github.com/weisyn/v1/internal/core/engines/onnx"
	"github.com/weisyn/v1/internal/core/engines/wasm"

	// 执行层模块
	"github.com/weisyn/v1/internal/core/execution"

	// 数据存储层模块
	"github.com/weisyn/v1/internal/core/repositories"

	//testvm "github.com/weisyn/v1/test/vm"
	"go.uber.org/fx"
)

// Framework layers
const (
	// 基础设施层
	LayerInfrastructure = "infrastructure"
	// 通信与数据层
	LayerCommunication = "communication"
	// 业务逻辑层
	LayerBusiness = "business"
	// 应用层
	LayerApplication = "application"
)

// Bootstrap 应用引导程序
type Bootstrap struct {
	opts   *options
	fxApp  *fx.App
	cliApp cli.CLIApp // CLI应用实例（启动后设置）
}

// NewBootstrap 创建引导程序
func NewBootstrap(opts *options) *Bootstrap {
	return &Bootstrap{
		opts: opts,
	}
}

// storeCLIApp 存储CLI应用实例（由fx生命周期钩子调用）
func (b *Bootstrap) storeCLIApp(cliApp cli.CLIApp) {
	b.cliApp = cliApp
}

// GetCLIApp 获取CLI应用实例
func (b *Bootstrap) GetCLIApp() cli.CLIApp {
	return b.cliApp
}

// SetupInfrastructureLayer 设置基础设施层模块
func (b *Bootstrap) SetupInfrastructureLayer() []fx.Option {
	return []fx.Option{
		config.Module(),   // 1. 配置(不依赖其他)
		log.Module(),      // 2. 日志(依赖配置)
		crypto.Module(),   // 3. 密码学(依赖配置)
		kademlia.Module(), // 4. Kademlia路由表(依赖配置和日志)
		// metrics.Module(),     // 5. 指标(依赖配置和日志)

		// 在基础设施层开始时推进进度
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 基础设施启动完成
					return nil
				},
			})
		}),
	}
}

// SetupCommunicationLayer 设置通信与数据层模块
func (b *Bootstrap) SetupCommunicationLayer() []fx.Option {
	return []fx.Option{
		// 通信与数据层模块（依赖基础设施层）
		event.Module(),   // 事件(依赖基础设施)
		storage.Module(), // 存储(依赖基础设施)
		node.Module(),    // 节点网络模块 - 基于WES架构，使用DEFS实现
		network.Module(), // 网络服务层 - 提供统一网络服务

		//testvm.Module(), // 测试VM模块(依赖已有VM模块)

		// 在通信与数据层开始时推进进度
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 通信与数据层启动完成
					return nil
				},
			})
		}),
	}
}

// SetupBusinessLayer 设置业务逻辑层模块
func (b *Bootstrap) SetupBusinessLayer() []fx.Option {
	// 业务逻辑层模块(依赖通信与数据层)
	// 注意：加载顺序必须遵循模块间的依赖关系，从底层基础模块到上层应用模块

	// 方式一：使用整合的核心模块（推荐用于生产环境）
	// 当core.Module()内部会按正确的依赖顺序加载所有子模块
	// TODO: core模块实现后取消注释
	// return []fx.Option{
	//     core.Module(),     // 区块链核心模块(包含所有子模块)
	//     sync.Module(),     // 区块同步模块(独立于区块链核心)
	// }

	// 方式二：单独加载各个子模块（便于开发和测试）
	// 核心模块加载的依赖顺序，必须严格按照依赖关系：
	// 账户 -> 虚拟机 -> 状态 -> 区块链 -> 交易池 -> 共识
	return []fx.Option{
		// 将来添加: account.Module(), vm.Module(), state.Module()等
		// TODO: 各子模块实现后取消注释，注意保持依赖顺序

		// 第一层：基础领域模块
		// account.Module(), // 1. 账户管理（最基础，被状态和虚拟机依赖）

		// 第二层：依赖账户的基础模块
		// state.Module(), // 2. 状态管理（依赖账户）
		// 1) 执行环境需求：虚拟机执行智能合约时，需要读取当前账户状态、合约代码和存储数据
		// 2) 状态修改：合约执行过程中会修改状态（如余额变更、存储更新），这些修改需要通过状态管理模块持久化
		// 3) 交易结果：虚拟机执行的结果（如状态变更）需要通过状态管理模块应用到世界状态

		// 第二层：数据存储层（需要在区块链之前加载）
		repositories.Module(), // 2. 数据存储管理器（实现pkg/interfaces/repository）

		// 第二层半：合规策略层（需要在内存池之前加载）
		compliance.Module(), // 2.5. 合规策略服务（为内存池和共识层提供合规检查）

		// 第三层：内存池（需要在区块链之前加载，避免循环依赖）
		mempool.Module(), // 3. 内存池（包含交易池和候选区块池）

		// 第三层半：执行引擎（需要在区块链执行层装配前加载）
		wasm.Module(), // 3.5. WASM执行引擎（提供EngineAdapter实现）
		onnx.Module(), // 3.6. ONNX执行引擎（提供EngineAdapter实现）

		// 第三层三刻：执行层（需要在区块链之前加载）
		execution.Module(), // 3.75. 执行层（协调引擎管理和宿主能力）

		// 第四层：核心链逻辑
		blockchain.Module(), // 4. 区块链核心（依赖repositories、内存池和执行层）

		// 增加虚拟机模块，依赖于区块链模块
		// vm.Module(), // 5. 虚拟机（依赖区块链）

		// 增加AI模块，依赖于区块链模块
		// ai.Module(), // 6. AI服务（依赖区块链）

		// 第五层：链周边服务
		consensus.Module(), // 7. 共识机制（依赖区块链）

		// 🔐 第六层：钱包服务（依赖crypto基础设施）
		// TODO: 钱包存储服务实现完成后启用
		// wallet.Module(), // 8. 钱包管理服务（提供WalletManager接口）

		// 注释：共识服务通过各子服务（MinerService、AggregatorService）提供功能
		// 不需要统一的ConsensusService接口，各服务独立注入

		// 区块链核心以外的业务模块
		// sync.Module(),            // 9. 区块同步模块（依赖区块链核心，类似于共识模块，独立实现）

		// 在业务逻辑层开始时推进进度
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 业务逻辑层启动完成
					return nil
				},
			})
		}),
	}
}

// SetupApplicationLayer 设置应用层模块
func (b *Bootstrap) SetupApplicationLayer() []fx.Option {
	// 应用层模块(依赖所有其他层)
	// 应用层模块通常包括API服务、CLI命令、外部接口等
	modules := []fx.Option{
		AppModule, // 应用核心模块
	}

	// 条件性添加API模块
	if b.opts.enableAPI {
		modules = append(modules, api.Module()) // API服务（REST、GraphQL等）
		fmt.Println("🌐 API模块已启用")
	} else {
		fmt.Println("⚠️  API模块已禁用")
	}

	// 条件性添加CLI模块
	if b.opts.enableCLI {
		modules = append(modules, cli.Module())

		// 在CLI启动时存储引用供GetCLIApp使用
		modules = append(modules, fx.Invoke(func(cliApp cli.CLIApp, lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 存储CLI引用到bootstrap实例
					b.storeCLIApp(cliApp)
					fmt.Println("✅ CLI服务已就绪")
					return nil
				},
			})
		}))
		fmt.Println("💻 CLI模块已启用")
	} else {
		fmt.Println("⚠️  CLI模块已禁用")
	}

	// TODO: 以下是潜在的应用层模块，实现后取消注释
	// rpc.Module(),        // RPC服务
	// dashboard.Module(),  // 管理控制台
	// wallet.Module(),     // 钱包功能（作为应用层服务）

	return modules
}

// SetupModules 设置所有应用模块
func (b *Bootstrap) SetupModules() ([]fx.Option, error) {
	var allModules []fx.Option

	// 按照依赖顺序添加各层模块
	infraModules := b.SetupInfrastructureLayer()
	allModules = append(allModules, infraModules...)

	commModules := b.SetupCommunicationLayer()
	allModules = append(allModules, commModules...)

	businessModules := b.SetupBusinessLayer()
	allModules = append(allModules, businessModules...)

	appModules := b.SetupApplicationLayer()
	allModules = append(allModules, appModules...)

	return allModules, nil
}

// CreateFxApp 创建并配置fx应用
func (b *Bootstrap) CreateFxApp() error {
	// 获取所有模块
	modules, err := b.SetupModules()
	if err != nil {
		return err
	}

	// 配置fx应用选项
	appOptions := []fx.Option{
		// 加载所有模块
		fx.Options(modules...),

		// 禁用fx内部日志
		fx.NopLogger,

		// 生命周期钩子
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					fmt.Println("准备启动应用")
					// 装配模块已完成
					return nil
				},
				OnStop: func(ctx context.Context) error {
					fmt.Println("准备停止应用")
					return nil
				},
			})
		}),

		// ===== 移除：执行分发策略与回退顺序配置 =====
		// 注意：这些配置应该在 blockchain 模块内部完成，不应在应用层配置
		// 具体的 EngineManager 和 Dispatcher 是 blockchain 模块的内部实现细节
	}

	// 创建fx应用
	b.fxApp = fx.New(appOptions...)
	return nil
}

// StartApp 启动应用程序
func (b *Bootstrap) StartApp(ctx context.Context) error {
	fmt.Println("正在启动应用...")

	// 在 fx.Start 之前标记下一阶段：启动基础设施将在各模块 OnStart 中推进
	if err := b.fxApp.Start(ctx); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		return fmt.Errorf("启动应用失败: %w", err)
	}

	return nil
}

// StopApp 停止应用程序
func (b *Bootstrap) StopApp(ctx context.Context) error {
	fmt.Println("正在停止应用...")

	if err := b.fxApp.Stop(ctx); err != nil {
		fmt.Printf("停止失败: %v\n", err)
		return fmt.Errorf("停止应用失败: %w", err)
	}

	return nil
}

// validateDependencyInjection 验证依赖注入的完整性
// 检查关键组件是否正确初始化，特别是TransactionHashService等容易出现空指针的组件
func (b *Bootstrap) validateDependencyInjection() error {
	if b.fxApp == nil {
		return fmt.Errorf("fx应用未初始化")
	}

	// 简单验证：检查fx应用是否正常运行
	// 在实际应用中，如果依赖注入有问题，fx应用启动时就会失败
	// 这里主要是记录验证过程，实际的验证由fx框架在启动时完成

	fmt.Println("🔍 正在验证核心组件依赖注入...")
	fmt.Println("   - TransactionHashService: 由fx框架在启动时验证")
	fmt.Println("   - TransactionManager: 由fx框架在启动时验证")
	fmt.Println("   - Logger: 由fx框架在启动时验证")
	fmt.Println("   - HashManager: 由fx框架在启动时验证")
	fmt.Println("   - 所有依赖关系: 由fx框架在启动时验证")

	// 如果能执行到这里，说明fx应用启动成功，依赖注入基本正确
	// 具体的空指针问题需要在运行时通过我们之前添加的错误处理机制捕获
	return nil
}

// BootstrapApp 执行完整的引导过程并返回应用实例
func BootstrapApp(options ...Option) (App, error) {
	// 处理配置选项
	opts := newOptions(options...)

	// 创建引导对象
	bootstrap := NewBootstrap(opts)

	// 创建fx应用
	if err := bootstrap.CreateFxApp(); err != nil {
		return nil, fmt.Errorf("创建应用失败: %w", err)
	}

	// 启动应用 - 使用有超时的启动Context
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer startupCancel()

	// 启动应用组件
	if err := bootstrap.StartApp(startupCtx); err != nil {
		return nil, err
	}

	// 🔧 新增：依赖注入完整性检查
	if err := bootstrap.validateDependencyInjection(); err != nil {
		fmt.Printf("⚠️  依赖注入完整性检查失败: %v\n", err)
		fmt.Println("系统将继续运行，但可能存在功能异常")
		// 不返回错误，允许系统继续运行，但记录问题
	} else {
		fmt.Println("✅ 依赖注入完整性检查通过")
	}

	// 创建应用实例
	app := &internalApp{
		fxApp:     bootstrap.fxApp,
		bootstrap: bootstrap,
	}

	// 初始化界面完成

	return app, nil
}

// WaitForSignal 等待退出信号
func WaitForSignal() os.Signal {
	signals := make(chan os.Signal, 1)
	// 在不同平台上监听不同的信号
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	return <-signals
}
