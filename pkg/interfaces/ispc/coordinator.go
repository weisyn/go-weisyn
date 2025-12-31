// Package ispc provides coordinator interfaces for ISPC operations.
package ispc

import (
	"context"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TensorInput 张量输入（支持多维张量和多种数据类型）
//
// 🎯 **设计目的**：
// - 支持多维张量输入（如 [1, 3, 224, 224]）
// - 提供形状信息，确保与模型要求匹配
// - 支持多种数据类型（float32, int64, uint8等）
//
// 📋 **字段说明**：
//   - Name: 输入名称（可选，按顺序匹配时可为空）
//   - Data: 展平的数据（float64数组，用于float32/float64类型）
//   - Int64Data: int64类型数据（用于int64类型，如BERT等文本模型）
//   - Int32Data: int32类型数据（用于int32类型）
//   - Int16Data: int16类型数据（用于int16类型）
//   - Uint8Data: uint8类型数据（用于uint8类型，如图像原始数据）
//   - Shape: 形状信息（如 [1, 3, 224, 224]）
//   - DataType: 数据类型（可选："float32", "float64", "int64", "int32", "int16", "uint8"）
//
// 📋 **使用说明**：
//   - 根据模型要求的数据类型，使用对应的Data字段
//   - 如果DataType为空，将从模型元数据中自动获取
//
// 📚 **官方类型支持参考** (github.com/yalue/onnxruntime_go@v1.22.0):
//   - tensor_type_constraints.go: IntData 接口定义包含 ~int32 | ~int16 | ~int64 等
//   - onnxruntime_go 完全支持: int8, uint8, int16, uint16, int32, uint32, int64, uint64, float32, float64
type TensorInput struct {
	Name      string    // 输入名称（可选）
	Data      []float64 // float32/float64类型数据（通过float64传递）
	Int64Data []int64   // int64类型数据（用于文本模型）
	Int32Data []int32   // int32类型数据（onnxruntime_go 完全支持）
	Int16Data []int16   // int16类型数据（onnxruntime_go 完全支持）
	Uint8Data []uint8   // uint8类型数据（用于图像原始数据）
	Shape     []int64   // 形状信息
	DataType  string    // 数据类型（可选："float32", "float64", "int64", "int32", "int16", "uint8"）
}

// ████████████████████████████████████████████████████████████████████████████
// █                                                                            █
// █                    ISPC 执行协调器公共接口                                   █
// █                                                                            █
// █   ISPC (Intrinsic Self-Proving Computing) - 本征自证计算                     █
// █   提供强类型的WASM/ONNX执行接口，自动生成零知识证明                              █
// █                                                                            █
// ████████████████████████████████████████████████████████████████████████████

// WASMExecutionResult WASM执行产物
//
// 🎯 **设计目的**: ISPC层执行WASM合约后返回强类型结果
// TX层获取此执行产物后,负责完整的交易生命周期编排
//
// 📋 **产物内容**:
//   - ReturnValues: WASM执行的原生返回值 ([]uint64)
//   - StateOutput: 完整的状态输出（包含ZKProof，直接使用protobuf定义）
//   - ExecutionContext: 执行上下文信息(用于调试和审计)
//
// 🏗️ **架构优势**:
//   - 零数据转换：直接使用pb.StateOutput，无需中间层
//   - 原子性保证：ZKProof与StateOutput一体，不会遗漏
//   - 类型安全：protobuf生成的类型，编译期保证一致性
type WASMExecutionResult struct {
	// WASM原生返回值
	ReturnValues []uint64

	// 完整的状态输出（包含ZKProof，可直接用于交易构建）
	StateOutput *pb.StateOutput

	// 交易草稿生成的未签名交易（由宿主函数构建，可能包含资产/资源输出）
	DraftTransaction *pb.Transaction

	// 业务返回数据（通过set_return_data设置）
	ReturnData []byte

	// 事件列表（通过emit_event发射）
	Events []*Event

	// 执行上下文信息 (辅助数据,不影响交易构建)
	ExecutionContext map[string]interface{}
}

// Event 事件结构
type Event struct {
	Type      string                 // 事件类型
	Timestamp int64                  // 事件时间戳
	Data      map[string]interface{} // 事件数据
}

// ONNXTensorOutput ONNX 张量输出（公共接口层富张量结构）
type ONNXTensorOutput struct {
	// 输出名称（来自模型元数据）
	Name string
	// 数据类型字符串（如 "float32", "float64", "int64", "float16"）
	DType string
	// 张量形状（来自模型元数据或推断）
	Shape []int64
	// 布局说明（可选，如 "NCHW"）
	Layout string
	// 展平后的数值视图（便于可视化和简单消费）
	Values []float64
	// 原始字节视图（按底层元素类型编码，当前阶段主要用于 float32/float64）
	RawData []byte
}

// ONNXExecutionResult ONNX执行产物
//
// 🎯 **设计目的**: ISPC层执行ONNX推理后返回强类型结果
// TX层获取此执行产物后,负责完整的交易生命周期编排
//
// 📋 **产物内容**:
//   - ReturnTensors: 兼容字段，按 Values 派生的 [][]float64 视图
//   - TensorOutputs: 富张量结构列表（包含 dtype/shape/rawData 等）
//   - StateOutput: 完整的状态输出（包含ZKProof，直接使用protobuf定义）
//   - ExecutionContext: 执行上下文信息(用于调试和审计)
//
// 🏗️ **架构优势**:
//   - 零数据转换：直接使用pb.StateOutput，无需中间层
//   - 原子性保证：ZKProof与StateOutput一体，不会遗漏
//   - 类型安全：protobuf生成的类型，编译期保证一致性
type ONNXExecutionResult struct {
	// 兼容字段：按 TensorOutputs.Values 派生的视图
	ReturnTensors [][]float64

	// 富张量结构列表（完整表达 dtype/shape/rawData）
	TensorOutputs []ONNXTensorOutput

	// 完整的状态输出（包含ZKProof，可直接用于交易构建）
	StateOutput *pb.StateOutput

	// 执行上下文信息 (辅助数据,不影响交易构建)
	ExecutionContext map[string]interface{}
}

// ISPCCoordinator ISPC 执行协调器公共接口
//
// 🎯 **ISPC（Intrinsic Self-Proving Computing）职责**：
//   - 提供WASM智能合约执行能力（强类型）
//   - 提供ONNX模型推理能力（强类型）
//   - 自动生成零知识证明（必须非nil）
//   - 直接构建完整的pb.StateOutput（包含ZKProof）
//   - 不依赖TX层，仅返回执行产物
//
// 🏗️ **设计原则**：
//   - WASM与ONNX分离，使用强类型参数和返回值
//   - ZKProof必须非nil，生成失败直接报错
//   - 直接返回pb.StateOutput，TX层零转换使用
//   - 执行产物包含原生引擎返回值、完整StateOutput
//
// 🔄 **调用流程**：
//  1. TX层调用 ExecuteWASMContract/ExecuteONNXModel
//  2. ISPC层执行并返回 WASMExecutionResult/ONNXExecutionResult
//  3. TX层直接使用 StateOutput 构建交易（零转换）
//
// 📚 **详细规范**：
//   - _docs/specs/ispc/INTRINSIC_SELF_PROVING_COMPUTING_SPECIFICATION.md
//   - pb/blockchain/block/transaction/transaction.proto (StateOutput定义)
type ISPCCoordinator interface {
	// ExecuteWASMContract 执行WASM智能合约 (强类型)
	//
	// 🎯 **核心职责**:
	//   - 调度WASM引擎执行合约
	//   - 生成零知识证明 (必须成功，否则报错)
	//   - 直接构建完整的pb.StateOutput（包含ZKProof）
	//   - 返回WASMExecutionResult (不涉及交易构建/签名/提交)
	//
	// 📋 **参数**:
	//   - ctx: 上下文
	//   - contractHash: 合约内容哈希 (用于定位合约资源)
	//   - methodName: 要调用的方法名
	//   - params: 方法参数 (WASM原生类型 []uint64)
	//   - initParams: 合约调用参数（JSON/二进制负载）
	//   - callerAddress: 调用者地址（Base58Check格式）
	//
	// 🔧 **返回值**:
	//   - *WASMExecutionResult: 执行产物
	//     - ReturnValues: WASM原生返回值 []uint64
	//     - StateOutput: 完整的pb.StateOutput（包含ZKProof）
	//     - ExecutionContext: 执行上下文（调试用）
	//   - error: 执行失败或ZK证明生成失败时的错误
	//
	// 🌐 **单向依赖**: ISPC → 无
	ExecuteWASMContract(ctx context.Context, contractHash []byte, methodName string, params []uint64, initParams []byte, callerAddress string) (*WASMExecutionResult, error)

	// ExecuteONNXModel 执行ONNX模型推理 (强类型)
	//
	// 🎯 **核心职责**:
	//   - 调度ONNX引擎执行推理
	//   - 生成零知识证明 (必须成功，否则报错)
	//   - 直接构建完整的pb.StateOutput（包含ZKProof）
	//   - 返回ONNXExecutionResult (不涉及交易构建/签名/提交)
	//
	// 📋 **参数**:
	//   - ctx: 上下文
	//   - modelHash: AI模型内容哈希 (用于定位模型资源)
	//   - tensorInputs: 张量输入列表 (包含数据和形状信息)
	//
	// 🔧 **返回值**:
	//   - *ONNXExecutionResult: 执行产物
	//     - ReturnTensors: ONNX原生返回值 [][]float64
	//     - StateOutput: 完整的pb.StateOutput（包含ZKProof）
	//     - ExecutionContext: 执行上下文（调试用）
	//   - error: 执行失败或ZK证明生成失败时的错误
	ExecuteONNXModel(ctx context.Context, modelHash []byte, tensorInputs []TensorInput) (*ONNXExecutionResult, error)
}
