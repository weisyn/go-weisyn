// Package resourcesvc 实现资源视图服务
package resourcesvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
	"github.com/weisyn/v1/internal/core/persistence/consistency"
	"github.com/weisyn/v1/internal/core/persistence/query/history"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	resourcesvciface "github.com/weisyn/v1/pkg/interfaces/resourcesvc"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"google.golang.org/protobuf/proto"
)

// Prometheus 指标：用于观测回退路径的调用频率和耗时
var (
	resourcesvcFallbackRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "resourcesvc_fallback_requests_total",
		Help: "Total number of ListResources fallback calls when ResourceUTXO index is empty.",
	})
	resourcesvcFallbackInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "resourcesvc_fallback_inflight",
		Help: "Number of in-flight ListResources fallback calls.",
	})
	resourcesvcFallbackDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "resourcesvc_fallback_duration_seconds",
		Help:    "Duration of ListResources fallback calls.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(
		resourcesvcFallbackRequests,
		resourcesvcFallbackInFlight,
		resourcesvcFallbackDuration,
	)
}

// Service 资源视图服务实现
type Service struct {
	resourceUTXOQuery eutxo.ResourceUTXOQuery
	resourceQuery     persistence.ResourceQuery
	utxoQuery         persistence.UTXOQuery  // ✅ 新增：用于查询 UTXO 获取锁定条件（使用 persistence 的 UTXOQuery）
	txQuery           persistence.TxQuery    // ✅ 新增：用于查询交易和区块时间戳
	blockQuery        persistence.BlockQuery // ✅ 新增：用于通过 blockHash 查询区块
	historyQuery      *history.Service       // ✅ 新增：用于查询资源历史交易
	badgerStore       storage.BadgerStore    // ✅ 新增：用于直接查询区块数据（备用方案）
	logger            log.Logger

	// ResourceView 缓存（按 contentHash 聚合的代码级视图）
	viewCache *resourceViewCache

	// fallback 控制：限制基于链上交易 + UTXO 的回退路径的并发和频率
	fallbackOnce sync.Once
	fallbackSem  chan struct{}

	// 节点启动时间：用于判断是否为新节点，避免启动初期触发回退路径
	startTime time.Time
}

// CheckResourceUTXOConsistency 执行一次 ResourceUTXO 索引健康检查
//
// 设计目标：
// - 复用现有的 ResourceIndexChecker，对 ResourceCode/ResourceInstance 索引做一致性检查
// - 如果发现不一致问题，则将 ResourceUTXO 标记为 Inconsistent，便于上层触发修复流程
// - 由调用方决定是否进一步切换 NodeMode（例如切换到 NodeModeRepairingUTXO）
func (s *Service) CheckResourceUTXOConsistency(ctx context.Context) (*consistency.CheckResult, error) {
	if s.badgerStore == nil {
		return nil, fmt.Errorf("badgerStore 未注入，无法执行 ResourceUTXO 一致性检查")
	}

	checker := consistency.NewResourceIndexChecker(s.badgerStore, s.logger)
	result, err := checker.CheckConsistency(ctx)
	if err != nil {
		return nil, err
	}

	// 根据检查结果更新 ResourceUTXO 的健康状态
	if len(result.Inconsistencies) > 0 || len(result.OrphanedInstances) > 0 || len(result.OrphanedCodes) > 0 {
		runtimectx.SetUTXOHealth(runtimectx.UTXOTypeResource, runtimectx.UTXOHealthInconsistent)
		if s.logger != nil {
			s.logger.Warnf("ResourceUTXO 一致性检查发现问题: inconsistencies=%d, orphanedInstances=%d, orphanedCodes=%d",
				len(result.Inconsistencies), len(result.OrphanedInstances), len(result.OrphanedCodes))
		}
	} else {
		// 无明显不一致，标记为健康
		runtimectx.SetUTXOHealth(runtimectx.UTXOTypeResource, runtimectx.UTXOHealthHealthy)
		if s.logger != nil {
			s.logger.Info("ResourceUTXO 一致性检查通过，索引处于健康状态")
		}
	}

	return result, nil
}

// ResourceRepairStats 描述一次 ResourceUTXO 修复的统计信息
type ResourceRepairStats struct {
	StartHeight       uint64
	EndHeight         uint64
	RepairedBlocks    uint64
	RepairedResources uint64
	FailedBlocks      uint64
}

// RunResourceUTXORepair 基于区块数据重建 Resource 索引和 ResourceUTXO 视图
//
// 参数：
//   - startHeight: 起始高度（包含），0 表示从高度 1 开始
//   - endHeight: 结束高度（包含），0 表示自动使用当前最高高度
//   - dryRun: 为 true 时仅统计和打印日志，不实际写入索引
func (s *Service) RunResourceUTXORepair(ctx context.Context, startHeight, endHeight uint64, dryRun bool) (*ResourceRepairStats, error) {
	if s.blockQuery == nil || s.badgerStore == nil {
		return nil, fmt.Errorf("缺少 blockQuery 或 badgerStore 依赖，无法执行 ResourceUTXO 修复")
	}

	stats := &ResourceRepairStats{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	// 自动确定结束高度
	if endHeight == 0 {
		h, _, err := s.blockQuery.GetHighestBlock(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取最高区块高度失败: %w", err)
		}
		if h == 0 {
			// 链为空或仅有创世块
			return stats, nil
		}
		endHeight = h
		stats.EndHeight = endHeight
	}

	if startHeight == 0 {
		startHeight = 1 // 跳过创世块
		stats.StartHeight = startHeight
	}

	if endHeight < startHeight {
		return nil, fmt.Errorf("结束高度小于起始高度: start=%d, end=%d", startHeight, endHeight)
	}

	if s.logger != nil {
		s.logger.Infof("开始执行 ResourceUTXO 自动修复: [%d, %d], dryRun=%v", startHeight, endHeight, dryRun)
	}

	for h := startHeight; h <= endHeight; h++ {
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Warnf("ResourceUTXO 修复在高度 %d 被取消: %v", h, ctx.Err())
			}
			return stats, ctx.Err()
		default:
		}

		block, err := s.blockQuery.GetBlockByHeight(ctx, h)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("读取区块失败，跳过: height=%d, error=%v", h, err)
			}
			stats.FailedBlocks++
			continue
		}
		if block == nil || block.Body == nil || len(block.Body.Transactions) == 0 {
			continue
		}

		resourceCount := countResourceOutputs(block)
		if resourceCount == 0 {
			continue
		}

		if dryRun {
			if s.logger != nil {
				s.logger.Infof("DRY-RUN: 高度=%d 检测到 %d 个 ResourceOutput，将执行索引重建", h, resourceCount)
			}
			stats.RepairedBlocks++
			stats.RepairedResources += uint64(resourceCount)
			continue
		}

		startTime := time.Now()

		// 在事务中重建本区块的 Resource 索引和 UTXO 视图
		err = s.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
			return rebuildResourceIndicesForBlock(ctx, tx, block, s.logger)
		})
		if err != nil {
			if s.logger != nil {
				s.logger.Errorf("重建 Resource 索引失败: height=%d, error=%v", h, err)
			}
			stats.FailedBlocks++
			continue
		}

		elapsed := time.Since(startTime)
		if s.logger != nil {
			s.logger.Infof("✅ 高度=%d Resource 索引重建完成，资源数=%d，耗时=%s", h, resourceCount, elapsed)
		}
		stats.RepairedBlocks++
		stats.RepairedResources += uint64(resourceCount)
	}

	if s.logger != nil {
		s.logger.Infof("ResourceUTXO 自动修复结束: repairedBlocks=%d, repairedResources=%d, failedBlocks=%d",
			stats.RepairedBlocks, stats.RepairedResources, stats.FailedBlocks)
	}

	return stats, nil
}

// countResourceOutputs 统计区块中的 ResourceOutput 数量
func countResourceOutputs(block *core.Block) int {
	if block == nil || block.Body == nil {
		return 0
	}
	count := 0
	for _, tx := range block.Body.Transactions {
		for _, output := range tx.Outputs {
			if output.GetResource() != nil {
				count++
			}
		}
	}
	return count
}

// rebuildResourceIndicesForBlock 在单个事务中重建一个区块的 Resource 索引和 ResourceUTXO 视图
//
// 实现参考 internal/core/persistence/writer/resource.go 中的 Resource 索引写入逻辑，
// 但简化为仅处理 ResourceOutput 相关的键。
func rebuildResourceIndicesForBlock(
	ctx context.Context,
	tx storage.BadgerTransaction,
	block *core.Block,
	logger log.Logger,
) error {
	if block == nil || block.Body == nil {
		return nil
	}

	blockHash := block.Header.PreviousHash // 对修复逻辑而言，仅用于元数据记录，不参与共识
	blockHeight := block.Header.Height
	blockTimestamp := uint64(block.Header.Timestamp)

	for _, txProto := range block.Body.Transactions {
		if txProto == nil {
			continue
		}

		// 暂时使用简化版哈希计算（与 ResourceUTXOIndexUpdater 相同逻辑）；
		// 在生产环境中应通过 TransactionHashService 统一计算交易哈希。
		txHash := computeTxHashForRepair(txProto)
		if len(txHash) != 32 {
			continue
		}

		for outputIndex, output := range txProto.Outputs {
			resourceOutput := output.GetResource()
			if resourceOutput == nil || resourceOutput.Resource == nil {
				continue
			}

			if err := rebuildSingleResourceOutput(
				ctx,
				tx,
				txHash,
				uint32(outputIndex),
				output,
				resourceOutput,
				blockHash,
				blockHeight,
				blockTimestamp,
			); err != nil {
				if logger != nil {
					logger.Errorf("重建单个 ResourceOutput 索引失败: height=%d, txHash=%x, index=%d, error=%v",
						blockHeight, txHash[:8], outputIndex, err)
				}
				return err
			}
		}
	}

	return nil
}

// rebuildSingleResourceOutput 重建单个 ResourceOutput 的所有索引
func rebuildSingleResourceOutput(
	ctx context.Context,
	tx storage.BadgerTransaction,
	txHash []byte,
	outputIndex uint32,
	output *transaction.TxOutput,
	resourceOutput *transaction.ResourceOutput,
	blockHash []byte,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	if resourceOutput == nil || resourceOutput.Resource == nil {
		return fmt.Errorf("ResourceOutput.resource 不能为空")
	}

	resource := resourceOutput.Resource
	codeHash := resource.ContentHash
	if len(codeHash) != 32 {
		return fmt.Errorf("codeHash 必须是 32 字节，实际: %d", len(codeHash))
	}

	// 1. 构建资源实例和代码标识
	instanceID := eutxo.NewResourceInstanceID(txHash, outputIndex)
	codeID := eutxo.NewResourceCodeID(codeHash)

	// 2. 构建 ResourceUTXORecord（新索引视图）
	record := &eutxo.ResourceUTXORecord{
		InstanceID:        instanceID,
		CodeID:            codeID,
		ContentHash:       codeHash,
		TxId:              txHash,
		OutputIndex:       outputIndex,
		Owner:             output.Owner,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: resourceOutput.CreationTimestamp,
		IsImmutable:       resourceOutput.IsImmutable,
	}

	if resourceOutput.ExpiryTimestamp != nil && *resourceOutput.ExpiryTimestamp > 0 {
		expiry := *resourceOutput.ExpiryTimestamp
		record.ExpiryTimestamp = &expiry
		if blockTimestamp >= expiry {
			record.Status = eutxo.ResourceUTXOStatusExpired
		}
	}

	// 确保向后兼容字段被填充
	record.EnsureBackwardCompatibility()

	// 3. 实例主索引：indices:resource-instance:{instanceID} -> {blockHash, blockHeight, codeID}
	instanceIndexKey := fmt.Sprintf("indices:resource-instance:%s", instanceID.Encode())
	instanceIndexValue := make([]byte, 72) // blockHash(32) + blockHeight(8) + codeID(32)
	copy(instanceIndexValue[0:32], blockHash)
	copy(instanceIndexValue[32:40], encodeUint64(blockHeight))
	copy(instanceIndexValue[40:72], codeID.Bytes())
	if err := tx.Set([]byte(instanceIndexKey), instanceIndexValue); err != nil {
		return fmt.Errorf("存储资源实例索引失败: %w", err)
	}

	// 4. 实例 UTXO 记录：resource:utxo-instance:{instanceID} -> ResourceUTXORecord
	instanceRecordKey := fmt.Sprintf("resource:utxo-instance:%s", instanceID.Encode())
	recordData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化 ResourceUTXORecord 失败: %w", err)
	}
	if err := tx.Set([]byte(instanceRecordKey), recordData); err != nil {
		return fmt.Errorf("存储 ResourceUTXORecord 失败: %w", err)
	}

	// 5. 代码→实例索引：indices:resource-code:{codeID} -> [instanceID1, instanceID2, ...]
	codeIndexKey := fmt.Sprintf("indices:resource-code:%x", codeID.Bytes())
	existingCodeData, _ := tx.Get([]byte(codeIndexKey))
	var instanceList []string
	instanceIDStr := instanceID.Encode()
	if len(existingCodeData) > 0 {
		if err := json.Unmarshal(existingCodeData, &instanceList); err != nil {
			instanceList = []string{instanceIDStr}
		} else {
			found := false
			for _, id := range instanceList {
				if id == instanceIDStr {
					found = true
					break
				}
			}
			if !found {
				instanceList = append(instanceList, instanceIDStr)
			}
		}
	} else {
		instanceList = []string{instanceIDStr}
	}
	codeIndexValue, err := json.Marshal(instanceList)
	if err != nil {
		return fmt.Errorf("序列化代码→实例索引失败: %w", err)
	}
	if err := tx.Set([]byte(codeIndexKey), codeIndexValue); err != nil {
		return fmt.Errorf("存储代码→实例索引失败: %w", err)
	}

	// 6. Owner 索引：index:resource:owner-instance:{owner}:{instanceID} -> instanceID
	if len(output.Owner) > 0 {
		ownerIndexKey := fmt.Sprintf("index:resource:owner-instance:%x:%s", output.Owner, instanceIDStr)
		if err := tx.Set([]byte(ownerIndexKey), []byte(instanceIDStr)); err != nil {
			return fmt.Errorf("更新 owner 索引失败: %w", err)
		}
	}

	// 7. 使用计数：resource:counters-instance:{instanceID} -> ResourceUsageCounters
	countersKey := fmt.Sprintf("resource:counters-instance:%s", instanceIDStr)
	counters := &eutxo.ResourceUsageCounters{
		InstanceID:               instanceID,
		CodeID:                   codeID,
		CurrentReferenceCount:    0,
		TotalReferenceTimes:      0,
		LastReferenceBlockHeight: blockHeight,
		LastReferenceTimestamp:   blockTimestamp,
	}
	// 确保向后兼容字段被填充
	counters.EnsureBackwardCompatibility()

	countersData, err := json.Marshal(counters)
	if err != nil {
		return fmt.Errorf("序列化 ResourceUsageCounters 失败: %w", err)
	}
	if err := tx.Set([]byte(countersKey), countersData); err != nil {
		return fmt.Errorf("存储 ResourceUsageCounters 失败: %w", err)
	}

	return nil
}

// encodeUint64 将 uint64 编码为大端字节数组（8 字节）
func encodeUint64(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}

// computeTxHashForRepair 计算交易哈希（修复流程中的简化版）
//
// ⚠️ 注意：仅用于离线修复索引，不参与共识；生产环境中应通过 TransactionHashService 计算。
func computeTxHashForRepair(tx *transaction.Transaction) []byte {
	if tx == nil {
		return make([]byte, 32)
	}
	data, err := proto.Marshal(tx)
	if err != nil {
		return make([]byte, 32)
	}
	sum := sha256.Sum256(data)
	return sum[:]
}

// ensureFallbackLimiter 初始化回退路径的并发限制器
func (s *Service) ensureFallbackLimiter() {
	s.fallbackOnce.Do(func() {
		// 默认最大并发构建 ResourceView 的数量，避免读盘风暴
		const defaultMaxConcurrentFallback = 4
		s.fallbackSem = make(chan struct{}, defaultMaxConcurrentFallback)
	})
}

// listResourcesWithFallback 使用链上交易 + UTXO 回退路径构建资源视图，并做限流保护
func (s *Service) listResourcesWithFallback(ctx context.Context, filter ResourceViewFilter, page PageRequest) ([]*ResourceView, PageResponse, error) {
	s.ensureFallbackLimiter()

	// 标记 ResourceUTXO 处于降级状态，便于全局健康视图与运维感知
	runtimectx.SetUTXOHealth(runtimectx.UTXOTypeResource, runtimectx.UTXOHealthDegraded)

	// 如果节点当前处于 UTXO 修复模式，避免再走昂贵的回退路径，直接降级返回
	if runtimectx.GetNodeMode() == runtimectx.NodeModeRepairingUTXO &&
		runtimectx.GetUTXOHealth(runtimectx.UTXOTypeResource) == runtimectx.UTXOHealthInconsistent {
		if s.logger != nil {
			s.logger.Warn("节点处于 UTXO 修复模式且 ResourceUTXO 状态不一致，拒绝回退路径查询以保护系统")
		}
		return nil, PageResponse{}, fmt.Errorf("资源索引正在修复，请稍后再试")
	}

	// 统计回退调用频率与耗时
	resourcesvcFallbackRequests.Inc()
	resourcesvcFallbackInFlight.Inc()
	timer := prometheus.NewTimer(resourcesvcFallbackDuration)
	defer func() {
		resourcesvcFallbackInFlight.Dec()
		timer.ObserveDuration()
	}()

	if s.logger != nil {
		s.logger.Info("🔍 ResourceUTXO 索引为空，使用链上交易 + UTXO 回退路径构建资源视图（已启用限流保护）")
	}

	// 防御性：限制单次请求的最大 limit，避免一次性构建过多 ResourceView
	const maxFallbackPageSize = 200
	if page.Limit <= 0 || page.Limit > maxFallbackPageSize {
		if s.logger != nil {
			s.logger.Warnf("ListResources 回退路径请求的 limit=%d 超出安全范围，调整为 %d", page.Limit, maxFallbackPageSize)
		}
		page.Limit = maxFallbackPageSize
	}

	// 根据 IO 压力等级进一步收紧单批次大小
	switch metricsutil.GetIOPressureLevel() {
	case metricsutil.IOPressureWarning:
		const pageSizeWarning = 50
		if page.Limit > pageSizeWarning {
			if s.logger != nil {
				s.logger.Warnf("IO Warning 模式下收紧回退查询 page.limit: %d -> %d", page.Limit, pageSizeWarning)
			}
			page.Limit = pageSizeWarning
		}
	case metricsutil.IOPressureCritical:
		const pageSizeCritical = 20
		if page.Limit > pageSizeCritical {
			if s.logger != nil {
				s.logger.Warnf("IO Critical 模式下收紧回退查询 page.limit: %d -> %d", page.Limit, pageSizeCritical)
			}
			page.Limit = pageSizeCritical
		}
	}

	// 使用 ResourceQuery 列出资源 contentHash 列表
	hashes, err := s.resourceQuery.ListResourceHashes(ctx, page.Offset, page.Limit)
	if err != nil {
		return nil, PageResponse{}, fmt.Errorf("查询资源哈希列表失败: %w", err)
	}

	views := make([]*ResourceView, 0, len(hashes))
	for _, h := range hashes {
		// 进入限流信号量，防止过多并发构建
		select {
		case s.fallbackSem <- struct{}{}:
			// 正常获取到令牌
		case <-ctx.Done():
			return nil, PageResponse{}, ctx.Err()
		}

		start := time.Now()
		view, err := s.buildResourceViewFromChain(ctx, h)

		// 释放令牌
		<-s.fallbackSem

		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("基于链上数据构建 ResourceView 失败: contentHash=%x, error=%v, elapsed=%s", h, err, time.Since(start))
			}
			continue
		}

		// 应用过滤条件（Owner、category、executableType、status）
		if len(filter.Owner) > 0 && !bytes.Equal(view.Owner, filter.Owner) {
			continue
		}
		if filter.Category != nil && view.Category != *filter.Category {
			continue
		}
		if filter.ExecutableType != nil && view.ExecutableType != *filter.ExecutableType {
			continue
		}
		if filter.Status != nil && view.Status != *filter.Status {
			continue
		}

		views = append(views, view)
	}

	return views, PageResponse{
		Total:  len(views),
		Offset: page.Offset,
		Limit:  page.Limit,
	}, nil
}

// getCachedResourceView 从本地缓存中按 contentHash 查询 ResourceView
func (s *Service) getCachedResourceView(contentHash []byte) (*ResourceView, bool) {
	if s.viewCache == nil || len(contentHash) == 0 {
		return nil, false
	}
	key := hex.EncodeToString(contentHash)
	return s.viewCache.Get(key)
}

// cacheResourceView 将 ResourceView 写入本地缓存（按 ContentHash 聚合）
func (s *Service) cacheResourceView(view *ResourceView) {
	if s.viewCache == nil || view == nil || len(view.ContentHash) == 0 {
		return
	}
	key := hex.EncodeToString(view.ContentHash)
	s.viewCache.Put(key, view)
}

// ShrinkCache 主动裁剪 ResourceView 缓存到目标大小（供 MemoryDoctor 调用）
func (s *Service) ShrinkCache(targetSize int) {
	if s.viewCache == nil {
		return
	}
	if targetSize <= 0 {
		targetSize = 1
	}
	if s.logger != nil {
		s.logger.Warnf("MemoryDoctor 触发 ResourceView 缓存收缩: targetSize=%d (current=%d)",
			targetSize, s.viewCache.Size())
	}
	s.viewCache.Shrink(targetSize)
}

// NewService 创建资源视图服务
func NewService(
	resourceUTXOQuery eutxo.ResourceUTXOQuery,
	resourceQuery persistence.ResourceQuery,
	utxoQuery persistence.UTXOQuery, // ✅ 新增：UTXOQuery 依赖（使用 persistence 的 UTXOQuery）
	txQuery persistence.TxQuery, // ✅ 新增：TxQuery 依赖
	blockQuery persistence.BlockQuery, // ✅ 新增：BlockQuery 依赖（用于通过 blockHash 查询区块）
	badgerStore storage.BadgerStore, // ✅ 新增：用于创建历史查询服务
	logger log.Logger,
) (resourcesvciface.Service, error) {
	if resourceUTXOQuery == nil {
		return nil, fmt.Errorf("resourceUTXOQuery 不能为空")
	}
	if resourceQuery == nil {
		return nil, fmt.Errorf("resourceQuery 不能为空")
	}
	if utxoQuery == nil {
		return nil, fmt.Errorf("utxoQuery 不能为空")
	}
	if txQuery == nil {
		return nil, fmt.Errorf("txQuery 不能为空")
	}
	if blockQuery == nil {
		return nil, fmt.Errorf("blockQuery 不能为空")
	}
	if badgerStore == nil {
		return nil, fmt.Errorf("badgerStore 不能为空")
	}

	// 创建历史查询服务
	historyQuery, err := history.NewService(badgerStore, logger)
	if err != nil {
		return nil, fmt.Errorf("创建历史查询服务失败: %w", err)
	}

	s := &Service{
		resourceUTXOQuery: resourceUTXOQuery,
		resourceQuery:     resourceQuery,
		utxoQuery:         utxoQuery,    // ✅ 新增
		txQuery:           txQuery,      // ✅ 新增
		blockQuery:        blockQuery,   // ✅ 新增
		historyQuery:      historyQuery, // ✅ 新增
		badgerStore:       badgerStore,  // ✅ 新增：保存 badgerStore 用于备用查询
		logger:            logger,
		// 默认缓存 1000 条资源视图（代码级别），避免重复读盘
		viewCache: newResourceViewCache(1000),
		startTime: time.Now(), // 记录启动时间，用于判断是否为新节点
	}

	if logger != nil {
		logger.Info("✅ ResourceViewService 已创建（包含锁定条件和历史查询支持）")
	}

	return s, nil
}

// getDeployTimestamp 查询部署区块时间戳（统一的时间戳查询逻辑）
//
// 优先使用 blockHash 直接查询区块，失败时回退到通过 blockHeight 查询
func (s *Service) getDeployTimestamp(ctx context.Context, blockHash []byte, blockHeight uint64) uint64 {
	if len(blockHash) > 0 {
		// 优先通过 blockHash 查询区块获取时间戳
		block, err := s.blockQuery.GetBlockByHash(ctx, blockHash)
		if err == nil && block != nil && block.Header != nil {
			return uint64(block.Header.Timestamp)
		}
	}
	// 如果通过 blockHash 查询失败，回退到通过高度查询
	if blockHeight > 0 {
		timestamp, err := s.txQuery.GetBlockTimestamp(ctx, blockHeight)
		if err == nil && timestamp > 0 {
			return uint64(timestamp)
		}
	}
	return 0
}

// extractTransactionMetadata 从交易中提取元数据（统一的元数据提取逻辑）
//
// 返回：deployMemo, deployTags, creationContext
func (s *Service) extractTransactionMetadata(ctx context.Context, txHash []byte, contentHash []byte) (string, []string, string) {
	var deployMemo string
	var deployTags []string
	var creationContext string

	if len(txHash) == 0 {
		return deployMemo, deployTags, creationContext
	}

	_, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
	if err != nil || tx == nil {
		return deployMemo, deployTags, creationContext
	}

	// 提取交易元数据
	if tx.Metadata != nil {
		if tx.Metadata.Memo != nil {
			deployMemo = *tx.Metadata.Memo
		}
		deployTags = tx.Metadata.Tags
	}

	// 从 ResourceOutput 中提取创建上下文
	for _, output := range tx.Outputs {
		if output == nil {
			continue
		}
		resourceOutput := output.GetResource()
		if resourceOutput == nil || resourceOutput.Resource == nil {
			continue
		}
		resourceContentHash := resourceOutput.Resource.ContentHash
		if len(resourceContentHash) == 32 && len(contentHash) == 32 {
			if bytes.Equal(resourceContentHash, contentHash) {
				creationContext = resourceOutput.CreationContext
				break
			}
		}
	}

	return deployMemo, deployTags, creationContext
}

// ListResources 列出资源列表
//
// ⚠️ **标识协议对齐**（Phase 2）：
// - 如果 filter.ContentHash 指定，按代码聚合查询（返回该代码的所有实例）
// - 如果 filter.InstanceTxHash + InstanceOutputIndex 指定，精确查询单个实例
// - 否则，按实例列表查询（支持多实例场景）
func (s *Service) ListResources(ctx context.Context, filter ResourceViewFilter, page PageRequest) ([]*ResourceView, PageResponse, error) {
	// 1. 按实例精确查询（如果指定了 InstanceTxHash + InstanceOutputIndex）
	if len(filter.InstanceTxHash) == 32 && filter.InstanceOutputIndex != nil {
		view, err := s.GetResourceByInstance(ctx, filter.InstanceTxHash, *filter.InstanceOutputIndex)
		if err != nil {
			return nil, PageResponse{}, fmt.Errorf("查询资源实例失败: %w", err)
		}
		return []*ResourceView{view}, PageResponse{
			Total:  1,
			Offset: page.Offset,
			Limit:  page.Limit,
		}, nil
	}

	// 2. 按代码聚合查询（如果指定了 ContentHash）
	if len(filter.ContentHash) == 32 {
		instances, err := s.ListResourceInstancesByCode(ctx, filter.ContentHash)
		if err != nil {
			return nil, PageResponse{}, fmt.Errorf("查询资源实例列表失败: %w", err)
		}

		// 应用过滤条件
		filtered := make([]*ResourceView, 0)
		for _, view := range instances {
			if s.matchesViewFilter(view, filter) {
				filtered = append(filtered, view)
			}
		}

		// 如果 GroupByCode=true，每个代码只返回第一个实例
		if filter.GroupByCode && len(filtered) > 0 {
			filtered = filtered[:1]
		}

		// 应用分页
		start := page.Offset
		end := page.Offset + page.Limit
		if start > len(filtered) {
			return []*ResourceView{}, PageResponse{
				Total:  len(filtered),
				Offset: page.Offset,
				Limit:  page.Limit,
			}, nil
		}
		if end > len(filtered) {
			end = len(filtered)
		}

		return filtered[start:end], PageResponse{
			Total:  len(filtered),
			Offset: page.Offset,
			Limit:  page.Limit,
		}, nil
	}

	// 3. 默认：按实例列表查询（原有逻辑）
	// 构建 EUTXO 过滤条件
	eutxoFilter := eutxo.ResourceUTXOFilter{
		Owner: filter.Owner,
	}
	if filter.Status != nil {
		status := eutxo.ResourceUTXOStatus(*filter.Status)
		eutxoFilter.Status = &status
	}

	// 查询资源 UTXO 列表
	records, err := s.resourceUTXOQuery.ListResourceUTXOs(ctx, eutxoFilter, page.Offset, page.Limit)
	if err != nil {
		return nil, PageResponse{}, fmt.Errorf("查询资源 UTXO 列表失败: %w", err)
	}

	// 2.1 兼容性补偿路径：
	// 如果当前链上尚未建立 resource:utxo:* 索引（records 为空），
	// 则退回到基于链上交易 + UTXO 的视角构建 ResourceView。
	// 这里仍然以 UTXO 为真相：通过 TxQuery + UTXOQuery 校验资源是否有活动 UTXO。
	//
	// ⚠️ **新节点启动保护**：
	// - 节点启动后前 5 分钟内，如果索引为空，不立即触发回退路径（避免启动初期大量磁盘I/O）
	// - 返回空结果，让自动健康检查控制器在后台构建索引
	if len(records) == 0 {
		// 检查是否为新节点启动初期（前5分钟）
		startupGracePeriod := 5 * time.Minute
		if time.Since(s.startTime) < startupGracePeriod {
			if s.logger != nil {
				s.logger.Infof("节点启动初期（%v内），ResourceUTXO 索引为空，返回空结果（避免触发回退路径）", startupGracePeriod)
			}
			// 返回空结果，不触发回退路径
			return []*ResourceView{}, PageResponse{
				Total:  0,
				Offset: page.Offset,
				Limit:  page.Limit,
			}, nil
		}
		// 节点已运行超过启动保护期，触发回退路径
		return s.listResourcesWithFallback(ctx, filter, page)
	}

	// 3. 转换为 ResourceView
	views := make([]*ResourceView, 0, len(records))
	for _, record := range records {
		// ✅ 检查 record 是否为 nil
		if record == nil {
			if s.logger != nil {
				s.logger.Warn("ResourceUTXO 记录为 nil，跳过")
			}
			continue
		}
		// ✅ 检查 ContentHash 是否为空
		if len(record.ContentHash) == 0 {
			if s.logger != nil {
				s.logger.Warn("ResourceUTXO ContentHash 为空，跳过")
			}
			continue
		}

		// 查询资源元信息
		resource, err := s.resourceQuery.GetResourceByContentHash(ctx, record.ContentHash)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询资源元信息失败: contentHash=%x, error=%v", record.ContentHash, err)
			}
			continue
		}
		// ✅ 检查 resource 是否为 nil
		if resource == nil {
			if s.logger != nil {
				s.logger.Warnf("资源元信息为 nil: contentHash=%x", record.ContentHash)
			}
			continue
		}

		// ✅ 检查 resource 的关键字段（防止空指针）
		if s.logger != nil {
			s.logger.Debugf("处理资源: contentHash=%x, category=%v, execType=%v",
				record.ContentHash, resource.Category, resource.ExecutableType)
		}

		// 查询使用统计
		counters, _, err := s.resourceUTXOQuery.GetResourceUsageCounters(ctx, record.ContentHash)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询资源使用统计失败: contentHash=%x, error=%v", record.ContentHash, err)
			}
			counters = &eutxo.ResourceUsageCounters{
				ContentHash:              record.ContentHash,
				CurrentReferenceCount:    0,
				TotalReferenceTimes:      0,
				LastReferenceBlockHeight: 0,
				LastReferenceTimestamp:   0,
			}
		}

		// 查询部署交易信息
		txHash, blockHash, blockHeight, err := s.resourceQuery.GetResourceTransaction(ctx, record.ContentHash)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询资源部署交易失败: contentHash=%x, error=%v", record.ContentHash, err)
			}
		}
		// ⚠️ 防御性补丁：资源索引中的 blockHeight 可能因为历史原因为 0。
		// 在高度缺失但 txHash 存在时，从交易索引中补全高度，保证 DeployBlockHeight 对前端可用。
		if blockHeight == 0 && len(txHash) == 32 {
			if h, err := s.txQuery.GetTxBlockHeight(ctx, txHash); err == nil && h > 0 {
				blockHeight = h
			} else if s.logger != nil && err != nil {
				s.logger.Warnf("通过 TxQuery.GetTxBlockHeight 补全区块高度失败: txHash=%x, error=%v", txHash, err)
			}
		}

		// ✅ 查询交易元数据和创建上下文（使用统一的提取方法）
		deployMemo, deployTags, creationContext := s.extractTransactionMetadata(ctx, txHash, record.ContentHash)

		// ✅ 查询区块时间戳（使用统一的查询方法）
		deployTimestamp := s.getDeployTimestamp(ctx, blockHash, blockHeight)

		// ✅ 新增：查询 UTXO 获取锁定条件
		var lockingConditions []*transaction.LockingCondition
		var outPoint *transaction.OutPoint

		// ✅ 严格要求 TxId 必须存在，否则视为索引不完整，跳过该记录
		if len(record.TxId) == 0 {
			if s.logger != nil {
				s.logger.Warnf("ResourceUTXORecord 缺少 TxId，跳过该记录: contentHash=%x", record.ContentHash)
			}
			continue
		}

		outPoint = record.GetOutPoint()
		if outPoint != nil {
			utxoObj, err := s.utxoQuery.GetUTXO(ctx, outPoint)
			if err != nil {
				if s.logger != nil {
					s.logger.Warnf("查询 UTXO 获取锁定条件失败: contentHash=%x, outPoint=%x:%d, error=%v",
						record.ContentHash, outPoint.TxId, outPoint.OutputIndex, err)
				}
				// 锁定条件查询失败不影响其他信息，继续处理
			} else if utxoObj != nil {
				cachedOutput := utxoObj.GetCachedOutput()
				if cachedOutput != nil {
					lockingConditions = cachedOutput.LockingConditions
				}
			}
		}

		// ✅ 新增：提取执行配置
		var executionConfig interface{}
		if resource.ExecutionConfig != nil {
			executionConfig = resource.ExecutionConfig
		}

		// 构建 ResourceView
		// ⚠️ **标识协议对齐**：InstanceOutPoint 作为主键
		view := &ResourceView{
			InstanceOutPoint:      outPoint,           // ✅ Phase 2: 实例标识（ResourceInstanceId，主键）
			ContentHash:           record.ContentHash, // ResourceCodeId（内容维度）
			Category:              mapCategory(resource.Category),
			ExecutableType:        mapExecutableType(resource.ExecutableType),
			MimeType:              resource.MimeType,
			Size:                  resource.Size,
			OutPoint:              outPoint, // 保留字段（向后兼容）
			Owner:                 record.Owner,
			Status:                string(record.Status),
			CreationTimestamp:     record.CreationTimestamp,
			ExpiryTimestamp:       record.ExpiryTimestamp,
			IsImmutable:           record.IsImmutable,
			LockingConditions:     lockingConditions, // ✅ 新增：锁定条件
			CurrentReferenceCount: counters.CurrentReferenceCount,
			TotalReferenceTimes:   counters.TotalReferenceTimes,
			DeployTxId:            txHash,
			DeployBlockHeight:     blockHeight,
			DeployBlockHash:       blockHash,
			DeployTimestamp:       deployTimestamp,           // ✅ 新增：部署时间戳
			ExecutionConfig:       executionConfig,           // ✅ 新增：执行配置
			OriginalFilename:      resource.OriginalFilename, // ✅ 新增：原始文件名
			FileExtension:         resource.FileExtension,    // ✅ 新增：文件扩展名
			CreationContext:       creationContext,           // ✅ 新增：创建上下文
			DeployMemo:            deployMemo,                // ✅ 新增：部署备注
			DeployTags:            deployTags,                // ✅ 新增：部署标签
		}

		// 应用过滤条件（category、executableType、tags）
		if filter.Category != nil && view.Category != *filter.Category {
			continue
		}
		if filter.ExecutableType != nil && view.ExecutableType != *filter.ExecutableType {
			continue
		}

		views = append(views, view)
	}

	// 4. 返回结果
	return views, PageResponse{
		Total:  len(views),
		Offset: page.Offset,
		Limit:  page.Limit,
	}, nil
}

// GetResource 获取单个资源（基于 ResourceCodeId）
//
// ⚠️ **标识协议对齐**：
// - 此方法使用 ResourceCodeId（ContentHash）查询
// - 在多实例场景下，此方法返回"第一个找到的实例"（兼容性）
// - 推荐：优先使用 GetResourceByInstance 进行精确查询
// - 如需列出所有实例，使用 ListResourceInstancesByCode
func (s *Service) GetResource(ctx context.Context, contentHash []byte) (*ResourceView, error) {
	// 优先从本地缓存获取
	if view, ok := s.getCachedResourceView(contentHash); ok {
		return view, nil
	}
	// 1. 尝试查询资源实例列表（新索引）
	instances, err := s.resourceUTXOQuery.ListResourceInstancesByCode(ctx, contentHash)
	if err == nil && len(instances) > 0 {
		// 多实例场景：返回第一个实例（兼容性）
		if len(instances) > 1 && s.logger != nil {
			s.logger.Warnf("GetResource: 发现多个实例（%d个），返回第一个: contentHash=%x", len(instances), contentHash)
		}
		firstInstance := instances[0]
		return s.GetResourceByInstance(ctx, firstInstance.TxId, firstInstance.OutputIndex)
	}

	// 2. 回退：查询资源 UTXO（旧索引，兼容性）
	record, exists, err := s.resourceUTXOQuery.GetResourceUTXOByContentHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源 UTXO 失败: %w", err)
	}
	if !exists {
		// 兼容性补偿路径：
		// 当前链上可能已经有资源文件和交易，但尚未建立 resource:utxo:* 索引。
		// 此时尝试直接基于链上交易 + UTXO 构建 ResourceView。
		if s.logger != nil {
			s.logger.Warnf("ResourceUTXO 记录不存在，尝试基于链上交易 + UTXO 构建资源视图: contentHash=%x", contentHash)
		}
		return s.buildResourceViewFromChain(ctx, contentHash)
	}

	// 2. 查询资源元信息
	resource, err := s.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源元信息失败: %w", err)
	}

	// 3. 查询使用统计
	counters, _, err := s.resourceUTXOQuery.GetResourceUsageCounters(ctx, contentHash)
	if err != nil {
		counters = &eutxo.ResourceUsageCounters{
			ContentHash:              contentHash,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
	}

	// 4. 查询部署交易信息
	txHash, blockHash, blockHeight, err := s.resourceQuery.GetResourceTransaction(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源部署交易失败: %w", err)
	}
	// ⚠️ 防御性补丁：当资源索引中的 blockHeight 为 0，但 txHash 存在时，
	// 通过 TxQuery 直接从交易索引补全高度，避免前端看到“未知区块高度”。
	if blockHeight == 0 && len(txHash) == 32 {
		if h, err := s.txQuery.GetTxBlockHeight(ctx, txHash); err == nil && h > 0 {
			blockHeight = h
		} else if s.logger != nil && err != nil {
			s.logger.Warnf("GetResource: 通过 TxQuery.GetTxBlockHeight 补全区块高度失败: txHash=%x, error=%v", txHash, err)
		}
	}

	// ✅ 新增：查询 UTXO 获取锁定条件
	var lockingConditions []*transaction.LockingCondition
	outPoint := record.GetOutPoint()
	if outPoint != nil {
		utxoObj, err := s.utxoQuery.GetUTXO(ctx, outPoint)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询 UTXO 获取锁定条件失败: contentHash=%x, outPoint=%x:%d, error=%v",
					contentHash, outPoint.TxId, outPoint.OutputIndex, err)
			}
			// 锁定条件查询失败不影响其他信息，继续处理
		} else {
			cachedOutput := utxoObj.GetCachedOutput()
			if cachedOutput != nil {
				lockingConditions = cachedOutput.LockingConditions
			}
		}
	}

	// ✅ 查询交易元数据和创建上下文（使用统一的提取方法）
	deployMemo, deployTags, creationContext := s.extractTransactionMetadata(ctx, txHash, contentHash)

	// ✅ 查询区块时间戳（使用统一的查询方法）
	deployTimestamp := s.getDeployTimestamp(ctx, blockHash, blockHeight)

	// ✅ 新增：提取执行配置
	var executionConfig interface{}
	if resource.ExecutionConfig != nil {
		executionConfig = resource.ExecutionConfig
		// 🔍 调试日志：检查 ExecutionConfig 提取
		if contract, ok := resource.ExecutionConfig.(*pbresource.Resource_Contract); ok && contract.Contract != nil {
			if s.logger != nil {
				s.logger.Infof("🔍 [DEBUG] GetResource: 提取 ExecutionConfig 成功 (abi_version=%s, functions=%d)",
					contract.Contract.AbiVersion, len(contract.Contract.ExportedFunctions))
			}
		} else {
			if s.logger != nil {
				s.logger.Warnf("🔍 [DEBUG] GetResource: ExecutionConfig 类型不匹配或为空 (contentHash=%x)", contentHash)
			}
		}
	} else {
		if s.logger != nil {
			s.logger.Warnf("🔍 [DEBUG] GetResource: resource.ExecutionConfig 为 nil (contentHash=%x)", contentHash)
		}
	}

	// 5. 构建 ResourceView
	// ⚠️ **标识协议对齐**：InstanceOutPoint 作为主键，ContentHash 作为代码标识
	view := &ResourceView{
		InstanceOutPoint:      outPoint,    // ✅ Phase 2: 实例标识（ResourceInstanceId，主键）
		ContentHash:           contentHash, // ResourceCodeId（内容维度）
		Category:              mapCategory(resource.Category),
		ExecutableType:        mapExecutableType(resource.ExecutableType),
		MimeType:              resource.MimeType,
		Size:                  resource.Size,
		OutPoint:              outPoint, // 保留字段（向后兼容）
		Owner:                 record.Owner,
		Status:                string(record.Status),
		CreationTimestamp:     record.CreationTimestamp,
		ExpiryTimestamp:       record.ExpiryTimestamp,
		IsImmutable:           record.IsImmutable,
		LockingConditions:     lockingConditions, // ✅ 新增：锁定条件
		CurrentReferenceCount: counters.CurrentReferenceCount,
		TotalReferenceTimes:   counters.TotalReferenceTimes,
		DeployTxId:            txHash,
		DeployBlockHeight:     blockHeight,
		DeployBlockHash:       blockHash,
		DeployTimestamp:       deployTimestamp,           // ✅ 新增：部署时间戳
		ExecutionConfig:       executionConfig,           // ✅ 新增：执行配置
		OriginalFilename:      resource.OriginalFilename, // ✅ 新增：原始文件名
		FileExtension:         resource.FileExtension,    // ✅ 新增：文件扩展名
		CreationContext:       creationContext,           // ✅ 新增：创建上下文
		DeployMemo:            deployMemo,                // ✅ 新增：部署备注
		DeployTags:            deployTags,                // ✅ 新增：部署标签
	}

	// 缓存结果（按 ContentHash 聚合）
	s.cacheResourceView(view)

	return view, nil
}

// GetResourceByInstance 根据资源实例标识获取资源视图
//
// ⚠️ **标识协议对齐**：使用 ResourceInstanceId（OutPoint）作为主键
// 此方法支持多实例场景下的精确查询，推荐优先使用
func (s *Service) GetResourceByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*ResourceView, error) {
	// 1. 验证参数
	if len(txHash) != 32 {
		return nil, fmt.Errorf("txHash 必须是 32 字节，实际: %d", len(txHash))
	}

	// 2. 查询资源 UTXO（使用实例索引）
	record, exists, err := s.resourceUTXOQuery.GetResourceUTXOByInstance(ctx, txHash, outputIndex)
	if err != nil {
		return nil, fmt.Errorf("查询资源 UTXO（实例）失败: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("资源实例不存在: txHash=%x, outputIndex=%d", txHash, outputIndex)
	}

	// 3. 查询资源元信息（基于 ContentHash）
	resource, err := s.resourceQuery.GetResourceByContentHash(ctx, record.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源元信息失败: %w", err)
	}

	// 4. 查询使用统计（使用实例索引）
	counters, _, err := s.resourceUTXOQuery.GetResourceUsageCountersByInstance(ctx, txHash, outputIndex)
	if err != nil {
		// 统计查询失败不影响主流程，使用默认值
		counters = &eutxo.ResourceUsageCounters{
			InstanceTxId:             txHash,
			InstanceIndex:            outputIndex,
			ContentHash:              record.ContentHash,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
	}

	// 5. 查询部署交易信息
	// 优先从实例索引获取，失败时回退到代码索引（兼容性）
	var blockHash []byte
	var blockHeight uint64

	// 尝试从实例索引获取（新索引）
	instanceID := eutxo.EncodeInstanceID(txHash, outputIndex)
	instanceIndexKey := fmt.Sprintf("indices:resource-instance:%s", instanceID)
	if s.badgerStore != nil {
		instanceIndexData, err := s.badgerStore.Get(ctx, []byte(instanceIndexKey))
		if err == nil && len(instanceIndexData) >= 72 {
			blockHash = instanceIndexData[0:32]
			blockHeight = bytesToUint64(instanceIndexData[32:40])
		}
	}

	// 如果实例索引未找到，回退到代码索引（兼容性）
	if len(blockHash) == 0 {
		txHashFromCode, blockHashFromCode, blockHeightFromCode, err2 := s.resourceQuery.GetResourceTransaction(ctx, record.ContentHash)
		if err2 == nil && len(txHashFromCode) == 32 {
			blockHash = blockHashFromCode
			blockHeight = blockHeightFromCode
		}
	}

	// 如果仍然没有，尝试从交易查询区块信息
	if len(blockHash) == 0 {
		if h, err := s.txQuery.GetTxBlockHeight(ctx, txHash); err == nil && h > 0 {
			blockHeight = h
		}
	}

	// 6. 查询 UTXO 获取锁定条件
	var lockingConditions []*transaction.LockingCondition
	outPoint := record.GetOutPoint()
	if outPoint != nil {
		utxoObj, err := s.utxoQuery.GetUTXO(ctx, outPoint)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询 UTXO 获取锁定条件失败: instanceID=%s, outPoint=%x:%d, error=%v",
					instanceID, outPoint.TxId, outPoint.OutputIndex, err)
			}
		} else {
			cachedOutput := utxoObj.GetCachedOutput()
			if cachedOutput != nil {
				lockingConditions = cachedOutput.LockingConditions
			}
		}
	}

	// 7. 查询交易元数据和创建上下文
	deployMemo, deployTags, creationContext := s.extractTransactionMetadata(ctx, txHash, record.ContentHash)

	// 8. 查询区块时间戳
	deployTimestamp := s.getDeployTimestamp(ctx, blockHash, blockHeight)

	// 9. 提取执行配置
	var executionConfig interface{}
	if resource.ExecutionConfig != nil {
		executionConfig = resource.ExecutionConfig
	}

	// 10. 构建 ResourceView（基于实例）
	view := &ResourceView{
		InstanceOutPoint:      outPoint,           // ResourceInstanceId（主键）
		ContentHash:           record.ContentHash, // ResourceCodeId
		Category:              mapCategory(resource.Category),
		ExecutableType:        mapExecutableType(resource.ExecutableType),
		MimeType:              resource.MimeType,
		Size:                  resource.Size,
		OutPoint:              outPoint, // 保留字段（向后兼容）
		Owner:                 record.Owner,
		Status:                string(record.Status),
		CreationTimestamp:     record.CreationTimestamp,
		ExpiryTimestamp:       record.ExpiryTimestamp,
		IsImmutable:           record.IsImmutable,
		LockingConditions:     lockingConditions,
		CurrentReferenceCount: counters.CurrentReferenceCount,
		TotalReferenceTimes:   counters.TotalReferenceTimes,
		DeployTxId:            txHash,
		DeployBlockHeight:     blockHeight,
		DeployBlockHash:       blockHash,
		DeployTimestamp:       deployTimestamp,
		ExecutionConfig:       executionConfig,
		OriginalFilename:      resource.OriginalFilename,
		FileExtension:         resource.FileExtension,
		CreationContext:       creationContext,
		DeployMemo:            deployMemo,
		DeployTags:            deployTags,
	}

	return view, nil
}

// ListResourceInstancesByCode 列出指定代码的所有实例
//
// ⚠️ **标识协议对齐**：展示 ResourceCodeId → ResourceInstanceId 的 1:N 关系
func (s *Service) ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*ResourceView, error) {
	// 1. 验证参数
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 2. 查询所有实例记录
	records, err := s.resourceUTXOQuery.ListResourceInstancesByCode(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源实例列表失败: %w", err)
	}

	// 3. 逐个构建 ResourceView
	views := make([]*ResourceView, 0, len(records))
	for _, record := range records {
		// 复用 GetResourceByInstance 的逻辑
		view, err := s.GetResourceByInstance(ctx, record.TxId, record.OutputIndex)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("构建资源实例视图失败: txHash=%x, outputIndex=%d, error=%v",
					record.TxId, record.OutputIndex, err)
			}
			continue
		}
		views = append(views, view)
	}

	return views, nil
}

// buildResourceViewFromChain 基于链上交易 + UTXO 构建 ResourceView。
//
// 🎯 设计原则：
// - 仍然以 UTXO 为真相：必须能从 UTXO 集合中找到对应的 OutPoint，资源才视为“存在且激活”。
// - 不依赖 resource:utxo:* 索引，适合作为索引缺失时的回退路径。
func (s *Service) buildResourceViewFromChain(ctx context.Context, contentHash []byte) (*ResourceView, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 优先从本地缓存获取
	if view, ok := s.getCachedResourceView(contentHash); ok {
		return view, nil
	}

	// 1. 查询资源元信息（Resource 本体）
	resource, err := s.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil || resource == nil {
		return nil, fmt.Errorf("查询资源元信息失败: %w", err)
	}

	// 2. 查询部署交易（获取 txHash / blockHash / blockHeight）
	txHash, blockHash, blockHeight, err := s.resourceQuery.GetResourceTransaction(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源部署交易失败: %w", err)
	}
	// ⚠️ 防御性补丁：资源索引中的 blockHeight 可能为 0（历史数据或索引缺失）。
	// 在这种情况下，如果 txHash 存在，则尝试通过 TxQuery 直接从交易索引补全高度。
	if blockHeight == 0 && len(txHash) == 32 {
		if h, err := s.txQuery.GetTxBlockHeight(ctx, txHash); err == nil && h > 0 {
			blockHeight = h
		} else if s.logger != nil && err != nil {
			s.logger.Warnf("buildResourceViewFromChain: 通过 TxQuery.GetTxBlockHeight 补全区块高度失败: txHash=%x, error=%v", txHash, err)
		}
	}

	// 3. 通过交易查找对应的 ResourceOutput，以获取 Owner / CreationTimestamp 等信息
	var outPoint *transaction.OutPoint
	var owner []byte
	var creationTimestamp uint64
	var expiryTimestamp *uint64
	var isImmutable bool

	if len(txHash) > 0 {
		_, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
		if err != nil || tx == nil {
			return nil, fmt.Errorf("获取部署交易详情失败: %w", err)
		}

		for idx, output := range tx.Outputs {
			if output == nil {
				continue
			}
			resOut := output.GetResource()
			if resOut == nil || resOut.Resource == nil {
				continue
			}
			resContentHash := resOut.Resource.ContentHash
			if len(resContentHash) == 32 && bytes.Equal(resContentHash, contentHash) {
				// 找到对应的 ResourceOutput
				owner = output.Owner
				creationTimestamp = resOut.CreationTimestamp
				if resOut.ExpiryTimestamp != nil && *resOut.ExpiryTimestamp > 0 {
					exp := *resOut.ExpiryTimestamp
					expiryTimestamp = &exp
				}
				isImmutable = resOut.IsImmutable
				outPoint = &transaction.OutPoint{
					TxId:        txHash,
					OutputIndex: uint32(idx),
				}
				break
			}
		}
	}

	if outPoint == nil {
		return nil, fmt.Errorf("在部署交易中未找到匹配的 ResourceOutput: contentHash=%x", contentHash)
	}

	// 4. 通过 UTXO 集合确认资源是否仍然存在，并提取锁定条件
	utxoObj, err := s.utxoQuery.GetUTXO(ctx, outPoint)
	if err != nil || utxoObj == nil {
		return nil, fmt.Errorf("在 UTXO 集合中未找到资源输出: contentHash=%x, outPoint=%x:%d", contentHash, outPoint.TxId, outPoint.OutputIndex)
	}

	var lockingConditions []*transaction.LockingCondition
	if cachedOutput := utxoObj.GetCachedOutput(); cachedOutput != nil {
		lockingConditions = cachedOutput.LockingConditions
	}

	// 5. 查询使用统计（如果有 counters）
	counters, _, err := s.resourceUTXOQuery.GetResourceUsageCounters(ctx, contentHash)
	if err != nil || counters == nil {
		counters = &eutxo.ResourceUsageCounters{
			ContentHash:              contentHash,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
	}

	// 6. 查询交易元数据、创建上下文和部署时间戳
	var deployMemo string
	var deployTags []string
	var creationContext string
	var deployTimestamp uint64

	if len(txHash) > 0 {
		_, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
		if err == nil && tx != nil {
			if tx.Metadata != nil {
				if tx.Metadata.Memo != nil {
					deployMemo = *tx.Metadata.Memo
				}
				deployTags = tx.Metadata.Tags
			}
			for _, output := range tx.Outputs {
				if output == nil {
					continue
				}
				resOut := output.GetResource()
				if resOut != nil && resOut.Resource != nil {
					resContentHash := resOut.Resource.ContentHash
					if len(resContentHash) == 32 && bytes.Equal(resContentHash, contentHash) {
						creationContext = resOut.CreationContext
						break
					}
				}
			}
		}
		if blockHeight > 0 {
			if ts, err := s.txQuery.GetBlockTimestamp(ctx, blockHeight); err == nil && ts > 0 {
				deployTimestamp = uint64(ts)
			}
		}
	}

	// 7. 执行配置 & 文件信息
	var executionConfig interface{}
	if resource.ExecutionConfig != nil {
		executionConfig = resource.ExecutionConfig
	}

	// 8. 构建 ResourceView（状态：只要 UTXO 存在，即视为 ACTIVE）
	// ⚠️ **标识协议对齐**：InstanceOutPoint 作为主键
	view := &ResourceView{
		InstanceOutPoint:      outPoint,    // ✅ Phase 2: 实例标识（ResourceInstanceId，主键）
		ContentHash:           contentHash, // ResourceCodeId（内容维度）
		Category:              mapCategory(resource.Category),
		ExecutableType:        mapExecutableType(resource.ExecutableType),
		MimeType:              resource.MimeType,
		Size:                  resource.Size,
		OutPoint:              outPoint, // 保留字段（向后兼容）
		Owner:                 owner,
		Status:                "ACTIVE",
		CreationTimestamp:     creationTimestamp,
		ExpiryTimestamp:       expiryTimestamp,
		IsImmutable:           isImmutable,
		LockingConditions:     lockingConditions,
		CurrentReferenceCount: counters.CurrentReferenceCount,
		TotalReferenceTimes:   counters.TotalReferenceTimes,
		DeployTxId:            txHash,
		DeployBlockHeight:     blockHeight,
		DeployBlockHash:       blockHash,
		DeployTimestamp:       deployTimestamp,
		ExecutionConfig:       executionConfig,
		OriginalFilename:      resource.OriginalFilename,
		FileExtension:         resource.FileExtension,
		CreationContext:       creationContext,
		DeployMemo:            deployMemo,
		DeployTags:            deployTags,
	}

	// 缓存构建后的视图（按 ContentHash 聚合）
	s.cacheResourceView(view)

	return view, nil
}

// GetResourceHistory 获取资源历史
func (s *Service) GetResourceHistory(ctx context.Context, contentHash []byte, page PageRequest) (*ResourceHistory, error) {
	// 在 IO Critical 模式下，历史查询属于非关键路径，直接降级为“系统繁忙”
	if metricsutil.GetIOPressureLevel() == metricsutil.IOPressureCritical {
		if s.logger != nil {
			s.logger.Warnf("GetResourceHistory 在 IO Critical 模式下被拒绝，以保护系统资源: contentHash=%s",
				hex.EncodeToString(contentHash))
		}
		return nil, fmt.Errorf("系统当前负载较高，请稍后再试")
	}
	// 1. 查询部署交易
	txHash, blockHash, blockHeight, err := s.resourceQuery.GetResourceTransaction(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源部署交易失败: %w", err)
	}

	// ⚠️ 防御性补丁：部署索引中的 blockHeight 可能为 0（历史数据，还未回填高度）。
	// 如果高度缺失但 txHash 存在，则尝试通过 TxQuery 从交易索引补全。
	if blockHeight == 0 && len(txHash) == 32 {
		if h, err := s.txQuery.GetTxBlockHeight(ctx, txHash); err == nil && h > 0 {
			blockHeight = h
		} else if s.logger != nil && err != nil {
			s.logger.Warnf("GetResourceHistory: 通过 TxQuery.GetTxBlockHeight 补全区块高度失败: txHash=%x, error=%v", txHash, err)
		}
	}

	// 2. 查询区块时间戳
	var deployTimestamp uint64
	if blockHeight > 0 {
		timestamp, err := s.txQuery.GetBlockTimestamp(ctx, blockHeight)
		if err == nil && timestamp > 0 {
			deployTimestamp = uint64(timestamp)
		}
	}

	// 3. 构建部署交易摘要
	deployTx := &TxSummary{
		TxId:        txHash,
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
		Timestamp:   deployTimestamp,
	}

	// 4. 查询使用统计
	counters, _, err := s.resourceUTXOQuery.GetResourceUsageCounters(ctx, contentHash)
	if err != nil {
		counters = &eutxo.ResourceUsageCounters{
			ContentHash:              contentHash,
			CurrentReferenceCount:    0,
			TotalReferenceTimes:      0,
			LastReferenceBlockHeight: 0,
			LastReferenceTimestamp:   0,
		}
	}

	// 5. ✅ 新增：查询历史交易（引用和升级）
	historyEntries, err := s.historyQuery.GetResourceHistory(ctx, contentHash, page.Offset, page.Limit)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("查询资源历史交易失败: contentHash=%x, error=%v", contentHash, err)
		}
		// 历史查询失败不影响其他信息，继续处理
		historyEntries = []*history.TxHistoryEntry{}
	}

	// 6. 构建引用和升级交易摘要
	references := make([]*TxSummary, 0)
	upgrades := make([]*TxSummary, 0)

	for _, entry := range historyEntries {
		// 跳过部署交易（已在deployTx中）
		if bytes.Equal(entry.TxHash, txHash) {
			continue
		}

		// 查询交易详情和位置信息
		blockHash, _, tx, err := s.txQuery.GetTransaction(ctx, entry.TxHash)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询历史交易详情失败: txHash=%x, error=%v", entry.TxHash, err)
			}
			continue
		}

		// 查询交易所在区块高度
		blockHeight, err := s.txQuery.GetTxBlockHeight(ctx, entry.TxHash)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("查询历史交易区块高度失败: txHash=%x, error=%v", entry.TxHash, err)
			}
			continue
		}
		_ = blockHash // 暂时不使用

		// 查询区块时间戳
		var timestamp uint64
		if blockHeight > 0 {
			if ts, err := s.txQuery.GetBlockTimestamp(ctx, blockHeight); err == nil && ts > 0 {
				timestamp = uint64(ts)
			}
		}

		// 判断交易类型：升级（消费资源UTXO）还是引用（引用资源UTXO）
		isUpgrade := false
		for _, input := range tx.Inputs {
			if input.PreviousOutput == nil {
				continue
			}
			// 检查是否是消费型输入（is_reference_only=false）
			if !input.IsReferenceOnly {
				// 查询被消费的UTXO是否是资源UTXO
				utxoObj, err := s.utxoQuery.GetUTXO(ctx, input.PreviousOutput)
				if err == nil && utxoObj != nil {
					if utxoObj.Category == 2 { // UTXO_CATEGORY_RESOURCE
						cachedOutput := utxoObj.GetCachedOutput()
						if cachedOutput != nil {
							resourceOutput := cachedOutput.GetResource()
							if resourceOutput != nil && resourceOutput.Resource != nil {
								if bytes.Equal(resourceOutput.Resource.ContentHash, contentHash) {
									isUpgrade = true
									break
								}
							}
						}
					}
				}
			}
		}

		txSummary := &TxSummary{
			TxId:        entry.TxHash,
			BlockHeight: blockHeight,
			Timestamp:   timestamp,
		}

		if isUpgrade {
			upgrades = append(upgrades, txSummary)
		} else {
			references = append(references, txSummary)
		}
	}

	// 7. 构建引用统计摘要
	referencesSummary := &ReferenceSummary{
		TotalReferences:   counters.TotalReferenceTimes,
		UniqueCallers:     uint64(len(references)), // ✅ 使用实际引用交易数量
		LastReferenceTime: counters.LastReferenceTimestamp,
	}

	// 8. 构建历史记录
	history := &ResourceHistory{
		DeployTx:          deployTx,
		Upgrades:          upgrades,   // ✅ 使用实际查询结果
		References:        references, // ✅ 新增：引用交易列表
		ReferencesSummary: referencesSummary,
	}

	return history, nil
}

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (s *Service) ModuleName() string {
	return "resourcesvc.view"
}

// CollectMemoryStats 收集资源视图服务的内存统计信息（实现 MemoryReporter 接口）
func (s *Service) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	var cacheItems int64
	if s.viewCache != nil {
		cacheItems = int64(s.viewCache.Size())
	}

	return metricsiface.ModuleMemoryStats{
		Module:      "resourcesvc.view",
		Layer:       "L4-CoreBusiness",
		Objects:     0,
		ApproxBytes: 0,
		CacheItems:  cacheItems,
		QueueLength: 0,
	}
}

// mapCategory 映射资源类别
func mapCategory(category pbresource.ResourceCategory) string {
	switch category {
	case pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE:
		return "EXECUTABLE"
	case pbresource.ResourceCategory_RESOURCE_CATEGORY_STATIC:
		return "STATIC"
	default:
		return "UNKNOWN"
	}
}

// mapExecutableType 映射可执行类型
func mapExecutableType(execType pbresource.ExecutableType) string {
	switch execType {
	case pbresource.ExecutableType_EXECUTABLE_TYPE_CONTRACT:
		return "CONTRACT"
	case pbresource.ExecutableType_EXECUTABLE_TYPE_AIMODEL:
		return "AI_MODEL"
	default:
		return ""
	}
}

// bytesToUint64 将字节数组转换为 uint64（BigEndian）
// 用于解析实例索引中的 blockHeight
func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}

// matchesViewFilter 检查 ResourceView 是否匹配过滤条件
// 用于 ListResources 中的过滤逻辑
func (s *Service) matchesViewFilter(view *ResourceView, filter ResourceViewFilter) bool {
	// Owner 过滤
	if len(filter.Owner) > 0 && !bytes.Equal(view.Owner, filter.Owner) {
		return false
	}

	// Category 过滤
	if filter.Category != nil && view.Category != *filter.Category {
		return false
	}

	// ExecutableType 过滤
	if filter.ExecutableType != nil && view.ExecutableType != *filter.ExecutableType {
		return false
	}

	// Status 过滤
	if filter.Status != nil && view.Status != *filter.Status {
		return false
	}

	// Tags 过滤（如果指定了 Tags，view 必须包含所有指定的 Tags）
	if len(filter.Tags) > 0 {
		if len(view.DeployTags) == 0 {
			return false
		}
		tagMap := make(map[string]bool)
		for _, tag := range view.DeployTags {
			tagMap[tag] = true
		}
		for _, filterTag := range filter.Tags {
			if !tagMap[filterTag] {
				return false
			}
		}
	}

	return true
}
