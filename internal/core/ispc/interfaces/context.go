package interfaces

import (
	"context"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ExecutionContextManager 执行上下文管理器接口
//
// 负责创建、管理和销毁执行上下文，为每次ISPC执行提供隔离环境
type ExecutionContextManager interface {
	// ==================== 核心上下文管理 ====================

	// CreateContext 创建执行上下文
	// 为每次函数调用创建独立的执行环境
	CreateContext(ctx context.Context, executionID string, callerAddress string) (ExecutionContext, error)

	// DestroyContext 销毁执行上下文
	// 清理执行完成后的上下文资源
	DestroyContext(ctx context.Context, executionID string) error

	// GetContext 获取执行上下文
	// 根据执行ID获取对应的上下文
	GetContext(executionID string) (ExecutionContext, error)
}

// ExecutionContext ISPC执行上下文接口
//
// 为单次执行提供隔离的运行环境，专注交易草稿的动态构建
type ExecutionContext interface {
	// ==================== 基本信息 ====================

	// GetExecutionID 获取执行ID
	GetExecutionID() string

	// GetDraftID 获取交易草稿ID
	//
	// 🎯 **用途**：供宿主函数通过 TransactionDraftService 访问草稿
	//   - 用于委托模式：宿主函数不直接操作 Draft，而是通过 DraftID 委托给 DraftService
	//
	// 📋 **返回**：
	//   - string: 草稿唯一标识符
	//
	// ⚠️ **注意**：ExecutionContext 创建时应关联一个 Draft
	GetDraftID() string

	// ==================== 确定性区块视图（v2.0 新增）====================

	// GetBlockHeight 获取执行时的区块高度（固定快照）
	//
	// 🎯 **用途**：供宿主函数获取确定性的区块高度
	//   - 基于创建时的固定高度，确保执行确定性
	//   - 不会动态查询最新高度
	//
	// 📋 **返回**：
	//   - uint64: 当前区块高度（固定快照）
	GetBlockHeight() uint64

	// GetBlockTimestamp 获取执行时的区块时间戳（固定快照）
	//
	// 🎯 **用途**：供宿主函数获取确定性的时间戳
	//   - 基于创建时的固定时间戳
	//   - 不会动态查询最新时间
	//
	// 📋 **返回**：
	//   - uint64: Unix 时间戳（秒）
	GetBlockTimestamp() uint64

	// GetChainID 获取链标识
	//
	// 🎯 **用途**：供宿主函数识别当前链
	//   - 用于区分不同的区块链网络（主网、测试网等）
	//   - 固定值，不会变化
	//
	// 📋 **返回**：
	//   - []byte: 链 ID
	GetChainID() []byte

	// GetTransactionID 获取当前交易ID
	//
	// 🎯 **用途**：供宿主函数获取交易唯一标识
	//   - 表示当前正在构建的交易的唯一标识
	//   - 用于幂等性检查、事件关联等场景
	//
	// 📋 **返回**：
	//   - []byte: 交易 ID（32 字节哈希）
	GetTransactionID() []byte

	// ==================== 🔧 执行期服务聚合（新增：断环关键）====================

	// 已移除 Services/SetServices 兼容方法，请使用 HostABI()/SetHostABI()

	// ==================== 🔧 引擎无关宿主能力接口（v1.0 新增）====================

	// HostABI 获取引擎无关宿主能力接口
	//
	// 🎯 **设计目的**：
	//   - 统一 WASM/ONNX 等执行引擎的宿主能力接口
	//   - 宿主函数业务语义（链上上下文/状态访问/草稿记录/外部交互）归口到 ISPC
	//   - 执行引擎仅提供绑定层/适配层，不重复实现业务逻辑
	//
	// 📋 **返回**：
	//   - HostABI: 引擎无关宿主能力接口
	//
	// 🔒 **并发安全**：
	//   - 每次执行独立上下文，无跨执行竞争
	//   - 实现方保证接口本身的并发安全
	//
	// ⚠️ **使用约束**：
	//   - 仅在执行期使用，不进入 Provider 依赖图
	//   - 实现在 ISPC Coordinator 装配时注入（通过 SetHostABI）
	//   - WASM 绑定层和 ONNX 适配层从此获取能力，不直持 blockchain/tx 句柄
	//
	// 🎯 **典型使用**：
	//   hostABI := ctx.HostABI()
	//   height, _ := hostABI.GetBlockHeight(ctx)
	//   outputIndex, _ := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, lockingConditions)
	HostABI() HostABI

	// SetHostABI 设置引擎无关宿主能力接口（运行时注入）
	//
	// 🎯 **设计目的**：
	//   - 由 ISPC Coordinator 在创建上下文后注入 HostABI 实例
	//   - 实现运行时依赖注入，避免构造期循环依赖
	//
	// 📋 **参数**：
	//   - hostABI: 引擎无关宿主能力接口实例
	//
	// ⚠️ **使用约束**：
	//   - 仅在运行时由 Coordinator 调用，不暴露给宿主函数
	//   - 必须在执行前调用，否则 HostABI() 返回 nil
	SetHostABI(hostABI HostABI) error

	// GetCallerAddress 获取调用者地址
	//
	// 🎯 **用途**：供宿主函数获取调用者地址（v1.0 新增）
	//   - 用于权限检查、所有权验证
	//
	// 📋 **返回**：
	//   - []byte: 调用者地址（20字节）
	//
	// ⚠️ **注意**：执行上下文初始化时应设置调用者地址
	GetCallerAddress() []byte

	// ==================== 交易草稿管理（核心功能）====================

	// GetTransactionDraft 获取交易草稿
	// 执行过程中构建的交易草稿，供宿主函数动态填充
	GetTransactionDraft() (*TransactionDraft, error)

	// UpdateTransactionDraft 更新交易草稿
	// 宿主函数调用时动态更新交易内容
	UpdateTransactionDraft(draft *TransactionDraft) error

	// ==================== 执行轨迹管理（v2.0 优化）====================

	// RecordHostFunctionCall 记录宿主函数调用
	//
	// 🎯 **用途**：记录执行过程中的状态变化，用于 ZK 证明生成
	//   - 记录函数名、参数、返回值、调用时序
	//   - 不返回错误，确保执行流程不中断
	//
	// 📋 **参数**：
	//   - call: 宿主函数调用记录（包含序号、函数名、参数、返回值、时间戳）
	//
	// ⚠️ **注意**：
	//   - 实现层负责将记录写入执行轨迹
	//   - 调用此方法不应中断执行流程
	RecordHostFunctionCall(call *HostFunctionCall)

	// GetExecutionTrace 获取执行轨迹
	//
	// 🎯 **用途**：获取完整的执行轨迹用于 ZK 证明生成
	//   - 返回所有宿主函数调用的记录
	//   - 用于 ZK 证明生成、审计、调试等场景
	//
	// 📋 **返回**：
	//   - []*HostFunctionCall: 执行轨迹列表
	//   - error: 获取失败时的错误信息
	GetExecutionTrace() ([]*HostFunctionCall, error)

	// RecordTraceRecords 批量记录轨迹记录（异步轨迹记录优化）
	//
	// 🎯 **用途**：供TraceWorker批量写入轨迹记录
	//   - 支持批量写入宿主函数调用、状态变更、执行事件
	//   - 用于异步轨迹记录优化，提升性能
	//
	// 📋 **参数**：
	//   - records: 轨迹记录列表（包含host_function_call、state_change、execution_event）
	//
	// 📋 **返回**：
	//   - error: 写入失败时的错误信息
	//
	// ⚠️ **注意**：
	//   - 此方法由TraceWorker调用，不应由外部直接调用
	//   - 实现层负责线程安全和批量写入优化
	RecordTraceRecords(records []TraceRecord) error

	// ==================== 业务数据管理 ====================

	// SetReturnData 设置业务返回数据
	// 合约通过 set_return_data 宿主函数调用此方法
	SetReturnData(data []byte) error

	// GetReturnData 获取业务返回数据
	// 执行完成后提取返回给调用方的业务数据
	GetReturnData() ([]byte, error)

	// AddEvent 添加事件
	// 合约通过 emit_event 宿主函数调用此方法
	AddEvent(event *Event) error

	// GetEvents 获取所有事件
	// 执行完成后提取所有发射的事件
	GetEvents() ([]*Event, error)

	// ==================== 合约调用参数管理 ====================

	// SetInitParams 设置合约调用参数（init params）
	// 在执行前由 TX 层注入，供合约通过 get_contract_init_params 读取
	SetInitParams(params []byte) error

	// GetInitParams 获取合约调用参数
	// 宿主函数 get_contract_init_params 调用此方法获取参数
	GetInitParams() ([]byte, error)

	// GetContractAddress 获取当前执行的合约地址
	//
	// 🎯 **用途**：供宿主函数获取合约地址（v1.0 新增）
	//   - 用于创建 ContractTokenAsset 时填充 contract_address 字段
	//
	// 📋 **返回**：
	//   - []byte: 合约地址（20字节）
	//
	// ⚠️ **注意**：执行上下文初始化时应设置合约地址
	GetContractAddress() []byte

	// ==================== 资源使用统计（P0 新增）====================

	// GetResourceUsage 获取资源使用统计
	//
	// 🎯 **用途**：供coordinator获取资源使用统计，用于性能分析和问题诊断
	//   - 记录执行时间、内存使用、操作统计等
	//   - 注意：WES不需要Gas计费，这是本地资源配额管理
	//
	// 📋 **返回**：
	//   - *types.ResourceUsage: 资源使用统计（如果未启用则返回nil）
	GetResourceUsage() *types.ResourceUsage

	// FinalizeResourceUsage 完成资源使用统计
	//
	// 🎯 **用途**：在执行结束时调用，完成资源使用统计的计算
	//   - 设置结束时间、计算执行轨迹大小、完成统计
	FinalizeResourceUsage()
}

// StateSnapshotProvider 提供状态快照能力（用于ExecutionProof metadata）
type StateSnapshotProvider interface {
	// SetStateSnapshots 设置执行前/后的状态哈希（32字节）
	SetStateSnapshots(stateBefore []byte, stateAfter []byte)

	// GetStateBefore 返回执行前状态哈希
	GetStateBefore() []byte

	// GetStateAfter 返回执行后状态哈希
	GetStateAfter() []byte
}

// Event 事件结构
type Event struct {
	Type      string                 // 事件类型
	Timestamp int64                  // 事件时间戳
	Data      map[string]interface{} // 事件数据
}

// HostFunctionCall 宿主函数调用记录（v2.0 新增）
//
// 📋 **记录内容**：
//   - 函数名、参数、返回值、调用时序
//   - 用于 ZK 证明生成和执行审计
//
// 🔒 **设计约束**：
//   - 所有字段不可变
//   - 不包含敏感信息
type HostFunctionCall struct {
	// 序号（调用顺序）
	Sequence uint64

	// 函数名（如 "tx_add_input", "utxo_lookup" 等）
	FunctionName string

	// 参数（JSON 编码）
	Parameters map[string]interface{}

	// 返回值（JSON 编码）
	Result map[string]interface{}

	// 调用时间戳（Unix 纳秒）
	Timestamp int64
}

// TraceRecord 轨迹记录（异步轨迹记录优化）
//
// 🎯 **用途**：用于异步轨迹记录优化，支持批量写入
//
// 📋 **记录类型**：
//   - "host_function_call": 宿主函数调用记录
//   - "state_change": 状态变更记录
//   - "execution_event": 执行事件记录
type TraceRecord struct {
	// 记录类型
	RecordType string // "host_function_call", "state_change", "execution_event"

	// 宿主函数调用记录（如果RecordType为"host_function_call"）
	HostFunctionCall *HostFunctionCall

	// 状态变更记录（如果RecordType为"state_change"）
	StateChange *StateChangeRecord

	// 执行事件记录（如果RecordType为"execution_event"）
	ExecutionEvent *ExecutionEventRecord

	// 执行上下文ID（用于关联到对应的ExecutionContext）
	ExecutionID string
}

// StateChangeRecord 状态变更记录
type StateChangeRecord struct {
	Type      string      // 变更类型（utxo_create, utxo_spend, storage_set等）
	Key       string      // 变更键值
	OldValue  interface{} // 旧值
	NewValue  interface{} // 新值
	Timestamp int64       // 变更时间戳（Unix 纳秒）
}

// ExecutionEventRecord 执行事件记录
type ExecutionEventRecord struct {
	EventType string                 // 事件类型（contract_call, host_function_call等）
	Data      map[string]interface{} // 事件数据
	Timestamp int64                  // 事件时间戳（Unix 纳秒）
}

// TransactionDraft 交易草稿结构
//
// 执行过程中动态构建的交易内容，供宿主函数填充
type TransactionDraft struct {
	// 基本信息
	DraftID       string      // 草稿ID
	ExecutionID   string      // 执行ID
	CallerAddress string      // 调用者地址
	CreatedAt     interface{} // 创建时间
	IsSealed      bool        // 是否已封闭

	// 交易对象
	Tx *pb.Transaction // 交易对象（强类型）

	// 动态构建的输出列表
	Outputs []*pb.TxOutput // 交易输出（强类型）

	// 代币生命周期意图（v1.0 新增）
	BurnIntents    []*TokenBurnIntent    // 销毁意图列表
	ApproveIntents []*TokenApproveIntent // 授权意图列表

	// 资源消耗记录
	ResourceUsage interface{}
}

// TokenBurnIntent 代币销毁意图
type TokenBurnIntent struct {
	TokenID   []byte // 代币标识
	Amount    uint64 // 销毁数量
	BurnProof []byte // 销毁证明（可选）
}

// TokenApproveIntent 代币授权意图
type TokenApproveIntent struct {
	TokenID []byte // 代币标识
	Spender []byte // 被授权者地址
	Amount  uint64 // 授权额度
	Expiry  uint64 // 过期时间（Unix秒，0=永久）
}

// TransactionHandle 交易句柄（占位类型）
// 用于在执行上下文中引用交易对象
type TransactionHandle interface{}
