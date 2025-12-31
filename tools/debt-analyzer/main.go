// Package main provides a tool for analyzing architectural debt.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DebtAnalyzer 架构债务分析器
type DebtAnalyzer struct {
	rootDir string
	fileSet *token.FileSet
	debts   []ArchitecturalDebt
	rules   []DebtRule
	config  *DebtConfig
}

// ArchitecturalDebt 架构债务记录
type ArchitecturalDebt struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	File         string    `json:"file"`
	Line         int       `json:"line"`
	Description  string    `json:"description"`
	Severity     string    `json:"severity"`
	DebtPoints   int       `json:"debt_points"`
	EstimatedFix string    `json:"estimated_fix"`
	CreatedAt    time.Time `json:"created_at"`
	Category     string    `json:"category"`
	Impact       string    `json:"impact"`
	Effort       string    `json:"effort"`
	Priority     string    `json:"priority"`
}

// DebtRule 债务检测规则接口
type DebtRule interface {
	Name() string
	Analyze(analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt
	GetDebtPoints(debtType string) int
}

// DebtConfig 债务分析配置
type DebtConfig struct {
	MaxDebtPoints     int               `json:"max_debt_points"`
	DebtPointsMapping map[string]int    `json:"debt_points_mapping"`
	Categories        []string          `json:"categories"`
	PriorityMatrix    map[string]string `json:"priority_matrix"`
}

// NewDebtAnalyzer 创建债务分析器
func NewDebtAnalyzer(rootDir string) *DebtAnalyzer {
	return &DebtAnalyzer{
		rootDir: rootDir,
		fileSet: token.NewFileSet(),
		debts:   make([]ArchitecturalDebt, 0),
		config:  getDefaultDebtConfig(),
		rules: []DebtRule{
			&ComplexityDebtRule{},
			&CouplingDebtRule{},
			&CohesionDebtRule{},
			&TestabilityDebtRule{},
			&MaintenanceDebtRule{},
			&PerformanceDebtRule{},
			&SecurityDebtRule{},
		},
	}
}

// getDefaultDebtConfig 获取默认债务配置
func getDefaultDebtConfig() *DebtConfig {
	return &DebtConfig{
		MaxDebtPoints: 100,
		DebtPointsMapping: map[string]int{
			"HighComplexity":        10,
			"TightCoupling":         8,
			"LowCohesion":           6,
			"PoorTestability":       5,
			"MaintenanceIssue":      4,
			"PerformanceIssue":      7,
			"SecurityVulnerability": 15,
		},
		Categories: []string{
			"Architecture", "Design", "Implementation",
			"Testing", "Performance", "Security",
		},
		PriorityMatrix: map[string]string{
			"HighHigh":     "P0",
			"HighMedium":   "P1",
			"MediumHigh":   "P1",
			"MediumMedium": "P2",
			"LowHigh":      "P2",
			"HighLow":      "P3",
			"MediumLow":    "P3",
			"LowMedium":    "P3",
			"LowLow":       "P4",
		},
	}
}

// ComplexityDebtRule 复杂度债务规则
type ComplexityDebtRule struct{}

func (r *ComplexityDebtRule) Name() string {
	return "ComplexityDebt"
}

func (r *ComplexityDebtRule) GetDebtPoints(debtType string) int {
	switch debtType {
	case "HighCyclomaticComplexity":
		return 10
	case "DeepNesting":
		return 8
	case "LongMethod":
		return 6
	case "LargeClass":
		return 8
	default:
		return 5
	}
}

func (r *ComplexityDebtRule) Analyze(analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	debts := make([]ArchitecturalDebt, 0)

	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			// 分析函数复杂度
			complexity := r.calculateCyclomaticComplexity(funcDecl)
			if complexity > 15 {
				debt := ArchitecturalDebt{
					ID:           fmt.Sprintf("DEBT-%s-%d", filepath.Base(file), analyzer.fileSet.Position(funcDecl.Pos()).Line),
					Type:         "HighCyclomaticComplexity",
					File:         file,
					Line:         analyzer.fileSet.Position(funcDecl.Pos()).Line,
					Description:  fmt.Sprintf("函数 %s 的圈复杂度过高 (%d)，建议重构", funcDecl.Name.Name, complexity),
					Severity:     "HIGH",
					DebtPoints:   r.GetDebtPoints("HighCyclomaticComplexity"),
					EstimatedFix: "2-4小时",
					CreatedAt:    time.Now(),
					Category:     "Design",
					Impact:       "HIGH",
					Effort:       "MEDIUM",
					Priority:     analyzer.config.PriorityMatrix["HighMedium"],
				}
				debts = append(debts, debt)
			}

			// 检查嵌套深度
			depth := r.calculateNestingDepth(funcDecl.Body)
			if depth > 5 {
				debt := ArchitecturalDebt{
					ID:           fmt.Sprintf("DEBT-%s-%d-nesting", filepath.Base(file), analyzer.fileSet.Position(funcDecl.Pos()).Line),
					Type:         "DeepNesting",
					File:         file,
					Line:         analyzer.fileSet.Position(funcDecl.Pos()).Line,
					Description:  fmt.Sprintf("函数 %s 嵌套层次过深 (%d)，影响可读性", funcDecl.Name.Name, depth),
					Severity:     "MEDIUM",
					DebtPoints:   r.GetDebtPoints("DeepNesting"),
					EstimatedFix: "1-2小时",
					CreatedAt:    time.Now(),
					Category:     "Implementation",
					Impact:       "MEDIUM",
					Effort:       "LOW",
					Priority:     analyzer.config.PriorityMatrix["MediumLow"],
				}
				debts = append(debts, debt)
			}
		}

		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				// 检查结构体大小
				fieldCount := len(structType.Fields.List)
				if fieldCount > 20 {
					debt := ArchitecturalDebt{
						ID:           fmt.Sprintf("DEBT-%s-%d-large", filepath.Base(file), analyzer.fileSet.Position(typeSpec.Pos()).Line),
						Type:         "LargeClass",
						File:         file,
						Line:         analyzer.fileSet.Position(typeSpec.Pos()).Line,
						Description:  fmt.Sprintf("结构体 %s 字段过多 (%d)，可能违反单一职责原则", typeSpec.Name.Name, fieldCount),
						Severity:     "MEDIUM",
						DebtPoints:   r.GetDebtPoints("LargeClass"),
						EstimatedFix: "4-8小时",
						CreatedAt:    time.Now(),
						Category:     "Design",
						Impact:       "MEDIUM",
						Effort:       "HIGH",
						Priority:     analyzer.config.PriorityMatrix["MediumHigh"],
					}
					debts = append(debts, debt)
				}
			}
		}

		return true
	})

	return debts
}

func (r *ComplexityDebtRule) calculateCyclomaticComplexity(funcDecl *ast.FuncDecl) int {
	complexity := 1 // 基础复杂度

	if funcDecl.Body == nil {
		return complexity
	}

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.TypeSwitchStmt, *ast.SwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})

	return complexity
}

func (r *ComplexityDebtRule) calculateNestingDepth(block *ast.BlockStmt) int {
	if block == nil {
		return 0
	}

	maxDepth := 0
	r.calculateNestingDepthRecursive(block, 1, &maxDepth)
	return maxDepth
}

func (r *ComplexityDebtRule) calculateNestingDepthRecursive(node ast.Node, currentDepth int, maxDepth *int) {
	if currentDepth > *maxDepth {
		*maxDepth = currentDepth
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.IfStmt:
			if stmt.Body != nil {
				r.calculateNestingDepthRecursive(stmt.Body, currentDepth+1, maxDepth)
			}
			if stmt.Else != nil {
				r.calculateNestingDepthRecursive(stmt.Else, currentDepth+1, maxDepth)
			}
			return false
		case *ast.ForStmt:
			if stmt.Body != nil {
				r.calculateNestingDepthRecursive(stmt.Body, currentDepth+1, maxDepth)
			}
			return false
		case *ast.RangeStmt:
			if stmt.Body != nil {
				r.calculateNestingDepthRecursive(stmt.Body, currentDepth+1, maxDepth)
			}
			return false
		}
		return true
	})
}

// CouplingDebtRule 耦合债务规则
type CouplingDebtRule struct{}

func (r *CouplingDebtRule) Name() string {
	return "CouplingDebt"
}

func (r *CouplingDebtRule) GetDebtPoints(debtType string) int {
	switch debtType {
	case "TightCoupling":
		return 8
	case "CircularDependency":
		return 12
	case "ExcessiveDependencies":
		return 6
	default:
		return 5
	}
}

func (r *CouplingDebtRule) Analyze(analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	debts := make([]ArchitecturalDebt, 0)

	// 分析导入依赖
	imports := make([]string, 0)
	ast.Inspect(node, func(n ast.Node) bool {
		if importSpec, ok := n.(*ast.ImportSpec); ok {
			importPath := strings.Trim(importSpec.Path.Value, "\"")
			imports = append(imports, importPath)
		}
		return true
	})

	// 检查过多的依赖
	if len(imports) > 15 {
		debt := ArchitecturalDebt{
			ID:           fmt.Sprintf("DEBT-%s-imports", filepath.Base(file)),
			Type:         "ExcessiveDependencies",
			File:         file,
			Line:         1,
			Description:  fmt.Sprintf("文件导入了过多的依赖 (%d)，可能存在职责不清晰的问题", len(imports)),
			Severity:     "MEDIUM",
			DebtPoints:   r.GetDebtPoints("ExcessiveDependencies"),
			EstimatedFix: "2-4小时",
			CreatedAt:    time.Now(),
			Category:     "Architecture",
			Impact:       "MEDIUM",
			Effort:       "MEDIUM",
			Priority:     analyzer.config.PriorityMatrix["MediumMedium"],
		}
		debts = append(debts, debt)
	}

	return debts
}

// CohesionDebtRule 内聚债务规则
type CohesionDebtRule struct{}

func (r *CohesionDebtRule) Name() string                      { return "CohesionDebt" }
func (r *CohesionDebtRule) GetDebtPoints(__debtType string) int { return 6 }
func (r *CohesionDebtRule) Analyze(__analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	return []ArchitecturalDebt{} // 简化实现
}

// TestabilityDebtRule 可测试性债务规则
type TestabilityDebtRule struct{}

func (r *TestabilityDebtRule) Name() string                      { return "TestabilityDebt" }
func (r *TestabilityDebtRule) GetDebtPoints(__debtType string) int { return 5 }
func (r *TestabilityDebtRule) Analyze(__analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	return []ArchitecturalDebt{} // 简化实现
}

// MaintenanceDebtRule 维护性债务规则
type MaintenanceDebtRule struct{}

func (r *MaintenanceDebtRule) Name() string                      { return "MaintenanceDebt" }
func (r *MaintenanceDebtRule) GetDebtPoints(__debtType string) int { return 4 }
func (r *MaintenanceDebtRule) Analyze(__analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	return []ArchitecturalDebt{} // 简化实现
}

// PerformanceDebtRule 性能债务规则
type PerformanceDebtRule struct{}

func (r *PerformanceDebtRule) Name() string                      { return "PerformanceDebt" }
func (r *PerformanceDebtRule) GetDebtPoints(__debtType string) int { return 7 }
func (r *PerformanceDebtRule) Analyze(__analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	return []ArchitecturalDebt{} // 简化实现
}

// SecurityDebtRule 安全债务规则
type SecurityDebtRule struct{}

func (r *SecurityDebtRule) Name() string                      { return "SecurityDebt" }
func (r *SecurityDebtRule) GetDebtPoints(__debtType string) int { return 15 }
func (r *SecurityDebtRule) Analyze(__analyzer *DebtAnalyzer, file string, node ast.Node) []ArchitecturalDebt {
	return []ArchitecturalDebt{} // 简化实现
}

// AnalyzeDirectory 分析目录中的架构债务
func (d *DebtAnalyzer) AnalyzeDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, __info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		return d.analyzeFile(path)
	})
}

// analyzeFile 分析单个文件的架构债务
func (d *DebtAnalyzer) analyzeFile(filename string) error {
	//nolint:gosec // G304: filename 来自命令行参数，用户可控但工具用途明确
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(d.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	// 应用所有债务检测规则
	for _, rule := range d.rules {
		debts := rule.Analyze(d, filename, file)
		d.debts = append(d.debts, debts...)
	}

	return nil
}

// GenerateReport 生成债务报告
func (d *DebtAnalyzer) GenerateReport() *DebtReport {
	report := &DebtReport{
		TotalDebts:      len(d.debts),
		TotalDebtPoints: d.calculateTotalDebtPoints(),
		GeneratedAt:     time.Now(),
		Summary:         d.generateSummary(),
		Categories:      d.groupByCategory(),
		Priorities:      d.groupByPriority(),
		TopDebts:        d.getTopDebts(10),
		Recommendations: d.generateRecommendations(),
	}

	return report
}

// DebtReport 债务报告
type DebtReport struct {
	TotalDebts      int                            `json:"total_debts"`
	TotalDebtPoints int                            `json:"total_debt_points"`
	GeneratedAt     time.Time                      `json:"generated_at"`
	Summary         DebtSummary                    `json:"summary"`
	Categories      map[string][]ArchitecturalDebt `json:"categories"`
	Priorities      map[string][]ArchitecturalDebt `json:"priorities"`
	TopDebts        []ArchitecturalDebt            `json:"top_debts"`
	Recommendations []string                       `json:"recommendations"`
}

// DebtSummary 债务摘要
type DebtSummary struct {
	HighSeverity   int `json:"high_severity"`
	MediumSeverity int `json:"medium_severity"`
	LowSeverity    int `json:"low_severity"`
	P0Count        int `json:"p0_count"`
	P1Count        int `json:"p1_count"`
	P2Count        int `json:"p2_count"`
}

func (d *DebtAnalyzer) calculateTotalDebtPoints() int {
	total := 0
	for _, debt := range d.debts {
		total += debt.DebtPoints
	}
	return total
}

func (d *DebtAnalyzer) generateSummary() DebtSummary {
	summary := DebtSummary{}

	for _, debt := range d.debts {
		switch debt.Severity {
		case "HIGH":
			summary.HighSeverity++
		case "MEDIUM":
			summary.MediumSeverity++
		case "LOW":
			summary.LowSeverity++
		}

		switch debt.Priority {
		case "P0":
			summary.P0Count++
		case "P1":
			summary.P1Count++
		case "P2":
			summary.P2Count++
		}
	}

	return summary
}

func (d *DebtAnalyzer) groupByCategory() map[string][]ArchitecturalDebt {
	categories := make(map[string][]ArchitecturalDebt)

	for _, debt := range d.debts {
		categories[debt.Category] = append(categories[debt.Category], debt)
	}

	return categories
}

func (d *DebtAnalyzer) groupByPriority() map[string][]ArchitecturalDebt {
	priorities := make(map[string][]ArchitecturalDebt)

	for _, debt := range d.debts {
		priorities[debt.Priority] = append(priorities[debt.Priority], debt)
	}

	return priorities
}

func (d *DebtAnalyzer) getTopDebts(limit int) []ArchitecturalDebt {
	// 按债务点数排序
	sortedDebts := make([]ArchitecturalDebt, len(d.debts))
	copy(sortedDebts, d.debts)

	sort.Slice(sortedDebts, func(i, j int) bool {
		return sortedDebts[i].DebtPoints > sortedDebts[j].DebtPoints
	})

	if len(sortedDebts) > limit {
		return sortedDebts[:limit]
	}
	return sortedDebts
}

func (d *DebtAnalyzer) generateRecommendations() []string {
	recommendations := make([]string, 0)

	totalPoints := d.calculateTotalDebtPoints()

	if totalPoints > d.config.MaxDebtPoints {
		recommendations = append(recommendations,
			fmt.Sprintf("总债务点数 (%d) 超过阈值 (%d)，建议优先处理高优先级债务",
				totalPoints, d.config.MaxDebtPoints))
	}

	// 按类别分析
	categories := d.groupByCategory()
	for category, debts := range categories {
		if len(debts) > 5 {
			recommendations = append(recommendations,
				fmt.Sprintf("%s 类别债务较多 (%d个)，建议制定专项整改计划",
					category, len(debts)))
		}
	}

	return recommendations
}

// SaveReportToFile 保存报告到文件
func (d *DebtAnalyzer) SaveReportToFile(report *DebtReport, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	//nolint:gosec // G306: 报告文件需要用户可读权限，0644 是合理的
	return os.WriteFile(filename, data, 0644)
}

// PrintReport 打印报告
func (d *DebtAnalyzer) PrintReport(report *DebtReport) {
	fmt.Println("🏗️  架构债务分析报告")
	fmt.Println("========================")
	fmt.Printf("📊 总债务数量: %d\n", report.TotalDebts)
	fmt.Printf("📈 总债务点数: %d\n", report.TotalDebtPoints)
	fmt.Printf("📅 生成时间: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))

	fmt.Println("📋 严重程度分布:")
	fmt.Printf("  🔴 高: %d\n", report.Summary.HighSeverity)
	fmt.Printf("  🟡 中: %d\n", report.Summary.MediumSeverity)
	fmt.Printf("  🟢 低: %d\n\n", report.Summary.LowSeverity)

	fmt.Println("🎯 优先级分布:")
	fmt.Printf("  P0: %d\n", report.Summary.P0Count)
	fmt.Printf("  P1: %d\n", report.Summary.P1Count)
	fmt.Printf("  P2: %d\n\n", report.Summary.P2Count)

	fmt.Println("🔥 Top 10 债务:")
	for i, debt := range report.TopDebts {
		fmt.Printf("  %d. %s:%d - %s (%d点)\n",
			i+1, filepath.Base(debt.File), debt.Line, debt.Description, debt.DebtPoints)
	}

	fmt.Println("\n💡 建议:")
	for i, rec := range report.Recommendations {
		fmt.Printf("  %d. %s\n", i+1, rec)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: debt-analyzer <目录路径>")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	analyzer := NewDebtAnalyzer(rootDir)

	fmt.Println("🔍 开始分析架构债务...")

	if err := analyzer.AnalyzeDirectory(rootDir); err != nil {
		fmt.Printf("❌ 分析失败: %v\n", err)
		os.Exit(1)
	}

	report := analyzer.GenerateReport()
	analyzer.PrintReport(report)

	// 保存详细报告到文件
	if err := analyzer.SaveReportToFile(report, "reports/debt-analysis.json"); err != nil {
		fmt.Printf("⚠️ 保存报告失败: %v\n", err)
	} else {
		fmt.Println("\n📄 详细报告已保存到: reports/debt-analysis.json")
	}
}
