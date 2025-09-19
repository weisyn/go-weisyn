// Package fork 提供区块链分叉处理的核心实现
//
// 🔄 **分叉管理器 (Fork Manager)**
//
// 本文件实现分叉管理的薄管理层，遵循项目的通用设计原则：
// - 实现内部接口：继承公共接口并扩展内部功能
// - 依赖注入：通过构造函数注入所需依赖
// - 职责单一：专注分叉管理协调，具体处理委托给专门文件
// - 薄管理层：保持简洁，主要负责方法路由和依赖协调
//
// 🎯 **职责定位**：
// - 实现InternalForkService接口
// - 协调分叉处理流程
// - 委托具体处理给processor组件
//
// 详细设计文档：docs/implementation/FORK_HANDLING_DESIGN.md
package fork

import (
	"context"
	"fmt"

	// 内部接口
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"

	// 公共接口
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 协议定义
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ============================================================================
//                              管理器实现
// ============================================================================

// Manager 分叉处理管理器
//
// 🎯 **职责定位**：提供完整的分叉处理服务
//
// 依赖关系：
// - ChainService：链状态管理服务
// - BlockService：区块验证处理服务
// - RepositoryManager：底层数据存储访问
// - EventPublisher：事件发布服务
// - Logger：日志记录器（可选）
//
// 实现特点：
// - 继承内部接口，确保API兼容性
// - 采用薄管理层设计，处理逻辑委托给processor
// - 支持完整的异步分叉处理流程
// - 提供详细的错误处理和日志记录
type Manager struct {
	// 核心依赖
	chainService            interfaces.InternalChainService    // 链状态管理服务
	blockValidatorProcessor interfaces.BlockValidatorProcessor // 🎯 区块验证和处理服务（细粒度接口）
	repo                    repository.RepositoryManager       // 数据存储管理器
	eventPub                eventiface.EventBus                // 事件总线
	logger                  log.Logger                         // 日志记录器（可选）

	// 处理组件
	processor *Processor // 分叉处理器
}

// NewManager 创建新的分叉管理器实例
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数说明：
//   - chainService: 链状态管理服务
//   - blockService: 区块验证处理服务
//   - repo: 仓储管理器，提供底层数据访问能力
//   - eventPub: 事件发布器，用于发送分叉事件
//   - logger: 日志记录器，用于记录操作日志（可选）
//
// 返回：
//   - interfaces.InternalForkService: 内部分叉服务接口实例
//
// 设计说明：
// - 使用依赖注入模式，便于测试和扩展
// - 返回内部接口类型，确保实现完整性
// - 自动满足公共 ForkService 接口要求（如果有的话）
// - 初始化处理器组件，支持委托处理架构
func NewManager(
	chainService interfaces.InternalChainService,
	blockValidatorProcessor interfaces.BlockValidatorProcessor, // 🎯 使用细粒度接口替代完整BlockService
	repo repository.RepositoryManager,
	eventPub eventiface.EventBus,
	logger log.Logger,
) interfaces.InternalForkService {
	manager := &Manager{
		chainService:            chainService,
		blockValidatorProcessor: blockValidatorProcessor, // 🎯 使用细粒度接口
		repo:                    repo,
		eventPub:                eventPub,
		logger:                  logger,
	}

	// 创建处理器
	manager.processor = NewProcessor(
		chainService,
		blockValidatorProcessor, // 🎯 使用细粒度接口
		repo,
		eventPub,
		logger,
	)

	return manager
}

// ============================================================================
//                              接口实现
// ============================================================================

// HandleFork 处理分叉区块
//
// 🎯 **InternalForkService接口实现**
//
// 此方法实现InternalForkService接口，提供异步分叉处理能力。
// 按照薄管理层设计原则，主要负责参数验证和委托处理。
//
// 参数：
//   - ctx: 处理上下文
//   - forkBlock: 分叉区块数据
//
// 返回：
//   - error: 处理失败的错误（nil表示成功启动处理）
func (m *Manager) HandleFork(ctx context.Context, forkBlock *core.Block) error {
	// 参数验证
	if forkBlock == nil {
		if m.logger != nil {
			m.logger.Errorf("[ForkManager] 分叉区块为空")
		}
		return fmt.Errorf("分叉区块不能为空")
	}

	// 委托给处理器执行具体逻辑
	return m.processor.HandleFork(ctx, forkBlock)
}
