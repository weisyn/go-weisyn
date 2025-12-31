package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
)

func main() {
	// 读取WASM文件
	wasmBytes, err := os.ReadFile("build/hello_world.wasm")
	if err != nil {
		panic(err)
	}

	// 创建运行时
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// 编译模块
	compiled, err := r.CompileModule(ctx, wasmBytes)
	if err != nil {
		panic(err)
	}
	defer compiled.Close(ctx)

	// 打印导入需求
	fmt.Println("=== WASM Module Import Requirements ===")
	for _, importDesc := range compiled.ImportedFunctions() {
		fmt.Printf("Module: %s, Name: %s, ParamTypes: %v, ResultTypes: %v\n",
			importDesc.ModuleName(),
			importDesc.Name(),
			importDesc.ParamTypes(),
			importDesc.ResultTypes())
	}

	fmt.Println("\n=== WASM Module Export Functions ===")
	for name := range compiled.ExportedFunctions() {
		fmt.Printf("Export: %s\n", name)
	}
}
