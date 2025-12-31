// Package writer 测试文件
package writer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ures/testutil"
)

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(casStorage, hasher, logger)

	// Assert
	require.NoError(t, err, "应该成功创建服务")
	assert.NotNil(t, service, "服务实例不应为nil")
}

// TestNewService_WithNilCASStorage_ReturnsError 测试casStorage为nil时返回错误
func TestNewService_WithNilCASStorage_ReturnsError(t *testing.T) {
	// Arrange
	hasher := &testutil.MockHashManager{}
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(nil, hasher, logger)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, service, "服务实例应为nil")
	assert.Equal(t, ErrCASStorageNil, err, "应该返回ErrCASStorageNil错误")
}

// TestNewService_WithNilHasher_ReturnsError 测试hasher为nil时返回错误
func TestNewService_WithNilHasher_ReturnsError(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	logger := &testutil.MockLogger{}

	// Act
	service, err := NewService(casStorage, nil, logger)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, service, "服务实例应为nil")
	assert.Equal(t, ErrHasherNil, err, "应该返回ErrHasherNil错误")
}

// TestNewService_WithNilLogger_ReturnsService 测试logger为nil时仍能创建服务
func TestNewService_WithNilLogger_ReturnsService(t *testing.T) {
	// Arrange
	casStorage := testutil.NewMockCASStorage()
	hasher := &testutil.MockHashManager{}

	// Act
	service, err := NewService(casStorage, hasher, nil)

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
	// 应该返回第一个nil依赖的错误（casStorage）
	assert.Equal(t, ErrCASStorageNil, err, "应该返回ErrCASStorageNil错误")
}

