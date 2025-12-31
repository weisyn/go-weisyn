// Package crypto 提供系统的地址管理接口定义
//
// 📍 **地址管理服务 (Address Management Service)**
//
// 本文件定义了区块链系统的地址管理接口，专注于：
// -地址标准：Bitcoin风格的Base58Check编码地址格式
// - 地址生成算法：secp256k1公钥到标准地址的完整推导流程
// - 格式验证机制：严格的地址格式、校验和、版本字节验证
// - 转换工具集：地址、字节数组、十六进制之间的互相转换
//
// 🎯 **核心功能**
// - AddressManager：地址管理器接口，提供完整的地址操作服务
// - 地址生成：从secp256k1公钥生成标准地址
// - 格式验证：Base58Check编码和校验和验证
// - 类型识别：地址类型判断和比较操作
//
// 🏗️ **设计原则**
// - 标准兼容：完全兼容Bitcoin地址标准和算法
// - 安全可靠：多重验证确保地址的正确性和安全性
// - 格式统一：统一的地址格式标准
// - 易用便捷：丰富的转换和验证工具方法
//
// 🔗 **组件关系**
// - AddressManager：被钱包、交易、查询等模块使用
// - 与KeyManager：配合进行公钥到地址的转换
// - 与TransactionService：用于交易地址验证和处理
package crypto

import "github.com/weisyn/v1/pkg/types"

// AddressManager 定义区块链地址管理相关接口
//
// 提供区块链系统的完整地址管理服务：
// - 地址生成：从secp256k1公钥生成符合标准的地址
// - 格式验证：严格的Base58Check编码和校验和验证
// - 类型识别：地址类型判断、比较和零地址检查
// - 格式转换：地址、字节数组、十六进制字符串间的转换
//
// 🎯 **地址格式标准（Bitcoin风格）**：
// - **标准格式**：Base58Check编码，25-34字符
// - **示例**：Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn
// - **版本字节**：0x1C ( P2PKH)
// - **校验和**：双SHA256的前4字节
//
// 🔧 **推导算法**：
// 私钥 → 公钥(secp256k1) → SHA256 → RIPEMD160 → Base58Check → 地址
//
// 🛡️ **安全特性**：
// - 地址校验和保护
// - 严格的格式验证
// - Bitcoin兼容的密钥推导
type AddressManager interface {
	// PrivateKeyToAddress 从私钥直接生成标准地址
	//
	// 无状态设计的核心方法，完整的私钥到地址推导流程：
	// 私钥(32字节) → 公钥(secp256k1) → SHA256 → RIPEMD160 → Base58Check → 标准地址
	//
	// 推导算法：
	//   1. 使用secp256k1椭圆曲线从私钥导出公钥
	//   2. 对公钥进行SHA256哈希运算
	//   3. 对SHA256结果进行RIPEMD160哈希运算
	//   4. 添加版本字节和校验和
	//   5. 进行Base58Check编码生成最终地址
	//
	// 参数：
	//   - privateKey: 32字节secp256k1私钥
	//
	// 返回：
	//   - string: 标准地址 (Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn)
	//   - error: 私钥无效或生成失败
	//
	// 使用场景：
	//   - 转账交易中计算发送方地址
	//   - 钱包导入私钥后生成对应地址
	//   - 无状态交易验证和处理
	PrivateKeyToAddress(privateKey []byte) (string, error)

	// PublicKeyToAddress 从公钥生成标准地址
	//
	// 支持的公钥格式：
	//   - 33字节压缩公钥 (推荐)
	//   - 64字节未压缩公钥 (兼容)
	//
	// 推导流程：
	//   公钥 → SHA256 → RIPEMD160 → 版本字节+校验和 → Base58编码
	//
	// 参数：
	//   - publicKey: secp256k1公钥字节数组
	//
	// 返回：
	//   - string:标准地址 (Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn)
	//   - error: 公钥格式错误或生成失败
	PublicKeyToAddress(publicKey []byte) (string, error)

	// StringToAddress 解析字符串为标准地址格式
	//
	// 只接受 Bitcoin风格地址：
	//   - Base58Check编码
	//   - 正确的版本字节
	//   - 有效的校验和
	//
	// 参数：
	//   - addressStr:地址字符串
	//
	// 返回：
	//   - string: 验证后的标准地址
	//   - error: 地址格式无效
	StringToAddress(addressStr string) (string, error)

	// ValidateAddress 验证地址格式和校验和
	//
	// 验证内容：
	//   - Base58字符有效性
	//   - 地址长度（25-34字符）
	//   - Base58Check校验和
	//   -版本字节匹配
	//
	// 参数：
	//   - address:地址字符串
	//
	// 返回：
	//   - bool: 是否为有效地址
	//   - error: 验证过程中的错误
	ValidateAddress(address string) (bool, error)

	// AddressToBytes 将地址转换为原始字节数组
	//
	// 用途：用于内部protobuf处理、UTXO索引、哈希计算
	//
	// 参数：
	//   - address:标准地址字符串
	//
	// 返回：
	//   - []byte: 20字节地址哈希
	//   - error: 地址无效或解码失败
	AddressToBytes(address string) ([]byte, error)

	// BytesToAddress 将字节数组转换为标准地址
	//
	// 用途：从存储、索引等场景恢复地址字符串
	//
	// 参数：
	//   - addressBytes: 20字节地址哈希
	//
	// 返回：
	//   - string:标准地址
	//   - error: 输入长度错误
	BytesToAddress(addressBytes []byte) (string, error)

	// AddressToHexString 将地址转换为十六进制字符串
	//
	// 用途：调试和内部处理
	//
	// 参数：
	//   - address:标准地址
	//
	// 返回：
	//   - string: 十六进制字符串（40字符，无前缀）
	//   - error: 地址无效
	AddressToHexString(address string) (string, error)

	// HexStringToAddress 将十六进制字符串转换为地址
	//
	// 用途：从调试数据或内部格式转换
	//
	// 参数：
	//   - hexStr: 十六进制地址字符串（40字符）
	//
	// 返回：
	//   - string:标准地址
	//   - error: 格式无效
	HexStringToAddress(hexStr string) (string, error)

	// GetAddressType 获取地址类型
	//
	// 对于系统，只返回Bitcoin类型或Invalid
	//
	// 参数：
	//   - address: 地址字符串
	//
	// 返回：
	//   - AddressType: 地址类型枚举
	//   - error: 地址无效
	GetAddressType(address string) (AddressType, error)

	// CompareAddresses 比较两个地址是否相等
	//
	// 通过比较字节数组确保准确性
	//
	// 参数：
	//   - addr1, addr2: 待比较的地址字符串
	//
	// 返回：
	//   - bool: 地址是否相等
	//   - error: 任一地址格式无效
	CompareAddresses(addr1, addr2 string) (bool, error)

	// IsZeroAddress 检查是否为零地址
	//
	// 参数：
	//   - address:地址字符串
	//
	// 返回：
	//   - bool: 是否为零地址
	IsZeroAddress(address string) bool
}

// 兼容别名（枚举迁至 pkg/types）
type AddressType = types.AddressType

// 常量别名
const (
	AddressTypeBitcoin = types.AddressTypeBitcoin
	AddressTypeInvalid = types.AddressTypeInvalid
)
