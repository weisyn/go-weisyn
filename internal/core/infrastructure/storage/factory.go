// Package storage 提供存储服务工厂实现
package storage

import (
	"fmt"
	"path/filepath"

	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	temporaryconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	"github.com/weisyn/v1/internal/core/infrastructure/storage/badger"
	"github.com/weisyn/v1/internal/core/infrastructure/storage/file"
	"github.com/weisyn/v1/internal/core/infrastructure/storage/memory"
	tempstore "github.com/weisyn/v1/internal/core/infrastructure/storage/tempstore"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storageInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ServiceInput 定义存储服务工厂的输入参数
type ServiceInput struct {
	Provider config.Provider // 配置提供者
	Logger   log.Logger      // 日志记录器
	EventBus event.EventBus  `optional:"true"` // 事件总线（可选）
}

// ServiceOutput 定义存储服务工厂的输出结果
type ServiceOutput struct {
	Provider    storageInterface.Provider
	BadgerStore storageInterface.BadgerStore
	FileStore   storageInterface.FileStore
	MemoryStore storageInterface.MemoryStore
	SQLiteStore storageInterface.SQLiteStore
	TempStore   storageInterface.TempStore
}

// CreateStorageServices 创建存储服务
//
// 🏭 **存储服务工厂**：
// 该函数负责创建存储模块的所有服务，处理各种存储引擎的初始化。
// 将复杂的存储初始化逻辑从module.go中分离出来，保持module.go的薄实现。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - ServiceOutput: 创建的服务实例集合
//   - error: 创建过程中的错误
func CreateStorageServices(input ServiceInput) (ServiceOutput, error) {
	provider := input.Provider
	logger := input.Logger

	// 获取各存储配置
	badgerOptions := provider.GetBadger()
	memoryOptions := provider.GetMemory()
	_ = provider.GetFile()      // 文件存储配置（暂用默认值）
	_ = provider.GetTemporary() // 临时存储配置（暂用默认值）

	// 创建配置对象
	badgerCfg := badgerconfig.NewFromOptions(badgerOptions)
	memoryCfg := memoryconfig.New(memoryOptions)
	fileCfg := fileconfig.New(nil)      // 使用默认配置
	tempCfg := temporaryconfig.New(nil) // 使用默认配置

	// 声明存储实例
	var (
		badgerStore     storageInterface.BadgerStore
		memoryStore     storageInterface.MemoryStore
		fileStore       storageInterface.FileStore
		tempStore       storageInterface.TempStore
		storageProvider storageInterface.Provider
	)

	// 初始化BadgerDB存储（必需）
	badgerStore = badger.New(badgerCfg, logger)
	if badgerStore == nil {
		logger.Error("BadgerDB存储初始化失败")
		return ServiceOutput{}, fmt.Errorf("存储初始化失败：BadgerDB存储不可用")
	}
	// 显示实际使用的数据路径，并转换为绝对路径
	actualPath := badgerOptions.Path
	if actualPath == "" {
		actualPath = "./data/badger" // 默认路径
	}

	// 转换为绝对路径以避免混淆
	absPath, err := filepath.Abs(actualPath)
	if err != nil {
		logger.Warnf("无法转换为绝对路径 %s: %v，使用原路径", actualPath, err)
		absPath = actualPath
	}

	logger.Infof("✅ BadgerDB存储初始化成功")
	logger.Infof("📁 数据存储路径: %s", absPath)
	if absPath != actualPath {
		logger.Infof("   (配置路径: %s)", actualPath)
	}

	// 初始化内存存储（兜底）
	memoryStore = memory.New(memoryCfg, logger)
	if memoryStore == nil {
		logger.Warn("内存存储初始化失败，将影响缓存功能")
		// 内存存储失败不阻止启动，但记录警告
	} else {
		logger.Info("✅ 内存存储初始化成功")
	}

	// 初始化文件存储（必需）
	fileStore = file.New(fileCfg, logger)
	if fileStore == nil {
		logger.Error("文件存储初始化失败")
		return ServiceOutput{}, fmt.Errorf("存储初始化失败：文件存储不可用")
	}
	logger.Info("✅ 文件存储初始化成功")

	// 初始化临时存储
	tempStore = tempstore.New(tempCfg, logger)
	if tempStore == nil {
		logger.Warn("临时存储初始化失败，将影响临时数据处理")
		// 临时存储失败不阻止启动，但记录警告
	} else {
		logger.Info("✅ 临时存储初始化成功")
	}

	// 创建存储提供者（聚合所有存储引擎）
	storageProvider = NewProvider(badgerStore, fileStore, memoryStore, nil, tempStore, logger)
	logger.Info("✅ 存储提供者初始化成功")

	logger.Info("🎯 存储模块所有服务初始化完成")

	return ServiceOutput{
		Provider:    storageProvider,
		BadgerStore: badgerStore,
		FileStore:   fileStore,
		MemoryStore: memoryStore,
		SQLiteStore: nil, // SQLite存储暂未实现
		TempStore:   tempStore,
	}, nil
}
