// Package transport provides health check functionality for client transport.
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status string `json:"status"`
}

// WaitForNodeReady 等待节点就绪
// endpoint: 节点 API 端点（如 http://localhost:28680）
// timeout: 超时时间
//
// 语义说明（CLI 启动器场景）：
//   - 对于本地开发/单节点私链，我们更关心「HTTP 服务是否已经可用」，
//     而不是「P2P 是否有对等节点、是否完全同步等生产级就绪条件」。
//   - 因此这里采用 **宽松语义**：
//     1. 必须通过 `/api/v1/health/live`（进程存活 + HTTP 服务可用）
//     2. `/api/v1/health/ready` 仅作为附加检查，失败时打印日志但不阻塞 CLI 进入交互界面
//
// 返回:
//   - 当在 timeout 内检测到 live OK 时返回 nil
//   - 超时或上下文取消时返回错误
func WaitForNodeReady(ctx context.Context, endpoint string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("等待节点就绪超时（%v）", timeout)
			}

			// 1. 必须先通过存活检查
			if err := checkLiveness(client, endpoint); err != nil {
				// 存活检查未通过，继续下一轮重试
				continue
			}

			// 2. 存活检查通过后，尝试做一次就绪检查（非强制）
			if err := checkReadiness(client, endpoint); err != nil {
				// 在本地开发场景下，就绪检查不通过通常是：
				//   - P2P 尚未有对等节点
				//   - 链尚未完全同步
				// 这些不会影响 CLI 的基本使用，这里仅记录日志提示。
				log.Printf("节点存活，但就绪检查未通过（继续启动 CLI）: %v", err)
			}

			// 3. 只要 live OK，则认为节点已「足够可用」，允许 CLI 继续
			return nil
		}
	}
}

// CheckNodeHealth 检查节点健康状态（一次性）
func CheckNodeHealth(endpoint string) error {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// 检查存活
	if err := checkLiveness(client, endpoint); err != nil {
		return fmt.Errorf("存活检查失败: %w", err)
	}

	// 检查就绪
	if err := checkReadiness(client, endpoint); err != nil {
		return fmt.Errorf("就绪检查失败: %w", err)
	}

	return nil
}

// checkLiveness 检查节点存活状态
func checkLiveness(client *http.Client, endpoint string) error {
	url := endpoint + "/api/v1/health/live"
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("状态码: %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return err
	}

	if health.Status != "ok" {
		return fmt.Errorf("健康状态: %s", health.Status)
	}

	return nil
}

// checkReadiness 检查节点就绪状态
func checkReadiness(client *http.Client, endpoint string) error {
	url := endpoint + "/api/v1/health/ready"
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("状态码: %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return err
	}

	if health.Status != "ready" {
		return fmt.Errorf("就绪状态: %s", health.Status)
	}

	return nil
}
