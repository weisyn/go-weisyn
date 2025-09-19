// Package repository 提供WES区块链统一数据存储层的实现
//
// 🗄️ **数据仓储管理器 (Repository Manager)**
//
// 本文件实现了数据仓储服务，专注于：
// - 区块数据操作：存储、查询、索引管理
// - 交易权利管理：交易查询、nonce防重放攻击
// - 资源能力管理：基于内容哈希的资源查询
//
// 🏗️ **设计原则**
// - 单一数据源：严格遵循区块作为唯一数据写入点
// - 依赖注入：通过构造函数注入所需依赖
// - 职责分离：将不同业务域操作分散到专门文件
// - 高效查询：基于多重索引提供O(1)查询性能
package repository

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	// protobuf定义
	core "github.com/weisyn/v1/pb/blockchain/block"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"

	// 配置
	repositoryConfig "github.com/weisyn/v1/internal/config/repository"

	// 内部接口
	"github.com/weisyn/v1/internal/core/repositories/interfaces"

	// 子模块
	"github.com/weisyn/v1/internal/core/repositories/repository/index"
	"github.com/weisyn/v1/internal/core/repositories/repository/resource"
	"github.com/weisyn/v1/internal/core/repositories/repository/transaction"
	"github.com/weisyn/v1/internal/core/repositories/repository/utxo"
)

// ============================================================================
//                              组件类型定义
// ============================================================================

// 各子模块组件类型（已完成集成）

// ============================================================================
//                              服务结构定义
// ============================================================================

// Manager 数据仓储管理器
//
// 🎯 **统一数据仓储服务入口**
//
// 负责实现 RepositoryManager 的所有公共接口方法，并将具体实现
// 委托给专门的子文件处理。遵循单一数据源原则，确保数据一致性。
//
// 架构特点：
// - 统一入口：所有数据仓储操作的统一访问点
// - 依赖注入：通过构造函数注入必需的存储依赖
// - 委托实现：将具体业务逻辑委托给专门的子文件
// - 数据完整性：原子性操作确保数据一致性
type Manager struct {
	// ========== 核心依赖 ==========
	logger      log.Logger          // 日志服务
	badgerStore storage.BadgerStore // 持久化存储
	memoryStore storage.MemoryStore // 内存缓存
	hashManager crypto.HashManager  // 哈希计算服务

	// ========== 存储核心组件 ==========
	blockStorage *BlockStorage // 区块存储组件
	chainState   *ChainState   // 区块链状态管理

	// ========== 子模块服务 ==========
	indexManager       *index.IndexManager             // 统一索引管理器
	txService          *transaction.TransactionService // 交易服务
	resService         *resource.ResourceService       // 资源服务
	utxoClient         *utxo.UTXOService               // UTXO服务
	outboxManager      *OutboxManager                  // Outbox事件管理器
	performanceMonitor *PerformanceMonitor             // 性能监控器

	// ========== 配置参数 ==========
	config         *repositoryConfig.RepositoryOptions // 配置选项
	configProvider config.Provider                     // 配置提供者

}

// ============================================================================
//                              构造函数
// ============================================================================

// NewManager 创建数据仓储管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//
//	logger: 日志服务
//	badgerStore: 持久化存储
//	memoryStore: 内存缓存
//	hashManager: 哈希计算服务
//
// 返回：
//
//	*Manager: 数据仓储管理器实例
//	error: 创建错误
func NewManager(
	logger log.Logger,
	badgerStore storage.BadgerStore,
	memoryStore storage.MemoryStore,
	hashManager crypto.HashManager,
	transactionHashServiceClient transactionpb.TransactionHashServiceClient,
	blockHashServiceClient core.BlockHashServiceClient,
	utxoManager interfaces.InternalUTXOManager,
	config *repositoryConfig.RepositoryOptions,
	configProvider config.Provider,
) (*Manager, error) {
	if badgerStore == nil {
		return nil, fmt.Errorf("badger store 不能为空")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("hash manager 不能为空")
	}
	if transactionHashServiceClient == nil {
		return nil, fmt.Errorf("transaction hash service client 不能为空")
	}
	if blockHashServiceClient == nil {
		return nil, fmt.Errorf("block hash service client 不能为空")
	}

	// 初始化存储核心组件
	blockStorage := &BlockStorage{
		storage:                badgerStore,
		blockHashServiceClient: blockHashServiceClient,
		config:                 &config.Performance,
	}

	chainState := &ChainState{
		storage: badgerStore,
	}

	// 初始化索引管理器
	indexManager := index.NewIndexManager(badgerStore, logger, blockHashServiceClient)

	// 初始化交易服务
	txService := transaction.NewTransactionService(badgerStore, blockStorage, logger, transactionHashServiceClient, blockHashServiceClient)

	// 初始化资源服务
	resService := resource.NewResourceService(badgerStore, blockStorage, logger, transactionHashServiceClient, blockHashServiceClient)

	// 初始化UTXO服务
	utxoClient := utxo.NewUTXOService(utxoManager, badgerStore, logger)

	// 初始化Outbox管理器（使用配置参数）
	outboxManager := NewOutboxManagerWithConfig(badgerStore, logger, &config.Outbox)

	// 初始化性能监控器（使用配置参数）
	performanceMonitor := NewPerformanceMonitorWithConfig(&config.Performance)

	manager := &Manager{
		// 核心依赖
		logger:         logger,
		configProvider: configProvider,
		badgerStore:    badgerStore,
		memoryStore:    memoryStore,
		hashManager:    hashManager,

		// 存储核心组件
		blockStorage: blockStorage,
		chainState:   chainState,

		// 子模块服务
		indexManager:       indexManager,
		txService:          txService,
		resService:         resService,
		utxoClient:         utxoClient,
		outboxManager:      outboxManager,
		performanceMonitor: performanceMonitor,
		config:             config,
	}

	if logger != nil {
		logger.Debug("数据仓储管理器及所有子组件初始化完成")
	}

	return manager, nil
}

// ========== Outbox事件处理 ==========

// processOutboxEvents 处理outbox事件
func (m *Manager) processOutboxEvents(ctx context.Context) {
	processor := NewOutboxProcessorWithConfig(m.outboxManager, m.utxoClient, m.logger, &m.config.Outbox)

	if err := processor.ProcessEvents(ctx); err != nil && m.logger != nil {
		m.logger.Errorf("处理outbox事件失败: %v", err)
	}
}

// StartOutboxProcessor 启动outbox事件处理器（后台服务）
func (m *Manager) StartOutboxProcessor(ctx context.Context) {
	processor := NewOutboxProcessorWithConfig(m.outboxManager, m.utxoClient, m.logger, &m.config.Outbox)

	ticker := time.NewTicker(m.config.Outbox.ProcessorInterval) // 使用配置的处理器间隔
	defer ticker.Stop()

	if m.logger != nil {
		m.logger.Info("Outbox事件处理器已启动")
	}

	for {
		select {
		case <-ctx.Done():
			if m.logger != nil {
				m.logger.Info("Outbox事件处理器已停止")
			}
			return
		case <-ticker.C:
			if err := processor.ProcessEvents(ctx); err != nil && m.logger != nil {
				m.logger.Errorf("定期处理outbox事件失败: %v", err)
			}
		}
	}
}

// ============================================================================
//                              内部辅助方法
// ============================================================================

// storeBlockInTransaction 在事务中存储区块数据
func (m *Manager) storeBlockInTransaction(ctx context.Context, tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	// 1. 存储区块数据
	if err := m.blockStorage.StoreBlockInTransaction(ctx, tx, block); err != nil {
		return fmt.Errorf("存储区块数据失败: %w", err)
	}

	// 2. 更新链状态
	if err := m.chainState.UpdateHighestBlockInTransaction(ctx, tx, block, blockHash); err != nil {
		return fmt.Errorf("更新链状态失败: %w", err)
	}

	return nil
}

// ============================================================================
//                            🏗️ 区块数据操作实现
// ============================================================================

// StoreBlock 存储区块
//
// 🎯 **统一协调入口**：原子性存储区块并更新所有相关索引
//
// 协调流程：
// 1. 存储区块数据（单一数据源）
// 2. 更新区块链状态
// 3. 更新索引系统
// 4. 异步通知UTXO系统
func (m *Manager) StoreBlock(ctx context.Context, block *core.Block) error {
	if m.logger != nil {
		m.logger.Debugf("开始统一协调存储区块 - height: %d, txCount: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	// 链ID安全验证（存储前的最后防线）
	if err := m.validateBlockChainIdForStorage(block); err != nil {
		if m.logger != nil {
			m.logger.Errorf("拒绝存储区块 - 链ID验证失败: %v", err)
		}
		return fmt.Errorf("链ID验证失败: %w", err)
	}

	// 先计算区块哈希和所有交易哈希，避免重复计算
	startTime := time.Now()
	blockHash, err := m.blockStorage.computeBlockHashWithService(ctx, block)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}

	// 计算所有交易哈希（UTXO处理需要）
	var txHashes [][]byte
	for i, tx := range block.Body.Transactions {
		// 使用哈希管理器计算交易哈希
		txData, err := proto.Marshal(tx)
		if err != nil {
			return fmt.Errorf("序列化交易失败 (tx %d): %w", i, err)
		}
		txHash := m.hashManager.SHA256(txData)
		txHashes = append(txHashes, txHash)
	}
	hashTime := time.Since(startTime)

	// 在单个原子事务中完成所有存储操作
	indexStartTime := time.Now()
	err = m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 1. 存储区块数据
		if err := m.storeBlockInTransaction(ctx, tx, block, blockHash); err != nil {
			return fmt.Errorf("存储区块失败: %w", err)
		}

		// 2. 更新链状态（关键修复：添加链状态更新）
		if err := m.updateChainState(ctx, tx, block); err != nil {
			return fmt.Errorf("更新链状态失败: %w", err)
		}

		// 3. 更新区块索引
		if err := m.indexManager.UpdateBlockIndex(ctx, tx, block); err != nil {
			return fmt.Errorf("更新区块索引失败: %w", err)
		}

		// 4. 更新交易索引
		if err := m.txService.IndexTransactions(ctx, tx, blockHash, block); err != nil {
			return fmt.Errorf("更新交易索引失败: %w", err)
		}

		// 5. 更新资源元数据索引
		if err := m.resService.IndexResourceMetadata(ctx, tx, blockHash, block); err != nil {
			return fmt.Errorf("更新资源索引失败: %w", err)
		}

		// 6. 处理UTXO变更（关键添加：UTXO创建和消费处理）
		if m.utxoClient != nil {
			// UTXOService需要添加ProcessBlockUTXOs方法来代理调用
			if err := m.utxoClient.ProcessBlockUTXOs(ctx, tx, block, blockHash, txHashes); err != nil {
				return fmt.Errorf("处理UTXO变更失败: %w", err)
			}
		} else {
			if m.logger != nil {
				m.logger.Warn("UTXO服务不可用，跳过UTXO处理")
			}
		}

		// 7. 添加UTXO更新事件到outbox（保证原子性）
		if err := m.outboxManager.AddBlockAddedEvent(tx, block, blockHash); err != nil {
			return fmt.Errorf("添加UTXO更新事件失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("原子性存储区块失败: %w", err)
	}

	indexTime := time.Since(indexStartTime)
	totalTime := time.Since(startTime)

	// 6. 触发区块备份检查（每10个区块自动备份）
	m.triggerBlockBackup(ctx, block)

	// 7. 记录性能指标
	metrics := &PerformanceMetrics{
		BlockHeight:         block.Header.Height,
		BlockProcessingTime: totalTime,
		TransactionCount:    len(block.Body.Transactions),
		IndexUpdateTime:     indexTime,
		HashCalculationTime: hashTime,
		StorageWriteTime:    indexTime, // 索引更新包含了存储写入时间
	}
	m.performanceMonitor.RecordMetrics(metrics)

	// 7. 触发outbox事件处理（异步，可靠性由outbox保证）
	go m.processOutboxEvents(ctx)

	// 8. 记录性能日志
	if m.logger != nil {
		m.logger.Debugf("区块存储性能指标 - height: %d, 总时间: %v, 哈希计算: %v, 索引更新: %v, 交易数: %d",
			block.Header.Height, totalTime, hashTime, indexTime, len(block.Body.Transactions))
	}

	return nil
}

// GetBlock 获取指定哈希的区块
func (m *Manager) GetBlock(ctx context.Context, blockHash []byte) (*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("获取区块 - blockHash: %x", blockHash)
	}
	// 调用具体实现方法 (block.go)
	return m.getBlock(ctx, blockHash)
}

// GetBlockByHeight 按高度获取区块
func (m *Manager) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("按高度获取区块 - height: %d", height)
	}
	// 调用具体实现方法 (block.go)
	return m.getBlockByHeight(ctx, height)
}

// GetBlockRange 获取区块高度范围
func (m *Manager) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("获取区块范围 - startHeight: %d, endHeight: %d", startHeight, endHeight)
	}
	// 调用具体实现方法 (block.go)
	return m.getBlockRange(ctx, startHeight, endHeight)
}

// GetHighestBlock 获取最高区块信息
func (m *Manager) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	if m.logger != nil {
		m.logger.Debug("获取最高区块信息 - method: GetHighestBlock")
	}
	// 调用具体实现方法 (block.go)
	return m.getHighestBlock(ctx)
}

// GetChainState 获取区块链状态信息
//
// 🎯 **链状态查询入口**：获取完整的区块链状态信息
// 包括最高区块、统计信息、创世区块信息等
func (m *Manager) GetChainState(ctx context.Context) (*ChainStateInfo, error) {
	if m.logger != nil {
		m.logger.Debug("获取区块链状态信息 - method: GetChainState")
	}
	// 调用具体实现方法 (chain.go)
	return m.getChainState(ctx)
}

// ValidateChainConsistency 验证区块链状态一致性
//
// 🎯 **链状态健康检查**：验证区块链状态的一致性
// 用于系统健康检查和故障诊断
func (m *Manager) ValidateChainConsistency(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("验证区块链状态一致性 - method: ValidateChainConsistency")
	}
	// 调用具体实现方法 (chain.go)
	return m.validateChainConsistency(ctx)
}

// ValidateFullConsistency 验证完整系统一致性
//
// 🎯 **系统定位**：全面一致性检查核心
// 验证区块、索引、UTXO状态的一致性
func (m *Manager) ValidateFullConsistency(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Info("开始全面一致性检查")
	}

	// 1. 验证区块链状态一致性
	if err := m.validateChainConsistency(ctx); err != nil {
		return fmt.Errorf("区块链状态一致性验证失败: %w", err)
	}

	// 2. 验证索引一致性
	if err := m.indexManager.ValidateIndexConsistency(ctx); err != nil {
		return fmt.Errorf("索引一致性验证失败: %w", err)
	}

	// 3. 验证区块与索引的一致性
	if err := m.validateBlockIndexConsistency(ctx); err != nil {
		return fmt.Errorf("区块-索引一致性验证失败: %w", err)
	}

	// 4. 验证outbox事件一致性
	if err := m.validateOutboxConsistency(ctx); err != nil {
		return fmt.Errorf("Outbox事件一致性验证失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("全面一致性检查通过")
	}

	return nil
}

// RepairChainState 修复区块链状态
//
// 🎯 **链状态修复入口**：从区块数据重建状态信息
// 用于故障恢复和数据修复
func (m *Manager) RepairChainState(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("修复区块链状态 - method: RepairChainState")
	}
	// 调用具体实现方法 (chain.go)
	return m.repairChainState(ctx)
}

// ============================================================================
//                           💰 交易权利管理实现
// ============================================================================

// GetTransaction 根据交易哈希获取完整交易及其位置信息
func (m *Manager) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *transactionpb.Transaction, err error) {
	if m.logger != nil {
		m.logger.Debugf("获取交易 - txHash: %x", txHash)
	}

	// 调用交易服务获取交易详情
	detail, err := m.txService.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("获取交易失败: %w", err)
	}

	return detail.BlockHash, detail.TxIndex, detail.Transaction, nil
}

// GetAccountNonce 获取账户当前nonce
func (m *Manager) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	if m.logger != nil {
		m.logger.Debugf("获取账户nonce - address: %x", address)
	}

	// 注意：账户nonce需要通过分析所有交易来计算
	// 这是一个复杂的操作，需要遍历地址相关的所有交易
	// 当前返回基础实现，生产环境应该维护nonce缓存

	if m.logger != nil {
		m.logger.Warnf("账户nonce查询需要完整的地址索引支持，当前返回0")
	}

	return 0, nil
}

// GetTransactionsByBlock 获取区块中的所有交易
func (m *Manager) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transactionpb.Transaction, error) {
	if m.logger != nil {
		m.logger.Debugf("获取区块交易 - blockHash: %x", blockHash)
	}

	// 调用交易服务获取区块中的所有交易
	details, err := m.txService.GetTransactionsByBlockHash(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("获取区块交易失败: %w", err)
	}

	// 提取交易对象
	transactions := make([]*transactionpb.Transaction, len(details))
	for i, detail := range details {
		transactions[i] = detail.Transaction
	}

	return transactions, nil
}

// ============================================================================
//                           ⚙️ 资源能力管理实现
// ============================================================================

// GetResourceByContentHash 根据内容哈希查询完整资源
func (m *Manager) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*resourcepb.Resource, error) {
	if m.logger != nil {
		m.logger.Debugf("获取资源 - contentHash: %x", contentHash)
	}

	// 调用资源服务获取资源元数据
	detail, err := m.resService.GetResourceMetadata(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("获取资源失败: %w", err)
	}

	return detail.Resource, nil
}

// ============================================================================
//                           📊 性能监控接口
// ============================================================================

// GetPerformanceMetrics 获取平均性能指标
func (m *Manager) GetPerformanceMetrics() *PerformanceMetrics {
	return m.performanceMonitor.GetAverageMetrics()
}

// GetRecentPerformanceMetrics 获取最近N个区块的性能指标
func (m *Manager) GetRecentPerformanceMetrics(count int) []*PerformanceMetrics {
	metrics := m.performanceMonitor.recentMetrics
	if len(metrics) <= count {
		return metrics
	}
	return metrics[len(metrics)-count:]
}

// RunProductionValidation 运行生产级验证
func (m *Manager) RunProductionValidation(ctx context.Context) error {
	validationSuite := NewValidationSuite(m, m.logger)
	return validationSuite.RunFullValidation(ctx)
}

// ============================================================================
//                           ✅ 一致性检查扩展
// ============================================================================

// validateBlockIndexConsistency 验证区块与索引的一致性
func (m *Manager) validateBlockIndexConsistency(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("验证区块-索引一致性")
	}

	// 获取最高区块
	height, blockHash, err := m.getHighestBlock(ctx)
	if err != nil {
		return fmt.Errorf("获取最高区块失败: %w", err)
	}

	if height == 0 && blockHash == nil {
		return nil // 空链，无需验证
	}

	// 验证前N个区块的一致性（避免性能问题）
	checkCount := uint64(m.config.Performance.ConsistencyCheckRange)
	if height < checkCount {
		checkCount = height + 1
	}

	startHeight := height - checkCount + 1
	if height < checkCount {
		startHeight = 0
	}

	for h := startHeight; h <= height; h++ {
		// 1. 通过高度获取区块哈希
		indexHash, err := m.indexManager.GetBlockHashByHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("从索引获取区块哈希失败 - height: %d, error: %w", h, err)
		}

		// 2. 通过高度获取完整区块
		block, err := m.getBlockByHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("获取区块失败 - height: %d, error: %w", h, err)
		}

		// 3. 计算区块的实际哈希
		actualHash, err := m.blockStorage.computeBlockHashWithService(ctx, block)
		if err != nil {
			return fmt.Errorf("计算区块哈希失败 - height: %d, error: %w", h, err)
		}

		// 4. 验证哈希一致性
		if !equalBytes(indexHash, actualHash) {
			return fmt.Errorf("区块哈希不一致 - height: %d, index: %x, actual: %x",
				h, indexHash, actualHash)
		}

		// 5. 验证区块在哈希索引中存在
		exists, err := m.indexManager.HasBlockHash(ctx, actualHash)
		if err != nil {
			return fmt.Errorf("检查哈希索引失败 - height: %d, error: %w", h, err)
		}
		if !exists {
			return fmt.Errorf("区块在哈希索引中不存在 - height: %d, hash: %x", h, actualHash)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("区块-索引一致性验证通过 - 检查了%d个区块", checkCount)
	}

	return nil
}

// validateOutboxConsistency 验证outbox事件一致性
func (m *Manager) validateOutboxConsistency(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("验证Outbox事件一致性")
	}

	// 获取待处理事件
	events, err := m.outboxManager.GetPendingEvents(ctx)
	if err != nil {
		return fmt.Errorf("获取待处理事件失败: %w", err)
	}

	// 检查是否有长期未处理的事件
	now := time.Now()
	for _, event := range events {
		age := now.Sub(event.CreatedAt)
		if age > time.Hour*24 { // 超过24小时的事件
			if m.logger != nil {
				m.logger.Warnf("发现长期未处理的outbox事件 - eventID: %s, age: %v", event.ID, age)
			}
		}

		// 检查失败次数过多的事件
		if event.Attempts >= 3 {
			if m.logger != nil {
				m.logger.Warnf("发现多次失败的outbox事件 - eventID: %s, attempts: %d, lastError: %s",
					event.ID, event.Attempts, event.LastError)
			}
		}
	}

	if m.logger != nil {
		m.logger.Debugf("Outbox事件一致性验证完成 - 待处理事件: %d", len(events))
	}

	return nil
}

// ========== 辅助函数 ==========

// equalBytes 比较两个字节数组是否相等
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// validateBlockChainIdForStorage 验证区块存储前的链ID
func (m *Manager) validateBlockChainIdForStorage(block *core.Block) error {
	if block == nil || block.Header == nil {
		return fmt.Errorf("区块或区块头为空")
	}

	// 🔧 修复：从配置提供者的区块链配置获取期望的链ID
	expectedChainId := uint64(1) // 安全默认值

	if m.configProvider != nil {
		if blockchainConfig := m.configProvider.GetBlockchain(); blockchainConfig != nil {
			expectedChainId = blockchainConfig.ChainID
			if m.logger != nil {
				m.logger.Debugf("✅ 从区块链配置获取期望链ID: %d", expectedChainId)
			}
		} else if m.logger != nil {
			m.logger.Warnf("⚠️  无法获取区块链配置，使用默认链ID: %d", expectedChainId)
		}
	} else if m.logger != nil {
		m.logger.Warnf("⚠️  配置提供者未初始化，使用默认链ID: %d", expectedChainId)
	}

	if block.Header.ChainId != expectedChainId {
		if m.logger != nil {
			m.logger.Errorf("❌ 区块存储链ID验证失败: 期望=%d, 实际=%d, 区块高度=%d",
				expectedChainId, block.Header.ChainId, block.Header.Height)
		}
		return fmt.Errorf("链ID不匹配，期望: %d, 实际: %d（拒绝存储错误链的区块）", expectedChainId, block.Header.ChainId)
	}

	if m.logger != nil {
		m.logger.Debugf("✅ 区块存储链ID验证通过: %d (高度: %d)", block.Header.ChainId, block.Header.Height)
	}

	if m.logger != nil {
		m.logger.Debugf("区块存储链ID验证通过: %d, 高度: %d", block.Header.ChainId, block.Header.Height)
	}

	return nil
}

// triggerBlockBackup 触发区块备份检查
// 每10个区块创建一次自动备份，确保数据安全
func (m *Manager) triggerBlockBackup(ctx context.Context, block *core.Block) {
	if m.logger != nil {
		m.logger.Debugf("检查区块高度 %d 是否需要触发备份", block.Header.Height)
	}

	// 每10个区块触发一次备份
	if block.Header.Height%10 == 0 && block.Header.Height > 0 {
		if m.logger != nil {
			m.logger.Infof("🔄 触发区块高度 %d 的自动备份", block.Header.Height)
		}

		// 异步执行备份，避免阻塞区块处理
		go func() {
			backupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// 创建备份文件路径
			timestamp := time.Now().Format("20060102_150405")
			backupName := fmt.Sprintf("badger_backup_%s_height_%d_triggered.bak", timestamp, block.Header.Height)

			// 获取数据目录
			var backupDir string
			if m.configProvider != nil {
				badgerConfig := m.configProvider.GetBadger()
				if badgerConfig != nil && badgerConfig.Path != "" {
					backupDir = fmt.Sprintf("%s/backups", badgerConfig.Path)
				}
			}

			// 使用默认备份路径
			if backupDir == "" {
				backupDir = "./data/development/single/badger/backups"
			}

			backupPath := fmt.Sprintf("%s/%s", backupDir, backupName)

			// 执行备份
			// 注意：BadgerStore接口没有CreateBackup方法，需要转换为具体实现
			if concreteStore, ok := m.badgerStore.(interface {
				CreateBackup(context.Context, string) error
			}); ok {
				if err := concreteStore.CreateBackup(backupCtx, backupPath); err != nil {
					if m.logger != nil {
						m.logger.Errorf("区块触发备份失败 (height: %d): %v", block.Header.Height, err)
					}
				} else {
					if m.logger != nil {
						m.logger.Infof("✅ 区块触发备份成功 (height: %d): %s", block.Header.Height, backupPath)
					}
				}
			} else {
				if m.logger != nil {
					m.logger.Warnf("BadgerStore不支持CreateBackup方法，跳过区块触发备份 (height: %d)", block.Header.Height)
				}
			}
		}()
	}
}
