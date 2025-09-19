package runtime

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Metrics 为执行指标
type Metrics struct {
	ExecDurationMillis int64
	ResourceUsed       uint64
	PoolHitRate        float64
}

// PerformancePredictor WASM性能预测器
// 基于历史数据预测执行性能、资源需求和容量规划
type PerformancePredictor struct {
	mutex   sync.RWMutex
	enabled bool

	// 历史数据样本
	executionSamples []ExecutionSample
	resourceSamples  []ResourceSample
	loadSamples      []LoadSample

	// 配置参数
	sampleWindow     int           // 样本窗口大小
	predictionWindow time.Duration // 预测时间窗口
	updateInterval   time.Duration // 更新间隔

	// 预测模型参数
	loadTrendWeight  float64 // 负载趋势权重
	seasonalWeight   float64 // 季节性权重
	volatilityWeight float64 // 波动性权重

	// 缓存的预测结果
	lastPrediction *PredictionResult
	lastUpdateTime time.Time

	// 模型精度统计
	predictionAccuracy float64
	totalPredictions   uint64
	correctPredictions uint64
}

// ExecutionSample 执行样本
type ExecutionSample struct {
	Timestamp        time.Time     `json:"timestamp"`
	Duration         time.Duration `json:"duration"`
	ResourceUsed     uint64        `json:"resource_used"`
	MemoryUsed       uint64        `json:"memory_used"`
	ModuleComplexity int           `json:"module_complexity"`
	FunctionCalls    int           `json:"function_calls"`
	Success          bool          `json:"success"`
}

// ResourceSample 资源样本
type ResourceSample struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     uint64    `json:"memory_usage"`
	NetworkIO       uint64    `json:"network_io"`
	DiskIO          uint64    `json:"disk_io"`
	ActiveInstances int       `json:"active_instances"`
	QueueDepth      int       `json:"queue_depth"`
}

// LoadSample 负载样本
type LoadSample struct {
	Timestamp       time.Time     `json:"timestamp"`
	RequestRate     float64       `json:"request_rate"`
	ConcurrentUsers int           `json:"concurrent_users"`
	ResponseTime    time.Duration `json:"response_time"`
	ErrorRate       float64       `json:"error_rate"`
	ThroughputRPS   float64       `json:"throughput_rps"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	Timestamp         time.Time     `json:"timestamp"`
	PredictionHorizon time.Duration `json:"prediction_horizon"`

	// 性能预测
	ExpectedDuration      time.Duration `json:"expected_duration"`
	ExpectedResourceUsage uint64        `json:"expected_resource_usage"`
	ExpectedMemoryUsage   uint64        `json:"expected_memory_usage"`

	// 容量预测
	RecommendedInstances int     `json:"recommended_instances"`
	ExpectedLoad         float64 `json:"expected_load"`
	ExpectedThroughput   float64 `json:"expected_throughput"`

	// 资源预测
	ExpectedCPUUsage    float64 `json:"expected_cpu_usage"`
	ExpectedMemoryTotal uint64  `json:"expected_memory_total"`

	// 风险评估
	BottleneckRisk       float64 `json:"bottleneck_risk"`
	OverloadRisk         float64 `json:"overload_risk"`
	ResourceShortageRisk float64 `json:"resource_shortage_risk"`

	// 建议
	Recommendations []string `json:"recommendations"`

	// 置信度
	ConfidenceLevel    float64 `json:"confidence_level"`
	PredictionAccuracy float64 `json:"prediction_accuracy"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	Direction  string    `json:"direction"`  // "increasing", "decreasing", "stable"
	Slope      float64   `json:"slope"`      // 趋势斜率
	Confidence float64   `json:"confidence"` // 置信度
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	DataPoints int       `json:"data_points"`
}

// AnomalyDetection 异常检测
type AnomalyDetection struct {
	IsAnomaly    bool      `json:"is_anomaly"`
	AnomalyScore float64   `json:"anomaly_score"`
	AnomalyType  string    `json:"anomaly_type"` // "spike", "drop", "drift"
	Severity     int       `json:"severity"`     // 1-10
	Description  string    `json:"description"`
	DetectedAt   time.Time `json:"detected_at"`
}

// NewPerformancePredictor 创建性能预测器
func NewPerformancePredictor(sampleWindow int, predictionWindow time.Duration) *PerformancePredictor {
	if sampleWindow <= 0 {
		sampleWindow = 1000 // 默认保持1000个样本
	}
	if predictionWindow <= 0 {
		predictionWindow = time.Hour // 默认预测1小时
	}

	return &PerformancePredictor{
		enabled:            true,
		sampleWindow:       sampleWindow,
		predictionWindow:   predictionWindow,
		updateInterval:     5 * time.Minute,
		loadTrendWeight:    0.4,
		seasonalWeight:     0.3,
		volatilityWeight:   0.3,
		executionSamples:   make([]ExecutionSample, 0, sampleWindow),
		resourceSamples:    make([]ResourceSample, 0, sampleWindow),
		loadSamples:        make([]LoadSample, 0, sampleWindow),
		predictionAccuracy: 0.5, // 初始精度50%
	}
}

// PredictInstanceCount 根据历史指标预测所需实例数
func PredictInstanceCount(past *Metrics) (int, error) {
	if past == nil {
		return 1, fmt.Errorf("metrics cannot be nil")
	}

	// 基于历史指标的简化预测算法
	baseInstances := 1

	// 根据执行时长调整
	if past.ExecDurationMillis > 1000 { // 超过1秒
		baseInstances += int(past.ExecDurationMillis / 1000)
	}

	// 根据资源使用量调整
	if past.ResourceUsed > 100000 { // 高资源使用
		baseInstances += int(past.ResourceUsed / 100000)
	}

	// 根据池命中率调整
	if past.PoolHitRate < 0.5 { // 低命中率表示负载高
		baseInstances += 2
	} else if past.PoolHitRate < 0.8 {
		baseInstances += 1
	}

	// 设置合理的上下限
	if baseInstances < 1 {
		baseInstances = 1
	}
	if baseInstances > 20 {
		baseInstances = 20
	}

	return baseInstances, nil
}

// PredictPerformance 预测性能（高级版本）
func (pp *PerformancePredictor) PredictPerformance() (*PredictionResult, error) {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	// 检查是否需要更新预测
	if pp.lastPrediction != nil && time.Since(pp.lastUpdateTime) < pp.updateInterval {
		return pp.lastPrediction, nil
	}

	// 生成新的预测
	prediction := &PredictionResult{
		Timestamp:         time.Now(),
		PredictionHorizon: pp.predictionWindow,
		Recommendations:   make([]string, 0),
	}

	// 性能预测
	pp.predictExecutionMetrics(prediction)

	// 容量预测
	pp.predictCapacityRequirements(prediction)

	// 资源预测
	pp.predictResourceUsage(prediction)

	// 风险评估
	pp.assessRisks(prediction)

	// 生成建议
	pp.generateRecommendations(prediction)

	// 计算置信度
	prediction.ConfidenceLevel = pp.calculateConfidenceLevel()
	prediction.PredictionAccuracy = pp.predictionAccuracy

	// 缓存结果
	pp.lastPrediction = prediction
	pp.lastUpdateTime = time.Now()

	return prediction, nil
}

// AddExecutionSample 添加执行样本
func (pp *PerformancePredictor) AddExecutionSample(sample ExecutionSample) {
	if !pp.enabled {
		return
	}

	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	// 维护滑动窗口
	if len(pp.executionSamples) >= pp.sampleWindow {
		pp.executionSamples = pp.executionSamples[1:]
	}
	pp.executionSamples = append(pp.executionSamples, sample)
}

// AnalyzeTrend 分析趋势
func (pp *PerformancePredictor) AnalyzeTrend(metric string) *TrendAnalysis {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	// 简化的趋势分析
	dataPoints := len(pp.loadSamples)
	if dataPoints < 5 {
		return &TrendAnalysis{
			Direction:  "stable",
			Slope:      0.0,
			Confidence: 0.0,
			DataPoints: dataPoints,
		}
	}

	// 计算简单的线性趋势
	slope := pp.calculateLinearTrend(metric)

	direction := "stable"
	if slope > 0.05 {
		direction = "increasing"
	} else if slope < -0.05 {
		direction = "decreasing"
	}

	confidence := math.Min(1.0, float64(dataPoints)/100.0)

	return &TrendAnalysis{
		Direction:  direction,
		Slope:      slope,
		Confidence: confidence,
		StartTime:  pp.loadSamples[0].Timestamp,
		EndTime:    pp.loadSamples[dataPoints-1].Timestamp,
		DataPoints: dataPoints,
	}
}

// DetectAnomaly 检测异常
func (pp *PerformancePredictor) DetectAnomaly(currentMetric string, currentValue float64) *AnomalyDetection {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	// 简化的异常检测算法
	if len(pp.loadSamples) < 10 {
		return &AnomalyDetection{
			IsAnomaly:    false,
			AnomalyScore: 0.0,
			DetectedAt:   time.Now(),
		}
	}

	// 计算历史均值和标准差
	mean, stdDev := pp.calculateStatistics(currentMetric)

	// Z-score检测
	zScore := math.Abs((currentValue - mean) / stdDev)

	isAnomaly := zScore > 2.5 // 2.5个标准差作为异常阈值
	anomalyType := "normal"
	severity := 1

	if isAnomaly {
		if currentValue > mean {
			anomalyType = "spike"
		} else {
			anomalyType = "drop"
		}

		if zScore > 4.0 {
			severity = 9
		} else if zScore > 3.5 {
			severity = 7
		} else if zScore > 3.0 {
			severity = 5
		} else {
			severity = 3
		}
	}

	return &AnomalyDetection{
		IsAnomaly:    isAnomaly,
		AnomalyScore: zScore,
		AnomalyType:  anomalyType,
		Severity:     severity,
		Description:  pp.generateAnomalyDescription(currentMetric, currentValue, mean, zScore),
		DetectedAt:   time.Now(),
	}
}

// 内部方法

// predictExecutionMetrics 预测执行指标
func (pp *PerformancePredictor) predictExecutionMetrics(prediction *PredictionResult) {
	if len(pp.executionSamples) == 0 {
		prediction.ExpectedDuration = 100 * time.Millisecond
		prediction.ExpectedResourceUsage = 1000
		prediction.ExpectedMemoryUsage = 1024 * 1024
		return
	}

	// 计算加权平均
	totalWeight := 0.0
	weightedDuration := 0.0
	weightedResource := 0.0
	weightedMemory := 0.0

	for i, sample := range pp.executionSamples {
		// 越近期的样本权重越高
		weight := float64(i+1) / float64(len(pp.executionSamples))
		totalWeight += weight

		weightedDuration += float64(sample.Duration.Nanoseconds()) * weight
		weightedResource += float64(sample.ResourceUsed) * weight
		weightedMemory += float64(sample.MemoryUsed) * weight
	}

	prediction.ExpectedDuration = time.Duration(weightedDuration / totalWeight)
	prediction.ExpectedResourceUsage = uint64(weightedResource / totalWeight)
	prediction.ExpectedMemoryUsage = uint64(weightedMemory / totalWeight)
}

// predictCapacityRequirements 预测容量需求
func (pp *PerformancePredictor) predictCapacityRequirements(prediction *PredictionResult) {
	if len(pp.loadSamples) == 0 {
		prediction.RecommendedInstances = 2
		prediction.ExpectedLoad = 0.5
		prediction.ExpectedThroughput = 100.0
		return
	}

	// 分析负载趋势
	currentLoad := pp.loadSamples[len(pp.loadSamples)-1].RequestRate
	trend := pp.calculateLinearTrend("request_rate")

	// 预测未来负载
	futureLoad := currentLoad * (1.0 + trend*float64(pp.predictionWindow.Hours()))

	prediction.ExpectedLoad = futureLoad
	prediction.ExpectedThroughput = futureLoad * 0.8 // 假设80%的处理能力

	// 基于负载预测实例数量
	baseInstances := 2
	loadFactor := math.Max(1.0, futureLoad/100.0) // 每100请求/秒需要1个实例
	prediction.RecommendedInstances = int(math.Ceil(float64(baseInstances) * loadFactor))

	// 限制在合理范围内
	if prediction.RecommendedInstances < 1 {
		prediction.RecommendedInstances = 1
	}
	if prediction.RecommendedInstances > 100 {
		prediction.RecommendedInstances = 100
	}
}

// predictResourceUsage 预测资源使用
func (pp *PerformancePredictor) predictResourceUsage(prediction *PredictionResult) {
	if len(pp.resourceSamples) == 0 {
		prediction.ExpectedCPUUsage = 0.5
		prediction.ExpectedMemoryTotal = 1024 * 1024 * 1024 // 1GB
		return
	}

	// 计算资源使用趋势
	latestSample := pp.resourceSamples[len(pp.resourceSamples)-1]

	prediction.ExpectedCPUUsage = latestSample.CPUUsage
	prediction.ExpectedMemoryTotal = latestSample.MemoryUsage * uint64(prediction.RecommendedInstances)
}

// assessRisks 评估风险
func (pp *PerformancePredictor) assessRisks(prediction *PredictionResult) {
	// 瓶颈风险
	if prediction.ExpectedCPUUsage > 0.8 {
		prediction.BottleneckRisk = 0.8
	} else {
		prediction.BottleneckRisk = prediction.ExpectedCPUUsage
	}

	// 过载风险
	if prediction.ExpectedLoad > prediction.ExpectedThroughput {
		prediction.OverloadRisk = 0.9
	} else {
		prediction.OverloadRisk = prediction.ExpectedLoad / prediction.ExpectedThroughput
	}

	// 资源短缺风险
	memoryGiB := float64(prediction.ExpectedMemoryTotal) / (1024 * 1024 * 1024)
	if memoryGiB > 8.0 { // 假设系统内存限制为8GB
		prediction.ResourceShortageRisk = 0.8
	} else {
		prediction.ResourceShortageRisk = memoryGiB / 8.0
	}
}

// generateRecommendations 生成建议
func (pp *PerformancePredictor) generateRecommendations(prediction *PredictionResult) {
	recommendations := make([]string, 0)

	if prediction.BottleneckRisk > 0.7 {
		recommendations = append(recommendations, "CPU使用率过高，建议增加实例数量或优化代码")
	}

	if prediction.OverloadRisk > 0.8 {
		recommendations = append(recommendations, "预期负载超过处理能力，建议水平扩展")
	}

	if prediction.ResourceShortageRisk > 0.7 {
		recommendations = append(recommendations, "内存使用量接近限制，建议优化内存使用或增加资源")
	}

	if prediction.RecommendedInstances > 10 {
		recommendations = append(recommendations, "建议实例数量较多，考虑进行容量优化")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统运行状态良好，维持当前配置")
	}

	prediction.Recommendations = recommendations
}

// calculateConfidenceLevel 计算置信度
func (pp *PerformancePredictor) calculateConfidenceLevel() float64 {
	dataScore := 0.0
	if len(pp.executionSamples) > 50 {
		dataScore = 0.4
	} else {
		dataScore = float64(len(pp.executionSamples)) / 50.0 * 0.4
	}

	accuracyScore := pp.predictionAccuracy * 0.4

	stabilityScore := 0.2 // 简化为固定值

	return dataScore + accuracyScore + stabilityScore
}

// calculateLinearTrend 计算线性趋势
func (pp *PerformancePredictor) calculateLinearTrend(metric string) float64 {
	if len(pp.loadSamples) < 2 {
		return 0.0
	}

	// 简化的线性回归
	first := pp.loadSamples[0].RequestRate
	last := pp.loadSamples[len(pp.loadSamples)-1].RequestRate

	timeDiff := pp.loadSamples[len(pp.loadSamples)-1].Timestamp.Sub(pp.loadSamples[0].Timestamp).Hours()
	if timeDiff == 0 {
		return 0.0
	}

	return (last - first) / (first * timeDiff) // 相对变化率
}

// calculateStatistics 计算统计数据
func (pp *PerformancePredictor) calculateStatistics(metric string) (mean, stdDev float64) {
	if len(pp.loadSamples) == 0 {
		return 0.0, 1.0
	}

	// 计算均值
	sum := 0.0
	for _, sample := range pp.loadSamples {
		sum += sample.RequestRate
	}
	mean = sum / float64(len(pp.loadSamples))

	// 计算标准差
	variance := 0.0
	for _, sample := range pp.loadSamples {
		diff := sample.RequestRate - mean
		variance += diff * diff
	}
	variance /= float64(len(pp.loadSamples))
	stdDev = math.Sqrt(variance)

	if stdDev == 0 {
		stdDev = 1.0 // 避免除零
	}

	return mean, stdDev
}

// generateAnomalyDescription 生成异常描述
func (pp *PerformancePredictor) generateAnomalyDescription(metric string, currentValue, mean, zScore float64) string {
	if zScore <= 2.5 {
		return "正常范围内"
	}

	deviation := math.Abs(currentValue - mean)
	percentage := (deviation / mean) * 100

	if currentValue > mean {
		return fmt.Sprintf("%s异常高于平均值%.1f%%（Z-score: %.2f）", metric, percentage, zScore)
	} else {
		return fmt.Sprintf("%s异常低于平均值%.1f%%（Z-score: %.2f）", metric, percentage, zScore)
	}
}
