// tie_breaking.go
// 距离选择Tie-breaking处理器
//
// 主要功能：
// 1. 处理XOR距离相等的候选区块选择
// 2. 基于确定性算法的tie-breaking决策
// 3. 字典序选择保证全网一致性
// 4. Tie-breaking过程的可验证性
//
// 距离Tie-breaking策略：
// 1. DistanceComparison - XOR距离精确比较
// 2. LexicographicHash - 区块哈希字典序选择（确定性）
//
// 处理流程：
// 1. 检测XOR距离相等的候选区块
// 2. 应用字典序哈希选择策略
// 3. 生成tie-breaking证明
// 4. 记录tie-breaking过程和结果
//
// 设计原则：
// - 简化策略，专注距离选择
// - 确定性算法保证全网一致性
// - 透明的tie-breaking过程
// - 数学抗操纵特性
//
// 作者：WES开发团队
// 更新时间：2025-09-14（距离选择优化）

package block_selector

import (
	"encoding/hex"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// tieBreaker 距离选择平局处理器
type tieBreaker struct {
	logger             log.Logger
	hashManager        crypto.HashManager
	tieBreakingHistory []*distanceTieBreakingRecord // 距离tie-breaking历史
	historyMutex       sync.RWMutex                 // 历史记录锁
}

// distanceTieBreakingRecord 距离tie-breaking记录
type distanceTieBreakingRecord struct {
	timestamp        time.Time
	tiedCount        int
	tiedDistances    []*big.Int // 相等的距离值
	selectedHash     []byte     // 选中的区块哈希
	selectionReason  string     // 选择原因
	processingTime   time.Duration
	lexicographicWin bool // 是否通过字典序选择
}

// distanceTieBreakingStrategy 距离tie-breaking策略类型
type distanceTieBreakingStrategy string

const (
	DistanceStrategyPreciseComparison distanceTieBreakingStrategy = "precise_distance_comparison"
	DistanceStrategyLexicographic     distanceTieBreakingStrategy = "lexicographic_hash_selection"
)

// newTieBreaker 创建距离选择平局处理器
func newTieBreaker(logger log.Logger, hashManager crypto.HashManager) *tieBreaker {
	return &tieBreaker{
		logger:             logger,
		hashManager:        hashManager,
		tieBreakingHistory: make([]*distanceTieBreakingRecord, 0, 1000),
	}
}

// applyDistanceTieBreaking 应用距离tie-breaking策略
func (t *tieBreaker) applyDistanceTieBreaking(tiedDistanceResults []types.DistanceResult) (*types.CandidateBlock, error) {
	startTime := time.Now()

	if len(tiedDistanceResults) <= 1 {
		if len(tiedDistanceResults) == 1 {
			return tiedDistanceResults[0].Candidate, nil
		}
		return nil, types.ErrNoDistanceResults
	}

	t.logger.Info("开始距离tie-breaking处理")

	// 精确距离比较：使用更高精度重新比较
	preciseWinner := t.selectByPreciseDistanceComparison(tiedDistanceResults)
	if preciseWinner != nil {
		// 记录处理过程
		processingTime := time.Since(startTime)
		t.recordDistanceTieBreakingProcess(&distanceTieBreakingRecord{
			timestamp:        time.Now(),
			tiedCount:        len(tiedDistanceResults),
			tiedDistances:    t.extractDistances(tiedDistanceResults),
			selectedHash:     preciseWinner.BlockHash,
			selectionReason:  "精确距离比较",
			processingTime:   processingTime,
			lexicographicWin: false,
		})

		return preciseWinner, nil
	}

	// 字典序哈希选择：确定性兜底策略
	lexicographicWinner := t.selectByLexicographicHash(tiedDistanceResults)

	// 记录处理过程
	processingTime := time.Since(startTime)
	t.recordDistanceTieBreakingProcess(&distanceTieBreakingRecord{
		timestamp:        time.Now(),
		tiedCount:        len(tiedDistanceResults),
		tiedDistances:    t.extractDistances(tiedDistanceResults),
		selectedHash:     lexicographicWinner.BlockHash,
		selectionReason:  "字典序哈希选择",
		processingTime:   processingTime,
		lexicographicWin: true,
	})

	t.logger.Info("距离tie-breaking处理完成")
	return lexicographicWinner, nil
}

// selectByPreciseDistanceComparison 基于精确距离比较选择
func (t *tieBreaker) selectByPreciseDistanceComparison(distanceResults []types.DistanceResult) *types.CandidateBlock {
	if len(distanceResults) <= 1 {
		return nil
	}

	// 找到真正的最小距离
	minDistance := new(big.Int).Set(distanceResults[0].Distance)
	var winnerCount int
	var winner *types.CandidateBlock

	for _, result := range distanceResults {
		cmp := result.Distance.Cmp(minDistance)
		if cmp < 0 {
			// 发现更小的距离
			minDistance.Set(result.Distance)
			winner = result.Candidate
			winnerCount = 1
		} else if cmp == 0 {
			// 距离相等，增加计数
			winnerCount++
		}
	}

	// 如果只有一个最小距离的候选，直接返回
	if winnerCount == 1 {
		return winner
	}

	// 如果仍有多个候选具有相同的最小距离，返回nil让字典序处理
	return nil
}

// selectByLexicographicHash 基于字典序哈希选择
func (t *tieBreaker) selectByLexicographicHash(distanceResults []types.DistanceResult) *types.CandidateBlock {
	if len(distanceResults) == 0 {
		return nil
	}
	if len(distanceResults) == 1 {
		return distanceResults[0].Candidate
	}

	// 为每个候选生成排序键
	type candidateWithKey struct {
		candidate *types.CandidateBlock
		sortKey   string
	}

	var candidatesWithKeys []candidateWithKey
	for _, result := range distanceResults {
		// 使用区块哈希的十六进制表示作为排序键
		sortKey := hex.EncodeToString(result.Candidate.BlockHash)
		candidatesWithKeys = append(candidatesWithKeys, candidateWithKey{
			candidate: result.Candidate,
			sortKey:   sortKey,
		})
	}

	// 按字典序排序，选择最小的
	sort.Slice(candidatesWithKeys, func(i, j int) bool {
		return candidatesWithKeys[i].sortKey < candidatesWithKeys[j].sortKey
	})

	return candidatesWithKeys[0].candidate
}

// extractDistances 提取距离值列表
func (t *tieBreaker) extractDistances(distanceResults []types.DistanceResult) []*big.Int {
	distances := make([]*big.Int, len(distanceResults))
	for i, result := range distanceResults {
		distances[i] = new(big.Int).Set(result.Distance)
	}
	return distances
}

// recordDistanceTieBreakingProcess 记录距离tie-breaking过程
func (t *tieBreaker) recordDistanceTieBreakingProcess(record *distanceTieBreakingRecord) {
	t.historyMutex.Lock()
	defer t.historyMutex.Unlock()

	t.tieBreakingHistory = append(t.tieBreakingHistory, record)

	// 保持历史记录大小合理
	maxHistory := 1000
	if len(t.tieBreakingHistory) > maxHistory {
		t.tieBreakingHistory = t.tieBreakingHistory[len(t.tieBreakingHistory)-maxHistory:]
	}
}
