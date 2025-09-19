package framework

import (
	"unsafe"
)

// ==================== WES Go合约开发框架 ====================
//
// 🌟 **设计理念**：为WES合约开发提供统一的Go语言框架
//
// 🎯 **核心特性**：
// - 基于TinyGo编译到WASM的合约开发支持
// - 统一的宿主函数绑定和封装
// - 标准化的合约接口实现辅助
// - 内置错误处理和类型转换
// - 简化的UTXO操作和事件发出
//
// 📋 **主要组件**：
// - ContractBase: 基础合约结构
// - HostFunctions: 宿主函数绑定
// - Utils: 通用辅助工具
// - Types: 标准数据类型定义
//

// ==================== 标准错误码 ====================

const (
	SUCCESS                    = 0
	ERROR_INVALID_PARAMS       = 1
	ERROR_INSUFFICIENT_BALANCE = 2
	ERROR_UNAUTHORIZED         = 3
	ERROR_NOT_FOUND            = 4
	ERROR_ALREADY_EXISTS       = 5
	ERROR_EXECUTION_FAILED     = 6
	ERROR_INVALID_STATE        = 7
	ERROR_TIMEOUT              = 8
	ERROR_UNKNOWN              = 999
)

// ==================== 基础数据类型 ====================

// Address 地址类型（20字节）
type Address [20]byte

// Hash 哈希类型（32字节）
type Hash [32]byte

// TokenID 代币ID类型
type TokenID string

// Amount 金额类型
type Amount uint64

// ==================== 合约基础结构 ====================

// ContractBase 合约基础结构
// 提供所有WES合约的通用功能和接口实现
type ContractBase struct {
	// 合约元数据
	Name        string
	Symbol      string
	Version     string
	Description string
	Author      string
	License     string

	// 合约配置
	Interfaces []string
	Features   []string
}

// NewContractBase 创建新的合约基础实例
func NewContractBase(name, symbol, version string) *ContractBase {
	return &ContractBase{
		Name:       name,
		Symbol:     symbol,
		Version:    version,
		Interfaces: []string{"IContractBase"},
		Features:   []string{},
	}
}

// AddInterface 添加实现的接口
func (cb *ContractBase) AddInterface(interfaceName string) {
	cb.Interfaces = append(cb.Interfaces, interfaceName)
}

// AddFeature 添加合约特性
func (cb *ContractBase) AddFeature(feature string) {
	cb.Features = append(cb.Features, feature)
}

// ==================== 通用辅助函数 ====================

// GetString 从内存指针构造字符串
func GetString(ptr uint32, len uint32) string {
	if ptr == 0 || len == 0 {
		return ""
	}
	return string((*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:len:len])
}

// GetBytes 从内存指针获取字节数组
func GetBytes(ptr uint32, len uint32) []byte {
	if ptr == 0 || len == 0 {
		return nil
	}
	return (*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:len:len]
}

// AllocateString 分配字符串到WASM内存并返回指针和长度
func AllocateString(s string) (uint32, uint32) {
	if len(s) == 0 {
		return 0, 0
	}
	ptr := Malloc(uint32(len(s)))
	if ptr == 0 {
		return 0, 0
	}
	copy((*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:len(s)], s)
	return ptr, uint32(len(s))
}

// AllocateBytes 分配字节数组到WASM内存
func AllocateBytes(data []byte) (uint32, uint32) {
	if len(data) == 0 {
		return 0, 0
	}
	ptr := Malloc(uint32(len(data)))
	if ptr == 0 {
		return 0, 0
	}
	copy((*[1 << 20]byte)(unsafe.Pointer(uintptr(ptr)))[:len(data)], data)
	return ptr, uint32(len(data))
}

// Uint64ToString 将uint64转换为字符串
func Uint64ToString(n uint64) string {
	if n == 0 {
		return "0"
	}

	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}

	// 反转数字
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}

	return string(digits)
}

// ParseUint64 从字符串解析uint64
func ParseUint64(s string) uint64 {
	var result uint64
	for _, digit := range s {
		if digit >= '0' && digit <= '9' {
			result = result*10 + uint64(digit-'0')
		} else {
			break
		}
	}
	return result
}

// ==================== 地址和哈希处理 ====================

// AddressFromBytes 从字节数组创建地址
func AddressFromBytes(data []byte) Address {
	var addr Address
	if len(data) >= 20 {
		copy(addr[:], data[:20])
	}
	return addr
}

// AddressToBytes 将地址转换为字节数组
func (addr Address) ToBytes() []byte {
	return addr[:]
}

// AddressToString 将地址转换为十六进制字符串
func (addr Address) ToString() string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, 42) // "0x" + 40 hex chars
	result[0] = '0'
	result[1] = 'x'

	for i, b := range addr {
		result[2+i*2] = hexChars[b>>4]
		result[2+i*2+1] = hexChars[b&0xf]
	}

	return string(result)
}

// HashFromBytes 从字节数组创建哈希
func HashFromBytes(data []byte) Hash {
	var hash Hash
	if len(data) >= 32 {
		copy(hash[:], data[:32])
	}
	return hash
}

// HashToBytes 将哈希转换为字节数组
func (hash Hash) ToBytes() []byte {
	return hash[:]
}

// ==================== JSON辅助函数 ====================

// BuildJSONField 构建JSON字段
func BuildJSONField(key, value string) string {
	return `"` + key + `":"` + value + `"`
}

// BuildJSONObject 构建JSON对象
func BuildJSONObject(fields []string) string {
	result := "{"
	for i, field := range fields {
		if i > 0 {
			result += ","
		}
		result += field
	}
	result += "}"
	return result
}

// BuildJSONArray 构建JSON数组
func BuildJSONArray(items []string) string {
	result := "["
	for i, item := range items {
		if i > 0 {
			result += ","
		}
		result += `"` + item + `"`
	}
	result += "]"
	return result
}

// ==================== 合约参数解析 ====================

// ContractParams 合约调用参数
type ContractParams struct {
	data []byte
}

// NewContractParams 创建参数解析器
func NewContractParams(data []byte) *ContractParams {
	return &ContractParams{data: data}
}

// GetRawData 获取原始数据
func (cp *ContractParams) GetRawData() []byte {
	return cp.data
}

// GetString 获取字符串参数
func (cp *ContractParams) GetString() string {
	return string(cp.data)
}

// ParseJSON 简单的JSON字段提取（简化实现）
func (cp *ContractParams) ParseJSON(key string) string {
	data := string(cp.data)
	keyPattern := `"` + key + `":"`

	startIdx := -1
	for i := 0; i <= len(data)-len(keyPattern); i++ {
		if data[i:i+len(keyPattern)] == keyPattern {
			startIdx = i + len(keyPattern)
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	endIdx := startIdx
	for endIdx < len(data) && data[endIdx] != '"' {
		endIdx++
	}

	if endIdx > startIdx {
		return data[startIdx:endIdx]
	}

	return ""
}

// ==================== 错误处理 ====================

// ContractError 合约错误类型
type ContractError struct {
	Code    uint32
	Message string
}

// Error 实现error接口
func (ce *ContractError) Error() string {
	return ce.Message
}

// NewContractError 创建新的合约错误
func NewContractError(code uint32, message string) *ContractError {
	return &ContractError{
		Code:    code,
		Message: message,
	}
}

// WrapError 封装错误为合约错误
func WrapError(code uint32, err error) *ContractError {
	if err == nil {
		return nil
	}
	return &ContractError{
		Code:    code,
		Message: err.Error(),
	}
}

// ==================== 事件辅助 ====================

// Event 事件结构
type Event struct {
	Name string
	Data map[string]interface{}
}

// NewEvent 创建新事件
func NewEvent(name string) *Event {
	return &Event{
		Name: name,
		Data: make(map[string]interface{}),
	}
}

// AddField 添加事件字段
func (e *Event) AddField(key string, value interface{}) {
	e.Data[key] = value
}

// AddStringField 添加字符串字段
func (e *Event) AddStringField(key, value string) {
	e.Data[key] = value
}

// AddUint64Field 添加数值字段
func (e *Event) AddUint64Field(key string, value uint64) {
	e.Data[key] = value
}

// AddAddressField 添加地址字段
func (e *Event) AddAddressField(key string, addr Address) {
	e.Data[key] = addr.ToString()
}

// ToJSON 转换为JSON字符串（简化实现）
func (e *Event) ToJSON() string {
	fields := []string{
		BuildJSONField("event", e.Name),
		BuildJSONField("timestamp", Uint64ToString(GetTimestamp())),
	}

	// 添加数据字段（简化实现）
	dataFields := []string{}
	for key, value := range e.Data {
		switch v := value.(type) {
		case string:
			dataFields = append(dataFields, BuildJSONField(key, v))
		case uint64:
			dataFields = append(dataFields, BuildJSONField(key, Uint64ToString(v)))
		}
	}

	if len(dataFields) > 0 {
		fields = append(fields, `"data":`+BuildJSONObject(dataFields))
	}

	return BuildJSONObject(fields)
}

// ==================== 元数据辅助 ====================

// BuildMetadataJSON 构建合约元数据JSON
func (cb *ContractBase) BuildMetadataJSON() string {
	fields := []string{
		BuildJSONField("name", cb.Name),
		BuildJSONField("symbol", cb.Symbol),
		BuildJSONField("version", cb.Version),
		BuildJSONField("description", cb.Description),
		BuildJSONField("author", cb.Author),
		BuildJSONField("license", cb.License),
	}

	if len(cb.Interfaces) > 0 {
		fields = append(fields, `"interfaces":`+BuildJSONArray(cb.Interfaces))
	}

	if len(cb.Features) > 0 {
		fields = append(fields, `"features":`+BuildJSONArray(cb.Features))
	}

	return BuildJSONObject(fields)
}
