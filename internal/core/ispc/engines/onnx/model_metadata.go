//go:build !android && !ios && cgo
// +build !android,!ios,cgo

package onnx

import (
	"fmt"
	"os"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// contains 检查字符串是否包含子字符串（辅助函数）
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ModelMetadata ONNX模型元数据
type ModelMetadata struct {
	InputNames  []string // 输入张量名称
	OutputNames []string // 输出张量名称
	InputInfos  []ort.InputOutputInfo  // 输入信息（形状、类型等）
	OutputInfos []ort.InputOutputInfo  // 输出信息（形状、类型等）
}

// extractModelMetadata 从ONNX模型字节数据提取元数据
//
// 使用 GetInputOutputInfoWithONNXData API 直接获取模型的输入/输出信息
// 注意：此函数要求 ONNX Runtime 环境已经初始化（调用 InitializeEnvironment）
// 
// ⚠️ 重要：此函数不负责初始化 ONNX Runtime，初始化应该由 initializeONNXRuntime() 完成。
// 如果 Runtime 未初始化，说明 initializeONNXRuntime() 调用失败，应该返回错误。
func extractModelMetadata(modelBytes []byte) (*ModelMetadata, error) {
	// 直接写入文件以确保日志被捕获
	traceFile := "/tmp/onnx_trace.log"
	f, _ := os.OpenFile(traceFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, "[TRACE extractModelMetadata] 开始提取模型元数据\n")
		f.Close()
	}
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] 开始提取模型元数据\n")
	
	// 检查 ONNX Runtime 是否已初始化
	// 注意：不在这里初始化，初始化应该由 initializeONNXRuntime() 完成
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] 检查 IsInitialized()...\n")
	isInit := ort.IsInitialized()
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] IsInitialized() = %v\n", isInit)
	if !isInit {
		err := fmt.Errorf("ONNX Runtime环境未初始化，请确保 initializeONNXRuntime() 已成功调用")
		errMsg := err.Error()
		// 直接写入文件以确保日志被捕获
		f, _ := os.OpenFile("/tmp/onnx_trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ IsInitialized() = false，返回错误\n")
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息: %q\n", errMsg)
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息长度: %d\n", len(errMsg))
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且初始化失败': %v\n", contains(errMsg, "且初始化失败"))
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且': %v\n", contains(errMsg, "且"))
			f.Close()
		}
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ IsInitialized() = false，返回错误\n")
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息: %q\n", errMsg)
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息长度: %d\n", len(errMsg))
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且初始化失败': %v\n", contains(errMsg, "且初始化失败"))
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且': %v\n", contains(errMsg, "且"))
		return nil, err
	}
	
	// 获取模型的输入/输出信息
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] 调用 GetInputOutputInfoWithONNXData()...\n")
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] 模型数据大小: %d 字节\n", len(modelBytes))
	inputInfos, outputInfos, err := ort.GetInputOutputInfoWithONNXData(modelBytes)
	if err != nil {
		errMsg := err.Error()
		// 直接写入文件以确保日志被捕获
		f, _ := os.OpenFile("/tmp/onnx_trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ GetInputOutputInfoWithONNXData() 失败\n")
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息完整内容: %q\n", errMsg)
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息长度: %d 字符\n", len(errMsg))
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误类型: %T\n", err)
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且初始化失败': %v\n", 
				contains(errMsg, "且初始化失败"))
			fmt.Fprintf(f, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且': %v\n", 
				contains(errMsg, "且"))
			f.Close()
		}
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ GetInputOutputInfoWithONNXData() 失败\n")
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息完整内容: %q\n", errMsg)
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息长度: %d 字符\n", len(errMsg))
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误类型: %T\n", err)
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且初始化失败': %v\n", 
			contains(errMsg, "且初始化失败"))
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 错误信息是否包含'且': %v\n", 
			contains(errMsg, "且"))
		
		// 如果 ONNX Runtime 在调用后变为未初始化状态，说明初始化失败
		isInitAfter := ort.IsInitialized()
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] IsInitialized() 调用后 = %v\n", isInitAfter)
		if !isInitAfter {
			// ONNX Runtime 在调用后变为未初始化状态，说明初始化失败
			err2 := fmt.Errorf("ONNX Runtime环境未初始化：GetInputOutputInfoWithONNXData 调用失败，可能因为初始化失败: %w", err)
			fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 返回错误（初始化失败）: %q\n", err2.Error())
			return nil, err2
		}
		err3 := fmt.Errorf("获取模型输入/输出信息失败: %w", err)
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ❌ 返回错误（其他原因）: %q\n", err3.Error())
		return nil, err3
	}
	fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] ✅ GetInputOutputInfoWithONNXData() 成功\n")

	// 提取输入/输出名称
	inputNames := make([]string, len(inputInfos))
	for i, info := range inputInfos {
		inputNames[i] = info.Name
	}

	outputNames := make([]string, len(outputInfos))
	for i, info := range outputInfos {
		outputNames[i] = info.Name
		// 添加调试日志：输出每个输出的类型信息
		fmt.Fprintf(os.Stderr, "[TRACE extractModelMetadata] 输出[%d]: name=%s, OrtValueType=%v (值=%d), DataType=%v\n",
			i, info.Name, info.OrtValueType, int(info.OrtValueType), info.DataType)
	}

	return &ModelMetadata{
		InputNames:  inputNames,
		OutputNames: outputNames,
		InputInfos:  inputInfos,
		OutputInfos: outputInfos,
	}, nil
}

