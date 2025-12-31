// Package types provides repository type definitions.
package types

import "time"

// ==================== 存储库接口辅助类型 ====================
// 这些类型对应于 internal/core/blockchain/interfaces/repository.go 中的接口方法需求
// 只定义pb中没有的、Go特定的辅助类型

// ==================== 存储相关通用数据结构 ====================

// FileInfo 定义文件的元数据信息
type FileInfo struct {
	Size       int64     // 文件大小（字节）
	CreateTime time.Time // 创建时间
	ModTime    time.Time // 修改时间
	IsDir      bool      // 是否目录
}

// TempFileInfo 定义临时文件的信息
type TempFileInfo struct {
	ID         string    // 临时文件唯一标识
	Size       int64     // 大小
	CreateTime time.Time // 创建时间
	ExpireTime time.Time // 过期时间
}

// ProviderOptions 存储提供者选项（聚合各存储实例配置）
type ProviderOptions struct {
	StorageDir   string                            `json:"storage_dir"`
	BadgerStores map[string]map[string]interface{} `json:"badger_stores"`
	MemoryStores map[string]map[string]interface{} `json:"memory_stores"`
	FileStores   map[string]map[string]interface{} `json:"file_stores"`
	TempStores   map[string]map[string]interface{} `json:"temp_stores"`
}

// ==================== UTXO一致性报告类型 ====================
// 被 internal/core/blockchain/repositories/storage/utxo_storage.go 调用

// 注意：UTXOConsistencyReport 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复

// ==================== 索引相关类型 ====================

// 注意：TransactionLocation 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复

// ==================== 其他存储库辅助类型 ====================
// 注意：以下类型已被移除（未使用，过度设计）：
// - StorageResult, RetrievalResult, IndexResult, SearchResult, SearchItem
// - BackupResult, RestoreResult, SyncResult, VerificationResult, MaintenanceResult
// - RepositoryStats, RepositoryHealth, HealthIssue, PerformanceMetrics
// - StorageRequest, RetrievalRequest, SearchRequest, BackupRequest
// - HealthStatus, StorageType（枚举类型）
// 如需使用，可从 git 历史中恢复
