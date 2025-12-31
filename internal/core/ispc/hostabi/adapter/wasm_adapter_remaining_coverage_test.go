package adapter

import (
	"context"
	"crypto/sha256"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// WASMAdapter剩余覆盖率测试 - 提高覆盖率到80%+
// ============================================================================
//
// 🎯 **测试目的**：发现更多宿主函数的缺陷和BUG，提高覆盖率
//
// ============================================================================

// TestWASMAdapter_AddressBytesToBase58_ConversionFailed 测试地址转换失败
func TestWASMAdapter_AddressBytesToBase58_ConversionFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 设置AddressManager返回错误
	adapter.addressManager = &mockAddressManagerWithError{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	resultPtr := uint32(2048)
	maxLen := uint32(100)
	result := addressBytesToBase58(ctx, module, addrPtr, resultPtr, maxLen)
	assert.Equal(t, uint32(0), result, "地址转换失败应该返回0")
}

// mockAddressManagerWithError Mock的AddressManager（返回错误）
type mockAddressManagerWithError struct{}

func (m *mockAddressManagerWithError) BytesToAddress(bytes []byte) (string, error) {
	return "", assert.AnError
}

func (m *mockAddressManagerWithError) AddressToBytes(address string) ([]byte, error) {
	return nil, assert.AnError
}

func (m *mockAddressManagerWithError) ValidateAddress(address string) (bool, error) {
	return false, assert.AnError
}

func (m *mockAddressManagerWithError) AddressToHexString(address string) (string, error) {
	return "", assert.AnError
}

func (m *mockAddressManagerWithError) HexStringToAddress(hexStr string) (string, error) {
	return "", assert.AnError
}

func (m *mockAddressManagerWithError) CompareAddresses(addr1, addr2 string) (bool, error) {
	return false, assert.AnError
}

func (m *mockAddressManagerWithError) GetAddressType(address string) (crypto.AddressType, error) {
	return crypto.AddressTypeBitcoin, assert.AnError
}

func (m *mockAddressManagerWithError) IsZeroAddress(address string) bool {
	return false
}

func (m *mockAddressManagerWithError) StringToAddress(s string) (string, error) {
	return "", assert.AnError
}

func (m *mockAddressManagerWithError) PrivateKeyToAddress(privateKey []byte) (string, error) {
	return "", assert.AnError
}

func (m *mockAddressManagerWithError) PublicKeyToAddress(publicKey []byte) (string, error) {
	return "", assert.AnError
}

// TestWASMAdapter_AddressBase58ToBytes_NilAddressManager 测试nil AddressManager
func TestWASMAdapter_AddressBase58ToBytes_NilAddressManager(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	adapter.addressManager = nil // 设置为nil

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBase58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	base58Ptr := uint32(1024)
	base58Str := []byte("test-address")
	memory.Write(base58Ptr, base58Str)

	resultPtr := uint32(2048)
	result := addressBase58ToBytes(ctx, module, base58Ptr, uint32(len(base58Str)), resultPtr)
	assert.Equal(t, uint32(0), result, "nil AddressManager应该返回0")
}

// TestWASMAdapter_AppendTxInput_InvalidTxIDLength 测试无效txID长度
func TestWASMAdapter_AppendTxInput_InvalidTxIDLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用无效的txID长度（不是32字节）
	txIDPtr := uint32(1024)
	txID := make([]byte, 20) // 20字节，不是32字节
	memory.Write(txIDPtr, txID)

	result := appendTxInput(ctx, module, txIDPtr, 20, 0, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidParameter), result, "无效txID长度应该返回错误")
}

// TestWASMAdapter_AppendTxInput_InvalidProof 测试无效proof
func TestWASMAdapter_AppendTxInput_InvalidProof(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	memory.Write(txIDPtr, txID)

	// 写入无效的proof（不是有效的protobuf）
	proofPtr := uint32(2048)
	invalidProof := []byte("invalid-protobuf")
	memory.Write(proofPtr, invalidProof)

	result := appendTxInput(ctx, module, txIDPtr, 32, 0, 1, proofPtr, uint32(len(invalidProof)))
	assert.Equal(t, uint32(ErrEncodingFailed), result, "无效proof应该返回错误")
}

// TestWASMAdapter_GetBlockHash_NilBlockQuery 测试nil blockQuery
func TestWASMAdapter_GetBlockHash_NilBlockQuery(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	adapter.blockQuery = nil // 设置为nil

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getBlockHash, ok := functions["get_block_hash"].(func(context.Context, api.Module, uint64, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	hashPtr := uint32(1024)
	result := getBlockHash(ctx, module, 100, hashPtr)
	assert.Equal(t, uint32(0), result, "nil blockQuery应该返回0")
}

// TestWASMAdapter_GetBlockHash_InvalidHashLength 测试哈希长度无效
func TestWASMAdapter_GetBlockHash_InvalidHashLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 设置hashManager返回非32字节的哈希
	adapter.blockQuery = &mockBlockQuery{}
	adapter.hashManager = &mockHashManagerWithInvalidLength{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getBlockHash, ok := functions["get_block_hash"].(func(context.Context, api.Module, uint64, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	hashPtr := uint32(1024)
	result := getBlockHash(ctx, module, 100, hashPtr)
	assert.Equal(t, uint32(0), result, "无效哈希长度应该返回0")
}

// mockHashManagerWithInvalidLength Mock的HashManager（返回无效长度的哈希）
type mockHashManagerWithInvalidLength struct{}

func (m *mockHashManagerWithInvalidLength) SHA256(data []byte) []byte {
	return make([]byte, 20) // 返回20字节，不是32字节
}

func (m *mockHashManagerWithInvalidLength) Keccak256(data []byte) []byte {
	return make([]byte, 20) // 返回20字节，不是32字节
}

func (m *mockHashManagerWithInvalidLength) DoubleSHA256(data []byte) []byte {
	return make([]byte, 20) // 返回20字节，不是32字节
}

func (m *mockHashManagerWithInvalidLength) Hash160(data []byte) []byte {
	return make([]byte, 20)
}

func (m *mockHashManagerWithInvalidLength) RIPEMD160(data []byte) []byte {
	return make([]byte, 20)
}

func (m *mockHashManagerWithInvalidLength) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

func (m *mockHashManagerWithInvalidLength) NewRIPEMD160Hasher() hash.Hash {
	return sha256.New() // 简化实现
}

// TestWASMAdapter_GetTransactionID_NilTx 测试get_transaction_id nil Tx
func TestWASMAdapter_GetTransactionID_NilTx(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.draftID = "draft-123"
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}
	adapter.draftService = &mockDraftServiceForAdapterWithNilTx{}
	adapter.txHashClient = &mockTxHashServiceClientForAdapter{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getTxID, ok := functions["get_transaction_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	result := getTxID(ctx, module, txIDPtr)
	assert.Equal(t, uint32(ErrInternalError), result, "nil Tx应该返回错误")
}

// mockDraftServiceForAdapterWithNilTx Mock的DraftService（返回nil Tx的draft）
type mockDraftServiceForAdapterWithNilTx struct{}

func (m *mockDraftServiceForAdapterWithNilTx) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	return &types.DraftTx{Tx: nil}, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{Tx: nil}, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftServiceForAdapterWithNilTx) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{Tx: nil}, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftServiceForAdapterWithNilTx) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}

func (m *mockDraftServiceForAdapterWithNilTx) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapterWithNilTx) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

// TestWASMAdapter_GetTransactionID_InvalidHashLength 测试get_transaction_id无效哈希长度
func TestWASMAdapter_GetTransactionID_InvalidHashLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.draftID = "draft-123"
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}
	adapter.draftService = &mockDraftServiceForAdapter{}
	adapter.txHashClient = &mockTxHashServiceClientForAdapterWithInvalidHash{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getTxID, ok := functions["get_transaction_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	result := getTxID(ctx, module, txIDPtr)
	assert.Equal(t, uint32(ErrInternalError), result, "无效哈希长度应该返回错误")
}

// mockTxHashServiceClientForAdapterWithInvalidHash Mock的TransactionHashServiceClient（返回无效长度的哈希）
type mockTxHashServiceClientForAdapterWithInvalidHash struct{}

func (m *mockTxHashServiceClientForAdapterWithInvalidHash) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	return &transaction.ComputeHashResponse{
		Hash:    make([]byte, 20), // 返回20字节，不是32字节
		IsValid: true,
	}, nil
}

func (m *mockTxHashServiceClientForAdapterWithInvalidHash) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{IsValid: true}, nil
}

func (m *mockTxHashServiceClientForAdapterWithInvalidHash) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return &transaction.ComputeSignatureHashResponse{Hash: make([]byte, 32)}, nil
}

func (m *mockTxHashServiceClientForAdapterWithInvalidHash) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{IsValid: true}, nil
}

// TestWASMAdapter_GetTransactionID_WriteFailed 测试get_transaction_id写入失败
func TestWASMAdapter_GetTransactionID_WriteFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.draftID = "draft-123"
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}
	adapter.draftService = &mockDraftServiceForAdapter{}
	adapter.txHashClient = &mockTxHashServiceClientForAdapter{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getTxID, ok := functions["get_transaction_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	txIDPtr := uint32(memSize + 100) // 超出范围

	result := getTxID(ctx, module, txIDPtr)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "写入失败应该返回错误")
}

// TestWASMAdapter_AddressBytesToBase58_ReadFailed 测试读取地址失败
func TestWASMAdapter_AddressBytesToBase58_ReadFailed(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	addrPtr := uint32(memSize + 100) // 超出范围

	resultPtr := uint32(2048)
	maxLen := uint32(100)
	result := addressBytesToBase58(ctx, module, addrPtr, resultPtr, maxLen)
	assert.Equal(t, uint32(0), result, "读取地址失败应该返回0")
}

// TestWASMAdapter_AddressBase58ToBytes_ReadFailed 测试读取Base58字符串失败
func TestWASMAdapter_AddressBase58ToBytes_ReadFailed(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBase58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	base58Ptr := uint32(memSize + 100) // 超出范围

	resultPtr := uint32(2048)
	result := addressBase58ToBytes(ctx, module, base58Ptr, 10, resultPtr)
	assert.Equal(t, uint32(0), result, "读取Base58字符串失败应该返回0")
}

// TestWASMAdapter_AddressBase58ToBytes_InvalidLength 测试解码后长度无效
func TestWASMAdapter_AddressBase58ToBytes_InvalidLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 设置AddressManager返回非20字节的地址
	adapter.addressManager = &mockAddressManagerWithInvalidLength{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBase58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	base58Ptr := uint32(1024)
	base58Str := []byte("test-address")
	memory.Write(base58Ptr, base58Str)

	resultPtr := uint32(2048)
	result := addressBase58ToBytes(ctx, module, base58Ptr, uint32(len(base58Str)), resultPtr)
	assert.Equal(t, uint32(0), result, "无效长度应该返回0")
}

// mockAddressManagerWithInvalidLength Mock的AddressManager（返回无效长度的地址）
type mockAddressManagerWithInvalidLength struct{}

func (m *mockAddressManagerWithInvalidLength) BytesToAddress(bytes []byte) (string, error) {
	return "test-address", nil
}

func (m *mockAddressManagerWithInvalidLength) AddressToBytes(address string) ([]byte, error) {
	return make([]byte, 19), nil // 返回19字节，不是20字节
}

func (m *mockAddressManagerWithInvalidLength) ValidateAddress(address string) (bool, error) {
	return true, nil
}

func (m *mockAddressManagerWithInvalidLength) AddressToHexString(address string) (string, error) {
	return "", nil
}

func (m *mockAddressManagerWithInvalidLength) HexStringToAddress(hexStr string) (string, error) {
	return "", nil
}

func (m *mockAddressManagerWithInvalidLength) CompareAddresses(addr1, addr2 string) (bool, error) {
	return false, nil
}

func (m *mockAddressManagerWithInvalidLength) GetAddressType(address string) (crypto.AddressType, error) {
	return crypto.AddressTypeBitcoin, nil
}

func (m *mockAddressManagerWithInvalidLength) IsZeroAddress(address string) bool {
	return false
}

func (m *mockAddressManagerWithInvalidLength) StringToAddress(s string) (string, error) {
	return "", nil
}

func (m *mockAddressManagerWithInvalidLength) PrivateKeyToAddress(privateKey []byte) (string, error) {
	return "", nil
}

func (m *mockAddressManagerWithInvalidLength) PublicKeyToAddress(publicKey []byte) (string, error) {
	return "", nil
}

// TestWASMAdapter_AddressBase58ToBytes_WriteFailed 测试写入失败
func TestWASMAdapter_AddressBase58ToBytes_WriteFailed(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBase58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	base58Ptr := uint32(1024)
	base58Str := []byte("test-address")
	memory.Write(base58Ptr, base58Str)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	resultPtr := uint32(memSize + 100) // 超出范围

	result := addressBase58ToBytes(ctx, module, base58Ptr, uint32(len(base58Str)), resultPtr)
	assert.Equal(t, uint32(0), result, "写入失败应该返回0")
}

// TestWASMAdapter_AddressBytesToBase58_WriteFailed 测试写入失败
func TestWASMAdapter_AddressBytesToBase58_WriteFailed(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	addressBytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	resultPtr := uint32(memSize + 100) // 超出范围

	maxLen := uint32(100)
	result := addressBytesToBase58(ctx, module, addrPtr, resultPtr, maxLen)
	assert.Equal(t, uint32(0), result, "写入失败应该返回0")
}

// TestWASMAdapter_AppendTxInput_ReadTxIDFailed 测试读取txID失败
func TestWASMAdapter_AppendTxInput_ReadTxIDFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	txIDPtr := uint32(memSize + 100) // 超出范围

	result := appendTxInput(ctx, module, txIDPtr, 32, 0, 0, 0, 0)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "读取txID失败应该返回错误")
}

// TestWASMAdapter_AppendTxInput_ReadProofFailed 测试读取proof失败
func TestWASMAdapter_AppendTxInput_ReadProofFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	memory.Write(txIDPtr, txID)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	proofPtr := uint32(memSize + 100) // 超出范围

	result := appendTxInput(ctx, module, txIDPtr, 32, 0, 1, proofPtr, 10)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "读取proof失败应该返回错误")
}

// TestWASMAdapter_GetBlockHash_WriteFailed 测试写入哈希失败
func TestWASMAdapter_GetBlockHash_WriteFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	adapter.blockQuery = &mockBlockQuery{}
	adapter.hashManager = testutil.NewTestHashManager()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getBlockHash, ok := functions["get_block_hash"].(func(context.Context, api.Module, uint64, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	hashPtr := uint32(memSize + 100) // 超出范围

	result := getBlockHash(ctx, module, 100, hashPtr)
	assert.Equal(t, uint32(0), result, "写入哈希失败应该返回0")
}

// TestWASMAdapter_GetChainID_WriteFailed 测试写入链ID失败
func TestWASMAdapter_GetChainID_WriteFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getChainID, ok := functions["get_chain_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	chainIDPtr := uint32(memSize + 100) // 超出范围

	result := getChainID(ctx, module, chainIDPtr)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "写入链ID失败应该返回错误")
}

// TestWASMAdapter_GetContractAddress_WriteFailed 测试写入合约地址失败
func TestWASMAdapter_GetContractAddress_WriteFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getContractAddress, ok := functions["get_contract_address"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	addrPtr := uint32(memSize + 100) // 超出范围

	result := getContractAddress(ctx, module, addrPtr)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "写入合约地址失败应该返回错误")
}

// TestWASMAdapter_GetCaller_InvalidAddressLength 测试get_caller无效地址长度
func TestWASMAdapter_GetCaller_InvalidAddressLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.callerAddress = make([]byte, 19) // 19字节，不是20字节
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getCaller, ok := functions["get_caller"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	result := getCaller(ctx, module, addrPtr)
	// 🔧 **修复后**：返回 ErrInvalidAddress 而不是 0
	assert.Equal(t, uint32(ErrInvalidAddress), result, "无效地址长度应该返回 ErrInvalidAddress")
}

// TestWASMAdapter_GetCaller_WriteFailed 测试get_caller写入失败
func TestWASMAdapter_GetCaller_WriteFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getCaller, ok := functions["get_caller"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	addrPtr := uint32(memSize + 100) // 超出范围

	result := getCaller(ctx, module, addrPtr)
	// 🔧 **修复后**：返回 ErrInvalidParameter（内存越界）而不是 0
	assert.Equal(t, uint32(ErrInvalidParameter), result, "内存越界应该返回 ErrInvalidParameter")
}

// TestWASMAdapter_GetContractAddress_InvalidLength 测试get_contract_address无效长度
func TestWASMAdapter_GetContractAddress_InvalidLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.contractAddress = make([]byte, 19) // 19字节，不是20字节
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getContractAddress, ok := functions["get_contract_address"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	result := getContractAddress(ctx, module, addrPtr)
	assert.Equal(t, uint32(ErrInvalidAddress), result, "无效地址长度应该返回错误")
}

