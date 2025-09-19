package fee

import (
	"context"
	"fmt"
	"math/big"

	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ===============================
// 费用系统管理器
// ===============================

// Manager 费用系统统一管理器
type Manager struct {
	calculator      *Calculator
	validator       *MechanismValidator
	estimator       *FeeEstimator
	coinbaseBuilder *CoinbaseBuilder
}

// NewManager 创建新的费用管理器
func NewManager(hashService pbtx.TransactionHashServiceClient) *Manager {
	if hashService == nil {
		panic("交易哈希服务不能为nil")
	}

	calculator := NewCalculator(hashService)
	return &Manager{
		calculator:      calculator,
		validator:       NewMechanismValidator(calculator),
		estimator:       NewFeeEstimator(calculator),
		coinbaseBuilder: NewCoinbaseBuilder(),
	}
}

// ===============================
// 对外核心接口（仅两个方法）
// ===============================

// ValidateFee 验证交易费用（前置处理 - 用户端）
//
// 功能：验证交易的UTXO差额是否满足其费用机制要求
// 用途：用户构造和提交交易时的验证入口
// 约束：基于已签名交易进行验证，不能修改交易内容
func (m *Manager) ValidateFee(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) error {
	if tx == nil {
		return NewValidationError("transaction", nil, "交易不能为nil")
	}

	if fetchUTXO == nil {
		return NewValidationError("fetchUTXO", nil, "UTXO查询回调不能为nil")
	}

	// 委托给费用机制验证器
	return m.validator.ValidateFee(ctx, tx, fetchUTXO)
}

// CollectFeesAndBuildCoinbase 收集费用并构建Coinbase（后置处理 - 矿工端）
//
// 功能：从交易列表中收集所有费用，按Token汇总，构建Coinbase交易
// 用途：矿工生成挖矿模板时的主要入口
// 约束：处理UTXO查询失败的容错情况，确保Coinbase构建的确定性
func (m *Manager) CollectFeesAndBuildCoinbase(ctx context.Context, txs []*pbtx.Transaction, minerAddr []byte, chainID []byte, fetchUTXO UTXOFetcher) (*pbtx.Transaction, error) {
	if len(txs) == 0 {
		// 即使没有交易，也构建空的Coinbase以保持模板结构一致
		return m.buildEmptyCoinbase(minerAddr, chainID)
	}

	if err := ValidateAddress(minerAddr); err != nil {
		return nil, WrapError(err, "矿工地址无效")
	}

	if len(chainID) == 0 {
		return nil, NewValidationError("chain_id", chainID, "链ID不能为空")
	}

	if fetchUTXO == nil {
		return nil, NewValidationError("fetchUTXO", nil, "UTXO查询回调不能为nil")
	}

	// 1. 批量计算所有交易的费用
	txFees, err := m.calculator.BatchCalculateUTXODifferences(ctx, txs, fetchUTXO)
	if err != nil {
		return nil, WrapError(err, "批量计算交易费用失败")
	}

	// 2. 聚合所有费用
	aggregatedFees := m.calculator.AggregateFees(txFees)

	// 3. 构建Coinbase交易
	coinbase, err := m.coinbaseBuilder.BuildCoinbase(aggregatedFees, minerAddr, chainID)
	if err != nil {
		return nil, WrapError(err, "构建Coinbase交易失败")
	}

	// 4. 验证构建的Coinbase交易
	if err := m.coinbaseBuilder.ValidateCoinbase(coinbase, aggregatedFees, minerAddr); err != nil {
		return nil, WrapError(err, "Coinbase交易验证失败")
	}

	return coinbase, nil
}

// ===============================
// 扩展功能接口
// ===============================

// EstimateFee 估算交易费用
func (m *Manager) EstimateFee(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*FeeEstimate, error) {
	if tx == nil {
		return nil, NewValidationError("transaction", nil, "交易不能为nil")
	}

	if fetchUTXO == nil {
		return nil, NewValidationError("fetchUTXO", nil, "UTXO查询回调不能为nil")
	}

	return m.estimator.EstimateFee(ctx, tx, fetchUTXO)
}

// CalculateTransactionFee 计算单个交易的费用
func (m *Manager) CalculateTransactionFee(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*TransactionFee, error) {
	if tx == nil {
		return nil, NewValidationError("transaction", nil, "交易不能为nil")
	}

	if fetchUTXO == nil {
		return nil, NewValidationError("fetchUTXO", nil, "UTXO查询回调不能为nil")
	}

	return m.calculator.CalculateUTXODifference(ctx, tx, fetchUTXO)
}

// BatchCalculateTransactionFees 批量计算多个交易的费用
func (m *Manager) BatchCalculateTransactionFees(ctx context.Context, txs []*pbtx.Transaction, fetchUTXO UTXOFetcher) ([]*TransactionFee, error) {
	if len(txs) == 0 {
		return []*TransactionFee{}, nil
	}

	if fetchUTXO == nil {
		return nil, NewValidationError("fetchUTXO", nil, "UTXO查询回调不能为nil")
	}

	return m.calculator.BatchCalculateUTXODifferences(ctx, txs, fetchUTXO)
}

// AggregateFees 聚合多个交易的费用
func (m *Manager) AggregateFees(txFees []*TransactionFee) *AggregatedFees {
	return m.calculator.AggregateFees(txFees)
}

// ValidateCoinbase 验证Coinbase交易
func (m *Manager) ValidateCoinbase(coinbase *pbtx.Transaction, expectedFees *AggregatedFees, minerAddr []byte) error {
	return m.coinbaseBuilder.ValidateCoinbase(coinbase, expectedFees, minerAddr)
}

// ===============================
// 工具和诊断方法
// ===============================

// AnalyzeTransaction 分析交易类型和特征
func (m *Manager) AnalyzeTransaction(tx *pbtx.Transaction) *TransactionAnalysis {
	if tx == nil {
		return &TransactionAnalysis{
			Type:        "无效",
			Description: "交易为nil",
			IsValid:     false,
		}
	}

	analysis := &TransactionAnalysis{
		InputCount:  len(tx.Inputs),
		OutputCount: len(tx.Outputs),
		IsValid:     true,
	}

	// 分析交易类型
	if len(tx.Inputs) == 0 {
		analysis.Type = "空投交易"
		analysis.Description = "无输入的空投交易，费用为0"
		analysis.IsAirdrop = true
	} else if len(tx.Outputs) == 0 {
		analysis.Type = "销毁交易"
		analysis.Description = "无输出的销毁交易，不计入矿工费用"
		analysis.IsBurn = true
	} else {
		analysis.Type = "正常交易"
		analysis.Description = "有输入有输出的正常交易"
		analysis.IsNormal = true
	}

	// 分析费用机制
	if tx.FeeMechanism == nil {
		analysis.FeeMechanism = "默认UTXO差额"
		analysis.MechanismDescription = "使用输入减输出的差额作为费用"
	} else {
		switch tx.FeeMechanism.(type) {
		case *pbtx.Transaction_MinimumFee:
			analysis.FeeMechanism = "最低费用"
			analysis.MechanismDescription = "要求费用不低于指定最低金额"
		case *pbtx.Transaction_ProportionalFee:
			analysis.FeeMechanism = "比例费用"
			analysis.MechanismDescription = "按转账金额的一定比例收取费用"
		case *pbtx.Transaction_ContractFee:
			analysis.FeeMechanism = "合约执行费用"
			analysis.MechanismDescription = "包含基础费用和执行费用费用"
		case *pbtx.Transaction_PriorityFee:
			analysis.FeeMechanism = "优先级费用"
			analysis.MechanismDescription = "按优先级倍率收取费用"
		default:
			analysis.FeeMechanism = "未知机制"
			analysis.MechanismDescription = "未识别的费用机制"
		}
	}

	return analysis
}

// TransactionAnalysis 交易分析结果
type TransactionAnalysis struct {
	Type                 string // 交易类型
	Description          string // 类型描述
	FeeMechanism         string // 费用机制
	MechanismDescription string // 机制描述
	InputCount           int    // 输入数量
	OutputCount          int    // 输出数量
	IsValid              bool   // 是否有效
	IsAirdrop            bool   // 是否为空投
	IsBurn               bool   // 是否为销毁
	IsNormal             bool   // 是否为正常交易
}

// GetSystemStats 获取费用系统统计信息
func (m *Manager) GetSystemStats() *SystemStats {
	return &SystemStats{
		ManagerVersion: "1.0.0",
		SupportedMechanisms: []string{
			"默认UTXO差额",
			"最低费用",
			"比例费用",
			"合约执行费用",
			"优先级费用",
		},
		SupportedTokenTypes: []string{
			"原生代币",
			"同质化代币(FT)",
			"非同质化代币(NFT)",
			"半同质化代币(SFT)",
		},
		Features: []string{
			"批量处理",
			"容错处理",
			"找零识别",
			"多Token支持",
			"确定性构建",
		},
	}
}

// SystemStats 系统统计信息
type SystemStats struct {
	ManagerVersion      string   // 管理器版本
	SupportedMechanisms []string // 支持的费用机制
	SupportedTokenTypes []string // 支持的Token类型
	Features            []string // 支持的功能特性
}

// ===============================
// 私有辅助方法
// ===============================

// buildEmptyCoinbase 构建空的Coinbase交易
func (m *Manager) buildEmptyCoinbase(minerAddr []byte, chainID []byte) (*pbtx.Transaction, error) {
	emptyFees := &AggregatedFees{
		ByToken: make(map[TokenKey]*big.Int),
		Stats: &AggregationStats{
			TotalTxs:       0,
			ZeroFeeTxs:     0,
			TokenTypes:     make(map[TokenKey]int),
			TotalFeeAmount: make(map[TokenKey]*big.Int),
		},
	}

	return m.coinbaseBuilder.BuildCoinbase(emptyFees, minerAddr, chainID)
}

// ===============================
// 格式化和调试方法
// ===============================

// FormatTransactionFee 格式化交易费用信息
func (m *Manager) FormatTransactionFee(txFee *TransactionFee) string {
	return m.calculator.FormatTransactionFee(txFee)
}

// FormatAggregatedFees 格式化聚合费用信息
func (m *Manager) FormatAggregatedFees(aggregated *AggregatedFees) string {
	if aggregated == nil {
		return "nil"
	}

	return fmt.Sprintf("聚合费用{总交易: %d, 零费用: %d, Token类型: %d, 费用: %s}",
		aggregated.Stats.TotalTxs,
		aggregated.Stats.ZeroFeeTxs,
		len(aggregated.Stats.TokenTypes),
		FormatTokenMap(aggregated.ByToken),
	)
}

// FormatCoinbase 格式化Coinbase交易信息
func (m *Manager) FormatCoinbase(coinbase *pbtx.Transaction) string {
	return m.coinbaseBuilder.FormatCoinbase(coinbase)
}

// FormatFeeEstimate 格式化费用估算信息
func (m *Manager) FormatFeeEstimate(estimate *FeeEstimate) string {
	return m.estimator.FormatFeeEstimate(estimate)
}

// FormatTransactionAnalysis 格式化交易分析信息
func (m *Manager) FormatTransactionAnalysis(analysis *TransactionAnalysis) string {
	if analysis == nil {
		return "nil"
	}

	return fmt.Sprintf("交易分析{类型: %s, 费用机制: %s, 输入: %d, 输出: %d, 有效: %v, 描述: %s}",
		analysis.Type,
		analysis.FeeMechanism,
		analysis.InputCount,
		analysis.OutputCount,
		analysis.IsValid,
		analysis.Description,
	)
}

// ===============================
// 高级功能方法
// ===============================

// ValidateTransactionBatch 批量验证多个交易的费用
func (m *Manager) ValidateTransactionBatch(ctx context.Context, txs []*pbtx.Transaction, fetchUTXO UTXOFetcher) []error {
	if len(txs) == 0 {
		return []error{}
	}

	errors := make([]error, len(txs))

	for i, tx := range txs {
		errors[i] = m.ValidateFee(ctx, tx, fetchUTXO)
	}

	return errors
}

// EstimateTransactionBatch 批量估算多个交易的费用
func (m *Manager) EstimateTransactionBatch(ctx context.Context, txs []*pbtx.Transaction, fetchUTXO UTXOFetcher) ([]*FeeEstimate, []error) {
	if len(txs) == 0 {
		return []*FeeEstimate{}, []error{}
	}

	estimates := make([]*FeeEstimate, len(txs))
	errors := make([]error, len(txs))

	for i, tx := range txs {
		estimates[i], errors[i] = m.EstimateFee(ctx, tx, fetchUTXO)
	}

	return estimates, errors
}

// GetTransactionType 获取交易类型描述
func (m *Manager) GetTransactionType(tx *pbtx.Transaction) string {
	return m.calculator.GetTransactionType(tx)
}

// IsCoinbaseTransaction 判断是否为Coinbase交易
func (m *Manager) IsCoinbaseTransaction(tx *pbtx.Transaction) bool {
	return m.coinbaseBuilder.IsCoinbaseTransaction(tx)
}

// IsEmptyCoinbase 判断是否为空Coinbase交易
func (m *Manager) IsEmptyCoinbase(tx *pbtx.Transaction) bool {
	return m.coinbaseBuilder.IsEmptyCoinbase(tx)
}

// GetCoinbaseFees 从Coinbase交易中提取费用信息
func (m *Manager) GetCoinbaseFees(coinbase *pbtx.Transaction) (map[TokenKey]*big.Int, error) {
	return m.coinbaseBuilder.GetCoinbaseFees(coinbase)
}
