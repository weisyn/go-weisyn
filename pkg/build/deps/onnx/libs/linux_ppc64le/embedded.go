//go:build linux && ppc64le
// +build linux,ppc64le

package onnx

import _ "embed"

// 需要从源码编译 ONNX Runtime，编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibLinuxPPC64LE []byte

// libLinuxPPC64LE 存储 Linux PPC64LE 平台的库文件数据
// 此变量在父目录的 embedded.go 中也有声明，但由于条件编译，不会冲突
var libLinuxPPC64LE []byte

// func init() {
// 	libLinuxPPC64LE = embeddedLibLinuxPPC64LE
// }

