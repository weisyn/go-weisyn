package kbucket

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// ============================================================================
//                              输入输出定义
// ============================================================================

// ModuleInput 定义K桶模块的输入依赖
type ModuleInput struct {
	fx.In

	Config         kademlia.KBucketConfig `name:"kbucket_config"`
	Logger         log.Logger             // 日志记录器（必需）
	P2PService     p2pi.Service           `name:"p2p_service" optional:"true"` // P2P 服务，用于WES节点验证和连接状态检查
	ConfigProvider config.Provider        `optional:"true"`                    // 配置提供者，用于获取本地链身份
}

// ModuleOutput 定义K桶模块的输出
type ModuleOutput struct {
	fx.Out

	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager"`
	DistanceCalculator  kademlia.DistanceCalculator  `name:"distance_calculator"`
	PeerSelector        kademlia.PeerSelector        `name:"peer_selector"`
}

// ============================================================================
//                              主模块定义
// ============================================================================

// Module K桶模块
// 采用fx依赖注入模式，提供完整的Kademlia路由表功能
func Module() fx.Option {
	return fx.Module("kbucket",
		// === 配置提供 ===
		fx.Provide(fx.Annotate(
			ProvideKBucketConfig,
			fx.ResultTags(`name:"kbucket_config"`),
		)),

		// === 核心组件提供 ===
		fx.Provide(
			func(in ModuleInput) ModuleOutput {
				// 🎯 为 Kademlia 模块添加 module 字段，日志将路由到 node-system.log
				var kademliaLogger log.Logger
				if in.Logger != nil {
					kademliaLogger = in.Logger.With("module", "kademlia")
				}

				// 创建核心组件
				routingTableManager := NewRoutingTableManager(in.Config, kademliaLogger, in.P2PService, in.ConfigProvider)
				distanceCalculator := NewXORDistanceCalculator(kademliaLogger)
				peerSelector := NewKademliaPeerSelector(kademliaLogger)

				return ModuleOutput{
					RoutingTableManager: routingTableManager,
					DistanceCalculator:  distanceCalculator,
					PeerSelector:        peerSelector,
				}
			},
		),

		// === 生命周期管理 ===
		fx.Invoke(RegisterKBucketLifecycle),
	)
}

// LifecycleInput 定义生命周期管理的输入依赖
type LifecycleInput struct {
	fx.In

	Lifecycle           fx.Lifecycle
	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager"`
	Logger              log.Logger
	EventBus            event.EventBus  `optional:"true"`                    // 事件总线，用于订阅peer连接事件
	P2PService          p2pi.Service    `name:"p2p_service" optional:"true"` // P2P 服务，用于获取已连接peers进行全量导入
	ConfigProvider      config.Provider `optional:"true"`                    // 配置提供者，用于读取 sync.advanced 的入桶保障配置
}

// RegisterKBucketLifecycle 注册K桶生命周期管理
func RegisterKBucketLifecycle(in LifecycleInput) {
	// 🎯 为 Kademlia 模块添加 module 字段
	var kademliaLogger log.Logger
	if in.Logger != nil {
		kademliaLogger = in.Logger.With("module", "kademlia")
	}

	// 日志瘦身：对 “peer未加入K桶” 进行按 peer 去重，避免连接抖动/重复事件刷屏
	// - external peers（kubo/p2pd/...）默认 Debug 且更短窗口
	// - weisyn peers 才使用 Warn，窗口更长
	var (
		rejectMu   sync.Mutex
		rejectLast = map[string]time.Time{}
	)

	in.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if kademliaLogger != nil {
				kademliaLogger.Info("🗂️  正在启动K桶路由表管理器...")
			}

			// 读取可配置的入桶保障参数（默认值与代码内置保持一致）
			reconcileInterval := 30 * time.Second
			retryBackoffs := []time.Duration{200 * time.Millisecond, 1 * time.Second, 3 * time.Second, 8 * time.Second, 15 * time.Second}
			if in.ConfigProvider != nil {
				if bc := in.ConfigProvider.GetBlockchain(); bc != nil {
					if bc.Sync.Advanced.KBucketReconcileIntervalSeconds > 0 {
						reconcileInterval = time.Duration(bc.Sync.Advanced.KBucketReconcileIntervalSeconds) * time.Second
					}
					if len(bc.Sync.Advanced.KBucketPeerAddRetryBackoffsMs) > 0 {
						var tmp []time.Duration
						for _, ms := range bc.Sync.Advanced.KBucketPeerAddRetryBackoffsMs {
							if ms <= 0 {
								continue
							}
							tmp = append(tmp, time.Duration(ms)*time.Millisecond)
						}
						if len(tmp) > 0 {
							retryBackoffs = tmp
						}
					}
				}
			}

			// 使用类型断言调用具体实现的Start方法
			if manager, ok := in.RoutingTableManager.(*RoutingTableManager); ok {
				// 🔧 Phase 3: 注入事件总线（用于发布Discovery间隔重置事件）
				if in.EventBus != nil {
					manager.SetEventBus(in.EventBus)
					if kademliaLogger != nil {
						kademliaLogger.Debug("K桶已注入事件总线，可发布Discovery重置事件")
					}
				}

				if err := manager.Start(ctx); err != nil {
					if kademliaLogger != nil {
						kademliaLogger.Errorf("启动K桶路由表管理器失败: %v", err)
					}
					return err
				}

				// 全量导入已连接的peers到K桶（避免订阅时序问题）
				if in.P2PService != nil {
					// 获取底层libp2p host
					libp2pHost := in.P2PService.Host()
					if libp2pHost != nil {
						connectedPeers := libp2pHost.Network().Peers()
						if kademliaLogger != nil {
							kademliaLogger.Infof("🔒 开始全量导入已连接peers到K桶（含WES过滤）: 共%d个peer", len(connectedPeers))
						}

						importedCount := 0
						rejectedCount := 0
						for _, peerID := range connectedPeers {
							// 跳过自己
							if peerID == libp2pHost.ID() {
								continue
							}

							addrInfo := peer.AddrInfo{ID: peerID}
							// 调用AddPeer，内部会进行WES节点验证
							if added, err := manager.AddPeer(ctx, addrInfo); err != nil {
								if kademliaLogger != nil {
									kademliaLogger.Warnf("全量导入peer失败: %s, 错误: %v", peerID, err)
								}
							} else if added {
								importedCount++
								if kademliaLogger != nil {
									kademliaLogger.Debugf("WES节点导入成功: %s", peerID)
								}
							} else {
								rejectedCount++
								// AddPeer返回false通常表示外部节点被过滤
							}
						}
						if kademliaLogger != nil {
							total := len(connectedPeers) - 1
							if total < 0 {
								total = 0
							}
							kademliaLogger.Infof("🔒 全量导入完成: WES节点=%d, 外部节点已过滤=%d, 总计=%d",
								importedCount, rejectedCount, total)
							// 如果已连接 peers 存在，但一个都没能入桶，给出一次明确、可操作的告警：
							// - 常见原因：bootstrap peers 指向了 IPFS/kubo 等外部网络；或链身份(namespace/chain_id/genesis)不匹配；
							// - 结果：K桶长期为空 -> sync/选举只能 fallback 或 no-op。
							if total > 0 && importedCount == 0 {
								kademliaLogger.Warnf("⚠️ K桶为空风险：当前已连接 peers=%d，但 WES 节点导入=0（外部/链不匹配已过滤=%d）。请检查 bootstrap_peers / rendezvous_ns / network_namespace / chain_id / genesis 是否指向同一条 WES 网络。",
									total, rejectedCount)
							}
						}
					} else {
						if kademliaLogger != nil {
							kademliaLogger.Warn("🗂️  无法获取libp2p Host，跳过全量导入已连接peers")
						}
					}
				} else {
					if kademliaLogger != nil {
						kademliaLogger.Warn("🗂️  P2PService为nil，跳过全量导入已连接peers")
					}
				}

				// 发布一次摘要（便于 diagnostics 立即看到当前K桶状态）
				if in.EventBus != nil {
					in.EventBus.Publish(event.EventTypeKBucketSummaryUpdated, context.Background(), manager.GetDiagnosticsSummary())
				}

				// 订阅peer连接事件，自动添加到路由表
				if in.EventBus != nil {
					peerConnectedHandler := func(ctx context.Context, data interface{}) error {
						if peerID, ok := data.(peer.ID); ok {
							if kademliaLogger != nil {
								// ⚠️ 日志瘦身：该事件在主网/公网会非常频繁，INFO 会造成日志臃肿
								kademliaLogger.Debugf("[kbucket] 🌐 peer.connected -> try_add: %s", peerID)
							}

							// 🔧 连接成功时更新LastUsefulAt（表示peer仍活跃）
							manager.RecordPeerSuccess(peerID)

							// 创建AddrInfo（地址留空，因为我们主要关心路由表的peer ID）
							addrInfo := peer.AddrInfo{ID: peerID}

							// ⚠️ 生产级时序处理：
							// libp2p 的 Identify/协议列表写入 peerstore 可能滞后于 “connected” 事件，
							// 如果立刻校验协议能力（ProtocolBlockSubmission），可能出现“协议列表为空 → 误判外部节点”的竞态。
							//
							// 这里用“更长窗口”的延迟 + 重试，等待 peerstore 填充协议信息后再入 K 桶。
							// 目标：从根本上避免 K桶为空（业务节点不入桶）的致命缺陷。
							go func(pid peer.ID) {
								for i, d := range retryBackoffs {
									time.Sleep(d)

									added, err := manager.AddPeer(context.Background(), addrInfo)
									if err != nil {
										if kademliaLogger != nil {
											kademliaLogger.Debugf("延迟入表尝试失败: peer=%s attempt=%d/%d err=%v", pid, i+1, len(retryBackoffs), err)
										}
										continue
									}
									if added {
										if kademliaLogger != nil {
											kademliaLogger.Infof("✅ 已将peer加入K桶路由表: %s", pid)
										}

										// ✅ 修复缺陷M：保护WES业务节点连接，防止被连接管理器淘汰
										// 场景：当连接数超过 HighWater 时，连接管理器会淘汰未保护的连接
										// - bootstrap节点已被保护（runtime.go:78）
										// - WES业务节点也需要保护，否则会因为连接到大量libp2p公共节点而被淘汰
										// - 保护标签 "kbucket" 表明这是K桶核心节点，应优先保留
										if manager.p2pService != nil && manager.p2pService.Host() != nil {
											if cm := manager.p2pService.Host().ConnManager(); cm != nil {
												cm.Protect(pid, "kbucket")
												if kademliaLogger != nil {
													kademliaLogger.Debugf("🔒 已保护K桶peer连接: %s (tag=kbucket)", pid)
												}
											}
										}

										if in.EventBus != nil {
											in.EventBus.Publish(event.EventTypeKBucketSummaryUpdated, context.Background(), manager.GetDiagnosticsSummary())
										}
										return
									}
								}

								// 仍未加入：输出一次“可解释”的诊断信息，便于从日志直接定位原因
								// （重要：按 peer 做时间窗口去重，避免刷屏）
								if kademliaLogger != nil {
									var (
										connected        bool
										connectednessStr string
										protoCount       int
										hasWESProto      bool
										agentStr         string
										protoList        string
										wesOK            bool
										wesErr           error
										chainOK          bool
										chainReason      string
										chainErr         error
										peerstoreAddrs   int
									)

									// 连接状态
									if manager.p2pService != nil && manager.p2pService.Host() != nil {
										h := manager.p2pService.Host()
										connectedness := h.Network().Connectedness(pid)
										connected = connectedness.String() == "Connected"
										connectednessStr = connectedness.String()

										// 协议列表
										if ps, err := h.Peerstore().GetProtocols(pid); err == nil {
											protoCount = len(ps)
											// 快速判定是否包含 weisyn 协议族（仅用于日志可读性，不参与决策）
											for _, p := range ps {
												if strings.HasPrefix(string(p), "/weisyn/") {
													hasWESProto = true
													break
												}
											}
											// 记录协议列表（截断），便于从日志直接确认是否是“命名空间化协议不匹配”
											// 例如：/weisyn/public-testnet-demo/consensus/block_submission/1.0.0
											var list []string
											for i, p := range ps {
												if i >= 20 {
													break
												}
												list = append(list, string(p))
											}
											if len(list) > 0 {
												protoList = strings.Join(list, ",")
											}
										}

										// UserAgent（AgentVersion）
										if av, err := h.Peerstore().Get(pid, "AgentVersion"); err == nil {
											if s, ok := av.(string); ok {
												agentStr = s
											}
										}

										// 获取peerstore中的地址数量（用于诊断地址发布问题）
										if addrs := h.Peerstore().Addrs(pid); addrs != nil {
											peerstoreAddrs = len(addrs)
										}
									}

									// WES/链身份策略（同包内可调用未导出方法，直接给"原因"）
									wesOK, wesErr = manager.validateWESPeer(context.Background(), pid)
									chainOK, chainReason, chainErr = manager.validatePeerChainIdentity(context.Background(), pid)

									// 简单截断，避免极长 UserAgent 刷屏
									if len(agentStr) > 200 {
										agentStr = agentStr[:200] + "..."
									}
									if len(protoList) > 800 {
										protoList = protoList[:800] + "..."
									}

									// 识别是否是 weisyn 节点（只有这类才值得 Warn）
									agentLower := strings.ToLower(agentStr)
									isWeisyn := strings.Contains(agentLower, "weisyn")
									isExternalKnown := strings.HasPrefix(agentLower, "kubo/") ||
										strings.HasPrefix(agentLower, "go-ipfs/") ||
										strings.HasPrefix(agentLower, "p2pd/") ||
										strings.Contains(agentLower, "bootstrap.libp2p.io") ||
										strings.Contains(agentLower, "ipfs")

									// ✅ 诊断增强：若该 peer 属于“配置的 WES bootstrap peers”（非公网 bootstrap），即使 agent 未包含 weisyn 也要提升到 warn。
									// 目的：快速看清“业务关键节点为何未入桶”（协议未就绪/链身份不匹配/地址缺失）。
									isConfiguredWESBootstrap := false
									if in.ConfigProvider != nil {
										if nc := in.ConfigProvider.GetNode(); nc != nil {
											for _, s := range nc.Discovery.BootstrapPeers {
												ls := strings.ToLower(s)
												if strings.Contains(ls, "bootstrap.libp2p.io") || strings.Contains(ls, "ipfs") {
													continue
												}
												parts := strings.Split(s, "/p2p/")
												if len(parts) == 2 && parts[1] == pid.String() {
													isConfiguredWESBootstrap = true
													break
												}
											}
										}
									}

									// 按 peer + 级别做去重
									now := time.Now()
									peerKey := pid.String()
									ttl := 2 * time.Minute
									if isWeisyn {
										ttl = 10 * time.Minute
									} else if isExternalKnown {
										ttl = 1 * time.Minute
									}
									rejectMu.Lock()
									last, ok := rejectLast[peerKey]
									if ok && now.Sub(last) < ttl {
										rejectMu.Unlock()
										return
									}
									rejectLast[peerKey] = now
									rejectMu.Unlock()

									args := []interface{}{pid, connectednessStr, connected, peerstoreAddrs, wesOK, hasWESProto, protoCount, chainOK, chainReason, agentStr, protoList, wesErr, chainErr}

									// 日志瘦身：
									// 1. 外部已知节点（kubo/go-ipfs/p2pd/IPFS公网）被拒绝是**预期行为**，Debug 级别 + 🚫 表情
									// 2. weisyn 节点异常 或 配置的 WES bootstrap 未入桶，必须 Warn + ❌ 表情（避免业务节点问题无从定位）
									// 3. 未知外部节点（无 agent 或未识别），Info 级别 + 🚫 表情
									if isWeisyn || isConfiguredWESBootstrap {
										// 业务节点未入桶 → 需要关注
										kademliaLogger.Warnf("[kbucket] ❌ peer未加入K桶: peer=%s connectedness=%s connected=%v peerstore_addrs=%d wes_ok=%v has_wes_proto=%v proto_count=%d chain_ok=%v chain_reason=%s agent=%q proto_list=%q wes_err=%v chain_err=%v", args...)
									} else if isExternalKnown {
										// 已知外部节点（kubo/IPFS）被拒绝 → 预期行为，Debug
										kademliaLogger.Debugf("[kbucket] 🚫 外部节点被过滤（预期行为）: peer=%s agent=%q chain_reason=%s", pid, agentStr, chainReason)
									} else {
										// 未知外部节点 → Info（首次可见，后续去重）
										kademliaLogger.Infof("[kbucket] 🚫 非WES节点被过滤: peer=%s connectedness=%s wes_ok=%v chain_ok=%v chain_reason=%s agent=%q", pid, connectednessStr, wesOK, chainOK, chainReason, agentStr)
									}
								}
							}(peerID)
						}
						return nil
					}

					// 订阅peer断连事件，标记为Suspect（温和处理，不立即删除）
					//
					// ⚠️ 注意：
					// - 该事件在主网上会非常频繁，如果使用 Info 级别会产生大量无价值噪音日志。
					// - 这里只保留 Debug 级别的明细日志，将关键状态变更交由 manager 内部度量。
					peerDisconnectedHandler := func(ctx context.Context, data interface{}) error {
						if peerID, ok := data.(peer.ID); ok {
							if kademliaLogger != nil {
								// 使用 Debug 级别，避免在生产环境刷屏
								kademliaLogger.Debugf("收到peer断连事件: %s，标记为Suspect", peerID)
							}

							// 记录失败（触发状态转换为Suspect或Quarantined）
							manager.RecordPeerFailure(peerID)

							if kademliaLogger != nil {
								kademliaLogger.Debugf("节点断连后状态已更新: %s", peerID)
							}
						}
						return nil
					}

					// 订阅peer连接事件
					if err := in.EventBus.Subscribe(event.EventTypeNetworkPeerConnected, peerConnectedHandler); err != nil {
						if kademliaLogger != nil {
							kademliaLogger.Warnf("订阅peer连接事件失败: %v", err)
						}
					} else {
						if kademliaLogger != nil {
							kademliaLogger.Info("🗂️  已订阅peer连接事件，将自动维护路由表")
						}
					}

					// 订阅peer断连事件
					if err := in.EventBus.Subscribe(event.EventTypeNetworkPeerDisconnected, peerDisconnectedHandler); err != nil {
						if kademliaLogger != nil {
							kademliaLogger.Warnf("订阅peer断连事件失败: %v", err)
						}
					} else {
						if kademliaLogger != nil {
							kademliaLogger.Info("🗂️  已订阅peer断连事件，将自动标记为Suspect（温和处理）")
						}
					}
				}
			}

			// 周期性 reconcile：把“当前已连接 peers”持续导入 K桶，防止抖动/时序导致长期空桶
			// - connected 事件可能因为组件启动顺序错过
			// - Identify/peerstore 协议写入可能比连接事件更晚
			// - 节点短暂断连/重连后，需要再次尝试入桶
			if in.P2PService != nil {
				if manager, ok := in.RoutingTableManager.(*RoutingTableManager); ok {
					go func() {
						ticker := time.NewTicker(reconcileInterval)
						defer ticker.Stop()
						for {
							select {
							case <-manager.ctx.Done():
								return
							case <-ticker.C:
								h := in.P2PService.Host()
								if h == nil {
									continue
								}
								peers := h.Network().Peers()
								if len(peers) == 0 {
									continue
								}
								for _, pid := range peers {
									if pid == h.ID() {
										continue
									}
									_, _ = manager.AddPeer(context.Background(), peer.AddrInfo{ID: pid})
								}
								if in.EventBus != nil {
									in.EventBus.Publish(event.EventTypeKBucketSummaryUpdated, context.Background(), manager.GetDiagnosticsSummary())
								}
							}
						}
					}()
					if kademliaLogger != nil {
						kademliaLogger.Infof("🧭 已启动K桶周期性reconcile：持续导入已连接peers，防止空桶 (interval=%s retry_backoffs=%d)",
							reconcileInterval.String(), len(retryBackoffs))
					}
				}
			}

			if kademliaLogger != nil {
				kademliaLogger.Info("🗂️  K桶路由表管理器已启动")
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if kademliaLogger != nil {
				kademliaLogger.Info("🗂️  正在关闭K桶路由表管理器...")
			}

			// 使用类型断言调用具体实现的Stop方法
			if manager, ok := in.RoutingTableManager.(*RoutingTableManager); ok {
				if err := manager.Stop(ctx); err != nil {
					if kademliaLogger != nil {
						kademliaLogger.Errorf("关闭K桶路由表管理器失败: %v", err)
					}
					return err
				}
			}

			if kademliaLogger != nil {
				kademliaLogger.Info("🗂️  K桶路由表管理器已关闭")
			}
			return nil
		},
	})
}
