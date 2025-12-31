// Package hash 提供区块哈希服务的客户端实现
package hash

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/grpc"
)

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

