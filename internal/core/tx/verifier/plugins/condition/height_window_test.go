// Package condition_test 提供 HeightWindowPlugin 的单元测试
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

// ==================== HeightWindowPlugin 测试 ====================

// TestNewHeightWindowPlugin 测试创建 HeightWindowPlugin
func TestNewHeightWindowPlugin(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	assert.NotNil(t, plugin)
}

// TestHeightWindowPlugin_Name 测试插件名称
func TestHeightWindowPlugin_Name(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	assert.Equal(t, "height_window", plugin.Name())
}

// TestHeightWindowPlugin_Check_NoHeightWindow 测试没有高度窗口
func TestHeightWindowPlugin_Check_NoHeightWindow(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	tx := testutil.CreateTransaction(nil, nil)
	// 不设置高度窗口

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestHeightWindowPlugin_Check_NotBeforeOnly 测试只有 not_before
func TestHeightWindowPlugin_Check_NotBeforeOnly(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	currentHeight := uint64(100)
	notBefore := uint64(50)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_HeightWindow{
		HeightWindow: &transaction.HeightBasedWindow{
			NotBeforeHeight: &notBefore,
		},
	}

	// 当前高度 >= notBefore，应该通过
	err := plugin.Check(context.Background(), tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 当前高度 < notBefore，应该失败
	err = plugin.Check(context.Background(), tx, notBefore-1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")
}

// TestHeightWindowPlugin_Check_NotAfterOnly 测试只有 not_after
func TestHeightWindowPlugin_Check_NotAfterOnly(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	currentHeight := uint64(100)
	notAfter := uint64(150)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_HeightWindow{
		HeightWindow: &transaction.HeightBasedWindow{
			NotAfterHeight: &notAfter,
		},
	}

	// 当前高度 <= notAfter，应该通过
	err := plugin.Check(context.Background(), tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 当前高度 > notAfter，应该失败
	err = plugin.Check(context.Background(), tx, notAfter+1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestHeightWindowPlugin_Check_BothNotBeforeAndNotAfter 测试同时设置 not_before 和 not_after
func TestHeightWindowPlugin_Check_BothNotBeforeAndNotAfter(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	currentHeight := uint64(100)
	notBefore := uint64(50)
	notAfter := uint64(150)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_HeightWindow{
		HeightWindow: &transaction.HeightBasedWindow{
			NotBeforeHeight: &notBefore,
			NotAfterHeight:  &notAfter,
		},
	}

	// 在窗口内，应该通过
	err := plugin.Check(context.Background(), tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 太早，应该失败
	err = plugin.Check(context.Background(), tx, notBefore-1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")

	// 过期，应该失败
	err = plugin.Check(context.Background(), tx, notAfter+1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestHeightWindowPlugin_Check_InvalidWindow 测试无效窗口（not_before > not_after）
// 注意：由于代码逻辑先检查 not_before 和 not_after，然后才检查窗口合法性，
// 当 notBefore > notAfter 时，任何高度都会先触发 "too early" 或 "expired" 错误
// 窗口合法性检查只有在 currentHeight >= notBefore 且 currentHeight <= notAfter 时才会执行
func TestHeightWindowPlugin_Check_InvalidWindow(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	currentHeight := uint64(100)
	notBefore := uint64(150)
	notAfter := uint64(50) // 无效：notBefore > notAfter

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_HeightWindow{
		HeightWindow: &transaction.HeightBasedWindow{
			NotBeforeHeight: &notBefore,
			NotAfterHeight:  &notAfter,
		},
	}

	// 由于代码先检查 not_before，当 currentHeight < notBefore 时返回 "too early"
	err := plugin.Check(context.Background(), tx, currentHeight, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too early")

	// 当 currentHeight >= notBefore 时，由于 notBefore > notAfter，currentHeight > notAfter，返回 "expired"
	err = plugin.Check(context.Background(), tx, notBefore, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestHeightWindowPlugin_Check_ExactBoundary 测试边界值
func TestHeightWindowPlugin_Check_ExactBoundary(t *testing.T) {
	plugin := NewHeightWindowPlugin()

	height := uint64(100)
	notBefore := height
	notAfter := height

	tx := testutil.CreateTransaction(nil, nil)
	tx.ValidityWindow = &transaction.Transaction_HeightWindow{
		HeightWindow: &transaction.HeightBasedWindow{
			NotBeforeHeight: &notBefore,
			NotAfterHeight:  &notAfter,
		},
	}

	// 正好在边界上，应该通过
	err := plugin.Check(context.Background(), tx, height, uint64(time.Now().Unix()))
	assert.NoError(t, err)
}

