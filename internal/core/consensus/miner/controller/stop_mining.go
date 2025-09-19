// Package controller 实现矿工控制器的停止挖矿功能
//
// 🛑 **停止挖矿功能模块**
//
// 本文件实现 stopMining 方法的具体业务逻辑，包括：
// - 幂等性检查和状态验证
// - 优雅停止信号发送
// - WaitGroup等待循环退出
// - 状态重置和资源清理
// - 事件发布和日志记录
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/types"
)

// stopMining 停止挖矿服务的具体实现
//
// 🎯 **核心功能**：
// 1. 幂等性检查：确保服务正在运行
// 2. 状态转换：设置为Stopping状态
// 3. 停止信号：发送取消信号给挖矿循环
// 4. 等待退出：使用WaitGroup等待循环完全退出
// 5. 资源清理：重置状态和清理资源
// 6. 事件发布：发布状态变更事件
//
// @param ctx 上下文，支持超时控制
// @return error 停止过程中的错误
func (s *MinerControllerService) stopMining(ctx context.Context) error {
	s.logger.Info("开始停止挖矿服务")

	// 1. 幂等性检查：如果已经停止，直接返回成功
	if !s.isRunning.Load() {
		s.logger.Info("挖矿服务已处于停止状态")
		return nil
	}

	// 2. 状态转换：设置为停止中状态
	if err := s.setStoppingState(); err != nil {
		s.logger.Info(fmt.Sprintf("设置停止状态失败: %v", err))
		return fmt.Errorf("设置停止状态失败: %w", err)
	}

	// 3. 发送停止信号
	if err := s.sendStopSignal(); err != nil {
		s.logger.Info(fmt.Sprintf("发送停止信号失败: %v", err))
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	// 4. 等待挖矿循环完全退出
	if err := s.waitForMiningLoopExit(ctx); err != nil {
		s.logger.Info(fmt.Sprintf("等待挖矿循环退出失败: %v", err))
		return fmt.Errorf("等待挖矿循环退出失败: %w", err)
	}

	// 5. 清理资源和重置状态
	if err := s.cleanupAndResetState(); err != nil {
		s.logger.Info(fmt.Sprintf("清理资源失败: %v", err))
		return fmt.Errorf("清理资源失败: %w", err)
	}

	// 6. 发布挖矿停止事件
	if err := s.publishMiningStoppedEvent(); err != nil {
		s.logger.Info(fmt.Sprintf("发布停止事件失败: %v", err))
		// 事件发布失败不影响停止结果
	}

	s.logger.Info("挖矿服务停止成功")
	return nil
}

// setStoppingState 设置停止中状态
func (s *MinerControllerService) setStoppingState() error {
	// 通过状态管理器设置为停止中状态
	if err := s.stateManagerService.SetMinerState(types.MinerStateStopping); err != nil {
		return fmt.Errorf("无法设置停止中状态: %w", err)
	}

	s.logger.Info("矿工状态已设置为停止中")
	return nil
}

// sendStopSignal 发送停止信号
func (s *MinerControllerService) sendStopSignal() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查取消函数是否存在
	if s.miningLoopCancel == nil {
		s.logger.Info("挖矿循环取消函数不存在")
		return nil
	}

	// 发送取消信号
	s.miningLoopCancel()
	s.logger.Info("挖矿循环停止信号已发送")
	return nil
}

// waitForMiningLoopExit 等待挖矿循环退出
func (s *MinerControllerService) waitForMiningLoopExit(ctx context.Context) error {
	// 创建带超时的上下文（默认30秒超时）
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 使用channel等待WaitGroup完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("挖矿循环已完全退出")
		return nil
	case <-waitCtx.Done():
		s.logger.Info("等待挖矿循环退出超时")
		return fmt.Errorf("等待挖矿循环退出超时")
	}
}

// cleanupAndResetState 清理资源并重置状态
func (s *MinerControllerService) cleanupAndResetState() error {
	// 重置原子状态标记
	s.isRunning.Store(false)

	// 清理资源（加锁保护）
	s.mu.Lock()
	s.minerAddress = nil
	s.miningLoopCancel = nil
	s.mu.Unlock()

	// 通过状态管理器重置为空闲状态
	if err := s.stateManagerService.SetMinerState(types.MinerStateIdle); err != nil {
		return fmt.Errorf("重置矿工状态失败: %w", err)
	}

	s.logger.Info("矿工状态和资源已重置")
	return nil
}

// publishMiningStoppedEvent 发布挖矿停止事件
func (s *MinerControllerService) publishMiningStoppedEvent() error {
	// 获取当前矿工地址的副本
	s.mu.RLock()
	var minerAddress []byte
	if s.minerAddress != nil {
		minerAddress = make([]byte, len(s.minerAddress))
		copy(minerAddress, s.minerAddress)
	}
	s.mu.RUnlock()

	if s.eventBus == nil {
		// eventBus为nil时不发布事件，但不返回错误
		return nil
	}

	// 直接使用eventBus发布矿工状态变化事件
	// 事件类型定义在integration/event/events.go中
	eventType := event.EventType("consensus.miner.state_changed") // EventTypeMinerStateChanged
	eventData := map[string]interface{}{
		"old_state":     types.MinerStateActive.String(),
		"new_state":     types.MinerStateIdle.String(),
		"miner_address": minerAddress,
		"message":       "矿工停止挖矿服务",
	}

	// Publish方法没有返回值，所以不能用return
	s.eventBus.Publish(eventType, eventData)
	return nil
}
