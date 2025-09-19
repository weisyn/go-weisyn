package transaction

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// TransactionService 交易服务统一接口
// 整合交易索引和查询功能，提供完整的交易管理服务
type TransactionService struct {
	index        *TransactionIndex        // 交易位置索引
	queryService *TransactionQueryService // 交易查询服务
	storage      storage.BadgerStore      // 持久化存储
	logger       log.Logger               // 日志服务
}

// NewTransactionService 创建交易服务
func NewTransactionService(
	storage storage.BadgerStore,
	blockStorage BlockStorageInterface,
	logger log.Logger,
	transactionHashServiceClient transaction.TransactionHashServiceClient,
	blockHashServiceClient pb.BlockHashServiceClient,
) *TransactionService {
	// 创建交易索引
	index := NewTransactionIndex(storage, logger, transactionHashServiceClient)

	// 创建查询服务
	queryService := NewTransactionQueryService(index, blockStorage, logger, transactionHashServiceClient, blockHashServiceClient)

	return &TransactionService{
		index:        index,
		queryService: queryService,
		storage:      storage,
		logger:       logger,
	}
}

// ========== 索引管理接口 ==========

// IndexTransactions 为区块中的所有交易建立索引
// ⚠️ 【写入边界】此方法只能在Manager.StoreBlock的事务中调用
// 在存储新区块时调用此方法
func (ts *TransactionService) IndexTransactions(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte, block *pb.Block) error {
	return ts.index.IndexTransactions(ctx, tx, blockHash, block)
}

// RemoveTransactionIndex 移除交易索引
// ⚠️ 【写入边界】此方法只能在Manager区块回滚事务中调用
// 在区块回滚时调用此方法
func (ts *TransactionService) RemoveTransactionIndex(ctx context.Context, tx storage.BadgerTransaction, txHash []byte) error {
	return ts.index.RemoveTransactionIndex(ctx, tx, txHash)
}

// ========== 查询服务接口 ==========

// GetTransaction 根据交易哈希获取完整交易信息
func (ts *TransactionService) GetTransaction(ctx context.Context, txHash []byte) (*TransactionDetail, error) {
	return ts.queryService.GetTransaction(ctx, txHash)
}

// GetTransactionsByBlockHash 获取指定区块中的所有交易
func (ts *TransactionService) GetTransactionsByBlockHash(ctx context.Context, blockHash []byte) ([]*TransactionDetail, error) {
	return ts.queryService.GetTransactionsByBlockHash(ctx, blockHash)
}

// GetTransactionsByBlockHeight 获取指定高度区块中的所有交易
func (ts *TransactionService) GetTransactionsByBlockHeight(ctx context.Context, height uint64) ([]*TransactionDetail, error) {
	return ts.queryService.GetTransactionsByBlockHeight(ctx, height)
}

// HasTransaction 检查交易是否存在
func (ts *TransactionService) HasTransaction(ctx context.Context, txHash []byte) (bool, error) {
	return ts.queryService.HasTransaction(ctx, txHash)
}

// GetTransactionLocation 获取交易位置信息
func (ts *TransactionService) GetTransactionLocation(ctx context.Context, txHash []byte) (*TransactionLocation, error) {
	return ts.queryService.GetTransactionLocation(ctx, txHash)
}

// ValidateTransaction 验证交易数据完整性
func (ts *TransactionService) ValidateTransaction(ctx context.Context, txHash []byte) error {
	return ts.queryService.ValidateTransaction(ctx, txHash)
}

// ========== 批量操作接口 ==========

// GetMultipleTransactions 批量获取多个交易
func (ts *TransactionService) GetMultipleTransactions(ctx context.Context, txHashes [][]byte) ([]*TransactionDetail, []error) {
	if len(txHashes) == 0 {
		return nil, []error{fmt.Errorf("交易哈希列表不能为空")}
	}

	if ts.logger != nil {
		ts.logger.Debugf("批量查询交易 - count: %d", len(txHashes))
	}

	results := make([]*TransactionDetail, len(txHashes))
	errors := make([]error, len(txHashes))

	// 并发查询每个交易
	for i, txHash := range txHashes {
		detail, err := ts.queryService.GetTransaction(ctx, txHash)
		results[i] = detail
		errors[i] = err
	}

	if ts.logger != nil {
		successCount := 0
		for _, err := range errors {
			if err == nil {
				successCount++
			}
		}
		ts.logger.Debugf("批量查询完成 - total: %d, success: %d, failed: %d",
			len(txHashes), successCount, len(txHashes)-successCount)
	}

	return results, errors
}

// ValidateTransactions 批量验证交易
func (ts *TransactionService) ValidateTransactions(ctx context.Context, txHashes [][]byte) []error {
	if len(txHashes) == 0 {
		return []error{fmt.Errorf("交易哈希列表不能为空")}
	}

	if ts.logger != nil {
		ts.logger.Debugf("批量验证交易 - count: %d", len(txHashes))
	}

	errors := make([]error, len(txHashes))

	// 验证每个交易
	for i, txHash := range txHashes {
		errors[i] = ts.queryService.ValidateTransaction(ctx, txHash)
	}

	if ts.logger != nil {
		validCount := 0
		for _, err := range errors {
			if err == nil {
				validCount++
			}
		}
		ts.logger.Debugf("批量验证完成 - total: %d, valid: %d, invalid: %d",
			len(txHashes), validCount, len(txHashes)-validCount)
	}

	return errors
}

// ========== 统计查询接口 ==========

// GetTransactionStats 获取交易统计信息
func (ts *TransactionService) GetTransactionStats(ctx context.Context) (*TransactionStats, error) {
	if ts.logger != nil {
		ts.logger.Debugf("查询交易统计信息")
	}

	// 返回基础统计信息
	// 注意：详细统计信息需要从ChainState或专门的统计索引获取
	stats := &TransactionStats{
		TotalTransactions: 0, // 从 ChainState 获取
		// 其他统计信息根据业务需要扩展
	}

	return stats, nil
}

// TransactionStats 交易统计信息
type TransactionStats struct {
	TotalTransactions uint64 `json:"total_transactions"` // 总交易数量
	// 可以添加更多统计字段：
	// TotalAssetTransfers uint64 `json:"total_asset_transfers"`   // 资产转移交易数
	// TotalStateUpdates   uint64 `json:"total_state_updates"`     // 状态更新交易数
	// TotalResourceOps    uint64 `json:"total_resource_ops"`      // 资源操作交易数
}

// ========== 高级查询接口 ==========

// SearchTransactionsByAddress 根据地址搜索相关交易
func (ts *TransactionService) SearchTransactionsByAddress(ctx context.Context, address []byte) ([]*TransactionDetail, error) {
	if len(address) == 0 {
		return nil, fmt.Errorf("地址不能为空")
	}

	if ts.logger != nil {
		ts.logger.Debugf("根据地址搜索交易 - address: %x", address)
	}

	// 当前实现：返回空结果
	// 地址索引功能需要在index模块中实现专门的地址到交易的映射
	// 这涉及到对所有交易的输入输出地址进行索引建立
	var results []*TransactionDetail

	if ts.logger != nil {
		ts.logger.Warnf("地址交易搜索需要地址索引支持，当前返回空结果")
	}

	return results, nil
}

// GetTransactionHistory 获取地址的交易历史（分页）
func (ts *TransactionService) GetTransactionHistory(ctx context.Context, address []byte, offset, limit uint64) ([]*TransactionDetail, uint64, error) {
	if len(address) == 0 {
		return nil, 0, fmt.Errorf("地址不能为空")
	}

	if ts.logger != nil {
		ts.logger.Debugf("查询地址交易历史 - address: %x, offset: %d, limit: %d", address, offset, limit)
	}

	// 当前实现：返回空结果和0总数
	// 分页查询功能需要配合地址索引和分页机制
	var results []*TransactionDetail
	var totalCount uint64 = 0

	if ts.logger != nil {
		ts.logger.Warnf("交易历史分页查询需要地址索引支持，当前返回空结果")
	}

	return results, totalCount, nil
}
