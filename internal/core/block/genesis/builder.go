// Package genesis 创世区块构建实现
//
// 🎯 **创世区块专业构建**
//
// 本文件专门处理创世区块的构建逻辑，包括：
// - 创世区块头构建：设置特殊的创世区块头字段
// - Merkle根计算：使用创世交易计算Merkle根
// - 状态根处理：处理初始UTXO状态根
// - 创世参数设置：难度、时间戳、版本等特殊处理
//
// 🏗️ **设计原则**
// - 专业分工：专门处理创世区块构建业务逻辑
// - 配置驱动：完全基于GenesisConfig和创世交易
// - 确定性构建：相同输入产生相同的创世区块
// - 原子性操作：要么全部成功要么全部失败
package genesis

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/block/merkle"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// BuildBlock 构建创世区块
//
// 🎯 **创世区块构建服务**
//
// 基于创世交易和配置构建完整的创世区块，包括：
// 1. 构建创世区块头：设置特殊的创世区块头字段
// 2. 计算Merkle根：使用创世交易计算Merkle根
// 3. 设置创世参数：难度、时间戳、版本等
// 4. 计算状态根：基于初始UTXO状态
//
// 参数：
//   - ctx: 操作上下文
//   - genesisTransactions: 创世交易列表
//   - genesisConfig: 创世配置信息
//   - txHashClient: 交易哈希服务客户端
//   - hashManager: 哈希管理器（用于Merkle树）
//   - utxoQuery: UTXO查询服务（用于获取状态根）
//   - logger: 日志服务
//
// 返回：
//   - *core.Block: 构建完成的创世区块
//   - error: 构建过程中的错误
func BuildBlock(
	ctx context.Context,
	genesisTransactions []*transaction.Transaction,
	genesisConfig *types.GenesisConfig,
	txHashClient transaction.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	utxoQuery persistence.UTXOQuery,
	logger log.Logger,
) (*core.Block, error) {
	if logger != nil {
		logger.Infof("开始构建创世区块，交易数: %d", len(genesisTransactions))
	}

	if genesisConfig == nil {
		return nil, fmt.Errorf("创世配置不能为空")
	}

	if len(genesisTransactions) == 0 {
		return nil, fmt.Errorf("创世交易列表不能为空")
	}

	// 1. 计算Merkle根（使用统一交易哈希服务 + Merkle树）
	merkleRoot, err := calculateMerkleRoot(ctx, genesisTransactions, txHashClient, hashManager, logger)
	if err != nil {
		return nil, fmt.Errorf("计算创世区块Merkle根失败: %w", err)
	}

	// 2. 获取初始UTXO状态根（创世前应该是空状态）
	var stateRoot []byte
	if utxoQuery != nil {
		stateRoot, err = utxoQuery.GetCurrentStateRoot(ctx)
		if err != nil {
			if logger != nil {
				logger.Debugf("获取初始状态根失败，使用空状态根: %v", err)
			}
			stateRoot = make([]byte, 32) // 使用全零状态根
		}
	} else {
		stateRoot = make([]byte, 32) // 使用全零状态根
	}

	// 3. 构建创世区块头
	genesisHeader := &core.BlockHeader{
		ChainId:      genesisConfig.ChainID,     // ✅ 从配置获取链ID，防止跨链重放攻击
		Version:      1,                          // 协议版本
		PreviousHash: make([]byte, 32),           // 创世区块：父哈希为全零
		MerkleRoot:   merkleRoot,                 // 交易Merkle根
		Timestamp:    uint64(genesisConfig.Timestamp), // 使用配置中的时间戳
		Height:       0,                          // 创世区块高度为0
		Nonce:        make([]byte, 8),           // Nonce为空（创世区块无POW）
		Difficulty:   1,                          // 创世区块固定难度
		StateRoot:    stateRoot,                  // UTXO状态根
	}

	// 4. 构建创世区块体
	genesisBody := &core.BlockBody{
		Transactions: genesisTransactions,
	}

	// 5. 组装完整创世区块
	genesisBlock := &core.Block{
		Header: genesisHeader,
		Body:   genesisBody,
	}

	if logger != nil {
		logger.Infof("✅ 创世区块构建完成，高度: %d, 交易数: %d, Merkle根: %x",
			genesisBlock.Header.Height, len(genesisTransactions), merkleRoot)
	}

	return genesisBlock, nil
}

// ==================== 内部辅助函数 ====================

// calculateMerkleRoot 计算创世交易的Merkle根
func calculateMerkleRoot(
	ctx context.Context,
	transactions []*transaction.Transaction,
	txHashClient transaction.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	logger log.Logger,
) ([]byte, error) {
	if len(transactions) == 0 {
		return nil, fmt.Errorf("交易列表不能为空")
	}

	if txHashClient == nil {
		return nil, fmt.Errorf("交易哈希服务客户端不能为空")
	}

	if hashManager == nil {
		return nil, fmt.Errorf("哈希管理器不能为空")
	}

	// 提取交易哈希列表（通过统一哈希服务计算）
	txHashes := make([][]byte, 0, len(transactions))
	for i, tx := range transactions {
		if tx == nil {
			return nil, fmt.Errorf("交易[%d]不能为空", i)
		}
		req := &transaction.ComputeHashRequest{Transaction: tx, IncludeDebugInfo: false}
		resp, err := txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("计算交易[%d]哈希失败: %w", i, err)
		}
		if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
			return nil, fmt.Errorf("交易[%d]哈希无效", i)
		}
		txHashes = append(txHashes, resp.Hash)
	}

	// 使用Merkle树计算根哈希
	hasher := merkle.NewHashManagerAdapter(hashManager)
	merkleRoot, err := buildMerkleTree(hasher, txHashes)
	if err != nil {
		return nil, fmt.Errorf("计算Merkle根失败: %w", err)
	}

	if logger != nil {
		logger.Debugf("创世区块Merkle根计算完成: %x", merkleRoot)
	}

	return merkleRoot, nil
}

// buildMerkleTree 递归构建Merkle树（从merkle包复制，用于处理哈希列表）
func buildMerkleTree(hasher merkle.Hasher, hashes [][]byte) ([]byte, error) {
	// 🔧 与 BlockBuilder/PoWHandler/BlockValidator 保持一致
	// 如果节点数为奇数（包括1个），先复制
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 基础情况：2个节点配对后返回
	if len(hashes) == 2 {
		combined := append(hashes[0], hashes[1]...)
		parentHash, err := hasher.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}
		return parentHash, nil
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

