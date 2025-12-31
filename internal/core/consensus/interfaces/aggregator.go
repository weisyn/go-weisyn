// Package interfaces 定义共识模块的内部接口
//
// 🎯 **聚合器内部接口定义**
//
// 本文件定义聚合器模块内部子组件之间的接口，用于实现 PoW + 距离聚合（XOR）共识架构：
// - 每个接口对应一个子目录的业务实现
// - 接口方法仅用于内部子组件间交互
// - 聚合入口通过 AggregatorController 在内部统一编排
//
// 🏗️ **设计原则**：
// - 基于统一聚合架构：候选收集期 → 聚合选择期 → 结果分发期
// - 职责单一：每个接口对应明确的聚合业务阶段
// - 避免重复：直接使用mempool.CandidatePool等公共接口
// - 状态驱动：基于 8 状态的聚合状态机进行流程控制
package interfaces

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	eventintegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	networkintegration "github.com/weisyn/v1/internal/core/consensus/integration/network"
	"github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                           聚合器内部接口定义
// ============================================================================

// AggregatorController 聚合器控制器
//
// 🎯 **职责**：作为聚合器的统一编排入口
//
// 设计说明：
// - 由 controller/ 子目录实现具体业务逻辑
// - 聚合入口由网络层和矿工服务通过此接口触发
// - 不再对外暴露独立的 AggregatorService 公共接口
type AggregatorController interface {
	// ProcessAggregationRound 处理区块提交的聚合轮次
	//
	// 接收一个候选区块，执行完整的聚合器处理流程：
	// 1. 聚合节点选举判断（基于Kademlia距离）
	// 2. 非聚合节点：转发给正确的聚合节点
	// 3. 聚合节点：添加到候选池并触发聚合流程
	// 4. 执行多因子评估、区块选择和结果分发
	//
	// 设计意图：
	// - 统一处理网络和本地的区块提交
	// - 复用现有的聚合节点选举和候选收集逻辑
	// - 作为聚合器的核心业务入口，代替旧的基于高度的处理
	//
	// @param ctx 上下文，支持聚合中断和超时控制
	// @param candidateBlock 候选区块（来自网络或本地矿工）
	// @return error 聚合过程中的错误
	ProcessAggregationRound(ctx context.Context, candidateBlock *block.Block) error

	// StartAggregatorService 启动聚合器服务
	//
	// 初始化聚合器的所有子组件并开始监听区块提交和系统事件。
	// 服务启动后，聚合器将处于待命状态，等待触发聚合流程。
	//
	// @param ctx 上下文，支持启动中断
	// @return error 启动过程中的错误
	StartAggregatorService(ctx context.Context) error

	// StopAggregatorService 停止聚合器服务
	//
	// 优雅关闭聚合器的所有子组件，完成正在进行的聚合操作，
	// 并释放相关资源。
	//
	// @param ctx 上下文，支持停止超时控制
	// @return error 停止过程中的错误
	StopAggregatorService(ctx context.Context) error
}

// ============================================================================
//                        阶段1：候选生产期接口
// ============================================================================

// AggregatorElection 聚合节点选举器
//
// 🎯 **职责**：确定性聚合节点选举和判断逻辑
//
// 核心功能：
// - 基于Hash(height || SEED) + KademliaClosestPeer算法
// - 判断当前节点是否为指定高度的聚合节点
// - 支持每高度重新选举机制
//
// 仅在 aggregator 内部子组件间使用
type AggregatorElection interface {
	// IsAggregatorForHeight 判断当前节点是否为指定高度的聚合节点
	//
	// 确定性算法：
	// 1. routing_key = Hash(height || SEED)，其中SEED = 上一已确定区块哈希
	// 2. 使用KademliaClosestPeer算法计算最近节点
	// 3. 判断最近节点是否为当前节点
	//
	// @param height 区块高度
	// @return bool 是否为聚合节点
	// @return error 选举过程中的错误
	IsAggregatorForHeight(height uint64) (bool, error)

	// GetAggregatorForHeight 获取指定高度的聚合节点ID
	//
	// 用于区块转发时确定目标聚合节点
	//
	// @param height 区块高度
	// @return peer.ID 聚合节点的peer ID
	// @return error 获取过程中的错误
	GetAggregatorForHeight(height uint64) (peer.ID, error)

	// GetAggregatorForHeightWithWaivers 获取指定高度的聚合节点ID（排除弃权节点）
	//
	// V2 新增：支持弃权与重选机制
	// 用于区块转发时确定目标聚合节点，排除已知弃权的节点，避免回环
	//
	// @param height 区块高度
	// @param waivedAggregators 已弃权的聚合器节点ID列表（避免回环）
	// @return peer.ID 聚合节点的peer ID（排除弃权节点后的最近邻）
	// @return error 获取过程中的错误
	GetAggregatorForHeightWithWaivers(height uint64, waivedAggregators []peer.ID) (peer.ID, error)

	// ValidateAggregatorEligibility 验证聚合节点资格
	//
	// 验证节点是否具备成为聚合节点的基本条件
	//
	// @param peerID 待验证的节点ID
	// @return bool 是否符合聚合节点资格
	// @return error 验证过程中的错误
	ValidateAggregatorEligibility(peerID peer.ID) (bool, error)
}

// NetworkProtocolHandler 网络协议处理器
//
// 🎯 **职责**：处理聚合器相关的网络协议消息
//
// 核心功能：
// - 继承UnifiedAggregatorRouter接口（流式协议处理）
// - 继承UnifiedAggregatorSubscribeRouter接口（订阅协议处理）
// - 提供aggregator特有的网络方法
//
// 仅在 aggregator 内部子组件间使用
type NetworkProtocolHandler interface {
	// 继承基础网络接口，避免重复定义方法
	networkintegration.UnifiedAggregatorRouter          // 流式协议处理
	networkintegration.UnifiedAggregatorSubscribeRouter // 订阅协议处理

	// 注意：已移除 ForwardBlockToAggregator 方法
	// 转发逻辑已移动到 ProcessAggregationRound 内部处理
}

// ============================================================================
//                        阶段2：聚合选择期接口
// ============================================================================

// CandidateCollector 候选收集器
//
// 🎯 **职责**：管理候选区块收集窗口（仅窗口管理，不重复实现候选池）
//
// 核心功能：
// - 管理收集窗口的启动和关闭
// - 配置收集窗口持续时间
// - 与mempool.CandidatePool协作获取候选
// - 支持n+1高度验证
//
// 重要：不重复实现候选池，直接使用mempool.CandidatePool
//
// 仅在 aggregator 内部子组件间使用
type CandidateCollector interface {
	// StartCollectionWindow 启动候选收集窗口
	//
	// 收集窗口机制是聚合架构的核心：
	// - 在指定时间窗口内收集多个候选区块
	// - 防止分叉恶化，通过智能选择而非网络竞争
	//
	// @param height 收集的目标高度
	// @param duration 收集窗口持续时间
	// @return error 启动过程中的错误
	StartCollectionWindow(height uint64, duration time.Duration) error

	// CloseCollectionWindow 关闭收集窗口
	//
	// 关闭窗口并从mempool.CandidatePool获取收集到的候选区块
	//
	// @param height 目标高度
	// @return []CandidateBlock 收集到的候选区块列表
	// @return error 关闭过程中的错误
	CloseCollectionWindow(height uint64) ([]types.CandidateBlock, error)

	// IsCollectionActive 检查收集窗口是否活跃
	//
	// @param height 目标高度
	// @return bool 收集窗口是否活跃
	IsCollectionActive(height uint64) bool

	// GetCollectionProgress 获取收集进度
	//
	// @param height 目标高度
	// @return CollectionProgress 收集进度信息
	// @return error 获取过程中的错误
	GetCollectionProgress(height uint64) (*types.CollectionProgress, error)

	// ClearCandidatePool 清空候选区块内存池
	//
	// 在聚合选择完成并分发后调用，清空所有候选区块开始下一轮
	// 这是当前聚合架构的核心机制：选择完成后清空内存池，而非标记已处理
	//
	// @return int 清理的候选区块数量
	// @return error 清理过程中的错误
	ClearCandidatePool() (int, error)
}

// DecisionCalculator 基础验证器
//
// 🎯 **职责**：执行候选区块的基础合法性验证
//
// 核心功能：
// - 基础PoW验证：确保候选区块满足工作量证明要求
// - 格式完整性验证：检查区块和交易的基本格式
// - 验证结果生成：返回基础验证结果
//
// ⚠️ 注意：实际的区块选择由distance_selector模块中的XOR距离算法完成
// 仅在 aggregator 内部子组件间使用
type DecisionCalculator interface {
	// ValidateCandidate 执行基础验证
	//
	// 验证内容：
	// - 基础PoW验证：验证区块是否满足PoW要求
	// - 格式检查：验证区块头和交易格式
	//
	// @param candidate 待验证的候选区块
	// @return *CandidateValidationResult 基础验证结果
	// @return error 验证过程中的错误
	ValidateCandidate(candidate *types.CandidateBlock) (*types.CandidateValidationResult, error)

	// EvaluateAllCandidates 批量基础验证所有候选区块
	//
	// 对候选区块执行基础验证，仅返回通过验证的候选区块列表
	//
	// @param candidates 候选区块列表
	// @return []CandidateBlock 通过验证的候选区块列表
	// @return error 验证过程中的错误
	EvaluateAllCandidates(candidates []types.CandidateBlock) ([]types.CandidateBlock, error)

	// GetEvaluationStatistics 获取验证统计信息
	//
	// 返回验证统计信息
	//
	// @return *EvaluationStats 统计数据
	// @return error 获取过程中的错误
	GetEvaluationStatistics() (*types.EvaluationStats, error)
}

// ============================================================================
//                        阶段3：结果分发期接口
// ============================================================================

// ResultDistributor 结果分发器
//
// 🎯 **职责**：分发聚合选择结果到全网
//
// 核心功能：
// - 构建分发消息和选择证明
// - 执行多路径分发策略
// - 监控共识收敛状态
//
// 仅在 aggregator 内部子组件间使用
type ResultDistributor interface {
	// DistributeSelectedBlock 分发选中的区块
	//
	// 将聚合选择的最优区块分发到全网
	//
	// @param ctx 上下文，支持分发中断
	// @param selected 选中的区块
	// @param proof 距离选择证明
	// @param totalCandidates 总候选区块数量
	// @return error 分发过程中的错误
	DistributeSelectedBlock(ctx context.Context, selected *types.CandidateBlock, proof *types.DistanceSelectionProof, totalCandidates uint32) error

	// BroadcastToNetwork 网络广播
	//
	// 通过优化的分发拓扑进行网络广播
	//
	// @param ctx 上下文，支持广播中断
	// @param message 距离分发消息
	// @return error 广播过程中的错误
	BroadcastToNetwork(ctx context.Context, message *types.DistanceDistributionMessage) error
}

// ============================================================================
//                           通用支撑接口
// ============================================================================

// AggregationState 聚合状态枚举类型别名
//
// 🎯 **状态定义**：聚合器的 8 状态流程控制
//
// 使用 types.AggregationState 作为底层类型
type AggregationState = types.AggregationState

// AggregatorStateManager 聚合器状态管理器
//
// 🎯 **职责**：管理聚合器的状态机转换
//
// 核心功能：
// - 维护 8 状态聚合状态机
// - 验证状态转换的合法性
// - 记录状态转换历史
// - 支持聚合节点"按需激活，分发后结束"的生命周期
//
// 仅在 aggregator 内部子组件间使用
type AggregatorStateManager interface {
	// GetCurrentState 获取当前聚合状态
	//
	// @return AggregationState 当前聚合状态
	GetCurrentState() AggregationState

	// TransitionTo 转换到目标状态
	//
	// 执行状态转换，包含合法性验证和转换逻辑
	//
	// @param newState 目标状态
	// @return error 状态转换错误（如非法转换）
	TransitionTo(newState AggregationState) error

	// EnsureState 确保处于目标状态（幂等操作）
	//
	// 用于错误恢复、状态修复等场景，如果已经是目标状态则直接返回成功
	//
	// @param targetState 目标状态
	// @return error 状态转换错误（如非法路径）
	EnsureState(targetState AggregationState) error

	// EnsureIdle 确保处于 Idle 状态的便捷方法
	//
	// 用于只读模式弃权、停止聚合、链重组恢复等场景
	//
	// @return error 状态转换错误
	EnsureIdle() error

	// IsValidTransition 验证状态转换
	//
	// 检查从当前状态到目标状态的转换是否合法
	//
	// @param from 源状态
	// @param to 目标状态
	// @return bool 转换是否合法
	IsValidTransition(from, to AggregationState) bool

	// GetCurrentHeight 获取当前聚合高度
	//
	// @return uint64 当前正在聚合的区块高度
	GetCurrentHeight() uint64

	// SetCurrentHeight 设置当前聚合高度
	//
	// @param height 聚合高度
	// @return error 设置过程中的错误
	SetCurrentHeight(height uint64) error
}

// ============================================================================
//                           事件处理接口定义
// ============================================================================

// AggregatorEventHandler 聚合器事件处理接口
//
// 🎯 **职责**：处理聚合器关心的系统事件
//
// 核心功能：
// - 处理区块链重组事件，调整聚合器状态
// - 处理网络质量变化事件，优化聚合策略
// - 确保事件处理的统一性和可测试性
//
// 设计说明：
// - 继承 eventintegration.AggregatorEventSubscriber 接口
// - 由 event_handler/ 子目录实现具体业务逻辑
// - 与其他聚合器组件松耦合交互
type AggregatorEventHandler interface {
	eventintegration.AggregatorEventSubscriber // 继承事件订阅接口

	// 注意：不添加额外方法，直接继承integration层定义的所有事件处理方法
	// 这样确保接口的统一性和可测试性
}

// ============================================================================
//                           激励验证接口（内部）
// ============================================================================

// IncentiveValidator 聚合器侧激励验证器接口（内部）
//
// 🎯 **聚合器侧激励验证**
//
// 职责:
//   - 验证Coinbase费用守恒（零增发）
//   - 验证赞助领取交易结构和数量约束
//   - 识别激励区与普通交易区
//
// 调用时机:
//
//	BlockManager.validateBlock() 验证区块时调用
//
// 实现位置:
//
//	internal/core/consensus/aggregator/validator/incentive.go
//
// 注意：这是Consensus内部接口，不对外暴露
type IncentiveValidator interface {
	// ValidateIncentiveTxs 验证区块中的激励交易
	//
	// 在 BlockManager.validateBlock() 中调用。
	//
	// 参数:
	//   ctx: 上下文
	//   block: 待验证的区块
	//   minerAddr: 矿工地址（用于验证Coinbase输出Owner）
	//
	// 返回:
	//   error: 验证错误，nil表示通过
	//
	// 验证内容:
	//   1. Coinbase位于首位
	//   2. Coinbase费用守恒（期望费用 == Coinbase输出）
	//   3. Coinbase所有输出Owner = minerAddr
	//   4. 赞助领取交易结构正确
	//   5. 赞助领取交易数量不超过上限
	//
	// 约束:
	//   - 必须在交易级验证（TxVerifier）之前调用
	//   - 验证失败应拒绝整个区块
	ValidateIncentiveTxs(
		ctx context.Context,
		block *block.Block,
		minerAddr []byte,
	) error
}

// InternalAggregatorService 内部聚合器服务聚合接口
//
// 🎯 **职责**：聚合所有内部接口，提供完整的聚合服务能力（统一 Aggregator + 距离选择）
//
// 设计说明：
// - 聚合所有子组件接口
// - 由 manager.go 实现完整服务
// - 支持统一的依赖注入和测试
// - 通过NetworkProtocolHandler继承网络接口，委托给network_handler实现
//
// 注意：这是内部聚合接口，不对外暴露
// DistanceSelector 距离选择器接口
//
// 🎯 **职责**：基于XOR距离的确定性区块选择
//
// 核心算法：
// Distance(candidate, parent) = XOR(BigInt(candidate.hash), BigInt(parent.hash))
// selected = argmin(Distance(candidate.BlockHash, parent.BlockHash))
//
// 设计说明：
// - 替换复杂的多因子评分系统
// - 提供确定性的区块选择机制
// - 支持选择证明生成和验证
// - 由 distance_selector/ 子目录实现
type DistanceSelector interface {
	// CalculateDistances 计算所有候选区块与父区块的XOR距离
	//
	// 参数：
	// - candidates: 候选区块列表
	// - parentBlockHash: 父区块哈希（距离计算基准）
	//
	// 返回：
	// - []types.DistanceResult: 距离计算结果列表
	// - error: 计算错误
	CalculateDistances(ctx context.Context, candidates []types.CandidateBlock, parentBlockHash []byte) ([]types.DistanceResult, error)

	// SelectClosestBlock 选择距离最近的区块
	//
	// 参数：
	// - distanceResults: 距离计算结果
	//
	// 返回：
	// - *types.CandidateBlock: 选中的区块
	// - error: 选择错误
	SelectClosestBlock(ctx context.Context, distanceResults []types.DistanceResult) (*types.CandidateBlock, error)

	// GenerateDistanceProof 生成距离选择证明
	//
	// 参数：
	// - selected: 选中的区块
	// - allResults: 所有距离计算结果
	// - parentBlockHash: 父区块哈希
	//
	// 返回：
	// - *types.DistanceSelectionProof: 选择证明
	// - error: 证明生成错误
	GenerateDistanceProof(ctx context.Context, selected *types.CandidateBlock, allResults []types.DistanceResult, parentBlockHash []byte) (*types.DistanceSelectionProof, error)

	// VerifyDistanceSelection 验证距离选择的正确性
	//
	// 参数：
	// - selected: 声称选中的区块
	// - proof: 选择证明
	//
	// 返回：
	// - error: 验证错误，nil表示验证通过
	VerifyDistanceSelection(ctx context.Context, selected *types.CandidateBlock, proof *types.DistanceSelectionProof) error

	// GetDistanceStatistics 获取距离选择统计信息
	//
	// 返回：
	// - *types.DistanceStatistics: 统计信息
	GetDistanceStatistics() *types.DistanceStatistics
}

type InternalAggregatorService interface {
	AggregatorController   // 公共接口实现
	AggregatorElection     // 聚合节点选举
	NetworkProtocolHandler // 网络协议处理（包含所有网络协议处理能力）
	AggregatorEventHandler // 事件处理（处理系统事件如重组、网络变化）
	CandidateCollector     // 候选收集
	DecisionCalculator     // 基础验证器
	DistanceSelector       // 距离选择器（核心选择算法）
	ResultDistributor      // 结果分发
	AggregatorStateManager // 状态管理
}
