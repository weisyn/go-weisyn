// Package writer 存储操作测试
package writer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/weisyn/v1/internal/core/infrastructure/writegate" // 导入实现包以触发 init()
	"github.com/weisyn/v1/internal/core/ures/testutil"
)

// TestStoreResourceFile_WithValidFile_StoresFile 测试使用有效文件存储资源
func TestStoreResourceFile_WithValidFile_StoresFile(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	data := []byte("test file content")
	
	// 创建临时文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wasm")
	err = os.WriteFile(tmpFile, data, 0644)
	require.NoError(t, err, "应该成功创建临时文件")

	// 计算预期哈希
	expectedHash := sha256.Sum256(data)

	// Act
	contentHash, err := service.StoreResourceFile(ctx, tmpFile)

	// Assert
	assert.NoError(t, err, "应该成功存储文件")
	assert.Equal(t, expectedHash[:], contentHash, "返回的哈希应该正确")
	
	// 验证文件已存储到CAS
	assert.True(t, casStorage.FileExists(contentHash), "文件应该已存储到CAS")
	
	// 验证存储的数据正确
	storedData, err := casStorage.ReadFile(ctx, contentHash)
	require.NoError(t, err, "应该能读取存储的文件")
	assert.Equal(t, data, storedData, "存储的数据应该与原始数据一致")
}

// TestStoreResourceFile_WithNonExistentFile_ReturnsError 测试不存在的文件时返回错误
func TestStoreResourceFile_WithNonExistentFile_ReturnsError(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	nonExistentFile := "/path/to/non/existent/file.wasm"

	// Act
	contentHash, err := service.StoreResourceFile(ctx, nonExistentFile)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, contentHash, "哈希应该为nil")
	assert.Contains(t, err.Error(), "读取源文件失败", "错误信息应该包含读取失败相关描述")
}

// TestStoreResourceFile_WithSameContent_IsIdempotent 测试相同内容存储的幂等性
func TestStoreResourceFile_WithSameContent_IsIdempotent(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	data := []byte("test file content")
	
	// 创建两个临时文件，内容相同
	tmpDir := t.TempDir()
	tmpFile1 := filepath.Join(tmpDir, "test1.wasm")
	tmpFile2 := filepath.Join(tmpDir, "test2.wasm")
	err = os.WriteFile(tmpFile1, data, 0644)
	require.NoError(t, err)
	err = os.WriteFile(tmpFile2, data, 0644)
	require.NoError(t, err)

	// Act - 第一次存储
	contentHash1, err1 := service.StoreResourceFile(ctx, tmpFile1)
	require.NoError(t, err1, "第一次存储应该成功")

	// Act - 第二次存储相同内容
	contentHash2, err2 := service.StoreResourceFile(ctx, tmpFile2)

	// Assert
	assert.NoError(t, err2, "第二次存储应该成功（幂等性）")
	assert.Equal(t, contentHash1, contentHash2, "相同内容的哈希应该相同")
	
	// 验证文件已存储到CAS
	assert.True(t, casStorage.FileExists(contentHash1), "文件应该已存储到CAS")
}

// TestStoreResourceFile_WithEmptyFile_StoresFile 测试空文件也能存储
func TestStoreResourceFile_WithEmptyFile_StoresFile(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	emptyData := []byte{}
	
	// 创建空文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.wasm")
	err = os.WriteFile(tmpFile, emptyData, 0644)
	require.NoError(t, err, "应该成功创建空文件")

	// 计算预期哈希
	expectedHash := sha256.Sum256(emptyData)

	// Act
	contentHash, err := service.StoreResourceFile(ctx, tmpFile)

	// Assert
	assert.NoError(t, err, "应该成功存储空文件")
	assert.Equal(t, expectedHash[:], contentHash, "返回的哈希应该正确")
	
	// 验证文件已存储到CAS
	assert.True(t, casStorage.FileExists(contentHash), "空文件应该已存储到CAS")
}

// TestStoreResourceFile_WithLargeFile_StoresFile 测试大文件也能存储
func TestStoreResourceFile_WithLargeFile_StoresFile(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	// 创建1MB的数据
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	
	// 创建大文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.wasm")
	err = os.WriteFile(tmpFile, largeData, 0644)
	require.NoError(t, err, "应该成功创建大文件")

	// 计算预期哈希
	expectedHash := sha256.Sum256(largeData)

	// Act
	contentHash, err := service.StoreResourceFile(ctx, tmpFile)

	// Assert
	assert.NoError(t, err, "应该成功存储大文件")
	assert.Equal(t, expectedHash[:], contentHash, "返回的哈希应该正确")
	
	// 验证文件已存储到CAS
	assert.True(t, casStorage.FileExists(contentHash), "大文件应该已存储到CAS")
	
	// 验证存储的数据正确
	storedData, err := casStorage.ReadFile(ctx, contentHash)
	require.NoError(t, err, "应该能读取存储的大文件")
	assert.Equal(t, len(largeData), len(storedData), "存储的文件大小应该正确")
	assert.Equal(t, largeData[:100], storedData[:100], "存储的数据前100字节应该正确")
}

// TestStoreResourceFile_ConcurrentAccess_IsSafe 测试并发存储的安全性
func TestStoreResourceFile_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	
	// 创建多个不同的文件
	numFiles := 10
	tmpDir := t.TempDir()
	files := make([]string, numFiles)
	hashes := make([][]byte, numFiles)
	
	for i := 0; i < numFiles; i++ {
		data := []byte{byte(i)}
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.wasm", i))
		err := os.WriteFile(tmpFile, data, 0644)
		require.NoError(t, err)
		files[i] = tmpFile
		hash := sha256.Sum256(data)
		hashes[i] = hash[:]
	}

	// Act - 并发存储
	done := make(chan error, numFiles)
	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			_, err := service.StoreResourceFile(ctx, files[idx])
			done <- err
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numFiles; i++ {
		err := <-done
		assert.NoError(t, err, "并发存储应该成功")
	}

	// Assert - 验证所有文件都已存储
	for i := 0; i < numFiles; i++ {
		exists := casStorage.FileExists(hashes[i])
		assert.True(t, exists, "文件%d应该存在", i)
	}
}

// TestStoreResourceFile_WithDifferentFiles_StoresDifferentHashes 测试不同文件返回不同哈希
func TestStoreResourceFile_WithDifferentFiles_StoresDifferentHashes(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(casStorage, hasher, logger)
	require.NoError(t, err)

	ctx := context.Background()
	
	// 创建两个不同的文件
	tmpDir := t.TempDir()
	tmpFile1 := filepath.Join(tmpDir, "test1.wasm")
	tmpFile2 := filepath.Join(tmpDir, "test2.wasm")
	err = os.WriteFile(tmpFile1, []byte("file 1 content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(tmpFile2, []byte("file 2 content"), 0644)
	require.NoError(t, err)

	// Act
	contentHash1, err1 := service.StoreResourceFile(ctx, tmpFile1)
	require.NoError(t, err1, "第一次存储应该成功")
	
	contentHash2, err2 := service.StoreResourceFile(ctx, tmpFile2)
	require.NoError(t, err2, "第二次存储应该成功")

	// Assert
	assert.NotEqual(t, contentHash1, contentHash2, "不同内容的文件应该返回不同哈希")
	
	// 验证两个文件都已存储
	assert.True(t, casStorage.FileExists(contentHash1), "文件1应该已存储")
	assert.True(t, casStorage.FileExists(contentHash2), "文件2应该已存储")
}

