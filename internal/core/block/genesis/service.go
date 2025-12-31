// Package genesis 实现创世区块构建服务
//
// 🎯 **创世区块构建服务 (Genesis Block Builder Service)**
//
// 本包实现了创世区块的构建和验证服务，提供：
// - 创世区块构建：基于配置和交易构建创世区块
// - 创世区块验证：验证创世区块的有效性
//
// 🏗️ **设计原则**
// - 实现 GenesisBlockBuilder 接口（定义在 pkg/interfaces/block/genesis.go）
// - 委托给 builder.go 和 validator.go 实现具体逻辑
// - 依赖注入：通过构造函数注入所需依赖
package genesis

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/block/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                              服务结构定义
// ============================================================================

// Service 创世区块构建服务实现
//
// 🎯 **职责**：
// - 实现 InternalGenesisBlockBuilder 接口（内部接口）
// - 委托给 builder.go 和 validator.go 执行具体逻辑
//
// 🏗️ **架构原则**：
// - 实现内部接口，遵循代码组织规范
// - 公共接口通过内部接口桥接
type Service struct {
	// 依赖
	txHashClient transaction.TransactionHashServiceClient
	hashManager  crypto.HashManager
	utxoQuery    persistence.UTXOQuery
	logger       log.Logger
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewService 创建创世区块构建服务
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数：
//   - txHashClient: 交易哈希服务客户端（必需）
//   - hashManager: 哈希管理器（必需，用于Merkle树）
//   - utxoQuery: UTXO查询服务（可选，用于获取状态根）
//   - logger: 日志服务（可选）
//
// 返回：
//   - interfaces.InternalGenesisBlockBuilder: 创世区块构建内部接口
//   - error: 创建错误
func NewService(
	txHashClient transaction.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	utxoQuery persistence.UTXOQuery,
	logger log.Logger,
) (interfaces.InternalGenesisBlockBuilder, error) {
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("hashManager 不能为空")
	}

	service := &Service{
		txHashClient: txHashClient,
		hashManager:  hashManager,
		utxoQuery:    utxoQuery,
		logger:       logger,
	}

	if logger != nil {
		logger.Info("✅ GenesisBlockBuilder 服务已创建")
	}

	return service, nil
}

// ============================================================================
//                              接口实现
// ============================================================================

// CreateGenesisBlock 创建创世区块
//
// 🎯 **GenesisBlockBuilder 接口实现**
//
// 委托给 builder.BuildBlock 执行实际构建。
func (s *Service) CreateGenesisBlock(
	ctx context.Context,
	genesisTransactions []*transaction.Transaction,
	genesisConfig *types.GenesisConfig,
) (*core.Block, error) {
	return BuildBlock(
		ctx,
		genesisTransactions,
		genesisConfig,
		s.txHashClient,
		s.hashManager,
		s.utxoQuery,
		s.logger,
	)
}

// ValidateGenesisBlock 验证创世区块
//
// 🎯 **GenesisBlockBuilder 接口实现**
//
// 委托给 validator.ValidateBlock 执行实际验证。
func (s *Service) ValidateGenesisBlock(
	ctx context.Context,
	genesisBlock *core.Block,
) (bool, error) {
	return ValidateBlock(
		ctx,
		genesisBlock,
		s.txHashClient,
		s.hashManager,
		s.logger,
	)
}

// ============================================================================
//                              编译时检查
// ============================================================================

// 确保 Service 实现了 InternalGenesisBlockBuilder 接口
// 这会自动满足 block.GenesisBlockBuilder 接口（因为内部接口嵌入了公共接口）
var _ interfaces.InternalGenesisBlockBuilder = (*Service)(nil)

