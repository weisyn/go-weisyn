// Package router provides routing quality management functionality.
package router

// quality.go
// 质量评估（方法框架）：
// - 计算链路质量分数
// - 提供与路由引擎协作的评分接口

// QualityEstimator 质量评估器（方法框架）
type QualityEstimator struct{}

// NewQualityEstimator 创建质量评估器
func NewQualityEstimator() *QualityEstimator { return &QualityEstimator{} }

// Score 计算路径/下一跳的质量分数
func (e *QualityEstimator) Score(_sample interface{}) (float64, error) { return 0, nil }
