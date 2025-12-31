// Package orchestrator 实现挖矿编排器的核心业务编排功能
//
// ⚡ **挖矿轮次执行模块**
//
// 本文件实现 ExecuteMiningRound 方法的具体业务逻辑，包含完整的挖矿轮次编排流程：
// 1. 挖矿前置条件检查（高度门闸、矿工状态、网络状态等）
// 2. 候选区块创建（调用区块服务获取候选区块模板）
// 3. PoW计算协调（调用PoW处理器执行工作量证明）
// 4. 区块提交处理（通过Aggregator接口提交挖出的区块）
// 5. 确认等待管理（等待区块确认或超时触发同步）
package orchestrator

import (
	"context"
	"fmt"
	"time"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/types"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// executeMiningRound 执行一轮完整的挖矿业务编排流程
// 这是 ExecuteMiningRound 公共接口方法的具体实现，遵循薄封装原则
func (s *MiningOrchestratorService) executeMiningRound(ctx context.Context) error {
	s.logger.Info("开始执行挖矿轮次编排")

	// 在进入挖矿轮次前检查节点运行模式（生产环境安全兜底）
	// - RepairingUTXO / ReadOnly 模式下不执行挖矿，优先保证状态修复或只读安全
	if !runtimectx.IsMiningAllowed() {
		mode := runtimectx.GetNodeMode()
		if s.logger != nil {
			s.logger.Warnf("当前节点运行模式为 %s，本轮挖矿将被跳过", mode.String())
		}
		return fmt.Errorf("当前节点运行模式不允许挖矿: %s", mode.String())
	}

	// 🆕 2025-12-18 优化：渐进式 IO 高压减速
	//
	// 原问题：82 次警告，每次硬编码减速 2 秒，共减速约 164 秒
	//
	// 优化策略：
	// 1. 区分 Warning（500ms）和 Critical（2s）减速时间
	// 2. 连续正常 3 次后可豁免一次 Warning 级别减速
	// 3. 输出具体触发指标（QPS/延迟/Goroutine/FD），便于问题定位
	shouldSlowdown, slowDownDelay, reason := metricsutil.ShouldSlowdown()
	if shouldSlowdown && slowDownDelay > 0 {
		// 获取诊断信息，输出具体触发原因
		diag := metricsutil.GetIOPressureDiagnostic()
		s.logger.Warnf("检测到 IO 高压状态，本轮挖矿前减速 %s (reason=%s, triggers=%v, qps=%.1f, lat=%.1fms, goroutines=%d, fd_usage=%.1f%%)",
			slowDownDelay, reason, diag.Triggers,
			diag.EMAQPS, diag.EMALatency*1000, diag.Goroutines, diag.FDUsage*100)

		select {
		case <-time.After(slowDownDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	} else if reason == "exempt_by_consecutive_normal" {
		// 豁免日志（Debug 级别）
		s.logger.Debugf("IO 高压检测到 Warning 但因连续正常而豁免减速")
	}

	// 1. 检查挖矿前置条件
	if err := s.checkPreconditions(ctx); err != nil {
		s.logger.Info("前置条件检查失败")
		return fmt.Errorf("前置条件检查失败: %v", err)
	}

	// 1.5 v2：最小出块间隔/MTP 窗口等待（非向后兼容：防止喷发式出块）
	if err := s.waitForMiningSlot(ctx); err != nil {
		s.logger.Info("挖矿窗口等待失败")
		return fmt.Errorf("挖矿窗口等待失败: %v", err)
	}

	// 2~5：构建候选 + PoW + 提交 + 等待确认
	//
	// v2（共识关键约束）：
	// - Difficulty 的 expectedDifficulty 绑定 block.Header.Timestamp（NextDifficultyForTimestamp）；
	// - PoW 引擎在 nonce 搜索期间禁止滚动 Timestamp（否则 got/expected 不一致，区块会被拒绝）。
	//
	// ⚠️ 重要变更（按真实 PoW 语义）：
	// - 不在编排层强制引入 “PoW slice(5s/10s/…) 超时 => 重建候选块” 的策略；
	// - PoW 是概率过程，slice 会在高难度/低算力下持续触发 ctx deadline，表现为“高度卡死/有效算力下降”；
	// - 是否限制挖矿时间应由外部 ctx 或配置显式开启（miner.mining_timeout），默认不限制。
	roundCtx := ctx
	var roundCancel context.CancelFunc
	if s.minerConfig != nil && s.minerConfig.MiningTimeout > 0 {
		roundCtx, roundCancel = context.WithTimeout(ctx, s.minerConfig.MiningTimeout)
		defer roundCancel()
	}

	for {
		if err := roundCtx.Err(); err != nil {
			return err
		}

		// 2. 创建候选区块（每次都重新构建，刷新 timestamp/difficulty）
		candidateBlock, err := s.createCandidateBlock(roundCtx)
		if err != nil {
			s.logger.Info("创建候选区块失败")
			return fmt.Errorf("创建候选区块失败: %v", err)
		}

		// 2.5. 合规性二次验证（双重保险）
		if err := s.validateBlockCompliance(roundCtx, candidateBlock); err != nil {
			s.logger.Info("候选区块合规验证失败")
			return fmt.Errorf("候选区块合规验证失败: %v", err)
		}

		// 3. 执行PoW计算（不做 slice 限制；仅响应外部 ctx 的取消/超时）
		minedBlock, err := s.executePoWComputation(roundCtx, candidateBlock)
		if err != nil {
			s.logger.Info("PoW计算失败")
			return fmt.Errorf("PoW计算失败: %w", err)
		}

		// 4. 提交挖出的区块
		if err := s.submitMinedBlock(roundCtx, minedBlock); err != nil {
			s.logger.Info("区块提交失败")
			return fmt.Errorf("区块提交失败: %v", err)
		}

		// 5. 等待确认（委托给 wait_confirmation.go 实现）
		if err := s.waitForConfirmation(roundCtx, minedBlock); err != nil {
			s.logger.Info("确认等待失败")
			return fmt.Errorf("确认等待失败: %v", err)
		}

		break
	}

	s.logger.Info("挖矿轮次编排执行完成")
	return nil
}

// CheckMiningGate 检查挖矿门闸（V2）。
//
// 语义：
// - 若不满足“网络法定人数 + 高度一致性 + 链尖前置条件”，必须返回 error（硬门槛）。
// - 供 StartMining/StartMiningOnce 与每轮 ExecuteMiningRound 复用（双保险）。
func (s *MiningOrchestratorService) CheckMiningGate(ctx context.Context) error {
	if s.quorumChecker == nil {
		return fmt.Errorf("挖矿门闸检查器未注入，拒绝挖矿（避免错误出块）")
	}
	res, err := s.quorumChecker.Check(ctx)
	if err != nil {
		return fmt.Errorf("挖矿门闸检查失败: %w", err)
	}
	if res != nil && !res.AllowMining {
		return fmt.Errorf("挖矿门闸未通过: %s", res.Reason)
	}
	return nil
}

// checkPreconditions 检查挖矿前置条件
// 包括高度门闸检查、矿工状态验证、网络连接检查等
func (s *MiningOrchestratorService) checkPreconditions(ctx context.Context) error {
	s.logger.Info("开始检查挖矿前置条件")

	// 1. 检查矿工状态
	minerState := s.stateManagerService.GetMinerState()
	if minerState != types.MinerStateActive {
		return fmt.Errorf("矿工未处于挖矿状态，当前状态: %v", minerState)
	}

	// 1.5 检查链是否就绪（硬前置条件）
	// - IsDataFresh 在当前实现中已废弃且始终返回 false（会导致永远无法挖矿），因此这里改为 IsReady；
	// - IsReady=false 通常表示创世尚未提交或关键状态未完成初始化，此时禁止进入挖矿轮次是合理的。
	if s.chainQuery == nil {
		return fmt.Errorf("链查询服务未注入，无法检查链就绪状态")
	}
	ready, err := s.chainQuery.IsReady(ctx)
	if err != nil {
		return fmt.Errorf("检查链就绪状态失败: %v", err)
	}
	if !ready {
		return fmt.Errorf("链尚未就绪，拒绝进入本轮挖矿")
	}

	// 2. v2：挖矿稳定性门闸（硬前置）
	// - 禁止孤岛挖矿（除非 allow_single_node_mining=true 且通过配置验证的 dev+from_genesis 场景）
	// - 必须网络确认（法定人数达标 + 高度一致性确认）
	if err := s.CheckMiningGate(ctx); err != nil {
		return err
	}

	// 3. 检查高度门闸 - 防止重复挖矿
	if err := s.checkHeightGate(ctx); err != nil {
		return fmt.Errorf("高度门闸检查失败: %v", err)
	}

	s.logger.Info("前置条件检查通过")
	return nil
}

// createCandidateBlock 创建挖矿区块
// 🎯 **哈希+缓存架构**：从BlockService获取候选区块哈希，然后从缓存获取真实区块
//
// 遵循项目标准的哈希+缓存模式：
// 1. BlockService.CreateMiningCandidate 返回32字节区块哈希
// 2. 候选区块存储在内存缓存中，通过哈希检索
// 3. 矿工获取包含交易的完整候选区块进行PoW计算
func (s *MiningOrchestratorService) createCandidateBlock(ctx context.Context) (*blocktypes.Block, error) {
	s.logger.Info("开始创建挖矿区块")

	// 1. 从 BlockBuilder 获取候选区块哈希
	candidateHash, err := s.blockBuilder.CreateMiningCandidate(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建候选区块失败: %v", err)
	}

	if len(candidateHash) != 32 {
		return nil, fmt.Errorf("无效的候选区块哈希长度: %d", len(candidateHash))
	}

	// 2. 使用 BlockBuilder 的 GetCachedCandidate 方法获取候选区块
	// 🔧 修复：直接使用 BlockBuilder 的缓存方法，而不是从 MemoryStore 获取
	// BlockBuilder 内部使用 LRU 缓存存储候选区块
	candidateBlock, err := s.blockBuilder.GetCachedCandidate(ctx, candidateHash)
	if err != nil {
		return nil, fmt.Errorf("获取候选区块失败: %v", err)
	}

	if candidateBlock == nil {
		return nil, fmt.Errorf("候选区块为nil, 哈希: %x", candidateHash)
	}

	// 检查区块头和区块体是否为 nil
	if candidateBlock.Header == nil {
		return nil, fmt.Errorf("候选区块头为nil, 哈希: %x", candidateHash)
	}

	if candidateBlock.Body == nil {
		return nil, fmt.Errorf("候选区块体为nil, 哈希: %x", candidateHash)
	}

	s.logger.Infof("✅ 成功获取候选区块, 哈希: %x, 高度: %d, 交易数: %d",
		candidateHash[:8], candidateBlock.Header.Height, len(candidateBlock.Body.Transactions))

	return candidateBlock, nil
}

// executePoWComputation 执行PoW计算
// 协调 PoW 计算处理器执行工作量证明
func (s *MiningOrchestratorService) executePoWComputation(ctx context.Context, candidateBlock *blocktypes.Block) (*blocktypes.Block, error) {
	s.logger.Info("开始执行PoW计算")

	// 调用PoW处理器从候选区块模板生成挖出的区块
	// 注意：PoW处理器返回 interface{} 类型，需要类型断言
	minedBlockInterface, err := s.powHandlerService.ProduceBlockFromTemplate(ctx, candidateBlock)
	if err != nil {
		return nil, fmt.Errorf("PoW计算失败: %w", err)
	}

	// 类型断言为区块类型
	minedBlock, ok := minedBlockInterface.(*blocktypes.Block)
	if !ok {
		return nil, fmt.Errorf("无效的PoW返回类型")
	}

	s.logger.Info("PoW计算完成")
	return minedBlock, nil
}

// submitMinedBlock 提交挖出的区块
// 通过内部接口向Aggregator委托挖出的区块，遵循分布式架构规范
func (s *MiningOrchestratorService) submitMinedBlock(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("开始提交挖出的区块")

	// 使用内部接口向Aggregator委托区块
	// 避免直接调用 blockService.ProcessBlock，遵循"单一写入入口"约束
	// Aggregator负责处理网络协议、K-bucket路由等复杂逻辑
	if err := s.submitBlockToAggregator(ctx, minedBlock); err != nil {
		return fmt.Errorf("向Aggregator提交区块失败: %v", err)
	}

	s.logger.Info("区块已成功提交给Aggregator")
	return nil
}

// ==================== 高度门闸检查 ====================

// checkHeightGate 检查高度门闸以防止同高度重复挖矿
//
// 🎯 **高度门闸逻辑**
//
// 对比当前链高度和已处理的高度，确保不会在同一高度重复挖矿：
// 1. 获取当前链的最新高度
// 2. 获取高度门闸记录的已处理高度
// 3. 只有当链高度推进时才允许挖矿
//
// 参数：
//
//	ctx: 上下文对象
//
// 返回值：
//
//	error: 高度检查失败时的错误，nil表示可以挖矿
func (s *MiningOrchestratorService) checkHeightGate(ctx context.Context) error {
	// 1. 获取当前链高度
	if s.chainQuery == nil {
		return fmt.Errorf("ChainQuery未注入，无法检查高度")
	}

	chainInfo, err := s.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	currentChainHeight := chainInfo.Height

	// 2. 获取已处理的高度
	lastProcessedHeight := s.heightGateService.GetLastProcessedHeight()

	s.logger.Debugf("高度门闸检查 - 当前链高度: %d, 已处理高度: %d",
		currentChainHeight, lastProcessedHeight)

	// 3. 高度门闸逻辑检查
	// 特殊情况：如果当前链高度和已处理高度都为0，说明是初始状态，允许挖矿
	if currentChainHeight == 0 && lastProcessedHeight == 0 {
		s.logger.Info("检测到初始状态（链高度=0，已处理高度=0），允许开始挖矿")
	} else if currentChainHeight < lastProcessedHeight {
		// 只有当前高度小于已处理高度时才阻止（分叉回退情况）
		return fmt.Errorf("检测到分叉回退（current < lastProcessed），当前高度: %d, 已处理高度: %d",
			currentChainHeight, lastProcessedHeight)
	} else if currentChainHeight == lastProcessedHeight {
		// 当前高度等于已处理高度时，允许挖下一个区块
		s.logger.Info("当前高度等于已处理高度，允许挖掘下一个区块")
	}

	// 3.5 v2：确认门闸退路（非阻塞）下的“提交节流”
	// 防止在同一高度确认长期未达成时，矿工以极高频率反复提交候选导致网络/聚合器被打爆。
	//
	// 策略：如果当前高度+1 存在未完成的确认 watcher，则在两次提交之间至少间隔 ConfirmationResubmitMinInterval。
	if s.minerConfig != nil && s.minerConfig.ConfirmationResubmitMinInterval > 0 {
		expectedHeight := currentChainHeight + 1
		var wait time.Duration
		s.confirmMu.Lock()
		if w := s.confirmWatches[expectedHeight]; w != nil {
			since := time.Since(w.lastSubmitAt)
			if since < s.minerConfig.ConfirmationResubmitMinInterval {
				wait = s.minerConfig.ConfirmationResubmitMinInterval - since
			}
		}
		s.confirmMu.Unlock()

		if wait > 0 {
			if s.logger != nil {
				s.logger.Infof("⏳ v2 提交节流：等待 %s 后再尝试提交同高度候选（height=%d）", wait, expectedHeight)
			}
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	// 4. 检查高度推进是否合理（避免跳跃过大）
	if currentChainHeight > lastProcessedHeight+1 {
		// 高度跳跃过大，可能需要同步
		s.logger.Warnf("检测到高度跳跃：从 %d 到 %d，可能需要同步",
			lastProcessedHeight, currentChainHeight)

		// 触发同步但不阻止挖矿（允许矿工追赶）
		// 🎯 语义说明：TriggerSync 在无上游节点时会返回 nil（视为无事可做），只有真正的同步失败才会返回 error
		if s.syncService != nil {
			if s.logger != nil {
				s.logger.Infof("⏩ 即将调用同步服务，补齐缺失区块: %d → %d",
					lastProcessedHeight+1, currentChainHeight-1)
			}
			if err := s.syncService.TriggerSync(ctx); err != nil {
				if s.logger != nil {
					s.logger.Warnf("触发同步失败（真正的同步错误）: %v", err)
				}
				// 同步触发失败不阻止挖矿，仅记录告警
			} else if s.logger != nil {
				// err == nil 可能表示同步完成或当前无上游节点（无事可做）
				s.logger.Infof("✅ 同步流程已执行，尝试补齐缺失区块: %d 到 %d（可能已完成同步，或当前无上游节点）",
					lastProcessedHeight+1, currentChainHeight-1)
			}
		} else if s.logger != nil {
			s.logger.Errorf("❌ 无法触发同步：syncService 未注入（current=%d, lastProcessed=%d）",
				currentChainHeight, lastProcessedHeight)
		}
	}

	s.logger.Info("高度门闸检查通过，允许挖矿")
	return nil
}

// validateBlockCompliance 验证候选区块的合规性（双重保险）
//
// 🔒 **共识层合规验证 (Consensus Layer Compliance Validation)**
//
// 在矿工编排器中对候选区块进行二次合规验证，作为内存池合规检查的双重保险。
// 虽然交易池已经在GetTransactionsForMining()中进行了合规过滤，
// 但在共识层再次验证确保没有不合规交易进入区块。
//
// 验证范围：
// 1. 验证区块中所有普通交易的合规性
// 2. 跳过Coinbase交易（系统生成的奖励交易）
// 3. 记录详细的合规检查统计信息
//
// 参数：
// - ctx: 上下文对象
// - candidateBlock: 待验证的候选区块
//
// 返回：
// - error: 如果发现不合规交易则返回错误，nil表示所有交易都合规
func (s *MiningOrchestratorService) validateBlockCompliance(ctx context.Context, candidateBlock *blocktypes.Block) error {
	// 如果未配置合规策略，跳过检查
	if s.compliancePolicy == nil {
		s.logger.Debug("未配置合规策略，跳过共识层合规检查")
		return nil
	}

	s.logger.Info("🔒 开始共识层合规验证（双重保险）")

	transactions := candidateBlock.Body.Transactions
	if len(transactions) == 0 {
		s.logger.Info("候选区块无交易，跳过合规验证")
		return nil
	}

	// 创建交易来源信息
	source := &complianceIfaces.TransactionSource{
		Protocol:  "consensus_miner",
		Timestamp: time.Now(),
	}

	validCount := 0
	rejectedCount := 0

	// 验证所有交易的合规性
	for i, tx := range transactions {
		// 跳过Coinbase交易（第一个交易通常是Coinbase）
		if i == 0 {
			// 简单判断：第一个交易且没有输入的可能是Coinbase交易
			if len(tx.Inputs) == 0 {
				s.logger.Debug("跳过Coinbase交易的合规检查")
				continue
			}
		}

		// 执行合规检查
		decision, err := s.compliancePolicy.CheckTransaction(ctx, tx, source)
		if err != nil {
			s.logger.Errorf("合规策略检查失败: %v", err)
			return fmt.Errorf("合规策略检查失败: %v", err)
		}

		if !decision.Allowed {
			// 发现不合规交易，这表明内存池的合规检查可能被绕过或失效
			rejectedCount++
			s.logger.Errorf("🚨 共识层发现不合规交易！原因=%s, 详情=%s, 国家=%s, 信息源=%s",
				decision.Reason, decision.ReasonDetail, decision.Country, decision.Source)

			return fmt.Errorf("候选区块包含不合规交易: %s (%s)",
				decision.Reason, decision.ReasonDetail)
		}

		validCount++
	}

	// 记录合规验证结果
	if rejectedCount > 0 {
		s.logger.Warnf("🔒 共识层合规验证完成：有效交易 %d 笔，拒绝交易 %d 笔",
			validCount, rejectedCount)
	} else {
		s.logger.Infof("🔒 共识层合规验证通过：所有 %d 笔交易均符合合规要求", validCount)
	}

	return nil
}
