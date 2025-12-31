// Package health 提供链健康检查功能
//
// 🎯 **核心职责**：
// - 启动时快速检查链尖和最近区块
// - 后台深度扫描全链健康状态
// - 自动触发修复流程
//
// 📋 **检查项**：
// - Tip一致性：state:chain:tip与实际区块hash
// - 索引完整性：height↔hash映射、TX索引
// - 区块时间戳连续性
// - UTXO-Block一致性
package health

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// ============================================================================
//                              数据结构
// ============================================================================

// ChainHealthChecker 链健康检查器
type ChainHealthChecker struct {
	queryService persistence.QueryService
	blockQuery   persistence.BlockQuery
	blockHasher  core.BlockHashServiceClient
	store        storage.BadgerStore
	fileStore    storage.FileStore
	recoveryMgr  RecoveryManagerInterface
	logger       logiface.Logger
	config       HealthCheckConfig
}

// RecoveryManagerInterface 恢复管理器接口
type RecoveryManagerInterface interface {
	RepairWithStrategy(ctx context.Context, issue CorruptionIssue) error
	GetRepairHistory() []RepairRecord
}

// CorruptionIssue 损坏问题（与recovery包保持一致）
type CorruptionIssue struct {
	Type        string
	Severity    string
	Height      *uint64
	Description string
	RawError    error
}

// RepairRecord 修复记录（与recovery包保持一致）
type RepairRecord struct {
	Timestamp   time.Time
	IssueType   string
	Severity    string
	Height      *uint64
	RepairLevel string
	Result      string
	Duration    time.Duration
	Error       string
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	QuickCheckEnabled      bool // 启用快速检查
	QuickCheckRecentBlocks int  // 快速检查最近N个区块，默认10
	DeepScanEnabled        bool // 启用深度扫描
	DeepScanAsync          bool // 后台异步深度扫描
	AutoRepair             bool // 发现问题自动修复
}

// HealthReport 健康检查报告
type HealthReport struct {
	StartTime time.Time
	EndTime   time.Time
	CheckType string // "quick" | "deep"

	// 检查结果
	TipConsistent       bool
	RecentBlocksHealthy bool
	IndexIntegrity      bool
	BlockTimestampValid bool
	UTXOConsistent      bool

	// 问题详情
	Issues             []HealthIssue
	AutoRepairedIssues []HealthIssue
	UnrepairableIssues []HealthIssue
}

// HealthIssue 健康问题
type HealthIssue struct {
	Type        string // "tip_inconsistent", "timestamp_regression", etc.
	Severity    string // "critical", "high", "medium", "low"
	Height      *uint64
	Description string
	Repairable  bool
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewChainHealthChecker 创建链健康检查器
func NewChainHealthChecker(
	queryService persistence.QueryService,
	blockQuery persistence.BlockQuery,
	blockHasher core.BlockHashServiceClient,
	store storage.BadgerStore,
	fileStore storage.FileStore,
	recoveryMgr RecoveryManagerInterface,
	logger logiface.Logger,
	config HealthCheckConfig,
) *ChainHealthChecker {
	// 设置默认值
	if config.QuickCheckRecentBlocks == 0 {
		config.QuickCheckRecentBlocks = 10
	}

	return &ChainHealthChecker{
		queryService: queryService,
		blockQuery:   blockQuery,
		blockHasher:  blockHasher,
		store:        store,
		fileStore:    fileStore,
		recoveryMgr:  recoveryMgr,
		logger:       logger,
		config:       config,
	}
}

// ============================================================================
//                              快速检查
// ============================================================================

// QuickCheck 快速健康检查（~1秒）
//
// 🎯 **检查项**：
// 1. 链尖一致性
// 2. 最近N个区块的索引
// 3. 最近N个区块的时间戳
//
// 参数：
//   - ctx: 操作上下文
//
// 返回：
//   - *HealthReport: 健康检查报告
//   - error: 检查失败的错误
func (c *ChainHealthChecker) QuickCheck(ctx context.Context) (*HealthReport, error) {
	startTime := time.Now()
	report := &HealthReport{
		StartTime:           startTime,
		CheckType:           "quick",
		TipConsistent:       true,
		RecentBlocksHealthy: true,
		IndexIntegrity:      true,
		BlockTimestampValid: true,
		UTXOConsistent:      true,
		Issues:              make([]HealthIssue, 0),
		AutoRepairedIssues:  make([]HealthIssue, 0),
		UnrepairableIssues:  make([]HealthIssue, 0),
	}

	if c.logger != nil {
		c.logger.Info("🔍 开始快速健康检查...")
	}

	// 1. 检查链尖一致性
	c.checkTipConsistency(ctx, report)

	// 2. 检查最近N个区块的索引
	c.checkRecentBlocksIndex(ctx, report, c.config.QuickCheckRecentBlocks)

	// 3. 检查最近N个区块的时间戳
	c.checkRecentBlocksTimestamp(ctx, report, c.config.QuickCheckRecentBlocks)

	// 4. 触发自动修复
	if c.config.AutoRepair && len(report.Issues) > 0 {
		c.autoRepair(ctx, report)
	}

	report.EndTime = time.Now()

	if c.logger != nil {
		c.logger.Infof("✅ 快速检查完成: 发现问题=%d 已修复=%d 无法修复=%d 耗时=%v",
			len(report.Issues), len(report.AutoRepairedIssues), len(report.UnrepairableIssues),
			report.EndTime.Sub(report.StartTime))
	}

	return report, nil
}

// ============================================================================
//                              深度扫描
// ============================================================================

// DeepScan 深度健康扫描（可能数分钟）
//
// 🎯 **检查项**：
// 1. 全量索引完整性
// 2. 全量区块时间戳
// 3. UTXO-Block一致性
// 4. 交易索引完整性
//
// 参数：
//   - ctx: 操作上下文
//
// 返回：
//   - *HealthReport: 健康检查报告
//   - error: 检查失败的错误
func (c *ChainHealthChecker) DeepScan(ctx context.Context) (*HealthReport, error) {
	startTime := time.Now()
	report := &HealthReport{
		StartTime:           startTime,
		CheckType:           "deep",
		TipConsistent:       true,
		RecentBlocksHealthy: true,
		IndexIntegrity:      true,
		BlockTimestampValid: true,
		UTXOConsistent:      true,
		Issues:              make([]HealthIssue, 0),
		AutoRepairedIssues:  make([]HealthIssue, 0),
		UnrepairableIssues:  make([]HealthIssue, 0),
	}

	if c.logger != nil {
		c.logger.Info("🔍 开始深度健康扫描...")
	}

	// 1. 检查链尖一致性
	c.checkTipConsistency(ctx, report)

	// 2. 全量索引完整性验证
	c.verifyFullIndexIntegrity(ctx, report)

	// 3. 全量区块时间戳扫描
	c.verifyAllBlocksTimestamp(ctx, report)

	// 4. UTXO-Block一致性（简化版）
	c.verifyUTXOBlockConsistency(ctx, report)

	// 5. 交易索引完整性
	c.verifyTxIndexIntegrity(ctx, report)

	// 6. 触发自动修复
	if c.config.AutoRepair && len(report.Issues) > 0 {
		c.autoRepair(ctx, report)
	}

	report.EndTime = time.Now()

	if c.logger != nil {
		c.logger.Infof("✅ 深度扫描完成: 发现问题=%d 已修复=%d 无法修复=%d 耗时=%v",
			len(report.Issues), len(report.AutoRepairedIssues), len(report.UnrepairableIssues),
			report.EndTime.Sub(report.StartTime))
	}

	return report, nil
}

// ============================================================================
//                              检查逻辑：Tip一致性
// ============================================================================

// checkTipConsistency 检查链尖一致性
//
// 🎯 **检查逻辑**：
// 1. 读取 state:chain:tip
// 2. 读取实际区块并计算hash
// 3. 比较stored hash vs actual hash
func (c *ChainHealthChecker) checkTipConsistency(ctx context.Context, report *HealthReport) {
	// 1. 读取state:chain:tip
	tipData, err := c.store.Get(ctx, []byte("state:chain:tip"))
	if err != nil {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "tip_read_failed",
			Severity:    "critical",
			Description: fmt.Sprintf("读取链尖失败: %v", err),
			Repairable:  false,
		})
		return
	}

	if len(tipData) < 40 {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "tip_invalid_format",
			Severity:    "critical",
			Description: fmt.Sprintf("链尖数据格式错误: len=%d", len(tipData)),
			Repairable:  true,
		})
		return
	}

	storedHeight := binary.BigEndian.Uint64(tipData[:8])
	storedHash := tipData[8:40]

	// 2. 读取实际区块并计算hash
	block, err := c.blockQuery.GetBlockByHeight(ctx, storedHeight)
	if err != nil {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "block_read_failed",
			Severity:    "critical",
			Height:      &storedHeight,
			Description: fmt.Sprintf("读取区块失败: %v", err),
			Repairable:  false,
		})
		return
	}

	if block == nil || block.Header == nil {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "block_nil",
			Severity:    "critical",
			Height:      &storedHeight,
			Description: "区块数据为空",
			Repairable:  false,
		})
		return
	}

	// 计算实际hash
	if c.blockHasher == nil {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "hash_compute_failed",
			Severity:    "high",
			Height:      &storedHeight,
			Description: "blockHasher 未注入，无法计算区块hash",
			Repairable:  false,
		})
		return
	}
	resp, err := c.blockHasher.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: block})
	if err != nil || resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "hash_compute_failed",
			Severity:    "high",
			Height:      &storedHeight,
			Description: fmt.Sprintf("计算区块hash失败: %v", err),
			Repairable:  false,
		})
		return
	}

	actualHash := resp.Hash

	// 3. 比较
	if !bytes.Equal(storedHash, actualHash) {
		report.TipConsistent = false
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "tip_inconsistent",
			Severity:    "critical",
			Height:      &storedHeight,
			Description: fmt.Sprintf("Tip hash不一致: stored=%x actual=%x", storedHash[:6], actualHash[:6]),
			Repairable:  true,
		})

		if c.logger != nil {
			c.logger.Warnf("⚠️ Tip不一致: height=%d stored=%x actual=%x",
				storedHeight, storedHash[:6], actualHash[:6])
		}
	}
}

// ============================================================================
//                              检查逻辑：最近区块索引
// ============================================================================

// checkRecentBlocksIndex 检查最近N个区块的索引
func (c *ChainHealthChecker) checkRecentBlocksIndex(ctx context.Context, report *HealthReport, recentN int) {
	chainInfo, err := c.queryService.GetChainInfo(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Warnf("获取链信息失败: %v", err)
		}
		return
	}

	currentHeight := chainInfo.Height
	fromHeight := uint64(0)
	if currentHeight > uint64(recentN) {
		fromHeight = currentHeight - uint64(recentN)
	}

	for height := fromHeight; height <= currentHeight; height++ {
		// 检查 indices:height:{height}
		heightKey := []byte(fmt.Sprintf("indices:height:%d", height))
		val, err := c.store.Get(ctx, heightKey)
		if err != nil || len(val) == 0 {
			report.IndexIntegrity = false
			report.Issues = append(report.Issues, HealthIssue{
				Type:        "index_corrupt_height_index",
				Severity:    "high",
				Height:      &height,
				Description: fmt.Sprintf("高度索引缺失: %v", err),
				Repairable:  true,
			})
		}
	}
}

// ============================================================================
//                              检查逻辑：最近区块时间戳
// ============================================================================

// checkRecentBlocksTimestamp 检查最近N个区块的时间戳
func (c *ChainHealthChecker) checkRecentBlocksTimestamp(ctx context.Context, report *HealthReport, recentN int) {
	chainInfo, err := c.queryService.GetChainInfo(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Warnf("获取链信息失败: %v", err)
		}
		return
	}

	currentHeight := chainInfo.Height
	fromHeight := uint64(1) // 从1开始，需要检查父区块
	if currentHeight > uint64(recentN) {
		fromHeight = currentHeight - uint64(recentN)
	}

	for height := fromHeight; height <= currentHeight; height++ {
		// 读取父区块
		parentBlock, err := c.blockQuery.GetBlockByHeight(ctx, height-1)
		if err != nil {
			continue
		}

		// 读取子区块
		childBlock, err := c.blockQuery.GetBlockByHeight(ctx, height)
		if err != nil {
			continue
		}

		if parentBlock == nil || parentBlock.Header == nil || childBlock == nil || childBlock.Header == nil {
			continue
		}

		// 检查时间戳
		if childBlock.Header.Timestamp < parentBlock.Header.Timestamp {
			report.BlockTimestampValid = false
			report.Issues = append(report.Issues, HealthIssue{
				Type:     "timestamp_regression",
				Severity: "high",
				Height:   &height,
				Description: fmt.Sprintf("时间戳倒退: parent=%d child=%d",
					parentBlock.Header.Timestamp, childBlock.Header.Timestamp),
				Repairable: true,
			})

			if c.logger != nil {
				c.logger.Warnf("⚠️ 时间戳倒退: height=%d parent=%d child=%d",
					height, parentBlock.Header.Timestamp, childBlock.Header.Timestamp)
			}
		}
	}
}

// ============================================================================
//                              深度扫描：全量索引完整性
// ============================================================================

// verifyFullIndexIntegrity 全量索引完整性验证
func (c *ChainHealthChecker) verifyFullIndexIntegrity(ctx context.Context, report *HealthReport) {
	chainInfo, err := c.queryService.GetChainInfo(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Warnf("获取链信息失败: %v", err)
		}
		return
	}

	maxHeight := chainInfo.Height

	if c.logger != nil {
		c.logger.Infof("验证全量索引完整性: [0..%d]", maxHeight)
	}

	// 检查所有区块的索引
	c.checkRecentBlocksIndex(ctx, report, int(maxHeight)+1)
}

// ============================================================================
//                              深度扫描：全量时间戳
// ============================================================================

// verifyAllBlocksTimestamp 全量区块时间戳验证
func (c *ChainHealthChecker) verifyAllBlocksTimestamp(ctx context.Context, report *HealthReport) {
	chainInfo, err := c.queryService.GetChainInfo(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Warnf("获取链信息失败: %v", err)
		}
		return
	}

	maxHeight := chainInfo.Height

	if c.logger != nil {
		c.logger.Infof("验证全量时间戳: [0..%d]", maxHeight)
	}

	// 检查所有区块的时间戳
	c.checkRecentBlocksTimestamp(ctx, report, int(maxHeight)+1)
}

// ============================================================================
//                              深度扫描：UTXO-Block一致性
// ============================================================================

// verifyUTXOBlockConsistency UTXO-Block一致性验证（简化版）
func (c *ChainHealthChecker) verifyUTXOBlockConsistency(ctx context.Context, report *HealthReport) {
	// TODO: 实现UTXO-Block一致性检查
	// 这需要扫描UTXO集，验证每个UTXO的BlockHeight是否存在于链上
	if c.logger != nil {
		c.logger.Debug("UTXO-Block一致性检查已跳过（待实现）")
	}
}

// ============================================================================
//                              深度扫描：TX索引完整性
// ============================================================================

// verifyTxIndexIntegrity 交易索引完整性验证（简化版）
func (c *ChainHealthChecker) verifyTxIndexIntegrity(ctx context.Context, report *HealthReport) {
	// TODO: 实现TX索引完整性检查
	// 这需要扫描所有区块的交易，验证indices:tx索引是否存在
	if c.logger != nil {
		c.logger.Debug("TX索引完整性检查已跳过（待实现）")
	}
}

// ============================================================================
//                              自动修复
// ============================================================================

// autoRepair 自动修复检测到的问题
func (c *ChainHealthChecker) autoRepair(ctx context.Context, report *HealthReport) {
	if c.recoveryMgr == nil {
		if c.logger != nil {
			c.logger.Warn("恢复管理器未初始化，无法自动修复")
		}
		return
	}

	if c.logger != nil {
		c.logger.Infof("🔧 开始自动修复: 共 %d 个问题", len(report.Issues))
	}

	for _, issue := range report.Issues {
		if !issue.Repairable {
			report.UnrepairableIssues = append(report.UnrepairableIssues, issue)
			continue
		}

		// 转换为CorruptionIssue
		corruptIssue := CorruptionIssue{
			Type:        issue.Type,
			Severity:    issue.Severity,
			Height:      issue.Height,
			Description: issue.Description,
		}

		// 触发修复
		if err := c.recoveryMgr.RepairWithStrategy(ctx, corruptIssue); err != nil {
			if c.logger != nil {
				c.logger.Errorf("修复失败: type=%s err=%v", issue.Type, err)
			}
			report.UnrepairableIssues = append(report.UnrepairableIssues, issue)
		} else {
			if c.logger != nil {
				c.logger.Infof("✅ 修复成功: type=%s", issue.Type)
			}
			report.AutoRepairedIssues = append(report.AutoRepairedIssues, issue)
		}
	}

	if c.logger != nil {
		c.logger.Infof("自动修复完成: 成功=%d 失败=%d",
			len(report.AutoRepairedIssues), len(report.UnrepairableIssues))
	}
}

// ============================================================================
//                              辅助方法
// ============================================================================

// GetConfig 获取配置
func (c *ChainHealthChecker) GetConfig() HealthCheckConfig {
	return c.config
}
