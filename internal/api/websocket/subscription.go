package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"go.uber.org/zap"
)

// SubscriptionManager 订阅管理器
// 🔔 支持重组安全订阅和断线重连
// 特性：
// - removed字段标记重组移除的事件
// - reorgId标识重组事件
// - resumeToken支持断线重连
// - 事件历史存储支持事件回放
type SubscriptionManager struct {
	logger        *zap.Logger
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
	eventBus      event.EventBus
	eventStore    storage.BadgerStore // 事件历史存储（可选）
}

// Subscription 订阅信息
type Subscription struct {
	ID          string            // 订阅ID
	Type        string            // 订阅类型（newHeads, logs, newPendingTxs等）
	Filters     interface{}       // 过滤器
	Conn        *websocket.Conn   // WebSocket连接
	ResumeToken string            // 恢复令牌（用于断线重连）
	LastReorgID string            // 最后处理的重组ID
	Handler     func(interface{}) // 事件处理器函数（用于取消订阅）
}

// NewSubscriptionManager 创建订阅管理器
func NewSubscriptionManager(logger *zap.Logger, eventBus event.EventBus, eventStore storage.BadgerStore) *SubscriptionManager {
	return &SubscriptionManager{
		logger:        logger,
		subscriptions: make(map[string]*Subscription),
		eventBus:      eventBus,
		eventStore:    eventStore, // 可以为nil，表示不支持事件回放
	}
}

// Subscribe 创建新订阅
// 🔔 支持：
// - 断线重连（resumeToken）
// - 事件回放（从令牌恢复）
// - 自动重组检测
func (m *SubscriptionManager) Subscribe(ctx context.Context, conn *websocket.Conn, subType string, filters interface{}, resumeToken string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 步骤1: 生成订阅ID
	subscriptionID := fmt.Sprintf("0x%s", uuid.New().String()[:8])

	// 步骤2: 创建事件处理器（需要在订阅对象外创建以便保存引用）
	handler := func(data interface{}) {
		m.handleEventForSubscription(subscriptionID, data)
	}

	// 步骤3: 创建订阅对象
	subscription := &Subscription{
		ID:          subscriptionID,
		Type:        subType,
		Filters:     filters,
		Conn:        conn,
		ResumeToken: resumeToken,
		LastReorgID: "",      // 初始为空
		Handler:     handler, // 保存handler引用用于取消订阅
	}

	// 步骤4: 订阅EventBus事件
	eventType := mapSubscriptionTypeToEventType(subType)
	if eventType != "" && m.eventBus != nil {
		// 订阅EventBus（使用event.EventType类型）
		if err := m.eventBus.Subscribe(event.EventType(eventType), handler); err != nil {
			m.logger.Error("Failed to subscribe to event bus",
				zap.String("eventType", eventType),
				zap.Error(err))
			return "", fmt.Errorf("failed to subscribe to event bus: %w", err)
		}

		m.logger.Debug("Subscribed to event bus",
			zap.String("eventType", eventType),
			zap.String("subscriptionType", subType))
	}

	// 步骤5: 如果有resumeToken，重放缺失的事件
	if resumeToken != "" {
		m.logger.Info("Attempting to resume subscription",
			zap.String("id", subscriptionID),
			zap.String("resumeToken", resumeToken))

		// 解析resumeToken并尝试重放事件
		if err := m.replayMissedEvents(subscription, resumeToken); err != nil {
			m.logger.Warn("Failed to replay missed events",
				zap.String("subscriptionID", subscriptionID),
				zap.Error(err))
			// 不阻塞订阅创建，仅记录警告
		}
	}

	// 步骤6: 保存订阅信息
	m.subscriptions[subscriptionID] = subscription

	m.logger.Info("New subscription created",
		zap.String("id", subscriptionID),
		zap.String("type", subType),
		zap.Bool("resumed", resumeToken != ""),
		zap.String("remote_addr", conn.RemoteAddr().String()))

	return subscriptionID, nil
}

// Unsubscribe 取消订阅
func (m *SubscriptionManager) Unsubscribe(subscriptionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.subscriptions[subscriptionID]; !ok {
		return nil // 订阅不存在，静默成功
	}

	// 取消EventBus订阅
	subscription := m.subscriptions[subscriptionID]
	if subscription != nil {
		eventType := mapSubscriptionTypeToEventType(subscription.Type)
		if eventType != "" && m.eventBus != nil && subscription.Handler != nil {
			// 使用保存的handler引用取消订阅
			if err := m.eventBus.Unsubscribe(event.EventType(eventType), subscription.Handler); err != nil {
				m.logger.Warn("Failed to unsubscribe from event bus",
					zap.String("eventType", eventType),
					zap.Error(err))
			} else {
				m.logger.Debug("Unsubscribed from event bus",
					zap.String("eventType", eventType),
					zap.String("subscriptionID", subscriptionID))
			}
		}
	}

	delete(m.subscriptions, subscriptionID)

	m.logger.Info("Subscription cancelled", zap.String("id", subscriptionID))
	return nil
}

// CleanupByConnection 清理指定连接的所有订阅（修复内存泄漏）
// 🔧 用于WebSocket连接关闭时清理所有相关订阅
func (m *SubscriptionManager) CleanupByConnection(conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toRemove []string

	// 找出该连接的所有订阅
	for id, sub := range m.subscriptions {
		if sub.Conn == conn {
			toRemove = append(toRemove, id)
		}
	}

	if len(toRemove) == 0 {
		return // 没有订阅需要清理
	}

	m.logger.Info("清理WebSocket连接的订阅",
		zap.Int("subscription_count", len(toRemove)),
		zap.String("remote_addr", conn.RemoteAddr().String()))

	// 清理所有订阅
	for _, id := range toRemove {
		sub := m.subscriptions[id]

		// 取消EventBus订阅
		if sub.Handler != nil {
			eventType := mapSubscriptionTypeToEventType(sub.Type)
			if eventType != "" && m.eventBus != nil {
				if err := m.eventBus.Unsubscribe(event.EventType(eventType), sub.Handler); err != nil {
					m.logger.Warn("Failed to unsubscribe from event bus during cleanup",
						zap.String("eventType", eventType),
						zap.String("subscriptionID", id),
						zap.Error(err))
				}
			}
		}

		delete(m.subscriptions, id)
		m.logger.Debug("清理订阅", zap.String("id", id))
	}
}

// HandleReorg 处理链重组
// 🔄 重组安全推送：
// - removed=true: 标记被移除的区块/交易
// - reorgId: 唯一标识此次重组
// - resumeToken: 支持断线重连和事件重放
func (m *SubscriptionManager) HandleReorg(ctx context.Context, reorgID string, removedBlocks []string, newBlocks []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("Handling chain reorg",
		zap.String("reorgId", reorgID),
		zap.Int("removedBlocks", len(removedBlocks)),
		zap.Int("newBlocks", len(newBlocks)))

	// 步骤1: 向所有订阅者发送removed事件
	for _, subscription := range m.subscriptions {
		// 只处理newHeads订阅（其他类型订阅类似处理）
		if subscription.Type == "newHeads" {
			// 发送removed区块事件
			for _, removedBlockHash := range removedBlocks {
				removedEvent := map[string]interface{}{
					"removed": true,    // 🔴 标记为移除
					"reorgId": reorgID, // 🔄 重组ID
					"hash":    removedBlockHash,
					"reason":  "chain_reorganization",
				}

				if err := m.SendEvent(subscription.ID, removedEvent); err != nil {
					m.logger.Error("Failed to send removed event",
						zap.String("subscription_id", subscription.ID),
						zap.String("block_hash", removedBlockHash),
						zap.Error(err))
				}
			}

			// 步骤2: 发送新的规范区块事件
			for _, newBlockHash := range newBlocks {
				canonicalEvent := map[string]interface{}{
					"removed":     false,   // ✅ 规范区块
					"reorgId":     reorgID, // 🔄 同一个重组ID
					"hash":        newBlockHash,
					"resumeToken": generateResumeToken(reorgID, newBlockHash), // 🔖 恢复令牌
				}

				if err := m.SendEvent(subscription.ID, canonicalEvent); err != nil {
					m.logger.Error("Failed to send canonical event",
						zap.String("subscription_id", subscription.ID),
						zap.String("block_hash", newBlockHash),
						zap.Error(err))
				}
			}

			// 步骤3: 更新订阅的LastReorgID
			subscription.LastReorgID = reorgID
		}
	}

	m.logger.Info("Chain reorg handled successfully",
		zap.String("reorgId", reorgID),
		zap.Int("subscriptions_notified", len(m.subscriptions)))
}

// generateResumeToken 生成恢复令牌
// 格式: base64(reorgId:lastEventHash:timestamp:signature)
// 签名使用SHA256哈希确保令牌完整性
func generateResumeToken(reorgID string, lastEventHash string) string {
	// 使用真实时间戳（Unix时间戳，秒）
	timestamp := time.Now().Unix()

	// 构造令牌原始内容
	tokenContent := fmt.Sprintf("%s:%s:%d", reorgID, lastEventHash, timestamp)

	// 生成签名（使用SHA256哈希）
	// 注：生产环境应使用HMAC-SHA256配合密钥
	hash := sha256.Sum256([]byte(tokenContent))
	signature := fmt.Sprintf("%x", hash[:8]) // 使用前8字节作为简化签名

	// 完整令牌
	fullToken := fmt.Sprintf("%s:%s", tokenContent, signature)

	// Base64编码
	encodedToken := base64.StdEncoding.EncodeToString([]byte(fullToken))

	return encodedToken
}

// handleEventForSubscription 处理订阅的事件
func (m *SubscriptionManager) handleEventForSubscription(subscriptionID string, data interface{}) {
	m.mu.RLock()
	_, ok := m.subscriptions[subscriptionID]
	m.mu.RUnlock()

	if !ok {
		return // 订阅已取消
	}

	// 发送事件到客户端
	if err := m.SendEvent(subscriptionID, data); err != nil {
		m.logger.Error("Failed to send event to subscription",
			zap.String("subscriptionID", subscriptionID),
			zap.Error(err))
	}
}

// replayMissedEvents 重放缺失的事件
// 从BadgerStore中读取历史事件并重放
func (m *SubscriptionManager) replayMissedEvents(subscription *Subscription, resumeToken string) error {
	m.logger.Info("Replaying missed events",
		zap.String("subscriptionID", subscription.ID),
		zap.String("resumeToken", resumeToken))

	// 步骤1: 解析resumeToken
	decoded, err := base64.StdEncoding.DecodeString(resumeToken)
	if err != nil {
		return fmt.Errorf("invalid resume token format: %w", err)
	}

	// 格式: reorgId:lastEventHash:timestamp:signature
	parts := strings.Split(string(decoded), ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid resume token structure")
	}

	reorgID := parts[0]
	lastEventHash := parts[1]
	timestampStr := parts[2]
	expectedSig := parts[3]

	// 步骤2: 验证令牌签名
	tokenContent := fmt.Sprintf("%s:%s:%s", reorgID, lastEventHash, timestampStr)
	hash := sha256.Sum256([]byte(tokenContent))
	actualSig := fmt.Sprintf("%x", hash[:8])
	if actualSig != expectedSig {
		return fmt.Errorf("invalid resume token signature")
	}

	// 步骤3: 解析时间戳
	lastTimestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp in resume token: %w", err)
	}

	// 步骤4: 如果没有事件存储，无法回放
	if m.eventStore == nil {
		m.logger.Warn("Event store not available, skipping event replay")
		return nil
	}

	// 步骤5: 从存储中查询缺失的事件
	// 事件键格式: event:{eventType}:{timestamp}:{eventHash}
	eventType := mapSubscriptionTypeToEventType(subscription.Type)
	if eventType == "" {
		return nil // 无对应事件类型
	}

	// 扫描该类型的所有事件
	prefix := []byte(fmt.Sprintf("event:%s:", eventType))
	events, err := m.eventStore.PrefixScan(context.Background(), prefix)
	if err != nil {
		m.logger.Error("Failed to scan event history",
			zap.String("eventType", eventType),
			zap.Error(err))
		return fmt.Errorf("failed to scan event history: %w", err)
	}

	// 步骤6: 筛选时间戳之后的事件并排序
	type eventItem struct {
		timestamp int64
		data      []byte
	}
	missedEvents := make([]eventItem, 0)

	for key, value := range events {
		// 解析键获取时间戳: event:{eventType}:{timestamp}:{eventHash}
		keyParts := strings.Split(key, ":")
		if len(keyParts) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(keyParts[2], 10, 64)
		if err != nil {
			continue
		}
		// 只重放时间戳之后的事件
		if ts > lastTimestamp {
			missedEvents = append(missedEvents, eventItem{
				timestamp: ts,
				data:      value,
			})
		}
	}

	// 步骤7: 按时间戳排序并重放
	// 简化：这里假设扫描结果已按键排序
	m.logger.Info("Replaying missed events",
		zap.String("subscriptionID", subscription.ID),
		zap.Int("eventCount", len(missedEvents)))

	for _, evt := range missedEvents {
		// 反序列化事件数据
		var eventData interface{}
		if err := json.Unmarshal(evt.data, &eventData); err != nil {
			m.logger.Warn("Failed to unmarshal event data",
				zap.Error(err))
			continue
		}

		// 发送事件给订阅者
		if err := m.SendEvent(subscription.ID, eventData); err != nil {
			m.logger.Warn("Failed to replay event",
				zap.String("subscriptionID", subscription.ID),
				zap.Error(err))
			// 继续重放其他事件
		}
	}

	return nil
}

// SendEvent 向订阅者发送事件
// 事件格式符合JSON-RPC 2.0规范
func (m *SubscriptionManager) SendEvent(subscriptionID string, event interface{}) error {
	m.mu.RLock()
	subscription, ok := m.subscriptions[subscriptionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	// 构造JSON-RPC通知消息（WES命名）
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "wes_subscription",
		"params": map[string]interface{}{
			"subscription": subscriptionID,
			"result":       event,
		},
	}

	// 序列化并发送
	data, err := json.Marshal(notification)
	if err != nil {
		m.logger.Error("Failed to marshal event",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err))
		return err
	}

	// 通过WebSocket发送
	if err := subscription.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		m.logger.Error("Failed to send event to subscriber",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err))
		// 连接断开，清理订阅
		go m.Unsubscribe(subscriptionID)
		return err
	}

	m.logger.Debug("Event sent to subscriber",
		zap.String("subscription_id", subscriptionID),
		zap.Int("data_size", len(data)))

	return nil
}

// mapSubscriptionTypeToEventType 将订阅类型映射到EventBus事件类型
func mapSubscriptionTypeToEventType(subType string) string {
	mapping := map[string]string{
		"newHeads":      "NewBlock",
		"logs":          "NewLog",
		"newPendingTxs": "NewPendingTransaction",
		"syncing":       "SyncStateChanged",
	}
	return mapping[subType]
}
