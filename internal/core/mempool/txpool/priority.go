// 文件说明：
// 本文件定义交易池内部使用的状态枚举、交易包装器与优先级队列，
// 以及与存储与排序相关的辅助函数（费用率提取、大小估算、优先级计算器等）。
// 职责限定：仅服务于 TxPool 的存储/排序，不涉及业务域的费用/签名/UTXO验证。
package txpool

import (
	"container/heap"
	"encoding/binary"
	"math"
	"strconv"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TxStatus 表示交易在池中的状态
// 取值：Pending/Rejected/Confirmed/Expired/Mining。
type TxStatus int

// 交易状态常量
const (
	TxStatusPending        TxStatus = iota // 待处理
	TxStatusRejected                       // 已拒绝
	TxStatusConfirmed                      // 已确认
	TxStatusExpired                        // 已过期
	TxStatusMining                         // 挖矿中
	TxStatusPendingConfirm                 // 待确认（已挖出区块，等待网络确认）
)

// 交易排序策略（供外部选择排序策略时使用）
const (
	SortByTime     = "time"     // 按时间排序（先进先出）
	SortBySize     = "size"     // 按交易大小排序（小交易优先）
	SortByType     = "type"     // 按交易类型排序（系统交易优先）
	SortByPriority = "priority" // 按优先级排序（综合考虑时间、大小、类型等）
)

// TxWrapper 交易包装器，包含交易与排序所需元数据。
// 字段：
// - Tx：交易对象；
// - TxID：交易ID；
// - ReceivedAt：接收时间；
// - Status：池内状态；
// - Priority：优先级（用于队列排序）；
// - Size：交易大小（字节）；
// - TxType：交易类型（普通/系统/合约等）；
// - DependentCount：被依赖数量（用于排序倾向）；
// - index：优先队列内部索引。
// TxType 交易类型枚举
type TxType int

const (
	TxTypeNormal   TxType = iota // 普通交易
	TxTypeSystem                 // 系统交易（优先级最高）
	TxTypeContract               // 合约交易
	TxTypeResource               // 资源交易
)

type TxWrapper struct {
	Tx             *transaction.Transaction
	TxID           []byte
	ReceivedAt     time.Time
	Status         TxStatus
	Priority       int32
	Size           uint64 // 交易大小（字节）
	TxType         TxType // 交易类型
	DependentCount int
	index          int
}

// PriorityQueue 交易优先级队列实现。
type PriorityQueue []*TxWrapper

// Len 队列长度。
func (pq PriorityQueue) Len() int { return len(pq) }

// Less 比较器：优先级高的排在前面。
func (pq PriorityQueue) Less(i, j int) bool { return pq[i].Priority > pq[j].Priority }

// Swap 交换两个元素。
func (pq PriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i]; pq[i].index = i; pq[j].index = j }

// Push 添加元素到队列末尾。
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*TxWrapper)
	item.index = n
	*pq = append(*pq, item)
}

// Pop 移除并返回队列中最低优先级的元素。
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// Update 更新队列中元素的优先级并重新排序。
func (pq *PriorityQueue) Update(item *TxWrapper, priority int32) {
	item.Priority = priority
	heap.Fix(pq, item.index)
}

// Remove 从队列中移除指定元素。
func (pq *PriorityQueue) Remove(item *TxWrapper) {
	if item.index >= 0 && item.index < pq.Len() {
		heap.Remove(pq, item.index)
	}
}

// Top 获取队列中优先级最高的元素但不移除。
func (pq *PriorityQueue) Top() *TxWrapper {
	if pq.Len() == 0 {
		return nil
	}
	return (*pq)[0]
}

// Copy 创建队列的深拷贝（用于只读遍历场景）。
func (pq *PriorityQueue) Copy() *PriorityQueue {
	newPQ := make(PriorityQueue, pq.Len())
	copy(newPQ, *pq)
	return &newPQ
}

// NewPriorityQueue 创建新的优先级队列。
func NewPriorityQueue() *PriorityQueue { pq := make(PriorityQueue, 0); heap.Init(&pq); return &pq }

// NewTxWrapper 创建新的交易包装器。
// 参数：
// - tx：交易对象；
// - txID：交易ID。
// 返回：*TxWrapper。
func NewTxWrapper(tx *transaction.Transaction, txID []byte) *TxWrapper {
	return &TxWrapper{
		Tx:         tx,
		TxID:       txID,
		ReceivedAt: time.Now(),
		Status:     TxStatusPending,
		Priority:   0,
		Size:       calculateTransactionSize(tx),
		TxType:     determineTransactionType(tx),
	}
}

// calculateTransactionSize 计算交易的字节大小
func calculateTransactionSize(tx *transaction.Transaction) uint64 {
	if tx == nil {
		return 0
	}

	// 简单估算：输入数量 * 180 + 输出数量 * 34 + 基础开销
	baseSize := uint64(100) // 基础开销
	inputSize := uint64(len(tx.Inputs)) * 180
	outputSize := uint64(len(tx.Outputs)) * 34

	return baseSize + inputSize + outputSize
}

// determineTransactionType 确定交易类型
func determineTransactionType(tx *transaction.Transaction) TxType {
	if tx == nil {
		return TxTypeNormal
	}

	// 根据交易特征判断类型
	for _, output := range tx.Outputs {
		if output.GetResource() != nil {
			return TxTypeResource
		}
		if output.GetState() != nil {
			return TxTypeContract
		}
	}

	// 检查是否有复杂的费用机制（可能是系统交易）
	if tx.GetFeeMechanism() != nil {
		return TxTypeSystem
	}

	return TxTypeNormal
}

// PriorityCalculator 交易优先级计算器（基于技术指标，不涉及业务费用）
type PriorityCalculator struct {
	typeWeight     float64 // 交易类型权重
	timeWeight     float64 // 时间权重
	sizeWeight     float64 // 大小权重
	metadataWeight float64 // 元数据权重
}

// NewPriorityCalculator 创建优先级计算器
func NewPriorityCalculator() *PriorityCalculator {
	return &PriorityCalculator{
		typeWeight:     1000.0, // 交易类型最重要
		timeWeight:     100.0,  // 时间次重要（FIFO）
		sizeWeight:     50.0,   // 大小权重（小交易优先）
		metadataWeight: 200.0,  // 元数据权重（用户指定优先级）
	}
}

// CalculatePriority 计算交易优先级得分（基于技术指标）
func (pc *PriorityCalculator) CalculatePriority(wrapper *TxWrapper) int32 {
	if wrapper == nil || wrapper.Tx == nil {
		return 0
	}

	var totalScore float64

	// 1. 交易类型得分（系统交易优先级最高）
	typeScore := float64(0)
	switch wrapper.TxType {
	case TxTypeSystem:
		typeScore = 4 * pc.typeWeight
	case TxTypeContract:
		typeScore = 3 * pc.typeWeight
	case TxTypeResource:
		typeScore = 2 * pc.typeWeight
	case TxTypeNormal:
		typeScore = 1 * pc.typeWeight
	}

	// 2. 时间得分（FIFO - 先进先出）
	timeInPool := time.Since(wrapper.ReceivedAt).Seconds()
	timeScore := math.Min(timeInPool/60.0, 60.0) * pc.timeWeight // 最多等待60分钟

	// 3. 大小得分（小交易优先）
	sizeScore := float64(0)
	if wrapper.Size > 0 {
		// 大小越小，得分越高
		sizeScore = (10000.0 / float64(wrapper.Size)) * pc.sizeWeight
	}

	// 4. 元数据得分（用户指定的优先级）
	metadataScore := pc.extractMetadataPriority(wrapper.Tx)

	// 5. 依赖得分（被依赖越少越优先）
	dependencyScore := float64(-wrapper.DependentCount) * 10.0

	totalScore = typeScore + timeScore + sizeScore + metadataScore + dependencyScore

	// 确保得分在有效范围内
	if totalScore > float64(math.MaxInt32) {
		return math.MaxInt32
	} else if totalScore < float64(math.MinInt32) {
		return math.MinInt32
	}

	return int32(totalScore)
}

// extractMetadataPriority 从交易元数据中提取用户指定的优先级
func (pc *PriorityCalculator) extractMetadataPriority(tx *transaction.Transaction) float64 {
	if tx.Metadata == nil {
		return 0
	}

	// 从自定义字段中提取优先级
	if priorityBytes, ok := tx.Metadata.CustomFields["priority"]; ok {
		if priority, err := strconv.ParseUint(string(priorityBytes), 10, 32); err == nil {
			return float64(priority) * pc.metadataWeight
		}
	}

	// 从结构化数据中提取优先级
	if structuredData := tx.Metadata.GetStructuredData(); structuredData != nil {
		if priorityBytes, ok := structuredData.Fields["priority"]; ok && len(priorityBytes) >= 4 {
			metaPriority := binary.LittleEndian.Uint32(priorityBytes)
			return float64(metaPriority) * pc.metadataWeight
		}
	}

	return 0
}
