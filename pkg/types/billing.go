// Package types 提供计费相关的业务抽象数据结构
package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// BillingMode 计费模式枚举
//
// 🎯 **设计说明**：
// - FREE: 免费模式（仍记录 CU，但不收费）
// - FIXED: 固定费用模式（每次调用固定费用）
// - CU_BASED: 基于 CU 的计费模式（费用 = CU × CUPrice）
type BillingMode string

const (
	BillingModeFREE    BillingMode = "FREE"      // 免费模式
	BillingModeFIXED   BillingMode = "FIXED"     // 固定费用模式
	BillingModeCUBASED BillingMode = "CU_BASED"  // 基于 CU 的计费模式
)

// String 返回计费模式的字符串表示
func (bm BillingMode) String() string {
	return string(bm)
}

// IsValid 验证计费模式是否有效
func (bm BillingMode) IsValid() bool {
	return bm == BillingModeFREE || bm == BillingModeFIXED || bm == BillingModeCUBASED
}

// TokenID 代币标识符（字符串格式，协议对齐语义）
//
// 🎯 **设计说明（与 transaction.proto 保持一致）**：
// - 不再使用任意的用户自定义别名（例如 "WES_TOKEN" 这类字符串）作为主标识；
// - 原生代币：使用 **空字符串 ""** 表示，对应协议层 TokenReference.native_token = true；
// - 合约代币：使用 40 字符的十六进制字符串表示 **合约地址**（20 字节），
//   对应协议层 TokenReference.contract_address / ContractTokenAsset.contract_address。
//
// ✅ 因此，全局唯一性来自：
// - 原生代币：唯一且无需额外标识；
// - 合约代币：由合约地址保证唯一性，而不是由任意名字保证。
type TokenID string

// TokenConfig 代币配置
//
// 定义资源支持的支付代币及其 CU 单价
type TokenConfig struct {
	TokenID TokenID  `json:"token_id"` // 代币标识符
	CUPrice *big.Int `json:"cu_price"` // 该代币的 CU 单价（最小单位，如 wei）
}

// MarshalJSON 自定义 JSON 序列化（big.Int 转字符串）
func (tc TokenConfig) MarshalJSON() ([]byte, error) {
	type Alias TokenConfig
	cuPriceStr := ""
	if tc.CUPrice != nil {
		cuPriceStr = tc.CUPrice.String()
	}
	return json.Marshal(&struct {
		TokenID string `json:"token_id"`
		CUPrice string `json:"cu_price"`
	}{
		TokenID: string(tc.TokenID),
		CUPrice: cuPriceStr,
	})
}

// UnmarshalJSON 自定义 JSON 反序列化（字符串转 big.Int）
func (tc *TokenConfig) UnmarshalJSON(data []byte) error {
	aux := &struct {
		TokenID string `json:"token_id"`
		CUPrice string `json:"cu_price"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	tc.TokenID = TokenID(aux.TokenID)
	if aux.CUPrice != "" {
		var ok bool
		tc.CUPrice, ok = new(big.Int).SetString(aux.CUPrice, 10)
		if !ok {
			return fmt.Errorf("无效的 CU 单价: %s", aux.CUPrice)
		}
	}
	return nil
}

// ResourcePricingState 资源定价状态
//
// 🎯 **核心职责**：
// - 定义资源的定价策略和支付方式
// - 存储在部署交易的 StateOutput.metadata 中
// - 通过 resource_hash 与 ResourceOutput 锚定
//
// 💡 **设计原则**：
// - 内容与定价分离：ResourceOutput 只承载内容，定价策略在 StateOutput 中
// - 多 Token 支持：支持多种代币支付，每种代币有独立的 CU 单价
// - 灵活计费模式：支持免费、固定费用、基于 CU 的计费
//
// 📋 **存储位置**：
// - 部署交易：StateOutput.metadata["pricing_state"] = JSON序列化的 ResourcePricingState
// - 索引：resource_hash → ResourcePricingState（本地 KV 存储）
type ResourcePricingState struct {
	// 资源标识
	ResourceHash []byte `json:"resource_hash"` // 指向的资源 content_hash（32字节）
	OwnerAddress []byte `json:"owner_address"` // 资源所有者地址（接收 Token，20字节）

	// 多 Token 支付支持（结构层允许多 Token）
	// ⚠️ 当前实现约束：对于 CU_BASED 计费模式，每个资源 **只能配置 1 个支付代币**。
	//    这样可以简化调用端的交互和结算逻辑，未来如有需要可以放宽为多 Token。
	PaymentTokens []TokenConfig `json:"payment_tokens"` // 支持的支付代币列表

	// CU 定价（仅 CU_BASED 模式需要）
	// key: TokenID (string), value: CU 单价（big.Int）
	// 注意：JSON 序列化时，big.Int 会被转换为字符串
	CUPrice map[TokenID]*big.Int `json:"cu_price"` // 每个 Token 的 CU 单价

	// 计费模式（仅与可执行资源本身相关）
	BillingMode BillingMode `json:"billing_mode"` // FREE / FIXED / CU_BASED

	// 固定费用（仅 FIXED 模式需要）
	FixedFee *big.Int `json:"fixed_fee,omitempty"` // 固定费用金额

	// 可选配置
	FreeUntil uint64 `json:"free_until,omitempty"` // 免费期限（Unix 时间戳，0 表示永不过期）
}

// NewResourcePricingState 创建新的资源定价状态
//
// 参数：
//   - resourceHash: 资源内容哈希
//   - ownerAddress: 资源所有者地址
//   - billingMode: 计费模式
//
// 返回：
//   - *ResourcePricingState: 新创建的定价状态
func NewResourcePricingState(
	resourceHash []byte,
	ownerAddress []byte,
	billingMode BillingMode,
) *ResourcePricingState {
	if !billingMode.IsValid() {
		panic(fmt.Sprintf("无效的计费模式: %s", billingMode))
	}

	return &ResourcePricingState{
		ResourceHash:  resourceHash,
		OwnerAddress:  ownerAddress,
		PaymentTokens: make([]TokenConfig, 0),
		CUPrice:       make(map[TokenID]*big.Int),
		BillingMode:   billingMode,
	}
}

// AddPaymentToken 添加支付代币配置
//
// 参数：
//   - tokenID: 代币标识符
//   - cuPrice: CU 单价（最小单位，如 wei）
//
// 返回：
//   - *ResourcePricingState: 返回自身，支持链式调用
func (ps *ResourcePricingState) AddPaymentToken(tokenID TokenID, cuPrice *big.Int) *ResourcePricingState {
	if cuPrice == nil || cuPrice.Sign() < 0 {
		panic("CU 单价必须 >= 0")
	}

	// 基础格式校验（开发时尽早发现错误）
	if err := validateTokenID(tokenID); err != nil {
		panic(err)
	}

	// 添加到 PaymentTokens 列表
	ps.PaymentTokens = append(ps.PaymentTokens, TokenConfig{
		TokenID: tokenID,
		CUPrice: new(big.Int).Set(cuPrice), // 复制，避免外部修改
	})

	// 添加到 CUPrice map（用于快速查询）
	ps.CUPrice[tokenID] = new(big.Int).Set(cuPrice)

	return ps
}

// SetFixedFee 设置固定费用（仅 FIXED 模式）
//
// 参数：
//   - fee: 固定费用金额
//
// 返回：
//   - *ResourcePricingState: 返回自身，支持链式调用
func (ps *ResourcePricingState) SetFixedFee(fee *big.Int) *ResourcePricingState {
	if ps.BillingMode != BillingModeFIXED {
		panic("只有 FIXED 模式才能设置固定费用")
	}
	if fee == nil || fee.Sign() < 0 {
		panic("固定费用必须 >= 0")
	}

	ps.FixedFee = new(big.Int).Set(fee)
	return ps
}

// SetFreeUntil 设置免费期限
//
// 参数：
//   - timestamp: Unix 时间戳（0 表示永不过期）
//
// 返回：
//   - *ResourcePricingState: 返回自身，支持链式调用
func (ps *ResourcePricingState) SetFreeUntil(timestamp uint64) *ResourcePricingState {
	ps.FreeUntil = timestamp
	return ps
}

// IsFree 检查当前是否免费
//
// 返回：
//   - bool: true 表示免费，false 表示需要付费
func (ps *ResourcePricingState) IsFree() bool {
	if ps.BillingMode == BillingModeFREE {
		return true
	}

	// 检查免费期限
	if ps.FreeUntil > 0 {
		now := uint64(time.Now().Unix())
		return now < ps.FreeUntil
	}

	return false
}

// GetCUPrice 获取指定代币的 CU 单价
//
// 参数：
//   - tokenID: 代币标识符
//
// 返回：
//   - *big.Int: CU 单价（nil 表示不支持该代币）
//   - bool: true 表示支持该代币，false 表示不支持
func (ps *ResourcePricingState) GetCUPrice(tokenID TokenID) (*big.Int, bool) {
	price, ok := ps.CUPrice[tokenID]
	if !ok {
		return nil, false
	}
	return new(big.Int).Set(price), true // 返回副本，避免外部修改
}

// GetFixedFee 获取固定费用（仅 FIXED 模式）
//
// 返回：
//   - *big.Int: 固定费用金额（nil 表示未设置）
//   - bool: true 表示已设置固定费用，false 表示未设置
func (ps *ResourcePricingState) GetFixedFee() (*big.Int, bool) {
	if ps.FixedFee == nil {
		return nil, false
	}
	return new(big.Int).Set(ps.FixedFee), true // 返回副本，避免外部修改
}

// GetFreeUntil 获取免费期限
//
// 返回：
//   - uint64: 免费期限（Unix 时间戳，0 表示永不过期）
//   - bool: true 表示已设置免费期限，false 表示未设置
func (ps *ResourcePricingState) GetFreeUntil() (uint64, bool) {
	if ps.FreeUntil == 0 {
		return 0, false
	}
	return ps.FreeUntil, true
}

// Validate 验证定价状态的完整性
//
// 返回：
//   - error: 验证失败时的错误
func (ps *ResourcePricingState) Validate() error {
	// 验证资源哈希
	if len(ps.ResourceHash) == 0 {
		return fmt.Errorf("resource_hash 不能为空")
	}

	// 验证所有者地址
	if len(ps.OwnerAddress) == 0 {
		return fmt.Errorf("owner_address 不能为空")
	}

	// 验证计费模式
	if !ps.BillingMode.IsValid() {
		return fmt.Errorf("无效的计费模式: %s", ps.BillingMode)
	}

	// 验证 CU_BASED 模式必须有 CUPrice
	if ps.BillingMode == BillingModeCUBASED {
		// 当前实现约束：每个资源 **只能配置 1 个支付代币**
		if len(ps.PaymentTokens) != 1 {
			return fmt.Errorf("CU_BASED 模式当前仅支持配置 1 个支付代币，实际: %d", len(ps.PaymentTokens))
		}

		if len(ps.CUPrice) == 0 {
			return fmt.Errorf("CU_BASED 模式必须至少配置一个代币的 CU 单价")
		}

		// 校验 TokenID 格式 & CU 单价有效
		for tokenID, price := range ps.CUPrice {
			if err := validateTokenID(tokenID); err != nil {
				return err
			}
			if price == nil || price.Sign() < 0 {
				return fmt.Errorf("代币 %s 的 CU 单价无效", tokenID)
			}
		}

		// PaymentTokens 与 CUPrice 之间的一致性校验
		expectedTokenID := ps.PaymentTokens[0].TokenID
		if err := validateTokenID(expectedTokenID); err != nil {
			return err
		}

		if len(ps.CUPrice) != 1 {
			return fmt.Errorf("CU_BASED 模式当前仅支持 1 个 TokenID 对应的 CU 单价，实际: %d", len(ps.CUPrice))
		}

		if _, ok := ps.CUPrice[expectedTokenID]; !ok {
			return fmt.Errorf("CU_BASED 模式下 PaymentTokens 与 CUPrice 不一致，缺少 TokenID=%s 的定价", expectedTokenID)
		}
	}

	// 验证 FIXED 模式必须有 FixedFee
	if ps.BillingMode == BillingModeFIXED {
		if ps.FixedFee == nil || ps.FixedFee.Sign() < 0 {
			return fmt.Errorf("FIXED 模式必须设置固定费用")
		}
	}

	return nil
}

// validateTokenID 校验 TokenID 的格式是否符合协议语义
//
// 约束：
// - 原生代币：TokenID == ""（空字符串）
// - 合约代币：TokenID 为 40 字符十六进制字符串（对应 20 字节合约地址）
func validateTokenID(id TokenID) error {
	s := string(id)
	if s == "" {
		// 空字符串表示原生代币，对应 TokenReference.native_token = true
		return nil
	}

	// 合约代币：必须是 40 字符的十六进制字符串
	if len(s) != 40 {
		return fmt.Errorf("TokenID[%s] 长度必须为 40（20 字节合约地址的十六进制表示）或为空字符串（原生代币）", s)
	}

	if _, err := hex.DecodeString(s); err != nil {
		return fmt.Errorf("TokenID[%s] 必须是有效的十六进制字符串: %w", s, err)
	}

	return nil
}

// Encode 序列化定价状态为 JSON 字节
//
// 返回：
//   - []byte: JSON 序列化后的字节
//   - error: 序列化失败时的错误
//
// 💡 **序列化格式**：
//   - big.Int 字段会被转换为字符串（避免精度丢失）
//   - []byte 字段会被转换为十六进制字符串
func (ps *ResourcePricingState) Encode() ([]byte, error) {
	if err := ps.Validate(); err != nil {
		return nil, fmt.Errorf("定价状态验证失败: %w", err)
	}

	// 创建临时结构体，将 big.Int 和 []byte 转换为字符串
	type PricingStateJSON struct {
		ResourceHash  string        `json:"resource_hash"`
		OwnerAddress  string        `json:"owner_address"`
		PaymentTokens []TokenConfig `json:"payment_tokens"`
		CUPrice       map[string]string `json:"cu_price"`
		BillingMode   string        `json:"billing_mode"`
		FixedFee      string        `json:"fixed_fee,omitempty"`
		FreeUntil     uint64        `json:"free_until,omitempty"`
	}

	jsonData := PricingStateJSON{
		ResourceHash:  fmt.Sprintf("%x", ps.ResourceHash),
		OwnerAddress:  fmt.Sprintf("%x", ps.OwnerAddress),
		PaymentTokens: ps.PaymentTokens,
		CUPrice:       make(map[string]string),
		BillingMode:   string(ps.BillingMode),
		FreeUntil:     ps.FreeUntil,
	}

	// 转换 CUPrice map
	for tokenID, price := range ps.CUPrice {
		if price != nil {
			jsonData.CUPrice[string(tokenID)] = price.String()
		}
	}

	// 转换 FixedFee
	if ps.FixedFee != nil {
		jsonData.FixedFee = ps.FixedFee.String()
	}

	return json.Marshal(jsonData)
}

// DecodePricingState 从 JSON 字节反序列化定价状态
//
// 参数：
//   - data: JSON 序列化后的字节
//
// 返回：
//   - *ResourcePricingState: 反序列化后的定价状态
//   - error: 反序列化失败时的错误
func DecodePricingState(data []byte) (*ResourcePricingState, error) {
	// 先解析为临时结构体
	type PricingStateJSON struct {
		ResourceHash  string            `json:"resource_hash"`
		OwnerAddress  string            `json:"owner_address"`
		PaymentTokens []TokenConfig     `json:"payment_tokens"`
		CUPrice       map[string]string `json:"cu_price"`
		BillingMode   string            `json:"billing_mode"`
		FixedFee      string            `json:"fixed_fee,omitempty"`
		FreeUntil     uint64            `json:"free_until,omitempty"`
	}

	var jsonData PricingStateJSON
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("反序列化定价状态失败: %w", err)
	}

	// 转换为 ResourcePricingState
	ps := &ResourcePricingState{
		BillingMode:   BillingMode(jsonData.BillingMode),
		PaymentTokens: jsonData.PaymentTokens,
		CUPrice:       make(map[TokenID]*big.Int),
		FreeUntil:     jsonData.FreeUntil,
	}

	// 解析 ResourceHash
	if len(jsonData.ResourceHash) > 0 {
		// 尝试解析十六进制字符串
		if hash, err := hex.DecodeString(jsonData.ResourceHash); err == nil {
			ps.ResourceHash = hash
		} else {
			return nil, fmt.Errorf("无效的 resource_hash 格式: %s", jsonData.ResourceHash)
		}
	}

	// 解析 OwnerAddress
	if len(jsonData.OwnerAddress) > 0 {
		if addr, err := hex.DecodeString(jsonData.OwnerAddress); err == nil {
			ps.OwnerAddress = addr
		} else {
			return nil, fmt.Errorf("无效的 owner_address 格式: %s", jsonData.OwnerAddress)
		}
	}

	// 解析 CUPrice map
	for tokenIDStr, priceStr := range jsonData.CUPrice {
		price, ok := new(big.Int).SetString(priceStr, 10)
		if !ok {
			return nil, fmt.Errorf("无效的 CU 单价: %s (代币: %s)", priceStr, tokenIDStr)
		}
		ps.CUPrice[TokenID(tokenIDStr)] = price
	}

	// 解析 FixedFee
	if jsonData.FixedFee != "" {
		fee, ok := new(big.Int).SetString(jsonData.FixedFee, 10)
		if !ok {
			return nil, fmt.Errorf("无效的固定费用: %s", jsonData.FixedFee)
		}
		ps.FixedFee = fee
	}

	// 验证反序列化后的状态
	if err := ps.Validate(); err != nil {
		return nil, fmt.Errorf("反序列化后的定价状态验证失败: %w", err)
	}

	return ps, nil
}

// CalculateFee 计算费用
//
// 参数：
//   - cu: 消耗的 CU 数量
//   - tokenID: 选择的支付代币
//
// 返回：
//   - *big.Int: 应付费用（最小单位，如 wei）
//   - error: 计算失败时的错误
func (ps *ResourcePricingState) CalculateFee(cu float64, tokenID TokenID) (*big.Int, error) {
	// 检查是否免费
	if ps.IsFree() {
		return big.NewInt(0), nil
	}

	// 根据计费模式计算费用
	switch ps.BillingMode {
	case BillingModeFREE:
		return big.NewInt(0), nil

	case BillingModeFIXED:
		if ps.FixedFee == nil {
			return nil, fmt.Errorf("FIXED 模式未设置固定费用")
		}
		return new(big.Int).Set(ps.FixedFee), nil

	case BillingModeCUBASED:
		cuPrice, ok := ps.GetCUPrice(tokenID)
		if !ok {
			return nil, fmt.Errorf("代币 %s 不支持或未配置 CU 单价", tokenID)
		}

		// fee = CU × CUPrice
		cuBigFloat := new(big.Float).SetFloat64(cu)
		priceBigFloat := new(big.Float).SetInt(cuPrice)
		feeBigFloat := new(big.Float).Mul(cuBigFloat, priceBigFloat)

		// 转换为 big.Int（向下取整）
		feeInt, _ := feeBigFloat.Int(nil)
		if feeInt == nil {
			feeInt = big.NewInt(0)
		}

		return feeInt, nil

	default:
		return nil, fmt.Errorf("不支持的计费模式: %s", ps.BillingMode)
	}
}

