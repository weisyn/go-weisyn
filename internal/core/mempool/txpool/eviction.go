// 文件说明：
// 本文件定义交易淘汰策略接口及多种实现（基于优先级、时间、大小与混合策略）。
// 职责限定：仅提供在内存受限时的候选交易选择规则，不负责实际移除。
package txpool

import (
	"sort"
	"time"
)

// EvictionPolicy 交易淘汰策略接口
// 说明：根据所需释放空间选择应淘汰的交易ID列表（不直接修改池）。
type EvictionPolicy interface {
	// SelectTransactionsToEvict 选择要淘汰的交易
	// 参数:
	//   txs: 所有交易的列表
	//   requiredSpace: 需要释放的字节数
	// 返回:
	//   要淘汰的交易ID列表
	SelectTransactionsToEvict(txs []*TxWrapper, requiredSpace uint64) [][]byte
}

// PriorityBasedEviction 基于优先级的淘汰策略
// 综合考虑类型、大小、时间、依赖等因素的权重。
type PriorityBasedEviction struct {
	typeWeight       float64 // 交易类型权重
	sizeWeight       float64 // 大小权重
	timeWeight       float64 // 时间权重
	dependencyWeight float64 // 依赖权重
}

// NewPriorityBasedEviction 创建基于优先级的淘汰策略。
func NewPriorityBasedEviction(typeWeight, sizeWeight, timeWeight, dependencyWeight float64) *PriorityBasedEviction {
	return &PriorityBasedEviction{typeWeight: typeWeight, sizeWeight: sizeWeight, timeWeight: timeWeight, dependencyWeight: dependencyWeight}
}

// SelectTransactionsToEvict 选择要淘汰的交易。
func (p *PriorityBasedEviction) SelectTransactionsToEvict(txs []*TxWrapper, requiredSpace uint64) [][]byte {
	if requiredSpace == 0 {
		return [][]byte{}
	}
	pendingTxs := make([]*TxWrapper, 0)
	for _, tx := range txs {
		if tx.Status == TxStatusPending {
			pendingTxs = append(pendingTxs, tx)
		}
	}
	sortedTxs := p.scoreAndSortTransactions(pendingTxs)
	evictTxIDs := make([][]byte, 0)
	var freedSpace uint64 = 0
	for _, tx := range sortedTxs {
		txSize := uint64(calculateTransactionSize(tx.Tx))
		evictTxIDs = append(evictTxIDs, tx.TxID)
		freedSpace += txSize
		if freedSpace >= requiredSpace {
			break
		}
	}
	return evictTxIDs
}

// 对交易进行评分并排序（评分越低越易淘汰）。
func (p *PriorityBasedEviction) scoreAndSortTransactions(txs []*TxWrapper) []*TxWrapper {
	sortedTxs := make([]*TxWrapper, len(txs))
	copy(sortedTxs, txs)
	now := time.Now()
	sort.Slice(sortedTxs, func(i, j int) bool {
		txI := sortedTxs[i]
		scoreI := p.calculateTransactionScore(txI, now)
		txJ := sortedTxs[j]
		scoreJ := p.calculateTransactionScore(txJ, now)
		return scoreI < scoreJ
	})
	return sortedTxs
}

// 计算交易的综合评分。
func (p *PriorityBasedEviction) calculateTransactionScore(tx *TxWrapper, now time.Time) float64 {
	// 交易类型得分（系统交易不易被淘汰）
	typeScore := float64(0)
	switch tx.TxType {
	case TxTypeSystem:
		typeScore = 4.0 * p.typeWeight
	case TxTypeContract:
		typeScore = 3.0 * p.typeWeight
	case TxTypeResource:
		typeScore = 2.0 * p.typeWeight
	case TxTypeNormal:
		typeScore = 1.0 * p.typeWeight
	}

	// 大小得分（大交易更易被淘汰）
	sizeScore := (1.0 / float64(tx.Size)) * p.sizeWeight * 1000.0

	// 时间得分（老交易更易被淘汰）
	timeInPool := now.Sub(tx.ReceivedAt).Seconds()
	timeScore := (1.0 / (timeInPool + 1.0)) * p.timeWeight * 100.0

	// 依赖得分（被依赖多的交易不易被淘汰）
	dependencyScore := float64(tx.DependentCount) * p.dependencyWeight

	return typeScore + sizeScore + timeScore + dependencyScore
}

// TimeBasedEviction 基于时间的淘汰策略
// 先淘汰超过最长停留时间的交易，不足部分按最早接收优先。
type TimeBasedEviction struct{ maxTimeInPool int64 }

// NewTimeBasedEviction 创建新的基于时间的淘汰策略。
func NewTimeBasedEviction(maxTimeInPool int64) *TimeBasedEviction {
	return &TimeBasedEviction{maxTimeInPool: maxTimeInPool}
}

// SelectTransactionsToEvict 选择要淘汰的交易。
func (t *TimeBasedEviction) SelectTransactionsToEvict(txs []*TxWrapper, requiredSpace uint64) [][]byte {
	if requiredSpace == 0 {
		return [][]byte{}
	}
	now := time.Now()
	threshold := now.Add(-time.Duration(t.maxTimeInPool) * time.Second)
	expiredTxIDs := make([][]byte, 0)
	var freedSpace uint64 = 0
	for _, tx := range txs {
		if tx.Status == TxStatusPending && tx.ReceivedAt.Before(threshold) {
			txSize := uint64(calculateTransactionSize(tx.Tx))
			expiredTxIDs = append(expiredTxIDs, tx.TxID)
			freedSpace += txSize
		}
	}
	if freedSpace < requiredSpace {
		pendingTxs := make([]*TxWrapper, 0)
		for _, tx := range txs {
			if tx.Status == TxStatusPending && !tx.ReceivedAt.Before(threshold) {
				pendingTxs = append(pendingTxs, tx)
			}
		}
		sort.Slice(pendingTxs, func(i, j int) bool { return pendingTxs[i].ReceivedAt.Before(pendingTxs[j].ReceivedAt) })
		for _, tx := range pendingTxs {
			if freedSpace >= requiredSpace {
				break
			}
			txSize := uint64(calculateTransactionSize(tx.Tx))
			expiredTxIDs = append(expiredTxIDs, tx.TxID)
			freedSpace += txSize
		}
	}
	return expiredTxIDs
}

// SizeBasedEviction 基于大小的淘汰策略
// 优先淘汰占用内存较大的交易。
type SizeBasedEviction struct{}

// NewSizeBasedEviction 创建新的基于大小的淘汰策略。
func NewSizeBasedEviction() *SizeBasedEviction { return &SizeBasedEviction{} }

// SelectTransactionsToEvict 选择要淘汰的交易。
func (s *SizeBasedEviction) SelectTransactionsToEvict(txs []*TxWrapper, requiredSpace uint64) [][]byte {
	if requiredSpace == 0 {
		return [][]byte{}
	}
	pendingTxs := make([]*TxWrapper, 0)
	for _, tx := range txs {
		if tx.Status == TxStatusPending {
			pendingTxs = append(pendingTxs, tx)
		}
	}
	sort.Slice(pendingTxs, func(i, j int) bool {
		return calculateTransactionSize(pendingTxs[i].Tx) > calculateTransactionSize(pendingTxs[j].Tx)
	})
	evictTxIDs := make([][]byte, 0)
	var freedSpace uint64 = 0
	for _, tx := range pendingTxs {
		txSize := uint64(calculateTransactionSize(tx.Tx))
		evictTxIDs = append(evictTxIDs, tx.TxID)
		freedSpace += txSize
		if freedSpace >= requiredSpace {
			break
		}
	}
	return evictTxIDs
}

// HybridEviction 混合策略：综合三种策略的投票权重。
type HybridEviction struct {
	priorityWeight   float64
	timeWeight       float64
	sizeWeight       float64
	priorityStrategy *PriorityBasedEviction
	timeStrategy     *TimeBasedEviction
	sizeStrategy     *SizeBasedEviction
}

// NewHybridEviction 创建混合策略。
func NewHybridEviction(priorityWeight, timeWeight, sizeWeight float64, maxTimeInPool int64) *HybridEviction {
	return &HybridEviction{
		priorityWeight:   priorityWeight,
		timeWeight:       timeWeight,
		sizeWeight:       sizeWeight,
		priorityStrategy: NewPriorityBasedEviction(0.4, 0.2, 0.2, 0.2),
		timeStrategy:     NewTimeBasedEviction(maxTimeInPool),
		sizeStrategy:     NewSizeBasedEviction(),
	}
}

// SelectTransactionsToEvict 选择要淘汰的交易。
func (h *HybridEviction) SelectTransactionsToEvict(txs []*TxWrapper, requiredSpace uint64) [][]byte {
	if requiredSpace == 0 {
		return [][]byte{}
	}
	votes := make(map[string]float64)
	priorityEvictions := h.priorityStrategy.SelectTransactionsToEvict(txs, requiredSpace)
	timeEvictions := h.timeStrategy.SelectTransactionsToEvict(txs, requiredSpace)
	sizeEvictions := h.sizeStrategy.SelectTransactionsToEvict(txs, requiredSpace)
	for i, txID := range priorityEvictions {
		txIDStr := string(txID)
		weight := h.priorityWeight * (1.0 - float64(i)/float64(len(priorityEvictions)))
		votes[txIDStr] += weight
	}
	for i, txID := range timeEvictions {
		txIDStr := string(txID)
		weight := h.timeWeight * (1.0 - float64(i)/float64(len(timeEvictions)))
		votes[txIDStr] += weight
	}
	for i, txID := range sizeEvictions {
		txIDStr := string(txID)
		weight := h.sizeWeight * (1.0 - float64(i)/float64(len(sizeEvictions)))
		votes[txIDStr] += weight
	}
	type voteEntry struct {
		txID   string
		weight float64
	}
	voteList := make([]voteEntry, 0, len(votes))
	for txID, weight := range votes {
		voteList = append(voteList, voteEntry{txID: txID, weight: weight})
	}
	sort.Slice(voteList, func(i, j int) bool { return voteList[i].weight > voteList[j].weight })
	evictTxIDs := make([][]byte, 0)
	var freedSpace uint64 = 0
	txSizeMap := make(map[string]uint64)
	for _, tx := range txs {
		txSizeMap[string(tx.TxID)] = uint64(calculateTransactionSize(tx.Tx))
	}
	for _, entry := range voteList {
		if freedSpace >= requiredSpace {
			break
		}
		if size, exists := txSizeMap[entry.txID]; exists {
			evictTxIDs = append(evictTxIDs, []byte(entry.txID))
			freedSpace += size
		}
	}
	return evictTxIDs
}
