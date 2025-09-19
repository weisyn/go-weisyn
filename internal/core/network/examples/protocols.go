package examples

// protocols.go
// 领域协议常量定义与命名规范示例
// 用途：为区块同步、交易传播等核心业务场景提供协议ID与Topic命名模板

// ==================== 协议ID常量（流式） ====================

const (
	// 区块同步相关协议
	ProtocolBlockSync       = "/weisyn/block/sync/v1.0.0"    // 区块范围同步
	ProtocolBlockRequest    = "/weisyn/block/request/v1.0.0" // 单个区块请求
	ProtocolBlockHeaderSync = "/weisyn/block/header/v1.0.0"  // 区块头同步
	ProtocolStateSync       = "/weisyn/state/sync/v1.0.0"    // 状态同步

	// 交易相关协议
	ProtocolTxRequest   = "/weisyn/tx/request/v1.0.0"   // 交易详情请求
	ProtocolTxBatch     = "/weisyn/tx/batch/v1.0.0"     // 批量交易传输
	ProtocolMempoolSync = "/weisyn/mempool/sync/v1.0.0" // 内存池同步

	// 共识相关协议
	ProtocolConsensusVote     = "/weisyn/consensus/vote/v1.0.0"     // 共识投票
	ProtocolConsensusProposal = "/weisyn/consensus/proposal/v1.0.0" // 共识提议

	// 发现与健康检查
	ProtocolPeerInfo = "/weisyn/peer/info/v1.0.0" // 节点信息交换
	ProtocolPing     = "/weisyn/peer/ping/v1.0.0" // 心跳检测
)

// ==================== Topic常量（订阅） ====================

const (
	// 区块广播
	TopicNewBlock       = "weisyn.block.announce.v1"  // 新区块公告
	TopicBlockFinalized = "weisyn.block.finalized.v1" // 区块确认
	TopicChainReorg     = "weisyn.chain.reorg.v1"     // 链重组通知

	// 交易广播
	TopicNewTransaction = "weisyn.tx.announce.v1" // 新交易公告
	TopicTxPool         = "weisyn.tx.pool.v1"     // 交易池状态
	TopicTxInvalid      = "weisyn.tx.invalid.v1"  // 无效交易通知

	// 共识广播
	TopicConsensusRound  = "weisyn.consensus.round.v1"  // 共识轮次
	TopicConsensusCommit = "weisyn.consensus.commit.v1" // 共识提交

	// 网络状态
	TopicPeerJoin     = "weisyn.peer.join.v1"     // 节点加入
	TopicPeerLeave    = "weisyn.peer.leave.v1"    // 节点离开
	TopicNetworkAlert = "weisyn.network.alert.v1" // 网络警报
)

// ==================== 消息大小限制 ====================

const (
	// 流式协议消息限制
	MaxBlockRequestSize  = 4 * 1024 * 1024  // 4MB：单个区块请求
	MaxBlockResponseSize = 16 * 1024 * 1024 // 16MB：区块响应（含完整区块数据）
	MaxTxBatchSize       = 8 * 1024 * 1024  // 8MB：批量交易
	MaxStateChunkSize    = 32 * 1024 * 1024 // 32MB：状态同步分片

	// 订阅协议消息限制
	MaxBlockAnnounceSize = 64 * 1024       // 64KB：区块公告
	MaxTxAnnounceSize    = 256 * 1024      // 256KB：交易公告
	MaxConsensusSize     = 1 * 1024 * 1024 // 1MB：共识消息
	MaxPeerInfoSize      = 16 * 1024       // 16KB：节点信息
)

// ==================== 超时配置 ====================

const (
	// 连接超时
	DefaultConnectTimeout = 10 * 1000 // 10秒

	// 读写超时
	BlockSyncReadTimeout  = 30 * 1000 // 30秒：区块同步读取
	BlockSyncWriteTimeout = 10 * 1000 // 10秒：区块同步写入
	TxRequestTimeout      = 5 * 1000  // 5秒：交易请求
	PingTimeout           = 3 * 1000  // 3秒：心跳检测

	// 重试配置
	DefaultMaxRetries    = 3        // 默认最大重试次数
	DefaultRetryDelay    = 1 * 1000 // 1秒：默认重试延迟
	DefaultBackoffFactor = 2.0      // 2倍：退避因子
)

// ==================== 协议版本管理 ====================

// SupportedVersions 返回支持的协议版本集合
func SupportedVersions() map[string][]string {
	return map[string][]string{
		"weisyn/block":     {"v1.0.0"},
		"weisyn/tx":        {"v1.0.0"},
		"weisyn/consensus": {"v1.0.0"},
		"weisyn/peer":      {"v1.0.0"},
		"weisyn/state":     {"v1.0.0"},
	}
}

// GetLatestVersion 获取指定协议族的最新版本
func GetLatestVersion(family string) string {
	versions := SupportedVersions()[family]
	if len(versions) == 0 {
		return "v1.0.0"
	}
	return versions[len(versions)-1]
}
