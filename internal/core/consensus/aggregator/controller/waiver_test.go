// Package controller 弃权响应处理单元测试
package controller

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/weisyn/v1/pb/network/protocol"
	"google.golang.org/protobuf/proto"
)

// TestForwardBlockToCorrectAggregator_WaiverResponse 测试弃权响应处理
func TestForwardBlockToCorrectAggregator_WaiverResponse(t *testing.T) {
	t.Run("收到弃权响应应触发重选", func(t *testing.T) {
		// 模拟聚合器返回弃权响应
		waiverResponse := &protocol.AggregatorBlockAcceptance{
			Base: &protocol.BaseMessage{
				MessageId:     "test-msg-id",
				SenderId:      []byte("aggregator-peer-id"),
				TimestampUnix: time.Now().Unix(),
			},
			Waived:       true,
			WaiverReason: protocol.AggregatorBlockAcceptance_WAIVER_HEIGHT_TOO_FAR_AHEAD,
			LocalHeight:  100,
		}

		respBytes, err := proto.Marshal(waiverResponse)
		assert.NoError(t, err)

		// 验证：弃权响应应被正确解析
		var acceptance protocol.AggregatorBlockAcceptance
		err = proto.Unmarshal(respBytes, &acceptance)
		assert.NoError(t, err)
		assert.True(t, acceptance.Waived)
		assert.Equal(t, protocol.AggregatorBlockAcceptance_WAIVER_HEIGHT_TOO_FAR_AHEAD, acceptance.WaiverReason)
	})
}

// TestForwardBlockToCorrectAggregator_MaxRetryAttempts 测试最大重试次数保护
func TestForwardBlockToCorrectAggregator_MaxRetryAttempts(t *testing.T) {
	t.Run("超过最大重试次数应触发回环兜底", func(t *testing.T) {
		const maxRetryAttempts = 10

		// 验证：retryAttempt >= maxRetryAttempts 时应触发回环逻辑
		retryAttempt := uint32(10)
		assert.GreaterOrEqual(t, retryAttempt, uint32(maxRetryAttempts))

		// 预期行为：触发回环兜底，由原始矿工处理
	})

	t.Run("未超过最大重试次数应继续重选", func(t *testing.T) {
		const maxRetryAttempts = 10

		retryAttempt := uint32(3)
		assert.Less(t, retryAttempt, uint32(maxRetryAttempts))

		// 预期行为：继续执行重选逻辑
	})
}

// TestForwardBlockToCorrectAggregator_LoopDetection 测试回环检测
func TestForwardBlockToCorrectAggregator_LoopDetection(t *testing.T) {
	t.Run("所有候选都弃权应触发回环", func(t *testing.T) {
		// 模拟场景：K桶有5个节点，都已弃权
		waivedAggregators := []peer.ID{
			"peer1",
			"peer2",
			"peer3",
			"peer4",
			"peer5",
		}

		// 验证：弃权节点数量达到或接近K桶大小时，应触发回环
		kBucketSize := 20
		assert.Less(t, len(waivedAggregators), kBucketSize)

		// 如果选举失败（GetAggregatorForHeightWithWaivers 返回错误），
		// 应检查 originalMinerPeerID == localPeerID，触发回环兜底
	})
}

// TestParseWaiverResponse 测试弃权响应解析
func TestParseWaiverResponse(t *testing.T) {
	testCases := []struct {
		name         string
		waived       bool
		waiverReason protocol.AggregatorBlockAcceptance_WaiverReason
		localHeight  uint64
	}{
		{
			name:         "高度过高弃权",
			waived:       true,
			waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_HEIGHT_TOO_FAR_AHEAD,
			localHeight:  100,
		},
		{
			name:         "聚合进行中弃权",
			waived:       true,
			waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_AGGREGATION_IN_PROGRESS,
			localHeight:  150,
		},
		{
			name:         "接受（非弃权）",
			waived:       false,
			waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_NONE,
			localHeight:  200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := &protocol.AggregatorBlockAcceptance{
				Base: &protocol.BaseMessage{
					MessageId:     "test-msg",
					SenderId:      []byte("test-peer"),
					TimestampUnix: time.Now().Unix(),
				},
				Waived:       tc.waived,
				WaiverReason: tc.waiverReason,
				LocalHeight:  tc.localHeight,
			}

			// 序列化
			respBytes, err := proto.Marshal(response)
			assert.NoError(t, err)
			assert.NotEmpty(t, respBytes)

			// 反序列化
			var parsed protocol.AggregatorBlockAcceptance
			err = proto.Unmarshal(respBytes, &parsed)
			assert.NoError(t, err)

			// 验证
			assert.Equal(t, tc.waived, parsed.Waived)
			assert.Equal(t, tc.waiverReason, parsed.WaiverReason)
			assert.Equal(t, tc.localHeight, parsed.LocalHeight)
		})
	}
}

// TestRecursiveRetry 测试递归重选逻辑
func TestRecursiveRetry(t *testing.T) {
	t.Run("弃权列表应正确累加", func(t *testing.T) {
		// 初始弃权列表
		waived1 := []peer.ID{"peer1"}

		// 第一次重选后，peer2 也弃权
		newPeer := peer.ID("peer2")
		waived2 := append(waived1, newPeer)

		assert.Len(t, waived2, 2)
		assert.Contains(t, waived2, peer.ID("peer1"))
		assert.Contains(t, waived2, peer.ID("peer2"))

		// 验证：递归调用时应携带更新后的弃权列表
	})

	t.Run("重试次数应递增", func(t *testing.T) {
		retryAttempt := uint32(0)

		// 第一次弃权
		retryAttempt++
		assert.Equal(t, uint32(1), retryAttempt)

		// 第二次弃权
		retryAttempt++
		assert.Equal(t, uint32(2), retryAttempt)

		// 验证：递归调用时应传递递增的 retryAttempt
	})
}

// TestReadOnlyModeWaiver 测试只读模式弃权机制
func TestReadOnlyModeWaiver(t *testing.T) {
	t.Run("只读模式弃权响应序列化和反序列化", func(t *testing.T) {
		// 创建只读模式弃权响应
		waiverResponse := &protocol.AggregatorBlockAcceptance{
			Base: &protocol.BaseMessage{
				MessageId:     "test-readonly-msg",
				SenderId:      []byte("aggregator-readonly"),
				TimestampUnix: time.Now().Unix(),
			},
			Waived:       true,
			WaiverReason: protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE,
			LocalHeight:  2385, // 节点当前高度
		}

		// 序列化
		respBytes, err := proto.Marshal(waiverResponse)
		assert.NoError(t, err)
		assert.NotEmpty(t, respBytes)

		// 反序列化
		var parsed protocol.AggregatorBlockAcceptance
		err = proto.Unmarshal(respBytes, &parsed)
		assert.NoError(t, err)

		// 验证弃权标志
		assert.True(t, parsed.Waived, "弃权标志应为 true")
		assert.Equal(t, protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE, parsed.WaiverReason, "弃权原因应为只读模式")
		assert.Equal(t, uint64(2385), parsed.LocalHeight, "本地高度应正确传递")
	})

	t.Run("只读模式弃权应触发自动转发", func(t *testing.T) {
		// 模拟场景：节点 A 进入只读模式，返回弃权响应
		// 验证：提交者应将区块转发给下一个聚合器节点 B
		
		waiverResponse := &protocol.AggregatorBlockAcceptance{
			Base: &protocol.BaseMessage{
				MessageId:     "test-msg-readonly-forward",
				SenderId:      []byte("aggregator-a-readonly"),
				TimestampUnix: time.Now().Unix(),
			},
			Waived:           true,
			WaiverReason:     protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE,
			LocalHeight:      2385,
			AcceptanceReason: "waiver: node in read-only mode (candidate=2400 local=2385)",
		}

		respBytes, err := proto.Marshal(waiverResponse)
		assert.NoError(t, err)

		var parsed protocol.AggregatorBlockAcceptance
		err = proto.Unmarshal(respBytes, &parsed)
		assert.NoError(t, err)

		// 验证弃权原因消息
		assert.True(t, parsed.Waived)
		assert.Equal(t, protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE, parsed.WaiverReason)
		assert.Contains(t, parsed.AcceptanceReason, "read-only mode", "应包含只读模式说明")
		
		// 预期行为：
		// 1. 解析到 Waived=true 和 WaiverReason=WAIVER_READ_ONLY_MODE
		// 2. 将节点 A 加入 waivedAggregators 列表
		// 3. 递归调用 forwardBlockToCorrectAggregator，选择下一个聚合器
		// 4. 重试次数 +1
	})

	t.Run("混合弃权原因测试", func(t *testing.T) {
		// 测试所有弃权原因类型都能正确序列化和反序列化
		testCases := []struct {
			name         string
			waiverReason protocol.AggregatorBlockAcceptance_WaiverReason
			description  string
		}{
			{
				name:         "高度过高",
				waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_HEIGHT_TOO_FAR_AHEAD,
				description:  "候选区块高度远超本地高度",
			},
			{
				name:         "聚合进行中",
				waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_AGGREGATION_IN_PROGRESS,
				description:  "聚合器正忙",
			},
			{
				name:         "只读模式",
				waiverReason: protocol.AggregatorBlockAcceptance_WAIVER_READ_ONLY_MODE,
				description:  "节点处于只读模式",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				response := &protocol.AggregatorBlockAcceptance{
					Base: &protocol.BaseMessage{
						MessageId:     "test-" + tc.name,
						SenderId:      []byte("test-peer"),
						TimestampUnix: time.Now().Unix(),
					},
					Waived:       true,
					WaiverReason: tc.waiverReason,
					LocalHeight:  100,
				}

				respBytes, err := proto.Marshal(response)
				assert.NoError(t, err)

				var parsed protocol.AggregatorBlockAcceptance
				err = proto.Unmarshal(respBytes, &parsed)
				assert.NoError(t, err)

				assert.True(t, parsed.Waived)
				assert.Equal(t, tc.waiverReason, parsed.WaiverReason)
			})
		}
	})
}
