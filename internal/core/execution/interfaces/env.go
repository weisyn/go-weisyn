package interfaces

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// ==================== 环境智能分析内部接口 ====================
// 这些接口供execution内部子目录相互调用，不对外暴露

// MLAdvisor ML环境顾问接口
// 由env包实现，供coordinator调用
type MLAdvisor interface {
	// 建议资源限制
	AdviseResourceLimits(ctx context.Context, params types.ExecutionParams) (*ResourceAdvice, error)

	// 预测执行成本
	PredictExecutionCost(ctx context.Context, params types.ExecutionParams) (*CostPrediction, error)

	// 分析性能历史
	AnalyzePerformanceHistory(ctx context.Context, resourceID []byte) (*PerformanceAnalysis, error)

	// 优化执行配置
	OptimizeConfiguration(ctx context.Context, profile ExecutionProfile) (*OptimizedConfig, error)

	// 获取环境统计
	GetEnvironmentStats() *EnvironmentStats
}

// ResourceOptimizer 资源优化器接口
// 由env包实现，供coordinator调用
type ResourceOptimizer interface {
	// 优化内存分配
	OptimizeMemoryAllocation(ctx context.Context, requirement MemoryRequirement) (*MemoryAllocation, error)

	// 优化CPU使用
	OptimizeCPUUsage(ctx context.Context, workload CPUWorkload) (*CPUOptimization, error)

	// 优化并发度
	OptimizeConcurrency(ctx context.Context, task ConcurrencyTask) (*ConcurrencyConfig, error)

	// 获取优化建议
	GetOptimizationSuggestions(ctx context.Context, metrics PerformanceMetrics) ([]*OptimizationSuggestion, error)
}

// PerformanceAnalyzer 性能分析器接口
// 由env包实现，供monitoring调用
type PerformanceAnalyzer interface {
	// 分析执行模式
	AnalyzeExecutionPattern(ctx context.Context, executions []ExecutionRecord) (*PatternAnalysis, error)

	// 检测性能瓶颈
	DetectBottlenecks(ctx context.Context, metrics PerformanceMetrics) ([]*Bottleneck, error)

	// 预测性能趋势
	PredictPerformanceTrend(ctx context.Context, history PerformanceHistory) (*TrendPrediction, error)

	// 生成性能报告
	GeneratePerformanceReport(ctx context.Context, period ReportPeriod) (*PerformanceReport, error)
}

// ==================== 数据结构定义 ====================

// ResourceAdvice 资源建议
type ResourceAdvice struct {
	MemoryLimit       uint64   `json:"memory_limit"`
	CPULimit          uint64   `json:"cpu_limit"`
	TimeoutMs         uint64   `json:"timeout_ms"`
	ExecutionFeeLimit uint64   `json:"execution_fee_limit"`
	Confidence        float64  `json:"confidence"`
	Reasoning         string   `json:"reasoning"`
	Optimizations     []string `json:"optimizations"`
}

// CostPrediction 成本预测
type CostPrediction struct {
	EstimatedResource uint64        `json:"estimated_resource"`
	EstimatedTime     time.Duration `json:"estimated_time"`
	EstimatedMemory   uint32        `json:"estimated_memory"`
	CostRange         CostRange     `json:"cost_range"`
	Confidence        float64       `json:"confidence"`
	FactorsConsidered []string      `json:"factors_considered"`
}

// CostRange 成本范围
type CostRange struct {
	MinResource uint64 `json:"min_resource"`
	MaxResource uint64 `json:"max_resource"`
	AvgResource uint64 `json:"avg_resource"`
}

// PerformanceAnalysis 性能分析
type PerformanceAnalysis struct {
	AverageExecutionTime time.Duration               `json:"average_execution_time"`
	MedianExecutionTime  time.Duration               `json:"median_execution_time"`
	P95ExecutionTime     time.Duration               `json:"p95_execution_time"`
	SuccessRate          float64                     `json:"success_rate"`
	ErrorPatterns        []ErrorPattern              `json:"error_patterns"`
	ResourceUsage        AverageResourceUsage        `json:"resource_usage"`
	Recommendations      []PerformanceRecommendation `json:"recommendations"`
}

// ErrorPattern 错误模式
type ErrorPattern struct {
	ErrorType   string  `json:"error_type"`
	Frequency   int64   `json:"frequency"`
	Percentage  float64 `json:"percentage"`
	Description string  `json:"description"`
}

// AverageResourceUsage 平均资源使用
type AverageResourceUsage struct {
	MemoryUsage   uint32 `json:"memory_usage"`
	ResourceUsage uint64 `json:"resource_usage"`
	CPUUsage      uint32 `json:"cpu_usage"`
}

// PerformanceRecommendation 性能建议
type PerformanceRecommendation struct {
	Type        string                 `json:"type"`
	Priority    string                 `json:"priority"`
	Description string                 `json:"description"`
	Impact      string                 `json:"impact"`
	Details     map[string]interface{} `json:"details"`
}

// ExecutionProfile 执行配置文件
type ExecutionProfile struct {
	EngineType     types.EngineType     `json:"engine_type"`
	ResourceType   string               `json:"resource_type"`
	CallerProfile  CallerProfile        `json:"caller_profile"`
	HistoricalData HistoricalData       `json:"historical_data"`
	Constraints    ExecutionConstraints `json:"constraints"`
}

// CallerProfile 调用者配置文件
type CallerProfile struct {
	Address              string            `json:"address"`
	CallFrequency        int64             `json:"call_frequency"`
	AverageResourceUsage uint64            `json:"average_resource_usage"`
	PreferredLimits      map[string]uint64 `json:"preferred_limits"`
}

// HistoricalData 历史数据
type HistoricalData struct {
	ExecutionCount  int64              `json:"execution_count"`
	SuccessfulCount int64              `json:"successful_count"`
	AverageTime     time.Duration      `json:"average_time"`
	AverageResource uint64             `json:"average_resource"`
	PeakMemory      uint32             `json:"peak_memory"`
	RecentTrends    map[string]float64 `json:"recent_trends"`
}

// ExecutionConstraints 执行约束
type ExecutionConstraints struct {
	MaxMemory      uint64        `json:"max_memory"`
	MaxTime        time.Duration `json:"max_time"`
	MaxResource    uint64        `json:"max_resource"`
	AllowedImports []string      `json:"allowed_imports"`
}

// OptimizedConfig 优化配置
type OptimizedConfig struct {
	RecommendedLimits   ExecutionResourceLimits `json:"recommended_limits"`
	ConfigChanges       []ConfigChange          `json:"config_changes"`
	ExpectedImprovement ExpectedImprovement     `json:"expected_improvement"`
	Confidence          float64                 `json:"confidence"`
}

// ExecutionResourceLimits 执行资源限制
type ExecutionResourceLimits struct {
	Memory   uint64        `json:"memory"`
	CPU      uint64        `json:"cpu"`
	Timeout  time.Duration `json:"timeout"`
	Resource uint64        `json:"resource"`
}

// ConfigChange 配置变更
type ConfigChange struct {
	Parameter string      `json:"parameter"`
	OldValue  interface{} `json:"old_value"`
	NewValue  interface{} `json:"new_value"`
	Reason    string      `json:"reason"`
}

// ExpectedImprovement 预期改进
type ExpectedImprovement struct {
	TimeReduction     float64 `json:"time_reduction"`
	ResourceReduction float64 `json:"resource_reduction"`
	MemoryReduction   float64 `json:"memory_reduction"`
	SuccessRateBoost  float64 `json:"success_rate_boost"`
}

// EnvironmentStats 环境统计
type EnvironmentStats struct {
	TotalAnalyses     int64     `json:"total_analyses"`
	SuccessfulAdvices int64     `json:"successful_advices"`
	AverageConfidence float64   `json:"average_confidence"`
	TopOptimizations  []string  `json:"top_optimizations"`
	LastAnalysisTime  time.Time `json:"last_analysis_time"`
}

// MemoryRequirement 内存需求
type MemoryRequirement struct {
	MinSize     uint64                 `json:"min_size"`
	MaxSize     uint64                 `json:"max_size"`
	Pattern     string                 `json:"pattern"`
	Priority    string                 `json:"priority"`
	Constraints map[string]interface{} `json:"constraints"`
}

// MemoryAllocation 内存分配
type MemoryAllocation struct {
	AllocatedSize   uint64   `json:"allocated_size"`
	AllocationMode  string   `json:"allocation_mode"`
	GCStrategy      string   `json:"gc_strategy"`
	OptimizedLayout bool     `json:"optimized_layout"`
	Recommendations []string `json:"recommendations"`
}

// CPUWorkload CPU工作负载
type CPUWorkload struct {
	ComputeIntensity    string                 `json:"compute_intensity"`
	ParallelizationType string                 `json:"parallelization_type"`
	DataAccessPattern   string                 `json:"data_access_pattern"`
	DependencyChain     []string               `json:"dependency_chain"`
	Requirements        map[string]interface{} `json:"requirements"`
}

// CPUOptimization CPU优化
type CPUOptimization struct {
	RecommendedCores   int      `json:"recommended_cores"`
	SchedulingStrategy string   `json:"scheduling_strategy"`
	CacheOptimizations []string `json:"cache_optimizations"`
	Vectorizations     []string `json:"vectorizations"`
	ExpectedSpeedup    float64  `json:"expected_speedup"`
}

// ConcurrencyTask 并发任务
type ConcurrencyTask struct {
	TaskType             string   `json:"task_type"`
	DataSharing          string   `json:"data_sharing"`
	SynchronizationNeeds []string `json:"synchronization_needs"`
	ScalabilityTarget    int      `json:"scalability_target"`
}

// ConcurrencyConfig 并发配置
type ConcurrencyConfig struct {
	MaxWorkers         int    `json:"max_workers"`
	QueueSize          int    `json:"queue_size"`
	SyncMechanism      string `json:"sync_mechanism"`
	LoadBalancingMode  string `json:"load_balancing_mode"`
	BackpressurePolicy string `json:"backpressure_policy"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	ExecutionTime    time.Duration `json:"execution_time"`
	MemoryUsage      uint32        `json:"memory_usage"`
	ResourceConsumed uint64        `json:"resource_consumed"`
	CPUUtilization   float64       `json:"cpu_utilization"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	ErrorRate        float64       `json:"error_rate"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	Type           string              `json:"type"`
	Priority       int                 `json:"priority"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	Impact         ImpactAssessment    `json:"impact"`
	Implementation ImplementationGuide `json:"implementation"`
}

// ImpactAssessment 影响评估
type ImpactAssessment struct {
	PerformanceGain float64 `json:"performance_gain"`
	ResourceSaving  float64 `json:"resource_saving"`
	Complexity      string  `json:"complexity"`
	Risk            string  `json:"risk"`
}

// ImplementationGuide 实施指南
type ImplementationGuide struct {
	Steps         []string               `json:"steps"`
	ConfigChanges map[string]interface{} `json:"config_changes"`
	Prerequisites []string               `json:"prerequisites"`
	TestingTips   []string               `json:"testing_tips"`
}

// ExecutionRecord 执行记录
type ExecutionRecord struct {
	Timestamp        time.Time        `json:"timestamp"`
	EngineType       types.EngineType `json:"engine_type"`
	ResourceID       string           `json:"resource_id"`
	Caller           string           `json:"caller"`
	ExecutionTime    time.Duration    `json:"execution_time"`
	ResourceConsumed uint64           `json:"resource_consumed"`
	MemoryUsed       uint32           `json:"memory_used"`
	Success          bool             `json:"success"`
	ErrorType        string           `json:"error_type,omitempty"`
}

// PatternAnalysis 模式分析
type PatternAnalysis struct {
	CommonPatterns    []ExecutionPattern `json:"common_patterns"`
	AnomalousPatterns []ExecutionPattern `json:"anomalous_patterns"`
	TrendAnalysis     TrendAnalysis      `json:"trend_analysis"`
	Insights          []string           `json:"insights"`
}

// ExecutionPattern 执行模式
type ExecutionPattern struct {
	PatternID       string                 `json:"pattern_id"`
	Description     string                 `json:"description"`
	Frequency       int64                  `json:"frequency"`
	Characteristics map[string]interface{} `json:"characteristics"`
	Examples        []string               `json:"examples"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	PerformanceTrend string             `json:"performance_trend"`
	UsageTrend       string             `json:"usage_trend"`
	ErrorTrend       string             `json:"error_trend"`
	Predictions      map[string]float64 `json:"predictions"`
}

// Bottleneck 瓶颈
type Bottleneck struct {
	Type        string               `json:"type"`
	Location    string               `json:"location"`
	Severity    string               `json:"severity"`
	Description string               `json:"description"`
	Impact      BottleneckImpact     `json:"impact"`
	Solutions   []BottleneckSolution `json:"solutions"`
}

// BottleneckImpact 瓶颈影响
type BottleneckImpact struct {
	PerformanceDegradation float64 `json:"performance_degradation"`
	ResourceWaste          float64 `json:"resource_waste"`
	UserExperienceImpact   string  `json:"user_experience_impact"`
}

// BottleneckSolution 瓶颈解决方案
type BottleneckSolution struct {
	SolutionID    string   `json:"solution_id"`
	Description   string   `json:"description"`
	Effort        string   `json:"effort"`
	Effectiveness float64  `json:"effectiveness"`
	Steps         []string `json:"steps"`
}

// PerformanceHistory 性能历史
type PerformanceHistory struct {
	TimeRange    TimeRange               `json:"time_range"`
	DataPoints   []PerformanceDataPoint  `json:"data_points"`
	Aggregations PerformanceAggregations `json:"aggregations"`
}

// TimeRange 时间范围
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// PerformanceDataPoint 性能数据点
type PerformanceDataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Metrics   PerformanceMetrics     `json:"metrics"`
	Context   map[string]interface{} `json:"context"`
}

// PerformanceAggregations 性能聚合
type PerformanceAggregations struct {
	Average PerformanceMetrics `json:"average"`
	Median  PerformanceMetrics `json:"median"`
	P95     PerformanceMetrics `json:"p95"`
	P99     PerformanceMetrics `json:"p99"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	PredictionHorizon time.Duration         `json:"prediction_horizon"`
	PredictedMetrics  []PredictedMetric     `json:"predicted_metrics"`
	Confidence        float64               `json:"confidence"`
	Factors           []TrendFactor         `json:"factors"`
	Recommendations   []TrendRecommendation `json:"recommendations"`
}

// PredictedMetric 预测指标
type PredictedMetric struct {
	MetricName     string  `json:"metric_name"`
	CurrentValue   float64 `json:"current_value"`
	PredictedValue float64 `json:"predicted_value"`
	ChangePercent  float64 `json:"change_percent"`
	Confidence     float64 `json:"confidence"`
}

// TrendFactor 趋势因子
type TrendFactor struct {
	FactorName  string  `json:"factor_name"`
	Impact      float64 `json:"impact"`
	Explanation string  `json:"explanation"`
}

// TrendRecommendation 趋势建议
type TrendRecommendation struct {
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Priority int    `json:"priority"`
	Timeline string `json:"timeline"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Interval  string    `json:"interval"`
}

// PerformanceReport 性能报告
type PerformanceReport struct {
	Period          ReportPeriod           `json:"period"`
	Summary         ReportSummary          `json:"summary"`
	DetailedMetrics DetailedMetrics        `json:"detailed_metrics"`
	TopIssues       []PerformanceIssue     `json:"top_issues"`
	Improvements    []PerformanceGain      `json:"improvements"`
	Recommendations []ActionRecommendation `json:"recommendations"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalExecutions  int64         `json:"total_executions"`
	SuccessRate      float64       `json:"success_rate"`
	AverageTime      time.Duration `json:"average_time"`
	AverageResource  uint64        `json:"average_resource"`
	PeakMemory       uint32        `json:"peak_memory"`
	ThroughputChange float64       `json:"throughput_change"`
}

// DetailedMetrics 详细指标
type DetailedMetrics struct {
	TimeDistribution     Distribution `json:"time_distribution"`
	ResourceDistribution Distribution `json:"resource_distribution"`
	MemoryDistribution   Distribution `json:"memory_distribution"`
	ErrorDistribution    Distribution `json:"error_distribution"`
}

// Distribution 分布
type Distribution struct {
	Min     float64          `json:"min"`
	Max     float64          `json:"max"`
	Mean    float64          `json:"mean"`
	Median  float64          `json:"median"`
	P95     float64          `json:"p95"`
	P99     float64          `json:"p99"`
	Buckets map[string]int64 `json:"buckets"`
}

// PerformanceIssue 性能问题
type PerformanceIssue struct {
	IssueType   string  `json:"issue_type"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Frequency   int64   `json:"frequency"`
	Impact      float64 `json:"impact"`
}

// PerformanceGain 性能提升
type PerformanceGain struct {
	Area        string  `json:"area"`
	Description string  `json:"description"`
	Improvement float64 `json:"improvement"`
	Cause       string  `json:"cause"`
}

// ActionRecommendation 行动建议
type ActionRecommendation struct {
	Priority    int     `json:"priority"`
	Action      string  `json:"action"`
	Rationale   string  `json:"rationale"`
	ExpectedROI float64 `json:"expected_roi"`
	Timeline    string  `json:"timeline"`
}
