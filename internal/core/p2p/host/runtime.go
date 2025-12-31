// Package host provides libp2p Host construction for P2P module.
//
// This package implements Host building logic directly using p2pcfg.Options,
// migrating away from nodeconfig.NodeOptions dependency.
package host

import (
	"context"
	"crypto/x509"
	"fmt"
	"reflect"

	libp2p "github.com/libp2p/go-libp2p"
	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
)

// hostProvider 全局 Host 访问器，供 AutoRelay PeerSource 在运行时访问
var hostProvider func() lphost.Host

// setHostProvider 设置全局 Host 访问器
func setHostProvider(f func() lphost.Host) {
	hostProvider = f
}

// Runtime 负责 Host 的创建与关闭（使用 p2p.Options）
type Runtime struct {
	cfg                 *p2pcfg.Options
	logger              interface{} // 暂时不依赖 logger 接口，避免循环依赖
	host                lphost.Host
	connectionProtector *ConnectionProtector
	// caCertPool 存储联盟链的 CA 证书池（用于 mTLS 证书验证）
	// 仅在联盟链模式下使用，通过 withMTLSOptions() 加载
	caCertPool *x509.CertPool
}

// NewRuntime 创建新的 Host Runtime（使用 p2p.Options）
func NewRuntime(cfg *p2pcfg.Options) (*Runtime, error) {
	return &Runtime{
		cfg:                 cfg,
		connectionProtector: NewConnectionProtector(),
	}, nil
}

// Start 装配选项并启动 Host
func (r *Runtime) Start(ctx context.Context) error {
	if r.host != nil {
		return nil
	}

	// 构建连接选项
	opts := r.withAddressFactoryByConfig()

	// 创建主机
	h, err := r.newHost(ctx, opts)
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	r.host = h

	// 设置全局 Host 访问器，供 AutoRelay PeerSource 使用
	setHostProvider(func() lphost.Host { return r.host })

	// 保护引导/核心 peers，避免被连接管理器修剪
	if r.cfg != nil {
		for _, s := range r.cfg.BootstrapPeers {
			m, err := ma.NewMultiaddr(s)
			if err != nil {
				continue
			}
			if info, err := peer.AddrInfoFromP2pAddr(m); err == nil {
				if cm := r.host.ConnManager(); cm != nil {
					cm.Protect(info.ID, "bootstrap")
				}
			}
		}
	}

	return nil
}

// Stop 关闭 Host 和所有相关服务
func (r *Runtime) Stop(ctx context.Context) error {
	if r.host != nil {
		_ = r.host.Close()
		r.host = nil
	}
	return nil
}

// Host 返回内部 host
func (r *Runtime) Host() lphost.Host {
	return r.host
}

// GetConnectionProtector 返回连接保护器
func (r *Runtime) GetConnectionProtector() *ConnectionProtector {
	return r.connectionProtector
}

// ============= 实现内部接口 =============

// 编译期保证实现 BandwidthProvider 接口
var _ interfaces.BandwidthProvider = (*Runtime)(nil)

// BandwidthReporter 返回带宽统计 Reporter（实现 BandwidthProvider 接口）
func (r *Runtime) BandwidthReporter() metrics.Reporter {
	return getBandwidthCounter()
}

// 编译期保证实现 ResourceManagerInspector 接口
var _ interfaces.ResourceManagerInspector = (*Runtime)(nil)

// ResourceManagerLimits 返回 ResourceManager 限额信息（实现 ResourceManagerInspector 接口）
func (r *Runtime) ResourceManagerLimits() map[string]interface{} {
	rm := CurrentResourceManager()
	if rm == nil {
		return map[string]interface{}{"enabled": false}
	}

	limits, hasLimits := CurrentRcmgrLimits()
	if !hasLimits {
		return map[string]interface{}{"enabled": true}
	}

	// 使用反射访问未导出字段
	result := map[string]interface{}{
		"enabled": true,
	}

	// 通过反射访问 limits 的 system 字段
	limitsValue := reflect.ValueOf(limits)
	if limitsValue.Kind() == reflect.Struct {
		systemField := limitsValue.FieldByName("system")
		if systemField.IsValid() && systemField.Kind() == reflect.Struct {
			// 辅助函数：提取 LimitVal 的值
			extractLimitValue := func(field reflect.Value) interface{} {
				if !field.IsValid() {
					return nil
				}
				// 尝试直接获取 int/int64 值
				switch field.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					return field.Int()
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					return int64(field.Uint())
				default:
					// 如果是自定义类型（如 LimitVal），尝试获取底层值
					if field.CanInterface() {
						// 尝试转换为 int64
						if field.Type().Kind() == reflect.Int64 {
							return field.Int()
						}
						// 尝试通过方法获取值
						if field.CanAddr() {
							addrField := field.Addr()
							if method := addrField.MethodByName("Int64"); method.IsValid() {
								results := method.Call(nil)
								if len(results) > 0 {
									return results[0].Int()
								}
							}
						}
					}
				}
				return nil
			}

			// 访问 system 的各个字段
			if memoryField := systemField.FieldByName("Memory"); memoryField.IsValid() {
				if val := extractLimitValue(memoryField); val != nil {
					result["system_memory_bytes"] = val
				}
			}
			if fdField := systemField.FieldByName("FD"); fdField.IsValid() {
				if val := extractLimitValue(fdField); val != nil {
					result["system_fd"] = val
				}
			}
			if connsField := systemField.FieldByName("Conns"); connsField.IsValid() {
				if val := extractLimitValue(connsField); val != nil {
					result["system_conns"] = val
				}
			}
			if connsInboundField := systemField.FieldByName("ConnsInbound"); connsInboundField.IsValid() {
				if val := extractLimitValue(connsInboundField); val != nil {
					result["system_conns_inbound"] = val
				}
			}
			if connsOutboundField := systemField.FieldByName("ConnsOutbound"); connsOutboundField.IsValid() {
				if val := extractLimitValue(connsOutboundField); val != nil {
					result["system_conns_outbound"] = val
				}
			}
			if streamsField := systemField.FieldByName("Streams"); streamsField.IsValid() {
				if val := extractLimitValue(streamsField); val != nil {
					result["system_streams"] = val
				}
			}
			if streamsInboundField := systemField.FieldByName("StreamsInbound"); streamsInboundField.IsValid() {
				if val := extractLimitValue(streamsInboundField); val != nil {
					result["system_streams_inbound"] = val
				}
			}
			if streamsOutboundField := systemField.FieldByName("StreamsOutbound"); streamsOutboundField.IsValid() {
				if val := extractLimitValue(streamsOutboundField); val != nil {
					result["system_streams_outbound"] = val
				}
			}
		}
	}

	return result
}

// newHost 根据 p2p 配置装配 libp2p Host
func (r *Runtime) newHost(ctx context.Context, extra ...libp2p.Option) (lphost.Host, error) {
	var opts []libp2p.Option

	// 传输/安全/复用（直接使用 p2p 配置）
	opts = append(opts, r.withTransportOptions()...)
	opts = append(opts, r.withSecurityOptions()...)
	opts = append(opts, r.withMuxerOptions()...)

	// 私有网络（PSK）
	opts = append(opts, r.withPrivateNetworkOptions()...)

	// 连接/资源/带宽/身份（直接使用 p2p 配置）
	opts = append(opts, r.withConnectionManagerOptions()...)
	opts = append(opts, r.withResourceManagerOptions()...)
	opts = append(opts, r.withBandwidthLimiterOptions()...)
	opts = append(opts, r.withIdentityOptions()...)

	// Identify 协议配置（ObservedAddr 激活阈值）
	opts = append(opts, r.withIdentifyOptions()...)

	// 地址过滤（直接使用 p2p 配置）
	opts = append(opts, r.withAdvancedAddressFiltering()...)

	// AutoNAT 服务（直接使用 p2p 配置）
	opts = append(opts, r.withAutoNATServiceOptions()...)

	// Connectivity 选项（直接使用 p2p 配置）
	opts = append(opts, r.withNATPortMapOptions()...)
	opts = append(opts, r.withReachabilityOptions()...)
	opts = append(opts, r.withRelayTransportOptions()...)
	// 动态 AutoRelay（零配置启用）优先于静态
	opts = append(opts, r.withAutoRelayDynamicOptions()...)
	opts = append(opts, r.withAutoRelayStaticOptions()...)
	opts = append(opts, r.withHolePunchingOptions()...)
	opts = append(opts, r.withRelayServiceOptions()...)

	// 监听地址
	if r.cfg == nil || len(r.cfg.ListenAddrs) == 0 {
		fallback := []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/udp/0/quic-v1",
		}
		opts = append(opts, libp2p.ListenAddrStrings(fallback...))
	} else {
		addrs := r.cfg.ListenAddrs
		addrs = r.enrichListenAddresses(addrs)
		opts = append(opts, libp2p.ListenAddrStrings(addrs...))
	}

	opts = append(opts, extra...)
	return libp2p.New(opts...)
}

// enrichListenAddresses 在启用 QUIC/WS 时自动补全监听 multiaddrs
func (r *Runtime) enrichListenAddresses(base []string) []string {
	hasQUIC := r.cfg != nil && r.cfg.EnableQUIC
	hasWS := r.cfg != nil && r.cfg.EnableWebSocket
	if !hasQUIC && !hasWS {
		return base
	}

	existing := make(map[string]struct{}, len(base))
	for _, s := range base {
		existing[s] = struct{}{}
	}

	for _, s := range base {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if hasQUIC {
			if _, err := m.ValueForProtocol(ma.P_TCP); err == nil {
				if ip4, err := m.ValueForProtocol(ma.P_IP4); err == nil && ip4 != "" {
					port, _ := m.ValueForProtocol(ma.P_TCP)
					quicStr := "/ip4/" + ip4 + "/udp/" + port + "/quic-v1"
					if _, ok := existing[quicStr]; !ok {
						base = append(base, quicStr)
						existing[quicStr] = struct{}{}
					}
				}
				if ip6, err := m.ValueForProtocol(ma.P_IP6); err == nil && ip6 != "" {
					port, _ := m.ValueForProtocol(ma.P_TCP)
					quicStr := "/ip6/" + ip6 + "/udp/" + port + "/quic-v1"
					if _, ok := existing[quicStr]; !ok {
						base = append(base, quicStr)
						existing[quicStr] = struct{}{}
					}
				}
			}
		}
		if hasWS {
			if _, err := m.ValueForProtocol(ma.P_TCP); err == nil {
				wsStr := s + "/ws"
				if _, ok := existing[wsStr]; !ok {
					base = append(base, wsStr)
					existing[wsStr] = struct{}{}
				}
			}
		}
	}
	return base
}
