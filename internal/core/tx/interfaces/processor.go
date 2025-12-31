// Package interfaces provides transaction processor interfaces.
package interfaces

import (
	txevent "github.com/weisyn/v1/internal/core/tx/integration/event"
	txnet "github.com/weisyn/v1/internal/core/tx/integration/network"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// Processor 交易处理器内部接口
//
// 🎯 **职责**：对外统一入口，协调 Verifier + TxPool 完成交易提交，并处理网络和事件集成
//
// 🔄 **继承关系**：
//   - 继承 tx.TxProcessor 公共接口（交易处理核心能力）
//   - 继承 integration/network 网络协议接口（网络交易接收能力）
//   - 继承 integration/event 事件订阅接口（交易状态跟踪能力）
//
// 📁 **实现目录**：internal/core/tx/processor/
//   - service.go            - 核心处理逻辑
//   - network_handler/      - 网络协议处理实现
//   - event_handler/        - 事件订阅处理实现
//
// 💡 **设计说明**：
//   - Processor 是协调器，不做具体验证逻辑
//   - 核心流程：Verify(通过Verifier) → Submit(到TxPool) → TxPool内部自动广播
//   - 网络能力：接收P2P网络的交易并处理（解析 → 去重 → 验证 → 入池）
//   - 事件能力：监听交易生命周期事件，维护统计和状态跟踪
//   - 验证失败不入池，验证通过后入池并自动广播
//
// ⚠️ **核心约束**：
//   - 必须先验证后提交（不允许跳过验证）
//   - 验证失败立即返回错误
//   - 验证通过后入池，广播由 TxPool 内部处理
//   - 网络处理只负责接收，不负责主动广播
//   - 事件处理只负责监听和统计，不修改交易状态
type Processor interface {
	// ==================== 继承公共接口 ====================

	// 继承公共交易处理器接口
	tx.TxProcessor

	// ==================== 继承网络协议接口 ====================

	// 网络交易接收能力：从P2P网络接收交易并处理
	// 实现 integration/network 层定义的协议接口
	txnet.TxProtocolRouter // 流式协议：HandleTransactionDirect
	txnet.TxAnnounceRouter // 订阅协议：HandleTransactionAnnounce

	// ==================== 继承事件订阅接口 ====================

	// 交易状态跟踪能力：监听交易生命周期事件
	// 实现 integration/event 层定义的事件订阅接口
	txevent.TransactionEventSubscriber

	// ==================== 内部扩展方法 ====================

	// 💡 内部扩展方法（暂无，保留接口以便未来扩展）
	// 如需添加内部专用方法（如批量提交、优先级调度等），在此扩展
}
