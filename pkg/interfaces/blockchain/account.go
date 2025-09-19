// Package blockchain 提供WES系统的账户服务接口定义
//
// 👤 **账户服务接口 (Account Service Interface)**
//
// 本文件定义了面向用户的账户管理完整接口，专注于：
// - 账户余额和资产信息查询（用户友好的账户视角）
// - 地址历史和账户统计
// - 为外部组件提供账户相关能力
//
// 🎯 **核心设计理念**
// 本接口采用**账户抽象设计**，为外部组件（API、钱包、DApp）提供用户友好的账户概念：
// - **业务语义**：提供直观的账户概念，隐藏底层UTXO技术细节
// - **用户视角**：以用户关心的账户、余额、转账为核心概念
// - **高层抽象**：将复杂的UTXO模型抽象为简单的账户操作
// - **性能优化**：针对账户查询场景优化算法和缓存策略
//
// 🔗 **与其他服务的关系**
// - 被 TransactionService 使用：提供余额验证和账户信息
// - 被 ResourceService 使用：验证资源操作的余额要求
// - 被外部组件使用：钱包、API、监控系统等
// - 内部使用底层UTXO存储：但不对外暴露UTXO操作细节
//
// ⚠️ **职责边界**
// - ✅ 专注于账户和余额相关功能
// - ✅ 提供业务语义，隐藏UTXO技术细节
// - ❌ 不直接暴露UTXO集合操作
// - ❌ 不处理基础链状态查询
//
// 详细使用说明和示例请参考：pkg/interfaces/blockchain/README.md
package blockchain

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// AccountService 账户服务接口
//
// 🎯 **核心职责**
// 为外部组件提供用户友好的账户抽象，专注于：
// - 账户余额和资产查询（隐藏UTXO技术细节）
// - 账户历史和统计信息
// - 复杂余额状态管理（锁定、待确认）
//
// 💡 **与UTXO模型的关系**
// 本服务是UTXO模型之上的业务抽象层：
// - **对内**：使用UTXO聚合计算账户余额
// - **对外**：提供账户概念，隐藏UTXO技术细节
// - **转换**：将分散的UTXO转换为统一的账户视图
// - **优化**：缓存账户状态，减少重复UTXO扫描
type AccountService interface {

	// ==================== 账户和余额查询 ====================

	// GetPlatformBalance 获取平台主币余额
	//
	// 获取指定地址的平台主币余额，包括可用余额、锁定余额和待确认余额的完整信息。
	//
	// 参数：
	//   ctx: 上下文对象，用于控制查询超时和取消操作
	//   address: 查询的账户地址，32字节，必须是有效的地址格式
	//
	// 返回：
	//   *types.BalanceInfo: 完整的余额信息对象，包含Available/Locked/Pending/Total
	//   error: 错误信息，成功时为nil
	//
	// 示例：
	//   balance, err := accountService.GetPlatformBalance(ctx, userAddress)
	//   fmt.Printf("可用余额: %.6f\n", float64(balance.Available)/1e9)
	GetPlatformBalance(ctx context.Context, address []byte) (*types.BalanceInfo, error)

	// GetTokenBalance 获取指定代币余额
	//
	// 获取指定地址的特定代币余额信息。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 账户地址
	//   tokenID: 代币ID（32字节哈希）
	//
	// 返回：
	//   *types.BalanceInfo: 代币余额信息
	//   error: 错误信息
	GetTokenBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error)

	// GetAllTokenBalances 获取账户所有代币余额
	//
	// 获取指定地址持有的所有代币余额，包括平台主币和各种ERC20代币的完整持仓信息。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 查询的账户地址
	//
	// 返回：
	//   map[string]*types.BalanceInfo: 代币余额映射
	//     - 键: 代币标识符（""表示主币，其他为代币合约地址）
	//     - 值: 对应代币的完整余额信息
	//   error: 错误信息
	GetAllTokenBalances(ctx context.Context, address []byte) (map[string]*types.BalanceInfo, error)

	// GetLockedBalances 获取锁定余额详情
	//
	// 获取指定地址和代币的锁定余额详细信息，包括每笔锁定的金额、类型、解锁条件等。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 账户地址
	//   tokenID: 代币ID（nil表示平台主币）
	//
	// 返回：
	//   []*types.LockedBalanceEntry: 锁定余额条目列表
	//   error: 错误信息
	GetLockedBalances(ctx context.Context, address []byte, tokenID []byte) ([]*types.LockedBalanceEntry, error)

	// GetPendingBalances 获取待确认余额详情
	//
	// 获取指定地址和代币的待确认余额详细信息，包括每笔待确认交易的金额、确认数、预计时间等。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 账户地址
	//   tokenID: 代币ID（nil表示平台主币）
	//
	// 返回：
	//   []*types.PendingBalanceEntry: 待确认余额条目列表
	//   error: 错误信息
	GetPendingBalances(ctx context.Context, address []byte, tokenID []byte) ([]*types.PendingBalanceEntry, error)

	// GetEffectiveBalance 获取有效可用余额
	//
	// 计算用户的实际可动用余额，公式为：有效可用余额 = 已确认可用余额 - 待确认支出 + 待确认收入
	// 这个接口提供用户真正关心的"我现在能花多少钱"的答案。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 账户地址
	//   tokenID: 代币ID（nil表示平台主币）
	//
	// 返回：
	//   *types.EffectiveBalanceInfo: 有效余额信息，包含详细的计算过程
	//   error: 错误信息
	//
	// 示例：
	//   effective, err := accountService.GetEffectiveBalance(ctx, userAddress, nil)
	//   fmt.Printf("可动用余额: %.6f\n", float64(effective.SpendableAmount)/1e9)
	GetEffectiveBalance(ctx context.Context, address []byte, tokenID []byte) (*types.EffectiveBalanceInfo, error)

	// GetAccountInfo 获取账户信息
	//
	// 获取账户的完整信息，包括总体统计、交易历史统计、权限信息等（不包含详细余额，余额需单独查询）。
	//
	// 参数：
	//   ctx: 上下文对象
	//   address: 账户地址
	//
	// 返回：
	//   *types.AccountInfo: 账户信息
	//   error: 错误信息
	GetAccountInfo(ctx context.Context, address []byte) (*types.AccountInfo, error)
}

// ============================================================================
//                              设计说明
// ============================================================================

// 🎯 **AccountService 核心设计理念**
//
// 1. **账户抽象设计**：
//    - 对外提供直观的账户概念，完全隐藏UTXO技术细节
//    - 余额查询自动聚合底层UTXO，呈现统一的账户余额
//    - 使用"账户、余额、转账"等业务术语，避免"UTXO、输入、输出"等技术术语
//
// 2. **分层职责设计**：
//    - **外部接口层**: 提供账户概念，面向钱包、API、DApp等外部组件
//    - **业务抽象层**: 将UTXO模型抽象为账户模型，提供业务友好的操作语义
//    - **技术实现层**: 内部使用UTXO聚合和管理，但完全不对外暴露
//
// 3. **性能优化策略**：
//    - 账户余额智能缓存，减少重复UTXO扫描开销
//    - 支持批量账户查询，优化多地址场景性能
//    - 增量余额更新，只更新发生变化的账户状态
//    - 并发友好设计，支持高并发账户查询
//
// 🔧 **典型使用方式**：
//
// ```go
// // 基础账户余额查询
// balance, err := accountService.GetPlatformBalance(ctx, userAddress)
//
// // 多代币资产查询
// allBalances, err := accountService.GetAllTokenBalances(ctx, userAddress)
//
// // 账户完整信息
// accountInfo, err := accountService.GetAccountInfo(ctx, userAddress)
// ```
//
// 详细使用示例和最佳实践请参考：pkg/interfaces/blockchain/README.md
