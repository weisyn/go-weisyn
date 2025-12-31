// handle_block_submission.go
// 区块提交协议处理器（薄网络层：按职责边界设计）
//
// 📝 **设计原则（避免“简化=未完成”的误解）**：
// 网络层只负责协议转换和基本路由，不再进行业务验证和存储操作。
// 所有的验证、存储、评估、选择等业务逻辑由聚合控制器统一处理。
//
// 🔄 **简化后的处理流程**：
// 1. 反序列化网络协议消息（MinerBlockSubmission）
// 2. 基本消息格式检查（非空字段验证）
// 3. 聚合节点选举判断（基于Kademlia距离）
// 4. 非聚合节点：转发给正确的聚合节点
// 5. 聚合节点：直接调用 ProcessAggregationRound 统一处理
//
// ✅ **移除的复杂逻辑**：
// - 严格高度校验（移动到聚合控制器）
// - 候选区块基础校验（移动到聚合控制器）
// - 候选池直接存储（移动到聚合控制器）
// - 复杂的错误处理和状态管理（统一到聚合控制器）
//
// 🎯 **设计优势**：
// - 职责单一：网络层专注协议转换，业务层专注逻辑处理
// - 错误处理简化：统一在聚合控制器中处理各种异常情况
// - 测试友好：减少网络层的复杂性，提高可测试性
// - 维护性更好：业务逻辑集中在一个地方，更容易理解和修改
//
// 作者：WES开发团队
// 创建时间：2025-09-13
// 说明：历史上这里曾包含更多逻辑；当前版本将业务判断上移到控制器是刻意的架构边界选择。

package network_handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"google.golang.org/protobuf/proto"

	chainsync "github.com/weisyn/v1/internal/core/chain/sync"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/controller"
	"github.com/weisyn/v1/pkg/types"
)

// blockSubmissionHandler 区块提交处理器
type blockSubmissionHandler struct {
	logger          log.Logger
	electionService interfaces.AggregatorElection
	chainQuery      persistence.QueryService // 统一查询服务（包含 ChainQuery 和 BlockQuery）
	candidatePool   mempool.CandidatePool
	p2pService      p2pi.Service
	netService      netiface.Network
	controller      interfaces.AggregatorController
	syncService     chain.SystemSyncService // 同步服务字段
}

// newBlockSubmissionHandler 创建区块提交处理器
func newBlockSubmissionHandler(
	logger log.Logger,
	electionService interfaces.AggregatorElection,
	chainQuery persistence.QueryService, // 统一查询服务（包含 ChainQuery 和 BlockQuery）
	candidatePool mempool.CandidatePool,
	p2pService p2pi.Service,
	netService netiface.Network,
	controller interfaces.AggregatorController,
	syncService chain.SystemSyncService, // 同步服务参数
) *blockSubmissionHandler {
	return &blockSubmissionHandler{
		logger:          logger,
		electionService: electionService,
		chainQuery:      chainQuery,
		candidatePool:   candidatePool,
		p2pService:      p2pService,
		netService:      netService,
		controller:      controller,
		syncService:     syncService, // 初始化同步服务
	}
}

// handleMinerBlockSubmission 处理矿工区块提交
//
// 🎯 **极简网络层设计**：
// 网络层只负责协议转换，不做任何业务判断：
// 1. 反序列化网络协议消息
// 2. 基本消息格式检查（协议安全要求）
// 3. 直接调用 ProcessAggregationRound 统一处理
//
// ❌ **移除的越界逻辑**：
// - 聚合节点选举判断（应在 ProcessAggregationRound 内部）
// - 区块转发逻辑（应在 ProcessAggregationRound 内部）
// - 复杂的错误分支处理（统一到业务层）
func (h *blockSubmissionHandler) handleMinerBlockSubmission(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	h.logger.Info("网络层接收区块提交 - 直接转发给聚合控制器")

	// 反序列化协议消息
	var submission protocol.MinerBlockSubmission
	if err := proto.Unmarshal(reqBytes, &submission); err != nil {
		// 🔍 详细序列化调试信息
		h.logger.Errorf("🚫 MinerBlockSubmission反序列化失败 - from=%s, size=%d, error=%v", from.String(), len(reqBytes), err)
		// 安全显示序列化数据前32字节
		displayLen := 32
		if len(reqBytes) < 32 {
			displayLen = len(reqBytes)
		}
		if displayLen > 0 {
			h.logger.Debugf("💾 序列化数据前%d字节: %x", displayLen, reqBytes[:displayLen])
		}
		return h.buildRejectionResponse("invalid message format", ""), nil
	}

	// 🔍 成功反序列化的调试信息
	blockHeight := uint64(0)
	if submission.CandidateBlock != nil && submission.CandidateBlock.Header != nil {
		blockHeight = submission.CandidateBlock.Header.Height
	}
	h.logger.Debugf("✅ MinerBlockSubmission反序列化成功 - from=%s, height=%d, size=%d", from.String(), blockHeight, len(reqBytes))

	// 基本消息格式检查（协议安全要求）
	if submission.Base == nil || submission.CandidateBlock == nil {
		return h.buildRejectionResponse("missing required fields", submission.Base.MessageId), nil
	}

	// 链ID安全验证（防止跨链攻击）
	if err := h.validateBlockChainId(submission.CandidateBlock); err != nil {
		h.logger.Warnf("拒绝区块提交 - 链ID验证失败: %v", err)
		return h.buildRejectionResponse(fmt.Sprintf("invalid chain ID: %v", err), submission.Base.MessageId), nil
	}

	// 直接调用聚合控制器统一处理
	// 聚合控制器内部将处理：选举判断、验证、存储、转发、评估、选择、分发
	// 将来源 peer 写入 ctx：用于上层同步/诊断（例如候选高度领先触发 sync 时作为 peer hint）
	if from != "" {
		ctx = chainsync.ContextWithPeerHint(ctx, from)
	}

	// V2 新增：将 submission 信息写入 context（用于重选逻辑）
	if len(submission.WaivedAggregators) > 0 || submission.RetryAttempt > 0 || len(submission.OriginalMinerPeerId) > 0 {
		waivedAggregators := make([]peer.ID, len(submission.WaivedAggregators))
		for i, waivedBytes := range submission.WaivedAggregators {
			waivedAggregators[i], _ = peer.IDFromBytes(waivedBytes)
		}
		var originalMinerPeerID peer.ID
		if len(submission.OriginalMinerPeerId) > 0 {
			originalMinerPeerID, _ = peer.IDFromBytes(submission.OriginalMinerPeerId)
		}
		submissionInfo := &controller.SubmissionInfo{
			WaivedAggregators:   waivedAggregators,
			RetryAttempt:        submission.RetryAttempt,
			OriginalMinerPeerID: originalMinerPeerID,
		}
		ctx = controller.ContextWithSubmissionInfo(ctx, submissionInfo)
	}

	if err := h.controller.ProcessAggregationRound(ctx, submission.CandidateBlock); err != nil {
		// ✅ “已处理/过期(stale)”不是错误：对端需要一个 ACK 来停止重试，否则会形成持续重发+日志刷屏。
		// 典型场景：聚合器已成功处理该高度并推进链尖后，矿工/转发节点还在重发同一高度候选。
		if strings.Contains(err.Error(), "stale candidate height") {
			if h.logger != nil {
				h.logger.Infof("⏩ stale candidate ignored (ack): from=%s height=%d err=%v", from.String(), blockHeight, err)
			}
			return h.buildAcceptanceResponse("stale candidate ignored (already processed)", submission.Base.MessageId), nil
		}

		// ✅ V2 新增：检查是否为弃权错误
		if waiverErr, ok := h.checkWaiverError(err); ok {
			if h.logger != nil {
				reasonMsg := ""
				switch waiverErr.Reason {
				case types.WaiverReasonReadOnlyMode:
					reasonMsg = "只读模式，转发至其他节点"
				case types.WaiverReasonHeightTooFarAhead:
					reasonMsg = "高度过高"
				case types.WaiverReasonAggregationInProgress:
					reasonMsg = "聚合进行中"
				default:
					reasonMsg = "未知原因"
				}
				h.logger.Infof("🔄 聚合器弃权（%s）: from=%s height=%d local_height=%d, 将触发自动转发",
					reasonMsg, from.String(), blockHeight, waiverErr.LocalHeight)
			}
			return h.buildWaiverResponse(waiverErr, submission.Base.MessageId), nil
		}

		// ✅ 关键可观测性：上层若卡在选举/链查询等处，这里会长期拿不到返回；但只要返回，就必须把原因打出来
		height := uint64(0)
		if submission.CandidateBlock != nil && submission.CandidateBlock.Header != nil {
			height = submission.CandidateBlock.Header.Height
		}
		// 非 stale / 非弃权 的失败属于“真实错误”（通常意味着本地处理链路出错），需要 ERROR 级别暴露。
		h.logger.Errorf("❌ 聚合控制器处理失败: from=%s height=%d err=%v", from.String(), height, err)
		return h.buildRejectionResponse(fmt.Sprintf("aggregation processing failed: %v", err), submission.Base.MessageId), nil
	}

	// 返回接受响应
	return h.buildAcceptanceResponse("block accepted by aggregation controller", submission.Base.MessageId), nil
}

// validateBlockHeight 过去用于网络层做严格高度校验。
// ⚠️ 当前按设计不在网络层做业务裁决，该函数已移除以避免“看起来会校验但其实没走到”的误导。

// validateBlockChainId 验证区块链ID
//
// 🔐 **安全关键**：防止跨链攻击，确保只接受本链的区块
//
// 验证策略：
// 1. 尝试从创世区块获取ChainID（最可靠）
// 2. 如果创世区块不可用，从当前链顶区块获取ChainID
// 3. 如果都不可用（系统初始化阶段），允许通过但记录警告
func (h *blockSubmissionHandler) validateBlockChainId(block *core.Block) error {
	if block == nil || block.Header == nil {
		return fmt.Errorf("区块或区块头为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var expectedChainId uint64
	chainIdSource := "unknown"

	// 策略1：从创世区块获取ChainID（最可靠）
	genesisBlock, err := h.chainQuery.GetBlockByHeight(ctx, 0)
	if err == nil && genesisBlock != nil && genesisBlock.Header != nil {
		expectedChainId = genesisBlock.Header.ChainId
		chainIdSource = "genesis_block"
	} else {
		// 策略2：从当前链顶区块获取ChainID
		chainInfo, err := h.chainQuery.GetChainInfo(ctx)
		if err == nil && chainInfo.Height > 0 {
			tipBlock, err := h.chainQuery.GetBlockByHeight(ctx, chainInfo.Height)
			if err == nil && tipBlock != nil && tipBlock.Header != nil {
				expectedChainId = tipBlock.Header.ChainId
				chainIdSource = "chain_tip"
			}
		}
	}

	// 如果无法获取本地ChainID（系统初始化阶段）
	if chainIdSource == "unknown" {
		h.logger.Warnf("⚠️  无法获取本地ChainID，跳过验证（系统可能处于初始化阶段），接收区块ChainID=%d", block.Header.ChainId)
		return nil
	}

	// 执行ChainID验证
	if block.Header.ChainId != expectedChainId {
		h.logger.Errorf("🚫 ChainID不匹配 - 拒绝区块: 期望=%d(来源:%s), 实际=%d, 高度=%d",
			expectedChainId, chainIdSource, block.Header.ChainId, block.Header.Height)
		return fmt.Errorf("chainID不匹配: 期望=%d, 实际=%d", expectedChainId, block.Header.ChainId)
	}

	h.logger.Debugf("✅ ChainID验证通过: ChainID=%d(来源:%s), 区块高度=%d",
		expectedChainId, chainIdSource, block.Header.Height)
	return nil
}

// buildAcceptanceResponse 构建接受响应
func (h *blockSubmissionHandler) buildAcceptanceResponse(reason, requestID string) []byte {
	response := &protocol.AggregatorBlockAcceptance{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
			// 不设置Signature字段，libp2p层已提供传输安全性
		},
		RequestId:        requestID,
		Accepted:         true,
		AcceptanceReason: reason,
		AggregatorPeerId: []byte(h.p2pService.Host().ID()),
		Timestamp:        uint64(time.Now().Unix()),
	}

	respBytes, err := proto.Marshal(response)
	if err != nil {
		// 序列化失败，返回空响应
		return []byte{}
	}
	return respBytes
}

// buildRejectionResponse 构建拒绝响应
func (h *blockSubmissionHandler) buildRejectionResponse(reason, requestID string) []byte {
	response := &protocol.AggregatorBlockAcceptance{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
			// 不设置Signature字段，libp2p层已提供传输安全性
		},
		RequestId:        requestID,
		Accepted:         false,
		AcceptanceReason: reason,
		AggregatorPeerId: []byte(h.p2pService.Host().ID()),
		Timestamp:        uint64(time.Now().Unix()),
		Waived:           false, // 非弃权拒绝
	}

	respBytes, err := proto.Marshal(response)
	if err != nil {
		// 序列化失败，返回空响应
		return []byte{}
	}
	return respBytes
}

// checkWaiverError 检查错误是否为弃权错误
//
// V2 新增：支持弃权错误检测
func (h *blockSubmissionHandler) checkWaiverError(err error) (*types.WaiverError, bool) {
	return types.IsWaiverError(err)
}

// buildWaiverResponse 构建弃权响应
//
// V2 新增：构建弃权响应（AggregatorBlockAcceptance.waived=true）
func (h *blockSubmissionHandler) buildWaiverResponse(waiverErr *types.WaiverError, requestID string) []byte {
	var waiverReason protocol.AggregatorBlockAcceptance_WaiverReason
	switch waiverErr.Reason {
	case types.WaiverReasonHeightTooFarAhead:
		waiverReason = protocol.AggregatorBlockAcceptance_WAIVER_HEIGHT_TOO_FAR_AHEAD
	case types.WaiverReasonAggregationInProgress:
		waiverReason = protocol.AggregatorBlockAcceptance_WAIVER_AGGREGATION_IN_PROGRESS
	case types.WaiverReasonReadOnlyMode:
		waiverReason = protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE
	default:
		waiverReason = protocol.AggregatorBlockAcceptance_WAIVER_NONE
	}

	response := &protocol.AggregatorBlockAcceptance{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
		},
		RequestId:        requestID,
		Accepted:         false, // 弃权视为不接受
		AcceptanceReason: waiverErr.Error(),
		AggregatorPeerId: []byte(h.p2pService.Host().ID()),
		Timestamp:        uint64(time.Now().Unix()),
		Waived:           true, // V2 新增：标记为弃权
		WaiverReason:     waiverReason,
		LocalHeight:      waiverErr.LocalHeight,
	}

	respBytes, err := proto.Marshal(response)
	if err != nil {
		// 序列化失败，返回空响应
		return []byte{}
	}
	return respBytes
}

// generateMessageID 生成唯一消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), "aggregator")
}
