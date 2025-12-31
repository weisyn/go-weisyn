// Package query 提供资源 UTXO 查询服务实现（基于实例索引的彻底版本）
//
// ⚠️ **Phase 4：彻底迭代**
// - 只使用基于 ResourceInstanceId 的新索引
// - 不再依赖任何旧的 contentHash → 单实例 索引
// - contentHash 仅作为 ResourceCodeId（代码维度）使用
package query

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/internal/core/eutxo/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ResourceService 资源 UTXO 查询服务
//
// 🎯 **核心职责**：
// - 基于 ResourceInstanceId（OutPoint）和 ResourceCodeId（ContentHash）提供资源 UTXO 查询能力
// - 支持多实例部署场景：1 个 CodeId → N 个 InstanceId
//
// ⚠️ **实现约束**：
// - 只使用如下键空间：
//   - indices:resource-instance:{instanceID}
//   - resource:utxo-instance:{instanceID}
//   - indices:resource-code:{codeID}
//   - index:resource:owner-instance:{owner}:{instanceID}
//   - resource:counters-instance:{instanceID}
type ResourceService struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewResourceService 创建资源 UTXO 查询服务
//
// 实现 interfaces.InternalResourceUTXOQuery
func NewResourceService(storage storage.BadgerStore, logger log.Logger) (interfaces.InternalResourceUTXOQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}

	s := &ResourceService{
		storage: storage,
		logger:  logger,
	}

	if logger != nil {
		logger.Info("✅ ResourceUTXOQuery 服务已创建（实例索引版）")
	}

	return s, nil
}

// GetResourceUTXOByContentHash 根据内容哈希查询资源 UTXO
//
// ⚠️ **Phase 4 彻底迭代**：
// - 不再使用旧的 resource:utxo:{contentHash} 索引
// - 通过代码→实例索引获取该代码的第一个实例，作为兼容行为
func (s *ResourceService) GetResourceUTXOByContentHash(ctx context.Context, contentHash []byte) (*eutxo.ResourceUTXORecord, bool, error) {
	if len(contentHash) != 32 {
		return nil, false, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	records, err := s.ListResourceInstancesByCode(ctx, contentHash)
	if err != nil {
		return nil, false, fmt.Errorf("查询资源实例列表失败: %w", err)
	}

	if len(records) == 0 {
		return nil, false, nil
	}

	// 兼容行为：返回第一个实例
	return records[0], true, nil
}

// GetResourceUTXOByInstance 根据资源实例标识查询资源 UTXO
//
// 实现 eutxo.ResourceUTXOQuery.GetResourceUTXOByInstance
// ⚠️ **标识协议对齐**：使用 ResourceInstanceId（OutPoint）作为主键
func (s *ResourceService) GetResourceUTXOByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*eutxo.ResourceUTXORecord, bool, error) {
	if len(txHash) != 32 {
		return nil, false, fmt.Errorf("txHash 必须是 32 字节，实际: %d", len(txHash))
	}

	instanceID := eutxo.NewResourceInstanceID(txHash, outputIndex)
	key := fmt.Sprintf("resource:utxo-instance:%s", instanceID.Encode())

	data, err := s.storage.Get(ctx, []byte(key))
	if err != nil {
		return nil, false, fmt.Errorf("查询资源 UTXO 失败: %w", err)
	}

	if data == nil || len(data) == 0 {
		return nil, false, nil
	}

	record := &eutxo.ResourceUTXORecord{}
	if err := json.Unmarshal(data, record); err != nil {
		return nil, false, fmt.Errorf("反序列化资源 UTXO 记录失败: %w", err)
	}

	// 旧数据兼容：如果 InstanceID/CodeID 为空，则从旧字段恢复
	if len(record.InstanceID.TxId) == 0 && len(record.TxId) == 32 {
		record.InstanceID = eutxo.NewResourceInstanceID(record.TxId, record.OutputIndex)
	}
	if len(record.CodeID) == 0 && len(record.ContentHash) == 32 {
		record.CodeID = eutxo.NewResourceCodeID(record.ContentHash)
	}

	return record, true, nil
}

// ListResourceInstancesByCode 列出指定代码的所有实例
//
// 实现 eutxo.ResourceUTXOQuery.ListResourceInstancesByCode
// ⚠️ **标识协议对齐**：展示 ResourceCodeId → ResourceInstanceId 的 1:N 关系
func (s *ResourceService) ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*eutxo.ResourceUTXORecord, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	codeIndexKey := fmt.Sprintf("indices:resource-code:%x", contentHash)

	data, err := s.storage.Get(ctx, []byte(codeIndexKey))
	if err != nil || len(data) == 0 {
		// 索引不存在或无数据，返回空列表
		return []*eutxo.ResourceUTXORecord{}, nil
	}

	var instanceList []string
	if err := json.Unmarshal(data, &instanceList); err != nil {
		return nil, fmt.Errorf("解析代码→实例索引失败: %w", err)
	}

	records := make([]*eutxo.ResourceUTXORecord, 0, len(instanceList))
	for _, instanceIDStr := range instanceList {
		txHash, outputIndex, err := eutxo.DecodeInstanceID(instanceIDStr)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("解码实例 ID 失败: instanceID=%s, error=%v", instanceIDStr, err)
			}
			continue
		}

		record, exists, err := s.GetResourceUTXOByInstance(ctx, txHash, outputIndex)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询实例记录失败: instanceID=%s, error=%v", instanceIDStr, err)
			}
			continue
		}

		if exists {
			records = append(records, record)
		}
	}

	return records, nil
}

// ListResourceUTXOs 列出资源 UTXO 列表
//
// 实现 eutxo.ResourceUTXOQuery.ListResourceUTXOs
// ⚠️ **Phase 4 彻底迭代**：只使用实例索引，不再使用旧索引
func (s *ResourceService) ListResourceUTXOs(ctx context.Context, filter eutxo.ResourceUTXOFilter, offset, limit int) ([]*eutxo.ResourceUTXORecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var prefix string
	if len(filter.Owner) > 0 {
		// Owner 实例索引：index:resource:owner-instance:{owner}:{instanceID}
		prefix = fmt.Sprintf("index:resource:owner-instance:%x:", filter.Owner)
	} else {
		// 实例 UTXO 记录：resource:utxo-instance:{instanceID}
		prefix = "resource:utxo-instance:"
	}

	results, err := s.storage.PrefixScan(ctx, []byte(prefix))
	if err != nil {
		return nil, fmt.Errorf("扫描资源 UTXO 失败: %w", err)
	}

	records := make([]*eutxo.ResourceUTXORecord, 0)
	for keyStr, value := range results {
		_ = keyStr // 当前实现中不解析 key

		if len(filter.Owner) > 0 {
			// Owner 索引：值为 instanceID
			instanceIDStr := string(value)
			txHash, outputIndex, err := eutxo.DecodeInstanceID(instanceIDStr)
			if err != nil {
				if s.logger != nil {
					s.logger.Warnf("解码实例 ID 失败: instanceID=%s, error=%v", instanceIDStr, err)
				}
				continue
			}

			record, exists, err := s.GetResourceUTXOByInstance(ctx, txHash, outputIndex)
			if err != nil || !exists {
				continue
			}

			if s.matchesFilter(record, filter) {
				records = append(records, record)
			}
		} else {
			// 直接反序列化实例记录
			record := &eutxo.ResourceUTXORecord{}
			if err := json.Unmarshal(value, record); err != nil {
				continue
			}

			if s.matchesFilter(record, filter) {
				records = append(records, record)
			}
		}
	}

	// 分页
	start := offset
	end := offset + limit
	if start > len(records) {
		return []*eutxo.ResourceUTXORecord{}, nil
	}
	if end > len(records) {
		end = len(records)
	}

	return records[start:end], nil
}

// GetResourceUsageCounters 获取资源使用统计
//
// 实现 eutxo.ResourceUTXOQuery.GetResourceUsageCounters
// ⚠️ **Phase 4 彻底迭代**：通过代码的第一个实例获取统计
func (s *ResourceService) GetResourceUsageCounters(ctx context.Context, contentHash []byte) (*eutxo.ResourceUsageCounters, bool, error) {
	if len(contentHash) != 32 {
		return nil, false, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	instances, err := s.ListResourceInstancesByCode(ctx, contentHash)
	if err != nil {
		return nil, false, fmt.Errorf("查询资源实例列表失败: %w", err)
	}

	if len(instances) == 0 {
		// 返回代码级默认统计
		codeID := eutxo.NewResourceCodeID(contentHash)
		counters := &eutxo.ResourceUsageCounters{
			InstanceID:               eutxo.ResourceInstanceID{}, // 空实例
			CodeID:                   codeID,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
		counters.EnsureBackwardCompatibility()
		return counters, false, nil
	}

	first := instances[0]
	return s.GetResourceUsageCountersByInstance(ctx, first.TxId, first.OutputIndex)
}

// GetResourceUsageCountersByInstance 根据资源实例标识获取使用统计
//
// 实现 eutxo.ResourceUTXOQuery.GetResourceUsageCountersByInstance
// ⚠️ **标识协议对齐**：使用 ResourceInstanceId 作为主键，确保每个实例有独立统计
func (s *ResourceService) GetResourceUsageCountersByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*eutxo.ResourceUsageCounters, bool, error) {
	if len(txHash) != 32 {
		return nil, false, fmt.Errorf("txHash 必须是 32 字节，实际: %d", len(txHash))
	}

	instanceID := eutxo.NewResourceInstanceID(txHash, outputIndex)
	key := fmt.Sprintf("resource:counters-instance:%s", instanceID.Encode())

	data, err := s.storage.Get(ctx, []byte(key))
	if err != nil {
		return nil, false, fmt.Errorf("查询资源使用统计失败: %w", err)
	}

	if data == nil || len(data) == 0 {
		// 返回默认值（引用计数为 0）
		counters := &eutxo.ResourceUsageCounters{
			InstanceID:               instanceID,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
		counters.EnsureBackwardCompatibility()
		return counters, false, nil
	}

	counters := &eutxo.ResourceUsageCounters{}
	if err := json.Unmarshal(data, counters); err != nil {
		return nil, false, fmt.Errorf("反序列化资源使用统计失败: %w", err)
	}

	// 旧数据兼容：从旧字段恢复新字段
	if len(counters.InstanceID.TxId) == 0 && len(counters.InstanceTxId) == 32 {
		counters.InstanceID = eutxo.NewResourceInstanceID(counters.InstanceTxId, counters.InstanceIndex)
	}
	if len(counters.CodeID) == 0 && len(counters.ContentHash) == 32 {
		counters.CodeID = eutxo.NewResourceCodeID(counters.ContentHash)
	}

	// 确保兼容字段存在（便于旧测试/调用方使用）
	counters.EnsureBackwardCompatibility()

	return counters, true, nil
}

// matchesFilter 检查记录是否匹配过滤条件
func (s *ResourceService) matchesFilter(record *eutxo.ResourceUTXORecord, filter eutxo.ResourceUTXOFilter) bool {
	// Owner 过滤（如果提供）
	if len(filter.Owner) > 0 {
		if len(record.Owner) != len(filter.Owner) {
			return false
		}
		for i := range record.Owner {
			if record.Owner[i] != filter.Owner[i] {
				return false
			}
		}
	}

	// Status 过滤
	if filter.Status != nil && record.Status != *filter.Status {
		return false
	}

	// 时间范围过滤
	if filter.MinCreationTimestamp != nil && record.CreationTimestamp < *filter.MinCreationTimestamp {
		return false
	}
	if filter.MaxCreationTimestamp != nil && record.CreationTimestamp > *filter.MaxCreationTimestamp {
		return false
	}

	return true
}
