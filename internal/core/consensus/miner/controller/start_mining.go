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
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
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

	// 全局写门闸：只读/写围栏下禁止启动挖矿（硬失败）
	if err := writegate.Default().AssertWriteAllowed(ctx, "miner.startMining"); err != nil {
		return err
	}

	// 0. 启动前检查链是否已初始化/就绪，避免在创世块尚未提交时开始挖矿
	if err := s.ensureChainReady(ctx); err != nil {
		s.logger.Info(fmt.Sprintf("链未就绪，无法启动挖矿: %v", err))
		return err
	}

	// 0.5 v2：挖矿稳定性门闸（硬门槛）
	// - 要求至少完成一轮网络交互确认（法定人数 + 高度一致性）后才能开启挖矿
	// - 单节点模式仅允许在 dev + from_genesis + allow_single_node_mining=true 下启用（由配置验证保证）
	// - **语义保证**: 门闸未通过时直接返回错误，确保 wes_startMining API 语义与状态机一致
	if s.quorumChecker != nil {
		res, err := s.quorumChecker.Check(ctx)
		if err != nil {
			return fmt.Errorf("挖矿门闸检查失败: %w", err)
		}
		if res != nil && !res.AllowMining {
			// 构建详细的错误信息，包含建议操作
			errMsg := fmt.Sprintf("挖矿门槛未通过(门闸): %s", res.Reason)
			if res.SuggestedAction != "" {
				errMsg += fmt.Sprintf("（建议操作: %s）", res.SuggestedAction)
			}
			return fmt.Errorf("%s", errMsg)
		}
	}

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

	// 2.5 确保 PoW 引擎已启动（支持在 StartMining 路径下按需重试启动）
	if s.powHandlerService != nil && !s.powHandlerService.IsRunning() {
		s.logger.Info("检测到 PoW 引擎未运行，尝试在 StartMining 路径下按需启动")
		params := types.MiningParameters{
			MiningTimeout:   s.minerConfig.MiningTimeout,
			LoopInterval:    s.minerConfig.LoopInterval,
			MaxTransactions: int(s.minerConfig.MaxTransactions),
			MinTransactions: int(s.minerConfig.MinTransactions),
			TxSelectionMode: s.minerConfig.TxSelectionMode,
		}
		if err := s.powHandlerService.StartPoWEngine(ctx, params); err != nil {
			s.logger.Errorf("在 StartMining 中启动 PoW 引擎失败: %v", err)
			return fmt.Errorf("无法启动 PoW 引擎: %w", err)
		}
		s.logger.Info("PoW 引擎已在 StartMining 路径下成功启动")
	}

	// 3. 状态更新：转为活跃状态
	if err := s.updateMinerStateToActive(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("更新矿工状态失败: %v", err))
		return fmt.Errorf("状态更新失败: %w", err)
	}

	// 3.5. 设置矿工地址到编排器（传递给激励收集器）
	if err := s.orchestratorService.SetMinerAddress(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("设置矿工地址失败: %v", err))
		return fmt.Errorf("设置矿工地址失败: %w", err)
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

// startMiningOnce 启动单次挖矿服务的具体实现
//
// 🎯 **核心功能**：
// 与 startMining 类似，但设置单次挖矿模式标志，
// 挖矿循环会在挖出一个区块后自动退出。
//
// @param ctx 上下文，支持取消操作
// @param minerAddress 矿工地址
// @return error 启动过程中的错误
func (s *MinerControllerService) startMiningOnce(ctx context.Context, minerAddress []byte) error {
	s.logger.Info("开始启动单次挖矿服务")

	// 0. 启动前检查链是否已初始化/就绪，避免在创世块尚未提交时开始挖矿
	if err := s.ensureChainReady(ctx); err != nil {
		s.logger.Info(fmt.Sprintf("链未就绪，无法启动单次挖矿: %v", err))
		return err
	}

	// 0.5 v2：挖矿稳定性门闸（开关阶段硬拒绝）
	if s.quorumChecker != nil {
		res, err := s.quorumChecker.Check(ctx)
		if err != nil {
			return fmt.Errorf("挖矿门闸检查失败: %w", err)
		}
		if res != nil && !res.AllowMining {
			return fmt.Errorf("挖矿门槛未通过(门闸): %s", res.Reason)
		}
	}

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

	// 2.5 确保 PoW 引擎已启动（支持在 StartMiningOnce 路径下按需重试启动）
	if s.powHandlerService != nil && !s.powHandlerService.IsRunning() {
		s.logger.Info("检测到 PoW 引擎未运行，尝试在 StartMiningOnce 路径下按需启动")
		params := types.MiningParameters{
			MiningTimeout:   s.minerConfig.MiningTimeout,
			LoopInterval:    s.minerConfig.LoopInterval,
			MaxTransactions: int(s.minerConfig.MaxTransactions),
			MinTransactions: int(s.minerConfig.MinTransactions),
			TxSelectionMode: s.minerConfig.TxSelectionMode,
		}
		if err := s.powHandlerService.StartPoWEngine(ctx, params); err != nil {
			s.logger.Errorf("在 StartMiningOnce 中启动 PoW 引擎失败: %v", err)
			return fmt.Errorf("无法启动 PoW 引擎: %w", err)
		}
		s.logger.Info("PoW 引擎已在 StartMiningOnce 路径下成功启动")
	}

	// 3. 状态更新：转为活跃状态，并设置单次模式标志
	if err := s.updateMinerStateToActive(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("更新矿工状态失败: %v", err))
		return fmt.Errorf("状态更新失败: %w", err)
	}

	// 3.5. 设置矿工地址到编排器（传递给激励收集器）
	if err := s.orchestratorService.SetMinerAddress(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("设置矿工地址失败: %v", err))
		return fmt.Errorf("设置矿工地址失败: %w", err)
	}

	// 🔧 设置单次挖矿模式标志
	s.mu.Lock()
	s.mineOnceMode = true
	s.mu.Unlock()
	s.logger.Info("✅ 单次挖矿模式已启用")

	// 4. 异步启动挖矿主循环
	loopCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.miningLoopCancel = cancel
	s.mu.Unlock()

	// 启动挖矿循环goroutine
	s.wg.Add(1)
	s.logger.Info("🔧 DEBUG: 准备启动单次挖矿循环goroutine")
	go s.runMiningLoop(loopCtx)

	// 5. 发布挖矿启动事件
	if err := s.publishMiningStartedEvent(minerAddress); err != nil {
		s.logger.Info(fmt.Sprintf("发布启动事件失败: %v", err))
		// 事件发布失败不影响挖矿启动
	}

	s.logger.Info("单次挖矿服务启动成功")
	return nil
}

// validateMiningParameters 验证挖矿参数
func (s *MinerControllerService) validateMiningParameters(minerAddress []byte) error {
	// 验证矿工地址
	if len(minerAddress) == 0 {
		return fmt.Errorf("矿工地址不能为空")
	}

	// 长度校验：MinerService 文档要求固定 20 字节 raw_hash
	if len(minerAddress) != 20 {
		return fmt.Errorf("矿工地址长度无效，必须为20字节")
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

// ensureChainReady 确保在启动挖矿之前链已完成创世初始化
//
// 🎯 设计目的：
// - 避免在 state:chain:tip 仍为空（创世块尚未写入）时启动挖矿
// - 防止 BlockBuilder 基于“空链尖”构造高度1且 PreviousHash 为全零的候选区块
//
// 语义：
// - 如果 ChainQuery 注入且 IsReady 返回 false，则阻止挖矿启动
// - 如果 ChainQuery 注入但调用出错，则保守起见也阻止挖矿启动
// - 如果 ChainQuery 未注入（例如某些测试场景），保持向后兼容，允许挖矿，但记录告警
func (s *MinerControllerService) ensureChainReady(ctx context.Context) error {
	if s.chainQuery == nil {
		if s.logger != nil {
			s.logger.Warn("ChainQuery 未注入，无法检查链就绪状态，在当前模式下允许启动挖矿（仅建议用于测试环境）")
		}
		return nil
	}

	isReady, err := s.chainQuery.IsReady(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("检查链就绪状态失败: %v", err)
		}
		return fmt.Errorf("检查链就绪状态失败: %w", err)
	}

	if !isReady {
		return fmt.Errorf("链尚未就绪，请等待创世区块初始化完成后再启动挖矿")
	}

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

		// 全局写门闸：一旦进入只读/写围栏，必须自动停写并退出
		if err := writegate.Default().AssertWriteAllowed(ctx, "miner.runLoop"); err != nil {
			s.logger.Warnf("写门闸阻断挖矿循环（将自动停止）: %v", err)
			go func() { _ = s.StopMining(context.Background()) }()
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

		// 🔧 修复：如果是单次挖矿模式，挖完一个区块后自动停止
		s.mu.RLock()
		isOnceMode := s.mineOnceMode
		s.mu.RUnlock()

		if isOnceMode {
			s.logger.Info("✅ 单次挖矿模式：挖矿轮次完成，自动停止挖矿循环")

			// 主动调用停止挖矿，确保状态正确清理
			go func() {
				time.Sleep(100 * time.Millisecond) // 短暂延迟，确保循环已退出
				if err := s.StopMining(context.Background()); err != nil {
					s.logger.Warnf("单次挖矿自动停止失败: %v", err)
				} else {
					s.logger.Info("✅ 单次挖矿自动停止成功")
				}
			}()

			return // 退出循环，停止挖矿
		}
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
