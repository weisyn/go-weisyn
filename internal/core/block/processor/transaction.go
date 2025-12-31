// Package processor 实现区块处理服务
package processor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// executeTransactions 验证区块中的所有交易执行结果
//
// 🎯 **交易验证流程**：
// 根据WES的两种输入、三种输出架构，分类验证交易执行结果：
// 1. StateOutput: 验证ZK证明和执行结果哈希
// 2. ResourceOutput: 验证资源生命周期
// 3. AssetOutput: 最终确认交易有效性（已在提交时验证）
// 4. 引用型输入: 验证引用UTXO的有效性
//
// ✅ **职责分离**：
// - UTXO变更 → DataWriter 处理（在后续的 storeBlock 中完成）
// - 引用计数管理 → processReferenceCounts 处理
// - 交易验证 → executeTransactions 处理（本函数）
//
// ❌ **不重新执行智能合约**（合约已在TX层执行）
//
// 参数：
//   - ctx: 上下文
//   - block: 包含交易的区块
//
// 返回：
//   - error: 验证错误
func (s *Service) executeTransactions(ctx context.Context, block *core.Block) error {
	if s.logger != nil {
		s.logger.Debugf("开始验证区块交易执行结果，交易数: %d", len(block.Body.Transactions))
	}

	if block == nil || block.Header == nil {
		return fmt.Errorf("区块/区块头为空")
	}

	// ✅ 强校验：同一块内“引用(ReferenceOnly)”与“消费(Consume)”不能指向同一个 OutPoint。
	// 否则会造成语义不一致：ResourceUTXO 处于被引用状态时禁止消费（utxo.proto 约束）。
	referencedInBlock := make(map[string]struct{})
	for _, tx := range block.Body.Transactions {
		if tx == nil {
			continue
		}
		for _, in := range tx.Inputs {
			if in == nil || in.PreviousOutput == nil {
				continue
			}
			if in.IsReferenceOnly {
				referencedInBlock[outpointKey(in.PreviousOutput)] = struct{}{}
			}
		}
	}

	// 遍历每个交易并分类验证
	for i, tx := range block.Body.Transactions {
		if tx == nil {
			if s.logger != nil {
				s.logger.Warnf("区块第 %d 个交易为空，跳过", i)
			}
			continue
		}

		// ========== 1. 处理StateOutput（ISPC执行的合约调用）==========
		for _, output := range tx.Outputs {
			if output == nil {
				continue
			}

			if stateOutput := output.GetState(); stateOutput != nil {
				// 验证StateOutput的ZK证明和执行结果哈希
				if err := s.verifyStateOutput(ctx, stateOutput, i); err != nil {
					return fmt.Errorf("交易 %d 的StateOutput验证失败: %w", i, err)
				}
			}
		}

		// ========== 2. 处理ResourceOutput（资源交易）==========
		for _, output := range tx.Outputs {
			if output == nil {
				continue
			}

			if resourceOutput := output.GetResource(); resourceOutput != nil {
				// 验证资源生命周期（版本号、过期时间等）
				if err := s.verifyResourceLifecycle(ctx, block.Header.Timestamp, tx, resourceOutput, i); err != nil {
					return fmt.Errorf("交易 %d 的资源生命周期验证失败: %w", i, err)
				}
			}
		}

		// ========== 3. 处理AssetOutput（普通交易）==========
		// 注意：普通交易的验证已经在提交时完成（通过TxVerifier）
		// 这里主要是最终确认，确保交易在区块中的有效性
		// UTXO变更已经在DataWriter中处理，这里可能只需要记录日志和统计

		// ========== 4. 处理引用型输入 ==========
		for _, input := range tx.Inputs {
			if input == nil {
				continue
			}

			if input.IsReferenceOnly {
				// 验证引用的UTXO是否存在且有效
				if err := s.verifyReferenceUTXO(ctx, input.PreviousOutput, i); err != nil {
					return fmt.Errorf("交易 %d 的引用UTXO验证失败: %w", i, err)
				}
			} else {
				// 消费型输入：禁止消费“本块内已被引用”的 UTXO（并发引用语义）
				if input.PreviousOutput != nil {
					if _, ok := referencedInBlock[outpointKey(input.PreviousOutput)]; ok {
						return fmt.Errorf("交易 %d 试图消费一个在同一块中被引用的UTXO: txId=%x outputIndex=%d",
							i,
							input.PreviousOutput.TxId[:minHelper(8, len(input.PreviousOutput.TxId))],
							input.PreviousOutput.OutputIndex,
						)
					}
				}
			}
		}

		// ========== 5. 记录日志和统计 ==========
		if s.logger != nil {
			s.logger.Debugf("✅ 交易 %d 验证完成 (输入数=%d, 输出数=%d)",
				i, len(tx.Inputs), len(tx.Outputs))
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 所有交易验证完成，总计: %d", len(block.Body.Transactions))
	}

	return nil
}

func outpointKey(o *transaction.OutPoint) string {
	if o == nil {
		return ""
	}
	// txid 直接作为 bytes 拼接 string（不用于持久化，仅用于本次验证的 map key）
	return string(o.TxId) + ":" + fmt.Sprintf("%d", o.OutputIndex)
}

// verifyStateOutput 验证StateOutput的ZK证明和执行结果哈希
//
// 🎯 **验证内容**：
// 1. 验证ZK证明（必须）
// 2. 验证执行结果哈希的一致性（可选，但推荐）
//
// 参数：
//   - ctx: 上下文
//   - stateOutput: StateOutput对象
//   - txIndex: 交易索引（用于错误信息）
//
// 返回：
//   - error: 验证错误
func (s *Service) verifyStateOutput(ctx context.Context, stateOutput *transaction.StateOutput, txIndex int) error {
	if stateOutput == nil {
		return fmt.Errorf("StateOutput为空")
	}

	// 1. 验证ZK证明（必须）
	if stateOutput.ZkProof == nil {
		return fmt.Errorf("StateOutput缺少ZK证明")
	}

	// 在任何环境下，缺失 zkProofService 都视为致命错误，防止在生产链上“裸奔”
	if s.zkProofService == nil {
		return fmt.Errorf("zkProofService 未注入，无法验证 StateOutput 的 ZK 证明（交易 %d）", txIndex)
	}

	// 验证ZK证明
	valid, err := s.zkProofService.VerifyStateProof(ctx, stateOutput.ZkProof)
	if err != nil {
		return fmt.Errorf("ZK证明验证过程出错: %w", err)
	}
	if !valid {
		return fmt.Errorf("ZK证明验证失败")
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 交易 %d 的ZK证明验证通过", txIndex)
	}

	// 2. ✅ 强校验：ExecutionResultHash 必须与 ZKProof.PublicInputs 中的某个 32-byte 输入一致
	// 说明：
	// - 不假设 public_inputs 的固定 index（避免不同电路/版本不兼容）
	// - 只要存在一个 32-byte public_input 与 execution_result_hash 相同，即视为一致
	if len(stateOutput.ExecutionResultHash) > 0 {
		if len(stateOutput.ExecutionResultHash) != 32 {
			return fmt.Errorf("执行结果哈希长度错误: 期望32字节, 得到%d字节", len(stateOutput.ExecutionResultHash))
		}
		if stateOutput.ZkProof == nil {
			return fmt.Errorf("StateOutput缺少ZK证明，无法校验 execution_result_hash")
		}
		matched := false
		for _, pi := range stateOutput.ZkProof.PublicInputs {
			if len(pi) == 32 && string(pi) == string(stateOutput.ExecutionResultHash) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("执行结果哈希与ZK公开输入不一致：execution_result_hash 未出现在 public_inputs 中（交易 %d）", txIndex)
		}
		if s.logger != nil {
			s.logger.Debugf("✅ 交易 %d 的执行结果哈希与ZK公开输入一致", txIndex)
		}
	}

	return nil
}

// verifyResourceLifecycle 验证ResourceOutput的资源生命周期
//
// 🎯 **验证内容**：
// 1. 验证资源版本号
// 2. 验证资源过期时间（如果设置了TTL）
// 3. 验证资源创建时间戳
//
// 参数：
//   - ctx: 上下文
//   - resourceOutput: ResourceOutput对象
//   - txIndex: 交易索引（用于错误信息）
//
// 返回：
//   - error: 验证错误
func (s *Service) verifyResourceLifecycle(ctx context.Context, blockTimestamp uint64, tx *transaction.Transaction, resourceOutput *transaction.ResourceOutput, txIndex int) error {
	if resourceOutput == nil {
		return fmt.Errorf("ResourceOutput为空")
	}

	if resourceOutput.Resource == nil {
		return fmt.Errorf("ResourceOutput缺少Resource定义")
	}

	// 1. ✅ 版本语义 + 严格递增规则（非向后兼容）
	// - “新资源”：不消费任何 ResourceUTXO → version 必须为 1
	// - “更新资源”：消费且仅消费 1 个 ResourceUTXO → version 必须为 prev_version + 1
	verStr := strings.TrimSpace(resourceOutput.Resource.Version)
	if verStr == "" {
		return fmt.Errorf("资源 version 不能为空（交易 %d）", txIndex)
	}
	ver, err := strconv.ParseUint(verStr, 10, 64)
	if err != nil {
		return fmt.Errorf("资源 version 必须为十进制整数（交易 %d）: %w", txIndex, err)
	}
	if ver == 0 {
		return fmt.Errorf("资源 version 必须 >= 1（交易 %d）", txIndex)
	}

	if s.utxoQuery == nil {
		return fmt.Errorf("utxoQuery 未注入，无法校验资源版本规则（交易 %d）", txIndex)
	}
	if tx == nil {
		return fmt.Errorf("交易为空，无法校验资源版本规则（交易 %d）", txIndex)
	}

	var prevResUTXO *utxopb.UTXO
	var consumedResCount int
	for _, in := range tx.Inputs {
		if in == nil || in.PreviousOutput == nil || in.IsReferenceOnly {
			continue
		}
		u, err := s.utxoQuery.GetUTXO(ctx, in.PreviousOutput)
		if err != nil || u == nil {
			return fmt.Errorf("获取输入UTXO失败（资源版本规则）: %w", err)
		}
		if u.GetCategory() == utxopb.UTXOCategory_UTXO_CATEGORY_RESOURCE {
			consumedResCount++
			prevResUTXO = u
		}
	}

	if consumedResCount == 0 {
		if ver != 1 {
			return fmt.Errorf("新资源的 version 必须为 1，但得到 %d（交易 %d）", ver, txIndex)
		}
	} else if consumedResCount == 1 {
		// 从被消费的 ResourceUTXO 中提取旧版本号（必须是 cached_output 的 ResourceOutput）
		cached := prevResUTXO.GetCachedOutput()
		if cached == nil || cached.GetResource() == nil || cached.GetResource().Resource == nil {
			return fmt.Errorf("被消费的 ResourceUTXO 缺少 cached_output.resource，无法校验版本递增（交易 %d）", txIndex)
		}
		prevVerStr := strings.TrimSpace(cached.GetResource().Resource.Version)
		prevVer, err := strconv.ParseUint(prevVerStr, 10, 64)
		if err != nil {
			return fmt.Errorf("旧资源 version 非法（必须为十进制整数）: %w", err)
		}
		if ver != prevVer+1 {
			return fmt.Errorf("资源 version 必须严格递增：prev=%d current=%d（交易 %d）", prevVer, ver, txIndex)
		}
	} else {
		return fmt.Errorf("资源更新交易不允许同时消费多个 ResourceUTXO（count=%d，交易 %d）", consumedResCount, txIndex)
	}

	// 2. 验证资源创建时间戳
	if resourceOutput.CreationTimestamp > 0 {
		// ✅ 共识一致性：使用 blockTimestamp 校验，不使用 wall-clock
		if blockTimestamp > 0 && resourceOutput.CreationTimestamp > blockTimestamp {
			return fmt.Errorf("资源创建时间戳无效: creation=%d 晚于 block_time=%d（交易 %d）",
				resourceOutput.CreationTimestamp, blockTimestamp, txIndex)
		}
	}

	// 3. ✅ 资源 TTL/过期验证（确定性来源：Resource.custom_attributes）
	// 兼容来源（按优先级）：
	// - custom_attributes["expires_at"] / ["expires_at_timestamp"]：绝对过期时间戳（秒）
	// - custom_attributes["ttl_seconds"]：相对TTL（秒），基于 creation_timestamp 计算
	attrs := resourceOutput.Resource.CustomAttributes
	var expiresAt uint64
	if attrs != nil {
		if v := strings.TrimSpace(attrs["expires_at_timestamp"]); v != "" {
			if ts, err := strconv.ParseUint(v, 10, 64); err == nil {
				expiresAt = ts
			} else {
				return fmt.Errorf("资源 expires_at_timestamp 非法（交易 %d）: %w", txIndex, err)
			}
		} else if v := strings.TrimSpace(attrs["expires_at"]); v != "" {
			if ts, err := strconv.ParseUint(v, 10, 64); err == nil {
				expiresAt = ts
			} else {
				return fmt.Errorf("资源 expires_at 非法（交易 %d）: %w", txIndex, err)
			}
		} else if v := strings.TrimSpace(attrs["ttl_seconds"]); v != "" {
			ttl, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return fmt.Errorf("资源 ttl_seconds 非法（交易 %d）: %w", txIndex, err)
			}
			var base uint64
			if resourceOutput.CreationTimestamp > 0 {
				base = resourceOutput.CreationTimestamp
			} else if resourceOutput.Resource.CreatedTimestamp > 0 {
				base = resourceOutput.Resource.CreatedTimestamp
			}
			if base > 0 && ttl > 0 {
				expiresAt = base + ttl
			}
		}
	}
	if expiresAt > 0 && blockTimestamp > 0 && blockTimestamp >= expiresAt {
		return fmt.Errorf("资源已过期: block_time=%d expires_at=%d（交易 %d）", blockTimestamp, expiresAt, txIndex)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 交易 %d 的资源生命周期验证通过", txIndex)
	}

	return nil
}

// verifyReferenceUTXO 验证引用型输入的有效性
//
// 🎯 **验证内容**：
// 1. 验证引用的UTXO是否存在
// 2. 验证引用的UTXO是否有效（未被消费）
// 3. 验证引用的UTXO类型是否允许引用
//
// 参数：
//   - ctx: 上下文
//   - previousOutput: 引用的UTXO输出点
//   - txIndex: 交易索引（用于错误信息）
//
// 返回：
//   - error: 验证错误
func (s *Service) verifyReferenceUTXO(ctx context.Context, previousOutput *transaction.OutPoint, txIndex int) error {
	if previousOutput == nil {
		return fmt.Errorf("引用UTXO的输出点为空")
	}

	// 1. 验证输出点基本字段
	if len(previousOutput.TxId) == 0 {
		return fmt.Errorf("引用UTXO的交易ID为空")
	}

	// 2. 验证引用的UTXO是否存在
	// 注意：这里需要查询UTXO集合，检查UTXO是否存在
	// 由于UTXO变更已经在DataWriter中处理，这里应该查询最新的UTXO状态
	if s.utxoQuery == nil {
		return fmt.Errorf("utxoQuery 未注入，无法验证引用UTXO存在性（交易 %d）", txIndex)
	}

	// 查询UTXO是否存在
	utxo, err := s.utxoQuery.GetUTXO(ctx, previousOutput)
	if err != nil {
		return fmt.Errorf("查询引用UTXO失败: %w", err)
	}
	if utxo == nil {
		return fmt.Errorf("引用的UTXO不存在: txHash=%x, outputIndex=%d",
			previousOutput.TxId, previousOutput.OutputIndex)
	}

	// 3. ✅ 强校验：仅 ResourceUTXO 允许被引用（EUTXO 两类输入语义）
	if utxo.GetCategory() != utxopb.UTXOCategory_UTXO_CATEGORY_RESOURCE {
		return fmt.Errorf("引用型输入只允许引用 ResourceUTXO，但得到: category=%s txId=%x outputIndex=%d",
			utxo.GetCategory().String(),
			previousOutput.TxId[:minHelper(8, len(previousOutput.TxId))],
			previousOutput.OutputIndex,
		)
	}

	// 4. 生命周期状态检查：ResourceUTXO 在 AVAILABLE/REFERENCED 下可被引用
	switch utxo.GetStatus() {
	case utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
		utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED:
		// ok
	default:
		return fmt.Errorf("引用型输入引用了非可用/非引用态的 ResourceUTXO: status=%s txId=%x outputIndex=%d",
			utxo.GetStatus().String(),
			previousOutput.TxId[:minHelper(8, len(previousOutput.TxId))],
			previousOutput.OutputIndex,
		)
	}

	// 5. 并发引用上限检查（若设置）
	if rc := utxo.GetResourceConstraints(); rc != nil {
		max := rc.GetMaxConcurrentReferences()
		if max > 0 && rc.GetReferenceCount() >= max {
			return fmt.Errorf("ResourceUTXO 并发引用超限: ref_count=%d max=%d txId=%x outputIndex=%d",
				rc.GetReferenceCount(),
				max,
				previousOutput.TxId[:minHelper(8, len(previousOutput.TxId))],
				previousOutput.OutputIndex,
			)
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 交易 %d 的引用UTXO验证通过: category=RESOURCE status=%s txId=%x outputIndex=%d",
			txIndex,
			utxo.GetStatus().String(),
			previousOutput.TxId[:minHelper(8, len(previousOutput.TxId))],
			previousOutput.OutputIndex,
		)
	}

	return nil
}
