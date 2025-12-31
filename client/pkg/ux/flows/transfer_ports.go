// Package flows 提供可复用的交互流程
package flows

import (
	"context"
)

// ============================================================================
// Transfer Flow Ports（端口接口）
//
// 这些接口定义了转账流程需要的后端服务能力，解耦UI交互与具体实现。
// 客户端可以通过 transport（JSON-RPC/REST）或 mock 实现这些接口。
// ============================================================================

// TransferService 转账服务端口接口
//
// 功能：
//   - 提供单笔转账、批量转账、时间锁转账能力
//   - 后端自动处理：余额检查、UTXO选择、找零、手续费估算
//
// 实现方式：
//   - 通过 JSON-RPC/REST 客户端调用节点 API
//   - Mock 实现用于测试
type TransferService interface {
	// Transfer 执行单笔转账
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 转账请求
	//
	// 返回：
	//   - txHash: 交易哈希
	//   - error: 错误信息
	Transfer(ctx context.Context, req *TransferRequest) (txHash string, err error)

	// BatchTransfer 执行批量转账
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 批量转账请求
	//
	// 返回：
	//   - txHash: 交易哈希
	//   - error: 错误信息
	BatchTransfer(ctx context.Context, req *BatchTransferRequest) (txHash string, err error)

	// TimeLockTransfer 执行时间锁定转账
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 时间锁转账请求
	//
	// 返回：
	//   - txHash: 交易哈希
	//   - error: 错误信息
	TimeLockTransfer(ctx context.Context, req *TimeLockTransferRequest) (txHash string, err error)

	// EstimateFee 估算转账手续费
	//
	// 参数：
	//   - ctx: 上下文
	//   - from: 发送方地址
	//   - to: 接收方地址
	//   - amount: 转账金额
	//
	// 返回：
	//   - fee: 手续费估算值
	//   - error: 错误信息
	EstimateFee(ctx context.Context, from, to string, amount uint64) (fee uint64, err error)
}

// TransactionSigner 交易签名器端口接口
//
// 功能：
//   - 使用私钥签名交易
//   - 支持离线签名
type TransactionSigner interface {
	// SignTransaction 签名交易
	//
	// 参数：
	//   - ctx: 上下文
	//   - txHash: 交易哈希
	//   - privateKey: 私钥（字节数组）
	//
	// 返回：
	//   - signedTxHash: 签名后的交易哈希
	//   - error: 错误信息
	SignTransaction(ctx context.Context, txHash []byte, privateKey []byte) (signedTxHash []byte, err error)
}

// TransactionBroadcaster 交易广播器端口接口
//
// 功能：
//   - 将签名后的交易广播到区块链网络
type TransactionBroadcaster interface {
	// BroadcastTransaction 广播交易
	//
	// 参数：
	//   - ctx: 上下文
	//   - signedTx: 签名后的交易数据
	//
	// 返回：
	//   - txHash: 交易哈希
	//   - error: 错误信息
	BroadcastTransaction(ctx context.Context, signedTx []byte) (txHash string, err error)
}

// ============================================================================
// Data Transfer Objects (DTOs)
// ============================================================================

// TransferRequest 单笔转账请求
type TransferRequest struct {
	FromAddress string // 发送方地址
	ToAddress   string // 接收方地址
	Amount      uint64 // 转账金额（最小单位）
	PrivateKey  []byte // 发送方私钥
	Memo        string // 转账备注（可选）
}

// BatchTransferRequest 批量转账请求
type BatchTransferRequest struct {
	FromAddress string         // 发送方地址
	Transfers   []TransferItem // 转账项列表
	PrivateKey  []byte         // 发送方私钥
}

// TransferItem 批量转账项
type TransferItem struct {
	ToAddress string // 接收方地址
	Amount    uint64 // 转账金额（最小单位）
}

// TimeLockTransferRequest 时间锁定转账请求
type TimeLockTransferRequest struct {
	FromAddress string // 发送方地址
	ToAddress   string // 接收方地址
	Amount      uint64 // 转账金额（最小单位）
	LockTime    uint64 // 锁定时间（Unix时间戳）
	PrivateKey  []byte // 发送方私钥
}

// TransferResult 转账结果
type TransferResult struct {
	TxHash      string // 交易哈希
	Success     bool   // 是否成功
	Message     string // 消息
	BlockHeight uint64 // 区块高度（0表示待确认）
}

// FeeEstimate 手续费估算结果
type FeeEstimate struct {
	EstimatedFee uint64 // 估算手续费
	Unit         string // 单位
	Message      string // 说明信息
}
