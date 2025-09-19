// Package genesis 创世区块验证实现
//
// 🎯 **创世区块专业验证**
//
// 本文件专门处理创世区块的验证逻辑，包括：
// - 创世区块结构验证：区块头和区块体的完整性
// - 创世特殊属性验证：高度为0、父哈希为全零等
// - Merkle根验证：验证交易Merkle根的正确性
// - 创世规则验证：跳过POW验证、跳过父区块检查等
//
// 🏗️ **设计原则**
// - 专业分工：专门处理创世区块验证业务逻辑
// - 严格验证：确保创世区块符合所有规则
// - 特殊处理：使用创世区块专用的验证规则
// - 明确错误：提供详细的验证失败信息
package genesis

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 协议定义
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 创世区块验证实现 ====================

// ValidateBlock 验证创世区块
//
// 🎯 **创世区块验证服务**
//
// 对创世区块进行专门验证，使用创世区块的特殊验证规则：
// 1. 结构验证：区块头和区块体的完整性
// 2. 创世特殊验证：高度为0、父哈希为全零
// 3. 交易验证：验证创世交易的有效性
// 4. Merkle根验证：验证交易Merkle根的正确性
// 5. 跳过POW验证：创世区块不需要工作量证明
// 6. 跳过父区块检查：创世区块没有父区块
//
// 参数：
//   - ctx: 操作上下文
//   - genesisBlock: 待验证的创世区块
//   - txHashServiceClient: 交易哈希服务客户端
//   - merkleTreeManager: Merkle树管理服务
//   - logger: 日志服务
//
// 返回：
//   - bool: 验证结果，true表示通过
//   - error: 验证过程中的错误
func ValidateBlock(
	ctx context.Context,
	genesisBlock *core.Block,
	txHashServiceClient transaction.TransactionHashServiceClient,
	merkleTreeManager crypto.MerkleTreeManager,
	logger log.Logger,
) (bool, error) {
	if logger != nil {
		logger.Infof("开始验证创世区块")
	}

	// 基础结构验证
	if genesisBlock == nil {
		return false, fmt.Errorf("创世区块不能为空")
	}

	if genesisBlock.Header == nil {
		return false, fmt.Errorf("创世区块头不能为空")
	}

	if genesisBlock.Body == nil {
		return false, fmt.Errorf("创世区块体不能为空")
	}

	// 验证创世区块特殊属性
	if genesisBlock.Header.Height != 0 {
		return false, fmt.Errorf("创世区块高度必须为0，当前为: %d", genesisBlock.Header.Height)
	}

	// 验证父区块哈希为全零
	if len(genesisBlock.Header.PreviousHash) != 32 {
		return false, fmt.Errorf("创世区块父哈希长度必须为32字节，当前为: %d", len(genesisBlock.Header.PreviousHash))
	}

	for i, b := range genesisBlock.Header.PreviousHash {
		if b != 0 {
			return false, fmt.Errorf("创世区块父哈希第%d字节必须为0，当前为: %02x", i, b)
		}
	}

	// 验证时间戳
	if genesisBlock.Header.Timestamp == 0 {
		return false, fmt.Errorf("创世区块时间戳不能为0")
	}

	// 验证交易列表
	if len(genesisBlock.Body.Transactions) == 0 {
		return false, fmt.Errorf("创世区块交易列表不能为空")
	}

	// 验证Merkle根（使用统一交易哈希服务 + MerkleTreeManager）
	valid, err := validateMerkleRoot(ctx, genesisBlock.Body.Transactions, genesisBlock.Header.MerkleRoot, txHashServiceClient, merkleTreeManager, logger)
	if err != nil {
		return false, fmt.Errorf("验证创世区块Merkle根失败: %w", err)
	}
	if !valid {
		return false, fmt.Errorf("创世区块Merkle根不匹配")
	}

	if logger != nil {
		logger.Infof("✅ 创世区块验证通过")
	}

	return true, nil
}

// ==================== 内部辅助函数 ====================

// validateMerkleRoot 验证创世区块的Merkle根
func validateMerkleRoot(
	ctx context.Context,
	transactions []*transaction.Transaction,
	expectedMerkleRoot []byte,
	txHashServiceClient transaction.TransactionHashServiceClient,
	merkleTreeManager crypto.MerkleTreeManager,
	logger log.Logger,
) (bool, error) {
	if len(transactions) == 0 {
		return false, fmt.Errorf("交易列表不能为空")
	}

	if len(expectedMerkleRoot) == 0 {
		return false, fmt.Errorf("期望的Merkle根不能为空")
	}

	if txHashServiceClient == nil {
		return false, fmt.Errorf("交易哈希服务客户端不能为空")
	}

	// 计算每个交易的哈希
	txHashes := make([][]byte, 0, len(transactions))
	for i, tx := range transactions {
		if tx == nil {
			return false, fmt.Errorf("交易[%d]不能为空", i)
		}
		req := &transaction.ComputeHashRequest{Transaction: tx, IncludeDebugInfo: false}
		resp, err := txHashServiceClient.ComputeHash(ctx, req)
		if err != nil {
			return false, fmt.Errorf("计算交易[%d]哈希失败: %w", i, err)
		}
		if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
			return false, fmt.Errorf("交易[%d]哈希无效", i)
		}
		txHashes = append(txHashes, resp.Hash)
	}

	// 使用Merkle树管理器计算根哈希
	merkleTree, err := merkleTreeManager.NewMerkleTree(txHashes)
	if err != nil {
		return false, fmt.Errorf("创建Merkle树失败: %w", err)
	}
	calculatedRoot := merkleTree.GetRoot()

	// 比较计算出的根哈希与期望的根哈希
	if len(calculatedRoot) != len(expectedMerkleRoot) {
		return false, fmt.Errorf("Merkle根长度不匹配: 计算值长度=%d, 期望值长度=%d",
			len(calculatedRoot), len(expectedMerkleRoot))
	}

	for i, b := range calculatedRoot {
		if b != expectedMerkleRoot[i] {
			return false, fmt.Errorf("Merkle根不匹配: 位置%d计算值=%02x, 期望值=%02x",
				i, b, expectedMerkleRoot[i])
		}
	}

	if logger != nil {
		logger.Debugf("创世区块Merkle根验证通过: %x", calculatedRoot)
	}

	return true, nil
}
