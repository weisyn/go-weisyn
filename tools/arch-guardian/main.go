package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ArchGuardian 架构守护工具
type ArchGuardian struct {
	rootDir    string
	fileSet    *token.FileSet
	violations []Violation
	rules      []Rule
	config     *Config
}

// Violation 架构违规记录
type Violation struct {
	Type        string
	File        string
	Line        int
	Description string
	Severity    string
}

// Rule 架构规则接口
type Rule interface {
	Name() string
	Check(guardian *ArchGuardian, file string, node ast.Node) []Violation
}

// NewArchGuardian 创建架构守护实例
func NewArchGuardian(rootDir string, config *Config) *ArchGuardian {
	return &ArchGuardian{
		rootDir:    rootDir,
		fileSet:    token.NewFileSet(),
		violations: make([]Violation, 0),
		config:     config,
		rules: []Rule{
			&DirectPublicInterfaceRule{config: config},
			&CrossModuleDependencyRule{config: config},
			&ManagerComplexityRule{config: config},
			&InterfaceConsistencyRule{config: config},
			&HardcodedConstantRule{config: config},
			&PerformanceAntiPatternRule{config: config},
			&SecurityVulnerabilityRule{config: config},
			&ConcurrencyIssueRule{config: config},
			&DesignPatternViolationRule{config: config},
			&TestabilityIssueRule{config: config},
		},
	}
}

// DirectPublicInterfaceRule 直接实现公共接口检查规则
type DirectPublicInterfaceRule struct {
	config *Config
}

func (r *DirectPublicInterfaceRule) Name() string {
	return "DirectPublicInterface"
}

func (r *DirectPublicInterfaceRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	// 检查规则是否启用
	if !r.config.IsRuleEnabled("DirectPublicInterface") {
		return violations
	}

	// 检查是否在白名单中
	if r.config.IsWhitelisted(file) {
		return violations
	}

	// 检查是否匹配例外规则
	if isException, _ := r.config.IsExceptionMatch("DirectPublicInterface", file); isException {
		return violations
	}

	// 检查是否在内部实现中直接引用公共接口
	if !strings.Contains(file, "/interfaces/") && strings.Contains(file, "/internal/core/") {
		ast.Inspect(node, func(n ast.Node) bool {
			if importSpec, ok := n.(*ast.ImportSpec); ok {
				importPath := strings.Trim(importSpec.Path.Value, "\"")
				if strings.Contains(importPath, "pkg/interfaces/") {
					// 检查是否绕过了内部接口
					if !r.hasInternalInterface(file, importPath) {
						violations = append(violations, Violation{
							Type:        "DirectPublicInterface",
							File:        file,
							Line:        guardian.fileSet.Position(importSpec.Pos()).Line,
							Description: fmt.Sprintf("直接导入公共接口 %s，应该通过内部接口继承", importPath),
							Severity:    r.config.GetRuleSeverity("DirectPublicInterface"),
						})
					}
				}
			}
			return true
		})
	}

	return violations
}

func (r *DirectPublicInterfaceRule) hasInternalInterface(file, publicInterface string) bool {
	// 检查同模块下是否存在对应的内部接口
	dir := filepath.Dir(file)
	interfacesDir := filepath.Join(dir, "../interfaces")

	if _, err := os.Stat(interfacesDir); os.IsNotExist(err) {
		return false
	}

	// 简化检查：如果存在 interfaces 目录，认为有内部接口
	return true
}

// CrossModuleDependencyRule 跨模块依赖检查规则
type CrossModuleDependencyRule struct {
	config *Config
}

func (r *CrossModuleDependencyRule) Name() string {
	return "CrossModuleDependency"
}

func (r *CrossModuleDependencyRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	// engines 模块不得依赖 execution 模块
	if strings.Contains(file, "/engines/") && !strings.Contains(file, "/interfaces/") {
		ast.Inspect(node, func(n ast.Node) bool {
			if importSpec, ok := n.(*ast.ImportSpec); ok {
				importPath := strings.Trim(importSpec.Path.Value, "\"")
				if strings.Contains(importPath, "internal/core/execution") && !strings.Contains(importPath, "/interfaces") {
					violations = append(violations, Violation{
						Type:        "CrossModuleDependency",
						File:        file,
						Line:        guardian.fileSet.Position(importSpec.Pos()).Line,
						Description: "engines 模块不得依赖 execution 模块的具体实现",
						Severity:    "ERROR",
					})
				}
			}
			return true
		})
	}

	// execution 模块不得依赖具体的 engines 实现
	if strings.Contains(file, "/execution/") && !strings.Contains(file, "/interfaces/") {
		ast.Inspect(node, func(n ast.Node) bool {
			if importSpec, ok := n.(*ast.ImportSpec); ok {
				importPath := strings.Trim(importSpec.Path.Value, "\"")
				// engines 已迁移到 ispc/engines，检查旧的引用
				if strings.Contains(importPath, "internal/core/engines") && !strings.Contains(importPath, "ispc/engines") {
					violations = append(violations, Violation{
						Type:        "CrossModuleDependency",
						File:        file,
						Line:        guardian.fileSet.Position(importSpec.Pos()).Line,
						Description: "execution 模块不得依赖具体的 engines 实现",
						Severity:    "ERROR",
					})
				}
			}
			return true
		})
	}

	return violations
}

// ManagerComplexityRule Manager 复杂度检查规则
type ManagerComplexityRule struct {
	config *Config
}

func (r *ManagerComplexityRule) Name() string {
	return "ManagerComplexity"
}

func (r *ManagerComplexityRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if !strings.HasSuffix(file, "manager.go") {
		return violations
	}

	// 检查文件行数
	if lineCount := r.countLines(file); lineCount > 200 {
		violations = append(violations, Violation{
			Type:        "ManagerComplexity",
			File:        file,
			Line:        1,
			Description: fmt.Sprintf("Manager 文件过于复杂 (%d 行)，应该拆分为更小的组件", lineCount),
			Severity:    "WARNING",
		})
	}

	// 检查方法复杂度
	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Body != nil {
				stmtCount := r.countStatements(funcDecl.Body)
				if stmtCount > 20 {
					violations = append(violations, Violation{
						Type:        "ManagerComplexity",
						File:        file,
						Line:        guardian.fileSet.Position(funcDecl.Pos()).Line,
						Description: fmt.Sprintf("方法 %s 过于复杂 (%d 语句)，应该委托给子组件", funcDecl.Name.Name, stmtCount),
						Severity:    "WARNING",
					})
				}
			}
		}
		return true
	})

	return violations
}

func (r *ManagerComplexityRule) countLines(file string) int {
	f, err := os.Open(file)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	return lines
}

func (r *ManagerComplexityRule) countStatements(block *ast.BlockStmt) int {
	count := 0
	ast.Inspect(block, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ExprStmt, *ast.AssignStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.ReturnStmt:
			count++
		}
		return true
	})
	return count
}

// InterfaceConsistencyRule 接口一致性检查规则
type InterfaceConsistencyRule struct {
	config *Config
}

func (r *InterfaceConsistencyRule) Name() string {
	return "InterfaceConsistency"
}

func (r *InterfaceConsistencyRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	// 检查内部接口是否继承了对应的公共接口
	if strings.Contains(file, "/interfaces/") && strings.Contains(file, "/internal/core/") {
		ast.Inspect(node, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					if !r.hasPublicInterfaceInheritance(interfaceType) {
						violations = append(violations, Violation{
							Type:        "InterfaceConsistency",
							File:        file,
							Line:        guardian.fileSet.Position(typeSpec.Pos()).Line,
							Description: fmt.Sprintf("内部接口 %s 未继承对应的公共接口", typeSpec.Name.Name),
							Severity:    "WARNING",
						})
					}
				}
			}
			return true
		})
	}

	return violations
}

func (r *InterfaceConsistencyRule) hasPublicInterfaceInheritance(interfaceType *ast.InterfaceType) bool {
	// 简化检查：查看是否有嵌入的接口
	for _, field := range interfaceType.Methods.List {
		if len(field.Names) == 0 { // 嵌入接口
			return true
		}
	}
	return false
}

// HardcodedConstantRule 硬编码常量检查规则
type HardcodedConstantRule struct {
	config *Config
}

func (r *HardcodedConstantRule) Name() string {
	return "HardcodedConstant"
}

func (r *HardcodedConstantRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	// 检查 WASM 函数名是否硬编码
	if strings.Contains(file, "/engines/wasm/") {
		wasmFunctions := []string{
			"get_caller", "get_block_height", "get_block_timestamp",
			"query_utxo_balance", "execute_utxo_transfer",
			"get_current_transaction", "emit_event", "log",
		}

		for _, funcName := range wasmFunctions {
			if r.hasHardcodedString(file, funcName) {
				violations = append(violations, Violation{
					Type:        "HardcodedConstant",
					File:        file,
					Line:        r.findStringLine(file, funcName),
					Description: fmt.Sprintf("发现硬编码的 WASM 函数名 '%s'，应使用 wasm_abi.go 中的常量", funcName),
					Severity:    "WARNING",
				})
			}
		}
	}

	return violations
}

func (r *HardcodedConstantRule) hasHardcodedString(file, str string) bool {
	content, err := os.ReadFile(file)
	if err != nil {
		return false
	}

	// 查找字符串字面量，排除注释和常量定义
	pattern := fmt.Sprintf(`"(%s)"`, regexp.QuoteMeta(str))
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringIndex(string(content), -1)

	for _, match := range matches {
		line := r.getLineContent(string(content), match[0])
		// 排除注释和常量定义
		if !strings.Contains(line, "//") && !strings.Contains(line, "const") && !strings.Contains(line, "var") {
			return true
		}
	}

	return false
}

func (r *HardcodedConstantRule) findStringLine(file, str string) int {
	content, err := os.ReadFile(file)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, fmt.Sprintf(`"%s"`, str)) && !strings.Contains(line, "//") {
			return i + 1
		}
	}
	return 0
}

func (r *HardcodedConstantRule) getLineContent(content string, pos int) string {
	lines := strings.Split(content[:pos], "\n")
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return ""
}

// CheckDirectory 检查指定目录
func (g *ArchGuardian) CheckDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		return g.CheckFile(path)
	})
}

// CheckFile 检查单个文件
func (g *ArchGuardian) CheckFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(g.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// 应用所有规则
	for _, rule := range g.rules {
		violations := rule.Check(g, filename, file)
		g.violations = append(g.violations, violations...)
	}

	return nil
}

// Report 生成检查报告
func (g *ArchGuardian) Report() {
	if len(g.violations) == 0 {
		fmt.Println("🎉 架构检查通过，未发现违规问题！")
		return
	}

	fmt.Printf("🚨 发现 %d 个架构问题：\n\n", len(g.violations))

	// 按类型分组显示
	groupedViolations := make(map[string][]Violation)
	for _, v := range g.violations {
		groupedViolations[v.Type] = append(groupedViolations[v.Type], v)
	}

	for ruleType, violations := range groupedViolations {
		fmt.Printf("📋 %s (%d 个问题):\n", ruleType, len(violations))
		for _, v := range violations {
			fmt.Printf("  %s %s:%d - %s\n", g.getSeverityIcon(v.Severity), v.File, v.Line, v.Description)
		}
		fmt.Println()
	}

	// 统计信息
	errorCount := 0
	warningCount := 0
	for _, v := range g.violations {
		if v.Severity == "ERROR" {
			errorCount++
		} else {
			warningCount++
		}
	}

	fmt.Printf("📊 统计: %d 错误, %d 警告\n", errorCount, warningCount)
}

func (g *ArchGuardian) getSeverityIcon(severity string) string {
	switch severity {
	case "ERROR":
		return "❌"
	case "WARNING":
		return "⚠️"
	default:
		return "ℹ️"
	}
}

// HasErrors 是否有错误级别的违规
func (g *ArchGuardian) HasErrors() bool {
	for _, v := range g.violations {
		if v.Severity == "ERROR" {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: arch-guardian <目录路径> [--config=配置文件路径]")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	configPath := ""

	// 解析命令行参数
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
		}
	}

	// 如果没有指定配置文件，尝试使用默认配置文件
	if configPath == "" {
		defaultConfigPath := "tools/arch-guardian/config.yaml"
		if _, err := os.Stat(defaultConfigPath); err == nil {
			configPath = defaultConfigPath
		}
	}

	// 加载配置
	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	guardian := NewArchGuardian(rootDir, config)

	fmt.Println("🛡️ 开始架构守护检查...")
	if configPath != "" {
		fmt.Printf("📋 使用配置文件: %s\n", configPath)
	}

	if err := guardian.CheckDirectory(rootDir); err != nil {
		fmt.Printf("检查失败: %v\n", err)
		os.Exit(1)
	}

	guardian.Report()

	if guardian.HasErrors() {
		os.Exit(1)
	}
}
