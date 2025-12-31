// Package builder_test 提供 SponsorTools 的单元测试
//
// 🧪 **测试覆盖**：
// - SponsorTools 核心功能测试
// - 赞助UTXO查询测试
// - 配置验证测试
// - 锁定条件生成测试
package builder

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== NewSponsorTools 测试 ====================

// TestNewSponsorTools 测试创建 SponsorTools
func TestNewSponsorTools(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	assert.NotNil(t, tools)
	assert.NotNil(t, tools.eutxoQuery)
	assert.NotNil(t, tools.helper)
	assert.NotNil(t, tools.audit)
}

// ==================== ListSponsorUTXOs 测试 ====================

// TestListSponsorUTXOs_Success 测试列出赞助UTXO成功
func TestListSponsorUTXOs_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	ctx := context.Background()
	currentHeight := uint64(200)
	onlyAvailable := true

	result, err := tools.ListSponsorUTXOs(ctx, currentHeight, onlyAvailable)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, sponsorUTXO, result[0].UTXO)
	assert.Equal(t, SponsorStateActive, result[0].LifecycleState)
}

// TestListSponsorUTXOs_Empty 测试没有赞助UTXO
func TestListSponsorUTXOs_Empty(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	currentHeight := uint64(200)
	onlyAvailable := true

	result, err := tools.ListSponsorUTXOs(ctx, currentHeight, onlyAvailable)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

// TestListSponsorUTXOs_QueryError 测试查询失败
func TestListSponsorUTXOs_QueryError(t *testing.T) {
	utxoQuery := NewMockUTXOQueryWithErrorForTools(fmt.Errorf("查询失败"))
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	currentHeight := uint64(200)
	onlyAvailable := true

	result, err := tools.ListSponsorUTXOs(ctx, currentHeight, onlyAvailable)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "查询赞助池UTXO失败")
}

// TestListSponsorUTXOs_FilterInvalid 测试过滤无效UTXO
func TestListSponsorUTXOs_FilterInvalid(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加有效的赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	// 添加无效的UTXO（不是赞助UTXO）
	invalidUTXO := testutil.CreateUTXO(
		testutil.CreateOutPoint(nil, 0),
		testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil)),
		utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
	)
	utxoQuery.AddUTXO(invalidUTXO)

	ctx := context.Background()
	currentHeight := uint64(200)
	onlyAvailable := true

	result, err := tools.ListSponsorUTXOs(ctx, currentHeight, onlyAvailable)

	assert.NoError(t, err)
	assert.Len(t, result, 1) // 只返回有效的赞助UTXO
	assert.Equal(t, sponsorUTXO, result[0].UTXO)
}

// ==================== GetSponsorUTXOInfo 测试 ====================

// TestGetSponsorUTXOInfo_Success 测试获取赞助UTXO信息成功
func TestGetSponsorUTXOInfo_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	utxoQuery.AddUTXO(sponsorUTXO)

	ctx := context.Background()
	outpoint := sponsorUTXO.Outpoint
	currentHeight := uint64(200)

	result, err := tools.GetSponsorUTXOInfo(ctx, outpoint, currentHeight)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Info)
	assert.Equal(t, sponsorUTXO, result.Info.UTXO)
	assert.Equal(t, SponsorStateActive, result.Info.LifecycleState)
	assert.NotNil(t, result.ClaimHistory)
}

// TestGetSponsorUTXOInfo_UTXONotFound 测试UTXO不存在
func TestGetSponsorUTXOInfo_UTXONotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	outpoint := testutil.CreateOutPoint(nil, 0)
	currentHeight := uint64(200)

	result, err := tools.GetSponsorUTXOInfo(ctx, outpoint, currentHeight)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "查询UTXO失败")
}

// TestGetSponsorUTXOInfo_NotSponsorUTXO 测试不是赞助UTXO
func TestGetSponsorUTXOInfo_NotSponsorUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加普通UTXO
	normalUTXO := testutil.CreateUTXO(
		testutil.CreateOutPoint(nil, 0),
		testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil)),
		utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
	)
	utxoQuery.AddUTXO(normalUTXO)

	ctx := context.Background()
	outpoint := normalUTXO.Outpoint
	currentHeight := uint64(200)

	result, err := tools.GetSponsorUTXOInfo(ctx, outpoint, currentHeight)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "不是赞助UTXO")
}

// ==================== ValidateSponsorUTXO 测试 ====================

// TestSponsorTools_ValidateSponsorUTXO_Success 测试验证成功
func TestSponsorTools_ValidateSponsorUTXO_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)

	err := tools.ValidateSponsorUTXO(sponsorUTXO)

	assert.NoError(t, err)
}

// ==================== GetStatistics 测试 ====================

// TestGetStatistics_Success 测试获取统计信息成功
func TestGetStatistics_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{currentHeight: 200}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加多个赞助UTXO
	sponsorUTXO1 := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO2 := createSponsorUTXOForTest("2000000", []string{"consume"}, nil, 150)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO1)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO2)

	ctx := context.Background()

	stats, err := tools.GetStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 2, stats.TotalSponsors)
	assert.Equal(t, big.NewInt(3000000), stats.TotalAmount)
	assert.Equal(t, 2, stats.ActiveSponsors)
}

// ==================== GetMinerClaimHistory 测试 ====================

// TestGetMinerClaimHistory_Success 测试获取矿工领取历史成功
func TestGetMinerClaimHistory_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()

	// 当前实现返回空列表（因为GetSponsorClaimHistory返回空列表）
	history, err := tools.GetMinerClaimHistory(ctx, minerAddr)

	assert.NoError(t, err)
	// 注意：当前实现可能返回 nil，需要检查
	if history == nil {
		history = []*ClaimRecord{} // 如果返回 nil，使用空列表
	}
	assert.Empty(t, history) // 当前实现返回空列表
}

// TestGetMinerClaimHistory_EmptyAddress 测试空地址
func TestGetMinerClaimHistory_EmptyAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	tools := NewSponsorTools(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()

	history, err := tools.GetMinerClaimHistory(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "minerAddr不能为空")
}

// ==================== SponsorUTXOConfig 测试 ====================

// TestValidateConfig_Success 测试配置验证成功
func TestValidateConfig_Success(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseDelegationLock:    true,
		MaxValuePerOperation: 1000000,
	}

	err := config.ValidateConfig()

	assert.NoError(t, err)
}

// TestValidateConfig_NoLockSelected 测试未选择锁定方式
func TestValidateConfig_NoLockSelected(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType: "native",
		Amount:    big.NewInt(1000000),
		// 未选择任何锁定方式
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须且只能选择一种锁定方式")
}

// TestValidateConfig_MultipleLocks 测试选择了多种锁定方式
func TestValidateConfig_MultipleLocks(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseDelegationLock:    true,
		UseContractLock:      true, // 选择了两种锁定方式
		MaxValuePerOperation: 1000000,
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须且只能选择一种锁定方式")
}

// TestValidateConfig_InvalidAmount 测试无效金额
func TestValidateConfig_InvalidAmount(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:         "native",
		Amount:            big.NewInt(0), // 金额为0
		UseDelegationLock: true,
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额必须大于0")
}

// TestValidateConfig_EmptyTokenType 测试空代币类型
func TestValidateConfig_EmptyTokenType(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:         "", // 空代币类型
		Amount:            big.NewInt(1000000),
		UseDelegationLock: true,
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "代币类型不能为空")
}

// TestValidateConfig_ContractLock_MissingAddress 测试ContractLock缺少地址
func TestValidateConfig_ContractLock_MissingAddress(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:         "native",
		Amount:            big.NewInt(1000000),
		UseContractLock:   true,
		RequiredMethod:    "claim",
		// 缺少ContractAddress
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ContractLock需要合约地址")
}

// TestValidateConfig_ContractLock_MissingMethod 测试ContractLock缺少方法名
func TestValidateConfig_ContractLock_MissingMethod(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:         "native",
		Amount:            big.NewInt(1000000),
		UseContractLock:   true,
		ContractAddress:   testutil.RandomAddress(),
		// 缺少RequiredMethod
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ContractLock需要方法名")
}

// TestValidateConfig_HeightLock_InvalidHeight 测试HeightLock无效高度
func TestValidateConfig_HeightLock_InvalidHeight(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseHeightLock:        true,
		UnlockHeight:         0, // 无效高度
		MaxValuePerOperation: 1000000,
	}

	err := config.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UnlockHeight必须大于0")
}

// ==================== ToLockingConditions 测试 ====================

// TestToLockingConditions_DelegationLock 测试生成DelegationLock条件
func TestToLockingConditions_DelegationLock(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseDelegationLock:    true,
		MaxValuePerOperation: 1000000,
	}

	conditions, err := config.ToLockingConditions()

	assert.NoError(t, err)
	assert.Len(t, conditions, 1)
	assert.NotNil(t, conditions[0].GetDelegationLock())
	assert.Equal(t, uint64(1000000), conditions[0].GetDelegationLock().MaxValuePerOperation)
}

// TestToLockingConditions_DelegationLock_WithExpiry 测试带过期时间的DelegationLock
func TestToLockingConditions_DelegationLock_WithExpiry(t *testing.T) {
	expiryBlocks := uint64(50)
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseDelegationLock:    true,
		MaxValuePerOperation: 1000000,
		ExpiryDurationBlocks: &expiryBlocks,
	}

	conditions, err := config.ToLockingConditions()

	assert.NoError(t, err)
	assert.Len(t, conditions, 1)
	assert.NotNil(t, conditions[0].GetDelegationLock())
	assert.Equal(t, &expiryBlocks, conditions[0].GetDelegationLock().ExpiryDurationBlocks)
}

// TestToLockingConditions_DelegationLock_WithAllowedDelegates 测试带允许委托地址的DelegationLock
func TestToLockingConditions_DelegationLock_WithAllowedDelegates(t *testing.T) {
	allowedDelegates := [][]byte{testutil.RandomAddress(), testutil.RandomAddress()}
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseDelegationLock:    true,
		MaxValuePerOperation: 1000000,
		AllowedDelegates:     allowedDelegates,
	}

	conditions, err := config.ToLockingConditions()

	assert.NoError(t, err)
	assert.Len(t, conditions, 1)
	delegationLock := conditions[0].GetDelegationLock()
	assert.NotNil(t, delegationLock)
	assert.Len(t, delegationLock.AllowedDelegates, 2)
	assert.Equal(t, allowedDelegates[0], delegationLock.AllowedDelegates[0])
	assert.Equal(t, allowedDelegates[1], delegationLock.AllowedDelegates[1])
}

// TestToLockingConditions_ContractLock 测试生成ContractLock条件
func TestToLockingConditions_ContractLock(t *testing.T) {
	contractAddr := testutil.RandomAddress()
	config := &SponsorUTXOConfig{
		TokenType:         "native",
		Amount:            big.NewInt(1000000),
		UseContractLock:   true,
		ContractAddress:   contractAddr,
		RequiredMethod:    "claim",
	}

	conditions, err := config.ToLockingConditions()

	assert.NoError(t, err)
	assert.Len(t, conditions, 1)
	assert.NotNil(t, conditions[0].GetContractLock())
	assert.Equal(t, contractAddr, conditions[0].GetContractLock().ContractAddress)
	assert.Equal(t, "claim", conditions[0].GetContractLock().RequiredMethod)
}

// TestToLockingConditions_HeightLock 测试生成HeightLock条件
func TestToLockingConditions_HeightLock(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType:            "native",
		Amount:               big.NewInt(1000000),
		UseHeightLock:        true,
		UnlockHeight:         1000,
		ConfirmationBlocks:  10,
		MaxValuePerOperation: 1000000,
	}

	conditions, err := config.ToLockingConditions()

	assert.NoError(t, err)
	assert.Len(t, conditions, 1)
	assert.NotNil(t, conditions[0].GetHeightLock())
	assert.Equal(t, uint64(1000), conditions[0].GetHeightLock().UnlockHeight)
	assert.Equal(t, uint32(10), conditions[0].GetHeightLock().ConfirmationBlocks)
	assert.NotNil(t, conditions[0].GetHeightLock().BaseLock)
	assert.NotNil(t, conditions[0].GetHeightLock().BaseLock.GetDelegationLock())
}

// TestToLockingConditions_InvalidConfig 测试无效配置
func TestToLockingConditions_InvalidConfig(t *testing.T) {
	config := &SponsorUTXOConfig{
		TokenType: "native",
		Amount:    big.NewInt(0), // 无效金额
		UseDelegationLock: true,
	}

	conditions, err := config.ToLockingConditions()

	assert.Error(t, err)
	assert.Nil(t, conditions)
}

// ==================== Mock 对象 ====================

// MockTxQuery 模拟交易查询服务
type MockTxQuery struct{}

func (m *MockTxQuery) GetTransaction(ctx context.Context, txHash []byte) ([]byte, uint32, *transaction_pb.Transaction, error) {
	return nil, 0, nil, fmt.Errorf("not implemented")
}

func (m *MockTxQuery) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (m *MockTxQuery) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (m *MockTxQuery) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}

func (m *MockTxQuery) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction_pb.Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockChainQuery 模拟链查询服务
type MockChainQuery struct {
	currentHeight uint64
}

func (m *MockChainQuery) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	return &types.ChainInfo{}, nil
}

func (m *MockChainQuery) GetCurrentHeight(ctx context.Context) (uint64, error) {
	return m.currentHeight, nil
}

func (m *MockChainQuery) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	return testutil.RandomHash(), nil
}

func (m *MockChainQuery) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *MockChainQuery) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockChainQuery) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockChainQuery) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{}, nil
}

// MockUTXOQueryWithErrorForTools 带错误的UTXO查询器（用于 sponsor_tools_test.go）
type MockUTXOQueryWithErrorForTools struct {
	*testutil.MockUTXOQuery
	queryError error
}

func NewMockUTXOQueryWithErrorForTools(queryError error) *MockUTXOQueryWithErrorForTools {
	return &MockUTXOQueryWithErrorForTools{
		MockUTXOQuery: testutil.NewMockUTXOQuery(),
		queryError:    queryError,
	}
}

func (m *MockUTXOQueryWithErrorForTools) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.MockUTXOQuery.GetSponsorPoolUTXOs(ctx, onlyAvailable)
}

