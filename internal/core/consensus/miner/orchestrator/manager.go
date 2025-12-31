// Package orchestrator 实现挖矿编排器服务
//
// 🎯 **编排器服务模块**
//
// 本包实现 MiningOrchestrator 接口，提供挖矿流程的编排和控制：
// - 协调整个挖矿流程的执行
// - 管理候选区块创建和PoW计算
// - 处理区块发送和确认等待
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	blockInternalIf "github.com/weisyn/v1/internal/core/block/interfaces"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// MiningOrchestratorService 挖矿编排器服务实现
type MiningOrchestratorService struct {
	logger               log.Logger                           // 日志记录器
	cacheStore           storage.MemoryStore                  // 内存缓存服务（已废弃，保留兼容性）
	blockBuilder         blockInternalIf.InternalBlockBuilder // 区块构建服务（内部接口）
	blockProcessor       block.BlockProcessor                 // 区块处理服务（用于单节点模式）
	chainQuery           persistence.ChainQuery               // 链查询服务（用于状态查询）
	queryService         persistence.QueryService             // 统一查询服务（用于读取父块时间戳/MTP窗口）
	syncService          chain.SystemSyncService              // 同步服务（触发区块同步）
	powHandlerService    interfaces.PoWComputeHandler         // PoW计算服务
	heightGateService    interfaces.HeightGateManager         // 高度门闸服务
	stateManagerService  interfaces.MinerStateManager         // 状态管理服务
	networkService       netiface.Network                     // 网络服务（用于诊断 peers/quorum）
	aggregatorController interfaces.AggregatorController      // 聚合器控制器（用于区块提交）
	incentiveCollector   interfaces.IncentiveCollector        // 🔥 激励收集器（用于设置矿工地址）
	minerConfig          *consensusconfig.MinerConfig         // Miner配置（用于超时和间隔设置）
	consensusOptions     *consensusconfig.ConsensusOptions    // 共识配置（用于判断共识模式）
	compliancePolicy     complianceIfaces.Policy              // 合规策略服务（可选）
	configProvider       config.Provider                      // 配置提供者（用于读取 min_block_interval 等链参数）
	quorumChecker        quorum.Checker                       // v2：挖矿稳定性门闸检查器（网络法定人数+高度一致性+链尖前置）

	// v2：确认等待非阻塞化（防止确认门闸卡住导致“全链停摆”）
	confirmMu      sync.Mutex
	confirmWatches map[uint64]*confirmationWatch
}

// NewMiningOrchestratorService 创建挖矿编排器服务实例
func NewMiningOrchestratorService(
	logger log.Logger,
	blockService blockInternalIf.InternalBlockBuilder, // 🔧 使用内部接口以访问缓存方法
	blockProcessor block.BlockProcessor, // 区块处理服务（用于单节点模式）
	chainQuery persistence.ChainQuery,
	queryService persistence.QueryService,
	cacheStore storage.MemoryStore,
	powHandlerService interfaces.PoWComputeHandler,
	heightGateService interfaces.HeightGateManager,
	stateManagerService interfaces.MinerStateManager,
	syncService chain.SystemSyncService,
	networkService netiface.Network,
	aggregatorController interfaces.AggregatorController, // 聚合器控制器接口
	incentiveCollector interfaces.IncentiveCollector, // 🔥 激励收集器（用于设置矿工地址）
	minerConfig *consensusconfig.MinerConfig,
	consensusOptions *consensusconfig.ConsensusOptions, // 共识配置（用于判断共识模式）
	compliancePolicy complianceIfaces.Policy, // 合规策略服务（可选）
	configProvider config.Provider,
	quorumChecker quorum.Checker,
) interfaces.MiningOrchestrator {
	return &MiningOrchestratorService{
		logger:               logger,
		cacheStore:           cacheStore,
		blockBuilder:         blockService,
		blockProcessor:       blockProcessor,
		chainQuery:           chainQuery,
		queryService:         queryService,
		syncService:          syncService,
		powHandlerService:    powHandlerService,
		heightGateService:    heightGateService,
		stateManagerService:  stateManagerService,
		networkService:       networkService,
		aggregatorController: aggregatorController, // 聚合器控制器接口
		incentiveCollector:   incentiveCollector,   // 🔥 激励收集器
		minerConfig:          minerConfig,
		consensusOptions:     consensusOptions, // 共识配置
		compliancePolicy:     compliancePolicy, // 合规策略服务
		configProvider:       configProvider,
		quorumChecker:        quorumChecker,
		confirmWatches:       make(map[uint64]*confirmationWatch),
	}
}

// 编译时确保 MiningOrchestratorService 实现了 MiningOrchestrator 接口
var _ interfaces.MiningOrchestrator = (*MiningOrchestratorService)(nil)

// ExecuteMiningRound 执行一轮挖矿
// 实现薄封装原则：仅进行接口方法委托，具体业务逻辑在 execute_mining_round.go 中实现
func (s *MiningOrchestratorService) ExecuteMiningRound(ctx context.Context) error {
	return s.executeMiningRound(ctx)
}

// SetMinerAddress 设置矿工地址
//
// 🎯 **运行时矿工地址设置**
//
// 在挖矿启动时由 MinerController 调用，将矿工地址传递给：
// 1. IncentiveCollector - 构建激励交易
// 2. BlockBuilder - 构建包含区块奖励的 Coinbase
//
// 参数:
//
//	minerAddr: 矿工地址（20字节）
//
// 返回:
//
//	error: 设置失败
func (s *MiningOrchestratorService) SetMinerAddress(minerAddr []byte) error {
	// 1. 设置到激励收集器（如果可用）
	if s.incentiveCollector != nil {
		if err := s.incentiveCollector.SetMinerAddress(minerAddr); err != nil {
			return fmt.Errorf("设置矿工地址到IncentiveCollector失败: %w", err)
		}
	}

	// 2. 🔧 设置到 BlockBuilder（用于构建包含区块奖励的 Coinbase）
	if s.blockBuilder != nil && minerAddr != nil && len(minerAddr) >= 8 {
		s.blockBuilder.SetMinerAddress(minerAddr)
		if s.logger != nil {
			s.logger.Infof("✅ 矿工地址已设置到 BlockBuilder: %x", minerAddr[:8])
		}
	}

	return nil
}

// ==================== 共识模式判断方法 ====================

// isDistributedConsensusMode 判断是否为分布式共识模式
//
// 🎯 **共识模式分类**：
//   - true: 分布式聚合器共识模式
//   - 多节点通过聚合器达成共识
//   - 区块需要提交给聚合器并等待网络确认
//   - 提供拜占庭容错能力
//   - false: 单节点开发模式
//   - 区块立即本地确认
//   - 无网络共识保障
//   - ⚠️ 仅用于开发/测试，禁止用于生产
//
// @return bool 是否为分布式共识模式
func (s *MiningOrchestratorService) isDistributedConsensusMode() bool {
	// ⚠️ 系统内不存在“单节点模式”：
	// 即便暂时没发现其它节点/作为网络中第一个启动的节点，也应走同一套共识逻辑（由同步/网络状态驱动）。
	if s.consensusOptions == nil {
		// 配置缺失，默认使用分布式模式（安全优先）
		if s.logger != nil {
			s.logger.Warn("共识配置缺失，默认使用分布式共识模式（安全优先）")
		}
		return true
	}
	if !s.consensusOptions.Aggregator.EnableAggregator && s.logger != nil {
		s.logger.Warn("检测到 enable_aggregator=false，但系统不支持单节点共识语义；将强制按分布式共识路径运行")
	}
	return true
}
