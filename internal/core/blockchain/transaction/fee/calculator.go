package fee

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ===============================
// 类型定义
// ===============================

// UTXOFetcher UTXO查询回调函数类型
type UTXOFetcher func(ctx context.Context, outpoint *pbtx.OutPoint) (*pbtx.TxOutput, error)

// TransactionFee 单个交易的费用信息
type TransactionFee struct {
	TxID  []byte                // 交易ID
	Fees  map[TokenKey]*big.Int // 按Token分类的费用
	Stats *FeeCalculationStats  // 计算统计信息
}

// FeeCalculationStats 费用计算统计信息
type FeeCalculationStats struct {
	InputCount       int  // 输入数量
	OutputCount      int  // 输出数量
	SuccessfulInputs int  // 成功处理的输入数量
	FailedInputs     int  // 失败的输入数量
	TokenTypes       int  // 涉及的Token类型数量
	IsAirdrop        bool // 是否为空投交易
	IsBurn           bool // 是否为销毁交易
	HasZeroFee       bool // 是否为零费用交易
}

// TransactionType 交易类型枚举
type TransactionType int

const (
	TxTypeUnknown TransactionType = iota
	TxTypeAirdrop                 // 空投交易（无输入）
	TxTypeBurn                    // 销毁交易（无输出）
	TxTypeNormal                  // 正常交易
	TxTypeZeroFee                 // 零费用交易
)

// ===============================
// 核心计算器
// ===============================

// Calculator UTXO差额计算器
type Calculator struct {
	hashService pbtx.TransactionHashServiceClient // 交易哈希服务客户端
}

// NewCalculator 创建新的计算器实例
func NewCalculator(hashService pbtx.TransactionHashServiceClient) *Calculator {
	if hashService == nil {
		panic("交易哈希服务不能为nil")
	}
	return &Calculator{
		hashService: hashService,
	}
}

// CalculateUTXODifference 计算单个交易的UTXO差额（费用）
func (c *Calculator) CalculateUTXODifference(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*TransactionFee, error) {
	if tx == nil {
		return nil, NewValidationError("transaction", nil, "交易不能为nil")
	}

	// 生成交易ID（使用统一的交易哈希服务）
	txID, err := c.computeTransactionHash(ctx, tx)
	if err != nil {
		return nil, WrapError(err, "计算交易哈希失败")
	}

	// 初始化统计信息
	stats := &FeeCalculationStats{
		InputCount:       len(tx.Inputs),
		OutputCount:      len(tx.Outputs),
		SuccessfulInputs: 0,
		FailedInputs:     0,
		TokenTypes:       0,
		IsAirdrop:        len(tx.Inputs) == 0,
		IsBurn:           len(tx.Outputs) == 0,
		HasZeroFee:       false,
	}

	// 计算输入总额
	inputSums, err := c.calculateInputSums(ctx, tx, fetchUTXO, stats)
	if err != nil {
		return nil, WrapError(err, "计算输入总额失败")
	}

	// 计算输出总额
	outputSums := c.calculateOutputSums(tx)

	// 计算UTXO差额（费用）
	fees := make(map[TokenKey]*big.Int)
	for tokenKey, inputAmount := range inputSums {
		outputAmount, exists := outputSums[tokenKey]
		if !exists {
			outputAmount = big.NewInt(0)
		}

		// 费用 = 输入 - 输出
		fee := SafeSub(inputAmount, outputAmount)
		if IsPositive(fee) {
			fees[tokenKey] = fee
		}
	}

	// 检查是否为零费用交易
	stats.HasZeroFee = len(fees) == 0
	stats.TokenTypes = len(fees)

	return &TransactionFee{
		TxID:  txID,
		Fees:  fees,
		Stats: stats,
	}, nil
}

// calculateInputSums 计算输入总额（按Token分类）
func (c *Calculator) calculateInputSums(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher, stats *FeeCalculationStats) (map[TokenKey]*big.Int, error) {
	inputSums := make(map[TokenKey]*big.Int)

	for _, input := range tx.Inputs {
		// 通过回调获取前序UTXO的输出
		prevOutput, err := fetchUTXO(ctx, input.PreviousOutput)
		if err != nil {
			stats.FailedInputs++
			continue // 容错处理：跳过无法获取的UTXO
		}

		stats.SuccessfulInputs++

		// 提取Token信息
		tokenKey, amount, err := ExtractTokenInfo(prevOutput)
		if err != nil {
			// Token解析失败，跳过该输入
			continue
		}

		// 累加到对应Token
		if existing, exists := inputSums[tokenKey]; exists {
			inputSums[tokenKey] = SafeAdd(existing, amount)
		} else {
			inputSums[tokenKey] = new(big.Int).Set(amount)
		}
	}

	return inputSums, nil
}

// calculateOutputSums 计算输出总额（按Token分类）
func (c *Calculator) calculateOutputSums(tx *pbtx.Transaction) map[TokenKey]*big.Int {
	outputSums := make(map[TokenKey]*big.Int)

	for _, output := range tx.Outputs {
		// 提取Token信息
		tokenKey, amount, err := ExtractTokenInfo(output)
		if err != nil {
			// 非资产输出（如资源输出、状态输出）不参与费用计算
			continue
		}

		// 累加到对应Token
		if existing, exists := outputSums[tokenKey]; exists {
			outputSums[tokenKey] = SafeAdd(existing, amount)
		} else {
			outputSums[tokenKey] = new(big.Int).Set(amount)
		}
	}

	return outputSums
}

// BatchCalculateUTXODifferences 批量计算多个交易的UTXO差额
func (c *Calculator) BatchCalculateUTXODifferences(ctx context.Context, txs []*pbtx.Transaction, fetchUTXO UTXOFetcher) ([]*TransactionFee, error) {
	if len(txs) == 0 {
		return []*TransactionFee{}, nil
	}

	results := make([]*TransactionFee, 0, len(txs))

	for _, tx := range txs {
		txFee, err := c.CalculateUTXODifference(ctx, tx, fetchUTXO)
		if err != nil {
			// 容错处理：记录错误但继续处理其他交易
			continue
		}
		results = append(results, txFee)
	}

	return results, nil
}

// AggregateFees 聚合多个交易的费用
func (c *Calculator) AggregateFees(txFees []*TransactionFee) *AggregatedFees {
	if len(txFees) == 0 {
		return &AggregatedFees{
			ByToken: make(map[TokenKey]*big.Int),
			Stats: &AggregationStats{
				TotalTxs:       0,
				ZeroFeeTxs:     0,
				TokenTypes:     make(map[TokenKey]int),
				TotalFeeAmount: make(map[TokenKey]*big.Int),
			},
		}
	}

	aggregated := &AggregatedFees{
		ByToken: make(map[TokenKey]*big.Int),
		Stats: &AggregationStats{
			TotalTxs:       len(txFees),
			ZeroFeeTxs:     0,
			TokenTypes:     make(map[TokenKey]int),
			TotalFeeAmount: make(map[TokenKey]*big.Int),
		},
	}

	for _, txFee := range txFees {
		if txFee.Stats.HasZeroFee {
			aggregated.Stats.ZeroFeeTxs++
			continue
		}

		for tokenKey, amount := range txFee.Fees {
			// 累加总费用
			if existing, exists := aggregated.ByToken[tokenKey]; exists {
				aggregated.ByToken[tokenKey] = SafeAdd(existing, amount)
			} else {
				aggregated.ByToken[tokenKey] = new(big.Int).Set(amount)
			}

			// 更新统计信息
			aggregated.Stats.TokenTypes[tokenKey]++
			if existing, exists := aggregated.Stats.TotalFeeAmount[tokenKey]; exists {
				aggregated.Stats.TotalFeeAmount[tokenKey] = SafeAdd(existing, amount)
			} else {
				aggregated.Stats.TotalFeeAmount[tokenKey] = new(big.Int).Set(amount)
			}
		}
	}

	return aggregated
}

// ===============================
// 工具方法
// ===============================

// computeTransactionHash 使用统一的交易哈希服务计算交易哈希
func (c *Calculator) computeTransactionHash(ctx context.Context, tx *pbtx.Transaction) ([]byte, error) {
	req := &pbtx.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false, // 费用计算不需要调试信息
	}

	resp, err := c.hashService.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用交易哈希服务失败: %v", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("交易结构无效，无法计算哈希")
	}

	if len(resp.Hash) == 0 {
		return nil, fmt.Errorf("交易哈希服务返回空哈希")
	}

	return resp.Hash, nil
}

// GetTransactionType 获取交易类型
func (c *Calculator) GetTransactionType(tx *pbtx.Transaction) string {
	if tx == nil {
		return "unknown"
	}

	if len(tx.Inputs) == 0 {
		return "airdrop"
	}

	if len(tx.Outputs) == 0 {
		return "burn"
	}

	return "normal"
}

// FormatTransactionFee 格式化交易费用信息
func (c *Calculator) FormatTransactionFee(txFee *TransactionFee) string {
	if txFee == nil {
		return "nil transaction fee"
	}

	if len(txFee.Fees) == 0 {
		return fmt.Sprintf("Transaction %s: zero fees", hex.EncodeToString(txFee.TxID[:8]))
	}

	result := fmt.Sprintf("Transaction %s fees:\n", hex.EncodeToString(txFee.TxID[:8]))
	for tokenKey, amount := range txFee.Fees {
		result += fmt.Sprintf("  - %s: %s\n", tokenKey, FormatAmount(amount))
	}

	return result
}

// ===============================
// 聚合费用结构
// ===============================

// AggregatedFees 聚合费用信息
type AggregatedFees struct {
	ByToken map[TokenKey]*big.Int // 按Token分类的总费用
	Stats   *AggregationStats     // 聚合统计信息
}

// AggregationStats 聚合统计信息
type AggregationStats struct {
	TotalTxs       int                   // 总交易数
	ZeroFeeTxs     int                   // 零费用交易数
	TokenTypes     map[TokenKey]int      // 各Token类型的交易数
	TotalFeeAmount map[TokenKey]*big.Int // 各Token的总费用
}
