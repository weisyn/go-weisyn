//go:build darwin && amd64
// +build darwin,amd64

package onnx

import _ "embed"

// macOS Intel 平台嵌入库
//
// 注意：
// - 这里只负责嵌入当前平台的库文件
// - 变量 libDarwinAMD64 在 embedded.go 中声明，这里通过 init() 进行赋值
// - 路径以本文件所在目录为基准
//
//go:embed libs/darwin_amd64/libonnxruntime.dylib
var embeddedLibDarwinAMD64 []byte

func init() {
	// 将嵌入的库字节赋值给通用变量，供 getEmbeddedLibrary() 使用
	libDarwinAMD64 = embeddedLibDarwinAMD64
}


