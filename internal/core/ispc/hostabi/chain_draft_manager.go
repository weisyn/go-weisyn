// Package hostabi 提供 Host ABI 实现
//
// chain_draft_manager.go: 链上 Draft 管理器实现
package hostabi

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// chainDraftManagerImpl 链上 Draft 管理器实现
//
// 🎯 **职责**:
//   - 管理链上 Draft 的创建、查询、清理
//   - 绑定 Draft 到执行上下文（生命周期一致）
//   - 内存存储，执行结束自动清理
//
// ⚠️ **并发安全**:
//   - 使用 sync.RWMutex 保护 drafts 映射
//   - 使用 atomic.Int32 生成唯一 draftHandle
type chainDraftManagerImpl struct {
	drafts       map[int32]*draftEntry
	mu           sync.RWMutex
	nextHandle   atomic.Int32
	draftService tx.TransactionDraftService
}

// draftEntry Draft 条目（包含元数据）
type draftEntry struct {
	draft          *types.DraftTx // Draft 实例（从 DraftService 创建）
	blockHeight    uint64         // 固定区块高度
	blockTimestamp uint64         // 固定区块时间戳
	createdAt      time.Time      // 创建时间
}

// newChainDraftManager 创建链上 Draft 管理器
func newChainDraftManager(draftService tx.TransactionDraftService) *chainDraftManagerImpl {
	return &chainDraftManagerImpl{
		drafts:       make(map[int32]*draftEntry),
		draftService: draftService,
	}
}

// CreateDraft 创建链上 Draft
//
// 🔄 流程：
//  1. 生成唯一 draftHandle（从 1 开始递增）
//  2. 调用 DraftService.CreateDraft() 创建 Draft
//  3. 存储到 map 中（绑定 blockHeight/blockTimestamp）
//
// 参数：
//   - ctx: 上下文
//   - blockHeight: 固定区块高度
//   - blockTimestamp: 固定区块时间戳
//
// 返回：
//   - draftHandle: Draft 句柄（>0）
//   - error: 创建失败
func (m *chainDraftManagerImpl) CreateDraft(
	ctx context.Context,
	blockHeight uint64,
	blockTimestamp uint64,
) (int32, error) {
	// 1. 生成唯一 draftHandle（从 1 开始）
	handle := m.nextHandle.Add(1)

	// 2. 调用 DraftService 创建 Draft
	draft, err := m.draftService.CreateDraft(ctx)
	if err != nil {
		return 0, fmt.Errorf("创建 Draft 失败: %w", err)
	}

	// 3. 存储到 map
	m.mu.Lock()
	defer m.mu.Unlock()

	m.drafts[handle] = &draftEntry{
		draft:          draft,
		blockHeight:    blockHeight,
		blockTimestamp: blockTimestamp,
		createdAt:      time.Now(),
	}

	return handle, nil
}

// GetDraft 获取 Draft
//
// 参数：
//   - ctx: 上下文
//   - draftHandle: Draft 句柄
//
// 返回：
//   - DraftTx: Draft 实例
//   - error: Draft 不存在
func (m *chainDraftManagerImpl) GetDraft(
	ctx context.Context,
	draftHandle int32,
) (*types.DraftTx, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.drafts[draftHandle]
	if !ok {
		return nil, fmt.Errorf("draft 不存在: handle=%d", draftHandle)
	}

	return entry.draft, nil
}

// RemoveDraft 清理 Draft
//
// 参数：
//   - ctx: 上下文
//   - draftHandle: Draft 句柄
//
// 返回：
//   - error: Draft 不存在
func (m *chainDraftManagerImpl) RemoveDraft(
	ctx context.Context,
	draftHandle int32,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.drafts[draftHandle]; !ok {
		return fmt.Errorf("draft 不存在: handle=%d", draftHandle)
	}

	delete(m.drafts, draftHandle)
	return nil
}

// CleanupAll 清理所有 Draft
//
// 🎯 **用途**：执行结束时调用，清理所有 Draft
//
// 参数：
//   - ctx: 上下文
//
// 返回：
//   - error: 清理失败
func (m *chainDraftManagerImpl) CleanupAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空 map
	m.drafts = make(map[int32]*draftEntry)

	return nil
}
