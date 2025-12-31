// Package flows 提供可复用的交互流程
package flows

import (
	"context"

	"github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// Contract Flow Ports（端口接口）
//
// 这些接口定义了合约流程需要的后端服务能力，解耦UI交互与具体实现。
// 客户端可以通过 transport（JSON-RPC/REST）或 mock 实现这些接口。
// ============================================================================

// ContractService 合约服务端口接口
//
// 功能：
//   - 提供合约部署、调用、查询能力
//   - 后端自动处理：WASM验证、内容哈希计算、交易构建、签名、提交
//
// 实现方式：
//   - 通过 JSON-RPC/REST 客户端调用节点 API
//   - Mock 实现用于测试
type ContractService interface {
	// DeployContract 部署智能合约
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 合约部署请求
	//
	// 返回：
	//   - result: 部署结果（包含合约ID/ContentHash）
	//   - error: 错误信息
	DeployContract(ctx context.Context, req *ContractDeployRequest) (*ContractDeployResult, error)

	// CallContract 调用智能合约
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 合约调用请求
	//
	// 返回：
	//   - result: 调用结果
	//   - error: 错误信息
	CallContract(ctx context.Context, req *ContractCallRequest) (*ContractCallResult, error)

	// QueryContract 查询合约状态（只读调用）
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 合约查询请求
	//
	// 返回：
	//   - result: 查询结果
	//   - error: 错误信息
	QueryContract(ctx context.Context, req *ContractQueryRequest) (*ContractQueryResult, error)
}

// ============================================================================
// 合约部署相关类型
// ============================================================================

// ContractDeployRequest 合约部署请求
type ContractDeployRequest struct {
	WalletName  string                            // 钱包名称（用于获取私钥）
	Password    string                            // 钱包密码（用于解锁私钥）
	FilePath    string                            // WASM 文件路径
	Config      *resource.ContractExecutionConfig // 执行配置（abi_version/exported_functions）
	Name        string                            // 合约名称
	Description string                            // 合约描述（可选）
}

// ContractDeployResult 合约部署结果
type ContractDeployResult struct {
	ContentHash string // 内容哈希（合约地址，用于调用和引用）
	TxHash      string // 交易哈希
	Success     bool   // 是否成功
	Message     string // 结果消息
}

// ============================================================================
// 合约调用相关类型
// ============================================================================

// ContractCallRequest 合约调用请求
type ContractCallRequest struct {
	WalletName  string   // 钱包名称（用于获取私钥）
	Password    string   // 钱包密码（用于解锁私钥）
	ContentHash []byte   // 合约地址（32字节内容哈希）
	Method      string   // 方法名称
	Params      []uint64 // 方法参数（WASM u64 数组）
	Payload     []byte   // 合约调用参数（JSON/二进制负载）
}

// ContractCallResult 合约调用结果
type ContractCallResult struct {
	TxHash     string      // 交易哈希
	Results    []uint64    // 执行结果（WASM u64 数组）
	ReturnData []byte      // 业务返回数据
	Events     []EventInfo // 事件列表
	Success    bool        // 是否成功
	Message    string      // 结果消息
}

// EventInfo 事件信息
type EventInfo struct {
	Type      string                 // 事件类型
	Timestamp int64                  // 时间戳
	Data      map[string]interface{} // 事件数据
}

// ============================================================================
// 合约查询相关类型
// ============================================================================

// ContractQueryRequest 合约查询请求（只读调用）
type ContractQueryRequest struct {
	ContentHash string   // 合约地址（内容哈希）
	Method      string   // 查询方法
	Params      []uint64 // 方法参数
}

// ContractQueryResult 合约查询结果
type ContractQueryResult struct {
	Results    []uint64               // 执行结果
	ReturnData []byte                 // 返回数据
	Success    bool                   // 是否成功
	Message    string                 // 结果消息
	Metadata   map[string]interface{} // 额外元数据
}

