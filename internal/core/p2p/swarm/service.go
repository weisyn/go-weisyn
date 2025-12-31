package swarm

import (
	"context"
	"fmt"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// Service Swarm 服务实现
//
// 对标 Kubo Swarm：管理所有连接、流、带宽统计
type Service struct {
	host       lphost.Host
	bwReporter metrics.Reporter
}

var _ p2pi.Swarm = (*Service)(nil)

// NewService 创建 Swarm 服务
//
// 通过 BandwidthProvider 接口获取带宽计数器，避免直接依赖 host 包
func NewService(host lphost.Host, bwProvider interfaces.BandwidthProvider) *Service {
	var reporter metrics.Reporter
	if bwProvider != nil {
		reporter = bwProvider.BandwidthReporter()
	}
	return &Service{
		host:       host,
		bwReporter: reporter,
	}
}

// Peers 返回当前连接的 Peer 列表
func (s *Service) Peers() []libpeer.AddrInfo {
	if s.host == nil {
		return nil
	}

	peers := make([]libpeer.AddrInfo, 0)
	for _, conn := range s.host.Network().Conns() {
		peerID := conn.RemotePeer()
		addrs := conn.RemoteMultiaddr()
		peers = append(peers, libpeer.AddrInfo{
			ID:    peerID,
			Addrs: []ma.Multiaddr{addrs},
		})
	}
	return peers
}

// Connections 返回当前连接信息
func (s *Service) Connections() []p2pi.ConnInfo {
	if s.host == nil {
		return nil
	}

	conns := make([]p2pi.ConnInfo, 0)
	for _, conn := range s.host.Network().Conns() {
		direction := "outbound"
		if conn.Stat().Direction == libnetwork.DirInbound {
			direction = "inbound"
		}

		// 获取流数量
		streams := conn.GetStreams()
		streamCount := len(streams)

		conns = append(conns, p2pi.ConnInfo{
			Peer:        conn.RemotePeer(),
			Direction:   direction,
			RemoteAddr:  conn.RemoteMultiaddr().String(),
			LocalAddr:   conn.LocalMultiaddr().String(),
			OpenedAt:    conn.Stat().Opened.Unix(),
			StreamCount: streamCount,
		})
	}
	return conns
}

// Stats 返回 Swarm 统计信息
func (s *Service) Stats() p2pi.SwarmStats {
	if s.host == nil {
		return p2pi.SwarmStats{}
	}

	network := s.host.Network()
	allConns := network.Conns()

	// 统计连接方向
	inboundConns := 0
	outboundConns := 0
	totalStreams := 0

	for _, conn := range allConns {
		if conn.Stat().Direction == libnetwork.DirInbound {
			inboundConns++
		} else {
			outboundConns++
		}
		totalStreams += len(conn.GetStreams())
	}

	stats := p2pi.SwarmStats{
		NumPeers:      len(network.Peers()),
		NumConns:      len(allConns),
		NumStreams:    totalStreams,
		InboundConns:  inboundConns,
		OutboundConns: outboundConns,
	}

	// 从带宽计数器获取统计信息
	if s.bwReporter != nil {
		if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
			totals := bwCounter.GetBandwidthTotals()
			stats.InboundRateBps = float64(totals.RateIn)
			stats.OutboundRateBps = float64(totals.RateOut)
			stats.InboundTotal = totals.TotalIn
			stats.OutboundTotal = totals.TotalOut
		}
	}

	return stats
}

// Dial 连接到指定 Peer
func (s *Service) Dial(ctx context.Context, info libpeer.AddrInfo) error {
	if s.host == nil {
		return fmt.Errorf("host not available")
	}

	// 如果已经连接，直接返回
	if s.host.Network().Connectedness(info.ID) == libnetwork.Connected {
		return nil
	}

	// 添加到 peerstore
	s.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	// 使用 Host.Connect 连接（会自动处理拨号）
	return s.host.Connect(ctx, info)
}
