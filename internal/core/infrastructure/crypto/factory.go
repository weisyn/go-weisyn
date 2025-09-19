// Package crypto 提供加密服务工厂实现
package crypto

import (
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/encryption"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/merkle"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/pow"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/signature"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	config "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	log "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ServiceInput 定义加密服务工厂的输入参数
type ServiceInput struct {
	ConfigProvider   config.Provider                   `optional:"false"`
	Logger           log.Logger                        `optional:"true"`
	ConsensusOptions *consensusconfig.ConsensusOptions `optional:"true"`
}

// ServiceOutput 定义加密服务工厂的输出结果
type ServiceOutput struct {
	KeyManager                   crypto.KeyManager
	AddressManager               crypto.AddressManager
	SignatureManager             crypto.SignatureManager
	HashManager                  crypto.HashManager
	EncryptionManager            crypto.EncryptionManager
	MerkleTreeManager            crypto.MerkleTreeManager
	POWEngine                    crypto.POWEngine
	TransactionHashServiceClient transaction.TransactionHashServiceClient
	BlockHashServiceClient       core.BlockHashServiceClient
}

// CreateCryptoServices 创建加密服务
//
// 🏭 **加密服务工厂**：
// 该函数负责创建加密模块的所有服务，处理服务间的依赖关系。
// 将复杂的服务创建逻辑从module.go中分离出来，保持module.go的薄实现。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - ServiceOutput: 创建的服务实例集合
//   - error: 创建过程中的错误
func CreateCryptoServices(input ServiceInput) (ServiceOutput, error) {
	// 初始化日志（处理可选Logger）
	var logger log.Logger
	if input.Logger != nil {
		logger = input.Logger.With("module", "crypto")
		logger.Info("初始化加密模块")
	} else {
		// 创建no-op logger作为回退
		logger = &noopLogger{}
	}

	// 创建哈希服务
	hashService := hash.NewHashService()
	logger.Info("哈希服务已初始化")

	// 创建密钥管理服务
	keyManager := key.NewKeyManager()
	logger.Info("密钥管理服务已初始化")

	// 创建地址服务（需要KeyManager依赖）
	addressService := address.NewAddressService(keyManager)
	logger.Info("地址服务已初始化")

	// 创建签名服务
	sigService := signature.NewSignatureService(keyManager, addressService)
	logger.Info("签名服务已初始化")

	// 创建加密服务
	encryptionService := encryption.NewEncryptionService(hashService)
	logger.Info("加密服务已初始化")

	// 创建Merkle树服务
	merkleService := merkle.NewMerkleService()
	logger.Info("Merkle树服务已初始化")

	// 创建交易哈希服务
	transactionHashService := hash.NewTransactionHashService(hashService, logger)
	transactionHashClient := hash.NewLocalTransactionHashClient(transactionHashService)
	logger.Info("交易哈希服务已初始化")

	// 创建区块哈希服务
	blockHashService := hash.NewBlockHashService(hashService, logger)
	blockHashClient := hash.NewLocalBlockHashClient(blockHashService)
	logger.Info("区块哈希服务已初始化")

	// 创建POW引擎服务
	var powConfig *consensusconfig.POWConfig
	if input.ConsensusOptions != nil {
		powConfig = &input.ConsensusOptions.POW
	}
	powEngine, err := pow.NewEngine(hashService, logger, powConfig)
	if err != nil {
		logger.Errorf("初始化POW引擎失败: %v", err)
		return ServiceOutput{}, err
	}
	logger.Info("POW引擎服务已初始化")

	logger.Info("✅ 加密模块所有服务初始化完成")

	return ServiceOutput{
		KeyManager:                   keyManager,
		AddressManager:               addressService,
		SignatureManager:             sigService,
		HashManager:                  hashService,
		EncryptionManager:            encryptionService,
		MerkleTreeManager:            merkleService,
		POWEngine:                    powEngine,
		TransactionHashServiceClient: transactionHashClient,
		BlockHashServiceClient:       blockHashClient,
	}, nil
}

// noopLogger在module.go中已定义，这里直接使用
