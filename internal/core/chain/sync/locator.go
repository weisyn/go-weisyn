package sync

import (
	"context"
	"encoding/hex"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// BuildBlockLocatorBinary 构造 locator 的二进制编码（与服务端 parseBlockLocatorBinary 对应）：
// 每个 entry 固定 40 bytes = height(8, big-endian) + hash(32)。
//
// 采样策略：
// - 先线性取最近的若干高度（例如 10 个）
// - 之后步长指数增长（1,2,4,8...），直到 0
// - 始终包含 genesis(0)
func BuildBlockLocatorBinary(
	ctx context.Context,
	queryService persistence.QueryService,
	blockHashClient core.BlockHashServiceClient,
	tipHeight uint64,
	maxEntries int,
	configProvider config.Provider,
) ([]byte, error) {
	if maxEntries <= 0 {
		maxEntries = 32
	}
	if queryService == nil {
		return nil, fmt.Errorf("queryService is nil")
	}
	if blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient is nil")
	}

	heights := make([]uint64, 0, maxEntries)

	// Bitcoin 风格 locator：先密集，后指数回退
	const denseCount = 10
	step := uint64(1)
	h := tipHeight
	for len(heights) < maxEntries && h > 0 {
		heights = append(heights, h)
		// 前 denseCount 个：逐块回退；之后步长翻倍
		if len(heights) >= denseCount {
			step *= 2
		}
		if h < step {
			break
		}
		h -= step
	}
	// 总是包含 genesis
	heights = append(heights, 0)

	// 去重（可能出现重复 0）
	uniq := make([]uint64, 0, len(heights))
	seen := map[uint64]struct{}{}
	for _, hh := range heights {
		if _, ok := seen[hh]; ok {
			continue
		}
		seen[hh] = struct{}{}
		uniq = append(uniq, hh)
	}
	heights = uniq
	if len(heights) > maxEntries {
		heights = heights[:maxEntries]
	}

	out := make([]byte, 0, len(heights)*(8+32))
	for _, height := range heights {
		blk, err := queryService.GetBlockByHeight(ctx, height)
		if err != nil {
			continue
		}
		if blk == nil || blk.Header == nil {
			continue
		}
		resp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
		if err != nil || resp == nil || !resp.IsValid || len(resp.Hash) != 32 {
			continue
		}
		out = append(out, uint64ToBytesBE(height)...)
		out = append(out, resp.Hash...)
	}

	// 至少要有一个 entry 才有意义
	if len(out) == 0 {
		// 尝试兜底：至少包含 genesis(0) entry（使用配置计算的 genesis hash）
		if configProvider != nil {
			if id, err := GetLocalChainIdentity(ctx, configProvider, queryService); err == nil && id.IsValid() {
				if b, err := hex.DecodeString(id.GenesisHash); err == nil && len(b) == 32 {
					out = append(out, uint64ToBytesBE(0)...)
					out = append(out, b...)
				}
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("locator build produced empty result")
		}
	}
	return out, nil
}

func uint64ToBytesBE(v uint64) []byte {
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
