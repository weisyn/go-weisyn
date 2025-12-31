// Package fork 分叉检测实现
package fork

import (
	"bytes"
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ============================================================================
//                              分叉检测实现
// ============================================================================

// detectFork 检测是否存在分叉
//
// 🎯 **分叉检测核心逻辑**
//
// 检测方法：
// 1. 获取区块的父哈希
// 2. 获取当前主链在该高度的区块
// 3. 比较父哈希是否匹配
// 4. 如果不匹配，向前回溯查找分叉点
//
// 返回：
//   - isFork: 是否是分叉
//   - forkHeight: 分叉点高度
//   - error: 检测错误
func (s *Service) detectFork(ctx context.Context, block *core.Block) (bool, uint64, error) {
	if block == nil || block.Header == nil {
		return false, 0, fmt.Errorf("无效的区块")
	}

	blockHeight := block.Header.Height
	parentHash := block.Header.PreviousHash

	if s.logger != nil {
		s.logger.Debugf("检测分叉: 区块高度=%d, 父哈希=%x",
			blockHeight, parentHash[:min(8, len(parentHash))])
	}

	// 1. 获取当前链信息
	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("获取链信息失败: %w", err)
	}

	currentHeight := chainInfo.Height

	// 2. 如果新区块高度小于等于当前高度，可能是分叉
	if blockHeight <= currentHeight {
		// 获取主链在该高度-1的区块哈希
		if blockHeight == 0 {
			// 创世区块不会有分叉
			return false, 0, nil
		}

		// 获取主链在 blockHeight-1 的区块
		mainChainBlock, err := s.queryService.GetBlockByHeight(ctx, blockHeight-1)
		if err != nil {
			return false, 0, fmt.Errorf("获取主链区块失败: %w", err)
		}

		// 计算主链区块哈希
		mainChainBlockHash, err := s.calculateBlockHash(ctx, mainChainBlock.Header)
		if err != nil {
			return false, 0, fmt.Errorf("计算主链区块哈希失败: %w", err)
		}

		// 比较父哈希
		if !bytes.Equal(mainChainBlockHash, parentHash) {
			// 发现分叉
			if s.logger != nil {
				s.logger.Infof("🔍 检测到分叉: 主链块哈希=%x, 新块父哈希=%x",
					mainChainBlockHash[:min(8, len(mainChainBlockHash))], parentHash[:min(8, len(parentHash))])
			}

			// 向前回溯查找分叉点
			forkHeight, err := s.findForkPoint(ctx, block)
			if err != nil {
				return true, blockHeight - 1, fmt.Errorf("查找分叉点失败: %w", err)
			}

			return true, forkHeight, nil
		}
	}

	// 3. 如果新区块是当前链的直接后继，不是分叉
	if blockHeight == currentHeight+1 {
		// 获取当前链尖
		bestBlock, err := s.queryService.GetBlockByHeight(ctx, currentHeight)
		if err != nil {
			return false, 0, fmt.Errorf("获取链尖区块失败: %w", err)
		}

		// 计算链尖区块哈希
		bestBlockHash, err := s.calculateBlockHash(ctx, bestBlock.Header)
		if err != nil {
			return false, 0, fmt.Errorf("计算链尖区块哈希失败: %w", err)
		}

		// 检查父哈希是否匹配
		if bytes.Equal(bestBlockHash, parentHash) {
			// 正常的下一个区块，不是分叉
			if s.logger != nil {
				s.logger.Debugf("✅ 正常后继区块: 高度=%d", blockHeight)
			}
			return false, 0, nil
		}

		// 父哈希不匹配，这是分叉
		if s.logger != nil {
			s.logger.Infof("🔍 检测到分叉（直接后继）: 链尖哈希=%x, 新块父哈希=%x",
				bestBlockHash[:min(8, len(bestBlockHash))], parentHash[:min(8, len(parentHash))])
		}

		// 向前回溯查找分叉点
		forkHeight, err := s.findForkPoint(ctx, block)
		if err != nil {
			return true, currentHeight, fmt.Errorf("查找分叉点失败: %w", err)
		}

		return true, forkHeight, nil
	}

	// 4. 其他情况：新区块高度远大于当前高度，可能是缺失区块
	if blockHeight > currentHeight+1 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 区块高度跳跃: 当前=%d, 新区块=%d", currentHeight, blockHeight)
		}
		return false, 0, fmt.Errorf("区块高度不连续: 当前=%d, 新区块=%d", currentHeight, blockHeight)
	}

	return false, 0, nil
}

// ============================================================================
//                              分叉点查找
// ============================================================================

// findForkPoint 查找分叉点
//
// 🔍 **向前回溯查找共同祖先**
//
// 算法：
// 1. 从分叉区块的父区块开始
// 2. 向前回溯，直到找到主链上存在的区块
// 3. 该区块的高度即为分叉点
//
// 返回：
//   - 分叉点高度
//   - error: 查找错误
func (s *Service) findForkPoint(ctx context.Context, forkBlock *core.Block) (uint64, error) {
	if forkBlock == nil || forkBlock.Header == nil {
		return 0, fmt.Errorf("无效的分叉区块")
	}

	// 共同祖先查找的正确语义：
	// - 我们要找到“最高的共同祖先区块高度”（main chain 与 fork chain 在该高度 hash 相同）
	// - 必须沿 fork 链向前（父哈希）逐步回溯，不能用“主链 previousHash”去伪推导 fork 链（那是错误的）
	//
	// 在“非 sync v2 自动 reorg”场景（例如收到分叉区块事件后再处理）下：
	// - 期望 fork 链上的祖先块能够通过 GetBlockByHash 从本地存储取回
	// - 若缺失祖先块，需要触发同步来补齐（否则无法严谨定位共同祖先）
	currentHash := forkBlock.Header.PreviousHash
	currentHeight := forkBlock.Header.Height - 1

	if s.logger != nil {
		s.logger.Debugf("查找分叉点: 从高度=%d 开始", currentHeight)
	}

	// 向前回溯，最多回溯 N 个区块（从配置获取，默认 100）
	maxBacktrack := s.getMaxForkBacktrack()
	for i := 0; i < maxBacktrack; i++ {
		// 1) 取主链同高度块并计算 hash
		mainChainBlock, err := s.queryService.GetBlockByHeight(ctx, currentHeight)
		if err != nil || mainChainBlock == nil || mainChainBlock.Header == nil {
			// 主链该高度不存在：说明本地主链不足或索引损坏，无法确定共同祖先
			return 0, fmt.Errorf("主链缺失高度=%d 的区块，无法定位共同祖先: %w", currentHeight, err)
		}
		mainHash, err := s.calculateBlockHash(ctx, mainChainBlock.Header)
		if err != nil {
			return 0, fmt.Errorf("计算主链区块哈希失败 (height=%d): %w", currentHeight, err)
		}

		// 2) 对比：fork 链当前候选 hash 是否与主链一致
		if bytes.Equal(mainHash, currentHash) {
			if s.logger != nil {
				s.logger.Infof("✅ 找到分叉点(共同祖先): 高度=%d, 哈希=%x",
					currentHeight, mainHash[:min(8, len(mainHash))])
			}
			return currentHeight, nil
		}

		// 3) 继续沿 fork 链回溯：必须从 fork 链上按 hash 取回父块，再读取其 PreviousHash
		//    如果 fork 链祖先不在本地存储，则无法严谨定位共同祖先（应触发同步补齐）。
		forkAncestor, err := s.queryService.GetBlockByHash(ctx, currentHash)
		if err != nil || forkAncestor == nil || forkAncestor.Header == nil {
			if s.logger != nil {
				s.logger.Warnf("⚠️ 无法从本地按 hash 获取 fork 祖先块: height=%d hash=%x err=%v（需要同步补齐祖先块后再定位分叉点）",
					currentHeight, currentHash[:min(8, len(currentHash))], err)
			}
			return 0, fmt.Errorf("fork 祖先区块缺失(hash=%x height~%d)，无法定位分叉点，请先同步补齐: %w",
				currentHash[:min(8, len(currentHash))], currentHeight, err)
		}
		// 安全性：尽量使用 forkAncestor.Header.Height 更新 currentHeight，避免仅靠外部传入高度推演
		if forkAncestor.Header.Height == 0 {
			// 回溯到创世区块：共同祖先只能是 0
			return 0, nil
		}
		currentHeight = forkAncestor.Header.Height - 1
		currentHash = forkAncestor.Header.PreviousHash
	}

	// 回溯次数超过限制
	if s.logger != nil {
		s.logger.Errorf("❌ 无法找到分叉点：回溯次数超过限制 (%d 层)，当前配置 blockchain.sync.advanced.auto_reorg_max_depth=%d。"+
			"这通常意味着发生了异常深度的分叉或长时间网络分区，建议："+
			"1) 检查节点与上游的网络连接；2) 评估是否需要临时调高 auto_reorg_max_depth；3) 必要时执行离线重组/重建节点。",
			maxBacktrack, s.getMaxForkBacktrack())
	}

	return 0, fmt.Errorf("无法找到分叉点，回溯次数超过限制: %d（受 blockchain.sync.advanced.auto_reorg_max_depth 限制）", maxBacktrack)
}

// ============================================================================
//                              辅助函数
// ============================================================================

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateBlockHash 计算区块哈希
//
// 🎯 **区块哈希计算辅助方法**
//
// 复用 block/shared/hash.go 的 CalculateBlockHash 函数
//
// 参数：
//   - header: 区块头
//
// 返回：
//   - []byte: 区块哈希
//   - error: 计算错误

// calculateBlockHash 计算区块哈希（辅助方法，使用 gRPC 服务）
func (s *Service) calculateBlockHash(ctx context.Context, header *core.BlockHeader) ([]byte, error) {
	if header == nil {
		return nil, fmt.Errorf("区块头为空")
	}
	if s.blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 未初始化")
	}

	// 构建区块（只有Header）
	block := &core.Block{
		Header: header,
	}

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

	return resp.Hash, nil
}
