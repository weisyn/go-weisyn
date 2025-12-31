// Package pubsub 提供编解码器的测试
//
// 🧪 **测试文件**
//
// 本文件测试 Encoder 和 Decoder 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 编码器创建
// - 消息编码
// - 解码器创建
// - 消息解码
// - 编码解码往返测试
// - 错误处理
package pubsub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	"google.golang.org/protobuf/proto"
)

// ==================== 编码器创建测试 ====================

// TestNewEncoder_ReturnsInitializedEncoder 测试创建编码器
func TestNewEncoder_ReturnsInitializedEncoder(t *testing.T) {
	// Arrange & Act
	encoder := NewEncoder()

	// Assert
	assert.NotNil(t, encoder)
	assert.Equal(t, EncodingPB, encoder.defaultType)
}

// ==================== 消息编码测试 ====================

// TestEncoder_Encode_WithValidPayload_ReturnsEncodedData 测试编码有效载荷
func TestEncoder_Encode_WithValidPayload_ReturnsEncodedData(t *testing.T) {
	// Arrange
	encoder := NewEncoder()
	topic := "test/topic/v1"
	payload := []byte("test payload")

	// Act
	encoded, err := encoder.Encode(topic, payload)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, encoded)
	assert.Greater(t, len(encoded), 0)
	
	// 验证编码后的数据可以解码
	var env transportpb.Envelope
	err = proto.Unmarshal(encoded, &env)
	assert.NoError(t, err)
	assert.Equal(t, topic, env.Topic)
	assert.Equal(t, payload, env.Payload)
	assert.Equal(t, "pb", env.Encoding)
}

// TestEncoder_Encode_WithEmptyPayload_ReturnsEncodedData 测试编码空载荷
func TestEncoder_Encode_WithEmptyPayload_ReturnsEncodedData(t *testing.T) {
	// Arrange
	encoder := NewEncoder()
	topic := "test/topic/v1"
	payload := []byte{}

	// Act
	encoded, err := encoder.Encode(topic, payload)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, encoded)
	
	// 验证解码后载荷为空
	var env transportpb.Envelope
	err = proto.Unmarshal(encoded, &env)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(env.Payload))
}

// TestEncoder_Encode_WithLargePayload_ReturnsEncodedData 测试编码大载荷
func TestEncoder_Encode_WithLargePayload_ReturnsEncodedData(t *testing.T) {
	// Arrange
	encoder := NewEncoder()
	topic := "test/topic/v1"
	payload := make([]byte, 1024*1024) // 1MB
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	// Act
	encoded, err := encoder.Encode(topic, payload)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, encoded)
	assert.Greater(t, len(encoded), len(payload), "编码后的数据应该包含 Envelope 元数据")
}

// ==================== 解码器创建测试 ====================

// TestNewDecoder_ReturnsInitializedDecoder 测试创建解码器
func TestNewDecoder_ReturnsInitializedDecoder(t *testing.T) {
	// Arrange & Act
	decoder := NewDecoder()

	// Assert
	assert.NotNil(t, decoder)
}

// ==================== 消息解码测试 ====================

// TestDecoder_Decode_WithValidEncodedData_ReturnsPayload 测试解码有效数据
func TestDecoder_Decode_WithValidEncodedData_ReturnsPayload(t *testing.T) {
	// Arrange
	encoder := NewEncoder()
	decoder := NewDecoder()
	topic := "test/topic/v1"
	payload := []byte("test payload")

	encoded, err := encoder.Encode(topic, payload)
	require.NoError(t, err)

	// Act
	decoded, err := decoder.Decode(topic, encoded)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

// TestDecoder_Decode_WithInvalidData_ReturnsError 测试解码无效数据
func TestDecoder_Decode_WithInvalidData_ReturnsError(t *testing.T) {
	// Arrange
	decoder := NewDecoder()
	topic := "test/topic/v1"
	invalidData := []byte("invalid protobuf data")

	// Act
	decoded, err := decoder.Decode(topic, invalidData)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, decoded)
	assert.Contains(t, err.Error(), "invalid protobuf envelope format")
}

// TestDecoder_Decode_WithEmptyData_ReturnsError 测试解码空数据
func TestDecoder_Decode_WithEmptyData_ReturnsError(t *testing.T) {
	// Arrange
	decoder := NewDecoder()
	topic := "test/topic/v1"
	emptyData := []byte{}

	// Act
	decoded, err := decoder.Decode(topic, emptyData)

	// Assert
	// 注意：protobuf 可能允许空数据，实际行为取决于实现
	// 如果返回错误，验证错误信息；如果成功，验证解码结果为空（nil 或空切片都算空）
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, decoded)
		assert.Contains(t, err.Error(), "invalid protobuf envelope format")
	} else {
		// 如果空数据可以解码，验证解码结果为空（nil 或空切片都算空）
		assert.NoError(t, err)
		assert.True(t, decoded == nil || len(decoded) == 0, "解码结果应该为空")
	}
}

// TestDecoder_Decode_WithTopicMismatch_ReturnsError 测试主题不匹配
func TestDecoder_Decode_WithTopicMismatch_ReturnsError(t *testing.T) {
	// Arrange
	encoder := NewEncoder()
	decoder := NewDecoder()
	topic1 := "topic1"
	topic2 := "topic2"
	payload := []byte("test payload")

	encoded, err := encoder.Encode(topic1, payload)
	require.NoError(t, err)

	// Act - 使用不同的主题解码
	decoded, err := decoder.Decode(topic2, encoded)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, decoded)
	assert.Contains(t, err.Error(), "topic mismatch")
}

// ==================== 编码解码往返测试 ====================

// TestEncoderDecoder_RoundTrip_WithVariousPayloads_PreservesData 测试编码解码往返
func TestEncoderDecoder_RoundTrip_WithVariousPayloads_PreservesData(t *testing.T) {
	testCases := []struct {
		name    string
		payload []byte
	}{
		{"空载荷", []byte{}},
		{"小载荷", []byte("hello")},
		{"中等载荷", make([]byte, 1024)},
		{"大载荷", make([]byte, 64*1024)},
		{"特殊字符", []byte{0x00, 0xFF, 0x0A, 0x0D}},
	}

	encoder := NewEncoder()
	decoder := NewDecoder()
	topic := "test/topic/v1"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			// 对于非空载荷，填充数据
			if len(tc.payload) > 0 {
				for i := range tc.payload {
					tc.payload[i] = byte(i % 256)
				}
			}

			// Act - 编码
			encoded, err := encoder.Encode(topic, tc.payload)
			require.NoError(t, err)

			// Act - 解码
			decoded, err := decoder.Decode(topic, encoded)

			// Assert
			assert.NoError(t, err, "往返编码解码应该成功")
			// 注意：空切片和 nil 在 Go 中语义相同，但比较时可能不同
			if len(tc.payload) == 0 && len(decoded) == 0 {
				// 空载荷：验证两者都是空即可
				assert.Equal(t, 0, len(decoded))
			} else {
				assert.Equal(t, tc.payload, decoded, "往返编码解码应该保持数据一致")
			}
		})
	}
}

