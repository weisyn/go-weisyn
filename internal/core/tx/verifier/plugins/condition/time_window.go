// Package condition 提供条件检查验证插件实现
//
// time_window.go: 时间窗口验证插件
package condition

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// TimeWindowPlugin 时间窗口验证插件
//
// 🎯 **核心职责**：验证交易的 validity_window.time_window 条件
//
// 💡 **设计理念**：
// 检查当前区块时间（blockTime）是否在交易指定的时间窗口内：
// - not_before_timestamp: 最早执行时间
// - not_after_timestamp: 过期时间
//
// 🔒 **验证规则**：
// 1. 如果 not_before_timestamp 设置：blockTime >= not_before_timestamp
// 2. 如果 not_after_timestamp 设置：blockTime <= not_after_timestamp
// 3. 如果两者都设置：not_before <= blockTime <= not_after
// 4. 如果都不设置：直接通过（无限制）
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Condition Hook）
type TimeWindowPlugin struct{}

// NewTimeWindowPlugin 创建新的 TimeWindowPlugin
//
// 返回：
//   - *TimeWindowPlugin: 新创建的实例
func NewTimeWindowPlugin() *TimeWindowPlugin {
	return &TimeWindowPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConditionPlugin 接口
//
// 返回：
//   - string: "time_window"
func (p *TimeWindowPlugin) Name() string {
	return "time_window"
}

// Check 检查时间窗口条件
//
// 实现 tx.ConditionPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 time_window
// 2. 如果未设置，直接通过（无时间限制）
// 3. 如果设置了，检查 blockTime 是否在窗口内
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 完整的交易对象
//   - blockHeight: 当前区块高度（本插件不使用）
//   - blockTime: 当前区块时间（Unix 时间戳，秒）
//
// 返回：
//   - error: 时间窗口检查失败的原因
//   - nil: 检查通过
//
// 📝 **错误情况**：
// - 交易还未到执行时间（too early）
// - 交易已过期（expired）
//
// 📝 **示例**：
//
//	// 场景 1：定期存款，30 天后才能解锁
//	time_window {
//	    not_before_timestamp: now + 30*24*3600  // 30 天后
//	}
//
//	// 场景 2：限时交易，必须在 24 小时内执行
//	time_window {
//	    not_after_timestamp: now + 24*3600  // 24 小时内
//	}
//
//	// 场景 3：指定时间段内执行
//	time_window {
//	    not_before_timestamp: 2025-11-01 00:00:00
//	    not_after_timestamp: 2025-12-31 23:59:59
//	}
func (p *TimeWindowPlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 1. 检查是否设置了 time_window
	timeWindow := tx.GetTimeWindow()
	if timeWindow == nil {
		// 未设置时间窗口，直接通过
		return nil
	}

	// 2. 检查 not_before_timestamp（最早执行时间）
	if timeWindow.NotBeforeTimestamp != nil {
		notBefore := *timeWindow.NotBeforeTimestamp
		if blockTime < notBefore {
			return fmt.Errorf(
				"transaction too early: current_time=%d, not_before=%d, diff=%d seconds",
				blockTime, notBefore, notBefore-blockTime,
			)
		}
	}

	// 3. 检查 not_after_timestamp（过期时间）
	if timeWindow.NotAfterTimestamp != nil {
		notAfter := *timeWindow.NotAfterTimestamp
		if blockTime > notAfter {
			return fmt.Errorf(
				"transaction expired: current_time=%d, not_after=%d, overdue=%d seconds",
				blockTime, notAfter, blockTime-notAfter,
			)
		}
	}

	// 4. 检查窗口合法性（not_before <= not_after）
	if timeWindow.NotBeforeTimestamp != nil && timeWindow.NotAfterTimestamp != nil {
		notBefore := *timeWindow.NotBeforeTimestamp
		notAfter := *timeWindow.NotAfterTimestamp
		if notBefore > notAfter {
			return fmt.Errorf(
				"invalid time window: not_before=%d > not_after=%d",
				notBefore, notAfter,
			)
		}
	}

	// 5. 所有检查通过
	return nil
}

// 编译期检查：确保 TimeWindowPlugin 实现了 tx.ConditionPlugin 接口
var _ tx.ConditionPlugin = (*TimeWindowPlugin)(nil)
