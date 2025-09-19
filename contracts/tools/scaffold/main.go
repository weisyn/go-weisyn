package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ==================== WES 合约脚手架工具 ====================
//
// 🌟 **设计理念**：为WES合约开发提供快速项目初始化
//
// 🎯 **核心特性**：
// - 快速生成标准合约项目结构
// - 内置多种合约模板选择
// - 自动配置构建和部署脚本
// - 生成完整的开发环境
// - 包含测试和文档模板
//

const (
	VERSION = "1.0.0"
	USAGE   = `WES Contract Scaffold v%s

用法:
  weisyn-contract init [选项] <项目名称>

选项:
  -t, --template <模板>     合约模板 (token|nft|governance|defi|custom)
  -d, --directory <目录>    项目目录 (默认: 当前目录)
  -a, --author <作者>       项目作者
  -l, --license <许可证>    项目许可证 (默认: MIT)
  -v, --verbose            详细输出
  -f, --force              强制覆盖现有文件
  -h, --help               显示帮助信息
  --version                显示版本信息

模板类型:
  token       - ERC20风格的代币合约
  nft         - ERC721风格的NFT合约
  governance  - DAO治理合约
  defi        - DeFi AMM DEX合约
  custom      - 自定义基础合约

示例:
  weisyn-contract init MyToken
  weisyn-contract init -t nft -a "John Doe" MyNFT
  weisyn-contract init -t governance --license Apache-2.0 MyDAO
`
)

// ScaffoldConfig 脚手架配置
type ScaffoldConfig struct {
	ProjectName string
	Template    string
	Directory   string
	Author      string
	License     string
	Verbose     bool
	Force       bool

	// 生成选项
	IncludeTests   bool
	IncludeDocs    bool
	IncludeScripts bool
	IncludeExample bool
}

// DefaultScaffoldConfig 默认脚手架配置
func DefaultScaffoldConfig() *ScaffoldConfig {
	return &ScaffoldConfig{
		Template:       "token",
		Directory:      ".",
		Author:         "WES Developer",
		License:        "MIT",
		Verbose:        false,
		Force:          false,
		IncludeTests:   true,
		IncludeDocs:    true,
		IncludeScripts: true,
		IncludeExample: true,
	}
}

// ProjectTemplate 项目模板
type ProjectTemplate struct {
	Name        string
	Description string
	Files       map[string]string
	Directories []string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(USAGE, VERSION)
		os.Exit(1)
	}

	config := DefaultScaffoldConfig()
	var projectName string

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			fmt.Printf(USAGE, VERSION)
			os.Exit(0)
		case "--version":
			fmt.Printf("WES Contract Scaffold v%s\n", VERSION)
			os.Exit(0)
		case "-v", "--verbose":
			config.Verbose = true
		case "-f", "--force":
			config.Force = true
		case "-t", "--template":
			if i+1 < len(os.Args) {
				config.Template = os.Args[i+1]
				i++
			}
		case "-d", "--directory":
			if i+1 < len(os.Args) {
				config.Directory = os.Args[i+1]
				i++
			}
		case "-a", "--author":
			if i+1 < len(os.Args) {
				config.Author = os.Args[i+1]
				i++
			}
		case "-l", "--license":
			if i+1 < len(os.Args) {
				config.License = os.Args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				projectName = arg
			}
		}
	}

	if projectName == "" {
		fmt.Println("错误: 请指定项目名称")
		os.Exit(1)
	}

	config.ProjectName = projectName

	// 执行脚手架
	scaffold := NewScaffold(config)
	if err := scaffold.Generate(); err != nil {
		fmt.Printf("生成项目失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ 项目 '%s' 生成成功！\n", projectName)
	fmt.Printf("\n下一步:\n")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  weisyn-contract compile\n")
	fmt.Printf("  weisyn-contract deploy\n")
}

// Scaffold 脚手架
type Scaffold struct {
	config   *ScaffoldConfig
	template *ProjectTemplate
}

// NewScaffold 创建脚手架
func NewScaffold(config *ScaffoldConfig) *Scaffold {
	template := getTemplate(config.Template)
	return &Scaffold{
		config:   config,
		template: template,
	}
}

// Generate 生成项目
func (s *Scaffold) Generate() error {
	// 创建项目目录
	projectDir := filepath.Join(s.config.Directory, s.config.ProjectName)
	if err := s.createProjectDirectory(projectDir); err != nil {
		return err
	}

	// 创建子目录
	if err := s.createDirectories(projectDir); err != nil {
		return err
	}

	// 生成文件
	if err := s.generateFiles(projectDir); err != nil {
		return err
	}

	// 生成构建脚本
	if s.config.IncludeScripts {
		if err := s.generateBuildScripts(projectDir); err != nil {
			return err
		}
	}

	// 生成测试文件
	if s.config.IncludeTests {
		if err := s.generateTests(projectDir); err != nil {
			return err
		}
	}

	// 生成文档
	if s.config.IncludeDocs {
		if err := s.generateDocs(projectDir); err != nil {
			return err
		}
	}

	return nil
}

// createProjectDirectory 创建项目目录
func (s *Scaffold) createProjectDirectory(projectDir string) error {
	// 检查目录是否存在
	if _, err := os.Stat(projectDir); err == nil {
		if !s.config.Force {
			return fmt.Errorf("目录 %s 已存在，使用 -f 强制覆盖", projectDir)
		}
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if s.config.Verbose {
		fmt.Printf("创建项目目录: %s\n", projectDir)
	}

	return nil
}

// createDirectories 创建子目录
func (s *Scaffold) createDirectories(projectDir string) error {
	dirs := []string{
		"src",
		"tests",
		"docs",
		"scripts",
		"build",
		"deploy",
		"examples",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(projectDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}

		if s.config.Verbose {
			fmt.Printf("创建目录: %s\n", dir)
		}
	}

	return nil
}

// generateFiles 生成文件
func (s *Scaffold) generateFiles(projectDir string) error {
	// 生成主合约文件
	contractFile := filepath.Join(projectDir, "src", s.config.ProjectName+".go")
	contractContent := s.generateContractContent()
	if err := s.writeFile(contractFile, contractContent); err != nil {
		return err
	}

	// 生成go.mod文件
	goModFile := filepath.Join(projectDir, "go.mod")
	goModContent := s.generateGoModContent()
	if err := s.writeFile(goModFile, goModContent); err != nil {
		return err
	}

	// 生成README.md文件
	readmeFile := filepath.Join(projectDir, "README.md")
	readmeContent := s.generateReadmeContent()
	if err := s.writeFile(readmeFile, readmeContent); err != nil {
		return err
	}

	// 生成.gitignore文件
	gitignoreFile := filepath.Join(projectDir, ".gitignore")
	gitignoreContent := s.generateGitignoreContent()
	if err := s.writeFile(gitignoreFile, gitignoreContent); err != nil {
		return err
	}

	// 生成LICENSE文件
	licenseFile := filepath.Join(projectDir, "LICENSE")
	licenseContent := s.generateLicenseContent()
	if err := s.writeFile(licenseFile, licenseContent); err != nil {
		return err
	}

	return nil
}

// generateBuildScripts 生成构建脚本
func (s *Scaffold) generateBuildScripts(projectDir string) error {
	// 生成构建脚本
	buildScript := filepath.Join(projectDir, "scripts", "build.sh")
	buildContent := s.generateBuildScriptContent()
	if err := s.writeFile(buildScript, buildContent); err != nil {
		return err
	}

	// 设置执行权限
	if err := os.Chmod(buildScript, 0755); err != nil {
		return err
	}

	// 生成部署配置
	deployConfig := filepath.Join(projectDir, "deploy", "config.json")
	deployContent := s.generateDeployConfigContent()
	if err := s.writeFile(deployConfig, deployContent); err != nil {
		return err
	}

	return nil
}

// generateTests 生成测试文件
func (s *Scaffold) generateTests(projectDir string) error {
	testFile := filepath.Join(projectDir, "tests", s.config.ProjectName+"_test.go")
	testContent := s.generateTestContent()
	return s.writeFile(testFile, testContent)
}

// generateDocs 生成文档
func (s *Scaffold) generateDocs(projectDir string) error {
	apiDoc := filepath.Join(projectDir, "docs", "API.md")
	apiContent := s.generateAPIDocContent()
	return s.writeFile(apiDoc, apiContent)
}

// writeFile 写入文件
func (s *Scaffold) writeFile(filename, content string) error {
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件 %s 失败: %w", filename, err)
	}

	if s.config.Verbose {
		fmt.Printf("生成文件: %s\n", filename)
	}

	return nil
}

// ==================== 内容生成函数 ====================

// generateContractContent 生成合约内容
func (s *Scaffold) generateContractContent() string {
	switch s.config.Template {
	case "token":
		return s.generateTokenContract()
	case "nft":
		return s.generateNFTContract()
	case "governance":
		return s.generateGovernanceContract()
	case "defi":
		return s.generateDeFiContract()
	default:
		return s.generateCustomContract()
	}
}

// generateTokenContract 生成代币合约
func (s *Scaffold) generateTokenContract() string {
	return fmt.Sprintf(`//go:build tinygo.wasm

package main

import (
	"unsafe"
)

// ==================== %s 代币合约 ====================
//
// 作者: %s
// 许可证: %s
// 创建时间: %s
//
// 基于WES URES模型的标准代币合约

// 标准错误码
const (
	SUCCESS                    = 0
	ERROR_INVALID_PARAMS       = 1
	ERROR_INSUFFICIENT_BALANCE = 2
	ERROR_UNAUTHORIZED         = 3
	ERROR_UNKNOWN             = 999
)

// 宿主函数声明
//go:wasmimport env get_caller
func getCaller(addrPtr uint32) uint32

//go:wasmimport env set_return_data
func setReturnData(dataPtr uint32, dataLen uint32) uint32

//go:wasmimport env emit_event
func emitEvent(eventPtr uint32, eventLen uint32) uint32

//go:wasmimport env create_utxo_output
func createUTXOOutput(recipientPtr uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32

//go:wasmimport env query_utxo_balance
func queryUTXOBalance(addressPtr uint32, tokenIDPtr uint32, tokenIDLen uint32) uint64

//go:wasmimport env malloc
func malloc(size uint32) uint32

// Initialize 初始化合约
//export Initialize
func Initialize() uint32 {
	// TODO: 实现合约初始化逻辑
	return SUCCESS
}

// Transfer 转账代币
//export Transfer
func Transfer() uint32 {
	// TODO: 实现代币转账逻辑
	return SUCCESS
}

// GetBalance 查询余额
//export GetBalance
func GetBalance() uint32 {
	// TODO: 实现余额查询逻辑
	return SUCCESS
}

// GetMetadata 获取合约元数据
//export GetMetadata
func GetMetadata() uint32 {
	metadata := "{\"name\":\"%s\",\"symbol\":\"TKN\",\"version\":\"1.0.0\"}"
	// TODO: 实现元数据返回
	return SUCCESS
}

func main() {
	// WASM入口点
}
`, s.config.ProjectName, s.config.Author, s.config.License, time.Now().Format("2006-01-02"), s.config.ProjectName)
}

// generateNFTContract 生成NFT合约
func (s *Scaffold) generateNFTContract() string {
	return fmt.Sprintf(`//go:build tinygo.wasm

package main

// ==================== %s NFT合约 ====================
//
// 作者: %s
// 许可证: %s
// 创建时间: %s

// NFT合约基础框架
// TODO: 实现NFT相关功能

//export Initialize
func Initialize() uint32 {
	return 0
}

//export MintNFT
func MintNFT() uint32 {
	return 0
}

//export TransferNFT
func TransferNFT() uint32 {
	return 0
}

func main() {}
`, s.config.ProjectName, s.config.Author, s.config.License, time.Now().Format("2006-01-02"))
}

// generateGovernanceContract 生成治理合约
func (s *Scaffold) generateGovernanceContract() string {
	return fmt.Sprintf(`//go:build tinygo.wasm

package main

// ==================== %s 治理合约 ====================
//
// 作者: %s
// 许可证: %s
// 创建时间: %s

// DAO治理合约基础框架
// TODO: 实现治理相关功能

//export Initialize
func Initialize() uint32 {
	return 0
}

//export CreateProposal
func CreateProposal() uint32 {
	return 0
}

//export Vote
func Vote() uint32 {
	return 0
}

func main() {}
`, s.config.ProjectName, s.config.Author, s.config.License, time.Now().Format("2006-01-02"))
}

// generateDeFiContract 生成DeFi合约
func (s *Scaffold) generateDeFiContract() string {
	return fmt.Sprintf(`//go:build tinygo.wasm

package main

// ==================== %s DeFi合约 ====================
//
// 作者: %s
// 许可证: %s
// 创建时间: %s

// DeFi AMM合约基础框架
// TODO: 实现DeFi相关功能

//export Initialize
func Initialize() uint32 {
	return 0
}

//export AddLiquidity
func AddLiquidity() uint32 {
	return 0
}

//export SwapTokens
func SwapTokens() uint32 {
	return 0
}

func main() {}
`, s.config.ProjectName, s.config.Author, s.config.License, time.Now().Format("2006-01-02"))
}

// generateCustomContract 生成自定义合约
func (s *Scaffold) generateCustomContract() string {
	return fmt.Sprintf(`//go:build tinygo.wasm

package main

// ==================== %s 合约 ====================
//
// 作者: %s
// 许可证: %s
// 创建时间: %s

// 自定义合约基础框架
// TODO: 根据需求实现合约功能

//export Initialize
func Initialize() uint32 {
	return 0
}

//export GetMetadata
func GetMetadata() uint32 {
	return 0
}

func main() {}
`, s.config.ProjectName, s.config.Author, s.config.License, time.Now().Format("2006-01-02"))
}

// generateGoModContent 生成go.mod内容
func (s *Scaffold) generateGoModContent() string {
	return fmt.Sprintf(`module %s

go 1.21

require (
	github.com/weisyn/v1 v0.0.1
)
`, strings.ToLower(s.config.ProjectName))
}

// generateReadmeContent 生成README内容
func (s *Scaffold) generateReadmeContent() string {
	return fmt.Sprintf(`# %s

%s合约项目，基于WES区块链平台开发。

## 项目信息

- **作者**: %s
- **许可证**: %s
- **模板**: %s
- **创建时间**: %s

## 快速开始

### 编译合约

`+"```bash"+`
weisyn-contract compile ./src/%s.go
`+"```"+`

### 部署合约

`+"```bash"+`
weisyn-contract deploy ./build/%s.wasm
`+"```"+`

### 验证合约

`+"```bash"+`
weisyn-contract verify ./src/%s.go
`+"```"+`

## 项目结构

`+"```"+`
%s/
├── src/                # 合约源码
├── tests/              # 测试文件
├── docs/               # 文档
├── scripts/            # 构建脚本
├── build/              # 编译输出
├── deploy/             # 部署配置
└── examples/           # 示例代码
`+"```"+`

## 开发指南

1. 在 src/ 目录下编写合约代码
2. 在 tests/ 目录下添加测试用例
3. 使用构建脚本编译和部署
4. 查看 docs/ 目录了解API文档

## 许可证

本项目采用 %s 许可证。
`, s.config.ProjectName, s.config.ProjectName, s.config.Author, s.config.License,
		s.config.Template, time.Now().Format("2006-01-02"), s.config.ProjectName,
		s.config.ProjectName, s.config.ProjectName, s.config.ProjectName, s.config.License)
}

// generateGitignoreContent 生成.gitignore内容
func (s *Scaffold) generateGitignoreContent() string {
	return `# 编译输出
/build/
*.wasm

# 临时文件
*.tmp
*.log

# IDE文件
.vscode/
.idea/
*.swp
*.swo

# OS文件
.DS_Store
Thumbs.db

# 依赖
/vendor/

# 测试输出
coverage.out
`
}

// generateLicenseContent 生成LICENSE内容
func (s *Scaffold) generateLicenseContent() string {
	year := time.Now().Year()
	switch s.config.License {
	case "MIT":
		return fmt.Sprintf(`MIT License

Copyright (c) %d %s

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`, year, s.config.Author)
	default:
		return fmt.Sprintf(`Copyright (c) %d %s

All rights reserved.
`, year, s.config.Author)
	}
}

// generateBuildScriptContent 生成构建脚本内容
func (s *Scaffold) generateBuildScriptContent() string {
	return fmt.Sprintf(`#!/bin/bash

# %s 构建脚本

set -e

echo "开始编译合约..."

# 编译合约
weisyn-contract compile ./src/%s.go -o ./build

echo "编译完成！"

# 可选：验证合约
if command -v weisyn-contract verify &> /dev/null; then
    echo "验证合约..."
    weisyn-contract verify ./build/%s.wasm
fi

echo "构建成功！"
`, s.config.ProjectName, s.config.ProjectName, s.config.ProjectName)
}

// generateDeployConfigContent 生成部署配置内容
func (s *Scaffold) generateDeployConfigContent() string {
	return fmt.Sprintf(`{
  "%s": {
    "执行费用_limit": 1000000,
    "执行费用_price": 1000000000,
    "init_params": {},
    "verification": {
      "enable": true,
      "test_calls": []
    }
  }
}
`, s.config.ProjectName)
}

// generateTestContent 生成测试内容
func (s *Scaffold) generateTestContent() string {
	return fmt.Sprintf(`package main

import (
	"testing"
)

// %s 合约测试

func TestContractInitialize(t *testing.T) {
	// TODO: 添加初始化测试
}

func TestContractFunctions(t *testing.T) {
	// TODO: 添加功能测试
}
`, s.config.ProjectName)
}

// generateAPIDocContent 生成API文档内容
func (s *Scaffold) generateAPIDocContent() string {
	return fmt.Sprintf(`# %s API 文档

## 合约接口

### Initialize

初始化合约。

**参数**: 无

**返回值**: 错误码

### GetMetadata

获取合约元数据。

**参数**: 无

**返回值**: JSON格式的元数据

## 使用示例

`+"```javascript"+`
// 调用合约示例
const result = await contract.call("Initialize", []);
`+"```"+`

## 更新日志

- v1.0.0: 初始版本
`, s.config.ProjectName)
}

// getTemplate 获取模板
func getTemplate(templateName string) *ProjectTemplate {
	templates := map[string]*ProjectTemplate{
		"token": {
			Name:        "Token Contract",
			Description: "ERC20风格的代币合约模板",
		},
		"nft": {
			Name:        "NFT Contract",
			Description: "ERC721风格的NFT合约模板",
		},
		"governance": {
			Name:        "Governance Contract",
			Description: "DAO治理合约模板",
		},
		"defi": {
			Name:        "DeFi Contract",
			Description: "DeFi AMM DEX合约模板",
		},
		"custom": {
			Name:        "Custom Contract",
			Description: "自定义合约模板",
		},
	}

	if template, exists := templates[templateName]; exists {
		return template
	}

	return templates["custom"]
}
