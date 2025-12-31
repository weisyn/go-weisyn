// Package utils provides amount parsing and validation utility functions.
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
// - WES总供应量：100亿
// - 系统精度：8位小数（1 WES = 10^8 BaseUnit）
// - 最大 BaseUnit 值：10^18（在uint64安全范围内）
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
// - 最大供应量：100亿 WES = 10^18 BaseUnit
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

	// WES特定约束：100亿 * 10^8 = 10^18 BaseUnit //nolint:gocritic // commentFormatting: 已修复格式
	if amount > MaxSupplyWei {
		return 0, fmt.Errorf("金额超出最大供应量: %s (最大: 10^18 BaseUnit)", amountStr)
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

// Format 格式化 BaseUnit 金额为显示（用于调试/日志）
//
// 用于调试和日志输出，将BaseUnit转换为可读的格式
//
// 参数：
//   - amountWei: BaseUnit金额
//   - decimals: 小数位数（默认使用 Decimals）
//
// 返回：
//   - string: 格式化的字符串
func Format(amountWei uint64, decimals ...uint8) string {
	dec := uint8(Decimals) // 默认小数位
	if len(decimals) > 0 {
		dec = decimals[0]
	}

	divisor := uint64(1)
	for i := uint8(0); i < dec; i++ {
		divisor *= 10
	}

	integerPart := amountWei / divisor
	fractionalPart := amountWei % divisor

	// 动态构建格式字符串，根据 decimals 参数确定小数位数
	formatStr := fmt.Sprintf("%%d.%%0%dd", dec)
	return fmt.Sprintf(formatStr, integerPart, fractionalPart)
}

// ValidateAmountRange 验证金额在合理范围内
//
// 用于输入验证和安全检查
//
// 参数：
//   - amount: 金额（BaseUnit单位）
//   - minAmount: 最小金额（BaseUnit单位）
//   - maxAmount: 最大金额（BaseUnit单位）
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
	// Decimals WES小数位数（8位小数，1 WES = 10^8 BaseUnit）
	Decimals = 8
	// WeiPer 1 WES = 10^8 BaseUnit（支持uint64）
	WeiPer = 100_000_000
	// MaxSupply WES最大供应量（100亿）
	MaxSupply = 10_000_000_000
	// MaxSupplyWei WES最大供应量（BaseUnit），100亿 * 10^8 = 10^18 BaseUnit（在uint64范围内）
	MaxSupplyWei = 1_000_000_000_000_000_000
	// DustThresholdWei 统一粉尘阈值（BaseUnit），0.000001 WES = 0.000001 * 10^8 = 100 BaseUnit
	DustThresholdWei = 100
)

// ========================================
// 【生产级】精确整数金额计算工具（强制使用）
// ========================================

// ParseDecimalToWei 安全解析带小数的金额字符串为 BaseUnit（uint64）
//
// 使用big.Rat进行无损精度计算，完全避免浮点误差
// 支持标准小数表示（如 "123.456789"）
//
// 算法：
// 1. 使用big.Rat解析小数字符串
// 2. 乘以10^8转换为 BaseUnit（适配uint64）
// 3. 检查范围和精度溢出
//
// 参数：
//   - amountStr: 金额字符串（支持小数，最多9位小数）
//
// 返回：
//   - uint64: BaseUnit单位的金额（整数）
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

	// 乘以10^8转换为 BaseUnit
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
	if weiValue > MaxSupplyWei {
		return 0, fmt.Errorf("金额超出最大供应量: %s (最大: 10^18 BaseUnit)", amountStr)
	}

	return weiValue, nil
}

// FormatWeiToDecimal 将 BaseUnit 金额格式化为标准8位小数字符串
//
// 输出格式：整数部分 + "." + 8位小数部分（去除末尾0）
// 例如：150000000 BaseUnit → "1.5"，100000000 BaseUnit → "1.0"
//
// 参数：
//   - weiAmount: BaseUnit 金额（uint64，内部变量名保留历史命名）
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
