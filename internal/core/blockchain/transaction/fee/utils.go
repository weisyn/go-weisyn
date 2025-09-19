package fee

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ===============================
// 数学计算工具
// ===============================

// SafeAdd 安全加法，防止nil指针异常
func SafeAdd(a, b *big.Int) *big.Int {
	if a == nil {
		a = big.NewInt(0)
	}
	if b == nil {
		b = big.NewInt(0)
	}
	return new(big.Int).Add(a, b)
}

// SafeSub 安全减法，防止负数和nil指针异常
func SafeSub(a, b *big.Int) *big.Int {
	if a == nil {
		a = big.NewInt(0)
	}
	if b == nil {
		b = big.NewInt(0)
	}
	result := new(big.Int).Sub(a, b)
	if result.Sign() < 0 {
		return big.NewInt(0)
	}
	return result
}

// SafeMul 安全乘法，防止nil指针异常
func SafeMul(a, b *big.Int) *big.Int {
	if a == nil || b == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Mul(a, b)
}

// SafeDiv 安全除法，防止除零和nil指针异常
func SafeDiv(a, b *big.Int) *big.Int {
	if a == nil || b == nil || b.Sign() == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(a, b)
}

// ParseAmountString 解析金额字符串为big.Int
func ParseAmountString(amountStr string) *big.Int {
	if amountStr == "" {
		return big.NewInt(0)
	}

	// 移除空格和换行符
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return big.NewInt(0)
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return big.NewInt(0)
	}

	// 确保非负数
	if amount.Sign() < 0 {
		return big.NewInt(0)
	}

	return amount
}

// IsZero 检查big.Int是否为零
func IsZero(amount *big.Int) bool {
	return amount == nil || amount.Sign() == 0
}

// IsPositive 检查big.Int是否为正数
func IsPositive(amount *big.Int) bool {
	return amount != nil && amount.Sign() > 0
}

// Max 返回两个big.Int中的较大值
func Max(a, b *big.Int) *big.Int {
	if a == nil && b == nil {
		return big.NewInt(0)
	}
	if a == nil {
		return new(big.Int).Set(b)
	}
	if b == nil {
		return new(big.Int).Set(a)
	}

	if a.Cmp(b) > 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}

// Min 返回两个big.Int中的较小值
func Min(a, b *big.Int) *big.Int {
	if a == nil && b == nil {
		return big.NewInt(0)
	}
	if a == nil {
		return new(big.Int).Set(b)
	}
	if b == nil {
		return new(big.Int).Set(a)
	}

	if a.Cmp(b) < 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}

// ===============================
// Token标识符工具
// ===============================

// TokenKey Token的唯一标识符类型
type TokenKey string

const (
	// NativeTokenKey 原生代币的标识符
	NativeTokenKey TokenKey = "native"
)

// GenerateNativeTokenKey 生成原生代币的TokenKey
func GenerateNativeTokenKey() TokenKey {
	return NativeTokenKey
}

// GenerateContractTokenKey 生成合约代币的TokenKey
func GenerateContractTokenKey(contractAddr []byte, tokenType string, tokenID []byte) TokenKey {
	if len(contractAddr) == 0 {
		return NativeTokenKey
	}

	contractHex := fmt.Sprintf("%x", contractAddr)
	tokenIDHex := fmt.Sprintf("%x", tokenID)

	return TokenKey(fmt.Sprintf("%s|%s|%s", contractHex, tokenType, tokenIDHex))
}

// GenerateSemiFungibleTokenKey 生成半同质化代币的TokenKey
func GenerateSemiFungibleTokenKey(contractAddr []byte, batchID []byte, instanceID uint64) TokenKey {
	if len(contractAddr) == 0 {
		return NativeTokenKey
	}

	contractHex := fmt.Sprintf("%x", contractAddr)
	batchIDHex := fmt.Sprintf("%x", batchID)

	return TokenKey(fmt.Sprintf("%s|sft|%s|%d", contractHex, batchIDHex, instanceID))
}

// IsNativeToken 检查是否为原生代币
func IsNativeToken(tokenKey TokenKey) bool {
	return tokenKey == NativeTokenKey
}

// ParseTokenKey 解析TokenKey，返回合约地址和代币信息
func ParseTokenKey(tokenKey TokenKey) (contractAddr []byte, tokenType string, tokenInfo string, err error) {
	if tokenKey == NativeTokenKey {
		return nil, "native", "", nil
	}

	parts := strings.Split(string(tokenKey), "|")
	if len(parts) < 3 {
		return nil, "", "", fmt.Errorf("invalid token key format: %s", tokenKey)
	}

	// 解析合约地址
	contractAddr = make([]byte, len(parts[0])/2)
	_, err = fmt.Sscanf(parts[0], "%x", &contractAddr)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid contract address in token key: %v", err)
	}

	tokenType = parts[1]

	// 组合剩余部分作为token信息
	if len(parts) > 3 {
		tokenInfo = strings.Join(parts[2:], "|")
	} else {
		tokenInfo = parts[2]
	}

	return contractAddr, tokenType, tokenInfo, nil
}

// ===============================
// 验证工具
// ===============================

// ValidateAddress 验证地址的基本格式
func ValidateAddress(addr []byte) error {
	if len(addr) == 0 {
		return fmt.Errorf("地址不能为空")
	}

	// 基本长度检查（通常是20字节）
	if len(addr) != 20 {
		return fmt.Errorf("地址长度无效: 期望20字节，实际%d字节", len(addr))
	}

	return nil
}

// ValidateAmount 验证金额的有效性
func ValidateAmount(amount *big.Int) error {
	if amount == nil {
		return fmt.Errorf("金额不能为nil")
	}

	if amount.Sign() < 0 {
		return fmt.Errorf("金额不能为负数: %s", amount.String())
	}

	return nil
}

// ValidateAmountString 验证金额字符串的有效性
func ValidateAmountString(amountStr string) error {
	if amountStr == "" {
		return nil // 空字符串视为0，是有效的
	}

	amountStr = strings.TrimSpace(amountStr)
	amount := ParseAmountString(amountStr)

	return ValidateAmount(amount)
}

// ===============================
// 排序工具
// ===============================

// SortTokenKeys 对TokenKey进行确定性排序
func SortTokenKeys(tokenKeys []TokenKey) []TokenKey {
	sorted := make([]TokenKey, len(tokenKeys))
	copy(sorted, tokenKeys)

	sort.Slice(sorted, func(i, j int) bool {
		// 原生代币始终排在第一位
		if sorted[i] == NativeTokenKey {
			return true
		}
		if sorted[j] == NativeTokenKey {
			return false
		}

		// 其他代币按字典序排序
		return string(sorted[i]) < string(sorted[j])
	})

	return sorted
}

// SortTokenKeysFromMap 从map中提取TokenKey并排序
func SortTokenKeysFromMap(tokenMap map[TokenKey]*big.Int) []TokenKey {
	keys := make([]TokenKey, 0, len(tokenMap))
	for key := range tokenMap {
		keys = append(keys, key)
	}

	return SortTokenKeys(keys)
}

// ===============================
// 错误处理工具
// ===============================

// WrapError 包装错误信息，添加上下文
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %v", context, err)
}

// NewValidationError 创建验证错误
func NewValidationError(field string, value interface{}, reason string) error {
	return fmt.Errorf("验证失败 - 字段: %s, 值: %v, 原因: %s", field, value, reason)
}

// NewCalculationError 创建计算错误
func NewCalculationError(operation string, details string) error {
	return fmt.Errorf("计算错误 - 操作: %s, 详情: %s", operation, details)
}

// ===============================
// Token信息提取工具函数
// ===============================

// ExtractTokenInfo 从 TxOutput 提取 Token 信息
func ExtractTokenInfo(output *pbtx.TxOutput) (TokenKey, *big.Int, error) {
	if output == nil {
		return "", big.NewInt(0), fmt.Errorf("输出不能为nil")
	}

	switch outputContent := output.OutputContent.(type) {
	case *pbtx.TxOutput_Asset:
		return extractFromAssetOutput(outputContent.Asset)
	case *pbtx.TxOutput_Resource:
		// 资源输出不产生费用
		return "", big.NewInt(0), fmt.Errorf("资源输出不参与费用计算")
	case *pbtx.TxOutput_State:
		// 状态输出不产生费用
		return "", big.NewInt(0), fmt.Errorf("状态输出不参与费用计算")
	default:
		return "", big.NewInt(0), fmt.Errorf("未知输出类型")
	}
}

// extractFromAssetOutput 从资产输出提取Token信息
func extractFromAssetOutput(asset *pbtx.AssetOutput) (TokenKey, *big.Int, error) {
	if asset == nil {
		return "", big.NewInt(0), fmt.Errorf("资产输出不能为nil")
	}

	switch assetContent := asset.AssetContent.(type) {
	case *pbtx.AssetOutput_NativeCoin:
		return extractFromNativeCoin(assetContent.NativeCoin)
	case *pbtx.AssetOutput_ContractToken:
		return extractFromContractToken(assetContent.ContractToken)
	default:
		return "", big.NewInt(0), fmt.Errorf("未知资产类型")
	}
}

// extractFromNativeCoin 从原生币输出提取信息
func extractFromNativeCoin(nativeCoin *pbtx.NativeCoinAsset) (TokenKey, *big.Int, error) {
	if nativeCoin == nil {
		return "", big.NewInt(0), fmt.Errorf("原生币资产不能为nil")
	}

	amount := ParseAmountString(nativeCoin.Amount)
	if err := ValidateAmount(amount); err != nil {
		return "", big.NewInt(0), WrapError(err, "原生币金额无效")
	}

	return GenerateNativeTokenKey(), amount, nil
}

// extractFromContractToken 从合约代币输出提取信息
func extractFromContractToken(contractToken *pbtx.ContractTokenAsset) (TokenKey, *big.Int, error) {
	if contractToken == nil {
		return "", big.NewInt(0), fmt.Errorf("合约代币资产不能为nil")
	}

	amount := ParseAmountString(contractToken.Amount)
	if err := ValidateAmount(amount); err != nil {
		return "", big.NewInt(0), WrapError(err, "合约代币金额无效")
	}

	tokenKey := GenerateContractTokenKey(contractToken.ContractAddress, "", contractToken.ContractAddress)

	return tokenKey, amount, nil
}

// ===============================
// 调试工具
// ===============================

// FormatAmount 格式化big.Int为可读字符串
func FormatAmount(amount *big.Int) string {
	if amount == nil {
		return "0"
	}
	return amount.String()
}

// FormatTokenMap 格式化Token映射为可读字符串
func FormatTokenMap(tokenMap map[TokenKey]*big.Int) string {
	if len(tokenMap) == 0 {
		return "{}"
	}

	var parts []string
	sortedKeys := SortTokenKeysFromMap(tokenMap)

	for _, key := range sortedKeys {
		amount := tokenMap[key]
		if IsPositive(amount) {
			parts = append(parts, fmt.Sprintf("%s: %s", key, FormatAmount(amount)))
		}
	}

	if len(parts) == 0 {
		return "{}"
	}

	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

// ===============================
// 常量定义
// ===============================

const (
	// BasisPointsDivisor 基点除数（用于比例计算）
	BasisPointsDivisor = 10000

	// MaxTokenKeyLength Token标识符的最大长度
	MaxTokenKeyLength = 256

	// DefaultExecutionFeeLimit 默认执行费用限制
	DefaultExecutionFeeLimit = 21000
)

// ===============================
// 类型检查工具
// ===============================

// IsValidTokenKey 检查TokenKey是否有效
func IsValidTokenKey(tokenKey TokenKey) bool {
	if len(tokenKey) == 0 || len(tokenKey) > MaxTokenKeyLength {
		return false
	}

	if tokenKey == NativeTokenKey {
		return true
	}

	// 检查格式是否符合 "contract|type|id" 的模式
	parts := strings.Split(string(tokenKey), "|")
	return len(parts) >= 3
}

// GetBasisPointsDivisor 获取基点除数
func GetBasisPointsDivisor() *big.Int {
	return big.NewInt(BasisPointsDivisor)
}
