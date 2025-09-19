package node

// 本文件提供面向 Network 的最小 节点网络 服务适配：实现 pkg/interfaces/infrastructure/node.Host
// 说明：仅负责连通性保障、开流、入站流注册；不暴露生命周期与指标。

import (
	"context"
	"fmt"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	hostpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/host"
	"github.com/weisyn/v1/pkg/constants/protocols"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
)

// rawStreamAdapter 将 libp2p 的 network.Stream 适配为最小 RawStream
type rawStreamAdapter struct{ s libnetwork.Stream }

func (a *rawStreamAdapter) Read(p []byte) (int, error)    { return a.s.Read(p) }
func (a *rawStreamAdapter) Write(p []byte) (int, error)   { return a.s.Write(p) }
func (a *rawStreamAdapter) Close() error                  { return a.s.Close() }
func (a *rawStreamAdapter) CloseWrite() error             { return a.s.CloseWrite() }
func (a *rawStreamAdapter) Reset() error                  { return a.s.Reset() }
func (a *rawStreamAdapter) SetDeadline(t time.Time) error { return a.s.SetDeadline(t) }

// hostService 实现 node.Host 接口
type hostService struct {
	runtime         *hostpkg.Runtime
	logger          logiface.Logger                    // 添加logger字段
	pendingHandlers map[string]nodeiface.StreamHandler // 🔧 延迟注册的协议处理器
}

// newHostService 创建最小宿主机适配服务
func newHostService(runtime *hostpkg.Runtime) nodeiface.Host {
	return &hostService{
		runtime:         runtime,
		logger:          runtime.GetLogger(),                      // 从runtime获取logger
		pendingHandlers: make(map[string]nodeiface.StreamHandler), // 🔧 初始化延迟注册映射
	}
}

// EnsureConnected 确保与目标节点连通（幂等）
func (h *hostService) EnsureConnected(ctx context.Context, to libpeer.ID, deadline time.Time) error {
	if h.runtime == nil || h.runtime.Host() == nil {
		return nil
	}
	netw := h.runtime.Host().Network()
	if netw == nil {
		return nil
	}
	// 已连接则直接返回
	if netw.Connectedness(to) == libnetwork.Connected {
		return nil
	}
	// 尝试拨号（libp2p 网络层支持按 PeerID 拨号；地址由 peerstore/发现填充）
	_, err := netw.DialPeer(ctx, to)
	return err
}

// NewStream 打开出站流
func (h *hostService) NewStream(ctx context.Context, to libpeer.ID, protocolID string) (nodeiface.RawStream, error) {
	if h.runtime == nil || h.runtime.Host() == nil {
		return nil, libnetwork.ErrNoConn
	}
	stream, err := h.runtime.Host().NewStream(ctx, to, libprotocol.ID(protocolID))
	if err != nil {
		return nil, err
	}
	return &rawStreamAdapter{s: stream}, nil
}

// RegisterStreamHandler 注册入站协议处理器
func (h *hostService) RegisterStreamHandler(protocolID string, handler nodeiface.StreamHandler) {
	if h.runtime == nil || h.runtime.Host() == nil {
		// 🔧 延迟注册：将协议处理器保存起来，等Host启动后再注册
		h.pendingHandlers[protocolID] = handler
		return
	}
	// 使用logger而不是fmt.Printf
	if h.logger != nil {
		h.logger.Debugf("🔧 DEBUG: 在libp2p层注册协议: %s", protocolID)
	}

	// 注册协议处理器
	h.runtime.Host().SetStreamHandler(libprotocol.ID(protocolID), func(s libnetwork.Stream) {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Debugf("🔧 DEBUG: libp2p收到协议流: %s, 来自: %s", protocolID, s.Conn().RemotePeer())
		}
		// 使用无派生的上下文；上层可在 handler 内部再行管理超时/取消
		handler(context.Background(), s.Conn().RemotePeer(), &rawStreamAdapter{s: s})
	})

	// 验证协议是否真的注册成功
	protocols := h.runtime.Host().Mux().Protocols()
	found := false
	for _, p := range protocols {
		if string(p) == protocolID {
			found = true
			break
		}
	}
	if found {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Debugf("✅ 协议注册验证成功: %s", protocolID)
		}
	} else {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Warnf("❌ 协议注册验证失败: %s, 当前支持的协议: %v", protocolID, protocols)
		}
	}
}

// UnregisterStreamHandler 取消入站协议处理器
func (h *hostService) UnregisterStreamHandler(protocolID string) {
	if h.runtime == nil || h.runtime.Host() == nil {
		return
	}
	h.runtime.Host().RemoveStreamHandler(libprotocol.ID(protocolID))
}

// ID 返回本地 PeerID
func (h *hostService) ID() libpeer.ID {
	if h.runtime == nil || h.runtime.Host() == nil {
		return ""
	}
	return h.runtime.Host().ID()
}

// AnnounceAddrs 返回对外可达地址
func (h *hostService) AnnounceAddrs() []ma.Multiaddr {
	if h.runtime == nil || h.runtime.Host() == nil {
		return nil
	}
	return h.runtime.Host().Addrs()
}

// Libp2pHost 返回底层 libp2p Host
func (h *hostService) Libp2pHost() libhost.Host {
	if h.runtime == nil || h.runtime.Host() == nil {
		return nil
	}
	return h.runtime.Host()
}

// RegisterPendingHandlers 注册所有延迟的协议处理器
// 🔧 在P2P Host启动后调用此方法来注册之前无法注册的协议
func (h *hostService) RegisterPendingHandlers() {
	if h.runtime == nil || h.runtime.Host() == nil {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Debugf("🔧 DEBUG: Host仍未初始化，无法注册延迟协议")
		}
		return
	}

	if len(h.pendingHandlers) == 0 {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Debugf("🔧 DEBUG: 没有延迟的协议需要注册")
		}
		return
	}

	// 使用logger而不是fmt.Printf
	if h.logger != nil {
		h.logger.Infof("🔧 DEBUG: 开始注册 %d 个延迟的协议处理器", len(h.pendingHandlers))
	}

	for protocolID, handler := range h.pendingHandlers {
		// 使用logger而不是fmt.Printf
		if h.logger != nil {
			h.logger.Infof("🔧 DEBUG: 注册延迟协议: %s", protocolID)
		}

		// 注册协议处理器
		h.runtime.Host().SetStreamHandler(libprotocol.ID(protocolID), func(s libnetwork.Stream) {
			// 使用logger而不是fmt.Printf
			if h.logger != nil {
				h.logger.Debugf("🔧 DEBUG: libp2p收到延迟注册协议流: %s, 来自: %s", protocolID, s.Conn().RemotePeer())
			}
			handler(context.Background(), s.Conn().RemotePeer(), &rawStreamAdapter{s: s})
		})

		// 验证协议是否注册成功
		protocols := h.runtime.Host().Mux().Protocols()
		found := false
		for _, p := range protocols {
			if string(p) == protocolID {
				found = true
				break
			}
		}

		if found {
			// 使用logger而不是fmt.Printf
			if h.logger != nil {
				h.logger.Infof("✅ 延迟协议注册成功: %s", protocolID)
			}
		} else {
			// 使用logger而不是fmt.Printf
			if h.logger != nil {
				h.logger.Warnf("❌ 延迟协议注册失败: %s", protocolID)
			}
		}
	}

	// 清空延迟注册列表
	h.pendingHandlers = make(map[string]nodeiface.StreamHandler)
	// 使用logger而不是fmt.Printf
	if h.logger != nil {
		h.logger.Infof("🔧 DEBUG: 延迟协议注册完成，已清空延迟列表")
	}
}

// ValidateWESPeer 验证节点是否为WES业务节点
func (h *hostService) ValidateWESPeer(ctx context.Context, peerID libpeer.ID) (bool, error) {
	// 获取底层 libp2p Host
	if h.runtime == nil || h.runtime.Host() == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	libp2pHost := h.runtime.Host()

	// 检查节点是否已连接
	if libp2pHost.Network().Connectedness(peerID) != libnetwork.Connected {
		// 如果未连接，快速返回false，避免触发连接（保持轻量级）
		// 这是合理的，因为K桶通常处理的是已连接的节点
		return false, nil
	}

	// 获取节点支持的协议
	peerProtocols, err := libp2pHost.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// 检查是否支持WES核心协议
	for _, p := range peerProtocols {
		if string(p) == protocols.ProtocolBlockSubmission {
			return true, nil
		}
	}

	// 不支持WES核心协议，认为是外部节点
	return false, nil
}
