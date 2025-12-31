// Package builder 提供区块构建服务的实现
// nolint:U1000 // 允许未使用的函数以备将来使用
package builder

import (
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// cacheCandidate 缓存候选区块
//
// 🎯 **缓存策略**：
// - LRU淘汰：缓存满时自动淘汰最近最少使用的候选区块
// - 哈希索引：使用区块哈希作为键
// - 并发安全：LRU缓存内部保证并发安全
//
// 参数：
//   - blockHash: 区块哈希
//   - block: 候选区块
//
// 返回：
//   - error: 缓存错误
func (s *Service) cacheCandidate(blockHash []byte, block *core.Block) error {
	// 生成缓存键
	hashKey := fmt.Sprintf("%x", blockHash)

	// 使用LRU缓存存储
	s.cache.Put(hashKey, block)

	// 更新指标
	s.metricsMu.Lock()
	s.metrics.CacheSize = s.cache.Size()
	s.metricsMu.Unlock()

	if s.logger != nil {
		if len(blockHash) >= 8 {
		s.logger.Debugf("候选区块已缓存: %x, 当前缓存大小: %d", blockHash[:8], s.cache.Size())
		} else {
			s.logger.Debugf("候选区块已缓存: %x, 当前缓存大小: %d", blockHash, s.cache.Size())
		}
	}

	return nil
}

// removeCachedCandidate 从缓存中移除候选区块（内部实现）
//
// 参数：
//   - blockHash: 区块哈希
//
// 返回：
//   - error: 移除错误
func (s *Service) removeCachedCandidate(blockHash []byte) error {
	hashKey := fmt.Sprintf("%x", blockHash)

	// 检查是否存在
	if _, exists := s.cache.Get(hashKey); !exists {
		if len(blockHash) >= 8 {
		return fmt.Errorf("候选区块不在缓存中: %x", blockHash[:8])
		}
		return fmt.Errorf("候选区块不在缓存中: %x", blockHash)
	}

	// 从LRU缓存删除
	s.cache.Delete(hashKey)

	// 更新指标
	s.metricsMu.Lock()
	s.metrics.CacheSize = s.cache.Size()
	s.metricsMu.Unlock()

	if s.logger != nil {
		if len(blockHash) >= 8 {
		s.logger.Debugf("候选区块已从缓存移除: %x", blockHash[:8])
		} else {
			s.logger.Debugf("候选区块已从缓存移除: %x", blockHash)
		}
	}

	return nil
}
