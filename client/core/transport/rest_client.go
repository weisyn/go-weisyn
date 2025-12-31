package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RESTClient REST API 客户端实现(降级选项)
type RESTClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRESTClient 创建REST客户端
func NewRESTClient(baseURL string, timeout time.Duration) *RESTClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 确保baseURL以/api/v1结尾
	if !strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = strings.TrimRight(baseURL, "/") + "/api/v1"
	}

	return &RESTClient{
		baseURL: baseURL,
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

// get 发送GET请求
func (c *RESTClient) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	// 构建URL
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// post 发送POST请求
func (c *RESTClient) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	// 序列化请求体
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// ===== 接口实现 =====

func (c *RESTClient) ChainID(ctx context.Context) (string, error) {
	var result struct {
		ChainID string `json:"chain_id"`
	}
	err := c.get(ctx, "/chain/info", nil, &result)
	return result.ChainID, err
}

func (c *RESTClient) Syncing(ctx context.Context) (*SyncStatus, error) {
	var status SyncStatus
	err := c.get(ctx, "/chain/syncing", nil, &status)
	return &status, err
}

func (c *RESTClient) BlockNumber(ctx context.Context) (uint64, error) {
	var result struct {
		Height uint64 `json:"height"`
	}
	err := c.get(ctx, "/chain/head", nil, &result)
	return result.Height, err
}

func (c *RESTClient) GetBlockByHeight(ctx context.Context, height uint64, fullTx bool, anchor *StateAnchor) (*Block, error) {
	params := url.Values{}
	params.Set("full_tx", fmt.Sprintf("%t", fullTx))

	// 添加状态锚定参数
	if anchor != nil {
		if anchor.Height != nil {
			params.Set("at_height", fmt.Sprintf("%d", *anchor.Height))
		}
		if anchor.Hash != nil {
			params.Set("at_hash", *anchor.Hash)
		}
	}

	// 先解析为 map，以便手动处理字段类型转换
	var blockMap map[string]interface{}
	err := c.get(ctx, fmt.Sprintf("/blocks/%d", height), params, &blockMap)
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

func (c *RESTClient) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (*Block, error) {
	params := url.Values{}
	params.Set("full_tx", fmt.Sprintf("%t", fullTx))

	// 先解析为 map，以便手动处理字段类型转换
	var blockMap map[string]interface{}
	err := c.get(ctx, fmt.Sprintf("/blocks/hash/%s", hash), params, &blockMap)
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

func (c *RESTClient) SendRawTransaction(ctx context.Context, signedTxHex string) (*SendTxResult, error) {
	payload := map[string]string{
		"signed_tx": signedTxHex,
	}

	var result SendTxResult
	err := c.post(ctx, "/transactions", payload, &result)
	return &result, err
}

func (c *RESTClient) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var txMap map[string]interface{}
	err := c.get(ctx, fmt.Sprintf("/transactions/%s", txHash), nil, &txMap)
	if err != nil {
		return nil, err
	}

	// 手动转换 nonce 字段（从字符串转换为 uint64）
	if nonceVal, ok := txMap["nonce"]; ok {
		var nonce uint64
		switch v := nonceVal.(type) {
		case string:
			nonceStr := strings.TrimPrefix(v, "0x")
			parsed, err := strconv.ParseUint(nonceStr, 10, 64)
			if err != nil {
				parsed, err = strconv.ParseUint(nonceStr, 16, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid nonce format: %v", v)
				}
			}
			nonce = parsed
		case float64:
			nonce = uint64(v)
		case uint64:
			nonce = v
		default:
			nonce = 0
		}
		txMap["nonce"] = nonce
	}

	// 处理 timestamp 字段
	if tsVal, ok := txMap["creation_timestamp"]; ok {
		if _, hasTimestamp := txMap["timestamp"]; !hasTimestamp {
			txMap["timestamp"] = tsVal
		}
	}

	if tsVal, ok := txMap["timestamp"]; ok {
		switch v := tsVal.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				txMap["timestamp"] = t
			} else if tsInt, err := strconv.ParseInt(v, 10, 64); err == nil {
				txMap["timestamp"] = time.Unix(tsInt, 0)
			}
		case float64:
			txMap["timestamp"] = time.Unix(int64(v), 0)
		}
	}

	// 处理 block_height 字段
	if bhVal, ok := txMap["block_height"]; ok {
		switch v := bhVal.(type) {
		case string:
			if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
				txMap["block_height"] = parsed
			}
		case float64:
			txMap["block_height"] = uint64(v)
		}
	}

	// 将 map 转换为 Transaction 结构体
	txJSON, err := json.Marshal(txMap)
	if err != nil {
		return nil, fmt.Errorf("marshal tx map: %w", err)
	}

	var tx Transaction
	if err := json.Unmarshal(txJSON, &tx); err != nil {
		return nil, fmt.Errorf("unmarshal transaction: %w", err)
	}

	return &tx, nil
}

func (c *RESTClient) GetTransactionReceipt(ctx context.Context, txHash string) (*Receipt, error) {
	// 先解析为 map，以便手动处理字段类型转换
	var receiptMap map[string]interface{}
	err := c.get(ctx, fmt.Sprintf("/transactions/%s/receipt", txHash), nil, &receiptMap)
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

func (c *RESTClient) GetTransactionHistory(ctx context.Context, txID string, resourceID string, limit int, offset int) ([]*Transaction, error) {
	return nil, fmt.Errorf("GetTransactionHistory not supported by REST client, use JSON-RPC client instead")
}

func (c *RESTClient) EstimateFee(ctx context.Context, tx *UnsignedTx) (*FeeEstimate, error) {
	var estimate FeeEstimate
	err := c.post(ctx, "/transactions/estimate-fee", tx, &estimate)
	return &estimate, err
}

func (c *RESTClient) GetBalance(ctx context.Context, address string, anchor *StateAnchor) (*Balance, error) {
	params := url.Values{}

	// 添加状态锚定参数
	if anchor != nil {
		if anchor.Height != nil {
			params.Set("at_height", fmt.Sprintf("%d", *anchor.Height))
		}
		if anchor.Hash != nil {
			params.Set("at_hash", *anchor.Hash)
		}
	}

	// 先解析为 map，以便手动处理字段类型转换
	var balanceMap map[string]interface{}
	err := c.get(ctx, fmt.Sprintf("/accounts/%s/balance", address), params, &balanceMap)
	if err != nil {
		return nil, err
	}

	// 处理 balance 字段（保持与 transport.Balance 的 string 字段兼容）
	if bal, ok := parseUint64FromMap(balanceMap, "balance"); ok {
		balanceMap["balance"] = fmt.Sprintf("%d", bal)
	}

	// 处理 height 字段
	if height, ok := parseUint64FromMap(balanceMap, "height"); ok {
		balanceMap["height"] = height
	}

	// 处理 timestamp 字段
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

func (c *RESTClient) GetContractTokenBalance(ctx context.Context, req *ContractTokenBalanceRequest) (*ContractTokenBalanceResult, error) {
	return nil, fmt.Errorf("GetContractTokenBalance not supported by REST client, use JSON-RPC client instead")
}

func (c *RESTClient) GetUTXOs(ctx context.Context, address string, anchor *StateAnchor) ([]*UTXO, error) {
	params := url.Values{}

	// 添加状态锚定参数
	if anchor != nil {
		if anchor.Height != nil {
			params.Set("at_height", fmt.Sprintf("%d", *anchor.Height))
		}
		if anchor.Hash != nil {
			params.Set("at_hash", *anchor.Hash)
		}
	}

	var result struct {
		UTXOs []*UTXO `json:"utxos"`
	}
	err := c.get(ctx, fmt.Sprintf("/accounts/%s/utxos", address), params, &result)
	return result.UTXOs, err
}

func (c *RESTClient) Call(ctx context.Context, call *CallRequest, anchor *StateAnchor) (*CallResult, error) {
	// 将anchor添加到请求体
	payload := struct {
		*CallRequest
		AtHeight *uint64 `json:"at_height,omitempty"`
		AtHash   *string `json:"at_hash,omitempty"`
	}{
		CallRequest: call,
	}

	if anchor != nil {
		payload.AtHeight = anchor.Height
		payload.AtHash = anchor.Hash
	}

	var result CallResult
	err := c.post(ctx, "/call", payload, &result)
	return &result, err
}

func (c *RESTClient) TxPoolStatus(ctx context.Context) (*TxPoolStatus, error) {
	var status TxPoolStatus
	err := c.get(ctx, "/txpool/status", nil, &status)
	return &status, err
}

func (c *RESTClient) TxPoolContent(ctx context.Context) (*TxPoolContent, error) {
	var content TxPoolContent
	err := c.get(ctx, "/txpool/content", nil, &content)
	return &content, err
}

func (c *RESTClient) Subscribe(ctx context.Context, eventType SubscriptionType, filters map[string]interface{}, resumeToken string) (Subscription, error) {
	// REST不支持订阅,需要使用WebSocket
	return nil, fmt.Errorf("subscription requires WebSocket client, use NewWebSocketClient")
}

func (c *RESTClient) GetBlockHeader(ctx context.Context, height uint64) (*BlockHeader, error) {
	var header BlockHeader
	err := c.get(ctx, fmt.Sprintf("/spv/headers/%d", height), nil, &header)
	return &header, err
}

func (c *RESTClient) GetTxProof(ctx context.Context, txHash string) (*MerkleProof, error) {
	var proof MerkleProof
	err := c.get(ctx, fmt.Sprintf("/spv/tx/%s/proof", txHash), nil, &proof)
	return &proof, err
}

func (c *RESTClient) Ping(ctx context.Context) error {
	return c.get(ctx, "/health", nil, nil)
}

func (c *RESTClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// SendTransaction 执行转账（RESTClient暂不支持）
func (c *RESTClient) SendTransaction(ctx context.Context, fromAddress string, toAddress string, amount uint64, privateKey []byte) (*SendTxResult, error) {
	return nil, fmt.Errorf("SendTransaction not supported by REST client, use JSON-RPC client instead")
}

// DeployContract 部署智能合约（RESTClient暂不支持）
func (c *RESTClient) DeployContract(ctx context.Context, req *DeployContractRequest) (*DeployContractResult, error) {
	return nil, fmt.Errorf("DeployContract not supported by REST client, use JSON-RPC client instead")
}

// CallContract 调用智能合约（RESTClient暂不支持）
func (c *RESTClient) CallContract(ctx context.Context, req *CallContractRequest) (*CallContractResult, error) {
	return nil, fmt.Errorf("CallContract not supported by REST client, use JSON-RPC client instead")
}

// GetContract 查询合约元数据（RESTClient暂不支持）
func (c *RESTClient) GetContract(ctx context.Context, contentHash string) (*ContractMetadata, error) {
	return nil, fmt.Errorf("GetContract not supported by REST client, use JSON-RPC client instead")
}

// CallAIModel 调用AI模型（RESTClient暂不支持）
func (c *RESTClient) CallAIModel(ctx context.Context, req *CallAIModelRequest) (*CallAIModelResult, error) {
	return nil, fmt.Errorf("CallAIModel not supported by REST client, use JSON-RPC client instead")
}

// DeployAIModel 部署AI模型（RESTClient暂不支持）
func (c *RESTClient) DeployAIModel(ctx context.Context, req *DeployAIModelRequest) (*DeployAIModelResult, error) {
	return nil, fmt.Errorf("DeployAIModel not supported by REST client, use JSON-RPC client instead")
}

// CallRaw 在 RESTClient 上不支持，提示使用 JSON-RPC
func (c *RESTClient) CallRaw(ctx context.Context, method string, params []interface{}) (interface{}, error) {
	return nil, fmt.Errorf("CallRaw not supported by REST client, use JSON-RPC client instead")
}

// 确保实现了Client接口
var _ Client = (*RESTClient)(nil)
