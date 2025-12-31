// Package interfaces 定义 mempool 组件的内部接口
//
// 🔧 **内部接口层 (Internal Interfaces Layer)**
//
// 本包定义 mempool 组件的内部接口，作为公共接口和具体实现之间的桥梁。
//
// 🎯 **核心职责**：
// - 继承公共接口（mempool.TxPool）
// - 扩展内部专用方法（如需要）
//
// 🏗️ **架构定位**：
// ```
// pkg/interfaces/mempool (公共接口)
//
//	↓ 继承
//
// internal/core/mempool/interfaces (内部接口) ← 本目录
//
//	↓ 实现
//
// internal/core/mempool/txpool (服务实现)
// ```
package interfaces

import (
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// InternalTxPool 交易池内部接口
//
// 🎯 **核心职责**：
// 继承公共接口 mempoolIfaces.TxPool，作为实现层与公共接口的桥接。
//
// 💡 **设计理念**：
// - 继承公共接口：嵌入 mempoolIfaces.TxPool
// - 内部扩展：目前无额外内部方法（纯继承）
// - 实现约束：所有实现必须实现此内部接口
//
// 📋 **继承关系**：
// - 继承：mempoolIfaces.TxPool
//   - SubmitTx(tx *transaction.Transaction) ([]byte, error)
//   - SubmitTxs(txs []*transaction.Transaction) ([][]byte, error)
//   - GetTransactionsForMining() ([]*transaction.Transaction, error)
//   - MarkTransactionsAsMining(txIDs [][]byte) error
//   - ConfirmTransactions(txIDs [][]byte, blockHeight uint64) error
//   - RejectTransactions(txIDs [][]byte) error
//   - MarkTransactionsAsPendingConfirm(txIDs [][]byte, blockHeight uint64) error
//   - SyncStatus(height uint64, stateRoot []byte) error
//   - UpdateTransactionStatus(txID []byte, status types.TxStatus) error
//   - GetAllPendingTransactions() ([]*transaction.Transaction, error)
//   - GetTx(txID []byte) (*transaction.Transaction, error)
//   - GetTxStatus(txID []byte) (types.TxStatus, error)
//   - GetTransactionsByStatus(status types.TxStatus) ([]*transaction.Transaction, error)
//   - GetTransactionByID(txID []byte) (*transaction.Transaction, error)
//   - GetPendingTransactions() ([]*transaction.Transaction, error)
//
// ⚠️ **注意事项**：
// - 内部接口仅用于实现层，不对外暴露
// - 通过 module.go 绑定到公共接口
// - 如果未来需要内部协作方法，可在此扩展
type InternalTxPool interface {
	mempoolIfaces.TxPool // 嵌入公共接口（强制继承）

	// 内部专用方法（目前无，如需要可在此添加）
	//
	// 💡 **何时添加内部方法**：
	// - 组件内部模块间需要协作
	// - 需要暴露给组件内部但不应暴露到公共接口的方法
	// - 例如：SetEventSink(sink TxEventSink) 供 event_handler 注入使用
	//
	// ⚠️ **注意**：
	// - 内部方法通常小写（包内可见）
	// - 仅在确实需要跨实现域调用时才添加
	// - 如果只是同一实现域内的私有方法，直接定义为私有方法即可
}

