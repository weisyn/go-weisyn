// Package gc implements garbage collection for blockchain data
package gc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// BlockFileGCConfig 块文件 GC 配置
type BlockFileGCConfig struct {
	// 是否启用 GC
	Enabled bool

	// Dry-run 模式：只检测不删除
	DryRun bool

	// 限速配置：每秒最多扫描/删除的文件数
	RateLimitFilesPerSecond int

	// 批量大小：每批处理的文件数
	BatchSize int

	// GC 间隔（自动 GC 模式）
	IntervalSeconds int

	// 保护最近 N 个高度的区块（避免误删）
	ProtectRecentHeight uint64
}

// DefaultBlockFileGCConfig 返回默认配置
func DefaultBlockFileGCConfig() *BlockFileGCConfig {
	return &BlockFileGCConfig{
		Enabled:                 false, // 默认不启用
		DryRun:                  true,  // 默认 dry-run
		RateLimitFilesPerSecond: 100,   // 每秒最多处理 100 个文件
		BatchSize:               50,    // 每批处理 50 个文件
		IntervalSeconds:         3600,  // 每小时运行一次
		ProtectRecentHeight:     1000,  // 保护最近 1000 个区块
	}
}

// BlockFileGC 块文件 GC 服务
type BlockFileGC struct {
	config    *BlockFileGCConfig
	logger    log.Logger
	store     storage.BadgerStore
	fileStore storage.FileStore

	// 运行状态
	running atomic.Bool
	mu      sync.Mutex

	// 指标
	metrics *GCMetrics
}

// GCMetrics GC 指标
type GCMetrics struct {
	LastRunTime         time.Time
	LastRunDuration     time.Duration
	TotalScannedFiles   atomic.Int64
	TotalDeletedFiles   atomic.Int64
	TotalReclaimedBytes atomic.Int64
	TotalRuns           atomic.Int64
	LastRunResult       *GCRunResult
}

// GCRunResult 单次 GC 运行结果
type GCRunResult struct {
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	ReachableBlocks  int
	ScannedFiles     int
	UnreachableFiles int
	DeletedFiles     int
	ReclaimedBytes   int64
	Errors           []string
	DryRun           bool
}

// NewBlockFileGC 创建块文件 GC 服务
func NewBlockFileGC(
	config *BlockFileGCConfig,
	logger log.Logger,
	store storage.BadgerStore,
	fileStore storage.FileStore,
) *BlockFileGC {
	if config == nil {
		config = DefaultBlockFileGCConfig()
	}

	return &BlockFileGC{
		config:    config,
		logger:    logger,
		store:     store,
		fileStore: fileStore,
		metrics:   &GCMetrics{},
	}
}

// Start 启动 GC 服务（自动模式）
func (gc *BlockFileGC) Start(ctx context.Context) error {
	if !gc.config.Enabled {
		if gc.logger != nil {
			gc.logger.Info("🗑️  块文件 GC 未启用，跳过启动")
		}
		return nil
	}

	if gc.running.Load() {
		return fmt.Errorf("GC 服务已在运行中")
	}

	gc.running.Store(true)

	if gc.logger != nil {
		gc.logger.Infof("🗑️  块文件 GC 服务已启动（间隔: %d秒, dry-run: %v）",
			gc.config.IntervalSeconds, gc.config.DryRun)
	}

	// 启动定期 GC goroutine
	go gc.runPeriodic(ctx)

	return nil
}

// Stop 停止 GC 服务
func (gc *BlockFileGC) Stop(ctx context.Context) error {
	gc.running.Store(false)

	if gc.logger != nil {
		gc.logger.Info("🗑️  块文件 GC 服务已停止")
	}

	return nil
}

// runPeriodic 定期运行 GC
func (gc *BlockFileGC) runPeriodic(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(gc.config.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !gc.running.Load() {
				return
			}

			if gc.logger != nil {
				gc.logger.Info("🗑️  开始定期块文件 GC")
			}

			result, err := gc.RunGC(ctx)
			if err != nil {
				if gc.logger != nil {
					gc.logger.Errorf("定期 GC 失败: %v", err)
				}
				continue
			}

			if gc.logger != nil {
				gc.logger.Infof("✅ 定期 GC 完成：扫描=%d 不可达=%d 删除=%d 回收=%d bytes",
					result.ScannedFiles, result.UnreachableFiles, result.DeletedFiles, result.ReclaimedBytes)
			}
		}
	}
}

// RunGC 手动运行一次 GC（阻塞模式）
func (gc *BlockFileGC) RunGC(ctx context.Context) (*GCRunResult, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	startTime := time.Now()
	result := &GCRunResult{
		StartTime: startTime,
		DryRun:    gc.config.DryRun,
		Errors:    []string{},
	}

	// 设置运行状态
	gc.setRunningStatus(true)
	defer gc.setRunningStatus(false)

	if gc.logger != nil {
		gc.logger.Infof("🗑️  开始块文件 GC（dry-run: %v）", gc.config.DryRun)
	}

	// Phase 1: Mark - 构建可达集合
	reachableSet, err := gc.buildReachableSet(ctx)
	if err != nil {
		// 更新错误指标
		gc.updateMetrics(nil, 0, err)
		return nil, fmt.Errorf("构建可达集合失败: %w", err)
	}
	result.ReachableBlocks = len(reachableSet)

	if gc.logger != nil {
		gc.logger.Infof("📊 可达区块数: %d", result.ReachableBlocks)
	}

	// Phase 2: Sweep - 扫描并删除不可达文件
	scannedFiles, unreachableFiles, deletedFiles, reclaimedBytes, errors := gc.sweepUnreachableFiles(ctx, reachableSet)
	result.ScannedFiles = scannedFiles
	result.UnreachableFiles = unreachableFiles
	result.DeletedFiles = deletedFiles
	result.ReclaimedBytes = reclaimedBytes
	result.Errors = errors

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 更新内部指标
	gc.metrics.LastRunTime = result.EndTime
	gc.metrics.LastRunDuration = result.Duration
	gc.metrics.TotalScannedFiles.Add(int64(scannedFiles))
	gc.metrics.TotalDeletedFiles.Add(int64(deletedFiles))
	gc.metrics.TotalReclaimedBytes.Add(reclaimedBytes)
	gc.metrics.TotalRuns.Add(1)
	gc.metrics.LastRunResult = result

	// 更新 Prometheus 指标
	gc.updateMetrics(result, result.Duration.Seconds(), nil)

	if gc.logger != nil {
		gc.logger.Infof("✅ GC 完成：耗时=%v 扫描=%d 不可达=%d 删除=%d 回收=%d bytes 错误=%d",
			result.Duration, result.ScannedFiles, result.UnreachableFiles,
			result.DeletedFiles, result.ReclaimedBytes, len(result.Errors))
	}

	return result, nil
}

// buildReachableSet 构建可达区块集合（基于 indices:height）
func (gc *BlockFileGC) buildReachableSet(ctx context.Context) (map[uint64]bool, error) {
	reachableSet := make(map[uint64]bool)

	// 扫描 indices:height: 前缀
	prefix := []byte("indices:height:")

	results, err := gc.store.PrefixScan(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("扫描 indices:height 失败: %w", err)
	}

	for keyStr := range results {
		// key 格式：indices:height:{height}
		if !strings.HasPrefix(keyStr, "indices:height:") {
			continue
		}

		// 解析高度
		heightStr := strings.TrimPrefix(keyStr, "indices:height:")
		var height uint64
		if _, err := fmt.Sscanf(heightStr, "%d", &height); err != nil {
			if gc.logger != nil {
				gc.logger.Warnf("解析区块高度失败: key=%s err=%v", keyStr, err)
			}
			continue
		}

		reachableSet[height] = true
	}

	return reachableSet, nil
}

// sweepUnreachableFiles 扫描并删除不可达文件
func (gc *BlockFileGC) sweepUnreachableFiles(
	ctx context.Context,
	reachableSet map[uint64]bool,
) (scannedFiles, unreachableFiles, deletedFiles int, reclaimedBytes int64, errors []string) {
	// 限速器：每秒最多处理 RateLimitFilesPerSecond 个文件
	rateLimiter := time.NewTicker(time.Second / time.Duration(gc.config.RateLimitFilesPerSecond))
	defer rateLimiter.Stop()

	// 获取当前最高区块高度（用于保护最近的区块）
	currentHeight := gc.getCurrentHeight(ctx, reachableSet)
	protectThreshold := uint64(0)
	if currentHeight > gc.config.ProtectRecentHeight {
		protectThreshold = currentHeight - gc.config.ProtectRecentHeight
	}

	if gc.logger != nil {
		gc.logger.Infof("🛡️  保护阈值: height >= %d（当前高度: %d, 保护窗口: %d）",
			protectThreshold, currentHeight, gc.config.ProtectRecentHeight)
	}

	// 扫描 blocks/ 目录
	// 目录结构：blocks/{heightSegment:010d}/{height:010d}.bin
	// 我们需要递归扫描所有子目录

	// 使用 FileStore 列出 blocks/ 目录下的所有子目录
	// 首先，我们需要手动扫描子目录（段目录）
	// 由于 ListFiles 不包含目录，我们需要枚举所有可能的段目录

	// 为了简化实现，我们直接扫描所有可能的段目录（0, 1000, 2000, ...）
	// 最大高度可以从 reachableSet 中获取
	maxHeight := gc.getCurrentHeight(ctx, reachableSet)
	maxSegment := (maxHeight / 1000) * 1000

	for segment := uint64(0); segment <= maxSegment+10000; segment += 1000 {
		segmentPath := fmt.Sprintf("blocks/%010d", segment)

		// 列出该段目录下的所有文件
		files, err := gc.fileStore.ListFiles(ctx, segmentPath, "*.bin")
		if err != nil {
			// 目录可能不存在，跳过
			continue
		}

		for _, filePath := range files {
			scannedFiles++

			// 解析文件名获取高度
			// filePath 格式：blocks/0000000000/0000000001.bin
			fileName := filepath.Base(filePath)
			if !strings.HasSuffix(fileName, ".bin") {
				continue
			}

			heightStr := strings.TrimSuffix(fileName, ".bin")
			var height uint64
			if _, err := fmt.Sscanf(heightStr, "%d", &height); err != nil {
				if gc.logger != nil {
					gc.logger.Warnf("解析文件名失败: %s err=%v", fileName, err)
				}
				continue
			}

			// 检查是否在可达集合中
			if reachableSet[height] {
				// 可达，跳过
				continue
			}

			// 检查是否在保护窗口内
			if height >= protectThreshold {
				// 在保护窗口内，跳过
				if gc.logger != nil && scannedFiles%100 == 0 {
					gc.logger.Debugf("跳过保护窗口内的文件: height=%d file=%s", height, fileName)
				}
				continue
			}

			// 不可达且不在保护窗口内
			unreachableFiles++

			// 获取文件大小
			fileInfo, err := gc.fileStore.FileInfo(ctx, filePath)
			if err != nil {
				if gc.logger != nil {
					gc.logger.Warnf("获取文件信息失败: %s err=%v", filePath, err)
				}
				// 估算文件大小为 100KB（平均区块大小）
				fileInfo.Size = 100 * 1024
			}

			if gc.config.DryRun {
				// Dry-run 模式：只记录，不删除
				if gc.logger != nil && unreachableFiles%10 == 0 {
					gc.logger.Debugf("🔍 [DRY-RUN] 不可达文件: height=%d file=%s size=%d",
						height, filePath, fileInfo.Size)
				}
				reclaimedBytes += fileInfo.Size
			} else {
				// 限速
				select {
				case <-ctx.Done():
					errors = append(errors, "GC 被取消")
					return
				case <-rateLimiter.C:
					// 继续
				}

				// 删除文件
				if err := gc.fileStore.Delete(ctx, filePath); err != nil {
					errors = append(errors, fmt.Sprintf("删除文件失败: %s err=%v", filePath, err))
					continue
				}

				deletedFiles++
				reclaimedBytes += fileInfo.Size

				if gc.logger != nil && deletedFiles%10 == 0 {
					gc.logger.Infof("🗑️  已删除不可达文件: height=%d file=%s size=%d",
						height, filePath, fileInfo.Size)
				}
			}
		}
	}

	return
}

// getCurrentHeight 从可达集合中获取当前最高区块高度
func (gc *BlockFileGC) getCurrentHeight(ctx context.Context, reachableSet map[uint64]bool) uint64 {
	var maxHeight uint64
	for height := range reachableSet {
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxHeight
}

// GetMetrics 获取 GC 指标
func (gc *BlockFileGC) GetMetrics() *GCMetrics {
	return gc.metrics
}

// IsRunning 检查 GC 是否正在运行
func (gc *BlockFileGC) IsRunning() bool {
	return gc.running.Load()
}

// GCStatus GC 状态信息
type GCStatus struct {
	Enabled       bool
	Running       bool
	LastRunTime   time.Time
	LastRunResult *GCRunResult
	Metrics       *GCMetrics
}

// GetStatus 获取 GC 状态
//
// 返回 GC 的当前状态，包括是否启用、是否运行中、最后运行时间等信息
func (gc *BlockFileGC) GetStatus() *GCStatus {
	return &GCStatus{
		Enabled:       gc.config.Enabled,
		Running:       gc.running.Load(),
		LastRunTime:   gc.metrics.LastRunTime,
		LastRunResult: gc.metrics.LastRunResult,
		Metrics:       gc.metrics,
	}
}

// ManualRun 手动触发 GC（支持覆盖 dry-run 设置）
//
// 允许运维人员手动触发 GC，可以覆盖配置中的 dry-run 设置。
// 如果 dryRun 参数为 nil，则使用配置中的值；否则使用提供的值。
//
// 参数：
//   - ctx: 上下文
//   - dryRun: 是否使用 dry-run 模式（nil 表示使用配置值）
//
// 返回：
//   - result: GC 运行结果
//   - err: 错误信息
func (gc *BlockFileGC) ManualRun(ctx context.Context, dryRun *bool) (*GCRunResult, error) {
	// 保存原始配置
	originalDryRun := gc.config.DryRun

	// 如果提供了 dryRun 参数，临时覆盖配置
	if dryRun != nil {
		gc.config.DryRun = *dryRun
		defer func() {
			// 恢复原始配置
			gc.config.DryRun = originalDryRun
		}()
	}

	// 执行 GC
	return gc.RunGC(ctx)
}
