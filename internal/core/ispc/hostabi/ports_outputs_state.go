// Package hostabi 实现 ISPC HostABI（引擎无关宿主能力接口）
//
// 本文件：状态输出
// 提供状态输出（StateOutput）创建功能。
// StateOutput 用于记录执行结果哈希与 ZK 公开输入，作为证据载体。
package hostabi

import (
	"context"
	"fmt"
)

// ==================== 状态输出 ====================

// AppendStateOutput 追加状态输出
//
// 参数：
//   - ctx: 上下文对象
//   - stateID: 状态标识符
//   - stateVersion: 状态版本号
//   - executionResultHash: 执行结果哈希
//   - publicInputs: ZK 证明公开输入
//   - parentStateHash: 父状态哈希
//
// 返回值：
//   - uint32: 输出索引
//   - error: 创建失败时的错误信息
//
// 说明：
//   - ✅ 架构重构：委托给 TransactionDraftService
func (h *HostRuntimePorts) AppendStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	// 1. 获取草稿ID
	draftID := h.execCtx.GetDraftID()
	if draftID == "" {
		return 0, fmt.Errorf("获取草稿ID失败")
	}

	// 2. 从 DraftService 加载草稿
	draft, err := h.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return 0, fmt.Errorf("加载交易草稿失败: %w", err)
	}

	// 3. ✅ 委托给 DraftService 构建输出
	idx, err := h.draftService.AddStateOutput(ctx, draft, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)
	if err != nil {
		return 0, fmt.Errorf("追加状态输出失败: %w", err)
	}

	// 4. DraftService 已经保存了草稿，无需再次更新

	return idx, nil
}
