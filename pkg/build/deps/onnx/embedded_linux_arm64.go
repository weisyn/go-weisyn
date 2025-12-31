//go:build linux && arm64
// +build linux,arm64

package onnx

import _ "embed"

// Linux ARM64 平台嵌入库
//
//go:embed libs/linux_arm64/libonnxruntime.so
var embeddedLibLinuxARM64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libLinuxARM64 = embeddedLibLinuxARM64
}


