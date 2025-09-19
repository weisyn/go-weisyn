package utxo

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/repositories/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// UTXOService UTXO服务统一接口
// 整合UTXO客户端和同步管理功能，提供完整的UTXO管理服务
// 作为repository模块与独立的utxo存储实体的桥梁
type UTXOService struct {
	utxoManager interfaces.InternalUTXOManager // UTXO管理器接口
	storage     storage.BadgerStore            // 持久化存储
	logger      log.Logger                     // 日志服务

	// 内部组件
	client      *UTXOClient  // UTXO客户端（简化版）
	syncManager *SyncManager // 同步管理器（简化版）
}

// NewUTXOService 创建UTXO服务
func NewUTXOService(
	utxoManager interfaces.InternalUTXOManager,
	storage storage.BadgerStore,
	logger log.Logger,
) *UTXOService {
	service := &UTXOService{
		utxoManager: utxoManager,
		storage:     storage,
		logger:      logger,
	}

	// 创建简化版的内部组件
	service.client = &UTXOClient{
		utxoManager: utxoManager,
		logger:      logger,
	}

	service.syncManager = &SyncManager{
		utxoService: service,
		logger:      logger,
		status: &SyncStatus{
			IsRunning: true,
		},
	}

	return service
}

// ========== 区块同步接口 ==========

// NotifyBlockAdded 通知UTXO系统有新区块添加
func (us *UTXOService) NotifyBlockAdded(ctx context.Context, block *pb.Block) error {
	if us.logger != nil {
		us.logger.Debugf("通知UTXO系统新区块 - height: %d", block.Header.Height)
	}

	// 处理区块添加：通知UTXO管理器更新状态
	return us.processBlockAddition(ctx, block)
}

// NotifyBlockRemoved 通知UTXO系统区块被移除
func (us *UTXOService) NotifyBlockRemoved(ctx context.Context, block *pb.Block) error {
	if us.logger != nil {
		us.logger.Debugf("通知UTXO系统区块移除 - height: %d", block.Header.Height)
	}

	// 处理区块移除：通知UTXO管理器回滚状态
	return us.processBlockRemoval(ctx, block)
}

// ========== 基础查询接口 ==========

// GetUTXO 根据输出点获取UTXO（使用标准接口）
func (us *UTXOService) GetUTXO(ctx context.Context, outpoint interface{}) (interface{}, error) {
	if us.logger != nil {
		us.logger.Debugf("查询UTXO")
	}

	// 调用标准UTXO管理器接口
	// 注意：这里需要类型断言
	if outpointTyped, ok := outpoint.(*transaction.OutPoint); ok {
		return us.utxoManager.GetUTXO(ctx, outpointTyped)
	}
	return nil, fmt.Errorf("无效的outpoint类型")
}

// GetUTXOsByAddress 根据地址获取UTXO列表（使用标准接口）
func (us *UTXOService) GetUTXOsByAddress(ctx context.Context, address []byte) (interface{}, error) {
	if us.logger != nil {
		us.logger.Debugf("查询地址UTXO - address: %x", address)
	}

	// 调用标准UTXO管理器接口
	// 注意：传递nil作为category，true作为onlyAvailable
	return us.utxoManager.GetUTXOsByAddress(ctx, address, nil, true)
}

// ========== 区块处理接口 ==========

// ProcessBlockUTXOs 处理区块中的UTXO变更（代理调用）
func (us *UTXOService) ProcessBlockUTXOs(ctx context.Context, tx storage.BadgerTransaction, block *pb.Block, blockHash []byte, txHashes [][]byte) error {
	if us.logger != nil {
		us.logger.Debugf("UTXOService代理处理区块UTXO - height: %d", block.Header.Height)
	}

	// 代理调用内部UTXO管理器
	return us.utxoManager.ProcessBlockUTXOs(ctx, tx, block, blockHash, txHashes)
}

// ========== 引用管理接口 ==========

// ReferenceUTXO 引用UTXO（增加引用计数）
func (us *UTXOService) ReferenceUTXO(ctx context.Context, outpoint interface{}) error {
	if us.logger != nil {
		us.logger.Debugf("引用UTXO")
	}

	// 类型断言
	if outpointTyped, ok := outpoint.(*transaction.OutPoint); ok {
		return us.utxoManager.ReferenceUTXO(ctx, outpointTyped)
	}
	return fmt.Errorf("无效的outpoint类型")
}

// UnreferenceUTXO 解除UTXO引用（减少引用计数）
func (us *UTXOService) UnreferenceUTXO(ctx context.Context, outpoint interface{}) error {
	if us.logger != nil {
		us.logger.Debugf("解除UTXO引用")
	}

	// 类型断言
	if outpointTyped, ok := outpoint.(*transaction.OutPoint); ok {
		return us.utxoManager.UnreferenceUTXO(ctx, outpointTyped)
	}
	return fmt.Errorf("无效的outpoint类型")
}

// ========== 状态查询接口 ==========

// GetCurrentStateRoot 获取当前UTXO状态根
func (us *UTXOService) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	if us.logger != nil {
		us.logger.Debugf("查询UTXO状态根")
	}

	return us.utxoManager.GetCurrentStateRoot(ctx)
}

// GetSyncStatus 获取同步状态
func (us *UTXOService) GetSyncStatus() *SyncStatus {
	return us.syncManager.GetStatus()
}

// ========== 健康检查接口 ==========

// CheckHealth 检查UTXO服务健康状态
func (us *UTXOService) CheckHealth(ctx context.Context) error {
	if us.logger != nil {
		us.logger.Debugf("检查UTXO服务健康状态")
	}

	// 简化健康检查：验证UTXO管理器是否可用
	_, err := us.utxoManager.GetCurrentStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("UTXO服务健康检查失败: %w", err)
	}

	if us.logger != nil {
		us.logger.Debugf("UTXO服务健康状态正常")
	}

	return nil
}

// ========== 内部处理方法 ==========

// processBlockAddition 处理区块添加
func (us *UTXOService) processBlockAddition(ctx context.Context, block *pb.Block) error {
	// 处理区块添加：分析所有交易并更新同步状态
	// 实际的UTXO状态管理由独立的UTXO存储实体负责

	if us.logger != nil {
		us.logger.Debugf("处理区块添加 - height: %d, tx_count: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	// 更新同步状态
	us.syncManager.status.LastSyncHeight = block.Header.Height
	us.syncManager.status.ProcessedBlocks++

	// 分析区块中的交易，统计UTXO变化
	txCount := len(block.Body.Transactions)
	if txCount > 0 && us.logger != nil {
		us.logger.Debugf("区块包含 %d 个交易，UTXO状态将由外部UTXO管理器处理", txCount)
	}

	if us.logger != nil {
		us.logger.Debugf("区块添加处理完成 - height: %d", block.Header.Height)
	}

	return nil
}

// processBlockRemoval 处理区块移除
func (us *UTXOService) processBlockRemoval(ctx context.Context, block *pb.Block) error {
	// 处理区块移除：回滚交易并更新同步状态

	if us.logger != nil {
		us.logger.Debugf("处理区块移除 - height: %d", block.Header.Height)
	}

	// 更新同步状态
	if us.syncManager.status.ProcessedBlocks > 0 {
		us.syncManager.status.ProcessedBlocks--
	}

	// 分析需要回滚的交易
	txCount := len(block.Body.Transactions)
	if txCount > 0 && us.logger != nil {
		us.logger.Debugf("需要回滚 %d 个交易，UTXO状态将由外部UTXO管理器处理", txCount)
	}

	if us.logger != nil {
		us.logger.Debugf("区块移除处理完成 - height: %d", block.Header.Height)
	}

	return nil
}

// ========== 简化的内部组件定义 ==========

// UTXOClient 简化版UTXO客户端
type UTXOClient struct {
	utxoManager interfaces.InternalUTXOManager
	logger      log.Logger
}

// SyncManager 简化版同步管理器
type SyncManager struct {
	utxoService *UTXOService
	logger      log.Logger
	status      *SyncStatus
}

// GetStatus 获取同步状态
func (sm *SyncManager) GetStatus() *SyncStatus {
	// 返回状态副本
	return &SyncStatus{
		IsRunning:       sm.status.IsRunning,
		LastSyncHeight:  sm.status.LastSyncHeight,
		ProcessedBlocks: sm.status.ProcessedBlocks,
		FailedBlocks:    sm.status.FailedBlocks,
		PendingTasks:    sm.status.PendingTasks,
	}
}

// SyncStatus 同步状态（简化版）
type SyncStatus struct {
	IsRunning       bool   `json:"is_running"`       // 是否正在运行
	LastSyncHeight  uint64 `json:"last_sync_height"` // 最后同步的区块高度
	ProcessedBlocks uint64 `json:"processed_blocks"` // 已处理区块数量
	FailedBlocks    uint64 `json:"failed_blocks"`    // 失败区块数量
	PendingTasks    int    `json:"pending_tasks"`    // 待处理任务数量
}
