//go:build linux && riscv64
// +build linux,riscv64

package onnx

import _ "embed"

// 需要从源码编译 ONNX Runtime，编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibLinuxRISCV64 []byte

// libLinuxRISCV64 存储 Linux RISCV64 平台的库文件数据
// 此变量在父目录的 embedded.go 中也有声明，但由于条件编译，不会冲突
var libLinuxRISCV64 []byte

// func init() {
// 	libLinuxRISCV64 = embeddedLibLinuxRISCV64
// }

