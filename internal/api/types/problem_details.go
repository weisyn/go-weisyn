package types

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ProblemDetails WES Problem Details 结构（基于 RFC7807 + WES 扩展）
type ProblemDetails struct {
	// RFC7807 标准字段
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`

	// WES 扩展字段（必填）
	Code        string                 `json:"code"`
	Layer       string                 `json:"layer"`
	UserMessage string                 `json:"userMessage"`
	Details     map[string]interface{} `json:"details,omitempty"`
	TraceID     string                 `json:"traceId"`
	Timestamp   string                 `json:"timestamp"`
}

// Error 实现 error 接口
func (p *ProblemDetails) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.UserMessage
}

// WriteJSON 将 Problem Details 写入 HTTP 响应
func (p *ProblemDetails) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	json.NewEncoder(w).Encode(p)
}

// NewProblemDetails 创建新的 Problem Details
func NewProblemDetails(
	code string,
	layer string,
	userMessage string,
	detail string,
	status int,
	details map[string]interface{},
) *ProblemDetails {
	traceID := uuid.New().String()
	if details == nil {
		details = make(map[string]interface{})
	}

	return &ProblemDetails{
		Code:        code,
		Layer:       layer,
		UserMessage: userMessage,
		Detail:      detail,
		Status:      status,
		Details:     details,
		TraceID:     traceID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

// IsProblemDetails 检查错误是否为 Problem Details
func IsProblemDetails(err error) (*ProblemDetails, bool) {
	if pd, ok := err.(*ProblemDetails); ok {
		return pd, true
	}
	return nil, false
}

// 错误码常量（映射到 WES 规范）
const (
	// 区块链/交易错误
	CodeBCTxValidationFailed      = "BC_TX_VALIDATION_FAILED"
	CodeBCTxNotFound              = "BC_TX_NOT_FOUND"
	CodeBCBlockNotFound           = "BC_BLOCK_NOT_FOUND"
	CodeBCContractInvocationFailed = "BC_CONTRACT_INVOCATION_FAILED"
	CodeBCContractNotFound        = "BC_CONTRACT_NOT_FOUND"
	CodeBCInsufficientBalance     = "BC_INSUFFICIENT_BALANCE"

	// 通用错误
	CodeCommonValidationError = "COMMON_VALIDATION_ERROR"
	CodeCommonInternalError   = "COMMON_INTERNAL_ERROR"
	CodeCommonTimeout         = "COMMON_TIMEOUT"
	CodeCommonServiceUnavailable = "COMMON_SERVICE_UNAVAILABLE"
)

// Layer 常量
const (
	LayerBlockchainService = "blockchain-service"
)

