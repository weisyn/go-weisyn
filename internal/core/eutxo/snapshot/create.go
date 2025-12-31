package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"

	// core "github.com/weisyn/v1/pb/blockchain/block" // ⚠️ 已移除：不再需要 BlockHashServiceClient
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// CreateSnapshot 创建 UTXO 快照
//
// 实现 eutxo.UTXOSnapshot.CreateSnapshot
func (s *Service) CreateSnapshot(ctx context.Context, height uint64) (*types.UTXOSnapshotData, error) {
	// 1. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 注意：CreateSnapshot 使用 PrefixScan 直接扫描 storage，不依赖 query 服务
	// query 服务主要用于其他查询场景

	// ✅ height=0 特判：仅允许在链尖也为 0 时创建 genesis 快照，避免在非0高度下生成“伪 genesis 快照”
	// 背景：reorg 场景可能传入 forkHeight=0，若直接扫描当前 UTXO 集会得到“当前状态”而非 genesis 状态。
	if height == 0 {
		tipKey := []byte("state:chain:tip")
		tipData, err := s.storage.Get(ctx, tipKey)
		if err != nil {
			return nil, fmt.Errorf("读取链尖状态失败（genesis 快照校验）: %w", err)
		}
		// tipData 为空：链尚未初始化（也不应该创建 genesis 快照）
		if len(tipData) == 0 {
			return nil, fmt.Errorf("拒绝创建 height=0 快照：链尚未初始化（tip 不存在）")
		}
		if len(tipData) < 8 {
			return nil, fmt.Errorf("拒绝创建 height=0 快照：链尖数据损坏（len=%d）", len(tipData))
		}
		// tip 格式：height(8) + hash(32)
		tipHeight := bytesToUint64(tipData[:8])
		if tipHeight != 0 {
			return nil, fmt.Errorf("拒绝创建 height=0 快照：当前链尖高度=%d（仅允许在 tip=0 时创建 genesis 快照）", tipHeight)
		}
	}

	// 3. 获取所有 UTXO（通过前缀扫描）
	// 符合 docs/system/designs/storage/data-architecture.md 规范
	utxoPrefix := []byte("utxo:set:")
	utxoMap, err := s.storage.PrefixScan(ctx, utxoPrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描 UTXO 失败: %w", err)
	}

	// 4. 构建 UTXO 列表（以 proto bytes 形式保存），并统计高度范围用于基本自检
	//
	// ✅ 关键修复：不要对 protobuf 结构体（含 oneof/interface 字段）直接用 encoding/json 做 round-trip。
	// 旧实现用 json.Marshal([]*utxo.UTXO) 会在 RestoreSnapshot 时因 oneof 反序列化失败而导致 reorg/sync 失败。
	// 这里改为保存每条 UTXO 的 proto bytes（JSON 会以 base64 编码保存 [][]byte），恢复时逐条 proto.Unmarshal。
	//
	// ✅ 关键修复：对 PrefixScan 返回的 map 做排序，保证快照内容序列化顺序确定性（snapshot id/hash 稳定）。
	utxoBytes := make([][]byte, 0, len(utxoMap))
	var maxObservedHeight uint64
	var repairedCount int // ✅ 记录修复数量
	keys := make([]string, 0, len(utxoMap))
	for k := range utxoMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		utxoData := utxoMap[k]
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(utxoData, utxoObj); err != nil {
			if s.logger != nil {
				s.logger.Warnf("反序列化 UTXO 失败，跳过: %v", err)
			}
			continue
		}
		
		// ✅ 容错验证：确保 BlockHeight 字段正确
		// 如果是非创世快照（height > 0），UTXO 的 BlockHeight 不应为 0
		if height > 0 && utxoObj.BlockHeight == 0 {
			if s.logger != nil {
				s.logger.Errorf("❌ 快照创建检测到异常：UTXO的BlockHeight为0 (snapshot_height=%d)", height)
				s.logger.Errorf("   UTXO详情: outpoint=%x:%d category=%d owner=%x", 
					utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, 
					utxoObj.Category, utxoObj.OwnerAddress)
			}

			// ✅ 根据配置策略处理损坏UTXO
			switch s.config.CorruptUTXOPolicy {
			case "reject":
				// 严格模式：拒绝创建快照
			return nil, fmt.Errorf("快照创建失败: UTXO的BlockHeight为0 (snapshot_height=%d, outpoint=%x:%d)", 
				height, utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex)

			case "repair":
				// 修复模式：自动修复并继续
				if repairedCount >= s.config.MaxRepairableCount {
					return nil, fmt.Errorf("修复UTXO数量超过限制: %d (outpoint=%x:%d)", 
						s.config.MaxRepairableCount, utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex)
		}
		
				// 修复BlockHeight字段
				utxoObj.BlockHeight = height
				repairedCount++

				// 重新序列化
				repairedData, err := proto.Marshal(utxoObj)
				if err != nil {
					if s.logger != nil {
						s.logger.Errorf("重新序列化UTXO失败: %v", err)
					}
					return nil, fmt.Errorf("重新序列化修复后的UTXO失败: %w", err)
				}

				// 持久化修复（写回数据库）
				utxoKey := buildUTXOKey(utxoObj.Outpoint)
				if err := s.storage.Set(ctx, []byte(utxoKey), repairedData); err != nil {
					if s.logger != nil {
						s.logger.Errorf("写回修复后的UTXO失败: %v", err)
					}
					return nil, fmt.Errorf("持久化修复后的UTXO失败: %w", err)
				}

				// 使用修复后的数据
				utxoData = repairedData

				if s.logger != nil {
					s.logger.Warnf("✅ 已自动修复UTXO BlockHeight: outpoint=%x:%d, new_height=%d",
						utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, height)
				}

			case "warn":
				// 告警模式：记录日志但继续（使用快照高度）
				utxoObj.BlockHeight = height
				repairedCount++

				repairedData, err := proto.Marshal(utxoObj)
				if err != nil {
					if s.logger != nil {
						s.logger.Errorf("重新序列化UTXO失败: %v", err)
					}
					continue
				}

				utxoKey := buildUTXOKey(utxoObj.Outpoint)
				if err := s.storage.Set(ctx, []byte(utxoKey), repairedData); err != nil {
					if s.logger != nil {
						s.logger.Warnf("写回修复后的UTXO失败，跳过: %v", err)
					}
					continue
				}

				utxoData = repairedData

				if s.logger != nil {
					s.logger.Warnf("⚠️ 检测到损坏UTXO，使用快照高度修复: outpoint=%x:%d, height=%d",
						utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, height)
				}

			default:
				// 默认：使用repair策略
				utxoObj.BlockHeight = height
				repairedCount++

				repairedData, _ := proto.Marshal(utxoObj)
				utxoKey := buildUTXOKey(utxoObj.Outpoint)
				s.storage.Set(ctx, []byte(utxoKey), repairedData)
				utxoData = repairedData

				if s.logger != nil {
					s.logger.Warnf("⚠️ 使用默认策略修复UTXO: outpoint=%x:%d, height=%d",
						utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex, height)
				}
			}
		}
		
		// ✅ REORG容错：UTXO 的 BlockHeight 超过快照高度时，跳过该UTXO
		// 场景：REORG回滚时，当前链在高度H2，需要创建高度H1的快照（H1 < H2）
		// 此时UTXO集中会包含H1到H2之间的UTXO，这些UTXO不应包含在H1的快照中
		if utxoObj.BlockHeight > height {
			if s.logger != nil {
				s.logger.Debugf("⏭️ 跳过未来UTXO: BlockHeight(%d) > snapshot_height(%d), outpoint=%x:%d", 
					utxoObj.BlockHeight, height, utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex)
			}
			// 跳过该UTXO，继续处理下一个
			continue
		}
		
		// 保存原始 bytes，避免 protobuf oneof 的 JSON 兼容性问题
		utxoBytes = append(utxoBytes, utxoData)
		if utxoObj.BlockHeight > maxObservedHeight {
			maxObservedHeight = utxoObj.BlockHeight
		}
	}

	// 4.1 基本高度一致性自检（非致命，仅日志告警）
	// 如果调用方传入的快照高度大于当前 UTXO 集中的最大高度，通常意味着在链尚未到达该高度时创建了快照。
	if maxObservedHeight > 0 && height > maxObservedHeight {
		if s.logger != nil {
			s.logger.Warnf("⚠️ CreateSnapshot: 传入快照高度=%d 大于当前 UTXO 集最大高度=%d，这可能表示在链尚未达到该高度时创建了快照（仅记录告警，不阻止创建）",
				height, maxObservedHeight)
		}
	}

	// 5. 序列化快照（JSON + gzip）
	// 注意：此处 JSON 仅用于封装结构；UTXO 本体为 proto bytes（[][]byte 自动 base64 编码），可稳定 round-trip。
	utxoData, err := json.Marshal(utxoSnapshotData{
		Version: 2,
		Utxos:   utxoBytes,
		Height:  height,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化 UTXO 失败: %w", err)
	}

	// 6. 计算快照哈希
	snapshotHash := s.hasher.SHA256(utxoData)

	// 7. 压缩快照数据（gzip）
	var compressedBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&compressedBuf)
	if _, err := gzWriter.Write(utxoData); err != nil {
		gzWriter.Close()
		return nil, fmt.Errorf("压缩快照数据失败: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("关闭压缩器失败: %w", err)
	}
	compressedData := compressedBuf.Bytes()

	// 8. 生成快照 ID
	snapshotID := fmt.Sprintf("snapshot_%d_%x", height, snapshotHash[:8])

	// 9. 区块哈希（可选）
	// ⚠️ **架构修复**：EUTXO 模块不应依赖 persistence 模块
	// 区块哈希应该由调用方（CHAIN 层的 ForkHandler）提供
	// 这里使用 nil，调用方可以在创建快照后手动设置 blockHash（如果需要）
	var blockHash *transaction.Hash
	if s.logger != nil {
		s.logger.Debugf("快照创建时 blockHash 为 nil（应由调用方提供，如果需要）")
	}

	// 10. 存储快照数据（使用 BadgerStore）
	snapshotKey := []byte(fmt.Sprintf("snapshot:%s", snapshotID))
	if err := s.storage.Set(ctx, snapshotKey, compressedData); err != nil {
		return nil, fmt.Errorf("存储快照失败: %w", err)
	}

	// 10. 存储快照元数据（使用 JSON）
	metaData := &types.UTXOSnapshotData{
		SnapshotID:  snapshotID,
		Height:      height,
		BlockHash:   blockHash, // P3-13: 从 BlockQuery 获取区块哈希
		StateRoot:   snapshotHash,
		UTXOCount:   uint64(len(utxoBytes)),
		CreatedTime: time.Now(),
	}
	metaBytes, err := json.Marshal(metaData)
	if err != nil {
		return nil, fmt.Errorf("序列化快照元数据失败: %w", err)
	}
	metaKey := []byte(fmt.Sprintf("snapshot:meta:%s", snapshotID))
	if err := s.storage.Set(ctx, metaKey, metaBytes); err != nil {
		return nil, fmt.Errorf("存储快照元数据失败: %w", err)
	}

	// ✅ 记录修复统计
	if repairedCount > 0 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 快照创建期间自动修复了 %d 个UTXO", repairedCount)
		}

		// 发布修复事件
		if s.eventBus != nil {
			s.eventBus.Publish("utxo.snapshot_repair", ctx, map[string]interface{}{
				"snapshot_height": height,
				"repaired_count":  repairedCount,
				"timestamp":       time.Now(),
			})
		}
	}

	if s.logger != nil {
		s.logger.Infof("✅ 快照创建完成: height=%d, id=%s, utxo_count=%d, repaired=%d", 
			height, snapshotID, len(utxoBytes), repairedCount)
	}

	return metaData, nil
}

// bytesToUint64 将字节数组转换为 uint64（BigEndian）
func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 |
		uint64(b[1])<<48 |
		uint64(b[2])<<40 |
		uint64(b[3])<<32 |
		uint64(b[4])<<24 |
		uint64(b[5])<<16 |
		uint64(b[6])<<8 |
		uint64(b[7])
}

// utxoSnapshotData 用于序列化的临时结构
type utxoSnapshotData struct {
	// Version 快照格式版本：
	// - 1（历史/错误）：Utxos 为 []*utxo.UTXO 的 JSON（含 oneof/interface，无法稳定反序列化）
	// - 2（当前）：Utxos 为 [][]byte（每条 UTXO 的 proto bytes，JSON 中以 base64 编码保存）
	Version int `json:"version"`

	Utxos  [][]byte `json:"utxos"`
	Height uint64   `json:"height"`
}

// buildUTXOKey 构建UTXO存储键
func buildUTXOKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}
