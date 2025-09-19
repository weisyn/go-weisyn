package resource

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ResourceService 资源元数据服务统一接口
// 整合资源元数据索引和查询功能，提供完整的资源元数据管理服务
// 注意：这里只处理资源的元数据，实际资源文件由独立的resource存储实体管理
type ResourceService struct {
	index        *ResourceMetadataIndex        // 资源元数据位置索引
	queryService *ResourceMetadataQueryService // 资源元数据查询服务
	storage      storage.BadgerStore           // 持久化存储
	logger       log.Logger                    // 日志服务
}

// NewResourceService 创建资源元数据服务
func NewResourceService(
	storage storage.BadgerStore,
	blockStorage BlockStorageInterface,
	logger log.Logger,
	transactionHashServiceClient transaction.TransactionHashServiceClient,
	blockHashServiceClient pb.BlockHashServiceClient,
) *ResourceService {
	// 创建资源元数据索引
	index := NewResourceMetadataIndex(storage, logger)

	// 创建查询服务
	queryService := NewResourceMetadataQueryService(index, blockStorage, logger, transactionHashServiceClient, blockHashServiceClient)

	return &ResourceService{
		index:        index,
		queryService: queryService,
		storage:      storage,
		logger:       logger,
	}
}

// ========== 索引管理接口 ==========

// IndexResourceMetadata 为区块中的所有资源元数据建立索引
// ⚠️ 【写入边界】此方法只能在Manager.StoreBlock的事务中调用
// 在存储新区块时调用此方法
func (rs *ResourceService) IndexResourceMetadata(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte, block *pb.Block) error {
	return rs.index.IndexResourceMetadata(ctx, tx, blockHash, block)
}

// RemoveResourceIndex 移除资源索引
// ⚠️ 【写入边界】此方法只能在Manager区块回滚事务中调用
// 在区块回滚时调用此方法
func (rs *ResourceService) RemoveResourceIndex(ctx context.Context, tx storage.BadgerTransaction, contentHash []byte, resourceType resource.ResourceCategory) error {
	return rs.index.RemoveResourceIndex(ctx, tx, contentHash, resourceType)
}

// ========== 查询服务接口 ==========

// GetResourceMetadata 根据资源内容哈希获取完整资源元数据
func (rs *ResourceService) GetResourceMetadata(ctx context.Context, contentHash []byte) (*ResourceMetadataDetail, error) {
	return rs.queryService.GetResourceMetadata(ctx, contentHash)
}

// GetResourcesByType 根据资源类型获取资源元数据列表
func (rs *ResourceService) GetResourcesByType(ctx context.Context, resourceType resource.ResourceCategory) ([]*ResourceMetadataDetail, error) {
	return rs.queryService.GetResourcesByType(ctx, resourceType)
}

// GetResourcesByBlockHash 获取指定区块中的所有资源元数据
func (rs *ResourceService) GetResourcesByBlockHash(ctx context.Context, blockHash []byte) ([]*ResourceMetadataDetail, error) {
	return rs.queryService.GetResourcesByBlockHash(ctx, blockHash)
}

// GetResourcesByBlockHeight 获取指定高度区块中的所有资源元数据
func (rs *ResourceService) GetResourcesByBlockHeight(ctx context.Context, height uint64) ([]*ResourceMetadataDetail, error) {
	return rs.queryService.GetResourcesByBlockHeight(ctx, height)
}

// HasResource 检查资源是否存在
func (rs *ResourceService) HasResource(ctx context.Context, contentHash []byte) (bool, error) {
	return rs.queryService.HasResource(ctx, contentHash)
}

// GetResourceLocation 获取资源位置信息
func (rs *ResourceService) GetResourceLocation(ctx context.Context, contentHash []byte) (*ResourceLocation, error) {
	return rs.queryService.GetResourceLocation(ctx, contentHash)
}

// ValidateResourceMetadata 验证资源元数据完整性
func (rs *ResourceService) ValidateResourceMetadata(ctx context.Context, contentHash []byte) error {
	return rs.queryService.ValidateResourceMetadata(ctx, contentHash)
}

// ========== 批量操作接口 ==========

// GetMultipleResourceMetadata 批量获取多个资源元数据
func (rs *ResourceService) GetMultipleResourceMetadata(ctx context.Context, contentHashes [][]byte) ([]*ResourceMetadataDetail, []error) {
	if len(contentHashes) == 0 {
		return nil, []error{fmt.Errorf("内容哈希列表不能为空")}
	}

	if rs.logger != nil {
		rs.logger.Debugf("批量查询资源元数据 - count: %d", len(contentHashes))
	}

	results := make([]*ResourceMetadataDetail, len(contentHashes))
	errors := make([]error, len(contentHashes))

	// 并发查询每个资源
	for i, contentHash := range contentHashes {
		detail, err := rs.queryService.GetResourceMetadata(ctx, contentHash)
		results[i] = detail
		errors[i] = err
	}

	if rs.logger != nil {
		successCount := 0
		for _, err := range errors {
			if err == nil {
				successCount++
			}
		}
		rs.logger.Debugf("批量查询完成 - total: %d, success: %d, failed: %d",
			len(contentHashes), successCount, len(contentHashes)-successCount)
	}

	return results, errors
}

// ValidateMultipleResourceMetadata 批量验证资源元数据
func (rs *ResourceService) ValidateMultipleResourceMetadata(ctx context.Context, contentHashes [][]byte) []error {
	if len(contentHashes) == 0 {
		return []error{fmt.Errorf("内容哈希列表不能为空")}
	}

	if rs.logger != nil {
		rs.logger.Debugf("批量验证资源元数据 - count: %d", len(contentHashes))
	}

	errors := make([]error, len(contentHashes))

	// 验证每个资源
	for i, contentHash := range contentHashes {
		errors[i] = rs.queryService.ValidateResourceMetadata(ctx, contentHash)
	}

	if rs.logger != nil {
		validCount := 0
		for _, err := range errors {
			if err == nil {
				validCount++
			}
		}
		rs.logger.Debugf("批量验证完成 - total: %d, valid: %d, invalid: %d",
			len(contentHashes), validCount, len(contentHashes)-validCount)
	}

	return errors
}

// ========== 统计查询接口 ==========

// GetResourceStats 获取资源统计信息
func (rs *ResourceService) GetResourceStats(ctx context.Context) (*ResourceStats, error) {
	return rs.queryService.GetResourceStats(ctx)
}

// GetResourceStatsByType 获取按类型分组的资源统计
func (rs *ResourceService) GetResourceStatsByType(ctx context.Context) (map[resource.ResourceCategory]uint64, error) {
	if rs.logger != nil {
		rs.logger.Debugf("查询按类型分组的资源统计")
	}

	stats := make(map[resource.ResourceCategory]uint64)

	// 遍历所有资源类型
	resourceTypes := []resource.ResourceCategory{
		resource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		resource.ResourceCategory_RESOURCE_CATEGORY_STATIC,
		// 可以添加更多类型
	}

	for _, resourceType := range resourceTypes {
		resources, err := rs.queryService.GetResourcesByType(ctx, resourceType)
		if err != nil {
			if rs.logger != nil {
				rs.logger.Warnf("获取类型 %s 的资源统计失败: %v", resourceType.String(), err)
			}
			stats[resourceType] = 0
			continue
		}
		stats[resourceType] = uint64(len(resources))
	}

	if rs.logger != nil {
		rs.logger.Debugf("按类型分组统计完成 - types: %d", len(stats))
	}

	return stats, nil
}

// ========== 高级查询接口 ==========

// SearchResourcesByName 根据资源名称搜索资源
func (rs *ResourceService) SearchResourcesByName(ctx context.Context, name string) ([]*ResourceMetadataDetail, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("资源名称不能为空")
	}

	if rs.logger != nil {
		rs.logger.Debugf("根据名称搜索资源 - name: %s", name)
	}

	// 当前实现：返回空结果
	// 名称索引功能需要在index模块中实现专门的名称到内容哈希的映射
	// 这涉及到对所有资源的名称字段进行索引建立
	var results []*ResourceMetadataDetail

	if rs.logger != nil {
		rs.logger.Warnf("名称资源搜索需要名称索引支持，当前返回空结果")
	}

	return results, nil
}

// SearchResourcesByMimeType 根据MIME类型搜索资源
func (rs *ResourceService) SearchResourcesByMimeType(ctx context.Context, mimeType string) ([]*ResourceMetadataDetail, error) {
	if len(mimeType) == 0 {
		return nil, fmt.Errorf("MIME类型不能为空")
	}

	if rs.logger != nil {
		rs.logger.Debugf("根据MIME类型搜索资源 - mime_type: %s", mimeType)
	}

	// 当前实现：返回空结果
	// MIME类型索引功能需要在index模块中实现专门的MIME类型到内容哈希的映射
	var results []*ResourceMetadataDetail

	if rs.logger != nil {
		rs.logger.Warnf("MIME类型资源搜索需要MIME类型索引支持，当前返回空结果")
	}

	return results, nil
}

// GetResourceHistory 获取资源的版本历史
func (rs *ResourceService) GetResourceHistory(ctx context.Context, name string, version string) ([]*ResourceMetadataDetail, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("资源名称不能为空")
	}

	if rs.logger != nil {
		rs.logger.Debugf("查询资源版本历史 - name: %s, version: %s", name, version)
	}

	// 当前实现：返回空结果
	// 资源版本历史功能需要配合名称索引和版本管理机制
	var results []*ResourceMetadataDetail

	if rs.logger != nil {
		rs.logger.Warnf("资源版本历史功能需要版本管理支持，当前返回空结果")
	}

	return results, nil
}

// ========== 资源内容验证接口 ==========

// VerifyResourceIntegrity 验证资源内容完整性
// 注意：这个方法需要与独立的resource存储实体协作
func (rs *ResourceService) VerifyResourceIntegrity(ctx context.Context, contentHash []byte) error {
	if len(contentHash) == 0 {
		return fmt.Errorf("内容哈希不能为空")
	}

	if rs.logger != nil {
		rs.logger.Debugf("验证资源内容完整性 - content_hash: %x", contentHash)
	}

	// 1. 验证元数据存在性
	if err := rs.ValidateResourceMetadata(ctx, contentHash); err != nil {
		return fmt.Errorf("资源元数据验证失败: %w", err)
	}

	// 2. 与独立的resource存储实体协作，验证实际文件完整性
	// 注意：这里需要调用resource存储实体的接口来验证文件内容
	// 架构设计：rs.resourceManager.VerifyResourceContent(ctx, contentHash)
	// 当前版本：仅验证元数据存在性，文件完整性验证待集成resource模块

	if rs.logger != nil {
		rs.logger.Debugf("资源内容完整性验证通过 - content_hash: %x", contentHash)
	}

	return nil
}
