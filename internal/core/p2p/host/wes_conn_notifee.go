// Package host provides WES-aware connection management for libp2p.
package host

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	lphost "github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// WESConnNotifee 实现 network.Notifiee 接口，为非 WES 节点设置负权重
//
// 背景：
// - 阿里云公网节点 Goroutine 峰值 34,832（本地的 19 倍）
// - 核心原因：大量非 WES 的 libp2p 节点（IPFS/kubo 等）涌入，占用连接槽位
// - 解决方案：对非 WES 节点设置负权重，使其更容易被 ConnManager 淘汰
//
// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
type WESConnNotifee struct {
	host   lphost.Host
	cm     connmgr.ConnManager
	logger logiface.Logger

	// 非 WES 节点断开配置
	nonWESTimeout time.Duration // 非 WES 节点在多少秒后断开（默认 60s）

	// 已验证的 peer 缓存（避免重复验证）
	validatedPeers sync.Map // map[peer.ID]bool
}

// WESConnNotifeeConfig WES 连接通知器配置
type WESConnNotifeeConfig struct {
	// NonWESTimeout 非 WES 节点连接超时时间（超时后断开）
	// 设置为 0 表示不断开，只设置负权重
	NonWESTimeout time.Duration
}

// DefaultWESConnNotifeeConfig 返回默认配置
func DefaultWESConnNotifeeConfig() WESConnNotifeeConfig {
	return WESConnNotifeeConfig{
		NonWESTimeout: 60 * time.Second, // 非 WES 节点 60 秒后断开
	}
}

// NewWESConnNotifee 创建 WES 连接通知器
func NewWESConnNotifee(host lphost.Host, logger logiface.Logger, cfg WESConnNotifeeConfig) *WESConnNotifee {
	if host == nil {
		return nil
	}
	return &WESConnNotifee{
		host:          host,
		cm:            host.ConnManager(),
		logger:        logger,
		nonWESTimeout: cfg.NonWESTimeout,
	}
}

// Tag 权重常量
const (
	// WESBusinessPeerTag WES 业务节点标签（高优先级保护）
	WESBusinessPeerTag = "wes-business"
	// WESBusinessPeerWeight WES 业务节点权重（正值，不易被淘汰）
	WESBusinessPeerWeight = 20

	// NonWESPeerTag 非 WES 节点标签
	NonWESPeerTag = "non-wes"
	// NonWESPeerWeight 非 WES 节点权重（负值，容易被淘汰）
	NonWESPeerWeight = -10

	// InboundNonWESPeerTag 入站非 WES 节点标签（更低权重）
	InboundNonWESPeerTag = "inbound-non-wes"
	// InboundNonWESPeerWeight 入站非 WES 节点权重（更低，更容易被淘汰）
	InboundNonWESPeerWeight = -20
)

// Listen 监听地址变化（不处理）
func (n *WESConnNotifee) Listen(_ network.Network, _ ma.Multiaddr) {}

// ListenClose 监听地址关闭（不处理）
func (n *WESConnNotifee) ListenClose(_ network.Network, _ ma.Multiaddr) {}

// Connected 处理节点连接事件
//
// 策略：
// - 入站连接的非 WES 节点：设置更低的权重（-20），更容易被淘汰
// - 出站连接的非 WES 节点：设置负权重（-10）
// - WES 业务节点：设置正权重（+20），保护连接
func (n *WESConnNotifee) Connected(_ network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	direction := conn.Stat().Direction

	// 异步验证，避免阻塞连接流程
	go n.validateAndTagPeer(peerID, direction)
}

// validateAndTagPeer 验证并标记 peer
func (n *WESConnNotifee) validateAndTagPeer(peerID peer.ID, direction network.Direction) {
	// 检查缓存
	if _, ok := n.validatedPeers.Load(peerID); ok {
		return // 已验证过
	}

	// 等待 Identify 完成（最多 10 秒）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 简单等待协议信息可用
	var isWES bool
	for i := 0; i < 20; i++ { // 20 次，每次 500ms
		select {
		case <-ctx.Done():
			break
		case <-time.After(500 * time.Millisecond):
		}

		isWES = n.isWESPeer(peerID)
		if isWES {
			break
		}

		// 检查是否有协议信息（说明 Identify 完成）
		protos, err := n.host.Peerstore().GetProtocols(peerID)
		if err == nil && len(protos) > 0 {
			break // Identify 完成，可以做判断了
		}
	}

	// 缓存结果
	n.validatedPeers.Store(peerID, isWES)

	// 设置连接权重
	if n.cm == nil {
		return
	}

	if isWES {
		// WES 业务节点：设置正权重，保护连接
		n.cm.TagPeer(peerID, WESBusinessPeerTag, WESBusinessPeerWeight)
		if n.logger != nil {
			n.logger.Debugf("✅ WES 业务节点已保护: %s (direction=%s)", peerID.String()[:12], direction)
		}
	} else {
		// 非 WES 节点：设置负权重
		if direction == network.DirInbound {
			// 入站连接的非 WES 节点：更低权重
			n.cm.TagPeer(peerID, InboundNonWESPeerTag, InboundNonWESPeerWeight)
			if n.logger != nil {
				n.logger.Debugf("⚠️ 入站非 WES 节点已标记: %s (weight=%d)", peerID.String()[:12], InboundNonWESPeerWeight)
			}

			// 如果配置了超时断开
			if n.nonWESTimeout > 0 {
				go n.scheduleDisconnect(peerID, n.nonWESTimeout)
			}
		} else {
			// 出站连接的非 WES 节点：正常负权重
			n.cm.TagPeer(peerID, NonWESPeerTag, NonWESPeerWeight)
			if n.logger != nil {
				n.logger.Debugf("⚠️ 出站非 WES 节点已标记: %s (weight=%d)", peerID.String()[:12], NonWESPeerWeight)
			}
		}
	}
}

// isWESPeer 检查 peer 是否是 WES 业务节点
//
// 判断标准：协议列表中包含 "/weisyn/" 前缀的协议
func (n *WESConnNotifee) isWESPeer(peerID peer.ID) bool {
	if n.host == nil {
		return false
	}

	protos, err := n.host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false
	}

	for _, p := range protos {
		if strings.Contains(string(p), "/weisyn/") {
			return true
		}
	}
	return false
}

// scheduleDisconnect 计划断开非 WES 节点连接
func (n *WESConnNotifee) scheduleDisconnect(peerID peer.ID, timeout time.Duration) {
	time.Sleep(timeout)

	// 再次检查是否仍然连接且仍然是非 WES 节点
	if n.host == nil {
		return
	}

	// 检查连接状态
	if n.host.Network().Connectedness(peerID) != network.Connected {
		return // 已断开
	}

	// 再次验证是否是 WES 节点（可能在等待期间变成了 WES 节点）
	if n.isWESPeer(peerID) {
		// 已变为 WES 节点，更新标签
		if n.cm != nil {
			n.cm.UntagPeer(peerID, InboundNonWESPeerTag)
			n.cm.UntagPeer(peerID, NonWESPeerTag)
			n.cm.TagPeer(peerID, WESBusinessPeerTag, WESBusinessPeerWeight)
		}
		n.validatedPeers.Store(peerID, true)
		return
	}

	// 断开连接
	if err := n.host.Network().ClosePeer(peerID); err != nil {
		if n.logger != nil {
			n.logger.Debugf("断开非 WES 节点失败: %s, err=%v", peerID.String()[:12], err)
		}
	} else {
		if n.logger != nil {
			n.logger.Infof("🔌 已断开入站非 WES 节点: %s (timeout=%s)", peerID.String()[:12], timeout)
		}
	}
}

// Disconnected 处理节点断连事件
func (n *WESConnNotifee) Disconnected(_ network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	// 清理缓存
	n.validatedPeers.Delete(peerID)
}

// OpenedStream 处理流打开事件（不处理）
func (n *WESConnNotifee) OpenedStream(_ network.Network, _ network.Stream) {}

// ClosedStream 处理流关闭事件（不处理）
func (n *WESConnNotifee) ClosedStream(_ network.Network, _ network.Stream) {}

// RegisterWESConnNotifee 注册 WES 连接通知器到 libp2p host
//
// 应在 host 启动后调用
func RegisterWESConnNotifee(host lphost.Host, logger logiface.Logger, cfg WESConnNotifeeConfig) *WESConnNotifee {
	if host == nil {
		if logger != nil {
			logger.Warn("无法注册 WES 连接通知器：host 为 nil")
		}
		return nil
	}

	notifee := NewWESConnNotifee(host, logger, cfg)
	if notifee == nil {
		return nil
	}

	host.Network().Notify(notifee)

	if logger != nil {
		logger.Info("✅ 已注册 WES 连接通知器（非 WES 节点将被降权/断开）")
	}

	return notifee
}

