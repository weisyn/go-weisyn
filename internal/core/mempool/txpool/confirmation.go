// 文件说明：
// 本文件实现交易确认管理器（ConfirmationManager）与相关处理流程，
// 负责对区块确认/回滚事件进行响应，批量更新交易池状态，并与事件总线对接。
// 职责限定：仅作为事件处理与池状态桥接层；交易确认事件的最终对外发布由事件下沉统一出口承担。
package txpool

import (
	"context"
	"fmt"
	"sync"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/utils"
)

// ExtendedTxPool 扩展的交易池接口，提供内部管理方法（状态更新与优先级重算）。
type ExtendedTxPool interface {
	mempool.TxPool
	UpdateTransactionStatus(txID []byte, status mempool.TxStatus) error
	recomputePriorities()
}

// ==================== 确认管理器 ====================

// ConfirmationManager 负责监听区块确认/回滚并更新交易池状态。
// 说明：去除历史的 SubmissionCoordinator 依赖，直接通过 TxPool 提交/确认。
type ConfirmationManager struct {
	txPool   ExtendedTxPool
	eventBus event.EventBus
	logger   log.Logger

	blockHashClient core.BlockHashServiceClient
	txHashClient    transaction.TransactionHashServiceClient

	confirmationDepth uint64        // 确认深度
	batchSize         uint32        // 批处理大小
	timeout           time.Duration // 处理超时

	syncMutex           sync.RWMutex
	lastConfirmedHeight uint64
	lastConfirmedHash   []byte

	confirmationQueue chan *ConfirmationTask
	workers           []*ConfirmationWorker
	workerCount       int

	quit chan struct{}
	wg   sync.WaitGroup
}

// ConfirmationTask 确认任务。
type ConfirmationTask struct {
	BlockHeight uint64
	BlockHash   []byte
	Block       *core.Block
	TaskType    ConfirmationTaskType
	Timestamp   time.Time
	Context     context.Context
}

// ConfirmationTaskType 确认任务类型。
type ConfirmationTaskType int

const (
	TaskTypeBlockConfirmed ConfirmationTaskType = iota // 区块确认
	TaskTypeBlockReverted                              // 区块回滚
)

// ConfirmationWorker 确认处理工作器。
type ConfirmationWorker struct {
	id      int
	manager *ConfirmationManager
	quit    chan struct{}
	logger  log.Logger
}

// NewConfirmationManager 创建确认管理器实例。
func NewConfirmationManager(txPool ExtendedTxPool, eventBus event.EventBus, txHashClient transaction.TransactionHashServiceClient, logger log.Logger) *ConfirmationManager {
	return &ConfirmationManager{txPool: txPool, eventBus: eventBus, txHashClient: txHashClient, logger: logger}
}

// Start 启动确认管理器，订阅事件并启动工作线程。
func (cm *ConfirmationManager) Start() error {
	cm.logger.Info("启动确认管理器")
	for i := 0; i < cm.workerCount; i++ {
		cm.wg.Add(1)
		go cm.workers[i].run()
	}
	if cm.eventBus != nil {
		if err := cm.eventBus.Subscribe("block:confirmed", cm.handleBlockConfirmedEvent); err != nil {
			return fmt.Errorf("订阅区块确认事件失败: %w", err)
		}
		if err := cm.eventBus.Subscribe("block:reverted", cm.handleBlockRevertedEvent); err != nil {
			return fmt.Errorf("订阅区块回滚事件失败: %w", err)
		}
	}
	cm.logger.Info("确认管理器启动完成")
	return nil
}

// Stop 停止确认管理器，释放资源。
func (cm *ConfirmationManager) Stop() error {
	cm.logger.Info("停止确认管理器")
	close(cm.quit)
	for _, worker := range cm.workers {
		close(worker.quit)
	}
	cm.wg.Wait()
	cm.logger.Info("确认管理器已停止")
	return nil
}

// OnBlockConfirmed 处理区块确认。
func (cm *ConfirmationManager) OnBlockConfirmed(ctx context.Context, block *core.Block) error {
	resp, err := cm.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: block})
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash
	blockHeight := block.GetHeader().GetHeight()
	cm.logger.Infof("处理区块确认: height=%d, hash=%x", blockHeight, blockHash)
	task := &ConfirmationTask{BlockHeight: blockHeight, BlockHash: blockHash, Block: block, TaskType: TaskTypeBlockConfirmed, Timestamp: time.Now(), Context: ctx}
	select {
	case cm.confirmationQueue <- task:
		cm.logger.Debugf("区块确认任务已添加到队列: height=%d", blockHeight)
		return nil
	case <-time.After(cm.timeout):
		return fmt.Errorf("添加区块确认任务超时: height=%d", blockHeight)
	}
}

// OnBlockReverted 处理区块回滚。
func (cm *ConfirmationManager) OnBlockReverted(ctx context.Context, block *core.Block) error {
	resp, err := cm.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: block})
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash
	blockHeight := block.GetHeader().GetHeight()
	cm.logger.Infof("处理区块回滚: height=%d, hash=%x", blockHeight, blockHash)
	task := &ConfirmationTask{BlockHeight: blockHeight, BlockHash: blockHash, Block: block, TaskType: TaskTypeBlockReverted, Timestamp: time.Now(), Context: ctx}
	select {
	case cm.confirmationQueue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(cm.timeout):
		return fmt.Errorf("提交回滚任务超时")
	}
}

// processBlockConfirmation 处理区块确认任务（内部）。
func (cm *ConfirmationManager) processBlockConfirmation(ctx context.Context, task *ConfirmationTask) error {
	cm.logger.Debugf("处理区块确认任务: height=%d", task.BlockHeight)
	cm.syncMutex.Lock()
	if task.BlockHeight > cm.lastConfirmedHeight {
		cm.lastConfirmedHeight = task.BlockHeight
		cm.lastConfirmedHash = task.BlockHash
	}
	cm.syncMutex.Unlock()
	txIDs := cm.extractTransactionIDs(task.Block)
	if len(txIDs) == 0 {
		cm.logger.Debug("区块中没有交易，跳过处理")
		return nil
	}
	if err := cm.batchUpdateTransactionStatus(ctx, txIDs, mempool.TxStatusConfirmed); err != nil {
		return fmt.Errorf("批量更新交易状态失败: %w", err)
	}
	if err := cm.txPool.ConfirmTransactions(txIDs, task.BlockHeight); err != nil {
		cm.logger.Warnf("确认交易失败: %v", err)
	}
	cm.publishConfirmationEvents(txIDs, task.BlockHeight)
	cm.updatePoolAfterConfirmation(ctx)
	cm.logger.Infof("区块确认处理完成: height=%d, confirmed_txs=%d", task.BlockHeight, len(txIDs))
	return nil
}

// processBlockRevert 处理区块回滚任务（内部）。
func (cm *ConfirmationManager) processBlockRevert(ctx context.Context, task *ConfirmationTask) error {
	cm.logger.Debugf("处理区块回滚任务: height=%d", task.BlockHeight)
	transactions := cm.extractTransactions(task.Block)
	if len(transactions) == 0 {
		return nil
	}
	revertedTxs := 0
	for _, tx := range transactions {
		if utils.IsCoinbaseTx(tx) {
			continue
		}
		_, err := cm.txPool.SubmitTx(tx)
		if err != nil {
			if cm.txHashClient != nil {
				req := &transaction.ComputeHashRequest{Transaction: tx, IncludeDebugInfo: false}
				resp, hashErr := cm.txHashClient.ComputeHash(ctx, req)
				if hashErr != nil {
					cm.logger.Warnf("重新提交交易失败: hash_error=%v, submit_error=%v", hashErr, err)
				} else if resp != nil && resp.IsValid {
					cm.logger.Warnf("重新提交交易失败: %x, error: %v", resp.Hash, err)
				} else {
					cm.logger.Warnf("重新提交交易失败: invalid_hash, submit_error=%v", err)
				}
			} else {
				cm.logger.Warnf("重新提交交易失败: no_hash_client, submit_error=%v", err)
			}
			continue
		}
		revertedTxs++
	}
	cm.syncMutex.Lock()
	if task.BlockHeight <= cm.lastConfirmedHeight {
		cm.recalculateConfirmedHeight(ctx)
	}
	cm.syncMutex.Unlock()
	cm.logger.Infof("区块回滚处理完成: height=%d, reverted_txs=%d", task.BlockHeight, revertedTxs)
	return nil
}

// batchUpdateTransactionStatus 批量更新交易状态。
func (cm *ConfirmationManager) batchUpdateTransactionStatus(ctx context.Context, txIDs [][]byte, status mempool.TxStatus) error {
	batchSize := int(cm.batchSize)
	totalBatches := (len(txIDs) + batchSize - 1) / batchSize
	for i := 0; i < len(txIDs); i += batchSize {
		end := i + batchSize
		if end > len(txIDs) {
			end = len(txIDs)
		}
		batch := txIDs[i:end]
		batchIndex := i/batchSize + 1
		cm.logger.Debugf("处理交易状态更新批次 %d/%d, size=%d", batchIndex, totalBatches, len(batch))
		for _, txID := range batch {
			if err := cm.txPool.UpdateTransactionStatus(txID, status); err != nil {
				cm.logger.Warnf("更新交易状态失败: %x, error: %v", txID, err)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// extractTransactionIDs 提取区块中的交易ID。
func (cm *ConfirmationManager) extractTransactionIDs(block *core.Block) [][]byte {
	body := block.GetBody()
	if body == nil || len(body.GetTransactions()) == 0 {
		return nil
	}
	transactions := body.GetTransactions()
	txIDs := make([][]byte, 0, len(transactions))
	for _, tx := range transactions {
		if cm.txHashClient != nil {
			req := &transaction.ComputeHashRequest{Transaction: tx, IncludeDebugInfo: false}
			resp, err := cm.txHashClient.ComputeHash(context.Background(), req)
			if err != nil {
				cm.logger.Warnf("计算交易哈希失败: %v", err)
				continue
			}
			if resp != nil && resp.IsValid {
				txIDs = append(txIDs, resp.Hash)
			}
		}
	}
	return txIDs
}

// extractTransactions 提取区块中的交易。
func (cm *ConfirmationManager) extractTransactions(block *core.Block) []*transaction.Transaction {
	body := block.GetBody()
	if body == nil || len(body.GetTransactions()) == 0 {
		return nil
	}
	return body.GetTransactions()
}

// isCoinbaseTransaction 判断是否为Coinbase交易（封装 common 调用）。
func (cm *ConfirmationManager) isCoinbaseTransaction(tx *transaction.Transaction) bool {
	return utils.IsCoinbaseTx(tx)
}

// publishConfirmationEvents 发布确认事件（由 TxPool 下沉统一对外发布，此处不重复）。
func (cm *ConfirmationManager) publishConfirmationEvents(txIDs [][]byte, blockHeight uint64) {
	if cm.eventBus == nil {
		return
	}
}

// updatePoolAfterConfirmation 确认后交易池内部将自动优化（无外部干预）。
func (cm *ConfirmationManager) updatePoolAfterConfirmation(ctx context.Context) {
	cm.logger.Debug("交易确认完成，交易池将自动优化内部状态")
}

// recalculateConfirmedHeight 重新计算确认高度（简化占位）。
func (cm *ConfirmationManager) recalculateConfirmedHeight(ctx context.Context) {
	if cm.lastConfirmedHeight > 0 {
		cm.lastConfirmedHeight = cm.lastConfirmedHeight - 1
	}
}

// handleBlockConfirmedEvent 事件回调：区块确认。
func (cm *ConfirmationManager) handleBlockConfirmedEvent(data interface{}) {
	blockData, ok := data.(*core.Block)
	if !ok {
		cm.logger.Warn("收到无效的区块确认事件数据")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cm.timeout)
	defer cancel()
	if err := cm.OnBlockConfirmed(ctx, blockData); err != nil {
		cm.logger.Errorf("处理区块确认事件失败: %v", err)
	}
}

// handleBlockRevertedEvent 事件回调：区块回滚。
func (cm *ConfirmationManager) handleBlockRevertedEvent(data interface{}) {
	blockData, ok := data.(*core.Block)
	if !ok {
		cm.logger.Warn("收到无效的区块回滚事件数据")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cm.timeout)
	defer cancel()
	if err := cm.OnBlockReverted(ctx, blockData); err != nil {
		cm.logger.Errorf("处理区块回滚事件失败: %v", err)
	}
}

// run 工作线程运行函数。
func (worker *ConfirmationWorker) run() {
	defer worker.manager.wg.Done()
	worker.logger.Info("确认处理工作线程启动")
	for {
		select {
		case task := <-worker.manager.confirmationQueue:
			if err := worker.processTask(task); err != nil {
				worker.logger.Errorf("处理确认任务失败: %v", err)
			}
		case <-worker.quit:
			worker.logger.Info("确认处理工作线程退出")
			return
		}
	}
}

// processTask 处理确认任务。
func (worker *ConfirmationWorker) processTask(task *ConfirmationTask) error {
	start := time.Now()
	var err error
	switch task.TaskType {
	case TaskTypeBlockConfirmed:
		err = worker.manager.processBlockConfirmation(task.Context, task)
	case TaskTypeBlockReverted:
		err = worker.manager.processBlockRevert(task.Context, task)
	default:
		err = fmt.Errorf("未知的任务类型: %d", task.TaskType)
	}
	duration := time.Since(start)
	worker.logger.Debugf("任务处理完成: type=%d, duration=%v, error=%v", task.TaskType, duration, err)
	return err
}
