// Package types 提供交易处理的核心数据结构定义
//
// 🎯 **职责边界**：
// - ✅ **只定义数据结构**：Type-state 类型、Draft 类型、辅助数据类型
// - ❌ **不定义接口**：所有接口定义在 pkg/interfaces/tx
// - ❌ **不实现方法**：所有实现在 internal/core/tx
//
// 📋 **包含的数据结构**：
// 1. Type-state 类型：ComposedTx、ProvenTx、SignedTx、SubmittedTx
// 2. Draft 类型：DraftTx（Builder 的辅助工具）
// 3. 状态类型：TxBroadcastState（交易广播状态）、BroadcastStatus
//
// ⚠️ **核心约束**：
// - 所有字段都是公开的（便于序列化和访问）
// - 不包含任何方法实现（纯数据结构）
// - 不引用任何接口（避免循环依赖）
package types

import (
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 交易状态枚举 ====================

// TxStatus 表示交易在池中的状态（从 pkg/interfaces/mempool 迁移）
type TxStatus int

const (
	TxStatusUnknown   TxStatus = iota // 未知状态
	TxStatusPending                   // 等待处理(已验证但未打包)
	TxStatusIncluded                  // 已包含在池中(等待验证)
	TxStatusConfirmed                 // 已确认(已打包进区块)
	TxStatusRejected                  // 被拒绝(验证失败)
	TxStatusExpired                   // 已过期(超过生存时间)
)

// String 返回TxStatus的字符串表示
func (s TxStatus) String() string {
	switch s {
	case TxStatusUnknown:
		return "Unknown"
	case TxStatusPending:
		return "Pending"
	case TxStatusIncluded:
		return "Included"
	case TxStatusConfirmed:
		return "Confirmed"
	case TxStatusRejected:
		return "Rejected"
	case TxStatusExpired:
		return "Expired"
	default:
		return "Invalid"
	}
}

// ================================================================================================
// 🎯 Part 1: Type-state 数据结构（4 个状态）
// ================================================================================================

// ComposedTx 类型状态1：已组合，未授权
//
// 🎯 **定位**：交易输入输出已完成装配，但尚未添加解锁证明
//
// ✅ **已有内容**：
// - Tx.Inputs: 输入列表（引用已有 UTXO）
// - Tx.Outputs: 输出列表（定义新 UTXO）
// - Tx.Nonce、Tx.CreationTimestamp 等基础字段
//
// ❌ **未有内容**：
// - UnlockingProof: 解锁证明（尚未生成）
// - Signature: 签名（尚未签名）
//
// 📝 **状态转换**：
// ComposedTx → ProvenTx（通过 TxBuilder 的实现添加 proof）
type ComposedTx struct {
	Tx     *transaction.Transaction // 底层交易对象
	Sealed bool                     // 是否已封闭（防止直接修改）
}

// ProvenTx 类型状态2：已授权，未签名
//
// 🎯 **定位**：交易已添加解锁证明，但尚未签名
//
// ✅ **已有内容**：
// - Tx.Inputs: 输入列表（含 UnlockingProof）
// - Tx.Outputs: 输出列表
// - Tx 的所有基础字段
//
// ❌ **未有内容**：
// - Signature: 签名（尚未签名）
//
// 📝 **状态转换**：
// ProvenTx → SignedTx（通过 Signer 签名）
type ProvenTx struct {
	Tx     *transaction.Transaction // 底层交易对象（已添加 proof）
	Sealed bool                     // 是否已封闭
}

// SignedTx 类型状态3：已签名，可提交
//
// 🎯 **定位**：交易已完成签名，可以提交到网络
//
// ✅ **已有内容**：
// - Tx.Inputs: 输入列表（含 UnlockingProof）
// - Tx.Outputs: 输出列表
// - Signature: 签名数据（已签名）
// - Tx 的所有字段完整
//
// 📝 **状态转换**：
// SignedTx → SubmittedTx（通过 TxProcessor 提交）
type SignedTx struct {
	Tx *transaction.Transaction // 底层交易对象（已签名）
}

// SubmittedTx 类型状态4：已提交
//
// 🎯 **定位**：交易已提交到交易池，等待打包到区块
//
// ✅ **已有内容**：
// - TxHash: 交易哈希（唯一标识）
// - Tx: 完整的交易对象
// - SubmittedAt: 提交时间
//
// 📝 **最终状态**：
// 交易已在网络中传播，等待矿工打包
type SubmittedTx struct {
	TxHash      []byte                   // 交易哈希（32 字节）
	Tx          *transaction.Transaction // 完整的交易对象
	SubmittedAt time.Time                // 提交时间
}

// ================================================================================================
// 🎯 Part 2: Draft 数据结构（Builder 的辅助工具）
// ================================================================================================

// DraftTx 交易草稿（可变工作空间）
//
// 🎯 **定位**：Builder 的辅助工具（Compose/Plan 隐式），不是正式 Type-state
//
// 💡 **与 Type-state 的关系**：
// - Draft 不是正式 Type-state 的一部分
// - Draft.Seal() → ComposedTx（进入正式状态机）
// - 映射到架构文档中的 "Compose/Plan（隐式辅助工具）"
//
// ✅ **特性**：
// - 可变：可以多次添加 input/output
// - 有 ID：可以存储和检索
// - 支持链式调用
// - 封闭转换：Seal() 后转换为 ComposedTx
//
// 🔄 **使用场景**：
// - ISPC：合约执行过程中渐进式添加 output
// - Off-chain：CLI/API 用户交互式构建交易
type DraftTx struct {
	// ==================== 基本信息 ====================
	DraftID   string    // 草稿唯一 ID
	CreatedAt time.Time // 创建时间
	IsSealed  bool      // 是否已封闭（Seal() 后为 true）

	// ==================== 交易内容（可变）====================
	Tx *transaction.Transaction // 底层交易对象（Seal() 前可以修改）
}

// ================================================================================================
// 🎯 Part 3: 关于业务语义的架构说明
// ================================================================================================

// ⚠️ **为什么 DraftTx 不包含 BurnIntents、ApproveIntents 等业务意图？**
//
// 根据 _docs/architecture/TX_STATE_MACHINE_ARCHITECTURE.md 的核心设计原则：
//
// 1️⃣ **协议层不包含业务语义**
//    - TX 协议层只定义 inputs/outputs 的组合，不知道"销毁"、"授权"等业务概念
//    - 业务语义由**输入输出组合模式**表达，而非显式字段
//
// 2️⃣ **业务语义的正确表达方式**：
//
//    ❌ **错误方式**（违背架构）：
//    - 在 TX 中添加 `BurnIntents`、`ApproveIntents` 等字段
//    - 在协议层定义 `TransferType`、`OperationType` 等枚举
//
//    ✅ **正确方式**（符合架构）：
//    - **销毁（Burn）**：N inputs + 0 outputs（只消费不创建）
//    - **授权（Approve）**：通过 `LockingCondition` 定义权限
//      - 使用 `MultiKeyLock`（1-of-N）表达"白名单授权"
//      - 使用 `DelegationLock` 表达"委托授权"
//      - 使用 `ContractLock` 表达"智能合约裁决"
//    - **转账（Transfer）**：1 input + 2 outputs（转账+找零）
//    - **质押（Stake）**：N inputs + M outputs + ContractLock
//
// 3️⃣ **ISPC 场景的正确实现**：
//
//    ISPC 合约执行时，应该：
//    - ✅ 通过 `draft.AddInput()` 添加输入
//    - ✅ 通过 `draft.AddAssetOutput()` 添加输出
//    - ✅ 通过 `LockingCondition` 定义权限约束
//    - ❌ 不应该添加 `BurnIntent`、`ApproveIntent` 等业务标记
//
//    示例：合约授权其他用户使用资源
//    ```go
//    // ❌ 错误：添加业务意图
//    draft.AddApproveIntent(tokenID, spender, amount)
//
//    // ✅ 正确：通过 LockingCondition 表达
//    lock := &transaction.LockingCondition{
//        Condition: &transaction.LockingCondition_MultiKeyLock{
//            MultiKeyLock: &transaction.MultiKeyLock{
//                RequiredSignatures: 1,  // 1-of-N（任一授权用户可使用）
//                AuthorizedKeys: []*transaction.PublicKey{
//                    owner_pubkey,    // 所有者
//                    spender_pubkey,  // 被授权者
//                },
//            },
//        },
//    }
//    draft.AddAssetOutput(owner, amount, tokenID, lock)
//    ```
//
// 4️⃣ **架构优势**：
//    - ✅ 协议层永不改变（向后兼容）
//    - ✅ 业务层自由演进（无需修改协议）
//    - ✅ 验证逻辑统一（只验证 inputs/outputs 和权限）
//    - ✅ 符合 EUTXO 模型的本质（纯粹的输入输出组合）
//
// 📚 **相关文档**：
// - _docs/architecture/TX_STATE_MACHINE_ARCHITECTURE.md（第 234-264 行：业务语义由组合决定）
// - pb/blockchain/block/transaction/transaction.proto（第 130-160 行：输入输出组合 = 业务语义）

// ================================================================================================
// 🎯 Part 4: 交易状态数据结构
// ================================================================================================

// TxBroadcastState 交易广播状态
//
// 用途：记录交易在网络中的传播和确认状态
type TxBroadcastState struct {
	TxHash        []byte          // 交易哈希（全局唯一标识）
	Status        BroadcastStatus // 广播状态
	SubmittedAt   time.Time       // 提交时间
	BroadcastedAt *time.Time      // 广播时间（可选）
	ConfirmedAt   *time.Time      // 确认时间（可选）
	BlockHeight   uint64          // 所在区块高度（0 表示未打包）
	Confirmations uint64          // 确认数（0 表示未确认）
	ErrorMessage  string          // 错误消息（如果失败）
}

// BroadcastStatus 广播状态枚举
type BroadcastStatus string

const (
	// BroadcastStatusLocalSubmitted 已提交到本地交易池，等待广播
	BroadcastStatusLocalSubmitted BroadcastStatus = "local_submitted"

	// BroadcastStatusBroadcasted 已广播到网络，等待确认
	BroadcastStatusBroadcasted BroadcastStatus = "broadcasted"

	// BroadcastStatusConfirmed 已被区块收录并确认
	BroadcastStatusConfirmed BroadcastStatus = "confirmed"

	// BroadcastStatusBroadcastFailed 广播失败
	BroadcastStatusBroadcastFailed BroadcastStatus = "broadcast_failed"

	// BroadcastStatusExpired 已过期（超出有效期窗口）
	BroadcastStatusExpired BroadcastStatus = "expired"
)

// ================================================================================================
// 🎯 设计说明
// ================================================================================================

// 职责边界说明：
//
// ✅ **pkg/types/tx.go（本文件）**：
// - 只定义数据结构
// - 所有字段都是公开的（便于访问和序列化）
// - 不包含任何方法实现
// - 不引用任何接口
//
// ✅ **pkg/interfaces/tx/*.go**：
// - 定义所有接口（TxBuilder、TxProcessor、TxVerifier 等）
// - 定义接口方法签名
// - 不包含实现
//
// ✅ **internal/core/tx/**：
// - 实现所有接口
// - 实现 Type-state 转换方法（WithProofs、Sign、Submit）
// - 实现 Draft 操作方法（AddInput、AddOutput、Seal）
//
// 设计权衡：
//
// 问题：为什么不在 types 中定义方法？
// 回答：
// 1. 职责分离：types 只定义数据，方法是行为
// 2. 避免循环依赖：方法需要引用接口，接口又需要引用 types
// 3. 测试友好：数据结构可以独立测试，不依赖实现
// 4. 序列化友好：纯数据结构更容易序列化和反序列化
//
// 问题：为什么要将 tx_typestate.go 和 tx_draft.go 合并？
// 回答：
// 1. 归集管理：所有 TX 数据结构在一个文件中，清晰明了
// 2. 避免分散：不需要在多个文件中查找数据结构定义
// 3. 减少文件数：types 包更加简洁
// 4. 符合惯例：Go 标准库中，同一领域的数据结构通常在一个文件中
