// Package network_handler 网络协议处理器服务
//
// 🎯 **网络协议处理器服务模块**
//
// 本包实现 NetworkProtocolHandler 接口，提供网络协议处理功能：
// - 处理矿工区块提交协议
// - 处理共识心跳协议
// - 处理共识结果广播
// - 集成所有网络协议处理器
package network_handler

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	chainsync "github.com/weisyn/v1/internal/core/chain/sync"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"google.golang.org/protobuf/proto"
)

// NetworkProtocolHandlerService 网络协议处理器服务实现（薄委托层）
type NetworkProtocolHandlerService struct {
	logger            log.Logger                      // 日志记录器
	electionService   interfaces.AggregatorElection   // 选举服务
	chainQuery        persistence.QueryService        // 统一查询服务（包含 ChainQuery 和 BlockQuery）
	candidatePool     mempool.CandidatePool           // 候选池
	p2pService        p2pi.Service                    // P2P 服务
	network           netiface.Network                // 网络服务
	controllerService interfaces.AggregatorController // 控制器服务
	forkHandler       chain.ForkHandler               // 分叉处理服务
	syncService       chain.SystemSyncService         // ✅ P1修复：同步服务（用于触发同步）
	blockValidator    block.BlockValidator            // 区块验证服务
	blockProcessor    block.BlockProcessor            // 区块处理服务
	tempStore         storage.TempStore               // ✅ P1修复：临时存储服务（用于存储乱序区块）
	blockHashClient   core.BlockHashServiceClient     // ✅ P1修复：区块哈希服务客户端

	// 协议处理器
	blockSubmissionHandler *blockSubmissionHandler
	heartbeatHandler       *consensusHeartbeatHandler
	statusHandler          *aggregatorStatusHandler // V2 新增：状态查询处理器
}

// NewNetworkProtocolHandlerService 创建网络协议处理器服务实例
func NewNetworkProtocolHandlerService(
	logger log.Logger,
	electionService interfaces.AggregatorElection,
	chainQuery persistence.QueryService,
	candidatePool mempool.CandidatePool,
	p2pService p2pi.Service,
	network netiface.Network,
	controllerService interfaces.AggregatorController,
	forkHandler chain.ForkHandler,
	syncService chain.SystemSyncService, // ✅ P1修复：同步服务（可选）
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	tempStore storage.TempStore, // ✅ P1修复：临时存储服务（可选）
	blockHashClient core.BlockHashServiceClient, // ✅ P1修复：区块哈希服务客户端（可选）
	stateManager interfaces.AggregatorStateManager, // V2 新增：状态管理器
) interfaces.NetworkProtocolHandler {
	// 创建协议处理器
	blockSubmissionHandler := newBlockSubmissionHandler(
		logger,
		electionService,
		chainQuery,
		candidatePool,
		p2pService,
		network,
		controllerService,
		syncService,
	)

	heartbeatHandler := newConsensusHeartbeatHandler(
		logger,
		chainQuery,
		p2pService,
		syncService,
	)

	// V2 新增：创建状态查询处理器
	statusHandler := newAggregatorStatusHandler(
		logger,
		electionService,
		stateManager,
		chainQuery,
		p2pService,
	)

	return &NetworkProtocolHandlerService{
		logger:                 logger,
		electionService:        electionService,
		chainQuery:             chainQuery,
		candidatePool:          candidatePool,
		p2pService:             p2pService,
		network:                network,
		controllerService:      controllerService,
		forkHandler:            forkHandler,
		syncService:            syncService, // ✅ P1修复：同步服务
		blockValidator:         blockValidator,
		blockProcessor:         blockProcessor,
		tempStore:              tempStore,       // ✅ P1修复：临时存储服务
		blockHashClient:        blockHashClient, // ✅ P1修复：区块哈希服务客户端
		blockSubmissionHandler: blockSubmissionHandler,
		heartbeatHandler:       heartbeatHandler,
		statusHandler:          statusHandler, // V2 新增：状态查询处理器
	}
}

// 编译时确保 NetworkProtocolHandlerService 实现了 NetworkProtocolHandler 接口
var _ interfaces.NetworkProtocolHandler = (*NetworkProtocolHandlerService)(nil)

// HandleMinerBlockSubmission 处理矿工区块提交
func (s *NetworkProtocolHandlerService) HandleMinerBlockSubmission(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	s.logger.Debugf("处理矿工区块提交: from=%s, size=%d", from, len(reqBytes))
	return s.blockSubmissionHandler.handleMinerBlockSubmission(ctx, from, reqBytes)
}

// HandleConsensusHeartbeat 处理共识心跳协议
func (s *NetworkProtocolHandlerService) HandleConsensusHeartbeat(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	s.logger.Debugf("处理共识心跳: from=%s, size=%d", from, len(reqBytes))
	return s.heartbeatHandler.handleConsensusHeartbeat(ctx, from, reqBytes)
}

// HandleAggregatorStatusQuery 处理聚合器状态查询
//
// V2 新增：处理提交者的聚合器状态查询请求
func (s *NetworkProtocolHandlerService) HandleAggregatorStatusQuery(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	s.logger.Debugf("处理聚合器状态查询: from=%s, size=%d", from, len(reqBytes))
	if s.statusHandler == nil {
		s.logger.Warn("⚠️  状态查询处理器未初始化")
		return []byte{}, fmt.Errorf("status handler not initialized")
	}
	return s.statusHandler.handleAggregatorStatusQuery(ctx, from, reqBytes)
}

// HandleConsensusResultBroadcast 处理共识结果广播
//
// ✅ P1修复：实现智能路由，根据区块高度智能处理
func (s *NetworkProtocolHandlerService) HandleConsensusResultBroadcast(ctx context.Context, from peer.ID, topic string, data []byte) error {
	s.logger.Debugf("处理共识结果广播: from=%s, topic=%s, size=%d", from, topic, len(data))

	// 跳过自己发送的消息
	if from == s.p2pService.Host().ID() {
		if s.logger != nil {
			s.logger.Debug("跳过自己发送的共识结果广播")
		}
		return nil
	}

	// 反序列化共识结果广播消息
	var broadcast protocol.ConsensusResultBroadcast
	if err := proto.Unmarshal(data, &broadcast); err != nil {
		if s.logger != nil {
			s.logger.Errorf("反序列化共识结果广播失败: %v", err)
		}
		return fmt.Errorf("failed to unmarshal consensus result broadcast: %v", err)
	}

	// 验证消息基本格式
	if broadcast.FinalBlock == nil {
		if s.logger != nil {
			s.logger.Error("共识结果广播消息缺少最终区块")
		}
		return fmt.Errorf("consensus result broadcast missing final block")
	}

	finalBlock := broadcast.FinalBlock
	if s.logger != nil && finalBlock.Header != nil {
		parentHash := ""
		if len(finalBlock.Header.PreviousHash) >= 8 {
			parentHash = fmt.Sprintf("%x", finalBlock.Header.PreviousHash[:8])
		} else {
			parentHash = fmt.Sprintf("%x", finalBlock.Header.PreviousHash)
		}
		fromStr := from.String()
		if len(fromStr) > 8 {
			fromStr = fromStr[:8]
		}
		s.logger.Infof("🔔 收到最终区块广播: height=%d, parent=%s, txs=%d, from=%s",
			finalBlock.Header.Height, parentHash, len(finalBlock.GetBody().GetTransactions()), fromStr)

	}

	// ✅ P1修复：智能路由 - 根据区块高度决定处理策略
	ctxWithHint := chainsync.ContextWithPeerHint(ctx, from)
	if err := s.routeBlockByHeight(ctxWithHint, finalBlock); err != nil {
		if s.logger != nil {
			s.logger.Errorf("区块路由处理失败: %v", err)
		}
		return fmt.Errorf("block routing failed: %v", err)
	}

	if s.logger != nil {
		s.logger.Info("网络处理器成功处理共识结果广播")
	}
	return nil
}

// routeBlockByHeight 根据区块高度智能路由处理
//
// 🎯 **破坏性重构：职责边界明确化**
// 1. height == currentHeight + 1 → 正常处理流程（验证+处理）
// 2. height > currentHeight + 1 → 仅触发同步服务，不存储区块（补齐逻辑集中在 sync 模块）
// 3. height <= currentHeight → 检测分叉或重复
//
// ⚠️ **重要原则**：
// - 共识广播只负责"告诉别人：我这里刚出了高度 H"
// - 严格只处理"正常后继块"（height == currentHeight + 1）
// - 对"高度跳跃"只做"触发 SyncService.TriggerSync"的作用
// - 补齐中间缺失块逻辑全部集中在 sync 模块
//
// 参数：
//   - ctx: 上下文
//   - block: 待处理的区块
//
// 返回：
//   - error: 路由错误
func (s *NetworkProtocolHandlerService) routeBlockByHeight(ctx context.Context, block *core.Block) error {
	// 1. 获取当前链高度
	chainInfo, err := s.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	currentHeight := chainInfo.Height
	blockHeight := block.Header.Height

	// 🔧 容错：检测链尖高度异常
	// 如果 currentHeight == 0 且收到的区块高度 > 1，说明链尖数据可能损坏
	if currentHeight == 0 && blockHeight > 1 {
		if s.logger != nil {
			// 说明：
			// - 首次启动/仅完成创世时，本地高度为0是正常状态；
			// - 如果此时已连接到网络，上游广播的高度通常会远大于1，属于“同步未完成”的常见场景；
			// - 只有在“历史上已经同步过/应当 >0”但仍长期为0时，才更可能是链尖元数据损坏。
			s.logger.Warnf("⚠️ 收到高高度区块但本地链尖仍为0: local_tip=%d, received=%d（可能处于首次启动/同步未完成；若已同步过仍为0再考虑链尖修复）",
				currentHeight, blockHeight)
			s.logger.Infof("💡 将触发同步机制尝试补齐缺失区块，目标高度: %d", blockHeight)
		}
		// 继续正常流程，让同步机制处理
		// 注意：这不是错误，只是警告，因为可能是节点刚启动或数据损坏后的恢复阶段
	}

	// 2. 情况1：正常后继区块（height == currentHeight + 1）
	// 🎯 这是共识广播的唯一正常处理路径
	if blockHeight == currentHeight+1 {
		if s.blockValidator == nil {
			return fmt.Errorf("block validator 未初始化（聚合模式需要启用 block 模块）")
		}
		if s.blockProcessor == nil {
			return fmt.Errorf("block processor 未初始化（聚合模式需要启用 block 模块）")
		}

		// 验证区块
		valid, err := s.blockValidator.ValidateBlock(ctx, block)
		if err != nil {
			return fmt.Errorf("区块验证失败: %w", err)
		}
		if !valid {
			return fmt.Errorf("区块验证未通过")
		}

		// 处理区块
		if err := s.blockProcessor.ProcessBlock(ctx, block); err != nil {
			return fmt.Errorf("区块处理失败: %w", err)
		}

		if s.logger != nil {
			s.logger.Infof("✅ [共识广播] 正常处理区块: height=%d", blockHeight)
		}
		return nil
	}

	// 3. 情况2：高度跳跃（height > currentHeight + 1）
	// 🎯 破坏性重构：仅触发同步，不存储区块，补齐逻辑集中在 sync 模块
	if blockHeight > currentHeight+1 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ [共识广播] 检测到区块高度跳跃: current=%d, received=%d，触发同步服务补齐缺失区块",
				currentHeight, blockHeight)
		}

		// 🎯 仅触发同步服务，不存储区块
		// 补齐中间缺失块的逻辑全部集中在 sync 模块的 block_sync.go / range_paginated 实现里
		// 🎯 语义说明：TriggerSync 在无上游节点时会返回 nil（视为无事可做），只有真正的同步失败才会返回 error
		if s.syncService != nil {
			// 异步触发同步，避免阻塞共识广播处理
			go func() {
				if err := s.syncService.TriggerSync(context.Background()); err != nil {
					if s.logger != nil {
						s.logger.Warnf("⏩ [共识广播] 触发同步失败（真正的同步错误）: %v", err)
					}
				} else {
					// err == nil 可能表示同步完成或当前无上游节点（无事可做）
					if s.logger != nil {
						s.logger.Infof("✅ [共识广播] 同步流程已执行，尝试补齐缺失区块: %d → %d（可能已完成同步，或当前无上游节点）", currentHeight+1, blockHeight-1)
					}
				}
			}()
		} else if s.logger != nil {
			s.logger.Errorf("❌ [共识广播] 无法触发同步：syncService 未注入（current=%d, received=%d）", currentHeight, blockHeight)
		}

		// ✅ 订阅式广播（PubSub）没有“重试语义”，返回 error 只会污染日志；
		// 触发同步即视为已处理该广播事件。
		return nil
	}

	// 4. 情况3：高度低于或等于当前（height <= currentHeight）
	// 可能是分叉或重复区块
	if blockHeight <= currentHeight {
		// 4.1 特殊情况：高度相等（可能是重复区块）
		// 🔧 修复：对于 blockHeight == currentHeight 的情况，优先检查重复区块
		if blockHeight == currentHeight {
			// 先计算区块哈希
			blockHash, err := s.calculateBlockHash(ctx, block)
			if err == nil {
				// 查询区块是否已存在
				existingBlock, err := s.chainQuery.GetBlockByHash(ctx, blockHash)
				if err == nil && existingBlock != nil {
					// ✅ 重复区块，幂等返回成功
					if s.logger != nil {
						s.logger.Debugf("✅ 重复区块（幂等）: height=%d, hash=%x", blockHeight, blockHash[:8])
					}
					return nil
				}
			}

			// 不是重复区块，可能是分叉（高度相同但哈希不同）
			if s.forkHandler != nil {
				if err := s.forkHandler.HandleFork(ctx, block); err != nil {
					if s.logger != nil {
						s.logger.Warnf("分叉处理失败（高度相等）: %v", err)
					}
					// 分叉处理失败也不报错，避免误判重复区块为错误
					// 因为 HandleFork 内部会判断是否真的是分叉
					if s.logger != nil {
						s.logger.Warnf("⚠️ 高度相等的区块分叉处理失败，可能是旧区块: current=%d, received=%d",
							currentHeight, blockHeight)
					}
					return nil // 幂等返回成功，避免误报
				}
				if s.logger != nil {
					s.logger.Infof("✅ 分叉处理完成（高度相等）: height=%d", blockHeight)
				}
				return nil
			}

			// 无分叉处理器，但高度相等，很可能是重复区块
			// 容错处理：幂等返回成功，避免误报 "block height too low"
			if s.logger != nil {
				s.logger.Warnf("⚠️ 高度相等但无法确认是否重复（无forkHandler）: current=%d, received=%d",
					currentHeight, blockHeight)
			}
			return nil // 幂等返回成功
		}

		// 4.2 高度低于当前（blockHeight < currentHeight）
		// 可能是旧区块或分叉
		if s.forkHandler != nil {
			if err := s.forkHandler.HandleFork(ctx, block); err != nil {
				if s.logger != nil {
					s.logger.Warnf("分叉处理失败: %v", err)
				}
				return fmt.Errorf("分叉处理失败: %w", err)
			}
			if s.logger != nil {
				s.logger.Infof("✅ 分叉处理完成: height=%d", blockHeight)
			}
			return nil
		}

		// 4.3 没有分叉处理器，检查是否为重复区块
		blockHash, err := s.calculateBlockHash(ctx, block)
		if err == nil {
			existingBlock, err := s.chainQuery.GetBlockByHash(ctx, blockHash)
			if err == nil && existingBlock != nil {
				// 区块已存在，幂等返回成功
				if s.logger != nil {
					s.logger.Debugf("✅ 旧区块已存在，幂等返回: height=%d, hash=%x", blockHeight, blockHash[:8])
				}
				return nil
			}
		}

		// 4.4 未知情况，记录警告并返回错误
		if s.logger != nil {
			s.logger.Warnf("⚠️ 接收到低高度区块: current=%d, received=%d，可能是旧区块或无效区块",
				currentHeight, blockHeight)
		}
		return fmt.Errorf("block height too low: current=%d, received=%d", currentHeight, blockHeight)
	}

	return nil
}

// calculateBlockHash 计算区块哈希（辅助方法）
func (s *NetworkProtocolHandlerService) calculateBlockHash(ctx context.Context, block *core.Block) ([]byte, error) {
	if block.Header == nil {
		return nil, fmt.Errorf("区块头为空")
	}

	// ✅ P1修复：使用 BlockHashServiceClient 计算区块哈希
	if s.blockHashClient != nil {
		req := &core.ComputeBlockHashRequest{
			Block: block,
		}
		resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("计算区块哈希失败: %w", err)
		}
		if !resp.IsValid {
			return nil, fmt.Errorf("区块结构无效")
		}
		return resp.Hash, nil
	}

	// 区块哈希是共识/分叉判定的根基：禁止简化回退。
	return nil, fmt.Errorf("BlockHashServiceClient 未初始化：拒绝计算区块哈希（height=%d）", block.Header.Height)
}
