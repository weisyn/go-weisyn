package transport

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// JSONRPCClient JSON-RPC 2.0 客户端实现
type JSONRPCClient struct {
	endpoint   string
	httpClient *http.Client
	nextID     atomic.Uint64
}

// NewJSONRPCClient 创建JSON-RPC客户端
func NewJSONRPCClient(endpoint string, timeout time.Duration) *JSONRPCClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &JSONRPCClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// jsonrpcRequest JSON-RPC 2.0 请求
type jsonrpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      uint64        `json:"id"`
}

// jsonrpcResponse JSON-RPC 2.0 响应
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
	ID      uint64          `json:"id"`
}

// jsonrpcError JSON-RPC 2.0 错误
type jsonrpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// call 统一的JSON-RPC调用方法
func (c *JSONRPCClient) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	// 构建请求
	req := &jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID.Add(1),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// 解析响应
	var jsonResp jsonrpcResponse
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if jsonResp.Error != nil {
		return fmt.Errorf("jsonrpc error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	// 解析结果
	if result != nil && len(jsonResp.Result) > 0 {
		if err := json.Unmarshal(jsonResp.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}

	return nil
}

// ===== 接口实现 =====

func (c *JSONRPCClient) ChainID(ctx context.Context) (string, error) {
	var chainID string
	err := c.call(ctx, "wes_chainId", nil, &chainID)
	return chainID, err
}

func (c *JSONRPCClient) Syncing(ctx context.Context) (*SyncStatus, error) {
	var result interface{}
	if err := c.call(ctx, "wes_syncing", nil, &result); err != nil {
		return nil, err
	}

	// 如果返回false,表示未同步
	if isSyncing, ok := result.(bool); ok && !isSyncing {
		return &SyncStatus{Syncing: false}, nil
	}

	// 解析同步状态（先解析为 map 以便处理字段类型）
	var statusMap map[string]interface{}
	data, _ := json.Marshal(result)
	if err := json.Unmarshal(data, &statusMap); err != nil {
		return nil, fmt.Errorf("parse sync status: %w", err)
	}

	// 处理 uint64 字段
	if startingBlock, ok := parseUint64FromMap(statusMap, "starting_block"); ok {
		statusMap["starting_block"] = startingBlock
	}
	if currentBlock, ok := parseUint64FromMap(statusMap, "current_block"); ok {
		statusMap["current_block"] = currentBlock
	}
	if highestBlock, ok := parseUint64FromMap(statusMap, "highest_block"); ok {
		statusMap["highest_block"] = highestBlock
	}

	// 将 map 转换为 SyncStatus 结构体
	statusJSON, err := json.Marshal(statusMap)
	if err != nil {
		return nil, fmt.Errorf("marshal sync status: %w", err)
	}

	var status SyncStatus
	if err := json.Unmarshal(statusJSON, &status); err != nil {
		return nil, fmt.Errorf("unmarshal sync status: %w", err)
	}

	return &status, nil
}

func (c *JSONRPCClient) BlockNumber(ctx context.Context) (uint64, error) {
	var height string // JSON-RPC返回十六进制字符串
	if err := c.call(ctx, "wes_blockNumber", nil, &height); err != nil {
		return 0, err
	}

	var blockNum uint64
	if _, err := fmt.Sscanf(height, "0x%x", &blockNum); err != nil {
		return 0, fmt.Errorf("parse block number: %w", err)
	}

	return blockNum, nil
}

func (c *JSONRPCClient) GetBlockByHeight(ctx context.Context, height uint64, fullTx bool, anchor *StateAnchor) (*Block, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var blockMap map[string]interface{}

	// 构建参数
	params := []interface{}{
		fmt.Sprintf("0x%x", height),
		fullTx,
	}

	// 添加状态锚定参数
	if anchor != nil {
		anchorParam := make(map[string]interface{})
		if anchor.Height != nil {
			anchorParam["blockHeight"] = fmt.Sprintf("0x%x", *anchor.Height)
		}
		if anchor.Hash != nil {
			anchorParam["blockHash"] = *anchor.Hash
		}
		params = append(params, anchorParam)
	}

	err := c.call(ctx, "wes_getBlockByHeight", params, &blockMap)
	if err != nil {
		return nil, err
	}

	// 处理 timestamp 字段（API 返回 RFC3339 字符串）
	if ts, ok := parseTimeFromMap(blockMap, "timestamp"); ok {
		blockMap["timestamp"] = ts
	}

	// 处理 height 字段（可能是字符串）
	if blockHeight, ok := parseUint64FromMap(blockMap, "height"); ok {
		blockMap["height"] = blockHeight
	}

	// 处理 transactions 数组中的 nonce 字段（如果 fullTx=true）
	if transactions, ok := blockMap["transactions"].([]interface{}); ok {
		for _, tx := range transactions {
			if txMap, ok := tx.(map[string]interface{}); ok {
				if nonce, ok := parseUint64FromMap(txMap, "nonce"); ok {
					txMap["nonce"] = nonce
				}
			}
		}
	}

	// 将 map 转换为 Block 结构体
	blockJSON, err := json.Marshal(blockMap)
	if err != nil {
		return nil, fmt.Errorf("marshal block map: %w", err)
	}

	var block Block
	if err := json.Unmarshal(blockJSON, &block); err != nil {
		return nil, fmt.Errorf("unmarshal block: %w", err)
	}

	return &block, nil
}

func (c *JSONRPCClient) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (*Block, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var blockMap map[string]interface{}
	err := c.call(ctx, "wes_getBlockByHash", []interface{}{hash, fullTx}, &blockMap)
	if err != nil {
		return nil, err
	}

	// 处理 timestamp 字段
	if ts, ok := parseTimeFromMap(blockMap, "timestamp"); ok {
		blockMap["timestamp"] = ts
	}

	// 处理 height 字段
	if blockHeight, ok := parseUint64FromMap(blockMap, "height"); ok {
		blockMap["height"] = blockHeight
	}

	// 处理 transactions 数组中的 nonce 字段（如果 fullTx=true）
	if transactions, ok := blockMap["transactions"].([]interface{}); ok {
		for _, tx := range transactions {
			if txMap, ok := tx.(map[string]interface{}); ok {
				if nonce, ok := parseUint64FromMap(txMap, "nonce"); ok {
					txMap["nonce"] = nonce
				}
			}
		}
	}

	// 将 map 转换为 Block 结构体
	blockJSON, err := json.Marshal(blockMap)
	if err != nil {
		return nil, fmt.Errorf("marshal block map: %w", err)
	}

	var block Block
	if err := json.Unmarshal(blockJSON, &block); err != nil {
		return nil, fmt.Errorf("unmarshal block: %w", err)
	}

	return &block, nil
}

// SendTransaction 执行转账（调用节点的 wes_sendTransaction）
// 节点内部会完成：构建交易 → 签名交易 → 验证 → 提交到mempool
func (c *JSONRPCClient) SendTransaction(ctx context.Context, fromAddress string, toAddress string, amount uint64, privateKey []byte) (*SendTxResult, error) {
	params := map[string]interface{}{
		"fromAddress": fromAddress,
		"toAddress":   toAddress,
		"amount":      fmt.Sprintf("%d", amount),
		"privateKey":  "0x" + hex.EncodeToString(privateKey),
	}

	var result map[string]interface{}
	err := c.call(ctx, "wes_sendTransaction", []interface{}{params}, &result)
	if err != nil {
		return &SendTxResult{
			Accepted: false,
			Reason:   err.Error(),
		}, err
	}

	// 解析返回结果
	txHash, _ := result["txHash"].(string)
	accepted, _ := result["accepted"].(bool)
	reason, _ := result["reason"].(string)

	return &SendTxResult{
		TxHash:   txHash,
		Accepted: accepted,
		Reason:   reason,
	}, nil
}

func (c *JSONRPCClient) SendRawTransaction(ctx context.Context, signedTxHex string) (*SendTxResult, error) {
	var txHash string
	err := c.call(ctx, "wes_sendRawTransaction", []interface{}{signedTxHex}, &txHash)
	if err != nil {
		// 尝试解析拒绝原因
		return &SendTxResult{
			Accepted: false,
			Reason:   err.Error(),
		}, nil
	}

	return &SendTxResult{
		TxHash:   txHash,
		Accepted: true,
	}, nil
}

func (c *JSONRPCClient) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var txMap map[string]interface{}
	err := c.call(ctx, "wes_getTransactionByHash", []interface{}{txHash}, &txMap)
	if err != nil {
		return nil, err
	}

	// 手动转换 nonce 字段（从字符串转换为 uint64）
	if nonce, ok := parseUint64FromMap(txMap, "nonce"); ok {
		txMap["nonce"] = nonce
	}

	// 处理 timestamp 字段：API 可能返回 creation_timestamp（protobuf 字段名）或 timestamp
	if ts, ok := parseTimeFromMap(txMap, "creation_timestamp"); ok {
		txMap["timestamp"] = ts
	} else if ts, ok := parseTimeFromMap(txMap, "timestamp"); ok {
		txMap["timestamp"] = ts
	}

	// 处理 block_height 字段（可能是字符串）
	if blockHeight, ok := parseUint64FromMap(txMap, "block_height"); ok {
		txMap["block_height"] = blockHeight
	}

	// 处理 tx_index 字段
	if txIndex, ok := parseUint64FromMap(txMap, "tx_index"); ok {
		txMap["tx_index"] = uint32(txIndex)
	}

	// 处理 version 字段
	if version, ok := parseUint64FromMap(txMap, "version"); ok {
		txMap["version"] = uint32(version)
	}

	// 构建 Transaction 结构体
	tx := &Transaction{
		RawData: txMap, // 保存原始数据用于调试
	}

	// 解析基础字段
	if hash, ok := txMap["tx_hash"].(string); ok {
		tx.Hash = hash
	}
	if version, ok := txMap["version"].(uint32); ok {
		tx.Version = version
	} else if version, ok := txMap["version"].(float64); ok {
		tx.Version = uint32(version)
	}
	if nonce, ok := txMap["nonce"].(uint64); ok {
		tx.Nonce = nonce
	} else if nonce, ok := txMap["nonce"].(float64); ok {
		tx.Nonce = uint64(nonce)
	}
	if ts, ok := txMap["timestamp"].(time.Time); ok {
		tx.Timestamp = ts
	}
	if status, ok := txMap["status"].(string); ok {
		tx.Status = status
	}
	if blockHash, ok := txMap["block_hash"].(string); ok {
		tx.BlockHash = blockHash
	}
	if blockHeight, ok := txMap["block_height"].(uint64); ok {
		tx.BlockHeight = blockHeight
	} else if blockHeight, ok := txMap["block_height"].(float64); ok {
		tx.BlockHeight = uint64(blockHeight)
	}
	if txIndex, ok := txMap["tx_index"].(uint32); ok {
		tx.TxIndex = txIndex
	} else if txIndex, ok := txMap["tx_index"].(float64); ok {
		tx.TxIndex = uint32(txIndex)
	}
	if chainID, ok := txMap["chain_id"].(string); ok {
		tx.ChainID = chainID
	}

	// 解析 inputs
	if inputsRaw, ok := txMap["inputs"].([]interface{}); ok {
		tx.Inputs = parseInputs(inputsRaw)
	}

	// 解析 outputs
	if outputsRaw, ok := txMap["outputs"].([]interface{}); ok {
		tx.Outputs = parseOutputs(outputsRaw)
	}

	return tx, nil
}

// parseInputs 解析交易输入列表
func parseInputs(inputsRaw []interface{}) []TxInput {
	var inputs []TxInput
	for _, inputRaw := range inputsRaw {
		inputMap, ok := inputRaw.(map[string]interface{})
		if !ok {
			continue
		}

		input := TxInput{}

		// 解析 previous_output
		if prevOut, ok := inputMap["previous_output"].(map[string]interface{}); ok {
			input.PreviousOutput = &OutPoint{}
			if txID, ok := prevOut["tx_id"].(string); ok {
				input.PreviousOutput.TxID = txID
			}
			if idx, ok := prevOut["output_index"].(float64); ok {
				input.PreviousOutput.OutputIndex = uint32(idx)
			}
		}

		// 解析 is_reference_only
		if refOnly, ok := inputMap["is_reference_only"].(bool); ok {
			input.IsReferenceOnly = refOnly
		}

		// 解析 sequence
		if seq, ok := inputMap["sequence"].(float64); ok {
			input.Sequence = uint32(seq)
		}

		// 确定解锁证明类型
		input.UnlockingProofType = detectUnlockingProofType(inputMap)

		inputs = append(inputs, input)
	}
	return inputs
}

// parseOutputs 解析交易输出列表
func parseOutputs(outputsRaw []interface{}) []TxOutput {
	var outputs []TxOutput
	for _, outputRaw := range outputsRaw {
		outputMap, ok := outputRaw.(map[string]interface{})
		if !ok {
			continue
		}

		output := TxOutput{}

		// 解析 owner
		if owner, ok := outputMap["owner"].(string); ok {
			output.Owner = owner
		}

		// 解析 locking_conditions
		if conditions, ok := outputMap["locking_conditions"].([]interface{}); ok {
			output.LockingConditions = conditions
		}

		// 检测输出类型并解析对应内容
		if assetRaw, ok := outputMap["asset"].(map[string]interface{}); ok {
			output.OutputType = "asset"
			output.Asset = parseAssetOutput(assetRaw)
		} else if resourceRaw, ok := outputMap["resource"].(map[string]interface{}); ok {
			output.OutputType = "resource"
			output.Resource = parseResourceOutput(resourceRaw)
		} else if stateRaw, ok := outputMap["state"].(map[string]interface{}); ok {
			output.OutputType = "state"
			output.State = parseStateOutput(stateRaw)
		}

		outputs = append(outputs, output)
	}
	return outputs
}

// detectUnlockingProofType 检测解锁证明类型
func detectUnlockingProofType(inputMap map[string]interface{}) string {
	if _, ok := inputMap["single_key_proof"]; ok {
		return "single_key"
	}
	if _, ok := inputMap["multi_key_proof"]; ok {
		return "multi_key"
	}
	if _, ok := inputMap["execution_proof"]; ok {
		return "execution"
	}
	if _, ok := inputMap["delegation_proof"]; ok {
		return "delegation"
	}
	if _, ok := inputMap["threshold_proof"]; ok {
		return "threshold"
	}
	if _, ok := inputMap["time_proof"]; ok {
		return "time_lock"
	}
	if _, ok := inputMap["height_proof"]; ok {
		return "height_lock"
	}
	return "unknown"
}

// parseAssetOutput 解析资产输出
func parseAssetOutput(assetRaw map[string]interface{}) *AssetOutput {
	asset := &AssetOutput{}

	if nativeCoin, ok := assetRaw["native_coin"].(map[string]interface{}); ok {
		asset.NativeCoin = &NativeCoinAsset{}
		if amount, ok := nativeCoin["amount"].(string); ok {
			asset.NativeCoin.Amount = amount
		}
	}

	if contractToken, ok := assetRaw["contract_token"].(map[string]interface{}); ok {
		asset.ContractToken = &ContractTokenAsset{}
		if addr, ok := contractToken["contract_address"].(string); ok {
			asset.ContractToken.ContractAddress = addr
		}
		if amount, ok := contractToken["amount"].(string); ok {
			asset.ContractToken.Amount = amount
		}
	}

	return asset
}

// parseResourceOutput 解析资源输出
func parseResourceOutput(resourceRaw map[string]interface{}) *ResourceOutput {
	resource := &ResourceOutput{}

	// 解析嵌套的 resource 字段
	if innerResource, ok := resourceRaw["resource"].(map[string]interface{}); ok {
		if contentHash, ok := innerResource["content_hash"].(string); ok {
			resource.ContentHash = contentHash
		}
		if category, ok := innerResource["category"].(string); ok {
			resource.Category = category
		}
		if execType, ok := innerResource["executable_type"].(string); ok {
			resource.ExecutableType = execType
		}
		if mimeType, ok := innerResource["mime_type"].(string); ok {
			resource.MimeType = mimeType
		}
		if size, ok := innerResource["size"].(float64); ok {
			resource.Size = int64(size)
		}
	}

	if ts, ok := resourceRaw["creation_timestamp"].(float64); ok {
		resource.CreationTimestamp = uint64(ts)
	}
	if immutable, ok := resourceRaw["is_immutable"].(bool); ok {
		resource.IsImmutable = immutable
	}

	return resource
}

// parseStateOutput 解析状态输出
func parseStateOutput(stateRaw map[string]interface{}) *StateOutput {
	state := &StateOutput{}

	if stateID, ok := stateRaw["state_id"].(string); ok {
		state.StateID = stateID
	}
	if version, ok := stateRaw["state_version"].(float64); ok {
		state.StateVersion = uint64(version)
	}
	if execHash, ok := stateRaw["execution_result_hash"].(string); ok {
		state.ExecutionResultHash = execHash
	}
	if parentHash, ok := stateRaw["parent_state_hash"].(string); ok {
		state.ParentStateHash = parentHash
	}

	return state
}

func (c *JSONRPCClient) GetTransactionReceipt(ctx context.Context, txHash string) (*Receipt, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var receiptMap map[string]interface{}
	err := c.call(ctx, "wes_getTransactionReceipt", []interface{}{txHash}, &receiptMap)
	if err != nil {
		return nil, err
	}

	// 处理 block_height 字段
	if blockHeight, ok := parseUint64FromMap(receiptMap, "block_height"); ok {
		receiptMap["block_height"] = blockHeight
	}

	// 将 map 转换为 Receipt 结构体
	receiptJSON, err := json.Marshal(receiptMap)
	if err != nil {
		return nil, fmt.Errorf("marshal receipt map: %w", err)
	}

	var receipt Receipt
	if err := json.Unmarshal(receiptJSON, &receipt); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}

	return &receipt, nil
}

func (c *JSONRPCClient) GetTransactionHistory(ctx context.Context, txID string, resourceID string, limit int, offset int) ([]*Transaction, error) {
	// 构建参数
	filters := make(map[string]interface{})
	if txID != "" {
		filters["txId"] = txID
	}
	if resourceID != "" {
		filters["resourceId"] = resourceID
	}
	if limit > 0 {
		filters["limit"] = limit
	} else {
		filters["limit"] = 10 // 默认10条
	}
	if offset > 0 {
		filters["offset"] = offset
	} else {
		filters["offset"] = 0
	}

	params := []interface{}{map[string]interface{}{"filters": filters}}

	// 先解析为数组的 map
	var resultArray []map[string]interface{}
	err := c.call(ctx, "wes_getTransactionHistory", params, &resultArray)
	if err != nil {
		return nil, fmt.Errorf("wes_getTransactionHistory RPC调用失败: %w", err)
	}

	// 处理每个交易的字段类型转换
	transactions := make([]*Transaction, 0, len(resultArray))
	for _, txMap := range resultArray {
		// 处理 nonce 字段
		if nonce, ok := parseUint64FromMap(txMap, "nonce"); ok {
			txMap["nonce"] = nonce
		}

		// 处理 timestamp 字段
		if ts, ok := parseTimeFromMap(txMap, "timestamp"); ok {
			txMap["timestamp"] = ts
		} else if ts, ok := parseTimeFromMap(txMap, "creation_timestamp"); ok {
			txMap["timestamp"] = ts
		}

		// 处理 block_height 字段
		if blockHeight, ok := parseUint64FromMap(txMap, "block_height"); ok {
			txMap["block_height"] = blockHeight
		}

		// 将 map 转换为 Transaction 结构体
		txJSON, err := json.Marshal(txMap)
		if err != nil {
			continue // 跳过无法解析的交易
		}

		var tx Transaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			continue // 跳过无法解析的交易
		}

		transactions = append(transactions, &tx)
	}

	return transactions, nil
}

func (c *JSONRPCClient) EstimateFee(ctx context.Context, tx *UnsignedTx) (*FeeEstimate, error) {
	var estimate FeeEstimate
	err := c.call(ctx, "wes_estimateFee", []interface{}{tx}, &estimate)
	return &estimate, err
}

func (c *JSONRPCClient) GetBalance(ctx context.Context, address string, anchor *StateAnchor) (*Balance, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var balanceMap map[string]interface{}

	// 构建参数
	params := []interface{}{address}

	// 添加状态锚定参数
	if anchor != nil {
		anchorParam := make(map[string]interface{})
		if anchor.Height != nil {
			anchorParam["blockHeight"] = fmt.Sprintf("0x%x", *anchor.Height)
		}
		if anchor.Hash != nil {
			anchorParam["blockHash"] = *anchor.Hash
		}
		params = append(params, anchorParam)
	}

	err := c.call(ctx, "wes_getBalance", params, &balanceMap)
	if err != nil {
		return nil, err
	}

	// 处理 balance 字段（服务端返回 number，结构体字段为 string；这里强制转为字符串以保持兼容）
	if bal, ok := parseUint64FromMap(balanceMap, "balance"); ok {
		balanceMap["balance"] = fmt.Sprintf("%d", bal)
	}

	// 处理 height 字段
	if height, ok := parseUint64FromMap(balanceMap, "height"); ok {
		balanceMap["height"] = height
	}

	// 处理 timestamp 字段（API 返回 Unix 时间戳）
	if ts, ok := parseTimeFromMap(balanceMap, "timestamp"); ok {
		balanceMap["timestamp"] = ts
	}

	// 将 map 转换为 Balance 结构体
	balanceJSON, err := json.Marshal(balanceMap)
	if err != nil {
		return nil, fmt.Errorf("marshal balance map: %w", err)
	}

	var balance Balance
	if err := json.Unmarshal(balanceJSON, &balance); err != nil {
		return nil, fmt.Errorf("unmarshal balance: %w", err)
	}

	return &balance, nil
}

func (c *JSONRPCClient) GetContractTokenBalance(ctx context.Context, req *ContractTokenBalanceRequest) (*ContractTokenBalanceResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	params := map[string]interface{}{
		"address":      strings.TrimSpace(req.Address),
		"content_hash": strings.TrimPrefix(strings.TrimSpace(req.ContentHash), "0x"),
	}
	if req.TokenID != "" {
		params["token_id"] = req.TokenID
	}

	var result ContractTokenBalanceResult
	err := c.call(ctx, "wes_getContractTokenBalance", []interface{}{params}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *JSONRPCClient) GetUTXOs(ctx context.Context, address string, anchor *StateAnchor) ([]*UTXO, error) {
	// 构建参数
	params := []interface{}{address}

	// 添加状态锚定参数
	if anchor != nil {
		anchorParam := make(map[string]interface{})
		if anchor.Height != nil {
			anchorParam["blockHeight"] = fmt.Sprintf("0x%x", *anchor.Height)
		}
		if anchor.Hash != nil {
			anchorParam["blockHash"] = *anchor.Hash
		}
		params = append(params, anchorParam)
	}

	// 解析响应对象格式 {"utxos": [...], "height": "0x..."}
	var response map[string]interface{}
	err := c.call(ctx, "wes_getUTXO", params, &response)
	if err != nil {
		return nil, err
	}

	// 提取utxos数组
	utxosArray, ok := response["utxos"].([]interface{})
	if !ok {
		return []*UTXO{}, nil // 返回空列表而不是错误
	}

	// 转换为UTXO对象
	utxos := make([]*UTXO, 0, len(utxosArray))
	for i, item := range utxosArray {
		if utxoMap, ok := item.(map[string]interface{}); ok {
			utxo := &UTXO{}

			// 🔍 调试：打印原始UTXO数据
			if i == 0 {
				fmt.Printf("[GetUTXOs] UTXO[0] 原始数据: %+v\n", utxoMap)
			}

			// 解析outpoint (格式: "txhash:index"，例如：70364e0c...f50b:0）
			if outpoint, ok := utxoMap["outpoint"].(string); ok {
				// 使用 strings.Split 分割 txhash 和 index
				parts := strings.Split(outpoint, ":")
				if len(parts) == 2 {
					// 移除0x前缀（如果有）
					txHashStr := parts[0]
					if len(txHashStr) > 2 && txHashStr[:2] == "0x" {
						utxo.TxHash = txHashStr[2:]
					} else {
						utxo.TxHash = txHashStr
					}

					// 解析index
					if idx, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
						utxo.OutputIndex = uint32(idx)
					}

					if i == 0 {
						fmt.Printf("[GetUTXOs] ✅ UTXO[0] 解析成功: TxHash=%s, Index=%d\n", utxo.TxHash, utxo.OutputIndex)
					}
				} else {
					fmt.Printf("[GetUTXOs] ❌ UTXO[%d] outpoint格式错误: %s\n", i, outpoint)
				}
			} else {
				fmt.Printf("[GetUTXOs] ❌ UTXO[%d] 没有outpoint字段或类型错误\n", i)
			}

			// 解析amount（可能是uint64, float64, 或string）
			if amount, ok := utxoMap["amount"].(uint64); ok {
				utxo.Amount = fmt.Sprintf("%d", amount)
			} else if amountFloat, ok := utxoMap["amount"].(float64); ok {
				// JSON解析数字时可能变成float64
				utxo.Amount = fmt.Sprintf("%.0f", amountFloat)
			} else if amountStr, ok := utxoMap["amount"].(string); ok {
				// 如果是字符串（如 "0x123" 或纯数字），直接使用
				utxo.Amount = amountStr
			}

			utxos = append(utxos, utxo)
		}
	}

	return utxos, nil
}

func (c *JSONRPCClient) Call(ctx context.Context, call *CallRequest, anchor *StateAnchor) (*CallResult, error) {
	var result CallResult

	// 构建参数
	params := []interface{}{call}

	// 添加状态锚定参数
	if anchor != nil {
		anchorParam := make(map[string]interface{})
		if anchor.Height != nil {
			anchorParam["blockHeight"] = fmt.Sprintf("0x%x", *anchor.Height)
		}
		if anchor.Hash != nil {
			anchorParam["blockHash"] = *anchor.Hash
		}
		params = append(params, anchorParam)
	}

	err := c.call(ctx, "wes_call", params, &result)
	return &result, err
}

func (c *JSONRPCClient) TxPoolStatus(ctx context.Context) (*TxPoolStatus, error) {
	var status TxPoolStatus
	err := c.call(ctx, "wes_txpool_status", nil, &status)
	return &status, err
}

func (c *JSONRPCClient) TxPoolContent(ctx context.Context) (*TxPoolContent, error) {
	var content TxPoolContent
	err := c.call(ctx, "wes_txpool_content", nil, &content)
	return &content, err
}

func (c *JSONRPCClient) Subscribe(ctx context.Context, eventType SubscriptionType, filters map[string]interface{}, resumeToken string) (Subscription, error) {
	// JSON-RPC over HTTP 不支持订阅,需要使用WebSocket
	return nil, fmt.Errorf("subscription requires WebSocket client, use NewWebSocketClient")
}

func (c *JSONRPCClient) GetBlockHeader(ctx context.Context, height uint64) (*BlockHeader, error) {
	var header BlockHeader
	params := []interface{}{fmt.Sprintf("0x%x", height)}
	err := c.call(ctx, "wes_getBlockHeader", params, &header)
	return &header, err
}

func (c *JSONRPCClient) GetTxProof(ctx context.Context, txHash string) (*MerkleProof, error) {
	var proof MerkleProof
	err := c.call(ctx, "wes_getTxProof", []interface{}{txHash}, &proof)
	return &proof, err
}

func (c *JSONRPCClient) Ping(ctx context.Context) error {
	_, err := c.ChainID(ctx)
	return err
}

func (c *JSONRPCClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// CallRaw 调用任意 JSON-RPC 方法并返回原始结果
func (c *JSONRPCClient) CallRaw(ctx context.Context, method string, params []interface{}) (interface{}, error) {
	var result interface{}
	if err := c.call(ctx, method, params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ============================================================================
// 智能合约相关RPC方法
// ============================================================================

// DeployContract 部署智能合约
//
// 调用 wes_deployContract RPC，传递WASM内容（Base64编码）、私钥、合约元数据
func (c *JSONRPCClient) DeployContract(ctx context.Context, req *DeployContractRequest) (*DeployContractResult, error) {
	params := map[string]interface{}{
		"private_key":  req.PrivateKey,
		"wasm_content": req.WasmContentBase64,
		"abi_version":  req.AbiVersion,
		"name":         req.Name,
		"description":  req.Description,
	}

	var result struct {
		ContentHash string `json:"content_hash"`
		TxHash      string `json:"tx_hash"`
		Success     bool   `json:"success"`
		Message     string `json:"message"`
	}

	if err := c.call(ctx, "wes_deployContract", []interface{}{params}, &result); err != nil {
		return nil, fmt.Errorf("wes_deployContract RPC调用失败: %w", err)
	}

	return &DeployContractResult{
		ContentHash: result.ContentHash,
		TxHash:      result.TxHash,
		Success:     result.Success,
		Message:     result.Message,
	}, nil
}

// CallContract 调用智能合约
//
// 调用 wes_callContract RPC，执行合约方法
func (c *JSONRPCClient) CallContract(ctx context.Context, req *CallContractRequest) (*CallContractResult, error) {
	params := map[string]interface{}{
		"private_key":  req.PrivateKey,
		"content_hash": req.ContentHash,
		"method":       req.Method,
		"params":       req.Params,
		"payload":      req.PayloadBase64,
	}

	var result struct {
		TxHash     string                   `json:"tx_hash"`
		Results    []uint64                 `json:"results"`
		ReturnData string                   `json:"return_data"`
		Events     []map[string]interface{} `json:"events"`
		Success    bool                     `json:"success"`
		Message    string                   `json:"message"`
	}

	if err := c.call(ctx, "wes_callContract", []interface{}{params}, &result); err != nil {
		return nil, fmt.Errorf("wes_callContract RPC调用失败: %w", err)
	}

	return &CallContractResult{
		TxHash:     result.TxHash,
		Results:    result.Results,
		ReturnData: result.ReturnData,
		Events:     result.Events,
		Success:    result.Success,
		Message:    result.Message,
	}, nil
}

// GetContract 查询合约元数据
//
// 调用 wes_getContract RPC，获取合约信息
func (c *JSONRPCClient) GetContract(ctx context.Context, contentHash string) (*ContractMetadata, error) {
	params := map[string]interface{}{
		"content_hash": contentHash,
	}

	var result ContractMetadata

	if err := c.call(ctx, "wes_getContract", []interface{}{params}, &result); err != nil {
		return nil, fmt.Errorf("wes_getContract RPC调用失败: %w", err)
	}

	return &result, nil
}

// CallAIModel 调用AI模型
//
// 调用 wes_callAIModel RPC，执行AI模型推理
func (c *JSONRPCClient) CallAIModel(ctx context.Context, req *CallAIModelRequest) (*CallAIModelResult, error) {
	params := map[string]interface{}{
		"private_key": req.PrivateKey,
		"model_hash":  req.ModelHash,
		"inputs":      req.Inputs,
	}

	var result struct {
		TxHash        string         `json:"tx_hash"`
		TensorOutputs []TensorOutput `json:"tensor_outputs"`
		Success       bool           `json:"success"`
		Message       string         `json:"message"`
	}

	if err := c.call(ctx, "wes_callAIModel", []interface{}{params}, &result); err != nil {
		return nil, fmt.Errorf("wes_callAIModel RPC调用失败: %w", err)
	}

	return &CallAIModelResult{
		TxHash:        result.TxHash,
		TensorOutputs: result.TensorOutputs,
		Success:       result.Success,
		Message:       result.Message,
	}, nil
}

// DeployAIModel 部署AI模型
//
// 调用 wes_deployAIModel RPC，部署ONNX模型到区块链
func (c *JSONRPCClient) DeployAIModel(ctx context.Context, req *DeployAIModelRequest) (*DeployAIModelResult, error) {
	params := map[string]interface{}{
		"private_key":  req.PrivateKey,
		"onnx_content": req.OnnxContent,
		"name":         req.Name,
		"description":  req.Description,
	}

	var result struct {
		ContentHash string `json:"content_hash"`
		TxHash      string `json:"tx_hash"`
		Success     bool   `json:"success"`
		Message     string `json:"message"`
	}

	if err := c.call(ctx, "wes_deployAIModel", []interface{}{params}, &result); err != nil {
		return nil, fmt.Errorf("wes_deployAIModel RPC调用失败: %w", err)
	}

	return &DeployAIModelResult{
		ContentHash: result.ContentHash,
		TxHash:      result.TxHash,
		Success:     result.Success,
		Message:     result.Message,
	}, nil
}

// 确保实现了Client接口
var _ Client = (*JSONRPCClient)(nil)
