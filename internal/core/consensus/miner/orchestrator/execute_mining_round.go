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

	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// executeMiningRound 执行一轮完整的挖矿业务编排流程
// 这是 ExecuteMiningRound 公共接口方法的具体实现，遵循薄封装原则
func (s *MiningOrchestratorService) executeMiningRound(ctx context.Context) error {
	s.logger.Info("开始执行挖矿轮次编排")

	// 1. 检查挖矿前置条件
	if err := s.checkPreconditions(ctx); err != nil {
		s.logger.Info("前置条件检查失败")
		return fmt.Errorf("前置条件检查失败: %v", err)
	}

	// 2. 创建候选区块
	candidateBlock, err := s.createCandidateBlock(ctx)
	if err != nil {
		s.logger.Info("创建候选区块失败")
		return fmt.Errorf("创建候选区块失败: %v", err)
	}

	// 2.5. 合规性二次验证（双重保险）
	if err := s.validateBlockCompliance(ctx, candidateBlock); err != nil {
		s.logger.Info("候选区块合规验证失败")
		return fmt.Errorf("候选区块合规验证失败: %v", err)
	}

	// 3. 执行PoW计算
	minedBlock, err := s.executePoWComputation(ctx, candidateBlock)
	if err != nil {
		s.logger.Info("PoW计算失败")
		return fmt.Errorf("PoW计算失败: %v", err)
	}

	// 4. 提交挖出的区块
	if err := s.submitMinedBlock(ctx, minedBlock); err != nil {
		s.logger.Info("区块提交失败")
		return fmt.Errorf("区块提交失败: %v", err)
	}

	// 5. 等待确认（委托给 wait_confirmation.go 实现）
	if err := s.waitForConfirmation(ctx, minedBlock); err != nil {
		s.logger.Info("确认等待失败")
		return fmt.Errorf("确认等待失败: %v", err)
	}

	s.logger.Info("挖矿轮次编排执行完成")
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

	// 2. 检查系统同步状态 - 确保网络同步完成
	syncStatus, err := s.syncService.CheckSync(ctx)
	if err != nil {
		return fmt.Errorf("检查同步状态失败: %v", err)
	}
	if syncStatus.Status == types.SyncStatusSyncing {
		return fmt.Errorf("系统正在同步中，无法开始挖矿")
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

	// 1. 从 BlockService 获取候选区块哈希
	candidateHash, err := s.blockService.CreateMiningCandidate(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建候选区块失败: %v", err)
	}

	if len(candidateHash) != 32 {
		return nil, fmt.Errorf("无效的候选区块哈希长度: %d", len(candidateHash))
	}

	// 2. 使用哈希作为缓存键从内存缓存获取候选区块
	// 注意：缓存键格式必须与 BlockService.storeCandidateBlock 保持一致
	cacheKey := fmt.Sprintf("candidate_block:%x", candidateHash)
	candidateData, exists, err := s.cacheStore.Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("从缓存获取候选区块失败: %v", err)
	}
	if !exists {
		return nil, fmt.Errorf("候选区块不在缓存中, 哈希: %x", candidateHash)
	}

	// 3. 反序列化候选区块数据
	candidateBlock := &blocktypes.Block{}
	if err := proto.Unmarshal(candidateData, candidateBlock); err != nil {
		return nil, fmt.Errorf("候选区块反序列化失败: %v", err)
	}

	s.logger.Infof("成功获取候选区块, 哈希: %x, 高度: %d, 交易数: %d",
		candidateHash, candidateBlock.Header.Height, len(candidateBlock.Body.Transactions))

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
		return nil, fmt.Errorf("PoW计算失败: %v", err)
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
	if s.chainService == nil {
		return fmt.Errorf("ChainService未注入，无法检查高度")
	}

	chainInfo, err := s.chainService.GetChainInfo(ctx)
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

	// 4. 检查高度推进是否合理（避免跳跃过大）
	if currentChainHeight > lastProcessedHeight+1 {
		// 高度跳跃过大，可能需要同步
		s.logger.Warnf("检测到高度跳跃：从 %d 到 %d，可能需要同步",
			lastProcessedHeight, currentChainHeight)

		// 触发同步但不阻止挖矿（允许矿工追赶）
		if syncErr := s.syncService.TriggerSync(ctx); syncErr != nil {
			s.logger.Warnf("触发同步失败: %v", syncErr)
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
