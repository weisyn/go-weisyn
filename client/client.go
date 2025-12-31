package client

import (
	"context"
	"time"

	"github.com/weisyn/v1/client/core/transport"
)

// Client WES 区块链客户端 - 统一的客户端入口
// 提供完整的区块链交互能力：查询、交易、合约等
type Client struct {
	transport transport.Client
}

// New 创建新的客户端实例
// nodeURL: 节点 JSON-RPC 地址，如 "http://localhost:28680/jsonrpc"
func New(nodeURL string) *Client {
	return &Client{
		transport: transport.NewJSONRPCClient(nodeURL, 30*time.Second),
	}
}

// NewWithTimeout 创建带自定义超时的客户端实例
func NewWithTimeout(nodeURL string, timeout time.Duration) *Client {
	return &Client{
		transport: transport.NewJSONRPCClient(nodeURL, timeout),
	}
}

// NewWithTransport 使用自定义 transport 创建客户端
// 支持自定义的 RPC 客户端实现（JSON-RPC/REST/WebSocket）
func NewWithTransport(t transport.Client) *Client {
	return &Client{
		transport: t,
	}
}

// Transport 获取底层的 transport 客户端
// 用于直接调用 RPC 方法
func (c *Client) Transport() transport.Client {
	return c.transport
}

// === 便捷方法：链信息 ===

// ChainID 获取链 ID
func (c *Client) ChainID(ctx context.Context) (string, error) {
	return c.transport.ChainID(ctx)
}

// BlockNumber 获取最新区块高度
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	return c.transport.BlockNumber(ctx)
}

// === 便捷方法：账户查询 ===

// GetBalance 获取账户余额
func (c *Client) GetBalance(ctx context.Context, address string) (*transport.Balance, error) {
	return c.transport.GetBalance(ctx, address, nil)
}

// GetUTXOs 获取账户 UTXO 列表
func (c *Client) GetUTXOs(ctx context.Context, address string) ([]*transport.UTXO, error) {
	return c.transport.GetUTXOs(ctx, address, nil)
}

// === 便捷方法：交易操作 ===

// SendRawTransaction 发送已签名的原始交易
func (c *Client) SendRawTransaction(ctx context.Context, signedTxHex string) (*transport.SendTxResult, error) {
	return c.transport.SendRawTransaction(ctx, signedTxHex)
}

// GetTransaction 获取交易详情
func (c *Client) GetTransaction(ctx context.Context, txHash string) (*transport.Transaction, error) {
	return c.transport.GetTransaction(ctx, txHash)
}

// GetTransactionReceipt 获取交易回执
func (c *Client) GetTransactionReceipt(ctx context.Context, txHash string) (*transport.Receipt, error) {
	return c.transport.GetTransactionReceipt(ctx, txHash)
}

// === 便捷方法：区块查询 ===

// GetBlockByHeight 根据高度获取区块
func (c *Client) GetBlockByHeight(ctx context.Context, height uint64, fullTx bool) (*transport.Block, error) {
	return c.transport.GetBlockByHeight(ctx, height, fullTx, nil)
}

// GetBlockByHash 根据哈希获取区块
func (c *Client) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (*transport.Block, error) {
	return c.transport.GetBlockByHash(ctx, hash, fullTx)
}
