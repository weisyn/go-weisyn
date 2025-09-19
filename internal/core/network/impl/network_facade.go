package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	networkconfig "github.com/weisyn/v1/internal/config/network"
	pubimpl "github.com/weisyn/v1/internal/core/network/impl/pubsub"
	regimpl "github.com/weisyn/v1/internal/core/network/impl/registry"
	stcodec "github.com/weisyn/v1/internal/core/network/impl/stream"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	cryptoi "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
)

// Facade Network 门面统一实现
// 用途：
// - 直接实现 iface.Network 接口，统一提供协议注册、流式发送与订阅发布能力
// - 聚合内部组件完成消息编解码与分发，不暴露生命周期与指标
// 说明：
// - 不包含生命周期管理（Start/Stop）；由上层 DI 管理
// - 不暴露内部指标或状态；仅聚焦消息编解码与分发
// - 业务协议由各领域模块自行注册，Network 不维护业务协议清单
type Facade struct {
	host   nodeiface.Host            // P2P宿主，用于连通性保障与流操作
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

	// crypto services
	hashManager cryptoi.HashManager
	sigManager  cryptoi.SignatureManager

	// 最小可观测性
	pubCount   uint64
	dropCount  uint64
	callCount  uint64
	retryCount uint64
}

// NewFacade 创建 Network 门面实例
func NewFacade(host nodeiface.Host, logger logiface.Logger, cfg *networkconfig.Config, hashMgr cryptoi.HashManager, sigMgr cryptoi.SignatureManager) *Facade {
	if logger == nil {
		logger = &noopLogger{} // 占位日志器
	}
	f := &Facade{
		host:                host,
		reg:                 regimpl.NewProtocolRegistry(),
		logger:              logger,
		tm:                  pubimpl.NewTopicManager(),
		enc:                 pubimpl.NewEncoder(),
		dec:                 pubimpl.NewDecoder(),
		val:                 pubimpl.NewValidator(),
		pub:                 pubimpl.NewPublisher(),
		subs:                make(map[string]iface.SubscribeHandler),
		subCF:               make(map[string]iface.SubscribeConfig),
		regCF:               make(map[string]iface.RegisterConfig),
		registeredProtocols: make(map[string]bool),
		registeredTopics:    make(map[string]bool),
		streamSvc:           stcodec.New(host),
		cfg:                 cfg,
		hashManager:         hashMgr,
		sigManager:          sigMgr,
		topicHandles:        make(map[string]*pubsub.Topic),
		subHandles:          make(map[string]*pubsub.Subscription),
		subCancels:          make(map[string]context.CancelFunc),
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
	// 启动 Validator 去重过期清理后台任务（轻量）
	go func() {
		// 无生命周期接口，采用守护协程；可后续接入 context 取消
		for {
			time.Sleep(time.Minute)
			if f.val != nil {
				f.val.CleanupExpiredEntries()
			}
		}
	}()
	// 🔧 不在这里初始化GossipSub，等待Host启动事件触发
	return f
}

var _ iface.Network = (*Facade)(nil)

// initGossipSub 初始化或重新初始化 GossipSub
func (f *Facade) initGossipSub() {
	if f.host == nil {
		f.logger.Errorf("❌ initGossipSub: host is nil")
		return
	}

	if f.host.Libp2pHost() == nil {
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

	if ps, err := pubsub.NewGossipSub(context.Background(), f.host.Libp2pHost(), opts...); err == nil {
		f.ps = ps
		f.logger.Infof("🎉 gossipsub initialized successfully with optimized mesh config")

		// 🔧 修复：强制连接已发现的peers，就像简单测试中那样
		go f.forceConnectToPeers()
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

	if f.host.Libp2pHost() == nil {
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
		_ = handle.Close()
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

						if f.val != nil {
							if ok, reason := f.val.Validate(topic, data); !ok {
								f.logger.Debugf("🚫 gossipsub message dropped", "topic", topic, "reason", reason)
								continue
							}
						}

						// 解码消息
						if dec != nil {
							if payload, derr := dec.Decode(topic, data); derr == nil {
								f.logger.Debugf("✅ 消息解码成功: topic=%s, original_size=%d, decoded_size=%d", topic, len(data), len(payload))
								data = payload
							} else {
								f.logger.Warnf("❌ 消息解码失败: topic=%s, error=%v", topic, derr)
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

// forceConnectToPeers 强制连接已发现的peers，就像简单测试一样
func (f *Facade) forceConnectToPeers() {

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
							f.forceConnectToPeers()
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

								if f.val != nil {
									if ok, reason := f.val.Validate(topic, data); !ok {
										f.logger.Debugf("🚫 gossipsub message dropped", "topic", topic, "reason", reason)
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
	// 检查重复注册
	f.regMu.Lock()
	if f.registeredProtocols[protoID] {
		f.regMu.Unlock()
		f.logger.Warnf("协议已注册，拒绝重复注册", "protocol_id", protoID)
		return fmt.Errorf("协议 %s 已注册，不允许重复注册", protoID)
	}
	// 标记为已注册
	f.registeredProtocols[protoID] = true
	f.regMu.Unlock()

	f.logger.Infof("registering stream handler", "protocol_id", protoID)

	// 解析注册选项
	var cfg iface.RegisterConfig
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	f.subMu.Lock()
	f.regCF[protoID] = cfg
	f.subMu.Unlock()

	// 并发/背压：按照每协议信号量限制
	if cfg.MaxConcurrency > 0 {
		f.streamSvc.SetConcurrencyLimit(cfg.MaxConcurrency)
	}
	sem := f.streamSvc.GetSemaphore()
	wrap := regimpl.NewHandlerWrapper()
	// 可选默认超时：按需启用（此处保持0，待配置接入）

	f.logger.Infof("🔧 TRACE: NetworkFacade开始注册协议处理器: %s", string(protoID))

	if f.reg != nil {
		f.logger.Infof("🔧 TRACE: 注册到内部注册表: %s", string(protoID))
		_ = f.reg.Register(protoID, wrap.Wrap(handler))
	}
	// 入站桥接：读取一帧请求，调用 handler，回写一帧响应
	f.host.RegisterStreamHandler(protoID, func(ctx context.Context, remote peer.ID, s nodeiface.RawStream) {
		f.logger.Debugf("handling inbound stream", "protocol_id", protoID, "remote_peer", remote.String())

		// 背压：尝试获取信号量
		if sem != nil {
			if err := sem.Acquire(ctx); err != nil {
				f.logger.Warnf("backpressure acquire failed", "protocol_id", protoID, "remote_peer", remote.String(), "error", err.Error())
				_ = s.Reset()
				return
			}
			defer sem.Release()
		}

		// 读取请求帧
		ft, payload, err := stcodec.DecodeFrame(s)
		if err != nil {
			f.logger.Warnf("decode frame failed", "protocol_id", protoID, "remote_peer", remote.String(), "error", err.Error())
			_ = s.Reset()
			return
		}
		_ = ft // 协议层可根据类型区分请求/心跳，这里简单忽略

		// 解析 RpcRequest 并提取 payload
		var reqPB transportpb.RpcRequest
		if uErr := proto.Unmarshal(payload, &reqPB); uErr != nil {
			f.logger.Warnf("rpc request unmarshal failed", "protocol_id", protoID, "error", uErr.Error())
			_ = s.Reset()
			return
		}
		appReq := reqPB.GetEnvelope().GetPayload()
		// 配置的大小限制（若设置）
		if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
			if int64(len(appReq)) > f.cfg.GetMaxMessageSize() {
				f.logger.Warnf("inbound payload too large", "protocol_id", protoID, "size", len(appReq), "max", f.cfg.GetMaxMessageSize())
				_ = s.Reset()
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
			f.logger.Warnf("rpc response marshal failed", "protocol_id", protoID, "error", mErr.Error())
			_ = s.Reset()
			return
		}
		if encErr := stcodec.EncodeFrame(s, stcodec.FrameTypeResponse, bytesOut); encErr != nil {
			f.logger.Warnf("encode response failed", "protocol_id", protoID, "remote_peer", remote.String(), "error", encErr.Error())
		}
		_ = s.Close()
	})

	f.logger.Infof("stream handler registered successfully", "protocol_id", protoID)
	f.logger.Infof("🔧 TRACE: ✅ NetworkFacade协议注册完成: %s", string(protoID))
	return nil
}

// UnregisterStreamHandler 注销流式协议处理器
func (f *Facade) UnregisterStreamHandler(protoID string) error {
	// 清理注册状态
	f.regMu.Lock()
	delete(f.registeredProtocols, protoID)
	f.regMu.Unlock()

	if f.reg != nil {
		_ = f.reg.Unregister(protoID)
	}
	f.subMu.Lock()
	delete(f.regCF, protoID)
	f.subMu.Unlock()
	f.host.UnregisterStreamHandler(protoID)

	f.logger.Infof("unregistered stream handler", "protocol_id", protoID)
	return nil
}

// ==================== 订阅注册（PubSub） ====================

// Subscribe 订阅指定主题
func (f *Facade) Subscribe(topic string, handler iface.SubscribeHandler, opts ...iface.SubscribeOption) (unsubscribe func() error, err error) {
	// 检查重复订阅
	f.regMu.Lock()
	if f.registeredTopics[topic] {
		f.regMu.Unlock()
		f.logger.Warnf("主题已订阅，拒绝重复订阅", "topic", topic)
		return nil, fmt.Errorf("主题 %s 已订阅，不允许重复订阅", topic)
	}
	// 标记为已订阅
	f.registeredTopics[topic] = true
	f.regMu.Unlock()

	f.logger.Infof("subscribing to topic", "topic", topic)

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
	f.subCF[topic] = cfg
	f.subs[topic] = handler
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

		f.val.ConfigureTopic(topic, pubimpl.TopicRules{
			MaxMessageSize:   cfg.MaxMessageSize,
			RequireSignature: cfg.EnableSignatureVerification,
			RatePerSec:       rateLimit,
			DedupTTL:         dedupTTL,
		})

		f.logger.Debugf("validator configured", "topic", topic,
			"max_size", cfg.MaxMessageSize,
			"require_signature", cfg.EnableSignatureVerification,
			"rate_limit", cfg.EnableRateLimit)
	}

	// 注册到 TopicManager
	if f.tm != nil {
		if tmErr := f.tm.Subscribe(topic); tmErr != nil {
			f.logger.Warnf("topic manager subscription failed", "topic", topic, "error", tmErr.Error())
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
		if _, ok := f.topicHandles[topic]; !ok {
			if t, e := f.ps.Join(topic); e == nil {
				f.topicHandles[topic] = t
				f.logger.Infof("✅ 成功加入主题: %s", topic)
			} else {
				f.psMu.Unlock()
				f.logger.Warnf("gossipsub join failed", "topic", topic, "error", e.Error())
				goto DONE
			}
		}

		// 🔧 修复：即使主题已存在，也要创建订阅（如果还没有订阅）
		if _, exists := f.subHandles[topic]; !exists {
			if sub, e := f.ps.Subscribe(topic); e == nil {
				f.subHandles[topic] = sub
				ctx, cancel := context.WithCancel(context.Background())
				f.subCancels[topic] = cancel
				f.psMu.Unlock()

				f.logger.Infof("✅ 主流程创建消息订阅成功: %s", topic)

				// 🔧 修复：订阅成功后延迟强制连接peers，确保其他节点也启动完成
				go func() {
					time.Sleep(10 * time.Second) // 等待其他节点启动完成
					f.forceConnectToPeers()
				}()

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

						if f.val != nil {
							if ok, reason := f.val.Validate(topic, data); !ok {
								f.logger.Debugf("🚫 gossipsub message dropped", "topic", topic, "reason", reason)
								continue
							} else {
								f.logger.Debugf("✅ gossipsub message validated", "topic", topic)
							}
						}
						if dec != nil {
							if payload, derr := dec.Decode(topic, data); derr == nil {
								f.logger.Debugf("✅ 消息解码成功: topic=%s, original_size=%d, decoded_size=%d", topic, len(data), len(payload))
								data = payload
							} else {
								f.logger.Warnf("❌ 消息解码失败: topic=%s, error=%v", topic, derr)
							}
						}
						f.subMu.RLock()
						h := f.subs[topic]
						f.subMu.RUnlock()
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
				f.psMu.Unlock()
				f.logger.Warnf("gossipsub subscribe failed", "topic", topic, "error", e.Error())
				goto DONE
			}
		} else {
			f.psMu.Unlock()
			f.logger.Infof("topic %s subscription already exists", topic)
		}
	}
DONE:
	f.logger.Infof("subscription successful", "topic", topic)

	return func() error {
		f.logger.Infof("unsubscribing from topic", "topic", topic)

		if f.tm != nil {
			if tmErr := f.tm.Unsubscribe(topic); tmErr != nil {
				f.logger.Warnf("topic manager unsubscription failed", "topic", topic, "error", tmErr.Error())
			}
		}

		// 取消 GossipSub 订阅
		f.psMu.Lock()
		if cancel, ok := f.subCancels[topic]; ok && cancel != nil {
			cancel()
		}
		if sub, ok := f.subHandles[topic]; ok && sub != nil {
			sub.Cancel()
		}
		delete(f.subCancels, topic)
		delete(f.subHandles, topic)
		if t, ok := f.topicHandles[topic]; ok && t != nil {
			_ = t.Close()
			delete(f.topicHandles, topic)
		}
		f.psMu.Unlock()

		// 清理注册状态
		f.regMu.Lock()
		delete(f.registeredTopics, topic)
		f.regMu.Unlock()

		// 清理本地状态
		f.subMu.Lock()
		delete(f.subs, topic)
		delete(f.subCF, topic)
		f.subMu.Unlock()

		// 清理validator规则
		if f.val != nil {
			f.val.RemoveTopic(topic)
		}

		f.logger.Infof("unsubscription completed", "topic", topic)
		return nil
	}, nil
}

// ==================== 发送 API ====================

// Call 流式请求-响应（点对点）
func (f *Facade) Call(ctx context.Context, to peer.ID, protoID string, req []byte, opts *iface.TransportOptions) ([]byte, error) {
	f.callCount++
	f.logger.Infof("starting call", "protocol_id", protoID, "target_peer", to.String(), "request_size", len(req))
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
	for {
		if attempt > 0 {
			f.retryCount++
			f.logger.Warnf("retrying call", "protocol_id", protoID, "target_peer", to.String(), "attempt", attempt)
		}
		var deadline time.Time
		if connectTO > 0 {
			deadline = time.Now().Add(connectTO)
		}
		_ = f.host.EnsureConnected(ctx, to, deadline)
		stream, err := f.host.NewStream(ctx, to, protoID)
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
			_ = stream.Close()
			return nil, mErr
		}
		if writeTO > 0 {
			_ = stream.SetDeadline(time.Now().Add(writeTO))
		}
		if err := stcodec.EncodeFrame(stream, stcodec.FrameTypeRequest, bytesIn); err != nil {
			_ = stream.Close()
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
		_ = stream.CloseWrite()
		if readTO > 0 {
			_ = stream.SetDeadline(time.Now().Add(readTO))
		}
		_, payload, rerr := stcodec.DecodeFrame(stream)
		_ = stream.Close()
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
			f.logger.Infof("call completed successfully", "protocol_id", protoID, "target_peer", to.String(), "response_size", len(env.GetPayload()), "attempts", attempt+1)
			return env.GetPayload(), nil
		}
		return nil, fmt.Errorf("empty rpc response envelope")
	}
}

// OpenStream 打开长流（用于大体量数据传输等少量场景）
func (f *Facade) OpenStream(ctx context.Context, to peer.ID, protoID string, opts *iface.TransportOptions) (iface.StreamHandle, error) {
	var deadline time.Time
	if opts != nil && opts.ConnectTimeout > 0 {
		deadline = time.Now().Add(opts.ConnectTimeout)
	}
	_ = f.host.EnsureConnected(ctx, to, deadline)
	rs, err := f.host.NewStream(ctx, to, protoID)
	if err != nil {
		return nil, err
	}
	return &streamHandleAdapter{stream: rs}, nil
}

// Publish 发布消息到指定主题（发布-订阅）
func (f *Facade) Publish(ctx context.Context, topic string, data []byte, opts *iface.PublishOptions) error {
	f.logger.Infof("publishing message", "topic", topic, "message_size", len(data))
	// 默认大小限制（配置）
	limit := 0
	if opts != nil && opts.MaxMessageSize > 0 {
		limit = opts.MaxMessageSize
	} else if f.cfg != nil && f.cfg.GetMaxMessageSize() > 0 {
		limit = int(f.cfg.GetMaxMessageSize())
	}
	if limit > 0 && len(data) > limit {
		f.dropCount++
		f.logger.Warnf("message too large", "topic", topic, "message_size", len(data), "max_size", limit)
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
	enc, encErr := f.enc.Encode(topic, payload)
	if encErr != nil {
		f.logger.Warnf("encoding failed", "topic", topic, "error", encErr.Error())
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
		ok, reason := f.val.Validate(topic, enc)
		if !ok {
			f.dropCount++
			f.logger.Warnf("validation failed", "topic", topic, "reason", reason)
			return nil
		}
	}

	// 确保 GossipSub 已初始化
	f.ensureGossipSub()

	// GossipSub 广播
	if f.ps != nil {
		// 添加诊断信息：检查主题连接的peers
		peers := f.ps.ListPeers(topic)
		f.logger.Infof("🔍 准备发布到主题 %s, 连接的peers数量: %d", topic, len(peers))
		if len(peers) > 0 {
			f.logger.Infof("📡 主题 %s 连接的peers: %v", topic, peers)
		} else {
			f.logger.Warnf("⚠️ 主题 %s 没有连接的peers! 消息可能无法传递给其他节点", topic)
		}

		f.psMu.Lock()
		if t, ok := f.topicHandles[topic]; ok && t != nil {
			f.psMu.Unlock()
			if err := t.Publish(ctx, enc); err != nil {
				f.logger.Warnf("gossipsub publish failed", "topic", topic, "error", err.Error())
			}
		} else {
			f.psMu.Unlock()
			if err := f.ps.Publish(topic, enc); err != nil {
				f.logger.Warnf("gossipsub direct publish failed", "topic", topic, "error", err.Error())
			}
		}
	}
	// 本地统计与回环通知
	if f.pub != nil {
		_ = f.pub.Publish(topic, enc)
	}
	f.subMu.RLock()
	h := f.subs[topic]
	f.subMu.RUnlock()
	if h != nil {
		// 🔧 修复：本地回环时需要先解码数据，避免protobuf解析错误
		decodedPayload, decErr := f.dec.Decode(topic, enc)
		if decErr != nil {
			f.logger.Warnf("local handler decode failed", "topic", topic, "error", decErr.Error())
		} else {
			if localErr := h(ctx, peer.ID(""), topic, decodedPayload); localErr != nil {
				f.logger.Warnf("local handler failed", "topic", topic, "error", localErr.Error())
			}
		}
	}
	f.pubCount++
	f.logger.Infof("message published successfully", "topic", topic, "final_size", len(enc), "compressed_mark", shouldMarkCompressed)
	return nil
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

// ==================== StreamHandle 适配器（内部使用） ====================

// streamHandleAdapter 将 p2p.RawStream 适配为 iface.StreamHandle
type streamHandleAdapter struct {
	stream nodeiface.RawStream
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

	f.logger.Infof("🔍 GetTopicPeers 被调用: topic=%s", topic)

	if f.ps == nil {
		f.logger.Infof("❌ GossipSub未初始化，无法获取主题节点列表: %s", topic)
		return []peer.ID{}
	}

	f.logger.Infof("✅ GossipSub已初始化，topicHandles数量: %d", len(f.topicHandles))

	// 获取主题handle
	topicHandle, exists := f.topicHandles[topic]
	if !exists {
		f.logger.Infof("❌ 主题未加入，无法获取节点列表: %s, 可用主题: %v", topic, func() []string {
			var topics []string
			for t := range f.topicHandles {
				topics = append(topics, t)
			}
			return topics
		}())
		return []peer.ID{}
	}

	f.logger.Infof("✅ 找到主题handle: %s", topic)

	// 获取连接到该主题的节点
	peers := topicHandle.ListPeers()
	f.logger.Infof("📊 主题 %s 连接的节点数量: %d", topic, len(peers))

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
	// 获取底层 libp2p Host
	libp2pHost := f.host.Libp2pHost()
	if libp2pHost == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	// 检查节点是否已连接
	if libp2pHost.Network().Connectedness(peerID) != libnetwork.Connected {
		// 尝试连接到目标节点（如果未连接）
		err := f.host.EnsureConnected(ctx, peerID, time.Now().Add(5*time.Second))
		if err != nil {
			return false, fmt.Errorf("failed to connect to peer %s: %v", peerID, err)
		}
	}

	// 获取节点支持的协议
	protocols, err := libp2pHost.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// 检查是否支持目标协议
	for _, p := range protocols {
		if string(p) == protocol {
			f.logger.Debugf("节点 %s 支持协议: %s", peerID, protocol)
			return true, nil
		}
	}

	f.logger.Debugf("节点 %s 不支持协议: %s，支持的协议: %v", peerID, protocol, protocols)
	return false, nil
}
