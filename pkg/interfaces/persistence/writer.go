// Package persistence 提供统一数据写入接口定义
//
// ✍️ **统一数据写入接口 (Unified Data Writer)**
//
// 本包定义 WES 系统的 CQRS 写路径统一写入接口，所有模块的写操作都通过此服务，
// 实现真正的读写分离，确保数据一致性和原子性。
//
// 🎯 **核心职责**：
// - 提供统一的区块写入入口，协调所有数据写入操作
// - 与 QueryService 对应，实现真正的读写分离
// - 确保所有写操作在单一事务中完成
//
// 🏗️ **设计原则**：
// - CQRS 架构：所有写操作统一通过 DataWriter
// - 统一入口：区块是唯一数据写入点
// - 有序写入：严格按高度顺序写入，不接受跳序、倒序
// - 原子性：所有写操作在单一事务中完成
// - 数据源头：交易、UTXO 都从区块中提取
//
// 📋 **核心接口**：
// - DataWriter: 统一数据写入接口
//
// 详细使用说明请参考：docs/components/infrastructure/persistence/
package persistence

import (
	"context"
	"errors"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ErrInvalidHeight 高度不匹配错误
//
// 当区块高度不等于 currentHeight + 1 时返回此错误。
var ErrInvalidHeight = errors.New("block height does not match expected height (must be currentHeight + 1)")

// ErrBlockAlreadyProcessed 区块已处理错误（幂等性保护）
//
// 🆕 2025-12-18: 区分"区块高度低于期望"和"区块高度高于期望"的情况
//
// 当区块高度 <= currentHeight 时返回此错误，表示该区块已被其他流程处理。
// 调用方（如同步流程）可以捕获此错误并跳过，而不是将其视为严重错误。
//
// 场景：
// - 同步流程获取区块 4015 准备写入
// - 同时，聚合器/挖矿流程已经写入了区块 4015
// - DataWriter 检测到高度不匹配，返回 ErrBlockAlreadyProcessed
// - 同步流程捕获此错误，跳过该区块继续处理下一个
var ErrBlockAlreadyProcessed = errors.New("block already processed (height <= currentHeight)")

// DataWriter 统一数据写入接口（CQRS写路径）
//
// 🎯 **核心职责**：
// 提供统一的区块写入入口，协调所有数据写入操作。
// 与 QueryService 对应，实现真正的读写分离。
//
// 💡 **设计理念**：
// - 单一入口：区块是唯一数据写入点
// - 有序写入：严格按高度顺序写入，不接受跳序、倒序
// - 原子性：所有写操作在单一事务中完成
// - 数据源头：交易、UTXO 都从区块中提取
// - 协调写入：内部协调 BlockWriter、TxWriter、UTXOWriter 等
//
// 📞 **调用方**：
// - BlockProcessor：处理验证通过的有序区块
// - ForkHandler：分叉处理后按顺序应用新链区块
// - SyncService：同步过程中按顺序应用历史区块
//
// ⚠️ **核心约束**：
// - 区块必须已通过验证
// - 区块必须按高度顺序写入（n+1 必须在 n 之后）
// - 不接受跳序、倒序或批量乱序区块
// - 所有操作在事务中原子性完成
// - 失败时全部回滚
//
// 🔒 **有序写入原则**：
// - WriteBlock() 只接受高度 = currentHeight + 1 的区块
// - WriteBlocks() 只接受连续且从 currentHeight + 1 开始的区块列表
// - 如果高度不匹配，返回 ErrInvalidHeight，由调用方（BLOCK/CHAIN层）处理
//
// 🌿 **分叉处理职责边界**：
// - DataWriter 不处理分叉逻辑，只负责按顺序写入
// - 分叉检测和链重组由 BLOCK/CHAIN 层（ForkHandler）完成
// - 分叉处理流程：
//   1. ForkHandler 检测分叉，决定是否切换链
//   2. ForkHandler 回滚 UTXO 到分叉点（使用 UTXOSnapshot）
//   3. ForkHandler 按顺序调用 DataWriter.WriteBlock() 应用新链区块
//   4. DataWriter 只负责写入，不关心是否是分叉区块
type DataWriter interface {
	// WriteBlock 写入区块（统一入口，严格有序）
	//
	// 🎯 **核心方法**：
	// 这是数据层的唯一写入入口，所有数据（区块、交易索引、UTXO、状态）
	// 都通过此方法写入。
	//
	// 📋 **处理流程**：
	// 1. 验证高度顺序（必须 = currentHeight + 1）
	// 2. 存储区块数据（blocks/ 文件 + BadgerDB 索引）
	// 3. 提取交易并更新交易索引（只存储索引，不重复存储）
	// 4. 处理 UTXO 变更（从交易中提取，创建/删除 UTXO）
	// 5. 更新链状态（链尖、状态根等）
	// 6. 更新资源索引（如果有资源相关交易）
	// 7. 原子性提交（全部成功或全部失败）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - block: 已验证的区块
	//
	// 返回：
	//   - error: 写入错误，nil表示成功
	//     如果高度不匹配（!= currentHeight + 1），返回 ErrInvalidHeight
	//
	// 使用场景：
	//   - 正常区块处理（高度 = currentHeight + 1）
	//   - 分叉处理后按顺序应用新链区块
	//   - 同步过程中按顺序应用历史区块
	//
	// 说明：
	//   - 区块必须已通过验证（调用 BlockValidator.ValidateBlock）
	//   - 区块高度必须 = currentHeight + 1，否则返回错误
	//   - 所有操作在事务中原子性完成
	//   - 失败时全部回滚
	WriteBlock(ctx context.Context, block *core.Block) error

	// WriteBlocks 批量写入连续区块（优化同步场景，严格有序）
	//
	// 🎯 **批量优化**：
	// 用于同步场景，批量写入多个连续区块，提升性能。
	//
	// ⚠️ **严格有序约束**：
	// - 区块列表必须连续（高度 n, n+1, n+2, ...）
	// - 第一个区块高度必须 = currentHeight + 1
	// - 不接受跳序、倒序或非连续区块
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - blocks: 已验证的区块列表（必须连续且从 currentHeight + 1 开始）
	//
	// 返回：
	//   - error: 写入错误，nil表示成功
	//     如果高度不连续或起始高度不匹配，返回 ErrInvalidHeight
	//
	// 说明：
	//   - 所有区块必须在单一事务中原子性完成
	//   - 如果任一区块失败，全部回滚
	//   - 高度验证：blocks[0].Height == currentHeight + 1
	//   - 连续性验证：blocks[i].Height == blocks[i-1].Height + 1
	WriteBlocks(ctx context.Context, blocks []*core.Block) error

	// DeleteBlockTransactionIndices 删除区块的交易索引（用于分叉处理）
	//
	// 🎯 **用途**：
	// 在分叉处理时，删除原主链区块的交易索引，确保索引一致性。
	//
	// 📋 **处理流程**：
	// 1. 遍历区块中的所有交易
	// 2. 计算每笔交易的哈希
	// 3. 删除对应的交易索引（indices:tx:{txHash}）
	//
	// ⚠️ **关键原则**：
	// - 只在分叉处理时调用，用于清理原主链的交易索引
	// - 不删除区块数据本身（区块保留用于历史查询）
	// - 不影响 UTXO（UTXO 由 UTXOSnapshot 处理）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - block: 要删除交易索引的区块
	//
	// 返回：
	//   - error: 删除错误，nil表示成功
	//
	// 使用场景：
	//   - 分叉处理时，删除原主链区块的交易索引
	//   - 确保索引只指向当前主链上的交易
	DeleteBlockTransactionIndices(ctx context.Context, block *core.Block) error
}

