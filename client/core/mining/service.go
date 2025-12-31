package mining

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/client/core/transport"
)

// MiningService 挖矿业务服务
// 等价于旧TX的MinerService，提供完整的挖矿控制业务逻辑
type MiningService struct {
	transport transport.Client
}

// NewMiningService 创建挖矿业务服务
func NewMiningService(client transport.Client) *MiningService {
	return &MiningService{
		transport: client,
	}
}

// MiningStatus 挖矿状态
type MiningStatus struct {
	IsMining       bool    `json:"is_mining"`       // 是否正在挖矿
	HashRate       float64 `json:"hash_rate"`       // 算力（H/s）
	MinerAddress   string  `json:"miner_address"`   // 矿工地址
	BlocksMined    uint64  `json:"blocks_mined"`    // 已挖出的区块数
	CurrentHeight  uint64  `json:"current_height"`  // 当前区块高度
	Difficulty     uint64  `json:"difficulty"`      // 当前难度
	PendingRewards string  `json:"pending_rewards"` // 待领取奖励
}

// StartMiningRequest 启动挖矿请求
type StartMiningRequest struct {
	MinerAddress string // 矿工地址（接收挖矿奖励的地址）
	Threads      int    // 挖矿线程数（可选，默认为CPU核心数）
}

// StartMiningResult 启动挖矿结果
type StartMiningResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	MinerAddress string `json:"miner_address"`
	Threads      int    `json:"threads"`
}

// StopMiningResult 停止挖矿结果
type StopMiningResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	BlocksMined  uint64 `json:"blocks_mined"`  // 本次挖矿期间挖出的区块数
	TotalRewards string `json:"total_rewards"` // 本次挖矿期间获得的总奖励
}

// GetMiningStatus 获取挖矿状态
//
// 等价于旧CLI的MiningCommands.GetMiningStatus()
func (s *MiningService) GetMiningStatus(ctx context.Context) (*MiningStatus, error) {
	if s.transport == nil {
		return nil, fmt.Errorf("transport client is nil")
	}

	// ✅ V2：挖矿状态以 JSON-RPC 为准
	// wes_getMiningStatus -> {is_running, miner_address}
	raw, err := s.transport.CallRaw(ctx, "wes_getMiningStatus", []interface{}{})
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(raw)
	var resp struct {
		IsRunning    bool   `json:"is_running"`
		MinerAddress string `json:"miner_address"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal wes_getMiningStatus response: %w", err)
	}

	// 尽力补齐高度信息（方便观察“已同步到哪里”）
	var height uint64
	if h, herr := s.transport.BlockNumber(ctx); herr == nil {
		height = h
	}

	return &MiningStatus{
		IsMining:       resp.IsRunning,
		HashRate:       0, // 当前 JSON-RPC 未暴露实时算力指标
		MinerAddress:   resp.MinerAddress,
		BlocksMined:    0,   // 当前 JSON-RPC 未暴露
		CurrentHeight:  height,
		Difficulty:     0,   // 当前 JSON-RPC 未暴露
		PendingRewards: "0", // 如需精确奖励，需后续接入专用接口
	}, nil
}

// StartMining 启动挖矿
//
// 等价于旧CLI的MiningCommands.StartMining()
//
// 流程：
//  1. 验证矿工地址
//  2. 调用节点API启动挖矿
//  3. 返回启动结果
func (s *MiningService) StartMining(ctx context.Context, req *StartMiningRequest) (*StartMiningResult, error) {
	if err := s.validateStartRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if s.transport == nil {
		return nil, fmt.Errorf("transport client is nil")
	}

	// ✅ V2：挖矿开启必须走 wes_startMining（矿工地址 + 门闸检查）
	raw, err := s.transport.CallRaw(ctx, "wes_startMining", []interface{}{req.MinerAddress})
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(raw)
	var resp struct {
		Status       string `json:"status"`
		Message      string `json:"message"`
		MinerAddress string `json:"miner_address"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal wes_startMining response: %w", err)
	}
	if resp.Status != "" && resp.Status != "success" {
		if resp.Message != "" {
			return nil, fmt.Errorf(resp.Message)
		}
		return nil, fmt.Errorf("start mining failed")
	}

	threads := req.Threads
	if threads <= 0 {
		threads = 1 // UI 展示用；当前 JSON-RPC 不支持传线程数
	}

	return &StartMiningResult{
		Success:      true,
		Message:      "挖矿已启动",
		MinerAddress: firstNonEmpty(resp.MinerAddress, req.MinerAddress),
		Threads:      threads,
	}, nil
}

// StopMining 停止挖矿
//
// 等价于旧CLI的MiningCommands.StopMining()
//
// 流程：
//  1. 检查当前是否在挖矿
//  2. 调用节点API停止挖矿
//  3. 统计本次挖矿数据
//  4. 返回结果
func (s *MiningService) StopMining(ctx context.Context) (*StopMiningResult, error) {
	if s.transport == nil {
		return nil, fmt.Errorf("transport client is nil")
	}

	raw, err := s.transport.CallRaw(ctx, "wes_stopMining", []interface{}{})
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(raw)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal wes_stopMining response: %w", err)
	}
	if resp.Status != "" && resp.Status != "success" {
		if resp.Message != "" {
			return nil, fmt.Errorf(resp.Message)
		}
		return nil, fmt.Errorf("stop mining failed")
	}

	msg := resp.Message
	if msg == "" {
		msg = "挖矿已停止"
	}

	return &StopMiningResult{
		Success:      true,
		Message:      msg,
		BlocksMined:  0,
		TotalRewards: "0",
	}, nil
}

// GetHashRate 获取当前算力
func (s *MiningService) GetHashRate(ctx context.Context) (float64, error) {
	status, err := s.GetMiningStatus(ctx)
	if err != nil {
		return 0, err
	}

	return status.HashRate, nil
}

// GetMinerAddress 获取当前矿工地址
func (s *MiningService) GetMinerAddress(ctx context.Context) (string, error) {
	status, err := s.GetMiningStatus(ctx)
	if err != nil {
		return "", err
	}

	if !status.IsMining {
		return "", fmt.Errorf("not mining")
	}

	return status.MinerAddress, nil
}

// validateStartRequest 验证启动挖矿请求
func (s *MiningService) validateStartRequest(req *StartMiningRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.MinerAddress == "" {
		return fmt.Errorf("miner address is empty")
	}

	// TODO: 验证地址格式

	if req.Threads < 0 {
		return fmt.Errorf("invalid threads: %d", req.Threads)
	}

	return nil
}

// SetMinerAddress 设置矿工地址（用于后续挖矿）
// 这是一个便捷方法，实际应该通过节点配置管理
func (s *MiningService) SetMinerAddress(ctx context.Context, address string) error {
	if address == "" {
		return fmt.Errorf("address is empty")
	}

	// TODO: 调用节点API设置矿工地址
	// err := s.transport.SetMinerAddress(ctx, address)
	// if err != nil {
	// 	return fmt.Errorf("set miner address: %w", err)
	// }

	return nil
}

// GetPendingRewards 查询待领取的挖矿奖励
func (s *MiningService) GetPendingRewards(ctx context.Context, address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("address is empty")
	}

	if s.transport == nil {
		return "", fmt.Errorf("transport client is nil")
	}

	// 当前节点侧未暴露“挖矿奖励明细/待领取奖励”专用接口，这里先返回余额作为近似展示，避免误导为固定 0。
	bal, err := s.transport.GetBalance(ctx, address, nil)
	if err != nil {
		return "", err
	}
	if bal == nil {
		return "0", nil
	}
	return bal.Balance, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
