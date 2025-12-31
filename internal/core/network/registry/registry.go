// Package registry provides protocol registry functionality for network communication.
package registry

import (
	"sync"
	"time"

	iface "github.com/weisyn/v1/pkg/interfaces/network"
)

// registry.go
// 能力：
// - 协议表/处理器注册与查找：维护 协议ID → handler 的映射，提供查询/遍历
// - 协议信息维护：为上层提供协议信息快照（ProtocolInfo）
// - 与版本无关：不做协商与兼容判断（见 negotiation.go/compatibility.go）
// 依赖/公共接口：
// - pkg/interfaces/network.ProtocolService：对外暴露的协议服务接口（由 service.go 聚合）
// - 可选桥接：pkg/interfaces/p2p.StreamProtocolManager 用于向底层注册流式处理器（若需要）
// 目的：
// - 为 stream/dispatcher 提供 handler 查询，避免直接依赖上层业务
// 非目标：
// - 不直接访问 libp2p.Host（通过更高层适配）
// - 不做消息编解码/签名校验（见 impl/internal/*）

// ProtocolRegistry 维护协议到处理器与协议信息的映射表
// 并发安全实现：读多写少场景使用 RWMutex
type ProtocolRegistry struct {
	mu       sync.RWMutex
	handlers map[string]iface.MessageHandler
	infos    map[string]iface.ProtocolInfo
}

// NewProtocolRegistry 创建协议注册表实例（方法框架）
// 返回：ProtocolRegistry 指针；内部字段可能为空，后续实现填充
func NewProtocolRegistry() *ProtocolRegistry {
	return &ProtocolRegistry{handlers: make(map[string]iface.MessageHandler), infos: make(map[string]iface.ProtocolInfo)}
}

// Register 注册协议处理器
// 参数：
//   - protocol: 协议ID（含版本），例如 "/weisyn/consensus/1.0.0"
//   - handler: 协议对应的消息处理器
//
// 返回：
//   - error: 注册失败时返回错误
//
// 说明：覆盖同名协议的处理器；infos 同步更新时间
func (r *ProtocolRegistry) Register(protocol string, handler iface.MessageHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handlers == nil {
		r.handlers = make(map[string]iface.MessageHandler)
	}
	if r.infos == nil {
		r.infos = make(map[string]iface.ProtocolInfo)
	}
	r.handlers[protocol] = handler
	r.infos[protocol] = iface.ProtocolInfo{ID: protocol, Version: "", RegisteredAt: time.Now(), Metadata: map[string]string{}}
	return nil
}

// Unregister 注销指定协议处理器
func (r *ProtocolRegistry) Unregister(protocol string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, protocol)
	delete(r.infos, protocol)
	return nil
}

// Get 获取指定协议的处理器
func (r *ProtocolRegistry) Get(protocol string) (iface.MessageHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[protocol]
	return h, ok
}

// List 返回当前已注册的协议信息快照
func (r *ProtocolRegistry) List() []iface.ProtocolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]iface.ProtocolInfo, 0, len(r.infos))
	for _, info := range r.infos {
		out = append(out, info)
	}
	return out
}

// Info 返回指定协议的协议信息
func (r *ProtocolRegistry) Info(protocol string) (*iface.ProtocolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if info, ok := r.infos[protocol]; ok {
		cp := info
		return &cp, true
	}
	return nil, false
}

// ResolveHandler 标准化查找桥接点（方法框架）：供 stream/dispatcher 直连调用
// 返回：处理器与是否存在标记
func (r *ProtocolRegistry) ResolveHandler(protocol string) (iface.MessageHandler, bool) {
	return r.Get(protocol)
}
