package host

import (
	"context"
	"fmt"
	"sync"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
)

var (
	// sharedBandwidthCounter 共享带宽计数器，用于跨组件观测
	sharedBandwidthCounter metrics.Reporter
	bandwidthCounterOnce   sync.Once
)

// getBandwidthCounter 获取共享带宽计数器
func getBandwidthCounter() metrics.Reporter {
	bandwidthCounterOnce.Do(func() {
		sharedBandwidthCounter = metrics.NewBandwidthCounter()
	})
	return sharedBandwidthCounter
}

// HostRuntime 包含 Host 和 Runtime 的引用
type HostRuntime struct {
	Host    lphost.Host
	Runtime *Runtime // 使用新的 P2P Runtime
}

// BuildHost 根据 P2P 配置构建 libp2p Host
//
// 直接使用 p2pcfg.Options，不再依赖 nodeconfig.NodeOptions
func BuildHost(ctx context.Context, p2pOpts *p2pcfg.Options) (lphost.Host, error) {
	hr, err := BuildHostWithRuntime(ctx, p2pOpts)
	if err != nil {
		return nil, err
	}
	return hr.Host, nil
}

// BuildHostWithRuntime 根据 P2P 配置构建 libp2p Host 和 Runtime
//
// 返回 Host 和 Runtime 的引用，以便访问 ConnectionProtector 等内部组件
func BuildHostWithRuntime(ctx context.Context, p2pOpts *p2pcfg.Options) (*HostRuntime, error) {
	// 使用新的 P2P Runtime（直接使用 p2pcfg.Options）
	runtime, err := NewRuntime(p2pOpts)
	if err != nil {
		return nil, fmt.Errorf("create p2p host runtime: %w", err)
	}

	// 启动 Runtime 来构建 Host
	if err := runtime.Start(ctx); err != nil {
		return nil, fmt.Errorf("start p2p host runtime: %w", err)
	}

	hostRef := runtime.Host()

	// AutoRelay 动态 PeerSource 通过 Runtime.host 直接访问（无需全局 provider）

	return &HostRuntime{
		Host:    hostRef,
		Runtime: runtime, // 使用新的 P2P Runtime
	}, nil
}

// GetBandwidthCounter 获取共享带宽计数器（供其他模块使用）
func GetBandwidthCounter() metrics.Reporter {
	return getBandwidthCounter()
}
