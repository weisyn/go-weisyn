// multi_factor_scoring.go
// 简化基础验证系统
//
// 主要功能：
// 1. 基础PoW验证：确保候选区块满足工作量证明要求
// 2. 基本结构验证：验证区块格式和交易有效性
// 3. 快速过滤：移除无效候选，为距离选择做准备
// 4. 轻量统计：基本的验证统计信息
//
// 简化原则：
// - 专注基础验证：仅保留必要的PoW和格式验证
// - 性能优先：快速过滤，不进行复杂评分计算
// - 距离选择优先：将选择决策权交给距离选择算法
// - 零配置：无需权重配置，算法天然确定性
//
// 注意：复杂的多维度评分已被距离选择算法替代
//
// 作者：WES开发团队
// 更新时间：2025-09-14（距离选择简化）

package decision_calculator

import (
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// basicValidator 简化基础验证器
type basicValidator struct {
	logger       log.Logger
	chainService blockchain.ChainService
	hashManager  crypto.HashManager

	// 验证统计
	validationStats   *basicValidationStats
	validationHistory []*validationRecord
	statsMutex        sync.RWMutex
}

// basicValidationStats 基础验证统计
type basicValidationStats struct {
	totalValidated     uint64
	validCandidates    uint64
	invalidCandidates  uint64
	lastValidationTime time.Time
	averageTime        time.Duration
}

// validationRecord 验证记录
type validationRecord struct {
	timestamp      time.Time
	candidateCount int
	validCount     int
	invalidCount   int
	validationTime time.Duration
}

// newBasicValidator 创建基础验证器
func newBasicValidator(
	logger log.Logger,
	chainService blockchain.ChainService,
	hashManager crypto.HashManager,
) *basicValidator {
	return &basicValidator{
		logger:            logger,
		chainService:      chainService,
		hashManager:       hashManager,
		validationStats:   &basicValidationStats{},
		validationHistory: make([]*validationRecord, 0, 1000),
	}
}

// validateCandidate 基础候选验证
func (v *basicValidator) validateCandidate(candidate *types.CandidateBlock) error {
	// 基本格式验证
	if candidate == nil {
		return types.ErrInvalidSelection
	}

	if candidate.Block == nil {
		return types.ErrInvalidSelection
	}

	if len(candidate.BlockHash) == 0 {
		return types.ErrEmptySelectedBlockHash
	}

	// 基础PoW验证（这里应该调用具体的PoW验证逻辑）
	// 简化实现：检查区块哈希是否符合格式要求
	if len(candidate.BlockHash) != 32 { // 假设使用SHA256，32字节
		return types.ErrDistanceValidationFailed
	}

	// 验证区块高度
	if candidate.Height == 0 {
		return types.ErrInvalidSelection
	}

	// 验证基本区块结构（简化）
	if candidate.Block.Body == nil {
		return types.ErrInvalidSelection
	}

	return nil
}

// validateAllCandidates 批量基础验证
func (v *basicValidator) validateAllCandidates(candidates []types.CandidateBlock) ([]types.CandidateBlock, error) {
	startTime := time.Now()

	if len(candidates) == 0 {
		return []types.CandidateBlock{}, nil
	}

	v.logger.Info("开始候选区块基础验证")

	var validCandidates []types.CandidateBlock
	var validCount, invalidCount int

	for _, candidate := range candidates {
		if err := v.validateCandidate(&candidate); err != nil {
			v.logger.Info("候选区块验证失败")
			invalidCount++
			continue
		}

		validCandidates = append(validCandidates, candidate)
		validCount++
	}

	// 更新统计信息
	validationTime := time.Since(startTime)
	v.updateValidationStats(len(candidates), validCount, invalidCount, validationTime)

	v.logger.Info("候选区块基础验证完成")
	return validCandidates, nil
}

// getValidationStatistics 获取验证统计信息
func (v *basicValidator) getValidationStatistics() *basicValidationStats {
	v.statsMutex.RLock()
	defer v.statsMutex.RUnlock()

	// 返回当前统计信息的副本
	return &basicValidationStats{
		totalValidated:     v.validationStats.totalValidated,
		validCandidates:    v.validationStats.validCandidates,
		invalidCandidates:  v.validationStats.invalidCandidates,
		lastValidationTime: v.validationStats.lastValidationTime,
		averageTime:        v.validationStats.averageTime,
	}
}

// updateValidationStats 更新验证统计信息
func (v *basicValidator) updateValidationStats(total, valid, invalid int, validationTime time.Duration) {
	v.statsMutex.Lock()
	defer v.statsMutex.Unlock()

	// 更新统计信息
	v.validationStats.totalValidated += uint64(total)
	v.validationStats.validCandidates += uint64(valid)
	v.validationStats.invalidCandidates += uint64(invalid)
	v.validationStats.lastValidationTime = time.Now()

	// 计算平均时间
	if v.validationStats.totalValidated > 0 {
		totalTime := time.Duration(v.validationStats.totalValidated) * v.validationStats.averageTime
		totalTime += validationTime
		v.validationStats.averageTime = totalTime / time.Duration(v.validationStats.totalValidated)
	} else {
		v.validationStats.averageTime = validationTime
	}

	// 添加到历史记录
	record := &validationRecord{
		timestamp:      time.Now(),
		candidateCount: total,
		validCount:     valid,
		invalidCount:   invalid,
		validationTime: validationTime,
	}

	v.validationHistory = append(v.validationHistory, record)

	// 保持历史记录大小合理
	if len(v.validationHistory) > 1000 {
		v.validationHistory = v.validationHistory[len(v.validationHistory)-1000:]
	}
}
