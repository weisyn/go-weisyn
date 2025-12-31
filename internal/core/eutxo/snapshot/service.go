// Package snapshot 实现 UTXO 快照服务
//
// 🎯 **核心职责**：
// - UTXO 快照创建
// - UTXO 快照恢复
// - 快照管理（删除、列表）
// - 性能指标收集
//
// 🏗️ **设计理念**：
// - 快照创建：获取所有 UTXO，序列化，压缩，存储
// - 快照恢复：加载快照，解压，验证，恢复 UTXO
// - 延迟注入：Writer 和 Query 通过延迟注入避免循环依赖
// - 并发安全：使用 Mutex 保护
//
// 详细设计说明请参考：internal/core/eutxo/TECHNICAL_DESIGN.md
package snapshot

import (
	"context"
	"fmt"
	"sync"

	"github.com/weisyn/v1/internal/core/eutxo/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	// persistence "github.com/weisyn/v1/pkg/interfaces/persistence" // ⚠️ 已移除：EUTXO 模块不应依赖 persistence 模块
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

// Service UTXO 快照服务
//
// 🎯 **核心职责**：
// - 实现 InternalUTXOSnapshot 接口
// - 管理 UTXO 快照的创建、恢复、删除
// - 提供性能指标
//
// 💡 **并发安全**：
// - mu: 保护快照操作（互斥锁）
// - metricsMu: 保护性能指标更新（互斥锁）
type Service struct {
	// ==================== 依赖注入 ====================

	// storage 存储服务（必需）
	storage storage.BadgerStore

	// hasher 哈希服务（必需）
	hasher crypto.HashManager

	// blockHashClient 区块哈希服务客户端（用于计算区块哈希）
	blockHashClient core.BlockHashServiceClient

	// logger 日志记录器（可选）
	logger log.Logger

	// eventBus 事件总线（可选）
	eventBus event.EventBus

	// ==================== 延迟依赖注入 ====================

	// writer UTXO 写入器（用于快照恢复）
	writer interfaces.InternalUTXOWriter

	// query UTXO 查询器（用于快照创建）
	query interfaces.InternalUTXOQuery

	// blockQuery 区块查询器（已移除，架构修复）
	// ⚠️ **架构修复**：EUTXO 模块不应依赖 persistence 模块
	// 区块哈希应该由调用方（CHAIN 层的 ForkHandler）提供
	// blockQuery persistence.BlockQuery // 已移除

	// ==================== 状态与并发保护 ====================

	// mu 并发保护
	mu sync.Mutex

	// ==================== 配置 ====================

	// config 快照配置（容错策略）
	config SnapshotConfig
}

// SnapshotConfig 快照配置
type SnapshotConfig struct {
	// CorruptUTXOPolicy 损坏UTXO处理策略
	// - "reject": 严格模式，拒绝创建快照（默认）
	// - "repair": 修复模式，自动修复并继续
	// - "warn": 告警模式，记录日志但继续
	CorruptUTXOPolicy string

	// MaxRepairableCount 最多自动修复的UTXO数量
	MaxRepairableCount int
}

// NewService 创建 UTXO 快照服务
//
// 🎯 **创建流程**：
// 1. 验证必需依赖
// 2. 初始化性能指标
// 3. 返回服务实例
//
// 参数：
//   - storage: 存储服务（必需）
//   - hasher: 哈希服务（必需）
//   - blockHashClient: 区块哈希服务客户端（可选，用于计算区块哈希）
//   - logger: 日志记录器（可选）
//
// 返回：
//   - interfaces.InternalUTXOSnapshot: UTXO 快照服务实例
//   - error: 创建错误，nil 表示成功
func NewService(
	storage storage.BadgerStore,
	hasher crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	logger log.Logger,
) (interfaces.InternalUTXOSnapshot, error) {
	// 默认配置
	defaultConfig := SnapshotConfig{
		CorruptUTXOPolicy:  "repair", // 默认修复模式
		MaxRepairableCount: 100,      // 最多修复100个
	}

	// 验证必需依赖
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if hasher == nil {
		return nil, fmt.Errorf("hasher 不能为空")
	}

	// 创建服务实例
	s := &Service{
		storage:         storage,
		hasher:          hasher,
		blockHashClient: blockHashClient,
		logger:          logger,
		config:          defaultConfig,
	}

	if logger != nil {
		logger.Info("✅ UTXOSnapshot 服务已创建")
		logger.Infof("   容错策略: %s, 最大修复数: %d", defaultConfig.CorruptUTXOPolicy, defaultConfig.MaxRepairableCount)
	}

	return s, nil
}

// ============================================================================
//                          延迟依赖注入
// ============================================================================

// SetWriter 设置 UTXO 写入器（延迟注入）
//
// 实现 interfaces.InternalUTXOSnapshot.SetWriter
func (s *Service) SetWriter(writer interfaces.InternalUTXOWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = writer

	if s.logger != nil {
		s.logger.Info("🔗 UTXOWriter 已注入到 UTXOSnapshot")
	}
}

// SetQuery 设置 UTXO 查询器（延迟注入）
//
// 实现 interfaces.InternalUTXOSnapshot.SetQuery
func (s *Service) SetQuery(query interfaces.InternalUTXOQuery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query

	if s.logger != nil {
		s.logger.Info("🔗 UTXOQuery 已注入到 UTXOSnapshot")
	}
}

// SetBlockQuery 设置区块查询器（已移除，架构修复）
//
// ⚠️ **架构修复**：EUTXO 模块不应依赖 persistence 模块
// 区块哈希应该由调用方（CHAIN 层的 ForkHandler）提供
// 此方法已移除，不再需要 BlockQuery 依赖
// func (s *Service) SetBlockQuery(blockQuery persistence.BlockQuery) {
// 	// 已移除
// }

// ============================================================================
//                          内部管理方法
// ============================================================================

// ValidateSnapshot 验证快照数据的有效性
//
// 实现 interfaces.InternalUTXOSnapshot.ValidateSnapshot
func (s *Service) ValidateSnapshot(ctx context.Context, snapshot *types.UTXOSnapshotData) error {
	if snapshot == nil {
		return fmt.Errorf("快照数据不能为空")
	}

	if snapshot.SnapshotID == "" {
		return fmt.Errorf("快照ID不能为空")
	}

	if len(snapshot.StateRoot) != 32 {
		return fmt.Errorf("快照状态根长度必须为32字节")
	}

	// ✅ 支持 height=0（genesis 快照）
	// 注意：CreateSnapshot(height=0) 只允许在链尖也为 0 时创建（见 CreateSnapshot 特判），避免产生“伪快照”。

	return nil
}

// 编译时检查接口实现
var _ interfaces.InternalUTXOSnapshot = (*Service)(nil)
