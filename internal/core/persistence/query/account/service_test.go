// Package account 提供账户查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 AccountQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 账户余额查询
// - 代币过滤
// - UTXO状态分类
package account

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/internal/core/persistence/testutil"
	txtestutil "github.com/weisyn/v1/internal/core/tx/testutil"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, utxoQuery, logger)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsError 测试使用 nil storage 创建服务
func TestNewService_WithNilStorage_ReturnsError(t *testing.T) {
	// Arrange
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(nil, utxoQuery, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "storage 不能为空")
}

// TestNewService_WithNilUTXOQuery_ReturnsError 测试使用 nil utxoQuery 创建服务
func TestNewService_WithNilUTXOQuery_ReturnsError(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, nil, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "utxoQuery 不能为空")
}

// ==================== 账户余额查询测试 ====================

// TestGetAccountBalance_WithNoUTXOs_ReturnsZeroBalance 测试无UTXO时返回零余额
func TestGetAccountBalance_WithNoUTXOs_ReturnsZeroBalance(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, utxoQuery, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()
	tokenID := testutil.RandomHash()

	// Act
	balance, err := service.GetAccountBalance(ctx, address, tokenID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, uint64(0), balance.Total)
	assert.Equal(t, uint64(0), balance.Available)
	assert.Equal(t, uint64(0), balance.Locked)
}

// TestGetAccountBalance_WithNativeCoinUTXO_ReturnsBalance 测试原生代币UTXO余额查询
func TestGetAccountBalance_WithNativeCoinUTXO_ReturnsBalance(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	
	// 创建自定义的 UTXOQuery Mock，返回包含原生代币的 UTXO
	utxoQuery := &mockUTXOQueryWithData{
		utxos: []*utxo.UTXO{
			txtestutil.CreateUTXO(nil, txtestutil.CreateNativeCoinOutput(nil, "100", nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE),
			txtestutil.CreateUTXO(nil, txtestutil.CreateNativeCoinOutput(nil, "50", nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED),
		},
	}
	
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, utxoQuery, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()
	var tokenID []byte // nil 表示原生代币

	// Act
	balance, err := service.GetAccountBalance(ctx, address, tokenID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, uint64(150), balance.Total)
	assert.Equal(t, uint64(100), balance.Available)
	assert.Equal(t, uint64(50), balance.Locked)
}

// TestGetAccountBalance_WithContractTokenUTXO_ReturnsBalance 测试合约代币UTXO余额查询
func TestGetAccountBalance_WithContractTokenUTXO_ReturnsBalance(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	
	tokenID := testutil.RandomHash()
	utxoQuery := &mockUTXOQueryWithData{
		utxos: []*utxo.UTXO{
			txtestutil.CreateUTXO(nil, txtestutil.CreateContractTokenOutput(nil, "200", tokenID, nil, nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE),
			txtestutil.CreateUTXO(nil, txtestutil.CreateContractTokenOutput(nil, "75", tokenID, nil, nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED),
		},
	}
	
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, utxoQuery, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()

	// Act
	balance, err := service.GetAccountBalance(ctx, address, tokenID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, uint64(275), balance.Total)
	assert.Equal(t, uint64(200), balance.Available)
	assert.Equal(t, uint64(75), balance.Locked)
}

// TestGetAccountBalance_WithMixedTokenUTXOs_FiltersByTokenID 测试混合代币UTXO时按代币ID过滤
func TestGetAccountBalance_WithMixedTokenUTXOs_FiltersByTokenID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	
	tokenID1 := testutil.RandomHash()
	tokenID2 := testutil.RandomHash()
	utxoQuery := &mockUTXOQueryWithData{
		utxos: []*utxo.UTXO{
			txtestutil.CreateUTXO(nil, txtestutil.CreateContractTokenOutput(nil, "100", tokenID1, nil, nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE),
			txtestutil.CreateUTXO(nil, txtestutil.CreateContractTokenOutput(nil, "200", tokenID2, nil, nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE),
			txtestutil.CreateUTXO(nil, txtestutil.CreateNativeCoinOutput(nil, "50", nil), utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE),
		},
	}
	
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, utxoQuery, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()

	// Act - 查询 tokenID1
	balance1, err := service.GetAccountBalance(ctx, address, tokenID1)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, balance1)
	assert.Equal(t, uint64(100), balance1.Total)
}

// TestGetAccountBalance_WithUTXOQueryError_ReturnsError 测试UTXO查询错误时返回错误
func TestGetAccountBalance_WithUTXOQueryError_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	utxoQuery := &mockUTXOQueryWithError{err: assert.AnError}
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, utxoQuery, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()
	tokenID := testutil.RandomHash()

	// Act
	balance, err := service.GetAccountBalance(ctx, address, tokenID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, balance)
	assert.Contains(t, err.Error(), "获取地址UTXO失败")
}

// ==================== 辅助函数和 Mock ====================

// mockUTXOQueryWithData 带数据的 UTXOQuery Mock
type mockUTXOQueryWithData struct {
	interfaces.InternalUTXOQuery
	utxos []*utxo.UTXO
}

func (m *mockUTXOQueryWithData) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return m.utxos, nil
}

// mockUTXOQueryWithError 带错误的 UTXOQuery Mock
type mockUTXOQueryWithError struct {
	interfaces.InternalUTXOQuery
	err error
}

func (m *mockUTXOQueryWithError) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, m.err
}

// ==================== 编译时检查 ====================

// 确保 Service 实现了接口
var _ interfaces.InternalAccountQuery = (*Service)(nil)

