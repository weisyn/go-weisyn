// Package genesis 创世交易处理实现
//
// 🎯 **创世交易专业处理**
//
// 本文件专门处理创世交易的创建逻辑，包括：
// - 创世交易生成：基于GenesisConfig创建初始代币分配交易
// - 确定性排序：保证相同配置产生相同的交易顺序
// - 系统合约部署：可选的系统合约初始化交易
// - 原子性操作：要么全部生成成功要么全部失败
//
// 🏗️ **设计原则**
// - 专业分工：专门处理创世交易生成业务逻辑
// - 配置驱动：完全基于GenesisConfig生成交易
// - 确定性：相同输入产生相同的交易列表
// - 无依赖输入：创世交易无UTXO输入，只有输出
package genesis

import (
	"context"
	"fmt"
	"strconv"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 协议定义
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 创世交易生成实现 ====================

// CreateTransactions 创建创世区块交易
//
// 🎯 **创世交易生成服务**
//
// 基于创世配置生成所有创世交易，包括：
// 1. 初始账户分配交易：为预设账户分配初始代币
// 2. 系统合约部署交易：部署核心系统合约（可选）
// 3. 网络参数设置交易：设置网络初始参数（可选）
//
// 设计特点：
// - 配置驱动：完全基于GenesisConfig生成
// - 确定性：相同配置产生相同的交易列表
// - 验证性：生成的交易必须能通过标准验证
// - 原子性：要么全部生成成功要么全部失败
//
// 参数：
//   - ctx: 操作上下文
//   - genesisConfig: 创世配置信息
//   - keyManager: 密钥管理服务
//   - addressManager: 地址管理服务
//   - logger: 日志服务
//
// 返回：
//   - []*transaction.Transaction: 创世交易列表
//   - error: 生成过程中的错误
func CreateTransactions(
	ctx context.Context,
	genesisConfig interface{},
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	logger log.Logger,
) ([]*transaction.Transaction, error) {
	if logger != nil {
		logger.Infof("开始创建创世交易")
	}

	// 类型转换：将interface{}转换为具体的GenesisConfig类型
	config, ok := genesisConfig.(*types.GenesisConfig)
	if !ok {
		return nil, fmt.Errorf("无效的创世配置类型: %T", genesisConfig)
	}

	if config == nil {
		return nil, fmt.Errorf("创世配置不能为空")
	}

	// 验证配置
	if err := types.ValidateGenesisConfig(config); err != nil {
		return nil, fmt.Errorf("创世配置验证失败: %w", err)
	}

	var transactions []*transaction.Transaction

	// 创建代币分配交易（确定性排序）
	if len(config.GenesisAccounts) > 0 {
		if logger != nil {
			logger.Infof("创建 %d 个代币分配交易", len(config.GenesisAccounts))
		}

		// 按公钥排序确保确定性
		accounts := make([]types.GenesisAccount, len(config.GenesisAccounts))
		copy(accounts, config.GenesisAccounts)

		// 简单排序（按公钥字典序）
		for i := 0; i < len(accounts)-1; i++ {
			for j := i + 1; j < len(accounts); j++ {
				if accounts[i].PublicKey > accounts[j].PublicKey {
					accounts[i], accounts[j] = accounts[j], accounts[i]
				}
			}
		}

		// 按确定性顺序创建交易
		for i, account := range accounts {
			if logger != nil {
				logger.Infof("🔧 创建分配交易 [%d]: 公钥=%s, 初始余额=%s", i, account.PublicKey, account.InitialBalance)
			}

			// 解析公钥
			publicKeyBytes, err := keyManager.ParsePublicKeyString(account.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("解析公钥失败 %s: %w", account.PublicKey, err)
			}

			// 从公钥生成地址
			address, err := addressManager.PublicKeyToAddress(publicKeyBytes)
			if err != nil {
				return nil, fmt.Errorf("从公钥生成地址失败: %w", err)
			}

			// 转换地址为字节
			addressBytes, err := addressManager.AddressToBytes(address)
			if err != nil {
				return nil, fmt.Errorf("地址转换失败: %w", err)
			}

			// 解析初始余额
			if logger != nil {
				logger.Infof("🔧 解析金额字符串: %s", account.InitialBalance)
			}
			amount, err := strconv.ParseUint(account.InitialBalance, 10, 64)
			if err != nil {
				if logger != nil {
					logger.Errorf("🔧 金额解析失败: %s -> %v", account.InitialBalance, err)
				}
				return nil, fmt.Errorf("解析初始余额失败 %s: %w", account.InitialBalance, err)
			}
			if logger != nil {
				logger.Infof("🔧 解析后的金额: %d", amount)
			}

			// 创建分配交易
			allocationTx := &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{}, // 创世交易无输入
				Outputs: []*transaction.TxOutput{
					{
						Owner: addressBytes,
						LockingConditions: []*transaction.LockingCondition{
							{
								Condition: &transaction.LockingCondition_SingleKeyLock{
									SingleKeyLock: &transaction.SingleKeyLock{
										KeyRequirement: &transaction.SingleKeyLock_RequiredPublicKey{
											RequiredPublicKey: &transaction.PublicKey{
												Value: publicKeyBytes,
											},
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
										Amount: fmt.Sprintf("%d", amount),
									},
								},
							},
						},
					},
				},
				FeeMechanism: &transaction.Transaction_MinimumFee{
					MinimumFee: &transaction.MinimumFee{
						MinimumAmount: "0", // 创世交易免费
						FeeToken: &transaction.TokenReference{
							TokenType: &transaction.TokenReference_NativeToken{
								NativeToken: true, // 使用原生代币
							},
						},
					},
				},
				Nonce:             uint64(i), // 使用序号确保唯一性
				CreationTimestamp: uint64(config.Timestamp),
			}
			transactions = append(transactions, allocationTx)
		}
	}

	// 如果没有账户配置，创建空的启动标记交易
	if len(transactions) == 0 {
		if logger != nil {
			logger.Info("创建启动标记交易")
		}
		emptyTx := &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
			FeeMechanism: &transaction.Transaction_MinimumFee{
				MinimumFee: &transaction.MinimumFee{
					MinimumAmount: "0", // 创世交易免费
					FeeToken: &transaction.TokenReference{
						TokenType: &transaction.TokenReference_NativeToken{
							NativeToken: true, // 使用原生代币
						},
					},
				},
			},
			Nonce:             0,
			CreationTimestamp: uint64(config.Timestamp),
		}
		transactions = append(transactions, emptyTx)
	}

	if logger != nil {
		logger.Infof("✅ 创世交易创建完成，共 %d 个交易", len(transactions))
	}

	return transactions, nil
}
