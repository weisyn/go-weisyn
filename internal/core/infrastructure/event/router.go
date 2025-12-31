// Package event 事件智能路由器实现
package event

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// RouteStrategy 路由策略类型
type RouteStrategy int

const (
	// RouteDirect 直接路由：点对点通信
	RouteDirect RouteStrategy = iota
	// RouteBroadcast 广播路由：发送给所有订阅者
	RouteBroadcast
	// RouteRoundRobin 轮询路由：负载均衡分发
	RouteRoundRobin
	// RoutePriority 优先级路由：按优先级排序处理
	RoutePriority
	// RouteFilter 过滤路由：基于条件过滤
	RouteFilter
)

// Priority 事件优先级
type Priority int

const (
	PriorityLow      Priority = 0 // 低优先级：统计、监控类事件
	PriorityNormal   Priority = 1 // 普通优先级：常规业务事件
	PriorityHigh     Priority = 2 // 高优先级：关键状态变更
	PriorityCritical Priority = 3 // 紧急优先级：系统级紧急事件
)

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	ID           string                    // 订阅ID
	EventType    string                    // 事件类型
	Handler      interface{}               // 事件处理器
	Priority     Priority                  // 优先级
	Component    string                    // 订阅组件
	CreatedAt    time.Time                 // 创建时间
	Active       bool                      // 是否活跃
	TriggerCount atomic.Uint64             // 触发次数
	LastTrigger  atomic.Pointer[time.Time] // 最后触发时间
	Filter       EventFilter               // 事件过滤器（可选）
}

// EventFilter 事件过滤器接口
type EventFilter interface {
	// Match 检查事件是否匹配过滤条件
	Match(eventType string, data interface{}) bool
	// GetFilterInfo 获取过滤器信息
	GetFilterInfo() *FilterInfo
}

// FilterInfo 过滤器信息
type FilterInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Priority    int                    `json:"priority"`
	Conditions  map[string]interface{} `json:"conditions"`
	Description string                 `json:"description"`
}

// EventRouter 智能事件路由器
// 负责根据不同策略将事件路由到相应的订阅者
type EventRouter struct {
	mu            sync.RWMutex
	subscriptions map[string][]*SubscriptionInfo // eventType -> subscriptions
	strategies    map[string]RouteStrategy       // eventType -> strategy
	filters       []EventFilter                  // 全局过滤器

	// 轮询计数器（用于轮询策略）
	roundRobinCounters map[string]*atomic.Uint64

	// 优先级队列
	priorityQueues map[Priority]chan *RouteTask

	// 路由器状态
	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 统计信息
	stats  *RouterStats
	logger log.Logger
}

// RouteTask 路由任务
type RouteTask struct {
	EventType string
	Data      interface{}
	Timestamp time.Time
	Priority  Priority
	Source    string
}

// RouterStats 路由器统计信息
type RouterStats struct {
	TotalRouted   atomic.Uint64
	SuccessRouted atomic.Uint64
	FailedRouted  atomic.Uint64

	RoutesByStrategy map[RouteStrategy]*atomic.Uint64
	RoutesByPriority map[Priority]*atomic.Uint64

	AvgRouteTime  atomic.Pointer[time.Duration]
	LastRouteTime atomic.Pointer[time.Time]
}

// NewEventRouter 创建新的事件路由器
func NewEventRouter(logger log.Logger) *EventRouter {
	var componentLogger log.Logger
	if logger != nil {
		componentLogger = logger.With("component", "event_router")
	}

	router := &EventRouter{
		subscriptions:      make(map[string][]*SubscriptionInfo),
		strategies:         make(map[string]RouteStrategy),
		filters:            make([]EventFilter, 0),
		roundRobinCounters: make(map[string]*atomic.Uint64),
		priorityQueues:     make(map[Priority]chan *RouteTask),
		stats:              newRouterStats(),
		logger:             componentLogger,
	}

	// 初始化优先级队列
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		router.priorityQueues[priority] = make(chan *RouteTask, 1000)
	}

	return router
}

// newRouterStats 创建路由器统计信息
func newRouterStats() *RouterStats {
	stats := &RouterStats{
		RoutesByStrategy: make(map[RouteStrategy]*atomic.Uint64),
		RoutesByPriority: make(map[Priority]*atomic.Uint64),
	}

	// 初始化策略统计
	for strategy := RouteDirect; strategy <= RouteFilter; strategy++ {
		stats.RoutesByStrategy[strategy] = &atomic.Uint64{}
	}

	// 初始化优先级统计
	for priority := PriorityLow; priority <= PriorityCritical; priority++ {
		stats.RoutesByPriority[priority] = &atomic.Uint64{}
	}

	return stats
}

// Start 启动路由器
func (r *EventRouter) Start(ctx context.Context) error {
	if r.running.Load() {
		return fmt.Errorf("event router already running")
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.running.Store(true)

	// 启动优先级队列处理器
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		go r.processPriorityQueue(priority)
	}

	r.logger.Info("事件路由器已启动")
	return nil
}

// Stop 停止路由器
func (r *EventRouter) Stop() error {
	if !r.running.Load() {
		return fmt.Errorf("event router not running")
	}

	r.running.Store(false)
	if r.cancel != nil {
		r.cancel()
	}

	r.logger.Info("事件路由器已停止")
	return nil
}

// AddSubscription 添加事件订阅
func (r *EventRouter) AddSubscription(eventType string, handler interface{}, options ...SubscriptionOption) (string, error) {
	if eventType == "" {
		return "", fmt.Errorf("event type cannot be empty")
	}
	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 创建订阅信息
	subscription := &SubscriptionInfo{
		ID:        generateSubscriptionID(),
		EventType: eventType,
		Handler:   handler,
		Priority:  PriorityNormal, // 默认优先级
		CreatedAt: time.Now(),
		Active:    true,
	}

	// 应用订阅选项
	for _, option := range options {
		option(subscription)
	}

	// 添加到订阅列表
	if r.subscriptions[eventType] == nil {
		r.subscriptions[eventType] = make([]*SubscriptionInfo, 0)
	}
	r.subscriptions[eventType] = append(r.subscriptions[eventType], subscription)

	// 初始化轮询计数器
	if r.roundRobinCounters[eventType] == nil {
		r.roundRobinCounters[eventType] = &atomic.Uint64{}
	}

	r.logger.Debugf("添加事件订阅: type=%s, id=%s, component=%s",
		eventType, subscription.ID, subscription.Component)

	return subscription.ID, nil
}

// RemoveSubscription 移除事件订阅
func (r *EventRouter) RemoveSubscription(subscriptionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 查找并移除订阅
	for eventType, subscriptions := range r.subscriptions {
		for i, sub := range subscriptions {
			if sub.ID == subscriptionID {
				// 标记为不活跃
				sub.Active = false
				// 从列表中移除
				r.subscriptions[eventType] = append(subscriptions[:i], subscriptions[i+1:]...)

				r.logger.Debugf("移除事件订阅: type=%s, id=%s", eventType, subscriptionID)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription %s not found", subscriptionID)
}

// SetRouteStrategy 设置事件类型的路由策略
func (r *EventRouter) SetRouteStrategy(eventType string, strategy RouteStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.strategies[eventType] = strategy
	r.logger.Debugf("设置路由策略: type=%s, strategy=%v", eventType, strategy)
}

// GetRouteStrategy 获取事件类型的路由策略
func (r *EventRouter) GetRouteStrategy(eventType string) RouteStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if strategy, exists := r.strategies[eventType]; exists {
		return strategy
	}
	return RouteBroadcast // 默认广播策略
}

// RouteEvent 路由事件到订阅者
func (r *EventRouter) RouteEvent(eventType string, data interface{}, priority Priority, source string) error {
	if !r.running.Load() {
		return fmt.Errorf("event router not running")
	}

	startTime := time.Now()

	// 创建路由任务
	task := &RouteTask{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  priority,
		Source:    source,
	}

	// 根据优先级选择处理方式
	if priority == PriorityCritical {
		// 紧急事件直接处理
		return r.processRouteTask(task)
	}

	// 其他优先级事件放入队列
	select {
	case r.priorityQueues[priority] <- task:
		// 更新统计
		r.updateRouteStats(startTime, true)
		return nil
	case <-r.ctx.Done():
		return fmt.Errorf("event router stopped")
	default:
		// 队列满，记录错误
		r.updateRouteStats(startTime, false)
		return fmt.Errorf("priority queue full for priority %v", priority)
	}
}

// processRouteTask 处理路由任务
func (r *EventRouter) processRouteTask(task *RouteTask) error {
	r.mu.RLock()
	subscriptions := r.getActiveSubscriptions(task.EventType)
	// ⚠️ 注意：这里已经持有 r.mu.RLock()。
	// 不能再调用 GetRouteStrategy()（其内部会再次尝试 RLock），否则在有 writer 等待时会触发 RWMutex 的 writer-preference，
	// 导致“读锁重入阻塞”进而卡死（测试中表现为超时）。
	strategy, ok := r.strategies[task.EventType]
	if !ok {
		strategy = RouteBroadcast
	}
	r.mu.RUnlock()

	if len(subscriptions) == 0 {
		r.logger.Debugf("没有找到事件订阅者: type=%s", task.EventType)
		return nil
	}

	// 应用全局过滤器
	filteredSubscriptions := r.applyFilters(task.EventType, task.Data, subscriptions)

	// 根据策略路由事件
	switch strategy {
	case RouteDirect:
		return r.routeDirect(task, filteredSubscriptions)
	case RouteBroadcast:
		return r.routeBroadcast(task, filteredSubscriptions)
	case RouteRoundRobin:
		return r.routeRoundRobin(task, filteredSubscriptions)
	case RoutePriority:
		return r.routePriority(task, filteredSubscriptions)
	case RouteFilter:
		return r.routeFilter(task, filteredSubscriptions)
	default:
		return r.routeBroadcast(task, filteredSubscriptions)
	}
}

// getActiveSubscriptions 获取活跃的订阅者
func (r *EventRouter) getActiveSubscriptions(eventType string) []*SubscriptionInfo {
	subscriptions := r.subscriptions[eventType]
	active := make([]*SubscriptionInfo, 0, len(subscriptions))

	for _, sub := range subscriptions {
		if sub.Active {
			active = append(active, sub)
		}
	}

	return active
}

// applyFilters 应用过滤器
func (r *EventRouter) applyFilters(eventType string, data interface{}, subscriptions []*SubscriptionInfo) []*SubscriptionInfo {
	if len(r.filters) == 0 {
		return subscriptions
	}

	filtered := make([]*SubscriptionInfo, 0, len(subscriptions))

	for _, sub := range subscriptions {
		// 检查全局过滤器
		passGlobal := true
		for _, filter := range r.filters {
			if !filter.Match(eventType, data) {
				passGlobal = false
				break
			}
		}

		// 检查订阅级过滤器
		passLocal := true
		if sub.Filter != nil {
			passLocal = sub.Filter.Match(eventType, data)
		}

		if passGlobal && passLocal {
			filtered = append(filtered, sub)
		}
	}

	return filtered
}

// routeDirect 直接路由（点对点）
func (r *EventRouter) routeDirect(task *RouteTask, subscriptions []*SubscriptionInfo) error {
	if len(subscriptions) == 0 {
		return nil
	}

	// 选择第一个订阅者
	return r.invokeHandler(subscriptions[0], task)
}

// routeBroadcast 广播路由
func (r *EventRouter) routeBroadcast(task *RouteTask, subscriptions []*SubscriptionInfo) error {
	var lastErr error

	for _, sub := range subscriptions {
		if err := r.invokeHandler(sub, task); err != nil {
			lastErr = err
			r.logger.Errorf("广播路由失败: subscription=%s, error=%v", sub.ID, err)
		}
	}

	return lastErr
}

// routeRoundRobin 轮询路由
func (r *EventRouter) routeRoundRobin(task *RouteTask, subscriptions []*SubscriptionInfo) error {
	if len(subscriptions) == 0 {
		return nil
	}

	counter := r.roundRobinCounters[task.EventType]
	index := counter.Add(1) % uint64(len(subscriptions))

	return r.invokeHandler(subscriptions[index], task)
}

// routePriority 优先级路由
func (r *EventRouter) routePriority(task *RouteTask, subscriptions []*SubscriptionInfo) error {
	if len(subscriptions) == 0 {
		return nil
	}

	// 按优先级排序订阅者
	prioritySubs := make(map[Priority][]*SubscriptionInfo)
	for _, sub := range subscriptions {
		prioritySubs[sub.Priority] = append(prioritySubs[sub.Priority], sub)
	}

	// 从高优先级到低优先级处理
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		if subs := prioritySubs[priority]; len(subs) > 0 {
			// 对同优先级的订阅者使用广播
			for _, sub := range subs {
				if err := r.invokeHandler(sub, task); err != nil {
					r.logger.Errorf("优先级路由失败: subscription=%s, priority=%v, error=%v",
						sub.ID, priority, err)
				}
			}
			break // 只处理最高优先级的订阅者
		}
	}

	return nil
}

// routeFilter 过滤路由
func (r *EventRouter) routeFilter(task *RouteTask, subscriptions []*SubscriptionInfo) error {
	// 过滤路由实际上在applyFilters中已经处理
	// 这里只需要广播给通过过滤的订阅者
	return r.routeBroadcast(task, subscriptions)
}

// invokeHandler 调用事件处理器
func (r *EventRouter) invokeHandler(subscription *SubscriptionInfo, task *RouteTask) error {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logger.Errorf("事件处理器panic: subscription=%s, error=%v", subscription.ID, recovered)
		}
	}()

	if subscription == nil || task == nil {
		return fmt.Errorf("invalid invoke args: subscription/task is nil")
	}
	if !subscription.Active {
		return nil
	}
	// 订阅级过滤器（双保险：applyFilters 是全局+订阅过滤，这里再检查一次避免绕过）
	if subscription.Filter != nil && !subscription.Filter.Match(task.EventType, task.Data) {
		return nil
	}

	// 更新订阅统计
	subscription.TriggerCount.Add(1)
	now := time.Now()
	subscription.LastTrigger.Store(&now)

	h := subscription.Handler
	// 1) 推荐形态：对象方法
	switch hh := h.(type) {
	case interface {
		Handle(string, interface{}) error
	}:
		return hh.Handle(task.EventType, task.Data)
	case interface{ Handle(string, interface{}) }:
		hh.Handle(task.EventType, task.Data)
		return nil
	case func(string, interface{}) error:
		return hh(task.EventType, task.Data)
	case func(string, interface{}):
		hh(task.EventType, task.Data)
		return nil
	case func(*RouteTask) error:
		return hh(task)
	case func(*RouteTask):
		hh(task)
		return nil
	}

	// 2) 兜底：反射调用（支持多种签名）
	v := reflect.ValueOf(h)
	if v.Kind() != reflect.Func {
		return fmt.Errorf("unsupported handler type: %T", h)
	}

	// 允许的签名（按优先级）：
	// - func(ctx context.Context, eventType string, data any) error
	// - func(ctx context.Context, task *RouteTask) error
	// - func(eventType string, data any) error
	// - func(task *RouteTask) error
	// - 上述不返回 error 的版本也允许
	t := v.Type()
	var args []reflect.Value

	// 先尝试带 ctx
	if t.NumIn() >= 1 && t.In(0) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		// ctx 来源：router ctx（若未启动，退化为 Background）
		c := r.ctx
		if c == nil {
			c = context.Background()
		}
		args = append(args, reflect.ValueOf(c))
	}

	// 填剩余参数
	switch t.NumIn() - len(args) {
	case 1:
		// 期望 task
		if t.In(len(args)) == reflect.TypeOf(&RouteTask{}) {
			args = append(args, reflect.ValueOf(task))
		} else {
			return fmt.Errorf("unsupported handler signature (want *RouteTask): %T", h)
		}
	case 2:
		// 期望 eventType,data
		if t.In(len(args)).Kind() == reflect.String {
			args = append(args, reflect.ValueOf(task.EventType))
		} else {
			return fmt.Errorf("unsupported handler signature (want string,event data): %T", h)
		}
		// data 任意类型：只要可赋值
		dataV := reflect.ValueOf(task.Data)
		if !dataV.IsValid() {
			dataV = reflect.Zero(t.In(len(args)))
		}
		if dataV.Type().AssignableTo(t.In(len(args))) {
			args = append(args, dataV)
		} else if dataV.Type().ConvertibleTo(t.In(len(args))) {
			args = append(args, dataV.Convert(t.In(len(args))))
		} else if t.In(len(args)).Kind() == reflect.Interface {
			args = append(args, dataV)
		} else {
			return fmt.Errorf("handler arg type mismatch: got=%v want=%v", dataV.Type(), t.In(len(args)))
		}
	default:
		return fmt.Errorf("unsupported handler signature: %T (numIn=%d)", h, t.NumIn())
	}

	outs := v.Call(args)
	if len(outs) == 0 {
		return nil
	}
	// 允许返回 (error) 或 (any,error)
	last := outs[len(outs)-1]
	if last.IsValid() && !last.IsZero() {
		if err, ok := last.Interface().(error); ok {
			return err
		}
	}

	return nil
}

// processPriorityQueue 处理优先级队列
func (r *EventRouter) processPriorityQueue(priority Priority) {
	queue := r.priorityQueues[priority]

	for {
		select {
		case task := <-queue:
			if task == nil {
				return // 队列已关闭
			}

			if err := r.processRouteTask(task); err != nil {
				r.logger.Errorf("处理优先级队列任务失败: priority=%v, error=%v", priority, err)
			}

		case <-r.ctx.Done():
			return
		}
	}
}

// updateRouteStats 更新路由统计
func (r *EventRouter) updateRouteStats(startTime time.Time, success bool) {
	duration := time.Since(startTime)

	r.stats.TotalRouted.Add(1)
	if success {
		r.stats.SuccessRouted.Add(1)
	} else {
		r.stats.FailedRouted.Add(1)
	}

	r.stats.AvgRouteTime.Store(&duration)
	now := time.Now()
	r.stats.LastRouteTime.Store(&now)
}

// GetSubscriptions 获取指定事件类型的所有订阅
func (r *EventRouter) GetSubscriptions(eventType string) []*SubscriptionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subscriptions := r.subscriptions[eventType]
	result := make([]*SubscriptionInfo, 0, len(subscriptions))
	for _, sub := range subscriptions {
		if sub == nil {
			continue
		}
		copied := *sub
		// 深拷贝原子字段，避免返回的副本修改影响内部状态
		copied.TriggerCount.Store(sub.TriggerCount.Load())
		if last := sub.LastTrigger.Load(); last != nil {
			t := *last
			copied.LastTrigger.Store(&t)
		} else {
			copied.LastTrigger.Store(nil)
		}
		result = append(result, &copied)
	}

	return result
}

// GetAllSubscriptions 获取所有订阅信息
func (r *EventRouter) GetAllSubscriptions() map[string][]*SubscriptionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*SubscriptionInfo)
	for eventType, subscriptions := range r.subscriptions {
		copiedSubs := make([]*SubscriptionInfo, 0, len(subscriptions))
		for _, sub := range subscriptions {
			if sub == nil {
				continue
			}
			copied := *sub
			copied.TriggerCount.Store(sub.TriggerCount.Load())
			if last := sub.LastTrigger.Load(); last != nil {
				t := *last
				copied.LastTrigger.Store(&t)
			} else {
				copied.LastTrigger.Store(nil)
			}
			copiedSubs = append(copiedSubs, &copied)
		}
		result[eventType] = copiedSubs
	}

	return result
}

// GetStatistics 获取路由器统计信息
func (r *EventRouter) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"total_routed":   r.stats.TotalRouted.Load(),
		"success_routed": r.stats.SuccessRouted.Load(),
		"failed_routed":  r.stats.FailedRouted.Load(),
		"running":        r.running.Load(),
	}

	// 策略统计
	strategyStats := make(map[string]uint64)
	for strategy, counter := range r.stats.RoutesByStrategy {
		strategyStats[fmt.Sprintf("strategy_%d", strategy)] = counter.Load()
	}
	stats["routes_by_strategy"] = strategyStats

	// 优先级统计
	priorityStats := make(map[string]uint64)
	for priority, counter := range r.stats.RoutesByPriority {
		priorityStats[fmt.Sprintf("priority_%d", priority)] = counter.Load()
	}
	stats["routes_by_priority"] = priorityStats

	// 队列状态
	queueStats := make(map[string]int)
	for priority, queue := range r.priorityQueues {
		queueStats[fmt.Sprintf("queue_%d", priority)] = len(queue)
	}
	stats["queue_lengths"] = queueStats

	if avgTime := r.stats.AvgRouteTime.Load(); avgTime != nil {
		stats["avg_route_time"] = avgTime.String()
	}

	if lastTime := r.stats.LastRouteTime.Load(); lastTime != nil {
		stats["last_route_time"] = *lastTime
	}

	return stats
}

// AddFilter 添加全局事件过滤器
func (r *EventRouter) AddFilter(filter EventFilter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.filters = append(r.filters, filter)
	r.logger.Debugf("添加全局事件过滤器: filter=%s", filter.GetFilterInfo().Name)
}

// RemoveFilter 移除全局事件过滤器
func (r *EventRouter) RemoveFilter(filterID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, filter := range r.filters {
		if filter.GetFilterInfo().ID == filterID {
			r.filters = append(r.filters[:i], r.filters[i+1:]...)
			r.logger.Debugf("移除全局事件过滤器: filter=%s", filterID)
			return
		}
	}
}

// SubscriptionOption 订阅选项函数
type SubscriptionOption func(*SubscriptionInfo)

// WithPriority 设置订阅优先级
func WithPriority(priority Priority) SubscriptionOption {
	return func(s *SubscriptionInfo) {
		s.Priority = priority
	}
}

// WithComponent 设置订阅组件
func WithComponent(component string) SubscriptionOption {
	return func(s *SubscriptionInfo) {
		s.Component = component
	}
}

// WithFilter 设置订阅过滤器
func WithFilter(filter EventFilter) SubscriptionOption {
	return func(s *SubscriptionInfo) {
		s.Filter = filter
	}
}

// generateSubscriptionID 生成订阅ID
func generateSubscriptionID() string {
	return fmt.Sprintf("sub_%d", time.Now().UnixNano())
}

// IsRunning 检查路由器是否运行中
func (r *EventRouter) IsRunning() bool {
	return r.running.Load()
}
