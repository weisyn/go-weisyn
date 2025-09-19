// node_selection.go - K桶节点选择逻辑
// 负责使用Kademlia算法选择最优的同步节点
package sync

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                           K桶节点选择实现
// ============================================================================

// selectKBucketPeersForSync 为同步选择K桶节点
//
// 🎯 **智能节点选择策略**：
// 1. 使用本地最佳区块哈希作为路由键，选择拥有最新数据的节点
// 2. 通过K桶管理器查找距离最近的节点
// 3. 验证节点是否为WES节点，过滤掉非业务节点
// 4. 返回经过验证的优质节点列表
func selectKBucketPeersForSync(
	ctx context.Context,
	routingManager kademlia.RoutingTableManager,
	host node.Host,
	localChainInfo *types.ChainInfo,
	logger log.Logger,
) ([]peer.ID, error) {
	if logger != nil {
		logger.Debug("🔍 基于链状态选择K桶同步节点")
	}

	// 使用本地最佳区块哈希作为路由键
	// 这确保同步请求能够找到拥有最新数据的节点
	routingKey := localChainInfo.BestBlockHash
	if len(routingKey) == 0 {
		// 如果没有最佳区块哈希，使用链高度生成路由键
		routingKey = []byte(fmt.Sprintf("height-%d", localChainInfo.Height))
	}

	// 直接调用路由表管理器查找最近节点（使用简化接口）
	candidates := routingManager.FindClosestPeers(routingKey, 8) // 选择8个最近的节点

	if len(candidates) == 0 {
		return nil, fmt.Errorf("路由表中没有可用的节点")
	}

	// 验证WES节点
	var selectedPeers []peer.ID
	for _, peerID := range candidates {
		// 验证节点是否为WES节点
		isWES, err := host.ValidateWESPeer(ctx, peerID)
		if err != nil {
			if logger != nil {
				logger.Warnf("⚠️ 验证WES节点失败: %s, 错误: %v", peerID.String(), err)
			}
			continue
		}

		if !isWES {
			if logger != nil {
				logger.Debugf("跳过非WES节点: %s", peerID.String())
			}
			continue
		}

		selectedPeers = append(selectedPeers, peerID)
	}

	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("没有找到可用的WES同步节点")
	}

	if logger != nil {
		logger.Infof("✅ K桶节点选择完成: 候选=%d, 已验证=%d",
			len(candidates), len(selectedPeers))
	}

	return selectedPeers, nil
}
