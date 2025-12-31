package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/weisyn/v1/client/pkg/transport/jsonrpc"
)

// BlockchainAdapter 区块链服务适配器（通过 JSON-RPC 连接到节点）
type BlockchainAdapter struct {
	client *jsonrpc.Client
}

// NewBlockchainAdapter 创建区块链服务适配器
func NewBlockchainAdapter(client *jsonrpc.Client) *BlockchainAdapter {
	return &BlockchainAdapter{
		client: client,
	}
}

// ChainInfo 链信息
type ChainInfo struct {
	ChainID   uint64 `json:"chain_id"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"block_hash"`
	IsSyncing bool   `json:"is_syncing"`
	NetworkID string `json:"network_id"`
}

// BlockInfo 区块信息
type BlockInfo struct {
	Height       uint64   `json:"height"`
	Hash         string   `json:"hash"`
	ParentHash   string   `json:"parent_hash"`
	Timestamp    uint64   `json:"timestamp"`
	MerkleRoot   string   `json:"merkle_root"`
	StateRoot    string   `json:"state_root"`
	TxCount      int      `json:"tx_count"`
	Transactions []string `json:"transactions"` // 交易哈希列表
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	Hash        string `json:"hash"`
	BlockHash   string `json:"block_hash"`
	BlockHeight uint64 `json:"block_height"`
	Index       uint32 `json:"index"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Value       string `json:"value,omitempty"`
	Fee         string `json:"fee,omitempty"`
	Status      string `json:"status"`
}

// GetChainID 获取链ID
func (b *BlockchainAdapter) GetChainID(ctx context.Context) (uint64, error) {
	result, err := b.client.Call(ctx, "wes_chainId", nil)
	if err != nil {
		return 0, fmt.Errorf("calling wes_chainId: %w", err)
	}

	var chainIDHex string
	if err := json.Unmarshal(result, &chainIDHex); err != nil {
		return 0, fmt.Errorf("unmarshaling chainID: %w", err)
	}

	chainID, err := strconv.ParseUint(chainIDHex[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing chainID: %w", err)
	}

	return chainID, nil
}

// GetBlockNumber 获取当前区块高度
func (b *BlockchainAdapter) GetBlockNumber(ctx context.Context) (uint64, error) {
	result, err := b.client.Call(ctx, "wes_blockNumber", nil)
	if err != nil {
		return 0, fmt.Errorf("calling wes_blockNumber: %w", err)
	}

	var heightHex string
	if err := json.Unmarshal(result, &heightHex); err != nil {
		return 0, fmt.Errorf("unmarshaling height: %w", err)
	}

	height, err := strconv.ParseUint(heightHex[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing height: %w", err)
	}

	return height, nil
}

// GetBlockByHeight 通过高度获取区块
func (b *BlockchainAdapter) GetBlockByHeight(ctx context.Context, height uint64, fullTx bool) (*BlockInfo, error) {
	heightHex := fmt.Sprintf("0x%x", height)
	result, err := b.client.Call(ctx, "wes_getBlockByHeight", heightHex, fullTx)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getBlockByHeight: %w", err)
	}

	return b.parseBlockResponse(result)
}

// GetBlockByHash 通过哈希获取区块
func (b *BlockchainAdapter) GetBlockByHash(ctx context.Context, blockHash string, fullTx bool) (*BlockInfo, error) {
	result, err := b.client.Call(ctx, "wes_getBlockByHash", blockHash, fullTx)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getBlockByHash: %w", err)
	}

	return b.parseBlockResponse(result)
}

// GetTransactionByHash 获取交易详情
func (b *BlockchainAdapter) GetTransactionByHash(ctx context.Context, txHash string) (*TransactionInfo, error) {
	result, err := b.client.Call(ctx, "wes_getTransactionByHash", txHash)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getTransactionByHash: %w", err)
	}

	// 先解析为 map 以便处理字段类型转换
	var txMap map[string]interface{}
	if err := json.Unmarshal(result, &txMap); err != nil {
		return nil, fmt.Errorf("unmarshaling transaction map: %w", err)
	}

	// 处理 block_height 字段（可能是字符串）
	if blockHeight, ok := parseUint64FromMap(txMap, "block_height"); ok {
		txMap["block_height"] = blockHeight
	}

	// 处理 index 字段（可能是字符串）
	if index, ok := parseUint64FromMap(txMap, "index"); ok {
		txMap["index"] = uint32(index)
	} else if index, ok := parseUint64FromMap(txMap, "tx_index"); ok {
		txMap["index"] = uint32(index)
	}

	// 将 map 转换为结构体
	txJSON, err := json.Marshal(txMap)
	if err != nil {
		return nil, fmt.Errorf("marshal tx map: %w", err)
	}

	var txInfo TransactionInfo
	if err := json.Unmarshal(txJSON, &txInfo); err != nil {
		return nil, fmt.Errorf("unmarshaling transaction: %w", err)
	}

	return &txInfo, nil
}

// GetTransactionReceipt 获取交易收据
func (b *BlockchainAdapter) GetTransactionReceipt(ctx context.Context, txHash string) (map[string]interface{}, error) {
	result, err := b.client.Call(ctx, "wes_getTransactionReceipt", txHash)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getTransactionReceipt: %w", err)
	}

	var receipt map[string]interface{}
	if err := json.Unmarshal(result, &receipt); err != nil {
		return nil, fmt.Errorf("unmarshaling receipt: %w", err)
	}

	return receipt, nil
}

// IsSyncing 检查同步状态
func (b *BlockchainAdapter) IsSyncing(ctx context.Context) (bool, error) {
	result, err := b.client.Call(ctx, "wes_syncing", nil)
	if err != nil {
		return false, fmt.Errorf("calling wes_syncing: %w", err)
	}

	// 如果返回 false，表示未同步
	var syncing interface{}
	if err := json.Unmarshal(result, &syncing); err != nil {
		return false, fmt.Errorf("unmarshaling syncing: %w", err)
	}

	// 如果是 bool 且为 false，表示未同步
	if b, ok := syncing.(bool); ok && !b {
		return false, nil
	}

	// 如果是对象，表示正在同步
	return true, nil
}

// parseBlockResponse 解析区块响应
func (b *BlockchainAdapter) parseBlockResponse(result json.RawMessage) (*BlockInfo, error) {
	var response struct {
		Height       interface{} `json:"height"`
		Hash         string      `json:"hash"`
		ParentHash   string      `json:"parentHash"`
		Timestamp    interface{} `json:"timestamp"`
		MerkleRoot   string      `json:"merkleRoot"`
		StateRoot    string      `json:"stateRoot"`
		TxHashes     interface{} `json:"tx_hashes"`    // fullTx=false 时返回交易哈希列表
		Transactions interface{} `json:"transactions"` // fullTx=true 时返回完整交易
		TxCount      interface{} `json:"tx_count"`     // 交易数量
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("unmarshaling block: %w", err)
	}

	// 解析高度（可能是数字或字符串）
	height, err := parseFlexibleUint64(response.Height)
	if err != nil {
		return nil, fmt.Errorf("parsing height: %w", err)
	}

	// 解析时间戳（可能是 RFC3339 字符串、Unix 时间戳数字或十六进制字符串）
	timestamp, err := parseFlexibleTimestamp(response.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp: %w", err)
	}

	// 解析交易列表（优先使用 tx_hashes，其次使用 transactions）
	var txHashes []string
	if txs, ok := response.TxHashes.([]interface{}); ok && len(txs) > 0 {
		for _, tx := range txs {
			if txHash, ok := tx.(string); ok {
				txHashes = append(txHashes, txHash)
			}
		}
	} else if txs, ok := response.Transactions.([]interface{}); ok {
		for _, tx := range txs {
			if txHash, ok := tx.(string); ok {
				txHashes = append(txHashes, txHash)
			}
		}
	}

	// 解析交易数量
	txCount := len(txHashes)
	if count, err := parseFlexibleUint64(response.TxCount); err == nil && count > 0 {
		txCount = int(count)
	}

	return &BlockInfo{
		Height:       height,
		Hash:         response.Hash,
		ParentHash:   response.ParentHash,
		Timestamp:    timestamp,
		MerkleRoot:   response.MerkleRoot,
		StateRoot:    response.StateRoot,
		TxCount:      txCount,
		Transactions: txHashes,
	}, nil
}

// parseHexUint64 解析十六进制字符串为uint64
func parseHexUint64(hexStr string) (uint64, error) {
	if hexStr == "" {
		return 0, nil
	}
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	return strconv.ParseUint(hexStr, 16, 64)
}

// parseFlexibleUint64 灵活解析各种格式的 uint64 值
func parseFlexibleUint64(val interface{}) (uint64, error) {
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case float64:
		return uint64(v), nil
	case uint64:
		return v, nil
	case int64:
		return uint64(v), nil
	case string:
		// 移除 0x 前缀（如果有）
		valStr := strings.TrimPrefix(v, "0x")
		// 先尝试十进制解析
		if parsed, err := strconv.ParseUint(valStr, 10, 64); err == nil {
			return parsed, nil
		}
		// 再尝试十六进制解析
		return strconv.ParseUint(valStr, 16, 64)
	default:
		return 0, fmt.Errorf("unsupported type for uint64: %T", val)
	}
}

// parseFlexibleTimestamp 灵活解析各种格式的时间戳
func parseFlexibleTimestamp(val interface{}) (uint64, error) {
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case float64:
		// Unix 时间戳（秒）
		return uint64(v), nil
	case uint64:
		return v, nil
	case int64:
		return uint64(v), nil
	case string:
		// 尝试解析为 RFC3339 格式
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return uint64(t.Unix()), nil
		}
		// 尝试解析为 Unix 时间戳字符串
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			return uint64(ts), nil
		}
		// 尝试十六进制
		return parseHexUint64(v)
	default:
		return 0, fmt.Errorf("unsupported timestamp type: %T", val)
	}
}

// parseUint64FromMap 从 map 中解析 uint64 字段
func parseUint64FromMap(m map[string]interface{}, key string) (uint64, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	result, err := parseFlexibleUint64(val)
	if err != nil {
		return 0, false
	}
	return result, true
}

// GetBlockHash 获取指定高度的区块哈希
func (b *BlockchainAdapter) GetBlockHash(ctx context.Context, height uint64) (string, error) {
	heightHex := fmt.Sprintf("0x%x", height)
	result, err := b.client.Call(ctx, "wes_getBlockHash", heightHex)
	if err != nil {
		return "", fmt.Errorf("calling wes_getBlockHash: %w", err)
	}

	var blockHash string
	if err := json.Unmarshal(result, &blockHash); err != nil {
		return "", fmt.Errorf("unmarshaling block hash: %w", err)
	}

	return blockHash, nil
}

// GetChainInfo 获取链信息
func (b *BlockchainAdapter) GetChainInfo(ctx context.Context) (*ChainInfo, error) {
	// 获取链ID
	chainID, err := b.GetChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain id: %w", err)
	}

	// 获取当前高度
	height, err := b.GetBlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("get block number: %w", err)
	}

	// 获取最新区块哈希
	blockHash, err := b.GetBlockHash(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("get block hash: %w", err)
	}

	// 检查同步状态
	isSyncing, err := b.IsSyncing(ctx)
	if err != nil {
		return nil, fmt.Errorf("get syncing status: %w", err)
	}

	// 获取网络版本
	networkResult, err := b.client.Call(ctx, "net_version", nil)
	if err != nil {
		return nil, fmt.Errorf("get network version: %w", err)
	}

	var networkID string
	if err := json.Unmarshal(networkResult, &networkID); err != nil {
		networkID = "unknown"
	}

	return &ChainInfo{
		ChainID:   chainID,
		Height:    height,
		BlockHash: blockHash,
		IsSyncing: isSyncing,
		NetworkID: networkID,
	}, nil
}

// EstimateFee 估算交易费用
func (b *BlockchainAdapter) EstimateFee(ctx context.Context, txData map[string]interface{}) (uint64, error) {
	result, err := b.client.Call(ctx, "wes_estimateFee", txData)
	if err != nil {
		return 0, fmt.Errorf("calling wes_estimateFee: %w", err)
	}

	var feeHex string
	if err := json.Unmarshal(result, &feeHex); err != nil {
		return 0, fmt.Errorf("unmarshaling fee: %w", err)
	}

	fee, err := parseHexUint64(feeHex)
	if err != nil {
		return 0, fmt.Errorf("parsing fee: %w", err)
	}

	return fee, nil
}

// SendRawTransaction 发送原始交易
func (b *BlockchainAdapter) SendRawTransaction(ctx context.Context, signedTxHex string) (string, error) {
	result, err := b.client.Call(ctx, "wes_sendRawTransaction", signedTxHex)
	if err != nil {
		return "", fmt.Errorf("calling wes_sendRawTransaction: %w", err)
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("unmarshaling tx hash: %w", err)
	}

	return txHash, nil
}

// GetUTXOs 获取账户UTXO列表
func (b *BlockchainAdapter) GetUTXOs(ctx context.Context, addressHex string) ([]map[string]interface{}, error) {
	result, err := b.client.Call(ctx, "wes_getUTXO", addressHex)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getUTXO: %w", err)
	}

	var utxos []map[string]interface{}
	if err := json.Unmarshal(result, &utxos); err != nil {
		return nil, fmt.Errorf("unmarshaling UTXOs: %w", err)
	}

	return utxos, nil
}

// TxPoolStatus 交易池状态
type TxPoolStatus struct {
	Pending int `json:"pending"`
	Queued  int `json:"queued"`
}

// GetTxPoolStatus 获取交易池状态
func (b *BlockchainAdapter) GetTxPoolStatus(ctx context.Context) (*TxPoolStatus, error) {
	result, err := b.client.Call(ctx, "wes_txpool_status", nil)
	if err != nil {
		return nil, fmt.Errorf("calling wes_txpool_status: %w", err)
	}

	var status TxPoolStatus
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("unmarshaling txpool status: %w", err)
	}

	return &status, nil
}
