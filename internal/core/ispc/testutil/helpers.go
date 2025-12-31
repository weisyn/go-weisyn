// Package testutil 提供 ISPC 模块测试的辅助工具
//
// 🧪 **测试辅助函数**
//
// 本文件提供测试辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
//
// ⚠️ **注意**：本文件不包含依赖具体组件的辅助函数，避免循环依赖。
// 具体组件的测试辅助函数应该在各自的测试文件中定义，使用testutil中的Mock对象。

package testutil

import (
	"time"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// 确保 MockConfigProvider 实现了 config.Provider 接口
var _ config.Provider = (*MockConfigProvider)(nil)
var _ config.Provider = (*ConfigurableMockConfigProvider)(nil)

// NewTestClock 创建测试用的时钟
func NewTestClock() *MockClock {
	return NewMockClock(NewTestTime())
}

// NewTestClockWithTime 创建带指定时间的测试时钟
func NewTestClockWithTime(t time.Time) *MockClock {
	return NewMockClock(t)
}

// NewTestLogger 创建测试用的Logger
func NewTestLogger() log.Logger {
	return &MockLogger{}
}

// NewTestBehavioralLogger 创建行为Logger（记录调用）
func NewTestBehavioralLogger() *BehavioralMockLogger {
	return &BehavioralMockLogger{
		logs: make([]string, 0),
	}
}

// NewTestHashManager 创建测试用的HashManager
func NewTestHashManager() crypto.HashManager {
	return &MockHashManager{}
}

// NewTestSignatureManager 创建测试用的SignatureManager
func NewTestSignatureManager() crypto.SignatureManager {
	return &MockSignatureManager{}
}

// NewTestConfigProvider 创建测试用的ConfigProvider
func NewTestConfigProvider() config.Provider {
	return &MockConfigProvider{}
}

// NewTestConfigurableConfigProvider 创建可配置的ConfigProvider
func NewTestConfigurableConfigProvider() *ConfigurableMockConfigProvider {
	return &ConfigurableMockConfigProvider{}
}

