// Package merkle 提供Merkle树计算和验证功能
//
// 🎯 **标准Merkle树实现**
//
// Merkle树是一种哈希树，用于高效验证数据集的完整性。
// 特点：
// - 叶子节点：交易哈希
// - 非叶子节点：子节点哈希的哈希
// - 根节点：Merkle根
package merkle

import (
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

// Hasher 定义简化的哈希接口
//
// 这是一个适配器接口，用于统一不同的哈希实现。
// 它提供了一个简单的 Hash 方法，返回32字节的哈希值。
type Hasher interface {
	// Hash 计算数据的哈希值
	//
	// 参数：
	//   - data: 输入数据
	//
	// 返回：
	//   - []byte: 哈希值（通常是32字节）
	//   - error: 计算错误
	Hash(data []byte) ([]byte, error)
}

// CalculateMerkleRoot 计算交易列表的Merkle根
//
// 🎯 **标准Merkle树实现**
//
// 算法：
// 1. 计算所有交易的哈希作为叶子节点
// 2. 两两配对，计算父节点哈希
// 3. 重复步骤2，直到只剩一个根节点
// 4. 如果节点数为奇数，复制最后一个节点
//
// 参数：
//   - hasher: 哈希服务
//   - transactions: 交易列表
//
// 返回：
//   - []byte: 32字节Merkle根
//   - error: 计算错误
func CalculateMerkleRoot(hasher Hasher, transactions []*transaction.Transaction) ([]byte, error) {
	if hasher == nil {
		return nil, fmt.Errorf("hasher 不能为空")
	}
	if len(transactions) == 0 {
		return nil, fmt.Errorf("交易列表不能为空")
	}

	// 1. 计算所有交易的哈希（叶子节点）
	hashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		// 直接使用Hasher接口计算交易哈希
		txHash, err := calculateTransactionHash(hasher, tx)
		if err != nil {
			return nil, fmt.Errorf("计算交易%d哈希失败: %w", i, err)
		}
		hashes[i] = txHash
	}

	// 2. 构建Merkle树
	return buildMerkleTree(hasher, hashes)
}

// calculateTransactionHash 计算交易哈希（内部辅助函数）
// 直接使用Hasher接口，不依赖外部工具函数
func calculateTransactionHash(hasher Hasher, tx *transaction.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易不能为空")
	}

	// 序列化交易
	data, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 计算哈希
	hash, err := hasher.Hash(data)
	if err != nil {
		return nil, fmt.Errorf("计算哈希失败: %w", err)
	}

	if len(hash) != 32 {
		return nil, fmt.Errorf("哈希长度错误: 期望32字节, 得到%d字节", len(hash))
	}

	return hash, nil
}

// buildMerkleTree 递归构建Merkle树
func buildMerkleTree(hasher Hasher, hashes [][]byte) ([]byte, error) {
	// 基础情况：只有一个节点，返回该节点
	if len(hashes) == 1 {
		return hashes[0], nil
	}

	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 计算下一层节点
	nextLevel := make([][]byte, 0, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		// 连接两个子节点的哈希
		combined := append(hashes[i], hashes[i+1]...)

		// 计算父节点哈希
		parentHash, err := hasher.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return buildMerkleTree(hasher, nextLevel)
}

// VerifyMerkleProof 验证Merkle证明
//
// 🎯 **Merkle证明验证**
//
// 用于验证某个交易是否在区块中，而无需下载整个区块。
//
// 参数：
//   - hasher: 哈希服务
//   - txHash: 交易哈希
//   - merkleRoot: Merkle根
//   - proof: Merkle证明路径
//   - index: 交易在区块中的索引
//
// 返回：
//   - bool: 验证结果
//   - error: 验证错误
func VerifyMerkleProof(
	hasher Hasher,
	txHash []byte,
	merkleRoot []byte,
	proof [][]byte,
	index int,
) (bool, error) {
	if hasher == nil {
		return false, fmt.Errorf("hasher 不能为空")
	}
	if len(txHash) != 32 {
		return false, fmt.Errorf("交易哈希长度错误")
	}
	if len(merkleRoot) != 32 {
		return false, fmt.Errorf("Merkle根长度错误")
	}

	// 从叶子节点开始，逐层向上计算
	currentHash := txHash
	currentIndex := index

	for _, siblingHash := range proof {
		if len(siblingHash) != 32 {
			return false, fmt.Errorf("证明哈希长度错误")
		}

		var combined []byte
		if currentIndex%2 == 0 {
			// 当前节点在左边
			combined = append(currentHash, siblingHash...)
		} else {
			// 当前节点在右边
			combined = append(siblingHash, currentHash...)
		}

		// 计算父节点哈希
		parentHash, err := hasher.Hash(combined)
		if err != nil {
			return false, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		currentHash = parentHash
		currentIndex = currentIndex / 2
	}

	// 比较计算出的根哈希与给定的Merkle根
	if len(currentHash) != len(merkleRoot) {
		return false, nil
	}

	for i := range currentHash {
		if currentHash[i] != merkleRoot[i] {
			return false, nil
		}
	}

	return true, nil
}

