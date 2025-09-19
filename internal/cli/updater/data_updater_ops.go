package updater

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// CancelUpdate 取消更新请求
func (du *dataUpdater) CancelUpdate(ctx context.Context, requestID string) error {
	du.requestsMu.Lock()
	defer du.requestsMu.Unlock()

	result, exists := du.results[requestID]
	if !exists {
		return fmt.Errorf("更新请求不存在: %s", requestID)
	}

	// 只能取消等待中或运行中的请求
	if result.Status != PendingUpdate && result.Status != RunningUpdate {
		return fmt.Errorf("无法取消已完成的更新请求: %s", result.Status.String())
	}

	// 更新状态
	result.Status = CancelledUpdate
	result.EndTime = time.Now()
	result.Error = fmt.Errorf("用户取消")

	// 更新统计信息
	du.updateStatistics(func(stats *UpdateStatistics) {
		if result.Status == PendingUpdate {
			stats.PendingRequests--
		} else if result.Status == RunningUpdate {
			stats.RunningRequests--
		}
		stats.CancelledRequests++
		stats.LastUpdateTime = time.Now()
	})

	du.logger.Info(fmt.Sprintf("更新请求已取消: id=%s", requestID))

	return nil
}

// GetUpdateStatus 获取更新状态
func (du *dataUpdater) GetUpdateStatus(ctx context.Context, requestID string) (*UpdateResult, error) {
	du.requestsMu.RLock()
	defer du.requestsMu.RUnlock()

	result, exists := du.results[requestID]
	if !exists {
		return nil, fmt.Errorf("更新请求不存在: %s", requestID)
	}

	// 返回结果副本
	resultCopy := *result
	return &resultCopy, nil
}

// SubmitBatchUpdates 提交批量更新请求
func (du *dataUpdater) SubmitBatchUpdates(ctx context.Context, requests []*UpdateRequest) ([]string, error) {
	if len(requests) == 0 {
		return []string{}, nil
	}

	requestIDs := make([]string, 0, len(requests))

	// 逐个提交请求
	for i, request := range requests {
		requestID, err := du.SubmitUpdate(ctx, request)
		if err != nil {
			// 如果有任何请求失败，取消已提交的请求
			du.logger.Error(fmt.Sprintf("批量提交在第 %d 个请求时失败: %v", i, err))

			// 取消已提交的请求
			for _, id := range requestIDs {
				du.CancelUpdate(ctx, id)
			}

			return nil, fmt.Errorf("批量提交失败在第 %d 个请求: %v", i, err)
		}

		requestIDs = append(requestIDs, requestID)
	}

	du.logger.Info(fmt.Sprintf("批量提交完成: 成功提交 %d 个更新请求", len(requestIDs)))

	return requestIDs, nil
}

// GetBatchStatus 获取批量更新状态
func (du *dataUpdater) GetBatchStatus(ctx context.Context, requestIDs []string) ([]*UpdateResult, error) {
	if len(requestIDs) == 0 {
		return []*UpdateResult{}, nil
	}

	results := make([]*UpdateResult, 0, len(requestIDs))

	for _, requestID := range requestIDs {
		result, err := du.GetUpdateStatus(ctx, requestID)
		if err != nil {
			// 创建一个表示错误的结果
			errorResult := &UpdateResult{
				Request: &UpdateRequest{ID: requestID},
				Status:  FailedUpdate,
				Error:   err,
				EndTime: time.Now(),
			}
			results = append(results, errorResult)
		} else {
			results = append(results, result)
		}
	}

	return results, nil
}

// RegisterHandler 注册处理器
func (du *dataUpdater) RegisterHandler(handler UpdateHandler) error {
	if handler == nil {
		return fmt.Errorf("处理器不能为空")
	}

	info := handler.GetHandlerInfo()
	if info.Name == "" {
		return fmt.Errorf("处理器名称不能为空")
	}

	du.handlersMu.Lock()
	defer du.handlersMu.Unlock()

	// 检查是否已存在
	if _, exists := du.handlers[info.Name]; exists {
		return fmt.Errorf("处理器已存在: %s", info.Name)
	}

	du.handlers[info.Name] = handler

	du.logger.Info(fmt.Sprintf("注册更新处理器: name=%s, version=%s, types=%v",
		info.Name, info.Version, info.SupportedTypes))

	return nil
}

// UnregisterHandler 取消注册处理器
func (du *dataUpdater) UnregisterHandler(handlerName string) error {
	if handlerName == "" {
		return fmt.Errorf("处理器名称不能为空")
	}

	du.handlersMu.Lock()
	defer du.handlersMu.Unlock()

	if _, exists := du.handlers[handlerName]; !exists {
		return fmt.Errorf("处理器不存在: %s", handlerName)
	}

	delete(du.handlers, handlerName)

	du.logger.Info(fmt.Sprintf("取消注册更新处理器: name=%s", handlerName))

	return nil
}

// ListHandlers 列出所有处理器
func (du *dataUpdater) ListHandlers() []HandlerInfo {
	du.handlersMu.RLock()
	defer du.handlersMu.RUnlock()

	infos := make([]HandlerInfo, 0, len(du.handlers))

	for _, handler := range du.handlers {
		infos = append(infos, handler.GetHandlerInfo())
	}

	// 按名称排序
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// GetUpdateHistory 获取更新历史
func (du *dataUpdater) GetUpdateHistory(ctx context.Context, limit int) ([]*UpdateResult, error) {
	du.requestsMu.RLock()
	defer du.requestsMu.RUnlock()

	// 收集所有结果
	allResults := make([]*UpdateResult, 0, len(du.results))
	for _, result := range du.results {
		allResults = append(allResults, result)
	}

	// 按创建时间排序（最新的在前）
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Request.CreatedAt.After(allResults[j].Request.CreatedAt)
	})

	// 应用限制
	if limit > 0 && limit < len(allResults) {
		allResults = allResults[:limit]
	}

	// 返回副本
	results := make([]*UpdateResult, len(allResults))
	for i, result := range allResults {
		resultCopy := *result
		results[i] = &resultCopy
	}

	return results, nil
}

// GetUpdateStatistics 获取更新统计信息
func (du *dataUpdater) GetUpdateStatistics(ctx context.Context) *UpdateStatistics {
	du.statsMu.RLock()
	defer du.statsMu.RUnlock()

	// 计算吞吐量
	stats := *du.statistics

	// 计算每分钟吞吐量
	if !stats.LastUpdateTime.IsZero() {
		duration := time.Since(stats.LastUpdateTime)
		if duration > 0 {
			completedInLastMinute := float64(stats.CompletedRequests)
			stats.ThroughputPerMin = completedInLastMinute / duration.Minutes()
		}
	}

	return &stats
}

// SetConfig 设置配置
func (du *dataUpdater) SetConfig(config DataUpdaterConfig) {
	du.config = config
	du.logger.Info("数据更新器配置已更新")
}

// GetConfig 获取配置
func (du *dataUpdater) GetConfig() DataUpdaterConfig {
	return du.config
}

// WaitForCompletion 等待指定更新完成
func (du *dataUpdater) WaitForCompletion(ctx context.Context, requestID string, timeout time.Duration) (*UpdateResult, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		result, err := du.GetUpdateStatus(ctx, requestID)
		if err != nil {
			return nil, err
		}

		// 检查是否完成
		if result.Status == CompletedUpdate || result.Status == FailedUpdate || result.Status == CancelledUpdate {
			return result, nil
		}

		// 短暂等待
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// 继续检查
		}
	}

	return nil, fmt.Errorf("等待更新完成超时: %s", requestID)
}

// WaitForBatchCompletion 等待批量更新完成
func (du *dataUpdater) WaitForBatchCompletion(ctx context.Context, requestIDs []string, timeout time.Duration) ([]*UpdateResult, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		results, err := du.GetBatchStatus(ctx, requestIDs)
		if err != nil {
			return nil, err
		}

		// 检查是否全部完成
		allCompleted := true
		for _, result := range results {
			if result.Status != CompletedUpdate && result.Status != FailedUpdate && result.Status != CancelledUpdate {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			return results, nil
		}

		// 短暂等待
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// 继续检查
		}
	}

	return nil, fmt.Errorf("等待批量更新完成超时")
}

// GetQueueStatus 获取队列状态
func (du *dataUpdater) GetQueueStatus() QueueStatus {
	return QueueStatus{
		SystemQueueSize: len(du.systemQueue),
		UserQueueSize:   len(du.userQueue),
		SystemQueueCap:  cap(du.systemQueue),
		UserQueueCap:    cap(du.userQueue),
	}
}

// QueueStatus 队列状态
type QueueStatus struct {
	SystemQueueSize int // 系统队列当前大小
	UserQueueSize   int // 用户队列当前大小
	SystemQueueCap  int // 系统队列容量
	UserQueueCap    int // 用户队列容量
}

// PrioritySubmitUpdate 按优先级提交更新（高优先级会被优先处理）
func (du *dataUpdater) PrioritySubmitUpdate(ctx context.Context, request *UpdateRequest) (string, error) {
	// 基础提交逻辑相同
	requestID, err := du.SubmitUpdate(ctx, request)
	if err != nil {
		return "", err
	}

	// 根据优先级调整处理顺序（这里简化实现，实际可以用优先队列）
	if request.Priority == CriticalPriority {
		du.logger.Info(fmt.Sprintf("关键优先级更新: %s", requestID))
		// 这里可以实现优先处理逻辑
	}

	return requestID, nil
}

// ScheduleUpdate 调度定时更新
func (du *dataUpdater) ScheduleUpdate(ctx context.Context, request *UpdateRequest, scheduleTime time.Time) (string, error) {
	// 生成调度ID
	scheduleID := fmt.Sprintf("schedule_%s_%d", request.Type, scheduleTime.Unix())

	// 计算延迟
	delay := time.Until(scheduleTime)
	if delay <= 0 {
		// 立即执行
		return du.SubmitUpdate(ctx, request)
	}

	// 启动定时器
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// 时间到，提交更新
			if _, err := du.SubmitUpdate(ctx, request); err != nil {
				du.logger.Error(fmt.Sprintf("调度更新提交失败: %v", err))
			} else {
				du.logger.Info(fmt.Sprintf("调度更新已提交: schedule_id=%s", scheduleID))
			}
		case <-ctx.Done():
			// 上下文取消
			du.logger.Info(fmt.Sprintf("调度更新被取消: schedule_id=%s", scheduleID))
		}
	}()

	du.logger.Info(fmt.Sprintf("更新已调度: schedule_id=%s, execute_at=%s",
		scheduleID, scheduleTime.Format("2006-01-02 15:04:05")))

	return scheduleID, nil
}

// GetActiveRequests 获取活跃的请求（等待中和运行中）
func (du *dataUpdater) GetActiveRequests() []*UpdateResult {
	du.requestsMu.RLock()
	defer du.requestsMu.RUnlock()

	activeResults := make([]*UpdateResult, 0)

	for _, result := range du.results {
		if result.Status == PendingUpdate || result.Status == RunningUpdate {
			resultCopy := *result
			activeResults = append(activeResults, &resultCopy)
		}
	}

	// 按创建时间排序
	sort.Slice(activeResults, func(i, j int) bool {
		return activeResults[i].Request.CreatedAt.Before(activeResults[j].Request.CreatedAt)
	})

	return activeResults
}

// CancelAllUpdates 取消所有活跃的更新
func (du *dataUpdater) CancelAllUpdates(ctx context.Context) error {
	activeRequests := du.GetActiveRequests()

	cancelledCount := 0
	var lastError error

	for _, result := range activeRequests {
		if err := du.CancelUpdate(ctx, result.Request.ID); err != nil {
			lastError = err
			du.logger.Error(fmt.Sprintf("取消更新失败: id=%s, error=%v", result.Request.ID, err))
		} else {
			cancelledCount++
		}
	}

	du.logger.Info(fmt.Sprintf("批量取消完成: 取消了 %d 个更新", cancelledCount))

	return lastError
}
