// maintenance.go - 数据库维护相关功能

package badger

import (
	"context"
	"fmt"
	"strings"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
)

// RunValueLogGC 执行值日志垃圾回收
// 清理已删除或过期的值，降低磁盘占用
func (s *Store) RunValueLogGC(ctx context.Context, discardRatio float64) error {
	// 添加超时控制，避免垃圾回收占用过长时间
	type gcResult struct {
		err error
	}

	// 创建结果通道
	resultCh := make(chan gcResult, 1)

	// 在goroutine中执行垃圾回收，避免阻塞
	go func() {
		err := s.db.RunValueLogGC(discardRatio)
		select {
		case resultCh <- gcResult{err: err}:
			// 成功发送结果
		case <-ctx.Done():
			// 上下文已取消，不需要返回结果
		}
	}()

	// 等待结果或超时
	select {
	case result := <-resultCh:
		// 处理垃圾回收结果
		if result.err != nil && result.err != badgerdb.ErrNoRewrite {
			// 忽略"GC request rejected"错误，这通常发生在关闭过程中
			if !strings.Contains(result.err.Error(), "GC request rejected") {
				return fmt.Errorf("值日志垃圾回收失败: %w", result.err)
			}
		}
		return nil
	case <-ctx.Done():
		// 上下文超时或取消
		return fmt.Errorf("值日志垃圾回收被取消: %w", ctx.Err())
	}
}

// StartMaintenanceRoutines 启动定期维护任务
// 包括值日志垃圾回收和磁盘空间监控
func (s *Store) StartMaintenanceRoutines(ctx context.Context) {
	// 值日志垃圾回收
	go func() {
		ticker := time.NewTicker(2 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.RunValueLogGC(ctx, 0.5); err != nil {
					s.logger.Warnf("定期值日志垃圾回收失败: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 磁盘空间监控
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.checkDiskSpace(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// checkDiskSpace 检查数据库目录所在磁盘空间
// 平台特定实现在 maintenance_unix.go 和 maintenance_wasm.go 中
