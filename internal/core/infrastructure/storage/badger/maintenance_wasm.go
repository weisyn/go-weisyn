//go:build js || wasm
// +build js wasm

package badger

import (
	"context"
)

// checkDiskSpace 检查数据库目录所在磁盘空间 (WebAssembly版本)
// 在WASM环境中，无法直接访问文件系统，所以这是一个空实现
func (s *Store) checkDiskSpace(ctx context.Context) {
	// WebAssembly环境中无法检查磁盘空间
	// 这里可以记录一个调试信息或者什么都不做
	s.logger.Debugf("磁盘空间检查在WebAssembly环境中不可用")
}
