package types

import (
	"fmt"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// EventType 事件类型
type EventType string

// ProtocolType 协议类型（解耦 pbnetwork，使用字符串或自定义枚举）
type ProtocolType string

// WESEvent WES事件结构（从 interfaces 迁移）
type WESEvent struct {
	// 基础事件信息
	ID        string    `json:"id"`
	EventType EventType `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Source    peer.ID   `json:"source"`

	// 网络消息内容（去耦 pbnetwork）
	Envelope []byte       `json:"envelope,omitempty"`
	Protocol ProtocolType `json:"protocol"`

	// 事件上下文
	Context  map[string]interface{} `json:"context"`
	Metadata map[string]string      `json:"metadata"`

	// 事件优先级和处理要求
	Priority Priority      `json:"priority"`
	Async    bool          `json:"async"`
	TTL      time.Duration `json:"ttl,omitempty"`
}

// WESEventHandler WES事件处理器（供接口侧引用）
type WESEventHandler func(event *WESEvent) error

// Type 实现Event接口 - 返回事件类型
func (e *WESEvent) Type() EventType {
	return e.EventType
}

// Data 实现Event接口 - 返回事件数据
func (e *WESEvent) Data() interface{} {
	return map[string]interface{}{
		"id":        e.ID,
		"timestamp": e.Timestamp,
		"source":    e.Source,
		"envelope":  e.Envelope,
		"protocol":  e.Protocol,
		"context":   e.Context,
		"metadata":  e.Metadata,
		"priority":  e.Priority,
		"async":     e.Async,
		"ttl":       e.TTL,
	}
}

// === 与 pb/network/envelope.proto 对齐的常量与校验 ===

// ContentType 常见内容类型前缀
const (
	ContentTypeProto = "application/pb"
	ContentTypeJSON  = "application/json"
	ContentTypeRaw   = "application/octet-stream"
)

// Priority 优先级常量
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// String 实现 fmt.Stringer 接口
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// SubscriptionID 订阅ID
type SubscriptionID string

// EventFilter 事件过滤器函数类型
type EventFilter func(event *WESEvent) bool

// ============================================================================
//                        基础设施层类型定义
// ============================================================================

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	ID            SubscriptionID         `json:"id"`                       // 订阅ID
	EventType     EventType              `json:"event_type"`               // 事件类型
	Handler       interface{}            `json:"-"`                        // 处理函数（不序列化）
	CreatedAt     time.Time              `json:"created_at"`               // 创建时间
	LastTriggered *time.Time             `json:"last_triggered,omitempty"` // 最后触发时间
	TriggerCount  uint64                 `json:"trigger_count"`            // 触发计数
	IsActive      bool                   `json:"is_active"`                // 是否激活
	Protocols     []string               `json:"protocols,omitempty"`      // 支持的协议
	Metadata      map[string]interface{} `json:"metadata,omitempty"`       // 元数据
}

// FilterInfo 过滤器信息
type FilterInfo struct {
	ID          string      `json:"id"`          // 过滤器ID
	Name        string      `json:"name"`        // 过滤器名称
	Description string      `json:"description"` // 描述
	Filter      EventFilter `json:"-"`           // 过滤函数（不序列化）
	CreatedAt   time.Time   `json:"created_at"`  // 创建时间
	Active      bool        `json:"active"`      // 是否激活
}

// InterceptorInfo 拦截器信息
type InterceptorInfo struct {
	ID          string                 `json:"id"`                 // 拦截器ID
	Name        string                 `json:"name"`               // 拦截器名称
	Description string                 `json:"description"`        // 描述
	Order       int                    `json:"order"`              // 执行顺序
	CreatedAt   time.Time              `json:"created_at"`         // 创建时间
	Active      bool                   `json:"active"`             // 是否激活
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// EventBusConfig EventBus配置
type EventBusConfig struct {
	// 基本配置
	MaxSubscriptions int           `json:"max_subscriptions"`  // 最大订阅数
	MaxPendingEvents int           `json:"max_pending_events"` // 最大待处理事件数
	EventTimeout     time.Duration `json:"event_timeout"`      // 事件处理超时
	BufferSize       int           `json:"buffer_size"`        // 缓冲区大小

	// 性能优化
	WorkerPoolSize int           `json:"worker_pool_size"` // 工作池大小
	BatchSize      int           `json:"batch_size"`       // 批处理大小
	FlushInterval  time.Duration `json:"flush_interval"`   // 刷新间隔

	// 错误处理
	MaxRetries       int           `json:"max_retries"`        // 最大重试次数
	RetryInterval    time.Duration `json:"retry_interval"`     // 重试间隔
	EnableDeadLetter bool          `json:"enable_dead_letter"` // 启用死信队列

	// 监控和日志
	EnableMetrics bool   `json:"enable_metrics"` // 启用指标
	EnableTracing bool   `json:"enable_tracing"` // 启用追踪
	LogLevel      string `json:"log_level"`      // 日志级别

	// 扩展字段 - 与EventBus实现保持兼容
	MaxEventHistory     int           `json:"max_event_history"`     // 最大事件历史记录
	DefaultAsync        bool          `json:"default_async"`         // 默认异步处理
	MetricsInterval     time.Duration `json:"metrics_interval"`      // 指标收集间隔
	MaxConcurrentEvents int           `json:"max_concurrent_events"` // 最大并发事件数
	EventQueueSize      int           `json:"event_queue_size"`      // 事件队列大小
	ProcessingTimeout   time.Duration `json:"processing_timeout"`    // 处理超时时间
	EnableFiltering     bool          `json:"enable_filtering"`      // 启用过滤
	EnableInterception  bool          `json:"enable_interception"`   // 启用拦截
	EnablePersistence   bool          `json:"enable_persistence"`    // 启用持久化
	RequireAuth         bool          `json:"require_auth"`          // 需要认证
	MaxEventSize        int           `json:"max_event_size"`        // 最大事件大小
	RateLimit           int           `json:"rate_limit"`            // 速率限制
	EnableAudit         bool          `json:"enable_audit"`          // 启用审计

	// 扩展配置
	Metadata map[string]interface{} `json:"metadata,omitempty"` // 扩展元数据
}

// ValidateContentType 验证内容类型格式
func ValidateContentType(contentType string) error {
	if contentType == "" {
		return fmt.Errorf("content type cannot be empty")
	}

	if !strings.Contains(contentType, "/") {
		return fmt.Errorf("invalid content type format: %s", contentType)
	}

	parts := strings.Split(contentType, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid content type format: %s", contentType)
	}

	return nil
}

// ValidateEventType 验证事件类型格式
func ValidateEventType(eventType string) error {
	if eventType == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	parts := strings.Split(eventType, ".")
	if len(parts) < 2 {
		return fmt.Errorf("event type must follow domain.category format: %s", eventType)
	}

	return nil
}

// ValidateProtocolType 验证协议类型格式
func ValidateProtocolType(protocolType string) error {
	if protocolType == "" {
		return fmt.Errorf("protocol type cannot be empty")
	}

	// 协议类型通常以 "/" 开头，例如 "/myapp/protocol/1.0.0"
	if !strings.HasPrefix(protocolType, "/") {
		return fmt.Errorf("protocol type should start with '/': %s", protocolType)
	}

	return nil
}

// ============================================================================
//                        核心事件数据结构定义
// ============================================================================

// ==================== 区块链分叉事件数据结构 ====================

// ForkDetectedEventData 分叉检测事件数据
type ForkDetectedEventData struct {
	// 分叉基础信息
	Height         uint64 `json:"height"`           // 分叉发生的块高度
	LocalBlockHash string `json:"local_block_hash"` // 本地链上该高度的区块哈希
	ForkBlockHash  string `json:"fork_block_hash"`  // 分叉链上该高度的区块哈希

	// 分叉详情
	DetectedAt   int64  `json:"detected_at"`   // 检测时间戳
	Source       string `json:"source"`        // 检测来源（sync/validation/peer）
	ConflictType string `json:"conflict_type"` // 冲突类型（block_hash/parent_hash等）

	// 网络状态
	PeerID         string `json:"peer_id"`         // 报告分叉的节点ID
	LocalHeight    uint64 `json:"local_height"`    // 本地链高度
	ReportedHeight uint64 `json:"reported_height"` // 报告的链高度

	// 影响评估
	AffectedTxCount int `json:"affected_tx_count"` // 受影响的交易数量

	// === 兼容consensus事件处理器的字段 ===
	ForkHeight uint64 `json:"fork_height"` // 分叉高度（与Height相同）
	ForkType   string `json:"fork_type"`   // 分叉类型（与ConflictType相同）
	Message    string `json:"message"`     // 分叉消息描述
}

// ForkProcessingEventData 分叉处理中事件数据
type ForkProcessingEventData struct {
	// 处理状态
	ProcessID string `json:"process_id"` // 处理进程ID
	Status    string `json:"status"`     // 处理状态（started/syncing/validating/resolving）
	StartedAt int64  `json:"started_at"` // 开始时间戳
	Progress  int    `json:"progress"`   // 处理进度（0-100）

	// 分叉信息
	Height     uint64 `json:"height"`      // 分叉高度
	LocalHash  string `json:"local_hash"`  // 本地哈希
	TargetHash string `json:"target_hash"` // 目标哈希

	// 处理细节
	SyncBlocks     int    `json:"sync_blocks"`     // 需要同步的块数
	ValidatedCount int    `json:"validated_count"` // 已验证的块数
	CurrentBlock   uint64 `json:"current_block"`   // 当前处理的块高度

	// 资源消耗
	MemoryUsage int64 `json:"memory_usage"` // 内存使用量（字节）
	DiskIO      int64 `json:"disk_io"`      // 磁盘IO量（字节）

	// === 兼容consensus事件处理器的字段 ===
	ProcessStage string `json:"process_stage"` // 处理阶段（与Status相同）
	Message      string `json:"message"`       // 处理消息描述
}

// ForkCompletedEventData 分叉处理完成事件数据
type ForkCompletedEventData struct {
	// 处理结果
	ProcessID   string `json:"process_id"`   // 处理进程ID
	Resolution  string `json:"resolution"`   // 处理结果（local_kept/remote_adopted/merged）
	CompletedAt int64  `json:"completed_at"` // 完成时间戳
	Duration    int64  `json:"duration"`     // 处理耗时（毫秒）

	// 最终状态
	FinalHeight uint64 `json:"final_height"` // 最终链高度
	FinalHash   string `json:"final_hash"`   // 最终块哈希

	// 影响统计
	RevertedBlocks  int `json:"reverted_blocks"`   // 回滚的块数
	AppliedBlocks   int `json:"applied_blocks"`    // 应用的新块数
	AffectedTxCount int `json:"affected_tx_count"` // 受影响的交易总数
	InvalidatedTxs  int `json:"invalidated_txs"`   // 失效的交易数

	// 性能统计
	PeakMemoryUsage int64 `json:"peak_memory_usage"` // 峰值内存使用量
	TotalDiskIO     int64 `json:"total_disk_io"`     // 总磁盘IO量
	DbOperations    int   `json:"db_operations"`     // 数据库操作次数

	// === 兼容consensus事件处理器的字段 ===
	ProcessingTime int64  `json:"processing_time"` // 处理时间（与Duration相同）
	Success        bool   `json:"success"`         // 处理是否成功
	ChainSwitched  bool   `json:"chain_switched"`  // 是否切换链
	Error          string `json:"error"`           // 错误信息（如有）
}

// ==================== 区块链状态事件数据结构 ====================

// ChainReorganizedEventData 链重组事件数据
type ChainReorganizedEventData struct {
	// 重组基础信息
	TriggerHeight  uint64 `json:"trigger_height"`  // 触发重组的高度
	OldChainTip    string `json:"old_chain_tip"`   // 旧链顶端块哈希
	NewChainTip    string `json:"new_chain_tip"`   // 新链顶端块哈希
	CommonAncestor string `json:"common_ancestor"` // 共同祖先块哈希

	// 重组规模
	RevertedBlocks []string `json:"reverted_blocks"` // 被回滚的块哈希列表
	AppliedBlocks  []string `json:"applied_blocks"`  // 新应用的块哈希列表
	DepthReverted  int      `json:"depth_reverted"`  // 回滚深度
	DepthApplied   int      `json:"depth_applied"`   // 应用深度

	// 交易影响
	RevertedTxHashes []string `json:"reverted_tx_hashes"` // 被回滚的交易哈希
	ReappliedTxCount int      `json:"reapplied_tx_count"` // 重新应用的交易数
	LostTxCount      int      `json:"lost_tx_count"`      // 丢失的交易数（未能重新应用）

	// 状态影响
	StateRootOld   string `json:"state_root_old"`   // 旧状态根
	StateRootNew   string `json:"state_root_new"`   // 新状态根
	UTXOSetChanged bool   `json:"utxo_set_changed"` // UTXO集合是否改变

	// 时间和性能
	StartedAt      int64 `json:"started_at"`      // 重组开始时间
	CompletedAt    int64 `json:"completed_at"`    // 重组完成时间
	ProcessingTime int64 `json:"processing_time"` // 处理耗时（毫秒）

	// === 兼容consensus事件处理器的字段 ===
	OldHeight   uint64 `json:"old_height"`   // 旧链高度
	NewHeight   uint64 `json:"new_height"`   // 新链高度
	ReorgLength int    `json:"reorg_length"` // 重组长度（与DepthReverted相同）
}

// NetworkQualityChangedEventData 网络质量变化事件数据
type NetworkQualityChangedEventData struct {
	// 质量指标
	Quality    string  `json:"quality"`     // 网络质量等级（excellent/good/poor/critical）
	Latency    int64   `json:"latency"`     // 平均延迟（毫秒）
	Bandwidth  int64   `json:"bandwidth"`   // 可用带宽（bps）
	PacketLoss float64 `json:"packet_loss"` // 丢包率（0-1）
	Jitter     int64   `json:"jitter"`      // 抖动（毫秒）

	// 连接信息
	ConnectedPeers int `json:"connected_peers"` // 连接的节点数
	ActivePeers    int `json:"active_peers"`    // 活跃节点数
	ReliablePeers  int `json:"reliable_peers"`  // 可靠节点数

	// 变化详情
	PreviousQuality string `json:"previous_quality"` // 之前的质量等级
	ChangeReason    string `json:"change_reason"`    // 变化原因
	DetectedAt      int64  `json:"detected_at"`      // 检测时间

	// 网络拓扑
	NetworkPartitions bool     `json:"network_partitions"` // 是否检测到网络分区
	IsolatedNodes     []string `json:"isolated_nodes"`     // 隔离的节点列表

	// === 兼容consensus事件处理器的字段 ===
	ChangeType    string `json:"change_type"`    // 变化类型（与ChangeReason相同）
	PeerCount     int    `json:"peer_count"`     // 节点计数（与ConnectedPeers相同）
	NetworkHealth string `json:"network_health"` // 网络健康度（与Quality相同）
}

// ==================== 系统资源和状态事件数据结构 ====================

// SystemStoppingEventData 系统停止事件数据
type SystemStoppingEventData struct {
	Reason      string `json:"reason"`       // 停止原因
	Graceful    bool   `json:"graceful"`     // 是否优雅停止
	Timeout     int64  `json:"timeout"`      // 停止超时时间（毫秒）
	InitiatedAt int64  `json:"initiated_at"` // 开始停止时间
}

// ResourceExhaustedEventData 资源耗尽事件数据
type ResourceExhaustedEventData struct {
	ResourceType string  `json:"resource_type"` // 资源类型（memory/cpu/disk/network）
	CurrentUsage int64   `json:"current_usage"` // 当前使用量
	MaxLimit     int64   `json:"max_limit"`     // 最大限制
	UsagePercent float64 `json:"usage_percent"` // 使用百分比
	DetectedAt   int64   `json:"detected_at"`   // 检测时间
}

// MemoryPressureEventData 内存压力事件数据
type MemoryPressureEventData struct {
	UsedMemory    int64  `json:"used_memory"`    // 已使用内存（字节）
	TotalMemory   int64  `json:"total_memory"`   // 总内存（字节）
	FreeMemory    int64  `json:"free_memory"`    // 空闲内存（字节）
	PressureLevel string `json:"pressure_level"` // 压力等级（low/medium/high/critical）
	DetectedAt    int64  `json:"detected_at"`    // 检测时间

	// === 兼容mempool事件处理器的字段 ===
	Threshold float64 `json:"threshold"` // 压力阈值
}

// StorageSpaceLowEventData 存储空间不足事件数据
type StorageSpaceLowEventData struct {
	Path         string  `json:"path"`          // 存储路径
	UsedSpace    int64   `json:"used_space"`    // 已使用空间（字节）
	TotalSpace   int64   `json:"total_space"`   // 总空间（字节）
	FreeSpace    int64   `json:"free_space"`    // 空闲空间（字节）
	UsagePercent float64 `json:"usage_percent"` // 使用百分比
	StorageType  string  `json:"storage_type"`  // 存储类型
	DetectedAt   int64   `json:"detected_at"`   // 检测时间

	// === 兼容mempool事件处理器的字段 ===
	AvailableSpace int64 `json:"available_space"` // 可用空间（与FreeSpace相同）
}

// ==================== 区块和交易事件数据结构 ====================

// BlockProcessedEventData 区块处理完成事件数据
type BlockProcessedEventData struct {
	Height           uint64                 `json:"height"`             // 区块高度
	Hash             string                 `json:"hash"`               // 区块哈希
	ParentHash       string                 `json:"parent_hash"`        // 父区块哈希
	StateRoot        string                 `json:"state_root"`         // 状态根
	TxCount          int                    `json:"tx_count"`           // 交易数量
	ProcessTime      int64                  `json:"process_time"`       // 处理时间（毫秒）
	ExecutionFeeUsed uint64                 `json:"execution_fee_used"` // 使用的执行费用
	Timestamp        int64                  `json:"timestamp"`          // 区块时间戳
	Validator        string                 `json:"validator"`          // 验证者
	Size             int64                  `json:"size"`               // 区块大小（字节）
	Details          map[string]interface{} `json:"details"`            // 其他详情

	// === 兼容mempool事件处理器的字段 ===
	TransactionCount int `json:"transaction_count"` // 交易数量（与TxCount相同）
}

// BlockProducedEventData 区块生产事件数据
type BlockProducedEventData struct {
	Height    uint64                 `json:"height"`    // 区块高度
	Hash      string                 `json:"hash"`      // 区块哈希
	Producer  string                 `json:"producer"`  // 生产者
	TxCount   int                    `json:"tx_count"`  // 包含交易数
	Timestamp int64                  `json:"timestamp"` // 生产时间戳
	Size      int64                  `json:"size"`      // 区块大小
	Details   map[string]interface{} `json:"details"`   // 其他详情
}

// ChainHeightChangedEventData 链高度变化事件数据
type ChainHeightChangedEventData struct {
	OldHeight uint64 `json:"old_height"` // 旧高度
	NewHeight uint64 `json:"new_height"` // 新高度
	BlockHash string `json:"block_hash"` // 新块哈希
	Timestamp int64  `json:"timestamp"`  // 变化时间戳
}

// ChainStateUpdatedEventData 链状态更新事件数据
type ChainStateUpdatedEventData struct {
	Height       uint64 `json:"height"`         // 当前高度
	StateRoot    string `json:"state_root"`     // 状态根
	TotalTxCount uint64 `json:"total_tx_count"` // 总交易数
	Timestamp    int64  `json:"timestamp"`      // 更新时间戳
}

// BlockValidatedEventData 区块验证完成事件数据
type BlockValidatedEventData struct {
	Height    uint64   `json:"height"`    // 区块高度
	Hash      string   `json:"hash"`      // 区块哈希
	Valid     bool     `json:"valid"`     // 是否有效
	Errors    []string `json:"errors"`    // 验证错误（如有）
	Timestamp int64    `json:"timestamp"` // 验证时间戳
}

// BlockConfirmedEventData 区块确认事件数据
type BlockConfirmedEventData struct {
	Height        uint64 `json:"height"`        // 区块高度
	Hash          string `json:"hash"`          // 区块哈希
	Confirmations int    `json:"confirmations"` // 确认数
	Timestamp     int64  `json:"timestamp"`     // 确认时间戳
}

// BlockRevertedEventData 区块回滚事件数据
type BlockRevertedEventData struct {
	Height    uint64 `json:"height"`    // 回滚的区块高度
	Hash      string `json:"hash"`      // 回滚的区块哈希
	Reason    string `json:"reason"`    // 回滚原因
	Timestamp int64  `json:"timestamp"` // 回滚时间戳
}

// BlockFinalizedEventData 区块最终确认事件数据
type BlockFinalizedEventData struct {
	Height    uint64 `json:"height"`    // 区块高度
	Hash      string `json:"hash"`      // 区块哈希
	Timestamp int64  `json:"timestamp"` // 最终确认时间戳
}

// ==================== 同步事件数据结构 ====================

// SyncStartedEventData 同步开始事件数据
type SyncStartedEventData struct {
	StartHeight  uint64 `json:"start_height"`  // 开始同步的高度
	TargetHeight uint64 `json:"target_height"` // 目标高度
	Timestamp    int64  `json:"timestamp"`     // 开始时间戳
}

// SyncProgressEventData 同步进度事件数据
type SyncProgressEventData struct {
	CurrentHeight uint64  `json:"current_height"` // 当前高度
	TargetHeight  uint64  `json:"target_height"`  // 目标高度
	Progress      float64 `json:"progress"`       // 进度百分比
	BlocksPerSec  float64 `json:"blocks_per_sec"` // 同步速度（块/秒）
	Timestamp     int64   `json:"timestamp"`      // 进度时间戳
}

// SyncCompletedEventData 同步完成事件数据
type SyncCompletedEventData struct {
	StartHeight  uint64 `json:"start_height"`  // 开始高度
	FinalHeight  uint64 `json:"final_height"`  // 最终高度
	BlocksSynced uint64 `json:"blocks_synced"` // 同步的块数
	Duration     int64  `json:"duration"`      // 耗时（毫秒）
	Timestamp    int64  `json:"timestamp"`     // 完成时间戳
}

// SyncFailedEventData 同步失败事件数据
type SyncFailedEventData struct {
	CurrentHeight uint64 `json:"current_height"` // 失败时的高度
	TargetHeight  uint64 `json:"target_height"`  // 目标高度
	Error         string `json:"error"`          // 错误信息
	Timestamp     int64  `json:"timestamp"`      // 失败时间戳
}

// ==================== 交易事件数据结构 ====================

// TransactionReceivedEventData 交易接收事件数据
type TransactionReceivedEventData struct {
	Hash      string `json:"hash"`      // 交易哈希
	From      string `json:"from"`      // 发送者
	To        string `json:"to"`        // 接收者
	Value     uint64 `json:"value"`     // 金额
	Fee       uint64 `json:"fee"`       // 手续费
	Timestamp int64  `json:"timestamp"` // 接收时间戳
}

// TransactionValidatedEventData 交易验证完成事件数据
type TransactionValidatedEventData struct {
	Hash      string   `json:"hash"`      // 交易哈希
	Valid     bool     `json:"valid"`     // 是否有效
	Errors    []string `json:"errors"`    // 验证错误（如有）
	Timestamp int64    `json:"timestamp"` // 验证时间戳
}

// TransactionExecutedEventData 交易执行完成事件数据
type TransactionExecutedEventData struct {
	Hash             string `json:"hash"`               // 交易哈希
	BlockHeight      uint64 `json:"block_height"`       // 所在区块高度
	ExecutionFeeUsed uint64 `json:"execution_fee_used"` // 使用的执行费用
	Success          bool   `json:"success"`            // 是否执行成功
	Result           string `json:"result"`             // 执行结果
	Timestamp        int64  `json:"timestamp"`          // 执行时间戳
}

// TransactionFailedEventData 交易执行失败事件数据
type TransactionFailedEventData struct {
	Hash             string `json:"hash"`               // 交易哈希
	BlockHeight      uint64 `json:"block_height"`       // 失败的区块高度
	Error            string `json:"error"`              // 失败原因
	ExecutionFeeUsed uint64 `json:"execution_fee_used"` // 已消耗的执行费用
	Timestamp        int64  `json:"timestamp"`          // 失败时间戳

	// === 兼容mempool事件处理器的字段 ===
	Reason      string      `json:"reason"`      // 失败原因（与Error相同）
	Transaction interface{} `json:"transaction"` // 交易详情
}

// TransactionConfirmedEventData 交易确认事件数据
type TransactionConfirmedEventData struct {
	Hash          string `json:"hash"`          // 交易哈希
	BlockHeight   uint64 `json:"block_height"`  // 所在区块高度
	BlockHash     string `json:"block_hash"`    // 所在区块哈希
	Confirmations int    `json:"confirmations"` // 确认数
	Final         bool   `json:"final"`         // 是否最终确认
	Timestamp     int64  `json:"timestamp"`     // 确认时间戳
}

// ==================== 网络事件数据结构 ====================

// NetworkPartitionedEventData 网络分区事件数据
type NetworkPartitionedEventData struct {
	PartitionID   string   `json:"partition_id"`   // 分区ID
	AffectedPeers []string `json:"affected_peers"` // 受影响的节点
	DetectedAt    int64    `json:"detected_at"`    // 检测时间
	Severity      string   `json:"severity"`       // 严重程度
}

// NetworkRecoveredEventData 网络恢复事件数据
type NetworkRecoveredEventData struct {
	PartitionID    string   `json:"partition_id"`    // 恢复的分区ID
	RecoveredPeers []string `json:"recovered_peers"` // 恢复的节点
	RecoveredAt    int64    `json:"recovered_at"`    // 恢复时间
	Duration       int64    `json:"duration"`        // 分区持续时间
}

// ==================== 共识事件数据结构 ====================

// ConsensusResultEventData 共识结果事件数据
type ConsensusResultEventData struct {
	Round        uint64 `json:"round"`        // 共识轮次
	Result       string `json:"result"`       // 共识结果
	BlockHash    string `json:"block_hash"`   // 达成共识的区块哈希
	Participants int    `json:"participants"` // 参与者数量
	Timestamp    int64  `json:"timestamp"`    // 共识时间戳
}

// ConsensusStateChangedEventData 共识状态变化事件数据
type ConsensusStateChangedEventData struct {
	OldState  string                 `json:"old_state"`  // 旧状态
	NewState  string                 `json:"new_state"`  // 新状态
	Round     uint64                 `json:"round"`      // 当前轮次
	StateData map[string]interface{} `json:"state_data"` // 状态数据
	Timestamp int64                  `json:"timestamp"`  // 状态变化时间戳
}

// ==================== 内存池事件数据结构 ====================

// TransactionRemovedEventData 交易移除事件数据（mempool相关）
type TransactionRemovedEventData struct {
	Hash      string `json:"hash"`      // 交易哈希
	Reason    string `json:"reason"`    // 移除原因（expired/included/invalid/replaced）
	Pool      string `json:"pool"`      // 所在池（tx_pool/candidate_pool）
	Timestamp int64  `json:"timestamp"` // 移除时间戳
}

// UTXOStateChangedEventData UTXO状态变化事件数据
type UTXOStateChangedEventData struct {
	UTXOHash    string `json:"utxo_hash"`    // UTXO哈希
	Operation   string `json:"operation"`    // 操作类型（created/spent/locked/unlocked）
	TxHash      string `json:"tx_hash"`      // 相关交易哈希
	BlockHeight uint64 `json:"block_height"` // 区块高度
	Timestamp   int64  `json:"timestamp"`    // 状态变化时间戳
}
