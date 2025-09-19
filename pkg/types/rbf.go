package types

import (
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ================================================================================================
// 🔄 RBF (Replace-By-Fee) 交易合并系统类型定义
// ================================================================================================

// MergeStrategy 交易合并策略
type MergeStrategy struct {
	Type          MergeStrategyType `json:"type"`           // 策略类型
	OptimizeUTXO  bool              `json:"optimize_utxo"`  // 是否优化UTXO
	OptimizeFee   bool              `json:"optimize_fee"`   // 是否优化费用
	MaxComplexity int               `json:"max_complexity"` // 最大复杂度
	TimeLimit     time.Duration     `json:"time_limit"`     // 时间限制
	Priority      int               `json:"priority"`       // 策略优先级

	// 细分策略配置
	InputStrategy  InputMergeMode  `json:"input_strategy"`  // 输入合并模式
	OutputStrategy OutputMergeMode `json:"output_strategy"` // 输出合并模式
	FeeStrategy    FeeMergeMode    `json:"fee_strategy"`    // 费用合并模式
}

// MergeStrategyType 合并策略类型枚举
type MergeStrategyType string

const (
	MergeStrategyAggressive   MergeStrategyType = "aggressive"   // 激进合并，最大化合并效果
	MergeStrategyConservative MergeStrategyType = "conservative" // 保守合并，优先保证安全性
	MergeStrategyBalanced     MergeStrategyType = "balanced"     // 平衡合并，兼顾效果和安全性
	MergeStrategyOptimal      MergeStrategyType = "optimal"      // 最优合并，基于网络状况动态选择
)

// InputMergeMode 输入合并模式枚举
type InputMergeMode string

const (
	InputMergeUnion     InputMergeMode = "union"     // 输入联合
	InputMergeOptimized InputMergeMode = "optimized" // 输入优化
	InputMergeMinimal   InputMergeMode = "minimal"   // 最小输入
	InputMergeBalanced  InputMergeMode = "balanced"  // 平衡输入
)

// OutputMergeMode 输出合并模式枚举
type OutputMergeMode string

const (
	OutputMergeConsolidate OutputMergeMode = "consolidate" // 输出合并
	OutputMergeSeparate    OutputMergeMode = "separate"    // 输出分离
	OutputMergeOptimized   OutputMergeMode = "optimized"   // 输出优化
	OutputMergeBalanced    OutputMergeMode = "balanced"    // 输出平衡
)

// FeeMergeMode 费用合并模式枚举
type FeeMergeMode string

const (
	FeeMergeSum          FeeMergeMode = "sum"          // 费用相加
	FeeMergeOptimized    FeeMergeMode = "optimized"    // 费用优化
	FeeMergeProportional FeeMergeMode = "proportional" // 费用按比例
	FeeMergeMinimized    FeeMergeMode = "minimized"    // 费用最小化
)

// MergeMetadata 合并元数据
type MergeMetadata struct {
	// 基础统计
	OriginalTxCount   int `json:"original_tx_count"`   // 原始交易数量
	MergedInputCount  int `json:"merged_input_count"`  // 合并后输入数量
	MergedOutputCount int `json:"merged_output_count"` // 合并后输出数量

	// 优化结果
	FeeOptimization  *FeeOptimizationResult  `json:"fee_optimization"`  // 费用优化结果
	UTXOOptimization *UTXOOptimizationResult `json:"utxo_optimization"` // UTXO优化结果

	// 性能指标
	MergeComplexity       float64       `json:"merge_complexity"`        // 合并复杂度
	ProcessingTime        time.Duration `json:"processing_time"`         // 处理时间
	ComputationalCost     int64         `json:"computational_cost"`      // 计算成本
	MemoryUsage           int64         `json:"memory_usage"`            // 内存使用量
	NetworkEfficiencyGain float64       `json:"network_efficiency_gain"` // 网络效率收益

	// 元数据
	CreationTime       time.Time      `json:"creation_time"`       // 创建时间
	Strategy           *MergeStrategy `json:"strategy"`            // 使用的策略
	ConflictResolution string         `json:"conflict_resolution"` // 冲突解决方式
}

// FeeOptimizationResult 费用优化结果
type FeeOptimizationResult struct {
	OriginalTotalFee             uint64  `json:"original_total_fee"`              // 原始总费用
	OptimizedFee                 uint64  `json:"optimized_fee"`                   // 优化后费用
	SavingsAmount                uint64  `json:"savings_amount"`                  // 节省金额
	SavingsPercentage            float64 `json:"savings_percentage"`              // 节省百分比
	FeeEfficiencyRatio           float64 `json:"fee_efficiency_ratio"`            // 费用效率比
	EstimatedExecutionFeeSavings uint64  `json:"estimated_execution_fee_savings"` // 预估执行费用节省
}

// UTXOOptimizationResult UTXO优化结果
type UTXOOptimizationResult struct {
	OriginalUTXOCount      int     `json:"original_utxo_count"`     // 原始UTXO数量
	OptimizedUTXOCount     int     `json:"optimized_utxo_count"`    // 优化后UTXO数量
	FragmentationReduction float64 `json:"fragmentation_reduction"` // 碎片化减少比例
	ConsolidationRatio     float64 `json:"consolidation_ratio"`     // 合并比率
	StorageEfficiencyGain  float64 `json:"storage_efficiency_gain"` // 存储效率收益
	FutureTransactionCost  int64   `json:"future_transaction_cost"` // 未来交易成本影响
}

// MergeValidationResult 合并验证结果
type MergeValidationResult struct {
	CanMerge              bool                    `json:"can_merge"`              // 是否可以合并
	Reason                string                  `json:"reason"`                 // 不能合并的原因
	EstimatedFeeSavings   float64                 `json:"estimated_fee_savings"`  // 预期费用节省百分比
	EstimatedComplexity   int                     `json:"estimated_complexity"`   // 预估复杂度
	RecommendedStrategy   *MergeStrategy          `json:"recommended_strategy"`   // 推荐策略
	RiskAssessment        *MergeRiskAssessment    `json:"risk_assessment"`        // 风险评估
	ConflictAnalysis      *ConflictAnalysisResult `json:"conflict_analysis"`      // 冲突分析
	OptimizationPotential *OptimizationPotential  `json:"optimization_potential"` // 优化潜力（复用 utxo.go 定义）
}

// MergeRiskAssessment 合并风险评估
type MergeRiskAssessment struct {
	RiskLevel        string   `json:"risk_level"`        // 风险级别 (low/medium/high/critical)
	RiskFactors      []string `json:"risk_factors"`      // 风险因素
	MitigationSteps  []string `json:"mitigation_steps"`  // 缓解措施
	RecommendProceed bool     `json:"recommend_proceed"` // 是否推荐继续
}

// ==================== 最小 RBF 请求/结果/冲突类型（接口所需） ====================

// ConflictSeverity 冲突严重程度（最小枚举，用于接口签名）
type ConflictSeverity string

const (
	ConflictSeverityLow      ConflictSeverity = "low"
	ConflictSeverityMedium   ConflictSeverity = "medium"
	ConflictSeverityHigh     ConflictSeverity = "high"
	ConflictSeverityCritical ConflictSeverity = "critical"
)

// ConflictInfo 冲突信息（最小VO，供接口返回使用）
type ConflictInfo struct {
	TxID             []byte           `json:"tx_id,omitempty"`
	ConflictType     ConflictType     `json:"conflict_type,omitempty"`
	ConflictedUTXOs  [][]byte         `json:"conflicted_utxos,omitempty"`
	ConflictSeverity ConflictSeverity `json:"conflict_severity,omitempty"`
	Reason           string           `json:"reason,omitempty"`
}

// RBFRequest RBF 请求（最小VO，供接口入参使用）
type RBFRequest struct {
	Transactions   []*transaction.Transaction `json:"transactions,omitempty"`
	TargetFee      uint64                     `json:"target_fee,omitempty"`
	NewTransaction *transaction.Transaction   `json:"new_transaction,omitempty"`
	Strategy       *RBFStrategy               `json:"strategy,omitempty"`
	Options        *RBFOptions                `json:"options,omitempty"`
}

// MergeEstimation 合并估算结果（最小VO，供接口返回使用）
type MergeEstimation struct {
	EstimatedFee       uint64        `json:"estimated_fee,omitempty"`
	Complexity         int           `json:"complexity,omitempty"`
	FeeSavings         uint64        `json:"fee_savings,omitempty"`
	ExpectedFee        uint64        `json:"expected_fee,omitempty"`
	ExpectedSize       uint64        `json:"expected_size,omitempty"`
	UTXOReduction      int           `json:"utxo_reduction,omitempty"`
	EstimatedDuration  time.Duration `json:"estimated_duration,omitempty"`
	SuccessProbability float64       `json:"success_probability,omitempty"`
}

// RBFResult RBF 处理结果（最小VO，供接口返回使用）
type RBFResult struct {
	MergedTx     *transaction.Transaction `json:"merged_tx,omitempty"`
	Savings      uint64                   `json:"savings,omitempty"`
	Success      bool                     `json:"success,omitempty"`
	Action       RBFAction                `json:"action,omitempty"`
	Message      string                   `json:"message,omitempty"`
	FinalTx      *transaction.Transaction `json:"final_tx,omitempty"`
	RemovedTxIDs [][]byte                 `json:"removed_tx_ids,omitempty"`
	Metadata     *RBFMetadata             `json:"metadata,omitempty"`
}

// RBFAction 动作类型
type RBFAction string

const (
	RBFActionFailed   RBFAction = "failed"
	RBFActionReplaced RBFAction = "replaced"
	RBFActionMerged   RBFAction = "merged"
	RBFActionRejected RBFAction = "rejected"
)

func (a RBFAction) String() string { return string(a) }

// DefaultRBFConfig 返回RBF默认配置
func DefaultRBFConfig() *RBFConfig {
	return &RBFConfig{
		Enabled:             true,
		MaxConcurrentMerges: 2,
		ProcessTimeout:      30 * time.Second,
	}
}

// RBFMetadata 处理元数据（最小VO）
type RBFMetadata struct {
	ProcessingTime   time.Duration    `json:"processing_time,omitempty"`
	OriginalTxCount  int              `json:"original_tx_count,omitempty"`
	FeeSavings       uint64           `json:"fee_savings,omitempty"`
	UTXOReduction    int              `json:"utxo_reduction,omitempty"`
	ConflictSeverity ConflictSeverity `json:"conflict_severity,omitempty"`
}

// ConflictAnalysisResult 冲突分析结果
type ConflictAnalysisResult struct {
	ConflictType        string          `json:"conflict_type"`        // 冲突类型
	ConflictedUTXOs     []*UTXOConflict `json:"conflicted_utxos"`     // 冲突的UTXO
	ConflictSeverity    string          `json:"conflict_severity"`    // 冲突严重程度
	ResolutionStrategy  string          `json:"resolution_strategy"`  // 解决策略
	EstimatedDifficulty int             `json:"estimated_difficulty"` // 预估难度
}

// UTXOConflict UTXO冲突信息
type UTXOConflict struct {
	UTXO             *transaction.OutPoint `json:"utxo"`              // 冲突的UTXO
	ConflictingTxs   [][]byte              `json:"conflicting_txs"`   // 冲突的交易ID列表
	ConflictType     string                `json:"conflict_type"`     // 冲突类型
	ConflictSeverity int                   `json:"conflict_severity"` // 冲突严重程度 (1-10)
	DetectionTime    time.Time             `json:"detection_time"`    // 检测时间
}

// FeeEstimationResult 费用估算结果
type FeeEstimationResult struct {
	EstimatedFee         uint64             `json:"estimated_fee"`         // 预估费用
	FeeBrackets          []*FeeBracket      `json:"fee_brackets"`          // 费用档次
	OptimizationAdvice   []string           `json:"optimization_advice"`   // 优化建议
	NetworkConditions    *NetworkConditions `json:"network_conditions"`    // 网络状况
	FeeComparison        *FeeComparison     `json:"fee_comparison"`        // 费用比较
	EstimationConfidence float64            `json:"estimation_confidence"` // 估算置信度 (0-1)
}

// FeeBracket 费用档次
type FeeBracket struct {
	Priority        string        `json:"priority"`         // 优先级 (low/medium/high)
	FeeAmount       uint64        `json:"fee_amount"`       // 费用金额
	EstimatedTime   time.Duration `json:"estimated_time"`   // 预估确认时间
	ConfidenceLevel float64       `json:"confidence_level"` // 置信度
}

// NetworkConditions 网络状况
type NetworkConditions struct {
	Congestion               float64   `json:"congestion"`                  // 网络拥堵程度 (0-1)
	AverageExecutionFeePrice uint64    `json:"average_execution_fee_price"` // 平均执行费用价格
	MempoolSize              int       `json:"mempool_size"`                // 内存池大小
	BlockUtilization         float64   `json:"block_utilization"`           // 区块利用率
	LastUpdateTime           time.Time `json:"last_update_time"`            // 最后更新时间
}

// FeeComparison 费用比较
type FeeComparison struct {
	OriginalTotalFee  uint64  `json:"original_total_fee"` // 原始总费用
	MergedFee         uint64  `json:"merged_fee"`         // 合并后费用
	AbsoluteSavings   uint64  `json:"absolute_savings"`   // 绝对节省
	PercentageSavings float64 `json:"percentage_savings"` // 百分比节省
	PaybackPeriod     int     `json:"payback_period"`     // 回收期（区块数）
}

// ================================================================================================
// 🔍 UTXO查询和选择相关类型定义
// ================================================================================================

// UTXOQueryFilter UTXO查询过滤器
type UTXOQueryFilter struct {
	TokenID          []byte                `json:"token_id,omitempty"`          // 代币ID过滤
	MinValue         uint64                `json:"min_value,omitempty"`         // 最小值过滤
	MaxValue         uint64                `json:"max_value,omitempty"`         // 最大值过滤
	IncludeLocked    bool                  `json:"include_locked"`              // 是否包含锁定的UTXO
	IncludePending   bool                  `json:"include_pending"`             // 是否包含待确认的UTXO
	MaxAge           time.Duration         `json:"max_age,omitempty"`           // 最大年龄
	MinAge           time.Duration         `json:"min_age,omitempty"`           // 最小年龄
	SortBy           UTXOSortStrategy      `json:"sort_by"`                     // 排序策略
	Limit            int                   `json:"limit,omitempty"`             // 结果限制
	LockingCondition *LockingConditionType `json:"locking_condition,omitempty"` // 锁定条件类型过滤
}

// UTXOSortStrategy UTXO排序策略
type UTXOSortStrategy string

const (
	UTXOSortByValue      UTXOSortStrategy = "value"      // 按价值排序
	UTXOSortByAge        UTXOSortStrategy = "age"        // 按年龄排序
	UTXOSortBySize       UTXOSortStrategy = "size"       // 按大小排序
	UTXOSortByEfficiency UTXOSortStrategy = "efficiency" // 按效率排序
)

// LockingConditionType 锁定条件类型
type LockingConditionType string

const (
	LockingConditionSingleKey  LockingConditionType = "single_key"  // 单密钥锁定
	LockingConditionMultiKey   LockingConditionType = "multi_key"   // 多密钥锁定
	LockingConditionContract   LockingConditionType = "contract"    // 合约锁定
	LockingConditionTimeLock   LockingConditionType = "time_lock"   // 时间锁定
	LockingConditionHeightLock LockingConditionType = "height_lock" // 高度锁定
)

// UTXOSelectionResult UTXO选择结果
type UTXOSelectionResult struct {
	Success         bool                `json:"success"`         // 是否成功选择
	SelectedUTXOs   []*SelectedUTXOInfo `json:"selected_utxos"`  // 选中的UTXO信息
	TotalValue      uint64              `json:"total_value"`     // 总价值
	ChangeAmount    uint64              `json:"change_amount"`   // 找零金额
	EstimatedFee    uint64              `json:"estimated_fee"`   // 预估费用
	SelectionStats  *UTXOSelectionStats `json:"selection_stats"` // 选择统计
	Recommendations []string            `json:"recommendations"` // 优化建议
	ErrorMessage    string              `json:"error_message"`   // 错误信息
}

// SelectedUTXOInfo 选中的UTXO信息
type SelectedUTXOInfo struct {
	UTXO           *transaction.OutPoint `json:"utxo"`            // UTXO引用
	Value          uint64                `json:"value"`           // 价值
	TokenID        []byte                `json:"token_id"`        // 代币ID
	SelectionScore float64               `json:"selection_score"` // 选择评分
	OptimalReason  string                `json:"optimal_reason"`  // 选择原因
}

// UTXOSelectionStats UTXO选择统计
type UTXOSelectionStats struct {
	TotalAvailableUTXOs uint32  `json:"total_available_utxos"` // 总可用UTXO数
	SelectedCount       uint32  `json:"selected_count"`        // 选中数量
	SelectionRatio      float64 `json:"selection_ratio"`       // 选择比例
	EfficiencyScore     float64 `json:"efficiency_score"`      // 效率得分
	OptimizationLevel   string  `json:"optimization_level"`    // 优化级别
}

// FragmentationAnalysis 碎片化分析结果
type FragmentationAnalysis struct {
	FragmentationIndex    float64                `json:"fragmentation_index"`    // 碎片化指数 (0-1)
	UTXODistribution      *UTXODistribution      `json:"utxo_distribution"`      // UTXO分布
	OptimizationPotential *OptimizationPotential `json:"optimization_potential"` // 优化潜力（复用 utxo.go 定义）
	RecommendedActions    []string               `json:"recommended_actions"`    // 推荐操作
	FragmentationCauses   []string               `json:"fragmentation_causes"`   // 碎片化原因
	EstimatedCost         uint64                 `json:"estimated_cost"`         // 预估整理成本
	EstimatedSavings      uint64                 `json:"estimated_savings"`      // 预估节省
	AnalysisTimestamp     time.Time              `json:"analysis_timestamp"`     // 分析时间
}

// UTXODistribution UTXO分布信息
type UTXODistribution struct {
	DustUTXOs         uint32  `json:"dust_utxos"`         // 灰尘UTXO数量
	SmallUTXOs        uint32  `json:"small_utxos"`        // 小额UTXO数量
	MediumUTXOs       uint32  `json:"medium_utxos"`       // 中等UTXO数量
	LargeUTXOs        uint32  `json:"large_utxos"`        // 大额UTXO数量
	AverageValue      uint64  `json:"average_value"`      // 平均价值
	MedianValue       uint64  `json:"median_value"`       // 中位数价值
	StandardDeviation float64 `json:"standard_deviation"` // 标准差
	GiniCoefficient   float64 `json:"gini_coefficient"`   // 基尼系数
}

// ConsolidationStrategy UTXO整理策略
type ConsolidationStrategy string

const (
	ConsolidationStrategyAggressive    ConsolidationStrategy = "aggressive"     // 激进策略
	ConsolidationStrategyConservative  ConsolidationStrategy = "conservative"   // 保守策略
	ConsolidationStrategyBalanced      ConsolidationStrategy = "balanced"       // 平衡策略
	ConsolidationStrategyCostEffective ConsolidationStrategy = "cost_effective" // 成本效益策略
)

// ConsolidationPlan UTXO整理计划
type ConsolidationPlan struct {
	Strategy             ConsolidationStrategy   `json:"strategy"`               // 整理策略
	TargetUTXOs          []*transaction.OutPoint `json:"target_utxos"`           // 目标UTXO列表
	ConsolidationBatches []*ConsolidationBatch   `json:"consolidation_batches"`  // 整理批次
	TotalCost            uint64                  `json:"total_cost"`             // 总成本
	ExpectedSavings      uint64                  `json:"expected_savings"`       // 预期节省
	EstimatedDuration    time.Duration           `json:"estimated_duration"`     // 预计持续时间
	OptimalExecutionTime time.Time               `json:"optimal_execution_time"` // 最佳执行时间
	CostBreakdown        *CostBreakdown          `json:"cost_breakdown"`         // 成本明细
	RiskAssessment       *RiskAssessment         `json:"risk_assessment"`        // 风险评估
}

// ConsolidationBatch 整理批次
type ConsolidationBatch struct {
	BatchID         string                  `json:"batch_id"`         // 批次ID
	UTXOs           []*transaction.OutPoint `json:"utxos"`            // 本批次的UTXO
	EstimatedCost   uint64                  `json:"estimated_cost"`   // 预估成本
	Priority        int                     `json:"priority"`         // 优先级
	OptimalTiming   time.Time               `json:"optimal_timing"`   // 最佳执行时间
	ExpectedSavings uint64                  `json:"expected_savings"` // 预期节省
}

// CostBreakdown 成本明细
type CostBreakdown struct {
	BaseFee          uint64 `json:"base_fee"`          // 基础费用
	PriorityFee      uint64 `json:"priority_fee"`      // 优先级费用
	NetworkFee       uint64 `json:"network_fee"`       // 网络费用
	ConsolidationFee uint64 `json:"consolidation_fee"` // 整理费用
	TotalFee         uint64 `json:"total_fee"`         // 总费用
}

// ==================== 最小 Merge 计划/统计类型（接口所需） ====================

// MergePlanRequest 合并计划请求（最小VO）
type MergePlanRequest struct {
	Transactions []*transaction.Transaction `json:"transactions,omitempty"`
	TargetFee    uint64                     `json:"target_fee,omitempty"`
	Strategy     *RBFStrategy               `json:"strategy,omitempty"`
	Constraints  *MergeConstraints          `json:"constraints,omitempty"`
}

// MergePlan 合并计划（最小VO）
type MergePlan struct {
	ID                 string                     `json:"id"`
	SourceTransactions []*transaction.Transaction `json:"source_transactions"`
	Strategy           *RBFStrategy               `json:"strategy"`
	EstimatedResult    *MergeEstimation           `json:"estimated_result"`
	Steps              []*MergeStep               `json:"steps"`
}

// TxPoolStats 交易池统计（最小VO）
type TxPoolStats struct {
	Pending int `json:"pending,omitempty"`
}

// ConsolidationBenefit 整理收益（最小VO）
type ConsolidationBenefit struct {
	EstimatedSavings uint64 `json:"estimated_savings,omitempty"`
}

// RBFConfig RBF 配置（最小VO）
type RBFConfig struct {
	Enabled             bool          `json:"enabled"`
	MaxConcurrentMerges int           `json:"max_concurrent_merges,omitempty"`
	ProcessTimeout      time.Duration `json:"process_timeout,omitempty"`
}

// ==================== 最小 RBF 策略/约束/步骤/冲突类型（接口所需） ====================

// RBFStrategy RBF 策略（最小VO）
type RBFStrategy struct {
	Method           string           `json:"method,omitempty"`
	MergePolicy      MergePolicy      `json:"merge_policy,omitempty"`
	OptimizationGoal OptimizationGoal `json:"optimization_goal,omitempty"`
	FallbackPolicy   FallbackPolicy   `json:"fallback_policy,omitempty"`
	MaxComplexity    int              `json:"max_complexity,omitempty"`
	TimeLimit        time.Duration    `json:"time_limit,omitempty"`
}

// MergePolicy 合并策略
type MergePolicy string

const (
	MergePolicyConservative MergePolicy = "conservative"
	MergePolicyBalanced     MergePolicy = "balanced"
)

func (mp MergePolicy) String() string { return string(mp) }

// FallbackPolicy 回退策略
type FallbackPolicy string

const (
	FallbackKeepOld    FallbackPolicy = "keep_old"
	FallbackAcceptNew  FallbackPolicy = "accept_new"
	FallbackRejectBoth FallbackPolicy = "reject_both"
)

// OptimizationGoal 优化目标
type OptimizationGoal string

const (
	OptimizeMinimizeFee   OptimizationGoal = "minimize_fee"
	OptimizeReduceUTXO    OptimizationGoal = "reduce_utxo"
	OptimizeMaximizeSpeed OptimizationGoal = "maximize_speed"
	OptimizeBalanced      OptimizationGoal = "balanced"
)

func (og OptimizationGoal) String() string { return string(og) }

// MergeConstraints 合并约束（最小VO）
type MergeConstraints struct {
	MaxInputs       int           `json:"max_inputs,omitempty"`
	MaxOutputs      int           `json:"max_outputs,omitempty"`
	MaxFee          uint64        `json:"max_fee,omitempty"`
	MaxSize         uint64        `json:"max_size,omitempty"`
	MaxProcessTime  time.Duration `json:"max_process_time,omitempty"`
	RequiredSigners []string      `json:"required_signers,omitempty"`
}

// MergeStep 合并步骤（最小VO）
type MergeStep struct {
	StepType    MergeStepType `json:"step_type,omitempty"`
	Description string        `json:"description,omitempty"`
	InputTxs    [][]byte      `json:"input_txs,omitempty"`
}

// MergeStepType 步骤类型
type MergeStepType string

const (
	MergeStepCombineInputs   MergeStepType = "combine_inputs"
	MergeStepOptimizeOutputs MergeStepType = "optimize_outputs"
	MergeStepCalculateFee    MergeStepType = "calculate_fee"
	MergeStepSign            MergeStepType = "sign"
)

// ConflictType 冲突类型（最小枚举）
type ConflictType string

const (
	ConflictTimeLock        ConflictType = "time_lock"
	ConflictSequenceNumber  ConflictType = "sequence_number"
	ConflictUTXODoubleSpend ConflictType = "utxo_double_spend"
)

// String 返回字符串表示
func (cs ConflictSeverity) String() string { return string(cs) }

// String 返回字符串表示
func (ct ConflictType) String() string { return string(ct) }

// ==================== 最小 RBF 选项（接口所需） ====================

// RBFOptions RBF 选项（最小VO）
type RBFOptions struct {
	MaxMergeTransactions int  `json:"max_merge_transactions,omitempty"`
	AllowPartialMerge    bool `json:"allow_partial_merge,omitempty"`
	RequireSignerConsent bool `json:"require_signer_consent,omitempty"`
	DryRun               bool `json:"dry_run,omitempty"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	RiskLevel       string            `json:"risk_level"`       // 风险级别
	RiskFactors     []string          `json:"risk_factors"`     // 风险因素
	MitigationSteps []string          `json:"mitigation_steps"` // 缓解措施
	ConfidenceLevel float64           `json:"confidence_level"` // 置信度
	RiskDetails     map[string]string `json:"risk_details"`     // 风险详情
}

// 复用 UTXO 域的 UTXOCountStats 定义，避免重复。
