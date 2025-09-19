// Package chain 最佳区块哈希查询实现
package chain

import (
	"context"
	"fmt"
)

// getBestBlockHash 获取最佳区块哈希
func (m *Manager) getBestBlockHash(ctx context.Context) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询最佳区块哈希")
	}

	// 通过 repository 获取最高区块的哈希信息
	// GetHighestBlock 返回: (height uint64, blockHash []byte, err error)
	// 我们只需要第二个返回值 blockHash
	_, bestHash, err := m.repo.GetHighestBlock(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("获取最佳区块哈希失败: %v", err)
		}
		return nil, fmt.Errorf("获取最佳区块哈希失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("最佳区块哈希查询完成 - hash: %x", bestHash)
	}

	return bestHash, nil
}
