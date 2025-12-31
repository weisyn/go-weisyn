// Package eutxo 提供 EUTXO 模块的类型定义
package eutxo

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ResourceUTXOStatus 资源 UTXO 状态
type ResourceUTXOStatus string

const (
	// ResourceUTXOStatusActive 活跃状态：UTXO 存在且可用
	ResourceUTXOStatusActive ResourceUTXOStatus = "ACTIVE"
	// ResourceUTXOStatusConsumed 已消费状态：UTXO 已被消费（is_reference_only=false）
	ResourceUTXOStatusConsumed ResourceUTXOStatus = "CONSUMED"
	// ResourceUTXOStatusExpired 已过期状态：UTXO 已过期（expiry_timestamp 已过）
	ResourceUTXOStatusExpired ResourceUTXOStatus = "EXPIRED"
)

// ResourceUTXORecord 资源 UTXO 记录
//
// 🎯 **核心职责**：
// 记录资源 UTXO 的完整信息，包括位置、状态、所有者等。
//
// 💡 **设计理念**：
// - 基于实例标识（ResourceInstanceId）索引：每个资源实例有唯一的 OutPoint
// - ContentHash 作为资源代码标识（ResourceCodeId），用于内容寻址和去重
// - 包含完整的 OutPoint 信息：tx_id + output_index（即 ResourceInstanceId）
// - 记录状态信息：ACTIVE | CONSUMED | EXPIRED
// - 支持生命周期管理：creation_timestamp、expiry_timestamp
//
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// - ContentHash = ResourceCodeId（内容维度，相同内容 → 相同 CodeId）
// - OutPoint(TxId, OutputIndex) = ResourceInstanceId（实例维度，每次部署 → 唯一 InstanceId）
// - 同一份代码可以对应多个实例，每个实例有独立的权限、计费、治理配置
type ResourceUTXORecord struct {
	// InstanceID 资源实例标识符（ResourceInstanceId，主键）
	// 语义：标识资源实例，每次 ResourceOutput 创建 → 唯一 InstanceId
	// 用途：权限、计费、治理、生命周期管理的主键
	InstanceID ResourceInstanceID

	// CodeID 资源代码标识符（ResourceCodeId）
	// 语义：标识资源代码/内容本身，相同内容 → 相同 CodeId
	// 用途：内容寻址存储、去重、缓存、按代码聚合查询
	CodeID ResourceCodeID

	// 向后兼容字段（已废弃，使用 InstanceID 和 CodeID）
	// Deprecated: 使用 InstanceID.TxId 和 InstanceID.OutputIndex
	TxId        []byte
	OutputIndex uint32
	// Deprecated: 使用 CodeID.Bytes()
	ContentHash []byte

	// Owner 所有者地址（从 TxOutput.owner 提取）
	Owner []byte

	// Status 资源 UTXO 状态
	Status ResourceUTXOStatus

	// CreationTimestamp 创建时间戳（从 ResourceOutput.creation_timestamp 提取）
	CreationTimestamp uint64

	// ExpiryTimestamp 过期时间戳（可选，从 ResourceOutput.expiry_timestamp 提取）
	ExpiryTimestamp *uint64

	// IsImmutable 是否不可变（从 ResourceOutput.is_immutable 提取）
	IsImmutable bool
}

// GetOutPoint 获取 OutPoint（ResourceInstanceId）
func (r *ResourceUTXORecord) GetOutPoint() *transaction.OutPoint {
	return r.InstanceID.ToOutPoint()
}

// GetInstanceIDString 获取资源实例标识字符串（用于索引键构建）
func (r *ResourceUTXORecord) GetInstanceIDString() string {
	return r.InstanceID.Encode()
}

// EnsureBackwardCompatibility 确保向后兼容字段被填充（用于序列化兼容）
func (r *ResourceUTXORecord) EnsureBackwardCompatibility() {
	if len(r.TxId) == 0 && len(r.InstanceID.TxId) > 0 {
		r.TxId = []byte(r.InstanceID.TxId)
		r.OutputIndex = r.InstanceID.OutputIndex
	}
	if len(r.ContentHash) == 0 && len(r.CodeID) > 0 {
		r.ContentHash = r.CodeID.Bytes()
	}
}

// EncodeInstanceID 编码资源实例标识为字符串
// 格式：{txHashHex}:{outputIndex}
// 用途：用于构建索引键，如 indices:resource-instance:{instanceID}
func EncodeInstanceID(txHash []byte, outputIndex uint32) string {
	return fmt.Sprintf("%x:%d", txHash, outputIndex)
}

// DecodeInstanceID 解码资源实例标识字符串
// 输入格式：{txHashHex}:{outputIndex}
// 返回：txHash bytes 和 outputIndex
func DecodeInstanceID(instanceID string) ([]byte, uint32, error) {
	parts := strings.Split(instanceID, ":")
	if len(parts) != 2 {
		return nil, 0, fmt.Errorf("invalid instance ID format: %s", instanceID)
	}
	txHash, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tx hash in instance ID: %w", err)
	}
	outputIndex, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid output index in instance ID: %w", err)
	}
	return txHash, uint32(outputIndex), nil
}

// ResourceUsageCounters 资源使用统计
//
// 🎯 **核心职责**：
// 记录资源实例的引用计数和使用统计信息。
//
// 💡 **设计理念**：
// - 引用计数管理：current_reference_count 记录当前引用数
// - 使用统计：total_reference_times 记录总引用次数
// - 时间追踪：记录最后引用的区块高度和时间戳
//
// ⚠️ **标识协议对齐**：
// - 统计应基于 ResourceInstanceId（OutPoint），而非 ContentHash
// - 同一份代码的不同实例应有独立的统计计数
type ResourceUsageCounters struct {
	// InstanceID 资源实例标识（ResourceInstanceId，主键）
	// 语义：标识被统计的资源实例
	// 用途：作为统计的主键，确保每个实例有独立的计数
	InstanceID ResourceInstanceID

	// CodeID 资源代码标识（ResourceCodeId，用于聚合查询）
	// 语义：标识资源代码，用于按代码维度聚合统计
	CodeID ResourceCodeID

	// CurrentReferenceCount 当前引用计数
	// 当 TxInput.is_reference_only=true 时，此计数增加
	// 当 UTXO 被消费时，此计数重置为 0
	CurrentReferenceCount uint64

	// TotalReferenceTimes 总引用次数（累计）
	// 每次引用时增加，不随消费而减少
	TotalReferenceTimes uint64

	// LastReferenceBlockHeight 最后引用的区块高度
	LastReferenceBlockHeight uint64

	// LastReferenceTimestamp 最后引用的时间戳
	LastReferenceTimestamp uint64

	// 向后兼容字段（已废弃，仅用于序列化兼容）
	// Deprecated: 使用 InstanceID
	InstanceTxId  []byte
	InstanceIndex uint32
	// Deprecated: 使用 CodeID.Bytes()
	ContentHash []byte
}

// EnsureBackwardCompatibility 确保向后兼容字段被填充（用于序列化兼容）
func (c *ResourceUsageCounters) EnsureBackwardCompatibility() {
	if len(c.InstanceTxId) == 0 && len(c.InstanceID.TxId) > 0 {
		c.InstanceTxId = []byte(c.InstanceID.TxId)
		c.InstanceIndex = c.InstanceID.OutputIndex
	}
	if len(c.ContentHash) == 0 && len(c.CodeID) > 0 {
		c.ContentHash = c.CodeID.Bytes()
	}
}

// ResourceUTXOFilter 资源 UTXO 过滤条件
type ResourceUTXOFilter struct {
	// Owner 按所有者过滤（可选）
	Owner []byte

	// Status 按状态过滤（可选）
	Status *ResourceUTXOStatus

	// MinCreationTimestamp 最小创建时间戳（可选）
	MinCreationTimestamp *uint64

	// MaxCreationTimestamp 最大创建时间戳（可选）
	MaxCreationTimestamp *uint64
}

