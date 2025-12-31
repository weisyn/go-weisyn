package keyspace

import (
	"bytes"
	"crypto/subtle"
	"math/big"
	"math/bits"

	"crypto/sha256"
)

// XORKeySpace 是一个基于异或操作的键空间实现
// 严格按照defs-back/kbucket/keyspace/xor.go的实现
var XORKeySpace = &xorKeySpace{}

// 确保xorKeySpace实现了KeySpace接口
var _ KeySpace = XORKeySpace

// xorKeySpace 实现了基于异或操作的键空间
type xorKeySpace struct{}

// Key 将原始标识符转换为此空间中的键
func (s *xorKeySpace) Key(id []byte) Key {
	hash := sha256.Sum256(id) // 使用SHA-256对输入标识符进行哈希
	key := hash[:]            // 将哈希结果转换为字节切片
	return Key{
		Space:    s,   // 键空间引用
		Original: id,  // 原始标识符
		Bytes:    key, // 规范化后的字节表示
	}
}

// Equal 判断两个键在此键空间中是否相等
func (s *xorKeySpace) Equal(k1, k2 Key) bool {
	return subtle.ConstantTimeCompare(k1.Bytes, k2.Bytes) == 1
}

// Distance 计算两个键之间的XOR距离
// 这是Kademlia协议的核心算法
func (s *xorKeySpace) Distance(k1, k2 Key) *big.Int {
	// 对两个键的字节表示进行异或操作
	a := k1.Bytes
	b := k2.Bytes

	// 确保长度相等
	if len(a) != len(b) {
		panic("键长度不相等")
	}

	// 计算XOR距离
	distance := make([]byte, len(a))
	for i := range a {
		distance[i] = a[i] ^ b[i]
	}

	// 将距离转换为big.Int并返回
	return new(big.Int).SetBytes(distance)
}

// Less 判断第一个键是否在第二个键之前
func (s *xorKeySpace) Less(k1, k2 Key) bool {
	return bytes.Compare(k1.Bytes, k2.Bytes) == -1
}

// XORDistance 计算两个字节数组的XOR距离（快速版本）
func XORDistance(a, b []byte) []byte {
	if len(a) != len(b) {
		panic("字节数组长度不相等")
	}

	distance := make([]byte, len(a))
	for i := range a {
		distance[i] = a[i] ^ b[i]
	}

	return distance
}

// FastCommonPrefixLength 快速计算公共前缀长度
// 使用位操作优化性能
func FastCommonPrefixLength(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}

	prefixLen := 0

	// 按64位块处理（在64位系统上更快）
	for i := 0; i+8 <= len(a); i += 8 {
		// 将8字节转换为uint64进行比较
		aVal := uint64(a[i])<<56 | uint64(a[i+1])<<48 | uint64(a[i+2])<<40 | uint64(a[i+3])<<32 |
			uint64(a[i+4])<<24 | uint64(a[i+5])<<16 | uint64(a[i+6])<<8 | uint64(a[i+7])
		bVal := uint64(b[i])<<56 | uint64(b[i+1])<<48 | uint64(b[i+2])<<40 | uint64(b[i+3])<<32 |
			uint64(b[i+4])<<24 | uint64(b[i+5])<<16 | uint64(b[i+6])<<8 | uint64(b[i+7])

		if aVal == bVal {
			prefixLen += 64
			continue
		}

		// 找到第一个不同的位
		xor := aVal ^ bVal
		prefixLen += bits.LeadingZeros64(xor)
		return prefixLen
	}

	// 处理剩余字节
	for i := (len(a) / 8) * 8; i < len(a); i++ {
		if a[i] == b[i] {
			prefixLen += 8
			continue
		}

		xor := a[i] ^ b[i]
		prefixLen += bits.LeadingZeros8(xor)
		return prefixLen
	}

	return prefixLen
}

// ComputeDistance 计算两个键的距离并返回big.Int
func ComputeDistance(k1, k2 Key) *big.Int {
	return XORKeySpace.Distance(k1, k2)
}
