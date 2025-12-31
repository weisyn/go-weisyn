// Package loader provides error definitions for WASM contract loading operations.
package loader

// WASM合约加载错误定义
//
// 🎯 **职责范围**：合约加载相关的错误定义
// 📋 **设计原则**：预留错误类型定义，实际使用 fmt.Errorf 构建详细错误信息
//
// ⚠️ **当前状态**：
// 本文件预留了标准化错误类型的定义，但当前实现中，
// 所有错误都通过 fmt.Errorf 动态构建以提供更详细的上下文信息。
//
// 💡 **未来优化方向**：
// 如果需要错误标准化（如错误码、分类处理等），可以启用这些预定义错误：
//
// import (
//     "errors"
//     "fmt"
// )
//
// var (
//     // ErrContractNotFound 合约文件未找到错误
//     ErrContractNotFound = errors.New("WASM合约文件未找到")
//
//     // ErrInvalidAddress 无效的合约地址错误
//     ErrInvalidAddress = errors.New("无效的WASM合约地址")
//
//     // ErrLoadFailed 合约加载失败错误
//     ErrLoadFailed = errors.New("WASM合约加载失败")
//
//     // ErrInvalidFormat WASM格式无效错误
//     ErrInvalidFormat = errors.New("无效的WASM格式")
// )
//
// 使用示例：
// return nil, fmt.Errorf("%w: 不允许0x前缀", ErrInvalidAddress)
