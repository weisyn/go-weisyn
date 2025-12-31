//go:build windows && arm64
// +build windows,arm64

package onnx

import _ "embed"

// Windows ARM64 平台嵌入库
//
//go:embed libs/windows_arm64/onnxruntime.dll
var embeddedLibWindowsARM64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libWindowsARM64 = embeddedLibWindowsARM64
}
