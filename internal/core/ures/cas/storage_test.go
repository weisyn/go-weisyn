// Package cas 存储操作测试
package cas

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ures/testutil"
)

// TestStoreFile_WithValidInput_StoresFile 测试使用有效输入存储文件
func TestStoreFile_WithValidInput_StoresFile(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)

	// Act
	err = service.StoreFile(ctx, contentHash[:], data)

	// Assert
	assert.NoError(t, err, "应该成功存储文件")
	
	// 验证文件已存储
	exists := service.FileExists(contentHash[:])
	assert.True(t, exists, "文件应该存在")
}

// TestStoreFile_WithInvalidHashLength_ReturnsError 测试哈希长度无效时返回错误
func TestStoreFile_WithInvalidHashLength_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	invalidHash := []byte{0x01, 0x02} // 只有2字节，不是32字节

	// Act
	err = service.StoreFile(ctx, invalidHash, data)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "无效的哈希长度", "错误信息应该包含哈希长度相关描述")
}

// TestStoreFile_WithEmptyData_ReturnsError 测试数据为空时返回错误
func TestStoreFile_WithEmptyData_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	emptyData := []byte{}
	contentHash := sha256.Sum256([]byte("test"))

	// Act
	err = service.StoreFile(ctx, contentHash[:], emptyData)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, ErrEmptyData, err, "应该返回ErrEmptyData错误")
}

// TestStoreFile_WithSameContent_IsIdempotent 测试相同内容存储的幂等性
func TestStoreFile_WithSameContent_IsIdempotent(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)

	// Act - 第一次存储
	err1 := service.StoreFile(ctx, contentHash[:], data)
	require.NoError(t, err1, "第一次存储应该成功")

	// Act - 第二次存储相同内容
	err2 := service.StoreFile(ctx, contentHash[:], data)

	// Assert
	assert.NoError(t, err2, "第二次存储应该成功（幂等性）")
	
	// 验证文件仍然存在
	exists := service.FileExists(contentHash[:])
	assert.True(t, exists, "文件应该存在")
}

// TestReadFile_WithValidHash_ReturnsData 测试使用有效哈希读取文件
func TestReadFile_WithValidHash_ReturnsData(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)
	
	// 先存储文件
	err = service.StoreFile(ctx, contentHash[:], data)
	require.NoError(t, err, "应该成功存储文件")

	// Act
	readData, err := service.ReadFile(ctx, contentHash[:])

	// Assert
	assert.NoError(t, err, "应该成功读取文件")
	assert.Equal(t, data, readData, "读取的数据应该与原始数据一致")
}

// TestReadFile_WithInvalidHashLength_ReturnsError 测试哈希长度无效时返回错误
func TestReadFile_WithInvalidHashLength_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	invalidHash := []byte{0x01, 0x02} // 只有2字节

	// Act
	data, err := service.ReadFile(ctx, invalidHash)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, data, "数据应该为nil")
	assert.Contains(t, err.Error(), "无效的哈希长度", "错误信息应该包含哈希长度相关描述")
}

// TestReadFile_WithNonExistentFile_ReturnsError 测试读取不存在的文件时返回错误
func TestReadFile_WithNonExistentFile_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	nonExistentHash := sha256.Sum256([]byte("non-existent file"))

	// Act
	data, err := service.ReadFile(ctx, nonExistentHash[:])

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, data, "数据应该为nil")
	assert.Contains(t, err.Error(), "读取文件失败", "错误信息应该包含读取失败相关描述")
}

// TestFileExists_WithExistingFile_ReturnsTrue 测试存在文件时返回true
func TestFileExists_WithExistingFile_ReturnsTrue(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)
	
	// 先存储文件
	err = service.StoreFile(ctx, contentHash[:], data)
	require.NoError(t, err, "应该成功存储文件")

	// Act
	exists := service.FileExists(contentHash[:])

	// Assert
	assert.True(t, exists, "文件应该存在")
}

// TestFileExists_WithNonExistentFile_ReturnsFalse 测试不存在文件时返回false
func TestFileExists_WithNonExistentFile_ReturnsFalse(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	nonExistentHash := sha256.Sum256([]byte("non-existent file"))

	// Act
	exists := service.FileExists(nonExistentHash[:])

	// Assert
	assert.False(t, exists, "文件不应该存在")
}

// TestFileExists_WithInvalidHashLength_ReturnsFalse 测试哈希长度无效时返回false
func TestFileExists_WithInvalidHashLength_ReturnsFalse(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	invalidHash := []byte{0x01, 0x02} // 只有2字节

	// Act
	exists := service.FileExists(invalidHash)

	// Assert
	assert.False(t, exists, "无效哈希应该返回false")
}

// TestStoreFile_ConcurrentAccess_IsSafe 测试并发存储的安全性
func TestStoreFile_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// 创建多个不同的文件
	numFiles := 10
	hashes := make([][]byte, numFiles)
	datas := make([][]byte, numFiles)
	
	for i := 0; i < numFiles; i++ {
		data := []byte{byte(i)}
		hash := sha256.Sum256(data)
		hashes[i] = hash[:]
		datas[i] = data
	}

	// Act - 并发存储
	done := make(chan error, numFiles)
	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			err := service.StoreFile(ctx, hashes[idx], datas[idx])
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
		exists := service.FileExists(hashes[i])
		assert.True(t, exists, "文件%d应该存在", i)
	}
}

// TestReadFile_ConcurrentAccess_IsSafe 测试并发读取的安全性
func TestReadFile_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)
	
	// 先存储文件
	err = service.StoreFile(ctx, contentHash[:], data)
	require.NoError(t, err, "应该成功存储文件")

	// Act - 并发读取
	numReaders := 10
	done := make(chan error, numReaders)
	for i := 0; i < numReaders; i++ {
		go func() {
			readData, err := service.ReadFile(ctx, contentHash[:])
			if err != nil {
				done <- err
				return
			}
			if string(readData) != string(data) {
				done <- assert.AnError
				return
			}
			done <- nil
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < numReaders; i++ {
		err := <-done
		assert.NoError(t, err, "并发读取应该成功")
	}
}

// TestStoreFile_WithDifferentContent_SameHash_Overwrites 测试相同哈希不同内容的情况
// 注意：这实际上不应该发生，因为CAS是基于内容哈希的
// 但我们需要测试如果发生这种情况的行为
func TestStoreFile_WithDifferentContent_SameHash_Overwrites(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// 创建两个不同的数据，但使用相同的哈希（这在现实中不可能，但测试边界情况）
	data1 := []byte("test file content 1")
	hash1 := sha256.Sum256(data1)
	
	// 存储第一个文件
	err1 := service.StoreFile(ctx, hash1[:], data1)
	require.NoError(t, err1, "第一次存储应该成功")

	// Act - 使用相同哈希但不同内容（模拟哈希冲突）
	data2 := []byte("test file content 2")
	err2 := service.StoreFile(ctx, hash1[:], data2)

	// Assert - 由于幂等性检查，第二次存储应该被跳过
	assert.NoError(t, err2, "第二次存储应该成功（幂等性）")
	
	// 读取文件，应该还是第一次的内容（因为文件已存在，第二次存储被跳过）
	readData, err := service.ReadFile(ctx, hash1[:])
	require.NoError(t, err, "应该能读取文件")
	assert.Equal(t, data1, readData, "读取的数据应该是第一次存储的数据（幂等性）")
}

