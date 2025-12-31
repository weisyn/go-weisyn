// Package format 提供通用格式化工具函数
package format

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ParseContentHash 解析内容哈希（32字节）
//
// 功能：
//   - 解析纯十六进制字符串（64个字符）
//   - 严格校验 32 字节长度
//   - 返回解析后的字节数组
//
// 使用场景：
//   - 资源下载命令
//   - 合约调用命令
//   - 其他需要 contentHash 输入的场景
//
// 参数：
//   - hashStr: 64位十六进制字符串（纯hex，不带前缀）
//
// 返回：
//   - []byte: 解析后的 32 字节数组
//   - error: 解析错误或长度校验失败
//
// 注意：
//   - 为了兼容性，仍支持 0x 前缀，但不推荐使用
//   - 系统标准是纯十六进制格式，无前缀
func ParseContentHash(hashStr string) ([]byte, error) {
	// 去除可能的 0x 前缀（兼容性支持，但不推荐）
	hashStr = strings.TrimPrefix(hashStr, "0x")
	hashStr = strings.TrimPrefix(hashStr, "0X")

	// 解码 hex 字符串
	contentHash, err := hex.DecodeString(hashStr)
	if err != nil {
		return nil, fmt.Errorf("无效的hex格式: %w", err)
	}

	// 验证长度（32字节）
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("内容哈希必须是32字节，当前: %d字节", len(contentHash))
	}

	return contentHash, nil
}

// FormatFileSize 格式化文件大小为人类可读格式
//
// 功能：
//   - 自动选择合适的单位（B/KB/MB/GB）
//   - 保留两位小数
//
// 参数：
//   - size: 文件大小（字节）
//
// 返回：
//   - string: 格式化后的文件大小字符串
func FormatFileSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// FormatAddress 格式化地址（截断中间部分）
//
// 功能：
//   - 保留地址的前后部分
//   - 中间使用省略号
//
// 参数：
//   - address: 完整地址
//   - prefixLen: 前缀长度
//   - suffixLen: 后缀长度
//
// 返回：
//   - string: 格式化后的地址
func FormatAddress(address string, prefixLen, suffixLen int) string {
	if len(address) <= prefixLen+suffixLen {
		return address
	}
	return address[:prefixLen] + "..." + address[len(address)-suffixLen:]
}

// FormatHash 格式化哈希（十六进制字节数组）
//
// 功能：
//   - 将字节数组转换为十六进制字符串
//
// 参数：
//   - hash: 哈希字节数组
//
// 返回：
//   - string: 十六进制字符串（无0x前缀）
func FormatHash(hash []byte) string {
	return hex.EncodeToString(hash)
}

// FormatHashShort 格式化哈希（短格式）
//
// 功能：
//   - 将字节数组转换为十六进制字符串
//   - 只显示前后各 n 个字符
//
// 参数：
//   - hash: 哈希字节数组
//   - prefixLen: 前缀字符数
//   - suffixLen: 后缀字符数
//
// 返回：
//   - string: 十六进制字符串（截断）
func FormatHashShort(hash []byte, prefixLen, suffixLen int) string {
	full := hex.EncodeToString(hash)
	return FormatAddress(full, prefixLen, suffixLen)
}

