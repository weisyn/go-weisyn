package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	eutxoiface "github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// BuildClearPlan 构建“清空当前 UTXO/索引/引用关系”的删除计划（事务外预收集）。
//
// 实现 eutxo.UTXOSnapshot.BuildClearPlan
func (s *Service) BuildClearPlan(ctx context.Context) (*eutxoiface.UTXOClearPlan, error) {
	if s == nil || s.storage == nil {
		return nil, fmt.Errorf("storage 未注入")
	}
	collect := func(prefix []byte) ([][]byte, error) {
		m, err := s.storage.PrefixScan(ctx, prefix)
		if err != nil {
			return nil, err
		}
		keys := make([][]byte, 0, len(m))
		for k := range m {
			keys = append(keys, []byte(k))
		}
		return keys, nil
	}

	utxoKeys, err := collect([]byte("utxo:set:"))
	if err != nil {
		return nil, fmt.Errorf("扫描UTXO失败: %w", err)
	}
	addrKeys, err := collect([]byte("index:address:"))
	if err != nil {
		return nil, fmt.Errorf("扫描地址索引失败: %w", err)
	}
	heightKeys, err := collect([]byte("index:height:"))
	if err != nil {
		return nil, fmt.Errorf("扫描高度索引失败: %w", err)
	}
	assetKeys, err := collect([]byte("index:asset:"))
	if err != nil {
		return nil, fmt.Errorf("扫描资产索引失败: %w", err)
	}
	refKeys, err := collect([]byte("ref:"))
	if err != nil {
		return nil, fmt.Errorf("扫描引用前缀失败: %w", err)
	}

	return &eutxoiface.UTXOClearPlan{
		UTXOKeys:         utxoKeys,
		IndexAddressKeys: addrKeys,
		IndexHeightKeys:  heightKeys,
		IndexAssetKeys:   assetKeys,
		RefKeys:          refKeys,
	}, nil
}

// RestoreSnapshotInTransaction 在已有 BadgerTransaction 中恢复快照（严格原子写入）。
//
// 实现 eutxo.UTXOSnapshot.RestoreSnapshotInTransaction
func (s *Service) RestoreSnapshotInTransaction(
	ctx context.Context,
	tx storage.BadgerTransaction,
	snapshot *types.UTXOSnapshotData,
	payload *eutxoiface.UTXOSnapshotPayload,
	clearPlan *eutxoiface.UTXOClearPlan,
) error {
	if s == nil || s.writer == nil {
		return fmt.Errorf("UTXOWriter 未注入，无法恢复快照")
	}
	if tx == nil {
		return fmt.Errorf("transaction 不能为空")
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot 不能为空")
	}
	if payload == nil {
		return fmt.Errorf("payload 不能为空")
	}
	if clearPlan == nil {
		return fmt.Errorf("clearPlan 不能为空")
	}

	// 1) 验证快照（版本/哈希/结构）
	if err := s.ValidateSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("快照验证失败: %w", err)
	}

	// 3) 事务内清空当前 UTXO/索引/引用
	for _, k := range clearPlan.UTXOKeys {
		_ = tx.Delete(k)
	}
	for _, k := range clearPlan.IndexAddressKeys {
		_ = tx.Delete(k)
	}
	for _, k := range clearPlan.IndexHeightKeys {
		_ = tx.Delete(k)
	}
	for _, k := range clearPlan.IndexAssetKeys {
		_ = tx.Delete(k)
	}
	for _, k := range clearPlan.RefKeys {
		_ = tx.Delete(k)
	}

	// 4) 事务内写入快照中的 UTXO，并在事务内重建索引
	createdCount := 0
	repairedCount := 0
	ctxWithMode := context.WithValue(ctx, "snapshot_restore_mode", true)
	for i, raw := range payload.Utxos {
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(raw, utxoObj); err != nil {
			return fmt.Errorf("反序列化UTXO失败（快照内容损坏）: idx=%d err=%w", i, err)
		}
		// BlockHeight 修复与约束
		if utxoObj.BlockHeight == 0 && snapshot.Height > 0 {
			utxoObj.BlockHeight = snapshot.Height
			repairedCount++
		}
		if utxoObj.BlockHeight > snapshot.Height {
			return fmt.Errorf("快照恢复失败: UTXO的BlockHeight(%d)超过快照高度(%d) idx=%d",
				utxoObj.BlockHeight, snapshot.Height, i)
		}
		if err := s.writer.CreateUTXOInTransaction(ctxWithMode, tx, utxoObj); err != nil {
			return fmt.Errorf("事务内创建 UTXO 失败: idx=%d err=%w", i, err)
		}
		createdCount++
	}

	// 5) 事务内更新状态根（utxo_state_root + state:chain:root）
	if err := s.writer.UpdateStateRootInTransaction(ctx, tx, snapshot.StateRoot); err != nil {
		return fmt.Errorf("事务内更新状态根失败: %w", err)
	}
	if err := tx.Set([]byte("state:chain:root"), snapshot.StateRoot); err != nil {
		return fmt.Errorf("事务内更新 state:chain:root 失败: %w", err)
	}

	if s.logger != nil {
		if repairedCount > 0 {
			s.logger.Warnf("⚠️ 快照恢复(事务内)自动修复了%d个BlockHeight=0的UTXO", repairedCount)
		}
		s.logger.Infof("✅ 快照恢复(事务内)完成: height=%d id=%s restored_utxos=%d", snapshot.Height, snapshot.SnapshotID, createdCount)
	}
	return nil
}

// 🆕 RestoreSnapshotWithBatching 分批恢复快照（解决"Txn is too big"问题）
//
// 与 RestoreSnapshotInTransaction 不同，此方法：
// - 不接收外部事务，自己管理多个小事务
// - 将大量UTXO分批提交，避免单个事务过大
// - 适用于Fork回滚等需要恢复大量UTXO的场景
func (s *Service) RestoreSnapshotWithBatching(
	ctx context.Context,
	snapshot *types.UTXOSnapshotData,
	payload *eutxoiface.UTXOSnapshotPayload,
	clearPlan *eutxoiface.UTXOClearPlan,
) error {
	if s == nil || s.writer == nil {
		return fmt.Errorf("UTXOWriter 未注入，无法恢复快照")
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot 不能为空")
	}
	if payload == nil {
		return fmt.Errorf("payload 不能为空")
	}
	if clearPlan == nil {
		return fmt.Errorf("clearPlan 不能为空")
	}

	// 1) 验证快照（版本/哈希/结构）
	if err := s.ValidateSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("快照验证失败: %w", err)
	}

	// 🆕 动态批次控制配置
	// 考虑索引写入开销，使用智能批次大小避免 "Txn is too big" 错误
	initialBatchSize := 100 // 初始批次大小（从 500 降低到 100）
	maxBatchSize := 500     // 最大批次大小
	minBatchSize := 10      // 最小批次大小
	currentBatchSize := initialBatchSize

	totalUtxos := len(payload.Utxos)
	if s.logger != nil {
		s.logger.Infof("🔄 开始智能分批恢复快照: 总计%d个UTXO, 初始批次%d个", totalUtxos, currentBatchSize)
	}

	// 2) 第一个事务：清空现有数据
	err := s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		for _, k := range clearPlan.UTXOKeys {
			_ = tx.Delete(k)
		}
		for _, k := range clearPlan.IndexAddressKeys {
			_ = tx.Delete(k)
		}
		for _, k := range clearPlan.IndexHeightKeys {
			_ = tx.Delete(k)
		}
		for _, k := range clearPlan.IndexAssetKeys {
			_ = tx.Delete(k)
		}
		for _, k := range clearPlan.RefKeys {
			_ = tx.Delete(k)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("清空现有数据失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("🗑️  清空现有数据完成: UTXO=%d, 地址索引=%d, 高度索引=%d, 资产索引=%d, 引用=%d",
			len(clearPlan.UTXOKeys), len(clearPlan.IndexAddressKeys), len(clearPlan.IndexHeightKeys),
			len(clearPlan.IndexAssetKeys), len(clearPlan.RefKeys))
	}

	// 3) 智能分批恢复UTXO（动态调整批次大小）
	ctxWithMode := context.WithValue(ctx, "snapshot_restore_mode", true)
	createdCount := 0
	repairedCount := 0
	batchCount := 0

	for i := 0; i < totalUtxos; {
		// 计算当前批次范围
		end := i + currentBatchSize
		if end > totalUtxos {
			end = totalUtxos
		}

		batchUtxos := payload.Utxos[i:end]
		batchCount++

		// 在新事务中写入当前批次
		var txSizeExceeded bool
		var actualProcessed int
		err := s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
			// 获取事务大小估算器
			sizeEst := tx.GetSizeEstimator()

			for idx, raw := range batchUtxos {
				// 🆕 创建前检查事务大小
				if sizeEst != nil && sizeEst.IsNearLimit() {
					if s.logger != nil {
						s.logger.Warnf("⚠️ 批次%d事务接近限制(%.1f%%), 仅处理了%d/%d个UTXO",
							batchCount, sizeEst.GetUsagePercent(), idx, len(batchUtxos))
					}
					txSizeExceeded = true
					actualProcessed = idx
					// 提前结束当前批次
					break
				}

				// 反序列化
				utxoObj := &utxo.UTXO{}
				if err := proto.Unmarshal(raw, utxoObj); err != nil {
					return fmt.Errorf("反序列化UTXO失败: idx=%d err=%w", i+idx, err)
				}

				// BlockHeight修复
				if utxoObj.BlockHeight == 0 && snapshot.Height > 0 {
					utxoObj.BlockHeight = snapshot.Height
					repairedCount++
				}
				if utxoObj.BlockHeight > snapshot.Height {
					return fmt.Errorf("UTXO的BlockHeight(%d)超过快照高度(%d) idx=%d",
						utxoObj.BlockHeight, snapshot.Height, i+idx)
				}

				// 创建UTXO和索引
				if err := s.writer.CreateUTXOInTransaction(ctxWithMode, tx, utxoObj); err != nil {
					return fmt.Errorf("创建UTXO失败: idx=%d err=%w", i+idx, err)
				}
				createdCount++
				actualProcessed = idx + 1
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("批次%d恢复失败（UTXO %d-%d）: %w", batchCount, i, i+actualProcessed-1, err)
		}

		// 🆕 动态调整批次大小
		if txSizeExceeded {
			// 事务过大，减小批次
			oldSize := currentBatchSize
			currentBatchSize = max(currentBatchSize/2, minBatchSize)
			if s.logger != nil {
				s.logger.Infof("📉 批次大小调整: %d -> %d (事务接近限制)", oldSize, currentBatchSize)
			}
			// 只移动实际处理的数量
			i += actualProcessed
		} else {
			// 事务成功，尝试优化批次大小
			if batchCount > 0 && batchCount%5 == 0 && currentBatchSize < maxBatchSize {
				// 每5个批次，尝试增加批次大小
				oldSize := currentBatchSize
				currentBatchSize = min(currentBatchSize*2, maxBatchSize)
				if s.logger != nil {
					s.logger.Debugf("📈 批次大小调整: %d -> %d (优化性能)", oldSize, currentBatchSize)
				}
			}
			// 移动到下一批
			i = end
		}

		// 进度日志
		if s.logger != nil && totalUtxos > 100 {
			progress := float64(i) * 100 / float64(totalUtxos)
			s.logger.Infof("📦 批次%d完成: %d/%d (%.1f%%), 当前批次大小=%d",
				batchCount, i, totalUtxos, progress, currentBatchSize)
		}
	}

	// 4) 最后一个事务：更新状态根
	err = s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		if err := s.writer.UpdateStateRootInTransaction(ctx, tx, snapshot.StateRoot); err != nil {
			return fmt.Errorf("更新状态根失败: %w", err)
		}
		if err := tx.Set([]byte("state:chain:root"), snapshot.StateRoot); err != nil {
			return fmt.Errorf("更新state:chain:root失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("更新状态根失败: %w", err)
	}

	if s.logger != nil {
		if repairedCount > 0 {
			s.logger.Warnf("⚠️ 快照恢复自动修复了%d个BlockHeight=0的UTXO", repairedCount)
		}
		s.logger.Infof("✅ 智能分批恢复完成: height=%d id=%s 恢复UTXO=%d 总批次=%d 修复=%d",
			snapshot.Height, snapshot.SnapshotID, createdCount, batchCount, repairedCount)
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RestoreSnapshotAtomic 原子恢复快照（内部开启事务）。
//
// 实现 eutxo.UTXOSnapshot.RestoreSnapshotAtomic
func (s *Service) RestoreSnapshotAtomic(ctx context.Context, snapshot *types.UTXOSnapshotData) error {
	// 1. 检查依赖
	if s.writer == nil {
		return fmt.Errorf("UTXOWriter 未注入，无法恢复快照")
	}

	// 2. 验证快照数据
	if err := s.ValidateSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("快照验证失败: %w", err)
	}

	// 3. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 4. 加载快照数据
	snapshotKey := []byte(fmt.Sprintf("snapshot:%s", snapshot.SnapshotID))
	compressedData, err := s.storage.Get(ctx, snapshotKey)
	if err != nil {
		return fmt.Errorf("加载快照失败: %w", err)
	}
	if compressedData == nil {
		return fmt.Errorf("快照不存在: %s", snapshot.SnapshotID)
	}

	// 5. 解压缩快照数据
	gzReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("创建解压缩器失败: %w", err)
	}
	defer func() {
		if err := gzReader.Close(); err != nil {
			if s.logger != nil {
				s.logger.Warnf("关闭gzip读取器失败: %v", err)
			}
		}
	}()

	utxoData, err := io.ReadAll(gzReader)
	if err != nil {
		return fmt.Errorf("解压缩失败: %w", err)
	}

	// 6. 验证快照哈希
	calculatedHash := s.hasher.SHA256(utxoData)
	if !bytes.Equal(calculatedHash, snapshot.StateRoot) {
		return fmt.Errorf("快照哈希不匹配: 期望=%x, 实际=%x", snapshot.StateRoot, calculatedHash)
	}

	// 7. 反序列化 UTXO 列表
	var snapshotData utxoSnapshotData
	if err := json.Unmarshal(utxoData, &snapshotData); err != nil {
		return fmt.Errorf("反序列化快照失败: %w", err)
	}

	// ✅ 生产级硬门槛：旧版(Version=1)快照格式在 proto oneof 字段上无法稳定 round-trip，
	// 会导致 reorg/sync 进入“必失败”状态。这里直接拒绝并提示上层走自省修复（丢弃坏快照/重建UTXO）。
	if snapshotData.Version != 2 {
		return fmt.Errorf("不支持的快照格式版本: version=%d（需要 version=2）", snapshotData.Version)
	}

	clearPlan, err := s.BuildClearPlan(ctx)
	if err != nil {
		return err
	}
	payload, err := s.LoadSnapshotPayload(ctx, snapshot)
	if err != nil {
		return err
	}
	return s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return s.RestoreSnapshotInTransaction(ctx, tx, snapshot, payload, clearPlan)
	})
}

// LoadSnapshotPayload 加载并解码快照内容（gzip+json），并进行哈希校验与版本校验。
//
// 实现 eutxo.UTXOSnapshot.LoadSnapshotPayload
func (s *Service) LoadSnapshotPayload(ctx context.Context, snapshot *types.UTXOSnapshotData) (*eutxoiface.UTXOSnapshotPayload, error) {
	snapshotKey := []byte(fmt.Sprintf("snapshot:%s", snapshot.SnapshotID))
	compressedData, err := s.storage.Get(ctx, snapshotKey)
	if err != nil {
		return nil, fmt.Errorf("加载快照失败: %w", err)
	}
	if compressedData == nil {
		return nil, fmt.Errorf("快照不存在: %s", snapshot.SnapshotID)
	}
	gzReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("创建解压缩器失败: %w", err)
	}
	defer func() { _ = gzReader.Close() }()
	utxoData, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("解压缩失败: %w", err)
	}
	calculatedHash := s.hasher.SHA256(utxoData)
	if !bytes.Equal(calculatedHash, snapshot.StateRoot) {
		return nil, fmt.Errorf("快照哈希不匹配: 期望=%x, 实际=%x", snapshot.StateRoot, calculatedHash)
	}
	var raw utxoSnapshotData
	if err := json.Unmarshal(utxoData, &raw); err != nil {
		return nil, fmt.Errorf("反序列化快照失败: %w", err)
	}
	if raw.Version != 2 {
		return nil, fmt.Errorf("不支持的快照格式版本: version=%d（需要 version=2）", raw.Version)
	}
	return &eutxoiface.UTXOSnapshotPayload{
		Version: raw.Version,
		Utxos:   raw.Utxos,
	}, nil
}
