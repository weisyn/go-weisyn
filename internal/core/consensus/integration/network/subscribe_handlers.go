package network

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// 订阅协议常量已迁移至 protocols.go 统一管理
// 使用 protocols.go 中定义的主题常量，与Proto定义严格对齐

// UnifiedAggregatorSubscribeRouter 统一Aggregator订阅路由器接口
// 所有订阅消息统一转发给Aggregator处理
type UnifiedAggregatorSubscribeRouter interface {
	// HandleConsensusResultBroadcast 处理共识结果广播
	// 输入: ConsensusResultBroadcast (序列化后的字节数组)
	// 特性: 聚合器决策结果全网广播，由Aggregator统一处理状态更新
	// 流程: 解析最终区块 → 验证决策结果 → 更新本地状态
	HandleConsensusResultBroadcast(ctx context.Context, from peer.ID, topic string, data []byte) error
}

// RegisterSubscribeHandlers 注册共识订阅式协议处理器
// 🎯 简化集成层职责：订阅注册、消息转发给Aggregator
// 🏗️ 基于pb/network/protocol/consensus.proto，移除复杂处理逻辑
func RegisterSubscribeHandlers(
	network netiface.Network,
	aggregatorRouter UnifiedAggregatorSubscribeRouter,
	logger log.Logger,
) error {
	if network == nil || aggregatorRouter == nil {
		return nil
	}

	// ============================================================================
	// 共识结果广播订阅: weisyn.consensus.latest_block.v1
	// 消息类型: ConsensusResultBroadcast
	// ============================================================================
	if logger != nil {
		logger.Infof("🔧 [简化集成] 注册共识结果广播订阅: %s", protocols.TopicConsensusResult)
	}

	// 🎯 破坏性重构：强制使用 SubscribeTopic API
	topicDef := protocols.BaseTopicConsensusResult

	var (
		unsubscribe func() error
		err         error
	)

	// 强制使用类型化 Topic API
	if nt, ok := network.(interface {
		SubscribeTopic(t protocols.Topic, handler netiface.SubscribeHandler, opts ...netiface.SubscribeOption) (func() error, error)
	}); ok {
		unsubscribe, err = nt.SubscribeTopic(topicDef, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
			if logger != nil {
				logger.Debugf("📡 [简化集成] 转发共识结果广播到Aggregator: from=%s, topic=%s, size=%d", from.String(), topic, len(data))
			}
			return aggregatorRouter.HandleConsensusResultBroadcast(ctx, from, topic, data)
		})
	} else {
		if logger != nil {
			logger.Errorf("❌ network does not support SubscribeTopic API, upgrade required")
		}
		return fmt.Errorf("network does not support SubscribeTopic API, upgrade required")
	}
	if err != nil {
		if logger != nil {
			logger.Errorf("❌ [简化集成] 共识结果广播订阅失败: %v", err)
		}
		return err
	}

	if logger != nil {
		logger.Infof("✅ [简化集成] 共识结果广播订阅成功: %s", protocols.TopicConsensusResult)
	}

	// 注意：这里不立即调用unsubscribe，它应该由调用者管理生命周期
	_ = unsubscribe

	return nil
}
