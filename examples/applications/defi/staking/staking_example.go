// Package main 展示如何与WES系统质押合约交互
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"
)

/*
🎯 DeFi质押应用示例

这是一个简单的示例，展示如何：
1. 连接到WES网络
2. 与系统质押合约交互
3. 执行基础的质押操作

💡 学习重点：
- 如何构建交易参数
- 如何调用系统合约
- 如何处理返回结果
- 如何查询链上状态
*/

// StakingClient 简单的质押客户端
type StakingClient struct {
	contractAddress string // 系统质押合约地址
	userAddress     string // 用户地址
}

// StakingInfo 质押信息
type StakingInfo struct {
	StakeID    string   `json:"stake_id"`    // 质押ID
	Amount     *big.Int `json:"amount"`      // 质押金额
	StartTime  int64    `json:"start_time"`  // 开始时间
	LockPeriod uint64   `json:"lock_period"` // 锁定期（秒）
	Rewards    *big.Int `json:"rewards"`     // 当前奖励
	Status     string   `json:"status"`      // 状态
}

// NewStakingClient 创建质押客户端
func NewStakingClient(contractAddr, userAddr string) *StakingClient {
	return &StakingClient{
		contractAddress: contractAddr,
		userAddress:     userAddr,
	}
}

// Stake 执行质押操作
func (c *StakingClient) Stake(amount *big.Int, lockPeriod uint64) (string, error) {
	fmt.Printf("🔄 正在质押 %s 代币，锁定期 %d 秒...\n", amount.String(), lockPeriod)

	// 📋 步骤1：构建合约调用参数
	params := map[string]interface{}{
		"amount":      amount.String(),
		"lock_period": lockPeriod,
		"stake_type":  "fixed", // 固定期限质押
	}

	// 📦 步骤2：序列化参数
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %v", err)
	}

	// 🚀 步骤3：调用系统质押合约
	// 在真实实现中，这里会调用区块链交易
	txHash, err := c.callContract("stake", paramsBytes)
	if err != nil {
		return "", fmt.Errorf("调用合约失败: %v", err)
	}

	fmt.Printf("✅ 质押成功！交易哈希: %s\n", txHash)
	return txHash, nil
}

// Unstake 取消质押
func (c *StakingClient) Unstake(stakeID string) (string, error) {
	fmt.Printf("🔄 正在取消质押 %s...\n", stakeID)

	// 📋 步骤1：构建取消质押参数
	params := map[string]interface{}{
		"stake_id": stakeID,
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %v", err)
	}

	// 🚀 步骤2：调用取消质押
	txHash, err := c.callContract("unstake", paramsBytes)
	if err != nil {
		return "", fmt.Errorf("取消质押失败: %v", err)
	}

	fmt.Printf("✅ 取消质押成功！交易哈希: %s\n", txHash)
	return txHash, nil
}

// ClaimRewards 领取奖励
func (c *StakingClient) ClaimRewards(stakeID string) (*big.Int, error) {
	fmt.Printf("🔄 正在领取质押奖励 %s...\n", stakeID)

	// 📋 步骤1：构建领取参数
	params := map[string]interface{}{
		"stake_id": stakeID,
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化参数失败: %v", err)
	}

	// 🚀 步骤2：调用领取奖励
	result, err := c.queryContract("claim_rewards", paramsBytes)
	if err != nil {
		return nil, fmt.Errorf("领取奖励失败: %v", err)
	}

	// 📊 步骤3：解析奖励金额
	var response map[string]interface{}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	rewardsStr, ok := response["rewards"].(string)
	if !ok {
		return nil, fmt.Errorf("奖励格式错误")
	}

	rewards := new(big.Int)
	rewards.SetString(rewardsStr, 10)

	fmt.Printf("✅ 成功领取奖励: %s\n", rewards.String())
	return rewards, nil
}

// GetStakingInfo 查询质押信息
func (c *StakingClient) GetStakingInfo(stakeID string) (*StakingInfo, error) {
	fmt.Printf("🔍 正在查询质押信息 %s...\n", stakeID)

	// 📋 步骤1：构建查询参数
	params := map[string]interface{}{
		"stake_id": stakeID,
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化参数失败: %v", err)
	}

	// 🔍 步骤2：查询合约状态
	result, err := c.queryContract("get_stake_info", paramsBytes)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}

	// 📊 步骤3：解析质押信息
	var info StakingInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("解析质押信息失败: %v", err)
	}

	fmt.Printf("📊 质押信息: 金额=%s, 状态=%s\n", info.Amount.String(), info.Status)
	return &info, nil
}

// GetTotalStaked 查询用户总质押金额
func (c *StakingClient) GetTotalStaked() (*big.Int, error) {
	fmt.Printf("🔍 正在查询用户总质押金额...\n")

	// 📋 步骤1：构建查询参数
	params := map[string]interface{}{
		"user_address": c.userAddress,
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化参数失败: %v", err)
	}

	// 🔍 步骤2：查询总质押
	result, err := c.queryContract("get_user_total_staked", paramsBytes)
	if err != nil {
		return nil, fmt.Errorf("查询总质押失败: %v", err)
	}

	// 📊 步骤3：解析结果
	var response map[string]interface{}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	totalStr, ok := response["total_staked"].(string)
	if !ok {
		return nil, fmt.Errorf("总质押格式错误")
	}

	total := new(big.Int)
	total.SetString(totalStr, 10)

	fmt.Printf("📊 用户总质押: %s\n", total.String())
	return total, nil
}

// 私有方法：调用合约（写操作）
func (c *StakingClient) callContract(method string, params []byte) (string, error) {
	// 🔧 在真实实现中，这里会：
	// 1. 构建交易参数
	// 2. 签名交易
	// 3. 发送到WES网络
	// 4. 等待交易确认
	// 5. 返回交易哈希

	// 💡 模拟实现
	fmt.Printf("📤 调用合约方法: %s\n", method)
	fmt.Printf("📝 参数: %s\n", string(params))

	// 生成模拟交易哈希
	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())

	// 模拟网络延迟
	time.Sleep(100 * time.Millisecond)

	return txHash, nil
}

// 私有方法：查询合约（读操作）
func (c *StakingClient) queryContract(method string, params []byte) ([]byte, error) {
	// 🔧 在真实实现中，这里会：
	// 1. 构建查询请求
	// 2. 发送到WES网络
	// 3. 获取链上状态
	// 4. 返回查询结果

	// 💡 模拟实现
	fmt.Printf("🔍 查询合约方法: %s\n", method)
	fmt.Printf("📝 参数: %s\n", string(params))

	// 模拟返回数据
	switch method {
	case "get_stake_info":
		info := StakingInfo{
			StakeID:    "stake_123",
			Amount:     big.NewInt(1000000), // 1,000,000 代币
			StartTime:  time.Now().Unix(),
			LockPeriod: 30 * 24 * 3600,    // 30天
			Rewards:    big.NewInt(50000), // 50,000 奖励
			Status:     "active",
		}
		return json.Marshal(info)

	case "claim_rewards":
		response := map[string]interface{}{
			"rewards": "50000",
			"status":  "success",
		}
		return json.Marshal(response)

	case "get_user_total_staked":
		response := map[string]interface{}{
			"total_staked":  "5000000", // 5,000,000 代币
			"active_stakes": 3,
		}
		return json.Marshal(response)

	default:
		return nil, fmt.Errorf("未知查询方法: %s", method)
	}
}

// 演示函数：完整的质押流程
func DemoStakingFlow() {
	fmt.Println("🎮 WES质押应用演示")
	fmt.Println("===================")
	fmt.Println()

	// 🏗️ 步骤1：创建客户端
	fmt.Println("📱 1. 初始化质押客户端...")
	client := NewStakingClient(
		"0x1234567890abcdef1234567890abcdef12345678", // 系统质押合约地址
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd", // 用户地址
	)
	fmt.Println()

	// 💰 步骤2：查询当前总质押
	fmt.Println("💰 2. 查询当前总质押...")
	totalStaked, err := client.GetTotalStaked()
	if err != nil {
		log.Printf("查询总质押失败: %v", err)
	} else {
		fmt.Printf("当前总质押: %s 代币\n", totalStaked.String())
	}
	fmt.Println()

	// 🔒 步骤3：执行新质押
	fmt.Println("🔒 3. 执行新质押...")
	stakeAmount := big.NewInt(1000000)   // 1,000,000 代币
	lockPeriod := uint64(30 * 24 * 3600) // 30天

	txHash, err := client.Stake(stakeAmount, lockPeriod)
	if err != nil {
		log.Printf("质押失败: %v", err)
		return
	}
	fmt.Printf("质押交易哈希: %s\n", txHash)
	fmt.Println()

	// 📊 步骤4：查询质押信息
	fmt.Println("📊 4. 查询质押信息...")
	stakeID := "stake_123" // 在真实场景中，这个ID会从质押交易返回

	info, err := client.GetStakingInfo(stakeID)
	if err != nil {
		log.Printf("查询质押信息失败: %v", err)
	} else {
		fmt.Printf("质押详情:\n")
		fmt.Printf("  - 质押ID: %s\n", info.StakeID)
		fmt.Printf("  - 金额: %s 代币\n", info.Amount.String())
		fmt.Printf("  - 状态: %s\n", info.Status)
		fmt.Printf("  - 当前奖励: %s 代币\n", info.Rewards.String())
	}
	fmt.Println()

	// 🎁 步骤5：领取奖励
	fmt.Println("🎁 5. 领取质押奖励...")
	rewards, err := client.ClaimRewards(stakeID)
	if err != nil {
		log.Printf("领取奖励失败: %v", err)
	} else {
		fmt.Printf("成功领取奖励: %s 代币\n", rewards.String())
	}
	fmt.Println()

	// 💰 步骤6：查询更新后的总质押
	fmt.Println("💰 6. 查询更新后的总质押...")
	newTotalStaked, err := client.GetTotalStaked()
	if err != nil {
		log.Printf("查询总质押失败: %v", err)
	} else {
		fmt.Printf("更新后总质押: %s 代币\n", newTotalStaked.String())
	}
	fmt.Println()

	fmt.Println("✅ 演示完成！")
	fmt.Println()
	fmt.Println("💡 说明:")
	fmt.Println("  - 本示例展示了与WES系统质押合约的基础交互")
	fmt.Println("  - 在真实环境中，需要连接到实际的WES网络")
	fmt.Println("  - 所有操作都会在区块链上留下不可篡改的记录")
}

func main() {
	DemoStakingFlow()
}
