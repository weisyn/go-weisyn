// Package sync 提供同步中的区块临时存储功能
//
// ✅ P1修复：实现临时存储机制，支持乱序接收和连续性检测
package sync

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                           临时存储辅助函数
// ============================================================================

// storeBlocksInTempStore 将区块存储到临时存储
//
// 🎯 **临时存储策略**：
// 1. 序列化区块数据
// 2. 生成临时文件ID：sync_pending_{height:010d}_{hash:8}
// 3. 存储到 TempStore：sync/pending/{id}.block
//
// 参数：
//   - ctx: 上下文
//   - tempStore: 临时存储服务
//   - blocks: 要存储的区块列表
//   - logger: 日志记录器
//
// 返回：
//   - []string: 存储的临时文件ID列表
//   - error: 存储错误
func storeBlocksInTempStore(
	ctx context.Context,
	tempStore storage.TempStore,
	blocks []*core.Block,
	logger log.Logger,
) ([]string, error) {
	if tempStore == nil {
		return nil, fmt.Errorf("tempStore 未初始化")
	}

	if len(blocks) == 0 {
		return nil, nil
	}

	var tempFileIDs []string

	for _, block := range blocks {
		if block == nil || block.Header == nil {
			continue
		}

		// 序列化区块
		blockData, err := proto.Marshal(block)
		if err != nil {
			return nil, fmt.Errorf("序列化区块失败 (高度=%d): %w", block.Header.Height, err)
		}

		// 计算区块哈希（简化：使用高度和部分数据生成唯一ID）
		// 实际实现中应该使用真实的区块哈希
		height := block.Header.Height
		hashPrefix := ""
		if len(block.Header.PreviousHash) >= 8 {
			hashPrefix = hex.EncodeToString(block.Header.PreviousHash[:8])
		} else {
			hashPrefix = fmt.Sprintf("%010d", height)
		}

		// 生成临时文件ID：sync_pending_{height:010d}_{hash:8}
		tempFileID := fmt.Sprintf("sync_pending_%010d_%s", height, hashPrefix)

		// 存储到 TempStore
		// 使用 CreateTempFileWithContent 创建临时文件
		// prefix: "sync_pending", suffix: ".block"
		id, err := tempStore.CreateTempFileWithContent(ctx, "sync_pending", ".block", blockData)
		if err != nil {
			return nil, fmt.Errorf("存储区块到临时存储失败 (高度=%d): %w", height, err)
		}

		// 如果返回的ID与预期不同，使用返回的ID
		if id != "" {
			tempFileID = id
		}

		tempFileIDs = append(tempFileIDs, tempFileID)

		if logger != nil {
			logger.Debugf("✅ 区块已存储到临时存储: height=%d, tempID=%s", height, tempFileID)
		}
	}

	return tempFileIDs, nil
}

// loadBlocksFromTempStore 从临时存储加载区块
//
// 🎯 **加载策略**：
// 1. 根据临时文件ID列表加载区块
// 2. 反序列化区块数据
// 3. 按高度排序
//
// 参数：
//   - ctx: 上下文
//   - tempStore: 临时存储服务
//   - tempFileIDs: 临时文件ID列表
//   - logger: 日志记录器
//
// 返回：
//   - []*core.Block: 加载的区块列表（按高度排序）
//   - error: 加载错误
func loadBlocksFromTempStore(
	ctx context.Context,
	tempStore storage.TempStore,
	tempFileIDs []string,
	logger log.Logger,
) ([]*core.Block, error) {
	if tempStore == nil {
		return nil, fmt.Errorf("tempStore 未初始化")
	}

	if len(tempFileIDs) == 0 {
		return nil, nil
	}

	var blocks []*core.Block

	for _, tempFileID := range tempFileIDs {
		// 从 TempStore 读取区块数据
		blockData, err := tempStore.GetTempFile(ctx, tempFileID)
		if err != nil {
			if logger != nil {
				logger.Warnf("从临时存储加载区块失败 (ID=%s): %v，跳过", tempFileID, err)
			}
			continue
		}

		// 反序列化区块
		block := &core.Block{}
		if err := proto.Unmarshal(blockData, block); err != nil {
			if logger != nil {
				logger.Warnf("反序列化区块失败 (ID=%s): %v，跳过", tempFileID, err)
			}
			continue
		}

		blocks = append(blocks, block)
	}

	// 按高度排序
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Header.Height < blocks[j].Header.Height
	})

	return blocks, nil
}

// removeBlocksFromTempStore 从临时存储删除区块
//
// 🎯 **清理策略**：
// 1. 删除指定的临时文件
// 2. 忽略不存在的文件错误
//
// 参数：
//   - ctx: 上下文
//   - tempStore: 临时存储服务
//   - tempFileIDs: 要删除的临时文件ID列表
//   - logger: 日志记录器
func removeBlocksFromTempStore(
	ctx context.Context,
	tempStore storage.TempStore,
	tempFileIDs []string,
	logger log.Logger,
) {
	if tempStore == nil {
		return
	}

	for _, tempFileID := range tempFileIDs {
		if err := tempStore.RemoveTempFile(ctx, tempFileID); err != nil {
			if logger != nil {
				logger.Warnf("删除临时区块文件失败 (ID=%s): %v", tempFileID, err)
			}
		} else if logger != nil {
			logger.Debugf("✅ 临时区块文件已删除: ID=%s", tempFileID)
		}
	}
}

// findContinuousBlocks 查找连续区块（从指定高度开始）
//
// 🎯 **连续性检测策略**：
// 1. 从 TempStore 列出所有待处理区块
// 2. 查找从 startHeight 开始的连续区块
// 3. 返回连续区块列表和下一个缺失的高度
//
// 参数：
//   - ctx: 上下文
//   - tempStore: 临时存储服务
//   - startHeight: 起始高度
//   - maxBlocks: 最大返回区块数
//   - logger: 日志记录器
//
// 返回：
//   - []*core.Block: 连续区块列表
//   - uint64: 下一个缺失的高度（如果没有缺失，返回 0）
//   - error: 查找错误
func findContinuousBlocks(
	ctx context.Context,
	tempStore storage.TempStore,
	startHeight uint64,
	maxBlocks int,
	logger log.Logger,
) ([]*core.Block, uint64, error) {
	if tempStore == nil {
		return nil, 0, fmt.Errorf("tempStore 未初始化")
	}

	// 列出所有临时文件
	tempFiles, err := tempStore.ListTempFiles(ctx, "sync_pending_*")
	if err != nil {
		return nil, 0, fmt.Errorf("列出临时文件失败: %w", err)
	}

	// 加载所有区块
	var allBlocks []*core.Block
	for _, tempFile := range tempFiles {
		blockData, err := tempStore.GetTempFile(ctx, tempFile.ID)
		if err != nil {
			if logger != nil {
				logger.Warnf("加载临时区块失败 (ID=%s): %v，跳过", tempFile.ID, err)
			}
			continue
		}

		block := &core.Block{}
		if err := proto.Unmarshal(blockData, block); err != nil {
			if logger != nil {
				logger.Warnf("反序列化临时区块失败 (ID=%s): %v，跳过", tempFile.ID, err)
			}
			continue
		}

		allBlocks = append(allBlocks, block)
	}

	// 按高度排序（过滤掉 nil 区块）
	validBlocks := make([]*core.Block, 0, len(allBlocks))
	for _, block := range allBlocks {
		if block != nil && block.Header != nil {
			validBlocks = append(validBlocks, block)
		}
	}

	sort.Slice(validBlocks, func(i, j int) bool {
		return validBlocks[i].Header.Height < validBlocks[j].Header.Height
	})

	// 查找从 startHeight 开始的连续区块
	var continuousBlocks []*core.Block
	expectedHeight := startHeight

	for _, block := range validBlocks {
		// 再次检查 block 和 Header（虽然已过滤，但确保安全）
		if block == nil || block.Header == nil {
			continue
		}

		if block.Header.Height < startHeight {
			continue // 跳过低于起始高度的区块
		}

		if block.Header.Height == expectedHeight {
			// 找到连续区块
			continuousBlocks = append(continuousBlocks, block)
			expectedHeight++

			// 达到最大数量限制
			if len(continuousBlocks) >= maxBlocks {
				break
			}
		} else if block.Header.Height > expectedHeight {
			// 发现缺失：expectedHeight 缺失
			return continuousBlocks, expectedHeight, nil
		}
	}

	// 如果没有缺失，返回所有连续区块
	if len(continuousBlocks) == 0 {
		return nil, startHeight, nil
	}

	return continuousBlocks, 0, nil
}

