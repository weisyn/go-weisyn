package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/weisyn/v1/client/core/contract"
	"github.com/weisyn/v1/client/core/mining"
	"github.com/weisyn/v1/client/core/resource"
	"github.com/weisyn/v1/client/core/transfer"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/client/pkg/ux/screens"
	"github.com/weisyn/v1/client/pkg/ux/ui"
	"github.com/weisyn/v1/configs"
	"github.com/weisyn/v1/internal/app"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	"github.com/weisyn/v1/pkg/types"
	runtimeutil "github.com/weisyn/v1/pkg/utils/runtime"
)

// runningApp 用于让信号处理器拿到正在运行的节点应用句柄，
// 以便在 Ctrl+C 时执行 Stop() 完成清理。
var runningApp app.App

func main() {
	// 创建上下文，支持取消信号
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理（Ctrl+C）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 在单独的 goroutine 中处理信号
	go func() {
		<-sigChan
		fmt.Println("\n\n收到退出信号，正在优雅关闭...")
		// 取消上下文，通知各子模块停止
		cancel()
		// 停止节点应用
		if runningApp != nil {
			fmt.Println("正在停止节点...")
			_ = runningApp.Stop()
		}
		os.Exit(0)
	}()

	// 执行主逻辑
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// ✅ CLI 模式：设置环境变量，强制关闭节点控制台日志输出（避免刷屏影响交互界面）
	// 日志将只写入文件，保持终端干净用于交互式 CLI
	os.Setenv("WES_CLI_MODE", "true")

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          WES 可视化启动器                                      ║")
	fmt.Println("║      节点 + 交互式控制台 (All-in-One)                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 步骤1: 启动内嵌节点（in-process）
	fmt.Println("【步骤 1/4】启动节点（内嵌模式）...")
	nodeApp, endpoint, err := launchEmbeddedNode(ctx)
	if err != nil {
		return fmt.Errorf("启动节点失败: %w", err)
	}
	// 让信号处理器可见
	runningApp = nodeApp
	defer func() {
		fmt.Println("\n正在停止节点...")
		if err := nodeApp.Stop(); err != nil {
			fmt.Printf("停止节点时出错: %v\n", err)
		}
		// 清理句柄，避免误用
		if runningApp == nodeApp {
			runningApp = nil
		}
	}()

	// ✅ app.Start() 成功后，API 已经启动（fx 框架保证 OnStart 钩子完成）
	// 注意：如果配置端口被占用，API 可能在其他端口启动（如 28681）
	// 动态探测实际可用的 API 端点
	actualEndpoint := discoverActualEndpoint(endpoint)
	fmt.Printf("✓ 节点已启动，API 端点: %s\n", actualEndpoint)
	fmt.Println()

	// 步骤2: 初始化客户端和服务
	fmt.Println("【步骤 2/3】初始化客户端和服务...")
	services, err := initializeServices(ctx, actualEndpoint)
	if err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}
	fmt.Println("✓ 服务初始化完成")
	fmt.Println()

	// 步骤3: 启动交互式控制台
	fmt.Println("【步骤 3/3】启动交互式控制台...")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// 运行主菜单（阻塞式，直到用户退出）
	if err := services.mainMenu.Render(ctx); err != nil {
		if err.Error() == "exit" {
			// 正常退出
			return nil
		}
		return fmt.Errorf("控制台运行错误: %w", err)
	}

	return nil
}

// launchEmbeddedNode 启动内嵌节点（in-process，不是子进程）
func launchEmbeddedNode(ctx context.Context) (app.App, string, error) {
	// 使用公链测试网配置（test-public-demo）
	configData := configs.GetPublicChainConfig()

	// 解析配置以获取端口信息
	var appConfig types.AppConfig
	if err := json.Unmarshal(configData, &appConfig); err != nil {
		return nil, "", fmt.Errorf("解析配置失败: %w", err)
	}

	// 确定 API 端点
	httpPort := 28680
	if appConfig.API != nil && appConfig.API.HTTPPort != nil {
		httpPort = *appConfig.API.HTTPPort
	}
	endpoint := fmt.Sprintf("http://localhost:%d", httpPort)

	// 输出启动信息
	chainMode := "public"
	if appConfig.Network != nil && appConfig.Network.ChainMode != nil {
		chainMode = *appConfig.Network.ChainMode
	}
	env := "test"
	if appConfig.Environment != nil {
		env = *appConfig.Environment
	}

	fmt.Printf("   链模式: %s\n", chainMode)
	fmt.Printf("   运行环境: %s\n", env)
	fmt.Printf("   配置来源: 内嵌公链配置（公共测试网 test-public-demo）\n")

	// 输出 network_namespace 信息
	if appConfig.Network != nil && appConfig.Network.NetworkNamespace != nil {
		namespace := *appConfig.Network.NetworkNamespace
		fmt.Printf("   📡 网络命名空间: %s\n", namespace)
	}

	// 计算并输出日志文件位置
	// 日志目录遵循：{data_root}/{env}/{instance_slug}/logs/
	// 其中 instance_slug 默认按规则生成：{env}-{chain_mode}-{network.network_name}
	networkName := "WES_public_testnet_demo_2024" // 默认值
	if appConfig.Network != nil && appConfig.Network.NetworkName != nil {
		networkName = *appConfig.Network.NetworkName
	}
	instanceSlug := fmt.Sprintf("%s-%s-%s", env, chainMode, networkName)
	logDir := filepath.Join(".", "data", env, instanceSlug, "logs")
	fmt.Printf("   📝 日志目录: %s/\n", logDir)
	fmt.Println("      （节点日志将写入文件，不在终端显示）")

	// ✅ 容器内存上限自动感知
	if applied, limit, err := runtimeutil.ApplyCgroupMemoryLimit(0.80); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  [MEMLIMIT] 自动设置 GOMEMLIMIT 失败: %v\n", err)
	} else if applied {
		fmt.Fprintf(os.Stderr, "✅ [MEMLIMIT] 已自动应用 cgroup 内存上限: limit=%d bytes\n", limit)
	}

	// 启动节点（in-process）
	startOptions := []app.Option{
		app.WithEmbeddedConfig(configData),
		app.WithAPI(), // 启用API
	}

	nodeApp, err := app.Start(startOptions...)
	if err != nil {
		return nil, "", fmt.Errorf("节点启动失败: %w", err)
	}

	return nodeApp, endpoint, nil
}

// discoverActualEndpoint 探测实际可用的 API 端点
// 由于端口冲突时 API 会自动切换到下一个可用端口，这里尝试探测实际端口
func discoverActualEndpoint(configuredEndpoint string) string {
	// 解析配置的端口
	// 格式: http://localhost:28680
	basePort := 28680
	if _, err := fmt.Sscanf(configuredEndpoint, "http://localhost:%d", &basePort); err != nil {
		return configuredEndpoint
	}

	client := &http.Client{Timeout: 1 * time.Second}

	// 尝试配置端口和后续几个端口（API 端口冲突时会自动 +1）
	for offset := 0; offset < 10; offset++ {
		port := basePort + offset
		testURL := fmt.Sprintf("http://localhost:%d/health", port)
		resp, err := client.Get(testURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return fmt.Sprintf("http://localhost:%d", port)
			}
		}
	}

	// 如果都没找到，返回配置的端点（可能 API 启动较慢）
	return configuredEndpoint
}

// services 服务集合
type services struct {
	transport       transport.Client
	walletManager   *wallet.AccountManager
	transferService *transfer.TransferService
	miningService   *mining.MiningService
	contractService *contract.ContractService
	resourceService *resource.ResourceService
	mainMenu        *screens.MainMenuScreen
}

// initializeServices 初始化所有服务
func initializeServices(ctx context.Context, endpoint string) (*services, error) {
	// 1. 创建传输客户端
	clientConfig := transport.ClientConfig{
		Endpoints: []transport.EndpointConfig{
			{
				Name:     "local-embedded",
				Priority: 1,
				JSONRPC:  endpoint + "/jsonrpc",
				REST:     endpoint,
			},
		},
		Timeout:             30 * time.Second,
		RetryAttempts:       3,
		RetryBackoff:        time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	transportClient, err := transport.NewFallbackClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建传输客户端失败: %w", err)
	}

	// 2. 创建钱包管理器
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}
	keystoreDir := filepath.Join(homeDir, ".wes", "keystore")

	// 创建密钥管理器（用于地址推导）
	keyManager := key.NewKeyManager()

	// 创建地址管理器（用于地址推导）
	addressManager := address.NewAddressService(keyManager)

	walletManager, err := wallet.NewAccountManager(keystoreDir, addressManager)
	if err != nil {
		return nil, fmt.Errorf("创建钱包管理器失败: %w", err)
	}

	// 3. 创建签名器适配器（从 AccountManager 获取）
	signer := createSignerAdapter(walletManager, keystoreDir)

	// 4. 创建业务服务
	transferService := transfer.NewTransferService(transportClient, signer, addressManager)
	miningService := mining.NewMiningService(transportClient)
	contractService := contract.NewContractService(transportClient, signer)
	resourceService := resource.NewResourceService(transportClient, signer)

	// 5. 创建 UI 组件（使用空日志器）
	uiComponents := ui.NewComponents(ui.NoopLogger())

	// 6. 创建主菜单屏幕
	mainMenu := screens.NewMainMenuScreen(
		transportClient,
		walletManager,
		transferService,
		miningService,
		contractService,
		resourceService,
		uiComponents,
	)

	return &services{
		transport:       transportClient,
		walletManager:   walletManager,
		transferService: transferService,
		miningService:   miningService,
		contractService: contractService,
		resourceService: resourceService,
		mainMenu:        mainMenu,
	}, nil
}

// createSignerAdapter 创建签名器适配器
func createSignerAdapter(am *wallet.AccountManager, keystoreDir string) *wallet.Signer {
	// 尝试获取第一个账户的 Signer
	accounts, err := am.ListAccounts()
	if err == nil && len(accounts) > 0 {
		// 如果有账户，尝试创建 Signer（但需要密码解锁，这里先返回 nil）
		// 实际使用时，用户需要先解锁账户
		return nil
	}
	// 如果没有账户，返回 nil（用户需要先创建账户）
	return nil
}
