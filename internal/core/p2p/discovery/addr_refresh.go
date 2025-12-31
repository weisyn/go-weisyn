package discovery

import (
	"time"

	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
)

// refreshLoop 主动刷新循环
//
// 定期检查所有peer的地址TTL，对即将过期的地址触发重新查询
func (am *AddrManager) refreshLoop() {
	ticker := time.NewTicker(am.refreshInterval)
	defer ticker.Stop()

	if am.logger != nil {
		am.logger.Infof("addr_manager refresh_loop started interval=%s", am.refreshInterval)
	}

	for {
		select {
		case <-am.ctx.Done():
			if am.logger != nil {
				am.logger.Infof("addr_manager refresh_loop stopped")
			}
			return

		case <-ticker.C:
			// ✅ 有界化：先做轻量淘汰，避免 peerstore/队列无界增长导致 RSS 逐步抬升
			am.enforceBounds()
			am.refreshAllPeers()
		}
	}
}

// refreshAllPeers 刷新所有peer地址
//
// 遍历peerstore中的所有peer，检查是否需要刷新地址
func (am *AddrManager) refreshAllPeers() {
	if am.logger != nil {
		am.logger.Debugf("addr_manager refresh_all_peers start")
	}

	// refresh 预算：避免每次遍历全部 peers 触发大量 trigger_lookup/日志/锁竞争
	budget := am.refreshBudget
	if budget <= 0 {
		budget = 1000
	}

	var refreshCount int
	var skipCount int
	var processed int

	// 1) 优先处理当前已连接 peers（通常数量较小、价值最高）
	if am.host != nil {
		for _, pid := range am.host.Network().Peers() {
			if processed >= budget {
				break
			}
			if pid == am.host.ID() {
				continue
			}

			// 🆕 P0-009: 已连接 peer 的地址需要“主动续期”，不依赖 FindPeer 成功。
			// 目的：避免长期连接的 peer 因 TTL 到期而在 peerstore 中变为 addrs=0。
			if addrs := am.peerstore.Addrs(pid); len(addrs) > 0 {
				am.peerstore.AddAddrs(pid, addrs, am.ttl.Connected)
				now := time.Now()
				am.mu.Lock()
				am.lastConnectedAt[pid] = now
				// 同时视为“仍可见”，避免被有界化逻辑当作长期未见候选淘汰
				am.lastSeenAt[pid] = now
				am.mu.Unlock()
			}

			if am.shouldRefresh(pid) {
				am.triggerAddrLookup(pid)
				refreshCount++
			} else {
				skipCount++
			}
			processed++
		}
	}

	// 2) 预算未用完时，按游标分片遍历 peerstore.Peers()，避免全量扫描
	if processed < budget {
		peers := am.peerstore.Peers()
		if len(peers) > 0 {
			am.mu.Lock()
			start := am.refreshCursor
			am.mu.Unlock()

			visited := 0
			for visited < len(peers) && processed < budget {
				p := peers[(start+visited)%len(peers)]
				visited++
				if p == "" || p == am.host.ID() {
					continue
				}
				// 连接的 peer 前面已处理过，避免重复
				if am.host != nil && am.host.Network().Connectedness(p) == libnetwork.Connected {
					continue
				}
				if am.shouldRefresh(p) {
					am.triggerAddrLookup(p)
					refreshCount++
				} else {
					skipCount++
				}
				processed++
			}

			am.mu.Lock()
			am.refreshCursor = (start + visited) % len(peers)
			am.mu.Unlock()
		}
	}

	if am.logger != nil {
		totalPeers := 0
		if am.peerstore != nil {
			totalPeers = len(am.peerstore.Peers())
		}
		am.logger.Infof("addr_manager refresh_all_peers done total=%d processed=%d budget=%d refresh=%d skip=%d",
			totalPeers, processed, budget, refreshCount, skipCount)
	}
}

// shouldRefresh 判断peer是否需要刷新
//
// 刷新策略：
// - 如果peer无地址，必须刷新
// - 如果距离上次刷新时间超过阈值（DHT TTL - RefreshThreshold），需要刷新
//
// 注意：由于libp2p的peerstore不提供查询TTL剩余时间的API，
// 我们使用启发式策略：根据最后刷新时间判断
func (am *AddrManager) shouldRefresh(id libpeer.ID) bool {
	// 获取地址
	addrs := am.peerstore.Addrs(id)

	// 无地址，必须刷新
	if len(addrs) == 0 {
		return true
	}

	// 检查最后刷新时间
	am.mu.RLock()
	lastRefresh, exists := am.lastRefreshAt[id]
	am.mu.RUnlock()

	if !exists {
		// 无刷新记录（可能是历史peer或持久化加载的），需要刷新
		return true
	}

	// 计算距离上次刷新的时间
	timeSinceRefresh := time.Since(lastRefresh)

	// 刷新策略精细化：
	// - 若 peer 当前已连接（或近期连接过），使用 Connected TTL 作为刷新窗口，避免对稳定连接的 peer 频繁 FindPeer；
	// - 否则使用 DHT TTL。
	ttl := am.ttl.DHT
	connectedNow := false
	if am.host != nil {
		connectedNow = am.host.Network().Connectedness(id) == libnetwork.Connected
	}
	am.mu.RLock()
	lastConn, hasConn := am.lastConnectedAt[id]
	am.mu.RUnlock()
	if connectedNow || (hasConn && !lastConn.IsZero() && time.Since(lastConn) < am.ttl.Connected) {
		ttl = am.ttl.Connected
	}

	// 如果距离上次刷新已超过 (ttl - refreshThreshold)，则需要刷新
	refreshDeadline := ttl - am.refreshThreshold
	if refreshDeadline <= 0 {
		// 兜底：阈值配置异常时，直接触发刷新
		return true
	}
	if timeSinceRefresh >= refreshDeadline {
		if am.logger != nil {
			am.logger.Debugf("addr_manager should_refresh peer=%s time_since_refresh=%s deadline=%s",
				id.String(), timeSinceRefresh, refreshDeadline)
		}
		return true
	}

	return false
}

