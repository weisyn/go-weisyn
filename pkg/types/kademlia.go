// Package types provides Kademlia type definitions.
package types

import (
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// ================== 基础信息结构 ==================

// PeerInfo 节点信息
type PeerInfo struct {
	ID                string        `json:"id"`                 // 节点ID
	Address           string        `json:"address"`            // 节点地址
	LastSeen          Timestamp     `json:"last_seen"`          // 最后见过时间
	LastUsefulAt      Timestamp     `json:"last_useful_at"`     // 最后有用时间
	AddedAt           Timestamp     `json:"added_at"`           // 添加时间
	ConnectionLatency time.Duration `json:"connection_latency"` // 连接延迟
	IsReplaceable     bool          `json:"is_replaceable"`     // 是否可替换
	DHTId             []byte        `json:"dht_id"`             // DHT ID
	Mode              int           `json:"mode"`               // 节点模式
}

// RoutingTable 路由表
type RoutingTable struct {
	LocalID    string    `json:"local_id"`    // 本地节点ID
	Buckets    []*Bucket `json:"buckets"`     // 桶列表
	BucketSize int       `json:"bucket_size"` // 桶大小
	TableSize  int       `json:"table_size"`  // 表大小
	UpdatedAt  Timestamp `json:"updated_at"`  // 更新时间
}

// Bucket K桶
type Bucket struct {
	Index     int         `json:"index"`      // 桶索引
	Peers     []*PeerInfo `json:"peers"`      // 节点列表
	Capacity  int         `json:"capacity"`   // 容量
	UpdatedAt Timestamp   `json:"updated_at"` // 更新时间
}

// PeerGroupInfo 节点组信息
type PeerGroupInfo struct {
	GroupID  string `json:"group_id"` // 组ID
	PeerID   string `json:"peer_id"`  // 节点ID
	IPGroup  string `json:"ip_group"` // IP组
	Region   string `json:"region"`   // 地区
	Provider string `json:"provider"` // 提供商
}

// SelectionCriteria 选择标准
type SelectionCriteria struct {
	TargetKey        []byte        `json:"target_key"`        // 目标密钥
	ExcludePeers     []peer.ID     `json:"exclude_peers"`     // 排除的节点
	MaxLatency       time.Duration `json:"max_latency"`       // 最大延迟
	RequireConnected bool          `json:"require_connected"` // 是否要求已连接
	PreferClose      bool          `json:"prefer_close"`      // 是否偏好近距离
	MinScore         float64       `json:"min_score"`         // 最小评分
	PreferredRegions []string      `json:"preferred_regions"` // 偏好地区
}

// ================== Manager模式统一类型 ==================

// AddPeerRequest 添加节点请求
type AddPeerRequest struct {
	PeerInfo  *PeerInfo `json:"peer_info"`  // 节点信息
	Force     bool      `json:"force"`      // 是否强制添加
	BucketIdx int       `json:"bucket_idx"` // 指定桶索引（-1表示自动选择）
}

// UpdatePeerRequest 更新节点请求
type UpdatePeerRequest struct {
	PeerID   string                 `json:"peer_id"`  // 节点ID
	Updates  map[string]interface{} `json:"updates"`  // 更新字段
	Metadata map[string]string      `json:"metadata"` // 元数据更新
}

// ListPeersRequest 列出节点请求
type ListPeersRequest struct {
	BucketIdx  int               `json:"bucket_idx"`  // 桶索引（-1表示所有桶）
	Status     string            `json:"status"`      // 状态过滤
	MaxResults int               `json:"max_results"` // 最大结果数
	Offset     int               `json:"offset"`      // 偏移量
	SortBy     string            `json:"sort_by"`     // 排序字段
	Filters    map[string]string `json:"filters"`     // 过滤条件
}

// ListPeersResponse 列出节点响应
type ListPeersResponse struct {
	Peers      []*PeerInfo `json:"peers"`       // 节点列表
	TotalCount int         `json:"total_count"` // 总数量
	HasMore    bool        `json:"has_more"`    // 是否还有更多
}

// FindPeersRequest 查找节点请求
type FindPeersRequest struct {
	Target       string             `json:"target"`        // 目标ID
	Count        int                `json:"count"`         // 查找数量
	Exclude      []string           `json:"exclude"`       // 排除节点
	Criteria     *SelectionCriteria `json:"criteria"`      // 选择条件
	IncludeStats bool               `json:"include_stats"` // 是否包含统计信息
}

// FindPeersResponse 查找节点响应
type FindPeersResponse struct {
	Peers     []*PeerInfo   `json:"peers"`      // 找到的节点
	Distances []int         `json:"distances"`  // 距离信息
	QueryTime time.Duration `json:"query_time"` // 查询耗时
	Sources   []string      `json:"sources"`    // 数据源
}

// DistanceRequest 距离计算请求
type DistanceRequest struct {
	From   string `json:"from"`   // 起始节点
	To     string `json:"to"`     // 目标节点
	Method string `json:"method"` // 计算方法
}

// DistanceResponse 距离计算响应
type DistanceResponse struct {
	Distance    int           `json:"distance"`    // 距离值
	Calculation string        `json:"calculation"` // 计算详情
	Method      string        `json:"method"`      // 使用的方法
	Duration    time.Duration `json:"duration"`    // 计算耗时
}

// ================== 事件与状态 ==================

// RoutingTableInfo 路由表信息
type RoutingTableInfo struct {
	LocalID         string    `json:"local_id"`          // 本地节点ID
	TotalPeers      int       `json:"total_peers"`       // 总节点数
	ActiveBuckets   int       `json:"active_buckets"`    // 活跃桶数
	TableSize       int       `json:"table_size"`        // 表大小
	LastRefreshTime Timestamp `json:"last_refresh_time"` // 最后刷新时间
	RefreshCount    int64     `json:"refresh_count"`     // 刷新次数
}

// NodeHealthInfo 节点健康信息
type NodeHealthInfo struct {
	NodeID       peer.ID       `json:"node_id"`       // 节点ID
	IsHealthy    bool          `json:"is_healthy"`    // 是否健康
	LastCheck    Timestamp     `json:"last_check"`    // 最后检查时间
	FailureCount int           `json:"failure_count"` // 失败次数
	ResponseTime time.Duration `json:"response_time"` // 响应时间
	Status       string        `json:"status"`        // 状态描述
}

// RoutingTableEvent 路由表事件
type RoutingTableEvent struct {
	Type      string      `json:"type"`      // 事件类型
	Timestamp Timestamp   `json:"timestamp"` // 时间戳
	Data      interface{} `json:"data"`      // 事件数据
	Source    string      `json:"source"`    // 事件源
}

// 路由事件类型常量（使用字符串避免与全局 EventType 冲突）
const (
	EventTypePeerAdded      = "routing.peer.added"
	EventTypePeerRemoved    = "routing.peer.removed"
	EventTypePeerUpdated    = "routing.peer.updated"
	EventTypeBucketRefresh  = "routing.bucket.refresh"
	EventTypeTableOptimized = "routing.table.optimized"
	EventTypeHealthCheck    = "routing.health.check"
)

// PeerAddedEvent 节点添加事件
type PeerAddedEvent struct {
	PeerInfo  *PeerInfo `json:"peer_info"`  // 节点信息
	BucketIdx int       `json:"bucket_idx"` // 桶索引
	Reason    string    `json:"reason"`     // 添加原因
}

// PeerRemovedEvent 节点移除事件
type PeerRemovedEvent struct {
	PeerID    string `json:"peer_id"`    // 节点ID
	BucketIdx int    `json:"bucket_idx"` // 桶索引
	Reason    string `json:"reason"`     // 移除原因
}

// PeerUpdatedEvent 节点更新事件
type PeerUpdatedEvent struct {
	PeerID  string                 `json:"peer_id"` // 节点ID
	Changes map[string]interface{} `json:"changes"` // 变更内容
	Reason  string                 `json:"reason"`  // 更新原因
}

// BucketRefreshEvent 桶刷新事件
type BucketRefreshEvent struct {
	BucketIdx    int           `json:"bucket_idx"`    // 桶索引
	RefreshType  string        `json:"refresh_type"`  // 刷新类型
	Duration     time.Duration `json:"duration"`      // 刷新耗时
	PeersAdded   int           `json:"peers_added"`   // 新增节点数
	PeersRemoved int           `json:"peers_removed"` // 移除节点数
}
