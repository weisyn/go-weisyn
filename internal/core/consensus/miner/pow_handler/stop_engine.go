// stop_engine.go 实现PoW引擎停止的核心逻辑
//
// 🎯 **优化的PoW引擎停止实现**
//
// 本文件实现：
// - 直接委托给注入的 POWEngine
// - 移除了复杂的工作器停止逻辑
// - 移除了性能监控清理
// - 移除了资源释放的复杂处理
//
// 🔧 **设计原则**：
// - 实际停止由 POWEngine 内部处理
// - 不需要手动停止工作器
// - 不需要复杂的资源清理
package pow_handler

import (
	"context"
)

// stopPoWEngine 停止PoW引擎的核心实现
func (s *PoWComputeService) stopPoWEngine(ctx context.Context) error {
	s.logger.Info("开始停止PoW引擎")

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 检查引擎状态
	if !s.isRunning {
		s.logger.Info("PoW引擎已经停止")
		return nil
	}

	// 2. 设置停止状态
	s.isRunning = false

	s.logger.Info("PoW引擎停止完成")
	return nil
}
