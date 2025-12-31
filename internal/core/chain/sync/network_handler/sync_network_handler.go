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
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/config/node"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
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
	logger          log.Logger                  // 日志服务
	chainQuery      persistence.ChainQuery      // 链状态查询服务（读操作）
	queryService    persistence.QueryService    // 统一查询服务（读操作，替代RepositoryManager）
	configProvider  config.Provider             // 配置提供器
	blockHashClient core.BlockHashServiceClient // 本地区块哈希客户端（用于计算任意高度的hash）

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
// 🎯 **适配新的依赖注入架构**：
// - chainQuery: 使用persistence.ChainQuery替代ChainService（读操作）
// - queryService: 使用persistence.QueryService替代RepositoryManager（读操作）
//
// 参数：
//   - logger: 日志服务，用于记录处理过程
//   - chainQuery: 链状态查询服务（读操作）
//   - queryService: 统一查询服务（读操作，替代RepositoryManager）
//   - configProvider: 配置提供器，用于获取同步配置参数
//
// 返回：
//   - *SyncNetworkHandler: 同步网络协议处理器实例
func NewSyncNetworkHandler(logger log.Logger, chainQuery persistence.ChainQuery, queryService persistence.QueryService, configProvider config.Provider, blockHashClient core.BlockHashServiceClient) *SyncNetworkHandler {
	return &SyncNetworkHandler{
		logger:          logger,
		chainQuery:      chainQuery,
		queryService:    queryService,
		configProvider:  configProvider,
		blockHashClient: blockHashClient,
	}
}

func (h *SyncNetworkHandler) getBlockHashByHeight(ctx context.Context, height uint64) ([]byte, error) {
	if h == nil || h.queryService == nil {
		return nil, fmt.Errorf("queryService 未注入")
	}
	blk, err := h.queryService.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	if blk == nil || blk.Header == nil {
		return nil, fmt.Errorf("block is nil at height=%d", height)
	}
	// 为避免依赖 indices:height 的存储 hash，统一用同一套确定性算法计算
	if h.blockHashClient == nil {
		return nil, fmt.Errorf("blockHashClient 未注入")
	}
	resp, err := h.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
		return nil, fmt.Errorf("invalid block hash response")
	}
	return resp.Hash, nil
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

	// 2. 获取本地链身份（用于后续响应填充）
	var localIdentity types.ChainIdentity
	var hasLocalIdentity bool
	localIdentity, err := h.getLocalChainIdentity(ctx)
	if err != nil {
		if h.logger != nil {
			h.logger.Warnf("获取本地链身份失败: %v", err)
		}
	} else {
		hasLocalIdentity = localIdentity.IsValid()
	}

	// 2.1 验证请求的链身份（如果请求中包含）
	if request.ChainIdentity != nil {
		if !hasLocalIdentity {
			if h.logger != nil {
				h.logger.Warnf("本地链身份无效，跳过链身份验证")
			}
		} else {
			remoteIdentity := node.FromProtoChainIdentity(request.ChainIdentity)
			if !localIdentity.IsSameChain(remoteIdentity) {
				if h.logger != nil {
					h.logger.Warnf("policy.reject_sync_peer: 链身份不匹配, remote=%v local=%v", remoteIdentity, localIdentity)
				}
				return h.createErrorResponse(request.RequestId, "链身份不匹配", fmt.Sprintf("remote=%v local=%v", remoteIdentity, localIdentity))
			}
		}
	}

	// 3. 查询本地区块链高度和状态
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
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
		return h.createHeightQueryResponse(request.RequestId, chainInfo.Height, hasLocalIdentity, localIdentity)
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
		return h.createEmptyResponse(request.RequestId, chainInfo.Height, hasLocalIdentity, localIdentity)
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

	// 填充链身份（供客户端 double-check）
	if hasLocalIdentity {
		response.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
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

	// 获取本地链身份（用于后续响应填充）
	var localIdentity types.ChainIdentity
	var hasLocalIdentity bool
	localIdentity, err := h.getLocalChainIdentity(ctx)
	if err != nil {
		if h.logger != nil {
			h.logger.Warnf("获取本地链身份失败: %v", err)
		}
	} else {
		hasLocalIdentity = localIdentity.IsValid()
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
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
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
		return h.createEmptyResponse(request.RequestId, startHeight, hasLocalIdentity, localIdentity)
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

	// 填充链身份（供客户端 double-check）
	if hasLocalIdentity {
		response.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
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

	data, err := proto.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化错误响应失败: %w", err)
	}
	return data, nil
}

// createEmptyResponse 创建空响应（没有新区块）
func (h *SyncNetworkHandler) createEmptyResponse(requestId string, nextHeight uint64, hasLocalIdentity bool, localIdentity types.ChainIdentity) ([]byte, error) {
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

	// 填充链身份
	if hasLocalIdentity {
		response.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
	}

	data, err := proto.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化空响应失败: %w", err)
	}
	return data, nil
}

// buildBlockBatch 构建区块批次（智能分页逻辑）
//
// 🎯 **智能分页算法**：
// 1. 从startHeight开始逐个查询区块
// 2. 累积区块大小，直到接近maxResponseSize限制
// 3. 至少返回1个区块，确保同步进展
// 4. 返回区块列表和分页信息
//
// 📋 **使用persistence.QueryService进行区块查询**：
// - 严格遵循单一数据源原则，通过QueryService获取区块数据
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
		// 使用persistence.QueryService获取单个区块
		block, err := h.queryService.GetBlockByHeight(ctx, currentHeight)
		if err != nil {
			if h.logger != nil {
				h.logger.Warnf("获取区块失败: 高度=%d, 错误=%v", currentHeight, err)
			}
			// 区块获取失败，结束批次构建
			break
		}

		// 检查区块是否为 nil
		if block == nil {
			if h.logger != nil {
				h.logger.Warnf("获取到的区块为 nil: 高度=%d", currentHeight)
			}
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

// HandleSyncHelloV2 处理 Sync v2 握手（fork-aware）。
//
// ⚠️ 当前实现使用 KBucketSyncRequest 作为 v2 hello 的载体：
// - request.local_height 视为请求方 tip_height
// - request.routing_key 视为请求方 tip_hash（32 bytes）
//
// 原因：当前环境无法运行 protoc（x86_64 主机 + arm64 protoc），无法自动再生 pb.go。
// 语义上仍然实现“携带高度+哈希并判定分叉/同链”的核心能力，后续可替换为 SyncHelloV2Request/Response。
func (h *SyncNetworkHandler) HandleSyncHelloV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if h.logger != nil {
		h.logger.Debugf("[SyncNetworkHandler] 🤝 收到SyncHelloV2请求: from=%s, size=%d bytes", from.String()[:8], len(reqBytes))
	}

	req := &protocol.KBucketSyncRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return h.createErrorResponse(req.GetRequestId(), "解析请求失败", fmt.Sprintf("protobuf解析错误: %v", err))
	}

	// 链身份校验（复用现有逻辑）
	localIdentity, err := h.getLocalChainIdentity(ctx)
	hasLocalIdentity := err == nil && localIdentity.IsValid()
	// ✅ v2 硬门槛：必须携带 chain_identity，且必须与本地一致；否则视为“不兼容 peer”
	if !hasLocalIdentity {
		return h.createErrorResponse(req.RequestId, "本地链身份不可用", "local chain identity not available")
	}
	if req.ChainIdentity == nil {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncHelloV2 缺少 chain_identity, from=%s", from.String()[:8])
		}
		return h.createErrorResponse(req.RequestId, "缺少链身份", "missing chain_identity")
	}
	remoteIdentity := node.FromProtoChainIdentity(req.ChainIdentity)
	if !remoteIdentity.IsValid() {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncHelloV2 链身份无效, from=%s remote=%v", from.String()[:8], remoteIdentity)
		}
		return h.createErrorResponse(req.RequestId, "链身份无效", fmt.Sprintf("remote=%v", remoteIdentity))
	}
	if !localIdentity.IsSameChain(remoteIdentity) {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncHelloV2 链身份不匹配, remote=%v local=%v", remoteIdentity, localIdentity)
		}
		return h.createErrorResponse(req.RequestId, "链身份不匹配", fmt.Sprintf("remote=%v local=%v", remoteIdentity, localIdentity))
	}

	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return h.createErrorResponse(req.RequestId, "链状态查询失败", err.Error())
	}
	if chainInfo == nil {
		return h.createErrorResponse(req.RequestId, "链状态为空", "chainInfo is nil")
	}

	remoteTipHeight := chainInfo.Height
	remoteTipHash := chainInfo.BestBlockHash

	localTipHeight := req.LocalHeight
	localTipHash := req.RoutingKey

	relationship := "UNKNOWN"
	commonAncestorHeight := uint64(0)
	commonAncestorHash := []byte(nil)
	locatorLen := len(req.RequesterPeerId)
	locatorValid := false

	// 关键：用 (height, hash) 判断“是否在同一条链上”
	switch {
	case localTipHeight > remoteTipHeight:
		relationship = "REMOTE_BEHIND"
	case localTipHeight == remoteTipHeight:
		if len(remoteTipHash) == 32 && len(localTipHash) == 32 && bytes.Equal(remoteTipHash, localTipHash) {
			relationship = "UP_TO_DATE"
		} else {
			relationship = "FORK_DETECTED"
		}
	case localTipHeight < remoteTipHeight:
		// 🆕 优化：如果请求方高度为0，视为空链，直接返回 REMOTE_AHEAD_SAME_CHAIN
		if localTipHeight == 0 {
			relationship = "REMOTE_AHEAD_SAME_CHAIN"
			// 空链场景：不需要进行fork检测，直接允许普通同步
		} else {
		// 对端领先：检查对端在 localTipHeight 处的 hash 是否与请求方一致
		if len(localTipHash) == 32 {
			hh, he := h.getBlockHashByHeight(ctx, localTipHeight)
			if he == nil && len(hh) == 32 && bytes.Equal(hh, localTipHash) {
				relationship = "REMOTE_AHEAD_SAME_CHAIN"
			} else {
				relationship = "FORK_DETECTED"
			}
		} else {
			relationship = "UNKNOWN"
			}
		}
	}

	// fork detected：尝试用 locator 反查共同祖先（不依赖 hash->height 索引）
	if relationship == "FORK_DETECTED" {
		if len(parseBlockLocatorBinary(req.RequesterPeerId)) > 0 {
			locatorValid = true
		}
		if ah, ahash, ok := h.findCommonAncestorByLocator(ctx, req.RequesterPeerId, remoteTipHeight); ok {
			commonAncestorHeight = ah
			commonAncestorHash = ahash
		}
	}

	reason := fmt.Sprintf("SYNCV2_HELLO:%s remote_tip=%d local_tip=%d local_tip_hash=%s ancestor=%d:%s locator_len=%d locator_valid=%t",
		relationship, remoteTipHeight, localTipHeight, shortHex(localTipHash), commonAncestorHeight, hex.EncodeToString(commonAncestorHash), locatorLen, locatorValid)

	resp := &protocol.IntelligentPaginationResponse{
		RequestId:        req.RequestId,
		Blocks:           []*core.Block{},
		NextHeight:       remoteTipHeight,
		HasMore:          false,
		ActualSize:       0,
		PaginationReason: reason,
		Success:          true,
		ErrorMessage:     nil,
	}
	if hasLocalIdentity {
		resp.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
	}

	out, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("序列化SyncHelloV2响应失败: %w", err)
	}
	return out, nil
}

// HandleSyncBlocksV2 处理 Sync v2 区块批量同步（fork-aware）。
//
// ⚠️ 当前实现使用 KBucketSyncRequest 作为 v2 blocks 的载体：
// - request.local_height 视为 from_height-1
// - request.target_height 视为 to_height
// - request.max_response_size 作为响应大小上限
func (h *SyncNetworkHandler) HandleSyncBlocksV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if h.logger != nil {
		h.logger.Debugf("[SyncNetworkHandler] 📦 收到SyncBlocksV2请求: from=%s, size=%d bytes", from.String()[:8], len(reqBytes))
	}

	req := &protocol.KBucketSyncRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return h.createErrorResponse(req.GetRequestId(), "解析请求失败", fmt.Sprintf("protobuf解析错误: %v", err))
	}

	// 链身份校验（复用现有逻辑）
	localIdentity, err := h.getLocalChainIdentity(ctx)
	hasLocalIdentity := err == nil && localIdentity.IsValid()
	// ✅ v2 硬门槛：必须携带 chain_identity，且必须与本地一致；否则视为“不兼容 peer”
	if !hasLocalIdentity {
		return h.createErrorResponse(req.RequestId, "本地链身份不可用", "local chain identity not available")
	}
	if req.ChainIdentity == nil {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncBlocksV2 缺少 chain_identity, from=%s", from.String()[:8])
		}
		return h.createErrorResponse(req.RequestId, "缺少链身份", "missing chain_identity")
	}
	remoteIdentity := node.FromProtoChainIdentity(req.ChainIdentity)
	if !remoteIdentity.IsValid() {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncBlocksV2 链身份无效, from=%s remote=%v", from.String()[:8], remoteIdentity)
		}
		return h.createErrorResponse(req.RequestId, "链身份无效", fmt.Sprintf("remote=%v", remoteIdentity))
	}
	if !localIdentity.IsSameChain(remoteIdentity) {
		if h.logger != nil {
			h.logger.Warnf("policy.reject_sync_peer: SyncBlocksV2 链身份不匹配, remote=%v local=%v", remoteIdentity, localIdentity)
		}
		return h.createErrorResponse(req.RequestId, "链身份不匹配", fmt.Sprintf("remote=%v local=%v", remoteIdentity, localIdentity))
	}

	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return h.createErrorResponse(req.RequestId, "链状态查询失败", err.Error())
	}
	if chainInfo == nil {
		return h.createErrorResponse(req.RequestId, "链状态为空", "chainInfo is nil")
	}

	startHeight := req.LocalHeight + 1
	targetHeight := chainInfo.Height
	if req.TargetHeight != nil && *req.TargetHeight < targetHeight {
		targetHeight = *req.TargetHeight
	}
	if startHeight > targetHeight {
		// 无新区块
		return h.createEmptyResponse(req.RequestId, startHeight, hasLocalIdentity, localIdentity)
	}

	maxResponseSize := req.MaxResponseSize
	if maxResponseSize == 0 {
		maxResponseSize = 5 * 1024 * 1024
		if h.configProvider != nil {
			if bc := h.configProvider.GetBlockchain(); bc != nil && bc.Sync.Advanced.MaxResponseSizeBytes > 0 {
				maxResponseSize = bc.Sync.Advanced.MaxResponseSizeBytes
			}
		}
	}

	blocks, nextHeight, hasMore, actualSize, paginationReason := h.buildBlockBatch(ctx, startHeight, targetHeight, maxResponseSize)
	resp := &protocol.IntelligentPaginationResponse{
		RequestId:        req.RequestId,
		Blocks:           blocks,
		NextHeight:       nextHeight,
		HasMore:          hasMore,
		ActualSize:       actualSize,
		PaginationReason: "SYNCV2_BLOCKS_" + paginationReason,
		Success:          true,
		ErrorMessage:     nil,
	}
	if hasLocalIdentity {
		resp.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
	}

	out, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("序列化SyncBlocksV2响应失败: %w", err)
	}
	return out, nil
}

func shortHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	h := hex.EncodeToString(b)
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "..."
}

type locatorEntry struct {
	height uint64
	hash   []byte
}

// parseBlockLocatorBinary 解析 locator 的二进制编码：
// 每个 entry 固定 40 bytes = height(8, big-endian) + hash(32)
func parseBlockLocatorBinary(b []byte) []locatorEntry {
	const entrySize = 8 + 32
	if len(b) < entrySize || len(b)%entrySize != 0 {
		return nil
	}
	n := len(b) / entrySize
	out := make([]locatorEntry, 0, n)
	for i := 0; i < n; i++ {
		off := i * entrySize
		h := bytesToUint64BE(b[off : off+8])
		hash := append([]byte(nil), b[off+8:off+entrySize]...)
		out = append(out, locatorEntry{height: h, hash: hash})
	}
	return out
}

func bytesToUint64BE(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// findCommonAncestorByLocator 在本地链上寻找与对端 locator 匹配的最高共同祖先。
func (h *SyncNetworkHandler) findCommonAncestorByLocator(ctx context.Context, locatorBytes []byte, remoteTipHeight uint64) (uint64, []byte, bool) {
	entries := parseBlockLocatorBinary(locatorBytes)
	if len(entries) == 0 {
		return 0, nil, false
	}
	for _, e := range entries {
		if e.height > remoteTipHeight {
			continue
		}
		rh, err := h.getBlockHashByHeight(ctx, e.height)
		if err != nil || len(rh) != 32 || len(e.hash) != 32 {
			continue
		}
		if bytes.Equal(rh, e.hash) {
			return e.height, e.hash, true
		}
	}
	return 0, nil, false
}

// createHeightQueryResponse 创建高度查询响应
func (h *SyncNetworkHandler) createHeightQueryResponse(requestId string, currentHeight uint64, hasLocalIdentity bool, localIdentity types.ChainIdentity) ([]byte, error) {
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

	// 填充链身份
	if hasLocalIdentity {
		response.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
	}

	data, err := proto.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化高度查询响应失败: %w", err)
	}
	return data, nil
}

// getLocalChainIdentity 获取本地链身份（内部辅助方法）
func (h *SyncNetworkHandler) getLocalChainIdentity(ctx context.Context) (types.ChainIdentity, error) {
	if h.configProvider == nil {
		return types.ChainIdentity{}, fmt.Errorf("config provider 不能为空")
	}

	appConfig := h.configProvider.GetAppConfig()
	if appConfig == nil {
		return types.ChainIdentity{}, fmt.Errorf("app config 不能为空")
	}

	genesisConfig := h.configProvider.GetUnifiedGenesisConfig()
	if genesisConfig == nil {
		return types.ChainIdentity{}, fmt.Errorf("genesis config 不能为空")
	}

	// 从配置计算 genesis hash
	genesisHash, err := node.CalculateGenesisHash(genesisConfig)
	if err != nil {
		return types.ChainIdentity{}, fmt.Errorf("计算 genesis hash 失败: %w", err)
	}

	// 构建 ChainIdentity
	identity := node.BuildLocalChainIdentity(appConfig, genesisHash)
	if !identity.IsValid() {
		return types.ChainIdentity{}, fmt.Errorf("构建的链身份无效: %v", identity)
	}

	return identity, nil
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
