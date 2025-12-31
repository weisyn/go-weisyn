package types

import "fmt"

// RPCError JSON-RPC错误
type RPCError struct {
	Code    int
	Message string
	Data    interface{}
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// 标准JSON-RPC 2.0错误码
const (
	// CodeParseError 解析错误
	CodeParseError = -32700
	// CodeInvalidRequest 无效请求
	CodeInvalidRequest = -32600
	// CodeMethodNotFound 方法不存在
	CodeMethodNotFound = -32601
	// CodeInvalidParams 无效参数
	CodeInvalidParams = -32602
	// CodeInternalError 内部错误
	CodeInternalError = -32603
)

// WES自定义错误码（-32000至-32099）
const (
	// CodeChainSyncing 链正在同步
	CodeChainSyncing = -32000
	// CodeBlockNotFound 区块不存在
	CodeBlockNotFound = -32001
	// CodeInvalidBlockParam 无效的区块参数
	CodeInvalidBlockParam = -32002
	// CodeTxFeeTooLow 交易费过低
	CodeTxFeeTooLow = -32003
	// CodeTxAlreadyKnown 交易已存在
	CodeTxAlreadyKnown = -32004
	// CodeTxConflicts 交易冲突
	CodeTxConflicts = -32005
	// CodeInvalidSignature 无效签名
	CodeInvalidSignature = -32006
	// CodeMempoolFull 内存池已满
	CodeMempoolFull = -32008
	// CodeChainReorganized 链重组
	CodeChainReorganized = -32010
)

// NewRPCError 创建RPC错误
func NewRPCError(code int, message string, data interface{}) *RPCError {
	return &RPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ErrParseError 创建解析错误
func ErrParseError(data interface{}) *RPCError {
	return NewRPCError(CodeParseError, "Parse error", data)
}

// ErrInvalidRequest 创建无效请求错误
func ErrInvalidRequest(data interface{}) *RPCError {
	return NewRPCError(CodeInvalidRequest, "Invalid Request", data)
}

// ErrMethodNotFound 创建方法未找到错误
func ErrMethodNotFound(method string) *RPCError {
	return NewRPCError(CodeMethodNotFound, "Method not found", method)
}

// ErrInvalidParams 创建无效参数错误
func ErrInvalidParams(data interface{}) *RPCError {
	return NewRPCError(CodeInvalidParams, "Invalid params", data)
}

// ErrInternalError 创建内部错误
func ErrInternalError(data interface{}) *RPCError {
	return NewRPCError(CodeInternalError, "Internal error", data)
}

// ErrChainSyncing 创建链同步错误
func ErrChainSyncing() *RPCError {
	return NewRPCError(CodeChainSyncing, "Node is syncing", nil)
}

// ErrBlockNotFound 创建区块未找到错误
func ErrBlockNotFound(heightOrHash interface{}) *RPCError {
	return NewRPCError(CodeBlockNotFound, "Block not found", heightOrHash)
}

// ErrTxFeeTooLow 创建交易费过低错误
func ErrTxFeeTooLow(details interface{}) *RPCError {
	return NewRPCError(CodeTxFeeTooLow, "Transaction fee too low", details)
}

// ErrInvalidSignature 创建无效签名错误
func ErrInvalidSignature(details interface{}) *RPCError {
	return NewRPCError(CodeInvalidSignature, "Invalid transaction signature", details)
}

// ErrTxAlreadyKnown 创建交易已存在错误
func ErrTxAlreadyKnown(txHash interface{}) *RPCError {
	return NewRPCError(CodeTxAlreadyKnown, "Transaction already known", txHash)
}

// ErrTxConflicts 创建交易冲突错误
func ErrTxConflicts(details interface{}) *RPCError {
	return NewRPCError(CodeTxConflicts, "Transaction conflicts with existing transaction", details)
}

// ErrMempoolFull 创建内存池已满错误
func ErrMempoolFull() *RPCError {
	return NewRPCError(CodeMempoolFull, "Mempool is full", map[string]interface{}{
		"hint": "Increase transaction fee or try again later",
	})
}
