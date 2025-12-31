// Package stream 提供流式传输服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 Service 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 信号量获取
// - 并发限制设置
package stream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	libconnmgr "github.com/libp2p/go-libp2p/core/connmgr"
	libeventbus "github.com/libp2p/go-libp2p/core/event"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	libpeerstore "github.com/libp2p/go-libp2p/core/peerstore"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

// ==================== 服务创建测试 ====================

// mockHost 简单的 Mock Host 实现（避免循环导入）
// 注意：Service 实际上不使用 host 的方法，所以这里提供一个最小实现即可
type mockHost struct{}

// 实现 libhost.Host 的最小接口（实际上 Service 不使用这些方法）
func (m *mockHost) ID() libpeer.ID { return libpeer.ID("") }
func (m *mockHost) Peerstore() libpeerstore.Peerstore { return nil }
func (m *mockHost) Addrs() []ma.Multiaddr { return nil }
func (m *mockHost) Network() libnetwork.Network { return nil }
func (m *mockHost) Mux() libprotocol.Switch { return nil }
func (m *mockHost) Connect(ctx context.Context, pi libpeer.AddrInfo) error { return nil }
func (m *mockHost) SetStreamHandler(pid libprotocol.ID, handler libnetwork.StreamHandler) {}
func (m *mockHost) SetStreamHandlerMatch(pid libprotocol.ID, matcher func(libprotocol.ID) bool, handler libnetwork.StreamHandler) {}
func (m *mockHost) RemoveStreamHandler(pid libprotocol.ID) {}
func (m *mockHost) NewStream(ctx context.Context, p libpeer.ID, pids ...libprotocol.ID) (libnetwork.Stream, error) { return nil, nil }
func (m *mockHost) Close() error { return nil }
func (m *mockHost) ConnManager() libconnmgr.ConnManager { return nil }
func (m *mockHost) EventBus() libeventbus.Bus { return nil }

// TestNew_WithValidHost_ReturnsService 测试使用有效 Host 创建服务
func TestNew_WithValidHost_ReturnsService(t *testing.T) {
	// Arrange
	host := &mockHost{}

	// Act
	service := New(host)

	// Assert
	assert.NotNil(t, service)
	assert.NotNil(t, service.sem)
	assert.Equal(t, 100, service.sem.Capacity(), "默认并发数应该是 100")
}

// ==================== 信号量获取测试 ====================

// TestService_GetSemaphore_ReturnsSemaphore 测试获取信号量
func TestService_GetSemaphore_ReturnsSemaphore(t *testing.T) {
	// Arrange
	host := &mockHost{}
	service := New(host)

	// Act
	sem := service.GetSemaphore()

	// Assert
	assert.NotNil(t, sem)
	assert.Equal(t, service.sem, sem)
}

// ==================== 并发限制设置测试 ====================

// TestService_SetConcurrencyLimit_UpdatesSemaphore 测试设置并发限制
func TestService_SetConcurrencyLimit_UpdatesSemaphore(t *testing.T) {
	// Arrange
	host := &mockHost{}
	service := New(host)
	newLimit := 50

	// Act
	service.SetConcurrencyLimit(newLimit)

	// Assert
	assert.Equal(t, newLimit, service.sem.Capacity(), "并发限制应该被更新")
}

// TestService_SetConcurrencyLimit_WithZeroLimit_UsesDefault 测试零限制时使用默认值
func TestService_SetConcurrencyLimit_WithZeroLimit_UsesDefault(t *testing.T) {
	// Arrange
	host := &mockHost{}
	service := New(host)

	// Act
	service.SetConcurrencyLimit(0)

	// Assert
	assert.Equal(t, 1, service.sem.Capacity(), "零限制应该使用默认容量 1")
}

