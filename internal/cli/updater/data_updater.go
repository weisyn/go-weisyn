// Package updater 提供CLI的分层数据更新机制
package updater

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// UpdateType 更新类型
type UpdateType string

const (
	// SystemUpdate 系统级更新
	SystemUpdate UpdateType = "system"
	// UserUpdate 用户级更新
	UserUpdate UpdateType = "user"
	// ConfigUpdate 配置更新
	ConfigUpdate UpdateType = "config"
	// DataUpdate 数据更新
	DataUpdate UpdateType = "data"
)

// UpdatePriority 更新优先级
type UpdatePriority int

const (
	// LowPriority 低优先级
	LowPriority UpdatePriority = iota
	// MediumPriority 中等优先级
	MediumPriority
	// HighPriority 高优先级
	HighPriority
	// CriticalPriority 关键优先级
	CriticalPriority
)

// UpdateStatus 更新状态
type UpdateStatus int

const (
	// PendingUpdate 等待更新
	PendingUpdate UpdateStatus = iota
	// RunningUpdate 正在更新
	RunningUpdate
	// CompletedUpdate 更新完成
	CompletedUpdate
	// FailedUpdate 更新失败
	FailedUpdate
	// CancelledUpdate 更新取消
	CancelledUpdate
)

// String 返回更新状态的字符串表示
func (us UpdateStatus) String() string {
	switch us {
	case PendingUpdate:
		return "Pending"
	case RunningUpdate:
		return "Running"
	case CompletedUpdate:
		return "Completed"
	case FailedUpdate:
		return "Failed"
	case CancelledUpdate:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// UpdateRequest 更新请求
type UpdateRequest struct {
	ID          string                 // 更新ID
	Type        UpdateType             // 更新类型
	Priority    UpdatePriority         // 优先级
	Title       string                 // 标题
	Description string                 // 描述
	Source      string                 // 数据源
	Target      string                 // 目标
	Parameters  map[string]interface{} // 参数
	Timeout     time.Duration          // 超时时间
	Retry       int                    // 重试次数
	CreatedAt   time.Time              // 创建时间
	UpdatedAt   time.Time              // 更新时间
}

// UpdateResult 更新结果
type UpdateResult struct {
	Request    *UpdateRequest         // 对应的请求
	Status     UpdateStatus           // 状态
	StartTime  time.Time              // 开始时间
	EndTime    time.Time              // 结束时间
	Duration   time.Duration          // 执行时长
	Data       interface{}            // 结果数据
	Error      error                  // 错误信息
	Metadata   map[string]interface{} // 元数据
	RetryCount int                    // 重试次数
}

// UpdateHandler 更新处理器接口
type UpdateHandler interface {
	// CanHandle 检查是否能处理此类型的更新
	CanHandle(request *UpdateRequest) bool

	// Handle 处理更新请求
	Handle(ctx context.Context, request *UpdateRequest) (*UpdateResult, error)

	// GetHandlerInfo 获取处理器信息
	GetHandlerInfo() HandlerInfo
}

// HandlerInfo 处理器信息
type HandlerInfo struct {
	Name           string       // 处理器名称
	Version        string       // 版本
	SupportedTypes []UpdateType // 支持的更新类型
	Description    string       // 描述
}

// DataUpdaterConfig 数据更新器配置
type DataUpdaterConfig struct {
	// 并发设置
	MaxConcurrentUpdates int // 最大并发更新数
	SystemUpdateWorkers  int // 系统级更新工作者数量
	UserUpdateWorkers    int // 用户级更新工作者数量

	// 超时设置
	DefaultTimeout      time.Duration // 默认超时时间
	SystemUpdateTimeout time.Duration // 系统级更新超时
	UserUpdateTimeout   time.Duration // 用户级更新超时

	// 重试设置
	DefaultRetryCount int           // 默认重试次数
	RetryDelay        time.Duration // 重试延迟

	// 存储设置
	MaxHistorySize  int           // 最大历史记录数
	CleanupInterval time.Duration // 清理间隔

	// 权限设置
	RequirePermissions bool // 是否需要权限检查
}

// DataUpdater 分层数据更新器接口
type DataUpdater interface {
	// 请求管理
	SubmitUpdate(ctx context.Context, request *UpdateRequest) (string, error)
	CancelUpdate(ctx context.Context, requestID string) error
	GetUpdateStatus(ctx context.Context, requestID string) (*UpdateResult, error)

	// 批量操作
	SubmitBatchUpdates(ctx context.Context, requests []*UpdateRequest) ([]string, error)
	GetBatchStatus(ctx context.Context, requestIDs []string) ([]*UpdateResult, error)

	// 处理器管理
	RegisterHandler(handler UpdateHandler) error
	UnregisterHandler(handlerName string) error
	ListHandlers() []HandlerInfo

	// 监控和统计
	GetUpdateHistory(ctx context.Context, limit int) ([]*UpdateResult, error)
	GetUpdateStatistics(ctx context.Context) *UpdateStatistics

	// 配置管理
	SetConfig(config DataUpdaterConfig)
	GetConfig() DataUpdaterConfig

	// 生命周期
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
}

// UpdateStatistics 更新统计信息
type UpdateStatistics struct {
	TotalRequests     int64         // 总请求数
	PendingRequests   int64         // 等待中请求数
	RunningRequests   int64         // 执行中请求数
	CompletedRequests int64         // 完成请求数
	FailedRequests    int64         // 失败请求数
	CancelledRequests int64         // 取消请求数
	AverageLatency    time.Duration // 平均延迟
	ThroughputPerMin  float64       // 每分钟吞吐量
	LastUpdateTime    time.Time     // 最后更新时间
}

// dataUpdater 分层数据更新器实现
type dataUpdater struct {
	logger            log.Logger
	permissionManager *permissions.Manager
	config            DataUpdaterConfig

	// 处理器管理
	handlers   map[string]UpdateHandler
	handlersMu sync.RWMutex

	// 请求管理
	requests   map[string]*UpdateRequest
	results    map[string]*UpdateResult
	requestsMu sync.RWMutex

	// 工作队列
	systemQueue chan *UpdateRequest
	userQueue   chan *UpdateRequest

	// 统计信息
	statistics *UpdateStatistics
	statsMu    sync.RWMutex

	// 生命周期
	running    bool
	runningMu  sync.RWMutex
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewDataUpdater 创建数据更新器
func NewDataUpdater(
	logger log.Logger,
	permissionManager *permissions.Manager,
	config DataUpdaterConfig,
) DataUpdater {
	if config.MaxConcurrentUpdates == 0 {
		config.MaxConcurrentUpdates = 10
	}
	if config.SystemUpdateWorkers == 0 {
		config.SystemUpdateWorkers = 3
	}
	if config.UserUpdateWorkers == 0 {
		config.UserUpdateWorkers = 2
	}
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	return &dataUpdater{
		logger:            logger,
		permissionManager: permissionManager,
		config:            config,
		handlers:          make(map[string]UpdateHandler),
		requests:          make(map[string]*UpdateRequest),
		results:           make(map[string]*UpdateResult),
		systemQueue:       make(chan *UpdateRequest, 100),
		userQueue:         make(chan *UpdateRequest, 100),
		statistics: &UpdateStatistics{
			LastUpdateTime: time.Now(),
		},
	}
}

// Start 启动数据更新器
func (du *dataUpdater) Start(ctx context.Context) error {
	du.runningMu.Lock()
	defer du.runningMu.Unlock()

	if du.running {
		return fmt.Errorf("数据更新器已在运行")
	}

	du.logger.Info("启动分层数据更新器")

	// 创建取消上下文
	ctx, du.cancelFunc = context.WithCancel(ctx)

	// 启动系统级工作者
	for i := 0; i < du.config.SystemUpdateWorkers; i++ {
		du.wg.Add(1)
		go du.systemWorker(ctx, i)
	}

	// 启动用户级工作者
	for i := 0; i < du.config.UserUpdateWorkers; i++ {
		du.wg.Add(1)
		go du.userWorker(ctx, i)
	}

	// 启动清理工作者
	du.wg.Add(1)
	go du.cleanupWorker(ctx)

	du.running = true
	du.logger.Info("分层数据更新器启动完成")

	return nil
}

// Stop 停止数据更新器
func (du *dataUpdater) Stop(ctx context.Context) error {
	du.runningMu.Lock()
	defer du.runningMu.Unlock()

	if !du.running {
		return fmt.Errorf("数据更新器未运行")
	}

	du.logger.Info("停止分层数据更新器")

	// 取消上下文
	if du.cancelFunc != nil {
		du.cancelFunc()
	}

	// 等待所有工作者完成
	du.wg.Wait()

	du.running = false
	du.logger.Info("分层数据更新器已停止")

	return nil
}

// IsRunning 检查是否正在运行
func (du *dataUpdater) IsRunning() bool {
	du.runningMu.RLock()
	defer du.runningMu.RUnlock()
	return du.running
}

// SubmitUpdate 提交更新请求
func (du *dataUpdater) SubmitUpdate(ctx context.Context, request *UpdateRequest) (string, error) {
	// 生成请求ID
	if request.ID == "" {
		request.ID = generateRequestID()
	}

	// 设置创建时间
	if request.CreatedAt.IsZero() {
		request.CreatedAt = time.Now()
	}
	request.UpdatedAt = time.Now()

	// 权限检查
	if du.config.RequirePermissions {
		if err := du.checkUpdatePermissions(ctx, request); err != nil {
			return "", fmt.Errorf("权限检查失败: %v", err)
		}
	}

	// 查找处理器
	handler := du.findHandler(request)
	if handler == nil {
		return "", fmt.Errorf("未找到适合的处理器: type=%s", request.Type)
	}

	// 存储请求
	du.requestsMu.Lock()
	du.requests[request.ID] = request
	du.results[request.ID] = &UpdateResult{
		Request:   request,
		Status:    PendingUpdate,
		StartTime: time.Time{},
		Metadata:  make(map[string]interface{}),
	}
	du.requestsMu.Unlock()

	// 根据类型将请求加入相应队列
	switch request.Type {
	case SystemUpdate, ConfigUpdate:
		select {
		case du.systemQueue <- request:
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			return "", fmt.Errorf("系统更新队列已满")
		}
	case UserUpdate, DataUpdate:
		select {
		case du.userQueue <- request:
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			return "", fmt.Errorf("用户更新队列已满")
		}
	default:
		return "", fmt.Errorf("不支持的更新类型: %s", request.Type)
	}

	// 更新统计信息
	du.updateStatistics(func(stats *UpdateStatistics) {
		stats.TotalRequests++
		stats.PendingRequests++
		stats.LastUpdateTime = time.Now()
	})

	du.logger.Info(fmt.Sprintf("提交更新请求: id=%s, type=%s", request.ID, request.Type))

	return request.ID, nil
}

// systemWorker 系统级更新工作者
func (du *dataUpdater) systemWorker(ctx context.Context, workerID int) {
	defer du.wg.Done()

	du.logger.Info(fmt.Sprintf("启动系统级更新工作者 %d", workerID))

	for {
		select {
		case request := <-du.systemQueue:
			du.processUpdate(ctx, request, fmt.Sprintf("system-worker-%d", workerID))
		case <-ctx.Done():
			du.logger.Info(fmt.Sprintf("系统级更新工作者 %d 已停止", workerID))
			return
		}
	}
}

// userWorker 用户级更新工作者
func (du *dataUpdater) userWorker(ctx context.Context, workerID int) {
	defer du.wg.Done()

	du.logger.Info(fmt.Sprintf("启动用户级更新工作者 %d", workerID))

	for {
		select {
		case request := <-du.userQueue:
			du.processUpdate(ctx, request, fmt.Sprintf("user-worker-%d", workerID))
		case <-ctx.Done():
			du.logger.Info(fmt.Sprintf("用户级更新工作者 %d 已停止", workerID))
			return
		}
	}
}

// processUpdate 处理更新请求
func (du *dataUpdater) processUpdate(ctx context.Context, request *UpdateRequest, workerID string) {
	startTime := time.Now()

	// 查找处理器
	handler := du.findHandler(request)
	if handler == nil {
		du.updateResult(request.ID, func(result *UpdateResult) {
			result.Status = FailedUpdate
			result.Error = fmt.Errorf("未找到适合的处理器")
			result.EndTime = time.Now()
			result.Duration = time.Since(startTime)
		})
		return
	}

	// 更新状态为运行中
	du.updateResult(request.ID, func(result *UpdateResult) {
		result.Status = RunningUpdate
		result.StartTime = startTime
		result.Metadata["worker_id"] = workerID
		result.Metadata["handler"] = handler.GetHandlerInfo().Name
	})

	// 更新统计信息
	du.updateStatistics(func(stats *UpdateStatistics) {
		stats.PendingRequests--
		stats.RunningRequests++
	})

	du.logger.Info(fmt.Sprintf("开始处理更新请求: id=%s, type=%s, worker=%s",
		request.ID, request.Type, workerID))

	// 设置超时
	timeout := request.Timeout
	if timeout == 0 {
		switch request.Type {
		case SystemUpdate:
			timeout = du.config.SystemUpdateTimeout
		case UserUpdate:
			timeout = du.config.UserUpdateTimeout
		default:
			timeout = du.config.DefaultTimeout
		}
	}

	processCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 执行处理器
	result, err := handler.Handle(processCtx, request)
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// 更新结果
	du.updateResult(request.ID, func(updateResult *UpdateResult) {
		updateResult.EndTime = endTime
		updateResult.Duration = duration

		if err != nil {
			updateResult.Status = FailedUpdate
			updateResult.Error = err
		} else if result != nil {
			updateResult.Status = CompletedUpdate
			updateResult.Data = result.Data
			updateResult.Error = result.Error

			// 合并元数据
			for key, value := range result.Metadata {
				updateResult.Metadata[key] = value
			}
		} else {
			updateResult.Status = CompletedUpdate
		}
	})

	// 更新统计信息
	du.updateStatistics(func(stats *UpdateStatistics) {
		stats.RunningRequests--
		if err != nil {
			stats.FailedRequests++
		} else {
			stats.CompletedRequests++
		}

		// 更新平均延迟
		if stats.AverageLatency == 0 {
			stats.AverageLatency = duration
		} else {
			stats.AverageLatency = (stats.AverageLatency + duration) / 2
		}

		stats.LastUpdateTime = time.Now()
	})

	if err != nil {
		du.logger.Error(fmt.Sprintf("更新请求处理失败: id=%s, error=%v, duration=%v",
			request.ID, err, duration))
	} else {
		du.logger.Info(fmt.Sprintf("更新请求处理完成: id=%s, duration=%v",
			request.ID, duration))
	}
}

// cleanupWorker 清理工作者
func (du *dataUpdater) cleanupWorker(ctx context.Context) {
	defer du.wg.Done()

	ticker := time.NewTicker(du.config.CleanupInterval)
	if du.config.CleanupInterval == 0 {
		ticker = time.NewTicker(1 * time.Hour) // 默认1小时清理一次
	}
	defer ticker.Stop()

	du.logger.Info("启动清理工作者")

	for {
		select {
		case <-ticker.C:
			du.performCleanup()
		case <-ctx.Done():
			du.logger.Info("清理工作者已停止")
			return
		}
	}
}

// performCleanup 执行清理
func (du *dataUpdater) performCleanup() {
	du.logger.Info("开始执行更新历史清理")

	du.requestsMu.Lock()
	defer du.requestsMu.Unlock()

	// 如果历史记录超过最大限制，删除最旧的记录
	if du.config.MaxHistorySize > 0 && len(du.results) > du.config.MaxHistorySize {
		// 收集所有完成的结果，按时间排序
		type resultWithTime struct {
			id     string
			result *UpdateResult
		}

		completedResults := make([]resultWithTime, 0)
		for id, result := range du.results {
			if result.Status == CompletedUpdate || result.Status == FailedUpdate || result.Status == CancelledUpdate {
				completedResults = append(completedResults, resultWithTime{id: id, result: result})
			}
		}

		// 删除最旧的记录
		if len(completedResults) > du.config.MaxHistorySize {
			deleteCount := len(completedResults) - du.config.MaxHistorySize
			for i := 0; i < deleteCount; i++ {
				id := completedResults[i].id
				delete(du.requests, id)
				delete(du.results, id)
			}

			du.logger.Info(fmt.Sprintf("清理了 %d 条历史记录", deleteCount))
		}
	}
}

// findHandler 查找处理器
func (du *dataUpdater) findHandler(request *UpdateRequest) UpdateHandler {
	du.handlersMu.RLock()
	defer du.handlersMu.RUnlock()

	for _, handler := range du.handlers {
		if handler.CanHandle(request) {
			return handler
		}
	}

	return nil
}

// updateResult 更新结果
func (du *dataUpdater) updateResult(requestID string, updater func(*UpdateResult)) {
	du.requestsMu.Lock()
	defer du.requestsMu.Unlock()

	if result, exists := du.results[requestID]; exists {
		updater(result)
	}
}

// updateStatistics 更新统计信息
func (du *dataUpdater) updateStatistics(updater func(*UpdateStatistics)) {
	du.statsMu.Lock()
	defer du.statsMu.Unlock()
	updater(du.statistics)
}

// checkUpdatePermissions 检查更新权限
func (du *dataUpdater) checkUpdatePermissions(ctx context.Context, request *UpdateRequest) error {
	if du.permissionManager == nil {
		return nil // 没有权限管理器，跳过检查
	}

	userContext := du.permissionManager.GetUserContext()

	switch request.Type {
	case SystemUpdate, ConfigUpdate:
		// 系统级更新需要系统级权限
		if userContext.PermissionLevel < permissions.SystemOnly {
			return fmt.Errorf("系统级更新需要系统级权限")
		}
	case UserUpdate, DataUpdate:
		// 用户级更新需要完全访问权限
		if userContext.PermissionLevel < permissions.FullAccess {
			return fmt.Errorf("用户级更新需要完全访问权限")
		}
	}

	return nil
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("update_%d", time.Now().UnixNano())
}

// 其他接口方法的实现将在下个文件中继续...
