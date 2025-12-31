package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_CheckConnection(t *testing.T) {
	rl := NewRateLimiter(10, 3)

	// 测试正常连接
	err := rl.CheckConnection("peer1", "192.168.1.1")
	assert.NoError(t, err)

	// 测试同一IP多次连接
	err = rl.CheckConnection("peer2", "192.168.1.1")
	assert.NoError(t, err)
	err = rl.CheckConnection("peer3", "192.168.1.1")
	assert.NoError(t, err)

	// 测试单IP连接数限制
	err = rl.CheckConnection("peer4", "192.168.1.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IP连接数已达上限")
}

func TestRateLimiter_RemoveConnection(t *testing.T) {
	rl := NewRateLimiter(10, 3)

	// 添加连接
	rl.CheckConnection("peer1", "192.168.1.1")
	rl.CheckConnection("peer2", "192.168.1.1")

	// 移除连接
	rl.RemoveConnection("peer1", "192.168.1.1")

	// 验证可以再次添加连接
	err := rl.CheckConnection("peer3", "192.168.1.1")
	assert.NoError(t, err)
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(10, 3)

	// 停止速率限制器
	rl.Stop()

	// 等待清理协程停止
	time.Sleep(100 * time.Millisecond)
}
