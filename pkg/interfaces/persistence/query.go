// Package persistence 提供统一查询服务的公共接口定义
//
// 🔍 **统一查询服务 (Unified Query Service)**
//
// 本包定义 WES 系统的 CQRS 读路径统一查询接口，所有模块的读操作都通过此服务，
// 避免循环依赖，实现清晰的架构边界。
//
// 🎯 **核心职责**：
// - 提供统一的查询入口，避免模块间相互依赖
// - 支持缓存、索引优化等性能优化
// - 可路由到只读副本，提升查询性能
//
// 🏗️ **设计原则**：
// - CQRS 架构：所有读操作统一通过 QueryService
// - 避免循环依赖：模块间不直接查询，都通过 QueryService
// - 性能优化：内部实现可以缓存、索引优化
// - 可扩展性：可路由到只读副本
//
// 📋 **核心接口**：
// - QueryService: 统一查询服务接口（组合所有领域查询接口）
//
// 📁 **接口组织**：
// 所有 Query 接口统一在本文件中，包括：
// - ChainQuery - 链状态查询
// - BlockQuery - 区块查询
// - TxQuery - 交易查询
// - UTXOQuery - EUTXO查询
// - ResourceQuery - URES资源查询
// - AccountQuery - 账户查询（聚合视图）
// - PricingQuery - 定价查询（Phase 2，可选）
//
// 详细使用说明请参考：docs/components/infrastructure/persistence/
package persistence

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
)

// QueryService 统一查询服务接口（CQRS读路径）
//
// 🎯 **核心职责**：
// 所有模块的读操作都通过此服务，避免循环依赖。
// 这是 WES 系统 CQRS 架构的核心组件，提供统一的查询入口。
//
// 💡 **设计理念**：
// - 统一查询入口：所有模块的读操作都通过 QueryService
// - 避免循环依赖：模块间不直接查询，都通过 QueryService
// - 性能优化：内部实现可以缓存、索引优化
// - 可扩展性：可路由到只读副本
//
// 📞 **调用方**：
// - 所有业务模块（ISPC、URES、EUTXO、TX、Block、Chain）
// - API 服务层
// - CLI 工具
//
// ⚠️ **核心约束**：
// - 只读操作：所有方法都是查询操作，不修改状态
// - 线程安全：支持并发调用
// - 性能要求：关键查询方法要求高性能实现
//
// 🏗️ **接口组合**：
// QueryService 通过组合所有领域查询接口，提供完整的查询能力：
// - ChainQuery: 链状态查询
// - BlockQuery: 区块查询
// - TxQuery: 交易查询
// - UTXOQuery: EUTXO查询
// - ResourceQuery: URES资源查询
// - AccountQuery: 账户查询
type QueryService interface {
	// 组合所有领域查询接口
	ChainQuery
	BlockQuery
	TxQuery
	UTXOQuery
	ResourceQuery
	AccountQuery
	PricingQuery // Phase 2: 定价查询接口
}

// ChainQuery 链状态查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供链状态的查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询链状态的组件
type ChainQuery interface {
	// GetChainInfo 获取链基础信息
	//
	// 返回链的基础状态，包括：
	// - 当前高度和最佳区块哈希
	// - 同步状态（是否与网络同步）
	// - 节点模式（轻节点/全节点）
	GetChainInfo(ctx context.Context) (*types.ChainInfo, error)

	// GetCurrentHeight 获取当前链高度
	//
	// 返回当前区块链的高度（最新区块的高度）。
	GetCurrentHeight(ctx context.Context) (uint64, error)

	// GetBestBlockHash 获取最佳区块哈希
	//
	// 返回当前最佳（最新）区块的哈希值。
	GetBestBlockHash(ctx context.Context) ([]byte, error)

	// GetNodeMode 获取节点模式
	//
	// 返回节点的运行模式（Light/Full）。
	GetNodeMode(ctx context.Context) (types.NodeMode, error)

	// IsDataFresh 检查数据新鲜度
	//
	// 检查本地数据是否与网络保持同步。
	// 返回 true 表示数据是最新的，false 表示正在同步中。
	IsDataFresh(ctx context.Context) (bool, error)

	// IsReady 检查系统就绪状态
	//
	// 检查区块链系统是否完全就绪可用。
	// 返回 true 表示系统就绪，false 表示系统未就绪。
	IsReady(ctx context.Context) (bool, error)

	// GetSyncStatus 获取同步状态
	//
	// ⚠️ **已废弃**：同步状态不再持久化，此方法仅返回基本状态信息。
	// 如需完整的同步状态（包括网络高度、同步进度等），请使用 `chain.SystemSyncService.CheckSync()`。
	//
	// 查询当前同步状态，包括：
	// - 本地链高度
	// - 网络高度（可选，需要通过Network查询）
	// - 同步进度
	// - 同步状态（idle/syncing/synced/error）
	//
	// 返回：
	//   - *types.SystemSyncStatus: 同步状态信息（仅包含本地高度，网络高度和进度需要通过SystemSyncService查询）
	//   - error: 查询错误
	//
	// Deprecated: 使用 chain.SystemSyncService.CheckSync() 替代
	GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error)
}

// BlockQuery 区块查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供区块查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询区块信息的组件
type BlockQuery interface {
	// GetBlockByHeight 按高度获取区块
	//
	// 根据区块高度获取完整的区块数据。
	GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error)

	// GetBlockByHash 按哈希获取区块
	//
	// 根据区块哈希获取完整的区块数据。
	GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error)

	// GetBlockHeader 获取区块头
	//
	// 根据区块哈希获取区块头信息（不包含交易列表）。
	GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error)

	// GetBlockRange 获取区块范围
	//
	// 获取指定高度范围内的所有区块。
	// 参数 startHeight 和 endHeight 都包含在内。
	GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error)

	// GetHighestBlock 获取最高区块信息
	//
	// 返回当前最高区块的高度和哈希。
	GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error)
}

// TxQuery 交易查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供交易查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询交易信息的组件
type TxQuery interface {
	// GetTransaction 根据交易哈希获取完整交易及其位置信息
	//
	// 返回完整交易对象，以及交易所在的区块哈希和交易索引。
	GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, transaction *transaction.Transaction, err error)

	// GetTxBlockHeight 获取交易所在的区块高度
	//
	// 根据交易哈希查找交易所在的区块高度。
	GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error)

	// GetBlockTimestamp 获取指定高度的区块时间戳
	//
	// 返回指定高度区块的时间戳。
	GetBlockTimestamp(ctx context.Context, height uint64) (int64, error)

	// GetAccountNonce 获取账户当前nonce
	//
	// 返回指定地址的当前 nonce 值。
	GetAccountNonce(ctx context.Context, address []byte) (uint64, error)

	// GetTransactionsByBlock 获取区块中的所有交易
	//
	// 返回指定区块中包含的所有交易列表。
	GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error)
}

// UTXOQuery EUTXO查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供 EUTXO 查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询 UTXO 信息的组件
type UTXOQuery interface {
	// GetUTXO 根据OutPoint精确获取UTXO
	//
	// 根据交易哈希和输出索引获取 UTXO。
	GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error)

	// GetUTXOsByAddress 获取地址拥有的UTXO列表
	//
	// 返回指定地址拥有的所有 UTXO。
	// 参数：
	//   - address: 所有者地址
	//   - category: UTXO类型过滤（nil表示所有类型）
	//   - onlyAvailable: 是否只返回可用状态的UTXO
	GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error)

	// GetSponsorPoolUTXOs 获取赞助池UTXO列表
	//
	// 返回所有赞助池 UTXO（具有特殊Owner地址的UTXO）。
	// 参数：
	//   - onlyAvailable: 是否只返回可用状态的UTXO
	GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error)

	// GetCurrentStateRoot 获取当前UTXO状态根
	//
	// 返回当前 UTXO 集合的状态根哈希。
	GetCurrentStateRoot(ctx context.Context) ([]byte, error)
}

// ResourceQuery 资源查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供资源查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询资源信息的组件
type ResourceQuery interface {
	// GetResourceByContentHash 根据内容哈希查询完整资源
	//
	// 根据内容哈希获取完整的资源对象。
	GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error)

	// GetResourceFromBlockchain 从区块链获取资源元信息
	//
	// 从区块链查询资源元信息。
	// 返回资源对象和是否存在标志。
	GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error)

	// GetResourceTransaction 获取资源关联的交易信息
	//
	// 返回资源关联的交易哈希、区块哈希和区块高度。
	GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error)

	// CheckFileExists 检查本地文件是否存在
	//
	// 检查指定内容哈希的资源文件是否存在于本地文件系统。
	CheckFileExists(contentHash []byte) bool

	// BuildFilePath 构建本地文件路径
	//
	// 根据内容哈希构建资源文件的本地存储路径。
	BuildFilePath(contentHash []byte) string

	// ListResourceHashes 列出所有资源哈希
	//
	// 返回所有资源的哈希列表。
	// 参数：
	//   - offset: 偏移量
	//   - limit: 返回数量限制
	ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error)
}

// AccountQuery 账户查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供账户查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供账户级别的聚合视图（隐藏UTXO细节）
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询账户信息的组件
type AccountQuery interface {
	// GetAccountBalance 获取账户余额（聚合视图）
	//
	// 返回指定地址的账户余额（聚合所有 UTXO 的余额）。
	// 参数：
	//   - address: 账户地址
	//   - tokenID: 代币ID（nil表示原生代币）
	GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error)
}

// PricingQuery 定价查询接口（QueryService 的组成部分）
//
// 🎯 **核心职责**：
// 提供资源定价状态的查询操作，作为 QueryService 的一部分。
//
// 💡 **设计理念**：
// - 只包含查询操作，不包含写操作
// - 作为 QueryService 的组合接口
// - 提供领域特定的查询方法
//
// 📞 **调用方**：
// - 通过 QueryService 调用
// - 所有需要查询资源定价信息的组件
type PricingQuery interface {
	// GetPricingState 根据资源哈希查询定价状态
	//
	// 根据资源内容哈希获取资源的定价状态对象。
	// 返回 ResourcePricingState 对象。
	GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error)
}

