// height_query.go - 网络高度查询逻辑
// 负责查询网络中其他节点的区块链高度
package sync

import (
	"context"
	"fmt"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                           网络高度查询实现
// ============================================================================

// queryNetworkHeightFromCandidates 从指定的候选节点列表查询网络高度
//
// 🎯 **候选节点查询策略**：
// 1. 使用上游已筛选的候选节点列表，避免重复选择
// 2. 对候选节点进行WES协议过滤，跳过非WES引导节点
// 3. 依次查询这些节点的高度信息
// 4. 返回查询成功的第一个节点的高度和节点信息
func queryNetworkHeightFromCandidates(
	ctx context.Context,
	candidatePeers []peer.ID,
	networkService network.Network,
	host node.Host,
	localChainInfo *types.ChainInfo,
	configProvider config.Provider,
	logger log.Logger,
) (uint64, peer.ID, error) {
	if logger != nil {
		logger.Debug("🔍 开始网络高度查询（使用上游候选节点）")
	}

	// 阶段1: 过滤非WES节点（跳过公共引导节点）
	var weisynPeers []peer.ID
	for _, peerID := range candidatePeers {
		// 验证是否为WES协议节点
		if isValid, err := host.ValidateWESPeer(ctx, peerID); err != nil {
			if logger != nil {
				logger.Debugf("⚠️ WES节点验证失败，跳过节点: %s, 错误: %v", peerID.String()[:12]+"...", err)
			}
			continue
		} else if !isValid {
			if logger != nil {
				logger.Debugf("🚫 跳过非WES节点: %s", peerID.String()[:12]+"...")
			}
			continue
		}
		weisynPeers = append(weisynPeers, peerID)
	}

	if len(weisynPeers) == 0 {
		if logger != nil {
			logger.Debug("📊 过滤后无可用WES节点，跳过网络高度查询")
		}
		return 0, "", fmt.Errorf("过滤后无可用的WES节点")
	}

	if logger != nil {
		logger.Debugf("📊 过滤后可用WES节点: %d/%d 个", len(weisynPeers), len(candidatePeers))
	}

	closestPeers := weisynPeers

	// 阶段2: 优先级查询节点高度
	// 🎯 **优化策略**: 优先使用第一个节点作为高度源，确保数据包节点优先级
	var bestHeight uint64
	var bestPeer peer.ID
	var firstSuccess bool = false

	if logger != nil {
		logger.Infof("🔄 开始网络高度查询协议调用，候选节点: %d个", len(closestPeers))
	}

	for i, peerID := range closestPeers {
		if logger != nil {
			priority := "高优先级"
			if i > 0 {
				priority = "备用"
			}
			logger.Debugf("📡 查询%s节点 %d/%d: %s", priority, i+1, len(closestPeers), peerID.String())
		}

		if logger != nil {
			logger.Debugf("📞 调用高度查询协议，目标节点: %s", peerID.String()[:12]+"...")
		}

		height, err := queryPeerHeight(ctx, networkService, host, peerID, configProvider, logger)
		if err != nil {
			if logger != nil {
				logger.Warnf("⚠️ 高度查询协议调用失败，节点: %s, 错误: %v", peerID.String()[:12]+"...", err)
			}
			continue // 尝试下一个节点
		}

		if logger != nil {
			logger.Debugf("✅ 高度查询协议调用成功，节点: %s, 高度: %d", peerID.String()[:12]+"...", height)
		}

		if logger != nil {
			logger.Infof("✅ 成功获取节点高度: %d (来源节点: %s)", height, peerID.String())
		}

		// 优先使用第一个成功的节点（最高优先级）
		if !firstSuccess {
			bestHeight = height
			bestPeer = peerID
			firstSuccess = true

			if logger != nil {
				logger.Infof("🎯 选择第一个可用节点作为高度源: %d (节点: %s)", height, peerID.String())
			}

			// 如果是第一个节点就成功了，直接返回（最高优先级）
			if i == 0 {
				return height, peerID, nil
			}
		}

		// 如果后续节点高度更高，仅作记录但不替换（保持第一个节点的优先级）
		if height > bestHeight && logger != nil {
			logger.Debugf("📊 发现更高节点高度: %d vs %d，但保持第一个节点优先级", height, bestHeight)
		}
	}

	// 返回第一个成功的节点结果
	if firstSuccess {
		if logger != nil {
			logger.Infof("✅ 最终选择网络高度: %d (优先级节点: %s)", bestHeight, bestPeer.String())
		}
		return bestHeight, bestPeer, nil
	}

	// 所有节点都查询失败
	return 0, "", fmt.Errorf("所有K桶节点的高度查询都失败了")
}

// queryPeerHeight 查询指定节点的区块链高度
//
// 🎯 **使用标准KBucketSync协议进行高度查询**：
// 1. 构建KBucketSyncRequest（仅用于高度查询）
// 2. 使用ProtocolKBucketSync协议通信
// 3. 从IntelligentPaginationResponse中提取高度信息
// 4. 统一使用protobuf序列化，避免JSON依赖
func queryPeerHeight(
	ctx context.Context,
	networkService network.Network,
	host node.Host,
	peerID peer.ID,
	configProvider config.Provider,
	logger log.Logger,
) (uint64, error) {
	if logger != nil {
		logger.Debugf("📡 向节点 %s 查询区块链高度（使用KBucketSync协议）", peerID.String())
	}

	// 🎯 **智能高度查询响应大小计算**
	// 高度查询只需要基本的响应头信息，不需要区块数据，因此使用很小的大小限制
	var maxResponseSize uint32 = 1024 // 默认1KB：足够响应头和高度信息
	blockchainConfig := configProvider.GetBlockchain()
	if blockchainConfig != nil {
		// 基于通用配置智能计算高度查询响应大小
		if blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
			// 高度查询响应大小 = 通用响应大小 / 1000（极小比例）
			generalSize := blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
			maxResponseSize = generalSize / 1000

			// 确保在合理范围内：最小512字节，最大4KB
			if maxResponseSize < 512 {
				maxResponseSize = 512
			} else if maxResponseSize > 4096 {
				maxResponseSize = 4096
			}
		} else if blockchainConfig.Sync.Advanced.IntelligentPagingThreshold > 0 {
			// 备选：基于智能分页阈值计算
			maxResponseSize = blockchainConfig.Sync.Advanced.IntelligentPagingThreshold / 1000
			if maxResponseSize < 1024 {
				maxResponseSize = 1024
			}
		}
	}

	if logger != nil {
		logger.Debugf("📊 高度查询响应大小限制: %d 字节", maxResponseSize)
	}

	// 构建KBucketSyncRequest（专用于高度查询）
	request := &protocol.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("height-query-%d", time.Now().UnixNano()),
		LocalHeight:     0,                          // 高度查询时设为0
		RoutingKey:      []byte("height-query"),     // 高度查询路由键
		MaxResponseSize: maxResponseSize,            // 从配置获取，仅需要响应头信息
		RequesterPeerId: []byte(host.ID().String()), // 本地节点ID（请求者）
		TargetHeight:    nil,                        // 不指定目标高度，获取对端当前高度
	}

	// 序列化请求
	requestData, err := proto.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf("序列化高度查询请求失败: %w", err)
	}

	// 配置传输选项（从配置获取，高度查询使用较短超时）
	var connectTimeout = 10 * time.Second
	var writeTimeout = 5 * time.Second
	var readTimeout = 10 * time.Second
	var maxRetries = 2
	var retryDelay = 1 * time.Second

	if blockchainConfig != nil {
		if blockchainConfig.Sync.Advanced.ConnectTimeout > 0 {
			connectTimeout = blockchainConfig.Sync.Advanced.ConnectTimeout / 2 // 高度查询用一半时间
		}
		if blockchainConfig.Sync.Advanced.WriteTimeout > 0 {
			writeTimeout = blockchainConfig.Sync.Advanced.WriteTimeout / 2
		}
		if blockchainConfig.Sync.Advanced.ReadTimeout > 0 {
			readTimeout = blockchainConfig.Sync.Advanced.ReadTimeout / 3 // 高度查询读取很快
		}
		if blockchainConfig.Sync.Advanced.MaxRetryAttempts > 0 {
			maxRetries = blockchainConfig.Sync.Advanced.MaxRetryAttempts
		}
		if blockchainConfig.Sync.Advanced.RetryDelay > 0 {
			retryDelay = blockchainConfig.Sync.Advanced.RetryDelay
		}
	}

	transportOpts := &types.TransportOptions{
		ConnectTimeout: connectTimeout,
		WriteTimeout:   writeTimeout,
		ReadTimeout:    readTimeout,
		MaxRetries:     maxRetries,
		RetryDelay:     retryDelay,
		BackoffFactor:  1.5,
	}

	// 发送网络请求（使用标准KBucketSync协议）
	responseData, err := networkService.Call(ctx, peerID, protocols.ProtocolKBucketSync, requestData, transportOpts)
	if err != nil {
		return 0, fmt.Errorf("KBucket高度查询调用失败: %w", err)
	}

	// 解析IntelligentPaginationResponse
	response := &protocol.IntelligentPaginationResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return 0, fmt.Errorf("解析高度查询响应失败: %w", err)
	}

	// 验证响应
	if response.RequestId != request.RequestId {
		return 0, fmt.Errorf("响应RequestID不匹配: 期望=%s, 实际=%s",
			request.RequestId, response.RequestId)
	}

	if !response.Success {
		errorMsg := "未知错误"
		if response.ErrorMessage != nil {
			errorMsg = *response.ErrorMessage
		}
		return 0, fmt.Errorf("对端高度查询失败: %s", errorMsg)
	}

	// 从响应中提取高度信息
	// 对端会在NextHeight字段中返回其当前高度
	peerHeight := response.NextHeight

	if logger != nil {
		logger.Debugf("✅ 节点 %s 高度查询成功: %d（通过KBucketSync协议）",
			peerID.String(), peerHeight)
	}

	return peerHeight, nil
}
