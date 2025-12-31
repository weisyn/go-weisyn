// height_query.go - 网络高度查询逻辑
// 负责查询网络中其他节点的区块链高度
package sync

import (
	"context"
	"fmt"
	"time"

	libnetwork "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
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
// 4. ✅ 选择“最高高度”的成功节点作为网络高度（避免被低高度节点误导）
func queryNetworkHeightFromCandidates(
	ctx context.Context,
	candidatePeers []peer.ID,
	networkService network.Network,
	p2pService p2pi.Service,
	localChainInfo *types.ChainInfo,
	configProvider config.Provider,
	logger log.Logger,
) (uint64, peer.ID, error) {
	if logger != nil {
		logger.Debug("🔍 开始网络高度查询（使用上游候选节点）")
	}

	// ✅ 重要：不要在这里用“connectedness/protocol cache”做硬过滤。
	// 原因：
	// - P2P 的 discovery→dial→identify→protocols 是渐进式的，启动早期/跨网场景协议缓存可能为空；
	// - 过早过滤会把真实业务节点“瞬态杀死”，导致阶段1.5直接失败，系统无法进入阶段2做 hello/fork 判定。
	//
	// 做法：直接对候选执行 queryPeerHeight（它内部有链身份校验与坏节点标记），失败就换下一个。
	closestPeers := candidatePeers

	// ✅ SYNC-201修复：收集所有成功查询的高度，使用中位数验证
	type heightResponse struct {
		peer   peer.ID
		height uint64
	}
	var responses []heightResponse

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

		height, err := queryPeerHeight(ctx, networkService, p2pService, peerID, configProvider, logger)
		if err != nil {
			// ✅ SYNC-003修复：记录高度查询失败原因（细化分类）
			recordSyncFailure(peerID, "height_query", ClassifyError(err), err.Error(), logger)
			if logger != nil {
				logger.Warnf("⚠️ 高度查询协议调用失败，节点: %s, 错误: %v", peerID.String()[:12]+"...", err)
			}
			// 该节点连续失败会触发"坏上游快速切换"，避免粘住不健康节点
			recordUpstreamFailure(peerID, logger)
			continue // 尝试下一个节点
		}

		// 查询成功：更新/刷新上游记忆并清零失败计数
		recordUpstreamSuccess(peerID)
		
		// 🔥 查询成功，重置节点健康度（清除熔断状态）
		ResetPeerHealth(peerID)

		if logger != nil {
			logger.Debugf("✅ 高度查询协议调用成功，节点: %s, 高度: %d", peerID.String()[:12]+"...", height)
		}

		responses = append(responses, heightResponse{
			peer:   peerID,
			height: height,
		})
	}

	if len(responses) == 0 {
		return 0, "", fmt.Errorf("所有候选节点的高度查询都失败了")
	}

	// ✅ SYNC-201修复：计算中位数高度（防止被单一恶意节点误导）
	heights := make([]uint64, len(responses))
	for i, r := range responses {
		heights[i] = r.height
		}

	// 简单排序用于中位数计算
	for i := 0; i < len(heights); i++ {
		for j := i + 1; j < len(heights); j++ {
			if heights[i] > heights[j] {
				heights[i], heights[j] = heights[j], heights[i]
			}
		}
	}
	
	medianHeight := heights[len(heights)/2]

	// ✅ 选择最接近中位数且高度最高的节点作为数据源
	var bestPeer peer.ID
	var bestHeight uint64
	for _, r := range responses {
		// 在中位数±10的范围内选择最高高度
		if r.height >= medianHeight && r.height <= medianHeight+10 {
			if r.height > bestHeight {
				bestHeight = r.height
				bestPeer = r.peer
			}
		}
	}

	if bestPeer == "" {
		// 如果没有在范围内的，直接使用中位数对应的节点
		for _, r := range responses {
			if r.height == medianHeight {
				bestPeer = r.peer
				bestHeight = medianHeight
				break
			}
		}
	}

		if logger != nil {
		logger.Infof("✅ 高度一致性检查: 查询=%d个节点, 中位数=%d, 最终选择=%d (节点: %s)", 
			len(responses), medianHeight, bestHeight, bestPeer.String()[:12]+"...")
		}

		return bestHeight, bestPeer, nil
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
	p2pService p2pi.Service,
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
		LocalHeight:     0,                                       // 高度查询时设为0
		RoutingKey:      []byte("height-query"),                  // 高度查询路由键
		MaxResponseSize: maxResponseSize,                         // 从配置获取，仅需要响应头信息
		RequesterPeerId: []byte(p2pService.Host().ID().String()), // 本地节点ID（请求者）
		TargetHeight:    nil,                                     // 不指定目标高度，获取对端当前高度
	}

	// ✅ v2 约束：请求尽量携带本地链身份（用于对端校验）；若本地链身份不可用则保持兼容（不 fail-fast）
	if localID, err := GetLocalChainIdentity(ctx, configProvider, nil); err == nil && localID.IsValid() {
		request.ChainIdentity = node.ToProtoChainIdentity(localID)
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
		return 0, fmt.Errorf("kBucket高度查询调用失败: %w", err)
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

	// ✅ v2：如果对端回传 ChainIdentity，则本地必须校验同链，并缓存到 peerstore 供 K桶准入复用
	if response.ChainIdentity != nil {
		if localID, err := GetLocalChainIdentity(ctx, configProvider, nil); err == nil && localID.IsValid() {
			remoteID := node.FromProtoChainIdentity(response.ChainIdentity)
			if !remoteID.IsValid() || !localID.IsSameChain(remoteID) {
				MarkBadPeer(peerID)
				recordUpstreamFailure(peerID, logger)
				return 0, fmt.Errorf("高度查询响应链身份不匹配: remote=%v local=%v", remoteID, localID)
			}
			// 同链：写入 peerstore 缓存（系统路径）
			cachePeerChainIdentity(p2pService, peerID, remoteID)
		}
	}

	// 从响应中提取高度信息
	// 对端会在NextHeight字段中返回其当前高度
	peerHeight := response.NextHeight
	
	// 🔥 查询成功，重置节点健康度
	ResetPeerHealth(peerID)

	if logger != nil {
		logger.Debugf("✅ 节点 %s 高度查询成功: %d（通过KBucketSync协议）",
			peerID.String(), peerHeight)
	}

	return peerHeight, nil
}

// validateWESPeer 验证节点是否为WES业务节点
// 基于协议能力检查实现简单的节点分类
func validateWESPeer(ctx context.Context, p2pService p2pi.Service, peerID peer.ID, configProvider config.Provider) (bool, error) {
	if p2pService == nil {
		return false, fmt.Errorf("p2p service not available")
	}

	host := p2pService.Host()
	if host == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	// 检查节点是否已连接
	if host.Network().Connectedness(peerID) != libnetwork.Connected {
		// 如果未连接，快速返回false，避免触发连接（保持轻量级）
		return false, nil
	}

	// 获取节点支持的协议
	peerProtocols, err := host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// ✅ 生产级 WES 节点识别（用于同步/高度查询）
	//
	// 说明：
	// - 高度查询走的是 KBucketSync（见 queryPeerHeight 使用 ProtocolKBucketSync）。
	// - 之前这里错误地用 ProtocolBlockSubmission 作为“WES 节点”判定条件，
	//   会导致“已入K桶的 weisyn 节点”在同步阶段1.5被再次过滤掉，从而出现：
	//   [TriggerSync] 网络高度查询失败: 过滤后无可用的WES节点
	//
	// 策略：只要对端支持任一 weisyn 的基础/同步协议即可认为是 WES 业务节点。
	candidates := []string{
		// 基础
		protocols.ProtocolNodeInfo,
		protocols.ProtocolHeartbeat,

		// 同步相关（高度查询/拉块依赖）
		protocols.ProtocolKBucketSync,
		protocols.ProtocolRangePaginated,
		protocols.ProtocolBlockSync,
		protocols.ProtocolHeaderSync,
		protocols.ProtocolStateSync,

		// 共识提交（可选）
		protocols.ProtocolBlockSubmission,
	}

	ns := ""
	if configProvider != nil {
		func() {
			defer func() { _ = recover() }()
			ns = configProvider.GetNetworkNamespace()
		}()
	}

	match := func(sp, base string) bool {
		if sp == base {
			return true
		}
		if ns != "" {
			return sp == protocols.QualifyProtocol(base, ns)
		}
		return false
	}

	for _, p := range peerProtocols {
		sp := string(p)
		for _, base := range candidates {
			if match(sp, base) {
				return true, nil
			}
		}
	}

	// 不支持WES核心协议，认为是外部节点
	return false, nil
}
