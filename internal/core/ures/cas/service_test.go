// Package cas 测试文件
package cas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ures/testutil"
)

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(fileStore, hasher, logger)

	// Assert
	require.NoError(t, err, "应该成功创建服务")
	assert.NotNil(t, service, "服务实例不应为nil")
}

// TestNewService_WithNilFileStore_ReturnsError 测试fileStore为nil时返回错误
func TestNewService_WithNilFileStore_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(nil, hasher, logger)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, service, "服务实例应为nil")
	assert.Equal(t, ErrFileStoreNil, err, "应该返回ErrFileStoreNil错误")
}

// TestNewService_WithNilHasher_ReturnsError 测试hasher为nil时返回错误
func TestNewService_WithNilHasher_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(fileStore, nil, logger)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, service, "服务实例应为nil")
	assert.Equal(t, ErrHasherNil, err, "应该返回ErrHasherNil错误")
}

// TestNewService_WithNilLogger_ReturnsService 测试logger为nil时仍能创建服务
func TestNewService_WithNilLogger_ReturnsService(t *testing.T) {
	// Arrange
	fileStore := testutil.NewMockFileStore()
	hasher := &testutil.MockHashManager{}

	// Act
	service, err := NewService(fileStore, hasher, nil)

	// Assert
	require.NoError(t, err, "logger为nil时应该仍能创建服务")
	assert.NotNil(t, service, "服务实例不应为nil")
}

// TestNewService_WithAllNilDependencies_ReturnsError 测试所有依赖为nil时返回错误
func TestNewService_WithAllNilDependencies_ReturnsError(t *testing.T) {
	// Act
	service, err := NewService(nil, nil, nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, service, "服务实例应为nil")
	// 应该返回第一个nil依赖的错误（fileStore）
	assert.Equal(t, ErrFileStoreNil, err, "应该返回ErrFileStoreNil错误")
}

