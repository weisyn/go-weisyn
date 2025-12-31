// Package writer 实现 UTXO 变更写入逻辑
//
// 💰 **UTXO 变更写入 (UTXO Changes Writing)**
//
// 本文件实现 UTXO 变更的写入逻辑，处理交易输入和输出。
//
// 🎯 **核心职责**：
// - 处理交易输入（删除 UTXO，记录花费历史）
// - 处理交易输出（创建 UTXO，更新地址索引）
// - 更新 Nonce 索引
//
// ⚠️ **关键原则**：
// - UTXO 从交易中提取
// - 处理输入时删除 UTXO，记录花费历史
// - 处理输出时创建 UTXO，更新地址索引
// - ✅ **架构修复**：直接操作存储，不依赖业务层组件（eutxo.UTXOWriter）
package writer

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	eutxoiface "github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

// writeUTXOChanges 处理 UTXO 变更
//
// 🎯 **核心职责**：
// 从区块的交易中提取 UTXO 变更，直接操作存储更新 UTXO 集合。
//
// 📋 **处理流程**：
// 1. 遍历区块中的所有交易
// 2. 处理交易输入：
//   - 直接删除 UTXO（utxo:set:{outpoint}）
//   - 记录花费历史（utxo:spent:{txHash}:{outputIndex}）
//   - 更新地址索引（从地址索引中移除）
//   - 检查是否消费了引用交易，收集需要减少引用的资源UTXO
//
// 3. 处理交易输出：
//   - 构建完整的 UTXO 对象
//   - 直接存储 UTXO（utxo:set:{outpoint}）
//   - 更新地址索引（添加到地址索引）
//
// ⚠️ **关键原则**：
// - UTXO 从交易中提取
// - ✅ **架构修复**：直接操作存储，不依赖业务层组件
// - 所有操作在事务中完成
// - 引用计数管理在事务提交后通过回调处理
//
// 参数：
//   - ctx: 上下文
//   - tx: BadgerDB 事务
//   - block: 区块数据
func (s *Service) writeUTXOChanges(
	ctx context.Context,
	tx storage.BadgerTransaction,
	block *core.Block,
) error {
	if s.txHashClient == nil {
		return fmt.Errorf("txHashClient 未初始化")
	}

	transactions := block.Body.Transactions
	if transactions == nil {
		return nil
	}

	// 遍历区块中的所有交易
	for i, txProto := range transactions {
		// 计算交易哈希（用于构建 OutPoint，使用 gRPC 服务）
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

		// 1. 处理交易输入
		for _, input := range txProto.Inputs {
			if input.PreviousOutput == nil {
				continue
			}

			// ✅ 处理引用型输入（is_reference_only=true）
			if input.IsReferenceOnly {
				// 引用型输入：不删除 UTXO，仅更新资源使用统计（不形成跨区块锁定语义）
				if err := s.recordReferenceOnlyUsageInTransaction(ctx, tx, input.PreviousOutput, block.Header.Height, block.Header.Timestamp); err != nil {
					return fmt.Errorf("记录引用型输入使用统计失败（交易 %d）: %w", i, err)
				}
				continue
			}

			// 消费型输入：删除 UTXO
			// ✅ 架构修复：直接操作存储，不调用 eutxo.UTXOWriter
			if err := s.deleteUTXOInTransaction(ctx, tx, input.PreviousOutput); err != nil {
				return fmt.Errorf("删除 UTXO 失败（交易 %d，输入）: %w", i, err)
			}

			// 记录花费历史（utxo:spent:{txHash}:{outputIndex}）
			spentKey := fmt.Sprintf("utxo:spent:%x:%d", input.PreviousOutput.TxId, input.PreviousOutput.OutputIndex)
			spentValue := make([]byte, 32+8)
			copy(spentValue[0:32], txHash)
			copy(spentValue[32:40], uint64ToBytes(block.Header.Height))
			if err := tx.Set([]byte(spentKey), spentValue); err != nil {
				return fmt.Errorf("记录花费历史失败（交易 %d）: %w", i, err)
			}
		}

		// 2. 处理交易输出（创建 UTXO）
		for j, output := range txProto.Outputs {
			if output == nil {
				continue
			}

			// 构建完整的 UTXO 对象
			var category utxo.UTXOCategory
			if output.GetAsset() != nil {
				category = utxo.UTXOCategory_UTXO_CATEGORY_ASSET
			} else if output.GetResource() != nil {
				category = utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE
			} else if output.GetState() != nil {
				category = utxo.UTXOCategory_UTXO_CATEGORY_STATE
			} else {
				category = utxo.UTXOCategory_UTXO_CATEGORY_UNKNOWN
			}

			utxoObj := &utxo.UTXO{
				Outpoint: &transaction.OutPoint{
					TxId:        txHash,
					OutputIndex: uint32(j),
				},
				Category:     category,
				OwnerAddress: output.Owner,
				BlockHeight:  block.Header.Height,
				Status:       utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: output,
				},
			}

			// ✅ 架构修复：直接操作存储，不调用 eutxo.UTXOWriter
			if err := s.createUTXOInTransaction(ctx, tx, utxoObj); err != nil {
				return fmt.Errorf("创建 UTXO 失败（交易 %d，输出 %d）: %w", i, j, err)
			}
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 变更已处理: height=%d, txCount=%d",
			block.Header.Height, len(transactions))
	}

	return nil
}

// recordReferenceOnlyUsageInTransaction 记录一次引用型输入（is_reference_only=true）的使用统计。
//
// 彻底迭代语义：
// - 引用型输入是“只读依赖”，不形成跨区块的锁定语义；
// - 因此不再维护“会影响删除/消费”的引用计数门闸；
// - 仅更新 ResourceUsageCounters：TotalReferenceTimes + LastReference*，用于观测/统计。
func (s *Service) recordReferenceOnlyUsageInTransaction(
	ctx context.Context,
	tx storage.BadgerTransaction,
	outpoint *transaction.OutPoint,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	if outpoint == nil || len(outpoint.TxId) != 32 {
		return fmt.Errorf("invalid outpoint")
	}

	// 1) 确认 referenced UTXO 存在且为 ResourceOutput（否则引用型输入无意义，应视为无效区块）
	utxoKey := buildUTXOKey(outpoint)
	utxoBytes, err := tx.Get([]byte(utxoKey))
	if err != nil || len(utxoBytes) == 0 {
		return fmt.Errorf("referenced utxo not found: %s", utxoKey)
	}
	utxoObj := &utxo.UTXO{}
	if err := proto.Unmarshal(utxoBytes, utxoObj); err != nil {
		return fmt.Errorf("unmarshal referenced utxo failed: %w", err)
	}
	cached := utxoObj.GetCachedOutput()
	if cached == nil || cached.GetResource() == nil {
		return fmt.Errorf("referenced utxo is not ResourceOutput: %s", utxoKey)
	}

	// 2) 获取/初始化 counters
	instanceID := eutxoiface.NewResourceInstanceID(outpoint.TxId, uint32(outpoint.OutputIndex))
	countersKey := fmt.Sprintf("resource:counters-instance:%s", instanceID.Encode())

	counters := &eutxoiface.ResourceUsageCounters{}
	data, _ := tx.Get([]byte(countersKey))
	if len(data) > 0 {
		_ = json.Unmarshal(data, counters)
	}

	// 如果 counters 缺少 InstanceID/CodeID，则尝试从 resource:utxo-instance 记录恢复
	if len(counters.InstanceID.TxId) == 0 || len(counters.CodeID) == 0 {
		recordKey := fmt.Sprintf("resource:utxo-instance:%s", instanceID.Encode())
		recordBytes, rerr := tx.Get([]byte(recordKey))
		if rerr != nil || len(recordBytes) == 0 {
			return fmt.Errorf("missing ResourceUTXORecord for counters init: %s", recordKey)
		}
		record := &eutxoiface.ResourceUTXORecord{}
		if err := json.Unmarshal(recordBytes, record); err != nil {
			return fmt.Errorf("unmarshal ResourceUTXORecord failed: %w", err)
		}
		// 旧数据兼容：如果 InstanceID/CodeID 为空，则从旧字段恢复
		if len(record.InstanceID.TxId) == 0 && len(record.TxId) == 32 {
			record.InstanceID = eutxoiface.NewResourceInstanceID(record.TxId, record.OutputIndex)
		}
		if len(record.CodeID) == 0 && len(record.ContentHash) == 32 {
			record.CodeID = eutxoiface.NewResourceCodeID(record.ContentHash)
		}
		counters.InstanceID = record.InstanceID
		counters.CodeID = record.CodeID
	}

	// 3) 更新统计字段
	counters.TotalReferenceTimes++
	counters.LastReferenceBlockHeight = blockHeight
	counters.LastReferenceTimestamp = blockTimestamp
	counters.EnsureBackwardCompatibility()

	// 4) 写回
	encoded, err := json.Marshal(counters)
	if err != nil {
		return fmt.Errorf("marshal ResourceUsageCounters failed: %w", err)
	}
	if err := tx.Set([]byte(countersKey), encoded); err != nil {
		return fmt.Errorf("write ResourceUsageCounters failed: %w", err)
	}
	return nil
}

// createUTXOInTransaction 在事务中创建 UTXO
//
// 🎯 **核心职责**：
// 直接操作存储，创建 UTXO 并更新索引。
//
// ⚠️ **架构修复**：
// 此方法直接操作存储，不依赖业务层组件（eutxo.UTXOWriter）。
//
// 参数：
//   - ctx: 上下文
//   - tx: BadgerDB 事务
//   - utxoObj: UTXO 对象
func (s *Service) createUTXOInTransaction(
	ctx context.Context,
	tx storage.BadgerTransaction,
	utxoObj *utxo.UTXO,
) error {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return fmt.Errorf("无效的 UTXO 对象")
	}

	// 1. 序列化 UTXO
	utxoData, err := proto.Marshal(utxoObj)
	if err != nil {
		return fmt.Errorf("序列化 UTXO 失败: %w", err)
	}

	// 2. 构造存储键（utxo:set:{txHash}:{outputIndex}）
	utxoKey := buildUTXOKey(utxoObj.Outpoint)

	// 3. 在事务中存储 UTXO
	if err := tx.Set([]byte(utxoKey), utxoData); err != nil {
		return fmt.Errorf("存储 UTXO 失败: %w", err)
	}

	// 4. 🔧 更新地址索引（index:address:{address} -> []outpoint）
	if err := s.addToAddressIndexInTransaction(tx, utxoObj); err != nil {
		// 索引更新失败不应该阻止 UTXO 创建，记录警告即可
		if s.logger != nil {
			s.logger.Warnf("更新地址索引失败: %v", err)
		}
	}

	return nil
}

// deleteUTXOInTransaction 在事务中删除 UTXO
//
// 🎯 **核心职责**：
// 直接操作存储，删除 UTXO 并更新索引。
//
// ⚠️ **架构修复**：
// 此方法直接操作存储，不依赖业务层组件（eutxo.UTXOWriter）。
//
// ⚠️ **注意事项**：
// - 不检查引用计数（引用计数检查应在业务层完成）
// - 直接删除 UTXO 和索引
//
// 参数：
//   - ctx: 上下文
//   - tx: BadgerDB 事务
//   - outpoint: UTXO 的输出点
func (s *Service) deleteUTXOInTransaction(
	ctx context.Context,
	tx storage.BadgerTransaction,
	outpoint *transaction.OutPoint,
) error {
	if outpoint == nil || outpoint.TxId == nil {
		return fmt.Errorf("无效的 OutPoint")
	}

	// 1. 构造存储键
	utxoKey := buildUTXOKey(outpoint)

	// 2. 先获取 UTXO 对象（用于索引移除）
	var utxoObj *utxo.UTXO
	data, err := tx.Get([]byte(utxoKey))
	if err == nil && len(data) > 0 {
		tempObj := &utxo.UTXO{}
		if err := proto.Unmarshal(data, tempObj); err == nil {
			utxoObj = tempObj
		} else {
			// 反序列化失败，记录警告但继续删除操作
			// 因为即使无法读取UTXO，删除操作也应该继续
		}
	}

	// 3. 在事务中删除 UTXO
	if err := tx.Delete([]byte(utxoKey)); err != nil {
		return fmt.Errorf("删除 UTXO 失败: %w", err)
	}

	// 4. 从地址索引移除
	if utxoObj != nil {
		if err := s.removeFromAddressIndexInTransaction(tx, utxoObj); err != nil {
			// 索引更新失败不应该阻止 UTXO 删除，记录警告即可
			if s.logger != nil {
				s.logger.Warnf("移除地址索引失败: %v", err)
			}
		}
	}

	return nil
}

// addToAddressIndexInTransaction 在事务中添加 UTXO 到地址索引
//
// 🔧 索引格式：index:address:{address} -> []outpoint（每个 outpoint 为 36 字节：32字节 txHash + 4字节 outputIndex）
func (s *Service) addToAddressIndexInTransaction(tx storage.BadgerTransaction, utxoObj *utxo.UTXO) error {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return nil
	}

	output := utxoObj.GetCachedOutput()
	if output == nil || len(output.Owner) == 0 {
		return nil
	}

	// 🔧 修复：使用统一的地址索引键格式（与查询层保持一致）
	addressKey := fmt.Sprintf("index:address:%x", output.Owner)

	// 编码 outpoint（32字节 txHash + 4字节 outputIndex）
	outpointBytes := make([]byte, 36)
	copy(outpointBytes[0:32], utxoObj.Outpoint.TxId)
	binary.BigEndian.PutUint32(outpointBytes[32:36], utxoObj.Outpoint.OutputIndex)

	// 读取现有索引
	existingData, err := tx.Get([]byte(addressKey))
	var existingOutpoints []byte
	if err == nil && len(existingData) > 0 {
		existingOutpoints = existingData
	}

	// 检查是否已存在（避免重复）
	if len(existingOutpoints) > 0 {
		for i := 0; i < len(existingOutpoints); i += 36 {
			if i+36 <= len(existingOutpoints) {
				if string(existingOutpoints[i:i+36]) == string(outpointBytes) {
					// 已存在，不重复添加
					return nil
				}
			}
		}
	}

	// 追加新的 outpoint
	newOutpoints := append(existingOutpoints, outpointBytes...)
	return tx.Set([]byte(addressKey), newOutpoints)
}

// removeFromAddressIndexInTransaction 在事务中从地址索引移除 UTXO
func (s *Service) removeFromAddressIndexInTransaction(tx storage.BadgerTransaction, utxoObj *utxo.UTXO) error {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return nil
	}

	output := utxoObj.GetCachedOutput()
	if output == nil || len(output.Owner) == 0 {
		return nil
	}

	// 🔧 修复：使用统一的地址索引键格式（与查询层保持一致）
	addressKey := fmt.Sprintf("index:address:%x", output.Owner)

	// 编码 outpoint
	outpointBytes := make([]byte, 36)
	copy(outpointBytes[0:32], utxoObj.Outpoint.TxId)
	binary.BigEndian.PutUint32(outpointBytes[32:36], utxoObj.Outpoint.OutputIndex)

	// 读取现有索引
	existingData, err := tx.Get([]byte(addressKey))
	if err != nil || len(existingData) == 0 {
		return nil // 索引不存在，无需移除
	}

	// 查找并移除 outpoint
	var newOutpoints []byte
	for i := 0; i < len(existingData); i += 36 {
		if i+36 <= len(existingData) {
			existingOutpoint := existingData[i : i+36]
			if string(existingOutpoint) != string(outpointBytes) {
				newOutpoints = append(newOutpoints, existingOutpoint...)
			}
		}
	}

	// 更新索引
	if len(newOutpoints) == 0 {
		return tx.Delete([]byte(addressKey))
	}
	return tx.Set([]byte(addressKey), newOutpoints)
}

// buildUTXOKey 构造 UTXO 存储键
//
// 格式：utxo:set:{txHash}:{outputIndex}
// 符合 docs/system/designs/storage/data-architecture.md 规范
func buildUTXOKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}

// uint32ToBytes 将 uint32 转换为字节数组（辅助函数）
func uint32ToBytes(val uint32) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(val >> 24)
	bytes[1] = byte(val >> 16)
	bytes[2] = byte(val >> 8)
	bytes[3] = byte(val)
	return bytes
}

// bytesToUint32 将字节数组转换为 uint32（辅助函数）
func bytesToUint32(bytes []byte) uint32 {
	if len(bytes) < 4 {
		return 0
	}
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}
