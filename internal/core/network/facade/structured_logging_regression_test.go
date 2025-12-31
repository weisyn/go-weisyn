package facade

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// 防回归：禁止把 Infof/Warnf/Debugf 当结构化日志用（会触发 %!(EXTRA ...)）。
func TestFacadeService_NoStructuredMisuseOfPrintfLoggers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	servicePath := filepath.Join(dir, "service.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, servicePath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", servicePath, err)
	}

	// 精准检测：logger.Infof("msg", "k", v) 这种“第二个参数还是字符串字面量”的用法
	// （避免用正则误伤注释/字符串内容）
	bad := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Infof", "Warnf", "Debugf", "Errorf":
		default:
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		// 只关注第一个参数是 format string 的情况
		if _, ok := call.Args[0].(*ast.BasicLit); !ok {
			return true
		}
		// 第二个参数是字符串字面量 => 极大概率是在模拟结构化 key/value（错误用法）
		if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			pos := fset.Position(lit.Pos())
			t.Errorf("printf-style structured logging misuse at %s: use With(...).Info/Warn/Debug instead", pos)
			bad = true
		}
		return true
	})
	if bad {
		t.Fatalf("found printf-style structured logging misuse in %s", servicePath)
	}
}


