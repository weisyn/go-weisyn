// manager.go
// 距离选择器管理器
//
// 主要功能：
// 1. 实现DistanceSelector接口的所有方法
// 2. 管理XOR距离计算核心逻辑
// 3. 提供简单确定性的区块选择机制
// 4. 替换复杂的多因子评分系统
// 5. 提供选择证明生成和验证
//
// 核心算法：
// Distance(candidate, parent) = XOR(BigInt(candidate.hash), BigInt(parent.hash))
// selected = argmin(Distance(candidate.BlockHash, parent.BlockHash))
//
// 设计原则：
// - 确定性：相同输入必产生相同结果
// - 简洁性：单一XOR距离计算，无复杂权重
// - 高效性：O(n)时间复杂度，微秒级选择
// - 可验证：其他节点可立即验证选择正确性
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package distance_selector

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// Manager 距离选择器管理器
type Manager struct {
	logger      log.Logger
	hashManager crypto.HashManager

	// 组件实现
	calculator *distanceCalculator
	selector   *blockDistanceSelector
	prover     *selectionProver
}

// New 创建距离选择器管理器
func New(
	logger log.Logger,
	hashManager crypto.HashManager,
) *Manager {
	calculator := newDistanceCalculator(logger, hashManager)
	selector := newBlockDistanceSelector(logger, calculator)
	prover := newSelectionProver(logger, hashManager)

	return &Manager{
		logger:      logger,
		hashManager: hashManager,
		calculator:  calculator,
		selector:    selector,
		prover:      prover,
	}
}

// CalculateDistances 计算所有候选区块与父区块的XOR距离
func (m *Manager) CalculateDistances(
	ctx context.Context,
	candidates []types.CandidateBlock,
	parentBlockHash []byte,
) ([]types.DistanceResult, error) {
	m.logger.Info("开始计算候选区块距离")

	return m.calculator.calculateAllDistances(ctx, candidates, parentBlockHash)
}

// SelectClosestBlock 选择距离最近的区块
func (m *Manager) SelectClosestBlock(
	ctx context.Context,
	distanceResults []types.DistanceResult,
) (*types.CandidateBlock, error) {
	m.logger.Info("开始选择最近距离区块")

	return m.selector.selectClosest(ctx, distanceResults)
}

// GenerateDistanceProof 生成距离选择证明
func (m *Manager) GenerateDistanceProof(
	ctx context.Context,
	selected *types.CandidateBlock,
	allResults []types.DistanceResult,
	parentBlockHash []byte,
) (*types.DistanceSelectionProof, error) {
	m.logger.Info("生成距离选择证明")

	return m.prover.generateProof(ctx, selected, allResults, parentBlockHash)
}

// VerifyDistanceSelection 验证距离选择的正确性
func (m *Manager) VerifyDistanceSelection(
	ctx context.Context,
	selected *types.CandidateBlock,
	proof *types.DistanceSelectionProof,
) error {
	m.logger.Info("验证距离选择正确性")

	return m.prover.verifySelection(ctx, selected, proof)
}

// GetDistanceStatistics 获取距离选择统计信息
func (m *Manager) GetDistanceStatistics() *types.DistanceStatistics {
	return m.calculator.getStatistics()
}
