package hash

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
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

// LocalBlockHashClient 本地BlockHashService客户端实现
// 直接调用本地服务，无需网络通信，避免gRPC开销
type LocalBlockHashClient struct {
	service *BlockHashService
}

// NewLocalBlockHashClient 创建本地区块哈希客户端
func NewLocalBlockHashClient(service *BlockHashService) core.BlockHashServiceClient {
	return &LocalBlockHashClient{
		service: service,
	}
}

// ComputeBlockHash 实现BlockHashServiceClient接口
func (c *LocalBlockHashClient) ComputeBlockHash(ctx context.Context, in *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	return c.service.ComputeBlockHash(ctx, in)
}

// ValidateBlockHash 实现BlockHashServiceClient接口
func (c *LocalBlockHashClient) ValidateBlockHash(ctx context.Context, in *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	return c.service.ValidateBlockHash(ctx, in)
}
