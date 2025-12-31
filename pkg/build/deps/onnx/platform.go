// Package onnx 提供 ONNX Runtime 库文件的嵌入和加载功能
// 包含平台支持检测和引擎可用性判断

package onnx

import (
	"fmt"
	"runtime"
)

// PlatformSupportInfo 平台支持信息
type PlatformSupportInfo struct {
	// IsSupported 当前平台是否支持 ONNX Runtime
	IsSupported bool
	// Platform 平台标识符（格式：GOOS_GOARCH）
	Platform string
	// Reason 不支持的原因（如果 IsSupported 为 false）
	Reason string
	// EngineSupport 引擎支持情况
	EngineSupport EngineSupportMatrix
}

// EngineSupportMatrix 引擎支持矩阵
type EngineSupportMatrix struct {
	// WASM 是否支持 WASM 引擎（所有平台都支持）
	WASM bool
	// ONNX 是否支持 ONNX 引擎（取决于平台）
	ONNX bool
}

// IsPlatformSupported 检查当前平台是否支持 ONNX Runtime
//
// 注意：ONNX Runtime 官方"支持"这些平台，但并非所有平台都有预编译库。
// 实际有预编译库的平台（v1.23.2）：
// - Windows: x86_64, ARM64 (2个)
// - Linux: x86_64, ARM64 (2个)
// - macOS: x86_64, ARM64 (2个)
// 其他平台（linux-386, linux-arm, windows-386, android, ios等）需要从源码编译。
//
// 本函数检查的是"理论上支持"的平台（可以从源码编译），
// 实际是否有预编译库由 HasPrebuiltLibrary() 检查。
func IsPlatformSupported() bool {
	info := GetPlatformSupportInfo()
	return info.IsSupported
}

// GetPlatformSupportInfo 获取当前平台的详细支持信息
func GetPlatformSupportInfo() PlatformSupportInfo {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	
	// 默认：WASM 引擎在所有平台都支持
	engineSupport := EngineSupportMatrix{
		WASM: true,
		ONNX: false,
	}
	
	// 检查 ONNX Runtime 官方支持的平台
	switch runtime.GOOS {
	case "windows":
		// Windows: x86_32, x86_64, ARM32v7, ARM64
		switch runtime.GOARCH {
		case "386": // x86_32
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "amd64": // x86_64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm": // ARM32v7
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm64": // ARM64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		default:
			return PlatformSupportInfo{
				IsSupported:   false,
				Platform:      platform,
				Reason:        fmt.Sprintf("Windows %s 架构不在 ONNX Runtime 官方支持列表中", runtime.GOARCH),
				EngineSupport: engineSupport,
			}
		}
		
	case "linux":
		// Linux: x86_32, x86_64, ARM32v7, ARM64, PPC64LE, RISCV64, S390X
		switch runtime.GOARCH {
		case "386": // x86_32
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "amd64": // x86_64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm": // ARM32v7
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm64": // ARM64 (aarch64)
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "ppc64le": // PPC64LE
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "riscv64": // RISCV64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "s390x": // S390X
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		default:
			return PlatformSupportInfo{
				IsSupported:   false,
				Platform:      platform,
				Reason:        fmt.Sprintf("Linux %s 架构不在 ONNX Runtime 官方支持列表中", runtime.GOARCH),
				EngineSupport: engineSupport,
			}
		}
		
	case "darwin":
		// macOS: x86_64, ARM64
		switch runtime.GOARCH {
		case "amd64": // x86_64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm64": // ARM64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		default:
			return PlatformSupportInfo{
				IsSupported:   false,
				Platform:      platform,
				Reason:        fmt.Sprintf("macOS %s 架构不在 ONNX Runtime 官方支持列表中", runtime.GOARCH),
				EngineSupport: engineSupport,
			}
		}
		
	case "android":
		// Android: ARM32v7, ARM64
		switch runtime.GOARCH {
		case "arm": // ARM32v7
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		case "arm64": // ARM64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		default:
			return PlatformSupportInfo{
				IsSupported:   false,
				Platform:      platform,
				Reason:        fmt.Sprintf("Android %s 架构不在 ONNX Runtime 官方支持列表中", runtime.GOARCH),
				EngineSupport: engineSupport,
			}
		}
		
	case "ios":
		// iOS: ARM64
		switch runtime.GOARCH {
		case "arm64": // ARM64
			engineSupport.ONNX = true
			return PlatformSupportInfo{
				IsSupported:   true,
				Platform:      platform,
				Reason:        "",
				EngineSupport: engineSupport,
			}
		default:
			return PlatformSupportInfo{
				IsSupported:   false,
				Platform:      platform,
				Reason:        fmt.Sprintf("iOS %s 架构不在 ONNX Runtime 官方支持列表中", runtime.GOARCH),
				EngineSupport: engineSupport,
			}
		}
		
	default:
		// 其他操作系统（BSD、Solaris 等）不支持 ONNX Runtime
		return PlatformSupportInfo{
			IsSupported:   false,
			Platform:      platform,
			Reason:        fmt.Sprintf("操作系统 %s 不在 ONNX Runtime 官方支持列表中", runtime.GOOS),
			EngineSupport: engineSupport,
		}
	}
}

// HasPrebuiltLibrary 检查当前平台是否有预编译库
//
// ONNX Runtime v1.23.2 实际提供的预编译库：
// - darwin_amd64, darwin_arm64
// - linux_amd64, linux_arm64
// - windows_amd64, windows_arm64
func HasPrebuiltLibrary() bool {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	switch platform {
	case "darwin_amd64", "darwin_arm64",
		 "linux_amd64", "linux_arm64",
		 "windows_amd64", "windows_arm64":
		return true
	default:
		return false
	}
}

// GetSupportedPlatforms 获取所有支持的平台列表（用于文档）
//
// 返回所有理论上支持的平台（可以从源码编译）
func GetSupportedPlatforms() []PlatformSupportInfo {
	platforms := []struct {
		os   string
		arch []string
	}{
		{"windows", []string{"386", "amd64", "arm", "arm64"}},
		{"linux", []string{"386", "amd64", "arm", "arm64", "ppc64le", "riscv64", "s390x"}},
		{"darwin", []string{"amd64", "arm64"}},
		{"android", []string{"arm", "arm64"}},
		{"ios", []string{"arm64"}},
	}
	
	var result []PlatformSupportInfo
	for _, p := range platforms {
		for _, arch := range p.arch {
			platform := fmt.Sprintf("%s_%s", p.os, arch)
			hasPrebuilt := HasPrebuiltLibraryForPlatform(p.os, arch)
			result = append(result, PlatformSupportInfo{
				IsSupported: true,
				Platform:    platform,
				Reason:      func() string {
					if hasPrebuilt {
						return ""
					}
					return "需要从源码编译"
				}(),
				EngineSupport: EngineSupportMatrix{
					WASM: true,
					ONNX: true,
				},
			})
		}
	}
	return result
}

// HasPrebuiltLibraryForPlatform 检查指定平台是否有预编译库
func HasPrebuiltLibraryForPlatform(os, arch string) bool {
	platform := fmt.Sprintf("%s_%s", os, arch)
	switch platform {
	case "darwin_amd64", "darwin_arm64",
		 "linux_amd64", "linux_arm64",
		 "windows_amd64", "windows_arm64":
		return true
	default:
		return false
	}
}

