// Package candidate_collector 实现候选收集服务
//
// 🎯 **候选收集服务模块**
//
// 本包实现 CandidateCollector 接口，提供候选区块收集窗口管理功能：
// - 管理收集窗口的启动和关闭
// - 配置收集窗口持续时间
// - 与mempool.CandidatePool协作获取候选
// - 支持n+1高度验证
//
// 重要：不重复实现候选池，直接使用mempool.CandidatePool公共接口
package candidate_collector

import (
	"time"

	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
)

// CandidateCollectorService 候选收集服务实现（薄委托层）
type CandidateCollectorService struct {
	logger        log.Logger                  // 日志记录器
	candidatePool mempool.CandidatePool       // 候选池接口（直接使用，不重复实现）
	collectionMgr *collectionManager          // 收集管理器
	config        *consensus.ConsensusOptions // 配置选项
	// ❌ windowOptimizer *windowOptimizer - 已删除：基于错误架构的优化器
}

// NewCandidateCollectorService 创建候选收集服务实例
func NewCandidateCollectorService(
	logger log.Logger,
	candidatePool mempool.CandidatePool,
	chainService blockchain.ChainService,
	hashManager crypto.HashManager,
	host node.Host,
	powEngine crypto.POWEngine,
	config *consensus.ConsensusOptions, // 添加配置参数
) interfaces.CandidateCollector {
	// 创建候选验证器，传入配置避免硬编码
	validator := newCandidateValidator(logger, chainService, hashManager, powEngine, config)

	// 创建收集管理器
	collectionMgr := newCollectionManager(logger, candidatePool, validator)

	// ❌ 删除窗口优化器 - 基于错误架构
	// windowOptimizer := newWindowOptimizer(logger, chainService, host)

	return &CandidateCollectorService{
		logger:        logger,
		candidatePool: candidatePool,
		collectionMgr: collectionMgr,
		config:        config, // 保存配置引用
		// ❌ windowOptimizer: windowOptimizer, - 已删除
	}
}

// 编译时确保 CandidateCollectorService 实现了 CandidateCollector 接口
var _ interfaces.CandidateCollector = (*CandidateCollectorService)(nil)

// StartCollectionWindow 启动候选收集窗口
func (s *CandidateCollectorService) StartCollectionWindow(height uint64, duration time.Duration) error {
	s.logger.Info("启动候选收集窗口")

	// 如果duration为0，使用配置的默认收集超时时间
	if duration == 0 {
		duration = s.config.Aggregator.CollectionTimeout
	}

	// 委托给收集管理器
	return s.collectionMgr.startCollectionWindow(height, duration)
}

// CloseCollectionWindow 关闭收集窗口
func (s *CandidateCollectorService) CloseCollectionWindow(height uint64) ([]types.CandidateBlock, error) {
	s.logger.Info("关闭候选收集窗口")

	// 先从候选池收集候选区块
	if err := s.collectionMgr.collectCandidateFromMempool(height); err != nil {
		s.logger.Info("从候选池收集候选区块失败")
	}

	// 委托给收集管理器
	return s.collectionMgr.closeCollectionWindow(height)
}

// IsCollectionActive 检查收集窗口是否活跃
func (s *CandidateCollectorService) IsCollectionActive(height uint64) bool {
	// 委托给收集管理器
	return s.collectionMgr.isCollectionActive(height)
}

// GetCollectionProgress 获取收集进度
func (s *CandidateCollectorService) GetCollectionProgress(height uint64) (*types.CollectionProgress, error) {
	s.logger.Info("获取收集进度")

	// 委托给收集管理器
	return s.collectionMgr.getCollectionProgress(height)
}

// ClearCandidatePool 清空候选区块内存池（修复：实现正确的清理机制）
func (s *CandidateCollectorService) ClearCandidatePool() (int, error) {
	s.logger.Info("清空候选区块内存池")

	// 调用内存池的清理接口
	count, err := s.candidatePool.ClearCandidates()
	if err != nil {
		s.logger.Info("清空候选区块内存池失败")
		return 0, err
	}

	s.logger.Info("候选区块内存池清理完成")
	return count, nil
}
