// Package types provides type definitions for JSON-RPC API responses.
package types

// Response JSON-RPC 2.0 响应
type Response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      interface{}    `json:"id"`
	Result  interface{}    `json:"result,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse JSON-RPC 2.0 错误响应
type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
