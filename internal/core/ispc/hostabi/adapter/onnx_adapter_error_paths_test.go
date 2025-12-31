package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// ONNXAdapter BuildHostFunctions 错误路径测试
// ============================================================================
//
// 🎯 **测试目的**：发现 ONNXAdapter BuildHostFunctions 中错误处理路径的缺陷和BUG
//
// ============================================================================

// mockHostABIForONNXErrors Mock的HostABI，用于测试错误路径
type mockHostABIForONNXErrors struct {
	getBlockHeightError    error
	getBlockTimestampError error
	getChainIDError        error
	utxoExistsError        error
	resourceExistsError    error
	utxoExistsResult       bool
	resourceExistsResult   bool
}

func (m *mockHostABIForONNXErrors) GetBlockHeight(ctx context.Context) (uint64, error) {
	if m.getBlockHeightError != nil {
		return 0, m.getBlockHeightError
	}
	return 100, nil
}

func (m *mockHostABIForONNXErrors) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	if m.getBlockTimestampError != nil {
		return 0, m.getBlockTimestampError
	}
	return 1234567890, nil
}

func (m *mockHostABIForONNXErrors) GetChainID(ctx context.Context) ([]byte, error) {
	if m.getChainIDError != nil {
		return nil, m.getChainIDError
	}
	return []byte("test-chain"), nil
}

func (m *mockHostABIForONNXErrors) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	if m.utxoExistsError != nil {
		return false, m.utxoExistsError
	}
	return m.utxoExistsResult, nil
}

func (m *mockHostABIForONNXErrors) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	if m.resourceExistsError != nil {
		return false, m.resourceExistsError
	}
	return m.resourceExistsResult, nil
}

// 实现其他必需的方法（最小实现）
func (m *mockHostABIForONNXErrors) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) { return nil, nil }
func (m *mockHostABIForONNXErrors) GetCaller(ctx context.Context) ([]byte, error)                    { return nil, nil }
func (m *mockHostABIForONNXErrors) GetContractAddress(ctx context.Context) ([]byte, error)          { return nil, nil }
func (m *mockHostABIForONNXErrors) GetTransactionID(ctx context.Context) ([]byte, error)            { return nil, nil }
func (m *mockHostABIForONNXErrors) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	return nil, nil
}
func (m *mockHostABIForONNXErrors) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	return nil, nil
}
func (m *mockHostABIForONNXErrors) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIForONNXErrors) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIForONNXErrors) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIForONNXErrors) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIForONNXErrors) EmitEvent(ctx context.Context, eventType string, eventData []byte) error { return nil }
func (m *mockHostABIForONNXErrors) LogDebug(ctx context.Context, message string) error                      { return nil }

// TestONNXAdapter_BuildHostFunctions_GetBlockHeight_Error 测试 get_block_height 错误处理
func TestONNXAdapter_BuildHostFunctions_GetBlockHeight_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		getBlockHeightError: errors.New("获取区块高度失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() int64)
	require.True(t, ok, "get_block_height应该是func() int64类型")

	height := getBlockHeight()
	assert.Equal(t, int64(0), height, "错误时应该返回0")
}

// TestONNXAdapter_BuildHostFunctions_GetBlockTimestamp_Error 测试 get_block_timestamp 错误处理
func TestONNXAdapter_BuildHostFunctions_GetBlockTimestamp_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		getBlockTimestampError: errors.New("获取区块时间戳失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockTimestamp, ok := functions["get_block_timestamp"].(func() int64)
	require.True(t, ok, "get_block_timestamp应该是func() int64类型")

	timestamp := getBlockTimestamp()
	assert.Equal(t, int64(0), timestamp, "错误时应该返回0")
}

// TestONNXAdapter_BuildHostFunctions_GetChainID_Error 测试 get_chain_id 错误处理
func TestONNXAdapter_BuildHostFunctions_GetChainID_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		getChainIDError: errors.New("获取链ID失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func() []byte)
	require.True(t, ok, "get_chain_id应该是func() []byte类型")

	chainID := getChainID()
	assert.Nil(t, chainID, "错误时应该返回nil")
}

// TestONNXAdapter_BuildHostFunctions_UTXOExists_InvalidHashLength 测试 utxo_exists 无效哈希长度
func TestONNXAdapter_BuildHostFunctions_UTXOExists_InvalidHashLength(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	// 测试无效哈希长度（不是32字节）
	invalidHash := make([]byte, 31) // 31字节，不是32字节
	result := utxoExists(invalidHash, 0)
	assert.False(t, result, "无效哈希长度应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_UTXOExists_Error 测试 utxo_exists 错误处理
func TestONNXAdapter_BuildHostFunctions_UTXOExists_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		utxoExistsError: errors.New("查询UTXO失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	// 测试有效哈希长度但查询错误
	validHash := make([]byte, 32)
	result := utxoExists(validHash, 0)
	assert.False(t, result, "查询错误应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_UTXOExists_Success 测试 utxo_exists 成功路径
func TestONNXAdapter_BuildHostFunctions_UTXOExists_Success(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		utxoExistsResult: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	// 测试有效哈希长度且存在
	validHash := make([]byte, 32)
	result := utxoExists(validHash, 0)
	assert.True(t, result, "UTXO存在应该返回true")

	// 测试不存在的情况
	mockABI.utxoExistsResult = false
	result = utxoExists(validHash, 0)
	assert.False(t, result, "UTXO不存在应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_ResourceExists_InvalidHashLength 测试 resource_exists 无效哈希长度
func TestONNXAdapter_BuildHostFunctions_ResourceExists_InvalidHashLength(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	// 测试无效哈希长度（不是32字节）
	invalidHash := make([]byte, 31) // 31字节，不是32字节
	result := resourceExists(invalidHash)
	assert.False(t, result, "无效哈希长度应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_ResourceExists_Error 测试 resource_exists 错误处理
func TestONNXAdapter_BuildHostFunctions_ResourceExists_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		resourceExistsError: errors.New("查询资源失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	// 测试有效哈希长度但查询错误
	validHash := make([]byte, 32)
	result := resourceExists(validHash)
	assert.False(t, result, "查询错误应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_ResourceExists_Success 测试 resource_exists 成功路径
func TestONNXAdapter_BuildHostFunctions_ResourceExists_Success(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{
		resourceExistsResult: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	// 测试有效哈希长度且存在
	validHash := make([]byte, 32)
	result := resourceExists(validHash)
	assert.True(t, result, "资源存在应该返回true")

	// 测试不存在的情况
	mockABI.resourceExistsResult = false
	result = resourceExists(validHash)
	assert.False(t, result, "资源不存在应该返回false")
}

// TestONNXAdapter_BuildHostFunctions_GetBlockHeight_Success 测试 get_block_height 成功路径
func TestONNXAdapter_BuildHostFunctions_GetBlockHeight_Success(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{} // 不设置错误

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() int64)
	require.True(t, ok, "get_block_height应该是func() int64类型")

	height := getBlockHeight()
	assert.Equal(t, int64(100), height, "成功时应该返回正确的区块高度")
}

// TestONNXAdapter_BuildHostFunctions_GetBlockTimestamp_Success 测试 get_block_timestamp 成功路径
func TestONNXAdapter_BuildHostFunctions_GetBlockTimestamp_Success(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{} // 不设置错误

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockTimestamp, ok := functions["get_block_timestamp"].(func() int64)
	require.True(t, ok, "get_block_timestamp应该是func() int64类型")

	timestamp := getBlockTimestamp()
	assert.Equal(t, int64(1234567890), timestamp, "成功时应该返回正确的时间戳")
}

// TestONNXAdapter_BuildHostFunctions_GetChainID_Success 测试 get_chain_id 成功路径
func TestONNXAdapter_BuildHostFunctions_GetChainID_Success(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABIForONNXErrors{} // 不设置错误

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func() []byte)
	require.True(t, ok, "get_chain_id应该是func() []byte类型")

	chainID := getChainID()
	assert.Equal(t, []byte("test-chain"), chainID, "成功时应该返回正确的链ID")
}

