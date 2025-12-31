// Package hostabi provides error definitions for host ABI operations.
package hostabi

// ============================================================================
// Host ABI 错误码定义（P2: 错误处理优化）
// ============================================================================
//
// 🎯 **设计原则**：
//   - 错误码统一管理，便于追踪和调试
//   - 分类清晰：参数错误、系统错误、编码错误等
//   - 兼容 WASM 宿主函数返回值约定（只能返回数值）
//
// 📋 **错误码范围**：
//   - 1000-1999: 参数错误（客户端可修复）
//   - 2000-2999: 业务逻辑错误（需要业务层处理）
//   - 5000-5999: 系统错误（内部问题）
//   - 9000-9999: 编码/序列化错误

const (
	// ==================== 参数错误 (1000-1999) ====================
	
	// ErrInvalidParameter 参数无效
	// 用途：参数格式错误、必填参数缺失、参数值超出范围等
	ErrInvalidParameter = 1001
	
	// ErrBufferTooSmall 缓冲区太小
	// 用途：WASM内存缓冲区不足以容纳返回数据
	ErrBufferTooSmall = 1005
	
	// ErrInvalidAddress 地址格式无效
	// 用途：地址长度不正确、地址格式错误
	ErrInvalidAddress = 1010
	
	// ErrInvalidHash 哈希格式无效
	// 用途：哈希长度不正确、哈希格式错误
	ErrInvalidHash = 1011
	
	// ==================== 业务逻辑错误 (2000-2999) ====================
	
	// ErrInsufficientBalance 余额不足
	// 用途：UTXO余额不足以完成交易
	ErrInsufficientBalance = 2001
	
	// ErrUTXONotFound UTXO未找到
	// 用途：引用的UTXO不存在或已被消费
	ErrUTXONotFound = 2002
	
	// ErrResourceNotFound 资源未找到
	// 用途：引用的资源不存在
	ErrResourceNotFound = 2003
	
	// ErrPermissionDenied 权限不足
	// 用途：调用者没有权限执行操作
	ErrPermissionDenied = 2004
	
	// ==================== 系统错误 (5000-5999) ====================
	
	// ErrInternalError 内部错误
	// 用途：系统内部错误，如服务未初始化、依赖缺失等
	ErrInternalError = 5001
	
	// ErrEncodingFailed 编码失败
	// 用途：JSON序列化/反序列化失败、Protobuf编码失败等
	ErrEncodingFailed = 5002
	
	// ErrContextNotFound 执行上下文未找到
	// 用途：无法从context中提取ExecutionContext
	ErrContextNotFound = 5003
	
	// ErrMemoryAccessFailed 内存访问失败
	// 用途：WASM内存读写失败
	ErrMemoryAccessFailed = 5004
	
	// ErrServiceUnavailable 服务不可用
	// 用途：依赖的服务（如BlockQuery、HashManager）未注入
	ErrServiceUnavailable = 5005
)

// GetErrorMessage 获取错误码对应的错误消息
//
// 📋 **参数**：
//   - code: 错误码
//
// 🔧 **返回值**：
//   - string: 错误消息（中文，用于日志和调试）
func GetErrorMessage(code uint32) string {
	switch code {
	case ErrInvalidParameter:
		return "参数无效"
	case ErrBufferTooSmall:
		return "缓冲区太小"
	case ErrInvalidAddress:
		return "地址格式无效"
	case ErrInvalidHash:
		return "哈希格式无效"
	case ErrInsufficientBalance:
		return "余额不足"
	case ErrUTXONotFound:
		return "UTXO未找到"
	case ErrResourceNotFound:
		return "资源未找到"
	case ErrPermissionDenied:
		return "权限不足"
	case ErrInternalError:
		return "内部错误"
	case ErrEncodingFailed:
		return "编码失败"
	case ErrContextNotFound:
		return "执行上下文未找到"
	case ErrMemoryAccessFailed:
		return "内存访问失败"
	case ErrServiceUnavailable:
		return "服务不可用"
	default:
		return "未知错误"
	}
}

