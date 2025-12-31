// Package shared 提供 EUTXO 模块的共享工具
package shared

import (
	"context"
	"encoding/binary"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// IndexManager UTXO 索引管理器
//
// 🎯 **设计目的**：
// - 维护 UTXO 索引，加速查询
// - 支持按高度、地址、资产ID等维度查询
//
// 💡 **索引类型**：
// - 地址索引：index:address:{address} -> []outpoint
// - 高度索引：index:height:{height} -> []outpoint
// - 资产索引：index:asset:{assetId} -> []outpoint
type IndexManager struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewIndexManager 创建索引管理器
func NewIndexManager(storage storage.BadgerStore, logger log.Logger) *IndexManager {
	return &IndexManager{
		storage: storage,
		logger:  logger,
	}
}

// AddUTXO 添加 UTXO 到索引
//
// 🎯 **索引维护**：
// 1. 按地址索引：添加 outpoint 到地址索引（P0 修复：实现地址索引）
// 2. 按资产索引：添加 outpoint 到资产索引
// 3. 按高度索引：添加 outpoint 到高度索引（如果UTXO有高度信息）
func (m *IndexManager) AddUTXO(utxoObj *utxo.UTXO) {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return
	}

	ctx := context.Background()
	outpointBytes := m.encodeOutPoint(utxoObj.Outpoint)

	// 1. 按地址索引（P0 修复：使用 TxOutput.owner 字段）
	if output := utxoObj.GetCachedOutput(); output != nil {
		if len(output.Owner) > 0 {
			addressKey := m.buildAddressIndexKey(output.Owner)
			if err := m.addToIndex(ctx, addressKey, outpointBytes); err != nil && m.logger != nil {
				m.logger.Warnf("添加地址索引失败: %v", err)
			}
		}
	}

	// 2. 按资产索引
	if output := utxoObj.GetCachedOutput(); output != nil {
		if asset := output.GetAsset(); asset != nil {
			var assetID []byte
			if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
				// 原生币资产ID
				assetID = []byte("native")
			} else if contractToken := asset.GetContractToken(); contractToken != nil {
				// 合约代币资产ID为合约地址
				if len(contractToken.ContractAddress) > 0 {
					assetID = contractToken.ContractAddress
				}
			}

			if len(assetID) > 0 {
				assetKey := m.buildAssetIndexKey(assetID)
				if err := m.addToIndex(ctx, assetKey, outpointBytes); err != nil && m.logger != nil {
					m.logger.Warnf("添加资产索引失败: %v", err)
				}
			}
		}
	}

	// 3. 按高度索引（如果UTXO有高度信息）
	if utxoObj.BlockHeight > 0 {
		heightKey := m.buildHeightIndexKey(utxoObj.BlockHeight)
		if err := m.addToIndex(ctx, heightKey, outpointBytes); err != nil && m.logger != nil {
			m.logger.Warnf("添加高度索引失败: %v", err)
		}
	}
}

// RemoveUTXO 从索引移除 UTXO（已废弃）
//
// ⚠️ **已废弃**：此方法无法完整移除索引，因为缺少 UTXO 的详细信息。
//
// 🎯 **问题说明**：
// 为了从索引中移除 UTXO，需要知道 UTXO 的地址、资产ID、高度等信息。
// 但此方法只接收 outpoint，无法获取这些信息。
//
// 🔄 **替代方案**：
// 请使用 `RemoveUTXOWithDetails` 方法，该方法接收完整的 UTXO 对象，
// 可以完整移除所有相关索引。
//
// 💡 **正确用法**：
//
//	// ❌ 错误：使用 RemoveUTXO（不完整）
//	indexManager.RemoveUTXO(outpoint)
//
//	// ✅ 正确：使用 RemoveUTXOWithDetails（完整）
//	utxoObj, err := getUTXO(outpoint)  // 先获取 UTXO 对象
//	if err == nil {
//	    indexManager.RemoveUTXOWithDetails(ctx, utxoObj)
//	}
//
// ⚠️ **注意**：此方法保留是为了向后兼容，但不会执行任何索引移除操作。
//
// Deprecated: 使用 RemoveUTXOWithDetails 替代
func (m *IndexManager) RemoveUTXO(outpoint *transaction.OutPoint) {
	// 已废弃：无法完整移除索引
	// 请使用 RemoveUTXOWithDetails 方法
	
	if m.logger != nil {
		m.logger.Warnf("⚠️ RemoveUTXO 已废弃，无法完整移除索引。请使用 RemoveUTXOWithDetails。outpoint: %x:%d", 
			outpoint.TxId[:8], outpoint.OutputIndex)
	}
}

// RemoveUTXOWithDetails 从索引移除 UTXO（带详细信息）
//
// 当有UTXO对象时，使用此方法可以完整移除所有相关索引
func (m *IndexManager) RemoveUTXOWithDetails(ctx context.Context, utxoObj *utxo.UTXO) {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return
	}

	outpointBytes := m.encodeOutPoint(utxoObj.Outpoint)

	// 1. 从地址索引移除（P0 修复：实现地址索引移除）
	if output := utxoObj.GetCachedOutput(); output != nil {
		if len(output.Owner) > 0 {
			addressKey := m.buildAddressIndexKey(output.Owner)
			if err := m.removeFromIndex(ctx, addressKey, outpointBytes); err != nil && m.logger != nil {
				m.logger.Warnf("移除地址索引失败: %v", err)
			}
		}
	}

	// 2. 从资产索引移除
	if output := utxoObj.GetCachedOutput(); output != nil {
		if asset := output.GetAsset(); asset != nil {
			var assetID []byte
			if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
				assetID = []byte("native")
			} else if contractToken := asset.GetContractToken(); contractToken != nil {
				if len(contractToken.ContractAddress) > 0 {
					assetID = contractToken.ContractAddress
				}
			}

			if len(assetID) > 0 {
				assetKey := m.buildAssetIndexKey(assetID)
				if err := m.removeFromIndex(ctx, assetKey, outpointBytes); err != nil && m.logger != nil {
					m.logger.Warnf("移除资产索引失败: %v", err)
				}
			}
		}
	}

	// 3. 从高度索引移除
	if utxoObj.BlockHeight > 0 {
		heightKey := m.buildHeightIndexKey(utxoObj.BlockHeight)
		if err := m.removeFromIndex(ctx, heightKey, outpointBytes); err != nil && m.logger != nil {
			m.logger.Warnf("移除高度索引失败: %v", err)
		}
	}
}

// AddUTXOInTransaction 在事务中添加 UTXO 到索引（事务版本）
//
// 🎯 **索引维护**：
// 1. 按地址索引：添加 outpoint 到地址索引
// 2. 按资产索引：添加 outpoint 到资产索引
// 3. 按高度索引：添加 outpoint 到高度索引（如果UTXO有高度信息）
func (m *IndexManager) AddUTXOInTransaction(tx storage.BadgerTransaction, utxoObj *utxo.UTXO) {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return
	}

	outpointBytes := m.encodeOutPoint(utxoObj.Outpoint)

	// 1. 按地址索引
	if output := utxoObj.GetCachedOutput(); output != nil {
		if len(output.Owner) > 0 {
			addressKey := m.buildAddressIndexKey(output.Owner)
			if err := m.addToIndexInTransaction(tx, addressKey, outpointBytes); err != nil && m.logger != nil {
				m.logger.Warnf("添加地址索引失败: %v", err)
			}
		}
	}

	// 2. 按资产索引
	if output := utxoObj.GetCachedOutput(); output != nil {
		if asset := output.GetAsset(); asset != nil {
			var assetID []byte
			if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
				assetID = []byte("native")
			} else if contractToken := asset.GetContractToken(); contractToken != nil {
				if len(contractToken.ContractAddress) > 0 {
					assetID = contractToken.ContractAddress
				}
			}

			if len(assetID) > 0 {
				assetKey := m.buildAssetIndexKey(assetID)
				if err := m.addToIndexInTransaction(tx, assetKey, outpointBytes); err != nil && m.logger != nil {
					m.logger.Warnf("添加资产索引失败: %v", err)
				}
			}
		}
	}

	// 3. 按高度索引（如果UTXO有高度信息）
	if utxoObj.BlockHeight > 0 {
		heightKey := m.buildHeightIndexKey(utxoObj.BlockHeight)
		if err := m.addToIndexInTransaction(tx, heightKey, outpointBytes); err != nil && m.logger != nil {
			m.logger.Warnf("添加高度索引失败: %v", err)
		}
	}
}

// RemoveUTXOWithDetailsInTransaction 在事务中从索引移除 UTXO（带详细信息，事务版本）
//
// 当有UTXO对象时，使用此方法可以完整移除所有相关索引
func (m *IndexManager) RemoveUTXOWithDetailsInTransaction(tx storage.BadgerTransaction, utxoObj *utxo.UTXO) {
	if utxoObj == nil || utxoObj.Outpoint == nil {
		return
	}

	outpointBytes := m.encodeOutPoint(utxoObj.Outpoint)

	// 1. 从地址索引移除
	if output := utxoObj.GetCachedOutput(); output != nil {
		if len(output.Owner) > 0 {
			addressKey := m.buildAddressIndexKey(output.Owner)
			if err := m.removeFromIndexInTransaction(tx, addressKey, outpointBytes); err != nil && m.logger != nil {
				m.logger.Warnf("移除地址索引失败: %v", err)
			}
		}
	}

	// 2. 从资产索引移除
	if output := utxoObj.GetCachedOutput(); output != nil {
		if asset := output.GetAsset(); asset != nil {
			var assetID []byte
			if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
				assetID = []byte("native")
			} else if contractToken := asset.GetContractToken(); contractToken != nil {
				if len(contractToken.ContractAddress) > 0 {
					assetID = contractToken.ContractAddress
				}
			}

			if len(assetID) > 0 {
				assetKey := m.buildAssetIndexKey(assetID)
				if err := m.removeFromIndexInTransaction(tx, assetKey, outpointBytes); err != nil && m.logger != nil {
					m.logger.Warnf("移除资产索引失败: %v", err)
				}
			}
		}
	}

	// 3. 从高度索引移除
	if utxoObj.BlockHeight > 0 {
		heightKey := m.buildHeightIndexKey(utxoObj.BlockHeight)
		if err := m.removeFromIndexInTransaction(tx, heightKey, outpointBytes); err != nil && m.logger != nil {
			m.logger.Warnf("移除高度索引失败: %v", err)
		}
	}
}

// ============================================================================
//                               索引键构建
// ============================================================================

// buildAddressIndexKey 构建地址索引键
// 格式：index:address:{address}
func (m *IndexManager) buildAddressIndexKey(address []byte) []byte {
	return []byte(fmt.Sprintf("index:address:%x", address))
}

// buildHeightIndexKey 构建高度索引键
// 格式：index:height:{height}
func (m *IndexManager) buildHeightIndexKey(height uint64) []byte {
	return []byte(fmt.Sprintf("index:height:%d", height))
}

// buildAssetIndexKey 构建资产索引键
// 格式：index:asset:{assetId}
func (m *IndexManager) buildAssetIndexKey(assetID []byte) []byte {
	return []byte(fmt.Sprintf("index:asset:%x", assetID))
}

// ============================================================================
//                               索引操作
// ============================================================================

// addToIndex 添加 outpoint 到索引
//
// 索引值格式：多个 outpoint 的序列化数组
// 每个 outpoint: [4字节TxId长度][TxId][4字节OutputIndex]
func (m *IndexManager) addToIndex(ctx context.Context, indexKey []byte, outpointBytes []byte) error {
	// 获取现有索引
	existingData, err := m.storage.Get(ctx, indexKey)
	if err != nil {
		// 如果不存在，创建新索引
		return m.storage.Set(ctx, indexKey, outpointBytes)
	}

	// 检查是否已存在
	if m.containsOutPoint(existingData, outpointBytes) {
		// 已存在，无需重复添加
		return nil
	}

	// 追加到现有索引
	newData := append(existingData, outpointBytes...)
	return m.storage.Set(ctx, indexKey, newData)
}

// addToIndexInTransaction 在事务中添加 outpoint 到索引（事务版本）
func (m *IndexManager) addToIndexInTransaction(tx storage.BadgerTransaction, indexKey []byte, outpointBytes []byte) error {
	// 获取现有索引
	existingData, err := tx.Get(indexKey)
	if err != nil {
		// 如果不存在，创建新索引
		return tx.Set(indexKey, outpointBytes)
	}

	// 检查是否已存在
	if m.containsOutPoint(existingData, outpointBytes) {
		// 已存在，无需重复添加
		return nil
	}

	// 追加到现有索引
	newData := append(existingData, outpointBytes...)
	return tx.Set(indexKey, newData)
}

// removeFromIndex 从索引移除 outpoint
func (m *IndexManager) removeFromIndex(ctx context.Context, indexKey []byte, outpointBytes []byte) error {
	// 获取现有索引
	existingData, err := m.storage.Get(ctx, indexKey)
	if err != nil || len(existingData) == 0 {
		// 索引不存在或为空，无需移除
		return nil
	}

	// 移除匹配的 outpoint
	newData := m.removeOutPoint(existingData, outpointBytes)
	if len(newData) == 0 {
		// 索引为空，删除索引键
		return m.storage.Delete(ctx, indexKey)
	}

	// 更新索引
	return m.storage.Set(ctx, indexKey, newData)
}

// removeFromIndexInTransaction 在事务中从索引移除 outpoint（事务版本）
func (m *IndexManager) removeFromIndexInTransaction(tx storage.BadgerTransaction, indexKey []byte, outpointBytes []byte) error {
	// 获取现有索引
	existingData, err := tx.Get(indexKey)
	if err != nil || len(existingData) == 0 {
		// 索引不存在或为空，无需移除
		return nil
	}

	// 移除匹配的 outpoint
	newData := m.removeOutPoint(existingData, outpointBytes)
	if len(newData) == 0 {
		// 索引为空，删除索引键
		return tx.Delete(indexKey)
	}

	// 更新索引
	return tx.Set(indexKey, newData)
}

// ============================================================================
//                               OutPoint 编码/解码
// ============================================================================

// encodeOutPoint 编码 OutPoint
// 格式：[4字节TxId长度][TxId][4字节OutputIndex]
func (m *IndexManager) encodeOutPoint(outpoint *transaction.OutPoint) []byte {
	txIDLen := len(outpoint.TxId)
	buf := make([]byte, 4+txIDLen+4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(txIDLen))
	copy(buf[4:4+txIDLen], outpoint.TxId)
	binary.BigEndian.PutUint32(buf[4+txIDLen:], outpoint.OutputIndex)
	return buf
}

// decodeOutPoint 解码 OutPoint
func (m *IndexManager) decodeOutPoint(data []byte) (*transaction.OutPoint, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("数据长度不足")
	}

	txIDLen := binary.BigEndian.Uint32(data[0:4])
	if len(data) < int(4+txIDLen+4) {
		return nil, fmt.Errorf("数据长度不足")
	}

	txID := make([]byte, txIDLen)
	copy(txID, data[4:4+txIDLen])
	outputIndex := binary.BigEndian.Uint32(data[4+txIDLen:])

	return &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: outputIndex,
	}, nil
}

// ============================================================================
//                               索引查询辅助
// ============================================================================

// containsOutPoint 检查索引数据中是否包含指定的 outpoint
func (m *IndexManager) containsOutPoint(indexData []byte, outpointBytes []byte) bool {
	i := 0
	for i < len(indexData) {
		if i+4 > len(indexData) {
			break
		}

		txIDLen := int(binary.BigEndian.Uint32(indexData[i:]))
		entryLen := 4 + txIDLen + 4
		if i+entryLen > len(indexData) {
			break
		}

		entryBytes := indexData[i : i+entryLen]
		if len(entryBytes) == len(outpointBytes) {
			match := true
			for j := range entryBytes {
				if entryBytes[j] != outpointBytes[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}

		i += entryLen
	}
	return false
}

// removeOutPoint 从索引数据中移除指定的 outpoint
func (m *IndexManager) removeOutPoint(indexData []byte, outpointBytes []byte) []byte {
	result := make([]byte, 0, len(indexData))
	i := 0

	for i < len(indexData) {
		if i+4 > len(indexData) {
			break
		}

		txIDLen := int(binary.BigEndian.Uint32(indexData[i:]))
		entryLen := 4 + txIDLen + 4
		if i+entryLen > len(indexData) {
			break
		}

		entryBytes := indexData[i : i+entryLen]
		if len(entryBytes) == len(outpointBytes) {
			match := true
			for j := range entryBytes {
				if entryBytes[j] != outpointBytes[j] {
					match = false
					break
				}
			}
			if !match {
				// 不匹配，保留
				result = append(result, entryBytes...)
			}
			// 匹配的跳过（移除）
		} else {
			// 长度不匹配，保留
			result = append(result, entryBytes...)
		}

		i += entryLen
	}

	return result
}

