package network

import (
	"context"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// 协议常量已迁移至 protocols.go 统一管理
// 使用 protocols.go 中定义的协议常量，与Proto定义严格对齐

// UnifiedAggregatorRouter 统一Aggregator路由器接口
// 所有网络消息统一转发给Aggregator处理，由Aggregator决定角色和路由
type UnifiedAggregatorRouter interface {
	// HandleMinerBlockSubmission 处理矿工区块提交请求
	// 输入: MinerBlockSubmission (序列化后的字节数组)
	// 输出: AggregatorBlockAcceptance (序列化后的字节数组)
	// 在Aggregator中执行距离计算和角色决策
	HandleMinerBlockSubmission(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleConsensusHeartbeat 处理共识心跳请求
	// 输入: ConsensusHeartbeat (序列化后的字节数组)
	// 输出: ConsensusHeartbeat (序列化后的字节数组，响应心跳)
	// 用途: 节点状态同步、网络健康监控
	HandleConsensusHeartbeat(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleAggregatorStatusQuery 处理聚合器状态查询请求（V2 新增）
	// 输入: AggregatorStatusQuery (序列化后的字节数组)
	// 输出: AggregatorStatusResponse (响应字节数组)
	// 用途: 提交者主动查询聚合器状态，处理广播丢失场景
	HandleAggregatorStatusQuery(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)
}

// RegisterStreamHandlers 注册共识流式协议处理器
// 🎯 简化集成层职责：协议注册、消息转发给Aggregator
// 🏗️ 基于pb/network/protocol/consensus.proto，移除复杂中继逻辑
func RegisterStreamHandlers(
	network netiface.Network,
	aggregatorRouter UnifiedAggregatorRouter,
	logger log.Logger,
) error {
	if network == nil || aggregatorRouter == nil {
		return nil
	}

	// ============================================================================
	// 矿工-聚合器区块提交协议: /weisyn/consensus/block_submission/1.0.0
	// 消息类型: MinerBlockSubmission -> AggregatorBlockAcceptance
	// ============================================================================
	if logger != nil {
		logger.Infof("🔧 [简化集成] 注册矿工区块提交协议: %s", protocols.ProtocolBlockSubmission)
	}
	if err := network.RegisterStreamHandler(protocols.ProtocolBlockSubmission, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📨 [简化集成] 转发矿工区块提交到Aggregator: from=%s, size=%d", from.String(), len(reqBytes))
		}
		return aggregatorRouter.HandleMinerBlockSubmission(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❌ [简化集成] 矿工区块提交协议注册失败: %v", err)
		}
		return err
	}
	if logger != nil {
		logger.Infof("✅ [简化集成] 矿工区块提交协议注册成功: %s", protocols.ProtocolBlockSubmission)
	}

	// ============================================================================
	// 共识心跳协议: /weisyn/consensus/heartbeat/1.0.0
	// 消息类型: ConsensusHeartbeat -> ConsensusHeartbeat
	// ============================================================================
	if logger != nil {
		logger.Infof("🔧 [简化集成] 注册共识心跳协议: %s", protocols.ProtocolConsensusHeartbeat)
	}
	if err := network.RegisterStreamHandler(protocols.ProtocolConsensusHeartbeat, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("💓 [简化集成] 转发共识心跳到Aggregator: from=%s, size=%d", from.String(), len(reqBytes))
		}
		return aggregatorRouter.HandleConsensusHeartbeat(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❌ [简化集成] 共识心跳协议注册失败: %v", err)
		}
		return err
	}
	if logger != nil {
		logger.Infof("✅ [简化集成] 共识心跳协议注册成功: %s", protocols.ProtocolConsensusHeartbeat)
	}

	// ============================================================================
	// V2 新增：聚合器状态查询协议: /weisyn/consensus/aggregator_status/1.0.0
	// 消息类型: AggregatorStatusQuery -> AggregatorStatusResponse
	// ============================================================================
	if logger != nil {
		logger.Infof("🔧 [V2] 注册聚合器状态查询协议: %s", protocols.ProtocolAggregatorStatus)
	}
	if err := network.RegisterStreamHandler(protocols.ProtocolAggregatorStatus, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📨 [V2] 转发聚合器状态查询到Aggregator: from=%s, size=%d", from.String(), len(reqBytes))
		}
		return aggregatorRouter.HandleAggregatorStatusQuery(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❌ [V2] 聚合器状态查询协议注册失败: %v", err)
		}
		return err
	}

	if logger != nil {
		logger.Infof("✅ [V2] 聚合器状态查询协议注册成功: %s", protocols.ProtocolAggregatorStatus)
	}

	return nil
}
