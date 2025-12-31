// Package eutxo 提供 EUTXO 模块的类型定义
package eutxo

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// 标识符类型别名（强类型，防止误用）
// ============================================================================
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// 这些类型别名用于在 Go 代码层提供类型安全，防止不同命名空间的标识符混用。

// TxID 交易标识符（32 字节 SHA-256 哈希）
// 属于对象标识命名空间：TxId
type TxID []byte

// BlockID 区块标识符（32 字节 SHA-256 哈希）
// 属于对象标识命名空间：BlockId
type BlockID []byte

// ChainID 链标识符（uint64）
// 属于对象标识命名空间：ChainId
type ChainID uint64

// StateID 状态标识符（32 字节哈希）
// 属于对象标识命名空间：StateId
type StateID []byte

// ContentHash 内容哈希（32 字节 SHA-256 哈希）
// 属于承诺类哈希命名空间：ContentHash
// 在资源场景中，ContentHash = ResourceCodeId
type ContentHash []byte

// ResourceInstanceID 资源实例标识符
// 属于对象标识命名空间：ResourceInstanceId
// 底层表示：OutPoint(TxId, OutputIndex)
type ResourceInstanceID struct {
	TxId        TxID
	OutputIndex uint32
}

// NewResourceInstanceID 创建资源实例标识符
func NewResourceInstanceID(txId []byte, outputIndex uint32) ResourceInstanceID {
	return ResourceInstanceID{
		TxId:        TxID(txId),
		OutputIndex: outputIndex,
	}
}

// ToOutPoint 转换为 OutPoint
func (id ResourceInstanceID) ToOutPoint() *transaction.OutPoint {
	return &transaction.OutPoint{
		TxId:        []byte(id.TxId),
		OutputIndex: id.OutputIndex,
	}
}

// Encode 编码为字符串（格式：{txHashHex}:{outputIndex}）
func (id ResourceInstanceID) Encode() string {
	return EncodeInstanceID([]byte(id.TxId), id.OutputIndex)
}

// DecodeResourceInstanceID 从字符串解码资源实例标识符
func DecodeResourceInstanceID(instanceID string) (ResourceInstanceID, error) {
	txHash, outputIndex, err := DecodeInstanceID(instanceID)
	if err != nil {
		return ResourceInstanceID{}, err
	}
	return NewResourceInstanceID(txHash, outputIndex), nil
}

// ResourceCodeID 资源代码标识符（32 字节内容哈希）
// 属于对象标识命名空间：ResourceCodeId
// 语义：ContentHash = ResourceCodeId（内容维度）
type ResourceCodeID ContentHash

// NewResourceCodeID 创建资源代码标识符
func NewResourceCodeID(contentHash []byte) ResourceCodeID {
	return ResourceCodeID(contentHash)
}

// Bytes 返回字节表示
func (id ResourceCodeID) Bytes() []byte {
	return []byte(id)
}

