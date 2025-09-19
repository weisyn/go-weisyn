// Package validation 提供交易验证的统一管理服务
//
// 🎯 **验证服务统一管理器**
//
// 本文件实现验证服务的统一管理，作为所有验证功能的中心入口：
// - 单交易验证：单个交易的完整性和有效性验证
// - 区块交易验证：区块中交易批量验证和一致性检查
// - 创世交易验证：创世区块交易的特殊规则验证
// - 交易对象验证：通用交易对象验证
//
// 🏗️ **架构定位**：
// - 服务聚合层：统一管理各种专业验证器
// - 接口适配层：为不同调用方提供统一接口
// - 依赖管理层：统一管理验证器的依赖注入
//
// 🔧 **设计原则**：
// - 单一入口：所有验证请求通过本管理器分发
// - 职责分离：各专业验证器专注特定验证逻辑
// - 依赖统一：统一管理所有验证器的依赖
// - 接口标准：提供标准化的验证接口
package validation

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/blockchain/transaction/genesis"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// ValidationManager 验证服务统一管理器
//
// 🎯 **验证服务的中央协调器**
//
// 负责统一管理和协调所有类型的交易验证：
// - 提供统一的验证入口点
// - 管理各专业验证器的生命周期
// - 处理验证请求的路由和分发
// - 统一错误处理和日志记录
//
// 💡 **核心价值**：
// - ✅ **统一接口**：为外部提供一致的验证接口
// - ✅ **职责聚合**：将分散的验证逻辑统一管理
// - ✅ **依赖注入**：统一管理验证器依赖
// - ✅ **可维护性**：便于扩展和维护验证逻辑
//
// 📝 **典型调用链**：
// 外部调用 → ValidationManager → 专业验证器 → 具体验证实现
type ValidationManager struct {
	logger            log.Logger                               // 日志记录器（可选）
	cacheStore        storage.MemoryStore                      // 缓存存储
	utxoManager       repository.UTXOManager                   // UTXO管理器
	hashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	localChainID      uint64                                   // 本地链ID（用于跨网防护）

	// 专业验证器实例
	singleValidator *SingleTransactionValidator // 单交易验证器
	blockValidator  *BlockTransactionValidator  // 区块交易验证器
}

// NewValidationManager 创建验证服务管理器
//
// 🏗️ **验证管理器工厂方法**
//
// 创建并初始化验证服务管理器，统一管理所有验证器实例。
// 使用依赖注入模式，确保所有验证器都有正确的依赖。
//
// 💡 **参数说明**：
//   - logger: 日志记录器（可选，传nil则不记录日志）
//   - cacheStore: 内存缓存（用于获取交易，可为nil）
//   - utxoManager: UTXO管理器（用于状态验证，可为nil）
//   - hashServiceClient: 交易哈希服务客户端（用于哈希计算）
//   - localChainID: 本地链ID（用于跨网防护，0表示不检查）
//
// 💡 **返回值说明**：
//   - *ValidationManager: 验证管理器实例
func NewValidationManager(
	logger log.Logger,
	cacheStore storage.MemoryStore,
	utxoManager repository.UTXOManager,
	hashServiceClient transaction.TransactionHashServiceClient,
	localChainID uint64,
) *ValidationManager {
	return &ValidationManager{
		logger:            logger,
		cacheStore:        cacheStore,
		utxoManager:       utxoManager,
		hashServiceClient: hashServiceClient,
		localChainID:      localChainID,

		// 初始化专业验证器（传入本地链ID用于跨网防护）
		singleValidator: NewSingleTransactionValidator(logger, cacheStore, utxoManager, localChainID),
		blockValidator:  NewBlockTransactionValidator(utxoManager, hashServiceClient, logger),
	}
}

// ============================================================================
//                           单交易验证接口
// ============================================================================

// ValidateTransaction 通过交易哈希验证交易
//
// 🎯 **公共接口实现**
//
// 实现公共接口的交易验证方法，通过交易哈希查找并验证交易。
// 适用于外部API调用和生命周期管理。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时控制
//   - txHash: 交易哈希（32字节）
//
// 💡 **返回值说明**：
//   - bool: 验证结果（true=通过，false=不通过）
//   - error: 验证过程中的错误
func (vm *ValidationManager) ValidateTransaction(
	ctx context.Context,
	txHash []byte,
) (bool, error) {
	if vm.logger != nil {
		vm.logger.Debugf("验证管理器：开始验证交易 - 哈希: %x", txHash[:8])
	}

	// 委托给单交易验证器
	return vm.singleValidator.ValidateTransactionByHash(ctx, txHash)
}

// ValidateTransactionObject 验证交易对象
//
// 🎯 **交易对象直接验证**
//
// 直接验证交易对象，无需哈希查找。适用于新构建的交易
// 或已知交易对象的验证场景。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - tx: 完整的交易对象
//
// 💡 **返回值说明**：
//   - bool: 验证结果
//   - error: 验证错误
func (vm *ValidationManager) ValidateTransactionObject(
	ctx context.Context,
	tx *transaction.Transaction,
) (bool, error) {
	if vm.logger != nil {
		vm.logger.Debug("验证管理器：开始验证交易对象")
	}

	if tx == nil {
		return false, fmt.Errorf("交易对象为空")
	}

	// 委托给单交易验证器
	return vm.singleValidator.ValidateTransactionObject(ctx, tx)
}

// ============================================================================
//                           区块交易验证接口
// ============================================================================

// ValidateTransactionsInBlock 批量验证区块中的交易
//
// 🎯 **区块交易批量验证**
//
// 对区块中的所有交易进行批量验证，确保交易的完整性、
// 有效性和一致性。包括并行验证和早期返回优化。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时
//   - transactions: 需要验证的交易列表
//
// 💡 **返回值说明**：
//   - bool: 是否所有交易都有效
//   - error: 验证过程中的错误
func (vm *ValidationManager) ValidateTransactionsInBlock(
	ctx context.Context,
	transactions []*transaction.Transaction,
) (bool, error) {
	if vm.logger != nil {
		vm.logger.Debugf("验证管理器：开始批量验证区块交易 - 数量: %d", len(transactions))
	}

	// 委托给区块交易验证器
	return vm.blockValidator.ValidateTransactionsInBlock(ctx, transactions)
}

// ============================================================================
//                           创世交易验证接口
// ============================================================================

// ValidateGenesisTransactions 验证创世交易有效性
//
// 🎯 **创世交易专门验证**
//
// 对创世交易进行专门验证，包括创世交易的特殊规则：
// - 无输入交易验证
// - 初始余额分配验证
// - 系统合约部署验证
// - 创世交易确定性验证
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - transactions: 创世交易列表
//
// 💡 **返回值说明**：
//   - bool: 验证结果，true表示所有交易有效
//   - error: 验证过程中的错误
func (vm *ValidationManager) ValidateGenesisTransactions(
	ctx context.Context,
	transactions []*transaction.Transaction,
) (bool, error) {
	if vm.logger != nil {
		vm.logger.Debugf("验证管理器：开始验证创世交易 - 数量: %d", len(transactions))
	}

	// 委托给创世交易验证函数
	return genesis.ValidateTransactions(ctx, transactions, vm.logger)
}

// ============================================================================
//                           验证器管理接口
// ============================================================================

// GetSingleValidator 获取单交易验证器
//
// 🔧 **验证器访问接口**
//
// 为需要直接访问单交易验证器的内部组件提供访问接口。
// 主要用于测试和特殊场景。
//
// 💡 **返回值说明**：
//   - *SingleTransactionValidator: 单交易验证器实例
func (vm *ValidationManager) GetSingleValidator() *SingleTransactionValidator {
	return vm.singleValidator
}

// GetBlockValidator 获取区块交易验证器
//
// 🔧 **验证器访问接口**
//
// 为需要直接访问区块交易验证器的内部组件提供访问接口。
// 主要用于测试和特殊场景。
//
// 💡 **返回值说明**：
//   - *BlockTransactionValidator: 区块交易验证器实例
func (vm *ValidationManager) GetBlockValidator() *BlockTransactionValidator {
	return vm.blockValidator
}
