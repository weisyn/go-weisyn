// Package block_selector 实现区块选择服务
//
// 🎯 **区块选择服务模块**
//
// 本包实现 BlockSelector 接口，提供最优区块选择功能：
// - 基于ABS评分选择最优候选
// - 处理评分相同的平局情况
// - 验证选择结果的合法性
// - 生成选择证明
package block_selector

import (
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/types"
)

// BlockSelectorService 区块选择服务实现（薄委托层）
type BlockSelectorService struct {
	logger             log.Logger          // 日志记录器
	blockSelector      *blockSelector      // 区块选择器
	selectionValidator *selectionValidator // 选择验证器
}

// NewBlockSelectorService 创建区块选择服务实例
func NewBlockSelectorService(
	logger log.Logger,
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	keyManager crypto.KeyManager,
	host node.Host,
) interfaces.BlockSelector {
	// 创建平局处理器
	tieBreaker := newTieBreaker(logger, hashManager)

	// 创建区块选择器
	blockSelector := newBlockSelector(logger, tieBreaker)

	// 创建选择验证器
	selectionValidator := newSelectionValidator(logger, hashManager, signatureManager, keyManager, host)

	return &BlockSelectorService{
		logger:             logger,
		blockSelector:      blockSelector,
		selectionValidator: selectionValidator,
	}
}

// 编译时确保 BlockSelectorService 实现了 BlockSelector 接口
var _ interfaces.BlockSelector = (*BlockSelectorService)(nil)

// SelectBestCandidate 选择最优候选区块
func (s *BlockSelectorService) SelectBestCandidate(scores []types.ScoredCandidate) (*types.CandidateBlock, error) {
	s.logger.Info("选择最优候选区块")

	// 委托给区块选择器
	return s.blockSelector.selectBestCandidate(scores)
}

// ApplyTieBreaking 处理旧评分平局情况（兼容性方法）
func (s *BlockSelectorService) ApplyTieBreaking(tiedCandidates []types.ScoredCandidate) (*types.CandidateBlock, error) {
	s.logger.Info("处理评分平局情况（兼容性）")

	// 兼容性实现：简单选择第一个候选
	// TODO: 在新架构中，这个方法应该被距离选择算法替代
	if len(tiedCandidates) == 0 {
		return nil, types.ErrNoDistanceResults
	}

	s.logger.Info("使用兼容性tie-breaking，选择第一个候选")
	return tiedCandidates[0].Candidate, nil
}

// ApplyDistanceTieBreaking 处理距离选择平局情况
func (s *BlockSelectorService) ApplyDistanceTieBreaking(tiedDistanceResults []types.DistanceResult) (*types.CandidateBlock, error) {
	s.logger.Info("处理距离选择平局情况")

	// 委托给距离平局处理器
	return s.blockSelector.tieBreaker.applyDistanceTieBreaking(tiedDistanceResults)
}

// ❌ ValidateSelection 已移除 - 架构错误
// 聚合节点不应验证自己的选择，这是荒谬的逻辑
// 选择证明的验证应由接收节点执行，而非聚合节点自身
// func (s *BlockSelectorService) ValidateSelection(selected *types.CandidateBlock, allCandidates []types.ScoredCandidate) error {
// 	s.logger.Info("验证选择结果")
// 	return s.selectionValidator.validateSelection(selected, allCandidates)
// }

// GenerateSelectionProof 生成选择证明
func (s *BlockSelectorService) GenerateSelectionProof(selected *types.CandidateBlock, scores []types.ScoredCandidate) (*types.SelectionProof, error) {
	s.logger.Info("生成选择证明")

	// 委托给选择验证器
	return s.selectionValidator.generateSelectionProof(selected, scores)
}
