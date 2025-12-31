package incremental

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// verifier.go 测试补充 - min函数
// ============================================================================

// TestMin 测试min辅助函数
func TestMin(t *testing.T) {
	// 测试min函数的基本功能
	result := min(5, 10)
	require.Equal(t, 5, result)
	
	result = min(10, 5)
	require.Equal(t, 5, result)
	
	result = min(5, 5)
	require.Equal(t, 5, result)
	
	result = min(0, 10)
	require.Equal(t, 0, result)
	
	result = min(-5, 10)
	require.Equal(t, -5, result)
	
	result = min(10, -5)
	require.Equal(t, -5, result)
	
	result = min(100, 50)
	require.Equal(t, 50, result)
	
	result = min(1, 1)
	require.Equal(t, 1, result)
}

