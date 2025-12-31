// Package orchestrator 实现挖矿编排器的确认等待和同步触发功能
//
// ⏳ **共识模式感知的确认等待模块**
//
// 🎯 **根据共识模式采用不同的确认策略**：
//   - 分布式共识模式: 等待网络确认，超时触发同步
//   - 单节点开发模式: 本地验证，立即确认
//
// 本文件实现区块提交后的确认等待和同步触发逻辑：
// 1. 区块确认等待 - 等待区块在网络中的确认
// 2. 确认超时处理 - 设置合理的等待超时并处理超时情况
// 3. 同步触发机制 - 确认超时时主动触发同步以获取最新状态
// 4. 高度门闸更新 - 确认成功或超时后更新已处理高度
// 5. 状态协调管理 - 与其他组件协调挖矿后续处理
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	consensusif "github.com/weisyn/v1/internal/core/consensus/interfaces"
	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/types"
)

// 注意：确认超时和检查间隔现在从配置中获取，不再使用硬编码常量

// waitForConfirmation 等待区块确认（根据共识模式自动分支）
//
// 🎯 **共识模式分支处理**：
//   - 分布式共识模式: 等待网络确认
//   - 单节点开发模式: 本地验证
//
// 这是确认等待的主入口方法，被 execute_mining_round.go 调用
func (s *MiningOrchestratorService) waitForConfirmation(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("开始等待区块确认")

	// ⚠️ 系统内不存在“单节点共识模式”：
	// v2：确认不再强串行阻塞挖矿主循环，而是启动 watcher 监控确认。
	return s.startDistributedConfirmationWatch(ctx, minedBlock)
}

// confirmationWatch tracks a single height confirmation process (v2 non-blocking).
type confirmationWatch struct {
	height       uint64
	startedAt    time.Time
	lastSubmitAt time.Time
	submits      uint64
	cancel       context.CancelFunc
}

// startDistributedConfirmationWatch 分布式共识模式：启动后台确认 watcher（非阻塞）
//
// 🎯 **生产环境标准路径**：
//   - 不阻塞挖矿主循环（避免确认门闸卡住导致“全链停摆”）
//   - 后台通过检查链高度变化来判断确认状态
//   - 超时后触发同步 + 输出诊断日志（peer 数、进度、同步状态等）
//
// @param ctx 上下文对象
// @param minedBlock 已挖出的完整区块
// @return error 确认过程中的错误
func (s *MiningOrchestratorService) startDistributedConfirmationWatch(ctx context.Context, minedBlock *blocktypes.Block) error {
	if minedBlock == nil || minedBlock.Header == nil {
		return fmt.Errorf("minedBlock/header 不能为空")
	}
	if s.minerConfig == nil {
		return fmt.Errorf("minerConfig 未注入，无法启动确认 watcher")
	}

	expectedHeight := minedBlock.Header.Height
	now := time.Now()

	// v2：按高度去重，只允许一个 watcher 追踪该高度，避免 goroutine 泄漏与日志风暴
	s.confirmMu.Lock()
	if existing := s.confirmWatches[expectedHeight]; existing != nil {
		existing.lastSubmitAt = now
		existing.submits++
		s.confirmMu.Unlock()
		return nil
	}

	watchCtx, cancel := context.WithCancel(ctx)
	w := &confirmationWatch{
		height:       expectedHeight,
		startedAt:    now,
		lastSubmitAt: now,
		submits:      1,
		cancel:       cancel,
	}
	s.confirmWatches[expectedHeight] = w
	s.confirmMu.Unlock()

	if s.logger != nil {
		s.logger.Infof("🔭 v2 启动确认 watcher: height=%d", expectedHeight)
	}

	go s.runConfirmationWatch(watchCtx, w)
	return nil
}

func (s *MiningOrchestratorService) runConfirmationWatch(ctx context.Context, w *confirmationWatch) {
	if w == nil {
		return
	}
	defer func() {
		s.confirmMu.Lock()
		delete(s.confirmWatches, w.height)
		s.confirmMu.Unlock()
	}()

	// 超时配置（兜底 30s）
	timeout := s.minerConfig.ConfirmationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 检查间隔配置（兜底 1s）
	interval := s.minerConfig.ConfirmationCheckInterval
	if interval <= 0 {
		interval = 1 * time.Second
	}

	// 诊断日志间隔（兜底 5s）
	diagInterval := 5 * time.Second
	if s.minerConfig != nil && s.minerConfig.ConfirmationDiagInterval > 0 {
		diagInterval = s.minerConfig.ConfirmationDiagInterval
	}
	nextDiagAt := time.Now().Add(diagInterval)

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			// 超时：触发同步 + 输出诊断，但不阻塞主挖矿循环
			s.logConfirmationStall(ctx, w, fmt.Errorf("确认超时（timeout=%s）", timeout))
			s.applyConfirmationTimeoutFallback(w)
			return
		case <-ticker.C:
			// 已确认？
			if err := s.checkBlockConfirmation(ctx, w.height); err == nil {
				// 二次验证并更新高度门闸
				if err := s.validateChainHeightBeforeGateUpdate(ctx, w.height); err == nil {
					s.updateHeightGate(w.height)
				} else if s.logger != nil {
					s.logger.Warnf("确认 watcher：门闸更新前验证失败: %v", err)
				}
				if s.logger != nil {
					s.logger.Infof("✅ v2 确认 watcher：区块已确认: height=%d", w.height)
				}
				return
			}

			// 周期性诊断（避免日志刷屏）
			if time.Now().After(nextDiagAt) {
				nextDiagAt = time.Now().Add(diagInterval)
				s.logConfirmationStall(ctx, w, nil)
			}
		}
	}
}

func (s *MiningOrchestratorService) applyConfirmationTimeoutFallback(w *confirmationWatch) {
	if s == nil {
		return
	}
	action := "sync"
	if s.minerConfig != nil && strings.TrimSpace(s.minerConfig.ConfirmationTimeoutFallback) != "" {
		action = strings.ToLower(strings.TrimSpace(s.minerConfig.ConfirmationTimeoutFallback))
	}

	switch action {
	case "drop":
		// 仅记录诊断，继续挖矿（不触发任何额外动作）
		if s.logger != nil && w != nil {
			s.logger.Warnf("🗑️ v2 确认超时退路=drop：丢弃本轮确认跟踪（height=%d）", w.height)
		}
		return

	default: // "sync"
	}

	// sync：触发一次同步，继续挖矿
	if err := s.triggerSyncIfNeeded(context.Background()); err != nil && s.logger != nil {
		s.logger.Warnf("确认超时后触发同步失败: %v", err)
	}
}

// logConfirmationStall prints actionable diagnostics for confirmation stalls.
// It is best-effort and must never panic.
func (s *MiningOrchestratorService) logConfirmationStall(ctx context.Context, w *confirmationWatch, cause error) {
	if s == nil || s.logger == nil || w == nil {
		return
	}

	// 1) 链高度/状态
	var chainHeight uint64
	var chainStatus string
	if s.chainQuery != nil {
		if chainInfo, err := s.chainQuery.GetChainInfo(ctx); err == nil && chainInfo != nil {
			chainHeight = chainInfo.Height
			chainStatus = chainInfo.Status
		}
	}

	// 2) 同步状态（网络高度/peer不足的常见根因）
	var (
		localHeight   uint64
		networkHeight uint64
		syncStatus    string
	)
	if s.syncService != nil {
		if st, err := s.syncService.CheckSync(ctx); err == nil && st != nil {
			localHeight = st.CurrentHeight
			networkHeight = st.NetworkHeight
			syncStatus = st.Status.String()
		}
	}

	// 3) gossip 订阅 peers（粗略反映“是否能收到共识结果广播”）
	var consensusPeers int
	var registeredProtocols int
	if s.networkService != nil {
		consensusPeers = len(s.networkService.GetTopicPeers(protocols.TopicConsensusResult))
		registeredProtocols = len(s.networkService.ListProtocols())
	}

	// 4) 聚合器侧（如果注入的是 aggregator.Manager，可通过 type assertion 拿到更多状态）
	var (
		aggState  string
		aggHeight uint64
		progress  *types.CollectionProgress
		distStats *types.DistanceStatistics
	)
	if s.aggregatorController != nil {
		if p, ok := any(s.aggregatorController).(interface {
			GetCurrentState() consensusif.AggregationState
			GetCurrentHeight() uint64
		}); ok {
			aggState = p.GetCurrentState().String()
			aggHeight = p.GetCurrentHeight()
		}
		if p, ok := any(s.aggregatorController).(interface {
			GetCollectionProgress(height uint64) (*types.CollectionProgress, error)
		}); ok {
			if cp, err := p.GetCollectionProgress(w.height); err == nil {
				progress = cp
			}
		}
		if p, ok := any(s.aggregatorController).(interface {
			GetDistanceStatistics() *types.DistanceStatistics
		}); ok {
			distStats = p.GetDistanceStatistics()
		}
	}

	elapsed := time.Since(w.startedAt)
	minPeer := 0
	enableAgg := false
	maxCandidates := 0
	if s.consensusOptions != nil {
		enableAgg = s.consensusOptions.Aggregator.EnableAggregator
		minPeer = s.consensusOptions.Aggregator.MinPeerThreshold
		maxCandidates = s.consensusOptions.Aggregator.MaxCandidates
	}
	msg := fmt.Sprintf("⏳ v2 确认阻塞诊断: expected=%d elapsed=%s submits=%d chainHeight=%d chainStatus=%s syncStatus=%v local=%d network=%d enableAggregator=%v minPeerThreshold=%d topicPeers(consensus)=%d protocols=%d aggState=%s aggHeight=%d",
		w.height, elapsed, w.submits, chainHeight, chainStatus, syncStatus, localHeight, networkHeight, enableAgg, minPeer, consensusPeers, registeredProtocols, aggState, aggHeight)
	if progress != nil {
		msg = fmt.Sprintf("%s collectionProgress={active:%v collected:%d validated:%d rejected:%d dup:%d maxCandidates:%d progress:%.2f%%}",
			msg,
			progress.IsActive,
			progress.CandidatesCollected,
			progress.CandidatesValidated,
			progress.CandidatesRejected,
			progress.DuplicatesDetected,
			maxCandidates,
			progress.ProgressPercentage*100,
		)
	}
	if distStats != nil {
		msg = fmt.Sprintf("%s distanceStats={total:%d avg:%s last:%s}",
			msg, distStats.TotalCalculations, distStats.AverageTime, distStats.LastCalculatedAt.Format(time.RFC3339))
	}
	if cause != nil {
		msg = fmt.Sprintf("%s cause=%v", msg, cause)
	}
	s.logger.Warn(msg)
}

// waitForBlockConfirmation 等待区块确认
// 通过定期检查链高度来判断区块是否已被网络确认
func (s *MiningOrchestratorService) waitForBlockConfirmation(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("开始监听区块确认")

	expectedHeight := minedBlock.Header.Height
	s.logger.Debugf("等待区块确认，期望高度: %d", expectedHeight)

	// 从配置获取检查间隔（配置必须提供有效值）
	checkInterval := s.minerConfig.ConfirmationCheckInterval
	if checkInterval <= 0 {
		return fmt.Errorf("配置错误：ConfirmationCheckInterval必须大于0，当前值: %v", checkInterval)
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 上下文超时或取消
			return fmt.Errorf("等待区块确认超时: %v", ctx.Err())

		case <-ticker.C:
			// 使用ChainService检查当前链高度
			if err := s.checkBlockConfirmation(ctx, expectedHeight); err != nil {
				s.logger.Debugf("区块确认检查失败: %v", err)
				continue // 继续等待
			}

			// 确认成功
			s.logger.Infof("区块确认成功，高度: %d", expectedHeight)
			return nil
		}
	}
}

// handleConfirmationTimeout 处理确认超时
// 当区块确认超时时的处理逻辑
func (s *MiningOrchestratorService) handleConfirmationTimeout(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("处理区块确认超时")

	// 1. 获取当前链状态进行诊断
	if s.chainQuery != nil {
		chainInfo, err := s.chainQuery.GetChainInfo(ctx)
		if err != nil {
			s.logger.Errorf("获取链状态失败: %v", err)
		} else {
			s.logger.Infof("确认超时诊断 - 当前高度: %d, 期望高度: %d, 链状态: %s",
				chainInfo.Height, minedBlock.Header.Height, chainInfo.Status)
		}
	}

	// 2. 返回超时错误
	return fmt.Errorf("区块确认超时，高度: %d", minedBlock.Header.Height)
}

// triggerSyncIfNeeded 触发同步
// 当确认失败时，主动触发同步以获取网络最新状态
func (s *MiningOrchestratorService) triggerSyncIfNeeded(ctx context.Context) error {
	s.logger.Info("触发网络同步以获取最新状态")

	// 同步服务是链管理的职责，这里仅在需要时触发一次同步请求
	if s.syncService == nil {
		return fmt.Errorf("同步服务未注入，无法触发系统同步")
	}

	// 🎯 先通过 CheckSync 实时查询同步状态，只在“确实落后网络高度”时才触发同步
	status, err := s.syncService.CheckSync(ctx)
	if err != nil {
		return fmt.Errorf("检查同步状态失败: %w", err)
	}
	if status == nil {
		return fmt.Errorf("同步状态为空，无法判断是否需要同步")
	}

	// 如果网络高度不高于本地高度，则认为当前不存在需要追赶的上游区块：
	// - 可能是单节点 / 无任何WES对端，仅有本地链；
	// - 也可能是本地高度已与网络持平或领先。
	// 这种情况下，同步应视为“无事可做”，而不是强行触发一次完整同步流程。
	if status.NetworkHeight <= status.CurrentHeight {
		s.logger.Infof("跳过同步：未发现更高的网络高度 (local=%d, network=%d, status=%v)",
			status.CurrentHeight, status.NetworkHeight, status.Status)
		return nil
	}

	// 仅当明确观测到 NetworkHeight > CurrentHeight 时，才真正触发一次系统同步
	if err := s.syncService.TriggerSync(ctx); err != nil {
		return fmt.Errorf("触发系统同步失败: %w", err)
	}

	s.logger.Info("系统同步触发成功，等待同步过程修复链状态")
	return nil
}

// updateHeightGate 更新高度门闸
// 无论确认成功与否，都需要更新已处理高度以防止重复挖矿
func (s *MiningOrchestratorService) updateHeightGate(height uint64) {
	s.logger.Info("更新高度门闸")

	// 更新已处理的最高高度
	s.heightGateService.UpdateLastProcessedHeight(height)

	s.logger.Info("高度门闸更新完成")
}

// ==================== 区块确认检查 ====================

// checkBlockConfirmation 检查区块是否已被确认
//
// 🎯 **确认检查逻辑**
//
// 通过ChainService检查当前链的状态，判断指定高度的区块是否已被网络确认。
//
// 参数：
//
//	ctx: 上下文对象
//	expectedHeight: 期望确认的区块高度
//
// 返回值：
//
//	error: 确认失败时的错误，nil表示已确认
func (s *MiningOrchestratorService) checkBlockConfirmation(ctx context.Context, expectedHeight uint64) error {
	// 1. 检查ChainService是否可用
	if s.chainQuery == nil {
		return fmt.Errorf("ChainQuery未注入")
	}

	// 2. 获取当前链信息
	chainInfo, err := s.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	currentHeight := chainInfo.Height
	s.logger.Debugf("当前链高度: %d, 期望高度: %d", currentHeight, expectedHeight)

	// 3. 检查高度是否已达到或超过期望高度
	if currentHeight >= expectedHeight {
		// 区块已确认
		return nil
	}

	// 4. 高度未达到，继续等待
	return fmt.Errorf("区块尚未确认，当前高度: %d, 期望高度: %d", currentHeight, expectedHeight)
}

// validateChainHeightBeforeGateUpdate 在更新门闸前验证链高度
//
// 🔒 **防御性验证**
//
// 在确认成功后，更新门闸前再次验证链高度，确保门闸不会超前于实际链高度。
// 这是防止门闸与链状态不一致的最后一道防线。
//
// 参数：
//
//	ctx: 上下文对象
//	expectedHeight: 期望的区块高度
//
// 返回值：
//
//	error: 验证失败时的错误，nil表示验证通过
func (s *MiningOrchestratorService) validateChainHeightBeforeGateUpdate(ctx context.Context, expectedHeight uint64) error {
	// 获取当前链信息
	if s.chainQuery == nil {
		return fmt.Errorf("ChainQuery未注入")
	}

	chainInfo, err := s.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	currentHeight := chainInfo.Height
	s.logger.Infof("门闸更新前验证 - 当前链高度: %d, 期望高度: %d", currentHeight, expectedHeight)

	// 严格验证：链高度必须大于等于期望高度
	if currentHeight < expectedHeight {
		return fmt.Errorf("链高度验证失败：当前高度 %d 小于期望高度 %d", currentHeight, expectedHeight)
	}

	s.logger.Info("链高度验证通过，允许更新门闸")
	return nil
}
