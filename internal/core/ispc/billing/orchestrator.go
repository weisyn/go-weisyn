// Package billing 实现资源计费编排器
package billing

import (
	"context"
	"fmt"
	"math/big"

	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// BillingOrchestrator 计费编排器接口
//
// 🎯 **核心职责**：
// 根据资源哈希、CU 和选定的支付代币，生成计费计划（BillingPlan）。
//
// 💡 **设计理念**：
// - 查询定价状态：通过 PricingQuery 获取资源的定价策略
// - 计算费用：根据计费模式和 CU 计算实际费用
// - 生成计费计划：返回结构化的计费计划，供 TX Builder 使用
//
// 📞 **调用方**：
// - ISPC 执行协调器（执行完成后生成计费计划）
// - TX Builder（构建计费交易）
type BillingOrchestrator interface {
	// GenerateBillingPlan 生成计费计划
	//
	// 根据资源哈希、CU 和选定的支付代币，生成计费计划。
	// 参数：
	//   - ctx: 上下文
	//   - resourceHash: 资源内容哈希（32字节）
	//   - cu: 计算单元（Compute Units）
	//   - selectedToken: 选定的支付代币标识符（如果为空，使用第一个可用代币）
	//                    约束规则与 TokenID 一致：
	//                    - ""     表示原生代币
	//                    - 40hex 表示合约代币合约地址
	// 返回：
	//   - *BillingPlan: 计费计划对象
	//   - error: 生成失败的错误
	GenerateBillingPlan(
		ctx context.Context,
		resourceHash []byte,
		cu float64,
		selectedToken string,
	) (*BillingPlan, error)
}

// BillingPlan 计费计划
//
// 🎯 **用途**：
// 描述一次资源调用所需的费用和支付方式，供 TX Builder 使用。
type BillingPlan struct {
	ResourceHash []byte   // 资源内容哈希
	CU           float64  // 计算单元
	FeeAmount    *big.Int // 费用金额（最小单位，如 wei）
	PaymentToken string   // 支付代币标识符
	OwnerAddress []byte   // 资源所有者地址（费用接收方）
	BillingMode  types.BillingMode // 计费模式
}

// DefaultBillingOrchestrator 默认计费编排器实现
type DefaultBillingOrchestrator struct {
	pricingQuery persistence.PricingQuery
}

// NewDefaultBillingOrchestrator 创建默认计费编排器
func NewDefaultBillingOrchestrator(pricingQuery persistence.PricingQuery) BillingOrchestrator {
	if pricingQuery == nil {
		panic("pricingQuery cannot be nil")
	}
	return &DefaultBillingOrchestrator{
		pricingQuery: pricingQuery,
	}
}

// GenerateBillingPlan 生成计费计划
func (o *DefaultBillingOrchestrator) GenerateBillingPlan(
	ctx context.Context,
	resourceHash []byte,
	cu float64,
	selectedToken string,
) (*BillingPlan, error) {
	if len(resourceHash) != 32 {
		return nil, fmt.Errorf("资源哈希必须是 32 字节，实际: %d", len(resourceHash))
	}
	if cu < 0 {
		return nil, fmt.Errorf("CU 必须 >= 0，实际: %f", cu)
	}

	// 1. 查询定价状态
	pricingStateInterface, err := o.pricingQuery.GetPricingState(ctx, resourceHash)
	if err != nil {
		return nil, fmt.Errorf("查询定价状态失败: %w", err)
	}

	// pricingState 已经是 *types.ResourcePricingState 类型（接口返回具体类型）
	pricingState := pricingStateInterface

	// 2. 检查是否免费
	if pricingState.IsFree() {
		return &BillingPlan{
			ResourceHash: resourceHash,
			CU:           cu,
			FeeAmount:    big.NewInt(0),
			PaymentToken: "",
			OwnerAddress: pricingState.OwnerAddress,
			BillingMode:  pricingState.BillingMode,
		}, nil
	}

	// 3. 根据计费模式计算费用
	var feeAmount *big.Int
	var paymentToken string

	switch pricingState.BillingMode {
	case types.BillingModeFREE:
		// 免费模式（已在上一步处理，这里不应该到达）
		feeAmount = big.NewInt(0)
		paymentToken = ""

	case types.BillingModeFIXED:
		// 固定费用模式
		// Phase 2: 等 ResourcePricingState 暴露固定费用字段后再启用，这里暂时视为免费（fee=0）
		feeAmount = big.NewInt(0)
		paymentToken = "" // FIXED 模式暂不支持多代币（MVP）

	case types.BillingModeCUBASED:
		// CU 计费模式
		if len(pricingState.PaymentTokens) == 0 {
			return nil, fmt.Errorf("CU_BASED 模式必须至少配置一个支付代币")
		}

		// 选择支付代币（当前实现：定价状态层已经约束为仅 1 个）
		if selectedToken == "" {
			// 如果未指定，使用定价状态中配置的唯一 TokenID
			selectedToken = string(pricingState.PaymentTokens[0].TokenID)
		}

			// 获取 CU 单价（selectedToken 语义与 TokenID 一致）
			cuPrice, exists := pricingState.GetCUPrice(types.TokenID(selectedToken))
		if !exists {
			return nil, fmt.Errorf("支付代币 %s 未配置 CU 单价", selectedToken)
		}

		// 计算费用：fee = cu × cu_price
		// 注意：cu 是 float64，需要转换为 big.Float 进行精确计算
		cuBigFloat := big.NewFloat(cu)
		cuPriceBigFloat := new(big.Float).SetInt(cuPrice)
		feeBigFloat := new(big.Float).Mul(cuBigFloat, cuPriceBigFloat)

		// 转换为 big.Int（向下取整）
		feeAmount, _ = feeBigFloat.Int(nil)
		if feeAmount == nil {
			feeAmount = big.NewInt(0)
		}

		paymentToken = selectedToken

	default:
		return nil, fmt.Errorf("不支持的计费模式: %s", pricingState.BillingMode)
	}

	// 4. 构建并返回计费计划
	return &BillingPlan{
		ResourceHash: resourceHash,
		CU:           cu,
		FeeAmount:    feeAmount,
		PaymentToken: paymentToken,
		OwnerAddress: pricingState.OwnerAddress,
		BillingMode:  pricingState.BillingMode,
	}, nil
}

