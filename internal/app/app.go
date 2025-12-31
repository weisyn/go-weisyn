package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	// "github.com/weisyn/v1/internal/cli"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/fx"
)

// AppModule 应用模块定义
var AppModule = fx.Options(
	// 提供应用配置选项，供config模块使用
	fx.Provide(ProvideAppOptions),
)

// ProvideAppOptions 提供应用配置选项实例
// 这个函数为依赖注入系统提供config.AppOptions接口的实现
func ProvideAppOptions(lifecycle fx.Lifecycle) config.AppOptions {
	fmt.Println("🔧 开始加载应用配置...")

	// 尝试从配置文件加载配置（支持自定义路径）
	appOptions := loadConfigFromFile()

	// 在应用启动时记录日志
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			fmt.Println("✅ 应用配置选项已初始化")
			// 配置加载完成
			return nil
		},
	})

	return appOptions
}

// loadConfigFromFile 从配置文件加载配置（支持自定义路径和嵌入配置）
func loadConfigFromFile() config.AppOptions {
	// 首先创建默认配置
	defaultOptions := newOptions()

	var configData []byte
	var configSource string

	// 1. 优先使用全局嵌入配置（如果通过SetEmbeddedConfig设置）
	if len(globalEmbeddedConfig) > 0 {
		configData = globalEmbeddedConfig
		configSource = "嵌入配置（全局）"
	} else if len(defaultOptions.embeddedConfig) > 0 {
		// 2. 其次使用选项中的嵌入配置（如果通过WithEmbeddedConfig设置）
		configData = defaultOptions.embeddedConfig
		configSource = "嵌入配置（选项）"
	} else {
		// 3. 最后使用配置文件路径
		configPath := getConfigFilePath()

		// 检查配置文件是否存在
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("配置文件 %s 不存在，使用默认配置\n", configPath)
			return defaultOptions
		}

		// 读取文件内容
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("读取配置文件失败: %v，使用默认配置\n", err)
			return defaultOptions
		}
		configData = data
		configSource = configPath
	}

	// 解析JSON配置为标准的AppConfig结构
	var appConfig types.AppConfig
	if err := json.Unmarshal(configData, &appConfig); err != nil {
		fmt.Printf("解析配置文件失败: %v，使用默认配置\n", err)
		return defaultOptions
	}

	fmt.Printf("已成功加载配置文件: %s\n", configSource)

	// 使用解析后的AppConfig更新选项
	defaultOptions.appConfig = &appConfig
	fmt.Printf("配置应用完成：已使用统一配置结构\n")

	// 根据配置自动创建数据目录
	if err := createDataDirectories(defaultOptions); err != nil {
		fmt.Printf("⚠️  创建数据目录失败: %v\n", err)
		// 不返回错误，允许系统继续运行，但记录问题
	}

	return defaultOptions
}

// createDataDirectories 根据配置自动创建数据目录结构
func createDataDirectories(opts config.AppOptions) error {
	// 获取配置信息
	appConfig := opts.GetAppConfig()
	if appConfig == nil {
		return fmt.Errorf("无法获取应用配置")
	}

	var directories []string

	// 1. 创建存储根目录（{data_root}）
	if appConfig.Storage != nil && appConfig.Storage.DataRoot != nil {
		storageRoot := *appConfig.Storage.DataRoot
		directories = append(directories, storageRoot)
		fmt.Printf("📁 检测到数据根目录(data_root): %s\n", storageRoot)
	}

	// 2. 日志目录由日志模块自动创建，不需要在这里创建
	// 原因：日志模块会根据 storage.data_root / 实例数据目录自动构建正确的日志路径
	// 如果在这里创建，可能会使用错误的默认路径（如 ./data/logs/）
	// 日志模块会在初始化时创建所需的目录（internal/core/infrastructure/log/log.go）

	// 3. 创建P2P身份目录（从node配置中推导）
	// 这里需要从配置文件中解析P2P配置，暂时跳过具体实现
	// TODO: 添加P2P目录创建逻辑

	// 创建所有目录
	for _, dir := range directories {
		if dir == "" {
			continue
		}

		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %v", dir, err)
		}

		fmt.Printf("✅ 目录已创建: %s\n", dir)
	}

	if len(directories) > 0 {
		fmt.Printf("🎯 共创建 %d 个数据目录\n", len(directories))
	}

	return nil
}

// App 是WES应用的对外接口
type App interface {
	// Stop 停止应用
	Stop() error

	// Wait 等待应用收到退出信号
	Wait()
}

// internalApp WES应用的内部实现
type internalApp struct {
	fxApp     *fx.App
	bootstrap *Bootstrap
}

// Stop 停止应用
func (a *internalApp) Stop() error {
	fmt.Println("🛑 停止应用...")

	// 停止fx应用（包括所有生命周期钩子）
	// 增加超时时间，确保数据库有足够时间完成同步和关闭
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return a.bootstrap.StopApp(ctx)
}

// Wait 等待应用收到退出信号
func (a *internalApp) Wait() {
	fmt.Println("🔄 应用正在运行，按 Ctrl+C 停止...")

	// 创建信号通道
	signals := make(chan os.Signal, 1)

	// 监听中断信号和终止信号
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞等待信号
	sig := <-signals
	fmt.Printf("\n🛑 收到信号 %v，正在优雅退出...\n", sig)

	// 调用Stop方法停止应用
	if err := a.Stop(); err != nil {
		fmt.Printf("⚠️ 停止应用时出错: %v\n", err)
	}
}

// Start 启动WES应用
func Start(appOptions ...Option) (App, error) {
	// 处理选项
	opts := newOptions(appOptions...)

	// 如果指定了嵌入配置，设置全局变量（供ProvideAppOptions使用）
	if len(opts.embeddedConfig) > 0 {
		SetEmbeddedConfig(opts.embeddedConfig)
	} else if opts.configFilePath != "" {
		// 如果指定了配置文件路径（且没有嵌入配置），设置全局变量
		SetConfigFilePath(opts.configFilePath)
	}

	return BootstrapApp(appOptions...)
}

// globalConfigPath 全局配置文件路径变量
var globalConfigPath string

// globalEmbeddedConfig 全局嵌入配置内容（优先级高于configFilePath）
var globalEmbeddedConfig []byte

// SetConfigFilePath 设置全局配置文件路径
func SetConfigFilePath(path string) {
	globalConfigPath = path
}

// SetEmbeddedConfig 设置全局嵌入配置内容
func SetEmbeddedConfig(configBytes []byte) {
	globalEmbeddedConfig = configBytes
}

// getConfigFilePath 获取配置文件路径
func getConfigFilePath() string {
	// 1. 优先使用环境变量 WES_CONFIG_PATH
	if envPath := os.Getenv("WES_CONFIG_PATH"); envPath != "" {
		return envPath
	}

	// 2. 其次使用全局变量（通过SetConfigFilePath设置）
	if globalConfigPath != "" {
		return globalConfigPath
	}

	// 3. 最后使用默认配置路径
	return "configs/development/single/config.json" // 使用开发环境默认配置
}
