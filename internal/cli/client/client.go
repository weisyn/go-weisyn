package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	blockpb "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Client API客户端，封装与HTTP API的交互
type Client struct {
	baseURL        string
	httpClient     *http.Client
	logger         log.Logger
	configProvider config.Provider
}

// NewClient 创建新的API客户端
func NewClient(logger log.Logger, configProvider config.Provider) *Client {
	// 从配置获取API地址
	baseURL := "http://localhost:8080/api/v1" // 默认值
	if configProvider != nil {
		apiConfig := configProvider.GetAPI()
		if apiConfig != nil {
			// 构建API基础URL
			host := apiConfig.HTTP.Host
			port := apiConfig.HTTP.Port

			// 如果host是0.0.0.0，对于客户端连接应该使用localhost
			if host == "0.0.0.0" {
				host = "localhost"
			}

			baseURL = fmt.Sprintf("http://%s:%d/api/v1", host, port)
		}
	}

	return &Client{
		baseURL:        baseURL,
		configProvider: configProvider,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// SetBaseURL 设置API基础地址
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// Get 执行GET请求
func (c *Client) Get(ctx context.Context, endpoint string) (*APIResponse, error) {
	return c.request(ctx, "GET", endpoint, nil)
}

// Post 执行POST请求
func (c *Client) Post(ctx context.Context, endpoint string, data interface{}) (*APIResponse, error) {
	return c.request(ctx, "POST", endpoint, data)
}

// request 执行HTTP请求的通用方法
func (c *Client) request(ctx context.Context, method, endpoint string, data interface{}) (*APIResponse, error) {
	url := c.baseURL + endpoint

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("序列化请求数据失败: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	c.logger.Info(fmt.Sprintf("发送API请求: method=%s, url=%s", method, url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	// 检查响应体是否为空
	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到API响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应数据失败: %w, response_body: %s", err, string(respBody))
	}

	if resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, apiResp.Error.Message)
	}

	return &apiResp, nil
}

// GetBalance 查询账户余额
func (c *Client) GetBalance(ctx context.Context, address string) (*BalanceInfo, error) {
	c.logger.Info(fmt.Sprintf("查询账户余额: address=%s, address_length=%d", address, len(address)))

	url := c.baseURL + fmt.Sprintf("/accounts/%s/balance", address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.logger.Info(fmt.Sprintf("发送API请求: method=GET, url=%s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到余额查询响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 解析余额响应格式 {"success": true, "data": {...}, "message": "..."}
	var balanceResp struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}

	if err := json.Unmarshal(respBody, &balanceResp); err != nil {
		return nil, fmt.Errorf("解析余额响应失败: %w, response_body: %s", err, string(respBody))
	}

	if !balanceResp.Success {
		return nil, fmt.Errorf("API返回失败状态: %s", balanceResp.Message)
	}

	if len(balanceResp.Data) == 0 {
		return nil, fmt.Errorf("API响应中的余额数据为空")
	}

	var balance BalanceInfo
	if err := json.Unmarshal(balanceResp.Data, &balance); err != nil {
		return nil, fmt.Errorf("解析余额数据失败: %w, raw_data: %s", err, string(balanceResp.Data))
	}

	return &balance, nil
}

// GetNodeInfo 获取节点信息
func (c *Client) GetNodeInfo(ctx context.Context) (*NodeInfo, error) {
	url := c.baseURL + "/node/info"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.logger.Info(fmt.Sprintf("发送API请求: method=GET, url=%s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到节点信息响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 直接解析节点信息（没有data包装）
	var nodeInfo NodeInfo
	if err := json.Unmarshal(respBody, &nodeInfo); err != nil {
		return nil, fmt.Errorf("解析节点信息失败: %w, response_body: %s", err, string(respBody))
	}

	return &nodeInfo, nil
}

// GetLatestBlock 获取最新区块
func (c *Client) GetLatestBlock(ctx context.Context) (*BlockInfo, error) {
	url := c.baseURL + "/blocks/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.logger.Info(fmt.Sprintf("发送API请求: method=GET, url=%s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	// 检查响应体是否为空
	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到API响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 解析JSON响应，其中包含protobuf编码的区块数据
	var blockResp struct {
		Success bool            `json:"success"`
		Block   json.RawMessage `json:"block"`
		Message string          `json:"message"`
	}

	if err := json.Unmarshal(respBody, &blockResp); err != nil {
		return nil, fmt.Errorf("解析区块响应失败: %w, response_body: %s", err, string(respBody))
	}

	if !blockResp.Success {
		return nil, fmt.Errorf("API返回失败状态: %s", blockResp.Message)
	}

	if len(blockResp.Block) == 0 {
		return nil, fmt.Errorf("API响应中的区块数据为空")
	}

	// 现在解析内部的区块数据，这应该是protobuf结构的JSON表示
	var protoBlock blockpb.Block
	if err := json.Unmarshal(blockResp.Block, &protoBlock); err != nil {
		return nil, fmt.Errorf("解析protobuf区块数据失败: %w, raw_data: %s", err, string(blockResp.Block))
	}

	// 创建BlockInfo包装器
	blockInfo := NewBlockInfoFromProto(&protoBlock)

	return blockInfo, nil
}

// GetMiningStatus 获取挖矿状态
func (c *Client) GetMiningStatus(ctx context.Context) (*MiningStatus, error) {
	url := c.baseURL + "/mining/status"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.logger.Info(fmt.Sprintf("发送API请求: method=GET, url=%s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到挖矿状态响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 直接解析挖矿状态（没有data包装）
	var status MiningStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("解析挖矿状态失败: %w, response_body: %s", err, string(respBody))
	}

	return &status, nil
}

// GetNodePeers 获取连接的节点列表
func (c *Client) GetNodePeers(ctx context.Context) ([]PeerInfo, error) {
	url := c.baseURL + "/node/peers"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.logger.Info(fmt.Sprintf("发送API请求: method=GET, url=%s", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("API响应为空")
	}

	c.logger.Info(fmt.Sprintf("收到节点连接列表响应: status=%d, body_length=%d", resp.StatusCode, len(respBody)))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 解析peer列表响应格式 {"success": true, "data": [...], "message": "..."}
	var peerResp struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}

	if err := json.Unmarshal(respBody, &peerResp); err != nil {
		return nil, fmt.Errorf("解析peer响应失败: %w, response_body: %s", err, string(respBody))
	}

	if !peerResp.Success {
		return nil, fmt.Errorf("API返回失败状态: %s", peerResp.Message)
	}

	if len(peerResp.Data) == 0 {
		return nil, fmt.Errorf("API响应中的peer数据为空")
	}

	var peers []PeerInfo
	if err := json.Unmarshal(peerResp.Data, &peers); err != nil {
		return nil, fmt.Errorf("解析peer数据失败: %w, raw_data: %s", err, string(peerResp.Data))
	}

	return peers, nil
}

// Transfer 执行转账
func (c *Client) Transfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	resp, err := c.Post(ctx, "/transactions/simple-transfer", req)
	if err != nil {
		return nil, err
	}

	var transfer TransferResponse
	if err := json.Unmarshal(resp.Data, &transfer); err != nil {
		return nil, fmt.Errorf("解析转账响应失败: %w", err)
	}

	return &transfer, nil
}
