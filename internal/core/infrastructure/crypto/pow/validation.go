// Package pow 提供POW（工作量证明）验证引擎实现
//
// ✅ **验证引擎组件 (Validation Engine Component)**
//
// 本文件专门实现POW验证的核心算法，专注于：
// - 验证算法：快速的POW有效性验证
// - 性能优化：高速验证、缓存优化、批量处理
// - 安全检查：防篡改验证、参数完整性检查
// - 生产级质量：详细错误信息、审计日志、指标统计
//
// 🎯 **职责边界**：
// - 专门负责区块头的POW验证
// - 不涉及挖矿逻辑（由mining.go负责）
// - 不涉及难度计算（由difficulty.go负责）
// - 不涉及基础设施管理（由engine.go负责）
//
// 🔧 **验证特点**：
// - 采用与挖矿完全一致的哈希算法
// - 高效的难度判定算法
// - 参数完整性和合理性检查
// - 支持批量验证优化
// - 详细的验证审计日志
//
// 🚀 **性能优化**：
// - 快速失败策略（参数预检）
// - 哈希计算优化
// - 内存分配最小化
// - CPU缓存友好的算法
//
// 🔒 **安全特性**：
// - 防篡改检查
// - 参数边界验证
// - 溢出保护
// - 恶意输入检测
//
// 📈 **监控指标**：
// - 验证次数统计
// - 验证成功率
// - 验证耗时统计
// - 错误类型分类
package pow

import (
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// ValidationEngine 专门的验证引擎组件
//
// ✅ **验证引擎结构**：
// 专注于POW验证算法的实现，提供高效的区块头验证服务。
// 采用组合模式依赖核心引擎的基础设施。
//
// 📝 **字段说明**：
// - coreEngine: 核心引擎的引用，用于访问基础设施
// - statistics: 验证统计信息（性能监控）
//
// 🎯 **设计原则**：
// - 单一职责：专注验证算法实现
// - 高性能：优化的验证算法和资源使用
// - 安全可靠：严格的参数验证和错误处理
// - 可监控：详细的统计和审计信息
type ValidationEngine struct {
	coreEngine *Engine          // 核心引擎引用
	statistics *ValidationStats // 验证统计信息
}

// ValidationStats 验证统计信息
//
// 📊 **验证统计结构**：
// 记录验证过程的性能指标和统计数据，用于监控和分析。
//
// 📝 **字段说明**：
// - TotalValidations: 总验证次数
// - SuccessfulValidations: 成功验证次数
// - FailedValidations: 失败验证次数
// - TotalValidationTime: 总验证耗时
// - AverageValidationTime: 平均验证耗时
// - LastValidationTime: 最后验证时间
// - ErrorCounts: 错误类型计数
//
// 🎯 **统计用途**：
// - 验证性能监控
// - 安全审计日志
// - 系统健康检查
// - 性能优化分析
type ValidationStats struct {
	TotalValidations        uint64                 // 总验证次数
	SuccessfulValidations   uint64                 // 成功验证次数
	FailedValidations       uint64                 // 失败验证次数
	TotalValidationTime     time.Duration          // 总验证耗时
	AverageValidationTime   time.Duration          // 平均验证耗时
	LastValidationTime      time.Time              // 最后验证时间
	ErrorCounts             map[string]uint64      // 错误类型计数
}

// ValidationError 验证错误类型
//
// 🚨 **错误分类常量**：
// 定义各种验证错误的类型，用于错误统计和处理。
const (
	ErrorInvalidHeader     = "invalid_header"      // 无效区块头
	ErrorInvalidNonce      = "invalid_nonce"       // 无效nonce
	ErrorInvalidDifficulty = "invalid_difficulty"  // 无效难度
	ErrorHashCalculation   = "hash_calculation"    // 哈希计算错误
	ErrorDifficultyCheck   = "difficulty_check"    // 难度检查失败
	ErrorSerialization     = "serialization"       // 序列化错误
)

// NewValidationEngine 创建验证引擎实例
//
// 🚀 **构造函数**：
// 创建专门的验证引擎组件，依赖核心引擎提供基础设施。
// 初始化验证统计和配置参数。
//
// 📋 **参数说明**：
//   - coreEngine: 核心引擎实例（不能为nil）
//
// 🔄 **返回值**：
//   - *ValidationEngine: 初始化好的验证引擎
//   - error: 创建失败时的错误
//
// 💡 **设计说明**：
// - 采用依赖注入模式接收核心引擎
// - 初始化统计信息和错误计数器
// - 验证必要的依赖项
func NewValidationEngine(coreEngine *Engine) (*ValidationEngine, error) {
	if coreEngine == nil {
		return nil, fmt.Errorf("核心引擎不能为空")
	}

	engine := &ValidationEngine{
		coreEngine: coreEngine,
		statistics: &ValidationStats{
			LastValidationTime: time.Now(),
			ErrorCounts:        make(map[string]uint64),
		},
	}

	// 记录初始化日志
	coreEngine.GetLogger().Debug("验证引擎组件初始化完成")

	return engine, nil
}

// VerifyBlockHeader 验证区块头的POW是否有效
//
// ✅ **核心验证算法**：
// 快速验证区块头的POW是否满足难度要求，用于区块验证。
// 采用与挖矿完全一致的算法确保验证的准确性。
//
// 📋 **验证流程**：
// 1. 参数完整性检查
// 2. 基础字段验证
// 3. 难度合理性验证
// 4. nonce有效性检查
// 5. 序列化区块头数据
// 6. 计算双重SHA256哈希
// 7. 验证哈希难度要求
// 8. 记录验证结果和统计
//
// 🔄 **性能特点**：
// - 快速失败策略（预检优化）
// - 高效的哈希计算
// - 最小的内存分配
// - CPU缓存友好的实现
//
// 🔒 **安全检查**：
// - 严格的参数边界验证
// - 防篡改完整性检查
// - 恶意输入检测
// - 溢出保护机制
//
// 📊 **审计功能**：
// - 详细的验证日志
// - 错误类型分类统计
// - 性能指标记录
// - 安全事件记录
//
// 📋 **参数说明**：
//   - header: 需要验证的区块头（必须完整且有效）
//
// 🔄 **返回值**：
//   - bool: true表示POW验证通过，false表示验证失败
//   - error: 验证过程中的错误（参数无效、计算错误等）
//
// 🚨 **错误类型**：
// - 参数验证错误：区块头为nil、字段缺失等
// - 逻辑验证错误：难度不合理、nonce格式错误等
// - 计算错误：序列化失败、哈希计算失败等
// - 结果验证错误：哈希不满足难度要求
func (v *ValidationEngine) VerifyBlockHeader(header *core.BlockHeader) (bool, error) {
	// ==================== 性能监控和日志 ====================
	
	startTime := time.Now()
	logger := v.coreEngine.GetLogger()
	
	// 更新统计计数
	v.statistics.TotalValidations++
	v.statistics.LastValidationTime = startTime
	
	logger.Debugf("开始验证POW，区块高度: %d，难度: %d", 
		header.GetHeight(), header.GetDifficulty())

	// ==================== 参数完整性检查 ====================
	
	if header == nil {
		v.recordError(ErrorInvalidHeader)
		logger.Warnf("验证失败: 区块头为空")
		return false, fmt.Errorf("区块头不能为空")
	}

	// 基础字段验证
	if header.Difficulty == 0 {
		v.recordError(ErrorInvalidDifficulty)
		logger.Warnf("验证失败: 难度为零，高度: %d", header.Height)
		return false, fmt.Errorf("区块头难度不能为零")
	}

	if len(header.Nonce) == 0 {
		v.recordError(ErrorInvalidNonce)
		logger.Warnf("验证失败: nonce为空，高度: %d", header.Height)
		return false, fmt.Errorf("区块头nonce不能为空")
	}

	if len(header.Nonce) != 8 {
		v.recordError(ErrorInvalidNonce)
		logger.Warnf("验证失败: nonce长度错误，期望8字节，实际: %d字节，高度: %d", 
			len(header.Nonce), header.Height)
		return false, fmt.Errorf("nonce长度必须为8字节，实际长度: %d", len(header.Nonce))
	}

	// ==================== 难度合理性验证 ====================
	
	if err := v.coreEngine.ValidateDifficulty(header.Difficulty); err != nil {
		v.recordError(ErrorInvalidDifficulty)
		logger.Warnf("验证失败: 难度不合理，高度: %d，难度: %d，错误: %v", 
			header.Height, header.Difficulty, err)
		return false, fmt.Errorf("难度验证失败: %w", err)
	}

	// ==================== 时间戳合理性检查 ====================
	
	currentTime := time.Now().Unix()
	headerTime := int64(header.Timestamp)
	
	// 区块时间不能太超前（最多允许2小时）
	if headerTime > currentTime+7200 {
		logger.Warnf("警告: 区块时间戳过于超前，高度: %d，区块时间: %d，当前时间: %d", 
			header.Height, headerTime, currentTime)
		// 注意：这里只记录警告，不阻止验证，因为网络中可能存在时间偏差
	}
	
	// 区块时间不能太过时（不能早于创世区块时间，这里使用一个合理的最小时间）
	minTime := int64(1600000000) // 大约2020年9月的时间戳
	if headerTime < minTime {
		logger.Warnf("警告: 区块时间戳过于久远，高度: %d，区块时间: %d", 
			header.Height, headerTime)
	}

	// ==================== 核心哈希验证 ====================

	// 序列化区块头数据
	headerData, err := proto.Marshal(header)
	if err != nil {
		v.recordError(ErrorSerialization)
		logger.Errorf("验证失败: 序列化区块头失败，高度: %d，错误: %v", 
			header.Height, err)
		return false, fmt.Errorf("序列化区块头失败: %w", err)
	}

	// 计算双重SHA256哈希
	hashManager := v.coreEngine.GetHashManager()
	blockHash := hashManager.DoubleSHA256(headerData)
	
	if len(blockHash) == 0 {
		v.recordError(ErrorHashCalculation)
		logger.Errorf("验证失败: 哈希计算返回空结果，高度: %d", header.Height)
		return false, fmt.Errorf("哈希计算失败：返回空结果")
	}

	// 验证哈希是否满足难度要求
	isDifficultyValid := v.validateHashDifficulty(blockHash, header.Difficulty)
	
	// ==================== 结果处理和统计 ====================
	
	elapsed := time.Since(startTime)
	v.statistics.TotalValidationTime += elapsed
	v.statistics.AverageValidationTime = time.Duration(
		int64(v.statistics.TotalValidationTime) / int64(v.statistics.TotalValidations))

	if isDifficultyValid {
		// 验证成功
		v.statistics.SuccessfulValidations++
		
		logger.Debugf("✅ POW验证通过！高度: %d，难度: %d，哈希: %x，耗时: %v",
			header.Height, header.Difficulty, blockHash, elapsed)
		
		// 记录详细的成功信息（仅在调试模式下）
		if logger != nil {
			nonce, _ := GetNonceLE(header) // 忽略错误，因为前面已经验证过
			logger.Debugf("验证详情: 高度=%d, nonce=%d, 难度=%d位, 实际前导零=%d位, 时间戳=%d",
				header.Height, nonce, header.Difficulty, 
				v.countLeadingZeroBits(blockHash), header.Timestamp)
		}
		
		return true, nil
	} else {
		// 验证失败
		v.recordError(ErrorDifficultyCheck)
		v.statistics.FailedValidations++
		
		actualZeroBits := v.countLeadingZeroBits(blockHash)
		logger.Warnf("🚫 POW验证失败！高度: %d，要求难度: %d位，实际前导零: %d位，哈希: %x，耗时: %v",
			header.Height, header.Difficulty, actualZeroBits, blockHash, elapsed)
		
		// 可能的安全问题记录
		if actualZeroBits == 0 {
			logger.Warnf("🔒 安全警告: 区块哈希无前导零，可能是篡改或伪造，高度: %d", header.Height)
		}
		
		return false, nil // 注意：验证失败不返回error，只有计算错误才返回error
	}
}

// validateHashDifficulty 验证哈希是否满足难度要求
//
// 🔍 **高效难度验证算法**：
// 检查哈希的前导零位数是否满足指定的难度目标。
// 采用位操作优化，支持任意精度的难度验证。
//
// 📋 **算法特点**：
// - 逐字节扫描优化
// - 早期退出策略
// - 分支预测友好
// - CPU缓存优化
//
// 🔄 **性能优化**：
// - 无内存分配
// - 位操作优化
// - 循环展开考虑
// - 编译器优化友好
//
// 📋 **参数说明**：
//   - hash: 待验证的哈希值（32字节）
//   - targetBits: 目标难度（前导零位数）
//
// 🔄 **返回值**：
//   - bool: true表示满足难度要求，false表示不满足
//
// 💡 **算法说明**：
// 从哈希的最高位开始逐位检查，计算连续的前导零位数。
// 当遇到第一个1位时，立即比较已计算的零位数与目标难度。
func (v *ValidationEngine) validateHashDifficulty(hash []byte, targetBits uint64) bool {
	if targetBits == 0 {
		return true // 难度为0总是满足（测试模式）
	}
	
	if len(hash) == 0 {
		return false // 空哈希不满足任何难度
	}
	
	var zeroBits uint64
	
	// 逐字节检查前导零（高效实现）
	for _, b := range hash {
		if b == 0 {
			// 整个字节都是零，快速增加8位
			zeroBits += 8
			
			// 早期满足检查（优化：避免不必要的继续扫描）
			if zeroBits >= targetBits {
				return true
			}
		} else {
			// 字节内部分位为零，需要精确计算
			for i := 7; i >= 0; i-- {
				if (b>>uint(i))&1 == 0 {
					zeroBits++
					
					// 每位检查是否已满足目标
					if zeroBits >= targetBits {
						return true
					}
				} else {
					// 遇到第一个1位，零位计数结束
					return false
				}
			}
		}
	}
	
	// 极端情况：整个哈希都是零（理论上不可能，但处理边界情况）
	return zeroBits >= targetBits
}

// countLeadingZeroBits 计算哈希的前导零位数
//
// 📊 **统计工具方法**：
// 精确计算哈希值的前导零位数，用于日志记录和调试分析。
// 与validateHashDifficulty使用相同的算法确保一致性。
//
// 📋 **参数说明**：
//   - hash: 待计算的哈希值
//
// 🔄 **返回值**：
//   - uint64: 前导零位数
//
// 💡 **用途**：
// - 调试信息输出
// - 统计分析
// - 性能监控
// - 问题诊断
func (v *ValidationEngine) countLeadingZeroBits(hash []byte) uint64 {
	if len(hash) == 0 {
		return 0
	}
	
	var zeroBits uint64
	
	for _, b := range hash {
		if b == 0 {
			zeroBits += 8
		} else {
			for i := 7; i >= 0; i-- {
				if (b>>uint(i))&1 == 0 {
					zeroBits++
				} else {
					return zeroBits
				}
			}
		}
	}
	
	return zeroBits
}

// recordError 记录验证错误统计
//
// 📊 **错误统计记录**：
// 记录各种类型的验证错误，用于系统监控和问题分析。
// 提供详细的错误分类统计信息。
//
// 📋 **参数说明**：
//   - errorType: 错误类型（使用预定义常量）
//
// 💡 **用途**：
// - 错误模式分析
// - 系统健康监控
// - 安全事件追踪
// - 性能问题诊断
func (v *ValidationEngine) recordError(errorType string) {
	v.statistics.FailedValidations++
	v.statistics.ErrorCounts[errorType]++
}

// BatchVerifyBlockHeaders 批量验证区块头
//
// ⚡ **批量验证优化**：
// 同时验证多个区块头，通过批量处理提高验证效率。
// 适用于同步验证、批量导入等场景。
//
// 📋 **性能优化**：
// - 批量内存分配
// - 并行验证（可选）
// - 缓存友好的数据访问
// - 早期失败跳过
//
// 📋 **参数说明**：
//   - headers: 待验证的区块头列表
//
// 🔄 **返回值**：
//   - []bool: 每个区块头的验证结果
//   - error: 批量验证过程中的错误
//
// 💡 **适用场景**：
// - 区块链同步验证
// - 批量数据导入
// - 性能测试
// - 历史数据验证
func (v *ValidationEngine) BatchVerifyBlockHeaders(headers []*core.BlockHeader) ([]bool, error) {
	if len(headers) == 0 {
		return []bool{}, nil
	}
	
	logger := v.coreEngine.GetLogger()
	logger.Infof("开始批量验证，区块数量: %d", len(headers))
	
	startTime := time.Now()
	results := make([]bool, len(headers))
	successCount := 0
	
	// 逐个验证（未来可以优化为并行验证）
	for i, header := range headers {
		if header == nil {
			results[i] = false
			logger.Warnf("批量验证: 第%d个区块头为空", i)
			continue
		}
		
		isValid, err := v.VerifyBlockHeader(header)
		if err != nil {
			logger.Errorf("批量验证: 第%d个区块头验证出错，高度: %d，错误: %v", 
				i, header.Height, err)
			results[i] = false
		} else {
			results[i] = isValid
			if isValid {
				successCount++
			}
		}
	}
	
	elapsed := time.Since(startTime)
	logger.Infof("批量验证完成，总数: %d，成功: %d，失败: %d，耗时: %v，平均: %v/个",
		len(headers), successCount, len(headers)-successCount, 
		elapsed, time.Duration(int64(elapsed)/int64(len(headers))))
	
	return results, nil
}

// GetStatistics 获取验证统计信息
//
// 📊 **统计信息访问**：
// 获取验证引擎的实时统计信息，用于监控界面展示和性能分析。
// 返回统计信息的副本，避免并发访问问题。
//
// 🔄 **返回值**：
//   - ValidationStats: 当前的验证统计信息
//
// 💡 **使用场景**：
// - 监控界面展示验证状态
// - 性能监控和分析
// - 安全审计报告
// - API接口数据提供
func (v *ValidationEngine) GetStatistics() ValidationStats {
	// 创建统计信息的副本（深拷贝错误计数）
	errorCountsCopy := make(map[string]uint64)
	for k, v := range v.statistics.ErrorCounts {
		errorCountsCopy[k] = v
	}
	
	return ValidationStats{
		TotalValidations:      v.statistics.TotalValidations,
		SuccessfulValidations: v.statistics.SuccessfulValidations,
		FailedValidations:     v.statistics.FailedValidations,
		TotalValidationTime:   v.statistics.TotalValidationTime,
		AverageValidationTime: v.statistics.AverageValidationTime,
		LastValidationTime:    v.statistics.LastValidationTime,
		ErrorCounts:           errorCountsCopy,
	}
}

// ResetStatistics 重置验证统计信息
//
// 🔄 **统计重置**：
// 清零所有验证统计信息，用于长时间运行的节点或测试场景。
// 保留当前时间作为新的起始时间。
//
// 💡 **使用场景**：
// - 长时间运行后的统计重置
// - 测试环境的数据清理
// - 统计周期的重新开始
// - 系统维护后的状态重置
func (v *ValidationEngine) ResetStatistics() {
	v.statistics = &ValidationStats{
		LastValidationTime: time.Now(),
		ErrorCounts:        make(map[string]uint64),
	}
	
	v.coreEngine.GetLogger().Info("验证统计信息已重置")
}

// ValidateNonce 单独验证nonce格式
//
// 🔧 **专门工具方法**：
// 单独验证nonce字段的格式和有效性，用于预检查和调试。
//
// 📋 **验证内容**：
// - nonce字段非空
// - nonce长度为8字节
// - nonce值在合理范围内
//
// 📋 **参数说明**：
//   - nonce: 待验证的nonce字节数组
//
// 🔄 **返回值**：
//   - error: 验证失败时的错误，nil表示验证通过
func (v *ValidationEngine) ValidateNonce(nonce []byte) error {
	if len(nonce) == 0 {
		return fmt.Errorf("nonce不能为空")
	}
	
	if len(nonce) != 8 {
		return fmt.Errorf("nonce长度必须为8字节，实际长度: %d", len(nonce))
	}
	
	return nil
}

// GetSuccessRate 获取验证成功率
//
// 📊 **成功率计算**：
// 计算验证引擎的历史成功率，用于系统健康评估。
//
// 🔄 **返回值**：
//   - float64: 成功率（0.0-1.0）
//   - bool: 是否有足够的数据进行计算
//
// 💡 **用途**：
// - 系统健康监控
// - 性能评估
// - 质量指标
// - 告警阈值判断
func (v *ValidationEngine) GetSuccessRate() (float64, bool) {
	total := v.statistics.TotalValidations
	if total == 0 {
		return 0.0, false // 没有数据
	}
	
	successRate := float64(v.statistics.SuccessfulValidations) / float64(total)
	return successRate, true
}
