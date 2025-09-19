//go:build !tinygo && !(js && wasm)

package framework

// 该文件为非TinyGo/非WASM环境提供空实现，使得 go build ./... 能通过编译。
// 注意：这些实现仅用于宿主环境的编译占位，不会在合约WASM中使用。

// 基础环境函数占位
func getCaller(addrPtr uint32) uint32                           { return 0 }
func getContractAddress(addrPtr uint32) uint32                  { return 0 }
func setReturnData(dataPtr uint32, dataLen uint32) uint32       { return SUCCESS }
func emitEvent(eventPtr uint32, eventLen uint32) uint32         { return SUCCESS }
func getContractInitParams(bufPtr uint32, bufLen uint32) uint32 { return 0 }
func getTimestamp() uint64                                      { return 0 }
func getBlockHeight() uint64                                    { return 0 }
func getBlockHash(height uint64, hashPtr uint32) uint32         { return SUCCESS }

// UTXO操作函数占位
func createUTXOOutput(recipientPtr uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32 {
	return SUCCESS
}
func executeUTXOTransfer(fromPtr uint32, toPtr uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32 {
	return SUCCESS
}
func queryUTXOBalance(addressPtr uint32, tokenIDPtr uint32, tokenIDLen uint32) uint64 { return 0 }

// 状态查询占位
func stateGet(keyPtr uint32, keyLen uint32, valuePtr uint32, valueLen uint32) uint32 { return SUCCESS }
func stateExists(keyPtr uint32, keyLen uint32) uint32                                { return 0 }

// 内存管理占位
func malloc(size uint32) uint32 { return 1 }

// ==================== 导出封装函数（宿主占位实现） ====================

// GetCaller 获取合约调用者地址（占位实现）
func GetCaller() Address { return Address{} }

// GetContractAddress 获取当前合约地址（占位实现）
func GetContractAddress() Address { return Address{} }

// GetTimestamp 获取当前时间戳（占位实现）
func GetTimestamp() uint64 { return 0 }

// GetBlockHeight 获取当前区块高度（占位实现）
func GetBlockHeight() uint64 { return 0 }

// GetBlockHash 获取指定高度的区块哈希（占位实现）
func GetBlockHash(height uint64) Hash { return Hash{} }

// GetContractParams 获取合约调用参数（占位实现）
func GetContractParams() *ContractParams { return NewContractParams([]byte{}) }

// SetReturnData 设置返回数据（占位实现）
func SetReturnData(data []byte) error { return nil }

// SetReturnString 设置字符串返回数据（占位实现）
func SetReturnString(s string) error { return nil }

// SetReturnJSON 设置JSON返回数据（占位实现）
func SetReturnJSON(obj interface{}) error { return nil }

// EmitEvent 发出事件（占位实现）
func EmitEvent(event *Event) error { return nil }

// EmitSimpleEvent 发出简单事件（占位实现）
func EmitSimpleEvent(name string, data map[string]string) error { return nil }

// CreateUTXO 创建UTXO输出（占位实现）
func CreateUTXO(recipient Address, amount Amount, tokenID TokenID) error { return nil }

// TransferUTXO 执行UTXO转移（占位实现）
func TransferUTXO(from, to Address, amount Amount, tokenID TokenID) error { return nil }

// QueryBalance 查询UTXO余额（占位实现）
func QueryBalance(address Address, tokenID TokenID) Amount { return 0 }

// GetState 获取状态数据（占位实现）
func GetState(key string) ([]byte, error) { return []byte{}, nil }

// StateExists 状态是否存在（占位实现）
func StateExists(key string) bool { return false }

// Malloc 分配内存（占位实现）
func Malloc(size uint32) uint32 { return malloc(size) }
