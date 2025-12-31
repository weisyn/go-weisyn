// Package adapter provides error definitions for host ABI adapter operations.
package adapter

import (
	"errors"
	"fmt"
)

// ==================== 业务错误 ====================

var (
	// ErrDeprecatedAPI API已废弃
	ErrDeprecatedAPI = errors.New("API已废弃")

	// ErrUnsupportedVersion 版本不支持
	ErrUnsupportedVersion = errors.New("版本不支持")

	// ErrIncompatibleInterface 接口不兼容
	ErrIncompatibleInterface = errors.New("接口不兼容")

	// ErrLegacyModeOnly 仅遗留模式
	ErrLegacyModeOnly = errors.New("仅遗留模式")

	// ErrMigrationRequired 需要迁移
	ErrMigrationRequired = errors.New("需要迁移")
)

// ==================== 系统错误 ====================

var (
	// ErrAdapterNotInitialized 适配器未初始化
	ErrAdapterNotInitialized = errors.New("适配器未初始化")

	// ErrLegacyComponentUnavailable 遗留组件不可用
	ErrLegacyComponentUnavailable = errors.New("遗留组件不可用")

	// ErrNewComponentUnavailable 新组件不可用
	ErrNewComponentUnavailable = errors.New("新组件不可用")
)

// ==================== 错误包装函数 ====================

// WrapDeprecatedAPIError 包装API已废弃错误
func WrapDeprecatedAPIError(api string, replacement string) error {
	return fmt.Errorf("%w: api=%s, use %s instead", ErrDeprecatedAPI, api, replacement)
}

// WrapUnsupportedVersionError 包装版本不支持错误
func WrapUnsupportedVersionError(version, minVersion string) error {
	return fmt.Errorf("%w: version=%s, minVersion=%s", ErrUnsupportedVersion, version, minVersion)
}

// WrapMigrationRequiredError 包装需要迁移错误
func WrapMigrationRequiredError(from, to string) error {
	return fmt.Errorf("%w: from=%s to=%s", ErrMigrationRequired, from, to)
}
