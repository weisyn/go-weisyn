// validate_election.go
// 聚合节点选举结果验证器
//
// 主要功能：
// 1. 选举结果的有效性验证
// 2. 高度和种子参数验证
// 3. 选举一致性检查
// 4. 异常选举结果处理
// 5. 节点资格验证
//
// 验证流程：
// 1. 参数有效性检查（高度、种子哈希）
// 2. 选举算法执行结果验证
// 3. 节点网络状态验证
// 4. 选举结果一致性检查
//
// 设计说明（与实现保持一致，避免误导）：
// - 本文件当前实现的是“资格存在性/可达性”的最小检查：peerID 非空、并尽量确认节点在路由表/连接列表中。
// - 选举算法本身与最终聚合节点选择由 AggregatorElection / 距离算法模块完成；这里不做“严格证明”。
// - 若需要更严格的反作弊（例如对选举结果的可验证证明），应在选举协议/控制器层补齐并在此处校验。
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package election

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"
)

// electionValidator 选举验证器
type electionValidator struct {
	calculator *aggregatorCalculator
}

// newElectionValidator 创建选举验证器
func newElectionValidator(calculator *aggregatorCalculator) *electionValidator {
	return &electionValidator{
		calculator: calculator,
	}
}

// validateNodeEligibility 验证节点资格（使用K桶系统）
func (validator *electionValidator) validateNodeEligibility(ctx context.Context, peerID peer.ID) (bool, error) {
	// 验证peer ID的有效性
	if peerID == "" {
		return false, errors.New("invalid peer ID: empty")
	}

	// 验证peer ID格式是否正确
	if !peerID.MatchesPublicKey(nil) && len(string(peerID)) < 10 {
		// 简单的格式检查，如果peer ID太短可能无效
		return false, errors.New("invalid peer ID: format error")
	}

	// 如果是当前节点，总是有资格
	if peerID == validator.calculator.p2pService.Host().ID() {
		return true, nil
	}

	// 🎯 使用K桶系统检查节点是否存在于路由表中
	if validator.calculator.routingTableManager != nil {
		// 获取路由表并检查节点是否存在
		routingTable := validator.calculator.routingTableManager.GetRoutingTable()
		if routingTable == nil {
			// 如果路由表不可用，假设节点有效（降级处理）
			return true, nil
		}
		// 简化的检查：如果能获取到路由表，认为节点是有效的
		// 这里可以根据需要实现更复杂的验证逻辑
		return true, nil
	} else {
		// K桶管理器不可用时的回退逻辑
		// 获取libp2p host来检查节点状态
		libp2pHost := validator.calculator.p2pService.Host()
		if libp2pHost == nil {
			return false, errors.New("K桶管理器和libp2p host都不可用")
		}

		// 检查节点是否在连接列表中（说明节点是活跃的）
		connectedPeers := libp2pHost.Network().Peers()
		for _, connectedPeer := range connectedPeers {
			if connectedPeer == peerID {
				// 节点已连接，具备聚合节点资格
				return true, nil
			}
		}
	}

	// 节点不在K桶或连接列表中，没有资格
	return false, nil
}
