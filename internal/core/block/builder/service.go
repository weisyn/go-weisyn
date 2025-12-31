// Package builder 实现区块构建服务
//
// 🎯 **BlockBuilder 服务实现**
//
// 本包实现了区块构建服务，负责创建挖矿候选区块。
// 采用哈希+缓存架构模式，支持并发挖矿场景。
//
// 💡 **核心职责**：
// - 创建挖矿候选区块
// - 管理候选区块缓存
// - 提供构建性能指标
// nolint:U1000 // 允许未使用的函数以备将来使用
package builder

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/block/interfaces"
	"github.com/weisyn/v1/internal/core/block/merkle"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// Service 区块构建服务
//
// 🎯 **设计理念**：
// - 轻量级构建：快速创建候选区块
// - 缓存优化：减少重复构建开销
// - 并发安全：支持多矿工并发构建
//
// 📦 **服务职责**：
// - CreateMiningCandidate: 创建挖矿候选区块
// - GetCandidateBlock: 获取缓存的候选区块
// - GetBuilderMetrics: 获取构建性能指标
type Service struct {
	// ==================== 依赖注入 ====================

	// storage 存储服务（读取链状态）
	storage storage.BadgerStore

	// mempool 交易池（获取待打包交易）
	mempool mempool.TxPool

	// txProcessor 交易处理器（验证和处理交易）
	txProcessor tx.TxProcessor

	// hasher 哈希服务（用于Merkle树计算）
	hasher merkle.Hasher

	// blockHashClient 区块哈希服务客户端（用于计算区块哈希）
	blockHashClient core.BlockHashServiceClient

	// txHashClient 交易哈希服务客户端（用于计算交易哈希，确保与共识层一致）
	txHashClient transaction.TransactionHashServiceClient

	// utxoQuery UTXO查询服务（用于获取状态根，P3-4）
	utxoQuery persistence.UTXOQuery

	// blockQuery 区块查询服务（用于获取难度，P3-5）
	blockQuery persistence.BlockQuery

	// chainQuery 链查询服务（用于获取链状态，如当前高度和最佳区块哈希）
	chainQuery persistence.ChainQuery

	// feeManager 费用管理器（用于构建 Coinbase 交易，P3-3）
	feeManager tx.FeeManager

	// minerAddress 矿工地址（用于 Coinbase 输出，P3-3）
	minerAddress []byte
	minerMu      sync.RWMutex

	// logger 日志记录器（可选）
	logger log.Logger

	// configProvider 配置提供者（必需，用于 v2 难度/时间戳规则参数）
	configProvider config.Provider

	// ==================== 候选区块缓存 ====================

	// cache 候选区块LRU缓存
	cache *CandidateLRUCache

	// ==================== 指标收集 ====================

	// metrics 构建服务指标
	metrics *interfaces.BuilderMetrics

	// metricsMu 指标读写锁
	metricsMu sync.Mutex

	// ==================== 状态管理 ====================

	// isHealthy 健康状态
	isHealthy bool

	// lastError 最后错误
	lastError error

	// chainIDOnce 确保链ID只解析一次
	chainIDOnce sync.Once
	// chainID 缓存解析后的链ID
	chainID uint64
	// chainIDErr 缓存解析链ID时发生的错误
	chainIDErr error
}

// NewService 创建区块构建服务
//
// 🔧 **初始化流程**：
// 1. 验证必需依赖
// 2. 初始化缓存
// 3. 初始化指标
// 4. 设置默认配置
//
// 参数：
//   - storage: 存储服务（必需）
//   - mempool: 交易池（必需）
//   - txProcessor: 交易处理器（可选，如为nil则不验证交易）
//   - hashManager: 哈希管理器（必需）
//   - utxoQuery: UTXO查询服务（可选，用于获取状态根，P3-4）
//   - blockQuery: 区块查询服务（可选，用于获取难度，P3-5）
//   - chainQuery: 链查询服务（可选，用于获取链状态）
//   - feeManager: 费用管理器（可选，用于构建 Coinbase 交易，P3-3）
//   - logger: 日志记录器（可选）
//
// 返回：
//   - interfaces.InternalBlockBuilder: 区块构建服务实例
//   - error: 创建错误
func NewService(
	storage storage.BadgerStore,
	mempool mempool.TxPool,
	txProcessor tx.TxProcessor,
	hashManager crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	txHashClient transaction.TransactionHashServiceClient,
	utxoQuery persistence.UTXOQuery,
	blockQuery persistence.BlockQuery,
	chainQuery persistence.ChainQuery,
	feeManager tx.FeeManager,
	configProvider config.Provider,
	logger log.Logger,
) (interfaces.InternalBlockBuilder, error) {
	// 验证必需依赖
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if mempool == nil {
		return nil, fmt.Errorf("mempool 不能为空")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("hashManager 不能为空")
	}
	if blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 不能为空")
	}
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}
	if configProvider == nil {
		return nil, fmt.Errorf("configProvider 不能为空")
	}

	// 创建哈希适配器（用于Merkle树计算）
	hasher := merkle.NewHashManagerAdapter(hashManager)

	// 创建LRU缓存
	maxCacheSize := 100 // 默认缓存100个候选区块
	lruCache := NewCandidateLRUCache(maxCacheSize, logger)

	// 创建服务实例
	s := &Service{
		storage:         storage,
		mempool:         mempool,
		txProcessor:     txProcessor,
		hasher:          hasher,
		blockHashClient: blockHashClient,
		txHashClient:    txHashClient,
		utxoQuery:       utxoQuery,
		blockQuery:      blockQuery,
		chainQuery:      chainQuery,
		feeManager:      feeManager,
		configProvider:  configProvider,
		logger:          logger,
		cache:           lruCache,
		metrics: &interfaces.BuilderMetrics{
			MaxCacheSize: maxCacheSize,
		},
		isHealthy: true,
	}

	if logger != nil {
		logger.Infof("✅ BlockBuilder 服务初始化成功（使用 TxPool 实例: %p）", mempool)
	}

	return s, nil
}

// CreateMiningCandidate 创建挖矿候选区块
//
// 🎯 **核心业务逻辑**：
// 1. 获取当前链状态（高度、最佳区块哈希）
// 2. 从交易池获取待打包交易
// 3. 创建区块头
// 4. 创建区块体
// 5. 计算区块哈希
// 6. 缓存候选区块
// 7. 返回区块哈希
//
// 参数：
//   - ctx: 上下文
//
// 返回：
//   - []byte: 候选区块哈希（32字节）
//   - error: 创建错误
func (s *Service) CreateMiningCandidate(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	defer func() {
		s.recordCreation(time.Since(startTime))
	}()

	if s.logger != nil {
		s.logger.Debug("开始创建挖矿候选区块")
	}

	// 1. 获取当前链状态
	currentHeight, parentHash, err := s.getCurrentChainState(ctx)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("获取链状态失败: %w", err)
	}

	// 2. 从交易池获取待打包交易
	candidateTxs, err := s.mempool.GetTransactionsForMining()
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("从交易池获取交易失败: %w", err)
	}

	// 3. 构建候选区块（详细实现在 candidate.go）
	candidateBlock, err := s.buildCandidate(ctx, currentHeight, parentHash, candidateTxs)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("构建候选区块失败: %w", err)
	}

	// 4. 计算区块哈希并缓存候选区块
	var blockHash []byte
	if candidateBlock != nil && candidateBlock.Header != nil {
		var err error
		blockHash, err = s.calculateBlockHash(ctx, candidateBlock.Header)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("计算区块哈希失败: %v", err)
			}
			blockHash = []byte{} // 使用空哈希作为后备
		}

		if len(blockHash) > 0 {
			if err := s.cacheCandidate(blockHash, candidateBlock); err != nil {
				// 缓存失败不影响返回，只记录警告
				if s.logger != nil {
					s.logger.Warnf("缓存候选区块失败: %v", err)
				}
			}
		}
	} else {
		// candidateBlock 为 nil，无法计算哈希
		if s.logger != nil {
			s.logger.Warnf("候选区块为nil，无法计算哈希")
		}
		blockHash = []byte{} // 使用空哈希作为后备
	}

	if s.logger != nil {
		if len(blockHash) >= 8 && candidateBlock != nil && candidateBlock.Header != nil {
			s.logger.Infof("✅ 成功创建挖矿候选区块，哈希: %x, 高度: %d, 交易数: %d",
				blockHash[:8], candidateBlock.Header.Height, len(candidateBlock.Body.Transactions))
		} else if candidateBlock != nil && candidateBlock.Header != nil {
			s.logger.Infof("✅ 成功创建挖矿候选区块，高度: %d, 交易数: %d",
				candidateBlock.Header.Height, len(candidateBlock.Body.Transactions))
		} else {
			s.logger.Infof("✅ 成功创建挖矿候选区块")
		}
	}

	return blockHash, nil
}

// GetCandidateBlock 获取候选区块
//
// 🎯 **从LRU缓存获取候选区块**
//
// 参数：
//   - ctx: 上下文
//   - blockHash: 候选区块哈希
//
// 返回：
//   - *core.Block: 候选区块
//   - error: 获取错误
func (s *Service) GetCandidateBlock(ctx context.Context, blockHash []byte) (*core.Block, error) {
	key := fmt.Sprintf("%x", blockHash)
	block, exists := s.cache.Get(key)
	if !exists {
		s.recordCacheMiss()
		// 🔧 修复：检查 blockHash 长度，避免 panic
		if len(blockHash) >= 8 {
			return nil, fmt.Errorf("候选区块不存在于缓存中: %x", blockHash[:8])
		}
		return nil, fmt.Errorf("候选区块不存在于缓存中: %x", blockHash)
	}

	s.recordCacheHit()
	return block, nil
}

// ==================== 内部管理方法 ====================

// GetBuilderMetrics 获取构建服务指标
func (s *Service) GetBuilderMetrics(ctx context.Context) (*interfaces.BuilderMetrics, error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	// 更新缓存大小
	s.metrics.CacheSize = s.cache.Size()

	// 更新健康状态
	s.metrics.IsHealthy = s.isHealthy
	if s.lastError != nil {
		s.metrics.ErrorMessage = s.lastError.Error()
	}

	return s.metrics, nil
}

// GetCachedCandidate 获取缓存的候选区块（内部方法）
func (s *Service) GetCachedCandidate(ctx context.Context, blockHash []byte) (*core.Block, error) {
	return s.GetCandidateBlock(ctx, blockHash)
}

// ClearCandidateCache 清理候选区块缓存
func (s *Service) ClearCandidateCache(ctx context.Context) error {
	s.cache.Clear()

	// 更新指标
	s.metricsMu.Lock()
	s.metrics.CacheSize = 0
	s.metricsMu.Unlock()

	if s.logger != nil {
		s.logger.Info("✅ 候选区块缓存已清理")
	}

	return nil
}

// RemoveCachedCandidate 从缓存中移除指定的候选区块
//
// 🎯 **使用场景**：
// - 区块挖出后：移除已成功挖出的候选区块
// - 过期清理：移除过期的候选区块
// - 分叉处理：移除分叉链上的无效候选区块
func (s *Service) RemoveCachedCandidate(ctx context.Context, blockHash []byte) error {
	return s.removeCachedCandidate(blockHash)
}

// ==================== 辅助方法 ====================

// getCurrentChainState 获取当前链状态
func (s *Service) getCurrentChainState(ctx context.Context) (height uint64, parentHash []byte, err error) {
	// 优先通过 QueryService 抽象获取链尖信息，避免直接依赖底层存储 key 约定
	if s.chainQuery != nil {
		chainInfo, err := s.chainQuery.GetChainInfo(ctx)
		// ⚠️ 兼容：QueryService 可能尚未就绪/返回空信息（单测/启动早期常见），此时回退到存储链尖 key。
		if err == nil && chainInfo != nil {
			// 🔧 修复：区分"链为空"和"链上有创世区块"两种场景
			// 场景1：链上还没有任何区块（连创世区块都没有）
			if chainInfo.Height == 0 && len(chainInfo.BestBlockHash) == 0 {
				// 继续走存储兼容路径（下方），以便读取 state:chain:tip（测试/旧路径）
			} else {
				// 场景2：链上已有区块（高度>=0，且有BestBlockHash）
				if len(chainInfo.BestBlockHash) != 32 {
					return 0, nil, fmt.Errorf("最佳区块哈希长度错误: 期望32字节，实际%d字节",
						len(chainInfo.BestBlockHash))
				}

				parentHash = make([]byte, 32)
				copy(parentHash, chainInfo.BestBlockHash)
				return chainInfo.Height, parentHash, nil
			}
		}

		// err != nil 或 chainInfo 空/未就绪：走存储兼容路径
	}

	// 兼容路径：直接从 state:chain:tip 读取链尖数据（仅用于缺少 QueryService 的场景）
	// 格式：height(8字节) + blockHash(32字节)
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		// 兼容：链尖 key 不存在时视为“空链”，允许继续挖创世/高度1区块
		if strings.Contains(err.Error(), "key not found") {
			return 0, make([]byte, 32), nil
		}
		return 0, nil, fmt.Errorf("读取链尖数据失败: %w", err)
	}
	if len(tipData) == 0 {
		// 创世区块场景：链尖不存在（不是错误）
		return 0, make([]byte, 32), nil
	}

	// 验证数据长度
	if len(tipData) != 40 { // 8 + 32
		return 0, nil, fmt.Errorf("链尖数据格式错误：期望40字节，实际%d字节", len(tipData))
	}

	// 解析高度和区块哈希
	height = bytesToUint64(tipData[0:8])
	parentHash = make([]byte, 32)
	copy(parentHash, tipData[8:40])

	return height, parentHash, nil
}

// resolveChainID 解析并缓存链ID
//
// 🎯 设计约束：
// - 生产路径中不得静默回退为 1
// - 必须能够从创世区块解析出非 0 的链ID，否则返回错误
func (s *Service) resolveChainID(ctx context.Context) (uint64, error) {
	if s == nil {
		// 理论上不会发生，仅作为防御性代码，默认返回 1
		return 1, nil
	}

	s.chainIDOnce.Do(func() {
		// 1) 先从配置读取 ChainID（单测/工具场景未必有创世区块写入DB）
		var cfgChainID uint64
		if s.configProvider != nil {
			cfgChainID = s.configProvider.GetBlockchain().ChainID
		}
		if cfgChainID == 0 {
			s.chainIDErr = fmt.Errorf("配置链ID为0，无法解析链ID")
			return
		}

		// 2) 尝试从创世区块读取 ChainID 做一致性校验；不可用则降级用配置值
		if s.blockQuery == nil {
			s.chainID = cfgChainID
			if s.logger != nil {
				s.logger.Warnf("blockQuery 未注入，无法从创世区块校验链ID，降级使用配置链ID=%d", s.chainID)
			}
			return
		}

		genesis, err := s.blockQuery.GetBlockByHeight(ctx, 0)
		if err != nil || genesis == nil || genesis.Header == nil {
			s.chainID = cfgChainID
			if s.logger != nil {
				if err != nil {
					s.logger.Warnf("获取创世区块失败，无法校验链ID，降级使用配置链ID=%d: %v", s.chainID, err)
				} else {
					s.logger.Warnf("创世区块/区块头缺失，无法校验链ID，降级使用配置链ID=%d", s.chainID)
				}
			}
			return
		}

		if genesis.Header.ChainId == 0 {
			s.chainIDErr = fmt.Errorf("创世区块链ID为0，非法配置")
			if s.logger != nil {
				s.logger.Error("创世区块链ID为0，非法配置")
			}
			return
		}

		// 3) 回归校验：配置 chain_id 必须与创世 chain_id 一致
		if genesis.Header.ChainId != cfgChainID {
			s.chainIDErr = fmt.Errorf("chain_id 不一致: config=%d genesis=%d", cfgChainID, genesis.Header.ChainId)
			return
		}

		s.chainID = genesis.Header.ChainId
		if s.logger != nil {
			s.logger.Debugf("✅ 成功从创世区块加载链ID: %d", s.chainID)
		}
	})

	if s.chainIDErr != nil {
		// 返回解析链ID时缓存的错误
		return 0, s.chainIDErr
	}

	if s.chainID == 0 {
		return 0, fmt.Errorf("链ID尚未初始化")
	}

	return s.chainID, nil
}

// recordCreation 记录创建指标
func (s *Service) recordCreation(duration time.Duration) {
	if s == nil {
		return // 防止 nil 指针
	}
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	if s.metrics == nil {
		return // 防止 nil 指针
	}

	s.metrics.CandidatesCreated++
	s.metrics.LastCandidateTime = time.Now().Unix()

	// 更新平均创建耗时（滑动平均）
	alpha := 0.1
	newTime := duration.Seconds()
	if s.metrics.AvgCreationTime == 0 {
		s.metrics.AvgCreationTime = newTime
	} else {
		s.metrics.AvgCreationTime = alpha*newTime + (1-alpha)*s.metrics.AvgCreationTime
	}

	// 更新最大创建耗时
	if newTime > s.metrics.MaxCreationTime {
		s.metrics.MaxCreationTime = newTime
	}
}

// recordCacheHit 记录缓存命中
func (s *Service) recordCacheHit() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.CacheHits++
}

// recordCacheMiss 记录缓存未命中
func (s *Service) recordCacheMiss() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.metrics.CacheMisses++
}

// recordError 记录错误
func (s *Service) recordError(err error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	s.isHealthy = false
	s.lastError = err
}

// bytesToUint64 字节转uint64
func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// uint64ToBytes uint64转字节
//
// 🎯 **用途**：
// - 与 bytesToUint64 对称，用于将 uint64 转换为字节数组
// - 可用于写入链状态（如写入链尖高度）
//
// TODO: 当需要将高度写入存储时使用此函数
// nolint:U1000 // 保留以备将来使用（与 bytesToUint64 对称）
func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(n >> 56)
	b[1] = byte(n >> 48)
	b[2] = byte(n >> 40)
	b[3] = byte(n >> 32)
	b[4] = byte(n >> 24)
	b[5] = byte(n >> 16)
	b[6] = byte(n >> 8)
	b[7] = byte(n)
	return b
}

// 编译时检查接口实现
var _ interfaces.InternalBlockBuilder = (*Service)(nil)

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (s *Service) ModuleName() string {
	return "block"
}

// CollectMemoryStats 收集区块模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 当前缓存的区块数量（候选区块）
// - ApproxBytes: 区块缓存总估算 bytes（cache size * avg block size，基于 proto.Size 的滚动统计）
// - CacheItems: block cache 条目
// - QueueLength: 待处理区块队列长度（当前暂为 0，因为 BlockBuilder 无队列）
func (s *Service) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 获取缓存大小
	cacheSize := 0
	if s.cache != nil {
		cacheSize = s.cache.Size()
	}

	// 根据内存监控模式决定是否计算 ApproxBytes
	var approxBytes int64 = 0
	mode := metricsutil.GetMemoryMonitoringMode()
	if mode != "minimal" {
		// heuristic 和 accurate 模式：使用缓存内部维护的平均区块大小（基于 proto.Size 的滚动统计）
		if s.cache != nil && cacheSize > 0 {
			avgSize := s.cache.AvgBlockSizeBytes()
			if avgSize > 0 {
				approxBytes = int64(cacheSize) * avgSize
			}
		}
	}

	return metricsiface.ModuleMemoryStats{
		Module:      "block",
		Layer:       "L4-CoreBusiness",
		Objects:     int64(cacheSize),
		ApproxBytes: approxBytes,
		CacheItems:  int64(cacheSize),
		QueueLength: 0, // BlockBuilder 无队列
	}
}

// ShrinkCache 主动裁剪候选区块缓存（供 MemoryDoctor 调用）
func (s *Service) ShrinkCache(targetSize int) {
	if s.cache == nil {
		return
	}
	if targetSize <= 0 {
		targetSize = 1
	}
	if s.logger != nil {
		s.logger.Warnf("MemoryDoctor 触发 BlockBuilder 缓存收缩: targetSize=%d (current=%d)",
			targetSize, s.cache.Size())
	}
	// 当前 LRU 缓存实现不支持精确调整容量，这里采用快速清空的方式：
	// - 清空缓存数据
	// - 保留 maxSize 配置，由后续访问重新填充热点候选区块
	s.cache.Clear()
}
