// Package pubsub 提供 PubSub 组件的测试
//
// 🧪 **测试文件**
//
// 本文件测试 TopicManager 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 主题管理器创建
// - 主题订阅
// - 主题退订
// - 订阅状态查询
// - 主题列表查询
// - 并发安全测试
package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 主题管理器创建测试 ====================

// TestNewTopicManager_ReturnsInitializedManager 测试创建主题管理器
func TestNewTopicManager_ReturnsInitializedManager(t *testing.T) {
	// Arrange & Act
	manager := NewTopicManager()

	// Assert
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.topics)
	assert.Equal(t, 0, len(manager.topics))
}

// ==================== 主题订阅测试 ====================

// TestTopicManager_Subscribe_WithNewTopic_AddsTopic 测试订阅新主题
func TestTopicManager_Subscribe_WithNewTopic_AddsTopic(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"

	// Act
	err := manager.Subscribe(topic)

	// Assert
	assert.NoError(t, err)
	assert.True(t, manager.IsSubscribed(topic))
	
	info, exists := manager.GetTopicInfo(topic)
	require.True(t, exists)
	assert.Equal(t, topic, info.Topic)
	assert.Equal(t, 1, info.HandlerCount)
	assert.WithinDuration(t, time.Now(), info.SubscribedAt, time.Second)
}

// TestTopicManager_Subscribe_WithExistingTopic_IncrementsHandlerCount 测试重复订阅同一主题
func TestTopicManager_Subscribe_WithExistingTopic_IncrementsHandlerCount(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	
	// 第一次订阅
	err1 := manager.Subscribe(topic)
	require.NoError(t, err1)

	// Act - 第二次订阅
	err2 := manager.Subscribe(topic)

	// Assert
	assert.NoError(t, err2)
	info, exists := manager.GetTopicInfo(topic)
	require.True(t, exists)
	assert.Equal(t, 2, info.HandlerCount, "HandlerCount 应该增加到 2")
}

// ==================== 主题退订测试 ====================

// TestTopicManager_Unsubscribe_WithExistingTopic_DecrementsHandlerCount 测试退订主题
func TestTopicManager_Unsubscribe_WithExistingTopic_DecrementsHandlerCount(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	
	// 订阅两次
	manager.Subscribe(topic)
	manager.Subscribe(topic)

	// Act - 退订一次
	err := manager.Unsubscribe(topic)

	// Assert
	assert.NoError(t, err)
	info, exists := manager.GetTopicInfo(topic)
	require.True(t, exists)
	assert.Equal(t, 1, info.HandlerCount, "HandlerCount 应该减少到 1")
	assert.True(t, manager.IsSubscribed(topic), "主题应该仍然存在")
}

// TestTopicManager_Unsubscribe_WithLastHandler_RemovesTopic 测试最后一个处理器退订时删除主题
func TestTopicManager_Unsubscribe_WithLastHandler_RemovesTopic(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	
	manager.Subscribe(topic)

	// Act - 退订
	err := manager.Unsubscribe(topic)

	// Assert
	assert.NoError(t, err)
	assert.False(t, manager.IsSubscribed(topic), "主题应该被删除")
	
	_, exists := manager.GetTopicInfo(topic)
	assert.False(t, exists, "主题信息应该不存在")
}

// TestTopicManager_Unsubscribe_WithNonExistentTopic_ReturnsNoError 测试退订不存在的主题
func TestTopicManager_Unsubscribe_WithNonExistentTopic_ReturnsNoError(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "nonexistent/topic"

	// Act
	err := manager.Unsubscribe(topic)

	// Assert
	assert.NoError(t, err, "退订不存在的主题不应该返回错误")
	assert.False(t, manager.IsSubscribed(topic))
}

// ==================== 订阅状态查询测试 ====================

// TestTopicManager_IsSubscribed_WithSubscribedTopic_ReturnsTrue 测试查询已订阅的主题
func TestTopicManager_IsSubscribed_WithSubscribedTopic_ReturnsTrue(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	manager.Subscribe(topic)

	// Act
	isSubscribed := manager.IsSubscribed(topic)

	// Assert
	assert.True(t, isSubscribed)
}

// TestTopicManager_IsSubscribed_WithNonSubscribedTopic_ReturnsFalse 测试查询未订阅的主题
func TestTopicManager_IsSubscribed_WithNonSubscribedTopic_ReturnsFalse(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "nonexistent/topic"

	// Act
	isSubscribed := manager.IsSubscribed(topic)

	// Assert
	assert.False(t, isSubscribed)
}

// ==================== 主题列表查询测试 ====================

// TestTopicManager_ListTopics_WithMultipleTopics_ReturnsAllTopics 测试列出所有主题
func TestTopicManager_ListTopics_WithMultipleTopics_ReturnsAllTopics(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topics := []string{"topic1", "topic2", "topic3"}
	
	for _, topic := range topics {
		manager.Subscribe(topic)
	}

	// Act
	list := manager.ListTopics()

	// Assert
	assert.Equal(t, len(topics), len(list))
	
	// 验证所有主题都在列表中
	topicMap := make(map[string]bool)
	for _, info := range list {
		topicMap[info.Topic] = true
	}
	for _, topic := range topics {
		assert.True(t, topicMap[topic], "主题 %s 应该在列表中", topic)
	}
}

// TestTopicManager_ListTopics_WithEmptyManager_ReturnsEmptyList 测试空管理器返回空列表
func TestTopicManager_ListTopics_WithEmptyManager_ReturnsEmptyList(t *testing.T) {
	// Arrange
	manager := NewTopicManager()

	// Act
	list := manager.ListTopics()

	// Assert
	assert.NotNil(t, list)
	assert.Equal(t, 0, len(list))
}

// ==================== 主题信息查询测试 ====================

// TestTopicManager_GetTopicInfo_WithExistingTopic_ReturnsInfo 测试获取存在的主题信息
func TestTopicManager_GetTopicInfo_WithExistingTopic_ReturnsInfo(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	manager.Subscribe(topic)

	// Act
	info, exists := manager.GetTopicInfo(topic)

	// Assert
	assert.True(t, exists)
	assert.NotNil(t, info)
	assert.Equal(t, topic, info.Topic)
	assert.Equal(t, 1, info.HandlerCount)
}

// TestTopicManager_GetTopicInfo_WithNonExistentTopic_ReturnsFalse 测试获取不存在的主题信息
func TestTopicManager_GetTopicInfo_WithNonExistentTopic_ReturnsFalse(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "nonexistent/topic"

	// Act
	info, exists := manager.GetTopicInfo(topic)

	// Assert
	assert.False(t, exists)
	assert.Nil(t, info)
}

// ==================== 并发安全测试 ====================

// TestTopicManager_ConcurrentSubscribe_IsThreadSafe 测试并发订阅的线程安全性
func TestTopicManager_ConcurrentSubscribe_IsThreadSafe(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	goroutines := 10
	done := make(chan bool, goroutines)

	// Act - 并发订阅
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- true }()
			err := manager.Subscribe(topic)
			assert.NoError(t, err)
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert
	info, exists := manager.GetTopicInfo(topic)
	require.True(t, exists)
	assert.Equal(t, goroutines, info.HandlerCount, "HandlerCount 应该等于并发订阅次数")
}

// TestTopicManager_ConcurrentUnsubscribe_IsThreadSafe 测试并发退订的线程安全性
func TestTopicManager_ConcurrentUnsubscribe_IsThreadSafe(t *testing.T) {
	// Arrange
	manager := NewTopicManager()
	topic := "test/topic/v1"
	goroutines := 10
	
	// 先订阅多次
	for i := 0; i < goroutines; i++ {
		manager.Subscribe(topic)
	}

	done := make(chan bool, goroutines)

	// Act - 并发退订
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- true }()
			err := manager.Unsubscribe(topic)
			assert.NoError(t, err)
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert
	assert.False(t, manager.IsSubscribed(topic), "所有处理器退订后，主题应该被删除")
}

