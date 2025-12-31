package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	// 节点端点(按优先级排序)
	Endpoints []EndpointConfig `json:"endpoints"`

	// 超时配置
	Timeout       time.Duration `json:"timeout"`
	RetryAttempts int           `json:"retry_attempts"`
	RetryBackoff  time.Duration `json:"retry_backoff"`

	// 健康检查
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// EndpointConfig 端点配置
type EndpointConfig struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"` // 优先级,数字越小越优先

	// 协议端点
	JSONRPC string `json:"jsonrpc,omitempty"`
	REST    string `json:"rest,omitempty"`
	WS      string `json:"ws,omitempty"`
	GRPC    string `json:"grpc,omitempty"`
}

// FallbackClient 支持故障转移的客户端
type FallbackClient struct {
	config    ClientConfig
	clients   []clientWithPriority
	current   int
	mu        sync.RWMutex
	closeCh   chan struct{}
	closeOnce sync.Once
}

type clientWithPriority struct {
	name      string
	priority  int
	client    Client
	healthy   bool
	lastCheck time.Time
}

// NewFallbackClient 创建支持故障转移的客户端
func NewFallbackClient(config ClientConfig) (*FallbackClient, error) {
	if len(config.Endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}

	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}

	fc := &FallbackClient{
		config:  config,
		clients: make([]clientWithPriority, 0, len(config.Endpoints)),
		closeCh: make(chan struct{}),
	}

	// 创建客户端
	for _, ep := range config.Endpoints {
		var client Client
		var err error

		// 优先使用JSON-RPC
		if ep.JSONRPC != "" {
			client = NewJSONRPCClient(ep.JSONRPC, config.Timeout)
		} else if ep.REST != "" {
			client = NewRESTClient(ep.REST, config.Timeout)
		} else {
			continue // 跳过无效端点
		}

		fc.clients = append(fc.clients, clientWithPriority{
			name:     ep.Name,
			priority: ep.Priority,
			client:   client,
			healthy:  true, // 初始假设健康
		})

		if err != nil {
			// 记录但不失败
			continue
		}
	}

	if len(fc.clients) == 0 {
		return nil, fmt.Errorf("no valid clients created")
	}

	// 按优先级排序
	fc.sortByPriority()

	// 启动健康检查
	go fc.healthCheckLoop()

	return fc, nil
}

// sortByPriority 按优先级排序客户端
func (fc *FallbackClient) sortByPriority() {
	// 简单冒泡排序(客户端数量少)
	for i := 0; i < len(fc.clients)-1; i++ {
		for j := i + 1; j < len(fc.clients); j++ {
			if fc.clients[i].priority > fc.clients[j].priority {
				fc.clients[i], fc.clients[j] = fc.clients[j], fc.clients[i]
			}
		}
	}
}

// healthCheckLoop 健康检查循环
func (fc *FallbackClient) healthCheckLoop() {
	ticker := time.NewTicker(fc.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fc.checkAllClients()
		case <-fc.closeCh:
			return
		}
	}
}

// checkAllClients 检查所有客户端健康状态
func (fc *FallbackClient) checkAllClients() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fc.mu.Lock()
	defer fc.mu.Unlock()

	for i := range fc.clients {
		err := fc.clients[i].client.Ping(ctx)
		fc.clients[i].healthy = (err == nil)
		fc.clients[i].lastCheck = time.Now()
	}
}

// getClient 获取当前可用客户端
func (fc *FallbackClient) getClient() Client {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// 优先使用当前客户端
	if fc.current < len(fc.clients) && fc.clients[fc.current].healthy {
		return fc.clients[fc.current].client
	}

	// 查找下一个健康的客户端
	for i, c := range fc.clients {
		if c.healthy {
			fc.current = i
			return c.client
		}
	}

	// 所有客户端都不健康,返回第一个
	if len(fc.clients) > 0 {
		return fc.clients[0].client
	}

	return nil
}

// tryWithFallback 尝试执行操作,失败时降级
func (fc *FallbackClient) tryWithFallback(ctx context.Context, op func(Client) error) error {
	var lastErr error

	for attempt := 0; attempt < fc.config.RetryAttempts; attempt++ {
		client := fc.getClient()
		if client == nil {
			return fmt.Errorf("no available client")
		}

		err := op(client)
		if err == nil {
			return nil
		}

		lastErr = err

		// 标记当前客户端不健康
		fc.mu.Lock()
		if fc.current < len(fc.clients) {
			fc.clients[fc.current].healthy = false
		}
		fc.mu.Unlock()

		// 退避重试
		if attempt < fc.config.RetryAttempts-1 {
			select {
			case <-time.After(fc.config.RetryBackoff * time.Duration(attempt+1)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("all endpoints failed: %w", lastErr)
}

// ===== Client接口实现(通过tryWithFallback降级) =====

func (fc *FallbackClient) ChainID(ctx context.Context) (string, error) {
	var result string
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.ChainID(ctx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) Syncing(ctx context.Context) (*SyncStatus, error) {
	var result *SyncStatus
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.Syncing(ctx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) BlockNumber(ctx context.Context) (uint64, error) {
	var result uint64
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.BlockNumber(ctx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetBlockByHeight(ctx context.Context, height uint64, fullTx bool, anchor *StateAnchor) (*Block, error) {
	var result *Block
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetBlockByHeight(ctx, height, fullTx, anchor)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (*Block, error) {
	var result *Block
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetBlockByHash(ctx, hash, fullTx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) SendRawTransaction(ctx context.Context, signedTxHex string) (*SendTxResult, error) {
	var result *SendTxResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.SendRawTransaction(ctx, signedTxHex)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	var result *Transaction
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetTransaction(ctx, txHash)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetTransactionReceipt(ctx context.Context, txHash string) (*Receipt, error) {
	var result *Receipt
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetTransactionReceipt(ctx, txHash)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetTransactionHistory(ctx context.Context, txID string, resourceID string, limit int, offset int) ([]*Transaction, error) {
	var result []*Transaction
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetTransactionHistory(ctx, txID, resourceID, limit, offset)
		return e
	})
	return result, err
}

func (fc *FallbackClient) EstimateFee(ctx context.Context, tx *UnsignedTx) (*FeeEstimate, error) {
	var result *FeeEstimate
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.EstimateFee(ctx, tx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetBalance(ctx context.Context, address string, anchor *StateAnchor) (*Balance, error) {
	var result *Balance
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetBalance(ctx, address, anchor)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetContractTokenBalance(ctx context.Context, req *ContractTokenBalanceRequest) (*ContractTokenBalanceResult, error) {
	var result *ContractTokenBalanceResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetContractTokenBalance(ctx, req)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetUTXOs(ctx context.Context, address string, anchor *StateAnchor) ([]*UTXO, error) {
	var result []*UTXO
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetUTXOs(ctx, address, anchor)
		return e
	})
	return result, err
}

func (fc *FallbackClient) Call(ctx context.Context, call *CallRequest, anchor *StateAnchor) (*CallResult, error) {
	var result *CallResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.Call(ctx, call, anchor)
		return e
	})
	return result, err
}

func (fc *FallbackClient) TxPoolStatus(ctx context.Context) (*TxPoolStatus, error) {
	var result *TxPoolStatus
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.TxPoolStatus(ctx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) TxPoolContent(ctx context.Context) (*TxPoolContent, error) {
	var result *TxPoolContent
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.TxPoolContent(ctx)
		return e
	})
	return result, err
}

func (fc *FallbackClient) Subscribe(ctx context.Context, eventType SubscriptionType, filters map[string]interface{}, resumeToken string) (Subscription, error) {
	// 订阅不支持故障转移,直接使用第一个WebSocket端点
	// 实际应该寻找有WebSocket的端点
	return nil, fmt.Errorf("subscription not supported in fallback mode, use dedicated WebSocket client")
}

func (fc *FallbackClient) GetBlockHeader(ctx context.Context, height uint64) (*BlockHeader, error) {
	var result *BlockHeader
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetBlockHeader(ctx, height)
		return e
	})
	return result, err
}

func (fc *FallbackClient) GetTxProof(ctx context.Context, txHash string) (*MerkleProof, error) {
	var result *MerkleProof
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.GetTxProof(ctx, txHash)
		return e
	})
	return result, err
}

func (fc *FallbackClient) Ping(ctx context.Context) error {
	return fc.tryWithFallback(ctx, func(c Client) error {
		return c.Ping(ctx)
	})
}

// SendTransaction 执行转账（通过当前可用客户端）
func (fc *FallbackClient) SendTransaction(ctx context.Context, fromAddress string, toAddress string, amount uint64, privateKey []byte) (*SendTxResult, error) {
	var result *SendTxResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var err error
		result, err = c.SendTransaction(ctx, fromAddress, toAddress, amount, privateKey)
		return err
	})
	return result, err
}

func (fc *FallbackClient) DeployContract(ctx context.Context, req *DeployContractRequest) (*DeployContractResult, error) {
	var result *DeployContractResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var err error
		result, err = c.DeployContract(ctx, req)
		return err
	})
	return result, err
}

func (fc *FallbackClient) CallContract(ctx context.Context, req *CallContractRequest) (*CallContractResult, error) {
	var result *CallContractResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var err error
		result, err = c.CallContract(ctx, req)
		return err
	})
	return result, err
}

func (fc *FallbackClient) GetContract(ctx context.Context, contentHash string) (*ContractMetadata, error) {
	var result *ContractMetadata
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var err error
		result, err = c.GetContract(ctx, contentHash)
		return err
	})
	return result, err
}

func (fc *FallbackClient) Close() error {
	var err error
	fc.closeOnce.Do(func() {
		close(fc.closeCh)

		fc.mu.Lock()
		defer fc.mu.Unlock()

		for _, c := range fc.clients {
			if e := c.client.Close(); e != nil && err == nil {
				err = e
			}
		}
	})
	return err
}

func (fc *FallbackClient) CallRaw(ctx context.Context, method string, params []interface{}) (interface{}, error) {
	var result interface{}
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.CallRaw(ctx, method, params)
		return e
	})
	return result, err
}

func (fc *FallbackClient) CallAIModel(ctx context.Context, req *CallAIModelRequest) (*CallAIModelResult, error) {
	var result *CallAIModelResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.CallAIModel(ctx, req)
		return e
	})
	return result, err
}

func (fc *FallbackClient) DeployAIModel(ctx context.Context, req *DeployAIModelRequest) (*DeployAIModelResult, error) {
	var result *DeployAIModelResult
	err := fc.tryWithFallback(ctx, func(c Client) error {
		var e error
		result, e = c.DeployAIModel(ctx, req)
		return e
	})
	return result, err
}

// 确保实现了Client接口
var _ Client = (*FallbackClient)(nil)
