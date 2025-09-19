// Package resource 提供WES区块链资源存储服务实现
//
// 🎯 **资源存储管理器 (Resource Storage Manager)**
//
// 本文件实现了资源存储服务，专注于：
// - 混合存储架构：FileStore + BadgerStore + MemoryStore
// - 内容寻址：基于SHA-256哈希的去重存储
// - 事务一致性：文件存储与索引的原子性操作
// - 引用管理：ResourceUTXO的生命周期管理
//
// 🏗️ **设计原则**
// - 依赖注入：通过构造函数注入所需依赖
// - 职责分离：将具体实现委托给专门的子文件
// - 薄接口：Manager作为统一入口，具体逻辑在子文件中
// - 业务导向：基于实际业务需求设计，专注核心场景
package resource

import (
	"context"
	"fmt"

	// 公共接口和类型
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 内部接口和配置
	repositoryconfig "github.com/weisyn/v1/internal/config/repository"
	"github.com/weisyn/v1/internal/core/repositories/interfaces"
)

// ============================================================================
//                              服务结构定义
// ============================================================================

// Manager 资源存储管理器
//
// 🎯 **统一资源存储服务入口**
//
// 负责实现 InternalResourceManager 的所有接口方法，并将具体实现
// 委托给专门的子文件处理。采用混合存储架构，确保高性能和数据一致性。
//
// 架构特点：
// - 统一入口：所有资源存储操作的统一访问点
// - 混合存储：FileStore(文件) + BadgerStore(索引) + MemoryStore(缓存)
// - 依赖注入：通过构造函数注入必需的存储和加密依赖
// - 委托实现：将具体业务逻辑委托给专门的子文件
// - 事务安全：所有状态变更都在事务中进行
// 确保Manager实现了公共接口
var _ repository.ResourceManager = (*Manager)(nil)

type Manager struct {
	// 核心依赖
	logger      log.Logger          // 日志服务
	fileStore   storage.FileStore   // 文件存储服务
	badgerStore storage.BadgerStore // 索引存储服务
	memoryStore storage.MemoryStore // 内存缓存服务

	// 密码学依赖
	hashManager crypto.HashManager // 哈希计算服务

	// 配置参数
	config *repositoryconfig.RepositoryOptions // 资源仓库配置
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewManager 创建资源存储管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//   - logger: 日志服务
//   - fileStore: 文件存储服务
//   - badgerStore: 索引存储服务
//   - memoryStore: 内存缓存服务（可选）
//   - hashManager: 哈希计算服务
//   - resourceBasePath: 资源存储根路径
//
// 返回：
//   - interfaces.InternalResourceManager: 内部资源管理器接口实例
//   - error: 创建错误
func NewManager(
	logger log.Logger,
	fileStore storage.FileStore,
	badgerStore storage.BadgerStore,
	memoryStore storage.MemoryStore,
	hashManager crypto.HashManager,
	config *repositoryconfig.RepositoryOptions,
) (interfaces.InternalResourceManager, error) {
	// 必需依赖验证
	if fileStore == nil {
		return nil, fmt.Errorf("resource manager: file store 不能为空")
	}
	if badgerStore == nil {
		return nil, fmt.Errorf("resource manager: badger store 不能为空")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("resource manager: hash manager 不能为空")
	}

	manager := &Manager{
		logger:      logger,
		fileStore:   fileStore,
		badgerStore: badgerStore,
		memoryStore: memoryStore, // 可选，允许为nil
		hashManager: hashManager,
		config:      config,
	}

	if logger != nil {
		logger.Debug("资源存储管理器初始化完成")
	}

	return manager, nil
}

// ============================================================================
//                           编译时接口检查
// ============================================================================

// 确保 Manager 实现了 InternalResourceManager 接口
var _ interfaces.InternalResourceManager = (*Manager)(nil)

// ============================================================================
//                          📦 公共接口实现 - 资源存储
// ============================================================================

// StoreResourceFile 存储资源文件
//
// 🎯 **统一文件存储方法 (Unified File Storage Method)**
//
// 基于"文件到文件"的简单理念，支持任意大小的文件统一处理。
// 内部自动优化，无需调用方做文件大小判断或选择不同接口。
//
// 📝 **处理流程**：
//  1. 流式读取源文件并计算SHA-256哈希
//  2. 检查内容去重（相同哈希只存储一次）
//  3. 复制文件到基于哈希的存储路径
//  4. 建立元数据索引
//
// 💡 **技术特点**：
//   - 🎯 统一处理：所有文件用同一套逻辑，无大小区分
//   - 🧠 内存高效：流式操作，内存占用恒定
//   - ⚡ 高性能：避免临时文件和重复读写
//   - 🔒 去重存储：基于内容哈希的自动去重
//
// 📝 **参数说明**：
//   - ctx: 上下文，用于取消操作和超时控制
//   - sourceFilePath: 源文件的完整路径
//   - metadata: 元数据映射，包含类型、创建者等信息
//
// 🔄 **返回值**：
//   - []byte: 文件内容的SHA-256哈希值（32字节）
//   - error: 存储操作错误信息
func (m *Manager) StoreResourceFile(ctx context.Context, sourceFilePath string, metadata map[string]string) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("📁 存储文件: %s", sourceFilePath)
	}
	// 委托给store.go中的统一文件存储实现
	return m.storeResourceFile(ctx, sourceFilePath, metadata)
}

// GetResourceByHash 基于内容哈希获取资源信息
func (m *Manager) GetResourceByHash(ctx context.Context, contentHash []byte) (*types.ResourceStorageInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("🔍 按哈希查询资源: %x", contentHash)
	}
	// 调用具体实现方法 (query.go)
	return m.getResourceByHash(ctx, contentHash)
}

// ListResourcesByType 按类型列出资源
func (m *Manager) ListResourcesByType(ctx context.Context, resourceType string, offset int, limit int) ([]*types.ResourceStorageInfo, error) {
	if m.logger != nil {
		m.logger.Debugf("列出资源: 类型=%s, offset=%d, limit=%d", resourceType, offset, limit)
	}
	// 调用具体实现方法 (query.go)
	return m.listResourcesByType(ctx, resourceType, offset, limit)
}

// ============================================================================
//                        🔍 内部扩展接口实现 - 一致性管理
// ============================================================================

// VerifyResourceIntegrity 验证单个资源的存储完整性
func (m *Manager) VerifyResourceIntegrity(ctx context.Context, contentHash []byte) error {
	if m.logger != nil {
		m.logger.Debugf("验证资源完整性: %x", contentHash)
	}
	// 调用具体实现方法 (consistency.go)
	return m.verifyResourceIntegrity(ctx, contentHash)
}

// RepairStorageInconsistency 修复存储不一致状态
func (m *Manager) RepairStorageInconsistency(ctx context.Context) (int, error) {
	if m.logger != nil {
		m.logger.Debug("开始修复存储不一致状态")
	}
	// 调用具体实现方法 (consistency.go)
	return m.repairStorageInconsistency(ctx)
}

// ============================================================================
//                        📊 内部扩展接口实现 - 引用管理
// ============================================================================

// GetResourceReferenceCount 获取资源引用计数
func (m *Manager) GetResourceReferenceCount(ctx context.Context, contentHash []byte) (int32, error) {
	if m.logger != nil {
		m.logger.Debugf("获取资源引用计数: %x", contentHash)
	}
	// 调用具体实现方法 (lifecycle.go)
	return m.getResourceReferenceCount(ctx, contentHash)
}

// IncrementResourceReference 增加资源引用计数
func (m *Manager) IncrementResourceReference(ctx context.Context, contentHash []byte) error {
	if m.logger != nil {
		m.logger.Debugf("增加资源引用: %x", contentHash)
	}
	// 调用具体实现方法 (lifecycle.go)
	return m.incrementResourceReference(ctx, contentHash)
}

// DecrementResourceReference 减少资源引用计数
func (m *Manager) DecrementResourceReference(ctx context.Context, contentHash []byte) error {
	if m.logger != nil {
		m.logger.Debugf("减少资源引用: %x", contentHash)
	}
	// 调用具体实现方法 (lifecycle.go)
	return m.decrementResourceReference(ctx, contentHash)
}

// ============================================================================
//                       🗑️ 内部扩展接口实现 - 生命周期管理
// ============================================================================

// MarkResourceForCleanup 标记资源待清理
func (m *Manager) MarkResourceForCleanup(ctx context.Context, contentHash []byte) error {
	if m.logger != nil {
		m.logger.Debugf("标记资源待清理: %x", contentHash)
	}
	// 调用具体实现方法 (lifecycle.go)
	return m.markResourceForCleanup(ctx, contentHash)
}

// CleanupUnreferencedResources 清理无引用的资源
func (m *Manager) CleanupUnreferencedResources(ctx context.Context, maxCleanupCount int) (int, error) {
	if m.logger != nil {
		m.logger.Debugf("清理无引用资源, 最大清理数量: %d", maxCleanupCount)
	}
	// 调用具体实现方法 (lifecycle.go)
	return m.cleanupUnreferencedResources(ctx, maxCleanupCount)
}
