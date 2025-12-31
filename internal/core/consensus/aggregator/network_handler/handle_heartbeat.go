// handle_heartbeat.go
// 共识心跳协议处理器
//
// 主要功能：
// 1. 实现 HandleConsensusHeartbeat 方法
// 2. 处理共识层的心跳协议
// 3. 聚合节点状态同步
// 4. 网络健康状态监控
// 5. 节点可用性检测
//
// 处理流程：
// 1. 反序列化心跳消息（ConsensusHeartbeat）
// 2. 验证心跳消息的有效性
// 3. 更新节点状态信息
// 4. 构造心跳响应消息
// 5. 返回本地节点状态信息
//
// 心跳信息：
// - 节点ID和角色信息
// - 当前聚合状态
// - 网络连接状态
// - 时间戳信息
//
// 设计原则：
// - 轻量级心跳协议减少网络负载
// - 快速响应确保实时性
// - 状态信息同步保证网络一致性
// - 异常检测提升网络健壮性
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package network_handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// consensusHeartbeatHandler 共识心跳处理器
type consensusHeartbeatHandler struct {
	logger     log.Logger
	chainQuery persistence.ChainQuery
	p2pService p2pi.Service
	syncService chain.SystemSyncService
}

// newConsensusHeartbeatHandler 创建共识心跳处理器
func newConsensusHeartbeatHandler(
	logger log.Logger,
	chainQuery persistence.ChainQuery,
	p2pService p2pi.Service,
	syncService chain.SystemSyncService,
) *consensusHeartbeatHandler {
	return &consensusHeartbeatHandler{
		logger:      logger,
		chainQuery:  chainQuery,
		p2pService:  p2pService,
		syncService: syncService,
	}
}

// handleConsensusHeartbeat 处理共识心跳协议
func (h *consensusHeartbeatHandler) handleConsensusHeartbeat(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	h.logger.Info("处理共识心跳协议")

	// 反序列化心跳消息
	var heartbeat protocol.ConsensusHeartbeat
	if err := proto.Unmarshal(reqBytes, &heartbeat); err != nil {
		return h.buildErrorResponse("invalid heartbeat message format"), nil
	}

	// 验证心跳消息的有效性
	if err := h.validateHeartbeat(&heartbeat, from); err != nil {
		return h.buildErrorResponse(fmt.Sprintf("heartbeat validation failed: %v", err)), nil
	}

	// 构建心跳响应
	response, err := h.buildHeartbeatResponse(ctx)
	if err != nil {
		return h.buildErrorResponse(fmt.Sprintf("failed to build response: %v", err)), nil
	}

	// 序列化响应
	respBytes, err := proto.Marshal(response)
	if err != nil {
		return h.buildErrorResponse("failed to marshal response"), nil
	}

	return respBytes, nil
}

// validateHeartbeat 验证心跳消息的有效性
func (h *consensusHeartbeatHandler) validateHeartbeat(heartbeat *protocol.ConsensusHeartbeat, from peer.ID) error {
	// 验证基础消息结构
	if heartbeat.Base == nil {
		return errors.New("missing base message")
	}

	// 验证发送者ID一致性
	if string(heartbeat.Base.SenderId) != string(from) {
		return errors.New("sender ID mismatch")
	}

	// 验证时间戳合理性
	now := time.Now().Unix()
	msgTime := heartbeat.Base.TimestampUnix

	// 允许5分钟的时钟偏差
	if msgTime > now+300 || msgTime < now-300 {
		return fmt.Errorf("invalid timestamp: %d, current: %d", msgTime, now)
	}

	// 验证节点状态的合理性
	if heartbeat.NodeStatus == protocol.ConsensusHeartbeat_NODE_STATUS_UNKNOWN {
		return errors.New("unknown node status")
	}

	return nil
}

// buildHeartbeatResponse 构建心跳响应
func (h *consensusHeartbeatHandler) buildHeartbeatResponse(ctx context.Context) (*protocol.ConsensusHeartbeat, error) {
	// 获取当前链状态
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %v", err)
	}

	// 获取连接的节点数量
	libp2pHost := h.p2pService.Host()
	connectedPeers := 0
	if libp2pHost != nil {
		connectedPeers = len(libp2pHost.Network().Peers())
	}

	// 确定当前节点状态
	nodeStatus := h.determineNodeStatus(ctx)

	// 构建响应
	response := &protocol.ConsensusHeartbeat{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
			// 不设置Signature字段，libp2p层已提供传输安全性
		},
		NodeStatus:      nodeStatus,
		LastBlockHeight: chainInfo.Height,
		LastBlockHash:   chainInfo.BestBlockHash,
		ConnectedPeers:  uint32(connectedPeers),
	}

	return response, nil
}

// determineNodeStatus 确定当前节点状态
func (h *consensusHeartbeatHandler) determineNodeStatus(ctx context.Context) protocol.ConsensusHeartbeat_NodeStatus {
	// 检查链是否就绪
	isReady, err := h.chainQuery.IsReady(ctx)
	if err != nil || !isReady {
		return protocol.ConsensusHeartbeat_NODE_STATUS_SYNCING
	}

	// 使用 SystemSyncService.CheckSync 实时检查同步状态，避免依赖已废弃的持久化状态
	if h.syncService == nil {
		return protocol.ConsensusHeartbeat_NODE_STATUS_SYNCING
	}

	status, err := h.syncService.CheckSync(ctx)
	if err != nil || status == nil {
		return protocol.ConsensusHeartbeat_NODE_STATUS_SYNCING
	}

	// 判定节点是否“足够同步”：
	// 1. 状态为 Synced，或处于 Syncing 且高度差在允许范围内
	// 2. 网络高度与本地高度差在 0 或 1 之内
	var heightLag uint64
	if status.NetworkHeight > status.CurrentHeight {
		heightLag = status.NetworkHeight - status.CurrentHeight
	}
	const maxAllowedLag = uint64(1)

	isSyncedState := status.Status == types.SyncStatusSynced ||
		(status.Status == types.SyncStatusSyncing && heightLag <= maxAllowedLag)

	if !isSyncedState || heightLag > maxAllowedLag {
		return protocol.ConsensusHeartbeat_NODE_STATUS_SYNCING
	}

	// 节点处于活跃状态
	// 注意：更具体的状态（挖矿中、聚合中）需要从相应的组件获取
	// 当前设计：ConsensusHeartbeat 只区分“同步中/可服务”，不把挖矿/聚合等业务状态塞到心跳里。
	return protocol.ConsensusHeartbeat_NODE_STATUS_ACTIVE
}

// buildErrorResponse 构建错误响应
func (h *consensusHeartbeatHandler) buildErrorResponse(reason string) []byte {
	errorResponse := &protocol.ConsensusHeartbeat{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(h.p2pService.Host().ID()),
			TimestampUnix: time.Now().Unix(),
			// 不设置Signature字段，libp2p层已提供传输安全性
		},
		NodeStatus:      protocol.ConsensusHeartbeat_NODE_STATUS_OFFLINE,
		LastBlockHeight: 0,
		LastBlockHash:   []byte{},
		ConnectedPeers:  0,
	}

	respBytes, err := proto.Marshal(errorResponse)
	if err != nil {
		// 序列化失败，返回空响应
		return []byte{}
	}
	return respBytes
}
