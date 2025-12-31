//go:build linux && amd64
// +build linux,amd64

package onnx

import _ "embed"

// Linux x64 平台嵌入库
//
//go:embed libs/linux_amd64/libonnxruntime.so
var embeddedLibLinuxAMD64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libLinuxAMD64 = embeddedLibLinuxAMD64
}


