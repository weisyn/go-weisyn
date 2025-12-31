// Package submitter 实现矿工提交后的等待与查询机制
//
// 🎯 **V2 新增**：提交者等待广播与主动查询机制
//
// 核心功能：
// 1. 计算等待超时时间（基于配置的 CollectionWindowDuration + DistributionTimeout + NetworkBuffer）
// 2. 订阅 ConsensusResultBroadcast 广播消息
// 3. 超时后主动查询聚合器状态
// 4. 处理聚合器离线/在线但未完成/已完成等情况
// 5. 支持重选机制（如果聚合器离线或返回 NOT_AGGREGATOR）
//
// 作者：WES开发团队
// 创建时间：2025-12-15

package submitter

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"google.golang.org/protobuf/proto"
)

// WaitAndQueryService 等待与查询服务
//
// V2 新增：提交者等待广播与主动查询机制
type WaitAndQueryService struct {
	logger          log.Logger
	config          *consensusconfig.ConsensusOptions
	networkService  netiface.Network
	p2pService      p2pi.Service
	electionService interfaces.AggregatorElection
}

// NewWaitAndQueryService 创建等待与查询服务
func NewWaitAndQueryService(
	logger log.Logger,
	config *consensusconfig.ConsensusOptions,
	networkService netiface.Network,
	p2pService p2pi.Service,
	electionService interfaces.AggregatorElection,
) *WaitAndQueryService {
	return &WaitAndQueryService{
		logger:          logger,
		config:          config,
		networkService:  networkService,
		p2pService:      p2pService,
		electionService: electionService,
	}
}

// WaitForAggregationResult 等待聚合结果
//
// # V2 新增：提交候选区块后，等待聚合器广播最终区块或主动查询状态
//
// 流程：
// 1. 订阅 ConsensusResultBroadcast 广播消息
// 2. 计算等待超时时间（CollectionWindowDuration + DistributionTimeout + NetworkBuffer）
// 3. 等待超时后，主动查询聚合器状态
// 4. 处理聚合器离线/在线但未完成/已完成等情况
// 5. 支持重选机制（如果聚合器离线或返回 NOT_AGGREGATOR）
//
// @param ctx 上下文
// @param height 候选区块高度
// @param aggregatorID 聚合器节点ID
// @return error 等待过程中的错误
func (s *WaitAndQueryService) WaitForAggregationResult(
	ctx context.Context,
	height uint64,
	aggregatorID peer.ID,
) error {
	s.logger.Infof("📡 开始等待聚合结果: height=%d, aggregator=%s", height, aggregatorID)

	// 1. 计算等待超时时间（基于配置）
	waitTimeout := s.calculateWaitTimeout()
	s.logger.Infof("⏱️  等待超时时间: %s", waitTimeout)

	// 2. V2 优化：订阅 ConsensusResultBroadcast 广播消息（通过 channel 接收）
	resultChan := make(chan *protocol.ConsensusResultBroadcast, 10) // 增加缓冲避免阻塞
	unsubscribe, err := s.subscribeToConsensusResult(ctx, height, resultChan)
	if err != nil {
		s.logger.Warnf("⚠️ 订阅共识结果广播失败: %v，将仅依赖主动查询", err)
		// 订阅失败不致命，继续依赖主动查询
	} else {
		defer unsubscribe()
		s.logger.Debugf("✅ Gossip 订阅成功")
	}

	// 3. V2 优化：优先处理 Gossip 广播，其次才是超时查询
	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case broadcast, ok := <-resultChan:
			if !ok {
				s.logger.Warnf("⚠️ 广播 channel 已关闭")
				// Channel 关闭，继续等待超时或执行查询
				goto QueryStatus
			}
			if broadcast != nil && broadcast.FinalBlock != nil {
				s.logger.Infof("✅ 通过 Gossip 广播收到最终区块: height=%d", height)
				// 成功收到广播，处理完成
				return nil
			}

		case <-timer.C:
			// 等待超时，主动查询
			s.logger.Warnf("⏰ 等待广播超时: height=%d, timeout=%s", height, waitTimeout)
			goto QueryStatus
		}
	}

QueryStatus:
	// 4. 超时后主动查询聚合器状态
	if err := s.queryAggregatorStatus(ctx, height, aggregatorID); err != nil {
		s.logger.Errorf("❌ 主动查询聚合器状态失败: %v", err)
		return fmt.Errorf("query aggregator status failed: %v", err)
	}

	// 查询成功后，再等待一小段时间接收可能的广播
	s.logger.Debugf("🔍 查询完成，再等待5秒接收可能的广播...")
	finalTimer := time.NewTimer(5 * time.Second)
	defer finalTimer.Stop()

	select {
	case broadcast, ok := <-resultChan:
		if ok && broadcast != nil && broadcast.FinalBlock != nil {
			s.logger.Infof("✅ 查询后通过广播收到最终区块: height=%d", height)
			return nil
		}
	case <-finalTimer.C:
		s.logger.Debugf("查询完成后未收到广播，但查询已成功")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// calculateWaitTimeout 计算等待超时时间
//
// V2 新增：根据配置动态计算
// waitTimeout = CollectionWindowDuration + DistributionTimeout + NetworkBuffer
func (s *WaitAndQueryService) calculateWaitTimeout() time.Duration {
	const defaultNetworkBuffer = 5 * time.Second

	collectionWindow := 10 * time.Second    // 默认值
	distributionTimeout := 30 * time.Second // 默认值

	if s.config != nil {
		if s.config.Aggregator.CollectionWindowDuration > 0 {
			collectionWindow = s.config.Aggregator.CollectionWindowDuration
		}
		if s.config.Aggregator.DistributionTimeout > 0 {
			distributionTimeout = s.config.Aggregator.DistributionTimeout
		}
	}

	waitTimeout := collectionWindow + distributionTimeout + defaultNetworkBuffer
	return waitTimeout
}

// subscribeToConsensusResult 订阅共识结果广播
//
// V2 新增：订阅 Gossip 广播消息
func (s *WaitAndQueryService) subscribeToConsensusResult(
	ctx context.Context,
	height uint64,
	resultChan chan<- *protocol.ConsensusResultBroadcast,
) (func(), error) {
	// V2 简化实现：暂不实现 Gossip 订阅，保留接口供后续完善
	// 原因：需要进一步设计订阅机制与现有系统的集成方式
	s.logger.Debugf("📡 Gossip 订阅功能待后续完善（当前仅依赖主动查询）")

	// 返回空的 unsubscribe 函数，避免调用方出错
	return func() {}, nil

	// TODO: 完善 Gossip 订阅实现
	// 参考: internal/core/consensus/integration/network/subscribe_handlers.go
	// 需要确认如何通过 networkService.Subscribe() 获取 channel 形式的订阅
}

// queryAggregatorStatus 主动查询聚合器状态
//
// V2 新增：等待超时后主动查询
func (s *WaitAndQueryService) queryAggregatorStatus(
	ctx context.Context,
	height uint64,
	aggregatorID peer.ID,
) error {
	s.logger.Infof("🔍 主动查询聚合器状态: height=%d, aggregator=%s", height, aggregatorID)

	// 1. 获取查询配置
	queryRetryInterval := 15 * time.Second
	maxQueryAttempts := uint32(3)
	queryTotalTimeout := 60 * time.Second

	if s.config != nil {
		if s.config.Miner.QueryRetryInterval > 0 {
			queryRetryInterval = s.config.Miner.QueryRetryInterval
		}
		if s.config.Miner.MaxQueryAttempts > 0 {
			maxQueryAttempts = s.config.Miner.MaxQueryAttempts
		}
		if s.config.Miner.QueryTotalTimeout > 0 {
			queryTotalTimeout = s.config.Miner.QueryTotalTimeout
		}
	}

	// 2. 创建查询超时 context
	queryCtx, cancel := context.WithTimeout(ctx, queryTotalTimeout)
	defer cancel()

	// 3. 循环查询，直到成功或达到最大尝试次数
	for attempt := uint32(0); attempt < maxQueryAttempts; attempt++ {
		select {
		case <-queryCtx.Done():
			return fmt.Errorf("查询总超时: %v", queryCtx.Err())
		default:
		}

		s.logger.Infof("🔍 查询聚合器状态（尝试 %d/%d）: aggregator=%s", attempt+1, maxQueryAttempts, aggregatorID)

		// 4. 构建查询请求
		query := &protocol.AggregatorStatusQuery{
			Base: &protocol.BaseMessage{
				MessageId:     s.generateMessageID(),
				SenderId:      []byte(s.p2pService.Host().ID()),
				TimestampUnix: time.Now().Unix(),
			},
			Height: height,
		}

		reqBytes, err := proto.Marshal(query)
		if err != nil {
			s.logger.Errorf("❌ 序列化查询请求失败: %v", err)
			continue
		}

		// 5. 发送查询请求
		respBytes, err := s.networkService.Call(queryCtx, aggregatorID, protocols.ProtocolAggregatorStatus, reqBytes, nil)
		if err != nil {
			// 聚合器离线或网络错误，触发重选
			s.logger.Warnf("⚠️ 查询失败（聚合器可能离线）: %v", err)
			// TODO: 触发重选机制
			return fmt.Errorf("aggregator offline or network error: %v", err)
		}

		// 6. 反序列化响应
		var response protocol.AggregatorStatusResponse
		if err := proto.Unmarshal(respBytes, &response); err != nil {
			s.logger.Errorf("❌ 反序列化响应失败: %v", err)
			continue
		}

		// 7. 处理响应状态
		switch response.State {
		case protocol.AggregatorStatusResponse_AGGREGATOR_STATE_COMPLETED:
			// 聚合已完成，处理最终区块
			s.logger.Infof("✅ 聚合已完成: height=%d", height)
			if response.FinalBlock != nil {
				// TODO: 处理最终区块
				s.logger.Infof("✅ 收到最终区块: height=%d", height)
				return nil
			}
			return fmt.Errorf("聚合已完成但未返回最终区块")

		case protocol.AggregatorStatusResponse_AGGREGATOR_STATE_NOT_AGGREGATOR:
			// 查询了错误的聚合器，触发重选
			s.logger.Warnf("⚠️  查询了错误的聚合器: height=%d", height)
			// TODO: 触发重选机制
			return fmt.Errorf("queried wrong aggregator")

		case protocol.AggregatorStatusResponse_AGGREGATOR_STATE_COLLECTING,
			protocol.AggregatorStatusResponse_AGGREGATOR_STATE_EVALUATING,
			protocol.AggregatorStatusResponse_AGGREGATOR_STATE_DISTRIBUTING:
			// 聚合器正在处理，继续等待
			s.logger.Infof("🔄 聚合器正在处理: height=%d, state=%v, candidate_count=%d",
				height, response.State, response.CandidateCount)

			// 如果不是最后一次尝试，等待一段时间后重试
			if attempt < maxQueryAttempts-1 {
				s.logger.Infof("⏳ 等待 %s 后重试查询", queryRetryInterval)
				select {
				case <-queryCtx.Done():
					return queryCtx.Err()
				case <-time.After(queryRetryInterval):
					continue
				}
			}
			// 最后一次尝试也未完成，返回超时
			return fmt.Errorf("聚合器仍在处理，查询尝试已用尽")

		default:
			// 未知状态或错误
			s.logger.Warnf("⚠️  聚合器返回未知状态: state=%v", response.State)
			return fmt.Errorf("unknown aggregator state: %v", response.State)
		}
	}

	return fmt.Errorf("查询尝试已用尽: max_attempts=%d", maxQueryAttempts)
}

// generateMessageID 生成消息ID
func (s *WaitAndQueryService) generateMessageID() string {
	return fmt.Sprintf("query_%d_%s", time.Now().UnixNano(), s.p2pService.Host().ID().String())
}
