// Package testutil 提供 Mempool 模块测试的区块辅助工具
//
// 🧪 **测试区块Fixtures**
//
// 本文件提供测试区块的创建函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"crypto/rand"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// randomHash 生成随机哈希（32字节）
func randomHash() []byte {
	hash := make([]byte, 32)
	rand.Read(hash)
	return hash
}

// CreateTestBlock 创建测试区块
//
// 参数：
// - height: 区块高度
// - prevHash: 前一个区块哈希（nil时自动生成）
// - txCount: 交易数量
//
// 返回：测试区块实例
func CreateTestBlock(height uint64, prevHash []byte, txCount int) *core.Block {
	if prevHash == nil {
		prevHash = randomHash() // 生成32字节的随机哈希
	}

	// 创建交易列表
	txs := make([]*transaction.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		txs[i] = CreateSimpleTestTransaction(i)
	}

	return &core.Block{
		Header: &core.BlockHeader{
			Height:       height,
			PreviousHash: prevHash,
			// 使用纳秒时间戳，避免测试中同秒创建多个区块导致哈希碰撞（MockBlockHashService 使用 height+timestamp 生成哈希）
			Timestamp:  uint64(time.Now().UnixNano()),
			Difficulty: 1,
		},
		Body: &core.BlockBody{
			Transactions: txs,
		},
	}
}

// CreateSimpleTestBlock 创建简单的测试区块（单个交易）
func CreateSimpleTestBlock(height uint64) *core.Block {
	return CreateTestBlock(height, nil, 1)
}

// CreateEmptyTestBlock 创建空区块（无交易）
func CreateEmptyTestBlock(height uint64) *core.Block {
	return CreateTestBlock(height, nil, 0)
}

// CreateTestBlockWithHash 创建指定哈希的测试区块
func CreateTestBlockWithHash(height uint64, blockHash []byte) *core.Block {
	block := CreateSimpleTestBlock(height)
	// 注意：实际哈希由哈希服务计算，这里只是设置一个标识
	return block
}
