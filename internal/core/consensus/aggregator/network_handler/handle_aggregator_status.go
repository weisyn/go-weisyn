// handle_aggregator_status.go
// 聚合器状态查询协议处理器
//
// 🎯 **V2 新增**：处理提交者的聚合器状态查询请求
//
// 核心功能：
// 1. 接收 AggregatorStatusQuery 请求
// 2. 检查本节点是否为该高度的聚合器
// 3. 返回当前聚合状态（COLLECTING/EVALUATING/DISTRIBUTING/COMPLETED/NOT_AGGREGATOR）
// 4. 如果已完成，返回最终区块
//
// 协议映射：/weisyn/consensus/aggregator_status/1.0.0 (RPC)
//
// 作者：WES开发团队
// 创建时间：2025-12-15

package network_handler

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// aggregatorStatusHandler 聚合器状态查询处理器
type aggregatorStatusHandler struct {
	logger          log.Logger
	electionService interfaces.AggregatorElection
	stateManager    interfaces.AggregatorStateManager
	chainQuery      persistence.QueryService
	p2pService      p2pi.Service
}

// newAggregatorStatusHandler 创建聚合器状态查询处理器
func newAggregatorStatusHandler(
	logger log.Logger,
	electionService interfaces.AggregatorElection,
	stateManager interfaces.AggregatorStateManager,
	chainQuery persistence.QueryService,
	p2pService p2pi.Service,
) *aggregatorStatusHandler {
	return &aggregatorStatusHandler{
		logger:          logger,
		electionService: electionService,
		stateManager:    stateManager,
		chainQuery:      chainQuery,
		p2pService:      p2pService,
	}
}

// handleAggregatorStatusQuery 处理聚合器状态查询请求
//
// V2 新增：供提交者查询聚合器当前状态
func (h *aggregatorStatusHandler) handleAggregatorStatusQuery(
	ctx context.Context,
	from peer.ID,
	reqBytes []byte,
) ([]byte, error) {
	h.logger.Infof("📡 收到聚合器状态查询: from=%s", from.String())

	// 1. 反序列化请求
	var query protocol.AggregatorStatusQuery
	if err := proto.Unmarshal(reqBytes, &query); err != nil {
		h.logger.Errorf("❌ AggregatorStatusQuery 反序列化失败: %v", err)
		return h.buildErrorResponse("", "invalid message format"), nil
	}

	height := query.Height
	requestID := query.Base.MessageId

	h.logger.Infof("📊 查询详情: height=%d, request_id=%s", height, requestID)

	// 2. 检查本节点是否为该高度的聚合器
	isAggregator, err := h.electionService.IsAggregatorForHeight(height)
	if err != nil {
		h.logger.Errorf("❌ 聚合器选举判断失败: %v", err)
		return h.buildErrorResponse(requestID, fmt.Sprintf("election failed: %v", err)), nil
	}

	if !isAggregator {
		// 本节点不是该高度的聚合器
		h.logger.Warnf("⚠️  本节点不是高度 %d 的聚合器，返回 NOT_AGGREGATOR", height)
		return h.buildNotAggregatorResponse(requestID, height), nil
	}

	// 3. 获取当前聚合状态
	currentState := h.stateManager.GetCurrentState()
	currentHeight := h.stateManager.GetCurrentHeight()

	h.logger.Infof("📊 当前聚合状态: state=%v, current_height=%d, query_height=%d",
		currentState, currentHeight, height)

	// 4. 检查高度是否匹配
	if currentHeight != height {
		// 聚合器已处理其他高度，说明该高度已完成或未开始
		// 尝试从链上查询该高度的区块
		if h.chainQuery != nil {
			chainInfo, err := h.chainQuery.GetChainInfo(ctx)
			if err == nil && chainInfo != nil && chainInfo.Height >= height {
				// 该高度已上链，返回 COMPLETED
				finalBlock, err := h.chainQuery.GetBlockByHeight(ctx, height)
				if err == nil && finalBlock != nil {
					h.logger.Infof("✅ 该高度已完成（已上链）: height=%d", height)
					return h.buildCompletedResponse(requestID, height, finalBlock, 0), nil
				}
			}
		}
		// 高度不匹配且未上链，返回 NOT_AGGREGATOR
		h.logger.Warnf("⚠️  高度不匹配（current=%d, query=%d），返回 NOT_AGGREGATOR", currentHeight, height)
		return h.buildNotAggregatorResponse(requestID, height), nil
	}

	// 5. 根据聚合状态返回响应
	switch currentState {
	case types.AggregationStateIdle:
		// 空闲状态，尚未开始聚合
		h.logger.Infof("🔄 聚合器空闲，尚未开始聚合: height=%d", height)
		return h.buildCollectingResponse(requestID, height, 0, 0), nil

	case types.AggregationStateListening, types.AggregationStateCollecting:
		// 正在收集候选
		h.logger.Infof("📥 聚合器正在收集候选: height=%d, state=%v", height, currentState)
		// V2 新增：从 candidatePool 获取候选数量（简化实现）
		candidateCount := uint32(0)
		// TODO: 如果 candidateCollector 提供 GetCollectionProgress，可以获取更详细的信息
		return h.buildCollectingResponse(requestID, height, 0, candidateCount), nil

	case types.AggregationStateEvaluating, types.AggregationStateSelecting:
		// 正在评估/选举
		h.logger.Infof("🧮 聚合器正在评估/选举: height=%d, state=%v", height, currentState)
		candidateCount := uint32(0)
		return h.buildEvaluatingResponse(requestID, height, candidateCount), nil

	case types.AggregationStateDistributing:
		// 正在分发结果
		h.logger.Infof("📡 聚合器正在分发结果: height=%d", height)
		return h.buildDistributingResponse(requestID, height), nil

	case types.AggregationStateError:
		// 聚合错误状态
		h.logger.Warnf("🟠 聚合器处于错误状态: height=%d", height)
		return h.buildErrorResponse(requestID, "aggregator in error state"), nil

	default:
		// 未知状态
		h.logger.Warnf("⚠️  聚合器状态未知: state=%v, height=%d", currentState, height)
		return h.buildErrorResponse(requestID, fmt.Sprintf("unknown state: %v", currentState)), nil
	}
}

// buildNotAggregatorResponse 构建 NOT_AGGREGATOR 响应
func (h *aggregatorStatusHandler) buildNotAggregatorResponse(requestID string, height uint64) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId: requestID,
		State:     protocol.AggregatorStatusResponse_AGGREGATOR_STATE_NOT_AGGREGATOR,
		Height:    height,
		Reason:    protocol.AggregatorStatusResponse_REASON_WRONG_AGGREGATOR,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// buildCollectingResponse 构建 COLLECTING 响应
func (h *aggregatorStatusHandler) buildCollectingResponse(
	requestID string,
	height uint64,
	collectionWindowEndTime uint64,
	candidateCount uint32,
) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId:               requestID,
		State:                   protocol.AggregatorStatusResponse_AGGREGATOR_STATE_COLLECTING,
		Height:                  height,
		CollectionWindowEndTime: collectionWindowEndTime,
		CandidateCount:          candidateCount,
		Reason:                  protocol.AggregatorStatusResponse_REASON_WAITING_FOR_CANDIDATES,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// buildEvaluatingResponse 构建 EVALUATING 响应
func (h *aggregatorStatusHandler) buildEvaluatingResponse(
	requestID string,
	height uint64,
	candidateCount uint32,
) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId:      requestID,
		State:          protocol.AggregatorStatusResponse_AGGREGATOR_STATE_EVALUATING,
		Height:         height,
		CandidateCount: candidateCount,
		Reason:         protocol.AggregatorStatusResponse_REASON_CALCULATING_DISTANCES,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// buildDistributingResponse 构建 DISTRIBUTING 响应
func (h *aggregatorStatusHandler) buildDistributingResponse(requestID string, height uint64) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId: requestID,
		State:     protocol.AggregatorStatusResponse_AGGREGATOR_STATE_DISTRIBUTING,
		Height:    height,
		Reason:    protocol.AggregatorStatusResponse_REASON_BROADCASTING_RESULT,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// buildCompletedResponse 构建 COMPLETED 响应
func (h *aggregatorStatusHandler) buildCompletedResponse(
	requestID string,
	height uint64,
	finalBlock *core.Block,
	candidateCount uint32,
) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId:      requestID,
		State:          protocol.AggregatorStatusResponse_AGGREGATOR_STATE_COMPLETED,
		Height:         height,
		FinalBlock:     finalBlock,
		CandidateCount: candidateCount,
		Reason:         protocol.AggregatorStatusResponse_REASON_ALREADY_COMPLETED,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// buildErrorResponse 构建错误响应
func (h *aggregatorStatusHandler) buildErrorResponse(requestID string, errorMsg string) []byte {
	response := &protocol.AggregatorStatusResponse{
		Base: &protocol.BaseMessage{
			MessageId:     generateStatusQueryMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId: requestID,
		State:     protocol.AggregatorStatusResponse_AGGREGATOR_STATE_UNKNOWN,
		Reason:    protocol.AggregatorStatusResponse_REASON_NONE,
	}

	respBytes, _ := proto.Marshal(response)
	return respBytes
}

// generateStatusQueryMessageID 生成状态查询消息ID
func generateStatusQueryMessageID() string {
	return fmt.Sprintf("status_query_%d_%s", time.Now().UnixNano(), "aggregator")
}
