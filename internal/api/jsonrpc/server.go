package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/weisyn/v1/internal/api/jsonrpc/types"
	apitypes "github.com/weisyn/v1/internal/api/types"
	"go.uber.org/zap"
)

// Server JSON-RPC 2.0 服务器
type Server struct {
	logger  *zap.Logger
	methods map[string]MethodHandler
}

// MethodHandler JSON-RPC方法处理器
type MethodHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// JSON-RPC 2.0 标准错误码（规范约定）
const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603

	// -32000 ~ -32099 预留给实现方自定义 Server error
	jsonRPCServerError = -32000
)

// NewServer 创建JSON-RPC服务器
func NewServer(logger *zap.Logger) *Server {
	return &Server{
		logger:  logger,
		methods: make(map[string]MethodHandler),
	}
}

// RegisterMethod 注册JSON-RPC方法
func (s *Server) RegisterMethod(method string, handler MethodHandler) {
	s.methods[method] = handler
}

// ServeHTTP 处理HTTP请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ✅ 添加 panic recovery（带堆栈）
	defer func() {
		if rec := recover(); rec != nil {
			stack := debug.Stack()
			s.logger.Error("JSON-RPC handler panic recovered",
				zap.Any("panic", rec),
				zap.String("http_method", r.Method),
				zap.String("path", r.URL.Path),
				zap.ByteString("stack", stack),
			)

			// 如果还没有写任何响应，则返回 ProblemDetails 错误
			if w.Header().Get("Content-Type") == "" {
				problem := apitypes.NewProblemDetails(
					apitypes.CodeCommonInternalError,
					apitypes.LayerBlockchainService,
					"服务器内部错误，请稍后重试或联系管理员。",
					fmt.Sprintf("Panic recovered: %v", rec),
					500,
					map[string]interface{}{
						"panic": fmt.Sprintf("%v", rec),
					},
				)
				s.writeErrorWithProblemDetails(w, nil, problem, jsonRPCInternalError, "")
			}
		}
	}()

	if r.Method != http.MethodPost {
		problem := apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"请求方法无效，仅支持 POST 方法。",
			"Only POST method is allowed",
			405,
			nil,
		)
		s.writeErrorWithProblemDetails(w, nil, problem, jsonRPCInvalidRequest, "")
		return
	}

	var req types.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem := apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"请求格式无效，无法解析 JSON。",
			fmt.Sprintf("Parse error: %v", err),
			400,
			nil,
		)
		s.writeErrorWithProblemDetails(w, nil, problem, jsonRPCParseError, "")
		return
	}

	// 验证JSON-RPC版本
	if req.JSONRPC != "2.0" {
		problem := apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"请求格式无效，jsonrpc 字段必须为 '2.0'。",
			"jsonrpc field must be '2.0'",
			400,
			map[string]interface{}{
				"provided": req.JSONRPC,
			},
		)
		s.writeErrorWithProblemDetails(w, req.ID, problem, jsonRPCInvalidRequest, req.Method)
		return
	}

	// 查找方法处理器
	handler, ok := s.methods[req.Method]
	if !ok {
		problem := apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"方法不存在，请检查方法名称。",
			fmt.Sprintf("Method '%s' not found", req.Method),
			404,
			map[string]interface{}{
				"method": req.Method,
			},
		)
		s.writeErrorWithProblemDetails(w, req.ID, problem, jsonRPCMethodNotFound, req.Method)
		return
	}

	// 执行方法
	result, err := handler(r.Context(), req.Params)
	if err != nil {
		// 强制要求所有错误必须是 Problem Details
		problem, ok := apitypes.IsProblemDetails(err)
		if !ok {
			// 如果不是 Problem Details，记录错误并返回通用错误
			s.logger.Error("Handler returned non-ProblemDetails error",
				zap.String("method", req.Method),
				zap.Error(err))
			problem = apitypes.NewProblemDetails(
				apitypes.CodeCommonInternalError,
				apitypes.LayerBlockchainService,
				"服务器内部错误，请稍后重试或联系管理员。",
				fmt.Sprintf("Internal error: %v", err),
				500,
				map[string]interface{}{
					"method": req.Method,
				},
			)
		}
		// handler 层错误：保持 JSON-RPC 2.0 结构，使用实现方自定义 server error code
		s.writeErrorWithProblemDetails(w, req.ID, problem, jsonRPCServerError, req.Method)
		return
	}

	// 返回成功响应
	s.writeSuccess(w, req.ID, result)
}

// writeSuccess 写入成功响应
func (s *Server) writeSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := types.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("Failed to encode response", zap.Error(err))
		// 响应头已设置，无法返回错误，只能记录日志
	}
}

// writeErrorWithProblemDetails 写入包含 Problem Details 的错误响应
func (s *Server) writeErrorWithProblemDetails(w http.ResponseWriter, id interface{}, problem *apitypes.ProblemDetails, jsonrpcCode int, method string) {
	// ✅ 检查响应是否已写入（避免重复写入）
	if w.Header().Get("Content-Type") != "" {
		return
	}

	// 将 Problem Details 嵌入到 JSON-RPC 错误的 data 字段
	resp := types.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.ErrorResponse{
			Code:    jsonrpcCode,
			Message: problem.UserMessage,
			Data:    problem, // 嵌入完整的 Problem Details
		},
	}

	w.Header().Set("Content-Type", "application/json")
	// JSON-RPC 规范：即使发生错误，也应返回 200（错误信息在 body 中体现）
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("Failed to encode error response", zap.Error(err))
	}

	// 记录错误日志（包含 traceId）
	s.logger.Error("JSON-RPC error",
		zap.String("code", problem.Code),
		zap.String("traceId", problem.TraceID),
		zap.String("method", method),
		zap.Error(problem))
}
