// Package chain 创世区块管理实现
//
// 🎯 **创世区块处理服务 (Genesis Block Service)**
//
// 本文件实现了创世区块的完整处理流程，包括：
// - 创世状态检查：检查链是否已经初始化
// - 创世区块生成：基于配置生成确定性创世区块
// - 创世区块验证：验证创世区块的有效性
// - 创世状态初始化：初始化链的基础状态
//
// 🏗️ **设计原则**
// - 配置驱动：完全基于genesis.json和blockchain配置
// - 确定性：相同配置在任何节点都产生相同创世区块
// - 原子性：创世初始化过程要么全部成功要么全部失败
// - 幂等性：支持重复执行创世初始化而不产生副作用
package chain

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                              创世区块检查
// ============================================================================

// NeedsGenesisBlock 检查是否需要创建创世区块
//
// 🎯 **创世需求检查核心**
//
// 判断当前链状态是否需要创建创世区块：
// 1. 检查链是否已初始化（ChainInitializedKey）
// 2. 检查是否存在高度为0的区块
// 3. 检查链状态的一致性
//
// 返回：
//
//	bool: true表示需要创建创世区块
//	error: 检查过程中的错误
func (m *Manager) NeedsGenesisBlock(ctx context.Context) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("检查是否需要创建创世区块")
	}

	// 1. 通过repository检查链是否已初始化
	height, hash, err := m.repo.GetHighestBlock(ctx)
	if err != nil {
		// 如果获取失败，可能是空链，需要创世区块
		if m.logger != nil {
			m.logger.Debugf("获取最高区块失败，可能是空链: %v", err)
		}
		return true, nil
	}

	// 2. 如果能够获取到链高度，说明已存在区块
	if height > 0 || (height == 0 && len(hash) > 0) {
		if m.logger != nil {
			m.logger.Debugf("链已存在区块，高度: %d，无需创世区块", height)
		}
		return false, nil
	}

	// 3. 默认需要创建创世区块
	if m.logger != nil {
		m.logger.Debugf("链状态未初始化，需要创建创世区块")
	}
	return true, nil
}

// ============================================================================
//                              创世区块生成
// ============================================================================

// CreateGenesisBlock 创建创世区块
//
// 🎯 **创世区块生成核心**
//
// 完整的创世区块创建流程：
// 1. 验证创世配置的有效性
// 2. 通过交易服务创建创世交易
// 3. 通过区块服务构建创世区块
// 4. 验证创世区块的正确性
//
// 参数：
//
//	ctx: 操作上下文
//	genesisConfig: 创世配置（来自配置文件）
//
// 返回：
//
//	*core.Block: 完整的创世区块
//	error: 创建过程中的错误
func (m *Manager) CreateGenesisBlock(ctx context.Context, genesisConfig *types.GenesisConfig) (*core.Block, error) {
	if m.logger != nil {
		m.logger.Infof("开始创建创世区块...")
	}

	// 1. 验证创世配置
	if err := m.validateGenesisConfig(genesisConfig); err != nil {
		return nil, fmt.Errorf("创世配置验证失败: %w", err)
	}

	// 2. 检查服务依赖是否已注入
	if m.transactionService == nil {
		return nil, fmt.Errorf("交易服务未初始化")
	}
	if m.blockService == nil {
		return nil, fmt.Errorf("区块服务未初始化")
	}

	// 3. 创建创世交易（通过内部transaction服务）
	genesisTransactions, err := m.transactionService.CreateGenesisTransactions(ctx, genesisConfig)
	if err != nil {
		return nil, fmt.Errorf("创建创世交易失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("创世交易创建完成，数量: %d", len(genesisTransactions))
	}

	// 4. 验证创世交易
	valid, err := m.transactionService.ValidateGenesisTransactions(ctx, genesisTransactions)
	if err != nil {
		return nil, fmt.Errorf("验证创世交易失败: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("创世交易验证失败")
	}

	// 5. 构建创世区块（通过内部block服务）
	genesisBlock, err := m.blockService.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	if err != nil {
		return nil, fmt.Errorf("构建创世区块失败: %w", err)
	}

	// 6. 验证创世区块
	valid, err = m.blockService.ValidateGenesisBlock(ctx, genesisBlock)
	if err != nil {
		return nil, fmt.Errorf("验证创世区块失败: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("创世区块验证失败")
	}

	if m.logger != nil {
		m.logger.Infof("✅ 创世区块创建成功，高度: %d, 交易数: %d",
			genesisBlock.Header.Height, len(genesisTransactions))
	}

	return genesisBlock, nil
}

// ============================================================================
//                              创世区块处理
// ============================================================================

// ProcessGenesisBlock 处理创世区块
//
// 🎯 **创世区块处理核心**
//
// 处理创世区块的完整流程：
// 1. 验证创世区块的有效性
// 2. 存储创世区块到数据库
// 3. 初始化链状态和索引
// 4. 触发创世完成事件
//
// 参数：
//
//	ctx: 操作上下文
//	genesisBlock: 创世区块
//
// 返回：
//
//	error: 处理过程中的错误
func (m *Manager) ProcessGenesisBlock(ctx context.Context, genesisBlock *core.Block) error {
	if m.logger != nil {
		m.logger.Infof("开始处理创世区块...")
	}

	// 1. 最终验证创世区块
	if err := m.validateCreatedGenesisBlock(genesisBlock); err != nil {
		return fmt.Errorf("创世区块最终验证失败: %w", err)
	}

	// 2. 存储创世区块（这会触发repository层的创世状态初始化）
	if err := m.repo.StoreBlock(ctx, genesisBlock); err != nil {
		return fmt.Errorf("存储创世区块失败: %w", err)
	}

	// 3. 验证创世后链状态
	if err := m.verifyGenesisState(ctx); err != nil {
		return fmt.Errorf("创世后状态验证失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("✅ 创世区块处理完成")
	}

	return nil
}

// ============================================================================
//                              验证辅助方法
// ============================================================================

// validateGenesisConfig 验证创世配置
func (m *Manager) validateGenesisConfig(config *types.GenesisConfig) error {
	if config == nil {
		return fmt.Errorf("创世配置不能为空")
	}

	if config.ChainID == 0 {
		return fmt.Errorf("链ID不能为0")
	}

	if config.NetworkID == "" {
		return fmt.Errorf("网络ID不能为空")
	}

	if config.Timestamp == 0 {
		return fmt.Errorf("时间戳不能为0")
	}

	// 验证创世账户配置
	if len(config.GenesisAccounts) == 0 {
		if m.logger != nil {
			m.logger.Warnf("创世配置中没有预设账户")
		}
	}

	for i, account := range config.GenesisAccounts {
		if account.PublicKey == "" {
			return fmt.Errorf("第%d个创世账户的公钥不能为空", i)
		}
		if account.InitialBalance == "" || account.InitialBalance == "0" {
			return fmt.Errorf("第%d个创世账户的初始余额不能为空或为0", i)
		}
	}

	return nil
}

// validateCreatedGenesisBlock 验证创建的创世区块
func (m *Manager) validateCreatedGenesisBlock(block *core.Block) error {
	if block == nil {
		return fmt.Errorf("创世区块不能为空")
	}

	if block.Header == nil {
		return fmt.Errorf("创世区块头不能为空")
	}

	if block.Body == nil {
		return fmt.Errorf("创世区块体不能为空")
	}

	// 验证创世区块的特殊属性
	if block.Header.Height != 0 {
		return fmt.Errorf("创世区块高度必须为0，当前为: %d", block.Header.Height)
	}

	// 验证父区块哈希为全零
	if len(block.Header.PreviousHash) != 32 {
		return fmt.Errorf("创世区块父哈希长度必须为32字节，当前为: %d", len(block.Header.PreviousHash))
	}

	for _, b := range block.Header.PreviousHash {
		if b != 0 {
			return fmt.Errorf("创世区块父哈希必须为全零")
		}
	}

	if block.Header.Timestamp == 0 {
		return fmt.Errorf("创世区块时间戳不能为0")
	}

	return nil
}

// verifyGenesisState 验证创世后的链状态
func (m *Manager) verifyGenesisState(ctx context.Context) error {
	// 1. 检查链是否已标记为初始化
	height, hash, err := m.repo.GetHighestBlock(ctx)
	if err != nil {
		return fmt.Errorf("获取最高区块失败: %w", err)
	}

	if height != 0 {
		return fmt.Errorf("创世后链高度应该为0，当前为: %d", height)
	}

	if len(hash) == 0 {
		return fmt.Errorf("创世后链哈希不能为空")
	}

	// 2. 获取链信息验证完整性
	chainInfo, err := m.getChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	if chainInfo.Height != 0 {
		return fmt.Errorf("创世后链信息高度应该为0，当前为: %d", chainInfo.Height)
	}

	if m.logger != nil {
		m.logger.Debugf("创世后链状态验证通过 - 高度: %d, 哈希: %x", height, hash)
	}

	return nil
}

// ============================================================================
//                              公共接口实现
// ============================================================================

// InitializeGenesisIfNeeded 根据需要初始化创世区块
//
// 🎯 **创世区块自动初始化**
//
// 这是对外的主要接口，用于在系统启动时自动检查和创建创世区块：
// 1. 检查是否需要创世区块
// 2. 如果需要，则创建和处理创世区块
// 3. 如果不需要，则跳过处理
//
// 参数：
//
//	ctx: 操作上下文
//	genesisConfig: 创世配置
//
// 返回：
//
//	bool: true表示创建了创世区块，false表示跳过
//	error: 处理过程中的错误
func (m *Manager) InitializeGenesisIfNeeded(ctx context.Context, genesisConfig *types.GenesisConfig) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("检查是否需要初始化创世区块")
	}

	// 1. 检查是否需要创世区块
	needed, err := m.NeedsGenesisBlock(ctx)
	if err != nil {
		return false, fmt.Errorf("检查创世需求失败: %w", err)
	}

	if !needed {
		if m.logger != nil {
			m.logger.Infof("链已初始化，跳过创世区块创建")
		}
		return false, nil
	}

	// 2. 创建创世区块
	genesisBlock, err := m.CreateGenesisBlock(ctx, genesisConfig)
	if err != nil {
		return false, fmt.Errorf("创建创世区块失败: %w", err)
	}

	// 3. 处理创世区块
	if err := m.ProcessGenesisBlock(ctx, genesisBlock); err != nil {
		return false, fmt.Errorf("处理创世区块失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("🎉 创世区块初始化完成")
	}

	return true, nil
}
