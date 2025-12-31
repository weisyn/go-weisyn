package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageRateLimiter_CheckMessage(t *testing.T) {
	mrl := NewMessageRateLimiter(10, 1*time.Second)

	// 测试正常消息
	for i := 0; i < 10; i++ {
		err := mrl.CheckMessage("peer1")
		assert.NoError(t, err)
		assert.Equal(t, i+1, mrl.GetMessageCount("peer1"))
	}

	// 测试消息速率限制
	err := mrl.CheckMessage("peer1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "消息速率超限")

	// 测试不同节点的消息
	err = mrl.CheckMessage("peer2")
	assert.NoError(t, err)
	assert.Equal(t, 1, mrl.GetMessageCount("peer2"))
}

func TestMessageRateLimiter_Reset(t *testing.T) {
	mrl := NewMessageRateLimiter(10, 1*time.Second)

	// 添加消息
	for i := 0; i < 5; i++ {
		mrl.CheckMessage("peer1")
	}
	assert.Equal(t, 5, mrl.GetMessageCount("peer1"))

	// 重置
	mrl.Reset("peer1")
	assert.Equal(t, 0, mrl.GetMessageCount("peer1"))

	// 重置后可以继续发送消息
	err := mrl.CheckMessage("peer1")
	assert.NoError(t, err)
}

func TestMessageRateLimiter_ResetAll(t *testing.T) {
	mrl := NewMessageRateLimiter(10, 1*time.Second)

	// 添加消息
	mrl.CheckMessage("peer1")
	mrl.CheckMessage("peer2")
	mrl.CheckMessage("peer3")

	// 重置所有
	mrl.ResetAll()

	// 验证所有消息计数被清空
	assert.Equal(t, 0, mrl.GetMessageCount("peer1"))
	assert.Equal(t, 0, mrl.GetMessageCount("peer2"))
	assert.Equal(t, 0, mrl.GetMessageCount("peer3"))
}

func TestMessageRateLimiter_Stop(t *testing.T) {
	mrl := NewMessageRateLimiter(10, 1*time.Second)

	// 停止速率限制器
	mrl.Stop()

	// 等待清理协程停止
	time.Sleep(100 * time.Millisecond)
}
