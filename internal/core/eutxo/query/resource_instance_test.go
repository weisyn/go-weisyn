// Package query 提供资源实例查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试资源实例查询的核心功能，重点测试多实例场景：
// - 按实例精确查询
// - 列出代码的所有实例
// - 实例统计查询
//
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// - 测试 ResourceInstanceId（OutPoint）作为主键的查询
// - 测试 ResourceCodeId → ResourceInstanceId 的 1:N 关系
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/eutxo/testutil"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ==================== 按实例查询测试 ====================

// TestGetResourceUTXOByInstance_WithExistingInstance_ReturnsRecord 测试查询存在的实例
func TestGetResourceUTXOByInstance_WithExistingInstance_ReturnsRecord(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()

	// 创建资源 UTXO（模拟部署）
	contentHash := testutil.RandomBytes(32) // 32字节内容哈希
	txHash := testutil.RandomTxID()
	outputIndex := uint32(0)

	// 创建 ResourceOutput
	resourceOutput := &transaction.ResourceOutput{
		Resource: &pbresource.Resource{
			ContentHash: contentHash,
			Category:    pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		},
		CreationTimestamp: 1000,
		IsImmutable:        false,
	}

	// 创建 TxOutput
	txOutput := &transaction.TxOutput{
		Owner: testutil.RandomAddress(),
		OutputContent: &transaction.TxOutput_Resource{
			Resource: resourceOutput,
		},
	}

	// 写入资源索引（使用 writer 的内部方法）
	blockHash := testutil.RandomBytes(32) // 32字节区块哈希
	blockHeight := uint64(1)

	// 直接写入索引（模拟 writer 的行为）
	record := &eutxo.ResourceUTXORecord{
		ContentHash:       contentHash,
		TxId:              txHash,
		OutputIndex:       outputIndex,
		Owner:             txOutput.Owner,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: resourceOutput.CreationTimestamp,
		IsImmutable:       resourceOutput.IsImmutable,
	}

	instanceID := eutxo.EncodeInstanceID(txHash, outputIndex)
	instanceRecordKey := fmt.Sprintf("resource:utxo-instance:%s", instanceID)
	recordData, err := json.Marshal(record)
	require.NoError(t, err)
	err = storage.Set(ctx, []byte(instanceRecordKey), recordData)
	require.NoError(t, err)

	// 写入实例索引
	instanceIndexKey := fmt.Sprintf("indices:resource-instance:%s", instanceID)
	instanceIndexValue := make([]byte, 72) // blockHash(32) + blockHeight(8) + contentHash(32)
	copy(instanceIndexValue[0:32], blockHash)
	copy(instanceIndexValue[32:40], uint64ToBytes(blockHeight))
	copy(instanceIndexValue[40:72], contentHash)
	err = storage.Set(ctx, []byte(instanceIndexKey), instanceIndexValue)
	require.NoError(t, err)

	// 写入代码→实例索引
	codeIndexKey := fmt.Sprintf("indices:resource-code:%x", contentHash)
	instanceList := []string{instanceID}
	codeIndexValue, err := json.Marshal(instanceList)
	require.NoError(t, err)
	err = storage.Set(ctx, []byte(codeIndexKey), codeIndexValue)
	require.NoError(t, err)

	// 创建查询服务
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	// Act
	retrieved, exists, err := queryService.GetResourceUTXOByInstance(ctx, txHash, outputIndex)

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, contentHash, retrieved.ContentHash)
	assert.Equal(t, txHash, retrieved.TxId)
	assert.Equal(t, outputIndex, retrieved.OutputIndex)
	assert.Equal(t, txOutput.Owner, retrieved.Owner)
}

// TestGetResourceUTXOByInstance_WithNonExistentInstance_ReturnsFalse 测试查询不存在的实例
func TestGetResourceUTXOByInstance_WithNonExistentInstance_ReturnsFalse(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	txHash := testutil.RandomTxID()
	outputIndex := uint32(999)

	// Act
	retrieved, exists, err := queryService.GetResourceUTXOByInstance(ctx, txHash, outputIndex)

	// Assert
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, retrieved)
}

// TestGetResourceUTXOByInstance_WithInvalidTxHash_ReturnsError 测试使用无效的交易哈希查询
func TestGetResourceUTXOByInstance_WithInvalidTxHash_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	invalidTxHash := []byte{1, 2, 3} // 不是32字节
	outputIndex := uint32(0)

	// Act
	retrieved, exists, err := queryService.GetResourceUTXOByInstance(ctx, invalidTxHash, outputIndex)

	// Assert
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, retrieved)
	assert.Contains(t, err.Error(), "txHash 必须是 32 字节")
}

// ==================== 列出代码的所有实例测试 ====================

// TestListResourceInstancesByCode_WithMultipleInstances_ReturnsAllInstances 测试列出代码的所有实例（多实例场景）
func TestListResourceInstancesByCode_WithMultipleInstances_ReturnsAllInstances(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()

	// 同一个代码的多个实例
	contentHash := testutil.RandomBytes(32) // 32字节内容哈希
	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	// 实例1
	txHash1 := testutil.RandomTxID()
	outputIndex1 := uint32(0)
	instanceID1 := eutxo.EncodeInstanceID(txHash1, outputIndex1)
	record1 := &eutxo.ResourceUTXORecord{
		ContentHash:       contentHash,
		TxId:              txHash1,
		OutputIndex:       outputIndex1,
		Owner:             owner1,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: 1000,
		IsImmutable:       false,
	}

	// 实例2
	txHash2 := testutil.RandomTxID()
	outputIndex2 := uint32(0)
	instanceID2 := eutxo.EncodeInstanceID(txHash2, outputIndex2)
	record2 := &eutxo.ResourceUTXORecord{
		ContentHash:       contentHash,
		TxId:              txHash2,
		OutputIndex:       outputIndex2,
		Owner:             owner2,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: 2000,
		IsImmutable:       false,
	}

	// 写入实例记录
	instanceRecordKey1 := fmt.Sprintf("resource:utxo-instance:%s", instanceID1)
	recordData1, _ := json.Marshal(record1)
	storage.Set(ctx, []byte(instanceRecordKey1), recordData1)

	instanceRecordKey2 := fmt.Sprintf("resource:utxo-instance:%s", instanceID2)
	recordData2, _ := json.Marshal(record2)
	storage.Set(ctx, []byte(instanceRecordKey2), recordData2)

	// 写入代码→实例索引
	codeIndexKey := fmt.Sprintf("indices:resource-code:%x", contentHash)
	instanceList := []string{instanceID1, instanceID2}
	codeIndexValue, _ := json.Marshal(instanceList)
	storage.Set(ctx, []byte(codeIndexKey), codeIndexValue)

	// 创建查询服务
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	// Act
	instances, err := queryService.ListResourceInstancesByCode(ctx, contentHash)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, instances)
	assert.Equal(t, 2, len(instances), "应该返回2个实例")

	// 验证实例1
	found1 := false
	for _, inst := range instances {
		if inst.TxId != nil && len(inst.TxId) == 32 {
			if string(inst.TxId) == string(txHash1) && inst.OutputIndex == outputIndex1 {
				assert.Equal(t, owner1, inst.Owner)
				found1 = true
				break
			}
		}
	}
	assert.True(t, found1, "应该找到实例1")

	// 验证实例2
	found2 := false
	for _, inst := range instances {
		if inst.TxId != nil && len(inst.TxId) == 32 {
			if string(inst.TxId) == string(txHash2) && inst.OutputIndex == outputIndex2 {
				assert.Equal(t, owner2, inst.Owner)
				found2 = true
				break
			}
		}
	}
	assert.True(t, found2, "应该找到实例2")
}

// TestListResourceInstancesByCode_WithNoInstances_ReturnsEmptyList 测试列出没有实例的代码
func TestListResourceInstancesByCode_WithNoInstances_ReturnsEmptyList(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	contentHash := testutil.RandomBytes(32) // 32字节内容哈希

	// Act
	instances, err := queryService.ListResourceInstancesByCode(ctx, contentHash)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, instances)
	assert.Equal(t, 0, len(instances), "应该返回空列表")
}

// TestListResourceInstancesByCode_WithInvalidContentHash_ReturnsError 测试使用无效的内容哈希
func TestListResourceInstancesByCode_WithInvalidContentHash_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	invalidContentHash := []byte{1, 2, 3} // 不是32字节

	// Act
	instances, err := queryService.ListResourceInstancesByCode(ctx, invalidContentHash)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "contentHash 必须是 32 字节")
}

// ==================== 实例统计查询测试 ====================

// TestGetResourceUsageCountersByInstance_WithExistingInstance_ReturnsCounters 测试查询存在的实例统计
func TestGetResourceUsageCountersByInstance_WithExistingInstance_ReturnsCounters(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()

	txHash := testutil.RandomTxID()
	outputIndex := uint32(0)
	contentHash := testutil.RandomBytes(32) // 32字节内容哈希
	instanceID := eutxo.EncodeInstanceID(txHash, outputIndex)

	// 写入实例统计
	counters := &eutxo.ResourceUsageCounters{
		InstanceTxId:            txHash,
		InstanceIndex:           outputIndex,
		ContentHash:             contentHash,
		CurrentReferenceCount:   5,
		TotalReferenceTimes:     10,
		LastReferenceBlockHeight: 100,
		LastReferenceTimestamp:   2000,
	}

	countersKey := fmt.Sprintf("resource:counters-instance:%s", instanceID)
	countersData, _ := json.Marshal(counters)
	storage.Set(ctx, []byte(countersKey), countersData)

	// 创建查询服务
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	// Act
	retrieved, exists, err := queryService.GetResourceUsageCountersByInstance(ctx, txHash, outputIndex)

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, txHash, retrieved.InstanceTxId)
	assert.Equal(t, outputIndex, retrieved.InstanceIndex)
	assert.Equal(t, contentHash, retrieved.ContentHash)
	assert.Equal(t, uint64(5), retrieved.CurrentReferenceCount)
	assert.Equal(t, uint64(10), retrieved.TotalReferenceTimes)
}

// TestGetResourceUsageCountersByInstance_WithNonExistentInstance_ReturnsDefault 测试查询不存在的实例统计
func TestGetResourceUsageCountersByInstance_WithNonExistentInstance_ReturnsDefault(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	queryService, err := NewResourceService(storage, nil)
	require.NoError(t, err)

	txHash := testutil.RandomTxID()
	outputIndex := uint32(999)

	// Act
	retrieved, exists, err := queryService.GetResourceUsageCountersByInstance(ctx, txHash, outputIndex)

	// Assert
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, txHash, retrieved.InstanceTxId)
	assert.Equal(t, outputIndex, retrieved.InstanceIndex)
	assert.Equal(t, uint64(0), retrieved.CurrentReferenceCount)
	assert.Equal(t, uint64(0), retrieved.TotalReferenceTimes)
}

// ==================== 辅助函数 ====================

// uint64ToBytes 将 uint64 转换为字节数组（BigEndian）
func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(n >> 56)
	b[1] = byte(n >> 48)
	b[2] = byte(n >> 40)
	b[3] = byte(n >> 32)
	b[4] = byte(n >> 24)
	b[5] = byte(n >> 16)
	b[6] = byte(n >> 8)
	b[7] = byte(n)
	return b
}

