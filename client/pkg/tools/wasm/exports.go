// Package wasm 提供 WASM 工具函数
package wasm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
)

// ExtractExports 从WASM文件中提取导出函数列表
//
// 功能：
//   - 使用wazero库解析WASM二进制文件
//   - 提取所有导出的函数名称
//   - 自动过滤非函数导出（如memory、global等）
//
// 使用场景：
//   - 智能合约部署时自动提取导出函数
//   - 合约调用时验证函数是否存在
//   - 工具命令显示合约接口信息
//
// 参数：
//   - wasmPath: WASM文件的完整路径
//
// 返回：
//   - []string: 导出的函数名称列表
//   - error: 文件读取或解析错误
//
// 示例：
//
//	exports, err := ExtractExports("./hello_world.wasm")
//	// exports: ["SayHello", "GetGreeting", "SetMessage", ...]
func ExtractExports(wasmPath string) ([]string, error) {
	// 读取WASM文件
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("读取WASM文件失败: %w", err)
	}

	// 创建wazero运行时
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// 编译WASM模块（不实例化，只解析）
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("解析WASM模块失败: %w", err)
	}
	defer compiled.Close(ctx)

	// 提取导出的函数名称
	// 定义需要过滤的内部函数（TinyGo/WASI标准函数）
	internalFunctions := map[string]bool{
		"malloc":      true,
		"calloc":      true,
		"realloc":     true,
		"free":        true,
		"_start":      true,
		"_initialize": true,
	}

	var exports []string
	for _, export := range compiled.ExportedFunctions() {
		funcName := export.Name()
		// 过滤掉内部函数和以_开头的私有函数
		if funcName != "" && !internalFunctions[funcName] && !strings.HasPrefix(funcName, "_") {
			exports = append(exports, funcName)
		}
	}

	if len(exports) == 0 {
		return nil, fmt.Errorf("未找到业务导出函数（WASM文件可能未使用//export标记导出函数）")
	}

	return exports, nil
}
