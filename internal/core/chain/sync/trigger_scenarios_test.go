// trigger_scenarios_test.go - 同步场景单元测试
// 测试SYNC_CRITICAL_DEFECTS_ANALYSIS.md中描述的关键场景
package sync

import (
	"errors"
	"fmt"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
)

// ============================================================================
// 测试场景1：高高度节点在阶段2失败
// ============================================================================

// TestScenario1_HighHeightNodeFailsInStage2 测试场景1
// 
// 场景描述：
//   1. 阶段1.5选择节点A（height=697）
//   2. 阶段2 hello时节点A超时
//   3. 切换到节点B（height=605）
//
// 预期结果：
//   - ✅ networkHeight保持697（不被605覆盖）
//   - ✅ 阶段3判断 localHeight=605 < networkHeight=697
//   - ✅ 继续分页同步到697
func TestScenario1_HighHeightNodeFailsInStage2(t *testing.T) {
	t.Skip("需要完整的Mock框架支持，暂时跳过")
	
	// TODO: 创建所有必需的Mock对象
	// - mockChainQuery
	// - mockQueryService
	// - mockBlockValidator
	// - mockBlockProcessor
	// - mockRoutingManager
	// - mockNetworkService
	// - mockP2PService
	// - mockConfigProvider
	// - mockTempStore
	// - mockBlockHashClient
	// - mockForkHandler
	// - mockLogger
	// - mockEventBus
	
	// 模拟节点
	peerA, err := peer.Decode("12D3KooWKP9kKHLeGnjVNNmqzGNmYPqFshQ5aYXEwCxKS4Yp7Qcw")
	require.NoError(t, err)
	
	_, err = peer.Decode("12D3KooWFHXfqxVKqgP9LyFyGhjXqYwPKLfKzJJKvn6xW6wnfAmY")
	require.NoError(t, err)
	
	// 设置场景：阶段1.5返回高度697，来源节点A
	// TODO: mockNetworkService.On("Call", ...).Return(...)
	
	// 执行同步
	// err = triggerSyncImpl(ctx, ...)
	
	// 验证结果
	// assert.NoError(t, err)
	
	// 验证失败原因被正确记录
	failures := GetSyncFailureHistory()
	foundTimeout := false
	for _, f := range failures {
		if f.Peer == peerA && f.Reason == FailureReasonTimeout {
			foundTimeout = true
			break
		}
	}
	assert.True(t, foundTimeout, "应该记录peerA的timeout失败")
	
	// 验证网络高度保持697（未被605覆盖）
	diag := GetSyncDiagnostics()
	assert.Equal(t, uint64(697), diag.CurrentNetworkHeight, "网络高度应保持697")
}

// ============================================================================
// 测试场景2：对端返回REMOTE_BEHIND
// ============================================================================

// TestScenario2_RemoteBehind 测试场景2
//
// 场景描述：
//   1. 阶段1.5选择节点A（height=697）
//   2. 阶段2 hello返回REMOTE_BEHIND（节点B只有605）
//
// 预期结果：
//   - ✅ 识别REMOTE_BEHIND，跳过节点B
//   - ✅ 尝试下一个节点
//   - ✅ 最终使用高高度节点完成同步
func TestScenario2_RemoteBehind(t *testing.T) {
	t.Skip("需要完整的Mock框架支持，暂时跳过")
	
	// 模拟节点
	_, err := peer.Decode("12D3KooWKP9kKHLeGnjVNNmqzGNmYPqFshQ5aYXEwCxKS4Yp7Qcw")
	require.NoError(t, err)
	
	peerB, err := peer.Decode("12D3KooWFHXfqxVKqgP9LyFyGhjXqYwPKLfKzJJKvn6xW6wnfAmY")
	require.NoError(t, err)
	
	// 阶段2：peerB返回REMOTE_BEHIND
	// TODO: mockNetworkService.On("Call", mock.Anything, peerB, protocols.ProtocolSyncHelloV2, ...).
	//       Return(createHelloResponse("REMOTE_BEHIND", 605), nil).Once()
	
	// 执行同步
	// err = triggerSyncImpl(...)
	
	// 验证结果
	// assert.NoError(t, err)
	
	// 验证peerB被记录为低高度节点
	assert.True(t, isLowHeightPeer(peerB), "peerB应被标记为低高度节点")
	
	// 验证最终使用peerA完成同步
	_ = GetSyncDiagnostics()
	// assert.Contains(t, diag.CurrentDataSourcePeer, peerA.String(), "应使用peerA作为数据源")
}

// ============================================================================
// 测试场景3：所有节点都失败
// ============================================================================

// TestScenario3_AllNodesFail 测试场景3
//
// 场景描述：
//   1. 阶段1.5选择节点A（height=697）
//   2. 阶段2所有节点hello都失败
//
// 预期结果：
//   - ✅ 记录所有失败原因到syncFailureHistory
//   - ✅ 返回错误，但保留networkHeight=697
//   - ✅ 下次同步时可通过诊断接口查看失败原因
func TestScenario3_AllNodesFail(t *testing.T) {
	t.Skip("需要完整的Mock框架支持，暂时跳过")
	
	// 阶段2：所有节点都超时
	// TODO: mockNetworkService.On("Call", mock.Anything, mock.Anything, protocols.ProtocolSyncHelloV2, ...).
	//       Return(nil, errors.New("timeout")).Times(len(candidatePeers))
	
	// 执行同步
	// err := triggerSyncImpl(...)
	
	// 验证结果
	// assert.Error(t, err, "应返回错误")
	// assert.Contains(t, err.Error(), "已达最大重试次数", "错误信息应包含重试信息")
	
	// 验证所有失败原因被记录
	_ = GetSyncFailureHistory()
	// assert.GreaterOrEqual(t, len(failures), len(candidatePeers), "应记录所有节点的失败")
	
	// 验证网络高度保持697（未被修改）
	diag := GetSyncDiagnostics()
	// assert.Equal(t, uint64(697), diag.CurrentNetworkHeight, "网络高度应保持697")
	
	// 验证可通过诊断接口查看失败原因
	assert.NotEmpty(t, diag.RecentFailures, "诊断接口应返回失败历史")
}

// ============================================================================
// 测试辅助函数：失败原因分类
// ============================================================================

// TestClassifyError 测试ClassifyError函数
func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: FailureReasonInternalError,
		},
		{
			name:     "timeout error",
			err:      errors.New("i/o timeout"),
			expected: FailureReasonTimeout,
		},
		{
			name:     "deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: FailureReasonTimeout,
		},
		{
			name:     "protocol not supported",
			err:      errors.New("protocol not supported"),
			expected: FailureReasonProtocolNotSupported,
		},
		{
			name:     "no protocol handler",
			err:      errors.New("no protocol handler"),
			expected: FailureReasonProtocolNotSupported,
		},
		{
			name:     "chain identity mismatch",
			err:      errors.New("chain identity mismatch: remote=testnet local=mainnet"),
			expected: FailureReasonChainIdentityMismatch,
		},
		{
			name:     "invalid response - unmarshal",
			err:      errors.New("unmarshal failed"),
			expected: FailureReasonInvalidResponse,
		},
		{
			name:     "invalid response - decode",
			err:      errors.New("decode error"),
			expected: FailureReasonInvalidResponse,
		},
		{
			name:     "generic network error",
			err:      errors.New("connection refused"),
			expected: FailureReasonNetworkError,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Mock辅助函数
// ============================================================================

// createHeightQueryResponse 创建高度查询响应
// 注意：实际的高度查询响应使用自定义protobuf消息，此处仅为测试框架预留
func createHeightQueryResponse(height uint64) []byte {
	// TODO: 使用实际的QueryBlockHeightResponse protobuf定义
	// 目前暂时返回JSON编码的高度
	return []byte(fmt.Sprintf(`{"height":%d}`, height))
}

// createHelloResponse 创建hello响应
func createHelloResponse(relationshipStr string, remoteTipHeight uint64) []byte {
	// 将字符串转换为SyncRelationship枚举
	var relationship protocol.SyncRelationship
	switch relationshipStr {
	case "UP_TO_DATE":
		relationship = protocol.SyncRelationship_UP_TO_DATE
	case "REMOTE_AHEAD_SAME_CHAIN":
		relationship = protocol.SyncRelationship_REMOTE_AHEAD_SAME_CHAIN
	case "REMOTE_BEHIND":
		relationship = protocol.SyncRelationship_REMOTE_BEHIND
	case "FORK_DETECTED":
		relationship = protocol.SyncRelationship_FORK_DETECTED
	default:
		relationship = protocol.SyncRelationship_UNKNOWN
	}
	
	resp := &protocol.SyncHelloV2Response{
		RequestId:       "test-request",
		Success:         true,
		Relationship:    relationship,
		RemoteTipHeight: remoteTipHeight,
		ChainIdentity: &protocol.ChainIdentity{
			ChainId:   "test_chain",
			NetworkId: "testnet",
		},
	}
	data, _ := proto.Marshal(resp)
	return data
}

// createBlocksResponse 创建区块响应
func createBlocksResponse(startHeight, endHeight uint64) []byte {
	blocks := make([]*core.Block, 0)
	
	for h := startHeight; h <= endHeight && h < startHeight+10; h++ {
		block := &core.Block{
			Header: &core.BlockHeader{
				Height:       h,
				PreviousHash: []byte(fmt.Sprintf("hash_%d", h-1)),
			},
		}
		blocks = append(blocks, block)
	}
	
	resp := &protocol.SyncBlocksV2Response{
		RequestId: "test-request",
		Success:   true,
		Blocks:    blocks,
	}
	data, _ := proto.Marshal(resp)
	return data
}

// ============================================================================
// 测试诊断功能
// ============================================================================

// TestSyncDiagnosticsUpdate 测试诊断信息更新
func TestSyncDiagnosticsUpdate(t *testing.T) {
	// 清空诊断信息
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.BlocksFetched = 0
		d.BlocksProcessed = 0
		d.CurrentDataSourcePeer = ""
	})
	
	// 模拟区块拉取
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.BlocksFetched += 100
		d.CurrentDataSourcePeer = "12D3KooWKP9kKH"
	})
	
	// 验证
	diag := GetSyncDiagnostics()
	assert.Equal(t, uint64(100), diag.BlocksFetched)
	assert.Equal(t, "12D3KooWKP9kKH", diag.CurrentDataSourcePeer)
	
	// 模拟区块处理
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.BlocksProcessed += 100
	})
	
	// 验证
	diag = GetSyncDiagnostics()
	assert.Equal(t, uint64(100), diag.BlocksProcessed)
}

// TestFailureHistoryTracking 测试失败历史追踪
func TestFailureHistoryTracking(t *testing.T) {
	// 清空失败历史
	ClearSyncFailureHistory()
	
	// 模拟peer ID
	peerA, err := peer.Decode("12D3KooWKP9kKHLeGnjVNNmqzGNmYPqFshQ5aYXEwCxKS4Yp7Qcw")
	require.NoError(t, err)
	
	// 记录失败
	recordSyncFailure(peerA, "hello", FailureReasonTimeout, "i/o timeout", nil)
	
	// 验证
	history := GetSyncFailureHistory()
	assert.Len(t, history, 1)
	assert.Equal(t, peerA, history[0].Peer)
	assert.Equal(t, "hello", history[0].Stage)
	assert.Equal(t, FailureReasonTimeout, history[0].Reason)
}

// TestNetworkHeightTracking 测试网络高度追踪
func TestNetworkHeightTracking(t *testing.T) {
	// 清空高度历史
	heightHistoryMu.Lock()
	heightHistory = nil
	heightHistoryMu.Unlock()
	
	// 模拟peer ID
	peerA, err := peer.Decode("12D3KooWKP9kKHLeGnjVNNmqzGNmYPqFshQ5aYXEwCxKS4Yp7Qcw")
	require.NoError(t, err)
	
	// 记录网络高度
	recordNetworkHeight(697, peerA, "height_query")
	
	// 验证
	history := GetNetworkHeightHistory()
	assert.Len(t, history, 1)
	assert.Equal(t, uint64(697), history[0].Height)
	assert.Equal(t, peerA, history[0].SourcePeer)
	assert.Equal(t, "height_query", history[0].Stage)
}

