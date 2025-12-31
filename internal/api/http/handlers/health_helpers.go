package handlers

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// checkDatabase 检查数据库连接状态
//
// 实现细节：
// - 调用 Repository.GetHighestBlock 测试连接
// - 测量延迟
// - 返回状态和延迟信息
func (h *HealthHandler) checkDatabase(ctx context.Context) map[string]interface{} {
	start := time.Now()

	if h.blockQuery == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "block query service not available",
		}
	}

	// 尝试查询最高块以测试数据库连接
	_, _, err := h.blockQuery.GetHighestBlock(ctx)
	latency := time.Since(start)

	if err != nil {
		h.logger.Warn("Database health check failed",
			zap.Error(err),
			zap.Duration("latency", latency))
		return map[string]interface{}{
			"status":     "unhealthy",
			"latency_ms": latency.Milliseconds(),
			"error":      err.Error(),
		}
	}

	return map[string]interface{}{
		"status":     "healthy",
		"latency_ms": latency.Milliseconds(),
	}
}

// checkBlockchain 检查区块链状态
//
// 实现细节：
// - 调用 ChainService.IsDataFresh 检查同步状态
// - 调用 ChainService.GetChainInfo 获取当前高度
// - 返回状态和高度信息
func (h *HealthHandler) checkBlockchain(ctx context.Context) map[string]interface{} {
	if h.chainQuery == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "chain query service not available",
		}
	}

	// 检查是否同步完成
	fresh, err := h.chainQuery.IsDataFresh(ctx)
	if err != nil {
		h.logger.Warn("Blockchain health check failed",
			zap.Error(err))
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	// 获取链信息
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		h.logger.Warn("Failed to get chain info",
			zap.Error(err))
		return map[string]interface{}{
			"status":  "degraded",
			"syncing": !fresh,
			"error":   err.Error(),
		}
	}

	status := "healthy"
	if !fresh {
		status = "syncing"
	}

	return map[string]interface{}{
		"status":        status,
		"syncing":       !fresh,
		"currentHeight": chainInfo.Height,
		"tipAge":        time.Since(time.Unix(chainInfo.LastBlockTime, 0)).String(),
	}
}

// checkP2P 检查P2P网络状态
//
// 实现细节：
// - 从 ChainInfo.PeerCount 获取对等节点数量
// - 返回状态和节点数量信息
func (h *HealthHandler) checkP2P(ctx context.Context) map[string]interface{} {
	if h.chainQuery == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "chain query service not available",
		}
	}

	// 从ChainInfo获取对等节点数量
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		h.logger.Warn("P2P health check failed",
			zap.Error(err))
		return map[string]interface{}{
			"status":    "unhealthy",
			"peerCount": 0,
			"error":     err.Error(),
		}
	}

	peerCount := chainInfo.PeerCount
	status := "healthy"
	if peerCount == 0 {
		status = "degraded"
	}

	return map[string]interface{}{
		"status":    status,
		"peerCount": peerCount,
	}
}

// checkMempool 检查内存池状态
//
// 实现细节：
// - 调用 TxPool.GetPendingTransactions 获取交易数量
// - 返回状态和交易数量信息
func (h *HealthHandler) checkMempool(ctx context.Context) map[string]interface{} {
	if h.mempool == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "mempool not available",
		}
	}

	// 获取待处理交易
	pendingTxs, err := h.mempool.GetPendingTransactions()
	if err != nil {
		h.logger.Warn("Mempool health check failed",
			zap.Error(err))
		return map[string]interface{}{
			"status":  "unhealthy",
			"txCount": 0,
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"status":  "healthy",
		"txCount": len(pendingTxs),
	}
}

// determineReadiness 根据组件状态确定就绪状态
//
// 实现细节：
// - 检查所有组件是否健康
// - 返回 "ready" 或 "not_ready"
func (h *HealthHandler) determineReadiness(components map[string]interface{}) string {
	for _, component := range components {
		if comp, ok := component.(map[string]interface{}); ok {
			if status, ok := comp["status"].(string); ok {
				if status == "unhealthy" {
					return "not_ready"
				}
			}
		}
	}
	return "ready"
}

// isDatabaseReady 检查数据库是否就绪
func (h *HealthHandler) isDatabaseReady(ctx context.Context) bool {
	if h.blockQuery == nil {
		return false
	}

	_, _, err := h.blockQuery.GetHighestBlock(ctx)
	return err == nil
}

// isP2PReady 检查P2P网络是否就绪（至少1个对等节点）
func (h *HealthHandler) isP2PReady(ctx context.Context) bool {
	if h.chainQuery == nil {
		return false
	}

	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return false
	}

	return chainInfo.PeerCount > 0
}

// isSyncComplete 检查同步是否完成
func (h *HealthHandler) isSyncComplete(ctx context.Context) bool {
	if h.chainQuery == nil {
		return false
	}

	fresh, err := h.chainQuery.IsDataFresh(ctx)
	if err != nil {
		return false
	}

	return fresh
}

// isMempoolReady 检查内存池是否就绪
func (h *HealthHandler) isMempoolReady(ctx context.Context) bool {
	if h.mempool == nil {
		return false
	}

	_, err := h.mempool.GetPendingTransactions()
	return err == nil
}
