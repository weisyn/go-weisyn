package resource

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 资源元数据位置索引 - 仅存储位置信息，不存储资源内容
// 严格遵循"区块单一数据源"原则，索引只存储位置映射
// 注意：这里只处理资源的元数据，实际资源文件由独立的resource存储实体管理

const (
	// 资源索引键前缀
	ResourceIndexKeyPrefix = "res:"
	// 资源类型索引键前缀（按类型查找资源）
	ResourceTypeIndexKeyPrefix = "res_type:"
)

// ResourceLocation 资源位置信息
// 描述资源元数据在区块链中的精确位置
type ResourceLocation struct {
	BlockHash   []byte // 所在区块哈希
	TxIndex     uint32 // 在区块中的交易索引
	OutputIndex uint32 // 在交易输出中的索引
}

// ResourceMetadataIndex 资源元数据位置索引管理器
type ResourceMetadataIndex struct {
	storage storage.BadgerStore // 持久化存储
	logger  log.Logger          // 日志服务
}

// NewResourceMetadataIndex 创建资源元数据索引管理器
func NewResourceMetadataIndex(storage storage.BadgerStore, logger log.Logger) *ResourceMetadataIndex {
	return &ResourceMetadataIndex{
		storage: storage,
		logger:  logger,
	}
}

// IndexResourceMetadata 为区块中的所有资源元数据建立索引
// ⚠️ 【写入边界】此方法只能在ResourceService.IndexResourceMetadata中调用
// 这个方法在存储区块时被调用，批量建立资源索引
func (rmi *ResourceMetadataIndex) IndexResourceMetadata(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte, block *pb.Block) error {
	if rmi.logger != nil {
		rmi.logger.Debugf("为区块建立资源元数据索引 - height: %d", block.Header.Height)
	}

	// 验证区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}

	resourceCount := 0

	// 遍历区块中的所有交易
	for txIndex, transaction := range block.Body.Transactions {
		// 遍历交易的所有输出
		for outputIndex, output := range transaction.Outputs {
			// 检查是否为资源输出
			if resourceOutput := output.GetResource(); resourceOutput != nil {
				// 为资源元数据建立索引
				if err := rmi.indexSingleResource(tx, blockHash, uint32(txIndex), uint32(outputIndex), resourceOutput.Resource); err != nil {
					return fmt.Errorf("索引资源失败 - tx_index: %d, output_index: %d, error: %w",
						txIndex, outputIndex, err)
				}
				resourceCount++
			}
		}
	}

	if rmi.logger != nil {
		rmi.logger.Debugf("完成区块资源元数据索引建立 - height: %d, resource_count: %d",
			block.Header.Height, resourceCount)
	}

	return nil
}

// GetResourceLocation 根据资源内容哈希获取资源位置
func (rmi *ResourceMetadataIndex) GetResourceLocation(ctx context.Context, contentHash []byte) (*ResourceLocation, error) {
	if len(contentHash) == 0 {
		return nil, fmt.Errorf("资源内容哈希不能为空")
	}

	key := formatResourceIndexKey(contentHash)
	data, err := rmi.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("查询资源位置失败: %w", err)
	}

	// 根据BadgerDB接口的设计，键不存在时返回nil值和nil错误
	if data == nil {
		return nil, fmt.Errorf("资源不存在 - content_hash: %x", contentHash)
	}

	location, err := deserializeResourceLocation(data)
	if err != nil {
		return nil, fmt.Errorf("反序列化资源位置失败: %w", err)
	}

	return location, nil
}

// HasResource 检查资源是否存在
func (rmi *ResourceMetadataIndex) HasResource(ctx context.Context, contentHash []byte) (bool, error) {
	if len(contentHash) == 0 {
		return false, fmt.Errorf("资源内容哈希不能为空")
	}

	key := formatResourceIndexKey(contentHash)
	exists, err := rmi.storage.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查资源存在性失败: %w", err)
	}

	return exists, nil
}

// GetResourcesByType 根据资源类型获取资源列表
func (rmi *ResourceMetadataIndex) GetResourcesByType(ctx context.Context, resourceType resource.ResourceCategory) ([][]byte, error) {
	if rmi.logger != nil {
		rmi.logger.Debugf("根据类型查询资源 - type: %s", resourceType.String())
	}

	prefix := formatResourceTypeIndexKey(resourceType)
	results, err := rmi.storage.PrefixScan(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("按类型扫描资源失败: %w", err)
	}

	// 提取内容哈希列表
	contentHashes := make([][]byte, 0, len(results))
	for _, data := range results {
		// 类型索引的值就是内容哈希
		contentHashes = append(contentHashes, data)
	}

	if rmi.logger != nil {
		rmi.logger.Debugf("按类型查询完成 - type: %s, count: %d", resourceType.String(), len(contentHashes))
	}

	return contentHashes, nil
}

// RemoveResourceIndex 移除资源索引（用于区块回滚等场景）
// ⚠️ 【写入边界】此方法只能在ResourceService.RemoveResourceIndex中调用
func (rmi *ResourceMetadataIndex) RemoveResourceIndex(ctx context.Context, tx storage.BadgerTransaction, contentHash []byte, resourceType resource.ResourceCategory) error {
	if len(contentHash) == 0 {
		return fmt.Errorf("资源内容哈希不能为空")
	}

	// 删除主索引
	mainKey := formatResourceIndexKey(contentHash)
	if err := tx.Delete(mainKey); err != nil {
		return fmt.Errorf("删除资源主索引失败: %w", err)
	}

	// 删除类型索引
	typeKey := formatResourceTypeIndexKey(resourceType)
	typeKey = append(typeKey, contentHash...)
	if err := tx.Delete(typeKey); err != nil {
		return fmt.Errorf("删除资源类型索引失败: %w", err)
	}

	if rmi.logger != nil {
		rmi.logger.Debugf("成功删除资源索引 - content_hash: %x, type: %s", contentHash, resourceType.String())
	}

	return nil
}

// indexSingleResource 为单个资源建立索引（内部方法）
func (rmi *ResourceMetadataIndex) indexSingleResource(tx storage.BadgerTransaction, blockHash []byte, txIndex, outputIndex uint32, res *resource.Resource) error {
	// 获取资源内容哈希
	contentHash := res.ContentHash
	if len(contentHash) == 0 {
		return fmt.Errorf("资源内容哈希为空")
	}

	// 创建位置信息
	location := &ResourceLocation{
		BlockHash:   blockHash,
		TxIndex:     txIndex,
		OutputIndex: outputIndex,
	}

	// 存储主索引：content_hash -> location
	if err := rmi.setResourceLocation(tx, contentHash, location); err != nil {
		return fmt.Errorf("存储资源位置索引失败: %w", err)
	}

	// 存储类型索引：type:content_hash -> content_hash
	if err := rmi.setResourceTypeIndex(tx, res.Category, contentHash); err != nil {
		return fmt.Errorf("存储资源类型索引失败: %w", err)
	}

	if rmi.logger != nil {
		rmi.logger.Debugf("成功建立资源索引 - content_hash: %x, type: %s, tx_index: %d, output_index: %d",
			contentHash, res.Category.String(), txIndex, outputIndex)
	}

	return nil
}

// setResourceLocation 设置资源位置（内部方法）
func (rmi *ResourceMetadataIndex) setResourceLocation(tx storage.BadgerTransaction, contentHash []byte, location *ResourceLocation) error {
	key := formatResourceIndexKey(contentHash)
	data, err := serializeResourceLocation(location)
	if err != nil {
		return fmt.Errorf("序列化资源位置失败: %w", err)
	}

	if err := tx.Set(key, data); err != nil {
		return fmt.Errorf("存储资源位置失败: %w", err)
	}

	return nil
}

// setResourceTypeIndex 设置资源类型索引（内部方法）
func (rmi *ResourceMetadataIndex) setResourceTypeIndex(tx storage.BadgerTransaction, resourceType resource.ResourceCategory, contentHash []byte) error {
	key := formatResourceTypeIndexKey(resourceType)
	key = append(key, contentHash...)

	// 类型索引的值就是内容哈希本身
	if err := tx.Set(key, contentHash); err != nil {
		return fmt.Errorf("存储资源类型索引失败: %w", err)
	}

	return nil
}

// ========== 辅助函数 ==========

// formatResourceIndexKey 格式化资源索引键
func formatResourceIndexKey(contentHash []byte) []byte {
	key := make([]byte, len(ResourceIndexKeyPrefix)+len(contentHash))
	copy(key, []byte(ResourceIndexKeyPrefix))
	copy(key[len(ResourceIndexKeyPrefix):], contentHash)
	return key
}

// formatResourceTypeIndexKey 格式化资源类型索引键前缀
func formatResourceTypeIndexKey(resourceType resource.ResourceCategory) []byte {
	typeStr := resourceType.String()
	key := make([]byte, len(ResourceTypeIndexKeyPrefix)+len(typeStr)+1)
	copy(key, []byte(ResourceTypeIndexKeyPrefix))
	copy(key[len(ResourceTypeIndexKeyPrefix):], []byte(typeStr))
	key[len(ResourceTypeIndexKeyPrefix)+len(typeStr)] = ':'
	return key
}

// serializeResourceLocation 序列化资源位置
func serializeResourceLocation(location *ResourceLocation) ([]byte, error) {
	// 使用简单的二进制格式：[BlockHashLength(4字节)][BlockHash][TxIndex(4字节)][OutputIndex(4字节)]
	blockHashLen := len(location.BlockHash)
	data := make([]byte, 4+blockHashLen+4+4)

	// 写入区块哈希长度
	data[0] = byte(blockHashLen >> 24)
	data[1] = byte(blockHashLen >> 16)
	data[2] = byte(blockHashLen >> 8)
	data[3] = byte(blockHashLen)

	// 写入区块哈希
	copy(data[4:4+blockHashLen], location.BlockHash)

	// 写入交易索引
	offset := 4 + blockHashLen
	data[offset] = byte(location.TxIndex >> 24)
	data[offset+1] = byte(location.TxIndex >> 16)
	data[offset+2] = byte(location.TxIndex >> 8)
	data[offset+3] = byte(location.TxIndex)

	// 写入输出索引
	offset += 4
	data[offset] = byte(location.OutputIndex >> 24)
	data[offset+1] = byte(location.OutputIndex >> 16)
	data[offset+2] = byte(location.OutputIndex >> 8)
	data[offset+3] = byte(location.OutputIndex)

	return data, nil
}

// deserializeResourceLocation 反序列化资源位置
func deserializeResourceLocation(data []byte) (*ResourceLocation, error) {
	if len(data) < 12 { // 至少需要 4字节(BlockHashLength) + 4字节(TxIndex) + 4字节(OutputIndex)
		return nil, fmt.Errorf("数据长度不足")
	}

	// 读取区块哈希长度
	blockHashLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if blockHashLen < 0 || len(data) < 4+blockHashLen+4+4 {
		return nil, fmt.Errorf("数据格式错误")
	}

	// 读取区块哈希
	blockHash := make([]byte, blockHashLen)
	copy(blockHash, data[4:4+blockHashLen])

	// 读取交易索引
	offset := 4 + blockHashLen
	txIndex := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 | uint32(data[offset+3])

	// 读取输出索引
	offset += 4
	outputIndex := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 | uint32(data[offset+3])

	return &ResourceLocation{
		BlockHash:   blockHash,
		TxIndex:     txIndex,
		OutputIndex: outputIndex,
	}, nil
}
