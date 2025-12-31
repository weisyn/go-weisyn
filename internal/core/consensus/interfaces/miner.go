// Package interfaces 定义共识模块的内部接口
//
// 🎯 **矿工内部接口定义**
//
// 本文件定义矿工模块内部子组件之间的接口，用于实现模块化架构：
// - 每个接口对应一个子目录的业务实现
// - 接口方法仅用于内部子组件间交互
// - 公共接口通过 MinerController 继承实现
//
// 🏗️ **设计原则**：
// - 薄接口：只定义必要的内部交互方法
// - 避免重复：不重新包装公共接口
// - 职责单一：每个接口对应明确的业务职责
// - 依赖注入：支持测试和模块替换
package interfaces

import (
	"context"

	eventintegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                              内部接口定义
// ============================================================================

// MinerController 公共接口控制器
//
// 🎯 **职责**：继承并实现 consensus.MinerService 公共接口
//
// 设计说明：
// - 直接继承公共接口，不添加额外方法
// - 由 controller/ 子目录实现具体业务逻辑
// - 作为对外服务的统一入口
type MinerController interface {
	StartMining(ctx context.Context, minerAddress []byte) error
	StartMiningOnce(ctx context.Context, minerAddress []byte) error // 🔧 单次挖矿（挖一个区块后自动停止）
	StopMining(ctx context.Context) error
	GetMiningStatus(ctx context.Context) (bool, []byte, error)
}

// MiningOrchestrator 挖矿编排器
//
// 🎯 **职责**：协调整个挖矿流程的执行
//
// 核心功能：
// - 执行一轮完整的挖矿流程
// - 协调候选区块创建和PoW计算
// - 管理区块发送和确认等待
//
// 仅在 miner 内部子组件间使用
type MiningOrchestrator interface {
	// SetMinerAddress 设置矿工地址
	//
	// 🎯 **运行时矿工地址设置**
	//
	// 在挖矿启动时调用，将矿工地址传递给激励收集器。
	//
	// @param minerAddr 矿工地址（20字节）
	// @return error 设置失败
	SetMinerAddress(minerAddr []byte) error

	// CheckMiningGate 检查挖矿门闸（V2）。
	//
	// 语义：
	// - 若不满足“网络法定人数 + 高度一致性 + 链尖前置条件”，必须返回 error（硬门闸）。
	// - 供 StartMining/StartMiningOnce 与每轮 ExecuteMiningRound 复用（双保险）。
	CheckMiningGate(ctx context.Context) error

	// ExecuteMiningRound 执行一轮挖矿
	//
	// 完整流程：
	// 1. 检查高度门闸，防止重复挖矿
	// 2. 创建候选区块模板
	// 3. 执行PoW计算
	// 4. 发送挖矿结果到网络
	// 5. 等待确认或触发同步
	//
	// @param ctx 上下文，支持取消和超时
	// @return error 挖矿过程中的错误
	ExecuteMiningRound(ctx context.Context) error
}

// PoWComputeHandler PoW计算处理器
//
// 🎯 **职责**：管理PoW计算引擎和相关操作
//
// 核心功能：
// - 管理PoW引擎的启动和停止
// - 执行区块头的挖矿计算
// - 验证区块头的PoW有效性
// - 从模板生成完整区块
//
// 仅在 miner 内部子组件间使用
type PoWComputeHandler interface {
	// MineBlockHeader 挖矿区块头
	//
	// 对给定的区块头执行PoW计算，找到满足难度要求的nonce
	//
	// @param ctx 上下文，支持挖矿中断
	// @param header 待挖矿的区块头
	// @return *core.BlockHeader 挖矿成功的区块头（包含有效nonce）
	// @return error 挖矿过程中的错误
	MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error)

	// VerifyBlockHeader 验证区块头PoW
	//
	// 验证区块头的PoW是否满足当前网络难度要求
	//
	// @param header 待验证的区块头
	// @return bool 验证结果，true表示有效
	// @return error 验证过程中的错误
	VerifyBlockHeader(header *core.BlockHeader) (bool, error)

	// ProduceBlockFromTemplate 从模板生成区块
	//
	// 基于候选区块模板，执行完整的区块生成流程
	//
	// @param ctx 上下文，支持生成中断
	// @param candidateBlock 候选区块模板
	// @return interface{} 生成的完整区块
	// @return error 生成过程中的错误
	ProduceBlockFromTemplate(ctx context.Context, candidateBlock interface{}) (interface{}, error)

	// IsRunning 检查PoW引擎是否在运行状态
	//
	// @return bool 是否在运行
	IsRunning() bool

	// StartPoWEngine 启动PoW引擎
	//
	// 配置并启动PoW计算引擎，准备挖矿操作
	//
	// @param ctx 上下文，支持启动中断
	// @param params 挖矿参数配置
	// @return error 启动过程中的错误
	StartPoWEngine(ctx context.Context, params types.MiningParameters) error

	// StopPoWEngine 停止PoW引擎
	//
	// 优雅停止PoW计算引擎，清理相关资源
	//
	// @param ctx 上下文，支持停止超时
	// @return error 停止过程中的错误
	StopPoWEngine(ctx context.Context) error
}

// HeightGateManager 高度门闸管理器
//
// 🎯 **职责**：管理挖矿高度门闸，防止重复挖矿
//
// 核心功能：
// - 记录最后处理的区块高度
// - 防止在同一高度重复挖矿
// - 支持区块链分叉和同步场景
//
// 仅在 miner 内部子组件间使用
type HeightGateManager interface {
	// UpdateLastProcessedHeight 更新最后处理高度
	//
	// 当区块被成功处理（挖出或确认）时调用
	//
	// @param height 最新处理的区块高度
	UpdateLastProcessedHeight(height uint64)

	// GetLastProcessedHeight 获取最后处理高度
	//
	// 用于挖矿前检查，避免重复挖矿
	//
	// @return uint64 最后处理的区块高度
	GetLastProcessedHeight() uint64
}

// MinerInternalState 矿工内部状态枚举类型别名
//
// 🎯 **状态定义**：矿工内部运行状态
//
// 使用 types.MinerState 作为底层类型
type MinerInternalState = types.MinerState

// MinerStateManager 内部状态管理器
//
// 🎯 **职责**：管理矿工内部运行状态
//
// 核心功能：
// - 维护矿工当前运行状态
// - 验证状态转换的合法性
// - 支持状态查询和更新
//
// 仅在 miner 内部子组件间使用
type MinerStateManager interface {
	// GetMinerState 获取当前矿工状态
	//
	// @return MinerInternalState 当前内部状态
	GetMinerState() MinerInternalState

	// SetMinerState 设置矿工状态
	//
	// 更新矿工内部状态，会进行状态转换验证
	//
	// @param state 目标状态
	// @return error 状态设置错误（如非法转换）
	SetMinerState(state MinerInternalState) error

	// ValidateStateTransition 验证状态转换
	//
	// 检查从当前状态到目标状态的转换是否合法
	//
	// @param from 源状态
	// @param to 目标状态
	// @return bool 转换是否合法
	ValidateStateTransition(from, to MinerInternalState) bool
}

// ============================================================================
//                           事件处理接口定义
// ============================================================================

// MinerEventHandler 矿工事件处理接口
//
// 🎯 **职责**：处理矿工关心的系统事件
//
// 核心功能：
// - 处理分叉检测事件，立即暂停挖矿避免冲突
// - 处理分叉处理中事件，维持暂停状态
// - 处理分叉完成事件，根据结果决定恢复挖矿
// - 确保挖矿状态与区块链状态的一致性
//
// 设计说明：
// - 继承 eventintegration.MinerEventSubscriber 接口
// - 由 event_handler/ 子目录实现具体业务逻辑
// - 与矿工状态管理器协调工作，避免冲突挖矿
type MinerEventHandler interface {
	eventintegration.MinerEventSubscriber // 继承事件订阅接口

	// 注意：不添加额外方法，直接继承integration层定义的所有事件处理方法
	// 这样确保接口的统一性和可测试性，同时保持与现有fork_handler的兼容性
}

// ============================================================================
//                           激励收集接口（内部）
// ============================================================================

// IncentiveCollector 矿工侧激励收集器接口（内部）
//
// 🎯 **矿工侧激励收集**
//
// 职责:
//   - 调用 IncentiveTxBuilder 构建激励交易
//   - 返回 [Coinbase, ClaimTxs...] 供区块组装
//
// 调用时机:
//
//	BlockManager.createMiningCandidate() 创建候选区块时调用
//
// 实现位置:
//
//	internal/core/consensus/miner/incentive/collector.go
//
// 注意：这是Consensus内部接口，不对外暴露
type IncentiveCollector interface {
	// SetMinerAddress 运行时设置矿工地址
	//
	// 🎯 **动态矿工地址设置**
	//
	// 在挖矿启动时由 MinerController 调用，设置当前矿工地址。
	// 支持挖矿过程中切换矿工地址。
	//
	// 参数:
	//   minerAddr: 矿工地址（20字节）
	//
	// 返回:
	//   error: 设置失败（地址长度错误等）
	SetMinerAddress(minerAddr []byte) error

	// CollectIncentiveTxs 收集激励交易
	//
	// 在 BlockManager.CreateMiningCandidate() 中调用。
	//
	// 参数:
	//   ctx: 上下文
	//   candidateTxs: 候选交易列表（用于计算手续费）
	//   blockHeight: 当前区块高度
	//
	// 返回:
	//   []*Transaction: [Coinbase, ClaimTx1, ClaimTx2, ...]
	//   error: 收集错误
	//
	// 约束:
	//   - Coinbase必须是第一笔
	//   - 赞助领取交易紧随其后
	//   - 返回的交易已构建完整，无需进一步处理
	CollectIncentiveTxs(
		ctx context.Context,
		candidateTxs []*transaction_pb.Transaction,
		blockHeight uint64,
	) ([]*transaction_pb.Transaction, error)
}

// InternalMinerService 内部服务聚合接口
//
// 🎯 **职责**：聚合所有内部接口，提供完整的矿工服务能力
//
// 设计说明：
// - 聚合所有子组件接口
// - 由 manager.go 实现完整服务
// - 支持统一的依赖注入和测试
//
// 注意：这是内部聚合接口，不对外暴露
type InternalMinerService interface {
	MinerController    // 公共接口实现
	MiningOrchestrator // 挖矿编排
	PoWComputeHandler  // PoW计算
	HeightGateManager  // 高度门闸
	MinerStateManager  // 内部状态管理
	MinerEventHandler  // 事件处理（处理分叉事件，防止冲突挖矿）
}
