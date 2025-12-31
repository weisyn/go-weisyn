// Package condition 提供条件检查验证插件实现
//
// height_window.go: 区块高度窗口验证插件
package condition

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// HeightWindowPlugin 区块高度窗口验证插件
//
// 🎯 **核心职责**：验证交易的 validity_window.height_window 条件
//
// 💡 **设计理念**：
// 检查当前区块高度（blockHeight）是否在交易指定的高度窗口内：
// - not_before_height: 最早执行区块高度
// - not_after_height: 过期区块高度
//
// 🔒 **验证规则**：
// 1. 如果 not_before_height 设置：blockHeight >= not_before_height
// 2. 如果 not_after_height 设置：blockHeight <= not_after_height
// 3. 如果两者都设置：not_before <= blockHeight <= not_after
// 4. 如果都不设置：直接通过（无限制）
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Condition Hook）
type HeightWindowPlugin struct{}

// NewHeightWindowPlugin 创建新的 HeightWindowPlugin
//
// 返回：
//   - *HeightWindowPlugin: 新创建的实例
func NewHeightWindowPlugin() *HeightWindowPlugin {
	return &HeightWindowPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConditionPlugin 接口
//
// 返回：
//   - string: "height_window"
func (p *HeightWindowPlugin) Name() string {
	return "height_window"
}

// Check 检查区块高度窗口条件
//
// 实现 tx.ConditionPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 height_window
// 2. 如果未设置，直接通过（无高度限制）
// 3. 如果设置了，检查 blockHeight 是否在窗口内
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 完整的交易对象
//   - blockHeight: 当前区块高度
//   - blockTime: 当前区块时间（本插件不使用）
//
// 返回：
//   - error: 高度窗口检查失败的原因
//   - nil: 检查通过
//
// 📝 **错误情况**：
// - 交易还未到执行高度（too early）
// - 交易已过期（expired）
//
// 📝 **示例**：
//
//	// 场景 1：锁仓释放，1000 个区块后才能解锁
//	height_window {
//	    not_before_height: current_height + 1000
//	}
//
//	// 场景 2：限时交易，必须在 100 个区块内执行
//	height_window {
//	    not_after_height: current_height + 100
//	}
//
//	// 场景 3：指定高度段内执行
//	height_window {
//	    not_before_height: 1000000
//	    not_after_height: 2000000
//	}
func (p *HeightWindowPlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 1. 检查是否设置了 height_window
	heightWindow := tx.GetHeightWindow()
	if heightWindow == nil {
		// 未设置高度窗口，直接通过
		return nil
	}

	// 2. 检查 not_before_height（最早执行高度）
	if heightWindow.NotBeforeHeight != nil {
		notBefore := *heightWindow.NotBeforeHeight
		if blockHeight < notBefore {
			return fmt.Errorf(
				"transaction too early: current_height=%d, not_before=%d, diff=%d blocks",
				blockHeight, notBefore, notBefore-blockHeight,
			)
		}
	}

	// 3. 检查 not_after_height（过期高度）
	if heightWindow.NotAfterHeight != nil {
		notAfter := *heightWindow.NotAfterHeight
		if blockHeight > notAfter {
			return fmt.Errorf(
				"transaction expired: current_height=%d, not_after=%d, overdue=%d blocks",
				blockHeight, notAfter, blockHeight-notAfter,
			)
		}
	}

	// 4. 检查窗口合法性（not_before <= not_after）
	if heightWindow.NotBeforeHeight != nil && heightWindow.NotAfterHeight != nil {
		notBefore := *heightWindow.NotBeforeHeight
		notAfter := *heightWindow.NotAfterHeight
		if notBefore > notAfter {
			return fmt.Errorf(
				"invalid height window: not_before=%d > not_after=%d",
				notBefore, notAfter,
			)
		}
	}

	// 5. 所有检查通过
	return nil
}

// 编译期检查：确保 HeightWindowPlugin 实现了 tx.ConditionPlugin 接口
var _ tx.ConditionPlugin = (*HeightWindowPlugin)(nil)
