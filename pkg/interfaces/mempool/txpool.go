// Package mempool 提供WES系统的交易池接口定义
//
// 🌊 **交易池管理 (Transaction Pool Management)**
//
// 本文件定义了WES交易池的公共接口，专注于：
// - 交易的存储和管理
// - 交易的验证和筛选
// - 交易排序和优先级管理
// - 矿工和区块构建模块的交互
//
// 🎯 **设计原则**
// - 纯粹容器：作为纯粹的交易存储容器，不处理复杂业务
// - 高并发：支持高并发的交易提交和检索
// - 内存优化：优化内存使用，支持大量交易存储
// - 快速访问：提供快速的交易查询和管理接口
package mempool

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// TxPool 定义交易池对外接口，供其他组件调用
//
// 🎯 核心职责：纯粹的交易存储容器
// - 交易生命周期管理（存储、检索、移除）
// - 内存管理和容量控制
// - 优先级排序和队列管理
// - 统计信息提供
//
// ❌ 不包含的职责：
// - 交易验证逻辑（由TransactionValidator负责）
// - UTXO状态管理（由UTXOManager负责）
// - 双重花费检测（由验证器在入池前完成）
// - 业务规则验证（由验证器负责）
//
// 📋 按8大使用场景组织的接口方法：
// 1️⃣ 交易提交验证 - 用户/API通过TransactionSubmitter提交交易
// 2️⃣ 挖矿选择交易 - 矿工选择优质交易打包成区块
// 3️⃣ 区块确认回滚 - 区块确认后同步交易池状态
// 4️⃣ 余额查询协调 - UTXO域查询pending交易影响余额
// 5️⃣ 交易状态跟踪 - 用户查询交易处理进度
// 6️⃣ P2P网络传播 - 网络节点传播和存储交易
// 7️⃣ 监控统计 - 系统监控交易池性能指标
// 8️⃣ 系统维护 - 生命周期管理和状态同步
type TxPool interface {

	// ================== 1️⃣ 交易提交验证场景 ==================
	// 使用流程：用户/API → TransactionSubmitter → TransactionValidator → TxPool
	//
	// 👥 调用方：TransactionSubmitter（Transaction域协调器）
	// 🔄 调用时机：交易通过完整验证后
	// ⚠️  注意：这些方法只负责存储，不进行任何业务验证

	// SubmitTx 向交易池提交单个已验证的交易
	// 📝 使用场景：用户通过钱包/API提交交易，TransactionSubmitter验证后存储
	// 🔐 前置条件：交易必须已通过TransactionValidator完整验证
	// 📤 返回：交易哈希（用于跟踪）和错误信息
	SubmitTx(tx *transaction.Transaction) ([]byte, error)

	// SubmitTxs 批量提交已验证的交易
	// 📝 使用场景：区块回滚后重新提交交易，或P2P网络批量接收交易
	// 🔐 前置条件：所有交易必须已通过TransactionValidator验证
	// 📤 返回：成功提交的交易哈希列表和错误信息
	SubmitTxs(txs []*transaction.Transaction) ([][]byte, error)

	// ================== 2️⃣ 挖矿选择交易场景 ==================
	// 使用流程：Miner → TxPool → 选择优质交易 → 构建区块
	//
	// 👥 调用方：挖矿模块（ConsensusEngine）
	// 🔄 调用时机：开始挖新区块时
	// 🎯 目标：选择手续费高、优先级高的交易最大化收益

	// GetTransactionsForMining 获取用于挖矿的优质交易
	// 📝 使用场景：矿工构建区块时选择交易
	// 🎯 策略：按手续费率和优先级排序，优选高价值交易
	// 🎛️ 参数控制：通过内存池配置文件中的挖矿参数控制数量和大小限制
	// 📤 返回：按优先级排序的交易列表，数量和大小由配置决定
	GetTransactionsForMining() ([]*transaction.Transaction, error)

	// MarkTransactionsAsMining 标记交易为挖矿中状态
	// 📝 使用场景：矿工开始打包区块时锁定选中的交易
	// 🔒 作用：防止同一交易被多个矿工重复选择
	// ⏱️  状态：pending → mining
	MarkTransactionsAsMining(txIDs [][]byte) error

	// ConfirmTransactions 确认交易已被成功打包
	// 📝 使用场景：区块被网络接受后移除已确认交易
	// 🗑️  作用：清理交易池，释放内存空间
	// ⏱️  状态：mining → confirmed → removed
	// 📊 参数：txIDs-交易ID列表，blockHeight-确认区块高度
	ConfirmTransactions(txIDs [][]byte, blockHeight uint64) error

	// RejectTransactions 恢复挖矿失败的交易状态
	// 📝 使用场景：区块被拒绝或挖矿失败时恢复交易状态
	// 🔄 作用：将锁定的交易重新放回待处理队列
	// ⏱️  状态：mining → pending
	RejectTransactions(txIDs [][]byte) error

	// MarkTransactionsAsPendingConfirm 标记交易为待确认状态
	// 📝 使用场景：挖出区块后，等待网络确认期间的状态管理
	// 🔒 作用：防止交易被过早删除，保障交易安全
	// ⏱️  状态：mining → pending_confirm
	// 📊 参数：txIDs-交易ID列表，blockHeight-区块高度
	MarkTransactionsAsPendingConfirm(txIDs [][]byte, blockHeight uint64) error

	// ================== 3️⃣ 区块确认回滚场景 ==================
	// 使用流程：ConfirmationManager → TxPool → 同步区块链状态
	//
	// 👥 调用方：ConfirmationManager（确认管理器）
	// 🔄 调用时机：区块确认、分叉处理、链重组时
	// 🎯 目标：保持交易池与区块链状态一致

	// SyncStatus 同步交易池状态与区块链最新状态
	// 📝 使用场景：区块链状态变更（分叉、重组）时同步交易池
	// 🔗 原理：根据新的链头更新交易状态，处理孤块中的交易
	// 📊 参数：height-最新区块高度，stateRoot-状态根哈希
	SyncStatus(height uint64, stateRoot []byte) error

	// UpdateTransactionStatus 更新特定交易的状态
	// 📝 使用场景：确认管理器更新交易确认状态
	// 🔄 时机：区块确认、交易过期、验证失败时
	// 📊 状态转换：pending↔confirmed↔rejected↔expired
	UpdateTransactionStatus(txID []byte, status types.TxStatus) error

	// ================== 4️⃣ 余额查询协调场景 ==================
	// 使用流程：UTXOManager → TxPool → 分析pending交易影响
	//
	// 👥 调用方：UTXO域（余额计算）、Account模块
	// 🔄 调用时机：用户查询余额时
	// 🎯 目标：计算真实可用余额 = 确认余额 - pending支出 + pending接收

	// GetAllPendingTransactions 获取所有待处理交易供余额分析
	// 📝 使用场景：计算用户真实可用余额时分析pending交易影响
	// 💰 用途：确定pending支出和pending接收以计算准确余额
	// 📤 返回：所有pending状态交易，按优先级排序
	GetAllPendingTransactions() ([]*transaction.Transaction, error)

	// ================== 5️⃣ 交易状态跟踪场景 ==================
	// 使用流程：StatusTracker → TxPool → 查询交易处理进度
	//
	// 👥 调用方：用户钱包、区块浏览器、API服务
	// 🔄 调用时机：用户查询交易状态时
	// 🎯 目标：提供交易处理进度的实时反馈

	// GetTx 通过交易ID获取完整交易信息
	// 📝 使用场景：用户查询特定交易详情
	// 🔍 查询：先查交易池，再查区块链
	// 📤 返回：交易详情或nil（不存在）
	GetTx(txID []byte) (*transaction.Transaction, error)

	// GetTxStatus 获取交易当前处理状态
	// 📝 使用场景：用户追踪交易处理进度
	// 📊 状态：pending→mining→confirmed 或 rejected/expired
	// 📤 返回：当前状态枚举值
	GetTxStatus(txID []byte) (types.TxStatus, error)

	// GetTransactionsByStatus 按状态批量查询交易
	// 📝 使用场景：管理界面展示不同状态的交易列表
	// 📊 支持状态：pending、confirmed、rejected、expired
	// 📤 返回：指定状态的所有交易列表
	GetTransactionsByStatus(status types.TxStatus) ([]*transaction.Transaction, error)

	// ================== 6️⃣ P2P网络传播场景 ==================
	// 使用流程：P2P节点 → TransactionSubmitter → TxPool
	//
	// 👥 调用方：P2P网络层（通过TransactionSubmitter）
	// 🔄 调用时机：接收到网络传播的交易时
	// 🎯 目标：验证并存储网络交易，继续传播

	// 注意：P2P场景复用"交易提交验证场景"的SubmitTx/SubmitTxs方法
	// 网络接收的交易同样需要通过TransactionSubmitter进行完整验证后存储

	// ================== ❌ 已删除：无意义的监控统计接口 ==================
	//
	// 🚨 **为什么删除监控统计接口？**
	//
	// 在自运行区块链系统中，以下监控接口没有实际价值：
	//
	// ❌ **删除的接口及原因**：
	//   • GetStats() - 获取交易池统计信息
	//     问题：统计数据给谁看？看了能做什么？系统会自动处理所有情况
	//   • GetPoolCapacity() - 获取容量和内存使用
	//     问题：容量由系统配置自动管理，不需要外部监控和干预
	//
	// 🎯 **自运行系统的设计原则**：
	//   • 组件专注核心业务逻辑，不暴露内部运行状态
	//   • 异常情况由内部自动处理，无需外部监控
	//   • 避免过度工程化的"可观测性"设计
	//
	// ⚠️ **重要提醒**：
	//   如果将来有人想重新添加这些监控接口，请先思考：
	//   1. 这些数据给谁看？
	//   2. 看了这些数据会执行什么操作？
	//   3. 在自运行系统中，外部监控是否真的必要？
	//
	// 📜 **架构决策**：坚持"自治优于监控"的设计哲学

	// ================== 8️⃣ 系统维护场景 ==================
	// 使用流程：系统组件 → TxPool → 生命周期管理
	//
	// 👥 调用方：系统启动器、关闭管理器、维护工具
	// 🔄 调用时机：系统启动、关闭、维护时
	// 🎯 目标：管理交易池生命周期，确保资源正确释放

	// 注意：交易池由DI容器自动管理生命周期，无需手动Close()

	// ================== 🚫 已移除的UTXO耦合方法 ==================
	// 🎉 重构成果：以下UTXO相关方法已在UTXO解耦重构中移除
	// ❌ 已删除：checkUTXOConflicts、updateUTXOReferences、cleanUTXOReferences
	// ❌ 已删除：IsUTXOReferencedInPool、GetMemPoolUTXOState
	// ✅ 解耦后：UTXO验证由TransactionValidator独立处理
	// ✅ 架构：验证与存储完全分离，职责更清晰

	// ================== 🚫 已移除的内部方法 ==================
	// 以下方法在重构中发现属于内部实现，不应暴露给外部：
	// ❌ GetPendingTxs - 已合并到GetTransactionsForMining
	// ❌ RemoveTxs - 已整合到ConfirmTransactions

	// ❌ **已删除：GetDetailedStats() - 又一个无意义的统计接口**
	//
	// 🚨 **删除原因**：
	// 这个方法和之前删除的GetStats()本质相同，都是试图暴露交易池的内部统计：
	//   • "监控系统获取完整的性能指标" - 监控系统是谁？在自运行系统中不存在
	//   • "性能调优、容量规划、问题诊断" - 这些都应该由内部算法自动处理
	//
	// 🎯 **清理不彻底的反思**：
	// 我们之前删除了GetStats()但遗漏了GetDetailedStats()，说明：
	// 1. 同一个错误理念可能以不同的名字出现多次
	// 2. 需要更彻底地搜索和清理相似的监控接口
	// 3. "详细统计"和"普通统计"在自运行系统中都是无意义的
	//
	// ⚠️ **教训**：不要以为加个"Detailed"前缀就有意义了！

	// GetTransactionByID 根据交易ID获取交易
	// 📝 使用场景：查询具体的交易内容
	// 🔍 查找范围：交易池中的所有状态交易（pending/mining）
	// 📤 返回：完整的交易对象，如果不存在则返回nil
	GetTransactionByID(txID []byte) (*transaction.Transaction, error)

	// GetPendingTransactions 获取所有待处理交易
	// 📝 使用场景：遍历候选交易
	// 📊 返回：当前处于pending状态的所有交易
	// ⚠️ 注意：不包括mining状态的交易（已被矿工锁定）
	GetPendingTransactions() ([]*transaction.Transaction, error)
}

// 兼容别名（数据结构迁至 pkg/types）
type TxStatus = types.TxStatus

// 常量别名（向后兼容）
const (
	TxStatusUnknown   = types.TxStatusUnknown
	TxStatusPending   = types.TxStatusPending
	TxStatusIncluded  = types.TxStatusIncluded
	TxStatusConfirmed = types.TxStatusConfirmed
	TxStatusRejected  = types.TxStatusRejected
	TxStatusExpired   = types.TxStatusExpired
)
