//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package onnx 提供 ONNX Runtime 库文件的嵌入和加载功能
// 所有平台的库文件已预下载并使用 go:embed 嵌入
// 用户可以直接运行 `go run cmd/weisyn/main.go`，无需任何下载步骤
package onnx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

// ErrPlatformNotSupported 平台不支持错误
var ErrPlatformNotSupported = fmt.Errorf("当前平台不支持 ONNX Runtime")

// LoadEmbeddedLibrary 加载当前平台的嵌入 ONNX Runtime 库文件
// 将库文件提取到应用数据目录并设置库路径
//
// 此函数应在初始化 ONNX Runtime 环境之前调用
//
// 返回值：
//   - error: 如果平台不支持或没有预编译库，返回错误
func LoadEmbeddedLibrary() error {
	// 检查平台支持（理论上是否支持）
	if !IsPlatformSupported() {
		info := GetPlatformSupportInfo()
		return fmt.Errorf("%w: %s (%s)", ErrPlatformNotSupported, info.Platform, info.Reason)
	}

	// 检查是否有预编译库（实际是否有可用的库文件）
	if !HasPrebuiltLibrary() {
		info := GetPlatformSupportInfo()
		return fmt.Errorf("当前平台 %s 受 ONNX Runtime 官方支持，但无预编译库。需要从源码编译 ONNX Runtime", info.Platform)
	}

	// 获取应用数据目录
	appDataDir, err := getAppDataDir()
	if err != nil {
		return fmt.Errorf("获取应用数据目录失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		return fmt.Errorf("创建应用数据目录失败: %w", err)
	}

	// 根据平台确定库文件名
	libName := getLibraryName()
	if libName == "" {
		// 这不应该发生，因为已经检查过平台支持
		return fmt.Errorf("无法确定库文件名: %s_%s", runtime.GOOS, runtime.GOARCH)
	}

	libPath := filepath.Join(appDataDir, libName)

	// 检查库文件是否已存在且有效
	if isLibraryValid(libPath) {
		// 库文件已存在且有效，直接初始化环境
		return initializeEnvironmentWithLibPath(libPath)
	}

	// 尝试获取嵌入的库文件数据
	libData, err := getEmbeddedLibrary()
	if err != nil {
		// 如果嵌入的库文件不可用，返回友好的错误信息
		return fmt.Errorf(
			"ONNX Runtime库文件未找到。\n"+
				"这通常是因为库文件未下载或未提交到 Git。\n"+
				"解决方法（开发者）:\n"+
				"  1. 运行: bash pkg/build/deps/onnx/download.sh\n"+
				"  2. 提交库文件到 Git: git add pkg/build/deps/onnx/libs/\n"+
				"  3. 然后构建: go build ./cmd/weisyn\n"+
				"原始错误: %w", err)
	}

	// 写入库文件
	if err := os.WriteFile(libPath, libData, 0755); err != nil {
		return fmt.Errorf("写入ONNX Runtime库文件失败: %w", err)
	}

	// 使用写入后的库文件初始化环境
	return initializeEnvironmentWithLibPath(libPath)
}

// initializeEnvironmentWithLibPath 使用指定路径初始化 ONNX Runtime 环境
//
// 职责：
//   1. 将库路径设置到 onnxruntime_go
//   2. 如果尚未初始化，调用 InitializeEnvironment()
//   3. 验证 IsInitialized() 状态，确保环境可用
func initializeEnvironmentWithLibPath(libPath string) error {
	// 解析为绝对路径，避免工作目录变化影响
	absLibPath, err := filepath.Abs(libPath)
	if err != nil {
		return fmt.Errorf("解析ONNX Runtime库路径失败: %w", err)
	}

	// ⚠️ 关键：无论是否已初始化，都要确保库路径正确设置
	ort.SetSharedLibraryPath(absLibPath)

	// 如果环境已经初始化，直接返回
	if ort.IsInitialized() {
		return nil
	}

	// 初始化 ONNX Runtime 环境
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("初始化ONNX Runtime环境失败（库路径: %s）: %w", absLibPath, err)
	}

	// 验证初始化是否真正成功
	if !ort.IsInitialized() {
		return fmt.Errorf("ONNX Runtime初始化失败：InitializeEnvironment() 成功但 IsInitialized() 返回 false")
	}

	return nil
}

// getAppDataDir 返回用于存储 ONNX Runtime 库文件的应用数据目录
func getAppDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// 使用 ~/.weisyn/libs/ 作为应用数据目录
	return filepath.Join(homeDir, ".weisyn", "libs"), nil
}

// getLibraryName 返回当前平台的库文件名
func getLibraryName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "linux":
		return "libonnxruntime.so"
	case "windows":
		return "onnxruntime.dll"
	case "android":
		return "libonnxruntime.so"
	case "ios":
		return "libonnxruntime.dylib"
	default:
		return ""
	}
}

// isLibraryValid 检查库文件是否存在且有效
func isLibraryValid(libPath string) bool {
	info, err := os.Stat(libPath)
	if err != nil {
		return false
	}
	// 检查是否为常规文件且大小合理（> 1MB）
	return info.Mode().IsRegular() && info.Size() > 1024*1024
}

// （已移除 getPlatform 辅助函数，避免未使用代码触发 linter 警告）
