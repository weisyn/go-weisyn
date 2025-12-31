// Package pubsub 提供发布器的测试
//
// 🧪 **测试文件**
//
// 本文件测试 Publisher 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 发布器创建
// - 消息发布
// - 带上下文的发布
// - 发布统计
// - 并发安全测试
package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 发布器创建测试 ====================

// TestNewPublisher_ReturnsInitializedPublisher 测试创建发布器
func TestNewPublisher_ReturnsInitializedPublisher(t *testing.T) {
	// Arrange & Act
	publisher := NewPublisher()

	// Assert
	assert.NotNil(t, publisher)
	assert.NotNil(t, publisher.stats)
	assert.Equal(t, 0, len(publisher.stats))
}

// ==================== 消息发布测试 ====================

// TestPublisher_Publish_WithValidData_ReturnsNoError 测试发布有效消息
func TestPublisher_Publish_WithValidData_ReturnsNoError(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic := "test/topic/v1"
	payload := []byte("test payload")

	// Act
	err := publisher.Publish(topic, payload)

	// Assert
	assert.NoError(t, err)
	
	// 验证统计已更新
	stats := publisher.GetStats(topic)
	require.NotNil(t, stats)
	assert.Equal(t, uint64(1), stats.TotalMessages)
	assert.Equal(t, uint64(0), stats.FailedMessages)
}

// TestPublisher_Publish_WithMultipleMessages_UpdatesStats 测试发布多条消息
func TestPublisher_Publish_WithMultipleMessages_UpdatesStats(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic := "test/topic/v1"
	count := 10

	// Act
	for i := 0; i < count; i++ {
		payload := []byte{byte(i)}
		err := publisher.Publish(topic, payload)
		assert.NoError(t, err)
	}

	// Assert
	stats := publisher.GetStats(topic)
	require.NotNil(t, stats)
	assert.Equal(t, uint64(count), stats.TotalMessages)
	assert.WithinDuration(t, time.Now(), stats.LastPublishAt, time.Second)
}

// TestPublisher_Publish_WithDifferentTopics_TracksSeparateStats 测试不同主题的独立统计
func TestPublisher_Publish_WithDifferentTopics_TracksSeparateStats(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic1 := "topic1"
	topic2 := "topic2"

	// Act
	publisher.Publish(topic1, []byte("payload1"))
	publisher.Publish(topic1, []byte("payload2"))
	publisher.Publish(topic2, []byte("payload3"))

	// Assert
	stats1 := publisher.GetStats(topic1)
	require.NotNil(t, stats1)
	assert.Equal(t, uint64(2), stats1.TotalMessages)

	stats2 := publisher.GetStats(topic2)
	require.NotNil(t, stats2)
	assert.Equal(t, uint64(1), stats2.TotalMessages)
}

// ==================== 带上下文的发布测试 ====================

// TestPublisher_PublishWithContext_WithValidContext_ReturnsNoError 测试带有效上下文的发布
func TestPublisher_PublishWithContext_WithValidContext_ReturnsNoError(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	ctx := context.Background()
	topic := "test/topic/v1"
	payload := []byte("test payload")

	// Act
	err := publisher.PublishWithContext(ctx, topic, payload)

	// Assert
	assert.NoError(t, err)
}

// TestPublisher_PublishWithContext_WithCancelledContext_ReturnsError 测试带已取消上下文的发布
func TestPublisher_PublishWithContext_WithCancelledContext_ReturnsError(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	topic := "test/topic/v1"
	payload := []byte("test payload")

	// Act
	err := publisher.PublishWithContext(ctx, topic, payload)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	
	// 验证失败统计已更新
	stats := publisher.GetStats(topic)
	require.NotNil(t, stats)
	assert.Equal(t, uint64(1), stats.TotalMessages)
	assert.Equal(t, uint64(1), stats.FailedMessages)
}

// TestPublisher_PublishWithContext_WithTimeoutContext_ReturnsError 测试带超时上下文的发布
func TestPublisher_PublishWithContext_WithTimeoutContext_ReturnsError(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // 确保超时
	topic := "test/topic/v1"
	payload := []byte("test payload")

	// Act
	err := publisher.PublishWithContext(ctx, topic, payload)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// ==================== 发布统计测试 ====================

// TestPublisher_GetStats_WithNonExistentTopic_ReturnsNil 测试获取不存在主题的统计
func TestPublisher_GetStats_WithNonExistentTopic_ReturnsNil(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic := "nonexistent/topic"

	// Act
	stats := publisher.GetStats(topic)

	// Assert
	assert.Nil(t, stats)
}

// TestPublisher_GetStats_ReturnsCopy 测试 GetStats 返回副本
func TestPublisher_GetStats_ReturnsCopy(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic := "test/topic/v1"
	publisher.Publish(topic, []byte("payload"))

	// Act
	stats1 := publisher.GetStats(topic)
	require.NotNil(t, stats1)
	
	// 修改返回的统计（不应该影响内部状态）
	originalTotal := stats1.TotalMessages
	stats1.TotalMessages = 999

	// 再次获取统计
	stats2 := publisher.GetStats(topic)

	// Assert
	assert.NotNil(t, stats2)
	assert.Equal(t, originalTotal, stats2.TotalMessages, "修改返回的统计不应该影响内部状态")
	assert.NotEqual(t, uint64(999), stats2.TotalMessages)
}

// ==================== 并发安全测试 ====================

// TestPublisher_ConcurrentPublish_IsThreadSafe 测试并发发布的线程安全性
func TestPublisher_ConcurrentPublish_IsThreadSafe(t *testing.T) {
	// Arrange
	publisher := NewPublisher()
	topic := "test/topic/v1"
	goroutines := 10
	iterations := 10
	done := make(chan bool, goroutines)

	// Act - 并发发布
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < iterations; j++ {
				payload := []byte{byte(id), byte(j)}
				err := publisher.Publish(topic, payload)
				assert.NoError(t, err)
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert
	stats := publisher.GetStats(topic)
	require.NotNil(t, stats)
	assert.Equal(t, uint64(goroutines*iterations), stats.TotalMessages, "总消息数应该等于并发发布次数")
}

