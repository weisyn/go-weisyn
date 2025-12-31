package hostabi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// HostABI 错误码测试
// ============================================================================
//
// 🎯 **测试目的**：发现错误码定义和使用中的缺陷和BUG
//
// ============================================================================

// TestGetErrorMessage_AllErrorCodes 测试所有错误码的消息
func TestGetErrorMessage_AllErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     uint32
		expected string
	}{
		{"ErrInvalidParameter", ErrInvalidParameter, "参数无效"},
		{"ErrBufferTooSmall", ErrBufferTooSmall, "缓冲区太小"},
		{"ErrInvalidAddress", ErrInvalidAddress, "地址格式无效"},
		{"ErrInvalidHash", ErrInvalidHash, "哈希格式无效"},
		{"ErrInsufficientBalance", ErrInsufficientBalance, "余额不足"},
		{"ErrUTXONotFound", ErrUTXONotFound, "UTXO未找到"},
		{"ErrResourceNotFound", ErrResourceNotFound, "资源未找到"},
		{"ErrPermissionDenied", ErrPermissionDenied, "权限不足"},
		{"ErrInternalError", ErrInternalError, "内部错误"},
		{"ErrEncodingFailed", ErrEncodingFailed, "编码失败"},
		{"ErrContextNotFound", ErrContextNotFound, "执行上下文未找到"},
		{"ErrMemoryAccessFailed", ErrMemoryAccessFailed, "内存访问失败"},
		{"ErrServiceUnavailable", ErrServiceUnavailable, "服务不可用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorMessage(tt.code)
			assert.Equal(t, tt.expected, result, "错误码 %d 的消息应该正确", tt.code)
		})
	}
}

// TestGetErrorMessage_UnknownErrorCode 测试未知错误码
func TestGetErrorMessage_UnknownErrorCode(t *testing.T) {
	result := GetErrorMessage(9999)
	assert.Equal(t, "未知错误", result, "未知错误码应该返回'未知错误'")
}

// TestGetErrorMessage_ZeroCode 测试零错误码
func TestGetErrorMessage_ZeroCode(t *testing.T) {
	result := GetErrorMessage(0)
	assert.Equal(t, "未知错误", result, "零错误码应该返回'未知错误'")
}

// TestGetErrorMessage_OutOfRange 测试超出范围的错误码
func TestGetErrorMessage_OutOfRange(t *testing.T) {
	tests := []struct {
		name string
		code uint32
	}{
		{"负数转换为uint32", 0xFFFFFFFF},
		{"非常大的错误码", 10000},
		{"边界值-1", 999},
		{"边界值+1", 2000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorMessage(tt.code)
			assert.Equal(t, "未知错误", result, "超出范围的错误码应该返回'未知错误'")
		})
	}
}

// TestErrorCodeConstants 测试错误码常量值
func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		code     uint32
		expected uint32
	}{
		{"ErrInvalidParameter", ErrInvalidParameter, 1001},
		{"ErrBufferTooSmall", ErrBufferTooSmall, 1005},
		{"ErrInvalidAddress", ErrInvalidAddress, 1010},
		{"ErrInvalidHash", ErrInvalidHash, 1011},
		{"ErrInsufficientBalance", ErrInsufficientBalance, 2001},
		{"ErrUTXONotFound", ErrUTXONotFound, 2002},
		{"ErrResourceNotFound", ErrResourceNotFound, 2003},
		{"ErrPermissionDenied", ErrPermissionDenied, 2004},
		{"ErrInternalError", ErrInternalError, 5001},
		{"ErrEncodingFailed", ErrEncodingFailed, 5002},
		{"ErrContextNotFound", ErrContextNotFound, 5003},
		{"ErrMemoryAccessFailed", ErrMemoryAccessFailed, 5004},
		{"ErrServiceUnavailable", ErrServiceUnavailable, 5005},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code, "错误码常量值应该正确")
		})
	}
}

// TestErrorCodeRanges 测试错误码范围分类
func TestErrorCodeRanges(t *testing.T) {
	// 参数错误 (1000-1999)
	paramErrors := []uint32{
		ErrInvalidParameter,  // 1001
		ErrBufferTooSmall,    // 1005
		ErrInvalidAddress,    // 1010
		ErrInvalidHash,       // 1011
	}
	for _, code := range paramErrors {
		assert.GreaterOrEqual(t, code, uint32(1000), "参数错误码应该在1000-1999范围内")
		assert.Less(t, code, uint32(2000), "参数错误码应该在1000-1999范围内")
	}

	// 业务逻辑错误 (2000-2999)
	businessErrors := []uint32{
		ErrInsufficientBalance, // 2001
		ErrUTXONotFound,        // 2002
		ErrResourceNotFound,    // 2003
		ErrPermissionDenied,    // 2004
	}
	for _, code := range businessErrors {
		assert.GreaterOrEqual(t, code, uint32(2000), "业务逻辑错误码应该在2000-2999范围内")
		assert.Less(t, code, uint32(3000), "业务逻辑错误码应该在2000-2999范围内")
	}

	// 系统错误 (5000-5999)
	systemErrors := []uint32{
		ErrInternalError,        // 5001
		ErrEncodingFailed,       // 5002
		ErrContextNotFound,      // 5003
		ErrMemoryAccessFailed,   // 5004
		ErrServiceUnavailable,   // 5005
	}
	for _, code := range systemErrors {
		assert.GreaterOrEqual(t, code, uint32(5000), "系统错误码应该在5000-5999范围内")
		assert.Less(t, code, uint32(6000), "系统错误码应该在5000-5999范围内")
	}
}

// TestGetErrorMessage_Concurrent 测试并发安全性
func TestGetErrorMessage_Concurrent(t *testing.T) {
	errorCodes := []uint32{
		ErrInvalidParameter,
		ErrInternalError,
		ErrUTXONotFound,
		ErrServiceUnavailable,
		9999, // 未知错误码
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for _, code := range errorCodes {
				_ = GetErrorMessage(code)
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

