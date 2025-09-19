package fee

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ===============================
// Coinbase构建器
// ===============================

// CoinbaseBuilder Coinbase交易构建器
type CoinbaseBuilder struct{}

// NewCoinbaseBuilder 创建新的Coinbase构建器
func NewCoinbaseBuilder() *CoinbaseBuilder {
	return &CoinbaseBuilder{}
}

// BuildCoinbase 构建Coinbase交易
func (cb *CoinbaseBuilder) BuildCoinbase(aggregatedFees *AggregatedFees, minerAddr []byte, chainID []byte) (*pbtx.Transaction, error) {
	if aggregatedFees == nil {
		return nil, NewValidationError("aggregated_fees", nil, "聚合费用不能为nil")
	}

	if err := ValidateAddress(minerAddr); err != nil {
		return nil, WrapError(err, "矿工地址无效")
	}

	if len(chainID) == 0 {
		return nil, NewValidationError("chain_id", chainID, "链ID不能为空")
	}

	// 创建基础Coinbase交易
	coinbase := &pbtx.Transaction{
		Version:           1,
		Inputs:            []*pbtx.TxInput{}, // Coinbase交易必须无输入
		Outputs:           []*pbtx.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           chainID,
	}

	// 按Token创建输出（确保确定性排序）
	sortedTokenKeys := SortTokenKeysFromMap(aggregatedFees.ByToken)

	for _, tokenKey := range sortedTokenKeys {
		amount := aggregatedFees.ByToken[tokenKey]
		if IsPositive(amount) {
			output, err := cb.createFeeOutput(tokenKey, amount, minerAddr)
			if err != nil {
				return nil, WrapError(err, fmt.Sprintf("创建费用输出失败 [%s]", tokenKey))
			}
			coinbase.Outputs = append(coinbase.Outputs, output)
		}
	}

	// 即使没有费用，也返回空的Coinbase交易以保持模板结构的一致性
	return coinbase, nil
}

// createFeeOutput 创建单个Token的费用输出
func (cb *CoinbaseBuilder) createFeeOutput(tokenKey TokenKey, amount *big.Int, minerAddr []byte) (*pbtx.TxOutput, error) {
	if !IsPositive(amount) {
		return nil, NewValidationError("amount", amount, "费用金额必须大于0")
	}

	// 创建基础输出结构
	output := &pbtx.TxOutput{
		Owner: minerAddr,
		LockingConditions: []*pbtx.LockingCondition{
			cb.createMinerLockingCondition(minerAddr),
		},
	}

	// 根据Token类型创建对应的资产输出
	if IsNativeToken(tokenKey) {
		output.OutputContent = cb.createNativeCoinOutput(amount)
	} else {
		contractOutput, err := cb.createContractTokenOutput(tokenKey, amount)
		if err != nil {
			return nil, WrapError(err, "创建合约代币输出失败")
		}
		output.OutputContent = contractOutput
	}

	return output, nil
}

// createMinerLockingCondition 创建矿工的锁定条件
func (cb *CoinbaseBuilder) createMinerLockingCondition(minerAddr []byte) *pbtx.LockingCondition {
	return &pbtx.LockingCondition{
		Condition: &pbtx.LockingCondition_SingleKeyLock{
			SingleKeyLock: &pbtx.SingleKeyLock{
				KeyRequirement: &pbtx.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: minerAddr,
				},
				RequiredAlgorithm: pbtx.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:       pbtx.SignatureHashType_SIGHASH_ALL,
			},
		},
	}
}

// createNativeCoinOutput 创建原生币输出
func (cb *CoinbaseBuilder) createNativeCoinOutput(amount *big.Int) *pbtx.TxOutput_Asset {
	return &pbtx.TxOutput_Asset{
		Asset: &pbtx.AssetOutput{
			AssetContent: &pbtx.AssetOutput_NativeCoin{
				NativeCoin: &pbtx.NativeCoinAsset{
					Amount: amount.String(),
				},
			},
		},
	}
}

// createContractTokenOutput 创建合约代币输出
func (cb *CoinbaseBuilder) createContractTokenOutput(tokenKey TokenKey, amount *big.Int) (*pbtx.TxOutput_Asset, error) {
	// 解析TokenKey获取合约信息
	contractAddr, tokenType, tokenInfo, err := ParseTokenKey(tokenKey)
	if err != nil {
		return nil, WrapError(err, "解析TokenKey失败")
	}

	// 创建合约代币资产
	contractToken := &pbtx.ContractTokenAsset{
		ContractAddress: contractAddr,
		Amount:          amount.String(),
	}

	// 根据Token类型设置TokenIdentifier
	switch tokenType {
	case "ft":
		// 同质化代币
		tokenIDBytes := []byte(tokenInfo)
		contractToken.TokenIdentifier = &pbtx.ContractTokenAsset_FungibleClassId{
			FungibleClassId: tokenIDBytes,
		}

	case "nft":
		// 非同质化代币
		tokenIDBytes := []byte(tokenInfo)
		contractToken.TokenIdentifier = &pbtx.ContractTokenAsset_NftUniqueId{
			NftUniqueId: tokenIDBytes,
		}

	case "sft":
		// 半同质化代币
		batchID, instanceID, err := cb.parseSemiFungibleTokenInfo(tokenInfo)
		if err != nil {
			return nil, WrapError(err, "解析半同质化代币信息失败")
		}

		contractToken.TokenIdentifier = &pbtx.ContractTokenAsset_SemiFungibleId{
			SemiFungibleId: &pbtx.SemiFungibleId{
				BatchId:    batchID,
				InstanceId: instanceID,
			},
		}

	default:
		// 未知类型，不设置TokenIdentifier
	}

	return &pbtx.TxOutput_Asset{
		Asset: &pbtx.AssetOutput{
			AssetContent: &pbtx.AssetOutput_ContractToken{
				ContractToken: contractToken,
			},
		},
	}, nil
}

// parseSemiFungibleTokenInfo 解析半同质化代币信息
// tokenInfo格式: "batchID|instanceID" 或 "batchID:instanceID"
func (cb *CoinbaseBuilder) parseSemiFungibleTokenInfo(tokenInfo string) ([]byte, uint64, error) {
	if tokenInfo == "" {
		return nil, 0, NewValidationError("tokenInfo", tokenInfo, "半同质化代币信息不能为空")
	}

	// 尝试使用 '|' 分隔符解析
	if parts := strings.Split(tokenInfo, "|"); len(parts) == 2 {
		batchID := []byte(strings.TrimSpace(parts[0]))
		instanceIDStr := strings.TrimSpace(parts[1])

		if len(batchID) == 0 {
			return nil, 0, NewValidationError("batchID", string(batchID), "批次ID不能为空")
		}

		instanceID, err := strconv.ParseUint(instanceIDStr, 10, 64)
		if err != nil {
			return nil, 0, WrapError(err, fmt.Sprintf("解析实例ID失败: %s", instanceIDStr))
		}

		return batchID, instanceID, nil
	}

	// 尝试使用 ':' 分隔符解析
	if parts := strings.Split(tokenInfo, ":"); len(parts) == 2 {
		batchID := []byte(strings.TrimSpace(parts[0]))
		instanceIDStr := strings.TrimSpace(parts[1])

		if len(batchID) == 0 {
			return nil, 0, NewValidationError("batchID", string(batchID), "批次ID不能为空")
		}

		instanceID, err := strconv.ParseUint(instanceIDStr, 10, 64)
		if err != nil {
			return nil, 0, WrapError(err, fmt.Sprintf("解析实例ID失败: %s", instanceIDStr))
		}

		return batchID, instanceID, nil
	}

	// 如果没有分隔符，将整个字符串作为batchID，instanceID为0
	batchID := []byte(strings.TrimSpace(tokenInfo))
	if len(batchID) == 0 {
		return nil, 0, NewValidationError("batchID", tokenInfo, "批次ID不能为空")
	}

	return batchID, 0, nil
}

// ===============================
// Coinbase验证器
// ===============================

// ValidateCoinbase 验证Coinbase交易的正确性
func (cb *CoinbaseBuilder) ValidateCoinbase(coinbase *pbtx.Transaction, expectedFees *AggregatedFees, minerAddr []byte) error {
	if coinbase == nil {
		return NewValidationError("coinbase", nil, "Coinbase交易不能为nil")
	}

	// 验证Coinbase交易的基本结构
	if len(coinbase.Inputs) != 0 {
		return NewValidationError("coinbase_inputs", coinbase.Inputs, "Coinbase交易不能有输入")
	}

	if coinbase.Version == 0 {
		return NewValidationError("coinbase_version", coinbase.Version, "Coinbase交易版本必须大于0")
	}

	if coinbase.CreationTimestamp == 0 {
		return NewValidationError("coinbase_timestamp", coinbase.CreationTimestamp, "Coinbase交易时间戳不能为0")
	}

	if len(coinbase.ChainId) == 0 {
		return NewValidationError("coinbase_chain_id", coinbase.ChainId, "Coinbase交易链ID不能为空")
	}

	if expectedFees == nil {
		return NewValidationError("expected_fees", nil, "预期费用不能为nil")
	}

	// 验证输出数量和费用匹配
	expectedOutputCount := 0
	for _, amount := range expectedFees.ByToken {
		if IsPositive(amount) {
			expectedOutputCount++
		}
	}

	if len(coinbase.Outputs) != expectedOutputCount {
		return fmt.Errorf("Coinbase输出数量不匹配: 期望 %d, 实际 %d", expectedOutputCount, len(coinbase.Outputs))
	}

	// 验证每个输出
	outputFees := make(map[TokenKey]*big.Int)
	for i, output := range coinbase.Outputs {
		if err := cb.validateCoinbaseOutput(output, minerAddr, i); err != nil {
			return WrapError(err, fmt.Sprintf("Coinbase输出[%d]验证失败", i))
		}

		// 提取输出的Token信息
		tokenKey, amount, err := ExtractTokenInfo(output)
		if err != nil {
			return WrapError(err, fmt.Sprintf("提取Coinbase输出[%d]Token信息失败", i))
		}

		outputFees[tokenKey] = amount
	}

	// 验证费用金额匹配
	for tokenKey, expectedAmount := range expectedFees.ByToken {
		if !IsPositive(expectedAmount) {
			continue
		}

		actualAmount := outputFees[tokenKey]
		if actualAmount == nil || actualAmount.Cmp(expectedAmount) != 0 {
			return fmt.Errorf("Coinbase费用不匹配 [%s]: 期望 %s, 实际 %s",
				tokenKey, FormatAmount(expectedAmount), FormatAmount(actualAmount))
		}
	}

	return nil
}

// validateCoinbaseOutput 验证单个Coinbase输出
func (cb *CoinbaseBuilder) validateCoinbaseOutput(output *pbtx.TxOutput, expectedMinerAddr []byte, index int) error {
	if output == nil {
		return NewValidationError("output", nil, "输出不能为nil")
	}

	// 验证所有者地址
	if string(output.Owner) != string(expectedMinerAddr) {
		return fmt.Errorf("输出所有者地址不匹配: 期望 %x, 实际 %x", expectedMinerAddr, output.Owner)
	}

	// 验证锁定条件
	if len(output.LockingConditions) == 0 {
		return NewValidationError("locking_conditions", output.LockingConditions, "输出必须有锁定条件")
	}

	// 验证是否为资产输出
	if output.OutputContent == nil {
		return NewValidationError("output_content", nil, "输出内容不能为nil")
	}

	switch outputContent := output.OutputContent.(type) {
	case *pbtx.TxOutput_Asset:
		return cb.validateAssetOutput(outputContent.Asset, index)
	default:
		return fmt.Errorf("Coinbase输出必须是资产输出，实际类型: %T", outputContent)
	}
}

// validateAssetOutput 验证资产输出
func (cb *CoinbaseBuilder) validateAssetOutput(asset *pbtx.AssetOutput, index int) error {
	if asset == nil {
		return NewValidationError("asset", nil, "资产输出不能为nil")
	}

	switch assetContent := asset.AssetContent.(type) {
	case *pbtx.AssetOutput_NativeCoin:
		return cb.validateNativeCoinAsset(assetContent.NativeCoin, index)
	case *pbtx.AssetOutput_ContractToken:
		return cb.validateContractTokenAsset(assetContent.ContractToken, index)
	default:
		return fmt.Errorf("未知的资产类型: %T", assetContent)
	}
}

// validateNativeCoinAsset 验证原生币资产
func (cb *CoinbaseBuilder) validateNativeCoinAsset(nativeCoin *pbtx.NativeCoinAsset, index int) error {
	if nativeCoin == nil {
		return NewValidationError("native_coin", nil, "原生币资产不能为nil")
	}

	amount := ParseAmountString(nativeCoin.Amount)
	if !IsPositive(amount) {
		return fmt.Errorf("原生币金额必须大于0: %s", nativeCoin.Amount)
	}

	return nil
}

// validateContractTokenAsset 验证合约代币资产
func (cb *CoinbaseBuilder) validateContractTokenAsset(contractToken *pbtx.ContractTokenAsset, index int) error {
	if contractToken == nil {
		return NewValidationError("contract_token", nil, "合约代币资产不能为nil")
	}

	if len(contractToken.ContractAddress) == 0 {
		return NewValidationError("contract_address", contractToken.ContractAddress, "合约地址不能为空")
	}

	amount := ParseAmountString(contractToken.Amount)
	if !IsPositive(amount) {
		return fmt.Errorf("合约代币金额必须大于0: %s", contractToken.Amount)
	}

	// TokenIdentifier可以为空（表示默认代币）

	return nil
}

// ===============================
// Coinbase工具方法
// ===============================

// GetCoinbaseFees 从Coinbase交易中提取费用信息
func (cb *CoinbaseBuilder) GetCoinbaseFees(coinbase *pbtx.Transaction) (map[TokenKey]*big.Int, error) {
	if coinbase == nil {
		return nil, NewValidationError("coinbase", nil, "Coinbase交易不能为nil")
	}

	fees := make(map[TokenKey]*big.Int)

	for i, output := range coinbase.Outputs {
		tokenKey, amount, err := ExtractTokenInfo(output)
		if err != nil {
			return nil, WrapError(err, fmt.Sprintf("提取输出[%d]Token信息失败", i))
		}

		if IsPositive(amount) {
			fees[tokenKey] = amount
		}
	}

	return fees, nil
}

// IsCoinbaseTransaction 判断是否为Coinbase交易
func (cb *CoinbaseBuilder) IsCoinbaseTransaction(tx *pbtx.Transaction) bool {
	return tx != nil && len(tx.Inputs) == 0 && len(tx.Outputs) > 0
}

// IsEmptyCoinbase 判断是否为空Coinbase交易
func (cb *CoinbaseBuilder) IsEmptyCoinbase(tx *pbtx.Transaction) bool {
	return tx != nil && len(tx.Inputs) == 0 && len(tx.Outputs) == 0
}

// GetCoinbaseValue 获取Coinbase交易的总价值
func (cb *CoinbaseBuilder) GetCoinbaseValue(coinbase *pbtx.Transaction) (map[TokenKey]*big.Int, error) {
	return cb.GetCoinbaseFees(coinbase)
}

// FormatCoinbase 格式化Coinbase交易信息
func (cb *CoinbaseBuilder) FormatCoinbase(coinbase *pbtx.Transaction) string {
	if coinbase == nil {
		return "nil"
	}

	fees, err := cb.GetCoinbaseFees(coinbase)
	if err != nil {
		return fmt.Sprintf("Coinbase(解析错误: %v)", err)
	}

	if len(fees) == 0 {
		return "Coinbase(空费用)"
	}

	return fmt.Sprintf("Coinbase(版本: %d, 输出: %d, 费用: %s, 时间戳: %d)",
		coinbase.Version,
		len(coinbase.Outputs),
		FormatTokenMap(fees),
		coinbase.CreationTimestamp,
	)
}
