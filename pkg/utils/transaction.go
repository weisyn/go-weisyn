// Package utils 提供跨组件共享的交易相关工具函数
//
// 🎯 **交易工具函数集合**
//
// 本文件提供与交易相关的通用工具函数，可被任何组件安全使用：
// - UTXO键生成和管理
// - Coinbase交易识别
// - OutPoint比较和处理
//
// 这些函数提供统一的交易处理工具，避免跨组件直接依赖和重复实现。
package utils

import (
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== UTXO键管理工具 ====================

// UTXOKey 生成标准化的UTXO键
//
// 📝 **UTXO键标准**：
// 使用 "txid:index" 格式，其中 txid 为十六进制字符串
//
// 🎯 **统一UTXO键生成规范**：
// 避免跨组件依赖，提供统一的UTXO键生成标准
func UTXOKey(txid []byte, index uint32) string {
	return fmt.Sprintf("%x:%d", txid, index)
}

// OutPointKey 从 OutPoint 生成标准化键
//
// 📝 **OutPoint键标准**：
// 统一 OutPoint 到字符串的转换格式
func OutPointKey(op *transaction.OutPoint) string {
	if op == nil {
		return ""
	}
	return fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
}

// EqualOutPoint 比较两个 OutPoint 是否相等
//
// 🎯 **精确比较**：
// 逐字节比较 TxId 和 OutputIndex，确保完全一致
func EqualOutPoint(a, b *transaction.OutPoint) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.OutputIndex != b.OutputIndex {
		return false
	}
	if len(a.TxId) != len(b.TxId) {
		return false
	}
	for i := range a.TxId {
		if a.TxId[i] != b.TxId[i] {
			return false
		}
	}
	return true
}

// OutPointRefBytes 生成规范化的 OutPoint 字节引用
//
// 📝 **字节格式**：
// txid || index (4字节大端序)
func OutPointRefBytes(txid []byte, index uint32) []byte {
	ref := make([]byte, len(txid)+4)
	copy(ref, txid)
	ref[len(txid)] = byte(index >> 24)
	ref[len(txid)+1] = byte(index >> 16)
	ref[len(txid)+2] = byte(index >> 8)
	ref[len(txid)+3] = byte(index)
	return ref
}

// ==================== Coinbase交易识别工具 ====================

// IsCoinbaseTx 判断交易是否为Coinbase交易
//
// 🔍 **Coinbase识别规则**：
// 1. 无输入（len(tx.Inputs) == 0）
// 2. 第一个输入的 PreviousOutput 为 nil
// 3. 第一个输入的 PreviousOutput 为空引用（txid空且index为0）
//
// 🎯 **统一Coinbase识别标准**：
// 提供统一的Coinbase识别标准，避免各组件重复实现
func IsCoinbaseTx(tx *transaction.Transaction) bool {
	if tx == nil {
		return false
	}
	if len(tx.Inputs) == 0 {
		return true
	}
	first := tx.Inputs[0]
	if first.PreviousOutput == nil {
		return true
	}
	if len(first.PreviousOutput.TxId) == 0 && first.PreviousOutput.OutputIndex == 0 {
		return true
	}
	return false
}

// ==================== 交易验证辅助工具 ====================

// HasUTXOConflict 检查两个交易是否存在UTXO冲突
//
// 🔍 **冲突检测逻辑**：
// 比较两个交易的所有输入，如果存在相同的OutPoint引用则为冲突
//
// 🎯 **使用场景**：
// - 交易池冲突检测
// - 区块验证中的重复花费检查
func HasUTXOConflict(tx1, tx2 *transaction.Transaction) bool {
	if tx1 == nil || tx2 == nil {
		return false
	}

	// Coinbase交易不参与UTXO冲突检测
	if IsCoinbaseTx(tx1) || IsCoinbaseTx(tx2) {
		return false
	}

	// 比较所有输入组合
	for _, input1 := range tx1.Inputs {
		for _, input2 := range tx2.Inputs {
			if input1.PreviousOutput == nil || input2.PreviousOutput == nil {
				continue
			}
			// 相同的OutPoint表示冲突
			if EqualOutPoint(input1.PreviousOutput, input2.PreviousOutput) {
				return true
			}
		}
	}

	return false
}

// GetTransactionInputKeys 获取交易所有输入的UTXO键
//
// 🎯 **批量键提取**：
// 返回交易所有输入对应的UTXO键，用于批量查询或索引
func GetTransactionInputKeys(tx *transaction.Transaction) []string {
	if tx == nil || IsCoinbaseTx(tx) {
		return nil
	}

	keys := make([]string, 0, len(tx.Inputs))
	for _, input := range tx.Inputs {
		if input.PreviousOutput != nil {
			key := UTXOKey(input.PreviousOutput.TxId, input.PreviousOutput.OutputIndex)
			keys = append(keys, key)
		}
	}

	return keys
}
