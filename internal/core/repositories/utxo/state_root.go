package utxo

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
//                           📊 UTXO状态根计算实现
// ============================================================================

// getCurrentStateRoot 获取当前UTXO状态根
//
// 🎯 **系统定位**：状态根哈希计算核心
// 计算当前所有UTXO状态的Merkle树根哈希，用于区块头中记录当前区块链状态的摘要。
// 支持轻客户端验证和状态一致性检查。
//
// 实现策略：
// - 获取所有可用UTXO的序列化数据
// - 使用统一的MerkleTreeManager构建Merkle树
// - 确保确定性计算（相同UTXO集合产生相同状态根）
// - 如果没有UTXO，返回空字节数组
//
// 🏗️ **架构价值**：
// - 状态证明：为区块头提供状态摘要
// - 一致性验证：支持节点间状态一致性检查
// - 轻客户端：为轻客户端提供状态验证基础
// - 确定性计算：相同UTXO集合总是产生相同状态根
func (m *Manager) getCurrentStateRoot(ctx context.Context) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debug("开始计算UTXO状态根")
	}

	// 1. 获取所有UTXO的序列化数据
	utxoData, err := m.getAllUTXOSerializedData(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取UTXO序列化数据失败: %w", err)
	}

	// 2. 如果没有UTXO，返回空字节数组
	if len(utxoData) == 0 {
		if m.logger != nil {
			m.logger.Debug("没有UTXO，返回空状态根")
		}
		return []byte{}, nil
	}

	// 3. 使用MerkleTreeManager构建Merkle树
	merkleTree, err := m.merkleTreeManager.NewMerkleTree(utxoData)
	if err != nil {
		return nil, fmt.Errorf("构建UTXO Merkle树失败: %w", err)
	}

	// 4. 获取Merkle树根哈希
	stateRoot := merkleTree.GetRoot()

	if m.logger != nil {
		m.logger.Debugf("UTXO状态根计算完成 - stateRoot: %x, utxoCount: %d", stateRoot, len(utxoData))
	}

	return stateRoot, nil
}

// getAllUTXOSerializedData 获取所有UTXO的序列化数据
//
// 🎯 **核心实现策略**：
// - 扫描BadgerStore中的所有UTXO记录
// - 使用protobuf序列化确保数据格式一致性
// - 按键排序确保确定性结果
// - 只包含有效状态的UTXO
//
// 实现要点：
// - 键前缀扫描：使用UTXO存储键前缀进行高效扫描
// - protobuf序列化：使用proto.Marshal确保数据一致性
// - 确定性排序：按存储键排序确保相同结果
// - 状态过滤：只包含AVAILABLE和REFERENCED状态的UTXO
func (m *Manager) getAllUTXOSerializedData(ctx context.Context) ([][]byte, error) {
	var utxoDataList [][]byte

	// UTXO存储键前缀（根据实际存储设计调整）
	const utxoKeyPrefix = "utxo:"

	// 使用PrefixScan扫描所有UTXO键值对
	utxoMap, err := m.badgerStore.PrefixScan(ctx, []byte(utxoKeyPrefix))
	if err != nil {
		return nil, fmt.Errorf("扫描UTXO存储失败: %w", err)
	}

	// 处理扫描结果
	for key, value := range utxoMap {
		// 反序列化UTXO对象
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(value, utxoObj); err != nil {
			if m.logger != nil {
				m.logger.Warnf("反序列化UTXO失败，跳过 - key: %s, error: %v", key, err)
			}
			continue // 跳过损坏的记录，继续处理
		}

		// 只包含有效状态的UTXO
		if utxoObj.GetStatus() == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE ||
			utxoObj.GetStatus() == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED {

			// 重新序列化确保数据格式一致性
			serializedData, err := proto.Marshal(utxoObj)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("序列化UTXO失败，跳过 - key: %s, error: %v", key, err)
				}
				continue // 跳过序列化失败的记录
			}

			utxoDataList = append(utxoDataList, serializedData)
		}
	}

	// 确保确定性结果：按序列化数据的哈希值排序
	// 这样可以确保相同的UTXO集合总是产生相同的状态根
	if len(utxoDataList) > 1 {
		utxoDataList = m.sortUTXODataDeterministically(utxoDataList)
	}

	if m.logger != nil {
		m.logger.Debugf("获取UTXO序列化数据完成 - count: %d", len(utxoDataList))
	}

	return utxoDataList, nil
}

// sortUTXODataDeterministically 确定性排序UTXO数据
//
// 🎯 **确定性排序策略**：
// - 使用HashManager计算每个UTXO数据的哈希值
// - 按哈希值进行字典序排序
// - 确保相同UTXO集合总是产生相同的排序结果
// - 支持大数据集的高效排序
func (m *Manager) sortUTXODataDeterministically(utxoDataList [][]byte) [][]byte {
	// 创建哈希-数据映射用于排序
	type hashDataPair struct {
		hash []byte
		data []byte
	}

	hashDataPairs := make([]hashDataPair, len(utxoDataList))

	// 计算每个UTXO数据的哈希值
	for i, data := range utxoDataList {
		hash := m.hashManager.SHA256(data)
		hashDataPairs[i] = hashDataPair{
			hash: hash,
			data: data,
		}
	}

	// 按哈希值排序（字典序）
	for i := 0; i < len(hashDataPairs)-1; i++ {
		for j := i + 1; j < len(hashDataPairs); j++ {
			// 比较哈希值
			if m.compareBytes(hashDataPairs[i].hash, hashDataPairs[j].hash) > 0 {
				hashDataPairs[i], hashDataPairs[j] = hashDataPairs[j], hashDataPairs[i]
			}
		}
	}

	// 提取排序后的数据
	sortedData := make([][]byte, len(hashDataPairs))
	for i, pair := range hashDataPairs {
		sortedData[i] = pair.data
	}

	return sortedData
}

// compareBytes 字节数组比较函数
//
// 返回值：
//   - < 0: a < b
//   - = 0: a == b
//   - > 0: a > b
func (m *Manager) compareBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	// 前缀相同，比较长度
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}

	return 0 // 完全相同
}
