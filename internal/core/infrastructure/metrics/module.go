// Package metrics 提供统一的内存监控指标收集机制
//
// 📋 **内存监控基础设施模块 (Memory Metrics Infrastructure Module)**
//
// 本模块提供：
// - MemoryDoctor: 周期性采样内存状态
// - 统一的内存指标收集接口
//
package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/config"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 返回 metrics 模块的 fx.Option
//
// 提供：
// - MemoryDoctor: 内存监控组件
//
// 依赖：
// - config.Provider: 配置提供者
// - *zap.Logger: 日志记录器
func Module() fx.Option {
	return fx.Module("metrics",
		// 提供 MemoryDoctor 实例
		fx.Provide(NewMemoryDoctorProvider),
		// 启动 MemoryDoctor 生命周期
		fx.Invoke(StartMemoryDoctor),
	)
}

// MemoryDoctorProviderInput 定义 MemoryDoctor 的输入依赖
type MemoryDoctorProviderInput struct {
	fx.In

	Config config.Provider `optional:"false"`
	Logger *zap.Logger    `optional:"true"`
}

// NewMemoryDoctorProvider 创建 MemoryDoctor 实例
func NewMemoryDoctorProvider(input MemoryDoctorProviderInput) *MemoryDoctor {
	cfg := DefaultMemoryDoctorConfig()

	// 从配置中读取 memory_monitoring.mode
	if input.Config != nil {
		memConfig := input.Config.GetMemoryMonitoring()
		if memConfig != nil && memConfig.Mode != nil && *memConfig.Mode != "" {
			modeStr := strings.ToLower(*memConfig.Mode)
			switch modeStr {
			case "minimal", "heuristic", "accurate":
				cfg.Mode = MemoryMonitoringMode(modeStr)
			default:
				// 无效模式，使用默认值
				if input.Logger != nil {
					input.Logger.Warn("无效的内存监控模式，使用默认值 heuristic",
						zap.String("provided_mode", modeStr))
				}
			}
		}
	}

	var logger *zap.Logger
	if input.Logger != nil {
		logger = input.Logger.With(zap.String("module", "metrics"))
	}

	md := NewMemoryDoctor(cfg, logger)

	// 设置全局监控模式（供各模块查询）
	metricsutil.SetMemoryMonitoringMode(string(cfg.Mode))

	if logger != nil {
		logger.Info("MemoryDoctor 配置完成",
			zap.String("mode", string(cfg.Mode)))
	}

	return md
}

// StartMemoryDoctor 启动 MemoryDoctor 的生命周期管理
func StartMemoryDoctor(
	lifecycle fx.Lifecycle,
	memoryDoctor *MemoryDoctor,
	logger *zap.Logger,
) {
	if memoryDoctor == nil {
		return
	}

	var metricsLogger *zap.Logger
	if logger != nil {
		metricsLogger = logger.With(zap.String("module", "metrics"))
	}

	// ✅ 创建独立的、长生命周期的context，由cancel显式控制生命周期
	// 修复原因：OnStart的ctx在函数返回后会被取消，导致MemoryDoctor仅运行7ms就停止
	ctx, cancel := context.WithCancel(context.Background())

	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			if metricsLogger != nil {
				metricsLogger.Info("启动 MemoryDoctor...")
			}

			// 在独立的 goroutine 中启动 MemoryDoctor
			// 使用独立的长生命周期ctx，而非OnStart的短生命周期参数ctx
			go func() {
				// 启动时立即采样一次，便于快速验证监控是否正常
				memoryDoctor.SampleOnce()
				// 然后进入定时采样循环
				memoryDoctor.Start(ctx)
			}()

			// 🆕 P2 修复：启动定期内存优化循环
			// 每 10 分钟执行一次 GC + FreeOSMemory，强制释放 RSS
			go memoryDoctor.StartMemoryOptimization(ctx)

			if metricsLogger != nil {
				metricsLogger.Info("✅ MemoryDoctor 已启动（含内存优化循环）")
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			if metricsLogger != nil {
				metricsLogger.Info("停止 MemoryDoctor...")
			}
			// ✅ 显式取消context，优雅停止MemoryDoctor
			cancel()
			
			// 短暂等待，确保goroutine优雅退出
			time.Sleep(100 * time.Millisecond)
			
			if metricsLogger != nil {
				metricsLogger.Info("✅ MemoryDoctor 已停止")
			}
			return nil
		},
	})
}

