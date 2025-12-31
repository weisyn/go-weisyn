package methods

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/api/format"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// BlockMethods 区块查询相关方法
type BlockMethods struct {
	logger     *zap.Logger
	blockQuery persistence.BlockQuery
	bhCli      core.BlockHashServiceClient
	thCli      txpb.TransactionHashServiceClient
}

// NewBlockMethods 创建区块方法处理器
func NewBlockMethods(logger *zap.Logger, blockQuery persistence.BlockQuery, bhCli core.BlockHashServiceClient, thCli txpb.TransactionHashServiceClient) *BlockMethods {
	return &BlockMethods{
		logger:     logger,
		blockQuery: blockQuery,
		bhCli:      bhCli,
		thCli:      thCli,
	}
}

// GetBlockByHeight 按高度查询区块
// Method: wes_getBlockByHeight
// Params: [height: string (hex), fullTx: boolean]
// 返回：区块对象（含状态锚点字段）或null（区块不存在）
func (m *BlockMethods) GetBlockByHeight(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) < 1 {
		return nil, NewInvalidParamsError("height parameter required", nil)
	}

	// 解析高度参数（十六进制字符串或数字）
	var height uint64
	switch v := args[0].(type) {
	case string:
		// 移除0x前缀并解析
		heightStr := v
		if len(heightStr) > 2 && heightStr[:2] == "0x" {
			heightStr = heightStr[2:]
		}
		_, err := fmt.Sscanf(heightStr, "%x", &height)
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid height format: %v", err), nil)
		}
	case float64:
		height = uint64(v)
	default:
		return nil, NewInvalidParamsError("height must be string or number", nil)
	}

	// 解析fullTx参数（默认false）
	fullTx := false
	if len(args) > 1 {
		if v, ok := args[1].(bool); ok {
			fullTx = v
		}
	}

	// 从blockQuery查询区块
	block, err := m.blockQuery.GetBlockByHeight(ctx, height)
	if err != nil {
		m.logger.Error("Failed to get block by height",
			zap.Uint64("height", height),
			zap.Error(err))
		return nil, NewBlockNotFoundError(height)
	}

	if block == nil {
		return nil, nil // 区块不存在，返回null
	}

	// 转换为JSON-RPC响应格式
	resp, err := m.formatBlockResponse(ctx, block, fullTx)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockByHash 按哈希查询区块
// Method: wes_getBlockByHash
// Params: [hash: string, fullTx: boolean]
// 返回：区块对象（含状态锚点字段）或null（区块不存在）
func (m *BlockMethods) GetBlockByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) < 1 {
		return nil, NewInvalidParamsError("hash parameter required", nil)
	}

	// 解析哈希参数
	hashStr, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("hash must be a string", nil)
	}

	// 移除0x前缀并解码
	if len(hashStr) > 2 && hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}

	blockHash, err := hex.DecodeString(hashStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid hash format: %v", err), nil)
	}

	if len(blockHash) != 32 {
		return nil, NewInvalidParamsError("hash must be 32 bytes", nil)
	}

	// 解析fullTx参数（默认false）
	fullTx := false
	if len(args) > 1 {
		if v, ok := args[1].(bool); ok {
			fullTx = v
		}
	}

	// 从repository查询区块
	block, err := m.blockQuery.GetBlockByHash(ctx, blockHash)
	if err != nil {
		m.logger.Error("Failed to get block by hash",
			zap.String("hash", hex.EncodeToString(blockHash)),
			zap.Error(err))
		return nil, NewBlockNotFoundError(hashStr)
	}

	if block == nil {
		return nil, nil // 区块不存在，返回null
	}

	// 转换为JSON-RPC响应格式
	resp, err := m.formatBlockResponse(ctx, block, fullTx)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// formatBlockResponse 格式化区块响应（含状态锚点字段）
// fullTx: true返回带基本字段的交易对象，false返回交易哈希列表
func (m *BlockMethods) formatBlockResponse(ctx context.Context, block *core.Block, fullTx bool) (map[string]interface{}, error) {
	if block == nil || block.Header == nil {
		return nil, NewInternalError("invalid block data", nil)
	}

	if m.bhCli == nil {
		return nil, NewInternalError("block hash service not available", nil)
	}
	bhResp, err := m.bhCli.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: block})
	if err != nil || bhResp == nil || len(bhResp.Hash) == 0 {
		return nil, NewInternalError("failed to compute block hash", nil)
	}

	// 交易列表：根据 fullTx 参数返回不同格式
	// - fullTx=true: 返回 transactions ([]Transaction 对象)
	// - fullTx=false: 返回 tx_hashes ([]string 哈希列表)
	var txObjects []map[string]interface{} // fullTx=true 时的交易对象列表
	var txHashes []string                  // fullTx=false 时的交易哈希列表

	if block.Body != nil && len(block.Body.Transactions) > 0 {
		for _, tx := range block.Body.Transactions {
			if tx == nil {
				continue
			}
			if m.thCli == nil {
				return nil, NewInternalError("transaction hash service not available", nil)
			}
			hResp, err := m.thCli.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: tx})
			if err != nil || hResp == nil || len(hResp.Hash) == 0 {
				return nil, NewInternalError("failed to compute transaction hash", nil)
			}
			txHashHex := format.HashToHex(hResp.Hash)

			if fullTx {
				// 返回交易对象
				txObjects = append(txObjects, map[string]interface{}{
					"hash":   txHashHex,
					"from":   "",
					"to":     "",
					"value":  "0",
					"fee":    "0",
					"status": "confirmed",
				})
			} else {
				// 返回交易哈希
				txHashes = append(txHashes, txHashHex)
			}
		}
	}

	// 计算区块序列化大小（近似）
	size := 0
	if b, err := proto.Marshal(block); err == nil {
		size = len(b)
	}

	var stateRootHex interface{}
	if len(block.Header.StateRoot) > 0 {
		stateRootHex = format.HashToHex(block.Header.StateRoot)
	} else {
		stateRootHex = nil
	}

	resp := map[string]interface{}{
		"height":      block.Header.Height,
		"hash":        format.HashToHex(bhResp.Hash), // 客户端期望 "hash" 而非 "block_hash"
		"block_hash":  format.HashToHex(bhResp.Hash), // 保留兼容字段
		"parent_hash": format.HashToHex(block.Header.PreviousHash),
		"timestamp":   time.Unix(int64(block.Header.Timestamp), 0).UTC().Format(time.RFC3339), // RFC3339 格式
		"state_root":  stateRootHex,
		"difficulty":  fmt.Sprintf("%d", block.Header.Difficulty), // 客户端期望字符串
		"miner":       "",                                         // 客户端期望 miner 字段
		"size":        size,
	}

	// 根据 fullTx 参数返回不同字段
	if fullTx {
		resp["transactions"] = txObjects
		resp["tx_count"] = len(txObjects)
	} else {
		resp["tx_hashes"] = txHashes
		resp["tx_count"] = len(txHashes)
	}

	return resp, nil
}
