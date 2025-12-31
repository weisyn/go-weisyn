package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// ONNXAdapter测试
// ============================================================================
//
// 🎯 **测试目的**：发现ONNXAdapter的缺陷和BUG
//
// ============================================================================

// mockHostABI Mock的HostABI
type mockHostABI struct {
	blockHeight    uint64
	blockTimestamp uint64
	chainID        []byte
	utxoExists     bool
	resourceExists bool
	err            error
}

func (m *mockHostABI) GetBlockHeight(ctx context.Context) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.blockHeight, nil
}

func (m *mockHostABI) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.blockTimestamp, nil
}

func (m *mockHostABI) GetChainID(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chainID, nil
}

func (m *mockHostABI) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.utxoExists, nil
}

func (m *mockHostABI) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.resourceExists, nil
}

// 实现其他必需的方法（最小实现）
func (m *mockHostABI) GetCaller(ctx context.Context) ([]byte, error) { return nil, nil }
func (m *mockHostABI) GetCallerAddress(ctx context.Context) ([]byte, error) { return nil, nil }
func (m *mockHostABI) GetContractAddress(ctx context.Context) ([]byte, error) { return nil, nil }
func (m *mockHostABI) GetTransactionID(ctx context.Context) ([]byte, error) { return nil, nil }
func (m *mockHostABI) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) { return nil, nil }
func (m *mockHostABI) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) { return nil, nil }
func (m *mockHostABI) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) { return nil, nil }
func (m *mockHostABI) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) { return 0, nil }
func (m *mockHostABI) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) { return 0, nil }
func (m *mockHostABI) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) { return 0, nil }
func (m *mockHostABI) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) { return 0, nil }
func (m *mockHostABI) EmitEvent(ctx context.Context, eventType string, data []byte) error { return nil }
func (m *mockHostABI) LogDebug(ctx context.Context, message string) error { return nil }

// TestNewONNXAdapter 测试创建ONNX适配器
func TestNewONNXAdapter(t *testing.T) {
	adapter := NewONNXAdapter()
	assert.NotNil(t, adapter, "适配器不应该为nil")
}

// TestONNXAdapter_BuildHostFunctions 测试构建ONNX宿主函数映射
func TestONNXAdapter_BuildHostFunctions(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		blockHeight:    100,
		blockTimestamp: 1234567890,
		chainID:        []byte{0x01, 0x02, 0x03},
		utxoExists:     true,
		resourceExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	assert.NotNil(t, functions, "函数映射不应该为nil")
	assert.Equal(t, 5, len(functions), "应该有5个宿主函数")

	// 验证函数存在
	assert.Contains(t, functions, "get_block_height", "应该包含get_block_height")
	assert.Contains(t, functions, "get_block_timestamp", "应该包含get_block_timestamp")
	assert.Contains(t, functions, "get_chain_id", "应该包含get_chain_id")
	assert.Contains(t, functions, "utxo_exists", "应该包含utxo_exists")
	assert.Contains(t, functions, "resource_exists", "应该包含resource_exists")
}

// TestONNXAdapter_GetBlockHeight 测试get_block_height函数
func TestONNXAdapter_GetBlockHeight(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		blockHeight: 100,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() int64)
	require.True(t, ok, "get_block_height应该是func() int64类型")

	height := getBlockHeight()
	assert.Equal(t, int64(100), height, "应该返回正确的区块高度")
}

// TestONNXAdapter_GetBlockHeight_Error 测试get_block_height错误处理
func TestONNXAdapter_GetBlockHeight_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		err: assert.AnError,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() int64)
	require.True(t, ok, "get_block_height应该是func() int64类型")

	height := getBlockHeight()
	assert.Equal(t, int64(0), height, "错误时应该返回0")
}

// TestONNXAdapter_GetBlockTimestamp 测试get_block_timestamp函数
func TestONNXAdapter_GetBlockTimestamp(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		blockTimestamp: 1234567890,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockTimestamp, ok := functions["get_block_timestamp"].(func() int64)
	require.True(t, ok, "get_block_timestamp应该是func() int64类型")

	timestamp := getBlockTimestamp()
	assert.Equal(t, int64(1234567890), timestamp, "应该返回正确的时间戳")
}

// TestONNXAdapter_GetChainID 测试get_chain_id函数
func TestONNXAdapter_GetChainID(t *testing.T) {
	adapter := NewONNXAdapter()
	expectedChainID := []byte{0x01, 0x02, 0x03}
	mockABI := &mockHostABI{
		chainID: expectedChainID,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func() []byte)
	require.True(t, ok, "get_chain_id应该是func() []byte类型")

	chainID := getChainID()
	assert.Equal(t, expectedChainID, chainID, "应该返回正确的链ID")
}

// TestONNXAdapter_GetChainID_Error 测试get_chain_id错误处理
func TestONNXAdapter_GetChainID_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		err: assert.AnError,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func() []byte)
	require.True(t, ok, "get_chain_id应该是func() []byte类型")

	chainID := getChainID()
	assert.Nil(t, chainID, "错误时应该返回nil")
}

// TestONNXAdapter_UTXOExists 测试utxo_exists函数
func TestONNXAdapter_UTXOExists(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		utxoExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	txHash := make([]byte, 32)
	txHash[0] = 0x12
	exists := utxoExists(txHash, 0)
	assert.True(t, exists, "应该返回true")
}

// TestONNXAdapter_UTXOExists_InvalidHash 测试utxo_exists无效哈希
// 🐛 **BUG检测**：无效哈希长度应该返回false
func TestONNXAdapter_UTXOExists_InvalidHash(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		utxoExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	invalidHash := []byte{0x12, 0x34} // 长度不是32字节
	exists := utxoExists(invalidHash, 0)
	assert.False(t, exists, "无效哈希应该返回false")
}

// TestONNXAdapter_UTXOExists_Error 测试utxo_exists错误处理
func TestONNXAdapter_UTXOExists_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		err: assert.AnError,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	utxoExists, ok := functions["utxo_exists"].(func([]byte, uint32) bool)
	require.True(t, ok, "utxo_exists应该是func([]byte, uint32) bool类型")

	txHash := make([]byte, 32)
	exists := utxoExists(txHash, 0)
	assert.False(t, exists, "错误时应该返回false")
}

// TestONNXAdapter_ResourceExists 测试resource_exists函数
func TestONNXAdapter_ResourceExists(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		resourceExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	contentHash := make([]byte, 32)
	contentHash[0] = 0x12
	exists := resourceExists(contentHash)
	assert.True(t, exists, "应该返回true")
}

// TestONNXAdapter_ResourceExists_InvalidHash 测试resource_exists无效哈希
// 🐛 **BUG检测**：无效哈希长度应该返回false
func TestONNXAdapter_ResourceExists_InvalidHash(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		resourceExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	invalidHash := []byte{0x12, 0x34} // 长度不是32字节
	exists := resourceExists(invalidHash)
	assert.False(t, exists, "无效哈希应该返回false")
}

// TestONNXAdapter_ResourceExists_Error 测试resource_exists错误处理
func TestONNXAdapter_ResourceExists_Error(t *testing.T) {
	adapter := NewONNXAdapter()
	mockABI := &mockHostABI{
		err: assert.AnError,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	resourceExists, ok := functions["resource_exists"].(func([]byte) bool)
	require.True(t, ok, "resource_exists应该是func([]byte) bool类型")

	contentHash := make([]byte, 32)
	exists := resourceExists(contentHash)
	assert.False(t, exists, "错误时应该返回false")
}

