// Package cas 错误定义
package cas

import "errors"

// 服务初始化错误
var (
	// ErrFileStoreNil fileStore不能为空
	ErrFileStoreNil = errors.New("fileStore 不能为空")

	// ErrHasherNil hasher不能为空
	ErrHasherNil = errors.New("hasher 不能为空")
)

// 路径构建错误
var (
	// ErrInvalidHashLength 无效的哈希长度
	ErrInvalidHashLength = errors.New("无效的哈希长度（必须是32字节）")
)

// 文件操作错误
var (
	// ErrEmptyData 文件数据为空
	ErrEmptyData = errors.New("文件数据为空")

	// ErrBuildPathFailed 构建文件路径失败
	ErrBuildPathFailed = errors.New("构建文件路径失败")
)

