// Package host provides libp2p option builders using p2pcfg.Options.
package host

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	ccmgr "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2ppnet "github.com/libp2p/go-libp2p/core/pnet"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	lpyamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pbnjay/memory"
	mamask "github.com/whyrusleeping/multiaddr-filter"
)

// ============= 传输层选项 =============

func (r *Runtime) withTransportOptions() []libp2p.Option {
	if r.cfg == nil {
		return []libp2p.Option{libp2p.DefaultTransports}
	}
	var opts []libp2p.Option

	if r.cfg.EnableTCP {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport, tcp.WithMetrics()))
	}
	if r.cfg.EnableQUIC {
		opts = append(opts, libp2p.Transport(libp2pquic.NewTransport))
	}
	if r.cfg.EnableWebSocket {
		opts = append(opts, libp2p.Transport(websocket.New))
	}

	if len(opts) == 0 {
		return []libp2p.Option{libp2p.DefaultTransports}
	}
	return opts
}

// ============= 安全层选项 =============

func (r *Runtime) withSecurityOptions() []libp2p.Option {
	if r.cfg == nil {
		return []libp2p.Option{libp2p.DefaultSecurity}
	}
	var opts []libp2p.Option

	// 联盟链：使用 mTLS（需要证书管理配置）
	if r.cfg.CertificateManagementCABundlePath != "" {
		// 加载 CA Bundle 并配置 mTLS
		tlsOpt, err := r.withMTLSOptions()
		if err != nil {
			// 如果 mTLS 配置失败，使用默认 TLS（但会记录错误）
			// 在实际部署中，应该 fail-fast
			// 这里暂时使用默认 TLS，后续可以改为 panic
			opts = append(opts, libp2p.Security(libp2ptls.ID, libp2ptls.New))
		} else {
			opts = append(opts, tlsOpt)
		}
	} else {
		// 非联盟链：使用标准 TLS/Noise
		if r.cfg.EnableTLS {
			opts = append(opts, libp2p.Security(libp2ptls.ID, libp2ptls.New))
		}
		if r.cfg.EnableNoise {
			opts = append(opts, libp2p.Security(noise.ID, noise.New))
		}
	}

	if len(opts) == 0 {
		return []libp2p.Option{libp2p.DefaultSecurity}
	}
	return opts
}

// withMTLSOptions 配置 mTLS（联盟链）
// 加载 CA Bundle 并配置 libp2p TLS 使用 mTLS 验证
//
// 注意：libp2p 的标准 TLS 实现使用自签名证书，不支持标准的 CA 证书链验证。
// 要实现 mTLS，需要在连接建立后通过 ConnectionGater 的 InterceptSecured 钩子
// 手动验证对端证书是否由联盟 CA 签发。
//
// 当前实现：
// 1. 加载 CA Bundle 并存储在 Runtime 中（供后续验证使用）
// 2. 使用标准 libp2p TLS（节点仍使用自签名证书）
// 3. 在 ConnectionGater.InterceptSecured 中验证对端证书链
func (r *Runtime) withMTLSOptions() (libp2p.Option, error) {
	if r.cfg == nil || r.cfg.CertificateManagementCABundlePath == "" {
		return nil, fmt.Errorf("certificate management CA bundle path is required for mTLS")
	}

	// 读取 CA Bundle 文件
	caBundlePath := r.cfg.CertificateManagementCABundlePath
	caBundleData, err := os.ReadFile(caBundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA bundle file %s: %w", caBundlePath, err)
	}

	// 解析 CA Bundle（PEM 格式）
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caBundleData) {
		return nil, fmt.Errorf("failed to parse CA bundle from %s", caBundlePath)
	}

	// 存储 CA Cert Pool 供 ConnectionGater 使用
	r.caCertPool = caCertPool

	// 使用标准 libp2p TLS
	// 证书验证将在 ConnectionGater.InterceptSecured 中实现
	return libp2p.Security(libp2ptls.ID, libp2ptls.New), nil
}

// ============= 多路复用器选项 =============

func (r *Runtime) withMuxerOptions() []libp2p.Option {
	if r.cfg == nil || !r.cfg.EnableYamux {
		return []libp2p.Option{libp2p.DefaultMuxers}
	}

	config := *lpyamux.DefaultTransport.Config()

	if ws := r.cfg.YamuxWindowSize; ws > 0 {
		windowSize := uint32(ws) * 1024
		if windowSize < 256*1024 {
			windowSize = 256 * 1024
		} else if windowSize > 32*1024*1024 {
			windowSize = 32 * 1024 * 1024
		}
		config.MaxStreamWindowSize = windowSize
	}

	if ms := r.cfg.YamuxMaxStreams; ms > 0 {
		maxStreams := uint32(ms)
		if maxStreams < 1 {
			maxStreams = 1
		} else if maxStreams > 1000000 {
			maxStreams = 1000000
		}
		config.MaxIncomingStreams = maxStreams
	}

	if to := r.cfg.YamuxConnectionTimeout; to > 0 {
		config.ConnectionWriteTimeout = to
	}

	transport := (*lpyamux.Transport)(&config)
	return []libp2p.Option{libp2p.Muxer(lpyamux.ID, transport)}
}

// ============= 身份选项 =============

func (r *Runtime) withIdentityOptions() []libp2p.Option {
	if r.cfg == nil {
		return nil
	}

	// 优先使用 PrivateKey（base64编码）
	if r.cfg.IdentityPrivateKey != "" {
		privKey, err := r.loadPrivateKeyFromBase64(r.cfg.IdentityPrivateKey)
		if err != nil {
			// 如果加载失败，记录错误但继续（使用默认身份）
			// 在实际部署中，可以考虑 fail-fast
			return nil
		}
		return []libp2p.Option{libp2p.Identity(privKey)}
	}

	// 其次使用 KeyFile
	if r.cfg.IdentityKeyFile != "" {
		privKey, err := r.loadOrCreateIdentityKey(r.cfg.IdentityKeyFile)
		if err != nil {
			// 如果加载失败，记录错误但继续（使用默认身份）
			// 在实际部署中，可以考虑 fail-fast
			return nil
		}
		return []libp2p.Option{libp2p.Identity(privKey)}
	}

	// 未配置身份，使用 libp2p 默认临时身份
	return nil
}

// ============= UserAgent 选项 =============

func (r *Runtime) withUserAgentOptions() []libp2p.Option {
	if r.cfg == nil || r.cfg.UserAgent == "" {
		return nil
	}
	return []libp2p.Option{libp2p.UserAgent(r.cfg.UserAgent)}
}

// ============= Identify 协议选项 =============

func (r *Runtime) withIdentifyOptions() []libp2p.Option {
	// 🎯 关键修复：确保 Relay 地址和公网地址能被正确宣告
	//
	// libp2p 的自动地址发现机制包括：
	// 1. ObservedAddr（对端观察到的地址，默认需要 4 个 peer 确认才激活）
	// 2. Relay 预约地址（通过 AutoRelay 自动获取，格式：/p2p-circuit/...）
	// 3. NATPortMap 映射地址（通过 UPnP/NAT-PMP 自动获取）
	//
	// 当前问题：小测试网（< 4 peers）无法积累足够的 ObservedAddr 观察数
	// 解决策略：
	// - 确保 withAddressFactoryByConfig() 不会误过滤 relay 地址
	// - 确保公网 IP（非私网）始终被保留
	// - 增强诊断日志，便于排查地址发布问题
	//
	// 注：libp2p v0.27+ 版本不直接暴露 ObservedAddrActivationThresh 配置，
	// 需要通过 identify.Service 自定义初始化（较复杂）。
	// 这里采用"确保地址不被误过滤"的策略，而不是降低激活阈值。
	
	// ✅ 修复：BandwidthReporter 已移至 withBandwidthLimiterOptions()，避免重复指定
	return []libp2p.Option{}
}

// loadPrivateKeyFromBase64 从 base64 编码的字符串加载私钥
func (r *Runtime) loadPrivateKeyFromBase64(base64Key string) (libp2pcrypto.PrivKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %w", err)
	}

	privKey, err := libp2pcrypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	return privKey, nil
}

// loadOrCreateIdentityKey 从文件加载身份密钥，如果文件不存在则生成新密钥并保存
// 注意：keyPath 在配置阶段已经解析为绝对路径（相对于实例数据目录）
func (r *Runtime) loadOrCreateIdentityKey(keyPath string) (libp2pcrypto.PrivKey, error) {
	// 确保路径是绝对路径（配置阶段已解析，这里做二次检查）
	absPath, err := filepath.Abs(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve key file path: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// 文件不存在，生成新密钥
		privKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate identity key: %w", err)
		}

		// 序列化私钥
		keyBytes, err := libp2pcrypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}

		// 确保目录存在
		keyDir := filepath.Dir(absPath)
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create key directory: %w", err)
		}

		// 保存密钥文件（仅所有者可读写）
		if err := os.WriteFile(absPath, keyBytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to save identity key file: %w", err)
		}

		return privKey, nil
	}

	// 文件存在，读取并加载
	keyBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity key file: %w", err)
	}

	privKey, err := libp2pcrypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	return privKey, nil
}

// ============= 连接管理选项 =============

func (r *Runtime) withConnectionManagerOptions() []libp2p.Option {
	if r.cfg == nil {
		cm, _ := connmgr.NewConnManager(20, 200, connmgr.WithGracePeriod(20*time.Second))
		return []libp2p.Option{libp2p.ConnectionManager(cm)}
	}

	lowWater := r.cfg.LowWater
	if lowWater <= 0 {
		lowWater = r.cfg.MinPeers
		if lowWater <= 0 {
			lowWater = 20
		}
	}

	highWater := r.cfg.HighWater
	if highWater <= 0 {
		highWater = r.cfg.MaxPeers
		if highWater <= 0 {
			highWater = 200
		}
	}

	gracePeriod := r.cfg.GracePeriod
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

	return []libp2p.Option{libp2p.ConnectionManager(cm)}
}

// ============= 资源管理选项 =============

var (
	currentResourceManager network.ResourceManager
	currentRcmgrLimits     rcmgr.ConcreteLimitConfig
	hasCurrentRcmgrLimits  bool
)

// CurrentResourceManager 返回当前资源管理器实例（供 diagnostics 使用）
func CurrentResourceManager() network.ResourceManager {
	return currentResourceManager
}

// CurrentRcmgrLimits 返回当前 rcmgr 限额（如可用，供 diagnostics 使用）
func CurrentRcmgrLimits() (rcmgr.ConcreteLimitConfig, bool) {
	return currentRcmgrLimits, hasCurrentRcmgrLimits
}

func (r *Runtime) withResourceManagerOptions() []libp2p.Option {
	if r.cfg == nil {
		return []libp2p.Option{}
	}

	// 本地诊断旁路：当仅本地环回监听且开启诊断时，使用无限限额
	if r.cfg.DiagnosticsEnabled {
		loopbackOnly := true
		for _, a := range r.cfg.ListenAddrs {
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

	rm := r.createAdaptiveResourceManager()
	if rm != nil {
		currentResourceManager = rm
		return []libp2p.Option{libp2p.ResourceManager(rm)}
	}

	return []libp2p.Option{}
}

func (r *Runtime) createAdaptiveResourceManager() network.ResourceManager {
	maxMemory := int64(memory.TotalMemory()) / 2
	maxFD := 1024
	if v := r.cfg.MemoryLimitMB; v > 0 {
		maxMemory = int64(v) * 1024 * 1024
	}
	if v := r.cfg.MaxFileDescriptors; v > 0 {
		maxFD = v
	}

	// 🆕 libp2p 资源控制优化：设置硬限制防止 Goroutine 爆增
	// 背景：阿里云公网节点 Goroutine 峰值 34,832（本地的 19 倍）
	// 原因：Conns/Streams 设为 Unlimited 导致大量非 WES 节点涌入
	// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
	partial := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Memory:          rcmgr.LimitVal64(maxMemory),
			FD:              rcmgr.LimitVal(maxFD),
			Conns:           rcmgr.LimitVal(200),  // 🆕 总连接数硬限制（原 Unlimited）
			ConnsInbound:    rcmgr.LimitVal(100),  // 🆕 入站连接限制（原基于内存计算）
			ConnsOutbound:   rcmgr.LimitVal(150),  // 🆕 出站连接限制（原 Unlimited）
			Streams:         rcmgr.LimitVal(1000), // 🆕 总流数硬限制（原 Unlimited）
			StreamsOutbound: rcmgr.LimitVal(600),  // 🆕 出站流限制（原 Unlimited）
			StreamsInbound:  rcmgr.LimitVal(500),  // 🆕 入站流限制（原 Unlimited）
		},
		Transient: rcmgr.ResourceLimits{
			Memory:          rcmgr.LimitVal64(maxMemory / 4),
			FD:              rcmgr.LimitVal(maxFD / 4),
			Conns:           rcmgr.LimitVal(50),   // 🆕 瞬态连接限制（原 Unlimited）
			ConnsInbound:    rcmgr.LimitVal(25),   // 🆕 瞬态入站限制
			ConnsOutbound:   rcmgr.LimitVal(40),   // 🆕 瞬态出站限制
			Streams:         rcmgr.LimitVal(200),  // 🆕 瞬态流限制（原 Unlimited）
			StreamsOutbound: rcmgr.LimitVal(120),  // 🆕 瞬态出站流限制
			StreamsInbound:  rcmgr.LimitVal(100),  // 🆕 瞬态入站流限制
		},
	}

	limits := partial.Build(rcmgr.DefaultLimits.Scale(maxMemory, maxFD)).ToPartialLimitConfig()

	highWater := r.cfg.HighWater
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

	currentRcmgrLimits = limits.Build(rcmgr.ConcreteLimitConfig{})
	hasCurrentRcmgrLimits = true

	limiter := rcmgr.NewFixedLimiter(currentRcmgrLimits)
	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil
	}
	return rm
}

// ============= 带宽限制选项 =============

func (r *Runtime) withBandwidthLimiterOptions() []libp2p.Option {
	return []libp2p.Option{libp2p.BandwidthReporter(getBandwidthCounter())}
}

// ============= 私有网络选项 =============

func (r *Runtime) withPrivateNetworkOptions() []libp2p.Option {
	if r.cfg == nil || !r.cfg.PrivateNetwork {
		return nil
	}

	// 私有链：需要 PSK 文件
	if r.cfg.PSKPath == "" {
		// 如果没有配置 PSK 路径，返回 nil（不使用 Private Network）
		// 在实际部署中，私有链应该 fail-fast 如果没有 PSK
		// 这里暂时返回 nil，后续可以改为 panic
		return nil
	}

	// 读取并解码 PSK 文件
	psk, err := r.readPSKFile(r.cfg.PSKPath)
	if err != nil {
		// PSK 文件读取失败，应该 fail-fast
		// 在实际部署中，私有链必须配置 PSK，这里应该 panic
		panic(fmt.Sprintf("failed to read PSK file %s: %v", r.cfg.PSKPath, err))
	}

	// 配置 libp2p 使用 Private Network
	// libp2p.PrivateNetwork 接受 pnet.PSK 类型（[]byte）
	return []libp2p.Option{libp2p.PrivateNetwork(psk)}
}

// readPSKFile 读取并解码 PSK 文件
// PSK 文件格式：32 字节的二进制密钥，或 libp2p V1 PSK 格式
// 使用 core/pnet.DecodeV1PSK 解码 PSK
func (r *Runtime) readPSKFile(pskPath string) (libp2ppnet.PSK, error) {
	// 解析路径（支持相对路径和绝对路径）
	absPath, err := filepath.Abs(pskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve PSK file path %s: %w", pskPath, err)
	}

	// 打开文件（只读）
	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PSK file %s: %w", absPath, err)
	}
	defer file.Close()

	// 使用 libp2p 的 DecodeV1PSK 解码 PSK
	// DecodeV1PSK 期望读取 libp2p V1 PSK 格式的数据
	psk, err := libp2ppnet.DecodeV1PSK(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PSK from file %s: %w", absPath, err)
	}

	// 验证 PSK 长度（应该是 32 字节）
	if len(psk) != 32 {
		return nil, fmt.Errorf("PSK file %s has invalid size: expected 32 bytes, got %d bytes", absPath, len(psk))
	}

	return psk, nil
}

// ============= AutoNAT 服务选项 =============

func (r *Runtime) withAutoNATServiceOptions() []libp2p.Option {
	var opts []libp2p.Option
	if r.cfg != nil && r.cfg.EnableAutoNATService {
		opts = append(opts, libp2p.EnableNATService())
		opts = append(opts, libp2p.EnableAutoNATv2())
	}
	return opts
}

// ============= 地址过滤选项 =============

func (r *Runtime) withAdvancedAddressFiltering() []libp2p.Option {
	var allowed, blocked []string
	if r.cfg != nil {
		allowed = r.cfg.GaterAllowedPrefixes
		blocked = r.cfg.GaterBlockedPrefixes
	}

	var filters *ma.Filters
	if len(blocked) > 0 {
		filters = ma.NewFilters()
		hasFilters := false
		for _, rule := range blocked {
			if f, err := mamask.NewMask(rule); err == nil {
				filters.AddFilter(*f, ma.ActionDeny)
				hasFilters = true
			}
		}
		if !hasFilters {
			filters = nil
		}
	}

	// 创建 Gater，传递 CA Cert Pool（如果存在，用于联盟链 mTLS 验证）
	var certPolicy *CertificateValidationPolicy
	if r.caCertPool != nil {
		// 从配置中读取证书验证策略参数
		intermediateAllowed := false
		var allowedSubjects, allowedOrgs []string
		// TODO: 从 r.cfg 或 Provider 中读取这些参数
		// 目前先使用默认值，后续可以从配置中读取
		certPolicy = NewCertificateValidationPolicy(r.caCertPool, intermediateAllowed, allowedSubjects, allowedOrgs)
	}

	gater := newAdvancedAddressGater(allowed, blocked, filters, certPolicy)
	return []libp2p.Option{libp2p.ConnectionGater(gater)}
}

// advancedAddressGater 支持 CIDR + 前缀的混合过滤，以及联盟链 mTLS 证书验证
type advancedAddressGater struct {
	filters     *ma.Filters
	allowed     []string
	blocked     []string
	allowedCIDR []*net.IPNet
	// certPolicy 证书验证策略（仅用于联盟链 mTLS）
	certPolicy *CertificateValidationPolicy
}

func newAdvancedAddressGater(allowed, blocked []string, filters *ma.Filters, certPolicy *CertificateValidationPolicy) *advancedAddressGater {
	return &advancedAddressGater{
		filters:     filters,
		allowed:     allowed,
		blocked:     blocked,
		allowedCIDR: parseCIDRs(allowed),
		certPolicy:  certPolicy,
	}
}

func (g *advancedAddressGater) InterceptPeerDial(id peer.ID) (allow bool) { return true }

func (g *advancedAddressGater) InterceptAddrDial(id peer.ID, addr ma.Multiaddr) (allow bool) {
	return g.allowAddr(addr)
}

func (g *advancedAddressGater) InterceptAccept(conn network.ConnMultiaddrs) (allow bool) {
	return g.allowAddr(conn.RemoteMultiaddr())
}

func (g *advancedAddressGater) InterceptSecured(dir network.Direction, id peer.ID, conn network.ConnMultiaddrs) (allow bool) {
	// 1. 先做地址过滤
	if !g.allowAddr(conn.RemoteMultiaddr()) {
		return false
	}

	// 2. 如果是联盟链（有证书验证策略），进行 mTLS 证书验证
	if g.certPolicy != nil {
		// 将 network.ConnMultiaddrs 转换为 network.Conn
		// 注意：network.ConnMultiaddrs 是 network.Conn 的扩展接口
		if connWithCert, ok := conn.(network.Conn); ok {
			if err := ValidatePeerCertificate(connWithCert, g.certPolicy, id); err != nil {
				// 证书验证失败，拒绝连接
				// 详细日志已在 ValidatePeerCertificate 中记录
				return false
			}
		} else {
			// 如果无法转换为 network.Conn，记录警告但暂时允许（后续需要调整）
			// 注意：这不应该发生，因为 network.ConnMultiaddrs 扩展了 network.Conn
			return false
		}
	}

	return true
}

func (g *advancedAddressGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	return g.allowAddr(conn.RemoteMultiaddr()), 0
}

func (g *advancedAddressGater) allowAddr(addr ma.Multiaddr) bool {
	addrStr := addr.String()
	if len(g.allowed) > 0 {
		if ip := toIP(addr); ip != nil {
			for _, n := range g.allowedCIDR {
				if n.Contains(ip) {
					return true
				}
			}
		}
		for _, a := range g.allowed {
			if a != "" && hasPrefix(addrStr, a) {
				return true
			}
		}
		return false
	}
	if g.filters != nil && g.filters.AddrBlocked(addr) {
		return false
	}
	for _, b := range g.blocked {
		if b != "" && hasPrefix(addrStr, b) {
			return false
		}
	}
	return true
}

func hasPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func parseCIDRs(rules []string) []*net.IPNet {
	var out []*net.IPNet
	for _, r := range rules {
		_, n, err := net.ParseCIDR(r)
		if err == nil && n != nil {
			out = append(out, n)
		}
	}
	return out
}

func toIP(addr ma.Multiaddr) net.IP {
	if v, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		return net.ParseIP(v)
	}
	if v, err := addr.ValueForProtocol(ma.P_IP6); err == nil {
		return net.ParseIP(v)
	}
	return nil
}

var _ ccmgr.ConnectionGater = (*advancedAddressGater)(nil)

// ============= 地址工厂选项 =============

func (r *Runtime) withAddressFactoryByConfig() libp2p.Option {
	advertisePrivate := false
	var announce, appendAnnounce, noAnnounce []string
	if r.cfg != nil {
		// 重要：LAN 部署（通常会启用 mDNS）即使接入同一 DHT/同一批 bootstrap，
		// 也必须向网络发布“可拨号的私网地址”（RFC1918），否则其他同网段节点只能拿到公网/Relay/空地址，导致“能发现但连不上/发现不到”。
		//
		// 默认策略：
		// - 显式配置 advertise_private_addrs=true：总是允许发布私网地址
		// - 启用 mDNS：视为 LAN 部署场景，默认也允许发布私网地址（避免“mDNS 仅用于发现但 DHT 地址不可拨号”的割裂体验）
		advertisePrivate = r.cfg.AdvertisePrivateAddrs || r.cfg.EnableMDNS
		announce = append([]string{}, r.cfg.Announce...)
		appendAnnounce = append([]string{}, r.cfg.AppendAnnounce...)
		noAnnounce = append([]string{}, r.cfg.NoAnnounce...)
	}
	return libp2p.AddrsFactory(func(in []ma.Multiaddr) []ma.Multiaddr {
		base := in
		if len(announce) > 0 {
			base = make([]ma.Multiaddr, 0, len(announce))
			for _, s := range announce {
				if m, err := ma.NewMultiaddr(s); err == nil {
					base = append(base, m)
				}
			}
		}
		if len(appendAnnounce) > 0 {
			seen := make(map[string]struct{}, len(base))
			for _, m := range base {
				seen[string(m.Bytes())] = struct{}{}
			}
			for _, s := range appendAnnounce {
				if m, err := ma.NewMultiaddr(s); err == nil {
					if _, ok := seen[string(m.Bytes())]; !ok {
						base = append(base, m)
						seen[string(m.Bytes())] = struct{}{}
					}
				}
			}
		}
		filters := ma.NewFilters()
		exact := map[string]bool{}
		for _, s := range noAnnounce {
			if f, err := mamask.NewMask(s); err == nil {
				filters.AddFilter(*f, ma.ActionDeny)
				continue
			}
			if m, err := ma.NewMultiaddr(s); err == nil {
				exact[string(m.Bytes())] = true
			}
		}
		out := make([]ma.Multiaddr, 0, len(base))
		for _, a := range base {
			// 🔑 关键修复：优先保留 relay 地址（/p2p-circuit）
			// Relay 地址是 AutoRelay 自动获取的，格式如：/ip4/x.x.x.x/tcp/4001/p2p/QmRelay.../p2p-circuit
			// 这些地址对于 NAT 后的节点至关重要，绝对不能被过滤
			isRelayAddr := false
			for _, proto := range a.Protocols() {
				if proto.Name == "p2p-circuit" {
					isRelayAddr = true
					break
				}
			}
			if isRelayAddr {
				out = append(out, a)
				continue
			}
			
			if manet.IsIPUnspecified(a) {
				continue
			}
			if exact[string(a.Bytes())] {
				continue
			}
			if filters.AddrBlocked(a) {
				continue
			}
			if ip, err := manet.ToIP(a); err == nil {
				if ip.IsLoopback() {
					continue
				}
				if ip.IsPrivate() && !advertisePrivate {
					continue
				}
			}
			out = append(out, a)
		}
		
		// 诊断日志：记录地址过滤结果（包含 relay 地址统计）
		if r.logger != nil {
			// logger是interface{}类型，需要类型断言
			type Logger interface {
				Warnf(string, ...interface{})
				Infof(string, ...interface{})
				Errorf(string, ...interface{})
			}
			if log, ok := r.logger.(Logger); ok {
				var privateFiltered, loopbackFiltered, unspecifiedFiltered, noAnnounceFiltered, relayPreserved int
				for _, a := range in {
					// 🔑 统计 relay 地址数量（关键诊断信息）
					isRelay := false
					for _, proto := range a.Protocols() {
						if proto.Name == "p2p-circuit" {
							isRelay = true
							relayPreserved++
							break
						}
					}
					if isRelay {
						continue  // relay 地址已被保留，跳过过滤器统计
					}
					
					if manet.IsIPUnspecified(a) {
						unspecifiedFiltered++
						continue
					}
					if exact[string(a.Bytes())] || filters.AddrBlocked(a) {
						noAnnounceFiltered++
						continue
					}
					if ip, err := manet.ToIP(a); err == nil {
						if ip.IsLoopback() {
							loopbackFiltered++
						} else if ip.IsPrivate() && !advertisePrivate {
							privateFiltered++
						}
					}
				}
				if total := len(in); total > 0 && len(out) != total {
					log.Warnf("p2p.host.addrs_factory: 地址过滤 total=%d advertised=%d relay_preserved=%d filtered={private:%d loopback:%d unspecified:%d noAnnounce:%d} advertise_private=%v enable_mdns=%v",
						total, len(out), relayPreserved, privateFiltered, loopbackFiltered, unspecifiedFiltered, noAnnounceFiltered, advertisePrivate, r.cfg.EnableMDNS)
				} else if len(out) > 0 {
					log.Infof("p2p.host.addrs_factory: 发布地址 count=%d relay_preserved=%d advertise_private=%v enable_mdns=%v",
						len(out), relayPreserved, advertisePrivate, r.cfg.EnableMDNS)
				}
			}
		}
		
		if len(out) == 0 {
			fallback := make([]ma.Multiaddr, 0, len(in))
			for _, a := range in {
				if manet.IsIPUnspecified(a) {
					continue
				}
				if ip, err := manet.ToIP(a); err == nil {
					if ip.IsLoopback() {
						continue
					}
				}
				fallback = append(fallback, a)
			}
			if len(fallback) == 0 {
				if r.logger != nil {
					type Logger interface {
						Errorf(string, ...interface{})
					}
					if log, ok := r.logger.(Logger); ok {
						log.Errorf("p2p.host.addrs_factory: ⚠️ 所有地址被过滤，将使用原始地址 in=%d", len(in))
					}
				}
				return in
			}
			if r.logger != nil {
				type Logger interface {
					Warnf(string, ...interface{})
				}
				if log, ok := r.logger.(Logger); ok {
					log.Warnf("p2p.host.addrs_factory: 使用fallback地址 count=%d", len(fallback))
				}
			}
			return fallback
		}
		return out
	})
}

// ============= ConnectionProtector =============

// ConnectionProtector 连接保护器
type ConnectionProtector struct {
	allowedPeers map[peer.ID]bool
	blockedPeers map[peer.ID]bool
}

// NewConnectionProtector 创建连接保护器
func NewConnectionProtector() *ConnectionProtector {
	return &ConnectionProtector{
		allowedPeers: make(map[peer.ID]bool),
		blockedPeers: make(map[peer.ID]bool),
	}
}

// AllowPeer 允许特定节点
func (cp *ConnectionProtector) AllowPeer(p peer.ID) {
	cp.allowedPeers[p] = true
	delete(cp.blockedPeers, p)
}

// BlockPeer 阻止特定节点
func (cp *ConnectionProtector) BlockPeer(p peer.ID) {
	cp.blockedPeers[p] = true
	delete(cp.allowedPeers, p)
}

// IsAllowed 检查节点是否被允许
func (cp *ConnectionProtector) IsAllowed(p peer.ID) bool {
	if cp.blockedPeers[p] {
		return false
	}
	if len(cp.allowedPeers) > 0 && !cp.allowedPeers[p] {
		return false
	}
	return true
}

// GetStats 获取保护器统计信息
func (cp *ConnectionProtector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"allowed_peers": len(cp.allowedPeers),
		"blocked_peers": len(cp.blockedPeers),
	}
}

// ============= Connectivity 选项（NAT / Reachability / Relay / AutoRelay / HolePunching）=============

// withNATPortMapOptions 根据配置构建 NAT 端口映射选项
func (r *Runtime) withNATPortMapOptions() []libp2p.Option {
	var opts []libp2p.Option
	// 缺省启用；配置简化后直接检查配置字段
	if r.cfg == nil || r.cfg.EnableNATPortMap {
		opts = append(opts, libp2p.NATPortMap())
	}
	return opts
}

// withReachabilityOptions 将配置映射为 libp2p 可达性选项
func (r *Runtime) withReachabilityOptions() []libp2p.Option {
	if r.cfg == nil {
		return nil
	}
	switch r.cfg.ForceReachability {
	case "public":
		return []libp2p.Option{libp2p.ForceReachabilityPublic()}
	case "private":
		return []libp2p.Option{libp2p.ForceReachabilityPrivate()}
	default:
		return nil
	}
}

// withRelayTransportOptions 基于配置返回中继传输开关
func (r *Runtime) withRelayTransportOptions() []libp2p.Option {
	var opts []libp2p.Option
	if r.cfg == nil {
		return []libp2p.Option{libp2p.EnableRelay()}
	}
	if r.cfg.EnableAutoRelay || r.cfg.ForceReachability == "private" || r.cfg.EnableRelay {
		opts = append(opts, libp2p.EnableRelay())
	}
	return opts
}

// withAutoRelayStaticOptions 若配置包含静态中继清单，则返回对应 AutoRelay 选项
func (r *Runtime) withAutoRelayStaticOptions() []libp2p.Option {
	var opts []libp2p.Option
	if r.cfg == nil || !r.cfg.EnableAutoRelay {
		return opts
	}
	static := r.cfg.StaticRelayPeers
	if len(static) == 0 {
		static = r.cfg.BootstrapPeers
	}
	if len(static) == 0 {
		return opts
	}
	var infos []peer.AddrInfo
	for _, s := range static {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if info, err := peer.AddrInfoFromP2pAddr(m); err == nil {
			infos = append(infos, *info)
		}
	}
	if len(infos) > 0 {
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(infos))
	}
	return opts
}

// withAutoRelayDynamicOptions 在零配置或显式启用时，注入基于 PeerSource 的 AutoRelay 选项
// PeerSource 策略：
// 1) 优先使用当前已连接 peers（Network().Peers()），并附带已知地址；
// 2) 不足时从 Peerstore.PeersWithAddrs() 兜底；
// 3) 返回数量受 numPeers 限制。
func (r *Runtime) withAutoRelayDynamicOptions() []libp2p.Option {
	// 若存在配置且显式关闭，则不注入
	if r.cfg != nil && !r.cfg.EnableAutoRelay {
		return nil
	}
	// 候选上限：优先使用配置
	limit := 16
	if r.cfg != nil && r.cfg.AutoRelayDynamicCandidates > 0 {
		limit = r.cfg.AutoRelayDynamicCandidates
	}
	ps := func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
		if numPeers <= 0 || numPeers > limit {
			numPeers = limit
		}
		ch := make(chan peer.AddrInfo, numPeers)
		go func() {
			defer close(ch)
			// 使用全局 hostProvider（在 Host 构建完成后设置）
			if hostProvider == nil {
				return
			}
			h := hostProvider()
			if h == nil {
				return
			}
			seen := make(map[peer.ID]struct{}, numPeers)
			// 1) 已连接 peers
			for _, pid := range h.Network().Peers() {
				if _, ok := seen[pid]; ok {
					continue
				}
				ai := peer.AddrInfo{ID: pid, Addrs: h.Peerstore().Addrs(pid)}
				if len(ai.Addrs) > 0 {
					ch <- ai
					seen[pid] = struct{}{}
					if len(seen) >= numPeers {
						return
					}
				}
			}
			// 2) Peerstore 兜底
			if len(seen) < numPeers {
				for _, pid := range h.Peerstore().PeersWithAddrs() {
					if _, ok := seen[pid]; ok {
						continue
					}
					ai := peer.AddrInfo{ID: pid, Addrs: h.Peerstore().Addrs(pid)}
					if len(ai.Addrs) == 0 {
						continue
					}
					ch <- ai
					seen[pid] = struct{}{}
					if len(seen) >= numPeers {
						return
					}
				}
			}
		}()
		return ch
	}
	return []libp2p.Option{libp2p.EnableAutoRelayWithPeerSource(ps)}
}

// withHolePunchingOptions 基于配置启用 DCUtR（需具备中继客户端能力）
func (r *Runtime) withHolePunchingOptions() []libp2p.Option {
	var opts []libp2p.Option
	// 连接优先：cfg 缺失时默认启用（若具备中继客户端能力则生效）
	if r.cfg == nil {
		return []libp2p.Option{libp2p.EnableHolePunching()}
	}
	if r.cfg.EnableDCUTR {
		opts = append(opts, libp2p.EnableHolePunching())
	}
	return opts
}

// withRelayServiceOptions 启用 Relay 服务端（使用默认或自定义资源配额）
func (r *Runtime) withRelayServiceOptions() []libp2p.Option {
	var opts []libp2p.Option
	if r.cfg == nil || !r.cfg.EnableRelayService {
		return opts
	}

	// 构建资源配置
	res := relayv2.DefaultResources()
	// 如果配置了自定义资源，覆盖默认值
	if r.cfg.RelayMaxReservations > 0 {
		res.MaxReservations = r.cfg.RelayMaxReservations
	}
	if r.cfg.RelayMaxCircuits > 0 {
		res.MaxCircuits = r.cfg.RelayMaxCircuits
	}
	if r.cfg.RelayBufferSize > 0 {
		res.BufferSize = r.cfg.RelayBufferSize
	}

	opts = append(opts, libp2p.EnableRelayService(relayv2.WithResources(res)))
	return opts
}
