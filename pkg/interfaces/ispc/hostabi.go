// Package ispc provides host ABI interfaces for ISPC operations.
package ispc

import (
	"context"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ════════════════════════════════════════════════════════════════════════════════════════════════
// HostABI - ISPC 宿主原语接口（最小集合，无业务语义）
// ════════════════════════════════════════════════════════════════════════════════════════════════
//
// 📋 **接口说明**：
//   - 该接口定义了 WASM/ONNX 智能合约可以调用的宿主原语
//   - 由 internal/core/ispc/hostabi 实现
//   - 严格遵循"无业务语义"原则，仅提供底层原语
//   - 所有调用自动记录到 ExecutionTrace，供 ZK 证明层验证
//
// 🔒 **设计约束**：
//   - ✅ 无业务语义：不提供 Transfer、GetBalance 等高阶方法
//   - ✅ 确定性执行：基于固定区块视图，确保可重放
//   - ✅ 最小权限：仅暴露合约执行必需的能力
//   - ✅ 引擎无关：不依赖 WASM/ONNX 特定实现
//
// 📚 **详细规范**：
//   - 原语规范：_docs/specs/HOST_ABI_MINIMAL_PRIMITIVES.md
//   - 实现文档：internal/core/ispc/hostabi/README.md
//
// ════════════════════════════════════════════════════════════════════════════════════════════════

type HostABI interface {

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 A：确定性区块视图（只读）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// GetBlockHeight 获取当前区块高度
	//
	// 返回值:
	//   - uint64: 当前区块高度（从 ExecutionContext 获取，固定视图）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 基于 ExecutionContext 的固定高度视图，确保执行确定性
	//   - 用于合约逻辑判断、时间锁验证等场景
	GetBlockHeight(ctx context.Context) (uint64, error)

	// GetBlockTimestamp 获取当前区块时间戳
	//
	// 返回值:
	//   - uint64: Unix 时间戳（秒）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 基于 ExecutionContext 的固定时间视图
	//   - 用于时间相关的业务逻辑（时间锁、过期检查等）
	GetBlockTimestamp(ctx context.Context) (uint64, error)

	// GetBlockHash 获取指定高度的区块哈希
	//
	// 参数:
	//   - height: 区块高度
	//
	// 返回值:
	//   - []byte: 区块哈希（32 字节）
	//   - error: 查询失败时的错误信息（高度不存在、查询失败等）
	//
	// 说明:
	//   - 用于验证区块存在性、构建轻客户端证明等
	//   - 仅支持查询已确认的区块
	GetBlockHash(ctx context.Context, height uint64) ([]byte, error)

	// GetChainID 获取链标识
	//
	// 返回值:
	//   - []byte: 链 ID（用于跨链场景、重放攻击防护）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 用于区分不同的区块链网络（主网、测试网等）
	//   - 用于签名验证时的链 ID 校验
	GetChainID(ctx context.Context) ([]byte, error)

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 B：执行上下文（只读）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// GetCaller 获取调用者地址
	//
	// 返回值:
	//   - []byte: 调用者地址（20 字节）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 从 ExecutionContext 获取，表示发起合约调用的账户地址
	//   - 用于权限验证、访问控制等场景
	GetCaller(ctx context.Context) ([]byte, error)

	// GetContractAddress 获取当前合约地址
	//
	// 返回值:
	//   - []byte: 合约地址（20 字节）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 从 ExecutionContext 获取
	//   - 用于合约自身逻辑判断、代理模式等场景
	GetContractAddress(ctx context.Context) ([]byte, error)

	// GetTransactionID 获取当前交易 ID
	//
	// 返回值:
	//   - []byte: 交易 ID（32 字节哈希）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 从 ExecutionContext 获取
	//   - 用于幂等性检查、事件关联等场景
	GetTransactionID(ctx context.Context) ([]byte, error)

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 C：UTXO 查询（只读）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// UTXOLookup 查询指定 UTXO
	//
	// 参数:
	//   - outpoint: UTXO 标识（TxHash + OutputIndex）
	//
	// 返回值:
	//   - *pb.TxOutput: UTXO 输出（nil 表示不存在）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 委托给 repository.UTXOManager
	//   - 基于固定区块高度的 UTXO 快照
	//   - 用于验证 UTXO 存在性和读取锁定条件
	UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error)

	// UTXOExists 检查 UTXO 是否存在
	//
	// 参数:
	//   - outpoint: UTXO 标识
	//
	// 返回值:
	//   - bool: 是否存在
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 轻量级存在性检查，不返回完整数据
	//   - 用于快速验证前置条件
	UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error)

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 D：资源查询（只读）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// ResourceLookup 查询资源元数据
	//
	// 参数:
	//   - contentHash: 资源内容哈希（32 字节）
	//
	// 返回值:
	//   - *pbresource.Resource: 资源元数据（nil 表示不存在）
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 委托给 repository.ResourceManager
	//   - 仅返回元数据，不返回资源内容
	//   - 用于验证资源存在性和读取资源属性
	ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error)

	// ResourceExists 检查资源是否存在
	//
	// 参数:
	//   - contentHash: 资源内容哈希（32 字节）
	//
	// 返回值:
	//   - bool: 是否存在
	//   - error: 查询失败时的错误信息
	//
	// 说明:
	//   - 轻量级存在性检查
	//   - 用于快速验证合约依赖的资源
	ResourceExists(ctx context.Context, contentHash []byte) (bool, error)

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 E：交易草稿构建（写操作）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// TxAddInput 添加交易输入
	//
	// 参数:
	//   - outpoint: 要消费/引用的 UTXO
	//   - isReferenceOnly: 是否仅引用（true=不消费，false=消费）
	//   - unlockingProof: 解锁证明
	//
	// 返回值:
	//   - uint32: 输入索引（在 TransactionDraft.Inputs 中的位置）
	//   - error: 添加失败时的错误信息
	//
	// 说明:
	//   - 委托给 TransactionDraftService
	//   - 直接对应 pb.TxInput 结构
	//   - 不自动验证 UTXO 存在性（由验证层负责）
	TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error)

	// TxAddAssetOutput 添加资产输出
	//
	// 参数:
	//   - owner: 资产所有者地址（20 字节）
	//   - amount: 资产数量（单位：最小单位）
	//   - tokenID: 代币标识（nil=原生币）
	//   - lockingConditions: 锁定条件数组
	//
	// 返回值:
	//   - uint32: 输出索引
	//   - error: 添加失败时的错误信息
	//
	// 说明:
	//   - 委托给 TransactionDraftService
	//   - 直接对应 pb.AssetOutput 结构
	//   - 不自动计算找零（由合约或 SDK 负责）
	TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error)

	// TxAddResourceOutput 添加资源输出
	//
	// 参数:
	//   - contentHash: 资源内容哈希（32 字节）
	//   - category: 资源类别（"wasm", "onnx", "document" 等）
	//   - owner: 资源所有者地址（20 字节）
	//   - lockingConditions: 锁定条件数组
	//   - metadata: 资源元数据（JSON 或 Protobuf 编码）
	//
	// 返回值:
	//   - uint32: 输出索引
	//   - error: 添加失败时的错误信息
	//
	// 说明:
	//   - 委托给 TransactionDraftService
	//   - 直接对应 pb.ResourceOutput 结构
	//   - contentHash 用于内容验证和去重
	TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error)

	// TxAddStateOutput 添加状态输出
	//
	// 参数:
	//   - stateID: 状态标识符（如合约地址+键）
	//   - stateVersion: 状态版本号（递增）
	//   - executionResultHash: 执行结果哈希（32 字节）
	//   - publicInputs: ZK 证明公开输入（字节数组）
	//   - parentStateHash: 父状态哈希（用于状态链追溯）
	//
	// 返回值:
	//   - uint32: 输出索引
	//   - error: 添加失败时的错误信息
	//
	// 说明:
	//   - 委托给 TransactionDraftService
	//   - 直接对应 pb.StateOutput 结构
	//   - 用于 ZK 证明验证、状态链追溯、审计等场景
	TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error)

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 类别 G：执行追踪（辅助）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// EmitEvent 发射链上事件
	//
	// 参数:
	//   - eventType: 事件类型（如 "Transfer", "Mint", "Burn"）
	//   - eventData: 事件数据（字节数组，通常为 JSON 或 Protobuf 编码）
	//
	// 返回值:
	//   - error: 发射失败时的错误信息
	//
	// 说明:
	//   - 事件会进入链上，可被外部监听和查询
	//   - 记录到 TransactionDraft.Events
	//   - 事件数据会占用链上存储，应控制大小
	EmitEvent(ctx context.Context, eventType string, eventData []byte) error

	// LogDebug 记录调试日志（非链上）
	//
	// 参数:
	//   - message: 日志消息
	//
	// 返回值:
	//   - error: 记录失败时的错误信息
	//
	// 说明:
	//   - 日志不进入链上，仅用于调试和监控
	//   - 记录到执行节点的日志系统
	//   - 不占用链上存储，可以任意使用
	LogDebug(ctx context.Context, message string) error
}
