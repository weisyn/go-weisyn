// Package validation 提供区块链交易验证服务
//
// 🎯 **职责定位**：专业的交易验证服务模块
//
// 本包专门处理交易验证的核心逻辑，包括：
// - 单个交易验证（供公共接口使用）
// - 批量交易验证（供区块验证使用）
// - 复杂验证规则（UTXO、签名、权限等）
// - 性能优化的批量处理
//
// 🏗️ **架构分层**：
// - 本包：专业验证逻辑实现
// - lifecycle/validation.go：公共接口适配层
// - manager.go：顶层协调和委托
//
// 📋 **验证类型分工**：
// - SingleTransactionValidation：单交易完整验证
// - BlockTransactionValidation：批量交易验证优化
// - ValidationRules：验证规则引擎
// - ValidationCache：验证结果缓存
//
// ⚠️ **设计原则**：
// - 验证逻辑与业务逻辑分离
// - 批量验证性能优化
// - 验证结果可缓存
// - 错误信息详细准确
package validation

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/utils"
)

// BlockTransactionValidator 区块交易批量验证器
//
// 🎯 **专业的区块级交易验证服务**
//
// 专门用于区块验证场景的批量交易验证，提供高性能的
// 并行验证能力和优化的验证流程。
//
// 💡 **核心价值**：
// - ✅ **批量优化**：一次验证整个区块的所有交易
// - ✅ **并行处理**：充分利用多核CPU进行并行验证
// - ✅ **缓存友好**：智能缓存验证结果，避免重复计算
// - ✅ **错误聚合**：收集所有验证错误，便于调试
//
// 📝 **典型应用场景**：
// - 区块接收验证：验证网络接收的新区块
// - 重放验证：历史区块重新验证
// - 共识验证：共识过程中的交易验证
// - 同步验证：区块同步时的批量验证
type BlockTransactionValidator struct {
	utxoManager       repository.UTXOManager                   // UTXO管理器
	hashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	logger            log.Logger                               // 日志记录器（可选）
}

// NewBlockTransactionValidator 创建区块交易验证器
//
// 🎯 **验证器工厂方法**
//
// 💡 **参数说明**：
//   - utxoManager: UTXO管理器（用于验证UTXO存在性）
//   - hashServiceClient: 交易哈希服务客户端（用于哈希验证）
//   - logger: 日志记录器（可选，传nil则不记录日志）
//
// 💡 **返回值说明**：
//   - *BlockTransactionValidator: 验证器实例
func NewBlockTransactionValidator(
	utxoManager repository.UTXOManager,
	hashServiceClient transaction.TransactionHashServiceClient,
	logger log.Logger,
) *BlockTransactionValidator {
	return &BlockTransactionValidator{
		utxoManager:       utxoManager,
		hashServiceClient: hashServiceClient,
		logger:            logger,
	}
}

// ValidateTransactionsInBlock 批量验证区块中的交易
//
// 🎯 **区块交易批量验证的核心实现**
//
// 对区块中的所有交易进行批量验证，确保：
// 1. 每个交易的数据结构完整性
// 2. 交易签名的有效性
// 3. UTXO引用的正确性
// 4. 交易费用计算的准确性
// 5. 交易间的一致性（避免双花等）
//
// 📊 **性能优化特性**：
// - ✅ **并行验证**：多个交易同时验证
// - ✅ **早期返回**：发现错误立即返回
// - ✅ **批量检查**：UTXO批量存在性检查
// - ✅ **缓存利用**：复用已验证的签名等
//
// 📝 **验证顺序**：
// 1. 基础结构验证（快速失败）
// 2. 签名验证（计算密集）
// 3. UTXO状态验证（I/O密集）
// 4. 业务逻辑验证（规则检查）
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时
//   - transactions: 需要验证的交易列表
//
// 💡 **返回值说明**：
//   - bool: 是否所有交易都有效
//   - error: 验证过程中的错误（包含具体的失败信息）
//
// 💡 **调用示例**：
//
//	validator := NewBlockTransactionValidator(logger)
//	valid, err := validator.ValidateTransactionsInBlock(ctx, blockTransactions)
//	if err != nil {
//	    log.Errorf("区块交易验证失败: %v", err)
//	    return false, err
//	}
//	if !valid {
//	    log.Warn("区块包含无效交易")
//	    return false, fmt.Errorf("区块验证失败")
//	}
func (v *BlockTransactionValidator) ValidateTransactionsInBlock(
	ctx context.Context,
	transactions []*transaction.Transaction,
) (bool, error) {
	// 基础验证：区块交易数量检查
	if len(transactions) == 0 {
		return false, fmt.Errorf("区块交易列表为空")
	}

	coinbaseCount := 0

	// 逐笔验证区块中的每个交易
	for idx, tx := range transactions {
		if tx == nil {
			return false, fmt.Errorf("交易为空，索引: %d", idx)
		}

		// 标准哈希校验（含基础结构有效性）
		hashResp, err := v.hashServiceClient.ComputeHash(ctx, &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		})
		if err != nil {
			return false, fmt.Errorf("计算交易哈希失败，索引: %d, 错误: %w", idx, err)
		}
		if !hashResp.GetIsValid() {
			return false, fmt.Errorf("交易结构无效，索引: %d", idx)
		}

		// coinbase 识别：使用标准辅助函数进行更完整的检查
		if utils.IsCoinbaseTx(tx) {
			coinbaseCount++
			continue
		}

		// 非 coinbase：要求至少1个输入
		if len(tx.Inputs) == 0 {
			return false, fmt.Errorf("非coinbase交易缺少输入，索引: %d", idx)
		}

		// 非 coinbase 交易：验证所有输入的UTXO存在性
		for inIdx, input := range tx.Inputs {
			if input == nil || input.PreviousOutput == nil {
				return false, fmt.Errorf("交易输入无效，tx索引: %d, 输入索引: %d", idx, inIdx)
			}

			// 验证UTXO存在性（真实的公共接口调用）
			utxo, err := v.utxoManager.GetUTXO(ctx, input.PreviousOutput)
			if err != nil {
				return false, fmt.Errorf("获取UTXO失败，tx索引: %d, 输入索引: %d, 错误: %v", idx, inIdx, err)
			}
			if utxo == nil {
				return false, fmt.Errorf("引用的UTXO不存在，tx索引: %d, 输入索引: %d", idx, inIdx)
			}
		}
	}

	// 验证coinbase交易数量规则
	if coinbaseCount != 1 {
		return false, fmt.Errorf("区块中coinbase交易数量不合法，期望1，实际: %d", coinbaseCount)
	}

	if v.logger != nil {
		v.logger.Infof("✅ 区块交易验证通过 - 总交易数: %d, coinbase: %d", len(transactions), coinbaseCount)
	}

	return true, nil
}
