package testutil

import (
	"time"

	"github.com/weisyn/v1/internal/core/block/builder"
	"github.com/weisyn/v1/internal/core/block/genesis"
	blockprocessor "github.com/weisyn/v1/internal/core/block/processor"
	"github.com/weisyn/v1/internal/core/block/validator"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== 辅助函数 ====================

// NewTestBlockBuilder 创建测试用的 BlockBuilder
func NewTestBlockBuilder() (*builder.Service, error) {
	storage := NewMockBadgerStore()
	mempool := NewMockTxPool()
	txProcessor := &MockTxProcessor{}
	hashManager := &MockHashManager{}
	blockHashClient := NewMockBlockHashClient()
	txHashClient := NewMockTransactionHashClient()
	queryService := NewMockQueryService()
	feeManager := &MockFeeManager{}
	logger := &MockLogger{}
	cfg := NewDefaultMockConfigProvider()

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		cfg,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return service.(*builder.Service), nil
}

// NewTestBlockValidator 创建测试用的 BlockValidator
func NewTestBlockValidator() (*validator.Service, error) {
	queryService := NewMockQueryService()
	// 预置一个“父区块”（key=全零哈希），避免多数测试用例因为找不到 parent block 而提前失败。
	// 这样可让测试聚焦在 PoW/哈希服务/规则本身，而不是 fixture 缺失。
	zeroHash := make([]byte, 32)
	queryService.SetBlock(zeroHash, &core.Block{
		Header: &core.BlockHeader{
			Height:       0,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    uint64(time.Now().Add(-time.Minute).Unix()),
			Difficulty:   1,
			Nonce:        make([]byte, 8),
		},
		Body: &core.BlockBody{},
	})
	hashManager := &MockHashManager{}
	blockHashClient := NewMockBlockHashClient()
	txHashClient := NewMockTransactionHashClient()
	txVerifier := NewMockTxVerifier()
	logger := &MockLogger{}
	cfg := NewDefaultMockConfigProvider()

	service, err := validator.NewService(
		queryService,
		hashManager,
		blockHashClient,
		txHashClient,
		txVerifier,
		cfg,
		nil, // eventBus 可选
		logger,
	)
	if err != nil {
		return nil, err
	}

	return service.(*validator.Service), nil
}

// NewTestBlockProcessor 创建测试用的 BlockProcessor
func NewTestBlockProcessor() (*blockprocessor.Service, error) {
	dataWriter := NewMockDataWriter()
	txProcessor := &MockTxProcessor{}
	utxoWriter := &MockUTXOWriter{}
	utxoQuery := NewMockQueryService()
	mempool := NewMockTxPool()
	hashManager := &MockHashManager{}
	blockHashClient := NewMockBlockHashClient()
	txHashClient := NewMockTransactionHashClient()
	zkProofService := NewMockZKProofService()
	eventBus := NewMockEventBus()
	logger := &MockLogger{}

	service, err := blockprocessor.NewService(
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
	if err != nil {
		return nil, err
	}

	return service.(*blockprocessor.Service), nil
}

// NewTestGenesisBuilder 创建测试用的 GenesisBuilder
func NewTestGenesisBuilder() (*genesis.Service, error) {
	txHashClient := NewMockTransactionHashClient()
	hashManager := &MockHashManager{}
	utxoQuery := NewMockQueryService()
	logger := &MockLogger{}

	service, err := genesis.NewService(
		txHashClient,
		hashManager,
		utxoQuery,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return service.(*genesis.Service), nil
}

// SetupChainTip 设置链尖数据到存储
func SetupChainTip(storage *MockBadgerStore, height uint64, blockHash []byte) {
	// 格式：height(8字节) + blockHash(32字节)
	tipData := make([]byte, 40)
	// 写入高度
	for i := 0; i < 8; i++ {
		tipData[i] = byte(height >> (56 - i*8))
	}
	// 写入区块哈希
	copy(tipData[8:], blockHash)
	storage.SetData([]byte("state:chain:tip"), tipData)
}
