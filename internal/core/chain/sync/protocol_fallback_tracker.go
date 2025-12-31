// protocol_fallback_tracker.go - 协议回退记录系统
// 负责记录和追踪协议namespace回退事件
//
// 背景：
// - 网络层在调用带namespace的协议时，如果失败会自动回退到原始协议ID
// - 记录这些回退事件有助于：
//   1. 诊断协议兼容性问题
//   2. 识别未升级的旧节点
//   3. 监控网络迁移进度
//
// 注意：
// - 回退发生在network层（internal/core/network/facade/service.go）
// - 本文件提供记录接口，实际调用需要在network层集成
package sync

import (
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// ProtocolFallbackRecord 协议回退记录
type ProtocolFallbackRecord struct {
	Peer           peer.ID   `json:"peer"`
	QualifiedProto string    `json:"qualified_protocol"` // 带namespace的协议ID
	OriginalProto  string    `json:"original_protocol"`  // 原始协议ID
	Timestamp      time.Time `json:"timestamp"`
}

var (
	protocolFallbackMu      sync.RWMutex
	protocolFallbackHistory []ProtocolFallbackRecord
	maxProtocolFallbackHistory = 50
)

// RecordProtocolFallback 记录协议回退事件
//
// 参数：
//   - peerID: 目标节点ID
//   - qualifiedProto: 带namespace的协议ID（例如：/weisyn/testnet/sync/hello/v2）
//   - originalProto: 原始协议ID（例如：/weisyn/sync/hello/v2）
//
// 调用位置：
//   - internal/core/network/facade/service.go 的 Call 方法中
//   - 当 qualified protocol 失败并回退到 original protocol 时
func RecordProtocolFallback(peerID peer.ID, qualifiedProto, originalProto string) {
	protocolFallbackMu.Lock()
	defer protocolFallbackMu.Unlock()

	record := ProtocolFallbackRecord{
		Peer:           peerID,
		QualifiedProto: qualifiedProto,
		OriginalProto:  originalProto,
		Timestamp:      time.Now(),
	}

	protocolFallbackHistory = append(protocolFallbackHistory, record)
	if len(protocolFallbackHistory) > maxProtocolFallbackHistory {
		protocolFallbackHistory = protocolFallbackHistory[1:]
	}
}

// GetProtocolFallbackHistory 获取协议回退历史
//
// 返回值：
//   - []ProtocolFallbackRecord: 回退历史列表（按时间顺序）
func GetProtocolFallbackHistory() []ProtocolFallbackRecord {
	protocolFallbackMu.RLock()
	defer protocolFallbackMu.RUnlock()
	
	result := make([]ProtocolFallbackRecord, len(protocolFallbackHistory))
	copy(result, protocolFallbackHistory)
	return result
}

// ClearProtocolFallbackHistory 清空协议回退历史（用于测试或管理）
func ClearProtocolFallbackHistory() {
	protocolFallbackMu.Lock()
	defer protocolFallbackMu.Unlock()
	protocolFallbackHistory = nil
}

