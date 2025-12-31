//go:build linux && s390x
// +build linux,s390x

package onnx

import _ "embed"

// 需要从源码编译 ONNX Runtime，编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibLinuxS390X []byte

// libLinuxS390X 存储 Linux S390X 平台的库文件数据
// 此变量在父目录的 embedded.go 中也有声明，但由于条件编译，不会冲突
var libLinuxS390X []byte

// func init() {
// 	libLinuxS390X = embeddedLibLinuxS390X
// }

