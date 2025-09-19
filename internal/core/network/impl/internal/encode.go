package internal

// encode.go
// 能力：统一二进制编解码（长度前缀帧、变长整型、TLV等）与 PB/JSON 适配。
// 依赖/公共接口：
// - pb/network/*：消息结构（由脚本生成 .pb.go，不手改）
// - 项目已有 JSON/CBOR 编解码工具（如有）
// 目的：
// - 为 stream/pubsub 提供一致的帧与载荷编解码能力
// 非目标：
// - 不在此实现业务消息结构；只做通用编解码与胶水层

// BinaryEncoder 二进制编码器（方法框架）
type BinaryEncoder struct{}

// NewBinaryEncoder 创建编码器
func NewBinaryEncoder() *BinaryEncoder { return &BinaryEncoder{} }

// Encode 编码任意对象为字节（方法框架）
func (e *BinaryEncoder) Encode(v interface{}) ([]byte, error) { return nil, nil }

// BinaryDecoder 二进制解码器（方法框架）
type BinaryDecoder struct{}

// NewBinaryDecoder 创建解码器
func NewBinaryDecoder() *BinaryDecoder { return &BinaryDecoder{} }

// Decode 解码字节为对象（方法框架）
func (d *BinaryDecoder) Decode(data []byte, v interface{}) error { return nil }
