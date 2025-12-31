// Package fork 分叉处理核心逻辑
package fork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/weisyn/v1/internal/core/chain/fork/reorg"
	"github.com/weisyn/v1/internal/core/chain/fork/reorg/managers"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// ============================================================================
//                              分叉处理实现
// ============================================================================

// handleFork 处理分叉的核心逻辑
//
// 🎯 **职责**：
// - 验证分叉区块
// - 比较链权重
// - 决定是否切换链
// - 执行重组
//
// 实现流程：
// 1. 检查是否正在处理分叉
// 2. 检测分叉点
// 3. 计算链权重
// 4. 比较权重决策
// 5. 执行链切换（如需要）
// 6. 更新指标
func (s *Service) handleFork(ctx context.Context, forkBlock *core.Block) error {
	// 检查分叉区块是否为 nil
	if forkBlock == nil {
		return fmt.Errorf("分叉区块不能为空")
	}

	// 检查区块头是否为 nil
	if forkBlock.Header == nil {
		return fmt.Errorf("分叉区块头不能为空")
	}

	// 1. 检查是否正在处理分叉
	if s.isProcessing() {
		return fmt.Errorf("正在处理另一个分叉，请稍后重试")
	}

	// 2. 设置处理状态
	s.setProcessing(true, forkBlock.Header.Height)
	defer s.setProcessing(false, 0)

	// 3. 增加分叉计数
	s.incrementMetric("total_forks")

	if s.logger != nil {
		s.logger.Infof("🔄 开始处理分叉: 高度=%d", forkBlock.Header.Height)
	}

	startTime := time.Now()

	// 4. 检测分叉点
	isFork, forkHeight, err := s.detectFork(ctx, forkBlock)
	if err != nil {
		h := forkBlock.Header.Height
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		// 自运行：如果属于可自愈的存储/索引错误，给 RepairManager 一个短窗口再重试一次
		if isRepairableForFork(err) {
			if waitAndRetry(ctx, 1200*time.Millisecond) == nil {
				isFork, forkHeight, err = s.detectFork(ctx, forkBlock)
				if err == nil {
					goto forkDetected
				}
			}
		}
		return fmt.Errorf("检测分叉失败: %w", err)
	}
forkDetected:

	if !isFork {
		if s.logger != nil {
			s.logger.Info("✅ 不是分叉区块，正常处理")
		}
		return nil
	}

	if s.logger != nil {
		s.logger.Infof("检测到分叉点: 高度=%d", forkHeight)
	}

	// 5. 获取当前主链信息
	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	currentHeight := chainInfo.Height

	// 6. 计算分叉深度
	forkDepth := uint32(currentHeight - forkHeight)

	if s.logger != nil {
		s.logger.Infof("分叉深度: %d 个区块", forkDepth)
	}

	// 7. 检查分叉深度是否超过阈值（从配置获取，默认 100）
	maxForkDepth := uint32(s.getMaxForkDepth())
	if forkDepth > maxForkDepth {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 分叉深度 %d 超过阈值 %d（consensus.miner.max_fork_depth），拒绝处理。"+
				"这通常意味着发生了异常深度的重组或长时间网络分区。建议运维操作："+
				"1) 检查网络和上游节点健康；2) 评估是否临时调高 max_fork_depth；3) 如链数据存在明显错误，考虑执行离线修复脚本或重建节点。",
				forkDepth, maxForkDepth)
		}
		return fmt.Errorf("分叉深度过大: %d > %d（受 consensus.miner.max_fork_depth 限制）", forkDepth, maxForkDepth)
	}

	// 8. 计算主链权重
	mainChainWeight, err := s.calculateChainWeight(ctx, forkHeight, currentHeight)
	if err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		if isRepairableForFork(err) {
			if waitAndRetry(ctx, 1200*time.Millisecond) == nil {
				mainChainWeight, err = s.calculateChainWeight(ctx, forkHeight, currentHeight)
				if err == nil {
					goto mainWeightOK
				}
			}
		}
		return fmt.Errorf("计算主链权重失败: %w", err)
	}
mainWeightOK:

	// 9. 计算分叉链权重
	forkChainWeight, err := s.calculateChainWeight(ctx, forkHeight, forkBlock.Header.Height)
	if err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		if isRepairableForFork(err) {
			if waitAndRetry(ctx, 1200*time.Millisecond) == nil {
				forkChainWeight, err = s.calculateChainWeight(ctx, forkHeight, forkBlock.Header.Height)
				if err == nil {
					goto forkWeightOK
				}
			}
		}
		return fmt.Errorf("计算分叉链权重失败: %w", err)
	}
forkWeightOK:

	if s.logger != nil {
		s.logger.Infof("链权重比较: 主链=%s, 分叉链=%s",
			mainChainWeight.String(), forkChainWeight.String())
	}

	// 10. 比较权重决定是否切换
	shouldSwitch := s.shouldSwitchChain(mainChainWeight, forkChainWeight)

	if !shouldSwitch {
		if s.logger != nil {
			s.logger.Info("✅ 主链权重更大，保持主链不变")
		}
		s.incrementMetric("resolved_forks")
		return nil
	}

	// 11. 执行链切换
	if s.logger != nil {
		s.logger.Warn("⚠️ 分叉链权重更大，准备切换主链")
	}

	if err := s.switchChain(ctx, forkBlock, forkHeight); err != nil {
		h := forkHeight
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseReorg, types.CorruptionSeverityCritical, &h, "", "", err)
		if isRepairableForFork(err) {
			if waitAndRetry(ctx, 1200*time.Millisecond) == nil {
				if err2 := s.switchChain(ctx, forkBlock, forkHeight); err2 == nil {
					goto switchOK
				}
			}
		}
		return fmt.Errorf("链切换失败: %w", err)
	}
switchOK:

	// 12. 更新指标
	s.incrementMetric("resolved_forks")
	s.incrementMetric("total_reorgs")
	s.updateReorgDepth(forkDepth)

	duration := time.Since(startTime)
	if s.logger != nil {
		s.logger.Infof("✅ 分叉处理完成，耗时: %.2fs", duration.Seconds())
	}

	return nil
}

func isRepairableForFork(err error) bool {
	cls := corruptutil.ClassifyErr(err)
	switch cls {
	case "index_corrupt_hash_height", "index_corrupt_height_index", "tip_inconsistent", "tx_index_corrupt":
		return true
	default:
		return false
	}
}

func waitAndRetry(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ============================================================================
//                              链切换实现
// ============================================================================

// switchChain 执行链切换
//
// 🔄 **链重组核心逻辑**
//
// 步骤：
// 1. 创建 UTXO 快照
// 2. 回滚主链区块
// 3. 应用分叉链区块
// 4. 验证新链状态
// 5. 更新链尖
func (s *Service) switchChain(ctx context.Context, forkBlock *core.Block, forkHeight uint64) error {
	return s.switchChainWithProvider(ctx, forkBlock, forkHeight, nil)
}

// deleteByPrefix 批量删除指定前缀的键（prefixScan + DeleteMany）
func (s *Service) deleteByPrefix(ctx context.Context, prefix []byte) (int, error) {
	if s == nil || s.store == nil {
		return 0, fmt.Errorf("badger store 未注入")
	}
	m, err := s.store.PrefixScan(ctx, prefix)
	if err != nil {
		return 0, err
	}
	if len(m) == 0 {
		return 0, nil
	}
	keys := make([][]byte, 0, len(m))
	for k := range m {
		keys = append(keys, []byte(k))
	}
	if err := s.store.DeleteMany(ctx, keys); err != nil {
		return 0, err
	}
	return len(keys), nil
}

// clearReorgStateForGenesisRebuild 清理 reorg 到 genesis 所需的可重建状态（UTXO/索引/链尖）
func (s *Service) clearReorgStateForGenesisRebuild(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("badger store 未注入，无法执行 genesis 重建")
	}

	// 1) 清理 UTXO 主集与索引/引用关系
	for _, p := range [][]byte{
		[]byte("utxo:set:"),
		[]byte("index:address:"),
		[]byte("index:height:"),
		[]byte("index:asset:"),
		[]byte("ref:"),
	} {
		if _, err := s.deleteByPrefix(ctx, p); err != nil {
			return fmt.Errorf("清理前缀失败(%s): %w", string(p), err)
		}
	}

	// 2) 清理交易索引（旧链残留会污染查询）
	if _, err := s.deleteByPrefix(ctx, []byte("indices:tx:")); err != nil {
		return fmt.Errorf("清理交易索引失败(indices:tx:): %w", err)
	}

	// 2.1) 清理区块索引（blocks/ 文件不强制删除，但索引必须清空以避免旧链残留被查询/修复逻辑误用）
	// - 高度索引：indices:height:{height}
	// - 哈希索引：indices:hash:{hash} -> height
	for _, p := range [][]byte{
		[]byte("indices:height:"),
		[]byte("indices:hash:"),
	} {
		if _, err := s.deleteByPrefix(ctx, p); err != nil {
			return fmt.Errorf("清理区块索引失败(%s): %w", string(p), err)
		}
	}

	// 3) 清理资源/历史索引（依赖 UTXO/链历史，可由重放重建）
	for _, p := range [][]byte{
		[]byte("indices:resource:"),          // 旧 contentHash 索引/资源历史
		[]byte("indices:resource-instance:"), // 新实例索引
		[]byte("indices:resource-code:"),     // code->instances
		[]byte("resource:utxo-instance:"),
		[]byte("resource:counters-instance:"),
		[]byte("index:resource:owner-instance:"),
		[]byte("indices:utxo:history:"),
	} {
		if _, err := s.deleteByPrefix(ctx, p); err != nil {
			return fmt.Errorf("清理资源/历史前缀失败(%s): %w", string(p), err)
		}
	}

	// 4) 清理链尖/状态根，使 DataWriter 进入“空链状态”，允许重新写入 genesis(0)
	if err := s.store.DeleteMany(ctx, [][]byte{
		[]byte("state:chain:tip"),
		[]byte("state:chain:root"),
	}); err != nil {
		return fmt.Errorf("清理链状态失败(state:chain:*): %w", err)
	}

	return nil
}

// rebuildChainFromGenesis 通过“从 genesis 顺序写入”完成切链（支持 forkHeight=0 的彻底 reorg）
func (s *Service) rebuildChainFromGenesis(
	ctx context.Context,
	newTip *core.Block,
	provider func(height uint64) (*core.Block, bool),
) error {
	if s == nil || s.blockProcessor == nil || s.queryService == nil {
		return fmt.Errorf("依赖未注入（blockProcessor/queryService）")
	}
	if newTip == nil || newTip.Header == nil {
		return fmt.Errorf("newTip 为空")
	}
	// genesis(0) 从本地读取（链身份硬校验已在 sync hello v2 完成）
	genesis, err := s.queryService.GetBlockByHeight(ctx, 0)
	if err != nil {
		return fmt.Errorf("读取 genesis 失败: %w", err)
	}
	if genesis == nil || genesis.Header == nil || genesis.Header.Height != 0 {
		return fmt.Errorf("genesis 无效或缺失")
	}

	if err := s.blockProcessor.ProcessBlock(ctx, genesis); err != nil {
		return fmt.Errorf("重建写入 genesis 失败: %w", err)
	}

	for h := uint64(1); h <= newTip.Header.Height; h++ {
		blk, ok := provider(h)
		if !ok || blk == nil || blk.Header == nil {
			return fmt.Errorf("重建缺失分叉段区块: height=%d", h)
		}
		if blk.Header.Height != h {
			return fmt.Errorf("重建分叉段高度不一致: expect=%d got=%d", h, blk.Header.Height)
		}
		if err := s.blockProcessor.ProcessBlock(ctx, blk); err != nil {
			return fmt.Errorf("重建写入区块失败: height=%d err=%w", h, err)
		}
	}
	return nil
}

// rebuildChainByLocalPrefixAndForkProvider 在“快照不可用/损坏”时，走生产级自省修复：
// - 清理可重建状态（UTXO/索引/链尖）
// - 从本地已有主链块（0..forkHeight）顺序重放
// - forkHeight+1..newTipHeight 必须由 provider 提供（即同步下载的分叉段）
//
// 说明：
// - 这是对快照恢复失败的“根治兜底”，避免同步/重组进入必失败状态。
// - 该路径比快照慢，但确定性强：以区块为唯一真相重建状态。
func (s *Service) rebuildChainByLocalPrefixAndForkProvider(
	ctx context.Context,
	forkHeight uint64,
	newTip *core.Block,
	provider func(height uint64) (*core.Block, bool),
) error {
	if s == nil || s.blockProcessor == nil || s.queryService == nil || s.store == nil {
		return fmt.Errorf("依赖未注入（blockProcessor/queryService/store）")
	}
	if newTip == nil || newTip.Header == nil {
		return fmt.Errorf("newTip 为空")
	}
	if provider == nil {
		return fmt.Errorf("provider 为空：无法获取分叉段区块")
	}

	// 🔧 启用 Recovery Mode（允许在只读模式下执行修复）
	var recoveryToken string
	var recoveryEnabled bool
	if s.writeGate != nil {
		tok, err := s.writeGate.EnableRecoveryMode("self-introspection-rebuild")
		if err != nil {
			return fmt.Errorf("启用恢复模式失败: %w", err)
		}
		recoveryToken = tok
		recoveryEnabled = true
		defer func() {
			if recoveryEnabled {
				_ = s.writeGate.DisableRecoveryMode(recoveryToken)
			}
		}()

		// 将 recovery token 绑定到 context
		ctx = writegate.WithWriteToken(ctx, recoveryToken)

		if s.logger != nil {
			s.logger.Infof("🔧 自省修复：已启用恢复模式（允许在只读模式下写入）")
		}
	}

	// 1) 清理可重建状态
	if err := s.clearReorgStateForGenesisRebuild(ctx); err != nil {
		return fmt.Errorf("自省修复：重建前状态清理失败: %w", err)
	}

	// 2) 重放 genesis
	genesis, err := s.queryService.GetBlockByHeight(ctx, 0)
	if err != nil {
		return fmt.Errorf("自省修复：读取 genesis 失败: %w", err)
	}
	if genesis == nil || genesis.Header == nil || genesis.Header.Height != 0 {
		return fmt.Errorf("自省修复：genesis 无效或缺失")
	}
	// ✅ 现在可以写入 genesis，因为持有 recovery token
	if err := s.blockProcessor.ProcessBlock(ctx, genesis); err != nil {
		return fmt.Errorf("自省修复：写入 genesis 失败: %w", err)
	}

	// 3) 重放本地主链前缀（1..forkHeight）
	for h := uint64(1); h <= forkHeight; h++ {
		blk, err := s.queryService.GetBlockByHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("自省修复：读取本地主链区块失败: height=%d err=%w", h, err)
		}
		if blk == nil || blk.Header == nil || blk.Header.Height != h {
			return fmt.Errorf("自省修复：本地主链区块缺失/损坏: height=%d", h)
		}
		if err := s.blockProcessor.ProcessBlock(ctx, blk); err != nil {
			return fmt.Errorf("自省修复：重放本地主链区块失败: height=%d err=%w", h, err)
		}
	}

	// 4) 重放分叉段（forkHeight+1..newTip）
	for h := forkHeight + 1; h <= newTip.Header.Height; h++ {
		blk, ok := provider(h)
		if !ok || blk == nil || blk.Header == nil || blk.Header.Height != h {
			return fmt.Errorf("自省修复：缺失分叉段区块: height=%d", h)
		}
		if err := s.blockProcessor.ProcessBlock(ctx, blk); err != nil {
			return fmt.Errorf("自省修复：重放分叉段区块失败: height=%d err=%w", h, err)
		}
	}

	// 🔧 成功后显式关闭 Recovery Mode
	if recoveryEnabled && s.writeGate != nil {
		if err := s.writeGate.DisableRecoveryMode(recoveryToken); err != nil {
			if s.logger != nil {
				s.logger.Warnf("关闭恢复模式失败: %v", err)
			}
		}
		recoveryEnabled = false

		if s.logger != nil {
			s.logger.Infof("✅ 自省修复：已关闭恢复模式")
		}
	}

	return nil
}

func (s *Service) switchChainWithProvider(
	ctx context.Context,
	forkBlock *core.Block,
	forkHeight uint64,
	provider func(height uint64) (*core.Block, bool),
) error {
	if forkBlock == nil || forkBlock.Header == nil {
		return fmt.Errorf("forkBlock 不能为空")
	}
	if s == nil || s.queryService == nil || s.blockProcessor == nil || s.utxoSnapshot == nil || s.store == nil {
		return fmt.Errorf("依赖未注入（queryService/blockProcessor/utxoSnapshot/store）")
	}

	chainInfo, err := s.queryService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}
	currentHeight := chainInfo.Height

	// tx-recovery（严格版）：预收集被抛弃主链段(forkHeight+1..currentHeight)上的可回收交易，
	// 并在 reorg 成功后回注到 mempool。
	var recoveredTxs []*transaction.Transaction
	if s.txPool != nil && currentHeight > forkHeight {
		for h := forkHeight + 1; h <= currentHeight; h++ {
			blk, err := s.queryService.GetBlockByHeight(ctx, h)
			if err != nil || blk == nil || blk.Body == nil {
				continue
			}
			for _, tx := range blk.Body.Transactions {
				if tx == nil {
					continue
				}
				// 跳过 0-input 交易（coinbase/创世类），不回注
				if len(tx.Inputs) == 0 {
					continue
				}
				recoveredTxs = append(recoveredTxs, tx)
			}
		}
	}

	doTxRecovery := func(newTip *core.Block) {
		if s.txPool == nil || len(recoveredTxs) == 0 {
			return
		}
		if newTip != nil && newTip.Header != nil {
			_ = s.txPool.SyncStatus(newTip.Header.Height, newTip.Header.StateRoot)
		}
		var okCnt, failCnt int
		for _, tx := range recoveredTxs {
			if tx == nil {
				continue
			}
			if _, err := s.txPool.SubmitTx(tx); err != nil {
				failCnt++
				continue
			}
			okCnt++
		}
		if s.logger != nil {
			s.logger.Infof("✅ tx-recovery 完成：submitted=%d failed=%d detached_total=%d", okCnt, failCnt, len(recoveredTxs))
		}
	}

	// 特判：forkHeight=0 仍使用 genesis 重建（严格语义：0 快照在非0链尖下会成为伪快照）
	if forkHeight == 0 {
		if s.logger != nil {
			s.logger.Warnf("🔁 REORG(genesis): 采用 genesis 重建路径（清理UTXO/索引并从0顺序写入到 new_tip=%d）", forkBlock.Header.Height)
		}
		if err := s.clearReorgStateForGenesisRebuild(ctx); err != nil {
			return fmt.Errorf("genesis 重建前状态清理失败: %w", err)
		}
		if err := s.rebuildChainFromGenesis(ctx, forkBlock, provider); err != nil {
			return fmt.Errorf("genesis 重建失败: %w", err)
		}
		doTxRecovery(forkBlock)
		return nil
	}

	// 全局写门闸：开启 reorg 写围栏（只有携带 token 的写路径允许写入）
	var fenceToken string
	var fenceEnabled bool
	if s.writeGate != nil {
		tok, err := s.writeGate.EnableWriteFence("reorg")
		if err != nil {
			return err
		}
		fenceToken = tok
		fenceEnabled = true
		defer func() {
			if fenceEnabled {
				_ = s.writeGate.DisableWriteFence(fenceToken)
			}
		}()
		ctx = writegate.WithWriteToken(ctx, fenceToken)
	}

	// provider：必须覆盖 forkHeight+1..newTip
	reorgProvider := func(height uint64) (*core.Block, bool) {
		if height == forkBlock.Header.Height {
			return forkBlock, true
		}
		if provider != nil {
			if blk, ok := provider(height); ok && blk != nil && blk.Header != nil && blk.Header.Height == height {
				return blk, true
			}
		}
		blk, err := s.queryService.GetBlockByHeight(ctx, height)
		if err != nil || blk == nil || blk.Header == nil || blk.Header.Height != height {
			return nil, false
		}
		return blk, true
	}

	// 构造协调器（将流程收口到 Coordinator）
	snapshotMgr := managers.NewSnapshotManager(s.utxoSnapshot)
	indexMgr := managers.NewIndexManager(func(ctx context.Context, height uint64) error {
		// rollback-plan-refactor：索引回滚必须走“预收集计划 + 事务内执行”
		return s.RollbackIndicesToHeight(ctx, height)
	})
	verifyFn := func(ctx context.Context, expectedHeight uint64) (*reorg.VerificationResult, error) {
		v, err := NewReorgValidator(s.store, s.queryService, s.txHashClient, s.logger)
		if err != nil {
			return nil, err
		}
		if err := v.VerifyReorgResult(ctx, expectedHeight); err != nil {
			return &reorg.VerificationResult{
				Passed: false,
				Checks: []reorg.CheckResult{
					{Name: "ForkValidator:VerifyReorgResult", Passed: false, Expected: fmt.Sprintf("height=%d", expectedHeight), Actual: "failed", Details: err.Error()},
				},
			}, err
		}
		return &reorg.VerificationResult{
			Passed: true,
			Checks: []reorg.CheckResult{
				{Name: "ForkValidator:VerifyReorgResult", Passed: true, Expected: fmt.Sprintf("height=%d", expectedHeight), Actual: "ok", Details: "验证通过"},
			},
		}, nil
	}
	enterReadOnlyFn := func(ctx context.Context, reason error) {
		if reason == nil {
			reason = fmt.Errorf("unknown reorg failure")
		}
		_ = s.enterReadOnlyMode(ctx, reason.Error())
	}

	// atomic-rollback-single-tx：严格原子化 Phase2（单事务）
	// 🆕 优化：当UTXO数量较大时，使用分批恢复避免"Txn is too big"错误
	atomicRollbackFn := func(ctx context.Context, session *reorg.ReorgSession) error {
		if session == nil {
			return fmt.Errorf("session 不能为空")
		}
		rollbackSnap, err := snapshotMgr.SnapshotForHandle(session.Handles["utxo_rollback"])
		if err != nil {
			return err
		}
		indexPlan, err := s.BuildIndexRollbackPlan(ctx, session.ForkHeight)
		if err != nil {
			return err
		}
		clearPlan, err := s.utxoSnapshot.BuildClearPlan(ctx)
		if err != nil {
			return err
		}
		payload, err := s.utxoSnapshot.LoadSnapshotPayload(ctx, rollbackSnap)
		if err != nil {
			return err
		}

		// 🆕 判断是否需要分批恢复（阈值：1000个UTXO）
		// TODO: 从配置中读取阈值
		utxoCount := len(payload.Utxos)
		useBatching := utxoCount > 1000

		if useBatching {
			// 分批恢复模式：索引回滚在第一个事务，UTXO恢复分多个事务
			if s.logger != nil {
				s.logger.Infof("🔄 使用分批恢复模式: UTXO数量=%d", utxoCount)
			}

			// 1) 索引回滚（单独事务）
			err := s.store.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
				return s.ApplyIndexRollbackPlanInTx(tx, indexPlan)
			})
			if err != nil {
				return fmt.Errorf("索引回滚失败: %w", err)
			}

			// 2) UTXO恢复（分批事务）
			if err := s.utxoSnapshot.RestoreSnapshotWithBatching(ctx, rollbackSnap, payload, clearPlan); err != nil {
				return fmt.Errorf("UTXO分批恢复失败: %w", err)
			}

			return nil
		}

		// 原子模式：单事务完成所有操作（适用于小规模UTXO）
		return s.store.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
			// 1) 索引回滚（删除计划 + tip 更新）
			if err := s.ApplyIndexRollbackPlanInTx(tx, indexPlan); err != nil {
				return err
			}
			// 2) UTXO 恢复（清空旧UTXO/索引/引用 + 写入快照UTXO + 重建索引 + 更新 root）
			if err := s.utxoSnapshot.RestoreSnapshotInTransaction(ctx, tx, rollbackSnap, payload, clearPlan); err != nil {
				return err
			}
			return nil
		})
	}

	// 创建事件发布器（用于发布 REORG 阶段事件和补偿事件）
	eventPublisher := reorg.NewEventPublisher(s.eventBus)

	coord, err := reorg.NewCoordinator(reorg.Options{
		Logger:           s.logger,
		QueryService:     s.queryService,
		BlockProcessor:   s.blockProcessor,
		SnapshotManager:  snapshotMgr,
		IndexManager:     indexMgr,
		VerifyFn:         verifyFn,
		AtomicRollbackFn: atomicRollbackFn,
		EnterReadOnlyFn:  enterReadOnlyFn,
		EventPublisher:   eventPublisher,
	})
	if err != nil {
		return err
	}

	session, err := coord.BeginReorg(ctx, currentHeight, forkHeight, forkBlock.Header.Height)
	if err != nil {
		return err
	}
	if err := coord.ExecuteReorg(ctx, session, reorgProvider); err != nil {
		// ✅ 兜底策略（生产级）：当 reorg 因快照/回滚阶段失败且我们拥有“外部分叉段 provider”时，
		// 走“自省重建”路径：
		// - 清理可重建状态（UTXO/索引/链尖）
		// - 重放本地主链前缀 0..forkHeight
		// - 再重放 provider 提供的分叉段 forkHeight+1..newTip
		//
		// 说明：
		// - 该兜底仅在 provider!=nil 时启用，避免在缺失分叉段时错误地用本地主链区块“冒充”分叉段。
		// - 该兜底比快照慢，但确定性强，可避免节点长期卡在“必失败 reorg”状态。
		if provider != nil && shouldFallbackToSelfRebuild(err) {
			if s.logger != nil {
				s.logger.Warnf("⚠️ REORG 失败，尝试走自省重建兜底: forkHeight=%d newTip=%d err=%v",
					forkHeight, forkBlock.Header.Height, err)
			}
			fallbackProvider := func(height uint64) (*core.Block, bool) {
				if height == forkBlock.Header.Height {
					return forkBlock, true
				}
				blk, ok := provider(height)
				if !ok || blk == nil || blk.Header == nil || blk.Header.Height != height {
					return nil, false
				}
				return blk, true
			}
			if ferr := s.rebuildChainByLocalPrefixAndForkProvider(ctx, forkHeight, forkBlock, fallbackProvider); ferr == nil {
				// 自省重建成功：先解除写围栏，再进行 tx-recovery
				if fenceEnabled && s.writeGate != nil {
					_ = s.writeGate.DisableWriteFence(fenceToken)
					fenceEnabled = false
				}
				doTxRecovery(forkBlock)
				return nil
			} else if s.logger != nil {
				s.logger.Errorf("❌ 自省重建兜底失败: forkHeight=%d newTip=%d err=%v",
					forkHeight, forkBlock.Header.Height, ferr)
			}
		}
		return err
	}
	// reorg 已完成：先解除写围栏，再进行 tx-recovery（TxPool 写路径无 token 语义）
	if fenceEnabled && s.writeGate != nil {
		_ = s.writeGate.DisableWriteFence(fenceToken)
		fenceEnabled = false
	}
	doTxRecovery(forkBlock)
	return nil
}

// shouldFallbackToSelfRebuild 判断一次 reorg 失败是否应触发“自省重建”兜底。
//
// 原则：
// - 仅对 Prepare/Rollback 阶段失败启用（通常为快照创建/恢复失败、索引/UTXO 原子回滚失败）。
// - 通过结构化错误 + 关键字进行保守判断，避免对 Replay/Verify 失败误触发（那通常意味着分叉段本身无效）。
func shouldFallbackToSelfRebuild(err error) bool {
	if err == nil {
		return false
	}
	var re *reorg.ReorgError
	if errors.As(err, &re) {
		if re.Phase != reorg.PhasePrepare && re.Phase != reorg.PhaseRollback {
			return false
		}
	}
	msg := err.Error()
	// 关键字覆盖：快照/UTXO 恢复/索引回滚/哈希校验等
	switch {
	case strings.Contains(msg, "snapshot"),
		strings.Contains(msg, "快照"),
		strings.Contains(msg, "RestoreSnapshot"),
		strings.Contains(msg, "CreateSnapshot"),
		strings.Contains(msg, "utxo"),
		strings.Contains(msg, "UTXO"),
		strings.Contains(msg, "state_root"),
		strings.Contains(msg, "BlockHeight"):
		return true
	default:
		return false
	}
}

// ============================================================================
//                              决策逻辑
// ============================================================================

// shouldSwitchChain 判断是否应该切换到分叉链
//
// 决策规则：
// - 如果分叉链的累积难度更大，则切换
// - 如果累积难度相同，比较区块数量
// - 如果区块数量相同，确定性 tie-break：tip hash 更小的优先（按固定字节序比较）
func (s *Service) shouldSwitchChain(mainChain, forkChain *types.ChainWeight) bool {
	// 1. 比较累积难度
	if forkChain.CumulativeDifficulty.Cmp(mainChain.CumulativeDifficulty) > 0 {
		return true
	}

	if forkChain.CumulativeDifficulty.Cmp(mainChain.CumulativeDifficulty) < 0 {
		return false
	}

	// 2. 累积难度相同，比较区块数量
	if forkChain.BlockCount > mainChain.BlockCount {
		return true
	}

	if forkChain.BlockCount < mainChain.BlockCount {
		return false
	}

	// 3. 区块数量相同，确定性 tie-break：tip hash 更小的优先
	//
	// 说明：
	// - 旧实现使用 LastBlockTime 作为最终裁决，但该字段可被矿工操纵（微调时间戳），并可能导致全网不收敛；
	// - 以 tip hash 的固定字节序比较作为 tie-break，能保证不同节点在相同信息下做出一致选择。
	if len(forkChain.TipHash) > 0 || len(mainChain.TipHash) > 0 {
		return bytes.Compare(forkChain.TipHash, mainChain.TipHash) < 0
	}

	// 向后兼容：若未提供 tip hash，退化为旧规则（更早时间戳优先）
	return forkChain.LastBlockTime < mainChain.LastBlockTime
}
