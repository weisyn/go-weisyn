// Package types provides HTTP response type definitions.
package types

// SuccessResponse 统一成功响应格式
type SuccessResponse struct {
	Data      interface{} `json:"data"`
	RequestID string      `json:"requestId,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Data: data,
	}
}

// WithRequestID 添加请求ID
func (r *SuccessResponse) WithRequestID(requestID string) *SuccessResponse {
	r.RequestID = requestID
	return r
}

// WithTimestamp 添加时间戳
func (r *SuccessResponse) WithTimestamp(timestamp string) *SuccessResponse {
	r.Timestamp = timestamp
	return r
}

// StateAnchorResponse 含状态锚点的响应
// 用于区块链查询接口，保证数据一致性
type StateAnchorResponse struct {
	Data      interface{} `json:"data"`
	Height    uint64      `json:"height"`    // ⭐ 查询时的区块高度
	Hash      string      `json:"hash"`      // ⭐ 查询时的区块哈希
	StateRoot string      `json:"stateRoot"` // ⭐ 状态根
	Timestamp int64       `json:"timestamp"` // 区块时间戳
	RequestID string      `json:"requestId,omitempty"`
}

// NewStateAnchorResponse 创建含状态锚点的响应
func NewStateAnchorResponse(data interface{}, height uint64, hash, stateRoot string, timestamp int64) *StateAnchorResponse {
	return &StateAnchorResponse{
		Data:      data,
		Height:    height,
		Hash:      hash,
		StateRoot: stateRoot,
		Timestamp: timestamp,
	}
}

// TxSubmitResponse 交易提交响应
type TxSubmitResponse struct {
	TxHash string `json:"txHash"`
	Status string `json:"status"` // pending, rejected
}

// TxSubmitSuccessResponse 交易提交成功响应
func TxSubmitSuccessResponse(txHash string) *TxSubmitResponse {
	return &TxSubmitResponse{
		TxHash: txHash,
		Status: "pending",
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status     string                 `json:"status"` // healthy, degraded, unhealthy
	Liveness   string                 `json:"liveness"`
	Readiness  string                 `json:"readiness"`
	Version    string                 `json:"version"`
	Uptime     string                 `json:"uptime"`
	Timestamp  string                 `json:"timestamp"`
	Components map[string]interface{} `json:"components"`
}
