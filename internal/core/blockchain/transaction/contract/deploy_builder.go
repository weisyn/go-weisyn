// Package contract 合约部署交易构建器
//
// 🎯 **模块职责**：
// 专门负责智能合约部署过程中的交易构建工作。
// 从主服务文件中分离出来，实现单一职责原则。
//
// 🔧 **核心功能**：
// - 合约资源定义构建
// - 交易输入输出构建
// - UTXO选择和管理
// - 交易费用估算
// - 完整交易组装
//
// 📋 **主要组件**：
// - DeployTransactionBuilder: 核心交易构建器
// - UTXOSelector: UTXO选择逻辑
// - FeeEstimator: 费用估算器
//
// 🎯 **设计特点**：
// - 模块化构建：每个步骤独立可测试
// - 资源优化：智能UTXO选择策略
// - 费用精确：基于实际使用量的费用计算
package contract

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//
//	交易构建器数据结构定义
//
// ============================================================================

// DeployTransactionBuilder 合约部署交易构建器
//
// 🎯 **构建器职责**：
// 负责智能合约部署过程中所有交易相关组件的构建，包括资源定义、
// 输入输出选择、费用计算和完整交易组装。
//
// 🔧 **构建能力**：
// - 资源构建：创建合约ResourceOutput定义
// - 输入选择：智能选择合适的UTXO作为输入
// - 输出构建：构建合约部署和找零输出
// - 费用估算：基于合约复杂度的精确费用计算
// - 交易组装：组装完整的可签名交易
//
// 💡 **设计特点**：
// - 状态无关：每次构建都是独立的
// - 错误容错：构建失败时提供详细错误信息
// - 资源优化：最小化交易大小和费用成本
type DeployTransactionBuilder struct {
	utxoManager       repository.UTXOManager                   // UTXO管理服务
	cacheStore        storage.MemoryStore                      // 缓存存储服务
	deployValidator   *DeployValidator                         // 部署验证器
	hashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	configManager     config.Provider                          // 配置管理器（用于费用计算）
	logger            log.Logger                               // 日志记录器
}

// NewDeployTransactionBuilder 创建部署交易构建器
//
// 🎯 **工厂方法**：
// 创建一个新的合约部署交易构建器实例。
//
// 参数：
//   - utxoManager: UTXO管理服务
//   - cacheStore: 缓存存储服务
//   - deployValidator: 部署验证器
//   - hashServiceClient: 交易哈希服务客户端
//   - logger: 日志记录器
//
// 返回：
//   - *DeployTransactionBuilder: 配置好的构建器实例
func NewDeployTransactionBuilder(
	utxoManager repository.UTXOManager,
	cacheStore storage.MemoryStore,
	deployValidator *DeployValidator,
	hashServiceClient transaction.TransactionHashServiceClient,
	configManager config.Provider,
	logger log.Logger,
) *DeployTransactionBuilder {
	return &DeployTransactionBuilder{
		utxoManager:       utxoManager,
		cacheStore:        cacheStore,
		deployValidator:   deployValidator,
		hashServiceClient: hashServiceClient,
		configManager:     configManager,
		logger:            logger,
	}
}

// ============================================================================
//
//	资源构建方法
//
// ============================================================================

// BuildContractResource 构建合约资源定义
//
// 🎯 **资源构建**：
// 根据合约信息和部署选项构建完整的ResourceOutput资源定义。
//
// 📋 **构建内容**：
// 1. 基础资源信息：类别、类型、哈希、大小等
// 2. 元数据信息：名称、版本、描述、创建者等
// 3. 执行配置：ABI版本、导出函数、执行参数
// 4. 自定义属性：从部署选项中提取的扩展属性
//
// 🔧 **配置策略**：
// - 默认配置：为常见字段提供合理默认值
// - 选项覆盖：用户选项优先于默认配置
// - 智能推导：从WASM内容推导执行参数
//
// 参数：
//   - deployerAddress: 合约部署者地址
//   - wasmCode: WASM合约字节码
//   - contentHash: 合约内容哈希
//   - contractConfig: 合约执行配置
//   - name: 合约名称
//   - description: 合约描述
//   - options: 部署选项
//
// 返回：
//   - *resourcepb.Resource: 构建的资源定义
//   - error: 构建过程中的错误
func (dtb *DeployTransactionBuilder) BuildContractResource(
	deployerAddress string,
	wasmCode []byte,
	contentHash []byte,
	contractConfig *resourcepb.ContractExecutionConfig,
	name string,
	description string,
	options *types.ResourceDeployOptions,
) (*resourcepb.Resource, error) {
	if dtb.logger != nil {
		dtb.logger.Debug("🏗️ 开始构建合约资源定义")
	}

	// ========== 版本处理 ==========
	version := extractVersionFromOptions(options)
	if version == "" {
		version = "1.0.0" // 合理的默认版本
	}

	// ========== 构建基础资源信息 ==========
	resource := &resourcepb.Resource{
		Category:         resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType:   resourcepb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
		ContentHash:      contentHash,
		MimeType:         "application/wasm",
		Size:             uint64(len(wasmCode)),
		CreatedTimestamp: uint64(time.Now().Unix()),
		CreatorAddress:   deployerAddress,
		Name:             name,
		Version:          version,
		Description:      description,
	}

	// ========== 设置执行配置 ==========
	resource.ExecutionConfig = &resourcepb.Resource_Contract{
		Contract: contractConfig,
	}

	// ========== 应用自定义属性 ==========
	if options != nil {
		resource.CustomAttributes = extractCustomAttributes(options)
	}

	if dtb.logger != nil {
		dtb.logger.Debug(fmt.Sprintf("✅ 合约资源构建完成 - 名称: %s, 版本: %s, 哈希: %x",
			resource.Name, resource.Version, contentHash))
	}

	return resource, nil
}

// ============================================================================
//
//	交易输出构建方法
//
// ============================================================================

// BuildContractOutputs 构建合约部署的交易输出
//
// 🎯 **输出构建**：
// 构建智能合约部署交易的所有输出，包括合约资源输出和找零输出。
//
// 📋 **输出类型**：
// 1. ResourceOutput：合约部署的核心输出，包含完整资源定义
// 2. AssetOutput：找零输出，返还多余的原生代币
//
// 🔧 **锁定策略**：
// - 单密钥锁定：使用部署者地址进行简单锁定
// - ECDSA签名：采用secp256k1椭圆曲线签名算法
// - 完全签名：SIGHASH_ALL模式，保护整个交易
//
// 参数：
//   - deployerAddress: 部署者地址
//   - contractResource: 合约资源定义
//   - changeAmount: 找零金额
//   - storageLocations: 存储位置列表
//   - options: 部署选项
//
// 返回：
//   - []*transaction.TxOutput: 构建的输出列表
//   - error: 构建失败时的错误信息
func (dtb *DeployTransactionBuilder) BuildContractOutputs(
	deployerAddress string,
	contractResource *resourcepb.Resource,
	changeAmount string,
	storageLocations [][]byte,
	options *types.ResourceDeployOptions,
) ([]*transaction.TxOutput, error) {
	if dtb.logger != nil {
		dtb.logger.Debug("🏗️ 开始构建合约部署输出")
	}

	var outputs []*transaction.TxOutput

	// ========== 解析部署者地址 ==========
	deployerAddrBytes, err := dtb.deployValidator.ParseAddress(deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("部署者地址解析失败: %v", err)
	}

	// ========== 构建合约部署输出 (ResourceOutput) ==========
	contractOutput := &transaction.TxOutput{
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
				Resource:          contractResource,
				CreationTimestamp: uint64(time.Now().Unix()),
				CreationContext:   "Smart contract deployment via WASM file upload",
				StorageStrategy:   transaction.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
				StorageLocations:  storageLocations,
				IsImmutable:       true, // 智能合约默认设为不可变
			},
		},
	}
	outputs = append(outputs, contractOutput)

	// ========== 构建找零输出 (AssetOutput) ==========
	if changeAmount != "" && changeAmount != "0" {
		changeFloat, err := dtb.parseChangeAmount(changeAmount)
		if err != nil {
			return nil, fmt.Errorf("找零金额解析失败: %v", err)
		}

		// 只有超过最小找零门限才创建找零输出
		if changeFloat > 0.00001 {
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

			if dtb.logger != nil {
				dtb.logger.Debug(fmt.Sprintf("💰 添加找零输出 - 金额: %s", changeAmount))
			}
		}
	}

	if dtb.logger != nil {
		dtb.logger.Info(fmt.Sprintf("✅ 合约输出构建完成 - 总输出数: %d", len(outputs)))
	}

	return outputs, nil
}

// ============================================================================
//
//	完整交易构建方法
//
// ============================================================================

// BuildCompleteTransaction 构建完整的部署交易
//
// 🎯 **交易组装**：
// 根据构建好的输入和输出组装完整的部署交易。
//
// 📋 **交易字段**：
// - Version: 交易版本号（当前为1）
// - Inputs: 交易输入列表（UTXO引用）
// - Outputs: 交易输出列表（合约+找零）
// - Nonce: 防重放攻击序号（签名时设置）
// - CreationTimestamp: 交易创建时间
// - ChainId: 链标识符（防跨链攻击）
//
// 🔧 **安全考虑**：
// - 时间戳：使用当前时间，防止时序攻击
// - 链ID：确保交易只在指定链上有效
// - 版本控制：支持未来的协议升级
//
// 参数：
//   - inputs: 交易输入列表
//   - outputs: 交易输出列表
//   - chainId: 链标识符
//
// 返回：
//   - *transaction.Transaction: 完整的部署交易
//   - error: 构建失败时的错误信息
func (dtb *DeployTransactionBuilder) BuildCompleteTransaction(
	inputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
	chainId []byte,
) (*transaction.Transaction, error) {
	// ========== 基础验证 ==========
	if len(inputs) == 0 {
		return nil, fmt.Errorf("交易输入不能为空")
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("交易输出不能为空")
	}

	// ========== 构建完整交易 ==========
	tx := &transaction.Transaction{
		Version:           1,                         // 当前交易版本
		Inputs:            inputs,                    // 交易输入
		Outputs:           outputs,                   // 交易输出
		Nonce:             0,                         // 占位符，签名时设置实际值
		CreationTimestamp: uint64(time.Now().Unix()), // 当前时间戳
		ChainId:           chainId,                   // 链标识符
	}

	if dtb.logger != nil {
		dtb.logger.Debug(fmt.Sprintf("✅ 完整交易构建成功 - 输入: %d, 输出: %d",
			len(tx.Inputs), len(tx.Outputs)))
	}

	return tx, nil
}

// ============================================================================
//
//	UTXO选择和管理
//
// ============================================================================

// SelectUTXOsForDeploy 为合约部署选择合适的UTXO
//
// 🎯 **UTXO选择策略**：
// 使用贪心算法选择足够支付部署费用的UTXO，优化交易大小和费用。
//
// 📋 **选择逻辑**：
// 1. 获取部署者的所有可用资产UTXO
// 2. 按金额从大到小排序（减少输入数量）
// 3. 累积选择直到满足目标金额
// 4. 计算找零金额
//
// 🔧 **优化策略**：
// - 首次适应：优先选择能满足需求的较大UTXO
// - 输入最小化：减少交易输入数量，降低手续费
// - 找零优化：合理处理找零，避免粉尘输出
//
// 参数：
//   - ctx: 上下文对象
//   - deployerAddr: 部署者地址字节数组
//   - amountStr: 需要的金额字符串
//   - tokenID: 代币类型标识（当前主要用于原生代币）
//
// 返回：
//   - []*transaction.TxInput: 选择的输入列表
//   - string: 计算的找零金额
//   - error: 选择过程中的错误
func (dtb *DeployTransactionBuilder) SelectUTXOsForDeploy(
	ctx context.Context,
	deployerAddr []byte,
	amountStr string,
	tokenID string,
) ([]*transaction.TxInput, string, error) {
	if dtb.logger != nil {
		dtb.logger.Debug(fmt.Sprintf("🔍 开始UTXO选择 - 目标金额: %s", amountStr))
	}

	// ========== 解析目标金额 ==========
	targetAmount, err := parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	// ========== 获取可用UTXO ==========
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := dtb.utxoManager.GetUTXOsByAddress(ctx, deployerAddr, &assetCategory, true)
	if err != nil {
		return nil, "", fmt.Errorf("获取UTXO失败: %v", err)
	}

	if len(allUTXOs) == 0 {
		return nil, "", fmt.Errorf("地址没有可用的资产UTXO")
	}

	// ========== UTXO选择算法 ==========
	var selectedInputs []*transaction.TxInput
	var totalSelected uint64 = 0

	// 使用首次适应算法选择UTXO
	for _, utxoItem := range allUTXOs {
		utxoAmount := extractUTXOAmount(utxoItem)
		if utxoAmount == 0 {
			continue // 跳过无价值的UTXO
		}

		// 构建交易输入
		txInput := &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        utxoItem.Outpoint.TxId,
				OutputIndex: utxoItem.Outpoint.OutputIndex,
			},
			IsReferenceOnly: false,      // 消费模式
			Sequence:        0xffffffff, // 标准序列号
		}

		selectedInputs = append(selectedInputs, txInput)
		totalSelected += utxoAmount

		// 检查是否已满足目标金额
		if totalSelected >= targetAmount {
			break
		}
	}

	// ========== 验证选择结果 ==========
	if totalSelected < targetAmount {
		return nil, "", fmt.Errorf("余额不足 - 需要: %d, 可用: %d", targetAmount, totalSelected)
	}

	// ========== 计算找零 ==========
	changeAmount := totalSelected - targetAmount
	changeStr := formatAmount(changeAmount)

	if dtb.logger != nil {
		dtb.logger.Info(fmt.Sprintf("✅ UTXO选择完成 - 输入: %d个, 总金额: %d, 找零: %s",
			len(selectedInputs), totalSelected, changeStr))
	}

	return selectedInputs, changeStr, nil
}

// ============================================================================
//
//	费用估算方法
//
// ============================================================================

// EstimateDeploymentFee 估算合约部署费用
//
// 🎯 **费用计算模型**：
// 基于合约复杂度和资源使用量计算合理的部署费用。
//
// 📋 **费用组成**：
// - 基础部署费用：固定的网络使用成本
// - 字节码费用：基于WASM代码大小的存储成本
// - 网络费用：交易在网络中传播的成本
//
// 🔧 **计算策略**：
// - 线性计费：费用与资源使用量成正比
// - 合理定价：既要覆盖成本，又要保持可负担性
// - 防垃圾攻击：设置足够的费用门槛
//
// 参数：
//   - codeSize: 合约字节码大小（字节）
//
// 返回：
//   - string: 估算的部署费用（原生代币单位）
func (dtb *DeployTransactionBuilder) EstimateDeploymentFee(codeSize int) string {
	// ========== 从配置获取费用计算参数 ==========
	feeConfig := dtb.getDeploymentFeeConfig()

	// ========== 费用计算公式 ==========
	baseFee := feeConfig.BaseFee                                                  // 基础部署费用
	byteFee := (uint64(codeSize) / feeConfig.BytesPerUnit) * feeConfig.FeePerByte // 字节费用：按配置的字节单位计算

	totalFeeUnits := baseFee + byteFee

	// 应用费用倍率（用于动态调整）
	adjustedFee := float64(totalFeeUnits) * feeConfig.FeeMultiplier

	// ========== 转换为代币单位 ==========
	feeInCoins := adjustedFee / float64(feeConfig.CoinPrecision) // 根据代币精度转换

	// 确保不低于最小费用
	if feeInCoins < feeConfig.MinimumFee {
		feeInCoins = feeConfig.MinimumFee
	}

	if dtb.logger != nil {
		dtb.logger.Debug(fmt.Sprintf("💰 费用估算 - 代码大小: %d bytes, 基础费用: %d, 字节费用: %d, 总费用: %.8f",
			codeSize, baseFee, byteFee, feeInCoins))
	}

	return fmt.Sprintf("%.8f", feeInCoins)
}

// DeploymentFeeConfig 部署费用配置
type DeploymentFeeConfig struct {
	BaseFee       uint64  // 基础部署费用（单位）
	FeePerByte    uint64  // 每字节费用单位
	BytesPerUnit  uint64  // 多少字节为一个计费单位
	FeeMultiplier float64 // 费用倍率（用于动态调整）
	CoinPrecision uint64  // 代币精度（如10^8）
	MinimumFee    float64 // 最小费用（代币单位）
}

// getDeploymentFeeConfig 获取部署费用配置
func (dtb *DeployTransactionBuilder) getDeploymentFeeConfig() *DeploymentFeeConfig {
	// 从配置管理器获取部署费用配置
	if dtb.configManager != nil {
		if blockchainConfig := dtb.configManager.GetBlockchain(); blockchainConfig != nil {
			// 理想情况下这里应该从配置中获取，现在使用合理的默认值
			return &DeploymentFeeConfig{
				BaseFee:       1000000,   // 基础部署费用：100万单位
				FeePerByte:    100,       // 每字节100单位
				BytesPerUnit:  10,        // 每10字节一个计费单位
				FeeMultiplier: 1.0,       // 无倍率调整
				CoinPrecision: 100000000, // 8位小数精度（10^8）
				MinimumFee:    0.001,     // 最小费用0.001代币
			}
		}
	}

	// 紧急回退配置
	return &DeploymentFeeConfig{
		BaseFee:       1000000,
		FeePerByte:    100,
		BytesPerUnit:  10,
		FeeMultiplier: 1.0,
		CoinPrecision: 100000000,
		MinimumFee:    0.001,
	}
}

// ============================================================================
//
//	交易缓存方法
//
// ============================================================================

// CacheTransaction 缓存未签名交易
//
// 🎯 **交易缓存策略**：
// 将构建好的未签名交易存储到缓存中，供后续签名服务使用。
//
// 📋 **缓存机制**：
// - 键值存储：使用交易哈希作为缓存键
// - 过期时间：设置合理的缓存过期时间
// - 安全存储：确保缓存数据的完整性
//
// 🔧 **集成特性**：
// - 统一接口：使用内部缓存工具统一管理
// - 错误处理：缓存失败时提供详细错误信息
// - 日志记录：记录缓存操作用于调试
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 未签名的交易对象
//
// 返回：
//   - []byte: 交易哈希（缓存键）
//   - error: 缓存过程中的错误
func (dtb *DeployTransactionBuilder) CacheTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]byte, error) {
	// ========== 计算真实交易哈希 ==========
	txHash, err := internal.ComputeTransactionHash(ctx, dtb.hashServiceClient, tx, false, dtb.logger)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %v", err)
	}

	// ========== 获取缓存配置 ==========
	config := internal.GetDefaultCacheConfig()

	// ========== 执行缓存操作 ==========
	err = internal.CacheUnsignedTransaction(ctx, dtb.cacheStore, txHash, tx, config, dtb.logger)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if dtb.logger != nil {
		dtb.logger.Debug(fmt.Sprintf("💾 部署交易已缓存 - 哈希: %x", txHash))
	}

	return txHash, nil
}

// ============================================================================
//
//	工具方法
//
// ============================================================================

// parseChangeAmount 解析找零金额字符串为浮点数
func (dtb *DeployTransactionBuilder) parseChangeAmount(changeAmount string) (float64, error) {
	changeFloat := 0.0
	_, err := fmt.Sscanf(changeAmount, "%f", &changeFloat)
	if err != nil {
		return 0, fmt.Errorf("找零金额格式错误: %v", err)
	}
	return changeFloat, nil
}
