// Package main provides a documentation generation tool.
package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// DocGenerator 文档生成器
type DocGenerator struct {
	rootDir   string
	fileSet   *token.FileSet
	packages  map[string]*PackageDoc
	config    *DocConfig
	templates map[string]*template.Template
}

// PackageDoc 包文档
type PackageDoc struct {
	Name       string         `json:"name"`
	ImportPath string         `json:"import_path"`
	Synopsis   string         `json:"synopsis"`
	Doc        string         `json:"doc"`
	Interfaces []InterfaceDoc `json:"interfaces"`
	Types      []TypeDoc      `json:"types"`
	Functions  []FunctionDoc  `json:"functions"`
	Constants  []ConstantDoc  `json:"constants"`
	Variables  []VariableDoc  `json:"variables"`
	Examples   []ExampleDoc   `json:"examples"`
	Coverage   CoverageInfo   `json:"coverage"`
}

// InterfaceDoc 接口文档
type InterfaceDoc struct {
	Name          string       `json:"name"`
	Doc           string       `json:"doc"`
	Methods       []MethodDoc  `json:"methods"`
	Examples      []ExampleDoc `json:"examples"`
	UsageGuide    string       `json:"usage_guide"`
	BestPractices []string     `json:"best_practices"`
}

// MethodDoc 方法文档
type MethodDoc struct {
	Name       string       `json:"name"`
	Doc        string       `json:"doc"`
	Signature  string       `json:"signature"`
	Parameters []ParamDoc   `json:"parameters"`
	Returns    []ReturnDoc  `json:"returns"`
	Examples   []ExampleDoc `json:"examples"`
	Notes      []string     `json:"notes"`
}

// TypeDoc 类型文档
type TypeDoc struct {
	Name     string       `json:"name"`
	Doc      string       `json:"doc"`
	Type     string       `json:"type"`
	Fields   []FieldDoc   `json:"fields"`
	Methods  []MethodDoc  `json:"methods"`
	Examples []ExampleDoc `json:"examples"`
}

// FunctionDoc 函数文档
type FunctionDoc struct {
	Name       string       `json:"name"`
	Doc        string       `json:"doc"`
	Signature  string       `json:"signature"`
	Parameters []ParamDoc   `json:"parameters"`
	Returns    []ReturnDoc  `json:"returns"`
	Examples   []ExampleDoc `json:"examples"`
}

// ConstantDoc 常量文档
type ConstantDoc struct {
	Name  string `json:"name"`
	Doc   string `json:"doc"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// VariableDoc 变量文档
type VariableDoc struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
	Type string `json:"type"`
}

// FieldDoc 字段文档
type FieldDoc struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

// ParamDoc 参数文档
type ParamDoc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ReturnDoc 返回值文档
type ReturnDoc struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ExampleDoc 示例文档
type ExampleDoc struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Output      string `json:"output"`
	Description string `json:"description"`
}

// CoverageInfo 文档覆盖率信息
type CoverageInfo struct {
	InterfacesCovered int     `json:"interfaces_covered"`
	TotalInterfaces   int     `json:"total_interfaces"`
	MethodsCovered    int     `json:"methods_covered"`
	TotalMethods      int     `json:"total_methods"`
	ExamplesCovered   int     `json:"examples_covered"`
	TotalExamples     int     `json:"total_examples"`
	OverallCoverage   float64 `json:"overall_coverage"`
}

// DocConfig 文档配置
type DocConfig struct {
	OutputDir        string   `json:"output_dir"`
	IncludePrivate   bool     `json:"include_private"`
	GenerateExamples bool     `json:"generate_examples"`
	ValidateExamples bool     `json:"validate_examples"`
	RequiredSections []string `json:"required_sections"`
	TemplateDir      string   `json:"template_dir"`
	OutputFormats    []string `json:"output_formats"`
}

// NewDocGenerator 创建文档生成器
func NewDocGenerator(rootDir string) *DocGenerator {
	return &DocGenerator{
		rootDir:   rootDir,
		fileSet:   token.NewFileSet(),
		packages:  make(map[string]*PackageDoc),
		config:    getDefaultDocConfig(),
		templates: make(map[string]*template.Template),
	}
}

// getDefaultDocConfig 获取默认文档配置
func getDefaultDocConfig() *DocConfig {
	return &DocConfig{
		OutputDir:        "docs/generated",
		IncludePrivate:   false,
		GenerateExamples: true,
		ValidateExamples: true,
		RequiredSections: []string{"Description", "Parameters", "Returns", "Examples"},
		TemplateDir:      "tools/doc-generator/templates",
		OutputFormats:    []string{"markdown", "html", "json"},
	}
}

// GenerateDocumentation 生成文档
func (g *DocGenerator) GenerateDocumentation() error {
	fmt.Println("🔍 扫描包...")
	if err := g.scanPackages(); err != nil {
		return err
	}

	fmt.Println("📝 解析文档...")
	if err := g.parseDocumentation(); err != nil {
		return err
	}

	fmt.Println("🧪 验证示例...")
	if g.config.ValidateExamples {
		if err := g.validateExamples(); err != nil {
			return err
		}
	}

	fmt.Println("📊 计算覆盖率...")
	g.calculateCoverage()

	fmt.Println("📄 生成文档文件...")
	if err := g.generateOutputFiles(); err != nil {
		return err
	}

	fmt.Println("✅ 文档生成完成！")
	return nil
}

// scanPackages 扫描包
func (g *DocGenerator) scanPackages() error {
	return filepath.Walk(g.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		// 检查是否包含 Go 文件
		files, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return err
		}

		if len(files) == 0 {
			return nil
		}

		// 排除测试文件和内部包（如果配置不包含私有包）
		goFiles := make([]string, 0)
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			if !g.config.IncludePrivate && strings.Contains(file, "/internal/") {
				continue
			}
			goFiles = append(goFiles, file)
		}

		if len(goFiles) == 0 {
			return nil
		}

		// 解析包
		pkgs, err := parser.ParseDir(g.fileSet, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for pkgName, pkg := range pkgs {
			if strings.HasSuffix(pkgName, "_test") {
				continue
			}

			packageDoc := &PackageDoc{
				Name:       pkgName,
				ImportPath: g.getImportPath(path),
				Interfaces: make([]InterfaceDoc, 0),
				Types:      make([]TypeDoc, 0),
				Functions:  make([]FunctionDoc, 0),
				Constants:  make([]ConstantDoc, 0),
				Variables:  make([]VariableDoc, 0),
				Examples:   make([]ExampleDoc, 0),
			}

			g.packages[packageDoc.ImportPath] = packageDoc

			// 使用 go/doc 提取文档
			docPkg := doc.New(pkg, packageDoc.ImportPath, doc.AllDecls)
			g.extractPackageDoc(packageDoc, docPkg)
		}

		return nil
	})
}

// getImportPath 获取导入路径
func (g *DocGenerator) getImportPath(path string) string {
	rel, err := filepath.Rel(g.rootDir, path)
	if err != nil {
		return path
	}
	return strings.ReplaceAll(rel, string(filepath.Separator), "/")
}

// extractPackageDoc 提取包文档
func (g *DocGenerator) extractPackageDoc(packageDoc *PackageDoc, docPkg *doc.Package) {
	packageDoc.Doc = docPkg.Doc
	//nolint:staticcheck // SA1019: doc.Synopsis 已废弃，但 Package.Synopsis 在 Go 1.20+ 才可用，保持兼容性
	packageDoc.Synopsis = doc.Synopsis(docPkg.Doc)

	// 提取接口
	for _, typ := range docPkg.Types {
		if g.isInterface(typ) {
			interfaceDoc := g.extractInterfaceDoc(typ)
			packageDoc.Interfaces = append(packageDoc.Interfaces, interfaceDoc)
		} else {
			typeDoc := g.extractTypeDoc(typ)
			packageDoc.Types = append(packageDoc.Types, typeDoc)
		}
	}

	// 提取函数
	for _, fn := range docPkg.Funcs {
		functionDoc := g.extractFunctionDoc(fn)
		packageDoc.Functions = append(packageDoc.Functions, functionDoc)
	}

	// 提取常量
	for _, val := range docPkg.Consts {
		for _, spec := range val.Decl.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for i, name := range valueSpec.Names {
					constantDoc := ConstantDoc{
						Name: name.Name,
						Doc:  val.Doc,
						Type: g.getTypeString(valueSpec.Type),
					}
					if i < len(valueSpec.Values) {
						constantDoc.Value = g.getValueString(valueSpec.Values[i])
					}
					packageDoc.Constants = append(packageDoc.Constants, constantDoc)
				}
			}
		}
	}

	// 提取变量
	for _, val := range docPkg.Vars {
		for _, spec := range val.Decl.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range valueSpec.Names {
					variableDoc := VariableDoc{
						Name: name.Name,
						Doc:  val.Doc,
						Type: g.getTypeString(valueSpec.Type),
					}
					packageDoc.Variables = append(packageDoc.Variables, variableDoc)
				}
			}
		}
	}
}

// isInterface 检查是否是接口类型
func (g *DocGenerator) isInterface(typ *doc.Type) bool {
	if typ.Decl == nil {
		return false
	}

	for _, spec := range typ.Decl.Specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				return true
			}
		}
	}
	return false
}

// extractInterfaceDoc 提取接口文档
func (g *DocGenerator) extractInterfaceDoc(typ *doc.Type) InterfaceDoc {
	interfaceDoc := InterfaceDoc{
		Name:          typ.Name,
		Doc:           typ.Doc,
		Methods:       make([]MethodDoc, 0),
		Examples:      make([]ExampleDoc, 0),
		BestPractices: make([]string, 0),
	}

	// 提取方法
	for _, method := range typ.Methods {
		methodDoc := g.extractMethodDoc(method)
		interfaceDoc.Methods = append(interfaceDoc.Methods, methodDoc)
	}

	// 提取使用指南和最佳实践
	interfaceDoc.UsageGuide = g.extractUsageGuide(typ.Doc)
	interfaceDoc.BestPractices = g.extractBestPractices(typ.Doc)

	return interfaceDoc
}

// extractTypeDoc 提取类型文档
func (g *DocGenerator) extractTypeDoc(typ *doc.Type) TypeDoc {
	typeDoc := TypeDoc{
		Name:     typ.Name,
		Doc:      typ.Doc,
		Type:     g.getTypeString(nil), // 简化实现
		Fields:   make([]FieldDoc, 0),
		Methods:  make([]MethodDoc, 0),
		Examples: make([]ExampleDoc, 0),
	}

	// 提取方法
	for _, method := range typ.Methods {
		methodDoc := g.extractMethodDoc(method)
		typeDoc.Methods = append(typeDoc.Methods, methodDoc)
	}

	return typeDoc
}

// extractFunctionDoc 提取函数文档
func (g *DocGenerator) extractFunctionDoc(fn *doc.Func) FunctionDoc {
	functionDoc := FunctionDoc{
		Name:       fn.Name,
		Doc:        fn.Doc,
		Signature:  g.getFunctionSignature(fn.Decl),
		Parameters: g.extractParameters(fn.Decl),
		Returns:    g.extractReturns(fn.Decl),
		Examples:   make([]ExampleDoc, 0),
	}

	return functionDoc
}

// extractMethodDoc 提取方法文档
func (g *DocGenerator) extractMethodDoc(fn *doc.Func) MethodDoc {
	methodDoc := MethodDoc{
		Name:       fn.Name,
		Doc:        fn.Doc,
		Signature:  g.getFunctionSignature(fn.Decl),
		Parameters: g.extractParameters(fn.Decl),
		Returns:    g.extractReturns(fn.Decl),
		Examples:   make([]ExampleDoc, 0),
		Notes:      make([]string, 0),
	}

	// 提取注意事项
	methodDoc.Notes = g.extractNotes(fn.Doc)

	return methodDoc
}

// parseDocumentation 解析文档
func (g *DocGenerator) parseDocumentation() error {
	for _, pkg := range g.packages {
		// 解析示例
		if g.config.GenerateExamples {
			g.generateExamples(pkg)
		}

		// 解析参数和返回值文档
		g.parseParameterDocs(pkg)
	}
	return nil
}

// validateExamples 验证示例代码
func (g *DocGenerator) validateExamples() error {
	for _, pkg := range g.packages {
		for _, iface := range pkg.Interfaces {
			for _, example := range iface.Examples {
				if err := g.validateExampleCode(example.Code); err != nil {
					fmt.Printf("⚠️ 示例验证失败 %s.%s: %v\n", pkg.Name, iface.Name, err)
				}
			}
		}
	}
	return nil
}

// validateExampleCode 验证示例代码语法
func (g *DocGenerator) validateExampleCode(code string) error {
	// 简化实现：检查 Go 语法
	_, err := parser.ParseExpr(code)
	return err
}

// calculateCoverage 计算文档覆盖率
func (g *DocGenerator) calculateCoverage() {
	for _, pkg := range g.packages {
		coverage := &pkg.Coverage

		// 计算接口覆盖率
		coverage.TotalInterfaces = len(pkg.Interfaces)
		for _, iface := range pkg.Interfaces {
			if iface.Doc != "" {
				coverage.InterfacesCovered++
			}
		}

		// 计算方法覆盖率
		for _, iface := range pkg.Interfaces {
			coverage.TotalMethods += len(iface.Methods)
			for _, method := range iface.Methods {
				if method.Doc != "" {
					coverage.MethodsCovered++
				}
			}
		}

		// 计算示例覆盖率
		for _, iface := range pkg.Interfaces {
			coverage.TotalExamples += len(iface.Methods)
			coverage.ExamplesCovered += len(iface.Examples)
		}

		// 计算总体覆盖率
		if coverage.TotalInterfaces > 0 {
			interfaceCoverage := float64(coverage.InterfacesCovered) / float64(coverage.TotalInterfaces)
			methodCoverage := float64(coverage.MethodsCovered) / float64(coverage.TotalMethods)
			exampleCoverage := float64(coverage.ExamplesCovered) / float64(coverage.TotalExamples)

			coverage.OverallCoverage = (interfaceCoverage + methodCoverage + exampleCoverage) / 3.0 * 100
		}
	}
}

// generateOutputFiles 生成输出文件
func (g *DocGenerator) generateOutputFiles() error {
	//nolint:gosec // G301: 文档输出目录需要用户可读权限，0755 是合理的
	if err := os.MkdirAll(g.config.OutputDir, 0755); err != nil {
		return err
	}

	for _, format := range g.config.OutputFormats {
		switch format {
		case "markdown":
			if err := g.generateMarkdownDocs(); err != nil {
				return err
			}
		case "html":
			if err := g.generateHTMLDocs(); err != nil {
				return err
			}
		case "json":
			if err := g.generateJSONDocs(); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateMarkdownDocs 生成 Markdown 文档
func (g *DocGenerator) generateMarkdownDocs() error {
	for _, pkg := range g.packages {
		filename := filepath.Join(g.config.OutputDir, pkg.Name+".md")
		content := g.generateMarkdownContent(pkg)

		//nolint:gosec // G306: 文档文件需要用户可读权限，0644 是合理的
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// generateMarkdownContent 生成 Markdown 内容
func (g *DocGenerator) generateMarkdownContent(pkg *PackageDoc) string {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s\n\n", pkg.Name))
	content.WriteString(fmt.Sprintf("**导入路径**: `%s`\n\n", pkg.ImportPath))

	if pkg.Synopsis != "" {
		content.WriteString(fmt.Sprintf("**概述**: %s\n\n", pkg.Synopsis))
	}

	if pkg.Doc != "" {
		content.WriteString(fmt.Sprintf("## 描述\n\n%s\n\n", pkg.Doc))
	}

	// 接口文档
	if len(pkg.Interfaces) > 0 {
		content.WriteString("## 接口\n\n")
		for _, iface := range pkg.Interfaces {
			content.WriteString(g.generateInterfaceMarkdown(iface))
		}
	}

	// 文档覆盖率
	content.WriteString("## 文档覆盖率\n\n")
	content.WriteString(fmt.Sprintf("- 接口覆盖率: %d/%d (%.1f%%)\n",
		pkg.Coverage.InterfacesCovered, pkg.Coverage.TotalInterfaces,
		float64(pkg.Coverage.InterfacesCovered)/float64(pkg.Coverage.TotalInterfaces)*100))
	content.WriteString(fmt.Sprintf("- 方法覆盖率: %d/%d (%.1f%%)\n",
		pkg.Coverage.MethodsCovered, pkg.Coverage.TotalMethods,
		float64(pkg.Coverage.MethodsCovered)/float64(pkg.Coverage.TotalMethods)*100))
	content.WriteString(fmt.Sprintf("- 总体覆盖率: %.1f%%\n\n", pkg.Coverage.OverallCoverage))

	content.WriteString(fmt.Sprintf("---\n*生成时间: %s*\n", time.Now().Format("2006-01-02 15:04:05")))

	return content.String()
}

// generateInterfaceMarkdown 生成接口 Markdown
func (g *DocGenerator) generateInterfaceMarkdown(iface InterfaceDoc) string {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("### %s\n\n", iface.Name))

	if iface.Doc != "" {
		content.WriteString(fmt.Sprintf("%s\n\n", iface.Doc))
	}

	// 方法列表
	if len(iface.Methods) > 0 {
		content.WriteString("#### 方法\n\n")
		for _, method := range iface.Methods {
			content.WriteString(fmt.Sprintf("##### %s\n\n", method.Name))
			content.WriteString(fmt.Sprintf("```go\n%s\n```\n\n", method.Signature))

			if method.Doc != "" {
				content.WriteString(fmt.Sprintf("%s\n\n", method.Doc))
			}

			// 参数
			if len(method.Parameters) > 0 {
				content.WriteString("**参数**:\n\n")
				for _, param := range method.Parameters {
					content.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", param.Name, param.Type, param.Description))
				}
				content.WriteString("\n")
			}

			// 返回值
			if len(method.Returns) > 0 {
				content.WriteString("**返回值**:\n\n")
				for _, ret := range method.Returns {
					content.WriteString(fmt.Sprintf("- `%s`: %s\n", ret.Type, ret.Description))
				}
				content.WriteString("\n")
			}
		}
	}

	return content.String()
}

// generateHTMLDocs 生成 HTML 文档
func (g *DocGenerator) generateHTMLDocs() error {
	// 简化实现
	return nil
}

// generateJSONDocs 生成 JSON 文档
func (g *DocGenerator) generateJSONDocs() error {
	// 简化实现
	return nil
}

// 辅助方法（简化实现）
func (g *DocGenerator) getTypeString(__expr ast.Expr) string              { return "interface{}" }
func (g *DocGenerator) getValueString(__expr ast.Expr) string             { return "" }
func (g *DocGenerator) getFunctionSignature(__decl *ast.FuncDecl) string  { return "" }
func (g *DocGenerator) extractParameters(__decl *ast.FuncDecl) []ParamDoc { return []ParamDoc{} }
func (g *DocGenerator) extractReturns(__decl *ast.FuncDecl) []ReturnDoc   { return []ReturnDoc{} }
func (g *DocGenerator) extractUsageGuide(__doc string) string             { return "" }
func (g *DocGenerator) extractBestPractices(__doc string) []string        { return []string{} }
func (g *DocGenerator) extractNotes(_doc string) []string                { return []string{} }
func (g *DocGenerator) generateExamples(_pkg *PackageDoc)                {}
func (g *DocGenerator) parseParameterDocs(_pkg *PackageDoc)              {}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: doc-generator <目录路径>")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	generator := NewDocGenerator(rootDir)

	if err := generator.GenerateDocumentation(); err != nil {
		fmt.Printf("❌ 文档生成失败: %v\n", err)
		os.Exit(1)
	}

	// 输出覆盖率报告
	fmt.Println("\n📊 文档覆盖率报告:")
	packages := make([]*PackageDoc, 0, len(generator.packages))
	for _, pkg := range generator.packages {
		packages = append(packages, pkg)
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Coverage.OverallCoverage > packages[j].Coverage.OverallCoverage
	})

	for _, pkg := range packages {
		fmt.Printf("  %s: %.1f%% (%d/%d 接口, %d/%d 方法)\n",
			pkg.Name, pkg.Coverage.OverallCoverage,
			pkg.Coverage.InterfacesCovered, pkg.Coverage.TotalInterfaces,
			pkg.Coverage.MethodsCovered, pkg.Coverage.TotalMethods)
	}
}
