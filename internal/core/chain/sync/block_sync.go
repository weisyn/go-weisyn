// block_sync.go - 区块同步核心逻辑
// 负责执行K桶智能同步和分页补齐同步
package sync

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/weisyn/v1/internal/config/node"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

type helloV2Info struct {
	relationship         string
	remoteTipHeight      uint64
	commonAncestorHeight uint64
	commonAncestorHash   []byte
}

func parseHelloV2Reason(reason string) helloV2Info {
	info := helloV2Info{
		relationship:         "UNKNOWN",
		remoteTipHeight:      0,
		commonAncestorHeight: 0,
		commonAncestorHash:   nil,
	}
	// 形如：SYNCV2_HELLO:<REL> remote_tip=... local_tip=... ancestor=<h>:<hex>
	if !strings.HasPrefix(reason, "SYNCV2_HELLO:") {
		return info
	}
	rest := strings.TrimPrefix(reason, "SYNCV2_HELLO:")
	// rel 在第一个空格之前
	if sp := strings.IndexByte(rest, ' '); sp > 0 {
		info.relationship = rest[:sp]
	} else if rest != "" {
		info.relationship = rest
	}

	// remote_tip
	if idx := strings.Index(reason, "remote_tip="); idx >= 0 {
		sub := reason[idx+len("remote_tip="):]
		end := strings.IndexByte(sub, ' ')
		if end > 0 {
			sub = sub[:end]
		}
		if v, err := strconv.ParseUint(sub, 10, 64); err == nil {
			info.remoteTipHeight = v
		}
	}

	// ancestor
	if idx := strings.Index(reason, "ancestor="); idx >= 0 {
		sub := reason[idx+len("ancestor="):]
		end := strings.IndexByte(sub, ' ')
		if end > 0 {
			sub = sub[:end]
		}
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) >= 1 {
			if v, err := strconv.ParseUint(parts[0], 10, 64); err == nil {
				info.commonAncestorHeight = v
			}
		}
		if len(parts) == 2 && parts[1] != "" {
			if b, err := hex.DecodeString(parts[1]); err == nil && len(b) == 32 {
				info.commonAncestorHash = b
			}
		}
	}
	return info
}

func performSyncHelloV2(
	ctx context.Context,
	targetPeer peer.ID,
	localTipHeight uint64,
	localTipHash []byte,
	locatorBytes []byte,
	localChainInfo *types.ChainInfo,
	networkService network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	logger log.Logger,
) (*helloV2Info, error) {
	if logger != nil {
		logger.Debugf("🤝 向节点 %s 发起 SyncHelloV2", targetPeer.String()[:8])
	}
	if len(localTipHash) != 32 {
		return nil, fmt.Errorf("local tip hash invalid (len=%d)", len(localTipHash))
	}
	// ✅ v2 硬门槛：SyncHelloV2 必须携带本地链身份（用于对端校验），且必须能被本地获取/验证
	localChainIdentity, err := GetLocalChainIdentity(ctx, configProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("获取本地链身份失败（SyncHelloV2 必需）: %w", err)
	}
	if !localChainIdentity.IsValid() {
		return nil, fmt.Errorf("本地链身份无效（SyncHelloV2 必需）: %v", localChainIdentity)
	}

	maxResponseSize := uint32(MAX_RESPONSE_SIZE_LIMIT)
	if bc := configProvider.GetBlockchain(); bc != nil && bc.Sync.Advanced.MaxResponseSizeBytes > 0 {
		if bc.Sync.Advanced.MaxResponseSizeBytes < maxResponseSize {
			maxResponseSize = bc.Sync.Advanced.MaxResponseSizeBytes
		}
	}

	req := &protocol.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("sync-hello-v2-%d", time.Now().UnixNano()),
		LocalHeight:     localTipHeight,
		RoutingKey:      localTipHash,
		MaxResponseSize: maxResponseSize,
		// v2: 复用 requester_peer_id 传输 locator（二进制编码），from peer 已由 stream 提供
		RequesterPeerId: locatorBytes,
		TargetHeight:    nil,
	}
	req.ChainIdentity = node.ToProtoChainIdentity(localChainIdentity)

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal hello v2 request failed: %w", err)
	}

	respBytes, err := networkService.Call(ctx, targetPeer, protocols.ProtocolSyncHelloV2, reqBytes, &types.TransportOptions{
		ConnectTimeout: 10 * time.Second,
		WriteTimeout:   10 * time.Second,
		ReadTimeout:    20 * time.Second,
		MaxRetries:     1,
		RetryDelay:     500 * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("hello v2 call failed: %w", err)
	}

	resp := &protocol.IntelligentPaginationResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal hello v2 response failed: %w", err)
	}
	if !resp.Success {
		msg := ""
		if resp.ErrorMessage != nil {
			msg = *resp.ErrorMessage
		}
		return nil, fmt.Errorf("hello v2 rejected: %s", msg)
	}

	// ✅ v2 硬门槛：响应必须回传 chain_identity，且必须与本地一致；否则视为"不兼容 peer"
	if resp.ChainIdentity == nil {
		MarkBadPeer(targetPeer)
		recordSyncFailure(targetPeer, "hello", FailureReasonChainIdentityMismatch, 
			"hello v2 missing chain_identity in response (incompatible peer)", logger)
		recordUpstreamFailure(targetPeer, logger)
		return nil, fmt.Errorf("hello v2 missing chain_identity in response (incompatible peer)")
	}
	remoteIdentity := node.FromProtoChainIdentity(resp.ChainIdentity)
	if !remoteIdentity.IsValid() || !localChainIdentity.IsSameChain(remoteIdentity) {
		if logger != nil {
			logger.Warnf("policy.reject_sync_peer: SyncHelloV2 响应链身份不匹配, peer=%s remote=%v local=%v",
				targetPeer.String()[:8], remoteIdentity, localChainIdentity)
		}
		MarkBadPeer(targetPeer)
		recordSyncFailure(targetPeer, "hello", FailureReasonChainIdentityMismatch,
			fmt.Sprintf("hello v2 incompatible peer: remote=%v local=%v", remoteIdentity, localChainIdentity), logger)
		recordUpstreamFailure(targetPeer, logger)
		return nil, fmt.Errorf("hello v2 incompatible peer: remote=%v local=%v", remoteIdentity, localChainIdentity)
	}

	// ✅ 系统路径缓存：将对端 chain identity 记入 peerstore，供 K桶等本地快路径复用（避免依赖 UserAgent）
	cachePeerChainIdentity(p2pService, targetPeer, remoteIdentity)

	// hello 成功：刷新上游记忆并清零失败计数（用于抗抖动/快速切换）
	recordUpstreamSuccess(targetPeer)

	info := parseHelloV2Reason(resp.PaginationReason)
	if info.remoteTipHeight == 0 {
		info.remoteTipHeight = resp.NextHeight
	}
	// 给调用方用指针
	return &info, nil
}

// cachePeerChainIdentity caches a peer's ChainIdentity into the local peerstore.
//
// 约束：
// - 仅写本地 peerstore（不触发网络 I/O，不 DialPeer）；
// - 失败时静默（不影响同步主流程）。
func cachePeerChainIdentity(p2pService p2pi.Service, pid peer.ID, identity types.ChainIdentity) {
	if p2pService == nil {
		return
	}
	h := p2pService.Host()
	if h == nil {
		return
	}
	if !identity.IsValid() {
		return
	}
	b, err := json.Marshal(identity)
	if err != nil {
		return
	}
	_ = h.Peerstore().Put(pid, constants.PeerstoreKeyChainIdentity, string(b))
}

// 内存优化相关常量
const (
	MAX_BLOCK_BATCH_SIZE    = 20                     // 减小批次大小，避免内存压力
	BATCH_PROCESS_DELAY     = 200 * time.Millisecond // 增加批次间延迟，让GC有时间工作
	MEMORY_GC_THRESHOLD     = 200 * 1024 * 1024      // 降低内存GC阈值，200MB
	MEMORY_CHECK_INTERVAL   = 10                     // 更频繁的内存检查，每10个区块
	MAX_RESPONSE_SIZE_LIMIT = 2 * 1024 * 1024        // 限制单次响应大小，2MB
	FORCE_GC_INTERVAL       = 100                    // 每100个区块强制GC一次
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
	p2pService p2pi.Service,
	configProvider config.Provider,
	logger log.Logger,
) (initialBlocks []*core.Block, err error) {
	if logger != nil {
		logger.Debugf("📡 向节点 %s 发起K桶智能同步", targetPeer.String()[:8])
	}

	// 获取本地节点ID
	localNodeID := p2pService.Host().ID()

	// 获取同步配置
	blockchainConfig := configProvider.GetBlockchain()
	var maxResponseSize uint32 = MAX_RESPONSE_SIZE_LIMIT // 使用优化的响应大小限制
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
		// 确保不超过我们的内存优化限制
		if blockchainConfig.Sync.Advanced.MaxResponseSizeBytes < maxResponseSize {
			maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
		}
	}

	// 获取本地链身份
	localChainIdentity, err := GetLocalChainIdentity(ctx, configProvider, nil)
	if err != nil {
		if logger != nil {
			logger.Warnf("获取本地链身份失败，跳过链身份验证: %v", err)
		}
		// 如果无法获取链身份，仍然继续同步（向后兼容）
		localChainIdentity = types.ChainIdentity{}
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

	// 填充链身份（如果可用）
	if localChainIdentity.IsValid() {
		request.ChainIdentity = node.ToProtoChainIdentity(localChainIdentity)
	}

	// 序列化请求
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化k桶同步请求失败: %w", err)
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
		recordUpstreamFailure(targetPeer, logger)
		return nil, fmt.Errorf("k桶智能同步调用失败: %w", err)
	}

	// 解析响应
	var response protocol.IntelligentPaginationResponse
	if err := proto.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("解析k桶同步响应失败: %w", err)
	}

	// 验证响应
	if !response.Success {
		errorMsg := "未知错误"
		if response.ErrorMessage != nil {
			errorMsg = *response.ErrorMessage
		}
		recordUpstreamFailure(targetPeer, logger)
		return nil, fmt.Errorf("k桶同步请求失败: %s", errorMsg)
	}

	if response.RequestId != request.RequestId {
		return nil, fmt.Errorf("响应RequestID不匹配: 期望=%s, 实际=%s",
			request.RequestId, response.RequestId)
	}

	// 校验响应的链身份（如果响应中包含）
	if response.ChainIdentity != nil {
		remoteIdentity := node.FromProtoChainIdentity(response.ChainIdentity)
		if !localChainIdentity.IsSameChain(remoteIdentity) {
			if logger != nil {
				logger.Warnf("policy.reject_sync_peer: 响应链身份不匹配, peer=%s remote=%v local=%v", targetPeer.String()[:8], remoteIdentity, localChainIdentity)
			}
			// 标记该 peer 为 bad-peer（后续不再向其发起 sync）
			MarkBadPeer(targetPeer)
			recordUpstreamFailure(targetPeer, logger)
			return nil, fmt.Errorf("响应链身份不匹配: remote=%v local=%v", remoteIdentity, localChainIdentity)
		}
		if logger != nil {
			logger.Debugf("✅ 响应链身份验证通过: peer=%s identity=%v", targetPeer.String()[:8], remoteIdentity)
		}
	}

	// 使用protobuf统一的区块格式
	blocks := response.Blocks

	if logger != nil {
		logger.Infof("✅ K桶智能同步成功: 接收区块=%d, 数据大小=%d, NextHeight=%d, HasMore=%t",
			len(blocks), response.ActualSize, response.NextHeight, response.HasMore)
	}

	// 同步成功：刷新上游记忆并清零失败计数
	recordUpstreamSuccess(targetPeer)

	// 🚨 **内存优化关键**：如果响应过大，记录警告并建议分页处理
	if response.ActualSize > maxResponseSize/2 {
		if logger != nil {
			logger.Warnf("⚠️ K桶同步响应较大 (%d字节)，建议后续使用分页同步", response.ActualSize)
		}
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
// 3. ✅ P1修复：支持临时存储乱序区块，检测连续性后批量处理
// 4. 逐批次处理和验证区块
func performRangePaginatedSync(
	ctx context.Context,
	sourcePeers []peer.ID, // 支持多个备用节点的故障转移
	currentHeight, targetHeight uint64,
	networkService network.Network,
	p2pService p2pi.Service,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	tempStore storage.TempStore, // ✅ P1修复：临时存储服务（可选）
	configProvider config.Provider,
	logger log.Logger,
) error {
	if len(sourcePeers) == 0 {
		return fmt.Errorf("没有可用的源节点进行分页同步")
	}

	remainingHeight := currentHeight

	// 从配置获取批次大小和故障转移参数
	batchSize := uint64(MAX_BLOCK_BATCH_SIZE) // 使用优化的批次大小
	maxFailuresPerPeer := 3                   // 默认每个节点最多失败3次

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
		blocks, err := fetchBlockRange(ctx, currentPeer, remainingHeight+1, batchEndHeight, networkService, p2pService, configProvider, logger)
		if err != nil {
			failedAttempts++
			// ✅ SYNC-103修复：记录分页同步失败原因（细化分类）
			recordSyncFailure(currentPeer, "paginated", ClassifyError(err), err.Error(), logger)
			// 记录失败：若 currentPeer 恰好是 lastGoodUpstream，将触发"坏上游快速切换"
			recordUpstreamFailure(currentPeer, logger)
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
		recordUpstreamSuccess(currentPeer)
		
		// ✅ 更新诊断信息：记录拉取的区块数和数据源节点
		UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
			d.BlocksFetched += uint64(len(blocks))
			d.CurrentDataSourcePeer = currentPeer.String()
		})

		// ✅ P1修复：如果区块高度不连续，存储到临时存储
		expectedHeight := remainingHeight + 1
		needTempStore := false
		if len(blocks) > 0 && blocks[0] != nil && blocks[0].Header != nil && blocks[0].Header.Height > expectedHeight {
			needTempStore = true
			if logger != nil {
				logger.Debugf("📦 检测到区块高度跳跃: 期望=%d, 实际=%d，存储到临时存储",
					expectedHeight, blocks[0].Header.Height)
			}
		}

		if needTempStore && tempStore != nil {
			// 存储到临时存储
			tempFileIDs, err := storeBlocksInTempStore(ctx, tempStore, blocks, logger)
			if err != nil {
				if logger != nil {
					logger.Warnf("存储区块到临时存储失败: %v，继续处理", err)
				}
				// 继续处理，不阻断同步流程
			} else {
				if logger != nil {
					logger.Debugf("✅ 已将 %d 个区块存储到临时存储", len(tempFileIDs))
				}
				// 查找连续区块并处理
				continuousBlocks, nextMissingHeight, err := findContinuousBlocks(
					ctx, tempStore, expectedHeight, MAX_BLOCK_BATCH_SIZE, logger)
				if err == nil && len(continuousBlocks) > 0 {
					// 处理连续区块
					if err := processBlockBatch(ctx, continuousBlocks, blockValidator, blockProcessor, logger); err != nil {
						if logger != nil {
							logger.Warnf("处理连续区块失败: %v", err)
						}
					} else {
						// 删除已处理的临时区块
						var processedTempIDs []string
						for _, block := range continuousBlocks {
							// 生成临时文件ID（简化实现）
							height := block.Header.Height
							hashPrefix := ""
							if len(block.Header.PreviousHash) >= 8 {
								hashPrefix = hex.EncodeToString(block.Header.PreviousHash[:8])
							} else {
								hashPrefix = fmt.Sprintf("%010d", height)
							}
							tempID := fmt.Sprintf("sync_pending_%010d_%s", height, hashPrefix)
							processedTempIDs = append(processedTempIDs, tempID)
						}
						removeBlocksFromTempStore(ctx, tempStore, processedTempIDs, logger)

						// 更新进度
						processedInBatch := uint64(len(continuousBlocks))
						updateSyncProgress(processedInBatch)
						remainingHeight += processedInBatch
						
						// ✅ 更新诊断信息：记录处理的区块数
						UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
							d.BlocksProcessed += processedInBatch
						})

						if logger != nil {
							logger.Debugf("✅ 处理了 %d 个连续区块，当前高度: %d", processedInBatch, remainingHeight)
						}

						// 如果还有缺失的高度，继续同步
						if nextMissingHeight > 0 {
							remainingHeight = nextMissingHeight - 1
							if logger != nil {
								logger.Debugf("📊 继续同步缺失区块，从高度 %d 开始", nextMissingHeight)
							}
						}

						// 跳过当前批次处理，因为已经处理了连续区块
						continue
					}
				}
			}
		}

		// 处理当前批次的区块（如果没有使用临时存储，或临时存储失败）
		err = processBlockBatch(ctx, blocks, blockValidator, blockProcessor, logger)
		if err != nil {
			return fmt.Errorf("处理区块批次失败: %w", err)
		}

		// 更新进度
		processedInBatch := uint64(len(blocks))
		updateSyncProgress(processedInBatch)
		remainingHeight += processedInBatch
		
		// ✅ 更新诊断信息：记录处理的区块数
		UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
			d.BlocksProcessed += processedInBatch
		})

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
	p2pService p2pi.Service,
	configProvider config.Provider,
	logger log.Logger,
) ([]*core.Block, error) {
	if logger != nil {
		logger.Infof("📥 开始从节点 %s 获取区块范围 [%d, %d] (共%d个区块)",
			sourcePeer.String()[:8], startHeight, endHeight, endHeight-startHeight+1)
	}

	// 获取同步配置
	blockchainConfig := configProvider.GetBlockchain()
	var maxResponseSize uint32 = MAX_RESPONSE_SIZE_LIMIT // 使用优化的响应大小限制
	if blockchainConfig != nil && blockchainConfig.Sync.Advanced.MaxResponseSizeBytes > 0 {
		// 确保不超过我们的内存优化限制
		if blockchainConfig.Sync.Advanced.MaxResponseSizeBytes < maxResponseSize {
			maxResponseSize = blockchainConfig.Sync.Advanced.MaxResponseSizeBytes
		}
	}

	// 1. 构造KBucketSyncRequest（复用为 SyncBlocksV2 范围请求）
	localChainIdentity, err := GetLocalChainIdentity(ctx, configProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("获取本地链身份失败（SyncBlocksV2 必需）: %w", err)
	}
	if !localChainIdentity.IsValid() {
		return nil, fmt.Errorf("本地链身份无效（SyncBlocksV2 必需）: %v", localChainIdentity)
	}
	request := &protocol.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("range-sync-%d-%d", startHeight, time.Now().UnixNano()),
		LocalHeight:     startHeight - 1, // 本地高度为起始高度前一个
		RoutingKey:      nil,
		MaxResponseSize: maxResponseSize,                         // 从配置获取
		RequesterPeerId: []byte(p2pService.Host().ID().String()), // 本地节点ID（请求者）
		TargetHeight:    &endHeight,                              // 目标高度
	}
	request.ChainIdentity = node.ToProtoChainIdentity(localChainIdentity)

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

	// 4. 发送协议请求（SyncBlocksV2）
	responseBytes, err := networkService.Call(
		ctx,
		sourcePeer,
		protocols.ProtocolSyncBlocksV2,
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

	// ✅ v2 硬门槛：响应必须回传 chain_identity，且必须与本地一致；否则视为"不兼容 peer"
	if response.ChainIdentity == nil {
		MarkBadPeer(sourcePeer)
		recordSyncFailure(sourcePeer, "blocks", FailureReasonChainIdentityMismatch,
			"SyncBlocksV2 missing chain_identity in response (incompatible peer)", logger)
		recordUpstreamFailure(sourcePeer, logger)
		return nil, fmt.Errorf("SyncBlocksV2 missing chain_identity in response (incompatible peer)")
	}
	remoteIdentity := node.FromProtoChainIdentity(response.ChainIdentity)
	if !remoteIdentity.IsValid() || !localChainIdentity.IsSameChain(remoteIdentity) {
		if logger != nil {
			logger.Warnf("policy.reject_sync_peer: SyncBlocksV2 响应链身份不匹配, peer=%s remote=%v local=%v",
				sourcePeer.String()[:8], remoteIdentity, localChainIdentity)
		}
		MarkBadPeer(sourcePeer)
		recordSyncFailure(sourcePeer, "blocks", FailureReasonChainIdentityMismatch,
			fmt.Sprintf("SyncBlocksV2 incompatible peer: remote=%v local=%v", remoteIdentity, localChainIdentity), logger)
		recordUpstreamFailure(sourcePeer, logger)
		return nil, fmt.Errorf("SyncBlocksV2 incompatible peer: remote=%v local=%v", remoteIdentity, localChainIdentity)
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

	// 检查第一个区块是否为 nil
	firstBlock := blocks[0]
	if firstBlock == nil {
		return fmt.Errorf("首个区块为 nil")
	}

	// 检查第一个区块头是否为 nil
	if firstBlock.Header == nil {
		return fmt.Errorf("首个区块头为 nil")
	}

	if firstBlock.Header.Height != expectedStartHeight {
		return fmt.Errorf("首个区块高度不匹配: 期望=%d, 实际=%d",
			expectedStartHeight, firstBlock.Header.Height)
	}

	// 检查区块高度连续性
	for i := 1; i < len(blocks); i++ {
		// 检查当前区块和前一个区块是否为 nil
		if blocks[i-1] == nil {
			return fmt.Errorf("区块序列中位置 %d 的区块为 nil", i-1)
		}
		if blocks[i-1].Header == nil {
			return fmt.Errorf("区块序列中位置 %d 的区块头为 nil", i-1)
		}
		if blocks[i] == nil {
			return fmt.Errorf("区块序列中位置 %d 的区块为 nil", i)
		}
		if blocks[i].Header == nil {
			return fmt.Errorf("区块序列中位置 %d 的区块头为 nil", i)
		}

		prevHeight := blocks[i-1].Header.Height
		currentHeight := blocks[i].Header.Height

		if currentHeight != prevHeight+1 {
			return fmt.Errorf("区块高度不连续: 位置%d height=%d, 位置%d height=%d",
				i-1, prevHeight, i, currentHeight)
		}
	}

	if logger != nil {
		// 再次检查最后一个区块是否为 nil
		lastBlock := blocks[len(blocks)-1]
		if lastBlock != nil && lastBlock.Header != nil {
			logger.Debugf("✅ 区块序列验证通过: 高度范围 [%d, %d]",
				blocks[0].Header.Height, lastBlock.Header.Height)
		} else {
			logger.Debugf("✅ 区块序列验证通过: 起始高度=%d", blocks[0].Header.Height)
		}
	}

	return nil
}

// ============================================================================
//                           区块批处理实现
// ============================================================================

// processBlockBatch 处理区块批次（内存优化版本）
//
// 🎯 **区块处理策略**：
// 1. 将大批次分割为小批次（默认50个区块一批）
// 2. 批次间添加延迟，让GC有时间工作
// 3. 定期检查内存使用，必要时触发GC
// 4. 逐个验证区块的有效性
// 5. 验证通过后处理区块（应用状态变更）
// 6. 记录处理结果和错误信息
func processBlockBatch(
	ctx context.Context,
	blocks []*core.Block,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	logger log.Logger,
) error {
	if len(blocks) == 0 {
		return nil // 空批次，直接返回
	}

	if logger != nil {
		logger.Infof("🔨 开始处理区块批次: %d 个区块 (分批处理，每批%d个)",
			len(blocks), MAX_BLOCK_BATCH_SIZE)
	}

	// 分批处理区块
	for i := 0; i < len(blocks); i += MAX_BLOCK_BATCH_SIZE {
		end := i + MAX_BLOCK_BATCH_SIZE
		if end > len(blocks) {
			end = len(blocks)
		}

		batch := blocks[i:end]

		// 处理当前批次
		if err := processBatch(ctx, batch, blockValidator, blockProcessor, logger, i+1, len(blocks)); err != nil {
			return err
		}

		// 批次间延迟，让GC有时间工作
		if end < len(blocks) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("区块处理被取消: %w", ctx.Err())
			case <-time.After(BATCH_PROCESS_DELAY):
				// 继续下一批次
				if logger != nil {
					logger.Debugf("⏳ 批次间延迟完成，继续处理下一批次")
				}
			}
		}
	}

	if logger != nil {
		logger.Infof("✅ 区块批次处理完成: %d 个区块", len(blocks))
	}

	return nil
}

// processBatch 处理单个小批次
func processBatch(ctx context.Context, batch []*core.Block,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	logger log.Logger,
	startIndex, totalBlocks int) error {

	var memStats runtime.MemStats

	for i, block := range batch {
		// 检查取消信号
		select {
		case <-ctx.Done():
			return fmt.Errorf("区块处理被取消: %w", ctx.Err())
		default:
			// 继续处理
		}

		// 验证区块（委托给BlockValidator，避免重复验证逻辑）
		valid, err := blockValidator.ValidateBlock(ctx, block)
		if err != nil {
			return fmt.Errorf("验证区块 %d 失败: %w", block.Header.Height, err)
		}

		if !valid {
			return fmt.Errorf("区块 %d 验证失败：区块无效", block.Header.Height)
		}

		// 处理区块（委托给BlockProcessor）
		err = blockProcessor.ProcessBlock(ctx, block)
		if err != nil {
			// 🆕 2025-12-18: 处理"区块已被其他流程处理"的情况（幂等性）
			// 场景：同步流程和聚合器/挖矿同时写入区块，后者先完成
			if errors.Is(err, persistence.ErrBlockAlreadyProcessed) {
				if logger != nil {
					logger.Infof("⏭️ 区块 %d 已被其他流程处理，跳过（幂等性保护）", block.Header.Height)
				}
				continue // 跳过该区块，继续处理下一个
			}
			return fmt.Errorf("处理区块 %d 失败: %w", block.Header.Height, err)
		}

		// 定期检查内存使用和强制GC
		shouldCheckMemory := (i+1)%MEMORY_CHECK_INTERVAL == 0
		shouldForceGC := (startIndex+i)%FORCE_GC_INTERVAL == 0

		if shouldCheckMemory || shouldForceGC {
			runtime.ReadMemStats(&memStats)
			currentMemMB := memStats.Alloc / 1024 / 1024

			// 如果内存使用超过阈值，强制GC
			if memStats.Alloc > MEMORY_GC_THRESHOLD || shouldForceGC {
				if logger != nil {
					logger.Debugf("🧹 %s 内存使用: %d MB，触发GC",
						map[bool]string{true: "强制", false: "阈值"}[shouldForceGC], currentMemMB)
				}
				runtime.GC()
				runtime.ReadMemStats(&memStats)
				newMemMB := memStats.Alloc / 1024 / 1024
				if logger != nil {
					logger.Debugf("🧹 GC完成，内存使用: %d MB -> %d MB (节省: %d MB)",
						currentMemMB, newMemMB, currentMemMB-newMemMB)
				}
			} else if shouldCheckMemory && logger != nil {
				logger.Debugf("💾 内存检查: %d MB (阈值: %d MB)", currentMemMB, MEMORY_GC_THRESHOLD/1024/1024)
			}
		}

		// 每10个区块记录一次进度
		if logger != nil && (i+1)%10 == 0 {
			currentIndex := startIndex + i
			logger.Debugf("✅ 区块 %d 处理成功 (%d/%d)",
				block.Header.Height, currentIndex, totalBlocks)
		}
	}

	if logger != nil {
		logger.Debugf("✅ 小批次处理完成: %d 个区块", len(batch))
	}

	return nil
}
