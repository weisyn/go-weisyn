// Package hostabi 实现 ISPC HostABI（引擎无关宿主能力接口）
//
// 本文件：资产输出与转账（账户抽象设计）
// 提供资产输出创建（原生币/合约代币）和账户转账功能。
// 所有操作记录到 TransactionDraft，不直接修改链上状态，遵循"写走草稿"语义。
package hostabi

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// ==================== 资产输出与转账 ====================

// AppendAssetOutput 追加资产输出
//
// 参数：
//   - ctx: 上下文对象
//   - recipient: 接收者地址（20字节）
//   - amount: 资产数量（单位：最小单位）
//   - tokenID: 代币标识（nil=原生币，非nil=合约代币）
//   - lockingConditions: 锁定条件数组（nil=无锁定，支持时间锁/高度锁/多签/合约锁）
//
// 返回值：
//   - uint32: 输出索引（在 TransactionDraft.Outputs 中的位置）
//   - error: 创建失败时的错误信息
//
// 说明：
//   - ✅ 架构重构：委托给 TransactionDraftService，而不是直接构建 TransactionDraft
//   - 创建价值载体输出（AssetOutput），记录到交易草稿
//   - 原生币（tokenID=nil）：创建 NativeCoinAsset
//   - 合约代币（tokenID!=nil）：创建 ContractTokenAsset（自动判断 FT/NFT）
func (h *HostRuntimePorts) AppendAssetOutput(ctx context.Context, recipient []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
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
	idx, err := h.draftService.AddAssetOutput(ctx, draft, recipient, fmt.Sprintf("%d", amount), tokenID, lockingConditions)
	if err != nil {
		return 0, fmt.Errorf("追加资产输出失败: %w", err)
	}

	// 4. 如果是合约代币，需要补充 contractAddress（DraftService 无法获取）
	if tokenID != nil {
		contractAddr := h.execCtx.GetContractAddress()
		if contractAddr == nil {
			return 0, fmt.Errorf("无法获取合约地址（创建合约代币输出需要）")
		}
		// 设置 contractAddress 到刚创建的输出
		if idx < uint32(len(draft.Tx.Outputs)) {
			output := draft.Tx.Outputs[idx]
			if asset := output.GetAsset(); asset != nil {
				if contractToken := asset.GetContractToken(); contractToken != nil {
					contractToken.ContractAddress = contractAddr
				}
			}
		}

		// 保存更新后的草稿
		if err := h.draftService.SaveDraft(ctx, draft); err != nil {
			return 0, fmt.Errorf("保存交易草稿失败: %w", err)
		}
	}

	h.syncDraftWithExecutionContext(draft)

	return idx, nil
}

// Transfer 执行资产转账（基础版）
//
// 参数：
//   - ctx: 上下文对象
//   - from: 发送方地址（20字节）
//   - to: 接收方地址（20字节）
//   - amount: 转账金额（单位：最小单位）
//   - tokenID: 代币标识（nil=原生币，非nil=指定代币）
//
// 返回值：
//   - error: 转账失败时的错误信息（余额不足、权限不足等）
//
// 说明：
//   - 执行账户间的资产转移（底层自动处理资产选择、找零等技术细节）
//   - 开发者只需关心业务逻辑：谁给谁转多少钱
//   - 简单直观，符合用户认知
//   - 用于日常转账、支付等场景
//   - 简化版实现：直接创建接收者输出（v1.1 将完善输入选择和找零）
//   - 所有操作记录到交易草稿，不直接修改链上状态
func (h *HostRuntimePorts) Transfer(ctx context.Context, from []byte, to []byte, amount uint64, tokenID []byte) error {
	_, err := h.AppendAssetOutput(ctx, to, amount, tokenID, nil)
	if err != nil {
		return fmt.Errorf("创建转账输出失败: %w", err)
	}
	if h.logger != nil {
		h.logger.Debugf("记录转账意图: from=%x, to=%x, amount=%d", from, to, amount)
	}
	return nil
}

// TransferEx 执行资产转账（扩展版）
//
// 参数：
//   - ctx: 上下文对象
//   - from: 发送方地址（20字节）
//   - to: 接收方地址（20字节）
//   - amount: 转账金额（单位：最小单位）
//   - tokenID: 代币标识（nil=原生币，非nil=指定代币）
//   - lockingConditions: 锁定条件数组（支持时间锁、高度锁、多签、合约锁等）
//
// 返回值：
//   - error: 转账失败时的错误信息
//
// 说明：
//   - 支持高级转账场景（定时释放、多签授权、合约控制等）
//   - 通过 lockingConditions 实现复杂业务逻辑
//   - 锁定条件应用于接收方，用于实现企业级资金管理
//   - 示例：
//   - 时间锁：锁定到 Unix 时间戳 1700000000
//   - 高度锁：锁定到区块高度 100000
//   - 多签：需要 2-of-3 签名才能解锁
//   - 合约锁：需要特定合约调用才能解锁
//   - 所有操作记录到交易草稿，不直接修改链上状态
func (h *HostRuntimePorts) TransferEx(ctx context.Context, from []byte, to []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) error {
	_, err := h.AppendAssetOutput(ctx, to, amount, tokenID, lockingConditions)
	if err != nil {
		return fmt.Errorf("创建转账输出失败: %w", err)
	}
	if h.logger != nil {
		h.logger.Debugf("记录扩展转账意图: from=%x, to=%x, amount=%d, lockingConditions=%d", from, to, amount, len(lockingConditions))
	}
	return nil
}

// syncDraftWithExecutionContext 将 DraftService 中的草稿同步回 ExecutionContext（用于后续协调器读取）
func (h *HostRuntimePorts) syncDraftWithExecutionContext(draft *types.DraftTx) {
	if draft == nil || h.execCtx == nil {
		return
	}

	ctxDraft, err := h.execCtx.GetTransactionDraft()
	if err != nil || ctxDraft == nil {
		if h.logger != nil && err != nil {
			h.logger.Debugf("syncDraftWithExecutionContext: 获取ExecutionContext草稿失败: %v", err)
		}
		return
	}

	if draft.Tx != nil {
		cloned, ok := proto.Clone(draft.Tx).(*pb.Transaction)
		if !ok {
			cloned = draft.Tx
		}
		ctxDraft.Tx = cloned
		ctxDraft.Outputs = cloned.GetOutputs()
	} else {
		ctxDraft.Tx = nil
		ctxDraft.Outputs = nil
	}

	if err := h.execCtx.UpdateTransactionDraft(ctxDraft); err != nil && h.logger != nil {
		h.logger.Debugf("syncDraftWithExecutionContext: 更新ExecutionContext草稿失败: %v", err)
	}
}
