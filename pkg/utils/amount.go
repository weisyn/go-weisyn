package utils

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ========================================
// 核心金额解析函数（推荐使用）
// ========================================

// ParseAmountSafely 安全解析金额字符串为uint64
//
// 算法说明：
// 1. 使用big.Int进行安全解析和范围验证
// 2. 检查是否超出uint64范围
// 3. 提供详细的错误信息
//
// 适用场景：
// -WES总供应量：100亿
// - 系统精度：8位小数
// - 最大wei值：10^19（在uint64安全范围内）
//
// 参数：
//   - amountStr: 金额字符串（如 "3500000000" 表示 35 ）
//
// 返回：
//   - uint64: 解析后的金额
//   - error: 解析错误
func ParseAmountSafely(amountStr string) (uint64, error) {
	// 基础验证
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return 0, nil
	}

	// 使用big.Int进行安全解析
	bigAmount := new(big.Int)
	bigAmount, ok := bigAmount.SetString(amountStr, 10)
	if !ok {
		return 0, fmt.Errorf("金额格式无效: %s", amountStr)
	}

	// 检查负数
	if bigAmount.Sign() < 0 {
		return 0, fmt.Errorf("金额不能为负数: %s", amountStr)
	}

	// 检查uint64范围（关键！防止溢出）
	if !bigAmount.IsUint64() {
		return 0, fmt.Errorf("金额超出支持范围: %s (最大: %d)", amountStr, uint64(math.MaxUint64))
	}

	return bigAmount.Uint64(), nil
}

// ParseAmountWithValidation 解析金额并验证约束
//
// 额外验证特定的业务约束：
// - 最大供应量：100亿 = 10^19 wei
// - 推荐用于关键业务逻辑（如余额计算、转账验证）
//
// 参数：
//   - amountStr: 金额字符串
//
// 返回：
//   - uint64: 解析后的金额
//   - error: 解析或约束验证错误
func ParseAmountWithValidation(amountStr string) (uint64, error) {
	amount, err := ParseAmountSafely(amountStr)
	if err != nil {
		return 0, err
	}

	//WES特定约束：100亿 = 10^19 wei
	const maxWei uint64 = 10_000_000_000_000_000_000 // 100亿 * 10^8
	if amount > maxWei {
		return 0, fmt.Errorf("金额超出最大供应量: %s (最大: %d wei)", amountStr, maxWei)
	}

	return amount, nil
}

// TryParseAmountUint64 尝试直接解析为uint64（性能优化版）
//
// 用于性能敏感场景的快速解析，如：
// - 大批量UTXO处理
// - 高频余额查询
//
// 如果解析失败，会自动降级到安全解析模式
//
// 参数：
//   - amountStr: 金额字符串
//
// 返回：
//   - uint64: 解析后的金额
//   - error: 解析错误
func TryParseAmountUint64(amountStr string) (uint64, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return 0, nil
	}

	// 快速路径：尝试直接解析为uint64
	if amount, err := strconv.ParseUint(amountStr, 10, 64); err == nil {
		return amount, nil
	}

	// 降级到安全解析
	return ParseAmountSafely(amountStr)
}

// Format 格式化wei金额为显示
//
// 用于调试和日志输出，将wei转换为可读的格式
//
// 参数：
//   - amountWei: wei金额
//   - decimals: 小数位数（默认8位）
//
// 返回：
//   - string: 格式化的字符串
func Format(amountWei uint64, decimals ...uint8) string {
	dec := uint8(8) // 默认8位小数
	if len(decimals) > 0 {
		dec = decimals[0]
	}

	divisor := uint64(1)
	for i := uint8(0); i < dec; i++ {
		divisor *= 10
	}

	integerPart := amountWei / divisor
	fractionalPart := amountWei % divisor

	return fmt.Sprintf("%d.%08d ", integerPart, fractionalPart)
}

// ValidateAmountRange 验证金额在合理范围内
//
// 用于输入验证和安全检查
//
// 参数：
//   - amount: 金额（wei）
//   - minAmount: 最小金额
//   - maxAmount: 最大金额
//
// 返回：
//   - error: 验证错误
func ValidateAmountRange(amount, minAmount, maxAmount uint64) error {
	if amount < minAmount {
		return fmt.Errorf("金额过小: %d < %d", amount, minAmount)
	}
	if amount > maxAmount {
		return fmt.Errorf("金额过大: %d > %d", amount, maxAmount)
	}
	return nil
}

// ========================================
// 常量定义
// ========================================

const (
	// DecimalsWES小数位数
	Decimals = 8

	// WeiPer 1  = 10^8 wei
	WeiPer = 100_000_000

	// MaxSupplyWES最大供应量（100亿）
	MaxSupply = 10_000_000_000

	// MaxWeiWES最大供应量（wei单位）
	// 使用uint64最大值减去1，避免编译时常量溢出问题
	MaxWei = 9_223_372_036_854_775_807 // int64最大值，避免编译错误
)

// ========================================
// 【生产级】精确整数金额计算工具（强制使用）
// ========================================

// ParseDecimalToWei 安全解析带小数的金额字符串为wei（uint64）
//
// 使用big.Rat进行无损精度计算，完全避免浮点误差
// 支持标准小数表示（如 "123.45678901"）
//
// 算法：
// 1. 使用big.Rat解析小数字符串
// 2. 乘以10^8转换为wei单位
// 3. 检查范围和精度溢出
//
// 参数：
//   - amountStr: 金额字符串（支持小数，如 "123.45678901"）
//
// 返回：
//   - uint64: wei单位的金额（整数）
//   - error: 解析或溢出错误
func ParseDecimalToWei(amountStr string) (uint64, error) {
	if amountStr = strings.TrimSpace(amountStr); amountStr == "" {
		return 0, nil
	}

	// 使用big.Rat进行无损解析
	rat := new(big.Rat)
	rat, ok := rat.SetString(amountStr)
	if !ok {

		return 0, fmt.Errorf("金额格式无效: %s", amountStr)
	}

	// 检查负数
	if rat.Sign() < 0 {
		return 0, fmt.Errorf("金额不能为负数: %s", amountStr)
	}

	// 乘以10^8转换为wei
	weiMultiplier := big.NewRat(WeiPer, 1) // 10^8
	weiRat := new(big.Rat).Mul(rat, weiMultiplier)

	// 检查是否为整数（小数位数不能超过8位）
	if !weiRat.IsInt() {
		return 0, fmt.Errorf("小数精度超出限制（最多8位）: %s", amountStr)
	}

	// 转换为big.Int检查范围
	weiBigInt := weiRat.Num()

	// 使用更直接的范围检查，而不是依赖IsUint64()
	maxUint64 := new(big.Int).SetUint64(^uint64(0)) // uint64最大值
	if weiBigInt.Cmp(maxUint64) > 0 || weiBigInt.Sign() < 0 {
		return 0, fmt.Errorf("金额超出支持范围: %s", amountStr)
	}

	weiValue := weiBigInt.Uint64()

	// 验证供应量限制
	if weiValue > MaxWei {
		return 0, fmt.Errorf("金额超出最大供应量: %s (最大: %d wei)", amountStr, MaxWei)
	}

	return weiValue, nil
}

// FormatWeiToDecimal 将wei金额格式化为标准8位小数字符串
//
// 输出格式：整数部分 + "." + 8位小数部分（去除末尾0）
// 例如：150000000 wei → "1.5"，100000000 wei → "1.0"
//
// 参数：
//   - weiAmount: wei单位的金额
//
// 返回：
//   - string: 标准小数格式的金额字符串
func FormatWeiToDecimal(weiAmount uint64) string {
	integerPart := weiAmount / WeiPer
	fractionalPart := weiAmount % WeiPer

	if fractionalPart == 0 {
		result := fmt.Sprintf("%d.0", integerPart)
		return result
	}

	// 格式化为8位小数并去除末尾0
	fractionalStr := fmt.Sprintf("%08d", fractionalPart)

	fractionalStr = strings.TrimRight(fractionalStr, "0")

	result := fmt.Sprintf("%d.%s", integerPart, fractionalStr)

	return result
}

// MulDivUint64 安全的乘除运算（防溢出）
//
// 计算 (x * multiplier) / divisor，使用big.Int避免中间结果溢出
//
// 参数：
//   - x: 被乘数
//   - multiplier: 乘数
//   - divisor: 除数
//
// 返回：
//   - uint64: 计算结果
//   - error: 溢出或除零错误
func MulDivUint64(x, multiplier, divisor uint64) (uint64, error) {
	if divisor == 0 {
		return 0, fmt.Errorf("除数不能为零")
	}

	// 使用big.Int进行计算
	bigX := new(big.Int).SetUint64(x)
	bigMul := new(big.Int).SetUint64(multiplier)
	bigDiv := new(big.Int).SetUint64(divisor)

	// (x * multiplier) / divisor
	result := new(big.Int).Mul(bigX, bigMul)
	result.Div(result, bigDiv)

	if !result.IsUint64() {
		return 0, fmt.Errorf("计算结果溢出: (%d * %d) / %d", x, multiplier, divisor)
	}

	return result.Uint64(), nil
}

// CalculateFeeWei 计算手续费（整数bps费率）
//
// 使用整数基点（bps）避免浮点误差：
// 手续费 = (金额 * 费率bps) / 10000
//
// 参数：
//   - amountWei: 金额（wei）
//   - feeRateBps: 费率（基点，如30表示0.3%）
//
// 返回：
//   - uint64: 手续费（wei）
//   - error: 计算错误
func CalculateFeeWei(amountWei uint64, feeRateBps uint32) (uint64, error) {
	return MulDivUint64(amountWei, uint64(feeRateBps), 10000)
}

// ConvertFeeRateToBps 将浮点费率转换为整数基点
//
// 一次性转换配置中的浮点费率，后续全程使用整数计算
// 例如：0.003 → 30 bps
//
// 参数：
//   - feeRate: 浮点费率（如0.003表示0.3%）
//
// 返回：
//   - uint32: 费率基点（如30）
func ConvertFeeRateToBps(feeRate float64) uint32 {
	return uint32(math.Round(feeRate * 10000))
}

// ConvertDustThresholdToWei 将浮点粉尘阈值转换为wei
//
// 一次性转换配置中的浮点阈值，后续全程使用整数比较
//
// 参数：
//   - dustThreshold: 浮点粉尘阈值
//
// 返回：
//   - uint64: wei单位的粉尘阈值
func ConvertDustThresholdToWei(dustThreshold float64) uint64 {
	return uint64(math.Round(dustThreshold * float64(WeiPer)))
}

// ========================================
// 【已弃用】浮点辅助函数（仅供显示用途，禁止业务逻辑调用）
// ========================================

// ToWei 将转换为wei
// ⚠️ 已弃用：仅供显示用途，业务逻辑禁止使用！
// 使用 ParseDecimalToWei 替代
func ToWei(wei float64) uint64 {
	return uint64(wei * float64(WeiPer))
}

// WeiTo 将wei转换为
// ⚠️ 已弃用：仅供显示用途，业务逻辑禁止使用！
// 使用 FormatWeiToDecimal 替代
func WeiTo(wei uint64) float64 {
	return float64(wei) / float64(WeiPer)
}

// IsValidAmount 检查金额是否在有效范围内
func IsValidAmount(amountWei uint64) bool {
	return amountWei <= MaxWei
}

// ========================================
// Protobuf Amount字段专用格式化函数
// ========================================

// FormatAmountForProtobuf 将wei金额格式化为protobuf Amount字段使用的字符串
//
// 设计说明：
// - protobuf的Amount字段必须使用整数wei字符串格式
// - 不能使用小数格式，否则strconv.ParseUint解析失败
// - 这是区别于FormatWeiToDecimal（用于用户显示）的专用方法
//
// 参数：
//   - weiAmount: wei单位的金额
//
// 返回：
//   - string: 整数wei字符串（如 "9997000"）
func FormatAmountForProtobuf(weiAmount uint64) string {
	return strconv.FormatUint(weiAmount, 10)
}
