// Package utils provides path manipulation utility functions.
package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetProjectRoot 获取项目根目录的绝对路径
// 通过查找go.mod文件来确定项目根目录
func GetProjectRoot() string {
	// 1. 首先尝试通过环境变量获取
	if projectRoot := os.Getenv("WES_PROJECT_ROOT"); projectRoot != "" {
		return projectRoot
	}

	// 2. 尝试通过go.mod文件定位项目根目录
	dir, err := os.Getwd()
	if err != nil {
		// 如果获取当前目录失败，使用运行时文件路径
		_, filename, _, ok := runtime.Caller(0)
		if ok {
			// 从当前文件路径向上查找项目根目录
			dir = filepath.Dir(filename)
		} else {
			dir = "."
		}
	}

	// 向上查找go.mod文件
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// 已到达根目录，未找到go.mod
			break
		}
		dir = parent
	}

	// 如果没找到go.mod，返回当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ResolveDataPath 解析数据目录路径为绝对路径
// 如果path已经是绝对路径，直接返回
// 如果是相对路径，基于项目根目录解析
func ResolveDataPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	projectRoot := GetProjectRoot()
	return filepath.Join(projectRoot, path)
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(path string) error {
	//nolint:gosec // G301: 目录需要用户可读权限，0755 是合理的
	return os.MkdirAll(path, 0755)
}

// EnsureDataDir 确保数据目录存在
func EnsureDataDir(relativePath string) (string, error) {
	absolutePath := ResolveDataPath(relativePath)
	err := EnsureDir(filepath.Dir(absolutePath))
	return absolutePath, err
}

// BuildContentAddressedPath 构建基于内容哈希的存储路径（内容寻址）
//
// 🎯 **统一路径构建规则**：
// 这是系统中唯一的内容寻址路径构建方法，确保所有模块使用一致的路径策略。
//
// 📋 **路径构建策略**：
// 使用二级目录结构避免单目录文件过多：
//   - 第一级：哈希前2字符（00-ff，共256个子目录）
//   - 文件名：完整哈希值（64位十六进制字符）
//
// 📝 **路径公式**：
//
//	路径 = hashHex[:2] / hashHex
//
// 📝 **示例**：
//
//	输入：hashHex = "d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b"
//	输出：path = "d2/d2ef233ef664052a09f1ca6e90b8319ab9f2b0e15d6b069069a8062619390a1b"
//
// 💡 **使用场景**：
//   - 资源存储：确定文件保存的相对路径
//   - 资源加载：从内容哈希定位文件位置
//   - 资源索引：构建文件系统索引结构
//
// ⚠️ **重要说明**：
//   - 返回的是相对路径（相对于 fileStoreRootPath）
//   - 需要与 fileStoreRootPath 结合才能得到完整物理路径
//   - 完整路径 = filepath.Join(fileStoreRootPath, BuildContentAddressedPath(hashHex))
//
// 参数：
//   - hashHex: 内容哈希的十六进制字符串（64位，表示32字节SHA-256）
//
// 返回：
//   - 基于内容哈希的相对存储路径
func BuildContentAddressedPath(hashHex string) string {
	if len(hashHex) < 2 {
		// 边界情况：哈希长度不足2位（理论上不应该出现）
		return hashHex
	}
	// 标准情况：二级目录结构（与resource manager一致）
	return filepath.Join(hashHex[:2], hashHex)
}
