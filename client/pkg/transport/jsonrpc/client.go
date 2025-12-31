// Package jsonrpc provides JSON-RPC client transport functionality.
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Client JSON-RPC 2.0 客户端
type Client struct {
	endpoint   string
	httpClient *http.Client
	idCounter  uint64
}

// Request JSON-RPC 请求
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      uint64        `json:"id"`
}

// Response JSON-RPC 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      uint64          `json:"id"`
}

// Error JSON-RPC 错误
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// NewClient 创建 JSON-RPC 客户端
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		idCounter: 0,
	}
}

// Call 发起 JSON-RPC 调用
func (c *Client) Call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	// 构建请求
	reqID := atomic.AddUint64(&c.idCounter, 1)
	reqBody := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      reqID,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		if err := httpResp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// 读取响应
	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// 解析响应
	var rpcResp Response
	if err := json.Unmarshal(respData, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	// 检查错误
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s (data: %s)", 
			rpcResp.Error.Code, rpcResp.Error.Message, rpcResp.Error.Data)
	}

	return rpcResp.Result, nil
}

// SetTimeout 设置超时时间
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// Ping 检查连接
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Call(ctx, "wes_chainId")
	return err
}

