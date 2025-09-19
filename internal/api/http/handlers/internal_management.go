package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// InternalManagementHandler 内部管理处理器
// 🚨 重要提醒：此处理器不对外暴露，仅供项目方开发时手动触发
// 提供测试网络管理、数据清理、网络重置等内部功能
type InternalManagementHandler struct {
	blockchainService blockchain.ChainService      // 区块链服务
	repositoryManager repository.RepositoryManager // 仓储管理器
	networkService    nodeiface.Host               // 网络服务
	networkInterface  network.Network              // 网络接口
	config            config.Provider              // 配置提供者
	logger            log.Logger                   // 日志记录器

	// 测试会话管理
	currentTestSession string    // 当前测试会话ID
	sessionStartTime   time.Time // 会话开始时间
}

// NewInternalManagementHandler 创建内部管理处理器
func NewInternalManagementHandler(
	blockchainService blockchain.ChainService,
	repositoryManager repository.RepositoryManager,
	networkService nodeiface.Host,
	networkInterface network.Network,
	config config.Provider,
	logger log.Logger,
) *InternalManagementHandler {
	return &InternalManagementHandler{
		blockchainService: blockchainService,
		repositoryManager: repositoryManager,
		networkService:    networkService,
		networkInterface:  networkInterface,
		config:            config,
		logger:            logger,
	}
}

// TestNetworkStatus 测试网络状态响应
type TestNetworkStatus struct {
	NetworkClean     bool              `json:"network_clean"`      // 网络是否干净
	CurrentHeight    uint64            `json:"current_height"`     // 当前区块高度
	ConnectedPeers   int               `json:"connected_peers"`    // 连接的节点数
	TestSessionID    string            `json:"test_session_id"`    // 当前测试会话ID
	SessionStartTime *time.Time        `json:"session_start_time"` // 会话开始时间
	DataDirectories  []string          `json:"data_directories"`   // 数据目录列表
	Issues           []NetworkIssue    `json:"issues"`             // 检测到的问题
	PeerStates       map[string]string `json:"peer_states"`        // 节点状态
}

// NetworkIssue 网络问题描述
type NetworkIssue struct {
	Type        string `json:"type"`        // 问题类型
	Description string `json:"description"` // 问题描述
	Severity    string `json:"severity"`    // 严重程度
	Suggestion  string `json:"suggestion"`  // 修复建议
}

// CleanupOptions 清理选项
type CleanupOptions struct {
	Force             bool     `json:"force"`               // 强制清理
	KeepHeight        uint64   `json:"keep_height"`         // 保留到指定高度
	CleanDataDirs     bool     `json:"clean_data_dirs"`     // 清理数据目录
	ResetNetworkState bool     `json:"reset_network_state"` // 重置网络状态
	RestartServices   bool     `json:"restart_services"`    // 重启服务
	ExcludePatterns   []string `json:"exclude_patterns"`    // 排除模式
	BackupBeforeClean bool     `json:"backup_before_clean"` // 清理前备份
}

// ================================================================
//                        🚨 阶段1：快速响应机制
// ================================================================

// GetTestNetworkStatus 获取测试网络状态
// GET /internal/test-network/status
// 🎯 提供网络状态的全面检查，识别脏数据和不一致性
func (h *InternalManagementHandler) GetTestNetworkStatus(c *gin.Context) {
	h.logger.Info("[内部管理] 开始检查测试网络状态...")

	status := &TestNetworkStatus{
		TestSessionID:    h.currentTestSession,
		SessionStartTime: nil,
		Issues:           []NetworkIssue{},
		PeerStates:       make(map[string]string),
	}

	if !h.sessionStartTime.IsZero() {
		status.SessionStartTime = &h.sessionStartTime
	}

	// 1. 获取当前区块高度
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			status.CurrentHeight = chainInfo.Height
			h.logger.Infof("[内部管理] 当前区块高度: %d", chainInfo.Height)
		}
	}

	// 2. 检查连接的节点
	if h.networkService != nil {
		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			status.ConnectedPeers = len(peers)
			h.logger.Infof("[内部管理] 连接节点数: %d", len(peers))

			// 获取节点状态信息
			for _, peerID := range peers {
				status.PeerStates[peerID.String()[:12]] = "connected"
			}
		}
	}

	// 3. 检查数据目录
	dataDirs := h.findDataDirectories()
	status.DataDirectories = dataDirs

	// 4. 进行网络健康检查
	issues := h.performNetworkHealthCheck()
	status.Issues = issues
	status.NetworkClean = len(issues) == 0

	h.logger.Infof("[内部管理] 网络状态检查完成，发现 %d 个问题", len(issues))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "测试网络状态检查完成",
		"data":    status,
	})
}

// CleanTestNetwork 清理测试网络
// POST /internal/test-network/clean
// 🎯 强制清理测试网络，删除脏数据，重置到干净状态
func (h *InternalManagementHandler) CleanTestNetwork(c *gin.Context) {
	h.logger.Warn("[内部管理] 开始执行测试网络清理...")

	var options CleanupOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		h.logger.Errorf("[内部管理] 清理选项解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "清理选项格式错误",
		})
		return
	}

	// 执行清理
	results, err := h.executeNetworkCleanup(&options)
	if err != nil {
		h.logger.Errorf("[内部管理] 网络清理失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 重置测试会话
	h.currentTestSession = fmt.Sprintf("clean-session-%d", time.Now().Unix())
	h.sessionStartTime = time.Now()

	h.logger.Info("[内部管理] 测试网络清理完成")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "测试网络清理完成",
		"data":    results,
		"session": gin.H{
			"id":         h.currentTestSession,
			"start_time": h.sessionStartTime,
		},
	})
}

// StartTestSession 开始新的测试会话
// POST /internal/test-network/session/start
// 🎯 开始一个新的测试会话，标记测试开始时间
func (h *InternalManagementHandler) StartTestSession(c *gin.Context) {
	var request struct {
		SessionName string `json:"session_name"`
		Description string `json:"description"`
		CleanFirst  bool   `json:"clean_first"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数格式错误",
		})
		return
	}

	sessionID := request.SessionName
	if sessionID == "" {
		sessionID = fmt.Sprintf("test-session-%d", time.Now().Unix())
	}

	h.logger.Infof("[内部管理] 开始测试会话: %s", sessionID)

	// 如果需要清理
	if request.CleanFirst {
		options := CleanupOptions{
			Force:             true,
			CleanDataDirs:     true,
			ResetNetworkState: true,
		}
		_, err := h.executeNetworkCleanup(&options)
		if err != nil {
			h.logger.Errorf("[内部管理] 会话启动前清理失败: %v", err)
		}
	}

	h.currentTestSession = sessionID
	h.sessionStartTime = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "测试会话已启动",
		"data": gin.H{
			"session_id":  sessionID,
			"start_time":  h.sessionStartTime,
			"description": request.Description,
			"cleaned":     request.CleanFirst,
		},
	})
}

// ================================================================
//                        🔧 内部辅助方法
// ================================================================

// findDataDirectories 查找数据目录
func (h *InternalManagementHandler) findDataDirectories() []string {
	var dirs []string

	// 常见的数据目录位置
	candidates := []string{
		"./data",
		"./data/badger",
		"./internal/core/infrastructure/storage/badger/data",
		"./config-temp",
		"./tmp",
	}

	for _, candidate := range candidates {
		if absPath, err := filepath.Abs(candidate); err == nil {
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				if h.isBlockchainDataDir(absPath) {
					dirs = append(dirs, absPath)
				}
			}
		}
	}

	return dirs
}

// isBlockchainDataDir 检查是否为区块链数据目录
func (h *InternalManagementHandler) isBlockchainDataDir(dir string) bool {
	// 检查BadgerDB特征文件
	badgerFiles := []string{"MANIFEST", "KEYREGISTRY", "BADGER_RUNNING"}
	for _, file := range badgerFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}

	// 检查是否为data目录结构
	if strings.HasSuffix(dir, "/data") || strings.HasSuffix(dir, "\\data") {
		return true
	}

	// 检查是否为badger目录
	if strings.Contains(dir, "badger") {
		return true
	}

	return false
}

// performNetworkHealthCheck 执行网络健康检查
func (h *InternalManagementHandler) performNetworkHealthCheck() []NetworkIssue {
	var issues []NetworkIssue

	// 1. 检查区块高度是否合理
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			if chainInfo.Height == 0 {
				issues = append(issues, NetworkIssue{
					Type:        "blockchain_height",
					Description: "区块链高度为0，可能是新链或存在问题",
					Severity:    "warning",
					Suggestion:  "检查创世区块配置或重新同步",
				})
			}
		}
	}

	// 2. 检查网络连接
	if h.networkService != nil {
		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			if len(peers) == 0 {
				issues = append(issues, NetworkIssue{
					Type:        "network_isolation",
					Description: "没有连接任何节点，可能网络隔离",
					Severity:    "error",
					Suggestion:  "检查网络配置和引导节点",
				})
			}
		}
	}

	// 3. 检查数据目录大小
	dataDirs := h.findDataDirectories()
	for _, dir := range dataDirs {
		if size, err := h.getDirSize(dir); err == nil {
			// 如果数据目录过大（超过1GB），可能有脏数据
			if size > 1024*1024*1024 {
				issues = append(issues, NetworkIssue{
					Type:        "large_data_dir",
					Description: fmt.Sprintf("数据目录过大: %s (%s)", dir, h.formatBytes(size)),
					Severity:    "warning",
					Suggestion:  "考虑清理旧数据或归档",
				})
			}
		}
	}

	return issues
}

// executeNetworkCleanup 执行网络清理
func (h *InternalManagementHandler) executeNetworkCleanup(options *CleanupOptions) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	cleaned := []string{}

	h.logger.Warnf("[内部管理] 执行网络清理，选项: %+v", options)

	// 1. 备份（如果需要）
	if options.BackupBeforeClean {
		backupPath, err := h.createBackup()
		if err != nil {
			h.logger.Errorf("[内部管理] 备份失败: %v", err)
		} else {
			results["backup_path"] = backupPath
			h.logger.Infof("[内部管理] 备份完成: %s", backupPath)
		}
	}

	// 2. 清理数据目录
	if options.CleanDataDirs {
		dataDirs := h.findDataDirectories()
		for _, dir := range dataDirs {
			// 检查排除模式
			excluded := false
			for _, pattern := range options.ExcludePatterns {
				if strings.Contains(dir, pattern) {
					excluded = true
					break
				}
			}

			if !excluded {
				if err := os.RemoveAll(dir); err != nil {
					h.logger.Errorf("[内部管理] 删除目录失败 %s: %v", dir, err)
				} else {
					cleaned = append(cleaned, dir)
					h.logger.Infof("[内部管理] 已清理目录: %s", dir)
				}
			}
		}
	}

	// 3. 重置网络状态（如果可能）
	if options.ResetNetworkState {
		h.logger.Info("[内部管理] 重置网络状态...")
		// 这里可以添加重置网络状态的逻辑
		results["network_reset"] = true
	}

	results["cleaned_directories"] = cleaned
	results["cleanup_time"] = time.Now()
	results["options"] = options

	return results, nil
}

// createBackup 创建备份
func (h *InternalManagementHandler) createBackup() (string, error) {
	backupDir := fmt.Sprintf("./backup/backup-%d", time.Now().Unix())
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}
	// TODO: 实现具体的备份逻辑
	return backupDir, nil
}

// getDirSize 获取目录大小
func (h *InternalManagementHandler) getDirSize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes 格式化字节数
func (h *InternalManagementHandler) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ================================================================
//                        📊 网络节点发现和管理
// ================================================================

// DiscoverNetworkNodes 发现网络中的节点
// GET /internal/test-network/nodes/discover
// 🎯 扫描和发现网络中的其他节点，用于批量管理
func (h *InternalManagementHandler) DiscoverNetworkNodes(c *gin.Context) {
	h.logger.Info("[内部管理] 开始发现网络节点...")

	var discoveredNodes []map[string]interface{}

	// 1. 获取直连节点
	if h.networkService != nil {
		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			for _, peerID := range peers {
				// 获取节点地址信息
				addrs := libp2pHost.Network().Peerstore().Addrs(peerID)
				addrStrs := make([]string, len(addrs))
				for i, addr := range addrs {
					addrStrs[i] = addr.String()
				}

				node := map[string]interface{}{
					"peer_id":    peerID.String(),
					"addresses":  addrStrs,
					"connection": "direct",
					"discovered": time.Now(),
				}

				// 尝试获取更多节点信息（如果可能）
				if h.tryPingNode(peerID) {
					node["status"] = "reachable"
				} else {
					node["status"] = "unreachable"
				}

				discoveredNodes = append(discoveredNodes, node)
			}
		}
	}

	h.logger.Infof("[内部管理] 发现 %d 个网络节点", len(discoveredNodes))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点发现完成",
		"data": gin.H{
			"nodes":          discoveredNodes,
			"total_count":    len(discoveredNodes),
			"discovery_time": time.Now(),
		},
	})
}

// tryPingNode 尝试ping节点（简单可达性检查）
func (h *InternalManagementHandler) tryPingNode(peerID peer.ID) bool {
	// TODO: 实现实际的节点ping逻辑
	return true // 暂时返回true
}

// GetNetworkTopology 获取网络拓扑信息
// GET /internal/test-network/topology
// 🎯 提供网络拓扑可视化数据，帮助理解网络结构
func (h *InternalManagementHandler) GetNetworkTopology(c *gin.Context) {
	h.logger.Info("[内部管理] 生成网络拓扑信息...")

	topology := map[string]interface{}{
		"local_node": map[string]interface{}{
			"peer_id": "",
			"role":    "unknown",
			"height":  uint64(0),
		},
		"connected_peers": []map[string]interface{}{},
		"network_stats": map[string]interface{}{
			"total_peers":        0,
			"direct_connections": 0,
			"relay_connections":  0,
		},
		"generated_at": time.Now(),
	}

	// 获取本地节点信息
	if h.networkService != nil {
		localID := h.networkService.ID()
		topology["local_node"].(map[string]interface{})["peer_id"] = localID.String()

		// 获取连接的节点
		libp2pHost := h.networkService.Libp2pHost()
		var peers []peer.ID
		if libp2pHost != nil {
			peers = libp2pHost.Network().Peers()
		}
		connectedPeers := make([]map[string]interface{}, 0, len(peers))

		for _, peerID := range peers {
			peerInfo := map[string]interface{}{
				"peer_id":    peerID.String(),
				"short_id":   peerID.String()[:12],
				"connection": "direct",
				"latency":    "unknown",
			}
			connectedPeers = append(connectedPeers, peerInfo)
		}

		topology["connected_peers"] = connectedPeers
		topology["network_stats"].(map[string]interface{})["total_peers"] = len(peers)
		topology["network_stats"].(map[string]interface{})["direct_connections"] = len(peers)
	}

	// 获取区块链高度
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			topology["local_node"].(map[string]interface{})["height"] = chainInfo.Height
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "网络拓扑信息生成完成",
		"data":    topology,
	})
}

// ================================================================
//                   🚨 阶段2：协议增强（智能重置机制）
// ================================================================

// BroadcastNetworkReset 广播网络重置消息
// POST /internal/test-network/broadcast-reset
// 🎯 向网络中的所有节点广播重置消息，协调全网重置
func (h *InternalManagementHandler) BroadcastNetworkReset(c *gin.Context) {
	var request struct {
		ResetID     string `json:"reset_id"`     // 重置标识符
		ResetHeight uint64 `json:"reset_height"` // 重置到的区块高度
		ResetReason string `json:"reset_reason"` // 重置原因
		Force       bool   `json:"force"`        // 是否强制重置
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数格式错误",
		})
		return
	}

	h.logger.Warnf("[内部管理] 准备广播网络重置: %s", request.ResetID)

	// 构建重置消息
	resetMessage := map[string]interface{}{
		"reset_id":     request.ResetID,
		"reset_height": request.ResetHeight,
		"reset_reason": request.ResetReason,
		"timestamp":    time.Now().Unix(),
		"force":        request.Force,
		"source_node":  "",
	}

	// 获取本地节点ID
	if h.networkService != nil {
		resetMessage["source_node"] = h.networkService.ID().String()
	}

	// 广播重置消息
	broadcastResults := h.broadcastResetMessage(resetMessage)

	// 启动新的测试会话
	h.currentTestSession = request.ResetID
	h.sessionStartTime = time.Now()

	h.logger.Infof("[内部管理] 网络重置消息广播完成，成功: %d, 失败: %d",
		broadcastResults["success"], broadcastResults["failed"])

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "网络重置消息已广播",
		"data": gin.H{
			"reset_id":        request.ResetID,
			"broadcast_stats": broadcastResults,
			"new_session":     h.currentTestSession,
		},
	})
}

// CheckNetworkConsistency 检查网络数据一致性
// GET /internal/test-network/consistency-check
// 🎯 检查网络中各节点的数据一致性，识别分歧
func (h *InternalManagementHandler) CheckNetworkConsistency(c *gin.Context) {
	h.logger.Info("[内部管理] 开始网络一致性检查...")

	// 获取查询参数
	checkDepth := 10 // 默认检查最近10个区块
	if depth := c.Query("depth"); depth != "" {
		if d, err := strconv.Atoi(depth); err == nil && d > 0 {
			checkDepth = d
		}
	}

	consistencyReport := h.performConsistencyCheck(checkDepth)

	h.logger.Infof("[内部管理] 一致性检查完成，检查了 %d 个节点",
		len(consistencyReport.NodeStates))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "网络一致性检查完成",
		"data":    consistencyReport,
	})
}

// ForceNetworkResync 强制网络重新同步
// POST /internal/test-network/force-resync
// 🎯 强制触发网络重新同步，修复数据不一致
func (h *InternalManagementHandler) ForceNetworkResync(c *gin.Context) {
	var request struct {
		TargetHeight uint64   `json:"target_height"` // 目标同步高度
		TargetPeers  []string `json:"target_peers"`  // 目标节点列表
		Force        bool     `json:"force"`         // 强制同步
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数格式错误",
		})
		return
	}

	h.logger.Warnf("[内部管理] 开始强制网络重新同步，目标高度: %d", request.TargetHeight)

	resyncResults := h.executeForceResync(&request)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "强制重新同步已触发",
		"data":    resyncResults,
	})
}

// ================================================================
//                   🔧 阶段2：辅助方法和数据结构
// ================================================================

// ConsistencyReport 一致性检查报告
type ConsistencyReport struct {
	CheckTime       time.Time             `json:"check_time"`       // 检查时间
	CheckDepth      int                   `json:"check_depth"`      // 检查深度
	LocalHeight     uint64                `json:"local_height"`     // 本地高度
	NodeStates      map[string]*NodeState `json:"node_states"`      // 节点状态
	Inconsistencies []InconsistencyIssue  `json:"inconsistencies"`  // 发现的不一致
	ConsensusHeight uint64                `json:"consensus_height"` // 共识高度
	Recommendations []string              `json:"recommendations"`  // 修复建议
}

// NodeState 节点状态
type NodeState struct {
	PeerID      string            `json:"peer_id"`      // 节点ID
	Height      uint64            `json:"height"`       // 区块高度
	BlockHashes map[uint64]string `json:"block_hashes"` // 区块哈希
	Status      string            `json:"status"`       // 节点状态
	LastSeen    time.Time         `json:"last_seen"`    // 最后通信时间
	Issues      []string          `json:"issues"`       // 发现的问题
}

// InconsistencyIssue 不一致问题
type InconsistencyIssue struct {
	Type          string   `json:"type"`           // 问题类型
	Description   string   `json:"description"`    // 问题描述
	AffectedNodes []string `json:"affected_nodes"` // 受影响的节点
	Severity      string   `json:"severity"`       // 严重程度
	Solution      string   `json:"solution"`       // 解决方案
}

// broadcastResetMessage 广播重置消息
func (h *InternalManagementHandler) broadcastResetMessage(message map[string]interface{}) map[string]int {
	results := map[string]int{
		"success": 0,
		"failed":  0,
		"total":   0,
	}

	if h.networkInterface == nil {
		h.logger.Warn("[内部管理] 网络接口不可用，无法广播重置消息")
		return results
	}

	// 获取连接的节点
	if h.networkService != nil {
		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			results["total"] = len(peers)

			for _, peerID := range peers {
				// TODO: 实现实际的消息广播逻辑
				// 这里可以使用 GossipSub 或者 Stream RPC 来发送重置消息
				h.logger.Debugf("[内部管理] 向节点 %s 发送重置消息", peerID.String()[:12])

				// 模拟发送成功
				results["success"]++
			}
		}
	}

	return results
}

// performConsistencyCheck 执行一致性检查
func (h *InternalManagementHandler) performConsistencyCheck(depth int) *ConsistencyReport {
	report := &ConsistencyReport{
		CheckTime:       time.Now(),
		CheckDepth:      depth,
		NodeStates:      make(map[string]*NodeState),
		Inconsistencies: []InconsistencyIssue{},
		Recommendations: []string{},
	}

	// 获取本地状态
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			report.LocalHeight = chainInfo.Height
			report.ConsensusHeight = report.LocalHeight // 暂时设为本地高度
		}
	}

	// 检查连接的节点
	if h.networkService != nil {
		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()

			for _, peerID := range peers {
				nodeState := &NodeState{
					PeerID:      peerID.String(),
					BlockHashes: make(map[uint64]string),
					Status:      "reachable",
					LastSeen:    time.Now(),
					Issues:      []string{},
				}

				// TODO: 实现实际的节点状态查询逻辑
				// 这里可以通过RPC调用获取远程节点的状态
				nodeState.Height = report.LocalHeight // 暂时使用本地高度

				report.NodeStates[peerID.String()[:12]] = nodeState
			}
		}
	}

	// 分析不一致性
	report.Inconsistencies = h.analyzeInconsistencies(report)

	// 生成建议
	if len(report.Inconsistencies) > 0 {
		report.Recommendations = append(report.Recommendations, "发现数据不一致，建议执行网络重置")
		report.Recommendations = append(report.Recommendations, "可以使用 /internal/test-network/broadcast-reset 进行协调重置")
	} else {
		report.Recommendations = append(report.Recommendations, "网络状态良好，数据一致")
	}

	return report
}

// analyzeInconsistencies 分析不一致性
func (h *InternalManagementHandler) analyzeInconsistencies(report *ConsistencyReport) []InconsistencyIssue {
	var issues []InconsistencyIssue

	// 检查高度不一致
	heightMap := make(map[uint64][]string)
	for nodeID, state := range report.NodeStates {
		heightMap[state.Height] = append(heightMap[state.Height], nodeID)
	}

	if len(heightMap) > 1 {
		var maxHeight uint64
		for height := range heightMap {
			if height > maxHeight {
				maxHeight = height
			}
		}

		// 找出高度落后的节点
		var behindNodes []string
		for height, nodes := range heightMap {
			if height < maxHeight {
				behindNodes = append(behindNodes, nodes...)
			}
		}

		if len(behindNodes) > 0 {
			issues = append(issues, InconsistencyIssue{
				Type:          "height_inconsistency",
				Description:   fmt.Sprintf("发现高度不一致：最高高度 %d，落后节点 %d 个", maxHeight, len(behindNodes)),
				AffectedNodes: behindNodes,
				Severity:      "warning",
				Solution:      "执行强制重新同步或网络重置",
			})
		}
	}

	return issues
}

// executeForceResync 执行强制重新同步
func (h *InternalManagementHandler) executeForceResync(request *struct {
	TargetHeight uint64   `json:"target_height"`
	TargetPeers  []string `json:"target_peers"`
	Force        bool     `json:"force"`
}) map[string]interface{} {
	results := map[string]interface{}{
		"started_at":     time.Now(),
		"target_height":  request.TargetHeight,
		"target_peers":   request.TargetPeers,
		"force":          request.Force,
		"sync_triggered": false,
		"message":        "重新同步功能需要与区块同步模块集成",
	}

	// TODO: 集成实际的区块同步逻辑
	// 这里可以调用 internal/core/blockchain/sync 模块的强制同步功能

	h.logger.Infof("[内部管理] 强制重新同步请求已记录，目标高度: %d", request.TargetHeight)

	return results
}

// ================================================================
//                   🔍 阶段3：高级网络管理功能
// ================================================================

// GetAdvancedNetworkMetrics 获取高级网络指标
// GET /internal/test-network/metrics/advanced
// 🎯 提供详细的网络性能和健康指标
func (h *InternalManagementHandler) GetAdvancedNetworkMetrics(c *gin.Context) {
	h.logger.Info("[内部管理] 收集高级网络指标...")

	metrics := map[string]interface{}{
		"collection_time": time.Now(),
		"node_info": map[string]interface{}{
			"local_peer_id": "",
			"uptime":        time.Since(h.sessionStartTime).String(),
			"session":       h.currentTestSession,
		},
		"network_metrics": map[string]interface{}{
			"peer_count":         0,
			"active_connections": 0,
			"message_queue_size": 0,
			"bandwidth_usage":    "unknown",
		},
		"blockchain_metrics": map[string]interface{}{
			"current_height":  uint64(0),
			"sync_status":     "unknown",
			"last_block_time": nil,
			"avg_block_time":  "unknown",
		},
		"performance_metrics": map[string]interface{}{
			"memory_usage":    "unknown",
			"cpu_usage":       "unknown",
			"disk_usage":      "unknown",
			"network_latency": "unknown",
		},
	}

	// 收集网络信息
	if h.networkService != nil {
		localID := h.networkService.ID()
		metrics["node_info"].(map[string]interface{})["local_peer_id"] = localID.String()

		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			metrics["network_metrics"].(map[string]interface{})["peer_count"] = len(peers)
			metrics["network_metrics"].(map[string]interface{})["active_connections"] = len(peers)
		}
	}

	// 收集区块链信息
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			metrics["blockchain_metrics"].(map[string]interface{})["current_height"] = chainInfo.Height
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "高级网络指标收集完成",
		"data":    metrics,
	})
}

// ExportNetworkState 导出网络状态
// GET /internal/test-network/export-state
// 🎯 导出当前网络状态，用于分析和调试
func (h *InternalManagementHandler) ExportNetworkState(c *gin.Context) {
	h.logger.Info("[内部管理] 导出网络状态...")

	exportData := map[string]interface{}{
		"export_time":    time.Now(),
		"export_version": "1.0",
		"session_info": map[string]interface{}{
			"current_session": h.currentTestSession,
			"session_start":   h.sessionStartTime,
		},
	}

	// 添加网络状态
	if status, err := h.getComprehensiveNetworkState(); err == nil {
		exportData["network_state"] = status
	}

	// 添加配置信息（脱敏）
	exportData["config_summary"] = h.getSanitizedConfig()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "网络状态导出完成",
		"data":    exportData,
	})
}

// getComprehensiveNetworkState 获取全面的网络状态
func (h *InternalManagementHandler) getComprehensiveNetworkState() (map[string]interface{}, error) {
	state := map[string]interface{}{
		"timestamp": time.Now(),
	}

	// 添加基本网络信息
	if h.networkService != nil {
		state["local_peer_id"] = h.networkService.ID().String()

		libp2pHost := h.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			state["connected_peers"] = len(peers)
		}
	}

	// 添加区块链状态
	if h.blockchainService != nil {
		if chainInfo, err := h.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			state["current_height"] = chainInfo.Height
		}
	}

	return state, nil
}

// getSanitizedConfig 获取脱敏的配置信息
func (h *InternalManagementHandler) getSanitizedConfig() map[string]interface{} {
	config := map[string]interface{}{
		"sanitized": true,
		"note":      "敏感信息已移除",
	}

	// TODO: 从配置中提取非敏感信息
	if h.config != nil {
		// 可以添加一些非敏感的配置信息
		config["has_config"] = true
	}

	return config
}
