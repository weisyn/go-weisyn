// Package crypto 提供加密相关功能
package crypto

import (
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
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
	KeyManager        crypto.KeyManager
	AddressManager    crypto.AddressManager
	SignatureManager  crypto.SignatureManager
	HashManager       crypto.HashManager
	EncryptionManager crypto.EncryptionManager
	MerkleTreeManager crypto.MerkleTreeManager

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
func ProvideCryptoServices(params CryptoParams) (CryptoOutput, error) {
	serviceInput := ServiceInput{
		ConfigProvider:   params.Provider,
		Logger:           params.Logger,
		ConsensusOptions: params.ConsensusConfig,
	}

	serviceOutput, err := CreateCryptoServices(serviceInput)
	if err != nil {
		return CryptoOutput{}, err
	}

	return CryptoOutput{
		KeyManager:                   serviceOutput.KeyManager,
		AddressManager:               serviceOutput.AddressManager,
		SignatureManager:             serviceOutput.SignatureManager,
		HashManager:                  serviceOutput.HashManager,
		EncryptionManager:            serviceOutput.EncryptionManager,
		MerkleTreeManager:            serviceOutput.MerkleTreeManager,
		POWEngine:                    serviceOutput.POWEngine,
		TransactionHashServiceClient: serviceOutput.TransactionHashServiceClient,
		BlockHashServiceClient:       serviceOutput.BlockHashServiceClient,
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
