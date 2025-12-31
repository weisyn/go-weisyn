// Package hostabi 实现 ISPC HostABI（引擎无关宿主能力接口）
//
// 本文件：代币生命周期管理
// 提供代币销毁（Burn）和授权（Approve）意图记录功能。
// 用于 FT/NFT/SFT 代币的生命周期管理，支持通缩机制和授权转账。
package hostabi

import (
	"context"
	"fmt"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 代币生命周期 ====================

// AppendContractTokenOutput 追加合约代币输出
//
// 参数：
//   - ctx: 上下文对象
//   - recipient: 接收者地址（20字节）
//   - amount: 代币数量
//   - tokenClassID: 代币类别ID（用于 FT/SFT，nil 表示使用 tokenUniqueID）
//   - tokenUniqueID: 代币唯一ID（用于 NFT，nil 表示使用 tokenClassID）
//   - lockingConditions: 锁定条件数组
//
// 返回值：
//   - uint32: 输出索引
//   - error: 创建失败时的错误信息
//
// 说明：
//   - 用于合约内部发行代币（Mint 操作）
//   - tokenUniqueID 优先：如果非 nil，创建 NFT 输出
//   - tokenClassID 其次：如果非 nil，创建 FT/SFT 输出
//   - 两者不能同时为 nil
//   - 自动关联当前合约地址作为代币发行方
func (h *HostRuntimePorts) AppendContractTokenOutput(ctx context.Context, recipient []byte, amount uint64, tokenClassID []byte, tokenUniqueID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	draft, err := h.execCtx.GetTransactionDraft()
	if err != nil {
		return 0, fmt.Errorf("获取交易草稿失败: %w", err)
	}
	contractAddr := h.execCtx.GetContractAddress()
	if contractAddr == nil {
		return 0, fmt.Errorf("无法获取合约地址（创建合约代币输出需要）")
	}
	contractToken := &pb.ContractTokenAsset{
		ContractAddress: contractAddr,
		Amount:          fmt.Sprintf("%d", amount),
		DisplayCache:    nil,
	}
	if tokenUniqueID != nil {
		contractToken.TokenIdentifier = &pb.ContractTokenAsset_NftUniqueId{NftUniqueId: tokenUniqueID}
	} else if tokenClassID != nil {
		contractToken.TokenIdentifier = &pb.ContractTokenAsset_FungibleClassId{FungibleClassId: tokenClassID}
	} else {
		return 0, fmt.Errorf("tokenClassID 和 tokenUniqueID 不能同时为 nil")
	}
	output := &pb.TxOutput{
		Owner:             recipient,
		LockingConditions: lockingConditions,
		OutputContent:     &pb.TxOutput_Asset{Asset: &pb.AssetOutput{AssetContent: &pb.AssetOutput_ContractToken{ContractToken: contractToken}}},
	}
	draft.Outputs = append(draft.Outputs, output)
	idx := uint32(len(draft.Outputs) - 1)
	if err := h.execCtx.UpdateTransactionDraft(draft); err != nil {
		return 0, fmt.Errorf("更新交易草稿失败: %w", err)
	}
	if h.logger != nil {
		h.logger.Debugf("✅ 追加合约代币输出: index=%d, recipient=%x, amount=%d", idx, recipient, amount)
	}
	return idx, nil
}

// AppendBurnIntent 追加代币销毁意图
//
// 参数：
//   - ctx: 上下文对象
//   - tokenID: 代币标识（可以是类别ID或唯一ID）
//   - amount: 销毁数量
//   - burnProof: 销毁证明（可选，如签名/ZK证明）
//
// 返回值：
//   - error: 记录失败时的错误信息
//
// 说明：
//   - 记录销毁意图到 TransactionDraft.BurnIntents
//   - 用于通缩机制、回购销毁等场景
//   - burnProof 可用于验证销毁授权（如多签验证、ZK 证明等）
//   - 最终由 TX 模块验证销毁合法性（持有量、授权等）
//   - 销毁操作不可逆，需谨慎使用
func (h *HostRuntimePorts) AppendBurnIntent(ctx context.Context, tokenID []byte, amount uint64, burnProof []byte) error {
	draft, err := h.execCtx.GetTransactionDraft()
	if err != nil {
		return fmt.Errorf("获取交易草稿失败: %w", err)
	}
	burnIntent := &ispcInterfaces.TokenBurnIntent{
		TokenID:   tokenID,
		Amount:    amount,
		BurnProof: burnProof,
	}
	draft.BurnIntents = append(draft.BurnIntents, burnIntent)
	if err := h.execCtx.UpdateTransactionDraft(draft); err != nil {
		return fmt.Errorf("更新交易草稿失败: %w", err)
	}
	if h.logger != nil {
		h.logger.Debugf("✅ 追加销毁意图: tokenID=%x, amount=%d", tokenID, amount)
	}
	return nil
}

// AppendApproveIntent 追加代币授权意图
//
// 参数：
//   - ctx: 上下文对象
//   - tokenID: 代币标识
//   - spender: 被授权者地址（20字节）
//   - amount: 授权额度
//   - expiry: 过期时间（Unix秒，0=永久授权）
//
// 返回值：
//   - error: 记录失败时的错误信息
//
// 说明：
//   - 记录授权意图到 TransactionDraft.ApproveIntents
//   - 用于授权第三方转账（如 DEX、托管等）
//   - expiry=0 表示永久授权，非0表示到期时间（Unix 秒）
//   - 最终由 TX 模块验证授权合法性（持有量、已授权额度等）
//   - 类似 ERC-20 的 approve 机制，但支持过期时间
func (h *HostRuntimePorts) AppendApproveIntent(ctx context.Context, tokenID []byte, spender []byte, amount uint64, expiry uint64) error {
	draft, err := h.execCtx.GetTransactionDraft()
	if err != nil {
		return fmt.Errorf("获取交易草稿失败: %w", err)
	}
	approveIntent := &ispcInterfaces.TokenApproveIntent{
		TokenID: tokenID,
		Spender: spender,
		Amount:  amount,
		Expiry:  expiry,
	}
	draft.ApproveIntents = append(draft.ApproveIntents, approveIntent)
	if err := h.execCtx.UpdateTransactionDraft(draft); err != nil {
		return fmt.Errorf("更新交易草稿失败: %w", err)
	}
	if h.logger != nil {
		h.logger.Debugf("✅ 追加授权意图: tokenID=%x, spender=%x, amount=%d, expiry=%d", tokenID, spender, amount, expiry)
	}
	return nil
}
