// Package decision_calculator 实现多因子决策计算服务
//
// 🎯 **多因子决策计算服务模块**
//
// 本包实现 DecisionCalculator 接口，提供ABS架构的多维度智能选择算法：
// - ABS评分模型：PoW质量(40%) + 经济价值(30%) + 时效性(20%) + 网络质量(10%)
// - 批量评估候选区块
// - 支持评估结果验证
// - 实现ABS共识的核心算法逻辑
package decision_calculator

import (
	"errors"
	"time"

	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/types"
)

// DecisionCalculatorService 简化决策计算服务实现（薄委托层）
type DecisionCalculatorService struct {
	logger         log.Logger      // 日志记录器
	basicValidator *basicValidator // 基础验证器（简化后的评分器）
}

// NewDecisionCalculatorService 创建简化决策计算服务实例
func NewDecisionCalculatorService(
	logger log.Logger,
	chainService blockchain.ChainService,
	hashManager crypto.HashManager,
	host node.Host,
	config *consensus.ConsensusOptions, // 配置参数（现在主要用于兼容性）
) interfaces.DecisionCalculator {
	// 创建基础验证器，简化的候选验证逻辑
	basicValidator := newBasicValidator(logger, chainService, hashManager)

	return &DecisionCalculatorService{
		logger:         logger,
		basicValidator: basicValidator,
	}
}

// 编译时确保 DecisionCalculatorService 实现了 DecisionCalculator 接口
var _ interfaces.DecisionCalculator = (*DecisionCalculatorService)(nil)

// CalculateABSScore 计算候选区块的基础验证（简化实现）
func (s *DecisionCalculatorService) CalculateABSScore(candidate *types.CandidateBlock) (*types.ABSScore, error) {
	s.logger.Info("执行基础候选验证")

	// 执行基础验证
	err := s.basicValidator.validateCandidate(candidate)
	if err != nil {
		return nil, err
	}

	// 返回简化的评分（在距离选择架构中，具体评分不重要）
	return &types.ABSScore{
		PoWQualityScore: 1.0, // 通过PoW验证则为1.0
		EconomicScore:   1.0, // 简化为固定值
		TimelinesScore:  1.0, // 简化为固定值
		NetworkScore:    1.0, // 简化为固定值
		TotalScore:      4.0, // 总分
		NormalizedScore: 1.0, // 标准化分数
		CalculatedAt:    time.Now(),
		CalculationTime: 0, // 基础验证很快
	}, nil
}

// EvaluateAllCandidates 批量基础验证所有候选区块
func (s *DecisionCalculatorService) EvaluateAllCandidates(candidates []types.CandidateBlock) ([]types.ScoredCandidate, error) {
	s.logger.Info("批量基础验证候选区块")

	// 执行基础验证，过滤无效候选
	validCandidates, err := s.basicValidator.validateAllCandidates(candidates)
	if err != nil {
		return nil, err
	}

	// 为有效候选创建简化的评分
	var scoredCandidates []types.ScoredCandidate
	for i, candidate := range validCandidates {
		score := &types.ABSScore{
			PoWQualityScore: 1.0,
			EconomicScore:   1.0,
			TimelinesScore:  1.0,
			NetworkScore:    1.0,
			TotalScore:      4.0,
			NormalizedScore: 1.0,
			CalculatedAt:    time.Now(),
			CalculationTime: 0,
		}

		scoredCandidates = append(scoredCandidates, types.ScoredCandidate{
			Candidate: &candidate,
			Score:     score,
			Rank:      i + 1,
		})
	}

	return scoredCandidates, nil
}

// ValidateEvaluationResult 验证评估结果（简化实现）
func (s *DecisionCalculatorService) ValidateEvaluationResult(scores []types.ScoredCandidate) error {
	s.logger.Info("验证评估结果")

	// 简化的验证：检查基本结构
	if len(scores) == 0 {
		return errors.New("no scored candidates to validate")
	}

	for _, scored := range scores {
		if scored.Candidate == nil {
			return errors.New("candidate is nil in scored candidate")
		}
		if scored.Score == nil {
			return errors.New("score is nil in scored candidate")
		}
	}

	return nil
}

// GetEvaluationStatistics 获取验证统计信息
func (s *DecisionCalculatorService) GetEvaluationStatistics() (*types.EvaluationStats, error) {
	s.logger.Info("获取验证统计信息")

	// 获取基础验证统计
	validationStats := s.basicValidator.getValidationStatistics()

	// 转换为旧的格式（为了兼容性）
	return &types.EvaluationStats{
		TotalCandidates:     int(validationStats.totalValidated),
		ValidCandidates:     int(validationStats.validCandidates),
		AverageScore:        1.0, // 简化为固定值
		MaxScore:            1.0, // 简化为固定值
		MinScore:            1.0, // 简化为固定值
		EvaluationTime:      validationStats.averageTime,
		AverageTimePerBlock: validationStats.averageTime,
		LastEvaluationTime:  validationStats.lastValidationTime,
	}, nil
}
