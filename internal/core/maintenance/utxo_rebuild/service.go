package utxo_rebuild

import (
	"context"
	"fmt"

	queryinterfaces "github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	eutxointerfaces "github.com/weisyn/v1/internal/core/eutxo/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storage "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// RebuildStats 描述一次全量 UTXO 重建的统计信息
type RebuildStats struct {
	StartHeight    uint64
	EndHeight      uint64
	ProcessedBlocks uint64
	FailedBlocks    uint64
	CreatedUTXOs    uint64
	DeletedUTXOs    uint64
}

// Service 提供“清空全量 UTXO + 按区块重放重建 UTXO 集合”的重型维护能力
//
// 设计边界：
// - 只在人工明确触发的维护场景下使用（例如离线运维 CLI）
// - 不在节点正常运行路径或 FX 自动控制器中调用
// - 依赖现有 BlockQuery 和 InternalUTXOWriter，确保与在线写路径语义一致
type Service struct {
	store       storage.BadgerStore
	blockQuery  queryinterfaces.InternalBlockQuery
	utxoWriter  eutxointerfaces.InternalUTXOWriter
	txHashClient transaction.TransactionHashServiceClient
	logger      log.Logger
}

// NewService 创建全量 UTXO 重建服务
func NewService(
	store storage.BadgerStore,
	blockQuery queryinterfaces.InternalBlockQuery,
	utxoWriter eutxointerfaces.InternalUTXOWriter,
	txHashClient transaction.TransactionHashServiceClient,
	logger log.Logger,
) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("BadgerStore 不能为空")
	}
	if blockQuery == nil {
		return nil, fmt.Errorf("blockQuery 不能为空")
	}
	if utxoWriter == nil {
		return nil, fmt.Errorf("utxoWriter 不能为空")
	}
	if txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 不能为空")
	}

	return &Service{
		store:        store,
		blockQuery:   blockQuery,
		utxoWriter:   utxoWriter,
		txHashClient: txHashClient,
		logger:       logger,
	}, nil
}

// RunFullUTXORebuild 清空现有 UTXO 集合与索引，并按区块高度重放重建 UTXO 集（资产/资源/状态）
//
// 参数：
//   - startHeight: 起始高度（包含），0 表示从高度 1 开始
//   - endHeight:   结束高度（包含），0 表示自动使用当前最高高度
//   - dryRun:      为 true 时不执行实际写入，仅统计将要处理的区块和 UTXO 数量
//
// 注意：
// - 这是一个“重型维护操作”，应在节点停止后、运维明确确认的前提下执行
// - 执行前会清空：
//   - utxo:set:*                  （UTXO 集合）
//   - index:address:* / index:asset:* / index:height:* （UTXO 索引）
//   - ref:*                       （引用计数与引用关系）
func (s *Service) RunFullUTXORebuild(ctx context.Context, startHeight, endHeight uint64, dryRun bool) (*RebuildStats, error) {
	stats := &RebuildStats{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	// 自动确定结束高度
	if endHeight == 0 {
		h, _, err := s.blockQuery.GetHighestBlock(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取最高区块高度失败: %w", err)
		}
		if h == 0 {
			if s.logger != nil {
				s.logger.Warn("链为空或仅有创世块，全量 UTXO 重建无需执行")
			}
			stats.EndHeight = 0
			return stats, nil
		}
		endHeight = h
		stats.EndHeight = endHeight
	}

	if startHeight == 0 {
		startHeight = 1 // 跳过创世块
		stats.StartHeight = startHeight
	}

	if endHeight < startHeight {
		return nil, fmt.Errorf("结束高度小于起始高度: start=%d, end=%d", startHeight, endHeight)
	}

	if s.logger != nil {
		s.logger.Warnf("⚠️ 准备执行全量 UTXO 重建: [%d, %d], dryRun=%v", startHeight, endHeight, dryRun)
	}

	// 1. 清空现有 UTXO 状态（仅在非 dryRun 模式）
	if !dryRun {
		if err := s.clearUTXOState(ctx); err != nil {
			return nil, fmt.Errorf("清空现有 UTXO 状态失败: %w", err)
		}
		if s.logger != nil {
			s.logger.Info("✅ 已清空现有 UTXO 集合、索引与引用相关键，开始按区块重放重建 UTXO")
		}
	} else if s.logger != nil {
		s.logger.Warn("DRY-RUN: 将跳过实际清空 UTXO 状态，仅模拟统计将要处理的区块与 UTXO 数量")
	}

	// 2. 按高度重放区块，重建 UTXO 集
	for h := startHeight; h <= endHeight; h++ {
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Warnf("全量 UTXO 重建在高度 %d 被取消: %v", h, ctx.Err())
			}
			return stats, ctx.Err()
		default:
		}

		block, err := s.blockQuery.GetBlockByHeight(ctx, h)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("读取区块失败，跳过: height=%d, error=%v", h, err)
			}
			stats.FailedBlocks++
			continue
		}
		if block == nil || block.Body == nil || len(block.Body.Transactions) == 0 {
			continue
		}

		if err := s.rebuildUTXOForBlock(ctx, block, stats, dryRun); err != nil {
			if s.logger != nil {
				s.logger.Errorf("重建区块 UTXO 状态失败: height=%d, error=%v", h, err)
			}
			stats.FailedBlocks++
			continue
		}

		stats.ProcessedBlocks++
	}

	if s.logger != nil {
		s.logger.Infof("✅ 全量 UTXO 重建完成: blocks=%d, failedBlocks=%d, createdUTXOs=%d, deletedUTXOs=%d",
			stats.ProcessedBlocks, stats.FailedBlocks, stats.CreatedUTXOs, stats.DeletedUTXOs)
	}

	return stats, nil
}

// clearUTXOState 清空现有 UTXO 集合、索引与引用相关键
func (s *Service) clearUTXOState(ctx context.Context) error {
	// 8. 删除当前所有 UTXO（通过前缀扫描和批量删除）
	utxoPrefix := []byte("utxo:set:")
	utxoMap, err := s.store.PrefixScan(ctx, utxoPrefix)
	if err != nil {
		return fmt.Errorf("扫描现有 UTXO 失败: %w", err)
	}

	if len(utxoMap) > 0 {
		keysToDelete := make([][]byte, 0, len(utxoMap))
		for key := range utxoMap {
			keysToDelete = append(keysToDelete, []byte(key))
		}
		if err := s.store.DeleteMany(ctx, keysToDelete); err != nil {
			return fmt.Errorf("清空 UTXO 失败: %w", err)
		}
		if s.logger != nil {
			s.logger.Infof("已删除 %d 个现有 UTXO (utxo:set:*)", len(keysToDelete))
		}
	}

	// 删除所有 UTXO 索引（地址 / 高度 / 资产）
	indexPrefixes := [][]byte{
		[]byte("index:address:"),
		[]byte("index:height:"),
		[]byte("index:asset:"),
	}

	for _, prefix := range indexPrefixes {
		indexMap, err := s.store.PrefixScan(ctx, prefix)
		if err != nil {
			return fmt.Errorf("扫描索引前缀 %s 失败: %w", string(prefix), err)
		}
		if len(indexMap) == 0 {
			continue
		}

		indexKeys := make([][]byte, 0, len(indexMap))
		for key := range indexMap {
			indexKeys = append(indexKeys, []byte(key))
		}

		if err := s.store.DeleteMany(ctx, indexKeys); err != nil {
			return fmt.Errorf("清空索引前缀 %s 失败: %w", string(prefix), err)
		}
		if s.logger != nil {
			s.logger.Infof("已删除 %d 个索引键（前缀=%s）", len(indexKeys), string(prefix))
		}
	}

	// 删除所有引用计数与引用关系键：ref:{...}
	refPrefix := []byte("ref:")
	refMap, err := s.store.PrefixScan(ctx, refPrefix)
	if err != nil {
		return fmt.Errorf("扫描引用前缀 ref: 失败: %w", err)
	}
	if len(refMap) > 0 {
		refKeys := make([][]byte, 0, len(refMap))
		for key := range refMap {
			refKeys = append(refKeys, []byte(key))
		}
		if err := s.store.DeleteMany(ctx, refKeys); err != nil {
			return fmt.Errorf("清空引用相关键失败: %w", err)
		}
		if s.logger != nil {
			s.logger.Infof("已删除 %d 个引用相关键（前缀=ref:）", len(refKeys))
		}
	}

	return nil
}

// rebuildUTXOForBlock 基于区块数据重建单个区块的 UTXO 状态
func (s *Service) rebuildUTXOForBlock(
	ctx context.Context,
	block *core.Block,
	stats *RebuildStats,
	dryRun bool,
) error {
	if block == nil || block.Body == nil {
		return nil
	}

	// 预计算所有交易哈希，确保与线上写路径一致（使用 TransactionHashServiceClient）
	txHashes := make([][]byte, len(block.Body.Transactions))
	for i, txProto := range block.Body.Transactions {
		if txProto == nil {
			continue
		}

		req := &transaction.ComputeHashRequest{Transaction: txProto}
		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return fmt.Errorf("计算交易哈希失败（height=%d, txIndex=%d）: %w", block.Header.Height, i, err)
		}
		if !resp.IsValid || len(resp.Hash) == 0 {
			return fmt.Errorf("交易哈希无效（height=%d, txIndex=%d）", block.Header.Height, i)
		}
		txHashes[i] = resp.Hash
	}

	// 1. 先处理所有输入：删除被消费的 UTXO
	for i, txProto := range block.Body.Transactions {
		if txProto == nil {
			continue
		}

		for _, input := range txProto.Inputs {
			if input == nil || input.PreviousOutput == nil {
				continue
			}

			// 引用型输入（is_reference_only=true）不消费 UTXO，此处只处理消费型输入
			if input.IsReferenceOnly {
				continue
			}

			if dryRun {
				stats.DeletedUTXOs++
				continue
			}

			if err := s.utxoWriter.DeleteUTXO(ctx, input.PreviousOutput); err != nil {
				return fmt.Errorf("删除 UTXO 失败（height=%d, txIndex=%d）: %w", block.Header.Height, i, err)
			}
			stats.DeletedUTXOs++
		}
	}

	// 2. 再处理所有输出：创建新的 UTXO
	for i, txProto := range block.Body.Transactions {
		if txProto == nil {
			continue
		}

		txHash := txHashes[i]
		if len(txHash) == 0 {
			continue
		}

		for j, output := range txProto.Outputs {
			if output == nil {
				continue
			}

			// 判定 UTXO 类型
			var category utxopb.UTXOCategory
			switch {
			case output.GetAsset() != nil:
				category = utxopb.UTXOCategory_UTXO_CATEGORY_ASSET
			case output.GetResource() != nil:
				category = utxopb.UTXOCategory_UTXO_CATEGORY_RESOURCE
			case output.GetState() != nil:
				category = utxopb.UTXOCategory_UTXO_CATEGORY_STATE
			default:
				category = utxopb.UTXOCategory_UTXO_CATEGORY_UNKNOWN
			}

			utxoObj := &utxopb.UTXO{
				Outpoint: &transaction.OutPoint{
					TxId:        txHash,
					OutputIndex: uint32(j),
				},
				Category:        category,
				OwnerAddress:    output.Owner,
				BlockHeight:     block.Header.Height,
				Status:          utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
				ContentStrategy: &utxopb.UTXO_CachedOutput{CachedOutput: output},
			}

			if dryRun {
				stats.CreatedUTXOs++
				continue
			}

			if err := s.utxoWriter.CreateUTXO(ctx, utxoObj); err != nil {
				return fmt.Errorf("创建 UTXO 失败（height=%d, txIndex=%d, outputIndex=%d）: %w",
					block.Header.Height, i, j, err)
			}
			stats.CreatedUTXOs++
		}
	}

	return nil
}


