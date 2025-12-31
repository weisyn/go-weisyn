package difficulty

import (
	"context"
	"sync"
	"testing"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// testMTPReader 实现 BlockHeightReader 接口用于测试
type testMTPReader struct {
	blocks map[uint64]*core.Block
	calls  int
	mu     sync.Mutex
}

func (r *testMTPReader) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	return r.blocks[height], nil
}

func (r *testMTPReader) GetCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// makeTestBlocks 创建测试用区块
func makeTestBlocks(count int, startTimestamp uint64) map[uint64]*core.Block {
	blocks := make(map[uint64]*core.Block, count)
	for i := 0; i < count; i++ {
		blocks[uint64(i)] = &core.Block{
			Header: &core.BlockHeader{
				Height:    uint64(i),
				Timestamp: startTimestamp + uint64(i)*30, // 每30秒一个区块
			},
		}
	}
	return blocks
}

func TestMTPCache_BasicOperations(t *testing.T) {
	cache := NewMTPCache(100)

	// 测试 Set 和 Get
	cache.Set(100, 1000)
	cache.Set(101, 1030)

	mtp, ok := cache.Get(100)
	if !ok || mtp != 1000 {
		t.Errorf("Get(100) = %d, %v; want 1000, true", mtp, ok)
	}

	mtp, ok = cache.Get(101)
	if !ok || mtp != 1030 {
		t.Errorf("Get(101) = %d, %v; want 1030, true", mtp, ok)
	}

	// 测试不存在的键
	_, ok = cache.Get(999)
	if ok {
		t.Error("Get(999) should return false for non-existent key")
	}
}

func TestMTPCache_LRUEviction(t *testing.T) {
	cache := NewMTPCache(3) // 只能存储3个条目

	// 添加4个条目，第一个应该被淘汰
	cache.Set(1, 100)
	cache.Set(2, 200)
	cache.Set(3, 300)
	cache.Set(4, 400)

	// 第一个条目应该被淘汰
	_, ok := cache.Get(1)
	if ok {
		t.Error("key 1 should have been evicted")
	}

	// 其他条目应该存在
	for i := uint64(2); i <= 4; i++ {
		if _, ok := cache.Get(i); !ok {
			t.Errorf("key %d should exist", i)
		}
	}
}

func TestMTPCache_LRUAccessOrder(t *testing.T) {
	cache := NewMTPCache(3)

	// 添加3个条目
	cache.Set(1, 100)
	cache.Set(2, 200)
	cache.Set(3, 300)

	// 访问第一个条目，使其变为最近使用
	cache.Get(1)

	// 添加第4个条目，应该淘汰第2个（现在是最旧的）
	cache.Set(4, 400)

	// 第2个条目应该被淘汰
	_, ok := cache.Get(2)
	if ok {
		t.Error("key 2 should have been evicted (LRU)")
	}

	// 第1个条目应该还在
	if _, ok := cache.Get(1); !ok {
		t.Error("key 1 should still exist (recently accessed)")
	}
}

func TestMTPCache_ComputeAndCache(t *testing.T) {
	reader := &testMTPReader{
		blocks: makeTestBlocks(20, 1000),
	}
	cache := NewMTPCache(100)

	// 第一次计算应该调用数据库
	mtp1, err := cache.ComputeAndCache(context.Background(), reader, 10, 11)
	if err != nil {
		t.Fatalf("ComputeAndCache failed: %v", err)
	}

	callsBefore := reader.GetCallCount()
	if callsBefore == 0 {
		t.Error("expected database calls for first computation")
	}

	// 第二次应该命中缓存，不调用数据库
	mtp2, err := cache.ComputeAndCache(context.Background(), reader, 10, 11)
	if err != nil {
		t.Fatalf("ComputeAndCache failed: %v", err)
	}

	callsAfter := reader.GetCallCount()
	if callsAfter != callsBefore {
		t.Errorf("expected no additional database calls, got %d -> %d", callsBefore, callsAfter)
	}

	// 结果应该相同
	if mtp1 != mtp2 {
		t.Errorf("MTP values should match: %d != %d", mtp1, mtp2)
	}
}

func TestMTPCache_ComputeCorrectness(t *testing.T) {
	// 创建具有已知时间戳的区块
	blocks := map[uint64]*core.Block{
		0:  {Header: &core.BlockHeader{Height: 0, Timestamp: 100}},
		1:  {Header: &core.BlockHeader{Height: 1, Timestamp: 130}},
		2:  {Header: &core.BlockHeader{Height: 2, Timestamp: 160}},
		3:  {Header: &core.BlockHeader{Height: 3, Timestamp: 190}},
		4:  {Header: &core.BlockHeader{Height: 4, Timestamp: 220}},
		5:  {Header: &core.BlockHeader{Height: 5, Timestamp: 250}},
		6:  {Header: &core.BlockHeader{Height: 6, Timestamp: 280}},
		7:  {Header: &core.BlockHeader{Height: 7, Timestamp: 310}},
		8:  {Header: &core.BlockHeader{Height: 8, Timestamp: 340}},
		9:  {Header: &core.BlockHeader{Height: 9, Timestamp: 370}},
		10: {Header: &core.BlockHeader{Height: 10, Timestamp: 400}},
	}
	reader := &testMTPReader{blocks: blocks}
	cache := NewMTPCache(100)

	// 对于高度10，窗口大小11，应该取高度0-10的中位数
	// 时间戳：100, 130, 160, 190, 220, 250, 280, 310, 340, 370, 400
	// 排序后中位数（第6个）= 250
	mtp, err := cache.ComputeAndCache(context.Background(), reader, 10, 11)
	if err != nil {
		t.Fatalf("ComputeAndCache failed: %v", err)
	}

	expected := uint64(250)
	if mtp != expected {
		t.Errorf("MTP = %d; want %d", mtp, expected)
	}
}

func TestMTPCache_WindowSmallerThanHeight(t *testing.T) {
	// 测试窗口小于高度的情况
	blocks := makeTestBlocks(100, 1000)
	reader := &testMTPReader{blocks: blocks}
	cache := NewMTPCache(100)

	// 对于高度50，窗口大小11，应该只取高度40-50的区块
	mtp, err := cache.ComputeAndCache(context.Background(), reader, 50, 11)
	if err != nil {
		t.Fatalf("ComputeAndCache failed: %v", err)
	}

	// 高度40-50的时间戳：1000+40*30=2200 到 1000+50*30=2500
	// 中位数应该是高度45的时间戳 = 1000 + 45*30 = 2350
	expected := uint64(1000 + 45*30)
	if mtp != expected {
		t.Errorf("MTP = %d; want %d", mtp, expected)
	}
}

func TestMTPCache_Invalidate(t *testing.T) {
	cache := NewMTPCache(100)

	cache.Set(100, 1000)
	cache.Set(101, 1030)
	cache.Set(102, 1060)

	// 使高度100失效
	cache.Invalidate(100)

	_, ok := cache.Get(100)
	if ok {
		t.Error("key 100 should have been invalidated")
	}

	// 其他键应该还在
	if _, ok := cache.Get(101); !ok {
		t.Error("key 101 should still exist")
	}
}

func TestMTPCache_InvalidateAbove(t *testing.T) {
	cache := NewMTPCache(100)

	cache.Set(98, 980)
	cache.Set(99, 990)
	cache.Set(100, 1000)
	cache.Set(101, 1010)
	cache.Set(102, 1020)

	// 使高度100以上的失效
	cache.InvalidateAbove(100)

	// 高度100及以下应该还在
	for h := uint64(98); h <= 100; h++ {
		if _, ok := cache.Get(h); !ok {
			t.Errorf("key %d should still exist", h)
		}
	}

	// 高度101及以上应该被删除
	for h := uint64(101); h <= 102; h++ {
		if _, ok := cache.Get(h); ok {
			t.Errorf("key %d should have been invalidated", h)
		}
	}
}

func TestMTPCache_Stats(t *testing.T) {
	cache := NewMTPCache(100)

	cache.Set(1, 100)
	cache.Set(2, 200)

	// 两次命中
	cache.Get(1)
	cache.Get(2)

	// 两次未命中
	cache.Get(3)
	cache.Get(4)

	size, capacity, hits, misses, hitRate := cache.Stats()

	if size != 2 {
		t.Errorf("size = %d; want 2", size)
	}
	if capacity != 100 {
		t.Errorf("capacity = %d; want 100", capacity)
	}
	if hits != 2 {
		t.Errorf("hits = %d; want 2", hits)
	}
	if misses != 2 {
		t.Errorf("misses = %d; want 2", misses)
	}
	expectedHitRate := 0.5
	if hitRate != expectedHitRate {
		t.Errorf("hitRate = %f; want %f", hitRate, expectedHitRate)
	}
}

func TestMTPCache_Clear(t *testing.T) {
	cache := NewMTPCache(100)

	cache.Set(1, 100)
	cache.Set(2, 200)
	cache.Get(1)
	cache.Get(999) // miss

	cache.Clear()

	// 统计应该重置（在 Clear 调用后立即检查，不要先调用 Get）
	size, _, hits, misses, _ := cache.Stats()
	if size != 0 {
		t.Errorf("size after clear = %d; want 0", size)
	}
	if hits != 0 || misses != 0 {
		t.Errorf("stats after clear: hits=%d, misses=%d; want 0, 0", hits, misses)
	}

	// 所有键应该被清除（这会增加 misses 计数，所以放在统计检查之后）
	if _, ok := cache.Get(1); ok {
		t.Error("key 1 should have been cleared")
	}
	if _, ok := cache.Get(2); ok {
		t.Error("key 2 should have been cleared")
	}
}

func TestMTPCache_ConcurrentAccess(t *testing.T) {
	cache := NewMTPCache(1000)
	reader := &testMTPReader{
		blocks: makeTestBlocks(200, 1000),
	}

	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(h uint64) {
			defer wg.Done()
			cache.Set(h, h*10)
		}(uint64(i))
	}

	// 并发读取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(h uint64) {
			defer wg.Done()
			cache.Get(h)
		}(uint64(i))
	}

	// 并发计算
	for i := 20; i < 40; i++ {
		wg.Add(1)
		go func(h uint64) {
			defer wg.Done()
			cache.ComputeAndCache(context.Background(), reader, h, 11)
		}(uint64(i))
	}

	wg.Wait()

	// 不应该有 panic，验证缓存状态正常
	size, _, _, _, _ := cache.Stats()
	if size == 0 {
		t.Error("cache should have some entries after concurrent operations")
	}
}

func TestMTPCache_ContextCancellation(t *testing.T) {
	reader := &testMTPReader{
		blocks: makeTestBlocks(100, 1000),
	}
	cache := NewMTPCache(100)

	// 创建已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 应该立即返回上下文错误
	_, err := cache.ComputeAndCache(ctx, reader, 50, 11)
	if err == nil {
		t.Error("expected context error")
	}
}

func TestGlobalMTPCache(t *testing.T) {
	// 验证全局缓存实例存在且可用
	if GlobalMTPCache == nil {
		t.Fatal("GlobalMTPCache should not be nil")
	}

	// 测试基本功能
	GlobalMTPCache.Set(9999, 99990)
	mtp, ok := GlobalMTPCache.Get(9999)
	if !ok || mtp != 99990 {
		t.Errorf("GlobalMTPCache.Get(9999) = %d, %v; want 99990, true", mtp, ok)
	}

	// 清理测试数据
	GlobalMTPCache.Invalidate(9999)
}

// BenchmarkMTPCache_Get 性能测试：缓存读取
func BenchmarkMTPCache_Get(b *testing.B) {
	cache := NewMTPCache(10000)
	for i := 0; i < 10000; i++ {
		cache.Set(uint64(i), uint64(i*30))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(uint64(i % 10000))
	}
}

// BenchmarkMTPCache_ComputeAndCache_Hit 性能测试：缓存命中
func BenchmarkMTPCache_ComputeAndCache_Hit(b *testing.B) {
	reader := &testMTPReader{
		blocks: makeTestBlocks(100, 1000),
	}
	cache := NewMTPCache(1000)

	// 预热缓存
	cache.ComputeAndCache(context.Background(), reader, 50, 11)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.ComputeAndCache(context.Background(), reader, 50, 11)
	}
}

// BenchmarkMTPCache_ComputeAndCache_Miss 性能测试：缓存未命中
func BenchmarkMTPCache_ComputeAndCache_Miss(b *testing.B) {
	reader := &testMTPReader{
		blocks: makeTestBlocks(1000, 1000),
	}
	cache := NewMTPCache(10) // 小缓存强制未命中

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		height := uint64(20 + (i % 900)) // 不同高度
		cache.ComputeAndCache(context.Background(), reader, height, 11)
	}
}

