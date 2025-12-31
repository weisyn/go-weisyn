package examples

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// tx_propagation.go
// 交易传播示例：严格基于pb定义的交易传播实现
// 🎯 核心原则：完全使用pb定义，无任何自定义类型或兼容层

// ==================== 核心服务 ====================

// TxPropagationService 交易传播服务 - 严格基于pb协议
type TxPropagationService struct {
	network iface.Network
	logger  logiface.Logger

	// 严格使用pb定义的交易存储
	mu          sync.RWMutex
	mempool     map[string]*transactionpb.Transaction // 交易池：哈希 -> pb.Transaction
	peerTxCache map[peer.ID]map[string]bool           // peer已知交易缓存
}

// NewTxPropagationService 创建交易传播服务
func NewTxPropagationService(network iface.Network, logger logiface.Logger) *TxPropagationService {
	return &TxPropagationService{
		network:     network,
		logger:      logger,
		mempool:     make(map[string]*transactionpb.Transaction),
		peerTxCache: make(map[peer.ID]map[string]bool),
	}
}

// ==================== 协议定义 ====================
// 协议和主题常量已在protocols.go中统一定义

const (
	// 本地使用的协议别名
	ProtocolTxBroadcast = "/weisyn/tx/broadcast/v1.0.0"
	TopicTxAnnounce     = "weisyn.tx.announce.v1"
)

// ==================== 公共接口实现 ====================

// AddTransaction 添加交易到传播池
func (s *TxPropagationService) AddTransaction(tx *transactionpb.Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction cannot be nil")
	}

	// 计算交易哈希作为唯一标识
	txHash := s.computeTransactionHash(tx)

	s.mu.Lock()
	s.mempool[txHash] = tx
	s.mu.Unlock()

	// 广播交易
	return s.broadcastTransaction(tx, txHash)
}

// broadcastTransaction 广播交易到网络
func (s *TxPropagationService) broadcastTransaction(tx *transactionpb.Transaction, txHash string) error {
	// 序列化交易为pb格式
	txData, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// 使用transport.Envelope包装
	envelope := &transportpb.Envelope{
		Version:       1,
		Topic:         TopicTxAnnounce,
		ContentType:   "application/x-protobuf",
		Encoding:      "pb",
		Compression:   "none",
		Payload:       txData,
		CorrelationId: txHash,
		Timestamp:     uint64(time.Now().UnixMilli()),
	}

	envelopeData, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	// 发布到topic
	ctx := context.Background()
	return s.network.Publish(ctx, TopicTxAnnounce, envelopeData, nil)
}

// ==================== 协议处理器 ====================

// RegisterHandlers 注册协议处理器
func (s *TxPropagationService) RegisterHandlers() error {
	// 注册交易请求处理器
	if err := s.network.RegisterStreamHandler(ProtocolTxRequest, s.handleTxRequest); err != nil {
		return fmt.Errorf("failed to register tx request handler: %w", err)
	}

	// 订阅交易公告
	_, err := s.network.Subscribe(TopicTxAnnounce, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
		return s.handleTxAnnouncement(topic, data, from)
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to subscribe to tx announcements: %w", err)
	}

	return nil
}

// handleTxRequest 处理交易请求 - 严格基于pb协议
func (s *TxPropagationService) handleTxRequest(ctx context.Context, from peer.ID, reqData []byte) ([]byte, error) {
	s.logger.Debugf("received tx request", "from", from.String())

	// 🚨 关键问题：缺少pb定义
	// 当前pb/network/protocol/中缺少TxRequest消息定义
	// 这里暴露了架构问题：需要完善pb协议定义

	// 临时方案：解析为Envelope，从中提取交易哈希列表
	var envelope transportpb.Envelope
	if err := proto.Unmarshal(reqData, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request envelope: %w", err)
	}

	// ⚠️ 设计缺陷暴露：需要定义专门的TxRequest消息类型
	// 目前暂时假设payload是交易哈希列表的简单编码

	return s.buildTxResponse(from, envelope.Payload)
}

// buildTxResponse 构建交易响应
func (s *TxPropagationService) buildTxResponse(from peer.ID, hashData []byte) ([]byte, error) {
	// 获取请求的交易
	s.mu.RLock()
	var responseTransactions []*transactionpb.Transaction
	// 简化处理：返回所有交易（实际应解析具体的哈希请求）
	for _, tx := range s.mempool {
		responseTransactions = append(responseTransactions, tx)
	}
	s.mu.RUnlock()

	// 构建响应Envelope
	var responseData []byte
	if len(responseTransactions) > 0 {
		// 序列化交易列表
		// ⚠️ 设计问题：需要定义TxResponse消息类型包含交易列表
		// 目前简化处理，序列化第一个交易
		var err error
		responseData, err = proto.Marshal(responseTransactions[0])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}
	}

	envelope := &transportpb.Envelope{
		Version:     1,
		ContentType: "application/x-protobuf",
		Encoding:    "pb",
		Compression: "none",
		Payload:     responseData,
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	return proto.Marshal(envelope)
}

// handleTxAnnouncement 处理交易公告
func (s *TxPropagationService) handleTxAnnouncement(topic string, data []byte, from peer.ID) error {
	s.logger.Debugf("received tx announcement", "topic", topic, "from", from.String())

	// 解析Envelope
	var envelope transportpb.Envelope
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal announcement: %w", err)
	}

	// 从payload中解析交易
	var tx transactionpb.Transaction
	if err := proto.Unmarshal(envelope.Payload, &tx); err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// 验证并存储交易
	txHash := s.computeTransactionHash(&tx)

	s.mu.Lock()
	if _, exists := s.mempool[txHash]; !exists {
		s.mempool[txHash] = &tx
		s.markPeerKnowsTx(from, txHash)
		s.logger.Infof("received new transaction", "hash", txHash, "from", from.String())
	}
	s.mu.Unlock()

	return nil
}

// ==================== 辅助方法 ====================

// computeTransactionHash 计算交易哈希
func (s *TxPropagationService) computeTransactionHash(tx *transactionpb.Transaction) string {
	// 基于pb字段计算哈希
	// 实际实现应使用项目标准哈希算法
	return fmt.Sprintf("tx_v%d_t%d_i%d",
		tx.GetVersion(),
		tx.GetCreationTimestamp(),
		len(tx.GetInputs()))
}

// markPeerKnowsTx 标记peer已知特定交易
func (s *TxPropagationService) markPeerKnowsTx(peerID peer.ID, txHash string) {
	if s.peerTxCache[peerID] == nil {
		s.peerTxCache[peerID] = make(map[string]bool)
	}
	s.peerTxCache[peerID][txHash] = true
}

// ==================== 架构问题暴露 ====================

/*
🚨 通过彻底实现pb优先原则，暴露了以下架构问题：

1. **pb协议定义不完整**：
   - 缺少 TxRequest 消息定义
   - 缺少 TxResponse 消息定义
   - 缺少 TxAnnouncement 消息定义

2. **需要补充的pb定义**：
   ```proto
   // 应在pb/network/protocol/transaction.proto中添加：
   message TxRequest {
     repeated string tx_hashes = 1;
     uint32 max_transactions = 2;
   }

   message TxResponse {
     repeated Transaction transactions = 1;
     repeated string missing_hashes = 2;
   }

   message TxAnnouncement {
     string tx_hash = 1;
     uint64 timestamp = 2;
     string peer_id = 3;
   }
   ```

3. **当前解决方案的局限性**：
   - 使用generic Envelope包装所有消息
   - 缺少类型化的消息处理
   - 无法充分利用protobuf的类型安全特性

✅ **正确的做法**：
1. 完善pb协议定义
2. 重新生成pb代码
3. 基于完整pb定义重新实现此示例

这种彻底的pb优先实现暴露了架构设计的真实状况，
比兼容层的伪实现更有价值。
*/
