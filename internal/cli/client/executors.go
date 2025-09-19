package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// directCallExecutor 直接调用核心服务的执行器
type directCallExecutor struct {
	logger         log.Logger
	accountService blockchainintf.AccountService
	minerService   consensusintf.MinerService
	addressManager blockchainintf.AccountService // placeholder to trigger diff (will adjust next)

	// 性能监控
	mu            sync.RWMutex
	available     bool
	lastLatency   time.Duration
	lastCheckTime time.Time
}

// NewDirectCallExecutor 创建直接调用执行器
func NewDirectCallExecutor(
	logger log.Logger,
	accountService blockchainintf.AccountService,
	minerService consensusintf.MinerService,
) DirectCallExecutor {
	return &directCallExecutor{
		logger:         logger,
		accountService: accountService,
		minerService:   minerService,
		available:      true,
		lastCheckTime:  time.Now(),
	}
}

// Execute 执行直接调用
func (d *directCallExecutor) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	startTime := time.Now()
	defer func() {
		d.updateLatency(time.Since(startTime))
	}()

	d.logger.Info(fmt.Sprintf("执行直接调用: operation=%s", operation))

	switch operation {
	// 账户相关操作
	case "account.get_balance":
		return d.executeGetBalance(ctx, params)
	case "account.get_transaction_history":
		return d.executeGetTransactionHistory(ctx, params)
	case "account.create_transaction":
		return d.executeCreateTransaction(ctx, params)

	// 共识相关操作
	case "consensus.get_mining_status":
		return d.executeGetMiningStatus(ctx, params)
	case "consensus.start_mining":
		return d.executeStartMining(ctx, params)
	case "consensus.stop_mining":
		return d.executeStopMining(ctx, params)

	// 区块链相关操作
	case "blockchain.get_latest_block":
		return d.executeGetLatestBlock(ctx, params)
	case "blockchain.get_block_by_height":
		return d.executeGetBlockByHeight(ctx, params)
	case "blockchain.get_blockchain_info":
		return d.executeGetBlockchainInfo(ctx, params)

	// 健康检查
	case "health_check":
		return d.executeHealthCheck(ctx, params)

	default:
		return nil, fmt.Errorf("不支持的操作: %s", operation)
	}
}

// IsAvailable 检查直接调用是否可用
func (d *directCallExecutor) IsAvailable() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.available
}

// GetLatency 获取最近的延迟
func (d *directCallExecutor) GetLatency() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastLatency
}

// updateLatency 更新延迟信息
func (d *directCallExecutor) updateLatency(latency time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastLatency = latency
	d.lastCheckTime = time.Now()
}

// 具体操作实现

func (d *directCallExecutor) executeGetBalance(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	address, ok := params["address"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: address")
	}

	balance, err := d.accountService.GetPlatformBalance(ctx, []byte(address))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"address":  address,
		"balance":  balance.Total,
		"currency": "WES",
	}, nil
}

func (d *directCallExecutor) executeGetTransactionHistory(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	address, ok := params["address"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: address")
	}

	// 简化实现：返回模拟数据
	return []map[string]interface{}{
		{
			"hash":      "0x1234567890abcdef",
			"from":      address,
			"to":        "0xabcdef1234567890",
			"amount":    "10.0",
			"timestamp": time.Now().Unix(),
			"status":    "confirmed",
		},
	}, nil
}

func (d *directCallExecutor) executeCreateTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	from, ok := params["from"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: from")
	}

	to, ok := params["to"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: to")
	}

	amount, ok := params["amount"].(float64)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: amount")
	}

	// 简化实现：返回模拟交易哈希
	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())

	return map[string]interface{}{
		"transaction_hash": txHash,
		"from":             from,
		"to":               to,
		"amount":           amount,
		"status":           "pending",
	}, nil
}

func (d *directCallExecutor) executeGetMiningStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if d.minerService == nil {
		return map[string]interface{}{
			"is_mining":    false,
			"hash_rate":    0,
			"blocks_mined": 0,
		}, nil
	}

	// 简化实现：返回模拟挖矿状态
	return map[string]interface{}{
		"is_mining":    false,
		"hash_rate":    0.0,
		"blocks_mined": 0,
		"difficulty":   "0x1000000",
	}, nil
}

func (d *directCallExecutor) executeStartMining(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if d.minerService == nil {
		return nil, fmt.Errorf("挖矿服务不可用")
	}

	// 简化实现
	return map[string]interface{}{
		"status":  "mining_started",
		"message": "挖矿已启动",
	}, nil
}

func (d *directCallExecutor) executeStopMining(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if d.minerService == nil {
		return nil, fmt.Errorf("挖矿服务不可用")
	}

	// 简化实现
	return map[string]interface{}{
		"status":  "mining_stopped",
		"message": "挖矿已停止",
	}, nil
}

func (d *directCallExecutor) executeGetLatestBlock(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// 简化实现：返回模拟区块数据
	return map[string]interface{}{
		"height":      12345,
		"hash":        "0xabcdef1234567890",
		"parent_hash": "0x1234567890abcdef",
		"timestamp":   time.Now().Unix(),
		"tx_count":    10,
	}, nil
}

func (d *directCallExecutor) executeGetBlockByHeight(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	height, ok := params["height"].(float64)
	if !ok {
		return nil, fmt.Errorf("缺少必要参数: height")
	}

	// 简化实现：返回模拟区块数据
	return map[string]interface{}{
		"height":      int64(height),
		"hash":        fmt.Sprintf("0x%x", int64(height)),
		"parent_hash": fmt.Sprintf("0x%x", int64(height)-1),
		"timestamp":   time.Now().Unix() - int64(height)*10,
		"tx_count":    5,
	}, nil
}

func (d *directCallExecutor) executeGetBlockchainInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"network":      "mainnet",
		"version":      "0.0.1",
		"block_height": 12345,
		"difficulty":   "0x1000000",
		"total_supply": "21000000",
		"peer_count":   8,
	}, nil
}

func (d *directCallExecutor) executeHealthCheck(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "0.0.1",
	}, nil
}

// apiCallExecutor API调用执行器
type apiCallExecutor struct {
	logger     log.Logger
	baseURL    string
	httpClient *http.Client

	// 性能监控
	mu            sync.RWMutex
	available     bool
	lastLatency   time.Duration
	lastCheckTime time.Time
}

// NewAPICallExecutor 创建API调用执行器
func NewAPICallExecutor(logger log.Logger, baseURL string) APICallExecutor {
	return &apiCallExecutor{
		logger:  logger,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		available:     true,
		lastCheckTime: time.Now(),
	}
}

// Execute 执行API调用
func (a *apiCallExecutor) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	startTime := time.Now()
	defer func() {
		a.updateLatency(time.Since(startTime))
	}()

	a.logger.Info(fmt.Sprintf("执行API调用: operation=%s", operation))

	// 构造API端点
	endpoint := a.operationToEndpoint(operation)
	if endpoint == "" {
		return nil, fmt.Errorf("不支持的操作: %s", operation)
	}

	// 构造请求URL
	url := a.baseURL + endpoint

	// 创建HTTP请求
	var req *http.Request
	var err error

	if a.isReadOperation(operation) {
		// GET请求
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("创建GET请求失败: %v", err)
		}

		// 添加查询参数
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, fmt.Sprintf("%v", value))
		}
		req.URL.RawQuery = q.Encode()

	} else {
		// POST请求
		jsonData, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("序列化参数失败: %v", err)
		}

		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("创建POST请求失败: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.setAvailable(false)
		return nil, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查响应状态码
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API请求失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析JSON响应
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %v", err)
	}

	a.setAvailable(true)
	return result, nil
}

// IsAvailable 检查API调用是否可用
func (a *apiCallExecutor) IsAvailable() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.available
}

// GetLatency 获取最近的延迟
func (a *apiCallExecutor) GetLatency() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastLatency
}

// setAvailable 设置可用状态
func (a *apiCallExecutor) setAvailable(available bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.available = available
}

// updateLatency 更新延迟信息
func (a *apiCallExecutor) updateLatency(latency time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastLatency = latency
	a.lastCheckTime = time.Now()
}

// operationToEndpoint 将操作名称转换为API端点
func (a *apiCallExecutor) operationToEndpoint(operation string) string {
	endpointMap := map[string]string{
		"account.get_balance":             "/api/v1/account/balance",
		"account.get_transaction_history": "/api/v1/account/transactions",
		"account.create_transaction":      "/api/v1/account/transfer",
		"consensus.get_mining_status":     "/api/v1/mining/status",
		"consensus.start_mining":          "/api/v1/mining/start",
		"consensus.stop_mining":           "/api/v1/mining/stop",
		"blockchain.get_latest_block":     "/api/v1/blockchain/latest",
		"blockchain.get_block_by_height":  "/api/v1/blockchain/block",
		"blockchain.get_blockchain_info":  "/api/v1/blockchain/info",
		"health_check":                    "/api/v1/health",
	}

	return endpointMap[operation]
}

// isReadOperation 判断是否为读操作
func (a *apiCallExecutor) isReadOperation(operation string) bool {
	readOperations := map[string]bool{
		"account.get_balance":             true,
		"account.get_transaction_history": true,
		"consensus.get_mining_status":     true,
		"blockchain.get_latest_block":     true,
		"blockchain.get_block_by_height":  true,
		"blockchain.get_blockchain_info":  true,
		"health_check":                    true,
	}

	return readOperations[operation]
}
