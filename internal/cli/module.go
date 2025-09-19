// Package cli 提供WES系统的命令行交互界面
//
// 📋 **CLI交互模块 (Command Line Interface Module)**
//
// 本包实现了WES区块链系统的交互式命令行界面，提供：
// - 可视化交互菜单和仪表盘
// - 区块链操作命令（余额查询、转账、挖矿等）
// - 多种运行模式支持（交互模式、单命令模式）
//
// 🎯 **模块职责**：
// - 提供用户友好的CLI交互界面
// - 封装HTTP API调用为命令行操作
// - 实现实时状态监控和可视化显示
// - 协调各种CLI组件的依赖关系
//
// 🏗️ **架构特点**：
// - 模块化设计：client、commands、interactive、ui等子模块
// - 依赖注入：通过fx框架管理组件生命周期
// - API封装：复用现有HTTP API，避免重复实现
// - 可扩展性：支持新增命令和交互方式
package cli

import (
	"context"
	"path/filepath"

	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/commands"
	"github.com/weisyn/v1/internal/cli/interactive"
	"github.com/weisyn/v1/internal/cli/manager"
	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/internal/cli/status"
	clipkg "github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/internal/cli/wallet"

	// 基础服务接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 区块链核心服务接口
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	repositoryintf "github.com/weisyn/v1/pkg/interfaces/repository"
)

// CLIApp CLI应用接口，供外部应用层使用
type CLIApp interface {
	// Run 运行CLI应用
	Run(ctx context.Context) error
}

// cliAppImpl CLIApp接口的内部实现
type cliAppImpl struct {
	controller *manager.Controller
}

// Run 实现CLIApp接口
func (c *cliAppImpl) Run(ctx context.Context) error {
	return c.controller.Run(ctx)
}

// Module 创建并配置CLI模块
//
// 🎯 **模块构建器**：
// 本函数是CLI模块的主要入口点，负责构建完整的fx模块配置。
// 通过fx.Module组织所有CLI子组件的依赖注入配置，确保组件的正确创建和协调。
//
// 🏗️ **构建流程**：
// 1. 创建API客户端：封装HTTP API调用
// 2. 创建UI组件：提供终端界面美化功能
// 3. 创建命令处理器：实现各种业务命令
// 4. 创建交互界面：提供菜单和仪表盘
// 5. 创建控制器：协调所有组件
//
// 📋 **服务创建顺序**：
// - Client: HTTP API客户端（底层服务）
// - UI: 界面组件（通用服务）
// - Commands: 各种命令处理器（业务逻辑）
// - Interactive: 交互式组件（用户界面）
// - Controller: 主控制器（顶层协调）
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    cli.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保以下依赖可用：
// - log模块：提供日志记录服务
// - HTTP API服务：提供数据访问接口
func Module() fx.Option {
	return fx.Module("cli",
		// CLI模块依赖注入配置
		// 按照服务层次顺序构建：底层服务 → 业务逻辑 → 用户界面 → 控制器

		// API客户端（底层服务）
		fx.Provide(
			fx.Annotate(
				func(logger log.Logger, configProvider config.Provider) *client.Client {
					return client.NewClient(logger, configProvider)
				},
			),
		),

		// UI组件（通用服务）
		fx.Provide(clipkg.NewComponents),

		// 钱包管理器（CLI内部）
		fx.Provide(
			fx.Annotate(
				func(logger log.Logger, configProvider config.Provider, addressManager cryptointf.AddressManager) wallet.WalletManager {
					// 从配置中获取存储路径
					cliOptions := configProvider.GetCLI()
					storageOptions := configProvider.GetBadger() // 使用BadgerDB的存储路径作为基础路径

					// 解析钱包存储路径：基础路径 + 钱包子目录
					var walletStoragePath string
					if filepath.IsAbs(cliOptions.WalletStoragePath) {
						// 如果是绝对路径，直接使用
						walletStoragePath = cliOptions.WalletStoragePath
					} else {
						// 如果是相对路径，基于存储基础路径解析
						walletStoragePath = filepath.Join(storageOptions.Path, cliOptions.WalletStoragePath)
					}

					return wallet.NewWalletManager(logger, walletStoragePath, addressManager)
				},
				fx.As(new(wallet.WalletManager)),
			),
		),

		// 权限管理器（CLI内部）
		fx.Provide(permissions.NewManager),

		// 状态管理器（CLI内部）
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					chainService blockchainintf.ChainService,
					minerService consensusintf.MinerService,
					configProvider config.Provider,
					apiClient *client.Client,
				) *status.StatusManager {
					return status.NewStatusManager(logger, chainService, minerService, configProvider, apiClient)
				},
				fx.ParamTags(``, ``, `name:"consensus_miner_service" optional:"true"`, ``, ``),
			),
		),

		// 命令处理器（业务逻辑层）
		// 使用fx.Annotate为命令注入核心区块链服务
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					apiClient *client.Client,
					ui clipkg.Components,
					accountService blockchainintf.AccountService,
					keyManager cryptointf.KeyManager,
					addressManager cryptointf.AddressManager,
					signatureManager cryptointf.SignatureManager,
					walletManager wallet.WalletManager,
				) *commands.AccountCommands {
					return commands.NewAccountCommands(logger, apiClient, ui, accountService, keyManager, addressManager, signatureManager, walletManager)
				},
			),
		),
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					apiClient *client.Client,
					ui clipkg.Components,
					transactionService blockchainintf.TransactionService,
					transactionManager blockchainintf.TransactionManager,
					addressManager cryptointf.AddressManager,
					signatureManager cryptointf.SignatureManager,
					walletManager wallet.WalletManager,
				) *commands.TransferCommands {
					return commands.NewTransferCommands(logger, apiClient, ui, transactionService, transactionManager, addressManager, signatureManager, walletManager)
				},
			),
		),
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					apiClient *client.Client,
					ui clipkg.Components,
					chainService blockchainintf.ChainService,
					blockService blockchainintf.BlockService,
					repositoryManager repositoryintf.RepositoryManager,
				) *commands.BlockchainCommands {
					return commands.NewBlockchainCommands(logger, apiClient, ui, chainService, blockService, repositoryManager)
				},
			),
		),
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					apiClient *client.Client,
					ui clipkg.Components,
					minerService consensusintf.MinerService,
					chainService blockchainintf.ChainService,
					addressManager cryptointf.AddressManager,
					walletManager wallet.WalletManager,
				) *commands.MiningCommands {
					return commands.NewMiningCommands(logger, apiClient, ui, minerService, chainService, addressManager, walletManager)
				},
				fx.ParamTags(``, ``, ``, `name:"consensus_miner_service"`, ``, ``, ``),
			),
		),
		fx.Provide(commands.NewNodeCommands), // 节点命令处理器（基础实现）

		// 交互式界面（用户界面层）
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					ui clipkg.Components,
					account *commands.AccountCommands,
					transfer *commands.TransferCommands,
					blockchain *commands.BlockchainCommands,
					mining *commands.MiningCommands,
					node *commands.NodeCommands,
					statusManager *status.StatusManager,
				) *interactive.Menu {
					// 注入全局状态栏渲染器，供ShowPageHeader使用
					clipkg.SetStatusManager(statusManager)
					return interactive.NewMenu(logger, ui, account, transfer, blockchain, mining, node, statusManager)
				},
			),
		),
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					apiClient *client.Client,
					ui clipkg.Components,
					chainService blockchainintf.ChainService,
					accountService blockchainintf.AccountService,
					minerService consensusintf.MinerService,
					configProvider config.Provider,
					statusManager *status.StatusManager,
				) *interactive.Dashboard {
					return interactive.NewDashboard(logger, apiClient, ui, chainService, accountService, minerService, configProvider, statusManager)
				},
				fx.ParamTags(``, ``, ``, ``, ``, `name:"consensus_miner_service" optional:"true"`, ``, ``),
			),
		),

		// 控制器（顶层协调）
		fx.Provide(
			fx.Annotate(
				func(
					logger log.Logger,
					statusManager *status.StatusManager,
					menu *interactive.Menu,
					dashboard *interactive.Dashboard,
					account *commands.AccountCommands,
					transfer *commands.TransferCommands,
					blockchain *commands.BlockchainCommands,
					mining *commands.MiningCommands,
					node *commands.NodeCommands,
					accountService blockchainintf.AccountService,
					permissionManager *permissions.Manager,
					uiComponents clipkg.Components,
				) *manager.Controller {
					return manager.NewController(
						logger,
						statusManager,
						menu,
						dashboard,
						account,
						transfer,
						blockchain,
						mining,
						node,
						accountService,
						permissionManager,
						uiComponents,
					)
				},
			),
		),

		// CLIApp接口实现
		fx.Provide(
			fx.Annotate(
				func(controller *manager.Controller) CLIApp { return &cliAppImpl{controller: controller} },
				fx.As(new(CLIApp)),
			),
		),

		// 应用层 - CLI生命周期钩子
		fx.Invoke(func(lifecycle fx.Lifecycle, controller *manager.Controller, logger log.Logger) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// 启动CLI应用
					go func() {
						_ = controller.Run(ctx)
					}()
					// CLI启动完成
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return nil
				},
			})
		}),
	)
}
