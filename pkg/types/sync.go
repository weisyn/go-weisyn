// Package types 提供WES系统的同步相关类型定义
package types

// ============================================================================
//                              同步状态类型
// ============================================================================

// SystemSyncStatusType 系统同步状态类型
//
// 定义区块链同步服务的各种状态，用于状态管理和外部查询。
type SystemSyncStatusType int

const (
	// SyncStatusIdle 空闲状态
	// 服务已启动但当前没有进行同步操作
	SyncStatusIdle SystemSyncStatusType = iota

	// SyncStatusSyncing 同步中
	// 正在执行区块数据同步操作
	SyncStatusSyncing

	// SyncStatusSynced 已同步
	// 已与网络保持同步状态，暂无新数据需要同步
	SyncStatusSynced

	// SyncStatusError 错误状态
	// 同步过程中遇到错误，需要人工干预或自动重试
	SyncStatusError
)

// String 返回状态类型的字符串表示
func (s SystemSyncStatusType) String() string {
	switch s {
	case SyncStatusIdle:
		return "idle"
	case SyncStatusSyncing:
		return "syncing"
	case SyncStatusSynced:
		return "synced"
	case SyncStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// ============================================================================
//                              同步状态结构
// ============================================================================

// SystemSyncStatus 系统同步状态信息
//
// 🎯 **简洁设计原则**：
// 只包含用户和系统真正需要的核心信息，避免过度设计
//
// 核心信息：
// - 当前同步状态和进度
// - 区块高度信息
// - 错误信息（如果有）
// - 最后同步时间（用于监控）
type SystemSyncStatus struct {
	// Status 当前同步状态
	Status SystemSyncStatusType `json:"status"`

	// CurrentHeight 当前本地区块高度
	CurrentHeight uint64 `json:"current_height"`

	// NetworkHeight 网络最新区块高度
	// 注意：这是一个估计值，可能不完全准确
	NetworkHeight uint64 `json:"network_height"`

	// SyncProgress 同步进度百分比 (0.0-100.0)
	// 计算公式：(CurrentHeight / NetworkHeight) * 100
	SyncProgress float64 `json:"sync_progress"`

	// LastSyncTime 最后一次同步时间
	// 用于监控和判断同步是否活跃
	LastSyncTime RFC3339Time `json:"last_sync_time"`

	// ErrorMessage 错误信息（仅在Status为SyncStatusError时有值）
	ErrorMessage string `json:"error_message,omitempty"`
}
