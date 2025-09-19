package discovery

import (
	"context"
	"fmt"
	"os"
	"time"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	lprouting "github.com/libp2p/go-libp2p/core/routing"
	routdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
)

// dhtRuntime 封装 Kademlia DHT 的初始化与生命周期：
// - 依据配置选择协议前缀与模式（Auto/Client/Server）；
// - 可选挂载 Badger 持久化存储（满足 go-datastore 接口）；
// - 启动后执行 Bootstrap 并记录路由表规模。
type dhtRuntime struct {
	cfg        *nodeconfig.NodeOptions
	logger     logiface.Logger
	host       lphost.Host
	kdht       *dht.IpfsDHT
	store      storageiface.Provider
	ctx        context.Context
	routingDsc *routdisc.RoutingDiscovery
}

func newDHTRuntime(cfg *nodeconfig.NodeOptions, logger logiface.Logger, h lphost.Host) *dhtRuntime {
	return &dhtRuntime{cfg: cfg, logger: logger, host: h}
}

func (r *dhtRuntime) Start(ctx context.Context) error {
	if r.host == nil {
		return nil
	}

	// 使用长期context替代启动时的临时context
	// 这样DHT发现可以在应用启动完成后继续运行
	r.ctx = context.Background()

	// 开关：未启用则跳过
	if r.cfg != nil && !r.cfg.Discovery.DHT.Enabled {
		if r.logger != nil {
			r.logger.Infof("p2p.discovery.dht disabled by config")
		}
		return nil
	}

	// 读取配置
	protocolPrefix := protocol.ID("/weisyn")
	mode := dht.ModeAuto
	if r.cfg != nil {
		if r.cfg.Discovery.DHT.ProtocolPrefix != "" {
			protocolPrefix = protocol.ID(r.cfg.Discovery.DHT.ProtocolPrefix)
		}
		switch r.cfg.Discovery.DHT.Mode {
		case "client":
			mode = dht.ModeClient
		case "server":
			mode = dht.ModeServer
		default:
			mode = dht.ModeAuto
		}
	}

	// 基础 DHT 选项
	opts := []dht.Option{
		//	dht.ProtocolPrefix(protocolPrefix),
		dht.Mode(mode),
	}
	if r.logger != nil {
		r.logger.Infof("p2p.discovery.dht init protocol_prefix=%s mode=%v", protocolPrefix, mode)
	}

	// Datastore：优先使用持久化，否则回退内存（当前构建未链接持久化实现时回退）
	var datastore ds.Batching = dsync.MutexWrap(ds.NewMapDatastore())
	if r.cfg != nil && r.cfg.Discovery.DHT.DataStorePath != "" {
		if r.logger != nil {
			r.logger.Warnf("p2p.discovery.dht persistent_datastore_requested path=%s (fallback memory in this build)", r.cfg.Discovery.DHT.DataStorePath)
		}
	}
	opts = append(opts, dht.Datastore(datastore))

	// 高级选项
	if r.cfg != nil {
		if r.cfg.Discovery.DHT.EnableLANLoopback {
			opts = append(opts, dht.AddressFilter(nil))
		}
		if r.cfg.Discovery.DHT.EnableOptimisticProvide {
			opts = append(opts, dht.EnableOptimisticProvide())
			if r.cfg.Discovery.DHT.OptimisticProvideJobsPoolSize > 0 {
				opts = append(opts, dht.OptimisticProvideJobsPoolSize(r.cfg.Discovery.DHT.OptimisticProvideJobsPoolSize))
			}
		}
	}

	// 创建 DHT
	kdht, err := dht.New(r.ctx, r.host, opts...)
	if err != nil {
		// 离线路由回退：不阻断发现运行，保留其他能力（如 mDNS / Bootstrap 拨号）
		if r.logger != nil {
			r.logger.Warnf("p2p.discovery.dht init_failed, fallback to offline: %v", err)
		}
		return nil
	}
	r.kdht = kdht

	// 连接到bootstrap节点
	r.connectToBootstrapPeers()
	// 启动DHT
	if err := r.kdht.Bootstrap(r.ctx); err != nil {
		if r.logger != nil {
			r.logger.Warnf("p2p.discovery.dht bootstrap_failed error=%v", err)
		}
	} else if r.logger != nil {
		r.logger.Infof("p2p.discovery.dht bootstrap_ok")
	}
	if r.logger != nil && r.kdht != nil && r.kdht.RoutingTable() != nil {
		r.logger.Infof("p2p.discovery.dht rt_size=%d", r.kdht.RoutingTable().Size())
	}

	return nil
}

// connectToBootstrapPeers bootstrap连接
func (r *dhtRuntime) connectToBootstrapPeers() {
	if r.cfg == nil {
		return
	}

	for _, peerAddr := range r.cfg.Discovery.BootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			continue
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}

		// 模仿_net：异步连接bootstrap peers
		go func(info peer.AddrInfo) {
			connectCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			if err := r.host.Connect(connectCtx, info); err != nil {
				// CLI模式下使用日志而不是直接打印，减少控制台噪音
				if os.Getenv("WES_CLI_MODE") == "true" {
					if r.logger != nil {
						r.logger.Debugf("[Bootstrap] Failed to connect to %s: %v", info.ID, err)
					}
				} else {
					fmt.Printf("[Bootstrap] Failed to connect to %s: %v\n", info.ID, err)
				}
			} else {
				// CLI模式下使用日志而不是直接打印
				if os.Getenv("WES_CLI_MODE") == "true" {
					if r.logger != nil {
						r.logger.Debugf("[Bootstrap] ✅ Connected to bootstrap peer: %s", info.ID)
					}
				} else {
					fmt.Printf("[Bootstrap] ✅ Connected to bootstrap peer: %s\n", info.ID)
				}
			}
		}(*peerInfo)
	}
}

func (r *dhtRuntime) Stop(ctx context.Context) error {
	if r.kdht != nil {
		if r.logger != nil {
			r.logger.Infof("p2p.discovery.dht stopping")
		}
		_ = r.kdht.Close()
		r.kdht = nil
		if r.logger != nil {
			r.logger.Infof("p2p.discovery.dht stopped")
		}
	}
	return nil
}

// ContentRouting 返回 DHT 的内容路由实现
func (r *dhtRuntime) ContentRouting() lprouting.ContentRouting {
	if r.kdht == nil {
		return nil
	}
	return r.kdht
}

// GetRoutingTableSize 返回DHT路由表大小
func (r *dhtRuntime) GetRoutingTableSize() int {
	if r.kdht == nil || r.kdht.RoutingTable() == nil {
		return 0
	}
	return r.kdht.RoutingTable().Size()
}
