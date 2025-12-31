// Package main provides a tool for tracking code evolution.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// EvolutionTracker 架构演进跟踪器
type EvolutionTracker struct {
	config  *EvolutionConfig
	changes []ArchitecturalChange
	trends  []EvolutionTrend
	metrics EvolutionMetrics
}

// ArchitecturalChange 架构变更记录
type ArchitecturalChange struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Author      string         `json:"author"`
	Type        string         `json:"type"`
	Scope       string         `json:"scope"`
	Description string         `json:"description"`
	Impact      string         `json:"impact"`
	Files       []string       `json:"files"`
	Metrics     map[string]int `json:"metrics"`
	Tags        []string       `json:"tags"`
}

// EvolutionTrend 演进趋势
type EvolutionTrend struct {
	Period      string         `json:"period"`
	ChangeTypes map[string]int `json:"change_types"`
	Velocity    float64        `json:"velocity"`
	Complexity  float64        `json:"complexity"`
	Quality     float64        `json:"quality"`
}

// EvolutionMetrics 演进指标
type EvolutionMetrics struct {
	TotalChanges      int               `json:"total_changes"`
	ChangeFrequency   float64           `json:"change_frequency"`
	AverageImpact     float64           `json:"average_impact"`
	TopChangeTypes    []ChangeTypeCount `json:"top_change_types"`
	ActiveAuthors     []AuthorActivity  `json:"active_authors"`
	ArchitecturalDebt float64           `json:"architectural_debt"`
	QualityTrend      string            `json:"quality_trend"`
}

// ChangeTypeCount 变更类型统计
type ChangeTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// AuthorActivity 作者活动统计
type AuthorActivity struct {
	Author  string `json:"author"`
	Changes int    `json:"changes"`
	Impact  string `json:"impact"`
}

// EvolutionConfig 演进配置
type EvolutionConfig struct {
	AnalysisPeriod    int      `json:"analysis_period"`    // 分析周期（天）
	ChangePatterns    []string `json:"change_patterns"`    // 变更模式
	ImpactKeywords    []string `json:"impact_keywords"`    // 影响关键词
	QualityIndicators []string `json:"quality_indicators"` // 质量指标
}

// NewEvolutionTracker 创建演进跟踪器
func NewEvolutionTracker() *EvolutionTracker {
	return &EvolutionTracker{
		config:  getDefaultEvolutionConfig(),
		changes: make([]ArchitecturalChange, 0),
		trends:  make([]EvolutionTrend, 0),
	}
}

// getDefaultEvolutionConfig 获取默认演进配置
func getDefaultEvolutionConfig() *EvolutionConfig {
	return &EvolutionConfig{
		AnalysisPeriod: 30,
		ChangePatterns: []string{
			"arch:", "refactor:", "design:", "interface:",
			"breaking:", "deprecate:", "optimize:",
		},
		ImpactKeywords: []string{
			"breaking", "major", "critical", "significant",
			"minor", "patch", "fix", "improvement",
		},
		QualityIndicators: []string{
			"test", "doc", "lint", "coverage", "performance",
		},
	}
}

// AnalyzeEvolution 分析架构演进
func (t *EvolutionTracker) AnalyzeEvolution(repoPath string) error {
	fmt.Println("🔍 分析架构演进...")

	// 获取 Git 提交历史
	if err := t.fetchGitHistory(repoPath); err != nil {
		return err
	}

	// 分析变更模式
	t.analyzeChangePatterns()

	// 计算演进趋势
	t.calculateTrends()

	// 计算演进指标
	t.calculateMetrics()

	return nil
}

// fetchGitHistory 获取 Git 历史
func (t *EvolutionTracker) fetchGitHistory(repoPath string) error {
	// 获取最近30天的提交
	since := time.Now().AddDate(0, 0, -t.config.AnalysisPeriod).Format("2006-01-02")

	//nolint:gosec // G204: git 命令参数来自格式化时间字符串，安全可控
	cmd := exec.Command("git", "log",
		"--since="+since,
		"--pretty=format:%H|%an|%ad|%s",
		"--date=iso",
		"--name-only")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取 Git 历史失败: %v", err)
	}

	return t.parseGitLog(string(output))
}

// parseGitLog 解析 Git 日志
func (t *EvolutionTracker) parseGitLog(gitLog string) error {
	lines := strings.Split(gitLog, "\n")
	var currentChange *ArchitecturalChange

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析提交信息
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				timestamp, err := time.Parse("2006-01-02 15:04:05 -0700", parts[2])
				if err != nil {
					continue
				}

				currentChange = &ArchitecturalChange{
					ID:          parts[0],
					Author:      parts[1],
					Timestamp:   timestamp,
					Description: parts[3],
					Files:       make([]string, 0),
					Metrics:     make(map[string]int),
					Tags:        make([]string, 0),
				}

				// 分析变更类型和影响
				t.analyzeChangeType(currentChange)
				t.analyzeImpact(currentChange)
			}
		} else if currentChange != nil {
			// 解析文件列表
			if line != "" {
				currentChange.Files = append(currentChange.Files, line)
			}
		}

		if currentChange != nil && (len(line) == 0 || strings.Contains(line, "|")) {
			if len(currentChange.Files) > 0 {
				t.changes = append(t.changes, *currentChange)
			}
		}
	}

	return nil
}

// analyzeChangeType 分析变更类型
func (t *EvolutionTracker) analyzeChangeType(change *ArchitecturalChange) {
	desc := strings.ToLower(change.Description)

	for _, pattern := range t.config.ChangePatterns {
		if strings.Contains(desc, pattern) {
			change.Type = strings.TrimSuffix(pattern, ":")
			break
		}
	}

	if change.Type == "" {
		// 根据文件类型推断
		for _, file := range change.Files {
			switch { //nolint:gocritic // ifElseChain: 使用 switch 更清晰
			case strings.Contains(file, "/interfaces/"):
				change.Type = "interface"
			case strings.Contains(file, "manager.go"):
				change.Type = "refactor"
			case strings.HasSuffix(file, "_test.go"):
				change.Type = "test"
			}
			if change.Type != "" {
				break
			}
		}
	}

	if change.Type == "" {
		change.Type = "other"
	}
}

// analyzeImpact 分析影响程度
func (t *EvolutionTracker) analyzeImpact(change *ArchitecturalChange) {
	desc := strings.ToLower(change.Description)

	switch { //nolint:gocritic // ifElseChain: 使用 switch 更清晰
	case strings.Contains(desc, "breaking") || strings.Contains(desc, "major"):
		change.Impact = "HIGH"
	case strings.Contains(desc, "significant") || strings.Contains(desc, "refactor"):
		change.Impact = "MEDIUM"
	default:
		change.Impact = "LOW"
	}

	// 根据文件数量调整影响
	if len(change.Files) > 10 {
		switch change.Impact { //nolint:staticcheck // QF1003: 使用 tagged switch 更清晰
		case "LOW":
			change.Impact = "MEDIUM"
		case "MEDIUM":
			change.Impact = "HIGH"
		}
	}
}

// analyzeChangePatterns 分析变更模式
func (t *EvolutionTracker) analyzeChangePatterns() {
	// 按作者分组分析
	authorChanges := make(map[string][]ArchitecturalChange)
	for _, change := range t.changes {
		authorChanges[change.Author] = append(authorChanges[change.Author], change)
	}

	// 按时间分组分析
	weeklyChanges := make(map[string][]ArchitecturalChange)
	for _, change := range t.changes {
		week := change.Timestamp.Format("2006-W02")
		weeklyChanges[week] = append(weeklyChanges[week], change)
	}

	// 分析文件热点
	fileChanges := make(map[string]int)
	for _, change := range t.changes {
		for _, file := range change.Files {
			fileChanges[file]++
		}
	}
}

// calculateTrends 计算演进趋势
func (t *EvolutionTracker) calculateTrends() {
	// 按周分组计算趋势
	weeklyData := make(map[string][]ArchitecturalChange)
	for _, change := range t.changes {
		week := change.Timestamp.Format("2006-W02")
		weeklyData[week] = append(weeklyData[week], change)
	}

	weeks := make([]string, 0, len(weeklyData))
	for week := range weeklyData {
		weeks = append(weeks, week)
	}
	sort.Strings(weeks)

	for _, week := range weeks {
		changes := weeklyData[week]

		trend := EvolutionTrend{
			Period:      week,
			ChangeTypes: make(map[string]int),
			Velocity:    float64(len(changes)),
		}

		// 统计变更类型
		for _, change := range changes {
			trend.ChangeTypes[change.Type]++
		}

		// 计算复杂度（基于文件变更数）
		totalFiles := 0
		for _, change := range changes {
			totalFiles += len(change.Files)
		}
		if len(changes) > 0 {
			trend.Complexity = float64(totalFiles) / float64(len(changes))
		}

		// 计算质量指标（基于测试和文档变更比例）
		qualityChanges := 0
		for _, change := range changes {
			for _, indicator := range t.config.QualityIndicators {
				if strings.Contains(strings.ToLower(change.Description), indicator) {
					qualityChanges++
					break
				}
			}
		}
		if len(changes) > 0 {
			trend.Quality = float64(qualityChanges) / float64(len(changes)) * 100
		}

		t.trends = append(t.trends, trend)
	}
}

// calculateMetrics 计算演进指标
func (t *EvolutionTracker) calculateMetrics() {
	t.metrics.TotalChanges = len(t.changes)

	if len(t.changes) > 0 {
		// 计算变更频率（每天）
		if len(t.trends) > 0 {
			totalDays := len(t.trends) * 7 // 按周计算
			t.metrics.ChangeFrequency = float64(t.metrics.TotalChanges) / float64(totalDays)
		}

		// 计算平均影响
		highImpact := 0
		mediumImpact := 0
		for _, change := range t.changes {
			switch change.Impact {
			case "HIGH":
				highImpact++
			case "MEDIUM":
				mediumImpact++
			}
		}
		t.metrics.AverageImpact = (float64(highImpact)*3 + float64(mediumImpact)*2) / float64(t.metrics.TotalChanges)

		// 统计变更类型
		typeCount := make(map[string]int)
		for _, change := range t.changes {
			typeCount[change.Type]++
		}

		for changeType, count := range typeCount {
			t.metrics.TopChangeTypes = append(t.metrics.TopChangeTypes, ChangeTypeCount{
				Type:  changeType,
				Count: count,
			})
		}

		sort.Slice(t.metrics.TopChangeTypes, func(i, j int) bool {
			return t.metrics.TopChangeTypes[i].Count > t.metrics.TopChangeTypes[j].Count
		})

		// 统计活跃作者
		authorCount := make(map[string]int)
		for _, change := range t.changes {
			authorCount[change.Author]++
		}

		for author, count := range authorCount {
			impact := "LOW"
			if count > 10 {
				impact = "HIGH"
			} else if count > 5 {
				impact = "MEDIUM"
			}

			t.metrics.ActiveAuthors = append(t.metrics.ActiveAuthors, AuthorActivity{
				Author:  author,
				Changes: count,
				Impact:  impact,
			})
		}

		sort.Slice(t.metrics.ActiveAuthors, func(i, j int) bool {
			return t.metrics.ActiveAuthors[i].Changes > t.metrics.ActiveAuthors[j].Changes
		})

		// 计算质量趋势
		if len(t.trends) >= 2 {
			recent := t.trends[len(t.trends)-1].Quality
			previous := t.trends[len(t.trends)-2].Quality

			switch { //nolint:gocritic // ifElseChain: 使用 switch 更清晰
			case recent > previous:
				t.metrics.QualityTrend = "IMPROVING"
			case recent < previous:
				t.metrics.QualityTrend = "DECLINING"
			default:
				t.metrics.QualityTrend = "STABLE"
			}
		}
	}
}

// GenerateReport 生成演进报告
func (t *EvolutionTracker) GenerateReport() *EvolutionReport {
	return &EvolutionReport{
		Summary:         t.metrics,
		Trends:          t.trends,
		Changes:         t.changes,
		Recommendations: t.generateRecommendations(),
		GeneratedAt:     time.Now(),
	}
}

// EvolutionReport 演进报告
type EvolutionReport struct {
	Summary         EvolutionMetrics      `json:"summary"`
	Trends          []EvolutionTrend      `json:"trends"`
	Changes         []ArchitecturalChange `json:"changes"`
	Recommendations []string              `json:"recommendations"`
	GeneratedAt     time.Time             `json:"generated_at"`
}

// generateRecommendations 生成建议
func (t *EvolutionTracker) generateRecommendations() []string {
	recommendations := make([]string, 0)

	// 基于变更频率的建议
	if t.metrics.ChangeFrequency > 2.0 {
		recommendations = append(recommendations, "变更频率较高，建议建立更严格的架构评审流程")
	}

	// 基于影响程度的建议
	if t.metrics.AverageImpact > 2.5 {
		recommendations = append(recommendations, "高影响变更较多，建议加强影响评估和测试覆盖")
	}

	// 基于质量趋势的建议
	switch t.metrics.QualityTrend {
	case "DECLINING":
		recommendations = append(recommendations, "代码质量呈下降趋势，建议加强代码审查和重构")
	case "STABLE":
		recommendations = append(recommendations, "质量保持稳定，建议继续保持当前实践")
	case "IMPROVING":
		recommendations = append(recommendations, "质量持续改善，建议总结最佳实践并推广")
	}

	// 基于变更类型的建议
	if len(t.metrics.TopChangeTypes) > 0 {
		topType := t.metrics.TopChangeTypes[0]
		if topType.Type == "refactor" && topType.Count > t.metrics.TotalChanges/3 {
			recommendations = append(recommendations, "重构活动频繁，建议制定系统性的重构计划")
		}
	}

	return recommendations
}

// SaveReport 保存报告
func (t *EvolutionTracker) SaveReport(report *EvolutionReport, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec // G306: 报告文件需要用户可读权限，0644 是合理的
	return os.WriteFile(filename, data, 0644)
}

// PrintReport 打印报告
func (t *EvolutionTracker) PrintReport(report *EvolutionReport) {
	fmt.Println("📈 架构演进分析报告")
	fmt.Println("====================")
	fmt.Printf("📊 总变更数: %d\n", report.Summary.TotalChanges)
	fmt.Printf("📅 变更频率: %.2f 次/天\n", report.Summary.ChangeFrequency)
	fmt.Printf("💥 平均影响: %.2f\n", report.Summary.AverageImpact)
	fmt.Printf("📈 质量趋势: %s\n\n", report.Summary.QualityTrend)

	fmt.Println("🔥 热门变更类型:")
	for i, changeType := range report.Summary.TopChangeTypes {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s: %d 次\n", i+1, changeType.Type, changeType.Count)
	}

	fmt.Println("\n👥 活跃贡献者:")
	for i, author := range report.Summary.ActiveAuthors {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s: %d 次变更 (%s 影响)\n", i+1, author.Author, author.Changes, author.Impact)
	}

	fmt.Println("\n💡 建议:")
	for i, rec := range report.Recommendations {
		fmt.Printf("  %d. %s\n", i+1, rec)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: evolution-tracker <仓库路径>")
		os.Exit(1)
	}

	repoPath := os.Args[1]
	tracker := NewEvolutionTracker()

	if err := tracker.AnalyzeEvolution(repoPath); err != nil {
		fmt.Printf("❌ 分析失败: %v\n", err)
		os.Exit(1)
	}

	report := tracker.GenerateReport()
	tracker.PrintReport(report)

	// 保存详细报告
	//nolint:gosec // G301: 报告目录需要用户可读权限，0755 是合理的
	if err := os.MkdirAll("reports", 0755); err == nil {
		if err := tracker.SaveReport(report, "reports/evolution-report.json"); err != nil {
			fmt.Printf("⚠️ 保存报告失败: %v\n", err)
		} else {
			fmt.Println("\n📄 详细报告已保存到: reports/evolution-report.json")
		}
	}
}
