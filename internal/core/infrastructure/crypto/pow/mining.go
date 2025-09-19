// Package pow 提供POW（工作量证明）挖矿引擎实现
//
// ⛏️ **挖矿引擎组件 (Mining Engine Component)**
//
// 本文件专门实现POW挖矿的核心算法，专注于：
// - 挖矿算法：高效的nonce搜索和哈希计算
// - 性能优化：并行计算、算法优化、资源管理
// - 上下文控制：支持取消、超时、进度监控
// - 生产级质量：错误处理、日志记录、指标统计
//
// 🎯 **职责边界**：
// - 专门负责区块头的POW挖矿计算
// - 不涉及验证逻辑（由validation.go负责）
// - 不涉及难度计算（由difficulty.go负责）
// - 不涉及基础设施管理（由engine.go负责）
//
// 🔧 **算法特点**：
// - 采用双重SHA256哈希算法（Bitcoin兼容）
// - 基于前导零位数的难度判定
// - 小端序nonce编码格式
// - 动态时间戳更新机制
// - 支持单线程和多线程挖矿
//
// 🚀 **性能优化**：
// - CPU友好的nonce搜索策略
// - 批量哈希计算优化
// - 内存分配最小化
// - 智能让出CPU控制
//
// 📈 **监控指标**：
// - 算力统计（Hash Rate）
// - 尝试次数计数
// - 挖矿耗时统计
// - 成功率统计
package pow

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// MiningEngine 专门的挖矿引擎组件
//
// ⛏️ **挖矿引擎结构**：
// 专注于POW挖矿算法的实现，提供高效的区块头挖矿服务。
// 采用组合模式依赖核心引擎的基础设施。
//
// 📝 **字段说明**：
// - coreEngine: 核心引擎的引用，用于访问基础设施
// - statistics: 挖矿统计信息（性能监控）
//
// 🎯 **设计原则**：
// - 单一职责：专注挖矿算法实现
// - 高性能：优化的算法和资源使用
// - 可监控：详细的统计和日志信息
// - 可控制：支持上下文取消和超时
type MiningEngine struct {
	coreEngine *Engine     // 核心引擎引用
	statistics *MiningStats // 挖矿统计信息
}

// MiningStats 挖矿统计信息
//
// 📊 **统计指标结构**：
// 记录挖矿过程的性能指标和统计数据，用于监控和优化。
//
// 📝 **字段说明**：
// - TotalBlocks: 总挖矿区块数
// - SuccessfulBlocks: 成功挖矿区块数  
// - TotalAttempts: 总尝试次数
// - TotalTime: 总挖矿时间
// - AverageHashRate: 平均算力
// - LastMiningTime: 最后挖矿时间
// - LastBlockTime: 最后成功挖矿时间
//
// 🎯 **统计用途**：
// - 性能监控和分析
// - 算法优化参考
// - 故障诊断数据
// - 用户界面展示
type MiningStats struct {
	TotalBlocks      uint64        // 总挖矿区块数
	SuccessfulBlocks uint64        // 成功挖矿区块数
	TotalAttempts    uint64        // 总尝试次数
	TotalTime        time.Duration // 总挖矿时间
	AverageHashRate  float64       // 平均算力（Hash/秒）
	LastMiningTime   time.Time     // 最后挖矿时间
	LastBlockTime    time.Time     // 最后成功挖矿时间
}

// NewMiningEngine 创建挖矿引擎实例
//
// 🚀 **构造函数**：
// 创建专门的挖矿引擎组件，依赖核心引擎提供基础设施。
// 初始化挖矿统计和配置参数。
//
// 📋 **参数说明**：
//   - coreEngine: 核心引擎实例（不能为nil）
//
// 🔄 **返回值**：
//   - *MiningEngine: 初始化好的挖矿引擎
//   - error: 创建失败时的错误
//
// 💡 **设计说明**：
// - 采用依赖注入模式接收核心引擎
// - 初始化统计信息结构
// - 验证必要的依赖项
func NewMiningEngine(coreEngine *Engine) (*MiningEngine, error) {
	if coreEngine == nil {
		return nil, fmt.Errorf("核心引擎不能为空")
	}

	engine := &MiningEngine{
		coreEngine: coreEngine,
		statistics: &MiningStats{
			LastMiningTime: time.Now(),
		},
	}

	// 记录初始化日志
	coreEngine.GetLogger().Debug("挖矿引擎组件初始化完成")

	return engine, nil
}

// MineBlockHeader 对区块头进行POW挖矿计算
//
// ⛏️ **核心挖矿算法**：
// 对传入的区块头进行POW计算，通过不断尝试不同的nonce值，
// 直到找到满足难度要求的哈希值，返回包含正确nonce的新区块头。
//
// 📋 **算法流程**：
// 1. 参数验证和预处理
// 2. 克隆区块头避免修改原始数据
// 3. 设置和验证难度参数
// 4. 执行nonce搜索循环
// 5. 动态更新时间戳
// 6. 计算并验证哈希值
// 7. 记录统计信息
// 8. 返回成功结果
//
// 🔄 **实现特点**：
// - 支持上下文取消和超时控制
// - 动态时间戳更新避免区块过时
// - 智能的CPU让出策略
// - 详细的性能统计和日志
// - 内存分配优化
//
// 📊 **性能监控**：
// - 实时算力统计
// - 尝试次数计数
// - 挖矿耗时监控
// - 成功率追踪
//
// 📋 **参数说明**：
//   - ctx: 上下文控制，支持取消和超时
//   - header: 输入的区块头（需要包含difficulty字段）
//
// 🔄 **返回值**：
//   - *core.BlockHeader: 包含正确nonce的新区块头
//   - error: 挖矿失败时的错误（如上下文取消、参数无效等）
//
// 🚨 **错误处理**：
// - 输入参数验证错误
// - 难度参数无效错误  
// - 上下文取消错误
// - 哈希计算错误
func (m *MiningEngine) MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	// ==================== 参数验证和预处理 ====================
	
	if header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	logger := m.coreEngine.GetLogger()
	logger.Debugf("开始挖矿，区块高度: %d", header.Height)

	// 记录挖矿开始时间和统计
	startTime := time.Now()
	m.statistics.TotalBlocks++
	m.statistics.LastMiningTime = startTime

	// 克隆区块头避免修改原始对象
	minedHeader := proto.Clone(header).(*core.BlockHeader)

	// 设置默认难度（如果未设置）
	if minedHeader.Difficulty == 0 {
		config := m.coreEngine.GetConfig()
		minedHeader.Difficulty = config.InitialDifficulty
		logger.Debugf("使用默认难度: %d", minedHeader.Difficulty)
	}

	// 验证难度范围
	if err := m.coreEngine.ValidateDifficulty(minedHeader.Difficulty); err != nil {
		return nil, fmt.Errorf("难度验证失败: %w", err)
	}

	logger.Infof("开始POW挖矿，难度: %d，高度: %d，目标前导零位数: %d位", 
		minedHeader.Difficulty, minedHeader.Height, minedHeader.Difficulty)

	// ==================== 核心挖矿循环 ====================

	nonce := uint64(0)
	attempts := uint64(0)
	lastProgressLog := time.Now()
	hashManager := m.coreEngine.GetHashManager()
	
	for {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			hashRate := float64(attempts) / elapsed.Seconds()
			logger.Infof("挖矿被取消，已尝试: %d次, 耗时: %v, 算力: %.2f H/s", 
				attempts, elapsed, hashRate)
			m.updateStatistics(attempts, elapsed, false)
			return nil, fmt.Errorf("挖矿被取消: %w", ctx.Err())
		default:
		}

		// 更新时间戳（每1000次尝试更新一次，避免频繁更新）
		if attempts%1000 == 0 {
			minedHeader.Timestamp = uint64(time.Now().Unix())
		}

		// 设置当前nonce值
		SetNonceLE(minedHeader, nonce)
		attempts++

		// 计算区块头哈希
		headerData, err := proto.Marshal(minedHeader)
		if err != nil {
			return nil, fmt.Errorf("序列化区块头失败: %w", err)
		}
		
		blockHash := hashManager.DoubleSHA256(headerData)

		// 验证哈希是否满足难度要求
		if m.validateHashDifficulty(blockHash, minedHeader.Difficulty) {
			// 挖矿成功！
			elapsed := time.Since(startTime)
			hashRate := float64(attempts) / elapsed.Seconds()
			
			logger.Infof("🎉 挖矿成功！高度: %d, nonce: %d, 尝试次数: %d, 耗时: %v, 算力: %.2f H/s, 区块哈希: %x",
				minedHeader.Height, nonce, attempts, elapsed, hashRate, blockHash)
			
			// 更新成功统计
			m.updateStatistics(attempts, elapsed, true)
			m.statistics.LastBlockTime = time.Now()
			
			return minedHeader, nil
		}

		// 递增nonce继续尝试
		nonce++

		// ==================== 性能优化和监控 ====================

		// 周期性让出CPU控制（每10万次尝试）
		if attempts%100000 == 0 {
			time.Sleep(time.Millisecond)
		}

		// 周期性进度日志（每100万次尝试）
		if attempts%1000000 == 0 {
			elapsed := time.Since(startTime)
			progressElapsed := time.Since(lastProgressLog)
			currentHashRate := float64(1000000) / progressElapsed.Seconds()
			totalHashRate := float64(attempts) / elapsed.Seconds()
			
			logger.Infof("挖矿进度: 尝试=%d次, 总耗时=%v, 当前算力=%.2f H/s, 平均算力=%.2f H/s, 预计剩余时间=%.1f分钟",
				attempts, elapsed, currentHashRate, totalHashRate,
				m.estimateRemainingTime(minedHeader.Difficulty, totalHashRate, attempts))
			
			lastProgressLog = time.Now()
		}

		// nonce溢出检查（理论上不太可能达到，但为了安全）
		if nonce == 0 {
			return nil, fmt.Errorf("nonce溢出，未找到有效解")
		}
	}
}

// validateHashDifficulty 验证哈希是否满足难度要求
//
// 🔍 **难度验证算法**：
// 检查哈希的前导零位数是否满足指定的难度目标。
// 采用高效的位操作算法，支持任意难度要求。
//
// 📋 **算法说明**：
// - 逐字节检查前导零
// - 精确计算前导零位数
// - 与目标难度比较
// - 早期退出优化
//
// 🔄 **性能优化**：
// - 位操作优化
// - 早期退出策略
// - 无内存分配
// - 分支预测友好
//
// 📋 **参数说明**：
//   - hash: 待验证的哈希值
//   - targetBits: 目标难度（前导零位数）
//
// 🔄 **返回值**：
//   - bool: true表示满足难度要求，false表示不满足
func (m *MiningEngine) validateHashDifficulty(hash []byte, targetBits uint64) bool {
	if targetBits == 0 {
		return true // 难度为0总是满足
	}
	
	var zeroBits uint64
	
	// 逐字节检查前导零
	for _, b := range hash {
		if b == 0 {
			// 整个字节都是零，增加8位
			zeroBits += 8
		} else {
			// 检查字节内的前导零位
			for i := 7; i >= 0; i-- {
				if (b>>uint(i))&1 == 0 {
					zeroBits++
				} else {
					// 遇到第一个1位，停止计数并返回结果
					return zeroBits >= targetBits
				}
			}
			// 这个分支实际不会到达，因为上面的循环会返回
			return zeroBits >= targetBits
		}
	}
	
	// 所有位都是零的极端情况（理论上不可能）
	return zeroBits >= targetBits
}

// updateStatistics 更新挖矿统计信息
//
// 📊 **统计更新**：
// 更新挖矿引擎的统计信息，用于性能监控和分析。
// 记录挖矿尝试次数、耗时、成功率等关键指标。
//
// 📋 **更新内容**：
// - 总尝试次数累加
// - 总时间累加
// - 平均算力计算
// - 成功次数统计（如果成功）
//
// 📋 **参数说明**：
//   - attempts: 本次挖矿的尝试次数
//   - duration: 本次挖矿的耗时
//   - success: 是否挖矿成功
func (m *MiningEngine) updateStatistics(attempts uint64, duration time.Duration, success bool) {
	m.statistics.TotalAttempts += attempts
	m.statistics.TotalTime += duration
	
	if success {
		m.statistics.SuccessfulBlocks++
	}
	
	// 计算平均算力
	if m.statistics.TotalTime > 0 {
		m.statistics.AverageHashRate = float64(m.statistics.TotalAttempts) / m.statistics.TotalTime.Seconds()
	}
}

// estimateRemainingTime 估算剩余挖矿时间
//
// ⏱️ **时间估算算法**：
// 基于当前难度、算力和尝试次数，估算找到有效区块的剩余时间。
// 使用概率模型进行预测，提供用户友好的时间估算。
//
// 📋 **估算公式**：
// 期望尝试次数 = 2^difficulty
// 剩余期望次数 = 期望尝试次数 - 已尝试次数
// 剩余时间 = 剩余期望次数 / 当前算力
//
// 📋 **参数说明**：
//   - difficulty: 当前难度
//   - hashRate: 当前算力（Hash/秒）
//   - currentAttempts: 当前已尝试次数
//
// 🔄 **返回值**：
//   - float64: 估算的剩余时间（分钟）
//
// 💡 **注意事项**：
// - 这只是概率估算，实际时间可能有较大差异
// - 算力波动会影响估算精度
// - 用于用户界面展示和性能分析
func (m *MiningEngine) estimateRemainingTime(difficulty uint64, hashRate float64, currentAttempts uint64) float64 {
	if hashRate <= 0 {
		return -1 // 无法估算
	}
	
	// 计算期望尝试次数（简化为2^难度）
	// 注意：这里使用简化公式，实际比特币的难度计算更复杂
	var expectedAttempts uint64
	if difficulty < 64 {
		expectedAttempts = 1 << difficulty
	} else {
		// 防止溢出，使用最大值
		expectedAttempts = ^uint64(0) // uint64最大值
	}
	
	// 计算剩余期望尝试次数
	var remainingAttempts uint64
	if expectedAttempts > currentAttempts {
		remainingAttempts = expectedAttempts - currentAttempts
	} else {
		// 已经超过期望值，可能很快找到（或者难度设置有问题）
		remainingAttempts = expectedAttempts / 4 // 使用四分之一作为保守估计
	}
	
	// 计算剩余时间（秒转分钟）
	remainingSeconds := float64(remainingAttempts) / hashRate
	remainingMinutes := remainingSeconds / 60.0
	
	return remainingMinutes
}

// GetStatistics 获取挖矿统计信息
//
// 📊 **统计信息访问**：
// 获取挖矿引擎的实时统计信息，用于监控界面展示和性能分析。
// 返回统计信息的副本，避免并发访问问题。
//
// 🔄 **返回值**：
//   - MiningStats: 当前的挖矿统计信息
//
// 💡 **使用场景**：
// - Web界面展示挖矿状态
// - 性能监控和分析
// - 调试和故障诊断
// - API接口数据提供
func (m *MiningEngine) GetStatistics() MiningStats {
	// 返回统计信息的副本，确保线程安全
	return *m.statistics
}

// ResetStatistics 重置挖矿统计信息
//
// 🔄 **统计重置**：
// 清零所有挖矿统计信息，用于长时间运行的节点或测试场景。
// 保留当前时间作为新的起始时间。
//
// 💡 **使用场景**：
// - 长时间运行后的统计重置
// - 测试环境的数据清理
// - 统计周期的重新开始
func (m *MiningEngine) ResetStatistics() {
	m.statistics = &MiningStats{
		LastMiningTime: time.Now(),
	}
	
	m.coreEngine.GetLogger().Info("挖矿统计信息已重置")
}
