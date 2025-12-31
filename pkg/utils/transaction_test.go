// Package utils_test 提供交易工具函数的单元测试
package utils

import (
	"testing"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== IsCoinbaseTx 测试 ====================

func TestIsCoinbaseTx_Scenarios(t *testing.T) {
	tests := []struct {
		name     string
		tx       *transaction.Transaction
		expected bool
		desc     string
	}{
		{
			name: "Coinbase - 无输入+AssetOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Asset{
							Asset: &transaction.AssetOutput{},
						},
					},
				},
			},
			expected: true,
			desc:     "标准Coinbase：无输入且第一个输出是AssetOutput",
		},
		{
			name: "资源部署 - 无输入+ResourceOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: false,
			desc:     "免费资源部署：无输入但输出是ResourceOutput，不应该被识别为Coinbase",
		},
		{
			name: "状态创建 - 无输入+StateOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_State{
							State: &transaction.StateOutput{},
						},
					},
				},
			},
			expected: false,
			desc:     "状态/证据创建：无输入但输出是StateOutput，不应该被识别为Coinbase",
		},
		{
			name: "传统Coinbase - 空引用（PreviousOutput为nil）",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: nil,
					},
				},
			},
			expected: true,
			desc:     "传统Coinbase标识方式：有输入但PreviousOutput为nil",
		},
		{
			name: "传统Coinbase - 空引用（TxId为空且Index为0）",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{},
							OutputIndex: 0,
						},
					},
				},
			},
			expected: true,
			desc:     "传统Coinbase标识方式：PreviousOutput是空引用",
		},
		{
			name: "付费资源部署 - 有输入+ResourceOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{1, 2, 3, 4},
							OutputIndex: 0,
						},
					},
				},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: false,
			desc:     "付费资源部署：有有效输入，不是Coinbase",
		},
		{
			name: "普通转账 - 有输入+AssetOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{1, 2, 3, 4},
							OutputIndex: 1,
						},
					},
				},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Asset{
							Asset: &transaction.AssetOutput{},
						},
					},
				},
			},
			expected: false,
			desc:     "普通转账：有输入有输出，不是Coinbase",
		},
		{
			name:     "空交易",
			tx:       nil,
			expected: false,
			desc:     "nil交易，返回false",
		},
		{
			name: "无效交易 - 无输入无输出",
			tx: &transaction.Transaction{
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			expected: false,
			desc:     "无效交易：无输入也无输出，返回false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCoinbaseTx(tt.tx)
			if result != tt.expected {
				t.Errorf("%s\n期望: %v, 实际: %v\n说明: %s",
					tt.name, tt.expected, result, tt.desc)
			}
		})
	}
}

// ==================== IsResourceDeployTx 测试 ====================

func TestIsResourceDeployTx(t *testing.T) {
	tests := []struct {
		name     string
		tx       *transaction.Transaction
		expected bool
	}{
		{
			name: "免费资源部署",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "付费资源部署",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{1, 2, 3},
							OutputIndex: 0,
						},
					},
				},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Coinbase - AssetOutput",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Asset{
							Asset: &transaction.AssetOutput{},
						},
					},
				},
			},
			expected: false,
		},
		{
			name:     "nil交易",
			tx:       nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResourceDeployTx(tt.tx)
			if result != tt.expected {
				t.Errorf("期望: %v, 实际: %v", tt.expected, result)
			}
		})
	}
}

// ==================== GetTransactionTypeCategory 测试 ====================

func TestGetTransactionTypeCategory(t *testing.T) {
	tests := []struct {
		name     string
		tx       *transaction.Transaction
		expected string
	}{
		{
			name: "Coinbase交易",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Asset{
							Asset: &transaction.AssetOutput{},
						},
					},
				},
			},
			expected: "coinbase",
		},
		{
			name: "资源部署交易",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: "resource_deploy",
		},
		{
			name: "状态创建交易",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_State{
							State: &transaction.StateOutput{},
						},
					},
				},
			},
			expected: "state_create",
		},
		{
			name: "普通转账",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{1, 2, 3},
							OutputIndex: 0,
						},
					},
				},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Asset{
							Asset: &transaction.AssetOutput{},
						},
					},
				},
			},
			expected: "transfer",
		},
		{
			name: "资源转移",
			tx: &transaction.Transaction{
				Inputs: []*transaction.TxInput{
					{
						PreviousOutput: &transaction.OutPoint{
							TxId:        []byte{1, 2, 3},
							OutputIndex: 0,
						},
					},
				},
				Outputs: []*transaction.TxOutput{
					{
						OutputContent: &transaction.TxOutput_Resource{
							Resource: &transaction.ResourceOutput{},
						},
					},
				},
			},
			expected: "resource_transfer",
		},
		{
			name:     "nil交易",
			tx:       nil,
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTransactionTypeCategory(tt.tx)
			if result != tt.expected {
				t.Errorf("期望: %s, 实际: %s", tt.expected, result)
			}
		})
	}
}

// ==================== 边界条件测试 ====================

func TestIsCoinbaseTx_EdgeCases(t *testing.T) {
	t.Run("多个输出但第一个是AssetOutput", func(t *testing.T) {
		tx := &transaction.Transaction{
			Inputs: []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{
				{
					OutputContent: &transaction.TxOutput_Asset{
						Asset: &transaction.AssetOutput{},
					},
				},
				{
					OutputContent: &transaction.TxOutput_Resource{
						Resource: &transaction.ResourceOutput{},
					},
				},
			},
		}
		if !IsCoinbaseTx(tx) {
			t.Error("第一个输出是AssetOutput，应该被识别为Coinbase")
		}
	})

	t.Run("第一个输出为nil", func(t *testing.T) {
		tx := &transaction.Transaction{
			Inputs: []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{
				nil,
			},
		}
		if IsCoinbaseTx(tx) {
			t.Error("第一个输出为nil，不应该被识别为Coinbase")
		}
	})

	t.Run("空引用但Index不为0", func(t *testing.T) {
		tx := &transaction.Transaction{
			Inputs: []*transaction.TxInput{
				{
					PreviousOutput: &transaction.OutPoint{
						TxId:        []byte{},
						OutputIndex: 1, // Index不为0
					},
				},
			},
		}
		if IsCoinbaseTx(tx) {
			t.Error("Index不为0，不应该被识别为Coinbase")
		}
	})
}
