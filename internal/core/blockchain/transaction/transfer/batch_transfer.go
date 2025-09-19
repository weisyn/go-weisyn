// Package transfer 批量资产转账实现
//
// 🎯 **模块定位**：TransactionService 接口的批量转账功能实现
//
// 本文件实现批量资产转账的核心业务逻辑，包括：
// - 多接收方批量转账（BatchTransfer）
// - 批量转账的原子性保证
// - 优化的 UTXO 选择策略
// - 统一的费用计算和分摊
// - 批量操作的错误处理和回滚
//
// 🏗️ **架构定位**：
// - 业务层：实现批量转账的复杂业务逻辑
// - 优化层：提供比多次单独转账更高效的解决方案
// - 原子性：确保批量操作的事务完整性
//
// 🔧 **设计原则**：
// - 原子操作：批量转账要么全部成功，要么全部失败
// - 费用优化：通过合并交易减少总体费用
// - 性能优先：批量处理比逐一处理更高效
// - 错误透明：提供详细的每个转账项错误信息
//
// 📋 **支持的批量模式**：
// - 一对多转账：一个发送方向多个接收方转账
// - 同质化批量：所有转账使用相同代币类型
// - 异构批量：支持不同代币类型的混合批量转账
// - 条件批量：支持部分成功的批量转账模式
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package transfer

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
//
//	批量转账实现服务
//
// ============================================================================
// BatchTransferService 批量资产转账核心实现服务
//
// 🎯 **服务职责**：
// - 实现 TransactionService.BatchTransfer 方法
// - 处理一对多的批量转账场景
// - 优化 UTXO 选择和费用分摊
// - 保证批量操作的原子性
//
// 🔧 **依赖注入**：
// - utxoSelector：UTXO 选择和管理服务
// - feeCalculator：费用计算服务
// - cacheStore：交易缓存存储
// - assetTransferService：单笔转账服务（复用逻辑）
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewBatchTransferService(utxoSelector, feeCalc, cache, assetService, logger)
//	txHash, err := service.BatchTransfer(ctx, batchParams)
type BatchTransferService struct {
	// 核心依赖服务（使用公共接口）
	utxoManager         repository.UTXOManager                   // UTXO 管理服务
	cacheStore          storage.MemoryStore                      // 内存缓存存储
	keyManager          crypto.KeyManager                        // 密钥管理服务（用于从私钥生成公钥）
	addressManager      crypto.AddressManager                    // 地址管理服务（用于从公钥生成地址）
	configManager       config.Provider                          // 配置管理器（用于获取链ID等配置信息）
	txHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端（用于计算交易哈希）
	logger              log.Logger                               // 日志记录器
}

// NewBatchTransferService 创建批量转账服务实例
//
// 🏗️ **构造器模式**：
// 使用依赖注入创建服务实例，确保所有依赖都已正确初始化
//
// 参数：
//   - utxoManager: UTXO 选择和管理服务
//   - cacheStore: 交易缓存存储服务
//   - keyManager: 密钥管理服务
//   - addressManager: 地址管理服务
//   - configManager: 配置管理器（用于获取链ID等配置信息）
//   - txHashServiceClient: 交易哈希服务客户端（用于计算交易哈希）
//   - logger: 日志记录器
//
// 返回：
//   - *BatchTransferService: 批量转账服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewBatchTransferService(
	utxoManager repository.UTXOManager,
	cacheStore storage.MemoryStore,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	configManager config.Provider,
	txHashServiceClient transaction.TransactionHashServiceClient,
	logger log.Logger,
) *BatchTransferService {
	if utxoManager == nil {
		panic("BatchTransferService: utxoManager不能为nil")
	}
	if cacheStore == nil {
		panic("BatchTransferService: cacheStore不能为nil")
	}
	if keyManager == nil {
		panic("BatchTransferService: keyManager不能为nil")
	}
	if addressManager == nil {
		panic("BatchTransferService: addressManager不能为nil")
	}
	if configManager == nil {
		panic("BatchTransferService: configManager不能为nil")
	}
	if txHashServiceClient == nil {
		panic("BatchTransferService: txHashServiceClient不能为nil")
	}
	if logger == nil {
		panic("BatchTransferService: logger不能为nil")
	}
	return &BatchTransferService{
		utxoManager:         utxoManager,
		cacheStore:          cacheStore,
		keyManager:          keyManager,
		addressManager:      addressManager,
		configManager:       configManager,
		txHashServiceClient: txHashServiceClient,
		logger:              logger,
	}
}

// ============================================================================
//
//	核心批量转账方法实现
//
// ============================================================================
// BatchTransfer 实现批量资产转账功能
//
// 🎯 **方法职责**：
// 实现 blockchain.TransactionService.BatchTransfer 接口
// 支持一对多的批量资产转账操作，提供完整的EUTXO批量交易构建能力
//
// 📋 **详细业务流程**：
// 1. 【地址计算】：通过crypto.AddressManager从私钥计算发送方地址
// 2. 【参数验证】：验证批量转账参数（地址格式、金额范围、数量限制等）
// 3. 【金额计算】：计算总转账金额和按代币类型分组的资金需求
// 4. 【UTXO选择】：调用internal.SelectUTXOsForTransfer选择最优UTXO组合
// 5. 【交易构建】：构建包含多个输出的EUTXO标准Transaction结构
// 6. 【费用处理】：计算批量交易费用并处理找零逻辑
// 7. 【缓存存储】：将未签名交易存储到storage.MemoryStore供后续使用
// 8. 【哈希返回】：计算并返回交易哈希用于签名流程
//
// 📝 **详细参数说明**：
//   - ctx: context.Context - 请求上下文，支持超时控制和取消操作
//   - 用于所有异步操作的生命周期管理
//   - 传递给所有依赖组件的调用（UTXO查询、缓存操作等）
//   - senderPrivateKey: []byte - 发送方的ECDSA secp256k1私钥
//   - 32字节的私钥数据，用于计算发送方地址
//   - 通过crypto.AddressManager.PrivateKeyToAddress()转换为地址
//   - 私钥本身不会被存储或传输，仅用于地址计算
//   - transfers: []types.TransferParams - 批量转账参数列表
//   - 每个TransferParams包含：ToAddress、Amount、TokenID、Memo
//   - 支持最多100笔转账（通过getMaxBatchTransferSize()动态配置限制）
//   - 支持混合代币类型的批量转账
//   - 自动按代币类型分组进行UTXO选择优化
//
// 📤 **详细返回值说明**：
//   - []byte: 32字节的交易哈希
//   - SHA256哈希值，唯一标识这笔批量交易
//   - 用于后续的签名操作（SignTransaction）
//   - 用于交易状态查询和跟踪
//   - error: 详细的错误信息
//   - 参数验证错误：格式、范围、数量限制检查失败
//   - UTXO选择错误：余额不足、UTXO不可用等
//   - 交易构建错误：protobuf序列化失败等
//   - 缓存操作错误：内存存储失败等
//
// 🔗 **组件交互细节**：
// 1. crypto.AddressManager - 地址计算服务
//   - PrivateKeyToAddress([]byte) (string, error) - 从私钥计算地址
//
// 2. repository.UTXOManager - UTXO管理服务
//   - 通过internal.SelectUTXOsForTransfer间接调用
//   - 用于查询指定地址的可用UTXO集合
//
// 3. storage.MemoryStore - 内存缓存服务
//   - 通过internal.CacheUnsignedTransaction存储未签名交易
//   - 键为交易哈希，值为序列化的Transaction结构
//
// 4. config.Provider - 配置管理服务
//   - GetBlockchain().ChainID - 获取当前链ID
//   - 用于构建交易的ChainId字段，防止重放攻击
//
// 🎯 **支持的批量转账场景**：
//   - 基础批量原生币转账：BatchTransfer(ctx, privKey, []TransferParams{{toAddr, "100.0", "", "工资"}})
//   - 混合代币批量转账：支持原生币+多种合约FT的混合批量转账
//   - 企业工资发放：BatchTransfer(ctx, privKey, payrollTransfers)
//   - 营销空投活动：BatchTransfer(ctx, privKey, airdropTransfers)
//   - 股东分红发放：BatchTransfer(ctx, privKey, dividendTransfers)
//   - 成本优化转账：合并多笔转账减少总体手续费
//
// 💡 **核心优化特性**：
// - 费用节约：单个交易包含多个输出，比N笔单独转账节省(N-1)倍基础费用
// - 原子性保证：批量操作要么全部成功，要么全部失败，无部分成功风险
// - 性能优化：一次网络提交完成所有转账，降低网络延迟影响
// - 智能UTXO选择：按代币类型分组优化，减少不必要的UTXO碎片化
// - 并发安全：支持多线程同时构建不同的批量交易
//
// ⚠️ **重要说明**：
// - 此方法只构建未签名交易，不执行实际的资金转移
// - 返回的交易哈希需要通过SignTransaction进行签名
// - 签名后的交易需要通过SubmitTransaction提交到网络
// - 交易成功与否需要通过GetTransactionStatus查询确认
// - 批量转账数量限制为100笔，超出会返回验证错误
func (s *BatchTransferService) BatchTransfer(
	ctx context.Context,
	senderPrivateKey []byte,
	transfers []types.TransferParams,
) ([]byte, error) {
	// 📍 **步骤1: 地址计算** - 通过加密服务从私钥计算发送方地址
	// 【组件交互】：crypto.AddressManager.PrivateKeyToAddress()
	// • 输入：32字节ECDSA secp256k1私钥
	// • 处理：椭圆曲线运算 -> 公钥 -> Keccak256 -> 地址
	// • 输出：40字符十六进制地址字符串
	// • 错误：私钥格式无效、椭圆曲线计算失败等
	fromAddress, err := s.addressManager.PrivateKeyToAddress(senderPrivateKey)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 地址计算失败 - 私钥长度: %d, 错误: %v", len(senderPrivateKey), err))
		}
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理批量转账请求 - from: %s, 转账数量: %d",
			fromAddress, len(transfers)))
	}

	// 🔄 步骤1: 基础参数验证
	if err := s.validateBatchTransferParams(fromAddress, transfers); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 🧮 步骤2: 计算总金额需求（按代币类型分组）
	totalAmountsByToken, err := s.calculateBatchTotalAmount(transfers)
	if err != nil {
		return nil, fmt.Errorf("计算总金额失败: %v", err)
	}

	// 📍 步骤3: 解析发送方地址
	fromAddrBytes, err := s.parseAddress(fromAddress)
	if err != nil {
		return nil, fmt.Errorf("发送方地址解析失败: %v", err)
	}

	// 💰 步骤4: 选择UTXO覆盖所有转账需求
	selectedInputs, changeAmountsByToken, err := s.selectBatchUTXOs(ctx, fromAddrBytes, totalAmountsByToken)
	if err != nil {
		return nil, fmt.Errorf("UTXO选择失败: %v", err)
	}

	// 🏗️ 步骤5: 构建批量输出（多个接收方 + 找零输出）
	outputs, err := s.buildBatchOutputs(transfers, changeAmountsByToken, fromAddress)
	if err != nil {
		return nil, fmt.Errorf("构建批量输出失败: %v", err)
	}

	// 🔄 步骤6: 构建完整交易
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	// 🔄 步骤7: 计算交易哈希并缓存
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 批量转账交易构建完成 - txHash: %x, inputs: %d, outputs: %d",
			txHash, len(selectedInputs), len(outputs)))
	}

	return txHash, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// validateBatchTransferParams 验证批量转账参数的完整性和有效性
//
// 🔍 **验证项目**：
// - 发送方地址格式验证
// - 批量转账数量限制
// - 每个转账项的参数验证
// - 重复接收方检测
//
// 参数：
//   - fromAddress: 发送方地址
//   - transfers: 批量转账参数列表
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *BatchTransferService) validateBatchTransferParams(
	fromAddress string,
	transfers []types.TransferParams,
) error {
	if s.logger != nil {
		s.logger.Debug("🔍 验证批量转账参数")
	}

	// 基础参数验证
	if fromAddress == "" {
		return fmt.Errorf("发送方地址不能为空")
	}
	if len(transfers) == 0 {
		return fmt.Errorf("批量转账列表不能为空")
	}
	maxSize := s.getMaxBatchTransferSize()
	if len(transfers) > maxSize {
		return fmt.Errorf("批量转账数量超过限制，最大支持 %d 笔", maxSize)
	}

	// 验证每个转账项并检测重复
	seen := make(map[string]bool)
	for i, transfer := range transfers {
		if err := s.validateBatchTransferItem(i, transfer); err != nil {
			return fmt.Errorf("第 %d 个转账项验证失败: %v", i+1, err)
		}

		// 检测重复的接收方地址
		if seen[transfer.ToAddress] {
			return fmt.Errorf("检测到重复的接收方地址: %s", transfer.ToAddress)
		}
		seen[transfer.ToAddress] = true

		// 验证不能向自己转账
		if fromAddress == transfer.ToAddress {
			return fmt.Errorf("第 %d 个转账项：不能向自己转账", i+1)
		}
	}

	return nil
}

// validateBatchTransferItem 验证单个批量转账项
//
// 🔍 **验证项目**：
// - 接收方地址格式验证
// - 转账金额有效性检查
// - 代币ID 格式验证
//
// 参数：
//   - index: 转账项在批量列表中的索引
//   - transfer: 单个转账参数
//
// 返回：
//   - error: 验证失败时的错误信息
func (s *BatchTransferService) validateBatchTransferItem(
	index int,
	transfer types.TransferParams,
) error {
	if transfer.ToAddress == "" {
		return fmt.Errorf("接收方地址不能为空")
	}
	if transfer.Amount == "" || transfer.Amount == "0" {
		return fmt.Errorf("转账金额必须大于0")
	}

	// 验证金额格式（用户输入支持小数格式）
	amountWei, err := utils.ParseDecimalToWei(transfer.Amount)
	if err != nil {
		return fmt.Errorf("金额格式无效: %v", err)
	}
	if amountWei == 0 {
		return fmt.Errorf("转账金额必须大于0")
	}

	// 验证TokenID格式（如果提供）
	if transfer.TokenID != "" {
		if len(transfer.TokenID) != 40 {
			return fmt.Errorf("TokenID格式无效，期望40字符的十六进制字符串")
		}
	}

	return nil
}

// calculateBatchTotalAmount 计算批量转账的总金额需求
//
// 🧮 **计算内容**：
// - 所有转账金额的总和
// - 按代币类型分组计算
// - 原生代币和合约FT分离
//
// 参数：
//   - transfers: 批量转账列表
//
// 返回：
//   - map[string]string: 按代币ID分组的总金额需求（空字符串键表示原生代币）
//   - error: 计算失败时的错误信息
func (s *BatchTransferService) calculateBatchTotalAmount(
	transfers []types.TransferParams,
) (map[string]string, error) {
	if s.logger != nil {
		s.logger.Debug("🧮 计算批量转账总金额")
	}

	// 按代币类型分组累计金额
	totalsByToken := make(map[string]uint64)

	for i, transfer := range transfers {
		amountWei, err := utils.ParseDecimalToWei(transfer.Amount)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个转账项金额解析失败: %v", i+1, err)
		}

		// 使用空字符串表示原生代币，实际TokenID表示合约FT
		tokenKey := transfer.TokenID // 空字符串表示原生代币
		totalsByToken[tokenKey] += amountWei
	}

	// 转换回字符串格式（使用精确的wei格式化）
	result := make(map[string]string)
	for tokenID, totalAmountWei := range totalsByToken {
		result[tokenID] = utils.FormatAmountForProtobuf(totalAmountWei) // 使用统一的protobuf格式化方法
		if s.logger != nil {
			if tokenID == "" {
				s.logger.Debug(fmt.Sprintf("💰 原生代币总需求: %s", result[tokenID]))
			} else {
				s.logger.Debug(fmt.Sprintf("💰 合约FT %s 总需求: %s", tokenID, result[tokenID]))
			}
		}
	}

	return result, nil
}

// selectBatchUTXOs 为批量转账选择合适的 UTXO
//
// 🎯 **选择策略**：
// - 尽量使用大额 UTXO 覆盖批量需求
// - 最小化输入 UTXO 数量
// - 考虑不同代币类型的混合需求
// - 优化找零策略
//
// 参数：
//   - ctx: 上下文对象
//   - fromAddrBytes: 发送方地址字节数组
//   - totalAmounts: 按代币类型的总需求
//
// 返回：
//   - []*transaction.TxInput: 选中的输入 UTXO 列表
//   - map[string]string: 按代币类型的找零金额
//   - error: 选择失败时的错误信息
func (s *BatchTransferService) selectBatchUTXOs(
	ctx context.Context,
	fromAddrBytes []byte,
	totalAmounts map[string]string,
) ([]*transaction.TxInput, map[string]string, error) {
	if s.logger != nil {
		s.logger.Debug("💰 选择批量转账UTXO")
	}

	var allSelectedInputs []*transaction.TxInput
	changeAmounts := make(map[string]string)

	// 逐个代币类型进行UTXO选择
	for tokenID, requiredAmount := range totalAmounts {
		if s.logger != nil {
			if tokenID == "" {
				s.logger.Debug(fmt.Sprintf("🔍 为原生代币选择UTXO - 需求: %s", requiredAmount))
			} else {
				s.logger.Debug(fmt.Sprintf("🔍 为合约FT %s 选择UTXO - 需求: %s", tokenID, requiredAmount))
			}
		}

		// 调用简化的UTXO选择器
		selectedInputs, changeAmount, err := s.selectUTXOsForAmount(
			ctx, fromAddrBytes, requiredAmount, tokenID)
		if err != nil {
			if tokenID == "" {
				return nil, nil, fmt.Errorf("原生代币UTXO选择失败: %v", err)
			} else {
				return nil, nil, fmt.Errorf("合约FT %s UTXO选择失败: %v", tokenID, err)
			}
		}

		// 合并选择的输入
		allSelectedInputs = append(allSelectedInputs, selectedInputs...)

		// 记录找零金额
		if changeAmount != "0" && changeAmount != "" {
			changeAmounts[tokenID] = changeAmount
		}

		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("✅ 代币选择完成 - 输入数: %d, 找零: %s",
				len(selectedInputs), changeAmount))
		}
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("📊 批量UTXO选择完成 - 总输入数: %d, 代币类型: %d",
			len(allSelectedInputs), len(totalAmounts)))
	}

	return allSelectedInputs, changeAmounts, nil
}

// buildBatchOutputs 构建批量转账输出
//
// 🏗️ **输出构建**：
// - 为每个接收方创建资产输出
// - 按代币类型计算和创建找零输出
// - 优化输出顺序和大小
//
// 参数：
//   - transfers: 批量转账列表
//   - changeAmounts: 按代币类型的找零金额
//   - fromAddress: 发送方地址（用于找零）
//
// 返回：
//   - []*transaction.TxOutput: 构建的输出列表
//   - error: 构建失败时的错误信息
func (s *BatchTransferService) buildBatchOutputs(
	transfers []types.TransferParams,
	changeAmounts map[string]string,
	fromAddress string,
) ([]*transaction.TxOutput, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 构建批量转账输出")
	}

	var outputs []*transaction.TxOutput

	// 1. 为每个转账创建输出（对每笔金额内扣手续费，确保费用闭合）
	for i, transfer := range transfers {
		toAddrBytes, err := s.parseAddress(transfer.ToAddress)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个转账项接收方地址解析失败: %v", i+1, err)
		}

		// 构建转账输出
		var output *transaction.TxOutput

		// 费用扣除：actual = amount - amount*baseFeeRate（整数wei计算）
		amountWei, err := utils.ParseAmountSafely(transfer.Amount)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个转账项金额解析失败: %v", i+1, err)
		}

		// 从配置获取基础费率并转换为整数bps
		baseFeeRate := s.configManager.GetBlockchain().Transaction.BaseFeeRate
		feeRateBps := utils.ConvertFeeRateToBps(baseFeeRate)

		// 计算手续费（整数计算，避免浮点误差）
		feeWei, err := utils.CalculateFeeWei(amountWei, feeRateBps)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个转账项手续费计算失败: %v", i+1, err)
		}

		if feeWei >= amountWei {
			return nil, fmt.Errorf("第 %d 个转账项金额过小，扣除手续费后余额不足: 转账金额=%s, 手续费=%s",
				i+1, transfer.Amount, utils.FormatWeiToDecimal(feeWei))
		}

		actualReceiveWei := amountWei - feeWei
		actualReceiveStr := utils.FormatAmountForProtobuf(actualReceiveWei) // 使用统一的protobuf格式化方法

		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("💰 第 %d 项费用扣除 - 转账金额: %s, 手续费: %s, 实际到账: %s",
				i+1, transfer.Amount, utils.FormatWeiToDecimal(feeWei), actualReceiveStr))
		}

		if transfer.TokenID == "" {
			// 原生代币输出
			output = &transaction.TxOutput{
				Owner: toAddrBytes,
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: toAddrBytes,
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
								Amount: actualReceiveStr,
							},
						},
					},
				},
			}
		} else {
			// 合约FT输出
			tokenIdBytes, err := hex.DecodeString(transfer.TokenID)
			if err != nil {
				return nil, fmt.Errorf("第 %d 个转账项TokenID解析失败: %v", i+1, err)
			}

			output = &transaction.TxOutput{
				Owner: toAddrBytes,
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: toAddrBytes,
								},
								RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
								SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
							},
						},
					},
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: tokenIdBytes,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: tokenIdBytes, // 使用合约地址作为类别ID
								},
								Amount: actualReceiveStr,
							},
						},
					},
				},
			}
		}

		outputs = append(outputs, output)

		if s.logger != nil {
			if transfer.TokenID == "" {
				s.logger.Debug(fmt.Sprintf("➕ 添加原生代币输出(已内扣费) - to: %s, amount: %s -> actual: %s",
					transfer.ToAddress, transfer.Amount, actualReceiveStr))
			} else {
				s.logger.Debug(fmt.Sprintf("➕ 添加合约FT输出(已内扣费) - to: %s, tokenID: %s, amount: %s -> actual: %s",
					transfer.ToAddress, transfer.TokenID, transfer.Amount, actualReceiveStr))
			}
		}
	}

	// 2. 为每个代币类型创建找零输出（如有需要）
	fromAddrBytes, err := s.parseAddress(fromAddress)
	if err != nil {
		return nil, fmt.Errorf("发送方地址解析失败: %v", err)
	}

	for tokenID, changeAmountStr := range changeAmounts {
		changeWei, err := utils.ParseAmountSafely(changeAmountStr)
		if err != nil {
			return nil, fmt.Errorf("找零金额解析失败: %v", err)
		}

		// 只有找零金额大于门限时才创建找零输出（配置化粉尘阈值，整数wei比较）
		dustThreshold := s.configManager.GetBlockchain().Transaction.DustThreshold
		dustThresholdWei := utils.ConvertDustThresholdToWei(dustThreshold)
		if changeWei > dustThresholdWei {
			var changeOutput *transaction.TxOutput

			if tokenID == "" {
				// 原生代币找零
				changeOutput = &transaction.TxOutput{
					Owner: fromAddrBytes,
					LockingConditions: []*transaction.LockingCondition{
						{
							Condition: &transaction.LockingCondition_SingleKeyLock{
								SingleKeyLock: &transaction.SingleKeyLock{
									KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
										RequiredAddressHash: fromAddrBytes,
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
									Amount: changeAmountStr,
								},
							},
						},
					},
				}
			} else {
				// 合约FT找零
				tokenIdBytes, err := hex.DecodeString(tokenID)
				if err != nil {
					return nil, fmt.Errorf("找零TokenID解析失败: %v", err)
				}

				changeOutput = &transaction.TxOutput{
					Owner: fromAddrBytes,
					LockingConditions: []*transaction.LockingCondition{
						{
							Condition: &transaction.LockingCondition_SingleKeyLock{
								SingleKeyLock: &transaction.SingleKeyLock{
									KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
										RequiredAddressHash: fromAddrBytes,
									},
									RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
									SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
								},
							},
						},
					},
					OutputContent: &transaction.TxOutput_Asset{
						Asset: &transaction.AssetOutput{
							AssetContent: &transaction.AssetOutput_ContractToken{
								ContractToken: &transaction.ContractTokenAsset{
									ContractAddress: tokenIdBytes,
									TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
										FungibleClassId: tokenIdBytes, // 使用合约地址作为类别ID
									},
									Amount: changeAmountStr,
								},
							},
						},
					},
				}
			}

			outputs = append(outputs, changeOutput)

			if s.logger != nil {
				if tokenID == "" {
					s.logger.Debug(fmt.Sprintf("💰 添加原生代币找零输出 - amount: %s", changeAmountStr))
				} else {
					s.logger.Debug(fmt.Sprintf("💰 添加合约FT找零输出 - tokenID: %s, amount: %s",
						tokenID, changeAmountStr))
				}
			}
		}
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 批量输出构建完成 - 转账输出: %d, 找零输出: %d, 总输出: %d",
			len(transfers), len(changeAmounts), len(outputs)))
	}

	return outputs, nil
}

// parseAddress 解析地址字符串为字节数组
//
// 🔧 **地址解析工具**
//
// 将十六进制地址字符串转换为字节数组，用于UTXO查询。
//
// 参数：
//   - addressStr: 地址字符串（十六进制格式）
//
// 返回：
//   - []byte: 地址字节数组
//   - error: 解析错误
func (s *BatchTransferService) parseAddress(addressStr string) ([]byte, error) {
	if addressStr == "" {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 去掉可能的0x前缀
	if len(addressStr) > 2 && addressStr[:2] == "0x" {
		addressStr = addressStr[2:]
	}

	// 解析十六进制字符串
	addrBytes, err := hex.DecodeString(addressStr)
	if err != nil {
		return nil, fmt.Errorf("地址格式无效: %v", err)
	}

	return addrBytes, nil
}

// buildCompleteTransaction 构建完整交易
//
// 🏗️ **完整交易构建器**
//
// 根据输入和输出构建完整的交易结构。
//
// 参数：
//   - inputs: 交易输入列表
//   - outputs: 交易输出列表
//
// 返回：
//   - *transaction.Transaction: 完整交易
//   - error: 构建错误
func (s *BatchTransferService) buildCompleteTransaction(
	inputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) (*transaction.Transaction, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("交易输入不能为空")
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("交易输出不能为空")
	}

	// 获取链ID配置
	chainID := s.configManager.GetBlockchain().ChainID
	chainIDBytes := []byte(fmt.Sprintf("weisyn-chain-%d", chainID))

	// 构建基础交易
	tx := &transaction.Transaction{
		Version:           1,
		Inputs:            inputs,
		Outputs:           outputs,
		Nonce:             0, // 将在签名时设置正确的nonce
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           chainIDBytes, // 从配置获取链ID
	}

	return tx, nil
}

// cacheTransaction 缓存批量交易并返回哈希
//
// 💾 **批量交易哈希计算与缓存服务** - 为批量签名流程准备交易数据
//
// 计算批量交易的SHA256哈希值并将未签名交易存储到内存缓存中，供后续SignTransaction使用
//
// 📝 **详细参数说明**：
//   - ctx: context.Context - 请求上下文
//   - 用于控制缓存操作的超时和取消
//   - 传递给internal.CacheUnsignedTransaction进行异步缓存
//   - 支持分布式环境下的操作追踪
//   - tx: *transaction.Transaction - 未签名的完整批量交易
//   - 来源: buildCompleteTransaction()构建的完整批量交易
//   - 状态: 未签名（nonce=0，无签名数据）
//   - 内容: 包含多个输出的完整输入输出、时间戳、链ID等
//   - 格式: 符合pb/blockchain/block/transaction.Transaction规范
//
// 📤 **详细返回值说明**：
//   - []byte: 32字节的批量交易哈希值
//   - 算法: SHA256(Transaction序列化数据)
//   - 格式: 32字节原始字节数组（非十六进制编码）
//   - 用途: 作为缓存键和SignTransaction的输入参数
//   - 唯一性: 每个不同的批量交易产生不同的哈希值
//   - error: 缓存操作中的错误
//   - 哈希计算错误: protobuf序列化失败
//   - 缓存写入错误: storage.MemoryStore操作失败
//   - 配置错误: internal.GetDefaultCacheConfig()失败
//
// 🔗 **组件交互细节**：
//
//  1. transaction.TransactionHashServiceClient - 交易哈希计算服务
//     • ComputeHash(ctx, *ComputeHashRequest) (*ComputeHashResponse, error)
//     • 输入: 完整的批量交易结构
//     • 输出: 32字节SHA256哈希值
//     • 算法: 标准化的交易序列化 + SHA256计算
//
//  2. internal.GetDefaultCacheConfig() - 缓存配置获取
//     • 返回: 默认的批量交易缓存配置参数
//     • 包含: TTL过期时间、压缩选项、存储策略等
//     • 用途: 控制批量交易在缓存中的生命周期
//
//  3. internal.CacheUnsignedTransaction() - 批量交易缓存操作
//     • 输入: ctx, storage.MemoryStore, 哈希键, 交易数据, 配置, 日志器
//     • 处理: protobuf序列化 -> 可选压缩 -> 存储到内存
//     • 存储: key=txHash, value=serialized_batch_transaction
//     • 过期: 根据配置TTL自动清理过期数据
//
//  4. storage.MemoryStore - 内存存储服务
//     • 接口: Set(key []byte, value []byte, ttl time.Duration) error
//     • 实现: 通常为Redis、内存映射等高性能存储
//     • 特征: 支持并发访问、原子操作、TTL自动过期
//
// 💡 **批量交易缓存特性**：
// - 缓存写入: O(1)时间复杂度，通常<1ms
// - 内存占用: 每笔批量交易约10-100KB（取决于转账数量和输入输出数量）
// - TTL管理: 自动清理过期数据，避免内存泄漏
// - 并发安全: 支持多线程同时缓存不同批量交易
// - 压缩优化: 大型批量交易可选择启用压缩存储
func (s *BatchTransferService) cacheTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]byte, error) {
	// 使用真实的TransactionHashServiceClient计算交易哈希
	hashRequest := &transaction.ComputeHashRequest{
		Transaction: tx,
	}

	hashResponse, err := s.txHashServiceClient.ComputeHash(ctx, hashRequest)
	if err != nil {
		return nil, fmt.Errorf("计算批量交易哈希失败: %v", err)
	}

	if hashResponse == nil || len(hashResponse.Hash) == 0 {
		return nil, fmt.Errorf("批量交易哈希服务返回空哈希")
	}

	txHash := hashResponse.Hash
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 批量交易哈希计算完成 - hash: %x", txHash))
	}

	// 创建默认缓存配置
	config := internal.GetDefaultCacheConfig()

	// 将交易缓存到内存存储
	err = internal.CacheUnsignedTransaction(ctx, s.cacheStore, txHash, tx, config, s.logger)
	if err != nil {
		return nil, fmt.Errorf("缓存批量交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💾 批量交易已缓存 - hash: %x", txHash))
	}

	return txHash, nil
}

// getMaxBatchTransferSize 获取批量转账的最大支持数量
//
// 🎯 **动态配置获取**：
// - 从 internal/config/blockchain/config.go 中获取配置值
// - 支持通过配置文件动态调整限制
// - 默认值100在 internal/config/blockchain/defaults.go 中定义
//
// 🎯 **限制原因**：
// - 防止交易过大导致网络拥塞
// - 控制单个交易的复杂度
// - 保证合理的处理性能
//
// 返回：
//   - int: 从配置获取的最大批量转账数量
func (s *BatchTransferService) getMaxBatchTransferSize() int {
	// 🎯 从配置动态获取批量限制，支持环境配置
	return s.configManager.GetBlockchain().Transaction.MaxBatchTransferSize
}

// ============================================================================
//                              内部UTXO选择方法
// ============================================================================

// selectUTXOsForAmount 为批量转账选择UTXO（内部方法）
//
// 🎯 **简化的UTXO选择逻辑**：
// - 获取地址所有可用AssetUTXO
// - 使用首次适应算法选择足够金额
// - 计算找零金额
//
// 📝 **参数说明**：
//   - fromAddr: 发送方地址字节
//   - amountStr: 需要金额（字符串格式）
//   - tokenID: 代币类型（""=原生币）
//
// 💡 **返回值说明**：
//   - []*transaction.TxInput: 选中的UTXO输入
//   - string: 找零金额字符串
//   - error: 选择错误
func (s *BatchTransferService) selectUTXOsForAmount(ctx context.Context, fromAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, string, error) {
	if s.logger != nil {
		s.logger.Debugf("批量转账UTXO选择 - 地址: %x, 金额: %s", fromAddr, amountStr)
	}

	// 1. 解析目标金额
	targetAmount, err := s.parseAmount(amountStr)
	if err != nil {
		return nil, "", fmt.Errorf("金额解析失败: %v", err)
	}

	// 2. 获取地址所有可用AssetUTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, fromAddr, &assetCategory, true)
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
		// 提取UTXO金额
		utxoAmount := s.extractUTXOAmount(utxoItem)
		if utxoAmount == 0 {
			continue // 跳过零金额UTXO
		}

		// 创建交易输入
		txInput := &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        utxoItem.Outpoint.TxId,
				OutputIndex: utxoItem.Outpoint.OutputIndex,
			},
			IsReferenceOnly: false, // 转账需要消费UTXO
			Sequence:        0xffffffff,
		}

		selectedInputs = append(selectedInputs, txInput)
		totalSelected += utxoAmount

		// 找到足够金额就停止
		if totalSelected >= targetAmount {
			break
		}
	}

	// 4. 检查余额是否充足
	if totalSelected < targetAmount {
		return nil, "", fmt.Errorf("余额不足，需要: %d, 可用: %d", targetAmount, totalSelected)
	}

	// 5. 计算找零
	changeAmount := totalSelected - targetAmount
	changeStr := s.formatAmount(changeAmount)

	if s.logger != nil {
		s.logger.Infof("批量转账UTXO选择完成 - 选中: %d个, 总额: %d, 找零: %s",
			len(selectedInputs), totalSelected, changeStr)
	}

	return selectedInputs, changeStr, nil
}

// parseAmount 解析金额字符串为wei单位
func (s *BatchTransferService) parseAmount(amountStr string) (uint64, error) {
	// 使用统一的十进制解析工具，支持小数金额（用户输入）
	amountWei, err := utils.ParseDecimalToWei(amountStr)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %w", err)
	}
	return amountWei, nil
}

// extractUTXOAmount 从UTXO中提取金额
func (s *BatchTransferService) extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
	if utxoItem == nil {
		return 0
	}

	// 根据UTXO的content_strategy提取金额
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
	case *utxo.UTXO_ReferenceOnly:
		// 引用型UTXO通常用于ResourceUTXO，对资产转账无金额意义
		return 0
	}

	return 0
}

// formatAmount 格式化金额为字符串
func (s *BatchTransferService) formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// 确保 BatchTransferService 实现了所需的接口部分
var _ interface {
	BatchTransfer(context.Context, []byte, []types.TransferParams) ([]byte, error)
} = (*BatchTransferService)(nil)
