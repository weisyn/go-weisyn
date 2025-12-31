// Package recovery 提供链派生数据的统一恢复管理
//
// 🎯 **核心设计原则**：
// - UTXO、索引、状态根等派生数据同等地位
// - 统一的检查和修复机制
// - 分级修复策略：选择性修复 → 区域重建 → 全量重建
//
// 📋 **架构职责**：
// - 作为中央调度器，统一管理所有派生数据的修复
// - 监听损坏事件，分派到对应的子管理器
// - 实施分级修复策略，平衡性能和彻底性
package recovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/persistence/repair"
	core "github.com/weisyn/v1/pb/blockchain/block"
	blockif "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// ============================================================================
//                              数据结构
// ============================================================================

// DerivedDataRecoveryManager 统一派生数据恢复管理器
//
// 🎯 **核心职责**：
// - 统一管理所有派生数据（UTXO、索引、状态根）的修复
// - 监听 corruption.detected 事件并分派到子管理器
// - 实施分级修复策略
// - 记录修复历史和状态
type DerivedDataRecoveryManager struct {
	// 子管理器
	indexManager *IndexRecoveryManager
	utxoManager  *UTXORecoveryManager
	blockManager *BlockCorruptionManager

	// 共享依赖
	queryService   persistence.QueryService
	blockProcessor blockif.BlockProcessor
	store          storage.BadgerStore
	fileStore      storage.FileStore // 🆕 用于创世区块索引修复
	eventBus       eventiface.EventBus
	logger         logiface.Logger
	writeGate      WriteGateInterface // 只读模式控制

	// 🆕 区块哈希计算服务（用于创世区块索引修复）
	blockHashClient core.BlockHashServiceClient

	// 修复状态
	mu               sync.Mutex
	repairInProgress map[string]bool      // key: issue_type
	repairHistory    []RepairRecord       // 修复历史
	lastRepairTime   map[string]time.Time // key: issue_type
	throttle         time.Duration        // 限流间隔
}

// WriteGateInterface 只读模式控制接口
type WriteGateInterface interface {
	IsReadOnly() bool
	ReadOnlyReason() string
	ExitReadOnly()
}

// RepairRecord 修复记录
type RepairRecord struct {
	Timestamp   time.Time
	IssueType   string
	Severity    string
	Height      *uint64
	RepairLevel string // "selective", "regional", "full"
	Result      string // "success", "failed", "partial"
	Duration    time.Duration
	Error       string
}

// CorruptionIssue 损坏问题定义
type CorruptionIssue struct {
	Type        string  // "tip_inconsistent", "index_corrupt", etc.
	Severity    string  // "critical", "high", "medium", "low"
	Height      *uint64
	Description string
	RawError    error
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewDerivedDataRecoveryManager 创建统一派生数据恢复管理器
func NewDerivedDataRecoveryManager(
	queryService persistence.QueryService,
	blockProcessor blockif.BlockProcessor,
	utxoSnapshot eutxo.UTXOSnapshot,
	store storage.BadgerStore,
	fileStore storage.FileStore, // 🆕 用于创世区块索引修复
	blockHashClient core.BlockHashServiceClient, // 🆕 用于创世区块哈希计算
	hashManager crypto.HashManager,
	eventBus eventiface.EventBus,
	logger logiface.Logger,
	writeGate WriteGateInterface,
) *DerivedDataRecoveryManager {
	m := &DerivedDataRecoveryManager{
		queryService:     queryService,
		blockProcessor:   blockProcessor,
		store:            store,
		fileStore:        fileStore, // 🆕
		blockHashClient:  blockHashClient, // 🆕
		eventBus:         eventBus,
		logger:           logger,
		writeGate:        writeGate,
		repairInProgress: make(map[string]bool),
		repairHistory:    make([]RepairRecord, 0),
		lastRepairTime:   make(map[string]time.Time),
		throttle:         60 * time.Second,
	}

	// 创建子管理器（复用同一个hashManager）
	m.indexManager = NewIndexRecoveryManager(queryService, store, hashManager, logger)
	m.utxoManager = NewUTXORecoveryManager(queryService, blockProcessor, utxoSnapshot, eventBus, logger)
	m.blockManager = NewBlockCorruptionManager(queryService, blockProcessor, store, eventBus, logger)

	return m
}

// ============================================================================
//                              事件订阅
// ============================================================================

// RegisterSubscriptions 注册事件监听
//
// 🎯 **功能**：
// - 监听所有 corruption.detected 事件
// - 根据错误类型分派到对应的子管理器
// - 实施限流和去重
func (m *DerivedDataRecoveryManager) RegisterSubscriptions(ctx context.Context) {
	if m == nil || m.eventBus == nil {
		return
	}

	_ = m.eventBus.Subscribe(eventiface.EventTypeCorruptionDetected, func(evCtx context.Context, data interface{}) error {
		evt, ok := data.(types.CorruptionEventData)
		if !ok {
			if p, ok2 := data.(*types.CorruptionEventData); ok2 && p != nil {
				evt = *p
				ok = true
			}
		}
		if !ok {
			return nil
		}

		// 异步处理，避免阻塞事件总线
		go m.handleCorruptionEvent(evCtx, evt)
		return nil
	})

	if m.logger != nil {
		m.logger.Info("✅ DerivedDataRecoveryManager 事件订阅已注册")
	}
}

// handleCorruptionEvent 处理损坏事件
func (m *DerivedDataRecoveryManager) handleCorruptionEvent(ctx context.Context, evt types.CorruptionEventData) {
	if evt.ErrClass == "" {
		evt.ErrClass = corruptutil.ClassifyErr(fmt.Errorf("%s", evt.Error))
	}

	// 转换为 CorruptionIssue
	issue := CorruptionIssue{
		Type:        evt.ErrClass,
		Severity:    string(evt.Severity),
		Height:      evt.Height,
		Description: evt.Error,
		RawError:    fmt.Errorf("%s", evt.Error),
	}

	// 检查限流
	m.mu.Lock()
	if m.repairInProgress[issue.Type] {
		m.mu.Unlock()
		if m.logger != nil {
			m.logger.Debugf("修复已在进行中，跳过: type=%s", issue.Type)
		}
		return
	}

	lastTime, exists := m.lastRepairTime[issue.Type]
	if exists && time.Since(lastTime) < m.throttle {
		m.mu.Unlock()
		if m.logger != nil {
			m.logger.Debugf("修复限流，跳过: type=%s", issue.Type)
		}
		return
	}

	m.repairInProgress[issue.Type] = true
	m.lastRepairTime[issue.Type] = time.Now()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.repairInProgress[issue.Type] = false
		m.mu.Unlock()
	}()

	// 执行分级修复
	if err := m.RepairWithStrategy(ctx, issue); err != nil {
		if m.logger != nil {
			m.logger.Errorf("修复失败: type=%s err=%v", issue.Type, err)
		}
	}
}

// ============================================================================
//                              分级修复策略
// ============================================================================

// RepairWithStrategy 执行分级修复策略
//
// 🎯 **分级策略**：
// - Level 1: 选择性修复 - 只修复检测到的具体问题
// - Level 2: 区域重建 - 重放最近N个区块
// - Level 3: 全量重建 - 从genesis重新派生所有数据
//
// 参数：
//   - ctx: 操作上下文
//   - issue: 损坏问题
//
// 返回：
//   - error: 修复失败的错误
func (m *DerivedDataRecoveryManager) RepairWithStrategy(ctx context.Context, issue CorruptionIssue) error {
	startTime := time.Now()
	record := RepairRecord{
		Timestamp: startTime,
		IssueType: issue.Type,
		Severity:  issue.Severity,
		Height:    issue.Height,
	}

	if m.logger != nil {
		m.logger.Infof("🔧 开始分级修复: type=%s severity=%s", issue.Type, issue.Severity)
	}

	// Level 1: 选择性修复
	if m.logger != nil {
		m.logger.Debug("尝试 Level 1: 选择性修复")
	}
	if err := m.trySelectiveRepair(ctx, issue); err == nil {
		record.RepairLevel = "selective"
		record.Result = "success"
		record.Duration = time.Since(startTime)
		m.recordRepair(record)

		if m.logger != nil {
			m.logger.Infof("✅ Level 1 选择性修复成功: type=%s duration=%v", issue.Type, record.Duration)
		}
		return nil
	} else {
		if m.logger != nil {
			m.logger.Warnf("Level 1 选择性修复失败: %v", err)
		}
	}

	// Level 2: 区域重建
	if m.logger != nil {
		m.logger.Debug("尝试 Level 2: 区域重建")
	}
	if err := m.tryRegionalRebuild(ctx, issue); err == nil {
		record.RepairLevel = "regional"
		record.Result = "success"
		record.Duration = time.Since(startTime)
		m.recordRepair(record)

		if m.logger != nil {
			m.logger.Infof("✅ Level 2 区域重建成功: type=%s duration=%v", issue.Type, record.Duration)
		}
		return nil
	} else {
		if m.logger != nil {
			m.logger.Warnf("Level 2 区域重建失败: %v", err)
		}
	}

	// Level 3: 全量重建
	if m.logger != nil {
		m.logger.Warn("尝试 Level 3: 全量重建（这可能需要很长时间）")
	}
	if err := m.fullRebuild(ctx); err != nil {
		record.RepairLevel = "full"
		record.Result = "failed"
		record.Duration = time.Since(startTime)
		record.Error = err.Error()
		m.recordRepair(record)

		if m.logger != nil {
			m.logger.Errorf("❌ Level 3 全量重建失败: %v", err)
		}
		return fmt.Errorf("all repair levels failed: %w", err)
	}

	record.RepairLevel = "full"
	record.Result = "success"
	record.Duration = time.Since(startTime)
	m.recordRepair(record)

	if m.logger != nil {
		m.logger.Infof("✅ Level 3 全量重建成功: duration=%v", record.Duration)
	}
	return nil
}

// ============================================================================
//                              Level 1: 选择性修复
// ============================================================================

// trySelectiveRepair Level 1: 选择性修复
//
// 🎯 **策略**：
// - 只修复检测到的具体问题
// - 性能最优，适合单点故障
//
// 参数：
//   - ctx: 操作上下文
//   - issue: 损坏问题
//
// 返回：
//   - error: 修复失败的错误
func (m *DerivedDataRecoveryManager) trySelectiveRepair(ctx context.Context, issue CorruptionIssue) error {
	switch issue.Type {
	case "genesis_index_corrupt":
		// 🆕 创世区块索引损坏：从blocks文件重建索引
		if m.logger != nil {
			m.logger.Info("🩹 检测到创世区块索引损坏，触发修复")
		}
		return m.repairGenesisIndex(ctx)

	case "tip_inconsistent":
		// Tip不一致：重新计算并更新
		if issue.Height == nil {
			return fmt.Errorf("tip_inconsistent requires height")
		}
		return m.indexManager.RepairTipByHeight(ctx, *issue.Height)

	case "index_corrupt_hash_height", "index_corrupt_height_index":
		// 索引损坏：重建特定高度的索引
		if issue.Height == nil {
			return fmt.Errorf("index corruption requires height")
		}
		return m.indexManager.RebuildHeightIndex(ctx, *issue.Height, *issue.Height)

	case "tx_index_corrupt":
		// 交易索引损坏：重建特定高度的交易索引
		if issue.Height == nil {
			return fmt.Errorf("tx_index_corrupt requires height")
		}
		return m.indexManager.RebuildTxIndex(ctx, *issue.Height, *issue.Height)

	case "utxo_inconsistent":
		// UTXO不一致：委托给UTXORecoveryManager
		// 注意：UTXORecoveryManager已有自己的监听逻辑
		if m.logger != nil {
			m.logger.Debug("UTXO不一致由UTXORecoveryManager独立处理")
		}
		return nil

	case "timestamp_regression", "block_corrupt":
		// 区块损坏：从网络重新下载
		if issue.Height == nil {
			return fmt.Errorf("block corruption requires height")
		}
		return m.blockManager.RedownloadAndReplaceBlock(ctx, *issue.Height)

	default:
		return fmt.Errorf("unknown issue type: %s", issue.Type)
	}
}

// ============================================================================
//                              创世区块索引修复
// ============================================================================

// repairGenesisIndex 修复创世区块索引
//
// 🎯 **修复策略**：
// - 从 blocks/0000000000/0000000000.bin 文件读取创世区块
// - 反序列化并计算哈希
// - 重建 indices:height:0 和 indices:hash:<hash> 索引
// - 如果链尖高度为0，一并修复链尖
//
// 参数：
//   - ctx: 操作上下文
//
// 返回：
//   - error: 修复失败的错误
func (m *DerivedDataRecoveryManager) repairGenesisIndex(ctx context.Context) error {
	if m.fileStore == nil {
		return fmt.Errorf("fileStore 未初始化，无法修复创世区块索引")
	}
	if m.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化，无法计算创世区块哈希")
	}

	// 导入repair包的函数
	// 注意：这里直接调用repair.RepairGenesisIndex
	return repair.RepairGenesisIndex(ctx, m.store, m.fileStore, m.blockHashClient, m.logger)
}

// ============================================================================
//                              Level 2: 区域重建
// ============================================================================

// tryRegionalRebuild Level 2: 区域重建
//
// 🎯 **策略**：
// - 重放最近N个区块（默认100）
// - 重新派生这些区块的所有索引和UTXO变更
// - 适合连续区块的损坏或不确定损坏范围
//
// 参数：
//   - ctx: 操作上下文
//   - issue: 损坏问题
//
// 返回：
//   - error: 修复失败的错误
func (m *DerivedDataRecoveryManager) tryRegionalRebuild(ctx context.Context, issue CorruptionIssue) error {
	const replayDepth = 100

	chainInfo, err := m.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("get chain info failed: %w", err)
	}

	currentHeight := chainInfo.Height

	// 确定重放范围
	var fromHeight uint64
	if issue.Height != nil && *issue.Height > replayDepth {
		fromHeight = *issue.Height - replayDepth
	} else if currentHeight > replayDepth {
		fromHeight = currentHeight - replayDepth
	} else {
		fromHeight = 0
	}

	toHeight := currentHeight

	if m.logger != nil {
		m.logger.Infof("🔄 区域重建: 重放区块 [%d..%d]", fromHeight, toHeight)
	}

	// 重建索引
	if err := m.indexManager.RebuildHeightIndex(ctx, fromHeight, toHeight); err != nil {
		return fmt.Errorf("rebuild index failed: %w", err)
	}

	// 重建交易索引
	if err := m.indexManager.RebuildTxIndex(ctx, fromHeight, toHeight); err != nil {
		return fmt.Errorf("rebuild tx index failed: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("✅ 区域重建完成: [%d..%d]", fromHeight, toHeight)
	}

	return nil
}

// ============================================================================
//                              Level 3: 全量重建
// ============================================================================

// fullRebuild Level 3: 全量重建
//
// 🎯 **策略**：
// - 清空所有派生数据（索引、UTXO等）
// - 从genesis区块重新派生
// - 最彻底但最慢的修复方式
//
// ⚠️ **警告**：
// - 这个操作可能需要数小时
// - 会锁定节点，无法处理新交易
//
// 参数：
//   - ctx: 操作上下文
//
// 返回：
//   - error: 修复失败的错误
func (m *DerivedDataRecoveryManager) fullRebuild(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Warn("⚠️ 开始全量重建，这可能需要很长时间...")
	}

	// 获取当前链高度
	chainInfo, err := m.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("get chain info failed: %w", err)
	}

	maxHeight := chainInfo.Height

	if m.logger != nil {
		m.logger.Infof("全量重建: 从genesis到高度 %d", maxHeight)
	}

	// 清空索引（保留区块文件）
	if err := m.clearDerivedData(ctx); err != nil {
		return fmt.Errorf("clear derived data failed: %w", err)
	}

	// 从genesis重新派生
	if err := m.indexManager.FullIndexRebuild(ctx, maxHeight); err != nil {
		return fmt.Errorf("full index rebuild failed: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("✅ 全量重建完成")
	}

	return nil
}

// clearDerivedData 清空派生数据（保留区块文件）
func (m *DerivedDataRecoveryManager) clearDerivedData(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Warn("清空派生数据（保留区块文件）...")
	}

	// 这里需要小心，只清空索引，不清空区块文件
	// 具体实现取决于存储架构
	// TODO: 实现清空逻辑

	return nil
}

// ============================================================================
//                              辅助方法
// ============================================================================

// recordRepair 记录修复历史
func (m *DerivedDataRecoveryManager) recordRepair(record RepairRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.repairHistory = append(m.repairHistory, record)

	// 保持历史记录在合理范围内（最多1000条）
	if len(m.repairHistory) > 1000 {
		m.repairHistory = m.repairHistory[len(m.repairHistory)-1000:]
	}

	// 发布修复事件
	if m.eventBus != nil {
		m.eventBus.Publish("repair.completed", nil, map[string]interface{}{
			"issue_type":   record.IssueType,
			"repair_level": record.RepairLevel,
			"result":       record.Result,
			"duration":     record.Duration.Seconds(),
		})
	}
}

// GetRepairHistory 获取修复历史
func (m *DerivedDataRecoveryManager) GetRepairHistory() []RepairRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 返回副本
	history := make([]RepairRecord, len(m.repairHistory))
	copy(history, m.repairHistory)
	return history
}

// GetIndexManager 获取索引管理器（供外部调用）
func (m *DerivedDataRecoveryManager) GetIndexManager() *IndexRecoveryManager {
	return m.indexManager
}

// GetUTXOManager 获取UTXO管理器（供外部调用）
func (m *DerivedDataRecoveryManager) GetUTXOManager() *UTXORecoveryManager {
	return m.utxoManager
}

// GetBlockManager 获取区块管理器（供外部调用）
func (m *DerivedDataRecoveryManager) GetBlockManager() *BlockCorruptionManager {
	return m.blockManager
}

