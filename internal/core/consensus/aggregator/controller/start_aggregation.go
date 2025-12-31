// start_aggregation.go
// 启动聚合轮次的业务逻辑实现
//
// 核心业务功能：
// 1. 启动指定高度的聚合轮次处理
// 2. 检查聚合节点资格
// 3. 初始化聚合流程状态
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package controller

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	chainsync "github.com/weisyn/v1/internal/core/chain/sync"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	kbucketimpl "github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	"github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	blockiface "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// aggregationStarter 聚合轮次启动器
type aggregationStarter struct {
	logger       log.Logger
	stateManager interfaces.AggregatorStateManager
	// 添加编排所需的子组件
	election           interfaces.AggregatorElection
	candidateCollector interfaces.CandidateCollector
	decisionCalculator interfaces.DecisionCalculator
	distanceSelector   interfaces.DistanceSelector // 距离选择器
	resultDistributor  interfaces.ResultDistributor
	// 新增网络和候选池依赖
	candidatePool  mempool.CandidatePool
	networkService netiface.Network
	p2pService     p2pi.Service
	// 新增K桶管理器依赖，用于清理不兼容的外部节点
	routingTableManager kademlia.RoutingTableManager
	// 配置依赖
	config *consensusconfig.ConsensusOptions
	// 新增链查询与区块哈希服务依赖，用于获取真实父块哈希
	chainQuery      persistence.QueryService
	blockHashClient block.BlockHashServiceClient
	// 区块处理服务，用于处理选中的区块
	blockProcessor blockiface.BlockProcessor

	// V2 新增：收集窗口结束时间（用于状态查询）
	collectionWindowEndTime map[uint64]uint64 // height -> unix_timestamp
	collectionWindowMu      sync.RWMutex

	// 🆕 2025-12-18: 聚合流程互斥锁
	// 防止并发聚合流程导致状态机竞态
	aggregationFlowMu sync.Mutex
}

// newAggregationStarter 创建聚合轮次启动器
func newAggregationStarter(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
	election interfaces.AggregatorElection,
	candidateCollector interfaces.CandidateCollector,
	decisionCalculator interfaces.DecisionCalculator,
	distanceSelector interfaces.DistanceSelector,
	resultDistributor interfaces.ResultDistributor,
	candidatePool mempool.CandidatePool,
	networkService netiface.Network,
	p2pService p2pi.Service,
	routingTableManager kademlia.RoutingTableManager,
	config *consensusconfig.ConsensusOptions, // 添加配置参数
	chainQuery persistence.QueryService,
	blockHashClient block.BlockHashServiceClient,
	blockProcessor blockiface.BlockProcessor, // 区块处理服务
) *aggregationStarter {
	return &aggregationStarter{
		logger:                  logger,
		stateManager:            stateManager,
		election:                election,
		candidateCollector:      candidateCollector,
		decisionCalculator:      decisionCalculator,
		distanceSelector:        distanceSelector,
		resultDistributor:       resultDistributor,
		candidatePool:           candidatePool,
		networkService:          networkService,
		p2pService:              p2pService,
		routingTableManager:     routingTableManager,
		config:                  config, // 保存配置引用
		chainQuery:              chainQuery,
		blockHashClient:         blockHashClient,
		blockProcessor:          blockProcessor,
		collectionWindowEndTime: make(map[uint64]uint64), // V2 新增
	}
}

// processAggregationRound 处理区块聚合轮次（新的统一入口）
//
// 🎯 **新的统一处理逻辑**：
// 1. 聚合节点选举判断
// 2. 非聚合节点：转发给正确的聚合节点
// 3. 聚合节点：添加到候选池并触发聚合流程
func (s *aggregationStarter) processAggregationRound(ctx context.Context, candidateBlock *block.Block) error {
	s.logger.Info("开始处理区块聚合轮次")

	// 检查候选区块是否为 nil
	if candidateBlock == nil {
		return fmt.Errorf("候选区块不能为空")
	}

	// 检查区块头是否为 nil
	if candidateBlock.Header == nil {
		return fmt.Errorf("候选区块头不能为空")
	}

	// 1. 聚合节点选举判断
	height := candidateBlock.Header.Height

	// 全局写门闸：只读/写围栏下禁止启动聚合（返回弃权错误以触发转发）
	if err := writegate.Default().AssertWriteAllowed(ctx, "aggregator.processAggregationRound"); err != nil {
		// 确保处于 Idle 状态（幂等操作）
		if transErr := s.stateManager.EnsureIdle(); transErr != nil {
			s.logger.Warnf("只读模式下无法确保Idle状态: %v", transErr)
			// 转换失败不影响弃权流程，继续返回弃权错误
		}

		// 获取当前链高度（用于诊断）
		localHeight := uint64(0)
		if s.chainQuery != nil {
			if ci, err := s.chainQuery.GetChainInfo(ctx); err == nil && ci != nil {
				localHeight = ci.Height
			}
		}

		// 记录只读模式弃权指标
		recordWaiver("read_only_mode")

		// 返回弃权错误，触发自动转发
		return &types.WaiverError{
			Reason:      types.WaiverReasonReadOnlyMode,
			LocalHeight: localHeight,
			Height:      height,
		}
	}

	// ====== 生产级高度门槛：拒绝过旧/过远未来的候选，避免收集窗口被噪声打爆 ======
	// 背景：公网/多网络环境下可能收到“旧高度/外网高度”的提交；若为其开聚合流程会导致大量 warn 和状态抖动。
	if s.chainQuery != nil {
		if ci, err := s.chainQuery.GetChainInfo(ctx); err == nil && ci != nil {
			localHeight := ci.Height
			if s.logger != nil {
				s.logger.Debugf("height.gate: local_height=%d candidate_height=%d", localHeight, height)
			}
			// 1) 旧高度：直接拒绝（让对端停止重发），不进入选举/收集
			if height <= localHeight {
				if s.logger != nil {
					// stale 在网络中很常见（重传/乱序/对端尚未收敛），不应刷屏为 WARN
					s.logger.Infof("⏩ height.gate: stale candidate ignored (candidate=%d local=%d)", height, localHeight)
				}
				return fmt.Errorf("stale candidate height: candidate=%d local=%d", height, localHeight)
			}
			// 2) 远未来高度：返回弃权错误（V2 新增）
			const maxFutureSkew = 8
			if height > localHeight+maxFutureSkew {
				// 尝试触发一次同步（非阻塞语义：失败也不影响拒绝）
				if s.config != nil {
					// syncService 不在 starter 中，使用候选验证器中的同步闭环；此处仅做硬拒绝避免噪声
				}
				if s.logger != nil {
					s.logger.Warnf("🚫 height.gate: candidate too far ahead, waiving (candidate=%d local=%d skew=%d max=%d)",
						height, localHeight, height-localHeight, maxFutureSkew)
				}
				// 记录高度过高弃权指标
				recordWaiver("height_too_far_ahead")
				// V2 新增：返回弃权错误而非普通错误
				return &types.WaiverError{
					Reason:      types.WaiverReasonHeightTooFarAhead,
					LocalHeight: localHeight,
					Height:      height,
				}
			}
		} else if err != nil {
			// ⚠️ 关键可观测性：如果这里失败，上层会直接进入选举判断，容易卡死/误判；必须打日志
			if s.logger != nil {
				s.logger.Warnf("⚠️ height.gate: GetChainInfo failed, skipping height gate (candidate=%d err=%v)", height, err)
			}
		} else if ci == nil {
			if s.logger != nil {
				s.logger.Warnf("⚠️ height.gate: GetChainInfo returned nil, skipping height gate (candidate=%d)", height)
			}
		}
	} else if s.logger != nil {
		s.logger.Warnf("⚠️ height.gate: chainQuery not injected, skipping height gate (candidate=%d)", height)
	}

	// 通过高度门槛后，才进入选举判断（避免 stale 噪声把“开始选举”刷屏）
	s.logger.Infof("🔍 开始聚合器选举判断，区块高度: %d", height)

	isAggregator, err := s.election.IsAggregatorForHeight(height)
	if err != nil {
		s.logger.Errorf("❌ 聚合器选举失败: %v", err)
		return fmt.Errorf("aggregator election failed: %v", err)
	}

	if !isAggregator {
		// 2. 不是聚合节点，转发给正确的聚合节点
		s.logger.Infof("🔄 当前节点不是高度 %d 的聚合节点，进行转发", height)
		// V2 新增：从 context 中读取 submission 信息（如果存在）
		var waivedAggregators []peer.ID
		var retryAttempt uint32
		var originalMinerPeerID peer.ID
		if submissionInfo, ok := SubmissionInfoFromContext(ctx); ok {
			waivedAggregators = submissionInfo.WaivedAggregators
			retryAttempt = submissionInfo.RetryAttempt
			originalMinerPeerID = submissionInfo.OriginalMinerPeerID
		}
		return s.forwardBlockToCorrectAggregator(ctx, candidateBlock, waivedAggregators, retryAttempt, originalMinerPeerID)
	}

	// 3. 是聚合节点，检查聚合状态（V2 新增：弃权检查）
	currentState := s.stateManager.GetCurrentState()
	if currentState != types.AggregationStateIdle {
		// 聚合器正忙，返回弃权错误
		// 记录聚合进行中弃权指标
		recordWaiver("aggregation_in_progress")
		if s.chainQuery != nil {
			if ci, err := s.chainQuery.GetChainInfo(ctx); err == nil && ci != nil {
				s.logger.Warnf("🚫 聚合器正忙，弃权: height=%d state=%v local_height=%d", height, currentState, ci.Height)
				return &types.WaiverError{
					Reason:      types.WaiverReasonAggregationInProgress,
					LocalHeight: ci.Height,
					Height:      height,
				}
			}
		}
		s.logger.Warnf("🚫 聚合器正忙，弃权: height=%d state=%v", height, currentState)
		return &types.WaiverError{
			Reason:      types.WaiverReasonAggregationInProgress,
			LocalHeight: 0,
			Height:      height,
		}
	}

	// 4. 是聚合节点且空闲，添加到候选池并触发聚合流程
	s.logger.Infof("✅ 确认为高度 %d 的聚合节点，开始本地处理候选区块", height)

	// 添加到候选池
	// ✅ 关键：候选来源必须来自“上下文 peer hint”（远端提交时由网络层写入），而不是本地 Host().ID 的原始 bytes。
	// - peer.ID 的底层是 multihash bytes（不是 UTF-8 字符串），直接 string(pid) 会导致日志乱码/peer hint 失效。
	// - 本地挖出的候选保持 fromPeer=""，由 CandidateBlock.LocalNode 语义标记为本地来源。
	fromPeer := ""
	if hint, ok := chainsync.PeerHintFromContext(ctx); ok && hint != "" {
		fromPeer = hint.String()
	}
	blockHash, err := s.candidatePool.AddCandidate(candidateBlock, fromPeer)
	if err != nil {
		return fmt.Errorf("failed to add candidate to pool: %v", err)
	}
	s.logger.Infof("候选区块已添加到候选池，哈希前缀: %s", hex.EncodeToString(blockHash)[:8])

	// 触发聚合流程
	return s.executeAggregationFlow(ctx, height)

}

// SubmissionInfo 提交信息（从 context 传递）
type SubmissionInfo struct {
	WaivedAggregators   []peer.ID
	RetryAttempt        uint32
	OriginalMinerPeerID peer.ID
}

type submissionInfoKey struct{}

// ContextWithSubmissionInfo 将提交信息写入 context
func ContextWithSubmissionInfo(ctx context.Context, info *SubmissionInfo) context.Context {
	return context.WithValue(ctx, submissionInfoKey{}, info)
}

// SubmissionInfoFromContext 从 context 读取提交信息
func SubmissionInfoFromContext(ctx context.Context) (*SubmissionInfo, bool) {
	info, ok := ctx.Value(submissionInfoKey{}).(*SubmissionInfo)
	return info, ok
}

// forwardBlockToCorrectAggregator 转发区块给正确的聚合节点
//
// V2 新增：支持弃权与重选机制
// - waivedAggregators: 已弃权的聚合器节点ID列表（避免回环）
// - retryAttempt: 重试次数（从0开始，每次重选+1）
// - originalMinerPeerID: 原始矿工节点ID（用于回环检测）
func (s *aggregationStarter) forwardBlockToCorrectAggregator(
	ctx context.Context,
	candidateBlock *block.Block,
	waivedAggregators []peer.ID,
	retryAttempt uint32,
	originalMinerPeerID peer.ID,
) error {
	// V2 新增：递归深度保护，防止无限递归
	const maxRetryAttempts = 10
	if retryAttempt >= maxRetryAttempts {
		s.logger.Warnf("⚠️ 重选次数超过最大限制 %d，触发回环兜底", maxRetryAttempts)
		// 超过最大重试次数，强制触发回环兜底逻辑
		localPeerID := s.p2pService.Host().ID()
		if originalMinerPeerID != "" && originalMinerPeerID == localPeerID {
			s.logger.Infof("🔄 回环兜底：原始矿工自己作为聚合器处理")
			// 将候选区块添加到候选池，然后执行聚合流程
			fromPeerStr := localPeerID.String()
			blockHash, err := s.candidatePool.AddCandidate(candidateBlock, fromPeerStr)
			if err != nil {
				return fmt.Errorf("failed to add candidate to pool: %v", err)
			}
			s.logger.Infof("候选区块已添加到候选池（回环兜底-超时保护），哈希前缀: %s", hex.EncodeToString(blockHash)[:8])
			return s.executeAggregationFlow(ctx, candidateBlock.Header.Height)
		}
		return fmt.Errorf("exceeded max retry attempts (%d) and fallback failed", maxRetryAttempts)
	}

	// 检查候选区块是否为 nil
	if candidateBlock == nil {
		return fmt.Errorf("候选区块不能为空")
	}

	// 检查区块头是否为 nil
	if candidateBlock.Header == nil {
		return fmt.Errorf("候选区块头不能为空")
	}

	height := candidateBlock.Header.Height

	// V2 新增：获取该高度的正确聚合节点（排除弃权节点）
	var targetAggregator peer.ID
	var err error
	localPeerID2 := s.p2pService.Host().ID()

	if len(waivedAggregators) > 0 {
		// V2 优化：记录显式回环检测条件
		s.logger.Debugf("🔍 回环检测：弃权节点数=%d, 重试次数=%d", len(waivedAggregators), retryAttempt)

		// 使用带弃权过滤的选举
		targetAggregator, err = s.election.GetAggregatorForHeightWithWaivers(height, waivedAggregators)
		if err != nil {
			// V2 优化：显式回环触发条件记录
			s.logger.Warnf("⚠️ 回环触发条件满足 - 原因: 选举失败(%v), 弃权节点数=%d", err, len(waivedAggregators))

			// 如果所有候选都弃权，检查是否回到原始矿工
			if originalMinerPeerID != "" && originalMinerPeerID == localPeerID2 {
				// 回环到原始矿工，由原始矿工作为聚合器处理
				s.logger.Infof("🔄 回环兜底：所有候选都弃权，由原始矿工 %s 作为聚合器处理", localPeerID2)
				// 直接进入聚合流程（跳过转发）
				fromPeer := ""
				if hint, ok := chainsync.PeerHintFromContext(ctx); ok && hint != "" {
					fromPeer = hint.String()
				}
				blockHash, err := s.candidatePool.AddCandidate(candidateBlock, fromPeer)
				if err != nil {
					return fmt.Errorf("failed to add candidate to pool: %v", err)
				}
				s.logger.Infof("候选区块已添加到候选池（回环兜底-选举失败），哈希前缀: %s", hex.EncodeToString(blockHash)[:8])
				return s.executeAggregationFlow(ctx, height)
			}
			return fmt.Errorf("failed to get aggregator for height %d with waivers: %v", height, err)
		}
		s.logger.Infof("🔄 重选聚合器（排除 %d 个弃权节点）: %s", len(waivedAggregators), targetAggregator)
	} else {
		// 首次提交，使用标准选举
		targetAggregator, err = s.election.GetAggregatorForHeight(height)
		if err != nil {
			return fmt.Errorf("failed to get aggregator for height %d: %v", height, err)
		}
		s.logger.Debugf("首次选举聚合器: %s", targetAggregator)
	}

	// 🔒 严格安全检查：验证目标聚合器是否支持区块提交协议
	supported := true

	// 🆕 2025-12-18 修复：检查目标聚合器是否是本地节点
	// 如果是本地节点，跳过 peerstore 协议检查（因为 peerstore 不存储本地节点的协议信息）
	localPeerID := s.p2pService.Host().ID()
	isLocalNode := targetAggregator == localPeerID

	if isLocalNode {
		// 本地节点肯定支持自己注册的协议，直接跳过检查
		s.logger.Debugf("✅ 目标聚合器是本地节点，跳过协议检查")
		supported = true
	} else if rm, ok := s.routingTableManager.(*kbucketimpl.RoutingTableManager); ok {
		// 🆕 2025-12-19 优化：使用增强的协议检查，支持多版本协议变体匹配
		// 1. 首先快速检查 peerstore 缓存
		supported, err = rm.SupportsProtocol(targetAggregator, protocols.ProtocolBlockSubmission)

		// 2. 如果快速检查失败且是首次重试，尝试带刷新的检查
		if err == nil && !supported && retryAttempt == 0 {
			s.logger.Debugf("🔄 协议快速检查失败，尝试带刷新的协议检查: peer=%s", targetAggregator.String()[:12])
			supported, err = rm.SupportsProtocolWithRefresh(ctx, targetAggregator, protocols.ProtocolBlockSubmission)
		}

		// 3. 🆕 额外检查：确认是否是 WES 节点
		if err == nil && !supported {
			isWESNode := rm.IsWESNode(targetAggregator)
			if !isWESNode {
				s.logger.Debugf("📋 节点 %s 不是 WES 节点（不支持任何 WES 核心协议）", targetAggregator.String()[:12])
			}
		}
	} else {
		// 防御：无 kbucketimpl 时不做协议探测（避免引入不确定性），由下游 Call() 失败反馈健康分。
		supported = true
	}

	// ❌ 协议检查失败 - 记录失败而非立即删除（可能是暂时网络问题）
	if err != nil {
		s.logger.Warnf("🚫 协议检查出错，记录节点 %s 失败: %v", targetAggregator.String()[:12], err)

		// 记录失败到健康系统（可能导致Suspect->Quarantined）
		if s.routingTableManager != nil {
			s.routingTableManager.RecordPeerFailure(targetAggregator)
		}

		// 🆕 2025-12-19 优化：协议检查出错时也尝试重选，而不是直接返回错误
		const maxProtocolRetries = 3
		if retryAttempt < maxProtocolRetries {
			newWaivers := append(waivedAggregators, targetAggregator)
			s.logger.Infof("🔄 协议检查出错，自动重选聚合器（排除 %s），重试次数: %d/%d",
				targetAggregator.String()[:12], retryAttempt+1, maxProtocolRetries)
			return s.forwardBlockToCorrectAggregator(ctx, candidateBlock, newWaivers, retryAttempt+1, originalMinerPeerID)
		}

		return fmt.Errorf("protocol check failed for aggregator %s: %v - peer marked as suspect", targetAggregator, err)
	}

	// ⚠️ 节点不支持协议 - 这是明确的不兼容外部节点（可能是非 WES libp2p 节点）
	//
	// 🆕 2025-12-19 优化：
	// - 使用 DEBUG 级别日志（这是已知且已处理的情况，不是真正的错误）
	// - 使用 QuarantineWithAnalysis 进行智能隔离，获取详细的节点类型分析
	// - 隔离期内该节点不会被选为聚合器/同步上游
	// - 自动重选聚合器（将不兼容节点加入弃权列表，递归重试）
	if !supported {
		// 🆕 使用增强的节点类型分析进行隔离
		var peerTypeInfo string
		if rm, ok := s.routingTableManager.(*kbucketimpl.RoutingTableManager); ok {
			compatInfo := rm.QuarantineWithAnalysis(targetAggregator, protocols.ProtocolBlockSubmission)
			peerTypeInfo = fmt.Sprintf("type=%s", compatInfo.Type)
			if compatInfo.IncompatibleReason != "" {
				peerTypeInfo += fmt.Sprintf(", reason=%s", compatInfo.IncompatibleReason)
			}
		} else if s.routingTableManager != nil {
			// 回退到简单隔离
			s.routingTableManager.QuarantineIncompatiblePeer(targetAggregator, "protocol_not_supported:"+protocols.ProtocolBlockSubmission)
			peerTypeInfo = "type=unknown (fallback)"
		}

		s.logger.Debugf("🚫 节点 %s 不支持协议 %s：判定为不兼容节点（%s），将自动重选聚合器",
			targetAggregator.String()[:12], protocols.ProtocolBlockSubmission, peerTypeInfo)

		// 🆕 自动重选聚合器：将不兼容节点加入弃权列表，递归重试
		// 限制最大重试次数，避免无限循环
		const maxProtocolRetries = 3
		if retryAttempt < maxProtocolRetries {
			newWaivers := append(waivedAggregators, targetAggregator)
			s.logger.Infof("🔄 自动重选聚合器（排除不兼容节点 %s [%s]），重试次数: %d/%d",
				targetAggregator.String()[:12], peerTypeInfo, retryAttempt+1, maxProtocolRetries)
			return s.forwardBlockToCorrectAggregator(ctx, candidateBlock, newWaivers, retryAttempt+1, originalMinerPeerID)
		}

		// 达到最大重试次数，使用本地处理作为兜底
		s.logger.Warnf("⚠️ 自动重选聚合器已达最大重试次数(%d)，将尝试本地处理", maxProtocolRetries)
		return fmt.Errorf("incompatible peer %s does not support protocol %s (max retries exceeded)", targetAggregator, protocols.ProtocolBlockSubmission)
	}

	// ✅ 协议检查通过 - 记录成功
	s.logger.Debugf("✅ 已验证聚合器 %s 支持协议: %s", targetAggregator, protocols.ProtocolBlockSubmission)
	if s.routingTableManager != nil && !isLocalNode {
		s.routingTableManager.RecordPeerSuccess(targetAggregator)
	}

	// 🆕 2025-12-18 修复：如果目标聚合器是本地节点（所有远程节点都弃权后回环），
	// 直接执行本地聚合流程，而不是通过网络调用自己
	if isLocalNode {
		s.logger.Infof("🔄 所有远程节点弃权后回环到本地节点，直接执行本地聚合流程")
		// 将候选区块添加到候选池
		height := candidateBlock.Header.Height
		fromPeerStr := localPeerID.String()
		blockHash, err := s.candidatePool.AddCandidate(candidateBlock, fromPeerStr)
		if err != nil {
			s.logger.Warnf("添加候选区块到候选池失败（回环兜底）: %v", err)
		} else {
			s.logger.Infof("候选区块已添加到候选池（回环兜底），哈希前缀: %s", hex.EncodeToString(blockHash)[:8])
		}
		// 直接执行聚合流程
		return s.executeAggregationFlow(ctx, height)
	}

	// V2 新增：构建 MinerBlockSubmission 消息（包含弃权信息）
	localPeerIDForMsg := s.p2pService.Host().ID()
	waivedAggregatorsBytes := make([][]byte, len(waivedAggregators))
	for i, waived := range waivedAggregators {
		waivedAggregatorsBytes[i] = []byte(waived)
	}

	// 确定原始矿工节点ID
	originalMinerBytes := []byte(localPeerIDForMsg)
	if originalMinerPeerID != "" {
		originalMinerBytes = []byte(originalMinerPeerID)
	} else {
		// 如果未指定，假设当前节点是原始矿工
		originalMinerBytes = []byte(localPeerIDForMsg)
	}

	submission := &protocol.MinerBlockSubmission{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(localPeerIDForMsg),
			TimestampUnix: time.Now().Unix(),
		},
		CandidateBlock:      candidateBlock,
		MinerPeerId:         []byte(localPeerIDForMsg),
		MiningDifficulty:    candidateBlock.Header.Difficulty,
		ParentHash:          candidateBlock.Header.PreviousHash,
		RelayHopLimit:       1,
		WaivedAggregators:   waivedAggregatorsBytes, // V2 新增
		RetryAttempt:        retryAttempt + 1,       // V2 新增：重试次数+1
		OriginalMinerPeerId: originalMinerBytes,     // V2 新增
	}

	// 序列化消息
	reqBytes, err := proto.Marshal(submission)
	if err != nil {
		// 🔍 序列化失败调试信息
		s.logger.Errorf("🚫 MinerBlockSubmission序列化失败 - height=%d, error=%v", height, err)
		return fmt.Errorf("failed to serialize submission: %v", err)
	}

	// 🔍 序列化成功调试信息
	s.logger.Debugf("✅ MinerBlockSubmission序列化成功 - height=%d, size=%d, target=%s", height, len(reqBytes), targetAggregator)

	// 发送给正确的聚合节点
	respBytes, err := s.networkService.Call(ctx, targetAggregator, protocols.ProtocolBlockSubmission, reqBytes, nil)
	if err != nil {
		// 网络调用失败 - 记录失败到健康系统
		if s.routingTableManager != nil {
			s.routingTableManager.RecordPeerFailure(targetAggregator)
		}
		s.logger.Errorf("🚫 转发区块失败，已记录节点 %s 健康分下降", targetAggregator)
		return fmt.Errorf("network call failed to %s: %v", targetAggregator, err)
	}

	// V2 新增：解析聚合器响应，检查弃权标志
	var acceptance protocol.AggregatorBlockAcceptance
	if err = proto.Unmarshal(respBytes, &acceptance); err != nil {
		s.logger.Errorf("🚫 解析聚合器响应失败 - target=%s, error=%v", targetAggregator, err)
		return fmt.Errorf("failed to parse aggregator acceptance from %s: %v", targetAggregator, err)
	}

	// 检查弃权标志
	if acceptance.Waived {
		s.logger.Infof("⚠️ 聚合器 %s 弃权 - reason=%s, local_height=%d",
			targetAggregator, acceptance.WaiverReason.String(), acceptance.LocalHeight)

		// 将弃权节点添加到列表
		newWaivedAggregators := append(waivedAggregators, targetAggregator)

		// 记录弃权，但不标记为失败（弃权是正常行为）
		s.logger.Infof("🔄 触发重选，已弃权节点数: %d", len(newWaivedAggregators))

		// 递归调用，重选下一个聚合器（不需要 fromPeer 参数）
		return s.forwardBlockToCorrectAggregator(
			ctx,
			candidateBlock,
			newWaivedAggregators,
			retryAttempt+1,
			originalMinerPeerID,
		)
	}

	// 接受成功 - 记录成功到健康系统
	if s.routingTableManager != nil {
		s.routingTableManager.RecordPeerSuccess(targetAggregator)
	}
	s.logger.Infof("✅ 聚合器接受区块 - target=%s, height=%d", targetAggregator, height)
	return nil
}

// executeAggregationFlow 执行聚合流程（距离选择）
//
// 🆕 2025-12-18 优化：
// - 添加互斥锁防止并发聚合流程
// - 避免状态机竞态导致的非法状态转换
func (s *aggregationStarter) executeAggregationFlow(ctx context.Context, height uint64) (retErr error) {
	// 🆕 获取聚合流程锁，防止并发执行
	s.aggregationFlowMu.Lock()
	defer s.aggregationFlowMu.Unlock()

	// ✅ 事务式状态机：任何错误都必须经由 Error -> Idle 回到可继续工作的状态，避免卡死在中间态。
	defer func() {
		if retErr != nil {
			if s.logger != nil {
				s.logger.Errorf("❌ 聚合流程失败（将进入 Error→Idle 自愈）: height=%d err=%v", height, retErr)
			}
			// 先尽力进入 Error（合法转换：Listening/Collecting/Evaluating/Selecting/Distributing/Paused -> Error）
			cur := s.stateManager.GetCurrentState()
			if cur != types.AggregationStateIdle && cur != types.AggregationStateError {
				if err := s.stateManager.TransitionTo(types.AggregationStateError); err != nil {
					if s.logger != nil {
						s.logger.Errorf("❌ 聚合失败后的状态修复：无法进入 Error: current=%s err=%v", cur.String(), err)
					}
				}
			}
		}

		// 最终必须回到 Idle（若无法回到 Idle，则下一轮会持续失败/刷屏）
		if err := s.ensureAggregatorStateIsIdle(); err != nil {
			if s.logger != nil {
				s.logger.Errorf("❌ 聚合失败后的状态修复：无法回到 Idle: %v", err)
			}
		}
	}()

	// 1. 检查并修复聚合器状态
	if err := s.ensureAggregatorStateIsIdle(); err != nil {
		return fmt.Errorf("无法确保聚合器状态为空闲: %v", err)
	}

	// 2. 状态转换：Listening
	if err := s.stateManager.TransitionTo(types.AggregationStateListening); err != nil {
		return err
	}
	if err := s.stateManager.SetCurrentHeight(height); err != nil {
		return err
	}

	// 3. 状态转换：Collecting - 启动固定收集窗口
	//
	// 🎯 **固定收集窗口策略**：
	// - 从接收第一个候选区块开始，启动固定时间窗口
	// - 窗口期间收集所有到达的候选区块
	// - 窗口结束后立即进行选择，不等待更多候选
	// - 目标：给足够时间让各矿工的候选区块到达聚合器
	if err := s.stateManager.TransitionTo(types.AggregationStateCollecting); err != nil {
		return err
	}

	// 固定收集窗口时间 - 从配置中获取
	collectionDuration := s.config.Aggregator.CollectionWindowDuration

	err := s.candidateCollector.StartCollectionWindow(height, collectionDuration)
	if err != nil {
		return err
	}

	// V2 新增：记录收集窗口结束时间（用于状态查询）
	collectionWindowEndTime := uint64(time.Now().Add(collectionDuration).Unix())
	s.collectionWindowMu.Lock()
	s.collectionWindowEndTime[height] = collectionWindowEndTime
	s.collectionWindowMu.Unlock()

	s.logger.Infof("🕐 固定收集窗口已启动：%v，高度: %d, 结束时间: %d", collectionDuration, height, collectionWindowEndTime)

	// 4. 等待收集窗口结束并获取所有候选区块
	candidates, err := s.candidateCollector.CloseCollectionWindow(height)
	if err != nil {
		return err
	}

	s.logger.Infof("✅ 收集窗口结束，共收集到 %d 个候选区块", len(candidates))

	// 5. 状态转换：Evaluating - XOR距离计算
	if err := s.stateManager.TransitionTo(types.AggregationStateEvaluating); err != nil {
		return err
	}

	// 获取父区块哈希作为距离计算基准（必须来自真实链状态）
	parentBlockHash, err := s.getParentBlockHash(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to get parent block hash: %v", err)
	}

	// 计算所有候选区块的XOR距离
	distanceResults, err := s.distanceSelector.CalculateDistances(ctx, candidates, parentBlockHash)
	if err != nil {
		return fmt.Errorf("failed to calculate distances: %v", err)
	}

	s.logger.Info("候选区块距离计算完成")

	// 6. 状态转换：Selecting - 选择距离最近的区块
	if err := s.stateManager.TransitionTo(types.AggregationStateSelecting); err != nil {
		return err
	}

	selected, err := s.distanceSelector.SelectClosestBlock(ctx, distanceResults)
	if err != nil {
		return fmt.Errorf("failed to select closest block: %v", err)
	}

	s.logger.Info("最优区块选择完成")

	// 7. 生成距离选择证明（给全网其他节点验证用）
	distanceProof, err := s.distanceSelector.GenerateDistanceProof(ctx, selected, distanceResults, parentBlockHash)
	if err != nil {
		return fmt.Errorf("failed to generate distance proof: %v", err)
	}

	s.logger.Info("距离选择证明生成完成")

	// 8. 状态转换：Distributing - 立即分发结果
	//
	// 🎯 **固定分发时机策略**：
	// - 收集窗口结束后立即选择最优区块并分发
	// - 不基于区块时间戳进行任何等待
	// - 不考虑最小区块间隔（由矿工侧难度调整控制）
	// - 目标：确保网络及时获得聚合结果，保持链的活跃性
	if err := s.stateManager.TransitionTo(types.AggregationStateDistributing); err != nil {
		return err
	}

	// 计算真实的候选数量
	totalCandidates := uint32(len(distanceResults))

	// 立即分发选择结果，使用距离选择证明
	err = s.resultDistributor.DistributeSelectedBlock(ctx, selected, distanceProof, totalCandidates)
	if err != nil {
		return fmt.Errorf("failed to distribute selected block: %v", err)
	}

	s.logger.Info("结果分发完成")

	// 🎯 **修复：聚合器选择区块后立即本地处理**
	// 问题：HandleConsensusResultBroadcast 会跳过自己发送的消息，导致区块没有被处理
	// 解决：在分发到网络的同时，立即调用 ProcessBlock 处理选中的区块
	if s.blockProcessor != nil {
		selectedHeight := uint64(0)
		if selected != nil && selected.Block != nil && selected.Block.Header != nil {
			selectedHeight = selected.Block.Header.Height
		}
		s.logger.Infof("🔧 开始本地处理选中的区块，高度: %d", selectedHeight)

		// ✅ 生产级幂等：收集窗口/同步/乱序重放 可能导致链尖在本轮结束前已推进，
		// 此时再次写入同高度区块会被 DataWriter 拒绝（它只接受严格有序写入）。
		// 这类情况不应当视为错误，而应当直接跳过，交给后续同步收敛。
		if s.chainQuery != nil && selectedHeight > 0 {
			if ci, err := s.chainQuery.GetChainInfo(ctx); err == nil && ci != nil {
				if ci.Height >= selectedHeight {
					s.logger.Infof("⏩ 本地链尖已达到/超过该高度，跳过重复写入: local_height=%d selected_height=%d",
						ci.Height, selectedHeight)
					// 注意：这里不返回错误；保持状态机可正常回到 Idle
					goto afterLocalProcess
				}
			}
		}

		// 全局写门闸：只读/写围栏下必须停止写入（返回弃权错误）
		if err := writegate.Default().AssertWriteAllowed(ctx, "aggregator.processSelectedBlock"); err != nil {
			// 确保处于 Idle 状态（幂等操作）
			if transErr := s.stateManager.EnsureIdle(); transErr != nil {
				s.logger.Warnf("只读模式下无法确保Idle状态: %v", transErr)
				// 转换失败不影响弃权流程，继续返回弃权错误
			}

			// 获取当前链高度
			localHeight := uint64(0)
			if s.chainQuery != nil {
				if ci, err := s.chainQuery.GetChainInfo(ctx); err == nil && ci != nil {
					localHeight = ci.Height
				}
			}

			// 记录只读模式弃权指标
			recordWaiver("read_only_mode")

			// 返回弃权错误
			return &types.WaiverError{
				Reason:      types.WaiverReasonReadOnlyMode,
				LocalHeight: localHeight,
				Height:      selectedHeight,
			}
		}
		if err := s.blockProcessor.ProcessBlock(ctx, selected.Block); err != nil {
			s.logger.Errorf("❌ 本地处理选中区块失败: %v", err)
			// 注意：即使本地处理失败，也不阻止状态转换，因为区块已经分发到网络
			// 其他节点可能会成功处理，本地可以在后续同步中修复
		} else {
			s.logger.Infof("✅ 本地处理选中区块成功，高度: %d", selectedHeight)
		}
	} else {
		s.logger.Warn("⚠️ blockProcessor 未注入，无法本地处理选中区块")
	}
afterLocalProcess:

	// 9. 状态转换：Idle - 聚合完成，回到空闲状态
	if err := s.stateManager.TransitionTo(types.AggregationStateIdle); err != nil {
		return err
	}

	s.logger.Info("聚合流程完成")
	return nil
}

// ensureAggregatorStateIsIdle 确保聚合器状态为空闲状态
func (s *aggregationStarter) ensureAggregatorStateIsIdle() error {
	currentState := s.stateManager.GetCurrentState()
	s.logger.Infof("检查聚合器状态: 当前状态=%s", currentState.String())

	// 如果已经是空闲状态，直接返回
	if currentState == types.AggregationStateIdle {
		s.logger.Info("聚合器状态已经是空闲状态，无需修复")
		return nil
	}

	// 如果状态不是空闲，记录警告并尝试修复
	s.logger.Warnf("聚合器状态不是空闲状态: %s，尝试修复", currentState.String())

	// 根据当前状态选择合适的修复策略
	switch currentState {
	case types.AggregationStateListening, types.AggregationStatePaused, types.AggregationStateDistributing, types.AggregationStateError:
		// ✅ 合法直达：Listening/Paused/Distributing/Error -> Idle
		if err := s.stateManager.TransitionTo(types.AggregationStateIdle); err != nil {
			s.logger.Errorf("无法从状态 %s 转换到Idle状态: %v", currentState.String(), err)
			return fmt.Errorf("状态转换失败: %v", err)
		}
		s.logger.Info("成功修复聚合器状态为空闲")

	case types.AggregationStateCollecting, types.AggregationStateEvaluating, types.AggregationStateSelecting:
		// ✅ 关键修复：这些中间态不允许直接 -> Idle（会触发“无效的状态转换”并导致刷屏/卡死）
		// 正确恢复路径必须是：<active> -> Error -> Idle
		if err := s.stateManager.TransitionTo(types.AggregationStateError); err != nil {
			s.logger.Errorf("无法从状态 %s 转换到 Error 状态: %v", currentState.String(), err)
			return fmt.Errorf("状态转换失败: %v", err)
		}
		if err := s.stateManager.TransitionTo(types.AggregationStateIdle); err != nil {
			s.logger.Errorf("无法从 Error 状态转换到 Idle 状态: %v", err)
			return fmt.Errorf("状态转换失败: %v", err)
		}
		s.logger.Infof("成功从中间态 %s 修复为 Idle（经由 Error）", currentState.String())

	default:
		s.logger.Errorf("未知的聚合器状态: %s", currentState.String())
		return fmt.Errorf("未知的聚合器状态: %s", currentState.String())
	}

	return nil
}

// generateMessageID 生成唯一消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), "aggregator")
}

// startAggregatorService 启动聚合器服务
func (s *aggregationStarter) startAggregatorService(ctx context.Context) error {
	s.logger.Info("启动聚合器服务")

	// 检查当前状态
	currentState := s.stateManager.GetCurrentState()
	if currentState != types.AggregationStateIdle {
		return errors.New("聚合器服务已在运行或处于异常状态")
	}

	// 保持在空闲状态，等待聚合轮次触发
	s.logger.Info("聚合器服务已启动，等待聚合轮次")
	return nil
}
