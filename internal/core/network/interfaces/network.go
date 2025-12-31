// Package interfaces 定义 network 组件的内部接口
//
// 🔧 **内部接口层 (Internal Interfaces Layer)**
//
// 本包定义 network 组件的内部接口，作为公共接口和具体实现之间的桥梁。
//
// 🎯 **核心职责**：
// - 继承公共接口（network.Network）
// - 扩展内部专用方法（如需要）
//
// 🏗️ **架构定位**：
// ```
// pkg/interfaces/network (公共接口)
//
//	↓ 继承
//
// internal/core/network/interfaces (内部接口) ← 本目录
//
//	↓ 实现
//
// internal/core/network/facade (服务实现)
// ```
package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/network"
)

// InternalNetwork 内部网络接口
//
// 🎯 **核心职责**：
// 继承公共接口 network.Network，作为实现层与公共接口的桥接。
//
// 💡 **设计理念**：
// - 继承公共接口：嵌入 network.Network
// - 内部扩展：目前无额外内部方法（纯继承）
// - 实现约束：所有实现必须实现此内部接口
//
// 📋 **继承关系**：
// - 继承：network.Network
//   - RegisterStreamHandler(protoID, handler, opts...) error
//   - UnregisterStreamHandler(protoID) error
//   - Subscribe(topic, handler, opts...) (unsubscribe func() error, err error)
//   - Call(ctx, to, protoID, req, opts) ([]byte, error)
//   - OpenStream(ctx, to, protoID, opts) (StreamHandle, error)
//   - Publish(ctx, topic, data, opts) error
//   - ListProtocols() []ProtocolInfo
//   - GetProtocolInfo(protoID) *ProtocolInfo
//   - GetTopicPeers(topic) []peer.ID
//   - IsSubscribed(topic) bool
//   - CheckProtocolSupport(ctx, peerID, protocol) (bool, error)
//
// ⚠️ **注意事项**：
// - 内部接口仅用于实现层，不对外暴露
// - 通过 module.go 绑定到公共接口
// - 如果未来需要内部协作方法，可在此扩展
type InternalNetwork interface {
	network.Network // 嵌入公共接口（强制继承）

	// 内部专用方法（目前无，如需要可在此添加）
	//
	// 💡 **何时添加内部方法**：
	// - 组件内部模块间需要协作
	// - 需要暴露给组件内部但不应暴露到公共接口的方法
	// - 例如：ForceInitializeGossipSub() 供 module.go 生命周期管理使用
	//
	// ⚠️ **注意**：
	// - 内部方法通常小写（包内可见）
	// - 仅在确实需要跨实现域调用时才添加
	// - 如果只是同一实现域内的私有方法，直接定义为私有方法即可
}
