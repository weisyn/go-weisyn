// Package pow_handler 实现PoW计算处理器服务
//
// 🎯 **PoW计算处理器模块**
//
// 本包实现 PoWComputeHandler 接口，提供完整的PoW计算技术实现：
// - 多线程并行PoW计算和nonce搜索
// - 高性能哈希计算优化（对象池、SIMD）
// - 从候选模板到完整区块的生成流程
// - PoW引擎生命周期管理和参数配置
// - 实时性能监控和算力统计
package pow_handler

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// PoWComputeService PoW计算服务实现
type PoWComputeService struct {
	// ========== 核心依赖 ==========
	logger            log.Logger                                   // 日志记录器
	powEngine         crypto.POWEngine                             // PoW引擎（用于底层计算验证）
	hashManager       crypto.HashManager                           // 哈希管理器（用于区块头哈希计算）
	merkleTreeManager crypto.MerkleTreeManager                     // Merkle树管理器（用于Merkle根计算和验证）
	txHashClient      transaction.TransactionHashServiceClient     // 交易哈希服务客户端（统一交易哈希计算）

	// ========== 轻量级状态管理 ==========
	mu        sync.RWMutex           // 读写锁保护状态
	isRunning bool                   // 引擎运行状态
	params    types.MiningParameters // 当前挖矿参数

	// 注意：移除了以下过度复杂的组件：
	// - 工作器池系统（直接使用 POWEngine 内部并行处理）
	// - 任务队列系统（不需要手动任务分发）
	// - 复杂性能监控（POWEngine 内部处理）
	// - 哈希池优化（违反项目约束）
}

// PoWTask PoW计算任务
type PoWTask struct {
	TaskID     string            // 任务ID
	Header     *core.BlockHeader // 区块头
	Target     *big.Int          // 目标难度
	StartNonce uint64            // 起始nonce
	EndNonce   uint64            // 结束nonce
	WorkerID   int               // 工作器ID
}

// PoWResult PoW计算结果
type PoWResult struct {
	TaskID   string            // 任务ID
	Success  bool              // 是否成功
	Header   *core.BlockHeader // 计算结果区块头
	Nonce    uint64            // 找到的nonce
	Hash     []byte            // 计算得到的哈希
	Attempts uint64            // 尝试次数
	Duration time.Duration     // 计算耗时
	WorkerID int               // 工作器ID
	Error    error             // 错误信息
}

// PoWWorker PoW工作器（注意：整个工作器池系统应该被移除，因为直接使用POWEngine更简单）
type PoWWorker struct {
	ID         int               // 工作器ID
	TaskChan   <-chan *PoWTask   // 任务接收通道
	ResultChan chan<- *PoWResult // 结果发送通道
	StopChan   <-chan struct{}   // 停止信号通道
	Logger     log.Logger        // 日志记录器
}

// 注意：移除了 PerformanceMonitor 结构体，因为：
// 1. 过度复杂的性能统计不符合 MVP 设计原则
// 2. 实际挖矿性能监控应由 POWEngine 内部处理
// 3. 上层组件不需要这些详细的算力统计

// 注意：移除了 HashPool 结构体，因为：
// 1. 违反了项目哈希服务约束 [[memory:8488830]]
// 2. 实际挖矿完全依赖 powEngine，不使用此池
// 3. 过度工程化，124行代码实现零价值功能

// NewPoWComputeService 创建PoW计算服务实例
func NewPoWComputeService(
	logger log.Logger,
	powEngine crypto.POWEngine,
	hashManager crypto.HashManager,
	merkleTreeManager crypto.MerkleTreeManager,
	txHashClient transaction.TransactionHashServiceClient,
) interfaces.PoWComputeHandler {
	service := &PoWComputeService{
		// 核心依赖 - 直接使用注入的服务，符合项目约束
		logger:            logger,
		powEngine:         powEngine,
		hashManager:       hashManager,
		merkleTreeManager: merkleTreeManager,
		txHashClient:      txHashClient,
		// 轻量级状态管理
		isRunning: false,
	}

	return service
}

// 编译时确保 PoWComputeService 实现了 PoWComputeHandler 接口
var _ interfaces.PoWComputeHandler = (*PoWComputeService)(nil)

// ========== 接口方法实现（薄实现，委托给具体方法文件） ==========

// MineBlockHeader 挖矿区块头 - 委托给 mine_block_header.go
func (s *PoWComputeService) MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	return s.mineBlockHeader(ctx, header)
}

// VerifyBlockHeader 验证区块头PoW - 委托给 mine_block_header.go
func (s *PoWComputeService) VerifyBlockHeader(header *core.BlockHeader) (bool, error) {
	return s.verifyBlockHeader(header)
}

// ProduceBlockFromTemplate 从模板生成区块 - 委托给 produce_block.go
func (s *PoWComputeService) ProduceBlockFromTemplate(ctx context.Context, candidateBlock interface{}) (interface{}, error) {
	return s.produceBlockFromTemplate(ctx, candidateBlock)
}

// StartPoWEngine 启动PoW引擎 - 委托给 start_engine.go
func (s *PoWComputeService) StartPoWEngine(ctx context.Context, params types.MiningParameters) error {
	return s.startPoWEngine(ctx, params)
}

// StopPoWEngine 停止PoW引擎 - 委托给 stop_engine.go
func (s *PoWComputeService) StopPoWEngine(ctx context.Context) error {
	return s.stopPoWEngine(ctx)
}

// ========== 辅助方法（供上层组件使用） ==========

// IsRunning 检查PoW引擎是否在运行状态
func (s *PoWComputeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetMiningParams 获取当前挖矿参数
func (s *PoWComputeService) GetMiningParams() types.MiningParameters {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.params
}

// 注意：移除了复杂的性能监控方法
// GetHashRate 和 GetTotalHashes，因为：
// 1. 实际算力统计应由 POWEngine 内部处理
// 2. 上层组件不需要这些详细指标
// 3. 符合接口轻量化原则

// ========== 注意：移除了复杂的辅助构造函数 ==========
// NewPerformanceMonitor 已移除，因为不再需要复杂的性能监控系统

// 注意：移除了所有 HashPool 相关方法，包括：
// - NewHashPool() *HashPool
// - GetHash() []byte
// - PutHash(hash []byte)
// - GetBatchBuffers() [][]byte
// - PutBatchBuffers(buffers [][]byte)
// - GetPrecomputeBuffer() []byte
// - PutPrecomputeBuffer(buffer []byte)
// - GetPoolStats() map[string]interface{}
// - ResetStats()
//
// 移除原因：124行无用代码，违反项目约束，实际挖矿不使用

// ========== 注意：移除了性能监控器的所有方法 ==========
// 移除的方法包括：
// - UpdateHashCount(count uint64)
// - GetCurrentHashRate() float64
// - GetPeakHashRate() float64
// - GetTotalHashes() uint64
// - GetUptime() time.Duration
// - Reset()
// - GetStatistics() map[string]interface{}
//
// 移除原因：过度复杂的性能统计系统，不符合 MVP 原则

// ========== 注意：移除了高级性能监控方法 ==========
// 移除的方法包括：
// - GetPerformanceReport() map[string]interface{}
// - PublishPerformanceMetrics()
// - StartPerformanceReporting(ctx context.Context, interval time.Duration)
//
// 移除原因：
// 1. 过度复杂的性能报告系统不符合项目约束
// 2. 实际挖矿不需要这些详细的统计信息
// 3. POWEngine 内部已处理相关逻辑
