// Package network_handler 实现网络协议处理服务
//
// 🎯 **网络协议处理服务模块**
//
// 本包实现 NetworkProtocolHandler 接口，提供聚合器网络协议处理功能：
// - 实现UnifiedAggregatorRouter接口
// - 处理矿工区块提交协议
// - 处理共识心跳协议
// - 支持内容寻址转发机制
package network_handler

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"google.golang.org/protobuf/proto"
)

// NetworkProtocolHandlerService 网络协议处理服务实现（薄委托层）
type NetworkProtocolHandlerService struct {
	logger                    log.Logger                 // 日志记录器
	blockService              blockchain.BlockService    // 区块服务依赖（用于处理共识结果）
	blockSubmissionHandler    *blockSubmissionHandler    // 区块提交处理器
	consensusHeartbeatHandler *consensusHeartbeatHandler // 心跳处理器
}

// NewNetworkProtocolHandlerService 创建网络协议处理服务实例
func NewNetworkProtocolHandlerService(
	logger log.Logger,
	electionService interfaces.AggregatorElection,
	chainService blockchain.ChainService,
	candidatePool mempool.CandidatePool,
	host node.Host,
	netService netiface.Network,
	controller interfaces.AggregatorController,
	syncService blockchain.SystemSyncService,
	blockService blockchain.BlockService, // 添加区块服务参数
) interfaces.NetworkProtocolHandler {
	// 创建子处理器
	blockSubmissionHandler := newBlockSubmissionHandler(logger, electionService, chainService, candidatePool, host, netService, controller, syncService)
	consensusHeartbeatHandler := newConsensusHeartbeatHandler(logger, chainService, host)

	return &NetworkProtocolHandlerService{
		logger:                    logger,
		blockService:              blockService,
		blockSubmissionHandler:    blockSubmissionHandler,
		consensusHeartbeatHandler: consensusHeartbeatHandler,
	}
}

// 编译时确保 NetworkProtocolHandlerService 实现了 NetworkProtocolHandler 接口
var _ interfaces.NetworkProtocolHandler = (*NetworkProtocolHandlerService)(nil)

// HandleMinerBlockSubmission 处理矿工区块提交
func (s *NetworkProtocolHandlerService) HandleMinerBlockSubmission(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	s.logger.Info("处理矿工区块提交")
	return s.blockSubmissionHandler.handleMinerBlockSubmission(ctx, from, reqBytes)
}

// HandleConsensusHeartbeat 处理共识心跳协议
func (s *NetworkProtocolHandlerService) HandleConsensusHeartbeat(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	s.logger.Info("处理共识心跳协议")
	return s.consensusHeartbeatHandler.handleConsensusHeartbeat(ctx, from, reqBytes)
}

// HandleConsensusResultBroadcast 处理共识结果广播
func (s *NetworkProtocolHandlerService) HandleConsensusResultBroadcast(ctx context.Context, from peer.ID, topic string, data []byte) error {
	s.logger.Info("网络处理器处理共识结果广播")

	// 反序列化共识结果广播消息
	var broadcast protocol.ConsensusResultBroadcast
	if err := proto.Unmarshal(data, &broadcast); err != nil {
		// 🛡️ 增强错误恢复：记录详细错误信息但不中断聚合器处理
		s.logger.Errorf("❌ 共识结果广播反序列化失败 - from=%s, size=%d, error=%v", from.String(), len(data), err)
		s.logger.Warnf("🔄 跳过损坏的共识结果广播，继续处理其他消息")
		// 返回nil以避免中断聚合器的正常运行
		return nil
	}

	// 基础结构检查
	if broadcast.Base == nil || broadcast.FinalBlock == nil {
		return fmt.Errorf("invalid broadcast message: missing required fields")
	}

	finalBlock := broadcast.FinalBlock

	// 委托给区块服务进行验证
	valid, err := s.blockService.ValidateBlock(ctx, finalBlock)
	if err != nil {
		return fmt.Errorf("block validation failed: %v", err)
	}

	if !valid {
		return fmt.Errorf("received invalid consensus result block")
	}

	// 委托给区块服务进行处理
	if err := s.blockService.ProcessBlock(ctx, finalBlock); err != nil {
		return fmt.Errorf("block processing failed: %v", err)
	}

	s.logger.Info("网络处理器成功处理共识结果广播")
	return nil
}
