package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// InternalRepositoryManager 内部数据仓储管理器接口
//
// 🎯 设计原则：简单继承公共接口
//
// 继承所有公共数据仓储方法，提供数据访问层的完整功能。
// 本接口专注于数据存储和查询，为内部实现层提供统一的数据访问规范。
//
// 📋 继承功能：
// - 区块数据操作：StoreBlock, GetBlock, GetBlockByHeight, GetBlockRange, GetHighestBlock
// - 交易权利管理：GetTransaction, GetAccountNonce, GetTransactionsByBlock
// - 资源能力管理：GetResourceByContentHash
//
// 💡 内部扩展：
// 当前版本保持简单继承，未来可根据内部实现需要添加专门的内部方法。
type InternalRepositoryManager interface {
	repository.RepositoryManager // 继承所有公共数据仓储方法

	// 此处可根据内部实现需要扩展专门的内部方法
	// 例如：内部缓存管理、批量操作优化等
}
