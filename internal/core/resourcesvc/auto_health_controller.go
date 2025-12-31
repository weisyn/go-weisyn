package resourcesvc

import (
	"context"
	"time"

	"go.uber.org/fx"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// 自动 ResourceUTXO 健康检查与运行模式控制
//
// 设计目标：
// - 节点启动后自动、周期性地检查 ResourceUTXO 索引健康度
// - 在检测到严重不一致时，自动将节点切换到 RepairingUTXO 模式，减少人工干预
// - 在 ResourceUTXO 恢复健康时，自动将节点从 RepairingUTXO 调整为 Degraded（由运维/工具决定何时回到 Normal）

const (
	defaultHealthCheckInterval = 10 * time.Minute
	defaultHealthCheckTimeout  = 30 * time.Second
)

// StartAutoResourceHealthController 作为 fx.Invoke 注入，在节点启动时自动启动后台健康检查协程
func StartAutoResourceHealthController(lc fx.Lifecycle, svc *Service, logger log.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// 后台协程：周期性执行 ResourceUTXO 一致性检查
			go func() {
				// 启动后先延时一小段时间，避免与启动高峰期 IO/CPU 冲突
				startDelay := 1 * time.Minute
				select {
				case <-ctx.Done():
					return
				case <-time.After(startDelay):
				}

				ticker := time.NewTicker(defaultHealthCheckInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						checkCtx, cancelCheck := context.WithTimeout(context.Background(), defaultHealthCheckTimeout)
						result, err := svc.CheckResourceUTXOConsistency(checkCtx)
						cancelCheck()

						if err != nil {
							if logger != nil {
								logger.Warnf("ResourceUTXO 自动一致性检查失败: %v", err)
							}
							continue
						}

						if logger != nil && result != nil {
							logger.Infof("ResourceUTXO 自动一致性检查: codes=%d, instances=%d, inconsistencies=%d, orphanedInstances=%d, orphanedCodes=%d",
								result.TotalCodesChecked,
								result.TotalInstancesFound,
								len(result.Inconsistencies),
								len(result.OrphanedInstances),
								len(result.OrphanedCodes),
							)
						}

						health := runtimectx.GetUTXOHealth(runtimectx.UTXOTypeResource)
						mode := runtimectx.GetNodeMode()

						// 检测到 ResourceUTXO 不一致且当前仍处于 Normal/Degraded 时：
						// 1）自动进入 RepairingUTXO 模式（停止挖矿）
						// 2）自动执行一次 ResourceUTXO 修复
						if health == runtimectx.UTXOHealthInconsistent &&
							(mode == runtimectx.NodeModeNormal || mode == runtimectx.NodeModeDegraded) {

							runtimectx.SetNodeMode(runtimectx.NodeModeRepairingUTXO)
							if logger != nil {
								logger.Warn("检测到 ResourceUTXO 不一致，自动将节点运行模式切换为 RepairingUTXO（停止挖矿并开始修复）")
							}

							// 执行自动修复：从高度 1 到当前最高高度
							repairCtx := context.Background()
							if _, err := svc.RunResourceUTXORepair(repairCtx, 0, 0, false); err != nil {
								if logger != nil {
									logger.Warnf("自动 ResourceUTXO 修复失败: %v", err)
								}
							}

							// 修复完成后再做一次一致性检查，便于更新 UTXOHealth 与后续模式调整
							postCheckCtx, cancelPost := context.WithTimeout(context.Background(), defaultHealthCheckTimeout)
							_, _ = svc.CheckResourceUTXOConsistency(postCheckCtx)
							cancelPost()

							// 下一轮 tick 再根据最新健康状态调整模式
							continue
						}

						// 当 ResourceUTXO 恢复健康，且当前处于 RepairingUTXO 模式时，自动降级为 Degraded
						// （是否回到 Normal 交由运维/工具根据整体状态决定）
						if health == runtimectx.UTXOHealthHealthy && mode == runtimectx.NodeModeRepairingUTXO {
							runtimectx.SetNodeMode(runtimectx.NodeModeDegraded)
							if logger != nil {
								logger.Info("ResourceUTXO 已恢复健康，自动将运行模式从 RepairingUTXO 调整为 Degraded")
							}
						}
					}
				}
			}()

			if logger != nil {
				logger.Infof("✅ ResourceUTXO 自动健康检查控制器已启动: interval=%s, timeout=%s",
					defaultHealthCheckInterval.String(), defaultHealthCheckTimeout.String())
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			// ctx 取消后，上面的 goroutine 会自动退出，这里无需额外操作
			return nil
		},
	})
}


