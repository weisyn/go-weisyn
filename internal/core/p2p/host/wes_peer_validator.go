// Package host provides WES peer validation for libp2p.
package host

import (
	"context"
	"fmt"
	"strings"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
)

// WESPeerValidatorImpl 实现 WESPeerValidator 接口
//
// 提供统一的 WES 业务节点验证逻辑，用于：
// 1. 连接管理器权重设置（WESConnNotifee）
// 2. DHT 路由表过滤（RoutingTableFilter）
// 3. K 桶节点验证（validateWESPeer）
type WESPeerValidatorImpl struct {
	host lphost.Host
}

// 编译期检查接口实现
var _ interfaces.WESPeerValidator = (*WESPeerValidatorImpl)(nil)

// NewWESPeerValidator 创建 WES 节点验证器
func NewWESPeerValidator(host lphost.Host) *WESPeerValidatorImpl {
	return &WESPeerValidatorImpl{
		host: host,
	}
}

// IsWESPeer 判断指定 peer 是否是 WES 业务节点
//
// 判断标准：协议列表中包含 "/weisyn/" 前缀的协议
//
// 返回值：
//   - bool: 是否是 WES 节点
//   - error: 验证过程中的错误（如 host/peerstore 不可用）
func (v *WESPeerValidatorImpl) IsWESPeer(ctx context.Context, peerID peer.ID) (bool, error) {
	if v.host == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	// 获取节点支持的协议
	protos, err := v.host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// 检查是否有 "/weisyn/" 协议
	for _, p := range protos {
		if strings.Contains(string(p), "/weisyn/") {
			return true, nil
		}
	}

	return false, nil
}

// IsWESPeerQuick 快速判断（不返回 error）
//
// 适用于不关心错误原因的场景，如连接权重设置
func (v *WESPeerValidatorImpl) IsWESPeerQuick(peerID peer.ID) bool {
	ok, _ := v.IsWESPeer(context.Background(), peerID)
	return ok
}

