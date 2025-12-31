package pubsub

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang/snappy"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"google.golang.org/protobuf/proto"
)

// encoding.go
// 🎯 破坏性重构：使用结构化 Topic 字段，移除字符串拼接

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

func (e *Encoder) encodeEnvelope(env *transportpb.Envelope) ([]byte, error) {
	data, err := proto.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("序列化Envelope失败: %w", err)
	}
	return data, nil
}

// EncodeTopic 编码主题消息（使用结构化 Topic）
//
// 🎯 破坏性变更：使用 protocols.Topic 替代 string topic
func (e *Encoder) EncodeTopic(t protocols.Topic, payload []byte) ([]byte, error) {
	env := &transportpb.Envelope{
		Version: 1,
		// Topic 字段保留为空（新接口以结构化字段为准）；旧接口 Encode 会显式写入原始 topic 字符串。
		Namespace:    t.Namespace,
		Domain:       t.Domain,
		Name:         t.Name,
		TopicVersion: t.Version,
		ContentType:  "application/octet-stream",
		Encoding:     "pb",
		Compression:  "none",
		Payload:      payload,
		Timestamp:    uint64(time.Now().UnixMilli()),
	}

	return e.encodeEnvelope(env)
}

// Encode 编码主题消息（兼容旧接口，内部转换为结构化 Topic）
//
// ⚠️ 废弃：此方法仅用于向后兼容，新代码应使用 EncodeTopic
func (e *Encoder) Encode(topic string, payload []byte) ([]byte, error) {
	// 兼容旧接口：保留原始 topic 字符串，且尽力填充结构化字段（如可解析）
	t := parseLegacyTopicString(topic)
	env := &transportpb.Envelope{
		Version:      1,
		Topic:        topic,
		Namespace:    t.Namespace,
		Domain:       t.Domain,
		Name:         t.Name,
		TopicVersion: t.Version,
		ContentType:  "application/octet-stream",
		Encoding:     "pb",
		Compression:  "none",
		Payload:      payload,
		Timestamp:    uint64(time.Now().UnixMilli()),
	}
	return e.encodeEnvelope(env)
}

// parseLegacyTopicString 解析旧格式的 topic 字符串为结构化 Topic
//
// 格式：weisyn.{namespace}.{domain}.{name}.{version} 或 weisyn.{domain}.{name}.{version}
func parseLegacyTopicString(topic string) protocols.Topic {
	parts := strings.Split(topic, ".")
	if len(parts) < 4 || parts[0] != "weisyn" {
		// 无法解析，返回空 Topic
		return protocols.Topic{}
	}

	// 判断是否有 namespace
	if len(parts) == 5 {
		// weisyn.{namespace}.{domain}.{name}.{version}
		return protocols.Topic{
			Namespace: parts[1],
			Domain:    parts[2],
			Name:      parts[3],
			Version:   parts[4],
		}
	} else if len(parts) == 4 {
		// weisyn.{domain}.{name}.{version}
		return protocols.Topic{
			Domain:  parts[1],
			Name:    parts[2],
			Version: parts[3],
		}
	}

	return protocols.Topic{}
}

// Decoder 解码器（仅支持protobuf Envelope）
type Decoder struct{}

// NewDecoder 创建解码器
func NewDecoder() *Decoder { return &Decoder{} }

func (d *Decoder) decodeEnvelope(data []byte) (*transportpb.Envelope, protocols.Topic, []byte, error) {
	var env transportpb.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		displayLen := 32
		if len(data) < 32 {
			displayLen = len(data)
		}
		if displayLen > 0 {
			return nil, protocols.Topic{}, nil, fmt.Errorf("invalid protobuf envelope format (size=%d, first_%d_bytes=%x): %w",
				len(data), displayLen, data[:displayLen], err)
		}
		return nil, protocols.Topic{}, nil, fmt.Errorf("invalid protobuf envelope format (size=%d): %w", len(data), err)
	}

	topic := protocols.Topic{
		Namespace: env.Namespace,
		Domain:    env.Domain,
		Name:      env.Name,
		Version:   env.TopicVersion,
	}
	if topic.Domain == "" && topic.Name == "" && env.Topic != "" {
		topic = parseLegacyTopicString(env.Topic)
	}

	// 预留解压路径
	if enc := env.GetCompression(); enc != "" && enc != "none" {
		// ✅ 生产级实现：解压 + 安全边界（防压缩炸弹）
		decompressed, err := decompressWithLimit(strings.ToLower(enc), env.Payload, maxDecompressedPayloadBytes)
		if err != nil {
			return nil, protocols.Topic{}, nil, fmt.Errorf("pubsub payload decompress failed: compression=%s payload_size=%d: %w",
				enc, len(env.Payload), err)
		}
		return &env, topic, decompressed, nil
	}

	return &env, topic, env.Payload, nil
}

const maxDecompressedPayloadBytes = 8 * 1024 * 1024 // 8MB：用于限制解压后大小，防止压缩炸弹

func decompressWithLimit(alg string, payload []byte, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid maxBytes=%d", maxBytes)
	}
	switch alg {
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		// 限制解压后字节数：最多读取 maxBytes+1，用于判断是否超限
		b, err := io.ReadAll(io.LimitReader(r, int64(maxBytes+1)))
		if err != nil {
			return nil, err
		}
		if len(b) > maxBytes {
			return nil, fmt.Errorf("decompressed payload too large: %d > %d", len(b), maxBytes)
		}
		return b, nil
	case "snappy":
		// snappy 提供解压后长度预测，可提前拒绝
		if n, err := snappy.DecodedLen(payload); err == nil {
			if n > maxBytes {
				return nil, fmt.Errorf("snappy decoded payload too large: %d > %d", n, maxBytes)
			}
		}
		b, err := snappy.Decode(nil, payload)
		if err != nil {
			return nil, err
		}
		if len(b) > maxBytes {
			return nil, fmt.Errorf("decompressed payload too large: %d > %d", len(b), maxBytes)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", alg)
	}
}

// DecodeTopic 解码主题消息（返回结构化 Topic）
//
// 🎯 破坏性变更：返回 protocols.Topic 而非仅校验字符串
func (d *Decoder) DecodeTopic(data []byte) (protocols.Topic, []byte, error) {
	_, topic, payload, err := d.decodeEnvelope(data)
	if err != nil {
		return protocols.Topic{}, nil, err
	}
	return topic, payload, nil
}

// Decode 解码主题消息（兼容旧接口）
//
// ⚠️ 废弃：此方法仅用于向后兼容，新代码应使用 DecodeTopic
func (d *Decoder) Decode(topic string, data []byte) ([]byte, error) {
	env, decodedTopic, payload, err := d.decodeEnvelope(data)
	if err != nil {
		return nil, err
	}

	// 校验 topic 一致性（如果提供了期望的 topic 字符串）
	if topic != "" {
		// 1) 优先按原始 topic 字符串严格匹配（旧接口/非 weisyn.* 格式）
		if env != nil && env.Topic != "" {
			if env.Topic != topic {
				return nil, fmt.Errorf("topic mismatch: decoded=%s, expect=%s", env.Topic, topic)
			}
		} else {
			// 2) 若没有原始 topic，则按结构化字段匹配（weisyn.* legacy 格式）
			expectedTopic := parseLegacyTopicString(topic)
			if decodedTopic.Domain != expectedTopic.Domain ||
				decodedTopic.Name != expectedTopic.Name ||
				decodedTopic.Version != expectedTopic.Version {
				return nil, fmt.Errorf("topic mismatch: decoded=%s, expect=%s", decodedTopic.String(), topic)
			}
		}
	}

	return payload, nil
}
