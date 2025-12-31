package interfaces

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ZKProofInput ZK证明输入数据
type ZKProofInput struct {
	// 公开输入（链上可见）
	PublicInputs [][]byte

	// 私有输入（隐私保护）
	PrivateInputs interface{}

	// 电路信息
	CircuitID      string
	CircuitVersion uint32
}

// ZKProofResult ZK证明生成结果
type ZKProofResult struct {
	// 证明数据
	ProofData []byte

	// 验证密钥哈希
	VKHash []byte

	// 约束数量
	ConstraintCount uint64

	// 性能指标
	GenerationTimeMs uint64
	ProofSizeBytes   uint64
}

// ZKProofManager 零知识证明管理器接口
//
// 负责ISPC执行结果的零知识证明生成和验证
// 支持Groth16和PlonK两种证明方案
type ZKProofManager interface {
	// ==================== 核心证明生成方法 ====================

	// GenerateProof 生成零知识证明
	// 基于执行结果和轨迹生成ZK证明
	GenerateProof(ctx context.Context, input *ZKProofInput) (*ZKProofResult, error)

	// GenerateStateProof 生成状态证明
	// 专门为StateOutput生成ZKStateProof
	GenerateStateProof(ctx context.Context, input *ZKProofInput) (*transaction.ZKStateProof, error)

	// ==================== 证明验证方法 ====================

	// ValidateProof 验证零知识证明
	// 验证ZK证明的有效性和正确性
	ValidateProof(ctx context.Context, proof *transaction.ZKStateProof) (bool, error)

	// ==================== 电路管理方法 ====================

	// LoadCircuit 加载证明电路
	// 加载指定的ZK电路定义
	LoadCircuit(circuitID string, circuitVersion uint32) error

	// IsCircuitLoaded 检查电路是否已加载
	// 检查指定电路是否可用
	IsCircuitLoaded(circuitID string) bool
}
