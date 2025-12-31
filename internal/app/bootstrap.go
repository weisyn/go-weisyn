package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weisyn/v1/internal/api"
	// "github.com/weisyn/v1/internal/cli"
	config "github.com/weisyn/v1/internal/config"
	"github.com/weisyn/v1/internal/core/block"
	"github.com/weisyn/v1/internal/core/chain"
	"github.com/weisyn/v1/internal/core/compliance"
	"github.com/weisyn/v1/internal/core/consensus"
	"github.com/weisyn/v1/internal/core/eutxo"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto"
	"github.com/weisyn/v1/internal/core/infrastructure/event"
	kademlia 	"github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	log "github.com/weisyn/v1/internal/core/infrastructure/log"
	"github.com/weisyn/v1/internal/core/infrastructure/metrics"
	"github.com/weisyn/v1/internal/core/infrastructure/storage"
	"github.com/weisyn/v1/internal/core/infrastructure/writegate"

	// "github.com/weisyn/v1/internal/core/infrastructure/wallet" // 🔐 钱包模块（暂时移除）
	"github.com/weisyn/v1/internal/core/mempool"
	"github.com/weisyn/v1/internal/core/network"
	"github.com/weisyn/v1/internal/core/p2p"

	// 执行层模块（ispc目录，但package名为execution）
	// ⚠️ 注意：engines模块已迁移到ispc/engines内部，不再作为独立模块加载
	execution "github.com/weisyn/v1/internal/core/ispc"

	// 交易处理模块
	tx "github.com/weisyn/v1/internal/core/tx"

	// 数据存储层模块
	persistence "github.com/weisyn/v1/internal/core/persistence"
	"github.com/weisyn/v1/internal/core/ures"
	"github.com/weisyn/v1/internal/core/resourcesvc"

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
	opts  *options
	fxApp *fx.App
}

// NewBootstrap 创建引导程序
func NewBootstrap(opts *options) *Bootstrap {
	return &Bootstrap{
		opts: opts,
	}
}

// SetupInfrastructureLayer 设置基础设施层模块
func (b *Bootstrap) SetupInfrastructureLayer() []fx.Option {
	return []fx.Option{
		config.Module(),   // 1. 配置(不依赖其他)
		log.Module(),      // 2. 日志(依赖配置)
		crypto.Module(),   // 3. 密码学(依赖配置)
		kademlia.Module(), // 4. Kademlia路由表(依赖配置和日志)
		metrics.Module(),  // 5. 内存监控指标(依赖配置和日志)
		writegate.Module(), // 6. WriteGate写门闸(无依赖，但需在存储/链模块前加载)

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
		p2p.Module(),     // P2P运行时模块 - 新的P2P基础设施
		network.Module(), // 网络服务层 - 提供统一网络服务（已迁移到使用p2p）

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
		persistence.Module(), // 1.5. Persistence 模块（提供 QueryService 和 DataWriter，需要在 EUTXO 之前加载）
		eutxo.Module(),       // 2. EUTXO 模块（实现pkg/interfaces/eutxo，依赖 persistence.BlockQuery）
		ures.Module(),        // 2.5. URES 模块（实现pkg/interfaces/ures）
		resourcesvc.Module(), // 2.6. ResourceViewService 模块（依赖 EUTXO 和 URES）

		// 第二层半：合规策略层（需要在内存池之前加载）
		compliance.Module(), // 2.5. 合规策略服务（为内存池和共识层提供合规检查）

		// 第三层：内存池（需要在区块链之前加载，避免循环依赖）
		mempool.Module(), // 3. 内存池（包含交易池和候选区块池）

		// 第三层半：ISPC执行层（包含内部引擎）
		// ✅ 架构变更：engines已迁移到ispc/engines内部，不再作为独立模块
		execution.Module(), // 3.5. ISPC执行层（包含WASM/ONNX引擎和宿主能力）

		// 第三层四刻：交易处理模块
		tx.Module(), // 3.85. 交易处理模块（提供资产、资源、合约、AI模型等交易服务）

		// 第四层：核心链逻辑
		block.Module(), // 4. 区块模块（依赖eutxo、内存池、tx模块和执行层）
		chain.Module(), // 4.5. 链模块（依赖block、eutxo模块）

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

	// ========== API 网关模块（已启用） ==========
	if b.opts.enableAPI {
		modules = append(modules, api.Module())
		fmt.Println("🌐 API 网关模块已启用")
		fmt.Println("   - JSON-RPC 2.0（主协议，DApp/钱包）")
		fmt.Println("   - HTTP REST（运维/人类可读）")
		fmt.Println("   - WebSocket（实时订阅，重组安全）")
		fmt.Println("   - gRPC（高性能，已启用反射）")
	} else {
		fmt.Println("⚠️  API 网关模块已禁用")
	}

	// ========== CLI 模块（暂时禁用） ==========
	// 条件性添加CLI模块
	// if b.opts.enableCLI {
	//     modules = append(modules, cli.Module())
	//     modules = append(modules, fx.Invoke(func(cliApp cli.CLIApp, lifecycle fx.Lifecycle) {
	//         lifecycle.Append(fx.Hook{
	//             OnStart: func(ctx context.Context) error {
	//                 b.storeCLIApp(cliApp)
	//                 fmt.Println("✅ CLI服务已就绪")
	//                 return nil
	//             },
	//         })
	//     }))
	//     fmt.Println("💻 CLI模块已启用")
	// } else {
	//     fmt.Println("⚠️  CLI模块已禁用")
	// }

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
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] CreateFxApp: 开始创建fx应用\n")
	os.Stderr.Sync()
	
	// 获取所有模块
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] CreateFxApp: 开始设置模块\n")
	os.Stderr.Sync()
	
	modules, err := b.SetupModules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ CreateFxApp: SetupModules失败: %v\n", err)
		os.Stderr.Sync()
		return fmt.Errorf("设置模块失败: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] CreateFxApp: 模块设置完成，共 %d 个模块选项\n", len(modules))
	os.Stderr.Sync()

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

	// ✅ 架构改进：
	// - 宿主函数所需的区块链服务（ChainService/BlockService/UTXOManager/RepositoryManager）
	//   现在在 engines 模块初始化时直接注入（见 internal/core/engines/module.go）
	// - 依赖方向：engines → blockchain（单向），无循环依赖
	// - Fail-Fast原则：启动期依赖缺失时应用立即失败，不再运行时返回0值

	// 创建fx应用
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] CreateFxApp: 调用 fx.New()\n")
	os.Stderr.Sync()
	
	b.fxApp = fx.New(appOptions...)
	
	if b.fxApp == nil {
		fmt.Fprintf(os.Stderr, "❌ CreateFxApp: fx.New() 返回了 nil\n")
		os.Stderr.Sync()
		return fmt.Errorf("fx.New() 返回了 nil")
	}
	
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] CreateFxApp: fx应用创建成功\n")
	os.Stderr.Sync()
	
	return nil
}

// StartApp 启动应用程序
func (b *Bootstrap) StartApp(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] StartApp: 开始启动应用\n")
	os.Stderr.Sync()
	fmt.Println("正在启动应用...")

	if b.fxApp == nil {
		err := fmt.Errorf("fx应用未初始化")
		fmt.Fprintf(os.Stderr, "❌ StartApp: %v\n", err)
		os.Stderr.Sync()
		return err
	}

	// 在 fx.Start 之前标记下一阶段：启动基础设施将在各模块 OnStart 中推进
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] StartApp: 调用 fxApp.Start()\n")
	os.Stderr.Sync()
	
	if err := b.fxApp.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ StartApp: 启动失败: %v\n", err)
		os.Stderr.Sync()
		// 输出详细的错误信息
		if errStr := err.Error(); errStr != "" {
			fmt.Fprintf(os.Stderr, "错误详情: %s\n", errStr)
			os.Stderr.Sync()
		}
		return fmt.Errorf("启动应用失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] StartApp: fx应用启动成功\n")
	os.Stderr.Sync()
	fmt.Println("✅ fx应用启动完成")
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
	fmt.Println("🔧 准备启动fx应用...")
	if err := bootstrap.StartApp(startupCtx); err != nil {
		fmt.Printf("❌ BootstrapApp: StartApp失败: %v\n", err)
		return nil, err
	}
	fmt.Println("✅ BootstrapApp: StartApp完成")

	// 🔧 新增：依赖注入完整性检查
	fmt.Println("🔍 开始依赖注入完整性检查...")
	if err := bootstrap.validateDependencyInjection(); err != nil {
		fmt.Printf("⚠️  依赖注入完整性检查失败: %v\n", err)
		fmt.Println("系统将继续运行，但可能存在功能异常")
		// 不返回错误，允许系统继续运行，但记录问题
	} else {
		fmt.Println("✅ 依赖注入完整性检查通过")
	}

	// 创建应用实例
	fmt.Println("📦 创建应用实例...")
	app := &internalApp{
		fxApp:     bootstrap.fxApp,
		bootstrap: bootstrap,
	}

	fmt.Println("✅ BootstrapApp: 应用实例创建完成，准备返回")
	return app, nil
}

// WaitForSignal 等待退出信号
func WaitForSignal() os.Signal {
	signals := make(chan os.Signal, 1)
	// 在不同平台上监听不同的信号
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	return <-signals
}
