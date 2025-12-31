package discovery

import (
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
)

// wasRecentlyConnected 检查peer是否最近有连接记录
// 用于判断无地址peer是否应该高优先级重发现
func (s *Service) wasRecentlyConnected(pid libpeer.ID) bool {
	if s.host == nil {
		return false
	}

	// 检查当前连接状态
	connectedness := s.host.Network().Connectedness(pid)
	
	// 如果当前已连接或可连接，则认为是"最近有连接"
	return connectedness == libnetwork.Connected || connectedness == libnetwork.CanConnect
}

