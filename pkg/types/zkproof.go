// Package types provides zero-knowledge proof type definitions.
package types

// ZKProofInput ZK证明输入数据
//
// 该结构体定义了生成零知识证明所需的输入参数。
// 用于ISPC（内在自证明计算）执行结果的零知识证明生成。
//
// 🎯 **使用场景**：
// - 交易执行结果的ZK证明生成
// - 状态转换的零知识验证
// - ISPC单次执行验证语义
//
// 📋 **字段说明**：
// - PublicInputs：公开输入数组，链上可见，用于证明验证
// - PrivateInputs：私有输入数据，保持隐私，不会出现在链上
// - CircuitID：电路标识符，指定使用的ZK电路类型
// - CircuitVersion：电路版本号，用于电路升级管理
type ZKProofInput struct {
	// 公开输入（链上可见）
	// 内容：执行结果哈希、状态哈希、合约地址等
	// 特征：在区块链上公开，用于证明验证
	PublicInputs [][]byte

	// 私有输入（隐私保护）
	// 内容：执行轨迹、中间状态、敏感业务数据等
	// 特征：仅用于证明生成，不会上链
	PrivateInputs interface{}

	// 电路信息
	// 🎯 **电路ID规范**：
	//   - CircuitID: 基础名（不含版本），如 "contract_execution"、"aimodel_inference"
	//   - CircuitVersion: 独立整型版本号，如 1、2
	//   - 展示格式: 可在日志/UI中拼接为 "contract_execution.v1" 便于阅读，但逻辑层统一使用基础名+版本号
	//
	// 示例：
	//   - CircuitID = "contract_execution", CircuitVersion = 1
	//   - CircuitID = "aimodel_inference", CircuitVersion = 1
	CircuitID      string
	CircuitVersion uint32
}

// ZKProofResult ZK证明生成结果
//
// 该结构体包含零知识证明生成的完整结果信息。
// 除了证明本身，还包含验证所需的辅助信息和性能统计。
//
// 🎯 **使用场景**：
// - 作为 GenerateProof 方法的返回值
// - 提供给上层业务进行证明封装
// - 性能监控和优化分析
//
// 📋 **字段说明**：
// - ProofData：序列化的零知识证明数据
// - VKHash：验证密钥哈希，用于确保使用正确的验证密钥
// - ConstraintCount：电路约束数量，反映证明复杂度
// - GenerationTimeMs：证明生成耗时（毫秒）
// - ProofSizeBytes：证明数据大小（字节）
type ZKProofResult struct {
	// 证明数据
	// 内容：根据proving_scheme序列化的证明对象
	// 大小：Groth16约256字节，PlonK约512字节
	ProofData []byte

	// 验证密钥哈希（32字节SHA-256）
	// 用途：验证密钥完整性检查，防止密钥篡改
	// 安全性：确保使用正确的验证密钥进行验证
	VKHash []byte

	// 约束数量
	// 含义：电路中的R1CS或算术约束数量
	// 用途：评估证明复杂度，优化电路设计
	ConstraintCount uint64

	// 性能指标
	// 生成时间（毫秒）
	// 用途：性能监控、瓶颈分析、SLA保障
	GenerationTimeMs uint64

	// 证明大小（字节）
	// 用途：网络传输成本评估、存储优化
	ProofSizeBytes uint64
}
