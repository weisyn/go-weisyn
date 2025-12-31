//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package onnx provides memory pool management for ONNX tensor operations.
package onnx

import (
	"sync"
)

// TensorMemoryPool 张量内存池
//
// 🎯 **核心职责**：
// - 复用张量内存分配
// - 减少GC压力
// - 提升推理性能
type TensorMemoryPool struct {
	pool *sync.Pool
}

// NewTensorMemoryPool 创建张量内存池
func NewTensorMemoryPool() *TensorMemoryPool {
	return &TensorMemoryPool{
		pool: &sync.Pool{
			New: func() interface{} {
				// 预分配常用大小的buffer（1024个float32，约4KB）
				// 使用指针避免 sync.Pool.Put 时的分配（SA6002）
				buf := make([]float32, 1024)
				return &buf
			},
		},
	}
}

// Get 获取张量buffer
//
// 参数：
//   - size: 需要的buffer大小（float32元素数量）
//
// 返回：
//   - []float32: 可用的buffer（长度可能大于size，使用前需要切片）
func (tmp *TensorMemoryPool) Get(size int) []float32 {
	ptr := tmp.pool.Get().(*[]float32)
	buf := *ptr

	// 如果pool返回的buffer太小，重新分配
	if cap(buf) < size {
		return make([]float32, size)
	}

	// 返回适当长度的切片
	return buf[:size]
}

// Put 归还张量buffer
//
// 参数：
//   - buf: 要归还的buffer
//
// 注意：只缓存合理大小的buffer（<=1MB），避免内存泄漏
//
// 实现说明：
// 使用指针类型 (*[]float32) 避免 sync.Pool.Put 时的分配（修复 SA6002 警告）。
// 虽然 slice 本身是引用类型，但作为 interface{} 传递时仍会分配新的 interface{} 对象。
// 使用指针可以避免这个分配，提升性能。
func (tmp *TensorMemoryPool) Put(buf []float32) {
	// 避免缓存过大的buffer（1MB = 256K float32）
	const maxCacheSize = 256 * 1024

	if cap(buf) <= maxCacheSize {
		// 重置 slice 长度，避免保留旧数据引用
		buf = buf[:cap(buf)]
		// 使用指针避免 sync.Pool.Put 时的分配（SA6002）
		tmp.pool.Put(&buf)
	}
	// 如果buffer太大，直接丢弃（让GC处理）
}
