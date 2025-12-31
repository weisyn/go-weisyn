// health_metrics.go - 节点健康度监控指标
// 提供节点健康度统计和监控接口
package sync

// GetPeerHealthMetrics 获取节点健康度指标（供监控使用）
//
// 返回值：
//   - map[string]interface{}: 包含健康度统计信息
//     - total_tracked_peers: 被跟踪的节点总数
//     - healthy_peers: 健康节点数量
//     - circuit_broken_peers: 熔断中的节点数量
func GetPeerHealthMetrics() map[string]interface{} {
	peerHealthMutex.RLock()
	defer peerHealthMutex.RUnlock()

	healthyCount := 0
	brokenCount := 0
	
	for _, health := range peerHealthMap {
		if health.IsCircuitBroken {
			brokenCount++
		} else {
			healthyCount++
		}
	}

	return map[string]interface{}{
		"total_tracked_peers":  len(peerHealthMap),
		"healthy_peers":        healthyCount,
		"circuit_broken_peers": brokenCount,
	}
}

