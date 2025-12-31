// Package aggregator 实现 QueryService 的聚合器
//
// 🔍 **查询服务聚合器 (Query Service Aggregator)**
//
// 本包实现 QueryService 的聚合逻辑，将所有子查询服务组合为统一的查询入口。
//
// 🎯 **核心职责**：
// - 实现 interfaces.InternalQueryService 接口
// - 聚合所有领域查询服务（通过组合）
// - 提供统一的查询入口（通过委托）
//
// 🏗️ **设计原则**：
// - 纯聚合：只做接口组合和方法委托，无业务逻辑
// - 遵循规范：实现层在子目录中（aggregator/）
// - 依赖注入：通过 fx 接收所有子服务
package aggregator

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// Service 统一查询服务实现
//
// 🎯 **核心职责**：
// 聚合所有领域查询服务，实现统一的 QueryService 接口。
//
// 💡 **实现方式**：
// - 组合所有领域查询服务
// - 通过委托模式实现查询方法
// - 遵循代码组织规范，实现内部接口
type Service struct {
	chainQuery    interfaces.InternalChainQuery    // 链状态查询
	blockQuery    interfaces.InternalBlockQuery    // 区块查询
	txQuery       interfaces.InternalTxQuery       // 交易查询
	utxoQuery     interfaces.InternalUTXOQuery     // EUTXO查询
	resourceQuery interfaces.InternalResourceQuery // 资源查询
	accountQuery  interfaces.InternalAccountQuery  // 账户查询
	pricingQuery  interfaces.InternalPricingQuery  // 定价查询（Phase 2）
	logger        log.Logger                       // 日志记录器
}

// NewService 创建新的查询服务
//
// 🏗️ **构造器模式**：
// 通过依赖注入方式创建服务实例，遵循代码组织规范。
//
// ⚙️ **参数说明**：
// - chainQuery: 链状态查询服务（内部接口）
// - blockQuery: 区块查询服务（内部接口）
// - txQuery: 交易查询服务（内部接口）
// - utxoQuery: EUTXO查询服务（内部接口）
// - resourceQuery: 资源查询服务（内部接口）
// - accountQuery: 账户查询服务（内部接口）
// - logger: 日志记录器
//
// 💡 **注意事项**：
// - 所有子查询服务必须非空
// - logger 可选，但强烈建议提供
// - 返回内部接口类型，由 module.go 绑定到公共接口
func NewService(
	chainQuery interfaces.InternalChainQuery,
	blockQuery interfaces.InternalBlockQuery,
	txQuery interfaces.InternalTxQuery,
	utxoQuery interfaces.InternalUTXOQuery,
	resourceQuery interfaces.InternalResourceQuery,
	accountQuery interfaces.InternalAccountQuery,
	pricingQuery interfaces.InternalPricingQuery, // Phase 2
	logger log.Logger,
) (interfaces.InternalQueryService, error) {
	// 验证所有查询服务
	if chainQuery == nil {
		return nil, fmt.Errorf("chainQuery 不能为空")
	}
	if blockQuery == nil {
		return nil, fmt.Errorf("blockQuery 不能为空")
	}
	if txQuery == nil {
		return nil, fmt.Errorf("txQuery 不能为空")
	}
	if utxoQuery == nil {
		return nil, fmt.Errorf("utxoQuery 不能为空")
	}
	if resourceQuery == nil {
		return nil, fmt.Errorf("resourceQuery 不能为空")
	}
	if accountQuery == nil {
		return nil, fmt.Errorf("accountQuery 不能为空")
	}
	if pricingQuery == nil {
		return nil, fmt.Errorf("pricingQuery 不能为空")
	}

	s := &Service{
		chainQuery:    chainQuery,
		blockQuery:    blockQuery,
		txQuery:       txQuery,
		utxoQuery:     utxoQuery,
		resourceQuery: resourceQuery,
		accountQuery:  accountQuery,
		pricingQuery:  pricingQuery,
		logger:        logger,
	}

	if logger != nil {
		logger.Info("✅ QueryService 统一查询服务已创建")
	}

	return s, nil
}

// ========================================
// ChainQuery 接口实现（委托）
// ========================================

func (s *Service) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	return s.chainQuery.GetChainInfo(ctx)
}

func (s *Service) GetCurrentHeight(ctx context.Context) (uint64, error) {
	return s.chainQuery.GetCurrentHeight(ctx)
}

func (s *Service) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	return s.chainQuery.GetBestBlockHash(ctx)
}

func (s *Service) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return s.chainQuery.GetNodeMode(ctx)
}

func (s *Service) IsDataFresh(ctx context.Context) (bool, error) {
	return s.chainQuery.IsDataFresh(ctx)
}

func (s *Service) IsReady(ctx context.Context) (bool, error) {
	return s.chainQuery.IsReady(ctx)
}

func (s *Service) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return s.chainQuery.GetSyncStatus(ctx)
}

// ========================================
// BlockQuery 接口实现（委托）
// ========================================

func (s *Service) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	return s.blockQuery.GetBlockByHeight(ctx, height)
}

func (s *Service) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) {
	return s.blockQuery.GetBlockByHash(ctx, blockHash)
}

func (s *Service) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	return s.blockQuery.GetBlockHeader(ctx, blockHash)
}

func (s *Service) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	return s.blockQuery.GetBlockRange(ctx, startHeight, endHeight)
}

func (s *Service) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	return s.blockQuery.GetHighestBlock(ctx)
}

// ========================================
// TxQuery 接口实现（委托）
// ========================================

func (s *Service) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *transaction.Transaction, err error) {
	return s.txQuery.GetTransaction(ctx, txHash)
}

func (s *Service) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	return s.txQuery.GetTxBlockHeight(ctx, txHash)
}

func (s *Service) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return s.txQuery.GetBlockTimestamp(ctx, height)
}

func (s *Service) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return s.txQuery.GetAccountNonce(ctx, address)
}

func (s *Service) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	return s.txQuery.GetTransactionsByBlock(ctx, blockHash)
}

// ========================================
// UTXOQuery 接口实现（委托）
// ========================================

func (s *Service) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	return s.utxoQuery.GetUTXO(ctx, outpoint)
}

func (s *Service) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return s.utxoQuery.GetUTXOsByAddress(ctx, address, category, onlyAvailable)
}

func (s *Service) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return s.utxoQuery.GetSponsorPoolUTXOs(ctx, onlyAvailable)
}

func (s *Service) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return s.utxoQuery.GetCurrentStateRoot(ctx)
}

// ========================================
// ResourceQuery 接口实现（委托）
// ========================================

func (s *Service) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	return s.resourceQuery.GetResourceByContentHash(ctx, contentHash)
}

func (s *Service) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	return s.resourceQuery.GetResourceFromBlockchain(ctx, contentHash)
}

func (s *Service) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) {
	return s.resourceQuery.GetResourceTransaction(ctx, contentHash)
}

func (s *Service) CheckFileExists(contentHash []byte) bool {
	return s.resourceQuery.CheckFileExists(contentHash)
}

func (s *Service) BuildFilePath(contentHash []byte) string {
	return s.resourceQuery.BuildFilePath(contentHash)
}

func (s *Service) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	return s.resourceQuery.ListResourceHashes(ctx, offset, limit)
}

// ========================================
// AccountQuery 接口实现（委托）
// ========================================

func (s *Service) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	return s.accountQuery.GetAccountBalance(ctx, address, tokenID)
}

// ========================================
// PricingQuery 接口实现（委托）
// ========================================

// GetPricingState 根据资源哈希查询定价状态（Phase 2）
func (s *Service) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	return s.pricingQuery.GetPricingState(ctx, resourceHash)
}

// 编译时检查接口实现
var _ interfaces.InternalQueryService = (*Service)(nil)
