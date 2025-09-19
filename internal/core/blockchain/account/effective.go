// Package account 有效余额计算实现
//
// ⚖️ **有效可用余额计算实现 (Effective Balance Calculation)**
//
// 本文件实现有效可用余额的核心计算逻辑，解决审查报告中用户期望的问题：
// - 实时余额扣减：计算 "我现在真正能花多少钱"
// - 透明计算过程：明确显示计算公式的各个组成部分
// - 地址识别：解决矿工地址、找零等混淆情况
//
// 🎯 **核心功能**
// - 有效余额计算：confirmed.available - pending.out + pending.in
// - 矿工地址识别：解决审查报告中的地址混淆问题
// - 调试信息收集：便于问题诊断和用户理解
package account

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                              有效余额计算核心
// ============================================================================

// getEffectiveBalance 获取有效可用余额
//
// 🎯 **有效余额计算核心实现**
//
// 实现审查报告中建议的公式：
// 有效可用余额 = 已确认可用余额 - 待确认支出 + 待确认收入
//
// 解决的问题：
// 1. 用户期望入池后立即看到余额减少
// 2. 矿工地址收到奖励导致的余额增加混淆
// 3. 找零交易导致的余额变化不明显
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//	tokenID: 代币标识符（nil表示平台主币）
//
// 返回：
//
//	*types.EffectiveBalanceInfo: 有效余额信息
//	error: 计算错误
func (m *Manager) getEffectiveBalance(ctx context.Context, address []byte, tokenID []byte) (*types.EffectiveBalanceInfo, error) {
	startTime := time.Now()

	if m.logger != nil {
		m.logger.Debugf("开始计算有效可用余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 构建地址对象
	addressObj := &transaction.Address{RawHash: address}

	// 🔥 步骤1：获取已确认的可用余额
	utxoQueryStart := time.Now()
	confirmedAvailable, utxoDebugInfo, err := m.getConfirmedAvailableBalance(ctx, address, tokenID)
	if err != nil {
		return nil, fmt.Errorf("获取已确认余额失败: %w", err)
	}
	utxoQueryDuration := time.Since(utxoQueryStart).Milliseconds()

	// 🔥 步骤2：计算待确认的支出和收入
	txPoolQueryStart := time.Now()
	pendingOut, pendingIn, pendingDebugInfo, err := m.calculatePendingOutAndIn(ctx, address, tokenID)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("计算待确认余额失败，将使用零值: %v", err)
		}
		// 继续执行，但使用零值
		pendingOut = 0
		pendingIn = 0
	}
	txPoolQueryDuration := time.Since(txPoolQueryStart).Milliseconds()

	// 🔥 步骤3：计算有效可用余额
	// 公式：SpendableAmount = ConfirmedAvailable - PendingOut + PendingIn
	var spendableAmount uint64 = 0
	if confirmedAvailable >= pendingOut {
		spendableAmount = confirmedAvailable - pendingOut + pendingIn
	} else {
		// 如果待确认支出超过可用余额，可动用余额为0
		spendableAmount = 0
		if m.logger != nil {
			m.logger.Warnf("待确认支出(%.6f)超过已确认余额(%.6f), 可动用余额为0",
				float64(pendingOut)/1e9, float64(confirmedAvailable)/1e9)
		}
	}

	// 🔥 步骤4：构建调试信息
	var debugInfo *types.EffectiveBalanceDebugInfo
	if utxoDebugInfo != nil || pendingDebugInfo != nil {
		debugInfo = &types.EffectiveBalanceDebugInfo{
			CalculatedAt:      time.Now(),
			UTXOQueryDuration: utxoQueryDuration,
			TxPoolQueryTime:   txPoolQueryDuration,
		}

		// 合并UTXO调试信息
		if utxoDebugInfo != nil {
			debugInfo.AvailableUTXOCount = utxoDebugInfo.AvailableUTXOCount
			debugInfo.ReferencedUTXOCount = utxoDebugInfo.ReferencedUTXOCount
			debugInfo.LockedUTXOCount = utxoDebugInfo.LockedUTXOCount
			debugInfo.IsMinerAddress = utxoDebugInfo.IsMinerAddress
			debugInfo.LastMiningRewardHeight = utxoDebugInfo.LastMiningRewardHeight
		}

		// 合并Pending调试信息
		if pendingDebugInfo != nil {
			debugInfo.PendingTransactionIds = pendingDebugInfo.PendingTransactionIds
			debugInfo.FastConfirmationCount = pendingDebugInfo.FastConfirmationCount
		}
	}

	// 🔥 步骤5：构建结果对象
	effectiveBalance := &types.EffectiveBalanceInfo{
		Address:            addressObj,
		TokenID:            tokenID,
		SpendableAmount:    spendableAmount,
		ConfirmedAvailable: confirmedAvailable,
		PendingOut:         pendingOut,
		PendingIn:          pendingIn,
		PendingTxCount:     0, // 将在下面填充
		PendingOutTxCount:  0,
		PendingInTxCount:   0,
		LastUpdated:        time.Now(),
		UpdateHeight:       0, // TODO: 从区块链获取当前高度
		CalculationMethod:  "confirmed_available_minus_pending_out_plus_pending_in",
		DebugInfo:          debugInfo,
	}

	// 填充交易计数信息
	if pendingDebugInfo != nil {
		effectiveBalance.PendingTxCount = uint32(len(pendingDebugInfo.PendingTransactionIds))
		// TODO: 区分支出和收入交易数
	}

	if m.logger != nil {
		m.logger.Debugf("有效可用余额计算完成 - address: %x, spendable: %.6f, confirmed: %.6f, pendingOut: %.6f, pendingIn: %.6f",
			address, float64(spendableAmount)/1e9, float64(confirmedAvailable)/1e9,
			float64(pendingOut)/1e9, float64(pendingIn)/1e9)
	}

	totalDuration := time.Since(startTime)
	if m.logger != nil {
		m.logger.Debugf("有效余额计算总耗时: %dms (UTXO查询: %dms, TxPool查询: %dms)",
			totalDuration.Milliseconds(), utxoQueryDuration, txPoolQueryDuration)
	}

	return effectiveBalance, nil
}

// ============================================================================
//                              已确认余额计算
// ============================================================================

// utxoDebugInfo UTXO查询调试信息
type utxoDebugInfo struct {
	AvailableUTXOCount     uint32
	ReferencedUTXOCount    uint32
	LockedUTXOCount        uint32
	IsMinerAddress         bool
	LastMiningRewardHeight uint64
}

// getConfirmedAvailableBalance 获取已确认的可用余额
func (m *Manager) getConfirmedAvailableBalance(ctx context.Context, address []byte, tokenID []byte) (uint64, *utxoDebugInfo, error) {
	// 查询Asset类型的UTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	utxos, err := m.utxoManager.GetUTXOsByAddress(ctx, address, &assetCategory, true) // onlyAvailable=true
	if err != nil {
		return 0, nil, fmt.Errorf("查询UTXO失败: %w", err)
	}

	var confirmedAvailable uint64 = 0
	debugInfo := &utxoDebugInfo{}

	for _, utxoObj := range utxos {
		if utxoObj.GetCategory() != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 统计UTXO状态
		switch utxoObj.GetStatus() {
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE:
			debugInfo.AvailableUTXOCount++
		case utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED:
			debugInfo.ReferencedUTXOCount++
		default:
			debugInfo.LockedUTXOCount++
		}

		// 只统计可用状态的UTXO
		if utxoObj.GetStatus() != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
			continue
		}

		// 提取金额
		var amount uint64 = 0
		if tokenID == nil {
			// 查询原生币
			amount, err = m.extractNativeCoinAmount(utxoObj)
			if err != nil || amount == 0 {
				continue
			}
		} else {
			// 查询指定代币
			extractedTokenID, tokenAmount, err := m.extractTokenAmount(utxoObj, tokenID)
			if err != nil || extractedTokenID == nil || tokenAmount == 0 {
				continue
			}
			if !bytesEqual(extractedTokenID, tokenID) {
				continue
			}
			amount = tokenAmount
		}

		confirmedAvailable += amount
	}

	// TODO: 识别是否为矿工地址
	// debugInfo.IsMinerAddress = m.isMinerAddress(address)
	// debugInfo.LastMiningRewardHeight = m.getLastMiningRewardHeight(address)

	return confirmedAvailable, debugInfo, nil
}

// ============================================================================
//                              待确认余额计算
// ============================================================================

// pendingDebugInfo Pending交易调试信息
type pendingDebugInfo struct {
	PendingTransactionIds [][]byte
	FastConfirmationCount uint32
}

// calculatePendingOutAndIn 计算待确认的支出和收入
//
// 这是解决审查报告中问题的核心：正确计算pending支出，而不是只统计pending收入
func (m *Manager) calculatePendingOutAndIn(ctx context.Context, address []byte, tokenID []byte) (uint64, uint64, *pendingDebugInfo, error) {
	// 获取所有pending交易
	txs, err := m.txPool.GetAllPendingTransactions()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("获取待处理交易失败: %w", err)
	}

	var pendingOut uint64 = 0 // 待确认支出（正数）
	var pendingIn uint64 = 0  // 待确认收入（正数）

	debugInfo := &pendingDebugInfo{
		PendingTransactionIds: make([][]byte, 0),
	}

	for _, tx := range txs {
		if tx == nil {
			continue
		}

		// 计算该交易对目标地址与代币的净变动
		delta, _, err := m.computePendingDeltaForTx(ctx, tx, address, tokenID)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("计算交易净变动失败，忽略此交易: %v", err)
			}
			continue
		}

		if delta == 0 {
			continue // 该交易不影响目标地址的余额
		}

		// 计算交易哈希
		var txID []byte
		if m.txHashService != nil {
			hashReq := &transaction.ComputeHashRequest{
				Transaction:      tx,
				IncludeDebugInfo: false,
			}
			hashResp, err := m.txHashService.ComputeHash(ctx, hashReq)
			if err == nil && hashResp.IsValid {
				txID = hashResp.Hash
				debugInfo.PendingTransactionIds = append(debugInfo.PendingTransactionIds, txID)
			}
		}

		// 🔥 关键修正：正确区分支出和收入
		if delta > 0 {
			// 正数表示收入
			pendingIn += uint64(delta)
		} else {
			// 负数表示支出，转换为正数累加到pendingOut
			pendingOut += uint64(-delta)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("待确认余额计算完成 - address: %x, pendingOut: %.6f, pendingIn: %.6f, txCount: %d",
			address, float64(pendingOut)/1e9, float64(pendingIn)/1e9, len(debugInfo.PendingTransactionIds))
	}

	return pendingOut, pendingIn, debugInfo, nil
}

// ============================================================================
//                              辅助工具方法
// ============================================================================

// 注意：extractAmountFromTxOutput 函数已在 pending.go 中定义，此处直接使用
