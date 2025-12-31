// Package hostabi 实现 ISPC HostABI（引擎无关宿主能力接口）
//
// 本文件：资源输出创建
// 提供资源输出（ResourceOutput）创建功能，用于管理 WASM 合约、ONNX 模型、文档等内容载体。
// 资源通过内容哈希（Content-Addressable Storage）唯一标识，支持版本管理和权限控制。
package hostabi

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 资源输出 ====================

// AppendResourceOutput 追加资源输出
//
// 参数：
//   - ctx: 上下文对象
//   - contentHash: 资源内容哈希（32字节 SHA-256）
//   - category: 资源类别（wasm/onnx/document等）
//   - owner: 资源所有者地址（20字节）
//   - lockingConditions: 锁定条件数组
//   - metadata: 资源元数据
//
// 返回值：
//   - uint32: 输出索引
//   - error: 创建失败时的错误信息
//
// 说明：
//   - ✅ 架构重构：委托给 TransactionDraftService
func (h *HostRuntimePorts) AppendResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
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
	idx, err := h.draftService.AddResourceOutput(ctx, draft, contentHash, category, owner, lockingConditions, metadata)
	if err != nil {
		return 0, fmt.Errorf("追加资源输出失败: %w", err)
	}

	// 4. DraftService 已经保存了草稿，无需再次更新

	return idx, nil
}
