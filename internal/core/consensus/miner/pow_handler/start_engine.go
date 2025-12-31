// start_engine.go 实现PoW引擎启动的核心逻辑
//
// 🎯 **优化的PoW引擎启动实现**
//
// 本文件实现：
// - 直接委托给注入的 POWEngine
// - 移除了过度复杂的工作器池系统
// - 移除了复杂的性能监控系统
// - 符合项目约束，使用依赖注入的哈希服务
//
// 🔧 **设计原则**：
// - 实际挖矿由 POWEngine 内部处理
// - 不需要手动的工作器管理
// - 不需要复杂的资源分配
package pow_handler

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// startPoWEngine 启动PoW引擎，准备挖矿环境
func (s *PoWComputeService) startPoWEngine(ctx context.Context, params types.MiningParameters) error {
	s.logger.Info("开始启动PoW引擎")

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 检查引擎状态
	if s.isRunning {
		s.logger.Info("PoW引擎已经在运行")
		return nil
	}

	// 2. 验证挖矿参数
	if err := s.validateMiningParams(params); err != nil {
		return fmt.Errorf("挖矿参数验证失败: %v", err)
	}

	// 3. 保存挖矿参数
	s.params = params

	// 4. 预热PoW引擎（可选优化）
	// 可以在这里进行引擎预热，例如：
	// - 验证 POWEngine 的可用性
	// - 进行一次测试挖矿来确保系统正常
	// - 预分配必要的计算资源
	if err := s.warmupPOWEngine(ctx); err != nil {
		return fmt.Errorf("PoW引擎预热失败: %v", err)
	}

	// 5. 设置运行状态
	s.isRunning = true

	s.logger.Info("PoW引擎启动完成，已准备好响应挖矿请求")
	return nil
}

// validateMiningParams 验证挖矿参数
func (s *PoWComputeService) validateMiningParams(params types.MiningParameters) error {
	// 这里可以添加必要的参数验证逻辑
	// 例如检查难度值、地址格式等
	return nil
}

// warmupPOWEngine 预热PoW引擎，确保系统就绪
func (s *PoWComputeService) warmupPOWEngine(ctx context.Context) error {
	s.logger.Info("开始预热PoW引擎")

	// 1. 验证注入的POWEngine是否可用
	if s.powEngine == nil {
		return fmt.Errorf("POWEngine未注入")
	}

	// 2. 可以进行一次轻量级的引擎测试（可选）
	// 这里可以添加引擎可用性测试逻辑

	s.logger.Info("PoW引擎预热完成")
	return nil
}
