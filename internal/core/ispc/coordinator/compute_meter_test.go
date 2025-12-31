package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// ComputeMeter 单元测试
// ============================================================================
//
// 🎯 **测试目的**：验证 CU 计算逻辑的正确性和一致性
//
// ============================================================================

// TestDefaultComputeMeter_GetComplexityFactor 测试获取复杂度系数
func TestDefaultComputeMeter_GetComplexityFactor(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	tests := []struct {
		name         string
		rType        ResourceType
		resourceHash []byte
		expected     float64
	}{
		{
			name:         "合约资源默认复杂度",
			rType:        ResourceTypeContract,
			resourceHash: []byte{0x12, 0x34, 0x56},
			expected:     1.0,
		},
		{
			name:         "AI模型资源默认复杂度",
			rType:        ResourceTypeAIModel,
			resourceHash: []byte{0x78, 0x9a, 0xbc},
			expected:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor, err := meter.GetComplexityFactor(ctx, tt.rType, tt.resourceHash)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, factor, "复杂度系数应该为默认值 1.0")
		})
	}
}

// TestDefaultComputeMeter_CalculateCU_Contract 测试合约 CU 计算
func TestDefaultComputeMeter_CalculateCU_Contract(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	tests := []struct {
		name          string
		inputSize     uint64
		execTimeMs    uint64
		ops           OperationStats
		expectedMin   float64
		expectedMax   float64
		description   string
	}{
		{
			name:        "最小CU（基础值）",
			inputSize:   0,
			execTimeMs:  0,
			ops:         OperationStats{},
			expectedMin: 1.0,
			expectedMax: 1.0,
			description: "只有基础 CU，无输入和时间贡献",
		},
		{
			name:        "小输入短时间",
			inputSize:   1024,  // 1 KB
			execTimeMs:  100,   // 100ms
			ops:         OperationStats{},
			expectedMin: 2.1,   // 1.0 (base) + 0.1 (input) + 1.0 (time) = 2.1
			expectedMax: 2.1,
			description: "1KB输入，100ms执行时间",
		},
		{
			name:        "大输入长时间",
			inputSize:   10240, // 10 KB
			execTimeMs:  500,  // 500ms
			ops:         OperationStats{},
			expectedMin: 6.0,  // 1.0 (base) + 1.0 (input) + 5.0 (time) = 7.0
			expectedMax: 7.0,
			description: "10KB输入，500ms执行时间",
		},
		{
			name:        "包含存储操作",
			inputSize:   2048,  // 2 KB
			execTimeMs:  200,  // 200ms
			ops:         OperationStats{StorageOps: 5},
			expectedMin: 4.7,  // 1.0 (base) + 0.2 (input) + 2.0 (time) + 2.5 (storage) = 5.7
			expectedMax: 5.7,
			description: "包含5次存储操作",
		},
		{
			name:        "包含跨合约调用",
			inputSize:   1024,
			execTimeMs:  100,
			ops:         OperationStats{CrossContractCalls: 2},
			expectedMin: 5.1,  // 1.0 (base) + 0.1 (input) + 1.0 (time) + 4.0 (calls) = 6.1
			expectedMax: 6.1,
			description: "包含2次跨合约调用",
		},
		{
			name:        "完整操作统计",
			inputSize:   2048,
			execTimeMs:  300,
			ops:         OperationStats{StorageOps: 3, CrossContractCalls: 1},
			expectedMin: 6.8,  // 1.0 (base) + 0.2 (input) + 3.0 (time) + 1.5 (storage) + 2.0 (calls) = 7.7
			expectedMax: 7.7,
			description: "包含存储操作和跨合约调用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceHash := []byte{0x12, 0x34, 0x56}
			cu, err := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, tt.inputSize, tt.execTimeMs, tt.ops)
			require.NoError(t, err, "计算 CU 不应该出错")
			assert.GreaterOrEqual(t, cu, tt.expectedMin, "%s: CU 应该 >= %.2f", tt.description, tt.expectedMin)
			assert.LessOrEqual(t, cu, tt.expectedMax, "%s: CU 应该 <= %.2f", tt.description, tt.expectedMax)
			assert.GreaterOrEqual(t, cu, 0.0, "CU 应该 >= 0")
		})
	}
}

// TestDefaultComputeMeter_CalculateCU_AIModel 测试 AI 模型 CU 计算
func TestDefaultComputeMeter_CalculateCU_AIModel(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	tests := []struct {
		name          string
		inputSize     uint64
		execTimeMs    uint64
		ops           OperationStats
		expectedMin   float64
		expectedMax   float64
		description   string
	}{
		{
			name:        "最小CU（基础值）",
			inputSize:   0,
			execTimeMs:  0,
			ops:         OperationStats{},
			expectedMin: 2.0, // AI 模型基础 CU 为 2.0
			expectedMax: 2.0,
			description: "只有基础 CU，无输入和时间贡献",
		},
		{
			name:        "小输入短时间",
			inputSize:   2048,  // 2 KB
			execTimeMs:  200,   // 200ms
			ops:         OperationStats{},
			expectedMin: 4.2,   // 2.0 (base) + 0.2 (input) + 2.0 (time) = 4.2
			expectedMax: 4.2,
			description: "2KB输入，200ms执行时间",
		},
		{
			name:        "大输入长时间",
			inputSize:   10240, // 10 KB
			execTimeMs:  1000, // 1秒
			ops:         OperationStats{},
			expectedMin: 13.0, // 2.0 (base) + 1.0 (input) + 10.0 (time) = 13.0
			expectedMax: 13.0,
			description: "10KB输入，1秒执行时间",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceHash := []byte{0x78, 0x9a, 0xbc}
			cu, err := meter.CalculateCU(ctx, ResourceTypeAIModel, resourceHash, tt.inputSize, tt.execTimeMs, tt.ops)
			require.NoError(t, err, "计算 CU 不应该出错")
			assert.GreaterOrEqual(t, cu, tt.expectedMin, "%s: CU 应该 >= %.2f", tt.description, tt.expectedMin)
			assert.LessOrEqual(t, cu, tt.expectedMax, "%s: CU 应该 <= %.2f", tt.description, tt.expectedMax)
			assert.GreaterOrEqual(t, cu, 0.0, "CU 应该 >= 0")
		})
	}
}

// TestDefaultComputeMeter_CalculateCU_InvalidResourceType 测试无效资源类型
func TestDefaultComputeMeter_CalculateCU_InvalidResourceType(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	invalidType := ResourceType(999)
	resourceHash := []byte{0x12, 0x34, 0x56}

	cu, err := meter.CalculateCU(ctx, invalidType, resourceHash, 1024, 100, OperationStats{})
	assert.Error(t, err, "无效资源类型应该返回错误")
	assert.Equal(t, 0.0, cu, "错误时 CU 应该为 0")
	assert.Contains(t, err.Error(), "不支持的资源类型", "错误信息应该包含类型错误")
}

// TestDefaultComputeMeter_CalculateCU_Deterministic 测试 CU 计算的确定性
func TestDefaultComputeMeter_CalculateCU_Deterministic(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	resourceHash := []byte{0x12, 0x34, 0x56}
	inputSize := uint64(2048)
	execTimeMs := uint64(300)
	ops := OperationStats{StorageOps: 2, CrossContractCalls: 1}

	// 多次计算应该得到相同的结果
	cu1, err1 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, inputSize, execTimeMs, ops)
	require.NoError(t, err1)

	cu2, err2 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, inputSize, execTimeMs, ops)
	require.NoError(t, err2)

	assert.Equal(t, cu1, cu2, "相同输入应该产生相同的 CU 值（确定性）")
}

// TestDefaultComputeMeter_CalculateCU_Monotonic 测试 CU 计算的单调性
func TestDefaultComputeMeter_CalculateCU_Monotonic(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	resourceHash := []byte{0x12, 0x34, 0x56}
	ops := OperationStats{}

	// 测试输入大小增加时 CU 应该增加
	cu1, err1 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, 1024, 100, ops)
	require.NoError(t, err1)

	cu2, err2 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, 2048, 100, ops)
	require.NoError(t, err2)

	assert.Greater(t, cu2, cu1, "输入大小增加时 CU 应该增加")

	// 测试执行时间增加时 CU 应该增加
	cu3, err3 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, 1024, 200, ops)
	require.NoError(t, err3)

	assert.Greater(t, cu3, cu1, "执行时间增加时 CU 应该增加")
}

// TestDefaultComputeMeter_CalculateCUFromExecution 测试便捷方法
func TestDefaultComputeMeter_CalculateCUFromExecution(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	resourceHash := []byte{0x12, 0x34, 0x56}
	inputSize := uint64(1024)
	ops := OperationStats{}

	startTime := time.Now()
	endTime := startTime.Add(100 * time.Millisecond) // 100ms 后

	cu, err := meter.CalculateCUFromExecution(ctx, ResourceTypeContract, resourceHash, inputSize, startTime, endTime, ops)
	require.NoError(t, err)
	assert.Greater(t, cu, 0.0, "CU 应该 > 0")

	// 验证与直接调用 CalculateCU 的结果一致（允许小的时间误差）
	expectedCU, err2 := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, inputSize, 100, ops)
	require.NoError(t, err2)
	// 由于时间计算可能有微小误差，允许 0.01 的差异
	assert.InDelta(t, expectedCU, cu, 0.01, "便捷方法应该与直接调用结果一致（允许微小误差）")
}

// TestDefaultComputeMeter_CalculateCU_ZeroInput 测试零输入情况
func TestDefaultComputeMeter_CalculateCU_ZeroInput(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	resourceHash := []byte{0x12, 0x34, 0x56}
	ops := OperationStats{}

	// 零输入、零时间应该只返回基础 CU
	cu, err := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, 0, 0, ops)
	require.NoError(t, err)
	assert.Equal(t, 1.0, cu, "零输入零时间应该返回基础 CU 1.0")

	// AI 模型零输入零时间应该返回基础 CU 2.0
	cuAI, err2 := meter.CalculateCU(ctx, ResourceTypeAIModel, resourceHash, 0, 0, ops)
	require.NoError(t, err2)
	assert.Equal(t, 2.0, cuAI, "AI 模型零输入零时间应该返回基础 CU 2.0")
}

// TestDefaultComputeMeter_CalculateCU_LargeValues 测试大值情况
func TestDefaultComputeMeter_CalculateCU_LargeValues(t *testing.T) {
	logger := testutil.NewTestLogger()
	meter := NewDefaultComputeMeter(logger)
	ctx := context.Background()

	resourceHash := []byte{0x12, 0x34, 0x56}
	ops := OperationStats{}

	// 测试非常大的输入和时间
	largeInputSize := uint64(100 * 1024 * 1024) // 100 MB
	largeExecTime := uint64(60000)              // 60 秒

	cu, err := meter.CalculateCU(ctx, ResourceTypeContract, resourceHash, largeInputSize, largeExecTime, ops)
	require.NoError(t, err)
	assert.Greater(t, cu, 0.0, "大值情况下 CU 应该 > 0")
	assert.Greater(t, cu, 100.0, "大值情况下 CU 应该显著增加")
}

// TestResourceType_String 测试 ResourceType 的字符串表示
func TestResourceType_String(t *testing.T) {
	tests := []struct {
		rType     ResourceType
		expected  string
	}{
		{ResourceTypeContract, "CONTRACT"},
		{ResourceTypeAIModel, "AI_MODEL"},
		{ResourceType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rType.String())
		})
	}
}

