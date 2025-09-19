// Package aimodel AI模型推理实现
//
// 🎯 **模块定位**：AIModelService 接口的AI模型推理功能实现
//
// 本文件实现AI模型推理的核心业务逻辑，包括：
// - AI模型推理调用（InferAIModel）
// - 推理数据预处理和后处理
// - 推理结果验证和证明生成
// - 推理费用计算和支付
// - 推理性能监控和优化
//
// 🏗️ **架构定位**：
// - 业务层：实现AI模型推理的业务逻辑
// - 执行层：与AI推理引擎的深度集成
// - 证明层：生成推理结果的零知识证明
// - 计费层：处理按次推理的费用计算
//
// 🔧 **设计原则**：
// - 确定性推理：相同输入产生相同输出（在确定性模式下）
// - 隐私保护：支持输入数据和推理结果的隐私保护
// - 性能监控：详细的推理性能指标和资源消耗统计
// - 结果可信：通过零知识证明确保推理结果的可验证性
// - 灵活计费：支持多种推理计费模式
//
// 📋 **支持的推理模式**：
// - 同步推理：实时推理调用，立即返回结果
// - 异步推理：批量推理处理，异步返回结果
// - 批量推理：多个输入的批量处理
// - 隐私推理：基于零知识证明的隐私保护推理
//
// 🎯 **推理结果处理**：
// - 成功推理：创建 StateOutput 记录推理结果和证明
// - 推理失败：记录错误信息，退还计算费用
// - 性能监控：记录推理时间、内存使用等性能指标
// - 证明生成：生成推理过程的零知识证明
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package aimodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 类型定义
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/utils"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//
//	AI模型推理实现服务
//
// ============================================================================
// AIModelInferService AI模型推理核心实现服务
//
// 🎯 **服务职责**：
// - 实现 AIModelService.InferAIModel 方法
// - 处理各类AI模型的推理调用和执行
// - 管理推理数据的预处理和结果后处理
// - 计算和验证推理费用和性能指标
//
// 🔧 **依赖注入**：
// - aiInferenceEngine：AI推理执行引擎
// - stateManager：推理状态管理服务
// - proofGenerator：零知识证明生成服务
// - feeCalculator：推理费用计算服务
// - utxoSelector：UTXO 选择服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewAIModelInferService(inferEngine, stateManager, proofGen, feeCalc, utxoSelector, cache, logger)
//	txHash, err := service.InferAIModel(ctx, user, modelAddr, inputData)
type AIModelInferService struct {
	// 核心依赖服务（使用公共接口）
	utxoManager    repository.UTXOManager // UTXO 管理服务
	hashManager    crypto.HashManager     // 哈希计算服务
	keyManager     crypto.KeyManager      // 密钥管理服务（用于从私钥生成公钥）
	addressManager crypto.AddressManager  // 地址管理服务（用于从公钥生成地址）
	cacheStore     storage.MemoryStore    // 内存缓存存储
	logger         log.Logger             // 日志记录器
}

// NewAIModelInferService 创建AI模型推理服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - aiInferenceEngine: AI推理执行引擎
//   - stateManager: 推理状态管理服务
//   - proofGenerator: 零知识证明生成服务
//   - feeCalculator: 费用计算服务
//   - utxoSelector: UTXO 选择和管理服务
//   - cacheStore: 交易缓存存储服务
//   - logger: 日志记录器
//
// 返回：
//   - *AIModelInferService: AI模型推理服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewAIModelInferService(
	utxoManager repository.UTXOManager,
	hashManager crypto.HashManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	cacheStore storage.MemoryStore,
	logger log.Logger,
) *AIModelInferService {
	// 严格的依赖检查
	if logger == nil {
		panic("AIModelInferService: logger不能为nil")
	}
	if utxoManager == nil {
		logger.Warn("AIModelInferService: utxoManager为nil，某些功能将不可用")
	}
	if cacheStore == nil {
		logger.Warn("AIModelInferService: cacheStore为nil，某些功能将不可用")
	}

	return &AIModelInferService{
		utxoManager:    utxoManager,
		hashManager:    hashManager,
		keyManager:     keyManager,
		addressManager: addressManager,
		cacheStore:     cacheStore,
		logger:         logger,
	}
}

// ============================================================================
//
//	核心模型推理方法实现
//
// ============================================================================
// InferAIModel 实现AI模型推理功能（薄实现）
//
// 🎯 **方法职责**：
// 实现 blockchain.AIModelService.InferAIModel 接口
// 支持各类AI模型的推理调用和结果处理
//
// 📋 **业务流程**：
// 1. 验证推理调用参数的有效性
// 2. 解析模型地址和加载模型信息
// 3. 验证调用者的访问权限和余额
// 4. 预处理输入数据（格式转换、验证等）
// 5. 执行AI模型推理并监控性能
// 6. 后处理推理结果（格式化、验证等）
// 7. 生成推理过程的零知识证明
// 8. 构建包含 StateOutput 的推理交易
// 9. 计算和扣除推理费用
// 10. 将推理交易存储到内存缓存
// 11. 返回交易哈希供用户签名
//
// 📝 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - callerAddress: 推理调用者地址
//   - modelAddress: 目标AI模型地址
//   - inputData: 推理输入数据（张量格式）
//
// 📤 **返回值**：
//   - []byte: 交易哈希，用于后续签名和提交
//   - error: 错误信息，推理失败时返回具体原因
//
// 🎯 **支持场景**：
// - 图像分类：InferAIModel(ctx, user, imageClassifierAddr, imageData)
// - 文本生成：InferAIModel(ctx, user, gptModelAddr, promptData)
// - 语音识别：InferAIModel(ctx, user, speechModelAddr, audioData)
// - 推荐系统：InferAIModel(ctx, user, recommenderAddr, userProfile)
//
// 💡 **推理特性**：
// - 性能监控：详细的推理时间、内存使用等指标
// - 结果验证：通过零知识证明确保推理结果可信
// - 隐私保护：可选的输入数据和结果隐私保护
// - 灵活计费：支持按次、按时长、按资源消耗等计费模式
//
// ⚠️ **当前状态**：薄实现，返回未实现错误
func (s *AIModelInferService) InferAIModel(
	ctx context.Context,
	callerPrivateKey []byte,
	modelAddress string,
	inputData interface{},
	parameters map[string]interface{},
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 从私钥计算调用者地址（无状态设计）
	callerAddress, err := s.calculateAddressFromPrivateKey(callerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理AI模型推理请求 - caller: %s, model: %s, 参数数量: %d",
			callerAddress, modelAddress, len(parameters)))
	}

	// 🔄 步骤1: 基础参数验证
	if err := s.validateInferParams(modelAddress, inputData, parameters, options); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 🔧 步骤2: 合并推理选项并提取调用者地址
	mergedOptions, _, err := s.mergeInferOptions(options)
	if err != nil {
		return nil, fmt.Errorf("推理选项处理失败: %v", err)
	}

	// 🔄 步骤3: 序列化输入数据
	inputDataBytes, err := s.serializeInputData(inputData)
	if err != nil {
		return nil, fmt.Errorf("输入数据序列化失败: %v", err)
	}

	// 📍 步骤4: 解析调用者地址
	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("调用者地址解析失败: %v", err)
	}

	// 🌐 步骤5: 加载模型元数据和配置
	modelMetadata, err := s.loadModelInfo(ctx, modelAddress)
	if err != nil {
		return nil, fmt.Errorf("加载模型信息失败: %v", err)
	}

	// 💰 步骤6: 计算推理费用
	inferenceFee, err := s.calculateInferenceFeeAmount(modelAddress, inputDataBytes, parameters)
	if err != nil {
		return nil, fmt.Errorf("计算推理费用失败: %v", err)
	}

	// 💰 步骤7: 选择支付推理费用的UTXO
	selectedInputs, changeAmount, err := s.selectUTXOsForInference(
		ctx, callerAddrBytes, inferenceFee, "") // 原生代币支付推理费
	if err != nil {
		return nil, fmt.Errorf("推理费用UTXO选择失败: %v", err)
	}

	// 🤖 步骤8: 执行模拟推理（生成虚拟结果）
	inferenceResult, err := s.simulateInference(ctx, modelAddress, inputDataBytes, parameters, modelMetadata)
	if err != nil {
		return nil, fmt.Errorf("模拟推理失败: %v", err)
	}

	// 🏗️ 步骤9: 构建推理结果输出（StateOutput + 找零）
	outputs, err := s.buildInferenceOutputs(callerAddress, modelAddress, inferenceResult, changeAmount, mergedOptions)
	if err != nil {
		return nil, fmt.Errorf("推理输出构建失败: %v", err)
	}

	// 🔄 步骤A: 构建完整交易
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	// 🔄 步骤B: 计算交易哈希并缓存
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ AI模型推理交易构建完成 - txHash: %x, model: %s, 费用: %s",
			txHash, modelAddress, inferenceFee))
	}

	return txHash, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// validateModelAddress 验证AI模型地址格式
//
// 🔍 **验证项目**：
// - 地址长度和格式检查
// - 校验和验证
// - 模型存在性检查
// - 模型类型确认（必须是 AIMODEL）
//
// 参数：
//   - modelAddress: AI模型地址
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *AIModelInferService) validateModelAddress(modelAddress string) error {
	if len(modelAddress) == 0 {
		return fmt.Errorf("AI模型地址不能为空")
	}
	if s.logger != nil {
		s.logger.Debug("验证AI模型地址格式")
	}
	// TODO: 实现完整的模型地址验证
	// - 地址长度检查
	// - Base58Check 解码验证
	// - 校验和验证
	// - 模型存在性检查
	// - 确认资源类型为 AIMODEL
	return nil
}

// validateInputDataFormat 验证推理输入数据格式
//
// 🔍 **验证项目**：
// - 数据大小合理性检查
// - 张量格式验证
// - 数据类型检查
// - 维度兼容性验证
//
// 参数：
//   - inputData: 推理输入数据
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *AIModelInferService) validateInputDataFormat(inputData []byte) error {
	// 委托给新的输入数据类型验证
	return s.validateInputDataType(inputData)
}

// loadModelMetadata 加载AI模型的元数据信息
//
// 🔍 **加载内容**：
// - 模型输入输出规格
// - 推理性能参数
// - 访问权限控制
// - 计费配置信息
//
// 参数：
//   - ctx: 上下文对象
//   - modelAddress: AI模型地址
//
// 返回：
//   - map[string]interface{}: 模型元数据信息
//   - error: 加载失败时的错误信息
func (s *AIModelInferService) loadModelMetadata(
	ctx context.Context,
	modelAddress string,
) (map[string]interface{}, error) {
	// 委托给新的加载模型信息方法
	return s.loadModelInfo(ctx, modelAddress)
}

// preprocessInputData 预处理推理输入数据
//
// 🔧 **预处理操作**：
// - 数据格式转换（JSON → 张量）
// - 数据归一化和标准化
// - 维度调整和填充
// - 数据类型转换
//
// 参数：
//   - inputData: 原始输入数据
//   - modelMetadata: 模型元数据
//
// 返回：
//   - []byte: 预处理后的数据
//   - error: 预处理失败时的错误信息
func (s *AIModelInferService) preprocessInputData(
	inputData []byte,
	modelMetadata map[string]interface{},
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("预处理推理输入数据")
	}
	// 🚧 薄实现：数据预处理逻辑
	return nil, fmt.Errorf("推理输入数据预处理功能尚未实现")
}

// executeInference 执行AI模型推理
//
// 🚀 **推理执行过程**：
// - 创建推理执行上下文
// - 加载模型到推理引擎
// - 输入数据并执行推理
// - 监控推理性能指标
// - 获取推理结果
//
// 参数：
//   - ctx: 上下文对象
//   - modelAddress: 模型地址
//   - preprocessedData: 预处理后的输入数据
//   - modelMetadata: 模型元数据
//
// 返回：
//   - map[string]interface{}: 推理结果
//   - error: 推理失败时的错误信息
func (s *AIModelInferService) executeInference(
	ctx context.Context,
	modelAddress string,
	preprocessedData []byte,
	modelMetadata map[string]interface{},
) (map[string]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug("执行AI模型推理")
	}
	// 🚧 薄实现：委托给推理引擎
	return nil, fmt.Errorf("AI模型推理执行功能尚未实现，将委托给公共接口实现")
}

// postprocessInferenceResult 后处理推理结果
//
// 🔧 **后处理操作**：
// - 结果格式转换（张量 → JSON）
// - 置信度分析和排序
// - 结果验证和异常检测
// - 输出格式化
//
// 参数：
//   - inferenceResult: 原始推理结果
//   - modelMetadata: 模型元数据
//
// 返回：
//   - []byte: 后处理后的结果数据
//   - error: 后处理失败时的错误信息
func (s *AIModelInferService) postprocessInferenceResult(
	inferenceResult map[string]interface{},
	modelMetadata map[string]interface{},
) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("后处理推理结果")
	}
	// 🚧 薄实现：结果后处理逻辑
	return nil, fmt.Errorf("推理结果后处理功能尚未实现")
}

// buildInferenceStateOutput 构建推理状态输出
//
// 🏗️ **输出构建**：
// - 创建 StateOutput 类型
// - 包含推理结果哈希
// - 生成推理过程的零知识证明
// - 设置推理性能指标
//
// 参数：
//   - callerAddress: 调用者地址
//   - modelAddress: 模型地址
//   - inferenceResult: 推理结果
//   - processedResult: 后处理结果
//
// 返回：
//   - *transaction.TxOutput: 构建的状态输出
//   - error: 构建失败时的错误信息
func (s *AIModelInferService) buildInferenceStateOutput(
	callerAddress string,
	modelAddress string,
	inferenceResult map[string]interface{},
	processedResult []byte,
) (*transaction.TxOutput, error) {
	// 委托给新的状态输出构建方法
	return s.buildInferenceStateOutputForResult(callerAddress, modelAddress, inferenceResult)
}

// calculateInferenceFee 计算推理费用
//
// 🧮 **费用计算**：
// - 基础推理费用
// - 计算资源消耗费用
// - 模型使用授权费用
// - 结果存储费用
//
// 参数：
//   - modelAddress: 模型地址
//   - inputData: 输入数据
//   - inferenceResult: 推理结果
//
// 返回：
//   - uint64: 计算的推理费用
//   - error: 计算失败时的错误信息
func (s *AIModelInferService) calculateInferenceFee(
	modelAddress string,
	inputData []byte,
	inferenceResult map[string]interface{},
) (uint64, error) {
	// 委托给新的费用计算方法
	feeStr, err := s.calculateInferenceFeeAmount(modelAddress, inputData, nil)
	if err != nil {
		return 0, err
	}

	feeFloat, err := strconv.ParseFloat(feeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("费用转换失败: %v", err)
	}

	// 转换为最小单位（假设8位小数）
	return uint64(feeFloat * 100000000), nil
}

// maxInferenceInputSize 返回推理输入数据的最大支持大小
//
// 🎯 **限制原因**：
// - 控制推理执行的内存消耗
// - 防止过大输入影响推理性能
// - 保证合理的网络传输时间
//
// 返回：
//   - int: 最大推理输入大小（字节）
func maxInferenceInputSize() int {
	return 50 * 1024 * 1024 // 50MB，足够支持高分辨率图像等大型输入
}

// ============================================================================
//
//	新增辅助方法实现
//
// ============================================================================

// validateInferParams 验证AI模型推理参数
func (s *AIModelInferService) validateInferParams(
	modelAddress string,
	inputData interface{},
	parameters map[string]interface{},
	options []*types.TransferOptions,
) error {
	if modelAddress == "" {
		return fmt.Errorf("AI模型地址不能为空")
	}
	if inputData == nil {
		return fmt.Errorf("推理输入数据不能为空")
	}
	// 检查inputData的基本类型和大小
	if err := s.validateInputDataType(inputData); err != nil {
		return fmt.Errorf("输入数据类型验证失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug("✅ AI推理参数验证通过")
	}
	return nil
}

// validateInputDataType 验证输入数据类型
func (s *AIModelInferService) validateInputDataType(inputData interface{}) error {
	if inputData == nil {
		return fmt.Errorf("输入数据不能为空")
	}

	// 支持的输入数据类型：[]byte, string, map, slice
	switch data := inputData.(type) {
	case []byte:
		if len(data) == 0 {
			return fmt.Errorf("字节数组输入不能为空")
		}
		if len(data) > maxInferenceInputSize() {
			return fmt.Errorf("输入数据大小 %d 超过限制 %d 字节", len(data), maxInferenceInputSize())
		}
	case string:
		if data == "" {
			return fmt.Errorf("字符串输入不能为空")
		}
		if len(data) > maxInferenceInputSize() {
			return fmt.Errorf("输入数据大小 %d 超过限制 %d 字节", len(data), maxInferenceInputSize())
		}
	case map[string]interface{}:
		if len(data) == 0 {
			return fmt.Errorf("映射输入不能为空")
		}
	case []interface{}:
		if len(data) == 0 {
			return fmt.Errorf("数组输入不能为空")
		}
	default:
		// 尝试JSON序列化检查
		if _, err := json.Marshal(inputData); err != nil {
			return fmt.Errorf("不支持的输入数据类型: %T", inputData)
		}
	}

	return nil
}

// mergeInferOptions 合并推理选项并提取调用者地址
func (s *AIModelInferService) mergeInferOptions(options []*types.TransferOptions) (*types.TransferOptions, string, error) {
	// 默认调用者地址（从选项中提取或从上下文获取）
	callerAddress := "default_ai_caller_address" // TODO: 从上下文或选项中获取

	if len(options) == 0 {
		return nil, callerAddress, nil
	}

	// 合并多个选项（暂时返回最后一个）
	merged := options[len(options)-1]

	if s.logger != nil {
		s.logger.Debug("✅ AI推理选项处理完成")
	}

	return merged, callerAddress, nil
}

// serializeInputData 序列化输入数据
func (s *AIModelInferService) serializeInputData(inputData interface{}) ([]byte, error) {
	switch data := inputData.(type) {
	case []byte:
		return data, nil
	case string:
		return []byte(data), nil
	default:
		// JSON序列化其他类型
		serialized, err := json.Marshal(inputData)
		if err != nil {
			return nil, fmt.Errorf("JSON序列化失败: %v", err)
		}

		if len(serialized) > maxInferenceInputSize() {
			return nil, fmt.Errorf("序列化后数据大小 %d 超过限制 %d 字节", len(serialized), maxInferenceInputSize())
		}

		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("✅ 输入数据序列化完成，大小: %d 字节", len(serialized)))
		}

		return serialized, nil
	}
}

// parseAddress 解析地址字符串为字节数组
func (s *AIModelInferService) parseAddress(address string) ([]byte, error) {
	if address == "" {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 简单地址解析（实际应该使用地址编码系统）
	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		// 如果不是十六进制，尝试使用字符串字节
		addrBytes = []byte(address)
	}

	if len(addrBytes) > 64 { // 限制地址最大长度
		return nil, fmt.Errorf("地址过长，最大支持 64 字节")
	}

	return addrBytes, nil
}

// loadModelInfo 加载AI模型信息（实现版本）
func (s *AIModelInferService) loadModelInfo(ctx context.Context, modelAddress string) (map[string]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🔍 加载AI模型信息 - address: %s", modelAddress))
	}

	// TODO: 从区块链加载真实的模型信息
	// 目前返回模拟的模型元数据
	modelInfo := map[string]interface{}{
		"model_type":     "image_classification",
		"input_shape":    []int{224, 224, 3},
		"output_shape":   []int{1000},
		"model_format":   "ONNX",
		"version":        "1.0.0",
		"fee_per_call":   "0.001", // 每次推理费用
		"max_batch_size": 1,
	}

	if s.logger != nil {
		s.logger.Debug("✅ 模型信息加载完成")
	}

	return modelInfo, nil
}

// calculateInferenceFeeAmount 计算推理费用金额
func (s *AIModelInferService) calculateInferenceFeeAmount(
	modelAddress string,
	inputData []byte,
	parameters map[string]interface{},
) (string, error) {
	// 基础推理费用
	baseFee := 0.001 // 0.001 原生代币

	// 根据输入数据大小计算额外费用
	sizeFeePerMB := 0.0001
	sizeInMB := float64(len(inputData)) / (1024 * 1024)
	sizeFee := sizeInMB * sizeFeePerMB

	// 根据参数复杂度计算费用
	paramsFee := float64(len(parameters)) * 0.00001

	// 总费用
	totalFee := baseFee + sizeFee + paramsFee
	totalFeeStr := fmt.Sprintf("%.8f", totalFee)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💰 推理费用计算: 基础=%.8f, 大小=%.8f, 参数=%.8f, 总计=%.8f",
			baseFee, sizeFee, paramsFee, totalFee))
	}

	return totalFeeStr, nil
}

// simulateInference 模拟执行AI推理
func (s *AIModelInferService) simulateInference(
	ctx context.Context,
	modelAddress string,
	inputData []byte,
	parameters map[string]interface{},
	modelMetadata map[string]interface{},
) (map[string]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug("🤖 执行模拟AI推理")
	}

	// TODO: 集成真实的AI推理引擎
	// 目前返回模拟的推理结果
	result := map[string]interface{}{
		"predictions": []map[string]interface{}{
			{
				"class":       "cat",
				"confidence":  0.95,
				"probability": 0.95,
			},
			{
				"class":       "dog",
				"confidence":  0.03,
				"probability": 0.03,
			},
		},
		"inference_time_ms": 150,
		"model_version":     "1.0.0",
		"input_shape":       []int{224, 224, 3},
		"output_shape":      []int{1000},
		"processing_info": map[string]interface{}{
			"batch_size":        1,
			"preprocessing_ms":  20,
			"inference_ms":      100,
			"postprocessing_ms": 30,
		},
	}

	if s.logger != nil {
		s.logger.Info("✅ 模拟推理执行完成")
	}

	return result, nil
}

// buildInferenceOutputs 构建推理输出
func (s *AIModelInferService) buildInferenceOutputs(
	callerAddress string,
	modelAddress string,
	inferenceResult map[string]interface{},
	changeAmount string,
	options *types.TransferOptions,
) ([]*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建AI推理输出")
	}

	var outputs []*transaction.TxOutput
	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("调用者地址解析失败: %v", err)
	}

	// 1. 构建推理结果StateOutput（记录推理结果）
	stateOutput, err := s.buildInferenceStateOutputForResult(callerAddress, modelAddress, inferenceResult)
	if err != nil {
		return nil, fmt.Errorf("构建推理状态输出失败: %v", err)
	}
	outputs = append(outputs, stateOutput)

	// 2. 构建找零输出（如有需要）
	if changeAmount != "" && changeAmount != "0" {
		changeFloat, err := strconv.ParseFloat(changeAmount, 64)
		if err == nil && changeFloat > 0.00001 { // 最小找零门限
			changeOutput := &transaction.TxOutput{
				Owner: callerAddrBytes,
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: callerAddrBytes,
								},
								RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
								SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
							},
						},
					},
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: strconv.FormatUint(uint64(changeFloat*1e8), 10), // 🔥 修复：转换为整数wei字符串
							},
						},
					},
				},
			}
			outputs = append(outputs, changeOutput)

			if s.logger != nil {
				s.logger.Debug(fmt.Sprintf("💰 添加找零输出 - 金额: %s", changeAmount))
			}
		}
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 推理输出构建完成 - 总输出数: %d", len(outputs)))
	}

	return outputs, nil
}

// buildInferenceStateOutputForResult 构建推理状态输出（推理结果）
func (s *AIModelInferService) buildInferenceStateOutputForResult(
	callerAddress string,
	modelAddress string,
	inferenceResult map[string]interface{},
) (*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建推理结果状态输出")
	}

	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("调用者地址解析失败: %v", err)
	}

	// 生成推理的状态ID
	stateID := s.generateInferenceStateID(callerAddress, modelAddress, inferenceResult)

	// 计算推理结果哈希
	resultHash := s.calculateInferenceResultHash(modelAddress, inferenceResult)

	// 构建 StateOutput
	stateOutput := &transaction.TxOutput{
		Owner: callerAddrBytes,
		LockingConditions: []*transaction.LockingCondition{
			{
				Condition: &transaction.LockingCondition_SingleKeyLock{
					SingleKeyLock: &transaction.SingleKeyLock{
						KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: callerAddrBytes,
						},
						RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},
		OutputContent: &transaction.TxOutput_State{
			State: &transaction.StateOutput{
				StateId:             stateID,
				StateVersion:        1,                           // 推理结果版本
				ZkProof:             &transaction.ZKStateProof{}, // TODO: 实现ZK证明
				ExecutionResultHash: resultHash,
				ParentStateHash:     nil, // 无父状态
			},
		},
	}

	return stateOutput, nil
}

// buildCompleteTransaction 构建完整交易
func (s *AIModelInferService) buildCompleteTransaction(
	selectedInputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建完整AI推理交易")
	}

	tx := &transaction.Transaction{
		Version:           1,
		Inputs:            selectedInputs,
		Outputs:           outputs,
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte("weisyn-mainnet"),
	}

	return tx, nil
}

// cacheTransaction 缓存交易并返回哈希
func (s *AIModelInferService) cacheTransaction(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("📋 缓存AI推理交易")
	}

	// TODO: 使用真实的哈希计算
	// txHash := internal.ComputeTransactionHash(tx, s.hashManager)
	txHash := sha256.Sum256([]byte(fmt.Sprintf("ai_inference_%d", time.Now().UnixNano())))

	// 缓存到内存
	if s.cacheStore != nil {
		cacheKey := hex.EncodeToString(txHash[:])
		internal.CacheUnsignedTransaction(ctx, s.cacheStore, []byte(cacheKey), tx, internal.GetDefaultCacheConfig(), s.logger)
	}

	return txHash[:], nil
}

// generateInferenceStateID 生成推理状态ID
func (s *AIModelInferService) generateInferenceStateID(callerAddress, modelAddress string, inferenceResult map[string]interface{}) []byte {
	resultBytes, _ := json.Marshal(inferenceResult)
	combined := fmt.Sprintf("inference:%s:%s:%x", callerAddress, modelAddress, resultBytes)
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// calculateInferenceResultHash 计算推理结果哈希
func (s *AIModelInferService) calculateInferenceResultHash(modelAddress string, inferenceResult map[string]interface{}) []byte {
	resultBytes, _ := json.Marshal(inferenceResult)
	combined := fmt.Sprintf("result:%s:%x", modelAddress, resultBytes)
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// calculateAddressFromPrivateKey 从私钥计算地址（无状态设计的核心方法）
//
// 实现完整的私钥到地址的推导流程：
// 私钥 → 公钥(secp256k1) → 地址(Base58Check)
//
// 参数：
//   - privateKey: 32字节私钥
//
// 返回：
//   - string: WES标准地址
//   - error: 计算失败时的错误
func (s *AIModelInferService) calculateAddressFromPrivateKey(privateKey []byte) (string, error) {
	// 1. 从私钥导出公钥
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("从私钥导出公钥失败: %v", err)
	}

	// 2. 从公钥生成地址
	address, err := s.addressManager.PublicKeyToAddress(publicKey)
	if err != nil {
		return "", fmt.Errorf("从公钥生成地址失败: %v", err)
	}

	return address, nil
}

// ============================================================================
//                              内部UTXO选择方法
// ============================================================================

// selectUTXOsForInference 为AI模型推理选择UTXO（内部方法）
func (s *AIModelInferService) selectUTXOsForInference(ctx context.Context, callerAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, string, error) {
	targetAmount, err := s.parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, callerAddr, &assetCategory, true)
	if err != nil {
		return nil, "", fmt.Errorf("获取UTXO失败: %v", err)
	}

	if len(allUTXOs) == 0 {
		return nil, "", fmt.Errorf("地址没有可用UTXO")
	}

	var selectedInputs []*transaction.TxInput
	var totalSelected uint64 = 0

	for _, utxoItem := range allUTXOs {
		utxoAmount := s.extractUTXOAmount(utxoItem)
		if utxoAmount == 0 {
			continue
		}

		txInput := &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        utxoItem.Outpoint.TxId,
				OutputIndex: utxoItem.Outpoint.OutputIndex,
			},
			IsReferenceOnly: false,
			Sequence:        0xffffffff,
		}

		selectedInputs = append(selectedInputs, txInput)
		totalSelected += utxoAmount

		if totalSelected >= targetAmount {
			break
		}
	}

	if totalSelected < targetAmount {
		return nil, "", fmt.Errorf("余额不足，需要: %d, 可用: %d", targetAmount, totalSelected)
	}

	changeAmount := totalSelected - targetAmount
	changeStr := s.formatAmount(changeAmount)

	return selectedInputs, changeStr, nil
}

func (s *AIModelInferService) parseAmount(amountStr string) (uint64, error) {
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %v", err)
	}
	return amount, nil
}

func (s *AIModelInferService) extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
	if utxoItem == nil {
		return 0
	}

	switch strategy := utxoItem.ContentStrategy.(type) {
	case *utxo.UTXO_CachedOutput:
		if cachedOutput := strategy.CachedOutput; cachedOutput != nil {
			if assetOutput := cachedOutput.GetAsset(); assetOutput != nil {
				if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
					amount, err := utils.ParseAmountSafely(nativeCoin.Amount)
					if err != nil {
						return 0
					}
					return amount
				}
				if contractToken := assetOutput.GetContractToken(); contractToken != nil {
					amount, err := utils.ParseAmountSafely(contractToken.Amount)
					if err != nil {
						return 0
					}
					return amount
				}
			}
		}
	}

	return 0
}

func (s *AIModelInferService) formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// 确保 AIModelInferService 实现了所需的接口部分
var _ interface {
	InferAIModel(context.Context, []byte, string, interface{}, map[string]interface{}, ...*types.TransferOptions) ([]byte, error)
} = (*AIModelInferService)(nil)
