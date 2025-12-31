// build_distribution.go
// 构建标准的ConsensusResultBroadcast分发消息
//
// 核心业务功能：
// 1. 构建符合consensus.proto标准的ConsensusResultBroadcast消息
// 2. 基本的消息完整性验证
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package result_distributor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	consensuspb "github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// consensusMessageBuilder 共识消息构建器
type consensusMessageBuilder struct {
	logger    log.Logger
	p2pService p2pi.Service
}

// newConsensusMessageBuilder 创建共识消息构建器
func newConsensusMessageBuilder(logger log.Logger, p2pService p2pi.Service) *consensusMessageBuilder {
	return &consensusMessageBuilder{
		logger:    logger,
		p2pService: p2pService,
	}
}

// buildConsensusResultBroadcast 构建标准的ConsensusResultBroadcast消息
func (b *consensusMessageBuilder) buildConsensusResultBroadcast(
	selected *types.CandidateBlock,
	proof *types.DistanceSelectionProof,
	totalCandidates uint32,
) (*consensuspb.ConsensusResultBroadcast, error) {
	// 基本验证
	if selected == nil {
		return nil, errors.New("selected candidate block is nil")
	}
	if proof == nil {
		return nil, errors.New("distance selection proof is nil")
	}

	// 生成消息ID
	messageID, err := b.generateMessageID()
	if err != nil {
		return nil, err
	}

	// 获取聚合器ID
	aggregatorID := b.p2pService.Host().ID()
	aggregatorBytes := []byte(aggregatorID)

	// 构建BaseMessage
	baseMessage := &consensuspb.BaseMessage{
		MessageId:     messageID,
		SenderId:      aggregatorBytes,
		TimestampUnix: time.Now().Unix(),
	}

	// 构建决策结果（使用距离语义）
	decisionResult := &consensuspb.AggregationDecisionResult{
		TotalCandidates:  totalCandidates,
		SelectedDistance: proof.SelectedDistance,
		TieBreakApplied:  proof.TieBreakingApplied,
		SelectionReason:  "xor_min_distance",
	}

	// 构建ConsensusResultBroadcast消息
	broadcast := &consensuspb.ConsensusResultBroadcast{
		Base:               baseMessage,
		SelectedBlockHash:  selected.BlockHash,
		FinalBlock:         selected.Block,
		AggregatorPeerId:   aggregatorBytes,
		DecisionResult:     decisionResult,
		BroadcastTimestamp: uint64(time.Now().Unix()),
	}

	return broadcast, nil
}

// generateMessageID 生成消息ID
func (b *consensusMessageBuilder) generateMessageID() (string, error) {
	// 生成16字节随机数
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}
