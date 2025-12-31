package interfaces

import (
	"github.com/weisyn/v1/pkg/types"
)

// ABIService ABI服务接口（ISPC内部接口）
//
// 📋 **架构说明**：
// - 此接口从 `pkg/interfaces/engines.ABIService` 迁移而来
// - 仅供 ISPC 内部使用，不对外暴露
//
// 📖 **规范引用**：
// - 本接口是 `docs/components/core/ispc/abi-and-payload.md` 在 ISPC 内部的 Go 绑定
// - 修改本接口必须同步更新文档
//
// 🎯 **核心职责**：
// - 合约 ABI 的注册和查询
// - 函数调用数据的编码
// - 执行结果的解码
type ABIService interface {
	// RegisterABI 注册合约 ABI 定义
	//
	// 📋 **参数说明**：
	//   - contractID: 合约标识符（64位十六进制字符串）
	//   - abi: 合约 ABI 定义
	//
	// 🔧 **返回值**：
	//   - error: 注册失败时返回错误
	RegisterABI(contractID string, abi *types.ContractABI) error

	// EncodeParameters 基于ABI对函数入参进行编码
	//
	// 📋 **参数说明**：
	//   - contractID: 合约标识符
	//   - method: 方法名称
	//   - args: 函数参数列表
	//
	// 🔧 **返回值**：
	//   - []byte: 编码后的调用数据
	//   - error: 编码失败时返回错误
	EncodeParameters(contractID, method string, args []interface{}) ([]byte, error)

	// DecodeResult 基于ABI对函数返回值进行解码
	//
	// 📋 **参数说明**：
	//   - contractID: 合约标识符
	//   - method: 方法名称
	//   - data: 原始结果数据
	//
	// 🔧 **返回值**：
	//   - []interface{}: 解码后的结果列表
	//   - error: 解码失败时返回错误
	DecodeResult(contractID, method string, data []byte) ([]interface{}, error)
}

