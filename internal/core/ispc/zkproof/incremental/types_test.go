package incremental

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// types.go 测试
// ============================================================================

// TestNewTraceRecord 测试创建轨迹记录
func TestNewTraceRecord(t *testing.T) {
	// 测试正常情况
	data := []byte("test data")
	hashFunc := DefaultHashFunction()
	
	record := NewTraceRecord(data, hashFunc)
	require.NotNil(t, record)
	require.Equal(t, data, record.SerializedData)
	require.NotNil(t, record.Hash)
	require.Equal(t, 32, len(record.Hash)) // SHA256 哈希长度为32字节
	
	// 验证哈希正确性
	expectedHash := sha256.Sum256(data)
	require.Equal(t, expectedHash[:], record.Hash)
}

// TestNewTraceRecord_NilData 测试nil数据
func TestNewTraceRecord_NilData(t *testing.T) {
	record := NewTraceRecord(nil, nil)
	require.Nil(t, record)
}

// TestNewTraceRecord_NilHashFunc 测试nil哈希函数（应使用默认函数）
func TestNewTraceRecord_NilHashFunc(t *testing.T) {
	data := []byte("test data")
	
	record := NewTraceRecord(data, nil)
	require.NotNil(t, record)
	require.Equal(t, data, record.SerializedData)
	require.NotNil(t, record.Hash)
	require.Equal(t, 32, len(record.Hash))
	
	// 验证使用默认哈希函数
	expectedHash := sha256.Sum256(data)
	require.Equal(t, expectedHash[:], record.Hash)
}

// TestNewTraceRecord_CustomHashFunc 测试自定义哈希函数
func TestNewTraceRecord_CustomHashFunc(t *testing.T) {
	data := []byte("test data")
	
	// 自定义哈希函数（简单返回前8字节）
	customHashFunc := func(data []byte) []byte {
		if len(data) < 8 {
			return data
		}
		return data[:8]
	}
	
	record := NewTraceRecord(data, customHashFunc)
	require.NotNil(t, record)
	require.Equal(t, data, record.SerializedData)
	require.Equal(t, 8, len(record.Hash))
	require.Equal(t, data[:8], record.Hash)
}

// TestSerializeRecord 测试序列化记录
func TestSerializeRecord(t *testing.T) {
	// 测试正常情况
	data := []byte("test data")
	record := NewTraceRecord(data, nil)
	
	serialized := SerializeRecord(record)
	require.Equal(t, data, serialized)
}

// TestSerializeRecord_Nil 测试nil记录
func TestSerializeRecord_Nil(t *testing.T) {
	serialized := SerializeRecord(nil)
	require.Nil(t, serialized)
}

// TestRecordsEqual 测试记录相等性
func TestRecordsEqual(t *testing.T) {
	data1 := []byte("test data 1")
	data2 := []byte("test data 2")
	data3 := []byte("test data 1")
	
	record1 := NewTraceRecord(data1, nil)
	record2 := NewTraceRecord(data2, nil)
	record3 := NewTraceRecord(data3, nil)
	
	// 相同数据应该相等
	require.True(t, RecordsEqual(record1, record3))
	
	// 不同数据应该不相等
	require.False(t, RecordsEqual(record1, record2))
	
	// nil情况
	require.True(t, RecordsEqual(nil, nil))
	require.False(t, RecordsEqual(record1, nil))
	require.False(t, RecordsEqual(nil, record1))
}

// TestRecordsEqual_WithoutHash 测试没有哈希时的比较
func TestRecordsEqual_WithoutHash(t *testing.T) {
	data := []byte("test data")
	
	// 创建没有哈希的记录
	record1 := &TraceRecord{
		SerializedData: data,
		Hash:           nil,
	}
	record2 := &TraceRecord{
		SerializedData: data,
		Hash:           nil,
	}
	
	// 应该通过序列化数据比较
	require.True(t, RecordsEqual(record1, record2))
	
	// 不同数据
	record3 := &TraceRecord{
		SerializedData: []byte("different"),
		Hash:           nil,
	}
	require.False(t, RecordsEqual(record1, record3))
}

// TestDefaultHashFunction 测试默认哈希函数
func TestDefaultHashFunction(t *testing.T) {
	hashFunc := DefaultHashFunction()
	require.NotNil(t, hashFunc)
	
	data := []byte("test data")
	hash := hashFunc(data)
	
	// 验证是SHA256
	expectedHash := sha256.Sum256(data)
	require.Equal(t, expectedHash[:], hash)
	require.Equal(t, 32, len(hash))
}

// TestDefaultHashFunction_Consistency 测试默认哈希函数的一致性
func TestDefaultHashFunction_Consistency(t *testing.T) {
	hashFunc1 := DefaultHashFunction()
	hashFunc2 := DefaultHashFunction()
	
	data := []byte("test data")
	hash1 := hashFunc1(data)
	hash2 := hashFunc2(data)
	
	// 相同输入应该产生相同输出
	require.Equal(t, hash1, hash2)
}

// TestDefaultHashFunction_DifferentInputs 测试不同输入
func TestDefaultHashFunction_DifferentInputs(t *testing.T) {
	hashFunc := DefaultHashFunction()
	
	data1 := []byte("test data 1")
	data2 := []byte("test data 2")
	
	hash1 := hashFunc(data1)
	hash2 := hashFunc(data2)
	
	// 不同输入应该产生不同输出
	require.NotEqual(t, hash1, hash2)
}

// TestDefaultHashFunction_EmptyInput 测试空输入
func TestDefaultHashFunction_EmptyInput(t *testing.T) {
	hashFunc := DefaultHashFunction()
	
	hash := hashFunc(nil)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash))
	
	hash2 := hashFunc([]byte{})
	require.NotNil(t, hash2)
	require.Equal(t, 32, len(hash2))
}

