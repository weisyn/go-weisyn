// Package account 待确认余额管理实现
//
// ⏳ **待确认余额跟踪实现 (Pending Balance Tracking)**
//
// 本文件实现待确认余额的状态跟踪功能，包括：
// - 内存池交易状态查询和分析
// - 交易确认进度跟踪
// - 预计确认时间评估
//
// 🎯 **核心功能**
// - 交易跟踪：实时跟踪内存池中的相关交易
// - 确认进度：计算交易确认数和剩余确认要求
// - 时间预估：基于网络状况评估预计确认时间
package account

import (
	"bytes"
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
//                              待确认余额查询
// ============================================================================

// getPendingBalances 获取待确认余额详情
//
// 🎯 **待确认余额状态跟踪核心实现**
//
// 实现流程：
// 1. 查询内存池中的相关交易
// 2. 筛选影响该地址和代币的交易
// 3. 分析每笔交易的确认进度
// 4. 评估预计确认时间
// 5. 构建待确认余额条目列表
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//	tokenID: 代币标识符（nil表示平台主币）
//
// 返回：
//
//	[]*types.PendingBalanceEntry: 待确认余额条目列表
//	error: 查询错误
func (m *Manager) getPendingBalances(ctx context.Context, address []byte, tokenID []byte) ([]*types.PendingBalanceEntry, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询待确认余额详情 - address: %x, tokenID: %x", address, tokenID)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 1. 获取所有pending交易
	txs, err := m.txPool.GetAllPendingTransactions()
	if err != nil {
		return nil, fmt.Errorf("获取待处理交易失败: %w", err)
	}

	addrObj := &transaction.Address{RawHash: address}
	var entries []*types.PendingBalanceEntry

	for _, tx := range txs {
		if tx == nil {
			continue
		}

		// 2. 计算该交易对目标地址与代币的净变动
		delta, fee, err := m.computePendingDeltaForTx(ctx, tx, address, tokenID)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("计算交易净变动失败，忽略此交易: %v", err)
			}
			continue
		}
		if delta == 0 {
			continue
		}

		changeType := "receive"
		if delta < 0 {
			changeType = "send"
		}
		submittedAt := time.Unix(int64(tx.GetCreationTimestamp()), 0)

		// 计算交易哈希作为TxID
		var txID []byte
		if m.txHashService != nil {
			hashReq := &transaction.ComputeHashRequest{
				Transaction:      tx,
				IncludeDebugInfo: false,
			}
			hashResp, err := m.txHashService.ComputeHash(ctx, hashReq)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("计算交易哈希失败，使用空TxID: %v", err)
				}
			} else if hashResp.IsValid {
				txID = hashResp.Hash
			}
		}

		entry := &types.PendingBalanceEntry{
			TxID:          txID,
			Address:       addrObj,
			TokenID:       tokenID,
			Amount:        delta,
			ChangeType:    changeType,
			Status:        "pending",
			SubmittedAt:   submittedAt,
			Confirmations: 0,
			RequiredConfs: 1,
			Fee:           fee,
			ExecutionFeeUsed:       0,
			ExecutionFeePrice:      0,
		}
		entries = append(entries, entry)
	}

	if m.logger != nil {
		m.logger.Debugf("待确认余额查询完成 - address: %x, tokenID: %x, entryCount: %d",
			address, tokenID, len(entries))
	}

	return entries, nil
}

// ============================================================================
//                              真实实现的辅助方法
// ============================================================================

// computePendingDeltaForTx 计算单笔pending交易对指定地址与代币的净余额变动
// delta = outputs_to_address - inputs_from_address
func (m *Manager) computePendingDeltaForTx(ctx context.Context, tx *transaction.Transaction, address []byte, tokenID []byte) (int64, uint64, error) {
	var outputsTo uint64 = 0
	var inputsFrom uint64 = 0

	// 输出：发往目标地址的金额
	for _, out := range tx.GetOutputs() {
		if out == nil {
			continue
		}
		if !bytes.Equal(out.GetOwner(), address) {
			continue
		}
		amount, matched, err := extractAmountFromTxOutput(out, tokenID)
		if err != nil {
			return 0, 0, fmt.Errorf("解析输出金额失败: %w", err)
		}
		if matched {
			outputsTo += amount
		}
	}

	// 输入：由目标地址拥有的UTXO被花费的金额
	for _, in := range tx.GetInputs() {
		if in == nil || in.GetPreviousOutput() == nil {
			continue
		}
		utxoObj, err := m.utxoManager.GetUTXO(ctx, in.GetPreviousOutput())
		if err != nil || utxoObj == nil {
			continue
		}
		if !bytes.Equal(utxoObj.GetOwnerAddress(), address) {
			continue
		}

		if tokenID == nil {
			amount, err := m.extractNativeCoinAmount(utxoObj)
			if err != nil {
				return 0, 0, fmt.Errorf("解析输入原生币金额失败: %w", err)
			}
			inputsFrom += amount
		} else {
			matchedTokenID, amount, err := m.extractTokenAmount(utxoObj, tokenID)
			if err != nil {
				return 0, 0, fmt.Errorf("解析输入代币金额失败: %w", err)
			}
			if matchedTokenID != nil && bytes.Equal(matchedTokenID, tokenID) {
				inputsFrom += amount
			}
		}
	}

	// 简单费用估算（如存在）
	var fee uint64 = 0
	if fm := tx.GetFeeMechanism(); fm != nil {
		switch v := fm.(type) {
		case *transaction.Transaction_MinimumFee:
			if v.MinimumFee != nil && v.MinimumFee.MinimumAmount != "" {
				// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
				if parsed, err := utils.ParseAmountSafely(v.MinimumFee.MinimumAmount); err == nil {
					fee = parsed
				}
			}
		case *transaction.Transaction_ContractFee:
			if v.ContractFee != nil && v.ContractFee.BaseFee != "" {
				// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
				if parsed, err := utils.ParseAmountSafely(v.ContractFee.BaseFee); err == nil {
					fee = parsed
				}
			}
		case *transaction.Transaction_PriorityFee:
			if v.PriorityFee != nil && v.PriorityFee.BaseFee != "" {
				// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
				if parsed, err := utils.ParseAmountSafely(v.PriorityFee.BaseFee); err == nil {
					fee = parsed
				}
			}
		}
	}

	// 计算净变动
	var delta int64
	if outputsTo >= inputsFrom {
		delta = int64(outputsTo - inputsFrom)
	} else {
		delta = -int64(inputsFrom - outputsTo)
	}
	return delta, fee, nil
}

// extractAmountFromTxOutput 从交易输出中提取与目标代币匹配的金额
// 返回 (amount, matched, error)
func extractAmountFromTxOutput(out *transaction.TxOutput, tokenID []byte) (uint64, bool, error) {
	assetOut, ok := out.GetOutputContent().(*transaction.TxOutput_Asset)
	if !ok || assetOut.Asset == nil {
		return 0, false, nil
	}
	if tokenID == nil {
		native, ok := assetOut.Asset.GetAssetContent().(*transaction.AssetOutput_NativeCoin)
		if !ok || native.NativeCoin == nil {
			return 0, false, nil
		}
		// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
		amount, err := utils.ParseAmountSafely(native.NativeCoin.Amount)
		if err != nil {
			return 0, false, fmt.Errorf("解析原生币金额失败: %w", err)
		}
		return amount, true, nil
	}
	contract, ok := assetOut.Asset.GetAssetContent().(*transaction.AssetOutput_ContractToken)
	if !ok || contract.ContractToken == nil {
		return 0, false, nil
	}
	var outTokenID []byte
	switch id := contract.ContractToken.GetTokenIdentifier().(type) {
	case *transaction.ContractTokenAsset_FungibleClassId:
		outTokenID = id.FungibleClassId
	case *transaction.ContractTokenAsset_NftUniqueId:
		outTokenID = id.NftUniqueId
	case *transaction.ContractTokenAsset_SemiFungibleId:
		if id.SemiFungibleId != nil {
			outTokenID = id.SemiFungibleId.BatchId
		}
	}
	if !bytes.Equal(outTokenID, tokenID) {
		return 0, false, nil
	}
	// 🔥 修正：解析存储的wei整数字符串（避免二次放大）
	amount, err := utils.ParseAmountSafely(contract.ContractToken.Amount)
	if err != nil {
		return 0, false, fmt.Errorf("解析合约代币金额失败: %w", err)
	}
	return amount, true, nil
}
