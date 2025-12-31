//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package onnx provides metrics collection functionality for ONNX inference engine.
package onnx

import (
	"sync"
	"sync/atomic"
	"time"
)

// InferenceMetrics 推理监控指标
//
// 🎯 **核心职责**：
// - 统计推理次数、延迟、错误率
// - 监控缓存命中率
// - 提供性能分析数据
type InferenceMetrics struct {
	// 原子操作统计
	totalInferences  atomic.Int64   // 总推理次数
	totalLatencyMs   atomic.Int64   // 总延迟（毫秒）
	errorCount       atomic.Int64   // 错误次数
	cacheHits        atomic.Int64   // 缓存命中次数
	cacheMisses      atomic.Int64   // 缓存未命中次数

	// 实时统计（需要锁保护）
	lastInferenceTime time.Time
	mu                sync.RWMutex
}

// NewInferenceMetrics 创建推理监控
func NewInferenceMetrics() *InferenceMetrics {
	return &InferenceMetrics{}
}

// RecordInference 记录推理
//
// 参数：
//   - duration: 推理耗时
//   - err: 推理错误（nil表示成功）
func (im *InferenceMetrics) RecordInference(duration time.Duration, err error) {
	im.totalInferences.Add(1)
	im.totalLatencyMs.Add(duration.Milliseconds())

	if err != nil {
		im.errorCount.Add(1)
	}

	im.mu.Lock()
	im.lastInferenceTime = time.Now()
	im.mu.Unlock()
}

// RecordCacheHit 记录缓存命中/未命中
func (im *InferenceMetrics) RecordCacheHit(hit bool) {
	if hit {
		im.cacheHits.Add(1)
	} else {
		im.cacheMisses.Add(1)
	}
}

// Stats 获取统计信息
//
// 返回：
//   - map[string]interface{}: 统计数据
func (im *InferenceMetrics) Stats() map[string]interface{} {
	total := im.totalInferences.Load()
	avgLatency := int64(0)
	if total > 0 {
		avgLatency = im.totalLatencyMs.Load() / total
	}

	cacheTotal := im.cacheHits.Load() + im.cacheMisses.Load()
	cacheHitRate := 0.0
	if cacheTotal > 0 {
		cacheHitRate = float64(im.cacheHits.Load()) / float64(cacheTotal)
	}

	errorRate := 0.0
	if total > 0 {
		errorRate = float64(im.errorCount.Load()) / float64(total)
	}

	im.mu.RLock()
	lastInferenceTime := im.lastInferenceTime
	im.mu.RUnlock()

	return map[string]interface{}{
		"total_inferences":   total,
		"average_latency_ms": avgLatency,
		"error_count":        im.errorCount.Load(),
		"error_rate":         errorRate,
		"cache_hits":         im.cacheHits.Load(),
		"cache_misses":       im.cacheMisses.Load(),
		"cache_hit_rate":     cacheHitRate,
		"last_inference_time": lastInferenceTime,
	}
}

// Reset 重置统计信息
func (im *InferenceMetrics) Reset() {
	im.totalInferences.Store(0)
	im.totalLatencyMs.Store(0)
	im.errorCount.Store(0)
	im.cacheHits.Store(0)
	im.cacheMisses.Store(0)

	im.mu.Lock()
	im.lastInferenceTime = time.Time{}
	im.mu.Unlock()
}

