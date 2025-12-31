package adapter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// 错误处理测试
// ============================================================================
//
// 🎯 **测试目的**：发现错误处理的缺陷和BUG
//
// ============================================================================

// TestErrorConstants 测试错误常量
func TestErrorConstants(t *testing.T) {
	assert.NotNil(t, ErrDeprecatedAPI, "ErrDeprecatedAPI不应该为nil")
	assert.NotNil(t, ErrUnsupportedVersion, "ErrUnsupportedVersion不应该为nil")
	assert.NotNil(t, ErrIncompatibleInterface, "ErrIncompatibleInterface不应该为nil")
	assert.NotNil(t, ErrLegacyModeOnly, "ErrLegacyModeOnly不应该为nil")
	assert.NotNil(t, ErrMigrationRequired, "ErrMigrationRequired不应该为nil")
	assert.NotNil(t, ErrAdapterNotInitialized, "ErrAdapterNotInitialized不应该为nil")
	assert.NotNil(t, ErrLegacyComponentUnavailable, "ErrLegacyComponentUnavailable不应该为nil")
	assert.NotNil(t, ErrNewComponentUnavailable, "ErrNewComponentUnavailable不应该为nil")
}

// TestWrapDeprecatedAPIError 测试包装API已废弃错误
func TestWrapDeprecatedAPIError(t *testing.T) {
	api := "old_api"
	replacement := "new_api"
	err := WrapDeprecatedAPIError(api, replacement)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API已废弃")
	assert.Contains(t, err.Error(), api)
	assert.Contains(t, err.Error(), replacement)
	assert.True(t, errors.Is(err, ErrDeprecatedAPI), "应该包装原始错误")
}

// TestWrapUnsupportedVersionError 测试包装版本不支持错误
func TestWrapUnsupportedVersionError(t *testing.T) {
	version := "1.0"
	minVersion := "2.0"
	err := WrapUnsupportedVersionError(version, minVersion)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "版本不支持")
	assert.Contains(t, err.Error(), version)
	assert.Contains(t, err.Error(), minVersion)
	assert.True(t, errors.Is(err, ErrUnsupportedVersion), "应该包装原始错误")
}

// TestWrapMigrationRequiredError 测试包装需要迁移错误
func TestWrapMigrationRequiredError(t *testing.T) {
	from := "old_version"
	to := "new_version"
	err := WrapMigrationRequiredError(from, to)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "需要迁移")
	assert.Contains(t, err.Error(), from)
	assert.Contains(t, err.Error(), to)
	assert.True(t, errors.Is(err, ErrMigrationRequired), "应该包装原始错误")
}

