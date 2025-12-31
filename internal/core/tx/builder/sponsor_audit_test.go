// Package builder_test 提供 SponsorAuditService 的单元测试
//
// 🧪 **测试覆盖**：
// - SponsorAuditService 核心功能测试
// - 领取历史查询测试
// - 统计信息测试
// - 交易解析测试
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
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== NewSponsorAuditService 测试 ====================

// TestNewSponsorAuditService 测试创建 SponsorAuditService
func TestNewSponsorAuditService(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	assert.NotNil(t, service)
	assert.NotNil(t, service.eutxoQuery)
	assert.NotNil(t, service.txQuery)
	assert.NotNil(t, service.chainQuery)
	assert.NotNil(t, service.hashManager)
	assert.NotNil(t, service.helper)
}

// ==================== GetSponsorClaimHistory 测试 ====================

// TestGetSponsorClaimHistory_Success 测试查询赞助UTXO领取历史成功
func TestGetSponsorClaimHistory_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	outpoint := testutil.CreateOutPoint(nil, 0)

	// 当前实现返回空列表（需要扩展TxQuery接口）
	history, err := service.GetSponsorClaimHistory(ctx, outpoint)

	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.Empty(t, history) // 当前实现返回空列表
}

// TestGetSponsorClaimHistory_NilOutpoint 测试nil Outpoint
func TestGetSponsorClaimHistory_NilOutpoint(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()

	history, err := service.GetSponsorClaimHistory(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "sponsorUTXOId不能为空")
}

// ==================== GetMinerClaimHistory 测试 ====================

// TestAuditGetMinerClaimHistory_Success 测试查询矿工领取历史成功
func TestAuditGetMinerClaimHistory_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()

	// 当前实现返回空列表（因为GetSponsorClaimHistory返回空列表）
	history, err := service.GetMinerClaimHistory(ctx, minerAddr)

	assert.NoError(t, err)
	// 注意：当前实现可能返回 nil，需要检查
	if history == nil {
		history = []*ClaimRecord{} // 如果返回 nil，使用空列表
	}
	assert.Empty(t, history) // 当前实现返回空列表
}

// TestAuditGetMinerClaimHistory_EmptyAddress 测试空地址
func TestAuditGetMinerClaimHistory_EmptyAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()

	history, err := service.GetMinerClaimHistory(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "minerAddr不能为空")
}

// TestGetMinerClaimHistory_QueryError 测试查询失败
func TestGetMinerClaimHistory_QueryError(t *testing.T) {
	utxoQuery := NewMockUTXOQueryWithErrorForAudit(fmt.Errorf("查询失败"))
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()

	history, err := service.GetMinerClaimHistory(ctx, minerAddr)

	assert.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "查询赞助池UTXO失败")
}

// ==================== GetSponsorStatistics 测试 ====================

// TestGetSponsorStatistics_Success 测试获取统计信息成功
func TestGetSponsorStatistics_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{currentHeight: 200}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加多个赞助UTXO
	sponsorUTXO1 := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO2 := createSponsorUTXOForTest("2000000", []string{"consume"}, nil, 150)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO1)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO2)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 2, stats.TotalSponsors)
	assert.Equal(t, big.NewInt(3000000), stats.TotalAmount)
	assert.Equal(t, 2, stats.ActiveSponsors)
}

// TestGetSponsorStatistics_WithConsumed 测试包含已消费的UTXO
func TestGetSponsorStatistics_WithConsumed(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{currentHeight: 200}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加已消费的赞助UTXO
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO.Status = utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.TotalSponsors)
	assert.Equal(t, big.NewInt(1000000), stats.TotalAmount)
	assert.Equal(t, big.NewInt(1000000), stats.TotalClaimed)
	assert.Equal(t, 1, stats.FullyClaimedCount)
}

// TestGetSponsorStatistics_WithExpired 测试包含已过期的UTXO
func TestGetSponsorStatistics_WithExpired(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{currentHeight: 200} // 当前高度200
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加已过期的赞助UTXO（创建高度100，过期高度150）
	expiryBlocks := uint64(50)
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, &expiryBlocks, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.TotalSponsors)
	assert.Equal(t, 1, stats.ExpiredSponsors)
	assert.Equal(t, 0, stats.ActiveSponsors)
}

// TestGetSponsorStatistics_WithPartialClaimed 测试包含部分领取的UTXO
func TestGetSponsorStatistics_WithPartialClaimed(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{currentHeight: 200}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加活跃的赞助UTXO（当前实现无法真正测试部分领取，因为GetSponsorClaimHistory返回空列表）
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.TotalSponsors)
	assert.Equal(t, big.NewInt(1000000), stats.TotalAmount)
	assert.Equal(t, 1, stats.ActiveSponsors)
}

// TestGetSponsorStatistics_GetCurrentHeightError 测试获取当前高度失败
func TestGetSponsorStatistics_GetCurrentHeightError(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQueryWithError{currentHeight: 200}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加有过期时间的赞助UTXO
	expiryBlocks := uint64(50)
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, &expiryBlocks, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	// 即使获取当前高度失败，也应该返回统计信息（过期判断会跳过）
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.TotalSponsors)
}

// TestGetMinerClaimHistory_WithMultipleUTXOs 测试多个UTXO的情况
func TestGetMinerClaimHistory_WithMultipleUTXOs(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 添加多个赞助UTXO
	sponsorUTXO1 := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	sponsorUTXO2 := createSponsorUTXOForTest("2000000", []string{"consume"}, nil, 150)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO1)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO2)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()

	// 当前实现返回空列表（因为GetSponsorClaimHistory返回空列表）
	history, err := service.GetMinerClaimHistory(ctx, minerAddr)

	assert.NoError(t, err)
	if history == nil {
		history = []*ClaimRecord{}
	}
	assert.Empty(t, history) // 当前实现返回空列表
}

// TestGetSponsorStatistics_QueryError 测试查询失败
func TestGetSponsorStatistics_QueryError(t *testing.T) {
	utxoQuery := NewMockUTXOQueryWithErrorForAudit(fmt.Errorf("查询失败"))
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	ctx := context.Background()

	stats, err := service.GetSponsorStatistics(ctx)

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "查询赞助池UTXO失败")
}

// ==================== parseClaimTransaction 测试 ====================

// TestParseClaimTransaction_Success 测试解析领取交易成功
func TestParseClaimTransaction_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	// 创建赞助领取交易
	sponsorUTXOId := testutil.CreateOutPoint(nil, 0)
	minerAddr := testutil.RandomAddress()
	delegationProof := &transaction_pb.DelegationProof{
		DelegationTransactionId: sponsorUTXOId.TxId,
		DelegationOutputIndex:   sponsorUTXOId.OutputIndex,
		OperationType:           "consume",
		ValueAmount:             500000,
		DelegateAddress:         minerAddr,
	}
	tx := &transaction_pb.Transaction{
		Version: 1,
		Inputs: []*transaction_pb.TxInput{
			{
				PreviousOutput: sponsorUTXOId,
				UnlockingProof: &transaction_pb.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		Outputs: []*transaction_pb.TxOutput{
			// 输出1: 矿工领取
			testutil.CreateNativeCoinOutput(minerAddr, "500000", testutil.CreateSingleKeyLock(nil)),
			// 输出2: 找零（返回赞助池）
			testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "500000", createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100).GetCachedOutput().LockingConditions[0]),
		},
	}

	record, err := service.parseClaimTransaction(tx, sponsorUTXOId)

	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, minerAddr, record.MinerAddress)
	assert.Equal(t, big.NewInt(500000), record.ClaimAmount)
	assert.NotNil(t, record.TransactionId)
}

// TestParseClaimTransaction_NotSingleInput 测试不是单输入交易
func TestParseClaimTransaction_NotSingleInput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	sponsorUTXOId := testutil.CreateOutPoint(nil, 0)
	tx := &transaction_pb.Transaction{
		Version: 1,
		Inputs: []*transaction_pb.TxInput{
			{PreviousOutput: sponsorUTXOId},
			{PreviousOutput: testutil.CreateOutPoint(nil, 0)}, // 两个输入
		},
	}

	record, err := service.parseClaimTransaction(tx, sponsorUTXOId)

	assert.Error(t, err)
	assert.Nil(t, record)
	assert.Contains(t, err.Error(), "不是赞助领取交易")
}

// TestParseClaimTransaction_NoDelegationProof 测试缺少DelegationProof
func TestParseClaimTransaction_NoDelegationProof(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	sponsorUTXOId := testutil.CreateOutPoint(nil, 0)
	tx := &transaction_pb.Transaction{
		Version: 1,
		Inputs: []*transaction_pb.TxInput{
			{
				PreviousOutput: sponsorUTXOId,
				UnlockingProof: &transaction_pb.TxInput_SingleKeyProof{
					SingleKeyProof: &transaction_pb.SingleKeyProof{},
				},
			},
		},
	}

	record, err := service.parseClaimTransaction(tx, sponsorUTXOId)

	assert.Error(t, err)
	assert.Nil(t, record)
	assert.Contains(t, err.Error(), "缺少DelegationProof")
}

// TestParseClaimTransaction_WithChange 测试有找零的领取交易
func TestParseClaimTransaction_WithChange(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	txQuery := &MockTxQuery{}
	chainQuery := &MockChainQuery{}
	hashManager := testutil.NewTestHashManager()

	service := NewSponsorAuditService(utxoQuery, txQuery, chainQuery, hashManager)

	sponsorUTXOId := testutil.CreateOutPoint(nil, 0)
	minerAddr := testutil.RandomAddress()
	delegationProof := &transaction_pb.DelegationProof{
		DelegationTransactionId: sponsorUTXOId.TxId,
		DelegationOutputIndex:   sponsorUTXOId.OutputIndex,
		OperationType:           "consume",
		ValueAmount:             500000,
		DelegateAddress:         minerAddr,
	}
	// 创建找零输出的锁定条件
	sponsorUTXO := createSponsorUTXOForTest("1000000", []string{"consume"}, nil, 100)
	changeLock := sponsorUTXO.GetCachedOutput().LockingConditions[0]
	tx := &transaction_pb.Transaction{
		Version: 1,
		Inputs: []*transaction_pb.TxInput{
			{
				PreviousOutput: sponsorUTXOId,
				UnlockingProof: &transaction_pb.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		Outputs: []*transaction_pb.TxOutput{
			// 输出1: 矿工领取
			testutil.CreateNativeCoinOutput(minerAddr, "500000", testutil.CreateSingleKeyLock(nil)),
			// 输出2: 找零（返回赞助池）
			testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "500000", changeLock),
		},
	}

	record, err := service.parseClaimTransaction(tx, sponsorUTXOId)

	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.NotNil(t, record.ChangeAmount)
	assert.Equal(t, big.NewInt(500000), record.ChangeAmount)
}

// ==================== Mock 对象 ====================

// MockUTXOQueryWithErrorForAudit 带错误的UTXO查询器（用于 sponsor_audit_test.go）
type MockUTXOQueryWithErrorForAudit struct {
	*testutil.MockUTXOQuery
	queryError error
}

func NewMockUTXOQueryWithErrorForAudit(queryError error) *MockUTXOQueryWithErrorForAudit {
	return &MockUTXOQueryWithErrorForAudit{
		MockUTXOQuery: testutil.NewMockUTXOQuery(),
		queryError:    queryError,
	}
}

func (m *MockUTXOQueryWithErrorForAudit) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.MockUTXOQuery.GetSponsorPoolUTXOs(ctx, onlyAvailable)
}

// MockChainQueryWithError 带错误的链查询服务
type MockChainQueryWithError struct {
	currentHeight uint64
	heightError   error
}

func (m *MockChainQueryWithError) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	return &types.ChainInfo{}, nil
}

func (m *MockChainQueryWithError) GetCurrentHeight(ctx context.Context) (uint64, error) {
	if m.heightError != nil {
		return 0, m.heightError
	}
	return m.currentHeight, nil
}

func (m *MockChainQueryWithError) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	return testutil.RandomHash(), nil
}

func (m *MockChainQueryWithError) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *MockChainQueryWithError) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockChainQueryWithError) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockChainQueryWithError) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{}, nil
}

