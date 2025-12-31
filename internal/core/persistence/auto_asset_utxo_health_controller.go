package persistence

import (
	"context"
	"time"

	"go.uber.org/fx"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
	queryinterfaces "github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

const (
	// 与 ResourceUTXO 保持一致的检查周期与超时设置
	defaultAssetHealthCheckInterval = 10 * time.Minute
	defaultAssetHealthCheckTimeout  = 30 * time.Second
)

// StartAutoAssetUTXOHealthController 作为 fx.Invoke 注入，在节点启动时自动启动资产 UTXO 健康检查与自动修复协程
//
// 设计目标：
// - 周期性检查资产 UTXO 的状态根是否与持久化状态根一致
// - 当检测到不一致时，自动切换到 RepairingUTXO 模式并执行一次资产 UTXO 修复
// - 修复完成且恢复健康后，自动将运行模式从 RepairingUTXO 调整为 Degraded
func StartAutoAssetUTXOHealthController(
	lc fx.Lifecycle,
	utxoQuery queryinterfaces.InternalUTXOQuery,
	logger log.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// 后台协程：周期性执行资产 UTXO 一致性检查与自动修复
			go func() {
				// 启动后先延时一小段时间，避免与启动高峰期 IO/CPU 冲突
				startDelay := 1 * time.Minute
				select {
				case <-ctx.Done():
					return
				case <-time.After(startDelay):
				}

				ticker := time.NewTicker(defaultAssetHealthCheckInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if utxoQuery == nil {
							continue
						}

						checkCtx, cancelCheck := context.WithTimeout(context.Background(), defaultAssetHealthCheckTimeout)
						inconsistent, err := utxoQuery.CheckAssetUTXOConsistency(checkCtx)
						cancelCheck()

						if err != nil {
							if logger != nil {
								logger.Warnf("AssetUTXO 自动一致性检查失败: %v", err)
							}
							continue
						}

						health := runtimectx.GetUTXOHealth(runtimectx.UTXOTypeAsset)
						mode := runtimectx.GetNodeMode()

						// 检测到资产 UTXO 不一致且当前仍处于 Normal/Degraded 时：
						// 1）自动进入 RepairingUTXO 模式（停止挖矿）
						// 2）自动执行一次资产 UTXO 修复
						if inconsistent &&
							(mode == runtimectx.NodeModeNormal || mode == runtimectx.NodeModeDegraded) {

							runtimectx.SetNodeMode(runtimectx.NodeModeRepairingUTXO)
							if logger != nil {
								logger.Warn("检测到 AssetUTXO 状态根与持久化状态根不一致，自动将节点运行模式切换为 RepairingUTXO（停止挖矿并开始资产 UTXO 修复）")
							}

							repairCtx := context.Background()
							if err := utxoQuery.RunAssetUTXORepair(repairCtx, false); err != nil {
								if logger != nil {
									logger.Warnf("自动 AssetUTXO 修复失败: %v", err)
								}
							}

							// 修复完成后再做一次一致性检查，便于更新 UTXOHealth 与后续模式调整
							postCheckCtx, cancelPost := context.WithTimeout(context.Background(), defaultAssetHealthCheckTimeout)
							_, _ = utxoQuery.CheckAssetUTXOConsistency(postCheckCtx)
							cancelPost()

							// 下一轮 tick 再根据最新健康状态调整模式
							continue
						}

						// 当资产 UTXO 恢复健康，且当前处于 RepairingUTXO 模式时，自动降级为 Degraded
						// （是否回到 Normal 交由运维/工具根据整体状态决定）
						if health == runtimectx.UTXOHealthHealthy && mode == runtimectx.NodeModeRepairingUTXO {
							runtimectx.SetNodeMode(runtimectx.NodeModeDegraded)
							if logger != nil {
								logger.Info("AssetUTXO 已恢复健康，自动将运行模式从 RepairingUTXO 调整为 Degraded")
							}
						}
					}
				}
			}()

			if logger != nil {
				logger.Infof("✅ AssetUTXO 自动健康检查控制器已启动: interval=%s, timeout=%s",
					defaultAssetHealthCheckInterval.String(), defaultAssetHealthCheckTimeout.String())
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			// ctx 取消后，上面的 goroutine 会自动退出，这里无需额外操作
			return nil
		},
	})
}


