package transaction

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// 交易查询服务 - 从区块中实时提取交易数据
// 严格遵循"区块单一数据源"原则，所有交易数据都从区块中提取

// BlockStorageInterface 区块存储接口
// 用于从区块存储中获取完整区块数据
type BlockStorageInterface interface {
	GetBlock(ctx context.Context, blockHash []byte) (*pb.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*pb.Block, error)
}

// TransactionQueryService 交易查询服务
type TransactionQueryService struct {
	txIndex                      *TransactionIndex                        // 交易位置索引
	blockStorage                 BlockStorageInterface                    // 区块存储接口
	logger                       log.Logger                               // 日志服务
	transactionHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	blockHashServiceClient       pb.BlockHashServiceClient                // 区块哈希服务客户端
}

// NewTransactionQueryService 创建交易查询服务
func NewTransactionQueryService(
	txIndex *TransactionIndex,
	blockStorage BlockStorageInterface,
	logger log.Logger,
	transactionHashServiceClient transaction.TransactionHashServiceClient,
	blockHashServiceClient pb.BlockHashServiceClient,
) *TransactionQueryService {
	return &TransactionQueryService{
		txIndex:                      txIndex,
		blockStorage:                 blockStorage,
		logger:                       logger,
		transactionHashServiceClient: transactionHashServiceClient,
		blockHashServiceClient:       blockHashServiceClient,
	}
}

// TransactionDetail 交易详细信息
// 包含交易数据及其在区块链中的位置信息
type TransactionDetail struct {
	Transaction *transaction.Transaction // 完整交易数据
	BlockHash   []byte                   // 所在区块哈希
	BlockHeight uint64                   // 所在区块高度
	TxIndex     uint32                   // 在区块中的索引
	TxHash      []byte                   // 交易哈希
}

// GetTransaction 根据交易哈希获取完整交易信息
func (tqs *TransactionQueryService) GetTransaction(ctx context.Context, txHash []byte) (*TransactionDetail, error) {
	if len(txHash) == 0 {
		return nil, fmt.Errorf("交易哈希不能为空")
	}

	if tqs.logger != nil {
		tqs.logger.Debugf("查询交易 - tx_hash: %x", txHash)
	}

	// 1. 从索引获取交易位置
	location, err := tqs.txIndex.GetTransactionLocation(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("获取交易位置失败: %w", err)
	}

	// 2. 从区块存储获取完整区块
	block, err := tqs.blockStorage.GetBlock(ctx, location.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("获取区块失败 - block_hash: %x, error: %w", location.BlockHash, err)
	}

	// 3. 验证交易索引范围
	if location.TxIndex >= uint32(len(block.Body.Transactions)) {
		return nil, fmt.Errorf("交易索引越界 - tx_index: %d, total_txs: %d",
			location.TxIndex, len(block.Body.Transactions))
	}

	// 4. 提取指定位置的交易
	tx := block.Body.Transactions[location.TxIndex]

	// 5. 验证交易哈希一致性
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}
	resp, err := tqs.transactionHashServiceClient.ComputeHash(ctx, req)
	if err != nil || !resp.IsValid {
		return nil, fmt.Errorf("计算交易哈希失败: %w", err)
	}
	actualTxHash := resp.Hash
	if !equalBytes(txHash, actualTxHash) {
		return nil, fmt.Errorf("交易哈希不匹配 - expected: %x, actual: %x", txHash, actualTxHash)
	}

	// 6. 构建交易详细信息
	detail := &TransactionDetail{
		Transaction: tx,
		BlockHash:   location.BlockHash,
		BlockHeight: block.Header.Height,
		TxIndex:     location.TxIndex,
		TxHash:      txHash,
	}

	if tqs.logger != nil {
		tqs.logger.Debugf("成功查询交易 - tx_hash: %x, block_height: %d, tx_index: %d",
			txHash, detail.BlockHeight, detail.TxIndex)
	}

	return detail, nil
}

// GetTransactionsByBlockHash 获取指定区块中的所有交易
func (tqs *TransactionQueryService) GetTransactionsByBlockHash(ctx context.Context, blockHash []byte) ([]*TransactionDetail, error) {
	if len(blockHash) == 0 {
		return nil, fmt.Errorf("区块哈希不能为空")
	}

	if tqs.logger != nil {
		tqs.logger.Debugf("查询区块交易列表 - block_hash: %x", blockHash)
	}

	// 1. 从区块存储获取完整区块
	block, err := tqs.blockStorage.GetBlock(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("获取区块失败: %w", err)
	}

	// 2. 构建所有交易的详细信息
	transactions := make([]*TransactionDetail, len(block.Body.Transactions))
	for i, tx := range block.Body.Transactions {
		// 使用交易哈希服务计算交易哈希
		req := &transaction.ComputeHashRequest{
			Transaction:      tx,
			IncludeDebugInfo: false,
		}
		resp, err := tqs.transactionHashServiceClient.ComputeHash(ctx, req)
		if err != nil || !resp.IsValid {
			return nil, fmt.Errorf("计算交易哈希失败 - tx_index: %d, error: %w", i, err)
		}
		txHash := resp.Hash

		transactions[i] = &TransactionDetail{
			Transaction: tx,
			BlockHash:   blockHash,
			BlockHeight: block.Header.Height,
			TxIndex:     uint32(i),
			TxHash:      txHash,
		}
	}

	if tqs.logger != nil {
		tqs.logger.Debugf("成功查询区块交易列表 - block_hash: %x, tx_count: %d",
			blockHash, len(transactions))
	}

	return transactions, nil
}

// GetTransactionsByBlockHeight 获取指定高度区块中的所有交易
func (tqs *TransactionQueryService) GetTransactionsByBlockHeight(ctx context.Context, height uint64) ([]*TransactionDetail, error) {
	if tqs.logger != nil {
		tqs.logger.Debugf("查询指定高度区块交易列表 - height: %d", height)
	}

	// 1. 从区块存储获取指定高度的区块
	block, err := tqs.blockStorage.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("获取指定高度区块失败: %w", err)
	}

	// 2. 使用区块哈希服务计算区块哈希
	req := &pb.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}
	resp, err := tqs.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil || !resp.IsValid {
		return nil, fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash

	return tqs.GetTransactionsByBlockHash(ctx, blockHash)
}

// HasTransaction 检查交易是否存在
func (tqs *TransactionQueryService) HasTransaction(ctx context.Context, txHash []byte) (bool, error) {
	return tqs.txIndex.HasTransaction(ctx, txHash)
}

// GetTransactionLocation 获取交易位置信息（不包含交易内容）
func (tqs *TransactionQueryService) GetTransactionLocation(ctx context.Context, txHash []byte) (*TransactionLocation, error) {
	return tqs.txIndex.GetTransactionLocation(ctx, txHash)
}

// ValidateTransaction 验证交易数据完整性
func (tqs *TransactionQueryService) ValidateTransaction(ctx context.Context, txHash []byte) error {
	if tqs.logger != nil {
		tqs.logger.Debugf("验证交易数据完整性 - tx_hash: %x", txHash)
	}

	// 1. 获取交易详细信息
	detail, err := tqs.GetTransaction(ctx, txHash)
	if err != nil {
		return fmt.Errorf("获取交易失败: %w", err)
	}

	// 2. 验证交易基本结构
	if detail.Transaction == nil {
		return fmt.Errorf("交易数据为空")
	}

	// 3. 验证输入输出数量
	if len(detail.Transaction.Inputs) == 0 && len(detail.Transaction.Outputs) == 0 {
		return fmt.Errorf("交易输入输出均为空")
	}

	// 4. 验证交易哈希一致性
	req := &transaction.ComputeHashRequest{
		Transaction:      detail.Transaction,
		IncludeDebugInfo: false,
	}
	resp, err := tqs.transactionHashServiceClient.ComputeHash(ctx, req)
	if err != nil || !resp.IsValid {
		return fmt.Errorf("计算交易哈希失败: %w", err)
	}
	computedHash := resp.Hash
	if !equalBytes(txHash, computedHash) {
		return fmt.Errorf("交易哈希验证失败 - expected: %x, computed: %x", txHash, computedHash)
	}

	if tqs.logger != nil {
		tqs.logger.Debugf("交易数据验证通过 - tx_hash: %x", txHash)
	}

	return nil
}

// GetTransactionCount 获取总交易数量统计
func (tqs *TransactionQueryService) GetTransactionCount(ctx context.Context) (uint64, error) {
	// 注意：这个方法需要遍历所有区块来统计，性能较低
	// 生产环境应该从ChainState获取预计算的统计信息

	if tqs.logger != nil {
		tqs.logger.Debugf("统计总交易数量（注意：此操作性能较低）")
	}

	// 当前实现：返回0作为默认值
	// 实际部署时，应该从ChainState模块获取这个统计信息
	var totalCount uint64 = 0

	if tqs.logger != nil {
		tqs.logger.Warnf("交易数量统计需要从ChainState获取，当前返回0")
	}

	return totalCount, nil
}

// ========== 辅助函数 ==========

// equalBytes 比较两个字节数组是否相等
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
