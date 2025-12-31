package sync

import (
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

var (
	triggerGateMu   sync.Mutex
	lastTriggerTime time.Time
)

var (
	noUpstreamMu       sync.Mutex
	noUpstreamUntil    time.Time
	noUpstreamBackoff  time.Duration
	noUpstreamBackoffMax = 5 * time.Minute
)

// shouldSkipTriggerByMinInterval implements a lightweight de-bounce for TriggerSync calls.
//
// 设计目标：
// - 多源触发（订阅/定时/候选验证/手工）在真实网络下会“同时”或“连续”到达；
// - 依靠全局同步锁虽然能避免并发执行，但仍会产生大量失败日志/无意义的重复计算；
// - 这里用一个“最小触发间隔”把触发请求合并掉（返回 nil 语义：已触发/无需重复触发）。
func shouldSkipTriggerByMinInterval(configProvider config.Provider, logger log.Logger) bool {
	minMs := 0
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil {
			if bc.Sync.Advanced.GlobalMinTriggerIntervalMs > 0 {
				minMs = bc.Sync.Advanced.GlobalMinTriggerIntervalMs
			}
		}
	}
	if minMs <= 0 {
		return false
	}

	minInterval := time.Duration(minMs) * time.Millisecond
	now := time.Now()

	triggerGateMu.Lock()
	defer triggerGateMu.Unlock()

	if !lastTriggerTime.IsZero() && now.Sub(lastTriggerTime) < minInterval {
		if logger != nil {
			logger.Debugf("[TriggerSync] ⏳ skip: global_min_trigger_interval hit (min=%s, since=%s)",
				minInterval, now.Sub(lastTriggerTime))
		}
		return true
	}

	lastTriggerTime = now
	return false
}

// shouldSkipTriggerByNoUpstreamBackoff implements a backoff gate when there is no usable upstream peer.
//
// 背景（对应你日志中的现象）：
// - 多个模块（共识/启动流程/运维接口）可能频繁调用 TriggerSync；
// - 当路由表为空/没有可用上游 WES 节点时，triggerSyncImpl 会在 selectionTimeout 内等待并重试；
// - 若每次都等待到超时，外部再立即再次触发，就会形成“固定周期空跑”（例如每 30s 一次），浪费资源并刷日志。
//
// 设计：
// - 一旦判定“无上游”，启动指数退避冷却窗口；
// - 在冷却期内，非 urgent 触发直接 no-op（返回 nil），避免空跑；
// - 只在出现可用上游时 reset。
func shouldSkipTriggerByNoUpstreamBackoff(logger log.Logger) bool {
	now := time.Now()

	noUpstreamMu.Lock()
	defer noUpstreamMu.Unlock()

	if noUpstreamUntil.IsZero() || now.After(noUpstreamUntil) {
		return false
	}
	if logger != nil {
		logger.Debugf("[TriggerSync] ⏳ skip: no-upstream backoff (remaining=%s)", noUpstreamUntil.Sub(now))
	}
	return true
}

// markNoUpstream records that there is no usable upstream and advances the backoff window.
func markNoUpstream(logger log.Logger) {
	now := time.Now()

	noUpstreamMu.Lock()
	defer noUpstreamMu.Unlock()

	// 初始化退避：默认 30s（与 selectionTimeout 默认值相近，但会逐步拉长，避免长期空跑）
	if noUpstreamBackoff <= 0 {
		noUpstreamBackoff = 30 * time.Second
	} else {
		noUpstreamBackoff *= 2
		if noUpstreamBackoff > noUpstreamBackoffMax {
			noUpstreamBackoff = noUpstreamBackoffMax
		}
	}

	noUpstreamUntil = now.Add(noUpstreamBackoff)
	if logger != nil {
		logger.Debugf("[TriggerSync] 💤 no-upstream backoff armed: %s", noUpstreamBackoff)
	}
}

// resetNoUpstreamBackoff clears the no-upstream backoff window once upstream peers are available.
func resetNoUpstreamBackoff() {
	noUpstreamMu.Lock()
	defer noUpstreamMu.Unlock()

	noUpstreamUntil = time.Time{}
	noUpstreamBackoff = 0
}
