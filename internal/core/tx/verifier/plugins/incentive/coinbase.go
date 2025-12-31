// Package incentive 提供激励交易验证插件
//
// 本包实现Coinbase和赞助领取交易的验证逻辑，
// 集成到TX State Machine的验证流程中。
package incentive

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/tx/ports/fee"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// CoinbasePlugin Coinbase交易验证插件
//
// 🎯 **零增发Coinbase验证**
//
// 集成到TX验证流程，识别并验证Coinbase交易。
//
// 验证内容：
//  1. 识别Coinbase（无输入）
//  2. 验证所有输出Owner = minerAddr
//  3. 验证费用守恒（Coinbase输出 == 期望费用）
//  4. 验证无增发（无额外Token）
type CoinbasePlugin struct {
	feeManager        txiface.FeeManager
	coinbaseValidator *fee.CoinbaseValidator
}

// NewCoinbasePlugin 创建Coinbase验证插件
func NewCoinbasePlugin(feeManager txiface.FeeManager) *CoinbasePlugin {
	return &CoinbasePlugin{
		feeManager:        feeManager,
		coinbaseValidator: fee.NewCoinbaseValidator(),
	}
}

// Name 插件名称
func (p *CoinbasePlugin) Name() string {
	return "CoinbaseValidator"
}

// Verify 验证交易（插件入口）
//
// 识别Coinbase交易并验证费用守恒。
// 非Coinbase交易跳过。
//
// 参数：
//
//	ctx: 上下文
//	tx: 待验证的交易
//	env: 验证环境（必须实现txiface.VerifierEnvironment）
//
// 返回：
//
//	error: 验证失败原因，nil表示通过
func (p *CoinbasePlugin) Verify(
	ctx context.Context,
	tx *transaction_pb.Transaction,
	env interface{},
) error {
	// 1. 识别Coinbase（无输入）
	if len(tx.Inputs) != 0 {
		return nil // 非Coinbase，跳过
	}

	// 2. 类型断言获取验证环境
	verifierEnv, ok := env.(txiface.VerifierEnvironment)
	if !ok {
		return fmt.Errorf("CoinbasePlugin: 环境类型错误，期望txiface.VerifierEnvironment")
	}

	// 3. 从环境获取必要信息
	expectedFees := verifierEnv.GetExpectedFees()
	minerAddr := verifierEnv.GetMinerAddress()

	if expectedFees == nil {
		return fmt.Errorf("CoinbasePlugin: 期望费用为nil")
	}
	if len(minerAddr) != 20 {
		return fmt.Errorf("CoinbasePlugin: 矿工地址长度必须为20字节，实际=%d", len(minerAddr))
	}

	// 4. 验证Coinbase费用守恒
	if err := p.coinbaseValidator.Validate(ctx, tx, expectedFees, minerAddr); err != nil {
		return fmt.Errorf("CoinbasePlugin: 验证失败: %w", err)
	}

	return nil
}
