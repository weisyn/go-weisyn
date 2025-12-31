// Package builder provides transaction building functionality for client operations.
package builder

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Amount 表示WES金额（使用最小单位）
//
// WES金额系统：
//   - 1 WES = 10^8 基础单位（类似比特币的聪）
//   - 使用 *big.Int 确保精确计算，避免浮点数精度问题
//   - 支持超大金额（>10^18）
type Amount struct {
	value *big.Int // 最小单位（1 WES = 10^8 基础单位）
}

// 常量定义
const (
	// DecimalPlaces WES的小数位数
	DecimalPlaces = 8

	// UnitsPerWES 1 WES对应的基础单位数量
	UnitsPerWES = 100_000_000 // 10^8
)

var (
	// ErrInvalidAmount 无效的金额
	ErrInvalidAmount = errors.New("invalid amount")

	// ErrNegativeAmount 负数金额
	ErrNegativeAmount = errors.New("negative amount")

	// ErrInsufficientAmount 金额不足
	ErrInsufficientAmount = errors.New("insufficient amount")

	// unitsPerWES 预计算的big.Int
	unitsPerWES = big.NewInt(UnitsPerWES)
)

// NewAmount 从WES单位创建Amount
//
// 示例：
//
//	NewAmount(1.5) → 150000000 基础单位
//	NewAmount(0.00000001) → 1 基础单位
func NewAmount(wes float64) (*Amount, error) {
	if wes < 0 {
		return nil, ErrNegativeAmount
	}

	// 转换为最小单位: wes * 10^8
	base := new(big.Float).Mul(
		big.NewFloat(wes),
		new(big.Float).SetInt(unitsPerWES),
	)

	// 转换为big.Int（向下取整）
	value, _ := base.Int(nil)

	return &Amount{value: value}, nil
}

// NewAmountFromString 从字符串创建Amount
//
// 支持格式：
//   - "100" → 100基础单位
//   - "1.5" → 150000000基础单位（作为WES解析）
//   - "1.50000000" → 150000000基础单位
func NewAmountFromString(s string) (*Amount, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("%w: empty string", ErrInvalidAmount)
	}

	// 检查是否包含小数点
	if strings.Contains(s, ".") {
		// 作为WES单位解析
		wes, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidAmount, err)
		}
		return NewAmount(wes)
	}

	// 作为基础单位解析
	value, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAmount, s)
	}

	if value.Sign() < 0 {
		return nil, ErrNegativeAmount
	}

	return &Amount{value: value}, nil
}

// NewAmountFromUnits 从基础单位创建Amount
func NewAmountFromUnits(units uint64) *Amount {
	return &Amount{value: new(big.Int).SetUint64(units)}
}

// NewAmountFromBigInt 从big.Int创建Amount
func NewAmountFromBigInt(value *big.Int) (*Amount, error) {
	if value == nil {
		return nil, fmt.Errorf("%w: nil value", ErrInvalidAmount)
	}

	if value.Sign() < 0 {
		return nil, ErrNegativeAmount
	}

	// 复制value，避免外部修改
	return &Amount{value: new(big.Int).Set(value)}, nil
}

// Zero 返回零金额
func Zero() *Amount {
	return &Amount{value: big.NewInt(0)}
}

// Add 加法：a + b
func (a *Amount) Add(b *Amount) *Amount {
	if a == nil || b == nil {
		return Zero()
	}

	result := new(big.Int).Add(a.value, b.value)
	return &Amount{value: result}
}

// Sub 减法：a - b
// 如果结果为负数，返回错误
func (a *Amount) Sub(b *Amount) (*Amount, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("%w: nil amount", ErrInvalidAmount)
	}

	result := new(big.Int).Sub(a.value, b.value)

	if result.Sign() < 0 {
		return nil, ErrInsufficientAmount
	}

	return &Amount{value: result}, nil
}

// Mul 乘法：a * n
func (a *Amount) Mul(n int64) *Amount {
	if a == nil {
		return Zero()
	}

	result := new(big.Int).Mul(a.value, big.NewInt(n))
	return &Amount{value: result}
}

// Div 除法：a / n
func (a *Amount) Div(n int64) (*Amount, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: nil amount", ErrInvalidAmount)
	}

	if n == 0 {
		return nil, errors.New("division by zero")
	}

	result := new(big.Int).Div(a.value, big.NewInt(n))
	return &Amount{value: result}, nil
}

// Cmp 比较两个金额
// 返回值：
//
//	-1: a < b
//	 0: a == b
//	 1: a > b
func (a *Amount) Cmp(b *Amount) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	return a.value.Cmp(b.value)
}

// IsZero 判断金额是否为零
func (a *Amount) IsZero() bool {
	return a == nil || a.value.Sign() == 0
}

// IsPositive 判断金额是否为正
func (a *Amount) IsPositive() bool {
	return a != nil && a.value.Sign() > 0
}

// LessThan 判断 a < b
func (a *Amount) LessThan(b *Amount) bool {
	return a.Cmp(b) < 0
}

// LessThanOrEqual 判断 a <= b
func (a *Amount) LessThanOrEqual(b *Amount) bool {
	return a.Cmp(b) <= 0
}

// GreaterThan 判断 a > b
func (a *Amount) GreaterThan(b *Amount) bool {
	return a.Cmp(b) > 0
}

// GreaterThanOrEqual 判断 a >= b
func (a *Amount) GreaterThanOrEqual(b *Amount) bool {
	return a.Cmp(b) >= 0
}

// Equal 判断 a == b
func (a *Amount) Equal(b *Amount) bool {
	return a.Cmp(b) == 0
}

// Units 返回基础单位数量
func (a *Amount) Units() uint64 {
	if a == nil || !a.value.IsUint64() {
		return 0
	}
	return a.value.Uint64()
}

// BigInt 返回big.Int副本
func (a *Amount) BigInt() *big.Int {
	if a == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(a.value)
}

// ToWES 转换为WES单位（float64）
// 注意：大额金额可能损失精度
func (a *Amount) ToWES() float64 {
	if a == nil {
		return 0
	}

	wes := new(big.Float).Quo(
		new(big.Float).SetInt(a.value),
		new(big.Float).SetInt(unitsPerWES),
	)

	result, _ := wes.Float64()
	return result
}

// String 转换为WES单位字符串（保留8位小数）
//
// 示例：
//
//	150000000 → "1.50000000"
//	1 → "0.00000001"
//	100000000 → "1.00000000"
func (a *Amount) String() string {
	if a == nil {
		return "0.00000000"
	}

	wes := new(big.Float).Quo(
		new(big.Float).SetInt(a.value),
		new(big.Float).SetInt(unitsPerWES),
	)

	return wes.Text('f', DecimalPlaces)
}

// StringTrimmed 转换为WES单位字符串（移除末尾的0）
//
// 示例：
//
//	150000000 → "1.5"
//	1 → "0.00000001"
//	100000000 → "1"
func (a *Amount) StringTrimmed() string {
	str := a.String()
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	return str
}

// StringUnits 转换为基础单位字符串
//
// 示例：
//
//	150000000 → "150000000"
func (a *Amount) StringUnits() string {
	if a == nil {
		return "0"
	}
	return a.value.String()
}

// Copy 创建副本
func (a *Amount) Copy() *Amount {
	if a == nil {
		return Zero()
	}
	return &Amount{value: new(big.Int).Set(a.value)}
}

// SumAmounts 计算多个金额的总和
func SumAmounts(amounts ...*Amount) *Amount {
	total := Zero()
	for _, amt := range amounts {
		if amt != nil {
			total = total.Add(amt)
		}
	}
	return total
}

// MaxAmount 返回两个金额中较大的一个
func MaxAmount(a, b *Amount) *Amount {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

// MinAmount 返回两个金额中较小的一个
func MinAmount(a, b *Amount) *Amount {
	if a.LessThan(b) {
		return a
	}
	return b
}
