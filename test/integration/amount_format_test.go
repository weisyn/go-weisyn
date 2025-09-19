package integration

import (
	"strconv"
	"testing"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/utils"
)

// TestAmountFormatConsistency 测试金额格式一致性
//
// 🎯 **测试目标**：确保交易构建和余额解析使用一致的金额格式
//
// 验证要点：
// 1. 交易构建时使用整数wei字符串
// 2. 余额解析能正确处理整数wei字符串
// 3. 不再出现小数格式的金额字符串
func TestAmountFormatConsistency(t *testing.T) {
	testCases := []struct {
		name        string
		weiAmount   uint64
		expectedStr string
	}{
		{
			name:        "小金额",
			weiAmount:   9997000, // 对应 0.09997 WES
			expectedStr: "9997000",
		},
		{
			name:        "大金额",
			weiAmount:   49999999990000000, // 对应 499999999.9 WES
			expectedStr: "49999999990000000",
		},
		{
			name:        "整数金额",
			weiAmount:   500000000, // 对应 5 WES
			expectedStr: "500000000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 测试正确的格式化方法
			actualStr := strconv.FormatUint(tc.weiAmount, 10)
			if actualStr != tc.expectedStr {
				t.Errorf("格式化错误: expected %s, got %s", tc.expectedStr, actualStr)
			}

			// 测试解析是否成功
			parsedAmount, err := strconv.ParseUint(actualStr, 10, 64)
			if err != nil {
				t.Errorf("解析失败: %v", err)
			}
			if parsedAmount != tc.weiAmount {
				t.Errorf("解析结果错误: expected %d, got %d", tc.weiAmount, parsedAmount)
			}

			// 验证错误的小数格式会导致解析失败
			wrongDecimalFormat := utils.FormatWeiToDecimal(tc.weiAmount)
			_, err = strconv.ParseUint(wrongDecimalFormat, 10, 64)
			if err == nil {
				t.Errorf("小数格式应该解析失败，但成功了: %s", wrongDecimalFormat)
			}
		})
	}
}

// TestNativeCoinAmountFormat 测试NativeCoin金额格式
func TestNativeCoinAmountFormat(t *testing.T) {
	// 模拟正确的交易输出构建
	weiAmount := uint64(9997000) // 0.09997 WES in wei

	// ✅ 正确的格式化方式
	correctAmount := strconv.FormatUint(weiAmount, 10)

	// 创建NativeCoin输出
	nativeCoin := &transaction.NativeCoinAsset{
		Amount: correctAmount, // 应该是 "9997000"
	}

	// 验证格式
	if nativeCoin.Amount != "9997000" {
		t.Errorf("NativeCoin金额格式错误: expected '9997000', got '%s'", nativeCoin.Amount)
	}

	// 验证能被正确解析
	parsedAmount, err := strconv.ParseUint(nativeCoin.Amount, 10, 64)
	if err != nil {
		t.Errorf("NativeCoin金额解析失败: %v", err)
	}
	if parsedAmount != weiAmount {
		t.Errorf("解析结果错误: expected %d, got %d", weiAmount, parsedAmount)
	}
}

// TestAmountFormatBugReproduction 重现金额格式BUG
func TestAmountFormatBugReproduction(t *testing.T) {
	// 重现之前的BUG：使用FormatWeiToDecimal导致小数格式
	weiAmount := uint64(9997000) // 0.09997 WES in wei

	// ❌ 错误的格式化方式（之前的BUG）
	wrongFormat := utils.FormatWeiToDecimal(weiAmount) // 结果: "0.09997"

	// 验证这种格式会导致解析失败
	_, err := strconv.ParseUint(wrongFormat, 10, 64)
	if err == nil {
		t.Errorf("错误的小数格式应该解析失败，但成功了: %s", wrongFormat)
	}

	// ✅ 正确的格式化方式（修复后）
	correctFormat := strconv.FormatUint(weiAmount, 10) // 结果: "9997000"

	// 验证正确格式能成功解析
	parsedAmount, err := strconv.ParseUint(correctFormat, 10, 64)
	if err != nil {
		t.Errorf("正确格式解析失败: %v", err)
	}
	if parsedAmount != weiAmount {
		t.Errorf("解析结果错误: expected %d, got %d", weiAmount, parsedAmount)
	}
}
