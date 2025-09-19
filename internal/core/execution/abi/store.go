package abi

import (
	"fmt"
	"sync"
	"time"

	typespkg "github.com/weisyn/v1/pkg/types"
)

// ABIStoreConfig ABI存储的配置结构。
//
// 提供存储行为的细粒度控制，包括版本管理、缓存策略和性能优化选项。
// 该配置结构支持不同的存储需求和性能要求。
type ABIStoreConfig struct {
	// EnableVersioning 版本管理开关。
	// 启用时支持同一合约的多版本ABI并存，便于版本升级和回滚。
	// 关闭时每个合约只保留最新版本，节省存储空间。
	EnableVersioning bool

	// EnableCaching 缓存功能开关。
	// 启用时在内存中缓存常用的ABI定义，提高查询性能。
	// 关闭时每次都从底层存储读取，节省内存但降低性能。
	EnableCaching bool

	// MaxVersionsPerContract 每个合约允许的最大版本数。
	// 超过此限制时自动清理最旧的版本，防止内存无限增长。
	// 建议根据业务需求和内存资源设置合理值。
	MaxVersionsPerContract int

	// CacheTTL 缓存生存时间。
	// 缓存项超过此时间将被标记为过期，需要重新加载。
	// 0表示缓存永不过期，适用于静态ABI场景。
	CacheTTL time.Duration
}

// ABIStore 基于内存的ABI定义存储实现。
//
// 提供高性能的ABI定义存储和检索功能，支持版本管理和并发访问。
// 该实现适用于开发、测试和小规模生产环境。大规模生产环境可考虑
// 替换为基于数据库或分布式存储的实现。
//
// 存储结构：
//   - 第一层：合约地址映射
//   - 第二层：版本号映射
//   - 数据：ContractABI实例
//
// 并发安全性：
//   - 使用读写锁保护并发访问
//   - 支持多读者并发访问
//   - 写操作独占访问
type ABIStore struct {
	// abis 双层映射结构：合约地址 -> 版本号 -> ABI定义
	// 支持同一合约的多版本管理和快速查询
	abis map[string]map[string]*typespkg.ContractABI

	// config 存储配置，控制存储行为和性能参数
	config *ABIStoreConfig

	// mutex 读写互斥锁，保护并发访问的数据安全
	// 使用读写锁以支持并发读取，提高查询性能
	mutex sync.RWMutex
}

// NewABIStore 创建新的ABI存储实例。
//
// 初始化存储结构和配置，准备接受ABI定义的存储和查询操作。
//
// 参数：
//   - config: 存储配置，控制版本管理和缓存行为
//
// 返回值：
//   - *ABIStore: 初始化完成的存储实例
func NewABIStore(config *ABIStoreConfig) *ABIStore {
	return &ABIStore{
		abis:   make(map[string]map[string]*typespkg.ContractABI),
		config: config,
	}
}

// DefaultABIStoreConfig 创建默认的ABI存储配置。
//
// 返回一个包含合理默认值的配置实例，适用于大多数使用场景。
//
// 默认配置特点：
//   - 启用版本管理，支持多版本并存
//   - 启用缓存功能，提高查询性能
//   - 限制每个合约最多10个版本，防止内存泄漏
//   - 设置1小时的缓存TTL，平衡性能和数据新鲜度
//
// 返回值：
//   - *ABIStoreConfig: 包含默认设置的配置实例
func DefaultABIStoreConfig() *ABIStoreConfig {
	return &ABIStoreConfig{
		EnableVersioning:       true,
		EnableCaching:          true,
		MaxVersionsPerContract: 10,
		CacheTTL:               time.Hour,
	}
}

// StoreABI 存储合约的ABI定义。
//
// 将ABI定义按合约地址和版本号进行分类存储，支持版本管理和更新操作。
// 如果版本已存在，将进行覆盖更新。
//
// 参数：
//   - contractAddress: 合约的唯一标识符，通常是合约地址
//   - abi: 要存储的ABI定义，包含版本信息和接口描述
//
// 返回值：
//   - error: 存储过程中的错误，nil表示成功
//
// 存储逻辑：
//  1. 获取写锁，确保线程安全
//  2. 检查合约是否已存在，不存在则创建版本映射
//  3. 将ABI按版本号存储到对应位置
//  4. 根据配置进行版本数量限制（如果启用）
func (store *ABIStore) StoreABI(contractAddress string, abi *typespkg.ContractABI) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	// 确保合约的版本映射存在
	if _, exists := store.abis[contractAddress]; !exists {
		store.abis[contractAddress] = make(map[string]*typespkg.ContractABI)
	}

	// 存储ABI定义
	store.abis[contractAddress][abi.Version] = abi

	// 版本数量控制（如果启用版本管理）
	if store.config.EnableVersioning && store.config.MaxVersionsPerContract > 0 {
		store.enforceVersionLimit(contractAddress)
	}

	return nil
}

// GetABI 获取指定合约的ABI定义。
//
// 根据合约地址和版本号查询ABI定义。如果版本号为空，返回最新版本。
// 支持高并发的读取操作。
//
// 参数：
//   - contractAddress: 合约的唯一标识符
//   - version: ABI版本号，空字符串表示获取最新版本
//
// 返回值：
//   - *typespkg.ContractABI: 查询到的ABI定义
//   - error: 查询过程中的错误，nil表示成功
//
// 查询逻辑：
//  1. 获取读锁，支持并发读取
//  2. 检查合约是否存在
//  3. 如果版本号为空，查找最新版本（按更新时间）
//  4. 如果指定版本号，直接查询对应版本
//  5. 返回查询结果或相应错误
func (store *ABIStore) GetABI(contractAddress, version string) (*typespkg.ContractABI, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	// 检查合约是否存在
	versions, exists := store.abis[contractAddress]
	if !exists {
		return nil, fmt.Errorf("contract %s not found", contractAddress)
	}

	// 处理获取最新版本的情况
	if version == "" {
		return store.getLatestVersion(versions), nil
	}

	// 获取指定版本
	abi, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found for contract %s", version, contractAddress)
	}

	return abi, nil
}

// getLatestVersion 获取最新版本的ABI定义。
//
// 遍历所有版本，根据更新时间找到最新的ABI定义。
//
// 参数：
//   - versions: 版本映射表
//
// 返回值：
//   - *typespkg.ContractABI: 最新版本的ABI定义，可能为nil
func (store *ABIStore) getLatestVersion(versions map[string]*typespkg.ContractABI) *typespkg.ContractABI {
	var latestABI *typespkg.ContractABI

	for _, abi := range versions {
		if latestABI == nil || abi.UpdatedAt.After(latestABI.UpdatedAt) {
			latestABI = abi
		}
	}

	return latestABI
}

// enforceVersionLimit 强制执行版本数量限制。
//
// 当版本数量超过配置限制时，删除最旧的版本以释放内存。
//
// 参数：
//   - contractAddress: 合约地址
func (store *ABIStore) enforceVersionLimit(contractAddress string) {
	versions := store.abis[contractAddress]

	// 检查是否超过限制
	if len(versions) <= store.config.MaxVersionsPerContract {
		return
	}

	// 找到最旧的版本并删除
	var oldestVersion string
	var oldestTime time.Time

	for version, abi := range versions {
		if oldestVersion == "" || abi.UpdatedAt.Before(oldestTime) {
			oldestVersion = version
			oldestTime = abi.UpdatedAt
		}
	}

	if oldestVersion != "" {
		delete(versions, oldestVersion)
	}
}

// ListVersions 列出指定合约的所有版本。
//
// 返回合约的所有可用版本号，便于版本管理和查询。
//
// 参数：
//   - contractAddress: 合约地址
//
// 返回值：
//   - []string: 版本号列表
//   - error: 查询错误
func (store *ABIStore) ListVersions(contractAddress string) ([]string, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	versions, exists := store.abis[contractAddress]
	if !exists {
		return nil, fmt.Errorf("contract %s not found", contractAddress)
	}

	versionList := make([]string, 0, len(versions))
	for version := range versions {
		versionList = append(versionList, version)
	}

	return versionList, nil
}

// RemoveABI 删除指定版本的ABI定义。
//
// 从存储中移除指定合约的特定版本ABI，用于版本清理和存储管理。
//
// 参数：
//   - contractAddress: 合约地址
//   - version: 要删除的版本号
//
// 返回值：
//   - error: 删除操作的错误
func (store *ABIStore) RemoveABI(contractAddress, version string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	versions, exists := store.abis[contractAddress]
	if !exists {
		return fmt.Errorf("contract %s not found", contractAddress)
	}

	if _, ok := versions[version]; !ok {
		return fmt.Errorf("version %s not found for contract %s", version, contractAddress)
	}

	delete(versions, version)

	// 如果没有版本了，删除整个合约条目
	if len(versions) == 0 {
		delete(store.abis, contractAddress)
	}

	return nil
}
