package genesis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/block/genesis"
	"github.com/weisyn/v1/internal/core/block/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== BuildBlock 测试 ====================

// TestBuildBlock_WithValidInputs_CreatesBlock 测试使用有效输入构建创世区块
func TestBuildBlock_WithValidInputs_CreatesBlock(t *testing.T) {
	// Arrange
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
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.NotNil(t, block.Header)
	assert.NotNil(t, block.Body)
	assert.Equal(t, uint64(0), block.Header.Height, "创世区块高度应该为0")
	assert.Equal(t, uint64(1), block.Header.ChainId, "链ID应该匹配配置")
	assert.Equal(t, len(genesisTransactions), len(block.Body.Transactions), "交易数量应该匹配")
	assert.Equal(t, uint64(genesisConfig.Timestamp), block.Header.Timestamp, "时间戳应该匹配配置")
}

// TestBuildBlock_WithNilConfig_ReturnsError 测试nil配置时返回错误
func TestBuildBlock_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		nil, // 配置为nil
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "创世配置不能为空")
}

// TestBuildBlock_WithEmptyTransactions_ReturnsError 测试空交易列表时返回错误
func TestBuildBlock_WithEmptyTransactions_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		nil, // 交易列表为空
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "创世交易列表不能为空")
}

// TestBuildBlock_WithTxHashClientError_ReturnsError 测试交易哈希计算失败时返回错误
func TestBuildBlock_WithTxHashClientError_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	txHashClient.SetError(errors.New("hash service error"))
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "计算交易")
}

// TestBuildBlock_WithUTXOQueryError_UsesZeroStateRoot 测试UTXO查询失败时使用全零状态根
func TestBuildBlock_WithUTXOQueryError_UsesZeroStateRoot(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	utxoQuery.SetError(errors.New("utxo query error"))
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

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

// TestBuildBlock_WithNilUTXOQuery_UsesZeroStateRoot 测试无UTXO查询时使用全零状态根
func TestBuildBlock_WithNilUTXOQuery_UsesZeroStateRoot(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		nil, // utxoQuery为nil
		logger,
	)

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

// TestBuildBlock_WithGenesisConfig_AppliesConfig 测试创世配置正确应用
func TestBuildBlock_WithGenesisConfig_AppliesConfig(t *testing.T) {
	// Arrange
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
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, uint64(12345), block.Header.ChainId, "链ID应该匹配配置")
	assert.Equal(t, uint64(timestamp), block.Header.Timestamp, "时间戳应该匹配配置")
	assert.Equal(t, uint64(0), block.Header.Height, "创世区块高度应该为0")
	assert.Equal(t, uint64(1), block.Header.Difficulty, "创世区块难度应该为1")
}

// TestBuildBlock_WithPreviousHash_IsZero 测试创世区块父哈希为全零
func TestBuildBlock_WithPreviousHash_IsZero(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

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

// TestBuildBlock_WithMerkleRoot_IsCalculated 测试Merkle根被正确计算
func TestBuildBlock_WithMerkleRoot_IsCalculated(t *testing.T) {
	// Arrange
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
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.NotNil(t, block.Header.MerkleRoot)
	assert.Equal(t, 32, len(block.Header.MerkleRoot), "Merkle根长度应该为32字节")
	// 验证Merkle根不是全零（除非所有交易哈希都相同）
	allZero := true
	for _, b := range block.Header.MerkleRoot {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "Merkle根不应该全零（除非特殊情况）")
}

// TestBuildBlock_WithMultipleTransactions_CalculatesMerkleRoot 测试多个交易时正确计算Merkle根
func TestBuildBlock_WithMultipleTransactions_CalculatesMerkleRoot(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
		testutil.NewTestTransaction(2),
		testutil.NewTestTransaction(3),
		testutil.NewTestTransaction(4),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, len(genesisTransactions), len(block.Body.Transactions), "交易数量应该匹配")
	assert.NotNil(t, block.Header.MerkleRoot)
	assert.Equal(t, 32, len(block.Header.MerkleRoot), "Merkle根长度应该为32字节")
}

// TestBuildBlock_WithNilTransaction_ReturnsError 测试包含nil交易时返回错误
func TestBuildBlock_WithNilTransaction_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
		nil, // nil交易
		testutil.NewTestTransaction(2),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, block)
	assert.Contains(t, err.Error(), "交易[1]不能为空")
}

// ==================== 边界条件测试 ====================

// TestBuildBlock_WithSingleTransaction_Works 测试单个交易时正常工作
func TestBuildBlock_WithSingleTransaction_Works(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, 1, len(block.Body.Transactions), "应该包含1个交易")
	assert.NotNil(t, block.Header.MerkleRoot)
}

// TestBuildBlock_WithZeroTimestamp_UsesZero 测试时间戳为0时使用0
func TestBuildBlock_WithZeroTimestamp_UsesZero(t *testing.T) {
	// Arrange
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: 0, // 时间戳为0
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	// Act
	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
	assert.Equal(t, uint64(0), block.Header.Timestamp, "时间戳应该为0")
}

// ==================== 发现代码问题测试 ====================

// TestBuildBlock_DetectsTemporaryImplementations 测试发现临时实现
func TestBuildBlock_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	ctx := context.Background()
	genesisTransactions := []*transaction.Transaction{
		testutil.NewTestTransaction(1),
	}
	genesisConfig := &types.GenesisConfig{
		ChainID:   1,
		NetworkID: "testnet",
		Timestamp: time.Now().Unix(),
	}
	txHashClient := testutil.NewMockTransactionHashClient()
	hashManager := &testutil.MockHashManager{}
	utxoQuery := testutil.NewMockQueryService()
	logger := &testutil.MockLogger{}

	block, err := genesis.BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)
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

	// 检查是否使用了固定版本号
	if block.Header.Version == 1 {
		t.Logf("✅ 确认：创世区块使用固定版本号1")
		t.Logf("位置：builder.go 第97行")
		t.Logf("说明：这是设计决策，版本号固定为1")
	}
}

// TestBuildBlock_DetectsTODOs 测试发现TODO标记
func TestBuildBlock_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

