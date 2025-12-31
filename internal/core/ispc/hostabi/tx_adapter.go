// Package hostabi 提供 Host ABI 实现
//
// tx_adapter.go: TxAdapter 接口定义（HostABI 与 TX 模块的适配层）
package hostabi

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// TxAdapter HostABI 与 TX 模块的适配层
//
// 🎯 **职责**:
//   - 封装 TX 模块能力，提供链上交易构建原语
//   - 确保确定性执行（固定区块视图、确定性 UTXO 选择）
//   - 管理链上 Draft 生命周期（绑定执行上下文）
//
// ⚠️ **约束**:
//   - Draft 仅内存存储，执行结束自动清理
//   - UTXO 选择基于固定区块快照，确保可重放
//   - 不提供签名能力，返回未签名交易
//
// 💡 **设计说明**：
//   - 这是"薄适配层"：参数编解码 + Draft/Planner 委托 + 验证/错误映射
//   - 不重复实现 TX 逻辑，复用现有 DraftService/Planner/Verifier
type TxAdapter interface {
	// BeginTransaction 开始构建交易
	//
	// 🔄 流程：
	//   1. 创建链上 Draft（内存）
	//   2. 绑定到当前执行上下文
	//   3. 返回 draftHandle（用于后续调用）
	//
	// 参数：
	//   - ctx: 执行上下文
	//   - blockHeight: 当前区块高度（固定区块视图）
	//   - blockTimestamp: 当前区块时间戳
	//
	// 返回：
	//   - draftHandle: Draft 句柄（>0 成功，0 失败）
	//   - error: 错误信息
	BeginTransaction(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error)

	// AddTransfer 添加转账意图
	//
	// 🔄 流程：
	//   1. 根据 draftHandle 获取 Draft
	//   2. 使用确定性 UTXO 选择器选择输入
	//   3. 添加转账输出
	//   4. 计算找零并添加找零输出
	//
	// 参数：
	//   - ctx: 执行上下文
	//   - draftHandle: Draft 句柄
	//   - from: 发送方地址
	//   - to: 接收方地址
	//   - amount: 转账金额
	//   - tokenID: 代币标识（空表示原生币）
	//
	// 返回：
	//   - success: 1 成功，0 失败
	//   - error: 错误信息
	AddTransfer(ctx context.Context, draftHandle int32, from []byte, to []byte, amount string, tokenID []byte) (int32, error)

	// AddCustomInput 添加自定义输入（高级用法）
	//
	// 🎯 用途：合约显式指定输入 UTXO（绕过自动选择）
	AddCustomInput(ctx context.Context, draftHandle int32, outpoint *transaction.OutPoint, isReferenceOnly bool) (int32, error)

	// AddCustomOutput 添加自定义输出（高级用法）
	//
	// 🎯 用途：合约显式构建输出（支持复杂锁定条件）
	AddCustomOutput(ctx context.Context, draftHandle int32, output *transaction.TxOutput) (int32, error)

	// GetDraft 获取Draft对象（高级用法）
	//
	// 🎯 用途：用于修改输出的锁定条件（delegated/threshold模式）
	// ⚠️ 注意：只能在Finalize之前调用，用于修改Draft内容
	GetDraft(ctx context.Context, draftHandle int32) (*types.DraftTx, error)

	// FinalizeTransaction 完成交易构建
	//
	// 🔄 流程：
	//   1. Seal Draft → ComposedTx
	//   2. 调用 Verifier 验证（AuthZ + Conservation + Condition）
	//   3. 验证失败返回错误（触发合约回滚）
	//   4. 验证通过返回未签名交易
	//
	// 参数：
	//   - ctx: 执行上下文
	//   - draftHandle: Draft 句柄
	//
	// 返回：
	//   - tx: 未签名的交易（需外部签名）
	//   - error: 错误信息
	FinalizeTransaction(ctx context.Context, draftHandle int32) (*transaction.Transaction, error)

	// CleanupDraft 清理 Draft（可选，执行结束自动调用）
	CleanupDraft(ctx context.Context, draftHandle int32) error
}

// chainDraftManager 链上 Draft 管理器（内部使用）
//
// 🎯 **职责**:
//   - 管理链上 Draft 的创建、查询、清理
//   - 绑定 Draft 到执行上下文（生命周期一致）
//   - 内存存储，执行结束自动清理
type chainDraftManager interface {
	// CreateDraft 创建链上 Draft
	CreateDraft(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error)

	// GetDraft 获取 Draft
	GetDraft(ctx context.Context, draftHandle int32) (*types.DraftTx, error)

	// RemoveDraft 清理 Draft
	RemoveDraft(ctx context.Context, draftHandle int32) error

	// CleanupAll 清理所有 Draft（执行结束调用）
	CleanupAll(ctx context.Context) error
}
