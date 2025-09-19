// select_best_block.go
// 基于评分的最优选择算法
//
// 主要功能：
// 1. 实现 SelectBestCandidate 方法
// 2. 基于评分结果的排序选择
// 3. 高效的选择算法实现
// 4. 选择过程的性能优化
// 5. 选择透明性保证
//
// 选择逻辑：
// 1. 验证评分结果的完整性
// 2. 按最终评分降序排序
// 3. 检查是否存在tie-breaking情况
// 4. 如有多个最高分则应用tie-breaking
// 5. 返回选择的最优候选区块
//
// 设计原则：
// - 基于科学评分的透明选择
// - 高效的排序和选择算法
// - 完整的选择过程记录
// - 选择结果的可验证性
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package block_selector

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// blockSelector 区块选择器
type blockSelector struct {
	logger           log.Logger
	tieBreaker       *tieBreaker        // 平局处理器
	selectionHistory []*selectionRecord // 选择历史
	historyMutex     sync.RWMutex       // 历史记录锁
}

// selectionRecord 选择记录
type selectionRecord struct {
	timestamp          time.Time
	candidatesCount    int
	selectedCandidate  *types.CandidateBlock
	selectionScore     float64
	tieBreakingApplied bool
	selectionTime      time.Duration
}

// newBlockSelector 创建区块选择器
func newBlockSelector(logger log.Logger, tieBreaker *tieBreaker) *blockSelector {
	return &blockSelector{
		logger:           logger,
		tieBreaker:       tieBreaker,
		selectionHistory: make([]*selectionRecord, 0, 1000),
	}
}

// selectBestCandidate 选择最优候选区块
func (s *blockSelector) selectBestCandidate(scores []types.ScoredCandidate) (*types.CandidateBlock, error) {
	startTime := time.Now()

	// 验证输入
	if len(scores) == 0 {
		return nil, errors.New("no candidates to select from")
	}

	// 验证评分完整性
	if err := s.validateScoreIntegrity(scores); err != nil {
		return nil, err
	}

	// 排序（按评分降序）
	sortedScores := s.sortCandidatesByScore(scores)

	// 检查平局情况
	topCandidates, hasTie := s.detectTiedScores(sortedScores)

	var selectedCandidate *types.CandidateBlock
	tieBreakingApplied := false

	if hasTie {
		// 注意：这是旧的评分tie-breaking逻辑，新架构应使用距离选择
		// TODO: 这个逻辑应该被重构为距离选择模式
		s.logger.Info("检测到评分tie-breaking情况，但距离选择应该已处理")
		// 暂时选择第一个作为fallback
		selectedCandidate = topCandidates[0].Candidate
		tieBreakingApplied = false
	} else {
		// 直接选择最高分候选
		selectedCandidate = sortedScores[0].Candidate
	}

	// 记录选择过程
	selectionTime := time.Since(startTime)
	s.recordSelectionProcess(&selectionRecord{
		timestamp:          time.Now(),
		candidatesCount:    len(scores),
		selectedCandidate:  selectedCandidate,
		selectionScore:     sortedScores[0].Score.NormalizedScore,
		tieBreakingApplied: tieBreakingApplied,
		selectionTime:      selectionTime,
	})

	return selectedCandidate, nil
}

// validateScoreIntegrity 验证评分完整性
func (s *blockSelector) validateScoreIntegrity(scores []types.ScoredCandidate) error {
	for i, scored := range scores {
		if scored.Candidate == nil {
			return errors.New("candidate is nil in scored candidate")
		}

		if scored.Score == nil {
			return errors.New("score is nil in scored candidate")
		}

		// 验证评分范围
		if scored.Score.NormalizedScore < 0 || scored.Score.NormalizedScore > 10.0 {
			return errors.New("normalized score out of valid range")
		}

		// 验证各分项评分
		if scored.Score.PoWQualityScore < 0 || scored.Score.EconomicScore < 0 ||
			scored.Score.TimelinesScore < 0 || scored.Score.NetworkScore < 0 {
			return errors.New("negative component score detected")
		}

		// 验证排名
		expectedRank := i + 1
		if scored.Rank != expectedRank {
			return errors.New("inconsistent ranking in scores")
		}
	}

	return nil
}

// sortCandidatesByScore 按评分排序候选区块
func (s *blockSelector) sortCandidatesByScore(scores []types.ScoredCandidate) []types.ScoredCandidate {
	// 创建副本以避免修改原数据
	sortedScores := make([]types.ScoredCandidate, len(scores))
	copy(sortedScores, scores)

	// 按标准化评分降序排序
	sort.Slice(sortedScores, func(i, j int) bool {
		return sortedScores[i].Score.NormalizedScore > sortedScores[j].Score.NormalizedScore
	})

	return sortedScores
}

// detectTiedScores 检测评分相同的情况
func (s *blockSelector) detectTiedScores(sortedScores []types.ScoredCandidate) ([]types.ScoredCandidate, bool) {
	if len(sortedScores) <= 1 {
		return sortedScores[:1], false
	}

	// 获取最高分
	maxScore := sortedScores[0].Score.NormalizedScore
	tolerance := 1e-6 // 浮点数比较容差

	// 收集所有与最高分相等的候选
	var tiedCandidates []types.ScoredCandidate
	for _, scored := range sortedScores {
		if math.Abs(scored.Score.NormalizedScore-maxScore) <= tolerance {
			tiedCandidates = append(tiedCandidates, scored)
		} else {
			break // 由于是降序排序，后面的分数都更低
		}
	}

	hasTie := len(tiedCandidates) > 1
	return tiedCandidates, hasTie
}

// recordSelectionProcess 记录选择过程
func (s *blockSelector) recordSelectionProcess(record *selectionRecord) {
	s.historyMutex.Lock()
	defer s.historyMutex.Unlock()

	s.selectionHistory = append(s.selectionHistory, record)

	// 保持历史记录大小合理
	maxHistory := 1000
	if len(s.selectionHistory) > maxHistory {
		s.selectionHistory = s.selectionHistory[len(s.selectionHistory)-maxHistory:]
	}
}
