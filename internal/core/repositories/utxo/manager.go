// Package utxo 提供WES区块链UTXO数据仓储服务的实现
//
// 💎 **UTXO数据管理器 (UTXO Manager)**
//
// 本文件实现了UTXO数据仓储服务，专注于：
// - UTXO查询操作：精确查询和地址聚合查询
// - 引用管理操作：ResourceUTXO的并发引用控制
// - 状态管理：UTXO生命周期状态转换和约束检查
//
// 🏗️ **设计原则**
// - 数据源头约束：所有UTXO数据来源于TxOutput，通过区块处理统一写入
// - 依赖注入：通过构造函数注入所需依赖
// - 职责分离：将查询和引用管理操作分散到专门文件
// - 业务导向：基于实际业务需求精简设计，专注核心场景
package utxo

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	// protobuf定义
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"

	// 内部接口
	"github.com/weisyn/v1/internal/core/repositories/interfaces"
)

// ============================================================================
//                              服务结构定义
// ============================================================================

// Manager UTXO数据管理器
//
// 🎯 **统一UTXO数据服务入口**
//
// 负责实现 UTXOManager 的所有公共接口方法，并将具体实现
// 委托给专门的子文件处理。遵循数据源头约束原则，确保数据一致性。
//
// 架构特点：
// - 统一入口：所有UTXO数据操作的统一访问点
// - 依赖注入：通过构造函数注入必需的存储依赖
// - 委托实现：将具体业务逻辑委托给专门的子文件
// - 并发安全：ResourceUTXO引用计数管理，防止并发冲突
type Manager struct {
	// 核心依赖
	logger            log.Logger               // 日志服务
	badgerStore       storage.BadgerStore      // 持久化存储
	memoryStore       storage.MemoryStore      // 内存缓存
	hashManager       crypto.HashManager       // 哈希计算服务
	merkleTreeManager crypto.MerkleTreeManager // Merkle树管理服务

	// 内部服务接口
	utxoService interfaces.InternalUTXOManager // UTXO内部服务接口
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewManager 创建UTXO数据管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//
//	logger: 日志服务
//	badgerStore: 持久化存储
//	memoryStore: 内存缓存
//	hashManager: 哈希计算服务
//	merkleTreeManager: Merkle树管理服务
//
// 返回：
//
//	*Manager: UTXO数据管理器实例
//	error: 创建错误
func NewManager(
	logger log.Logger,
	badgerStore storage.BadgerStore,
	memoryStore storage.MemoryStore,
	hashManager crypto.HashManager,
	merkleTreeManager crypto.MerkleTreeManager,
) (*Manager, error) {
	if badgerStore == nil {
		return nil, fmt.Errorf("badger store 不能为空")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("hash manager 不能为空")
	}
	if merkleTreeManager == nil {
		return nil, fmt.Errorf("merkle tree manager 不能为空")
	}

	manager := &Manager{
		logger:            logger,
		badgerStore:       badgerStore,
		memoryStore:       memoryStore,
		hashManager:       hashManager,
		merkleTreeManager: merkleTreeManager,
	}

	if logger != nil {
		logger.Debug("UTXO数据管理器初始化完成")
	}

	return manager, nil
}

// ============================================================================
//                            🔍 核心查询接口实现
// ============================================================================

// GetUTXO 根据OutPoint精确获取UTXO
func (m *Manager) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	if m.logger != nil {
		m.logger.Debugf("精确获取UTXO - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}
	// 调用具体实现方法 (query.go)
	return m.getUTXO(ctx, outpoint)
}

// GetUTXOsByAddress 获取地址拥有的UTXO列表
func (m *Manager) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	if m.logger != nil {
		m.logger.Debugf("获取地址UTXO列表 - address: %x, onlyAvailable: %t", address, onlyAvailable)
	}
	// 调用具体实现方法 (query.go)
	return m.getUTXOsByAddress(ctx, address, category, onlyAvailable)
}

// ============================================================================
//                           🔄 核心状态操作实现
// ============================================================================

// ReferenceUTXO 引用UTXO（增加引用计数）
func (m *Manager) ReferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if m.logger != nil {
		m.logger.Debugf("引用UTXO - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}
	// 调用具体实现方法 (reference.go)
	return m.referenceUTXO(ctx, outpoint)
}

// UnreferenceUTXO 解除UTXO引用（减少引用计数）
func (m *Manager) UnreferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if m.logger != nil {
		m.logger.Debugf("解除UTXO引用 - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}
	// 调用具体实现方法 (reference.go)
	return m.unreferenceUTXO(ctx, outpoint)
}

// ============================================================================
//                           📊 状态根管理接口实现
// ============================================================================

// GetCurrentStateRoot 获取当前UTXO状态根
func (m *Manager) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debug("获取当前UTXO状态根")
	}
	// 调用具体实现方法 (state_root.go)
	return m.getCurrentStateRoot(ctx)
}

// ============================================================================
//                           🔧 UTXO创建和管理操作
// ============================================================================

// ProcessBlockUTXOs 处理区块中的UTXO变更（创建新UTXO，标记已消费UTXO）
//
// 🎯 **生产级别的UTXO处理**：
// 在区块处理过程中调用，负责：
// 1. 创建区块中所有交易输出对应的新UTXO
// 2. 标记区块中所有交易输入消费的UTXO为已消费状态
// 3. 更新地址索引和类别索引
// 4. 更新UTXO状态根
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务（确保原子性）
//   - block: 要处理的区块
//   - blockHash: 区块哈希
//   - txHashes: 区块中所有交易的哈希列表
//
// 返回：
//   - error: 处理错误
func (m *Manager) ProcessBlockUTXOs(ctx context.Context, tx storage.BadgerTransaction, block *core.Block, blockHash []byte, txHashes [][]byte) error {
	if m.logger != nil {
		m.logger.Debugf("处理区块UTXO变更 - height: %d, txCount: %d", block.Header.Height, len(block.Body.Transactions))
	}

	// 1. 处理所有交易的输入（标记UTXO为已消费）
	for i, transaction := range block.Body.Transactions {
		if len(transaction.Inputs) > 0 { // 跳过Coinbase交易（创世交易没有输入）
			for _, input := range transaction.Inputs {
				if err := m.markUTXOAsSpent(ctx, tx, input.PreviousOutput); err != nil {
					return fmt.Errorf("标记UTXO已消费失败 (tx %d): %w", i, err)
				}
			}
		}
	}

	// 2. 处理所有交易的输出（创建新UTXO）
	for i, transaction := range block.Body.Transactions {
		if len(txHashes) <= i {
			return fmt.Errorf("交易哈希列表长度不匹配")
		}

		txHash := txHashes[i]
		for j, output := range transaction.Outputs {
			if err := m.createUTXO(ctx, tx, txHash, uint32(j), output, block.Header.Height); err != nil {
				return fmt.Errorf("创建UTXO失败 (tx %d, output %d): %w", i, j, err)
			}
		}
	}

	if m.logger != nil {
		m.logger.Debugf("区块UTXO处理完成 - height: %d", block.Header.Height)
	}

	return nil
}
