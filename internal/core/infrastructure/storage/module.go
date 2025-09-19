// Package storage 提供存储管理功能
package storage

import (
	"context"
	"strings"

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
	SQLiteStore storageInterface.SQLiteStore `optional:"true"` // SQLite存储（可选）
	TempStore   storageInterface.TempStore   `optional:"true"` // 临时存储（可选）
}

// Module 返回存储模块
func Module() fx.Option {
	return fx.Module("storage",
		// 提供存储服务
		fx.Provide(ProvideServices),

		// 激活存储
		fx.Invoke(func(lc fx.Lifecycle, provider storageInterface.Provider, badgerStore storageInterface.BadgerStore, tempStore storageInterface.TempStore, logger log.Logger) {
			// 只需获取存储即可激活它
			if _, err := provider.GetBadgerStore("default"); err != nil {
				logger.Warnf("BadgerDB存储激活失败: %v", err)
			} else {
				logger.Info("BadgerDB存储已激活")
			}

			// 添加生命周期钩子确保在应用停止时关闭数据库
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					logger.Info("正在关闭存储服务...")
					logger.Debugf("close_info tempStore_present=%v badgerStore_present=%v", tempStore != nil, badgerStore != nil)

					// 关闭临时存储
					if tempStore != nil {
						logger.Info("开始关闭临时存储...")
						if err := tempStore.Close(); err != nil {
							logger.Errorf("关闭临时存储失败: %v", err)
							// 不要返回错误，继续关闭其他存储
							// return err
						} else {
							logger.Info("临时存储已成功关闭")
						}
					} else {
						logger.Info("临时存储为空，跳过关闭")
					}
					logger.Info("临时存储处理完成，继续关闭BadgerDB...")

					// 关闭BadgerDB数据库连接
					logger.Info("开始关闭BadgerDB存储...")
					if badgerStore != nil {
						logger.Info("BadgerDB存储不为空，开始执行关闭...")
						if err := badgerStore.Close(); err != nil {
							// 如果是LOCK文件不存在的错误，只记录警告而不返回错误
							if strings.Contains(err.Error(), "LOCK: no such file or directory") {
								logger.Warn("BadgerDB LOCK文件已不存在，这通常是正常的关闭过程")
							} else {
								logger.Errorf("关闭BadgerDB存储失败: %v", err)
								return err
							}
						}
						logger.Info("BadgerDB存储已成功关闭")
					} else {
						logger.Warn("BadgerDB存储为空，跳过关闭")
					}

					logger.Info("存储服务已安全关闭")
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

	return ModuleOutput{
		Provider:    serviceOutput.Provider,
		BadgerStore: serviceOutput.BadgerStore,
		FileStore:   serviceOutput.FileStore,
		MemoryStore: serviceOutput.MemoryStore,
		SQLiteStore: serviceOutput.SQLiteStore,
		TempStore:   serviceOutput.TempStore,
	}, nil
}
