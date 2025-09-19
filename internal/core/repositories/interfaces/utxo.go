package interfaces

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// InternalUTXOManager 内部UTXO数据管理器接口
//
// 🎯 设计原则：简单继承公共接口
//
// 继承所有公共UTXO管理方法，提供UTXO数据访问层的完整功能。
// 本接口专注于UTXO状态管理和查询，为内部实现层提供统一的UTXO操作规范。
//
// 📋 继承功能：
// - 核心查询接口：GetUTXO, GetUTXOsByAddress
// - 核心状态操作：ReferenceUTXO, UnreferenceUTXO
//
// 💡 设计特点：
// - 遵循"数据源头约束"原则，所有UTXO数据来源于TxOutput
// - 支持ResourceUTXO并发引用机制
// - 提供高效的地址余额计算数据支撑
//
// 💡 内部扩展：
// 当前版本保持简单继承，未来可根据内部实现需要添加专门的内部方法。
type InternalUTXOManager interface {
	repository.UTXOManager // 继承所有公共UTXO管理方法

	// 内部区块处理方法
	ProcessBlockUTXOs(ctx context.Context, tx storage.BadgerTransaction, block *core.Block, blockHash []byte, txHashes [][]byte) error
}
