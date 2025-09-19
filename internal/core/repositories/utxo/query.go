package utxo

import (
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
//                              UTXO存储键定义
// ============================================================================

// UTXO存储键前缀定义
const (
	UTXOKeyPrefix       = "utxo:"      // UTXO数据键前缀: utxo:{txHash}:{outputIndex}
	UTXOAddressPrefix   = "utxo:addr:" // 地址索引键前缀: utxo:addr:{address}:{txHash}:{outputIndex}
	UTXOCategoryPrefix  = "utxo:cat:"  // 类别索引键前缀: utxo:cat:{category}:{txHash}:{outputIndex}
	UTXOStateRootPrefix = "utxo:root:" // 状态根键前缀: utxo:root:{height}
	UTXOMetaPrefix      = "utxo:meta:" // 元数据键前缀: utxo:meta:{key}
)

// UTXO元数据键
const (
	UTXOTotalCountKey  = "utxo:meta:total_count"  // 总UTXO数量
	UTXOLastUpdateKey  = "utxo:meta:last_update"  // 最后更新时间
	UTXOCurrentRootKey = "utxo:meta:current_root" // 当前状态根
)

// formatUTXOKey 格式化UTXO存储键
// 格式: utxo:{txHash}:{outputIndex}
func formatUTXOKey(txHash []byte, outputIndex uint32) []byte {
	key := make([]byte, len(UTXOKeyPrefix)+len(txHash)+4)
	offset := 0

	// 添加前缀
	copy(key[offset:], UTXOKeyPrefix)
	offset += len(UTXOKeyPrefix)

	// 添加交易哈希
	copy(key[offset:], txHash)
	offset += len(txHash)

	// 添加输出索引（大端序）
	binary.BigEndian.PutUint32(key[offset:], outputIndex)

	return key
}

// formatAddressIndexKey 格式化地址索引键
// 格式: utxo:addr:{address}:{txHash}:{outputIndex}
func formatAddressIndexKey(address []byte, txHash []byte, outputIndex uint32) []byte {
	key := make([]byte, len(UTXOAddressPrefix)+len(address)+len(txHash)+4)
	offset := 0

	// 添加前缀
	copy(key[offset:], UTXOAddressPrefix)
	offset += len(UTXOAddressPrefix)

	// 添加地址
	copy(key[offset:], address)
	offset += len(address)

	// 添加交易哈希
	copy(key[offset:], txHash)
	offset += len(txHash)

	// 添加输出索引（大端序）
	binary.BigEndian.PutUint32(key[offset:], outputIndex)

	return key
}

// ============================================================================
//                           🔍 UTXO查询操作实现
// ============================================================================

// getUTXO 根据OutPoint精确获取UTXO
//
// 🎯 **系统定位**：交易验证的基础操作
// 通过OutPoint（交易哈希+输出索引）精确定位并获取UTXO数据。
// 这是交易验证、合约执行等操作的核心依赖。
//
// 实现要点：
// - 精确定位：基于OutPoint的唯一标识进行精确查询
// - 高效查询：直接键值查询，O(1)时间复杂度
// - 完整数据：返回包含所有约束信息的完整UTXO
// - 状态验证：检查UTXO是否存在且可用
func (m *Manager) getUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	if m.logger != nil {
		m.logger.Debugf("查询UTXO实现 - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}

	// 1. 验证OutPoint参数
	if outpoint == nil {
		return nil, fmt.Errorf("OutPoint不能为空")
	}
	if len(outpoint.TxId) != 32 {
		return nil, fmt.Errorf("交易哈希长度错误，期望32字节，实际%d字节", len(outpoint.TxId))
	}

	// 2. 构建存储查询键
	utxoKey := formatUTXOKey(outpoint.TxId, outpoint.OutputIndex)

	// 3. 从BadgerStore查询UTXO数据
	utxoData, err := m.badgerStore.Get(ctx, utxoKey)
	if err != nil {
		return nil, fmt.Errorf("查询UTXO数据失败: %w", err)
	}
	if utxoData == nil {
		return nil, nil // UTXO不存在
	}

	// 4. 反序列化UTXO结构
	var utxoObj utxo.UTXO
	if err := proto.Unmarshal(utxoData, &utxoObj); err != nil {
		return nil, fmt.Errorf("反序列化UTXO数据失败: %w", err)
	}

	// 5. 验证UTXO状态（检查是否已被消费）
	if utxoObj.Status == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
		return nil, nil // 已消费的UTXO视为不存在
	}

	if m.logger != nil {
		m.logger.Debugf("成功查询UTXO - txId: %x, index: %d, status: %s",
			outpoint.TxId, outpoint.OutputIndex, utxoObj.Status.String())
	}

	return &utxoObj, nil
}

// getUTXOsByAddress 获取地址拥有的UTXO列表
//
// 🎯 **系统定位**：AccountService余额计算的数据基础
// 获取指定地址拥有的所有UTXO，支持按类型过滤和可用性过滤。
// 这是余额计算、资产统计等操作的核心数据源。
//
// 实现要点：
// - 地址索引：通过地址索引进行高效查询
// - 类型过滤：支持按UTXOCategory进行类型筛选
// - 可用性过滤：支持只返回可用状态的UTXO
// - 批量查询：一次性获取所有匹配的UTXO
// - 排序返回：按创建时间或高度进行排序
func (m *Manager) getUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	if m.logger != nil {
		var categoryStr string
		if category != nil {
			categoryStr = category.String()
		} else {
			categoryStr = "all"
		}
		m.logger.Debugf("查询地址UTXO列表实现 - address: %x, category: %s, onlyAvailable: %t", address, categoryStr, onlyAvailable)
	}

	// 1. 验证地址参数
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}
	if len(address) != 20 {
		return nil, fmt.Errorf("地址长度错误，期望20字节，实际%d字节", len(address))
	}

	// 2. 构建地址索引前缀进行范围查询
	addressPrefix := make([]byte, len(UTXOAddressPrefix)+len(address))
	copy(addressPrefix, UTXOAddressPrefix)
	copy(addressPrefix[len(UTXOAddressPrefix):], address)

	// 3. 通过前缀查询获取所有相关的地址索引
	indexEntries, err := m.badgerStore.PrefixScan(ctx, addressPrefix)
	if err != nil {
		return nil, fmt.Errorf("查询地址索引失败: %w", err)
	}

	if len(indexEntries) == 0 {
		if m.logger != nil {
			m.logger.Debugf("地址 %x 没有找到UTXO", address)
		}
		return []*utxo.UTXO{}, nil // 返回空列表而不是nil
	}

	// 4. 批量获取UTXO（优化：先获取所有UTXO键，再批量查询）
	var utxoKeys [][]byte
	var outpoints []*transaction.OutPoint

	for indexKeyStr := range indexEntries {
		indexKey := []byte(indexKeyStr)
		// 解析索引键获取txHash和outputIndex
		txHash, outputIndex, err := m.parseAddressIndexKey(indexKey)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("解析地址索引键失败，跳过: %v", err)
			}
			continue
		}

		// 构建UTXO存储键和OutPoint
		utxoKey := formatUTXOKey(txHash, outputIndex)
		utxoKeys = append(utxoKeys, utxoKey)

		outpoint := &transaction.OutPoint{
			TxId:        txHash,
			OutputIndex: outputIndex,
		}
		outpoints = append(outpoints, outpoint)
	}

	// 5. 批量获取UTXO数据（性能优化）
	if len(utxoKeys) == 0 {
		return []*utxo.UTXO{}, nil
	}

	// 注意：GetMany接口直接接受[][]byte，无需转换

	// 批量查询UTXO数据
	utxoDataMap, err := m.badgerStore.GetMany(ctx, utxoKeys)
	if err != nil {
		return nil, fmt.Errorf("批量获取UTXO数据失败: %w", err)
	}

	// 6. 处理查询结果并应用过滤条件
	var utxos []*utxo.UTXO
	for i, outpoint := range outpoints {
		utxoKey := string(utxoKeys[i])
		utxoData, exists := utxoDataMap[utxoKey]
		if !exists || utxoData == nil {
			continue // UTXO不存在，跳过
		}

		// 反序列化UTXO对象
		var utxoObj utxo.UTXO
		if err := proto.Unmarshal(utxoData, &utxoObj); err != nil {
			if m.logger != nil {
				m.logger.Warnf("反序列化UTXO失败，跳过 - txId: %x, index: %d, error: %v",
					outpoint.TxId, outpoint.OutputIndex, err)
			}
			continue
		}

		// 验证UTXO状态（检查是否已被消费）
		if utxoObj.Status == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
			continue // 已消费的UTXO跳过
		}

		// 7. 应用过滤条件
		// 类型过滤
		if category != nil && utxoObj.Category != *category {
			continue
		}

		// 可用性过滤：只返回可用状态的UTXO
		if onlyAvailable {
			if utxoObj.Status != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE &&
				utxoObj.Status != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED {
				continue // 非可用状态（如过期）跳过
			}
		}

		utxos = append(utxos, &utxoObj)
	}

	// 8. 对结果进行排序（按区块高度和创建时间排序，确保确定性结果）
	utxos = m.sortUTXOsByCreationOrder(utxos)

	if m.logger != nil {
		m.logger.Debugf("地址 %x 查询到 %d 个UTXO（过滤后）", address, len(utxos))
	}

	return utxos, nil
}

// parseAddressIndexKey 解析地址索引键，提取txHash和outputIndex
// 键格式: utxo:addr:{address}:{txHash}:{outputIndex}
func (m *Manager) parseAddressIndexKey(indexKey []byte) (txHash []byte, outputIndex uint32, err error) {
	// 检查键长度和前缀
	expectedMinLen := len(UTXOAddressPrefix) + 20 + 32 + 4 // 前缀+地址+哈希+索引
	if len(indexKey) < expectedMinLen {
		return nil, 0, fmt.Errorf("索引键长度错误")
	}

	// 跳过前缀和地址部分
	offset := len(UTXOAddressPrefix) + 20

	// 提取交易哈希（32字节）
	txHash = make([]byte, 32)
	copy(txHash, indexKey[offset:offset+32])
	offset += 32

	// 提取输出索引（4字节，大端序）
	outputIndex = binary.BigEndian.Uint32(indexKey[offset:])

	return txHash, outputIndex, nil
}

// ============================================================================
//                           🔧 UTXO查询辅助方法
// ============================================================================

// sortUTXOsByCreationOrder 按创建顺序对UTXO列表进行排序
//
// 🎯 **确定性排序策略**：
// 1. 优先按区块高度排序（较新的区块排在后面）
// 2. 区块高度相同时，按创建时间戳排序
// 3. 时间戳相同时，按OutPoint字典序排序（确保完全确定性）
//
// 这确保了查询结果的确定性和可预测性，对于余额计算和UTXO选择很重要。
func (m *Manager) sortUTXOsByCreationOrder(utxos []*utxo.UTXO) []*utxo.UTXO {
	if len(utxos) <= 1 {
		return utxos // 单个或空列表无需排序
	}

	// 使用sort.Slice进行自定义排序
	sort.Slice(utxos, func(i, j int) bool {
		utxoA, utxoB := utxos[i], utxos[j]

		// 1. 首先按区块高度排序（升序：较早的区块排在前面）
		if utxoA.BlockHeight != utxoB.BlockHeight {
			return utxoA.BlockHeight < utxoB.BlockHeight
		}

		// 2. 区块高度相同时，按创建时间戳排序（升序：较早创建的排在前面）
		if utxoA.CreatedTimestamp != utxoB.CreatedTimestamp {
			return utxoA.CreatedTimestamp < utxoB.CreatedTimestamp
		}

		// 3. 时间戳相同时，按OutPoint进行字典序排序（确保完全确定性）
		outpointA := utxoA.GetOutpoint()
		outpointB := utxoB.GetOutpoint()

		if outpointA == nil || outpointB == nil {
			// 处理异常情况：如果OutPoint为空，将其排在后面
			if outpointA == nil && outpointB != nil {
				return false
			}
			if outpointA != nil && outpointB == nil {
				return true
			}
			return false // 都为空时，保持原有顺序
		}

		// 比较交易哈希（字典序）
		txHashA := outpointA.GetTxId()
		txHashB := outpointB.GetTxId()
		if len(txHashA) != len(txHashB) {
			return len(txHashA) < len(txHashB)
		}

		for k := 0; k < len(txHashA) && k < len(txHashB); k++ {
			if txHashA[k] != txHashB[k] {
				return txHashA[k] < txHashB[k]
			}
		}

		// 交易哈希相同时，比较输出索引
		return outpointA.GetOutputIndex() < outpointB.GetOutputIndex()
	})

	return utxos
}
