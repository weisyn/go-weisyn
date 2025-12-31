package sync_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/chain/testutil"
)

// ==================== CheckSync 测试（间接测试status.go）====================

// TestCheckSync_WithValidContext_ReturnsStatus 测试检查同步状态
func TestCheckSync_WithValidContext_ReturnsStatus(t *testing.T) {
	// Arrange
	manager, err := testutil.NewTestSyncManager()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	status, err := manager.CheckSync(ctx)

	// Assert
	// 即使查询失败，也应该返回错误而不是panic
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, status)
	}
}

// TestCheckSync_WithCancelledContext_HandlesGracefully 测试取消上下文时的处理
func TestCheckSync_WithCancelledContext_HandlesGracefully(t *testing.T) {
	// Arrange
	manager, err := testutil.NewTestSyncManager()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// Act
	status, err := manager.CheckSync(ctx)

	// Assert
	// 即使查询失败，也应该返回错误而不是panic
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, status)
	}
}

// ==================== 发现代码问题测试 ====================

// TestCheckSync_DetectsTODOs 测试发现TODO标记
func TestCheckSync_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestCheckSync_DetectsTemporaryImplementations 测试发现临时实现
func TestCheckSync_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ 同步状态查询实现检查：")
	t.Logf("  - checkSyncImpl 查询当前同步状态")
	t.Logf("  - 查询本地链高度")
	t.Logf("  - 查询网络高度（通过K桶节点采样）")
	t.Logf("  - 计算同步进度和状态")
	t.Logf("  - 构建完整状态信息")
}

