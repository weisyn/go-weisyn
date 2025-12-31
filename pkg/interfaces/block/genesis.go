// Package block 提供区块构建的公共接口定义
//
// genesis.go - 创世区块构建接口
package block

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// GenesisBlockBuilder 创世区块构建接口
//
// 🎯 **核心职责**：
// - 构建创世区块：基于创世交易和配置构建完整的创世区块
// - 验证创世区块：对创世区块进行专门验证，使用创世区块的特殊验证规则
//
// 💡 **设计理念**：
// - 专门处理创世区块的构建和验证逻辑
// - 与普通区块构建分离，因为创世区块有特殊规则（高度为0、父哈希全零等）
// - 供 CHAIN 模块调用，用于初始化区块链
//
// 📞 **调用方**：
// - CHAIN 模块：初始化创世区块时调用
//
// ⚠️ **核心约束**：
// - 只负责构建和验证，不负责存储和处理（由其他接口提供）
type GenesisBlockBuilder interface {
	// CreateGenesisBlock 创建创世区块
	//
	// 基于创世交易和配置构建完整的创世区块。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - genesisTransactions: 创世交易列表
	//   - genesisConfig: 创世区块配置
	//
	// 返回：
	//   - *core.Block: 完整的创世区块
	//   - error: 构建过程中的错误
	CreateGenesisBlock(
		ctx context.Context,
		genesisTransactions []*transaction.Transaction,
		genesisConfig *types.GenesisConfig,
	) (*core.Block, error)

	// ValidateGenesisBlock 验证创世区块
	//
	// 对创世区块进行专门验证，使用创世区块的特殊验证规则。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - genesisBlock: 创世区块
	//
	// 返回：
	//   - bool: 验证结果，true表示创世区块有效
	//   - error: 验证过程中的错误
	ValidateGenesisBlock(
		ctx context.Context,
		genesisBlock *core.Block,
	) (bool, error)
}

