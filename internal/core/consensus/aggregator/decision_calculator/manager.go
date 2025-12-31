// Package decision_calculator 实现候选区块基础验证服务
//
// 🎯 **基础验证服务模块**
//
// 本包实现 DecisionCalculator 接口，承担候选区块的基础合法性校验：
// - 基础PoW验证：验证区块是否满足工作量证明要求
// - 格式完整性验证：检查区块和交易的基本格式
// - 实际的区块选择由 distance_selector 模块中的 XOR 距离算法完成
package decision_calculator

import (
	"time"

	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// DecisionCalculatorService 决策计算服务（薄委托层）
type DecisionCalculatorService struct {
	logger         log.Logger      // 日志记录器
	basicValidator *basicValidator // 基础过滤器（不做评分；评分/选择由距离算法完成）
}

// NewDecisionCalculatorService 创建简化决策计算服务实例
func NewDecisionCalculatorService(
	logger log.Logger,
	hashManager crypto.HashManager,
	p2pService p2pi.Service,
	config *consensus.ConsensusOptions, // 配置参数（现在主要用于兼容性）
) interfaces.DecisionCalculator {
	_ = p2pService // 暂时未使用，保留参数以保持接口一致性
	// 创建基础过滤器：仅做必要的结构/格式过滤；候选选择由 distance_selector 完成（按设计）。
	basicValidator := newBasicValidator(logger, hashManager)

	return &DecisionCalculatorService{
		logger:         logger,
		basicValidator: basicValidator,
	}
}

// 编译时确保 DecisionCalculatorService 实现了 DecisionCalculator 接口
var _ interfaces.DecisionCalculator = (*DecisionCalculatorService)(nil)

// ValidateCandidate 执行候选区块的基础验证
func (s *DecisionCalculatorService) ValidateCandidate(candidate *types.CandidateBlock) (*types.CandidateValidationResult, error) {
	s.logger.Info("执行基础候选验证")

	startTime := time.Now()

	// 执行基础验证
	err := s.basicValidator.validateCandidate(candidate)
	if err != nil {
		return &types.CandidateValidationResult{
			IsValid:        false,
			ValidatedAt:    time.Now(),
			ValidationTime: time.Since(startTime).Milliseconds(),
		}, err
	}

	// 返回验证结果
	return &types.CandidateValidationResult{
		IsValid:        true,
		ValidatedAt:    time.Now(),
		ValidationTime: time.Since(startTime).Milliseconds(),
	}, nil
}

// EvaluateAllCandidates 批量基础验证所有候选区块
func (s *DecisionCalculatorService) EvaluateAllCandidates(candidates []types.CandidateBlock) ([]types.CandidateBlock, error) {
	s.logger.Info("批量基础验证候选区块")

	// 执行基础验证，过滤无效候选，直接返回通过验证的候选区块列表
	validCandidates, err := s.basicValidator.validateAllCandidates(candidates)
	if err != nil {
		return nil, err
	}

	return validCandidates, nil
}

// GetEvaluationStatistics 获取验证统计信息
func (s *DecisionCalculatorService) GetEvaluationStatistics() (*types.EvaluationStats, error) {
	s.logger.Info("获取验证统计信息")

	// 获取基础验证统计
	validationStats := s.basicValidator.getValidationStatistics()

	// 返回验证统计信息
	return &types.EvaluationStats{
		TotalCandidates:     int(validationStats.totalValidated),
		ValidCandidates:     int(validationStats.validCandidates),
		EvaluationTime:      validationStats.averageTime,
		AverageTimePerBlock: validationStats.averageTime,
		LastEvaluationTime:  validationStats.lastValidationTime,
	}, nil
}
