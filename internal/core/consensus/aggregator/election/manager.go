// Package election 实现聚合节点选举服务
//
// 🎯 **聚合节点选举服务模块**
//
// 本包实现 AggregatorElection 接口，提供确定性聚合节点选举功能：
// - 基于Hash(height || SEED) + KademliaClosestPeer算法
// - 判断当前节点是否为指定高度的聚合节点
// - 支持每高度重新选举机制
// - 实现内容寻址路由的核心逻辑
package election

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// AggregatorElectionService 聚合节点选举服务实现（薄委托层）
type AggregatorElectionService struct {
	logger     log.Logger            // 日志记录器
	calculator *aggregatorCalculator // 选举计算器
	validator  *electionValidator    // 选举验证器
}

// NewAggregatorElectionService 创建聚合节点选举服务实例
func NewAggregatorElectionService(
	logger log.Logger,
	chainQuery persistence.ChainQuery,
	hashManager crypto.HashManager,
	kbucket kademlia.DistanceCalculator,
	p2pService p2pi.Service,
	networkService netiface.Network,
	routingTableManager kademlia.RoutingTableManager,
) interfaces.AggregatorElection {
	// 创建计算器和验证器（包含协议过滤能力）
	calculator := newAggregatorCalculator(chainQuery, hashManager, kbucket, p2pService, networkService, routingTableManager, logger)
	validator := newElectionValidator(calculator)

	return &AggregatorElectionService{
		logger:     logger,
		calculator: calculator,
		validator:  validator,
	}
}

// 编译时确保 AggregatorElectionService 实现了 AggregatorElection 接口
var _ interfaces.AggregatorElection = (*AggregatorElectionService)(nil)

// IsAggregatorForHeight 判断当前节点是否为指定高度的聚合节点
func (s *AggregatorElectionService) IsAggregatorForHeight(height uint64) (bool, error) {
	s.logger.Info("判断是否为指定高度的聚合节点")

	// ✅ 生产级修复：禁止使用 context.Background() 进行网络相关选举判断
	// 选举内部会做协议探测（DialPeer/GetProtocols），若无超时将导致聚合流程卡死（你日志里的“卡在534”即为此类现象）。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.calculator.isAggregatorForHeight(ctx, height)
}

// GetAggregatorForHeight 获取指定高度的聚合节点ID
func (s *AggregatorElectionService) GetAggregatorForHeight(height uint64) (peer.ID, error) {
	s.logger.Info("获取指定高度的聚合节点ID")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.calculator.getAggregatorForHeight(ctx, height)
}

// GetAggregatorForHeightWithWaivers 获取指定高度的聚合节点ID（排除弃权节点）
//
// V2 新增：支持弃权与重选机制
func (s *AggregatorElectionService) GetAggregatorForHeightWithWaivers(height uint64, waivedAggregators []peer.ID) (peer.ID, error) {
	s.logger.Infof("获取指定高度的聚合节点ID（排除弃权节点），高度=%d，弃权节点数=%d", height, len(waivedAggregators))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.calculator.getAggregatorForHeightWithWaivers(ctx, height, waivedAggregators)
}

// ValidateAggregatorEligibility 验证聚合节点资格
func (s *AggregatorElectionService) ValidateAggregatorEligibility(peerID peer.ID) (bool, error) {
	s.logger.Info("验证聚合节点资格")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.validator.validateNodeEligibility(ctx, peerID)
}
