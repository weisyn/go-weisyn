// Package account 提供区块链账户管理的实现
//
// 👤 **账户管理器 (Account Manager)**
//
// 本文件实现了账户管理服务，专注于：
// - 余额查询：支持平台主币和自定义代币的多维度余额查询
// - 状态管理：锁定余额、待确认余额的详细状态跟踪
// - 账户信息：统计分析、历史记录等综合账户服务
//
// 🏗️ **设计原则**
// - 实现内部接口：继承公共 AccountService 接口
// - 依赖注入：通过构造函数注入所需依赖
// - UTXO抽象：将底层UTXO模型抽象为用户友好的账户概念
// - 职责单一：专注账户业务逻辑，数据操作委托给repository层
package account

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 内部接口
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"

	// gRPC客户端
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
//                              服务结构定义
// ============================================================================

// Manager 账户管理器
//
// 🎯 **统一账户服务入口**
//
// 负责实现 AccountService 的所有公共接口方法，并将具体实现
// 委托给专门的子文件处理。
//
// 架构特点：
// - 统一入口：所有账户相关操作的统一访问点
// - 依赖注入：通过构造函数注入必需的服务依赖
// - 委托实现：将具体业务逻辑委托给专门的子文件
type Manager struct {
	// 核心依赖
	logger log.Logger                   // 日志服务
	repo   repository.RepositoryManager // 数据仓库管理器

	// 🔥 关键依赖：UTXO数据访问
	utxoManager repository.UTXOManager // UTXO管理器，用于余额计算的数据基础

	// 🔥 关键依赖：待确认交易查询
	txPool mempool.TxPool // 交易池，用于查询待确认交易

	// 🔥 关键依赖：交易哈希计算
	txHashService transaction.TransactionHashServiceClient // 交易哈希服务，用于计算交易ID

	// 内部服务接口
	accountService interfaces.InternalAccountService // 账户内部服务接口
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewManager 创建账户管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//
//	logger: 日志服务
//	repo: 数据仓库管理器
//	utxoManager: UTXO管理器
//	txPool: 交易池
//	txHashService: 交易哈希服务
//
// 返回：
//
//	*Manager: 账户管理器实例
//	error: 创建错误
func NewManager(
	logger log.Logger,
	repo repository.RepositoryManager,
	utxoManager repository.UTXOManager,
	txPool mempool.TxPool,
	txHashService transaction.TransactionHashServiceClient,
) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger 不能为空")
	}
	if repo == nil {
		return nil, fmt.Errorf("repository manager 不能为空")
	}
	if utxoManager == nil {
		return nil, fmt.Errorf("UTXO manager 不能为空")
	}
	if txPool == nil {
		return nil, fmt.Errorf("transaction pool 不能为空")
	}
	if txHashService == nil {
		return nil, fmt.Errorf("transaction hash service 不能为空")
	}

	manager := &Manager{
		logger:        logger,
		repo:          repo,
		utxoManager:   utxoManager,
		txPool:        txPool,
		txHashService: txHashService,
	}

	logger.Infof("✅ 账户管理器初始化完成")

	return manager, nil
}

// ============================================================================
//                              余额查询方法
// ============================================================================

// GetPlatformBalance 获取平台主币余额
//
// 📁 **实现文件**: balance.go
//
// 🎯 **平台主币余额查询**
//
// 查询指定地址的平台主币余额信息，包括可用余额、
// 锁定余额、待确认余额等完整状态。
//
// 实现要点：
// - 聚合地址相关的所有UTXO
// - 计算各种余额状态
// - 提供用户友好的余额视图
func (m *Manager) GetPlatformBalance(ctx context.Context, address []byte) (*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("查询平台主币余额 - address: %x", address)
	}

	// 调用具体实现方法 (balance.go)
	return m.getPlatformBalance(ctx, address)
}

// GetTokenBalance 获取指定代币余额
//
// 📁 **实现文件**: balance.go
//
// 🎯 **特定代币余额查询**
//
// 查询指定地址的特定代币余额信息，支持各种ERC20风格的
// 自定义代币。
//
// 实现要点：
// - 根据tokenID筛选相关UTXO
// - 计算代币专属余额状态
// - 处理代币特有的锁定机制
func (m *Manager) GetTokenBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("查询代币余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 调用具体实现方法 (balance.go)
	return m.getTokenBalance(ctx, address, tokenID)
}

// GetAllTokenBalances 获取账户所有代币余额
//
// 📁 **实现文件**: balance.go
//
// 🎯 **全量代币余额查询**
//
// 获取指定地址持有的所有代币余额，包括平台主币和各种自定义代币的完整持仓信息。
//
// 实现要点：
// - 扫描地址的所有UTXO
// - 按代币类型分组统计
// - 构建完整的资产视图
func (m *Manager) GetAllTokenBalances(ctx context.Context, address []byte) (map[string]*types.BalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("查询所有代币余额 - address: %x", address)
	}

	// 调用具体实现方法 (balance.go)
	return m.getAllTokenBalances(ctx, address)
}

// ============================================================================
//                              状态查询方法
// ============================================================================

// GetLockedBalances 获取锁定余额详情
//
// 📁 **实现文件**: locked.go
//
// 🎯 **锁定余额详细分析**
//
// 获取指定地址和代币的锁定余额详细信息，包括每笔锁定的
// 金额、类型、解锁条件等。
//
// 实现要点：
// - 解析各种锁定条件（时间锁、高度锁、多签锁、合约锁）
// - 计算解锁时间和条件
// - 提供锁定状态的完整视图
func (m *Manager) GetLockedBalances(ctx context.Context, address []byte, tokenID []byte) ([]*types.LockedBalanceEntry, error) {
	if m.logger != nil {
		m.logger.Debugf("查询锁定余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 调用具体实现方法 (locked.go)
	return m.getLockedBalances(ctx, address, tokenID)
}

// GetPendingBalances 获取待确认余额详情
//
// 📁 **实现文件**: pending.go
//
// 🎯 **待确认余额状态跟踪**
//
// 获取指定地址和代币的待确认余额详细信息，包括每笔待确认
// 交易的金额、确认数、预计确认时间等。
//
// 实现要点：
// - 查询内存池中的相关交易
// - 跟踪交易确认进度
// - 评估预计确认时间
func (m *Manager) GetPendingBalances(ctx context.Context, address []byte, tokenID []byte) ([]*types.PendingBalanceEntry, error) {
	if m.logger != nil {
		m.logger.Debugf("查询待确认余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 调用具体实现方法 (pending.go)
	return m.getPendingBalances(ctx, address, tokenID)
}

// GetEffectiveBalance 获取有效可用余额
//
// 📁 **实现文件**: effective.go
//
// 🎯 **有效可用余额计算核心**
//
// 实现审查报告中用户期望的余额实时扣减功能，计算公式：
// 可动用余额 = 已确认可用余额 - 待确认支出 + 待确认收入
//
// 实现要点：
// - 获取已确认的可用余额
// - 计算待确认的支出和收入
// - 提供透明的计算过程和调试信息
// - 识别矿工地址等特殊情况
func (m *Manager) GetEffectiveBalance(ctx context.Context, address []byte, tokenID []byte) (*types.EffectiveBalanceInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("计算有效可用余额 - address: %x, tokenID: %x", address, tokenID)
	}

	// 调用具体实现方法 (effective.go)
	return m.getEffectiveBalance(ctx, address, tokenID)
}

// ============================================================================
//                              账户信息方法
// ============================================================================

// GetAccountInfo 获取账户信息
//
// 📁 **实现文件**: info.go
//
// 🎯 **综合账户信息查询**
//
// 获取账户的完整信息，包括总体统计、交易历史统计、
// 权限信息等（不包含详细余额，余额需单独查询）。
//
// 实现要点：
// - 统计账户历史交易
// - 分析账户活跃度
// - 计算权限和状态信息
func (m *Manager) GetAccountInfo(ctx context.Context, address []byte) (*types.AccountInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("查询账户信息 - address: %x", address)
	}

	// 调用具体实现方法 (info.go)
	return m.getAccountInfo(ctx, address)
}

// ============================================================================
//                              编译时接口检查
// ============================================================================

// 确保 Manager 实现了 AccountService 接口
var _ blockchain.AccountService = (*Manager)(nil)

// 确保 Manager 实现了内部账户服务接口
var _ interfaces.InternalAccountService = (*Manager)(nil)
