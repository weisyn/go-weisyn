package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ==================== WES 合约编译工具 ====================
//
// 🌟 **设计理念**：为WES合约提供一键编译解决方案
//
// 🎯 **核心特性**：
// - 自动检测Go合约源码
// - 使用TinyGo编译到WASM
// - 内置优化和验证
// - 支持批量编译
// - 生成部署清单
//

const (
	VERSION = "1.0.0"
	USAGE   = `WES Contract Compiler v%s

用法:
  weisyn-contract compile [选项] <合约目录或文件>

选项:
  -o, --output <目录>     输出目录 (默认: ./build)
  -t, --target <目标>     编译目标 (默认: wasm)
  -O, --optimize <级别>   优化级别 (0-3, 默认: 2)
  -v, --verbose          详细输出
  -h, --help             显示帮助信息
  --version              显示版本信息

示例:
  weisyn-contract compile ./contracts/token
  weisyn-contract compile -o ./dist -O 3 ./contracts
  weisyn-contract compile --verbose ./contracts/nft/nft.go
`
)

// CompilerConfig 编译器配置
type CompilerConfig struct {
	SourcePath    string
	OutputDir     string
	Target        string
	OptimizeLevel int
	Verbose       bool

	// TinyGo特定配置
	TinyGoPath string
	GoRoot     string
	GoCache    string

	// WASM配置
	WasmOpt   bool
	WasmSize  bool
	WasmStrip bool
}

// DefaultCompilerConfig 默认编译器配置
func DefaultCompilerConfig() *CompilerConfig {
	return &CompilerConfig{
		OutputDir:     "./build",
		Target:        "wasm",
		OptimizeLevel: 2,
		Verbose:       false,
		TinyGoPath:    "tinygo",
		WasmOpt:       true,
		WasmSize:      true,
		WasmStrip:     true,
	}
}

// ContractInfo 合约信息
type ContractInfo struct {
	Name       string
	SourceFile string
	OutputFile string
	Package    string
	Version    string
}

// CompilerResult 编译结果
type CompilerResult struct {
	Contract   *ContractInfo
	Success    bool
	OutputFile string
	FileSize   int64
	BuildTime  float64
	Errors     []string
	Warnings   []string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(USAGE, VERSION)
		os.Exit(1)
	}

	config := DefaultCompilerConfig()
	var sourcePath string

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			fmt.Printf(USAGE, VERSION)
			os.Exit(0)
		case "--version":
			fmt.Printf("WES Contract Compiler v%s\n", VERSION)
			os.Exit(0)
		case "-v", "--verbose":
			config.Verbose = true
		case "-o", "--output":
			if i+1 < len(os.Args) {
				config.OutputDir = os.Args[i+1]
				i++
			}
		case "-t", "--target":
			if i+1 < len(os.Args) {
				config.Target = os.Args[i+1]
				i++
			}
		case "-O", "--optimize":
			if i+1 < len(os.Args) {
				if level := parseOptimizeLevel(os.Args[i+1]); level >= 0 {
					config.OptimizeLevel = level
				}
				i++
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				sourcePath = arg
			}
		}
	}

	if sourcePath == "" {
		fmt.Println("错误: 请指定合约源码路径")
		os.Exit(1)
	}

	config.SourcePath = sourcePath

	// 执行编译
	compiler := NewCompiler(config)
	results, err := compiler.Compile()
	if err != nil {
		fmt.Printf("编译失败: %v\n", err)
		os.Exit(1)
	}

	// 输出结果
	printResults(results, config.Verbose)

	// 检查是否有编译失败的合约
	failed := 0
	for _, result := range results {
		if !result.Success {
			failed++
		}
	}

	if failed > 0 {
		fmt.Printf("\n编译完成，%d个合约成功，%d个合约失败\n", len(results)-failed, failed)
		os.Exit(1)
	} else {
		fmt.Printf("\n编译完成，共%d个合约编译成功\n", len(results))
	}
}

// Compiler 编译器
type Compiler struct {
	config *CompilerConfig
}

// NewCompiler 创建编译器
func NewCompiler(config *CompilerConfig) *Compiler {
	return &Compiler{config: config}
}

// Compile 执行编译
func (c *Compiler) Compile() ([]*CompilerResult, error) {
	// 发现合约文件
	contracts, err := c.discoverContracts()
	if err != nil {
		return nil, fmt.Errorf("发现合约失败: %w", err)
	}

	if len(contracts) == 0 {
		return nil, fmt.Errorf("未找到合约文件")
	}

	if c.config.Verbose {
		fmt.Printf("发现 %d 个合约文件\n", len(contracts))
	}

	// 创建输出目录
	if err := os.MkdirAll(c.config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 编译每个合约
	results := make([]*CompilerResult, 0, len(contracts))
	for _, contract := range contracts {
		result := c.compileContract(contract)
		results = append(results, result)

		if c.config.Verbose {
			if result.Success {
				fmt.Printf("✓ %s 编译成功\n", contract.Name)
			} else {
				fmt.Printf("✗ %s 编译失败\n", contract.Name)
			}
		}
	}

	return results, nil
}

// discoverContracts 发现合约文件
func (c *Compiler) discoverContracts() ([]*ContractInfo, error) {
	var contracts []*ContractInfo

	err := filepath.Walk(c.config.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理Go文件
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// 跳过测试文件
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// 检查是否是合约文件（包含main函数或export函数）
		if isContractFile(path) {
			contract := &ContractInfo{
				Name:       getContractName(path),
				SourceFile: path,
				Package:    getPackageName(path),
			}

			// 生成输出文件路径
			relPath, _ := filepath.Rel(c.config.SourcePath, path)
			outputName := strings.TrimSuffix(relPath, ".go") + ".wasm"
			contract.OutputFile = filepath.Join(c.config.OutputDir, outputName)

			contracts = append(contracts, contract)
		}

		return nil
	})

	return contracts, err
}

// compileContract 编译单个合约
func (c *Compiler) compileContract(contract *ContractInfo) *CompilerResult {
	result := &CompilerResult{
		Contract: contract,
		Success:  false,
		Errors:   []string{},
		Warnings: []string{},
	}

	// 构建TinyGo编译命令
	cmd := c.buildTinyGoCommand(contract)

	if c.config.Verbose {
		fmt.Printf("执行命令: %s\n", strings.Join(cmd.Args, " "))
	}

	// 执行编译
	output, err := cmd.CombinedOutput()

	if err != nil {
		result.Errors = append(result.Errors, string(output))
		return result
	}

	// 检查输出文件
	if info, err := os.Stat(contract.OutputFile); err == nil {
		result.Success = true
		result.OutputFile = contract.OutputFile
		result.FileSize = info.Size()
	} else {
		result.Errors = append(result.Errors, "输出文件未生成")
		return result
	}

	// 后处理优化
	if c.config.WasmOpt {
		c.optimizeWasm(contract.OutputFile)
	}

	return result
}

// buildTinyGoCommand 构建TinyGo编译命令
func (c *Compiler) buildTinyGoCommand(contract *ContractInfo) *exec.Cmd {
	args := []string{
		"build",
		"-target", "wasm",
		"-o", contract.OutputFile,
	}

	// 添加优化级别
	if c.config.OptimizeLevel > 0 {
		args = append(args, "-opt", fmt.Sprintf("%d", c.config.OptimizeLevel))
	}

	// 添加其他选项
	if c.config.WasmSize {
		args = append(args, "-size", "short")
	}

	// 添加源文件
	args = append(args, contract.SourceFile)

	cmd := exec.Command(c.config.TinyGoPath, args...)

	// 设置环境变量
	cmd.Env = os.Environ()
	if c.config.GoRoot != "" {
		cmd.Env = append(cmd.Env, "GOROOT="+c.config.GoRoot)
	}
	if c.config.GoCache != "" {
		cmd.Env = append(cmd.Env, "GOCACHE="+c.config.GoCache)
	}

	return cmd
}

// optimizeWasm 优化WASM文件
func (c *Compiler) optimizeWasm(wasmFile string) error {
	// 尝试使用wasm-opt优化
	cmd := exec.Command("wasm-opt", "-Oz", wasmFile, "-o", wasmFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if c.config.Verbose {
			fmt.Printf("wasm-opt优化失败: %s\n", string(output))
		}
		return err
	}

	if c.config.Verbose {
		fmt.Printf("wasm-opt优化完成: %s\n", wasmFile)
	}

	return nil
}

// ==================== 辅助函数 ====================

// isContractFile 检查是否是合约文件
func isContractFile(filename string) bool {
	// 简化检查：查找main包和export注释
	content, err := os.ReadFile(filename)
	if err != nil {
		return false
	}

	source := string(content)
	return strings.Contains(source, "package main") &&
		(strings.Contains(source, "//export") || strings.Contains(source, "func main()"))
}

// getContractName 获取合约名称
func getContractName(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, ".go")
}

// getPackageName 获取包名
func getPackageName(filename string) string {
	// 简化实现：从目录名获取
	dir := filepath.Dir(filename)
	return filepath.Base(dir)
}

// parseOptimizeLevel 解析优化级别
func parseOptimizeLevel(s string) int {
	switch s {
	case "0":
		return 0
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	default:
		return -1
	}
}

// printResults 打印编译结果
func printResults(results []*CompilerResult, verbose bool) {
	fmt.Println("\n=== 编译结果 ===")

	for _, result := range results {
		status := "✗ 失败"
		if result.Success {
			status = "✓ 成功"
		}

		fmt.Printf("%-20s %s", result.Contract.Name, status)

		if result.Success {
			fmt.Printf(" (%d bytes)", result.FileSize)
		}

		fmt.Println()

		if verbose && len(result.Errors) > 0 {
			for _, err := range result.Errors {
				fmt.Printf("  错误: %s\n", err)
			}
		}

		if verbose && len(result.Warnings) > 0 {
			for _, warn := range result.Warnings {
				fmt.Printf("  警告: %s\n", warn)
			}
		}
	}
}
