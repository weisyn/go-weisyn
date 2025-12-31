package kbucket

import (
	"context"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	libhost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/pkg/constants/protocols"
)

type connectednessOverrideNetwork struct {
	libnetwork.Network
	c libnetwork.Connectedness
}

func (n connectednessOverrideNetwork) Connectedness(_ libpeer.ID) libnetwork.Connectedness {
	return n.c
}

type networkOverrideHost struct {
	libhost.Host
	n libnetwork.Network
}

func (h networkOverrideHost) Network() libnetwork.Network { return h.n }

func newTestHost(t *testing.T) libhost.Host {
	t.Helper()
	// Use an explicit dialable loopback listen address. (Some default addrs can be 0.0.0.0, which is not dialable.)
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	return h
}

func TestValidateWESPeer_AllowsConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1Real := newTestHost(t)
	defer h1Real.Close()

	h2 := newTestHost(t)
	defer h2.Close()

	// Ensure the peer advertises at least one /weisyn/ protocol in peerstore.
	_ = h1Real.Peerstore().AddProtocols(h2.ID(), libprotocol.ID(protocols.ProtocolNodeInfo))

	// Override Connectedness to Connected deterministically.
	h1 := networkOverrideHost{
		Host: h1Real,
		n:    connectednessOverrideNetwork{Network: h1Real.Network(), c: libnetwork.Connected},
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: "testns"})
	mgr := mgrIface.(*RoutingTableManager)

	ok, vErr := mgr.validateWESPeer(ctx, h2.ID())
	require.NoError(t, vErr)
	require.True(t, ok)
}

func TestValidateWESPeer_AllowsCanConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1Real := newTestHost(t)
	defer h1Real.Close()

	h2 := newTestHost(t)
	defer h2.Close()

	// Add some dial metadata to the peerstore (not strictly required for this unit test).
	h1Real.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	_ = h1Real.Peerstore().AddProtocols(h2.ID(), libprotocol.ID(protocols.ProtocolNodeInfo))

	// Override Connectedness to CanConnect deterministically.
	h1 := networkOverrideHost{
		Host: h1Real,
		n:    connectednessOverrideNetwork{Network: h1Real.Network(), c: libnetwork.CanConnect},
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: "testns"})
	mgr := mgrIface.(*RoutingTableManager)

	ok, vErr := mgr.validateWESPeer(ctx, h2.ID())
	require.NoError(t, vErr)
	require.True(t, ok)
}

func TestValidateWESPeer_AllowsNotConnectedWithProtocols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1Real := newTestHost(t)
	defer h1Real.Close()

	h2 := newTestHost(t)
	defer h2.Close()

	// ✅ 修复缺陷L：NotConnected + 有协议缓存 -> 允许入桶
	// 场景：连接管理器淘汰连接，但 peer 之前成功 Identify，有协议信息
	_ = h1Real.Peerstore().AddProtocols(h2.ID(), libprotocol.ID(protocols.ProtocolNodeInfo))

	// Override Connectedness to NotConnected deterministically.
	h1 := networkOverrideHost{
		Host: h1Real,
		n:    connectednessOverrideNetwork{Network: h1Real.Network(), c: libnetwork.NotConnected},
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: "testns"})
	mgr := mgrIface.(*RoutingTableManager)

	ok, vErr := mgr.validateWESPeer(ctx, h2.ID())
	require.NoError(t, vErr)
	require.True(t, ok, "should allow NotConnected peer with protocol cache")
}

func TestValidateWESPeer_RejectsNotConnectedWithoutProtocols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1Real := newTestHost(t)
	defer h1Real.Close()

	h2 := newTestHost(t)
	defer h2.Close()

	// 没有协议信息：NotConnected 应该被拒绝
	// （不添加协议到 peerstore）

	// Override Connectedness to NotConnected deterministically.
	h1 := networkOverrideHost{
		Host: h1Real,
		n:    connectednessOverrideNetwork{Network: h1Real.Network(), c: libnetwork.NotConnected},
	}

	mgrIface := NewRoutingTableManager(GetDefaultKBucketConfig(), nopLogger{}, stubP2PService{host: h1}, stubConfigProvider{ns: "testns"})
	mgr := mgrIface.(*RoutingTableManager)

	ok, vErr := mgr.validateWESPeer(ctx, h2.ID())
	require.NoError(t, vErr)
	require.False(t, ok, "should reject NotConnected peer without protocols")
}


