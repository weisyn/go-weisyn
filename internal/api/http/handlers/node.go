package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	networkirace "github.com/weisyn/v1/pkg/interfaces/network"
)

// NodeHandlers 节点网络API处理器
// 提供节点信息查询、连接状态监控等基础功能
type NodeHandlers struct {
	host                nodeiface.Host               // 最小 节点宿主
	network             networkirace.Network         // 网络服务
	routingTableManager kademlia.RoutingTableManager // K桶路由表管理器，用于诊断
	configProvider      config.Provider              // 配置提供者，用于获取网络命名空间等信息
	logger              log.Logger                   // 日志记录器
}

// NewNodeHandlers 创建节点处理器实例
// 参数:
//   - host: 最小 节点宿主
//   - network: 网络服务
//   - routingTableManager: K桶路由表管理器
//   - configProvider: 配置提供者
//   - logger: 日志记录器
//
// 返回:
//   - 节点处理器实例
func NewNodeHandlers(host nodeiface.Host, network networkirace.Network, routingTableManager kademlia.RoutingTableManager, configProvider config.Provider, logger log.Logger) *NodeHandlers {
	return &NodeHandlers{
		host:                host,
		network:             network,
		routingTableManager: routingTableManager,
		configProvider:      configProvider,
		logger:              logger.With("component", "node-api"),
	}
}

// GetNodeInfo 获取本地节点信息
//
// 📌 **接口说明**：获取当前节点的基本标识信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/node/info`
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "node_id": "12D3KooW...",
//	  "addresses": [
//	    "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW...",
//	    "/ip6/::1/tcp/4001/p2p/12D3KooW..."
//	  ],
//	  "address_count": 2,
//	  "actual_listen_addrs": [...],
//	  "actual_listen_count": 3,
//	  "supported_protocols": ["kad-dht", "gossipsub"],
//	  "protocol_count": 2
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "error": "节点网络未启动",
//	  "details": "无法获取节点ID"
//	}
//
// 💡 **使用说明**：
// - 返回节点的完整网络标识信息
// - addresses: 对外公告地址（经过过滤）
// - actual_listen_addrs: 实际监听地址（包含libp2p自动添加）
func (h *NodeHandlers) GetNodeInfo(c *gin.Context) {
	h.logger.Debug("处理获取节点信息请求")

	// 获取节点ID
	nodeID := h.host.ID()
	if nodeID == "" {
		h.logger.Error("节点ID为空，节点网络服务可能未启动")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "节点网络未启动",
			"details": "无法获取节点ID",
		})
		return
	}

	// 获取监听地址 (公告地址)
	announceAddrs := h.host.AnnounceAddrs()
	var announceAddrStrings []string
	for _, addr := range announceAddrs {
		announceAddrStrings = append(announceAddrStrings, addr.String())
	}

	// 获取libp2p host的所有监听地址 (实际监听的地址)
	libp2pHost := h.host.Libp2pHost()
	var actualListenAddrs []string
	if libp2pHost != nil {
		listenAddrs := libp2pHost.Network().ListenAddresses()
		for _, addr := range listenAddrs {
			actualListenAddrs = append(actualListenAddrs, addr.String())
		}
	}

	// 🔧 获取节点支持的协议列表
	var supportedProtocols []string
	if libp2pHost != nil {
		protocols := libp2pHost.Mux().Protocols()
		for _, protocol := range protocols {
			supportedProtocols = append(supportedProtocols, string(protocol))
		}
	}

	// 获取版本信息
	buildInfo := version.GetBuildInfo()

	// 获取网络配置信息
	var networkNamespace, chainID, networkType string
	var chainIDNum uint64
	if h.configProvider != nil {
		networkNamespace = h.configProvider.GetNetworkNamespace()
		if blockchain := h.configProvider.GetBlockchain(); blockchain != nil {
			chainIDNum = blockchain.ChainID
			chainID = fmt.Sprintf("%d", chainIDNum)
			networkType = blockchain.NetworkType
		}
	}

	// 获取连接的节点数量
	connectedPeers := 0
	if libp2pHost != nil {
		connectedPeers = len(libp2pHost.Network().Peers())
	}

	h.logger.Debugf("返回增强节点信息: ID=%s, 网络命名空间=%s, 链ID=%s, 版本=%s, 连接节点数=%d",
		nodeID, networkNamespace, chainID, buildInfo.Version, connectedPeers)

	// 返回增强的节点信息
	c.JSON(http.StatusOK, gin.H{
		// 基础网络信息
		"success":             true,
		"node_id":             nodeID.String(),
		"addresses":           announceAddrStrings,
		"address_count":       len(announceAddrStrings),
		"actual_listen_addrs": actualListenAddrs,
		"actual_listen_count": len(actualListenAddrs),
		"supported_protocols": supportedProtocols,
		"protocol_count":      len(supportedProtocols),
		"connected_peers":     connectedPeers,

		// 🆕 网络隔离信息（重要：用于环境识别）
		"network_namespace": networkNamespace,
		"chain_id":          chainID,
		"chain_id_numeric":  chainIDNum,
		"network_type":      networkType,

		// 🆕 版本信息（重要：用于兼容性检查）
		"version":    buildInfo.Version,
		"build_time": buildInfo.BuildTime,
		"build_env":  buildInfo.BuildEnv,
		"go_version": buildInfo.GoVersion,
		"go_arch":    buildInfo.GoArch,
		"go_os":      buildInfo.GoOS,

		// 说明信息
		"note": "🔧 增强节点信息 - 包含网络隔离和版本信息，用于环境识别和兼容性检查",
	})
}

// GetNodeStatus 获取节点运行状态
// GET /api/v1/node/status
//
// 功能：返回节点的运行状态信息，用于健康检查和监控
// 响应：状态标识、节点ID、运行时间等
func (h *NodeHandlers) GetNodeStatus(c *gin.Context) {
	h.logger.Debug("处理获取节点状态请求")

	// 获取节点ID验证网络服务状态
	nodeID := h.host.ID()
	if nodeID == "" {
		h.logger.Warn("节点网络服务未就绪")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"status":  "unavailable",
			"error":   "节点网络服务未就绪",
		})
		return
	}

	// 获取地址信息
	addrs := h.host.AnnounceAddrs()

	h.logger.Debugf("节点状态正常: ID=%s", nodeID)

	// 返回状态信息
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"status":        "running",
		"node_id":       nodeID.String(),
		"address_count": len(addrs),
		"timestamp":     time.Now().Unix(),
	})
}

// GetPeers 获取连接的节点列表
//
// 📌 **接口说明**：获取当前连接的对等节点列表
//
// **HTTP Method**: `GET`
// **URL Path**: `/node/peers`
//
// **查询参数**：
//   - limit (number, optional): 返回数量限制，默认100
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "peers": [
//	    "12D3KooWAbc...",
//	    "12D3KooWDef...",
//	    "12D3KooWGhi..."
//	  ],
//	  "total_count": 15,
//	  "returned": 3
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "error": "网络服务内部错误"
//	}
//
// 💡 **使用说明**：
// - 返回当前活跃连接的P2P节点列表
// - 用于网络状态监控和连接性诊断
// - limit参数控制返回的节点数量
func (h *NodeHandlers) GetPeers(c *gin.Context) {
	h.logger.Debug("处理获取对等节点列表请求")

	// 解析查询参数
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	// 获取libp2p host来访问连接的节点
	libp2pHost := h.host.Libp2pHost()
	if libp2pHost == nil {
		h.logger.Error("无法获取libp2p主机实例")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务内部错误",
		})
		return
	}

	// 获取已连接的节点
	connectedPeers := libp2pHost.Network().Peers()
	totalCount := len(connectedPeers)

	// 应用限制
	if limit > 0 && limit < len(connectedPeers) {
		connectedPeers = connectedPeers[:limit]
	}

	// 转换为字符串格式
	var peerStrings []string
	for _, peerID := range connectedPeers {
		peerStrings = append(peerStrings, peerID.String())
	}

	h.logger.Debugf("返回对等节点列表: 总数=%d, 返回数=%d", totalCount, len(peerStrings))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"peers":       peerStrings,
		"total_count": totalCount,
		"returned":    len(peerStrings),
	})
}

// GetPeerByID 获取特定节点信息
// GET /api/v1/node/peers/:peer_id
//
// 功能：返回指定节点ID的详细连接信息
// 路径参数：
//   - peer_id: 目标节点的PeerID
//
// 响应：节点详细信息，包括连接状态、地址等
func (h *NodeHandlers) GetPeerByID(c *gin.Context) {
	peerIDStr := c.Param("peer_id")
	h.logger.Debugf("处理获取特定节点信息请求: %s", peerIDStr)

	if peerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "缺少节点ID参数",
		})
		return
	}

	// 解析节点ID
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		h.logger.Warnf("无效的节点ID格式: %s", peerIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的节点ID格式",
			"details": err.Error(),
		})
		return
	}

	// 获取libp2p host
	libp2pHost := h.host.Libp2pHost()
	if libp2pHost == nil {
		h.logger.Error("无法获取libp2p主机实例")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务内部错误",
		})
		return
	}

	// 检查连接状态
	network := libp2pHost.Network()
	connectedness := network.Connectedness(peerID)

	// 获取节点地址信息
	peerStore := libp2pHost.Peerstore()
	addrs := peerStore.Addrs(peerID)
	var addrStrings []string
	for _, addr := range addrs {
		addrStrings = append(addrStrings, addr.String())
	}

	h.logger.Debugf("节点信息: ID=%s, 连接状态=%s, 地址数=%d", peerID, connectedness, len(addrStrings))

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"peer_id":       peerID.String(),
		"connectedness": connectedness.String(),
		"addresses":     addrStrings,
		"address_count": len(addrStrings),
	})
}

// Connect 主动连接到指定节点
//
// 📌 **接口说明**：主动连接到指定的P2P节点
//
// **HTTP Method**: `POST`
// **URL Path**: `/node/connect`
//
// **请求体参数**：
//   - multiaddr (string, required): 目标节点的完整多地址
//
// **请求体示例**：
//
//	{
//	  "multiaddr": "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW..."
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "peer_id": "12D3KooW..."
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "error": "连接失败",
//	  "details": "连接超时"
//	}
//
// 💡 **使用说明**：
// - 用于主动建立P2P连接
// - multiaddr必须包含完整的节点信息（IP、端口、节点ID）
// - 连接成功后可进行数据传输和协议通信
func (h *NodeHandlers) Connect(c *gin.Context) {
	h.logger.Debug("处理主动连接请求")

	var req struct {
		Multiaddr string `json:"multiaddr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("连接请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "参数错误",
			"details": err.Error(),
		})
		return
	}

	// 解析多地址
	maddr, err := ma.NewMultiaddr(req.Multiaddr)
	if err != nil {
		h.logger.Warnf("无效的多地址格式: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的多地址格式",
			"details": err.Error(),
		})
		return
	}

	// 从多地址中提取节点信息
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		h.logger.Warnf("无法从多地址提取节点信息: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "多地址中缺少节点ID",
			"details": err.Error(),
		})
		return
	}

	// 获取libp2p host进行连接
	libp2pHost := h.host.Libp2pHost()
	if libp2pHost == nil {
		h.logger.Error("无法获取libp2p主机实例")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务内部错误",
		})
		return
	}

	// 尝试连接
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h.logger.Infof("尝试连接到节点: %s", info.ID)
	err = libp2pHost.Connect(ctx, *info)
	if err != nil {
		h.logger.Warnf("连接节点失败: %s, 错误: %v", info.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "连接失败",
			"details": err.Error(),
		})
		return
	}
	h.logger.Infof("主动连接成功 peer=%s", info.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "peer_id": info.ID.String()})
}

// GetTopicPeers 获取特定主题的连接节点
// GET /api/v1/node/topics/:topic/peers
//
// 功能：返回指定GossipSub主题的连接节点列表
// 路径参数：
//   - topic: 主题名称
//
// 响应：主题连接的节点列表和数量
func (h *NodeHandlers) GetTopicPeers(c *gin.Context) {
	topic := c.Param("topic")
	h.logger.Debugf("处理获取主题连接节点请求: %s", topic)

	if topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "缺少主题参数",
		})
		return
	}

	if h.network == nil {
		h.logger.Error("网络服务未初始化")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务未初始化",
		})
		return
	}

	// 获取主题连接的节点
	connectedPeers := h.network.GetTopicPeers(topic)
	var peerStrings []string
	for _, peerID := range connectedPeers {
		peerStrings = append(peerStrings, peerID.String())
	}

	h.logger.Debugf("主题 %s 连接的节点数量: %d", topic, len(peerStrings))

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"topic":      topic,
		"peers":      peerStrings,
		"peer_count": len(peerStrings),
	})
}

// ForceConnectToPeer 强制连接到指定节点并等待GossipSub mesh建立
// POST /api/v1/node/force-connect
//
// 功能：强制连接到指定节点，并等待GossipSub mesh连接建立
// 请求体：
//
//	{
//	  "multiaddr": "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW...",
//	  "topic": "weisyn.consensus.latest_block.v1",
//	  "wait_seconds": 30
//	}
//
// 响应：连接结果和mesh状态
func (h *NodeHandlers) ForceConnectToPeer(c *gin.Context) {
	h.logger.Debug("处理强制连接并建立mesh请求")

	var req struct {
		Multiaddr   string `json:"multiaddr" binding:"required"`
		Topic       string `json:"topic"`
		WaitSeconds int    `json:"wait_seconds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("强制连接请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "参数错误",
			"details": err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Topic == "" {
		req.Topic = "weisyn.consensus.latest_block.v1"
	}
	if req.WaitSeconds <= 0 {
		req.WaitSeconds = 30
	}

	// 解析多地址
	maddr, err := ma.NewMultiaddr(req.Multiaddr)
	if err != nil {
		h.logger.Warnf("无效的多地址格式: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的多地址格式",
			"details": err.Error(),
		})
		return
	}

	// 从多地址中提取节点信息
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		h.logger.Warnf("无法从多地址提取节点信息: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "多地址中缺少节点ID",
			"details": err.Error(),
		})
		return
	}

	// 获取libp2p host进行连接
	libp2pHost := h.host.Libp2pHost()
	if libp2pHost == nil {
		h.logger.Error("无法获取libp2p主机实例")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务内部错误",
		})
		return
	}

	// 第一步：建立libp2p连接
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h.logger.Infof("步骤1：尝试建立libp2p连接到节点: %s", info.ID)
	err = libp2pHost.Connect(ctx, *info)
	if err != nil {
		h.logger.Warnf("libp2p连接失败: %s, 错误: %v", info.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "libp2p连接失败",
			"details": err.Error(),
		})
		return
	}
	h.logger.Infof("✅ libp2p连接成功: %s", info.ID)

	// 第二步：主动触发GossipSub发现和mesh建立
	h.logger.Infof("步骤2：主动触发主题 %s 的GossipSub mesh建立，最多等待 %d 秒", req.Topic, req.WaitSeconds)

	// 主动触发GossipSub发现的策略
	meshEstablished := false
	checkInterval := time.Millisecond * 500 // 更频繁的检查，每500ms一次
	maxChecks := req.WaitSeconds * 2        // 总检查次数

	for i := 0; i < maxChecks; i++ {
		// 1. 检查当前mesh状态
		if h.network != nil {
			topicPeers := h.network.GetTopicPeers(req.Topic)
			for _, peerID := range topicPeers {
				if peerID == info.ID {
					meshEstablished = true
					break
				}
			}
		}

		if meshEstablished {
			h.logger.Infof("✅ GossipSub mesh建立成功，耗时: %.1f秒", float64(i)*0.5)
			break
		}

		// 2. 主动触发策略（每2秒执行一次）
		if i%4 == 0 && i > 0 { // 每2秒（4*500ms）执行一次
			h.logger.Debugf("🔄 主动触发GossipSub发现机制 (尝试 %d/%d)", i/4+1, req.WaitSeconds/2)

			// 策略A: 尝试重新连接（刷新连接状态）
			if err := libp2pHost.Connect(context.Background(), *info); err != nil {
				h.logger.Debugf("重新连接失败，继续等待: %v", err)
			}

			// 策略B: 检查是否需要重新初始化GossipSub（如果网络支持的话）
			// 这里可以添加其他主动触发机制
		}

		// 更详细的进度日志
		if i%10 == 0 { // 每5秒输出一次进度
			currentPeers := []string{}
			if h.network != nil {
				peers := h.network.GetTopicPeers(req.Topic)
				for _, peerID := range peers {
					currentPeers = append(currentPeers, peerID.String())
				}
			}
			h.logger.Debugf("等待mesh建立中... (%.1f/%d秒), 当前主题节点: %v", float64(i)*0.5, req.WaitSeconds, currentPeers)
		}

		time.Sleep(checkInterval)
	}

	// 最终状态检查
	finalTopicPeers := []string{}
	if h.network != nil {
		peers := h.network.GetTopicPeers(req.Topic)
		for _, peerID := range peers {
			finalTopicPeers = append(finalTopicPeers, peerID.String())
		}
	}

	response := gin.H{
		"success":          true,
		"peer_id":          info.ID.String(),
		"libp2p_connected": true,
		"mesh_established": meshEstablished,
		"topic":            req.Topic,
		"topic_peers":      finalTopicPeers,
		"topic_peer_count": len(finalTopicPeers),
		"wait_seconds":     req.WaitSeconds,
	}

	if meshEstablished {
		h.logger.Infof("✅ GossipSub mesh建立成功: peer=%s, topic=%s", info.ID, req.Topic)
	} else {
		h.logger.Warnf("⚠️ GossipSub mesh建立超时: peer=%s, topic=%s", info.ID, req.Topic)
		response["warning"] = "GossipSub mesh建立超时，但libp2p连接成功"
	}

	c.JSON(http.StatusOK, response)
}

// CheckTopicMesh 检查指定主题的mesh连接状态
// GET /api/v1/node/topics/:topic/mesh
//
// 功能：检查指定主题的GossipSub mesh连接状态
// 路径参数：
//   - topic: 主题名称
//
// 响应：mesh连接详细状态
func (h *NodeHandlers) CheckTopicMesh(c *gin.Context) {
	topic := c.Param("topic")
	h.logger.Debugf("处理检查主题mesh状态请求: %s", topic)

	if topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "缺少主题参数",
		})
		return
	}

	if h.network == nil {
		h.logger.Error("网络服务未初始化")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务未初始化",
		})
		return
	}

	// 获取主题连接的节点
	connectedPeers := h.network.GetTopicPeers(topic)
	var peerDetails []gin.H

	libp2pHost := h.host.Libp2pHost()
	if libp2pHost != nil {
		for _, peerID := range connectedPeers {
			connectedness := libp2pHost.Network().Connectedness(peerID)
			peerDetails = append(peerDetails, gin.H{
				"peer_id":       peerID.String(),
				"connectedness": connectedness.String(),
			})
		}
	}

	// 检查是否已订阅该主题
	isSubscribed := false
	if h.network != nil {
		isSubscribed = h.network.IsSubscribed(topic)
	}

	h.logger.Debugf("主题 %s mesh状态: 连接节点数=%d, 已订阅=%v", topic, len(connectedPeers), isSubscribed)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"topic":        topic,
		"subscribed":   isSubscribed,
		"peer_count":   len(connectedPeers),
		"peers":        peerDetails,
		"mesh_healthy": len(connectedPeers) > 0,
	})
}

// QuickConnect 快速连接测试（无等待）
// POST /api/v1/node/quick-connect
//
// 功能：快速测试连接到指定节点，立即返回结果不等待mesh
// 请求体：
//
//	{
//	  "multiaddr": "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW..."
//	}
//
// 响应：连接结果状态
func (h *NodeHandlers) QuickConnect(c *gin.Context) {
	h.logger.Debug("处理快速连接测试请求")

	var req struct {
		Multiaddr string `json:"multiaddr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("快速连接请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "参数错误",
			"details": err.Error(),
		})
		return
	}

	// 解析多地址
	maddr, err := ma.NewMultiaddr(req.Multiaddr)
	if err != nil {
		h.logger.Warnf("无效的多地址格式: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的多地址格式",
			"details": err.Error(),
		})
		return
	}

	// 从多地址中提取节点信息
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		h.logger.Warnf("无法从多地址提取节点信息: %s", req.Multiaddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "多地址中缺少节点ID",
			"details": err.Error(),
		})
		return
	}

	// 获取libp2p host进行连接
	libp2pHost := h.host.Libp2pHost()
	if libp2pHost == nil {
		h.logger.Error("无法获取libp2p主机实例")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "网络服务内部错误",
		})
		return
	}

	// 尝试连接（快速超时）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h.logger.Infof("快速连接测试到节点: %s", info.ID)
	err = libp2pHost.Connect(ctx, *info)

	connected := err == nil

	// 立即检查当前状态（不等待）
	var currentTopicPeers []string
	if h.network != nil {
		peers := h.network.GetTopicPeers("weisyn.consensus.latest_block.v1")
		for _, peerID := range peers {
			currentTopicPeers = append(currentTopicPeers, peerID.String())
		}
	}

	response := gin.H{
		"success":          true,
		"peer_id":          info.ID.String(),
		"libp2p_connected": connected,
		"topic_peers":      currentTopicPeers,
		"topic_peer_count": len(currentTopicPeers),
		"note":             "快速连接测试，不等待GossipSub mesh建立",
	}

	if !connected {
		response["connect_error"] = err.Error()
		h.logger.Warnf("快速连接失败: %s, 错误: %v", info.ID, err)
	} else {
		h.logger.Infof("✅ 快速连接成功: %s", info.ID)
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes 注册节点API路由
func (h *NodeHandlers) RegisterRoutes(router *gin.RouterGroup) {
	h.logger.Info("注册节点网络API路由")

	// 节点信息路由
	router.GET("/info", h.GetNodeInfo)
	router.GET("/status", h.GetNodeStatus)

	// 节点列表和详情路由
	router.GET("/peers", h.GetPeers)
	router.GET("/peers/:peer_id", h.GetPeerByID)

	// 主动连接
	router.POST("/connect", h.Connect)
	router.POST("/quick-connect", h.QuickConnect)

	// GossipSub主题相关路由
	router.GET("/topics/:topic/peers", h.GetTopicPeers)
	router.GET("/topics/:topic/mesh", h.CheckTopicMesh)
	router.POST("/force-connect", h.ForceConnectToPeer)

	// K桶路由表诊断端点
	router.GET("/routing/kbucket", h.GetKBucketStatus)
	router.GET("/routing/diagnostics", h.GetRoutingDiagnostics)

	h.logger.Info("节点网络API路由注册完成")
	h.logger.Infof("注册的API端点数量: 11")
	h.logger.Infof("- GET /info - 获取本地节点信息")
	h.logger.Infof("- GET /status - 获取节点运行状态")
	h.logger.Infof("- GET /peers - 获取连接的节点列表")
	h.logger.Infof("- GET /peers/:peer_id - 获取特定节点信息")
	h.logger.Infof("- POST /connect - 主动连接指定multiaddr")
	h.logger.Infof("- POST /quick-connect - 快速连接测试（无等待）")
	h.logger.Infof("- GET /topics/:topic/peers - 获取主题连接的节点")
	h.logger.Infof("- GET /topics/:topic/mesh - 检查主题mesh状态")
	h.logger.Infof("- POST /force-connect - 强制连接并建立mesh")
	h.logger.Infof("- GET /routing/kbucket - 获取K桶路由表状态")
	h.logger.Infof("- GET /routing/diagnostics - 获取路由表诊断信息")
}

// GetKBucketStatus 获取K桶路由表状态
//
// 📌 **接口说明**：获取K桶路由表的当前状态信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/node/routing/kbucket`
//
// ✅ **成功响应示例**：
//
//	{
//	  "status": "success",
//	  "data": {
//	    "total_peers": 5,
//	    "total_buckets": 3,
//	    "bucket_size": 20,
//	    "local_id": "12D3KooW...",
//	    "updated_at": "2025-09-11T17:40:15.468+08:00",
//	    "buckets": [
//	      {
//	        "index": 0,
//	        "peer_count": 2,
//	        "peers": ["12D3KooWABC...", "12D3KooWDEF..."]
//	      }
//	    ]
//	  }
//	}
func (h *NodeHandlers) GetKBucketStatus(c *gin.Context) {
	h.logger.Debug("处理K桶状态查询请求")

	if h.routingTableManager == nil {
		h.logger.Warn("K桶路由表管理器不可用")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "K桶路由表管理器不可用",
		})
		return
	}

	// 获取路由表快照
	routingTable := h.routingTableManager.GetRoutingTable()
	if routingTable == nil {
		h.logger.Warn("无法获取路由表快照")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "无法获取路由表快照",
		})
		return
	}

	// 构建响应数据
	bucketData := make([]gin.H, 0, len(routingTable.Buckets))
	for _, bucket := range routingTable.Buckets {
		peerIDs := make([]string, 0, len(bucket.Peers))
		for _, peer := range bucket.Peers {
			peerIDs = append(peerIDs, peer.ID)
		}

		bucketData = append(bucketData, gin.H{
			"index":      bucket.Index,
			"peer_count": len(bucket.Peers),
			"peers":      peerIDs,
		})
	}

	response := gin.H{
		"status": "success",
		"data": gin.H{
			"total_peers":   routingTable.TableSize,
			"total_buckets": len(routingTable.Buckets),
			"bucket_size":   routingTable.BucketSize,
			"local_id":      routingTable.LocalID,
			"updated_at":    routingTable.UpdatedAt,
			"buckets":       bucketData,
		},
	}

	h.logger.Debugf("K桶状态查询成功: %d个peer, %d个桶", routingTable.TableSize, len(routingTable.Buckets))
	c.JSON(http.StatusOK, response)
}

// GetRoutingDiagnostics 获取路由表诊断信息
//
// 📌 **接口说明**：获取路由表的详细诊断信息，包括连接状态对比
//
// **HTTP Method**: `GET`
// **URL Path**: `/node/routing/diagnostics`
//
// ✅ **成功响应示例**：
//
//	{
//	  "status": "success",
//	  "data": {
//	    "routing_table": {
//	      "total_peers": 5,
//	      "healthy_peers": 4
//	    },
//	    "connected_peers": {
//	      "total": 6,
//	      "list": ["12D3KooWABC...", "12D3KooWDEF..."]
//	    },
//	    "diagnostics": {
//	      "in_kbucket_but_not_connected": [],
//	      "connected_but_not_in_kbucket": ["12D3KooWXYZ..."],
//	      "kbucket_sync_ratio": 0.83
//	    }
//	  }
//	}
func (h *NodeHandlers) GetRoutingDiagnostics(c *gin.Context) {
	h.logger.Debug("处理路由表诊断请求")

	// 检查依赖服务
	if h.routingTableManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "K桶路由表管理器不可用",
		})
		return
	}

	if h.host == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "节点Host不可用",
		})
		return
	}

	// 获取K桶路由表信息
	routingTable := h.routingTableManager.GetRoutingTable()
	totalPeers := 0
	healthyPeers := 0
	if routingTable != nil {
		totalPeers = routingTable.TableSize

		// 检查K桶中每个peer的实际连接状态
		libp2pHost := h.host.Libp2pHost()
		if libp2pHost != nil {
			network := libp2pHost.Network()
			for _, bucket := range routingTable.Buckets {
				for _, peerInfo := range bucket.Peers {
					// 解析peer ID
					if peerID, err := peer.Decode(peerInfo.ID); err == nil {
						// 检查连接状态
						if network.Connectedness(peerID) == libnetwork.Connected {
							healthyPeers++
						}
					}
				}
			}
		} else {
			// 如果无法获取libp2p host，则无法检查连接状态
			healthyPeers = 0
		}
	}

	// 获取已连接peers（通过节点Host接口）
	var connectedPeers []peer.ID
	if h.host != nil {
		// 获取底层libp2p host
		libp2pHost := h.host.Libp2pHost()
		if libp2pHost != nil {
			connectedPeers = libp2pHost.Network().Peers()
		}
	}
	connectedPeerIDs := make([]string, 0, len(connectedPeers))
	var selfID peer.ID
	if h.host != nil {
		selfID = h.host.ID()
	}
	for _, peerID := range connectedPeers {
		if peerID != selfID { // 跳过自己
			connectedPeerIDs = append(connectedPeerIDs, peerID.String())
		}
	}

	// 构建K桶中的peer集合
	kbucketPeers := make(map[string]bool)
	if routingTable != nil {
		for _, bucket := range routingTable.Buckets {
			for _, peer := range bucket.Peers {
				kbucketPeers[peer.ID] = true
			}
		}
	}

	// 诊断分析
	var inKBucketButNotConnected []string
	var connectedButNotInKBucket []string

	// 检查K桶中但未连接的peers
	for peerID := range kbucketPeers {
		found := false
		for _, connectedPeerID := range connectedPeerIDs {
			if peerID == connectedPeerID {
				found = true
				break
			}
		}
		if !found {
			inKBucketButNotConnected = append(inKBucketButNotConnected, peerID)
		}
	}

	// 检查已连接但不在K桶的peers
	for _, connectedPeerID := range connectedPeerIDs {
		if !kbucketPeers[connectedPeerID] {
			connectedButNotInKBucket = append(connectedButNotInKBucket, connectedPeerID)
		}
	}

	// 计算同步比率
	var syncRatio float64
	if len(connectedPeerIDs) > 0 {
		syncedCount := len(connectedPeerIDs) - len(connectedButNotInKBucket)
		syncRatio = float64(syncedCount) / float64(len(connectedPeerIDs))
	}

	response := gin.H{
		"status": "success",
		"data": gin.H{
			"routing_table": gin.H{
				"total_peers":   totalPeers,
				"healthy_peers": healthyPeers,
			},
			"connected_peers": gin.H{
				"total": len(connectedPeerIDs),
				"list":  connectedPeerIDs,
			},
			"diagnostics": gin.H{
				"in_kbucket_but_not_connected": inKBucketButNotConnected,
				"connected_but_not_in_kbucket": connectedButNotInKBucket,
				"kbucket_sync_ratio":           syncRatio,
			},
		},
	}

	h.logger.Debugf("路由表诊断完成: K桶%d个peer, 已连接%d个peer, 同步率%.2f",
		totalPeers, len(connectedPeerIDs), syncRatio)
	c.JSON(http.StatusOK, response)
}
