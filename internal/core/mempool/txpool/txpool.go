// 文件说明：
// 本文件实现交易池（TxPool）的核心存储与维护逻辑：
// - 分层验证：只做基础安全验证，业务验证(签名/余额/UTXO等)由上层负责；
// - 事件下沉：通过 TxEventSink 统一对外发布事件，由 integration 层桥接 EventBus；
// - 内存管理：以字节为单位追踪 memoryUsage 与 memoryLimit，执行清理与淘汰策略；
// - 线程安全：使用读写锁保护内部状态；
// - 挖矿支持：提供待打包交易的挑选、挖矿中标记、确认/拒绝等操作。
package txpool

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/config/txpool"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/utils"
)

// eventTopics 定义交易池相关的事件主题（名称规范，不直接耦合实现）
const (
	TopicTxSubmitted event.EventType = "tx:submitted"
	TopicTxAccepted  event.EventType = "tx:accepted"
	TopicTxRejected  event.EventType = "tx:rejected"
	TopicTxExpired   event.EventType = "tx:expired"
	TopicTxRemoved   event.EventType = "tx:removed"
	TopicPoolState   event.EventType = "pool:state"
)

// 错误定义
var (
	ErrTxAlreadyExists          = errors.New("交易已存在于交易池")
	ErrTxRejected               = errors.New("交易被拒绝")
	ErrTxPoolFull               = errors.New("交易池已满")
	ErrInvalidTransaction       = errors.New("无效交易")
	ErrMissingInputs            = errors.New("缺少交易输入")
	ErrDuplicateUTXOSpend       = errors.New("UTXO重复花费")
	ErrTxFeeTooLow              = errors.New("交易手续费太低")
	ErrTxPoolClosed             = errors.New("交易池已关闭")
	ErrInsufficientFunds        = errors.New("资金不足")
	ErrExceedsMaxTxSize         = errors.New("超过最大交易大小")
	ErrTxChainLimit             = errors.New("超过交易链长度限制")
	ErrTxSizeLimitExceeded      = errors.New("超过执行费用限制")
	ErrInputsSumLessThanOutputs = errors.New("输入总额小于输出总额")
)

// TxPool 分层验证交易池。
// 职责：
// - 基础安全验证（格式/哈希/大小/重复/内存）；
// - 存储/索引/优先队列；
// - 事件下沉；
// - 面向挖矿的待打包选择接口。
type TxPool struct {
	// ========== 分层验证组件 ==========
	basicValidator BasicTxValidator // 基础安全验证器（防网络攻击）

	// ========== 纯存储字段 ==========
	txs               map[string]*TxWrapper // 交易ID到交易包装器的映射
	pendingTxs        map[string]struct{}   // 待处理交易
	rejectedTxs       map[string]struct{}   // 被拒绝交易
	confirmedTxs      map[string]struct{}   // 已确认交易
	expiredTxs        map[string]struct{}   // 已过期交易
	pendingConfirmTxs map[string]struct{}   // 待确认交易（已挖出区块，等待网络确认）

	// ========== 存储管理字段 ==========
	config      *txpool.TxPoolOptions // 交易池配置
	memory      storage.MemoryStore   // 内存存储
	memoryLimit uint64                // 内存使用限制(字节)
	memoryUsage uint64                // 当前内存使用量(字节)

	// ========== 基础设施字段 ==========
	logger    log.Logger    // 日志记录器
	eventSink TxEventSink   // 事件下沉
	mu        sync.RWMutex  // 同步锁
	quit      chan struct{} // 关闭信号

	// ========== 注入的基础服务 ==========
	hashService      transaction.TransactionHashServiceClient // 交易哈希服务（来自crypto模块，避免循环依赖）
	chainStateCache  ChainStateProvider                       // 链状态缓存（可选，用于事件驱动架构）
	compliancePolicy complianceIfaces.Policy                  // 合规策略服务（可选）

	// ========== 保留的队列管理 ==========
	pendingQueue *PriorityQueue // 优先级队列（纯存储逻辑）
}

// ChainStateProvider 链状态提供者接口（用于事件驱动架构）
// 说明：TxPool 不直接做业务同步，仅保留可选状态入口。
type ChainStateProvider interface {
	GetCurrentHeight(ctx context.Context) (uint64, error)
	GetLatestBlockHash(ctx context.Context) ([]byte, error)
	IsValidHeight(height uint64) bool
}

// NewTxPool 创建新的分层验证交易池（简化入口）。
// 参数：
// - config：高层配置；
// - logger：日志接口；
// - eventBus：事件总线（由 integration 注入事件下沉实现）；
// - memory：内存存储；
// - hashService：交易哈希服务客户端。
// 返回：mempoolIfaces.TxPool 实例或错误。
func NewTxPool(
	config *txpool.Config,
	logger log.Logger,
	eventBus event.EventBus,
	memory storage.MemoryStore,
	hashService transaction.TransactionHashServiceClient,
) (mempoolIfaces.TxPool, error) {
	return NewTxPoolWithCache(config.GetOptions(), logger, eventBus, memory, hashService, nil)
}

// NewTxPoolWithCache 创建带链状态缓存的交易池（事件驱动版本）。
// 参数：
// - config：交易池选项；
// - logger：日志接口；
// - eventBus：事件总线（由 integration 注入事件下沉实现）；
// - memory：内存存储；
// - hashService：交易哈希服务客户端；
// - chainStateCache：可选链状态提供者。
// 返回：mempoolIfaces.TxPool 实例或错误。
func NewTxPoolWithCache(
	config *txpool.TxPoolOptions,
	logger log.Logger,
	eventBus event.EventBus,
	memory storage.MemoryStore,
	hashService transaction.TransactionHashServiceClient,
	chainStateCache ChainStateProvider, // 可选的链状态缓存
) (mempoolIfaces.TxPool, error) {
	if config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	// 创建基础验证器（使用配置参数，避免魔法数）
	basicValidator := NewProductionBasicValidator(
		config.MaxTxSize,
		config.MemoryLimit,
		nil,
		hashService,
		logger,
	)

	// 创建交易池
	memLimit := config.MemoryLimit

	pool := &TxPool{
		// ========== 分层验证组件 ==========
		basicValidator: basicValidator,

		// ========== 纯存储字段 ==========
		txs:               make(map[string]*TxWrapper),
		pendingTxs:        make(map[string]struct{}),
		rejectedTxs:       make(map[string]struct{}),
		confirmedTxs:      make(map[string]struct{}),
		expiredTxs:        make(map[string]struct{}),
		pendingConfirmTxs: make(map[string]struct{}),

		// ========== 存储管理字段 ==========
		config:      config,
		memory:      memory,
		memoryLimit: memLimit,
		memoryUsage: 0,

		// ========== 基础设施字段 ==========
		logger:          logger,
		quit:            make(chan struct{}),
		hashService:     hashService,
		chainStateCache: chainStateCache,
		eventSink:       NoopTxEventSink{},

		// ========== 保留的队列管理 ==========
		pendingQueue: NewPriorityQueue(),
	}

	// 启动维护协程
	go pool.maintenanceLoop()

	return pool, nil
}

// NewTxPoolWithCacheAndCompliance 创建带缓存和合规策略的交易池
//
// 🏗️ **合规增强交易池构造函数 (Compliance-Enhanced TxPool Constructor)**
//
// 创建一个支持合规检查的交易池实例，集成所有必要的依赖。
//
// 参数：
// - config: 交易池配置选项
// - logger: 日志记录器（可选）
// - eventBus: 事件总线（可选）
// - memory: 内存存储（可选）
// - hashService: 交易哈希服务
// - chainStateCache: 链状态缓存（可选）
// - compliancePolicy: 合规策略服务（可选）
//
// 返回：
// - mempoolIfaces.TxPool: 交易池接口实例
// - error: 构造失败时的错误
func NewTxPoolWithCacheAndCompliance(
	config *txpool.TxPoolOptions,
	logger log.Logger,
	eventBus event.EventBus,
	memory storage.MemoryStore,
	hashService transaction.TransactionHashServiceClient,
	chainStateCache ChainStateProvider,
	compliancePolicy complianceIfaces.Policy, // 合规策略服务
) (mempoolIfaces.TxPool, error) {
	if config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}

	// 创建基础验证器（使用配置参数，避免魔法数）
	basicValidator := NewProductionBasicValidator(
		config.MaxTxSize,
		config.MemoryLimit,
		nil,
		hashService,
		logger,
	)

	// 创建交易池
	memLimit := config.MemoryLimit

	pool := &TxPool{
		// ========== 分层验证组件 ==========
		basicValidator: basicValidator,

		// ========== 纯存储字段 ==========
		txs:               make(map[string]*TxWrapper),
		pendingTxs:        make(map[string]struct{}),
		rejectedTxs:       make(map[string]struct{}),
		confirmedTxs:      make(map[string]struct{}),
		expiredTxs:        make(map[string]struct{}),
		pendingConfirmTxs: make(map[string]struct{}),

		// ========== 存储管理字段 ==========
		config:      config,
		memory:      memory,
		memoryLimit: memLimit,
		memoryUsage: 0,

		// ========== 基础设施字段 ==========
		logger:           logger,
		quit:             make(chan struct{}),
		hashService:      hashService,
		chainStateCache:  chainStateCache,
		compliancePolicy: compliancePolicy, // 注入合规策略
		eventSink:        NoopTxEventSink{},

		// ========== 保留的队列管理 ==========
		pendingQueue: NewPriorityQueue(),
	}

	// 记录合规策略状态
	if compliancePolicy != nil && logger != nil {
		logger.Info("交易池已集成合规策略检查")
	}

	// 启动维护协程
	go pool.maintenanceLoop()

	return pool, nil
}

// maintenanceLoop 周期性维护：清理过期交易与重算优先级。
func (p *TxPool) maintenanceLoop() {
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-cleanupTicker.C:
			p.cleanExpiredTransactions()
			p.recomputePriorities()
		case <-p.quit:
			return
		}
	}
}

// AddTransaction 向交易池添加交易。
// 参数：
// - tx：待添加交易。
// 返回：
// - []byte：交易ID；
// - error：错误（包装为 TxPoolError 或具体错误）。
// 说明：使用统一哈希服务计算与验证哈希，避免与 blockchain 循环依赖。
func (p *TxPool) AddTransaction(tx *transaction.Transaction) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查交易池是否已关闭
	select {
	case <-p.quit:
		return nil, ErrTxPoolClosed
	default:
	}

	// 🛡️ 基础安全验证
	if err := p.basicValidator.ValidateFormat(tx); err != nil {
		p.logger.Warn("交易格式验证失败")
		return nil, WrapTxPoolError(ErrCodeInvalidFormat, "格式验证失败", err)
	}

	// 🔒 合规性检查（在哈希计算前进行，避免不必要的计算）
	if p.compliancePolicy != nil {
		if err := p.checkTransactionCompliance(tx); err != nil {
			p.logger.Warnf("交易合规检查失败: %v", err)
			return nil, WrapTxPoolError(ErrCodeComplianceViolation, "合规检查失败", err)
		}
	}

	// 计算与验证哈希
	txIDBytes, err := p.calcTxID(tx)
	if err != nil {
		return nil, WrapTxPoolError(ErrCodeInvalidHash, "哈希计算失败", err)
	}
	if err := p.basicValidator.ValidateHash(tx, txIDBytes); err != nil {
		p.logger.Warn("交易哈希验证失败")
		return nil, WrapTxPoolError(ErrCodeInvalidHash, "哈希验证失败", err)
	}

	// 大小验证
	if err := p.basicValidator.ValidateSize(tx); err != nil {
		p.logger.Warn("交易大小验证失败")
		return nil, WrapTxPoolError(ErrCodeTxTooLarge, "大小验证失败", err)
	}

	// 重复检测
	if err := p.basicValidator.ValidateDuplicate(txIDBytes); err != nil {
		return nil, WrapTxPoolError(ErrCodeDuplicateTx, "重复交易", err)
	}

	// ==================== UTXO冲突检测（防双花） ====================
	// 检查新交易是否与现有pending交易存在UTXO冲突
	// 基于历史实现：只检查pending状态交易，直接拒绝冲突交易（非RBF）
	conflictingTxs := p.detectUTXOConflicts(tx)
	if len(conflictingTxs) > 0 {
		if p.logger != nil {
			p.logger.Warnf("检测到UTXO冲突，拒绝新交易以防止双花，冲突交易数: %d", len(conflictingTxs))
		}
		return nil, ErrDuplicateUTXOSpend
	}

	// 内存限制
	txSize := uint64(calculateTransactionSize(tx))
	if err := p.basicValidator.ValidateMemoryLimit(p.memoryUsage, txSize); err != nil {
		p.logger.Warn("内存限制验证失败")
		return nil, WrapTxPoolError(ErrCodeMemoryLimit, "内存限制", err)
	}

	// 存储逻辑
	txIDStr := string(txIDBytes)
	if _, exists := p.txs[txIDStr]; exists {
		return txIDBytes, ErrTxAlreadyExists
	}

	newTxSize := calculateTransactionSize(tx)
	if p.memoryUsage+newTxSize > p.memoryLimit {
		p.cleanExpiredTransactions()
		if p.memoryUsage+newTxSize > p.memoryLimit {
			// 执行淘汰策略，同时保持内存计数准确
			txWrappers := make([]*TxWrapper, 0, len(p.txs))
			for _, wrapper := range p.txs {
				txWrappers = append(txWrappers, wrapper)
			}
			_ = p.executeEvictionStrategy(txWrappers, (p.memoryUsage+newTxSize)-p.memoryLimit)
			if p.memoryUsage+newTxSize > p.memoryLimit {
				return txIDBytes, ErrTxPoolFull
			}
		}
	}

	wrapper := NewTxWrapper(tx, txIDBytes)
	wrapper.Priority = int32(p.calculateTransactionPriority(wrapper))

	p.txs[txIDStr] = wrapper
	p.pendingTxs[txIDStr] = struct{}{}
	p.memoryUsage += newTxSize
	p.pendingQueue.Push(wrapper)

	// 发布事件
	p.eventSink.OnTxAdded(wrapper)

	return txIDBytes, nil
}

// checkTransactionCompliance 检查交易合规性
//
// 🔒 **合规性检查辅助方法 (Compliance Check Helper)**
//
// 对单个交易执行合规策略检查，包含：
// 1. 用户地理位置验证
// 2. 操作类型限制检查
// 3. 合规决策记录和事件发布
//
// 参数：
// - tx: 待检查的交易
//
// 返回：
// - error: 合规检查失败时返回错误，通过时返回nil
func (p *TxPool) checkTransactionCompliance(tx *transaction.Transaction) error {
	if p.compliancePolicy == nil {
		return nil // 未配置合规策略时直接通过
	}

	ctx := context.Background()

	// 创建交易来源信息
	source := &complianceIfaces.TransactionSource{
		Protocol:  "mempool",
		Timestamp: time.Now(),
	}

	// 执行合规检查
	decision, err := p.compliancePolicy.CheckTransaction(ctx, tx, source)
	if err != nil {
		p.logger.Errorf("合规策略检查失败: %v", err)
		return fmt.Errorf("合规策略检查失败: %v", err)
	}

	// 检查合规决策
	if !decision.Allowed {
		// 记录详细的合规拒绝信息
		p.logger.Warnf("交易被合规策略拒绝: 原因=%s, 详情=%s, 国家=%s, 信息源=%s",
			decision.Reason, decision.ReasonDetail, decision.Country, decision.Source)

		return fmt.Errorf("交易不符合合规要求: %s (%s)", decision.Reason, decision.ReasonDetail)
	}

	// 合规通过，记录信息（调试级别，避免日志过多）
	if p.logger != nil {
		p.logger.Debugf("交易通过合规检查: 国家=%s, 信息源=%s", decision.Country, decision.Source)
	}

	return nil
}

// GetTransaction 获取指定交易。
// 参数：
// - txID：交易ID。
// 返回：
// - *transaction.Transaction：若存在则返回；
// - error：不存在返回错误。
func (p *TxPool) GetTransaction(txID []byte) (*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txIDStr := string(txID)
	if wrapper, exists := p.txs[txIDStr]; exists {
		return wrapper.Tx, nil
	}

	return nil, errors.New("交易不存在")
}

// RemoveTransaction 从交易池移除交易（对外方法，加锁封装）。
// 参数：
// - txID：交易ID。
// 返回：
// - error：不存在返回错误，否则 nil。
func (p *TxPool) RemoveTransaction(txID []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.removeTransaction(txID)
}

// removeTransaction 内部实现（不加锁）。
func (p *TxPool) removeTransaction(txID []byte) error {
	txIDStr := string(txID)

	wrapper, exists := p.txs[txIDStr]
	if !exists {
		return errors.New("交易不存在")
	}

	txSize := calculateTransactionSize(wrapper.Tx)

	if wrapper.Status == TxStatusPending {
		p.pendingQueue.Remove(wrapper)
		delete(p.pendingTxs, txIDStr)
	} else if wrapper.Status == TxStatusRejected {
		delete(p.rejectedTxs, txIDStr)
	} else if wrapper.Status == TxStatusConfirmed {
		delete(p.confirmedTxs, txIDStr)
	} else if wrapper.Status == TxStatusExpired {
		delete(p.expiredTxs, txIDStr)
	} else if wrapper.Status == TxStatusPendingConfirm {
		delete(p.pendingConfirmTxs, txIDStr)
	}

	delete(p.txs, txIDStr)

	if p.memoryUsage >= txSize {
		p.memoryUsage -= txSize
	}

	p.eventSink.OnTxRemoved(wrapper)

	return nil
}

// detectUTXOConflicts 检测新交易与现有交易之间的UTXO冲突
func (p *TxPool) detectUTXOConflicts(newTx *transaction.Transaction) []*transaction.Transaction {
	conflictingTxs := make([]*transaction.Transaction, 0)

	if p.logger != nil {
		p.logger.Infof("🔍 [存储检查] 开始检测UTXO冲突，新交易输入数: %d", len(newTx.Inputs))
	}

	// 遍历所有现有交易，检查UTXO冲突
	for txIDStr, wrapper := range p.txs {
		if wrapper.Status == TxStatusPending { // 只检查待处理交易
			if p.logger != nil {
				p.logger.Infof("🔍 [存储检查] 检查与现有交易的冲突: %s (输入数: %d)",
					txIDStr[:16], len(wrapper.Tx.Inputs))
			}

			if p.hasUTXOConflict(newTx, wrapper.Tx) {
				if p.logger != nil {
					p.logger.Infof("⚠️ [存储检查] 检测到UTXO冲突！新交易与现有交易 %s 存在冲突", txIDStr[:16])
				}
				conflictingTxs = append(conflictingTxs, wrapper.Tx)
			}
		}
	}

	if p.logger != nil {
		p.logger.Infof("🔍 [存储检查] UTXO冲突检测完成，冲突交易数: %d", len(conflictingTxs))
	}

	return conflictingTxs
}

// hasUTXOConflict 检查两个交易之间是否存在UTXO冲突
func (p *TxPool) hasUTXOConflict(tx1, tx2 *transaction.Transaction) bool {
	// 检查是否使用了相同的UTXO输入
	for i, input1 := range tx1.Inputs {
		if input1.PreviousOutput == nil {
			continue
		}
		for j, input2 := range tx2.Inputs {
			if input2.PreviousOutput == nil {
				continue
			}
			// 比较UTXO引用：相同的 OutPoint 表示冲突
			if utils.UTXOKey(input1.PreviousOutput.TxId, input1.PreviousOutput.OutputIndex) == utils.UTXOKey(input2.PreviousOutput.TxId, input2.PreviousOutput.OutputIndex) {
				p.logger.Debugf("UTXO冲突: TX1输入[%d] vs TX2输入[%d] - %x:%d",
					i, j, input1.PreviousOutput.TxId, input1.PreviousOutput.OutputIndex)
				return true
			}
		}
	}
	return false
}

// GetPendingTransactionsWithLimit 获取待处理交易（带数量限制）。
// 参数：
// - limit：最大返回数量（<=0 表示不限）。
// 返回：
// - []*transaction.Transaction：交易列表。
func (p *TxPool) GetPendingTransactionsWithLimit(limit int) []*transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 {
		limit = len(p.pendingTxs)
	}

	pqCopy := p.pendingQueue.Copy()

	result := make([]*transaction.Transaction, 0, limit)
	for i := 0; i < limit && pqCopy.Len() > 0; i++ {
		item := heap.Pop(pqCopy).(*TxWrapper)
		result = append(result, item.Tx)
	}

	return result
}

// GetPendingTransactionsByDependencyOrder 按依赖顺序获取待处理交易（占位实现）。
// 参数：
// - limit：最大返回数量（<=0 表示不限）。
// 返回：
// - []*transaction.Transaction：交易列表。
func (p *TxPool) GetPendingTransactionsByDependencyOrder(limit int) []*transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pendingIDs := make([][]byte, 0, len(p.pendingTxs))
	for txIDStr := range p.pendingTxs {
		pendingIDs = append(pendingIDs, []byte(txIDStr))
	}

	sortedIDs := pendingIDs // 依赖排序由业务域处理

	if limit <= 0 {
		limit = len(sortedIDs)
	}

	result := make([]*transaction.Transaction, 0, limit)
	for i := 0; i < limit && i < len(sortedIDs); i++ {
		txIDStr := string(sortedIDs[i])
		if wrapper, exists := p.txs[txIDStr]; exists {
			result = append(result, wrapper.Tx)
		}
	}

	return result
}

// GetTransactionStatus 获取交易状态（接口适配）。
// 参数：
// - txID：交易ID。
// 返回：
// - mempoolIfaces.TxStatus：状态；
// - error：交易不存在时返回错误。
func (p *TxPool) GetTransactionStatus(txID []byte) (mempoolIfaces.TxStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txIDStr := string(txID)
	wrapper, exists := p.txs[txIDStr]
	if !exists {
		return mempoolIfaces.TxStatusUnknown, errors.New("交易不存在")
	}

	switch wrapper.Status {
	case TxStatusPending:
		return mempoolIfaces.TxStatusPending, nil
	case TxStatusRejected:
		return mempoolIfaces.TxStatusRejected, nil
	case TxStatusConfirmed:
		return mempoolIfaces.TxStatusConfirmed, nil
	case TxStatusExpired:
		return mempoolIfaces.TxStatusExpired, nil
	default:
		return mempoolIfaces.TxStatusUnknown, nil
	}
}

// UpdateTransactionStatus 更新交易状态（内部管理方法）。
// 参数：
// - txID：交易ID；
// - status：对外接口定义的交易状态。
// 返回：error。
func (p *TxPool) UpdateTransactionStatus(txID []byte, status mempoolIfaces.TxStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	txIDStr := string(txID)
	wrapper, exists := p.txs[txIDStr]
	if !exists {
		p.logger.Debugf("⚠️ [交易池] 尝试更新不存在的交易状态: %x", txID)
		return errors.New("交易不存在")
	}

	var newStatus TxStatus
	switch status {
	case mempoolIfaces.TxStatusPending:
		newStatus = TxStatusPending
	case mempoolIfaces.TxStatusRejected:
		newStatus = TxStatusRejected
	case mempoolIfaces.TxStatusConfirmed:
		newStatus = TxStatusConfirmed
	case mempoolIfaces.TxStatusExpired:
		newStatus = TxStatusExpired
	default:
		p.logger.Warnf("❌ [交易池] 无效的交易状态: %v", status)
		return errors.New("无效的交易状态")
	}

	if wrapper.Status == newStatus {
		p.logger.Debugf("💡 [交易池] 交易状态未变化: %x, 状态: %v", txID, newStatus)
		return nil
	}

	p.logger.Infof("🔄 [交易池] 更新交易状态: %x, %v -> %v", txID, wrapper.Status, newStatus)

	switch wrapper.Status {
	case TxStatusPending:
		delete(p.pendingTxs, txIDStr)
		p.pendingQueue.Remove(wrapper)
	case TxStatusRejected:
		delete(p.rejectedTxs, txIDStr)
	case TxStatusConfirmed:
		delete(p.confirmedTxs, txIDStr)
	case TxStatusExpired:
		delete(p.expiredTxs, txIDStr)
	case TxStatusPendingConfirm:
		delete(p.pendingConfirmTxs, txIDStr)
	}

	switch newStatus {
	case TxStatusPending:
		p.pendingTxs[txIDStr] = struct{}{}
		wrapper.Priority = int32(p.calculateTransactionPriority(wrapper))
		p.pendingQueue.Push(wrapper)
	case TxStatusRejected:
		p.rejectedTxs[txIDStr] = struct{}{}
	case TxStatusConfirmed:
		p.confirmedTxs[txIDStr] = struct{}{}
	case TxStatusExpired:
		p.expiredTxs[txIDStr] = struct{}{}
	case TxStatusPendingConfirm:
		p.pendingConfirmTxs[txIDStr] = struct{}{}
	}

	wrapper.Status = newStatus

	if newStatus == TxStatusConfirmed {
		p.logger.Infof("✅ [交易池] 交易已确认: %x", txID)
		p.eventSink.OnTxConfirmed(wrapper, 0)
	}

	p.logger.Debugf("✅ [交易池] 交易状态更新成功: %x", txID)
	return nil
}

// Close 关闭交易池（发出退出信号）。
func (p *TxPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.quit:
		return nil
	default:
		close(p.quit)
	}

	return nil
}

// Reset 重置交易池：清空所有存储并重置内存计数。
func (p *TxPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.txs = make(map[string]*TxWrapper)
	p.pendingQueue = NewPriorityQueue()
	p.pendingTxs = make(map[string]struct{})
	p.rejectedTxs = make(map[string]struct{})
	p.confirmedTxs = make(map[string]struct{})
	p.expiredTxs = make(map[string]struct{})
	p.pendingConfirmTxs = make(map[string]struct{})
	p.memoryUsage = 0
}

// SetEventSink 注入事件下沉实现（nil 时降级为 Noop）。
func (p *TxPool) SetEventSink(sink TxEventSink) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sink == nil {
		p.eventSink = NoopTxEventSink{}
		return
	}
	p.eventSink = sink
}

// BatchAddTransactions 批量添加交易。
// 返回：交易ID列表与错误列表（与输入一一对应）。
func (p *TxPool) BatchAddTransactions(txs []*transaction.Transaction) ([][]byte, []error) {
	txIDs := make([][]byte, len(txs))
	errors := make([]error, len(txs))
	for i, tx := range txs {
		txID, err := p.AddTransaction(tx)
		txIDs[i] = txID
		errors[i] = err
	}
	return txIDs, errors
}

// BatchRemoveTransactions 批量移除交易。
func (p *TxPool) BatchRemoveTransactions(txIDs [][]byte) []error {
	errors := make([]error, len(txIDs))
	for i, txID := range txIDs {
		errors[i] = p.RemoveTransaction(txID)
	}
	return errors
}

// cleanExpiredTransactions 清理过期交易（内部）。
func (p *TxPool) cleanExpiredTransactions() {
	currentTime := time.Now()
	lifetime := p.config.Lifetime

	for txIDStr, wrapper := range p.txs {
		if wrapper.Status == TxStatusPending {
			expireTime := wrapper.ReceivedAt.Add(lifetime)
			if currentTime.After(expireTime) {
				wrapper.Status = TxStatusExpired
				delete(p.pendingTxs, txIDStr)
				p.pendingQueue.Remove(wrapper)
				p.expiredTxs[txIDStr] = struct{}{}
				p.eventSink.OnTxRemoved(wrapper)
			}
		}
	}
}

// recomputePriorities 重新计算所有待处理交易的优先级。
func (p *TxPool) recomputePriorities() {
	for txIDStr := range p.pendingTxs {
		if wrapper, exists := p.txs[txIDStr]; exists {
			newPriority := int32(p.calculateTransactionPriority(wrapper))
			p.pendingQueue.Update(wrapper, newPriority)
		}
	}
}

// GetPendingTxs 获取用于区块打包的交易列表。
// 参数：
// - maxCount：最大交易数；
// - maxSizeLimit：执行费用 上限；
// - excludedTxs：排除的交易ID集合。
// 返回：交易列表和错误。
func (p *TxPool) GetPendingTxs(maxCount uint32, maxSizeLimit uint64, excludedTxs [][]byte) ([]*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	select {
	case <-p.quit:
		return nil, ErrTxPoolClosed
	default:
	}

	excluded := make(map[string]struct{})
	for _, txID := range excludedTxs {
		excluded[string(txID)] = struct{}{}
	}

	result := make([]*transaction.Transaction, 0, maxCount)
	Size := uint64(0)

	queueCopy := p.pendingQueue.Copy()
	for queueCopy.Len() > 0 && uint32(len(result)) < maxCount && Size < maxSizeLimit {
		wrapper := heap.Pop(queueCopy).(*TxWrapper)
		if _, isExcluded := excluded[string(wrapper.TxID)]; isExcluded {
			continue
		}
		txSize := wrapper.Size
		if txSize == 0 {
			txSize = estimateExecutionFeeUsage(wrapper.Tx)
		}
		if Size+txSize > maxSizeLimit {
			continue
		}
		result = append(result, wrapper.Tx)
		Size += txSize
	}
	return result, nil
}

// GetAllPendingTransactions 获取所有 pending 状态交易。
func (p *TxPool) GetAllPendingTransactions() ([]*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*transaction.Transaction, 0, len(p.pendingTxs))
	for txIDStr := range p.pendingTxs {
		if wrapper, exists := p.txs[txIDStr]; exists {
			if wrapper.Status == TxStatusPending {
				result = append(result, wrapper.Tx)
			}
		}
	}
	if p.logger != nil {
		p.logger.Debugf("返回 %d 个待处理交易", len(result))
	}
	return result, nil
}

// estimateExecutionFeeUsage 估算交易的执行费用使用量
func estimateExecutionFeeUsage(tx *transaction.Transaction) uint64 {
	// 基本执行费用消耗
	baseExecutionFee := uint64(21000) // 基础交易执行费用

	// 数据执行费用 - 计算元数据序列化后的大小
	var dataBytesExecutionFee uint64
	if tx.Metadata != nil {
		// 估算元数据序列化后的大小
		dataBytesExecutionFee = uint64(100) * 68 // 假设元数据大约100字节，每字节68执行费用
	}

	// 输入消耗
	inputExecutionFee := uint64(len(tx.Inputs)) * 2000 // 每个输入2000执行费用

	// 输出消耗
	outputExecutionFee := uint64(len(tx.Outputs)) * 1000 // 每个输出1000执行费用

	// 汇总执行费用消耗
	totalExecutionFee := baseExecutionFee + dataBytesExecutionFee + inputExecutionFee + outputExecutionFee

	return totalExecutionFee
}

// GetTransactionsByStatus 根据状态获取交易列表（接口实现）。
func (p *TxPool) GetTransactionsByStatus(status mempoolIfaces.TxStatus) ([]*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*transaction.Transaction, 0)
	var targetStatusMap map[string]struct{}
	switch status {
	case mempoolIfaces.TxStatusPending:
		targetStatusMap = p.pendingTxs
	case mempoolIfaces.TxStatusRejected:
		targetStatusMap = p.rejectedTxs
	case mempoolIfaces.TxStatusConfirmed:
		targetStatusMap = p.confirmedTxs
	case mempoolIfaces.TxStatusExpired:
		targetStatusMap = p.expiredTxs
	default:
		return nil, fmt.Errorf("不支持的交易状态: %v", status)
	}

	for txIDStr := range targetStatusMap {
		if wrapper, exists := p.txs[txIDStr]; exists {
			result = append(result, wrapper.Tx)
		}
	}
	return result, nil
}

// GetTx 实现接口：获取交易（等价于 GetTransaction）。
func (p *TxPool) GetTx(txID []byte) (*transaction.Transaction, error) { return p.GetTransaction(txID) }

// SyncStatus 同步交易池状态与区块链最新状态（简化实现）。
func (p *TxPool) SyncStatus(height uint64, stateRoot []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	syncTime := time.Now()
	for txIDStr, wrapper := range p.txs {
		if wrapper.Status == TxStatusPending {
			lifetime := p.config.Lifetime
			expireTime := wrapper.ReceivedAt.Add(lifetime)
			if syncTime.After(expireTime) {
				wrapper.Status = TxStatusExpired
				delete(p.pendingTxs, txIDStr)
				p.pendingQueue.Remove(wrapper)
				p.expiredTxs[txIDStr] = struct{}{}
				p.eventSink.OnTxRemoved(wrapper)
			}
		}
	}
	return nil
}

// GetTxStatus 作为 GetTransactionStatus 的别名以满足接口要求。
func (p *TxPool) GetTxStatus(txID []byte) (mempoolIfaces.TxStatus, error) {
	return p.GetTransactionStatus(txID)
}

// RemoveTxs 批量移除交易（包装 BatchRemoveTransactions）。
func (p *TxPool) RemoveTxs(txIDs [][]byte) error {
	errors := p.BatchRemoveTransactions(txIDs)
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

// SubmitTx 提交交易到交易池
func (p *TxPool) SubmitTx(tx *transaction.Transaction) ([]byte, error) {
	return p.AddTransaction(tx)
}

// SubmitTxs 批量提交交易到交易池
func (p *TxPool) SubmitTxs(txs []*transaction.Transaction) ([][]byte, error) {
	txIDs, errs := p.BatchAddTransactions(txs)
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return txIDs, nil
}

// ==================== 挖矿专用方法实现 ====================

// GetTransactionsForMining 获取用于挖矿的交易（按费率与大小排序选择）。
// 使用配置文件中的挖矿参数来控制选择的交易数量和区块大小限制。
func (p *TxPool) GetTransactionsForMining() ([]*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	select {
	case <-p.quit:
		return nil, ErrTxPoolClosed
	default:
	}

	// 从配置中获取挖矿参数
	maxCount := p.config.Mining.MaxTransactionsForMining
	maxSize := p.config.Mining.MaxBlockSizeForMining

	p.logger.Infof("🔍 [交易池] 开始获取挖矿交易，当前pending交易数: %d，队列长度: %d，配置限制: 最大交易数=%d，最大区块大小=%d bytes",
		len(p.pendingTxs), p.pendingQueue.Len(), maxCount, maxSize)

	type txCandidate struct {
		tx       *transaction.Transaction
		priority uint64
		size     uint64
	}
	var candidates []txCandidate
	complianceFilteredCount := 0
	for txIDStr := range p.pendingTxs {
		if wrapper, exists := p.txs[txIDStr]; exists && wrapper.Status == TxStatusPending {
			// 🔒 合规性过滤（挖矿阶段）
			if p.compliancePolicy != nil {
				if err := p.checkTransactionCompliance(wrapper.Tx); err != nil {
					p.logger.Debugf("挖矿阶段过滤不合规交易: %s", err.Error())
					complianceFilteredCount++
					continue
				}
			}

			txSize := calculateTransactionSize(wrapper.Tx)
			candidates = append(candidates, txCandidate{tx: wrapper.Tx, priority: uint64(wrapper.Priority), size: txSize})
		}
	}

	// 记录合规过滤统计
	if complianceFilteredCount > 0 && p.logger != nil {
		p.logger.Infof("🔒 [合规过滤] 挖矿阶段过滤了 %d 笔不合规交易", complianceFilteredCount)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].priority > candidates[j].priority })

	// ==================== UTXO冲突去重（防止区块内双花） ====================
	var selectedTxs []*transaction.Transaction
	var totalSize uint64
	count := uint32(0)
	usedOutPoints := make(map[string]struct{}) // 记录已使用的OutPoint，防止冲突
	conflictSkippedCount := 0

	for _, c := range candidates {
		if count >= maxCount {
			break
		}
		if totalSize+c.size > maxSize {
			break
		}

		// 检查当前交易是否与已选交易存在UTXO冲突
		hasConflict := false
		currentOutPoints := make([]string, 0, len(c.tx.Inputs))
		for _, input := range c.tx.Inputs {
			if input.PreviousOutput == nil {
				continue
			}
			outPointKey := p.makeOutPointKey(input.PreviousOutput)
			currentOutPoints = append(currentOutPoints, outPointKey)
			if _, exists := usedOutPoints[outPointKey]; exists {
				hasConflict = true
				break
			}
		}

		if hasConflict {
			conflictSkippedCount++
			if p.logger != nil {
				p.logger.Debugf("⚠️ [挖矿去重] 跳过与已选交易冲突的交易")
			}
			continue
		}

		// 无冲突，选中此交易并记录其OutPoint
		selectedTxs = append(selectedTxs, c.tx)
		totalSize += c.size
		count++
		for _, outPointKey := range currentOutPoints {
			usedOutPoints[outPointKey] = struct{}{}
		}
	}

	p.logger.Infof("✅ [交易池] 为挖矿选择了 %d 个交易 (候选: %d, 冲突跳过: %d, 总大小: %d bytes)",
		len(selectedTxs), len(candidates), conflictSkippedCount, totalSize)

	// 打印选中的交易数量（交易ID计算复杂，先省略）
	return selectedTxs, nil
}

// MarkTransactionsAsMining 标记交易为挖矿中。
func (p *TxPool) MarkTransactionsAsMining(txIDs [][]byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.quit:
		return ErrTxPoolClosed
	default:
	}

	p.logger.Infof("⛏️ [交易池] 开始标记 %d 个交易为挖矿中", len(txIDs))

	marked := 0
	notFound := 0
	notPending := 0

	for _, txID := range txIDs {
		txIDStr := string(txID)
		txWrapper, exists := p.txs[txIDStr]
		if !exists {
			notFound++
			p.logger.Debugf("⚠️ [交易池] 交易不存在，无法标记为挖矿中: %x", txID)
			continue
		}
		if _, isPending := p.pendingTxs[txIDStr]; isPending {
			delete(p.pendingTxs, txIDStr)
			txWrapper.Status = TxStatusMining
			txWrapper.ReceivedAt = time.Now()
			p.eventSink.OnTxAdded(txWrapper)
			marked++
			p.logger.Debugf("⛏️ [交易池] 交易已标记为挖矿中: %x", txID)
		} else {
			notPending++
			p.logger.Debugf("⚠️ [交易池] 交易不是Pending状态，无法标记为挖矿中: %x", txID)
		}
	}

	p.logger.Infof("📊 [交易池] 挖矿中标记完成: 成功=%d, 未找到=%d, 非Pending=%d", marked, notFound, notPending)
	return nil
}

// ConfirmTransactions 确认交易已被打包进区块，并更新内存计数。
func (p *TxPool) ConfirmTransactions(txIDs [][]byte, blockHeight uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.quit:
		return ErrTxPoolClosed
	default:
	}

	p.logger.Infof("✅ [交易池] 开始确认 %d 个交易，区块高度: %d", len(txIDs), blockHeight)

	confirmed := 0
	notFound := 0
	totalFreedMemory := uint64(0)

	for _, txID := range txIDs {
		txIDStr := string(txID)
		txWrapper, exists := p.txs[txIDStr]
		if !exists {
			notFound++
			p.logger.Debugf("⚠️ [交易池] 要确认的交易不存在: %x", txID)
			continue
		}

		// 确认交易并清理内存池
		p.confirmedTxs[txIDStr] = struct{}{}
		delete(p.pendingTxs, txIDStr)
		delete(p.rejectedTxs, txIDStr)
		delete(p.pendingConfirmTxs, txIDStr) // 清理 pending_confirm 状态

		txWrapper.Status = TxStatusConfirmed
		txWrapper.ReceivedAt = time.Now()
		p.eventSink.OnTxConfirmed(txWrapper, blockHeight)

		// 从内存池中完全移除
		delete(p.txs, txIDStr)
		txSize := calculateTransactionSize(txWrapper.Tx)
		if p.memoryUsage >= txSize {
			p.memoryUsage -= txSize
			totalFreedMemory += txSize
		}

		confirmed++
		p.logger.Debugf("✅ [交易池] 交易已确认并从内存池移除: %x", txID)
	}

	p.logger.Infof("📊 [交易池] 交易确认完成: 成功=%d, 未找到=%d, 释放内存=%d bytes", confirmed, notFound, totalFreedMemory)
	p.logger.Infof("📈 [交易池] 当前状态: pending=%d, mining=%d, confirmed=%d, pending_confirm=%d",
		len(p.pendingTxs), 0, len(p.confirmedTxs), len(p.pendingConfirmTxs))

	return nil
}

// RejectTransactions 拒绝交易（挖矿失败时恢复 pending）。
func (p *TxPool) RejectTransactions(txIDs [][]byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.quit:
		return ErrTxPoolClosed
	default:
	}
	for _, txID := range txIDs {
		txIDStr := string(txID)
		txWrapper, exists := p.txs[txIDStr]
		if !exists {
			continue
		}
		if txWrapper.Status == TxStatusMining {
			p.pendingTxs[txIDStr] = struct{}{}
			txWrapper.Status = TxStatusPending
			txWrapper.ReceivedAt = time.Now()
			txWrapper.Priority = int32(p.calculateTransactionPriority(txWrapper))
			heap.Push(p.pendingQueue, txWrapper)
			p.eventSink.OnTxRemoved(txWrapper)
		}
	}
	return nil
}

// MarkTransactionsAsPendingConfirm 标记交易为待确认状态
// 用于挖出区块后，等待网络确认期间的状态管理
func (p *TxPool) MarkTransactionsAsPendingConfirm(txIDs [][]byte, blockHeight uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.quit:
		return ErrTxPoolClosed
	default:
	}

	p.logger.Infof("🔄 [交易池] 开始标记 %d 个交易为待确认状态，区块高度: %d", len(txIDs), blockHeight)

	marked := 0
	notFound := 0
	wrongStatus := 0

	for _, txID := range txIDs {
		txIDStr := string(txID)
		txWrapper, exists := p.txs[txIDStr]
		if !exists {
			notFound++
			p.logger.Debugf("⚠️ [交易池] 交易不存在，无法标记为待确认: %x", txID)
			continue
		}

		// 只有mining状态的交易才能转为pending_confirm
		if txWrapper.Status == TxStatusMining {
			// 添加到pending_confirm
			p.pendingConfirmTxs[txIDStr] = struct{}{}
			txWrapper.Status = TxStatusPendingConfirm
			marked++
			p.logger.Debugf("✅ [交易池] 交易已标记为待确认: %x", txID)
		} else {
			wrongStatus++
			p.logger.Debugf("⚠️ [交易池] 交易状态不是Mining，无法标记为待确认: %x, 当前状态: %v", txID, txWrapper.Status)
		}
	}

	p.logger.Infof("📊 [交易池] 待确认标记完成: 成功=%d, 未找到=%d, 状态错误=%d", marked, notFound, wrongStatus)
	return nil
}

// 确保TxPool实现了ExtendedTxPool接口
var _ ExtendedTxPool = (*TxPool)(nil)

// Start 启动交易池（生命周期适配）。
func (tp *TxPool) Start(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	go tp.maintenanceLoop()
	return nil
}

// Stop 停止交易池（生命周期适配）。
func (tp *TxPool) Stop() error {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.logger.Infof("交易池已停止")
	return nil
}

// ==================== UTXO冲突检测辅助方法（基于历史实现） ====================

// equalOutPoint 比较两个OutPoint是否相等
func (p *TxPool) equalOutPoint(op1, op2 *transaction.OutPoint) bool {
	if op1 == nil || op2 == nil {
		return false
	}
	if len(op1.TxId) != len(op2.TxId) {
		return false
	}
	if op1.OutputIndex != op2.OutputIndex {
		return false
	}
	for i := range op1.TxId {
		if op1.TxId[i] != op2.TxId[i] {
			return false
		}
	}
	return true
}

// makeOutPointKey 生成OutPoint的唯一键，用于冲突检测
// 使用统一的 utils.OutPointKey 确保格式一致性
func (p *TxPool) makeOutPointKey(op *transaction.OutPoint) string {
	return utils.OutPointKey(op)
}

// 存储一致性、优先级与大小估算等辅助方法见同目录其他文件。

// GetTransactionByID 根据交易ID获取交易
func (p *TxPool) GetTransactionByID(txID []byte) (*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txIDStr := string(txID)
	if txWrapper, exists := p.txs[txIDStr]; exists {
		return txWrapper.Tx, nil
	}

	return nil, nil // 交易不存在，返回nil而不是错误
}

// GetPendingTransactions 获取所有待处理交易
func (p *TxPool) GetPendingTransactions() ([]*transaction.Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var pendingTxs []*transaction.Transaction
	for txID := range p.pendingTxs {
		if txWrapper, exists := p.txs[txID]; exists {
			pendingTxs = append(pendingTxs, txWrapper.Tx)
		}
	}

	if p.logger != nil {
		p.logger.Debugf("返回 %d 个待处理交易", len(pendingTxs))
	}

	return pendingTxs, nil
}

// ==================== 🔧 辅助方法 ====================

// ❌ 已删除 extractExecutionFeePrice - 执行费用价格提取应该在Transaction Domain的FeeService中处理
// ✅ 将在阶段2中通过注入的FeeService.EstimateFee()方法实现

// ==================== 🔧 核心验证方法 ====================
// ❌ 已删除 validateTransactionBasic - 交易验证应该在Transaction Domain中处理
// ✅ TxPool现在专注于存储容器职责，不处理业务逻辑验证

// ❌ 已删除 hasUTXOConflict - UTXO冲突检测应该在UTXO Domain中处理
// ✅ 将在阶段2中通过注入的UTXOService.DetectConflicts()方法实现

// ==================== 🔧 生产级淘汰策略 ====================

// executeEvictionStrategy 执行基于优先级和时间的生产级淘汰策略
// 策略：优先淘汰低优先级、长时间停留的交易
func (p *TxPool) executeEvictionStrategy(candidates []*TxWrapper, requiredSpace uint64) int {
	if len(candidates) == 0 {
		return 0
	}

	// 按淘汰优先级排序（优先级低、时间久的排在前面）
	sort.Slice(candidates, func(i, j int) bool {
		// 1. 优先按FeeRate排序（低费率先淘汰）
		if candidates[i].Size != candidates[j].Size {
			return candidates[i].Size < candidates[j].Size
		}

		// 2. FeeRate相同时按时间排序（老交易先淘汰）
		return candidates[i].ReceivedAt.Before(candidates[j].ReceivedAt)
	})

	evictedCount := 0
	freedSpace := uint64(0)

	// 逐个淘汰直到释放足够空间
	for _, wrapper := range candidates {
		if freedSpace >= requiredSpace {
			break
		}

		txIDStr := string(wrapper.TxID)

		// 从存储中移除
		if _, exists := p.txs[txIDStr]; exists {
			// 计算释放的空间（基于交易复杂度的生产级估算）
			txSize := p.estimateTransactionSize(wrapper.Tx)
			if txSize == 0 {
				txSize = 500 // 保底默认大小
			}

			// 执行移除
			delete(p.txs, txIDStr)
			delete(p.pendingTxs, txIDStr)

			// 从优先级队列中移除
			p.pendingQueue.Remove(wrapper)

			// 更新内存使用量
			if p.memoryUsage >= txSize {
				p.memoryUsage -= txSize
			}

			freedSpace += txSize
			evictedCount++

			// 记录淘汰事件
			if p.logger != nil {
				p.logger.Debugf("淘汰低优先级交易: txID=%x, 0=%d, age=%v",
					wrapper.TxID, wrapper.Size, time.Since(wrapper.ReceivedAt))
			}

			// 发布淘汰事件
			p.eventSink.OnTxRemoved(wrapper)
		}
	}

	if p.logger != nil {
		p.logger.Infof("淘汰策略执行完成: 淘汰%d个交易，释放%d字节空间", evictedCount, freedSpace)
	}

	return evictedCount
}

// calculateTransactionPriority 计算交易优先级（生产级算法）
// 基于费率、交易大小和接收时间的综合评分
func (p *TxPool) calculateTransactionPriority(wrapper *TxWrapper) uint64 {
	if wrapper == nil {
		return 0
	}

	// 基础分数：费率权重60%
	feeScore := wrapper.Size * 60 / 100

	// 时间分数：较新的交易优先级更高，权重30%
	ageSeconds := uint64(time.Since(wrapper.ReceivedAt).Seconds())
	timeScore := uint64(0)
	if ageSeconds < 3600 { // 1小时内的交易
		timeScore = (3600 - ageSeconds) * 30 / 3600 / 100
	}

	// 大小分数：较小的交易优先级更高，权重10%
	sizeScore := uint64(0)
	txSize := p.estimateTransactionSize(wrapper.Tx)
	if txSize > 0 && txSize < 10000 { // 小于10KB的交易
		sizeScore = (10000 - txSize) * 10 / 10000 / 100
	}

	// 综合优先级分数
	totalPriority := feeScore + timeScore + sizeScore

	// 确保最小优先级为1
	if totalPriority == 0 {
		totalPriority = 1
	}

	return totalPriority
}

// estimateTransactionSize 基于交易复杂度估算交易大小（生产级方法）
func (p *TxPool) estimateTransactionSize(tx *transaction.Transaction) uint64 {
	if tx == nil {
		return 500 // 默认大小
	}

	// 基础交易结构大小
	baseSize := uint64(100)

	// 输入数量影响（每个输入约200字节）
	inputSize := uint64(len(tx.Inputs)) * 200

	// 输出数量影响（每个输出约100字节）
	outputSize := uint64(len(tx.Outputs)) * 100

	// 元数据影响
	metadataSize := uint64(0)
	if tx.Metadata != nil {
		// 标准元数据大小估算
		metadataSize = 50
	}

	totalSize := baseSize + inputSize + outputSize + metadataSize

	// 确保合理的大小范围
	if totalSize < 200 {
		totalSize = 200
	}
	if totalSize > 10000 {
		totalSize = 10000
	}

	return totalSize
}

// calcTxID 计算交易ID
// ✅ 使用crypto模块的哈希服务，避免与blockchain模块循环依赖
func (p *TxPool) calcTxID(tx *transaction.Transaction) ([]byte, error) {
	if p.hashService == nil {
		return nil, fmt.Errorf("交易哈希服务未初始化")
	}

	// 使用crypto模块提供的TransactionHashService
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	resp, err := p.hashService.ComputeHash(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %w", err)
	}

	if resp == nil || !resp.IsValid {
		return nil, fmt.Errorf("计算交易哈希返回无效结果")
	}

	return resp.Hash, nil
}
