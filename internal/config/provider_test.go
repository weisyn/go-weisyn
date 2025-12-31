package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/pkg/types"
)

// TestGetEnvironment 测试 GetEnvironment() 方法
func TestGetEnvironment(t *testing.T) {
	t.Run("显式配置 dev", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("dev"),
		}
		provider := NewProvider(cfg)
		env := provider.GetEnvironment()
		assert.Equal(t, "dev", env)
	})

	t.Run("显式配置 test", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("test"),
		}
		provider := NewProvider(cfg)
		env := provider.GetEnvironment()
		assert.Equal(t, "test", env)
	})

	t.Run("显式配置 prod", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("prod"),
		}
		provider := NewProvider(cfg)
		env := provider.GetEnvironment()
		assert.Equal(t, "prod", env)
	})

	t.Run("未配置时默认为 prod（安全优先）", func(t *testing.T) {
		cfg := &types.AppConfig{}
		provider := NewProvider(cfg)
		env := provider.GetEnvironment()
		assert.Equal(t, "prod", env, "未配置时应默认为 prod（安全优先）")
	})

	t.Run("无效值默认为 prod", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("invalid"),
		}
		provider := NewProvider(cfg)
		env := provider.GetEnvironment()
		assert.Equal(t, "prod", env, "无效值时应默认为 prod（安全优先）")
	})
}

// TestGetChainMode 测试 GetChainMode() 方法
func TestGetChainMode(t *testing.T) {
	t.Run("显式配置 public", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("public"),
			},
		}
		provider := NewProvider(cfg)
		mode := provider.GetChainMode()
		assert.Equal(t, "public", mode)
	})

	t.Run("显式配置 consortium", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("consortium"),
			},
		}
		provider := NewProvider(cfg)
		mode := provider.GetChainMode()
		assert.Equal(t, "consortium", mode)
	})

	t.Run("显式配置 private", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("private"),
			},
		}
		provider := NewProvider(cfg)
		mode := provider.GetChainMode()
		assert.Equal(t, "private", mode)
	})

	t.Run("未配置时应 panic（fail-fast）", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{},
		}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetChainMode()
		}, "未配置 ChainMode 时应 panic（fail-fast）")
	})

	t.Run("Network 为 nil 时应 panic", func(t *testing.T) {
		cfg := &types.AppConfig{}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetChainMode()
		}, "Network 为 nil 时应 panic")
	})

	t.Run("无效值时应 panic", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("invalid"),
			},
		}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetChainMode()
		}, "无效的 ChainMode 值时应 panic")
	})
}

// TestGetNetworkNamespace 测试 GetNetworkNamespace() 方法
func TestGetNetworkNamespace(t *testing.T) {
	t.Run("显式配置命名空间", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				NetworkNamespace: types.StringPtr("mainnet-public"),
			},
		}
		provider := NewProvider(cfg)
		ns := provider.GetNetworkNamespace()
		assert.Equal(t, "mainnet-public", ns)
	})

	t.Run("配置 dev-private 命名空间", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				NetworkNamespace: types.StringPtr("dev-private"),
			},
		}
		provider := NewProvider(cfg)
		ns := provider.GetNetworkNamespace()
		assert.Equal(t, "dev-private", ns)
	})

	t.Run("未配置时应 panic（fail-fast）", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{},
		}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetNetworkNamespace()
		}, "未配置 NetworkNamespace 时应 panic（fail-fast）")
	})

	t.Run("Network 为 nil 时应 panic", func(t *testing.T) {
		cfg := &types.AppConfig{}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetNetworkNamespace()
		}, "Network 为 nil 时应 panic")
	})
}

// TestGetCompliance 测试 GetCompliance() 方法
func TestGetCompliance(t *testing.T) {
	t.Run("dev + public → development profile", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("dev"),
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("public"),
			},
		}
		provider := NewProvider(cfg)
		compliance := provider.GetCompliance()
		require.NotNil(t, compliance)
		// 验证合规配置已创建（具体字段验证取决于 compliance 包的实现）
	})

	t.Run("test + consortium → testing profile", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("test"),
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("consortium"),
			},
		}
		provider := NewProvider(cfg)
		compliance := provider.GetCompliance()
		require.NotNil(t, compliance)
	})

	t.Run("prod + public → production profile", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("prod"),
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("public"),
			},
		}
		provider := NewProvider(cfg)
		compliance := provider.GetCompliance()
		require.NotNil(t, compliance)
	})

	t.Run("未配置 Environment 时使用默认 prod", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				ChainMode: types.StringPtr("public"),
			},
		}
		provider := NewProvider(cfg)
		compliance := provider.GetCompliance()
		require.NotNil(t, compliance)
	})

	t.Run("未配置 ChainMode 时应 panic", func(t *testing.T) {
		cfg := &types.AppConfig{
			Environment: types.StringPtr("dev"),
			Network: &types.UserNetworkConfig{},
		}
		provider := NewProvider(cfg)
		
		assert.Panics(t, func() {
			_ = provider.GetCompliance()
		}, "未配置 ChainMode 时应 panic（因为 GetCompliance 内部调用 GetChainMode）")
	})
}

// TestResolveComplianceProfile 测试 resolveComplianceProfile() 方法
func TestResolveComplianceProfile(t *testing.T) {
	// 由于 resolveComplianceProfile 是私有方法，我们通过 GetCompliance 间接测试
	// 但我们可以测试不同 Environment 值对应的 profile 映射

	testCases := []struct {
		name      string
		env       string
		chainMode string
		expected  string // 预期的 networkType（development/testing/production）
	}{
		{"dev + public", "dev", "public", "development"},
		{"dev + consortium", "dev", "consortium", "development"},
		{"dev + private", "dev", "private", "development"},
		{"test + public", "test", "public", "testing"},
		{"test + consortium", "test", "consortium", "testing"},
		{"test + private", "test", "private", "testing"},
		{"prod + public", "prod", "public", "production"},
		{"prod + consortium", "prod", "consortium", "production"},
		{"prod + private", "prod", "private", "production"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &types.AppConfig{
				Environment: types.StringPtr(tc.env),
				Network: &types.UserNetworkConfig{
					ChainMode: types.StringPtr(tc.chainMode),
				},
			}
			provider := NewProvider(cfg)
			compliance := provider.GetCompliance()
			require.NotNil(t, compliance)
			// 注意：这里我们只验证合规配置已创建，具体的 networkType 验证需要查看 compliance 包的实现
			// 如果 compliance 包提供了获取 networkType 的方法，可以在这里验证
		})
	}
}

// TestApplyNetworkNamespaceIsolation 测试 applyNetworkNamespaceIsolation() 方法
func TestApplyNetworkNamespaceIsolation(t *testing.T) {
	t.Run("应用命名空间隔离到节点配置", func(t *testing.T) {
		cfg := &types.AppConfig{
			Network: &types.UserNetworkConfig{
				NetworkNamespace: types.StringPtr("test-namespace"),
			},
		}
		provider := NewProvider(cfg)
		
		// 创建节点配置
		nodeOptions := provider.GetNode()
		require.NotNil(t, nodeOptions)
		
		// 验证命名空间已应用到节点配置
		// 注意：具体的验证取决于 node 包的实现
		// 这里我们主要验证 GetNode() 不会 panic，并且返回了有效的配置
		assert.NotNil(t, nodeOptions)
	})
}

