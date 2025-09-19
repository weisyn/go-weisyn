// Package crypto 提供WES系统的哈希计算接口定义
//
// #️⃣ **哈希计算服务 (Hash Computation Service)**
//
// 本文件定义了WES区块链系统的哈希计算接口，专注于：
// - 多算法支持：SHA256、SHA3、Keccak256、RIPEMD160等主流算法
// - 安全哈希：双重SHA256、HMAC等安全哈希算法
// - 数据校验：数据完整性和一致性校验机制
// - 性能优化：支持流式计算和批量处理
//
// 🎯 **核心功能**
// - HashManager：哈希管理器接口，提供完整的哈希计算服务
// - 算法多样：支持多种主流加密哈希算法
// - 安全强化：双重SHA256、HMAC等安全机制
// - 数据校验：快速的数据完整性验证
//
// 🏧 **设计原则**
// - 算法全面：支持区块链领域常用的所有哈希算法
// - 性能优先：高效的计算实现和内存管理
// - 安全可靠：使用成熟的加密库和算法实现
// - 易用性：统一的接口设计和错误处理
//
// 🔗 **组件关系**
// - HashManager：被区块、交易、Merkle树等模块使用
// - 与MerkleTreeManager：配合进行Merkle树计算
// - 与SignatureManager：提供签名所需的哈希计算
package crypto

// HashManager 定义哈希计算相关接口
//
// 提供WES区块链系统的完整哈希计算服务：
// - 多算法支持：SHA256、SHA3、Keccak256、RIPEMD160等算法
// - 安全增强：双重SHA256、HMAC等安全哈希机制
// - 数据校验：快速的数据完整性和一致性验证
// - 格式转换：支持十六进制和字节数组格式
type HashManager interface {
	// SHA256 计算SHA-256哈希
	// 参数：
	//   - data: 输入数据
	// 返回：哈希值
	SHA256(data []byte) []byte

	// Keccak256 计算Keccak-256哈希
	// 参数：
	//   - data: 输入数据
	// 返回：哈希值
	Keccak256(data []byte) []byte

	// RIPEMD160 计算RIPEMD-160哈希
	// 参数：
	//   - data: 输入数据
	// 返回：哈希值
	RIPEMD160(data []byte) []byte

	// DoubleSHA256 计算双重SHA-256哈希
	// 参数：
	//   - data: 输入数据
	// 返回：哈希值
	DoubleSHA256(data []byte) []byte
}
