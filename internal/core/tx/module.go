// Package tx 提供 WES 系统的交易处理模块实现
//
// 📋 **WES 交易核心模块 (Transaction Core Module)**
//
// 本包基于 TX_STATE_MACHINE_ARCHITECTURE.md 架构设计，采用 Type-state + Verification Micro-kernel
// + Hexagonal Architecture 融合架构，实现类型安全的交易构建和插件化验证。
//
// 🎯 **核心理念**：TX = 权限验证 + 状态转换
//
// 🏗️ **架构特点**：
// - Type-state Pattern: 编译期防错，ComposedTx → ProvenTx → SignedTx → SubmittedTx
// - Verification Micro-kernel: 三钩子（AuthZ/Conservation/Condition）+ 插件系统
// - Hexagonal Architecture: 核心域 + 端口接口 + 适配器实现
// - 无业务语义: 底层只关心输入输出组合，业务语义由应用层解释
//
// 📦 **模块组织**：
// - interfaces/     - 内部接口（继承公共接口 + 内部扩展）
// - builder/        - TxBuilder 实现（纯装配器 + Type-state）
// - draft/          - DraftService 实现（渐进式构建）
// - processor/      - TxProcessor 实现（协调 Verifier + TxPool）
// - verifier/       - Verifier 微内核 + 插件系统
// - ports/          - 端口实现（signer/fee/proof/draftstore）
// - integration/    - 网络与事件集成
//
// 🔗 **依赖关系**：
// - repository.UTXOManager: UTXO 查询
// - mempool.TxPool: 交易池（验证后入池，自动广播）
// - 其他基础设施：log、storage、crypto 等
//
// 详细架构设计请参考：_dev/02-架构设计-architecture/tx/TX_STATE_MACHINE_ARCHITECTURE.md
// Package tx provides transaction processing functionality.
package tx

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"

	// 内部接口
	"github.com/weisyn/v1/internal/core/tx/interfaces"

	// 实现组件（按目录结构组织）
	processorPkg "github.com/weisyn/v1/internal/core/tx/processor"

	// P1 阶段实现
	"github.com/weisyn/v1/internal/core/tx/builder"
	"github.com/weisyn/v1/internal/core/tx/ports/fee"
	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	"github.com/weisyn/v1/internal/core/tx/ports/proof"
	"github.com/weisyn/v1/internal/core/tx/ports/signer"
	"github.com/weisyn/v1/internal/core/tx/verifier"

	// 基础设施组件
	"github.com/weisyn/v1/internal/core/tx/verifier/plugins/authz"
	"github.com/weisyn/v1/internal/core/tx/verifier/plugins/condition"
	"github.com/weisyn/v1/internal/core/tx/verifier/plugins/conservation"
	incentiveplugin "github.com/weisyn/v1/internal/core/tx/verifier/plugins/incentive" // 高优先级-3: 激励验证插件

	// P2 阶段实现
	"github.com/weisyn/v1/internal/core/tx/selector"

	// P2.5 阶段实现（规划器，协调 Selector 和 Builder）
	"github.com/weisyn/v1/internal/core/tx/planner"

	// P3 阶段实现
	draftstoreconfig "github.com/weisyn/v1/internal/config/tx/draftstore"
	"github.com/weisyn/v1/internal/core/tx/draft"
	"github.com/weisyn/v1/internal/core/tx/ports/draftstore"

	// P9 阶段实现（网络与事件集成）
	txEventIntegration "github.com/weisyn/v1/internal/core/tx/integration/event"
	txNetworkIntegration "github.com/weisyn/v1/internal/core/tx/integration/network"
	// P7+ 阶段实现（已启用）
	// Redis DraftStore 已集成到依赖注入系统
	// "github.com/weisyn/v1/internal/core/tx/ports/signer/hsm"
	// "github.com/weisyn/v1/internal/core/tx/ports/signer/kms"
)

// ==================== 模块输入依赖 ====================

// ModuleInput 定义交易模块的输入依赖
//
// 🎯 **依赖注入配置说明**：
// 本结构体定义了 TX 模块运行所需的所有外部依赖。
// 通过 fx.In 标签，fx 框架会自动注入这些依赖到模块构造函数中。
//
// 📋 **核心依赖**：
// - repository.UTXOManager: 查询 UTXO，验证时引用计数管理
// - mempool.TxPool: 验证后提交交易，TxPool 内部广播
// - crypto.*: 签名、哈希、地址等密码学操作
// - storage.Provider: 草稿存储、缓存等
//
// ⚠️ **可选性控制**：
// - optional:"false" - 必需依赖，缺失时启动失败
// - optional:"true"  - 可选依赖，允许为 nil，模块内需要 nil 检查
type ModuleInput struct {
	fx.In

	// 基础设施组件
	Logger          log.Logger       `optional:"true"`
	ConfigProvider  config.Provider  `optional:"false"`
	StorageProvider storage.Provider `optional:"false"`

	// 加密组件（签名、哈希、地址）
	KeyManager                 crypto.KeyManager                 `optional:"false"`
	SignatureManager           crypto.SignatureManager           `optional:"false"`
	MultiSignatureVerifier     crypto.MultiSignatureVerifier     `optional:"false"`
	ThresholdSignatureVerifier crypto.ThresholdSignatureVerifier `optional:"true"` // 门限签名验证器（可选）
	AddressManager             crypto.AddressManager             `optional:"false"`
	HashManager                crypto.HashManager                `optional:"false"`
	EncryptionManager          crypto.EncryptionManager          `optional:"true"` // 加密管理器（HSM需要，可选）

	// 数据访问组件
	EUTXOQuery   persistence.UTXOQuery   `optional:"false" name:"utxo_query"`
	QueryService persistence.QueryService `optional:"false" name:"query_service"`
	URESCAS      ures.CASStorage         `optional:"false" name:"cas_storage"`

	// 交易池（验证后入池）
	TxPool mempool.TxPool `name:"tx_pool" optional:"false"`

	// 哈希服务客户端（由 crypto 模块提供）
	TransactionHashServiceClient transaction.TransactionHashServiceClient `optional:"false"`

	// P9: 网络与事件集成（可选，用于 P2P 交易传播和事件订阅）
	Network  network.Network `optional:"true"` // P2P 网络服务
	EventBus event.EventBus  `optional:"true"` // 事件总线
}

// ==================== 模块输出服务 ====================

// ModuleOutput 定义 tx 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
// 注意：tx 模块还提供了大量验证插件和端口实现，这些通过 fx.Provide 直接提供，不在此结构体中。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	TxVerifier tx.TxVerifier `name:"tx_verifier"`           // 交易验证器
	FeeManager tx.FeeManager `name:"consensus_fee_manager"` // 费用管理器

	// 核心服务导出（未命名，供其他模块直接使用类型匹配）
	TransactionDraftService tx.TransactionDraftService // 交易草稿服务
	DraftStore              tx.DraftStore              // 草稿存储
	TxProcessor             tx.TxProcessor             // 交易处理器
	IncentiveTxBuilder      tx.IncentiveTxBuilder      // 激励交易构建器
	Signer                  tx.Signer                  // 签名器
	ProofProvider           tx.ProofProvider           // 证明提供者

	// 内部接口导出（未命名，供内部使用）
	InternalProcessor interfaces.Processor // 内部处理器接口
}

// ==================== 模块构建器 ====================

// Module 构建并返回交易模块的 fx 配置
//
// 🎯 **模块构建器**：
// 本函数是交易模块的主要入口点，负责构建完整的 fx 模块配置。
// 按照架构分层组织依赖注入：Builder → Draft → Verifier + Plugins → Processor → Ports
//
// 🏗️ **构建流程**：
// 1. 提供核心组件：Builder、Draft、Verifier、Processor
// 2. 注册验证插件：7种 AuthZ、4种 Conservation、4种 Condition
// 3. 提供端口实现：Signer、FeeEstimator、ProofProvider、DraftStore
// 4. 绑定接口：每个实现同时绑定内部接口和公共接口
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    tx.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保以下依赖模块已正确加载：
// - log、storage、crypto、repository、mempool 等基础模块
func Module() fx.Option {
	return fx.Module("tx",
		// ════════════════════════════════════════════════════════════════════════════
		//                        核心组件（Builder/Draft/Verifier/Processor）
		// ════════════════════════════════════════════════════════════════════════════
		fx.Provide(
			// 哈希规范化器（TX 内部工具，使用 gRPC 服务）
			// 提供为接口类型，供其他组件使用
			func(input ModuleInput) *hash.Canonicalizer {
				return hash.NewCanonicalizer(input.TransactionHashServiceClient)
			},
			// P3: DraftStore - 草稿存储（支持内存和Redis两种实现）
			// 根据配置自动选择存储后端：memory 或 redis
			// ⚠️ **注意**：DraftStore 必须在 DraftService 之前创建，因为 DraftService 依赖 DraftStore
			fx.Annotate(
				func(input ModuleInput) (tx.DraftStore, error) {
					// 从配置系统获取 DraftStore 配置
					draftStoreOptsRaw := input.ConfigProvider.GetDraftStore()
					if draftStoreOptsRaw == nil {
						// 如果没有配置，使用默认的内存存储
						return draftstore.NewMemoryStore(), nil
					}

					// 类型断言为 draftstore.DraftStoreOptions
					draftStoreOpts, ok := draftStoreOptsRaw.(*draftstoreconfig.DraftStoreOptions)
					if !ok {
						// 如果类型不匹配，使用默认的内存存储
						return draftstore.NewMemoryStore(), nil
					}

					// 根据配置类型选择存储实现
					switch draftStoreOpts.Type {
					case "redis":
						// 使用 Redis 存储
						redisConfig := draftStoreOpts.GetRedisConfig()
						if redisConfig == nil {
							return nil, fmt.Errorf("redis config is nil")
						}

						// 转换为 draftstore.Config
						cfg := &draftstore.Config{
							Addr:         redisConfig.Addr,
							Password:     redisConfig.Password,
							DB:           redisConfig.DB,
							KeyPrefix:    redisConfig.KeyPrefix,
							DefaultTTL:   redisConfig.DefaultTTL,
							PoolSize:     redisConfig.PoolSize,
							MinIdleConns: redisConfig.MinIdleConns,
							DialTimeout:  redisConfig.DialTimeout,
							ReadTimeout:  redisConfig.ReadTimeout,
							WriteTimeout: redisConfig.WriteTimeout,
						}

						// 创建 Redis DraftStore
						return draftstore.NewRedisStoreFromConfig(cfg)
					case "memory", "":
						// 使用内存存储（默认）
						return draftstore.NewMemoryStore(), nil
					default:
						return nil, fmt.Errorf("unsupported draft store type: %s", draftStoreOpts.Type)
					}
				},
				fx.As(new(tx.DraftStore)),
			),

			// P3: DraftService - 交易草稿服务（渐进式构建）
			// ⚠️ **依赖**：DraftService 依赖 DraftStore，必须在 DraftStore 之后创建
			fx.Annotate(
				func(draftStore tx.DraftStore, logger log.Logger) tx.TransactionDraftService {
					// 默认最大草稿数 1000
					service := draft.NewService(draftStore, 1000)

					// 注册 DraftService 到内存监控系统
					if reporter, ok := service.(metricsiface.MemoryReporter); ok {
						metricsutil.RegisterMemoryReporter(reporter)
						if logger != nil {
							txLogger := logger.With("module", "tx")
							txLogger.Info("✅ TX DraftService 已注册到内存监控系统")
						}
					}

					return service
				},
				fx.As(new(interfaces.DraftService)),
				fx.As(new(tx.TransactionDraftService)),
			),

			// P1: Verifier Kernel - 验证微内核（三钩子协调器）
			// 需要 UTXOQuery（命名依赖）
			fx.Annotate(
				verifier.NewKernel,
				fx.ParamTags(`name:"utxo_query"`), // persistence.UTXOQuery
			),

			// 提供接口实现（从具体类型转换）
			fx.Annotate(
				func(kernel *verifier.Kernel) tx.TxVerifier {
					return kernel
				},
				fx.ResultTags(`name:"tx_verifier"`),
			),
			func(kernel *verifier.Kernel) processorPkg.Verifier {
				return kernel
			},
			// 同时提供未命名版本的 TxVerifier（供其他模块直接使用类型匹配）
			fx.Annotate(
				func(txVerifier tx.TxVerifier) tx.TxVerifier {
					return txVerifier
				},
				fx.ParamTags(`name:"tx_verifier"`),
			),

			// P1: Verification Plugins（验证插件）
			// SingleKeyPlugin 需要 hashCanonicalizer，通过 fx 注入
			func(input ModuleInput) *authz.SingleKeyPlugin {
				hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
				return authz.NewSingleKeyPlugin(
					input.SignatureManager,
					input.HashManager,
					hashCanonicalizer,
				)
			},
			// BasicConservationPlugin 需要 UTXOQuery（命名依赖）
			fx.Annotate(
				conservation.NewBasicConservationPlugin,
				fx.ParamTags(`name:"utxo_query"`), // persistence.UTXOQuery
			),

			// 高优先级-3: 激励验证插件（Coinbase + 赞助领取）
			incentiveplugin.NewCoinbasePlugin,
			// SponsorClaimPlugin 需要 UTXOQuery（命名依赖）
			fx.Annotate(
				incentiveplugin.NewSponsorClaimPlugin,
				fx.ParamTags(`name:"utxo_query"`), // persistence.UTXOQuery
			),

			// P5: AuthZ Plugins（企业多签）
			// MultiKeyPlugin 需要 MultiSignatureVerifier 和 hashCanonicalizer
			func(input ModuleInput) *authz.MultiKeyPlugin {
				hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
				return authz.NewMultiKeyPlugin(
					input.MultiSignatureVerifier,
					hashCanonicalizer,
				)
			},

			// P8: AuthZ Plugins（复杂授权）
			// DelegationLockPlugin 需要 sigManager 和 hashCanonicalizer
			func(input ModuleInput) *authz.DelegationLockPlugin {
				hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
				return authz.NewDelegationLockPlugin(
					input.SignatureManager,
					hashCanonicalizer,
				)
			},
			// ThresholdLockPlugin 需要 thresholdVerifier 和 hashCanonicalizer
			func(input ModuleInput) *authz.ThresholdLockPlugin {
				hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
				// 注意：ThresholdSignatureVerifier 是可选的，如果未提供则为 nil
				// ThresholdLockPlugin 内部会处理 nil 情况（向后兼容）
				return authz.NewThresholdLockPlugin(
					input.ThresholdSignatureVerifier,
					hashCanonicalizer,
				)
			},
			// ContractLockPlugin 需要 hashManager 和 signatureManager
			func(input ModuleInput) *authz.ContractLockPlugin {
				return authz.NewContractLockPlugin(
					input.HashManager,
					input.SignatureManager,
					input.AddressManager,
				)
			},
			authz.NewContractPlugin,

			// P5: Conservation Plugins（费用机制）
			conservation.NewMinFeePlugin,
			conservation.NewProportionalFeePlugin,

			// P1: Condition Plugins（占位 + 结构性约束）
			condition.NewExecResourceInvariantPlugin,

			// P4: Condition Plugins（交易级窗口验证）
			condition.NewTimeWindowPlugin,
			condition.NewHeightWindowPlugin,

			// P2: Condition Plugins（输入级 Time/Height 锁验证）
			condition.NewTimeLockPlugin,
			condition.NewHeightLockPlugin,

			// P0: Condition Plugins（防重放：tx.nonce）
			condition.NewNoncePlugin,

			// P1: Ports（端口实现）
			// LocalSigner 提供签名功能（导出为 tx.Signer 接口）
			fx.Annotate(
				func(input ModuleInput) (tx.Signer, error) {
					// 🔧 修复：从配置系统获取签名器配置，移除硬编码测试私钥
					signerConfig := input.ConfigProvider.GetSigner()
					localConfig := signerConfig.GetLocalSignerConfig()

					// 构建LocalSigner配置
					config := &signer.LocalSignerConfig{
						PrivateKeyHex: localConfig.PrivateKeyHex,
						Algorithm:     localConfig.Algorithm,
						Environment:   localConfig.Environment,
					}

					hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
					// 🎯 为 TX 模块添加 module 字段
					var txLogger log.Logger
					if input.Logger != nil {
						txLogger = input.Logger.With("module", "tx")
					}
					// ✅ 修复：HashManager 已通过 ModuleInput 注入，直接使用
					return signer.NewLocalSigner(config, input.KeyManager, input.SignatureManager, hashCanonicalizer, txLogger)
				},
				fx.As(new(tx.Signer)),
			),
			// ✅ 修复：为 KMS 和 HSM 签名器提供依赖注入支持（可选）
			// 注意：这些签名器需要额外的配置和客户端，当前仅提供框架
			// 实际使用时需要：
			// 1. 配置 KMS/HSM 客户端
			// 2. 通过 fx.Provide 提供 KMSClient 或 HSM Config
			// 3. 使用 fx.Annotate 替换 LocalSigner
			//
			// KMSSigner 示例（需要 KMSClient 实现）：
			// fx.Annotate(
			//     func(input ModuleInput, kmsClient signer.KMSClient) (tx.Signer, error) {
			//         config := input.ConfigProvider.GetSigner().GetKMSSignerConfig()
			//         hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
			//         return signer.NewKMSSigner(config, kmsClient, input.TransactionHashServiceClient, input.HashManager, input.Logger)
			//     },
			//     fx.As(new(tx.Signer)),
			// ),
			//
			// HSMSigner 示例（需要 HSM Config）：
			// fx.Annotate(
			//     func(input ModuleInput) (tx.Signer, error) {
			//         config := input.ConfigProvider.GetSigner().GetHSMSignerConfig()
			//         hsmConfig := &hsm.Config{
			//             KeyLabel:      config.KeyLabel,
			//             Algorithm:     config.Algorithm,
			//             LibraryPath:   config.LibraryPath,
			//             EncryptedPIN:  config.EncryptedPIN,
			//             SessionPoolSize: config.SessionPoolSize,
			//             Environment:    config.Environment,
			//         }
			//         hashCanonicalizer := hash.NewCanonicalizer(input.TransactionHashServiceClient)
			//         return hsm.NewHSMSigner(hsmConfig, input.TransactionHashServiceClient, input.EncryptionManager, input.HashManager, input.Logger)
			//     },
			//     fx.As(new(tx.Signer)),
			// ),
			proof.NewSimpleProofProvider,

			// FeeManager - 费用管理器（供共识模块使用）
			// 提供命名版本（供 BlockBuilder 使用）
			fx.Annotate(
				func(eutxoQuery persistence.UTXOQuery) tx.FeeManager {
					// 创建UTXOFetcher适配器
					utxoFetcher := func(ctx context.Context, outpoint *transaction_pb.OutPoint) (*transaction_pb.TxOutput, error) {
						utxo, err := eutxoQuery.GetUTXO(ctx, outpoint)
						if err != nil || utxo == nil {
							return nil, err
						}
						return utxo.GetCachedOutput(), nil
					}
					return fee.NewManager(utxoFetcher)
				},
				fx.As(new(tx.FeeManager)),
				fx.ResultTags(`name:"consensus_fee_manager"`),
				fx.ParamTags(`name:"utxo_query"`), // persistence.UTXOQuery
			),
			// 同时提供未命名版本（供 InternalAggregatorService 使用）
			fx.Annotate(
				func(feeManager tx.FeeManager) tx.FeeManager {
					return feeManager
				},
				fx.ParamTags(`name:"consensus_fee_manager"`),
			),

			// IncentiveTxBuilder - 激励交易构建器（供共识模块使用）
			fx.Annotate(
				func(
					feeManager tx.FeeManager,
					eutxoQuery persistence.UTXOQuery,
					configProvider config.Provider,
					signer tx.Signer,
				) tx.IncentiveTxBuilder {
					return builder.NewIncentiveBuilder(
						feeManager,
						eutxoQuery,
						configProvider,
						signer,
					)
				},
				fx.As(new(tx.IncentiveTxBuilder)),
				fx.ParamTags(
					`name:"consensus_fee_manager"`, // tx.FeeManager
					`name:"utxo_query"`,            // persistence.UTXOQuery
					``,                             // config.Provider
					``,                             // tx.Signer
				),
			),

			// P2: Selector - UTXO 选择器（TX 内部实现）
			fx.Annotate(
				selector.NewService,
				fx.ParamTags(
					`name:"utxo_query"`, // persistence.UTXOQuery
					``,                  // log.Logger
				),
			),

			// P2.5: Planner - 交易规划器（协调 Selector 和 Builder）
			planner.NewService,

			// P1: Processor - 交易处理协调器
			// 直接提供，fx 会自动注入 *processorPkg.Service 具体类型
			fx.Annotate(
				processorPkg.NewService,
				fx.ParamTags(
					``,                   // tx.TxVerifier
					`name:"tx_pool"`,     // mempool.TxPool
					``,                   // config.Provider
					`name:"utxo_query"`,  // persistence.UTXOQuery
					`name:"query_service"`, // persistence.QueryService
					``,                   // log.Logger
				),
			),

			// 提供接口实现（从具体类型转换）
			func(svc *processorPkg.Service) interfaces.Processor {
				return svc
			},
			func(svc *processorPkg.Service) tx.TxProcessor {
				return svc
			},
		),

		// ════════════════════════════════════════════════════════════════════════════
		//                        P7: 验证插件自动注册
		// ════════════════════════════════════════════════════════════════════════════
		fx.Invoke(registerVerificationPlugins),

		// ════════════════════════════════════════════════════════════════════════════
		//                        P9: 网络与事件集成注册
		// ════════════════════════════════════════════════════════════════════════════
		fx.Invoke(func(
			inputs ModuleInput,
			processorSvc *processorPkg.Service,
		) error {
			// 🎯 为 TX 模块添加 module 字段，日志将路由到 node-business.log
			var txLogger log.Logger
			if inputs.Logger != nil {
				txLogger = inputs.Logger.With("module", "tx")
			}

			// P9.1: 注册网络协议处理器（如果 Network 可用）
			if inputs.Network != nil && processorSvc != nil {
				// 注册交易流式协议处理器（备用传播路径）
				if err := txNetworkIntegration.RegisterTxStreamHandlers(
					inputs.Network,
					processorSvc, // Processor 实现了 TxProtocolRouter 接口
					txLogger,
				); err != nil {
					if txLogger != nil {
						txLogger.Errorf("[TX] ❌ 注册交易流式协议处理器失败: %v", err)
					}
					return err
				}

				// 注册交易订阅协议处理器（主要传播路径）
				if err := txNetworkIntegration.RegisterSubscribeHandlers(
					inputs.Network,
					processorSvc, // Processor 实现了 TxAnnounceRouter 接口
					txLogger,
				); err != nil {
					if txLogger != nil {
						txLogger.Errorf("[TX] ❌ 注册交易订阅协议处理器失败: %v", err)
					}
					return err
				}

				if txLogger != nil {
					txLogger.Info("[TX] ✅ 交易网络协议处理器注册完成")
				}
			} else if txLogger != nil {
				txLogger.Info("[TX] ⏭️  跳过网络协议注册（Network 或 Processor 未注入）")
			}

			// P9.2: 注册事件订阅（如果 EventBus 可用）
			if inputs.EventBus != nil && processorSvc != nil {
				// 创建事件订阅注册器
				eventRegistry := txEventIntegration.NewEventSubscriptionRegistry(
					inputs.EventBus,
					txLogger,
					processorSvc, // Processor 实现了 TransactionEventSubscriber 接口
					nil,          // SyncEventSubscriber 在 TX 模块中不需要
				)

				// 注册所有事件订阅
				if err := eventRegistry.RegisterEventSubscriptions(); err != nil {
					if txLogger != nil {
						txLogger.Errorf("[TX] ❌ 注册交易事件订阅失败: %v", err)
					}
					return err
				}

				if txLogger != nil {
					txLogger.Info("[TX] ✅ 交易事件订阅注册完成")
				}
			} else if txLogger != nil {
				txLogger.Info("[TX] ⏭️  跳过事件订阅注册（EventBus 或 Processor 未注入）")
			}

			return nil
		}),

		// ════════════════════════════════════════════════════════════════════════════
		//                        模块初始化日志
		// ════════════════════════════════════════════════════════════════════════════
		fx.Invoke(func(logger log.Logger) {
			if logger != nil {
				// 🎯 为 TX 模块添加 module 字段
				txLogger := logger.With("module", "tx")
				txLogger.Info("✅ WES TX 模块已加载完成（Type-state + Micro-kernel + Hexagonal + P7 Auto-Register + P9 Network/Event）")
			}
		}),
	)
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// P7: 验证插件自动注册函数
// ════════════════════════════════════════════════════════════════════════════════════════════════

// registerVerificationPlugins 自动将所有验证插件注册到 Verifier
//
// 🎯 **核心职责**：启动时自动注册所有验证插件
//
// 🔧 **注册流程**：
// 1. 获取 Verifier Kernel 实例
// 2. 注册所有 AuthZ 插件（7种权限验证）
// 3. 注册所有 Conservation 插件（费用机制）
// 4. 注册所有 Condition 插件（交易级条件）
// 5. 设置 TimeLock/HeightLock 的 Verifier 引用（递归验证）
// 6. 输出注册日志
//
// 💡 **设计理念**：
//   - fx.Invoke 在所有 fx.Provide 完成后自动调用
//   - 所有插件通过依赖注入自动获取
//   - 使用 tx.TxVerifier 接口（包含 Register* 方法）
//
// 参数：
//   - kernel: Verifier Kernel 实例
//   - singleKey: SingleKey 插件
//   - multiKey: MultiKey 插件
//   - timeLock: TimeLock 插件
//   - heightLock: HeightLock 插件
//   - delegationLock: DelegationLock 插件（已完善签名验证）
//   - thresholdLock: ThresholdLock 插件（已完善门限签名验证）
//   - contract: Contract 插件
//   - basicCons: Basic Conservation 插件
//   - minFee: MinFee 插件
//   - propFee: ProportionalFee 插件
//   - timeWindow: TimeWindow 插件
//   - heightWindow: HeightWindow 插件
//   - logger: 日志服务
func registerVerificationPlugins(
	verifierKernel *verifier.Kernel, // 使用具体类型（包含 VerifyAuthZLock 方法）
	// AuthZ 插件
	singleKey *authz.SingleKeyPlugin,
	multiKey *authz.MultiKeyPlugin,
	delegationLock *authz.DelegationLockPlugin,
	thresholdLock *authz.ThresholdLockPlugin,
	contract *authz.ContractPlugin,
	// Conservation 插件
	basicCons *conservation.BasicConservationPlugin,
	minFee *conservation.MinFeePlugin,
	propFee *conservation.ProportionalFeePlugin,
	sponsorClaim *incentiveplugin.SponsorClaimPlugin, // P0-5: 赞助领取验证插件
	// Condition 插件
	timeWindow *condition.TimeWindowPlugin,
	heightWindow *condition.HeightWindowPlugin,
	timeLockCond *condition.TimeLockPlugin,
	heightLockCond *condition.HeightLockPlugin,
	// 日志服务
	logger log.Logger,
) error {
	// 🎯 为 TX 模块添加 module 字段
	var txLogger log.Logger
	if logger != nil {
		txLogger = logger.With("module", "tx")
	}

	if txLogger != nil {
		txLogger.Info("[TX Module] 开始注册验证插件...")
	}

	// ===== 1. 注册 AuthZ 插件（权限验证）=====
	authzPlugins := []tx.AuthZPlugin{
		singleKey,
		multiKey,
		delegationLock,
		thresholdLock,
		contract,
	}

	for _, plugin := range authzPlugins {
		verifierKernel.RegisterAuthZPlugin(plugin)
		if txLogger != nil {
			txLogger.Infof("[TX Module] ✅ 注册 AuthZ 插件: %s", plugin.Name())
		}
	}

	// ===== 2. 注册 Conservation 插件（价值守恒）=====
	conservationPlugins := []tx.ConservationPlugin{
		basicCons,
		minFee,
		propFee,
		sponsorClaim, // P0-5: 赞助领取验证插件（验证金额守恒和输出结构）
	}

	for _, plugin := range conservationPlugins {
		verifierKernel.RegisterConservationPlugin(plugin)
		if txLogger != nil {
			txLogger.Infof("[TX Module] ✅ 注册 Conservation 插件: %s", plugin.Name())
		}
	}

	// ===== 3. 注册 Condition 插件（交易级/输入级条件）=====
	conditionPlugins := []tx.ConditionPlugin{
		timeWindow,
		heightWindow,
		timeLockCond,
		heightLockCond,
	}

	for _, plugin := range conditionPlugins {
		verifierKernel.RegisterConditionPlugin(plugin)
		if txLogger != nil {
			txLogger.Infof("[TX Module] ✅ 注册 Condition 插件: %s", plugin.Name())
		}
	}

	if txLogger != nil {
		txLogger.Infof("[TX Module] 🎉 验证插件注册完成: AuthZ=%d, Conservation=%d, Condition=%d",
			len(authzPlugins), len(conservationPlugins), len(conditionPlugins))
	}

	return nil
}
