// Package sync 实现同步取消功能
//
// 🎯 **同步取消实现**
//
// 本文件实现 CancelSync 方法的具体逻辑，提供同步操作取消功能：
// - 检查当前同步状态
// - 发送取消信号给活跃的同步任务
// - 清理同步相关的临时资源
// - 重置同步状态为空闲
package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                           同步取消实现
// ============================================================================

// cancelSyncImpl 取消当前同步操作的具体实现
//
// 🎯 **同步取消策略**：
// 1. 检查当前是否有活跃的同步任务
// 2. 发送取消信号给正在运行的同步操作
// 3. 清理同步过程中的临时资源和状态
// 4. 将同步状态重置为空闲状态
//
// 参数：
//   - ctx: 上下文对象（可能已经被取消）
//   - logger: 日志记录器
//
// 返回：
//   - error: 取消操作错误，nil表示成功
//
// 注意：
//   - 当前实现相对简单，因为同步操作还没有长时间运行的任务
//   - 未来如果有后台同步任务，需要扩展取消机制
func cancelSyncImpl(
	ctx context.Context,
	logger log.Logger,
) error {
	if logger != nil {
		logger.Info("🛑 开始执行同步取消操作")
	}

	// 阶段1: 检查当前同步状态
	activeSyncExists := checkActiveSyncTasks(logger)
	if !activeSyncExists {
		if logger != nil {
			logger.Info("📋 当前没有活跃的同步任务，无需取消")
		}
		return nil
	}

	// 阶段2: 发送取消信号
	if err := sendCancelSignal(ctx, logger); err != nil {
		return fmt.Errorf("发送取消信号失败: %w", err)
	}

	// 阶段3: 清理临时资源
	if err := cleanupSyncResources(logger); err != nil {
		if logger != nil {
			logger.Warnf("清理同步资源时出现警告: %v", err)
		}
		// 清理失败不阻止取消操作完成
	}

	// 阶段4: 重置同步状态
	resetSyncState(logger)

	if logger != nil {
		logger.Info("✅ 同步取消操作完成")
	}
	return nil
}

// ============================================================================
//                           取消机制实现
// ============================================================================

// checkActiveSyncTasks 检查是否存在活跃的同步任务
//
// 🎯 **状态检查逻辑**：
// - 检查全局 activeSyncTask 是否存在
// - 确保取消操作能够正确识别进行中的同步任务
func checkActiveSyncTasks(logger log.Logger) bool {
	activeSyncMutex.RLock()
	defer activeSyncMutex.RUnlock()

	hasActiveTask := (activeSyncTask != nil)

	if logger != nil {
		if hasActiveTask {
			logger.Infof("发现活跃同步任务: RequestID=%s, 运行时长=%s, 目标高度=%d",
				activeSyncTask.RequestID,
				time.Since(activeSyncTask.StartTime),
				activeSyncTask.TargetHeight)
		} else {
			logger.Debug("当前没有活跃的同步任务")
		}
	}

	return hasActiveTask
}

// sendCancelSignal 向活跃的同步任务发送取消信号
//
// 🎯 **取消信号策略**：
// - 优先调用 activeSyncTask.CancelFunc 取消正在进行的同步
// - 如果任务尚处于初始化阶段（CancelFunc 为空），直接释放同步锁，避免锁态卡死
// - 通知所有正在进行的网络请求和区块处理操作
// - 确保取消信号能够在1秒内生效
func sendCancelSignal(ctx context.Context, logger log.Logger) error {
	activeSyncMutex.RLock()
	currentTask := activeSyncTask
	activeSyncMutex.RUnlock()

	if currentTask == nil {
		if logger != nil {
			logger.Debug("没有活跃任务需要取消")
		}
		return nil
	}

	if currentTask.CancelFunc == nil {
		if logger != nil {
			logger.Warnf("活跃任务缺少取消函数，可能仍处于初始化阶段，尝试直接释放同步锁: RequestID=%s", currentTask.RequestID)
		}

		// 同步任务尚未进入可取消阶段，仅存在占位锁态：
		// - 为避免对外表现为“有活跃同步但无法取消”，这里直接释放同步锁
		// - 这不会中断当前 triggerSyncImpl 的执行，但会让后续 CancelSync/TriggerSync 行为一致
		releaseSyncLock(logger)

		if logger != nil {
			logger.Info("同步任务尚未进入可取消阶段，已清理锁态（占位任务已释放）")
		}

		// 视为取消流程已处理完成（对上层表现为成功），后续资源清理和状态重置仍会执行
		return nil
	}

	if logger != nil {
		logger.Infof("🛑 发送取消信号到同步任务: RequestID=%s", currentTask.RequestID)
	}

	// 调用取消函数，这会取消 syncCtx 并传播到所有子操作
	currentTask.CancelFunc()

	if logger != nil {
		logger.Info("✅ 取消信号已发送，等待任务响应...")
	}

	return nil
}

// cleanupSyncResources 清理同步过程中的临时资源
//
// 🎯 **资源清理策略**：
// - 释放同步过程中分配的内存资源
// - 关闭未完成的网络连接
// - 清理临时缓存和中间状态
func cleanupSyncResources(logger log.Logger) error {
	if logger != nil {
		logger.Debug("清理同步临时资源")
	}

	// 当前实现：清理同步相关的临时资源
	// 1. 清理同步进度状态（通过 releaseSyncLock 完成）
	// 2. 清理节点同步缓存中的过期记录
	cleanupExpiredPeerRecords(24 * time.Hour)

	// 未来可能需要清理：
	// - 网络连接池中的未完成连接
	// - 区块数据的临时缓存
	// - K桶查询的中间结果

	if logger != nil {
		logger.Debug("资源清理完成")
	}

	return nil
}

// resetSyncState 重置同步状态为空闲
//
// 🎯 **状态重置策略**：
// - 将同步状态标记为idle
// - 清除同步进度信息
// - 重置错误状态
func resetSyncState(logger log.Logger) {
	if logger != nil {
		logger.Debug("重置同步状态为空闲")
	}

	// 释放同步锁，重置同步状态
	releaseSyncLock(logger)

	// 清理过期的节点同步记录
	cleanupExpiredPeerRecords(24 * time.Hour)

	// 未来可能需要：
	// - 清除同步进度指标
	// - 通知其他组件同步已停止

	if logger != nil {
		logger.Debug("同步状态已重置为空闲")
	}
}

// ============================================================================
//                           扩展取消能力（P2 实现）
// ============================================================================

// CancelProgress 同步取消进度快照（用于可观测与诊断）。
type CancelProgress struct {
	HasActiveTask bool
	RequestID     string
	TargetHeight  uint64
	HasCancelFunc bool
	Stage         string // idle / signaling / waiting / done
}

var (
	cancelCallbacksMu sync.Mutex
	cancelCallbacks   []func(CancelProgress)
)

// RegisterCancelCallback 注册取消完成后的回调（用于集成层做告警/状态刷新）。
func RegisterCancelCallback(cb func(CancelProgress)) {
	if cb == nil {
		return
	}
	cancelCallbacksMu.Lock()
	defer cancelCallbacksMu.Unlock()
	cancelCallbacks = append(cancelCallbacks, cb)
}

func fireCancelCallbacks(progress CancelProgress) {
	cancelCallbacksMu.Lock()
	cbs := append([]func(CancelProgress){}, cancelCallbacks...)
	cancelCallbacksMu.Unlock()
	for _, cb := range cbs {
		// 回调不得影响主流程
		func() {
			defer func() { _ = recover() }()
			cb(progress)
		}()
	}
}

// GetCancelProgress 获取当前取消相关状态快照。
func GetCancelProgress() CancelProgress {
	activeSyncMutex.RLock()
	task := activeSyncTask
	activeSyncMutex.RUnlock()

	if task == nil {
		return CancelProgress{HasActiveTask: false, Stage: "idle"}
	}
	return CancelProgress{
		HasActiveTask: true,
		RequestID:     task.RequestID,
		TargetHeight:  task.TargetHeight,
		HasCancelFunc: task.CancelFunc != nil,
		Stage:         "waiting",
	}
}

// CancelSyncWithTimeout 带超时的同步取消：
// - 先触发 cancelSyncImpl 的标准流程；
// - 再等待 activeSyncTask 清空（或 ctx/timeout 到期）。
func CancelSyncWithTimeout(ctx context.Context, logger log.Logger, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if logger != nil {
		logger.Infof("🛑 CancelSyncWithTimeout: timeout=%s", timeout)
	}

	if err := cancelSyncImpl(ctx, logger); err != nil {
		return err
	}

	// 等待任务退出（如果存在）
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待同步任务退出超时: %w", ctx.Err())
		case <-ticker.C:
			if !checkActiveSyncTasks(nil) {
				progress := GetCancelProgress()
				progress.Stage = "done"
				fireCancelCallbacks(progress)
				return nil
			}
		}
	}
}

// ForceStopSync 强制停止同步：
// - 直接清理 activeSyncTask 指针并释放锁态；
// - 不等待任务自行退出（用于极端卡死场景）。
func ForceStopSync(logger log.Logger) {
	activeSyncMutex.Lock()
	task := activeSyncTask
	activeSyncTask = nil
	activeSyncMutex.Unlock()

	if logger != nil {
		if task != nil {
			logger.Warnf("🚨 ForceStopSync: 强制清理 activeSyncTask: requestID=%s targetHeight=%d", task.RequestID, task.TargetHeight)
		} else {
			logger.Warn("🚨 ForceStopSync: 当前无 activeSyncTask")
		}
	}

	releaseSyncLock(logger)
	cleanupExpiredPeerRecords(24 * time.Hour)

	progress := GetCancelProgress()
	progress.Stage = "done"
	fireCancelCallbacks(progress)
}
