package main

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// PerformanceAntiPatternRule 性能反模式检查规则
type PerformanceAntiPatternRule struct {
	config *Config
}

func (r *PerformanceAntiPatternRule) Name() string {
	return "PerformanceAntiPattern"
}

func (r *PerformanceAntiPatternRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if r.config.IsWhitelisted(file) {
		return violations
	}

	// 检查性能反模式
	ast.Inspect(node, func(n ast.Node) bool {
		// 1. 检查循环中的字符串连接
		if forStmt, ok := n.(*ast.ForStmt); ok {
			violations = append(violations, r.checkStringConcatInLoop(guardian, file, forStmt)...)
		}

		// 2. 检查频繁的类型断言
		if typeAssert, ok := n.(*ast.TypeAssertExpr); ok {
			violations = append(violations, r.checkFrequentTypeAssertion(guardian, file, typeAssert)...)
		}

		// 3. 检查不必要的内存分配
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkUnnecessaryAllocation(guardian, file, callExpr)...)
		}

		return true
	})

	return violations
}

func (r *PerformanceAntiPatternRule) checkStringConcatInLoop(guardian *ArchGuardian, file string, forStmt *ast.ForStmt) []Violation {
	violations := make([]Violation, 0)

	// 检查循环体中是否有字符串连接
	ast.Inspect(forStmt.Body, func(n ast.Node) bool {
		if binExpr, ok := n.(*ast.BinaryExpr); ok {
			if binExpr.Op == token.ADD {
				// 检查是否是字符串类型的加法
				violations = append(violations, Violation{
					Type:        "PerformanceAntiPattern",
					File:        file,
					Line:        guardian.fileSet.Position(binExpr.Pos()).Line,
					Description: "循环中的字符串连接可能导致性能问题，建议使用 strings.Builder",
					Severity:    "WARNING",
				})
			}
		}
		return true
	})

	return violations
}

func (r *PerformanceAntiPatternRule) checkFrequentTypeAssertion(guardian *ArchGuardian, file string, typeAssert *ast.TypeAssertExpr) []Violation {
	violations := make([]Violation, 0)

	// 这里简化实现，实际应该分析上下文
	violations = append(violations, Violation{
		Type:        "PerformanceAntiPattern",
		File:        file,
		Line:        guardian.fileSet.Position(typeAssert.Pos()).Line,
		Description: "频繁的类型断言可能影响性能，考虑使用接口或泛型",
		Severity:    "INFO",
	})

	return violations
}

func (r *PerformanceAntiPatternRule) checkUnnecessaryAllocation(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查是否是 make 调用
	if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "make" {
		// 检查是否在循环中分配
		// 这里简化实现
		violations = append(violations, Violation{
			Type:        "PerformanceAntiPattern",
			File:        file,
			Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
			Description: "考虑是否可以复用内存分配以提高性能",
			Severity:    "INFO",
		})
	}

	return violations
}

// SecurityVulnerabilityRule 安全漏洞检查规则
type SecurityVulnerabilityRule struct {
	config *Config
}

func (r *SecurityVulnerabilityRule) Name() string {
	return "SecurityVulnerability"
}

func (r *SecurityVulnerabilityRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if r.config.IsWhitelisted(file) {
		return violations
	}

	// 检查安全漏洞
	ast.Inspect(node, func(n ast.Node) bool {
		// 1. 检查硬编码的密钥或密码
		if basicLit, ok := n.(*ast.BasicLit); ok {
			violations = append(violations, r.checkHardcodedSecrets(guardian, file, basicLit)...)
		}

		// 2. 检查不安全的随机数生成
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkInsecureRandom(guardian, file, callExpr)...)
		}

		// 3. 检查SQL注入风险
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkSQLInjection(guardian, file, callExpr)...)
		}

		return true
	})

	return violations
}

func (r *SecurityVulnerabilityRule) checkHardcodedSecrets(guardian *ArchGuardian, file string, lit *ast.BasicLit) []Violation {
	violations := make([]Violation, 0)

	if lit.Kind != token.STRING {
		return violations
	}

	value := strings.Trim(lit.Value, "\"'")

	// 检查可疑的密钥模式
	suspiciousPatterns := []string{
		"password", "passwd", "pwd",
		"secret", "key", "token",
		"api_key", "apikey",
		"private_key", "privatekey",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(strings.ToLower(value), pattern) && len(value) > 10 {
			violations = append(violations, Violation{
				Type:        "SecurityVulnerability",
				File:        file,
				Line:        guardian.fileSet.Position(lit.Pos()).Line,
				Description: "发现可能的硬编码密钥或密码，应使用环境变量或配置文件",
				Severity:    "ERROR",
			})
			break
		}
	}

	return violations
}

func (r *SecurityVulnerabilityRule) checkInsecureRandom(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查是否使用了不安全的随机数生成器
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "rand" && selExpr.Sel.Name == "Intn" {
				violations = append(violations, Violation{
					Type:        "SecurityVulnerability",
					File:        file,
					Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
					Description: "使用了不安全的随机数生成器，对于安全相关的用途应使用 crypto/rand",
					Severity:    "WARNING",
				})
			}
		}
	}

	return violations
}

func (r *SecurityVulnerabilityRule) checkSQLInjection(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查字符串拼接构建SQL查询
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if selExpr.Sel.Name == "Query" || selExpr.Sel.Name == "Exec" {
			// 检查参数是否包含字符串拼接
			for _, arg := range callExpr.Args {
				if binExpr, ok := arg.(*ast.BinaryExpr); ok {
					if binExpr.Op == token.ADD {
						violations = append(violations, Violation{
							Type:        "SecurityVulnerability",
							File:        file,
							Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
							Description: "字符串拼接构建SQL查询可能导致SQL注入，应使用参数化查询",
							Severity:    "ERROR",
						})
					}
				}
			}
		}
	}

	return violations
}

// ConcurrencyIssueRule 并发问题检查规则
type ConcurrencyIssueRule struct {
	config *Config
}

func (r *ConcurrencyIssueRule) Name() string {
	return "ConcurrencyIssue"
}

func (r *ConcurrencyIssueRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if r.config.IsWhitelisted(file) {
		return violations
	}

	// 检查并发问题
	ast.Inspect(node, func(n ast.Node) bool {
		// 1. 检查未保护的共享变量访问
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			violations = append(violations, r.checkUnprotectedSharedAccess(guardian, file, assignStmt)...)
		}

		// 2. 检查goroutine泄漏
		if goStmt, ok := n.(*ast.GoStmt); ok {
			violations = append(violations, r.checkGoroutineLeak(guardian, file, goStmt)...)
		}

		// 3. 检查channel使用问题
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkChannelIssues(guardian, file, callExpr)...)
		}

		return true
	})

	return violations
}

func (r *ConcurrencyIssueRule) checkUnprotectedSharedAccess(guardian *ArchGuardian, file string, assignStmt *ast.AssignStmt) []Violation {
	violations := make([]Violation, 0)

	// 简化实现：检查是否有全局变量赋值
	for _, lhs := range assignStmt.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			// 检查是否是大写开头的标识符（可能是包级变量）
			if len(ident.Name) > 0 && strings.ToUpper(ident.Name[:1]) == ident.Name[:1] {
				violations = append(violations, Violation{
					Type:        "ConcurrencyIssue",
					File:        file,
					Line:        guardian.fileSet.Position(assignStmt.Pos()).Line,
					Description: "可能的并发访问共享变量，考虑使用互斥锁或原子操作",
					Severity:    "WARNING",
				})
			}
		}
	}

	return violations
}

func (r *ConcurrencyIssueRule) checkGoroutineLeak(guardian *ArchGuardian, file string, goStmt *ast.GoStmt) []Violation {
	violations := make([]Violation, 0)

	// 检查goroutine是否有适当的生命周期管理
	violations = append(violations, Violation{
		Type:        "ConcurrencyIssue",
		File:        file,
		Line:        guardian.fileSet.Position(goStmt.Pos()).Line,
		Description: "确保goroutine有适当的生命周期管理，避免goroutine泄漏",
		Severity:    "INFO",
	})

	return violations
}

func (r *ConcurrencyIssueRule) checkChannelIssues(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查是否是make(chan)调用
	if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "make" {
		if len(callExpr.Args) > 0 {
			if chanType, ok := callExpr.Args[0].(*ast.ChanType); ok {
				// 检查是否是无缓冲channel
				if len(callExpr.Args) == 1 {
					violations = append(violations, Violation{
						Type:        "ConcurrencyIssue",
						File:        file,
						Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
						Description: "无缓冲channel可能导致死锁，确保有适当的goroutine处理",
						Severity:    "INFO",
					})
				}
				_ = chanType // 避免未使用变量警告
			}
		}
	}

	return violations
}

// DesignPatternViolationRule 设计模式违规检查规则
type DesignPatternViolationRule struct {
	config *Config
}

func (r *DesignPatternViolationRule) Name() string {
	return "DesignPatternViolation"
}

func (r *DesignPatternViolationRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if r.config.IsWhitelisted(file) {
		return violations
	}

	// 检查设计模式违规
	ast.Inspect(node, func(n ast.Node) bool {
		// 1. 检查单例模式实现问题
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			violations = append(violations, r.checkSingletonPattern(guardian, file, funcDecl)...)
		}

		// 2. 检查工厂模式问题
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			violations = append(violations, r.checkFactoryPattern(guardian, file, funcDecl)...)
		}

		// 3. 检查观察者模式问题
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			violations = append(violations, r.checkObserverPattern(guardian, file, typeSpec)...)
		}

		return true
	})

	return violations
}

func (r *DesignPatternViolationRule) checkSingletonPattern(guardian *ArchGuardian, file string, funcDecl *ast.FuncDecl) []Violation {
	violations := make([]Violation, 0)

	// 检查是否是GetInstance类型的函数
	if strings.Contains(strings.ToLower(funcDecl.Name.Name), "instance") {
		// 检查是否有适当的同步机制
		hasSync := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
					if selExpr.Sel.Name == "Do" || selExpr.Sel.Name == "Lock" {
						hasSync = true
					}
				}
			}
			return true
		})

		if !hasSync {
			violations = append(violations, Violation{
				Type:        "DesignPatternViolation",
				File:        file,
				Line:        guardian.fileSet.Position(funcDecl.Pos()).Line,
				Description: "单例模式实现缺少同步机制，在并发环境下可能不安全",
				Severity:    "WARNING",
			})
		}
	}

	return violations
}

func (r *DesignPatternViolationRule) checkFactoryPattern(guardian *ArchGuardian, file string, funcDecl *ast.FuncDecl) []Violation {
	violations := make([]Violation, 0)

	// 检查是否是New开头的函数
	if strings.HasPrefix(funcDecl.Name.Name, "New") {
		// 检查返回值是否是接口类型
		if funcDecl.Type.Results != nil {
			for _, result := range funcDecl.Type.Results.List {
				if _, ok := result.Type.(*ast.InterfaceType); ok {
					// 这是好的实践，返回接口
					continue
				}
				// 检查是否返回具体类型
				if ident, ok := result.Type.(*ast.Ident); ok {
					if strings.HasPrefix(ident.Name, "*") {
						violations = append(violations, Violation{
							Type:        "DesignPatternViolation",
							File:        file,
							Line:        guardian.fileSet.Position(funcDecl.Pos()).Line,
							Description: "工厂函数建议返回接口类型而不是具体类型，以提高可测试性",
							Severity:    "INFO",
						})
					}
				}
			}
		}
	}

	return violations
}

func (r *DesignPatternViolationRule) checkObserverPattern(guardian *ArchGuardian, file string, typeSpec *ast.TypeSpec) []Violation {
	violations := make([]Violation, 0)

	// 检查是否是观察者模式相关的结构体
	if structType, ok := typeSpec.Type.(*ast.StructType); ok {
		hasObservers := false
		for _, field := range structType.Fields.List {
			if field.Names != nil {
				for _, name := range field.Names {
					if strings.Contains(strings.ToLower(name.Name), "observer") ||
						strings.Contains(strings.ToLower(name.Name), "listener") {
						hasObservers = true
						break
					}
				}
			}
		}

		if hasObservers {
			// 检查是否有适当的并发保护
			violations = append(violations, Violation{
				Type:        "DesignPatternViolation",
				File:        file,
				Line:        guardian.fileSet.Position(typeSpec.Pos()).Line,
				Description: "观察者模式实现需要考虑并发安全性",
				Severity:    "INFO",
			})
		}
	}

	return violations
}

// TestabilityIssueRule 可测试性问题检查规则
type TestabilityIssueRule struct {
	config *Config
}

func (r *TestabilityIssueRule) Name() string {
	return "TestabilityIssue"
}

func (r *TestabilityIssueRule) Check(guardian *ArchGuardian, file string, node ast.Node) []Violation {
	violations := make([]Violation, 0)

	if r.config.IsWhitelisted(file) || strings.Contains(file, "_test.go") {
		return violations
	}

	// 检查可测试性问题
	ast.Inspect(node, func(n ast.Node) bool {
		// 1. 检查硬编码依赖
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkHardcodedDependencies(guardian, file, callExpr)...)
		}

		// 2. 检查全局状态依赖
		if ident, ok := n.(*ast.Ident); ok {
			violations = append(violations, r.checkGlobalStateDependency(guardian, file, ident)...)
		}

		// 3. 检查时间依赖
		if callExpr, ok := n.(*ast.CallExpr); ok {
			violations = append(violations, r.checkTimeDependency(guardian, file, callExpr)...)
		}

		return true
	})

	return violations
}

func (r *TestabilityIssueRule) checkHardcodedDependencies(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查是否直接创建依赖对象
	if ident, ok := callExpr.Fun.(*ast.Ident); ok {
		if strings.HasPrefix(ident.Name, "New") {
			violations = append(violations, Violation{
				Type:        "TestabilityIssue",
				File:        file,
				Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
				Description: "直接创建依赖对象可能影响可测试性，考虑使用依赖注入",
				Severity:    "INFO",
			})
		}
	}

	return violations
}

func (r *TestabilityIssueRule) checkGlobalStateDependency(guardian *ArchGuardian, file string, ident *ast.Ident) []Violation {
	violations := make([]Violation, 0)

	// 检查是否访问全局变量
	globalPatterns := []string{
		"^[A-Z][a-zA-Z0-9]*$", // 大写开头的标识符
	}

	for _, pattern := range globalPatterns {
		if matched, _ := regexp.MatchString(pattern, ident.Name); matched {
			violations = append(violations, Violation{
				Type:        "TestabilityIssue",
				File:        file,
				Line:        guardian.fileSet.Position(ident.Pos()).Line,
				Description: "依赖全局状态可能影响测试的独立性",
				Severity:    "INFO",
			})
			break
		}
	}

	return violations
}

func (r *TestabilityIssueRule) checkTimeDependency(guardian *ArchGuardian, file string, callExpr *ast.CallExpr) []Violation {
	violations := make([]Violation, 0)

	// 检查是否直接使用时间函数
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "time" && (selExpr.Sel.Name == "Now" || selExpr.Sel.Name == "Sleep") {
				violations = append(violations, Violation{
					Type:        "TestabilityIssue",
					File:        file,
					Line:        guardian.fileSet.Position(callExpr.Pos()).Line,
					Description: "直接使用时间函数可能影响测试的确定性，考虑注入时间接口",
					Severity:    "INFO",
				})
			}
		}
	}

	return violations
}
