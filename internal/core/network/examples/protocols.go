// Package examples provides protocol constant definitions and naming conventions examples.
package examples

// protocols.go
// 领域协议常量定义与命名规范示例
// 用途：为区块同步、交易传播等核心业务场景提供协议ID与Topic命名模板

// ==================== 协议ID常量（流式） ====================

const (
	// ProtocolBlockSync 区块同步协议
	ProtocolBlockSync = "/weisyn/block/sync/v1.0.0"
	// ProtocolBlockRequest 区块请求协议
	ProtocolBlockRequest = "/weisyn/block/request/v1.0.0"
	// ProtocolBlockHeaderSync 区块头同步协议
	ProtocolBlockHeaderSync = "/weisyn/block/header/v1.0.0"
	// ProtocolStateSync 状态同步协议
	ProtocolStateSync = "/weisyn/state/sync/v1.0.0"

	// ProtocolTxRequest 交易请求协议
	ProtocolTxRequest = "/weisyn/tx/request/v1.0.0"
	// ProtocolTxBatch 批量交易传输协议
	ProtocolTxBatch = "/weisyn/tx/batch/v1.0.0"
	// ProtocolMempoolSync 内存池同步协议
	ProtocolMempoolSync = "/weisyn/mempool/sync/v1.0.0"

	// ProtocolConsensusVote 共识投票协议
	ProtocolConsensusVote = "/weisyn/consensus/vote/v1.0.0"
	// ProtocolConsensusProposal 共识提议协议
	ProtocolConsensusProposal = "/weisyn/consensus/proposal/v1.0.0"

	// ProtocolPeerInfo 节点信息交换协议
	ProtocolPeerInfo = "/weisyn/peer/info/v1.0.0"
	// ProtocolPing 心跳检测协议
	ProtocolPing = "/weisyn/peer/ping/v1.0.0"
)

// ==================== Topic常量（订阅） ====================

const (
	// TopicNewBlock 新区块公告主题
	TopicNewBlock = "weisyn.block.announce.v1"
	// TopicBlockFinalized 区块确认主题
	TopicBlockFinalized = "weisyn.block.finalized.v1"
	// TopicChainReorg 链重组通知主题
	TopicChainReorg = "weisyn.chain.reorg.v1"

	// TopicNewTransaction 新交易公告主题
	TopicNewTransaction = "weisyn.tx.announce.v1"
	// TopicTxPool 交易池状态主题
	TopicTxPool = "weisyn.tx.pool.v1"
	// TopicTxInvalid 无效交易通知主题
	TopicTxInvalid = "weisyn.tx.invalid.v1"

	// TopicConsensusRound 共识轮次主题
	TopicConsensusRound = "weisyn.consensus.round.v1"
	// TopicConsensusCommit 共识提交主题
	TopicConsensusCommit = "weisyn.consensus.commit.v1"

	// TopicPeerJoin 节点加入主题
	TopicPeerJoin = "weisyn.peer.join.v1"
	// TopicPeerLeave 节点离开主题
	TopicPeerLeave = "weisyn.peer.leave.v1"
	// TopicNetworkAlert 网络警报主题
	TopicNetworkAlert = "weisyn.network.alert.v1"
)

// ==================== 消息大小限制 ====================

const (
	// MaxBlockRequestSize 单个区块请求的最大大小（4MB）
	MaxBlockRequestSize = 4 * 1024 * 1024
	// MaxBlockResponseSize 区块响应的最大大小（16MB，含完整区块数据）
	MaxBlockResponseSize = 16 * 1024 * 1024
	// MaxTxBatchSize 批量交易的最大大小（8MB）
	MaxTxBatchSize = 8 * 1024 * 1024
	// MaxStateChunkSize 状态同步分片的最大大小（32MB）
	MaxStateChunkSize = 32 * 1024 * 1024

	// MaxBlockAnnounceSize 区块公告的最大大小（64KB）
	MaxBlockAnnounceSize = 64 * 1024
	// MaxTxAnnounceSize 交易公告的最大大小（256KB）
	MaxTxAnnounceSize = 256 * 1024
	// MaxConsensusSize 共识消息的最大大小（1MB）
	MaxConsensusSize = 1 * 1024 * 1024
	// MaxPeerInfoSize 节点信息的最大大小（16KB）
	MaxPeerInfoSize = 16 * 1024
)

// ==================== 超时配置 ====================

const (
	// DefaultConnectTimeout 默认连接超时（10秒）
	DefaultConnectTimeout = 10 * 1000

	// BlockSyncReadTimeout 区块同步读取超时（30秒）
	BlockSyncReadTimeout = 30 * 1000
	// BlockSyncWriteTimeout 区块同步写入超时（10秒）
	BlockSyncWriteTimeout = 10 * 1000
	// TxRequestTimeout 交易请求超时（5秒）
	TxRequestTimeout = 5 * 1000
	// PingTimeout 心跳检测超时（3秒）
	PingTimeout = 3 * 1000

	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3
	// DefaultRetryDelay 默认重试延迟（1秒）
	DefaultRetryDelay = 1 * 1000
	// DefaultBackoffFactor 默认退避因子（2倍）
	DefaultBackoffFactor = 2.0
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
