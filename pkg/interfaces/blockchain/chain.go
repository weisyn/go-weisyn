// Package blockchain 提供WES系统的链状态查询接口定义
//
// 🔗 **链状态查询服务 (Chain State Query Service)**
//
// 本文件定义了区块链链状态查询接口，专注于：
// - 基础链状态查询：高度、最佳区块哈希
// - 系统就绪状态检查：系统是否可用
// - 数据新鲜度查询：数据是否同步最新
//
// 🎯 **核心业务场景**
// - 矿工组件：检查链状态决定是否挖矿
// - API服务：验证数据新鲜度提供准确响应
// - 监控系统：健康检查和状态告警
// - 其他组件：获取链的基础状态信息
//
// 🏗️ **设计原则**
// - 业务导向：只提供真实业务场景需要的查询接口
// - 高性能：频繁调用的接口要求毫秒级响应
// - 状态透明：提供清晰的链状态信息
// - 职责聚焦：专注链状态，不涉及网络管理
//
// 详细使用说明请参考：pkg/interfaces/blockchain/README.md
package blockchain

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// ChainService 链状态查询服务接口
//
// 🎯 **专注链状态查询，支持核心业务场景**
//
// 核心职责：
// - 基础链状态查询（高度、哈希、节点模式）
// - 系统就绪状态检查（是否可提供服务）
// - 数据新鲜度验证（数据是否最新同步）
//
// 设计理念：
// - 简单实用：只提供真实业务需要的查询功能
// - 高效响应：所有查询方法要求高性能实现
// - 状态清晰：返回明确的状态信息，不做复杂评分
// - 边界清晰：不涉及网络管理等其他组件职责
type ChainService interface {
	// ==================== 基础链状态查询 ====================

	// GetChainInfo 获取链基础信息
	//
	// 🎯 **获取链的核心状态信息**
	//
	// 返回链的基础状态，包括：
	// - 当前高度和最佳区块哈希
	// - 同步状态（是否与网络同步）
	// - 节点模式（轻节点/全节点）
	//
	// 这是最常用的查询方法，为避免多次调用提供综合信息。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//
	// 返回：
	//   *types.ChainInfo: 链基础信息
	//   error: 查询错误，nil表示成功
	//
	// 使用场景：
	//   • 矿工组件确定当前链状态和挖矿高度
	//   • API服务提供链概况信息
	//   • 其他组件获取链的基础状态
	//
	// 示例：
	//   info, err := chainService.GetChainInfo(ctx)
	//   if err != nil {
	//     return fmt.Errorf("获取链信息失败: %v", err)
	//   }
	//   log.Infof("链状态: 高度=%d, 哈希=%x, 同步=%t",
	//     info.Height, info.BestBlockHash, info.IsSynced)
	GetChainInfo(ctx context.Context) (*types.ChainInfo, error)

	// ==================== 系统状态检查 ====================

	// IsReady 检查系统就绪状态
	//
	// 🎯 **检查区块链系统是否就绪可用**
	//
	// 检查区块链系统是否完全就绪，判断标准：
	// - 所有核心组件已启动并正常运行
	// - 数据库连接正常且数据完整
	// - 同步状态正常，数据是最新的
	//
	// 注意：区块链系统任何组件异常都会导致整个系统不可用，
	// 此方法返回简单的可用/不可用状态，不做复杂的健康评分。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//
	// 返回：
	//   bool: 系统是否完全就绪
	//   error: 检查过程中的错误，nil表示检查成功
	//
	// 使用场景：
	//   • 负载均衡器的服务就绪检查
	//   • API网关的上游服务状态检查
	//   • 运维监控的服务可用性检查
	//
	// 示例：
	//   ready, err := chainService.IsReady(ctx)
	//   if err != nil {
	//     log.Errorf("系统状态检查失败: %v", err)
	//     return
	//   }
	//   if !ready {
	//     log.Warn("系统尚未就绪，请稍后重试")
	//     return
	//   }
	//   log.Info("✅ 系统已就绪")
	IsReady(ctx context.Context) (bool, error)

	// IsDataFresh 检查数据新鲜度
	//
	// 🎯 **检查链数据是否与网络同步最新**
	//
	// 快速检查本地数据是否与网络保持同步，适用于：
	// - API查询前验证数据的时效性
	// - 交易提交前确认链状态最新
	// - 用户操作前的数据新鲜度检查
	//
	// 此方法提供快速的是/否判断，不提供详细的同步进度。
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//
	// 返回：
	//   bool: 数据是否是最新的
	//   error: 检查错误，nil表示成功
	//
	// 使用场景：
	//   • API服务查询前验证数据时效性
	//   • 用户转账前确认余额数据最新
	//   • 矿工挖矿前确认链状态最新
	//
	// 示例：
	//   fresh, err := chainService.IsDataFresh(ctx)
	//   if err != nil {
	//     return fmt.Errorf("检查数据新鲜度失败: %v", err)
	//   }
	//   if !fresh {
	//     return errors.New("数据正在同步中，请稍后重试")
	//   }
	IsDataFresh(ctx context.Context) (bool, error)

	// ==================== 配置查询 ====================

	// GetNodeMode 获取当前节点模式
	//
	// 🎯 **获取节点运行模式**
	//
	// 获取节点的运行模式，节点模式在启动时确定且运行期间不变：
	// - Light: 轻节点模式，仅同步区块头
	// - Full: 全节点模式，同步完整区块数据
	//
	// 参数：
	//   ctx: 上下文对象，用于超时控制
	//
	// 返回：
	//   types.NodeMode: 节点模式
	//   error: 查询错误，nil表示成功
	//
	// 使用场景：
	//   • API服务返回节点信息
	//   • 其他组件根据节点模式调整行为
	//   • 监控系统记录节点配置
	//
	// 示例：
	//   mode, err := chainService.GetNodeMode(ctx)
	//   if err != nil {
	//     return fmt.Errorf("获取节点模式失败: %v", err)
	//   }
	//   log.Infof("节点模式: %s", mode)
	GetNodeMode(ctx context.Context) (types.NodeMode, error)
}

// ============================================================================
//                              设计说明
// ============================================================================

// 🎯 **ChainService设计理念**
//
// **专注链状态查询，职责边界清晰**：
//
// 1. **基础状态查询**：
//    ```go
//    info, err := chainService.GetChainInfo(ctx)  // 综合链状态
//    height, err := chainService.GetCurrentHeight(ctx)  // 当前高度
//    hash, err := chainService.GetBestBlockHash(ctx)  // 最佳区块哈希
//    ```
//    - 支持矿工、共识、API等组件的基础需求
//    - 高性能实现，支持高频调用
//
// 2. **系统状态检查**：
//    ```go
//    ready, err := chainService.IsReady(ctx)  // 系统是否就绪
//    fresh, err := chainService.IsDataFresh(ctx)  // 数据是否最新
//    ```
//    - 支持负载均衡、API验证等场景
//    - 简单的是/否判断，不做复杂评分
//
// 3. **配置查询**：
//    ```go
//    mode, err := chainService.GetNodeMode(ctx)  // 节点运行模式
//    ```
//    - 只读配置查询，支持其他组件适配
//    - 运行期间不可变的节点配置
//
// **这就是链状态查询的全部核心功能！**
//
// ✅ **正确的架构边界**：
// ```
// 业务组件（矿工、API、监控等）
//    ↓ (查询链状态)
// pkg/interfaces/blockchain/chain (公共接口) ← 当前文件
//    ↓ (实现层)
// internal/core/blockchain/domains/state
//    ↓ (数据层)
// pkg/interfaces/repository (数据访问)
// ```
//
// 🚫 **不包含的功能**：
// - 网络管理：网络连接和P2P通信由专门的网络组件负责
// - 同步控制：同步是内部自动服务，不暴露控制接口
// - 复杂统计：不提供非业务的统计分析功能
// - UTXO查询：避免与repository接口重复
// - 生命周期管理：fx框架自动管理组件生命周期
//
// 🎯 **与其他接口的清晰边界**：
// - ChainService：链状态查询（专注链本身的状态）
// - NetworkService：网络连接和P2P通信（由专门的网络组件提供）
// - AccountService：用户账户和资产查询
// - TransactionService：交易处理和查询
// - BlockService：区块操作和查询
// - ResourceService：资源管理和查询
// - RepositoryService：底层数据访问（不重复）
