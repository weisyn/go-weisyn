//go:build unix && !js && !wasm
// +build unix,!js,!wasm

package badger

import (
	"context"
	"syscall"
)

// checkDiskSpace 检查数据库目录所在磁盘空间 (Unix/Linux版本)
func (s *Store) checkDiskSpace(ctx context.Context) {
	// 检查数据库目录所在磁盘空间
	dataDir := s.config.GetPath()
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		s.logger.Errorf("检查磁盘空间失败: %v", err)
		return
	}

	// 计算可用空间百分比
	available := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	usedPercent := float64(total-available) / float64(total) * 100

	// 空间不足警告
	if usedPercent > 85 {
		s.logger.Warnf("数据库磁盘空间使用率高: %.2f%%", usedPercent)
	}
}
