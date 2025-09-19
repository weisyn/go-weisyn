//go:build tinygo.wasm

package main

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// 声明获取区块的主机函数
//
//go:wasm-module env
//go:export host_get_block_by_height
func hostGetBlockByHeight(heightPtr, heightLen uint32, resultPtr, resultLen uint32) uint32

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
const LAST_QUERY_HEIGHT_KEY = "last_query_height"
const LAST_QUERY_RESULT_KEY = "last_query_result"

// 从内存中读取区块高度参数
func parseHeightParam(paramsPtr, paramsLen uint32) uint64 {
	// 如果参数长度不足8字节(一个uint64)，那么参数不完整
	if paramsLen < 8 {
		return 0
	}

	// 创建切片引用参数内存
	mem := (*[1 << 30]byte)(unsafe.Pointer(uintptr(paramsPtr)))
	paramBytes := mem[:paramsLen]

	// 读取参数height (8字节)
	height := binary.LittleEndian.Uint64(paramBytes[0:8])

	return height
}

// GetBlockByHeight 通过区块高度获取区块信息
//
//go:export get_block_by_height
func GetBlockByHeight(paramsPtr, paramsLen uint32) int32 {
	// 解析区块高度参数
	height := parseHeightParam(paramsPtr, paramsLen)

	// 将高度转换为字节数组
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, height)

	// 分配内存存储高度
	heightPtr := bytesToMemory(heightBytes)
	if heightPtr == nil {
		return -1
	}

	// 分配内存接收区块数据 (预设大小，实际大小取决于区块大小)
	maxResultSize := uint32(4096) // 4KB 缓冲区
	resultPtr := malloc(maxResultSize)
	if resultPtr == nil {
		return -1
	}

	// 调用主机函数获取区块数据
	status := hostGetBlockByHeight(
		uint32(uintptr(heightPtr)),
		8,
		uint32(uintptr(resultPtr)),
		maxResultSize,
	)

	// 状态码常量 - 与主机端定义保持一致
	const (
		StatusSuccess           uint32 = 0 // 成功
		StatusOutOfExecutionFee uint32 = 1 // 执行费用不足
		StatusInvalidAccess     uint32 = 2 // 内存访问无效
		StatusRuntimeError      uint32 = 3 // 运行时错误
		StatusTimeout           uint32 = 4 // 执行超时
		StatusInvalidInput      uint32 = 5 // 输入参数无效
		StatusNotFound          uint32 = 6 // 资源未找到
	)

	// 处理不同的错误情况
	if status == StatusNotFound {
		// 区块不存在的情况，返回友好的错误消息
		errorMsg := []byte(fmt.Sprintf("Block with height %d not found", height))
		errorPtr := bytesToMemory(errorMsg)
		setReturnData(uint32(uintptr(errorPtr)), uint32(len(errorMsg)))
		return -1 // 返回错误状态
	} else if status != StatusSuccess {
		// 其他错误情况
		errorMsg := []byte("Failed to get block")
		errorPtr := bytesToMemory(errorMsg)
		setReturnData(uint32(uintptr(errorPtr)), uint32(len(errorMsg)))
		return -1
	}

	// 读取实际结果长度 (假设主机函数在前4个字节存储了数据长度)
	resultMem := (*[1 << 30]byte)(resultPtr)
	resultLength := binary.LittleEndian.Uint32(resultMem[0:4])

	// 复制区块数据
	blockData := make([]byte, resultLength)
	for i := uint32(0); i < resultLength; i++ {
		blockData[i] = resultMem[4+i] // 跳过长度前缀
	}

	// 存储查询结果
	storeLastQueryHeight(heightBytes)
	storeLastQueryResult(blockData)

	// 设置返回数据
	blockDataPtr := bytesToMemory(blockData)
	setReturnData(uint32(uintptr(blockDataPtr)), resultLength)

	return 0
}

// GetLastQueriedBlock 获取最后一次查询的区块
//
//go:export get_last_queried_block
func GetLastQueriedBlock(paramsPtr, paramsLen uint32) int32 {
	// 从状态中读取最后查询的区块数据
	blockData := getLastQueryResult()
	if blockData == nil || len(blockData) == 0 {
		// 设置错误信息作为返回数据
		errorMsg := []byte("没有查询记录")
		errorPtr := bytesToMemory(errorMsg)
		setReturnData(uint32(uintptr(errorPtr)), uint32(len(errorMsg)))
		return -1
	}

	// 设置返回数据
	blockDataPtr := bytesToMemory(blockData)
	setReturnData(uint32(uintptr(blockDataPtr)), uint32(len(blockData)))

	return 0
}

// 辅助函数：存储最后查询的区块高度
func storeLastQueryHeight(heightBytes []byte) bool {
	keyBytes := []byte(LAST_QUERY_HEIGHT_KEY)
	keyLen := uint32(len(keyBytes))

	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return false
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 存储高度数据
	valueLen := uint32(len(heightBytes))
	valuePtr := malloc(valueLen)
	if valuePtr == nil {
		return false
	}

	valueMem := (*[1 << 30]byte)(valuePtr)
	for i := uint32(0); i < valueLen; i++ {
		valueMem[i] = heightBytes[i]
	}

	status := stateSet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(valuePtr)), valueLen)
	return status == 0
}

// 辅助函数：存储最后查询的区块数据
func storeLastQueryResult(blockData []byte) bool {
	keyBytes := []byte(LAST_QUERY_RESULT_KEY)
	keyLen := uint32(len(keyBytes))

	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return false
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 存储区块数据
	valueLen := uint32(len(blockData))
	valuePtr := malloc(valueLen)
	if valuePtr == nil {
		return false
	}

	valueMem := (*[1 << 30]byte)(valuePtr)
	for i := uint32(0); i < valueLen; i++ {
		valueMem[i] = blockData[i]
	}

	status := stateSet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(valuePtr)), valueLen)
	return status == 0
}

// 辅助函数：获取最后查询的区块高度
func getLastQueryHeight() []byte {
	keyBytes := []byte(LAST_QUERY_HEIGHT_KEY)
	keyLen := uint32(len(keyBytes))

	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return nil
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 分配内存获取高度数据
	valueLen := uint32(8) // uint64 = 8字节
	valuePtr := malloc(valueLen)
	if valuePtr == nil {
		return nil
	}

	status := stateGet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(valuePtr)), valueLen)
	if status != 0 {
		return nil
	}

	// 读取高度数据
	valueMem := (*[1 << 30]byte)(valuePtr)
	heightBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		heightBytes[i] = valueMem[i]
	}

	return heightBytes
}

// 辅助函数：获取最后查询的区块数据
func getLastQueryResult() []byte {
	keyBytes := []byte(LAST_QUERY_RESULT_KEY)
	keyLen := uint32(len(keyBytes))

	keyPtr := malloc(keyLen)
	if keyPtr == nil {
		return nil
	}

	// 复制键到内存
	keyMem := (*[1 << 30]byte)(keyPtr)
	for i := uint32(0); i < keyLen; i++ {
		keyMem[i] = keyBytes[i]
	}

	// 首先获取状态大小
	sizePtr := malloc(4)
	if sizePtr == nil {
		return nil
	}

	// 尝试获取大小
	status := stateGet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(sizePtr)), 4)
	if status != 0 {
		return nil
	}

	// 读取大小
	sizeMem := (*[1 << 30]byte)(sizePtr)
	size := binary.LittleEndian.Uint32(sizeMem[0:4])

	// 如果大小为0，返回nil
	if size == 0 {
		return nil
	}

	// 分配内存获取完整数据
	dataPtr := malloc(size)
	if dataPtr == nil {
		return nil
	}

	// 获取完整数据
	status = stateGet(uint32(uintptr(keyPtr)), keyLen, uint32(uintptr(dataPtr)), size)
	if status != 0 {
		return nil
	}

	// 读取数据
	dataMem := (*[1 << 30]byte)(dataPtr)
	result := make([]byte, size)
	for i := uint32(0); i < size; i++ {
		result[i] = dataMem[i]
	}

	return result
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
