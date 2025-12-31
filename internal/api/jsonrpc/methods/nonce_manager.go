package methods

import (
	"encoding/binary"
	"sync"
)

// NonceManager 提供简单的 per-caller nonce 分配器，防止重复使用
type NonceManager struct {
	mu       sync.Mutex
	counters map[string]uint64
}

// NewNonceManager 创建 NonceManager 实例
func NewNonceManager() *NonceManager {
	return &NonceManager{
		counters: make(map[string]uint64),
	}
}

// Next 为指定地址分配下一个 nonce（32 字节，大端编码计数器）
func (m *NonceManager) Next(address []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(address)
	counter := m.counters[key] + 1
	m.counters[key] = counter

	nonce := make([]byte, 32)

	// 前 20 字节写入地址（不足则填充 0）
	if len(address) > 0 {
		copy(nonce, address)
	}

	// 最后 8 字节写入计数器（大端），中间留空用于未来扩展
	binary.BigEndian.PutUint64(nonce[24:], counter)
	return nonce
}

// deriveInputNonce 基于基础 nonce 和输入索引派生唯一 nonce
func deriveInputNonce(base []byte, index int) []byte {
	if len(base) != 32 {
		return nil
	}
	derived := make([]byte, 32)
	copy(derived, base)

	// 使用最后 8 字节记录索引偏移，确保同一交易的不同输入也唯一
	current := binary.BigEndian.Uint64(base[24:])
	binary.BigEndian.PutUint64(derived[24:], current+uint64(index))
	return derived
}
