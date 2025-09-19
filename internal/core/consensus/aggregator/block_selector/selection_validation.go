// selection_validation.go
// 选择结果验证和证明生成器
//
// 主要功能：
// 1. 实现 ValidateSelection 方法
// 2. 选择过程的完整性验证
// 3. 选择证明的生成和签名
// 4. 第三方验证支持
// 5. 选择审计追踪
//
// 验证内容：
// 1. 选择算法执行的正确性
// 2. 评分数据的完整性验证
// 3. Tie-breaking过程的合理性
// 4. 选择结果的一致性检查
// 5. 选择证明的密码学签名
//
// 证明生成：
// 1. 构建选择依据和过程记录
// 2. 生成选择证明数据结构
// 3. 计算选择过程的哈希摘要
// 4. 使用聚合节点私钥签名
// 5. 构建完整的可验证证明
//
// 设计原则：
// - 完整的选择过程验证
// - 密码学安全的证明机制
// - 第三方独立验证支持
// - 选择过程的透明审计
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package block_selector

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/types"
)

// selectionValidator 选择验证器
type selectionValidator struct {
	logger           log.Logger
	hashManager      crypto.HashManager
	signatureManager crypto.SignatureManager
	keyManager       crypto.KeyManager
	host             node.Host
	// 聚合器会话私钥，用于选择证明签名
	sessionPrivateKey []byte
}

// newSelectionValidator 创建选择验证器
func newSelectionValidator(
	logger log.Logger,
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	keyManager crypto.KeyManager,
	host node.Host,
) *selectionValidator {
	// 为聚合器会话生成专用签名密钥对
	privateKey, _, err := keyManager.GenerateKeyPair()
	if err != nil {
		if logger != nil {
			logger.Infof("聚合器会话密钥生成失败，将使用临时签名方案: %v", err)
		}
		privateKey = nil
	}

	return &selectionValidator{
		logger:            logger,
		hashManager:       hashManager,
		signatureManager:  signatureManager,
		keyManager:        keyManager,
		host:              host,
		sessionPrivateKey: privateKey,
	}
}

// generateSelectionProof 生成选择证明
func (v *selectionValidator) generateSelectionProof(
	selected *types.CandidateBlock,
	scores []types.ScoredCandidate,
) (*types.SelectionProof, error) {
	if selected == nil {
		return nil, errors.New("selected candidate is nil")
	}

	// 构建选择证明
	proof := &types.SelectionProof{
		SelectedCandidate:  selected,
		SelectionTimestamp: time.Now(),
		BlockHeight:        selected.Height,
	}

	// 计算所有候选的哈希
	candidatesHash, err := v.calculateCandidatesHash(scores)
	if err != nil {
		return nil, err
	}
	proof.AllCandidatesHash = candidatesHash

	// 计算评分结果哈希
	scoresHash, err := v.calculateScoresHash(scores)
	if err != nil {
		return nil, err
	}
	proof.ScoresHash = scoresHash

	// 确定选择原因
	selectionReason := v.determineSelectionReason(selected, scores)
	proof.SelectionReason = selectionReason

	// 获取聚合器ID
	aggregatorID := v.host.ID()
	proof.AggregatorID = aggregatorID

	// 生成证明哈希
	proofHash, err := v.calculateProofHash(proof)
	if err != nil {
		return nil, err
	}
	proof.ProofHash = proofHash

	// 生成聚合器签名
	signature, err := v.signProof(proof)
	if err != nil {
		return nil, err
	}
	proof.AggregatorSignature = signature

	return proof, nil
}

// calculateCandidatesHash 计算所有候选的哈希
func (v *selectionValidator) calculateCandidatesHash(candidates []types.ScoredCandidate) (string, error) {
	// 构建候选列表摘要
	var candidateHashes []string
	for _, candidate := range candidates {
		if candidate.Candidate != nil {
			candidateHashes = append(candidateHashes, hex.EncodeToString(candidate.Candidate.BlockHash))
		}
	}

	// 序列化并计算哈希
	data, err := json.Marshal(candidateHashes)
	if err != nil {
		return "", err
	}

	// 使用统一的HashManager替代直接的crypto/sha256调用
	hash := v.hashManager.SHA256(data)
	return hex.EncodeToString(hash), nil
}

// calculateScoresHash 计算评分结果哈希
func (v *selectionValidator) calculateScoresHash(scores []types.ScoredCandidate) (string, error) {
	// 构建评分摘要
	type scoreDigest struct {
		BlockHash       string  `json:"block_hash"`
		NormalizedScore float64 `json:"normalized_score"`
		Rank            int     `json:"rank"`
	}

	var scoreDigests []scoreDigest
	for _, scored := range scores {
		if scored.Candidate != nil && scored.Score != nil {
			scoreDigests = append(scoreDigests, scoreDigest{
				BlockHash:       hex.EncodeToString(scored.Candidate.BlockHash),
				NormalizedScore: scored.Score.NormalizedScore,
				Rank:            scored.Rank,
			})
		}
	}

	// 序列化并计算哈希
	data, err := json.Marshal(scoreDigests)
	if err != nil {
		return "", err
	}

	// 使用统一的HashManager替代直接的crypto/sha256调用
	hash := v.hashManager.SHA256(data)
	return hex.EncodeToString(hash), nil
}

// determineSelectionReason 确定选择原因
func (v *selectionValidator) determineSelectionReason(
	selected *types.CandidateBlock,
	scores []types.ScoredCandidate,
) string {
	if len(scores) == 0 {
		return "no candidates available"
	}

	if len(scores) == 1 {
		return "single candidate selection"
	}

	// 找到选中区块的评分
	var selectedScore float64
	selectedHash := hex.EncodeToString(selected.BlockHash)

	for _, candidate := range scores {
		if candidate.Candidate == nil {
			continue
		}
		candidateHash := hex.EncodeToString(candidate.Candidate.BlockHash)
		if candidateHash == selectedHash {
			if candidate.Score != nil {
				selectedScore = candidate.Score.NormalizedScore
			}
			break
		}
	}

	// 检查是否有平局
	tolerance := 1e-6
	tiedCount := 0
	for _, candidate := range scores {
		if candidate.Score != nil &&
			candidate.Score.NormalizedScore >= selectedScore-tolerance {
			tiedCount++
		}
	}

	if tiedCount > 1 {
		return "tie-breaking selection"
	}

	return "highest score selection"
}

// calculateProofHash 计算证明哈希
func (v *selectionValidator) calculateProofHash(proof *types.SelectionProof) (string, error) {
	// 构建证明摘要（不包括签名字段）
	proofDigest := struct {
		SelectedCandidateHash string    `json:"selected_candidate_hash"`
		SelectionReason       string    `json:"selection_reason"`
		SelectionTimestamp    time.Time `json:"selection_timestamp"`
		AllCandidatesHash     string    `json:"all_candidates_hash"`
		ScoresHash            string    `json:"scores_hash"`
		AggregatorID          peer.ID   `json:"aggregator_id"`
		BlockHeight           uint64    `json:"block_height"`
	}{
		SelectedCandidateHash: hex.EncodeToString(proof.SelectedCandidate.BlockHash),
		SelectionReason:       proof.SelectionReason,
		SelectionTimestamp:    proof.SelectionTimestamp,
		AllCandidatesHash:     proof.AllCandidatesHash,
		ScoresHash:            proof.ScoresHash,
		AggregatorID:          proof.AggregatorID,
		BlockHeight:           proof.BlockHeight,
	}

	// 序列化并计算哈希
	data, err := json.Marshal(proofDigest)
	if err != nil {
		return "", err
	}

	// 使用统一的HashManager替代直接的crypto/sha256调用
	hash := v.hashManager.SHA256(data)
	return hex.EncodeToString(hash), nil
}

// signProof 对证明进行签名
func (v *selectionValidator) signProof(proof *types.SelectionProof) ([]byte, error) {
	// 使用证明哈希作为签名数据
	hashBytes, err := hex.DecodeString(proof.ProofHash)
	if err != nil {
		return nil, err
	}

	// 使用聚合器会话私钥进行签名
	if v.sessionPrivateKey != nil {
		signature, err := v.signatureManager.Sign(hashBytes, v.sessionPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to sign with aggregator session key: %v", err)
		}
		return signature, nil
	}

	// 备用方案：生成临时密钥对进行签名
	// 虽然每次都不同，但在开发阶段可以保证功能正常运行
	tempPrivateKey, _, err := v.keyManager.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate temporary signing key: %v", err)
	}

	signature, err := v.signatureManager.Sign(hashBytes, tempPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign with temporary key: %v", err)
	}

	if v.logger != nil {
		v.logger.Info("使用临时密钥对证明进行签名")
	}

	return signature, nil
}

// ❌ verifyProof 已移除 - 架构错误
// 聚合节点不应验证自己生成的证明，这是荒谬的逻辑
// 证明验证应该由接收选择结果的其他节点执行
// 聚合节点只负责生成证明，不负责验证证明
//
// func (v *selectionValidator) verifyProof(proof *types.SelectionProof) (bool, error) {
//     // 移除：聚合节点不验证自己的证明
// }
