package zkproof

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 通用优先级队列（优先级调度算法优化 - 阶段1）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现通用的优先级队列，支持多种优先级策略和动态优先级调整。
//
// 🏗️ **实现策略**：
// - 使用堆数据结构实现优先级队列
// - 支持多种优先级策略（交易类型、执行时间、等待时间、混合策略）
// - 支持优先级动态调整
// - 提供统计信息
//
// ⚠️ **注意**：
// - 优先级队列使用最小堆，优先级值越小优先级越高（或使用最大堆，优先级值越大优先级越高）
// - 我们使用最大堆，优先级值越大优先级越高
// - 优先级可以动态调整，调整后需要重新堆化
//
// ============================================================================

// PriorityItem 优先级队列元素接口
//
// 🎯 **接口定义**：
// 所有需要加入优先级队列的元素必须实现此接口
type PriorityItem interface {
	// GetPriority 获取当前优先级
	GetPriority() int
	
	// SetPriority 设置优先级（用于动态调整）
	SetPriority(priority int)
	
	// GetID 获取元素ID（用于唯一标识）
	GetID() string
	
	// GetCreatedAt 获取创建时间（用于公平性调度）
	GetCreatedAt() time.Time
}

// PriorityStrategy 优先级策略接口
//
// 🎯 **策略模式**：
// 不同的优先级计算策略实现此接口
type PriorityStrategy interface {
	// CalculatePriority 计算优先级
	//
	// 📋 **参数**：
	//   - basePriority: 基础优先级
	//   - item: 优先级队列元素
	//   - currentTime: 当前时间
	//
	// 📋 **返回值**：
	//   - int: 计算后的优先级
	CalculatePriority(basePriority int, item PriorityItem, currentTime time.Time) int
}

// TransactionTypeStrategy 基于交易类型的优先级策略
//
// 🎯 **策略说明**：
// 根据交易类型设置基础优先级
type TransactionTypeStrategy struct {
	// 交易类型到优先级的映射
	typePriorityMap map[string]int
}

// NewTransactionTypeStrategy 创建交易类型策略
func NewTransactionTypeStrategy() *TransactionTypeStrategy {
	return &TransactionTypeStrategy{
		typePriorityMap: map[string]int{
			"critical":   100, // 关键交易（如治理投票）
			"high":       80,  // 高优先级（如支付交易）
			"normal":     50,  // 普通交易
			"low":        20,  // 低优先级（如批量操作）
			"background": 10,  // 后台任务
		},
	}
}

// CalculatePriority 计算优先级（基于交易类型）
func (s *TransactionTypeStrategy) CalculatePriority(basePriority int, item PriorityItem, currentTime time.Time) int {
	// 如果item实现了TransactionType接口，使用交易类型优先级
	if typedItem, ok := item.(interface{ GetTransactionType() string }); ok {
		if priority, exists := s.typePriorityMap[typedItem.GetTransactionType()]; exists {
			return priority
		}
	}
	// 否则使用基础优先级
	return basePriority
}

// ExecutionTimeStrategy 基于执行时间的优先级策略
//
// 🎯 **策略说明**：
// 执行时间越长，优先级越低（避免长时间任务阻塞）
type ExecutionTimeStrategy struct {
	// 惩罚系数（每秒降低的优先级）
	penaltyPerSecond int
}

// NewExecutionTimeStrategy 创建执行时间策略
func NewExecutionTimeStrategy(penaltyPerSecond int) *ExecutionTimeStrategy {
	if penaltyPerSecond <= 0 {
		penaltyPerSecond = 5 // 默认每秒降低5个优先级
	}
	return &ExecutionTimeStrategy{
		penaltyPerSecond: penaltyPerSecond,
	}
}

// CalculatePriority 计算优先级（基于执行时间）
func (s *ExecutionTimeStrategy) CalculatePriority(basePriority int, item PriorityItem, currentTime time.Time) int {
	// 如果item实现了ExecutionDuration接口，应用执行时间惩罚
	if timedItem, ok := item.(interface{ GetExecutionDuration() time.Duration }); ok {
		duration := timedItem.GetExecutionDuration()
		penalty := int(duration.Seconds()) * s.penaltyPerSecond
		newPriority := basePriority - penalty
		if newPriority < 0 {
			newPriority = 0
		}
		return newPriority
	}
	return basePriority
}

// WaitTimeStrategy 基于等待时间的优先级策略
//
// 🎯 **策略说明**：
// 等待时间越长，优先级越高（避免饥饿）
type WaitTimeStrategy struct {
	// 加成系数（每秒增加的优先级）
	bonusPerSecond int
}

// NewWaitTimeStrategy 创建等待时间策略
func NewWaitTimeStrategy(bonusPerSecond int) *WaitTimeStrategy {
	if bonusPerSecond <= 0 {
		bonusPerSecond = 2 // 默认每秒增加2个优先级
	}
	return &WaitTimeStrategy{
		bonusPerSecond: bonusPerSecond,
	}
}

// CalculatePriority 计算优先级（基于等待时间）
func (s *WaitTimeStrategy) CalculatePriority(basePriority int, item PriorityItem, currentTime time.Time) int {
	waitTime := currentTime.Sub(item.GetCreatedAt())
	bonus := int(waitTime.Seconds()) * s.bonusPerSecond
	return basePriority + bonus
}

// MixedStrategy 混合优先级策略
//
// 🎯 **策略说明**：
// 综合多个因素计算优先级
type MixedStrategy struct {
	// 基础策略（交易类型）
	baseStrategy *TransactionTypeStrategy
	
	// 执行时间策略
	executionTimeStrategy *ExecutionTimeStrategy
	
	// 等待时间策略
	waitTimeStrategy *WaitTimeStrategy
	
	// 用户等级加成（可选）
	userLevelBonus map[string]int
}

// NewMixedStrategy 创建混合策略
func NewMixedStrategy() *MixedStrategy {
	return &MixedStrategy{
		baseStrategy:          NewTransactionTypeStrategy(),
		executionTimeStrategy: NewExecutionTimeStrategy(3), // 每秒降低3个优先级
		waitTimeStrategy:      NewWaitTimeStrategy(2),     // 每秒增加2个优先级
		userLevelBonus: map[string]int{
			"vip":    10, // VIP用户+10
			"premium": 5, // Premium用户+5
			"normal":  0, // 普通用户+0
		},
	}
}

// CalculatePriority 计算优先级（混合策略）
func (s *MixedStrategy) CalculatePriority(basePriority int, item PriorityItem, currentTime time.Time) int {
	// 1. 基础优先级（交易类型）
	priority := s.baseStrategy.CalculatePriority(basePriority, item, currentTime)
	
	// 2. 用户等级加成
	if userItem, ok := item.(interface{ GetUserLevel() string }); ok {
		if bonus, exists := s.userLevelBonus[userItem.GetUserLevel()]; exists {
			priority += bonus
		}
	}
	
	// 3. 执行时间惩罚
	priority = s.executionTimeStrategy.CalculatePriority(priority, item, currentTime)
	
	// 4. 等待时间加成
	priority = s.waitTimeStrategy.CalculatePriority(priority, item, currentTime)
	
	return priority
}

// PriorityQueue 通用优先级队列
//
// 🎯 **核心职责**：
// - 管理优先级队列元素
// - 支持优先级动态调整
// - 支持多种优先级策略
// - 提供统计信息
type PriorityQueue struct {
	// 优先级队列（使用heap实现）
	queue *priorityQueueImpl
	
	// 元素映射（ID -> item）
	items map[string]PriorityItem
	
	// 优先级策略
	strategy PriorityStrategy
	
	// 同步控制
	mutex sync.RWMutex
	
	// 日志记录器
	logger log.Logger
	
	// 统计信息
	stats *PriorityQueueStats
}

// PriorityQueueStats 优先级队列统计信息
type PriorityQueueStats struct {
	TotalEnqueued int64 // 总入队数
	TotalDequeued int64 // 总出队数
	CurrentSize   int   // 当前队列大小
	MaxSize       int   // 最大队列大小
	PriorityAdjustments int64 // 优先级调整次数
}

// NewPriorityQueue 创建优先级队列
//
// 📋 **参数**：
//   - strategy: 优先级策略（如果为nil，使用默认策略）
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *PriorityQueue: 优先级队列实例
func NewPriorityQueue(strategy PriorityStrategy, logger log.Logger) *PriorityQueue {
	if strategy == nil {
		strategy = NewMixedStrategy() // 默认使用混合策略
	}
	
	return &PriorityQueue{
		queue:    newPriorityQueueImpl(),
		items:    make(map[string]PriorityItem),
		strategy: strategy,
		logger:   logger,
		stats:    &PriorityQueueStats{},
	}
}

// Enqueue 入队元素
//
// 📋 **参数**：
//   - item: 优先级队列元素
//
// 📋 **返回值**：
//   - error: 入队错误
func (pq *PriorityQueue) Enqueue(item PriorityItem) error {
	if item == nil {
		return fmt.Errorf("元素不能为空")
	}
	
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	// 检查元素是否已存在
	if _, exists := pq.items[item.GetID()]; exists {
		return fmt.Errorf("元素已存在: %s", item.GetID())
	}
	
	// 计算优先级
	currentTime := time.Now()
	priority := pq.strategy.CalculatePriority(item.GetPriority(), item, currentTime)
	item.SetPriority(priority)
	
	// 添加到队列和映射
	heap.Push(pq.queue, item)
	pq.items[item.GetID()] = item
	
	// 更新统计信息
	pq.stats.TotalEnqueued++
	pq.stats.CurrentSize = pq.queue.Len()
	if pq.stats.CurrentSize > pq.stats.MaxSize {
		pq.stats.MaxSize = pq.stats.CurrentSize
	}
	
	return nil
}

// Dequeue 出队元素（优先级最高的）
//
// 📋 **返回值**：
//   - PriorityItem: 优先级最高的元素（如果队列为空返回nil）
func (pq *PriorityQueue) Dequeue() PriorityItem {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	if pq.queue.Len() == 0 {
		return nil
	}
	
	item := heap.Pop(pq.queue).(PriorityItem)
	delete(pq.items, item.GetID())
	
	// 更新统计信息
	pq.stats.TotalDequeued++
	pq.stats.CurrentSize = pq.queue.Len()
	
	return item
}

// Peek 查看优先级最高的元素（不移除）
//
// 📋 **返回值**：
//   - PriorityItem: 优先级最高的元素（如果队列为空返回nil）
func (pq *PriorityQueue) Peek() PriorityItem {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	
	if pq.queue.Len() == 0 {
		return nil
	}
	
	return (*pq.queue)[0]
}

// Get 获取指定ID的元素
//
// 📋 **参数**：
//   - id: 元素ID
//
// 📋 **返回值**：
//   - PriorityItem: 元素（如果不存在返回nil）
func (pq *PriorityQueue) Get(id string) PriorityItem {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	
	return pq.items[id]
}

// Remove 移除指定ID的元素
//
// 📋 **参数**：
//   - id: 元素ID
//
// 📋 **返回值**：
//   - error: 移除错误
func (pq *PriorityQueue) Remove(id string) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	if _, exists := pq.items[id]; !exists {
		return fmt.Errorf("元素不存在: %s", id)
	}
	
	// 从队列中移除（需要找到索引）
	for i, v := range *pq.queue {
		if v.GetID() == id {
			heap.Remove(pq.queue, i)
			break
		}
	}
	
	delete(pq.items, id)
	pq.stats.CurrentSize = pq.queue.Len()
	
	return nil
}

// UpdatePriority 更新元素优先级
//
// 📋 **参数**：
//   - id: 元素ID
//   - newPriority: 新优先级
//
// 📋 **返回值**：
//   - error: 更新错误
func (pq *PriorityQueue) UpdatePriority(id string, newPriority int) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	item, exists := pq.items[id]
	if !exists {
		return fmt.Errorf("元素不存在: %s", id)
	}
	
	// 更新优先级
	item.SetPriority(newPriority)
	
	// 重新堆化（找到元素并调用Fix）
	for i, v := range *pq.queue {
		if v.GetID() == id {
			heap.Fix(pq.queue, i)
			break
		}
	}
	
	pq.stats.PriorityAdjustments++
	return nil
}

// AdjustPriority 根据策略动态调整优先级
//
// 📋 **参数**：
//   - id: 元素ID
//
// 📋 **返回值**：
//   - error: 调整错误
func (pq *PriorityQueue) AdjustPriority(id string) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	item, exists := pq.items[id]
	if !exists {
		return fmt.Errorf("元素不存在: %s", id)
	}
	
	// 根据策略重新计算优先级
	currentTime := time.Now()
	newPriority := pq.strategy.CalculatePriority(item.GetPriority(), item, currentTime)
	
	// 如果优先级发生变化，更新并重新堆化
	if newPriority != item.GetPriority() {
		item.SetPriority(newPriority)
		
		// 重新堆化
		for i, v := range *pq.queue {
			if v.GetID() == id {
				heap.Fix(pq.queue, i)
				break
			}
		}
		
		pq.stats.PriorityAdjustments++
	}
	
	return nil
}

// AdjustAllPriorities 调整所有元素的优先级
//
// 🎯 **用途**：
// 定期调用此方法，根据策略动态调整所有元素的优先级
func (pq *PriorityQueue) AdjustAllPriorities() {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	currentTime := time.Now()
	
	// 遍历所有元素，调整优先级
	for _, item := range pq.items {
		newPriority := pq.strategy.CalculatePriority(item.GetPriority(), item, currentTime)
		if newPriority != item.GetPriority() {
			item.SetPriority(newPriority)
			pq.stats.PriorityAdjustments++
		}
	}
	
	// 重新堆化整个队列
	heap.Init(pq.queue)
}

// Len 返回队列长度
func (pq *PriorityQueue) Len() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	
	return pq.queue.Len()
}

// IsEmpty 检查队列是否为空
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// GetStats 获取统计信息
func (pq *PriorityQueue) GetStats() *PriorityQueueStats {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	
	// 返回统计信息的副本
	stats := *pq.stats
	stats.CurrentSize = pq.queue.Len()
	return &stats
}

// SetStrategy 设置优先级策略
func (pq *PriorityQueue) SetStrategy(strategy PriorityStrategy) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	
	pq.strategy = strategy
	
	// 重新计算所有元素的优先级（已经在锁内，直接实现逻辑）
	currentTime := time.Now()
	for _, item := range pq.items {
		newPriority := pq.strategy.CalculatePriority(item.GetPriority(), item, currentTime)
		if newPriority != item.GetPriority() {
			item.SetPriority(newPriority)
			pq.stats.PriorityAdjustments++
		}
	}
	
	// 重新堆化整个队列
	heap.Init(pq.queue)
}

// ============================================================================
// 优先级队列实现（heap.Interface）
// ============================================================================

// priorityQueueImpl 优先级队列实现（使用heap）
type priorityQueueImpl []PriorityItem

// newPriorityQueueImpl 创建优先级队列实现
func newPriorityQueueImpl() *priorityQueueImpl {
	pq := make(priorityQueueImpl, 0)
	return &pq
}

// Len 返回队列长度
func (pq priorityQueueImpl) Len() int {
	return len(pq)
}

// Less 比较函数（优先级高的在前，使用最大堆）
func (pq priorityQueueImpl) Less(i, j int) bool {
	// 优先级高的在前（优先级值越大优先级越高）
	if pq[i].GetPriority() != pq[j].GetPriority() {
		return pq[i].GetPriority() > pq[j].GetPriority()
	}
	// 优先级相同，创建时间早的在前（FIFO）
	return pq[i].GetCreatedAt().Before(pq[j].GetCreatedAt())
}

// Swap 交换元素
func (pq priorityQueueImpl) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push 添加元素
func (pq *priorityQueueImpl) Push(x interface{}) {
	item := x.(PriorityItem)
	*pq = append(*pq, item)
}

// Pop 移除并返回优先级最高的元素
func (pq *priorityQueueImpl) Pop() interface{} {
	old := *pq
	n := len(old)
	*pq = old[0 : n-1]
	return old[n-1]
}

