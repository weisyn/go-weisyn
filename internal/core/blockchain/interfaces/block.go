// Package interfaces 定义区块链内部接口
package interfaces

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	blockchain "github.com/weisyn/v1/pkg/interfaces/blockchain"
)

// InternalBlockService 内部区块服务接口
//
// 🎯 设计理念: 继承公共BlockService接口，并提供内部扩展功能
// 📋 核心功能: 提供区块处理的业务服务和内部工具方法
//
// ⚠️ **架构边界说明**:
// - 矿工配置和难度计算等共识相关功能由pkg/interfaces/consensus负责
// - 本接口使用pkg/interfaces/infrastructure/crypto/merkle和hash公共接口
// - 地址验证使用pkg/interfaces/infrastructure/crypto/address公共接口
// - 本接口专注于区块业务逻辑的内部扩展
type InternalBlockService interface {
	blockchain.BlockService // 继承所有公共区块服务方法

	// ==================== 内部扩展方法 ====================

	// CalculateMerkleRoot 计算交易列表的Merkle根
	//
	// 🎯 **标准化Merkle根计算**
	//
	// 使用标准的TransactionHashService和MerkleTreeManager计算Merkle根。
	// 确保创建区块和验证区块时使用相同的计算逻辑，避免不一致问题。
	//
	// 计算过程：
	// 1. 使用TransactionHashServiceClient计算每个交易的标准哈希
	// 2. 使用MerkleTreeManager构建Merkle树
	// 3. 返回Merkle根哈希
	//
	// 参数：
	//   ctx: 上下文对象
	//   transactions: 交易列表（包含Coinbase交易）
	//
	// 返回值：
	//   []byte: 32字节的Merkle根哈希
	//   error: 计算过程中的错误
	//
	// 使用场景：
	//   • CreateMiningCandidate: 创建候选区块时计算Merkle根
	//   • ValidateBlock: 验证区块时重新计算并比较Merkle根
	//
	// 示例：
	//   merkleRoot, err := blockService.CalculateMerkleRoot(ctx, transactions)
	//   if err != nil {
	//     return fmt.Errorf("计算Merkle根失败: %w", err)
	//   }
	CalculateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction) ([]byte, error)

	// ValidateMerkleRoot 验证区块中的Merkle根
	//
	// 🎯 **Merkle根验证**
	//
	// 重新计算交易列表的Merkle根，并与区块头中的Merkle根进行比较。
	// 使用与CalculateMerkleRoot完全相同的计算逻辑，确保一致性。
	//
	// 验证过程：
	// 1. 调用CalculateMerkleRoot重新计算Merkle根
	// 2. 与区块头中声明的Merkle根进行字节级比较
	// 3. 返回验证结果和详细错误信息
	//
	// 参数：
	//   ctx: 上下文对象
	//   transactions: 交易列表（来自区块体）
	//   expectedMerkleRoot: 期望的Merkle根（来自区块头）
	//
	// 返回值：
	//   bool: 验证结果，true表示Merkle根正确
	//   error: 验证过程中的错误
	//
	// 使用场景：
	//   - ValidateBlock: 区块验证过程中的Merkle根校验
	//   - 轻客户端验证: 验证区块完整性而不需要完整区块数据
	//
	// 示例：
	//   valid, err := blockService.ValidateMerkleRoot(ctx, transactions, expectedRoot)
	//   if err != nil {
	//     return fmt.Errorf("Merkle根验证失败: %w", err)
	//   }
	//   if !valid {
	//     return fmt.Errorf("Merkle根不匹配")
	//   }
	ValidateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction, expectedMerkleRoot []byte) (bool, error)

	// ==================== 创世区块处理服务 ====================

	// CreateGenesisBlock 创建创世区块
	//
	// 🎯 **创世区块构建服务**
	//
	// 基于创世交易和配置构建完整的创世区块，包括：
	// 1. 构建创世区块头：设置特殊的创世区块头字段
	// 2. 计算Merkle根：使用创世交易计算Merkle根
	// 3. 设置创世参数：难度、时间戳、版本等
	// 4. 计算状态根：基于初始UTXO状态
	//
	// 创世区块特殊属性：
	// - Height = 0（创世区块标识）
	// - PreviousHash = 全零(32字节)
	// - 不需要POW验证
	// - 使用配置文件中的时间戳
	// - 包含初始代币分配交易
	//
	// 参数:
	//   ctx: 上下文对象
	//   genesisTransactions: 创世交易列表
	//   genesisConfig: 创世区块配置
	//
	// 返回值:
	//   *core.Block: 完整的创世区块
	//   error: 构建过程中的错误
	//
	// 使用示例:
	//   genesisBlock, err := blockService.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	//   if err != nil { return fmt.Errorf("创建创世区块失败: %w", err) }
	//   // 处理创世区块
	CreateGenesisBlock(ctx context.Context, genesisTransactions []*transaction.Transaction, genesisConfig interface{}) (*core.Block, error)

	// ValidateGenesisBlock 验证创世区块
	//
	// 🎯 **创世区块验证服务**
	//
	// 对创世区块进行专门验证，使用创世区块的特殊验证规则：
	// 1. 结构验证：区块头和区块体的完整性
	// 2. 创世特殊验证：高度为0、父哈希为全零
	// 3. 交易验证：验证创世交易的有效性
	// 4. Merkle根验证：验证交易Merkle根的正确性
	// 5. 跳过POW验证：创世区块不需要工作量证明
	// 6. 跳过父区块检查：创世区块没有父区块
	//
	// 验证特点：
	// - 使用创世区块专用的验证规则
	// - 比普通区块验证更宽松（无POW、无父区块）
	// - 更严格的创世特殊字段验证
	//
	// 参数:
	//   ctx: 上下文对象
	//   genesisBlock: 创世区块
	//
	// 返回值:
	//   bool: 验证结果，true表示创世区块有效
	//   error: 验证过程中的错误
	//
	// 使用示例:
	//   valid, err := blockService.ValidateGenesisBlock(ctx, genesisBlock)
	//   if err != nil { return fmt.Errorf("创世区块验证失败: %w", err) }
	//   if !valid { return fmt.Errorf("创世区块验证不通过") }
	ValidateGenesisBlock(ctx context.Context, genesisBlock *core.Block) (bool, error)
}

// ============================================================================
//                              细粒度接口分离
// ============================================================================

// 🎯 **接口分离原则**
//
// 为了解决循环依赖问题，将BlockService按职责分离为多个细粒度接口：
// - BlockValidator: 区块验证职责
// - BlockProcessor: 区块处理职责
// - BlockReader: 区块读取职责
// - BlockWriter: 区块写入职责
//
// 这样不同的服务可以只依赖它们真正需要的接口，避免循环依赖。

// ==================== 区块验证接口 ====================

// BlockValidator 区块验证器接口
//
// 🎯 **职责范围**：专注于区块验证逻辑
// 📋 **使用场景**：ForkService、SyncService需要验证区块时使用
//
// 设计原则：
// - 单一职责：只负责验证，不涉及读写
// - 无状态：验证逻辑应该是纯函数式的
// - 可测试：便于单元测试和Mock
type BlockValidator interface {
	// ValidateBlock 验证区块有效性
	//
	// 🔍 **完整区块验证**
	//
	// 执行完整的区块验证流程，包括：
	// 1. 区块头验证（时间戳、难度、父区块哈希等）
	// 2. 交易验证（签名、UTXO、双花检查等）
	// 3. Merkle根验证
	// 4. 共识规则验证
	//
	// 参数：
	//   ctx: 上下文对象
	//   block: 待验证的区块
	//
	// 返回值：
	//   bool: 验证结果，true表示区块有效
	//   error: 验证过程中的错误
	//
	// 使用场景：
	//   - ForkService: 验证分叉区块
	//   - SyncService: 验证同步的区块
	//   - Miner: 验证接收到的区块
	ValidateBlock(ctx context.Context, block *core.Block) (bool, error)

	// ValidateMerkleRoot 验证区块中的Merkle根
	//
	// 🎯 **Merkle根验证**
	//
	// 重新计算交易列表的Merkle根，并与区块头中的Merkle根进行比较。
	// 使用标准的哈希算法和Merkle树构建逻辑。
	//
	// 参数：
	//   ctx: 上下文对象
	//   transactions: 交易列表（来自区块体）
	//   expectedMerkleRoot: 期望的Merkle根（来自区块头）
	//
	// 返回值：
	//   bool: 验证结果，true表示Merkle根正确
	//   error: 验证过程中的错误
	ValidateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction, expectedMerkleRoot []byte) (bool, error)
}

// ==================== 区块处理接口 ====================

// BlockProcessor 区块处理器接口
//
// 🎯 **职责范围**：专注于区块处理逻辑
// 📋 **使用场景**：ForkService、SyncService需要处理区块时使用
//
// 设计原则：
// - 状态变更：负责更新区块链状态
// - 事务性：处理过程应该是原子性的
// - 可恢复：处理失败时能够回滚状态
type BlockProcessor interface {
	// ProcessBlock 处理区块
	//
	// 🔄 **完整区块处理**
	//
	// 执行完整的区块处理流程，包括：
	// 1. 验证区块（调用BlockValidator）
	// 2. 更新UTXO集合
	// 3. 执行交易（智能合约调用）
	// 4. 更新区块链状态
	// 5. 触发相关事件
	//
	// 参数：
	//   ctx: 上下文对象
	//   block: 待处理的区块
	//
	// 返回值：
	//   error: 处理过程中的错误
	//
	// 使用场景：
	//   - ForkService: 处理分叉区块
	//   - SyncService: 处理同步的区块
	//   - Miner: 处理新挖出的区块
	//
	// 注意事项：
	//   - 处理失败时应该回滚所有状态变更
	//   - 处理成功后应该触发相关事件通知
	ProcessBlock(ctx context.Context, block *core.Block) error
}

// ==================== 区块读取接口 ====================

// BlockReader 区块读取器接口
//
// 🎯 **职责范围**：专注于区块数据读取
// 📋 **使用场景**：需要查询区块数据的服务使用
//
// 设计原则：
// - 只读操作：不修改任何状态
// - 高性能：支持缓存和批量查询
// - 一致性：提供一致的数据视图
type BlockReader interface {
	// GetBlock 根据高度获取区块
	//
	// 参数：
	//   ctx: 上下文对象
	//   height: 区块高度
	//
	// 返回值：
	//   *core.Block: 区块数据
	//   error: 查询错误
	GetBlock(ctx context.Context, height uint64) (*core.Block, error)

	// GetBlockByHash 根据哈希获取区块
	//
	// 参数：
	//   ctx: 上下文对象
	//   hash: 区块哈希
	//
	// 返回值：
	//   *core.Block: 区块数据
	//   error: 查询错误
	GetBlockByHash(ctx context.Context, hash []byte) (*core.Block, error)

	// GetBlockHeight 获取最新区块高度
	//
	// 返回值：
	//   uint64: 最新区块高度
	//   error: 查询错误
	GetBlockHeight(ctx context.Context) (uint64, error)
}

// ==================== 区块写入接口 ====================

// BlockWriter 区块写入器接口
//
// 🎯 **职责范围**：专注于区块数据写入
// 📋 **使用场景**：需要持久化区块数据的服务使用
//
// 设计原则：
// - 原子性：写入操作应该是原子性的
// - 一致性：保证数据一致性
// - 持久性：确保数据持久化
type BlockWriter interface {
	// WriteBlock 写入区块
	//
	// 🔐 **原子写入操作**
	//
	// 将区块数据写入持久化存储，包括：
	// 1. 区块头和区块体
	// 2. 交易索引
	// 3. UTXO更新
	// 4. 元数据更新
	//
	// 参数：
	//   ctx: 上下文对象
	//   block: 要写入的区块
	//
	// 返回值：
	//   error: 写入错误
	//
	// 注意事项：
	//   - 写入失败时应该回滚所有变更
	//   - 写入成功后应该更新相关索引
	WriteBlock(ctx context.Context, block *core.Block) error
}

// ==================== 复合接口 ====================

// BlockValidatorProcessor 区块验证和处理复合接口
//
// 🎯 **便捷接口**：为同时需要验证和处理能力的服务提供
// 📋 **使用场景**：ForkService、SyncService等同时需要验证和处理的场景
type BlockValidatorProcessor interface {
	BlockValidator
	BlockProcessor
}

// BlockReaderWriter 区块读写复合接口
//
// 🎯 **便捷接口**：为同时需要读写能力的服务提供
// 📋 **使用场景**：需要完整区块操作能力的服务
type BlockReaderWriter interface {
	BlockReader
	BlockWriter
}
