// Package block 提供区块验证的公共接口定义
//
// ✓ **区块验证接口 (Block Validator)**
//
// 本包定义 WES 系统的区块验证接口，遵循 CQRS 架构原则，
// 专注于区块的验证操作。
//
// 🎯 **核心职责**：
// - 验证区块有效性
//
// 🏗️ **设计原则**：
// - CQRS 读路径：区块验证属于读操作（不修改状态）
// - 职责单一：只负责区块验证
//
// 详细使用说明请参考：pkg/interfaces/block/README.md
package block

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// BlockValidator 区块验证接口（读操作）
//
// 🎯 **核心职责**：
// 提供区块验证操作，确保区块符合共识规则和协议要求。
//
// 💡 **设计理念**：
// - 只包含验证操作，不包含处理（由 BlockProcessor 提供）
// - 只读操作，不修改状态
// - 完整的验证流程，包括格式、签名、共识等
//
// 📞 **调用方**：
// - SyncService：同步过程中验证区块
// - ConsensusService：接收区块时验证
//
// ⚠️ **核心约束**：
// - 只读不写：所有方法都是验证操作，不修改状态
// - 完整性：必须进行完整验证，不能跳过任何步骤
type BlockValidator interface {
	// ValidateBlock 验证区块
	//
	// 对区块进行完整验证，确保符合共识规则和协议要求。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - block: 待验证的区块
	//
	// 返回：
	//   - bool: 验证结果，true表示有效
	//   - error: 验证错误，nil表示验证完成
	//
	// 使用场景：
	//   - 节点验证从网络接收的区块
	//   - 同步过程中的区块有效性检查
	//   - 分叉处理中的区块验证
	//
	// 说明：
	//   - 验证包括格式验证、签名验证、共识验证等
	//   - 验证失败时返回 false 和错误信息
	ValidateBlock(ctx context.Context, block *core.Block) (bool, error)
}
