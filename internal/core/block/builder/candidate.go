// Package builder 实现区块构建服务
package builder

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/weisyn/v1/internal/core/block/difficulty"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// buildCandidate 构建候选区块
//
// 🎯 **候选区块构建核心逻辑**
//
// 完整构建流程：
// 1. 构建Coinbase交易（激励交易）
// 2. 组装完整交易列表
// 3. 构建区块头
// 4. 组装区块体
// 5. 计算区块哈希
// 6. 返回候选区块
//
// 参数：
//   - ctx: 上下文
//   - currentHeight: 当前区块高度
//   - parentHash: 父区块哈希
//   - candidateTxs: 候选交易列表
//
// 返回：
//   - *core.Block: 候选区块
//   - error: 构建错误
func (s *Service) buildCandidate(
	ctx context.Context,
	currentHeight uint64,
	parentHash []byte,
	candidateTxs []*transaction.Transaction,
) (*core.Block, error) {
	// 🔧 计算下一个区块的高度（要挖的新区块）
	nextHeight := currentHeight + 1

	if s.logger != nil {
		if len(parentHash) >= 8 {
			s.logger.Debugf("开始构建候选区块，当前链高度: %d, 新区块高度: %d, 父哈希: %x, 交易数: %d",
				currentHeight, nextHeight, parentHash[:8], len(candidateTxs))
		} else {
			s.logger.Debugf("开始构建候选区块，当前链高度: %d, 新区块高度: %d, 父哈希: %x, 交易数: %d",
				currentHeight, nextHeight, parentHash, len(candidateTxs))
		}
	}

	// 1. 构建Coinbase交易（P3-3：完整实现包含手续费聚合）
	// 使用 nextHeight，因为 Coinbase 交易属于新区块
	coinbaseTx, err := s.buildCoinbaseTransaction(ctx, nextHeight, candidateTxs)
	if err != nil {
		return nil, fmt.Errorf("构建Coinbase交易失败: %w", err)
	}
	if coinbaseTx == nil {
		return nil, fmt.Errorf("构建Coinbase交易失败：返回nil")
	}

	// 2. 组装完整交易列表（Coinbase在首位）
	allTxs := append([]*transaction.Transaction{coinbaseTx}, candidateTxs...)

	// 3. 构建区块头（使用 nextHeight）
	header, err := s.buildBlockHeader(ctx, nextHeight, parentHash, allTxs)
	if err != nil {
		return nil, fmt.Errorf("构建区块头失败: %w", err)
	}

	// 4. 组装区块体
	body := &core.BlockBody{
		Transactions: allTxs,
	}

	// 5. 组装完整区块
	block := &core.Block{
		Header: header,
		Body:   body,
	}

	// 6. 计算区块哈希（用于日志和验证）
	blockHash, err := s.calculateBlockHash(ctx, header)
	if err != nil {
		return nil, fmt.Errorf("计算区块哈希失败: %w", err)
	}

	// 注意：区块哈希不存储在Header中，而是通过计算得出

	if s.logger != nil {
		if len(blockHash) >= 8 {
			s.logger.Debugf("✅ 候选区块构建完成，哈希: %x, 高度: %d, 交易数: %d",
				blockHash[:8], header.Height, len(allTxs))
		} else {
			s.logger.Debugf("✅ 候选区块构建完成，哈希: %x, 高度: %d, 交易数: %d",
				blockHash, header.Height, len(allTxs))
		}
	}

	return block, nil
}

// ==================== 区块奖励配置（可开关） ====================

// calculateBlockReward 计算固定区块奖励
//
// 🎯 **测试用固定奖励**：
// - 用于测试转账功能，提供初始资金来源
// - 生产环境可以通过注释此方法来禁用区块奖励（恢复零增发）
//
// 💰 **奖励规则**：
// - 固定奖励：5 WES = 5,000,000,000 Wei（参考副本4）
// - 每个区块都有固定奖励，不随高度变化
//
// 🔧 **如何禁用区块奖励**：
// 方法1：将此方法的返回值改为 0
//
//	return 0
//
// 方法2：在 buildCoinbaseTransaction 中注释掉调用此方法的代码
//
// 参数：
//   - currentHeight: 当前区块高度（预留，未来可实现减半逻辑）
//
// 返回：
//   - uint64: 区块奖励金额（Wei单位）
func (s *Service) calculateBlockReward(currentHeight uint64) uint64 {
	// 🔧 测试用固定奖励：5 WES
	// 💡 如需禁用区块奖励，将下面这行改为: return 0
	return 5_000_000_000 // 5 WES = 5 * 10^9 Wei

	// 📝 未来可扩展为动态奖励（减半逻辑）：
	// if currentHeight < 210000 {
	//     return 50_000_000_000 // 50 WES
	// } else if currentHeight < 420000 {
	//     return 25_000_000_000 // 25 WES
	// } else {
	//     return 12_500_000_000 // 12.5 WES
	// }
}

// ==================== Coinbase 交易构建 ====================

// buildCoinbaseTransaction 构建Coinbase交易（支持可选的区块奖励）
//
// 🎯 **激励机制**：
// - 手续费奖励：聚合所有交易的手续费
// - 区块奖励：通过 calculateBlockReward() 方法计算（可开关）
// - 矿工总收入 = 区块奖励 + 交易手续费
//
// 📋 **完整实现流程**：
// 1. 计算区块奖励（通过独立方法，方便开关）
// 2. 如果有候选交易，计算并聚合所有交易的手续费
// 3. 合并区块奖励和手续费，构建 Coinbase 交易
// 4. 如果没有任何奖励，创建空的 Coinbase（向后兼容）
//
// 参数：
//   - ctx: 上下文
//   - currentHeight: 当前区块高度
//   - candidateTxs: 候选交易列表（用于计算手续费）
//
// 返回：
//   - *transaction.Transaction: Coinbase交易
//   - error: 构建错误
func (s *Service) buildCoinbaseTransaction(
	ctx context.Context,
	currentHeight uint64,
	candidateTxs []*transaction.Transaction,
) (*transaction.Transaction, error) {
	// 🔧 步骤1：计算区块奖励（可通过 calculateBlockReward 方法开关）
	blockReward := s.calculateBlockReward(currentHeight)

	if s.logger != nil {
		s.logger.Infof("🔧 [DEBUG] buildCoinbaseTransaction 调用: 高度=%d, 区块奖励=%d, 候选交易数=%d",
			currentHeight, blockReward, len(candidateTxs))
	}

	// 步骤2：聚合交易手续费
	var aggregatedFees *tx.AggregatedFees
	if s.feeManager != nil && len(candidateTxs) > 0 {
		// 2.1 计算每笔交易的手续费
		fees := make([]*tx.AggregatedFees, 0, len(candidateTxs))
		for _, tx := range candidateTxs {
			fee, err := s.feeManager.CalculateTransactionFee(ctx, tx)
			if err != nil {
				if s.logger != nil {
					s.logger.Warnf("计算交易手续费失败: %v，跳过该交易", err)
				}
				continue
			}
			if fee != nil && len(fee.ByToken) > 0 {
				fees = append(fees, fee)
			}
		}

		// 2.2 聚合所有手续费
		if len(fees) > 0 {
			aggregatedFees = s.feeManager.AggregateFees(fees)
		}
	}

	// 步骤3：获取矿工地址
	s.minerMu.RLock()
	minerAddr := s.minerAddress
	s.minerMu.RUnlock()

	if s.logger != nil {
		s.logger.Infof("🔧 [DEBUG] 矿工地址长度=%d, 区块奖励=%d", len(minerAddr), blockReward)
	}

	// 步骤4：构建 Coinbase 交易
	// 如果有区块奖励或手续费，且矿工地址可用，构建包含奖励的 Coinbase
	hasReward := blockReward > 0
	hasFees := aggregatedFees != nil && len(aggregatedFees.ByToken) > 0
	hasValidMiner := len(minerAddr) == 20

	if s.logger != nil {
		s.logger.Infof("🔧 [DEBUG] Coinbase条件检查: 有奖励=%v, 有手续费=%v, 矿工地址有效=%v",
			hasReward, hasFees, hasValidMiner)
	}

	if (hasReward || hasFees) && hasValidMiner {
		if s.logger != nil {
			s.logger.Infof("✅ [DEBUG] 调用 buildCoinbaseWithReward 构建奖励Coinbase")
		}
		return s.buildCoinbaseWithReward(ctx, blockReward, aggregatedFees, minerAddr)
	}

	// 步骤5：解析当前链ID（用于设置 Coinbase 交易的 ChainId）
	chainID, err := s.resolveChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("解析链ID失败: %w", err)
	}
	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, chainID)

	// 后备方案：创建空 Coinbase（向后兼容）
	if s.logger != nil {
		s.logger.Warnf("⚠️ [DEBUG] 创建空 Coinbase - 原因: 有奖励=%v, 有手续费=%v, 矿工地址有效=%v",
			hasReward, hasFees, hasValidMiner)
	}

	return &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{},
		Outputs:           []*transaction.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           chainIDBytes,
		FeeMechanism:      nil,
		Metadata:          nil,
	}, nil
}

// buildCoinbaseWithReward 构建包含区块奖励的Coinbase交易
//
// 🎯 **核心逻辑**：
// - 合并区块奖励和手续费到原生币输出
// - 为其他代币创建独立的手续费输出
//
// 💰 **原生币总额计算**：
// - 原生币输出金额 = 区块奖励 + 原生币手续费
//
// 参数：
//   - ctx: 上下文
//   - blockReward: 固定区块奖励（Wei单位）
//   - aggregatedFees: 聚合的手续费（可能为nil）
//   - minerAddr: 矿工地址
//
// 返回：
//   - *transaction.Transaction: Coinbase交易
//   - error: 构建错误
func (s *Service) buildCoinbaseWithReward(
	ctx context.Context,
	blockReward uint64,
	aggregatedFees *tx.AggregatedFees,
	minerAddr []byte,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		feeCount := 0
		if aggregatedFees != nil {
			feeCount = len(aggregatedFees.ByToken)
		}
		s.logger.Infof("🎯 [DEBUG] buildCoinbaseWithReward 调用: 区块奖励=%d, 手续费种类=%d, 矿工地址=%x",
			blockReward, feeCount, minerAddr[:8])
	}

	// 解析当前链ID（用于设置 Coinbase 交易的 ChainId）
	chainID, err := s.resolveChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("解析链ID失败: %w", err)
	}
	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, chainID)

	// 创建 Coinbase 交易基础结构
	coinbase := &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{}, // Coinbase 无输入
		Outputs:           []*transaction.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           chainIDBytes,
		FeeMechanism:      nil,
		Metadata:          nil,
	}

	// 1. 计算原生币总额（区块奖励 + 原生币手续费）
	nativeTotalAmount := big.NewInt(int64(blockReward))

	if aggregatedFees != nil && len(aggregatedFees.ByToken) > 0 {
		// 检查是否有原生币手续费
		nativeTokenKey := tx.TokenKey("native")
		if nativeFee, ok := aggregatedFees.ByToken[nativeTokenKey]; ok && nativeFee != nil {
			// 原生币总额 = 区块奖励 + 手续费
			nativeTotalAmount = new(big.Int).Add(nativeTotalAmount, nativeFee)
		}
	}

	// 2. 创建原生币输出（区块奖励 + 手续费）
	if nativeTotalAmount.Sign() > 0 {
		nativeOutput := &transaction.TxOutput{
			Owner: minerAddr,
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: nativeTotalAmount.String(), // big.Int 转为字符串
						},
					},
				},
			},
			LockingConditions: []*transaction.LockingCondition{
				{
					Condition: &transaction.LockingCondition_SingleKeyLock{
						SingleKeyLock: &transaction.SingleKeyLock{
							KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: minerAddr,
							},
							RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
							SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
						},
					},
				},
			},
		}
		coinbase.Outputs = append(coinbase.Outputs, nativeOutput)

		if s.logger != nil {
			s.logger.Infof("💰 Coinbase原生币输出: 区块奖励(%d Wei) + 手续费 = %s Wei",
				blockReward, nativeTotalAmount.String())
		}
	}

	// 3. 为其他代币创建手续费输出（如果有）
	if aggregatedFees != nil && len(aggregatedFees.ByToken) > 0 {
		for tokenKey, amount := range aggregatedFees.ByToken {
			// 跳过原生币（已经处理过了）
			if tokenKey == "native" {
				continue
			}

			if amount != nil && amount.Sign() > 0 {
				// 创建合约代币输出
				// TODO: 需要解析 tokenKey 来提取 contractAddress 和 tokenClassId
				// 当前简化实现，跳过非原生币（未来扩展）
				if s.logger != nil {
					s.logger.Warnf("暂不支持非原生币手续费输出: %s, 金额: %s", tokenKey, amount.String())
				}
			}
		}
	}

	if s.logger != nil {
		totalFeeTokens := 0
		if aggregatedFees != nil {
			totalFeeTokens = len(aggregatedFees.ByToken)
		}
		s.logger.Debugf("✅ 成功构建包含区块奖励的Coinbase交易，输出数: %d, 手续费代币种类: %d",
			len(coinbase.Outputs), totalFeeTokens)
	}

	return coinbase, nil
}

// SetMinerAddress 设置矿工地址（延迟注入，P3-3）
//
// 🎯 **设计目的**：
// - 矿工地址由共识层或挖矿控制器管理
// - 通过延迟注入避免循环依赖
// - 支持运行时动态设置
//
// 参数：
//   - minerAddr: 矿工地址（必须为20字节）
//
// 说明：
//   - 地址长度错误时会记录错误日志但不中断流程
func (s *Service) SetMinerAddress(minerAddr []byte) {
	if len(minerAddr) != 20 {
		if s.logger != nil {
			s.logger.Errorf("⚠️ 矿工地址长度错误: 期望20字节，实际%d字节", len(minerAddr))
		}
		return
	}

	s.minerMu.Lock()
	defer s.minerMu.Unlock()

	// 创建副本以避免外部修改
	s.minerAddress = make([]byte, 20)
	copy(s.minerAddress, minerAddr)

	if s.logger != nil {
		s.logger.Infof("✅ 矿工地址已设置到 BlockBuilder: %x", minerAddr[:8])
	}
}

// buildBlockHeader 构建区块头
//
// 🎯 **区块头构造**
//
// 参数：
//   - ctx: 上下文
//   - currentHeight: 当前区块高度
//   - parentHash: 父区块哈希
//   - transactions: 交易列表
//
// 返回：
//   - *core.BlockHeader: 区块头
//   - error: 构建错误
func (s *Service) buildBlockHeader(
	ctx context.Context,
	currentHeight uint64,
	parentHash []byte,
	transactions []*transaction.Transaction,
) (*core.BlockHeader, error) {
	// 1. 计算交易Merkle根
	merkleRoot, err := s.calculateMerkleRoot(ctx, transactions)
	if err != nil {
		return nil, fmt.Errorf("计算Merkle根失败: %w", err)
	}

	// 2. 获取状态根（P3-4：从UTXO服务获取当前状态根）
	var stateRoot []byte
	if s.utxoQuery != nil {
		var err error
		stateRoot, err = s.utxoQuery.GetCurrentStateRoot(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取UTXO状态根失败（拒绝出块）: %w", err)
		}
	} else {
		return nil, fmt.Errorf("UTXOQuery未注入，无法获取状态根（拒绝出块）")
	}
	if len(stateRoot) != 32 {
		return nil, fmt.Errorf("状态根长度无效（拒绝出块）：got=%d want=32", len(stateRoot))
	}

	// 3. 获取难度（P3-5：从当前区块获取难度）
	// v2：优先通过 DifficultyPolicy 计算下一高度难度；但在单测/工具/链未初始化等场景允许降级，
	// 避免因为“父区块/创世不存在”阻断候选区块构建。
	if s.configProvider == nil {
		return nil, fmt.Errorf("configProvider 未注入，无法计算难度")
	}

	consensusOpts := s.configProvider.GetConsensus()
	if consensusOpts == nil {
		return nil, fmt.Errorf("无法获取共识配置（GetConsensus 返回 nil）")
	}
	chainOpts := s.configProvider.GetBlockchain()
	if chainOpts == nil {
		return nil, fmt.Errorf("无法获取区块链配置（GetBlockchain 返回 nil）")
	}

	// 计算目标出块时间（秒，至少 1s）
	targetSec := uint64(consensusOpts.TargetBlockTime.Seconds())
	if targetSec == 0 {
		targetSec = 1
	}

	params := difficulty.Params{
		TargetBlockTimeSeconds:             targetSec,
		DifficultyWindow:                   consensusOpts.POW.DifficultyWindow,
		MaxAdjustUpPPM:                     consensusOpts.POW.MaxAdjustUpPPM,
		MaxAdjustDownPPM:                   consensusOpts.POW.MaxAdjustDownPPM,
		EMAAlphaPPM:                        consensusOpts.POW.EMAAlphaPPM,
		MinDifficulty:                      consensusOpts.POW.MinDifficulty,
		MaxDifficulty:                      consensusOpts.POW.MaxDifficulty,
		MTPWindow:                          consensusOpts.POW.MTPWindow,
		MinBlockIntervalSeconds:            uint64(chainOpts.Block.MinBlockInterval),
		MaxFutureDriftSeconds:              consensusOpts.POW.MaxFutureDriftSeconds,
		EmergencyDownshiftThresholdSeconds: consensusOpts.POW.EmergencyDownshiftThresholdSeconds,
		MaxEmergencyDownshiftBits:          consensusOpts.POW.MaxEmergencyDownshiftBits,
	}

	// 预先确定区块时间戳（difficulty 计算需要与 header.Timestamp 保持一致）
	headerTimestamp := uint64(time.Now().Unix())

	// 创世难度：高度0
	var difficultyValue uint64
	if currentHeight == 0 {
		difficultyValue = consensusOpts.POW.InitialDifficulty
	} else {
		parentHeight := currentHeight - 1
		// 如果缺少 blockQuery（或父区块不存在），降级使用最小难度，保证构建流程可继续。
		if s.blockQuery == nil {
			difficultyValue = params.MinDifficulty
			if difficultyValue == 0 {
				difficultyValue = 1
			}
			if s.logger != nil {
				s.logger.Warnf("blockQuery 未注入，无法计算下一难度，降级使用 difficulty=%d (height=%d)", difficultyValue, currentHeight)
			}
		} else {
			parentBlock, err := s.blockQuery.GetBlockByHeight(ctx, parentHeight)
			if err != nil || parentBlock == nil || parentBlock.Header == nil {
				// 常见于：链尚未写入创世区块（height=1 的父高度=0），或者测试未准备父区块
				if parentHeight == 0 {
					difficultyValue = consensusOpts.POW.InitialDifficulty
					if difficultyValue == 0 {
						difficultyValue = params.MinDifficulty
					}
					if difficultyValue == 0 {
						difficultyValue = 1
					}
				} else {
					difficultyValue = params.MinDifficulty
					if difficultyValue == 0 {
						difficultyValue = 1
					}
				}
				if s.logger != nil {
					s.logger.Warnf("获取父区块失败，无法计算下一难度，降级使用 difficulty=%d (parentHeight=%d): %v", difficultyValue, parentHeight, err)
				}
			} else {
				// 先确定新区块时间戳，再将其纳入难度计算（用于“长时间无块后的难度回落/恢复”）。
				// 注意：最终时间戳仍需通过验证侧的 MTP/min-interval/future-drift 规则。
				nowTS := uint64(time.Now().Unix())
				minTS := parentBlock.Header.Timestamp + params.MinBlockIntervalSeconds
				if nowTS < minTS {
					nowTS = minTS
				}
				headerTimestamp = nowTS

				difficultyValue, err = difficulty.NextDifficultyForTimestamp(ctx, s.blockQuery, parentBlock.Header, headerTimestamp, params)
				if err != nil {
					// 计算失败时，回退到父区块难度/最小难度
					difficultyValue = parentBlock.Header.Difficulty
					if difficultyValue == 0 {
						difficultyValue = params.MinDifficulty
					}
					if difficultyValue == 0 {
						difficultyValue = 1
					}
					if s.logger != nil {
						s.logger.Warnf("计算下一难度失败，降级使用 difficulty=%d: %v", difficultyValue, err)
					}
				}
			}
		}
	}

	// 4. 构建区块头
	// 注意：currentHeight 参数已经是下一个区块的高度（在 buildCandidate 中计算）
	chainID, err := s.resolveChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("解析链ID失败: %w", err)
	}

	header := &core.BlockHeader{
		ChainId:      chainID,
		Version:      1,
		PreviousHash: parentHash,
		MerkleRoot:   merkleRoot,
		Timestamp:    headerTimestamp,
		Height:       currentHeight,
		Nonce:        make([]byte, 8), // 初始nonce（挖矿时修改）
		Difficulty:   difficultyValue,
		StateRoot:    stateRoot,
	}

	return header, nil
}

// calculateMerkleRoot 计算Merkle根
//
// 🎯 **Merkle树计算**
//
// 使用 merkle.CalculateMerkleRoot 进行标准Merkle树计算
//
// 参数：
//   - transactions: 交易列表
//
// 返回：
//   - []byte: Merkle根（32字节）
//   - error: 计算错误
func (s *Service) calculateMerkleRoot(ctx context.Context, transactions []*transaction.Transaction) ([]byte, error) {
	if len(transactions) == 0 {
		// 空交易列表返回全零Merkle根
		return make([]byte, 32), nil
	}

	if s.logger != nil {
		s.logger.Infof("🔧 [BlockBuilder] 使用统一交易哈希服务计算Merkle根，交易数: %d", len(transactions))
	}

	// 🔧 使用统一的交易哈希服务计算交易哈希
	// 确保与共识层（PoW Handler）的计算方式完全一致
	transactionHashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		req := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}

		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("计算交易[%d]哈希失败: %w", i, err)
		}

		if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
			return nil, fmt.Errorf("交易[%d]哈希无效", i)
		}

		transactionHashes[i] = resp.Hash

		if s.logger != nil && i == 0 {
			s.logger.Infof("🔧 [BlockBuilder] 第一笔交易哈希: %x", resp.Hash[:16])
		}
	}

	// 使用 crypto 接口构建Merkle树
	merkleRoot, err := s.buildMerkleTreeFromHashes(transactionHashes)
	if err != nil {
		return nil, fmt.Errorf("构建Merkle树失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ [BlockBuilder] 计算Merkle根完成，交易数: %d, Merkle根: %x", len(transactions), merkleRoot[:16])
	}

	return merkleRoot, nil
}

// buildMerkleTreeFromHashes 从交易哈希列表构建Merkle树
// 🔧 与 MerkleTreeManager 保持一致：对奇数节点（包括单个节点）进行复制
func (s *Service) buildMerkleTreeFromHashes(hashes [][]byte) ([]byte, error) {
	// 🔧 修复：即使只有1个节点也要复制，与 MerkleTreeManager 保持一致
	// MerkleTreeManager 在构建时会对奇数节点复制，确保树的完整性

	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 基础情况：2个节点配对后返回
	if len(hashes) == 2 {
		combined := append(hashes[0], hashes[1]...)
		parentHash, err := s.hasher.Hash(combined)
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
		parentHash, err := s.hasher.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return s.buildMerkleTreeFromHashes(nextLevel)
}

// calculateBlockHash 计算区块哈希
//
// 🎯 **区块哈希计算**
//
// 使用 gRPC BlockHashService 进行标准区块哈希计算
//
// 参数：
//   - header: 区块头
//
// 返回：
//   - []byte: 区块哈希（32字节）
//   - error: 计算错误
func (s *Service) calculateBlockHash(ctx context.Context, header *core.BlockHeader) ([]byte, error) {
	if s.blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 未初始化")
	}

	// 构建区块（只有Header，Body可以为空）
	block := &core.Block{
		Header: header,
	}

	// 使用 gRPC 服务计算区块哈希
	req := &core.ComputeBlockHashRequest{
		Block: block,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("区块结构无效")
	}

	hash := resp.Hash
	if s.logger != nil {
		s.logger.Debugf("✅ 计算区块哈希成功，高度: %d, 哈希: %x", header.Height, hash[:8])
	}

	return hash, nil
}
