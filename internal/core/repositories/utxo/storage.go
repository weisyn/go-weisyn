// Package utxo UTXO存储操作实现
//
// 🔧 **UTXO存储操作 (UTXO Storage Operations)**
//
// 本文件实现UTXO的创建、更新和删除操作，包括：
// - UTXO创建：从交易输出创建新的UTXO记录
// - UTXO状态更新：标记UTXO为已消费状态
// - 索引管理：维护地址索引和类别索引
//
// 🎯 **核心功能**
// - 原子性操作：所有UTXO操作都在事务中进行
// - 索引一致性：确保UTXO数据与索引的一致性
// - 状态管理：正确管理UTXO的生命周期状态
package utxo

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// createUTXO 从交易输出创建新的UTXO
//
// 🎯 **生产级UTXO创建**：
// 从区块中的交易输出创建对应的UTXO记录，包括：
// 1. 构建完整的UTXO对象
// 2. 序列化并存储UTXO数据
// 3. 创建地址索引
// 4. 创建类别索引（如果需要）
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//   - output: 交易输出
//   - blockHeight: 区块高度
//
// 返回：
//   - error: 创建错误
func (m *Manager) createUTXO(ctx context.Context, tx storage.BadgerTransaction, txHash []byte, outputIndex uint32, output *transaction.TxOutput, blockHeight uint64) error {
	if m.logger != nil {
		m.logger.Debugf("创建UTXO - txHash: %x, index: %d, height: %d", txHash, outputIndex, blockHeight)
	}

	// 1. 构建UTXO对象
	utxoObj := &utxo.UTXO{
		Outpoint: &transaction.OutPoint{
			TxId:        txHash,
			OutputIndex: outputIndex,
		},
		Category:         m.determineUTXOCategory(output),
		OwnerAddress:     output.Owner,
		BlockHeight:      blockHeight,
		CreatedTimestamp: uint64(time.Now().Unix()),
		Status:           utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
	}

	// 2. 设置内容存储策略（缓存完整输出）
	utxoObj.ContentStrategy = &utxo.UTXO_CachedOutput{
		CachedOutput: output,
	}

	// 3. 序列化UTXO数据
	utxoData, err := proto.Marshal(utxoObj)
	if err != nil {
		return fmt.Errorf("序列化UTXO失败: %w", err)
	}

	// 4. 存储UTXO数据
	utxoKey := formatUTXOKey(txHash, outputIndex)
	if err := tx.Set(utxoKey, utxoData); err != nil {
		return fmt.Errorf("存储UTXO数据失败: %w", err)
	}

	// 5. 创建地址索引
	if err := m.createAddressIndex(tx, output.Owner, txHash, outputIndex); err != nil {
		return fmt.Errorf("创建地址索引失败: %w", err)
	}

	// 6. 创建类别索引（如果需要）
	if err := m.createCategoryIndex(tx, utxoObj.Category, txHash, outputIndex); err != nil {
		return fmt.Errorf("创建类别索引失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("UTXO创建成功 - txHash: %x, index: %d, category: %s",
			txHash, outputIndex, utxoObj.Category.String())
	}

	return nil
}

// markUTXOAsSpent 标记UTXO为已消费状态
//
// 🎯 **UTXO消费处理**：
// 当UTXO被交易输入消费时调用，负责：
// 1. 更新UTXO状态为已消费
// 2. 记录消费时间和消费交易
// 3. 保持索引不变（用于审计和查询历史）
func (m *Manager) markUTXOAsSpent(ctx context.Context, tx storage.BadgerTransaction, outpoint *transaction.OutPoint) error {
	if m.logger != nil {
		m.logger.Debugf("标记UTXO已消费 - txHash: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}

	// 1. 获取现有UTXO
	utxoKey := formatUTXOKey(outpoint.TxId, outpoint.OutputIndex)
	utxoData, err := m.badgerStore.Get(ctx, utxoKey)
	if err != nil {
		return fmt.Errorf("获取UTXO失败: %w", err)
	}
	if utxoData == nil {
		return fmt.Errorf("UTXO不存在")
	}

	// 2. 反序列化UTXO
	var utxoObj utxo.UTXO
	if err := proto.Unmarshal(utxoData, &utxoObj); err != nil {
		return fmt.Errorf("反序列化UTXO失败: %w", err)
	}

	// 3. 更新状态
	utxoObj.Status = utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED
	// 注意：这里应该记录消费这个UTXO的交易哈希，但需要从上层传入

	// 4. 重新序列化并存储
	updatedData, err := proto.Marshal(&utxoObj)
	if err != nil {
		return fmt.Errorf("序列化更新的UTXO失败: %w", err)
	}

	if err := tx.Set(utxoKey, updatedData); err != nil {
		return fmt.Errorf("更新UTXO状态失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("UTXO标记为已消费 - txHash: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}

	return nil
}

// createAddressIndex 创建地址索引
func (m *Manager) createAddressIndex(tx storage.BadgerTransaction, address []byte, txHash []byte, outputIndex uint32) error {
	indexKey := formatAddressIndexKey(address, txHash, outputIndex)
	// 索引值可以是空的，我们只需要键存在
	return tx.Set(indexKey, []byte{1})
}

// createCategoryIndex 创建类别索引
func (m *Manager) createCategoryIndex(tx storage.BadgerTransaction, category utxo.UTXOCategory, txHash []byte, outputIndex uint32) error {
	// 构建类别索引键
	categoryStr := category.String()
	categoryKey := make([]byte, len(UTXOCategoryPrefix)+len(categoryStr)+1+len(txHash)+4)
	offset := 0

	// 添加前缀
	copy(categoryKey[offset:], UTXOCategoryPrefix)
	offset += len(UTXOCategoryPrefix)

	// 添加类别字符串
	copy(categoryKey[offset:], categoryStr)
	offset += len(categoryStr)

	// 添加分隔符
	categoryKey[offset] = ':'
	offset++

	// 添加交易哈希
	copy(categoryKey[offset:], txHash)
	offset += len(txHash)

	// 添加输出索引
	binary.BigEndian.PutUint32(categoryKey[offset:], outputIndex)

	// 索引值可以是空的，我们只需要键存在
	return tx.Set(categoryKey, []byte{1})
}

// determineUTXOCategory 确定UTXO的类别
func (m *Manager) determineUTXOCategory(output *transaction.TxOutput) utxo.UTXOCategory {
	if output.OutputContent == nil {
		return utxo.UTXOCategory_UTXO_CATEGORY_UNKNOWN
	}

	switch output.OutputContent.(type) {
	case *transaction.TxOutput_Asset:
		return utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	case *transaction.TxOutput_Resource:
		return utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE
	case *transaction.TxOutput_State:
		return utxo.UTXOCategory_UTXO_CATEGORY_STATE
	default:
		return utxo.UTXOCategory_UTXO_CATEGORY_UNKNOWN
	}
}
