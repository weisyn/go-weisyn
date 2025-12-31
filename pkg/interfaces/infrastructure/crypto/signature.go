// Package crypto 提供WES系统的数字签名接口定义
//
// ✍️ **数字签名服务 (Digital Signature Service)**
//
// 本文件定义了WES区块链系统的数字签名接口，专注于：
// - secp256k1签名：Bitcoin兼容的数字签名算法
// - 交易签名：交易数据的安全签名和验证
// - 消息签名：任意数据的数字签名和验证
// - 签名格式：支持DER和Compact等多种签名格式
//
// 🎯 **核心功能**
// - SignatureManager：签名管理器接口，提供完整的签名服务
// - 交易签名：专门针对交易的签名和验证
// - 消息签名：通用的数据签名和验证机制
// - 公钥恢复：从签名中恢复公钥的功能
//
// 🏗️ **设计原则**
// - 算法标准：完全兼容Bitcoin的secp256k1签名算法
// - 安全可靠：使用成熟的加密库和签名算法
// - 格式灵活：支持多种签名格式和编码方式
// - 高效验证：快速的签名验证和公钥恢复
//
// 🔗 **组件关系**
// - SignatureManager：被交易、区块、钱包等模块使用
// - 与KeyManager：依赖密钥管理服务进行签名操作
// - 与HashManager：使用哈希服务进行数据摘要计算
package crypto

import "github.com/weisyn/v1/pkg/types"

// 兼容别名（签名哈希类型迁至 pkg/types）
type SignatureHashType = types.SignatureHashType

// 常量别名（向后兼容）
const (
	SigHashAll                = types.SigHashAll
	SigHashNone               = types.SigHashNone
	SigHashSingle             = types.SigHashSingle
	SigHashAnyoneCanPay       = types.SigHashAnyoneCanPay
	SigHashAllAnyoneCanPay    = types.SigHashAllAnyoneCanPay
	SigHashNoneAnyoneCanPay   = types.SigHashNoneAnyoneCanPay
	SigHashSingleAnyoneCanPay = types.SigHashSingleAnyoneCanPay
)

// SignatureManager 定义区块链签名管理相关接口
//
// 🎯 **签名标准（Bitcoin兼容）**：
// - **签名算法**：ECDSA with secp256k1
// - **签名格式**：DER编码 或 (r,s) 64字节格式
// - **哈希算法**：双SHA256（Bitcoin标准）
// - **签名类型**：支持完整的Bitcoin签名哈希类型
//
// 🔧 **签名流程**：
// 交易数据 → 签名哈希 → 私钥签名 → 验证
//
// 🛡️ **安全特性**：
// - 防重放攻击保护
// - 签名规范化（低S值）
// - 支持批量签名和验证
//
// # SignatureManager 定义区块链签名管理相关接口
//
// 提供WES区块链系统的完整数字签名服务：
// - 交易签名：专门针对交易的secp256k1签名
// - 消息签名：通用的数据签名和验证机制
// - 签名验证：对签名有效性和数据完整性的验证
// - 公钥恢复：从签名和数据中恢复公钥信息
type SignatureManager interface {
	// SignTransaction 签名交易数据
	//
	// 使用Bitcoin兼容的交易签名算法，支持多种签名哈希类型
	//
	// 参数：
	//   - txHash: 交易哈希（32字节）
	//   - privateKey: 32字节私钥
	//   - sigHashType: 签名哈希类型
	//
	// 返回：
	//   - []byte: 64字节签名 (r+s) 或 DER编码签名
	//   - error: 签名失败时的错误
	SignTransaction(txHash []byte, privateKey []byte, sigHashType SignatureHashType) ([]byte, error)

	// VerifyTransactionSignature 验证交易签名
	//
	// 验证Bitcoin风格的交易签名
	//
	// 参数：
	//   - txHash: 交易哈希（32字节）
	//   - signature: 签名数据
	//   - publicKey: 公钥（33字节压缩或65字节未压缩）
	//   - sigHashType: 签名哈希类型
	//
	// 返回：
	//   - bool: 签名是否有效
	VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType SignatureHashType) bool

	// Sign 签名任意数据
	//
	// 通用的数据签名方法
	//
	// 参数：
	//   - data: 待签名数据
	//   - privateKey: 32字节私钥
	//
	// 返回：
	//   - []byte: 64字节签名 (r+s)
	//   - error: 签名失败时的错误
	Sign(data []byte, privateKey []byte) ([]byte, error)

	// Verify 验证数据签名
	//
	// 通用的签名验证方法
	//
	// 参数：
	//   - data: 原始数据
	//   - signature: 签名数据
	//   - publicKey: 公钥
	//
	// 返回：
	//   - bool: 签名是否有效
	Verify(data, signature, publicKey []byte) bool

	// SignMessage 签名消息（带前缀）
	//
	// 用于签名用户消息，添加特定前缀防止交易重放
	//
	// 参数：
	//   - message: 用户消息
	//   - privateKey: 32字节私钥
	//
	// 返回：
	//   - []byte: 65字节签名 (r+s+v，支持公钥恢复)
	//   - error: 签名失败时的错误
	SignMessage(message []byte, privateKey []byte) ([]byte, error)

	// VerifyMessage 验证消息签名
	//
	// 验证带前缀的消息签名
	//
	// 参数：
	//   - message: 原始消息
	//   - signature: 65字节签名 (r+s+v)
	//   - publicKey: 公钥
	//
	// 返回：
	//   - bool: 签名是否有效
	VerifyMessage(message []byte, signature []byte, publicKey []byte) bool

	// RecoverPublicKey 从签名恢复公钥
	//
	// 支持从ECDSA签名中恢复公钥，用于无需预先知道公钥的验证场景
	//
	// 参数：
	//   - hash: 消息哈希（32字节）
	//   - signature: 65字节签名 (r+s+v)
	//
	// 返回：
	//   - []byte: 恢复的公钥（33字节压缩格式）
	//   - error: 恢复失败时的错误
	RecoverPublicKey(hash []byte, signature []byte) ([]byte, error)

	// RecoverAddress 从签名恢复地址
	//
	// 直接从签名恢复地址
	//
	// 参数：
	//   - hash: 消息哈希（32字节）
	//   - signature: 65字节签名 (r+s+v)
	//
	// 返回：
	//   - string:WES标准地址
	//   - error: 恢复失败时的错误
	RecoverAddress(hash []byte, signature []byte) (string, error)

	// SignBatch 批量签名
	//
	// 高效的批量签名操作，适用于批量交易处理
	//
	// 参数：
	//   - dataList: 待签名数据列表
	//   - privateKey: 32字节私钥
	//
	// 返回：
	//   - [][]byte: 签名列表
	//   - error: 批量签名失败时的错误
	SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error)

	// VerifyBatch 批量验证签名
	//
	// 高效的批量验证操作
	//
	// 参数：
	//   - dataList: 原始数据列表
	//   - signatureList: 签名列表
	//   - publicKeyList: 公钥列表
	//
	// 返回：
	//   - []bool: 验证结果列表
	//   - error: 批量验证失败时的错误
	VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error)

	// NormalizeSignature 规范化签名
	//
	// 确保签名使用低S值（Bitcoin标准要求）
	//
	// 参数：
	//   - signature: 64字节签名 (r+s)
	//
	// 返回：
	//   - []byte: 规范化后的签名
	//   - error: 规范化失败时的错误
	NormalizeSignature(signature []byte) ([]byte, error)

	// ValidateSignature 验证签名格式
	//
	// 检查签名是否符合Bitcoin ECDSA标准
	//
	// 参数：
	//   - signature: 签名数据
	//
	// 返回：
	//   - error: 签名格式无效时返回错误
	ValidateSignature(signature []byte) error
}
