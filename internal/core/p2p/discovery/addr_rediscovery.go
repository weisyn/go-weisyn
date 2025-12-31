package discovery

import (
	"context"
	"time"

	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

// 🆕 Peer地址重发现机制
// 为无地址peer建立优先重发现队列，周期重试，智能退避

// TriggerRediscovery 触发重发现（由外部调用，如Discovery发现无地址peer时）
func (am *AddrManager) TriggerRediscovery(pid libpeer.ID, highPriority bool) {
	am.rediscoveryMu.Lock()
	defer am.rediscoveryMu.Unlock()

	now := time.Now()

	// 检查是否已在队列中
	if info, exists := am.rediscoveryQueue[pid]; exists {
		// 更新优先级
		if highPriority && info.Priority < 1 {
			info.Priority = 1
			if am.logger != nil {
				am.logger.Debugf("addr_manager rediscovery_priority_upgraded peer=%s", pid.String())
			}
		}
		return
	}

	// 添加到队列
	priority := 0
	if highPriority {
		priority = 1
	}

	// ✅ 有界化：队列满时淘汰一个低价值条目（低优先级/高失败/最久未尝试）
	if am.maxRediscoveryQueue > 0 && len(am.rediscoveryQueue) >= am.maxRediscoveryQueue {
		var victim libpeer.ID
		var victimScore int64
		first := true
		for id, info := range am.rediscoveryQueue {
			// bootstrap/近期连接的尽量不淘汰
			if am.isBootstrapPeer(id) {
				continue
			}
			// score 越大越“该淘汰”：priority低、fail多、LastAttempt老
			age := int64(time.Since(info.LastAttemptAt) / time.Second)
			score := int64((1-info.Priority)*100000) + int64(info.FailCount*1000) + age
			if first || score > victimScore {
				first = false
				victim = id
				victimScore = score
			}
		}
		if victim != "" {
			delete(am.rediscoveryQueue, victim)
			if am.logger != nil {
				am.logger.Warnf("addr_manager rediscovery_queue_full evict peer=%s size=%d max=%d",
					victim.String(), len(am.rediscoveryQueue), am.maxRediscoveryQueue)
			}
		} else {
			// 没有可淘汰对象（全是保护项），直接拒绝新入队
			if am.logger != nil {
				am.logger.Warnf("addr_manager rediscovery_queue_full drop peer=%s size=%d max=%d",
					pid.String(), len(am.rediscoveryQueue), am.maxRediscoveryQueue)
			}
			return
		}
	}

	am.rediscoveryQueue[pid] = &PeerRediscoveryInfo{
		PeerID:        pid,
		LastAttemptAt: now,
		FailCount:     0,
		Priority:      priority,
	}

	if am.logger != nil {
		am.logger.Infof("🔍 addr_manager rediscovery_enqueued peer=%s priority=%d",
			pid.String(), priority)
	}

	// 🆕 立即触发一次查询（异步，严格并发控制）
	select {
	case am.rediscoverySem <- struct{}{}:
		go func(p libpeer.ID) {
			defer func() { <-am.rediscoverySem }()
			
			ctx, cancel := context.WithTimeout(am.ctx, 30*time.Second)
			defer cancel()
			
			am.attemptRediscoveryWithContext(ctx, p)
		}(pid)
	default:
		// semaphore满了，下次周期再试
		if am.logger != nil {
			am.logger.Debugf("addr_manager rediscovery_enqueue_throttled peer=%s", pid.String())
		}
	}
}

// rediscoveryLoop 重发现周期循环
func (am *AddrManager) rediscoveryLoop() {
	ticker := time.NewTicker(am.rediscoveryInterval)
	defer ticker.Stop()

	if am.logger != nil {
		am.logger.Infof("addr_manager rediscovery_loop started interval=%s max_retries=%d backoff_base=%s",
			am.rediscoveryInterval, am.rediscoveryMaxRetries, am.rediscoveryBackoffBase)
	}

	for {
		select {
		case <-am.ctx.Done():
			if am.logger != nil {
				am.logger.Info("addr_manager rediscovery_loop stopped")
			}
			return
		case <-ticker.C:
			am.processRediscoveryQueue()
		}
	}
}

// processRediscoveryQueue 处理重发现队列（🆕 优化并发控制）
func (am *AddrManager) processRediscoveryQueue() {
	am.rediscoveryMu.Lock()

	// 收集需要重试的peer
	var toRetry []libpeer.ID
	now := time.Now()

	for pid, info := range am.rediscoveryQueue {
		// 🆕 检查是否达到最大重试次数
		if info.FailCount >= am.rediscoveryMaxRetries {
			// 移除
			delete(am.rediscoveryQueue, pid)
			if am.logger != nil {
				am.logger.Warnf("⚠️ addr_manager rediscovery_max_retries_reached peer=%s fail_count=%d (removed from queue)",
					pid.String(), info.FailCount)
			}
			continue
		}

		// 计算退避时间
		backoff := am.calculateBackoff(info.FailCount)
		if now.Sub(info.LastAttemptAt) < backoff {
			// 还在退避期，跳过
			continue
		}

		toRetry = append(toRetry, pid)
	}

	queueSize := len(am.rediscoveryQueue)
	am.rediscoveryMu.Unlock()

	if len(toRetry) == 0 {
		return
	}

	if am.logger != nil {
		am.logger.Debugf("addr_manager rediscovery_scan queue_size=%d retry_count=%d",
			queueSize, len(toRetry))
	}

	// 🆕 使用semaphore严格控制并发数，而不是一次性spawn多个goroutine
	// 这样可以防止goroutine泄漏，确保最多只有cap(rediscoverySem)个并发任务
	for _, pid := range toRetry {
		// 非阻塞尝试获取semaphore
		select {
		case am.rediscoverySem <- struct{}{}:
			// 获取成功，启动goroutine
			go func(p libpeer.ID) {
				defer func() { <-am.rediscoverySem }()
				
				// 🆕 为每次重发现添加30秒超时
				ctx, cancel := context.WithTimeout(am.ctx, 30*time.Second)
				defer cancel()
				
				am.attemptRediscoveryWithContext(ctx, p)
			}(pid)
		default:
			// semaphore满了，不再启动新的goroutine，下一轮再试
			if am.logger != nil {
				am.logger.Debugf("addr_manager rediscovery_semaphore_full skipping remaining_peers=%d",
					len(toRetry))
			}
			return
		}
	}
}

// calculateBackoff 计算退避时间（指数退避）
func (am *AddrManager) calculateBackoff(failCount int) time.Duration {
	// 指数退避：base * 2^failCount
	backoff := am.rediscoveryBackoffBase
	for i := 0; i < failCount && i < 5; i++ {
		backoff *= 2
	}
	// 上限：10分钟
	if backoff > 10*time.Minute {
		backoff = 10 * time.Minute
	}
	return backoff
}

// attemptRediscovery 尝试重发现单个peer（无超时版本，兼容旧调用）
func (am *AddrManager) attemptRediscovery(pid libpeer.ID) {
	ctx, cancel := context.WithTimeout(am.ctx, 30*time.Second)
	defer cancel()
	am.attemptRediscoveryWithContext(ctx, pid)
}

// 🆕 attemptRediscoveryWithContext 尝试重发现单个peer（带超时上下文）
func (am *AddrManager) attemptRediscoveryWithContext(ctx context.Context, pid libpeer.ID) {
	am.rediscoveryMu.Lock()
	info, exists := am.rediscoveryQueue[pid]
	if !exists {
		am.rediscoveryMu.Unlock()
		return
	}
	info.LastAttemptAt = time.Now()
	am.rediscoveryMu.Unlock()

	// 🆕 执行DHT FindPeer（带超时上下文）
	success := am.executeFindPeerWithContext(ctx, pid)

	am.rediscoveryMu.Lock()
	defer am.rediscoveryMu.Unlock()
	
	if success {
		// 成功：从队列移除
		delete(am.rediscoveryQueue, pid)
		if am.logger != nil {
			am.logger.Infof("✅ addr_manager rediscovery_success peer=%s", pid.String())
		}
	} else {
		// 失败：增加失败计数
		if info, exists := am.rediscoveryQueue[pid]; exists {
			info.FailCount++
			
			// 🆕 达到最大重试次数后立即移除（双重保险）
			if info.FailCount >= am.rediscoveryMaxRetries {
				delete(am.rediscoveryQueue, pid)
				if am.logger != nil {
					am.logger.Warnf("⚠️ addr_manager rediscovery_abandoned_after_max_retries peer=%s fail_count=%d",
						pid.String(), info.FailCount)
				}
				return
			}
			
			backoff := am.calculateBackoff(info.FailCount)
			if am.logger != nil {
				am.logger.Debugf("addr_manager rediscovery_failed peer=%s fail_count=%d next_backoff=%s",
					pid.String(), info.FailCount, backoff)
			}
		}
	}
}

// executeFindPeer 执行DHT FindPeer查询（无超时版本，兼容旧调用）
func (am *AddrManager) executeFindPeer(pid libpeer.ID) bool {
	ctx, cancel := context.WithTimeout(context.Background(), am.lookupTimeout)
	defer cancel()
	return am.executeFindPeerWithContext(ctx, pid)
}

// 🆕 executeFindPeerWithContext 执行DHT FindPeer查询（带超时上下文）
// 注意：此方法由外部调用者管理并发控制（rediscoverySem），不在内部再次获取
func (am *AddrManager) executeFindPeerWithContext(ctx context.Context, pid libpeer.ID) bool {
	if am.routing == nil {
		return false
	}

	addrInfo, err := am.routing.FindPeer(ctx, pid)
	if err != nil {
		// 超时或其他错误
		if ctx.Err() == context.DeadlineExceeded {
			if am.logger != nil {
				am.logger.Debugf("addr_manager rediscovery_timeout peer=%s", pid.String())
			}
		}
		return false
	}

	if len(addrInfo.Addrs) == 0 {
		return false
	}

	// 更新peerstore
	am.peerstore.AddAddrs(pid, am.capAddrs(addrInfo.Addrs), peerstore.TempAddrTTL)

	// 添加到地址管理器（使用DHTAddrTTL）
	am.AddDHTAddr(pid, addrInfo.Addrs)

	return true
}

// GetRediscoveryQueueSize 获取重发现队列大小（用于指标）
func (am *AddrManager) GetRediscoveryQueueSize() int {
	am.rediscoveryMu.RLock()
	defer am.rediscoveryMu.RUnlock()
	return len(am.rediscoveryQueue)
}

// RediscoveryQueueStats 重发现队列统计信息
type RediscoveryQueueStats struct {
	QueueSize         int     // 队列中peer总数
	HighPriorityCount int     // 高优先级peer数量
	FailedCount       int     // 失败次数>0的peer数量
	AvgFailCount      float64 // 平均失败次数
	MaxFailCount      int     // 最大失败次数
	OldestAttemptAge  int64   // 最久未尝试的peer年龄（秒）
}

// GetRediscoveryQueueStats 获取重发现队列健康统计信息
// 用于诊断接口和监控
func (am *AddrManager) GetRediscoveryQueueStats() RediscoveryQueueStats {
	am.rediscoveryMu.RLock()
	defer am.rediscoveryMu.RUnlock()

	stats := RediscoveryQueueStats{
		QueueSize: len(am.rediscoveryQueue),
	}

	if stats.QueueSize == 0 {
		return stats
	}

	now := time.Now()
	totalFailCount := 0
	maxAge := int64(0)

	for _, info := range am.rediscoveryQueue {
		// 统计高优先级
		if info.Priority > 0 {
			stats.HighPriorityCount++
		}

		// 统计失败数
		if info.FailCount > 0 {
			stats.FailedCount++
			totalFailCount += info.FailCount
			if info.FailCount > stats.MaxFailCount {
				stats.MaxFailCount = info.FailCount
			}
		}

		// 计算最久未尝试的年龄
		age := int64(now.Sub(info.LastAttemptAt) / time.Second)
		if age > maxAge {
			maxAge = age
		}
	}

	// 计算平均失败次数
	if stats.FailedCount > 0 {
		stats.AvgFailCount = float64(totalFailCount) / float64(stats.FailedCount)
	}

	stats.OldestAttemptAge = maxAge

	return stats
}

