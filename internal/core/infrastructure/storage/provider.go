// Package storage 提供存储管理功能
package storage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// Provider 实现存储提供者接口
// 管理各类存储实例并提供访问方法
type Provider struct {
	badgerStores map[string]storage.BadgerStore
	memoryStores map[string]storage.MemoryStore
	fileStores   map[string]storage.FileStore
	sqliteStores map[string]storage.SQLiteStore
	tempStores   map[string]storage.TempStore

	// 核心存储实例（全局默认实例）
	defaultBadgerStore storage.BadgerStore
	defaultMemoryStore storage.MemoryStore
	defaultFileStore   storage.FileStore
	defaultSQLiteStore storage.SQLiteStore
	defaultTempStore   storage.TempStore

	// 日志记录器
	logger log.Logger

	// 读写锁，保护并发访问
	mu sync.RWMutex
}

// NewProvider 创建新的存储提供者实例
func NewProvider(
	badgerStore storage.BadgerStore,
	fileStore storage.FileStore,
	memoryStore storage.MemoryStore,
	sqliteStore storage.SQLiteStore,
	tempStore storage.TempStore,
	logger log.Logger,
) storage.Provider {
	provider := &Provider{
		badgerStores: make(map[string]storage.BadgerStore),
		memoryStores: make(map[string]storage.MemoryStore),
		fileStores:   make(map[string]storage.FileStore),
		sqliteStores: make(map[string]storage.SQLiteStore),
		tempStores:   make(map[string]storage.TempStore),
		logger:       logger,
	}

	// 设置默认存储实例
	if badgerStore != nil {
		provider.defaultBadgerStore = badgerStore
		provider.badgerStores["default"] = badgerStore
	}

	if memoryStore != nil {
		provider.defaultMemoryStore = memoryStore
		provider.memoryStores["default"] = memoryStore
	}

	if fileStore != nil {
		provider.defaultFileStore = fileStore
		provider.fileStores["default"] = fileStore
	}

	if sqliteStore != nil {
		provider.defaultSQLiteStore = sqliteStore
		provider.sqliteStores["default"] = sqliteStore
	}

	if tempStore != nil {
		provider.defaultTempStore = tempStore
		provider.tempStores["default"] = tempStore
	}

	return provider
}

// GetBadgerStore 获取BadgerDB键值存储
func (p *Provider) GetBadgerStore(name string) (storage.BadgerStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 如果未指定名称或请求默认存储，返回默认实例
	if name == "" || name == "default" {
		if p.defaultBadgerStore == nil {
			return nil, errors.New("默认BadgerDB存储未初始化")
		}
		return p.defaultBadgerStore, nil
	}

	// 查找指定名称的存储实例
	store, exists := p.badgerStores[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 %s 的BadgerDB存储", name)
	}

	return store, nil
}

// GetMemoryStore 获取内存存储
func (p *Provider) GetMemoryStore(name string) (storage.MemoryStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if name == "" || name == "default" {
		if p.defaultMemoryStore == nil {
			return nil, errors.New("默认内存存储未初始化")
		}
		return p.defaultMemoryStore, nil
	}

	store, exists := p.memoryStores[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 %s 的内存存储", name)
	}

	return store, nil
}

// GetFileStore 获取文件存储
func (p *Provider) GetFileStore(name string) (storage.FileStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if name == "" || name == "default" {
		if p.defaultFileStore == nil {
			return nil, errors.New("默认文件存储未初始化")
		}
		return p.defaultFileStore, nil
	}

	store, exists := p.fileStores[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 %s 的文件存储", name)
	}

	return store, nil
}

// GetSQLiteStore 获取SQLite存储
func (p *Provider) GetSQLiteStore(name string) (storage.SQLiteStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if name == "" || name == "default" {
		if p.defaultSQLiteStore == nil {
			return nil, errors.New("默认SQLite存储未初始化")
		}
		return p.defaultSQLiteStore, nil
	}

	store, exists := p.sqliteStores[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 %s 的SQLite存储", name)
	}

	return store, nil
}

// GetTempStore 获取临时存储
func (p *Provider) GetTempStore(name string) (storage.TempStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if name == "" || name == "default" {
		if p.defaultTempStore == nil {
			return nil, errors.New("默认临时存储未初始化")
		}
		return p.defaultTempStore, nil
	}

	store, exists := p.tempStores[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 %s 的临时存储", name)
	}

	return store, nil
}

// Close 关闭所有存储连接
// 注意：根据新的接口设计，存储资源由DI容器自动管理
// 但为了兼容性和手动管理，仍保留此方法
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("关闭所有存储连接...")

	// 注意：新的存储接口已移除Close()方法，由DI容器管理
	// 这里仅做清理工作，实际的资源释放由DI容器处理

	p.logger.Info("所有存储连接已关闭（由DI容器管理）")
	return nil
}
