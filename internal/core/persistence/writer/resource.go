// Package writer 实现资源索引更新逻辑
//
// 📁 **资源索引更新 (Resource Index Update)**
//
// 本文件实现资源索引的更新逻辑，扫描区块中的资源相关交易并更新索引。
//
// 🎯 **核心职责**：
// - 扫描区块中的资源相关交易
// - 更新资源索引（contentHash → txHash）
//
// ⚠️ **关键原则**：
// - 资源文件存储在文件系统中（由 ResourceWriter 负责）
// - 资源索引存储在 BadgerDB 中（由 DataWriter 统一处理）
// - 索引格式：indices:resource:{contentHash} → (txHash, blockHash, blockHeight)
package writer

import (
	"context"
	"encoding/json"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// writeResourceIndices 更新资源索引
//
// 🎯 **核心职责**：
// 扫描区块中的资源相关交易，更新资源索引。
//
// 📋 **处理流程**：
// 1. 遍历区块中的所有交易
// 2. 识别资源相关交易（包含 ResourceOutput 的交易）
// 3. 提取资源内容哈希（从 ResourceOutput.Resource.ContentHash）
// 4. 更新资源索引（indices:resource:{contentHash} → txHash + blockHash + blockHeight）
//
// ⚠️ **注意事项**：
// - 资源文件存储在文件系统中（由 ResourceWriter.StoreResourceFile() 负责）
// - 只更新资源索引（统一由 DataWriter 在事务中处理）
// - 索引格式：indices:resource:{contentHash} → txHash(32字节) + blockHash(32字节) + blockHeight(8字节)
func (s *Service) writeResourceIndices(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	transactions := block.Body.Transactions
	if transactions == nil {
		return nil
	}

	// 计算区块哈希（用于资源索引，使用 gRPC 服务）
	if s.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化")
	}
	blockReq := &core.ComputeBlockHashRequest{
		Block: block,
	}
	blockResp, err := s.blockHashClient.ComputeBlockHash(ctx, blockReq)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}
	if !blockResp.IsValid {
		return fmt.Errorf("区块结构无效")
	}
	blockHash := blockResp.Hash

	// 遍历区块中的所有交易
	for i, txProto := range transactions {
		// 计算交易哈希（使用 gRPC 服务）
		if s.txHashClient == nil {
			return fmt.Errorf("txHashClient 未初始化")
		}
		txReq := &transaction.ComputeHashRequest{
			Transaction: txProto,
		}
		txResp, err := s.txHashClient.ComputeHash(ctx, txReq)
		if err != nil {
			return fmt.Errorf("计算交易哈希失败（交易 %d）: %w", i, err)
		}
		if !txResp.IsValid {
			return fmt.Errorf("交易 %d 结构无效", i)
		}
		txHash := txResp.Hash

		// 遍历交易输出，查找 ResourceOutput
		for j, output := range txProto.Outputs {
			if output == nil {
				continue
			}

			// 检查是否是 ResourceOutput（使用 GetResource() 方法）
			resourceOutput := output.GetResource()
			if resourceOutput == nil {
				continue
			}

			// 提取资源内容哈希
			if resourceOutput.Resource == nil {
				continue
			}

			contentHash := resourceOutput.Resource.ContentHash
			if len(contentHash) == 0 {
				// 如果没有内容哈希，跳过
				continue
			}

			// 编码资源索引值：txHash(32字节) + blockHash(32字节) + blockHeight(8字节)
			indexValue := make([]byte, 32+32+8)
			copy(indexValue[0:32], txHash)
			copy(indexValue[32:64], blockHash)
			copy(indexValue[64:72], uint64ToBytes(block.Header.Height))

			// ========== Phase 4: 彻底迭代 - 移除旧索引，只使用实例索引 ==========
			// ⚠️ **彻底迭代**：不再写入 indices:resource:{contentHash}，只写入实例索引
			// 实例索引的写入在 writeResourceUTXOIndex 中完成

			// 同步更新 Resource UTXO 索引（基于 ResourceInstanceId）
			// 以 UTXO 为真相：基于当前交易输出构建 ResourceUTXORecord。
			if err := s.writeResourceUTXOIndex(ctx, tx, txHash, uint32(j), output, resourceOutput, blockHash, block.Header.Height, block.Header.Timestamp); err != nil {
				return fmt.Errorf("更新 ResourceUTXO 索引失败（交易 %d，输出 %d）: %w", i, j, err)
			}
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 资源索引已更新: height=%d",
			block.Header.Height)
	}

	return nil
}

// writeResourceUTXOIndex 更新资源 UTXO 索引（resource:utxo:* + resource:counters:* + index:resource:owner:*）
func (s *Service) writeResourceUTXOIndex(
	ctx context.Context,
	tx storage.BadgerTransaction,
	txHash []byte,
	outputIndex uint32,
	output *transaction.TxOutput,
	resourceOutput *transaction.ResourceOutput,
	blockHash []byte,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	resource := resourceOutput.Resource
	if resource == nil {
		return fmt.Errorf("ResourceOutput.resource 不能为空")
	}

	codeHash := resource.ContentHash
	if len(codeHash) != 32 {
		return fmt.Errorf("codeHash 必须是 32 字节，实际: %d", len(codeHash))
	}

	// ========== Phase 4: 彻底迭代 - 只使用新索引（实例维度）==========
	// ⚠️ **彻底迭代**：移除所有旧索引，只保留基于 ResourceInstanceId 的新索引

	// 1. 构建资源实例标识符和代码标识符
	instanceID := eutxo.NewResourceInstanceID(txHash, outputIndex)
	codeID := eutxo.NewResourceCodeID(codeHash)

	// 2. 构建 ResourceUTXORecord（使用新类型）
	record := &eutxo.ResourceUTXORecord{
		InstanceID:        instanceID,
		CodeID:            codeID,
		Owner:             output.Owner,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: resourceOutput.CreationTimestamp,
		IsImmutable:       resourceOutput.IsImmutable,
	}

	if resourceOutput.ExpiryTimestamp != nil && *resourceOutput.ExpiryTimestamp > 0 {
		expiry := *resourceOutput.ExpiryTimestamp
		record.ExpiryTimestamp = &expiry
		if blockTimestamp >= expiry {
			record.Status = eutxo.ResourceUTXOStatusExpired
		}
	}

	// 确保向后兼容字段被填充（用于序列化）
	record.EnsureBackwardCompatibility()

	// 3. 实例主索引：indices:resource-instance:{instanceID} -> {blockHash, blockHeight, codeID}
	instanceIndexKey := fmt.Sprintf("indices:resource-instance:%s", instanceID.Encode())
	instanceIndexValue := make([]byte, 72) // blockHash(32) + blockHeight(8) + codeID(32)
	copy(instanceIndexValue[0:32], blockHash)
	copy(instanceIndexValue[32:40], uint64ToBytes(blockHeight))
	copy(instanceIndexValue[40:72], codeID.Bytes())
	if err := tx.Set([]byte(instanceIndexKey), instanceIndexValue); err != nil {
		return fmt.Errorf("存储资源实例索引失败: %w", err)
	}

	// 4. 实例 UTXO 记录：resource:utxo-instance:{instanceID} -> ResourceUTXORecord
	instanceRecordKey := fmt.Sprintf("resource:utxo-instance:%s", instanceID.Encode())
	recordData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化 ResourceUTXORecord 失败: %w", err)
	}
	if err := tx.Set([]byte(instanceRecordKey), recordData); err != nil {
		return fmt.Errorf("存储 ResourceUTXORecord 失败: %w", err)
	}

	// 5. 代码→实例索引（1:N 关系）：indices:resource-code:{codeID} -> [instanceID1, instanceID2, ...]
	codeIndexKey := fmt.Sprintf("indices:resource-code:%x", codeID.Bytes())
	existingCodeData, _ := tx.Get([]byte(codeIndexKey))
	var instanceList []string
	if len(existingCodeData) > 0 {
		if err := json.Unmarshal(existingCodeData, &instanceList); err != nil {
			instanceList = []string{instanceID.Encode()}
		} else {
			// 检查是否已存在
			found := false
			instanceIDStr := instanceID.Encode()
			for _, id := range instanceList {
				if id == instanceIDStr {
					found = true
					break
				}
			}
			if !found {
				instanceList = append(instanceList, instanceIDStr)
			}
		}
	} else {
		instanceList = []string{instanceID.Encode()}
	}
	codeIndexValue, err := json.Marshal(instanceList)
	if err != nil {
		return fmt.Errorf("序列化代码→实例索引失败: %w", err)
	}
	if err := tx.Set([]byte(codeIndexKey), codeIndexValue); err != nil {
		return fmt.Errorf("存储代码→实例索引失败: %w", err)
	}

	// 6. Owner 索引：index:resource:owner-instance:{owner}:{instanceID} -> instanceID
	if len(output.Owner) > 0 {
		ownerIndexKey := fmt.Sprintf("index:resource:owner-instance:%x:%s", output.Owner, instanceID.Encode())
		if err := tx.Set([]byte(ownerIndexKey), []byte(instanceID.Encode())); err != nil {
			return fmt.Errorf("更新 owner 索引失败: %w", err)
		}
	}

	// 7. 使用计数：resource:counters-instance:{instanceID} -> ResourceUsageCounters
	countersKey := fmt.Sprintf("resource:counters-instance:%s", instanceID.Encode())
		counters := &eutxo.ResourceUsageCounters{
		InstanceID:            instanceID,
		CodeID:               codeID,
		CurrentReferenceCount: 0,
		TotalReferenceTimes:  0,
			LastReferenceBlockHeight: blockHeight,
			LastReferenceTimestamp:   blockTimestamp,
		}
	// 确保向后兼容字段被填充
	counters.EnsureBackwardCompatibility()

	countersData, err := json.Marshal(counters)
		if err != nil {
			return fmt.Errorf("序列化 ResourceUsageCounters 失败: %w", err)
		}
		if err := tx.Set([]byte(countersKey), countersData); err != nil {
			return fmt.Errorf("存储 ResourceUsageCounters 失败: %w", err)
	}

	return nil
}
