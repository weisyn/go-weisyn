package methods

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/api/format"
	"github.com/weisyn/v1/internal/config/node"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	resourcesvciface "github.com/weisyn/v1/pkg/interfaces/resourcesvc"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/zap"
)

// ChainMethods 链信息相关方法
type ChainMethods struct {
	logger              *zap.Logger
	chainQuery          persistence.ChainQuery
	blockQuery          persistence.BlockQuery
	syncService         chain.SystemSyncService
	cfg                 config.Provider
	blockHash           core.BlockHashServiceClient
	resourceViewService resourcesvciface.Service // 资源视图服务（用于统计资源）

	// 统计缓存（用于增量计算 totalTx 和 stateOutputsTotal）
	statsCache struct {
		sync.RWMutex
		lastCountedHeight uint64    // 最后统计的高度
		totalTx           uint64    // 累计交易总数
		stateOutputsTotal uint64    // 累计 StateOutput 总数
		lastUpdatedAt     time.Time // 最后更新时间
	}
}

// NewChainMethods 创建链方法处理器
func NewChainMethods(
	logger *zap.Logger,
	chainQuery persistence.ChainQuery,
	blockQuery persistence.BlockQuery,
	syncService chain.SystemSyncService,
	cfg config.Provider,
	blockHash core.BlockHashServiceClient,
	resourceViewService resourcesvciface.Service, // 资源视图服务（可选）
) *ChainMethods {
	return &ChainMethods{
		logger:              logger,
		chainQuery:          chainQuery,
		blockQuery:          blockQuery,
		syncService:         syncService,
		cfg:                 cfg,
		blockHash:           blockHash,
		resourceViewService: resourceViewService,
	}
}

// NetVersion 返回网络ID（WES配置）
// Method: wes_netVersion
// 返回：十进制字符串格式的网络ID
func (m *ChainMethods) NetVersion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.cfg == nil || m.cfg.GetBlockchain() == nil {
		return nil, NewInternalError("blockchain config not available", nil)
	}
	networkID := m.cfg.GetBlockchain().NetworkID
	return fmt.Sprintf("%d", networkID), nil
}

// ChainID 返回链ID（十六进制，来自WES配置）
// Method: wes_chainId
// 返回：十六进制字符串格式的链ID（0x前缀）
func (m *ChainMethods) ChainID(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.cfg == nil || m.cfg.GetBlockchain() == nil {
		return nil, NewInternalError("blockchain config not available", nil)
	}
	chainID := m.cfg.GetBlockchain().ChainID
	return fmt.Sprintf("0x%x", chainID), nil
}

// SyncStatus 同步状态响应结构
type SyncStatus struct {
	StartingBlock string `json:"startingBlock"` // 起始区块（十六进制）
	CurrentBlock  string `json:"currentBlock"`  // 当前区块（十六进制）
	HighestBlock  string `json:"highestBlock"`  // 最高区块（十六进制）
}

// Syncing 返回同步状态
// Method: wes_syncing
// 返回：false（已同步）或SyncStatus对象（同步中）
func (m *ChainMethods) Syncing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 使用 SystemSyncService.CheckSync 实时查询同步状态，避免依赖已废弃的持久化状态
	if m.syncService == nil {
		return nil, NewInternalError("sync service not available", nil)
	}

	status, err := m.syncService.CheckSync(ctx)
	if err != nil {
		m.logger.Error("Failed to check sync status", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	if status == nil {
		return nil, NewInternalError("sync status is nil", nil)
	}

	// 判定“已同步”条件（对外语义：返回 false 表示节点已与网络高度对齐，可以正常提供服务）：
	// 1. 状态为 Synced，或处于 Syncing 且高度差在允许范围内
	// 2. 网络高度与本地高度差在 0 或 1 之内（容忍轻微延迟）
	var heightLag uint64
	if status.NetworkHeight > status.CurrentHeight {
		heightLag = status.NetworkHeight - status.CurrentHeight
	}
	const maxAllowedLag = uint64(1)

	isSyncedState := status.Status == types.SyncStatusSynced ||
		(status.Status == types.SyncStatusSyncing && heightLag <= maxAllowedLag)

	if isSyncedState && heightLag <= maxAllowedLag {
		// 数据足够新鲜，返回 false（兼容以太坊语义：false 表示已同步）
		return false, nil
	}

	// 否则认为仍在同步中，返回当前进度
	return &SyncStatus{
		StartingBlock: "0x0", // 起始块：暂未精确记录，后续可从同步服务扩展
		CurrentBlock:  fmt.Sprintf("0x%x", status.CurrentHeight),
		HighestBlock:  fmt.Sprintf("0x%x", status.NetworkHeight),
	}, nil
}

// GetSyncStatus 返回完整的系统同步状态（内部 SystemSyncStatus 的 JSON 映射）
// Method: wes_getSyncStatus
// 返回：types.SystemSyncStatus 对象，包含状态、当前高度、网络高度、进度、最后同步时间等信息
func (m *ChainMethods) GetSyncStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.syncService == nil {
		return nil, NewInternalError("sync service not available", nil)
	}

	status, err := m.syncService.CheckSync(ctx)
	if err != nil {
		m.logger.Error("Failed to check sync status", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	if status == nil {
		return nil, NewInternalError("sync status is nil", nil)
	}

	// 直接返回内部结构，依赖其 JSON 标签进行序列化
	return status, nil
}

// GetChainIdentity 返回链身份信息
// Method: wes_getChainIdentity
// 返回：ChainIdentity 对象
func (m *ChainMethods) GetChainIdentity(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.cfg == nil {
		return nil, NewInternalError("config provider not available", nil)
	}

	appConfig := m.cfg.GetAppConfig()
	if appConfig == nil {
		return nil, NewInternalError("app config not available", nil)
	}

	genesisConfig := m.cfg.GetUnifiedGenesisConfig()
	if genesisConfig == nil {
		return nil, NewInternalError("genesis config not available", nil)
	}

	// 计算 genesis hash
	genesisHash, err := node.CalculateGenesisHash(genesisConfig)
	if err != nil {
		m.logger.Error("Failed to calculate genesis hash", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("failed to calculate genesis hash: %v", err), nil)
	}

	// 构建 ChainIdentity
	identity := node.BuildLocalChainIdentity(appConfig, genesisHash)
	if !identity.IsValid() {
		return nil, NewInternalError("invalid chain identity", nil)
	}

	return identity, nil
}

// BlockNumber 返回最新区块高度
// Method: wes_blockNumber
// 返回：十六进制字符串格式的区块高度（如 "0x1234"）
func (m *ChainMethods) BlockNumber(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 获取链信息
	chainInfo, err := m.chainQuery.GetChainInfo(ctx)
	if err != nil {
		m.logger.Error("Failed to get chain info", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	// 返回十六进制格式的区块高度
	return fmt.Sprintf("0x%x", chainInfo.Height), nil
}

// GetBlockHash 返回指定高度的区块哈希
// Method: wes_getBlockHash
// 参数：[height] (十六进制字符串或十进制整数)
// 返回：32字节区块哈希（十六进制字符串）
func (m *ChainMethods) GetBlockHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) < 1 {
		return nil, NewInvalidParamsError("height parameter required", nil)
	}

	// 解析高度参数（可能是字符串或数字）
	var height uint64
	switch v := args[0].(type) {
	case string:
		// 移除0x前缀
		if len(v) > 2 && v[:2] == "0x" {
			v = v[2:]
		}
		_, err := fmt.Sscanf(v, "%x", &height)
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid height format: %v", err), nil)
		}
	case float64:
		height = uint64(v)
	default:
		return nil, NewInvalidParamsError("height must be string or number", nil)
	}

	// 查询指定高度区块
	if m.blockQuery == nil {
		return nil, NewInternalError("block query not available", nil)
	}
	block, err := m.blockQuery.GetBlockByHeight(ctx, height)
	if err != nil || block == nil {
		return nil, NewBlockNotFoundError(height)
	}

	// 通过标准服务计算区块哈希
	if m.blockHash == nil {
		return nil, NewInternalError("block hash service not available", nil)
	}
	resp, err := m.blockHash.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: block})
	if err != nil || resp == nil || len(resp.Hash) == 0 {
		return nil, NewInternalError("failed to compute block hash", nil)
	}
	return format.HashToHex(resp.Hash), nil
}

// NetworkStats 网络统计信息结构
type NetworkStats struct {
	Height              uint64 `json:"height"`              // 当前区块高度
	TotalTx             uint64 `json:"totalTx"`             // 全网累计交易总数
	ResourcesTotal      uint64 `json:"resourcesTotal"`      // 当前活跃资源总数
	ContractsTotal      uint64 `json:"contractsTotal"`      // 合约资源数量
	AIModelsTotal       uint64 `json:"aiModelsTotal"`       // AI 模型资源数量
	RecentBlocksTxTotal uint64 `json:"recentBlocksTxTotal"` // 最近 N 块的交易总数
	RecentBlocksCount   uint64 `json:"recentBlocksCount"`   // 最近统计的区块数量
	AvgTxPerBlockRecent uint64 `json:"avgTxPerBlockRecent"` // 最近 N 块的平均每块交易数
	StateOutputsTotal   uint64 `json:"stateOutputsTotal"`   // 累计 StateOutput 数（可选）
}

// GetNetworkStats 获取网络统计信息
// Method: wes_getNetworkStats
// 参数: [] (无参数)
// 返回: NetworkStats 对象
func (m *ChainMethods) GetNetworkStats(ctx context.Context, params json.RawMessage) (interface{}, error) {
	stats := NetworkStats{}

	// 1. 获取当前区块高度
	chainInfo, err := m.chainQuery.GetChainInfo(ctx)
	if err != nil {
		m.logger.Error("Failed to get chain info", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}
	stats.Height = chainInfo.Height

	// 2. 获取资源统计（通过 ResourceViewService）
	if m.resourceViewService != nil {
		// 获取所有活跃资源
		allResources, _, err := m.resourceViewService.ListResources(ctx, resourcesvciface.ResourceViewFilter{
			Status: stringPtr("ACTIVE"),
		}, resourcesvciface.PageRequest{
			Offset: 0,
			Limit:  10000, // 获取前 10000 个资源用于统计
		})
		if err == nil {
			stats.ResourcesTotal = uint64(len(allResources))

			// 统计合约和 AI 模型
			for _, resource := range allResources {
				if resource.ExecutableType == "CONTRACT" {
					stats.ContractsTotal++
				} else if resource.ExecutableType == "AI_MODEL" || resource.ExecutableType == "AIMODEL" {
					stats.AIModelsTotal++
				}
			}
		} else {
			m.logger.Warn("Failed to get resources for stats", zap.Error(err))
		}
	}

	// 3. 获取最近 N 个区块的交易统计
	const RECENT_BLOCKS_COUNT = 100
	if stats.Height > 0 {
		var recentTxTotal uint64
		var recentBlocksCount uint64
		startHeight := stats.Height
		if startHeight >= RECENT_BLOCKS_COUNT {
			startHeight = startHeight - RECENT_BLOCKS_COUNT + 1
		} else {
			startHeight = 0
		}

		for height := startHeight; height <= stats.Height; height++ {
			block, err := m.blockQuery.GetBlockByHeight(ctx, height)
			if err != nil || block == nil {
				continue
			}
			recentBlocksCount++
			// 获取区块中的交易数量
			if block.Body != nil && block.Body.Transactions != nil {
				recentTxTotal += uint64(len(block.Body.Transactions))
			}
		}

		if recentBlocksCount > 0 {
			stats.RecentBlocksCount = recentBlocksCount
			stats.RecentBlocksTxTotal = recentTxTotal
			stats.AvgTxPerBlockRecent = recentTxTotal / recentBlocksCount
		}
	}

	// 4. 增量统计 totalTx 和 stateOutputsTotal
	m.statsCache.RLock()
	cachedHeight := m.statsCache.lastCountedHeight
	cachedTotalTx := m.statsCache.totalTx
	cachedStateOutputsTotal := m.statsCache.stateOutputsTotal
	m.statsCache.RUnlock()

	// 如果缓存为空或链高度增长，需要更新统计
	if cachedHeight == 0 || stats.Height > cachedHeight {
		m.statsCache.Lock()
		// 双重检查（可能其他 goroutine 已经更新）
		if m.statsCache.lastCountedHeight == 0 || stats.Height > m.statsCache.lastCountedHeight {
			// 计算需要扫描的区块范围
			startHeight := uint64(0)
			if m.statsCache.lastCountedHeight > 0 {
				startHeight = m.statsCache.lastCountedHeight + 1
			}
			endHeight := stats.Height

			// 增量扫描新区块
			var newTxCount uint64
			var newStateOutputsCount uint64
			const maxScanBlocks = 10000 // 单次最大扫描区块数（防止首次调用时扫描过多）
			const maxScanDuration = 5 * time.Second

			scanStartTime := time.Now()
			scannedCount := uint64(0)

			for height := startHeight; height <= endHeight; height++ {
				// 检查是否超时或超过最大扫描数
				if scannedCount >= maxScanBlocks || time.Since(scanStartTime) > maxScanDuration {
					m.logger.Warn("统计扫描达到限制，保留已统计部分",
						zap.Uint64("scanned_blocks", scannedCount),
						zap.Uint64("start_height", startHeight),
						zap.Uint64("current_height", height),
						zap.Uint64("target_height", endHeight),
					)
					// 更新已统计到的高度
					if height > startHeight {
						m.statsCache.lastCountedHeight = height - 1
						m.statsCache.totalTx += newTxCount
						m.statsCache.stateOutputsTotal += newStateOutputsCount
					}
					break
				}

				block, err := m.blockQuery.GetBlockByHeight(ctx, height)
				if err != nil || block == nil {
					// 区块不存在或查询失败，跳过
					continue
				}

				scannedCount++

				// 统计交易数量
				if block.Body != nil && block.Body.Transactions != nil {
					txCount := uint64(len(block.Body.Transactions))
					newTxCount += txCount

					// 统计 StateOutput 数量（遍历所有交易的输出）
					for _, tx := range block.Body.Transactions {
						if tx.Outputs != nil {
							for _, output := range tx.Outputs {
								// 检查是否为 StateOutput（根据输出类型判断）
								if output.GetState() != nil {
									newStateOutputsCount++
								}
							}
						}
					}
				}

				// 更新最后统计高度
				m.statsCache.lastCountedHeight = height
			}

			// 如果完整扫描完成，更新累计值
			if m.statsCache.lastCountedHeight >= endHeight {
				m.statsCache.totalTx += newTxCount
				m.statsCache.stateOutputsTotal += newStateOutputsCount
			} else {
				// 部分扫描，只累加已扫描部分
				m.statsCache.totalTx += newTxCount
				m.statsCache.stateOutputsTotal += newStateOutputsCount
			}

			m.statsCache.lastUpdatedAt = time.Now()

			m.statsCache.Unlock()

			// 重新读取最新值
			m.statsCache.RLock()
			cachedTotalTx = m.statsCache.totalTx
			cachedStateOutputsTotal = m.statsCache.stateOutputsTotal
			m.statsCache.RUnlock()
		} else {
			m.statsCache.Unlock()
		}
	}

	stats.TotalTx = cachedTotalTx
	stats.StateOutputsTotal = cachedStateOutputsTotal

	return stats, nil
}

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}
