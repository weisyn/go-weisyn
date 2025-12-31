// Package crypto 提供加密相关功能
package crypto

import (
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/encryption"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/merkle"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/multisig"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/pow"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/signature"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/threshold"
	blockhash "github.com/weisyn/v1/internal/core/block/hash"
	txhash "github.com/weisyn/v1/internal/core/tx/hash"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	config "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	log "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// CryptoParams 定义加密模块的依赖参数
type CryptoParams struct {
	fx.In

	Provider        config.Provider                   // 配置提供者
	Logger          log.Logger                        `optional:"true"` // 日志记录器
	ConsensusConfig *consensusconfig.ConsensusOptions `optional:"true"` // 共识配置（POW需要）
}

// CryptoOutput 定义加密模块的输出结构
type CryptoOutput struct {
	fx.Out

	// 各个子服务 - 移除命名以支持无名注入
	KeyManager                 crypto.KeyManager
	AddressManager             crypto.AddressManager
	SignatureManager           crypto.SignatureManager
	MultiSignatureVerifier     crypto.MultiSignatureVerifier
	ThresholdSignatureVerifier crypto.ThresholdSignatureVerifier
	HashManager                crypto.HashManager
	EncryptionManager      crypto.EncryptionManager
	MerkleTreeManager      crypto.MerkleTreeManager

	// POW引擎服务
	POWEngine crypto.POWEngine

	// 区块链哈希服务客户端（解决循环依赖）
	TransactionHashServiceClient transaction.TransactionHashServiceClient
	BlockHashServiceClient       core.BlockHashServiceClient
}

// Module 返回加密模块
func Module() fx.Option {
	return fx.Module("crypto",
		// 提供加密服务
		fx.Provide(ProvideCryptoServices),
	)
}

// ProvideCryptoServices 提供加密服务
//
// ✅ **符合代码组织规范**：单一装配点，所有服务创建逻辑在 module.go 中
//
// 🎯 **核心职责**：
// - 创建加密模块的所有服务
// - 处理服务间的依赖关系
// - 配置依赖注入
func ProvideCryptoServices(params CryptoParams) (CryptoOutput, error) {
	// 初始化日志（处理可选Logger）
	var logger log.Logger
	if params.Logger != nil {
		logger = params.Logger.With("module", "crypto")
		logger.Info("初始化加密模块")
	} else {
		// 创建no-op logger作为回退
		logger = &noopLogger{}
	}

	// 创建哈希服务
	hashService := hash.NewHashService()
	logger.Info("哈希服务已初始化（已启用LRU缓存，最大10000条目/缓存）")

	// 创建密钥管理服务
	keyManager := key.NewKeyManager()
	logger.Info("密钥管理服务已初始化")

	// 创建地址服务（需要KeyManager依赖）
	addressService := address.NewAddressService(keyManager)
	logger.Info("地址服务已初始化")

	// 创建签名服务
	sigService := signature.NewSignatureService(keyManager, addressService)
	logger.Info("签名服务已初始化")

	// 创建多重签名验证服务（依赖SignatureManager）
	multiSigVerifier := multisig.NewMultiSignatureVerifier(sigService)
	logger.Info("多重签名验证服务已初始化")

	// 创建门限签名验证服务
	thresholdVerifier := threshold.NewDefaultThresholdVerifier()
	logger.Info("门限签名验证服务已初始化")

	// 创建加密服务
	encryptionService := encryption.NewEncryptionService(hashService)
	logger.Info("加密服务已初始化")

	// 创建Merkle树服务
	merkleService := merkle.NewMerkleService()
	logger.Info("Merkle树服务已初始化")

	// 创建区块哈希服务（block 模块提供）
	blockHashService := blockhash.NewBlockHashService(hashService, logger)
	blockHashClient := blockhash.NewLocalBlockHashClient(blockHashService)
	logger.Info("区块哈希服务已初始化")

	// 创建交易哈希服务
	transactionHashService := txhash.NewTransactionHashService(hashService, logger)
	transactionHashClient := txhash.NewLocalTransactionHashClient(transactionHashService)
	logger.Info("交易哈希服务已初始化")

	// 创建POW引擎服务
	var powConfig *consensusconfig.POWConfig
	if params.ConsensusConfig != nil {
		powConfig = &params.ConsensusConfig.POW
	}
	powEngine, err := pow.NewEngine(hashService, logger, powConfig)
	if err != nil {
		logger.Errorf("初始化POW引擎失败: %v", err)
		return CryptoOutput{}, err
	}
	logger.Info("POW引擎服务已初始化")

	logger.Info("✅ 加密模块所有服务初始化完成")

	return CryptoOutput{
		KeyManager:                   keyManager,
		AddressManager:               addressService,
		SignatureManager:             sigService,
		MultiSignatureVerifier:       multiSigVerifier,
		ThresholdSignatureVerifier:   thresholdVerifier,
		HashManager:                  hashService,
		EncryptionManager:            encryptionService,
		MerkleTreeManager:            merkleService,
		POWEngine:                    powEngine,
		TransactionHashServiceClient: transactionHashClient,
		BlockHashServiceClient:       blockHashClient,
	}, nil
}

// noopLogger 是一个无操作的Logger实现，用于可选Logger为nil时的回退
type noopLogger struct{}

func (l *noopLogger) Debug(msg string)                          {}
func (l *noopLogger) Debugf(format string, args ...interface{}) {}
func (l *noopLogger) Info(msg string)                           {}
func (l *noopLogger) Infof(format string, args ...interface{})  {}
func (l *noopLogger) Warn(msg string)                           {}
func (l *noopLogger) Warnf(format string, args ...interface{})  {}
func (l *noopLogger) Error(msg string)                          {}
func (l *noopLogger) Errorf(format string, args ...interface{}) {}
func (l *noopLogger) Fatal(msg string)                          {}
func (l *noopLogger) Fatalf(format string, args ...interface{}) {}
func (l *noopLogger) With(keyvals ...interface{}) log.Logger    { return l }
func (l *noopLogger) Sync() error                               { return nil }
func (l *noopLogger) GetZapLogger() *zap.Logger                 { return nil }
