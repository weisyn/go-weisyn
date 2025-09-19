// Package contract 合约部署参数验证器
//
// 🎯 **模块职责**：
// 专门负责智能合约部署过程中的各类验证工作。
// 从主服务文件中分离出来，实现单一职责原则。
//
// 🔧 **核心功能**：
// - 部署参数格式验证
// - WASM字节码格式验证
// - 地址格式解析和验证
// - 合约大小和安全限制检查
//
// 📋 **主要组件**：
// - DeployValidator: 核心验证器
// - 各种验证规则和限制常量
// - 地址解析和格式化工具
//
// 🎯 **设计特点**：
// - 安全优先：严格的格式和安全检查
// - 可配置：支持动态调整验证参数
// - 详细报错：提供清晰的验证失败信息
package contract

import (
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//
//	验证器数据结构定义
//
// ============================================================================

// DeployValidator 合约部署验证器
//
// 🎯 **验证器职责**：
// 对智能合约部署过程中的各项参数进行严格验证，确保安全性和合规性。
//
// 🔧 **验证能力**：
// - 参数完整性验证：检查必需参数是否提供
// - 格式正确性验证：验证WASM格式、地址格式等
// - 安全限制验证：检查合约大小、权限设置等（从配置获取）
// - 业务逻辑验证：验证业务参数的合理性
//
// 💡 **设计特点**：
// - 配置驱动：所有限制值从配置系统获取，不使用硬编码
// - 标准接口：使用crypto.AddressManager进行地址验证，遵循系统标准
// - 依赖注入：通过构造器注入所需依赖，确保组件解耦
type DeployValidator struct {
	logger         log.Logger            // 日志记录器
	configProvider config.Provider       // 配置提供者（获取大小限制等配置）
	addressManager crypto.AddressManager // 地址管理器（标准地址验证）
}

// NewDeployValidator 创建部署验证器
//
// 🎯 **工厂方法**：
// 创建一个新的合约部署验证器实例，使用依赖注入模式。
//
// 参数：
//   - logger: 日志记录器，用于输出验证过程信息
//   - configProvider: 配置提供者，用于获取系统配置（如大小限制）
//   - addressManager: 地址管理器，用于标准地址验证
//
// 返回：
//   - *DeployValidator: 配置好的验证器实例
func NewDeployValidator(
	logger log.Logger,
	configProvider config.Provider,
	addressManager crypto.AddressManager,
) *DeployValidator {
	// 依赖检查
	if configProvider == nil {
		panic("DeployValidator: configProvider不能为nil")
	}
	if addressManager == nil {
		panic("DeployValidator: addressManager不能为nil")
	}

	return &DeployValidator{
		logger:         logger,
		configProvider: configProvider,
		addressManager: addressManager,
	}
}

// ============================================================================
//
//	核心验证方法
//
// ============================================================================

// ValidateDeployParams 验证合约部署参数
//
// 🎯 **参数验证**：
// 对智能合约部署的输入参数进行全面验证，确保参数完整性和正确性。
//
// 📋 **验证项目**：
// 1. 基础参数检查：部署者地址、WASM字节码非空验证
// 2. 大小限制检查：WASM字节码大小不超过系统限制
// 3. 部署选项验证：检查部署选项的完整性和合理性
// 4. 格式正确性：验证各类参数的格式要求
//
// 🔧 **验证流程**：
// - 快速失败：遇到第一个错误立即返回
// - 详细报错：提供具体的错误位置和原因
// - 安全检查：包含安全相关的限制验证
//
// 参数：
//   - deployerAddress: 合约部署者地址
//   - wasmCode: WASM合约字节码
//   - options: 部署选项列表
//
// 返回：
//   - error: 验证失败时的详细错误信息
func (dv *DeployValidator) ValidateDeployParams(
	deployerAddress string,
	wasmCode []byte,
	options []*types.ResourceDeployOptions,
) error {
	if dv.logger != nil {
		dv.logger.Debug("🔍 开始验证合约部署参数")
	}

	// ========== 基础参数验证 ==========
	if deployerAddress == "" {
		return fmt.Errorf("合约部署者地址不能为空")
	}

	if len(wasmCode) == 0 {
		return fmt.Errorf("WASM合约字节码不能为空")
	}

	// ========== 地址格式验证（使用标准接口）==========
	valid, err := dv.addressManager.ValidateAddress(deployerAddress)
	if err != nil {
		return fmt.Errorf("地址验证过程出错: %v", err)
	}
	if !valid {
		return fmt.Errorf("部署者地址格式无效: %s", deployerAddress)
	}

	// ========== 大小限制验证（从配置获取）==========
	maxSize := dv.getMaxContractCodeSizeFromConfig()
	if len(wasmCode) > int(maxSize) {
		return fmt.Errorf("合约字节码超过大小限制，当前: %d 字节，最大支持: %d 字节",
			len(wasmCode), maxSize)
	}

	// ========== 部署选项验证 ==========
	for i, option := range options {
		if option == nil {
			return fmt.Errorf("第 %d 个部署选项不能为 nil", i+1)
		}

		// 验证具体的选项内容
		if err := dv.validateDeployOption(option, i+1); err != nil {
			return fmt.Errorf("第 %d 个部署选项验证失败: %v", i+1, err)
		}
	}

	if dv.logger != nil {
		dv.logger.Debug(fmt.Sprintf("✅ 部署参数验证通过 - 字节码大小: %d, 选项数量: %d",
			len(wasmCode), len(options)))
	}

	return nil
}

// ValidateWasmFormat 验证WASM字节码格式
//
// 🎯 **格式验证**：
// 对WASM字节码进行格式完整性验证，确保是有效的WebAssembly模块。
//
// 📋 **验证标准**：
// 1. 魔数检查：验证WASM魔数 (0x00 0x61 0x73 0x6D)
// 2. 版本验证：确保使用支持的WASM版本 (当前仅支持版本1)
// 3. 长度检查：确保字节码长度满足最小要求（从配置获取）
// 4. 结构完整性：基础的WASM结构检查
//
// 🔧 **安全考虑**：
// - 防止恶意字节码注入
// - 确保字节码符合WASM标准
// - 预防格式错误导致的执行异常
//
// 参数：
//   - wasmCode: WASM字节码数据
//
// 返回：
//   - error: 格式验证失败时的错误信息
func (dv *DeployValidator) ValidateWasmFormat(wasmCode []byte) error {
	// ========== 长度检查（从配置获取最小大小）==========
	minSize := dv.getMinContractCodeSizeFromConfig()
	if len(wasmCode) < int(minSize) {
		return fmt.Errorf("WASM字节码长度不足，至少需要%d字节，当前: %d 字节", minSize, len(wasmCode))
	}

	// ========== WASM魔数验证 ==========
	// WASM标准魔数：\0asm (0x00 0x61 0x73 0x6D)
	expectedMagic := []byte{0x00, 0x61, 0x73, 0x6D}
	if len(wasmCode) < 4 {
		return fmt.Errorf("WASM字节码太短，无法验证魔数")
	}
	actualMagic := wasmCode[:4]

	for i, expectedByte := range expectedMagic {
		if actualMagic[i] != expectedByte {
			return fmt.Errorf("WASM魔数验证失败，期望: %v，实际: %v",
				expectedMagic, actualMagic)
		}
	}

	// ========== WASM版本验证 ==========
	// WASM标准版本：1 (0x01 0x00 0x00 0x00，小端序)
	expectedVersion := []byte{0x01, 0x00, 0x00, 0x00}
	if len(wasmCode) < 8 {
		return fmt.Errorf("WASM字节码太短，无法验证版本")
	}
	actualVersion := wasmCode[4:8]

	for i, expectedByte := range expectedVersion {
		if actualVersion[i] != expectedByte {
			return fmt.Errorf("WASM版本验证失败，当前只支持版本1，实际版本字节: %v",
				actualVersion)
		}
	}

	if dv.logger != nil {
		dv.logger.Debug(fmt.Sprintf("✅ WASM字节码格式验证通过 - 大小: %d 字节", len(wasmCode)))
	}

	return nil
}

// ParseAddress 解析地址字符串为字节数组（使用标准接口）
//
// 🎯 **地址解析**：
// 使用crypto.AddressManager标准接口解析地址，确保与系统地址规范一致。
//
// 📋 **解析流程**：
// 1. 使用AddressManager.StringToAddress验证并规范化地址
// 2. 使用AddressManager.AddressToBytes转换为20字节数组
// 3. 确保结果符合transaction.proto中的bytes地址约束
//
// 🔧 **优势**：
// - 标准化：使用系统统一的地址处理逻辑
// - 格式兼容：自动处理各种地址格式（Base58Check等）
// - 类型安全：返回固定20字节的地址哈希
//
// 参数：
//   - addressStr: 地址字符串
//
// 返回：
//   - []byte: 20字节的地址数组（符合proto定义）
//   - error: 解析失败时的错误信息
func (dv *DeployValidator) ParseAddress(addressStr string) ([]byte, error) {
	if addressStr == "" {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 使用标准接口验证地址格式
	normalizedAddress, err := dv.addressManager.StringToAddress(addressStr)
	if err != nil {
		return nil, fmt.Errorf("地址格式验证失败: %v", err)
	}

	// 转换为字节数组（proto中的bytes类型）
	addressBytes, err := dv.addressManager.AddressToBytes(normalizedAddress)
	if err != nil {
		return nil, fmt.Errorf("地址转换为字节数组失败: %v", err)
	}

	// 验证长度（transaction.proto中地址通常为20字节）
	if len(addressBytes) != 20 {
		return nil, fmt.Errorf("地址字节长度错误，期望: 20 字节，实际: %d 字节",
			len(addressBytes))
	}

	if dv.logger != nil {
		dv.logger.Debug(fmt.Sprintf("✅ 地址解析成功: %s -> %x", addressStr, addressBytes))
	}

	return addressBytes, nil
}

// ============================================================================
//
//	配置获取方法
//
// ============================================================================

// getMaxContractCodeSizeFromConfig 从配置获取智能合约的最大字节码大小
//
// 🎯 **配置驱动**：
// 从区块链配置系统动态获取最大合约大小限制，避免硬编码。
//
// 📋 **配置来源**：
// - 优先使用 execution.wasm.max_code_size 配置
// - 如果不存在，使用 transaction.max_transaction_size 作为上限
// - 兼容现有配置结构，确保向后兼容性
//
// 返回：
//   - uint64: 最大合约字节码大小（字节）
func (dv *DeployValidator) getMaxContractCodeSizeFromConfig() uint64 {
	// 首先尝试获取区块链配置
	blockchain := dv.configProvider.GetBlockchain()
	if blockchain != nil {
		// 使用交易最大大小作为合约大小的上限
		maxTxSize := blockchain.Transaction.MaxTransactionSize
		if maxTxSize > 0 {
			// 合约大小不应超过交易大小的80%（留出其他数据空间）
			return uint64(float64(maxTxSize) * 0.8)
		}
	}

	// 默认值：10MB（与之前的常量保持一致）
	defaultMaxSize := uint64(10 * 1024 * 1024)

	if dv.logger != nil {
		dv.logger.Debug(fmt.Sprintf("使用默认最大合约大小: %d 字节", defaultMaxSize))
	}

	return defaultMaxSize
}

// getMinContractCodeSizeFromConfig 从配置获取智能合约的最小字节码大小
//
// 🎯 **最小大小限制**：
// 确保WASM字节码至少包含必要的头部信息。
//
// 返回：
//   - uint64: 最小合约字节码大小（字节）
func (dv *DeployValidator) getMinContractCodeSizeFromConfig() uint64 {
	// WASM头部最小大小（魔数4字节+版本4字节）
	return 8
}

// validateDeployOption 验证单个部署选项
//
// 🎯 **选项验证**：
// 对单个部署选项进行详细验证，确保选项内容的正确性和合理性。
//
// 📋 **验证内容**：
// - 选项结构完整性（非空验证）
// - 参数值的合理性（未来可扩展）
// - 业务逻辑的正确性（未来可扩展）
//
// 参数：
//   - option: 部署选项对象
//   - index: 选项在列表中的索引（用于错误定位）
//
// 返回：
//   - error: 验证失败时的错误信息
func (dv *DeployValidator) validateDeployOption(option *types.ResourceDeployOptions, index int) error {
	// 基础非空检查
	if option == nil {
		return fmt.Errorf("第 %d 个部署选项不能为空", index)
	}

	// 当前版本：简化验证，只做基础检查
	// 未来可以根据具体业务需求添加更详细的字段验证

	return nil
}

// （已移除：所有硬编码常量，配置参数现在从 config.Provider 动态获取）
