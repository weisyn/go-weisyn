//go:build android && arm64
// +build android,arm64

package onnx

import _ "embed"

// 需要从源码编译 ONNX Runtime（交叉编译），编译后将库文件放到此目录
// 然后取消下面的注释以启用嵌入
//go:embed libonnxruntime.so
// var embeddedLibAndroidARM64 []byte

// libAndroidARM64 存储 Android ARM64 平台的库文件数据
// 此变量在父目录的 embedded.go 中也有声明，但由于条件编译，不会冲突
var libAndroidARM64 []byte

// func init() {
// 	libAndroidARM64 = embeddedLibAndroidARM64
// }
