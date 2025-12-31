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
// 🎯 **判断策略（基于结构推断）**：
//
// **策略1：无输入 + AssetOutput（新式判断）**
//   - 适用于：标准Coinbase、Genesis
//   - 区分原理：Coinbase输出AssetOutput，资源部署输出ResourceOutput
//   - 返回：true表示Coinbase/Genesis，false表示其他创造型交易
//
// **策略2：有输入但空引用（兼容传统）**
//   - 适用于：传统Coinbase标识方式
//   - 检查：PreviousOutput为nil或空引用（TxId为空且Index为0）
//   - 返回：true表示传统Coinbase
//
// 🏗️ **设计理念**：
//   - 基于结构推断，不依赖显式标记（Structure as Type）
//   - 支持未来扩展（付费资源部署、NFT铸造等无输入交易）
//   - 向后兼容历史交易（支持传统空引用标识）
//   - 区分创造型交易（通过输出类型识别交易性质）
//
// 📝 **判断表**：
//
//	| Inputs | 第一个Output类型 | 判断结果 |
//	|--------|-----------------|---------|
//	| []     | AssetOutput     | ✅ Coinbase/Genesis |
//	| []     | ResourceOutput  | ❌ 资源部署 |
//	| []     | ContractOutput  | ❌ 合约部署 |
//	| [空引用] | Any           | ✅ 传统Coinbase |
//	| [正常]  | Any            | ❌ 普通交易 |
func IsCoinbaseTx(tx *transaction.Transaction) bool {
	if tx == nil {
		return false
	}

	// ===== 策略1：无输入判断 =====
	if len(tx.Inputs) == 0 {
		// 必须有输出才能判断
		if len(tx.Outputs) == 0 {
			return false // 无效交易（无输入无输出）
		}

		// 检查第一个输出类型
		if firstOutput := tx.Outputs[0]; firstOutput != nil {
			// Coinbase的输出是AssetOutput（矿工奖励）
			if _, isAsset := firstOutput.OutputContent.(*transaction.TxOutput_Asset); isAsset {
				return true // ✅ Coinbase或Genesis
			}
		}

		// 其他输出类型 = 其他创造型交易（资源部署、NFT铸造等）
		return false
	}

	// ===== 策略2：传统空引用判断（向后兼容）=====
	if len(tx.Inputs) > 0 {
		first := tx.Inputs[0]

		// 检查1：PreviousOutput为nil
		if first.PreviousOutput == nil {
			return true // 传统Coinbase标识
		}

		// 检查2：空引用（TxId为空且OutputIndex为0）
		if len(first.PreviousOutput.TxId) == 0 && first.PreviousOutput.OutputIndex == 0 {
			return true // 传统Coinbase标识
		}
	}

	return false
}

// IsResourceDeployTx 判断是否为资源部署交易
//
// 🎯 **判断逻辑**：
//   - 第一个输出必须是ResourceOutput
//   - 可以有输入（付费部署）或无输入（免费部署）
//   - 区分部署和转移：部署创造新资源，转移消费已有资源
//
// 📝 **场景支持**：
//   - 免费部署：Inputs=[], Outputs=[ResourceOutput]
//   - 付费部署：Inputs=[AssetUTXO], Outputs=[ResourceOutput, ChangeOutput]
//   - 资源升级：Inputs=[OldResourceUTXO], Outputs=[NewResourceOutput]
//
// ⚠️ **注意**：
//   - 当前实现简化判断，仅检查第一个输出是否为ResourceOutput
//   - 未来可细化区分部署/转移/升级等子类型
func IsResourceDeployTx(tx *transaction.Transaction) bool {
	if tx == nil || len(tx.Outputs) == 0 {
		return false
	}

	// 检查第一个输出是否为ResourceOutput
	if firstOutput := tx.Outputs[0]; firstOutput != nil {
		if _, isResource := firstOutput.OutputContent.(*transaction.TxOutput_Resource); isResource {
			return true
		}
	}

	return false
}

// GetTransactionTypeCategory 获取交易类型类别
//
// 🎯 **用途**：
//   - 日志记录：标识交易类型便于调试
//   - 统计分析：按类型统计交易量
//   - 路由选择：根据类型选择不同处理逻辑
//
// 📊 **返回值**：
//   - "coinbase"         - Coinbase奖励交易
//   - "genesis"          - 创世分配交易（无输入+AssetOutput但不是Coinbase）
//   - "resource_deploy"  - 静态资源部署
//   - "contract_deploy"  - 智能合约部署
//   - "transfer"         - 普通转账交易
//   - "invalid"          - 无效交易
//   - "unknown"          - 未知类型
//
// 🏗️ **设计原则**：
//   - 基于结构推断，不依赖显式标记
//   - 分类粒度适中，便于理解和使用
//   - 可扩展，便于添加新类型
func GetTransactionTypeCategory(tx *transaction.Transaction) string {
	if tx == nil {
		return "invalid"
	}

	// 判断Coinbase
	if IsCoinbaseTx(tx) {
		return "coinbase"
	}

	// 判断创造型交易（无输入）
	if len(tx.Inputs) == 0 {
		if len(tx.Outputs) > 0 && tx.Outputs[0] != nil {
			switch tx.Outputs[0].OutputContent.(type) {
			case *transaction.TxOutput_Resource:
				return "resource_deploy"
			case *transaction.TxOutput_State:
				return "state_create" // 证据/状态创建
			case *transaction.TxOutput_Asset:
				// 无输入+AssetOutput但不是Coinbase = Genesis
				return "genesis"
			}
		}
	}

	// 判断转移型交易（有输入）
	if len(tx.Inputs) > 0 {
		// 可以根据第一个输出类型细分
		if len(tx.Outputs) > 0 && tx.Outputs[0] != nil {
			switch tx.Outputs[0].OutputContent.(type) {
			case *transaction.TxOutput_Resource:
				return "resource_transfer" // 资源所有权转移
			case *transaction.TxOutput_State:
				return "state_update" // 状态更新/合约调用
			default:
				return "transfer" // 普通资产转账
			}
		}
	}

	return "unknown"
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
