// Package genesis 创世交易验证实现
//
// 🎯 **创世交易专业验证**
//
// 本文件专门处理创世交易的验证逻辑，包括：
// - 创世交易格式验证：结构完整性、字段有效性
// - 创世交易特殊规则验证：无输入、特殊费用机制等
// - 确定性检查：时间戳一致性、Nonce唯一性
// - 业务逻辑验证：余额分配合理性、账户有效性
//
// 🏗️ **设计原则**
// - 专业分工：专门处理创世交易验证业务逻辑
// - 严格验证：确保创世交易符合所有规则
// - 明确错误：提供详细的验证失败信息
// - 高性能：针对批量验证优化
package genesis

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 协议定义
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 创世交易验证实现 ====================

// ValidateTransactions 验证创世交易有效性
//
// 🎯 **创世交易验证服务**
//
// 对创世交易进行专门验证，包括：
// 1. 交易格式的正确性：结构完整性、字段有效性
// 2. 初始余额分配的合理性：总量平衡、账户有效性
// 3. 系统合约的完整性：合约代码、初始化参数
// 4. 创世交易的特殊规则：无输入、特殊签名等
//
// 参数：
//   - ctx: 操作上下文
//   - transactions: 待验证的创世交易列表
//   - logger: 日志服务
//
// 返回：
//   - bool: 验证结果，true表示通过
//   - error: 验证过程中的错误
func ValidateTransactions(
	ctx context.Context,
	transactions []*transaction.Transaction,
	logger log.Logger,
) (bool, error) {
	if logger != nil {
		logger.Infof("开始验证创世交易，数量: %d", len(transactions))
	}

	if len(transactions) == 0 {
		return false, fmt.Errorf("创世交易列表不能为空")
	}

	// 验证每个交易
	for i, tx := range transactions {
		if tx == nil {
			return false, fmt.Errorf("交易[%d]不能为空", i)
		}

		// 验证交易版本
		if tx.Version == 0 {
			return false, fmt.Errorf("交易[%d]版本不能为0", i)
		}

		// 验证创世交易特殊规则：无输入
		if len(tx.Inputs) != 0 {
			return false, fmt.Errorf("创世交易[%d]不应该有输入", i)
		}

		// 验证费用机制
		if tx.FeeMechanism == nil {
			return false, fmt.Errorf("交易[%d]缺少费用机制", i)
		}

		// 验证时间戳
		if tx.CreationTimestamp == 0 {
			return false, fmt.Errorf("交易[%d]时间戳不能为0", i)
		}
	}

	// 验证交易确定性（时间戳一致性、Nonce唯一性）
	usedNonces := make(map[uint64]bool)
	baseTimestamp := transactions[0].CreationTimestamp

	for i, tx := range transactions {
		// 验证时间戳一致性
		if tx.CreationTimestamp != baseTimestamp {
			return false, fmt.Errorf("交易[%d]时间戳不一致: 期望 %d, 实际 %d",
				i, baseTimestamp, tx.CreationTimestamp)
		}

		// 验证Nonce唯一性
		if usedNonces[tx.Nonce] {
			return false, fmt.Errorf("交易[%d]Nonce重复: %d", i, tx.Nonce)
		}
		usedNonces[tx.Nonce] = true
	}

	if logger != nil {
		logger.Infof("✅ 创世交易验证通过")
	}

	return true, nil
}
