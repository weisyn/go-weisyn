package difficulty

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

const (
	ppmDenom = uint64(1_000_000)
)

// Params defines consensus-critical parameters for difficulty and timestamp rules.
//
// ⚠️ All ratio parameters MUST be expressed in PPM (parts-per-million).
// This package must remain deterministic across nodes given the same chain state,
// except for the "future drift" check which intentionally uses local wall clock.
type Params struct {
	// TargetBlockTimeSeconds desired average block interval (seconds), must be > 0.
	TargetBlockTimeSeconds uint64

	// DifficultyWindow blocks in retarget window (>=2).
	DifficultyWindow uint64

	// MaxAdjustUpPPM / MaxAdjustDownPPM clamp range for adjustment factor.
	// - MaxAdjustUpPPM >= 1_000_000
	// - 0 < MaxAdjustDownPPM <= 1_000_000
	MaxAdjustUpPPM   uint64
	MaxAdjustDownPPM uint64

	// EMAAlphaPPM smoothing parameter:
	// - 0 disables EMA (direct retarget)
	// - 0..1_000_000 enables EMA
	EMAAlphaPPM uint64

	// MinDifficulty / MaxDifficulty clamp range for final difficulty.
	MinDifficulty uint64
	MaxDifficulty uint64

	// MTPWindow median time past window size (typically 11).
	MTPWindow uint64

	// MinBlockIntervalSeconds enforces minimum timestamp delta from parent.
	MinBlockIntervalSeconds uint64

	// MaxFutureDriftSeconds allows rejecting blocks too far in the future.
	MaxFutureDriftSeconds uint64

	// EmergencyDownshiftThresholdSeconds triggers deterministic "emergency downshift"
	// when the gap(parentTS->childTS) is too large (anti-stall).
	//
	// - 0 disables emergency downshift.
	// - When enabled, the per-block deltaBits clamp [-1,+1] is bypassed for downward
	//   direction to allow faster recovery after long stalls (Bitcoin testnet-like).
	EmergencyDownshiftThresholdSeconds uint64
	// MaxEmergencyDownshiftBits caps the magnitude of emergency downshift (>=1).
	MaxEmergencyDownshiftBits uint64
}

// BlockHeightReader is the minimal interface required to read blocks by height.
// Used by difficulty retargeting to avoid pulling in wider query interfaces.
type BlockHeightReader interface {
	GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error)
}

// BlockHashReader is the minimal interface required to read blocks by hash.
type BlockHashReader interface {
	GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error)
}

// Query is the minimal interface required for timestamp rule validation (parent hash + MTP by height).
type Query interface {
	BlockHeightReader
	BlockHashReader
}

// QueryWithBatch extends Query with batch reading capability for optimized MTP calculation.
type QueryWithBatch interface {
	Query
	BlockHeaderBatchReader
}

func (p Params) Validate() error {
	if p.TargetBlockTimeSeconds == 0 {
		return errors.New("TargetBlockTimeSeconds must be > 0")
	}
	if p.DifficultyWindow < 2 {
		return errors.New("DifficultyWindow must be >= 2")
	}
	if p.MaxAdjustUpPPM < ppmDenom {
		return fmt.Errorf("MaxAdjustUpPPM must be >= %d", ppmDenom)
	}
	if p.MaxAdjustDownPPM == 0 || p.MaxAdjustDownPPM > ppmDenom {
		return fmt.Errorf("MaxAdjustDownPPM must be in (0, %d]", ppmDenom)
	}
	if p.EMAAlphaPPM > ppmDenom {
		return fmt.Errorf("EMAAlphaPPM must be in [0, %d]", ppmDenom)
	}
	if p.MinDifficulty == 0 {
		return errors.New("MinDifficulty must be >= 1")
	}
	if p.MaxDifficulty > 0 && p.MaxDifficulty < p.MinDifficulty {
		return errors.New("MaxDifficulty must be >= MinDifficulty")
	}
	if p.MTPWindow == 0 {
		return errors.New("MTPWindow must be >= 1")
	}
	if p.EmergencyDownshiftThresholdSeconds > 0 {
		if p.MaxEmergencyDownshiftBits == 0 {
			return errors.New("MaxEmergencyDownshiftBits must be >= 1 when EmergencyDownshiftThresholdSeconds is enabled")
		}
		// Keep this within int64 safe range (applyDeltaBitsClamped uses int64).
		if p.MaxEmergencyDownshiftBits > uint64(^uint64(0)>>1) {
			return errors.New("MaxEmergencyDownshiftBits too large")
		}
	}
	return nil
}

// NextDifficulty computes next block difficulty for height = parent.Height + 1.
//
// Deterministic: only depends on chain state timestamps/difficulty and Params.
func NextDifficulty(ctx context.Context, q BlockHeightReader, parentHeader *core.BlockHeader, params Params) (uint64, error) {
	// Legacy entrypoint: keep deterministic behavior by assuming the earliest allowed
	// timestamp for the next block (i.e., parentTS + MinBlockInterval).
	//
	// NOTE:
	// - Mining/validation should prefer NextDifficultyForTimestamp, which binds the
	//   expected difficulty to the block's own timestamp to allow recovery after long stalls.
	if err := params.Validate(); err != nil {
		return 0, err
	}
	if q == nil {
		return 0, errors.New("query service is nil")
	}
	if parentHeader == nil {
		return 0, errors.New("parent header is nil")
	}

	childTS := parentHeader.Timestamp + params.MinBlockIntervalSeconds
	return NextDifficultyForTimestamp(ctx, q, parentHeader, childTS, params)
}

// NextDifficultyForTimestamp computes the expected difficulty for a child block at
// height = parent.Height + 1 with the given childTimestamp.
//
// Rationale:
//   - If the chain stalls (no block for a long time), "next difficulty" that depends
//     only on historical blocks cannot adapt until a new block appears.
//   - Binding difficulty to the block's own timestamp (still constrained by MTP/min-interval/future-drift)
//     allows the network to recover similarly to Bitcoin testnet style behavior.
//
// Deterministic: depends only on chain state, childTimestamp and Params.
func NextDifficultyForTimestamp(ctx context.Context, q BlockHeightReader, parentHeader *core.BlockHeader, childTimestamp uint64, params Params) (uint64, error) {
	if err := params.Validate(); err != nil {
		return 0, err
	}
	if q == nil {
		return 0, errors.New("query service is nil")
	}
	if parentHeader == nil {
		return 0, errors.New("parent header is nil")
	}

	parentDiff := parentHeader.Difficulty
	if parentDiff == 0 {
		return 0, errors.New("parent difficulty is 0")
	}

	// Window: last `DifficultyWindow` blocks ending at parent.
	parentHeight := parentHeader.Height
	var startHeight uint64
	if parentHeight+1 > params.DifficultyWindow {
		startHeight = parentHeight + 1 - params.DifficultyWindow
	} else {
		// ⚠️ 启动期/链早期特殊处理：排除创世块（height=0）参与难度窗口统计。
		//
		// 背景：
		// - genesis.timestamp 在配置中是固定的历史时间（例如 2024-01-01），
		//   而后续区块 timestamp 取当前时间；
		// - 若窗口统计包含 height=0，则 (last-first) 会被“创世到当前”的巨大间隔主导，
		//   使 T_actual 远大于 TargetBlockTimeSeconds，从而让算法倾向下调难度并被 MinDifficulty 卡死；
		// - 结果是：PoW 维持极低难度，出块节奏长期贴着 MinBlockIntervalSeconds（喷发式出块）。
		//
		// 设计：
		// - 在 parentHeight>=1 且窗口尚未满（<DifficultyWindow）时，窗口起点使用 1；
		// - 对 height=1（parentHeight=0）仍保持起点为 0，使第一个块继承创世难度并保持兼容。
		if parentHeight >= 1 {
			startHeight = 1
		} else {
			startHeight = 0
		}
	}

	firstTS, lastTS, count, err := windowTimestamps(ctx, q, startHeight, parentHeight)
	if err != nil {
		return 0, err
	}
	// Not enough samples: keep parent difficulty.
	if count < 2 || lastTS <= firstTS {
		return clampDifficulty(parentDiff, params), nil
	}

	// T_actual = (last-first)/(count-1)
	tActualHist := (lastTS - firstTS) / uint64(count-1)
	if tActualHist == 0 {
		// Extremely fast blocks: keep minimum non-zero for division safety.
		tActualHist = 1
	}

	// Also consider the real gap between parent and the candidate block timestamp.
	// This is critical to prevent runaway "difficulty increase" and enable recovery
	// after long stalls without depending on local wall clock inside consensus-critical math.
	if childTimestamp < parentHeader.Timestamp {
		return 0, fmt.Errorf("child timestamp regression: parent=%d child=%d", parentHeader.Timestamp, childTimestamp)
	}
	gap := childTimestamp - parentHeader.Timestamp

	// Use the slower one to avoid overreacting upward when the chain is already slow.
	// - fast history but slow current gap => decrease/relax difficulty
	// - slow history but fast current gap => history still dominates (stability)
	tActual := tActualHist
	if gap > tActual {
		tActual = gap
	}
	if tActual == 0 {
		// Shouldn't happen, but keep safe.
		tActual = 1
	}

	// === Emergency downshift (anti-stall, deterministic) ===
	//
	// When a block arrives after a very long gap, the "per-block step clamp" (-1)
	// is too conservative and can keep the chain stalled for minutes/hours.
	//
	// We approximate required downshift in log2 domain:
	// - if gap is K * target, decrease difficulty by ~log2(K) bits (capped).
	if params.EmergencyDownshiftThresholdSeconds > 0 && gap >= params.EmergencyDownshiftThresholdSeconds {
		// ratioGap = gap / target (PPM)
		ratioGapPPM := mulDiv64(gap, ppmDenom, params.TargetBlockTimeSeconds)
		downBits := roundLog2FromPPM(ratioGapPPM)
		if downBits < 1 {
			downBits = 1
		}
		maxBits := int64(params.MaxEmergencyDownshiftBits)
		if maxBits < 1 {
			maxBits = 1
		}
		if downBits > maxBits {
			downBits = maxBits
		}
		// Direct apply without EMA and without per-block [-1,+1] clamp.
		targetDiff := applyDeltaBitsClamped(parentDiff, -downBits, params)
		return clampDifficulty(targetDiff, params), nil
	}

	// === Difficulty 语义一致性说明（共识关键） ===
	//
	// 当前 PoW 校验使用：Target = 2^(256 - Difficulty)
	// => Difficulty 实际上是“位难度/前导零位数”的近似（log2 难度域）：
	//    - Difficulty 每 +1，期望挖矿时间约 *2
	//    - Difficulty 每 -1，期望挖矿时间约 /2
	//
	// 因此，难度 retarget 应在 log2 域做加减，而不是在线性域直接乘法缩放 difficulty。
	//
	// 1) 先算倍率 ratio = T_target / T_actual（PPM），并做限幅；
	// 2) 再把倍率映射到“位难度增量” deltaBits ≈ round(log2(ratio))；
	// 3) targetDiff = parentDiff + deltaBits；
	// 4) （可选）用 EMA 在 log2 域做平滑（避免抖动 / 取整粘滞）。
	ratioRawPPM := mulDiv64(params.TargetBlockTimeSeconds, ppmDenom, tActual)
	ratioPPM := clampPPM(ratioRawPPM, params.MaxAdjustDownPPM, params.MaxAdjustUpPPM)

	deltaBits := roundLog2FromPPM(ratioPPM)
	// v2：每块最大步进限制（确定性规则）
	//
	// 背景：
	// - Difficulty 在当前 PoW 语义下近似为“前导零位数 / log2 域”；
	// - ratio 映射到 Δbits 后，可能出现一次跳变（例如 ratio=4x => Δbits=+2）；
	// - 在启动期（窗口未满）或 min_block_interval 很小（例如 2s）且 target 很大（例如 30s）时，
	//   如果允许每块 +2/+3 这种快速上调，容易在几十个块内推到难以挖出，造成“生产停滞”；
	// - 另一方面，回落过快也会导致喷发式出块。
	//
	// 设计：
	// - 将每个区块的 Δbits 限制在 [-1, +1]，用更多区块逐步逼近目标；
	// - 这与 ratio 的窗口级 clamp（MaxAdjustUp/Down）是正交的，属于“每块控制器增益”。
	deltaBits = clampDeltaBits(deltaBits, -1, 1)
	targetDiff := applyDeltaBitsClamped(parentDiff, deltaBits, params)

	var next uint64
	if params.EMAAlphaPPM == 0 {
		next = targetDiff
	} else {
		// next = parent*(1-a) + target*a
		//
		// ⚠️ 注意：这里必须避免“分项 floor 导致的粘滞”：
		// 当 parentDiff 很小（典型为 1）且 a<1 时，分项 floor 会长期算出 0，
		// 使 next 无法从 1 上调，即使 targetDiff 已明显更大。
		//
		// 解决：用 128-bit 累加后做一次除法，并使用 ceil 保证在“应上调”时能产生可见变化。
		next = weightedAvgPPMCeil(parentDiff, targetDiff, params.EMAAlphaPPM)
	}

	return clampDifficulty(next, params), nil
}

// ValidateTimestampRules validates consensus timestamp rules for the given block header.
//
// - timestamp >= MTP(parent)
// - timestamp - parent.timestamp >= MinBlockIntervalSeconds
// - timestamp <= now + MaxFutureDriftSeconds (local wall clock check)
func ValidateTimestampRules(ctx context.Context, q Query, header *core.BlockHeader, params Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	if q == nil {
		return errors.New("query service is nil")
	}
	if header == nil {
		return errors.New("header is nil")
	}

	// Genesis: allow (other rules may still apply elsewhere).
	if header.Height == 0 {
		return nil
	}

	if len(header.PreviousHash) != 32 {
		return fmt.Errorf("invalid previous hash length: %d", len(header.PreviousHash))
	}

	parentBlock, err := q.GetBlockByHash(ctx, header.PreviousHash)
	if err != nil {
		return fmt.Errorf("failed to load parent block by hash: %w", err)
	}
	if parentBlock == nil || parentBlock.Header == nil {
		return errors.New("parent block is nil")
	}

	parentTS := parentBlock.Header.Timestamp
	if header.Timestamp < parentTS {
		// ✅ REORG 模式下放宽时间戳回归限制
		// 背景：分叉链的区块可能在不同时间被挖出，时间戳可能不完全有序
		// 允许在 REORG 场景下容忍一定范围内的时间戳回归（±1小时）
		isReorgMode := ctx.Value("reorg_mode")
		timeDiff := parentTS - header.Timestamp
		
		if isReorgMode != nil && timeDiff <= 3600 {
			// REORG模式且偏差在1小时内：允许通过但记录警告
			// 注意：这里不记录日志以避免在policy包中引入logger依赖
			// 调用方（BlockProcessor/Validator）应该记录此类警告
			return nil
		}
		
		return fmt.Errorf("timestamp regression: parent=%d child=%d diff=%ds", parentTS, header.Timestamp, timeDiff)
	}

	// Min interval
	minTS := parentTS + params.MinBlockIntervalSeconds
	if header.Timestamp < minTS {
		return fmt.Errorf("block too fast: parent_ts=%d child_ts=%d min_interval=%ds",
			parentTS, header.Timestamp, params.MinBlockIntervalSeconds)
	}

	// MTP
	mtp, err := medianTimePast(ctx, q, parentBlock.Header.Height, params.MTPWindow)
	if err != nil {
		return fmt.Errorf("failed to compute MTP: %w", err)
	}
	if header.Timestamp < mtp {
		return fmt.Errorf("timestamp below MTP: mtp=%d child_ts=%d", mtp, header.Timestamp)
	}

	// Future drift (local time)
	if params.MaxFutureDriftSeconds > 0 {
		now := uint64(time.Now().Unix())
		if header.Timestamp > now+params.MaxFutureDriftSeconds {
			return fmt.Errorf("timestamp too far in future: now=%d child_ts=%d drift=%ds",
				now, header.Timestamp, params.MaxFutureDriftSeconds)
		}
	}

	return nil
}

// MedianTimePast returns the median timestamp of the last `window` blocks ending at height `endHeight`.
// This is deterministic given the same chain state.
func MedianTimePast(ctx context.Context, q BlockHeightReader, endHeight uint64, window uint64) (uint64, error) {
	return medianTimePast(ctx, q, endHeight, window)
}

// MedianTimePastWithCache returns the median timestamp using cache.
// This is the preferred method for high-frequency MTP queries.
func MedianTimePastWithCache(ctx context.Context, q BlockHeightReader, endHeight uint64, window uint64, cache *MTPCache) (uint64, error) {
	if cache == nil {
		return medianTimePast(ctx, q, endHeight, window)
	}
	return cache.ComputeAndCache(ctx, q, endHeight, window)
}

// ValidateTimestampRulesWithCache validates consensus timestamp rules using MTP cache.
//
// This is an optimized version of ValidateTimestampRules that:
// 1. Uses MTP cache to avoid repeated calculations
// 2. Supports batch reading if available
// 3. Reduces database I/O pressure during sync
//
// Parameters:
//   - ctx: context
//   - q: query service (BlockHeightReader + BlockHashReader)
//   - header: block header to validate
//   - params: consensus parameters
//   - cache: MTP cache (if nil, falls back to non-cached validation)
//
// Returns:
//   - error: validation error or nil if valid
func ValidateTimestampRulesWithCache(ctx context.Context, q Query, header *core.BlockHeader, params Params, cache *MTPCache) error {
	if err := params.Validate(); err != nil {
		return err
	}
	if q == nil {
		return errors.New("query service is nil")
	}
	if header == nil {
		return errors.New("header is nil")
	}

	// Genesis: allow (other rules may still apply elsewhere).
	if header.Height == 0 {
		return nil
	}

	if len(header.PreviousHash) != 32 {
		return fmt.Errorf("invalid previous hash length: %d", len(header.PreviousHash))
	}

	parentBlock, err := q.GetBlockByHash(ctx, header.PreviousHash)
	if err != nil {
		return fmt.Errorf("failed to load parent block by hash: %w", err)
	}
	if parentBlock == nil || parentBlock.Header == nil {
		return errors.New("parent block is nil")
	}

	parentTS := parentBlock.Header.Timestamp
	if header.Timestamp < parentTS {
		// ✅ REORG 模式下放宽时间戳回归限制
		isReorgMode := ctx.Value("reorg_mode")
		timeDiff := parentTS - header.Timestamp

		if isReorgMode != nil && timeDiff <= 3600 {
			return nil
		}

		return fmt.Errorf("timestamp regression: parent=%d child=%d diff=%ds", parentTS, header.Timestamp, timeDiff)
	}

	// Min interval
	minTS := parentTS + params.MinBlockIntervalSeconds
	if header.Timestamp < minTS {
		return fmt.Errorf("block too fast: parent_ts=%d child_ts=%d min_interval=%ds",
			parentTS, header.Timestamp, params.MinBlockIntervalSeconds)
	}

	// MTP with cache
	var mtp uint64
	if cache != nil {
		mtp, err = cache.ComputeAndCache(ctx, q, parentBlock.Header.Height, params.MTPWindow)
	} else {
		mtp, err = medianTimePast(ctx, q, parentBlock.Header.Height, params.MTPWindow)
	}
	if err != nil {
		return fmt.Errorf("failed to compute MTP: %w", err)
	}
	if header.Timestamp < mtp {
		return fmt.Errorf("timestamp below MTP: mtp=%d child_ts=%d", mtp, header.Timestamp)
	}

	// Future drift (local time)
	if params.MaxFutureDriftSeconds > 0 {
		now := uint64(time.Now().Unix())
		if header.Timestamp > now+params.MaxFutureDriftSeconds {
			return fmt.Errorf("timestamp too far in future: now=%d child_ts=%d drift=%ds",
				now, header.Timestamp, params.MaxFutureDriftSeconds)
		}
	}

	return nil
}

// EarliestAllowedTimestamp returns the earliest valid timestamp for the next block,
// based on the parent header and v2 timestamp rules.
//
// It does NOT consult local time; callers can decide whether to wait or abort.
func EarliestAllowedTimestamp(ctx context.Context, q Query, parentHeader *core.BlockHeader, params Params) (uint64, error) {
	return EarliestAllowedTimestampWithCache(ctx, q, parentHeader, params, nil)
}

// EarliestAllowedTimestampWithCache returns the earliest valid timestamp using MTP cache.
func EarliestAllowedTimestampWithCache(ctx context.Context, q Query, parentHeader *core.BlockHeader, params Params, cache *MTPCache) (uint64, error) {
	if err := params.Validate(); err != nil {
		return 0, err
	}
	if q == nil {
		return 0, errors.New("query is nil")
	}
	if parentHeader == nil {
		return 0, errors.New("parent header is nil")
	}

	parentTS := parentHeader.Timestamp
	minTS := parentTS + params.MinBlockIntervalSeconds

	var mtp uint64
	var err error
	if cache != nil {
		mtp, err = cache.ComputeAndCache(ctx, q, parentHeader.Height, params.MTPWindow)
	} else {
		mtp, err = medianTimePast(ctx, q, parentHeader.Height, params.MTPWindow)
	}
	if err != nil {
		return 0, err
	}

	if mtp > minTS {
		return mtp, nil
	}
	return minTS, nil
}

// medianTimePast returns the median timestamp of the last `window` blocks ending at height `endHeight`.
func medianTimePast(ctx context.Context, q BlockHeightReader, endHeight uint64, window uint64) (uint64, error) {
	if window == 0 {
		return 0, errors.New("window must be >= 1")
	}

	var start uint64
	if endHeight+1 > window {
		start = endHeight + 1 - window
	} else {
		start = 0
	}

	timestamps := make([]uint64, 0, endHeight-start+1)
	for h := start; h <= endHeight; h++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		b, err := q.GetBlockByHeight(ctx, h)
		if err != nil {
			return 0, fmt.Errorf("get block by height %d: %w", h, err)
		}
		if b == nil || b.Header == nil {
			return 0, fmt.Errorf("block/header nil at height %d", h)
		}
		timestamps = append(timestamps, b.Header.Timestamp)
	}

	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	return timestamps[len(timestamps)/2], nil
}

func windowTimestamps(ctx context.Context, q BlockHeightReader, startHeight, endHeight uint64) (first uint64, last uint64, count int, err error) {
	if startHeight > endHeight {
		return 0, 0, 0, errors.New("startHeight > endHeight")
	}
	var firstSet bool
	for h := startHeight; h <= endHeight; h++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, 0, err
		}
		b, err := q.GetBlockByHeight(ctx, h)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("get block by height %d: %w", h, err)
		}
		if b == nil || b.Header == nil {
			return 0, 0, 0, fmt.Errorf("block/header nil at height %d", h)
		}
		if !firstSet {
			first = b.Header.Timestamp
			firstSet = true
		}
		last = b.Header.Timestamp
		count++
	}
	return first, last, count, nil
}

func clampPPM(v, lo, hi uint64) uint64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampDifficulty(v uint64, p Params) uint64 {
	if v < p.MinDifficulty {
		v = p.MinDifficulty
	}
	if p.MaxDifficulty > 0 && v > p.MaxDifficulty {
		v = p.MaxDifficulty
	}
	if v == 0 {
		return 1
	}
	return v
}

// mulDiv64 computes floor((a*b)/denom) using 128-bit intermediate.
func mulDiv64(a, b, denom uint64) uint64 {
	if denom == 0 {
		// caller bug; avoid panic
		return 0
	}
	hi, lo := bits.Mul64(a, b)
	q, _ := bits.Div64(hi, lo, denom)
	return q
}

// roundLog2FromPPM approximates round(log2(ratioPPM/1e6)) as an int64.
//
// 实现策略（确定性、无浮点）：
// - 通过不断 *2 或 /2，把 ratio 归一化到 [1,2) 区间（以 PPM 表示）
// - 再用阈值 sqrt(2) 进行四舍五入（>=sqrt(2) 则进位）
//
// 说明：ratioPPM 会先经过 MaxAdjustUp/Down 限幅（默认 4x / 0.25x），
// 因此这里的循环次数通常很小。
func roundLog2FromPPM(ratioPPM uint64) int64 {
	if ratioPPM == 0 {
		return 0
	}

	const sqrt2PPM = uint64(1_414_214) // round(1e6 * sqrt(2))

	k := int64(0)
	x := ratioPPM

	// 归一化到 [1e6, 2e6)
	for x >= 2*ppmDenom {
		x /= 2
		k++
	}
	for x < ppmDenom {
		x *= 2
		k--
	}

	// 四舍五入：阈值为 sqrt(2)
	if x >= sqrt2PPM {
		return k + 1
	}
	return k
}

func clampDeltaBits(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func applyDeltaBitsClamped(parentDiff uint64, deltaBits int64, params Params) uint64 {
	if deltaBits == 0 {
		return clampDifficulty(parentDiff, params)
	}

	var v int64 = int64(parentDiff) + deltaBits
	if v < 1 {
		v = 1
	}
	return clampDifficulty(uint64(v), params)
}

// weightedAvgPPMCeil computes ceil((parent*(1-a)+target*a)/1e6) with 128-bit intermediates.
// Deterministic across nodes.
func weightedAvgPPMCeil(parentDiff, targetDiff, alphaPPM uint64) uint64 {
	if alphaPPM == 0 {
		if targetDiff == 0 {
			return 1
		}
		return targetDiff
	}
	if alphaPPM >= ppmDenom {
		// alpha=1 => fully follow target
		if targetDiff == 0 {
			return 1
		}
		return targetDiff
	}

	parentW := ppmDenom - alphaPPM
	hi1, lo1 := bits.Mul64(parentDiff, parentW)
	hi2, lo2 := bits.Mul64(targetDiff, alphaPPM)

	lo, carry := bits.Add64(lo1, lo2, 0)
	hi := hi1 + hi2 + carry

	// ceil: (num + denom - 1) / denom
	lo, carry = bits.Add64(lo, ppmDenom-1, 0)
	hi += carry

	q, _ := bits.Div64(hi, lo, ppmDenom)
	if q == 0 {
		return 1
	}
	return q
}
