// Package contract 智能合约调用实现
//
// 🎯 **模块定位**：ContractService 接口的智能合约调用功能实现
//
// 本文件实现智能合约调用的核心业务逻辑，包括：
// - WASM智能合约方法调用（CallContract）
// - 合约状态管理和转换
// - 执行费用计算
// - 合约执行结果处理
// - 状态输出和证明生成
//
// 🏗️ **架构定位**：
// - 业务层：实现智能合约的调用业务逻辑
// - 执行层：与 WASM 执行引擎的深度集成
// - 状态层：管理合约状态的读取和更新
// - 证明层：生成执行结果的零知识证明
//
// 🔧 **设计原则**：
// - 确定性执行：相同输入产生相同输出
// - 状态隔离：合约间状态完全隔离
// - 执行计量：精确的资源消耗计算
// - 错误透明：详细的执行错误信息
// - 证明生成：可验证的执行结果证明
//
// 📋 **支持的调用模式**：
// - 只读调用：不改变合约状态，无需创建交易
// - 状态变更调用：修改合约状态，需要交易上链
// - 跨合约调用：支持合约间的相互调用
// - 批量调用：一个交易中执行多个合约方法
//
// 🎯 **执行结果处理**：
// - 成功执行：创建 StateOutput 记录执行结果
// - 执行失败：回滚状态变更，返回错误信息
// - 执行时间耗尽：终止执行，消耗已用执行时间
// - 异常处理：捕获运行时异常，保护系统安全
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package contract

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 类型定义
	"github.com/weisyn/v1/pkg/types"
	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//
//	智能合约调用实现服务
//
// ============================================================================
// ContractCallService 智能合约调用核心实现服务
//
// 🎯 **服务职责**：
// - 实现 ContractService.CallContract 方法
// - 处理 WASM 智能合约的方法调用和执行
// - 管理合约状态的读取、更新和证明
// - 计算和验证 执行时间消耗和执行费用
//
// 🔧 **依赖注入**：
// - contractExecutor：WASM 合约执行引擎
// - stateManager：合约状态管理服务
// - feeCalculator：执行计量和费用计算服务
// - utxoSelector：UTXO 选择和管理服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewContractCallService(executor, stateManager, feeCalc, utxoSelector, cache, logger)
//	txHash, err := service.CallContract(ctx, caller, contractAddr, method, args)
type ContractCallService struct {
	// 核心依赖服务（使用公共接口）
	utxoManager                  repository.UTXOManager                     // UTXO 管理服务
	signatureManager             crypto.SignatureManager                    // 数字签名服务
	hashManager                  crypto.HashManager                         // 哈希计算服务
	keyManager                   crypto.KeyManager                          // 密钥管理服务（用于从私钥生成公钥）
	addressManager               crypto.AddressManager                      // 地址管理服务（用于从公钥生成地址）
	transactionHashServiceClient transactionpb.TransactionHashServiceClient // 统一交易哈希服务
	cacheStore                   storage.MemoryStore                        // 内存缓存存储
	logger                       log.Logger                                 // 日志记录器

	// 🎯 执行层依赖（新增）
	engineManager          execution.EngineManager          // 执行引擎管理器
	hostCapabilityRegistry execution.HostCapabilityRegistry // 宿主能力注册器
	executionCoordinator   execution.ExecutionCoordinator   // 执行协调器
	configManager          config.Provider                  // 配置管理器

	// 内部状态
	hostInterface execution.HostStandardInterface // 标准宿主接口（初始化后设置）
}

// NewContractCallService 创建智能合约调用服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - contractExecutor: WASM 合约执行引擎
//   - stateManager: 合约状态管理服务
//   - feeCalculator: 执行计量和费用计算服务
//   - utxoSelector: UTXO 选择和管理服务
//   - cacheStore: 交易缓存存储服务
//   - logger: 日志记录器
//
// 返回：
//   - *ContractCallService: 合约调用服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewContractCallService(
	utxoManager repository.UTXOManager,
	signatureManager crypto.SignatureManager,
	hashManager crypto.HashManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	transactionHashServiceClient transactionpb.TransactionHashServiceClient,
	cacheStore storage.MemoryStore,
	engineManager execution.EngineManager,
	hostCapabilityRegistry execution.HostCapabilityRegistry,
	executionCoordinator execution.ExecutionCoordinator,
	configManager config.Provider,
	logger log.Logger,
) *ContractCallService {
	// 严格的依赖检查
	if logger == nil {
		panic("ContractCallService: logger不能为nil")
	}
	if utxoManager == nil {
		logger.Warn("ContractCallService: utxoManager为nil，某些功能将不可用")
	}
	if keyManager == nil {
		panic("ContractCallService: keyManager不能为nil")
	}
	if addressManager == nil {
		panic("ContractCallService: addressManager不能为nil")
	}
	if transactionHashServiceClient == nil {
		panic("ContractCallService: transactionHashServiceClient不能为nil")
	}
	if cacheStore == nil {
		logger.Warn("ContractCallService: cacheStore为nil，某些功能将不可用")
	}
	// 🎯 execution接口依赖检查
	if engineManager == nil {
		panic("ContractCallService: engineManager不能为nil")
	}
	if hostCapabilityRegistry == nil {
		panic("ContractCallService: hostCapabilityRegistry不能为nil")
	}
	if executionCoordinator == nil {
		panic("ContractCallService: executionCoordinator不能为nil")
	}
	if configManager == nil {
		panic("ContractCallService: configManager不能为nil")
	}

	// 🎯 构建标准宿主接口
	hostInterface := hostCapabilityRegistry.BuildStandardInterface()

	return &ContractCallService{
		utxoManager:                  utxoManager,
		signatureManager:             signatureManager,
		hashManager:                  hashManager,
		keyManager:                   keyManager,
		addressManager:               addressManager,
		transactionHashServiceClient: transactionHashServiceClient,
		cacheStore:                   cacheStore,
		logger:                       logger,
		// 新增execution接口依赖
		engineManager:          engineManager,
		hostCapabilityRegistry: hostCapabilityRegistry,
		executionCoordinator:   executionCoordinator,
		configManager:          configManager,
		hostInterface:          hostInterface,
	}
}

// ============================================================================
//
//	核心合约调用方法实现
//
// ============================================================================
// CallContract 实现智能合约调用功能（薄实现）
//
// 🎯 **方法职责**：
// 实现 blockchain.ContractService.CallContract 接口
// 支持 WASM 智能合约的方法调用和状态管理
//
// 📋 **业务流程**：
// 1. 验证合约调用参数的有效性
// 2. 解析合约地址和方法名称
// 3. 加载合约字节码和当前状态
// 4. 验证调用者的权限和 执行费用余额
// 5. 执行合约方法并监控 执行时间消耗
// 6. 处理执行结果和状态变更
// 7. 生成状态转换证明（如需要）
// 8. 构建包含 StateOutput 的调用交易
// 9. 选择支付 执行费用的 UTXO
// 10. 将调用交易存储到内存缓存
// 11. 返回交易哈希供用户签名
//
// 📝 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//   - callerAddress: 合约调用者地址
//   - contractAddress: 目标合约地址
//   - methodName: 要调用的合约方法名
//   - methodArgs: 方法调用参数（JSON或二进制格式）
//
// 📤 **返回值**：
//   - []byte: 交易哈希，用于后续签名和提交
//   - error: 错误信息，调用失败时返回具体原因
//
// 🎯 **支持场景**：
// - DeFi操作：CallContract(ctx, user, dexContract, "swap", swapArgs)
// - 代币转账：CallContract(ctx, user, tokenContract, "transfer", transferArgs)
// - 治理投票：CallContract(ctx, voter, govContract, "vote", voteArgs)
// - 状态查询：CallContract(ctx, user, contract, "getBalance", queryArgs)
//
// 💡 **执行特性**：
// - 执行计量：精确计算和控制资源消耗
// - 状态隔离：确保合约间状态独立性
// - 异常安全：捕获执行异常，保护系统稳定性
// - 结果证明：生成可验证的执行证明
//
// ⚠️ **当前状态**：薄实现，返回未实现错误
func (s *ContractCallService) CallContract(
	ctx context.Context,
	callerPrivateKey []byte,
	contractAddress string,
	methodName string,
	parameters map[string]interface{},
	executionTimeLimit uint64,
	value string,
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 从私钥计算调用者地址（无状态设计）
	callerAddress, err := s.calculateAddressFromPrivateKey(callerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理智能合约调用请求 - caller: %s, contract: %s, method: %s",
			callerAddress, contractAddress, methodName))
	}

	// 🔄 步骤1: 基础参数验证
	if err := s.validateCallParams(contractAddress, methodName, parameters, executionTimeLimit, value, options); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 🏗️ 步骤2: 序列化方法参数
	methodArgsBytes, err := s.serializeParameters(parameters)
	if err != nil {
		return nil, fmt.Errorf("方法参数序列化失败: %v", err)
	}

	// 🎯 步骤3: 构建ExecutionParams（新增 - 使用真实execution接口）
	executionParams, err := s.buildExecutionParams(contractAddress, methodName, methodArgsBytes, executionTimeLimit, callerAddress)
	if err != nil {
		return nil, fmt.Errorf("构建执行参数失败: %v", err)
	}

	// 🔧 步骤4: 通过EngineManager执行合约（新增 - 真实执行）
	executionResult, err := s.engineManager.Execute(types.EngineTypeWASM, *executionParams)
	if err != nil {
		return nil, fmt.Errorf("合约执行失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🎯 合约执行完成 - success: %v, executionTimeUsed: %d",
			executionResult.Success, executionResult.Consumed))
	}

	// 🏗️ 步骤5: 处理执行结果，构建交易
	tx, err := s.buildTransactionFromExecutionResult(
		ctx,
		callerPrivateKey,
		contractAddress,
		methodName,
		executionParams,
		executionResult,
		value,
		options,
	)
	if err != nil {
		return nil, fmt.Errorf("根据执行结果构建交易失败: %v", err)
	}

	// 🔄 步骤6: 计算交易哈希并缓存
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 智能合约调用完成 - txHash: %x, contract: %s, method: %s, success: %v, executionTimeUsed: %d",
			txHash, contractAddress, methodName, executionResult.Success, executionResult.Consumed))
	}

	return txHash, nil
}

// ============================================================================
//
//	🎯 新增：ExecutionParams构建和ExecutionResult处理
//
// ============================================================================

// buildExecutionParams 构建标准化的ExecutionParams
//
// 🎯 **方法职责**：
// 将合约调用参数转换为标准的types.ExecutionParams结构
//
// 参数：
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - methodArgs: 方法参数（已序列化）
//   - executionTimeLimit: 执行时间限制
//   - callerAddress: 调用者地址
//
// 返回：
//   - *types.ExecutionParams: 标准化的执行参数
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildExecutionParams(
	contractAddress string,
	methodName string,
	methodArgs []byte,
	executionTimeLimit uint64,
	callerAddress string,
) (*types.ExecutionParams, error) {
	// 构建执行上下文
	executionContext := make(map[string]any)
	executionContext["caller"] = callerAddress
	executionContext["contract"] = contractAddress

	// 获取链ID和区块高度等上下文信息
	if s.configManager != nil {
		if chainConfig := s.configManager.GetBlockchain(); chainConfig != nil {
			executionContext["chain_id"] = chainConfig.ChainID
		}
	}
	executionContext["block_timestamp"] = time.Now().Unix()

	// 构建ExecutionParams
	params := &types.ExecutionParams{
		ResourceID:        []byte(contractAddress), // 合约地址作为资源ID
		Entry:             methodName,              // 方法名作为入口点
		Payload:           methodArgs,              // 方法参数
		Context:           executionContext,        // 执行上下文
		ExecutionFeeLimit: executionTimeLimit,      // 执行时间限制
		MemoryLimit:       16 * 1024 * 1024,        // 16MB内存限制
		Timeout:           30,                      // 30秒超时
		Caller:            callerAddress,           // 调用者地址
		ContractAddr:      contractAddress,         // 合约地址
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 构建ExecutionParams - contract: %s, method: %s, executionTimeLimit: %d",
			contractAddress, methodName, executionTimeLimit))
	}

	return params, nil
}

// buildTransactionFromExecutionResult 根据执行结果构建交易
//
// 🎯 **方法职责**：
// 将execution接口的执行结果转换为区块链交易
//
// 参数：
//   - ctx: 上下文
//   - callerPrivateKey: 调用者私钥
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - executionParams: 执行参数
//   - executionResult: 执行结果
//   - value: 转账金额
//   - options: 调用选项
//
// 返回：
//   - *transaction.Transaction: 构建的交易
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildTransactionFromExecutionResult(
	ctx context.Context,
	callerPrivateKey []byte,
	contractAddress string,
	methodName string,
	executionParams *types.ExecutionParams,
	executionResult *types.ExecutionResult,
	value string,
	options []*types.TransferOptions,
) (*transaction.Transaction, error) {
	// 📍 解析调用者地址
	callerAddress, err := s.calculateAddressFromPrivateKey(callerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("计算调用者地址失败: %v", err)
	}

	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("解析调用者地址失败: %v", err)
	}

	// 💰 计算总费用需求（基础费用 + 实际消耗的执行费用 + 转账金额）
	actualExecutionTimeUsed := executionResult.Consumed
	if actualExecutionTimeUsed > executionParams.ExecutionFeeLimit {
		actualExecutionTimeUsed = executionParams.ExecutionFeeLimit // 不能超过限制
	}

	totalRequiredAmount, err := s.calculateTotalCostWithActualGas(actualExecutionTimeUsed, value)
	if err != nil {
		return nil, fmt.Errorf("计算实际费用失败: %v", err)
	}

	// 💰 选择支付费用的UTXO
	selectedInputs, changeAmount, err := s.selectUTXOsForContract(
		ctx, callerAddrBytes, totalRequiredAmount, "") // 原生代币支付执行费用
	if err != nil {
		return nil, fmt.Errorf("选择UTXO失败: %v", err)
	}

	// 🔧 合并调用选项
	mergedOptions, err := s.mergeCallOptions(options)
	if err != nil {
		return nil, fmt.Errorf("合并调用选项失败: %v", err)
	}

	// 🏗️ 构建输出（包含真实的执行结果）
	outputs, err := s.buildCallOutputsWithExecutionResult(
		contractAddress,
		methodName,
		executionParams,
		executionResult,
		actualExecutionTimeUsed,
		value,
		changeAmount,
		callerAddress,
		mergedOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("构建输出失败: %v", err)
	}

	// 🔄 构建完整交易
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	return tx, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// validateContractAddress 验证合约地址格式
//
// 🔍 **验证项目**：
// - 地址长度和格式检查
// - 校验和验证
// - 合约存在性检查
//
// 参数：
//   - contractAddress: 合约地址
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *ContractCallService) validateContractAddress(contractAddress string) error {
	if len(contractAddress) == 0 {
		return fmt.Errorf("合约地址不能为空")
	}
	if s.logger != nil {
		s.logger.Debug("验证合约地址格式")
	}
	// 🔍 完整地址格式验证
	// 基本地址长度检查 (WES地址通常为34-62字符)
	if len(contractAddress) < 34 || len(contractAddress) > 62 {
		return fmt.Errorf("合约地址长度无效: %d (期望: 34-62字符)", len(contractAddress))
	}

	// 使用addressManager进行完整验证
	if s.addressManager != nil {
		isValid, err := s.addressManager.ValidateAddress(contractAddress)
		if err != nil {
			return fmt.Errorf("地址验证失败: %v", err)
		}
		if !isValid {
			return fmt.Errorf("无效的合约地址: %s", contractAddress)
		}
	}
	return nil
}

// validateMethodName 验证合约方法名
//
// 🔍 **验证项目**：
// - 方法名长度和字符检查
// - 保留字检查
// - 特殊字符过滤
//
// 参数：
//   - methodName: 方法名
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *ContractCallService) validateMethodName(methodName string) error {
	if len(methodName) == 0 {
		return fmt.Errorf("方法名不能为空")
	}
	if len(methodName) > maxMethodNameLength() {
		return fmt.Errorf("方法名长度超过限制，最大支持 %d 字符", maxMethodNameLength())
	}
	if s.logger != nil {
		s.logger.Debug("验证合约方法名")
	}
	// 🔍 完整的方法名验证
	// 字符集检查：只允许字母、数字、下划线
	for _, char := range methodName {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return fmt.Errorf("方法名包含无效字符: '%c' (只允许字母、数字、下划线)", char)
		}
	}
	// 首字符检查：不能以数字开始
	if len(methodName) > 0 && methodName[0] >= '0' && methodName[0] <= '9' {
		return fmt.Errorf("方法名不能以数字开始: '%s'", methodName)
	}
	// WASM保留字检查
	reservedNames := []string{"_start", "_initialize", "memory", "table", "__wbindgen", "_validate"}
	for _, reserved := range reservedNames {
		if methodName == reserved {
			return fmt.Errorf("方法名'%s'为系统保留名称", methodName)
		}
	}
	return nil
}

// loadContractState 加载合约的当前状态
//
// 🔍 **加载内容**：
// - 合约字节码
// - 合约当前状态数据
// - 合约执行配置
// - 权限控制信息
//
// 参数：
//   - ctx: 上下文对象
//   - contractAddress: 合约地址
//
// 返回：
//   - map[string]interface{}: 合约状态信息
//   - error: 加载失败时的错误信息
func (s *ContractCallService) loadContractState(
	ctx context.Context,
	contractAddress string,
) (map[string]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug("加载合约状态")
	}
	// 🚧 薄实现：委托给状态管理器
	return nil, fmt.Errorf("合约状态加载功能尚未实现，将委托给公共接口实现")
}

// executeContractMethod 执行合约方法
//
// 🚀 **执行过程**：
// - 创建执行上下文和沙箱环境
// - 加载 WASM 模块并初始化
// - 调用指定方法并传递参数
// - 监控 执行时间消耗和执行时间
// - 捕获执行结果和状态变更
//
// 参数：
//   - ctx: 上下文对象
//   - contractState: 合约状态
//   - methodName: 方法名
//   - methodArgs: 方法参数
//   - executionTimeLimit: 执行费用限制
//
// 返回：
//   - map[string]interface{}: 执行结果
//   - error: 执行失败时的错误信息
func (s *ContractCallService) executeContractMethod(
	ctx context.Context,
	contractState map[string]interface{},
	methodName string,
	methodArgs []byte,
	executionTimeLimit uint64,
) (map[string]interface{}, error) {
	if s.logger != nil {
		s.logger.Debug("执行合约方法")
	}
	// 🚧 薄实现：委托给合约执行器
	return nil, fmt.Errorf("合约方法执行功能尚未实现，将委托给公共接口实现")
}

// buildStateOutput 构建状态输出
//
// 🏗️ **输出构建**：
// - 创建 StateOutput 类型
// - 包含执行结果哈希
// - 生成零知识证明（如需要）
// - 设置状态版本和链接
//
// 参数：
//   - executionResult: 合约执行结果
//   - contractAddress: 合约地址
//   - methodName: 执行的方法名
//
// 返回：
//   - *transaction.TxOutput: 构建的状态输出
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildStateOutput(
	executionResult map[string]interface{},
	contractAddress string,
	methodName string,
) (*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("构建状态输出")
	}
	// 🚧 薄实现：状态输出构建逻辑
	return nil, fmt.Errorf("状态输出构建功能尚未实现")
}

// calculateExecutionFeeLimit 计算合约调用的 执行费用限制
//
// 🧮 **计算策略**：
// - 基于历史执行数据估算
// - 考虑方法复杂度
// - 用户指定的 执行费用限制
// - 系统最大限制检查
//
// 参数：
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - methodArgs: 方法参数
//
// 返回：
//   - uint64: 建议的 执行费用限制
//   - error: 计算失败时的错误信息
func (s *ContractCallService) calculateExecutionFeeLimit(
	contractAddress string,
	methodName string,
	methodArgs []byte,
) (uint64, error) {
	if s.logger != nil {
		s.logger.Debug("计算执行时间限制")
	}
	// 🚧 薄实现：委托给 Gas 计算器
	return 0, fmt.Errorf("执行时间限制计算功能尚未实现，将委托给公共接口实现")
}

// maxMethodArgsSize 返回合约方法参数的最大大小
//
// 🎯 **限制原因**：
// - 防止过大参数影响执行性能
// - 控制网络传输和存储成本
// - 保证合理的处理时间
//
// 返回：
//   - int: 最大方法参数大小（字节）
func maxMethodArgsSize() int {
	return 1 * 1024 * 1024 // 1MB，足够支持复杂的方法调用
}

// maxMethodNameLength 返回合约方法名的最大长度
//
// 🎯 **限制原因**：
// - 保证方法名的可读性
// - 防止恶意超长方法名
// - 符合编程语言的命名约定
//
// 返回：
//   - int: 最大方法名长度（字符）
func maxMethodNameLength() int {
	return 64 // 64字符，足够支持描述性的方法名
}

// ============================================================================
//                              新增辅助方法实现
// ============================================================================

// validateCallParams 验证合约调用参数
func (s *ContractCallService) validateCallParams(
	contractAddress string,
	methodName string,
	parameters map[string]interface{},
	executionTimeLimit uint64,
	value string,
	options []*types.TransferOptions,
) error {
	if contractAddress == "" {
		return fmt.Errorf("合约地址不能为空")
	}
	if methodName == "" {
		return fmt.Errorf("合约方法名不能为空")
	}
	if len(methodName) > maxMethodNameLength() {
		return fmt.Errorf("方法名长度超过限制，最大支持 %d 字符", maxMethodNameLength())
	}
	if executionTimeLimit == 0 {
		return fmt.Errorf("执行时间限制不能为0")
	}
	if executionTimeLimit > maxExecutionFeeLimit() {
		return fmt.Errorf("执行时间限制超过系统最大值 %d", maxExecutionFeeLimit())
	}
	// 验证value格式
	if value != "" {
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("转账金额格式错误: %v", err)
		}
	}

	if s.logger != nil {
		s.logger.Debug("✅ 参数验证通过")
	}
	return nil
}

// mergeCallOptions 合并调用选项
func (s *ContractCallService) mergeCallOptions(options []*types.TransferOptions) (*types.TransferOptions, error) {
	if len(options) == 0 {
		// 没有选项时返回nil，这是合法的情况
		return nil, nil
	}

	// 合并多个选项（使用第一个作为基础）
	mergedOptions := options[0]

	// 未来可以在这里实现多个选项的智能合并逻辑
	// 目前简单地使用第一个选项

	if s.logger != nil {
		s.logger.Debug("✅ 调用选项处理完成")
	}

	return mergedOptions, nil
}

// serializeParameters 序列化方法参数
func (s *ContractCallService) serializeParameters(parameters map[string]interface{}) ([]byte, error) {
	if len(parameters) == 0 {
		return []byte("{}"), nil
	}

	paramsBytes, err := json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf("JSON序列化失败: %v", err)
	}

	if len(paramsBytes) > maxMethodArgsSize() {
		return nil, fmt.Errorf("方法参数序列化后超过大小限制 %d 字节", maxMethodArgsSize())
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 参数序列化完成，大小: %d 字节", len(paramsBytes)))
	}

	return paramsBytes, nil
}

// parseAddress 解析地址字符串为字节数组
func (s *ContractCallService) parseAddress(address string) ([]byte, error) {
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

// calculateTotalCost 计算总费用需求
func (s *ContractCallService) calculateTotalCost(executionTimeLimit uint64, value string) (string, error) {
	// 估算执行费用用（简化计算）
	gasPrice := 0.000001 // 1 Gwei = 0.000001 原生代币
	gasCost := float64(executionTimeLimit) * gasPrice

	// 转账金额
	valueAmount := 0.0
	if value != "" && value != "0" {
		var err error
		valueAmount, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("转账金额解析失败: %v", err)
		}
	}

	// 总费用
	totalCost := gasCost + valueAmount
	totalCostStr := fmt.Sprintf("%.8f", totalCost)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💰 总费用计算: 执行费用=%.8f, 转账金额=%.8f, 总计=%.8f",
			gasCost, valueAmount, totalCost))
	}

	return totalCostStr, nil
}

// calculateTotalCostWithActualGas 基于实际Gas消耗计算总费用（新增）
//
// 🎯 **方法职责**：
// 根据execution接口返回的实际Gas消耗计算精确的费用
//
// 参数：
//   - actualExecutionTimeUsed: 实际消耗的Gas
//   - value: 转账金额
//
// 返回：
//   - string: 总费用字符串
//   - error: 计算失败时的错误信息
func (s *ContractCallService) calculateTotalCostWithActualGas(actualExecutionTimeUsed uint64, value string) (string, error) {
	// 精确的执行费用用计算（基于实际消耗）
	gasPrice := 0.000001 // 1 Gwei = 0.000001 原生代币
	gasCost := float64(actualExecutionTimeUsed) * gasPrice

	// 转账金额
	valueAmount := 0.0
	if value != "" && value != "0" {
		var err error
		valueAmount, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("转账金额解析失败: %v", err)
		}
	}

	// 总费用
	totalCost := gasCost + valueAmount
	totalCostStr := fmt.Sprintf("%.8f", totalCost)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💰 精确费用计算: 实际Gas=%d, 执行费用=%.8f, 转账金额=%.8f, 总计=%.8f",
			actualExecutionTimeUsed, gasCost, valueAmount, totalCost))
	}

	return totalCostStr, nil
}

// buildCallOutputs 构建合约调用输出
func (s *ContractCallService) buildCallOutputs(
	contractAddress string,
	methodName string,
	methodArgs []byte,
	executionTimeLimit uint64,
	value string,
	changeAmount string,
	callerAddress string,
	options *types.TransferOptions,
) ([]*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建合约调用输出")
	}

	var outputs []*transaction.TxOutput
	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("调用者地址解析失败: %v", err)
	}

	// 1. 构建合约调用StateOutput（记录执行结果）
	stateOutput, err := s.buildStateOutputForCall(contractAddress, methodName, methodArgs, executionTimeLimit, value, callerAddrBytes)
	if err != nil {
		return nil, fmt.Errorf("构建状态输出失败: %v", err)
	}
	outputs = append(outputs, stateOutput)

	// 2. 构建找零输出（如有需要）
	if changeAmount != "" && changeAmount != "0" {
		changeFloat, err := strconv.ParseFloat(changeAmount, 64)
		if err == nil && changeFloat > 0.00001 {
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
		s.logger.Info(fmt.Sprintf("✅ 合约调用输出构建完成 - 总输出数: %d", len(outputs)))
	}

	return outputs, nil
}

// buildCallOutputsWithExecutionResult 构建包含执行结果的合约调用输出（新增）
//
// 🎯 **方法职责**：
// 根据真实的execution接口执行结果构建交易输出
//
// 参数：
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - executionParams: 执行参数
//   - executionResult: 执行结果
//   - actualExecutionTimeUsed: 实际Gas消耗
//   - value: 转账金额
//   - changeAmount: 找零金额
//   - callerAddress: 调用者地址
//   - options: 调用选项
//
// 返回：
//   - []*transaction.TxOutput: 交易输出列表
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildCallOutputsWithExecutionResult(
	contractAddress string,
	methodName string,
	executionParams *types.ExecutionParams,
	executionResult *types.ExecutionResult,
	actualExecutionTimeUsed uint64,
	value string,
	changeAmount string,
	callerAddress string,
	options *types.TransferOptions,
) ([]*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🎯 构建包含执行结果的合约调用输出")
	}

	var outputs []*transaction.TxOutput
	callerAddrBytes, err := s.parseAddress(callerAddress)
	if err != nil {
		return nil, fmt.Errorf("调用者地址解析失败: %v", err)
	}

	// 1. 构建合约执行StateOutput（包含真实执行结果）
	stateOutput, err := s.buildStateOutputWithExecutionResult(
		contractAddress,
		methodName,
		executionParams,
		executionResult,
		actualExecutionTimeUsed,
		callerAddrBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("构建执行结果状态输出失败: %v", err)
	}
	outputs = append(outputs, stateOutput)

	// 2. 如果执行失败，构建错误状态记录
	if !executionResult.Success {
		errorMessage := "执行失败"
		if errorInfo, exists := executionResult.Metadata["error"]; exists {
			if errorStr, ok := errorInfo.(string); ok {
				errorMessage = errorStr
			}
		}
		errorOutput, err := s.buildErrorStateOutput(contractAddress, methodName, errorMessage, callerAddrBytes)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("构建错误状态输出失败: %v", err))
		} else {
			outputs = append(outputs, errorOutput)
		}
	}

	// 3. 构建找零输出（如有需要）
	if changeAmount != "" && changeAmount != "0" {
		changeFloat, err := strconv.ParseFloat(changeAmount, 64)
		if err == nil && changeFloat > 0.00001 {
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
		s.logger.Info(fmt.Sprintf("✅ 包含执行结果的合约调用输出构建完成 - 总输出数: %d, 执行成功: %v, Gas消耗: %d",
			len(outputs), executionResult.Success, actualExecutionTimeUsed))
	}

	return outputs, nil
}

// buildStateOutputWithExecutionResult 构建包含真实执行结果的状态输出（新增）
//
// 🎯 **方法职责**：
// 将execution接口的ExecutionResult转换为StateOutput
//
// 参数：
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - executionParams: 执行参数
//   - executionResult: 执行结果
//   - actualExecutionTimeUsed: 实际Gas消耗
//   - callerAddrBytes: 调用者地址字节
//
// 返回：
//   - *transaction.TxOutput: StateOutput包装的交易输出
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildStateOutputWithExecutionResult(
	contractAddress string,
	methodName string,
	executionParams *types.ExecutionParams,
	executionResult *types.ExecutionResult,
	actualExecutionTimeUsed uint64,
	callerAddrBytes []byte,
) (*transaction.TxOutput, error) {
	// 生成状态ID（基于合约地址+方法名+时间戳）
	stateID := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", contractAddress, methodName, time.Now().UnixNano())))

	// 计算执行结果哈希
	resultHash := sha256.Sum256(executionResult.ReturnData)

	// 构建执行结果元数据
	metadata := map[string]string{
		"contract":  contractAddress,
		"method":    methodName,
		"success":   fmt.Sprintf("%v", executionResult.Success),
		"gas_used":  fmt.Sprintf("%d", actualExecutionTimeUsed),
		"gas_limit": fmt.Sprintf("%d", executionParams.ExecutionFeeLimit),
		"caller":    executionParams.Caller,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	// 如果执行失败，添加错误信息
	if !executionResult.Success {
		if errorInfo, exists := executionResult.Metadata["error"]; exists {
			if errorStr, ok := errorInfo.(string); ok && errorStr != "" {
				metadata["error"] = errorStr
			}
		}
	}

	// 构建StateOutput
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
				StateId:             stateID[:],
				StateVersion:        1,
				ExecutionResultHash: resultHash[:],
				Metadata:            metadata,
			},
		},
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🎯 构建执行结果StateOutput - contract: %s, method: %s, success: %v, executionTimeUsed: %d",
			contractAddress, methodName, executionResult.Success, actualExecutionTimeUsed))
	}

	return stateOutput, nil
}

// buildErrorStateOutput 构建错误状态输出（新增）
//
// 🎯 **方法职责**：
// 为执行失败的合约调用构建专门的错误状态记录
//
// 参数：
//   - contractAddress: 合约地址
//   - methodName: 方法名
//   - errorMessage: 错误信息
//   - callerAddrBytes: 调用者地址字节
//
// 返回：
//   - *transaction.TxOutput: 错误StateOutput包装的交易输出
//   - error: 构建失败时的错误信息
func (s *ContractCallService) buildErrorStateOutput(
	contractAddress string,
	methodName string,
	errorMessage string,
	callerAddrBytes []byte,
) (*transaction.TxOutput, error) {
	// 生成错误状态ID
	errorStateID := sha256.Sum256([]byte(fmt.Sprintf("ERROR:%s:%s:%d", contractAddress, methodName, time.Now().UnixNano())))

	// 计算错误信息哈希
	errorHash := sha256.Sum256([]byte(errorMessage))

	// 构建错误元数据
	errorMetadata := map[string]string{
		"type":      "execution_error",
		"contract":  contractAddress,
		"method":    methodName,
		"error":     errorMessage,
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	// 构建错误StateOutput
	errorStateOutput := &transaction.TxOutput{
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
				StateId:             errorStateID[:],
				StateVersion:        1,
				ExecutionResultHash: errorHash[:],
				Metadata:            errorMetadata,
			},
		},
	}

	return errorStateOutput, nil
}

// buildStateOutputForCall 构建状态输出（合约调用结果）
func (s *ContractCallService) buildStateOutputForCall(
	contractAddress string,
	methodName string,
	methodArgs []byte,
	executionTimeLimit uint64,
	value string,
	callerAddrBytes []byte,
) (*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建合约调用状态输出")
	}

	// 生成合约调用的状态ID
	stateID := s.generateStateID(contractAddress, methodName, methodArgs)

	// 计算执行结果哈希（将来包含实际执行结果）
	executionResultHash := s.calculateExecutionResultHash(contractAddress, methodName, methodArgs, executionTimeLimit)

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
				StateVersion:        1,                           // 第一次执行
				ZkProof:             &transaction.ZKStateProof{}, // 薄实现：空ZK证明
				ExecutionResultHash: executionResultHash,
				ParentStateHash:     nil, // 无父状态
			},
		},
	}

	return stateOutput, nil
}

// buildCompleteTransaction 构建完整交易
func (s *ContractCallService) buildCompleteTransaction(
	selectedInputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建完整合约调用交易")
	}

	tx := &transaction.Transaction{
		Version:           1,
		Inputs:            selectedInputs,
		Outputs:           outputs,
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           s.getChainIdBytes(),
	}

	return tx, nil
}

// cacheTransaction 缓存交易并返回哈希
func (s *ContractCallService) cacheTransaction(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	if s.logger != nil {
		s.logger.Debug("📋 缓存合约调用交易")
	}

	// 使用统一的交易哈希服务计算交易哈希
	hashRequest := &transactionpb.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: false,
	}

	hashResponse, err := s.transactionHashServiceClient.ComputeHash(ctx, hashRequest)
	if err != nil {
		return nil, fmt.Errorf("交易哈希计算失败: %v", err)
	}
	if !hashResponse.IsValid {
		return nil, fmt.Errorf("交易结构无效，无法计算哈希")
	}

	txHash := hashResponse.Hash

	// 缓存到内存
	if s.cacheStore != nil {
		cacheKey := hex.EncodeToString(txHash[:])
		internal.CacheUnsignedTransaction(ctx, s.cacheStore, []byte(cacheKey), tx, internal.GetDefaultCacheConfig(), s.logger)
	}

	return txHash[:], nil
}

// generateStateID 生成状态ID
func (s *ContractCallService) generateStateID(contractAddress, methodName string, methodArgs []byte) []byte {
	combined := fmt.Sprintf("%s:%s:%x", contractAddress, methodName, methodArgs)
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// calculateExecutionResultHash 计算执行结果哈希
func (s *ContractCallService) calculateExecutionResultHash(contractAddress, methodName string, methodArgs []byte, executionTimeLimit uint64) []byte {
	combined := fmt.Sprintf("%s:%s:%x:%d", contractAddress, methodName, methodArgs, executionTimeLimit)
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// maxExecutionFeeLimit 返回系统最大执行时间限制
func maxExecutionFeeLimit() uint64 {
	return 10000000 // 10M Gas
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
func (s *ContractCallService) calculateAddressFromPrivateKey(privateKey []byte) (string, error) {
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

// selectUTXOsForContract 为合约调用选择UTXO（内部方法）
func (s *ContractCallService) selectUTXOsForContract(ctx context.Context, callerAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, string, error) {
	// 1. 解析目标金额
	targetAmount, err := parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	// 2. 获取地址所有可用AssetUTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, callerAddr, &assetCategory, true)
	if err != nil {
		return nil, "", fmt.Errorf("获取UTXO失败: %v", err)
	}

	if len(allUTXOs) == 0 {
		return nil, "", fmt.Errorf("地址没有可用UTXO")
	}

	// 3. 简单选择算法：首次适应
	var selectedInputs []*transaction.TxInput
	var totalSelected uint64 = 0

	for _, utxoItem := range allUTXOs {
		utxoAmount := extractUTXOAmount(utxoItem)
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
	changeStr := formatAmount(changeAmount)

	return selectedInputs, changeStr, nil
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// getChainIdBytes 获取链ID字节数组
//
// 🎯 从配置管理器获取真实的链ID配置
//
// 返回：
//   - []byte: 链ID字节数组
func (s *ContractCallService) getChainIdBytes() []byte {
	if s.configManager == nil {
		if s.logger != nil {
			s.logger.Error("配置管理器未初始化，使用默认链ID")
		}
		return []byte("weisyn-mainnet") // 紧急回退
	}

	// 从配置管理器获取区块链配置
	blockchainConfig := s.configManager.GetBlockchain()
	if blockchainConfig == nil {
		if s.logger != nil {
			s.logger.Error("无法获取区块链配置，使用默认链ID")
		}
		return []byte("weisyn-mainnet") // 紧急回退
	}

	// 将ChainID (uint64) 转换为字节数组
	chainID := blockchainConfig.ChainID
	chainIDBytes := make([]byte, 8) // uint64 需要8字节
	binary.BigEndian.PutUint64(chainIDBytes, chainID)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("获取链ID: %d (0x%x)", chainID, chainIDBytes))
	}

	return chainIDBytes
}

// buildZKProofFromExecutionResult 从执行结果构建ZK证明
//
// 🎯 基于execution结果生成零知识证明，证明执行的正确性
//
// 参数：
//   - executionResult: 执行结果
//
// 返回：
//   - *transaction.ZKStateProof: ZK状态证明
func (s *ContractCallService) buildZKProofFromExecutionResult(executionResult *types.ExecutionResult) *transaction.ZKStateProof {
	if executionResult == nil {
		if s.logger != nil {
			s.logger.Error("执行结果为空，无法构建ZK证明")
		}
		return &transaction.ZKStateProof{
			Proof:               []byte{}, // 空证明
			PublicInputs:        [][]byte{},
			ProvingScheme:       "groth16",
			Curve:               "bn254",
			VerificationKeyHash: make([]byte, 32), // 零填充
			CircuitId:           "contract_execution.v1",
			CircuitVersion:      1,
			ConstraintCount:     0,
		}
	}

	// 构建公开输入：执行哈希、成功状态、Gas消耗
	var publicInputs [][]byte

	// 输入1：执行结果哈希
	if len(executionResult.ReturnData) > 0 {
		resultHash := sha256.Sum256(executionResult.ReturnData)
		publicInputs = append(publicInputs, resultHash[:])
	} else {
		publicInputs = append(publicInputs, make([]byte, 32)) // 空结果哈希
	}

	// 输入2：成功状态（1字节：0=失败, 1=成功）
	if executionResult.Success {
		publicInputs = append(publicInputs, []byte{1})
	} else {
		publicInputs = append(publicInputs, []byte{0})
	}

	// 输入3：Gas消耗（8字节大端序）
	gasBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(gasBytes, executionResult.Consumed)
	publicInputs = append(publicInputs, gasBytes)

	// 生成模拟证明（实际中应该调用ZK引擎）
	// 注意：这里是真实的证明结构，只是证明数据是模拟生成的
	simulatedProof := make([]byte, 256) // Groth16典型大小
	if s.hashManager != nil {
		// 使用执行结果生成确定性的模拟证明
		proofSeed := fmt.Sprintf("proof_%x_%t_%d", executionResult.ReturnData, executionResult.Success, executionResult.Consumed)
		proofHash := s.hashManager.SHA256([]byte(proofSeed))
		copy(simulatedProof, proofHash)
		// 填充剩余部分
		for i := len(proofHash); i < 256; i++ {
			simulatedProof[i] = byte(i % 256)
		}
	}

	// 计算验证密钥哈希
	vkHashData := fmt.Sprintf("vk_contract_execution_v1_%s", "bn254")
	var vkHash []byte
	if s.hashManager != nil {
		vkHash = s.hashManager.SHA256([]byte(vkHashData))
	} else {
		sha256Hash := sha256.Sum256([]byte(vkHashData))
		vkHash = sha256Hash[:]
	}

	zkProof := &transaction.ZKStateProof{
		Proof:                 simulatedProof,
		PublicInputs:          publicInputs,
		ProvingScheme:         "groth16",
		Curve:                 "bn254",
		VerificationKeyHash:   vkHash,
		CircuitId:             "contract_execution.v1",
		CircuitVersion:        1,
		ConstraintCount:       10000, // 估算约束数量
		ProofGenerationTimeMs: nil,   // 生产环境不记录时间
		CustomAttributes: map[string]string{
			"execution_engine": "wasm",
			"result_hash":      hex.EncodeToString(publicInputs[0]),
		},
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("构建ZK证明完成 - 电路: %s, 公开输入数: %d", zkProof.CircuitId, len(zkProof.PublicInputs)))
	}

	return zkProof
}

// 确保 ContractCallService 实现了所需的接口部分
var _ interface {
	CallContract(context.Context, []byte, string, string, map[string]interface{}, uint64, string, ...*types.TransferOptions) ([]byte, error)
} = (*ContractCallService)(nil)
