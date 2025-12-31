// Package writer 错误定义
package writer

import "errors"

// 服务初始化错误
var (
	// ErrCASStorageNil casStorage不能为空
	ErrCASStorageNil = errors.New("casStorage 不能为空")

	// ErrHasherNil hasher不能为空
	ErrHasherNil = errors.New("hasher 不能为空")
)

// 文件操作错误（仅覆盖当前 Writer 实现实际使用的错误类型）
var (
	// ErrReadFileFailed 读取源文件失败
	ErrReadFileFailed = errors.New("读取源文件失败")

	// ErrStoreFileFailed 存储文件到CAS失败
	ErrStoreFileFailed = errors.New("存储文件到CAS失败")
)

