package keyspace

import (
	"math/big"
	"sort"
)

// Key 表示KeySpace中的标识符
// 严格按照defs-back/kbucket/keyspace/keyspace.go的结构
type Key struct {
	// Space 是与此Key相关联的KeySpace
	Space KeySpace

	// Original 是标识符的原始值
	Original []byte

	// Bytes 是标识符在KeySpace中的新值
	Bytes []byte
}

// KeySpace 定义了键空间接口
type KeySpace interface {
	// Key 将原始标识符转换为此空间中的键
	Key(id []byte) Key

	// Equal 判断两个键在此键空间中是否相等
	Equal(k1, k2 Key) bool

	// Distance 计算两个键之间的距离
	Distance(k1, k2 Key) *big.Int

	// Less 判断第一个键是否在第二个键之前
	Less(k1, k2 Key) bool
}

// Equal 判断此Key是否与另一个Key相等
// 如果两个Key不在同一个KeySpace中，会触发panic
func (k1 Key) Equal(k2 Key) bool {
	if k1.Space != k2.Space {
		panic("k1和k2不在同一个KeySpace中")
	}
	return k1.Space.Equal(k1, k2)
}

// Less 判断此Key是否在另一个Key之前
// 如果两个Key不在同一个KeySpace中，会触发panic
func (k1 Key) Less(k2 Key) bool {
	if k1.Space != k2.Space {
		panic("k1和k2不在同一个KeySpace中")
	}
	return k1.Space.Less(k1, k2)
}

// Distance 计算此Key与另一个Key之间的距离
// 如果两个Key不在同一个KeySpace中，会触发panic
func (k1 Key) Distance(k2 Key) *big.Int {
	if k1.Space != k2.Space {
		panic("k1和k2不在同一个KeySpace中")
	}
	return k1.Space.Distance(k1, k2)
}

// SortByDistance 根据距离目标键的远近对键数组进行排序
func SortByDistance(space KeySpace, target Key, list []Key) {
	sort.Slice(list, func(i, j int) bool {
		a := list[i].Distance(target)
		b := list[j].Distance(target)
		return a.Cmp(b) == -1
	})
}

// CommonPrefixLength 计算两个字节数组的公共前缀长度
// 这是Kademlia距离计算的核心函数
func CommonPrefixLength(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}

	for i := 0; i < len(a); i++ {
		xor := a[i] ^ b[i]
		if xor == 0 {
			continue
		}

		// 找到第一个不同的位
		return i*8 + clz8(xor)
	}

	return len(a) * 8
}

// clz8 计算8位数的前导零个数
func clz8(x byte) int {
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

// ZeroPrefixLength 计算字节数组前导零的长度
func ZeroPrefixLength(id []byte) int {
	for i, b := range id {
		if b != 0 {
			return i*8 + clz8(b)
		}
	}
	return len(id) * 8
}
