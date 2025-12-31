package sync

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/chain/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

func tryAutoReorgFromHello(
	ctx context.Context,
	peerID peer.ID,
	hello *helloV2Info,
	chainQuery persistence.ChainQuery,
	blockHashClient core.BlockHashServiceClient,
	forkHandler interfaces.InternalForkHandler,
	networkService network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	logger log.Logger,
) error {
	if hello == nil {
		return fmt.Errorf("hello is nil")
	}
	if forkHandler == nil {
		return fmt.Errorf("forkHandler 未注入，无法自动 reorg")
	}
	// 允许祖先为 genesis(0)，但必须携带 32 bytes 的祖先 hash；否则视为“未提供祖先”
	if hello.commonAncestorHeight == 0 && len(hello.commonAncestorHash) != 32 {
		return fmt.Errorf("hello 未提供 common ancestor（ancestor=0 且无有效hash），无法自动 reorg")
	}
	if hello.remoteTipHeight <= hello.commonAncestorHeight {
		return nil
	}

	// 限制自动 reorg 深度（默认 1000，可配置）
	maxDepth := uint64(1000)
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil && bc.Sync.Advanced.AutoReorgMaxDepth > 0 {
			maxDepth = uint64(bc.Sync.Advanced.AutoReorgMaxDepth)
		}
	}
	depth := hello.remoteTipHeight - hello.commonAncestorHeight
	if depth > maxDepth {
		return fmt.Errorf("auto reorg depth exceeded: depth=%d max=%d ancestor=%d remote_tip=%d",
			depth, maxDepth, hello.commonAncestorHeight, hello.remoteTipHeight)
	}

	// 可选：校验共同祖先 hash 一致性（避免误判）
	if len(hello.commonAncestorHash) == 32 {
		if qs, ok := chainQuery.(persistence.QueryService); ok && qs != nil && blockHashClient != nil {
			blk, err := qs.GetBlockByHeight(ctx, hello.commonAncestorHeight)
			if err != nil {
				return fmt.Errorf("读取共同祖先区块失败: %w", err)
			}
			resp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
			if err != nil || resp == nil || !resp.IsValid || len(resp.Hash) != 32 {
				return fmt.Errorf("计算共同祖先哈希失败: %v", err)
			}
			if string(resp.Hash) != string(hello.commonAncestorHash) {
				return fmt.Errorf("共同祖先哈希不一致：local!=remote(height=%d)", hello.commonAncestorHeight)
			}
		}
	}

	start := hello.commonAncestorHeight + 1
	end := hello.remoteTipHeight
	if logger != nil {
		logger.Warnf("[TriggerSync] 🔁 自动reorg：下载分叉段 blocks [%d..%d] from peer=%s",
			start, end, peerID.String()[:8])
	}

	forkBlocks := make(map[uint64]*core.Block, 0)
	next := start
	for next <= end {
		blocks, err := fetchBlockRange(ctx, peerID, next, end, networkService, p2pService, configProvider, logger)
		if err != nil {
			return fmt.Errorf("下载分叉段失败: %w", err)
		}
		if len(blocks) == 0 {
			return fmt.Errorf("下载分叉段返回空批次: next=%d end=%d", next, end)
		}
		lastH := uint64(0)
		for _, b := range blocks {
			if b == nil || b.Header == nil {
				continue
			}
			forkBlocks[b.Header.Height] = b
			if b.Header.Height > lastH {
				lastH = b.Header.Height
			}
		}
		if lastH < next {
			return fmt.Errorf("下载分叉段未推进: next=%d last=%d", next, lastH)
		}
		next = lastH + 1
	}

	forkTip := forkBlocks[end]
	if forkTip == nil {
		return fmt.Errorf("分叉段缺失 forkTip: height=%d", end)
	}

	if logger != nil {
		logger.Warnf("[TriggerSync] 🔁 自动reorg：调用 ForkHandler 执行重组 fork_height=%d new_tip=%d",
			hello.commonAncestorHeight, forkTip.Header.Height)
	}
	return forkHandler.HandleForkWithExternalBlocks(ctx, hello.commonAncestorHeight, forkTip, forkBlocks)
}
