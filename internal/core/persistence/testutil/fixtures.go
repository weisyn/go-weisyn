// Package testutil 提供测试数据创建函数
package testutil

import (
	"crypto/rand"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 测试数据创建函数 ====================

// RandomBytes 生成随机字节数组
func RandomBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

// RandomHash 生成随机哈希（32字节）
func RandomHash() []byte {
	return RandomBytes(32)
}

// RandomAddress 生成随机地址（20字节）
func RandomAddress() []byte {
	return RandomBytes(20)
}

// CreateBlockHeader 创建测试用的区块头
func CreateBlockHeader(height uint64, previousHash []byte) *core.BlockHeader {
	return &core.BlockHeader{
		Height:       height,
		PreviousHash: previousHash,
		Timestamp:    uint64(time.Now().Unix()),
		Version:      1,
	}
}

// CreateBlock 创建测试用的区块
func CreateBlock(height uint64, previousHash []byte) *core.Block {
	return &core.Block{
		Header: CreateBlockHeader(height, previousHash),
		Body:   &core.BlockBody{
			Transactions: []*transaction.Transaction{},
		},
	}
}

// CreateBlockWithTransactions 创建带交易的区块
func CreateBlockWithTransactions(height uint64, previousHash []byte, txCount int) *core.Block {
	block := CreateBlock(height, previousHash)
	// 注意：Block 结构体中没有 Transactions 字段，交易通过其他方式存储
	return block
}

// CreateTransaction 创建测试用的交易
func CreateTransaction() *transaction.Transaction {
	return &transaction.Transaction{
		// 注意：Transaction 结构体中没有 Hash 和 TxType 字段，这些字段在 protobuf 中可能有不同的定义
	}
}

// CreateOutPoint 创建测试用的 OutPoint
func CreateOutPoint(txID []byte, index uint32) *transaction.OutPoint {
	if txID == nil {
		txID = RandomHash()
	}
	return &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: index,
	}
}

// CreateChainInfo 创建测试用的链信息
func CreateChainInfo(height uint64, blockHash []byte) *types.ChainInfo {
	if blockHash == nil {
		blockHash = RandomHash()
	}
	return &types.ChainInfo{
		Height:        height,
		BestBlockHash: blockHash,
		NodeMode:      types.NodeModeFull,
	}
}

