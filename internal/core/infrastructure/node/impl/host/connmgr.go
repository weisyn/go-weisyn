package host

import (
	"strings"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/pbnjay/memory"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// 连接/资源/带宽管理：
// - ConnectionManager：通过低/高水位与宽限期定期修剪连接；
// - ResourceManager：按内存/FD/连接/流等维度限额，支持 AutoScale；
// - 带宽计数：复用共享计数器提供跨组件观测。
var sharedBandwidthCounter = metrics.NewBandwidthCounter()

// currentResourceManager 保存当前主机使用的资源管理器，便于 diagnostics 暴露统计
var currentResourceManager network.ResourceManager

// currentRcmgrLimits 保存已计算的 rcmgr 具体限额（用于摘要与指标）
var currentRcmgrLimits rcmgr.ConcreteLimitConfig
var hasCurrentRcmgrLimits bool

// CurrentResourceManager 返回当前资源管理器实例
func CurrentResourceManager() network.ResourceManager { return currentResourceManager }

// CurrentRcmgrLimits 返回当前 rcmgr 限额（如可用）
func CurrentRcmgrLimits() (rcmgr.ConcreteLimitConfig, bool) {
	return currentRcmgrLimits, hasCurrentRcmgrLimits
}

func withConnectionManagerOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option

	if cfg == nil {
		cm, _ := connmgr.NewConnManager(20, 200, connmgr.WithGracePeriod(20*time.Second))
		return []libp2p.Option{libp2p.ConnectionManager(cm)}
	}

	lowWater := cfg.Connectivity.LowWater
	if lowWater <= 0 {
		lowWater = cfg.Connectivity.MinPeers
		if lowWater <= 0 {
			lowWater = 20
		}
	}

	highWater := cfg.Connectivity.HighWater
	if highWater <= 0 {
		highWater = cfg.Connectivity.MaxPeers
		if highWater <= 0 {
			highWater = 200
		}
	}

	gracePeriod := cfg.Connectivity.GracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 20 * time.Second
	}

	cm, err := connmgr.NewConnManager(
		lowWater,
		highWater,
		connmgr.WithGracePeriod(gracePeriod),
	)
	if err != nil {
		cm, _ = connmgr.NewConnManager(20, 200, connmgr.WithGracePeriod(20*time.Second))
	}

	opts = append(opts, libp2p.ConnectionManager(cm))
	return opts
}

// withResourceManagerOptions 采用 Kubo 风格的自适应资源管理器
func withResourceManagerOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	if cfg == nil {
		return []libp2p.Option{}
	}

	// 本地诊断旁路：当仅本地环回监听且开启诊断时，使用无限限额，方便本地压测/调试
	if cfg.Host.DiagnosticsEnabled {
		loopbackOnly := true
		for _, a := range cfg.Host.ListenAddresses {
			if !strings.Contains(a, "/ip4/127.0.0.1/") && !strings.Contains(a, "/ip4/127.0.0.1") {
				loopbackOnly = false
				break
			}
		}
		if loopbackOnly {
			limiter := rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits)
			rm, err := rcmgr.NewResourceManager(limiter)
			if err == nil {
				currentResourceManager = rm
				hasCurrentRcmgrLimits = false
				return []libp2p.Option{libp2p.ResourceManager(rm)}
			}
		}
	}

	rm := createAdaptiveResourceManager(cfg)
	if rm != nil {
		currentResourceManager = rm
		return []libp2p.Option{libp2p.ResourceManager(rm)}
	}

	return []libp2p.Option{}
}

// createAdaptiveResourceManager Kubo 风格：自适应默认、白名单、调试追踪、与 ConnMgr 高水位的合理性检查
func createAdaptiveResourceManager(cfg *nodeconfig.NodeOptions) network.ResourceManager {
	// 资源管理器默认启用，由配置驱动限额（不依赖环境变量分叉）

	// 计算自适应默认值：以内存/FD 为基准
	maxMemory := int64(memory.TotalMemory()) / 2 // 默认使用系统内存的 50%
	maxFD := 1024                                // 保守默认
	if rc := &cfg.Connectivity.Resources; rc != nil {
		if v := rc.MemoryLimitMB; v > 0 {
			maxMemory = int64(v) * 1024 * 1024
		}
		if v := rc.MaxFileDescriptors; v > 0 {
			maxFD = v
		}
	}

	// 基于自适应默认构建 PartialLimitConfig
	partial := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Memory:          rcmgr.LimitVal64(maxMemory),
			FD:              rcmgr.LimitVal(maxFD),
			Conns:           rcmgr.Unlimited,
			ConnsInbound:    rcmgr.LimitVal(maxMemory / (1024 * 1024)), // 1 conn / MB
			ConnsOutbound:   rcmgr.Unlimited,
			Streams:         rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
		},
		Transient: rcmgr.ResourceLimits{
			Memory:          rcmgr.LimitVal64(maxMemory / 4),
			FD:              rcmgr.LimitVal(maxFD / 4),
			Conns:           rcmgr.Unlimited,
			ConnsInbound:    rcmgr.LimitVal(maxMemory / (1024 * 1024 * 4)),
			ConnsOutbound:   rcmgr.Unlimited,
			Streams:         rcmgr.Unlimited,
			StreamsOutbound: rcmgr.Unlimited,
			StreamsInbound:  rcmgr.Unlimited,
		},
	}

	// 合并 libp2p 的 DefaultLimits（自动缩放）
	limits := partial.Build(rcmgr.DefaultLimits.Scale(maxMemory, maxFD)).ToPartialLimitConfig()

	// 与连接管理器高水位的合理性检查：入站连接 >= 2x HighWater，且不低于 256
	highWater := cfg.Connectivity.HighWater
	if highWater <= 0 {
		highWater = 200
	}
	if limits.System.ConnsInbound > rcmgr.DefaultLimit {
		minInbound := int64(highWater * 2)
		if minInbound < 256 {
			minInbound = 256
		}
		if int64(limits.System.ConnsInbound) < minInbound {
			limits.System.ConnsInbound = rcmgr.LimitVal(minInbound)
		}
	}

	var rOpts []rcmgr.Option

	// 记录当前 limits 用于 diagnostics 摘要
	currentRcmgrLimits = limits.Build(rcmgr.ConcreteLimitConfig{})
	hasCurrentRcmgrLimits = true

	limiter := rcmgr.NewFixedLimiter(currentRcmgrLimits)
	rm, err := rcmgr.NewResourceManager(limiter, rOpts...)
	if err != nil {
		return nil
	}
	return rm
}

func withBandwidthLimiterOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	// 复用共享带宽计数器，便于在 DiagnosticsManager 中读取
	return []libp2p.Option{libp2p.BandwidthReporter(sharedBandwidthCounter)}
}
