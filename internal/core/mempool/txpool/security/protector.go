// Package security provides transaction pool security protection functionality.
package security

import (
	"fmt"
	"sync"
)

// TxPoolProtector 交易池保护器，用于防止交易池被填满
type TxPoolProtector struct {
	mu            sync.RWMutex
	txCount       map[string]int
	maxTxsPerPeer int
	maxTxsTotal   int
}

// NewTxPoolProtector 创建新的交易池保护器
func NewTxPoolProtector(maxTxsPerPeer, maxTxsTotal int) *TxPoolProtector {
	return &TxPoolProtector{
		txCount:       make(map[string]int),
		maxTxsPerPeer: maxTxsPerPeer,
		maxTxsTotal:   maxTxsTotal,
	}
}

// CheckTransaction 检查交易是否允许进入交易池
func (tpp *TxPoolProtector) CheckTransaction(peerID string) error {
	tpp.mu.Lock()
	defer tpp.mu.Unlock()

	// 检查单节点交易数
	if tpp.txCount[peerID] >= tpp.maxTxsPerPeer {
		return fmt.Errorf("单节点交易数已达上限: %d/%d",
			tpp.txCount[peerID], tpp.maxTxsPerPeer)
	}

	// 检查交易池总交易数
	totalTxs := 0
	for _, count := range tpp.txCount {
		totalTxs += count
	}

	if totalTxs >= tpp.maxTxsTotal {
		return fmt.Errorf("交易池已满: %d/%d", totalTxs, tpp.maxTxsTotal)
	}

	return nil
}

// AddTransaction 添加交易
func (tpp *TxPoolProtector) AddTransaction(peerID string) error {
	tpp.mu.Lock()
	defer tpp.mu.Unlock()

	// 检查单节点交易数
	if tpp.txCount[peerID] >= tpp.maxTxsPerPeer {
		return fmt.Errorf("单节点交易数已达上限: %d/%d",
			tpp.txCount[peerID], tpp.maxTxsPerPeer)
	}

	// 检查交易池总交易数
	totalTxs := 0
	for _, count := range tpp.txCount {
		totalTxs += count
	}

	if totalTxs >= tpp.maxTxsTotal {
		return fmt.Errorf("交易池已满: %d/%d", totalTxs, tpp.maxTxsTotal)
	}

	// 添加交易
	tpp.txCount[peerID]++

	return nil
}

// RemoveTransaction 移除交易
func (tpp *TxPoolProtector) RemoveTransaction(peerID string) {
	tpp.mu.Lock()
	defer tpp.mu.Unlock()

	if count := tpp.txCount[peerID]; count > 0 {
		tpp.txCount[peerID]--
		if tpp.txCount[peerID] == 0 {
			delete(tpp.txCount, peerID)
		}
	}
}

// GetTransactionCount 获取交易数量
func (tpp *TxPoolProtector) GetTransactionCount(peerID string) int {
	tpp.mu.RLock()
	defer tpp.mu.RUnlock()

	return tpp.txCount[peerID]
}

// GetTotalTransactionCount 获取总交易数量
func (tpp *TxPoolProtector) GetTotalTransactionCount() int {
	tpp.mu.RLock()
	defer tpp.mu.RUnlock()

	total := 0
	for _, count := range tpp.txCount {
		total += count
	}

	return total
}

// GetUsageRate 获取使用率
func (tpp *TxPoolProtector) GetUsageRate() float64 {
	tpp.mu.RLock()
	defer tpp.mu.RUnlock()

	total := 0
	for _, count := range tpp.txCount {
		total += count
	}

	if tpp.maxTxsTotal == 0 {
		return 0.0
	}

	return float64(total) / float64(tpp.maxTxsTotal)
}

// Reset 重置交易计数
func (tpp *TxPoolProtector) Reset(peerID string) {
	tpp.mu.Lock()
	defer tpp.mu.Unlock()

	delete(tpp.txCount, peerID)
}

// ResetAll 重置所有交易计数
func (tpp *TxPoolProtector) ResetAll() {
	tpp.mu.Lock()
	defer tpp.mu.Unlock()

	tpp.txCount = make(map[string]int)
}
