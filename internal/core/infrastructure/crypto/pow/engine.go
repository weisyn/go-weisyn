// Package pow 提供POW（工作量证明）算法的核心基础组件
//
// 🔧 **核心引擎组件 (Core Engine Component)**
//
// 本文件定义POW引擎的核心基础组件，专注于：
// - 基础算法：提供底层的哈希计算和难度判定
// - 配置管理：统一的POW参数配置管理
// - 工具函数：通用的辅助计算方法
// - 接口实现：实现pkg/interfaces中定义的POWEngine接口
//
// 🎯 **职责边界**：
// - 不直接实现挖矿和验证逻辑（委托给专门的组件）
// - 专注于基础设施和通用工具函数
// - 提供统一的配置和日志管理
// - 作为其他POW组件的基础依赖
//
// 🔗 **组件关系**：
// - Engine: 核心引擎，集成挖矿、验证、难度计算组件
// - 被mining.go中的MiningEngine使用
// - 被validation.go中的ValidationEngine使用
// - 被difficulty.go中的DifficultyCalculator使用
package pow

import (
	"context"
	"fmt"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Engine POW核心引擎基础组件
//
// 🔧 **基础组件结构**：
// 提供POW算法的基础设施和通用工具，作为其他专门组件的基础依赖。
// 集成挖矿引擎、验证引擎和难度计算器，对外提供统一的POWEngine接口。
//
// 📝 **字段说明**：
// - hashManager: 哈希计算管理器，提供双重SHA256等哈希算法
// - logger: 日志记录器，用于记录POW操作的详细信息
// - config: POW配置参数，包含难度范围、算法参数等
// - miningEngine: 专门的挖矿引擎组件（委托模式）
// - validationEngine: 专门的验证引擎组件（委托模式）
// - difficultyCalculator: 专门的难度计算组件（委托模式）
//
// 🎯 **设计模式**：
// - 组合模式: 将不同职责的组件组合在一起
// - 委托模式: 将具体的挖矿和验证逻辑委托给专门的组件
// - 门面模式: 对外提供统一的POWEngine接口
type Engine struct {
	// 基础设施组件
	hashManager crypto.HashManager
	logger      log.Logger
	config      *consensusconfig.POWConfig

	// 专门的功能组件（组合模式）
	miningEngine         *MiningEngine
	validationEngine     *ValidationEngine
	difficultyCalculator *DifficultyCalculator
}

// NewEngine 创建POW核心引擎实例
//
// 🚀 **构造函数**：
// 创建并初始化POW核心引擎，集成各个专门的功能组件。
// 采用组合模式将挖矿、验证、难度计算等功能组合在一起。
//
// 📋 **参数说明**：
//   - hashManager: 哈希计算管理器（不能为nil）
//   - logger: 日志记录器（不能为nil）
//   - config: POW配置参数（可以为nil，使用默认配置）
//
// 🔄 **返回值**：
//   - *Engine: 初始化好的POW引擎实例
//   - error: 创建失败时的错误
//
// 🎯 **初始化流程**：
// 1. 验证必要的依赖参数
// 2. 设置默认配置（如果未提供）
// 3. 创建各个专门的功能组件
// 4. 将组件组合成完整的引擎
// 5. 返回可用的引擎实例
func NewEngine(hashManager crypto.HashManager, logger log.Logger, config *consensusconfig.POWConfig) (*Engine, error) {
	if hashManager == nil {
		return nil, fmt.Errorf("哈希管理器不能为空")
	}
	if logger == nil {
		return nil, fmt.Errorf("日志记录器不能为空")
	}

	// 使用默认配置如果没有提供
	if config == nil {
		config = &consensusconfig.POWConfig{
			InitialDifficulty:          1000,
			MinDifficulty:              1,
			MaxDifficulty:              0,    // 0表示无最大限制
			DifficultyWindow:           2016, // 比特币标准
			DifficultyAdjustmentFactor: 4.0,  // 允许4倍调整
			WorkerCount:                1,
			MaxNonce:                   0xFFFFFFFFFFFFFFFF, // uint64最大值
			EnableParallel:             false,
			HashRateWindow:             100,
		}
	}

	// 创建基础引擎实例
	engine := &Engine{
		hashManager: hashManager,
		logger:      logger.With("component", "pow_core_engine"),
		config:      config,
	}

	// 创建各个专门的功能组件
	var err error

	// 创建难度计算器
	engine.difficultyCalculator, err = NewDifficultyCalculator(engine)
	if err != nil {
		return nil, fmt.Errorf("创建难度计算器失败: %w", err)
	}

	// 创建验证引擎
	engine.validationEngine, err = NewValidationEngine(engine)
	if err != nil {
		return nil, fmt.Errorf("创建验证引擎失败: %w", err)
	}

	// 创建挖矿引擎
	engine.miningEngine, err = NewMiningEngine(engine)
	if err != nil {
		return nil, fmt.Errorf("创建挖矿引擎失败: %w", err)
	}

	// 记录初始化完成
	engine.logger.Infof("POW引擎初始化完成，配置: 初始难度=%d, 范围=[%d, %d], 并行=%v",
		config.InitialDifficulty, config.MinDifficulty, config.MaxDifficulty, config.EnableParallel)

	return engine, nil
}

// ==================== POWEngine接口实现（门面模式）====================

// MineBlockHeader 对区块头进行POW挖矿计算
//
// 🎯 **委托实现**：
// 将挖矿请求委托给专门的挖矿引擎组件处理。
// 采用门面模式对外提供统一的接口，内部委托给专门的组件。
//
// 📋 **实现流程**：
// 1. 委托给miningEngine.MineBlockHeader()
// 2. 记录调用信息和结果
// 3. 返回挖矿结果
//
// 💡 **设计优势**：
// - 单一职责：挖矿逻辑完全独立在mining.go中
// - 易于测试：可以单独测试挖矿组件
// - 易于扩展：可以轻松替换不同的挖矿算法
// - 职责清晰：核心引擎专注于组件协调
func (e *Engine) MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	e.logger.Debug("开始委托挖矿引擎进行POW挖矿")

	// 委托给专门的挖矿引擎
	result, err := e.miningEngine.MineBlockHeader(ctx, header)
	if err != nil {
		e.logger.Errorf("挖矿失败: %v", err)
		return nil, err
	}

	e.logger.Debugf("挖矿成功，高度: %d，难度: %d", result.Height, result.Difficulty)
	return result, nil
}

// VerifyBlockHeader 验证区块头的POW是否有效
//
// 🎯 **委托实现**：
// 将验证请求委托给专门的验证引擎组件处理。
// 采用门面模式对外提供统一的接口，内部委托给专门的组件。
//
// 📋 **实现流程**：
// 1. 委托给validationEngine.VerifyBlockHeader()
// 2. 记录验证信息和结果
// 3. 返回验证结果
//
// 💡 **设计优势**：
// - 单一职责：验证逻辑完全独立在validation.go中
// - 易于测试：可以单独测试验证组件
// - 易于扩展：可以轻松替换不同的验证算法
// - 性能优化：验证组件可以专门针对性能优化
func (e *Engine) VerifyBlockHeader(header *core.BlockHeader) (bool, error) {
	e.logger.Debug("开始委托验证引擎进行POW验证")

	// 委托给专门的验证引擎
	result, err := e.validationEngine.VerifyBlockHeader(header)
	if err != nil {
		e.logger.Debugf("POW验证出错: %v", err)
		return false, err
	}

	e.logger.Debugf("POW验证完成，结果: %v，高度: %d，难度: %d",
		result, header.Height, header.Difficulty)
	return result, nil
}

// ==================== 基础工具方法（供其他组件使用）====================

// GetHashManager 获取哈希管理器
//
// 🔧 **基础设施访问**：
// 为其他POW组件提供哈希管理器的访问接口。
// 保持封装性的同时允许组件间的必要协作。
//
// 🔄 **返回值**：
//   - crypto.HashManager: 哈希管理器实例
func (e *Engine) GetHashManager() crypto.HashManager {
	return e.hashManager
}

// GetLogger 获取日志记录器
//
// 🔧 **基础设施访问**：
// 为其他POW组件提供统一的日志记录器。
// 确保所有组件使用一致的日志格式和级别。
//
// 🔄 **返回值**：
//   - log.Logger: 日志记录器实例
func (e *Engine) GetLogger() log.Logger {
	return e.logger
}

// GetConfig 获取POW配置
//
// 🔧 **基础设施访问**：
// 为其他POW组件提供配置参数的访问接口。
// 确保所有组件使用一致的配置参数。
//
// 🔄 **返回值**：
//   - *consensusconfig.POWConfig: POW配置实例
func (e *Engine) GetConfig() *consensusconfig.POWConfig {
	return e.config
}

// ValidateDifficulty 验证难度值的合理性
//
// 🔍 **基础工具方法**：
// 提供给其他组件使用的难度值验证工具。
// 确保难度值在配置的合理范围内。
//
// 📋 **验证规则**：
// - 难度不能为零
// - 不能低于最小难度
// - 不能超过最大难度（如果设置）
//
// 📋 **参数说明**：
//   - difficulty: 待验证的难度值
//
// 🔄 **返回值**：
//   - error: 验证失败时的错误，nil表示验证通过
func (e *Engine) ValidateDifficulty(difficulty uint64) error {
	if difficulty == 0 {
		return fmt.Errorf("难度不能为零")
	}

	if difficulty < e.config.MinDifficulty {
		return fmt.Errorf("难度 %d 低于最小值 %d", difficulty, e.config.MinDifficulty)
	}

	if e.config.MaxDifficulty > 0 && difficulty > e.config.MaxDifficulty {
		return fmt.Errorf("难度 %d 超过最大值 %d", difficulty, e.config.MaxDifficulty)
	}

	return nil
}

// SetNonceLE 设置区块头的nonce值（小端序）
//
// 🔧 **基础工具方法**：
// 提供给其他组件使用的nonce设置工具。
// 将uint64类型的nonce值以小端序格式写入区块头。
//
// 📋 **编码格式**：
// - 采用小端序（Little Endian）编码
// - 固定8字节长度
// - 兼容主流区块链标准
//
// 📋 **参数说明**：
//   - header: 目标区块头（会被修改）
//   - nonce: nonce值
func SetNonceLE(header *core.BlockHeader, nonce uint64) {
	if header.Nonce == nil || len(header.Nonce) != 8 {
		header.Nonce = make([]byte, 8)
	}

	// 小端序编码
	for i := 0; i < 8; i++ {
		header.Nonce[i] = byte(nonce >> (8 * i))
	}
}

// GetNonceLE 从区块头获取nonce值（小端序）
//
// 🔧 **基础工具方法**：
// 提供给其他组件使用的nonce读取工具。
// 从区块头的小端序nonce字段读取uint64值。
//
// 📋 **解码格式**：
// - 解析小端序（Little Endian）编码
// - 固定8字节长度
// - 兼容主流区块链标准
//
// 📋 **参数说明**：
//   - header: 源区块头
//
// 🔄 **返回值**：
//   - uint64: nonce值
//   - error: 解析错误
func GetNonceLE(header *core.BlockHeader) (uint64, error) {
	if header == nil {
		return 0, fmt.Errorf("区块头不能为空")
	}

	if len(header.Nonce) != 8 {
		return 0, fmt.Errorf("nonce长度必须为8字节，实际长度: %d", len(header.Nonce))
	}

	// 小端序解码
	var nonce uint64
	for i := 0; i < 8; i++ {
		nonce |= uint64(header.Nonce[i]) << (8 * i)
	}

	return nonce, nil
}
