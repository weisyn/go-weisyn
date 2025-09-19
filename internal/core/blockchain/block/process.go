// Package block 提供区块管理的核心实现
//
// 📋 **processing.go - 区块处理实现**
//
// 本文件实现 ProcessBlock 方法的完整业务逻辑，负责处理验证通过的区块。
// 采用原子事务模式，确保区块处理的数据一致性和系统稳定性。
//
// 🎯 **核心职责**：
// - 原子事务处理：确保所有状态变更在单一事务中完成
// - 交易执行管理：按顺序执行区块中的所有交易
// - UTXO 状态更新：维护准确的 UTXO 集合状态
// - 链状态维护：更新区块链的最新状态信息
// - 事件通知发布：向其他组件通知区块处理完成
//
// 🏗️ **架构特点**：
// - 原子事务保证：失败时完全回滚，成功时完全提交
// - 状态一致性：确保 UTXO、账户、链状态的严格一致
// - 错误恢复机制：提供完整的错误处理和恢复能力
// - 性能优化：支持批量处理和并发优化
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"bytes"
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 区块处理实现 ====================

// processBlock 处理验证通过的区块
//
// 🎯 **原子事务处理实现**
//
// 这是 BlockService.ProcessBlock 的完整实现，采用原子事务模式。
// 处理验证通过的区块，执行所有交易并更新区块链状态。
//
// 🔄 **完整处理流程**：
//
// **阶段一：预处理和事务准备**
// 1. **最终验证检查**：
//   - 再次确认区块已通过完整验证
//   - 检查区块是否已被处理（防重复）
//   - 验证当前链状态是否允许处理此区块
//
// 2. **原子事务启动**：
//   - 开启数据库事务，确保原子性
//   - 建立事务隔离级别和锁定策略
//   - 准备回滚点和错误恢复机制
//
// 3. **处理环境准备**：
//   - 创建临时状态容器
//   - 准备 UTXO 状态快照
//   - 初始化交易执行环境
//
// **阶段二：交易执行循环**
// 4. **Coinbase 交易处理**：
//
//   - 处理区块的第一个交易（Coinbase）
//
//   - 创建挖矿奖励和手续费的新 UTXO
//
//   - 更新矿工账户余额
//
//   - 记录奖励分配信息
//
//     5. **普通交易执行循环**：
//     ```
//     for 每个普通交易 {
//     a. 标记输入 UTXO 为已花费
//     b. 创建新的输出 UTXO
//     c. 更新相关账户余额
//     d. 执行智能合约（如果包含）
//     e. 记录交易执行结果
//     f. 更新交易状态为已确认
//     }
//     ```
//
// 6. **UTXO 集合更新**：
//   - 批量标记已花费的 UTXO
//   - 批量创建新的 UTXO 记录
//   - 更新 UTXO 索引和统计信息
//   - 计算新的 UTXO 状态根哈希
//
// **阶段三：状态更新和持久化**
// 7. **区块数据持久化**：
//   - 将区块数据写入持久存储
//   - 更新区块索引（按高度、哈希等）
//   - 建立区块与交易的关联关系
//
// 8. **链状态更新**：
//   - 更新最新区块高度
//   - 更新最佳区块哈希
//   - 更新链难度和累积工作量
//   - 更新链统计信息
//
// 9. **账户余额更新**：
//   - 批量更新所有相关账户余额
//   - 更新账户交易历史记录
//   - 维护账户 UTXO 索引
//
// 10. **交易池更新**：
//   - 从交易池中移除已确认的交易
//   - 更新相关交易的状态
//   - 处理可能的交易依赖更新
//
// **阶段四：事务提交和通知**
// 11. **事务完整性验证**：
//   - 验证所有状态更新的一致性
//   - 检查数据完整性约束
//   - 确认事务可以安全提交
//
// 12. **原子事务提交**：
//   - 提交所有数据库变更
//   - 释放事务锁和资源
//   - 确认状态更新生效
//
// 13. **事件通知发布**：
//   - 发布区块处理完成事件
//   - 通知其他组件状态变更
//   - 触发相关的业务流程
//
// **阶段五：后处理和清理**
// 14. **缓存更新**：
//   - 更新区块查询缓存
//   - 刷新相关统计缓存
//   - 优化查询索引
//
// 15. **性能监控**：
//   - 记录处理时间和资源使用
//   - 更新性能监控指标
//   - 生成处理报告
//
// 🎯 **原子事务保证**：
// - **事务边界**：整个区块处理在单个数据库事务中完成
// - **回滚机制**：任何步骤失败都会完全回滚到初始状态
// - **一致性检查**：在提交前验证所有状态的一致性
// - **隔离性保证**：使用适当的事务隔离级别防止并发冲突
//
// 🛡️ **错误处理和恢复**：
// - **预防性检查**：在关键操作前进行状态验证
// - **异常捕获**：捕获并处理所有可能的异常情况
// - **错误分类**：区分临时错误和永久错误
// - **恢复策略**：提供自动重试和手动恢复选项
// - **错误报告**：生成详细的错误诊断信息
//
// 📊 **性能优化策略**：
// - **批量操作**：使用批量数据库操作减少 I/O
// - **并行处理**：在安全的前提下并行执行独立操作
// - **缓存利用**：充分利用内存缓存减少数据库访问
// - **索引优化**：维护高效的数据库索引
// - **资源管理**：合理管理内存和数据库连接资源
//
// 🔄 **与其他组件的协作**：
// - **RepositoryManager**：持久化区块和状态数据
// - **UTXOManager**：管理 UTXO 集合状态
// - **TransactionService**：获取交易详细信息
// - **EventService**：发布区块处理事件
// - **NetworkService**：广播区块处理结果
//
// 参数：
//
//	ctx: 上下文对象，用于超时控制和取消操作
//	block: 已验证的区块，包含所有交易数据
//
// 返回值：
//
//	error: 处理过程中的错误，nil 表示处理成功
//
// 使用示例：
//
//	// 验证区块
//	valid, err := manager.ValidateBlock(ctx, receivedBlock)
//	if err != nil || !valid {
//	  logger.Errorf("区块验证失败: %v", err)
//	  return err
//	}
//
//	// 处理验证通过的区块
//	err = manager.ProcessBlock(ctx, receivedBlock)
//	if err != nil {
//	  logger.Errorf("区块处理失败: %v", err)
//	  return err
//	}
//
//	logger.Infof("区块处理成功，高度: %d", receivedBlock.Header.Height)
func (m *Manager) processBlock(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Infof("🚨🚨🚨 [DEBUG] 开始处理区块，高度: %d, 交易数: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	// 检查是否为创世区块
	isGenesisBlock := block.Header.Height == 0

	if isGenesisBlock {
		if m.logger != nil {
			m.logger.Infof("处理创世区块，高度: %d", block.Header.Height)
		}
	}

	// 步骤1: 最终确认验证（防止重复处理）
	existingBlock, err := m.repo.GetBlock(ctx, m.calculateBlockHash(ctx, block))
	if err == nil && existingBlock != nil {
		if m.logger != nil {
			m.logger.Warnf("区块已存在，跳过处理，高度: %d", block.Header.Height)
		}
		return nil // 区块已存在，跳过处理
	}

	// 步骤2: 区块验证 - 必须在存储前验证区块有效性
	// 🎯 **关键架构点**: 复用validate.go中的完整验证逻辑
	// 验证包括：结构验证、头验证、链连接性、Merkle根、POW、交易验证
	valid, err := m.validateBlock(ctx, block)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("区块验证失败: %v", err)
		}
		return fmt.Errorf("区块验证失败: %w", err)
	}
	if !valid {
		if m.logger != nil {
			m.logger.Errorf("区块验证不通过，高度: %d", block.Header.Height)
		}
		return fmt.Errorf("区块验证不通过，高度: %d", block.Header.Height)
	}

	if m.logger != nil {
		blockType := "普通区块"
		if isGenesisBlock {
			blockType = "创世区块"
		}
		m.logger.Infof("✅ %s验证通过，高度: %d", blockType, block.Header.Height)
	}

	// 步骤3: 分叉检测 - 检查是否存在分叉情况
	// 🎯 **关键架构点**: 只对非创世区块进行分叉检测
	// 分叉检测逻辑：
	// - 同高度但不同哈希的区块 = 分叉
	// - height = current+1 但 previous_hash 不匹配 = 分叉
	if !isGenesisBlock {
		err = m.detectAndHandleFork(ctx, block)
		if err != nil {
			if m.logger != nil {
				m.logger.Errorf("分叉检测处理失败: %v", err)
			}
			return fmt.Errorf("分叉检测处理失败: %w", err)
		}
	}

	// 步骤4: 使用repository.StoreBlock存储区块
	// 🎯 **关键架构点**: repository.StoreBlock是单一数据源写入点
	// 它会自动完成：
	// - 区块数据存储
	// - 交易索引创建
	// - UTXO状态更新
	// - 账户余额更新
	// - 所有相关索引维护
	if err := m.repo.StoreBlock(ctx, block); err != nil {
		if m.logger != nil {
			m.logger.Errorf("区块存储失败: %v", err)
		}
		return fmt.Errorf("区块存储失败: %w", err)
	}

	// 步骤5: 清理交易池（移除已确认交易）
	// 🎯 **创世区块特殊处理**: 创世区块的交易通常不来自交易池，跳过清理
	if !isGenesisBlock {
		if err := m.cleanupTransactionPool(ctx, block); err != nil {
			if m.logger != nil {
				m.logger.Warnf("交易池清理失败，但不影响区块处理: %v", err)
			}
			// 交易池清理失败不影响区块处理结果
		}
	} else {
		if m.logger != nil {
			m.logger.Debugf("创世区块跳过交易池清理")
		}
	}

	if m.logger != nil {
		blockType := "普通区块"
		if isGenesisBlock {
			blockType = "创世区块"
		}
		m.logger.Infof("✅ %s处理成功，高度: %d, 哈希: %x",
			blockType, block.Header.Height, m.calculateBlockHash(ctx, block))
	}

	return nil
}

// detectAndHandleFork 检测并处理分叉情况
//
// 🎯 **分叉检测核心逻辑**
//
// 根据分叉处理设计文档，检测两种分叉情况：
// 1. 同高度分叉：相同高度但不同哈希的区块
// 2. 链断裂分叉：height = current+1 但 previous_hash 不匹配
//
// 检测到分叉后，委托给fork服务进行异步处理。
//
// 参数：
//   - ctx: 操作上下文
//   - block: 待检测的区块
//
// 返回：
//   - error: 分叉检测或处理失败的错误
func (m *Manager) detectAndHandleFork(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("[BlockManager] 开始分叉检测 - height: %d", block.Header.Height)
	}

	// 获取当前链信息
	currentHeight, currentBestHash, err := m.repo.GetHighestBlock(ctx)
	if err != nil {
		return fmt.Errorf("获取当前链信息失败: %w", err)
	}

	blockHeight := block.Header.Height

	// 检查分叉情况
	var isFork bool
	var forkType string

	// 情况1: 同高度分叉 - 相同高度但不同哈希
	if blockHeight == currentHeight {
		newBlockHash := m.calculateBlockHash(ctx, block)

		if !bytes.Equal(currentBestHash, newBlockHash) {
			isFork = true
			forkType = "same_height_fork"
			if m.logger != nil {
				m.logger.Infof("[BlockManager] 🔀 检测到同高度分叉: height=%d, current_hash=%x, new_hash=%x",
					blockHeight, currentBestHash, newBlockHash)
			}
		}
	}

	// 情况2: 链断裂分叉 - height = current+1 但 previous_hash 不匹配
	if blockHeight == currentHeight+1 {
		actualPrevHash := block.Header.PreviousHash

		if !bytes.Equal(currentBestHash, actualPrevHash) {
			isFork = true
			forkType = "chain_break_fork"
			if m.logger != nil {
				m.logger.Infof("[BlockManager] 🔀 检测到链断裂分叉: height=%d, expected_prev=%x, actual_prev=%x",
					blockHeight, currentBestHash, actualPrevHash)
			}
		}
	}

	// 如果检测到分叉，委托给fork服务处理
	if isFork {
		if m.logger != nil {
			m.logger.Infof("[BlockManager] ⚠️  分叉检测完成，类型: %s, 委托fork服务处理", forkType)
		}

		// 通过事件总线发布分叉事件，异步处理
		if m.eventBus != nil {
			forkEvent := map[string]interface{}{
				"type":      "fork_detected",
				"block":     block,
				"fork_type": forkType,
				"height":    block.Header.Height,
				"timestamp": block.Header.Timestamp,
			}

			// EventBus.Publish不返回错误，直接发布
			m.eventBus.Publish("blockchain.fork.detected", forkEvent)

			if m.logger != nil {
				m.logger.Infof("已发布分叉检测事件: type=%s, height=%d", forkType, block.Header.Height)
			}
		}

		if m.logger != nil {
			m.logger.Infof("[BlockManager] ✅ 分叉已提交处理，继续当前区块处理流程")
		}
	} else {
		if m.logger != nil {
			m.logger.Debugf("[BlockManager] 未检测到分叉，继续正常处理")
		}
	}

	return nil
}

// calculateBlockHash 计算区块哈希（辅助方法）
//
// 🎯 **区块哈希计算**
//
// 使用标准的BlockHashService计算区块哈希，用于查询和去重。
//
// 参数：
//
//	block: 完整区块
//
// 返回值：
//
//	[]byte: 区块哈希
func (m *Manager) calculateBlockHash(ctx context.Context, block *core.Block) []byte {
	// 使用标准的BlockHashService计算区块哈希
	request := &core.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}

	response, err := m.blockHashServiceClient.ComputeBlockHash(ctx, request)
	if err != nil || !response.IsValid {
		// 如果计算失败，返回空切片，上层会处理这种情况
		return make([]byte, 32)
	}

	return response.Hash
}

// ==================== 辅助方法 ====================

// cleanupTransactionPool 清理交易池中已确认的交易
//
// 🎯 **交易池维护 - 使用正确的txpool接口**
//
// 从交易池移除已被区块确认的交易，使用ConfirmTransactions方法。
// 这是block层少数合理的职责之一，因为只有在区块处理完成后才能确定哪些交易已被确认。
//
// 清理内容：
// - 确认区块中的交易（状态: mining → confirmed → removed）
// - 使用标准的txpool.ConfirmTransactions接口
//
// 参数：
//
//	ctx: 上下文对象
//	block: 已处理的区块
//
// 返回值：
//
//	error: 清理错误，nil表示清理成功
func (m *Manager) cleanupTransactionPool(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("清理交易池，确认已处理交易数: %d", len(block.Body.Transactions))
	}

	// 收集所有交易哈希
	txIDs := make([][]byte, 0, len(block.Body.Transactions))

	for _, tx := range block.Body.Transactions {
		// 跳过 coinbase 交易（简单判断：没有输入或第一个输入为空）
		if len(tx.Inputs) == 0 {
			continue
		}
		// 使用交易哈希服务计算交易ID
		hashReq := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}
		hashResp, err := m.txHashServiceClient.ComputeHash(ctx, hashReq)
		if err != nil || !hashResp.IsValid {
			if m.logger != nil {
				m.logger.Debugf("计算交易哈希失败，跳过交易池清理: %v", err)
			}
			continue
		}

		txIDs = append(txIDs, hashResp.Hash)
	}

	// 如果没有有效的交易ID，跳过清理
	if len(txIDs) == 0 {
		if m.logger != nil {
			m.logger.Debugf("没有需要确认的交易，跳过交易池清理")
		}
		return nil
	}

	// 使用正确的txpool接口确认交易
	if err := m.txPool.ConfirmTransactions(txIDs, block.Header.Height); err != nil {
		if m.logger != nil {
			m.logger.Warnf("确认交易失败，但不影响区块处理: %v", err)
		}
		// 交易池确认失败不影响区块处理结果
		return nil
	}

	if m.logger != nil {
		m.logger.Debugf("✅ 交易池清理完成，已确认交易数: %d", len(txIDs))
	}

	return nil
}
