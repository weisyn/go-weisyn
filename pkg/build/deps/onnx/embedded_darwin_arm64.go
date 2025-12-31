//go:build darwin && arm64
// +build darwin,arm64

package onnx

import _ "embed"

// macOS Apple Silicon 平台嵌入库
//
//go:embed libs/darwin_arm64/libonnxruntime.dylib
var embeddedLibDarwinARM64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libDarwinARM64 = embeddedLibDarwinARM64
}


