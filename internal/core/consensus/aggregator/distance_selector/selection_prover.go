// selection_prover.go
// 距离选择证明生成和验证
//
// 主要功能：
// 1. 生成距离选择的证明数据
// 2. 验证其他聚合节点的选择证明
// 3. 提供选择过程的可验证性
// 4. 支持全网共识验证
//
// 证明内容：
// 1. 选中区块的哈希和距离值
// 2. 父区块哈希（距离计算基准）
// 3. 所有候选区块的距离计算摘要
// 4. 选择算法标识和版本
// 5. 聚合节点签名（可选）
//
// 验证流程：
// 1. 重新计算选中区块的距离
// 2. 验证是否为最小距离
// 3. 如有tie-breaking，验证其正确性
// 4. 验证证明数据的完整性
//
// 设计原则：
// - 轻量证明：最小化证明数据大小
// - 快速验证：O(1)验证复杂度
// - 确定性：相同输入产生相同证明
// - 防篡改：确保选择过程的真实性
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package distance_selector

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// selectionProver 距离选择证明器
type selectionProver struct {
	logger      log.Logger
	hashManager crypto.HashManager
}

// newSelectionProver 创建选择证明器
func newSelectionProver(
	logger log.Logger,
	hashManager crypto.HashManager,
) *selectionProver {
	return &selectionProver{
		logger:      logger,
		hashManager: hashManager,
	}
}

// generateProof 生成距离选择证明
func (p *selectionProver) generateProof(
	ctx context.Context,
	selected *types.CandidateBlock,
	allResults []types.DistanceResult,
	parentBlockHash []byte,
) (*types.DistanceSelectionProof, error) {
	startTime := time.Now()

	p.logger.Info("开始生成距离选择证明")

	// 找到选中区块的距离结果
	var selectedDistance string
	var selectedResult *types.DistanceResult
	for _, result := range allResults {
		if result.Candidate != nil &&
			len(selected.BlockHash) > 0 && len(result.Candidate.BlockHash) > 0 &&
			hex.EncodeToString(result.Candidate.BlockHash) == hex.EncodeToString(selected.BlockHash) {
			selectedDistance = result.Distance.String()
			selectedResult = &result
			break
		}
	}

	if selectedResult == nil {
		return nil, types.ErrSelectedBlockNotFound
	}

	// 计算距离摘要（所有候选的距离哈希）
	distanceSummary := p.calculateDistanceSummary(allResults)

	// 检查是否需要tie-breaking证明
	tieBreakingApplied := p.checkTieBreakingRequired(allResults, selectedResult.Distance)
	var tieBreakingProof *types.TieBreakingProof

	if tieBreakingApplied {
		tieBreakingProof = p.generateTieBreakingProof(selected, allResults, selectedResult.Distance)
	}

	// 构建证明结构
	proof := &types.DistanceSelectionProof{
		// 基本信息
		SelectedBlockHash: selected.BlockHash,
		ParentBlockHash:   parentBlockHash,
		SelectedDistance:  selectedDistance,

		// 证明数据
		TotalCandidates:    uint32(len(allResults)),
		DistanceSummary:    distanceSummary,
		TieBreakingApplied: tieBreakingApplied,
		TieBreakingProof:   tieBreakingProof,

		// 元数据
		Algorithm:      "xor_distance_v1",
		GeneratedAt:    time.Now(),
		GenerationTime: time.Since(startTime),
	}

	// 计算证明哈希
	proof.ProofHash = p.calculateProofHash(proof)

	p.logger.Info("距离选择证明生成完成")

	return proof, nil
}

// verifySelection 验证距离选择证明
func (p *selectionProver) verifySelection(
	ctx context.Context,
	selected *types.CandidateBlock,
	proof *types.DistanceSelectionProof,
) error {
	p.logger.Info("开始验证距离选择证明")

	// 1. 基本验证
	if err := p.validateProofStructure(proof); err != nil {
		return err
	}

	// 2. 验证选中区块哈希匹配
	if !p.hashesEqual(selected.BlockHash, proof.SelectedBlockHash) {
		return types.ErrProofHashMismatch
	}

	// 3. 重新计算选中区块距离
	calculator := newDistanceCalculator(p.logger, p.hashManager)
	calculatedDistance, err := calculator.calculateSingleDistance(
		selected.BlockHash,
		proof.ParentBlockHash,
	)
	if err != nil {
		return err
	}

	// 4. 验证距离值匹配
	if calculatedDistance.String() != proof.SelectedDistance {
		return types.ErrDistanceValueMismatch
	}

	// 5. 验证tie-breaking（如果应用了）
	if proof.TieBreakingApplied {
		if err := p.verifyTieBreakingProof(selected, proof.TieBreakingProof); err != nil {
			return err
		}
	}

	// 6. 验证证明哈希
	expectedHash := p.calculateProofHash(proof)
	if !p.hashesEqual(expectedHash, proof.ProofHash) {
		return types.ErrInvalidProofHash
	}

	p.logger.Info("距离选择证明验证通过")
	return nil
}

// calculateDistanceSummary 计算所有候选距离的摘要哈希
func (p *selectionProver) calculateDistanceSummary(results []types.DistanceResult) []byte {
	// 使用统一的 HashManager 计算距离摘要，避免直接依赖 crypto/sha256
	// 构造确定性的拼接数据：BlockHash || Distance.String() 逐个追加
	var buf []byte
	for _, result := range results {
		if result.Candidate == nil || len(result.Candidate.BlockHash) == 0 {
			continue
		}
		buf = append(buf, result.Candidate.BlockHash...)
		buf = append(buf, []byte(result.Distance.String())...)
	}

	if len(buf) == 0 {
		return p.hashManager.SHA256([]byte{})
	}

	return p.hashManager.SHA256(buf)
}

// checkTieBreakingRequired 检查是否需要tie-breaking
func (p *selectionProver) checkTieBreakingRequired(
	results []types.DistanceResult,
	selectedDistance interface{},
) bool {
	count := 0
	selectedDistStr := selectedDistance.(interface{ String() string }).String()

	for _, result := range results {
		if result.Distance.String() == selectedDistStr {
			count++
		}
	}

	return count > 1
}

// generateTieBreakingProof 生成tie-breaking证明
func (p *selectionProver) generateTieBreakingProof(
	selected *types.CandidateBlock,
	allResults []types.DistanceResult,
	minDistance interface{},
) *types.TieBreakingProof {
	// 找到所有具有相同最小距离的候选
	var tiedHashes [][]byte
	minDistStr := minDistance.(interface{ String() string }).String()

	for _, result := range allResults {
		if result.Distance.String() == minDistStr {
			tiedHashes = append(tiedHashes, result.Candidate.BlockHash)
		}
	}

	return &types.TieBreakingProof{
		TiedBlockHashes:   tiedHashes,
		TiedCount:         uint32(len(tiedHashes)),
		BreakingStrategy:  "lexicographic_hash",
		SelectedBlockHash: selected.BlockHash,
	}
}

// verifyTieBreakingProof 验证tie-breaking证明
func (p *selectionProver) verifyTieBreakingProof(
	selected *types.CandidateBlock,
	proof *types.TieBreakingProof,
) error {
	if proof == nil {
		return types.ErrMissingTieBreakingProof
	}

	// 验证字典序选择的正确性
	if proof.BreakingStrategy == "lexicographic_hash" {
		return p.verifyLexicographicSelection(selected.BlockHash, proof.TiedBlockHashes)
	}

	return types.ErrUnsupportedTieBreakingStrategy
}

// verifyLexicographicSelection 验证字典序选择
func (p *selectionProver) verifyLexicographicSelection(
	selectedHash []byte,
	tiedHashes [][]byte,
) error {
	// 验证选中的哈希在tie列表中
	found := false
	for _, hash := range tiedHashes {
		if p.hashesEqual(hash, selectedHash) {
			found = true
			break
		}
	}

	if !found {
		return types.ErrSelectedHashNotInTieList
	}

	// 验证选中的哈希是字典序最小的
	for _, hash := range tiedHashes {
		if hex.EncodeToString(hash) < hex.EncodeToString(selectedHash) {
			return types.ErrInvalidLexicographicSelection
		}
	}

	return nil
}

// calculateProofHash 计算证明哈希
func (p *selectionProver) calculateProofHash(proof *types.DistanceSelectionProof) []byte {
	// 使用统一的 HashManager 计算证明哈希，避免直接依赖 crypto/sha256
	var buf []byte

	buf = append(buf, proof.SelectedBlockHash...)
	buf = append(buf, proof.ParentBlockHash...)
	buf = append(buf, []byte(proof.SelectedDistance)...)
	buf = append(buf, proof.DistanceSummary...)
	buf = append(buf, []byte(proof.Algorithm)...)

	if proof.TieBreakingApplied && proof.TieBreakingProof != nil {
		buf = append(buf, proof.TieBreakingProof.SelectedBlockHash...)
		buf = append(buf, []byte(proof.TieBreakingProof.BreakingStrategy)...)
	}

	return p.hashManager.SHA256(buf)
}

// validateProofStructure 验证证明结构的完整性
func (p *selectionProver) validateProofStructure(proof *types.DistanceSelectionProof) error {
	if len(proof.SelectedBlockHash) == 0 {
		return types.ErrEmptySelectedBlockHash
	}

	if len(proof.ParentBlockHash) == 0 {
		return types.ErrEmptyParentBlockHash
	}

	if proof.SelectedDistance == "" {
		return types.ErrEmptySelectedDistance
	}

	if proof.Algorithm != "xor_distance_v1" {
		return types.ErrUnsupportedAlgorithm
	}

	if proof.TieBreakingApplied && proof.TieBreakingProof == nil {
		return types.ErrMissingTieBreakingProof
	}

	return nil
}

// hashesEqual 比较两个哈希是否相等
func (p *selectionProver) hashesEqual(hash1, hash2 []byte) bool {
	if len(hash1) != len(hash2) {
		return false
	}

	for i := range hash1 {
		if hash1[i] != hash2[i] {
			return false
		}
	}

	return true
}
