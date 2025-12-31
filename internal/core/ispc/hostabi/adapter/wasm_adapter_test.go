package adapter

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// WASMAdapter测试
// ============================================================================
//
// 🎯 **测试目的**：发现WASMAdapter的缺陷和BUG
//
// ============================================================================

// mockHostABIForWASM Mock的HostABI（用于WASM测试）
type mockHostABIForWASM struct {
	blockHeight    uint64
	blockTimestamp uint64
	chainID        []byte
	caller         []byte
	contractAddr   []byte
	txID           []byte
	utxoExists     bool
	resourceExists bool
	err            error
}

func (m *mockHostABIForWASM) GetBlockHeight(ctx context.Context) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.blockHeight, nil
}

func (m *mockHostABIForWASM) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.blockTimestamp, nil
}

func (m *mockHostABIForWASM) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte{0x01, 0x02, 0x03}, nil
}

func (m *mockHostABIForWASM) GetChainID(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chainID, nil
}

func (m *mockHostABIForWASM) GetCaller(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.caller, nil
}

func (m *mockHostABIForWASM) GetCallerAddress(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.caller, nil
}

func (m *mockHostABIForWASM) GetContractAddress(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.contractAddr, nil
}

func (m *mockHostABIForWASM) GetTransactionID(ctx context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.txID, nil
}

func (m *mockHostABIForWASM) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb.TxOutput{
		OutputContent: &pb.TxOutput_Asset{
			Asset: &pb.AssetOutput{
				AssetContent: &pb.AssetOutput_NativeCoin{
					NativeCoin: &pb.NativeCoinAsset{
						Amount: "1000",
					},
				},
			},
		},
	}, nil
}

func (m *mockHostABIForWASM) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.utxoExists, nil
}

func (m *mockHostABIForWASM) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pbresource.Resource{
		ContentHash: contentHash,
		Category:    pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
	}, nil
}

func (m *mockHostABIForWASM) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.resourceExists, nil
}

func (m *mockHostABIForWASM) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockHostABIForWASM) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}

func (m *mockHostABIForWASM) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}

func (m *mockHostABIForWASM) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

func (m *mockHostABIForWASM) EmitEvent(ctx context.Context, eventType string, data []byte) error {
	return nil
}

func (m *mockHostABIForWASM) LogDebug(ctx context.Context, message string) error {
	return nil
}

// createTestWASMAdapter 创建测试用的WASMAdapter
func createTestWASMAdapter(t *testing.T) *WASMAdapter {
	t.Helper()

	logger := testutil.NewTestLogger()
	hashManager := testutil.NewTestHashManager()

	adapter := NewWASMAdapter(
		logger,
		nil, // chainQuery
		nil, // blockQuery
		nil, // eutxoQuery
		nil, // uresCAS
		nil, // txQuery
		nil, // resourceQuery
		nil, // txHashClient
		nil, // addressManager
		hashManager,
		nil, // txAdapter
		nil, // draftService
		func(ctx context.Context) ispcInterfaces.ExecutionContext {
			return nil
		},
		nil, // buildTxFromDraft
		nil, // encodeTxReceipt
	)

	return adapter
}

// TestNewWASMAdapter 测试创建WASM适配器
func TestNewWASMAdapter(t *testing.T) {
	adapter := createTestWASMAdapter(t)

	assert.NotNil(t, adapter, "适配器不应该为nil")
	assert.NotNil(t, adapter.allocators, "allocators应该已初始化")
	assert.Equal(t, 0, len(adapter.allocators), "初始时应该没有分配器")
}

// TestWASMAdapter_BuildHostFunctions 测试构建WASM宿主函数映射
func TestWASMAdapter_BuildHostFunctions(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{
		blockHeight:    100,
		blockTimestamp: 1234567890,
		chainID:        []byte{0x01, 0x02},
		caller:         make([]byte, 20),
		contractAddr:   make([]byte, 20),
		txID:           make([]byte, 32),
		utxoExists:     true,
		resourceExists: true,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	assert.NotNil(t, functions, "函数映射不应该为nil")
	assert.Greater(t, len(functions), 0, "应该有宿主函数")

	// 验证一些关键函数存在
	assert.Contains(t, functions, "get_block_height", "应该包含get_block_height")
	assert.Contains(t, functions, "get_block_timestamp", "应该包含get_block_timestamp")
	assert.Contains(t, functions, "get_chain_id", "应该包含get_chain_id")
}

// TestWASMAdapter_BuildHostFunctions_ErrorHandling 测试错误处理
func TestWASMAdapter_BuildHostFunctions_ErrorHandling(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{
		err: assert.AnError,
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	assert.NotNil(t, functions, "函数映射不应该为nil")

	// 测试get_block_height的错误处理
	getBlockHeight, ok := functions["get_block_height"].(func() uint64)
	require.True(t, ok, "get_block_height应该是func() uint64类型")

	height := getBlockHeight()
	// 🔧 **修复后**：使用 math.MaxUint64 表示错误
	assert.Equal(t, uint64(math.MaxUint64), height, "错误时应该返回 math.MaxUint64")
}

