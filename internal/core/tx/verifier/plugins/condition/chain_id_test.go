// Package condition_test 提供 ChainIDPlugin 的单元测试
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
)

// ==================== ChainIDPlugin 测试 ====================

// TestNewChainIDPlugin 测试创建 ChainIDPlugin
func TestNewChainIDPlugin(t *testing.T) {
	chainID := []byte("test-chain-id")
	plugin := NewChainIDPlugin(chainID)

	assert.NotNil(t, plugin)
	assert.Equal(t, chainID, plugin.chainID)
}

// TestChainIDPlugin_Name 测试插件名称
func TestChainIDPlugin_Name(t *testing.T) {
	plugin := NewChainIDPlugin([]byte("test-chain-id"))

	assert.Equal(t, "chain_id", plugin.Name())
}

// TestChainIDPlugin_Check_NoChainID 测试交易没有设置 chain_id
func TestChainIDPlugin_Check_NoChainID(t *testing.T) {
	plugin := NewChainIDPlugin([]byte("test-chain-id"))

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = nil // 未设置 chain_id

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err) // 向后兼容，应该通过
}

// TestChainIDPlugin_Check_EmptyChainID 测试交易 chain_id 为空
func TestChainIDPlugin_Check_EmptyChainID(t *testing.T) {
	plugin := NewChainIDPlugin([]byte("test-chain-id"))

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte{} // 空 chain_id

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err) // 向后兼容，应该通过
}

// TestChainIDPlugin_Check_NoPluginChainID 测试插件没有配置 chain_id
func TestChainIDPlugin_Check_NoPluginChainID(t *testing.T) {
	plugin := NewChainIDPlugin(nil) // 插件未配置 chain_id

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte("any-chain-id")

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err) // 应该跳过验证
}

// TestChainIDPlugin_Check_Match 测试 chain_id 匹配
func TestChainIDPlugin_Check_Match(t *testing.T) {
	chainID := []byte("test-chain-id")
	plugin := NewChainIDPlugin(chainID)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = chainID // 匹配的 chain_id

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestChainIDPlugin_Check_Mismatch 测试 chain_id 不匹配
func TestChainIDPlugin_Check_Mismatch(t *testing.T) {
	chainID := []byte("test-chain-id")
	plugin := NewChainIDPlugin(chainID)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte("other-chain-id") // 不匹配的 chain_id

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chain_id 不匹配")
}

// TestChainIDPlugin_Check_DifferentLength 测试不同长度的 chain_id
func TestChainIDPlugin_Check_DifferentLength(t *testing.T) {
	chainID := []byte("test-chain-id")
	plugin := NewChainIDPlugin(chainID)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte("test-chain-id-extra") // 不同长度

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chain_id 不匹配")
}

// TestChainIDPlugin_Check_CaseSensitive 测试 chain_id 大小写敏感
func TestChainIDPlugin_Check_CaseSensitive(t *testing.T) {
	chainID := []byte("test-chain-id")
	plugin := NewChainIDPlugin(chainID)

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte("TEST-CHAIN-ID") // 不同大小写

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chain_id 不匹配")
}

// TestChainIDPlugin_Check_EmptyBoth 测试两者都为空
func TestChainIDPlugin_Check_EmptyBoth(t *testing.T) {
	plugin := NewChainIDPlugin([]byte{}) // 插件 chain_id 为空

	tx := testutil.CreateTransaction(nil, nil)
	tx.ChainId = []byte{} // 交易 chain_id 也为空

	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err) // 应该跳过验证
}

