//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/internal/core/infrastructure/log"
	nodeimpl "github.com/weisyn/v1/internal/core/infrastructure/node"
	netimpl "github.com/weisyn/v1/internal/core/network/impl"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessage 测试消息结构
type TestMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Sender    string `json:"sender"`
}

// TestSimplePubSub 简单的发布订阅测试
func TestSimplePubSub(t *testing.T) {
	// 创建两个节点进行测试
	node1, net1, cleanup1 := createTestNode(t, "node1", 14001)
	defer cleanup1()

	node2, net2, cleanup2 := createTestNode(t, "node2", 14002)
	defer cleanup2()

	// 等待节点启动完成
	time.Sleep(2 * time.Second)

	// 测试主题
	topic := "test.simple.message.v1"

	// 用于验证消息接收的通道
	received1 := make(chan TestMessage, 10)
	received2 := make(chan TestMessage, 10)

	// 节点1订阅主题
	unsub1, err := net1.Subscribe(topic, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
		var msg TestMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		t.Logf("Node1 收到消息: %+v (from: %s)", msg, from.String())
		received1 <- msg
		return nil
	})
	require.NoError(t, err)
	defer unsub1()

	// 节点2订阅主题
	unsub2, err := net2.Subscribe(topic, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
		var msg TestMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		t.Logf("Node2 收到消息: %+v (from: %s)", msg, from.String())
		received2 <- msg
		return nil
	})
	require.NoError(t, err)
	defer unsub2()

	// 等待订阅生效
	time.Sleep(3 * time.Second)

	// 尝试连接两个节点
	node1Info := node1.GetHost().GetPeerInfo()
	err = node2.GetHost().Connect(context.Background(), node1Info)
	if err != nil {
		t.Logf("连接失败，但继续测试: %v", err)
	}

	// 再等待一下，让网络稳定
	time.Sleep(2 * time.Second)

	// 检查pubsub连接情况
	peers1 := net1.GetTopicPeers(topic)
	peers2 := net2.GetTopicPeers(topic)
	t.Logf("Node1 在topic %s 上连接的peers: %v", topic, peers1)
	t.Logf("Node2 在topic %s 上连接的peers: %v", topic, peers2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 节点1发送消息
	msg1 := TestMessage{
		ID:        "msg1",
		Content:   "Hello from Node1!",
		Timestamp: time.Now().Unix(),
		Sender:    "node1",
	}

	data1, err := json.Marshal(msg1)
	require.NoError(t, err)

	t.Logf("Node1 正在发布消息: %+v", msg1)
	err = net1.Publish(ctx, topic, data1, &types.PublishOptions{
		MaxMessageSize: 1024,
	})
	require.NoError(t, err)

	// 节点2发送消息
	msg2 := TestMessage{
		ID:        "msg2",
		Content:   "Hello from Node2!",
		Timestamp: time.Now().Unix(),
		Sender:    "node2",
	}

	data2, err := json.Marshal(msg2)
	require.NoError(t, err)

	t.Logf("Node2 正在发布消息: %+v", msg2)
	err = net2.Publish(ctx, topic, data2, &types.PublishOptions{
		MaxMessageSize: 1024,
	})
	require.NoError(t, err)

	// 验证消息接收
	timeout := time.After(15 * time.Second)
	receivedCount1 := 0
	receivedCount2 := 0

	for receivedCount1 < 2 || receivedCount2 < 2 {
		select {
		case msg := <-received1:
			receivedCount1++
			t.Logf("Node1 验证收到消息 %d: %+v", receivedCount1, msg)
			if msg.Sender == "node1" {
				assert.Equal(t, msg1.ID, msg.ID)
				assert.Equal(t, msg1.Content, msg.Content)
			} else {
				assert.Equal(t, msg2.ID, msg.ID)
				assert.Equal(t, msg2.Content, msg.Content)
			}

		case msg := <-received2:
			receivedCount2++
			t.Logf("Node2 验证收到消息 %d: %+v", receivedCount2, msg)
			if msg.Sender == "node1" {
				assert.Equal(t, msg1.ID, msg.ID)
				assert.Equal(t, msg1.Content, msg.Content)
			} else {
				assert.Equal(t, msg2.ID, msg.ID)
				assert.Equal(t, msg2.Content, msg.Content)
			}

		case <-timeout:
			t.Errorf("超时！Node1收到%d条消息，Node2收到%d条消息", receivedCount1, receivedCount2)
			return
		}
	}

	t.Logf("✅ 测试成功！Node1收到%d条消息，Node2收到%d条消息", receivedCount1, receivedCount2)
}

// TestMultipleMessages 测试多条消息的发布订阅
func TestMultipleMessages(t *testing.T) {
	// 创建两个节点
	_, net1, cleanup1 := createTestNode(t, "sender", 15001)
	defer cleanup1()

	_, net2, cleanup2 := createTestNode(t, "receiver", 15002)
	defer cleanup2()

	// 等待节点启动
	time.Sleep(2 * time.Second)

	topic := "test.multiple.messages.v1"
	messageCount := 5

	// 接收消息的计数器
	var mu sync.Mutex
	receivedMessages := make(map[string]TestMessage)

	// 节点2订阅消息
	unsub, err := net2.Subscribe(topic, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
		var msg TestMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}

		mu.Lock()
		receivedMessages[msg.ID] = msg
		mu.Unlock()

		t.Logf("接收器收到消息: ID=%s, Content=%s", msg.ID, msg.Content)
		return nil
	})
	require.NoError(t, err)
	defer unsub()

	// 等待订阅生效
	time.Sleep(3 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 发送多条消息
	for i := 0; i < messageCount; i++ {
		msg := TestMessage{
			ID:        fmt.Sprintf("msg_%d", i),
			Content:   fmt.Sprintf("Message number %d", i),
			Timestamp: time.Now().Unix(),
			Sender:    "sender",
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		err = net1.Publish(ctx, topic, data, &types.PublishOptions{
			MaxMessageSize: 1024,
		})
		require.NoError(t, err)

		t.Logf("发送消息: %+v", msg)
		time.Sleep(100 * time.Millisecond) // 短暂间隔
	}

	// 等待消息接收
	time.Sleep(5 * time.Second)

	// 验证接收到的消息
	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, messageCount, len(receivedMessages), "应该接收到所有消息")

	for i := 0; i < messageCount; i++ {
		msgID := fmt.Sprintf("msg_%d", i)
		msg, exists := receivedMessages[msgID]
		assert.True(t, exists, "消息 %s 应该被接收", msgID)
		if exists {
			assert.Equal(t, fmt.Sprintf("Message number %d", i), msg.Content)
			assert.Equal(t, "sender", msg.Sender)
		}
	}

	t.Logf("✅ 多消息测试成功！发送%d条，接收%d条", messageCount, len(receivedMessages))
}

// createTestNode 创建测试节点
func createTestNode(t *testing.T, name string, port int) (nodeiface.NodeService, network.Network, func()) {
	// 创建日志器
	logger, err := log.New(nil)
	require.NoError(t, err)

	// 创建节点配置
	nodeConfig := createTestNodeConfig(port)

	// 创建简单的crypto managers
	hashMgr := &mockHashManager{}
	sigMgr := &mockSignatureManager{}

	// 创建节点服务
	nodeService, err := nodeimpl.New(nodeConfig, logger)
	require.NoError(t, err)

	// 启动节点
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = nodeService.Start(ctx)
	require.NoError(t, err)

	// 等待节点完全启动
	time.Sleep(1 * time.Second)

	// 创建网络门面
	networkFacade := netimpl.NewFacade(nodeService.GetHost(), logger, nil, hashMgr, sigMgr)

	// 初始化GossipSub
	networkFacade.InitializeGossipSub()

	t.Logf("创建节点 %s: PeerID=%s, 监听端口=%d",
		name,
		nodeService.GetHost().GetPeerInfo().ID.String(),
		port)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		nodeService.Stop(ctx)
	}

	return nodeService, networkFacade, cleanup
}

// createTestNodeConfig 创建测试节点配置
func createTestNodeConfig(port int) *node.NodeOptions {
	return &node.NodeOptions{
		Host: node.HostConfig{
			ListenAddresses: []string{
				fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port),
				fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic-v1", port),
			},
			Transport: node.TransportConfig{
				EnableTCP:  true,
				EnableQUIC: true,
			},
			Security: node.SecurityConfig{
				EnableTLS:   true,
				EnableNoise: true,
			},
		},
		Discovery: node.DiscoveryConfig{
			MDNS: node.MDNSConfig{
				Enabled: true,
			},
			DHT: node.DHTConfig{
				Enabled: false, // 简化测试，不启用DHT
			},
		},
		Connectivity: node.ConnectivityConfig{
			MinPeers:  1,
			MaxPeers:  10,
			LowWater:  2,
			HighWater: 8,
		},
	}
}

// Mock crypto managers for testing
type mockHashManager struct{}

func (m *mockHashManager) SHA256(data []byte) []byte {
	// 简单实现，仅用于测试
	hash := make([]byte, 32)
	for i, b := range data {
		if i < 32 {
			hash[i] = b
		}
	}
	return hash
}

func (m *mockHashManager) SHA3_256(data []byte) []byte {
	return m.SHA256(data)
}

func (m *mockHashManager) Blake2b(data []byte) []byte {
	return m.SHA256(data)
}

func (m *mockHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data)
}

type mockSignatureManager struct{}

func (m *mockSignatureManager) GenerateKeyPair() (crypto.PrivateKey, crypto.PublicKey, error) {
	return &mockPrivateKey{}, &mockPublicKey{}, nil
}

func (m *mockSignatureManager) Sign(privateKey crypto.PrivateKey, data []byte) ([]byte, error) {
	return []byte("mock_signature"), nil
}

func (m *mockSignatureManager) Verify(publicKey crypto.PublicKey, data []byte, signature []byte) bool {
	return string(signature) == "mock_signature"
}

func (m *mockSignatureManager) RecoverPublicKey(data []byte, signature []byte) (crypto.PublicKey, error) {
	return &mockPublicKey{}, nil
}

type mockPrivateKey struct{}

func (m *mockPrivateKey) Bytes() []byte            { return []byte("mock_private_key") }
func (m *mockPrivateKey) String() string           { return "mock_private_key" }
func (m *mockPrivateKey) Type() string             { return "mock" }
func (m *mockPrivateKey) Public() crypto.PublicKey { return &mockPublicKey{} }

type mockPublicKey struct{}

func (m *mockPublicKey) Bytes() []byte  { return []byte("mock_public_key") }
func (m *mockPublicKey) String() string { return "mock_public_key" }
func (m *mockPublicKey) Type() string   { return "mock" }
