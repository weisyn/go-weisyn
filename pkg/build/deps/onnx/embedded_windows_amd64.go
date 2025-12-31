//go:build windows && amd64
// +build windows,amd64

package onnx

import _ "embed"

// Windows x64 平台嵌入库
//
//go:embed libs/windows_amd64/onnxruntime.dll
var embeddedLibWindowsAMD64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libWindowsAMD64 = embeddedLibWindowsAMD64
}


