package runtime

import (
	"context"
	"testing"
)

// TestBasicWASMExecution 测试基础WASM合约执行流程
//
// 这是最简单的WASM执行测试：
// 1. 编译WASM字节码
// 2. 创建实例
// 3. 调用导出函数
// 4. 验证返回值
//
// 不涉及任何宿主函数，纯WASM计算
func TestBasicWASMExecution(t *testing.T) {
	// 1. 创建Runtime
	runtime := NewWazeroRuntime(nil, nil, nil)
	if runtime == nil {
		t.Fatal("创建WazeroRuntime失败")
	}

	// 2. 简单的WASM字节码（加法函数）
	// 这是一个手写的最小WASM模块，导出add(i32, i32) -> i32函数
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d, // WASM魔数
		0x01, 0x00, 0x00, 0x00, // 版本

		// 类型段（type section）
		0x01,             // section id
		0x07,             // section size
		0x01,             // 1个类型
		0x60,             // func type
		0x02, 0x7f, 0x7f, // 2个i32参数
		0x01, 0x7f, // 1个i32返回值

		// 函数段（function section）
		0x03, // section id
		0x02, // section size
		0x01, // 1个函数
		0x00, // 函数类型索引0

		// 导出段（export section）
		0x07,                   // section id
		0x07,                   // section size
		0x01,                   // 1个导出
		0x03, 0x61, 0x64, 0x64, // "add"
		0x00, 0x00, // 函数导出，索引0

		// 代码段（code section）
		0x0a,       // section id
		0x09,       // section size
		0x01,       // 1个函数体
		0x07,       // 函数体大小
		0x00,       // 局部变量数
		0x20, 0x00, // local.get 0
		0x20, 0x01, // local.get 1
		0x6a, // i32.add
		0x0b, // end
	}

	// 3. 编译合约
	ctx := context.Background()
	compiled, err := runtime.CompileContract(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("编译WASM失败: %v", err)
	}

	// 4. 创建实例
	instance, err := runtime.CreateInstance(ctx, compiled)
	if err != nil {
		t.Fatalf("创建WASM实例失败: %v", err)
	}

	// 5. 调用导出函数：add(10, 20) = 30
	result, err := runtime.ExecuteFunction(ctx, instance, "add", []uint64{10, 20})
	if err != nil {
		t.Fatalf("执行WASM函数失败: %v", err)
	}

	// 6. 验证结果
	if len(result) != 1 {
		t.Fatalf("期望1个返回值，实际得到%d个", len(result))
	}

	if result[0] != 30 {
		t.Fatalf("期望结果30，实际得到%d", result[0])
	}

	// 7. 销毁实例
	err = runtime.DestroyInstance(ctx, instance)
	if err != nil {
		t.Fatalf("销毁WASM实例失败: %v", err)
	}

	t.Log("基础WASM执行测试通过！")
}

// TestInvalidWASM 测试无效WASM字节码处理
func TestInvalidWASM(t *testing.T) {
	runtime := NewWazeroRuntime(nil, nil, nil)

	// 无效的WASM字节码
	invalidWasm := []byte{0x00, 0x01, 0x02, 0x03}

	ctx := context.Background()
	_, err := runtime.CompileContract(ctx, invalidWasm)

	// 应该返回错误
	if err == nil {
		t.Fatal("期望编译无效WASM返回错误，但成功了")
	}

	t.Logf("正确处理无效WASM: %v", err)
}

// TestEmptyWASM 测试空字节码处理
func TestEmptyWASM(t *testing.T) {
	runtime := NewWazeroRuntime(nil, nil, nil)

	ctx := context.Background()
	_, err := runtime.CompileContract(ctx, []byte{})

	// 应该返回错误
	if err == nil {
		t.Fatal("期望编译空WASM返回错误，但成功了")
	}

	t.Logf("正确处理空WASM: %v", err)
}
