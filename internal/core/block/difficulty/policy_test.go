package difficulty

import (
	"context"
	"testing"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

type testHeightReader struct {
	blocks map[uint64]*core.Block
}

func (r *testHeightReader) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	return r.blocks[height], nil
}

func TestNextDifficulty_ExcludesGenesisFromEarlyWindow(t *testing.T) {
	// 构造一个“创世时间戳固定在历史、后续块在当前附近”的典型场景：
	// - height=0: timestamp=1704067200（2024-01-01 00:00:00 UTC），difficulty=1
	// - height=1: timestamp=1765528815（某次运行的当前时间附近），difficulty=1
	// - height=2: timestamp=1765528817（2s 后），difficulty=1
	//
	// 若窗口包含 genesis，则 last-first 被年份级间隔主导，tActual 极大，难度会被压到 MinDifficulty；
	// 修复后窗口在 early(<DifficultyWindow) 且 parentHeight>=1 时从 height=1 开始统计，
	// 对 parentHeight=2，将看到 tActual≈2s，从而应上调难度。
	r := &testHeightReader{
		blocks: map[uint64]*core.Block{
			0: {Header: &core.BlockHeader{Height: 0, Timestamp: 1704067200, Difficulty: 1}},
			1: {Header: &core.BlockHeader{Height: 1, Timestamp: 1765528815, Difficulty: 1}},
			2: {Header: &core.BlockHeader{Height: 2, Timestamp: 1765528817, Difficulty: 1}},
		},
	}

	params := Params{
		TargetBlockTimeSeconds:  30,
		DifficultyWindow:        100,
		MaxAdjustUpPPM:          4_000_000,
		MaxAdjustDownPPM:        250_000,
		EMAAlphaPPM:             0, // 关闭 EMA，断言更稳定
		MinDifficulty:           1,
		MaxDifficulty:           0,
		MTPWindow:               11,
		MinBlockIntervalSeconds: 2,
		MaxFutureDriftSeconds:   2 * 60 * 60,
	}

	next, err := NextDifficulty(context.Background(), r, r.blocks[2].Header, params)
	if err != nil {
		t.Fatalf("NextDifficulty returned error: %v", err)
	}
	if next <= 1 {
		t.Fatalf("expected difficulty to increase from 1, got %d", next)
	}
}

func TestNextDifficultyForTimestamp_LongGapRelaxesDifficulty(t *testing.T) {
	// 场景：历史窗口显示“很快”（2s 一块），理论上会持续上调难度；
	// 但当前新区块的 timestamp 与父块差距很大（例如 300s），
	// 则应允许在该块上回落难度（用于链恢复，类似 BTC testnet 的“长时间无块”语义）。
	r := &testHeightReader{
		blocks: map[uint64]*core.Block{
			1: {Header: &core.BlockHeader{Height: 1, Timestamp: 1_000, Difficulty: 10}},
			2: {Header: &core.BlockHeader{Height: 2, Timestamp: 1_002, Difficulty: 10}},
			3: {Header: &core.BlockHeader{Height: 3, Timestamp: 1_004, Difficulty: 10}},
		},
	}

	params := Params{
		TargetBlockTimeSeconds:  30,
		DifficultyWindow:        100,
		MaxAdjustUpPPM:          4_000_000,
		MaxAdjustDownPPM:        250_000,
		EMAAlphaPPM:             0,
		MinDifficulty:           1,
		MaxDifficulty:           0,
		MTPWindow:               11,
		MinBlockIntervalSeconds: 2,
		MaxFutureDriftSeconds:   2 * 60 * 60,
	}

	parent := r.blocks[3].Header
	childTS := parent.Timestamp + 300
	next, err := NextDifficultyForTimestamp(context.Background(), r, parent, childTS, params)
	if err != nil {
		t.Fatalf("NextDifficultyForTimestamp returned error: %v", err)
	}
	if next >= parent.Difficulty {
		t.Fatalf("expected difficulty to relax below %d due to long gap, got %d", parent.Difficulty, next)
	}
}

func TestNextDifficultyForTimestamp_ClampsPerBlockStepToOneBit(t *testing.T) {
	// 场景：窗口与当前 gap 都显示出块“过快”（2s），目标为 30s。
	// - ratio_raw = 30/2 = 15x，但会先被 MaxAdjustUpPPM=4x 限幅；
	// - log2(4x) = +2 bits，但 v2 规则要求每块 Δbits 进一步限制在 [-1,+1]；
	// => 因此难度每块最多只上调 1。
	r := &testHeightReader{
		blocks: map[uint64]*core.Block{
			1: {Header: &core.BlockHeader{Height: 1, Timestamp: 1_000, Difficulty: 10}},
			2: {Header: &core.BlockHeader{Height: 2, Timestamp: 1_002, Difficulty: 10}},
			3: {Header: &core.BlockHeader{Height: 3, Timestamp: 1_004, Difficulty: 10}},
		},
	}

	params := Params{
		TargetBlockTimeSeconds:  30,
		DifficultyWindow:        100,
		MaxAdjustUpPPM:          4_000_000,
		MaxAdjustDownPPM:        250_000,
		EMAAlphaPPM:             0, // 关闭 EMA，便于断言精确步进
		MinDifficulty:           1,
		MaxDifficulty:           0,
		MTPWindow:               11,
		MinBlockIntervalSeconds: 2,
		MaxFutureDriftSeconds:   2 * 60 * 60,
	}

	parent := r.blocks[3].Header
	childTS := parent.Timestamp + 2
	next, err := NextDifficultyForTimestamp(context.Background(), r, parent, childTS, params)
	if err != nil {
		t.Fatalf("NextDifficultyForTimestamp returned error: %v", err)
	}
	if next != parent.Difficulty+1 {
		t.Fatalf("expected difficulty to increase by exactly 1 bit, got parent=%d next=%d", parent.Difficulty, next)
	}
}

func TestNextDifficultyForTimestamp_EmergencyDownshiftBypassesPerBlockClamp(t *testing.T) {
	// 场景：
	// - 历史窗口很快（2s 一块），但当前 parent->child gap 极大（例如 600s）
	// - 正常模式下 deltaBits 会被 [-1,+1] 钳制，回落太慢，易导致“长时间停摆”
	// - 启用 emergency 后：允许一次性下调多个 bit（类似 BTC testnet 的长时间无块语义）
	r := &testHeightReader{
		blocks: map[uint64]*core.Block{
			1: {Header: &core.BlockHeader{Height: 1, Timestamp: 1_000, Difficulty: 20}},
			2: {Header: &core.BlockHeader{Height: 2, Timestamp: 1_002, Difficulty: 20}},
			3: {Header: &core.BlockHeader{Height: 3, Timestamp: 1_004, Difficulty: 20}},
		},
	}

	params := Params{
		TargetBlockTimeSeconds:  30,
		DifficultyWindow:        100,
		MaxAdjustUpPPM:          4_000_000,
		MaxAdjustDownPPM:        250_000,
		EMAAlphaPPM:             0,
		MinDifficulty:           1,
		MaxDifficulty:           0,
		MTPWindow:               11,
		MinBlockIntervalSeconds: 2,
		MaxFutureDriftSeconds:   2 * 60 * 60,

		// emergency：gap>=120s 时触发，最多下调 8 bit
		EmergencyDownshiftThresholdSeconds: 120,
		MaxEmergencyDownshiftBits:          8,
	}

	parent := r.blocks[3].Header
	childTS := parent.Timestamp + 600 // 600/30=20x => log2≈4.32，round≈4
	next, err := NextDifficultyForTimestamp(context.Background(), r, parent, childTS, params)
	if err != nil {
		t.Fatalf("NextDifficultyForTimestamp returned error: %v", err)
	}
	if next != parent.Difficulty-4 {
		t.Fatalf("expected emergency downshift to reduce by ~4 bits, got parent=%d next=%d", parent.Difficulty, next)
	}
}
