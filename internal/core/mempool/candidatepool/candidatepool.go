// 文件说明：
// 本文件实现候选区块池（CandidatePool）的核心存储与维护逻辑。
// 设计目标：
// 1）高内聚低耦合：专注候选区块的接收、存储、索引与清理；
// 2）分层验证：仅做基础安全验证（格式/哈希/大小/重复/内存）；高度等业务校验由上层负责；
// 3）事件下沉：对外仅暴露 CandidateEventSink 接口，由 integration 层桥接 EventBus；
// 4）线程安全：全局采用锁保护内部状态，支持并发访问；
// 5）内存可控：按字节跟踪 memoryUsage 与 memoryLimit，定期清理。
package candidatepool

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// eventTopics 定义候选区块池相关的事件主题（仅用于规范事件命名，不直接耦合实现）
const (
	TopicCandidateAdded   event.EventType = "candidate:added"
	TopicCandidateRemoved event.EventType = "candidate:removed"
	TopicCandidateExpired event.EventType = "candidate:expired"
	TopicPoolCleared      event.EventType = "pool:cleared"
	TopicPoolState        event.EventType = "pool:state"
)

// 错误定义
var (
	ErrCandidateAlreadyExists = fmt.Errorf("候选区块已存在")
	ErrCandidateNotFound      = fmt.Errorf("候选区块未找到")
	ErrPoolClosed             = fmt.Errorf("候选区块池已关闭")
	ErrInvalidBlock           = fmt.Errorf("无效的候选区块")
	ErrMemoryLimit            = fmt.Errorf("内存限制超出")
	ErrPoolFull               = fmt.Errorf("候选区块池已满")
	ErrTimeout                = fmt.Errorf("操作超时")
	ErrInvalidHeight          = fmt.Errorf("候选区块高度无效")
	ErrOutdatedBlock          = fmt.Errorf("候选区块已过时")
	ErrFutureBlock            = fmt.Errorf("候选区块来自未来")
)

// ChainStateProvider 链状态提供者接口（用于事件驱动架构）
// 说明：仅作为可选依赖，帮助进行某些维护性判断；不参与业务校验。
type ChainStateProvider interface {
	GetCurrentHeight(ctx context.Context) (uint64, error)
	GetLatestBlockHash(ctx context.Context) ([]byte, error)
	IsValidHeight(height uint64) bool
}

// CandidatePool 候选区块池实现
//
// 🎯 设计原则：
// - 高内聚低耦合：专注于候选区块存储，业务逻辑委托给注入的服务
// - 分层验证：基础安全验证防止网络攻击，业务逻辑验证由外部负责
// - 线程安全：支持并发访问和操作
// - 内存可控：严格的内存使用限制和清理机制
type CandidatePool struct {
	// ========== 分层验证组件 ==========
	basicValidator BasicCandidateValidator // 基础安全验证器（防网络攻击）

	// ========== 纯存储字段 ==========
	candidates         map[string]*types.CandidateBlock   // 候选区块哈希到区块的映射
	candidatesByHeight map[uint64][]*types.CandidateBlock // 按高度索引的候选区块
	pendingCandidates  map[string]struct{}                // 待验证候选区块
	verifiedCandidates map[string]struct{}                // 已验证候选区块
	expiredCandidates  map[string]struct{}                // 已过期候选区块

	// ========== 存储管理字段 ==========
	config      *candidatepool.CandidatePoolOptions // 候选区块池配置
	memory      storage.MemoryStore                 // 内存存储
	memoryLimit uint64                              // 内存使用限制(字节)
	memoryUsage uint64                              // 当前内存使用量(字节)

	// ========== 基础设施字段 ==========
	logger    log.Logger         // 日志记录器
	eventSink CandidateEventSink // 事件下沉
	mu        sync.RWMutex       // 同步锁
	quit      chan struct{}      // 关闭信号
	isRunning bool               // 运行状态

	// ========== 注入的基础服务 ==========
	hashService     core.BlockHashServiceClient // 区块哈希服务（来自crypto模块）
	chainStateCache ChainStateProvider          // 链状态缓存（用于事件驱动架构）

	// ========== 时间和统计字段 ==========
	startTime     time.Time // 启动时间
	lastCleanupAt time.Time // 最后清理时间

	// 性能统计
	totalAdded   uint64 // 总添加次数
	totalRemoved uint64 // 总移除次数

	// 错误统计
	validationErrors uint64 // 验证错误次数
	duplicateBlocks  uint64 // 重复区块次数
	memoryErrors     uint64 // 内存不足错误次数

	// ========== 等待通道 ==========
	waitChannels map[string]chan []*types.CandidateBlock // 等待候选区块的通道
}

// 已移除向后兼容构造器 NewCandidatePool，统一使用 NewCandidatePoolWithCache

// NewCandidatePoolWithCache 创建带链状态缓存的候选区块池（事件驱动版本）
// 参数：
// - config：候选池配置，包含内存上限、清理间隔、最大数量等；
// - logger：日志接口；
// - eventBus：事件总线（由integration层注入下沉实现，此处不直接耦合使用）；
// - memory：内存存储；
// - hashService：区块哈希服务客户端；
// - chainStateCache：可选链状态提供者，用于维护性判断。
// 返回：
// - interfaces.InternalCandidatePool：候选池内部接口实例；
// - error：初始化失败时返回错误。
func NewCandidatePoolWithCache(
	config *candidatepool.CandidatePoolOptions,
	logger log.Logger,
	eventBus event.EventBus,
	memory storage.MemoryStore,
	hashService core.BlockHashServiceClient,
	chainStateCache ChainStateProvider,
) (interfaces.InternalCandidatePool, error) {
	// 🔐 基础防御：配置不能为空（优先于任何使用 config 的逻辑）
	if config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}

	// 🔐 防御性修复：CleanupInterval 必须为正值，避免 time.NewTicker(0) 直接 panic
	if config.CleanupInterval <= 0 {
		// 兼容：历史测试/调用方可能未显式设置 CleanupInterval。
		// 在运行时依然需要一个安全默认值来避免 ticker panic。
		config.CleanupInterval = 1 * time.Minute
	}

	// 创建重复检测回调：根据候选池存储键（区块哈希字符串）判断是否存在
	duplicateExistsFn := func(hash []byte) bool {
		if len(hash) == 0 {
			return false
		}
		// 注意：此处只用于验证器，Pool 尚未构造完成，先用临时 map 逻辑由闭包在稍后绑定
		return false
	}

	// 创建基础验证器（先占位回调，稍后实例化 Pool 后再绑定真正的实现）
	basicValidator := NewBasicCandidateValidator(
		config,
		logger,
		duplicateExistsFn,
	)

	// 创建候选区块池
	pool := &CandidatePool{
		// ========== 分层验证组件 ==========
		basicValidator: basicValidator,

		// ========== 纯存储字段 ==========
		candidates:         make(map[string]*types.CandidateBlock),
		candidatesByHeight: make(map[uint64][]*types.CandidateBlock),
		pendingCandidates:  make(map[string]struct{}),
		verifiedCandidates: make(map[string]struct{}),
		expiredCandidates:  make(map[string]struct{}),

		// ========== 存储管理字段 ==========
		config:      config,
		memory:      memory,
		memoryLimit: config.MemoryLimit,
		memoryUsage: 0,

		// ========== 基础设施字段 ==========
		logger:          logger,
		quit:            make(chan struct{}),
		hashService:     hashService,
		chainStateCache: chainStateCache,

		// ========== 时间和统计字段 ==========
		startTime:     time.Now(),
		lastCleanupAt: time.Now(),

		// ========== 等待通道 ==========
		waitChannels: make(map[string]chan []*types.CandidateBlock),
	}

	// 现在 Pool 已创建，重绑定验证器中的 duplicateExistsFn
	if pv, ok := pool.basicValidator.(*ProductionBasicCandidateValidator); ok {
		pv.duplicateExistsFn = func(hash []byte) bool {
			_, exists := pool.candidates[string(hash)]
			return exists
		}
	}

	// 事件下沉默认 Noop，由 integration 层在 Fx 中注入真实实现
	pool.eventSink = NoopCandidateEventSink{}

	return pool, nil
}

// Start 启动候选区块池服务。
// 参数：无。
// 返回：
// - error：已在运行或其他错误时返回错误。
func (p *CandidatePool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return fmt.Errorf("候选区块池已在运行")
	}

	p.isRunning = true
	p.startTime = time.Now()

	// 启动维护协程
	go p.maintenanceLoop()

	if p.logger != nil {
		p.logger.Info("候选区块池已启动")
	}

	return nil
}

// Stop 停止候选区块池服务。
// 参数：无。
// 返回：
// - error：未运行或其他错误时返回错误。
func (p *CandidatePool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return fmt.Errorf("候选区块池未运行")
	}

	close(p.quit)
	p.isRunning = false

	// 关闭所有等待通道
	for _, ch := range p.waitChannels {
		close(ch)
	}
	p.waitChannels = make(map[string]chan []*types.CandidateBlock)

	if p.logger != nil {
		p.logger.Info("候选区块池已停止")
	}

	return nil
}

// IsRunning 检查候选区块池是否正在运行。
// 参数：无。
// 返回：
// - bool：true 表示运行中，false 表示未运行。
func (p *CandidatePool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}

// SetEventSink 注入候选池事件下沉实现。
// 参数：
// - sink：事件下沉实现（nil 时自动降级为 Noop）。
// 返回：无。
func (p *CandidatePool) SetEventSink(sink CandidateEventSink) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sink == nil {
		p.eventSink = NoopCandidateEventSink{}
		return
	}
	p.eventSink = sink
}

// maintenanceLoop 执行候选区块池维护任务（定时清理）。
// 参数：无。
// 返回：无。
func (p *CandidatePool) maintenanceLoop() {
	cleanupTicker := time.NewTicker(p.config.CleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-cleanupTicker.C:
			p.performMaintenance()
		case <-p.quit:
			return
		}
	}
}

// performMaintenance 执行维护任务：清理过期与过时候选，并发布清理完成事件。
// 参数：无。
// 返回：无。
func (p *CandidatePool) performMaintenance() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 使用新的综合清理机制
	totalRemoved := p.cleanExpiredCandidatesInternal()

	// 更新最后清理时间
	p.lastCleanupAt = time.Now()

	// 发布池状态事件
	if p.eventSink != nil && totalRemoved > 0 {
		p.eventSink.OnCleanupCompleted()

		if p.logger != nil {
			p.logger.Infof("维护清理完成，清理候选区块: %d个, 当前池大小: %d",
				totalRemoved, len(p.candidates))
		}
	}
}

// AddCandidate 添加单个候选区块。
// 参数：
// - block：待添加的候选区块；
// - fromPeer：来源节点ID（本地提交可为空字符串）。
// 返回：
// - []byte：计算得到的区块哈希；
// - error：出错时返回（例如重复、超限、格式/哈希/大小校验失败等）。
func (p *CandidatePool) AddCandidate(block *core.Block, fromPeer string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查候选区块池是否已关闭
	select {
	case <-p.quit:
		return nil, ErrPoolClosed
	default:
	}

	// 🛡️ 第1步：基础安全验证
	if err := p.basicValidator.ValidateFormat(block); err != nil {
		p.validationErrors++
		if p.logger != nil {
			p.logger.Warnf("候选区块格式验证失败: %v", err)
		}
		return nil, fmt.Errorf("格式验证失败: %v", err)
	}

	// 高度校验由上层业务在提交前完成，此处不再校验

	// 计算区块哈希
	blockHash, err := p.calcBlockHash(block)
	if err != nil {
		return nil, fmt.Errorf("计算区块哈希失败: %v", err)
	}

	if err := p.basicValidator.ValidateHash(block, blockHash); err != nil {
		p.validationErrors++
		if p.logger != nil {
			p.logger.Warnf("候选区块哈希验证失败: %v", err)
		}
		return nil, fmt.Errorf("哈希验证失败: %v", err)
	}

	// 大小验证
	if err := p.basicValidator.ValidateSize(block); err != nil {
		p.validationErrors++
		if p.logger != nil {
			p.logger.Warnf("候选区块大小验证失败: %v", err)
		}
		return nil, fmt.Errorf("大小验证失败: %v", err)
	}

	// 重复检测
	// ⚠️ 注意：禁止使用 string([]byte) 作为 map key（可能包含不可见字符/非UTF-8，导致不可观测与潜在边界问题）
	blockHashKey := hex.EncodeToString(blockHash)
	if _, exists := p.candidates[blockHashKey]; exists {
		p.duplicateBlocks++
		return blockHash, ErrCandidateAlreadyExists
	}

	if dup, err := p.basicValidator.ValidateDuplicate(blockHash); err != nil {
		p.duplicateBlocks++
		return blockHash, fmt.Errorf("重复检测失败: %v", err)
	} else if dup {
		p.duplicateBlocks++
		return blockHash, ErrCandidateAlreadyExists
	}

	// 内存限制验证
	estimatedSize := uint64(estimateBlockSize(block))
	if p.memoryUsage+estimatedSize > p.memoryLimit {
		p.memoryErrors++
		if p.logger != nil {
			p.logger.Warnf("内存限制验证失败，当前: %d, 估算: %d, 限制: %d", p.memoryUsage, estimatedSize, p.memoryLimit)
		}
		return blockHash, ErrMemoryLimit
	}

	// 最大候选数量控制（多层清理策略）
	maxCandidates := p.config.MaxCandidates
	if len(p.candidates) >= maxCandidates {
		// 1. 先进行标准清理（基于时间和高度）
		cleanedCount := p.cleanExpiredCandidatesInternal()

		// 2. 如果标准清理后仍然满，尝试激进清理
		if len(p.candidates) >= maxCandidates {
			aggressiveCleanedCount := p.cleanAggressively()
			cleanedCount += aggressiveCleanedCount
		}

		// 3. 如果经过所有清理后仍然满，则返回错误
		if len(p.candidates) >= maxCandidates {
			if p.logger != nil {
				p.logger.Warnf("候选区块池已满且清理无效 (清理了%d个): %d/%d",
					cleanedCount, len(p.candidates), maxCandidates)
			}
			return blockHash, ErrPoolFull
		}

		if p.logger != nil && cleanedCount > 0 {
			p.logger.Infof("候选区块池清理完成，清理了%d个候选区块，当前: %d/%d",
				cleanedCount, len(p.candidates), maxCandidates)
		}
	}

	// 创建候选区块包装器
	var sourcePeer peer.ID
	if fromPeer != "" {
		// fromPeer 约定为 peer.ID 的可打印形式（base58 / peer.ID.String()）。
		// 若无法 decode，则保持为空（不阻断入池），上层会用 FromPeer 做诊断。
		if pid, err := peer.Decode(fromPeer); err == nil && pid != "" {
			sourcePeer = pid
		}
	}
	candidate := &types.CandidateBlock{
		Block:     block,
		BlockHash: blockHash,
		Height:    block.Header.Height,

		ReceivedAt: time.Now(),
		Source:     sourcePeer,
		FromPeer:   fromPeer,
		LocalNode:  fromPeer == "",

		Verified:     false,
		VerifiedAt:   time.Time{},
		VerifyErrors: []string{},

		Selected:   false,
		SelectedAt: time.Time{},
		Expired:    false,

		Priority:         0,
		Difficulty:       block.Header.Difficulty,
		TransactionCount: len(block.Body.Transactions),
		EstimatedSize:    int(estimatedSize),
	}

	// 存储候选区块
	p.candidates[blockHashKey] = candidate
	p.pendingCandidates[blockHashKey] = struct{}{}

	// 按高度索引
	height := block.Header.Height
	p.candidatesByHeight[height] = append(p.candidatesByHeight[height], candidate)

	// 更新内存使用量与统计
	p.memoryUsage += estimatedSize
	p.totalAdded++

	// 通知等待方
	p.notifyWaiters(height)

	// 发布事件
	p.eventSink.OnCandidateAdded(candidate)

	if p.logger != nil {
		displayFrom := fromPeer
		if displayFrom == "" {
			displayFrom = "<local>"
		} else if !utf8.ValidString(displayFrom) {
			// 防御：历史版本可能把 peer.ID 的原始 bytes 直接转成 string，导致日志乱码。
			// 这里转成 hex 展示，保证可观测性，不影响内部存储语义。
			displayFrom = fmt.Sprintf("0x%x", []byte(displayFrom))
		}
		p.logger.Infof("添加候选区块成功，高度: %d, 哈希: %x, 来源: %s, 交易数: %d",
			height, blockHash[:8], displayFrom, len(block.Body.Transactions))
	}

	return blockHash, nil
}

// AddCandidates 批量添加候选区块。
// 参数：
// - blocks：候选区块列表；
// - fromPeers：对应来源节点ID列表，长度需与 blocks 相同。
// 返回：
// - [][]byte：成功添加的区块哈希列表；
// - error：若存在部分失败，返回聚合错误（包含失败计数）。
func (p *CandidatePool) AddCandidates(blocks []*core.Block, fromPeers []string) ([][]byte, error) {
	if len(blocks) != len(fromPeers) {
		return nil, fmt.Errorf("区块数量与节点数量不匹配")
	}

	var hashes [][]byte
	var errors []error

	for i, block := range blocks {
		hash, err := p.AddCandidate(block, fromPeers[i])
		if err != nil {
			errors = append(errors, err)
		} else {
			hashes = append(hashes, hash)
		}
	}

	if len(errors) > 0 {
		return hashes, fmt.Errorf("部分候选区块添加失败: %d个错误", len(errors))
	}

	return hashes, nil
}

// GetCandidatesForHeight 获取指定高度的所有候选区块（若无则等待）。
// 参数：
// - height：目标高度；
// - timeout：等待超时时间。
// 返回：
// - []*types.CandidateBlock：候选区块列表；
// - error：超时返回 ErrTimeout；其他错误按需返回。
func (p *CandidatePool) GetCandidatesForHeight(height uint64, timeout time.Duration) ([]*types.CandidateBlock, error) {
	p.mu.RLock()
	candidates := p.candidatesByHeight[height]
	if len(candidates) > 0 {
		// 创建副本以避免并发问题
		result := make([]*types.CandidateBlock, len(candidates))
		copy(result, candidates)
		p.mu.RUnlock()
		return result, nil
	}
	p.mu.RUnlock()

	// 如果没有候选区块，等待
	return p.waitForCandidatesAtHeight(height, timeout)
}

// GetAllCandidates 获取所有当前候选区块（快照）。
// 参数：无。
// 返回：
// - []*types.CandidateBlock：候选区块切片；
// - error：恒为 nil（当前实现）。
func (p *CandidatePool) GetAllCandidates() ([]*types.CandidateBlock, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*types.CandidateBlock, 0, len(p.candidates))
	for _, candidate := range p.candidates {
		result = append(result, candidate)
	}

	return result, nil
}

// WaitForCandidates 等待候选区块达到指定数量或超时。
// 参数：
// - minCount：最小候选数量阈值；
// - timeout：等待超时时间。
// 返回：
// - []*types.CandidateBlock：当前候选区块列表；
// - error：超时返回 ErrTimeout，池关闭返回 ErrPoolClosed。
func (p *CandidatePool) WaitForCandidates(minCount int, timeout time.Duration) ([]*types.CandidateBlock, error) {
	p.mu.RLock()
	if len(p.candidates) >= minCount {
		result := make([]*types.CandidateBlock, 0, len(p.candidates))
		for _, candidate := range p.candidates {
			result = append(result, candidate)
		}
		p.mu.RUnlock()
		return result, nil
	}
	p.mu.RUnlock()

	// 等待更多候选区块
	waitCh := make(chan []*types.CandidateBlock, 1)

	p.mu.Lock()
	waitKey := fmt.Sprintf("count_%d_%d", minCount, time.Now().UnixNano())
	p.waitChannels[waitKey] = waitCh
	p.mu.Unlock()

	// 清理等待通道
	defer func() {
		p.mu.Lock()
		delete(p.waitChannels, waitKey)
		p.mu.Unlock()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case candidates := <-waitCh:
		return candidates, nil
	case <-timer.C:
		return nil, ErrTimeout
	case <-p.quit:
		return nil, ErrPoolClosed
	}
}

// calcBlockHash 使用统一哈希服务计算区块哈希（若无则采用简化近似）。
// 参数：
// - block：候选区块。
// 返回：
// - []byte：区块哈希；
// - error：调用哈希服务失败或区块无效时返回错误。
func (p *CandidatePool) calcBlockHash(block *core.Block) ([]byte, error) {
	if p.hashService == nil {
		// 如果没有哈希服务，使用简单的哈希计算（仅用于开发/测试）
		return []byte(fmt.Sprintf("hash_%d_%d", block.Header.Height, block.Header.Timestamp)), nil
	}

	// 使用注入的哈希服务
	req := &core.ComputeBlockHashRequest{
		Block: block,
	}

	resp, err := p.hashService.ComputeBlockHash(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("计算区块哈希失败: %v", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("区块结构无效")
	}

	return resp.Hash, nil
}

// 其他内部方法见同目录 candidatepool_methods.go。
// 确保CandidatePool实现了InternalCandidatePool接口（编译期检查）
var _ interfaces.InternalCandidatePool = (*CandidatePool)(nil)
