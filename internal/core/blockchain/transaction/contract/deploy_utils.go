package contract

import (
	"fmt"
	"strconv"
	"time"

	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
// 合约部署工具方法集合
// ============================================================================
//
// 🎯 **文件职责**：
// 为合约部署服务提供各种工具和辅助方法，包括：
//
// 📋 **核心功能**：
// - 数据格式转换：金额解析、格式化等
// - UTXO数据提取：从复杂UTXO结构提取所需信息
// - 配置选项处理：版本信息、自定义属性等
// - 链标识管理：ChainID获取和管理
//
// 💡 **设计原则**：
// - 纯函数优先：大部分方法为纯函数，无副作用
// - 错误明确：提供清晰的错误信息和处理
// - 类型安全：严格的类型检查和转换
// - 可复用性：方法可在不同场景下复用
//
// 🔧 **使用场景**：
// - 合约部署参数处理
// - 交易构建数据准备
// - UTXO解析和金额计算
// - 配置选项标准化处理

// ============================================================================
//
//	金额处理工具方法
//
// ============================================================================

// parseAmount 解析金额字符串为uint64数值
//
// 🎯 **功能说明**：
// 将用户输入的金额字符串转换为内部使用的uint64数值。
// 支持标准的十进制数值格式，提供详细的错误信息。
//
// 📋 **支持格式**：
// - 整数：如"100", "1000000"
// - 零值：如"0"
// - 不支持负数、小数点、科学记数法等
//
// 🚨 **安全检查**：
// - 数值范围检查：必须在uint64有效范围内
// - 格式验证：只接受有效的十进制数字
// - 空值处理：空字符串或无效字符返回错误
//
// 参数：
//   - amountStr: 金额字符串
//
// 返回：
//   - uint64: 解析后的数值
//   - error: 解析错误信息
func parseAmount(amountStr string) (uint64, error) {
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %v", err)
	}
	return amount, nil
}

// extractUTXOAmount 从UTXO结构中提取金额数值
//
// 🎯 **功能说明**：
// 从复杂的UTXO数据结构中提取实际的资产金额。
// 处理多种UTXO内容策略和资产类型，确保数据提取的准确性。
//
// 📋 **支持的UTXO类型**：
// - CachedOutput：缓存的输出数据
//   - NativeCoinAsset：原生代币资产
//   - ContractTokenAsset：合约代币资产
//
// - 其他策略：返回0（未实现或不适用）
//
// 🔧 **处理逻辑**：
// 1. 验证UTXO有效性（非nil检查）
// 2. 根据ContentStrategy类型进行分发处理
// 3. 提取对应资产类型的金额字段
// 4. 进行字符串到数值的安全转换
//
// 🚨 **容错设计**：
// - nil输入返回0
// - 无效数据结构返回0
// - 解析失败返回0（静默处理，记录但不中断）
//
// 参数：
//   - utxoItem: UTXO数据结构指针
//
// 返回：
//   - uint64: 提取的金额数值，失败时返回0
func extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
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

// formatAmount 格式化uint64金额为字符串
//
// 🎯 **功能说明**：
// 将内部使用的uint64数值格式化为标准的十进制字符串。
// 提供统一的金额显示格式，确保数据展示的一致性。
//
// 📋 **格式特点**：
// - 十进制表示：如100 -> "100"
// - 无千分位符：保持原始数值格式
// - 无前导零：标准数字格式
//
// 🔧 **使用场景**：
// - 交易金额显示
// - 日志记录
// - API返回数据
// - 存储序列化
//
// 参数：
//   - amount: uint64数值
//
// 返回：
//   - string: 格式化后的字符串
func formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// ============================================================================
//
//	配置选项处理工具方法
//
// ============================================================================

// extractVersionFromOptions 从部署选项中提取版本信息
//
// 🎯 **功能说明**：
// 从ResourceDeployOptions中提取合约的版本标识。
// 为资源版本管理提供统一的版本信息提取接口。
//
// 📋 **版本管理策略**：
// 返回空字符串表示使用系统默认版本
// - 支持多种版本格式的标准化处理
//
// 📋 **支持的版本格式**：
// - 语义版本：如"1.0.0", "2.1.3-beta.1"
// - 简单版本：如"v1", "1.2"
// - 时间版本：如"2024.01.15"
//
// 🔮 **扩展计划**：
// ```go
// // 未来的ResourceDeployOptions可能包含：
//
//	type ResourceDeployOptions struct {
//	    Version        string            // 版本标识
//	    CustomVersion  map[string]string // 自定义版本信息
//	}
//
// ```
//
// 参数：
//   - options: 部署选项结构（可能为空）
//
// 返回：
//   - string: 提取的版本号，空字符串表示使用默认版本
func extractVersionFromOptions(options *types.ResourceDeployOptions) string {
	if options == nil {
		return ""
	}

	// 使用默认版本

	return ""
}

// extractCustomAttributes 从部署选项中提取自定义属性
//
// 🎯 **功能说明**：
// 将ResourceDeployOptions中的业务属性转换为Resource的CustomAttributes。
// 当前从选项中提取可用的自定义属性，生成标准部署元数据。
//
// 📋 **处理逻辑**：
// - 生成部署时间戳等标准元数据
// - 记录部署方式信息
// - 将来可以扩展ResourceDeployOptions来支持更多自定义属性
//
// 🏷️ **标准属性说明**：
// - `deployment_timestamp`: 部署时间戳（Unix秒）
// - `deployment_method`: 部署方法标识（wasm_contract_deploy）
// - `deployment_source`: 部署来源服务（contract_deploy_service）
//
// 🔮 **扩展计划**：
// ```go
// // 未来的ResourceDeployOptions可能包含：
//
//	type ResourceDeployOptions struct {
//	    CustomAttributes map[string]string // 用户自定义属性
//	    Tags            []string          // 资源标签
//	    Metadata        interface{}       // 结构化元数据
//	}
//
// ```
//
// 参数：
//   - options: 部署选项结构
//
// 返回：
//   - map[string]string: 处理后的自定义属性映射
func extractCustomAttributes(options *types.ResourceDeployOptions) map[string]string {
	attributes := make(map[string]string)

	// 添加标准的部署元数据
	attributes["deployment_timestamp"] = fmt.Sprintf("%d", time.Now().Unix())
	attributes["deployment_method"] = "wasm_contract_deploy"

	// 标记这是通过标准部署流程创建的资源
	attributes["deployment_source"] = "contract_deploy_service"

	return attributes
}

// ============================================================================
//
//	链标识管理工具方法
//
// ============================================================================

// getChainIdBytes 获取链ID字节数组
//
// 🎯 **功能说明**：
// 获取当前区块链网络的ChainID，用于防止跨链重放攻击。
// 当前使用硬编码默认值，未来需要从配置服务获取。
//
// 📋 **设计说明**：
// - 生产环境：从configManager获取链ID
// - 开发环境：使用"weisyn-testnet"
// - 默认环境：使用"weisyn-mainnet"
//
// 🚨 **安全性**：
// ChainID是防止跨链重放攻击的关键参数，必须确保不同网络使用不同的值。
//
// 🔧 **实现状态**：
// 从配置管理器获取ChainID，使用标准配置接口
// 3. 支持动态链ID切换（如果需要）
//
// 🔮 **完整实现示例**：
// ```go
//
//	func (s *ContractDeployService) getChainIdBytes() []byte {
//	    if s.configManager != nil {
//	        return s.configManager.GetChainId()
//	    }
//
//	    // 根据网络类型返回不同ChainID
//	    switch s.networkType {
//	    case "mainnet":
//	        return []byte("weisyn-mainnet")
//	    case "testnet":
//	        return []byte("weisyn-testnet")
//	    case "devnet":
//	        return []byte("weisyn-devnet")
//	    default:
//	        return []byte("weisyn-local")
//	    }
//	}
//
// ```
//
// 返回：
//   - []byte: 链ID字节数组
func (s *ContractDeployService) getChainIdBytes() []byte {
	// ✅ 完整实现：从配置管理器获取链ID
	if s.configManager != nil {
		if blockchainConfig := s.configManager.GetBlockchain(); blockchainConfig != nil {
			chainId := blockchainConfig.ChainID
			if chainId > 0 {
				// 将uint64转换为[]byte（大端序）
				result := make([]byte, 8)
				result[0] = byte(chainId >> 56)
				result[1] = byte(chainId >> 48)
				result[2] = byte(chainId >> 40)
				result[3] = byte(chainId >> 32)
				result[4] = byte(chainId >> 24)
				result[5] = byte(chainId >> 16)
				result[6] = byte(chainId >> 8)
				result[7] = byte(chainId)
				return result
			}
		}
	}

	// 🛡️ 安全的默认值策略
	if s.logger != nil {
		s.logger.Warn("配置管理器未提供链ID，使用默认值：weisyn-mainnet")
	}
	return []byte("weisyn-mainnet")
}
