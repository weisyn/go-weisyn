package builder

import (
	"errors"
	"fmt"
	"sort"
)

// UTXO 表示一个未花费交易输出
type UTXO struct {
	TxHash    string  // 交易哈希
	Vout      uint32  // 输出索引
	Amount    *Amount // 金额
	Address   string  // 地址
	ScriptPub []byte  // 锁定脚本
}

// UTXOSelector UTXO选择策略接口
type UTXOSelector interface {
	// Select 从UTXOs中选择足够支付目标金额（包括手续费）的UTXO集合
	//
	// 参数：
	//   utxos: 可用的UTXO列表
	//   targetAmount: 目标金额（转账金额+手续费）
	//
	// 返回：
	//   selected: 选中的UTXO列表
	//   totalAmount: 选中UTXO的总金额
	//   error: 如果余额不足或其他错误
	Select(utxos []UTXO, targetAmount *Amount) (selected []UTXO, totalAmount *Amount, err error)
}

var (
	// ErrInsufficientBalance 余额不足
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrNoUTXOs 没有可用的UTXO
	ErrNoUTXOs = errors.New("no available UTXOs")
)

// FirstFitSelector 第一个足够的选择策略
//
// 策略：
//  1. 优先尝试找到单个UTXO满足需求（最小化输入数量）
//  2. 如果没有单个UTXO足够，则按顺序累加直到满足需求
//
// 适用场景：
//   - 简单转账
//   - 优先使用大额UTXO
//   - 最小化交易大小
type FirstFitSelector struct{}

// NewFirstFitSelector 创建FirstFit选择器
func NewFirstFitSelector() UTXOSelector {
	return &FirstFitSelector{}
}

// Select 实现UTXOSelector接口
func (s *FirstFitSelector) Select(utxos []UTXO, targetAmount *Amount) ([]UTXO, *Amount, error) {
	if len(utxos) == 0 {
		return nil, nil, ErrNoUTXOs
	}

	if targetAmount == nil || targetAmount.IsZero() {
		return nil, nil, fmt.Errorf("invalid target amount")
	}

	// 策略1：尝试找单个UTXO满足需求
	for _, utxo := range utxos {
		if utxo.Amount.GreaterThanOrEqual(targetAmount) {
			return []UTXO{utxo}, utxo.Amount, nil
		}
	}

	// 策略2：累加UTXO直到满足需求
	return accumulateUTXOs(utxos, targetAmount)
}

// GreedySelector 贪心最少输入选择策略
//
// 策略：
//  1. 将UTXO按金额从大到小排序
//  2. 贪心选择，每次选择最大的UTXO
//  3. 直到总金额>=目标金额
//
// 适用场景：
//   - 最小化输入数量
//   - 快速凑够金额
//   - 不考虑找零优化
type GreedySelector struct{}

// NewGreedySelector 创建Greedy选择器
func NewGreedySelector() UTXOSelector {
	return &GreedySelector{}
}

// Select 实现UTXOSelector接口
func (s *GreedySelector) Select(utxos []UTXO, targetAmount *Amount) ([]UTXO, *Amount, error) {
	if len(utxos) == 0 {
		return nil, nil, ErrNoUTXOs
	}

	if targetAmount == nil || targetAmount.IsZero() {
		return nil, nil, fmt.Errorf("invalid target amount")
	}

	// 按金额从大到小排序（不修改原切片）
	sorted := make([]UTXO, len(utxos))
	copy(sorted, utxos)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount.GreaterThan(sorted[j].Amount)
	})

	// 贪心累加
	return accumulateUTXOs(sorted, targetAmount)
}

// BranchAndBoundSelector 分支定界选择策略（精确找零优化）
//
// 策略：
//  1. 尝试找到总金额恰好等于目标金额的UTXO组合（零找零）
//  2. 如果找不到，选择找零最小的组合
//  3. 限制搜索深度避免性能问题
//
// 适用场景：
//   - 隐私保护（避免找零暴露关联）
//   - 费用优化（减少找零输出）
//   - 高级用户场景
//
// 注意：这是一个NP完全问题，需要限制搜索复杂度
type BranchAndBoundSelector struct {
	MaxSearchDepth int // 最大搜索深度（默认10000次尝试）
}

// NewBranchAndBoundSelector 创建BranchAndBound选择器
func NewBranchAndBoundSelector() UTXOSelector {
	return &BranchAndBoundSelector{
		MaxSearchDepth: 10000,
	}
}

// Select 实现UTXOSelector接口
func (s *BranchAndBoundSelector) Select(utxos []UTXO, targetAmount *Amount) ([]UTXO, *Amount, error) {
	if len(utxos) == 0 {
		return nil, nil, ErrNoUTXOs
	}

	if targetAmount == nil || targetAmount.IsZero() {
		return nil, nil, fmt.Errorf("invalid target amount")
	}

	// 如果UTXO数量很少，直接使用简单策略
	if len(utxos) <= 2 {
		return NewGreedySelector().Select(utxos, targetAmount)
	}

	// TODO: 实现完整的分支定界算法
	// 当前简化版本：降级到贪心策略
	// 完整实现需要：
	//  1. 动态规划或回溯搜索
	//  2. 剪枝优化
	//  3. 搜索深度限制
	return NewGreedySelector().Select(utxos, targetAmount)
}

// accumulateUTXOs 累加UTXO直到满足目标金额
func accumulateUTXOs(utxos []UTXO, targetAmount *Amount) ([]UTXO, *Amount, error) {
	selected := []UTXO{}
	total := Zero()

	for _, utxo := range utxos {
		selected = append(selected, utxo)
		total = total.Add(utxo.Amount)

		if total.GreaterThanOrEqual(targetAmount) {
			return selected, total, nil
		}
	}

	// 累加所有UTXO仍不够
	return nil, nil, fmt.Errorf("%w: need %s, have %s",
		ErrInsufficientBalance,
		targetAmount.String(),
		total.String(),
	)
}

// CalculateTotalAmount 计算UTXO列表的总金额
func CalculateTotalAmount(utxos []UTXO) *Amount {
	total := Zero()
	for _, utxo := range utxos {
		total = total.Add(utxo.Amount)
	}
	return total
}

// FilterUTXOsByMinAmount 过滤掉金额过小的UTXO
//
// 用途：
//   - 避免选择粉尘UTXO（dust）
//   - 减少交易输入数量
func FilterUTXOsByMinAmount(utxos []UTXO, minAmount *Amount) []UTXO {
	filtered := []UTXO{}
	for _, utxo := range utxos {
		if utxo.Amount.GreaterThanOrEqual(minAmount) {
			filtered = append(filtered, utxo)
		}
	}
	return filtered
}

// SortUTXOsByAmount 按金额排序UTXO（从大到小）
func SortUTXOsByAmount(utxos []UTXO) []UTXO {
	sorted := make([]UTXO, len(utxos))
	copy(sorted, utxos)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount.GreaterThan(sorted[j].Amount)
	})

	return sorted
}

// SortUTXOsByAmountAsc 按金额排序UTXO（从小到大）
func SortUTXOsByAmountAsc(utxos []UTXO) []UTXO {
	sorted := make([]UTXO, len(utxos))
	copy(sorted, utxos)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount.LessThan(sorted[j].Amount)
	})

	return sorted
}

// EstimateInputSize 估算单个输入的字节大小
//
// 假设：
//   - P2PKH输入 ~148字节
//   - P2WPKH输入 ~68字节
//   - 这里使用保守估计148字节
func EstimateInputSize() int {
	return 148
}

// EstimateOutputSize 估算单个输出的字节大小
//
// 假设：
//   - P2PKH输出 ~34字节
//   - P2WPKH输出 ~31字节
//   - 这里使用保守估计34字节
func EstimateOutputSize() int {
	return 34
}

// EstimateTransactionSize 估算交易大小（字节）
func EstimateTransactionSize(numInputs, numOutputs int) int {
	// 交易固定开销 ~10字节
	baseSize := 10

	// 输入总大小
	inputsSize := numInputs * EstimateInputSize()

	// 输出总大小
	outputsSize := numOutputs * EstimateOutputSize()

	return baseSize + inputsSize + outputsSize
}
