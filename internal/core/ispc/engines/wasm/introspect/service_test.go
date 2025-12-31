package introspect

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractExportedFunctionsFromFile 确认可从合约示例提取导出函数
func TestExtractExportedFunctionsFromFile(t *testing.T) {
	// 使用合约示例进行测试（替代已废弃的 system 合约）
	repoRoot := "."
	// 优先使用 hello-world，如果不存在则尝试 simple-token
	wasmPath := filepath.Join(repoRoot, "contracts", "examples", "basic", "hello-world", "hello-world.wasm")
	if _, err := os.Stat(wasmPath); err != nil {
		// 回退到 simple-token
		wasmPath = filepath.Join(repoRoot, "contracts", "examples", "basic", "simple-token", "simple-token.wasm")
		if _, err := os.Stat(wasmPath); err != nil {
			t.Skipf("skip: wasm file not found, please build contracts/examples first")
			return
		}
	}

	svc := NewIntrospectionService()
	functions, err := svc.ExtractExportedFunctionsFromFile(wasmPath)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if len(functions) == 0 {
		t.Fatalf("no exported functions found")
	}

	// 期望至少包含一些导出函数（具体依赖构建产物）
	// hello-world 应该有 SayHello, GetGreetingCount, GetDeployerInfo 等
	// simple-token 应该有 Transfer, GetBalance, GetTotalSupply 等
	t.Logf("found exported functions: %v", functions)
}

