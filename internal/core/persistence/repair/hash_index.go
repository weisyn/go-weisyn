package repair

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// RepairHashToHeightIndex 重建 indices:hash:<hash> -> height(8bytes) 索引。
//
// 策略：从 state:chain:tip 读取 tipHeight，然后向下扫描最近 window 个高度，
// 读取 indices:height:<h> 的 blockHash（前 32 bytes），命中则写回 indices:hash。
func RepairHashToHeightIndex(ctx context.Context, store storage.BadgerStore, logger log.Logger, blockHash []byte, window uint64) (uint64, error) {
	if store == nil {
		return 0, fmt.Errorf("store is nil")
	}
	if len(blockHash) == 0 {
		return 0, fmt.Errorf("empty blockHash")
	}
	if window == 0 {
		window = 5000
	}

	// 读取链尖
	tipKey := []byte("state:chain:tip")
	tipData, err := store.Get(ctx, tipKey)
	if err != nil {
		return 0, fmt.Errorf("repair: read tip failed: %w", err)
	}
	if len(tipData) < 8 {
		return 0, fmt.Errorf("repair: tip data invalid (len=%d)", len(tipData))
	}
	tipHeight := bytesToUint64(tipData[:8])

	var start uint64
	if tipHeight > window {
		start = tipHeight - window
	} else {
		start = 0
	}

	if logger != nil {
		logger.Warnf("🩹 repair: rebuilding hash->height index by scanning [%d..%d]", start, tipHeight)
	}

	for h := tipHeight; ; h-- {
		heightKey := []byte(fmt.Sprintf("indices:height:%d", h))
		indexData, err := store.Get(ctx, heightKey)
		if err == nil && len(indexData) >= 32 {
			if string(indexData[:32]) == string(blockHash) {
				hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
				if err := store.Set(ctx, hashKey, uint64ToBytes(h)); err != nil {
					return 0, fmt.Errorf("repair: write hash index failed: %w", err)
				}
				return h, nil
			}
		}

		if h == start {
			break
		}
		if h == 0 {
			break
		}
	}

	return 0, fmt.Errorf("repair: target hash not found in window (tip=%d window=%d)", tipHeight, window)
}

func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func uint64ToBytes(v uint64) []byte {
	return []byte{
		byte(v >> 56),
		byte(v >> 48),
		byte(v >> 40),
		byte(v >> 32),
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
}
