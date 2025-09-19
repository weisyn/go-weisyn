// Package crypto 提供WES系统的密钥管理接口定义
//
// 🔑 **密钥管理服务 (Key Management Service)**
//
// 本文件定义了WES区块链系统的密钥管理接口，专注于：
// - secp256k1密钥生成：Bitcoin兼容的椭圆曲线密钥对生成
// - 密钥格式管理：支持压缩和未压缩公钥格式的转换
// - 密钥验证机制：私钥、公钥格式和有效性的严格验证
// - 安全密钥操作：内存安全的密钥处理和自动清除机制
//
// 🎯 **核心功能**
// - KeyManager：密钥管理器接口，提供完整的密钥操作服务
// - 密钥生成：基于密码学安全随机数的密钥对生成
// - 格式转换：压缩和未压缩公钥格式之间的转换
// - 安全验证：密钥格式、长度、有效性的多重验证
//
// 🏗️ **设计原则**
// - 标准兼容：完全兼容Bitcoin的secp256k1密钥标准
// - 安全优先：使用密码学安全的随机数生成器
// - 内存安全：自动清除敏感数据，防止内存泄露
// - 格式灵活：支持多种密钥格式以适应不同场景
//
// 🔗 **组件关系**
// - KeyManager：被钱包、签名、地址等模块使用
// - 与AddressManager：配合进行公钥到地址的转换
// - 与SignatureManager：提供签名所需的密钥对
package crypto

// KeyManager 定义区块链密钥管理相关接口
//
// 提供WES区块链系统的完整密钥管理服务：
// - 密钥生成：基于secp256k1椭圆曲线的安全密钥对生成
// - 格式管理：支持压缩和未压缩公钥格式的转换和验证
// - 安全操作：内存安全的密钥处理和验证机制
// - Bitcoin兼容：完全兼容Bitcoin密钥标准和格式
//
// 🎯 **密钥标准（Bitcoin兼容）**：
// - **椭圆曲线**：secp256k1 (与Bitcoin完全一致)
// - **私钥格式**：32字节随机数
// - **公钥格式**：支持压缩(33字节)和未压缩(65字节)格式
// - **推荐格式**：33字节压缩公钥 (节省存储和传输)
//
// 🔧 **密钥推导流程**：
// 随机数 → 私钥(32字节) → 公钥(33/65字节) → 地址(Base58Check)
//
// 🛡️ **安全特性**：
// - 密码学安全的随机数生成
// - 内存安全（私钥自动清除）
// - 支持上下文控制和超时
type KeyManager interface {
	// GenerateKeyPair 生成secp256k1密钥对（Bitcoin标准压缩格式）
	//
	// 返回标准格式：
	//   - 私钥：32字节
	//   - 公钥：33字节压缩格式（Bitcoin标准，默认推荐）
	//
	// 注意：此方法始终返回33字节压缩公钥以确保一致性。
	// 如需未压缩格式，请使用DeriveUncompressedPublicKey方法。
	//
	// 返回：
	//   - []byte: 32字节私钥
	//   - []byte: 33字节压缩公钥
	//   - error: 生成失败时的错误
	GenerateKeyPair() ([]byte, []byte, error)

	// GenerateCompressedKeyPair 生成压缩格式密钥对
	//
	// 专门生成Bitcoin标准的33字节压缩公钥格式
	//
	// 返回：
	//   - []byte: 32字节私钥
	//   - []byte: 33字节压缩公钥
	//   - error: 生成失败时的错误
	GenerateCompressedKeyPair() ([]byte, []byte, error)

	// DerivePublicKey 从私钥导出公钥
	//
	// 参数：
	//   - privateKey: 32字节私钥
	//
	// 返回：
	//   - []byte: 33字节压缩公钥（Bitcoin标准）
	//   - error: 私钥无效时的错误
	DerivePublicKey(privateKey []byte) ([]byte, error)

	// DeriveUncompressedPublicKey 从私钥导出未压缩公钥
	//
	// 用于需要完整公钥坐标的场景
	//
	// 参数：
	//   - privateKey: 32字节私钥
	//
	// 返回：
	//   - []byte: 65字节未压缩公钥
	//   - error: 私钥无效时的错误
	DeriveUncompressedPublicKey(privateKey []byte) ([]byte, error)

	// ParsePublicKeyString 解析十六进制字符串公钥
	//
	// 支持多种格式：
	//   - "02abc123..." (66字符，33字节压缩公钥) - Bitcoin标准
	//   - "03abc123..." (66字符，33字节压缩公钥) - Bitcoin标准
	//   - "04abc123..." (130字符，65字节未压缩公钥) - 兼容格式
	//   - "0x04abc123..." (含0x前缀的格式) - 兼容格式
	//
	// 参数：
	//   - publicKeyHex: 十六进制公钥字符串
	//
	// 返回：
	//   - []byte: 解析后的公钥字节数组
	//   - error: 格式错误或解析失败
	ParsePublicKeyString(publicKeyHex string) ([]byte, error)

	// ValidatePrivateKey 验证私钥有效性
	//
	// 检查私钥是否符合secp256k1的要求
	//
	// 参数：
	//   - privateKey: 待验证的私钥字节
	//
	// 返回：
	//   - error: 私钥无效时返回错误
	ValidatePrivateKey(privateKey []byte) error

	// ValidatePublicKey 验证公钥有效性
	//
	// 检查公钥是否符合secp256k1的要求，支持压缩和未压缩格式
	//
	// 参数：
	//   - publicKey: 待验证的公钥字节
	//
	// 返回：
	//   - error: 公钥无效时返回错误
	ValidatePublicKey(publicKey []byte) error

	// CompressPublicKey 将未压缩公钥转换为压缩格式
	//
	// 参数：
	//   - uncompressedKey: 65字节未压缩公钥
	//
	// 返回：
	//   - []byte: 33字节压缩公钥
	//   - error: 格式错误时返回错误
	CompressPublicKey(uncompressedKey []byte) ([]byte, error)

	// DecompressPublicKey 将压缩公钥转换为未压缩格式
	//
	// 参数：
	//   - compressedKey: 33字节压缩公钥
	//
	// 返回：
	//   - []byte: 65字节未压缩公钥
	//   - error: 格式错误时返回错误
	DecompressPublicKey(compressedKey []byte) ([]byte, error)
}
