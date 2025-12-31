// Package ispc provides zero-knowledge proof interfaces for ISPC operations.
package ispc

import (
	"context"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ════════════════════════════════════════════════════════════════════════════════════════════════
// ZKProofService - 零知识证明服务接口（公共接口）
// ════════════════════════════════════════════════════════════════════════════════════════════════
//
// 📋 **接口说明**：
//   - 该接口定义了 ISPC 层对外提供的 ZK 证明生成和验证能力
//   - 由 internal/core/ispc/zkproof 实现
//   - 主要供 ISPC Coordinator 内部使用，不对外部调用者暴露
//
// 🔒 **设计约束**：
//   - ✅ 轻量级接口：只暴露核心方法，不暴露内部实现细节
//   - ✅ 无业务语义：不包含业务相关的证明类型
//   - ✅ 专注证明：只负责证明生成和验证，不涉及电路管理（内部关注）
//   - ✅ 简化参数：使用通用类型，避免暴露内部数据结构
//
// 📚 **实现文档**：
//   - 实现：internal/core/ispc/zkproof/README.md
//   - 内部接口：internal/core/ispc/interfaces/zkproof.go
//
// ════════════════════════════════════════════════════════════════════════════════════════════════

type ZKProofService interface {

	// ════════════════════════════════════════════════════════════════════════════════════════
	// 核心方法（对外暴露）
	// ════════════════════════════════════════════════════════════════════════════════════════

	// GenerateStateProof 为执行结果生成 ZK 状态证明
	//
	// 📋 **参数**：
	//   - ctx: 调用上下文（用于超时控制）
	//   - executionResultHash: 执行结果哈希（32 字节）
	//   - publicInputs: ZK 证明的公开输入（链上可见）
	//   - circuitID: 电路标识符（如 "contract_execution_v1"）
	//
	// 🔧 **返回值**：
	//   - *pb.ZKStateProof: 生成的 ZK 状态证明（protobuf 格式）
	//   - error: 生成失败时的错误信息
	//
	// 🎯 **用途**：
	//   - 由 ISPC Coordinator 在合约执行后调用
	//   - 生成的证明用于 StateOutput，确保执行可验证性
	//   - 证明包含：proof 字节、public inputs、proving scheme 等信息
	//
	// ⚠️ **注意**：
	//   - 生成证明是计算密集型操作，可能需要几秒到几分钟
	//   - 应设置合理的 context 超时时间
	//   - 公开输入会进入链上，应控制大小
	GenerateStateProof(
		ctx context.Context,
		executionResultHash []byte,
		publicInputs [][]byte,
		circuitID string,
	) (*pb.ZKStateProof, error)

	// VerifyStateProof 验证 ZK 状态证明的有效性
	//
	// 📋 **参数**：
	//   - ctx: 调用上下文
	//   - proof: 待验证的 ZK 状态证明
	//
	// 🔧 **返回值**：
	//   - bool: 验证是否通过（true=有效，false=无效）
	//   - error: 验证过程中的错误（nil 表示验证成功完成）
	//
	// 🎯 **用途**：
	//   - 由 TX 验证层在交易验证时调用
	//   - 确保 StateOutput 中的证明是有效的
	//   - 用于区块验证和同步过程中的证明验证
	//
	// ⚠️ **注意**：
	//   - 验证失败不一定是错误，可能是证明本身无效
	//   - 返回 (false, nil) 表示证明无效但验证过程正常
	//   - 返回 (false, err) 表示验证过程出错
	VerifyStateProof(
		ctx context.Context,
		proof *pb.ZKStateProof,
	) (bool, error)
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 设计说明
// ════════════════════════════════════════════════════════════════════════════════════════════════
//
// ## 为什么只暴露这两个方法？
//
// 1. **简化公共接口**：
//    - 证明生成和验证是核心能力
//    - 电路管理（LoadCircuit, IsCircuitLoaded）属于内部实现细节
//    - 性能指标（GenerationTimeMs, ProofSizeBytes）不需要对外暴露
//
// 2. **符合架构分层**：
//    - 公共接口：pkg/interfaces/ispc/zkproof.go（本文件，轻量级）
//    - 内部接口：internal/core/ispc/interfaces/zkproof.go（完整功能）
//    - 实现层：internal/core/ispc/zkproof/manager.go（具体实现）
//
// 3. **降低外部依赖**：
//    - 外部调用者不需要了解 ZKProofInput、ZKProofResult 等内部结构
//    - 使用 pb.ZKStateProof 作为统一的证明格式
//    - 参数简化，便于理解和使用
//
// 4. **ISPC 专用**：
//    - 该接口主要供 ISPC Coordinator 内部使用
//    - 不是通用的 ZK 证明服务（如 zkSNARK SDK）
//    - 专注于合约执行结果的证明生成
//
// ════════════════════════════════════════════════════════════════════════════════════════════════
