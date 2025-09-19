package execution

import (
	"github.com/weisyn/v1/pkg/types"
)

// ABIService 智能合约 ABI 服务（公共接口）
//
// 职责：
// 1) 为指定合约/资源注册 ABI 定义（版本化由实现维护）
// 2) 基于 ABI 对函数入参进行编码
// 3) 基于 ABI 对函数返回值进行解码
//
// 说明：
// - contractID 为资源标识（如合约地址/哈希），由调用方保证唯一性
// - ABI 结构使用 pkg/types.ContractABI，避免跨层重复定义
// - 解码返回使用通用切片，调用方按业务自行转换
type ABIService interface {
	RegisterABI(contractID string, abi *types.ContractABI) error
	EncodeParameters(contractID, method string, args []interface{}) ([]byte, error)
	DecodeResult(contractID, method string, data []byte) ([]interface{}, error)
}
