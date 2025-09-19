// Package crypto 提供WES系统的加密服务接口定义
//
// 🔐 **加密服务管理 (Encryption Service Management)**
//
// 本文件定义了WES区块链系统的加密服务接口，专注于：
// - 非对称加密：基于公钥/私钥对的数据加密和解密服务
// - 对称加密：基于密码的高效数据加密和解密服务
// - 密钥管理：公钥、私钥的安全处理和验证
// - 安全策略：加密算法选择、密钥强度、数据完整性保护
//
// 🎯 **核心功能**
// - EncryptionManager：加密管理器接口，提供完整的加密解密服务
// - 非对称加密：支持RSA、ECC等公钥加密算法
// - 对称加密：支持AES等高效对称加密算法
// - 密码保护：基于用户密码的数据保护机制
//
// 🏗️ **设计原则**
// - 算法无关：抽象加密算法实现细节，支持多种算法
// - 安全优先：采用业界标准的加密算法和安全实践
// - 性能友好：根据数据大小选择合适的加密方案
// - 易用性：简洁的接口设计降低使用复杂度
//
// 🔗 **组件关系**
// - EncryptionManager：被钱包、存储、通信等模块使用
// - 与KeyManager：协同进行密钥生成和管理
// - 与SignatureManager：配合提供完整的密码学服务
package crypto

// EncryptionManager 定义加密解密相关接口
//
// 提供WES区块链系统的完整数据加密服务：
// - 非对称加密：使用公钥/私钥对进行数据加密和解密
// - 对称加密：使用密码进行高效的数据加密和解密
// - 安全保障：确保数据机密性和传输安全
// - 算法支持：兼容多种主流加密算法和密钥格式
type EncryptionManager interface {
	// Encrypt 使用公钥加密数据
	// 参数：
	//   - data: 明文数据
	//   - publicKey: 公钥
	// 返回：加密后的数据、错误
	Encrypt(data []byte, publicKey []byte) ([]byte, error)

	// Decrypt 使用私钥解密数据
	// 参数：
	//   - encryptedData: 加密数据
	//   - privateKey: 私钥
	// 返回：解密后的数据、错误
	Decrypt(encryptedData []byte, privateKey []byte) ([]byte, error)

	// EncryptWithPassword 使用密码加密数据
	// 参数：
	//   - data: 明文数据
	//   - password: 密码
	// 返回：加密后的数据、错误
	EncryptWithPassword(data []byte, password string) ([]byte, error)

	// DecryptWithPassword 使用密码解密数据
	// 参数：
	//   - encryptedData: 加密数据
	//   - password: 密码
	// 返回：解密后的数据、错误
	DecryptWithPassword(encryptedData []byte, password string) ([]byte, error)
}
