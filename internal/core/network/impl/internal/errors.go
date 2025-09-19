package internal

// errors.go
// 内部错误模型与错误码归纳（方法框架）

import "errors"

var (
	ErrInvalidEnvelope  = errors.New("invalid envelope")
	ErrEncodeFailed     = errors.New("encode failed")
	ErrDecodeFailed     = errors.New("decode failed")
	ErrCompressFailed   = errors.New("compress failed")
	ErrDecompressFailed = errors.New("decompress failed")
	ErrSignFailed       = errors.New("sign failed")
	ErrVerifyFailed     = errors.New("verify failed")
)
