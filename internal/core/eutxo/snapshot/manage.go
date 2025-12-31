package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weisyn/v1/pkg/types"
)

// DeleteSnapshot 删除快照
//
// 实现 eutxo.UTXOSnapshot.DeleteSnapshot
func (s *Service) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	// 1. 验证快照ID
	if snapshotID == "" {
		return fmt.Errorf("快照ID不能为空")
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 删除快照数据和元数据
	snapshotKey := []byte(fmt.Sprintf("snapshot:%s", snapshotID))
	metaKey := []byte(fmt.Sprintf("snapshot:meta:%s", snapshotID))

	// 批量删除
	keysToDelete := [][]byte{snapshotKey, metaKey}
	if err := s.storage.DeleteMany(ctx, keysToDelete); err != nil {
		return fmt.Errorf("删除快照失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ 快照已删除: %s", snapshotID)
	}

	return nil
}

// ListSnapshots 列出所有快照
//
// 实现 eutxo.UTXOSnapshot.ListSnapshots
func (s *Service) ListSnapshots(ctx context.Context) ([]*types.UTXOSnapshotData, error) {
	// 1. 加锁（读锁）
	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. 从 Storage 查询所有快照元数据（通过前缀扫描）
	metaPrefix := []byte("snapshot:meta:")
	metaMap, err := s.storage.PrefixScan(ctx, metaPrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描快照元数据失败: %w", err)
	}

	// 3. 反序列化所有快照元数据
	snapshots := make([]*types.UTXOSnapshotData, 0, len(metaMap))
	for key, metaData := range metaMap {
		// 提取快照ID（从键中提取）
		// 键格式：snapshot:meta:{snapshotID}
		keyStr := string(key)
		if !strings.HasPrefix(keyStr, "snapshot:meta:") {
			continue
		}
		snapshotID := strings.TrimPrefix(keyStr, "snapshot:meta:")

		// 反序列化元数据
		var snapshot types.UTXOSnapshotData
		if err := json.Unmarshal(metaData, &snapshot); err != nil {
			if s.logger != nil {
				s.logger.Warnf("反序列化快照元数据失败 (ID=%s): %v", snapshotID, err)
			}
			continue
		}

		// 确保 SnapshotID 正确设置
		if snapshot.SnapshotID == "" {
			snapshot.SnapshotID = snapshotID
		}

		snapshots = append(snapshots, &snapshot)
	}

	if s.logger != nil {
		s.logger.Debugf("📋 快照列表查询完成: 共 %d 个快照", len(snapshots))
	}

	return snapshots, nil
}

