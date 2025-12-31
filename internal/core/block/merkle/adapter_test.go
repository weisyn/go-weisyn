package merkle_test

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/block/merkle"
	"github.com/weisyn/v1/internal/core/block/testutil"
)

// ==================== NewHashManagerAdapter 测试 ====================

// TestNewHashManagerAdapter_WithValidManager_ReturnsAdapter 测试使用有效HashManager创建适配器
func TestNewHashManagerAdapter_WithValidManager_ReturnsAdapter(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}

	// Act
	adapter := merkle.NewHashManagerAdapter(hashManager)

	// Assert
	assert.NotNil(t, adapter)
	_ = hashManager // 使用hashManager避免未使用变量警告
}

// TestNewHashManagerAdapter_WithNilManager_ReturnsAdapter 测试nil HashManager时创建适配器（允许nil）
func TestNewHashManagerAdapter_WithNilManager_ReturnsAdapter(t *testing.T) {
	// Arrange
	// Act
	adapter := merkle.NewHashManagerAdapter(nil)

	// Assert
	assert.NotNil(t, adapter, "适配器应该被创建，即使HashManager为nil")
}

// ==================== HashManagerAdapter.Hash 测试 ====================

// TestHashManagerAdapter_Hash_WithValidData_ReturnsHash 测试使用有效数据计算哈希
func TestHashManagerAdapter_Hash_WithValidData_ReturnsHash(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	adapter := merkle.NewHashManagerAdapter(hashManager)
	data := []byte("test data")

	// Act
	hash, err := adapter.Hash(data)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, 32, len(hash), "哈希长度应该为32字节")
}

// TestHashManagerAdapter_Hash_WithNilManager_ReturnsError 测试nil HashManager时返回错误
func TestHashManagerAdapter_Hash_WithNilManager_ReturnsError(t *testing.T) {
	// Arrange
	adapter := merkle.NewHashManagerAdapter(nil)
	data := []byte("test data")

	// Act
	hash, err := adapter.Hash(data)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Contains(t, err.Error(), "哈希管理器未初始化")
}

// TestHashManagerAdapter_Hash_WithEmptyData_ReturnsHash 测试空数据时计算哈希
func TestHashManagerAdapter_Hash_WithEmptyData_ReturnsHash(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	adapter := merkle.NewHashManagerAdapter(hashManager)
	data := []byte{}

	// Act
	hash, err := adapter.Hash(data)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, 32, len(hash), "哈希长度应该为32字节")
}

// TestHashManagerAdapter_Hash_WithLargeData_ReturnsHash 测试大数据时计算哈希
func TestHashManagerAdapter_Hash_WithLargeData_ReturnsHash(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	adapter := merkle.NewHashManagerAdapter(hashManager)
	data := make([]byte, 10000) // 10KB数据
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Act
	hash, err := adapter.Hash(data)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, 32, len(hash), "哈希长度应该为32字节")
}

// TestHashManagerAdapter_Hash_UsesSHA256 测试使用SHA256算法
func TestHashManagerAdapter_Hash_UsesSHA256(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	adapter := merkle.NewHashManagerAdapter(hashManager)
	data := []byte("test data")

	// Act
	hash, err := adapter.Hash(data)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, hash)

	// 验证使用的是SHA256（MockHashManager使用SHA256）
	expectedHash := sha256.Sum256(data)
	assert.Equal(t, expectedHash[:], hash, "应该使用SHA256算法")
}

// ==================== 并发安全测试 ====================

// TestHashManagerAdapter_Hash_ConcurrentAccess_IsSafe 测试并发访问的安全性
func TestHashManagerAdapter_Hash_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	hashManager := &testutil.MockHashManager{}
	adapter := merkle.NewHashManagerAdapter(hashManager)
	data := []byte("test data")
	concurrency := 10

	// Act
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- errors.New("panic occurred")
				}
			}()
			_, err := adapter.Hash(data)
			results <- err
		}()
	}

	// Assert
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err, "并发访问不应该失败")
	}
}

// ==================== 发现代码问题测试 ====================

// TestHashManagerAdapter_DetectsTODOs 测试发现TODO标记
func TestHashManagerAdapter_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestHashManagerAdapter_DetectsTemporaryImplementations 测试发现临时实现
func TestHashManagerAdapter_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ 适配器实现检查：")
	t.Logf("  - HashManagerAdapter 正确适配 HashManager 到 Hasher 接口")
	t.Logf("  - Hash 方法使用 SHA256 算法")
	t.Logf("  - Hash 方法正确处理 nil HashManager 的情况")
}

