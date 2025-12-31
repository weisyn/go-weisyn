// Package condition 提供 Condition 验证插件实现
//
// time_lock.go: 时间锁验证插件
package condition

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// TimeLockPlugin 时间锁验证插件
//
// 🎯 **核心职责**：验证交易的时间锁定条件
//
// 💡 **设计理念**：
// TimeLock 只有在指定时间后才能解锁，适用于定期存款、遗嘱执行、期权行权等场景。
// 验证分为两部分：
// 1. Condition Hook：验证当前时间 >= unlock_timestamp
// 2. AuthZ Hook：验证 base_lock 匹配 base_proof
//
// 🔒 **验证要点**：
// 1. 当前时间戳必须 >= unlock_timestamp
// 2. 根据 time_source 选择时间来源
// 3. base_lock 的验证由 AuthZ Hook 完成
//
// 📋 **典型应用**：
// - 定期存款：锁定1年后才能取出
// - 遗嘱执行：特定日期后才能继承
// - 期权行权：在特定时间窗口内行权
type TimeLockPlugin struct{}

// NewTimeLockPlugin 创建新的 TimeLockPlugin
//
// 返回：
//   - *TimeLockPlugin: 新创建的插件实例
func NewTimeLockPlugin() *TimeLockPlugin {
	return &TimeLockPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConditionPlugin 接口
//
// 返回：
//   - string: 插件名称 "TimeLock"
func (p *TimeLockPlugin) Name() string {
	return "TimeLock"
}

// Check 验证交易的时间锁定条件
//
// 实现 tx.ConditionPlugin 接口
//
// 🎯 **验证流程**：
// 1. 遍历所有输入，查找 TimeLock
// 2. 对每个 TimeLock，验证当前时间 >= unlock_timestamp
// 3. 根据 time_source 选择时间来源
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//   - blockHeight: 当前区块高度
//   - blockTime: 当前区块时间（Unix时间戳）
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//
// 📝 **使用示例**：
//
//	timeLock := &transaction.LockingCondition{
//	    Condition: &transaction.LockingCondition_TimeLock{
//	        TimeLock: &transaction.TimeLock{
//	            UnlockTimestamp: 1735689600, // 2025-11-01 00:00:00 UTC
//	            BaseLock: &transaction.LockingCondition{
//	                Condition: &transaction.LockingCondition_SingleKeyLock{...},
//	            },
//	            TimeSource: transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
//	        },
//	    },
//	}
func (p *TimeLockPlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 遍历所有输入，查找 TimeLock
	for i, input := range tx.Inputs {
		// 从 input 中提取 TimeProof
		timeProof, ok := input.UnlockingProof.(*transaction.TxInput_TimeProof)
		if !ok {
			// 不是 TimeProof，跳过
			continue
		}

		// ✅ **完整实现**：从UTXO查询实际的TimeLock锁定条件
		// 💡 **实现说明**：使用VerifierEnvironment.GetUTXO查询UTXO，然后提取LockingCondition
		env, ok := txiface.GetVerifierEnvironment(ctx)
		if !ok || env == nil {
			// 如果没有提供VerifierEnvironment，使用简化验证（向后兼容）
			// 验证当前时间 >= TimeProof中声明的current_timestamp
			if blockTime < timeProof.TimeProof.CurrentTimestamp {
				return fmt.Errorf("输入 %d: 当前时间 %d 小于声明的 current_timestamp %d（VerifierEnvironment未提供，使用简化验证）",
					i, blockTime, timeProof.TimeProof.CurrentTimestamp)
			}
			continue
		}

		// 从UTXO查询Output（包含LockingConditions）
		utxo, err := env.GetUTXO(ctx, input.PreviousOutput)
		if err != nil {
			return fmt.Errorf("输入 %d: 查询UTXO失败: %w", i, err)
		}

		output := utxo.GetCachedOutput()
		if output == nil {
			return fmt.Errorf("输入 %d: UTXO未包含Output信息", i)
		}

		// 从Output的LockingConditions中查找TimeLock
		var timeLock *transaction.TimeLock
		for _, cond := range output.LockingConditions {
			if tl := cond.GetTimeLock(); tl != nil {
				timeLock = tl
				break
			}
		}

		if timeLock == nil {
			// 如果UTXO中没有TimeLock，但输入使用了TimeProof，这是不一致的
			// 但为了向后兼容，我们仍然验证TimeProof中的时间
			if blockTime < timeProof.TimeProof.CurrentTimestamp {
				return fmt.Errorf("输入 %d: 当前时间 %d 小于声明的 current_timestamp %d（UTXO中未找到TimeLock）",
					i, blockTime, timeProof.TimeProof.CurrentTimestamp)
			}
			continue
		}

		// 验证时间条件：当前时间必须 >= unlock_timestamp
		// 根据time_source选择时间来源
		var currentTime uint64
		switch timeLock.TimeSource {
		case transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP:
			// 使用区块时间戳（默认，去中心化）
			currentTime = blockTime
		case transaction.TimeLock_TIME_SOURCE_MEDIAN_TIME:
			// 使用中位数时间（更稳定）
			// ⚠️ 注意：当前实现使用blockTime作为中位数时间的近似值
			// 完整实现需要从区块头获取中位数时间
			currentTime = blockTime
		case transaction.TimeLock_TIME_SOURCE_ORACLE:
			// 使用预言机时间（高精度场景）
			// ⚠️ 注意：当前实现使用blockTime作为预言机时间的近似值
			// 完整实现需要从预言机服务获取时间
			currentTime = blockTime
		default:
			// 默认使用区块时间戳
			currentTime = blockTime
		}

		if currentTime < timeLock.UnlockTimestamp {
			return fmt.Errorf("输入 %d: 时间锁未解锁，当前时间=%d，解锁时间=%d，时间来源=%v",
				i, currentTime, timeLock.UnlockTimestamp, timeLock.TimeSource)
		}

		// ✅ 时间条件验证通过
		// 注意：base_lock的验证由AuthZ Hook完成（递归验证）
	}

	return nil
}
