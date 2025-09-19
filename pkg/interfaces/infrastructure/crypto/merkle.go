// Package crypto 提供WES系统的Merkle树管理接口定义
//
// 🌳 **Merkle树管理服务 (Merkle Tree Management Service)**
//
// 本文件定义了WES区块链系统的Merkle树管理接口，专注于：
// - Merkle树构建：从交易列表构建完整的Merkle树结构
// - 根哈希计算：高效的Merkle根哈希计算算法
// - 证明生成：Merkle证明路径的生成和验证
// - 数据验证：交易存在性和完整性的快速验证
//
// 🎯 **核心功能**
// - MerkleTreeManager：Merkle树管理器接口，提供完整的树操作服务
// - MerkleTree：Merkle树实例接口，表示具体的树结构
// - 树构建：从交易数据到Merkle树的完整构建过程
// - 证明系统：Merkle证明的生成、验证和管理
//
// 🏗️ **设计原则**
// - 高效计算：优化的Merkle树构建和哈希计算算法
// - 安全可靠：使用成熟的加密哈希算法
// - 灵活扩展：支持不同大小的数据集和树结构
// - 内存优化：合理的内存使用和数据结构设计
//
// 🔗 **组件关系**
// - MerkleTreeManager：被区块、交易、存储等模块使用
// - 与HashManager：依赖哈希计算服务进行Merkle树构建
// - 与BlockService：为区块验证提供Merkle根和证明
package crypto

// MerkleTreeManager 定义Merkle树管理相关接口
//
// 提供WES区块链系统的完整Merkle树管理服务：
// - 树构建：从交易列表构建高效的Merkle树结构
// - 证明系统：Merkle证明路径的生成和验证
// - 根计算：快速准确的Merkle根哈希计算
// - 数据验证：交易存在性和完整性的高效验证
type MerkleTreeManager interface {
	// NewMerkleTree 创建一个新的Merkle树
	// 参数：
	//   - data: 用于构建树的数据切片
	// 返回：构建的Merkle树、错误
	NewMerkleTree(data [][]byte) (MerkleTree, error)

	// Verify 验证数据是否在Merkle树中
	// 参数：
	//   - tree: Merkle树
	//   - data: 要验证的数据
	// 返回：数据是否在树中
	Verify(tree MerkleTree, data []byte) bool

	// VerifyProof 验证Merkle证明
	// 参数：
	//   - tree: Merkle树
	//   - data: 要验证的数据
	//   - proof: Merkle证明(哈希路径)
	//   - rootHash: 根哈希
	// 返回：证明是否有效
	VerifyProof(tree MerkleTree, data []byte, proof [][]byte, rootHash []byte) bool

	// GetProof 生成Merkle证明
	// 参数：
	//   - tree: Merkle树
	//   - data: 要生成证明的数据
	// 返回：Merkle证明(哈希路径)、错误
	GetProof(tree MerkleTree, data []byte) ([][]byte, error)
}

// MerkleTree 定义Merkle树接口
type MerkleTree interface {
	// GetRoot 获取树的根节点哈希
	GetRoot() []byte

	// GetLeaves 获取所有叶子节点哈希
	GetLeaves() [][]byte

	// Verify 验证数据是否在Merkle树中
	Verify(data []byte) bool

	// VerifyProof 验证Merkle证明
	VerifyProof(data []byte, proof [][]byte, rootHash []byte) bool

	// GetProof 生成Merkle证明
	GetProof(data []byte) ([][]byte, error)
}
