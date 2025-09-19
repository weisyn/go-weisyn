package resource

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// 资源元数据查询服务 - 从区块中实时提取资源元数据
// 严格遵循"区块单一数据源"原则，所有资源元数据都从区块中提取
// 注意：这里只处理资源的元数据，实际资源文件由独立的resource存储实体管理

// BlockStorageInterface 区块存储接口
// 用于从区块存储中获取完整区块数据
type BlockStorageInterface interface {
	GetBlock(ctx context.Context, blockHash []byte) (*pb.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*pb.Block, error)
}

// ResourceMetadataQueryService 资源元数据查询服务
type ResourceMetadataQueryService struct {
	resourceIndex                *ResourceMetadataIndex                   // 资源位置索引
	blockStorage                 BlockStorageInterface                    // 区块存储接口
	logger                       log.Logger                               // 日志服务
	transactionHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	blockHashServiceClient       pb.BlockHashServiceClient                // 区块哈希服务客户端
}

// NewResourceMetadataQueryService 创建资源元数据查询服务
func NewResourceMetadataQueryService(
	resourceIndex *ResourceMetadataIndex,
	blockStorage BlockStorageInterface,
	logger log.Logger,
	transactionHashServiceClient transaction.TransactionHashServiceClient,
	blockHashServiceClient pb.BlockHashServiceClient,
) *ResourceMetadataQueryService {
	return &ResourceMetadataQueryService{
		resourceIndex:                resourceIndex,
		blockStorage:                 blockStorage,
		logger:                       logger,
		transactionHashServiceClient: transactionHashServiceClient,
		blockHashServiceClient:       blockHashServiceClient,
	}
}

// ResourceMetadataDetail 资源元数据详细信息
// 包含资源元数据及其在区块链中的位置信息
type ResourceMetadataDetail struct {
	Resource    *resource.Resource // 完整资源元数据
	BlockHash   []byte             // 所在区块哈希
	BlockHeight uint64             // 所在区块高度
	TxIndex     uint32             // 在区块中的交易索引
	OutputIndex uint32             // 在交易中的输出索引
	TxHash      []byte             // 所在交易哈希
	ContentHash []byte             // 资源内容哈希
}

// GetResourceMetadata 根据资源内容哈希获取完整资源元数据
func (rmqs *ResourceMetadataQueryService) GetResourceMetadata(ctx context.Context, contentHash []byte) (*ResourceMetadataDetail, error) {
	if len(contentHash) == 0 {
		return nil, fmt.Errorf("资源内容哈希不能为空")
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("查询资源元数据 - content_hash: %x", contentHash)
	}

	// 1. 从索引获取资源位置
	location, err := rmqs.resourceIndex.GetResourceLocation(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("获取资源位置失败: %w", err)
	}

	// 2. 从区块存储获取完整区块
	block, err := rmqs.blockStorage.GetBlock(ctx, location.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("获取区块失败 - block_hash: %x, error: %w", location.BlockHash, err)
	}

	// 3. 验证交易索引范围
	if location.TxIndex >= uint32(len(block.Body.Transactions)) {
		return nil, fmt.Errorf("交易索引越界 - tx_index: %d, total_txs: %d",
			location.TxIndex, len(block.Body.Transactions))
	}

	// 4. 获取指定位置的交易
	tx := block.Body.Transactions[location.TxIndex]

	// 5. 验证输出索引范围
	if location.OutputIndex >= uint32(len(tx.Outputs)) {
		return nil, fmt.Errorf("输出索引越界 - output_index: %d, total_outputs: %d",
			location.OutputIndex, len(tx.Outputs))
	}

	// 6. 获取指定位置的输出
	output := tx.Outputs[location.OutputIndex]

	// 7. 验证是否为资源输出
	resourceOutput := output.GetResource()
	if resourceOutput == nil {
		return nil, fmt.Errorf("指定位置不是资源输出 - tx_index: %d, output_index: %d",
			location.TxIndex, location.OutputIndex)
	}

	// 8. 验证资源内容哈希一致性
	if !equalBytes(contentHash, resourceOutput.Resource.ContentHash) {
		return nil, fmt.Errorf("资源内容哈希不匹配 - expected: %x, actual: %x",
			contentHash, resourceOutput.Resource.ContentHash)
	}

	// 9. 使用交易哈希服务计算交易哈希
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}
	resp, err := rmqs.transactionHashServiceClient.ComputeHash(ctx, req)
	if err != nil || !resp.IsValid {
		return nil, fmt.Errorf("计算交易哈希失败: %w", err)
	}
	txHash := resp.Hash

	// 10. 构建资源元数据详细信息
	detail := &ResourceMetadataDetail{
		Resource:    resourceOutput.Resource,
		BlockHash:   location.BlockHash,
		BlockHeight: block.Header.Height,
		TxIndex:     location.TxIndex,
		OutputIndex: location.OutputIndex,
		TxHash:      txHash,
		ContentHash: contentHash,
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("成功查询资源元数据 - content_hash: %x, block_height: %d, tx_index: %d, output_index: %d",
			contentHash, detail.BlockHeight, detail.TxIndex, detail.OutputIndex)
	}

	return detail, nil
}

// GetResourcesByType 根据资源类型获取资源元数据列表
func (rmqs *ResourceMetadataQueryService) GetResourcesByType(ctx context.Context, resourceType resource.ResourceCategory) ([]*ResourceMetadataDetail, error) {
	if rmqs.logger != nil {
		rmqs.logger.Debugf("按类型查询资源元数据 - type: %s", resourceType.String())
	}

	// 1. 从索引获取指定类型的所有资源内容哈希
	contentHashes, err := rmqs.resourceIndex.GetResourcesByType(ctx, resourceType)
	if err != nil {
		return nil, fmt.Errorf("获取指定类型资源列表失败: %w", err)
	}

	// 2. 批量查询每个资源的详细信息
	details := make([]*ResourceMetadataDetail, 0, len(contentHashes))
	for _, contentHash := range contentHashes {
		detail, err := rmqs.GetResourceMetadata(ctx, contentHash)
		if err != nil {
			// 记录错误但继续处理其他资源
			if rmqs.logger != nil {
				rmqs.logger.Warnf("查询资源元数据失败 - content_hash: %x, error: %v", contentHash, err)
			}
			continue
		}
		details = append(details, detail)
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("按类型查询完成 - type: %s, total: %d, success: %d",
			resourceType.String(), len(contentHashes), len(details))
	}

	return details, nil
}

// GetResourcesByBlockHash 获取指定区块中的所有资源元数据
func (rmqs *ResourceMetadataQueryService) GetResourcesByBlockHash(ctx context.Context, blockHash []byte) ([]*ResourceMetadataDetail, error) {
	if len(blockHash) == 0 {
		return nil, fmt.Errorf("区块哈希不能为空")
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("查询区块资源元数据列表 - block_hash: %x", blockHash)
	}

	// 1. 从区块存储获取完整区块
	block, err := rmqs.blockStorage.GetBlock(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("获取区块失败: %w", err)
	}

	// 2. 遍历区块中的所有交易和输出，提取资源元数据
	var resources []*ResourceMetadataDetail
	for txIndex, tx := range block.Body.Transactions {
		// 使用交易哈希服务计算交易哈希
		req := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}
		resp, err := rmqs.transactionHashServiceClient.ComputeHash(ctx, req)
		if err != nil || !resp.IsValid {
			continue // 跳过无效交易
		}
		txHash := resp.Hash

		for outputIndex, output := range tx.Outputs {
			// 检查是否为资源输出
			if resourceOutput := output.GetResource(); resourceOutput != nil {
				detail := &ResourceMetadataDetail{
					Resource:    resourceOutput.Resource,
					BlockHash:   blockHash,
					BlockHeight: block.Header.Height,
					TxIndex:     uint32(txIndex),
					OutputIndex: uint32(outputIndex),
					TxHash:      txHash,
					ContentHash: resourceOutput.Resource.ContentHash,
				}
				resources = append(resources, detail)
			}
		}
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("成功查询区块资源元数据列表 - block_hash: %x, resource_count: %d",
			blockHash, len(resources))
	}

	return resources, nil
}

// GetResourcesByBlockHeight 获取指定高度区块中的所有资源元数据
func (rmqs *ResourceMetadataQueryService) GetResourcesByBlockHeight(ctx context.Context, height uint64) ([]*ResourceMetadataDetail, error) {
	if rmqs.logger != nil {
		rmqs.logger.Debugf("查询指定高度区块资源元数据列表 - height: %d", height)
	}

	// 1. 从区块存储获取指定高度的区块
	block, err := rmqs.blockStorage.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("获取指定高度区块失败: %w", err)
	}

	// 2. 使用区块哈希服务计算区块哈希
	req := &pb.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}
	resp, err := rmqs.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil || !resp.IsValid {
		return nil, fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash

	return rmqs.GetResourcesByBlockHash(ctx, blockHash)
}

// HasResource 检查资源是否存在
func (rmqs *ResourceMetadataQueryService) HasResource(ctx context.Context, contentHash []byte) (bool, error) {
	return rmqs.resourceIndex.HasResource(ctx, contentHash)
}

// GetResourceLocation 获取资源位置信息（不包含资源内容）
func (rmqs *ResourceMetadataQueryService) GetResourceLocation(ctx context.Context, contentHash []byte) (*ResourceLocation, error) {
	return rmqs.resourceIndex.GetResourceLocation(ctx, contentHash)
}

// ValidateResourceMetadata 验证资源元数据完整性
func (rmqs *ResourceMetadataQueryService) ValidateResourceMetadata(ctx context.Context, contentHash []byte) error {
	if rmqs.logger != nil {
		rmqs.logger.Debugf("验证资源元数据完整性 - content_hash: %x", contentHash)
	}

	// 1. 获取资源元数据详细信息
	detail, err := rmqs.GetResourceMetadata(ctx, contentHash)
	if err != nil {
		return fmt.Errorf("获取资源元数据失败: %w", err)
	}

	// 2. 验证资源基本结构
	if detail.Resource == nil {
		return fmt.Errorf("资源元数据为空")
	}

	// 3. 验证必要字段
	if len(detail.Resource.ContentHash) == 0 {
		return fmt.Errorf("资源内容哈希为空")
	}

	if len(detail.Resource.MimeType) == 0 {
		return fmt.Errorf("资源MIME类型为空")
	}

	if detail.Resource.Size == 0 {
		return fmt.Errorf("资源大小为0")
	}

	// 4. 验证内容哈希一致性
	if !equalBytes(contentHash, detail.Resource.ContentHash) {
		return fmt.Errorf("内容哈希验证失败 - expected: %x, actual: %x",
			contentHash, detail.Resource.ContentHash)
	}

	if rmqs.logger != nil {
		rmqs.logger.Debugf("资源元数据验证通过 - content_hash: %x", contentHash)
	}

	return nil
}

// GetResourceStats 获取资源统计信息
func (rmqs *ResourceMetadataQueryService) GetResourceStats(ctx context.Context) (*ResourceStats, error) {
	if rmqs.logger != nil {
		rmqs.logger.Debugf("查询资源统计信息")
	}

	// 返回基础统计信息
	// 注意：详细统计信息需要从缓存或专门的统计索引获取
	stats := &ResourceStats{
		TotalResources: 0, // 需要遍历计算或从缓存获取
		// 其他统计信息根据业务需要扩展
	}

	return stats, nil
}

// ResourceStats 资源统计信息
type ResourceStats struct {
	TotalResources uint64 `json:"total_resources"` // 总资源数量
	// 可以添加更多统计字段：
	// ExecutableResources uint64 `json:"executable_resources"` // 可执行资源数量
	// StaticResources     uint64 `json:"static_resources"`     // 静态资源数量
	// ResourcesByType     map[string]uint64 `json:"resources_by_type"` // 按类型分组的资源数量
}

// ========== 辅助函数 ==========

// equalBytes 比较两个字节数组是否相等
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
