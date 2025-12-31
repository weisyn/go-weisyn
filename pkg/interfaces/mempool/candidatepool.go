// Package mempool 提供WES系统的候选区块池接口定义
//
// 🌊 **候选区块池管理 (Candidate Block Pool Management)**
//
// 本文件定义了WES候选区块池的公共接口，专注于：
// - 候选区块的存储和管理
// - 区块优先级和筛选策略
// - 候选区块的生命周期管理
// - 共识模块的交互接口
//
// 🎯 **设计原则**
// - 高效存储：优化候选区块的存储和检索性能
// - 智能筛选：基于优先级和质量的智能区块筛选
// - 内存控制：严格控制内存使用，防止内存泄漏
// - 并发安全：支持高并发访问和线程安全
package mempool

import (
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

// CandidatePool 定义候选区块池对外接口，供共识组件调用
//
// 🎯 核心职责：候选区块的临时存储和管理
// - 候选区块生命周期管理（存储、检索、移除）
// - 内存管理和容量控制
// - 超时清理和淘汰策略
// - 统计信息提供
//
// ❌ 不包含的职责：
// - 区块验证逻辑（由BlockValidator负责）
// - 共识选择逻辑（由ConsensusEngine负责）
// - 网络传输逻辑（由P2P层负责）
// - PoW验证逻辑（由共识层负责）
//
// 📋 按核心使用场景组织的接口方法：
// 1️⃣ 候选区块提交 - 矿工提交挖出的候选区块
// 2️⃣ 聚合节点收集 - 聚合节点收集并管理候选区块
// 3️⃣ VRF随机选择 - 为VRF选择提供候选区块列表
// 4️⃣ 状态查询监控 - 查询候选区块池状态和统计信息
// 5️⃣ 生命周期管理 - 清理超时区块和内存管理
type CandidatePool interface {

	// ================== 1️⃣ 候选区块提交场景 ==================
	// 使用流程：Miner → ConsensusEngine → CandidatePool
	//
	// 👥 调用方：共识引擎（ConsensusEngine）
	// 🔄 调用时机：矿工挖出候选区块后
	// ⚠️  注意：这些方法只负责存储，不进行区块验证

	// AddCandidate 添加单个候选区块
	// 📝 使用场景：矿工挖出区块后提交，或从其他节点接收候选区块
	// 🔐 前置条件：区块必须已通过基础PoW验证
	// 📤 返回：区块哈希（用于跟踪）和错误信息
	AddCandidate(block *core.Block, fromPeer string) ([]byte, error)

	// AddCandidates 批量添加候选区块
	// 📝 使用场景：网络同步或批量接收候选区块
	// 🔐 前置条件：所有区块必须已通过基础验证
	// 📤 返回：成功添加的区块哈希列表和错误信息
	AddCandidates(blocks []*core.Block, fromPeers []string) ([][]byte, error)

	// ================== 2️⃣ 聚合节点收集场景 ==================
	// 使用流程：AggregatorNode → CandidatePool → VRF选择
	//
	// 👥 调用方：聚合节点（聚合器组件）
	// 🔄 调用时机：聚合节点确定身份后开始收集
	// 🎯 目标：收集特定高度的所有候选区块

	// GetCandidatesForHeight 获取指定高度的所有候选区块
	// 📝 使用场景：聚合节点需要获取特定高度的候选区块进行选择
	// 📊 参数：height-区块高度，timeout-收集超时时间
	// 📤 返回：按接收时间排序的候选区块列表
	GetCandidatesForHeight(height uint64, timeout time.Duration) ([]*types.CandidateBlock, error)

	// GetAllCandidates 获取所有当前候选区块
	// 📝 使用场景：聚合节点获取池中所有候选区块
	// 📤 返回：所有候选区块的列表
	GetAllCandidates() ([]*types.CandidateBlock, error)

	// WaitForCandidates 等待候选区块达到指定数量或超时
	// 📝 使用场景：聚合节点等待足够的候选区块后进行选择
	// 📊 参数：minCount-最少候选区块数，timeout-等待超时时间
	// 📤 返回：收集到的候选区块列表
	WaitForCandidates(minCount int, timeout time.Duration) ([]*types.CandidateBlock, error)

	// ================== 3️⃣ VRF随机选择场景 ==================
	// 使用流程：VRFSelector → CandidatePool → 获取候选列表
	//
	// 👥 调用方：VRF选择器
	// 🔄 调用时机：聚合节点准备执行随机选择时
	// 🎯 目标：提供验证过的候选区块用于随机选择

	// GetCandidateHashes 获取所有候选区块的哈希值
	// 📝 使用场景：VRF选择算法需要所有候选区块的哈希值作为种子
	// 📤 返回：按时间顺序排列的候选区块哈希列表
	GetCandidateHashes() ([][]byte, error)

	// GetCandidateByHash 根据哈希获取候选区块
	// 📝 使用场景：VRF选择完成后，根据选中的哈希获取完整区块
	// 📊 参数：blockHash-区块哈希值
	// 📤 返回：对应的候选区块信息
	GetCandidateByHash(blockHash []byte) (*types.CandidateBlock, error)

	// ================== ❌ 已删除：无意义的状态监控接口 ==================
	//
	// 🚨 **为什么删除状态监控接口？**
	//
	// 在自运行区块链系统中，以下状态查询接口完全没有价值：
	//
	// ❌ **删除的接口及原因**：
	//   • GetPoolStatus() - 获取候选区块池状态
	//     问题：池状态给谁看？IsRunning、StartTime、MemoryUsage等信息有什么用？
	//   • GetCandidateStatus() - 获取特定候选区块状态
	//     问题：候选区块的详细状态给谁看？看了能做什么操作？
	//
	// 🎯 **核心问题**：
	//   1. 这些状态数据的消费者是谁？
	//   2. 基于这些状态会执行什么自动化决策？
	//   3. 在自治系统中，为什么需要向外暴露内部运行状态？
	//
	// 🎯 **自运行系统的正确设计**：
	//   • 候选区块池专注于管理候选区块的生命周期
	//   • 内部异常由池自身处理，不需要外部监控
	//   • 避免暴露无意义的运行时状态
	//
	// ⚠️ **给未来开发者的警告**：
	//   不要重新添加状态查询接口！如果你认为需要，请先回答：
	//   1. 这些状态信息的具体使用场景是什么？
	//   2. 谁会基于这些信息做出什么决策？
	//   3. 为什么不能由内部机制自动处理？

	// ================== 5️⃣ 生命周期管理场景 ==================
	// 使用流程：System → CandidatePool → 维护管理
	//
	// 👥 调用方：系统维护、清理任务
	// 🔄 调用时机：定期维护或手动触发
	// 🎯 目标：保持候选区块池的健康运行

	// ClearCandidates 清空候选区块池
	// 📝 使用场景：新区块确认后清理所有候选区块，或系统重置
	// 📤 返回：清理的候选区块数量和错误信息
	ClearCandidates() (int, error)

	// ClearExpiredCandidates 清理超时的候选区块
	// 📝 使用场景：定期清理超过生存时间的候选区块
	// 📊 参数：maxAge-最大生存时间
	// 📤 返回：清理的候选区块数量
	ClearExpiredCandidates(maxAge time.Duration) (int, error)

	// ClearOutdatedCandidates 清理过时高度的候选区块
	// 📝 使用场景：清理高度不等于currentHeight+1的候选区块
	// 📤 返回：清理的候选区块数量和错误信息
	ClearOutdatedCandidates() (int, error)

	// RemoveCandidate 移除指定的候选区块
	// 📝 使用场景：发现无效候选区块时移除
	// 📊 参数：blockHash-要移除的区块哈希
	// 📤 返回：是否成功移除
	RemoveCandidate(blockHash []byte) error

	// ================== 系统控制接口 ==================
	// 注意：候选区块池由DI容器自动管理生命周期，无需手动Start/Stop
}

// CandidateBlock 结构体已迁移至 pkg/types.CandidateBlock

// ❌ **已删除：PoolStatus - 过度复杂的状态监控结构**
//
// 🚨 **删除原因**：
// PoolStatus包含的各种统计字段在自运行系统中完全没有价值：
//   • IsRunning/StartTime - 系统知道自己在运行，无需外部确认
//   • TotalCandidates/VerifiedCandidates - 这些数量给谁看？有什么用？
//   • MemoryUsage/MemoryUtilization - 内存由系统自动管理，不需要外部监控
//   • AvgAddDuration/AvgVerifyDuration - 平均耗时统计有什么实际意义？
//   • ValidationErrors/DuplicateBlocks - 错误统计给谁看？能基于此做什么？
//
// 🎯 **根本问题**：
// 这种详细的状态结构体是"传统运维思维"的产物，试图向外暴露所有内部状态。
// 在自运行区块链系统中，这些信息的消费者根本不存在。
//
// ❌ **已删除：CandidateStatus - 过度详细的区块状态结构**
//
// 🚨 **删除原因**：
// CandidateStatus试图暴露单个候选区块的所有细节，但这些信息在自治系统中毫无意义：
//   • ValidationErrors/PropagationDelay - 这些详细信息给谁看？
//   • IsSelected/SelectionRank - 选择过程是内部逻辑，无需外部关注
//   • FromPeer/ReceivedAt - 网络细节由网络层处理，业务层不需要知道
//   • 各种Quality指标 - 区块质量评估是内部算法，不应暴露给外部
//
// ⚠️ **架构警告**：
// 这种过度详细的状态暴露违反了"信息隐藏"原则。自运行系统应该：
// 1. 隐藏内部实现细节
// 2. 专注核心业务逻辑
// 3. 避免无意义的状态暴露

// PoolConfig 候选区块池配置接口
//
// 🎯 定义候选区块池的可配置参数
type PoolConfig interface {
	// 获取候选区块池最大容量
	GetMaxCandidates() int

	// 获取候选区块最大生存时间
	GetMaxAge() time.Duration

	// 获取内存使用限制(字节)
	GetMemoryLimit() uint64

	// 获取清理任务执行间隔
	GetCleanupInterval() time.Duration

	// 获取验证超时时间
	GetVerificationTimeout() time.Duration

	// 是否启用优先级排序
	IsPriorityEnabled() bool

	// 获取最大区块大小限制
	GetMaxBlockSize() uint64
}

// 兼容别名（迁至 pkg/types）
type PoolOptions = types.PoolOptions
