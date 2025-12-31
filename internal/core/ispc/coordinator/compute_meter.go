package coordinator

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ResourceType 资源类型枚举
type ResourceType int

const (
	ResourceTypeContract ResourceType = iota + 1
	ResourceTypeAIModel
)

// String 返回资源类型的字符串表示
func (rt ResourceType) String() string {
	switch rt {
	case ResourceTypeContract:
		return "CONTRACT"
	case ResourceTypeAIModel:
		return "AI_MODEL"
	default:
		return "UNKNOWN"
	}
}

// OperationStats 操作统计信息
//
// 用于记录执行过程中的各种操作统计，用于更精确的 CU 计算
type OperationStats struct {
	StorageOps         uint64 // 存储操作次数
	CrossContractCalls uint64 // 跨合约调用次数
	// Phase 5: 预留多维资源使用字段（当前仅统计，不计费）
	StorageBytes       uint64 // 存储使用量（字节）- 未来扩展
	BandwidthInBytes   uint64 // 输入带宽使用量（字节）- 未来扩展
	BandwidthOutBytes  uint64 // 输出带宽使用量（字节）- 未来扩展
	// 未来可扩展：网络请求次数、文件IO次数等
}

// ComputeMeter 算力计量器接口
//
// 🎯 **核心职责**：
// - 为 CONTRACT 和 AI_MODEL 提供统一的算力计量能力
// - 计算 Compute Units (CU)，作为算力消费的标准化度量
// - 支持资源复杂度系数和操作统计的灵活计算
//
// 💡 **设计原则**：
// - 统一接口：CONTRACT 和 AI_MODEL 使用相同的计量接口
// - 可扩展性：支持未来多维资源计量（存储、带宽等）
// - 确定性：相同输入必须产生相同的 CU 值
//
// 📋 **CU 计算公式**：
//   CU = base_cu + (input_size_bytes / 1024) * input_factor + (exec_time_ms / 100) * time_factor + ops_contribution
//
//   其中：
//   - base_cu: 基础 CU（资源类型相关）
//   - input_factor: 输入大小因子（默认 0.1）
//   - time_factor: 执行时间因子（默认 1.0）
//   - 复杂度系数：资源特定的调整因子（默认 1.0）
type ComputeMeter interface {
	// GetComplexityFactor 获取资源复杂度系数
	//
	// 参数：
	//   - ctx: 上下文
	//   - rType: 资源类型（CONTRACT / AI_MODEL）
	//   - resourceHash: 资源内容哈希
	//
	// 返回：
	//   - float64: 复杂度系数（>= 1.0，默认 1.0）
	//   - error: 获取失败时的错误
	//
	// 💡 **用途**：
	//   - 不同资源可能有不同的计算复杂度
	//   - 例如：大型 AI 模型可能比简单合约需要更多 CU
	//   - 默认返回 1.0，表示标准复杂度
	GetComplexityFactor(ctx context.Context, rType ResourceType, resourceHash []byte) (float64, error)

	// CalculateCU 计算算力单位（Compute Units）
	//
	// 参数：
	//   - ctx: 上下文
	//   - rType: 资源类型（CONTRACT / AI_MODEL）
	//   - resourceHash: 资源内容哈希
	//   - inputSizeBytes: 输入数据大小（字节）
	//   - execTimeMs: 执行时间（毫秒）
	//   - ops: 操作统计信息
	//
	// 返回：
	//   - float64: 计算出的 CU 值（>= 0）
	//   - error: 计算失败时的错误
	//
	// 📋 **计算逻辑**：
	//   1. 获取资源复杂度系数
	//   2. 计算基础 CU（资源类型相关）
	//   3. 计算输入大小贡献：input_size_bytes / 1024 * input_factor
	//   4. 计算执行时间贡献：exec_time_ms / 100 * time_factor
	//   5. 计算操作统计贡献：ops.storage_ops * storage_factor + ops.cross_contract_calls * call_factor
	//   6. 应用复杂度系数：total_cu * complexity_factor
	//   7. 返回最终 CU 值
	CalculateCU(
		ctx context.Context,
		rType ResourceType,
		resourceHash []byte,
		inputSizeBytes uint64,
		execTimeMs uint64,
		ops OperationStats,
	) (float64, error)
}

// DefaultComputeMeter 默认算力计量器实现
//
// 🎯 **实现特点**：
// - 使用完整的 CU 计算公式（base_cu + input_contribution + time_contribution + ops_contribution）
// - 所有资源使用相同的复杂度系数（1.0，可扩展）
// - 支持未来扩展为更复杂的计算策略
type DefaultComputeMeter struct {
	logger log.Logger

	// 配置参数（未来可从配置文件读取）
	baseCUContract   float64 // 合约基础 CU（默认 1.0）
	baseCUAI         float64 // AI 模型基础 CU（默认 2.0）
	inputFactor      float64 // 输入大小因子（默认 0.1）
	timeFactor       float64 // 执行时间因子（默认 1.0）
	storageOpFactor  float64 // 存储操作因子（默认 0.5）
	crossCallFactor  float64 // 跨合约调用因子（默认 2.0）
}

// NewDefaultComputeMeter 创建默认算力计量器
//
// 参数：
//   - logger: 日志服务
//
// 返回：
//   - *DefaultComputeMeter: 新创建的实例
func NewDefaultComputeMeter(logger log.Logger) *DefaultComputeMeter {
	return &DefaultComputeMeter{
		logger:          logger,
		baseCUContract:  1.0,
		baseCUAI:       2.0,
		inputFactor:    0.1,
		timeFactor:     1.0,
		storageOpFactor: 0.5,
		crossCallFactor: 2.0,
	}
}

// GetComplexityFactor 获取资源复杂度系数
//
// MVP 实现：所有资源返回默认复杂度系数 1.0
// 未来可扩展：根据资源哈希查询资源元数据，返回实际复杂度系数
func (m *DefaultComputeMeter) GetComplexityFactor(
	ctx context.Context,
	rType ResourceType,
	resourceHash []byte,
) (float64, error) {
	// MVP: 返回默认复杂度系数
	// 未来可扩展：查询资源元数据，返回实际复杂度系数
	if m.logger != nil {
		m.logger.Debugf("获取资源复杂度系数: type=%s, hash=%x, factor=1.0 (default)",
			rType.String(), resourceHash)
	}
	return 1.0, nil
}

// CalculateCU 计算算力单位（Compute Units）
//
// 📋 **CU 计算公式**：
//   base_cu = (rType == CONTRACT) ? baseCUContract : baseCUAI
//   input_contribution = (input_size_bytes / 1024) * input_factor
//   time_contribution = (exec_time_ms / 100) * time_factor
//   ops_contribution = ops.storage_ops * storage_op_factor + ops.cross_contract_calls * cross_call_factor
//   total_cu = base_cu + input_contribution + time_contribution + ops_contribution
//   final_cu = total_cu * complexity_factor
func (m *DefaultComputeMeter) CalculateCU(
	ctx context.Context,
	rType ResourceType,
	resourceHash []byte,
	inputSizeBytes uint64,
	execTimeMs uint64,
	ops OperationStats,
) (float64, error) {
	// 1. 获取资源复杂度系数
	complexityFactor, err := m.GetComplexityFactor(ctx, rType, resourceHash)
	if err != nil {
		return 0, fmt.Errorf("获取资源复杂度系数失败: %w", err)
	}

	// 2. 计算基础 CU（资源类型相关）
	var baseCU float64
	switch rType {
	case ResourceTypeContract:
		baseCU = m.baseCUContract
	case ResourceTypeAIModel:
		baseCU = m.baseCUAI
	default:
		return 0, fmt.Errorf("不支持的资源类型: %d", rType)
	}

	// 3. 计算输入大小贡献（每 KB 贡献 input_factor CU）
	inputContribution := (float64(inputSizeBytes) / 1024.0) * m.inputFactor

	// 4. 计算执行时间贡献（每 100ms 贡献 time_factor CU）
	timeContribution := (float64(execTimeMs) / 100.0) * m.timeFactor

	// 5. 计算操作统计贡献
	storageContribution := float64(ops.StorageOps) * m.storageOpFactor
	crossCallContribution := float64(ops.CrossContractCalls) * m.crossCallFactor
	opsContribution := storageContribution + crossCallContribution

	// 6. 计算总 CU
	totalCU := baseCU + inputContribution + timeContribution + opsContribution

	// 7. 应用复杂度系数
	finalCU := totalCU * complexityFactor

	// 8. 确保 CU >= 0（防止负数）
	if finalCU < 0 {
		finalCU = 0
	}

	// 9. 记录日志（如果启用）
	if m.logger != nil {
		m.logger.Debugf("计算 CU: type=%s, hash=%x, base=%.2f, input=%.2f, time=%.2f, ops=%.2f, factor=%.2f, final=%.2f",
			rType.String(), resourceHash, baseCU, inputContribution, timeContribution, opsContribution, complexityFactor, finalCU)
	}

	return math.Round(finalCU*100) / 100, nil // 保留两位小数
}

// CalculateCUFromExecution 从执行结果计算 CU（便捷方法）
//
// 参数：
//   - ctx: 上下文
//   - rType: 资源类型
//   - resourceHash: 资源哈希
//   - inputSizeBytes: 输入大小
//   - startTime: 执行开始时间
//   - endTime: 执行结束时间
//   - ops: 操作统计
//
// 返回：
//   - float64: CU 值
//   - error: 错误
func (m *DefaultComputeMeter) CalculateCUFromExecution(
	ctx context.Context,
	rType ResourceType,
	resourceHash []byte,
	inputSizeBytes uint64,
	startTime time.Time,
	endTime time.Time,
	ops OperationStats,
) (float64, error) {
	execTimeMs := uint64(endTime.Sub(startTime).Milliseconds())
	return m.CalculateCU(ctx, rType, resourceHash, inputSizeBytes, execTimeMs, ops)
}

