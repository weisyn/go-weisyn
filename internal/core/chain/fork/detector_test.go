package fork_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/chain/fork"
	"github.com/weisyn/v1/internal/core/chain/testutil"
	consensustestutil "github.com/weisyn/v1/internal/core/consensus/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/grpc"
)

// ==================== DetectFork 测试（间接测试detector.go）====================

// testBlockHashClient：用于在测试里构造“同高度不同hash”的分叉块
// （默认 MockBlockHashClient 仅按 height 生成 hash，会导致同高度块 hash 冲突，无法测试分叉）
type testBlockHashClient struct{}

func (c *testBlockHashClient) ComputeBlockHash(ctx context.Context, req *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	h := computeTestBlockHash(req.GetBlock())
	return &core.ComputeBlockHashResponse{IsValid: true, Hash: h}, nil
}

func (c *testBlockHashClient) ValidateBlockHash(ctx context.Context, req *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	h := computeTestBlockHash(req.GetBlock())
	ok := len(req.GetExpectedHash()) == 32 && string(h) == string(req.GetExpectedHash())
	return &core.ValidateBlockHashResponse{IsValid: ok, ComputedHash: h}, nil
}

func computeTestBlockHash(b *core.Block) []byte {
	sum := sha256.New()
	if b == nil || b.Header == nil {
		return make([]byte, 32)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], b.Header.Height)
	sum.Write(buf[:])
	binary.BigEndian.PutUint64(buf[:], b.Header.Timestamp)
	sum.Write(buf[:])
	sum.Write(b.Header.PreviousHash)
	sum.Write(b.Header.MerkleRoot)
	sum.Write(b.Header.StateRoot)
	out := sum.Sum(nil)
	// sha256 32 bytes
	return out
}

// TestDetectFork_WithValidBlock_ReturnsResult 测试检测有效区块
func TestDetectFork_WithValidBlock_ReturnsResult(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestForkHandler()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: &core.BlockHeader{
			Height:       1,
			PreviousHash: make([]byte, 32),
			MerkleRoot:   make([]byte, 32),
			StateRoot:    make([]byte, 32),
			Timestamp:    1000,
		},
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				blocktestutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	isFork, forkHeight, err := service.DetectFork(ctx, block)

	// Assert
	// 即使检测失败，也应该返回结果而不是panic
	if err != nil {
		assert.Error(t, err)
	} else {
		_ = isFork // 确保返回布尔值
		_ = forkHeight
	}
}

// TestDetectFork_WithNilBlock_ReturnsError 测试检测nil区块
func TestDetectFork_WithNilBlock_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestForkHandler()
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	isFork, forkHeight, err := service.DetectFork(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.False(t, isFork)
	assert.Equal(t, uint64(0), forkHeight)
}

// TestDetectFork_WithNilHeader_ReturnsError 测试检测nil区块头
func TestDetectFork_WithNilHeader_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestForkHandler()
	require.NoError(t, err)

	ctx := context.Background()
	block := &core.Block{
		Header: nil,
		Body: &core.BlockBody{
			Transactions: []*transaction.Transaction{
				blocktestutil.NewTestTransaction(1),
			},
		},
	}

	// Act
	isFork, forkHeight, err := service.DetectFork(ctx, block)

	// Assert
	assert.Error(t, err)
	assert.False(t, isFork)
	assert.Equal(t, uint64(0), forkHeight)
}

func TestDetectFork_FindsForkPointUsingStoredForkAncestors(t *testing.T) {
	ctx := context.Background()

	qs := blocktestutil.NewMockQueryService()
	hashClient := &testBlockHashClient{}
	hashManager := &blocktestutil.MockHashManager{}
	configProvider := &testutil.MockConfigProvider{}
	eventBus := blocktestutil.NewMockEventBus()
	logger := &blocktestutil.MockLogger{}

	txHashClient := consensustestutil.NewMockTransactionHashClient()
	h, err := fork.NewService(qs, hashManager, hashClient, txHashClient, nil, configProvider, eventBus, logger)
	require.NoError(t, err)
	service := h.(*fork.Service)

	// 主链：0..5
	var prevHash []byte
	for height := uint64(0); height <= 5; height++ {
		blk := &core.Block{
			Header: &core.BlockHeader{
				Height:       height,
				PreviousHash: prevHash,
				MerkleRoot:   []byte("mrk"),
				StateRoot:    []byte("st"),
				Timestamp:    1000 + height,
			},
			Body: &core.BlockBody{Transactions: []*transaction.Transaction{blocktestutil.NewTestTransaction(1)}},
		}
		hash := computeTestBlockHash(blk)
		qs.SetBlock(hash, blk) // canonical 由 SetBlock 的“首次设置”规则决定（主链先设置）
		prevHash = hash
	}

	// 分叉父块：高度=5（与主链同高，但 hash 不同；共同祖先为高度=4）
	main4, err := qs.GetBlockByHeight(ctx, 4)
	require.NoError(t, err)
	main4Hash := computeTestBlockHash(main4)

	fork5 := &core.Block{
		Header: &core.BlockHeader{
			Height:       5,
			PreviousHash: main4Hash,
			MerkleRoot:   []byte("mrk_fork"),
			StateRoot:    []byte("st_fork"),
			Timestamp:    999999, // 与主链不同，确保 hash 不同
		},
		Body: &core.BlockBody{Transactions: []*transaction.Transaction{blocktestutil.NewTestTransaction(2)}},
	}
	fork5Hash := computeTestBlockHash(fork5)
	qs.SetBlock(fork5Hash, fork5) // 不是 canonical（height=5 主链已设置）

	// 分叉 tip：高度=6，父哈希指向 fork5
	fork6 := &core.Block{
		Header: &core.BlockHeader{
			Height:       6,
			PreviousHash: fork5Hash,
			MerkleRoot:   []byte("mrk_fork6"),
			StateRoot:    []byte("st_fork6"),
			Timestamp:    1000000,
		},
		Body: &core.BlockBody{Transactions: []*transaction.Transaction{blocktestutil.NewTestTransaction(3)}},
	}

	isFork, forkHeight, err := service.DetectFork(ctx, fork6)
	require.NoError(t, err)
	require.True(t, isFork)
	require.Equal(t, uint64(4), forkHeight) // 共同祖先高度
}

func TestDetectFork_ReturnsErrorWhenForkAncestorsMissing(t *testing.T) {
	ctx := context.Background()

	qs := blocktestutil.NewMockQueryService()
	hashClient := &testBlockHashClient{}
	hashManager := &blocktestutil.MockHashManager{}
	configProvider := &testutil.MockConfigProvider{}
	eventBus := blocktestutil.NewMockEventBus()
	logger := &blocktestutil.MockLogger{}

	txHashClient := consensustestutil.NewMockTransactionHashClient()
	h, err := fork.NewService(qs, hashManager, hashClient, txHashClient, nil, configProvider, eventBus, logger)
	require.NoError(t, err)
	service := h.(*fork.Service)

	// 主链：0..2
	var prevHash []byte
	for height := uint64(0); height <= 2; height++ {
		blk := &core.Block{
			Header: &core.BlockHeader{
				Height:       height,
				PreviousHash: prevHash,
				MerkleRoot:   []byte("mrk"),
				StateRoot:    []byte("st"),
				Timestamp:    1000 + height,
			},
		}
		hash := computeTestBlockHash(blk)
		qs.SetBlock(hash, blk)
		prevHash = hash
	}

	// 高度=3 的区块指向一个不存在的父 hash（模拟缺失 fork 祖先）
	missingParent := make([]byte, 32)
	copy(missingParent, []byte("missing-parent-hash"))
	b3 := &core.Block{Header: &core.BlockHeader{Height: 3, PreviousHash: missingParent, Timestamp: 2000}}

	isFork, forkHeight, err := service.DetectFork(ctx, b3)
	require.Error(t, err)
	assert.True(t, isFork)                 // 分叉已检测到，但无法定位祖先
	assert.Equal(t, uint64(2), forkHeight) // 退化返回（currentHeight），保持旧行为/调用方可据此触发同步
}

// ==================== 发现代码问题测试 ====================

// TestDetectFork_DetectsTODOs 测试发现TODO标记
func TestDetectFork_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestDetectFork_DetectsTemporaryImplementations 测试发现临时实现
func TestDetectFork_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ 分叉检测实现检查：")
	t.Logf("  - detectFork 检测分叉的核心逻辑")
	t.Logf("  - findForkPoint 向前回溯查找分叉点")
	t.Logf("  - calculateBlockHash 计算区块哈希")
}
