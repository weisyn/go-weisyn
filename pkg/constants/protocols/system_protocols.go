// Package protocols 提供WES系统全局网络协议常量定义
//
// 🎯 **全局协议常量归口管理**
//
// 本文件定义跨组件共享的标准网络协议，解决协议复用和版本管理问题：
// - 基础协议：心跳、发现、状态同步等通用协议
// - 跨组件协议：多个组件都需要使用的业务协议
// - 版本管理：统一的协议版本控制和兼容性管理
//
// 🔧 **设计原则**
// - 全局复用：跨组件协议必须在此定义
// - 版本控制：语义化版本管理，兼容性保证
// - 命名规范：/weisyn/domain/action/version 格式
// - 分类管理：基础设施协议、业务协议、管理协议
//
// 🏗️ **使用方式**
// ```go
// import "github.com/weisyn/v1/pkg/constants/protocols"
//
// // 注册协议处理器
// network.RegisterStreamHandler(protocols.ProtocolHeartbeat, heartbeatHandler)
//
// // 发起协议请求
// response, err := network.Call(peerID, protocols.ProtocolNodeInfo, request)
// ```
package protocols

import "strings"

// ============================================================================
//                           基础设施协议（跨组件复用）
// ============================================================================

// 节点发现和连接管理协议
const (
	// ProtocolNodeInfo 节点信息交换协议
	// 用途：交换节点基本信息，包括版本、能力、配置等
	// 使用者：所有需要了解对端节点信息的组件
	// 格式：/weisyn/node/info/v1.0.0
	ProtocolNodeInfo = "/weisyn/node/info/v1.0.0"

	// ProtocolHeartbeat 心跳检测协议
	// 用途：检测节点存活状态和网络连通性
	// 使用者：network、consensus、blockchain等组件
	// 格式：/weisyn/node/heartbeat/v1.0.0
	ProtocolHeartbeat = "/weisyn/node/heartbeat/v1.0.0"

	// ProtocolPeerDiscovery 节点发现协议
	// 用途：发现网络中的其他节点，建立连接图谱
	// 使用者：network组件，其他组件间接受益
	// 格式：/weisyn/node/discovery/v1.0.0
	ProtocolPeerDiscovery = "/weisyn/node/discovery/v1.0.0"
)

// 健康检查和监控协议
const (
	// ProtocolHealthCheck 健康状态检查协议
	// 用途：检查节点各组件的运行状态
	// 使用者：监控系统、运维工具
	// 格式：/weisyn/health/check/v1.0.0
	ProtocolHealthCheck = "/weisyn/health/check/v1.0.0"

	// ProtocolStatusSync 状态同步协议
	// 用途：同步节点间的运行状态信息
	// 使用者：集群管理、负载均衡组件
	// 格式：/weisyn/status/sync/v1.0.0
	ProtocolStatusSync = "/weisyn/status/sync/v1.0.0"
)

// ============================================================================
//                           业务协议（跨组件使用）
// ============================================================================

// 区块链同步协议（blockchain + consensus + network）
const (
	// ProtocolBlockSync 区块同步协议
	// 用途：节点间同步区块数据
	// 使用者：blockchain（同步管理）、consensus（高度同步）
	// 格式：/weisyn/blockchain/block_sync/v1.0.0
	ProtocolBlockSync = "/weisyn/blockchain/block_sync/v1.0.0"

	// ProtocolHeaderSync 区块头同步协议
	// 用途：快速同步区块头信息，用于高度检查
	// 使用者：blockchain、consensus组件
	// 格式：/weisyn/blockchain/header_sync/v1.0.0
	ProtocolHeaderSync = "/weisyn/blockchain/header_sync/v1.0.0"

	// ProtocolStateSync 状态同步协议
	// 用途：同步区块链状态数据（UTXO等）
	// 使用者：blockchain、repository组件
	// 格式：/weisyn/blockchain/state_sync/v1.0.0
	ProtocolStateSync = "/weisyn/blockchain/state_sync/v1.0.0"

	// ProtocolKBucketSync K-bucket智能同步协议
	// 用途：基于Kademlia距离算法进行智能节点选择和区块同步
	// 使用者：blockchain/sync组件
	// 格式：/weisyn/sync/kbucket/1.0.0
	ProtocolKBucketSync = "/weisyn/sync/kbucket/1.0.0"

	// ProtocolRangePaginated 智能分页区块范围同步协议
	// 用途：接收方智能分页的批量区块同步
	// 使用者：blockchain/sync组件
	// 格式：/weisyn/sync/range_paginated/1.0.0
	ProtocolRangePaginated = "/weisyn/sync/range_paginated/1.0.0"

	// ProtocolSyncHelloV2 同步握手协议（v2，fork-aware）
	// 用途：请求方携带 tip(height+hash)+locator，与对端判定链关系与共同祖先
	// 使用者：blockchain/sync组件
	// 格式：/weisyn/sync/hello/2.0.0
	ProtocolSyncHelloV2 = "/weisyn/sync/hello/2.0.0"

	// ProtocolSyncBlocksV2 区块批量同步协议（v2，fork-aware）
	// 用途：在确认同链可线性同步后，按范围拉取 blocks
	// 使用者：blockchain/sync组件
	// 格式：/weisyn/sync/blocks/2.0.0
	ProtocolSyncBlocksV2 = "/weisyn/sync/blocks/2.0.0"

	// ProtocolTransactionDirect 交易直连传播协议（备用传播路径）
	// 用途：Stream RPC确保送达模式，K-bucket选择2-3个邻近节点
	// 使用者：blockchain/transaction组件
	// 格式：/weisyn/blockchain/tx_direct/1.0.0
	ProtocolTransactionDirect = "/weisyn/blockchain/tx_direct/1.0.0"
)

// 共识协调协议（consensus + network）
const (
	// ProtocolConsensusCoordination 共识协调协议
	// 用途：共识节点间的协调通信
	// 使用者：consensus组件的聚合器和矿工
	// 格式：/weisyn/consensus/coordination/v1.0.0
	ProtocolConsensusCoordination = "/weisyn/consensus/coordination/v1.0.0"

	// ProtocolBlockSubmission 矿工区块提交协议
	// 用途：矿工向聚合器提交候选区块，基于K-bucket近邻选择和受控扇出
	// 使用者：consensus/aggregator和consensus/miner组件
	// 格式：/weisyn/consensus/block_submission/1.0.0
	ProtocolBlockSubmission = "/weisyn/consensus/block_submission/1.0.0"

	// ProtocolConsensusHeartbeat 共识心跳协议
	// 用途：节点间的状态同步和网络健康监控
	// 使用者：consensus组件的聚合器和矿工
	// 格式：/weisyn/consensus/heartbeat/1.0.0
	ProtocolConsensusHeartbeat = "/weisyn/consensus/heartbeat/1.0.0"

	// ProtocolAggregatorStatus 聚合器状态查询协议（V2 新增）
	// 用途：提交者主动查询聚合器状态，处理广播丢失场景
	// 使用者：consensus/miner 和 consensus/aggregator 组件
	// 格式：/weisyn/consensus/aggregator_status/1.0.0
	ProtocolAggregatorStatus = "/weisyn/consensus/aggregator_status/1.0.0"

	// ProtocolNetworkQualityReport 网络质量报告协议
	// 用途：上报和同步网络质量信息
	// 使用者：network（质量监控）、consensus（策略调整）
	// 格式：/weisyn/network/quality_report/v1.0.0
	ProtocolNetworkQualityReport = "/weisyn/network/quality_report/v1.0.0"
)

// ============================================================================
//                           订阅主题（跨组件广播）
// ============================================================================

// 系统级广播主题
const (
	// TopicSystemAnnouncements 系统公告主题
	// 用途：广播系统级重要通知
	// 使用者：所有组件都应该订阅
	// 格式：weisyn.system.announcements.v1
	TopicSystemAnnouncements = "weisyn.system.announcements.v1"

	// TopicNetworkStatus 网络状态主题
	// 用途：广播网络状态变化信息
	// 使用者：所有需要感知网络状态的组件
	// 格式：weisyn.network.status.v1
	TopicNetworkStatus = "weisyn.network.status.v1"

	// TopicEmergencyBroadcast 紧急广播主题
	// 用途：紧急情况通知（分叉、网络分区等）
	// 使用者：所有组件，高优先级处理
	// 格式：weisyn.emergency.broadcast.v1
	TopicEmergencyBroadcast = "weisyn.emergency.broadcast.v1"

	// TopicTransactionAnnounce 交易广播通告主题（主要传播路径）
	// 用途：GossipSub订阅模式，fire-and-forget全网交易广播
	// 使用者：blockchain/transaction组件
	// 格式：weisyn.blockchain.tx_announce.v1
	TopicTransactionAnnounce = "weisyn.blockchain.tx_announce.v1"

	// TopicConsensusResult 共识结果广播主题
	// 用途：聚合器向全网广播最终的共识决策结果
	// 使用者：consensus/aggregator组件
	// 格式：weisyn.consensus.latest_block.v1
	TopicConsensusResult = "weisyn.consensus.latest_block.v1"
)

// ============================================================================
//                           协议版本管理
// ============================================================================

// CurrentProtocolVersion 当前全局协议版本
const CurrentProtocolVersion = "v1.0.0"

// ProtocolVersionInfo 协议版本信息
type ProtocolVersionInfo struct {
	// CurrentVersion 当前版本
	CurrentVersion string
	// CompatibleVersions 兼容的版本列表（按优先级降序）
	CompatibleVersions []string
	// DeprecatedVersions 已废弃但仍支持的版本
	DeprecatedVersions []string
	// MinVersion 最低支持版本
	MinVersion string
}

// 协议版本兼容性映射（简化版，向后兼容）
var ProtocolCompatibility = map[string][]string{
	// 节点信息协议兼容性
	ProtocolNodeInfo: {"v1.0.0"},

	// 心跳协议兼容性
	ProtocolHeartbeat: {"v1.0.0"},

	// 区块同步协议兼容性
	ProtocolBlockSync: {"v1.0.0"},

	// 区块提交协议兼容性
	ProtocolBlockSubmission: {"1.0.0"},

	// 共识心跳协议兼容性
	ProtocolConsensusHeartbeat: {"1.0.0"},

	// 聚合器状态协议兼容性
	ProtocolAggregatorStatus: {"1.0.0"},
}

// ProtocolVersionRegistry 协议版本注册表（详细版）
// 🆕 2025-12-19 新增：支持多版本协议协商和回退
var ProtocolVersionRegistry = map[string]*ProtocolVersionInfo{
	// 区块提交协议 - 核心共识协议
	ProtocolBlockSubmission: {
		CurrentVersion:     "1.0.0",
		CompatibleVersions: []string{"1.0.0"},
		DeprecatedVersions: []string{},
		MinVersion:         "1.0.0",
	},

	// 共识心跳协议
	ProtocolConsensusHeartbeat: {
		CurrentVersion:     "1.0.0",
		CompatibleVersions: []string{"1.0.0"},
		DeprecatedVersions: []string{},
		MinVersion:         "1.0.0",
	},

	// 聚合器状态协议
	ProtocolAggregatorStatus: {
		CurrentVersion:     "1.0.0",
		CompatibleVersions: []string{"1.0.0"},
		DeprecatedVersions: []string{},
		MinVersion:         "1.0.0",
	},

	// 同步握手协议 V2
	ProtocolSyncHelloV2: {
		CurrentVersion:     "2.0.0",
		CompatibleVersions: []string{"2.0.0"},
		DeprecatedVersions: []string{},
		MinVersion:         "2.0.0",
	},

	// K-bucket 同步协议
	ProtocolKBucketSync: {
		CurrentVersion:     "1.0.0",
		CompatibleVersions: []string{"1.0.0"},
		DeprecatedVersions: []string{},
		MinVersion:         "1.0.0",
	},
}

// GetProtocolVersionInfo 获取协议的版本信息
func GetProtocolVersionInfo(protocol string) *ProtocolVersionInfo {
	if info, ok := ProtocolVersionRegistry[protocol]; ok {
		return info
	}
	return nil
}

// GetProtocolAllVersions 获取协议的所有支持版本（用于协议协商）
// 返回按优先级降序排列的版本列表
func GetProtocolAllVersions(protocol string) []string {
	info := GetProtocolVersionInfo(protocol)
	if info == nil {
		return nil
	}

	// 合并当前版本、兼容版本和废弃版本
	versions := make([]string, 0, len(info.CompatibleVersions)+len(info.DeprecatedVersions))
	versions = append(versions, info.CompatibleVersions...)
	versions = append(versions, info.DeprecatedVersions...)
	return versions
}

// GetProtocolVariants 获取协议的所有变体（用于协议检查）
// 返回协议的所有可能形式：原始ID、带命名空间的ID、不同版本等
func GetProtocolVariants(baseProtocol, namespace string) []string {
	variants := make([]string, 0, 4)

	// 1. 原始协议ID
	variants = append(variants, baseProtocol)

	// 2. 带命名空间的协议ID
	if namespace != "" {
		variants = append(variants, QualifyProtocol(baseProtocol, namespace))
	}

	// 3. 如果协议有多个版本，添加其他版本变体
	info := GetProtocolVersionInfo(baseProtocol)
	if info != nil {
		// 从协议ID中提取基础路径（不含版本）
		basePath := extractProtocolBasePath(baseProtocol)
		if basePath != "" {
			for _, version := range info.CompatibleVersions {
				variant := basePath + version
				if variant != baseProtocol {
					variants = append(variants, variant)
					if namespace != "" {
						variants = append(variants, QualifyProtocol(variant, namespace))
					}
				}
			}
		}
	}

	return variants
}

// extractProtocolBasePath 从协议ID中提取基础路径（不含版本）
// 例如：/weisyn/consensus/block_submission/1.0.0 -> /weisyn/consensus/block_submission/
func extractProtocolBasePath(protocol string) string {
	// 查找最后一个 / 的位置
	lastSlash := strings.LastIndex(protocol, "/")
	if lastSlash == -1 || lastSlash == len(protocol)-1 {
		return ""
	}
	return protocol[:lastSlash+1]
}

// ExtractProtocolBasePath 从协议ID中提取基础路径（不含版本）- 导出版本
// 例如：/weisyn/consensus/block_submission/1.0.0 -> /weisyn/consensus/block_submission/
func ExtractProtocolBasePath(protocol string) string {
	return extractProtocolBasePath(protocol)
}

// GetProtocolVersion 从协议ID中提取版本号
// 例如：/weisyn/consensus/block_submission/1.0.0 -> 1.0.0
func GetProtocolVersion(protocol string) string {
	// 查找最后一个 / 的位置
	lastSlash := strings.LastIndex(protocol, "/")
	if lastSlash == -1 || lastSlash == len(protocol)-1 {
		return ""
	}
	return protocol[lastSlash+1:]
}

// IsProtocolVersionCompatible 检查协议版本是否兼容
func IsProtocolVersionCompatible(protocol, version string) bool {
	info := GetProtocolVersionInfo(protocol)
	if info == nil {
		// 如果没有注册信息，使用简化的兼容性映射
		if versions, ok := ProtocolCompatibility[protocol]; ok {
			for _, v := range versions {
				if v == version {
					return true
				}
			}
		}
		return false
	}

	// 检查是否在兼容版本列表中
	for _, v := range info.CompatibleVersions {
		if v == version {
			return true
		}
	}

	// 检查是否在废弃版本列表中
	for _, v := range info.DeprecatedVersions {
		if v == version {
			return true
		}
	}

	return false
}

// ============================================================================
//                           协议工具函数
// ============================================================================

// QualifyProtocol 为协议ID添加网络命名空间
// 🎯 **网络命名空间化协议ID生成器**
//
// 将基础协议ID转换为带有网络命名空间的完整协议ID，实现网络隔离。
//
// 格式转换：
//   - 输入：/weisyn/node/info/v1.0.0
//   - 输出：/weisyn/{namespace}/node/info/v1.0.0
//
// 参数：
//   - baseProtocol: 基础协议ID（系统预定义的协议常量）
//   - namespace: 网络命名空间（如"mainnet", "testnet", "dev"）
//
// 返回：
//   - string: 带命名空间的完整协议ID
//
// 用法：
//
//	qualifiedProtocol := QualifyProtocol(ProtocolNodeInfo, "testnet")
//	// 结果：/weisyn/testnet/node/info/v1.0.0
func QualifyProtocol(baseProtocol, namespace string) string {
	// 🛡️ 强制要求 namespace 不能为空（fail-fast）
	if namespace == "" {
		panic("QualifyProtocol: namespace cannot be empty - network_namespace must be explicitly configured")
	}

	// ✅ 幂等：如果已经带了同样的 namespace，则直接返回，避免重复插入
	// 期望格式：/weisyn/{namespace}/...
	if strings.HasPrefix(baseProtocol, "/weisyn/"+namespace+"/") {
		return baseProtocol
	}

	// 检查是否为weisyn协议格式：/weisyn/...
	if len(baseProtocol) >= 8 && baseProtocol[:8] == "/weisyn/" {
		// 在/weisyn/后插入命名空间
		return "/weisyn/" + namespace + baseProtocol[7:]
	}

	// 非标准格式，直接返回原协议ID（但记录警告，建议使用标准格式）
	// 注意：这里不 panic，因为可能有一些系统协议不使用 /weisyn/ 前缀
	return baseProtocol
}

// QualifyTopic 为GossipSub主题添加网络命名空间
// 🎯 **网络命名空间化主题名生成器**
//
// 将基础主题名转换为带有网络命名空间的完整主题名，实现网络隔离。
//
// 格式转换：
//   - 输入：weisyn.blockchain.tx_announce.v1
//   - 输出：weisyn.{namespace}.blockchain.tx_announce.v1
//
// 参数：
//   - baseTopic: 基础主题名（系统预定义的主题常量）
//   - namespace: 网络命名空间（如"mainnet", "testnet", "dev"）
//
// 返回：
//   - string: 带命名空间的完整主题名
//
// 用法：
//
//	qualifiedTopic := QualifyTopic(TopicTransactionAnnounce, "testnet")
//	// 结果：weisyn.testnet.blockchain.tx_announce.v1
func QualifyTopic(baseTopic, namespace string) string {
	// 🛡️ 强制要求 namespace 不能为空（fail-fast）
	if namespace == "" {
		panic("QualifyTopic: namespace cannot be empty - network_namespace must be explicitly configured")
	}

	// ✅ 幂等：如果已经带了同样的 namespace，则直接返回，避免重复插入
	// 期望格式：weisyn.{namespace}.<domain>.<name>.<version>
	if strings.HasPrefix(baseTopic, "weisyn."+namespace+".") {
		return baseTopic
	}

	// 检查是否为weisyn主题格式：weisyn.
	if len(baseTopic) >= 7 && baseTopic[:7] == "weisyn." {
		// 在weisyn.后插入命名空间
		return "weisyn." + namespace + "." + baseTopic[7:]
	}

	// 非标准格式，直接返回原主题名（但记录警告，建议使用标准格式）
	// 注意：这里不 panic，因为可能有一些系统主题不使用 weisyn. 前缀
	return baseTopic
}

// QualifyDHTPrefix 为DHT协议前缀添加网络命名空间
// 🎯 **DHT协议前缀命名空间化生成器**
//
// 将基础DHT前缀转换为带有网络命名空间的完整前缀，实现DHT网络隔离。
//
// 格式转换：
//   - 输入：/weisyn
//   - 输出：/weisyn/{namespace}
//
// 参数：
//   - baseDHTPrefix: 基础DHT协议前缀
//   - namespace: 网络命名空间（如"mainnet", "testnet", "dev"）
//
// 返回：
//   - string: 带命名空间的完整DHT前缀
func QualifyDHTPrefix(baseDHTPrefix, namespace string) string {
	// 🛡️ 强制要求 namespace 不能为空（fail-fast）
	if namespace == "" {
		panic("QualifyDHTPrefix: namespace cannot be empty - network_namespace must be explicitly configured")
	}

	// 确保前缀以/结尾
	if baseDHTPrefix[len(baseDHTPrefix)-1] != '/' {
		return baseDHTPrefix + "/" + namespace
	}

	return baseDHTPrefix + namespace
}

// QualifyMDNSService 为mDNS服务名添加网络命名空间
// 🎯 **mDNS服务名命名空间化生成器**
//
// 将基础mDNS服务名转换为带有网络命名空间的完整服务名，实现mDNS发现隔离。
//
// 格式转换：
//   - 输入：weisyn-node
//   - 输出：weisyn-node-{namespace}
//
// 参数：
//   - baseMDNSService: 基础mDNS服务名
//   - namespace: 网络命名空间（如"mainnet", "testnet", "dev"）
//
// 返回：
//   - string: 带命名空间的完整mDNS服务名
func QualifyMDNSService(baseMDNSService, namespace string) string {
	// 🛡️ 强制要求 namespace 不能为空（fail-fast）
	if namespace == "" {
		panic("QualifyMDNSService: namespace cannot be empty - network_namespace must be explicitly configured")
	}

	return baseMDNSService + "-" + namespace
}

// IsSystemProtocol 判断是否为系统级协议
// 系统级协议具有更高的处理优先级
func IsSystemProtocol(protocol string) bool {
	systemProtocols := []string{
		ProtocolHeartbeat,
		ProtocolHealthCheck,
		ProtocolStatusSync,
		ProtocolPeerDiscovery,
	}

	for _, sysProtocol := range systemProtocols {
		if protocol == sysProtocol {
			return true
		}
	}
	return false
}

// GetProtocolCategory 获取协议分类
func GetProtocolCategory(protocol string) string {
	switch protocol { //nolint:staticcheck // QF1002: 使用 tagged switch 更清晰
	case ProtocolNodeInfo, ProtocolHeartbeat, ProtocolPeerDiscovery:
		return "node_management"
	case ProtocolHealthCheck, ProtocolStatusSync:
		return "monitoring"
	case ProtocolBlockSync, ProtocolHeaderSync, ProtocolStateSync:
		return "blockchain_sync"
	case ProtocolConsensusCoordination, ProtocolNetworkQualityReport:
		return "consensus_coordination"
	default:
		return "unknown"
	}
}

// IsCompatibleVersion 检查协议版本兼容性
func IsCompatibleVersion(protocol string, version string) bool {
	compatibleVersions, exists := ProtocolCompatibility[protocol]
	if !exists {
		return false
	}

	for _, compatVersion := range compatibleVersions {
		if version == compatVersion {
			return true
		}
	}
	return false
}

// ============================================================================
//                           协议列表管理
// ============================================================================

// AllSystemProtocols 所有全局系统协议列表
var AllSystemProtocols = []string{
	// 基础设施协议
	ProtocolNodeInfo,
	ProtocolHeartbeat,
	ProtocolPeerDiscovery,
	ProtocolHealthCheck,
	ProtocolStatusSync,

	// 业务协议
	ProtocolBlockSync,
	ProtocolHeaderSync,
	ProtocolStateSync,
	ProtocolKBucketSync,
	ProtocolRangePaginated,
	ProtocolSyncHelloV2,
	ProtocolSyncBlocksV2,
	ProtocolTransactionDirect,
	ProtocolConsensusCoordination,
	ProtocolBlockSubmission,
	ProtocolConsensusHeartbeat,
	ProtocolNetworkQualityReport,
}

// AllSystemTopics 所有全局系统主题列表
var AllSystemTopics = []string{
	TopicSystemAnnouncements,
	TopicNetworkStatus,
	TopicEmergencyBroadcast,
	TopicTransactionAnnounce,
	TopicConsensusResult,
}

// ============================================================================
//                           与各组件特定协议的关系说明
// ============================================================================

// 📋 **架构说明**：
//
// 1. **全局协议** (pkg/constants/protocols)：
//    - 跨组件复用的基础协议
//    - 系统级管理和监控协议
//    - 统一版本管理和兼容性
//
// 2. **组件特定协议** (internal/core/*/integration/network/protocols.go)：
//    - 组件业务专用协议
//    - 如：共识的区块提交协议、区块链的交易传播协议
//    - 只在组件内部使用，不跨组件复用
//
// 3. **使用原则**：
//    - 跨组件需要 → 使用全局协议
//    - 组件内部业务 → 使用组件特定协议
//    - 优先复用全局协议，避免重复定义
//
// 4. **迁移策略**：
//    - 现有组件特定协议逐步评估
//    - 如有跨组件复用需求，迁移到全局定义
//    - 保持向后兼容，不破坏现有功能
