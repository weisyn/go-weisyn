package types

// ==================== 存储库接口辅助类型 ====================
// 这些类型对应于 internal/core/blockchain/interfaces/repository.go 中的接口方法需求
// 只定义pb中没有的、Go特定的辅助类型

// ==================== UTXO一致性报告类型 ====================
// 被 internal/core/blockchain/repositories/storage/utxo_storage.go 调用

// UTXOConsistencyReport UTXO一致性检查报告
// 用于UTXOStorage.VerifyConsistency方法的返回值
type UTXOConsistencyReport struct {
	// 统计信息
	TotalUTXOs      int  `json:"total_utxos"`      // 总UTXO数量
	IndexMismatches int  `json:"index_mismatches"` // 索引不匹配数量
	OrphanedIndexes int  `json:"orphaned_indexes"` // 孤立索引数量
	IsConsistent    bool `json:"is_consistent"`    // 是否一致

	// 问题详情
	Issues []string `json:"issues"` // 具体问题列表
}

// ==================== 索引相关类型 ====================

// TransactionLocation 交易在区块链中的位置信息
// 用途：支持TransactionIndex.GetTransactionLocation方法，提供交易定位能力
type TransactionLocation struct {
	BlockHash []byte `json:"block_hash"` // 所在区块哈希
	TxIndex   uint32 `json:"tx_index"`   // 在区块中的索引
	Height    uint64 `json:"height"`     // 区块高度
}

// ==================== 其他存储库辅助类型 ====================

// 🚨 注意：以下结构体暂时未被使用，已注释以避免代码冗余
// 如需要使用，请取消注释并更新相关接口

// StorageResult 存储结果 - 暂未使用，已注释
/*
type StorageResult struct {
	Success     bool                   `json:"success"`
	StoredHash  Hash                   `json:"stored_hash,omitempty"`
	StoredTime  Timestamp              `json:"stored_time"`
	StorageSize uint64                 `json:"storage_size"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// RetrievalResult 检索结果 - 暂未使用，已注释
/*
type RetrievalResult struct {
	Success       bool                   `json:"success"`
	Data          []byte                 `json:"data,omitempty"`
	RetrievedTime Timestamp              `json:"retrieved_time"`
	DataSize      uint64                 `json:"data_size"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
*/

// IndexResult 索引结果 - 暂未使用，已注释
/*
type IndexResult struct {
	Success     bool                   `json:"success"`
	IndexedHash Hash                   `json:"indexed_hash,omitempty"`
	IndexedTime Timestamp              `json:"indexed_time"`
	IndexSize   uint64                 `json:"index_size"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// SearchResult 搜索结果 - 暂未使用，已注释（注意：UTXOSearchResult在utxo.go中另有定义）
/*
type SearchResult struct {
	Success    bool                   `json:"success"`
	Items      []*SearchItem          `json:"items"`
	TotalCount uint64                 `json:"total_count"`
	SearchTime Timestamp              `json:"search_time"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
*/

// SearchItem 搜索项 - 暂未使用，已注释
/*
type SearchItem struct {
	ID          string                 `json:"id"`
	Hash        Hash                   `json:"hash"`
	Type        string                 `json:"type"`
	Data        []byte                 `json:"data,omitempty"`
	Score       float64                `json:"score"`
	CreatedTime Timestamp              `json:"created_time"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// BackupResult 备份结果 - 暂未使用，已注释
/*
type BackupResult struct {
	Success    bool                   `json:"success"`
	BackupID   string                 `json:"backup_id"`
	BackupPath string                 `json:"backup_path,omitempty"`
	BackupSize uint64                 `json:"backup_size"`
	BackupTime Timestamp              `json:"backup_time"`
	Checksum   Hash                   `json:"checksum"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
*/

// RestoreResult 恢复结果 - 暂未使用，已注释
/*
type RestoreResult struct {
	Success       bool                   `json:"success"`
	RestoredItems uint64                 `json:"restored_items"`
	RestoreTime   Timestamp              `json:"restore_time"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
*/

// SyncResult 同步结果 - 暂未使用，已注释
/*
type SyncResult struct {
	Success     bool                   `json:"success"`
	SyncedItems uint64                 `json:"synced_items"`
	SyncTime    Timestamp              `json:"sync_time"`
	FromHeight  uint64                 `json:"from_height"`
	ToHeight    uint64                 `json:"to_height"`
	PeerID      string                 `json:"peer_id,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// VerificationResult 验证结果 - 暂未使用，已注释
/*
type VerificationResult struct {
	Valid            bool                   `json:"valid"`
	VerifiedHash     Hash                   `json:"verified_hash,omitempty"`
	ExpectedHash     Hash                   `json:"expected_hash,omitempty"`
	ValidationErrors []string               `json:"validation_errors,omitempty"`
	VerificationTime Timestamp              `json:"verification_time"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}
*/

// MaintenanceResult 维护结果 - 暂未使用，已注释
/*
type MaintenanceResult struct {
	Success         bool                   `json:"success"`
	OperationType   string                 `json:"operation_type"`
	ProcessedItems  uint64                 `json:"processed_items"`
	ReclaimedSpace  uint64                 `json:"reclaimed_space,omitempty"`
	MaintenanceTime Timestamp              `json:"maintenance_time"`
	Error           string                 `json:"error,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
*/

// RepositoryStats 存储库统计信息 - 暂未使用，已注释
/*
type RepositoryStats struct {
	TotalItems        uint64                 `json:"total_items"`
	TotalSize         uint64                 `json:"total_size"`
	IndexSize         uint64                 `json:"index_size"`
	LastUpdated       Timestamp              `json:"last_updated"`
	ActiveConnections uint32                 `json:"active_connections"`
	CacheHitRate      float64                `json:"cache_hit_rate"`
	DiskUsage         uint64                 `json:"disk_usage"`
	MemoryUsage       uint64                 `json:"memory_usage"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}
*/

// RepositoryHealth 存储库健康状态 - 暂未使用，已注释
/*
type RepositoryHealth struct {
	Status       HealthStatus           `json:"status"`
	LastChecked  Timestamp              `json:"last_checked"`
	ErrorCount   uint64                 `json:"error_count"`
	WarningCount uint64                 `json:"warning_count"`
	Issues       []HealthIssue          `json:"issues,omitempty"`
	Performance  *PerformanceMetrics    `json:"performance,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
*/

// HealthIssue 健康问题 - 暂未使用，已注释
/*
type HealthIssue struct {
	Level        string    `json:"level"`
	Component    string    `json:"component"`
	Description  string    `json:"description"`
	DetectedTime Timestamp `json:"detected_time"`
	Suggestion   string    `json:"suggestion,omitempty"`
}
*/

// PerformanceMetrics 性能指标 - 暂未使用，已注释
/*
type PerformanceMetrics struct {
	AvgReadLatency  uint64 `json:"avg_read_latency"`
	AvgWriteLatency uint64 `json:"avg_write_latency"`
	Throughput      uint64 `json:"throughput"`
	IOPS            uint64 `json:"iops"`
	ConcurrentOps   uint32 `json:"concurrent_ops"`
}
*/

// ==================== 请求类型 - 暂未使用，已注释 ====================

// StorageRequest 存储请求 - 暂未使用，已注释
/*
type StorageRequest struct {
	Key         Hash                   `json:"key" validate:"required"`
	Data        []byte                 `json:"data" validate:"required"`
	Type        string                 `json:"type" validate:"required"`
	TTL         uint64                 `json:"ttl,omitempty"`
	Compression bool                   `json:"compression"`
	Encryption  bool                   `json:"encryption"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// RetrievalRequest 检索请求 - 暂未使用，已注释
/*
type RetrievalRequest struct {
	Key             Hash   `json:"key" validate:"required"`
	Type            string `json:"type,omitempty"`
	IncludeMetadata bool   `json:"include_metadata"`
	VerifyIntegrity bool   `json:"verify_integrity"`
}
*/

// SearchRequest 搜索请求 - 暂未使用，已注释
/*
type SearchRequest struct {
	Query      string                 `json:"query" validate:"required"`
	Type       string                 `json:"type,omitempty"`
	FromTime   Timestamp              `json:"from_time,omitempty"`
	ToTime     Timestamp              `json:"to_time,omitempty"`
	PageSize   uint32                 `json:"page_size" validate:"min=1,max=100"`
	PageNumber uint32                 `json:"page_number" validate:"min=1"`
	SortBy     string                 `json:"sort_by,omitempty"`
	SortOrder  string                 `json:"sort_order,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
}
*/

// BackupRequest 备份请求 - 暂未使用，已注释
/*
type BackupRequest struct {
	BackupType  string                 `json:"backup_type" validate:"required"`
	Destination string                 `json:"destination" validate:"required"`
	Compression bool                   `json:"compression"`
	Encryption  bool                   `json:"encryption"`
	Incremental bool                   `json:"incremental"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
*/

// ==================== 枚举类型 ====================

// HealthStatus 健康状态 - 暂未使用，已注释
/*
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusWarning
	HealthStatusCritical
	HealthStatusDown
)
*/

// String 返回健康状态的字符串表示 - 暂未使用，已注释
/*
func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusUnknown:
		return "unknown"
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusWarning:
		return "warning"
	case HealthStatusCritical:
		return "critical"
	case HealthStatusDown:
		return "down"
	default:
		return "unknown"
	}
}
*/

// StorageType 存储类型 - 暂未使用，已注释
/*
type StorageType int

const (
	StorageTypeUnknown StorageType = iota
	StorageTypeBlock
	StorageTypeTransaction
	StorageTypeState
	StorageTypeIndex
	StorageTypeBackup
)
*/

// String 返回存储类型的字符串表示 - 暂未使用，已注释
/*
func (st StorageType) String() string {
	switch st {
	case StorageTypeUnknown:
		return "unknown"
	case StorageTypeBlock:
		return "block"
	case StorageTypeTransaction:
		return "transaction"
	case StorageTypeState:
		return "state"
	case StorageTypeIndex:
		return "index"
	case StorageTypeBackup:
		return "backup"
	default:
		return "unknown"
	}
}
*/
