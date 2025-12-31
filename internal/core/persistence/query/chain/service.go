// Package chain 实现链状态查询服务
package chain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

const chainIdentityGenesisHashKey = "system:chain_identity:genesis_hash"

// Service 链状态查询服务
type Service struct {
	storage    storage.BadgerStore
	logger     log.Logger
	blockQuery interfaces.InternalBlockQuery // 🆕 用于链尖修复

	// 指标（需要并发保护）
	metrics   *interfaces.QueryMetrics
	metricsMu sync.RWMutex

	// 状态（需要并发保护）
	mu            sync.RWMutex
	currentHeight uint64
	lastBlockHash []byte
	isHealthy     bool
	lastError     error
}

// NewService 创建链状态查询服务
// blockQuery 参数可选（用于链尖修复），如果为 nil 则使用备用修复策略
func NewService(storage storage.BadgerStore, logger log.Logger, blockQuery interfaces.InternalBlockQuery) (interfaces.InternalChainQuery, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}

	s := &Service{
		storage:    storage,
		logger:     logger,
		blockQuery: blockQuery, // 注入 blockQuery（可选）
		metrics: &interfaces.QueryMetrics{
			IsHealthy: true,
		},
		isHealthy: true,
	}

	if logger != nil {
		logger.Info("✅ ChainQuery 服务已创建")
	}

	return s, nil
}

// GetChainInfo 获取链基础信息
func (s *Service) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// 从存储获取链尖状态（遵循 data-architecture.md 规范）
	// 键格式：state:chain:tip
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("获取链尖状态失败: %w", err)
	}

	// 🆕 解析链尖数据（格式：height(8字节) + blockHash(32字节)）
	// 如果格式错误，尝试多层修复策略
	if len(tipData) < 40 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 链尖数据格式错误: len=%d, 尝试多层修复策略", len(tipData))
		}

		// 策略 1: 使用 blockQuery 修复
		if s.blockQuery != nil {
			if repaired, err := s.repairChainTip(ctx); err == nil && repaired != nil {
				if s.logger != nil {
					s.logger.Infof("✅ 链尖修复成功（策略1-blockQuery）: height=%d", repaired.Height)
				}
				return repaired, nil
			} else if s.logger != nil {
				s.logger.Warnf("策略1修复失败: %v, 尝试策略2", err)
			}
		}

		// 策略 2: 使用索引扫描修复
		if repaired, err := s.repairChainTipFallback(ctx); err == nil && repaired != nil {
			if s.logger != nil {
				s.logger.Infof("✅ 链尖修复成功（策略2-索引扫描）: height=%d", repaired.Height)
			}
			return repaired, nil
		} else if s.logger != nil {
			s.logger.Warnf("策略2修复失败: %v, 尝试策略3", err)
		}

		// 策略 3: 创世区块初始化（兜底）
		if repaired, err := s.repairChainTipGenesis(ctx); err == nil && repaired != nil {
			if s.logger != nil {
				s.logger.Warnf("⚠️ 链尖修复成功（策略3-创世区块）: 系统将从头同步")
			}
			return repaired, nil
		}

		// 所有策略都失败
		err := fmt.Errorf("链尖数据损坏且所有修复策略失败: len=%d", len(tipData))
		s.recordError(err)
		return nil, err
	}

	height := bytesToUint64(tipData[:8])
	blockHash := tipData[8:40]

	// 更新内部状态
	s.mu.Lock()
	s.currentHeight = height
	s.lastBlockHash = blockHash
	s.mu.Unlock()

	// 构造链信息
	chainInfo := &types.ChainInfo{
		Height:        height,
		BestBlockHash: blockHash,
		IsReady:       true,
		Status:        "normal",
	}

	return chainInfo, nil
}

// GetCurrentHeight 获取当前链高度
func (s *Service) GetCurrentHeight(ctx context.Context) (uint64, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// 键格式：state:chain:tip（遵循 data-architecture.md 规范）
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		s.recordError(err)
		return 0, fmt.Errorf("获取链尖状态失败: %w", err)
	}

	// 🆕 如果格式错误，尝试多层修复策略
	if len(tipData) < 8 {
		// 关键约束：空链首次启动时，不能由“查询侧自愈/创世兜底”抢跑写入链尖。
		// 仅当检测到 genesis_hash 元数据存在（链已创建）时，才允许触发修复逻辑。
		if len(tipData) == 0 {
			genesisHashBytes, metaErr := s.storage.Get(ctx, []byte(chainIdentityGenesisHashKey))
			if metaErr != nil {
				s.recordError(metaErr)
				return 0, fmt.Errorf("读取链身份元数据失败: %w", metaErr)
			}
			if len(genesisHashBytes) == 0 {
				if s.logger != nil {
					s.logger.Info("🆕 空链且无 genesis_hash：不触发链尖修复/创世兜底，等待启动流程创建创世区块")
				}
				return 0, nil
			}
		}

		if s.logger != nil {
			s.logger.Warnf("⚠️ 链尖数据格式错误: len=%d, 尝试多层修复策略", len(tipData))
		}

		// 策略 1: 使用 blockQuery 修复
		if s.blockQuery != nil {
			if repaired, err := s.repairChainTip(ctx); err == nil && repaired != nil {
				if s.logger != nil {
					s.logger.Infof("✅ 链尖修复成功（策略1-blockQuery）: height=%d", repaired.Height)
				}
				return repaired.Height, nil
			} else if s.logger != nil {
				s.logger.Warnf("策略1修复失败: %v, 尝试策略2", err)
			}
		}

		// 策略 2: 使用索引扫描修复
		if repaired, err := s.repairChainTipFallback(ctx); err == nil && repaired != nil {
			if s.logger != nil {
				s.logger.Infof("✅ 链尖修复成功（策略2-索引扫描）: height=%d", repaired.Height)
			}
			return repaired.Height, nil
		} else if s.logger != nil {
			s.logger.Warnf("策略2修复失败: %v, 尝试策略3", err)
		}

		// 策略 3: 创世区块初始化（兜底）
		if repaired, err := s.repairChainTipGenesis(ctx); err == nil && repaired != nil {
			if s.logger != nil {
				s.logger.Warnf("⚠️ 链尖修复成功（策略3-创世区块）: 系统将从头同步")
			}
			return repaired.Height, nil
		}

		// 所有策略都失败
		err := fmt.Errorf("链尖数据损坏且所有修复策略失败: len=%d", len(tipData))
		s.recordError(err)
		return 0, err
	}

	height := bytesToUint64(tipData[:8])

	// 更新内部状态
	s.mu.Lock()
	s.currentHeight = height
	s.mu.Unlock()

	return height, nil
}

// GetBestBlockHash 获取最佳区块哈希
func (s *Service) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// 键格式：state:chain:tip（遵循 data-architecture.md 规范）
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("获取链尖状态失败: %w", err)
	}

	if len(tipData) < 40 {
		err := fmt.Errorf("链尖数据格式错误")
		s.recordError(err)
		return nil, err
	}

	blockHash := tipData[8:40]

	// 更新内部状态
	s.mu.Lock()
	s.lastBlockHash = blockHash
	s.mu.Unlock()

	return blockHash, nil
}

// GetNodeMode 获取节点模式（P3-21：从配置或存储获取节点模式）
func (s *Service) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// P3-21: 优先从存储读取节点模式配置
	// 键格式：config:node:mode
	nodeModeKey := []byte("config:node:mode")
	if modeData, err := s.storage.Get(ctx, nodeModeKey); err == nil && len(modeData) > 0 {
		modeStr := string(modeData)
		// 验证节点模式是否有效
		mode := types.NodeMode(modeStr)
		if types.IsValidNodeMode(mode) {
			if s.logger != nil {
				s.logger.Debugf("从存储读取节点模式配置: %s", modeStr)
			}
			return mode, nil
		} else {
			if s.logger != nil {
				s.logger.Warnf("存储中的节点模式无效: %s，使用默认值", modeStr)
			}
		}
	}

	// 如果存储中没有配置，使用默认值（全节点模式）
	defaultMode := types.NodeModeFull
	if s.logger != nil {
		s.logger.Debugf("使用默认节点模式: %s", defaultMode)
	}

	// 可选：将默认值写入存储
	if err := s.storage.Set(ctx, nodeModeKey, []byte(defaultMode)); err != nil {
		// 写入失败不影响返回默认值，只记录警告
		if s.logger != nil {
			s.logger.Warnf("写入默认节点模式失败: %v", err)
		}
	}

	return defaultMode, nil
}

func (s *Service) IsDataFresh(ctx context.Context) (bool, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// ⚠️ 已废弃：同步状态不再持久化，chain:sync_status:current 仅在启动时初始化一次。
	// 为避免误判“数据新鲜”，此方法现在始终采用保守策略：
	// - 返回 false, nil，表示“不要信任本地数据一定是最新的”
	// - 调用方应改用 SystemSyncService.CheckSync() + 显式高度/时间阈值判断
	if s.logger != nil {
		s.logger.Warn("IsDataFresh 已废弃，请改用 SystemSyncService.CheckSync() 进行同步状态/新鲜度判断（当前实现始终返回 false）")
	}

	return false, nil
}

// IsReady 检查系统就绪状态
func (s *Service) IsReady(ctx context.Context) (bool, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// 检查链高度是否大于0
	height, err := s.GetCurrentHeight(ctx)
	if err != nil {
		s.recordError(err)
		return false, nil
	}

	if height > 0 {
		return true, nil
	}

	// 高度为0时代表仅有创世块，链已初始化，可视为就绪
	if s.logger != nil {
		s.logger.Debug("链高度为0，但已加载创世块，视为系统就绪")
	}
	return true, nil
}

// GetQueryMetrics 获取查询服务指标
//
// 🎯 **InternalChainQuery 接口实现**
func (s *Service) GetQueryMetrics(ctx context.Context) (*interfaces.QueryMetrics, error) {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	// 返回指标副本，避免外部修改
	metricsCopy := *s.metrics

	// 更新当前数据指标
	s.mu.RLock()
	metricsCopy.CurrentHeight = s.currentHeight
	if len(s.lastBlockHash) > 0 {
		metricsCopy.LastBlockHash = make([]byte, len(s.lastBlockHash))
		copy(metricsCopy.LastBlockHash, s.lastBlockHash)
	}
	metricsCopy.IsHealthy = s.isHealthy
	if s.lastError != nil {
		metricsCopy.ErrorMessage = s.lastError.Error()
	}
	s.mu.RUnlock()

	return &metricsCopy, nil
}

// GetSyncStatus 获取同步状态（已废弃，仅保留最小兼容性）。
//
// ⚠️ **强烈不推荐使用**：
//   - 持久化同步状态已废弃，本方法无法提供真实的网络同步信息；
//   - 仅返回“本地视角”的高度信息，Status/SyncProgress 不可用于任何业务决策；
//   - 调用方必须改用 `chain.SystemSyncService.CheckSync()` 获取实时同步状态。
//
// 当前实现策略（防误用）：
//   - 始终返回 Status=SyncStatusSyncing（表示“尚在同步或未知”）；
//   - NetworkHeight 固定为 0，SyncProgress 固定为 0；
//   - 仅将本地高度填入 CurrentHeight 作为诊断信息。
func (s *Service) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	startTime := time.Now()
	defer func() {
		s.recordQuery(time.Since(startTime), nil)
	}()

	// 1. 查询本地链高度
	localHeight, err := s.GetCurrentHeight(ctx)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("查询本地高度失败: %w", err)
	}

	// 2. 返回保守状态，避免被误判为“已同步”
	// ⚠️ 注意：此方法仅返回本地高度作为诊断信息，状态始终视为“未知/同步中”
	if s.logger != nil {
		s.logger.Warnf("GetSyncStatus 已废弃，仅返回本地高度信息（height=%d），请使用 SystemSyncService.CheckSync() 获取真实同步状态", localHeight)
	}

	return &types.SystemSyncStatus{
		Status:        types.SyncStatusSyncing, // 保守视为"正在同步/未知"
		CurrentHeight: localHeight,
		NetworkHeight: 0,    // 无法获取网络高度，固定为 0
		SyncProgress:  0.0,  // 无法判断进度，固定为 0
		LastSyncTime:  types.RFC3339Time(time.Now()),
	}, nil
}

// ============================================================================
//                              降级查询方法（可选优化）
// ============================================================================

// GetChainInfoWithFallback 获取链信息（带降级）
// 如果链尖不可用，返回降级信息而不是错误
//
// 🎯 **使用场景**：
// - 监控系统：即使链尖损坏也能获取部分信息
// - 健康检查：避免因链尖问题导致整体服务不可用
// - 诊断工具：在问题发生时仍能获取系统状态
//
// 返回：
// - 正常情况：返回完整的链信息
// - 降级情况：返回最小可用信息（高度0、空哈希、IsReady=false）
func (s *Service) GetChainInfoWithFallback(ctx context.Context) (*types.ChainInfo, error) {
	// 尝试正常获取
	info, err := s.GetChainInfo(ctx)
	if err == nil {
		return info, nil
	}

	// 降级：返回最小可用信息
	if s.logger != nil {
		s.logger.Warnf("⚠️ 链信息查询失败，返回降级信息: %v", err)
	}

	return &types.ChainInfo{
		Height:        0,
		BestBlockHash: make([]byte, 32),
		IsReady:       false,
		Status:        fmt.Sprintf("degraded: %v", err),
	}, nil
}

// ============================================================================
//                              辅助方法（指标收集）
// ============================================================================

// recordQuery 记录查询指标
//
// 🎯 **从 chain/query/service.go 迁移的优秀逻辑**
//
// 特点：
// - 滑动平均算法：使用指数加权移动平均（EWMA）
// - 并发安全：使用读写锁保护
// - 性能优化：低开销的指标更新
func (s *Service) recordQuery(duration time.Duration, err error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.QueryCount++
	s.metrics.LastQueryTime = time.Now().Unix()

	if err != nil {
		s.metrics.FailureCount++
	} else {
		s.metrics.SuccessCount++
	}

	// 更新平均查询耗时（滑动平均）
	alpha := 0.1 // 平滑系数
	newTime := duration.Seconds()
	if s.metrics.AverageQueryTime == 0 {
		s.metrics.AverageQueryTime = newTime
	} else {
		s.metrics.AverageQueryTime = alpha*newTime + (1-alpha)*s.metrics.AverageQueryTime
	}

	// 更新最大查询耗时
	if newTime > s.metrics.MaxQueryTime {
		s.metrics.MaxQueryTime = newTime
	}
}

// recordError 记录错误
//
// 🎯 **从 chain/query/service.go 迁移的优秀逻辑**
//
// 特点：
// - 错误状态跟踪
// - 健康状态管理
func (s *Service) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isHealthy = false
	s.lastError = err
}

// 🆕 repairChainTip 尝试从最新区块重建链尖数据
func (s *Service) repairChainTip(ctx context.Context) (*types.ChainInfo, error) {
	if s.blockQuery == nil {
		return nil, fmt.Errorf("blockQuery 未注入，无法修复链尖")
	}

	maxScanHeight := uint64(10000)
	scanStep := uint64(100)
	var foundBlock interface{}
	var foundHeight uint64

	for h := maxScanHeight; h > 0; h -= scanStep {
		block, err := s.blockQuery.GetBlockByHeight(ctx, h)
		if err == nil && block != nil {
			foundBlock = block
			foundHeight = h
			break
		}
		if h < scanStep {
			for hh := h; hh > 0; hh-- {
				block, err := s.blockQuery.GetBlockByHeight(ctx, hh)
				if err == nil && block != nil {
					foundBlock = block
					foundHeight = hh
					break
				}
			}
			if foundBlock != nil {
				break
			}
		}
	}

	if foundBlock == nil {
		return nil, fmt.Errorf("无法找到任何可用区块进行修复")
	}

	var blockHash []byte
	if blockWithHash, ok := foundBlock.(interface{ GetHash() []byte }); ok {
		blockHash = blockWithHash.GetHash()
	} else {
		blockHash = make([]byte, 32)
	}

	tipKey := []byte("state:chain:tip")
	tipData := make([]byte, 40)
	tipData[0] = byte(foundHeight >> 56)
	tipData[1] = byte(foundHeight >> 48)
	tipData[2] = byte(foundHeight >> 40)
	tipData[3] = byte(foundHeight >> 32)
	tipData[4] = byte(foundHeight >> 24)
	tipData[5] = byte(foundHeight >> 16)
	tipData[6] = byte(foundHeight >> 8)
	tipData[7] = byte(foundHeight)
	copy(tipData[8:40], blockHash)

	err := s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return tx.Set(tipKey, tipData)
	})
	if err != nil {
		return nil, fmt.Errorf("写入修复后的链尖失败: %w", err)
	}

	return &types.ChainInfo{
		Height:        foundHeight,
		BestBlockHash: blockHash,
		IsReady:       true,
		Status:        "repaired",
	}, nil
}

// repairChainTipFallback 备用修复策略（不依赖 blockQuery）
// 策略：扫描 indices:height:* 找到最大高度，重建链尖
func (s *Service) repairChainTipFallback(ctx context.Context) (*types.ChainInfo, error) {
	if s.logger != nil {
		s.logger.Warn("🔧 使用备用策略修复链尖（索引扫描）")
	}

	// 从 indices:height:* 找到最大高度
	const prefix = "indices:height:"
	maxHeight := uint64(0)
	var maxHash []byte
	found := false

	// 使用 storage 的 PrefixScan 方法扫描所有高度索引
	entries, err := s.storage.PrefixScan(ctx, []byte(prefix))
	if err != nil {
		return nil, fmt.Errorf("索引扫描失败: %w", err)
	}

	for key, value := range entries {
		// 解析高度（从键的后缀部分）
		if len(key) < len(prefix) {
			continue
		}
		heightStr := key[len(prefix):]
		
		// 直接按字符串解析为数字
		var height uint64
		_, err := fmt.Sscanf(heightStr, "%d", &height)
		if err != nil {
			continue
		}

		// 获取区块哈希（前32字节）
		if len(value) < 32 {
			continue
		}

		if !found || height > maxHeight {
			maxHeight = height
			maxHash = make([]byte, 32)
			copy(maxHash, value[:32])
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("索引扫描未找到任何有效区块")
	}

	// 重建链尖数据
	tipData := make([]byte, 40)
	tipData[0] = byte(maxHeight >> 56)
	tipData[1] = byte(maxHeight >> 48)
	tipData[2] = byte(maxHeight >> 40)
	tipData[3] = byte(maxHeight >> 32)
	tipData[4] = byte(maxHeight >> 24)
	tipData[5] = byte(maxHeight >> 16)
	tipData[6] = byte(maxHeight >> 8)
	tipData[7] = byte(maxHeight)
	copy(tipData[8:40], maxHash)

	tipKey := []byte("state:chain:tip")
	err = s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return tx.Set(tipKey, tipData)
	})
	if err != nil {
		return nil, fmt.Errorf("写入修复后的链尖失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ 备用策略修复成功: height=%d hash=%x", maxHeight, maxHash[:8])
	}

	return &types.ChainInfo{
		Height:        maxHeight,
		BestBlockHash: maxHash,
		IsReady:       true,
		Status:        "repaired_fallback",
	}, nil
}

// repairChainTipGenesis 最后的兜底策略：初始化为创世区块
func (s *Service) repairChainTipGenesis(ctx context.Context) (*types.ChainInfo, error) {
	if s.logger != nil {
		s.logger.Warn("🔧 使用创世区块初始化链尖（兜底策略）")
	}

	// 创世区块高度为 0，哈希为全零
	tipData := make([]byte, 40)
	// height = 0 (前8字节已经是0)
	// hash = 全零 (后32字节已经是0)

	tipKey := []byte("state:chain:tip")
	err := s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return tx.Set(tipKey, tipData)
	})
	if err != nil {
		return nil, fmt.Errorf("创世区块初始化失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("✅ 创世区块初始化完成，系统将从同步开始")
	}

	return &types.ChainInfo{
		Height:        0,
		BestBlockHash: make([]byte, 32),
		IsReady:       false, // 需要同步
		Status:        "genesis_initialized",
	}, nil
}

// bytesToUint64Safe 安全地将字节数组转换为uint64，处理各种长度
func bytesToUint64Safe(b []byte) (uint64, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("empty bytes")
	}
	if len(b) == 8 {
		return bytesToUint64(b), nil
	}
	// 处理其他长度的字节数组
	var result uint64
	for i := 0; i < len(b) && i < 8; i++ {
		result = (result << 8) | uint64(b[i])
	}
	return result, nil
}

// bytesToUint64 将字节数组转换为uint64
func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// ============================================================================
//                          启动时完整性检查和修复
// ============================================================================

// ValidateAndRepairOnStartup 启动时验证并修复链尖数据
// 应该在服务创建后立即调用
func (s *Service) ValidateAndRepairOnStartup(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("🔍 启动时链尖数据完整性检查...")
	}

	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)

	// 情况 1: 存储读取失败（非“键不存在”，BadgerStore.Get 键不存在时返回 (nil, nil)）
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 获取链尖数据失败，执行强制修复: %v", err)
		}
		return s.forceRepairChainTip(ctx)
	}

	// 情况 1.5: 空链且无 genesis_hash → 首次启动，不执行强制修复
	if len(tipData) == 0 {
		genesisHashBytes, metaErr := s.storage.Get(ctx, []byte(chainIdentityGenesisHashKey))
		if metaErr != nil {
			return fmt.Errorf("读取链身份元数据失败: %w", metaErr)
		}
		if len(genesisHashBytes) == 0 {
			if s.logger != nil {
				s.logger.Info("🆕 启动时检测为空链且无 genesis_hash：跳过链尖强制修复（由启动流程负责创世）")
			}
			return nil
		}
	}

	// 情况 2: 数据格式错误
	if len(tipData) < 40 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 链尖数据格式错误: len=%d, 执行强制修复", len(tipData))
		}
		return s.forceRepairChainTip(ctx)
	}

	// 情况 3: 高度为 0 但非创世状态（可能是损坏）
	height := bytesToUint64(tipData[:8])
	if height == 0 {
		// 检查是否有其他区块数据
		if hasBlocks, _ := s.hasAnyBlocks(ctx); hasBlocks {
			if s.logger != nil {
				s.logger.Warn("⚠️ 链尖高度为0但存在区块数据，执行强制修复")
			}
			return s.forceRepairChainTip(ctx)
		}
	}

	if s.logger != nil {
		s.logger.Infof("✅ 链尖数据完整性检查通过: height=%d", height)
	}

	return nil
}

// forceRepairChainTip 强制修复链尖（启动时专用）
func (s *Service) forceRepairChainTip(ctx context.Context) error {
	// 尝试所有修复策略
	var lastErr error

	// 策略 1
	if s.blockQuery != nil {
		if repaired, err := s.repairChainTip(ctx); err == nil && repaired != nil {
			if s.logger != nil {
				s.logger.Infof("✅ 启动时修复成功（策略1）: height=%d", repaired.Height)
			}
			return nil
		} else {
			lastErr = err
		}
	}

	// 策略 2
	if repaired, err := s.repairChainTipFallback(ctx); err == nil && repaired != nil {
		if s.logger != nil {
			s.logger.Infof("✅ 启动时修复成功（策略2）: height=%d", repaired.Height)
		}
		return nil
	} else {
		lastErr = err
	}

	// 策略 3
	if repaired, err := s.repairChainTipGenesis(ctx); err == nil && repaired != nil {
		if s.logger != nil {
			s.logger.Warn("⚠️ 启动时修复成功（策略3-创世区块）")
		}
		return nil
	} else {
		lastErr = err
	}

	return fmt.Errorf("启动时链尖修复失败: %w", lastErr)
}

// hasAnyBlocks 检查是否存在任何区块数据
func (s *Service) hasAnyBlocks(ctx context.Context) (bool, error) {
	// 检查 indices:height: 前缀是否有数据
	entries, err := s.storage.PrefixScan(ctx, []byte("indices:height:"))
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// 编译时检查接口实现
var _ interfaces.InternalChainQuery = (*Service)(nil)
