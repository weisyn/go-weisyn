//go:build tinygo.wasm

package main

import (
	"encoding/binary"
	"unsafe"
)

// 声明主机函数：node_add
//
//go:wasm-module env
//go:export node_add
func nodeAdd(a, b int32) int32

// 声明状态存储相关的主机函数
//
//go:wasm-module env
//go:export state_get
func stateGet(keyPtr, keyLen, valuePtr, valueLen uint32) uint32

//go:wasm-module env
//go:export state_set
func stateSet(keyPtr, keyLen, valuePtr, valueLen uint32) uint32

//go:wasm-module env
//go:export state_exists
func stateExists(keyPtr, keyLen uint32) uint32

// 内存分配函数
//
//go:wasm-module env
//go:export malloc
func malloc(size uint32) unsafe.Pointer

// 设置返回数据函数
//
//go:wasm-module env
//go:export set_return_data
func setReturnData(dataPtr, dataLen uint32) uint32

// 定义状态键
const RESULT_KEY = "last_add_result"

// 从内存中读取参数
func parseParams(paramsPtr, paramsLen uint32) (int32, int32) {
	// 如果参数长度不足8字节(两个int32)，那么参数不完整
	if paramsLen < 8 {
		// 返回零值，避免硬编码。在虚拟机测试代码中，
		// 实际使用的是EncodeAddParams(5, 7)来构造测试参数
		return 0, 0
	}

	// 创建切片引用参数内存
	mem := (*[1 << 30]byte)(unsafe.Pointer(uintptr(paramsPtr)))
	paramBytes := mem[:paramsLen]

	// 读取参数a (前4字节)
	a := int32(binary.LittleEndian.Uint32(paramBytes[0:4]))

	// 读取参数b (后4字节)
	b := int32(binary.LittleEndian.Uint32(paramBytes[4:8]))

	return a, b
}

// Add 计算两数之和并存储
//
//go:export add
func Add(paramsPtr, paramsLen uint32) int32 {
	// 解析参数
	a, b := parseParams(paramsPtr, paramsLen)

	// 调用主机函数进行计算
	// 这样构建了一个完整的链路：主机调用合约的Add，合约调用主机的nodeAdd
	result := nodeAdd(a, b)

	// 转换为字节
	resultBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(resultBytes, uint32(result))

	// 存储结果
	storeResult(resultBytes)

	// 设置返回数据
	resultPtr := bytesToMemory(resultBytes)
	setReturnData(uint32(uintptr(resultPtr)), 4)

	return result
}

// GetLastResult 读取最后一次计算结果
//
//go:export get_last_result
func GetLastResult(paramsPtr, paramsLen uint32) int32 {
	// 解析参数
	a, b := parseParams(paramsPtr, paramsLen)

	// 尝试从状态中读取
	resultBytes := getStoredResult()

	// 如果状态中没有数据，调用主机函数计算并返回
	if resultBytes == nil || len(resultBytes) < 4 {
		// 调用主机函数计算加法结果
		result := nodeAdd(a, b)

		// 设置返回数据
		retBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(retBytes, uint32(result))
		retPtr := bytesToMemory(retBytes)
		setReturnData(uint32(uintptr(retPtr)), 4)

		return result
	}

	// 如果状态中有数据，返回存储的结果
	result := int32(binary.LittleEndian.Uint32(resultBytes))

	// 设置返回数据
	retBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(retBytes, uint32(result))
	retPtr := bytesToMemory(retBytes)
	setReturnData(uint32(uintptr(retPtr)), 4)

	return result
}

// 辅助函数：检查键是否存在
func keyExists(key string) bool {
	// 转换键为字节数组
	keyBytes := []byte(key)

	// 分配内存
	keyPtr := malloc(uint32(len(keyBytes)))
	if keyPtr == nil {
		return false
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := 0; i < len(keyBytes); i++ {
		keyMem[i] = keyBytes[i]
	}

	// 检查键是否存在
	return stateExists(uint32(uintptr(keyPtr)), uint32(len(keyBytes))) == 1
}

// 辅助函数：获取存储的结果
func getStoredResult() []byte {
	// 键
	keyBytes := []byte(RESULT_KEY)
	keyLen := uint32(len(keyBytes))

	// 分配内存存储键
	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return nil
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 为结果分配内存 - 确保内存足够大
	valueLen := uint32(4) // int32 = 4字节
	valuePtr := malloc(valueLen)
	if valuePtr == nil {
		return nil
	}

	// 获取存储的结果
	status := stateGet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(valuePtr)), valueLen)
	if status != 0 {
		return nil
	}

	// 读取结果到Go切片 - 确保完全拷贝
	valueMem := (*[1 << 30]byte)(valuePtr)
	resultBytes := make([]byte, 4)
	for i := 0; i < 4; i++ {
		resultBytes[i] = valueMem[i]
	}

	return resultBytes
}

// 辅助函数：存储结果
func storeResult(resultBytes []byte) bool {
	// 键
	keyBytes := []byte(RESULT_KEY)
	keyLen := uint32(len(keyBytes))

	// 分配内存存储键
	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return false
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 为结果分配内存
	valueLen := uint32(len(resultBytes))
	valuePtr := malloc(valueLen)
	if valuePtr == nil {
		return false
	}

	// 复制结果到内存
	valueMem := (*[1 << 30]byte)(valuePtr)
	for i := uint32(0); i < valueLen; i++ {
		valueMem[i] = resultBytes[i]
	}

	// 存储结果
	status := stateSet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(valuePtr)), valueLen)
	return status == 0
}

// 辅助函数：将字节数组复制到新分配的内存
func bytesToMemory(data []byte) unsafe.Pointer {
	dataLen := uint32(len(data))
	ptr := malloc(dataLen)
	if ptr == nil {
		return nil
	}

	mem := (*[1 << 30]byte)(ptr)
	for i := uint32(0); i < dataLen; i++ {
		mem[i] = data[i]
	}

	return ptr
}

func main() {}
