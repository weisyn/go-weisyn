// Package condition_test 提供 TimeWindowPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package condition

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== TimeWindowPlugin 测试 ====================

// TestNewTimeWindowPlugin 测试创建 TimeWindowPlugin
func TestNewTimeWindowPlugin(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	assert.NotNil(t, plugin)
}

// TestTimeWindowPlugin_Name 测试插件名称
func TestTimeWindowPlugin_Name(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	assert.Equal(t, "time_window", plugin.Name())
}

// TestTimeWindowPlugin_Check_NoTimeWindow 测试没有时间窗口
func TestTimeWindowPlugin_Check_NoTimeWindow(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	tx := testutil.CreateTransaction(nil, nil)
	// 不设置时间窗口

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestTimeWindowPlugin_Check_NotBeforeOnly 测试只有 not_before
func TestTimeWindowPlugin_Check_NotBeforeOnly(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	now := uint64(time.Now().Unix())
	notBefore := now - 3600 // 1小时前

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_TimeWindow{
		TimeWindow: &transaction.TimeBasedWindow{
			NotBeforeTimestamp: &notBefore,
		},
	}

	// 当前时间 >= notBefore，应该通过
	err := plugin.Check(context.Background(), tx, 100, now)
	assert.NoError(t, err)

	// 当前时间 < notBefore，应该失败
	err = plugin.Check(context.Background(), tx, 100, notBefore-1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")
}

// TestTimeWindowPlugin_Check_NotAfterOnly 测试只有 not_after
func TestTimeWindowPlugin_Check_NotAfterOnly(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	now := uint64(time.Now().Unix())
	notAfter := now + 3600 // 1小时后

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_TimeWindow{
		TimeWindow: &transaction.TimeBasedWindow{
			NotAfterTimestamp: &notAfter,
		},
	}

	// 当前时间 <= notAfter，应该通过
	err := plugin.Check(context.Background(), tx, 100, now)
	assert.NoError(t, err)

	// 当前时间 > notAfter，应该失败
	err = plugin.Check(context.Background(), tx, 100, notAfter+1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestTimeWindowPlugin_Check_BothNotBeforeAndNotAfter 测试同时设置 not_before 和 not_after
func TestTimeWindowPlugin_Check_BothNotBeforeAndNotAfter(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	now := uint64(time.Now().Unix())
	notBefore := now - 3600 // 1小时前
	notAfter := now + 3600  // 1小时后

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_TimeWindow{
		TimeWindow: &transaction.TimeBasedWindow{
			NotBeforeTimestamp: &notBefore,
			NotAfterTimestamp:  &notAfter,
		},
	}

	// 在窗口内，应该通过
	err := plugin.Check(context.Background(), tx, 100, now)
	assert.NoError(t, err)

	// 太早，应该失败
	err = plugin.Check(context.Background(), tx, 100, notBefore-1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")

	// 过期，应该失败
	err = plugin.Check(context.Background(), tx, 100, notAfter+1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestTimeWindowPlugin_Check_InvalidWindow 测试无效窗口（not_before > not_after）
// 注意：由于代码逻辑先检查 not_before 和 not_after，然后才检查窗口合法性，
// 当 notBefore > notAfter 时，任何时间都会先触发 "too early" 或 "expired" 错误
// 窗口合法性检查只有在 now >= notBefore 且 now <= notAfter 时才会执行
// 因此这个测试用例实际上无法触发 "invalid time window" 错误
// 但我们可以测试边界情况：当 now 正好等于 notBefore 时，由于 notBefore > notAfter，会返回 "expired"
func TestTimeWindowPlugin_Check_InvalidWindow(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	now := uint64(time.Now().Unix())
	notBefore := now + 3600 // 1小时后
	notAfter := now - 3600  // 1小时前（无效：notBefore > notAfter）

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_TimeWindow{
		TimeWindow: &transaction.TimeBasedWindow{
			NotBeforeTimestamp: &notBefore,
			NotAfterTimestamp:  &notAfter,
		},
	}

	// 由于代码先检查 not_before，当 now < notBefore 时返回 "too early"
	err := plugin.Check(context.Background(), tx, 100, now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")

	// 当 now >= notBefore 时，由于 notBefore > notAfter，now > notAfter，返回 "expired"
	err = plugin.Check(context.Background(), tx, 100, notBefore)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestTimeWindowPlugin_Check_ExactBoundary 测试边界值
func TestTimeWindowPlugin_Check_ExactBoundary(t *testing.T) {
	plugin := NewTimeWindowPlugin()

	now := uint64(time.Now().Unix())
	notBefore := now
	notAfter := now

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_TimeWindow{
		TimeWindow: &transaction.TimeBasedWindow{
			NotBeforeTimestamp: &notBefore,
			NotAfterTimestamp:  &notAfter,
		},
	}

	// 正好在边界上，应该通过
	err := plugin.Check(context.Background(), tx, 100, now)
	assert.NoError(t, err)
}

