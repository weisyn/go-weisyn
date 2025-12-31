// Package tx 实现交易查询服务
package tx

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
	"google.golang.org/protobuf/proto"
)

// Service 交易查询服务
type Service struct {
	storage      storage.BadgerStore
	fileStore    storage.FileStore
	txHashClient transaction.TransactionHashServiceClient
	logger       log.Logger
	eventBus     eventiface.EventBus // 可选：发布corruption事件
}

// NewService 创建交易查询服务
func NewService(
	storage storage.BadgerStore,
	fileStore storage.FileStore,
	txHashClient transaction.TransactionHashServiceClient,
	eventBus eventiface.EventBus,
	logger log.Logger,
) (interfaces.InternalTxQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("fileStore 不能为空")
	}
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}

	s := &Service{
		storage:      storage,
		fileStore:    fileStore,
		txHashClient: txHashClient,
		eventBus:     eventBus,
		logger:       logger,
	}

	if logger != nil {
		logger.Info("✅ TxQuery 服务已创建")
	}

	return s, nil
}

func (s *Service) publishCorruptionDetected(phase types.CorruptionPhase, severity types.CorruptionSeverity, height *uint64, hashHex string, key string, err error) {
	if s.eventBus == nil || err == nil {
		return
	}
	data := types.CorruptionEventData{
		Component: types.CorruptionComponentPersistence,
		Phase:     phase,
		Severity:  severity,
		Height:    height,
		Hash:      hashHex,
		Key:       key,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        types.RFC3339Time(time.Now()),
	}
	s.eventBus.Publish(eventiface.EventTypeCorruptionDetected, context.Background(), data)
}

// GetTransaction 根据交易哈希获取完整交易及其位置信息
func (s *Service) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *transaction.Transaction, err error) {
	// 1. 根据交易哈希获取交易位置（遵循 data-architecture.md 规范）
	// 键格式：indices:tx:{txHash}
	txKey := []byte(fmt.Sprintf("indices:tx:%x", txHash))
	locationData, err := s.storage.Get(ctx, txKey)
	if err != nil {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityWarning, nil, fmt.Sprintf("%x", txHash), string(txKey), err)
		return nil, 0, nil, fmt.Errorf("获取交易位置失败: %w", err)
	}

	// 2. 解析位置数据（格式：blockHeight(8字节) + blockHash(32字节) + txIndex(4字节) = 44字节）
	// ✅ 修复 P0-2：支持44字节格式，正确解析高度、区块哈希、交易索引
	if len(locationData) < 44 {
		s.publishCorruptionDetected(types.CorruptionPhaseReadIndex, types.CorruptionSeverityCritical, nil, fmt.Sprintf("%x", txHash), string(txKey), fmt.Errorf("交易位置数据格式错误：期望至少44字节，实际%d字节", len(locationData)))
		return nil, 0, nil, fmt.Errorf("交易位置数据格式错误：期望至少44字节，实际%d字节", len(locationData))
	}
	// 读取高度（前8字节）- ✅ 修复：直接使用交易索引中的高度，而不是从区块哈希索引查询
	blockHeight := bytesToUint64(locationData[0:8])
	// 读取区块哈希（8-40字节）
	blockHash = locationData[8:40]
	// 读取交易索引（40-44字节）
	txIndex = binary.BigEndian.Uint32(locationData[40:44])

	// 3. 从高度索引获取区块文件路径（遵循 data-architecture.md 规范）
	// ⚠️ 修复：直接使用交易索引中的 blockHeight，而不是从区块哈希索引查询
	// 原因：交易索引中已经包含了正确的高度，从区块哈希索引查询可能导致不一致
	// 索引值格式：blockHash(32字节) + filePath长度(1字节) + filePath(N字节) + fileSize(8字节)
	heightKey := []byte(fmt.Sprintf("indices:height:%d", blockHeight))
	indexData, err := s.storage.Get(ctx, heightKey)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("获取区块索引失败: %w", err)
	}

	// 解析索引数据
	if len(indexData) < 33 {
		return nil, 0, nil, fmt.Errorf("区块索引数据格式错误")
	}
	filePathLen := int(indexData[32])
	if len(indexData) < 33+filePathLen+8 {
		return nil, 0, nil, fmt.Errorf("区块索引数据不完整")
	}
	filePath := string(indexData[33 : 33+filePathLen])

	// 🔧 调试日志：打印从索引读取的路径
	if s.logger != nil {
		s.logger.Infof("🔍 [区块查询] 从索引读取路径: blockHeight=%d, filePath=%s", blockHeight, filePath)
	}

	// 5. 从文件系统读取区块数据
	if s.fileStore == nil {
		return nil, 0, nil, fmt.Errorf("fileStore 未初始化")
	}
	blockData, err := s.fileStore.Load(ctx, filePath)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("读取区块文件失败 (%s): %w", filePath, err)
	}

	// 6. 反序列化区块
	block := &core.Block{}
	if err := proto.Unmarshal(blockData, block); err != nil {
		return nil, 0, nil, fmt.Errorf("反序列化区块失败: %w", err)
	}

	// 7. 从区块中提取交易
	if block.Body == nil {
		if s.logger != nil {
			s.logger.Errorf("❌ 区块Body为空，txHash=%x, blockHash=%x, txIndex=%d",
				txHash, blockHash, txIndex)
		}
		return nil, 0, nil, fmt.Errorf("区块Body为空")
	}

	if int(txIndex) >= len(block.Body.Transactions) {
		if s.logger != nil {
			s.logger.Errorf("❌ 交易索引超出范围，txHash=%x, blockHash=%x, txIndex=%d, 实际交易数=%d",
				txHash, blockHash, txIndex, len(block.Body.Transactions))
		}
		return nil, 0, nil, fmt.Errorf("交易索引超出范围")
	}

	tx = block.Body.Transactions[txIndex]
	return blockHash, txIndex, tx, nil
}

// GetTxBlockHeight 获取交易所在的区块高度（P1-1：优化为直接从交易索引读取）
func (s *Service) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	// ✅ 修复 P1-1：直接从交易索引值读取高度，无需查询区块
	// 键格式：indices:tx:{txHash}
	txKey := []byte(fmt.Sprintf("indices:tx:%x", txHash))
	locationData, err := s.storage.Get(ctx, txKey)
	if err != nil {
		return 0, fmt.Errorf("获取交易位置失败: %w", err)
	}

	// 解析位置数据（格式：blockHeight(8字节) + blockHash(32字节) + txIndex(4字节) = 44字节）
	if len(locationData) < 8 {
		return 0, fmt.Errorf("交易位置数据格式错误：期望至少8字节，实际%d字节", len(locationData))
	}

	// 直接读取高度（前8字节）
	height := bytesToUint64(locationData[0:8])
	return height, nil
}

// getHeightFromBlock 从区块数据中获取高度（后备方案）
//
// 当索引不存在时使用此方法作为后备
func (s *Service) getHeightFromBlock(ctx context.Context, blockHash []byte) (uint64, error) {
	// 读取区块数据
	blockKey := []byte(fmt.Sprintf("blocks:hash:%x", blockHash))
	blockData, err := s.storage.Get(ctx, blockKey)
	if err != nil {
		return 0, fmt.Errorf("获取区块数据失败: %w", err)
	}

	// 反序列化区块
	block := &core.Block{}
	if err := proto.Unmarshal(blockData, block); err != nil {
		return 0, fmt.Errorf("反序列化区块失败: %w", err)
	}

	return block.Header.Height, nil
}

// GetBlockTimestamp 获取指定高度的区块时间戳
func (s *Service) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	// ✅ 彻底迭代（不做向后兼容）：
	// indices:height:{height} 的值必须是新格式：
	//   blockHash(32字节) + filePathLen(1字节) + filePath(N字节) + fileSize(8字节)
	//
	// 直接从 FileStore 加载区块文件并解析 Header.Timestamp。
	if s.fileStore == nil {
		return 0, fmt.Errorf("fileStore 未初始化")
	}

	heightKey := []byte(fmt.Sprintf("indices:height:%d", height))
	indexData, err := s.storage.Get(ctx, heightKey)
	if err != nil {
		return 0, fmt.Errorf("获取区块索引失败: %w", err)
	}

	if len(indexData) < 33 {
		return 0, fmt.Errorf("区块索引数据格式错误：期望新格式（>=33字节），实际=%d", len(indexData))
	}

	filePathLen := int(indexData[32])
	if filePathLen <= 0 {
		return 0, fmt.Errorf("区块索引数据格式错误：filePathLen=%d", filePathLen)
	}
	if len(indexData) < 33+filePathLen+8 {
		return 0, fmt.Errorf("区块索引数据格式错误：长度不足，len=%d need=%d", len(indexData), 33+filePathLen+8)
	}

	filePath := string(indexData[33 : 33+filePathLen])
	fileSize := bytesToUint64(indexData[33+filePathLen : 33+filePathLen+8])

	blockData, err := s.fileStore.Load(ctx, filePath)
	if err != nil {
		return 0, fmt.Errorf("读取区块文件失败（路径=%s）: %w", filePath, err)
	}
	if fileSize > 0 && uint64(len(blockData)) != fileSize {
		return 0, fmt.Errorf("区块文件大小不匹配：索引=%d 实际=%d path=%s", fileSize, len(blockData), filePath)
	}

	block := &core.Block{}
	if err := proto.Unmarshal(blockData, block); err != nil {
		return 0, fmt.Errorf("反序列化区块失败: %w", err)
	}
	if block.Header == nil {
		return 0, fmt.Errorf("区块头为空: height=%d", height)
	}
	return int64(block.Header.Timestamp), nil
}

// 旧的 getBlockTimestampByHash（BadgerDB blocks:hash:%x）路径已删除：
// - 当前链路以“文件系统存储区块 + BadgerDB 仅存索引”为准
// - indices:height 的值必须为新索引格式（见 GetBlockTimestamp）

// GetAccountNonce 获取账户当前nonce
func (s *Service) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	// 从存储获取账户nonce（遵循 data-architecture.md 规范）
	// 键格式：indices:nonce:{address}
	nonceKey := []byte(fmt.Sprintf("indices:nonce:%x", address))
	nonceData, err := s.storage.Get(ctx, nonceKey)
	if err != nil {
		// 如果不存在，返回0（初始nonce）
		return 0, nil
	}

	return bytesToUint64(nonceData), nil
}

// GetTransactionsByBlock 获取区块中的所有交易
func (s *Service) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	// 根据哈希获取区块（遵循 data-architecture.md 规范）
	// 根据区块哈希获取区块数据
	// 注意：当前实现使用 BadgerDB 存储区块数据（blocks:hash:{hash}）
	// 根据 data-architecture.md，理想架构是：
	//   - 区块文件存储在 blocks/{segment}/{height}.bin
	//   - BadgerDB 存储索引：indices:height:{height} → {blockHash, fileOffset, fileSize}
	// 当前实现为简化版本，后续可优化为文件系统存储
	blockKey := []byte(fmt.Sprintf("blocks:hash:%x", blockHash))
	blockData, err := s.storage.Get(ctx, blockKey)
	if err != nil {
		return nil, fmt.Errorf("获取区块数据失败: %w", err)
	}

	// 反序列化区块
	block := &core.Block{}
	if err := proto.Unmarshal(blockData, block); err != nil {
		return nil, fmt.Errorf("反序列化区块失败: %w", err)
	}

	// 返回交易列表
	if block.Body == nil {
		return []*transaction.Transaction{}, nil
	}

	return block.Body.Transactions, nil
}

// bytesToUint64 将字节数组转换为uint64
func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// 编译时检查接口实现
var _ interfaces.InternalTxQuery = (*Service)(nil)
