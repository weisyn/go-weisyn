// Package history 实现历史交易查询逻辑
//
// 📜 **历史交易查询 (Transaction History Query)**
//
// 本文件实现历史交易查询逻辑，支持按资源/UTXO查询所有相关交易。
//
// 🎯 **核心职责**：
// - 查询资源的历史交易（部署、引用、升级）
// - 查询UTXO的历史交易（引用、消费）
// - 支持分页和过滤
//
// ⚠️ **关键原则**：
// - 从历史索引中读取交易哈希列表
// - 通过交易索引获取交易详情
// - 从区块中解析交易完整信息
package history

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Service 历史交易查询服务
type Service struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewService 创建历史交易查询服务
func NewService(storage storage.BadgerStore, logger log.Logger) (*Service, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}

	return &Service{
		storage: storage,
		logger:  logger,
	}, nil
}

// GetResourceHistory 获取资源的历史交易列表
//
// 📋 **查询流程**：
// 1. 从资源历史索引中读取交易哈希列表
// 2. 解析交易哈希列表
// 3. 应用分页和过滤
// 4. 返回交易哈希列表
//
// ⚠️ **索引格式**：
// - 键：`indices:resource:history:{contentHash}`
// - 值：交易哈希列表（变长，每32字节一个交易哈希）+ 最后更新高度（8字节）
func (s *Service) GetResourceHistory(ctx context.Context, contentHash []byte, offset, limit int) ([]*TxHistoryEntry, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 1. 构建资源历史索引键
	historyKey := fmt.Sprintf("indices:resource:history:%x", contentHash)

	// 2. 读取历史索引值
	// 注意：BadgerStore.Get 在键不存在时返回 nil 值和 nil 错误
	indexData, err := s.storage.Get(ctx, []byte(historyKey))
	if err != nil {
		return nil, fmt.Errorf("读取资源历史索引失败: %w", err)
	}
	if indexData == nil || len(indexData) == 0 {
		// 索引不存在是正常情况（历史数据可能还未建立索引），返回空列表
		return []*TxHistoryEntry{}, nil
	}

	if len(indexData) < 8 {
		// 数据格式错误，返回空列表
		return []*TxHistoryEntry{}, nil
	}

	// 3. 解析交易哈希列表（排除最后8字节的高度信息）
	txHashesData := indexData[:len(indexData)-8]
	if len(txHashesData)%32 != 0 {
		// 数据格式错误，返回空列表
		return []*TxHistoryEntry{}, nil
	}

	// 4. 解析交易哈希列表
	txHashes := make([][]byte, 0, len(txHashesData)/32)
	for i := 0; i < len(txHashesData); i += 32 {
		txHash := make([]byte, 32)
		copy(txHash, txHashesData[i:i+32])
		txHashes = append(txHashes, txHash)
	}

	// 5. 应用分页
	start := offset
	end := offset + limit
	if start > len(txHashes) {
		return []*TxHistoryEntry{}, nil
	}
	if end > len(txHashes) {
		end = len(txHashes)
	}

	// 6. 构建交易历史条目
	entries := make([]*TxHistoryEntry, 0, end-start)
	for i := start; i < end; i++ {
		entries = append(entries, &TxHistoryEntry{
			TxHash: txHashes[i],
		})
	}

	return entries, nil
}

// GetUTXOHistory 获取UTXO的历史交易列表
//
// 📋 **查询流程**：
// 1. 从UTXO历史索引中读取交易哈希列表
// 2. 解析交易哈希列表
// 3. 应用分页和过滤
// 4. 返回交易哈希列表
//
// ⚠️ **索引格式**：
// - 键：`indices:utxo:history:{txId}:{outputIndex}`
// - 值：交易哈希列表（变长，每32字节一个交易哈希）+ 最后更新高度（8字节）
func (s *Service) GetUTXOHistory(ctx context.Context, outpoint *transaction.OutPoint, offset, limit int) ([]*TxHistoryEntry, error) {
	if outpoint == nil || len(outpoint.TxId) != 32 {
		return nil, fmt.Errorf("无效的 OutPoint")
	}

	// 1. 构建UTXO历史索引键
	historyKey := fmt.Sprintf("indices:utxo:history:%x:%d", outpoint.TxId, outpoint.OutputIndex)

	// 2. 读取历史索引值
	indexData, err := s.storage.Get(ctx, []byte(historyKey))
	if err != nil {
		return nil, fmt.Errorf("读取UTXO历史索引失败: %w", err)
	}
	if indexData == nil || len(indexData) == 0 {
		// 索引不存在，返回空列表
		return []*TxHistoryEntry{}, nil
	}

	if len(indexData) < 8 {
		// 数据格式错误，返回空列表
		return []*TxHistoryEntry{}, nil
	}

	// 3. 解析交易哈希列表（排除最后8字节的高度信息）
	txHashesData := indexData[:len(indexData)-8]
	if len(txHashesData)%32 != 0 {
		// 数据格式错误，返回空列表
		return []*TxHistoryEntry{}, nil
	}

	// 4. 解析交易哈希列表
	txHashes := make([][]byte, 0, len(txHashesData)/32)
	for i := 0; i < len(txHashesData); i += 32 {
		txHash := make([]byte, 32)
		copy(txHash, txHashesData[i:i+32])
		txHashes = append(txHashes, txHash)
	}

	// 5. 应用分页
	start := offset
	end := offset + limit
	if start > len(txHashes) {
		return []*TxHistoryEntry{}, nil
	}
	if end > len(txHashes) {
		end = len(txHashes)
	}

	// 6. 构建交易历史条目
	entries := make([]*TxHistoryEntry, 0, end-start)
	for i := start; i < end; i++ {
		entries = append(entries, &TxHistoryEntry{
			TxHash: txHashes[i],
		})
	}

	return entries, nil
}

// GetResourceHistoryTotal 获取资源的历史交易总数
func (s *Service) GetResourceHistoryTotal(ctx context.Context, contentHash []byte) (int, error) {
	if len(contentHash) != 32 {
		return 0, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 构建资源历史索引键
	historyKey := fmt.Sprintf("indices:resource:history:%x", contentHash)

	// 读取历史索引值
	indexData, err := s.storage.Get(ctx, []byte(historyKey))
	if err != nil {
		return 0, fmt.Errorf("读取资源历史索引失败: %w", err)
	}
	if indexData == nil || len(indexData) == 0 {
		return 0, nil
	}

	if len(indexData) < 8 {
		return 0, nil
	}

	// 解析交易哈希列表（排除最后8字节的高度信息）
	txHashesData := indexData[:len(indexData)-8]
	if len(txHashesData)%32 != 0 {
		return 0, nil
	}

	return len(txHashesData) / 32, nil
}

// TxHistoryEntry 交易历史条目
type TxHistoryEntry struct {
	TxHash []byte // 交易哈希（32字节）
}

// GetLastUpdateHeight 从历史索引中获取最后更新的区块高度
func (s *Service) GetLastUpdateHeight(ctx context.Context, contentHash []byte) (uint64, error) {
	if len(contentHash) != 32 {
		return 0, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 构建资源历史索引键
	historyKey := fmt.Sprintf("indices:resource:history:%x", contentHash)

	// 读取历史索引值
	indexData, err := s.storage.Get(ctx, []byte(historyKey))
	if err != nil {
		return 0, fmt.Errorf("读取资源历史索引失败: %w", err)
	}
	if indexData == nil || len(indexData) == 0 {
		return 0, nil
	}

	if len(indexData) < 8 {
		return 0, nil
	}

	// 读取最后8字节的高度信息
	heightBytes := indexData[len(indexData)-8:]
	return binary.BigEndian.Uint64(heightBytes), nil
}

