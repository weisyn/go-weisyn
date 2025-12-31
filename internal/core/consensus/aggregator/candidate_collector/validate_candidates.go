// validate_candidates.go
// 候选验证、去重和质量过滤器
//
// 主要功能：
// 1. 实现候选区块的格式和有效性验证
// 2. 高效的重复检测机制
// 3. 候选质量预筛选
// 4. 基础PoW和时间戳验证
// 5. 父哈希一致性检查
//
// 验证层次：
// 1. 格式验证 - 区块结构和字段完整性
// 2. 基础PoW验证 - 工作量证明有效性
// 3. 时间戳验证 - 时间戳合理性检查
// 4. 父哈希验证 - 与当前链头的一致性
// 5. 重复检测 - 避免重复候选
//
// 质量预筛选：
// - 设置PoW质量最低阈值
// - 时间戳漂移范围检查
// - 交易数量和结构验证
// - 区块大小合理性检查
//
// 设计原则：
// - 快速的基础验证确保收集效率
// - 高效的重复检测避免资源浪费
// - 质量预筛选提升后续评分效率
// - 缓存机制优化验证性能
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package candidate_collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/config/consensus"
	chainsync "github.com/weisyn/v1/internal/core/chain/sync"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// candidateValidator 候选验证器
type candidateValidator struct {
	logger      log.Logger
	query       persistence.QueryService
	hashManager crypto.HashManager
	powEngine   crypto.POWEngine // 修复：添加POW验证引擎
	syncService chain.SystemSyncService
	// minBlockIntervalSeconds 从 blockchain.block.min_block_interval 读取（秒）
	minBlockIntervalSeconds uint64

	// 验证缓存和去重
	validationCache map[string]bool // 验证结果缓存
	duplicateCache  map[string]bool // 重复检测缓存
	cacheMutex      sync.RWMutex    // 缓存读写锁

	// 质量过滤参数
	minPoWQuality       float64       // 最小PoW质量
	maxTimestampDrift   time.Duration // 最大时间戳漂移
	minTransactionCount int           // 最小交易数量
	maxBlockSize        uint64        // 最大区块大小
}

// newCandidateValidator 创建候选验证器
func newCandidateValidator(
	logger log.Logger,
	query persistence.QueryService,
	hashManager crypto.HashManager,
	powEngine crypto.POWEngine,
	syncService chain.SystemSyncService,
	config *consensus.ConsensusOptions,
	minBlockIntervalSeconds uint64,
) *candidateValidator {
	// 从配置中获取聚合器参数，避免硬编码
	aggregatorConfig := config.Aggregator
	return &candidateValidator{
		logger:                  logger,
		query:                   query,
		hashManager:             hashManager,
		powEngine:               powEngine,
		syncService:             syncService,
		minBlockIntervalSeconds: minBlockIntervalSeconds,
		validationCache:         make(map[string]bool),
		duplicateCache:          make(map[string]bool),
		minPoWQuality:           aggregatorConfig.MinPoWQuality,
		maxTimestampDrift:       aggregatorConfig.MaxTimestampOffset,
		minTransactionCount:     int(aggregatorConfig.MinTransactionCount),
		maxBlockSize:            aggregatorConfig.MaxBlockSize,
	}
}

// validateCandidate 验证候选区块
func (v *candidateValidator) validateCandidate(candidate *types.CandidateBlock) error {
	// 基础结构验证
	if err := v.validateCandidateStructure(candidate); err != nil {
		return err
	}

	// 时间戳验证
	if err := v.validateTimestamp(candidate); err != nil {
		return err
	}

	// 父哈希验证
	if err := v.validateParentHash(candidate); err != nil {
		return err
	}

	// 质量预筛选
	if err := v.applyQualityFilter(candidate); err != nil {
		return err
	}

	return nil
}

// validateCandidateStructure 验证候选区块结构
func (v *candidateValidator) validateCandidateStructure(candidate *types.CandidateBlock) error {
	// 验证基础字段
	if candidate == nil {
		return errors.New("candidate is nil")
	}

	if candidate.Block == nil {
		return errors.New("candidate block is nil")
	}

	if candidate.Block.Header == nil {
		return errors.New("block header is nil")
	}

	if candidate.Block.Body == nil {
		return errors.New("block body is nil")
	}

	// 验证区块哈希
	if len(candidate.BlockHash) != 32 {
		return errors.New("invalid block hash length")
	}

	// 验证高度一致性
	if candidate.Height != candidate.Block.Header.Height {
		return errors.New("height mismatch between candidate and block")
	}

	// 验证Merkle根
	if len(candidate.Block.Header.MerkleRoot) == 0 {
		return errors.New("empty merkle root")
	}

	// 验证交易列表
	if candidate.Block.Body.Transactions == nil {
		return errors.New("transactions list is nil")
	}

	return nil
}

// validateTimestamp 验证时间戳
func (v *candidateValidator) validateTimestamp(candidate *types.CandidateBlock) error {
	blockTimestamp := time.Unix(int64(candidate.Block.Header.Timestamp), 0)
	now := time.Now()

	// 检查时间戳是否在合理范围内
	if blockTimestamp.After(now.Add(v.maxTimestampDrift)) {
		return errors.New("block timestamp too far in future")
	}

	if blockTimestamp.Before(now.Add(-v.maxTimestampDrift)) {
		return errors.New("block timestamp too old")
	}

	// 验证最小区块间隔（聚合器过滤过早候选）
	if err := v.validateMinBlockInterval(candidate); err != nil {
		return err
	}

	// 验证候选区块的生产时间和接收时间的一致性
	timeDiff := candidate.ReceivedAt.Sub(candidate.ProducedAt)
	if timeDiff < 0 {
		return errors.New("received time before produced time")
	}

	return nil
}

// validateMinBlockInterval 验证最小区块间隔（聚合器过滤）
//
// ⚠️ 重要说明：此验证基于区块的真实创建时间戳
// 聚合器通过固定收集窗口控制分发频率，而非调整时间戳
func (v *candidateValidator) validateMinBlockInterval(candidate *types.CandidateBlock) error {
	// 对于创世块（高度0），不检查间隔
	if candidate.Height == 0 {
		return nil
	}

	// 未配置最小间隔时不做过滤
	if v.minBlockIntervalSeconds == 0 {
		return nil
	}

	// 获取当前链信息
	if v.query == nil {
		return fmt.Errorf("QueryService 未注入（无法执行 min_block_interval 验证）")
	}
	chainInfo, err := v.query.GetChainInfo(context.Background())
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	// 如果没有父区块，跳过间隔检查
	if chainInfo.Height == 0 {
		return nil
	}

	// 只对“本地链尖的下一块候选”进行最小间隔过滤，避免对异常高度候选造成额外误杀。
	// 异常高度会在 validateParentHash/同步逻辑中处理。
	expectedHeight := chainInfo.Height + 1
	if candidate.Height != expectedHeight {
		return nil
	}

	// ✅ 彻底迭代：不允许 skip
	// - 对于 tip+1 的关键路径：拿不到父块时间戳视为本地状态异常，直接拒绝该候选。
	parentTS, err := v.query.GetBlockTimestamp(context.Background(), chainInfo.Height)
	if err != nil {
		if v.logger != nil {
			v.logger.Errorf("min_block_interval: failed to get parent timestamp (reject): parent_height=%d candidate_height=%d err=%v",
				chainInfo.Height, candidate.Height, err)
		}
		// 缺父块时间戳通常意味着“父块不可读/存储不一致”，必须优先补齐。
		if v.syncService != nil {
			ctx := chainsync.ContextWithUrgentSync(context.Background(), fmt.Sprintf("min_block_interval_missing_parent_ts:%d", chainInfo.Height))
			if candidate.Source != "" {
				ctx = chainsync.ContextWithPeerHint(ctx, candidate.Source)
			}
			_ = v.syncService.TriggerSync(ctx)
		}
		return fmt.Errorf("min_block_interval 无法获取父块时间戳（拒绝候选）: parent_height=%d: %w", chainInfo.Height, err)
	}

	candidateTS := int64(candidate.Block.Header.Timestamp)
	minAllowed := parentTS + int64(v.minBlockIntervalSeconds)
	if candidateTS < minAllowed {
		// ✅ 验收点：必须明确记录拒绝原因与关键数值（便于证明配置生效）
		if v.logger != nil {
			v.logger.Warnf("min_block_interval: reject candidate too early: parent_height=%d parent_ts=%d candidate_height=%d candidate_ts=%d min_interval=%ds min_allowed=%d",
				chainInfo.Height, parentTS, candidate.Height, candidateTS, v.minBlockIntervalSeconds, minAllowed)
		}
		return fmt.Errorf("候选区块过早（min_block_interval）: parent_ts=%d candidate_ts=%d min_interval=%ds",
			parentTS, candidateTS, v.minBlockIntervalSeconds)
	}

	// ✅ 验收点：给出“通过”记录（用 debug/info 级别避免日志过噪）
	if v.logger != nil {
		v.logger.Debugf("min_block_interval: pass: parent_height=%d parent_ts=%d candidate_height=%d candidate_ts=%d min_interval=%ds min_allowed=%d",
			chainInfo.Height, parentTS, candidate.Height, candidateTS, v.minBlockIntervalSeconds, minAllowed)
	}

	// 时间戳漂移保护（启发式防护，非共识裁决）：
	// - 这里使用本地墙钟（time.Now）做“明显异常”的拒绝，防止时间戳攻击/垃圾候选。
	// - 真正的共识时间约束（例如 MTP、MaxFutureDrift、MinBlockInterval 等）应以链规则/控制器为准。
	candidateTimestamp := time.Unix(int64(candidate.Block.Header.Timestamp), 0)
	now := time.Now()

	// 检查候选区块时间戳是否过于超前（防止时间戳攻击）
	if candidateTimestamp.After(now.Add(2 * time.Minute)) {
		return fmt.Errorf("候选区块时间戳过于超前: %v", candidateTimestamp)
	}

	// 检查候选区块时间戳是否过于陈旧
	if candidateTimestamp.Before(now.Add(-10 * time.Minute)) {
		return fmt.Errorf("候选区块时间戳过于陈旧: %v", candidateTimestamp)
	}

	return nil
}

// validateParentHash 验证父哈希
func (v *candidateValidator) validateParentHash(candidate *types.CandidateBlock) error {
	// 对于创世块（高度0），不检查父哈希
	if candidate.Height == 0 {
		return nil
	}

	// 获取当前链信息
	if v.query == nil {
		return fmt.Errorf("QueryService 未注入（无法执行 parent hash 验证）")
	}
	chainInfo, err := v.query.GetChainInfo(context.Background())
	if err != nil {
		return errors.New("failed to get chain info for parent validation")
	}

	// 验证父哈希字段长度
	if len(candidate.Block.Header.PreviousHash) != 32 {
		return errors.New("invalid parent hash length")
	}

	// 检查高度是否正确（应该是链头高度+1）
	expectedHeight := chainInfo.Height + 1
	if candidate.Height != expectedHeight {
		// 如果候选高度领先当前链，尝试触发同步补齐缺失区块
		if candidate.Height > expectedHeight && v.syncService != nil {
			ctx := chainsync.ContextWithUrgentSync(context.Background(), fmt.Sprintf("candidate_height_ahead:%d->%d", chainInfo.Height, candidate.Height))
			if candidate.Source != "" {
				ctx = chainsync.ContextWithPeerHint(ctx, candidate.Source)
			}
			missingStart := chainInfo.Height + 1
			missingEnd := candidate.Height - 1
			if v.logger != nil {
				v.logger.Warnf("候选高度领先本地链: current=%d, candidate=%d，触发同步补齐缺失区块 %d→%d",
					chainInfo.Height, candidate.Height, missingStart, missingEnd)
			}
			if err := v.syncService.TriggerSync(ctx); err != nil && v.logger != nil {
				v.logger.Warnf("触发同步失败: %v", err)
			}
		}
		return errors.New("invalid candidate height")
	}

	// ✅ 生产级硬门槛：进入聚合轮次前，父块必须“可读且可用”。
	// 背景：链信息里可能有 BestBlockHash，但 blocks/ 文件缺失或坏块会导致后续 getParentHash/评估阶段崩溃或卡死。
	if v.query != nil && chainInfo.Height > 0 {
		parentBlock, perr := v.query.GetBlockByHeight(context.Background(), chainInfo.Height)
		if perr != nil || parentBlock == nil || parentBlock.Header == nil {
			if v.logger != nil {
				v.logger.Warnf("missing parent block data: parent_height=%d candidate_height=%d err=%v (trigger urgent sync)",
					chainInfo.Height, candidate.Height, perr)
			}
			if v.syncService != nil {
				ctx := chainsync.ContextWithUrgentSync(context.Background(), fmt.Sprintf("missing_parent_block_data:%d", chainInfo.Height))
				if candidate.Source != "" {
					ctx = chainsync.ContextWithPeerHint(ctx, candidate.Source)
				}
				_ = v.syncService.TriggerSync(ctx)
			}
			if perr == nil {
				perr = fmt.Errorf("parent block is nil")
			}
			return fmt.Errorf("missing parent block data at height %d: %w", chainInfo.Height, perr)
		}
	}

	// 高度匹配时，进一步验证父哈希是否与本地链尖哈希一致
	if len(chainInfo.BestBlockHash) != 32 {
		// 如果本地 BestBlockHash 异常，直接返回错误，避免在不可信状态下继续挖矿
		return fmt.Errorf("local best block hash is invalid: len=%d", len(chainInfo.BestBlockHash))
	}

	if !bytes.Equal(candidate.Block.Header.PreviousHash, chainInfo.BestBlockHash) {
		// 父哈希不匹配，说明候选区块并非基于本地最佳链尖，可能存在分叉或不同视图
		if v.logger != nil {
			v.logger.Warnf("候选父哈希与本地链尖不一致: expected=%x, got=%x, height=%d",
				chainInfo.BestBlockHash[:8], candidate.Block.Header.PreviousHash[:8], candidate.Height)
		}

		// 可选：尝试触发一次同步，以获取最新链尖视图
		if v.syncService != nil {
			ctx := chainsync.ContextWithUrgentSync(context.Background(), "candidate_parent_hash_mismatch")
			if candidate.Source != "" {
				ctx = chainsync.ContextWithPeerHint(ctx, candidate.Source)
			}
			if err := v.syncService.TriggerSync(ctx); err != nil && v.logger != nil {
				v.logger.Warnf("父哈希不匹配时触发同步失败: %v", err)
			}
		}

		return errors.New("parent hash does not match local best block hash")
	}

	return nil
}

// applyQualityFilter 应用质量过滤器
func (v *candidateValidator) applyQualityFilter(candidate *types.CandidateBlock) error {
	// 检查交易数量
	txCount := len(candidate.Block.Body.Transactions)
	if txCount < v.minTransactionCount {
		return errors.New("insufficient transaction count")
	}

	// 修复：计算区块真实大小并检查
	actualSize := v.calculateBlockSize(candidate)
	if actualSize > v.maxBlockSize {
		return fmt.Errorf("区块大小超出限制: %d > %d 字节", actualSize, v.maxBlockSize)
	}

	// 检查PoW质量（基于难度和Nonce）
	if err := v.validatePoWQuality(candidate); err != nil {
		return err
	}

	return nil
}

// validatePoWQuality 验证PoW质量（修复：使用真实POW验证）
func (v *candidateValidator) validatePoWQuality(candidate *types.CandidateBlock) error {
	// 获取区块头的基础信息
	header := candidate.Block.Header

	// 基础字段检查
	if len(header.Nonce) == 0 {
		return errors.New("missing PoW nonce")
	}

	if header.Difficulty == 0 {
		return errors.New("zero difficulty")
	}

	// 修复：使用真实的POW验证逻辑
	// 使用POWEngine验证区块头的POW
	isValid, err := v.powEngine.VerifyBlockHeader(header)
	if err != nil {
		return fmt.Errorf("POW验证失败: %v", err)
	}

	if !isValid {
		return errors.New("POW验证不通过：哈希值未满足难度要求")
	}

	return nil
}

// calculateBlockSize 计算区块真实大小（修复：使用protobuf真实大小）
func (v *candidateValidator) calculateBlockSize(candidate *types.CandidateBlock) uint64 {
	// 修复：使用protobuf的真实序列化大小
	if candidate.Block == nil {
		return 0
	}

	// 计算protobuf序列化后的真实大小
	serializedSize := proto.Size(candidate.Block)
	return uint64(serializedSize)
}

// ❌ **已删除未使用的方法** - 基于错误架构假设
//
// 🚨 **删除原因**：
// 1. checkDuplicate() - 重复检测已在收集管理器中实现，无需验证器重复
// 2. markProcessed() - 在旧的多因子聚合架构中选择完成后直接清空内存池，无需标记处理状态
// 3. clearCache() - 缓存清理无意义，内存池清空后所有状态重置
//
// 🎯 **正确的聚合流程（当前距离选择架构）**：
// 选择完成 → 分发结果 → 清空整个内存池 → 开始下一轮
// 而不是：标记已处理 → 维护复杂状态 → 选择性清理
//
// func (v *candidateValidator) checkDuplicate(candidate *types.CandidateBlock) bool { ... }
// func (v *candidateValidator) markProcessed(candidate *types.CandidateBlock) { ... }
// func (v *candidateValidator) clearCache() { ... }
