package resourcesvc

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Service 资源视图服务接口（对外公共接口）
//
// 🎯 核心职责：
// - 提供统一的资源视图查询服务，整合 EUTXO 和 URES 两个视角。
// - 对外暴露稳定的查询契约，供 API / SDK / 其他模块使用。
type Service interface {
	// ListResources 列出资源列表
	ListResources(ctx context.Context, filter ResourceViewFilter, page PageRequest) ([]*ResourceView, PageResponse, error)

	// GetResource 获取单个资源（基于 ResourceCodeId）
	// ⚠️ 注意：在多实例场景下，此方法可能返回错误或需要调用方指定实例
	// 推荐：优先使用 GetResourceByInstance 进行精确查询
	GetResource(ctx context.Context, contentHash []byte) (*ResourceView, error)

	// GetResourceByInstance 根据资源实例标识获取资源视图
	// 使用 ResourceInstanceId（OutPoint）作为主键
	GetResourceByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*ResourceView, error)

	// ListResourceInstancesByCode 列出指定代码的所有实例（1:N 映射）
	ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*ResourceView, error)

	// GetResourceHistory 获取资源历史
	GetResourceHistory(ctx context.Context, contentHash []byte, page PageRequest) (*ResourceHistory, error)
}

// ResourceView 资源视图 DTO
//
// 与 internal/core/resourcesvc/types.go 中的定义保持一致，用于对外暴露统一视图。
type ResourceView struct {
	// InstanceOutPoint 资源实例标识（ResourceInstanceId，主键）
	InstanceOutPoint *transaction.OutPoint

	// ContentHash 资源内容哈希（ResourceCodeId）
	ContentHash []byte

	// 资源分类
	Category       string // EXECUTABLE | STATIC
	ExecutableType string // CONTRACT | AI_MODEL | ...

	// 资源元信息
	MimeType string
	Size     uint64

	// UTXO 视角
	OutPoint          *transaction.OutPoint
	Owner             []byte
	Status            string // ACTIVE | CONSUMED | EXPIRED
	CreationTimestamp uint64
	ExpiryTimestamp   *uint64
	IsImmutable       bool

	// 锁定条件（从 UTXO 查询获取）
	LockingConditions []*transaction.LockingCondition

	// 使用统计
	CurrentReferenceCount uint64
	TotalReferenceTimes   uint64

	// 区块信息
	DeployTxId        []byte
	DeployBlockHeight uint64
	DeployBlockHash   []byte
	DeployTimestamp   uint64

	// 执行配置（仅可执行资源）
	ExecutionConfig interface{} // *pbresource.Resource_Contract 或 *pbresource.Resource_Aimodel

	// 文件信息
	OriginalFilename string
	FileExtension    string

	// 创建上下文和交易元数据
	CreationContext string
	DeployMemo      string
	DeployTags      []string
}

// ResourceViewFilter 资源视图过滤条件
type ResourceViewFilter struct {
	Owner          []byte
	Category       *string
	ExecutableType *string
	Status         *string
	Tags           []string

	// ContentHash: 按代码过滤（ResourceCodeId），返回该代码的所有实例
	// InstanceTxHash + InstanceOutputIndex: 按实例过滤（ResourceInstanceId），精确查询
	ContentHash         []byte
	InstanceTxHash      []byte
	InstanceOutputIndex *uint32

	// GroupByCode: 是否按代码聚合（true = 每个代码只返回一个实例）
	GroupByCode bool
}

// PageRequest 分页请求
type PageRequest struct {
	Offset int
	Limit  int
}

// PageResponse 分页响应
type PageResponse struct {
	Total  int
	Offset int
	Limit  int
}

// TxSummary 交易摘要
type TxSummary struct {
	TxId        []byte
	BlockHash   []byte
	BlockHeight uint64
	Timestamp   uint64
}

// ReferenceSummary 引用统计摘要
type ReferenceSummary struct {
	TotalReferences   uint64
	UniqueCallers     uint64
	LastReferenceTime uint64
}

// ResourceHistory 资源历史记录
type ResourceHistory struct {
	DeployTx          *TxSummary
	Upgrades          []*TxSummary
	References        []*TxSummary
	ReferencesSummary *ReferenceSummary
}
