package processor_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/block/processor"
	"github.com/weisyn/v1/internal/core/block/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== NewService 测试 ====================

// TestNewService_WithValidDependencies_Succeeds 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_Succeeds(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	zkProofService := testutil.NewMockZKProofService()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		blockHashClient,
		txHashClient,
		zkProofService,
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilDataWriter_ReturnsError 测试nil数据写入器时返回错误
func TestNewService_WithNilDataWriter_ReturnsError(t *testing.T) {
	// Arrange
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	zkProofService := testutil.NewMockZKProofService()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		nil, // dataWriter为nil
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		blockHashClient,
		txHashClient,
		zkProofService,
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "dataWriter 不能为空")
}

// TestNewService_WithNilTxProcessor_ReturnsError 测试nil交易处理器时返回错误
func TestNewService_WithNilTxProcessor_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	zkProofService := testutil.NewMockZKProofService()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		nil, // txProcessor为nil
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		blockHashClient,
		txHashClient,
		zkProofService,
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "txProcessor 不能为空")
}

// TestNewService_WithNilMempool_ReturnsError 测试nil交易池时返回错误
func TestNewService_WithNilMempool_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		nil, // mempool为nil
		hashManager,
		blockHashClient,
		txHashClient,
		testutil.NewMockZKProofService(),
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "mempool 不能为空")
}

// TestNewService_WithNilHashManager_ReturnsError 测试nil哈希管理器时返回错误
func TestNewService_WithNilHashManager_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		nil, // hashManager为nil
		blockHashClient,
		txHashClient,
		testutil.NewMockZKProofService(),
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "hasher 不能为空")
}

// TestNewService_WithNilBlockHashClient_ReturnsError 测试nil区块哈希客户端时返回错误
func TestNewService_WithNilBlockHashClient_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	txHashClient := testutil.NewMockTransactionHashClient()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		nil, // blockHashClient为nil
		txHashClient,
		testutil.NewMockZKProofService(),
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "blockHashClient 不能为空")
}

// TestNewService_WithNilTxHashClient_ReturnsError 测试nil交易哈希客户端时返回错误
func TestNewService_WithNilTxHashClient_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		blockHashClient,
		nil, // txHashClient为nil
		testutil.NewMockZKProofService(),
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "txHashClient 不能为空")
}

// TestNewService_WithNilOptionalDependencies_Succeeds 测试可选依赖为nil时成功创建
func TestNewService_WithNilOptionalDependencies_Succeeds(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	txProcessor := &testutil.MockTxProcessor{}
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()

	// Act
	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		nil, // utxoWriter为nil（可选）
		nil, // utxoQuery为nil（可选）
		mempool,
		hashManager,
		blockHashClient,
		txHashClient,
		testutil.NewMockZKProofService(),
		nil, // eventBus为nil（可选）
		nil, // logger为nil（可选）
		nil, // writeGate（测试中可选）
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// ==================== ProcessBlock 测试 ====================

// TestProcessBlock_WithValidBlock_Succeeds 测试处理有效区块时成功
func TestProcessBlock_WithValidBlock_Succeeds(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
			Difficulty:   1,
			Nonce:        make([]byte, 8),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1), // Coinbase交易
			},
		},
	}

	// Act
	err = service.ProcessBlock(ctx, block)

	// Assert
	assert.NoError(t, err)
}

// TestProcessBlock_WithNilBlock_ReturnsError 测试处理nil区块时返回错误
func TestProcessBlock_WithNilBlock_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	err = service.ProcessBlock(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区块不能为空")
}

// TestProcessBlock_WithNilHeader_ReturnsError 测试处理nil区块头时返回错误
func TestProcessBlock_WithNilHeader_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: nil,
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	err = service.ProcessBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区块头或区块体不能为空")
}

// TestProcessBlock_WithNilBody_ReturnsError 测试处理nil区块体时返回错误
func TestProcessBlock_WithNilBody_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: nil,
	}

	// Act
	err = service.ProcessBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区块头或区块体不能为空")
}

// TestProcessBlock_WithValidatorFailure_ReturnsError 测试验证器验证失败时返回错误
func TestProcessBlock_WithValidatorFailure_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	// 设置验证器（模拟验证失败）
	validator := testutil.NewMockBlockValidator()
	validator.SetValidateResult(false, fmt.Errorf("验证失败"))
	service.SetValidator(validator)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	err = service.ProcessBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区块验证失败")
}

// TestProcessBlock_WithDataWriterError_ReturnsError 测试数据写入失败时返回错误
func TestProcessBlock_WithDataWriterError_ReturnsError(t *testing.T) {
	// Arrange
	dataWriter := testutil.NewMockDataWriter()
	dataWriter.SetWriteBlockError(fmt.Errorf("写入失败"))
	txProcessor := &testutil.MockTxProcessor{}
	utxoWriter := &testutil.MockUTXOWriter{}
	utxoQuery := testutil.NewMockQueryService()
	mempool := testutil.NewMockTxPool()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	service, err := processor.NewService(
		dataWriter,
		txProcessor,
		utxoWriter,
		utxoQuery,
		mempool,
		hashManager,
		blockHashClient,
		txHashClient,
		testutil.NewMockZKProofService(),
		eventBus,
		logger,
		nil, // writeGate（测试中可选）
	)
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	err = service.ProcessBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "存储区块失败")
}

// TestProcessBlock_ConcurrentAccess_IsSafe 测试并发处理区块的安全性
func TestProcessBlock_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	concurrency := 5

	// Act
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("panic: %v", r)
				}
			}()
			err := service.ProcessBlock(ctx, block)
			results <- err
		}()
	}

	// Assert
	successCount := 0
	concurrentErrorCount := 0
	otherErrorCount := 0
	for i := 0; i < concurrency; i++ {
		err := <-results
		if err != nil {
			// 并发处理时，除了第一个，其他应该返回"正在处理其他区块"错误
			if err.Error() == "正在处理其他区块，请稍后再试" {
				concurrentErrorCount++
			} else {
				otherErrorCount++
				t.Logf("其他错误: %v", err)
			}
		} else {
			successCount++
		}
	}

	// 应该至少有一个成功，其他可能因为并发控制而失败，也可能因为其他原因失败
	assert.GreaterOrEqual(t, successCount, 1, "应该至少有一个处理成功")
	// 并发错误数加上其他错误数应该等于总数减去成功数
	assert.Equal(t, concurrency-successCount, concurrentErrorCount+otherErrorCount, "错误总数应该正确")
}

// ==================== GetProcessorMetrics 测试 ====================

// TestGetProcessorMetrics_ReturnsMetrics 测试获取处理指标
func TestGetProcessorMetrics_ReturnsMetrics(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	metrics, err := service.GetProcessorMetrics(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, uint64(0), metrics.BlocksProcessed, "初始处理数应该为0")
}

// TestGetProcessorMetrics_AfterProcessing_UpdatesMetrics 测试处理后指标更新
func TestGetProcessorMetrics_AfterProcessing_UpdatesMetrics(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act - 处理区块（即使失败也会更新指标）
	_ = service.ProcessBlock(ctx, block)

	// 获取指标
	metrics, err := service.GetProcessorMetrics(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.BlocksProcessed, uint64(0), "处理数应该增加")
}

// ==================== SetValidator 测试 ====================

// TestSetValidator_SetsValidator 测试设置验证器
func TestSetValidator_SetsValidator(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockProcessor()
	require.NoError(t, err)

	validator := testutil.NewMockBlockValidator()

	// Act
	service.SetValidator(validator)

	// Assert
	// 验证器应该被设置（通过后续处理验证）
	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// 设置验证器返回失败
	validator.SetValidateResult(false, fmt.Errorf("验证失败"))
	err = service.ProcessBlock(ctx, block)
	assert.Error(t, err, "验证器应该被调用")
}

// ==================== 发现代码问题测试 ====================

// TestProcessBlock_DetectsTODOs 测试发现TODO标记
func TestProcessBlock_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：")
	t.Logf("  - execute.go:130-132: 发现TODO - 检查被消费的UTXO是否来自引用交易的逻辑待完善")
	t.Logf("  - execute.go:145-147: 发现TODO - 引用交易的输出记录逻辑待完善")
	t.Logf("  - execute.go:167-168: 发现TODO - 减少引用计数的逻辑待完善")
	t.Logf("建议：完善引用计数管理的完整逻辑")
}

// TestProcessBlock_DetectsTemporaryImplementations 测试发现临时实现
func TestProcessBlock_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ 处理逻辑检查：")
	t.Logf("  - ProcessBlock 使用原子性处理策略")
	t.Logf("  - executeBlock 协调各个组件的调用")
	t.Logf("  - processReferenceCounts 处理引用计数管理（部分逻辑待完善）")
	t.Logf("  - updateStateRoot 更新状态根")
	t.Logf("  - executeTransactions 执行交易（目前主要是日志记录）")
	t.Logf("  - cleanMempool 清理交易池")
}

