// Package height_gate 实现高度门闸管理器的高度更新功能
//
// 🔄 **高度更新功能模块**
//
// 实现 UpdateLastProcessedHeight 方法，提供分叉安全的高度更新能力。
// 该模块处理区块链分叉场景，确保高度更新的业务正确性和系统安全性。
package height_gate

// UpdateLastProcessedHeight 更新最后处理的区块高度
//
// 更新已处理的区块高度，支持正常递增和有限深度的回退。
// 超过配置深度的回退将被拒绝以防止恶意攻击。
//
// 更新规则：
// - height >= currentHeight: 允许更新
// - height < currentHeight: 仅在分叉深度限制内允许
//
// @param height 新的处理高度
func (s *HeightGateService) UpdateLastProcessedHeight(height uint64) {
	s.performHeightUpdateWithValidation(height)
}

// performHeightUpdateWithValidation 执行带验证的高度更新
//
// 执行高度更新的完整流程：验证合法性、原子更新、记录日志。
//
// @param targetHeight 目标高度
func (s *HeightGateService) performHeightUpdateWithValidation(targetHeight uint64) {
	// 步骤1: 获取当前高度进行比较分析
	currentHeight := s.getCurrentHeightForComparison()

	// 步骤2: 验证高度更新的业务合法性
	if !s.validateHeightUpdateRequest(currentHeight, targetHeight) {
		// 非法更新请求，记录错误日志并终止处理
		s.logInvalidHeightUpdateAttempt(currentHeight, targetHeight)
		return
	}

	// 步骤3: 执行原子高度更新操作
	s.executeAtomicHeightUpdate(targetHeight)

	// 步骤4: 记录高度变更的业务日志
	s.logHeightUpdateResult(currentHeight, targetHeight)
}

// getCurrentHeightForComparison 获取当前高度用于比较
//
// @return uint64 当前处理的高度
func (s *HeightGateService) getCurrentHeightForComparison() uint64 {
	return s.lastHeight.Load()
}

// validateHeightUpdateRequest 验证高度更新请求的合法性
//
// 验证高度更新是否符合业务规则：允许递增和幂等操作，
// 允许在分叉深度限制内的回退，拒绝过深的回退。
//
// @param current 当前高度
// @param target 目标高度
// @return bool 更新请求是否合法
func (s *HeightGateService) validateHeightUpdateRequest(current, target uint64) bool {
	return s.isHeightUpdateValid(current, target)
}

// executeAtomicHeightUpdate 执行原子高度更新
//
// ⚡ **极简原子操作**（遵循权威文档）：
// - 仅更新高度，无时间戳跟踪（极简设计原则）
// - 单一原子操作，确保并发安全
// - 纳秒级更新性能
//
// 🎯 **设计理由**：
// 权威文档明确要求极简设计，仅包含高度跟踪功能，
// 时间戳跟踪属于过度设计，已从架构中移除。
//
// @param newHeight 新的高度值
func (s *HeightGateService) executeAtomicHeightUpdate(newHeight uint64) {
	// 原子更新高度（极简实现）
	s.lastHeight.Store(newHeight)
}

// logHeightUpdateResult 记录高度更新结果日志
//
// 📊 **日志分类**：
// - 高度递增：记录为Info级别的正常业务日志
// - 相同高度：不记录日志（避免幂等操作的日志污染）
// - 高度回退：记录为Info级别的分叉处理日志
//
// 🎯 **日志格式**：
// 使用统一的中文格式，便于运维监控和问题排查
//
// @param previousHeight 更新前的高度
// @param newHeight 更新后的高度
func (s *HeightGateService) logHeightUpdateResult(previousHeight, newHeight uint64) {
	if newHeight > previousHeight {
		// 正常递增更新
		s.logger.Info("高度门闸更新：高度递增 " +
			s.formatHeight(previousHeight) + " → " + s.formatHeight(newHeight))
	} else if newHeight < previousHeight {
		// 分叉回退处理
		s.logger.Info("高度门闸更新：高度回退 " +
			s.formatHeight(previousHeight) + " ← " + s.formatHeight(newHeight) + " (分叉处理)")
	}
	// 相同高度不记录日志（幂等操作）
}

// logInvalidHeightUpdateAttempt 记录无效高度更新尝试
//
// 🚨 **安全日志**：
// - 记录被拒绝的恶意回退尝试
// - 便于安全审计和攻击检测
// - 使用Info级别避免误报为系统错误
//
// @param currentHeight 当前高度
// @param attemptedHeight 尝试更新的高度
func (s *HeightGateService) logInvalidHeightUpdateAttempt(currentHeight, attemptedHeight uint64) {
	rollbackDepth := currentHeight - attemptedHeight
	s.logger.Info("拒绝高度更新：回退深度过大 " +
		s.formatHeight(currentHeight) + " ← " + s.formatHeight(attemptedHeight) +
		" (深度:" + s.formatHeight(rollbackDepth) + ", 最大允许:" + s.formatHeight(s.maxForkDepth) + ")")
}

// formatHeight 格式化高度值为字符串
//
// 🚀 **性能优化**：
// - 复用manager.go中的formatUint64函数
// - 避免重复实现相同功能
// - 保持代码一致性
//
// @param height 高度值
// @return string 格式化的高度字符串
func (s *HeightGateService) formatHeight(height uint64) string {
	return formatUint64(height)
}
