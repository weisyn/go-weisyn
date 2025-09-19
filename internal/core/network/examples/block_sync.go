package examples

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	blockpb "github.com/weisyn/v1/pb/blockchain/block"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// block_sync.go
// 区块同步示例：严格基于pb定义的区块同步实现
// 🎯 核心原则：完全使用pb定义，无任何自定义类型

// ==================== 协议定义 ====================
// 协议和主题常量已在protocols.go中统一定义

const (
	// 本地使用的协议别名
	ProtocolBlockReq = "/weisyn/block/request/v1.0.0"
)

// ==================== 区块同步服务端 ====================

// BlockSyncServer 区块同步服务端 - 严格基于pb协议
type BlockSyncServer struct {
	network iface.Network
	logger  logiface.Logger

	// 严格使用pb定义的区块存储
	blockchain map[uint64]*blockpb.Block // 高度 -> pb.Block
}

// NewBlockSyncServer 创建区块同步服务端
func NewBlockSyncServer(network iface.Network, logger logiface.Logger) *BlockSyncServer {
	return &BlockSyncServer{
		network:    network,
		logger:     logger,
		blockchain: make(map[uint64]*blockpb.Block),
	}
}

// Start 启动区块同步服务端
func (s *BlockSyncServer) Start() error {
	// 注册区块同步协议处理器
	if err := s.network.RegisterStreamHandler(ProtocolBlockSync, s.handleBlockSync); err != nil {
		return fmt.Errorf("failed to register block sync handler: %w", err)
	}

	// 注册单区块请求处理器
	if err := s.network.RegisterStreamHandler(ProtocolBlockReq, s.handleBlockRequest); err != nil {
		return fmt.Errorf("failed to register block request handler: %w", err)
	}

	s.logger.Infof("block sync server started")
	return nil
}

// AddBlock 添加新区块并广播
func (s *BlockSyncServer) AddBlock(block *blockpb.Block) error {
	if block == nil {
		return fmt.Errorf("block cannot be nil")
	}

	height := block.GetHeader().GetHeight() // 从Block的Header获取高度
	s.blockchain[height] = block

	// 广播新区块
	return s.broadcastNewBlock(block)
}

// broadcastNewBlock 广播新区块
func (s *BlockSyncServer) broadcastNewBlock(block *blockpb.Block) error {
	// 序列化区块为pb格式
	blockData, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// 使用transport.Envelope包装
	envelope := &transportpb.Envelope{
		Version:     1,
		Topic:       TopicNewBlock,
		ContentType: "application/x-protobuf",
		Encoding:    "pb",
		Compression: "none",
		Payload:     blockData,
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	envelopeData, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	// 发布到topic
	ctx := context.Background()
	return s.network.Publish(ctx, TopicNewBlock, envelopeData, nil)
}

// handleBlockSync 处理区块同步请求
func (s *BlockSyncServer) handleBlockSync(ctx context.Context, from peer.ID, reqData []byte) ([]byte, error) {
	s.logger.Debugf("received block sync request", "from", from.String())

	// 🚨 架构问题：缺少pb定义
	// 需要在pb/network/protocol/中定义BlockSyncRequest消息：
	// message BlockSyncRequest {
	//   uint64 start_height = 1;
	//   uint64 end_height = 2;
	//   uint32 max_blocks = 3;
	// }

	// 解析请求Envelope
	var envelope transportpb.Envelope
	if err := proto.Unmarshal(reqData, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// ⚠️ 临时处理：简化请求解析
	// 实际应该有专门的BlockSyncRequest消息类型
	startHeight := uint64(1)
	endHeight := uint64(10) // 简化处理

	return s.buildBlockSyncResponse(startHeight, endHeight)
}

// buildBlockSyncResponse 构建区块同步响应
func (s *BlockSyncServer) buildBlockSyncResponse(startHeight, endHeight uint64) ([]byte, error) {
	var responseBlocks []*blockpb.Block

	// 获取指定范围的区块
	for height := startHeight; height <= endHeight; height++ {
		if block, exists := s.blockchain[height]; exists {
			responseBlocks = append(responseBlocks, block)
		}
	}

	// 🚨 架构问题：缺少BlockSyncResponse消息定义
	// 需要定义：
	// message BlockSyncResponse {
	//   repeated Block blocks = 1;
	//   uint64 next_height = 2;
	//   bool has_more = 3;
	// }

	// 临时方案：如果有区块，序列化第一个区块
	var responseData []byte
	if len(responseBlocks) > 0 {
		var err error
		responseData, err = proto.Marshal(responseBlocks[0])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}
	}

	// 构建响应Envelope
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

// handleBlockRequest 处理单区块请求
func (s *BlockSyncServer) handleBlockRequest(ctx context.Context, from peer.ID, reqData []byte) ([]byte, error) {
	s.logger.Debugf("received block request", "from", from.String())

	// 解析请求
	var envelope transportpb.Envelope
	if err := proto.Unmarshal(reqData, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// ⚠️ 简化处理：返回高度1的区块
	height := uint64(1)
	block := s.blockchain[height]
	if block == nil {
		return nil, fmt.Errorf("block not found at height %d", height)
	}

	// 序列化区块
	blockData, err := proto.Marshal(block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block: %w", err)
	}

	// 构建响应
	responseEnvelope := &transportpb.Envelope{
		Version:     1,
		ContentType: "application/x-protobuf",
		Encoding:    "pb",
		Compression: "none",
		Payload:     blockData,
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	return proto.Marshal(responseEnvelope)
}

// ==================== 区块同步客户端 ====================

// BlockSyncClient 区块同步客户端 - 严格基于pb协议
type BlockSyncClient struct {
	network iface.Network
	logger  logiface.Logger
}

// NewBlockSyncClient 创建区块同步客户端
func NewBlockSyncClient(network iface.Network, logger logiface.Logger) *BlockSyncClient {
	return &BlockSyncClient{
		network: network,
		logger:  logger,
	}
}

// SyncBlocks 同步区块范围
func (c *BlockSyncClient) SyncBlocks(ctx context.Context, targetPeer peer.ID, startHeight, endHeight uint64) ([]*blockpb.Block, error) {
	c.logger.Infof("syncing blocks", "target", targetPeer.String(), "range", fmt.Sprintf("%d-%d", startHeight, endHeight))

	// 构建请求Envelope
	// ⚠️ 简化处理：空payload，实际应使用BlockSyncRequest
	requestEnvelope := &transportpb.Envelope{
		Version:     1,
		ContentType: "application/x-protobuf",
		Encoding:    "pb",
		Compression: "none",
		Payload:     []byte{}, // 简化请求
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	reqData, err := proto.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送同步请求
	respData, err := c.network.Call(ctx, targetPeer, ProtocolBlockSync, reqData, nil)
	if err != nil {
		return nil, fmt.Errorf("sync request failed: %w", err)
	}

	// 解析响应
	var responseEnvelope transportpb.Envelope
	if err := proto.Unmarshal(respData, &responseEnvelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 解析区块
	var block blockpb.Block
	if err := proto.Unmarshal(responseEnvelope.Payload, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	// 返回单个区块（简化处理）
	return []*blockpb.Block{&block}, nil
}

// RequestBlock 请求单个区块
func (c *BlockSyncClient) RequestBlock(ctx context.Context, targetPeer peer.ID, height uint64) (*blockpb.Block, error) {
	c.logger.Debugf("requesting block", "target", targetPeer.String(), "height", height)

	// 构建请求
	requestEnvelope := &transportpb.Envelope{
		Version:     1,
		ContentType: "application/x-protobuf",
		Encoding:    "pb",
		Compression: "none",
		Payload:     []byte{}, // 简化请求
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	reqData, err := proto.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	respData, err := c.network.Call(ctx, targetPeer, ProtocolBlockReq, reqData, nil)
	if err != nil {
		return nil, fmt.Errorf("block request failed: %w", err)
	}

	// 解析响应
	var responseEnvelope transportpb.Envelope
	if err := proto.Unmarshal(respData, &responseEnvelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 解析区块
	var block blockpb.Block
	if err := proto.Unmarshal(responseEnvelope.Payload, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// SubscribeToNewBlocks 订阅新区块广播
func (c *BlockSyncClient) SubscribeToNewBlocks(handler func(*blockpb.Block) error) error {
	_, err := c.network.Subscribe(TopicNewBlock, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
		// 解析Envelope
		var envelope transportpb.Envelope
		if err := proto.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to unmarshal envelope: %w", err)
		}

		// 解析区块
		var block blockpb.Block
		if err := proto.Unmarshal(envelope.Payload, &block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}

		// 调用处理器
		return handler(&block)
	})

	return err
}

// ==================== 架构问题暴露 ====================

/*
🚨 通过严格的pb优先实现，暴露了关键架构问题：

1. **缺少专门的区块同步pb消息定义**：
   - BlockSyncRequest
   - BlockSyncResponse
   - BlockRequest
   - BlockResponse

2. **需要补充的pb定义**：
   ```proto
   // 应在pb/network/protocol/block.proto中添加：
   message BlockSyncRequest {
     uint64 start_height = 1;
     uint64 end_height = 2;
     uint32 max_blocks = 3;
   }

   message BlockSyncResponse {
     repeated Block blocks = 1;
     uint64 next_height = 2;
     bool has_more = 3;
   }

   message BlockRequest {
     uint64 height = 1;
   }

   message BlockResponse {
     Block block = 1;
     bool exists = 2;
   }
   ```

3. **当前实现的局限性**：
   - 使用generic Envelope包装所有消息
   - 缺少类型安全的消息处理
   - 简化的请求/响应逻辑

✅ **正确的解决方案**：
1. 完善pb协议定义
2. 基于完整pb定义重新实现
3. 利用protobuf的类型安全特性

这种彻底的pb优先方法揭示了真实的架构需求，
比兼容性妥协更有价值。
*/
