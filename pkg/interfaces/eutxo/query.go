// Package eutxo 提供 EUTXO 模块的公共接口定义
//
// ✍️ **UTXO 查询接口 (UTXO Query Interface)**
//
// 本包定义 WES 系统的 UTXO 查询接口，供外部模块查询 UTXO 状态。
//
// 🎯 **核心职责**：
// - 提供 UTXO 查询的公共接口
// - 与 InternalUTXOQuery 对应，实现接口分层
// - 确保外部模块可以通过统一接口查询 UTXO
//
// 🏗️ **设计原则**：
// - 公共接口优先：先定义对外能力，再扩展内部方法
// - 接口分层：公共接口 → 内部接口（继承）→ 具体实现
// - 接口隔离：只定义必需的查询方法
//
// 📋 **核心接口**：
// - UTXOQuery: UTXO 查询公共接口
//
// 详细使用说明请参考：docs/components/core/eutxo/
package eutxo

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// UTXOQuery UTXO 查询公共接口
//
// 🎯 **核心职责**：
// 提供 UTXO 查询的公共接口，供外部模块查询 UTXO 状态。
//
// 💡 **设计理念**：
// - 统一查询入口：所有 UTXO 查询都通过此接口
// - 简洁高效：只定义必需的查询方法
// - 类型安全：使用强类型定义，避免错误
//
// 📞 **调用方**：
// - TX 模块：查询 UTXO 状态，验证交易输入
// - Mempool 模块：检查 UTXO 可用性
// - QueryService：统一查询服务
// - 其他需要查询 UTXO 的模块
//
// ⚠️ **核心约束**：
// - 所有方法都是只读操作，不修改 UTXO 状态
// - 查询失败时返回错误，不返回 nil UTXO
// - UTXO 不存在时返回错误，而不是 nil
type UTXOQuery interface {
	// GetUTXO 获取单个 UTXO
	//
	// 🎯 **用途**：
	// - 验证 UTXO 存在性
	// - 检查 UTXO 状态
	// - 获取 UTXO 详细信息
	//
	// 📋 **处理流程**：
	// 1. 验证 OutPoint 有效性
	// 2. 从存储或缓存查询 UTXO
	// 3. 返回 UTXO 对象或错误
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - outpoint: UTXO 的输出点
	//
	// 返回：
	//   - *utxo.UTXO: UTXO 对象
	//   - error: 查询错误，nil 表示成功
	//     如果 UTXO 不存在，返回错误
	//
	// 使用场景：
	//   - TX 模块验证交易输入
	//   - Mempool 检查 UTXO 可用性
	//   - QueryService 提供查询服务
	GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error)

	// GetUTXOsByAddress 按地址查询 UTXO 列表
	//
	// 🎯 **用途**：
	// - 查询指定地址的所有 UTXO
	// - 计算地址余额
	// - 列出地址的可用 UTXO
	//
	// 📋 **处理流程**：
	// 1. 验证地址有效性
	// 2. 使用地址索引查询 UTXO
	// 3. 返回 UTXO 列表
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - address: 地址（字节数组）
	//   - category: UTXO 类别过滤（可选，nil 表示不过滤）
	//   - includeSpent: 是否包含已消费的 UTXO（通常为 false）
	//
	// 返回：
	//   - []*utxo.UTXO: UTXO 列表
	//   - error: 查询错误，nil 表示成功
	//
	// 使用场景：
	//   - 查询账户余额
	//   - 列出可用 UTXO
	//   - UTXO 选择算法
	GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, includeSpent bool) ([]*utxo.UTXO, error)

	// GetReferenceCount 获取 UTXO 的引用计数
	//
	// 🎯 **用途**：
	// - 检查资源 UTXO 的引用计数
	// - 验证删除前引用计数是否为 0
	// - 监控 UTXO 使用情况
	//
	// 📋 **处理流程**：
	// 1. 验证 OutPoint 有效性
	// 2. 从存储查询引用计数
	// 3. 返回引用计数（0 表示没有引用）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - outpoint: UTXO 的输出点
	//
	// 返回：
	//   - uint64: 引用计数（0 表示没有引用）
	//   - error: 查询错误，nil 表示成功
	//
	// 使用场景：
	//   - 验证删除操作
	//   - 监控资源使用
	//   - 调试和诊断
	GetReferenceCount(ctx context.Context, outpoint *transaction.OutPoint) (uint64, error)

	// ListUTXOs 列出指定高度的所有 UTXO
	//
	// ⚠️ 破坏性变更（用于 REORG 深度验证）：该方法从 internal 接口上升为公共接口。
	// - height=0：返回所有 UTXO
	// - height>0：返回该高度及之前的所有 UTXO（与快照语义一致）
	ListUTXOs(ctx context.Context, height uint64) ([]*utxo.UTXO, error)
}

// ResourceUTXOQuery 资源 UTXO 查询接口
//
// 🎯 **核心职责**：
// 提供资源 UTXO 的查询能力，基于 content_hash 索引查询资源 UTXO 信息。
//
// 💡 **设计理念**：
// - 基于 content_hash 查询：每个资源有唯一的 content_hash
// - 支持过滤查询：按 owner、status、时间范围等过滤
// - 只读操作：不修改 UTXO 状态
//
// 📞 **调用方**：
// - ResourceViewService：查询资源 UTXO 信息
// - API 层：提供资源查询服务
// - 其他需要查询资源 UTXO 的模块
//
// ⚠️ **核心约束**：
// - 所有方法都是只读操作，不修改 UTXO 状态
// - 查询失败时返回错误，不返回 nil
// - 资源不存在时返回 (nil, false, nil)，而不是错误
type ResourceUTXOQuery interface {
	// GetResourceUTXOByContentHash 根据内容哈希查询资源 UTXO
	//
	// 🎯 **用途**：
	// - 查询指定资源的 UTXO 信息
	// - 获取资源的 OutPoint、状态、所有者等信息
	//
	// 📋 **处理流程**：
	// 1. 验证 contentHash 有效性（32 字节）
	// 2. 从索引查询 ResourceUTXORecord
	// 3. 返回记录或不存在标志
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - contentHash: 资源内容哈希（32 字节）
	//
	// 返回：
	//   - *ResourceUTXORecord: 资源 UTXO 记录
	//   - bool: 是否存在（true 表示存在）
	//   - error: 查询错误，nil 表示成功
	//
	// 使用场景：
	//   - ResourceViewService.GetResource
	//   - API 层查询资源信息
	GetResourceUTXOByContentHash(ctx context.Context, contentHash []byte) (*ResourceUTXORecord, bool, error)

	// GetResourceUTXOByInstance 根据资源实例标识查询资源 UTXO 记录
	//
	// 🎯 **用途**：
	// - 通过 ResourceInstanceId（OutPoint）查询资源实例
	// - 支持多实例部署场景下的精确查询
	//
	// 📋 **处理流程**：
	// 1. 验证 txHash 和 outputIndex 有效性
	// 2. 从实例索引查询 ResourceUTXORecord
	// 3. 返回记录或不存在标志
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - txHash: 交易哈希（32 字节）
	//   - outputIndex: 输出索引
	//
	// 返回：
	//   - *ResourceUTXORecord: 资源 UTXO 记录
	//   - bool: 是否存在（true 表示存在）
	//   - error: 查询错误，nil 表示成功
	//
	// ⚠️ **标识协议对齐**：
	// - 此方法使用 ResourceInstanceId（OutPoint）作为主键
	// - 相比 GetResourceUTXOByContentHash，此方法支持多实例场景
	GetResourceUTXOByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*ResourceUTXORecord, bool, error)

	// ListResourceInstancesByCode 列出指定代码的所有实例
	//
	// 🎯 **用途**：
	// - 通过 ResourceCodeId（ContentHash）查询所有实例
	// - 支持"一份代码多个部署"的聚合查询
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - contentHash: 资源内容哈希（ResourceCodeId）
	//
	// 返回：
	//   - []*ResourceUTXORecord: 资源实例列表
	//   - error: 查询错误，nil 表示成功
	//
	// ⚠️ **标识协议对齐**：
	// - 此方法展示 ResourceCodeId → ResourceInstanceId 的 1:N 关系
	ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*ResourceUTXORecord, error)

	// ListResourceUTXOs 列出资源 UTXO 列表
	//
	// 🎯 **用途**：
	// - 查询符合条件的资源 UTXO 列表
	// - 支持分页和过滤
	//
	// 📋 **处理流程**：
	// 1. 应用过滤条件（owner、status、时间范围等）
	// 2. 从索引查询符合条件的记录
	// 3. 应用分页（offset、limit）
	// 4. 返回记录列表
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - filter: 过滤条件（可选字段）
	//   - offset: 偏移量（分页用）
	//   - limit: 返回数量限制
	//
	// 返回：
	//   - []*ResourceUTXORecord: 资源 UTXO 记录列表
	//   - error: 查询错误，nil 表示成功
	//
	// 使用场景：
	//   - ResourceViewService.ListResources
	//   - API 层查询资源列表
	ListResourceUTXOs(ctx context.Context, filter ResourceUTXOFilter, offset, limit int) ([]*ResourceUTXORecord, error)

	// GetResourceUsageCounters 获取资源使用统计
	//
	// 🎯 **用途**：
	// - 查询资源的引用计数和使用统计
	// - 监控资源使用情况
	//
	// 📋 **处理流程**：
	// 1. 验证 contentHash 有效性
	// 2. 从索引查询 ResourceUsageCounters
	// 3. 返回统计信息或不存在标志
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - contentHash: 资源内容哈希（32 字节）
	//
	// 返回：
	//   - *ResourceUsageCounters: 资源使用统计
	//   - bool: 是否存在（true 表示存在）
	//   - error: 查询错误，nil 表示成功
	//
	// 使用场景：
	//   - ResourceViewService.GetResource
	//   - 资源使用监控
	GetResourceUsageCounters(ctx context.Context, contentHash []byte) (*ResourceUsageCounters, bool, error)

	// GetResourceUsageCountersByInstance 根据资源实例标识获取使用统计
	//
	// 🎯 **用途**：
	// - 通过 ResourceInstanceId 查询实例级统计
	// - 支持多实例场景下的独立统计
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - txHash: 交易哈希（32 字节）
	//   - outputIndex: 输出索引
	//
	// 返回：
	//   - *ResourceUsageCounters: 资源使用统计
	//   - bool: 是否存在（true 表示存在）
	//   - error: 查询错误，nil 表示成功
	//
	// ⚠️ **标识协议对齐**：
	// - 此方法使用 ResourceInstanceId 作为主键
	// - 相比 GetResourceUsageCounters，此方法确保每个实例有独立统计
	GetResourceUsageCountersByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*ResourceUsageCounters, bool, error)
}

