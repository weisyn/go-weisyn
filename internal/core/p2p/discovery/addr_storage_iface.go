package discovery

import (
	"context"
	"time"
)

// PeerAddrRecordVersion 当前记录版本
const PeerAddrRecordVersion = 1

// PeerAddrRecord 持久化的 peer 地址记录（v1）
//
// 说明：
// - 这是“发现层/连接层”的地址元数据，不用于存储链状态，仅用于网络自愈与重启恢复。
// - 该结构需要向前兼容（通过 Version 字段），但“存储后端”允许切换（badger/json）。
type PeerAddrRecord struct {
	Version int `json:"v"`

	PeerID string   `json:"peer_id"`
	Addrs  []string `json:"addrs"`

	// 观测时间戳
	LastSeenAt      time.Time `json:"last_seen_at"`
	LastConnectedAt time.Time `json:"last_connected_at,omitempty"`
	LastFailedAt    time.Time `json:"last_failed_at,omitempty"`

	// 计数器（用于 prune 策略）
	SuccessCount int `json:"success_count,omitempty"`
	FailCount    int `json:"fail_count,omitempty"`

	// 是否为 bootstrap（用于更保守的清理策略）
	IsBootstrap bool `json:"is_bootstrap,omitempty"`
}

// AddrStore peer 地址持久化抽象
//
// 注意：我们选择“专用 BadgerDB”并允许保存 all_discovered，因此必须支持：
// - LoadAll：启动回填
// - ScanPrefix：定期 prune（避免爆内存，允许后续优化为迭代器）
type AddrStore interface {
	// LoadAll 加载全部记录（用于启动回填；数据过大时应结合 prune/TTL 控制规模）
	LoadAll(ctx context.Context) ([]*PeerAddrRecord, error)

	// Get 获取单条记录
	Get(ctx context.Context, peerID string) (*PeerAddrRecord, bool, error)

	// Upsert 插入或更新记录
	Upsert(ctx context.Context, rec *PeerAddrRecord) error

	// Delete 删除记录
	Delete(ctx context.Context, peerID string) error

	// Close 关闭底层资源
	Close() error
}


