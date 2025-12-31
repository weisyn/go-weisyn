// Package testutil 提供 ISPC 模块测试的辅助工具
//
// 🧪 **测试数据Fixtures**
//
// 本文件提供测试数据的创建函数，用于简化测试代码编写。

package testutil

import (
	"crypto/rand"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ==================== 测试数据 Fixtures ====================

// RandomBytes 生成随机字节数组
func RandomBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

// RandomAddress 生成随机地址（20 字节）
func RandomAddress() []byte {
	return RandomBytes(20)
}

// RandomPublicKey 生成随机公钥（33 字节，压缩格式）
func RandomPublicKey() []byte {
	return RandomBytes(33)
}

// RandomTxID 生成随机交易 ID（32 字节）
func RandomTxID() []byte {
	return RandomBytes(32)
}

// RandomHash 生成随机哈希（32 字节）
func RandomHash() []byte {
	return RandomBytes(32)
}

// NewTestZKProofInput 创建测试用的ZK证明输入
//
// ✅ **使用场景**：创建标准的ZK证明输入用于测试
func NewTestZKProofInput() *ispcInterfaces.ZKProofInput {
	return &ispcInterfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test_public_input")},
		PrivateInputs: map[string]interface{}{
			"test": "data",
		},
	}
}

// NewTestZKProofInputWithCircuit 创建指定电路的ZK证明输入
func NewTestZKProofInputWithCircuit(circuitID string, circuitVersion uint32) *ispcInterfaces.ZKProofInput {
	return &ispcInterfaces.ZKProofInput{
		CircuitID:      circuitID,
		CircuitVersion: circuitVersion,
		PublicInputs:   [][]byte{[]byte("test_public_input")},
		PrivateInputs: map[string]interface{}{
			"test": "data",
		},
	}
}

// NewTestZKProofInputWithExecutionTrace 创建包含执行轨迹的ZK证明输入
func NewTestZKProofInputWithExecutionTrace(executionTrace []byte) *ispcInterfaces.ZKProofInput {
	return &ispcInterfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test_public_input")},
		PrivateInputs: map[string]interface{}{
			"execution_trace": executionTrace,
			"state_diff":      []byte("test_state_diff"),
		},
	}
}

// NewTestTime 创建测试用的时间点
func NewTestTime() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

// NewTestTimeWithOffset 创建带偏移的测试时间
func NewTestTimeWithOffset(offset time.Duration) time.Time {
	return NewTestTime().Add(offset)
}

