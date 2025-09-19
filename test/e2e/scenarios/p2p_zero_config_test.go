//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/fx"

	p2pmod "github.com/weisyn/v1/internal/core/infrastructure/node"
	hostpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/host"
)

// startP2PApp 启动一个仅包含 P2P 模块的 fx.App，并返回 HostRuntime 句柄与停止函数。
func startP2PApp(t *testing.T) (*fx.App, *hostpkg.Runtime, func(context.Context) error) {
	t.Helper()
	var runtime *hostpkg.Runtime
	app := fx.New(
		p2pmod.Module(),
		fx.Populate(&runtime),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := app.Start(ctx); err != nil {
		t.Fatalf("start app: %v", err)
	}
	stop := func(ctx context.Context) error { return app.Stop(ctx) }
	return app, runtime, stop
}

// TestZeroConfigStarts 验证零配置场景：Host 能启动且暴露非回环地址。
func TestZeroConfigStarts(t *testing.T) {
	// 显式关闭内置引导，避免外部网络依赖
	_ = os.Setenv("WES_P2P_ENABLE_DEFAULT_BOOTSTRAPS", "false")
	_, rt, stop := startP2PApp(t)
	defer func() { _ = stop(context.Background()) }()

	host := rt.Host()
	if host == nil {
		t.Fatalf("host is nil")
	}
	// 最多等待 5 秒给监听与发现完成初始化
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		addrs := host.Addrs()
		if len(addrs) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(host.Addrs()) == 0 {
		t.Fatalf("expected non-empty host addrs, got 0")
	}
}

// TestZeroConfigLANConnect 验证两个零配置节点在同一网络中可在短时间内互联。
// 注意：在 CI 等无多播环境下可能不稳定，可通过 -run 过滤或在本地执行。
func TestZeroConfigLANConnect(t *testing.T) {
	if os.Getenv("WES_E2E_RUN_MDNS") != "true" {
		t.Skip("skip mdns connectivity test without WES_E2E_RUN_MDNS=true")
	}
	_ = os.Setenv("WES_P2P_ENABLE_DEFAULT_BOOTSTRAPS", "false")
	_, rt1, stop1 := startP2PApp(t)
	defer func() { _ = stop1(context.Background()) }()
	_, rt2, stop2 := startP2PApp(t)
	defer func() { _ = stop2(context.Background()) }()

	h1 := rt1.Host()
	h2 := rt2.Host()
	if h1 == nil || h2 == nil {
		t.Fatalf("hosts not ready")
	}
	// 等待 mDNS/DHT 建立连接，最多 30 秒
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		peers1 := h1.Network().Peers()
		peers2 := h2.Network().Peers()
		if containsPeer(peers1, h2.ID()) || containsPeer(peers2, h1.ID()) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("peers did not connect via zero-config discovery within timeout")
}

func containsPeer(list []string, id string) bool {
	for _, p := range list {
		if p == id {
			return true
		}
	}
	return false
}
