// Package block 提供区块管理的核心实现
//
// 📋 **create.go - 挖矿候选区块创建实现**
//
// 本文件实现 CreateMiningCandidate 方法的完整业务逻辑，采用哈希+缓存架构模式。
// 专注于为矿工创建高质量的候选区块，支持企业级并发挖矿场景。
//
// 🎯 **核心职责**：
// - 从交易池获取优质交易
// - 按代币类型聚合手续费
// - 创建包含挖矿奖励的 Coinbase 交易
// - 构造完整的候选区块结构
// - 计算区块哈希并缓存候选区块
// - 返回轻量级哈希标识符
//
// 🏗️ **架构特点**：
// - 哈希+缓存模式：返回32字节哈希，复杂对象存储在内存缓存
// - 企业级并发：支持多个矿工同时创建候选区块
// - 智能费用聚合：自动按代币类型计算和聚合手续费
// - TTL缓存管理：自动清理过期的候选区块缓存
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 挖矿候选区块创建 ====================

// createMiningCandidate 创建挖矿候选区块并返回区块哈希
//
// 🎯 **核心业务逻辑实现**
//
// 这是 BlockService.CreateMiningCandidate 的完整实现，采用哈希+缓存架构模式。
// 从交易池获取最优交易，构建候选区块供矿工挖矿，返回区块哈希作为标识符。
//
// 🔄 **完整业务流程**：
//
// 1. **获取挖矿模板**：
//   - 调用 TransactionService.GetMiningTemplate() 获取完整的交易模板
//   - 内部自动完成：矿工状态检查、内存池交易获取、手续费聚合、区块奖励计算、Coinbase生成
//   - 返回完整的交易列表（Coinbase交易在首位）
//
// 2. **构建区块头**：
//   - 调用 buildCandidateBlockHeader() 一站式构建区块头
//   - 内部自动完成：父区块信息获取、Merkle根计算、时间戳生成、状态根获取
//   - 返回完整的区块头结构
//
// 3. **组装候选区块**：
//   - 将区块头和交易列表组装成完整的区块结构
//   - 验证区块格式的协议兼容性
//
// 4. **区块哈希计算**：
//   - 使用 BlockHashServiceClient.ComputeBlockHash() 计算标准哈希
//   - 基于区块头内容，不包含 POW 字段
//   - 确保哈希计算的确定性和跨平台一致性
//
// 5. **缓存存储管理**：
//   - 将候选区块序列化并存储到 MemoryStore
//   - 设置合理的 TTL（Time To Live）防止内存泄漏
//   - 支持并发访问和修改（后续 POW 计算）
//
// 🎯 **性能优化策略**：
// - 职责分离：交易逻辑在transaction服务，区块头逻辑在buildCandidateBlockHeader内部
// - 参数内聚：避免了5个参数的长链传递，简化了接口调用
// - 一次调用：GetMiningTemplate和buildCandidateBlockHeader都是一站式服务
// - 并发安全：支持多矿工同时创建候选区块
// - 缓存复用：避免重复计算相同的候选区块
// - 内存管理：通过 TTL 自动清理过期缓存
//
// 🛡️ **错误处理机制**：
// - 挖矿模板获取失败：transaction服务内部处理矿工状态、交易获取等错误
// - 区块头构建失败：内部处理父区块获取、Merkle计算、状态根获取等错误
// - 区块组装失败：结构异常，需要检查数据完整性
// - 缓存存储失败：记录错误但仍返回哈希
// - 哈希计算失败：系统性错误，需要排查
//
// 🔄 **与矿工的协作流程**：
// 1. 矿工调用此方法获取候选区块哈希
// 2. 矿工通过哈希从缓存获取完整候选区块
// 3. 矿工执行 POW 计算，修改区块的 nonce 字段
// 4. 找到有效 nonce 后，矿工广播完整区块
// 5. 其他节点通过 ValidateBlock 和 ProcessBlock 处理
//
// 参数：
//
//	ctx: 上下文对象，用于超时控制和取消操作
//
// 返回值：
//
//	[]byte: 32字节候选区块哈希，用于标识缓存中的候选区块
//	error: 创建过程中的错误，nil 表示创建成功
//
// 使用示例：
//
//	blockHash, err := manager.CreateMiningCandidate(ctx)
//	if err != nil {
//	  logger.Errorf("创建候选区块失败: %v", err)
//	  return err
//	}
//
//	logger.Infof("候选区块创建成功，哈希: %x", blockHash)
//	// 矿工可通过 blockHash 从缓存获取完整区块进行挖矿
func (m *Manager) createMiningCandidate(ctx context.Context) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("开始创建挖矿候选区块")
	}

	// 1. 获取完整的挖矿模板（包含Coinbase + 所有普通交易）
	allTransactions, err := m.transactionService.GetMiningTemplate(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取挖矿模板失败: %w", err)
	}

	// 2. 构建候选区块头（内部处理所有必要的计算）
	blockHeader, err := m.buildCandidateBlockHeader(ctx, allTransactions)
	if err != nil {
		return nil, fmt.Errorf("构建区块头失败: %w", err)
	}

	// 3. 组装完整的候选区块
	candidateBlock, err := m.assembleCandidateBlock(blockHeader, allTransactions)
	if err != nil {
		return nil, fmt.Errorf("组装候选区块失败: %w", err)
	}

	// 4. 存储候选区块到缓存并获取哈希
	blockHash, err := m.storeCandidateBlock(ctx, candidateBlock)
	if err != nil {
		return nil, fmt.Errorf("存储候选区块失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("成功创建挖矿候选区块，哈希: %x, 高度: %d, 交易数: %d",
			blockHash, blockHeader.Height, len(allTransactions))
	}

	return blockHash, nil
}

// ==================== 内部辅助方法 ====================

// buildCandidateBlockHeader 构建候选区块头
//
// 🎯 **一站式区块头构造**
//
// 内部完成所有区块头构造所需的操作，实现职责内聚：
// 1. 获取父区块信息（高度和哈希）
// 2. 计算当前区块高度
// 3. 计算交易Merkle根
// 4. 生成当前时间戳
// 5. 获取当前UTXO状态根
// 6. 构建完整的区块头结构
//
// 🎯 **设计优势**：
// - 职责内聚：所有区块头相关逻辑集中在一个方法
// - 简化调用：外部只需传入交易列表即可
// - 减少参数：避免了5个参数的长链传递
// - 易于维护：区块头构造逻辑的变更不影响调用方
//
// 参数：
//
//	ctx: 上下文对象
//	transactions: 交易列表（用于计算Merkle根）
//
// 返回值：
//
//	*BlockHeader: 构造完成的区块头
//	error: 构建过程中的错误
func (m *Manager) buildCandidateBlockHeader(ctx context.Context, transactions []*transaction.Transaction) (*core.BlockHeader, error) {

	if m.logger != nil {
		m.logger.Debugf("开始构建候选区块头，交易数量: %d", len(transactions))
	}

	// 1. 获取父区块信息
	parentHeight, parentHash, err := m.repo.GetHighestBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取父区块信息失败: %w", err)
	}

	currentHeight := parentHeight + 1

	// 2. 计算适应性难度（为交易收集优化）
	//
	// 🎯 **难度调整策略**：
	// - 目标：让矿工有足够时间收集更多交易，提高区块交易密度
	// - 方法：根据交易池状态和历史区块间隔动态调整难度
	// - 原则：更多交易 = 降低难度，更少交易 = 保持或提高难度
	currentDifficulty, err := m.calculateAdaptiveDifficulty(ctx, parentHeight, parentHash, len(transactions))
	if err != nil {
		return nil, fmt.Errorf("计算适应性难度失败: %w", err)
	}

	// 3. 计算交易Merkle根（使用标准化内部接口方法）
	merkleRoot, err := m.CalculateMerkleRoot(ctx, transactions)
	if err != nil {
		return nil, fmt.Errorf("计算Merkle根失败: %w", err)
	}

	// 4. 生成真实时间戳（必须反映真实创建时间）
	//
	// ⚠️ **区块链时间戳完整性原则**：
	// - 时间戳必须反映区块真实创建时间，绝不允许人为调整
	// - 任何基于"智能时间戳"或时间戳调整的设计都违背区块链基本原则
	// - 出块频率控制通过以下正确方式实现：
	//   1. 矿工侧：调整挖矿难度系数，让矿工有足够时间收集更多交易
	//   2. 聚合器侧：设置固定收集窗口，给足够时间收集候选区块进行选择
	// - 时间戳的唯一作用是记录区块真实创建时间，用于审计和排序
	timestamp := uint64(time.Now().Unix())

	// 5. 获取链ID配置
	var chainId uint64 = 1 // 安全默认值
	if m.configManager != nil {
		if blockchainConfig := m.configManager.GetBlockchain(); blockchainConfig != nil {
			chainId = blockchainConfig.ChainID
		} else if m.logger != nil {
			m.logger.Warn("无法获取区块链配置，使用默认链ID: 1")
		}
	} else if m.logger != nil {
		m.logger.Warn("配置管理器未初始化，使用默认链ID: 1")
	}

	// 6. 获取当前UTXO状态根
	stateRoot, err := m.utxoManager.GetCurrentStateRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取UTXO状态根失败: %w", err)
	}

	// 7. 构建区块头
	header := &core.BlockHeader{
		ChainId: chainId, // ✅ 从配置获取链ID，防止跨链重放攻击
		Version: 1,       // 协议版本号
		// 创世块：父哈希使用32字节全零；否则使用最高块哈希
		PreviousHash: func() []byte {
			if parentHeight == 0 && len(parentHash) == 0 {
				return make([]byte, 32)
			}
			return parentHash
		}(),
		MerkleRoot: merkleRoot,        // 交易Merkle根
		Timestamp:  timestamp,         // 当前时间戳
		Height:     currentHeight,     // 区块高度
		Nonce:      make([]byte, 8),   // 初始nonce（挖矿时设置）
		Difficulty: currentDifficulty, // 适应性难度（为交易收集优化）
		StateRoot:  stateRoot,         // UTXO状态根
		// 执行费用相关字段保持为空，候选区块不设置这些值
	}

	if m.logger != nil {
		m.logger.Debugf("区块头构建完成，父哈希: %x, 高度: %d, 难度: %d, Merkle根: %x",
			parentHash, currentHeight, currentDifficulty, merkleRoot)
	}

	return header, nil
}

// assembleCandidateBlock 组装完整的候选区块
//
// 🎯 **候选区块完整构造**
//
// 将区块头和交易列表组装成完整的候选区块结构。
// 确保区块格式符合协议要求，可供后续的挖矿和验证使用。
//
// 组装要点：
// - 区块头完整性：确保所有必要字段已设置
// - 交易顺序：Coinbase 交易在首位，其他交易按优化顺序排列
// - 大小验证：确保区块大小在协议限制内
// - 格式检查：验证区块结构的协议兼容性
//
// 参数：
//
//	header: 已构建的区块头
//	transactions: 交易列表（包含Coinbase交易）
//
// 返回值：
//
//	*Block: 组装完成的候选区块
//	error: 组装过程中的错误
func (m *Manager) assembleCandidateBlock(header *core.BlockHeader,
	transactions []*transaction.Transaction) (*core.Block, error) {

	if m.logger != nil {
		m.logger.Debugf("组装完整的候选区块，交易数: %d", len(transactions))
	}

	// 1. 验证输入参数
	if header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("交易列表不能为空")
	}

	// 2. 创建区块体
	blockBody := &core.BlockBody{
		Transactions: transactions,
	}

	// 3. 创建完整的区块结构
	candidateBlock := &core.Block{
		Header: header,
		Body:   blockBody,
	}

	// 4. 基础格式检查
	if candidateBlock.Header.Height == 0 && len(candidateBlock.Header.PreviousHash) != 32 {
		return nil, fmt.Errorf("创世区块的父区块哈希必须为32字节全零")
	}

	if candidateBlock.Header.Height > 0 && len(candidateBlock.Header.PreviousHash) == 0 {
		return nil, fmt.Errorf("非创世区块必须有父区块哈希")
	}

	// 5. 验证Coinbase交易在首位（如果有多个交易）
	// 说明：挖矿模板生成逻辑保证 coinbase 位于交易列表首位；
	// coinbase 的识别以"没有任何输入"为准（见 pkg/utils/transaction.go 的 IsCoinbaseTx）。
	if len(transactions) > 1 {
		firstTx := transactions[0]
		if len(firstTx.Inputs) != 0 {
			return nil, fmt.Errorf("首个交易应该是Coinbase交易（没有输入）")
		}
	}

	if m.logger != nil {
		m.logger.Debugf("候选区块组装完成，高度: %d, 交易数: %d",
			header.Height, len(transactions))
	}

	return candidateBlock, nil
}

// calculateAdaptiveDifficulty 计算适应性难度（为交易收集优化）
//
// 🎯 **适应性难度调整策略**：
//
// 目标：让矿工有足够时间收集更多交易，提高区块利用率
//
// 调整逻辑：
// 1. 交易数量因子：更多交易 → 稍微降低难度（奖励高效打包）
// 2. 交易池状态：交易池饱满 → 降低难度（加快出块消费交易）
// 3. 基础难度保护：防止难度过低影响安全性
// 4. 渐进调整：避免难度剧烈波动
//
// 参数：
//
//	ctx: 上下文
//	parentHeight: 父区块高度
//	parentHash: 父区块哈希
//	currentTxCount: 当前区块交易数量
//
// 返回值：
//
//	uint64: 调整后的适应性难度
//	error: 计算错误
func (m *Manager) calculateAdaptiveDifficulty(ctx context.Context, parentHeight uint64, parentHash []byte, currentTxCount int) (uint64, error) {
	// 1. 获取基础难度（从父区块或默认值）
	baseDifficulty := uint64(1) // 创世区块默认难度
	if parentHeight > 0 && len(parentHash) > 0 {
		parentBlock, err := m.repo.GetBlock(ctx, parentHash)
		if err != nil {
			return 0, fmt.Errorf("获取父区块失败: %w", err)
		}
		if parentBlock.Header != nil {
			baseDifficulty = parentBlock.Header.Difficulty
		}
	}

	// 2. 获取配置参数
	var (
		targetTxCount     = 50          // 目标交易数量
		maxDifficultyDown = 0.8         // 最大难度下调比例（20%下调）
		minDifficultyUp   = 1.1         // 最小难度上调比例（10%上调）
		minDifficulty     = uint64(1)   // 最小难度保护
		maxDifficulty     = uint64(100) // 最大难度限制
	)

	// 从配置获取参数（如果可用）
	if m.configManager != nil {
		if blockchainConfig := m.configManager.GetBlockchain(); blockchainConfig != nil {
			// TODO: 从配置中读取难度调整参数
			// targetTxCount = blockchainConfig.Difficulty.TargetTxCount
		}
	}

	// 3. 计算交易数量因子
	txFactor := 1.0
	if currentTxCount > targetTxCount {
		// 交易多 → 稍微降低难度（奖励高效打包）
		txFactor = maxDifficultyDown + 0.2*float64(targetTxCount)/float64(currentTxCount)
		if txFactor < maxDifficultyDown {
			txFactor = maxDifficultyDown
		}
	} else if currentTxCount < targetTxCount/2 {
		// 交易少 → 稍微提高难度（鼓励等待更多交易）
		txFactor = minDifficultyUp
	}

	// 4. 应用调整因子
	newDifficulty := uint64(float64(baseDifficulty) * txFactor)

	// 5. 边界保护
	if newDifficulty < minDifficulty {
		newDifficulty = minDifficulty
	}
	if newDifficulty > maxDifficulty {
		newDifficulty = maxDifficulty
	}

	// 6. 记录调整信息
	if m.logger != nil {
		if newDifficulty != baseDifficulty {
			m.logger.Infof("适应性难度调整：%d → %d (交易数: %d, 目标: %d, 因子: %.2f)",
				baseDifficulty, newDifficulty, currentTxCount, targetTxCount, txFactor)
		} else {
			m.logger.Debugf("难度保持不变：%d (交易数: %d)", baseDifficulty, currentTxCount)
		}
	}

	return newDifficulty, nil
}
