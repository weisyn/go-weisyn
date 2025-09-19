// Package repositories 提供数据仓储服务工厂实现
package repositories

import (
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

	// 管理器实现
	repositorymanager "github.com/weisyn/v1/internal/core/repositories/repository"
	resourcemanager "github.com/weisyn/v1/internal/core/repositories/resource"
	utxomanager "github.com/weisyn/v1/internal/core/repositories/utxo"

	// 哈希服务客户端
	core "github.com/weisyn/v1/pb/blockchain/block"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ServiceInput 定义仓储服务工厂的输入参数
type ServiceInput struct {
	// 基础设施组件
	ConfigProvider   config.Provider
	Logger           log.Logger
	EventBus         event.EventBus
	RepositoryConfig *repositoryconfig.RepositoryOptions

	// 存储组件
	BadgerStore     storage.BadgerStore
	MemoryStore     storage.MemoryStore
	FileStore       storage.FileStore
	StorageProvider storage.Provider

	// 密码学组件
	HashManager       crypto.HashManager
	MerkleTreeManager crypto.MerkleTreeManager
	SignatureManager  crypto.SignatureManager
	KeyManager        crypto.KeyManager
	AddressManager    crypto.AddressManager

	// 哈希服务客户端
	TransactionHashServiceClient transactionpb.TransactionHashServiceClient
	BlockHashServiceClient       core.BlockHashServiceClient
}

// ServiceOutput 定义仓储服务工厂的输出结果
type ServiceOutput struct {
	RepositoryManager     repository.RepositoryManager
	UTXOManager           repository.UTXOManager
	ResourceManager       interfaces.InternalResourceManager
	PublicResourceManager repository.ResourceManager
}

// CreateUTXOManager 创建UTXO管理器
//
// 🏭 **UTXO管理器工厂**：
// 该函数负责创建UTXO管理器，处理所有必要的依赖注入和配置。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - repository.UTXOManager: UTXO管理器实例
//   - error: 创建过程中的错误
func CreateUTXOManager(input ServiceInput) (repository.UTXOManager, error) {
	return utxomanager.NewManager(
		input.Logger,
		input.BadgerStore,
		input.MemoryStore,
		input.HashManager,
		input.MerkleTreeManager,
	)
}

// CreateRepositoryManager 创建仓储管理器
//
// 🏭 **仓储管理器工厂**：
// 该函数负责创建仓储管理器，需要UTXO管理器作为依赖。
//
// 参数：
//   - input: 服务创建所需的输入参数
//   - utxoManager: UTXO管理器实例
//
// 返回：
//   - repository.RepositoryManager: 仓储管理器实例
//   - error: 创建过程中的错误
func CreateRepositoryManager(input ServiceInput, utxoManager interfaces.InternalUTXOManager) (repository.RepositoryManager, error) {
	return repositorymanager.NewManager(
		input.Logger,
		input.BadgerStore,
		input.MemoryStore,
		input.HashManager,
		input.TransactionHashServiceClient,
		input.BlockHashServiceClient,
		utxoManager,
		input.RepositoryConfig,
		input.ConfigProvider,
	)
}

// CreateResourceManager 创建资源管理器
//
// 🏭 **资源管理器工厂**：
// 该函数负责创建资源管理器，处理资源存储和管理功能。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - interfaces.InternalResourceManager: 内部资源管理器接口
//   - error: 创建过程中的错误
func CreateResourceManager(input ServiceInput) (interfaces.InternalResourceManager, error) {
	// ResourceManager 不再管理存储路径，完全委托给 FileStore
	return resourcemanager.NewManager(
		input.Logger,
		input.FileStore,
		input.BadgerStore,
		input.MemoryStore,
		input.HashManager,
		input.RepositoryConfig,
	)
}

// CreateAllServices 创建所有仓储服务
//
// 🏭 **统一服务工厂**：
// 该函数是仓储模块的主要工厂方法，负责创建所有相关服务。
// 它协调各个服务的创建顺序，处理服务间的依赖关系。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - ServiceOutput: 创建的所有服务实例
//   - error: 创建过程中的错误
func CreateAllServices(input ServiceInput) (ServiceOutput, error) {
	// 1. 创建UTXO管理器（基础服务）
	utxoManager, err := CreateUTXOManager(input)
	if err != nil {
		return ServiceOutput{}, err
	}

	// 2. 创建内部UTXO管理器接口（用于RepositoryManager）
	internalUTXOManager := utxoManager.(interfaces.InternalUTXOManager)

	// 3. 创建仓储管理器（依赖UTXO管理器）
	repositoryManager, err := CreateRepositoryManager(input, internalUTXOManager)
	if err != nil {
		return ServiceOutput{}, err
	}

	// 4. 创建资源管理器（独立服务）
	resourceManager, err := CreateResourceManager(input)
	if err != nil {
		return ServiceOutput{}, err
	}

	return ServiceOutput{
		RepositoryManager:     repositoryManager,
		UTXOManager:           utxoManager,
		ResourceManager:       resourceManager,
		PublicResourceManager: resourceManager, // 同一实例同时满足内部接口和公共接口
	}, nil
}
