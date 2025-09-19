// block_selector.go
// 基于距离的区块选择器
//
// 主要功能：
// 1. 从距离计算结果中选择最优区块
// 2. 处理距离相等的tie-breaking情况
// 3. 确保选择的确定性和一致性
// 4. 提供选择过程的透明性
//
// 选择算法：
// 1. 找到所有最小距离的候选区块
// 2. 如果只有一个，直接选择
// 3. 如果多个候选距离相等，使用确定性tie-breaking
// 4. Tie-breaking策略：按字典序比较区块哈希
//
// 设计原则：
// - 确定性：相同输入必定产生相同选择
// - 简洁性：避免复杂的tie-breaking逻辑
// - 高效性：O(n)时间复杂度
// - 可验证性：任何节点都能验证选择正确性
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package distance_selector

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// blockDistanceSelector 基于距离的区块选择器
type blockDistanceSelector struct {
	logger             log.Logger
	distanceCalculator *distanceCalculator

	// 选择历史记录
	selectionHistory []selectionRecord
	historyMutex     sync.RWMutex
}

// selectionRecord 选择记录
type selectionRecord struct {
	timestamp       time.Time
	candidatesCount int
	selectedBlock   *types.CandidateBlock
	minDistance     string // big.Int转换为字符串存储
	tieBreakApplied bool
	selectionTime   time.Duration
}

// newBlockDistanceSelector 创建基于距离的区块选择器
func newBlockDistanceSelector(
	logger log.Logger,
	calculator *distanceCalculator,
) *blockDistanceSelector {
	return &blockDistanceSelector{
		logger:             logger,
		distanceCalculator: calculator,
		selectionHistory:   make([]selectionRecord, 0, 1000),
	}
}

// selectClosest 选择距离最近的区块
func (s *blockDistanceSelector) selectClosest(
	ctx context.Context,
	distanceResults []types.DistanceResult,
) (*types.CandidateBlock, error) {
	startTime := time.Now()

	if len(distanceResults) == 0 {
		return nil, types.ErrNoDistanceResults
	}

	s.logger.Info("开始选择最近距离区块")

	// 找到所有最小距离的候选
	tiedResults, err := s.distanceCalculator.FindTiedDistances(distanceResults)
	if err != nil {
		return nil, err
	}

	var selectedBlock *types.CandidateBlock
	tieBreakApplied := false

	if len(tiedResults) == 1 {
		// 只有一个最小距离候选，直接选择
		selectedBlock = tiedResults[0].Candidate
		s.logger.Info("找到唯一最小距离区块")
	} else {
		// 多个候选具有相同最小距离，应用tie-breaking
		selectedBlock = s.applyDeterministicTieBreaking(tiedResults)
		tieBreakApplied = true
		s.logger.Info("应用tie-breaking选择区块")
	}

	// 记录选择过程
	selectionTime := time.Since(startTime)
	s.recordSelection(selectionRecord{
		timestamp:       time.Now(),
		candidatesCount: len(distanceResults),
		selectedBlock:   selectedBlock,
		minDistance:     tiedResults[0].Distance.String(),
		tieBreakApplied: tieBreakApplied,
		selectionTime:   selectionTime,
	})

	s.logger.Info("区块选择完成")

	return selectedBlock, nil
}

// applyDeterministicTieBreaking 应用确定性tie-breaking
// 策略：按区块哈希字典序选择最小的
func (s *blockDistanceSelector) applyDeterministicTieBreaking(
	tiedResults []types.DistanceResult,
) *types.CandidateBlock {
	if len(tiedResults) <= 1 {
		if len(tiedResults) == 1 {
			return tiedResults[0].Candidate
		}
		return nil
	}

	// 按区块哈希字典序排序
	sort.Slice(tiedResults, func(i, j int) bool {
		return bytes.Compare(
			tiedResults[i].Candidate.BlockHash,
			tiedResults[j].Candidate.BlockHash,
		) < 0
	})

	// 选择字典序最小的区块哈希对应的候选
	selected := tiedResults[0].Candidate

	s.logger.Info("确定性tie-breaking完成")

	return selected
}

// ValidateSelection 验证选择结果的正确性
func (s *blockDistanceSelector) ValidateSelection(
	selected *types.CandidateBlock,
	allResults []types.DistanceResult,
) error {
	// 找到选中区块在结果中的位置
	var selectedResult *types.DistanceResult
	for _, result := range allResults {
		if bytes.Equal(result.Candidate.BlockHash, selected.BlockHash) {
			selectedResult = &result
			break
		}
	}

	if selectedResult == nil {
		return types.ErrSelectedBlockNotFound
	}

	// 验证是否为最小距离
	for _, result := range allResults {
		if s.distanceCalculator.CompareDistances(result.Distance, selectedResult.Distance) < 0 {
			return types.ErrInvalidSelection
		}
	}

	// 验证tie-breaking的正确性
	tiedResults, err := s.distanceCalculator.FindTiedDistances(allResults)
	if err != nil {
		return err
	}

	if len(tiedResults) > 1 {
		// 验证tie-breaking选择
		expectedSelected := s.applyDeterministicTieBreaking(tiedResults)
		if !bytes.Equal(expectedSelected.BlockHash, selected.BlockHash) {
			return types.ErrInvalidTieBreaking
		}
	}

	return nil
}

// GetSelectionHistory 获取选择历史记录
func (s *blockDistanceSelector) GetSelectionHistory(limit int) []selectionRecord {
	s.historyMutex.RLock()
	defer s.historyMutex.RUnlock()

	if limit <= 0 || limit > len(s.selectionHistory) {
		limit = len(s.selectionHistory)
	}

	// 返回最近的记录
	start := len(s.selectionHistory) - limit
	return append([]selectionRecord{}, s.selectionHistory[start:]...)
}

// recordSelection 记录选择过程
func (s *blockDistanceSelector) recordSelection(record selectionRecord) {
	s.historyMutex.Lock()
	defer s.historyMutex.Unlock()

	s.selectionHistory = append(s.selectionHistory, record)

	// 保持历史记录大小合理
	if len(s.selectionHistory) > 1000 {
		s.selectionHistory = s.selectionHistory[len(s.selectionHistory)-1000:]
	}
}
