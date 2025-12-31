package methods

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/weisyn/v1/internal/api/jsonrpc/types"
	"go.uber.org/zap"
)

// SubscribeMethods 订阅相关方法
// ⚠️ 订阅方法只能通过WebSocket使用，HTTP会返回错误
// 支持的订阅类型：
// - newHeads: 新区块头（含removed/reorgId/resumeToken）
// - logs: 合约日志（含removed标记）
// - newPendingTxs: 新待处理交易
// - syncing: 同步状态变化
type SubscribeMethods struct {
	logger              *zap.Logger
	subscriptionManager SubscriptionManager
}

// SubscriptionManager 订阅管理器接口
// 为了避免循环依赖，这里定义接口而不是直接导入websocket包
type SubscriptionManager interface {
	Subscribe(ctx context.Context, conn *websocket.Conn, subType string, filters interface{}, resumeToken string) (string, error)
	Unsubscribe(subscriptionID string) error
}

// NewSubscribeMethods 创建订阅方法处理器
func NewSubscribeMethods(logger *zap.Logger, subscriptionManager SubscriptionManager) *SubscribeMethods {
	return &SubscribeMethods{
		logger:              logger,
		subscriptionManager: subscriptionManager,
	}
}

// Subscribe 订阅事件
// Method: wes_subscribe
// Params: [subscriptionType: string, filters: object (optional), resumeToken: string (optional)]
// 订阅类型:
//   - "newHeads" - 新区块头（含重组安全：removed/reorgId/resumeToken）
//   - "newPendingTxs" - 新待处理交易
//   - "logs" - 合约日志（含removed标记）
//   - "syncing" - 同步状态变化
//
// ⚠️ 仅支持WebSocket连接
func (m *SubscribeMethods) Subscribe(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing subscription type", nil)
	}

	// 解析订阅类型
	subType, ok := args[0].(string)
	if !ok {
		return nil, NewInvalidParamsError("subscription type must be string", nil)
	}

	// 验证订阅类型
	validTypes := map[string]bool{
		"newHeads":      true,
		"logs":          true,
		"newPendingTxs": true,
		"syncing":       true,
	}
	if !validTypes[subType] {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid subscription type: %s", subType), nil)
	}

	// 解析过滤器（可选）
	var filters interface{}
	if len(args) > 1 {
		filters = args[1]
	}

	// 解析resumeToken（可选，用于断线重连）
	var resumeToken string
	if len(args) > 2 {
		if token, ok := args[2].(string); ok {
			resumeToken = token
		}
	}

	// 从context获取WebSocket连接
	conn, ok := ctx.Value("websocket_conn").(*websocket.Conn)
	if !ok || conn == nil {
		return nil, types.NewRPCError(-32000, "Subscriptions are only available over WebSocket", nil)
	}

	// 创建订阅
	subscriptionID, err := m.subscriptionManager.Subscribe(ctx, conn, subType, filters, resumeToken)
	if err != nil {
		m.logger.Error("Failed to create subscription",
			zap.String("type", subType),
			zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	m.logger.Info("Subscription created successfully",
		zap.String("id", subscriptionID),
		zap.String("type", subType),
		zap.Bool("resumed", resumeToken != ""),
		zap.String("remote_addr", conn.RemoteAddr().String()))

	return subscriptionID, nil
}

// Unsubscribe 取消订阅
// Method: wes_unsubscribe
// Params: [subscriptionId: string]
// 返回：true（成功）或false（订阅不存在）
func (m *SubscribeMethods) Unsubscribe(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing subscription id", nil)
	}

	subscriptionID := args[0]

	// 取消订阅
	if err := m.subscriptionManager.Unsubscribe(subscriptionID); err != nil {
		m.logger.Warn("Failed to unsubscribe",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err))
		return false, nil
	}

	m.logger.Info("Subscription cancelled successfully",
		zap.String("subscription_id", subscriptionID))

	return true, nil
}
