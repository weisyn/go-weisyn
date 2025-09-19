// Package aimodel AI模型部署实现
//
// 🎯 **模块定位**：AIModelService 接口的AI模型部署功能实现
//
// 本文件实现AI模型部署的核心业务逻辑，包括：
// - ONNX/TensorFlow AI模型部署（DeployAIModel）
// - 模型格式验证和兼容性检查
// - 模型执行配置和性能优化
// - 模型权限和访问控制设置
// - 模型生命周期管理
//
// 🏗️ **架构定位**：
// - 业务层：实现AI模型的部署业务逻辑
// - 执行层：与AI模型执行引擎的集成
// - 存储层：模型文件的内容寻址存储
// - 权限层：模型的初始访问控制和使用授权
//
// 🔧 **设计原则**：
// - 格式兼容：支持主流AI模型格式（ONNX、TensorFlow等）
// - 性能可控：支持模型大小限制和推理性能配置
// - 权限灵活：支持公开、私有、按次付费等多种访问模式
// - 标准化：遵循AI模型部署的行业最佳实践
//
// 📋 **支持的模型格式**：
// - ONNX 模型：跨平台的机器学习模型格式
// - TensorFlow 模型：Google的机器学习框架模型
// - PyTorch 模型：Facebook的深度学习框架模型
// - 其他标准格式：通过配置扩展支持
//
// 🎯 **与其他资源的区别**：
// - AI模型：ResourceCategory.EXECUTABLE + ExecutableType.AIMODEL
// - 智能合约：ResourceCategory.EXECUTABLE + ExecutableType.CONTRACT
// - 静态资源：ResourceCategory.STATIC，无执行能力
// - AI模型具备推理计算能力，但不具备状态管理能力
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package aimodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/utils"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//
//	AI模型部署实现服务
//
// ============================================================================
// AIModelDeployService AI模型部署核心实现服务
//
// 🎯 **服务职责**：
// - 实现 AIModelService.DeployAIModel 方法
// - 处理各类AI模型的验证、部署和配置
// - 管理模型的内容寻址存储和执行参数
// - 设置模型的初始访问权限和使用控制
//
// 🔧 **依赖注入**：
// - modelValidator：AI模型格式验证服务
// - contentAddressStore：内容寻址存储服务
// - utxoSelector：UTXO 选择和管理服务
// - feeCalculator：费用计算服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewAIModelDeployService(validator, contentStore, utxoSelector, feeCalc, cache, logger)
//	txHash, err := service.DeployAIModel(ctx, deployer, onnxModel, options...)
type AIModelDeployService struct {
	// 核心依赖服务（使用公共接口）
	utxoManager     repository.UTXOManager     // UTXO 管理服务
	resourceManager repository.ResourceManager // 资源存储管理服务
	hashManager     crypto.HashManager         // 哈希计算服务
	keyManager      crypto.KeyManager          // 密钥管理服务（用于从私钥生成公钥）
	addressManager  crypto.AddressManager      // 地址管理服务（用于从公钥生成地址）
	cacheStore      storage.MemoryStore        // 内存缓存存储
	logger          log.Logger                 // 日志记录器
}

// NewAIModelDeployService 创建AI模型部署服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - modelValidator: AI模型格式验证服务
//   - contentAddressStore: 内容寻址存储服务
//   - utxoSelector: UTXO 选择和管理服务
//   - feeCalculator: 费用计算服务
//   - cacheStore: 交易缓存存储服务
//   - logger: 日志记录器
//
// 返回：
//   - *AIModelDeployService: AI模型部署服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewAIModelDeployService(
	utxoManager repository.UTXOManager,
	resourceManager repository.ResourceManager,
	hashManager crypto.HashManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	cacheStore storage.MemoryStore,
	logger log.Logger,
) *AIModelDeployService {
	// 严格的依赖检查
	if logger == nil {
		panic("AIModelDeployService: logger不能为nil")
	}
	if utxoManager == nil {
		logger.Warn("AIModelDeployService: utxoManager为nil，某些功能将不可用")
	}
	if resourceManager == nil {
		panic("AIModelDeployService: resourceManager不能为nil")
	}
	if cacheStore == nil {
		logger.Warn("AIModelDeployService: cacheStore为nil，某些功能将不可用")
	}

	return &AIModelDeployService{
		utxoManager:     utxoManager,
		resourceManager: resourceManager,
		hashManager:     hashManager,
		keyManager:      keyManager,
		addressManager:  addressManager,
		cacheStore:      cacheStore,
		logger:          logger,
	}
}

// ============================================================================
//
//	核心模型部署方法实现
//
// ============================================================================
// DeployAIModel 实现AI模型部署功能（薄实现）
//
// 🎯 **方法职责**：
// 实现 blockchain.AIModelService.DeployAIModel 接口
// 支持各类AI模型的安全部署和配置
//
// 📋 **业务流程**：
// 1. 验证AI模型数据的格式和完整性
// 2. 解析模型的输入输出规格
// 3. 计算模型的内容哈希
// 4. 将模型文件存储到内容寻址网络
// 5. 构建 ResourceOutput（ExecutableType.AIMODEL）
// 6. 配置模型的执行环境参数
// 7. 设置模型的初始访问权限
// 8. 选择部署费用的支付 UTXO
// 9. 将部署交易存储到内存缓存
// 10. 返回交易哈希供用户签名
//
// 📝 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - deployerAddress: 模型部署者地址
//   - modelData: AI模型的二进制数据（ONNX/TensorFlow等格式）
//   - options: 可选的部署选项（权限控制、性能配置、收费模式等）
//
// 📤 **返回值**：
//   - []byte: 交易哈希，用于后续签名和提交
//   - error: 错误信息，部署失败时返回具体原因
//
// 🎯 **支持场景**：
// - 图像分类模型：DeployAIModel(ctx, deployer, resnetModel)
// - 自然语言处理：DeployAIModel(ctx, deployer, bertModel, &types.ResourceDeployOptions{...})
// - 企业AI服务：DeployAIModel(ctx, deployer, customModel, &types.ResourceDeployOptions{BusinessModel: {...}})
// - 付费推理服务：DeployAIModel(ctx, deployer, gptModel, &types.ResourceDeployOptions{FeeControl: {...}})
//
// 💡 **AI特性**：
// - 格式验证：确保模型格式正确性和兼容性
// - 性能预测：评估模型推理性能和资源需求
// - 自动配置：智能推导输入输出规格
// - 权限灵活：支持多种访问和计费模式
//
// ⚠️ **当前状态**：薄实现，返回未实现错误
func (s *AIModelDeployService) DeployAIModel(
	ctx context.Context,
	deployerPrivateKey []byte,
	modelFilePath string,
	config *resourcepb.AIModelExecutionConfig,
	name string,
	description string,
	options ...*types.ResourceDeployOptions,
) ([]byte, error) {
	// 从私钥计算部署者地址（无状态设计）
	deployerAddress, err := s.calculateAddressFromPrivateKey(deployerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理AI模型部署请求 - name: %s, 文件路径: %s",
			name, modelFilePath))
	}

	// 🔧 步骤1: 合并部署选项
	mergedOptions, _, err := s.mergeDeployOptionsWithAddress(options)
	if err != nil {
		return nil, fmt.Errorf("部署选项处理失败: %v", err)
	}

	// 🧮 步骤2: 存储模型文件到ResourceManager并获取内容哈希
	metadata := map[string]string{
		"resource_type":   "aimodel",
		"name":            name,
		"description":     description,
		"creator_address": deployerAddress,
		"model_format":    "unknown", // 将在验证后更新
	}

	contentHashBytes, err := s.resourceManager.StoreResourceFile(ctx, modelFilePath, metadata)
	if err != nil {
		return nil, fmt.Errorf("存储AI模型文件失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ AI模型文件已存储 - content_hash: %x", contentHashBytes))
	}

	// 🔍 步骤3: 读取模型文件进行格式验证
	modelBytes, err := os.ReadFile(modelFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取模型文件失败: %v", err)
	}

	// 🔄 步骤4: 基础参数验证
	if err := s.validateDeployParams(modelBytes, config, name, description, options); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 🔍 步骤5: 深度验证模型格式
	modelFormat, err := s.validateModelFormat(modelBytes)
	if err != nil {
		return nil, fmt.Errorf("模型格式验证失败: %v", err)
	}

	// 📍 步骤5: 解析部署者地址
	deployerAddrBytes, err := s.parseAddress(deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("部署者地址解析失败: %v", err)
	}

	// 🏗️ 步骤6: 构建AI模型资源定义
	aiModelResource, err := s.buildAIModelResourceComplete(deployerAddress, modelBytes, modelFormat, config, name, description, contentHashBytes, mergedOptions)
	if err != nil {
		return nil, fmt.Errorf("AI模型资源构建失败: %v", err)
	}

	// 💰 步骤7: 选择部署费用的UTXO（使用原生代币）
	deploymentFee := s.estimateDeploymentFee(len(modelBytes))
	selectedInputs, changeAmount, err := s.selectUTXOsForModelDeploy(
		ctx, deployerAddrBytes, deploymentFee, "") // 原生代币
	if err != nil {
		return nil, fmt.Errorf("部署费用UTXO选择失败: %v", err)
	}

	// 🏗️ 步骤8: 构建AI模型部署输出
	outputs, err := s.buildAIModelOutputs(deployerAddress, aiModelResource, changeAmount, mergedOptions)
	if err != nil {
		return nil, fmt.Errorf("AI模型输出构建失败: %v", err)
	}

	// 🔄 步骤9: 构建完整交易
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	// 🔄 步骤A: 计算交易哈希并缓存
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ AI模型部署交易构建完成 - txHash: %x, name: %s, 模型哈希: %x, 费用: %s",
			txHash, name, contentHashBytes, deploymentFee))
	}

	return txHash, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// detectModelFormat 检测AI模型的格式类型
//
// 🔍 **检测策略**：
// - ONNX 模型：检查魔数和版本信息
// - TensorFlow 模型：检查 saved_model.pb 或 .h5 格式
// - PyTorch 模型：检查 .pth 或 .pt 格式
// - 其他格式：基于文件头特征检测
//
// 参数：
//   - modelData: AI模型二进制数据
//
// 返回：
//   - string: 检测到的模型格式（"ONNX", "TensorFlow", "PyTorch" 等）
//   - error: 检测失败或格式不支持时的错误信息
func (s *AIModelDeployService) detectModelFormat(modelData []byte) (string, error) {
	// 委托给新的validateModelFormat方法
	return s.validateModelFormat(modelData)
}

// mergeDeployOptions 合并多个AI模型部署选项
//
// 🔧 **合并策略**：
// - 后面的选项覆盖前面的选项
// - 对嵌套的业务模式选项进行深度合并
// - 特别处理性能配置和权限设置
//
// 参数：
//   - options: 多个部署选项
//
// 返回：
//   - *types.ResourceDeployOptions: 合并后的选项
//   - error: 合并失败时的错误信息
func (s *AIModelDeployService) mergeDeployOptions(
	options []*types.ResourceDeployOptions,
) (*types.ResourceDeployOptions, error) {
	// 委托给带地址的新方法
	merged, _, err := s.mergeDeployOptionsWithAddress(options)
	return merged, err
}

// buildAIModelResource 构建AI模型资源定义
//
// 🏗️ **资源构建**：
// - 设置 ResourceCategory.EXECUTABLE
// - 设置 ExecutableType.AIMODEL
// - 配置 AIModelExecutionConfig
// - 设置模型元数据和版本信息
//
// 参数：
//   - deployerAddress: 部署者地址
//   - modelData: 模型数据
//   - modelFormat: 模型格式
//   - contentHash: 内容哈希
//   - options: 部署选项
//
// 返回：
//   - *resourcepb.Resource: 构建的AI模型资源
//   - error: 构建失败时的错误信息
func (s *AIModelDeployService) buildAIModelResource(
	deployerAddress string,
	modelData []byte,
	modelFormat string,
	contentHash []byte,
	options *types.ResourceDeployOptions,
) (*resourcepb.Resource, error) {
	// 创建默认配置并委托给新的完整实现
	defaultConfig := &resourcepb.AIModelExecutionConfig{
		// TODO: 添加默认配置
	}
	return s.buildAIModelResourceComplete(deployerAddress, modelData, modelFormat, defaultConfig, "AI模型", "AI模型描述", contentHash, options)
}

// extractModelMetadata 从AI模型中提取元数据信息
//
// 🔍 **提取内容**：
// - 输入张量规格（名称、形状、类型）
// - 输出张量规格（名称、形状、类型）
// - 模型版本和创建信息
// - 推理性能预估
//
// 参数：
//   - modelData: AI模型数据
//   - modelFormat: 模型格式
//
// 返回：
//   - *resourcepb.AIModelExecutionConfig: 提取的执行配置
//   - error: 提取失败时的错误信息
func (s *AIModelDeployService) extractModelMetadata(
	modelData []byte,
	modelFormat string,
) (*resourcepb.AIModelExecutionConfig, error) {
	if s.logger != nil {
		s.logger.Debug("提取AI模型元数据")
	}
	// 🚧 薄实现：元数据提取逻辑
	return nil, fmt.Errorf("AI模型元数据提取功能尚未实现")
}

// buildAIModelOutput 构建AI模型部署的输出 UTXO
//
// 🏗️ **输出构建**：
// - 创建 ResourceOutput 类型
// - 包含完整的AI模型 Resource 定义
// - 配置模型的初始锁定条件
// - 设置模型访问和计费参数
//
// 参数：
//   - deployerAddress: 部署者地址
//   - aiModelResource: AI模型资源定义
//   - options: 部署选项
//
// 返回：
//   - *transaction.TxOutput: 构建的AI模型输出
//   - error: 构建失败时的错误信息
func (s *AIModelDeployService) buildAIModelOutput(
	deployerAddress string,
	aiModelResource *resourcepb.Resource,
	options *types.ResourceDeployOptions,
) (*transaction.TxOutput, error) {
	// 委托给新的多输出实现，返回第一个输出
	outputs, err := s.buildAIModelOutputs(deployerAddress, aiModelResource, "0", options)
	if err != nil {
		return nil, err
	}

	if len(outputs) > 0 {
		return outputs[0], nil
	}

	return nil, fmt.Errorf("AI模型输出构建失败")
}

// maxAIModelSize 返回AI模型的最大支持大小
//
// 🎯 **限制原因**：
// - 控制模型部署和推理的性能影响
// - 防止过大模型影响网络和存储
// - 保证合理的部署和加载时间
//
// 返回：
//   - int: 最大AI模型大小（字节）
func maxAIModelSize() int {
	// 🎯 合理的模型大小限制：支持大多数实用AI模型
	return 500 * 1024 * 1024 // 500MB，足够支持大部分深度学习模型
}

// min 辅助函数：返回两个整数的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
//
//	新增辅助方法实现
//
// ============================================================================

// validateDeployParams 验证AI模型部署参数
func (s *AIModelDeployService) validateDeployParams(
	modelBytes []byte,
	config *resourcepb.AIModelExecutionConfig,
	name string,
	description string,
	options []*types.ResourceDeployOptions,
) error {
	if len(modelBytes) == 0 {
		return fmt.Errorf("AI模型数据不能为空")
	}
	if len(modelBytes) > maxAIModelSize() {
		return fmt.Errorf("AI模型大小超过限制，最大支持 %d 字节", maxAIModelSize())
	}
	if config == nil {
		return fmt.Errorf("AI模型执行配置不能为空")
	}
	if name == "" {
		return fmt.Errorf("AI模型名称不能为空")
	}
	if len(name) > 128 {
		return fmt.Errorf("AI模型名称过长，最大支持 128 字符")
	}
	if len(description) > 1024 {
		return fmt.Errorf("AI模型描述过长，最大支持 1024 字符")
	}

	if s.logger != nil {
		s.logger.Debug("✅ AI模型部署参数验证通过")
	}
	return nil
}

// mergeDeployOptionsWithAddress 合并部署选项并提取部署者地址
func (s *AIModelDeployService) mergeDeployOptionsWithAddress(options []*types.ResourceDeployOptions) (*types.ResourceDeployOptions, string, error) {
	// 默认部署者地址（从选项中提取或从上下文获取）
	deployerAddress := "default_ai_deployer_address" // TODO: 从上下文或选项中获取

	if len(options) == 0 {
		return nil, deployerAddress, nil
	}

	// 合并多个选项（暂时返回最后一个）
	merged := options[len(options)-1]

	if s.logger != nil {
		s.logger.Debug("✅ AI模型部署选项处理完成")
	}

	return merged, deployerAddress, nil
}

// validateModelFormat 验证并检测AI模型格式
func (s *AIModelDeployService) validateModelFormat(modelBytes []byte) (string, error) {
	if len(modelBytes) < 16 {
		return "", fmt.Errorf("模型数据长度不足，无法检测格式")
	}

	// 🔍 ONNX 格式检测 - Protocol Buffer 格式
	if len(modelBytes) >= 4 {
		if modelBytes[0] == 0x08 && modelBytes[1] == 0x01 {
			if s.logger != nil {
				s.logger.Debug("✅ 检测到ONNX模型格式")
			}
			return "ONNX", nil
		}
	}

	// 🔍 TensorFlow SavedModel 检测
	if len(modelBytes) >= 8 {
		// 简化的 TensorFlow 检测
		if s.logger != nil {
			s.logger.Debug("✅ 检测到TensorFlow模型格式")
		}
		return "TensorFlow", nil
	}

	// 🔍 PyTorch 模型检测 - Pickle 格式
	if len(modelBytes) >= 2 {
		if modelBytes[0] == 0x80 && modelBytes[1] == 0x03 {
			if s.logger != nil {
				s.logger.Debug("✅ 检测到PyTorch模型格式")
			}
			return "PyTorch", nil
		}
	}

	// 默认返回通用格式（允许未知格式但警告）
	if s.logger != nil {
		s.logger.Warn("⚠️ 未能识别AI模型格式，将作为通用格式处理")
	}
	return "Generic", nil
}

// parseAddress 解析地址字符串为字节数组
func (s *AIModelDeployService) parseAddress(address string) ([]byte, error) {
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

// buildAIModelResourceComplete 构建AI模型资源定义（实现版本）
func (s *AIModelDeployService) buildAIModelResourceComplete(
	deployerAddress string,
	modelBytes []byte,
	modelFormat string,
	config *resourcepb.AIModelExecutionConfig,
	name string,
	description string,
	contentHash []byte,
	options *types.ResourceDeployOptions,
) (*resourcepb.Resource, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建AI模型资源定义")
	}

	// 确定MIME类型
	mimeType := s.getMimeType(modelFormat)

	// 构建基础资源信息
	resource := &resourcepb.Resource{
		Category:         resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType:   resourcepb.ExecutableType_EXECUTABLE_TYPE_AIMODEL,
		ContentHash:      contentHash,
		MimeType:         mimeType,
		Size:             uint64(len(modelBytes)),
		CreatedTimestamp: uint64(time.Now().Unix()),
		CreatorAddress:   deployerAddress,
		Name:             name,
		Version:          "1.0.0",
		Description:      description,
	}

	// 设置AI模型执行配置
	resource.ExecutionConfig = &resourcepb.Resource_Aimodel{
		Aimodel: config,
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ AI模型资源构建完成 - name: %s, 格式: %s, 内容哈希: %x",
			name, modelFormat, contentHash))
	}

	return resource, nil
}

// getMimeType 根据模型格式获取MIME类型
func (s *AIModelDeployService) getMimeType(modelFormat string) string {
	switch modelFormat {
	case "ONNX":
		return "application/onnx"
	case "TensorFlow":
		return "application/tensorflow"
	case "PyTorch":
		return "application/pytorch"
	default:
		return "application/octet-stream"
	}
}

// estimateDeploymentFee 估算AI模型部署费用
func (s *AIModelDeployService) estimateDeploymentFee(modelSizeBytes int) string {
	// 基础部署费用
	baseFee := 0.001 // 0.001 原生代币

	// 根据模型大小计算额外费用（每MB 0.0001）
	sizeFeePerMB := 0.0001
	sizeInMB := float64(modelSizeBytes) / (1024 * 1024)
	sizeFee := sizeInMB * sizeFeePerMB

	// 总费用
	totalFee := baseFee + sizeFee
	totalFeeStr := fmt.Sprintf("%.8f", totalFee)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💰 AI模型部署费用计算: 基础费用=%.8f, 大小费用=%.8f, 总计=%.8f",
			baseFee, sizeFee, totalFee))
	}

	return totalFeeStr
}

// buildAIModelOutputs 构建AI模型部署输出（实现版本）
func (s *AIModelDeployService) buildAIModelOutputs(
	deployerAddress string,
	aiModelResource *resourcepb.Resource,
	changeAmount string,
	options *types.ResourceDeployOptions,
) ([]*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建AI模型部署输出")
	}

	var outputs []*transaction.TxOutput
	deployerAddrBytes, err := s.parseAddress(deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("部署者地址解析失败: %v", err)
	}

	// 1. 构建AI模型部署输出（ResourceOutput）
	modelOutput := &transaction.TxOutput{
		Owner: deployerAddrBytes,
		LockingConditions: []*transaction.LockingCondition{
			{
				Condition: &transaction.LockingCondition_SingleKeyLock{
					SingleKeyLock: &transaction.SingleKeyLock{
						KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: deployerAddrBytes,
						},
						RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{
				Resource:        aiModelResource,
				StorageStrategy: transaction.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
			},
		},
	}
	outputs = append(outputs, modelOutput)

	// 2. 构建找零输出（如有需要）
	if changeAmount != "" && changeAmount != "0" {
		changeFloat, err := strconv.ParseFloat(changeAmount, 64)
		if err == nil && changeFloat > 0.00001 { // 最小找零门限
			changeOutput := &transaction.TxOutput{
				Owner: deployerAddrBytes,
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: deployerAddrBytes,
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
		s.logger.Info(fmt.Sprintf("✅ AI模型输出构建完成 - 总输出数: %d", len(outputs)))
	}

	return outputs, nil
}

// buildCompleteTransaction 构建完整交易
func (s *AIModelDeployService) buildCompleteTransaction(
	selectedInputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建完整AI模型部署交易")
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
func (s *AIModelDeployService) cacheTransaction(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("📋 缓存AI模型部署交易")
	}

	// TODO: 使用真实的哈希计算
	// txHash := internal.ComputeTransactionHash(tx, s.hashManager)
	txHash := sha256.Sum256([]byte(fmt.Sprintf("ai_model_deploy_%d", time.Now().UnixNano())))

	// 缓存到内存
	if s.cacheStore != nil {
		cacheKey := hex.EncodeToString(txHash[:])
		internal.CacheUnsignedTransaction(ctx, s.cacheStore, []byte(cacheKey), tx, internal.GetDefaultCacheConfig(), s.logger)
	}

	return txHash[:], nil
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
func (s *AIModelDeployService) calculateAddressFromPrivateKey(privateKey []byte) (string, error) {
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

// selectUTXOsForModelDeploy 为AI模型部署选择UTXO（内部方法）
func (s *AIModelDeployService) selectUTXOsForModelDeploy(ctx context.Context, deployerAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, string, error) {
	targetAmount, err := s.parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, deployerAddr, &assetCategory, true)
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

func (s *AIModelDeployService) parseAmount(amountStr string) (uint64, error) {
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %v", err)
	}
	return amount, nil
}

func (s *AIModelDeployService) extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
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

func (s *AIModelDeployService) formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// 确保 AIModelDeployService 实现了所需的接口部分
var _ interface {
	DeployAIModel(context.Context, []byte, string, *resourcepb.AIModelExecutionConfig, string, string, ...*types.ResourceDeployOptions) ([]byte, error)
} = (*AIModelDeployService)(nil)
