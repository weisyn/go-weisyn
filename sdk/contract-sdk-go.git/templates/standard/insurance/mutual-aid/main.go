//go:build tinygo || (js && wasm)

// Package main 提供互助险（类似相互宝）业务的示例合约。
//
// 设计目标：
// - 用最少的状态 & 事件，演示一个互助计划的核心流程
// - 展示如何在 WES 上把「事后分摊 + 定额给付」抽象成合约接口
// - 方便开发者按需扩展为更完整的业务（成员状态、资金池、黑名单等）
//
// ⚠️ 注意：本合约是「示例模板」，很多复杂逻辑仅通过事件记录，未做完整的链上状态管理，
// 实际生产环境应结合 StateOutput、事件回放以及独立的治理/风控模块来实现。
package main

import (
	"github.com/weisyn/contract-sdk-go/framework"
	"github.com/weisyn/contract-sdk-go/helpers/market"
)

// MutualAidContract 互助险合约
//
// 合约采用「轻状态+重事件」的设计：
//   - Initialize 记录计划初始化事件
//   - Join / SubmitClaim / ReviewClaim / SettleRound / PayContribution / Payout
//     主要通过事件记录关键业务动作，便于链下服务回放与风控
type MutualAidContract struct {
	framework.ContractBase
}

// ================================================================================================
// 导出方法（Host ABI v1.1 规范）
// ================================================================================================

// Initialize 初始化互助计划
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "name": "相互宝互助计划",
//	  "token_id": "",                 // 计价代币ID，空字符串表示原生币
//	  "coverage_amount": 300000,      // 单次给付额
//	  "service_fee_bp": 800,          // 服务费率，单位 bp（万分比），如 800 = 8%
//	  "settlement_period": 2592000    // 结算周期（秒），例如 30 天
//	}
//
// 输出：
// - Event: MutualAidPlanInitialized
//
//export Initialize
func Initialize() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	name := params.ParseJSON("name")
	tokenID := params.ParseJSON("token_id")
	coverageAmount := params.ParseJSONInt("coverage_amount")
	serviceFeeBP := params.ParseJSONInt("service_fee_bp")
	settlementPeriod := params.ParseJSONInt("settlement_period")

	if planID == "" || name == "" || coverageAmount <= 0 || settlementPeriod <= 0 {
		return framework.ERROR_INVALID_PARAMS
	}

	caller := framework.GetCaller()

	event := framework.NewEvent("MutualAidPlanInitialized")
	event.AddStringField("plan_id", planID)
	event.AddStringField("name", name)
	event.AddStringField("token_id", tokenID)
	event.AddIntField("coverage_amount", coverageAmount)
	event.AddIntField("service_fee_bp", serviceFeeBP)
	event.AddIntField("settlement_period", settlementPeriod)
	event.AddAddressField("operator", caller)

	framework.EmitEvent(event)
	return framework.SUCCESS
}

// Join 成为互助计划成员
//
// 设计思路：
// - 示例中不做复杂的健康告知、等待期等校验，仅记录事件
// - 实际业务可在合约中增加白名单、黑名单与等待期检查
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001"
//	}
//
// 输出：
// - Event: MutualAidMemberJoined
//
//export Join
func Join() uint32 {
	params := framework.GetContractParams()
	planID := params.ParseJSON("plan_id")
	if planID == "" {
		return framework.ERROR_INVALID_PARAMS
	}

	caller := framework.GetCaller()

	event := framework.NewEvent("MutualAidMemberJoined")
	event.AddStringField("plan_id", planID)
	event.AddAddressField("member", caller)

	framework.EmitEvent(event)
	return framework.SUCCESS
}

// SubmitClaim 提交互助申请（报案）
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "claim_id": "claim_202501_0001",
//	  "insured": "Cf1...",                // 被保人地址（Base58），可为空表示即为调用者
//	  "requested_amount": 300000,
//	  "event_time": 1736200000,           // 出险时间（时间戳）
//	  "evidence_hash": "0xabc...",        // 资料哈希
//	  "extra": "optional comments"
//	}
//
// 输出：
// - Event: MutualAidClaimSubmitted
//
//export SubmitClaim
func SubmitClaim() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	claimID := params.ParseJSON("claim_id")
	insuredStr := params.ParseJSON("insured")
	requestedAmount := params.ParseJSONInt("requested_amount")
	eventTime := params.ParseJSONInt("event_time")
	evidenceHash := params.ParseJSON("evidence_hash")
	extra := params.ParseJSON("extra")

	if planID == "" || claimID == "" || requestedAmount <= 0 || eventTime <= 0 {
		return framework.ERROR_INVALID_PARAMS
	}

	applicant := framework.GetCaller()

	event := framework.NewEvent("MutualAidClaimSubmitted")
	event.AddStringField("plan_id", planID)
	event.AddStringField("claim_id", claimID)
	event.AddAddressField("applicant", applicant)
	if insuredStr != "" {
		insured, err := framework.ParseAddressBase58(insuredStr)
		if err == nil {
			event.AddAddressField("insured", insured)
		} else {
			event.AddStringField("insured_raw", insuredStr)
		}
	}
	event.AddIntField("requested_amount", requestedAmount)
	event.AddIntField("event_time", eventTime)
	event.AddStringField("evidence_hash", evidenceHash)
	event.AddStringField("extra", extra)

	framework.EmitEvent(event)
	return framework.SUCCESS
}

// ReviewClaim 审核互助申请（简化版）
//
// 说明：
// - 这里不实现完整的 DAO/投票逻辑，仅记录一次「审核决策」事件
// - 实际生产可：
//   - 由独立 DAO 合约对 claim 提案投票，结果由 off-chain 服务写回
//   - 或在本合约中直接实现委员会/全体成员投票逻辑
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "claim_id": "claim_202501_0001",
//	  "decision": "APPROVE",              // APPROVE / REJECT
//	  "approved_amount": 280000,          // 决定给付金额，REJECT 时可为 0
//	  "reason": "符合互助规则",
//	  "review_round_id": "round_202501_01"
//	}
//
// 输出：
// - Event: MutualAidClaimReviewed
//
//export ReviewClaim
func ReviewClaim() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	claimID := params.ParseJSON("claim_id")
	decision := params.ParseJSON("decision")
	approvedAmount := params.ParseJSONInt("approved_amount")
	reason := params.ParseJSON("reason")
	reviewRoundID := params.ParseJSON("review_round_id")

	if planID == "" || claimID == "" || decision == "" {
		return framework.ERROR_INVALID_PARAMS
	}

	reviewer := framework.GetCaller()

	event := framework.NewEvent("MutualAidClaimReviewed")
	event.AddStringField("plan_id", planID)
	event.AddStringField("claim_id", claimID)
	event.AddStringField("decision", decision)
	event.AddIntField("approved_amount", approvedAmount)
	event.AddStringField("reason", reason)
	event.AddStringField("review_round_id", reviewRoundID)
	event.AddAddressField("reviewer", reviewer)

	framework.EmitEvent(event)
	return framework.SUCCESS
}

// SettleRound 结算一个互助周期，计算人均分摊额（纯计算 + 事件）
//
// 计算公式（简化版）：
//
//	total_with_fee = total_approved_payout * (10000 + service_fee_bp) / 10000
//	per_capita = ceil(total_with_fee / member_count_active)
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "round_id": "round_202501_01",
//	  "total_approved_payout": 1000000,
//	  "member_count_active": 2000000,
//	  "service_fee_bp": 800
//	}
//
// 输出：
// - Event: MutualAidRoundSettled
//
//export SettleRound
func SettleRound() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	roundID := params.ParseJSON("round_id")
	totalApproved := params.ParseJSONInt("total_approved_payout")
	memberCount := params.ParseJSONInt("member_count_active")
	serviceFeeBP := params.ParseJSONInt("service_fee_bp")

	if planID == "" || roundID == "" || totalApproved < 0 || memberCount <= 0 {
		return framework.ERROR_INVALID_PARAMS
	}

	totalWithFee := totalApproved * (10000 + serviceFeeBP) / 10000
	// 向上取整
	perCapita := (totalWithFee + memberCount - 1) / memberCount

	event := framework.NewEvent("MutualAidRoundSettled")
	event.AddStringField("plan_id", planID)
	event.AddStringField("round_id", roundID)
	event.AddIntField("total_approved_payout", totalApproved)
	event.AddIntField("member_count_active", memberCount)
	event.AddIntField("service_fee_bp", serviceFeeBP)
	event.AddIntField("total_with_fee", totalWithFee)
	event.AddIntField("per_capita_contribution", perCapita)

	framework.EmitEvent(event)
	return framework.SUCCESS
}

// PayContribution 成员为某一轮互助结算缴纳分摊
//
// 示例中复用 helpers/market.Escrow，将分摊视为：
//   - buyer: 成员地址
//   - seller: 互助资金池地址（可由平台统一管理）
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "round_id": "round_202501_01",
//	  "payer": "Cf1...",                  // 成员地址（Base58）
//	  "pool": "Df2...",                   // 资金池地址（Base58）
//	  "amount": 500,                      // 本次缴纳金额
//	  "contribution_id": "ctrb_202501_0001"
//	}
//
// 输出：
// - 使用 market.Escrow 创建实际资产托管
// - Event: 由 helpers/market 内部自动发出 Escrow 事件
//
//export PayContribution
func PayContribution() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	roundID := params.ParseJSON("round_id")
	payerStr := params.ParseJSON("payer")
	poolStr := params.ParseJSON("pool")
	amount := params.ParseJSONInt("amount")
	contributionID := params.ParseJSON("contribution_id")

	if planID == "" || roundID == "" || payerStr == "" || poolStr == "" || amount <= 0 || contributionID == "" {
		return framework.ERROR_INVALID_PARAMS
	}

	payer, err1 := framework.ParseAddressBase58(payerStr)
	pool, err2 := framework.ParseAddressBase58(poolStr)
	if err1 != nil || err2 != nil {
		return framework.ERROR_INVALID_PARAMS
	}

	// 使用托管实现成员 -> 资金池 的资金划转
	escrowID := []byte(planID + "_" + roundID + "_" + contributionID)
	err := market.Escrow(
		payer,
		pool,
		framework.TokenID(""), // 使用原生币；实际应用可改为稳定币或专用代币
		framework.Amount(amount),
		escrowID,
	)
	if err != nil {
		if contractErr, ok := err.(*framework.ContractError); ok {
			return contractErr.Code
		}
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

// Payout 为已通过审核的理赔案件进行给付
//
// 示例中直接创建「资金池 -> 受益人」的释放计划（一次性释放），
// 实际业务可按需做分期释放、时间锁等更复杂逻辑。
//
// 参数（JSON）：
//
//	{
//	  "plan_id": "plan_xianghubao_001",
//	  "claim_id": "claim_202501_0001",
//	  "from": "Df2...",                   // 资金池地址
//	  "beneficiary": "Cf1...",            // 受益人地址
//	  "amount": 300000,
//	  "payout_id": "payout_202501_0001"
//	}
//
// 输出：
// - 使用 market.Release 创建一次性释放计划
// - Event: 由 helpers/market 内部自动发出 Release 事件
//
//export Payout
func Payout() uint32 {
	params := framework.GetContractParams()

	planID := params.ParseJSON("plan_id")
	claimID := params.ParseJSON("claim_id")
	fromStr := params.ParseJSON("from")
	beneficiaryStr := params.ParseJSON("beneficiary")
	amount := params.ParseJSONInt("amount")
	payoutID := params.ParseJSON("payout_id")

	if planID == "" || claimID == "" || fromStr == "" || beneficiaryStr == "" || amount <= 0 || payoutID == "" {
		return framework.ERROR_INVALID_PARAMS
	}

	from, err1 := framework.ParseAddressBase58(fromStr)
	beneficiary, err2 := framework.ParseAddressBase58(beneficiaryStr)
	if err1 != nil || err2 != nil {
		return framework.ERROR_INVALID_PARAMS
	}

	vestingID := []byte(planID + "_" + claimID + "_" + payoutID)
	err := market.Release(
		from,
		beneficiary,
		framework.TokenID(""), // 使用原生币；实际应用可改为专用互助 Token
		framework.Amount(amount),
		vestingID,
	)
	if err != nil {
		if contractErr, ok := err.(*framework.ContractError); ok {
			return contractErr.Code
		}
		return framework.ERROR_EXECUTION_FAILED
	}

	return framework.SUCCESS
}

func main() {}
