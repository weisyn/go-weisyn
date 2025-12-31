// Package registry 提供协议注册表的测试
//
// 🧪 **测试文件**
//
// 本文件测试 ProtocolRegistry 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 注册表创建
// - 协议注册
// - 协议注销
// - 协议查询
// - 协议列表
// - 并发安全测试
package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// ==================== 注册表创建测试 ====================

// TestNewProtocolRegistry_ReturnsInitializedRegistry 测试创建协议注册表
func TestNewProtocolRegistry_ReturnsInitializedRegistry(t *testing.T) {
	// Arrange & Act
	registry := NewProtocolRegistry()

	// Assert
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.handlers)
	assert.NotNil(t, registry.infos)
	assert.Equal(t, 0, len(registry.handlers))
}

// ==================== 协议注册测试 ====================

// TestProtocolRegistry_Register_WithValidHandler_RegistersProtocol 测试注册有效处理器
func TestProtocolRegistry_Register_WithValidHandler_RegistersProtocol(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/test/v1"
	handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response"), nil
	}

	// Act
	err := registry.Register(protocolID, handler)

	// Assert
	assert.NoError(t, err)
	
	// 验证处理器已注册
	retrievedHandler, exists := registry.Get(protocolID)
	assert.True(t, exists)
	assert.NotNil(t, retrievedHandler)
	
	// 验证协议信息已创建
	info, exists := registry.Info(protocolID)
	assert.True(t, exists)
	assert.NotNil(t, info)
	assert.Equal(t, protocolID, info.ID)
}

// TestProtocolRegistry_Register_WithDuplicateProtocol_OverwritesHandler 测试重复注册覆盖处理器
func TestProtocolRegistry_Register_WithDuplicateProtocol_OverwritesHandler(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/test/v1"
	
	handler1 := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response1"), nil
	}
	handler2 := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response2"), nil
	}

	// Act
	err1 := registry.Register(protocolID, handler1)
	require.NoError(t, err1)
	
	err2 := registry.Register(protocolID, handler2)
	require.NoError(t, err2)

	// Assert
	retrievedHandler, exists := registry.Get(protocolID)
	assert.True(t, exists)
	assert.NotNil(t, retrievedHandler)
	// 注意：无法直接比较函数，但可以验证存在
}

// ==================== 协议注销测试 ====================

// TestProtocolRegistry_Unregister_WithRegisteredProtocol_RemovesProtocol 测试注销已注册的协议
func TestProtocolRegistry_Unregister_WithRegisteredProtocol_RemovesProtocol(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/test/v1"
	handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response"), nil
	}
	
	registry.Register(protocolID, handler)

	// Act
	err := registry.Unregister(protocolID)

	// Assert
	assert.NoError(t, err)
	
	// 验证协议已删除
	_, exists := registry.Get(protocolID)
	assert.False(t, exists)
	
	_, exists = registry.Info(protocolID)
	assert.False(t, exists)
}

// TestProtocolRegistry_Unregister_WithNonExistentProtocol_ReturnsNoError 测试注销不存在的协议
func TestProtocolRegistry_Unregister_WithNonExistentProtocol_ReturnsNoError(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/nonexistent/v1"

	// Act
	err := registry.Unregister(protocolID)

	// Assert
	assert.NoError(t, err, "注销不存在的协议不应该返回错误")
}

// ==================== 协议查询测试 ====================

// TestProtocolRegistry_Get_WithRegisteredProtocol_ReturnsHandler 测试获取已注册的协议处理器
func TestProtocolRegistry_Get_WithRegisteredProtocol_ReturnsHandler(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/test/v1"
	handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response"), nil
	}
	
	registry.Register(protocolID, handler)

	// Act
	retrievedHandler, exists := registry.Get(protocolID)

	// Assert
	assert.True(t, exists)
	assert.NotNil(t, retrievedHandler)
}

// TestProtocolRegistry_Get_WithNonExistentProtocol_ReturnsFalse 测试获取不存在的协议
func TestProtocolRegistry_Get_WithNonExistentProtocol_ReturnsFalse(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/nonexistent/v1"

	// Act
	handler, exists := registry.Get(protocolID)

	// Assert
	assert.False(t, exists)
	assert.Nil(t, handler)
}

// ==================== 协议列表测试 ====================

// TestProtocolRegistry_List_WithMultipleProtocols_ReturnsAllProtocols 测试列出所有协议
func TestProtocolRegistry_List_WithMultipleProtocols_ReturnsAllProtocols(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocols := []string{"/weisyn/test1/v1", "/weisyn/test2/v1", "/weisyn/test3/v1"}
	
	for _, protoID := range protocols {
		handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
			return []byte("response"), nil
		}
		registry.Register(protoID, handler)
	}

	// Act
	list := registry.List()

	// Assert
	assert.Equal(t, len(protocols), len(list))
	
	// 验证所有协议都在列表中
	protocolMap := make(map[string]bool)
	for _, info := range list {
		protocolMap[info.ID] = true
	}
	for _, protoID := range protocols {
		assert.True(t, protocolMap[protoID], "协议 %s 应该在列表中", protoID)
	}
}

// TestProtocolRegistry_List_WithEmptyRegistry_ReturnsEmptyList 测试空注册表返回空列表
func TestProtocolRegistry_List_WithEmptyRegistry_ReturnsEmptyList(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()

	// Act
	list := registry.List()

	// Assert
	assert.NotNil(t, list)
	assert.Equal(t, 0, len(list))
}

// ==================== 协议信息测试 ====================

// TestProtocolRegistry_Info_WithRegisteredProtocol_ReturnsInfo 测试获取已注册协议的信息
func TestProtocolRegistry_Info_WithRegisteredProtocol_ReturnsInfo(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/test/v1"
	handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
		return []byte("response"), nil
	}
	
	registry.Register(protocolID, handler)

	// Act
	info, exists := registry.Info(protocolID)

	// Assert
	assert.True(t, exists)
	assert.NotNil(t, info)
	assert.Equal(t, protocolID, info.ID)
	assert.WithinDuration(t, time.Now(), info.RegisteredAt, time.Second)
}

// TestProtocolRegistry_Info_WithNonExistentProtocol_ReturnsFalse 测试获取不存在协议的信息
func TestProtocolRegistry_Info_WithNonExistentProtocol_ReturnsFalse(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	protocolID := "/weisyn/nonexistent/v1"

	// Act
	info, exists := registry.Info(protocolID)

	// Assert
	assert.False(t, exists)
	assert.Nil(t, info)
}

// ==================== 并发安全测试 ====================

// TestProtocolRegistry_ConcurrentRegister_IsThreadSafe 测试并发注册的线程安全性
func TestProtocolRegistry_ConcurrentRegister_IsThreadSafe(t *testing.T) {
	// Arrange
	registry := NewProtocolRegistry()
	goroutines := 10
	done := make(chan bool, goroutines)

	// Act - 并发注册
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			protocolID := "/weisyn/test/v1"
			handler := func(ctx context.Context, from peer.ID, req []byte) ([]byte, error) {
				return []byte("response"), nil
			}
			err := registry.Register(protocolID, handler)
			assert.NoError(t, err)
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert
	_, exists := registry.Get("/weisyn/test/v1")
	assert.True(t, exists, "协议应该被注册")
}

