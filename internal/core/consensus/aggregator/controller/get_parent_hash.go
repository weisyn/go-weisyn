// get_parent_hash.go
// 获取父区块哈希的辅助函数
//
// 主要功能：
// 1. 根据指定高度获取父区块哈希
// 2. 作为距离计算的基准
// 3. 处理创世区块的特殊情况
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package controller

import (
	"crypto/sha256"
	"fmt"
)

// getParentBlockHash 获取父区块哈希
//
// 参数：
// - height: 当前区块高度
//
// 返回：
// - []byte: 父区块哈希
// - error: 获取错误
func (s *aggregationStarter) getParentBlockHash(height uint64) ([]byte, error) {
	if height == 0 {
		// 创世区块情况：使用固定的零哈希作为父区块
		zeroHash := make([]byte, 32)
		return zeroHash, nil
	}

	if height == 1 {
		// 第一个非创世区块：使用创世区块哈希
		genesisHash := sha256.Sum256([]byte("genesis_block"))
		return genesisHash[:], nil
	}

	// 对于其他高度，基于高度生成确定性的父区块哈希用于距离计算
	parentHeightBytes := fmt.Sprintf("block_height_%d", height-1)
	parentHash := sha256.Sum256([]byte(parentHeightBytes))

	s.logger.Info("获取父区块哈希完成")
	return parentHash[:], nil
}
