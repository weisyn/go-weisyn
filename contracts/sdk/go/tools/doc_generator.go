package tools

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ==================== WES 合约文档生成工具 ====================
//
// 🌟 **设计理念**：为WES合约自动生成完整的API文档
//
// 🎯 **核心特性**：
// - 从Go源码自动提取合约接口信息
// - 生成标准化的API文档
// - 支持多种输出格式（Markdown、HTML、JSON）
// - 内置合约规范验证
// - 生成开发者友好的使用示例
//

// ==================== 文档数据结构 ====================

// ContractDoc 合约文档
type ContractDoc struct {
	Name        string
	Version     string
	Description string
	Author      string
	License     string

	// 接口信息
	Interfaces []InterfaceDoc
	Functions  []FunctionDoc
	Events     []EventDoc
	Types      []TypeDoc

	// 元数据
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SourceFiles []string
}

// InterfaceDoc 接口文档
type InterfaceDoc struct {
	Name        string
	Description string
	Functions   []string
	Inherited   []string
}

// FunctionDoc 函数文档
type FunctionDoc struct {
	Name        string
	Description string
	Signature   string
	Parameters  []ParameterDoc
	Returns     []ReturnDoc
	Events      []string
	Examples    []ExampleDoc
	Notes       []string
}

// ParameterDoc 参数文档
type ParameterDoc struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     string
}

// ReturnDoc 返回值文档
type ReturnDoc struct {
	Name        string
	Type        string
	Description string
}

// EventDoc 事件文档
type EventDoc struct {
	Name        string
	Description string
	Fields      []EventFieldDoc
	Examples    []ExampleDoc
}

// EventFieldDoc 事件字段文档
type EventFieldDoc struct {
	Name        string
	Type        string
	Description string
	Indexed     bool
}

// TypeDoc 类型文档
type TypeDoc struct {
	Name        string
	Description string
	Definition  string
	Fields      []FieldDoc
	Methods     []FunctionDoc
}

// FieldDoc 字段文档
type FieldDoc struct {
	Name        string
	Type        string
	Description string
	Tags        map[string]string
}

// ExampleDoc 示例文档
type ExampleDoc struct {
	Title       string
	Description string
	Code        string
	Language    string
}

// ==================== 文档生成器 ====================

// DocGenerator 文档生成器
type DocGenerator struct {
	config     *DocConfig
	extractors map[string]SourceExtractor
	formatters map[string]DocFormatter
}

// DocConfig 文档生成配置
type DocConfig struct {
	ProjectName string
	Version     string
	OutputDir   string
	TemplateDir string

	// 生成选项
	IncludePrivate  bool
	IncludeExamples bool
	IncludeSource   bool
	GenerateIndex   bool

	// 输出格式
	OutputFormats []string // markdown, html, json
	Theme         string
	Language      string
}

// DefaultDocConfig 默认文档配置
func DefaultDocConfig() *DocConfig {
	return &DocConfig{
		ProjectName:     "WES Contract",
		Version:         "1.0.0",
		OutputDir:       "./docs",
		TemplateDir:     "./templates",
		IncludePrivate:  false,
		IncludeExamples: true,
		IncludeSource:   false,
		GenerateIndex:   true,
		OutputFormats:   []string{"markdown", "html"},
		Theme:           "default",
		Language:        "zh-CN",
	}
}

// NewDocGenerator 创建文档生成器
func NewDocGenerator(config *DocConfig) *DocGenerator {
	if config == nil {
		config = DefaultDocConfig()
	}

	generator := &DocGenerator{
		config:     config,
		extractors: make(map[string]SourceExtractor),
		formatters: make(map[string]DocFormatter),
	}

	// 注册默认提取器和格式化器
	generator.RegisterExtractor("go", &GoSourceExtractor{})
	generator.RegisterFormatter("markdown", &MarkdownFormatter{})
	generator.RegisterFormatter("html", &HTMLFormatter{})
	generator.RegisterFormatter("json", &JSONFormatter{})

	return generator
}

// RegisterExtractor 注册源码提取器
func (dg *DocGenerator) RegisterExtractor(language string, extractor SourceExtractor) {
	dg.extractors[language] = extractor
}

// RegisterFormatter 注册文档格式化器
func (dg *DocGenerator) RegisterFormatter(format string, formatter DocFormatter) {
	dg.formatters[format] = formatter
}

// GenerateDoc 生成文档
func (dg *DocGenerator) GenerateDoc(sourceFiles []string) (*ContractDoc, error) {
	doc := &ContractDoc{
		Name:        dg.config.ProjectName,
		Version:     dg.config.Version,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		SourceFiles: sourceFiles,
	}

	// 提取源码信息
	for _, file := range sourceFiles {
		if err := dg.extractFromFile(file, doc); err != nil {
			return nil, fmt.Errorf("failed to extract from %s: %w", file, err)
		}
	}

	// 后处理和验证
	dg.postProcessDoc(doc)

	return doc, nil
}

// extractFromFile 从文件提取信息
func (dg *DocGenerator) extractFromFile(file string, doc *ContractDoc) error {
	// 根据文件扩展名选择提取器
	ext := getFileExtension(file)
	extractor, exists := dg.extractors[ext]
	if !exists {
		return fmt.Errorf("no extractor for file type: %s", ext)
	}

	return extractor.Extract(file, doc)
}

// postProcessDoc 后处理文档
func (dg *DocGenerator) postProcessDoc(doc *ContractDoc) {
	// 排序函数列表
	sort.Slice(doc.Functions, func(i, j int) bool {
		return doc.Functions[i].Name < doc.Functions[j].Name
	})

	// 排序事件列表
	sort.Slice(doc.Events, func(i, j int) bool {
		return doc.Events[i].Name < doc.Events[j].Name
	})

	// 验证接口一致性
	dg.validateInterfaces(doc)

	// 生成示例代码
	if dg.config.IncludeExamples {
		dg.generateExamples(doc)
	}
}

// validateInterfaces 验证接口一致性
func (dg *DocGenerator) validateInterfaces(doc *ContractDoc) {
	// 验证函数是否实现了声明的接口
	for _, iface := range doc.Interfaces {
		for _, funcName := range iface.Functions {
			found := false
			for _, function := range doc.Functions {
				if function.Name == funcName {
					found = true
					break
				}
			}
			if !found {
				function := FunctionDoc{
					Name:        funcName,
					Description: fmt.Sprintf("Required by interface %s (not implemented)", iface.Name),
					Signature:   funcName + "()",
				}
				doc.Functions = append(doc.Functions, function)
			}
		}
	}
}

// generateExamples 生成示例代码
func (dg *DocGenerator) generateExamples(doc *ContractDoc) {
	for i := range doc.Functions {
		if len(doc.Functions[i].Examples) == 0 {
			example := dg.generateFunctionExample(&doc.Functions[i])
			if example != nil {
				doc.Functions[i].Examples = append(doc.Functions[i].Examples, *example)
			}
		}
	}
}

// generateFunctionExample 生成函数示例
func (dg *DocGenerator) generateFunctionExample(function *FunctionDoc) *ExampleDoc {
	// 简化的示例生成
	example := &ExampleDoc{
		Title:       "基本用法",
		Description: fmt.Sprintf("如何调用 %s 函数", function.Name),
		Language:    "go",
	}

	// 生成示例代码
	var codeBuilder strings.Builder
	codeBuilder.WriteString("// 调用合约函数\n")
	codeBuilder.WriteString(fmt.Sprintf("result := contract.%s(", function.Name))

	for i, param := range function.Parameters {
		if i > 0 {
			codeBuilder.WriteString(", ")
		}
		codeBuilder.WriteString(generateExampleValue(param.Type))
	}

	codeBuilder.WriteString(")\n")
	codeBuilder.WriteString("if result != SUCCESS {\n")
	codeBuilder.WriteString("    return result\n")
	codeBuilder.WriteString("}")

	example.Code = codeBuilder.String()
	return example
}

// generateExampleValue 生成示例值
func generateExampleValue(paramType string) string {
	switch paramType {
	case "string":
		return `"example_value"`
	case "uint64", "Amount":
		return "1000"
	case "Address":
		return "exampleAddress"
	case "TokenID":
		return `"TOKEN_ID"`
	case "bool":
		return "true"
	default:
		return "nil"
	}
}

// ==================== 源码提取器接口 ====================

// SourceExtractor 源码提取器接口
type SourceExtractor interface {
	Extract(filename string, doc *ContractDoc) error
}

// GoSourceExtractor Go源码提取器
type GoSourceExtractor struct{}

// Extract 提取Go源码信息
func (gse *GoSourceExtractor) Extract(filename string, doc *ContractDoc) error {
	// 简化的Go源码解析实现
	// 实际项目中应使用go/ast包进行完整的AST分析

	// 模拟提取的函数信息
	functions := []FunctionDoc{
		{
			Name:        "Initialize",
			Description: "初始化合约",
			Signature:   "Initialize() uint32",
			Parameters:  []ParameterDoc{},
			Returns: []ReturnDoc{
				{Name: "errorCode", Type: "uint32", Description: "错误码，0表示成功"},
			},
		},
		{
			Name:        "Transfer",
			Description: "转账代币",
			Signature:   "Transfer() uint32",
			Parameters: []ParameterDoc{
				{Name: "to", Type: "Address", Description: "接收者地址", Required: true},
				{Name: "amount", Type: "Amount", Description: "转账金额", Required: true},
			},
			Returns: []ReturnDoc{
				{Name: "errorCode", Type: "uint32", Description: "错误码，0表示成功"},
			},
		},
	}

	doc.Functions = append(doc.Functions, functions...)

	// 模拟提取的事件信息
	events := []EventDoc{
		{
			Name:        "Transfer",
			Description: "代币转账事件",
			Fields: []EventFieldDoc{
				{Name: "from", Type: "Address", Description: "发送者地址", Indexed: true},
				{Name: "to", Type: "Address", Description: "接收者地址", Indexed: true},
				{Name: "amount", Type: "Amount", Description: "转账金额", Indexed: false},
			},
		},
	}

	doc.Events = append(doc.Events, events...)

	return nil
}

// ==================== 文档格式化器接口 ====================

// DocFormatter 文档格式化器接口
type DocFormatter interface {
	Format(doc *ContractDoc) (string, error)
}

// MarkdownFormatter Markdown格式化器
type MarkdownFormatter struct{}

// Format 格式化为Markdown
func (mf *MarkdownFormatter) Format(doc *ContractDoc) (string, error) {
	var builder strings.Builder

	// 标题和基本信息
	builder.WriteString(fmt.Sprintf("# %s\n\n", doc.Name))
	builder.WriteString(fmt.Sprintf("**版本**: %s\n", doc.Version))
	builder.WriteString(fmt.Sprintf("**描述**: %s\n", doc.Description))
	builder.WriteString(fmt.Sprintf("**作者**: %s\n", doc.Author))
	builder.WriteString(fmt.Sprintf("**许可证**: %s\n\n", doc.License))

	// 函数列表
	builder.WriteString("## 函数接口\n\n")
	for _, function := range doc.Functions {
		builder.WriteString(fmt.Sprintf("### %s\n\n", function.Name))
		builder.WriteString(fmt.Sprintf("**描述**: %s\n\n", function.Description))
		builder.WriteString(fmt.Sprintf("**签名**: `%s`\n\n", function.Signature))

		if len(function.Parameters) > 0 {
			builder.WriteString("**参数**:\n")
			for _, param := range function.Parameters {
				required := ""
				if param.Required {
					required = " (必需)"
				}
				builder.WriteString(fmt.Sprintf("- `%s` (%s)%s: %s\n",
					param.Name, param.Type, required, param.Description))
			}
			builder.WriteString("\n")
		}

		if len(function.Returns) > 0 {
			builder.WriteString("**返回值**:\n")
			for _, ret := range function.Returns {
				builder.WriteString(fmt.Sprintf("- `%s` (%s): %s\n",
					ret.Name, ret.Type, ret.Description))
			}
			builder.WriteString("\n")
		}

		if len(function.Examples) > 0 {
			builder.WriteString("**示例**:\n")
			for _, example := range function.Examples {
				builder.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n",
					example.Language, example.Code))
			}
		}
	}

	// 事件列表
	if len(doc.Events) > 0 {
		builder.WriteString("## 事件\n\n")
		for _, event := range doc.Events {
			builder.WriteString(fmt.Sprintf("### %s\n\n", event.Name))
			builder.WriteString(fmt.Sprintf("**描述**: %s\n\n", event.Description))

			if len(event.Fields) > 0 {
				builder.WriteString("**字段**:\n")
				for _, field := range event.Fields {
					indexed := ""
					if field.Indexed {
						indexed = " (索引)"
					}
					builder.WriteString(fmt.Sprintf("- `%s` (%s)%s: %s\n",
						field.Name, field.Type, indexed, field.Description))
				}
				builder.WriteString("\n")
			}
		}
	}

	// 生成时间
	builder.WriteString(fmt.Sprintf("---\n*文档生成时间: %s*\n",
		doc.UpdatedAt.Format("2006-01-02 15:04:05")))

	return builder.String(), nil
}

// HTMLFormatter HTML格式化器
type HTMLFormatter struct{}

// Format 格式化为HTML
func (hf *HTMLFormatter) Format(doc *ContractDoc) (string, error) {
	var builder strings.Builder

	builder.WriteString("<!DOCTYPE html>\n")
	builder.WriteString("<html lang=\"zh-CN\">\n")
	builder.WriteString("<head>\n")
	builder.WriteString("<meta charset=\"UTF-8\">\n")
	builder.WriteString(fmt.Sprintf("<title>%s - API文档</title>\n", doc.Name))
	builder.WriteString("<style>\n")
	builder.WriteString(getDefaultCSS())
	builder.WriteString("</style>\n")
	builder.WriteString("</head>\n")
	builder.WriteString("<body>\n")

	// 页面内容
	builder.WriteString(fmt.Sprintf("<h1>%s</h1>\n", doc.Name))
	builder.WriteString("<div class=\"info\">\n")
	builder.WriteString(fmt.Sprintf("<p><strong>版本</strong>: %s</p>\n", doc.Version))
	builder.WriteString(fmt.Sprintf("<p><strong>描述</strong>: %s</p>\n", doc.Description))
	builder.WriteString("</div>\n")

	// 函数列表
	builder.WriteString("<h2>函数接口</h2>\n")
	for _, function := range doc.Functions {
		builder.WriteString("<div class=\"function\">\n")
		builder.WriteString(fmt.Sprintf("<h3>%s</h3>\n", function.Name))
		builder.WriteString(fmt.Sprintf("<p>%s</p>\n", function.Description))
		builder.WriteString(fmt.Sprintf("<code>%s</code>\n", function.Signature))
		builder.WriteString("</div>\n")
	}

	builder.WriteString("</body>\n")
	builder.WriteString("</html>\n")

	return builder.String(), nil
}

// JSONFormatter JSON格式化器
type JSONFormatter struct{}

// Format 格式化为JSON
func (jf *JSONFormatter) Format(doc *ContractDoc) (string, error) {
	// 简化的JSON序列化
	var builder strings.Builder

	builder.WriteString("{\n")
	builder.WriteString(fmt.Sprintf("  \"name\": \"%s\",\n", doc.Name))
	builder.WriteString(fmt.Sprintf("  \"version\": \"%s\",\n", doc.Version))
	builder.WriteString(fmt.Sprintf("  \"description\": \"%s\",\n", doc.Description))

	builder.WriteString("  \"functions\": [\n")
	for i, function := range doc.Functions {
		if i > 0 {
			builder.WriteString(",\n")
		}
		builder.WriteString("    {\n")
		builder.WriteString(fmt.Sprintf("      \"name\": \"%s\",\n", function.Name))
		builder.WriteString(fmt.Sprintf("      \"description\": \"%s\",\n", function.Description))
		builder.WriteString(fmt.Sprintf("      \"signature\": \"%s\"\n", function.Signature))
		builder.WriteString("    }")
	}
	builder.WriteString("\n  ]\n")

	builder.WriteString("}\n")

	return builder.String(), nil
}

// ==================== 辅助工具函数 ====================

// getFileExtension 获取文件扩展名
func getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// getDefaultCSS 获取默认CSS样式
func getDefaultCSS() string {
	return `
body {
    font-family: Arial, sans-serif;
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
    line-height: 1.6;
}

h1, h2, h3 {
    color: #2c3e50;
}

.info {
    background-color: #f8f9fa;
    padding: 15px;
    border-radius: 5px;
    margin-bottom: 20px;
}

.function {
    border: 1px solid #ddd;
    padding: 15px;
    margin-bottom: 15px;
    border-radius: 5px;
}

code {
    background-color: #f4f4f4;
    padding: 2px 4px;
    border-radius: 3px;
    font-family: monospace;
}
`
}

// ValidateDocumentation 验证文档完整性
func ValidateDocumentation(doc *ContractDoc) []string {
	var issues []string

	// 检查基本信息
	if doc.Name == "" {
		issues = append(issues, "合约名称不能为空")
	}
	if doc.Version == "" {
		issues = append(issues, "合约版本不能为空")
	}
	if doc.Description == "" {
		issues = append(issues, "合约描述不能为空")
	}

	// 检查函数文档
	for _, function := range doc.Functions {
		if function.Description == "" {
			issues = append(issues, fmt.Sprintf("函数 %s 缺少描述", function.Name))
		}
		if function.Signature == "" {
			issues = append(issues, fmt.Sprintf("函数 %s 缺少签名", function.Name))
		}
	}

	return issues
}

// GenerateTableOfContents 生成目录
func GenerateTableOfContents(doc *ContractDoc) string {
	var builder strings.Builder

	builder.WriteString("## 目录\n\n")

	if len(doc.Functions) > 0 {
		builder.WriteString("### 函数接口\n")
		for _, function := range doc.Functions {
			builder.WriteString(fmt.Sprintf("- [%s](#%s)\n",
				function.Name, strings.ToLower(function.Name)))
		}
		builder.WriteString("\n")
	}

	if len(doc.Events) > 0 {
		builder.WriteString("### 事件\n")
		for _, event := range doc.Events {
			builder.WriteString(fmt.Sprintf("- [%s](#%s)\n",
				event.Name, strings.ToLower(event.Name)))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}
