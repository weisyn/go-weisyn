package facade

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libhost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	networkconfig "github.com/weisyn/v1/internal/config/network"
	networkInterfaces "github.com/weisyn/v1/internal/core/network/interfaces"
	pubimpl "github.com/weisyn/v1/internal/core/network/pubsub"
	regimpl "github.com/weisyn/v1/internal/core/network/registry"
	netsec "github.com/weisyn/v1/internal/core/network/security"
	stcodec "github.com/weisyn/v1/internal/core/network/stream"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	"github.com/weisyn/v1/pkg/constants/protocols"
	cryptoi "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
)

// Facade Network 门面统一实现
// 用途：
// - 实现 networkInterfaces.InternalNetwork 接口，统一提供协议注册、流式发送与订阅发布能力
// - 聚合内部组件完成消息编解码与分发，不暴露生命周期与指标
// 说明：
// - 不包含生命周期管理（Start/Stop）；由上层 DI 管理
// - 不暴露内部指标或状态；仅聚焦消息编解码与分发
// - 业务协议由各领域模块自行注册，Network 不维护业务协议清单
// - 遵循代码组织规范：实现内部接口 InternalNetwork，而非直接实现公共接口
type Facade struct {
	host   libhost.Host              // P2P宿主，用于连通性保障与流操作
	reg    *regimpl.ProtocolRegistry // 协议注册表（供诊断与处理器查找）
	logger logiface.Logger           // 结构化日志器

	// PubSub 组件
	tm    *pubimpl.TopicManager
	enc   *pubimpl.Encoder
	dec   *pubimpl.Decoder
	val   *pubimpl.Validator
	pub   *pubimpl.Publisher
	subs  map[string]iface.SubscribeHandler // 本地订阅处理器
	subCF map[string]iface.SubscribeConfig  // 订阅配置快照
	regCF map[string]iface.RegisterConfig   // 注册配置快照
	// GossipSub 组件
	ps           *pubsub.PubSub
	topicHandles map[string]*pubsub.Topic
	subHandles   map[string]*pubsub.Subscription
	subCancels   map[string]context.CancelFunc

	// 注册状态管理（防重复注册）
	registeredProtocols map[string]bool // 已注册的流式协议
	registeredTopics    map[string]bool // 已注册的订阅主题

	// 互斥保护
	regMu sync.RWMutex // 保护注册状态管理
	subMu sync.RWMutex // 保护 subs/subCF/regCF
	psMu  sync.Mutex   // 保护 topicHandles/subHandles/subCancels

	// 入站并发/背压
	streamSvc *stcodec.Service

	// 配置（可选）
	cfg *networkconfig.Config

	// 网络命名空间（用于自动为协议 ID 和 Topic 添加 namespace）
	networkNamespace string

	// 🆕 协议协商器（MEDIUM-002 修复）
	protocolNegotiator *ProtocolNegotiator

	// crypto services
	hashManager cryptoi.HashManager
	sigManager  cryptoi.SignatureManager

	// 安全保护组件（真实接入）
	rateLimiter    *netsec.RateLimiter
	msgRateLimiter *netsec.MessageRateLimiter

	// 最小可观测性
	pubCount   uint64
	dropCount  uint64
	callCount  uint64
	retryCount uint64

	// validatorCleanupStop 用于停止 Validator 清理协程
	validatorCleanupStop chan struct{}

	// ====================
	// forceConnect：可控拨号（业务节点优先）
	// ====================
	forceConnectMu          sync.Mutex
	forceConnectReqCh       chan string
	forceConnectStopCtx     context.Context
	forceConnectStopCancel  context.CancelFunc
	forceConnectLastAt      time.Time
	forceConnectCfg         ForceConnectConfig
	forceConnectRand        *rand.Rand
}

// ForceConnectConfig forceConnect（GossipSub Mesh 拉活）配置
//
// 目标：避免对 peerstore 全量拨号造成 goroutine 风暴；业务节点优先，其余节点抽样辅助公网发现。
type ForceConnectConfig struct {
	Enabled           bool
	Cooldown          time.Duration
	Concurrency       int
	BudgetPerRound    int
	Tier2SampleBudget int
	Timeout           time.Duration

	BusinessPeers  []peer.ID
	BootstrapPeers []peer.ID
}

// SetForceConnectConfig 设置 forceConnect 配置（由上层模块注入）
func (f *Facade) SetForceConnectConfig(cfg ForceConnectConfig) {
	if f == nil {
		return
	}
	f.forceConnectMu.Lock()
	defer f.forceConnectMu.Unlock()

	f.forceConnectCfg = cfg
	// 默认兜底
	if f.forceConnectCfg.Cooldown <= 0 {
		f.forceConnectCfg.Cooldown = 2 * time.Minute
	}
	if f.forceConnectCfg.Concurrency <= 0 {
		f.forceConnectCfg.Concurrency = 15
	}
	if f.forceConnectCfg.BudgetPerRound <= 0 {
		f.forceConnectCfg.BudgetPerRound = 50
	}
	if f.forceConnectCfg.Tier2SampleBudget < 0 {
		f.forceConnectCfg.Tier2SampleBudget = 0
	}
	if f.forceConnectCfg.Timeout <= 0 {
		f.forceConnectCfg.Timeout = 10 * time.Second
	}

	if f.logger != nil {
		f.logger.Infof("forceConnect config loaded enabled=%t cooldown=%s concurrency=%d budget=%d tier2_sample=%d business_peers=%d bootstrap_peers=%d",
			f.forceConnectCfg.Enabled,
			f.forceConnectCfg.Cooldown,
			f.forceConnectCfg.Concurrency,
			f.forceConnectCfg.BudgetPerRound,
			f.forceConnectCfg.Tier2SampleBudget,
			len(f.forceConnectCfg.BusinessPeers),
			len(f.forceConnectCfg.BootstrapPeers),
		)
	}
}

func (f *Facade) ensureForceConnectLoop() {
	if f == nil {
		return
	}
	f.forceConnectMu.Lock()
	defer f.forceConnectMu.Unlock()

	if f.forceConnectReqCh != nil {
		return
	}
	f.forceConnectReqCh = make(chan string, 4) // 合并触发，避免并发触发堆积
	f.forceConnectStopCtx, f.forceConnectStopCancel = context.WithCancel(context.Background())
	f.forceConnectRand = rand.New(rand.NewSource(time.Now().UnixNano()))

	go f.forceConnectLoop()
}

// requestForceConnect 请求执行一轮 forceConnect（合并触发 + cooldown 节流）
func (f *Facade) requestForceConnect(reason string) {
	if f == nil {
		return
	}
	f.ensureForceConnectLoop()

	select {
	case f.forceConnectReqCh <- reason:
	default:
		// channel 满了，丢弃（合并触发）
	}
}

func (f *Facade) forceConnectLoop() {
	for {
		select {
		case <-f.forceConnectStopCtx.Done():
			return
		case reason := <-f.forceConnectReqCh:
			f.runForceConnectRound(reason)
		}
	}
}

// NewFacade 创建 Network 门面实例
func NewFacade(host libhost.Host, logger logiface.Logger, cfg *networkconfig.Config, hashMgr cryptoi.HashManager, sigMgr cryptoi.SignatureManager) *Facade {
	return NewFacadeWithNamespace(host, logger, cfg, hashMgr, sigMgr, "")
}

// NewFacadeWithNamespace 创建 Network 门面实例（带 namespace）
func NewFacadeWithNamespace(host libhost.Host, logger logiface.Logger, cfg *networkconfig.Config, hashMgr cryptoi.HashManager, sigMgr cryptoi.SignatureManager, namespace string) *Facade {
	if logger == nil {
		logger = &noopLogger{} // 占位日志器
	}

	// 初始化安全限制器参数（从配置读取，带默认值回退）
	maxConns := 1000             // 默认最大连接数
	maxPerIP := 50               // 默认每IP最大连接数
	maxMsgs := 100               // 默认每时间窗口最大消息数
	msgWindow := 1 * time.Minute // 默认消息窗口

	// 从配置读取安全参数（如果配置可用）
	if cfg != nil {
		if cfgMaxConns := cfg.GetMaxConnections(); cfgMaxConns > 0 {
			maxConns = cfgMaxConns
		}
		if cfgMaxPerIP := cfg.GetMaxConnectionsPerIP(); cfgMaxPerIP > 0 {
			maxPerIP = cfgMaxPerIP
		}
		if cfgMaxMsgs := cfg.GetMaxMessagesPerWindow(); cfgMaxMsgs > 0 {
			maxMsgs = cfgMaxMsgs
		}
		if cfgMsgWindow := cfg.GetMessageRateLimitWindow(); cfgMsgWindow > 0 {
			msgWindow = cfgMsgWindow
		}
	}

	f := &Facade{
		host:                 host,
		reg:                  regimpl.NewProtocolRegistry(),
		logger:               logger,
		tm:                   pubimpl.NewTopicManager(),
		enc:                  pubimpl.NewEncoder(),
		dec:                  pubimpl.NewDecoder(),
		val:                  pubimpl.NewValidator(),
		pub:                  pubimpl.NewPublisher(),
		subs:                 make(map[string]iface.SubscribeHandler),
		subCF:                make(map[string]iface.SubscribeConfig),
		regCF:                make(map[string]iface.RegisterConfig),
		registeredProtocols:  make(map[string]bool),
		registeredTopics:     make(map[string]bool),
		streamSvc:            stcodec.New(host),
		cfg:                  cfg,
		networkNamespace:     namespace,
		hashManager:          hashMgr,
		sigManager:           sigMgr,
		topicHandles:         make(map[string]*pubsub.Topic),
		subHandles:           make(map[string]*pubsub.Subscription),
		subCancels:           make(map[string]context.CancelFunc),
		rateLimiter:          netsec.NewRateLimiter(maxConns, maxPerIP),
		msgRateLimiter:       netsec.NewMessageRateLimiter(maxMsgs, msgWindow),
		validatorCleanupStop: make(chan struct{}),
		// 🆕 MEDIUM-002 修复：初始化协议协商器
		protocolNegotiator:   NewProtocolNegotiator(namespace, 30*time.Minute, 1000),
	}
	// 将统一哈希服务注入 validator 用于去重
	if f.hashManager != nil {
		f.val.WithHasher(func(b []byte) (string, error) {
			h := f.hashManager.SHA256(b)
			return fmt.Sprintf("%x", h), nil
		})
	}
	// 注入签名验签钩子：当前仅检查“签名存在”以避免缺少公钥导致误判
	f.val.WithVerifier(func(payload, sig []byte) (bool, error) {
		return len(sig) > 0, nil
	})
	// 启动 Validator 去重过期清理后台任务（轻量，可由 Facade.Stop() 停止）
	go func(stopCh <-chan struct{}) {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if f.val != nil {
					f.val.CleanupExpiredEntries()
				}
			case <-stopCh:
				return
			}
		}
	}(f.validatorCleanupStop)
	// 🔧 不在这里初始化GossipSub，等待Host启动事件触发
	return f
}

// qualifyProtocolID 为协议 ID 添加 namespace（如果配置了 namespace）
func (f *Facade) qualifyProtocolID(protoID string) string {
	if f.networkNamespace == "" {
		return protoID
	}
	return protocols.QualifyProtocol(protoID, f.networkNamespace)
}

// qualifyTopic 为 Topic 添加 namespace（如果配置了 namespace）
func (f *Facade) qualifyTopic(topic string) string {
	if f.networkNamespace == "" {
		return topic
	}
	return protocols.QualifyTopic(topic, f.networkNamespace)
}

// 编译期检查：确保 Facade 实现内部接口 InternalNetwork
// 遵循代码组织规范：实现层必须实现内部接口，不能直接实现公共接口
var _ networkInterfaces.InternalNetwork = (*Facade)(nil)

// initGossipSub 初始化或重新初始化 GossipSub
func (f *Facade) initGossipSub() {
	if f.host == nil {
		f.logger.Errorf("❌ initGossipSub: host is nil")
		return
	}

	if f.host == nil {
		f.logger.Errorf("❌ initGossipSub: libp2p host is nil")
		return
	}

	f.logger.Infof("🔧 Creating GossipSub with optimized config for small networks")

	// 🔧 修复：为小网络优化的GossipSub配置
	opts := []pubsub.Option{
		pubsub.WithPeerExchange(true),                          // 启用peer交换
		pubsub.WithFloodPublish(true),                          // 启用洪泛发布，支持小网络
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign), // 禁用消息签名加速传输
	}

	if ps, err := pubsub.NewGossipSub(context.Background(), f.host, opts...); err == nil {
		f.ps = ps
		f.logger.Infof("🎉 gossipsub initialized successfully with optimized mesh config")

		// ✅ 可控拉活：合并触发 + cooldown 节流 + 业务节点优先
		f.requestForceConnect("gossipsub_init")
	} else {
		f.logger.Errorf("❌ gossipsub init failed: %v", err)
	}
}

// ensureGossipSub 确保 GossipSub 已初始化（延迟初始化）
func (f *Facade) ensureGossipSub() {
	if f.ps != nil {
		return // 已经初始化
	}

	f.logger.Infof("gossipsub not initialized, checking host status")

	// 🔧 修复：静默等待host启动完成，不报错
	if f.host == nil {
		return // 静默等待
	}

	// host已就绪，直接初始化GossipSub
	f.logger.Infof("✅ host is ready, initializing gossipsub")
	f.initGossipSub()

	if f.ps == nil {
		f.logger.Errorf("❌ gossipsub initialization failed even with ready host")
	} else {
		f.logger.Infof("✅ gossipsub successfully initialized")
	}
}

// ForceInitializeGossipSub 强制初始化GossipSub（在Host启动后调用）
func (f *Facade) ForceInitializeGossipSub() {
	if f.ps != nil {
		f.logger.Infof("gossipsub already initialized")
		return
	}

	f.logger.Infof("🔧 强制初始化GossipSub")
	f.ensureGossipSub()

	// 🔧 重要：GossipSub初始化后，重新处理所有订阅以确保真正加入mesh
	if f.ps != nil {
		f.logger.Infof("🔧 GossipSub初始化成功，等待就绪后处理订阅")
		f.waitForGossipSubReady()
		f.reprocessAllSubscriptions()
	}
}

// waitForGossipSubReady 等待GossipSub完全就绪
func (f *Facade) waitForGossipSubReady() {
	f.logger.Infof("🔧 检查GossipSub就绪状态")

	maxRetries := 50 // 最多等待5秒
	for i := 0; i < maxRetries; i++ {
		if f.isGossipSubReady() {
			f.logger.Infof("✅ GossipSub已就绪，可以加入主题 (检查%d次)", i+1)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.logger.Warnf("⚠️ GossipSub就绪检查超时，继续执行")
}

// isGossipSubReady 检查GossipSub是否已经完全就绪
func (f *Facade) isGossipSubReady() bool {
	if f.ps == nil {
		return false
	}

	// 尝试创建一个测试主题来验证GossipSub状态
	testTopic := "test.readiness.check.v1"
	if handle, err := f.ps.Join(testTopic); err == nil {
		// 立即关闭测试主题，避免污染
		if err := handle.Close(); err != nil {
			f.logger.Warnf("关闭测试主题失败: %v", err)
		}
		f.logger.Debugf("🔧 GossipSub就绪检查通过")
		return true
	} else {
		f.logger.Debugf("🔧 GossipSub尚未就绪: %v", err)
		return false
	}
}

// reprocessAllSubscriptions 重新处理所有订阅，确保它们真正加入mesh网络
func (f *Facade) reprocessAllSubscriptions() {
	f.subMu.Lock()
	topics := make([]string, 0, len(f.subs))
	for topic := range f.subs {
		topics = append(topics, topic)
	}
	f.subMu.Unlock()

	f.logger.Infof("🔧 重新处理 %d 个订阅主题", len(topics))

	for _, topic := range topics {
		f.psMu.Lock()

		// 检查是否已经加入主题
		if _, ok := f.topicHandles[topic]; !ok {
			f.logger.Infof("🔧 为主题 %s 加入mesh网络", topic)
			if t, e := f.ps.Join(topic); e == nil {
				f.topicHandles[topic] = t
				f.logger.Infof("✅ 成功加入主题mesh: %s", topic)
			} else {
				f.logger.Errorf("❌ 加入主题mesh失败: %s, error: %v", topic, e)
			}
		}

		// 检查是否已经有订阅
		if _, exists := f.subHandles[topic]; !exists {
			f.logger.Infof("🔧 为主题 %s 创建订阅", topic)
			if sub, e := f.ps.Subscribe(topic); e == nil {
				f.subHandles[topic] = sub
				ctx, cancel := context.WithCancel(context.Background())
				f.subCancels[topic] = cancel

				// 启动消息处理协程
				go func() {
					dec := f.dec
					for {
						msg, err := sub.Next(ctx)
						if err != nil {
							f.logger.Debugf("订阅消息接收结束: topic=%s, error=%v", topic, err)
							return
						}
						if msg == nil {
							continue
						}
						data := msg.GetData()
						f.logger.Debugf("📨 收到gossipsub消息: topic=%s, from=%s, size=%d", topic, msg.ReceivedFrom.String(), len(data))

						// 🛡️ 消息速率限制检查
						peerID := msg.ReceivedFrom.String()
						if f.msgRateLimiter != nil {
							if err := f.msgRateLimiter.CheckMessage(peerID); err != nil {
								f.logger.Warnf("消息速率限制拒绝: topic=%s, peer=%s, error=%v", topic, peerID, err)
								continue
							}
						}

						if f.val != nil {
							if ok, reason := f.val.Validate(topic, data); !ok {
								f.logger.With("topic", topic, "reason", reason).Debug("🚫 gossipsub message dropped")
								continue
							}
						}

						// 解码消息
						if dec != nil {
							if payload, derr := dec.Decode(topic, data); derr == nil {
								f.logger.Debugf("✅ 消息解码成功: topic=%s, original_size=%d, decoded_size=%d", topic, len(data), len(payload))
								data = payload
							} else {
								f.logger.Warnf("⚠️ 消息解码失败: topic=%s, error=%v", topic, derr)
							}
						}

						// 调用处理器
						f.subMu.RLock()
						handler := f.subs[topic]
						f.subMu.RUnlock()

						if handler != nil {
							if handlerErr := handler(context.Background(), msg.ReceivedFrom, topic, data); handlerErr != nil {
								f.logger.Warnf("订阅处理器执行失败: topic=%s, error=%v", topic, handlerErr)
							}
						} else {
							f.logger.Warnf("未找到订阅处理器: topic=%s", topic)
						}
					}
				}()
				f.logger.Infof("✅ 成功创建主题订阅: %s", topic)
			} else {
				f.logger.Errorf("❌ 创建主题订阅失败: %s, error: %v", topic, e)
			}
		}

		f.psMu.Unlock()
	}
}

// runForceConnectRound 执行一轮可控拨号，用于“拉活” GossipSub mesh（业务节点优先 + 抽样辅助公网发现）。
//
// 设计目标：
//   - 利用 host.Peerstore() / host.Network().Peers() 中已有的 peer 信息，显式发起 Dial；
//   - 避免对自身或已连接 peer 反复拨号；
//   - 在 bootstrap/discovery 机制较弱的小网络中，帮助节点尽快形成连通 mesh。
func (f *Facade) runForceConnectRound(reason string) {
	if f == nil || f.host == nil {
		return
	}
	f.forceConnectMu.Lock()
	cfg := f.forceConnectCfg
	lastAt := f.forceConnectLastAt
	now := time.Now()
	// cooldown 节流
	if cfg.Enabled && !lastAt.IsZero() && now.Sub(lastAt) < cfg.Cooldown {
		f.forceConnectMu.Unlock()
		if f.logger != nil {
			f.logger.Debugf("forceConnect skipped by cooldown reason=%s since_last=%s cooldown=%s",
				reason, now.Sub(lastAt), cfg.Cooldown)
		}
		return
	}
	// 标记本轮开始
	f.forceConnectLastAt = now
	f.forceConnectMu.Unlock()

	if !cfg.Enabled {
		if f.logger != nil {
			f.logger.Debugf("forceConnect disabled reason=%s", reason)
		}
		return
	}

	host := f.host
	selfID := host.ID()

	// 收集 topic peers（Tier1.5）
	topicPeers := make([]peer.ID, 0, 64)
	f.psMu.Lock()
	for _, th := range f.topicHandles {
		if th == nil {
			continue
		}
		topicPeers = append(topicPeers, th.ListPeers()...)
	}
	f.psMu.Unlock()

	// peerstore peers（Tier2 候选池）
	peerstorePeers := host.Peerstore().Peers()

	targets, tierByPeer, skippedConnected, skippedNoAddr := f.buildForceConnectTargets(selfID, cfg, topicPeers, peerstorePeers)
	if len(targets) == 0 {
		if f.logger != nil {
			f.logger.Debugf("forceConnect no_targets reason=%s skipped_connected=%d skipped_no_addr=%d",
				reason, skippedConnected, skippedNoAddr)
		}
		return
	}

	start := time.Now()
	type result struct {
		ok bool
	}

	workers := cfg.Concurrency
	if workers <= 0 {
		workers = 15
	}
	if workers > len(targets) {
		workers = len(targets)
	}
	jobs := make(chan peer.AddrInfo, len(targets))
	results := make(chan result, len(targets))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range jobs {
				// 二次检查：避免重复拨号已连接 peer
				if info.ID == "" || info.ID == selfID {
					results <- result{ok: false}
					continue
				}
				if host.Network().Connectedness(info.ID) == libnetwork.Connected {
					results <- result{ok: true}
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
				err := host.Connect(ctx, info)
				cancel()
				if err != nil {
					if f.logger != nil {
						tier := tierByPeer[info.ID]
						f.logger.Debugf("forceConnect dial_failed peer=%s tier=%d reason=%s err=%v", info.ID.String(), tier, reason, err)
					}
					results <- result{ok: false}
					continue
				}
				// 成功日志：仅业务关键节点（Tier0）用 Info，其他 tier 用 Debug，避免刷屏
				if f.logger != nil {
					tier := tierByPeer[info.ID]
					if tier == 0 {
						f.logger.Infof("forceConnect dial_success peer=%s tier=0 reason=%s", info.ID.String(), reason)
					} else {
						f.logger.Debugf("forceConnect dial_success peer=%s tier=%d reason=%s", info.ID.String(), tier, reason)
					}
				}
				results <- result{ok: true}
			}
		}()
	}

	for _, t := range targets {
		jobs <- t
	}
	close(jobs)
	wg.Wait()
	close(results)

	attempted := len(targets)
	succeeded := 0
	failed := 0
	for r := range results {
		if r.ok {
			succeeded++
		} else {
			failed++
		}
	}

	if f.logger != nil {
		f.logger.Infof("forceConnect round_done reason=%s attempted=%d success=%d failed=%d skipped_connected=%d skipped_no_addr=%d duration=%s",
			reason, attempted, succeeded, failed, skippedConnected, skippedNoAddr, time.Since(start))
	}
}

func (f *Facade) buildForceConnectTargets(
	selfID peer.ID,
	cfg ForceConnectConfig,
	topicPeers []peer.ID,
	peerstorePeers []peer.ID,
) (targets []peer.AddrInfo, tierByPeer map[peer.ID]int, skippedConnected int, skippedNoAddr int) {
	host := f.host

	seen := make(map[peer.ID]struct{}, 256)
	tierByPeer = make(map[peer.ID]int, 256)
	add := func(id peer.ID, tier int) {
		if id == "" || id == selfID {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}

		if host.Network().Connectedness(id) == libnetwork.Connected {
			skippedConnected++
			return
		}
		addrs := host.Peerstore().Addrs(id)
		if len(addrs) == 0 {
			skippedNoAddr++
			return
		}
		tierByPeer[id] = tier
		targets = append(targets, peer.AddrInfo{ID: id, Addrs: addrs})
	}

	// Tier0: 业务关键节点（最高优先级）
	for _, id := range cfg.BusinessPeers {
		add(id, 0)
		if cfg.BudgetPerRound > 0 && len(targets) >= cfg.BudgetPerRound {
			return targets, tierByPeer, skippedConnected, skippedNoAddr
		}
	}

	// Tier1: bootstrap peers
	for _, id := range cfg.BootstrapPeers {
		add(id, 1)
		if cfg.BudgetPerRound > 0 && len(targets) >= cfg.BudgetPerRound {
			return targets, tierByPeer, skippedConnected, skippedNoAddr
		}
	}

	// Tier1.5: topic peers（关键 topic 的已连接集合）
	for _, id := range topicPeers {
		add(id, 2)
		if cfg.BudgetPerRound > 0 && len(targets) >= cfg.BudgetPerRound {
			return targets, tierByPeer, skippedConnected, skippedNoAddr
		}
	}

	// Tier2: peerstore peers 抽样（用于公网发现/mesh拉活）
	tier2Budget := cfg.Tier2SampleBudget
	if tier2Budget <= 0 {
		return targets, tierByPeer, skippedConnected, skippedNoAddr
	}
	if cfg.BudgetPerRound > 0 {
		remain := cfg.BudgetPerRound - len(targets)
		if remain <= 0 {
			return targets, tierByPeer, skippedConnected, skippedNoAddr
		}
		if tier2Budget > remain {
			tier2Budget = remain
		}
	}
	// 采样：打乱 peerstorePeers，取前 tier2Budget 个
	cands := make([]peer.ID, 0, len(peerstorePeers))
	for _, id := range peerstorePeers {
		if id == "" || id == selfID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		cands = append(cands, id)
	}
	if len(cands) == 0 {
		return targets, tierByPeer, skippedConnected, skippedNoAddr
	}
	// shuffle
	f.forceConnectMu.Lock()
	r := f.forceConnectRand
	f.forceConnectMu.Unlock()
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	r.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })

	for i := 0; i < len(cands) && i < tier2Budget; i++ {
		add(cands[i], 3)
	}

	return targets, tierByPeer, skippedConnected, skippedNoAddr
}

// InitializeGossipSub 公开方法，允许外部在Host启动完成后主动初始化GossipSub
func (f *Facade) InitializeGossipSub() {
	f.logger.Infof("🔧 InitializeGossipSub called")
	if f.ps == nil {
		f.logger.Infof("🔧 external trigger: initializing gossipsub")
		f.initGossipSub()
		if f.ps != nil {
			f.logger.Infof("✅ InitializeGossipSub: gossipsub successfully initialized")
		} else {
			f.logger.Errorf("❌ InitializeGossipSub: gossipsub initialization failed")
		}
	} else {
		f.logger.Infof("✅ InitializeGossipSub: gossipsub already initialized, skipping")
	}
}

// ensureGossipSubWithRetry 带重试机制的GossipSub初始化
func (f *Facade) ensureGossipSubWithRetry(topic string) {
	if f.ps != nil {
		return // 已经初始化
	}

	f.logger.Infof("gossipsub not initialized for topic %s, attempting with retry", topic)

	// 启动后台协程进行重试
	go func() {
		maxRetries := 10
		retryInterval := time.Second * 2

		for i := 0; i < maxRetries; i++ {
			if f.ps != nil {
				f.logger.Infof("gossipsub already initialized during retry for topic %s", topic)
				// 检查主题是否已加入，如果没有则加入
				f.psMu.Lock()
				if _, ok := f.topicHandles[topic]; !ok {
					if t, e := f.ps.Join(topic); e == nil {
						f.topicHandles[topic] = t
						f.logger.Infof("successfully joined topic %s after gossipsub retry init", topic)
					} else {
						f.logger.Warnf("failed to join topic %s: %v", topic, e)
					}
				} else {
					f.logger.Infof("topic %s already joined", topic)
				}

				// 🔧 修复：创建实际的消息订阅（这是关键的缺失部分！）
				if _, exists := f.subHandles[topic]; !exists {
					if sub, e := f.ps.Subscribe(topic); e == nil {
						f.subHandles[topic] = sub
						ctx, cancel := context.WithCancel(context.Background())
						f.subCancels[topic] = cancel
						f.logger.Infof("✅ 创建消息订阅成功: %s", topic)

						// 🔧 修复：订阅成功后延迟强制连接peers，确保其他节点也启动完成
						go func() {
							time.Sleep(10 * time.Second) // 等待其他节点启动完成
							f.requestForceConnect("gossipsub_retry_subscribe")
						}()

						// 启动消息处理循环
						go func() {
							f.subMu.RLock()
							h := f.subs[topic]
							f.subMu.RUnlock()

							dec := f.dec
							for {
								msg, err := sub.Next(ctx)
								if err != nil {
									f.logger.Debugf("订阅消息接收结束: topic=%s, error=%v", topic, err)
									return
								}
								if msg == nil {
									continue
								}
								data := msg.GetData()
								f.logger.Debugf("📨 收到gossipsub消息: topic=%s, from=%s, size=%d", topic, msg.ReceivedFrom.String(), len(data))

								// 🛡️ 消息速率限制检查
								peerID := msg.ReceivedFrom.String()
								if f.msgRateLimiter != nil {
									if err := f.msgRateLimiter.CheckMessage(peerID); err != nil {
										f.logger.Warnf("消息速率限制拒绝: topic=%s, peer=%s, error=%v", topic, peerID, err)
										continue
									}
								}

								if f.val != nil {
									if ok, reason := f.val.Validate(topic, data); !ok {
										f.logger.With("topic", topic, "reason", reason).Debug("🚫 gossipsub message dropped")
										continue
									}
								}
								if dec != nil {
									if payload, derr := dec.Decode(topic, data); derr == nil {
										data = payload
									}
								}

								if h != nil {
									if handlerErr := h(context.Background(), msg.ReceivedFrom, topic, data); handlerErr != nil {
										f.logger.Warnf("订阅处理器执行失败: topic=%s, error=%v", topic, handlerErr)
									}
								} else {
									f.logger.Warnf("未找到订阅处理器: topic=%s", topic)
								}
							}
						}()
					} else {
						f.logger.Warnf("创建消息订阅失败: topic=%s, error=%v", topic, e)
					}
				}
				f.psMu.Unlock()
				return
			}

			f.logger.Infof("retry %d/%d: attempting gossipsub initialization for topic %s", i+1, maxRetries, topic)
			f.initGossipSub()

			if f.ps != nil {
				f.logger.Infof("gossipsub successfully initialized on retry %d for topic %s", i+1, topic)

				// 重新尝试订阅这个主题
				f.psMu.Lock()
				if _, ok := f.topicHandles[topic]; !ok {
					if t, e := f.ps.Join(topic); e == nil {
						f.topicHandles[topic] = t
						f.logger.Infof("successfully joined topic %s after gossipsub retry init", topic)
					}
				}
				f.psMu.Unlock()
				return
			}

			if i < maxRetries-1 {
				time.Sleep(retryInterval)
			}
		}

		f.logger.Warnf("failed to initialize gossipsub after %d retries for topic %s", maxRetries, topic)
	}()
}

// ==================== 协议注册（流式） ====================

// RegisterStreamHandler 注册流式协议处理器
func (f *Facade) RegisterStreamHandler(protoID string, handler iface.MessageHandler, opts ...iface.RegisterOption) error {
	// 🛡️ 自动为协议 ID 添加 namespace（如果配置了 namespace）
	qualifiedProtoID := f.qualifyProtocolID(protoID)
	ids := []string{qualifiedProtoID}
	if qualifiedProtoID != protoID {
		// 兼容旧节点：同时注册未加 namespace 的原始协议 ID
		ids = append(ids, protoID)
	}

	// 检查重复注册
	f.regMu.Lock()
	for _, id := range ids {
		if f.registeredProtocols[id] {
			f.regMu.Unlock()
			f.logger.With("protocol_id", id).Warn("协议已注册，拒绝重复注册")
			return fmt.Errorf("协议 %s 已注册，不允许重复注册", id)
		}
	}
	// 标记为已注册（qualified + original）
	for _, id := range ids {
		f.registeredProtocols[id] = true
	}
	f.regMu.Unlock()

	f.logger.With("protocol_id", qualifiedProtoID, "original", protoID).Info("registering stream handler")

	// 解析注册选项
	var cfg iface.RegisterConfig
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	f.subMu.Lock()
	for _, id := range ids {
		f.regCF[id] = cfg
	}
	f.subMu.Unlock()

	// 并发/背压：按照每协议信号量限制
	if cfg.MaxConcurrency > 0 {
		f.streamSvc.SetConcurrencyLimit(cfg.MaxConcurrency)
	}
	sem := f.streamSvc.GetSemaphore()
	wrap := regimpl.NewHandlerWrapper()
	// 可选默认超时：按需启用（此处保持0，待配置接入）

	// 协议注册为低频操作，保留一条 Info 日志；但不再打出“TRACE”字样，避免与级别语义混淆。
	f.logger.Infof("注册协议处理器: %s", string(qualifiedProtoID))

	registerOne := func(bindProtoID string) {
		if f.reg != nil {
			f.logger.Debugf("注册到内部注册表: %s", string(bindProtoID))
			if err := f.reg.Register(bindProtoID, wrap.Wrap(handler)); err != nil {
				f.logger.Warnf("注册协议到内部注册表失败: %v", err)
			}
		}
		// 入站桥接：读取一帧请求，调用 handler，回写一帧响应
		f.host.SetStreamHandler(libprotocol.ID(bindProtoID), func(s libnetwork.Stream) {
			remote := s.Conn().RemotePeer()
			ctx := context.Background()
			f.logger.With("protocol_id", bindProtoID, "remote_peer", remote.String()).Debug("handling inbound stream")

			// 🛡️ 连接速率限制检查
			peerIDStr := remote.String()
			if f.rateLimiter != nil {
				// 使用 peerID 作为"IP"标识（简化实现，实际可从 multiaddr 提取真实IP）
				if err := f.rateLimiter.CheckConnection(peerIDStr, peerIDStr); err != nil {
					f.logger.Warnf("连接速率限制拒绝: protocol=%s, peer=%s, error=%v", bindProtoID, peerIDStr, err)
					if err := s.Reset(); err != nil {
						f.logger.Warnf("重置流失败: %v", err)
					}
					return
				}
				defer f.rateLimiter.RemoveConnection(peerIDStr, peerIDStr)
			}

			// 背压：尝试获取信号量
			if sem != nil {
				if err := sem.Acquire(ctx); err != nil {
					f.logger.With("protocol_id", protoID, "remote_peer", remote.String(), "error", err.Error()).Warn("backpressure acquire failed")
					if err := s.Reset(); err != nil {
						f.logger.Warnf("重置流失败: %v", err)
					}
					return
				}
				defer sem.Release()
			}

			// 读取请求帧
			ft, payload, err := stcodec.DecodeFrame(s)
			if err != nil {
				f.logger.With("protocol_id", protoID, "remote_peer", remote.String(), "error", err.Error()).Warn("decode frame failed")
				if err := s.Reset(); err != nil {
					f.logger.Warnf("重置流失败: %v", err)
				}
				return
			}
			_ = ft // 协议层可根据类型区分请求/心跳，这里简单忽略

			// 解析 RpcRequest 并提取 payload
			var reqPB transportpb.RpcRequest
			if uErr := proto.Unmarshal(payload, &reqPB); uErr != nil {
				f.logger.With("protocol_id", protoID, "error", uErr.Error()).Warn("rpc request unmarshal failed")
				if err := s.Reset(); err != nil {
					f.logger.Warnf("重置流失败: %v", err)
				}
				return
			}
			appReq := reqPB.GetEnvelope().GetPayload()
			// 配置的大小限制（若设置）
			if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
				if int64(len(appReq)) > f.cfg.GetMaxMessageSize() {
					f.logger.With("protocol_id", protoID, "size", len(appReq), "max", f.cfg.GetMaxMessageSize()).Warn("inbound payload too large")
					if err := s.Reset(); err != nil {
						f.logger.Warnf("重置流失败: %v", err)
					}
					return
				}
			}

			// 调用处理器（使用包装器增强健壮性）
			respData, handlerErr := wrap.Wrap(handler)(ctx, remote, appReq)

			// 构建 RpcResponse
		respPB := &transportpb.RpcResponse{RequestId: reqPB.GetRequestId()}
		if handlerErr != nil {
			respPB.Status = transportpb.RpcResponse_ERROR
			respPB.ErrorCode = 1 // 示例错误码
			respPB.ErrorMessage = handlerErr.Error()
		} else {
			respPB.Status = transportpb.RpcResponse_OK
			respPB.Envelope = &transportpb.Envelope{Version: 1, ProtocolId: protoID, Payload: respData, Encoding: "raw", Compression: "none"}
		}

		// 序列化并回写响应帧
		bytesOut, mErr := proto.Marshal(respPB)
		if mErr != nil {
			f.logger.With("protocol_id", protoID, "error", mErr.Error()).Warn("rpc response marshal failed")
			if err := s.Reset(); err != nil {
				f.logger.Warnf("重置流失败: %v", err)
			}
			return
		}
		if encErr := stcodec.EncodeFrame(s, stcodec.FrameTypeResponse, bytesOut); encErr != nil {
			f.logger.With("protocol_id", protoID, "remote_peer", remote.String(), "error", encErr.Error()).Warn("encode response failed")
		}
		if err := s.Close(); err != nil {
			f.logger.Warnf("关闭流失败: %v", err)
		}
		})
	}

	for _, id := range ids {
		registerOne(id)
	}

	f.logger.With("protocol_id", protoID).Info("stream handler registered successfully")
	return nil
}

// UnregisterStreamHandler 注销流式协议处理器
func (f *Facade) UnregisterStreamHandler(protoID string) error {
	qualifiedProtoID := f.qualifyProtocolID(protoID)
	ids := []string{qualifiedProtoID}
	if qualifiedProtoID != protoID {
		ids = append(ids, protoID)
	}

	// 清理注册状态
	f.regMu.Lock()
	for _, id := range ids {
		delete(f.registeredProtocols, id)
	}
	f.regMu.Unlock()

	if f.reg != nil {
		for _, id := range ids {
			if err := f.reg.Unregister(id); err != nil {
				f.logger.Warnf("注销协议注册失败: %v", err)
			}
		}
	}
	f.subMu.Lock()
	for _, id := range ids {
		delete(f.regCF, id)
	}
	f.subMu.Unlock()
	for _, id := range ids {
		f.host.RemoveStreamHandler(libprotocol.ID(id))
	}

	f.logger.With("protocol_id", protoID).Info("unregistered stream handler")
	return nil
}

// ==================== 订阅注册（PubSub） ====================

// Subscribe 订阅指定主题
func (f *Facade) Subscribe(topic string, handler iface.SubscribeHandler, opts ...iface.SubscribeOption) (unsubscribe func() error, err error) {
	// 🛡️ 自动为 Topic 添加 namespace（如果配置了 namespace）
	qualifiedTopic := f.qualifyTopic(topic)

	// 检查重复订阅
	f.regMu.Lock()
	if f.registeredTopics[qualifiedTopic] {
		f.regMu.Unlock()
		f.logger.With("topic", qualifiedTopic).Warn("主题已订阅，拒绝重复订阅")
		return nil, fmt.Errorf("主题 %s 已订阅，不允许重复订阅", qualifiedTopic)
	}
	// 标记为已订阅
	f.registeredTopics[qualifiedTopic] = true
	f.regMu.Unlock()

	f.logger.With("topic", qualifiedTopic, "original", topic).Info("subscribing to topic")

	// 解析订阅选项
	var cfg iface.SubscribeConfig
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	// 从全局配置补充默认限制
	if f.cfg != nil {
		if cfg.MaxMessageSize <= 0 && f.cfg.GetMaxMessageSize() > 0 {
			cfg.MaxMessageSize = int(f.cfg.GetMaxMessageSize())
		}
	}
	f.subMu.Lock()
	f.subCF[qualifiedTopic] = cfg
	f.subs[qualifiedTopic] = handler
	f.subMu.Unlock()

	// 配置 validator 规则（大小/签名/频率/去重）
	if f.val != nil {
		rateLimit := 100 // 默认速率
		if cfg.EnableRateLimit {
			rateLimit = 100 // 可根据需要调整
		}
		dedupTTL := time.Minute
		if f.cfg != nil && f.cfg.GetDeduplicationCacheTTL() > 0 {
			dedupTTL = time.Duration(f.cfg.GetDeduplicationCacheTTL()) * time.Second
		}

		f.val.ConfigureTopic(qualifiedTopic, pubimpl.TopicRules{
			MaxMessageSize:   cfg.MaxMessageSize,
			RequireSignature: cfg.EnableSignatureVerification,
			RatePerSec:       rateLimit,
			DedupTTL:         dedupTTL,
		})

		f.logger.With(
			"topic", qualifiedTopic,
			"max_size", cfg.MaxMessageSize,
			"require_signature", cfg.EnableSignatureVerification,
			"rate_limit", cfg.EnableRateLimit,
		).Debug("validator configured")
	}

	// 注册到 TopicManager
	if f.tm != nil {
		if tmErr := f.tm.Subscribe(qualifiedTopic); tmErr != nil {
			f.logger.With("topic", qualifiedTopic, "error", tmErr.Error()).Warn("topic manager subscription failed")
		}
	}

	// 🔧 修复：确保 GossipSub 已初始化
	f.ensureGossipSub()

	// 建立 GossipSub 订阅
	if f.ps == nil {
		f.logger.Infof("gossipsub not ready, host may not be started yet")
		goto DONE
	}

	if f.ps != nil {
		f.psMu.Lock()

		// 确保主题已加入（可能在retry中已经加入）
		if _, ok := f.topicHandles[qualifiedTopic]; !ok {
			if t, e := f.ps.Join(qualifiedTopic); e == nil {
				f.topicHandles[qualifiedTopic] = t
				f.logger.Infof("✅ 成功加入主题: %s", qualifiedTopic)
			} else {
				f.psMu.Unlock()
				f.logger.With("topic", qualifiedTopic, "error", e.Error()).Warn("gossipsub join failed")
				goto DONE
			}
		}

		// 🔧 修复：即使主题已存在，也要创建订阅（如果还没有订阅）
		if _, exists := f.subHandles[qualifiedTopic]; !exists {
			if sub, e := f.ps.Subscribe(qualifiedTopic); e == nil {
				f.subHandles[qualifiedTopic] = sub
				ctx, cancel := context.WithCancel(context.Background())
				f.subCancels[qualifiedTopic] = cancel
				f.psMu.Unlock()

				f.logger.Infof("✅ 主流程创建消息订阅成功: %s", qualifiedTopic)

				// 🔧 修复：订阅成功后延迟强制连接peers，确保其他节点也启动完成
				go func() {
					time.Sleep(10 * time.Second) // 等待其他节点启动完成
					f.requestForceConnect("gossipsub_subscribe")
				}()

				go func() {
					// 使用带 namespace 的完整主题名称进行解码与校验，避免 Envelope.Topic 与逻辑主题不一致
					dec := f.dec
					for {
						msg, err := sub.Next(ctx)
						if err != nil {
							f.logger.Debugf("订阅消息接收结束: topic=%s, error=%v", topic, err)
							return
						}
						if msg == nil {
							continue
						}
						data := msg.GetData()
						f.logger.Debugf("📨 收到gossipsub消息: topic=%s, from=%s, size=%d", topic, msg.ReceivedFrom.String(), len(data))

						// 🛡️ 消息速率限制检查
						peerID := msg.ReceivedFrom.String()
						if f.msgRateLimiter != nil {
							if err := f.msgRateLimiter.CheckMessage(peerID); err != nil {
								f.logger.Warnf("消息速率限制拒绝: topic=%s, peer=%s, error=%v", qualifiedTopic, peerID, err)
								continue
							}
						}

						if f.val != nil {
							// 使用 qualifiedTopic 进行校验，保持与配置的 Topic 规则一致
							if ok, reason := f.val.Validate(qualifiedTopic, data); !ok {
								f.logger.With("topic", qualifiedTopic, "reason", reason).Debug("🚫 gossipsub message dropped")
								continue
							} else {
								f.logger.With("topic", qualifiedTopic).Debug("✅ gossipsub message validated")
							}
						}
						if dec != nil {
							// 🎯 使用 DecodeTopic 解码结构化 Topic
							decodedTopic, payload, derr := dec.DecodeTopic(data)
							if derr == nil {
								f.logger.Debugf("✅ 消息解码成功: decoded_topic=%s, original_size=%d, decoded_size=%d", decodedTopic.String(), len(data), len(payload))
								data = payload
								// 校验解码后的 topic 是否与期望的 qualifiedTopic 匹配
								expectedTopic := parseLegacyTopicString(qualifiedTopic)
								if decodedTopic.Domain != expectedTopic.Domain ||
									decodedTopic.Name != expectedTopic.Name ||
									decodedTopic.Version != expectedTopic.Version {
									f.logger.Warnf("⚠️ topic mismatch: decoded=%s, expect=%s", decodedTopic.String(), qualifiedTopic)
									continue
								}
							} else {
								f.logger.Warnf("⚠️ 消息解码失败: topic=%s, error=%v", qualifiedTopic, derr)
								continue
							}
						}
						f.subMu.RLock()
						h := f.subs[qualifiedTopic]
						f.subMu.RUnlock()
						if h != nil {
							if handlerErr := h(context.Background(), msg.ReceivedFrom, qualifiedTopic, data); handlerErr != nil {
								f.logger.Warnf("订阅处理器执行失败: topic=%s, error=%v", qualifiedTopic, handlerErr)
							}
						} else {
							f.logger.Warnf("未找到订阅处理器: topic=%s", topic)
						}
					}
				}()
			} else {
				f.psMu.Unlock()
				f.logger.With("topic", topic, "error", e.Error()).Warn("gossipsub subscribe failed")
				goto DONE
			}
		} else {
			f.psMu.Unlock()
			f.logger.Infof("topic %s subscription already exists", topic)
		}
	}
DONE:
	f.logger.With("topic", topic).Info("subscription successful")

	return func() error {
		// 这里必须与 Subscribe 时使用的 key 完全一致，统一使用 qualifiedTopic
		f.logger.With("topic", qualifiedTopic, "original", topic).Info("unsubscribing from topic")

		if f.tm != nil {
			if tmErr := f.tm.Unsubscribe(qualifiedTopic); tmErr != nil {
				f.logger.With("topic", qualifiedTopic, "error", tmErr.Error()).Warn("topic manager unsubscription failed")
			}
		}

		// 取消 GossipSub 订阅
		f.psMu.Lock()
		if cancel, ok := f.subCancels[qualifiedTopic]; ok && cancel != nil {
			cancel()
		}
		if sub, ok := f.subHandles[qualifiedTopic]; ok && sub != nil {
			sub.Cancel()
		}
		delete(f.subCancels, qualifiedTopic)
		delete(f.subHandles, qualifiedTopic)
		if t, ok := f.topicHandles[qualifiedTopic]; ok && t != nil {
			if err := t.Close(); err != nil {
				f.logger.Warnf("关闭主题句柄失败: %v", err)
			}
			delete(f.topicHandles, qualifiedTopic)
		}
		f.psMu.Unlock()

		// 清理注册状态
		f.regMu.Lock()
		delete(f.registeredTopics, qualifiedTopic)
		f.regMu.Unlock()

		// 清理本地状态
		f.subMu.Lock()
		delete(f.subs, qualifiedTopic)
		delete(f.subCF, qualifiedTopic)
		f.subMu.Unlock()

		// 清理validator规则
		if f.val != nil {
			f.val.RemoveTopic(qualifiedTopic)
		}

		f.logger.With("topic", qualifiedTopic, "original", topic).Info("unsubscription completed")
		return nil
	}, nil
}

// ==================== 发送 API ====================

// Call 流式请求-响应（点对点）
func (f *Facade) Call(ctx context.Context, to peer.ID, protoID string, req []byte, opts *iface.TransportOptions) ([]byte, error) {
	// 🛡️ 自动为协议 ID 添加 namespace（如果配置了 namespace）
	qualifiedProtoID := f.qualifyProtocolID(protoID)
	f.callCount++
	
	// 🆕 MEDIUM-002 修复：使用协议协商器选择最优协议
	var selectedProto string
	var usedQualified, needFallbackAttempt bool
	if f.protocolNegotiator != nil {
		selectedProto, usedQualified, needFallbackAttempt = f.protocolNegotiator.SelectProtocol(to, protoID, qualifiedProtoID)
	} else {
		selectedProto = qualifiedProtoID
		usedQualified = qualifiedProtoID != protoID
		needFallbackAttempt = usedQualified
	}

	f.logger.Infof("starting call: protocol_id=%s selected=%s target_peer=%s request_size=%d", 
		qualifiedProtoID, selectedProto, to.String(), len(req))
	// 配置大小限制
	if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
		if int64(len(req)) > f.cfg.GetMaxMessageSize() {
			return nil, fmt.Errorf("request too large: %d > %d", len(req), f.cfg.GetMaxMessageSize())
		}
	}
	resolved := stcodec.ResolveTransportOptions(opts)
	maxRetries, connectTO, writeTO, readTO := resolved.MaxRetries, resolved.ConnectTimeout, resolved.WriteTimeout, resolved.ReadTimeout
	retryDelay, backoff := resolved.RetryDelay, resolved.BackoffFactor
	attempt := 0
	requestID := time.Now().Format("20060102T150405.000000000")
	hadFallback := false
	for {
		if attempt > 0 {
			f.retryCount++
			f.logger.With("protocol_id", protoID, "target_peer", to.String(), "attempt", attempt).Warn("retrying call")
		}
		// 连接/建流阶段使用 connectTO（如果提供），避免"上层 ctx 无 deadline 时无限等待"
		connectCtx := ctx
		var connectCancel context.CancelFunc = func() {}
		if connectTO > 0 {
			connectCtx, connectCancel = context.WithTimeout(ctx, connectTO)
		}

		// 确保连接
		netw := f.host.Network()
		if netw != nil && netw.Connectedness(to) != libnetwork.Connected {
			if _, err := netw.DialPeer(connectCtx, to); err != nil {
				f.logger.Warnf("确保连接失败: %v", err)
			}
		}
		
		// 🆕 使用选定的协议
		stream, err := f.host.NewStream(connectCtx, to, libprotocol.ID(selectedProto))
		if err != nil && needFallbackAttempt && selectedProto != protoID {
			// 兼容旧节点：对端可能只支持未加 namespace 的原始协议 ID
			f.logger.With(
				"protocol_id", selectedProto,
				"original", protoID,
				"target_peer", to.String(),
				"error", err.Error(),
			).Warn("qualified protocol not supported by peer, falling back to original protocol id")
			stream, err = f.host.NewStream(connectCtx, to, libprotocol.ID(protoID))
			if err == nil {
				hadFallback = true
				usedQualified = false
			}
		}
		// 释放 connectTO 定时器资源（建流成功/失败都应释放）
		connectCancel()
		if err != nil {
			if attempt < maxRetries {
				sleep := computeBackoff(retryDelay, backoff, attempt)
				select {
				case <-time.After(sleep):
					attempt++
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, err
		}
		// Build RpcRequest with Envelope (pb优先原则)
		env := &transportpb.Envelope{
			Version:       1,
			ProtocolId:    protoID,
			Payload:       req,
			Encoding:      "pb", // 明确使用protobuf
			Compression:   "none",
			CorrelationId: requestID,
			ContentType:   "application/x-protobuf", // pb内容类型
			Timestamp:     uint64(time.Now().UnixMilli()),
		}
		reqPB := &transportpb.RpcRequest{RequestId: requestID, Envelope: env}
		bytesIn, mErr := proto.Marshal(reqPB)
		if mErr != nil {
			if err := stream.Close(); err != nil {
				f.logger.Warnf("关闭流失败: %v", err)
			}
			return nil, mErr
		}
		if writeTO > 0 {
			if err := stream.SetDeadline(time.Now().Add(writeTO)); err != nil {
				f.logger.Warnf("设置写入超时失败: %v", err)
			}
		}
		if err := stcodec.EncodeFrame(stream, stcodec.FrameTypeRequest, bytesIn); err != nil {
			if err := stream.Close(); err != nil {
				f.logger.Warnf("关闭流失败: %v", err)
			}
			if attempt < maxRetries {
				sleep := computeBackoff(retryDelay, backoff, attempt)
				select {
				case <-time.After(sleep):
					attempt++
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, err
		}
		if err := stream.CloseWrite(); err != nil {
			f.logger.Warnf("关闭写入流失败: %v", err)
		}
		if readTO > 0 {
			if err := stream.SetDeadline(time.Now().Add(readTO)); err != nil {
				f.logger.Warnf("设置读取超时失败: %v", err)
			}
		}
		_, payload, rerr := stcodec.DecodeFrame(stream)
		if err := stream.Close(); err != nil {
			f.logger.Warnf("关闭流失败: %v", err)
		}
		if rerr != nil {
			if attempt < maxRetries {
				sleep := computeBackoff(retryDelay, backoff, attempt)
				select {
				case <-time.After(sleep):
					attempt++
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, rerr
		}
		var respPB transportpb.RpcResponse
		if uErr := proto.Unmarshal(payload, &respPB); uErr != nil {
			if attempt < maxRetries {
				sleep := computeBackoff(retryDelay, backoff, attempt)
				select {
				case <-time.After(sleep):
					attempt++
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, uErr
		}
		if respPB.GetStatus() != transportpb.RpcResponse_OK {
			return nil, fmt.Errorf("rpc error: code=%d msg=%s", respPB.GetErrorCode(), respPB.GetErrorMessage())
		}
		if env := respPB.GetEnvelope(); env != nil {
			// 🆕 MEDIUM-002 修复：记录协议协商结果
			if f.protocolNegotiator != nil {
				f.protocolNegotiator.RecordResult(to, usedQualified, hadFallback)
			}
			f.logger.Infof("call completed successfully: protocol_id=%s target_peer=%s response_size=%d attempts=%d fallback=%v", 
				protoID, to.String(), len(env.GetPayload()), attempt+1, hadFallback)
			return env.GetPayload(), nil
		}
		return nil, fmt.Errorf("empty rpc response envelope")
	}
}

// OpenStream 打开长流（用于大体量数据传输等少量场景）
func (f *Facade) OpenStream(ctx context.Context, to peer.ID, protoID string, opts *iface.TransportOptions) (iface.StreamHandle, error) {
	// 确保连接
	netw := f.host.Network()
	if netw != nil && netw.Connectedness(to) != libnetwork.Connected {
		if _, err := netw.DialPeer(ctx, to); err != nil {
			f.logger.Warnf("确保连接失败: %v", err)
		}
	}
	rs, err := f.host.NewStream(ctx, to, libprotocol.ID(protoID))
	if err != nil {
		return nil, err
	}
	return &streamHandleAdapter{stream: rs}, nil
}

// Publish 发布消息到指定主题（发布-订阅）
func (f *Facade) Publish(ctx context.Context, topic string, data []byte, opts *iface.PublishOptions) error {
	// 🛡️ 自动为 Topic 添加 namespace（如果配置了 namespace）
	qualifiedTopic := f.qualifyTopic(topic)
	f.logger.With("topic", qualifiedTopic, "original", topic, "message_size", len(data)).Info("publishing message")
	// 默认大小限制（配置）
	limit := 0
	if opts != nil && opts.MaxMessageSize > 0 {
		limit = opts.MaxMessageSize
	} else if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
		limit = int(f.cfg.GetMaxMessageSize())
	}
	if limit > 0 && len(data) > limit {
		f.dropCount++
		f.logger.With("topic", qualifiedTopic, "message_size", len(data), "max_size", limit).Warn("message too large")
		return nil
	}
	payload := data
	shouldMarkCompressed := false
	// 简化压缩逻辑，基于消息大小判断
	if f.cfg != nil && int64(len(payload)) > f.cfg.GetMaxMessageSize()/4 {
		shouldMarkCompressed = true
	}
	if opts != nil && opts.CompressionEnabled && opts.MaxMessageSize > 0 && len(payload) > opts.MaxMessageSize {
		shouldMarkCompressed = true
	}
	// 直接使用 Encoder 进行 Envelope 封装与编码
	enc, encErr := f.enc.Encode(qualifiedTopic, payload)
	if encErr != nil {
		f.logger.With("topic", qualifiedTopic, "error", encErr.Error()).Warn("encoding failed")
		return encErr
	}
	// 配置要求压缩时，仅标记 compression=gzip（先不改变 payload）
	if shouldMarkCompressed {
		var env transportpb.Envelope
		if err := proto.Unmarshal(enc, &env); err == nil {
			env.Compression = "gzip"
			if b, mErr := proto.Marshal(&env); mErr == nil {
				enc = b
			}
		}
	}
	if f.val != nil {
		ok, reason := f.val.Validate(qualifiedTopic, enc)
		if !ok {
			f.dropCount++
			f.logger.With("topic", qualifiedTopic, "reason", reason).Warn("validation failed")
			return nil
		}
	}

	// 确保 GossipSub 已初始化
	f.ensureGossipSub()

	// GossipSub 广播
	if f.ps != nil {
		// 添加诊断信息：检查主题连接的peers
		peers := f.ps.ListPeers(qualifiedTopic)
		f.logger.Infof("🔍 准备发布到主题 %s, 连接的peers数量: %d", qualifiedTopic, len(peers))
		if len(peers) > 0 {
			f.logger.Infof("📡 主题 %s 连接的peers: %v", qualifiedTopic, peers)
		} else {
			f.logger.Warnf("⚠️ 主题 %s 没有连接的peers! 消息可能无法传递给其他节点", qualifiedTopic)
		}

		f.psMu.Lock()
		if t, ok := f.topicHandles[qualifiedTopic]; ok && t != nil {
			f.psMu.Unlock()
			if err := t.Publish(ctx, enc); err != nil {
				f.logger.With("topic", qualifiedTopic, "error", err.Error()).Warn("gossipsub publish failed")
			}
		} else {
			f.psMu.Unlock()
			if err := f.ps.Publish(qualifiedTopic, enc); err != nil {
				f.logger.With("topic", qualifiedTopic, "error", err.Error()).Warn("gossipsub direct publish failed")
			}
		}
	}
	// 本地统计与回环通知
	if f.pub != nil {
		if err := f.pub.Publish(qualifiedTopic, enc); err != nil {
			f.logger.Warnf("发布消息失败: %v", err)
		}
	}
	f.subMu.RLock()
	h := f.subs[qualifiedTopic]
	f.subMu.RUnlock()
	if h != nil {
		// 🔧 修复：检查是否为单节点环境，决定是否执行本地回环处理
		peers := f.ps.ListPeers(qualifiedTopic)
		if len(peers) <= 1 {
			// 单节点环境：执行本地回环处理，用于单节点测试
			f.logger.Debugf("单节点环境，执行本地回环处理: topic=%s, peers=%d", qualifiedTopic, len(peers))

			// 🔧 修复：本地回环时需要先解码数据，避免protobuf解析错误
			decodedPayload, decErr := f.dec.Decode(qualifiedTopic, enc)
			if decErr != nil {
				f.logger.With("topic", qualifiedTopic, "error", decErr.Error()).Warn("local handler decode failed")
			} else {
				if localErr := h(ctx, peer.ID(""), qualifiedTopic, decodedPayload); localErr != nil {
					f.logger.With("topic", qualifiedTopic, "error", localErr.Error()).Warn("local handler failed")
				}
			}
		} else {
			// 多节点环境：跳过本地回环处理，避免重复处理
			// 节点会通过网络接收并处理自己发布的消息
			f.logger.Debugf("多节点环境，跳过本地回环处理: topic=%s, peers=%d", topic, len(peers))
		}
	}
	f.pubCount++
	f.logger.With("topic", topic, "final_size", len(enc), "compressed_mark", shouldMarkCompressed).Info("message published successfully")
	return nil
}

// PublishTopic 基于结构化 Topic 定义发布消息。
//
// 🎯 破坏性重构：直接使用结构化 Topic 字段，不再转换为字符串
func (f *Facade) PublishTopic(ctx context.Context, t protocols.Topic, data []byte, opts *iface.PublishOptions) error {
	// 🛡️ 自动为 Topic 添加 namespace（如果配置了 namespace）
	qualifiedTopic := t.WithNamespace(f.networkNamespace)

	qualifiedTopicStr := qualifiedTopic.String()
	if qualifiedTopicStr == "" {
		return fmt.Errorf("PublishTopic: invalid topic definition: %+v", t)
	}

	f.logger.With(
		"topic", qualifiedTopicStr,
		"namespace", qualifiedTopic.Namespace,
		"domain", qualifiedTopic.Domain,
		"name", qualifiedTopic.Name,
		"version", qualifiedTopic.Version,
		"message_size", len(data),
	).Info("publishing message")

	// 默认大小限制（配置）
	limit := 0
	if opts != nil && opts.MaxMessageSize > 0 {
		limit = opts.MaxMessageSize
	} else if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
		limit = int(f.cfg.GetMaxMessageSize())
	}
	if limit > 0 && len(data) > limit {
		f.dropCount++
		f.logger.With("topic", qualifiedTopicStr, "message_size", len(data), "max_size", limit).Warn("message too large")
		return nil
	}

	payload := data
	shouldMarkCompressed := false
	if f.cfg != nil && int64(len(payload)) > f.cfg.GetMaxMessageSize()/4 {
		shouldMarkCompressed = true
	}
	if opts != nil && opts.CompressionEnabled && opts.MaxMessageSize > 0 && len(payload) > opts.MaxMessageSize {
		shouldMarkCompressed = true
	}

	// 🎯 使用 EncodeTopic 直接编码结构化 Topic
	enc, encErr := f.enc.EncodeTopic(qualifiedTopic, payload)
	if encErr != nil {
		f.logger.With("topic", qualifiedTopicStr, "error", encErr.Error()).Warn("encoding failed")
		return encErr
	}

	// 配置要求压缩时，仅标记 compression=gzip（先不改变 payload）
	if shouldMarkCompressed {
		var env transportpb.Envelope
		if err := proto.Unmarshal(enc, &env); err == nil {
			env.Compression = "gzip"
			if b, mErr := proto.Marshal(&env); mErr == nil {
				enc = b
			}
		}
	}

	if f.val != nil {
		ok, reason := f.val.Validate(qualifiedTopicStr, enc)
		if !ok {
			f.dropCount++
			f.logger.With("topic", qualifiedTopicStr, "reason", reason).Warn("validation failed")
			return nil
		}
	}

	// 确保 GossipSub 已初始化
	f.ensureGossipSub()

	// GossipSub 广播
	if f.ps != nil {
		peers := f.ps.ListPeers(qualifiedTopicStr)
		f.logger.Infof("🔍 准备发布到主题 %s, 连接的peers数量: %d", qualifiedTopicStr, len(peers))
		if len(peers) > 0 {
			f.logger.Infof("📡 主题 %s 连接的peers: %v", qualifiedTopicStr, peers)
		} else {
			f.logger.Warnf("⚠️ 主题 %s 没有连接的peers! 消息可能无法传递给其他节点", qualifiedTopicStr)
		}

		f.psMu.Lock()
		if t, ok := f.topicHandles[qualifiedTopicStr]; ok && t != nil {
			f.psMu.Unlock()
			if err := t.Publish(ctx, enc); err != nil {
				f.logger.With("topic", qualifiedTopicStr, "error", err.Error()).Warn("gossipsub publish failed")
			}
		} else {
			f.psMu.Unlock()
			if err := f.ps.Publish(qualifiedTopicStr, enc); err != nil {
				f.logger.With("topic", qualifiedTopicStr, "error", err.Error()).Warn("gossipsub direct publish failed")
			}
		}
	}

	// 本地统计与回环通知
	if f.pub != nil {
		if err := f.pub.Publish(qualifiedTopicStr, enc); err != nil {
			f.logger.Warnf("发布消息失败: %v", err)
		}
	}

	f.subMu.RLock()
	h := f.subs[qualifiedTopicStr]
	f.subMu.RUnlock()
	if h != nil {
		peers := f.ps.ListPeers(qualifiedTopicStr)
		if len(peers) <= 1 {
			f.logger.Debugf("单节点环境，执行本地回环处理: topic=%s, peers=%d", qualifiedTopicStr, len(peers))

			// 🎯 使用 DecodeTopic 解码
			decodedTopic, decodedPayload, decErr := f.dec.DecodeTopic(enc)
			if decErr != nil {
				f.logger.With("topic", qualifiedTopicStr, "error", decErr.Error()).Warn("local handler decode failed")
			} else {
				if localErr := h(ctx, peer.ID(""), decodedTopic.String(), decodedPayload); localErr != nil {
					f.logger.With("topic", qualifiedTopicStr, "error", localErr.Error()).Warn("local handler failed")
				}
			}
		} else {
			f.logger.Debugf("多节点环境，跳过本地回环处理: topic=%s, peers=%d", qualifiedTopicStr, len(peers))
		}
	}

	f.pubCount++
	f.logger.With("topic", qualifiedTopicStr, "final_size", len(enc), "compressed_mark", shouldMarkCompressed).Info("message published successfully")
	return nil
}

// SubscribeTopic 基于结构化 Topic 定义订阅主题。
//
// 🎯 破坏性重构：直接使用结构化 Topic，内部转换为字符串用于 GossipSub
func (f *Facade) SubscribeTopic(t protocols.Topic, handler iface.SubscribeHandler, opts ...iface.SubscribeOption) (func() error, error) {
	// ⚠️ 重要：避免“双重 namespace 化”
	//
	// - Subscribe(topicStr) 内部会对字符串 topic 做 qualifyTopic()（即 weisyn.{ns}.xxx）
	// - 如果这里先 WithNamespace 再传给 Subscribe，会导致再次 QualifyTopic → weisyn.{ns}.{ns}.xxx
	//
	// 因此这里**只取基础 topic 字符串**，让 Subscribe 负责做一次（且仅一次）命名空间化。
	topicStr := t.String()
	if topicStr == "" {
		return nil, fmt.Errorf("SubscribeTopic: invalid topic definition: %+v", t)
	}
	// 暂时仍调用 Subscribe，但后续会完全移除 Subscribe 方法
	return f.Subscribe(topicStr, handler, opts...)
}

// parseLegacyTopicString 解析旧格式的 topic 字符串为结构化 Topic（辅助函数）
func parseLegacyTopicString(topic string) protocols.Topic {
	parts := strings.Split(topic, ".")
	if len(parts) < 4 || parts[0] != "weisyn" {
		return protocols.Topic{}
	}
	if len(parts) == 5 {
		return protocols.Topic{
			Namespace: parts[1],
			Domain:    parts[2],
			Name:      parts[3],
			Version:   parts[4],
		}
	} else if len(parts) == 4 {
		return protocols.Topic{
			Domain:  parts[1],
			Name:    parts[2],
			Version: parts[3],
		}
	}
	return protocols.Topic{}
}

// ==================== 自检/诊断（非指标） ====================

// ListProtocols 列出已注册的协议信息（用于诊断）
func (f *Facade) ListProtocols() []iface.ProtocolInfo {
	if f.reg == nil {
		return nil
	}
	return f.reg.List()
}

// GetProtocolInfo 获取指定协议的详细信息（用于诊断）
func (f *Facade) GetProtocolInfo(protoID string) *iface.ProtocolInfo {
	if f.reg == nil {
		return nil
	}
	info, ok := f.reg.Info(protoID)
	if !ok {
		return nil
	}
	return info
}

// ==================== 辅助 ====================

func computeBackoff(base time.Duration, factor float64, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	mul := 1.0
	for i := 0; i < attempt; i++ {
		mul *= factor
	}
	return time.Duration(float64(base) * mul)
}

// Stop 停止网络门面及其安全组件
func (f *Facade) Stop() {
	if f.rateLimiter != nil {
		f.rateLimiter.Stop()
	}
	if f.msgRateLimiter != nil {
		f.msgRateLimiter.Stop()
	}
	// 停止 Validator 清理协程
	if f.validatorCleanupStop != nil {
		close(f.validatorCleanupStop)
		f.validatorCleanupStop = nil
	}

	// 停止 forceConnect loop
	f.forceConnectMu.Lock()
	if f.forceConnectStopCancel != nil {
		f.forceConnectStopCancel()
		f.forceConnectStopCancel = nil
	}
	f.forceConnectReqCh = nil
	f.forceConnectMu.Unlock()
}

// ==================== StreamHandle 适配器（内部使用） ====================

// streamHandleAdapter 将 libp2p.Stream 适配为 iface.StreamHandle
type streamHandleAdapter struct {
	stream libnetwork.Stream
}

func (s *streamHandleAdapter) Read(p []byte) (int, error)    { return s.stream.Read(p) }
func (s *streamHandleAdapter) Write(p []byte) (int, error)   { return s.stream.Write(p) }
func (s *streamHandleAdapter) Close() error                  { return s.stream.Close() }
func (s *streamHandleAdapter) CloseWrite() error             { return s.stream.CloseWrite() }
func (s *streamHandleAdapter) Reset() error                  { return s.stream.Reset() }
func (s *streamHandleAdapter) SetDeadline(t time.Time) error { return s.stream.SetDeadline(t) }

// ==================== 辅助：占位日志器 ====================

// noopLogger 占位日志器（当 logger 为 nil 时使用）
type noopLogger struct{}

func (l *noopLogger) Debug(msg string)                          {}
func (l *noopLogger) Debugf(format string, args ...interface{}) {}
func (l *noopLogger) Info(msg string)                           {}
func (l *noopLogger) Infof(format string, args ...interface{})  {}
func (l *noopLogger) Warn(msg string)                           {}
func (l *noopLogger) Warnf(format string, args ...interface{})  {}
func (l *noopLogger) Error(msg string)                          {}
func (l *noopLogger) Errorf(format string, args ...interface{}) {}
func (l *noopLogger) Fatal(msg string)                          {}
func (l *noopLogger) Fatalf(format string, args ...interface{}) {}
func (l *noopLogger) With(args ...interface{}) logiface.Logger  { return l }
func (l *noopLogger) Sync() error                               { return nil }
func (l *noopLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// GetTopicPeers 获取指定主题连接的节点列表
func (f *Facade) GetTopicPeers(topic string) []peer.ID {
	f.psMu.Lock()
	defer f.psMu.Unlock()

	// ✅ 与 Subscribe/Publish 统一：查询时也做 namespace 化（幂等）
	// 这能保证：调用方传入 weisyn.consensus.latest_block.v1 也能查到已 join 的 weisyn.{ns}.consensus.latest_block.v1
	qualifiedTopic := f.qualifyTopic(topic)
	f.logger.Infof("🔍 GetTopicPeers 被调用: topic=%s qualified=%s", topic, qualifiedTopic)

	if f.ps == nil {
		f.logger.Infof("❌ GossipSub未初始化，无法获取主题节点列表: %s", topic)
		return []peer.ID{}
	}

	f.logger.Infof("✅ GossipSub已初始化，topicHandles数量: %d", len(f.topicHandles))

	// 获取主题handle
	topicHandle, exists := f.topicHandles[qualifiedTopic]
	if !exists {
		f.logger.Infof("❌ 主题未加入，无法获取节点列表: %s (qualified=%s), 可用主题: %v", topic, qualifiedTopic, func() []string {
			var topics []string
			for t := range f.topicHandles {
				topics = append(topics, t)
			}
			return topics
		}())
		return []peer.ID{}
	}

	f.logger.Infof("✅ 找到主题handle: %s", qualifiedTopic)

	// 获取连接到该主题的节点
	peers := topicHandle.ListPeers()
	f.logger.Infof("📊 主题 %s 连接的节点数量: %d", qualifiedTopic, len(peers))

	// 打印节点ID详情
	for i, peerID := range peers {
		f.logger.Infof("  - 节点%d: %s", i+1, peerID.String())
	}

	return peers
}

// IsSubscribed 检查是否已订阅指定主题
func (f *Facade) IsSubscribed(topic string) bool {
	f.regMu.RLock()
	defer f.regMu.RUnlock()

	// 检查是否在已注册主题列表中
	return f.registeredTopics[topic]
}

// CheckProtocolSupport 检查对等节点是否支持指定协议
func (f *Facade) CheckProtocolSupport(ctx context.Context, peerID peer.ID, protocol string) (bool, error) {
	if f.host == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	// 检查节点是否已连接
	netw := f.host.Network()
	if netw != nil && netw.Connectedness(peerID) != libnetwork.Connected {
		// 尝试连接到目标节点（如果未连接）
		if _, err := netw.DialPeer(ctx, peerID); err != nil {
			return false, fmt.Errorf("failed to connect to peer %s: %v", peerID, err)
		}
	}

	// 获取节点支持的协议
	protocols, err := f.host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// namespace 迁移期兼容：同时检查 original 与 qualified 两种协议ID
	candidates := map[string]struct{}{}
	if protocol != "" {
		candidates[protocol] = struct{}{}
	}
	if qp := f.qualifyProtocolID(protocol); qp != "" {
		candidates[qp] = struct{}{}
	}
	// 若传入的是 qualified，则补一个 dequalify（仅当匹配本节点 namespace 时才去除）
	if f.networkNamespace != "" && strings.HasPrefix(protocol, "/weisyn/"+f.networkNamespace+"/") {
		orig := "/weisyn/" + protocol[len("/weisyn/"+f.networkNamespace):] // keep the leading "/" from the remainder
		candidates[orig] = struct{}{}
	}

	// 检查是否支持目标协议（候选集任一命中即认为支持）
	for _, p := range protocols {
		if _, ok := candidates[string(p)]; ok {
			f.logger.Debugf("节点 %s 支持协议: %s", peerID, string(p))
			return true, nil
		}
	}

	f.logger.Debugf("节点 %s 不支持协议: %s（candidates=%v），支持的协议: %v", peerID, protocol, func() []string {
		out := make([]string, 0, len(candidates))
		for c := range candidates {
			out = append(out, c)
		}
		return out
	}(), protocols)
	return false, nil
}

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (f *Facade) ModuleName() string {
	return "network"
}

// CollectMemoryStats 收集 Network 模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 当前连接数 / session 数
// - ApproxBytes: 网络缓冲区估算（接收/发送队列）
// - CacheItems: 协议注册表、主题订阅等缓存条目
// - QueueLength: 内部 message 队列长度
func (f *Facade) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 统计连接数（从 host.Network() 获取真实连接数量）
	connCount := int64(0)
	if f.host != nil {
		if netw := f.host.Network(); netw != nil {
			conns := netw.Conns()
			connCount = int64(len(conns))
		}
	}

	// 统计订阅数量
	f.subMu.RLock()
	subCount := int64(len(f.subs))
	f.subMu.RUnlock()

	// 统计注册的协议和主题数量
	f.regMu.RLock()
	protocolCount := int64(len(f.registeredProtocols))
	topicCount := int64(len(f.registeredTopics))
	f.regMu.RUnlock()

	// 统计 GossipSub 主题和订阅句柄
	f.psMu.Lock()
	topicHandleCount := int64(len(f.topicHandles))
	subHandleCount := int64(len(f.subHandles))
	f.psMu.Unlock()

	objects := connCount + subCount

	// 📌 暂不对网络缓冲区做 bytes 级别估算，以避免使用固定常数误导分析。
	// 实际内存占用请结合：
	// - runtime.MemStats
	// - objects/cacheItems（连接数、订阅数、协议/主题数）
	approxBytes := int64(0)

	// 缓存条目：协议注册表、主题订阅等
	cacheItems := protocolCount + topicCount + topicHandleCount + subHandleCount

	// 队列长度：内部消息队列长度（估算，实际应该从 streamSvc 获取）
	queueLength := int64(0) // 简化估算

	return metricsiface.ModuleMemoryStats{
		Module:      "network",
		Layer:       "L2-Infrastructure",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: queueLength,
	}
}
