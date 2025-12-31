// Package hash 提供交易哈希服务的客户端实现
package hash

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/grpc"
)

// LocalTransactionHashClient 本地TransactionHashService客户端实现
// 直接调用本地服务，无需网络通信，避免gRPC开销
type LocalTransactionHashClient struct {
	service *TransactionHashService
}

// NewLocalTransactionHashClient 创建本地交易哈希客户端
func NewLocalTransactionHashClient(service *TransactionHashService) transaction.TransactionHashServiceClient {
	return &LocalTransactionHashClient{
		service: service,
	}
}

// ComputeHash 实现TransactionHashServiceClient接口
func (c *LocalTransactionHashClient) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	return c.service.ComputeHash(ctx, in)
}

// ValidateHash 实现TransactionHashServiceClient接口
func (c *LocalTransactionHashClient) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return c.service.ValidateHash(ctx, in)
}

// ComputeSignatureHash 实现TransactionHashServiceClient接口
func (c *LocalTransactionHashClient) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return c.service.ComputeSignatureHash(ctx, in)
}

// ValidateSignatureHash 实现TransactionHashServiceClient接口
func (c *LocalTransactionHashClient) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return c.service.ValidateSignatureHash(ctx, in)
}

