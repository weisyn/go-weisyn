package keepalive

import (
	"sync/atomic"
)

// KeyPeerMetrics KeyPeer监控指标
type KeyPeerMetrics struct {
	// 探测指标
	ProbeAttempts    int64 // 探测尝试总数
	ProbeSuccess     int64 // 探测成功次数
	ProbeFail        int64 // 探测失败次数
	ProbeTimeout     int64 // 探测超时次数
	
	// 重连指标
	ReconnectAttempts int64 // 重连尝试总数
	ReconnectSuccess  int64 // 重连成功次数
	ReconnectFail     int64 // 重连失败次数
	
	// DHT补地址指标
	FindPeerAttempts  int64 // FindPeer尝试总数
	FindPeerSuccess   int64 // FindPeer成功次数
	FindPeerFail      int64 // FindPeer失败次数
	
	// 自愈指标
	RepairTriggered   int64 // 触发修复总数
	RepairSuccess     int64 // 修复成功次数
	RepairFail        int64 // 修复失败次数
	
	// 重置事件指标
	ResetEventsPublished int64 // 发布的重置事件数
}

// RecordProbeAttempt 记录探测尝试
func (m *KeyPeerMetrics) RecordProbeAttempt() {
	atomic.AddInt64(&m.ProbeAttempts, 1)
}

// RecordProbeSuccess 记录探测成功
func (m *KeyPeerMetrics) RecordProbeSuccess() {
	atomic.AddInt64(&m.ProbeSuccess, 1)
}

// RecordProbeFail 记录探测失败
func (m *KeyPeerMetrics) RecordProbeFail() {
	atomic.AddInt64(&m.ProbeFail, 1)
}

// RecordProbeTimeout 记录探测超时
func (m *KeyPeerMetrics) RecordProbeTimeout() {
	atomic.AddInt64(&m.ProbeTimeout, 1)
}

// RecordReconnectAttempt 记录重连尝试
func (m *KeyPeerMetrics) RecordReconnectAttempt() {
	atomic.AddInt64(&m.ReconnectAttempts, 1)
}

// RecordReconnectSuccess 记录重连成功
func (m *KeyPeerMetrics) RecordReconnectSuccess() {
	atomic.AddInt64(&m.ReconnectSuccess, 1)
}

// RecordReconnectFail 记录重连失败
func (m *KeyPeerMetrics) RecordReconnectFail() {
	atomic.AddInt64(&m.ReconnectFail, 1)
}

// RecordFindPeerAttempt 记录FindPeer尝试
func (m *KeyPeerMetrics) RecordFindPeerAttempt() {
	atomic.AddInt64(&m.FindPeerAttempts, 1)
}

// RecordFindPeerSuccess 记录FindPeer成功
func (m *KeyPeerMetrics) RecordFindPeerSuccess() {
	atomic.AddInt64(&m.FindPeerSuccess, 1)
}

// RecordFindPeerFail 记录FindPeer失败
func (m *KeyPeerMetrics) RecordFindPeerFail() {
	atomic.AddInt64(&m.FindPeerFail, 1)
}

// RecordRepairTriggered 记录修复触发
func (m *KeyPeerMetrics) RecordRepairTriggered() {
	atomic.AddInt64(&m.RepairTriggered, 1)
}

// RecordRepairSuccess 记录修复成功
func (m *KeyPeerMetrics) RecordRepairSuccess() {
	atomic.AddInt64(&m.RepairSuccess, 1)
}

// RecordRepairFail 记录修复失败
func (m *KeyPeerMetrics) RecordRepairFail() {
	atomic.AddInt64(&m.RepairFail, 1)
}

// RecordResetEventPublished 记录重置事件发布
func (m *KeyPeerMetrics) RecordResetEventPublished() {
	atomic.AddInt64(&m.ResetEventsPublished, 1)
}

// GetMetrics 获取当前指标快照
func (m *KeyPeerMetrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"probe_attempts":         atomic.LoadInt64(&m.ProbeAttempts),
		"probe_success":          atomic.LoadInt64(&m.ProbeSuccess),
		"probe_fail":             atomic.LoadInt64(&m.ProbeFail),
		"probe_timeout":          atomic.LoadInt64(&m.ProbeTimeout),
		"reconnect_attempts":     atomic.LoadInt64(&m.ReconnectAttempts),
		"reconnect_success":      atomic.LoadInt64(&m.ReconnectSuccess),
		"reconnect_fail":         atomic.LoadInt64(&m.ReconnectFail),
		"findpeer_attempts":      atomic.LoadInt64(&m.FindPeerAttempts),
		"findpeer_success":       atomic.LoadInt64(&m.FindPeerSuccess),
		"findpeer_fail":          atomic.LoadInt64(&m.FindPeerFail),
		"repair_triggered":       atomic.LoadInt64(&m.RepairTriggered),
		"repair_success":         atomic.LoadInt64(&m.RepairSuccess),
		"repair_fail":            atomic.LoadInt64(&m.RepairFail),
		"reset_events_published": atomic.LoadInt64(&m.ResetEventsPublished),
	}
}

