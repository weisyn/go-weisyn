// Package diagnostics provides limited reader functionality for diagnostics operations.
package diagnostics

import (
	"errors"
	"io"
)

// ErrReadLimitExceeded 表示读取超过上限
var ErrReadLimitExceeded = errors.New("read limit exceeded")

// LimitedReadAll 在给定 reader 上读取不超过 maxBytes 的数据，超过则返回 ErrReadLimitExceeded
//
// 用于防御式读取，防止恶意或巨大的请求体导致 OOM
func LimitedReadAll(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	// 使用 LimitedReader 限制可读取字节数，并在到达上限后返回特定错误
	lr := &io.LimitedReader{R: r, N: maxBytes + 1}
	buf := make([]byte, 0, clamp64(maxBytes, 64<<10))
	tmp := make([]byte, 32<<10)
	var total int64
	for {
		n, err := lr.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > maxBytes {
				return nil, ErrReadLimitExceeded
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return nil, err
		}
	}
}

func clamp64(v int64, max int64) int64 {
	if v <= 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}

