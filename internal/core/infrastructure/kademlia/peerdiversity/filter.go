package peerdiversity

import (
	"fmt"
	"net"
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerIPGroupKey 是一个唯一键，表示对等节点所属的IP组中的一个组
// 基于defs-back/kbucket/peerdiversity/filter.go的设计
type PeerIPGroupKey string

// PeerGroupInfo 表示对等节点的分组信息
type PeerGroupInfo struct {
	Id         peer.ID        // 对等节点的唯一标识符
	Cpl        int            // 共同前缀长度（Common Prefix Length）
	IPGroupKey PeerIPGroupKey // 对等节点所属的IP组键
}

// PeerIPGroupFilter 是由调用方实现的接口，用于实例化peerdiversity.Filter
type PeerIPGroupFilter interface {
	// Allow 判断给定的节点组是否被允许
	Allow(p PeerGroupInfo) bool

	// Disallow 判断给定的节点组是否被禁止
	Disallow(p PeerGroupInfo) bool
}

// Filter 实现节点多样性过滤
// 确保路由表中的节点具有IP地址的多样性
type Filter struct {
	mu     sync.RWMutex
	logger log.Logger

	// IP分组相关
	ipGroupKeys map[peer.ID][]PeerIPGroupKey
	groupCounts map[PeerIPGroupKey]int

	// 配置参数
	maxPeersPerGroup int

	// 过滤器
	ipGroupFilter PeerIPGroupFilter
}

// NewFilter 创建新的多样性过滤器
func NewFilter(logger log.Logger, maxPeersPerGroup int, filter PeerIPGroupFilter) *Filter {
	return &Filter{
		logger:           logger,
		ipGroupKeys:      make(map[peer.ID][]PeerIPGroupKey),
		groupCounts:      make(map[PeerIPGroupKey]int),
		maxPeersPerGroup: maxPeersPerGroup,
		ipGroupFilter:    filter,
	}
}

// AllowPeer 检查是否允许添加给定的节点
func (f *Filter) AllowPeer(p peer.ID, addrs []string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// 为节点的每个地址计算IP组键
	groupKeys := f.computeIPGroupKeys(addrs)

	// 检查每个组是否还有空间
	for _, groupKey := range groupKeys {
		if f.groupCounts[groupKey] >= f.maxPeersPerGroup {
			f.logger.Debugf("节点 %s 被拒绝：IP组 %s 已满", p, groupKey)
			return false
		}

		// 如果有自定义过滤器，使用它进行检查
		if f.ipGroupFilter != nil {
			groupInfo := PeerGroupInfo{
				Id:         p,
				IPGroupKey: groupKey,
			}
			if !f.ipGroupFilter.Allow(groupInfo) {
				f.logger.Debugf("节点 %s 被自定义过滤器拒绝", p)
				return false
			}
		}
	}

	return true
}

// AddPeer 添加节点到过滤器
func (f *Filter) AddPeer(p peer.ID, addrs []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 计算IP组键
	groupKeys := f.computeIPGroupKeys(addrs)

	// 记录节点的组键
	f.ipGroupKeys[p] = groupKeys

	// 更新组计数
	for _, groupKey := range groupKeys {
		f.groupCounts[groupKey]++
	}

	f.logger.Debugf("添加节点 %s 到 %d 个IP组", p, len(groupKeys))
}

// RemovePeer 从过滤器中移除节点
func (f *Filter) RemovePeer(p peer.ID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	groupKeys, exists := f.ipGroupKeys[p]
	if !exists {
		return
	}

	// 减少组计数
	for _, groupKey := range groupKeys {
		f.groupCounts[groupKey]--
		if f.groupCounts[groupKey] <= 0 {
			delete(f.groupCounts, groupKey)
		}
	}

	// 移除节点记录
	delete(f.ipGroupKeys, p)

	f.logger.Debugf("从过滤器中移除节点 %s", p)
}

// GetStats 获取过滤器统计信息
func (f *Filter) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return map[string]interface{}{
		"total_peers":   len(f.ipGroupKeys),
		"total_groups":  len(f.groupCounts),
		"max_per_group": f.maxPeersPerGroup,
		"group_counts":  f.copyGroupCounts(),
	}
}

// computeIPGroupKeys 为给定的地址列表计算IP组键
func (f *Filter) computeIPGroupKeys(addrs []string) []PeerIPGroupKey {
	var groupKeys []PeerIPGroupKey
	seen := make(map[PeerIPGroupKey]bool)

	for _, addr := range addrs {
		groupKey := f.computeIPGroupKey(addr)
		if groupKey != "" && !seen[groupKey] {
			groupKeys = append(groupKeys, groupKey)
			seen[groupKey] = true
		}
	}

	return groupKeys
}

// computeIPGroupKey 为单个地址计算IP组键
func (f *Filter) computeIPGroupKey(addr string) PeerIPGroupKey {
	ip := net.ParseIP(addr)
	if ip == nil {
		// 尝试提取IP地址（可能包含端口）
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return ""
		}
		ip = net.ParseIP(host)
		if ip == nil {
			return ""
		}
	}

	if ip.To4() != nil {
		// IPv4地址：使用/16前缀作为组键
		return PeerIPGroupKey(fmt.Sprintf("ipv4:%d.%d.0.0/16", ip[12], ip[13]))
	} else {
		// IPv6地址：使用/32前缀作为组键
		return PeerIPGroupKey(fmt.Sprintf("ipv6:%02x%02x:%02x%02x::/32",
			ip[0], ip[1], ip[2], ip[3]))
	}
}

// copyGroupCounts 创建组计数的副本
func (f *Filter) copyGroupCounts() map[PeerIPGroupKey]int {
	counts := make(map[PeerIPGroupKey]int)
	for k, v := range f.groupCounts {
		counts[k] = v
	}
	return counts
}

// DefaultPeerIPGroupFilter 默认的IP组过滤器实现
type DefaultPeerIPGroupFilter struct {
	maxGroupSize int
}

// NewDefaultPeerIPGroupFilter 创建默认的IP组过滤器
func NewDefaultPeerIPGroupFilter(maxGroupSize int) *DefaultPeerIPGroupFilter {
	return &DefaultPeerIPGroupFilter{
		maxGroupSize: maxGroupSize,
	}
}

// Allow 检查是否允许给定的节点组
func (f *DefaultPeerIPGroupFilter) Allow(p PeerGroupInfo) bool {
	// 简单的实现：总是允许，具体限制由Filter处理
	return true
}

// Disallow 检查是否禁止给定的节点组
func (f *DefaultPeerIPGroupFilter) Disallow(p PeerGroupInfo) bool {
	// 简单的实现：从不主动禁止
	return false
}

// LegacyClassANetworks 传统Class A网络列表
// 基于defs-back/kbucket/peerdiversity/filter.go中的legacyClassA
var LegacyClassANetworks = []string{
	"12.0.0.0/8", "17.0.0.0/8", "19.0.0.0/8", "38.0.0.0/8",
	"48.0.0.0/8", "56.0.0.0/8", "73.0.0.0/8", "53.0.0.0/8",
}

// IsLegacyClassA 检查IP是否属于传统Class A网络
func IsLegacyClassA(ip net.IP) bool {
	if ip.To4() == nil {
		return false // 不是IPv4
	}

	for _, network := range LegacyClassANetworks {
		_, cidr, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
