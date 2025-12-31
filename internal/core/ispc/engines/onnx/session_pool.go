//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package onnx provides session pool management for ONNX inference engine.
package onnx

import (
	"context"
	"fmt"
)

// SessionPool ONNX推理会话池
//
// 🎯 **核心职责**：
// - 控制并发推理数量
// - 防止资源耗尽
// - 支持超时控制
type SessionPool struct {
	maxConcurrent int
	semaphore     chan struct{}
}

// NewSessionPool 创建会话池
func NewSessionPool() *SessionPool {
	maxConcurrent := 10 // 最大并发推理数（可配置）

	return &SessionPool{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Acquire 获取推理执行权限
//
// 使用信号量控制并发数量
func (sp *SessionPool) Acquire(ctx context.Context) error {
	select {
	case sp.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("获取推理执行权限超时: %w", ctx.Err())
	}
}

// Release 释放推理执行权限
func (sp *SessionPool) Release() {
	<-sp.semaphore
}

// Close 关闭会话池
func (sp *SessionPool) Close() error {
	close(sp.semaphore)
	return nil
}

