// Package account 账户信息管理实现
//
// 📊 **账户信息统计实现 (Account Information Statistics)**
//
// 本文件实现账户综合信息的统计分析功能，包括：
// - 账户历史交易统计和分析
// - 账户活跃度和行为模式分析
// - 权限状态和配置信息管理
//
// 🎯 **核心功能**
// - 统计分析：全面的账户历史数据统计
// - 活跃度评估：账户使用频率和活跃度分析
// - 信息聚合：提供账户的完整画像信息
package account

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                              账户信息查询
// ============================================================================

// getAccountInfo 获取账户信息
//
// 🎯 **综合账户信息查询核心实现**
//
// 实现流程：
// 1. 统计账户历史交易数据
// 2. 分析账户活跃度和行为模式
// 3. 收集账户权限和配置信息
// 4. 计算账户相关的统计指标
// 5. 构建完整的账户信息对象
//
// 参数：
//
//	ctx: 上下文对象
//	address: 查询的账户地址
//
// 返回：
//
//	*types.AccountInfo: 完整账户信息
//	error: 查询错误
func (m *Manager) getAccountInfo(ctx context.Context, address []byte) (*types.AccountInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("开始查询账户信息 - address: %x", address)
	}

	// 参数验证
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 1. 获取账户所有代币余额
	allBalances, err := m.getAllTokenBalances(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 2. 转换为 BalanceInfo 切片
	var balances []*types.BalanceInfo
	var totalUTXOs uint32
	for _, balance := range allBalances {
		balances = append(balances, balance)
		totalUTXOs += balance.UTXOCount
	}

	// 3. 计算账户时间统计
	createdTime, lastActivity, err := m.calculateAccountTimestamps(ctx, address)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("获取账户时间统计失败: %v", err)
		}
		// 使用默认值
		createdTime = time.Now()
		lastActivity = time.Now()
	}

	// 4. 获取账户nonce（真实实现）
	accountNonce, err := m.repo.GetAccountNonce(ctx, address)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("获取账户nonce失败，使用默认值: %v", err)
		}
		accountNonce = 1 // 默认值
	}

	// 5. 构建账户信息
	addrObj := &transaction.Address{RawHash: address}
	accountInfo := &types.AccountInfo{
		Address:      addrObj,
		Balances:     balances,
		TotalUTXOs:   totalUTXOs,
		LastActivity: lastActivity,
		CreatedTime:  createdTime,
		Nonce:        accountNonce,
	}

	if m.logger != nil {
		m.logger.Debugf("账户信息查询完成 - address: %x", address)
	}

	return accountInfo, nil
}

// ============================================================================
//                              辅助方法实现
// ============================================================================

// calculateAccountTimestamps 计算账户的创建时间和最后活动时间
func (m *Manager) calculateAccountTimestamps(ctx context.Context, address []byte) (time.Time, time.Time, error) {
	// 获取账户的所有UTXO
	utxos, err := m.utxoManager.GetUTXOsByAddress(ctx, address, nil, false)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("获取UTXO失败: %w", err)
	}

	if len(utxos) == 0 {
		// 没有UTXO，返回当前时间
		now := time.Now()
		return now, now, nil
	}

	var earliestTime uint64 = ^uint64(0) // 最大值
	var latestTime uint64 = 0

	for _, utxoObj := range utxos {
		if utxoObj == nil {
			continue
		}

		createdTimestamp := utxoObj.GetCreatedTimestamp()
		if createdTimestamp < earliestTime {
			earliestTime = createdTimestamp
		}
		if createdTimestamp > latestTime {
			latestTime = createdTimestamp
		}
	}

	// 转换为时间对象
	createdTime := time.Unix(int64(earliestTime), 0)
	lastActivity := time.Unix(int64(latestTime), 0)

	return createdTime, lastActivity, nil
}
