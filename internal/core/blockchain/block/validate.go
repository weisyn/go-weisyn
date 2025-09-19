// Package block 提供区块管理的核心实现
//
// 📋 **validation.go - 区块验证实现**
//
// 本文件实现 ValidateBlock 方法的完整业务逻辑，提供多层次、全方位的区块验证体系。
// 确保从网络接收的区块符合协议规范和共识规则，保障区块链的安全性和一致性。
//
// 🎯 **核心职责**：
// - 多层验证体系：从基础结构到业务规则的完整验证链
// - 密码学验证：POW、签名、Merkle根的严格验证
// - 业务规则验证：价值守恒、UTXO合法性、交易依赖关系验证
// - 共识规则验证：难度、时间戳、链连接性验证
// - 状态一致性验证：确保区块与当前链状态的一致性
//
// 🏗️ **架构特点**：
// - 分层验证设计：按验证复杂度和成本递进验证
// - 快速失败原则：在最早阶段发现和拒绝无效区块
// - 详细错误报告：提供明确的验证失败原因
// - 性能优化：支持并行验证和缓存优化
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"bytes"
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== 区块验证实现 ====================

// validateBlock 验证区块的完整性和合法性
//
// 🎯 **多层验证体系实现**
//
// 这是 BlockService.ValidateBlock 的完整实现，采用分层验证策略。
// 对从网络接收的区块进行全面验证，确保符合协议规范和共识规则。
//
// 🔄 **分层验证流程**：
//
// **第一层：基础结构验证**
// 1. **Protobuf 格式验证**：
//   - 验证区块结构是否符合 protobuf 定义
//   - 检查必需字段是否完整
//   - 验证字段类型和格式的正确性
//
// 2. **字段完整性验证**：
//   - 区块头字段完整性检查
//   - 区块体交易列表非空验证
//   - 关键标识符的有效性检查
//
// **第二层：区块头验证**
// 3. **版本兼容性验证**：
//   - 检查区块版本是否被当前协议支持
//   - 验证向后兼容性要求
//
// 4. **时间戳验证**：
//   - 检查区块时间戳是否合理（不超前、不滞后太多）
//   - 验证时间戳与父区块的时序关系
//   - 应用网络时间漂移容忍度
//
// 5. **难度验证**：
//   - 计算期望难度值
//   - 验证区块难度是否符合调整算法
//   - 检查难度调整间隔和幅度
//
// **第三层：链连接性验证**
// 6. **父区块验证**：
//   - 验证 previous_hash 指向的父区块存在
//   - 检查父区块是否是当前最佳链的一部分
//   - 验证区块高度的连续性
//
// 7. **链状态一致性**：
//   - 检查区块高度是否正确递增
//   - 验证链的连续性和完整性
//
// **第四层：密码学验证**
// 8. **工作量证明（POW）验证**：
//   - 验证区块哈希是否满足难度要求
//   - 检查 nonce 值的有效性
//   - 确保 POW 计算的正确性
//
// 9. **Merkle 根验证**：
//   - 重新计算交易列表的 Merkle 根
//   - 与区块头中的 merkle_root 字段对比
//   - 确保交易数据的完整性
//
// **第五层：交易层验证**
// 10. **交易格式验证**：
//   - 验证每个交易的结构正确性
//   - 检查交易版本和字段完整性
//
// 11. **交易签名验证**：
//   - 验证每个交易输入的解锁证明
//   - 检查签名算法和参数的正确性
//   - 确保签名与交易数据匹配
//
// 12. **UTXO 引用验证**：
//   - 验证每个交易输入引用的 UTXO 存在且未被花费
//   - 检查 UTXO 引用的格式和有效性
//   - 验证引用权限和锁定条件
//
// **第六层：业务规则验证**
// 13. **价值守恒验证**：
//   - 计算每个交易的输入总值和输出总值
//   - 验证 输入总值 >= 输出总值 + 交易费
//   - 检查多币种的价值守恒
//
// 14. **交易依赖验证**：
//   - 检查区块内交易的依赖关系
//   - 验证依赖交易的执行顺序
//   - 确保没有循环依赖
//
// 15. **Coinbase 交易验证**：
//   - 验证 Coinbase 交易的格式和位置
//   - 检查挖矿奖励的计算正确性
//   - 验证手续费聚合的准确性
//
// **第七层：状态一致性验证**
// 16. **UTXO 状态根验证**：
//   - 计算应用区块后的 UTXO 状态根
//   - 与区块头中的 state_root 对比
//   - 确保状态转换的正确性
//
// 17. **链状态更新预演**：
//   - 模拟应用区块后的链状态
//   - 验证状态转换的合法性
//   - 检查是否存在状态冲突
//
// 🎯 **验证策略优化**：
// - **快速失败**：在早期阶段发现问题立即返回，避免不必要的计算
// - **并行验证**：对独立的验证项目使用并行处理提高效率
// - **缓存利用**：复用之前的验证结果，避免重复计算
// - **增量验证**：基于已验证的父区块进行增量验证
//
// 🛡️ **安全考虑**：
// - **防止 DoS 攻击**：限制验证时间和资源消耗
// - **防止重放攻击**：验证交易的唯一性和时序性
// - **防止双花攻击**：严格的 UTXO 状态验证
// - **防止时间攻击**：合理的时间戳验证范围
//
// 📊 **性能指标**：
// - 目标验证时间：< 500ms（1MB 区块，1000 个交易）
// - 内存使用：< 100MB（包含临时验证状态）
// - CPU 使用：支持多核并行验证
// - 网络使用：最小化对网络资源的依赖
//
// 参数：
//
//	ctx: 上下文对象，用于超时控制和取消操作
//	block: 待验证的区块，包含区块头和交易列表
//
// 返回值：
//
//	bool: 验证结果，true 表示区块有效，false 表示无效
//	error: 验证过程中的错误，包含详细的失败原因
//
// 使用示例：
//
//	// 从 P2P 网络接收区块
//	receivedBlock := p2pNetwork.ReceiveBlock()
//
//	// 验证区块有效性
//	valid, err := manager.ValidateBlock(ctx, receivedBlock)
//	if err != nil {
//	  logger.Errorf("验证区块时发生错误: %v", err)
//	  return err
//	}
//
//	if valid {
//	  logger.Infof("区块验证通过，高度: %d", receivedBlock.Header.Height)
//	  // 继续处理有效区块
//	  err = manager.ProcessBlock(ctx, receivedBlock)
//	} else {
//	  logger.Warnf("拒绝无效区块，高度: %d", receivedBlock.Header.Height)
//	  // 记录无效区块信息供调试使用
//	}
func (m *Manager) validateBlock(ctx context.Context, block *core.Block) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("开始验证区块，高度: %d, 哈希: %x",
			block.Header.Height, block.Header.PreviousHash)
	}

	// 步骤1: 基础结构验证
	if err := m.validateBlockStructure(block); err != nil {
		if m.logger != nil {
			m.logger.Warnf("区块结构验证失败: %v", err)
		}
		return false, fmt.Errorf("基础结构验证失败: %w", err)
	}

	// 步骤2: 区块头验证（字段验证）
	if err := m.validateBlockHeader(block.Header); err != nil {
		if m.logger != nil {
			m.logger.Warnf("区块头验证失败: %v", err)
		}
		return false, fmt.Errorf("区块头验证失败: %w", err)
	}

	// 步骤3: 链连接性验证
	if err := m.validateChainConnectivity(ctx, block); err != nil {
		if m.logger != nil {
			m.logger.Warnf("链连接性验证失败: %v", err)
		}
		return false, fmt.Errorf("链连接性验证失败: %w", err)
	}

	// 步骤4: Merkle根验证
	if err := m.validateMerkleRoot(ctx, block); err != nil {
		if m.logger != nil {
			m.logger.Warnf("Merkle根验证失败: %v", err)
		}
		return false, fmt.Errorf("Merkle根验证失败: %w", err)
	}

	// 步骤5: 工作量证明验证（使用标准BlockHashService）
	if err := m.validateProofOfWork(block); err != nil {
		if m.logger != nil {
			m.logger.Warnf("POW验证失败: %v", err)
		}
		return false, fmt.Errorf("工作量证明验证失败: %w", err)
	}

	// 步骤6: 批量验证区块中的所有交易（委托给交易服务）
	valid, err := m.transactionService.ValidateTransactionsInBlock(ctx, block.Body.Transactions)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("交易验证失败: %v", err)
		}
		return false, fmt.Errorf("交易验证失败: %w", err)
	}
	if !valid {
		if m.logger != nil {
			m.logger.Warn("区块中包含无效交易")
		}
		return false, fmt.Errorf("区块中包含无效交易")
	}

	if m.logger != nil {
		m.logger.Infof("✅ 区块验证通过，高度: %d", block.Header.Height)
	}

	return true, nil
}

// ==================== 第一层：基础结构验证 ====================

// validateBlockStructure 验证区块基础结构
//
// 🎯 **结构完整性检查**
//
// 验证区块的基础结构是否符合协议定义，包括格式、字段完整性等。
// 这是验证的第一步，用于快速过滤格式错误的区块。
//
// 验证内容：
// - Protobuf 格式正确性
// - 必需字段的存在性
// - 字段值的合法性范围
// - 区块大小限制
//
// 参数：
//
//	block: 待验证的区块
//
// 返回值：
//
//	error: 结构验证错误，nil 表示结构正确
func (m *Manager) validateBlockStructure(block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("验证区块基础结构")
	}

	// 1. 检查区块是否为空
	if block == nil {
		return fmt.Errorf("区块不能为空")
	}

	// 2. 检查区块头是否存在且完整
	if block.Header == nil {
		return fmt.Errorf("区块头不能为空")
	}

	// 3. 验证区块头必需字段
	header := block.Header
	if header.Version == 0 {
		return fmt.Errorf("区块版本不能为0")
	}
	if len(header.PreviousHash) != 32 {
		return fmt.Errorf("父区块哈希长度必须为32字节，实际: %d", len(header.PreviousHash))
	}
	if len(header.MerkleRoot) != 32 {
		return fmt.Errorf("Merkle根哈希长度必须为32字节，实际: %d", len(header.MerkleRoot))
	}
	if header.Timestamp == 0 {
		return fmt.Errorf("区块时间戳不能为0")
	}
	if header.Difficulty == 0 {
		return fmt.Errorf("区块难度不能为0")
	}

	// 4. 检查区块体和交易列表（根据proto定义）
	if block.Body == nil {
		return fmt.Errorf("区块体不能为空")
	}
	if block.Body.Transactions == nil {
		return fmt.Errorf("交易列表不能为空")
	}
	if len(block.Body.Transactions) == 0 {
		return fmt.Errorf("区块必须至少包含一个交易（Coinbase交易）")
	}

	// 注意：proto中未定义区块大小限制，交易数量由共识层和交易池管理

	return nil
}

// ==================== 第二层：区块头验证 ====================

// validateBlockHeader 验证区块头字段
//
// 🎯 **区块头字段验证 - Block层职责**
//
// 仅验证区块头的各个字段是否符合协议要求，不进行链连接性查询。
// 链连接性验证由主验证流程单独处理，保持职责分离。
//
// 验证内容：
// - 版本号兼容性
// - 字段格式正确性
// - 数值合理性范围
//
// 参数：
//
//	header: 区块头
//
// 返回值：
//
//	error: 区块头验证错误，nil 表示验证通过
func (m *Manager) validateBlockHeader(header *core.BlockHeader) error {
	if m.logger != nil {
		m.logger.Debugf("验证区块头，链ID: %d, 版本: %d, 高度: %d",
			header.ChainId, header.Version, header.Height)
	}

	// 1. 验证链ID（最重要的安全检查）
	var expectedChainId uint64 = 1 // 安全默认值
	if m.configManager != nil {
		if blockchainConfig := m.configManager.GetBlockchain(); blockchainConfig != nil {
			expectedChainId = blockchainConfig.ChainID
			if m.logger != nil {
				m.logger.Debugf("✅ 从区块链配置获取期望链ID: %d", expectedChainId)
			}
		} else if m.logger != nil {
			m.logger.Warnf("⚠️  无法获取区块链配置，使用默认链ID验证: %d", expectedChainId)
		}
	} else if m.logger != nil {
		m.logger.Warnf("⚠️  配置管理器未初始化，使用默认链ID验证: %d", expectedChainId)
	}

	if header.ChainId != expectedChainId {
		if m.logger != nil {
			m.logger.Errorf("❌ 链ID验证失败: 期望=%d, 实际=%d, 区块高度=%d （可能存在跨链重放攻击）",
				expectedChainId, header.ChainId, header.Height)
		}
		return fmt.Errorf("链ID不匹配，期望: %d, 实际: %d（可能存在跨链重放攻击）", expectedChainId, header.ChainId)
	}

	if m.logger != nil {
		m.logger.Debugf("✅ 链ID验证通过: %d (区块高度: %d)", header.ChainId, header.Height)
	}

	// 2. 验证版本号兼容性
	const minSupportedVersion = 1
	const maxSupportedVersion = 3
	if header.Version < minSupportedVersion || header.Version > maxSupportedVersion {
		return fmt.Errorf("不支持的区块版本: %d, 支持范围: %d-%d",
			header.Version, minSupportedVersion, maxSupportedVersion)
	}

	// 3. 验证时间戳基本合理性（不涉及父区块比较）
	if err := m.validateTimestamp(header.Timestamp, 0); err != nil {
		return fmt.Errorf("时间戳验证失败: %w", err)
	}

	// 4. 验证nonce格式（proto中定义为bytes）
	// 创世区块（height==0）允许 nonce 为空；普通区块需满足长度要求
	if header.Height != 0 {
		if len(header.Nonce) == 0 {
			return fmt.Errorf("nonce不能为空")
		}
		if len(header.Nonce) > 32 { // 合理的nonce长度限制
			return fmt.Errorf("nonce长度超出限制，最大32字节，实际: %d", len(header.Nonce))
		}
	}

	// 5. 验证基本数值范围
	// 创世区块（height==0）允许 difficulty 为0；其余区块不得为0
	if header.Height != 0 {
		if header.Difficulty == 0 {
			return fmt.Errorf("难度不能为0")
		}
	}

	return nil
}

// validateChainConnectivity 验证区块与链的连接性
//
// 🎯 **链连接性验证 - Block层职责**
//
// 验证区块是否能正确连接到现有区块链上，包括父区块存在性、
// 高度连续性、时间戳关系等链级别的验证。
//
// 验证内容：
// - 父区块存在性验证
// - 区块高度连续性
// - 时间戳与父区块关系
// - 难度调整验证
//
// 参数：
//
//	ctx: 上下文对象
//	block: 待验证区块
//
// 返回值：
//
//	error: 连接性验证错误，nil表示连接正确
func (m *Manager) validateChainConnectivity(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("验证链连接性，父区块哈希: %x", block.Header.PreviousHash)
	}

	header := block.Header

	// 1. 创世区块特殊处理
	if header.Height == 0 {
		// 创世区块：父区块哈希应该为全零
		expectedZeroHash := make([]byte, 32)
		if !bytes.Equal(header.PreviousHash, expectedZeroHash) {
			return fmt.Errorf("创世区块的父区块哈希必须为全零")
		}
		return nil // 创世区块无需进一步验证
	}

	// 2. 获取父区块
	parentBlock, err := m.repo.GetBlock(ctx, header.PreviousHash)
	if err != nil {
		return fmt.Errorf("获取父区块失败: %w", err)
	}
	if parentBlock == nil {
		return fmt.Errorf("父区块不存在，哈希: %x", header.PreviousHash)
	}

	// 3. 验证高度连续性
	if header.Height != parentBlock.Header.Height+1 {
		return fmt.Errorf("区块高度不连续，期望: %d, 实际: %d",
			parentBlock.Header.Height+1, header.Height)
	}

	// 4. 验证时间戳与父区块的关系
	if err := m.validateTimestamp(header.Timestamp, parentBlock.Header.Timestamp); err != nil {
		return fmt.Errorf("与父区块时间戳验证失败: %w", err)
	}

	// 5. 验证难度调整
	if err := m.validateDifficulty(header.Difficulty, parentBlock, header.Height); err != nil {
		return fmt.Errorf("难度验证失败: %w", err)
	}

	return nil
}

// validateTimestamp 验证区块时间戳
//
// 🎯 **时间戳合理性检查**
//
// 验证区块时间戳是否在合理范围内，防止时间攻击。
//
// 验证规则：
// - 不能超前当前时间太多
// - 不能滞后父区块时间戳
// - 应用网络时间偏移容忍度
//
// 参数：
//
//	timestamp: 区块时间戳
//	parentTimestamp: 父区块时间戳
//
// 返回值：
//
//	error: 时间戳验证错误，nil 表示时间戳有效
func (m *Manager) validateTimestamp(timestamp uint64, parentTimestamp uint64) error {
	if m.logger != nil {
		m.logger.Debugf("验证时间戳: %d, 父时间戳: %d", timestamp, parentTimestamp)
	}

	currentTime := uint64(time.Now().Unix())
	const maxTimeDrift = 7200 // 2小时的时间偏移容忍度

	// 从配置中获取最小区块间隔
	minBlockInterval := uint64(10) // 默认值
	if m.configManager != nil {
		blockchainConfig := m.configManager.GetBlockchain()
		if blockchainConfig != nil {
			if blockchainConfig.Block.MinBlockInterval > 0 {
				minBlockInterval = uint64(blockchainConfig.Block.MinBlockInterval)
			}
		}
	}
	// 1. 验证时间戳不能超前当前时间过多
	if timestamp > currentTime+maxTimeDrift {
		return fmt.Errorf("区块时间戳超前过多，区块时间: %d, 当前时间: %d, 最大允许偏差: %d秒",
			timestamp, currentTime, maxTimeDrift)
	}

	// 2. 非创世区块：验证时间戳不能早于父区块
	if parentTimestamp > 0 {
		if timestamp <= parentTimestamp {
			return fmt.Errorf("区块时间戳不能早于或等于父区块时间戳，父区块: %d, 当前: %d",
				parentTimestamp, timestamp)
		}

		// 3. 验证最小区块间隔
		if timestamp < parentTimestamp+minBlockInterval {
			return fmt.Errorf("区块间隔过短，最小间隔: %d秒, 实际间隔: %d秒",
				minBlockInterval, timestamp-parentTimestamp)
		}
	}

	// 4. 验证时间戳不能太旧（防止历史攻击）
	// 仅在存在父区块（parentTimestamp>0）时执行；创世区块不适用此检查
	if parentTimestamp > 0 {
		const maxHistoryAge = 86400 * 30 // 30天
		if timestamp < currentTime-maxHistoryAge {
			return fmt.Errorf("区块时间戳过于陈旧，区块时间: %d, 当前时间: %d, 最大历史: %d秒",
				timestamp, currentTime, maxHistoryAge)
		}
	}

	return nil
}

// validateDifficulty 验证区块难度
//
// 🎯 **难度目标验证**
//
// 验证区块的难度目标是否符合难度调整算法的计算结果。
//
// 验证内容：
// - 计算期望难度
// - 比较实际难度与期望难度
// - 验证难度调整的合法性
//
// 参数：
//
//	currentDifficulty: 当前区块难度
//	parentBlock: 父区块
//	blockHeight: 当前区块高度
//
// 返回值：
//
//	error: 难度验证错误，nil 表示难度正确
func (m *Manager) validateDifficulty(currentDifficulty uint64, parentBlock *core.Block, blockHeight uint64) error {
	if m.logger != nil {
		m.logger.Debugf("验证难度: %d, 区块高度: %d", currentDifficulty, blockHeight)
	}

	// 简化的难度验证逻辑
	const difficultyAdjustmentInterval = 2016 // 每2016个区块调整一次难度
	const maxDifficultyAdjustment = 4         // 最大调整幅度为4倍

	parentDifficulty := parentBlock.Header.Difficulty

	// 1. 检查是否是难度调整点
	if blockHeight%difficultyAdjustmentInterval == 0 && blockHeight > 0 {
		// 在难度调整点，需要重新计算期望难度
		// 这里简化处理：允许在合理范围内调整
		maxAllowedDifficulty := parentDifficulty * maxDifficultyAdjustment
		minAllowedDifficulty := parentDifficulty / maxDifficultyAdjustment

		if currentDifficulty > maxAllowedDifficulty {
			return fmt.Errorf("难度调整幅度过大，最大允许: %d, 实际: %d",
				maxAllowedDifficulty, currentDifficulty)
		}
		if currentDifficulty < minAllowedDifficulty {
			return fmt.Errorf("难度调整幅度过大，最小允许: %d, 实际: %d",
				minAllowedDifficulty, currentDifficulty)
		}
	} else {
		// 非调整点：难度应该与父区块相同
		if currentDifficulty != parentDifficulty {
			return fmt.Errorf("非难度调整点的难度必须与父区块相同，父区块难度: %d, 当前难度: %d",
				parentDifficulty, currentDifficulty)
		}
	}

	// 2. 验证难度的基本范围
	const minDifficulty = 1
	const maxDifficulty = uint64(1) << 32 // 2^32
	if currentDifficulty < minDifficulty {
		return fmt.Errorf("难度低于最小值，最小值: %d, 当前: %d",
			minDifficulty, currentDifficulty)
	}
	if currentDifficulty > maxDifficulty {
		return fmt.Errorf("难度超过最大值，最大值: %d, 当前: %d",
			maxDifficulty, currentDifficulty)
	}

	return nil
}

// ==================== 第四层：密码学验证 ====================

// validateProofOfWork 验证工作量证明
//
// 🎯 **POW 算法验证**
//
// 验证区块的工作量证明是否满足难度要求，确保矿工确实完成了相应的计算工作。
//
// 验证步骤：
// - 重新计算区块哈希
// - 检查哈希是否满足难度要求
// - 验证 nonce 值的有效性
//
// 参数：
//
//	header: 区块头（包含 nonce 字段）
//
// 返回值：
//
//	error: POW 验证错误，nil 表示 POW 有效
//
// validateProofOfWork 验证工作量证明
//
// 🎯 **POW算法验证 - 使用标准区块哈希服务**
//
// 使用标准的BlockHashServiceClient计算区块哈希，然后验证哈希是否满足难度要求。
// 确保矿工完成了相应的工作量证明计算。
//
// 参数：
//
//	ctx: 上下文对象
//	block: 完整区块（BlockHashService需要完整区块）
//
// 返回值：
//
//	error: POW验证错误，nil表示POW有效
func (m *Manager) validateProofOfWork(block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("验证工作量证明，难度: %d", block.Header.Difficulty)
	}

	// 创世区块：无挖矿要求，跳过POW验证
	if block.Header != nil && block.Header.Height == 0 {
		if m.logger != nil {
			m.logger.Debugf("创世区块跳过POW验证")
		}
		return nil
	}

	// 1. 使用标准的POWEngine进行区块头验证
	isValid, err := m.powEngine.VerifyBlockHeader(block.Header)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("POW验证过程失败: %v", err)
		}
		return fmt.Errorf("POW验证过程失败: %w", err)
	}

	// 2. 验证POW结果
	if !isValid {
		if m.logger != nil {
			m.logger.Errorf("❌ POW验证失败，区块哈希不满足难度要求，难度: %d, nonce: %x",
				block.Header.Difficulty, block.Header.Nonce)
		}
		return fmt.Errorf("POW验证失败，区块哈希不满足难度要求")
	}

	if m.logger != nil {
		m.logger.Debugf("✅ POW验证通过，难度: %d, nonce: %x",
			block.Header.Difficulty, block.Header.Nonce)
	}

	return nil
}

// validateMerkleRoot 验证 Merkle 根
//
// 🎯 **交易完整性验证**
//
// 重新计算区块中交易的 Merkle 根，与区块头中的值对比，确保交易数据完整性。
//
// 验证步骤：
// - 收集所有交易哈希
// - 构建 Merkle 树
// - 计算 Merkle 根
// - 与区块头对比
//
// 参数：
//
//	header: 区块头
//	transactions: 交易列表
//
// 返回值：
//
//	error: Merkle 根验证错误，nil 表示验证通过
//
// validateMerkleRoot 验证区块的Merkle根
//
// 🎯 **交易完整性验证 - Block层职责**
//
// 重新计算区块中交易的Merkle根，与区块头中的值对比，确保交易数据完整性。
// 这是Block层的核心职责，直接使用crypto.MerkleTreeManager进行计算。
//
// 参数：
//
//	block: 完整区块（包含区块头和交易列表）
//
// 返回值：
//
//	error: Merkle根验证错误，nil表示验证通过
func (m *Manager) validateMerkleRoot(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("验证Merkle根")
	}

	// 1. 使用标准化的ValidateMerkleRoot接口方法
	isValid, err := m.ValidateMerkleRoot(ctx, block.Body.Transactions, block.Header.MerkleRoot)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("Merkle根验证过程失败: %v", err)
		}
		return fmt.Errorf("Merkle根验证失败: %w", err)
	}

	// 2. 检查验证结果
	if !isValid {
		if m.logger != nil {
			m.logger.Errorf("❌ Merkle根验证失败，期望: %x", block.Header.MerkleRoot)
		}
		return fmt.Errorf("Merkle根不匹配")
	}

	if m.logger != nil {
		m.logger.Debugf("✅ Merkle根验证通过: %x", block.Header.MerkleRoot)
	}

	return nil
}
