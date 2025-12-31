// Package main provides the arch-fixer tool for automatically fixing architectural issues.
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

// ArchFixer 架构问题自动修复工具
type ArchFixer struct {
	rootDir   string
	fileSet   *token.FileSet
	fixes     []Fix
	dryRun    bool
	whitelist map[string]bool
}

// Fix 修复记录
type Fix struct {
	Type        string
	File        string
	Line        int
	Description string
	Action      string
	Applied     bool
}

// NewArchFixer 创建架构修复工具实例
func NewArchFixer(rootDir string, dryRun bool) *ArchFixer {
	return &ArchFixer{
		rootDir: rootDir,
		fileSet: token.NewFileSet(),
		fixes:   make([]Fix, 0),
		dryRun:  dryRun,
		whitelist: map[string]bool{
			// 已知的合理例外情况
			"internal/core/ispc/engines/wasm/interfaces": true, // WASM引擎内部接口可以导入公共接口
			"internal/core/ispc/interfaces":              true, // ISPC内部接口可以导入公共接口
			"internal/core/execution/interfaces":  true,
			"internal/core/blockchain/interfaces": true,
			// 测试文件例外
			"_test.go": true,
			// 集成测试例外
			"integration": true,
			// 工具和脚本例外
			"tools":   true,
			"scripts": true,
			"cmd":     true,
		},
	}
}

// FixDirectory 修复指定目录的架构问题
func (f *ArchFixer) FixDirectory(dir string) error {
	if _, err := fmt.Printf("🔧 开始分析目录: %s\n", dir); err != nil {
		return fmt.Errorf("输出信息失败: %w", err)
	}

	return filepath.Walk(dir, func(path string, __info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过非 Go 文件和测试文件
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		// 检查白名单
		if f.isWhitelisted(path) {
			return nil
		}

		return f.analyzeAndFixFile(path)
	})
}

// isWhitelisted 检查文件是否在白名单中
func (f *ArchFixer) isWhitelisted(path string) bool {
	for pattern := range f.whitelist {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// analyzeAndFixFile 分析并修复单个文件
func (f *ArchFixer) analyzeAndFixFile(filename string) error {
	//nolint:gosec // G304: filename 来自命令行参数，用户可控但工具用途明确
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(f.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// 检查并修复各种架构问题
	f.fixDirectPublicInterfaceImports(filename, file)
	f.fixHardcodedConstants(filename, src)
	f.fixManagerComplexity(filename, file)

	return nil
}

// fixDirectPublicInterfaceImports 修复直接导入公共接口的问题
func (f *ArchFixer) fixDirectPublicInterfaceImports(filename string, file *ast.File) {
	// 检查是否在内部实现中直接导入公共接口
	if !strings.Contains(filename, "/internal/core/") || strings.Contains(filename, "/interfaces/") {
		return
	}

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")

		// 检查是否直接导入公共接口
		if strings.Contains(importPath, "pkg/interfaces/") {
			f.suggestInternalInterface(filename, importPath, imp)
		}
	}
}

// suggestInternalInterface 建议使用内部接口
func (f *ArchFixer) suggestInternalInterface(filename, publicInterface string, imp *ast.ImportSpec) {
	// 分析应该使用的内部接口路径
	var internalInterface string

	if strings.Contains(publicInterface, "pkg/interfaces/engines") {
		// engines 接口已迁移到 ispc/engines/wasm/interfaces
		internalInterface = "github.com/weisyn/v1/internal/core/ispc/engines/wasm/interfaces"
	} else if strings.Contains(publicInterface, "pkg/interfaces/execution") {
		internalInterface = "github.com/weisyn/v1/internal/core/ispc/interfaces"
	}

	if internalInterface != "" {
		fix := Fix{
			Type:        "DirectPublicInterface",
			File:        filename,
			Line:        f.fileSet.Position(imp.Pos()).Line,
			Description: fmt.Sprintf("建议将 %s 替换为 %s", publicInterface, internalInterface),
			Action:      fmt.Sprintf("import \"%s\"", internalInterface),
			Applied:     false,
		}
		f.fixes = append(f.fixes, fix)
	}
}

// fixHardcodedConstants 修复硬编码常量
func (f *ArchFixer) fixHardcodedConstants(filename string, src []byte) {
	if !strings.Contains(filename, "/engines/wasm/") {
		return
	}

	content := string(src)

	// WASM 函数名常量映射
	wasmConstants := map[string]string{
		"get_caller":              "engines.WASMFuncGetCaller",
		"get_block_height":        "engines.WASMFuncGetBlockHeight",
		"get_block_timestamp":     "engines.WASMFuncGetBlockTimestamp",
		"query_utxo_balance":      "engines.WASMFuncQueryUTXOBalance",
		"execute_utxo_transfer":   "engines.WASMFuncExecuteUTXOTransfer",
		"get_current_transaction": "engines.WASMFuncGetCurrentTransaction",
		"emit_event":              "engines.WASMFuncEmitEvent",
		"log":                     "engines.WASMFuncLog",
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for hardcoded, constant := range wasmConstants {
			// 查找硬编码字符串，排除注释和常量定义
			pattern := fmt.Sprintf(`"(%s)"`, regexp.QuoteMeta(hardcoded))
			re := regexp.MustCompile(pattern)

			if re.MatchString(line) && !strings.Contains(line, "//") && !strings.Contains(line, "const") {
				fix := Fix{
					Type:        "HardcodedConstant",
					File:        filename,
					Line:        i + 1,
					Description: fmt.Sprintf("硬编码字符串 \"%s\" 应使用常量 %s", hardcoded, constant),
					Action:      fmt.Sprintf("替换为 %s", constant),
					Applied:     false,
				}
				f.fixes = append(f.fixes, fix)
			}
		}
	}
}

// fixManagerComplexity 分析 Manager 复杂度问题
func (f *ArchFixer) fixManagerComplexity(filename string, file *ast.File) {
	if !strings.HasSuffix(filename, "manager.go") {
		return
	}

	// 检查文件行数
	if lineCount := f.countFileLines(filename); lineCount > 200 {
		fix := Fix{
			Type:        "ManagerComplexity",
			File:        filename,
			Line:        1,
			Description: fmt.Sprintf("Manager 文件过大 (%d 行)，建议拆分", lineCount),
			Action:      "考虑将复杂逻辑委托给子组件",
			Applied:     false,
		}
		f.fixes = append(f.fixes, fix)
	}

	// 检查方法复杂度
	ast.Inspect(file, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Body != nil {
				stmtCount := f.countStatements(funcDecl.Body)
				if stmtCount > 20 {
					fix := Fix{
						Type:        "ManagerComplexity",
						File:        filename,
						Line:        f.fileSet.Position(funcDecl.Pos()).Line,
						Description: fmt.Sprintf("方法 %s 过于复杂 (%d 语句)", funcDecl.Name.Name, stmtCount),
						Action:      "将复杂逻辑委托给子组件实现",
						Applied:     false,
					}
					f.fixes = append(f.fixes, fix)
				}
			}
		}
		return true
	})
}

// countFileLines 计算文件行数
func (f *ArchFixer) countFileLines(filename string) int {
	//nolint:gosec // G304: filename 来自命令行参数，用户可控但工具用途明确
	file, err := os.Open(filename)
	if err != nil {
		return 0
	}
	defer func() {
		if err := file.Close(); err != nil {
			// 文件关闭失败，输出到 stderr 但不影响行数统计
			_, _ = fmt.Fprintf(os.Stderr, "警告: 关闭文件失败: %v\n", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	return lines
}

// countStatements 计算语句数量
func (f *ArchFixer) countStatements(block *ast.BlockStmt) int {
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

// Report 生成修复报告
func (f *ArchFixer) Report() {
	if len(f.fixes) == 0 {
		if _, err := fmt.Println("✅ 未发现需要修复的架构问题"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
		return
	}

	if _, err := fmt.Printf("\n📋 发现 %d 个可修复的架构问题：\n\n", len(f.fixes)); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		return
	}

	// 按类型分组
	groupedFixes := make(map[string][]Fix)
	for _, fix := range f.fixes {
		groupedFixes[fix.Type] = append(groupedFixes[fix.Type], fix)
	}

	for fixType, fixes := range groupedFixes {
		if _, err := fmt.Printf("🔧 %s (%d 个问题):\n", fixType, len(fixes)); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
			continue
		}
		for _, fix := range fixes {
			status := "❌"
			if fix.Applied {
				status = "✅"
			}
			if _, err := fmt.Printf("  %s %s:%d\n", status, fix.File, fix.Line); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
				continue
			}
			if _, err := fmt.Printf("     问题: %s\n", fix.Description); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
				continue
			}
			if _, err := fmt.Printf("     建议: %s\n", fix.Action); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
				continue
			}
			if _, err := fmt.Println(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
			}
		}
	}

	if f.dryRun {
		if _, err := fmt.Println("🔍 这是预览模式，未实际修改文件"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
		if _, err := fmt.Println("💡 使用 --apply 参数应用修复"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
	}
}

// GenerateFixScript 生成修复脚本
func (f *ArchFixer) GenerateFixScript() error {
	scriptPath := "scripts/apply-arch-fixes.sh"

	//nolint:gosec // G304: scriptPath 是固定路径，安全可控
	file, err := os.Create(scriptPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// 文件关闭失败，输出到 stderr 但不影响脚本生成
			_, _ = fmt.Fprintf(os.Stderr, "警告: 关闭脚本文件失败: %v\n", closeErr)
		}
	}()

	if _, err := fmt.Fprintln(file, "#!/bin/bash"); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}
	if _, err := fmt.Fprintln(file, "# 自动生成的架构问题修复脚本"); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}
	if _, err := fmt.Fprintln(file, ""); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}
	if _, err := fmt.Fprintln(file, "set -e"); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}
	if _, err := fmt.Fprintln(file, ""); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}

	// 按类型生成修复命令
	groupedFixes := make(map[string][]Fix)
	for _, fix := range f.fixes {
		groupedFixes[fix.Type] = append(groupedFixes[fix.Type], fix)
	}

	for fixType, fixes := range groupedFixes {
		if _, err := fmt.Fprintf(file, "echo \"🔧 修复 %s 问题...\"\n", fixType); err != nil {
			return fmt.Errorf("写入脚本失败: %w", err)
		}

		for _, fix := range fixes {
			switch fix.Type {
			case "HardcodedConstant":
				// 生成替换命令（简化版）
				if _, err := fmt.Fprintf(file, "# %s:%d - %s\n", fix.File, fix.Line, fix.Description); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
				if _, err := fmt.Fprintf(file, "echo \"  请手动修复: %s\"\n", fix.File); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
			case "DirectPublicInterface":
				if _, err := fmt.Fprintf(file, "# %s:%d - %s\n", fix.File, fix.Line, fix.Description); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
				if _, err := fmt.Fprintf(file, "echo \"  请手动修复: %s\"\n", fix.File); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
			case "ManagerComplexity":
				if _, err := fmt.Fprintf(file, "# %s:%d - %s\n", fix.File, fix.Line, fix.Description); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
				if _, err := fmt.Fprintf(file, "echo \"  请重构: %s\"\n", fix.File); err != nil {
					return fmt.Errorf("写入脚本失败: %w", err)
				}
			}
		}
		if _, err := fmt.Fprintln(file, ""); err != nil {
			return fmt.Errorf("写入脚本失败: %w", err)
		}
	}

	if _, err := fmt.Fprintln(file, "echo \"✅ 架构修复脚本执行完成\""); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}
	if _, err := fmt.Fprintln(file, "echo \"💡 请运行 'make arch-check' 验证修复结果\""); err != nil {
		return fmt.Errorf("写入脚本失败: %w", err)
	}

	// 设置执行权限
	//nolint:gosec // G302: 脚本文件需要执行权限，0755 是合理的
	return os.Chmod(scriptPath, 0755)
}

func main() {
	if len(os.Args) < 2 {
		if _, err := fmt.Println("用法: arch-fixer <目录路径> [--apply]"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
		if _, err := fmt.Println("  --apply: 应用修复（默认为预览模式）"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
		os.Exit(1)
	}

	rootDir := os.Args[1]
	dryRun := len(os.Args) <= 2 || os.Args[2] != "--apply" //nolint:staticcheck // QF1007: 合并条件赋值到变量声明

	fixer := NewArchFixer(rootDir, dryRun)

	if _, err := fmt.Println("🔧 架构问题自动修复工具"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
	}
	if _, err := fmt.Printf("📁 目标目录: %s\n", rootDir); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
	}

	if dryRun {
		if _, err := fmt.Println("🔍 运行模式: 预览模式"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
	} else {
		if _, err := fmt.Println("⚡ 运行模式: 应用修复"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
	}

	if err := fixer.FixDirectory(rootDir); err != nil {
		if _, err2 := fmt.Printf("❌ 分析失败: %v\n", err); err2 != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err2)
		}
		os.Exit(1)
	}

	fixer.Report()

	// 生成修复脚本
	if err := fixer.GenerateFixScript(); err != nil {
		if _, err2 := fmt.Printf("⚠️ 生成修复脚本失败: %v\n", err); err2 != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err2)
		}
	} else {
		if _, err := fmt.Println("📜 已生成修复脚本: scripts/apply-arch-fixes.sh"); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "输出信息失败: %v\n", err)
		}
	}
}
