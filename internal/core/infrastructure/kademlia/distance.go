package kbucket

import (
	"crypto/sha256"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/libp2p/go-libp2p/core/peer"
)

// XORDistanceCalculator XOR距离计算器
// 实现Kademlia协议的核心距离计算算法
type XORDistanceCalculator struct {
	logger log.Logger
}

// NewXORDistanceCalculator 创建XOR距离计算器
func NewXORDistanceCalculator(logger log.Logger) kademlia.DistanceCalculator {
	return &XORDistanceCalculator{
		logger: logger,
	}
}

// Distance 计算两个节点之间的XOR距离
func (calc *XORDistanceCalculator) Distance(a, b peer.ID) []byte {
	// 将peer.ID转换为DHT ID
	aHash := sha256.Sum256([]byte(a))
	bHash := sha256.Sum256([]byte(b))

	// 计算XOR距离
	distance := make([]byte, 32)
	for i := range distance {
		distance[i] = aHash[i] ^ bHash[i]
	}

	return distance
}

// DistanceToKey 计算节点到密钥的距离
func (calc *XORDistanceCalculator) DistanceToKey(peerID peer.ID, key []byte) []byte {
	// 将peer.ID转换为DHT ID
	peerHash := sha256.Sum256([]byte(peerID))

	// 确保key长度为32字节
	var keyHash [32]byte
	if len(key) == 32 {
		copy(keyHash[:], key)
	} else {
		keyHash = sha256.Sum256(key)
	}

	// 计算XOR距离
	distance := make([]byte, 32)
	for i := range distance {
		distance[i] = peerHash[i] ^ keyHash[i]
	}

	return distance
}

// Compare 比较两个距离
func (calc *XORDistanceCalculator) Compare(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}

	return len(a) - len(b)
}

// CommonPrefixLen 计算公共前缀长度
func (calc *XORDistanceCalculator) CommonPrefixLen(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}

	for i := 0; i < len(a); i++ {
		xor := a[i] ^ b[i]
		if xor == 0 {
			continue
		}

		// 找到第一个不同的位
		return i*8 + (7 - clz8(xor))
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
