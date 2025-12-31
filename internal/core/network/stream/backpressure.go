// Package stream provides backpressure control mechanisms for network streams.
package stream

import (
	"context"
	"sync"
	"time"
)

// Semaphore 简化版信号量，用于背压控制
type Semaphore struct {
	ch chan struct{}
	mu sync.Mutex
}

// NewSemaphore 创建指定容量的信号量
func NewSemaphore(capacity int) *Semaphore {
	if capacity <= 0 {
		capacity = 1
	}
	return &Semaphore{ch: make(chan struct{}, capacity)}
}

// Acquire 获取信号量（阻塞直到有可用资源）
func (s *Semaphore) Acquire(ctx context.Context) error {
	// 若 ctx 已取消，应立即失败（避免在“资源可用”时仍然成功获取，导致调用方误判）。
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire 非阻塞获取信号量
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// AcquireWithTimeout 带超时的获取
func (s *Semaphore) AcquireWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Acquire(ctx)
}

// Release 释放信号量
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
	default:
		// 已经为空，忽略重复释放
	}
}

// Available 返回可用资源数
func (s *Semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}

// Capacity 返回总容量
func (s *Semaphore) Capacity() int {
	return cap(s.ch)
}
