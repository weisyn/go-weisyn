package runtimectx

import (
	"sync"
)

// NodeMode 表示节点当前的运行模式（用于协调挖矿、交易、查询等子系统行为）
//
// 设计目标：
// - 提供一个轻量级的全局运行模式开关，便于在生产环境中按需“刹车”或“降级”
// - 初期以内存单例实现，后续可接入配置 / 管理接口
type NodeMode int

const (
	// NodeModeNormal 正常模式：允许挖矿 / 交易 / 查询
	NodeModeNormal NodeMode = iota
	// NodeModeDegraded 降级模式：性能退化（例如索引缺失、IO 高压），允许挖矿但做限流/降级
	NodeModeDegraded
	// NodeModeRepairingUTXO UTXO 修复模式：优先修复 UTXO，限制相关写操作（挖矿 / 某些交易）
	NodeModeRepairingUTXO
	// NodeModeReadOnly 只读模式：不再写入新状态，只提供只读查询（极端场景使用）
	NodeModeReadOnly
)

func (m NodeMode) String() string {
	switch m {
	case NodeModeNormal:
		return "Normal"
	case NodeModeDegraded:
		return "Degraded"
	case NodeModeRepairingUTXO:
		return "RepairingUTXO"
	case NodeModeReadOnly:
		return "ReadOnly"
	default:
		return "Unknown"
	}
}

// UTXOType 表示 UTXO 的类型域（资产 / 资源等）
type UTXOType string

const (
	UTXOTypeAsset    UTXOType = "asset"
	UTXOTypeResource UTXOType = "resource"
)

// UTXOHealthLevel 表示某类 UTXO 的健康状态
type UTXOHealthLevel int

const (
	UTXOHealthHealthy UTXOHealthLevel = iota
	UTXOHealthDegraded
	UTXOHealthInconsistent
)

type stateManager struct {
	mu sync.RWMutex

	mode       NodeMode
	utxoHealth map[UTXOType]UTXOHealthLevel
}

var (
	globalState = &stateManager{
		mode: NodeModeNormal,
		utxoHealth: map[UTXOType]UTXOHealthLevel{
			UTXOTypeAsset:    UTXOHealthHealthy,
			UTXOTypeResource: UTXOHealthHealthy,
		},
	}
)

// GetNodeMode 返回当前节点运行模式
func GetNodeMode() NodeMode {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()
	return globalState.mode
}

// SetNodeMode 设置节点运行模式（供运维 / 状态管理组件调用）
func SetNodeMode(mode NodeMode) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.mode = mode
}

// GetUTXOHealth 返回指定 UTXO 类型的健康状态
func GetUTXOHealth(t UTXOType) UTXOHealthLevel {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()
	level, ok := globalState.utxoHealth[t]
	if !ok {
		return UTXOHealthHealthy
	}
	return level
}

// SetUTXOHealth 设置指定 UTXO 类型的健康状态
func SetUTXOHealth(t UTXOType, level UTXOHealthLevel) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	if globalState.utxoHealth == nil {
		globalState.utxoHealth = make(map[UTXOType]UTXOHealthLevel)
	}
	globalState.utxoHealth[t] = level
}

// IsMiningAllowed 返回当前模式下是否允许执行挖矿轮次
//
// 生产环境约定：
// - Normal / Degraded：允许挖矿（Degraded 下会由 IOGuard 等机制自动减速）
// - RepairingUTXO / ReadOnly：不允许挖矿，优先保证状态修复或只读安全
func IsMiningAllowed() bool {
	mode := GetNodeMode()
	switch mode {
	case NodeModeNormal, NodeModeDegraded:
		return true
	case NodeModeRepairingUTXO, NodeModeReadOnly:
		return false
	default:
		return true
	}
}
