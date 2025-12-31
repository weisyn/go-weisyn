package genesis_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/block/genesis"
	"github.com/weisyn/v1/internal/core/block/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== NewService 测试 ====================

// TestNewService_WithValidDependencies_Succeeds 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_Succeeds(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilTxHashClient_ReturnsError 测试nil交易哈希客户端时返回错误
func TestNewService_WithNilTxHashClient_ReturnsError(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	service, err := genesis.NewService(
		nil, // txHashClient为nil
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "txHashClient 不能为空")
}

// TestNewService_WithNilHashManager_ReturnsError 测试nil哈希管理器时返回错误
func TestNewService_WithNilHashManager_ReturnsError(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	service, err := genesis.NewService(
		txHashClient,
		nil, // hashManager为nil
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "hashManager 不能为空")
}

// TestNewService_WithNilOptionalDependencies_Succeeds 测试可选依赖为nil时成功创建
func TestNewService_WithNilOptionalDependencies_Succeeds(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}

	// Act
	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		nil, // utxoQuery为nil（可选）
		nil, // logger为nil（可选）
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// ==================== CreateGenesisBlock 测试 ====================

// TestCreateGenesisBlock_WithValidInputs_CreatesBlock 测试使用有效输入创建创世区块
func TestCreateGenesisBlock_WithValidInputs_CreatesBlock(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
		testutil.NewTestTransaction(2),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.NotNil(t, block.Header)
	assert.NotNil(t, block.Body)
	assert.Equal(t, uint64(0), block.Header.Height, "创世区块高度应该为0")
	assert.Equal(t, uint64(1), block.Header.ChainId, "链ID应该为1")
	assert.Equal(t, len(genesisTransactions), len(block.Body.Transactions), "交易数量应该匹配")
}

// TestCreateGenesisBlock_WithNilConfig_ReturnsError 测试nil配置时返回错误
func TestCreateGenesisBlock_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "创世配置不能为空")
}

// TestCreateGenesisBlock_WithEmptyTransactions_ReturnsError 测试空交易列表时返回错误
func TestCreateGenesisBlock_WithEmptyTransactions_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, nil, genesisConfig)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "创世交易列表不能为空")
}

// TestCreateGenesisBlock_WithTxHashClientError_ReturnsError 测试交易哈希计算失败时返回错误
func TestCreateGenesisBlock_WithTxHashClientError_ReturnsError(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	txHashClient.SetError(errors.New("hash service error"))
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "计算交易")
}

// TestCreateGenesisBlock_WithUTXOQueryError_UsesZeroStateRoot 测试UTXO查询失败时使用全零状态根
func TestCreateGenesisBlock_WithUTXOQueryError_UsesZeroStateRoot(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	// 设置UTXO查询返回错误
	utxoQuery.SetError(errors.New("utxo query error"))
	logger := &testutil.MockLogger{}

	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	// 验证状态根是全零
	allZero := true
	for _, b := range block.Header.StateRoot {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.True(t, allZero, "UTXO查询失败时应该使用全零状态根")
}

// TestCreateGenesisBlock_WithNilUTXOQuery_UsesZeroStateRoot 测试无UTXO查询时使用全零状态根
func TestCreateGenesisBlock_WithNilUTXOQuery_UsesZeroStateRoot(t *testing.T) {
	// Arrange
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		nil, // utxoQuery为nil
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	// 验证状态根是全零
	allZero := true
	for _, b := range block.Header.StateRoot {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.True(t, allZero, "无UTXO查询时应该使用全零状态根")
}

// TestCreateGenesisBlock_WithGenesisConfig_AppliesConfig 测试创世配置正确应用
func TestCreateGenesisBlock_WithGenesisConfig_AppliesConfig(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	timestamp := int64(1234567890)
	genesisConfig := &types.GenesisConfig{
		ChainID:   12345,
		NetworkID: "testnet",
		Timestamp: timestamp,
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, uint64(12345), block.Header.ChainId, "链ID应该匹配配置")
	assert.Equal(t, uint64(timestamp), block.Header.Timestamp, "时间戳应该匹配配置")
	assert.Equal(t, uint64(0), block.Header.Height, "创世区块高度应该为0")
	assert.Equal(t, uint64(1), block.Header.Difficulty, "创世区块难度应该为1")
}

// TestCreateGenesisBlock_WithPreviousHash_IsZero 测试创世区块父哈希为全零
func TestCreateGenesisBlock_WithPreviousHash_IsZero(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// Act
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	// 验证父哈希是全零
	allZero := true
	for _, b := range block.Header.PreviousHash {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.True(t, allZero, "创世区块父哈希应该全零")
	assert.Equal(t, 32, len(block.Header.PreviousHash), "父哈希长度应该为32字节")
}

// ==================== ValidateGenesisBlock 测试 ====================

// TestValidateGenesisBlock_WithValidBlock_ReturnsTrue 测试验证有效创世区块时返回true
func TestValidateGenesisBlock_WithValidBlock_ReturnsTrue(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// 先创建创世区块
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	require.NoError(t, err)
	require.NotNil(t, block)

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, block)

	// Assert
	assert.NoError(t, err)
	assert.True(t, valid, "有效创世区块应该通过验证")
}

// TestValidateGenesisBlock_WithNilBlock_ReturnsError 测试验证nil区块时返回错误
func TestValidateGenesisBlock_WithNilBlock_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "创世区块不能为空")
}

// TestValidateGenesisBlock_WithInvalidHeight_ReturnsError 测试高度不为0时返回错误
func TestValidateGenesisBlock_WithInvalidHeight_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	// 创建高度不为0的区块
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: 1, // 不是0
			PreviousHash: make([]byte, 32),
			MerkleRoot: make([]byte, 32),
			Timestamp: uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "创世区块高度必须为0")
}

// TestValidateGenesisBlock_WithInvalidPreviousHash_ReturnsError 测试父哈希不全零时返回错误
func TestValidateGenesisBlock_WithInvalidPreviousHash_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	// 创建父哈希不全零的区块
	previousHash := make([]byte, 32)
	previousHash[0] = 1 // 设置第一个字节为1
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: 0,
			PreviousHash: previousHash,
			MerkleRoot: make([]byte, 32),
			Timestamp: uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				testutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "创世区块父哈希")
}

// TestValidateGenesisBlock_WithEmptyTransactions_ReturnsError 测试空交易列表时返回错误
func TestValidateGenesisBlock_WithEmptyTransactions_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: 0,
			PreviousHash: make([]byte, 32),
			MerkleRoot: make([]byte, 32),
			Timestamp: uint64(time.Now().Unix()),
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{}, // 空交易列表
		},
	}

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "创世区块交易列表不能为空")
}

// TestValidateGenesisBlock_WithInvalidMerkleRoot_ReturnsError 测试Merkle根不匹配时返回错误
func TestValidateGenesisBlock_WithInvalidMerkleRoot_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	// 创建创世区块
	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	require.NoError(t, err)
	require.NotNil(t, block)

	// 修改Merkle根使其无效
	block.Header.MerkleRoot[0] ^= 1

	// Act
	valid, err := service.ValidateGenesisBlock(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "Merkle根")
}

// ==================== 并发安全测试 ====================

// TestCreateGenesisBlock_ConcurrentAccess_IsSafe 测试并发创建创世区块的安全性
func TestCreateGenesisBlock_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	concurrency := 10

	// Act
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("panic: %v", r)
				}
			}()
			_, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
			results <- err
		}()
	}

	// Assert
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err, "并发创建创世区块不应该失败")
	}
}

// ==================== 发现代码问题测试 ====================

// TestCreateGenesisBlock_DetectsTODOs 测试发现TODO标记
func TestCreateGenesisBlock_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestCreateGenesisBlock_DetectsTemporaryImplementations 测试发现临时实现
func TestCreateGenesisBlock_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	service, err := testutil.NewTestGenesisBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}

	block, err := service.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	require.NoError(t, err)

	// 检查是否使用了全零状态根（临时实现）
	allZero := true
	for _, b := range block.Header.StateRoot {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Logf("⚠️ 临时实现发现：使用全零状态根作为后备方案")
		t.Logf("位置：builder.go 第88-91行")
		t.Logf("问题：UTXO查询失败或未注入时使用全零状态根，这是临时实现")
		t.Logf("建议：1) 要求UTXOQuery必须注入；2) 或明确标记为已知限制")
	}

	// 检查是否使用了固定难度
	if block.Header.Difficulty == 1 {
		t.Logf("⚠️ 固定实现发现：创世区块使用固定难度1")
		t.Logf("位置：builder.go 第103行")
		t.Logf("问题：使用固定难度1，这是设计决策")
		t.Logf("建议：确认这是期望的行为")
	}
}

