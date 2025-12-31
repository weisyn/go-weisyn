package testutil

import (
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 测试数据 Fixtures ====================

// NewTestBlock 创建测试区块
func NewTestBlock(height uint64, parentHash []byte) *core.Block {
	if parentHash == nil {
		parentHash = make([]byte, 32)
	}

	// 创建Coinbase交易
	coinbaseTx := &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{},
		Outputs:           []*transaction.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte{0, 0, 0, 0, 0, 0, 0, 1},
	}

	// 创建区块头
	header := &core.BlockHeader{
		ChainId:      1,
		Version:      1,
		PreviousHash: parentHash,
		MerkleRoot:   make([]byte, 32),
		Timestamp:    uint64(time.Now().Unix()),
		Height:       height,
		Nonce:        make([]byte, 8),
		Difficulty:   1,
		StateRoot:    make([]byte, 32),
	}

	// 创建区块体
	body := &core.BlockBody{
		Transactions: []*transaction.Transaction{coinbaseTx},
	}

	return &core.Block{
		Header: header,
		Body:   body,
	}
}

// NewTestTransaction 创建测试交易
func NewTestTransaction(nonce uint64) *transaction.Transaction {
	return &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{},
		Outputs:           []*transaction.TxOutput{},
		Nonce:             nonce,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte{0, 0, 0, 0, 0, 0, 0, 1},
	}
}

// NewTestGenesisConfig 创建测试创世配置
func NewTestGenesisConfig() *types.GenesisConfig {
	return &types.GenesisConfig{
		ChainID:   1,
		Timestamp: time.Now().Unix(),
	}
}

// NewTestGenesisTransactions 创建测试创世交易列表
func NewTestGenesisTransactions() []*transaction.Transaction {
	return []*transaction.Transaction{
		NewTestTransaction(0),
	}
}

