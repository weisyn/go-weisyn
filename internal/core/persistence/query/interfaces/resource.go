package interfaces

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"

	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// InternalResourceQuery 内部资源查询接口
// 继承公共接口 persistence.ResourceQuery，遵循代码组织规范
//
// ⚠️ **Phase 4：标识协议收紧**
// - 在公共接口的基础上，增加基于 ResourceInstanceId 的实例级查询能力
type InternalResourceQuery interface {
	persistence.ResourceQuery // 嵌入公共接口

	// GetResourceByInstance 根据资源实例标识获取资源
	//
	// 🎯 **用途**：
	// - 通过 ResourceInstanceId（OutPoint）查询具体 Resource 对象
	// - 支持多实例部署场景下的精确查询
	//
	// 参数：
	//   - ctx: 上下文
	//   - txHash: 交易哈希（32 字节）
	//   - outputIndex: 输出索引
	//
	// 返回：
	//   - *pb_resource.Resource: 资源对象
	//   - bool: 是否存在
	//   - error: 查询错误
	GetResourceByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*pb_resource.Resource, bool, error)

	// ListResourceInstancesByCode 列出指定代码的所有实例 OutPoint
	//
	// 🎯 **用途**：
	// - 通过 ResourceCodeId（ContentHash）获取所有实例 OutPoint
	// - 支持 1 个 CodeId → N 个 InstanceId 的完整视图
	//
	// 参数：
	//   - ctx: 上下文
	//   - contentHash: 资源内容哈希（ResourceCodeId）
	//
	// 返回：
	//   - []*transaction.OutPoint: 实例列表
	//   - error: 查询错误
	ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*transaction.OutPoint, error)
}

