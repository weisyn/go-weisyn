// Package interfaces provides port interfaces for transaction operations.
package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 端口接口定义（Hexagonal Architecture - Ports）
// ════════════════════════════════════════════════════════════════════════════════════════════════
//
// 🎯 **设计理念**：
//   - 端口接口抽象外部依赖，支持多种实现的灵活替换
//   - 符合六边形架构的"端口/适配器"模式
//   - 每个端口接口对应 ports/ 下的一个实现子目录
//
// 📁 **实现目录映射**：
//   - Signer         → ports/signer/
//   - FeeEstimator   → ports/fee/
//   - ProofProvider  → ports/proof/
//   - DraftStore     → ports/draftstore/
//
// 🔌 **适配器实现示例**：
//   - LocalSigner / KMSSigner / HSMSigner
//   - StaticFeeEstimator / DynamicFeeEstimator
//   - SimpleProofProvider / MultiProofProvider
//   - MemoryDraftStore / RedisDraftStore
//
// ════════════════════════════════════════════════════════════════════════════════════════════════

// Signer 签名服务端口接口
//
// 🎯 **职责**：对交易进行数字签名
//
// 🔄 **继承关系**：
//   - 继承 tx.Signer 公共接口
//   - 当前无内部扩展方法（未来可根据需要扩展）
//
// 📁 **实现目录**：internal/core/tx/ports/signer/
//
// 🔌 **典型实现**：
//   - LocalSigner: 使用本地私钥签名（开发/测试环境）
//   - KMSSigner: 使用 AWS KMS 签名（云环境）
//   - HSMSigner: 使用硬件安全模块签名（企业环境）
//
// ⚠️ **核心约束**：
//   - 不能修改交易内容
//   - 签名必须可验证（与 LockingCondition 匹配）
//   - 签名算法必须符合系统要求
type Signer interface {
	// 继承公共签名服务接口
	tx.Signer

	// 💡 内部扩展方法（暂无，保留接口以便未来扩展）
}

// FeeEstimator 费用估算端口接口
//
// 🎯 **职责**：估算交易所需的费用
//
// 🔄 **继承关系**：
//   - 继承 tx.FeeEstimator 公共接口
//   - 当前无内部扩展方法（未来可根据需要扩展）
//
// 📁 **实现目录**：internal/core/tx/ports/fee/
//
// 🔌 **典型实现**：
//   - StaticFeeEstimator: 固定费率（最简单）
//   - DynamicFeeEstimator: 根据网络拥堵动态调整
//   - PriorityFeeEstimator: 支持优先级加速
//
// ⚠️ **核心约束**：
//   - 不能修改交易
//   - 估算结果只是建议，不强制执行
//   - 实际费用由 Verifier 检查
type FeeEstimator interface {
	// 继承公共费用估算接口
	tx.FeeEstimator

	// 💡 内部扩展方法（暂无，保留接口以便未来扩展）
}

// ProofProvider 证明提供者端口接口
//
// 🎯 **职责**：为交易输入生成解锁证明（UnlockingProof）
//
// 🔄 **继承关系**：
//   - 继承 tx.ProofProvider 公共接口
//   - 当前无内部扩展方法（未来可根据需要扩展）
//
// 📁 **实现目录**：internal/core/tx/ports/proof/
//
// 🔌 **典型实现**：
//   - SimpleProofProvider: 为所有 input 使用相同的签名
//   - MultiProofProvider: 为不同 input 使用不同的签名源
//   - DelegatedProofProvider: 支持委托授权证明
//
// ⚠️ **核心约束**：
//   - 必须为所有 input 生成对应的 proof
//   - 生成的 proof 必须匹配 UTXO 的 LockingCondition
//   - 不能修改交易的其他部分
type ProofProvider interface {
	// 继承公共证明提供者接口
	tx.ProofProvider

	// 💡 内部扩展方法（暂无，保留接口以便未来扩展）
}

// DraftStore 草稿存储端口接口
//
// 🎯 **职责**：存储和检索交易草稿
//
// 🔄 **继承关系**：
//   - 继承 tx.DraftStore 公共接口
//   - 当前无内部扩展方法（未来可根据需要扩展）
//
// 📁 **实现目录**：internal/core/tx/ports/draftstore/
//
// 🔌 **典型实现**：
//   - MemoryDraftStore: 内存存储（快速，但不持久）- 适用于 ISPC 场景
//   - RedisDraftStore: Redis 存储（分布式，支持 TTL）- 适用于 Off-chain 场景
//   - DBDraftStore: 数据库存储（持久化，支持查询）- 适用于企业场景
//
// ⚠️ **核心约束**：
//   - Save() 返回 draftID，用于后续检索
//   - Get() 返回的草稿可以继续修改（如果未封闭）
//   - Delete() 删除草稿，释放存储空间
//   - TTL（可选）：草稿可以设置过期时间
type DraftStore interface {
	// 继承公共草稿存储接口
	tx.DraftStore

	// 💡 内部扩展方法（暂无，保留接口以便未来扩展）
}
