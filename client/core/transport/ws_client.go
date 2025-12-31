package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient WebSocket客户端实现(用于订阅)
type WebSocketClient struct {
	endpoint  string
	conn      *websocket.Conn
	mu        sync.RWMutex
	subs      map[string]*wsSubscription
	nextSubID uint64
	muSubID   sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
}

// NewWebSocketClient 创建WebSocket客户端
func NewWebSocketClient(endpoint string) (*WebSocketClient, error) {
	// 连接WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	// 关闭响应体（WebSocket握手响应）
	if resp != nil && resp.Body != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Failed to close WebSocket response body: %v", err)
			}
		}()
	}

	client := &WebSocketClient{
		endpoint: endpoint,
		conn:     conn,
		subs:     make(map[string]*wsSubscription),
		closeCh:  make(chan struct{}),
	}

	// 启动消息处理循环
	go client.readLoop()

	return client, nil
}

// wsMessage WebSocket消息
type wsMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      uint64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// wsSubscription WebSocket订阅
type wsSubscription struct {
	id          string
	eventType   SubscriptionType
	eventCh     chan *Event
	errCh       chan error
	unsubscribe func()
}

func (s *wsSubscription) Events() <-chan *Event {
	return s.eventCh
}

func (s *wsSubscription) Err() <-chan error {
	return s.errCh
}

func (s *wsSubscription) Unsubscribe() {
	s.unsubscribe()
}

// readLoop 消息读取循环
func (c *WebSocketClient) readLoop() {
	defer func() {
		c.mu.Lock()
		for _, sub := range c.subs {
			close(sub.eventCh)
			close(sub.errCh)
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.closeCh:
			return
		default:
		}

		var msg wsMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			// 连接关闭或错误
			c.mu.RLock()
			for _, sub := range c.subs {
				select {
				case sub.errCh <- fmt.Errorf("websocket read: %w", err):
				default:
				}
			}
			c.mu.RUnlock()
			return
		}

		// 处理订阅消息
		if msg.Method == "wes_subscription" {
			c.handleSubscriptionMessage(&msg)
		}
	}
}

// handleSubscriptionMessage 处理订阅消息
func (c *WebSocketClient) handleSubscriptionMessage(msg *wsMessage) {
	// 解析params
	var params struct {
		Subscription string          `json:"subscription"`
		Result       json.RawMessage `json:"result"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	// 查找订阅
	c.mu.RLock()
	sub, exists := c.subs[params.Subscription]
	c.mu.RUnlock()

	if !exists {
		return
	}

	// 解析事件数据
	var event Event
	if err := json.Unmarshal(params.Result, &event); err != nil {
		select {
		case sub.errCh <- fmt.Errorf("parse event: %w", err):
		default:
		}
		return
	}

	// 设置事件类型
	event.Type = sub.eventType

	// 发送事件
	select {
	case sub.eventCh <- &event:
	case <-c.closeCh:
	}
}

// Subscribe 订阅事件
func (c *WebSocketClient) Subscribe(ctx context.Context, eventType SubscriptionType, filters map[string]interface{}, resumeToken string) (Subscription, error) {
	c.muSubID.Lock()
	c.nextSubID++
	reqID := c.nextSubID
	c.muSubID.Unlock()

	// 构建订阅请求
	params := []interface{}{string(eventType)}
	if len(filters) > 0 {
		params = append(params, filters)
	}
	if resumeToken != "" {
		params = append(params, map[string]string{"resumeToken": resumeToken})
	}

	req := wsMessage{
		JSONRPC: "2.0",
		Method:  "wes_subscribe",
		Params:  mustMarshal(params),
		ID:      reqID,
	}

	// 发送订阅请求
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("send subscribe: %w", err)
	}

	// 等待订阅响应
	// 注意:这里简化处理,实际应该有超时和响应匹配逻辑
	// 假设订阅ID就是请求ID的字符串形式
	subID := fmt.Sprintf("0x%x", reqID)

	// 创建订阅对象
	sub := &wsSubscription{
		id:        subID,
		eventType: eventType,
		eventCh:   make(chan *Event, 100), // 缓冲100个事件
		errCh:     make(chan error, 10),
	}

	// 设置取消订阅函数
	sub.unsubscribe = func() {
		c.unsubscribe(subID)
	}

	// 注册订阅
	c.mu.Lock()
	c.subs[subID] = sub
	c.mu.Unlock()

	return sub, nil
}

// unsubscribe 取消订阅
func (c *WebSocketClient) unsubscribe(subID string) {
	// 移除订阅
	c.mu.Lock()
	sub, exists := c.subs[subID]
	delete(c.subs, subID)
	c.mu.Unlock()

	if !exists {
		return
	}

	// 发送取消订阅请求
	req := wsMessage{
		JSONRPC: "2.0",
		Method:  "wes_unsubscribe",
		Params:  mustMarshal([]interface{}{subID}),
		ID:      uint64(time.Now().UnixNano()),
	}

	_ = c.conn.WriteJSON(req) // 忽略错误

	// 关闭通道
	close(sub.eventCh)
	close(sub.errCh)
}

// Close 关闭WebSocket连接
func (c *WebSocketClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closeCh)

		// 取消所有订阅
		c.mu.Lock()
		subIDs := make([]string, 0, len(c.subs))
		for id := range c.subs {
			subIDs = append(subIDs, id)
		}
		c.mu.Unlock()

		for _, id := range subIDs {
			c.unsubscribe(id)
		}

		// 关闭连接
		err = c.conn.Close()
	})
	return err
}

// WebSocket客户端不实现完整的Client接口(仅用于订阅)
// 其他方法应该降级到JSON-RPC或REST客户端

// mustMarshal 序列化,panic on error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal: %v", err))
	}
	return data
}
