package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/weisyn/v1/internal/api/jsonrpc/types"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"go.uber.org/zap"
)

// Server WebSocket服务器
// 🔌 支持JSON-RPC 2.0订阅与实时事件推送
type Server struct {
	logger              *zap.Logger
	subscriptionManager *SubscriptionManager
	upgrader            websocket.Upgrader
}

// NewServer 创建WebSocket服务器
// eventStore 参数可选，如果为nil则不支持事件回放
func NewServer(logger *zap.Logger, eventBus event.EventBus, eventStore storage.BadgerStore) *Server {
	return &Server{
		logger:              logger,
		subscriptionManager: NewSubscriptionManager(logger, eventBus, eventStore),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// 生产环境应严格检查Origin
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// HandleWebSocket 处理WebSocket连接（Gin Handler）
func (s *Server) HandleWebSocket(c *gin.Context) {
	// 升级HTTP连接为WebSocket
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade WebSocket connection",
			zap.Error(err))
		return
	}
	defer func() {
		// 🆕 修复内存泄漏：清理该连接的所有订阅
		s.subscriptionManager.CleanupByConnection(conn)
		
		if err := conn.Close(); err != nil {
			s.logger.Warn("关闭WebSocket连接失败", zap.Error(err))
		}
	}()

	s.logger.Info("WebSocket connection established",
		zap.String("remote_addr", conn.RemoteAddr().String()))

	// 处理JSON-RPC消息
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Warn("WebSocket connection closed unexpectedly",
					zap.Error(err))
			}
			break
		}

		if messageType != websocket.TextMessage {
			continue
		}

		// 解析JSON-RPC请求
		s.handleJSONRPCMessage(c.Request.Context(), conn, message)
	}

	s.logger.Info("WebSocket connection closed",
		zap.String("remote_addr", conn.RemoteAddr().String()))
}

// handleJSONRPCMessage 处理JSON-RPC消息
func (s *Server) handleJSONRPCMessage(ctx context.Context, conn *websocket.Conn, message []byte) {
	var request types.Request
	if err := json.Unmarshal(message, &request); err != nil {
		s.sendError(conn, nil, -32700, "Parse error", nil)
		return
	}

	// 路由到对应的方法
	switch request.Method {
	case "wes_subscribe":
		s.handleSubscribe(ctx, conn, &request)
	case "wes_unsubscribe":
		s.handleUnsubscribe(conn, &request)
	default:
		s.sendError(conn, request.ID, -32601, "Method not found", nil)
	}
}

// handleSubscribe 处理订阅请求
func (s *Server) handleSubscribe(ctx context.Context, conn *websocket.Conn, request *types.Request) {
	// 解析参数：[subscriptionType, filters (optional), resumeToken (optional)]
	var params []interface{}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		s.sendError(conn, request.ID, -32602, "Invalid params", nil)
		return
	}

	if len(params) == 0 {
		s.sendError(conn, request.ID, -32602, "Missing subscription type", nil)
		return
	}

	subType, ok := params[0].(string)
	if !ok {
		s.sendError(conn, request.ID, -32602, "Subscription type must be string", nil)
		return
	}

	// 提取过滤器和resumeToken（可选）
	var filters interface{}
	var resumeToken string
	if len(params) > 1 {
		filters = params[1]
	}
	if len(params) > 2 {
		if token, ok := params[2].(string); ok {
			resumeToken = token
		}
	}

	// 创建订阅
	subscriptionID, err := s.subscriptionManager.Subscribe(ctx, conn, subType, filters, resumeToken)
	if err != nil {
		s.logger.Error("Failed to create subscription",
			zap.String("type", subType),
			zap.Error(err))
		s.sendError(conn, request.ID, -32000, "Failed to subscribe", err.Error())
		return
	}

	// 返回订阅ID
	s.sendResult(conn, request.ID, subscriptionID)

	s.logger.Info("Subscription created",
		zap.String("id", subscriptionID),
		zap.String("type", subType))
}

// handleUnsubscribe 处理取消订阅请求
func (s *Server) handleUnsubscribe(conn *websocket.Conn, request *types.Request) {
	// 解析参数：[subscriptionID]
	var params []string
	if err := json.Unmarshal(request.Params, &params); err != nil {
		s.sendError(conn, request.ID, -32602, "Invalid params", nil)
		return
	}

	if len(params) == 0 {
		s.sendError(conn, request.ID, -32602, "Missing subscription ID", nil)
		return
	}

	subscriptionID := params[0]

	// 取消订阅
	if err := s.subscriptionManager.Unsubscribe(subscriptionID); err != nil {
		s.logger.Error("Failed to unsubscribe",
			zap.String("id", subscriptionID),
			zap.Error(err))
		s.sendError(conn, request.ID, -32000, "Failed to unsubscribe", err.Error())
		return
	}

	// 返回成功
	s.sendResult(conn, request.ID, true)

	s.logger.Info("Subscription cancelled",
		zap.String("id", subscriptionID))
}

// sendResult 发送JSON-RPC成功响应
func (s *Server) sendResult(conn *websocket.Conn, id interface{}, result interface{}) {
	response := types.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Failed to marshal response", zap.Error(err))
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.logger.Error("Failed to send response", zap.Error(err))
	}
}

// sendError 发送JSON-RPC错误响应
func (s *Server) sendError(conn *websocket.Conn, id interface{}, code int, message string, data interface{}) {
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if data != nil {
		errorObj["data"] = data
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorObj,
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Failed to marshal error response", zap.Error(err))
		return
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, responseData); err != nil {
		s.logger.Error("Failed to send error response", zap.Error(err))
	}
}

// RegisterRoutes 注册WebSocket路由到Gin
func (s *Server) RegisterRoutes(router *gin.Engine) {
	router.GET("/ws", s.HandleWebSocket)
	s.logger.Info("WebSocket server registered at /ws")
}

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (s *Server) ModuleName() string {
	return "api.websocket"
}

// CollectMemoryStats 收集 WebSocket API 模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 活跃 WebSocket / SSE 连接数
// - ApproxBytes: WebSocket 连接缓冲区估算 bytes
// - CacheItems: 订阅缓存条目数
// - QueueLength: 每连接的待发送队列长度（如果有）
func (s *Server) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 统计活跃的 WebSocket 连接和订阅数量
	s.subscriptionManager.mu.RLock()
	subscriptionCount := len(s.subscriptionManager.subscriptions)
	s.subscriptionManager.mu.RUnlock()

	// 估算连接数（当前实现：每个订阅对应一个连接；未来如支持多订阅共享连接，可单独统计连接集合）
	connCount := int64(subscriptionCount)
	objects := connCount

	// 根据内存监控模式决定是否计算 ApproxBytes
	var approxBytes int64 = 0
	mode := metricsutil.GetMemoryMonitoringMode()
	if mode == "accurate" {
		// accurate 模式：基于 Upgrader 的 ReadBufferSize/WriteBufferSize 估算每个连接的基础缓冲区占用
		// 这是与配置直接绑定的近似值，比拍脑袋的 KB/MB 常数更贴近实际。
		perConnBytes := int64(s.upgrader.ReadBufferSize + s.upgrader.WriteBufferSize)
		approxBytes = connCount * perConnBytes
	}

	// 缓存条目：订阅缓存条目数
	cacheItems := int64(subscriptionCount)

	// 队列长度：待发送消息队列长度（估算）
	queueLength := int64(0) // 简化估算

	return metricsiface.ModuleMemoryStats{
		Module:      "api.websocket",
		Layer:       "L2-Infrastructure",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: queueLength,
	}
}
