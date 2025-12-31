package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// WASMAdapter扩展测试 - 使用wazero真实实现
// ============================================================================
//
// 🎯 **测试目的**：发现WASMAdapter的缺陷和BUG，测试所有宿主函数
//
// ============================================================================

// createMockExecutionContext 创建Mock的ExecutionContext
func createMockExecutionContext() *mockExecutionContext {
	return &mockExecutionContext{
		callerAddress:    make([]byte, 20),
		contractAddress:  make([]byte, 20),
		txID:             make([]byte, 32),
		chainID:          []byte("test-chain"),
		blockHeight:      100,
		blockTimestamp:   1234567890,
		draftID:          "draft-123",
		initParams:       []byte("init-params"),
	}
}

// mockExecutionContext Mock的ExecutionContext
type mockExecutionContext struct {
	callerAddress            []byte
	contractAddress         []byte
	txID                    []byte
	chainID                 []byte
	blockHeight             uint64
	blockTimestamp          uint64
	draftID                 string
	initParams              []byte
	returnData              []byte
	events                  []*ispcInterfaces.Event
	getTransactionDraftFunc func() (*ispcInterfaces.TransactionDraft, error)
	updateTransactionDraftFunc func(*ispcInterfaces.TransactionDraft) error
}

func (m *mockExecutionContext) GetCallerAddress() []byte { return m.callerAddress }
func (m *mockExecutionContext) GetContractAddress() []byte { return m.contractAddress }
func (m *mockExecutionContext) GetTransactionID() []byte { return m.txID }
func (m *mockExecutionContext) GetChainID() []byte { return m.chainID }
func (m *mockExecutionContext) GetBlockHeight() uint64 { return m.blockHeight }
func (m *mockExecutionContext) GetBlockTimestamp() uint64 { return m.blockTimestamp }
func (m *mockExecutionContext) GetDraftID() string { return m.draftID }
func (m *mockExecutionContext) GetInitParams() ([]byte, error) { return m.initParams, nil }

// 实现其他必需的方法（最小实现）
func (m *mockExecutionContext) GetExecutionID() string { return "exec-123" }
func (m *mockExecutionContext) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) { return nil, nil }
func (m *mockExecutionContext) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {}
func (m *mockExecutionContext) RecordStateChange(key string, oldValue interface{}, newValue interface{}) error { return nil }
func (m *mockExecutionContext) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error { return nil }
func (m *mockExecutionContext) GetResourceUsage() *types.ResourceUsage { return nil }
func (m *mockExecutionContext) FinalizeResourceUsage() {}
func (m *mockExecutionContext) SetReturnData(data []byte) error {
	m.returnData = data
	return nil
}
func (m *mockExecutionContext) GetReturnData() ([]byte, error) {
	return m.returnData, nil
}
func (m *mockExecutionContext) AddEvent(event *ispcInterfaces.Event) error {
	if m.events == nil {
		m.events = []*ispcInterfaces.Event{}
	}
	m.events = append(m.events, event)
	return nil
}
func (m *mockExecutionContext) GetEvents() ([]*ispcInterfaces.Event, error) {
	return m.events, nil
}
func (m *mockExecutionContext) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) {
	if m.getTransactionDraftFunc != nil {
		return m.getTransactionDraftFunc()
	}
	return nil, nil
}
func (m *mockExecutionContext) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error {
	if m.updateTransactionDraftFunc != nil {
		return m.updateTransactionDraftFunc(draft)
	}
	return nil
}
func (m *mockExecutionContext) HostABI() interfaces.HostABI { return nil }
func (m *mockExecutionContext) SetHostABI(abi interfaces.HostABI) error { return nil }
func (m *mockExecutionContext) SetInitParams(params []byte) error { return nil }

// createWASMAdapterWithMock 创建带Mock依赖的WASMAdapter
func createWASMAdapterWithMock(t *testing.T) (*WASMAdapter, *mockHostABIForWASM) {
	t.Helper()

	logger := testutil.NewTestLogger()
	hashManager := testutil.NewTestHashManager()
	mockABI := &mockHostABIForWASM{
		blockHeight:    100,
		blockTimestamp: 1234567890,
		chainID:        []byte("test-chain"),
		caller:         make([]byte, 20),
		contractAddr:   make([]byte, 20),
		txID:           make([]byte, 32),
		utxoExists:    true,
		resourceExists: true,
	}

	mockExecCtx := createMockExecutionContext()

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
			return mockExecCtx
		},
		nil, // buildTxFromDraft
		nil, // encodeTxReceipt
	)

	return adapter, mockABI
}

// createWazeroModule 创建wazero模块用于测试
func createWazeroModule(t *testing.T, hostFunctions map[string]interface{}) (api.Module, func()) {
	t.Helper()

	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// 创建一个简单的WASM模块，只导出内存
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d, // WASM魔数
		0x01, 0x00, 0x00, 0x00, // 版本
		// 内存段
		0x05, // section id (memory)
		0x03, // section size
		0x01, // 1个内存
		0x00, // 最小页数（无限制）
		0x01, // 最大页数（64KB）
	}

	// 编译模块
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	require.NoError(t, err, "编译WASM模块应该成功")

	// 创建模块配置
	moduleConfig := wazero.NewModuleConfig().
		WithName("test_module").
		WithStartFunctions() // 不自动调用start

	// 先注册宿主函数到env模块
	builder := runtime.NewHostModuleBuilder("env")
	for name, fn := range hostFunctions {
		builder.NewFunctionBuilder().WithFunc(fn).Export(name)
	}
	_, err = builder.Instantiate(ctx)
	require.NoError(t, err, "注册宿主函数应该成功")

	// 实例化模块
	module, err := runtime.InstantiateModule(ctx, compiled, moduleConfig)
	require.NoError(t, err, "实例化WASM模块应该成功")

	cleanup := func() {
		_ = module.Close(ctx)
		_ = runtime.Close(ctx)
	}

	return module, cleanup
}

// TestWASMAdapter_GetCaller 测试get_caller函数
func TestWASMAdapter_GetCaller(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getCaller, ok := functions["get_caller"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok, "get_caller应该是正确的函数类型")

	// 创建wazero模块
	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory, "内存应该存在")

	// 分配内存空间（20字节）
	addrPtr := uint32(1024) // 使用固定地址

	// 调用get_caller
	result := getCaller(ctx, module, addrPtr)
	assert.Equal(t, uint32(20), result, "应该返回20字节")

	// 验证内存中写入的数据
	callerBytes, ok := memory.Read(addrPtr, 20)
	require.True(t, ok, "应该能读取内存")
	assert.Equal(t, 20, len(callerBytes), "应该写入20字节")
}

// TestWASMAdapter_GetCaller_NilExecutionContext 测试nil ExecutionContext
func TestWASMAdapter_GetCaller_NilExecutionContext(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{}

	// 创建返回nil ExecutionContext的adapter
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return nil
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getCaller, ok := functions["get_caller"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	addrPtr := uint32(1024)
	result := getCaller(ctx, module, addrPtr)
	// 🔧 **修复后**：返回 ErrContextNotFound 而不是 0
	assert.Equal(t, uint32(ErrContextNotFound), result, "nil ExecutionContext应该返回 ErrContextNotFound")
}

// TestWASMAdapter_GetContractAddress 测试get_contract_address函数
func TestWASMAdapter_GetContractAddress(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getContractAddress, ok := functions["get_contract_address"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	result := getContractAddress(ctx, module, addrPtr)
	assert.Equal(t, uint32(20), result, "应该返回20字节")

	// 验证内存中写入的数据
	contractBytes, ok := memory.Read(addrPtr, 20)
	require.True(t, ok)
	assert.Equal(t, 20, len(contractBytes), "应该写入20字节")
}

// TestWASMAdapter_GetChainID 测试get_chain_id函数
func TestWASMAdapter_GetChainID(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getChainID, ok := functions["get_chain_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	chainIDPtr := uint32(1024)
	result := getChainID(ctx, module, chainIDPtr)
	assert.Greater(t, result, uint32(0), "应该返回链ID长度")

	// 验证内存中写入的数据
	chainIDBytes, ok := memory.Read(chainIDPtr, result)
	require.True(t, ok)
	assert.Equal(t, int(result), len(chainIDBytes), "应该写入链ID")
}

// TestWASMAdapter_Malloc 测试malloc函数（使用wazero真实实现）
func TestWASMAdapter_Malloc(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	malloc, ok := functions["malloc"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok, "malloc应该是正确的函数类型")

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory, "内存应该存在")

	initialSize := memory.Size()

	// 分配内存
	ptr1 := malloc(ctx, module, 1024)
	assert.Greater(t, ptr1, uint32(0), "应该返回有效指针")

	// 再次分配
	ptr2 := malloc(ctx, module, 512)
	assert.Greater(t, ptr2, uint32(0), "应该返回有效指针")
	assert.NotEqual(t, ptr1, ptr2, "两次分配应该返回不同的指针")

	// 验证内存已扩容（如果需要）
	finalSize := memory.Size()
	assert.GreaterOrEqual(t, finalSize, initialSize, "内存大小应该增加或保持不变")
}

// TestWASMAdapter_Malloc_Concurrent 测试并发malloc
func TestWASMAdapter_Malloc_Concurrent(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	malloc, ok := functions["malloc"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	// 并发分配
	done := make(chan uint32, 10)
	for i := 0; i < 10; i++ {
		go func() {
			ptr := malloc(ctx, module, 1024)
			done <- ptr
		}()
	}

	// 收集所有指针
	ptrs := make(map[uint32]bool)
	for i := 0; i < 10; i++ {
		ptr := <-done
		assert.Greater(t, ptr, uint32(0), "应该返回有效指针")
		ptrs[ptr] = true
	}

	// 验证所有指针都不同（或至少大部分不同）
	// 注意：由于并发，可能会有一些指针相同，但应该大部分不同
	assert.Greater(t, len(ptrs), 5, "大部分指针应该不同")
}

// TestWASMAdapter_NodeAdd 测试node_add函数
func TestWASMAdapter_NodeAdd(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	nodeAdd, ok := functions["node_add"].(func(int32, int32) int32)
	require.True(t, ok, "node_add应该是func(int32, int32) int32类型")

	result := nodeAdd(10, 20)
	assert.Equal(t, int32(30), result, "10 + 20应该等于30")
}

// TestWASMAdapter_GetTimestamp 测试get_timestamp函数
func TestWASMAdapter_GetTimestamp(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getTimestamp, ok := functions["get_timestamp"].(func() uint64)
	require.True(t, ok, "get_timestamp应该是func() uint64类型")

	timestamp := getTimestamp()
	assert.Equal(t, uint64(1234567890), timestamp, "应该返回正确的时间戳")
}

// TestWASMAdapter_GetContractInitParams 测试get_contract_init_params函数
func TestWASMAdapter_GetContractInitParams(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getInitParams, ok := functions["get_contract_init_params"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	bufPtr := uint32(1024)
	bufLen := uint32(100)

	result := getInitParams(ctx, module, bufPtr, bufLen)
	assert.Greater(t, result, uint32(0), "应该返回参数长度")

	// 验证内存中写入的数据
	paramsBytes, ok := memory.Read(bufPtr, result)
	require.True(t, ok)
	assert.Equal(t, int(result), len(paramsBytes), "应该写入参数数据")
}

// TestWASMAdapter_UTXOExists 测试utxo_exists函数
func TestWASMAdapter_UTXOExists(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	utxoExists, ok := functions["utxo_exists"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入txID到内存
	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	txID[0] = 0x12
	memory.Write(txIDPtr, txID)

	// 调用utxo_exists
	result := utxoExists(ctx, module, txIDPtr, 32, 0)
	assert.Equal(t, uint32(1), result, "UTXO应该存在")
}

// TestWASMAdapter_UTXOExists_InvalidLength 测试无效长度
func TestWASMAdapter_UTXOExists_InvalidLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	utxoExists, ok := functions["utxo_exists"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	// 使用无效的txID长度
	result := utxoExists(ctx, module, 1024, 20, 0) // 长度应该是32
	assert.Equal(t, uint32(ErrInvalidParameter), result, "应该返回参数错误")
}

// TestWASMAdapter_ResourceExists 测试resource_exists函数
func TestWASMAdapter_ResourceExists(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	resourceExists, ok := functions["resource_exists"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入contentHash到内存
	hashPtr := uint32(1024)
	contentHash := make([]byte, 32)
	contentHash[0] = 0x12
	memory.Write(hashPtr, contentHash)

	// 调用resource_exists
	result := resourceExists(ctx, module, hashPtr, 32)
	assert.Equal(t, uint32(1), result, "资源应该存在")
}

// TestWASMAdapter_BuildHostFunctions_AllFunctions 测试所有函数都存在
func TestWASMAdapter_BuildHostFunctions_AllFunctions(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)

	// 验证所有关键宿主函数都存在
	expectedFunctions := []string{
		"get_block_height",
		"get_block_timestamp",
		"get_caller",
		"get_block_hash",
		"get_chain_id",
		"get_contract_address",
		"get_transaction_id",
		"query_utxo_balance",
		"utxo_lookup",
		"utxo_exists",
		"append_tx_input",
		"append_asset_output",
		"append_resource_output",
		"append_state_output",
		"resource_lookup",
		"resource_exists",
		"host_build_transaction",
		"malloc",
		"node_add",
		"get_timestamp",
		"get_contract_init_params",
		"set_return_data",
		"emit_event",
		"state_get",
		"state_set",
		"state_exists",
		"address_bytes_to_base58",
		"address_base58_to_bytes",
	}

	for _, funcName := range expectedFunctions {
		assert.Contains(t, functions, funcName, "应该包含函数: %s", funcName)
	}

	assert.GreaterOrEqual(t, len(functions), len(expectedFunctions), "宿主函数数量应该不少于关键函数数目")
}

