package repositories

import (
	"go.uber.org/fx"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 内部接口和配置
	repositoryconfig "github.com/weisyn/v1/internal/config/repository"
	"github.com/weisyn/v1/internal/core/repositories/interfaces"

	// 管理器实现已移至factory.go

	// 哈希服务客户端
	core "github.com/weisyn/v1/pb/blockchain/block"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ModuleInput 数据仓储模块输入依赖
type ModuleInput struct {
	fx.In

	// 基础设施组件
	ConfigProvider   config.Provider                     `optional:"false"`
	Logger           log.Logger                          `optional:"true"`
	EventBus         event.EventBus                      `optional:"true"`
	RepositoryConfig *repositoryconfig.RepositoryOptions `optional:"false"` // 资源仓库配置

	// 存储组件
	BadgerStore     storage.BadgerStore `optional:"false"`
	MemoryStore     storage.MemoryStore `optional:"true"`
	FileStore       storage.FileStore   `optional:"false"`
	StorageProvider storage.Provider    `optional:"false"`

	// 密码学组件
	HashManager       crypto.HashManager       `optional:"false"`
	MerkleTreeManager crypto.MerkleTreeManager `optional:"false"`
	SignatureManager  crypto.SignatureManager  `optional:"true"`
	KeyManager        crypto.KeyManager        `optional:"true"`
	AddressManager    crypto.AddressManager    `optional:"true"`

	// 哈希服务客户端（来自crypto模块，避免循环依赖）
	TransactionHashServiceClient transactionpb.TransactionHashServiceClient `optional:"false"`
	BlockHashServiceClient       core.BlockHashServiceClient                `optional:"false"`
}

// ModuleOutput 数据仓储模块输出
type ModuleOutput struct {
	fx.Out

	// 数据仓储服务
	RepositoryManager     repository.RepositoryManager
	UTXOManager           repository.UTXOManager
	ResourceManager       interfaces.InternalResourceManager `name:"resource_manager"`
	PublicResourceManager repository.ResourceManager         `name:"public_resource_manager"`
}

// Module 返回数据仓储核心模块的fx配置
func Module() fx.Option {
	return fx.Module("repositories",
		fx.Provide(
			// 使用工厂函数创建所有仓储服务
			func(input ModuleInput) (ModuleOutput, error) {
				serviceInput := ServiceInput{
					ConfigProvider:               input.ConfigProvider,
					Logger:                       input.Logger,
					EventBus:                     input.EventBus,
					RepositoryConfig:             input.RepositoryConfig,
					BadgerStore:                  input.BadgerStore,
					MemoryStore:                  input.MemoryStore,
					FileStore:                    input.FileStore,
					StorageProvider:              input.StorageProvider,
					HashManager:                  input.HashManager,
					MerkleTreeManager:            input.MerkleTreeManager,
					SignatureManager:             input.SignatureManager,
					KeyManager:                   input.KeyManager,
					AddressManager:               input.AddressManager,
					TransactionHashServiceClient: input.TransactionHashServiceClient,
					BlockHashServiceClient:       input.BlockHashServiceClient,
				}

				serviceOutput, err := CreateAllServices(serviceInput)
				if err != nil {
					return ModuleOutput{}, err
				}

				return ModuleOutput{
					RepositoryManager:     serviceOutput.RepositoryManager,
					UTXOManager:           serviceOutput.UTXOManager,
					ResourceManager:       serviceOutput.ResourceManager,
					PublicResourceManager: serviceOutput.PublicResourceManager,
				}, nil
			},
		),

		fx.Invoke(
			func(logger log.Logger) {
				if logger != nil {
					logger.Info("数据仓储模块已加载")
				}
			},
		),
	)
}
