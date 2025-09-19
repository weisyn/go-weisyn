package discovery

import (
	"math/rand"
	"time"
)

// Backoff 提供指数退避带抖动的等待时间生成：
// - 初始值 base，最大值 max，放大因子 factor；
// - 每次 Next() 返回当前值并推进到下一次的指数值；
// - jitter 让等待时间分布在 [1-jitter, 1+jitter] 区间，避免同步风暴；
// - 非线程安全，建议在单个调度协程内使用。
type Backoff struct {
	base   time.Duration
	max    time.Duration
	factor float64
	jitter float64
	cur    time.Duration
}

func NewBackoff(base, max time.Duration, factor, jitter float64) *Backoff {
	if base <= 0 {
		base = time.Second
	}
	if max <= 0 || max < base {
		max = 30 * time.Second
	}
	if factor < 1.0 {
		factor = 2.0
	}
	if jitter < 0 || jitter > 1 {
		jitter = 0.2
	}
	return &Backoff{base: base, max: max, factor: factor, jitter: jitter, cur: base}
}

func (b *Backoff) Next() time.Duration {
	d := b.cur
	// 抖动：在 [1-jitter, 1+jitter] 之间
	if b.jitter > 0 {
		f := 1 + (rand.Float64()*2-1)*b.jitter
		d = time.Duration(float64(d) * f)
	}
	// 下一个值
	next := time.Duration(float64(b.cur) * b.factor)
	if next > b.max {
		next = b.max
	}
	b.cur = next
	return d
}

func (b *Backoff) Reset() { b.cur = b.base }
