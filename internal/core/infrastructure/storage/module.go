// Package storage 提供存储管理功能
package storage

import (
	"context"
	"strings"

	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storageInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"go.uber.org/fx"
)

// ModuleParams 定义存储模块的依赖参数
type ModuleParams struct {
	fx.In

	Provider config.Provider // 配置提供者
	Logger   log.Logger      // 日志记录器
	EventBus event.EventBus  `optional:"true"` // 事件总线（可选）
}

// ModuleOutput 定义存储模块的输出结构
type ModuleOutput struct {
	fx.Out

	// 主存储提供者
	Provider storageInterface.Provider

	// 各个组件的存储接口
	BadgerStore storageInterface.BadgerStore // BadgerDB存储（必需，失败即错误）
	FileStore   storageInterface.FileStore   // 文件存储（必需，失败即错误）
	MemoryStore storageInterface.MemoryStore `optional:"true"` // 内存存储（可选）
	TempStore   storageInterface.TempStore   `optional:"true"` // 临时存储（可选）

	// 🔧 文件存储根路径（供其他模块使用）
	FileStoreRootPath string `name:"file_store_root_path"`
}

// Module 返回存储模块
func Module() fx.Option {
	return fx.Module("storage",
		// 提供存储服务
		fx.Provide(ProvideServices),

		// 激活存储
		fx.Invoke(func(lc fx.Lifecycle, provider storageInterface.Provider, badgerStore storageInterface.BadgerStore, tempStore storageInterface.TempStore, logger log.Logger) {
			// 🎯 为存储模块添加 module 字段，日志将路由到 node-system.log
			var storageLogger log.Logger
			if logger != nil {
				storageLogger = logger.With("module", "storage")
			}
			
			// 只需获取存储即可激活它
			if _, err := provider.GetBadgerStore("default"); err != nil {
				if storageLogger != nil {
					storageLogger.Warnf("BadgerDB存储激活失败: %v", err)
				}
			} else {
				if storageLogger != nil {
					storageLogger.Info("BadgerDB存储已激活")
				}
			}

			// 添加生命周期钩子确保在应用停止时关闭数据库
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if storageLogger != nil {
						storageLogger.Info("正在关闭存储服务...")
						storageLogger.Debugf("close_info tempStore_present=%v badgerStore_present=%v", tempStore != nil, badgerStore != nil)
					}

					// 关闭临时存储
					if tempStore != nil {
						if storageLogger != nil {
							storageLogger.Info("开始关闭临时存储...")
						}
						if err := tempStore.Close(); err != nil {
							if storageLogger != nil {
								storageLogger.Errorf("关闭临时存储失败: %v", err)
							}
							// 不要返回错误，继续关闭其他存储
							// return err
						} else {
							if storageLogger != nil {
								storageLogger.Info("临时存储已成功关闭")
							}
						}
					} else {
						if storageLogger != nil {
							storageLogger.Info("临时存储为空，跳过关闭")
						}
					}
					if storageLogger != nil {
						storageLogger.Info("临时存储处理完成，继续关闭BadgerDB...")
					}

					// 关闭BadgerDB数据库连接
					if storageLogger != nil {
						storageLogger.Info("开始关闭BadgerDB存储...")
					}
					if badgerStore != nil {
						if storageLogger != nil {
							storageLogger.Info("BadgerDB存储不为空，开始执行关闭...")
						}
						if err := badgerStore.Close(); err != nil {
							// 如果是LOCK文件不存在的错误，只记录警告而不返回错误
							if strings.Contains(err.Error(), "LOCK: no such file or directory") {
								if storageLogger != nil {
									storageLogger.Warn("BadgerDB LOCK文件已不存在，这通常是正常的关闭过程")
								}
							} else {
								if storageLogger != nil {
									storageLogger.Errorf("关闭BadgerDB存储失败: %v", err)
								}
								return err
							}
						}
						if storageLogger != nil {
							storageLogger.Info("BadgerDB存储已成功关闭")
						}
					} else {
						if storageLogger != nil {
							storageLogger.Warn("BadgerDB存储为空，跳过关闭")
						}
					}

					if storageLogger != nil {
						storageLogger.Info("存储服务已安全关闭")
					}
					return nil
				},
			})
		}),
	)
}

// ProvideServices 提供存储服务
// 根据配置初始化各类存储引擎并返回
func ProvideServices(params ModuleParams) (ModuleOutput, error) {
	serviceInput := ServiceInput{
		Provider: params.Provider,
		Logger:   params.Logger,
		EventBus: params.EventBus,
	}

	serviceOutput, err := CreateStorageServices(serviceInput)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 获取FileStore的根路径
	fileStoreOptions := params.Provider.GetFile()
	fileStoreRootPath := fileStoreOptions.RootPath
	// 注意：file.New() 已经正确设置了环境隔离路径，这里应该总是有值
	// 保留检查以防配置异常
	if fileStoreRootPath == "" {
		// 这不应该发生，但如果发生了，使用最后的默认值
		fileStoreRootPath = "./data/files" // 最后的默认值
	}

	// 注册 Storage Provider 到内存监控系统
	if reporter, ok := serviceOutput.Provider.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if params.Logger != nil {
			storageLogger := params.Logger.With("module", "storage")
			storageLogger.Info("✅ Storage Provider 已注册到内存监控系统")
		}
	}

	return ModuleOutput{
		Provider:          serviceOutput.Provider,
		BadgerStore:       serviceOutput.BadgerStore,
		FileStore:         serviceOutput.FileStore,
		MemoryStore:       serviceOutput.MemoryStore,
		TempStore:         serviceOutput.TempStore,
		FileStoreRootPath: fileStoreRootPath, // 传递文件存储根路径
	}, nil
}
