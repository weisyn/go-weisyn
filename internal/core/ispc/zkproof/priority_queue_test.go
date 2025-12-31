package zkproof

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// priority_queue.go 测试
// ============================================================================

// mockPriorityItem Mock的优先级队列元素
type mockPriorityItem struct {
	id        string
	priority  int
	createdAt time.Time
}

func (m *mockPriorityItem) GetPriority() int {
	return m.priority
}

func (m *mockPriorityItem) SetPriority(priority int) {
	m.priority = priority
}

func (m *mockPriorityItem) GetID() string {
	return m.id
}

func (m *mockPriorityItem) GetCreatedAt() time.Time {
	return m.createdAt
}

// TestNewPriorityQueue 测试创建优先级队列
func TestNewPriorityQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	strategy := NewMixedStrategy()
	
	queue := NewPriorityQueue(strategy, logger)
	require.NotNil(t, queue)
	require.NotNil(t, queue.queue)
	require.NotNil(t, queue.items)
	require.Equal(t, strategy, queue.strategy)
}

// TestNewPriorityQueue_DefaultStrategy 测试使用默认策略创建优先级队列
func TestNewPriorityQueue_DefaultStrategy(t *testing.T) {
	logger := testutil.NewTestLogger()
	
	queue := NewPriorityQueue(nil, logger)
	require.NotNil(t, queue)
	require.NotNil(t, queue.strategy)
}

// TestPriorityQueue_Enqueue 测试入队
func TestPriorityQueue_Enqueue(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	err := queue.Enqueue(item)
	require.NoError(t, err)
	require.Equal(t, 1, queue.Len())
	require.False(t, queue.IsEmpty())
}

// TestPriorityQueue_Enqueue_Duplicate 测试重复入队
func TestPriorityQueue_Enqueue_Duplicate(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	err := queue.Enqueue(item)
	require.NoError(t, err)
	
	// 尝试再次入队
	err = queue.Enqueue(item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "元素已存在")
}

// TestPriorityQueue_Enqueue_Nil 测试入队nil元素
func TestPriorityQueue_Enqueue_Nil(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	err := queue.Enqueue(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "元素不能为空")
}

// TestPriorityQueue_Dequeue 测试出队
func TestPriorityQueue_Dequeue(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item1)
	queue.Enqueue(item2)
	
	// 优先级高的先出队
	dequeued := queue.Dequeue()
	require.NotNil(t, dequeued)
	require.Equal(t, "item2", dequeued.GetID()) // 优先级20 > 10
	
	require.Equal(t, 1, queue.Len())
}

// TestPriorityQueue_Dequeue_Empty 测试空队列出队
func TestPriorityQueue_Dequeue_Empty(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := queue.Dequeue()
	require.Nil(t, item)
}

// TestPriorityQueue_Peek 测试查看队首元素
func TestPriorityQueue_Peek(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item1)
	queue.Enqueue(item2)
	
	peeked := queue.Peek()
	require.NotNil(t, peeked)
	require.Equal(t, "item2", peeked.GetID())
	
	// Peek不应该移除元素
	require.Equal(t, 2, queue.Len())
}

// TestPriorityQueue_Peek_Empty 测试空队列查看
func TestPriorityQueue_Peek_Empty(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := queue.Peek()
	require.Nil(t, item)
}

// TestPriorityQueue_Get 测试获取指定ID的元素
func TestPriorityQueue_Get(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item)
	
	retrieved := queue.Get("item1")
	require.NotNil(t, retrieved)
	require.Equal(t, "item1", retrieved.GetID())
	
	// 不存在的ID
	retrieved = queue.Get("nonexistent")
	require.Nil(t, retrieved)
}

// TestPriorityQueue_Remove 测试移除元素
func TestPriorityQueue_Remove(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item)
	require.Equal(t, 1, queue.Len())
	
	err := queue.Remove("item1")
	require.NoError(t, err)
	require.Equal(t, 0, queue.Len())
}

// TestPriorityQueue_Remove_NotExists 测试移除不存在的元素
func TestPriorityQueue_Remove_NotExists(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	err := queue.Remove("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "元素不存在")
}

// TestPriorityQueue_UpdatePriority 测试更新优先级
func TestPriorityQueue_UpdatePriority(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item)
	
	err := queue.UpdatePriority("item1", 20)
	require.NoError(t, err)
	require.Equal(t, 20, item.GetPriority())
}

// TestPriorityQueue_AdjustPriority 测试动态调整优先级
func TestPriorityQueue_AdjustPriority(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now().Add(-time.Minute), // 1分钟前创建
	}
	
	queue.Enqueue(item)
	
	err := queue.AdjustPriority("item1")
	require.NoError(t, err)
	// 等待时间策略会增加优先级
	require.Greater(t, item.GetPriority(), 10)
}

// TestPriorityQueue_AdjustAllPriorities 测试调整所有元素优先级
func TestPriorityQueue_AdjustAllPriorities(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now().Add(-time.Minute),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now().Add(-2 * time.Minute),
	}
	
	queue.Enqueue(item1)
	queue.Enqueue(item2)
	
	queue.AdjustAllPriorities()
	
	// 等待时间更长的应该优先级更高
	require.Greater(t, item2.GetPriority(), item1.GetPriority())
}

// TestPriorityQueue_SetStrategy 测试设置优先级策略
func TestPriorityQueue_SetStrategy(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	newStrategy := NewTransactionTypeStrategy()
	queue.SetStrategy(newStrategy)
	
	require.Equal(t, newStrategy, queue.strategy)
}

// TestPriorityQueue_GetStats 测试获取统计信息
func TestPriorityQueue_GetStats(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now(),
	}
	
	queue.Enqueue(item1)
	queue.Enqueue(item2)
	
	stats := queue.GetStats()
	require.NotNil(t, stats)
	require.Equal(t, int64(2), stats.TotalEnqueued)
	require.Equal(t, 2, stats.CurrentSize)
	
	queue.Dequeue()
	stats = queue.GetStats()
	require.Equal(t, int64(1), stats.TotalDequeued)
	require.Equal(t, 1, stats.CurrentSize)
}

// TestPriorityQueue_FIFO 测试相同优先级FIFO
func TestPriorityQueue_FIFO(t *testing.T) {
	queue := NewPriorityQueue(nil, testutil.NewTestLogger())
	
	now := time.Now()
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: now,
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  10,
		createdAt: now.Add(time.Second), // 稍后创建
	}
	
	queue.Enqueue(item1)
	queue.Enqueue(item2)
	
	// 相同优先级，先创建的应该先出队
	dequeued := queue.Dequeue()
	require.Equal(t, "item1", dequeued.GetID())
}

// TestNewTransactionTypeStrategy 测试创建交易类型策略
func TestNewTransactionTypeStrategy(t *testing.T) {
	strategy := NewTransactionTypeStrategy()
	require.NotNil(t, strategy)
	require.NotNil(t, strategy.typePriorityMap)
}

// TestNewExecutionTimeStrategy 测试创建执行时间策略
func TestNewExecutionTimeStrategy(t *testing.T) {
	strategy := NewExecutionTimeStrategy(5)
	require.NotNil(t, strategy)
	require.Equal(t, 5, strategy.penaltyPerSecond)
	
	// 测试默认值
	strategy = NewExecutionTimeStrategy(0)
	require.Equal(t, 5, strategy.penaltyPerSecond)
}

// TestNewWaitTimeStrategy 测试创建等待时间策略
func TestNewWaitTimeStrategy(t *testing.T) {
	strategy := NewWaitTimeStrategy(2)
	require.NotNil(t, strategy)
	require.Equal(t, 2, strategy.bonusPerSecond)
	
	// 测试默认值
	strategy = NewWaitTimeStrategy(0)
	require.Equal(t, 2, strategy.bonusPerSecond)
}

// TestNewMixedStrategy 测试创建混合策略
func TestNewMixedStrategy(t *testing.T) {
	strategy := NewMixedStrategy()
	require.NotNil(t, strategy)
	require.NotNil(t, strategy.baseStrategy)
	require.NotNil(t, strategy.executionTimeStrategy)
	require.NotNil(t, strategy.waitTimeStrategy)
}

// TestWaitTimeStrategy_CalculatePriority 测试等待时间策略计算优先级
func TestWaitTimeStrategy_CalculatePriority(t *testing.T) {
	strategy := NewWaitTimeStrategy(2)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now().Add(-10 * time.Second), // 10秒前创建
	}
	
	priority := strategy.CalculatePriority(10, item, time.Now())
	// 10秒 * 2 = 20，所以优先级应该是 10 + 20 = 30
	require.Greater(t, priority, 10)
}

// TestTransactionTypeStrategy_CalculatePriority 测试交易类型策略计算优先级
func TestTransactionTypeStrategy_CalculatePriority(t *testing.T) {
	strategy := NewTransactionTypeStrategy()
	
	// 测试策略的typePriorityMap
	require.Equal(t, 100, strategy.typePriorityMap["critical"])
	require.Equal(t, 80, strategy.typePriorityMap["high"])
	require.Equal(t, 50, strategy.typePriorityMap["normal"])
	require.Equal(t, 20, strategy.typePriorityMap["low"])
	require.Equal(t, 10, strategy.typePriorityMap["background"])
	
	// 测试没有交易类型的item（使用基础优先级）
	normalItem := &mockPriorityItem{
		id:        "item2",
		priority:  10,
		createdAt: time.Now(),
	}
	priority := strategy.CalculatePriority(10, normalItem, time.Now())
	require.Equal(t, 10, priority) // 使用基础优先级
}

