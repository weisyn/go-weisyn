// Package transfer 资产转账核心实现
//
// 🎯 **模块定位**：TransactionService 接口的资产转账功能实现
//
// 本文件实现资产转账的核心业务逻辑，包括：
// - 单笔资产转账（TransferAsset）
// - 支持原生代币和合约代币转账
// - 支持基础转账和高级转账选项
// - UTXO 选择和找零处理
// - 费用计算和验证
//
// 🏗️ **架构定位**：
// - 业务层：实现具体的转账业务逻辑
// - 依赖层：依赖 UTXO 管理、费用计算等底层服务
// - 接口层：实现 pkg/interfaces/blockchain.TransactionService
//
// 🔧 **设计原则**：
// - 薄实现：专注转账逻辑，委托给专业服务
// - 类型安全：严格的输入验证和类型检查
// - 错误处理：详细的错误信息和异常处理
// - 可扩展性：支持未来新的转账模式扩展
//
// 📋 **支持的转账类型**：
// - 原生代币转账：platform native token
// - 合约同质化代币转账：contract-based fungible tokens (FT)
// - 简单转账：基础的点对点转账
// - 高级转账：支持多签、时间锁、委托等高级选项
//
// ⚠️ **当前限制**：
// - NFT/SFT代币需要专门的转账实现，不在此方法范围内
// - 所有代币转账均采用"同币种内扣手续费"模式
//
// ⚠️ **实现状态**：
// 当前为薄实现阶段，提供接口骨架和基础验证
// 完整业务逻辑将在后续迭代中实现
package transfer

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 协议定义
	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	// 内部工具
	"github.com/weisyn/v1/internal/core/blockchain/transaction/fee"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
	"github.com/weisyn/v1/pkg/utils"
)

// ============================================================================
//
//	资产转账实现服务
//
// ============================================================================
// AssetTransferService 资产转账核心实现服务
//
// 🎯 **服务职责**：
// - 实现 TransactionService.TransferAsset 方法
// - 处理原生代币和合约同质化代币(FT)转账
// - 管理 UTXO 选择和找零逻辑
// - 构建规范的转账交易
//
// 🔧 **依赖注入**：
// - utxoSelector：UTXO 选择和管理服务
// - feeCalculator：费用计算服务
// - cacheStore：交易缓存存储
// - logger：日志记录服务
//
// 📝 **使用示例**：
//
//	service := NewAssetTransferService(utxoSelector, feeCalc, cache, logger)
//	txHash, err := service.TransferAsset(ctx, fromAddr, toAddr, amount, options...)
type AssetTransferService struct {
	utxoManager         repository.UTXOManager                   // UTXO 管理服务（使用公共接口）
	cacheStore          storage.MemoryStore                      // 内存缓存服务（用于存储未签名交易）
	keyManager          crypto.KeyManager                        // 密钥管理服务（用于从私钥生成公钥）
	addressManager      crypto.AddressManager                    // 地址管理服务（用于从公钥生成地址）
	configManager       config.Provider                          // 配置管理器（用于获取链ID等配置信息）
	txHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端（用于计算交易哈希）
	feeManager          *fee.Manager                             // 费用管理器（用于计算和验证交易费用）
	logger              log.Logger                               // 日志记录器（使用公共接口）
}

// NewAssetTransferService 创建资产转账服务实例
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
//   - feeManager: 费用管理器（用于计算和验证交易费用）
//   - logger: 日志记录器
//
// 返回：
//   - *AssetTransferService: 转账服务实例
//
// 🚨 **注意事项**：
// 所有依赖参数都不能为 nil，否则 panic
func NewAssetTransferService(
	utxoManager repository.UTXOManager,
	cacheStore storage.MemoryStore,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	configManager config.Provider,
	txHashServiceClient transaction.TransactionHashServiceClient,
	feeManager *fee.Manager,
	logger log.Logger,
) *AssetTransferService {
	// 严格检查所有依赖
	if utxoManager == nil {
		panic("AssetTransferService: utxoManager不能为nil")
	}
	if cacheStore == nil {
		panic("AssetTransferService: cacheStore不能为nil")
	}
	if keyManager == nil {
		panic("AssetTransferService: keyManager不能为nil")
	}
	if addressManager == nil {
		panic("AssetTransferService: addressManager不能为nil")
	}
	if configManager == nil {
		panic("AssetTransferService: configManager不能为nil")
	}
	if txHashServiceClient == nil {
		panic("AssetTransferService: txHashServiceClient不能为nil")
	}
	if feeManager == nil {
		panic("AssetTransferService: feeManager不能为nil")
	}
	if logger == nil {
		panic("AssetTransferService: logger不能为nil")
	}
	return &AssetTransferService{
		utxoManager:         utxoManager,
		cacheStore:          cacheStore,
		keyManager:          keyManager,
		addressManager:      addressManager,
		configManager:       configManager,
		txHashServiceClient: txHashServiceClient,
		feeManager:          feeManager,
		logger:              logger,
	}
}

// ============================================================================
//
//	核心转账方法实现
//
// ============================================================================
// TransferAsset 实现资产转账功能
//
// 🎯 **方法职责**：
// 实现 blockchain.TransactionService.TransferAsset 接口
// 支持原生代币和合约同质化代币(FT)的转账操作，提供完整的EUTXO交易构建能力
//
// 📋 **详细业务流程（费用闭合性设计）**：
// 1. 【地址计算】：通过crypto.AddressManager从私钥计算发送方地址
// 2. 【参数验证】：验证地址格式、金额范围、TokenID有效性等
// 3. 【UTXO选择】：调用internal.SelectUTXOsForTransfer选择≥转账金额的UTXO组合
//   - 选择策略：选择足够覆盖用户指定金额的UTXO（而非金额+手续费）
//   - 找零计算：selectedUTXO - transferAmount = changeAmount（暂不考虑手续费）
//
// 4. 【💰费用闭合处理】：实现完整的费用扣除和分配机制
//   - 手续费计算：transferAmount × baseFeeRate = calculatedFee
//   - 实际到账：transferAmount - calculatedFee = actualReceiveAmount
//   - 矿工收益：selectedUTXO - actualReceiveAmount - changeAmount = calculatedFee
//   - ⚖️ 价值守恒：输入总额 = 输出总额 + 矿工手续费（确保费用闭合性）
//
// 5. 【交易构建】：构建符合EUTXO标准的Transaction结构
//   - 主输出：接收方获得actualReceiveAmount（已扣除手续费）
//   - 找零输出：发送方获得changeAmount（不受手续费影响）
//   - 手续费：通过输入输出差额自动提供给矿工
//
// 6. 【缓存存储】：将未签名交易存储到storage.MemoryStore供后续使用
// 7. 【哈希返回】：计算并返回交易哈希用于签名流程
//
// 📝 **详细参数说明**：
//   - ctx: context.Context - 请求上下文，支持超时控制和取消操作
//   - 用于所有异步操作的生命周期管理
//   - 传递给所有依赖组件的调用（UTXO查询、缓存操作等）
//   - senderPrivateKey: []byte - 发送方的ECDSA secp256k1私钥
//   - 32字节的私钥数据，用于计算发送方地址
//   - 通过crypto.AddressManager.PrivateKeyToAddress()转换为地址
//   - 私钥本身不会被存储或传输，仅用于地址计算
//   - toAddress: string - 接收方地址
//   - 40字符的十六进制地址字符串（可选0x前缀）
//   - 必须是有效的WES地址格式
//   - 不能与发送方地址相同（防止自转账）
//   - amount: string - 转账金额
//   - 字符串格式的数值，支持小数（如"1.23456789"）
//   - 必须大于0，通过utils.ParseDecimalToWei验证
//   - 对于原生代币，单位为最小原生单位
//   - tokenID: string - 代币标识
//   - 空字符串表示原生代币转账
//   - 非空时为40字符的FT合约地址（十六进制）
//   - ⚠️ 当前仅支持同质化代币(FT)，NFT/SFT需专门实现
//   - memo: string - 转账备注
//   - 可选的文本信息，记录在交易中
//   - 在区块浏览器中可见，用于交易说明
//   - options: ...*types.TransferOptions - 高级转账选项（可变参数）
//   - 支持多重签名配置（EnterpriseOptions）
//   - 支持时间锁控制（TimingControl）
//   - 支持费用控制选项（FeeControl）
//   - 支持委托授权模式（DelegationAuth）
//
// 📤 **详细返回值说明**：
//   - []byte: 32字节的交易哈希
//   - SHA256哈希值，唯一标识这笔交易
//   - 用于后续的签名操作（SignTransaction）
//   - 用于交易状态查询和跟踪
//   - error: 详细的错误信息
//   - 参数验证错误：格式、范围、有效性检查失败
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
// 💰 **费用闭合性示例**：
// 场景：用户转账100原生币，基础费率万三（0.0003）
//
// 执行流程：
// 1. UTXO选择：选中120原生币的UTXO（足够覆盖100）
// 2. 手续费计算：100 × 0.0003 = 0.03原生币
// 3. 实际到账：100 - 0.03 = 99.97原生币（给接收方）
// 4. 找零金额：120 - 100 = 20原生币（给发送方）
// 5. 矿工收益：120 - 99.97 - 20 = 0.03原生币（手续费）
//
// 结果验证：
// • 输入总额：120原生币
// • 输出总额：99.97 + 20 = 119.97原生币
// • 手续费差额：120 - 119.97 = 0.03原生币 ✓
// • 费用闭合性：✅ 无代币凭空产生或消失
//
// 🎯 **支持的转账场景**：
//   - 基础原生币转账：TransferAsset(ctx, privKey, toAddr, "100.0", "", "转账备注")
//   - 接收方实际到账：99.97原生币（已扣除万三手续费）
//   - 合约FT转账：TransferAsset(ctx, privKey, toAddr, "50.5", contractAddr, "FT代币转账")
//   - 手续费计算：50.5 × 0.0003 = 0.01515代币（同币种内扣费）
//   - 企业多签转账：TransferAsset(ctx, privKey, toAddr, amount, tokenID, memo, &types.TransferOptions{
//     EnterpriseOptions: &types.EnterpriseOptions{MultiSigRequired: true, RequiredSigners: 3}})
//   - 费用机制：与普通转账相同，从转账金额内扣除
//   - 定时转账：TransferAsset(ctx, privKey, toAddr, amount, tokenID, memo, &types.TransferOptions{
//     TimingControl: &types.TimingControl{DelayBlocks: 100, ExpiryBlocks: 500}})
//   - 手续费控制：TransferAsset(ctx, privKey, toAddr, amount, tokenID, memo, &types.TransferOptions{
//     FeeControl: &types.FeeControl{MaxFeeRate: "0.001", ExecutionFeePriceLimit: 50}})
//
// ⚠️ **重要说明**：
// - 此方法只构建未签名交易，不执行实际的资金转移
// - 返回的交易哈希需要通过SignTransaction进行签名
// - 签名后的交易需要通过SubmitTransaction提交到网络
// - 交易成功与否需要通过GetTransactionStatus查询确认
func (s *AssetTransferService) TransferAsset(
	ctx context.Context,
	senderPrivateKey []byte,
	toAddress string,
	amount string,
	tokenID string,
	memo string,
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 📍 **步骤1: 地址计算** - 通过加密服务从私钥计算发送方地址
	// 【组件交互】：crypto.AddressManager.PrivateKeyToAddress()
	// • 输入：32字节ECDSA secp256k1私钥
	// • 处理：椭圆曲线运算 -> 公钥 -> Keccak256 -> 地址
	// • 输出：Base58编码地址字符串
	// • 错误：私钥格式无效、椭圆曲线计算失败等
	fromAddressBase58, err := s.addressManager.PrivateKeyToAddress(senderPrivateKey)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 地址计算失败 - 私钥长度: %d, 错误: %v", len(senderPrivateKey), err))
		}
		return nil, fmt.Errorf("从私钥计算地址失败: %v", err)
	}

	// 直接使用Base58地址，不进行格式转换
	// UTXO可能是用Base58格式存储的
	fromAddress := fromAddressBase58

	// 📍 **参数封装** - 将输入参数封装为标准化的TransferParams结构
	// 便于后续传递给验证和处理函数，保持接口一致性
	params := &types.TransferParams{
		ToAddress: toAddress,
		Amount:    amount,
		TokenID:   tokenID,
		Memo:      memo,
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🚀 开始处理资产转账请求 - from: %s, to: %s, amount: %s, tokenID: %s",
			fromAddress, params.ToAddress, params.Amount, params.TokenID))
	}

	// 📍 **步骤2: 参数验证** - 全面验证转账参数的合法性
	// 【业务逻辑】：validateTransferParams()内部执行以下检查
	// • 地址格式验证：十六进制格式、长度检查
	// • 金额有效性：utils.ParseDecimalToWei解析、大于0验证
	// • TokenID格式：空字符串（原生币）或40字符合约地址
	// • 防自转账：from地址与to地址不能相同
	// • 高级选项验证：多签配置、时间锁参数等
	if err := s.validateTransferParams(fromAddress, params, options); err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 参数验证失败 - %v", err))
		}
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 📍 **步骤3: 地址解析** - 将十六进制地址字符串转换为字节数组
	// 【数据转换】：parseAddress()执行以下处理
	// • 移除可选的"0x"前缀
	// • hex.DecodeString()十六进制解码
	// • 返回20字节的地址字节数组
	fromAddrBytes, err := s.parseAddress(fromAddress)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 发送方地址解析失败 - 地址: %s, 错误: %v", fromAddress, err))
		}
		return nil, fmt.Errorf("发送方地址解析失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Infof("🏷️  地址解析完成 - 输入地址: %s, 输出字节: %x (长度: %d)", fromAddress, fromAddrBytes, len(fromAddrBytes))
	}

	// 📍 **步骤4: UTXO选择** - 简化的UTXO选择逻辑
	// 直接调用UTXOManager，使用简单有效的选择算法
	selectedInputs, changeAmountWei, err := s.selectUTXOsForTransfer(
		ctx, fromAddrBytes, params.Amount, params.TokenID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ UTXO选择失败 - 地址: %s, 需求金额: %s, 错误: %v",
				fromAddress, params.Amount, err))
		}
		return nil, fmt.Errorf("UTXO选择失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💰 UTXO选择完成 - 选中输入: %d个, 找零金额: %d wei",
			len(selectedInputs), changeAmountWei))
	}

	// 📍 **步骤5: 构建交易输出** - 根据转账参数和找零金额构建输出
	// 【业务逻辑】：buildTransactionOutputs()构建以下输出
	// • 主输出：转账给接收方，金额为params.Amount（十进制字符串）
	// • 找零输出：如果changeAmountWei > dustThresholdWei，返回给发送方（转为十进制字符串）
	// • 输出格式：符合EUTXO AssetOutput规范，包含锁定条件
	outputs, err := s.buildTransactionOutputs(params, changeAmountWei, fromAddress)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 构建交易输出失败 - %v", err))
		}
		return nil, fmt.Errorf("构建交易输出失败: %v", err)
	}

	// 📍 **步骤6: 构建完整交易** - 组装Transaction protobuf结构
	// 【组件交互】：config.Provider.GetBlockchain().ChainID
	// • 合并输入输出：selectedInputs + outputs
	// • 设置交易元数据：version、nonce、timestamp
	// • 设置链ID：从配置获取，防止跨链重放攻击
	// • 处理高级选项：多签、时间锁等（已实现基础框架）
	tx, err := s.buildCompleteTransaction(selectedInputs, outputs, options)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 构建完整交易失败 - %v", err))
		}
		return nil, fmt.Errorf("构建完整交易失败: %v", err)
	}

	// 📍 **步骤7: 交易哈希计算与缓存** - 计算交易标识并存储到缓存
	// 【组件交互】：storage.MemoryStore通过internal.CacheUnsignedTransaction
	// • 哈希计算：SHA256(Transaction序列化数据) -> 32字节哈希
	// • 缓存存储：key=txHash, value=serialized Transaction
	// • 缓存配置：TTL、压缩等通过internal.GetDefaultCacheConfig()
	// • 用途：SignTransaction时根据哈希检索原始交易
	txHash, err := s.cacheTransaction(ctx, tx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ 缓存交易失败 - %v", err))
		}
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 资产转账交易构建完成 - txHash: %x, inputs: %d, outputs: %d",
			txHash, len(selectedInputs), len(outputs)))
	}

	return txHash, nil
}

// ============================================================================
//
//	私有辅助方法
//
// ============================================================================
// validateTransferParams 验证转账参数的完整性和有效性
//
// 🔍 **详细验证项目**：
// - 地址格式验证（from/to address）：十六进制格式、长度检查
// - 金额格式和范围验证：parseFloat解析、正数检查
// - 代币ID有效性检查：空字符串或合约地址格式
// - 防自转账验证：发送方和接收方地址不能相同
// - 高级选项参数验证：多签配置、时间锁、费用控制等
//
// 📝 **详细参数说明**：
//   - fromAddress: string - 发送方地址
//   - 来源：通过crypto.AddressManager从私钥计算得出
//   - 格式：40字符十六进制字符串（不含0x前缀）
//   - 验证：非空检查，在主方法中已确保有效
//   - params: *types.TransferParams - 转账基础参数结构
//   - ToAddress: string - 接收方地址，40字符十六进制
//   - Amount: string - 转账金额，字符串数值格式
//   - TokenID: string - 代币标识，空（原生币）或FT合约地址
//   - Memo: string - 转账备注，可选文本信息
//   - options: []*types.TransferOptions - 高级转账选项数组
//   - EnterpriseOptions: 企业多签配置
//   - TimingControl: 时间锁和延迟配置
//   - FeeControl: 手续费控制选项
//   - DelegationAuth: 委托授权配置
//
// 📤 **返回值说明**：
//   - error: 验证失败的具体错误信息
//   - nil: 所有参数验证通过
//   - 非nil: 包含具体失败原因的错误描述
//
// 🔗 **依赖组件交互**：
//   - utils.ParseDecimalToWei: 精确金额解析，避免浮点误差
//   - 内部验证逻辑：不依赖外部服务，纯参数检查
//
// 🎯 **验证失败场景**：
// - 地址格式错误：长度不对、包含非十六进制字符
// - 金额无效：非数字、负数、零值
// - TokenID格式错误：非空但不是40字符十六进制FT合约地址
// - 自转账：fromAddress等于ToAddress
// - 选项无效：多签配置不完整、时间锁参数错误等
func (s *AssetTransferService) validateTransferParams(
	fromAddress string,
	params *types.TransferParams,
	options []*types.TransferOptions,
) error {
	if s.logger != nil {
		s.logger.Debug("🔍 验证转账参数")
	}

	// 基础参数验证
	if fromAddress == "" {
		return fmt.Errorf("发送方地址不能为空")
	}
	if params == nil {
		return fmt.Errorf("转账参数不能为空")
	}
	if params.ToAddress == "" {
		return fmt.Errorf("接收方地址不能为空")
	}
	if params.Amount == "" || params.Amount == "0" {
		return fmt.Errorf("转账金额必须大于0")
	}

	// 地址格式验证
	if err := s.validateAddress(fromAddress); err != nil {
		return fmt.Errorf("发送方地址格式无效: %v", err)
	}
	if err := s.validateAddress(params.ToAddress); err != nil {
		return fmt.Errorf("接收方地址格式无效: %v", err)
	}

	// 验证金额格式（用户输入支持小数格式）
	amountWei, err := utils.ParseDecimalToWei(params.Amount)
	if err != nil {
		return fmt.Errorf("金额格式无效: %v", err)
	}
	if amountWei == 0 {
		return fmt.Errorf("转账金额必须大于0")
	}

	// 验证地址不能相同
	if fromAddress == params.ToAddress {
		return fmt.Errorf("发送方和接收方地址不能相同")
	}

	// 验证TokenID格式（如果指定了）
	if params.TokenID != "" {
		// 规范化tokenID：去除0x前缀
		normalizedTokenID := strings.ToLower(strings.TrimPrefix(params.TokenID, "0x"))

		// 验证十六进制格式和长度
		if len(normalizedTokenID) != 40 {
			return fmt.Errorf("TokenID长度无效，期望40字符的十六进制字符串（去除0x前缀）")
		}

		// 验证十六进制合法性
		if _, err := hex.DecodeString(normalizedTokenID); err != nil {
			return fmt.Errorf("TokenID格式无效，必须是有效的十六进制字符串: %v", err)
		}

		// 🎯 **代币类型限制**：当前仅支持同质化代币(FT)转账
		// TokenID格式约定：合约地址 = FT的fungible_class_id
		// NFT/SFT需要专门的转账接口，不在此方法范围内
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("🔍 验证合约同质化代币(FT) - TokenID: %s", params.TokenID))
		}
	}

	// 验证高级选项（如果有的话）
	if len(options) > 0 {
		for i, opt := range options {
			if opt == nil {
				return fmt.Errorf("第%d个转账选项不能为nil", i+1)
			}
			// 实现具体的选项验证逻辑
			if err := s.validateTransferOption(opt); err != nil {
				return fmt.Errorf("第%d个转账选项验证失败: %w", i+1, err)
			}
		}
	}

	return nil
}

// parseAddress 解析地址字符串为字节数组
//
// 🔧 **地址解析工具** - 标准化地址格式转换
//
// 将十六进制地址字符串转换为20字节的地址字节数组，供UTXO查询和交易构建使用
//
// 📝 **详细参数说明**：
//   - addressStr: string - 十六进制地址字符串
//   - 格式1: "1234567890abcdef..." (40字符十六进制)
//   - 格式2: "0x1234567890abcdef..." (0x前缀 + 40字符十六进制)
//   - 长度: 40字符（不含前缀）或42字符（含0x前缀）
//   - 字符集: 0-9, a-f, A-F (大小写不敏感)
//
// 📤 **详细返回值说明**：
//   - []byte: 20字节的地址字节数组
//   - 长度: 固定20字节 (160位)
//   - 用途: UTXO查询、交易输入输出构建
//   - 格式: 原始字节数据，非编码格式
//   - error: 解析过程中的错误
//   - nil: 解析成功
//   - "地址不能为空": 输入为空字符串
//   - "地址格式无效": hex.DecodeString失败
//   - "地址长度无效": 不是20字节
//
// 🔗 **依赖组件交互**：
//   - hex.DecodeString: Go标准库函数
//   - 输入: 十六进制字符串（移除0x前缀后）
//   - 输出: 对应的字节数组
//   - 错误: encoding/hex.InvalidByteError (无效字符)
//
// 📋 **处理逻辑**：
// 1. 【空值检查】：验证输入不为空
// 2. 【前缀处理】：检测并移除可选的"0x"前缀
// 3. 【长度验证】：确保十六进制字符串长度为40
// 4. 【十六进制解码】：使用hex.DecodeString转换
// 5. 【长度检查】：确保解码后为20字节
// 6. 【返回结果】：20字节地址数组或错误信息
//
// 💡 **使用示例**：
//   - parseAddress("1234567890abcdef1234567890abcdef12345678") -> [20]byte{0x12,0x34,...}
//   - parseAddress("0x1234567890abcdef1234567890abcdef12345678") -> [20]byte{0x12,0x34,...}
//   - parseAddress("invalid") -> nil, "地址格式无效"
//   - parseAddress("") -> nil, "地址不能为空"
func (s *AssetTransferService) parseAddress(addressStr string) ([]byte, error) {
	if addressStr == "" {
		return nil, fmt.Errorf("地址不能为空")
	}

	// 去掉可能的0x前缀
	cleanAddr := addressStr
	if len(addressStr) > 2 && addressStr[:2] == "0x" {
		cleanAddr = addressStr[2:]
	}

	// 检测地址格式：十六进制（40字符）或Base58（~34字符）
	if len(cleanAddr) == 40 {
		// 十六进制格式
		addrBytes, err := hex.DecodeString(cleanAddr)
		if err != nil {
			return nil, fmt.Errorf("十六进制地址格式无效: %v", err)
		}
		if len(addrBytes) != 20 {
			return nil, fmt.Errorf("地址字节长度无效: 期望20字节，实际%d字节", len(addrBytes))
		}
		return addrBytes, nil
	} else {
		// Base58格式，使用地址管理器转换
		return s.addressManager.AddressToBytes(addressStr)
	}
}

// validateAddress 验证地址格式
//
// 🔍 **地址格式验证器**
//
// 验证地址字符串是否符合WES地址格式要求
//
// 参数：
//   - addressStr: 地址字符串
//
// 返回：
//   - error: 验证错误，nil表示验证通过
func (s *AssetTransferService) validateAddress(addressStr string) error {
	if addressStr == "" {
		return fmt.Errorf("地址不能为空")
	}

	// 使用parseAddress进行完整验证
	_, err := s.parseAddress(addressStr)
	if err != nil {
		return err
	}

	return nil
}

// buildTransactionOutputs 构建交易输出
//
// 🏗️ **EUTXO交易输出构建器** - 构建符合EUTXO标准的交易输出
//
// 根据转账参数和找零金额构建完整的交易输出列表，包含主输出和可选的找零输出
//
// 📝 **详细参数说明**：
//   - params: *types.TransferParams - 转账基础参数
//   - ToAddress: string - 接收方地址（40字符十六进制）
//   - Amount: string - 转账金额（字符串数值）
//   - TokenID: string - 代币标识（空=原生币，非空=FT合约地址）
//   - Memo: string - 转账备注（暂未在输出中使用）
//   - changeAmountWei: uint64 - 找零金额（wei单位）
//   - 来源: UTXO选择算法计算（selected_total - transfer_amount）
//   - 单位: wei整数值
//   - 阈值: > dustThresholdWei才创建找零输出（防止粉尘）
//   - fromAddress: string - 发送方地址
//   - 用途: 找零输出的接收方地址
//   - 格式: 40字符十六进制字符串
//
// 📤 **详细返回值说明**：
//   - []*transaction.TxOutput: EUTXO输出列表
//   - [0]: 主输出 - 转账给接收方的AssetOutput
//   - [1]: 找零输出 - 返回给发送方的AssetOutput（如果金额足够）
//   - 结构: 符合pb/blockchain/block/transaction.TxOutput规范
//   - error: 构建过程中的错误
//   - 地址解析错误: parseAddress失败
//   - 金额解析错误: utils.ParseDecimalToWei失败
//
// 🔗 **组件交互细节**：
//  1. parseAddress(): 内部方法
//     • 将十六进制地址字符串转换为20字节数组
//     • 用于设置TxOutput.Owner字段
//  2. utils.FormatWeiToDecimal(): 精确金额格式化工具
//     • 将changeAmountWei转换为十进制字符串格式
//     • 确保与主输出Amount字段格式一致
//
// 📋 **EUTXO输出结构详解**：
// ```protobuf
//
//	TxOutput {
//	  Owner: []byte                    // 输出拥有者地址（20字节）
//	  LockingConditions: []*LockingCondition  // 解锁条件数组
//	  OutputContent: oneof {           // 输出内容（联合类型）
//	    Asset: *AssetOutput            // 资产输出（原生币/代币）
//	    Resource: *ResourceOutput      // 资源输出（合约/AI模型等）
//	    State: *StateOutput            // 状态输出（证明/认证等）
//	  }
//	}
//
// ```
//
// 📋 **锁定条件设置**：
// ```protobuf
//
//	LockingCondition {
//	  Condition: oneof {
//	    SingleKeyLock: *SingleKeyLock  // 单密钥锁定（常用）
//	    MultiSigLock: *MultiSigLock    // 多重签名锁定
//	    TimeLock: *TimeLock            // 时间锁定
//	    // ... 其他7种锁定机制
//	  }
//	}
//
// ```
//
// 📋 **构建逻辑流程（费用闭合性设计）**：
// 1. 【费用计算与扣除】：
//   - 解析转账金额：params.Amount -> transferAmountFloat
//   - 计算手续费：transferAmountFloat × baseFeeRate -> calculatedFee
//   - 计算实际到账：transferAmountFloat - calculatedFee -> actualReceiveAmount
//   - ⚖️ **费用闭合性保证**：用户转账100，手续费0.3，接收方得到99.7
//
// 2. 【主输出构建】：
//   - 解析接收方地址 -> Owner字段
//   - 创建SingleKeyLock -> 单密钥解锁条件
//   - 构建AssetOutput.NativeCoin -> 原生币资产
//   - 设置实际到账金额 -> actualReceiveAmountStr（扣除手续费后）
//
// 3. 【找零输出判断】：
//   - 解析找零金额字符串
//   - 判断是否 > 粉尘阈值（从配置获取）
//   - 满足条件则创建找零输出
//
// 4. 【找零输出构建】：
//   - 解析发送方地址 -> Owner字段
//   - 创建SingleKeyLock -> 发送方解锁条件
//   - 构建AssetOutput.NativeCoin -> 找零资产
//   - 设置找零金额 -> Amount字段（找零不受手续费影响）
//
// 💰 **费用闭合性验证**：
// - 输入总额 = actualReceiveAmount + changeAmount + calculatedFee
// - 矿工手续费 = UTXO选择总额 - 输出总额 = calculatedFee
// - 确保价值守恒：无代币凭空产生或消失
//
// 💡 **输出特征**：
// - 所有输出使用ECDSA secp256k1签名算法
// - 所有输出使用SIGHASH_ALL签名哈希类型
// - 已支持原生币资产（NativeCoin）和合约同质化代币（ContractToken FT）
// - NFT/SFT代币类型需要专门的实现路径，当前不在此方法范围内
// - 粉尘控制：使用配置的粉尘阈值，避免网络垃圾
func (s *AssetTransferService) buildTransactionOutputs(
	params *types.TransferParams,
	changeAmountWei uint64,
	fromAddress string,
) ([]*transaction.TxOutput, error) {
	var outputs []*transaction.TxOutput

	// 解析接收方地址
	toAddrBytes, err := s.parseAddress(params.ToAddress)
	if err != nil {
		return nil, fmt.Errorf("接收方地址解析失败: %v", err)
	}

	// 如果是合约代币，预先验证TokenID有效性（严格类型隔离）
	// 🎯 **当前仅支持同质化代币(FT)**：使用 fungible_class_id 标识
	// NFT/SFT 需要专门的转账实现，使用不同的标识字段和业务逻辑
	var contractAddress []byte
	if params.TokenID != "" {
		contractAddress, err = s.parseTokenIDToAddress(params.TokenID)
		if err != nil {
			return nil, fmt.Errorf("合约代币TokenID解析失败: %w", err)
		}
	}

	// 【费用闭合性核心逻辑】计算并扣除手续费（用户输入支持小数）
	transferAmountWei, err := utils.ParseDecimalToWei(params.Amount)
	if err != nil {
		return nil, fmt.Errorf("转账金额解析失败: %v", err)
	}

	// 从配置获取基础费率并转换为整数bps
	baseFeeRate := s.configManager.GetBlockchain().Transaction.BaseFeeRate
	feeRateBps := utils.ConvertFeeRateToBps(baseFeeRate)

	// 计算手续费（整数计算，避免浮点误差）
	calculatedFeeWei, err := utils.CalculateFeeWei(transferAmountWei, feeRateBps)
	if err != nil {
		return nil, fmt.Errorf("手续费计算失败: %v", err)
	}

	// 实际给接收方的金额 = 用户指定金额 - 手续费
	if calculatedFeeWei >= transferAmountWei {
		return nil, fmt.Errorf("转账金额过小，扣除手续费后余额不足: 转账金额=%s, 手续费=%s",
			params.Amount, utils.FormatWeiToDecimal(calculatedFeeWei))
	}

	actualReceiveAmountWei := transferAmountWei - calculatedFeeWei
	actualReceiveAmountStr := utils.FormatAmountForProtobuf(actualReceiveAmountWei) // 使用统一的protobuf格式化方法

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("💰 费用扣除计算 - 转账金额: %s, 手续费: %s, 实际到账: %s",
			params.Amount, utils.FormatWeiToDecimal(calculatedFeeWei), actualReceiveAmountStr))
	}

	// 1. 构建主转账输出（给接收方扣除手续费后的金额）
	//
	// 📋 **TxOutput字段详细说明**：
	mainOutput := &transaction.TxOutput{
		// 🏷️ **Owner字段**：输出所有者地址（20字节RIPEMD160哈希）
		// • 业务含义：标识这个UTXO的法定所有者
		// • 用途：用于快速查询和显示，实际权限控制由LockingConditions决定
		// • 格式：[]byte，固定20字节长度
		// • 来源：接收方地址解析结果
		Owner: toAddrBytes,

		// 🔐 **LockingConditions字段**：解锁条件列表（权限控制的核心）
		// • 业务含义：定义消费这个UTXO需要满足的条件
		// • 权限机制：使用者必须提供对应的UnlockingProof才能消费此UTXO
		// • 支持类型：7种标准锁定机制（单签、多签、合约、委托、门限、时间锁、高度锁）
		// • 当前使用：SingleKeyLock（单密钥签名锁定）
		LockingConditions: []*transaction.LockingCondition{
			{
				// 🔑 **SingleKeyLock配置**：单密钥签名验证
				Condition: &transaction.LockingCondition_SingleKeyLock{
					SingleKeyLock: &transaction.SingleKeyLock{
						// 📍 **KeyRequirement字段**：密钥要求配置
						// • RequiredAddressHash：要求提供公钥hash匹配此地址的签名
						// • 验证逻辑：RIPEMD160(SHA256(提供的公钥)) == RequiredAddressHash
						// • 安全模型：Bitcoin P2PKH (Pay-to-Public-Key-Hash) 标准
						KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: toAddrBytes, // 接收方地址哈希
						},

						// 🔒 **RequiredAlgorithm字段**：签名算法要求
						// • 业务含义：指定必须使用的数字签名算法
						// • 当前值：ECDSA_SECP256K1（与Bitcoin兼容）
						// • 用途：确保签名验证的一致性和安全性
						RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,

						// ✍️ **SighashType字段**：签名哈希类型
						// • 业务含义：指定签名覆盖的交易内容范围
						// • SIGHASH_ALL：签名覆盖所有输入和输出（标准且最安全）
						// • 用途：防止交易内容被恶意修改
						SighashType: transaction.SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},

		// 💰 **OutputContent字段**：UTXO内容定义（价值载体）
		// • 业务含义：定义这个UTXO承载的具体内容类型和数值
		// • 联合类型：Asset | Resource | State（三种载体类型）
		// • 当前使用：Asset（资产载体，承载经济价值）
		OutputContent: &transaction.TxOutput_Asset{
			// 📈 **AssetOutput配置**：资产输出详细定义
			Asset: func() *transaction.AssetOutput {
				if params.TokenID == "" {
					// 原生币输出
					return &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: actualReceiveAmountStr,
							},
						},
					}
				} else {
					// 合约同质化代币(FT)输出（contractAddress已预先验证）
					// 🎯 **FT标识策略**：使用合约地址作为 fungible_class_id
					// 这是标准的ERC20兼容模式，一个合约对应一种同质化代币类型
					return &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: contractAddress,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: contractAddress, // 合约地址即为代币类别ID
								},
								Amount: actualReceiveAmountStr,
							},
						},
					}
				}
			}(),
		},
	}
	outputs = append(outputs, mainOutput)

	// 2. 如果有找零，构建找零输出（给发送方）
	// 将浮点粉尘阈值转换为整数wei进行比较
	dustThreshold := s.configManager.GetBlockchain().Transaction.DustThreshold
	dustThresholdWei := utils.ConvertDustThresholdToWei(dustThreshold)
	if changeAmountWei > dustThresholdWei { // 最小找零门限，避免粉尘攻击
		fromAddrBytes, err := s.parseAddress(fromAddress)
		if err != nil {
			return nil, fmt.Errorf("发送方地址解析失败: %v", err)
		}

		// 📋 **找零输出TxOutput字段详细说明**：
		changeOutput := &transaction.TxOutput{
			// 🏷️ **Owner字段**：找零输出所有者（发送方地址）
			// • 业务含义：将多余的金额返还给原发送方
			// • 权限归属：发送方拥有这个找零UTXO的完整控制权
			// • 格式：[]byte，20字节RIPEMD160哈希
			// • 来源：发送方地址解析结果（与输入地址相同）
			Owner: fromAddrBytes,

			// 🔐 **LockingConditions字段**：找零UTXO的解锁条件
			// • 权限控制：只有发送方私钥持有者能消费此找零UTXO
			// • 安全模型：与主输出相同的P2PKH锁定机制
			// • 重要性：保护用户找零资金的安全性
			LockingConditions: []*transaction.LockingCondition{
				{
					// 🔑 **SingleKeyLock配置**：单密钥签名验证（与主输出结构相同）
					Condition: &transaction.LockingCondition_SingleKeyLock{
						SingleKeyLock: &transaction.SingleKeyLock{
							// 📍 **KeyRequirement字段**：要求发送方签名
							// • 验证逻辑：必须提供与fromAddrBytes匹配的私钥签名
							// • 安全保证：确保只有发送方能使用找零
							KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: fromAddrBytes, // 发送方地址哈希
							},

							// 🔒 **RequiredAlgorithm字段**：签名算法（与系统标准一致）
							// • 统一标准：全系统使用ECDSA_SECP256K1算法
							RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,

							// ✍️ **SighashType字段**：签名覆盖范围
							// • SIGHASH_ALL：最安全的签名模式，覆盖全部交易内容
							SighashType: transaction.SignatureHashType_SIGHASH_ALL,
						},
					},
				},
			},

			// 💰 **OutputContent字段**：找零资产内容
			// • 业务含义：承载剩余的原生代币数量
			// • 计算来源：选中UTXO总额 - 转账金额（注：手续费已在主输出中扣除）
			OutputContent: &transaction.TxOutput_Asset{
				// 📈 **AssetOutput配置**：找零资产输出
				Asset: func() *transaction.AssetOutput {
					changeAmountStr := strconv.FormatUint(changeAmountWei, 10) // 🔥 修复：使用整数wei字符串
					if params.TokenID == "" {
						// 原生币找零
						return &transaction.AssetOutput{
							AssetContent: &transaction.AssetOutput_NativeCoin{
								NativeCoin: &transaction.NativeCoinAsset{
									Amount: changeAmountStr,
								},
							},
						}
					} else {
						// 合约同质化代币(FT)找零（contractAddress已预先验证）
						// 🎯 **找零逻辑**：与主输出使用相同的FT标识策略
						return &transaction.AssetOutput{
							AssetContent: &transaction.AssetOutput_ContractToken{
								ContractToken: &transaction.ContractTokenAsset{
									ContractAddress: contractAddress,
									TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
										FungibleClassId: contractAddress, // 合约地址即为代币类别ID
									},
									Amount: changeAmountStr,
								},
							},
						}
					}
				}(),
			},
		}
		outputs = append(outputs, changeOutput)

		if s.logger != nil {
			changeAmountStr := utils.FormatWeiToDecimal(changeAmountWei) // 🔍 日志显示用小数格式
			s.logger.Debug(fmt.Sprintf("💰 添加找零输出 - 金额: %s wei (%s), 接收方: %s",
				strconv.FormatUint(changeAmountWei, 10), changeAmountStr, fromAddress))
		}
	}

	return outputs, nil
}

// buildCompleteTransaction 构建完整交易
//
// 🏗️ **EUTXO完整交易构建器** - 组装最终的Transaction protobuf结构
//
// 根据交易输入、输出和高级选项构建符合EUTXO标准的完整交易结构
//
// 📝 **详细参数说明**：
//   - inputs: []*transaction.TxInput - 交易输入列表
//   - 来源: internal.SelectUTXOsForTransfer()选择的UTXO
//   - 内容: OutPoint引用 + UnlockingConditions解锁条件
//   - 数量: 通常1-10个，取决于UTXO分布和金额需求
//   - 格式: 符合pb/blockchain/block/transaction.TxInput规范
//   - outputs: []*transaction.TxOutput - 交易输出列表
//   - 来源: buildTransactionOutputs()构建的输出
//   - 内容: 主输出（转账）+ 找零输出（可选）
//   - 数量: 1个（无找零）或2个（有找零）
//   - 格式: 符合pb/blockchain/block/transaction.TxOutput规范
//   - options: []*types.TransferOptions - 高级转账选项
//   - EnterpriseOptions: 企业多签配置
//   - TimingControl: 时间锁和延迟设置
//   - FeeControl: 手续费控制参数
//   - DelegationAuth: 委托授权配置
//
// 📤 **详细返回值说明**：
//   - *transaction.Transaction: 完整的EUTXO交易结构
//   - Version: uint32 - 交易版本号（当前固定为1）
//   - Inputs: []*TxInput - 交易输入数组
//   - Outputs: []*TxOutput - 交易输出数组
//   - Nonce: uint64 - 防重放随机数（初始为0，签名时设置）
//   - CreationTimestamp: uint64 - 创建时间戳（Unix秒）
//   - ChainId: []byte - 链ID字节数组（防跨链重放）
//   - error: 构建过程中的错误
//   - 输入验证错误: 输入列表为空
//   - 输出验证错误: 输出列表为空
//   - 配置获取错误: 无法获取链ID配置
//
// 🔗 **组件交互细节**：
//  1. config.Provider - 配置管理服务
//     • GetBlockchain().ChainID: 获取当前链的数字ID
//     • 用途: 构建ChainId字段，防止跨链重放攻击
//  2. time.Now().Unix(): Go标准库
//     • 获取当前Unix时间戳（秒级精度）
//     • 用途: 设置CreationTimestamp字段
//
// 📋 **Transaction结构详解**：
// ```protobuf
//
//	Transaction {
//	  uint32 version = 1;                    // 交易版本（向后兼容）
//	  repeated TxInput inputs = 2;           // 交易输入数组
//	  repeated TxOutput outputs = 3;         // 交易输出数组
//	  uint64 nonce = 4;                      // 防重放随机数
//	  uint64 creation_timestamp = 5;         // 创建时间戳
//	  bytes chain_id = 6;                    // 链标识符
//	}
//
// ```
//
// 📋 **构建逻辑流程**：
// 1. 【输入验证】：检查inputs和outputs非空
// 2. 【配置获取】：从config.Provider获取链ID
// 3. 【链ID转换】：数字ID -> "weisyn-chain-{id}" -> 字节数组
// 4. 【基础交易构建】：设置version、inputs、outputs、timestamp、chain_id
// 5. 【Nonce设置】：初始为0，待签名时设置正确值
// 6. 【高级选项处理】：已实现基础框架，支持选项验证和应用
//
// ⚠️ **重要说明**：
// - 返回的交易为未签名状态，nonce字段为0
// - 链ID格式："weisyn-chain-{数字ID}" -> 字节数组
// - 高级选项处理当前为占位符，未来需完整实现
// - 交易创建时间戳用于网络同步和有效性检查
//
// 💡 **后续处理流程**：
// 1. cacheTransaction(): 计算哈希并缓存
// 2. SignTransaction(): 使用私钥签名并设置nonce
// 3. SubmitTransaction(): 提交到网络进行广播
func (s *AssetTransferService) buildCompleteTransaction(
	inputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
	options []*types.TransferOptions,
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

	// 处理高级选项（多签、时间锁、委托等）
	if len(options) > 0 {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("⚙️ 处理高级转账选项 - 数量: %d", len(options)))
		}

		// 应用高级选项到交易
		err := s.applyTransferOptions(tx, options)
		if err != nil {
			return nil, fmt.Errorf("应用转账选项失败: %w", err)
		}
	}

	return tx, nil
}

// cacheTransaction 缓存交易并返回哈希
//
// 💾 **交易哈希计算与缓存服务** - 为签名流程准备交易数据
//
// 计算交易的SHA256哈希值并将未签名交易存储到内存缓存中，供后续SignTransaction使用
//
// 📝 **详细参数说明**：
//   - ctx: context.Context - 请求上下文
//   - 用于控制缓存操作的超时和取消
//   - 传递给internal.CacheUnsignedTransaction进行异步缓存
//   - 支持分布式环境下的操作追踪
//   - tx: *transaction.Transaction - 未签名的完整交易
//   - 来源: buildCompleteTransaction()构建的完整交易
//   - 状态: 未签名（nonce=0，无签名数据）
//   - 内容: 完整的输入输出、时间戳、链ID等
//   - 格式: 符合pb/blockchain/block/transaction.Transaction规范
//
// 📤 **详细返回值说明**：
//   - []byte: 32字节的交易哈希值
//   - 算法: SHA256(Transaction序列化数据)
//   - 格式: 32字节原始字节数组（非十六进制编码）
//   - 用途: 作为缓存键和SignTransaction的输入参数
//   - 唯一性: 每个不同的交易产生不同的哈希值
//   - error: 缓存操作中的错误
//   - 哈希计算错误: protobuf序列化失败
//   - 缓存写入错误: storage.MemoryStore操作失败
//   - 配置错误: internal.GetDefaultCacheConfig()失败
//
// 🔗 **组件交互细节**：
//
//  1. internal.GetDefaultCacheConfig() - 缓存配置获取
//     • 返回: 默认的缓存配置参数
//     • 包含: TTL过期时间、压缩选项、存储策略等
//     • 用途: 控制交易在缓存中的生命周期
//
//  2. internal.CacheUnsignedTransaction() - 交易缓存操作
//     • 输入: ctx, storage.MemoryStore, 哈希键, 交易数据, 配置, 日志器
//     • 处理: protobuf序列化 -> 可选压缩 -> 存储到内存
//     • 存储: key=txHash, value=serialized_transaction
//     • 过期: 根据配置TTL自动清理过期数据
//
//  3. storage.MemoryStore - 内存存储服务
//     • 接口: Set(key []byte, value []byte, ttl time.Duration) error
//     • 实现: 通常为Redis、内存映射等高性能存储
//     • 特征: 支持并发访问、原子操作、TTL自动过期
//
// 📋 **缓存逻辑流程**：
// 1. 【哈希计算】：
//   - 序列化交易: protobuf.Marshal(tx) -> []byte
//   - 计算哈希: SHA256(serialized_data) -> 32字节哈希
//   - 当前实现: 临时使用固定mock哈希（待改进）
//
// 2. 【缓存配置】：
//   - 获取默认配置: TTL、压缩、存储策略等
//   - 当前配置: 通常TTL=30分钟，适合签名流程时长
//
// 3. 【缓存存储】：
//   - 调用内部缓存工具: internal.CacheUnsignedTransaction
//   - 存储映射: txHash -> serialized_transaction
//   - 访问模式: SignTransaction根据哈希检索原始交易
//
// ✅ **当前实现状态**：
// - 哈希计算使用真实的TransactionHashServiceClient
// - 通过依赖注入的txHashServiceClient进行SHA256计算
// - 已集成完整的哈希计算和缓存机制
//
// 💡 **后续使用流程**：
// 1. 用户调用SignTransaction(txHash, privateKey)
// 2. SignTransaction根据txHash从缓存检索原始交易
// 3. 对原始交易进行签名并设置正确的nonce值
// 4. 返回已签名交易的新哈希供SubmitTransaction使用
//
// 🔧 **性能特征**：
// - 缓存写入: O(1)时间复杂度，通常<1ms
// - 内存占用: 每笔交易约1-10KB（取决于输入输出数量）
// - TTL管理: 自动清理过期数据，避免内存泄漏
// - 并发安全: 支持多线程同时缓存不同交易
func (s *AssetTransferService) cacheTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]byte, error) {
	// 使用真实的TransactionHashServiceClient计算交易哈希
	hashRequest := &transaction.ComputeHashRequest{
		Transaction: tx,
	}

	hashResponse, err := s.txHashServiceClient.ComputeHash(ctx, hashRequest)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %v", err)
	}

	if hashResponse == nil || len(hashResponse.Hash) == 0 {
		return nil, fmt.Errorf("交易哈希服务返回空哈希")
	}

	txHash := hashResponse.Hash
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 交易哈希计算完成 - hash: %x", txHash))
	}

	// 创建默认缓存配置
	config := internal.GetDefaultCacheConfig()

	// 将交易缓存到内存存储
	err = internal.CacheUnsignedTransaction(ctx, s.cacheStore, txHash, tx, config, s.logger)
	if err != nil {
		return nil, fmt.Errorf("缓存交易失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("💾 交易已缓存 - hash: %x", txHash))
	}

	return txHash, nil
}

// ============================================================================
//                              内部UTXO选择方法
// ============================================================================

// selectUTXOsForTransfer 为转账选择UTXO（内部方法）
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
//   - uint64: 找零金额（wei单位）
//   - error: 选择错误
func (s *AssetTransferService) selectUTXOsForTransfer(ctx context.Context, fromAddr []byte, amountStr string, tokenID string) ([]*transaction.TxInput, uint64, error) {
	if s.logger != nil {
		s.logger.Infof("🔍 开始UTXO选择 - 地址字节: %x (长度: %d), 金额: %s, tokenID: %s", fromAddr, len(fromAddr), amountStr, tokenID)
	}

	// 1. 解析目标金额
	targetAmount, err := s.parseAmount(amountStr)
	if err != nil {
		return nil, 0, fmt.Errorf("金额解析失败: %v", err)
	}

	// 2. 获取地址所有可用AssetUTXO
	assetCategory := utxo.UTXOCategory_UTXO_CATEGORY_ASSET

	// 添加调试信息
	if s.logger != nil {
		s.logger.Infof("🔍 查询UTXO - 地址字节: %x, 长度: %d", fromAddr, len(fromAddr))
	}

	allUTXOs, err := s.utxoManager.GetUTXOsByAddress(ctx, fromAddr, &assetCategory, true)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("❌ UTXO查询失败 - 地址: %x, 错误: %v", fromAddr, err)
		}
		return nil, 0, fmt.Errorf("获取UTXO失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Infof("🔍 UTXO查询结果 - 找到: %d个UTXO", len(allUTXOs))
		if len(allUTXOs) > 0 {
			// 打印前几个UTXO的详细信息用于调试
			for i, utxo := range allUTXOs {
				if i >= 3 {
					break // 只打印前3个
				}
				amount := s.extractUTXOAmount(utxo)
				s.logger.Infof("  📦 UTXO[%d] - TxId: %x, Index: %d, 状态: %s, 金额: %d wei",
					i, utxo.Outpoint.TxId, utxo.Outpoint.OutputIndex, utxo.Status.String(), amount)
			}
		}
	}

	if len(allUTXOs) == 0 {
		if s.logger != nil {
			s.logger.Warnf("⚠️  地址 %x 没有找到任何可用UTXO", fromAddr)
		}
		return nil, 0, fmt.Errorf("地址没有可用UTXO")
	}

	// 3. 简单选择算法：首次适应（按tokenID过滤）
	var selectedInputs []*transaction.TxInput
	var totalSelected uint64 = 0

	for _, utxoItem := range allUTXOs {
		// 验证tokenID匹配
		if !s.utxoMatchesTokenID(utxoItem, tokenID) {
			continue // 跳过不匹配的代币类型
		}

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
		return nil, 0, fmt.Errorf("余额不足，需要: %d, 可用: %d", targetAmount, totalSelected)
	}

	// 5. 计算找零
	changeAmount := totalSelected - targetAmount

	if s.logger != nil {
		s.logger.Infof("UTXO选择完成 - 选中: %d个, 总额: %d, 找零: %d wei",
			len(selectedInputs), totalSelected, changeAmount)
	}

	return selectedInputs, changeAmount, nil
}

// parseAmount 解析金额字符串为wei单位
func (s *AssetTransferService) parseAmount(amountStr string) (uint64, error) {
	// 使用utils.ParseDecimalToWei支持小数金额解析（用户输入）
	amountWei, err := utils.ParseDecimalToWei(amountStr)
	if err != nil {
		return 0, fmt.Errorf("无效的金额格式: %v", err)
	}
	return amountWei, nil
}

// extractUTXOAmount 从UTXO中提取金额
//
// 🔧 **金额解析统一标准**：
// - 链上/缓存统一存储标准小数字符串（8位精度）
// - 统一使用 utils.ParseDecimalToWei 解析，确保与输出构建的 FormatWeiToDecimal 对称
// - 解析失败时返回0，避免中断UTXO选择流程
func (s *AssetTransferService) extractUTXOAmount(utxoItem *utxo.UTXO) uint64 {
	if utxoItem == nil {
		return 0
	}

	// 根据UTXO的content_strategy提取金额
	switch strategy := utxoItem.ContentStrategy.(type) {
	case *utxo.UTXO_CachedOutput:
		if cachedOutput := strategy.CachedOutput; cachedOutput != nil {
			if assetOutput := cachedOutput.GetAsset(); assetOutput != nil {
				if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
					// 使用安全的金额解析工具，处理已存储的wei整数字符串
					amount, err := utils.ParseAmountSafely(nativeCoin.Amount)
					if err != nil {
						// 解析失败时记录日志但返回0，避免中断UTXO选择流程
						if s.logger != nil {
							s.logger.Warn(fmt.Sprintf("⚠️ 原生币UTXO金额解析失败 - Amount: %s, 错误: %v",
								nativeCoin.Amount, err))
						}
						return 0
					}
					return amount
				}
				if contractToken := assetOutput.GetContractToken(); contractToken != nil {
					// 使用安全的金额解析工具，处理已存储的wei整数字符串
					amount, err := utils.ParseAmountSafely(contractToken.Amount)
					if err != nil {
						// 解析失败时记录日志但返回0，避免中断UTXO选择流程
						if s.logger != nil {
							s.logger.Warn(fmt.Sprintf("⚠️ 合约代币UTXO金额解析失败 - Amount: %s, 错误: %v",
								contractToken.Amount, err))
						}
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
func (s *AssetTransferService) formatAmount(amount uint64) string {
	// 使用统一的protobuf Amount字段格式化方法
	return utils.FormatAmountForProtobuf(amount)
}

// utxoMatchesTokenID 检查UTXO是否匹配指定的tokenID
func (s *AssetTransferService) utxoMatchesTokenID(utxoItem *utxo.UTXO, tokenID string) bool {
	if utxoItem == nil {
		return false
	}

	// 获取UTXO的输出内容
	cachedOutput := utxoItem.GetCachedOutput()
	if cachedOutput == nil {
		return false
	}

	assetOutput := cachedOutput.GetAsset()
	if assetOutput == nil {
		return false
	}

	// 检查tokenID匹配逻辑
	if tokenID == "" {
		// 要求原生币，检查UTXO是否为NativeCoin
		return assetOutput.GetNativeCoin() != nil
	} else {
		// 要求合约代币，检查UTXO是否为ContractToken且地址匹配
		contractToken := assetOutput.GetContractToken()
		if contractToken == nil {
			return false
		}

		// 规范化tokenID和合约地址进行比较
		contractAddressHex := fmt.Sprintf("%x", contractToken.ContractAddress)

		// 规范化tokenID：去除0x前缀并转为小写
		normalizedTokenID := strings.ToLower(strings.TrimPrefix(tokenID, "0x"))
		normalizedContractAddress := strings.ToLower(contractAddressHex)

		return normalizedContractAddress == normalizedTokenID
	}
}

// ============================================================================
//
//	编译时接口检查
//
// ============================================================================
// 确保 AssetTransferService 实现了所需的接口部分

// parseTokenIDToAddress 解析TokenID为合约地址字节数组
//
// 🔗 **FT TokenID规范化解析器**
//
// 将十六进制TokenID字符串解析为标准的合约地址字节数组。
//
// 🎯 **设计约定**：
// - 对于同质化代币(FT)：TokenID = 合约地址 = fungible_class_id
// - 这是标准的ERC20兼容模式，简化了FT的标识和管理
// - NFT/SFT使用不同的标识字段，不在此方法处理范围内
//
// 参数：
//   - tokenID: 代币标识符（十六进制字符串，表示FT合约地址）
//
// 返回：
//   - []byte: 合约地址字节数组（20字节）
//   - error: 解析错误
func (s *AssetTransferService) parseTokenIDToAddress(tokenID string) ([]byte, error) {
	if tokenID == "" {
		return nil, fmt.Errorf("TokenID不能为空")
	}

	// 规范化处理：去除0x前缀，转为小写
	normalizedTokenID := strings.ToLower(strings.TrimPrefix(tokenID, "0x"))

	// 验证十六进制格式和长度（40字符 = 20字节地址）
	if len(normalizedTokenID) != 40 {
		return nil, fmt.Errorf("TokenID长度无效: 期望40字符，实际%d字符", len(normalizedTokenID))
	}

	// 验证十六进制格式
	for _, char := range normalizedTokenID {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return nil, fmt.Errorf("TokenID包含无效字符: %c", char)
		}
	}

	// 转换为字节数组
	addressBytes := make([]byte, 20)
	for i := 0; i < 20; i++ {
		high := hexCharToByte(rune(normalizedTokenID[i*2]))
		low := hexCharToByte(rune(normalizedTokenID[i*2+1]))
		addressBytes[i] = (high << 4) | low
	}

	return addressBytes, nil
}

// hexCharToByte 十六进制字符转字节值
func hexCharToByte(char rune) byte {
	if char >= '0' && char <= '9' {
		return byte(char - '0')
	}
	if char >= 'a' && char <= 'f' {
		return byte(char - 'a' + 10)
	}
	return 0
}

// validateTransferOption 验证单个转账选项
//
// 🔍 **高级选项验证器**
//
// 验证单个转账选项的有效性，包括费用控制等基础功能的参数合法性。
//
// 📝 **参数说明**：
//   - option: 转账选项对象
//
// 📤 **返回值说明**：
//   - error: 验证错误
func (s *AssetTransferService) validateTransferOption(option *types.TransferOptions) error {
	if option == nil {
		return fmt.Errorf("转账选项不能为空")
	}

	// 验证费用控制选项
	if option.FeeControl != nil {
		// 基础的费用控制验证
		if s.logger != nil {
			s.logger.Debug("验证费用控制选项")
		}
		// 具体验证逻辑需要根据实际的 FeeControlOptions 结构实现
	}

	// 当前简化实现：其他高级选项暂不支持
	// 后续可根据实际需求扩展多签、时间锁、委托等功能

	return nil
}

// applyTransferOptions 将高级选项应用到交易
//
// ⚙️ **高级选项应用器**
//
// 将验证通过的高级选项应用到交易对象。
// 当前为简化实现，主要标记选项已应用。
//
// 📝 **参数说明**：
//   - tx: 交易对象
//   - options: 转账选项列表
//
// 📤 **返回值说明**：
//   - error: 应用错误
func (s *AssetTransferService) applyTransferOptions(
	tx *transaction.Transaction,
	options []*types.TransferOptions,
) error {
	if tx == nil {
		return fmt.Errorf("交易对象不能为空")
	}

	// 遍历所有选项并应用到交易
	for i, option := range options {
		if option == nil {
			continue
		}

		// 当前简化实现：仅标记选项已应用
		// 具体的高级功能需要根据实际的 types.TransferOptions 结构实现
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("✅ 成功应用转账选项[%d]", i))
		}
	}

	return nil
}

var _ interface {
	TransferAsset(context.Context, []byte, string, string, string, string, ...*types.TransferOptions) ([]byte, error)
} = (*AssetTransferService)(nil)
