// Package network_handler 实现同步模块的网络协议处理服务
//
// 🎯 **同步网络协议处理服务模块**
//
// 本包实现 SyncProtocolRouter 接口，提供同步网络协议处理功能：
// - 实现 HandleKBucketSync 接口（K桶同步协议处理）
// - 实现 HandleRangePaginated 接口（分页区块范围同步协议处理）
// - 支持区块链数据的高效同步传输
package network_handler

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// SyncNetworkHandler 同步网络协议处理器实现（薄委托层）
//
// 🎯 **职责定位**：
// - 实现 integration/network.SyncProtocolRouter 接口（流式协议）
// - 处理来自其他节点的K桶同步请求
// - 处理来自其他节点的分页区块范围同步请求
// - 执行：解码 → 查询 → 编码 → 响应的完整流程
//
// 🏗️ **设计原则**：
// - 遵循Manager委托模式，作为sync域的网络子模块
// - 统一归口处理所有同步相关的网络消息
// - 使用protobuf标准协议，确保数据一致性
// - 严格遵循公共接口，不直接调用内部实现
type SyncNetworkHandler struct {
	logger            log.Logger                   // 日志服务
	chainService      blockchain.ChainService      // 区块链服务，用于查询本地状态
	repositoryManager repository.RepositoryManager // 数据存储管理器，用于查询区块数据（只读访问）
	configProvider    config.Provider              // 配置提供器

	// 统计信息
	kbucketRequestCount   uint64 // K桶同步请求计数
	rangeRequestCount     uint64 // 范围同步请求计数
	totalBytesTransmitted uint64 // 传输字节总数
}

// NewSyncNetworkHandler 创建同步网络协议处理器实例
//
// 🏗️ **构造函数**：
// 创建SyncNetworkHandler实例，注入必要的依赖。
//
// 参数：
//   - logger: 日志服务，用于记录处理过程
//   - chainService: 区块链服务，用于查询本地状态
//   - repositoryManager: 数据存储管理器，用于查询区块数据（只读访问）
//   - configProvider: 配置提供器，用于获取同步配置参数
//
// 返回：
//   - *SyncNetworkHandler: 同步网络协议处理器实例
func NewSyncNetworkHandler(logger log.Logger, chainService blockchain.ChainService, repositoryManager repository.RepositoryManager, configProvider config.Provider) *SyncNetworkHandler {
	return &SyncNetworkHandler{
		logger:            logger,
		chainService:      chainService,
		repositoryManager: repositoryManager,
		configProvider:    configProvider,
	}
}

// HandleKBucketSync 处理K桶同步协议请求
//
// 🎯 **实现 integration/network.SyncProtocolRouter 接口**
//
// 处理流程：
// 1. 解析K桶同步请求数据（包含请求节点信息）
// 2. 验证请求的完整性和格式
// 3. 查询本地区块高度和网络状态信息
// 4. 构造智能分页响应数据
// 5. 序列化响应并返回
//
// 📝 **K桶同步特性**：
// - 基于Kademlia距离计算的智能节点选择
// - 高效的网络拓扑感知同步
// - 支持分层的P2P网络架构
//
// 参数：
//   - ctx: 上下文对象（用于超时控制）
//   - from: 请求来源节点ID
//   - reqBytes: 序列化的K桶同步请求数据
//
// 返回：
//   - []byte: 序列化的智能分页响应数据
//   - error: 处理失败时的错误信息
func (h *SyncNetworkHandler) HandleKBucketSync(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	h.kbucketRequestCount++

	if h.logger != nil {
		h.logger.Debugf("[SyncNetworkHandler] 📚 收到K桶同步请求: from=%s, size=%d bytes",
			from.String()[:8], len(reqBytes))
	}

	// 1. 解析KBucketSyncRequest protobuf消息
	request := &protocol.KBucketSyncRequest{}
	if err := proto.Unmarshal(reqBytes, request); err != nil {
		if h.logger != nil {
			h.logger.Errorf("解析K桶同步请求失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "解析请求失败", fmt.Sprintf("protobuf解析错误: %v", err))
	}

	// 2. 查询本地区块链高度和状态
	chainInfo, err := h.chainService.GetChainInfo(ctx)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("查询本地链状态失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "链状态查询失败", err.Error())
	}

	// 3. 检查是否为高度查询请求
	isHeightQuery := (request.LocalHeight == 0 && string(request.RoutingKey) == "height-query")

	if isHeightQuery {
		// 高度查询：返回本地高度信息，不提供区块数据
		if h.logger != nil {
			h.logger.Debugf("处理高度查询请求: 本地高度=%d", chainInfo.Height)
		}
		return h.createHeightQueryResponse(request.RequestId, chainInfo.Height)
	}

	// 4. 处理标准K桶同步请求
	if h.logger != nil {
		h.logger.Debugf("处理K桶同步请求: 请求高度=%d, 本地高度=%d", request.LocalHeight, chainInfo.Height)
	}

	// 4.1 判断是否需要提供区块数据
	if request.LocalHeight >= chainInfo.Height {
		// 请求者已是最新或更新，返回空响应
		if h.logger != nil {
			h.logger.Debugf("请求者已是最新: 本地高度=%d >= 链高度=%d",
				request.LocalHeight, chainInfo.Height)
		}
		return h.createEmptyResponse(request.RequestId, chainInfo.Height)
	}

	// 4.2 使用智能分页逻辑构建区块响应
	startHeight := request.LocalHeight + 1
	targetHeight := chainInfo.Height
	if request.TargetHeight != nil && *request.TargetHeight < chainInfo.Height {
		targetHeight = *request.TargetHeight
	}

	maxResponseSize := request.MaxResponseSize
	if maxResponseSize == 0 {
		// 🔧 **统一服务端响应大小配置**：优先从配置获取，确保客户端/服务端策略一致
		maxResponseSize = 2 * 1024 * 1024 // 兜底默认值：2MB（KBucket同步较小响应）
		if h.configProvider != nil {
			blockchainConfig := h.configProvider.GetBlockchain()
			if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
				maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
				if h.logger != nil {
					h.logger.Debugf("📊 从配置获取KBucket响应大小限制: %d 字节", maxResponseSize)
				}
			}
		}
	}

	blocks, nextHeight, hasMore, actualSize, paginationReason := h.buildBlockBatch(
		ctx, startHeight, targetHeight, maxResponseSize)

	// 4.3 构造K桶同步响应（使用相同的protobuf结构）
	response := &protocol.IntelligentPaginationResponse{
		RequestId:        request.RequestId,
		Blocks:           blocks,
		NextHeight:       nextHeight,
		HasMore:          hasMore,
		ActualSize:       actualSize,
		PaginationReason: fmt.Sprintf("KBUCKET_SYNC_%s", paginationReason),
		Success:          true,
		ErrorMessage:     nil,
	}

	responseData, err := proto.Marshal(response)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("序列化K桶响应失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "响应序列化失败", err.Error())
	}

	h.totalBytesTransmitted += uint64(len(responseData))

	if h.logger != nil {
		h.logger.Infof("✅ [SyncNetworkHandler] K桶同步请求处理完成: from=%s, response_size=%d",
			from.String()[:8], len(responseData))
	}

	return responseData, nil
}

// HandleRangePaginated 处理分页区块范围同步协议请求
//
// 🎯 **实现 integration/network.SyncProtocolRouter 接口**
//
// 处理流程：
// 1. 解析分页区块范围同步请求（起始高度、结束高度、页大小）
// 2. 验证请求参数的合法性（高度范围、页大小限制）
// 3. 查询指定范围内的区块数据
// 4. 分页处理区块数据，支持大范围查询
// 5. 构造智能分页响应，包含区块数据和分页信息
//
// 📝 **分页同步特性**：
// - 支持大范围区块数据查询
// - 智能分页机制，避免单次传输过大
// - 断点续传支持，提高同步效率
// - 网络友好的批量传输优化
//
// 参数：
//   - ctx: 上下文对象（用于超时控制和取消）
//   - from: 请求来源节点ID
//   - reqBytes: 序列化的分页范围同步请求数据
//
// 返回：
//   - []byte: 序列化的智能分页响应数据（包含区块数据）
//   - error: 处理失败时的错误信息
func (h *SyncNetworkHandler) HandleRangePaginated(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	h.rangeRequestCount++

	if h.logger != nil {
		h.logger.Debugf("[SyncNetworkHandler] 📄 收到分页区块范围同步请求: from=%s, size=%d bytes",
			from.String()[:8], len(reqBytes))
	}

	// 1. 解析KBucketSyncRequest protobuf消息（复用相同请求格式）
	request := &protocol.KBucketSyncRequest{}
	if err := proto.Unmarshal(reqBytes, request); err != nil {
		if h.logger != nil {
			h.logger.Errorf("解析分页同步请求失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "解析请求失败", fmt.Sprintf("protobuf解析错误: %v", err))
	}

	// 2. 查询本地区块链状态
	chainInfo, err := h.chainService.GetChainInfo(ctx)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("查询本地链状态失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "链状态查询失败", err.Error())
	}

	// 3. 验证请求的高度范围合法性
	startHeight := request.LocalHeight + 1 // 请求者的下一个高度
	targetHeight := chainInfo.Height       // 本地最新高度

	if request.TargetHeight != nil && *request.TargetHeight < targetHeight {
		targetHeight = *request.TargetHeight // 使用请求的目标高度
	}

	if startHeight > targetHeight {
		// 请求者已是最新，返回空响应
		if h.logger != nil {
			h.logger.Debugf("请求者 %s 已是最新: 请求高度=%d, 本地高度=%d",
				from.String()[:8], startHeight, targetHeight)
		}
		return h.createEmptyResponse(request.RequestId, startHeight)
	}

	// 4. 实施智能分页逻辑
	maxResponseSize := request.MaxResponseSize
	if maxResponseSize == 0 {
		// 🔧 **统一服务端响应大小配置**：优先从配置获取，确保客户端/服务端策略一致
		maxResponseSize = 5 * 1024 * 1024 // 兜底默认值：5MB（范围分页需要更大响应）
		if h.configProvider != nil {
			blockchainConfig := h.configProvider.GetBlockchain()
			if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
				maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
				if h.logger != nil {
					h.logger.Debugf("📊 从配置获取范围分页响应大小限制: %d 字节", maxResponseSize)
				}
			}
		}
	}

	blocks, nextHeight, hasMore, actualSize, paginationReason := h.buildBlockBatch(
		ctx, startHeight, targetHeight, maxResponseSize)

	// 5. 构造IntelligentPaginationResponse
	response := &protocol.IntelligentPaginationResponse{
		RequestId:        request.RequestId,
		Blocks:           blocks,
		NextHeight:       nextHeight,
		HasMore:          hasMore,
		ActualSize:       actualSize,
		PaginationReason: paginationReason,
		Success:          true,
		ErrorMessage:     nil,
	}

	// 6. 序列化并返回响应数据
	responseData, err := proto.Marshal(response)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("序列化分页响应失败: %v", err)
		}
		return h.createErrorResponse(request.RequestId, "响应序列化失败", err.Error())
	}

	h.totalBytesTransmitted += uint64(len(responseData))

	if h.logger != nil {
		h.logger.Infof("✅ [SyncNetworkHandler] 分页范围同步请求处理完成: from=%s, response_size=%d",
			from.String()[:8], len(responseData))
	}

	return responseData, nil
}

// ============================================================================
//                              响应创建辅助方法
// ============================================================================

// createErrorResponse 创建错误响应
func (h *SyncNetworkHandler) createErrorResponse(requestId, reason, detail string) ([]byte, error) {
	response := &protocol.IntelligentPaginationResponse{
		RequestId:    requestId,
		Blocks:       []*core.Block{},
		NextHeight:   0,
		HasMore:      false,
		ActualSize:   0,
		Success:      false,
		ErrorMessage: &detail,
	}

	return proto.Marshal(response)
}

// createEmptyResponse 创建空响应（没有新区块）
func (h *SyncNetworkHandler) createEmptyResponse(requestId string, nextHeight uint64) ([]byte, error) {
	response := &protocol.IntelligentPaginationResponse{
		RequestId:        requestId,
		Blocks:           []*core.Block{},
		NextHeight:       nextHeight,
		HasMore:          false,
		ActualSize:       0,
		PaginationReason: "NO_NEW_BLOCKS",
		Success:          true,
		ErrorMessage:     nil,
	}

	return proto.Marshal(response)
}

// buildBlockBatch 构建区块批次（智能分页逻辑）
//
// 🎯 **智能分页算法**：
// 1. 从startHeight开始逐个查询区块
// 2. 累积区块大小，直到接近maxResponseSize限制
// 3. 至少返回1个区块，确保同步进展
// 4. 返回区块列表和分页信息
//
// 📋 **使用repository.RepositoryManager进行区块查询**：
// - 严格遵循单一数据源原则，通过repository层获取区块数据
// - 支持智能分页，根据区块大小动态调整批次
// - 确保网络传输效率和资源使用的平衡
func (h *SyncNetworkHandler) buildBlockBatch(
	ctx context.Context,
	startHeight, targetHeight uint64,
	maxResponseSize uint32,
) ([]*core.Block, uint64, bool, uint32, string) {

	var blocks []*core.Block
	var actualSize uint32
	paginationReason := "NORMAL_BATCH"

	if h.logger != nil {
		h.logger.Debugf("📄 构建区块批次: 范围[%d, %d], 大小限制=%d字节",
			startHeight, targetHeight, maxResponseSize)
	}

	// 智能分页逻辑：逐个获取区块，直到接近大小限制
	currentHeight := startHeight
	for currentHeight <= targetHeight {
		// 使用repository.RepositoryManager获取单个区块
		block, err := h.repositoryManager.GetBlockByHeight(ctx, currentHeight)
		if err != nil {
			if h.logger != nil {
				h.logger.Warnf("获取区块失败: 高度=%d, 错误=%v", currentHeight, err)
			}
			// 区块获取失败，结束批次构建
			break
		}

		// 计算区块序列化大小
		blockBytes, err := proto.Marshal(block)
		if err != nil {
			if h.logger != nil {
				h.logger.Warnf("区块序列化失败: 高度=%d, 错误=%v", currentHeight, err)
			}
			currentHeight++
			continue
		}

		blockSize := uint32(len(blockBytes))

		// 检查是否会超过大小限制
		if len(blocks) > 0 && actualSize+blockSize > maxResponseSize {
			// 已有区块且会超过限制，停止添加
			paginationReason = "SIZE_LIMIT_REACHED"
			break
		}

		// 添加区块到批次
		blocks = append(blocks, block)
		actualSize += blockSize
		currentHeight++

		// 确保至少返回一个区块
		if len(blocks) == 1 && actualSize > maxResponseSize {
			// 单个区块就超过限制，但仍需返回以确保同步进展
			paginationReason = "LARGE_BLOCK_FORCED"
			break
		}
	}

	// 计算下一个高度和是否还有更多区块
	nextHeight := currentHeight
	hasMore := (currentHeight <= targetHeight)

	if len(blocks) == 0 {
		paginationReason = "NO_BLOCKS_AVAILABLE"
		nextHeight = startHeight
	}

	if h.logger != nil {
		h.logger.Infof("✅ 区块批次构建完成: 返回%d个区块 [%d-%d], 大小=%d字节, 下一高度=%d, 还有更多=%t, 原因=%s",
			len(blocks), startHeight, currentHeight-1, actualSize, nextHeight, hasMore, paginationReason)
	}

	return blocks, nextHeight, hasMore, actualSize, paginationReason
}

// createHeightQueryResponse 创建高度查询响应
func (h *SyncNetworkHandler) createHeightQueryResponse(requestId string, currentHeight uint64) ([]byte, error) {
	response := &protocol.IntelligentPaginationResponse{
		RequestId:        requestId,
		Blocks:           []*core.Block{},
		NextHeight:       currentHeight, // 在NextHeight字段返回当前高度
		HasMore:          false,
		ActualSize:       0,
		PaginationReason: "HEIGHT_QUERY",
		Success:          true,
		ErrorMessage:     nil,
	}

	return proto.Marshal(response)
}

// GetSyncNetworkStats 获取同步网络处理统计信息
//
// 📊 **网络统计信息查询**
//
// 返回sync网络处理模块的统计数据，用于监控和性能分析。
//
// 返回：
//   - map[string]interface{}: 网络处理统计信息
func (h *SyncNetworkHandler) GetSyncNetworkStats() map[string]interface{} {
	totalRequests := h.kbucketRequestCount + h.rangeRequestCount

	avgBytesPerRequest := float64(0)
	if totalRequests > 0 {
		avgBytesPerRequest = float64(h.totalBytesTransmitted) / float64(totalRequests)
	}

	return map[string]interface{}{
		"kbucket_request_count":    h.kbucketRequestCount,
		"range_request_count":      h.rangeRequestCount,
		"total_requests_processed": totalRequests,
		"total_bytes_transmitted":  h.totalBytesTransmitted,
		"avg_bytes_per_request":    avgBytesPerRequest,
	}
}
