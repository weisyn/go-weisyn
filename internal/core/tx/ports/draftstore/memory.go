package draftstore

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// MemoryStore 内存版本的 DraftStore 实现
//
// 📋 **职责**：
//   - 在内存中存储和检索交易草稿
//   - 提供并发安全的读写操作
//   - 适用于短期草稿存储（无持久化）
//
// 🔒 **并发安全**：
//   - 使用 sync.RWMutex 保护共享状态
//   - 支持多个 goroutine 并发访问
//
// 📚 **使用场景**：
//   - ISPC 场景：合约执行中临时存储草稿
//   - CLI 场景：单机模式下的交互式构建
//   - 测试场景：快速的单元/集成测试
//
// ⚠️ **限制**：
//   - 进程重启后数据丢失
//   - 不支持跨进程/跨节点共享
//   - 适合短期/临时存储
type MemoryStore struct {
	// 草稿存储（key = draftID, value = draft）
	drafts map[string]*types.DraftTx
	// TTL 记录（key = draftID, value = TTL 秒数）
	ttls map[string]int
	mu   sync.RWMutex
}

// 确保实现接口
var _ tx.DraftStore = (*MemoryStore)(nil)

// NewMemoryStore 创建内存版 DraftStore 实例
//
// 返回值:
//   - tx.DraftStore: 服务实例
func NewMemoryStore() tx.DraftStore {
	return &MemoryStore{
		drafts: make(map[string]*types.DraftTx),
		ttls:   make(map[string]int),
	}
}

// Save 保存交易草稿
func (s *MemoryStore) Save(ctx context.Context, draft *types.DraftTx) (string, error) {
	if draft == nil {
		return "", fmt.Errorf("draft 不能为 nil")
	}

	draftID := draft.DraftID
	if draftID == "" {
		return "", fmt.Errorf("draftID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 保存草稿（如果已存在则覆盖）
	s.drafts[draftID] = draft

	return draftID, nil
}

// Get 获取交易草稿
func (s *MemoryStore) Get(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if draftID == "" {
		return nil, fmt.Errorf("draftID 不能为空")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	draft, exists := s.drafts[draftID]
	if !exists {
		return nil, fmt.Errorf("draft 不存在: %s", draftID)
	}

	return draft, nil
}

// Delete 删除交易草稿
func (s *MemoryStore) Delete(ctx context.Context, draftID string) error {
	if draftID == "" {
		return fmt.Errorf("draftID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除草稿（即使不存在也不报错）
	delete(s.drafts, draftID)

	return nil
}

// List 列出所有草稿
func (s *MemoryStore) List(ctx context.Context, ownerAddress []byte, limit, offset int) ([]*types.DraftTx, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.DraftTx

	// 遍历所有草稿
	for _, draft := range s.drafts {
		// 如果没有指定 owner，包含所有草稿
		if len(ownerAddress) == 0 {
			result = append(result, draft)
			continue
		}

		// 如果指定了 ownerAddress，检查草稿是否属于该 owner
		// 规则：只要草稿的任意一个 Output 的 owner 匹配，就认为该草稿属于该 owner
		if isDraftOwnedBy(draft, ownerAddress) {
			result = append(result, draft)
		}
	}

	// 应用 offset 和 limit
	if offset >= len(result) {
		return []*types.DraftTx{}, nil
	}

	start := offset
	end := len(result)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return result[start:end], nil
}

// isDraftOwnedBy 检查草稿是否属于指定的 owner
//
// 检查规则：
//   - 遍历草稿交易的所有 Outputs
//   - 如果任意一个 Output 的 owner 字段与 ownerAddress 匹配，返回 true
//   - 如果所有 Outputs 都不匹配，返回 false
//
// 参数：
//   - draft: 草稿交易
//   - ownerAddress: 要检查的 owner 地址
//
// 返回：
//   - bool: true 表示草稿属于该 owner
func isDraftOwnedBy(draft *types.DraftTx, ownerAddress []byte) bool {
	// 防御性检查
	if draft == nil || draft.Tx == nil {
		return false
	}

	// 遍历所有 Outputs
	for _, output := range draft.Tx.Outputs {
		if output == nil {
			continue
		}

		// 比较 owner 字段（字节数组比较）
		if bytes.Equal(output.Owner, ownerAddress) {
			return true
		}
	}

	return false
}

// SetTTL 设置草稿过期时间
//
// 📝 **内存实现说明**：
//   - 此实现仅记录 TTL，不自动删除过期草稿
//   - 实际自动清理需要后台 goroutine（可选）
//   - 适用于简单场景，生产环境建议使用 Redis
func (s *MemoryStore) SetTTL(ctx context.Context, draftID string, ttlSeconds int) error {
	if draftID == "" {
		return fmt.Errorf("draftID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查草稿是否存在
	if _, exists := s.drafts[draftID]; !exists {
		return fmt.Errorf("draft 不存在: %s", draftID)
	}

	// 记录 TTL
	s.ttls[draftID] = ttlSeconds

	// 注意：内存版不自动清理，仅记录 TTL
	// 如需自动清理，需要启动后台 goroutine 定期扫描

	return nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 辅助方法（非接口要求，但对调试有帮助）
// ════════════════════════════════════════════════════════════════════════════════════════════════

// Count 返回当前存储的草稿数量（用于监控/调试）
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.drafts)
}

// Clear 清空所有草稿（用于测试）
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.drafts = make(map[string]*types.DraftTx)
}
