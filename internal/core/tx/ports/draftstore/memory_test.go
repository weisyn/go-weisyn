// Package draftstore_test 提供 DraftStore 的单元测试
//
// 🧪 **测试覆盖**：
// - MemoryStore 核心功能测试
// - 并发安全测试
// - TTL 管理测试
// - 边界条件和错误场景测试
package draftstore

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== MemoryStore 核心功能测试 ====================

// TestNewMemoryStore 测试创建 MemoryStore
func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	assert.NotNil(t, store)
}

// TestMemoryStore_Save 测试保存草稿
func TestMemoryStore_Save(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "test-draft-1",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	draftID, err := store.Save(context.Background(), draft)
	assert.NoError(t, err)
	assert.Equal(t, "test-draft-1", draftID)
}

// TestMemoryStore_Get 测试获取草稿
func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "test-draft-2",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	loaded, err := store.Get(context.Background(), "test-draft-2")
	assert.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Equal(t, draft.DraftID, loaded.DraftID)
}

// TestMemoryStore_Get_NotFound 测试获取不存在的草稿
func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get(context.Background(), "non-existent")
	assert.Error(t, err)
}

// TestMemoryStore_Delete 测试删除草稿
func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "test-draft-3",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	err = store.Delete(context.Background(), "test-draft-3")
	assert.NoError(t, err)

	_, err = store.Get(context.Background(), "test-draft-3")
	assert.Error(t, err)
}

// TestMemoryStore_List 测试列出所有草稿
func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()

	// 保存多个草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	drafts, err := store.List(context.Background(), nil, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 3)
}

// TestMemoryStore_SetTTL 测试设置 TTL
func TestMemoryStore_SetTTL(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "test-draft-ttl",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	err = store.SetTTL(context.Background(), "test-draft-ttl", 60)
	assert.NoError(t, err)
}

// TestMemoryStore_ConcurrentAccess 测试并发访问
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()

	// 并发保存
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			draft := &types.DraftTx{
				DraftID: fmt.Sprintf("concurrent-draft-%d", idx),
				Tx: &transaction.Transaction{
					Version: 1,
					Inputs:  []*transaction.TxInput{},
					Outputs: []*transaction.TxOutput{},
				},
				IsSealed: false,
			}
			_, err := store.Save(context.Background(), draft)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	drafts, err := store.List(context.Background(), nil, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 10)
}

// ==================== Save 边界条件测试 ====================

// TestMemoryStore_Save_NilDraft 测试保存 nil draft
func TestMemoryStore_Save_NilDraft(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Save(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// TestMemoryStore_Save_EmptyDraftID 测试保存空 draftID
func TestMemoryStore_Save_EmptyDraftID(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "", // 空 draftID
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err := store.Save(context.Background(), draft)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftID 不能为空")
}

// TestMemoryStore_Save_Overwrite 测试覆盖已存在的草稿
func TestMemoryStore_Save_Overwrite(t *testing.T) {
	store := NewMemoryStore()

	draft1 := &types.DraftTx{
		DraftID: "test-draft-overwrite",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	// 第一次保存
	draftID1, err := store.Save(context.Background(), draft1)
	require.NoError(t, err)
	assert.Equal(t, "test-draft-overwrite", draftID1)

	// 第二次保存（覆盖）
	draft2 := &types.DraftTx{
		DraftID: "test-draft-overwrite",
		Tx: &transaction.Transaction{
			Version: 2, // 版本不同
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: true, // 状态不同
	}

	draftID2, err := store.Save(context.Background(), draft2)
	assert.NoError(t, err)
	assert.Equal(t, "test-draft-overwrite", draftID2)

	// 验证已覆盖
	loaded, err := store.Get(context.Background(), "test-draft-overwrite")
	assert.NoError(t, err)
	assert.Equal(t, uint32(2), loaded.Tx.Version)
	assert.True(t, loaded.IsSealed)
}

// ==================== Get 边界条件测试 ====================

// TestMemoryStore_Get_EmptyDraftID 测试获取空 draftID
func TestMemoryStore_Get_EmptyDraftID(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get(context.Background(), "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftID 不能为空")
}

// ==================== Delete 边界条件测试 ====================

// TestMemoryStore_Delete_EmptyDraftID 测试删除空 draftID
func TestMemoryStore_Delete_EmptyDraftID(t *testing.T) {
	store := NewMemoryStore()

	err := store.Delete(context.Background(), "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftID 不能为空")
}

// TestMemoryStore_Delete_NotFound 测试删除不存在的草稿（幂等性）
func TestMemoryStore_Delete_NotFound(t *testing.T) {
	store := NewMemoryStore()

	// 删除不存在的草稿应该不报错（幂等操作）
	err := store.Delete(context.Background(), "non-existent-draft")

	assert.NoError(t, err)
}

// TestMemoryStore_Delete_MultipleTimes 测试多次删除同一草稿（幂等性）
func TestMemoryStore_Delete_MultipleTimes(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "test-draft-multi-delete",
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 第一次删除
	err = store.Delete(context.Background(), "test-draft-multi-delete")
	assert.NoError(t, err)

	// 第二次删除（应该不报错）
	err = store.Delete(context.Background(), "test-draft-multi-delete")
	assert.NoError(t, err)

	// 第三次删除（应该不报错）
	err = store.Delete(context.Background(), "test-draft-multi-delete")
	assert.NoError(t, err)
}

// ==================== List 详细测试 ====================

// TestMemoryStore_List_EmptyStore 测试空存储列表
func TestMemoryStore_List_EmptyStore(t *testing.T) {
	store := NewMemoryStore()

	drafts, err := store.List(context.Background(), nil, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, drafts, 0)
}

// TestMemoryStore_List_WithOwnerFilter 测试按 owner 过滤
func TestMemoryStore_List_WithOwnerFilter(t *testing.T) {
	store := NewMemoryStore()

	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	// 创建属于 owner1 的草稿
	draft1 := &types.DraftTx{
		DraftID: "draft-owner1-1",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner1, "1000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}
	_, err := store.Save(context.Background(), draft1)
	require.NoError(t, err)

	draft2 := &types.DraftTx{
		DraftID: "draft-owner1-2",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner1, "2000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}
	_, err = store.Save(context.Background(), draft2)
	require.NoError(t, err)

	// 创建属于 owner2 的草稿
	draft3 := &types.DraftTx{
		DraftID: "draft-owner2-1",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner2, "3000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}
	_, err = store.Save(context.Background(), draft3)
	require.NoError(t, err)

	// 列出 owner1 的草稿
	drafts, err := store.List(context.Background(), owner1, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 2)

	// 验证所有草稿都属于 owner1
	for _, draft := range drafts {
		found := false
		for _, output := range draft.Tx.Outputs {
			if bytes.Equal(output.Owner, owner1) {
				found = true
				break
			}
		}
		assert.True(t, found, "草稿应该属于 owner1")
	}

	// 列出 owner2 的草稿
	drafts, err = store.List(context.Background(), owner2, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 1)
	assert.Equal(t, "draft-owner2-1", drafts[0].DraftID)
}

// TestMemoryStore_List_WithLimit 测试 limit 限制
func TestMemoryStore_List_WithLimit(t *testing.T) {
	store := NewMemoryStore()

	// 创建5个草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-limit-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// 限制返回3个
	drafts, err := store.List(context.Background(), nil, 3, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 3)
}

// TestMemoryStore_List_WithOffset 测试 offset 偏移
func TestMemoryStore_List_WithOffset(t *testing.T) {
	store := NewMemoryStore()

	// 创建5个草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-offset-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// offset=2, limit=2
	drafts, err := store.List(context.Background(), nil, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, drafts, 2)
}

// TestMemoryStore_List_OffsetOutOfRange 测试 offset 超出范围
func TestMemoryStore_List_OffsetOutOfRange(t *testing.T) {
	store := NewMemoryStore()

	// 创建3个草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-offset-out-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// offset=10（超出范围）
	drafts, err := store.List(context.Background(), nil, 10, 10)
	assert.NoError(t, err)
	assert.Len(t, drafts, 0)
}

// TestMemoryStore_List_ZeroLimit 测试 limit=0（无限制）
func TestMemoryStore_List_ZeroLimit(t *testing.T) {
	store := NewMemoryStore()

	// 创建10个草稿
	for i := 0; i < 10; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-zero-limit-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// limit=0 应该返回所有
	drafts, err := store.List(context.Background(), nil, 0, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 10)
}

// TestMemoryStore_List_OwnerNoMatch 测试 owner 不匹配
func TestMemoryStore_List_OwnerNoMatch(t *testing.T) {
	store := NewMemoryStore()

	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	// 创建属于 owner1 的草稿
	draft := &types.DraftTx{
		DraftID: "draft-owner1",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner1, "1000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}
	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 使用 owner2 查询（应该返回空）
	drafts, err := store.List(context.Background(), owner2, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 0)
}

// TestMemoryStore_List_OwnerMultipleOutputs 测试多个输出中有一个匹配 owner
func TestMemoryStore_List_OwnerMultipleOutputs(t *testing.T) {
	store := NewMemoryStore()

	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	// 创建有多个输出的草稿，其中一个属于 owner1
	draft := &types.DraftTx{
		DraftID: "draft-multi-output",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner2, "1000", testutil.CreateSingleKeyLock(nil)),
				testutil.CreateNativeCoinOutput(owner1, "2000", testutil.CreateSingleKeyLock(nil)), // 这个匹配
			},
		},
	}
	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 使用 owner1 查询应该能找到
	drafts, err := store.List(context.Background(), owner1, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 1)
	assert.Equal(t, "draft-multi-output", drafts[0].DraftID)
}

// ==================== SetTTL 边界条件测试 ====================

// TestMemoryStore_SetTTL_EmptyDraftID 测试设置空 draftID 的 TTL
func TestMemoryStore_SetTTL_EmptyDraftID(t *testing.T) {
	store := NewMemoryStore()

	err := store.SetTTL(context.Background(), "", 60)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftID 不能为空")
}

// TestMemoryStore_SetTTL_NotFound 测试设置不存在的草稿的 TTL
func TestMemoryStore_SetTTL_NotFound(t *testing.T) {
	store := NewMemoryStore()

	err := store.SetTTL(context.Background(), "non-existent-draft", 60)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// TestMemoryStore_SetTTL_ZeroTTL 测试设置 TTL=0
func TestMemoryStore_SetTTL_ZeroTTL(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "draft-zero-ttl",
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// TTL=0 应该成功（表示永不过期）
	err = store.SetTTL(context.Background(), "draft-zero-ttl", 0)
	assert.NoError(t, err)
}

// TestMemoryStore_SetTTL_UpdateTTL 测试更新已存在的 TTL
func TestMemoryStore_SetTTL_UpdateTTL(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "draft-update-ttl",
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 第一次设置 TTL
	err = store.SetTTL(context.Background(), "draft-update-ttl", 60)
	assert.NoError(t, err)

	// 第二次更新 TTL
	err = store.SetTTL(context.Background(), "draft-update-ttl", 120)
	assert.NoError(t, err)
}

// ==================== 辅助方法测试 ====================

// TestMemoryStore_Count 测试 Count 方法
func TestMemoryStore_Count(t *testing.T) {
	store := NewMemoryStore()
	memStore := store.(*MemoryStore)

	// 初始应该为0
	assert.Equal(t, 0, memStore.Count())

	// 添加3个草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-count-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, memStore.Count())

	// 删除一个
	err := store.Delete(context.Background(), "draft-count-0")
	require.NoError(t, err)

	assert.Equal(t, 2, memStore.Count())
}

// TestMemoryStore_Clear 测试 Clear 方法
func TestMemoryStore_Clear(t *testing.T) {
	store := NewMemoryStore()
	memStore := store.(*MemoryStore)

	// 添加一些草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("draft-clear-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, memStore.Count())

	// 清空
	memStore.Clear()

	assert.Equal(t, 0, memStore.Count())

	// 验证所有草稿都已删除
	drafts, err := store.List(context.Background(), nil, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 0)
}

// ==================== isDraftOwnedBy 辅助函数测试 ====================

// TestIsDraftOwnedBy_NilDraft 测试 nil draft
func TestIsDraftOwnedBy_NilDraft(t *testing.T) {
	owner := testutil.RandomAddress()

	result := isDraftOwnedBy(nil, owner)

	assert.False(t, result)
}

// TestIsDraftOwnedBy_NilTx 测试 nil Tx
func TestIsDraftOwnedBy_NilTx(t *testing.T) {
	owner := testutil.RandomAddress()

	draft := &types.DraftTx{
		DraftID: "test",
		Tx:      nil,
	}

	result := isDraftOwnedBy(draft, owner)

	assert.False(t, result)
}

// TestIsDraftOwnedBy_NoOutputs 测试无输出的草稿
func TestIsDraftOwnedBy_NoOutputs(t *testing.T) {
	owner := testutil.RandomAddress()

	draft := &types.DraftTx{
		DraftID: "test",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{},
		},
	}

	result := isDraftOwnedBy(draft, owner)

	assert.False(t, result)
}

// TestIsDraftOwnedBy_Match 测试匹配的 owner
func TestIsDraftOwnedBy_Match(t *testing.T) {
	owner := testutil.RandomAddress()

	draft := &types.DraftTx{
		DraftID: "test",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}

	result := isDraftOwnedBy(draft, owner)

	assert.True(t, result)
}

// TestIsDraftOwnedBy_NoMatch 测试不匹配的 owner
func TestIsDraftOwnedBy_NoMatch(t *testing.T) {
	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	draft := &types.DraftTx{
		DraftID: "test",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				testutil.CreateNativeCoinOutput(owner1, "1000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}

	result := isDraftOwnedBy(draft, owner2)

	assert.False(t, result)
}

// TestIsDraftOwnedBy_NilOutput 测试 nil output
func TestIsDraftOwnedBy_NilOutput(t *testing.T) {
	owner := testutil.RandomAddress()

	draft := &types.DraftTx{
		DraftID: "test",
		Tx: &transaction.Transaction{
			Version: 1,
			Outputs: []*transaction.TxOutput{
				nil, // nil output
				testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil)),
			},
		},
	}

	result := isDraftOwnedBy(draft, owner)

	assert.True(t, result) // 应该忽略 nil output，匹配第二个
}

// ==================== 并发安全测试 ====================

// TestMemoryStore_ConcurrentReadWrite 测试并发读写
func TestMemoryStore_ConcurrentReadWrite(t *testing.T) {
	store := NewMemoryStore()

	const numGoroutines = 20
	done := make(chan bool, numGoroutines)

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			draft := &types.DraftTx{
				DraftID: fmt.Sprintf("concurrent-rw-%d", idx),
				Tx: &transaction.Transaction{
					Version: 1,
				},
			}
			_, err := store.Save(context.Background(), draft)
			assert.NoError(t, err)

			// 立即读取
			loaded, err := store.Get(context.Background(), fmt.Sprintf("concurrent-rw-%d", idx))
			assert.NoError(t, err)
			assert.NotNil(t, loaded)

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证所有草稿都已保存
	drafts, err := store.List(context.Background(), nil, 100, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, numGoroutines)
}

// TestMemoryStore_ConcurrentDelete 测试并发删除
func TestMemoryStore_ConcurrentDelete(t *testing.T) {
	store := NewMemoryStore()

	// 先创建一些草稿
	const numDrafts = 10
	for i := 0; i < numDrafts; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("concurrent-delete-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// 并发删除
	done := make(chan bool, numDrafts)
	for i := 0; i < numDrafts; i++ {
		go func(idx int) {
			err := store.Delete(context.Background(), fmt.Sprintf("concurrent-delete-%d", idx))
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numDrafts; i++ {
		<-done
	}

	// 验证所有草稿都已删除
	drafts, err := store.List(context.Background(), nil, 100, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 0)
}

// TestMemoryStore_ConcurrentList 测试并发列表查询
func TestMemoryStore_ConcurrentList(t *testing.T) {
	store := NewMemoryStore()

	// 创建一些草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("concurrent-list-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
			},
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// 并发查询
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			drafts, err := store.List(context.Background(), nil, 10, 0)
			assert.NoError(t, err)
			assert.Len(t, drafts, 5)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestMemoryStore_ConcurrentSetTTL 测试并发设置 TTL
func TestMemoryStore_ConcurrentSetTTL(t *testing.T) {
	store := NewMemoryStore()

	draft := &types.DraftTx{
		DraftID: "concurrent-ttl",
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err := store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 并发设置 TTL
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			err := store.SetTTL(context.Background(), "concurrent-ttl", idx*10+10)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
