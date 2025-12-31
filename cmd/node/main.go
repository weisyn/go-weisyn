package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/weisyn/v1/configs"
	"github.com/weisyn/v1/internal/app"
	"github.com/weisyn/v1/pkg/types"
	runtimeutil "github.com/weisyn/v1/pkg/utils/runtime"
)

const (
	version = "1.0.0"
)

func main() {
	// 添加 panic recovery，确保任何 panic 都能被捕获
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n❌ [PANIC] 程序发生严重错误: %v\n", r)
			os.Stderr.Sync()
			// 打印堆栈信息
			fmt.Fprintf(os.Stderr, "请检查配置和依赖是否正确\n")
			os.Exit(1)
		}
	}()

	// 立即输出，确保程序开始执行
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 程序开始执行，参数: %v\n", os.Args)
	os.Stderr.Sync() // 强制刷新输出

	// 强制输出到标准输出，确保能看到
	fmt.Println("🚀 weisyn-node 启动中...")
	os.Stdout.Sync()

	// 检查是否是子命令（例如：chain init）
	if len(os.Args) > 1 && os.Args[1] == "chain" {
		if len(os.Args) > 2 && os.Args[2] == "init" {
			fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 执行子命令: chain init\n")
			os.Stderr.Sync()
			chainInitCommand(os.Args[3:])
			return
		}
	}

	var (
		chainMode       string // 链模式：public | consortium | private
		configPath      string // 用户配置文件路径（联盟链/私链必需）
		httpPort        int    // HTTP端口（节点级覆盖）
		grpcPort        int    // gRPC端口（节点级覆盖）
		diagnosticsPort int    // 诊断端口（节点级覆盖）
		dataDir         string // 数据目录（节点级覆盖）
		showHelp        bool   // 显示帮助
		showVersion     bool   // 显示版本
	)

	flag.StringVar(&chainMode, "chain", "", "链模式：public（公链）| consortium（联盟链）| private（私链）")
	flag.StringVar(&configPath, "config", "", "配置文件路径（联盟链/私链必需，公链不需要）")
	flag.IntVar(&httpPort, "http-port", 0, "HTTP端口（节点级覆盖，不影响链级配置）")
	flag.IntVar(&grpcPort, "grpc-port", 0, "gRPC端口（节点级覆盖，不影响链级配置）")
	flag.IntVar(&diagnosticsPort, "diagnostics-port", 0, "诊断HTTP端口（节点级覆盖，不影响链级配置，用于pprof/diagnostics）")
	flag.StringVar(&dataDir, "data-dir", "", "数据目录（节点级覆盖，不影响链级配置）")
	flag.BoolVar(&showHelp, "help", false, "显示帮助信息")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 参数解析完成: chain=%s, config=%s\n", chainMode, configPath)
	os.Stderr.Sync()

	// 显示版本
	if showVersion {
		fmt.Printf("weisyn-node v%s\n", version)
		return
	}

	// 显示帮助
	if showHelp {
		showHelpInfo()
		return
	}

	// 验证链模式
	if chainMode == "" {
		fmt.Println("❌ 错误: 必须指定 --chain 参数")
		fmt.Println("💡 使用 --help 查看帮助信息")
		os.Exit(1)
	}

	chainMode = strings.ToLower(chainMode)
	// mainnet 是 public 的别名，统一转换为 public
	if chainMode == "mainnet" {
		chainMode = "public"
	}
	if chainMode != "public" && chainMode != "consortium" && chainMode != "private" {
		fmt.Printf("❌ 错误: 无效的链模式 '%s'\n", chainMode)
		fmt.Println("💡 有效选项: public | mainnet | consortium | private")
		fmt.Println("   - public/mainnet: 公有链模式（--chain public 使用公共测试网，--chain public --config <path> 使用自建公链）")
		fmt.Println("   - consortium: 联盟链模式（必须提供 --config）")
		fmt.Println("   - private: 私有链模式（必须提供 --config）")
		os.Exit(1)
	}

	// 根据链模式加载配置
	var configData []byte
	var configSource string

	switch chainMode {
	case "public", "mainnet":
		// 公链模式：
		// - --chain public（无 --config）→ 公共测试网（内嵌配置，test-public-demo）
		// - --chain public --config <path> → 自建公链（用户配置）
		if configPath == "" {
			// 公共测试网：使用内嵌配置
			configData = configs.GetPublicChainConfig()
			configSource = "内嵌公链配置（公共测试网 test-public-demo）"
		} else {
			// 自建公链：读取用户配置文件
			data, err := os.ReadFile(configPath)
			if err != nil {
				fmt.Printf("❌ 读取配置文件失败: %v\n", err)
				os.Exit(1)
			}
			configData = data
			configSource = configPath

			// 验证配置文件中的 chain_mode 必须为 "public"
			if err := validateChainModeInConfig(configData, "public"); err != nil {
				fmt.Printf("❌ 配置文件验证失败: %v\n", err)
				fmt.Println("💡 自建公链的配置文件中 network.chain_mode 必须为 \"public\"")
				os.Exit(1)
			}
		}

	case "consortium", "private":
		// 联盟链/私链：必须提供配置文件
		if configPath == "" {
			fmt.Printf("❌ 错误: %s链模式必须通过 --config 指定配置文件\n", chainMode)
			fmt.Println("💡 使用以下命令生成配置文件模板：")
			fmt.Printf("   weisyn-node chain init --mode %s --out ./my-%s-chain.json\n", chainMode, chainMode)
			os.Exit(1)
		}

		// 读取用户配置文件
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("❌ 读取配置文件失败: %v\n", err)
			os.Exit(1)
		}
		configData = data
		configSource = configPath

		// 验证配置文件中的 chain_mode 是否匹配
		if err := validateChainModeInConfig(configData, chainMode); err != nil {
			fmt.Printf("❌ 配置文件验证失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 解析配置
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 开始解析配置，配置数据长度: %d 字节\n", len(configData))
	os.Stderr.Sync()

	var appConfig types.AppConfig
	if err := json.Unmarshal(configData, &appConfig); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 解析配置文件失败: %v\n", err)
		previewLen := 100
		if len(configData) < previewLen {
			previewLen = len(configData)
		}
		fmt.Fprintf(os.Stderr, "配置数据前%d字节: %s\n", previewLen, string(configData[:previewLen]))
		os.Stderr.Sync()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 配置解析成功\n")
	os.Stderr.Sync()

	// 验证配置文件中必须包含 environment 字段
	if appConfig.Environment == nil || *appConfig.Environment == "" {
		if chainMode == "public" {
			fmt.Println("❌ 错误: 内嵌公链配置缺少 environment 字段")
			fmt.Println("💡 这是内部错误，请报告给开发团队")
		} else {
			fmt.Printf("❌ 错误: 配置文件缺少 environment 字段\n")
			fmt.Println("💡 请在配置文件中添加 environment 字段（dev | test | prod）")
			fmt.Println("💡 示例:")
			fmt.Println(`   {
     "environment": "prod",
     "network": { ... }
   }`)
		}
		os.Exit(1)
	}

	// 验证 environment 字段值
	envValue := strings.ToLower(*appConfig.Environment)
	if envValue != "dev" && envValue != "test" && envValue != "prod" {
		fmt.Printf("❌ 错误: 配置文件中的 environment 字段值无效: %s\n", *appConfig.Environment)
		fmt.Println("💡 有效选项: dev | test | prod")
		os.Exit(1)
	}

	// 应用节点级覆盖（端口、数据目录）
	if err := applyNodeOverrides(&appConfig, httpPort, grpcPort, diagnosticsPort, dataDir); err != nil {
		fmt.Printf("❌ 应用节点级配置失败: %v\n", err)
		os.Exit(1)
	}

	// 验证配置
	if err := validateConfig(&appConfig, chainMode); err != nil {
		fmt.Printf("❌ 配置验证失败: %v\n", err)
		os.Exit(1)
	}

	// 注意：开源仓库内嵌的是测试网配置（test-public-demo），不再内嵌生产主网配置
	// 如需连接生产主网，请通过 BaaS 或运维工具获取生产配置
	// 因此不再进行"官方主网身份"校验

	// 重新序列化为JSON（用于内嵌配置）
	finalConfigData, err := json.Marshal(&appConfig)
	if err != nil {
		fmt.Printf("❌ 序列化配置失败: %v\n", err)
		os.Exit(1)
	}

	// 输出启动信息
	fmt.Printf("🚀 正在启动 weisyn-node\n")
	fmt.Printf("   链模式: %s\n", chainMode)
	fmt.Printf("   运行环境: %s\n", *appConfig.Environment)
	fmt.Printf("   配置来源: %s\n", configSource)

	// 🛡️ 输出 network_namespace 摘要信息（用于验证隔离）
	if appConfig.Network != nil && appConfig.Network.NetworkNamespace != nil {
		namespace := *appConfig.Network.NetworkNamespace
		fmt.Printf("   📡 网络命名空间: %s\n", namespace)
		fmt.Printf("      - 协议 ID 前缀: /weisyn/%s/\n", namespace)
		fmt.Printf("      - DHT 前缀: /weisyn/%s\n", namespace)
		fmt.Printf("      - Gossip 主题前缀: weisyn.%s.\n", namespace)
		fmt.Printf("      - Rendezvous namespace: weisyn-%s\n", namespace)
		fmt.Printf("      - mDNS 服务名: weisyn-node-%s\n", namespace)
	} else {
		fmt.Printf("   ⚠️  警告: network_namespace 未配置，可能导致网络隔离失败\n")
	}

	os.Stdout.Sync() // 强制刷新输出

	// ✅ 容器内存上限自动感知：避免 Go 堆无限增长后被 cgroup OOM killer 直接杀死
	if applied, limit, err := runtimeutil.ApplyCgroupMemoryLimit(0.80); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  [MEMLIMIT] 自动设置 GOMEMLIMIT 失败: %v\n", err)
		os.Stderr.Sync()
	} else if applied {
		fmt.Fprintf(os.Stderr, "✅ [MEMLIMIT] 已自动应用 cgroup 内存上限: limit=%d bytes (ratio=0.80)\n", limit)
		os.Stderr.Sync()
	}

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 准备调用 app.Start()\n")
	os.Stderr.Sync()

	// 启动节点
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 准备创建启动选项\n")
	os.Stderr.Sync()

	startOptions := []app.Option{
		app.WithEmbeddedConfig(finalConfigData),
		app.WithAPI(), // 启用API
	}

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 调用 app.Start()，配置数据长度: %d 字节\n", len(finalConfigData))
	os.Stderr.Sync()

	nodeApp, err := app.Start(startOptions...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 节点启动失败: %v\n", err)
		os.Stderr.Sync()
		// 尝试输出更详细的错误信息
		if errStr := err.Error(); errStr != "" {
			fmt.Fprintf(os.Stderr, "错误详情: %s\n", errStr)
			os.Stderr.Sync()
		}
		os.Exit(1)
	}

	if nodeApp == nil {
		fmt.Fprintf(os.Stderr, "❌ 节点启动失败: app.Start() 返回了 nil\n")
		os.Stderr.Sync()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] app.Start() 成功，准备调用 Wait()\n")
	os.Stderr.Sync()

	// ✅ 节点启动成功，输出摘要信息
	fmt.Println("✅ 节点启动成功！")
	if appConfig.Network != nil && appConfig.Network.NetworkNamespace != nil {
		namespace := *appConfig.Network.NetworkNamespace
		fmt.Printf("📡 当前节点网络命名空间: %s\n", namespace)
		fmt.Printf("   💡 提示: 只有相同 namespace 的节点才能相互发现和通信\n")
	}
	os.Stdout.Sync()

	// 等待退出信号
	fmt.Fprintf(os.Stderr, "🔍 [DEBUG] 调用 nodeApp.Wait()，程序将阻塞等待信号\n")
	os.Stderr.Sync()
	nodeApp.Wait()
}

// validateChainModeInConfig 验证配置文件中的 chain_mode 是否匹配
func validateChainModeInConfig(configData []byte, expectedMode string) error {
	var configMap map[string]interface{}
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	network, ok := configMap["network"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("配置文件缺少 network 字段")
	}

	chainMode, ok := network["chain_mode"].(string)
	if !ok {
		return fmt.Errorf("配置文件 network.chain_mode 字段缺失或无效")
	}

	chainMode = strings.ToLower(chainMode)
	if chainMode != expectedMode {
		return fmt.Errorf("配置文件中的 chain_mode (%s) 与命令行参数 (%s) 不匹配", chainMode, expectedMode)
	}

	return nil
}

// applyNodeOverrides 应用节点级覆盖配置（端口、数据目录等）
func applyNodeOverrides(appConfig *types.AppConfig, httpPort, grpcPort, diagnosticsPort int, dataDir string) error {
	// 覆盖HTTP端口
	if httpPort > 0 {
		if appConfig.API == nil {
			appConfig.API = &types.UserAPIConfig{}
		}
		port := httpPort
		appConfig.API.HTTPPort = &port
	}

	// 覆盖gRPC端口
	if grpcPort > 0 {
		if appConfig.API == nil {
			appConfig.API = &types.UserAPIConfig{}
		}
		port := grpcPort
		appConfig.API.GRPCPort = &port
	}

	// 覆盖诊断端口
	if diagnosticsPort > 0 {
		if appConfig.Node == nil {
			appConfig.Node = &types.UserNodeConfig{}
		}
		if appConfig.Node.Host == nil {
			appConfig.Node.Host = &types.UserHostConfig{}
		}
		port := diagnosticsPort
		appConfig.Node.Host.DiagnosticsPort = &port
	}

	// 覆盖数据根目录（data_root）
	if dataDir != "" {
		if appConfig.Storage == nil {
			appConfig.Storage = &types.UserStorageConfig{}
		}
		appConfig.Storage.DataRoot = &dataDir
	}

	return nil
}

// validateConfig 验证配置
func validateConfig(appConfig *types.AppConfig, chainMode string) error {
	// 验证 chain_mode
	if appConfig.Network == nil || appConfig.Network.ChainMode == nil {
		return fmt.Errorf("配置缺少 network.chain_mode 字段")
	}

	configChainMode := strings.ToLower(*appConfig.Network.ChainMode)
	if configChainMode != chainMode {
		return fmt.Errorf("配置中的 chain_mode (%s) 与命令行参数 (%s) 不匹配", configChainMode, chainMode)
	}

	// 公链模式：验证不允许修改的链级参数
	if chainMode == "public" {
		// 公链的链级参数由内嵌配置锁定，用户不能修改
		// 这里可以添加更多验证逻辑，确保用户没有修改链级参数
		// 例如：chain_id、genesis、network_namespace 等必须与内嵌配置一致
		// 注意：由于公链使用内嵌配置，这里主要验证环境配置是否正确应用
	}

	// 联盟链/私链模式：验证必需字段
	if chainMode == "consortium" || chainMode == "private" {
		if appConfig.Network.ChainID == nil {
			return fmt.Errorf("联盟链/私链配置必须包含 network.chain_id")
		}
		if appConfig.Network.NetworkID == nil || *appConfig.Network.NetworkID == "" {
			return fmt.Errorf("联盟链/私链配置必须包含 network.network_id")
		}
		if appConfig.Network.NetworkNamespace == nil || *appConfig.Network.NetworkNamespace == "" {
			return fmt.Errorf("联盟链/私链配置必须包含 network.network_namespace")
		}
		if appConfig.Genesis == nil || appConfig.Genesis.Timestamp == 0 {
			return fmt.Errorf("联盟链/私链配置必须包含 genesis.timestamp（必须大于0）")
		}
		if len(appConfig.Genesis.Accounts) == 0 {
			return fmt.Errorf("联盟链/私链配置必须包含至少一个 genesis.accounts")
		}

		// 验证创世账户必需字段
		for i, account := range appConfig.Genesis.Accounts {
			if account.Address == "" {
				return fmt.Errorf("创世账户[%d]缺少 address 字段", i)
			}
			if account.InitialBalance == "" {
				return fmt.Errorf("创世账户[%d]缺少 initial_balance 字段", i)
			}
		}

		// 联盟链特定验证
		if chainMode == "consortium" {
			if appConfig.Node == nil || appConfig.Node.BootstrapPeers == nil || len(appConfig.Node.BootstrapPeers) == 0 {
				fmt.Println("⚠️  警告: 联盟链配置缺少 bootstrap_peers，建议配置至少一个引导节点")
			}
		}
	}

	// 验证链模式一致性（chain_mode vs security.permission_model vs security.access_control.mode）
	if err := validateChainModeConsistency(appConfig, chainMode); err != nil {
		return err
	}

	// 验证 mining.enable_aggregator 约束
	if err := validateMiningAggregatorConstraint(appConfig, chainMode, *appConfig.Environment); err != nil {
		return err
	}

	return nil
}

// validateChainModeConsistency 验证链模式一致性
// 验证 chain_mode、security.permission_model、security.access_control.mode 的一致性
func validateChainModeConsistency(appConfig *types.AppConfig, chainMode string) error {
	// 验证 security.permission_model 与 chain_mode 一致
	if appConfig.Security != nil && appConfig.Security.PermissionModel != nil {
		permissionModel := strings.ToLower(*appConfig.Security.PermissionModel)
		if permissionModel != chainMode {
			return fmt.Errorf("配置不一致: security.permission_model (%s) 与 network.chain_mode (%s) 不匹配", permissionModel, chainMode)
		}
	}

	// 验证 security.access_control.mode 与 chain_mode 一致
	if appConfig.Security != nil && appConfig.Security.AccessControl != nil && appConfig.Security.AccessControl.Mode != nil {
		accessControlMode := strings.ToLower(*appConfig.Security.AccessControl.Mode)
		var expectedMode string
		switch chainMode {
		case "public":
			expectedMode = "open"
		case "consortium":
			expectedMode = "allowlist"
		case "private":
			expectedMode = "psk"
		default:
			return fmt.Errorf("未知的链模式: %s", chainMode)
		}

		if accessControlMode != expectedMode {
			return fmt.Errorf("配置不一致: security.access_control.mode (%s) 与 network.chain_mode (%s) 不匹配，应为 %s", accessControlMode, chainMode, expectedMode)
		}
	}

	// 验证 node.host.gater.mode 与 chain_mode 一致
	if appConfig.Node != nil && appConfig.Node.Host != nil && appConfig.Node.Host.Gater != nil && appConfig.Node.Host.Gater.Mode != nil {
		gaterMode := strings.ToLower(*appConfig.Node.Host.Gater.Mode)
		var expectedGaterMode string
		switch chainMode {
		case "public":
			expectedGaterMode = "open"
		case "consortium", "private":
			expectedGaterMode = "allowlist"
		default:
			return fmt.Errorf("未知的链模式: %s", chainMode)
		}

		if gaterMode != expectedGaterMode {
			return fmt.Errorf("配置不一致: node.host.gater.mode (%s) 与 network.chain_mode (%s) 不匹配，应为 %s", gaterMode, chainMode, expectedGaterMode)
		}
	}

	// 验证链模式特定的安全配置
	switch chainMode {
	case "consortium":
		// 联盟链应该有 certificate_management 配置（建议，非强制）
		if appConfig.Security == nil || appConfig.Security.CertificateManagement == nil {
			fmt.Println("⚠️  警告: 联盟链配置缺少 security.certificate_management，建议配置 CA 证书管理")
		}
		// 联盟链不应该有 PSK 配置
		if appConfig.Security != nil && appConfig.Security.PSK != nil {
			return fmt.Errorf("配置错误: 联盟链不应该包含 security.psk 配置")
		}

	case "private":
		// 私有链应该有 PSK 配置（建议，非强制）
		if appConfig.Security == nil || appConfig.Security.PSK == nil || appConfig.Security.PSK.File == nil || *appConfig.Security.PSK.File == "" {
			fmt.Println("⚠️  警告: 私有链配置缺少 security.psk.file，建议配置 PSK 文件路径")
		}
		// 私有链不应该有 certificate_management 配置
		if appConfig.Security != nil && appConfig.Security.CertificateManagement != nil {
			return fmt.Errorf("配置错误: 私有链不应该包含 security.certificate_management 配置")
		}

	case "public":
		// 公有链不应该有 certificate_management 或 PSK 配置
		if appConfig.Security != nil {
			if appConfig.Security.CertificateManagement != nil {
				return fmt.Errorf("配置错误: 公有链不应该包含 security.certificate_management 配置")
			}
			if appConfig.Security.PSK != nil {
				return fmt.Errorf("配置错误: 公有链不应该包含 security.psk 配置")
			}
		}
	}

	return nil
}

// validateMiningAggregatorConstraint 验证 mining.enable_aggregator 约束
// 根据链模式和运行环境验证 enable_aggregator 的值是否符合约束：
// - public: 生产/测试环境必须为 true，开发环境允许 false（单节点模式）
// - consortium: 生产/测试环境必须为 true，开发环境允许 false（单节点模式）
// - private: 可以为 false 或 true
func validateMiningAggregatorConstraint(appConfig *types.AppConfig, chainMode string, environment string) error {
	if appConfig.Mining == nil || appConfig.Mining.EnableAggregator == nil {
		// 未配置时，根据链模式和运行环境设置默认值
		switch chainMode {
		case "public", "consortium":
			// 开发环境允许不配置（默认为 false），生产/测试环境必须显式配置为 true
			env := strings.ToLower(environment)
			if env != "dev" {
				return fmt.Errorf("配置错误: %s链模式在 %s 环境必须显式配置 mining.enable_aggregator 为 true", chainMode, environment)
			}
			// 开发环境允许不配置，默认为 false（单节点模式）
			return nil
		case "private":
			// 私有链允许不配置，默认为 false
			return nil
		default:
			return fmt.Errorf("未知的链模式: %s", chainMode)
		}
	}

	enableAggregator := *appConfig.Mining.EnableAggregator
	env := strings.ToLower(environment)

	switch chainMode {
	case "public":
		// 开发环境允许 false（单节点模式），生产/测试环境必须为 true
		if !enableAggregator && env != "dev" {
			return fmt.Errorf("配置错误: 公有链模式在 %s 环境 mining.enable_aggregator 必须为 true（生产/测试环境必须使用分布式聚合器）", environment)
		}

	case "consortium":
		// 开发环境允许 false（单节点模式），生产/测试环境必须为 true
		if !enableAggregator && env != "dev" {
			return fmt.Errorf("配置错误: 联盟链模式在 %s 环境 mining.enable_aggregator 必须为 true（多机构共识需要聚合器）", environment)
		}

	case "private":
		// 私有链允许 false（单节点模式）或 true（多节点模式）
		// 不进行强制验证

	default:
		return fmt.Errorf("未知的链模式: %s", chainMode)
	}

	return nil
}

// showHelpInfo 显示帮助信息
func showHelpInfo() {
	fmt.Println("weisyn-node - WES 区块链节点")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  weisyn-node --chain <mode> [选项]")
	fmt.Println()
	fmt.Println("链模式（必需）:")
	fmt.Println("  --chain public      公链模式")
	fmt.Println("                      - 无 --config：使用官方公共测试网 test-public-demo（内嵌 configs/chains/test-public-demo.json）")
	fmt.Println("                      - 有 --config：使用自建公链（用户配置，例如 configs/chains/dev-public-local.json）")
	fmt.Println("  --chain mainnet     官方主网别名（当前等同于 --chain public，无 --config，指向公共测试网）")
	fmt.Println("  --chain consortium  联盟链模式（必须提供 --config）")
	fmt.Println("  --chain private     私链模式（必须提供 --config）")
	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Println("  --config <path>     配置文件路径")
	fmt.Println("                      - 公链：可选（无则使用内嵌 test-public-demo，有则使用自建公链）")
	fmt.Println("                      - 联盟链/私链：必需")
	fmt.Println()
	fmt.Println("节点级配置（可选，不影响链级配置）:")
	fmt.Println("  --http-port <port>        HTTP端口（覆盖配置中的 http_port，用于 REST/JSON-RPC/WebSocket）")
	fmt.Println("  --grpc-port <port>        gRPC端口（覆盖配置中的 grpc_port）")
	fmt.Println("  --diagnostics-port <port> 诊断HTTP端口（覆盖配置中的 diagnostics_port，用于 pprof/diagnostics）")
	fmt.Println("  --data-dir <path>        数据目录（覆盖配置中的 data_root）")
	fmt.Println()
	fmt.Println("其他选项:")
	fmt.Println("  --help              显示此帮助信息")
	fmt.Println("  --version           显示版本信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 官方公共测试网（0 配置，environment 从内嵌 test-public-demo 读取）")
	fmt.Println("  weisyn-node --chain public")
	fmt.Println("  weisyn-node --chain mainnet  # 当前等同于上一条命令")
	fmt.Println()
	fmt.Println("  # 公共测试网（覆盖 HTTP 端口）")
	fmt.Println("  weisyn-node --chain public --http-port 28700")
	fmt.Println()
	fmt.Println("  # 公共测试网（覆盖多个端口，适配本机环境）")
	fmt.Println("  weisyn-node --chain public --http-port 28700 --grpc-port 28702 --diagnostics-port 28706")
	fmt.Println()
	fmt.Println("  # 公共测试网（指定端口）")
	fmt.Println("  weisyn-node --chain public --http-port 28700")
	fmt.Println()
	fmt.Println("  # 自建公链开发环境（例如 dev-public-local，本地单机挖矿）")
	fmt.Println("  weisyn-node --chain public --config ./configs/chains/dev-public-local.json")
	fmt.Println()
	fmt.Println("  # 联盟链模式（必须提供配置，配置文件中需包含 environment 字段）")
	fmt.Println("  weisyn-node --chain consortium --config ./my-consortium.json")
	fmt.Println()
	fmt.Println("  # 私链模式（必须提供配置，配置文件中需包含 environment 字段）")
	fmt.Println("  weisyn-node --chain private --config ./my-private.json")
	fmt.Println()
	fmt.Println("生成配置文件模板:")
	fmt.Println("  weisyn-node chain init --mode consortium --out ./my-consortium.json")
	fmt.Println("  weisyn-node chain init --mode private --out ./my-private.json")
	fmt.Println()
}
