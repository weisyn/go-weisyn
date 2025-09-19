package transaction

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 交易位置索引 - 仅存储位置信息，不存储交易内容
// 严格遵循"区块单一数据源"原则，索引只存储位置映射

const (
	// 交易索引键前缀
	TxIndexKeyPrefix = "tx:"
)

// TransactionLocation 交易位置信息
// 描述交易在区块链中的精确位置
type TransactionLocation struct {
	BlockHash []byte // 所在区块哈希
	TxIndex   uint32 // 在区块中的交易索引（从0开始）
}

// TransactionIndex 交易位置索引管理器
type TransactionIndex struct {
	storage                      storage.BadgerStore                      // 持久化存储
	logger                       log.Logger                               // 日志服务
	transactionHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
}

// NewTransactionIndex 创建交易索引管理器
func NewTransactionIndex(storage storage.BadgerStore, logger log.Logger, transactionHashServiceClient transaction.TransactionHashServiceClient) *TransactionIndex {
	return &TransactionIndex{
		storage:                      storage,
		logger:                       logger,
		transactionHashServiceClient: transactionHashServiceClient,
	}
}

// IndexTransactions 为区块中的所有交易建立索引
// ⚠️ 【写入边界】此方法只能在TransactionService.IndexTransactions中调用
// 这个方法在存储区块时被调用，批量建立交易索引
func (ti *TransactionIndex) IndexTransactions(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte, block *pb.Block) error {
	if ti.logger != nil {
		ti.logger.Debugf("为区块建立交易索引 - height: %d, tx_count: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	// 验证区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}

	// 为每个交易建立索引
	for txIndex, txData := range block.Body.Transactions {
		// 使用交易哈希服务计算交易哈希
		req := &transaction.ComputeHashRequest{
			Transaction:      txData,
			IncludeDebugInfo: false,
		}
		resp, err := ti.transactionHashServiceClient.ComputeHash(ctx, req)
		if err != nil || !resp.IsValid {
			return fmt.Errorf("计算交易哈希失败 - tx_index: %d, error: %w", txIndex, err)
		}
		txHash := resp.Hash

		// 创建位置信息
		location := &TransactionLocation{
			BlockHash: blockHash,
			TxIndex:   uint32(txIndex),
		}

		// 存储交易位置索引
		if err := ti.setTransactionLocation(tx, txHash, location); err != nil {
			return fmt.Errorf("存储交易位置索引失败 - tx_index: %d, error: %w", txIndex, err)
		}

		if ti.logger != nil {
			ti.logger.Debugf("成功建立交易索引 - tx_index: %d, tx_hash: %x", txIndex, txHash)
		}
	}

	if ti.logger != nil {
		ti.logger.Debugf("完成区块交易索引建立 - height: %d", block.Header.Height)
	}

	return nil
}

// GetTransactionLocation 根据交易哈希获取交易位置
func (ti *TransactionIndex) GetTransactionLocation(ctx context.Context, txHash []byte) (*TransactionLocation, error) {
	if len(txHash) == 0 {
		return nil, fmt.Errorf("交易哈希不能为空")
	}

	key := formatTxIndexKey(txHash)
	data, err := ti.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("查询交易位置失败: %w", err)
	}

	// 根据BadgerDB接口的设计，键不存在时返回nil值和nil错误
	if data == nil {
		return nil, fmt.Errorf("交易不存在 - tx_hash: %x", txHash)
	}

	location, err := deserializeTransactionLocation(data)
	if err != nil {
		return nil, fmt.Errorf("反序列化交易位置失败: %w", err)
	}

	return location, nil
}

// HasTransaction 检查交易是否存在
func (ti *TransactionIndex) HasTransaction(ctx context.Context, txHash []byte) (bool, error) {
	if len(txHash) == 0 {
		return false, fmt.Errorf("交易哈希不能为空")
	}

	key := formatTxIndexKey(txHash)
	exists, err := ti.storage.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查交易存在性失败: %w", err)
	}

	return exists, nil
}

// RemoveTransactionIndex 移除交易索引（用于区块回滚等场景）
// ⚠️ 【写入边界】此方法只能在TransactionService.RemoveTransactionIndex中调用
func (ti *TransactionIndex) RemoveTransactionIndex(ctx context.Context, tx storage.BadgerTransaction, txHash []byte) error {
	if len(txHash) == 0 {
		return fmt.Errorf("交易哈希不能为空")
	}

	key := formatTxIndexKey(txHash)
	if err := tx.Delete(key); err != nil {
		return fmt.Errorf("删除交易索引失败: %w", err)
	}

	if ti.logger != nil {
		ti.logger.Debugf("成功删除交易索引 - tx_hash: %x", txHash)
	}

	return nil
}

// setTransactionLocation 设置交易位置（内部方法）
func (ti *TransactionIndex) setTransactionLocation(tx storage.BadgerTransaction, txHash []byte, location *TransactionLocation) error {
	key := formatTxIndexKey(txHash)
	data, err := serializeTransactionLocation(location)
	if err != nil {
		return fmt.Errorf("序列化交易位置失败: %w", err)
	}

	if err := tx.Set(key, data); err != nil {
		return fmt.Errorf("存储交易位置失败: %w", err)
	}

	return nil
}

// ========== 辅助函数 ==========

// formatTxIndexKey 格式化交易索引键
func formatTxIndexKey(txHash []byte) []byte {
	key := make([]byte, len(TxIndexKeyPrefix)+len(txHash))
	copy(key, []byte(TxIndexKeyPrefix))
	copy(key[len(TxIndexKeyPrefix):], txHash)
	return key
}

// serializeTransactionLocation 序列化交易位置
func serializeTransactionLocation(location *TransactionLocation) ([]byte, error) {
	// 使用简单的二进制格式：[BlockHashLength(4字节)][BlockHash][TxIndex(4字节)]
	blockHashLen := len(location.BlockHash)
	data := make([]byte, 4+blockHashLen+4)

	// 写入区块哈希长度
	data[0] = byte(blockHashLen >> 24)
	data[1] = byte(blockHashLen >> 16)
	data[2] = byte(blockHashLen >> 8)
	data[3] = byte(blockHashLen)

	// 写入区块哈希
	copy(data[4:4+blockHashLen], location.BlockHash)

	// 写入交易索引
	offset := 4 + blockHashLen
	data[offset] = byte(location.TxIndex >> 24)
	data[offset+1] = byte(location.TxIndex >> 16)
	data[offset+2] = byte(location.TxIndex >> 8)
	data[offset+3] = byte(location.TxIndex)

	return data, nil
}

// deserializeTransactionLocation 反序列化交易位置
func deserializeTransactionLocation(data []byte) (*TransactionLocation, error) {
	if len(data) < 8 { // 至少需要 4字节(BlockHashLength) + 4字节(TxIndex)
		return nil, fmt.Errorf("数据长度不足")
	}

	// 读取区块哈希长度
	blockHashLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if blockHashLen < 0 || len(data) < 4+blockHashLen+4 {
		return nil, fmt.Errorf("数据格式错误")
	}

	// 读取区块哈希
	blockHash := make([]byte, blockHashLen)
	copy(blockHash, data[4:4+blockHashLen])

	// 读取交易索引
	offset := 4 + blockHashLen
	txIndex := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 | uint32(data[offset+3])

	return &TransactionLocation{
		BlockHash: blockHash,
		TxIndex:   txIndex,
	}, nil
}
