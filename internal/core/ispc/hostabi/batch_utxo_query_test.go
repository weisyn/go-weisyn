package hostabi

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// BatchUTXOQuerier 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 BatchUTXOQuerier 的缺陷和BUG
//
// ============================================================================

// TestNewBatchUTXOQuerier 测试创建批量UTXO查询器
func TestNewBatchUTXOQuerier(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForBatch{}

	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	assert.NotNil(t, querier, "应该成功创建查询器")
	assert.Equal(t, mockUTXOQuery, querier.eutxoQuery, "应该设置UTXO查询服务")
	assert.Equal(t, logger, querier.logger, "应该设置日志器")
}

// TestBatchUTXOQuerier_BatchQueryUTXOs_Empty 测试空outpoint列表
func TestBatchUTXOQuerier_BatchQueryUTXOs_Empty(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForBatch{}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	result, err := querier.BatchQueryUTXOs(ctx, []*pb.OutPoint{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Empty(t, result.UTXOs, "UTXO映射应该为空")
	assert.Empty(t, result.Errors, "错误映射应该为空")
}

// TestBatchUTXOQuerier_BatchQueryUTXOs_Success 测试成功批量查询UTXO
func TestBatchUTXOQuerier_BatchQueryUTXOs_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	txID := make([]byte, 32)
	mockUTXOQuery := &mockUTXOQueryForBatch{
		utxos: map[string]*utxo.UTXO{
			generateOutpointKey(txID, 0): {
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						Owner: make([]byte, 20),
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "100",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		{TxId: txID, OutputIndex: 0},
	}

	result, err := querier.BatchQueryUTXOs(ctx, outpoints)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Len(t, result.UTXOs, 1, "应该返回1个UTXO")
	assert.Empty(t, result.Errors, "错误映射应该为空")
}

// TestBatchUTXOQuerier_BatchQueryUTXOs_NotFound 测试UTXO不存在
func TestBatchUTXOQuerier_BatchQueryUTXOs_NotFound(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForBatch{
		err: assert.AnError,
	}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		{TxId: make([]byte, 32), OutputIndex: 0},
	}

	result, err := querier.BatchQueryUTXOs(ctx, outpoints)

	assert.Error(t, err, "应该返回错误（所有查询都失败）")
	assert.NotNil(t, result, "应该返回结果")
	assert.Empty(t, result.UTXOs, "UTXO映射应该为空")
	assert.Len(t, result.Errors, 1, "错误映射应该有1个错误")
	assert.Contains(t, err.Error(), "批量UTXO查询全部失败", "错误信息应该正确")
}

// TestBatchUTXOQuerier_BatchQueryUTXOs_NilOutpoint 测试nil outpoint
func TestBatchUTXOQuerier_BatchQueryUTXOs_NilOutpoint(t *testing.T) {
	logger := testutil.NewTestLogger()
	txID := make([]byte, 32)
	mockUTXOQuery := &mockUTXOQueryForBatch{
		utxos: map[string]*utxo.UTXO{
			generateOutpointKey(txID, 0): {
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{},
				},
			},
		},
	}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		nil, // nil outpoint被跳过
		{TxId: txID, OutputIndex: 0}, // 存在的UTXO
	}

	result, err := querier.BatchQueryUTXOs(ctx, outpoints)

	assert.NoError(t, err, "应该成功（nil outpoint被跳过，第二个UTXO存在）")
	assert.NotNil(t, result, "应该返回结果")
	assert.Len(t, result.UTXOs, 1, "应该返回1个UTXO（nil outpoint被跳过）")
	assert.Empty(t, result.Errors, "错误映射应该为空")
}

// TestBatchUTXOQuerier_BatchQueryUTXOs_NoCachedOutput 测试UTXO存在但没有缓存的输出
func TestBatchUTXOQuerier_BatchQueryUTXOs_NoCachedOutput(t *testing.T) {
	logger := testutil.NewTestLogger()
	txID := make([]byte, 32)
	mockUTXOQuery := &mockUTXOQueryForBatch{
		utxos: map[string]*utxo.UTXO{
			generateOutpointKey(txID, 0): {
				ContentStrategy: &utxo.UTXO_ReferenceOnly{
					ReferenceOnly: true,
				},
			},
		},
	}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		{TxId: txID, OutputIndex: 0},
	}

	result, err := querier.BatchQueryUTXOs(ctx, outpoints)

	// 当UTXO存在但没有缓存的输出时，会记录错误
	// 如果所有查询都失败（包括这种情况），会返回总体错误
	assert.Error(t, err, "应该返回错误（所有查询都失败）")
	assert.NotNil(t, result, "应该返回结果")
	assert.Empty(t, result.UTXOs, "UTXO映射应该为空（没有缓存的输出）")
	assert.Len(t, result.Errors, 1, "错误映射应该有1个错误")
	outpointKey := generateOutpointKey(txID, 0)
	assert.Contains(t, result.Errors[outpointKey].Error(), "UTXO存在但无法获取输出", "错误信息应该正确")
}

// TestBatchUTXOQuerier_BatchQueryUTXOExists_Empty 测试空outpoint列表
func TestBatchUTXOQuerier_BatchQueryUTXOExists_Empty(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForBatch{}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	result, err := querier.BatchQueryUTXOExists(ctx, []*pb.OutPoint{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Empty(t, result, "结果映射应该为空")
}

// TestBatchUTXOQuerier_BatchQueryUTXOExists_Success 测试成功批量查询UTXO存在性
func TestBatchUTXOQuerier_BatchQueryUTXOExists_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	txID1 := make([]byte, 32)
	txID2 := make([]byte, 32)
	txID2[0] = 1 // 确保txID2不同
	mockUTXOQuery := &mockUTXOQueryForBatch{
		utxos: map[string]*utxo.UTXO{
			generateOutpointKey(txID1, 0): {
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{},
				},
			},
		},
	}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		{TxId: txID1, OutputIndex: 0},
		{TxId: txID2, OutputIndex: 1}, // 不存在的UTXO
	}

	result, err := querier.BatchQueryUTXOExists(ctx, outpoints)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Len(t, result, 2, "结果映射应该有2个条目")
	assert.True(t, result[generateOutpointKey(txID1, 0)], "第一个UTXO应该存在")
	assert.False(t, result[generateOutpointKey(txID2, 1)], "第二个UTXO应该不存在")
}

// TestBatchUTXOQuerier_BatchQueryUTXOExists_NilOutpoint 测试nil outpoint
func TestBatchUTXOQuerier_BatchQueryUTXOExists_NilOutpoint(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForBatch{}
	querier := NewBatchUTXOQuerier(mockUTXOQuery, logger)

	ctx := context.Background()
	outpoints := []*pb.OutPoint{
		nil,
		{TxId: make([]byte, 32), OutputIndex: 0},
	}

	result, err := querier.BatchQueryUTXOExists(ctx, outpoints)

	assert.NoError(t, err, "应该成功（nil outpoint被跳过）")
	assert.NotNil(t, result, "应该返回结果")
	// nil outpoint被跳过，所以结果映射可能只有1个条目
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockUTXOQueryForBatch Mock的UTXO查询服务（用于批量查询测试）
type mockUTXOQueryForBatch struct {
	utxos map[string]*utxo.UTXO
	err   error
}

func (m *mockUTXOQueryForBatch) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.utxos == nil {
		return nil, assert.AnError
	}
	key := outpointKey(outpoint)
	if utxo, ok := m.utxos[key]; ok {
		return utxo, nil
	}
	return nil, assert.AnError
}

func (m *mockUTXOQueryForBatch) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}
func (m *mockUTXOQueryForBatch) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}
func (m *mockUTXOQueryForBatch) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return nil, nil
}

// outpointKey 生成outpoint的字符串键
func outpointKey(outpoint *pb.OutPoint) string {
	if outpoint == nil {
		return ""
	}
	return generateOutpointKey(outpoint.TxId, outpoint.OutputIndex)
}

// generateOutpointKey 生成outpoint的字符串键（用于测试）
func generateOutpointKey(txID []byte, index uint32) string {
	return fmt.Sprintf("%x:%d", txID, index)
}

