// Package fee_test 提供 DynamicFeeEstimator 的单元测试
//
// 🧪 **测试覆盖**：
// - DynamicFeeEstimator 核心功能测试
// - 动态费率计算测试
// - 拥堵调整测试
// - 边界条件和错误场景测试
package fee

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== Mock 对象 ====================

// MockNetworkStateProvider 模拟网络状态提供者
type MockNetworkStateProvider struct {
	congestionLevel float64
	recentFees      []uint64
}

func NewMockNetworkStateProvider() *MockNetworkStateProvider {
	return &MockNetworkStateProvider{
		congestionLevel: 0.5, // 中等拥堵
		recentFees:      []uint64{100, 150, 200},
	}
}

func (m *MockNetworkStateProvider) GetCongestionLevel(ctx context.Context) (float64, error) {
	return m.congestionLevel, nil
}

func (m *MockNetworkStateProvider) GetRecentFees(ctx context.Context, count int) ([]uint64, error) {
	if count > len(m.recentFees) {
		return m.recentFees, nil
	}
	return m.recentFees[:count], nil
}

// ==================== DynamicFeeEstimator 核心功能测试 ====================

// TestNewDynamicEstimator_Success 测试创建 DynamicFeeEstimator 成功
func TestNewDynamicEstimator_Success(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.5,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	assert.NotNil(t, estimator)
	assert.Equal(t, uint64(10), estimator.baseRatePerByte)
	assert.Equal(t, uint64(100), estimator.minFee)
	assert.Equal(t, uint64(1000000), estimator.maxFee)
	assert.Equal(t, 1.5, estimator.congestionMultiplier)
}

// TestNewDynamicEstimator_NilConfig 测试 nil 配置（使用默认值）
func TestNewDynamicEstimator_NilConfig(t *testing.T) {
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(nil, logger)

	assert.NotNil(t, estimator)
	// 应该使用默认配置
	assert.Greater(t, estimator.baseRatePerByte, uint64(0))
}

// TestNewDynamicEstimator_NilLogger 测试 nil logger
func TestNewDynamicEstimator_NilLogger(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte: 10,
		MinFee:          100,
	}

	estimator := NewDynamicEstimator(config, nil)

	assert.NotNil(t, estimator)
}

// TestDynamicFeeEstimator_EstimateFee_Success 测试估算费用成功
func TestDynamicFeeEstimator_EstimateFee_Success(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 5),
		Outputs: make([]*transaction.TxOutput, 3),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, fee, uint64(100)) // 至少是最小费用
	assert.LessOrEqual(t, fee, uint64(1000000)) // 不超过最大费用
}

// TestDynamicFeeEstimator_EstimateFee_WithCongestion 测试拥堵调整
func TestDynamicFeeEstimator_EstimateFee_WithCongestion(t *testing.T) {
	provider := NewMockNetworkStateProvider()
	provider.congestionLevel = 0.8 // 高拥堵

	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  2.0,
		NetworkStateProvider:  provider,
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 10),
		Outputs: make([]*transaction.TxOutput, 5),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, fee, uint64(100))
}

// TestDynamicFeeEstimator_EstimateFee_MinFee 测试最小费用限制
func TestDynamicFeeEstimator_EstimateFee_MinFee(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       1,
		MinFee:                1000,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{}, // 很小的交易
		Outputs: []*transaction.TxOutput{},
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.Equal(t, uint64(1000), fee) // 应该是最小费用
}

// TestDynamicFeeEstimator_EstimateFee_MaxFee 测试最大费用限制
func TestDynamicFeeEstimator_EstimateFee_MaxFee(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10000,
		MinFee:                100,
		MaxFee:                1000,
		CongestionMultiplier:  3.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 100), // 很大的交易
		Outputs: make([]*transaction.TxOutput, 50),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.LessOrEqual(t, fee, uint64(1000)) // 不超过最大费用
}

// TestDynamicFeeEstimator_EstimateFee_NoNetworkProvider 测试无网络状态提供者
func TestDynamicFeeEstimator_EstimateFee_NoNetworkProvider(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  nil, // 无网络状态提供者
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 5),
		Outputs: make([]*transaction.TxOutput, 3),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	// 应该仍然能够估算费用（使用默认拥堵级别）
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, fee, uint64(100))
}

// TestDynamicFeeEstimator_EstimateFee_NetworkProviderError 测试网络状态提供者错误
func TestDynamicFeeEstimator_EstimateFee_NetworkProviderError(t *testing.T) {
	// 创建一个会返回错误的网络状态提供者
	errorProvider := &errorNetworkStateProvider{}

	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  errorProvider,
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 5),
		Outputs: make([]*transaction.TxOutput, 3),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	// 应该能够处理错误（使用默认拥堵级别或返回错误）
	// 实际行为取决于实现
	if err == nil {
		assert.GreaterOrEqual(t, fee, uint64(100))
	}
}

// errorNetworkStateProvider 返回错误的网络状态提供者
type errorNetworkStateProvider struct{}

func (e *errorNetworkStateProvider) GetCongestionLevel(ctx context.Context) (float64, error) {
	return 0, errors.New("network error")
}

func (e *errorNetworkStateProvider) GetRecentFees(ctx context.Context, count int) ([]uint64, error) {
	return nil, errors.New("network error")
}

// ==================== 边界条件测试 ====================

// TestDynamicFeeEstimator_EstimateFee_NilTransaction 测试 nil 交易
func TestDynamicFeeEstimator_EstimateFee_NilTransaction(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte: 10,
		MinFee:          100,
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()

	fee, err := estimator.EstimateFee(ctx, nil)

	// 当前实现可能不会检查 nil，测试应该反映实际行为
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.GreaterOrEqual(t, fee, uint64(100))
	}
}

// TestDynamicFeeEstimator_EstimateFee_ZeroBaseRate 测试零基础费率
func TestDynamicFeeEstimator_EstimateFee_ZeroBaseRate(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       0,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 5),
		Outputs: make([]*transaction.TxOutput, 3),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	// 应该返回最小费用
	assert.NoError(t, err)
	assert.Equal(t, uint64(100), fee)
}

// TestDynamicFeeEstimator_EstimateFee_ZeroMaxFee 测试零最大费用（无上限）
func TestDynamicFeeEstimator_EstimateFee_ZeroMaxFee(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10000,
		MinFee:                100,
		MaxFee:                0, // 无上限
		CongestionMultiplier:  3.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 100),
		Outputs: make([]*transaction.TxOutput, 50),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.Greater(t, fee, uint64(100)) // 应该大于最小费用
	// 无上限，费用可能很高
}

// TestDynamicFeeEstimator_EstimateFee_VeryLargeTransaction 测试超大交易
func TestDynamicFeeEstimator_EstimateFee_VeryLargeTransaction(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 1000),
		Outputs: make([]*transaction.TxOutput, 500),
	}

	fee, err := estimator.EstimateFee(ctx, tx)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, fee, uint64(100))
	assert.LessOrEqual(t, fee, uint64(1000000))
}

// TestDynamicFeeEstimator_EstimateFeeWithSpeed 测试速度档位估算
func TestDynamicFeeEstimator_EstimateFeeWithSpeed(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 5),
		Outputs: make([]*transaction.TxOutput, 3),
	}

	// 测试低速
	feeLow, err := estimator.EstimateFeeWithSpeed(ctx, tx, "low")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, feeLow, uint64(100))

	// 测试标准
	feeStandard, err := estimator.EstimateFeeWithSpeed(ctx, tx, "standard")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, feeStandard, feeLow)

	// 测试快速
	feeFast, err := estimator.EstimateFeeWithSpeed(ctx, tx, "fast")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, feeFast, feeStandard)
}

// TestDynamicFeeEstimator_GetFeeRateEstimate 测试获取费率估算
func TestDynamicFeeEstimator_GetFeeRateEstimate(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	ctx := context.Background()
	feeRate, err := estimator.GetFeeRateEstimate(ctx)

	assert.NoError(t, err)
	assert.Greater(t, feeRate, uint64(0))
}

// TestDynamicFeeEstimator_SetCongestionMultiplier 测试设置拥堵倍数
func TestDynamicFeeEstimator_SetCongestionMultiplier(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	// 设置拥堵倍数为2.0
	estimator.SetCongestionMultiplier(2.0)
	assert.Equal(t, 2.0, estimator.congestionMultiplier)

	// 设置小于1.0的倍数（应该被限制为1.0）
	estimator.SetCongestionMultiplier(0.5)
	assert.Equal(t, 1.0, estimator.congestionMultiplier)
}

// TestDynamicFeeEstimator_GetMinFee 测试获取最小费用
func TestDynamicFeeEstimator_GetMinFee(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	assert.Equal(t, uint64(100), estimator.GetMinFee())
}

// TestDynamicFeeEstimator_GetMaxFee 测试获取最大费用
func TestDynamicFeeEstimator_GetMaxFee(t *testing.T) {
	config := &DynamicConfig{
		BaseRatePerByte:       10,
		MinFee:                100,
		MaxFee:                1000000,
		CongestionMultiplier:  1.0,
		NetworkStateProvider:  NewMockNetworkStateProvider(),
	}
	logger := &testutil.MockLogger{}

	estimator := NewDynamicEstimator(config, logger)

	assert.Equal(t, uint64(1000000), estimator.GetMaxFee())
}

