// block_sync.go - 区块同步核心逻辑
// 负责执行K桶智能同步和分页补齐同步
package sync

import (
	"context"
	"fmt"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// EmptyBatchError 表示空批次的特殊错误，包含跳跃信息
type EmptyBatchError struct {
	StartHeight uint64
	EndHeight   uint64
	NextHeight  uint64
	Reason      string
}

func (e *EmptyBatchError) Error() string {
	return fmt.Sprintf("空批次跳跃: [%d, %d] -> %d (%s)",
		e.StartHeight, e.EndHeight, e.NextHeight, e.Reason)
}

// ============================================================================
//                           K桶智能同步实现
// ============================================================================

// performKBucketSmartSync 执行K桶智能同步（获取初始区块批次）
//
// 🎯 **智能同步策略**：
// 1. 发送K桶同步请求到最优节点
// 2. 接收初始区块批次数据
// 3. 验证响应的有效性和完整性
//
// 📝 **注意**：此函数不再返回"网络高度"，因为真实的网络高度应该通过
// 专门的高度查询获得，而非从同步响应的NextHeight推算。
func performKBucketSmartSync(
	ctx context.Context,
	targetPeer peer.ID,
	localHeight uint64,
	localChainInfo *types.ChainInfo,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) (initialBlocks []*core.Block, err error) {
	if logger != nil {
		logger.Debugf("📡 向节点 %s 发起K桶智能同步", targetPeer.String()[:8])
	}

	// 获取本地节点ID
	localNodeID := host.ID()

	// 获取同步配置
	blockchainConfig := configProvider.GetBlockchain()
	var maxResponseSize uint32 = 5 * 1024 * 1024 // 默认5MB
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
		maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
	}

	// 构造K桶同步请求
	request := &protocol.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("kbucket-sync-%d", time.Now().UnixNano()),
		LocalHeight:     localHeight,
		RoutingKey:      localChainInfo.BestBlockHash,
		MaxResponseSize: maxResponseSize,              // 从配置获取
		RequesterPeerId: []byte(localNodeID.String()), // 使用host接口获取真实节点ID
		TargetHeight:    nil,                          // 同步到最新高度
	}

	// 序列化请求
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化K桶同步请求失败: %w", err)
	}

	// 配置传输选项（从配置获取超时参数）
	var connectTimeout = 15 * time.Second
	var writeTimeout = 10 * time.Second
	var readTimeout = 30 * time.Second
	var maxRetries = 2
	var retryDelay = 2 * time.Second

	if blockchainConfig != nil {
		if blockchainConfig.Sync.Advanced.ConnectTimeout > 0 {
			connectTimeout = blockchainConfig.Sync.Advanced.ConnectTimeout
		}
		if blockchainConfig.Sync.Advanced.WriteTimeout > 0 {
			writeTimeout = blockchainConfig.Sync.Advanced.WriteTimeout
		}
		if blockchainConfig.Sync.Advanced.ReadTimeout > 0 {
			readTimeout = blockchainConfig.Sync.Advanced.ReadTimeout
		}
		if blockchainConfig.Sync.Advanced.MaxRetryAttempts > 0 {
			maxRetries = blockchainConfig.Sync.Advanced.MaxRetryAttempts
		}
		if blockchainConfig.Sync.Advanced.RetryDelay > 0 {
			retryDelay = blockchainConfig.Sync.Advanced.RetryDelay
		}
	}

	transportOpts := &types.TransportOptions{
		ConnectTimeout: connectTimeout,
		WriteTimeout:   writeTimeout,
		ReadTimeout:    readTimeout,
		MaxRetries:     maxRetries,
		RetryDelay:     retryDelay,
		BackoffFactor:  2.0,
	}

	// 发送K桶智能同步请求
	responseData, err := networkService.Call(ctx, targetPeer, protocols.ProtocolKBucketSync, requestData, transportOpts)
	if err != nil {
		return nil, fmt.Errorf("K桶智能同步调用失败: %w", err)
	}

	// 解析响应
	var response protocol.IntelligentPaginationResponse
	if err := proto.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("解析K桶同步响应失败: %w", err)
	}

	// 验证响应
	if !response.Success {
		errorMsg := "未知错误"
		if response.ErrorMessage != nil {
			errorMsg = *response.ErrorMessage
		}
		return nil, fmt.Errorf("K桶同步请求失败: %s", errorMsg)
	}

	if response.RequestId != request.RequestId {
		return nil, fmt.Errorf("响应RequestID不匹配: 期望=%s, 实际=%s",
			request.RequestId, response.RequestId)
	}

	// 使用protobuf统一的区块格式
	blocks := response.Blocks

	if logger != nil {
		logger.Infof("✅ K桶智能同步成功: 接收区块=%d, 数据大小=%d, NextHeight=%d",
			len(blocks), response.ActualSize, response.NextHeight)
	}

	return blocks, nil
}

// ============================================================================
//                           分页补齐同步实现
// ============================================================================

// performRangePaginatedSync 执行分页补齐同步
//
// 🎯 **分页同步策略**：
// 1. 根据剩余高度范围计算需要同步的区块
// 2. 使用分页方式获取区块数据，支持节点故障转移
// 3. 逐批次处理和验证区块
func performRangePaginatedSync(
	ctx context.Context,
	sourcePeers []peer.ID, // 支持多个备用节点的故障转移
	currentHeight, targetHeight uint64,
	networkService network.Network,
	host node.Host,
	blockService blockchain.BlockService,
	configProvider config.Provider,
	logger log.Logger,
) error {
	if len(sourcePeers) == 0 {
		return fmt.Errorf("没有可用的源节点进行分页同步")
	}

	remainingHeight := currentHeight

	// 从配置获取批次大小和故障转移参数
	batchSize := uint64(50) // 默认50个区块
	maxFailuresPerPeer := 3 // 默认每个节点最多失败3次

	blockchainConfig := configProvider.GetBlockchain()
	if blockchainConfig != nil {
		// 获取批次大小配置
		if blockchainConfig.Sync.BatchSize > 0 {
			batchSize = uint64(blockchainConfig.Sync.BatchSize)
		} else if blockchainConfig.Sync.Advanced.MaxBatchSize > 0 {
			batchSize = uint64(blockchainConfig.Sync.Advanced.MaxBatchSize)
		}

		// 获取故障转移策略参数
		if blockchainConfig.Sync.Advanced.MaxRetryAttempts > 0 {
			maxFailuresPerPeer = blockchainConfig.Sync.Advanced.MaxRetryAttempts
		}

		// 根据FailoverNodeCount限制可用节点数量
		if blockchainConfig.Sync.Advanced.FailoverNodeCount > 0 &&
			blockchainConfig.Sync.Advanced.FailoverNodeCount < len(sourcePeers) {
			maxNodes := blockchainConfig.Sync.Advanced.FailoverNodeCount
			if maxNodes < 1 {
				maxNodes = 1
			}
			sourcePeers = sourcePeers[:maxNodes]
			if logger != nil {
				logger.Debugf("📊 基于FailoverNodeCount配置限制节点数量: %d", maxNodes)
			}
		}
	}

	if logger != nil {
		logger.Infof("🔄 开始分页补齐同步: 从高度 %d 到 %d (共%d个区块), 可用节点=%d",
			currentHeight+1, targetHeight, targetHeight-currentHeight, len(sourcePeers))
		logger.Debugf("📊 故障转移配置: 每节点最大失败次数=%d, 批次大小=%d",
			maxFailuresPerPeer, batchSize)
	}

	// 故障转移状态管理
	currentPeerIndex := 0
	failedAttempts := 0

	for remainingHeight < targetHeight {
		// 计算当前批次的结束高度
		batchEndHeight := remainingHeight + batchSize
		if batchEndHeight > targetHeight {
			batchEndHeight = targetHeight
		}

		// 获取当前批次的区块（支持故障转移）
		if currentPeerIndex >= len(sourcePeers) {
			return fmt.Errorf("所有备用节点都已尝试失败")
		}

		currentPeer := sourcePeers[currentPeerIndex]
		blocks, err := fetchBlockRange(ctx, currentPeer, remainingHeight+1, batchEndHeight, networkService, host, configProvider, logger)
		if err != nil {
			failedAttempts++
			if logger != nil {
				logger.Warnf("💥 节点 %s 获取区块失败 (尝试 %d/%d): %v",
					currentPeer.String()[:8], failedAttempts, maxFailuresPerPeer, err)
			}

			// 检查是否需要切换到下一个节点
			if failedAttempts >= maxFailuresPerPeer {
				currentPeerIndex++
				failedAttempts = 0
				if logger != nil {
					logger.Warnf("🔄 节点 %s 失败次数过多，切换到下个节点 (索引: %d)",
						currentPeer.String()[:8], currentPeerIndex)
				}
				if currentPeerIndex >= len(sourcePeers) {
					return fmt.Errorf("所有备用节点都已尝试失败，最后错误: %w", err)
				}
			}
			continue // 重试当前批次
		}

		// 成功获取区块，重置失败计数
		failedAttempts = 0

		// 处理当前批次的区块
		err = processBlockBatch(ctx, blocks, blockService, logger)
		if err != nil {
			return fmt.Errorf("处理区块批次失败: %w", err)
		}

		// 更新进度
		processedInBatch := uint64(len(blocks))
		updateSyncProgress(processedInBatch)
		remainingHeight += processedInBatch

		if logger != nil {
			logger.Infof("📊 分页同步进度: %d/%d (%.1f%%)",
				remainingHeight, targetHeight,
				float64(remainingHeight)/float64(targetHeight)*100.0)
		}

		// 检查是否被取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("分页同步被取消: %w", ctx.Err())
		default:
			// 继续
		}
	}

	if logger != nil {
		logger.Info("✅ range_paginated 协议调用完成")
		logger.Info("🎉 分页补齐同步协议执行成功")
	}

	return nil
}

// fetchBlockRange 获取指定高度范围的区块
//
// 🎯 **智能分页区块范围同步**：
// 1. 构造KBucketSyncRequest（复用作为RangeRequest）
// 2. 使用ProtocolRangePaginated协议发送请求
// 3. 解析IntelligentPaginationResponse响应
// 4. 返回区块列表给调用方处理
//
// 参数：
//   - ctx: 上下文对象（超时控制）
//   - sourcePeer: 源节点ID
//   - startHeight, endHeight: 期望的区块高度范围
//   - networkService: 网络服务接口
//   - logger: 日志记录器
//
// 返回：
//   - []*core.Block: 获取到的区块列表
//   - error: 获取失败时的错误信息
func fetchBlockRange(
	ctx context.Context,
	sourcePeer peer.ID,
	startHeight, endHeight uint64,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) ([]*core.Block, error) {
	if logger != nil {
		logger.Infof("📥 开始从节点 %s 获取区块范围 [%d, %d] (共%d个区块)",
			sourcePeer.String()[:8], startHeight, endHeight, endHeight-startHeight+1)
	}

	// 获取同步配置
	blockchainConfig := configProvider.GetBlockchain()
	var maxResponseSize uint32 = 5 * 1024 * 1024 // 默认5MB
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
		maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
	}

	// 1. 构造KBucketSyncRequest（复用为范围请求）
	request := &protocol.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("range-sync-%d-%d", startHeight, time.Now().UnixNano()),
		LocalHeight:     startHeight - 1,                                            // 本地高度为起始高度前一个
		RoutingKey:      []byte(fmt.Sprintf("range-%d-%d", startHeight, endHeight)), // 使用范围作为路由键
		MaxResponseSize: maxResponseSize,                                            // 从配置获取
		RequesterPeerId: []byte(host.ID().String()),                                 // 本地节点ID（请求者）
		TargetHeight:    &endHeight,                                                 // 目标高度
	}

	// 2. 序列化请求
	reqBytes, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化范围同步请求失败: %w", err)
	}

	if logger != nil {
		logger.Debugf("📤 发送范围同步请求: ID=%s, 大小=%d字节", request.RequestId, len(reqBytes))
	}

	// 3. 配置传输选项（从配置获取）
	var connectTimeout = 10 * time.Second
	var writeTimeout = 15 * time.Second
	var readTimeout = 30 * time.Second
	var maxRetries = 2
	var retryDelay = 1 * time.Second

	if blockchainConfig != nil {
		if blockchainConfig.Sync.Advanced.ConnectTimeout > 0 {
			connectTimeout = blockchainConfig.Sync.Advanced.ConnectTimeout
		}
		if blockchainConfig.Sync.Advanced.WriteTimeout > 0 {
			writeTimeout = blockchainConfig.Sync.Advanced.WriteTimeout
		}
		if blockchainConfig.Sync.Advanced.ReadTimeout > 0 {
			readTimeout = blockchainConfig.Sync.Advanced.ReadTimeout
		}
		if blockchainConfig.Sync.Advanced.MaxRetryAttempts > 0 {
			maxRetries = blockchainConfig.Sync.Advanced.MaxRetryAttempts
		}
		if blockchainConfig.Sync.Advanced.RetryDelay > 0 {
			retryDelay = blockchainConfig.Sync.Advanced.RetryDelay
		}
	}

	// 4. 发送协议请求
	responseBytes, err := networkService.Call(
		ctx,
		sourcePeer,
		protocols.ProtocolRangePaginated,
		reqBytes,
		&types.TransportOptions{
			ConnectTimeout: connectTimeout,
			WriteTimeout:   writeTimeout,
			ReadTimeout:    readTimeout,
			MaxRetries:     maxRetries,
			RetryDelay:     retryDelay,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("发送范围同步请求失败: %w", err)
	}

	if logger != nil {
		logger.Debugf("📦 收到范围同步响应: 大小=%d字节", len(responseBytes))
	}

	// 4. 解析IntelligentPaginationResponse
	response := &protocol.IntelligentPaginationResponse{}
	if err := proto.Unmarshal(responseBytes, response); err != nil {
		return nil, fmt.Errorf("解析范围同步响应失败: %w", err)
	}

	// 5. 检查响应状态
	if !response.Success {
		errorMsg := "未知错误"
		if response.ErrorMessage != nil {
			errorMsg = *response.ErrorMessage
		}
		return nil, fmt.Errorf("对端处理失败: %s", errorMsg)
	}

	// 6. 验证响应内容
	if response.RequestId != request.RequestId {
		return nil, fmt.Errorf("响应ID不匹配: 期望=%s, 实际=%s", request.RequestId, response.RequestId)
	}

	blocks := response.Blocks
	if len(blocks) == 0 {
		if logger != nil {
			logger.Warnf("⚠️ 节点 %s 返回空区块列表 (范围 [%d, %d]), NextHeight=%d",
				sourcePeer.String()[:8], startHeight, endHeight, response.NextHeight)
		}

		// 🔧 **空批次处理策略**：
		// 如果对端返回空区块但提供了NextHeight，说明可以跳过当前范围
		if response.NextHeight > startHeight {
			// 返回特殊的"空跳跃"结果，让上层能根据NextHeight推进
			return []*core.Block{}, &EmptyBatchError{
				StartHeight: startHeight,
				EndHeight:   endHeight,
				NextHeight:  response.NextHeight,
				Reason:      response.PaginationReason,
			}
		}

		// NextHeight未前进，说明节点可能有问题
		return []*core.Block{}, fmt.Errorf("节点返回空批次且未提供有效的NextHeight: start=%d, next=%d",
			startHeight, response.NextHeight)
	}

	// 7. 验证区块高度连续性
	if err := validateBlockSequence(blocks, startHeight, logger); err != nil {
		return nil, fmt.Errorf("区块序列验证失败: %w", err)
	}

	if logger != nil {
		logger.Infof("✅ 成功获取区块范围 [%d, %d]: 返回%d个区块, 大小=%d字节, 分页=%s",
			startHeight, endHeight, len(blocks), response.ActualSize, response.PaginationReason)

		if response.HasMore {
			logger.Infof("📄 还有更多数据，下次请求高度: %d", response.NextHeight)
		}
	}

	return blocks, nil
}

// validateBlockSequence 验证区块序列的连续性和有效性
func validateBlockSequence(blocks []*core.Block, expectedStartHeight uint64, logger log.Logger) error {
	if len(blocks) == 0 {
		return nil // 空序列无需验证
	}

	// 检查第一个区块高度
	firstBlock := blocks[0]
	if firstBlock.Header.Height != expectedStartHeight {
		return fmt.Errorf("首个区块高度不匹配: 期望=%d, 实际=%d",
			expectedStartHeight, firstBlock.Header.Height)
	}

	// 检查区块高度连续性
	for i := 1; i < len(blocks); i++ {
		prevHeight := blocks[i-1].Header.Height
		currentHeight := blocks[i].Header.Height

		if currentHeight != prevHeight+1 {
			return fmt.Errorf("区块高度不连续: 位置%d height=%d, 位置%d height=%d",
				i-1, prevHeight, i, currentHeight)
		}
	}

	if logger != nil {
		logger.Debugf("✅ 区块序列验证通过: 高度范围 [%d, %d]",
			blocks[0].Header.Height, blocks[len(blocks)-1].Header.Height)
	}

	return nil
}

// ============================================================================
//                           区块批处理实现
// ============================================================================

// processBlockBatch 处理区块批次
//
// 🎯 **区块处理策略**：
// 1. 逐个验证区块的有效性
// 2. 验证通过后处理区块（应用状态变更）
// 3. 记录处理结果和错误信息
func processBlockBatch(
	ctx context.Context,
	blocks []*core.Block,
	blockService blockchain.BlockService,
	logger log.Logger,
) error {
	if len(blocks) == 0 {
		return nil // 空批次，直接返回
	}

	if logger != nil {
		logger.Infof("🔨 开始处理区块批次: %d 个区块", len(blocks))
	}

	for i, block := range blocks {
		// 检查取消信号
		select {
		case <-ctx.Done():
			return fmt.Errorf("区块处理被取消: %w", ctx.Err())
		default:
			// 继续处理
		}

		// 验证区块（委托给BlockService，避免重复验证逻辑）
		valid, err := blockService.ValidateBlock(ctx, block)
		if err != nil {
			return fmt.Errorf("验证区块 %d 失败: %w", block.Header.Height, err)
		}

		if !valid {
			return fmt.Errorf("区块 %d 验证失败：区块无效", block.Header.Height)
		}

		// 处理区块（委托给BlockService）
		err = blockService.ProcessBlock(ctx, block)
		if err != nil {
			return fmt.Errorf("处理区块 %d 失败: %w", block.Header.Height, err)
		}

		if logger != nil {
			logger.Debugf("✅ 区块 %d 处理成功 (%d/%d)",
				block.Header.Height, i+1, len(blocks))
		}
	}

	if logger != nil {
		logger.Infof("✅ 区块批次处理完成: %d 个区块", len(blocks))
	}

	return nil
}
