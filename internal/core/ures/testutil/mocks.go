// Package testutil 提供 URES 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"sync"

	"go.uber.org/zap"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// 注意：MockFileStore 实现了 storage.FileStore 接口
// 虽然这里没有显式导入 storage 包，但接口定义在 storage 包中
// 测试文件会导入 storage 包来使用接口类型

// ==================== Mock 对象 ====================

// MockLogger 统一的日志Mock实现
//
// ✅ **设计原则**：最小实现，所有方法返回空值，不记录日志
// 📋 **使用场景**：80%的测试用例，不需要验证日志调用
type MockLogger struct{}

func (m *MockLogger) Debug(msg string)                          {}
func (m *MockLogger) Debugf(format string, args ...interface{}) {}
func (m *MockLogger) Info(msg string)                           {}
func (m *MockLogger) Infof(format string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string)                           {}
func (m *MockLogger) Warnf(format string, args ...interface{})  {}
func (m *MockLogger) Error(msg string)                          {}
func (m *MockLogger) Errorf(format string, args ...interface{}) {}
func (m *MockLogger) Fatal(msg string)                          {}
func (m *MockLogger) Fatalf(format string, args ...interface{}) {}
func (m *MockLogger) With(args ...interface{}) log.Logger       { return m }
func (m *MockLogger) Sync() error                               { return nil }
func (m *MockLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// MockHashManager 统一的哈希计算Mock实现
//
// ✅ **设计原则**：最小实现，提供基本的哈希计算功能
// 📋 **使用场景**：CAS存储测试，需要计算文件哈希
type MockHashManager struct {
	mu sync.Mutex
}

// SHA256 计算SHA-256哈希
func (m *MockHashManager) SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// Keccak256 计算Keccak-256哈希
func (m *MockHashManager) Keccak256(data []byte) []byte {
	// Mock实现：使用SHA256代替（测试用）
	return m.SHA256(data)
}

// RIPEMD160 计算RIPEMD-160哈希
func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	// Mock实现：返回20字节（测试用）
	result := make([]byte, 20)
	copy(result, m.SHA256(data)[:20])
	return result
}

// DoubleSHA256 计算双重SHA-256哈希
func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := m.SHA256(data)
	return m.SHA256(first)
}

// NewSHA256Hasher 创建SHA-256流式哈希器
func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// NewRIPEMD160Hasher 创建RIPEMD-160流式哈希器
func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	// Mock实现：返回SHA256代替（测试用）
	return sha256.New()
}

// MockFileStore 统一的文件存储Mock实现
//
// ✅ **设计原则**：内存存储，支持基本的文件操作
// 📋 **使用场景**：CAS存储测试，需要模拟文件存储
type MockFileStore struct {
	mu    sync.RWMutex
	files map[string][]byte // path -> data
}

// NewMockFileStore 创建新的Mock文件存储
func NewMockFileStore() *MockFileStore {
	return &MockFileStore{
		files: make(map[string][]byte),
	}
}

// Save 保存数据到指定路径
func (m *MockFileStore) Save(ctx context.Context, path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = data
	return nil
}

// Load 从指定路径加载数据
func (m *MockFileStore) Load(ctx context.Context, path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, exists := m.files[path]
	if !exists {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}
	return data, nil
}

// Delete 删除指定路径的文件
func (m *MockFileStore) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.files[path]; !exists {
		return fmt.Errorf("文件不存在: %s", path)
	}
	delete(m.files, path)
	return nil
}

// Exists 检查指定路径的文件是否存在
func (m *MockFileStore) Exists(ctx context.Context, path string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.files[path]
	return exists, nil
}

// FileInfo 获取文件信息
func (m *MockFileStore) FileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, exists := m.files[path]
	if !exists {
		return types.FileInfo{}, fmt.Errorf("文件不存在: %s", path)
	}
	return types.FileInfo{
		Size: int64(len(data)),
	}, nil
}

// ListFiles 列出指定目录下的所有文件
func (m *MockFileStore) ListFiles(ctx context.Context, dirPath string, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for path := range m.files {
		// 简单实现：检查路径是否以dirPath开头
		if len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			result = append(result, path)
		}
	}
	return result, nil
}

// MakeDir 创建目录
func (m *MockFileStore) MakeDir(ctx context.Context, dirPath string, recursive bool) error {
	// Mock实现：目录创建总是成功
	return nil
}

// DeleteDir 删除目录
func (m *MockFileStore) DeleteDir(ctx context.Context, dirPath string, recursive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 删除所有以dirPath开头的文件
	for path := range m.files {
		if len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			delete(m.files, path)
		}
	}
	return nil
}

// OpenReadStream 打开文件的读取流
func (m *MockFileStore) OpenReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	data, err := m.Load(ctx, path)
	if err != nil {
		return nil, err
	}
	return &mockReadCloser{data: data}, nil
}

// OpenWriteStream 打开文件的写入流
func (m *MockFileStore) OpenWriteStream(ctx context.Context, path string) (io.WriteCloser, error) {
	return &mockWriteCloser{
		store: m,
		path:  path,
	}, nil
}

// Copy 复制文件
func (m *MockFileStore) Copy(ctx context.Context, sourcePath, destPath string) error {
	data, err := m.Load(ctx, sourcePath)
	if err != nil {
		return err
	}
	return m.Save(ctx, destPath, data)
}

// Move 移动文件
func (m *MockFileStore) Move(ctx context.Context, sourcePath, destPath string) error {
	data, err := m.Load(ctx, sourcePath)
	if err != nil {
		return err
	}
	if err := m.Save(ctx, destPath, data); err != nil {
		return err
	}
	return m.Delete(ctx, sourcePath)
}

// GetFiles 获取所有文件（用于测试）
func (m *MockFileStore) GetFiles() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for k, v := range m.files {
		result[k] = v
	}
	return result
}

// Clear 清空所有文件（用于测试）
func (m *MockFileStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files = make(map[string][]byte)
}

// mockReadCloser 模拟读取流
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

// mockWriteCloser 模拟写入流
type mockWriteCloser struct {
	store *MockFileStore
	path  string
	buf   []byte
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	m.buf = append(m.buf, p...)
	return len(p), nil
}

func (m *mockWriteCloser) Close() error {
	return m.store.Save(context.Background(), m.path, m.buf)
}

// BehavioralMockFileStore 行为Mock文件存储（记录调用）
//
// ✅ **设计原则**：记录所有文件操作调用，用于验证交互
// 📋 **使用场景**：需要验证文件操作调用的测试（5%的测试用例）
type BehavioralMockFileStore struct {
	mu         sync.RWMutex
	files      map[string][]byte
	saveCalls  []string
	loadCalls  []string
	existsCalls []string
}

// NewBehavioralMockFileStore 创建行为Mock文件存储
func NewBehavioralMockFileStore() *BehavioralMockFileStore {
	return &BehavioralMockFileStore{
		files:       make(map[string][]byte),
		saveCalls:   make([]string, 0),
		loadCalls:   make([]string, 0),
		existsCalls: make([]string, 0),
	}
}

// Save 保存数据并记录调用
func (m *BehavioralMockFileStore) Save(ctx context.Context, path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = data
	m.saveCalls = append(m.saveCalls, path)
	return nil
}

// Load 加载数据并记录调用
func (m *BehavioralMockFileStore) Load(ctx context.Context, path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadCalls = append(m.loadCalls, path)
	data, exists := m.files[path]
	if !exists {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}
	return data, nil
}

// Delete 删除文件
func (m *BehavioralMockFileStore) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.files[path]; !exists {
		return fmt.Errorf("文件不存在: %s", path)
	}
	delete(m.files, path)
	return nil
}

// Exists 检查文件存在并记录调用
func (m *BehavioralMockFileStore) Exists(ctx context.Context, path string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.existsCalls = append(m.existsCalls, path)
	_, exists := m.files[path]
	return exists, nil
}

// FileInfo 获取文件信息
func (m *BehavioralMockFileStore) FileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, exists := m.files[path]
	if !exists {
		return types.FileInfo{}, fmt.Errorf("文件不存在: %s", path)
	}
	return types.FileInfo{
		Size: int64(len(data)),
	}, nil
}

// ListFiles 列出文件
func (m *BehavioralMockFileStore) ListFiles(ctx context.Context, dirPath string, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for path := range m.files {
		if len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			result = append(result, path)
		}
	}
	return result, nil
}

// MakeDir 创建目录
func (m *BehavioralMockFileStore) MakeDir(ctx context.Context, dirPath string, recursive bool) error {
	return nil
}

// DeleteDir 删除目录
func (m *BehavioralMockFileStore) DeleteDir(ctx context.Context, dirPath string, recursive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for path := range m.files {
		if len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			delete(m.files, path)
		}
	}
	return nil
}

// OpenReadStream 打开读取流
func (m *BehavioralMockFileStore) OpenReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	data, err := m.Load(ctx, path)
	if err != nil {
		return nil, err
	}
	return &mockReadCloser{data: data}, nil
}

// OpenWriteStream 打开写入流
func (m *BehavioralMockFileStore) OpenWriteStream(ctx context.Context, path string) (io.WriteCloser, error) {
	return &mockWriteCloser{
		store: &MockFileStore{files: m.files},
		path:  path,
	}, nil
}

// Copy 复制文件
func (m *BehavioralMockFileStore) Copy(ctx context.Context, sourcePath, destPath string) error {
	data, err := m.Load(ctx, sourcePath)
	if err != nil {
		return err
	}
	return m.Save(ctx, destPath, data)
}

// Move 移动文件
func (m *BehavioralMockFileStore) Move(ctx context.Context, sourcePath, destPath string) error {
	data, err := m.Load(ctx, sourcePath)
	if err != nil {
		return err
	}
	if err := m.Save(ctx, destPath, data); err != nil {
		return err
	}
	return m.Delete(ctx, sourcePath)
}

// GetSaveCalls 获取Save调用记录
func (m *BehavioralMockFileStore) GetSaveCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.saveCalls...)
}

// GetLoadCalls 获取Load调用记录
func (m *BehavioralMockFileStore) GetLoadCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.loadCalls...)
}

// GetExistsCalls 获取Exists调用记录
func (m *BehavioralMockFileStore) GetExistsCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.existsCalls...)
}

// ClearCalls 清空调用记录
func (m *BehavioralMockFileStore) ClearCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCalls = m.saveCalls[:0]
	m.loadCalls = m.loadCalls[:0]
	m.existsCalls = m.existsCalls[:0]
}

// MockCASStorage 统一的CAS存储Mock实现
//
// ✅ **设计原则**：内存存储，支持内容寻址存储操作
// 📋 **使用场景**：ResourceWriter测试，需要模拟CAS存储
type MockCASStorage struct {
	mu    sync.RWMutex
	files map[string][]byte // contentHash (hex) -> data
}

// NewMockCASStorage 创建新的Mock CAS存储
func NewMockCASStorage() *MockCASStorage {
	return &MockCASStorage{
		files: make(map[string][]byte),
	}
}

// BuildFilePath 构建文件路径
func (m *MockCASStorage) BuildFilePath(contentHash []byte) string {
	if len(contentHash) != 32 {
		return ""
	}
	hashHex := hex.EncodeToString(contentHash)
	dir1 := hashHex[0:2]
	dir2 := hashHex[2:4]
	return fmt.Sprintf("%s/%s/%s", dir1, dir2, hashHex)
}

// StoreFile 存储文件
func (m *MockCASStorage) StoreFile(ctx context.Context, contentHash []byte, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	hashKey := hex.EncodeToString(contentHash)
	m.files[hashKey] = data
	return nil
}

// ReadFile 读取文件
func (m *MockCASStorage) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hashKey := hex.EncodeToString(contentHash)
	data, exists := m.files[hashKey]
	if !exists {
		return nil, fmt.Errorf("文件不存在: %x", contentHash[:8])
	}
	return data, nil
}

// FileExists 检查文件是否存在
func (m *MockCASStorage) FileExists(contentHash []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(contentHash) != 32 {
		return false
	}
	hashKey := hex.EncodeToString(contentHash)
	_, exists := m.files[hashKey]
	return exists
}

// GetFiles 获取所有文件（用于测试）
func (m *MockCASStorage) GetFiles() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for k, v := range m.files {
		result[k] = v
	}
	return result
}

// Clear 清空所有文件（用于测试）
func (m *MockCASStorage) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files = make(map[string][]byte)
}

