// Package stream provides frame encoding and decoding for network streams.
package stream

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 帧常量与安全限制
const (
	FrameMagic     uint16 = 0xBF50 // 魔数：WES的16进制表示
	FrameVersion   uint8  = 1
	MaxFrameSize   uint32 = 16 * 1024 * 1024 // 16MB 最大帧限制
	MaxPayloadSize uint32 = MaxFrameSize - 8 // 减去头部长度
)

// 帧类型定义（用于区分 request/response/heartbeat/chunk 等帧语义）
const (
	FrameTypeRequest   uint8 = 0x01
	FrameTypeResponse  uint8 = 0x02
	FrameTypeHeartbeat uint8 = 0x03
	FrameTypeChunk     uint8 = 0x04
)

// EncodeFrame 以固定头部编码一帧：magic(2)|version(1)|type(1)|len(4)|payload
func EncodeFrame(w io.Writer, frameType uint8, payload []byte) error {
	// 写入前进行长度校验，避免发送超大帧
	//nolint:gosec // G115: len() 返回值已通过 MaxPayloadSize 限制检查，不会溢出
	if uint32(len(payload)) > MaxPayloadSize {
		return &CodecError{Type: ErrTypeOversize, Msg: "payload too large", Retryable: false}
	}
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint16(hdr[0:2], FrameMagic)
	//nolint:gosec // G602: hdr 是固定大小 8 字节的切片，索引访问安全
	hdr[2] = FrameVersion
	//nolint:gosec // G602: hdr 是固定大小 8 字节的切片，索引访问安全
	hdr[3] = frameType
	//nolint:gosec // G115: len() 返回值已通过 MaxPayloadSize 限制检查，不会溢出
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(payload)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// DecodeFrame 解码一帧，返回类型与负载
func DecodeFrame(r io.Reader) (uint8, []byte, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, &CodecError{Type: ErrTypeIO, Msg: "header read failed", Cause: err}
	}
	if binary.BigEndian.Uint16(hdr[0:2]) != FrameMagic {
		return 0, nil, &CodecError{Type: ErrTypeProtocol, Msg: "invalid magic", Retryable: false}
	}
	//nolint:gosec // G602: hdr 是固定大小 8 字节的切片，索引访问安全
	if hdr[2] != FrameVersion {
		return 0, nil, &CodecError{Type: ErrTypeProtocol, Msg: "unsupported version", Retryable: false}
	}
	//nolint:gosec // G602: hdr 是固定大小 8 字节的切片，索引访问安全
	ft := hdr[3]
	ln := binary.BigEndian.Uint32(hdr[4:8])
	// 边界检查：防止过大帧导致内存溢出
	if ln > MaxPayloadSize {
		return 0, nil, &CodecError{Type: ErrTypeOversize, Msg: "payload too large", Retryable: false}
	}
	var payload []byte
	if ln > 0 {
		payload = make([]byte, ln)
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, &CodecError{Type: ErrTypeIO, Msg: "payload read failed", Cause: err, Retryable: true}
		}
	}
	return ft, payload, nil
}

// CodecErrorType 编解码器错误类型
type CodecErrorType int

const (
	// ErrTypeIO I/O错误，通常可重试
	ErrTypeIO CodecErrorType = iota
	// ErrTypeProtocol 协议错误，不可重试
	ErrTypeProtocol
	// ErrTypeOversize 超大帧错误，不可重试
	ErrTypeOversize
)

// CodecError 分类错误结构
type CodecError struct {
	Type      CodecErrorType
	Msg       string
	Cause     error
	Retryable bool
}

func (e *CodecError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Cause)
	}
	return e.Msg
}

func (e *CodecError) Unwrap() error { return e.Cause }

// IsRetryable 判断错误是否可重试
func (e *CodecError) IsRetryable() bool {
	return e.Retryable || e.Type == ErrTypeIO
}
