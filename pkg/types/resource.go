package types

// ==================== 未使用的资源类型（已注释） ====================
// 以下类型未被任何接口使用，按照"只保留被实际使用的类型"原则进行注释

/*
// ResourceLocation 资源位置信息
// 📞 **使用者**: 此类型未被任何接口使用
type ResourceLocation struct {
	ResourceHash *core.Hash `json:"resource_hash"` // 资源哈希
	BlockHeight  uint64     `json:"block_height"`  // 区块高度
	BlockHash    *core.Hash `json:"block_hash"`    // 区块哈希
	TxHash       *core.Hash `json:"tx_hash"`       // 交易哈希
	Available    bool       `json:"available"`     // 是否可用
}

// ResourceDeployResult 通用资源部署结果
// 包含资源部署完成后的核心信息，专注于业务结果
// 📞 **使用者**: 此类型未被任何接口使用
type ResourceDeployResult struct {
	ContentHash     *core.Hash `json:"content_hash"`     // 资源内容的SHA-256哈希，作为资源的唯一标识符
	TransactionHash *core.Hash `json:"transaction_hash"` // 部署交易的哈希值，用于追踪部署状态
	CreatedAt       Timestamp  `json:"created_at"`       // 资源创建的Unix时间戳
	Size            uint64     `json:"size"`             // 资源内容的字节大小，用于验证
	Status          Status     `json:"status"`           // 部署状态（成功/失败/待确认等）
}

// ResourceDeploymentResult 资源部署结果类型别名
// 为了兼容性，提供ResourceDeployResult的别名
// 📞 **使用者**: 此类型未被任何接口使用
type ResourceDeploymentResult = ResourceDeployResult
*/

// ResourceStorageInfo 资源存储信息
//
// 🎯 **资源存储信息结构**
//
// 描述存储在WES系统中的资源的完整元数据信息，
// 包含资源的身份标识、内容特征、存储状态等核心信息。
//
// 💡 **设计特点**：
// - 完整元数据：涵盖资源的所有存储相关信息
// - 内容寻址：通过ContentHash实现内容寻址
// - 存储状态：IsAvailable标识资源的可用性状态
// - 后端无关：StorageBackend记录实际存储位置
//
// 📋 **应用场景**：
// - 资源查询：为资源管理提供完整的存储信息
// - 内容验证：通过哈希和大小验证资源完整性
// - 存储管理：跟踪资源在不同存储后端的分布
// - API响应：为外部API提供标准的资源信息格式
type ResourceStorageInfo struct {
	// ResourcePath 资源路径标识符
	//
	// 资源在系统中的逻辑路径，用于标识和查找资源。
	// 通常采用层次化路径结构，如：/contracts/token.wasm
	ResourcePath string `json:"resource_path"`

	// ResourceType 资源类型标识符
	//
	// 标识资源的类型，用于分类管理和处理逻辑选择。
	// 常见类型：contract, aimodel, static, data等
	ResourceType string `json:"resource_type"`

	// ContentHash 内容哈希
	//
	// 资源内容的SHA-256哈希值（32字节），用于：
	// - 内容寻址：作为资源的全局唯一标识
	// - 完整性验证：检查资源内容是否被篡改
	// - 去重存储：相同内容的资源只存储一份
	ContentHash []byte `json:"content_hash"`

	// Size 资源文件大小
	//
	// 资源内容的字节大小，用于：
	// - 存储统计：计算存储空间占用
	// - 传输优化：选择合适的传输策略
	// - 验证检查：确保读取的内容完整
	Size int64 `json:"size"`

	// StoredAt 存储时间戳
	//
	// 资源首次存储的Unix时间戳，用于：
	// - 时间管理：资源生命周期跟踪
	// - 存储统计：按时间分析存储增长
	// - 清理策略：基于时间的资源清理
	StoredAt int64 `json:"stored_at"`

	// Metadata 资源元数据
	//
	// 键值对形式的资源元数据信息，包含：
	// - 业务属性：版本、作者、描述等
	// - 技术参数：格式、编码、压缩等
	// - 扩展信息：自定义的业务相关元数据
	Metadata map[string]string `json:"metadata"`

	// IsAvailable 资源可用性状态
	//
	// 标识资源当前是否可用：
	// - true：资源完整且可正常访问
	// - false：资源损坏、缺失或被标记为不可用
	IsAvailable bool `json:"is_available"`

	// StorageBackend 存储后端标识符
	//
	// 标识资源实际存储的后端系统：
	// - "file"：文件系统存储
	// - "badger"：BadgerDB存储
	// - "memory"：内存存储
	// - "hybrid"：混合存储模式
	StorageBackend string `json:"storage_backend"`
}

// 注意：其他复杂的资源管理类型都已经被正确注释掉
// 如需要时可取消注释
