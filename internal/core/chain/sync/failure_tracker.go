// failure_tracker.go - 同步失败原因记录系统
// 负责记录和追踪同步过程中的失败原因，用于诊断和快速切换
package sync

import (
	"context"
	"strings"
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ======================= 同步失败原因记录（SYNC-003修复） =======================
//
// 背景：
// - 同步过程中的失败可能发生在多个阶段：高度查询、hello握手、区块拉取、分页同步等。
// - 记录详细的失败原因有助于：
//   1. 诊断同步问题
//   2. 快速切换到其他节点
//   3. 避免重复向失败节点发起请求
//
// 功能：
// - 记录每个节点在不同阶段的失败原因
// - 保留最近的失败历史（默认100条）
// - 提供查询接口供诊断使用

// 失败原因常量
const (
	FailureReasonTimeout               = "timeout"
	FailureReasonProtocolNotSupported  = "protocol_not_supported"
	FailureReasonChainIdentityMismatch = "chain_identity_mismatch"
	FailureReasonNetworkError          = "network_error"
	FailureReasonInvalidResponse       = "invalid_response"
	FailureReasonInternalError         = "internal_error"
)

// SyncFailureReason 同步失败原因
type SyncFailureReason struct {
	Peer      peer.ID   `json:"peer"`      // 失败的节点ID
	Stage     string    `json:"stage"`     // 失败阶段：height_query/hello/blocks/paginated
	Reason    string    `json:"reason"`    // 失败原因分类
	Error     string    `json:"error"`     // 详细错误信息
	Timestamp time.Time `json:"timestamp"` // 失败时间
}

// PeerHealthStatus 节点健康状态（用于熔断机制）
type PeerHealthStatus struct {
	PeerID             peer.ID
	FailureCount       int       // 连续失败次数
	LastFailureTime    time.Time // 最近一次失败时间
	LastFailureReason  string    // 最近失败原因
	IsCircuitBroken    bool      // 是否熔断
	CircuitBrokenUntil time.Time // 熔断恢复时间
}

// PeerCircuitBrokenEvent 节点熔断事件（用于发布到事件总线）
type PeerCircuitBrokenEvent struct {
	PeerID       peer.ID
	FailureCount int
	RecoverAt    time.Time
}

var (
	syncFailureMu      sync.RWMutex
	syncFailureHistory []SyncFailureReason
	maxFailureHistory  = 100

	// 🔥 节点健康度跟踪（熔断机制）
	peerHealthMap   = make(map[peer.ID]*PeerHealthStatus)
	peerHealthMutex sync.RWMutex

	// 熔断策略配置（可通过 ConfigureCircuitBreaker 函数配置）
	circuitBreakerFailureThreshold = 3               // 连续失败次数阈值（默认3次）
	circuitBreakerRecoveryDuration = 5 * time.Minute // 熔断恢复时间（默认5分钟）

	// eventBus 用于发布熔断事件（可选）
	eventBus interface {
		Publish(topic string, ctx context.Context, data interface{}) error
	}
)

// ConfigureCircuitBreaker 配置熔断器参数
//
// 参数：
//   - failureThreshold: 连续失败次数阈值（达到后触发熔断），0表示使用默认值(3)
//   - recoverySeconds: 熔断恢复时间（秒），0表示使用默认值(300秒=5分钟)
//
// 调用时机：应在应用启动时调用，加载配置后立即配置熔断器
func ConfigureCircuitBreaker(failureThreshold, recoverySeconds int) {
	peerHealthMutex.Lock()
	defer peerHealthMutex.Unlock()

	if failureThreshold > 0 {
		circuitBreakerFailureThreshold = failureThreshold
	}
	if recoverySeconds > 0 {
		circuitBreakerRecoveryDuration = time.Duration(recoverySeconds) * time.Second
	}
}

// GetCircuitBreakerConfig 获取当前熔断器配置（用于诊断和监控）
//
// 返回值：
//   - failureThreshold: 当前失败阈值
//   - recoveryDuration: 当前恢复时间
func GetCircuitBreakerConfig() (failureThreshold int, recoveryDuration time.Duration) {
	peerHealthMutex.RLock()
	defer peerHealthMutex.RUnlock()
	return circuitBreakerFailureThreshold, circuitBreakerRecoveryDuration
}

// ClearAllCircuitBreakers 清除所有节点的熔断状态（用于管理员手动重置）
//
// 注意：此函数会重置所有节点的健康状态，应谨慎使用
func ClearAllCircuitBreakers() {
	peerHealthMutex.Lock()
	defer peerHealthMutex.Unlock()

	for _, health := range peerHealthMap {
		health.IsCircuitBroken = false
		health.FailureCount = 0
	}
}

// recordSyncFailure 记录一次同步失败
//
// 参数：
//   - peer: 失败的节点ID
//   - stage: 失败阶段（height_query/hello/blocks/paginated）
//   - reason: 失败原因分类（timeout/protocol_not_supported/chain_identity_mismatch/network_error/invalid_response）
//   - errMsg: 详细错误信息
//   - logger: 日志记录器
func recordSyncFailure(peerID peer.ID, stage, reason, errMsg string, logger log.Logger) {
	syncFailureMu.Lock()
	failure := SyncFailureReason{
		Peer:      peerID,
		Stage:     stage,
		Reason:    reason,
		Error:     errMsg,
		Timestamp: time.Now(),
	}

	syncFailureHistory = append(syncFailureHistory, failure)
	if len(syncFailureHistory) > maxFailureHistory {
		syncFailureHistory = syncFailureHistory[1:]
	}
	syncFailureMu.Unlock()

	// 🔥 更新节点健康度并判断是否需要熔断
	peerHealthMutex.Lock()
	health := peerHealthMap[peerID]
	if health == nil {
		health = &PeerHealthStatus{PeerID: peerID}
		peerHealthMap[peerID] = health
	}

	health.FailureCount++
	health.LastFailureTime = time.Now()
	health.LastFailureReason = errMsg

	// 🔥 熔断策略：连续失败 N 次 → 熔断 M 分钟
	if health.FailureCount >= circuitBreakerFailureThreshold && !health.IsCircuitBroken {
		health.IsCircuitBroken = true
		health.CircuitBrokenUntil = time.Now().Add(circuitBreakerRecoveryDuration)
		
		if logger != nil {
			logger.Warnf("⚡ 节点已熔断: peer=%s 失败次数=%d 恢复时间=%s",
				peerID.String()[:12]+"...", 
				health.FailureCount, 
				health.CircuitBrokenUntil.Format("15:04:05"))
		}

		// 🔥 发布熔断事件（如果事件总线可用）
		if eventBus != nil {
			_ = eventBus.Publish("peer.circuit_broken", context.Background(), PeerCircuitBrokenEvent{
				PeerID:       peerID,
				FailureCount: health.FailureCount,
				RecoverAt:    health.CircuitBrokenUntil,
			})
		}
	}
	peerHealthMutex.Unlock()

	if logger != nil {
		logger.Warnf("🔴 同步失败记录: peer=%s stage=%s reason=%s 失败次数=%d error=%s",
			peerID.String()[:12]+"...", stage, reason, health.FailureCount, errMsg)
	}
}

// GetSyncFailureHistory 获取同步失败历史
//
// 返回值：
//   - []SyncFailureReason: 失败历史列表（按时间顺序）
func GetSyncFailureHistory() []SyncFailureReason {
	syncFailureMu.RLock()
	defer syncFailureMu.RUnlock()
	result := make([]SyncFailureReason, len(syncFailureHistory))
	copy(result, syncFailureHistory)
	return result
}

// GetPeerFailureCount 获取指定节点的失败次数（最近N分钟内）
//
// 参数：
//   - peer: 节点ID
//   - duration: 时间窗口（例如10分钟）
//
// 返回值：
//   - int: 失败次数
func GetPeerFailureCount(peer peer.ID, duration time.Duration) int {
	syncFailureMu.RLock()
	defer syncFailureMu.RUnlock()

	count := 0
	cutoff := time.Now().Add(-duration)
	for _, f := range syncFailureHistory {
		if f.Peer == peer && f.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}

// GetStageFailureCount 获取指定阶段的失败次数（最近N分钟内）
//
// 参数：
//   - stage: 失败阶段
//   - duration: 时间窗口
//
// 返回值：
//   - int: 失败次数
func GetStageFailureCount(stage string, duration time.Duration) int {
	syncFailureMu.RLock()
	defer syncFailureMu.RUnlock()

	count := 0
	cutoff := time.Now().Add(-duration)
	for _, f := range syncFailureHistory {
		if f.Stage == stage && f.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}

// ClearSyncFailureHistory 清空同步失败历史（用于测试或管理）
func ClearSyncFailureHistory() {
	syncFailureMu.Lock()
	defer syncFailureMu.Unlock()
	syncFailureHistory = nil
}

// ClassifyError 根据错误类型分类失败原因
//
// 参数：
//   - err: 错误对象
//
// 返回值：
//   - string: 失败原因分类（timeout/protocol_not_supported/chain_identity_mismatch/network_error等）
func ClassifyError(err error) string {
	if err == nil {
		return FailureReasonInternalError
	}

	errMsg := err.Error()

	// 超时错误
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "context deadline exceeded") {
		return FailureReasonTimeout
	}

	// 网络层断连/重置（对端主动断开、链路不稳定、代理/NAT等导致）
	// 说明：这类错误在 libp2p 上经常表现为 stream reset / connection reset by peer，
	// 并不等价于“协议不支持”，否则会误导诊断。
	if strings.Contains(errMsg, "stream reset") ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "payload read failed") {
		return FailureReasonNetworkError
	}

	// 协议不支持（真正的“对端不支持该协议/无处理器”）
	if strings.Contains(errMsg, "protocol not supported") ||
		strings.Contains(errMsg, "no protocol handler") ||
		strings.Contains(errMsg, "failed to negotiate security protocol") {
		return FailureReasonProtocolNotSupported
	}

	// 链身份不匹配
	if strings.Contains(errMsg, "chain identity mismatch") ||
		strings.Contains(errMsg, "incompatible peer") ||
		strings.Contains(errMsg, "chain_identity") ||
		strings.Contains(errMsg, "链身份不匹配") {
		return FailureReasonChainIdentityMismatch
	}

	// 响应无效
	if strings.Contains(errMsg, "invalid response") ||
		strings.Contains(errMsg, "unmarshal") ||
		strings.Contains(errMsg, "decode") ||
		strings.Contains(errMsg, "parse") ||
		strings.Contains(errMsg, "解析") {
		return FailureReasonInvalidResponse
	}

	// 默认：网络错误
	return FailureReasonNetworkError
}

// ======================= 节点健康度管理（熔断机制） =======================

// IsHealthy 检查节点是否健康（未熔断或熔断已恢复）
//
// 参数：
//   - peerID: 节点ID
//
// 返回值：
//   - bool: true=健康可用, false=熔断中不可用
func IsHealthy(peerID peer.ID) bool {
	peerHealthMutex.Lock()
	defer peerHealthMutex.Unlock()

	health := peerHealthMap[peerID]
	if health == nil {
		return true // 未知节点，假定健康
	}

	// 如果被熔断且未到恢复时间，认为不健康
	if health.IsCircuitBroken && time.Now().Before(health.CircuitBrokenUntil) {
		return false
	}

	// 熔断时间已过，自动重置状态
	if health.IsCircuitBroken && time.Now().After(health.CircuitBrokenUntil) {
		health.IsCircuitBroken = false
		health.FailureCount = 0
	}

	return true
}

// ResetPeerHealth 重置节点健康度（成功响应后调用）
//
// 参数：
//   - peerID: 节点ID
func ResetPeerHealth(peerID peer.ID) {
	if peerID == "" {
		return
	}

	peerHealthMutex.Lock()
	defer peerHealthMutex.Unlock()

	health := peerHealthMap[peerID]
	if health != nil {
		health.FailureCount = 0
		health.IsCircuitBroken = false
	}
}

// GetPeerHealthStatus 获取节点健康状态（用于监控和诊断）
//
// 参数：
//   - peerID: 节点ID
//
// 返回值：
//   - *PeerHealthStatus: 健康状态信息，如果节点未被跟踪则返回nil
func GetPeerHealthStatus(peerID peer.ID) *PeerHealthStatus {
	peerHealthMutex.RLock()
	defer peerHealthMutex.RUnlock()

	health := peerHealthMap[peerID]
	if health == nil {
		return nil
	}

	// 返回副本，避免外部修改
	return &PeerHealthStatus{
		PeerID:             health.PeerID,
		FailureCount:       health.FailureCount,
		LastFailureTime:    health.LastFailureTime,
		LastFailureReason:  health.LastFailureReason,
		IsCircuitBroken:    health.IsCircuitBroken,
		CircuitBrokenUntil: health.CircuitBrokenUntil,
	}
}

