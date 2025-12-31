// Package incremental provides incremental Merkle tree verification data structures.
package incremental

import (
	"crypto/sha256"
	"time"
)

// ============================================================================
// Merkle Tree增量验证数据结构（增量验证算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 定义Merkle Tree增量验证所需的数据结构。
//
// 🏗️ **实现策略**：
// - 定义Merkle树节点结构
// - 直接使用序列化后的轨迹数据（[]byte），避免重复定义结构
// - 定义增量验证证明结构
// - 定义Merkle路径结构
//
// 📋 **设计原则**：
// - 不依赖 coordinator 包，避免循环依赖
// - 接受序列化后的轨迹数据（[]byte），由调用方负责序列化
// - 使用 coordinator.ExecutionTrace 的序列化方法（serializeExecutionTraceForZK）
//
// ============================================================================

// TraceRecord 轨迹记录（序列化后的数据）
//
// 🎯 **说明**：
// 这是增量验证模块使用的轨迹记录，直接存储序列化后的数据。
// 调用方应使用 coordinator.ExecutionTrace 并序列化为字节数组后传入。
//
// 📋 **序列化方法**：
// 使用 coordinator.Manager.serializeExecutionTraceForZK() 方法序列化 ExecutionTrace
// 该方法使用确定性编码（大端序），确保多次序列化结果一致
type TraceRecord struct {
	SerializedData []byte // 序列化后的轨迹数据（使用 coordinator.serializeExecutionTraceForZK）
	Hash           []byte // 记录哈希（预计算，用于快速比较）
}

// HashFunction 哈希函数接口
type HashFunction func(data []byte) []byte

// DefaultHashFunction 默认哈希函数（SHA256）
//
// ⚠️ **实现说明**：
// 此函数返回一个使用标准库 crypto/sha256 的哈希函数实现。
// 这是增量验证模块的独立实现，用于Merkle树构建和验证。
//
// 📋 **设计考虑**：
// - 增量验证模块是独立的，不依赖外部HashManager
// - HashFunction是函数类型接口，便于后续替换为Poseidon（ZK友好）
// - 如果需要使用HashManager，可以通过NewMerkleTreeBuilder传入自定义哈希函数
//
// 🔧 **使用示例**：
//
//	hashFunc := DefaultHashFunction()
//	hash := hashFunc([]byte("data"))
//
// 🔧 **替换为HashManager**：
//
//	hashFunc := func(data []byte) []byte {
//	    return hashManager.SHA256(data)
//	}
//	builder := NewMerkleTreeBuilder(hashFunc)
func DefaultHashFunction() HashFunction {
	return func(data []byte) []byte {
		hash := sha256.Sum256(data)
		return hash[:]
	}
}

// MerkleTraceNode Merkle树节点
type MerkleTraceNode struct {
	// 节点哈希（32字节）
	Hash []byte

	// 子树
	Left  *MerkleTraceNode
	Right *MerkleTraceNode

	// 节点属性
	IsLeaf bool         // 是否为叶子节点
	Data   *TraceRecord // 叶子节点数据（仅叶子节点）
	Index  int          // 节点索引（用于路径计算）
	Depth  int          // 节点深度（用于优化）
}

// MerkleTraceTree Merkle轨迹树
type MerkleTraceTree struct {
	Root      *MerkleTraceNode // 根节点
	LeafCount int              // 叶子节点数量
	Depth     int              // 树深度
	HashFunc  HashFunction     // 哈希函数（SHA256或Poseidon）
	CreatedAt time.Time        // 创建时间
}

// ChangeType 变更类型
type ChangeType int

const (
	ChangeTypeAdded    ChangeType = iota // 新增
	ChangeTypeModified                   // 修改
	ChangeTypeDeleted                    // 删除
)

// ChangeInfo 变更信息
type ChangeInfo struct {
	Type      ChangeType
	Index     int
	OldRecord *TraceRecord
	NewRecord *TraceRecord
}

// MerklePath Merkle路径
type MerklePath struct {
	LeafIndex      int      // 叶子节点索引
	LeafHash       []byte   // 叶子节点哈希
	SiblingHashes  [][]byte // 兄弟节点哈希列表（从叶子到根）
	PathDirections []int    // 路径方向列表（0=左，1=右）
	RootHash       []byte   // 根哈希（用于验证）
}

// IncrementalVerificationProof 增量验证证明
type IncrementalVerificationProof struct {
	// 旧轨迹信息
	OldRootHash []byte // 旧轨迹的Merkle根哈希

	// 变更信息
	ChangedPaths   []*MerklePath  // 变更路径列表
	ChangedRecords []*TraceRecord // 变更记录列表

	// 新轨迹信息
	NewRootHash []byte // 新轨迹的Merkle根哈希

	// 元数据
	CreatedAt time.Time // 创建时间
}

// SerializeRecord 序列化轨迹记录（用于哈希计算）
//
// 🎯 **说明**：
// TraceRecord 已经包含序列化后的数据，直接返回即可。
// 如果哈希未计算，则使用序列化数据计算哈希。
func SerializeRecord(record *TraceRecord) []byte {
	if record == nil {
		return nil
	}

	// TraceRecord 已经包含序列化后的数据，直接返回
	return record.SerializedData
}

// RecordsEqual 比较两个轨迹记录是否相等
//
// 🎯 **说明**：
// 优先使用预计算的哈希进行比较（快速），否则比较序列化数据。
func RecordsEqual(r1, r2 *TraceRecord) bool {
	if r1 == nil && r2 == nil {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}

	// 使用哈希快速比较
	if len(r1.Hash) > 0 && len(r2.Hash) > 0 {
		return bytesEqual(r1.Hash, r2.Hash)
	}

	// 如果哈希未计算，比较序列化数据
	return bytesEqual(r1.SerializedData, r2.SerializedData)
}

// NewTraceRecord 创建轨迹记录
//
// 🎯 **说明**：
// 从序列化后的轨迹数据创建 TraceRecord。
// 调用方应使用 coordinator.Manager.serializeExecutionTraceForZK() 序列化 ExecutionTrace。
//
// 📋 **参数**：
//   - serializedData: 序列化后的轨迹数据（使用 coordinator.serializeExecutionTraceForZK）
//   - hashFunc: 哈希函数（用于计算记录哈希）
//
// 📋 **返回值**：
//   - *TraceRecord: 轨迹记录
func NewTraceRecord(serializedData []byte, hashFunc HashFunction) *TraceRecord {
	if serializedData == nil {
		return nil
	}

	// 计算哈希（如果未提供哈希函数，使用默认函数）
	if hashFunc == nil {
		hashFunc = DefaultHashFunction()
	}

	hash := hashFunc(serializedData)

	return &TraceRecord{
		SerializedData: serializedData,
		Hash:           hash,
	}
}

// bytesEqual 比较两个字节切片是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============================================================================
// Merkle Tree增量验证数据结构（增量验证算法优化 - 阶段2）
// ============================================================================
