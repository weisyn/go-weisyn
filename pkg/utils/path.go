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
	return os.MkdirAll(path, 0755)
}

// EnsureDataDir 确保数据目录存在
func EnsureDataDir(relativePath string) (string, error) {
	absolutePath := ResolveDataPath(relativePath)
	err := EnsureDir(filepath.Dir(absolutePath))
	return absolutePath, err
}
