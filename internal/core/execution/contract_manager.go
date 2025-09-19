package execution

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// 合约管理器：负责合约元数据加载、校验与缓存；对外提供ABI查询能力。
// 设计目标：
// 1) 高内聚：仅承担“合约元数据/ABI”的查询与校验职责，不参与执行流程；
// 2) 低耦合：依赖通过构造函数注入；外部以接口方式使用；
// 3) 可观测：暴露基本统计信息；
// 4) 可测试：内部无全局状态，便于Mock替换。
type ContractManager struct {
	// 并发读写保护
	mu sync.RWMutex

	// 元数据LRU缓存
	metadataList  *list.List               // 头部为最近使用
	metadataIndex map[string]*list.Element // key → element

	// ABI LRU缓存
	abiList  *list.List
	abiIndex map[string]*list.Element

	// 缓存配置
	cacheCapacity int
	cacheTTL      time.Duration

	// 统计信息
	stats *ContractManagerStats

	// 外部依赖
	loader    ContractLoader
	validator ContractValidator
}

// ContractLoader 定义合约元数据与ABI的加载方式（可来自存储、网络或编译产物）
type ContractLoader interface {
	LoadMetadata(contractAddr string) (*types.ContractMetadata, error)
	LoadABI(contractAddr string) (*types.ContractABI, error)
}

// ContractValidator 定义合约元数据与ABI的校验逻辑
type ContractValidator interface {
	ValidateMetadata(md *types.ContractMetadata) error
	ValidateABI(abi *types.ContractABI) error
}

// ContractManagerStats 统计信息
type ContractManagerStats struct {
	LoadsTotal    uint64 // 总加载次数
	CacheHitsMeta uint64 // 元数据缓存命中
	CacheMissMeta uint64 // 元数据缓存未命中
	CacheHitsABI  uint64 // ABI缓存命中
	CacheMissABI  uint64 // ABI缓存未命中
	LastUpdatedAt int64  // 最后一次更新缓存的时间戳
}

// NewContractManager 创建合约管理器
func NewContractManager(loader ContractLoader, validator ContractValidator, cacheCapacity int, cacheTTL time.Duration) *ContractManager {
	if cacheCapacity <= 0 {
		cacheCapacity = 1024
	}
	if cacheTTL <= 0 {
		cacheTTL = time.Hour
	}

	return &ContractManager{
		metadataList:  list.New(),
		metadataIndex: make(map[string]*list.Element, cacheCapacity),
		abiList:       list.New(),
		abiIndex:      make(map[string]*list.Element, cacheCapacity),
		cacheCapacity: cacheCapacity,
		cacheTTL:      cacheTTL,
		stats:         &ContractManagerStats{},
		loader:        loader,
		validator:     validator,
	}
}

// LoadContract 加载合约元数据（带缓存与校验）
func (cm *ContractManager) LoadContract(contractAddr string) (*types.ContractMetadata, error) {
	if md, ok := cm.getMetadataFromCache(contractAddr); ok {
		cm.stats.CacheHitsMeta++
		return md, nil
	}
	cm.stats.CacheMissMeta++

	// 外部加载
	loaded, err := cm.loader.LoadMetadata(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("load contract metadata failed: %w", err)
	}

	// 校验
	if err := cm.validator.ValidateMetadata(loaded); err != nil {
		return nil, fmt.Errorf("validate contract metadata failed: %w", err)
	}

	// 刷新时间戳
	loaded.UpdatedAt = time.Now().Unix()

	// 写入LRU缓存
	cm.setMetadataCache(contractAddr, loaded)

	cm.stats.LoadsTotal++
	return loaded, nil
}

// ValidateContract 对合约元数据进行校验（对外方法，便于独立使用）
func (cm *ContractManager) ValidateContract(metadata *types.ContractMetadata) error {
	return cm.validator.ValidateMetadata(metadata)
}

// GetContractABI 获取合约ABI（带缓存与校验）
func (cm *ContractManager) GetContractABI(contractAddr string) (*types.ContractABI, error) {
	if abi, ok := cm.getABICache(contractAddr); ok {
		cm.stats.CacheHitsABI++
		return abi, nil
	}
	cm.stats.CacheMissABI++

	// 外部加载
	loaded, err := cm.loader.LoadABI(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("load contract ABI failed: %w", err)
	}

	// 校验
	if err := cm.validator.ValidateABI(loaded); err != nil {
		return nil, fmt.Errorf("validate contract ABI failed: %w", err)
	}

	// 写入LRU缓存
	cm.setABICache(contractAddr, loaded)

	cm.stats.LoadsTotal++
	return loaded, nil
}

// GetStats 返回统计信息（快照）
func (cm *ContractManager) GetStats() ContractManagerStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return *cm.stats
}

// isExpired 判断元数据是否过期
func (cm *ContractManager) isExpired(updatedAt int64) bool {
	if cm.cacheTTL <= 0 {
		return false
	}
	return time.Now().Unix()-updatedAt > int64(cm.cacheTTL/time.Second)
}

// isABIExpired 判断ABI是否过期（按更新时间字段）
func (cm *ContractManager) isABIExpired(abi *types.ContractABI) bool {
	if abi == nil {
		return true
	}
	if cm.cacheTTL <= 0 {
		return false
	}
	return time.Since(abi.UpdatedAt) > cm.cacheTTL
}

// ComputeCodeHash 计算代码哈希（便于校验与比对）
func ComputeCodeHash(code []byte) string {
	sum := sha256.Sum256(code)
	return hex.EncodeToString(sum[:])
}

// ==================== LRU 缓存实现 ====================

type mdEntry struct {
	key       string
	value     *types.ContractMetadata
	updatedAt int64
}

func (cm *ContractManager) getMetadataFromCache(key string) (*types.ContractMetadata, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if el, ok := cm.metadataIndex[key]; ok {
		entry := el.Value.(mdEntry)
		// 过期检查
		if cm.isExpired(entry.updatedAt) {
			cm.metadataList.Remove(el)
			delete(cm.metadataIndex, key)
			return nil, false
		}
		cm.metadataList.MoveToFront(el)
		return entry.value, true
	}
	return nil, false
}

func (cm *ContractManager) setMetadataCache(key string, md *types.ContractMetadata) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if el, ok := cm.metadataIndex[key]; ok {
		el.Value = mdEntry{key: key, value: md, updatedAt: md.UpdatedAt}
		cm.metadataList.MoveToFront(el)
	} else {
		el := cm.metadataList.PushFront(mdEntry{key: key, value: md, updatedAt: md.UpdatedAt})
		cm.metadataIndex[key] = el
	}
	cm.stats.LastUpdatedAt = md.UpdatedAt

	for cm.metadataList.Len() > cm.cacheCapacity {
		back := cm.metadataList.Back()
		if back == nil {
			break
		}
		entry := back.Value.(mdEntry)
		cm.metadataList.Remove(back)
		delete(cm.metadataIndex, entry.key)
	}
}

type abiEntry struct {
	key   string
	value *types.ContractABI
}

func (cm *ContractManager) getABICache(key string) (*types.ContractABI, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if el, ok := cm.abiIndex[key]; ok {
		entry := el.Value.(abiEntry)
		if cm.isABIExpired(entry.value) {
			cm.abiList.Remove(el)
			delete(cm.abiIndex, key)
			return nil, false
		}
		cm.abiList.MoveToFront(el)
		return entry.value, true
	}
	return nil, false
}

func (cm *ContractManager) setABICache(key string, abi *types.ContractABI) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now().Unix()
	if el, ok := cm.abiIndex[key]; ok {
		el.Value = abiEntry{key: key, value: abi}
		cm.abiList.MoveToFront(el)
	} else {
		el := cm.abiList.PushFront(abiEntry{key: key, value: abi})
		cm.abiIndex[key] = el
	}
	cm.stats.LastUpdatedAt = now

	for cm.abiList.Len() > cm.cacheCapacity {
		back := cm.abiList.Back()
		if back == nil {
			break
		}
		entry := back.Value.(abiEntry)
		cm.abiList.Remove(back)
		delete(cm.abiIndex, entry.key)
	}
}
