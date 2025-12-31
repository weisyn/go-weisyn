package guides

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/client/core/mining"
	"github.com/weisyn/v1/client/core/transfer"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
)

// FirstTimeGuide 首次使用引导
//
// 迁移自 _archived/old-internal-cli/internal/cli/domain/guides/first_time_guide.go
// 对接新的 client/core 业务层
type FirstTimeGuide struct {
	transport       transport.Client
	walletManager   *wallet.AccountManager
	transferService *transfer.TransferService
	miningService   *mining.MiningService
}

// NewFirstTimeGuide 创建首次引导
func NewFirstTimeGuide(
	client transport.Client,
	walletMgr *wallet.AccountManager,
	transferSvc *transfer.TransferService,
	miningSvc *mining.MiningService,
) *FirstTimeGuide {
	return &FirstTimeGuide{
		transport:       client,
		walletManager:   walletMgr,
		transferService: transferSvc,
		miningService:   miningSvc,
	}
}

// GuideStep 引导步骤
type GuideStep struct {
	Title       string                      // 步骤标题
	Description string                      // 步骤描述
	Action      func(context.Context) error // 步骤操作
}

// RunGuide 运行首次引导流程
//
// 引导步骤：
//  1. 创建钱包
//  2. 查询余额（预期为0）
//  3. 启动挖矿（获取初始资金）
//  4. 等待并检查余额
//  5. 执行简单转账测试
func (fg *FirstTimeGuide) RunGuide(ctx context.Context) error {
	steps := fg.getGuideSteps()

	fmt.Println("=== WES 首次使用引导 ===")
	fmt.Printf("共%d个步骤，将引导您完成首次设置和基本操作\n\n", len(steps))

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s\n", i+1, len(steps), step.Title)
		fmt.Printf("    %s\n", step.Description)

		if step.Action != nil {
			if err := step.Action(ctx); err != nil {
				return fmt.Errorf("步骤失败: %w", err)
			}
		}

		fmt.Println()
	}

	fmt.Println("=== 引导完成！===")
	return nil
}

// getGuideSteps 获取引导步骤列表
func (fg *FirstTimeGuide) getGuideSteps() []GuideStep {
	return []GuideStep{
		{
			Title:       "创建钱包",
			Description: "为您生成一个新的钱包地址",
			Action:      fg.guideWalletCreation,
		},
		{
			Title:       "检查余额",
			Description: "查询钱包当前余额（预期为0）",
			Action:      fg.guideBalanceCheck,
		},
		{
			Title:       "启动挖矿",
			Description: "开始挖矿以获取初始资金",
			Action:      fg.guideMining,
		},
		{
			Title:       "等待挖矿奖励",
			Description: "等待挖矿产生区块并获得奖励",
			Action:      fg.guideWaitForBalance,
		},
		{
			Title:       "执行转账测试",
			Description: "尝试执行一笔简单转账",
			Action:      fg.guideTransfer,
		},
	}
}

// guideWalletCreation 引导：创建钱包
func (fg *FirstTimeGuide) guideWalletCreation(ctx context.Context) error {
	fmt.Println("    正在生成新钱包...")

	// 创建新账户
	account, err := fg.walletManager.CreateAccount("default", "password123")
	if err != nil {
		return fmt.Errorf("创建账户失败: %w", err)
	}

	fmt.Printf("    ✅ 钱包创建成功！\n")
	fmt.Printf("    地址: %s\n", account.Address)

	return nil
}

// guideBalanceCheck 引导：检查余额
func (fg *FirstTimeGuide) guideBalanceCheck(ctx context.Context) error {
	// 获取默认账户
	accounts, err := fg.walletManager.ListAccounts()
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("未找到钱包账户")
	}

	address := accounts[0].Address

	// 查询余额
	balance, err := fg.transferService.GetBalance(ctx, address)
	if err != nil {
		return fmt.Errorf("查询余额失败: %w", err)
	}

	fmt.Printf("    当前余额: %s WES\n", balance)

	return nil
}

// guideMining 引导：启动挖矿
func (fg *FirstTimeGuide) guideMining(ctx context.Context) error {
	// 获取默认账户地址
	accounts, err := fg.walletManager.ListAccounts()
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("未找到钱包账户")
	}

	address := accounts[0].Address

	// 启动挖矿
	fmt.Println("    正在启动挖矿...")
	result, err := fg.miningService.StartMining(ctx, &mining.StartMiningRequest{
		MinerAddress: address,
		Threads:      1,
	})
	if err != nil {
		return fmt.Errorf("启动挖矿失败: %w", err)
	}

	fmt.Printf("    ✅ %s\n", result.Message)

	return nil
}

// guideWaitForBalance 引导：等待余额
func (fg *FirstTimeGuide) guideWaitForBalance(ctx context.Context) error {
	fmt.Println("    请等待挖矿产生区块...")
	fmt.Println("    (在实际环境中，这可能需要几分钟)")
	fmt.Println("    (当前为演示模式，跳过等待)")

	return nil
}

// guideTransfer 引导：执行转账
func (fg *FirstTimeGuide) guideTransfer(ctx context.Context) error {
	fmt.Println("    准备执行转账测试...")
	fmt.Println("    (需要确认余额充足)")
	fmt.Println("    (当前为演示模式，跳过转账)")

	// 实际转账逻辑：
	// 1. 获取账户信息和私钥
	// 2. 构造转账请求
	// 3. 调用 transferService.ExecuteTransfer()
	// 4. 显示结果

	return nil
}
