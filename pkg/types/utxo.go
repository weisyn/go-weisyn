package types

// ⚠️ **非业务性过度设计 - 已注释**
// 以下所有内容为底层UTXO技术实现细节，不被 pkg/interfaces/blockchain 业务接口层使用
// pkg/interfaces/blockchain 使用 BalanceInfo 等业务抽象类型（定义在 account.go 中）
// UTXO层面的技术细节应该隐藏在 repository 层内部实现中
// 如需要时可取消注释

/*
import (
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ================================================================================================
// 🎯 UTXO 业务查询和请求类型
// ================================================================================================

// GetUTXOsByOwnerRequest 根据所有者查询UTXO请求
type GetUTXOsByOwnerRequest struct {
	OwnerAddress *transaction.Address    `json:"owner_address"` // 所有者地址
	Filter       *UTXOQueryFilter `json:"filter"`        // 查询过滤器
}

// GetUTXOsByOwnerResponse 根据所有者查询UTXO响应
type GetUTXOsByOwnerResponse struct {
	UTXOs []*utxo.UTXO `json:"utxos"` // UTXO列表
	Found bool         `json:"found"` // 是否找到
}

// 注意：UTXOQueryFilter 已在 account.go 中定义

// GetBalanceRequest 获取余额请求
type GetBalanceRequest struct {
	OwnerAddress *transaction.Address `json:"owner_address"` // 所有者地址
	TokenID      []byte        `json:"token_id"`      // 代币ID
}

// GetBalanceResponse 获取余额响应
type GetBalanceResponse struct {
	OwnerAddress     *transaction.Address `json:"owner_address"`     // 所有者地址
	TokenID          []byte        `json:"token_id"`          // 代币ID
	AvailableBalance uint64        `json:"available_balance"` // 可用余额
	LockedBalance    uint64        `json:"locked_balance"`    // 锁定余额
	PendingBalance   uint64        `json:"pending_balance"`   // 待确认余额
	TotalBalance     uint64        `json:"total_balance"`     // 总余额
	Found            bool          `json:"found"`             // 是否找到
}

// GetAllBalancesRequest 获取所有余额请求
type GetAllBalancesRequest struct {
	OwnerAddress *transaction.Address `json:"owner_address"` // 所有者地址
}

// GetAllBalancesResponse 获取所有余额响应
type GetAllBalancesResponse struct {
	OwnerAddress *transaction.Address  `json:"owner_address"` // 所有者地址
	Balances     []*BalanceInfo `json:"balances"`      // 余额列表
	TotalTokens  uint32         `json:"total_tokens"`  // 代币种类数
	Found        bool           `json:"found"`         // 是否找到
}

// ================================================================================================
// 🎯 UTXO 选择和优化类型
// ================================================================================================

// UTXOSelectionRequest UTXO选择请求
type UTXOSelectionRequest struct {
	OwnerAddress *transaction.Address `json:"owner_address"`  // 所有者地址
	TargetAmount uint64        `json:"target_amount"`  // 目标金额
	TokenID      []byte        `json:"token_id"`       // 代币ID
	Strategy     string        `json:"strategy"`       // 选择策略
	MinUTXOValue uint64        `json:"min_utxo_value"` // 最小UTXO价值
}

// UTXOSelectionResult UTXO选择结果
type UTXOSelectionResult struct {
	Success        bool                `json:"success"`         // 是否成功
	SelectedUTXOs  []*utxo.UTXO        `json:"selected_utxos"`  // 选中的UTXO
	TotalValue     uint64              `json:"total_value"`     // 总价值
	ChangeAmount   uint64              `json:"change_amount"`   // 找零金额
	EstimatedFee   uint64              `json:"estimated_fee"`   // 预估费用
	SelectionStats *UTXOSelectionStats `json:"selection_stats"` // 选择统计
	ErrorMessage   string              `json:"error_message"`   // 错误信息
}

// UTXOSelectionStats UTXO选择统计
type UTXOSelectionStats struct {
	TotalAvailableUTXOs uint32  `json:"total_available_utxos"` // 总可用UTXO数
	SelectedCount       uint32  `json:"selected_count"`        // 选中数量
	SelectionRatio      float64 `json:"selection_ratio"`       // 选择比例
	EfficiencyScore     uint32  `json:"efficiency_score"`      // 效率得分
}

// ================================================================================================
// 🎯 UTXO 统计和分析类型
// ================================================================================================

// UTXOCountStats UTXO统计
type UTXOCountStats struct {
	Address               *transaction.Address          `json:"address"`                // 地址
	TotalUTXOs            uint32                 `json:"total_utxos"`            // 总UTXO数
	TokenDistribution     map[string]uint32      `json:"token_distribution"`     // 代币分布
	ValueDistribution     *ValueDistribution     `json:"value_distribution"`     // 价值分布
	AgeDistribution       *AgeDistribution       `json:"age_distribution"`       // 年龄分布
	FragmentationIndex    float64                `json:"fragmentation_index"`    // 碎片化指数
	OptimizationPotential *OptimizationPotential `json:"optimization_potential"` // 优化潜力
	LastAnalysisTime      time.Time              `json:"last_analysis_time"`     // 最后分析时间
}

// ValueDistribution 价值分布
type ValueDistribution struct {
	DustUTXOs         uint32  `json:"dust_utxos"`         // 灰尘UTXO数
	SmallUTXOs        uint32  `json:"small_utxos"`        // 小额UTXO数
	MediumUTXOs       uint32  `json:"medium_utxos"`       // 中额UTXO数
	LargeUTXOs        uint32  `json:"large_utxos"`        // 大额UTXO数
	AverageValue      uint64  `json:"average_value"`      // 平均价值
	MedianValue       uint64  `json:"median_value"`       // 中位数价值
	StandardDeviation float64 `json:"standard_deviation"` // 标准差
}

// AgeDistribution 年龄分布
type AgeDistribution struct {
	NewUTXOs    uint32  `json:"new_utxos"`    // 新UTXO数（<24小时）
	RecentUTXOs uint32  `json:"recent_utxos"` // 近期UTXO数（<7天）
	MatureUTXOs uint32  `json:"mature_utxos"` // 成熟UTXO数（>30天）
	OldUTXOs    uint32  `json:"old_utxos"`    // 老UTXO数（>180天）
	AverageAge  float64 `json:"average_age"`  // 平均年龄（天）
}

// OptimizationPotential 统一定义见 optimization.go

// ================================================================================================
// 🎯 UTXO 整理和优化类型
// ================================================================================================

// ConsolidationStrategy 整理策略
type ConsolidationStrategy string

const (
	ConsolidationStrategy_AGGRESSIVE     ConsolidationStrategy = "aggressive"     // 激进策略
	ConsolidationStrategy_CONSERVATIVE   ConsolidationStrategy = "conservative"   // 保守策略
	ConsolidationStrategy_BALANCED       ConsolidationStrategy = "balanced"       // 平衡策略
	ConsolidationStrategy_COST_EFFECTIVE ConsolidationStrategy = "cost_effective" // 成本效益策略
)

// ConsolidationPlan 整理计划
type ConsolidationPlan struct {
	Strategy             ConsolidationStrategy `json:"strategy"`               // 策略
	TargetUTXOs          []*transaction.OutPoint      `json:"target_utxos"`           // 目标UTXO
	ConsolidationSteps   uint32                `json:"consolidation_steps"`    // 整理步骤数
	TotalCost            uint64                `json:"total_cost"`             // 总成本
	ExpectedSavings      uint64                `json:"expected_savings"`       // 预期节省
	EstimatedDuration    uint32                `json:"estimated_duration"`     // 预计持续时间
	OptimalExecutionTime uint64                `json:"optimal_execution_time"` // 最佳执行时间
	CostBreakdown        *CostBreakdown        `json:"cost_breakdown"`         // 成本明细
}

// CostBreakdown 成本明细
type CostBreakdown struct {
	BaseFee     uint64 `json:"base_fee"`     // 基础费用
	PriorityFee uint64 `json:"priority_fee"` // 优先级费用
	NetworkFee  uint64 `json:"network_fee"`  // 网络费用
	TotalFee    uint64 `json:"total_fee"`    // 总费用
}

// UTXOOptimizationResult 优化结果
type UTXOOptimizationResult struct {
	Success          bool                 `json:"success"`           // 是否成功
	OptimizationPlan *OptimizationPlan    `json:"optimization_plan"` // 优化计划
	CostBenefit      *CostBenefitAnalysis `json:"cost_benefit"`      // 成本效益
	Recommendations  []string             `json:"recommendations"`   // 建议
	ErrorMessage     string               `json:"error_message"`     // 错误信息
}

// OptimizationPlan 优化计划
type OptimizationPlan struct {
	TargetUTXOs      []*transaction.OutPoint `json:"target_utxos"`      // 目标UTXO
	ConsolidationTxs uint32           `json:"consolidation_txs"` // 整理交易数
	EstimatedCost    uint64           `json:"estimated_cost"`    // 预计成本
	EstimatedSavings uint64           `json:"estimated_savings"` // 预计节省
	Priority         string           `json:"priority"`          // 优先级
	OptimalTiming    string           `json:"optimal_timing"`    // 最佳时机
}

// CostBenefitAnalysis 成本效益分析
type CostBenefitAnalysis struct {
	ImmediateCost    uint64  `json:"immediate_cost"`    // 即时成本
	LongTermSavings  uint64  `json:"long_term_savings"` // 长期节省
	ROI              float64 `json:"roi"`               // 投资回报率
	PaybackPeriod    uint32  `json:"payback_period"`    // 回收期（天）
	NetBenefit       uint64  `json:"net_benefit"`       // 净收益
	RecommendExecute bool    `json:"recommend_execute"` // 推荐执行
}

// ================================================================================================
// 🎯 Repository 接口支持类型
// ================================================================================================

// GetUTXORequest 获取单个UTXO请求
type GetUTXORequest struct {
	OutPoint       *transaction.OutPoint `json:"out_point"`       // 输出点
	IncludeMempool bool           `json:"include_mempool"` // 是否包含内存池
}

// GetUTXOResponse 获取单个UTXO响应
type GetUTXOResponse struct {
	UTXO  *utxo.UTXO `json:"utxo"`  // UTXO数据
	Found bool       `json:"found"` // 是否找到
}

// CreateUTXOsRequest 创建UTXO请求
type CreateUTXOsRequest struct {
	UTXOs []*utxo.UTXO `json:"utxos"` // UTXO列表
}

// CreateUTXOsResponse 创建UTXO响应
type CreateUTXOsResponse struct {
	Created uint32 `json:"created"` // 创建数量
	Success bool   `json:"success"` // 是否成功
}

// ConsumeUTXOsRequest 消费UTXO请求
type ConsumeUTXOsRequest struct {
	OutPoints []*transaction.OutPoint `json:"out_points"` // 输出点列表
}

// ConsumeUTXOsResponse 消费UTXO响应
type ConsumeUTXOsResponse struct {
	Consumed uint32 `json:"consumed"` // 消费数量
	Success  bool   `json:"success"`  // 是否成功
}

// AddReferenceRequest 添加引用请求
type AddReferenceRequest struct {
	OutPoint  *transaction.OutPoint `json:"out_point"` // 输出点
	Reference string         `json:"reference"` // 引用信息
}

// AddReferenceResponse 添加引用响应
type AddReferenceResponse struct {
	Success bool `json:"success"` // 是否成功
}

// RemoveReferenceRequest 移除引用请求
type RemoveReferenceRequest struct {
	OutPoint  *transaction.OutPoint `json:"out_point"` // 输出点
	Reference string         `json:"reference"` // 引用信息
}

// RemoveReferenceResponse 移除引用响应
type RemoveReferenceResponse struct {
	Success bool `json:"success"` // 是否成功
}

// ProcessTransactionUTXOsRequest 处理交易UTXO请求
type ProcessTransactionUTXOsRequest struct {
	Transaction *transaction.Transaction `json:"transaction"`  // 交易
	BlockHeight uint64            `json:"block_height"` // 区块高度
}

// ProcessTransactionUTXOsResponse 处理交易UTXO响应
type ProcessTransactionUTXOsResponse struct {
	Success bool `json:"success"` // 是否成功
}

// RevertTransactionUTXOsRequest 回滚交易UTXO请求
type RevertTransactionUTXOsRequest struct {
	Transaction *transaction.Transaction `json:"transaction"` // 交易
}

// RevertTransactionUTXOsResponse 回滚交易UTXO响应
type RevertTransactionUTXOsResponse struct {
	Success bool `json:"success"` // 是否成功
}

// ProcessBlockUTXOsRequest 处理区块UTXO请求
type ProcessBlockUTXOsRequest struct {
	Block *core.Block `json:"block"` // 区块
}

// ProcessBlockUTXOsResponse 处理区块UTXO响应
type ProcessBlockUTXOsResponse struct {
	Success bool `json:"success"` // 是否成功
}

// RevertBlockUTXOsRequest 回滚区块UTXO请求
type RevertBlockUTXOsRequest struct {
	Block *core.Block `json:"block"` // 区块
}

// RevertBlockUTXOsResponse 回滚区块UTXO响应
type RevertBlockUTXOsResponse struct {
	Success bool `json:"success"` // 是否成功
}

// ValidateUTXOForSpendingRequest UTXO花费验证请求
type ValidateUTXOForSpendingRequest struct {
	OutPoint    *transaction.OutPoint    `json:"out_point"`   // 输出点
	Transaction *transaction.Transaction `json:"transaction"` // 交易
}

// ValidateUTXOForSpendingResponse UTXO花费验证响应
type ValidateUTXOForSpendingResponse struct {
	Valid   bool   `json:"valid"`   // 是否有效
	Message string `json:"message"` // 验证消息
}

// ValidateUTXOSetRequest UTXO集验证请求
type ValidateUTXOSetRequest struct {
	OutPoints []*transaction.OutPoint `json:"out_points"` // 输出点列表
}

// ValidateUTXOSetResponse UTXO集验证响应
type ValidateUTXOSetResponse struct {
	Valid   bool   `json:"valid"`   // 是否有效
	Message string `json:"message"` // 验证消息
}
*/
