// Package main provides a contract verification tool.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ==================== WES 合约验证工具 ====================
//
// 🌟 **设计理念**：为WES合约提供全面的验证和审计功能
//
// 🎯 **核心特性**：
// - 静态代码分析和安全审计
// - WASM字节码验证
// - 合约接口规范检查
// - 性能和执行费用使用分析
// - 生成验证报告和建议
//

const (
	VERSION = "1.0.0"
	USAGE   = `WES Contract Verifier v%s

用法:
  weisyn-contract verify [选项] <合约文件或目录>

选项:
  -t, --type <类型>         验证类型 (source|wasm|deployed)
  -l, --level <级别>        验证级别 (basic|standard|strict)
  -r, --rules <规则文件>    自定义验证规则文件
  -o, --output <文件>       输出报告文件
  -f, --format <格式>       报告格式 (text|json|html)
  -v, --verbose            详细输出
  -q, --quiet              静默模式
  -h, --help               显示帮助信息
  --version                显示版本信息

验证类型:
  source     - 验证Go源码
  wasm       - 验证WASM字节码
  deployed   - 验证已部署的合约

验证级别:
  basic      - 基础验证（语法、接口）
  standard   - 标准验证（安全、性能）
  strict     - 严格验证（最佳实践、优化建议）

示例:
  weisyn-contract verify ./contracts/token.go
  weisyn-contract verify -t wasm -l strict ./build/nft.wasm
  weisyn-contract verify -f json -o report.json ./contracts
`
)

// VerifierConfig 验证器配置
type VerifierConfig struct {
	VerifyType string
	Level      string
	RulesFile  string
	OutputFile string
	Format     string
	Verbose    bool
	Quiet      bool

	// 验证选项
	CheckSecurity          bool
	CheckPerformance       bool
	CheckCompliance        bool
	CheckExecutionFeeUsage bool
}

// DefaultVerifierConfig 默认验证器配置
func DefaultVerifierConfig() *VerifierConfig {
	return &VerifierConfig{
		VerifyType:             "source",
		Level:                  "standard",
		Format:                 "text",
		Verbose:                false,
		Quiet:                  false,
		CheckSecurity:          true,
		CheckPerformance:       true,
		CheckCompliance:        true,
		CheckExecutionFeeUsage: true,
	}
}

// VerificationRule 验证规则
type VerificationRule struct {
	ID          string
	Category    string
	Level       string
	Title       string
	Description string
	Pattern     string
	Message     string
	Severity    string
	AutoFix     bool
}

// VerificationIssue 验证问题
type VerificationIssue struct {
	Rule       *VerificationRule
	File       string
	Line       int
	Column     int
	Message    string
	Severity   string
	Context    string
	Suggestion string
}

// VerificationResult 验证结果
type VerificationResult struct {
	File      string
	Success   bool
	Issues    []*VerificationIssue
	Stats     *VerificationStats
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// VerificationStats 验证统计
type VerificationStats struct {
	TotalLines        int
	TotalFunctions    int
	TotalExports      int
	ErrorCount        int
	WarningCount      int
	InfoCount         int
	SecurityIssues    int
	PerformanceIssues int
	ComplianceIssues  int
}

// OverallReport 总体报告
type OverallReport struct {
	Summary         *ReportSummary
	Results         []*VerificationResult
	Recommendations []string
	GeneratedAt     time.Time
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalFiles     int
	SuccessFiles   int
	FailedFiles    int
	TotalIssues    int
	CriticalIssues int
	HighIssues     int
	MediumIssues   int
	LowIssues      int
	OverallScore   float64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(USAGE, VERSION)
		os.Exit(1)
	}

	config := DefaultVerifierConfig()
	var sourcePath string

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			fmt.Printf(USAGE, VERSION)
			os.Exit(0)
		case "--version":
			fmt.Printf("WES Contract Verifier v%s\n", VERSION)
			os.Exit(0)
		case "-v", "--verbose":
			config.Verbose = true
		case "-q", "--quiet":
			config.Quiet = true
		case "-t", "--type":
			if i+1 < len(os.Args) {
				config.VerifyType = os.Args[i+1]
				i++
			}
		case "-l", "--level":
			if i+1 < len(os.Args) {
				config.Level = os.Args[i+1]
				i++
			}
		case "-r", "--rules":
			if i+1 < len(os.Args) {
				config.RulesFile = os.Args[i+1]
				i++
			}
		case "-o", "--output":
			if i+1 < len(os.Args) {
				config.OutputFile = os.Args[i+1]
				i++
			}
		case "-f", "--format":
			if i+1 < len(os.Args) {
				config.Format = os.Args[i+1]
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

	// 执行验证
	verifier := NewVerifier(config)
	report, err := verifier.Verify(sourcePath)
	if err != nil {
		fmt.Printf("验证失败: %v\n", err)
		os.Exit(1)
	}

	// 输出结果
	if err := outputReport(report, config); err != nil {
		fmt.Printf("输出报告失败: %v\n", err)
		os.Exit(1)
	}

	// 根据验证结果确定退出码
	if report.Summary.CriticalIssues > 0 || report.Summary.HighIssues > 0 {
		os.Exit(1)
	}
}

// Verifier 验证器
type Verifier struct {
	config *VerifierConfig
	rules  []*VerificationRule
}

// NewVerifier 创建验证器
func NewVerifier(config *VerifierConfig) *Verifier {
	verifier := &Verifier{
		config: config,
		rules:  getBuiltinRules(config.Level),
	}

	// 加载自定义规则
	if config.RulesFile != "" {
		customRules, err := loadCustomRules(config.RulesFile)
		if err == nil {
			verifier.rules = append(verifier.rules, customRules...)
		}
	}

	return verifier
}

// Verify 执行验证
func (v *Verifier) Verify(sourcePath string) (*OverallReport, error) {

	// 发现需要验证的文件
	files, err := v.discoverFiles(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("发现文件失败: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("未找到可验证的文件")
	}

	if v.config.Verbose {
		fmt.Printf("发现 %d 个文件需要验证\n", len(files))
	}

	// 逐个验证文件
	results := make([]*VerificationResult, 0, len(files))
	for _, file := range files {
		result := v.verifyFile(file)
		results = append(results, result)

		if v.config.Verbose && !v.config.Quiet {
			if result.Success {
				fmt.Printf("✓ %s\n", file)
			} else {
				fmt.Printf("✗ %s (%d issues)\n", file, len(result.Issues))
			}
		}
	}

	// 生成总体报告
	report := &OverallReport{
		Summary:         v.generateSummary(results),
		Results:         results,
		Recommendations: v.generateRecommendations(results),
		GeneratedAt:     time.Now(),
	}

	return report, nil
}

// discoverFiles 发现需要验证的文件
func (v *Verifier) discoverFiles(sourcePath string) ([]string, error) {
	var files []string

	// 根据验证类型选择文件扩展名
	var extensions []string
	switch v.config.VerifyType {
	case "source":
		extensions = []string{".go"}
	case "wasm":
		extensions = []string{".wasm"}
	default:
		extensions = []string{".go", ".wasm"}
	}

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			for _, ext := range extensions {
				if strings.HasSuffix(path, ext) {
					// 跳过测试文件
					if !strings.HasSuffix(path, "_test.go") {
						files = append(files, path)
					}
					break
				}
			}
		}

		return nil
	})

	return files, err
}

// verifyFile 验证单个文件
func (v *Verifier) verifyFile(filename string) *VerificationResult {
	startTime := time.Now()

	result := &VerificationResult{
		File:      filename,
		Success:   true,
		Issues:    []*VerificationIssue{},
		Stats:     &VerificationStats{},
		StartTime: startTime,
	}

	// 读取文件内容
	//nolint:gosec // G304: filename 来自命令行参数，用户可控但工具用途明确
	content, err := os.ReadFile(filename)
	if err != nil {
		result.Success = false
		result.Issues = append(result.Issues, &VerificationIssue{
			Message:  fmt.Sprintf("无法读取文件: %v", err),
			Severity: "error",
			File:     filename,
		})
		return result
	}

	// 根据文件类型选择验证方法
	if strings.HasSuffix(filename, ".go") {
		v.verifyGoSource(string(content), result)
	} else if strings.HasSuffix(filename, ".wasm") {
		v.verifyWasmBinary(content, result)
	}

	// 更新统计信息
	v.updateStats(result)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// verifyGoSource 验证Go源码
func (v *Verifier) verifyGoSource(content string, result *VerificationResult) {
	lines := strings.Split(content, "\n")
	result.Stats.TotalLines = len(lines)

	// 应用验证规则
	for _, rule := range v.rules {
		if rule.Category == "source" || rule.Category == "all" {
			v.applyRule(rule, content, lines, result)
		}
	}

	// 统计函数和导出
	result.Stats.TotalFunctions = countFunctions(content)
	result.Stats.TotalExports = countExports(content)

	// 特定检查
	if v.config.CheckSecurity {
		v.checkSecurityIssues(content, lines, result)
	}

	if v.config.CheckPerformance {
		v.checkPerformanceIssues(content, lines, result)
	}

	if v.config.CheckCompliance {
		v.checkComplianceIssues(content, lines, result)
	}
}

// verifyWasmBinary 验证WASM二进制
func (v *Verifier) verifyWasmBinary(content []byte, result *VerificationResult) {
	// 检查WASM魔数
	if len(content) < 4 || string(content[:4]) != "\x00asm" {
		result.Issues = append(result.Issues, &VerificationIssue{
			Message:  "无效的WASM文件格式",
			Severity: "error",
			File:     result.File,
			Line:     1,
		})
		result.Success = false
		return
	}

	// 检查WASM版本
	if len(content) < 8 {
		result.Issues = append(result.Issues, &VerificationIssue{
			Message:  "WASM文件过短",
			Severity: "error",
			File:     result.File,
			Line:     1,
		})
		result.Success = false
		return
	}

	// 简单的WASM验证
	v.checkWasmStructure(content, result)
	v.checkWasmExports(content, result)
	v.checkWasmImports(content, result)
}

// applyRule 应用验证规则
func (v *Verifier) applyRule(rule *VerificationRule, content string, lines []string, result *VerificationResult) {
	// 简化的规则匹配（实际应使用正则表达式或AST分析）
	if rule.Pattern != "" && strings.Contains(content, rule.Pattern) {
		// 查找具体位置
		for i, line := range lines {
			if strings.Contains(line, rule.Pattern) {
				issue := &VerificationIssue{
					Rule:     rule,
					File:     result.File,
					Line:     i + 1,
					Message:  rule.Message,
					Severity: rule.Severity,
					Context:  line,
				}
				result.Issues = append(result.Issues, issue)
			}
		}
	}
}

// checkSecurityIssues 检查安全问题
func (v *Verifier) checkSecurityIssues(content string, lines []string, result *VerificationResult) {
	securityChecks := []struct {
		pattern  string
		message  string
		severity string
	}{
		{"panic(", "避免使用panic，应该返回错误", "warning"},
		{"unsafe.", "使用unsafe包需要特别小心", "warning"},
		{"//TODO", "待办事项需要完成", "info"},
		{"//FIXME", "修复问题需要完成", "warning"},
	}

	for _, check := range securityChecks {
		if strings.Contains(content, check.pattern) {
			for i, line := range lines {
				if strings.Contains(line, check.pattern) {
					issue := &VerificationIssue{
						File:     result.File,
						Line:     i + 1,
						Message:  check.message,
						Severity: check.severity,
						Context:  strings.TrimSpace(line),
					}
					result.Issues = append(result.Issues, issue)
					result.Stats.SecurityIssues++
				}
			}
		}
	}
}

// checkPerformanceIssues 检查性能问题
func (v *Verifier) checkPerformanceIssues(content string, lines []string, result *VerificationResult) {
	performanceChecks := []struct {
		pattern  string
		message  string
		severity string
	}{
		{"make([]", "考虑预分配切片容量以提高性能", "info"},
		{"strings.Split", "频繁的字符串分割可能影响性能", "info"},
		{"fmt.Printf", "考虑使用更高效的字符串格式化方法", "info"},
	}

	for _, check := range performanceChecks {
		if strings.Contains(content, check.pattern) {
			for i, line := range lines {
				if strings.Contains(line, check.pattern) {
					issue := &VerificationIssue{
						File:     result.File,
						Line:     i + 1,
						Message:  check.message,
						Severity: check.severity,
						Context:  strings.TrimSpace(line),
					}
					result.Issues = append(result.Issues, issue)
					result.Stats.PerformanceIssues++
				}
			}
		}
	}
}

// checkComplianceIssues 检查合规问题
func (v *Verifier) checkComplianceIssues(content string, lines []string, result *VerificationResult) {
	complianceChecks := []struct {
		pattern  string
		message  string
		severity string
	}{
		{"//export", "导出函数应该有完整的文档注释", "warning"},
		{"func main()", "main函数应该为空（WASM模块）", "info"},
	}

	for _, check := range complianceChecks {
		if strings.Contains(content, check.pattern) {
			for i, line := range lines {
				if strings.Contains(line, check.pattern) {
					issue := &VerificationIssue{
						File:     result.File,
						Line:     i + 1,
						Message:  check.message,
						Severity: check.severity,
						Context:  strings.TrimSpace(line),
					}
					result.Issues = append(result.Issues, issue)
					result.Stats.ComplianceIssues++
				}
			}
		}
	}
}

// checkWasmStructure 检查WASM结构
func (v *Verifier) checkWasmStructure(content []byte, result *VerificationResult) {
	// 简化的WASM结构检查
	if len(content) < 100 {
		result.Issues = append(result.Issues, &VerificationIssue{
			File:     result.File,
			Message:  "WASM文件过小，可能不完整",
			Severity: "warning",
		})
	}
}

// checkWasmExports 检查WASM导出
func (v *Verifier) checkWasmExports(content []byte, result *VerificationResult) {
	// 简化的导出检查
	if !strings.Contains(string(content), "Initialize") {
		result.Issues = append(result.Issues, &VerificationIssue{
			File:     result.File,
			Message:  "未找到Initialize函数导出",
			Severity: "warning",
		})
	}
}

// checkWasmImports 检查WASM导入
func (v *Verifier) checkWasmImports(content []byte, result *VerificationResult) {
	// 简化的导入检查
	requiredImports := []string{"get_caller", "set_return_data", "emit_event"}
	for _, imp := range requiredImports {
		if !strings.Contains(string(content), imp) {
			result.Issues = append(result.Issues, &VerificationIssue{
				File:     result.File,
				Message:  fmt.Sprintf("未找到必需的导入函数: %s", imp),
				Severity: "warning",
			})
		}
	}
}

// ==================== 辅助函数 ====================

// getBuiltinRules 获取内置规则
func getBuiltinRules(level string) []*VerificationRule {
	rules := []*VerificationRule{
		{
			ID:          "SECURITY_001",
			Category:    "security",
			Level:       "basic",
			Title:       "避免使用panic",
			Description: "合约中应该避免使用panic，而是返回错误码",
			Pattern:     "panic(",
			Message:     "使用panic可能导致合约异常终止",
			Severity:    "warning",
		},
		{
			ID:          "PERFORMANCE_001",
			Category:    "performance",
			Level:       "standard",
			Title:       "优化内存分配",
			Description: "预分配切片和映射的容量可以提高性能",
			Pattern:     "make([]",
			Message:     "考虑预分配容量以提高性能",
			Severity:    "info",
		},
		{
			ID:          "COMPLIANCE_001",
			Category:    "compliance",
			Level:       "basic",
			Title:       "导出函数文档",
			Description: "所有导出函数都应该有完整的文档注释",
			Pattern:     "//export",
			Message:     "导出函数需要文档注释",
			Severity:    "warning",
		},
	}

	// 根据级别过滤规则
	var filteredRules []*VerificationRule
	for _, rule := range rules {
		if shouldIncludeRule(rule, level) {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules
}

// shouldIncludeRule 检查是否应该包含规则
func shouldIncludeRule(rule *VerificationRule, level string) bool {
	levelOrder := map[string]int{
		"basic":    1,
		"standard": 2,
		"strict":   3,
	}

	ruleLevel := levelOrder[rule.Level]
	targetLevel := levelOrder[level]

	return ruleLevel <= targetLevel
}

// loadCustomRules 加载自定义规则
func loadCustomRules(_filename string) ([]*VerificationRule, error) {
	// 简化实现：返回空规则列表
	return []*VerificationRule{}, nil
}

// countFunctions 统计函数数量
func countFunctions(content string) int {
	return strings.Count(content, "func ")
}

// countExports 统计导出数量
func countExports(content string) int {
	return strings.Count(content, "//export")
}

// updateStats 更新统计信息
func (v *Verifier) updateStats(result *VerificationResult) {
	for _, issue := range result.Issues {
		switch issue.Severity {
		case "error":
			result.Stats.ErrorCount++
		case "warning":
			result.Stats.WarningCount++
		case "info":
			result.Stats.InfoCount++
		}
	}

	if result.Stats.ErrorCount > 0 {
		result.Success = false
	}
}

// generateSummary 生成摘要
func (v *Verifier) generateSummary(results []*VerificationResult) *ReportSummary {
	summary := &ReportSummary{
		TotalFiles: len(results),
	}

	for _, result := range results {
		if result.Success {
			summary.SuccessFiles++
		} else {
			summary.FailedFiles++
		}

		for _, issue := range result.Issues {
			summary.TotalIssues++
			switch issue.Severity {
			case "critical":
				summary.CriticalIssues++
			case "high", "error":
				summary.HighIssues++
			case "medium", "warning":
				summary.MediumIssues++
			case "low", "info":
				summary.LowIssues++
			}
		}
	}

	// 计算总体评分
	if summary.TotalFiles > 0 {
		score := float64(summary.SuccessFiles) / float64(summary.TotalFiles) * 100
		if summary.CriticalIssues > 0 {
			score -= float64(summary.CriticalIssues) * 10
		}
		if summary.HighIssues > 0 {
			score -= float64(summary.HighIssues) * 5
		}
		if score < 0 {
			score = 0
		}
		summary.OverallScore = score
	}

	return summary
}

// generateRecommendations 生成建议
func (v *Verifier) generateRecommendations(results []*VerificationResult) []string {
	recommendations := []string{}

	// 分析常见问题并给出建议
	securityIssues := 0
	performanceIssues := 0
	complianceIssues := 0

	for _, result := range results {
		securityIssues += result.Stats.SecurityIssues
		performanceIssues += result.Stats.PerformanceIssues
		complianceIssues += result.Stats.ComplianceIssues
	}

	if securityIssues > 0 {
		recommendations = append(recommendations, "建议加强安全检查，避免使用可能导致合约异常的函数")
	}

	if performanceIssues > 0 {
		recommendations = append(recommendations, "建议优化性能，特别是内存分配和字符串操作")
	}

	if complianceIssues > 0 {
		recommendations = append(recommendations, "建议完善文档注释，确保代码符合WES合约规范")
	}

	return recommendations
}

// outputReport 输出报告
func outputReport(report *OverallReport, config *VerifierConfig) error {
	switch config.Format {
	case "json":
		return outputJSONReport(report, config)
	case "html":
		return outputHTMLReport(report, config)
	default:
		return outputTextReport(report, config)
	}
}

// outputTextReport 输出文本报告
func outputTextReport(report *OverallReport, config *VerifierConfig) error {
	output := generateTextReport(report)

	if config.OutputFile != "" {
		//nolint:gosec // G304,G306: config.OutputFile 来自命令行参数，用户可控但工具用途明确；报告文件需要用户可读权限，0644 是合理的
		return os.WriteFile(config.OutputFile, []byte(output), 0644)
	}
	fmt.Print(output)
	return nil
}

// outputJSONReport 输出JSON报告
func outputJSONReport(report *OverallReport, config *VerifierConfig) error {
	// 简化的JSON输出
	output := fmt.Sprintf(`{
  "summary": {
    "total_files": %d,
    "success_files": %d,
    "failed_files": %d,
    "total_issues": %d,
    "overall_score": %.1f
  },
  "generated_at": "%s"
}`,
		report.Summary.TotalFiles,
		report.Summary.SuccessFiles,
		report.Summary.FailedFiles,
		report.Summary.TotalIssues,
		report.Summary.OverallScore,
		report.GeneratedAt.Format(time.RFC3339))

	if config.OutputFile != "" {
		//nolint:gosec // G304,G306: config.OutputFile 来自命令行参数，用户可控但工具用途明确；报告文件需要用户可读权限，0644 是合理的
		return os.WriteFile(config.OutputFile, []byte(output), 0644)
	}
	fmt.Print(output)
	return nil
}

// outputHTMLReport 输出HTML报告
func outputHTMLReport(report *OverallReport, config *VerifierConfig) error {
	// 简化的HTML输出
	output := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>WES Contract Verification Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f0f0f0; padding: 15px; margin-bottom: 20px; }
        .score { font-size: 24px; font-weight: bold; }
    </style>
</head>
<body>
    <h1>WES Contract Verification Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p>Total Files: %d</p>
        <p>Success: %d, Failed: %d</p>
        <p>Total Issues: %d</p>
        <p class="score">Overall Score: %.1f/100</p>
    </div>
    <p>Generated at: %s</p>
</body>
</html>`,
		report.Summary.TotalFiles,
		report.Summary.SuccessFiles,
		report.Summary.FailedFiles,
		report.Summary.TotalIssues,
		report.Summary.OverallScore,
		report.GeneratedAt.Format("2006-01-02 15:04:05"))

	if config.OutputFile != "" {
		//nolint:gosec // G304,G306: config.OutputFile 来自命令行参数，用户可控但工具用途明确；报告文件需要用户可读权限，0644 是合理的
		return os.WriteFile(config.OutputFile, []byte(output), 0644)
	}
	fmt.Print(output)
	return nil
}

// generateTextReport 生成文本报告
func generateTextReport(report *OverallReport) string {
	var builder strings.Builder

	builder.WriteString("=== WES Contract Verification Report ===\n\n")

	// 摘要
	builder.WriteString("Summary:\n")
	builder.WriteString(fmt.Sprintf("  Total Files: %d\n", report.Summary.TotalFiles))
	builder.WriteString(fmt.Sprintf("  Success: %d, Failed: %d\n",
		report.Summary.SuccessFiles, report.Summary.FailedFiles))
	builder.WriteString(fmt.Sprintf("  Total Issues: %d\n", report.Summary.TotalIssues))
	builder.WriteString(fmt.Sprintf("  Overall Score: %.1f/100\n\n", report.Summary.OverallScore))

	// 问题分布
	builder.WriteString("Issue Distribution:\n")
	builder.WriteString(fmt.Sprintf("  Critical: %d\n", report.Summary.CriticalIssues))
	builder.WriteString(fmt.Sprintf("  High: %d\n", report.Summary.HighIssues))
	builder.WriteString(fmt.Sprintf("  Medium: %d\n", report.Summary.MediumIssues))
	builder.WriteString(fmt.Sprintf("  Low: %d\n\n", report.Summary.LowIssues))

	// 详细结果
	if len(report.Results) > 0 {
		builder.WriteString("Detailed Results:\n")
		for _, result := range report.Results {
			status := "✓ PASS"
			if !result.Success {
				status = "✗ FAIL"
			}
			builder.WriteString(fmt.Sprintf("  %s %s (%d issues)\n",
				status, result.File, len(result.Issues)))
		}
		builder.WriteString("\n")
	}

	// 建议
	if len(report.Recommendations) > 0 {
		builder.WriteString("Recommendations:\n")
		for _, rec := range report.Recommendations {
			builder.WriteString(fmt.Sprintf("  - %s\n", rec))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(fmt.Sprintf("Generated at: %s\n",
		report.GeneratedAt.Format("2006-01-02 15:04:05")))

	return builder.String()
}
