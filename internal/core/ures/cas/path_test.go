// Package cas 路径构建测试
package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ures/testutil"
)

// TestBuildFilePath_WithValidHash_ReturnsCorrectPath 测试使用有效哈希构建路径
func TestBuildFilePath_WithValidHash_ReturnsCorrectPath(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)

	// Act
	path := service.BuildFilePath(contentHash[:])

	// Assert
	assert.NotEmpty(t, path, "路径不应为空")
	
	// 验证路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	// 将哈希转换为十六进制字符串
	hashHex := hex.EncodeToString(contentHash[:])
	expectedDir1 := hashHex[0:2]
	expectedDir2 := hashHex[2:4]
	expectedFullHash := hashHex
	
	expectedPath := filepath.Join(expectedDir1, expectedDir2, expectedFullHash)
	assert.Equal(t, expectedPath, path, "路径格式应该正确")
}

// TestBuildFilePath_WithValidHash_ReturnsThreeLevelPath 测试路径包含三级目录
func TestBuildFilePath_WithValidHash_ReturnsThreeLevelPath(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	data := []byte("test")
	contentHash := sha256.Sum256(data)

	// Act
	path := service.BuildFilePath(contentHash[:])

	// Assert
	assert.NotEmpty(t, path, "路径不应为空")
	
	// 验证路径包含分隔符（三级目录）
	// 路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	// 应该包含2个分隔符
	separatorCount := 0
	for _, char := range path {
		if char == '/' || char == filepath.Separator {
			separatorCount++
		}
	}
	assert.GreaterOrEqual(t, separatorCount, 2, "路径应该包含至少2个分隔符（三级目录）")
}

// TestBuildFilePath_WithInvalidHashLength_ReturnsEmpty 测试哈希长度无效时返回空字符串
func TestBuildFilePath_WithInvalidHashLength_ReturnsEmpty(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	invalidHash := []byte{0x01, 0x02} // 只有2字节，不是32字节

	// Act
	path := service.BuildFilePath(invalidHash)

	// Assert
	assert.Empty(t, path, "无效哈希应该返回空字符串")
}

// TestBuildFilePath_WithEmptyHash_ReturnsEmpty 测试空哈希时返回空字符串
func TestBuildFilePath_WithEmptyHash_ReturnsEmpty(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	emptyHash := []byte{}

	// Act
	path := service.BuildFilePath(emptyHash)

	// Assert
	assert.Empty(t, path, "空哈希应该返回空字符串")
}

// TestBuildFilePath_WithNilHash_ReturnsEmpty 测试nil哈希时返回空字符串
func TestBuildFilePath_WithNilHash_ReturnsEmpty(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)

	// Act
	path := service.BuildFilePath(nil)

	// Assert
	assert.Empty(t, path, "nil哈希应该返回空字符串")
}

// TestBuildFilePath_WithDifferentHashes_ReturnsDifferentPaths 测试不同哈希返回不同路径
func TestBuildFilePath_WithDifferentHashes_ReturnsDifferentPaths(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	data1 := []byte("test file 1")
	data2 := []byte("test file 2")
	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)

	// Act
	path1 := service.BuildFilePath(hash1[:])
	path2 := service.BuildFilePath(hash2[:])

	// Assert
	assert.NotEmpty(t, path1, "路径1不应为空")
	assert.NotEmpty(t, path2, "路径2不应为空")
	assert.NotEqual(t, path1, path2, "不同哈希应该返回不同路径")
}

// TestBuildFilePath_WithSameHash_ReturnsSamePath 测试相同哈希返回相同路径
func TestBuildFilePath_WithSameHash_ReturnsSamePath(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	data := []byte("test file content")
	contentHash := sha256.Sum256(data)

	// Act
	path1 := service.BuildFilePath(contentHash[:])
	path2 := service.BuildFilePath(contentHash[:])

	// Assert
	assert.Equal(t, path1, path2, "相同哈希应该返回相同路径")
}

// TestBuildFilePath_PathFormat_IsCorrect 测试路径格式正确性
func TestBuildFilePath_PathFormat_IsCorrect(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	// 使用已知的哈希值进行测试
	knownHash := []byte{
		0x5a, 0x6b, 0x7c, 0x8d, 0x9e, 0x0f, 0x1a, 0x2b,
		0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
		0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f,
		0x9a, 0x0b, 0x1c, 0x2d, 0x3e, 0x4f, 0x5a, 0x6b,
	}

	// Act
	path := service.BuildFilePath(knownHash)

	// Assert
	assert.NotEmpty(t, path, "路径不应为空")
	
	// 验证路径格式：{hash[0:2]}/{hash[2:4]}/{fullHash}
	hashHex := hex.EncodeToString(knownHash)
	expectedDir1 := hashHex[0:2]   // "5a"
	expectedDir2 := hashHex[2:4]   // "6b"
	expectedFullHash := hashHex    // 完整64字符
	
	expectedPath := filepath.Join(expectedDir1, expectedDir2, expectedFullHash)
	assert.Equal(t, expectedPath, path, "路径格式应该正确")
}

// TestBuildFilePath_WithZeroHash_ReturnsPath 测试全零哈希也能构建路径
func TestBuildFilePath_WithZeroHash_ReturnsPath(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	zeroHash := make([]byte, 32) // 32字节全零

	// Act
	path := service.BuildFilePath(zeroHash)

	// Assert
	assert.NotEmpty(t, path, "全零哈希也应该能构建路径")
	
	// 验证路径格式
	// 全零的十六进制是 "00" * 32 = "0000...0000" (64个0)
	expectedPath := filepath.Join("00", "00", "0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, expectedPath, path, "全零哈希的路径应该正确")
}

// TestBuildFilePath_WithMaxHash_ReturnsPath 测试全F哈希也能构建路径
func TestBuildFilePath_WithMaxHash_ReturnsPath(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}
	service, err := NewService(fileStore, hasher, logger)
	require.NoError(t, err)
	
	maxHash := make([]byte, 32)
	for i := range maxHash {
		maxHash[i] = 0xFF
	}

	// Act
	path := service.BuildFilePath(maxHash)

	// Assert
	assert.NotEmpty(t, path, "全F哈希也应该能构建路径")
	
	// 验证路径格式
	// 全F的十六进制是 "ff" * 32 = "ffff...ffff" (64个f)
	expectedPath := filepath.Join("ff", "ff", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	assert.Equal(t, expectedPath, path, "全F哈希的路径应该正确")
}

