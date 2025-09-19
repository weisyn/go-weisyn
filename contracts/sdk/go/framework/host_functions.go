//go:build tinygo || (js && wasm)

package framework

// ==================== WES 宿主函数Go绑定库 ====================
//
// 🌟 **设计理念**：为WES合约提供统一的宿主函数访问接口
//
// 🎯 **核心特性**：
// - 封装所有WES宿主函数的底层调用
// - 提供类型安全的Go语言接口
// - 内置错误处理和参数验证
// - 支持UTXO操作、事件发出、环境查询等
// - 简化合约开发的复杂性
//

// ==================== 宿主函数原始声明 ====================

// 🔧 注意：TinyGo 0.31+ 要求 //go:wasmimport 函数必须是声明，不能有函数体
// 这些函数在WASM编译时会被链接到宿主函数
//
// 📋 版本兼容性：
// - TinyGo 0.30及以下：不兼容（需要函数体 { return 0 }）
// - TinyGo 0.31及以上：完全兼容（只需函数声明）
//
// 💡 如果您使用旧版本TinyGo，请升级到0.31+：
//   brew upgrade tinygo

// 基础环境函数
//
//go:wasmimport env get_caller
func getCaller(addrPtr uint32) uint32

//go:wasmimport env get_contract_address
func getContractAddress(addrPtr uint32) uint32

//go:wasmimport env set_return_data
func setReturnData(dataPtr uint32, dataLen uint32) uint32

//go:wasmimport env emit_event
func emitEvent(eventPtr uint32, eventLen uint32) uint32

//go:wasmimport env get_contract_init_params
func getContractInitParams(bufPtr uint32, bufLen uint32) uint32

//go:wasmimport env get_timestamp
func getTimestamp() uint64

//go:wasmimport env get_block_height
func getBlockHeight() uint64

//go:wasmimport env get_block_hash
func getBlockHash(height uint64, hashPtr uint32) uint32

// UTXO操作函数
//
//go:wasmimport env create_utxo_output
func createUTXOOutput(recipientPtr uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32

//go:wasmimport env execute_utxo_transfer
func executeUTXOTransfer(fromPtr uint32, toPtr uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32

//go:wasmimport env query_utxo_balance
func queryUTXOBalance(addressPtr uint32, tokenIDPtr uint32, tokenIDLen uint32) uint64

// 状态查询函数（可选）
//
//go:wasmimport env state_get
func stateGet(keyPtr uint32, keyLen uint32, valuePtr uint32, valueLen uint32) uint32

//go:wasmimport env state_exists
func stateExists(keyPtr uint32, keyLen uint32) uint32

// 内存管理函数
//
//go:wasmimport env malloc
func malloc(size uint32) uint32

// ==================== 封装的宿主函数接口 ====================

// ===== 环境信息函数 =====

// GetCaller 获取合约调用者地址
func GetCaller() Address {
	addr := malloc(20)
	if addr == 0 {
		return Address{}
	}

	getCaller(addr)
	return AddressFromBytes(GetBytes(addr, 20))
}

// GetContractAddress 获取当前合约地址
func GetContractAddress() Address {
	addr := malloc(20)
	if addr == 0 {
		return Address{}
	}

	getContractAddress(addr)
	return AddressFromBytes(GetBytes(addr, 20))
}

// GetTimestamp 获取当前时间戳
func GetTimestamp() uint64 {
	return getTimestamp()
}

// GetBlockHeight 获取当前区块高度
func GetBlockHeight() uint64 {
	return getBlockHeight()
}

// GetBlockHash 获取指定高度的区块哈希
func GetBlockHash(height uint64) Hash {
	hashPtr := malloc(32)
	if hashPtr == 0 {
		return Hash{}
	}

	result := getBlockHash(height, hashPtr)
	if result != SUCCESS {
		return Hash{}
	}

	return HashFromBytes(GetBytes(hashPtr, 32))
}

// ===== 合约参数和返回值函数 =====

// GetContractParams 获取合约调用参数
func GetContractParams() *ContractParams {
	// 分配足够大的缓冲区
	bufSize := uint32(8192)
	buffer := malloc(bufSize)
	if buffer == 0 {
		return NewContractParams([]byte{})
	}

	actualLen := getContractInitParams(buffer, bufSize)
	if actualLen == 0 {
		return NewContractParams([]byte{})
	}

	data := GetBytes(buffer, actualLen)
	return NewContractParams(data)
}

// SetReturnData 设置合约返回数据
func SetReturnData(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	dataPtr, dataLen := AllocateBytes(data)
	if dataPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate return data")
	}

	result := setReturnData(dataPtr, dataLen)
	if result != SUCCESS {
		return NewContractError(result, "failed to set return data")
	}

	return nil
}

// SetReturnString 设置字符串返回数据
func SetReturnString(s string) error {
	return SetReturnData([]byte(s))
}

// SetReturnJSON 设置JSON格式返回数据
func SetReturnJSON(obj interface{}) error {
	// 简化的JSON序列化（实际项目中应使用更完整的实现）
	var jsonStr string

	switch v := obj.(type) {
	case string:
		jsonStr = `"` + v + `"`
	case uint64:
		jsonStr = Uint64ToString(v)
	case map[string]interface{}:
		fields := []string{}
		for key, value := range v {
			switch val := value.(type) {
			case string:
				fields = append(fields, BuildJSONField(key, val))
			case uint64:
				fields = append(fields, BuildJSONField(key, Uint64ToString(val)))
			}
		}
		jsonStr = BuildJSONObject(fields)
	default:
		return NewContractError(ERROR_INVALID_PARAMS, "unsupported return type")
	}

	return SetReturnString(jsonStr)
}

// ===== 事件发出函数 =====

// EmitEvent 发出事件
func EmitEvent(event *Event) error {
	if event == nil {
		return NewContractError(ERROR_INVALID_PARAMS, "event cannot be nil")
	}

	eventJSON := event.ToJSON()
	eventPtr, eventLen := AllocateString(eventJSON)
	if eventPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate event data")
	}

	result := emitEvent(eventPtr, eventLen)
	if result != SUCCESS {
		return NewContractError(result, "failed to emit event")
	}

	return nil
}

// EmitSimpleEvent 发出简单事件
func EmitSimpleEvent(name string, data map[string]string) error {
	event := NewEvent(name)
	for key, value := range data {
		event.AddStringField(key, value)
	}
	return EmitEvent(event)
}

// ===== UTXO操作函数 =====

// CreateUTXO 创建UTXO输出
func CreateUTXO(recipient Address, amount Amount, tokenID TokenID) error {
	recipientPtr, _ := AllocateBytes(recipient.ToBytes())
	if recipientPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate recipient address")
	}

	tokenIDPtr, tokenIDLen := AllocateString(string(tokenID))
	if tokenIDPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate token ID")
	}

	result := createUTXOOutput(recipientPtr, uint64(amount), tokenIDPtr, tokenIDLen)
	if result != SUCCESS {
		return NewContractError(result, "failed to create UTXO output")
	}

	return nil
}

// TransferUTXO 执行UTXO转移
func TransferUTXO(from, to Address, amount Amount, tokenID TokenID) error {
	fromPtr, _ := AllocateBytes(from.ToBytes())
	if fromPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate from address")
	}

	toPtr, _ := AllocateBytes(to.ToBytes())
	if toPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate to address")
	}

	tokenIDPtr, tokenIDLen := AllocateString(string(tokenID))
	if tokenIDPtr == 0 {
		return NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate token ID")
	}

	result := executeUTXOTransfer(fromPtr, toPtr, uint64(amount), tokenIDPtr, tokenIDLen)
	if result != SUCCESS {
		return NewContractError(result, "failed to transfer UTXO")
	}

	return nil
}

// QueryBalance 查询UTXO余额
func QueryBalance(address Address, tokenID TokenID) Amount {
	addressPtr, _ := AllocateBytes(address.ToBytes())
	if addressPtr == 0 {
		return 0
	}

	tokenIDPtr, tokenIDLen := AllocateString(string(tokenID))
	if tokenIDPtr == 0 {
		return 0
	}

	balance := queryUTXOBalance(addressPtr, tokenIDPtr, tokenIDLen)
	return Amount(balance)
}

// ===== 状态查询函数（可选，仅限只读操作）=====

// GetState 获取状态数据（只读）
func GetState(key string) ([]byte, error) {
	keyPtr, keyLen := AllocateString(key)
	if keyPtr == 0 {
		return nil, NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate key")
	}

	// 分配返回值缓冲区
	maxValueSize := uint32(4096)
	valuePtr := malloc(maxValueSize)
	if valuePtr == 0 {
		return nil, NewContractError(ERROR_EXECUTION_FAILED, "failed to allocate value buffer")
	}

	result := stateGet(keyPtr, keyLen, valuePtr, maxValueSize)
	if result != SUCCESS {
		return nil, NewContractError(result, "failed to get state")
	}

	// 简化实现：假设实际长度存储在特定位置
	// 实际实现中需要根据具体的宿主函数规范来处理
	value := GetBytes(valuePtr, maxValueSize)
	return value, nil
}

// StateExists 检查状态是否存在
func StateExists(key string) bool {
	keyPtr, keyLen := AllocateString(key)
	if keyPtr == 0 {
		return false
	}

	result := stateExists(keyPtr, keyLen)
	return result == 1 // 假设1表示存在，0表示不存在
}

// ===== 内存管理函数 =====

// Malloc 分配内存
func Malloc(size uint32) uint32 {
	return malloc(size)
}

// ==================== 高级封装函数 ====================

// ===== 合约标准接口辅助 =====

// StandardInitialize 标准合约初始化辅助
func StandardInitialize(contract *ContractBase, customInit func(*ContractParams) error) error {
	params := GetContractParams()

	// 执行自定义初始化逻辑
	if customInit != nil {
		if err := customInit(params); err != nil {
			return err
		}
	}

	// 发出初始化事件
	event := NewEvent("Initialize")
	event.AddStringField("contract_name", contract.Name)
	event.AddStringField("version", contract.Version)
	event.AddAddressField("contract_address", GetContractAddress())
	event.AddUint64Field("timestamp", GetTimestamp())

	return EmitEvent(event)
}

// StandardGetMetadata 标准元数据获取辅助
func StandardGetMetadata(contract *ContractBase) error {
	metadata := contract.BuildMetadataJSON()
	return SetReturnString(metadata)
}

// StandardGetVersion 标准版本获取辅助
func StandardGetVersion(contract *ContractBase) error {
	return SetReturnString(contract.Version)
}

// ===== 代币合约辅助函数 =====

// TokenTransfer 代币转账辅助
func TokenTransfer(tokenID TokenID, to Address, amount Amount) error {
	caller := GetCaller()

	// 检查余额
	balance := QueryBalance(caller, tokenID)
	if balance < amount {
		return NewContractError(ERROR_INSUFFICIENT_BALANCE, "insufficient token balance")
	}

	// 执行转账
	if err := TransferUTXO(caller, to, amount, tokenID); err != nil {
		return err
	}

	// 发出转账事件
	event := NewEvent("Transfer")
	event.AddAddressField("from", caller)
	event.AddAddressField("to", to)
	event.AddStringField("token_id", string(tokenID))
	event.AddUint64Field("amount", uint64(amount))

	return EmitEvent(event)
}

// TokenMint 代币铸造辅助
func TokenMint(tokenID TokenID, to Address, amount Amount) error {
	// 创建新的代币UTXO
	if err := CreateUTXO(to, amount, tokenID); err != nil {
		return err
	}

	// 发出铸造事件
	event := NewEvent("Mint")
	event.AddAddressField("to", to)
	event.AddStringField("token_id", string(tokenID))
	event.AddUint64Field("amount", uint64(amount))
	event.AddAddressField("minter", GetCaller())

	return EmitEvent(event)
}

// TokenGetBalance 代币余额查询辅助
func TokenGetBalance(address Address, tokenID TokenID) error {
	balance := QueryBalance(address, tokenID)

	result := map[string]interface{}{
		"address":  address.ToString(),
		"token_id": string(tokenID),
		"balance":  uint64(balance),
	}

	return SetReturnJSON(result)
}

// ===== NFT合约辅助函数 =====

// NFTMint NFT铸造辅助
func NFTMint(tokenID TokenID, to Address, metadata map[string]string) error {
	// 检查NFT是否已存在
	existingBalance := QueryBalance(to, tokenID)
	if existingBalance > 0 {
		return NewContractError(ERROR_ALREADY_EXISTS, "NFT already exists")
	}

	// 创建NFT UTXO（数量为1表示不可分割）
	if err := CreateUTXO(to, 1, tokenID); err != nil {
		return err
	}

	// 发出铸造事件
	event := NewEvent("NFTMint")
	event.AddStringField("token_id", string(tokenID))
	event.AddAddressField("to", to)
	event.AddAddressField("minter", GetCaller())

	// 添加元数据
	for key, value := range metadata {
		event.AddStringField("metadata_"+key, value)
	}

	return EmitEvent(event)
}

// NFTTransfer NFT转移辅助
func NFTTransfer(tokenID TokenID, from, to Address) error {
	// 检查所有权
	balance := QueryBalance(from, tokenID)
	if balance == 0 {
		return NewContractError(ERROR_NOT_FOUND, "NFT not found or not owned")
	}

	// 执行转移
	if err := TransferUTXO(from, to, 1, tokenID); err != nil {
		return err
	}

	// 发出转移事件
	event := NewEvent("NFTTransfer")
	event.AddStringField("token_id", string(tokenID))
	event.AddAddressField("from", from)
	event.AddAddressField("to", to)

	return EmitEvent(event)
}

// ===== 工具函数 =====

// ValidateAddress 验证地址格式
func ValidateAddress(addr Address) error {
	// 简单验证：检查是否为零地址
	zeroAddr := Address{}
	if addr == zeroAddr {
		return NewContractError(ERROR_INVALID_PARAMS, "invalid zero address")
	}
	return nil
}

// ValidateAmount 验证金额
func ValidateAmount(amount Amount) error {
	if amount == 0 {
		return NewContractError(ERROR_INVALID_PARAMS, "invalid zero amount")
	}
	return nil
}

// ValidateTokenID 验证代币ID
func ValidateTokenID(tokenID TokenID) error {
	if len(string(tokenID)) == 0 {
		return NewContractError(ERROR_INVALID_PARAMS, "invalid empty token ID")
	}
	return nil
}
