// Package onnx 提供 ONNX Runtime 库文件的嵌入和加载功能
// 使用条件编译，只嵌入当前编译平台的库文件
//
// 📦 二进制打包说明：
// - 编译后的二进制文件只包含当前编译平台的库文件（约 96-134MB）
// - 二进制可以传输到其他相同平台的电脑直接运行
// - 运行时自动提取对应平台的库文件到 ~/.weisyn/libs/
// - 详细部署说明请参考：pkg/build/deps/onnx/DEPLOYMENT.md
//
// 🔧 平台特定的嵌入文件：
// - embedded_darwin_amd64.go   - macOS Intel
// - embedded_darwin_arm64.go   - macOS Apple Silicon
// - embedded_linux_amd64.go    - Linux x64
// - embedded_linux_arm64.go    - Linux ARM64
// - embedded_windows_amd64.go  - Windows x64
// - embedded_windows_arm64.go  - Windows ARM64

package onnx

import (
	"fmt"
	"runtime"
)

// 平台特定的库文件变量声明
// 这些变量在所有文件中可见，但在各平台的 embedded.go 文件中赋值
// 使用条件编译，每个平台只嵌入对应的库文件，减小二进制体积
// 嵌入文件位置：pkg/build/deps/onnx/libs/{platform}/embedded.go
var (
	libDarwinAMD64  []byte // macOS Intel 平台的库文件（在 libs/darwin_amd64/embedded.go 中赋值）
	libDarwinARM64  []byte // macOS Apple Silicon 平台的库文件（在 libs/darwin_arm64/embedded.go 中赋值）
	libLinuxAMD64   []byte // Linux x64 平台的库文件（在 libs/linux_amd64/embedded.go 中赋值）
	libLinuxARM64   []byte // Linux ARM64 平台的库文件（在 libs/linux_arm64/embedded.go 中赋值）
	libWindowsAMD64 []byte // Windows x64 平台的库文件（在 libs/windows_amd64/embedded.go 中赋值）
	libWindowsARM64 []byte // Windows ARM64 平台的库文件（在 libs/windows_arm64/embedded.go 中赋值）
	// 以下变量用于需要从源码编译的平台（在对应平台的 embedded.go 中定义）
	libLinux386     []byte // Linux x86_32 平台的库文件（在 libs/linux_386/embedded.go 中赋值）
	libLinuxARM     []byte // Linux ARM32v7 平台的库文件（在 libs/linux_arm/embedded.go 中赋值）
	libLinuxPPC64LE []byte // Linux PPC64LE 平台的库文件（在 libs/linux_ppc64le/embedded.go 中赋值）
	libLinuxRISCV64 []byte // Linux RISCV64 平台的库文件（在 libs/linux_riscv64/embedded.go 中赋值）
	libLinuxS390X   []byte // Linux S390X 平台的库文件（在 libs/linux_s390x/embedded.go 中赋值）
	libWindows386   []byte // Windows x86_32 平台的库文件（在 libs/windows_386/embedded.go 中赋值）
	libWindowsARM   []byte // Windows ARM32v7 平台的库文件（在 libs/windows_arm/embedded.go 中赋值）
	libAndroidARM   []byte // Android ARM32v7 平台的库文件（在 libs/android_arm/embedded.go 中赋值）
	libAndroidARM64 []byte // Android ARM64 平台的库文件（在 libs/android_arm64/embedded.go 中赋值）
	libIOSARM64     []byte // iOS ARM64 平台的库文件（在 libs/ios_arm64/embedded.go 中赋值）
)

// ============================================================================
// 以下平台需要从源码编译 ONNX Runtime，编译后将库文件放到对应目录并取消注释
// ============================================================================

// Linux x86_32 (386) - 需从源码编译
// 编译后库文件位置：build/Linux/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/linux_386/libonnxruntime.so
// 然后取消下面的注释：
// var libLinux386 []byte // Linux x86_32 平台的库文件
// //go:embed libs/linux_386/libonnxruntime.so

// Linux ARM32v7 (arm) - 需从源码编译
// 编译后库文件位置：build/Linux/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/linux_arm/libonnxruntime.so
// 然后取消下面的注释：
// var libLinuxARM []byte // Linux ARM32v7 平台的库文件
// //go:embed libs/linux_arm/libonnxruntime.so

// Linux PPC64LE - 需从源码编译
// 编译后库文件位置：build/Linux/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/linux_ppc64le/libonnxruntime.so
// 然后取消下面的注释：
// var libLinuxPPC64LE []byte // Linux PPC64LE 平台的库文件
// //go:embed libs/linux_ppc64le/libonnxruntime.so

// Linux RISCV64 - 需从源码编译
// 编译后库文件位置：build/Linux/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/linux_riscv64/libonnxruntime.so
// 然后取消下面的注释：
// var libLinuxRISCV64 []byte // Linux RISCV64 平台的库文件
// //go:embed libs/linux_riscv64/libonnxruntime.so

// Linux S390X - 需从源码编译
// 编译后库文件位置：build/Linux/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/linux_s390x/libonnxruntime.so
// 然后取消下面的注释：
// var libLinuxS390X []byte // Linux S390X 平台的库文件
// //go:embed libs/linux_s390x/libonnxruntime.so

// Windows x86_32 (386) - 需从源码编译
// 编译后库文件位置：build/Windows/Release/Release/onnxruntime.dll
// 复制到：pkg/build/deps/onnx/libs/windows_386/onnxruntime.dll
// 然后取消下面的注释：
// var libWindows386 []byte // Windows x86_32 平台的库文件
// //go:embed libs/windows_386/onnxruntime.dll

// Windows ARM32v7 (arm) - 需从源码编译
// 编译后库文件位置：build/Windows/Release/Release/onnxruntime.dll
// 复制到：pkg/build/deps/onnx/libs/windows_arm/onnxruntime.dll
// 然后取消下面的注释：
// var libWindowsARM []byte // Windows ARM32v7 平台的库文件
// //go:embed libs/windows_arm/onnxruntime.dll

// Android ARM32v7 (arm) - 需从源码编译（交叉编译）
// 编译命令：./build.sh --config Release --build_shared_lib --android --android_abi armeabi-v7a --android_api 29
// 编译后库文件位置：build/Android/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/android_arm/libonnxruntime.so
// 然后取消下面的注释：
// var libAndroidARM []byte // Android ARM32v7 平台的库文件
// //go:embed libs/android_arm/libonnxruntime.so

// Android ARM64 - 需从源码编译（交叉编译）
// 编译命令：./build.sh --config Release --build_shared_lib --android --android_abi arm64-v8a --android_api 29
// 编译后库文件位置：build/Android/Release/libonnxruntime.so
// 复制到：pkg/build/deps/onnx/libs/android_arm64/libonnxruntime.so
// 然后取消下面的注释：
// var libAndroidARM64 []byte // Android ARM64 平台的库文件
// //go:embed libs/android_arm64/libonnxruntime.so

// iOS ARM64 - 需从源码编译（交叉编译）
// 编译命令：./build.sh --config Release --build_shared_lib --ios --ios_sysroot <path> --ios_arch arm64
// 编译后库文件位置：build/iOS/Release/libonnxruntime.dylib
// 复制到：pkg/build/deps/onnx/libs/ios_arm64/libonnxruntime.dylib
// 然后取消下面的注释：
// var libIOSARM64 []byte // iOS ARM64 平台的库文件
// //go:embed libs/ios_arm64/libonnxruntime.dylib

// ============================================================================
// 详细编译说明请参考：pkg/build/deps/onnx/libs/README.md
// ============================================================================

// getEmbeddedLibrary 返回当前平台的嵌入库文件数据
//
// 注意：此函数仅返回已嵌入的平台库文件。
// 如果平台支持但未嵌入，会返回错误，提示需要下载库文件。
// 平台支持检测由 platform.go 中的 IsPlatformSupported() 处理。
func getEmbeddedLibrary() ([]byte, error) {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	switch platform {
	case "darwin_amd64":
		if len(libDarwinAMD64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (darwin_amd64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libDarwinAMD64, nil
	case "darwin_arm64":
		if len(libDarwinARM64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (darwin_arm64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libDarwinARM64, nil
	case "linux_amd64":
		if len(libLinuxAMD64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (linux_amd64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libLinuxAMD64, nil
	case "linux_arm64":
		if len(libLinuxARM64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (linux_arm64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libLinuxARM64, nil
	case "windows_amd64":
		if len(libWindowsAMD64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (windows_amd64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libWindowsAMD64, nil
	case "windows_arm64":
		if len(libWindowsARM64) == 0 {
			return nil, fmt.Errorf("嵌入的库文件为空 (windows_arm64)。请运行: bash pkg/build/deps/onnx/download.sh")
		}
		return libWindowsARM64, nil
	// ============================================================================
	// 以下平台需要从源码编译，编译后取消上面的注释并添加对应的 case 分支
	// ============================================================================
	// case "linux_386":
	//     if len(libLinux386) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (linux_386)")
	//     }
	//     return libLinux386, nil
	// case "linux_arm":
	//     if len(libLinuxARM) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (linux_arm)")
	//     }
	//     return libLinuxARM, nil
	// case "linux_ppc64le":
	//     if len(libLinuxPPC64LE) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (linux_ppc64le)")
	//     }
	//     return libLinuxPPC64LE, nil
	// case "linux_riscv64":
	//     if len(libLinuxRISCV64) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (linux_riscv64)")
	//     }
	//     return libLinuxRISCV64, nil
	// case "linux_s390x":
	//     if len(libLinuxS390X) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (linux_s390x)")
	//     }
	//     return libLinuxS390X, nil
	// case "windows_386":
	//     if len(libWindows386) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (windows_386)")
	//     }
	//     return libWindows386, nil
	// case "windows_arm":
	//     if len(libWindowsARM) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (windows_arm)")
	//     }
	//     return libWindowsARM, nil
	// case "android_arm":
	//     if len(libAndroidARM) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (android_arm)")
	//     }
	//     return libAndroidARM, nil
	// case "android_arm64":
	//     if len(libAndroidARM64) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (android_arm64)")
	//     }
	//     return libAndroidARM64, nil
	// case "ios_arm64":
	//     if len(libIOSARM64) == 0 {
	//         return nil, fmt.Errorf("嵌入的库文件为空 (ios_arm64)")
	//     }
	//     return libIOSARM64, nil
	default:
		// 检查是否是官方支持但未嵌入的平台
		if IsPlatformSupported() {
			if HasPrebuiltLibrary() {
				return nil, fmt.Errorf("平台 %s 有预编译库，但库文件未嵌入。请联系开发者添加此平台的库文件", platform)
			}
			return nil, fmt.Errorf("平台 %s 受 ONNX Runtime 官方支持，但无预编译库，需要从源码编译", platform)
		}
		// 平台不支持（由 platform.go 处理）
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}
