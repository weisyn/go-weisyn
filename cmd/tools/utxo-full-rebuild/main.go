package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/fx"

	app "github.com/weisyn/v1/internal/app"
	configmodule "github.com/weisyn/v1/internal/config"
	eutxointerfaces "github.com/weisyn/v1/internal/core/eutxo/interfaces"
	eutxowriter "github.com/weisyn/v1/internal/core/eutxo/writer"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto"
	"github.com/weisyn/v1/internal/core/infrastructure/event"
	logmodule "github.com/weisyn/v1/internal/core/infrastructure/log"
	storagemodule "github.com/weisyn/v1/internal/core/infrastructure/storage"
	"github.com/weisyn/v1/internal/core/maintenance/utxo_rebuild"
	blockquery "github.com/weisyn/v1/internal/core/persistence/query/block"
	queryinterfaces "github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	configiface "github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

var (
	startHeight = flag.Uint64("start-height", 0, "从指定区块高度开始重建（包含），0 表示从高度 1 开始")
	endHeight   = flag.Uint64("end-height", 0, "重建到指定区块高度（包含），0 表示一直到当前链高")
	dryRun      = flag.Bool("dry-run", false, "仅检查并打印将要处理的内容，不实际清空或重建 UTXO")
	configPath  = flag.String("config", "", "配置文件路径（可选，默认使用节点配置路径）")
)

func main() {
	flag.Parse()

	fmt.Fprintf(os.Stderr, "💥 UTXO 全量重建工具（清空 UTXO + 按区块重放重建）\n")
	fmt.Fprintf(os.Stderr, "参数: startHeight=%d, endHeight=%d, dryRun=%v, config=%s\n",
		*startHeight, *endHeight, *dryRun, *configPath)

	// 如果显式指定了配置文件路径，则通过 App 选项传递给 config 模块
	var appOptions []app.Option
	if *configPath != "" {
		appOptions = append(appOptions, app.WithConfigFile(*configPath))
	}

	// 为 config 模块提供 AppOptions（仅使用 AppModule，不启动完整节点）
	fxApp := fx.New(
		// 应用配置（AppOptions）
		app.AppModule,

		// 配置、日志、加密、事件、存储等基础设施模块
		configmodule.Module(),
		logmodule.Module(),
		crypto.Module(),
		event.Module(),
		storagemodule.Module(),

		// 为 config.AppOptions 应用 CLI 传入的附加选项（例如自定义 config 路径）
		fx.Provide(
			func() []app.Option {
				return appOptions
			},
		),

		// 提供 BlockQuery 和 InternalUTXOWriter（直接使用各自的 NewService）
		fx.Provide(
			func(badger storeiface.BadgerStore, fileStore storeiface.FileStore, configProvider configiface.Provider, eb eventiface.EventBus, logger logiface.Logger) (queryinterfaces.InternalBlockQuery, error) {
				return blockquery.NewService(badger, fileStore, configProvider, eb, logger)
			},
			func(badger storeiface.BadgerStore, cryptoOutput crypto.CryptoOutput) (eutxointerfaces.InternalUTXOWriter, error) {
				// eventBus 对于重建流程是可选的，这里传 nil 即可
				return eutxowriter.NewService(badger, cryptoOutput.HashManager, nil, nil)
			},
			func(
				badger storeiface.BadgerStore,
				blockQuery queryinterfaces.InternalBlockQuery,
				utxoWriter eutxointerfaces.InternalUTXOWriter,
				txHashClient transaction.TransactionHashServiceClient,
				logger logiface.Logger,
			) (*utxo_rebuild.Service, error) {
				return utxo_rebuild.NewService(badger, blockQuery, utxoWriter, txHashClient, logger)
			},
		),

		// 在应用启动时执行一次重建任务，然后直接退出进程
		fx.Invoke(func(lc fx.Lifecycle, svc *utxo_rebuild.Service, logger logiface.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go func() {
						runCtx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
						defer cancel()

						stats, err := svc.RunFullUTXORebuild(runCtx, *startHeight, *endHeight, *dryRun)
						if err != nil {
							fmt.Fprintf(os.Stderr, "❌ UTXO 全量重建失败: %v\n", err)
							if logger != nil {
								logger.Errorf("UTXO 全量重建失败: %v", err)
							}
							os.Exit(1)
						}

						fmt.Fprintf(os.Stderr, "✅ UTXO 全量重建完成: start=%d end=%d blocks=%d failedBlocks=%d createdUTXOs=%d deletedUTXOs=%d\n",
							stats.StartHeight, stats.EndHeight, stats.ProcessedBlocks, stats.FailedBlocks, stats.CreatedUTXOs, stats.DeletedUTXOs)
						if logger != nil {
							logger.Infof("UTXO 全量重建完成: start=%d end=%d blocks=%d failedBlocks=%d createdUTXOs=%d deletedUTXOs=%d",
								stats.StartHeight, stats.EndHeight, stats.ProcessedBlocks, stats.FailedBlocks, stats.CreatedUTXOs, stats.DeletedUTXOs)
						}

						// 这是离线维护 CLI，任务完成后直接退出进程即可
						os.Exit(0)
					}()
					return nil
				},
			})
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := fxApp.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动 UTXO 全量重建工具失败: %v\n", err)
		os.Exit(1)
	}

	// 阻塞等待 os.Exit，在 goroutine 中任务完成后会直接退出进程
	select {}
}
