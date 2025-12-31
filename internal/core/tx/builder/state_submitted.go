// Package builder 提供 Type-state Builder 实现
//
// state_submitted.go: SubmittedTx 状态实现
package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// SubmittedTx 已提交的交易（状态4）- 包装类型
//
// 🎯 **设计说明**：
// - 包装 types.SubmittedTx 以支持流式 API
// - 最终状态：提供状态查询能力，不再进行状态转换
type SubmittedTx struct {
	*types.SubmittedTx
	builder *Service
}

// GetStatus 获取交易状态
//
// 🎯 **用途**：查询交易的广播和确认状态
//
// 参数：
//   - ctx: 上下文对象
//   - processor: 交易处理器
//
// 返回：
//   - *types.TxBroadcastState: 交易广播状态
//   - error: 查询失败
//
// 💡 **使用示例**：
//
//	status, err := submittedTx.GetStatus(ctx, processor)
//	if status.Status == types.BroadcastStatusConfirmed {
//	    fmt.Println("交易已确认")
//	}
func (s *SubmittedTx) GetStatus(
	ctx context.Context,
	processor tx.TxProcessor,
) (*types.TxBroadcastState, error) {
	// 调用 processor.GetTxStatus 查询状态
	status, err := processor.GetTxStatus(ctx, s.TxHash)
	if err != nil {
		return nil, fmt.Errorf("查询交易状态失败: %w", err)
	}

	return status, nil
}

// WaitForConfirmation 等待交易确认（阻塞）
//
// 🎯 **用途**：阻塞等待交易上链确认
//
// 参数：
//   - ctx: 上下文对象
//   - processor: 交易处理器
//   - maxRetries: 最大重试次数（0 表示无限重试）
//   - interval: 轮询间隔（默认 3 秒）
//
// 返回：
//   - error: 确认失败或超时
//
// ⚠️ **注意**：
// - 这是一个简化版本，生产环境应使用事件订阅
// - 会阻塞当前 goroutine
//
// 💡 **使用示例**：
//
//	err := submittedTx.WaitForConfirmation(ctx, processor, 10, 3*time.Second)
//	if err != nil {
//	    fmt.Println("交易确认失败:", err)
//	}
func (s *SubmittedTx) WaitForConfirmation(
	ctx context.Context,
	processor tx.TxProcessor,
	maxRetries int,
	interval time.Duration,
) error {
	if interval == 0 {
		interval = 3 * time.Second // 默认 3 秒
	}

	retries := 0
	for {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("上下文已取消: %w", ctx.Err())
		default:
		}

		// 查询状态
		status, err := s.GetStatus(ctx, processor)
		if err != nil {
			return err
		}

		// 检查状态
		switch status.Status {
		case types.BroadcastStatusConfirmed:
			return nil // 确认成功
		case types.BroadcastStatusBroadcastFailed:
			return fmt.Errorf("交易广播失败: %s", status.ErrorMessage)
		case types.BroadcastStatusExpired:
			return fmt.Errorf("交易已过期")
		}

		// 检查重试次数
		retries++
		if maxRetries > 0 && retries >= maxRetries {
			return fmt.Errorf("等待确认超时（重试 %d 次）", maxRetries)
		}

		// 等待一段时间后重试
		time.Sleep(interval)
	}
}
