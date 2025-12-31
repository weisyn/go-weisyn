// Package runtime provides error definitions for the WASM runtime engine.
package runtime

import (
	"errors"
	"fmt"
)

// WASM运行时错误定义
//
// 🎯 **职责范围**：仅包含WASM编译、实例化、执行相关的错误
// 📋 **设计原则**：内部使用，不对外暴露，按需包装返回

// ==================== 基础错误定义 ====================

var (
	// 基础运行时错误定义
	errCompileFailed     = errors.New("WASM合约编译失败")
	errInstantiateFailed = errors.New("WASM合约实例化失败")
	errExecuteFailed     = errors.New("WASM合约执行失败")
	errFunctionNotFound  = errors.New("WASM导出函数未找到")
	errInvalidSignature  = errors.New("WASM函数签名不匹配")
	errMemoryAccess      = errors.New("WASM内存访问失败")
	errInvalidParams     = errors.New("WASM调用参数无效")
)

// ==================== 运行时错误包装 ====================

var (
	// ErrCompileFailed 编译失败错误
	ErrCompileFailed = fmt.Errorf("运行时编译错误: %w", errCompileFailed)

	// ErrInstantiateFailed 实例化失败错误
	ErrInstantiateFailed = fmt.Errorf("运行时实例化错误: %w", errInstantiateFailed)

	// ErrExecuteFailed 执行失败错误
	ErrExecuteFailed = fmt.Errorf("运行时执行错误: %w", errExecuteFailed)

	// ErrFunctionNotFound 函数未找到错误
	ErrFunctionNotFound = fmt.Errorf("运行时函数查找错误: %w", errFunctionNotFound)

	// ErrInvalidSignature 函数签名不匹配错误
	ErrInvalidSignature = fmt.Errorf("运行时签名错误: %w", errInvalidSignature)

	// ErrMemoryAccess 内存访问失败错误
	ErrMemoryAccess = fmt.Errorf("运行时内存错误: %w", errMemoryAccess)

	// ErrInvalidParams 无效参数错误
	ErrInvalidParams = fmt.Errorf("运行时参数错误: %w", errInvalidParams)
)

// ==================== 运行时常量定义 ====================

// WASM参数类型常量（运行时使用）
const (
	WASMTypeI32 = "i32" // 32位整数
	WASMTypeI64 = "i64" // 64位整数
	WASMTypeF32 = "f32" // 32位浮点数
	WASMTypeF64 = "f64" // 64位浮点数
)
