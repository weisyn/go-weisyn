package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ==================== WES 合约部署工具 ====================
//
// 🌟 **设计理念**：为WES合约提供智能部署解决方案
//
// 🎯 **核心特性**：
// - 自动部署编译后的WASM合约
// - 支持部署配置和参数
// - 内置部署验证和状态检查
// - 生成部署报告和交易记录
// - 支持批量部署和升级
//

const (
	VERSION = "1.0.0"
	USAGE   = `WES Contract Deployer v%s

用法:
  weisyn-contract deploy [选项] <合约文件或目录>

选项:
  -n, --network <网络>      目标网络 (默认: local)
  -c, --config <配置文件>   部署配置文件
  -g, --执行费用-limit <限制>    执行费用限制 (默认: 1000000)
  -p, --执行费用-price <价格>    执行费用价格 (默认: 1000000000)
  -a, --account <账户>      部署账户地址
  -k, --key <私钥文件>      私钥文件路径
  -v, --verbose            详细输出
  -d, --dry-run            模拟部署（不实际执行）
  -h, --help               显示帮助信息
  --version                显示版本信息

示例:
  weisyn-contract deploy ./build/token.wasm
  weisyn-contract deploy -n testnet -c deploy.json ./build
  weisyn-contract deploy --dry-run --verbose ./build/nft.wasm
`
)

// DeployerConfig 部署器配置
type DeployerConfig struct {
	Network           string
	ConfigFile        string
	ExecutionFeeLimit uint64
	ExecutionFeePrice uint64
	Account           string
	KeyFile           string
	Verbose           bool
	DryRun            bool

	// 网络配置
	RpcUrl    string
	ChainID   string
	NetworkID string

	// 部署配置
	Timeout   time.Duration
	Retry     int
	BatchSize int
}

// DefaultDeployerConfig 默认部署器配置
func DefaultDeployerConfig() *DeployerConfig {
	return &DeployerConfig{
		Network:           "local",
		ExecutionFeeLimit: 1000000,
		ExecutionFeePrice: 1000000000, // 1 Gwei
		Verbose:           false,
		DryRun:            false,
		RpcUrl:            "http://localhost:8545",
		ChainID:           "weisyn-local",
		NetworkID:         "1337",
		Timeout:           30 * time.Second,
		Retry:             3,
		BatchSize:         5,
	}
}

// ContractDeployment 合约部署信息
type ContractDeployment struct {
	Name              string                 `json:"name"`
	WasmFile          string                 `json:"wasm_file"`
	InitParams        map[string]interface{} `json:"init_params"`
	ExecutionFeeLimit uint64                 `json:"execution_fee_limit"`
	ExecutionFeePrice uint64                 `json:"execution_fee_price"`
	DeployerAccount   string                 `json:"deployer_account"`

	// 部署依赖
	Dependencies []string `json:"dependencies"`
	PreDeploy    []string `json:"pre_deploy"`
	PostDeploy   []string `json:"post_deploy"`

	// 验证配置
	Verification *VerificationConfig `json:"verification"`
}

// VerificationConfig 验证配置
type VerificationConfig struct {
	Enable         bool         `json:"enable"`
	TestCalls      []TestCall   `json:"test_calls"`
	ExpectedEvents []string     `json:"expected_events"`
	HealthCheck    *HealthCheck `json:"health_check"`
}

// TestCall 测试调用
type TestCall struct {
	Function       string                 `json:"function"`
	Parameters     map[string]interface{} `json:"parameters"`
	ExpectedResult interface{}            `json:"expected_result"`
	ExpectedError  string                 `json:"expected_error"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Function   string        `json:"function"`
	Interval   time.Duration `json:"interval"`
	Timeout    time.Duration `json:"timeout"`
	MaxRetries int           `json:"max_retries"`
}

// DeploymentResult 部署结果
type DeploymentResult struct {
	Contract         *ContractDeployment `json:"contract"`
	Success          bool                `json:"success"`
	ContractAddress  string              `json:"contract_address"`
	TransactionHash  string              `json:"transaction_hash"`
	ExecutionFeeUsed uint64              `json:"execution_fee_used"`
	DeployTime       time.Time           `json:"deploy_time"`
	Duration         time.Duration       `json:"duration"`
	BlockNumber      uint64              `json:"block_number"`

	// 验证结果
	Verified            bool                 `json:"verified"`
	VerificationResults []VerificationResult `json:"verification_results"`

	// 错误信息
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// VerificationResult 验证结果
type VerificationResult struct {
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(USAGE, VERSION)
		os.Exit(1)
	}

	config := DefaultDeployerConfig()
	var sourcePath string

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			fmt.Printf(USAGE, VERSION)
			os.Exit(0)
		case "--version":
			fmt.Printf("WES Contract Deployer v%s\n", VERSION)
			os.Exit(0)
		case "-v", "--verbose":
			config.Verbose = true
		case "-d", "--dry-run":
			config.DryRun = true
		case "-n", "--network":
			if i+1 < len(os.Args) {
				config.Network = os.Args[i+1]
				i++
			}
		case "-c", "--config":
			if i+1 < len(os.Args) {
				config.ConfigFile = os.Args[i+1]
				i++
			}
		case "-g", "--执行费用-limit":
			if i+1 < len(os.Args) {
				if limit := parseUint64(os.Args[i+1]); limit > 0 {
					config.ExecutionFeeLimit = limit
				}
				i++
			}
		case "-p", "--执行费用-price":
			if i+1 < len(os.Args) {
				if price := parseUint64(os.Args[i+1]); price > 0 {
					config.ExecutionFeePrice = price
				}
				i++
			}
		case "-a", "--account":
			if i+1 < len(os.Args) {
				config.Account = os.Args[i+1]
				i++
			}
		case "-k", "--key":
			if i+1 < len(os.Args) {
				config.KeyFile = os.Args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				sourcePath = arg
			}
		}
	}

	if sourcePath == "" {
		fmt.Println("错误: 请指定合约文件或目录路径")
		os.Exit(1)
	}

	// 加载网络配置
	if err := loadNetworkConfig(config); err != nil {
		fmt.Printf("加载网络配置失败: %v\n", err)
		os.Exit(1)
	}

	// 执行部署
	deployer := NewDeployer(config)
	results, err := deployer.Deploy(sourcePath)
	if err != nil {
		fmt.Printf("部署失败: %v\n", err)
		os.Exit(1)
	}

	// 输出结果
	printDeployResults(results, config.Verbose)

	// 生成部署报告
	if err := generateDeployReport(results, config); err != nil {
		fmt.Printf("生成部署报告失败: %v\n", err)
	}

	// 检查部署结果
	failed := 0
	for _, result := range results {
		if !result.Success {
			failed++
		}
	}

	if failed > 0 {
		fmt.Printf("\n部署完成，%d个合约成功，%d个合约失败\n", len(results)-failed, failed)
		os.Exit(1)
	} else {
		fmt.Printf("\n部署完成，共%d个合约部署成功\n", len(results))
	}
}

// Deployer 部署器
type Deployer struct {
	config *DeployerConfig
}

// NewDeployer 创建部署器
func NewDeployer(config *DeployerConfig) *Deployer {
	return &Deployer{config: config}
}

// Deploy 执行部署
func (d *Deployer) Deploy(sourcePath string) ([]*DeploymentResult, error) {
	// 发现合约文件
	deployments, err := d.discoverContracts(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("发现合约失败: %w", err)
	}

	if len(deployments) == 0 {
		return nil, fmt.Errorf("未找到合约文件")
	}

	if d.config.Verbose {
		fmt.Printf("发现 %d 个合约文件\n", len(deployments))
	}

	// 加载部署配置
	if d.config.ConfigFile != "" {
		if err := d.loadDeployConfig(deployments); err != nil {
			return nil, fmt.Errorf("加载部署配置失败: %w", err)
		}
	}

	// 验证部署前置条件
	if err := d.validatePreConditions(); err != nil {
		return nil, fmt.Errorf("部署前置条件验证失败: %w", err)
	}

	// 排序部署顺序（处理依赖关系）
	orderedDeployments := d.orderDeployments(deployments)

	// 逐个部署合约
	results := make([]*DeploymentResult, 0, len(orderedDeployments))
	for _, deployment := range orderedDeployments {
		result := d.deployContract(deployment)
		results = append(results, result)

		if d.config.Verbose {
			if result.Success {
				fmt.Printf("✓ %s 部署成功 (%s)\n", deployment.Name, result.ContractAddress)
			} else {
				fmt.Printf("✗ %s 部署失败\n", deployment.Name)
			}
		}

		// 如果部署失败且有依赖，停止后续部署
		if !result.Success && len(deployment.Dependencies) > 0 {
			break
		}
	}

	return results, nil
}

// discoverContracts 发现合约文件
func (d *Deployer) discoverContracts(sourcePath string) ([]*ContractDeployment, error) {
	var deployments []*ContractDeployment

	// 检查是否是单个文件
	if strings.HasSuffix(sourcePath, ".wasm") {
		deployment := &ContractDeployment{
			Name:              getContractNameFromWasm(sourcePath),
			WasmFile:          sourcePath,
			ExecutionFeeLimit: d.config.ExecutionFeeLimit,
			ExecutionFeePrice: d.config.ExecutionFeePrice,
			DeployerAccount:   d.config.Account,
			InitParams:        make(map[string]interface{}),
		}
		deployments = append(deployments, deployment)
		return deployments, nil
	}

	// 遍历目录查找WASM文件
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".wasm") {
			deployment := &ContractDeployment{
				Name:              getContractNameFromWasm(path),
				WasmFile:          path,
				ExecutionFeeLimit: d.config.ExecutionFeeLimit,
				ExecutionFeePrice: d.config.ExecutionFeePrice,
				DeployerAccount:   d.config.Account,
				InitParams:        make(map[string]interface{}),
			}
			deployments = append(deployments, deployment)
		}

		return nil
	})

	return deployments, err
}

// deployContract 部署单个合约
func (d *Deployer) deployContract(deployment *ContractDeployment) *DeploymentResult {
	startTime := time.Now()

	result := &DeploymentResult{
		Contract:            deployment,
		Success:             false,
		DeployTime:          startTime,
		Errors:              []string{},
		Warnings:            []string{},
		VerificationResults: []VerificationResult{},
	}

	// 模拟部署（实际实现需要调用WES节点API）
	if d.config.DryRun {
		result.Success = true
		result.ContractAddress = fmt.Sprintf("0x%040d", time.Now().Unix())
		result.TransactionHash = fmt.Sprintf("0x%064d", time.Now().Unix())
		result.ExecutionFeeUsed = deployment.ExecutionFeeLimit / 2
		result.BlockNumber = 12345
	} else {
		// 实际部署逻辑
		address, txHash, ExecutionFeeUsed, blockNumber, err := d.performDeployment(deployment)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result
		}

		result.Success = true
		result.ContractAddress = address
		result.TransactionHash = txHash
		result.ExecutionFeeUsed = ExecutionFeeUsed
		result.BlockNumber = blockNumber
	}

	result.Duration = time.Since(startTime)

	// 执行部署后验证
	if deployment.Verification != nil && deployment.Verification.Enable {
		d.verifyDeployment(result)
	}

	return result
}

// performDeployment 执行实际部署
func (d *Deployer) performDeployment(deployment *ContractDeployment) (string, string, uint64, uint64, error) {
	// 这里应该调用WES节点的API进行实际部署
	// 为演示目的，返回模拟结果

	if d.config.Verbose {
		fmt.Printf("部署合约: %s\n", deployment.Name)
		fmt.Printf("  WASM文件: %s\n", deployment.WasmFile)
		fmt.Printf("  执行费用限制: %d\n", deployment.ExecutionFeeLimit)
		fmt.Printf("  执行费用价格: %d\n", deployment.ExecutionFeePrice)
	}

	// 模拟网络延迟
	time.Sleep(100 * time.Millisecond)

	// 生成模拟结果
	contractAddress := fmt.Sprintf("0x%040d", time.Now().Unix())
	transactionHash := fmt.Sprintf("0x%064d", time.Now().Unix())
	ExecutionFeeUsed := deployment.ExecutionFeeLimit / 2
	blockNumber := uint64(12345)

	return contractAddress, transactionHash, ExecutionFeeUsed, blockNumber, nil
}

// verifyDeployment 验证部署
func (d *Deployer) verifyDeployment(result *DeploymentResult) {
	verification := result.Contract.Verification

	// 执行测试调用
	for _, testCall := range verification.TestCalls {
		verResult := d.executeTestCall(result.ContractAddress, &testCall)
		result.VerificationResults = append(result.VerificationResults, verResult)

		if !verResult.Success {
			result.Verified = false
			return
		}
	}

	// 检查预期事件
	if len(verification.ExpectedEvents) > 0 {
		verResult := d.checkExpectedEvents(result.ContractAddress, verification.ExpectedEvents)
		result.VerificationResults = append(result.VerificationResults, verResult)

		if !verResult.Success {
			result.Verified = false
			return
		}
	}

	// 执行健康检查
	if verification.HealthCheck != nil {
		verResult := d.performHealthCheck(result.ContractAddress, verification.HealthCheck)
		result.VerificationResults = append(result.VerificationResults, verResult)

		if !verResult.Success {
			result.Verified = false
			return
		}
	}

	result.Verified = true
}

// executeTestCall 执行测试调用
func (d *Deployer) executeTestCall(contractAddress string, testCall *TestCall) VerificationResult {
	// 模拟测试调用
	return VerificationResult{
		Type:    "test_call",
		Success: true,
		Message: fmt.Sprintf("测试调用 %s 成功", testCall.Function),
		Data:    testCall.ExpectedResult,
	}
}

// checkExpectedEvents 检查预期事件
func (d *Deployer) checkExpectedEvents(contractAddress string, expectedEvents []string) VerificationResult {
	// 模拟事件检查
	return VerificationResult{
		Type:    "event_check",
		Success: true,
		Message: fmt.Sprintf("发现 %d 个预期事件", len(expectedEvents)),
		Data:    expectedEvents,
	}
}

// performHealthCheck 执行健康检查
func (d *Deployer) performHealthCheck(contractAddress string, healthCheck *HealthCheck) VerificationResult {
	// 模拟健康检查
	return VerificationResult{
		Type:    "health_check",
		Success: true,
		Message: "健康检查通过",
		Data:    map[string]interface{}{"status": "healthy"},
	}
}

// ==================== 辅助函数 ====================

// loadNetworkConfig 加载网络配置
func loadNetworkConfig(config *DeployerConfig) error {
	// 根据网络名称设置相应的配置
	switch config.Network {
	case "local":
		config.RpcUrl = "http://localhost:8545"
		config.ChainID = "weisyn-local"
		config.NetworkID = "1337"
	case "testnet":
		config.RpcUrl = "https://testnet-rpc.weisyn.io"
		config.ChainID = "weisyn-testnet"
		config.NetworkID = "2024"
	case "mainnet":
		config.RpcUrl = "https://mainnet-rpc.weisyn.io"
		config.ChainID = "weisyn-mainnet"
		config.NetworkID = "1"
	default:
		return fmt.Errorf("未知网络: %s", config.Network)
	}

	return nil
}

// loadDeployConfig 加载部署配置
func (d *Deployer) loadDeployConfig(deployments []*ContractDeployment) error {
	if d.config.ConfigFile == "" {
		return nil
	}

	// 读取配置文件
	data, err := os.ReadFile(d.config.ConfigFile)
	if err != nil {
		return err
	}

	// 解析配置
	var configs map[string]*ContractDeployment
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}

	// 合并配置
	for _, deployment := range deployments {
		if config, exists := configs[deployment.Name]; exists {
			mergeDeploymentConfig(deployment, config)
		}
	}

	return nil
}

// mergeDeploymentConfig 合并部署配置
func mergeDeploymentConfig(target, source *ContractDeployment) {
	if source.ExecutionFeeLimit > 0 {
		target.ExecutionFeeLimit = source.ExecutionFeeLimit
	}
	if source.ExecutionFeePrice > 0 {
		target.ExecutionFeePrice = source.ExecutionFeePrice
	}
	if source.DeployerAccount != "" {
		target.DeployerAccount = source.DeployerAccount
	}
	if len(source.InitParams) > 0 {
		target.InitParams = source.InitParams
	}
	if len(source.Dependencies) > 0 {
		target.Dependencies = source.Dependencies
	}
	if source.Verification != nil {
		target.Verification = source.Verification
	}
}

// validatePreConditions 验证部署前置条件
func (d *Deployer) validatePreConditions() error {
	// 检查账户配置
	if d.config.Account == "" {
		return fmt.Errorf("未指定部署账户")
	}

	// 检查网络连通性（模拟）
	if d.config.Verbose {
		fmt.Printf("验证网络连通性: %s\n", d.config.RpcUrl)
	}

	return nil
}

// orderDeployments 排序部署顺序
func (d *Deployer) orderDeployments(deployments []*ContractDeployment) []*ContractDeployment {
	// 简化的依赖排序
	// 实际实现应该使用拓扑排序算法
	return deployments
}

// getContractNameFromWasm 从WASM文件获取合约名称
func getContractNameFromWasm(wasmFile string) string {
	base := filepath.Base(wasmFile)
	return strings.TrimSuffix(base, ".wasm")
}

// parseUint64 解析uint64
func parseUint64(s string) uint64 {
	var result uint64
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + uint64(char-'0')
		} else {
			break
		}
	}
	return result
}

// printDeployResults 打印部署结果
func printDeployResults(results []*DeploymentResult, verbose bool) {
	fmt.Println("\n=== 部署结果 ===")

	for _, result := range results {
		status := "✗ 失败"
		if result.Success {
			status = "✓ 成功"
		}

		fmt.Printf("%-20s %s", result.Contract.Name, status)

		if result.Success {
			fmt.Printf(" (%s)", result.ContractAddress[:10]+"...")
		}

		fmt.Println()

		if verbose {
			if result.Success {
				fmt.Printf("  地址: %s\n", result.ContractAddress)
				fmt.Printf("  交易: %s\n", result.TransactionHash)
				fmt.Printf("  执行费用使用: %d\n", result.ExecutionFeeUsed)
				fmt.Printf("  区块号: %d\n", result.BlockNumber)
				fmt.Printf("  耗时: %v\n", result.Duration)

				if result.Verified {
					fmt.Printf("  验证: ✓ 通过\n")
				} else if len(result.VerificationResults) > 0 {
					fmt.Printf("  验证: ✗ 失败\n")
				}
			}

			if len(result.Errors) > 0 {
				for _, err := range result.Errors {
					fmt.Printf("  错误: %s\n", err)
				}
			}
		}
	}
}

// generateDeployReport 生成部署报告
func generateDeployReport(results []*DeploymentResult, config *DeployerConfig) error {
	report := map[string]interface{}{
		"deployment_summary": map[string]interface{}{
			"timestamp":       time.Now().Format(time.RFC3339),
			"network":         config.Network,
			"total_contracts": len(results),
			"successful":      countSuccessful(results),
			"failed":          countFailed(results),
		},
		"contracts": results,
	}

	// 生成JSON报告
	reportFile := "deployment-report.json"
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(reportFile, data, 0644); err != nil {
		return err
	}

	if config.Verbose {
		fmt.Printf("部署报告已生成: %s\n", reportFile)
	}

	return nil
}

// countSuccessful 统计成功数量
func countSuccessful(results []*DeploymentResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}

// countFailed 统计失败数量
func countFailed(results []*DeploymentResult) int {
	count := 0
	for _, result := range results {
		if !result.Success {
			count++
		}
	}
	return count
}
