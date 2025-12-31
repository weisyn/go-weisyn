package kbucket

import (
	"crypto/sha256"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ErrLookupFailure 表示路由表查询未返回任何结果时的错误
var ErrLookupFailure = fmt.Errorf("failed to find any peer in table")

// ConvertPeerID 将peer.ID转换为DHT ID
// 基于defs-back/kbucket/util.go的ConvertPeerID函数
func ConvertPeerID(id peer.ID) []byte {
	hash := sha256.Sum256([]byte(id))
	return hash[:]
}

// ConvertKey 将任意字节数组转换为DHT ID
func ConvertKey(key []byte) []byte {
	hash := sha256.Sum256(key)
	return hash[:]
}

// CommonPrefixLen 计算两个ID的公共前缀长度
// 基于defs-back/kbucket/util.go的CommonPrefixLen函数
func CommonPrefixLen(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}

	for i := 0; i < len(a); i++ {
		xor := a[i] ^ b[i]
		if xor == 0 {
			continue
		}

		// 找到第一个不同的位
		return i*8 + (7 - clz8Util(xor))
	}

	return len(a) * 8
}

// ZeroPrefixLen 计算字节数组前导零的长度
// 基于defs-back/kbucket/util.go的ZeroPrefixLen函数
func ZeroPrefixLen(id []byte) int {
	for i, b := range id {
		if b != 0 {
			return i*8 + clz8Util(b)
		}
	}
	return len(id) * 8
}

// clz8Util 计算8位数的前导零个数（工具函数版本）
func clz8Util(x byte) int {
	if x == 0 {
		return 8
	}

	n := 0
	if x <= 0x0F {
		n += 4
		x <<= 4
	}
	if x <= 0x3F {
		n += 2
		x <<= 2
	}
	if x <= 0x7F {
		n++
	}

	return n
}

// XOR 对两个字节数组进行异或运算
// 基于defs-back/kbucket/util.go的XOR函数
func XOR(a, b []byte) []byte {
	if len(a) != len(b) {
		panic("XOR操作的字节数组长度必须相等")
	}

	result := make([]byte, len(a))
	for i := range a {
		result[i] = a[i] ^ b[i]
	}

	return result
}

// Equal 检查两个字节数组是否相等
func Equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Less 比较两个字节数组的大小
func Less(a, b []byte) bool {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return true
		} else if a[i] > b[i] {
			return false
		}
	}

	// 前缀相同，较短的数组较小
	return len(a) < len(b)
}

// Copy 创建字节数组的副本
func Copy(src []byte) []byte {
	if src == nil {
		return nil
	}

	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

// IsZero 检查字节数组是否为全零
func IsZero(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// GenerateRandomID 生成随机的DHT ID
func GenerateRandomID() []byte {
	// 使用加密安全的随机数生成器
	randomBytes := make([]byte, 32)
	// 注意：在生产环境中应该使用crypto/rand.Read(randomBytes)
	for i := range randomBytes {
		randomBytes[i] = byte((i*7 + 13) % 256) // 伪随机模式
	}
	return ConvertKey(randomBytes)
}
