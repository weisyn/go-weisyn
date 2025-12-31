// Package types provides JSON-RPC request type definitions.
package types

import "encoding/json"

// Request JSON-RPC 2.0 请求
type Request struct {
	JSONRPC string          `json:"jsonrpc"` // 必须是 "2.0"
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}
