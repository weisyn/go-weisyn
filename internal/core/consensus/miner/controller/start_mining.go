// Package controller 实现矿工控制器的启动挖矿功能
//
// 📋 **启动挖矿功能模块**
//
// 本文件实现 startMining 方法的具体业务逻辑，包括：
// - 参数校验和状态检查
// - 异步启动挖矿主循环
// - 状态转换和事件发布
// - 错误处理和资源清理
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/types"
)

// startMining 启动挖矿服务的具体实现
//
// 🎯 **核心功能**：
// 1. 参数校验：验证矿工地址有效性
// 2. 状态检查：确保当前状态允许启动挖矿
// 3. 状态转换：从Idle转为Active状态
// 4. 循环启动：异步启动挖矿主循环
// 5. 事件发布：发布状态变更事件
//
// @param ctx 上下文，支持取消操作
// @param minerAddress 矿工地址
// @return error 启动过程中的错误
func (s *MinerControllerService) startMining(ctx context.Context, minerAddress []byte) error {
	s.logger.Info("开始启动挖矿服务")

	// 1. 参数校验
	if err := s.validateMiningParameters(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("挖矿参数校验失败: %v", err))
		return fmt.Errorf("矿工参数校验失败: %w", err)
	}

	// 2. 状态检查：确保当前状态允许启动
	if err := s.checkCanStartMining(); err != nil {
		s.logger.Info(fmt.Sprintf("无法启动挖矿: %v", err))
		return err
	}

	// 3. 状态更新：转为活跃状态
	if err := s.updateMinerStateToActive(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("更新矿工状态失败: %v", err))
		return fmt.Errorf("状态更新失败: %w", err)
	}

	// 4. 异步启动挖矿主循环
	loopCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.miningLoopCancel = cancel
	s.mu.Unlock()

	// 启动挖矿循环goroutine
	s.wg.Add(1)
	s.logger.Info("🔧 DEBUG: 准备启动挖矿循环goroutine")
	go s.runMiningLoop(loopCtx)

	// 5. 发布挖矿启动事件
	if err := s.publishMiningStartedEvent(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("发布启动事件失败: %v", err))
		// 事件发布失败不影响挖矿启动
	}

	s.logger.Info("挖矿服务启动成功")
	return nil
}

// validateMiningParameters 验证挖矿参数
func (s *MinerControllerService) validateMiningParameters(minerAddress []byte) error {
	// 验证矿工地址
	if len(minerAddress) == 0 {
		return fmt.Errorf("矿工地址不能为空")
	}

	// 简单长度校验（具体格式校验应由地址管理器处理）
	if len(minerAddress) < 20 {
		return fmt.Errorf("矿工地址长度无效")
	}

	return nil
}

// checkCanStartMining 检查是否可以启动挖矿
func (s *MinerControllerService) checkCanStartMining() error {
	// 检查当前是否已经在运行
	if s.isRunning.Load() {
		return fmt.Errorf("挖矿服务已在运行中")
	}

	// 通过状态管理器检查内部状态
	currentState := s.stateManagerService.GetMinerState()
	if currentState != types.MinerStateIdle && currentState != types.MinerStateError {
		return fmt.Errorf("当前状态(%s)不允许启动挖矿", currentState.String())
	}

	return nil
}

// updateMinerStateToActive 更新矿工状态为活跃
func (s *MinerControllerService) updateMinerStateToActive(minerAddress []byte) error {
	// 更新原子状态标记
	s.isRunning.Store(true)

	// 保存矿工地址（加锁保护）
	s.mu.Lock()
	s.minerAddress = make([]byte, len(minerAddress))
	copy(s.minerAddress, minerAddress)
	s.mu.Unlock()

	// 通过状态管理器更新内部状态
	if err := s.stateManagerService.SetMinerState(types.MinerStateActive); err != nil {
		// 状态更新失败时回滚
		s.isRunning.Store(false)
		s.mu.Lock()
		s.minerAddress = nil
		s.mu.Unlock()
		return fmt.Errorf("设置矿工状态失败: %w", err)
	}

	return nil
}

// publishMiningStartedEvent 发布挖矿启动事件
func (s *MinerControllerService) publishMiningStartedEvent(minerAddress []byte) error {
	if s.eventBus == nil {
		// eventBus为nil时不发布事件，但不返回错误
		return nil
	}

	// 直接使用eventBus发布矿工状态变化事件
	// 事件类型定义在integration/event/events.go中
	eventType := event.EventType("consensus.miner.state_changed") // EventTypeMinerStateChanged
	eventData := map[string]interface{}{
		"old_state":     types.MinerStateIdle.String(),
		"new_state":     types.MinerStateActive.String(),
		"miner_address": minerAddress,
		"message":       "矿工启动挖矿服务",
	}

	// Publish方法没有返回值，所以不能用return
	s.eventBus.Publish(eventType, eventData)
	return nil
}

// runMiningLoop 挖矿主循环实现（修正架构设计）
//
// 🎯 **Controller作为启动器的职责**：
// 1. 基础状态检查：确保可以挖矿
// 2. 委托给编排器：通过MiningOrchestrator执行具体业务
// 3. 等待触发机制：挖出区块后等待新区块/同步事件触发，而非时间循环
// 4. 优雅退出：监听停止信号
//
// ⚠️  **架构修正**：
// - 不再是时间驱动的循环挖矿
// - 挖出区块提交后等待外部触发（新区块到达/同步完成）
// - 大部分逻辑委托给orchestrator等子组件，避免重复造轮子
//
// @param ctx 挖矿循环的上下文，支持取消操作
func (s *MinerControllerService) runMiningLoop(ctx context.Context) {
	defer s.wg.Done() // 确保WaitGroup计数正确递减

	s.logger.Info("挖矿监听循环启动")

	for {
		// 1. 检查停止信号
		select {
		case <-ctx.Done():
			s.logger.Info("收到停止信号，挖矿循环退出")
			return
		default:
		}

		// 2. 基础状态检查（Controller的基本职责）
		if !s.shouldContinueMining() {
			s.logger.Info("矿工状态不允许挖矿，循环退出")
			return
		}

		s.logger.Info("🔧 DEBUG: 开始执行挖矿轮次")

		// 3. 委托给编排器执行完整挖矿业务逻辑
		// 编排器负责：高度门闸、候选区块创建、PoW计算、提交aggregator、等待触发
		if err := s.orchestratorService.ExecuteMiningRound(ctx); err != nil {
			s.logger.Info(fmt.Sprintf("🔧 DEBUG: 挖矿轮次执行失败: %v", err))

			// 失败时的简单处理：短暂等待后重试
			// 具体的错误治理逻辑应该在orchestrator中处理
			if !s.waitWithCancellation(ctx, 1*time.Second) {
				return
			}
			continue
		}

		// 4. 挖矿轮次完成后，orchestrator内部会：
		//    - 如果挖出区块：提交给aggregator并等待新区块触发
		//    - 如果未挖出：等待同步或其他触发条件
		//    - 该方法会阻塞直到有新的挖矿条件
		s.logger.Info("🔧 DEBUG: 挖矿轮次完成，等待下一轮触发")
	}
}

// shouldContinueMining 检查是否应该继续挖矿（Controller基本职责）
func (s *MinerControllerService) shouldContinueMining() bool {
	// Controller只需要检查基本的运行状态
	// 具体的状态管理和业务判断由orchestrator等子组件负责
	return s.isRunning.Load()
}

// waitWithCancellation 带取消功能的等待（基础工具方法）
func (s *MinerControllerService) waitWithCancellation(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false // 收到取消信号
	case <-timer.C:
		return true // 等待完成
	}
}
