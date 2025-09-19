package types

// OptimizationPotential 优化潜力（统一定义）
//
// 说明：
// - 该类型为通用优化潜力评估结果，供 RBF/UTXO 等模块复用；
// - 不依赖外部 pb 类型，符合 pkg/types 边界约束。
type OptimizationPotential struct {
	CanOptimize       bool   `json:"can_optimize"`       // 是否可优化
	RecommendedAction string `json:"recommended_action"` // 推荐操作
	PotentialSavings  uint64 `json:"potential_savings"`  // 潜在节省
	OptimizationScore uint32 `json:"optimization_score"` // 优化评分
	Priority          string `json:"priority"`           // 优先级
}
