package pubsub

import (
	"fmt"
	"time"

	transportpb "github.com/weisyn/v1/pb/network/transport"
	"google.golang.org/protobuf/proto"
)

// encoding.go
// 🔧 修复：网络消息编解码 - 仅使用 Protobuf，移除JSON支持

// EncodingType 编码类型（仅保留protobuf）
type EncodingType int

const (
	EncodingPB EncodingType = iota // Protocol Buffers（唯一支持）
)

// Encoder 编码器（仅支持protobuf）
type Encoder struct {
	defaultType EncodingType
}

// NewEncoder 创建编码器
func NewEncoder() *Encoder {
	return &Encoder{defaultType: EncodingPB}
}

// Encode 编码主题消息（Envelope 包装）
func (e *Encoder) Encode(topic string, payload []byte) ([]byte, error) {
	// 🔧 修复：仅支持protobuf编码，移除所有回退机制
	env := &transportpb.Envelope{
		Version:     1,
		Topic:       topic,
		ContentType: "application/octet-stream",
		Encoding:    "pb", // 始终使用protobuf
		Compression: "none",
		Payload:     payload,
		Timestamp:   uint64(time.Now().UnixMilli()),
	}

	return proto.Marshal(env)
}

// Decoder 解码器（仅支持protobuf Envelope）
type Decoder struct{}

// NewDecoder 创建解码器
func NewDecoder() *Decoder { return &Decoder{} }

// Decode 解码主题消息
func (d *Decoder) Decode(topic string, data []byte) ([]byte, error) {
	var env transportpb.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		// 🔧 修复：移除JSON回退，直接返回解码失败
		// 🔍 添加详细的解码错误调试信息
		displayLen := 32
		if len(data) < 32 {
			displayLen = len(data)
		}
		if displayLen > 0 {
			return nil, fmt.Errorf("invalid protobuf envelope format (topic=%s, size=%d, first_%d_bytes=%x): %w",
				topic, len(data), displayLen, data[:displayLen], err)
		}
		return nil, fmt.Errorf("invalid protobuf envelope format (topic=%s, size=%d): %w", topic, len(data), err)
	}

	// 可选校验：topic 一致性
	if env.Topic != "" && env.Topic != topic {
		return nil, fmt.Errorf("topic mismatch: env=%s, expect=%s", env.Topic, topic)
	}

	// 预留解压路径
	if enc := env.GetCompression(); enc != "" && enc != "none" {
		// TODO: 调用 compressor 解压（当前直通）
		return env.Payload, nil
	}

	return env.Payload, nil
}
