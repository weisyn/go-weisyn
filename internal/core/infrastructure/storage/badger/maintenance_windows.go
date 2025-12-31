//go:build windows
// +build windows

package badger

import (
	"context"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// checkDiskSpace 检查数据库目录所在磁盘空间 (Windows版本)
func (s *Store) checkDiskSpace(ctx context.Context) {
	// 检查数据库目录所在磁盘空间
	dataDir := s.config.GetPath()
	
	// 获取目录的绝对路径
	absPath, err := filepath.Abs(dataDir)
	if err != nil {
		s.logger.Errorf("获取数据库目录绝对路径失败: %v", err)
		return
	}
	
	// 获取目录所在的驱动器根路径（例如 C:\）
	rootPath := filepath.VolumeName(absPath) + "\\"
	if rootPath == "" {
		// 如果无法获取驱动器，使用目录本身
		rootPath = absPath
	}
	
	// 使用 Windows API 获取磁盘空间
	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	
	// 将路径转换为 UTF-16
	rootPathPtr, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		s.logger.Errorf("转换路径失败: %v", err)
		return
	}
	
	// 调用 GetDiskFreeSpaceExW
	ret, _, _ := syscall.SyscallN(
		windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW").Addr(),
		uintptr(unsafe.Pointer(rootPathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	
	if ret == 0 {
		s.logger.Errorf("检查磁盘空间失败: 无法获取磁盘信息")
		return
	}
	
	// 计算已使用空间百分比
	if totalBytes > 0 {
		usedBytes := totalBytes - freeBytesAvailable
		usedPercent := float64(usedBytes) / float64(totalBytes) * 100
		
		// 空间不足警告
		if usedPercent > 85 {
			s.logger.Warnf("数据库磁盘空间使用率高: %.2f%%", usedPercent)
		}
	}
}

