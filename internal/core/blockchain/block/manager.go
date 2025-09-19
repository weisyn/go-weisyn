// Package block 提供区块链区块管理的实现
//
// ⛓️ **区块管理器 (Block Manager)**
//
// 本文件实现了区块管理服务，专注于：
// - 矿工挖矿：创建挖矿候选区块，返回区块哈希
// - 区块验证：验证从网络接收的区块
// - 区块处理：处理验证通过的区块并更新链状态
//
// 🏗️ **设计原则**
// - 实现内部接口：继承公共 BlockService 接口
// - 依赖注入：通过构造函数注入所需依赖
// - 哈希+缓存：采用与交易服务一致的架构模式
// - 职责单一：专注区块业务逻辑，数据操作委托给repository层
package block

import (
	"context"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 内部接口
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"

	// 内部实现模块
	"github.com/weisyn/v1/internal/core/blockchain/block/genesis"

	// 协议定义
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
//                              管理器实现
// ============================================================================

// Manager 区块管理器
//
// 🎯 **职责定位**：提供完整的区块管理服务
//
// 依赖关系：
// - RepositoryManager：底层数据存储访问
// - TxPool：交易池，获取挖矿交易
// - HashManager：哈希计算服务
// - Logger：日志记录（可选）
//
// 实现特点：
// - 继承内部接口，确保API兼容性
// - 采用哈希+缓存架构，与TransactionService保持一致
// - 支持完整的挖矿和验证流程
// - 提供详细的错误处理和日志记录
type Manager struct {
	// 核心依赖
	repo                   repository.RepositoryManager             // 数据存储访问层
	txPool                 mempool.TxPool                           // 交易池访问
	utxoManager            repository.UTXOManager                   // UTXO管理服务
	minerService           consensus.MinerService                   // 矿工服务，获取矿工地址等
	blockHashServiceClient core.BlockHashServiceClient              // 区块哈希服务客户端
	txHashServiceClient    transaction.TransactionHashServiceClient // 交易哈希服务客户端
	configManager          config.Provider                          // 配置管理器，用于获取链ID等配置
	logger                 log.Logger                               // 日志记录器（可选）

	// 内部服务依赖
	transactionService interfaces.InternalTransactionService // 交易内部服务，负责费用计算等
	networkService     netiface.Network                      // 网络服务，用于GossipSub广播
	eventBus           eventiface.EventBus                   // 事件总线，用于发布分叉检测等事件

	// 加密服务依赖
	merkleTreeManager crypto.MerkleTreeManager // Merkle树管理服务
	hashManager       crypto.HashManager       // 哈希计算服务
	addressManager    crypto.AddressManager    // 地址管理服务
	powEngine         crypto.POWEngine         // POW引擎，用于挖矿验证

	// 内存缓存（使用专业缓存服务）
	cacheStore storage.MemoryStore // 内存缓存服务
}

// NewManager 创建新的区块管理器实例
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数说明：
//   - repo: 仓储管理器，提供底层数据访问能力
//   - txPool: 交易池，用于获取挖矿交易
//   - hashManager: 哈希管理器，用于计算区块哈希
//   - logger: 日志记录器，用于记录操作日志（可选）
//
// 返回：
//   - interfaces.InternalBlockService: 内部区块服务接口实例
//
// 设计说明：
// - 使用依赖注入模式，便于测试和扩展
// - 返回内部接口类型，确保实现完整性
// - 自动满足公共 BlockService 接口要求
// - 初始化内存缓存，支持哈希+缓存架构
//
// 使用示例：
//
//	```go
//	manager := NewManager(repoManager, txPool, hashManager, logger)
//	blockService := manager.(blockchain.BlockService)
//	```
func NewManager(
	repo repository.RepositoryManager,
	txPool mempool.TxPool,
	utxoManager repository.UTXOManager,
	minerService consensus.MinerService,
	transactionService interfaces.InternalTransactionService,
	networkService netiface.Network,
	eventBus eventiface.EventBus,
	blockHashServiceClient core.BlockHashServiceClient,
	txHashServiceClient transaction.TransactionHashServiceClient,
	merkleTreeManager crypto.MerkleTreeManager,
	hashManager crypto.HashManager,
	addressManager crypto.AddressManager,
	powEngine crypto.POWEngine,
	cacheStore storage.MemoryStore,
	configManager config.Provider,
	logger log.Logger,
) interfaces.InternalBlockService {
	if repo == nil {
		panic("区块管理器初始化失败：仓储管理器不能为空")
	}
	if txPool == nil {
		panic("区块管理器初始化失败：交易池不能为空")
	}
	if utxoManager == nil {
		panic("区块管理器初始化失败：UTXO管理器不能为空")
	}
	// 矿工服务允许为nil，在共识模块启动后再注入
	// if minerService == nil {
	//     panic("区块管理器初始化失败：矿工服务不能为空")
	// }
	if transactionService == nil {
		panic("区块管理器初始化失败：交易服务不能为空")
	}
	if eventBus == nil {
		panic("区块管理器初始化失败：事件总线不能为空")
	}
	if blockHashServiceClient == nil {
		panic("区块管理器初始化失败：区块哈希服务客户端不能为空")
	}
	if txHashServiceClient == nil {
		panic("区块管理器初始化失败：交易哈希服务客户端不能为空")
	}
	if merkleTreeManager == nil {
		panic("区块管理器初始化失败：Merkle树管理器不能为空")
	}
	if addressManager == nil {
		panic("区块管理器初始化失败：地址管理器不能为空")
	}
	if powEngine == nil {
		panic("区块管理器初始化失败：POW引擎不能为空")
	}
	if cacheStore == nil {
		panic("区块管理器初始化失败：缓存服务不能为空")
	}

	manager := &Manager{
		repo:                   repo,
		txPool:                 txPool,
		utxoManager:            utxoManager,
		minerService:           minerService,
		transactionService:     transactionService,
		networkService:         networkService,
		eventBus:               eventBus,
		blockHashServiceClient: blockHashServiceClient,
		txHashServiceClient:    txHashServiceClient,
		merkleTreeManager:      merkleTreeManager,
		hashManager:            hashManager,
		addressManager:         addressManager,
		powEngine:              powEngine,
		cacheStore:             cacheStore,
		configManager:          configManager,
		logger:                 logger,
	}

	// 记录初始化日志
	if logger != nil {
		logger.Info("✅ 区块管理器初始化完成 - component: BlockManager, cacheEnabled: true")
	}

	return manager
}

// SetMinerService 设置矿工服务（用于延迟注入，解决循环依赖）
func (m *Manager) SetMinerService(minerService consensus.MinerService) {
	m.minerService = minerService
	if m.logger != nil {
		m.logger.Info("🔗 区块管理器已注入矿工服务")
	}
}

// ============================================================================
//                              矿工挖矿支持
// ============================================================================

// CreateMiningCandidate 创建挖矿候选区块并返回区块哈希
//
// 📁 **实现文件**: create.go
//
// 🎯 **核心挖矿支持方法 - 哈希+缓存架构**
//
// 从交易池获取最优交易，构建候选区块供矿工挖矿。
// 采用与TransactionService一致的哈希+缓存架构：
// - 候选区块保存在内存缓存中
// - 返回32字节区块哈希作为标识符
// - 矿工通过哈希从缓存获取完整区块
//
// 实现流程：
// 1. 从交易池获取优质交易
// 2. 获取当前链状态（高度、父区块哈希）
// 3. 构建候选区块（POW字段为空）
// 4. 计算区块哈希并缓存区块
// 5. 返回区块哈希供矿工使用
//
// 架构优势：
// - 减少网络传输：只传递32字节哈希
// - 支持修改：矿工可在缓存中更新POW字段
// - 性能优化：避免重复序列化大对象
func (m *Manager) CreateMiningCandidate(ctx context.Context) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debug("开始创建挖矿候选区块 - method: CreateMiningCandidate")
	}

	// 调用具体实现方法 (create.go)
	return m.createMiningCandidate(ctx)
}

// ============================================================================
//                              同步验证支持
// ============================================================================

// ValidateBlock 验证区块
//
// 📁 **实现文件**: validate.go
//
// 🎯 **区块验证核心方法**
//
// 对从其他节点接收的区块进行完整验证，确保符合共识规则和协议要求。
//
// 验证项目：
// - 区块结构完整性
// - 区块头字段有效性
// - POW计算正确性
// - 交易有效性
// - 链连接性（父区块存在）
//
// 实现要点：
// - 全面的验证逻辑，确保区块安全
// - 详细的错误信息，便于问题排查
// - 高性能实现，支持快速同步
func (m *Manager) ValidateBlock(ctx context.Context, block *core.Block) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("开始验证区块 - method: ValidateBlock, blockHeight: %d",
			block.Header.Height)
	}

	// 调用具体实现方法 (validate.go)
	return m.validateBlock(ctx, block)
}

// ProcessBlock 处理区块
//
// 📁 **实现文件**: process.go
//
// 🎯 **区块处理核心方法**
//
// 处理验证通过的区块，执行区块中的交易，更新区块链状态。
//
// 处理流程：
// 1. 执行区块中的所有交易
// 2. 更新UTXO状态
// 3. 更新链状态（高度、最佳区块哈希）
// 4. 持久化区块到数据库
// 5. 触发区块处理事件
//
// 实现要点：
// - 原子性：所有操作要么全部成功，要么全部失败
// - 一致性：确保链状态的正确更新
// - 事件通知：通知其他组件区块已处理
func (m *Manager) ProcessBlock(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("开始处理区块 - method: ProcessBlock, blockHeight: %d",
			block.Header.Height)
	}

	// 调用具体实现方法 (process.go)
	return m.processBlock(ctx, block)
}

// ==================== 创世区块处理服务 ====================

// CreateGenesisBlock 创建创世区块
//
// 📁 **实现模块**: genesis/builder.go
//
// 🎯 **薄实现委托模式**
//
// 委托给genesis子模块的BuildBlock函数实现具体业务逻辑
func (m *Manager) CreateGenesisBlock(ctx context.Context, genesisTransactions []*transaction.Transaction, genesisConfig interface{}) (*core.Block, error) {
	return m.createGenesisBlock(ctx, genesisTransactions, genesisConfig)
}

// ValidateGenesisBlock 验证创世区块
//
// 📁 **实现模块**: genesis/validator.go
//
// 🎯 **薄实现委托模式**
//
// 委托给genesis子模块的ValidateBlock函数实现具体业务逻辑
func (m *Manager) ValidateGenesisBlock(ctx context.Context, genesisBlock *core.Block) (bool, error) {
	return m.validateGenesisBlock(ctx, genesisBlock)
}

// ==================== 创世区块内部委托实现 ====================

// createGenesisBlock 内部方法：委托给genesis子模块构建创世区块
func (m *Manager) createGenesisBlock(ctx context.Context, genesisTransactions []*transaction.Transaction, genesisConfig interface{}) (*core.Block, error) {
	return genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		m.txHashServiceClient,
		m.merkleTreeManager,
		m.utxoManager,
		m.logger,
	)
}

// validateGenesisBlock 内部方法：委托给genesis子模块验证创世区块
func (m *Manager) validateGenesisBlock(ctx context.Context, genesisBlock *core.Block) (bool, error) {
	return genesis.ValidateBlock(
		ctx,
		genesisBlock,
		m.txHashServiceClient,
		m.merkleTreeManager,
		m.logger,
	)
}

// ============================================================================
//                              编译时接口检查
// ============================================================================

// 编译时检查接口实现
var (
	_ interfaces.InternalBlockService    = (*Manager)(nil) // 确保实现内部接口
	_ blockchain.BlockService            = (*Manager)(nil) // 确保实现公共接口
	_ interfaces.BlockValidatorProcessor = (*Manager)(nil) // 🎯 确保实现细粒度接口
	_ interfaces.BlockValidator          = (*Manager)(nil) // 🎯 确保实现验证器接口
	_ interfaces.BlockProcessor          = (*Manager)(nil) // 🎯 确保实现处理器接口
	// 注意：BlockReader和BlockWriter由Repository层提供，BlockService不需要实现
)
