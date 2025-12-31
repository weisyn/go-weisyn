package reorg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/chain/fork/reorg"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ============================================================================
//                              最小化 Mock 组件
// ============================================================================

// mockReversible 实现 reorg.Reversible 接口的最小 Mock
type mockReversible struct {
	createErr  error
	rollbackErr error
	discardErr error
	verifyErr  error
	verifyPassed bool
}

func (m *mockReversible) CreateRollbackPoint(ctx context.Context, height uint64) (reorg.RollbackHandle, error) {
	if m.createErr != nil {
		return reorg.RollbackHandle{}, m.createErr
	}
	return reorg.RollbackHandle{
		ID:     "test-handle",
		Height: height,
	}, nil
}

func (m *mockReversible) Rollback(ctx context.Context, handle reorg.RollbackHandle) error {
	return m.rollbackErr
}

func (m *mockReversible) Discard(ctx context.Context, handle reorg.RollbackHandle) error {
	return m.discardErr
}

func (m *mockReversible) Verify(ctx context.Context, expectedHeight uint64) (*reorg.VerificationResult, error) {
	if m.verifyErr != nil {
		return nil, m.verifyErr
	}
	return &reorg.VerificationResult{
		Passed: m.verifyPassed,
		Checks: []reorg.CheckResult{
			{Name: "test-check", Passed: m.verifyPassed},
		},
	}, nil
}

// mockBlockProvider 提供测试区块
func mockBlockProvider(start, end uint64) reorg.BlockProvider {
	return func(height uint64) (*core.Block, bool) {
		if height >= start && height <= end {
			return &core.Block{
				Header: &core.BlockHeader{Height: height},
				Body:   &core.BlockBody{},
			}, true
		}
		return nil, false
	}
}

// ============================================================================
//                         1. 构造函数测试
// ============================================================================

func TestNewCoordinator_Success(t *testing.T) {
	// Arrange - 使用标准测试工具
	queryService := blocktestutil.NewMockQueryService()
	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	require.NoError(t, err)
	
	opts := reorg.Options{
		QueryService:    queryService,
		BlockProcessor:  blockProcessor,
		SnapshotManager: &mockReversible{},
		IndexManager:    &mockReversible{},
		VerifyFn: func(ctx context.Context, height uint64) (*reorg.VerificationResult, error) {
			return &reorg.VerificationResult{Passed: true}, nil
		},
	}

	// Act
	coord, err := reorg.NewCoordinator(opts)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, coord)
}

func TestNewCoordinator_MissingSnapshotManager(t *testing.T) {
	// Arrange
	queryService := blocktestutil.NewMockQueryService()
	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	require.NoError(t, err)
	
	opts := reorg.Options{
		QueryService:   queryService,
		BlockProcessor: blockProcessor,
		IndexManager:   &mockReversible{},
		VerifyFn: func(ctx context.Context, height uint64) (*reorg.VerificationResult, error) {
			return &reorg.VerificationResult{Passed: true}, nil
		},
	}

	// Act
	coord, err := reorg.NewCoordinator(opts)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, coord)
	assert.Contains(t, err.Error(), "SnapshotManager")
}

func TestNewCoordinator_MissingIndexManager(t *testing.T) {
	// Arrange
	queryService := blocktestutil.NewMockQueryService()
	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	require.NoError(t, err)
	
	opts := reorg.Options{
		QueryService:    queryService,
		BlockProcessor:  blockProcessor,
		SnapshotManager: &mockReversible{},
		VerifyFn: func(ctx context.Context, height uint64) (*reorg.VerificationResult, error) {
			return &reorg.VerificationResult{Passed: true}, nil
		},
	}

	// Act
	coord, err := reorg.NewCoordinator(opts)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, coord)
	assert.Contains(t, err.Error(), "IndexManager")
}

// ============================================================================
//                         2. BeginReorg 测试
// ============================================================================

// defaultCoordinator 创建一个带有默认依赖的 Coordinator（避免重复代码）
func defaultCoordinator(t *testing.T, opts ...func(*reorg.Options)) *reorg.Coordinator {
	t.Helper()
	
	// 使用 block/testutil 中的标准 Mock
	queryService := blocktestutil.NewMockQueryService()
	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	require.NoError(t, err)
	
	options := reorg.Options{
		QueryService:    queryService,
		BlockProcessor:  blockProcessor,
		SnapshotManager: &mockReversible{verifyPassed: true},
		IndexManager:    &mockReversible{verifyPassed: true},
		VerifyFn: func(ctx context.Context, height uint64) (*reorg.VerificationResult, error) {
			return &reorg.VerificationResult{Passed: true}, nil
		},
	}
	for _, opt := range opts {
		opt(&options)
	}
	coord, err := reorg.NewCoordinator(options)
	require.NoError(t, err)
	return coord
}

func TestBeginReorg_Success(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	session, err := coord.BeginReorg(context.Background(), 200, 100, 250)

	// Assert
	assert.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, uint64(200), session.FromHeight)
	assert.Equal(t, uint64(100), session.ForkHeight)
	assert.Equal(t, uint64(250), session.ToHeight)
	assert.NotEmpty(t, session.ID)
	// Coordinator 创建了多个回滚点：utxo_recovery, utxo_rollback, index_rollback
	assert.GreaterOrEqual(t, len(session.Handles), 2, "应该至少创建 2 个回滚点")
}

func TestBeginReorg_InvalidHeights_ForkNotLessThanFrom(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	session, err := coord.BeginReorg(context.Background(), 100, 200, 250)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestBeginReorg_InvalidHeights_ToNotGreaterThanFork(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	session, err := coord.BeginReorg(context.Background(), 200, 100, 100)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestBeginReorg_SnapshotCreationFailure(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t, func(opts *reorg.Options) {
		opts.SnapshotManager = &mockReversible{createErr: errors.New("snapshot creation failed")}
	})

	// Act
	session, err := coord.BeginReorg(context.Background(), 200, 100, 250)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "snapshot creation failed")
}

func TestBeginReorg_IndexCreationFailure(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t, func(opts *reorg.Options) {
		opts.IndexManager = &mockReversible{createErr: errors.New("index creation failed")}
	})

	// Act
	session, err := coord.BeginReorg(context.Background(), 200, 100, 250)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "index creation failed")
}

// ============================================================================
//                         3. ExecuteReorg 测试
// ============================================================================

func TestExecuteReorg_Success(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)
	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.ExecuteReorg(context.Background(), session, mockBlockProvider(101, 110))

	// Assert
	assert.NoError(t, err)
}

func TestExecuteReorg_RollbackFailure(t *testing.T) {
	// Arrange
	enterReadOnlyCalled := false
	coord := defaultCoordinator(t, func(opts *reorg.Options) {
		opts.SnapshotManager = &mockReversible{rollbackErr: errors.New("rollback failed")}
		opts.EnterReadOnlyFn = func(ctx context.Context, reason error) {
			enterReadOnlyCalled = true
		}
	})

	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.ExecuteReorg(context.Background(), session, mockBlockProvider(101, 110))

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rollback failed")
	assert.True(t, enterReadOnlyCalled, "应该进入只读模式")
}

func TestExecuteReorg_BlockProviderFailure(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)
	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Provider 返回 nil（模拟区块获取失败）
	failProvider := func(height uint64) (*core.Block, bool) {
		return nil, false
	}

	// Act
	err = coord.ExecuteReorg(context.Background(), session, failProvider)

	// Assert
	assert.Error(t, err)
}

func TestExecuteReorg_VerifyFailure(t *testing.T) {
	// Arrange
	queryService := blocktestutil.NewMockQueryService()
	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	require.NoError(t, err)
	
	opts := reorg.Options{
		QueryService:    queryService,
		BlockProcessor:  blockProcessor,
		SnapshotManager: &mockReversible{verifyPassed: true},
		IndexManager:    &mockReversible{verifyPassed: true},
		VerifyFn: func(ctx context.Context, height uint64) (*reorg.VerificationResult, error) {
			return &reorg.VerificationResult{Passed: false}, nil // 验证失败
		},
	}
	
	coord, err := reorg.NewCoordinator(opts)
	require.NoError(t, err)

	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.ExecuteReorg(context.Background(), session, mockBlockProvider(101, 110))

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
}

// ============================================================================
//                         4. CommitReorg 测试
// ============================================================================

func TestCommitReorg_Success(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)
	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.CommitReorg(context.Background(), session)

	// Assert
	assert.NoError(t, err)
}

func TestCommitReorg_DiscardFailure_ShouldFail(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t, func(opts *reorg.Options) {
		opts.SnapshotManager = &mockReversible{discardErr: errors.New("discard failed")}
	})
	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.CommitReorg(context.Background(), session)

	// Assert
	// 根据当前实现，Discard 失败会返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discard")
}

// ============================================================================
//                         5. AbortReorg 测试
// ============================================================================

func TestAbortReorg_Success(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)
	session, err := coord.BeginReorg(context.Background(), 200, 100, 110)
	require.NoError(t, err)

	// Act
	err = coord.AbortReorg(context.Background(), session, errors.New("test abort"))

	// Assert
	assert.NoError(t, err)
}

// ============================================================================
//                         6. 边界条件和错误处理
// ============================================================================

func TestBeginReorg_ZeroHeights(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	session, err := coord.BeginReorg(context.Background(), 0, 0, 0)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestExecuteReorg_NilSession(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	err := coord.ExecuteReorg(context.Background(), nil, mockBlockProvider(1, 10))

	// Assert
	assert.Error(t, err)
}

func TestCommitReorg_NilSession(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	err := coord.CommitReorg(context.Background(), nil)

	// Assert
	assert.Error(t, err)
}

func TestAbortReorg_NilSession(t *testing.T) {
	// Arrange
	coord := defaultCoordinator(t)

	// Act
	err := coord.AbortReorg(context.Background(), nil, errors.New("test"))

	// Assert
	assert.Error(t, err)
}

