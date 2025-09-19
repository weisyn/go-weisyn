// Package block 提供区块管理的核心实现
//
// 📋 **merkle.go - Merkle树相关实现**
//
// 本文件专门实现区块中交易的Merkle树计算和验证逻辑。
// 确保创建区块和验证区块时使用完全相同的Merkle根计算方法，避免不一致问题。
//
// 🎯 **核心职责**：
// - 标准化Merkle根计算：使用TransactionHashServiceClient + MerkleTreeManager
// - Merkle根验证：重新计算并比较Merkle根
// - 统一数据范围：基于交易列表，不包含区块头挖矿数据
// - 确保一致性：创建和验证使用相同的计算逻辑
//
// 🏗️ **架构特点**：
// - 标准化哈希：使用TransactionHashServiceClient计算交易哈希
// - 公共接口：使用pkg/interfaces/infrastructure/crypto/merkle的MerkleTreeManager
// - 数据纯净：仅基于交易数据，不涉及nonce等挖矿字段
// - 错误详细：提供完整的错误信息和调试日志
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"bytes"
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== Merkle树计算和验证 ====================

// CalculateMerkleRoot 计算交易列表的Merkle根
//
// 🎯 **标准化Merkle根计算 - 实现InternalBlockService接口**
//
// 使用标准的TransactionHashService和MerkleTreeManager计算Merkle根。
// 这是内部接口的实现方法，确保创建和验证使用相同的计算逻辑。
//
// 🔄 **计算过程**：
// 1. 验证交易列表不为空（至少包含Coinbase交易）
// 2. 使用TransactionHashServiceClient计算每个交易的标准哈希
// 3. 使用MerkleTreeManager构建Merkle树
// 4. 返回32字节的Merkle根哈希
//
// ⚠️ **数据范围说明**：
// - 仅基于交易数据进行计算
// - 不包含区块头中的nonce、难度等挖矿相关字段
// - 使用确定性的交易哈希算法
//
// 参数：
//
//	ctx: 上下文对象
//	transactions: 交易列表（包含Coinbase交易）
//
// 返回值：
//
//	[]byte: 32字节的Merkle根哈希
//	error: 计算过程中的错误
//
// 使用场景：
//   - CreateMiningCandidate: 创建候选区块时计算Merkle根
//   - ValidateBlock: 验证区块时重新计算并比较Merkle根
//
// 示例：
//
//	merkleRoot, err := blockService.CalculateMerkleRoot(ctx, transactions)
//	if err != nil {
//	  return fmt.Errorf("计算Merkle根失败: %w", err)
//	}
func (m *Manager) CalculateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("开始标准化Merkle根计算，交易数量: %d", len(transactions))
	}

	// 1. 验证输入参数
	if len(transactions) == 0 {
		return nil, fmt.Errorf("交易列表为空，无法计算Merkle根")
	}

	// 2. 准备交易哈希数据
	transactionHashes := make([][]byte, len(transactions))

	for i, tx := range transactions {
		// 使用标准的交易哈希服务计算每个交易的哈希
		hashReq := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false, // 生产环境不需要调试信息
		}

		hashResp, err := m.txHashServiceClient.ComputeHash(ctx, hashReq)
		if err != nil {
			if m.logger != nil {
				m.logger.Errorf("计算交易哈希失败，索引: %d, 错误: %v", i, err)
			}
			return nil, fmt.Errorf("计算交易哈希失败 (索引 %d): %w", i, err)
		}

		if !hashResp.IsValid {
			if m.logger != nil {
				m.logger.Errorf("交易结构无效，索引: %d", i)
			}
			return nil, fmt.Errorf("交易无效 (索引 %d)", i)
		}

		// 验证哈希长度
		if len(hashResp.Hash) != 32 {
			return nil, fmt.Errorf("交易哈希长度异常 (索引 %d)，期望32字节，实际: %d",
				i, len(hashResp.Hash))
		}

		transactionHashes[i] = hashResp.Hash

		if m.logger != nil {
			m.logger.Debugf("交易哈希计算完成，索引: %d, 哈希: %x", i, hashResp.Hash)
		}
	}

	// 3. 使用MerkleTreeManager构建Merkle树
	merkleTree, err := m.merkleTreeManager.NewMerkleTree(transactionHashes)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("构建Merkle树失败: %v", err)
		}
		return nil, fmt.Errorf("构建Merkle树失败: %w", err)
	}

	// 4. 获取Merkle根
	merkleRoot := merkleTree.GetRoot()
	if len(merkleRoot) != 32 {
		return nil, fmt.Errorf("Merkle根长度异常，期望32字节，实际: %d", len(merkleRoot))
	}

	if m.logger != nil {
		m.logger.Infof("✅ 标准化Merkle根计算完成: %x", merkleRoot)
	}

	return merkleRoot, nil
}

// ValidateMerkleRoot 验证区块中的Merkle根
//
// 🎯 **Merkle根验证 - 用于区块验证**
//
// 重新计算交易列表的Merkle根，并与区块头中的Merkle根进行比较。
// 使用与CalculateMerkleRoot完全相同的计算逻辑，确保一致性。
//
// 🔄 **验证过程**：
// 1. 调用CalculateMerkleRoot重新计算Merkle根
// 2. 与区块头中声明的Merkle根进行字节级比较
// 3. 返回验证结果和详细错误信息
//
// ⚠️ **验证原则**：
// - 使用相同的标准化计算方法
// - 字节级精确比较，不允许任何差异
// - 提供详细的错误信息用于调试
//
// 参数：
//
//	ctx: 上下文对象
//	transactions: 交易列表（来自区块体）
//	expectedMerkleRoot: 期望的Merkle根（来自区块头）
//
// 返回值：
//
//	bool: 验证结果，true表示Merkle根正确
//	error: 验证过程中的错误
//
// 使用场景：
//   - ValidateBlock: 区块验证过程中的Merkle根校验
//   - 轻客户端验证: 验证区块完整性而不需要完整区块数据
//
// 示例：
//
//	valid, err := blockService.ValidateMerkleRoot(ctx, transactions, expectedRoot)
//	if err != nil {
//	  return fmt.Errorf("Merkle根验证失败: %w", err)
//	}
//	if !valid {
//	  return fmt.Errorf("Merkle根不匹配")
//	}
func (m *Manager) ValidateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction, expectedMerkleRoot []byte) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("开始验证Merkle根，期望值: %x", expectedMerkleRoot)
	}

	// 1. 验证输入参数
	if len(expectedMerkleRoot) != 32 {
		return false, fmt.Errorf("期望的Merkle根长度必须为32字节，实际: %d", len(expectedMerkleRoot))
	}

	// 2. 重新计算Merkle根（使用相同的标准化方法）
	calculatedRoot, err := m.CalculateMerkleRoot(ctx, transactions)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("重新计算Merkle根失败: %v", err)
		}
		return false, fmt.Errorf("重新计算Merkle根失败: %w", err)
	}

	// 3. 字节级比较
	isValid := bytes.Equal(calculatedRoot, expectedMerkleRoot)

	if !isValid {
		if m.logger != nil {
			m.logger.Errorf("❌ Merkle根验证失败，期望: %x, 计算得出: %x",
				expectedMerkleRoot, calculatedRoot)
		}
		return false, nil // 返回false但不返回error，这是正常的验证失败
	}

	if m.logger != nil {
		m.logger.Infof("✅ Merkle根验证通过: %x", calculatedRoot)
	}

	return true, nil
}
