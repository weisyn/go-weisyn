package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/weisyn/v1/internal/cli"
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

// ConfigFile 配置文件结构，只包含用户友好的配置字段
//
// 🔧 零值陷阱处理说明：
// 为了区分"用户未设置"和"用户设置为零值"，我们使用指针类型：
// - nil: 表示用户未在配置文件中设置该字段，将使用系统默认值
// - &value: 表示用户明确设置了该值，即使是零值（如0、false、""）也会被采用
//
// 示例：
// "min_peers": 0     → 用户明确设置为0个最小节点（允许无节点运行）
// 省略"min_peers"字段 → 使用系统默认值（通常是8个节点）
//
// 这种设计避免了以下问题：
// 1. 用户想设置0但被默认值覆盖
// 2. 用户想设置false但被默认的true覆盖
// 3. 用户想设置空字符串但被默认字符串覆盖

// loadConfigFromFile 从配置文件加载配置（支持自定义路径）
func loadConfigFromFile() config.AppOptions {
	// 首先创建默认配置
	defaultOptions := newOptions()

	// 确定配置文件路径
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

	// 解析JSON配置为标准的AppConfig结构
	var appConfig types.AppConfig
	if err := json.Unmarshal(data, &appConfig); err != nil {
		fmt.Printf("解析配置文件失败: %v，使用默认配置\n", err)
		return defaultOptions
	}

	fmt.Printf("已成功加载配置文件: %s\n", configPath)

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

	// 1. 创建存储目录
	if appConfig.Storage != nil && appConfig.Storage.DataPath != nil {
		storagePath := *appConfig.Storage.DataPath
		directories = append(directories, storagePath)
		fmt.Printf("📁 检测到存储路径: %s\n", storagePath)
	}

	// 2. 创建日志目录
	if appConfig.Log != nil && appConfig.Log.FilePath != nil {
		logPath := *appConfig.Log.FilePath
		logDir := filepath.Dir(logPath)
		directories = append(directories, logDir)
		fmt.Printf("📝 检测到日志路径: %s\n", logDir)
	}

	// 3. 创建P2P身份目录（从node配置中推导）
	// 这里需要从配置文件中解析P2P配置，暂时跳过具体实现
	// TODO: 添加P2P目录创建逻辑

	// 创建所有目录
	for _, dir := range directories {
		if dir == "" {
			continue
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %v", dir, err)
		}

		fmt.Printf("✅ 目录已创建: %s\n", dir)
	}

	if len(directories) > 0 {
		fmt.Printf("🎯 共创建 %d 个数据目录\n", len(directories))
	}

	return nil
}

// applyFileConfig 将文件配置应用到选项
// 注意：applyFileConfig 函数已删除，现在直接使用 types.AppConfig
// 配置解析统一在 internal/config/provider.go 中处理

// App 是WES应用的对外接口
type App interface {
	// Stop 停止应用
	Stop() error

	// Wait 等待应用收到退出信号
	Wait()

	// GetCLIApp 获取CLI应用实例（如果启用）
	GetCLIApp() cli.CLIApp
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

// GetCLIApp 获取CLI应用实例
func (a *internalApp) GetCLIApp() cli.CLIApp {
	// 从bootstrap获取CLI实例
	return a.bootstrap.GetCLIApp()
}

// Start 启动WES应用
func Start(appOptions ...Option) (App, error) {
	// 处理选项
	opts := newOptions(appOptions...)

	// 如果指定了配置文件路径，设置全局变量
	if opts.configFilePath != "" {
		SetConfigFilePath(opts.configFilePath)
	}

	return BootstrapApp(appOptions...)
}

// globalConfigPath 全局配置文件路径变量
var globalConfigPath string

// SetConfigFilePath 设置全局配置文件路径
func SetConfigFilePath(path string) {
	globalConfigPath = path
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
