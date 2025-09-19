// Package lifecycle 提供交易生命周期管理 - 费用估算服务
//
// 🎯 **职责定位**：TransactionManager费用估算接口的专门实现
//
// 本文件实现公共接口`TransactionManager.EstimateTransactionFee`方法，
// 负责为交易提供准确的费用估算和优化建议。
//
// 🏗️ **架构分层**：
// - 本文件：公共接口适配层（费用估算逻辑）
// - manager.go：顶层协调层（方法委托和依赖注入）
// - fee/子系统：专业费用计算和优化（外部依赖）
//
// 📋 **核心功能**：
// - 基础费用估算：根据交易大小和类型计算基础手续费
// - 网络费用调整：根据网络拥堵情况动态调整费用
// - 执行费用费用计算：智能合约和AI推理的执行费用费用估算
// - 优化策略建议：费用优化和确认时间权衡建议
//
// 💡 **设计价值**：
// - 准确估算：基于实时网络状况的精确费用计算
// - 用户友好：简洁的费用数值，避免复杂的费用结构
// - 性能优化：缓存常用的估算结果，提升响应速度
// - 策略灵活：支持不同的费用策略和优化目标
package lifecycle

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/blockchain/transaction/fee"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// TransactionFeeEstimationService 交易费用估算服务
//
// 🎯 **TransactionManager费用接口的专门实现**
//
// 负责实现公共接口中的交易费用估算相关方法，提供准确
// 的费用计算和优化策略建议。
//
// 💡 **核心价值**：
// - ✅ **准确估算**：基于交易大小、类型、网络状况的综合计算
// - ✅ **实时调整**：根据内存池状况和网络拥堵动态调整
// - ✅ **策略优化**：提供费用与确认时间的最优平衡建议
// - ✅ **缓存优化**：缓存常用的估算结果，提升响应性能
//
// 📝 **费用计算模型**：
// ```
// 总费用 = 基础费用 + 大小费用 + 网络调整费用 + 特殊费用
//
// 基础费用：固定的网络基础费用（防垃圾交易）
// 大小费用：按交易字节数计算的存储费用
// 网络调整：根据网络拥堵程度的动态调整
// 特殊费用：合约执行费用费用、AI推理费用等
// ```
//
// 📊 **费用策略**：
// - **经济模式**：最低费用，确认时间较长
// - **标准模式**：平衡费用，正常确认时间
// - **快速模式**：较高费用，优先确认
// - **紧急模式**：最高费用，最快确认
//
// 🔄 **缓存策略**：
// - **基础费率**：长期缓存（1小时）
// - **网络状况**：中期缓存（5分钟）
// - **具体估算**：短期缓存（30秒）
type TransactionFeeEstimationService struct {
	logger      log.Logger                   // 日志记录器（可选）
	feeManager  *fee.Manager                 // 费用系统管理器
	cacheStore  storage.MemoryStore          // 估算结果缓存
	utxoManager repository.UTXOManager       // UTXO管理器
	repository  repository.RepositoryManager // 区块链数据仓储（用于回溯获取TxOutput）
	cacheConfig *internal.CacheConfig        // 缓存配置
}

// NewTransactionFeeEstimationService 创建交易费用估算服务
//
// 🎯 **服务工厂方法**
//
// 创建完整的交易费用估算服务实例，集成所有必要的依赖服务。
//
// 💡 **参数说明**：
//   - logger: 日志记录器（可选，传nil则不记录日志）
//   - feeManager: 费用系统管理器
//   - cacheStore: 估算结果缓存存储
//   - utxoManager: UTXO管理器
//   - repository: 区块链数据仓储（用于回溯获取TxOutput）
//
// 💡 **返回值说明**：
//   - *TransactionFeeEstimationService: 费用估算服务实例
func NewTransactionFeeEstimationService(
	logger log.Logger,
	feeManager *fee.Manager,
	cacheStore storage.MemoryStore,
	utxoManager repository.UTXOManager,
	repository repository.RepositoryManager,
) *TransactionFeeEstimationService {
	if feeManager == nil {
		if logger != nil {
			logger.Warn("费用管理器为nil，功能可能受限")
		}
	}
	if cacheStore == nil {
		if logger != nil {
			logger.Warn("缓存存储为nil，将跳过缓存功能")
		}
	}
	if utxoManager == nil {
		if logger != nil {
			logger.Warn("UTXO管理器为nil，功能可能受限")
		}
	}
	if repository == nil {
		if logger != nil {
			logger.Warn("区块链仓储为nil，无法回溯获取TxOutput")
		}
	}

	return &TransactionFeeEstimationService{
		logger:      logger,
		feeManager:  feeManager,
		cacheStore:  cacheStore,
		utxoManager: utxoManager,
		repository:  repository,
		cacheConfig: internal.GetDefaultCacheConfig(),
	}
}

// EstimateTransactionFee 估算交易费用（公共接口实现）
//
// 🎯 **TransactionManager.EstimateTransactionFee接口实现**
//
// 为指定的交易计算准确的手续费估算，帮助用户在提交前
// 了解交易成本并选择合适的费用策略。
//
// 📝 **估算流程**：
// 1. **交易分析阶段**：
//   - 根据交易哈希获取交易数据
//   - 分析交易类型和复杂度
//   - 计算交易的序列化大小
//
// 2. **基础费用计算阶段**：
//   - 计算网络基础费用（固定部分）
//   - 按字节大小计算存储费用
//   - 应用交易类型的费率系数
//
// 3. **特殊费用计算阶段**：
//   - 智能合约调用：估算执行费用消耗和执行费用费用
//   - AI模型推理：估算计算资源和推理费用
//   - 资源部署：估算存储和验证费用
//
// 4. **网络调整阶段**：
//   - 获取当前网络拥堵状况
//   - 根据内存池状态调整费用倍数
//   - 应用动态定价策略
//
// 5. **结果优化阶段**：
//   - 提供多种费用策略选择
//   - 计算预期确认时间
//   - 缓存估算结果以优化性能
//
// 📊 **费用计算公式**：
// ```
// 基础费用 = 网络基础费 + (交易大小 × 字节费率)
// 执行费用费用 = 执行费用消耗量 × 执行费用价格
// 网络费用 = 基础费用 × 拥堵系数
// 总费用 = 基础费用 + 执行费用费用 + 网络调整费
// ```
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时控制
//   - txHash: 未签名交易哈希（32字节，用于获取交易数据）
//
// 💡 **返回值说明**：
//   - uint64: 预估费用（以原生代币的最小单位计算，如wei）
//   - error: 估算错误，nil表示估算成功
//
// 💡 **调用示例**：
//
//	service := NewTransactionFeeEstimationService(logger)
//	estimatedFee, err := service.EstimateTransactionFee(ctx, txHash)
//	if err != nil {
//	    log.Errorf("费用估算失败: %v", err)
//	    return 0, fmt.Errorf("估算失败: %w", err)
//	}
//
//	// 转换为用户友好的格式
//	feeInTokens := float64(estimatedFee) / 1e18  // 假设18位精度
//	log.Infof("预估手续费: %.6f 原生币", feeInTokens)
//
//	// 费用合理性检查
//	if estimatedFee > maxAcceptableFee {
//	    log.Warn("手续费较高，建议稍后再试")
//	}
//
// ⚠️ **注意事项**：
// - 估算结果是基于当前网络状况的预测值，实际费用可能有所差异
// - 网络拥堵时费用可能快速变化，建议及时重新估算
// - 复杂交易（如智能合约）的估算可能需要更多时间
// - 建议为估算结果预留10-20%的缓冲余量
func (s *TransactionFeeEstimationService) EstimateTransactionFee(
	ctx context.Context,
	txHash []byte,
) (uint64, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("开始估算交易费用 - 哈希: %x", txHash[:8]))
	}

	// 1. 基础参数验证
	if len(txHash) != 32 {
		err := fmt.Errorf("交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
		if s.logger != nil {
			s.logger.Warn(err.Error())
		}
		return 0, err
	}

	// 2. 检查费用缓存
	if cachedFee, found := s.getFeeFromCache(ctx, txHash); found {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("缓存命中，返回缓存的费用估算: %d", cachedFee))
		}
		return cachedFee, nil
	}

	// 3. 从缓存获取交易对象
	tx, err := s.getTransactionFromCache(ctx, txHash)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("获取交易数据失败: %v", err))
		}
		return 0, fmt.Errorf("获取交易数据失败: %w", err)
	}

	// 4. 使用费用系统进行估算
	if s.feeManager == nil {
		// 如果费用管理器不可用，使用简化估算
		estimatedFee := s.estimateBasicFee(tx)
		s.cacheFeeEstimation(ctx, txHash, estimatedFee)
		return estimatedFee, nil
	}

	// 5. 使用完整的费用系统进行精确估算
	feeEstimate, err := s.feeManager.EstimateFee(ctx, tx, s.createUTXOFetcher())
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("费用系统估算失败: %v", err))
		}
		// 降级到基础估算
		estimatedFee := s.estimateBasicFee(tx)
		s.cacheFeeEstimation(ctx, txHash, estimatedFee)
		return estimatedFee, nil
	}

	// 6. 转换为标准uint64格式（选择标准估算）
	standardFee := s.convertFeeEstimateToUint64(feeEstimate)

	// 7. 缓存估算结果
	s.cacheFeeEstimation(ctx, txHash, standardFee)

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("费用估算完成 - 标准费用: %d, 机制: %s",
			standardFee, feeEstimate.Mechanism))
	}

	return standardFee, nil
}

// ============================================================================
//                              辅助方法实现
// ============================================================================

// getTransactionFromCache 从缓存获取交易对象
//
// 🎯 **获取要估算费用的交易对象**
//
// 从缓存中获取已构建的交易对象，用于费用估算。
// 支持未签名和已签名交易的查找。
func (s *TransactionFeeEstimationService) getTransactionFromCache(
	ctx context.Context,
	txHash []byte,
) (*pbtx.Transaction, error) {
	if s.cacheStore == nil {
		return nil, fmt.Errorf("缓存存储服务不可用")
	}

	// 首先尝试获取已签名交易
	tx, found, err := internal.GetSignedTransactionFromCache(ctx, s.cacheStore, txHash, s.logger)
	if err != nil {
		return nil, fmt.Errorf("查询已签名交易缓存失败: %w", err)
	}
	if found && tx != nil {
		return tx, nil
	}

	// 再尝试获取未签名交易
	tx, found, err = internal.GetUnsignedTransactionFromCache(ctx, s.cacheStore, txHash, s.logger)
	if err != nil {
		return nil, fmt.Errorf("查询未签名交易缓存失败: %w", err)
	}
	if found && tx != nil {
		return tx, nil
	}

	return nil, fmt.Errorf("未找到交易缓存: %x", txHash[:8])
}

// estimateBasicFee 简化费用估算
//
// 🎯 **基础费用估算fallback方法**
//
// 当完整的费用系统不可用时，提供简化的费用估算。
// 基于交易大小和复杂度提供基本的估算结果。
func (s *TransactionFeeEstimationService) estimateBasicFee(tx *pbtx.Transaction) uint64 {
	if tx == nil {
		return 21000 // 最小基础费用
	}

	// 基础费用：21000（类似以太坊的基础执行费用费）
	baseFee := uint64(21000)

	// 输入输出费用
	inputFee := uint64(len(tx.Inputs)) * 500   // 每个输入500单位
	outputFee := uint64(len(tx.Outputs)) * 300 // 每个输出300单位

	// 复杂性费用（基于输出类型）
	complexityFee := uint64(0)
	for _, output := range tx.Outputs {
		if output.GetResource() != nil {
			complexityFee += 5000 // 资源部署/调用额外费用
		}
		if output.GetState() != nil {
			complexityFee += 2000 // 状态输出额外费用
		}
	}

	totalFee := baseFee + inputFee + outputFee + complexityFee

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("简化费用估算 - 基础: %d, 输入: %d, 输出: %d, 复杂性: %d, 总计: %d",
			baseFee, inputFee, outputFee, complexityFee, totalFee))
	}

	return totalFee
}

// createUTXOFetcher 创建UTXO获取器
//
// 🎯 **为费用系统创建UTXO查询回调**
//
// 创建费用系统所需的UTXO查询回调函数，用于获取交易输入引用的UTXO。
func (s *TransactionFeeEstimationService) createUTXOFetcher() fee.UTXOFetcher {
	return func(ctx context.Context, outpoint *pbtx.OutPoint) (*pbtx.TxOutput, error) {
		if s.utxoManager == nil {
			return nil, fmt.Errorf("UTXO管理器不可用")
		}

		if outpoint == nil {
			return nil, fmt.Errorf("输出点为空")
		}

		// 使用UTXO管理器获取UTXO
		utxo, err := s.utxoManager.GetUTXO(ctx, outpoint)
		if err != nil {
			return nil, fmt.Errorf("获取UTXO失败: %w", err)
		}

		if utxo == nil {
			return nil, fmt.Errorf("UTXO不存在: %x:%d", outpoint.GetTxId()[:8], outpoint.GetOutputIndex())
		}

		// 将UTXO转换为TxOutput格式
		// 注意：UTXO可能有cached_output或者需要从区块链回溯
		// 首先尝试获取缓存的输出
		if cachedOutput := utxo.GetCachedOutput(); cachedOutput != nil {
			return cachedOutput, nil
		}

		// 如果没有缓存输出，从区块链回溯获取
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("UTXO缓存输出为空，开始从区块链回溯获取 - OutPoint: %x:%d",
				utxo.Outpoint.TxId, utxo.Outpoint.OutputIndex))
		}

		return s.getTxOutputFromChain(ctx, utxo.Outpoint)
	}
}

// getTxOutputFromChain 从区块链回溯获取TxOutput
//
// 🔍 **区块链回溯查询核心方法**
//
// 当UTXO缓存输出为空时，通过Repository接口从区块链历史数据中
// 回溯获取对应的TxOutput，并可选择性地写入缓存以优化后续查询。
//
// 📝 **参数说明**：
//   - ctx: 请求上下文，支持超时控制和取消操作
//   - outpoint: UTXO位置引用（交易哈希 + 输出索引）
//
// 📤 **返回值说明**：
//   - *pbtx.TxOutput: 对应的交易输出结构
//   - error: 查询错误（交易不存在、索引越界等）
//
// 🔗 **依赖接口**：
//   - repository.Repository.GetTransaction: 根据交易哈希获取完整交易
//
// ⚡ **性能优化**：
//   - 查询结果可写入短期缓存（TTL受配置控制）
//   - 避免对同一OutPoint的重复回溯查询
func (s *TransactionFeeEstimationService) getTxOutputFromChain(
	ctx context.Context,
	outpoint *pbtx.OutPoint,
) (*pbtx.TxOutput, error) {
	if outpoint == nil {
		return nil, fmt.Errorf("OutPoint不能为空")
	}

	if s.repository == nil {
		return nil, fmt.Errorf("区块链仓储未初始化，无法回溯获取TxOutput")
	}

	// 从区块链获取完整交易
	_, _, tx, err := s.repository.GetTransaction(ctx, outpoint.TxId)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("从区块链获取交易失败 - TxId: %x, 错误: %v", outpoint.TxId, err))
		}
		return nil, fmt.Errorf("获取交易失败: %v", err)
	}

	if tx == nil {
		return nil, fmt.Errorf("交易不存在 - TxId: %x", outpoint.TxId)
	}

	// 检查输出索引边界
	if outpoint.OutputIndex >= uint32(len(tx.Outputs)) {
		return nil, fmt.Errorf("输出索引越界 - 索引: %d, 总输出数: %d",
			outpoint.OutputIndex, len(tx.Outputs))
	}

	// 获取目标输出
	targetOutput := tx.Outputs[outpoint.OutputIndex]
	if targetOutput == nil {
		return nil, fmt.Errorf("目标输出为空 - 索引: %d", outpoint.OutputIndex)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 成功从区块链回溯获取TxOutput - OutPoint: %x:%d",
			outpoint.TxId, outpoint.OutputIndex))
	}

	// TODO: 可选择性地将结果写入短期缓存以优化后续查询
	// 当前版本暂不实现缓存，保持简洁性

	return targetOutput, nil
}

// convertFeeEstimateToUint64 转换费用估算为uint64
//
// 🎯 **将复杂的费用估算结果转换为简单的uint64**
//
// 从费用系统的FeeEstimate结构中提取标准费用，转换为公共接口期望的uint64格式。
func (s *TransactionFeeEstimationService) convertFeeEstimateToUint64(estimate *fee.FeeEstimate) uint64 {
	if estimate == nil {
		return 21000 // 默认最小费用
	}

	// 优先选择标准估算
	if estimate.Standard != nil && estimate.Standard.Sign() > 0 {
		// 检查数值是否在uint64范围内
		if estimate.Standard.IsUint64() {
			return estimate.Standard.Uint64()
		} else {
			// 如果超出uint64范围，使用最大值
			if s.logger != nil {
				s.logger.Warn("费用估算超出uint64范围，使用最大值")
			}
			return ^uint64(0) // uint64最大值
		}
	}

	// 如果标准估算不可用，尝试保守估算
	if estimate.Conservative != nil && estimate.Conservative.Sign() > 0 {
		if estimate.Conservative.IsUint64() {
			return estimate.Conservative.Uint64()
		}
	}

	// 如果保守估算也不可用，尝试快速估算
	if estimate.Fast != nil && estimate.Fast.Sign() > 0 {
		if estimate.Fast.IsUint64() {
			return estimate.Fast.Uint64()
		}
	}

	// 如果所有估算都不可用，返回默认值
	return 21000
}

// getFeeFromCache 从缓存获取费用估算
//
// 🎯 **优化高频估算请求的性能**
//
// 通过缓存机制减少重复的费用计算，提升估算响应速度。
func (s *TransactionFeeEstimationService) getFeeFromCache(
	ctx context.Context,
	txHash []byte,
) (uint64, bool) {
	if s.cacheStore == nil {
		return 0, false
	}

	// 生成费用缓存键
	cacheKey := internal.GenerateCacheKey(internal.FeeEstimatePrefix, txHash)

	// 从缓存获取数据
	data, found, err := s.cacheStore.Get(ctx, cacheKey)
	if err != nil || !found {
		if s.logger != nil && err != nil {
			s.logger.Debug(fmt.Sprintf("费用缓存读取失败: %v", err))
		}
		return 0, false
	}

	// 验证数据长度
	if len(data) != 8 {
		if s.logger != nil {
			s.logger.Warn("费用缓存数据长度错误")
		}
		return 0, false
	}

	// 解析uint64费用
	fee := uint64(data[0])<<56 |
		uint64(data[1])<<48 |
		uint64(data[2])<<40 |
		uint64(data[3])<<32 |
		uint64(data[4])<<24 |
		uint64(data[5])<<16 |
		uint64(data[6])<<8 |
		uint64(data[7])

	return fee, true
}

// cacheFeeEstimation 缓存费用估算结果
//
// 🎯 **缓存估算结果以优化性能**
//
// 将费用估算结果缓存到本地存储，减少重复计算开销。
func (s *TransactionFeeEstimationService) cacheFeeEstimation(
	ctx context.Context,
	txHash []byte,
	fee uint64,
) {
	if s.cacheStore == nil {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("跳过费用缓存（缓存存储不可用）: %d", fee))
		}
		return
	}

	// 生成费用缓存键
	cacheKey := internal.GenerateCacheKey(internal.FeeEstimatePrefix, txHash)

	// 将uint64转换为字节数组
	feeData := make([]byte, 8)
	feeData[0] = byte(fee >> 56)
	feeData[1] = byte(fee >> 48)
	feeData[2] = byte(fee >> 40)
	feeData[3] = byte(fee >> 32)
	feeData[4] = byte(fee >> 24)
	feeData[5] = byte(fee >> 16)
	feeData[6] = byte(fee >> 8)
	feeData[7] = byte(fee)

	// 使用配置的TTL进行缓存
	ttl := s.cacheConfig.FeeEstimateTTL
	err := s.cacheStore.Set(ctx, cacheKey, feeData, ttl)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("费用缓存失败: %v", err))
		}
		return
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 费用估算已缓存 - 键: %s, 费用: %d, TTL: %v",
			cacheKey, fee, ttl))
	}
}
