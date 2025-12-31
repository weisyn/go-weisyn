// Package fork 实现只读模式入口（全局写门闸版本）
//
// 说明（严格对齐“全局 NodeWriteGate”设计）：
// - 只读模式 = 全禁写（任何写操作必须硬失败）
// - 写围栏 = 仅携带 token 的受控窗口可写（用于 reorg）
package fork

import (
	"context"
	"fmt"
	"time"

	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
)

// ReadOnlyModeEvent 只读模式事件数据
type ReadOnlyModeEvent struct {
	Reason    string
	Timestamp time.Time
	Component string
}

// enterReadOnlyMode 进入只读模式
//
// 🎯 **功能**：
// 1. 设置只读模式状态
// 2. 关闭所有写操作（挖矿、聚合器、交易池）
// 3. 发布告警事件
// 4. 记录详细日志
//
// ⚠️ **注意**：
// - 此方法应在 REORG 失败且无法恢复时调用
// - 进入只读模式后需要人工介入
// - 节点将无法处理新交易和出块
//
// 参数：
//   - ctx: 操作上下文
//   - reason: 进入只读模式的原因
//
// 返回：
//   - error: 设置失败的错误（通常不会失败）
func (s *Service) enterReadOnlyMode(ctx context.Context, reason string) error {
	if s.logger != nil {
		s.logger.Errorf("🔒 进入只读模式: reason=%s", reason)
		s.logger.Errorf("⚠️ 节点已进入只读模式，所有写操作将被拒绝")
		s.logger.Errorf("⚠️ 建议操作：")
		s.logger.Errorf("   1. 检查数据完整性（区块、UTXO、索引）")
		s.logger.Errorf("   2. 查看错误日志，识别根本原因")
		s.logger.Errorf("   3. 从备份恢复或从网络重新同步")
		s.logger.Errorf("   4. 联系技术支持")
	}

	// 1) 全局写门闸：进入只读后全禁写（硬失败）
	if s.writeGate != nil {
		s.writeGate.EnterReadOnly(reason)
	}

	// 2. 发布只读模式事件
	if s.eventBus != nil {
		event := ReadOnlyModeEvent{
			Reason:    reason,
			Timestamp: time.Now(),
			Component: "fork-handler",
		}
		s.eventBus.Publish(eventiface.EventType("readonly_mode_entered"), ctx, event)
	}

	// 3) 模块联动：由后续 enforce-gate-* 任务实现（所有写路径将硬接入 Gate）

	return fmt.Errorf("entered read-only mode: %s", reason)
}

// isReadOnly 检查是否处于只读模式
//
// 🎯 **功能**：
// - 快速检查只读模式状态
// - 用于拒绝写操作
//
// 返回：
//   - bool: 是否处于只读模式
func (s *Service) isReadOnly() bool {
	if s == nil || s.writeGate == nil {
		return false
	}
	return s.writeGate.IsReadOnly()
}

// getReadOnlyReason 获取只读模式原因
//
// 🎯 **功能**：
// - 获取进入只读模式的原因
// - 用于错误提示
//
// 返回：
//   - string: 只读模式原因
func (s *Service) getReadOnlyReason() string {
	if s == nil || s.writeGate == nil {
		return ""
	}
	return s.writeGate.ReadOnlyReason()
}

// CheckWriteAllowed 检查是否允许写操作
//
// 🎯 **功能**：
// - 统一的写操作检查接口
// - 在只读模式下拒绝写操作
//
// 参数：
//   - operation: 操作名称（用于日志）
//
// 返回：
//   - error: 如果处于只读模式，返回错误
func (s *Service) CheckWriteAllowed(ctx context.Context, operation string) error {
	if s == nil || s.writeGate == nil {
		return nil
	}
	return s.writeGate.AssertWriteAllowed(ctx, operation)
}

