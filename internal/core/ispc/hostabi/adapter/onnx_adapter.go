package adapter

import (
	"context"

	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ONNXAdapter ONNX宿主函数适配器
//
// 🎯 **设计目的**：从HostABI构建ONNX引擎兼容的宿主函数映射
// 📋 **职责**：为ONNX模型提供最小的只读宿主函数集合
//
// 🏗️ **架构位置**：
// - 作为hostabi/adapter的一部分
// - 为ONNX引擎提供专用的宿主函数集合
//
// 🔧 **设计原则**：
// - 只提供只读查询函数（5个最小原语）
// - 不提供任何写操作
// - 使用Go原生类型，便于ONNX引擎集成
type ONNXAdapter struct{}

// NewONNXAdapter 创建ONNX适配器
func NewONNXAdapter() *ONNXAdapter {
	return &ONNXAdapter{}
}

// BuildHostFunctions 构建ONNX宿主函数映射
//
// 📋 **参数**：
//   - ctx: 调用上下文（包含ExecutionContext）
//   - hostABI: HostABI实例
//
// 🔧 **返回值**：
//   - map[string]interface{}: ONNX宿主函数映射（5个最小只读原语）
//
// 🎯 **设计说明**：
// ONNX模型推理主要用于链上AI计算，提供最小的只读查询能力：
//  1. 确定性区块视图 - 用于时间相关的模型输入
//  2. UTXO存在性查询 - 用于验证模型输入的资产存在性
//  3. 资源存在性查询 - 用于加载模型依赖的其他资源
//
// ⚠️ **约束**：
//   - 只提供只读操作，不提供写操作
//   - 不提供交易草稿操作（ONNX不构建交易）
//   - 参数和返回值使用Go原生类型，便于ONNX引擎集成
func (a *ONNXAdapter) BuildHostFunctions(
	ctx context.Context,
	hostABI publicispc.HostABI,
) map[string]interface{} {
	// 🎯 **5个最小只读原语的ONNX适配**
	//
	// 注意：ONNX引擎调用约定与WASM不同，这里使用Go原生类型

	return map[string]interface{}{
		// ═══════════════════════════════════════════════
		// 类别 A：确定性区块视图（只读）
		// ═══════════════════════════════════════════════

		"get_block_height": func() int64 {
			height, err := hostABI.GetBlockHeight(ctx)
			if err != nil {
				return 0
			}
			return int64(height)
		},

		"get_block_timestamp": func() int64 {
			timestamp, err := hostABI.GetBlockTimestamp(ctx)
			if err != nil {
				return 0
			}
			return int64(timestamp)
		},

		"get_chain_id": func() []byte {
			chainID, err := hostABI.GetChainID(ctx)
			if err != nil {
				return nil
			}
			return chainID
		},

		// ═══════════════════════════════════════════════
		// 类别 B：存在性查询（只读）
		// ═══════════════════════════════════════════════

		"utxo_exists": func(txHash []byte, index uint32) bool {
			if len(txHash) != 32 {
				return false
			}
			outpoint := &pb.OutPoint{
				TxId:        txHash,
				OutputIndex: index,
			}
			exists, err := hostABI.UTXOExists(ctx, outpoint)
			if err != nil {
				return false
			}
			return exists
		},

		"resource_exists": func(contentHash []byte) bool {
			if len(contentHash) != 32 {
				return false
			}
			exists, err := hostABI.ResourceExists(ctx, contentHash)
			if err != nil {
				return false
			}
			return exists
		},

		// 注意：ONNX不提供以下能力（与WASM的区别）：
		// - ❌ 不提供GetCaller/GetContractAddress（ONNX模型无调用者概念）
		// - ❌ 不提供UTXOLookup/ResourceLookup（ONNX只需要存在性检查）
		// - ❌ 不提供任何交易草稿操作（ONNX不构建交易）
		// - ❌ 不提供EmitEvent/LogDebug（ONNX是纯计算，无副作用）
	}
}

