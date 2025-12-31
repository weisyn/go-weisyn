// Package testutil 提供 EUTXO 模块测试的辅助工具
//
// 🧪 **测试数据 Fixtures**
//
// 本文件提供测试数据的创建函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"crypto/rand"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 测试数据创建函数 ====================

// RandomBytes 生成指定长度的随机字节
func RandomBytes(length int) []byte {
	data := make([]byte, length)
	_, _ = rand.Read(data)
	return data
}

// RandomTxID 生成随机交易ID（32字节）
func RandomTxID() []byte {
	return RandomBytes(32)
}

// RandomAddress 生成随机地址（20字节）
func RandomAddress() []byte {
	return RandomBytes(20)
}

// CreateOutPoint 创建测试用的 OutPoint
func CreateOutPoint(txID []byte, index uint32) *transaction.OutPoint {
	if txID == nil {
		txID = RandomTxID()
	}
	return &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: index,
	}
}

// CreateUTXO 创建测试用的 UTXO
//
// 参数：
//   - outpoint: UTXO 的输出点（nil 时自动生成）
//   - ownerAddress: 所有者地址（nil 时自动生成）
//   - category: UTXO 类别（nil 时使用 ASSET）
//
// 返回：
//   - *utxo.UTXO: UTXO 对象
func CreateUTXO(outpoint *transaction.OutPoint, ownerAddress []byte, category *utxo.UTXOCategory) *utxo.UTXO {
	if outpoint == nil {
		outpoint = CreateOutPoint(nil, 0)
	}
	if ownerAddress == nil {
		ownerAddress = RandomAddress()
	}
	if category == nil {
		cat := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
		category = &cat
	}

	utxoObj := &utxo.UTXO{
		Outpoint:     outpoint,
		OwnerAddress: ownerAddress,
		Category:     *category,
		Status:       utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
		BlockHeight:  1, // 默认高度为1
	}

	// 根据类别设置对应的约束（简化实现，不设置 CachedOutput）
	switch *category {
	case utxo.UTXOCategory_UTXO_CATEGORY_ASSET:
		utxoObj.TypeSpecificConstraints = &utxo.UTXO_AssetConstraints{
			AssetConstraints: &utxo.AssetUTXOConstraints{},
		}
	case utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE:
		utxoObj.TypeSpecificConstraints = &utxo.UTXO_ResourceConstraints{
			ResourceConstraints: &utxo.ResourceUTXOConstraints{
				ReferenceCount: 0,
			},
		}
	case utxo.UTXOCategory_UTXO_CATEGORY_STATE:
		utxoObj.TypeSpecificConstraints = &utxo.UTXO_StateConstraints{
			StateConstraints: &utxo.StateUTXOConstraints{},
		}
	}

	return utxoObj
}

// CreateAssetUTXO 创建资产 UTXO
func CreateAssetUTXO(outpoint *transaction.OutPoint, ownerAddress []byte, amount uint64) *utxo.UTXO {
	cat := utxo.UTXOCategory_UTXO_CATEGORY_ASSET
	return CreateUTXO(outpoint, ownerAddress, &cat)
}

// CreateResourceUTXO 创建资源 UTXO
func CreateResourceUTXO(outpoint *transaction.OutPoint, ownerAddress []byte, resourceID []byte) *utxo.UTXO {
	cat := utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE
	return CreateUTXO(outpoint, ownerAddress, &cat)
}

// CreateStateUTXO 创建状态 UTXO
func CreateStateUTXO(outpoint *transaction.OutPoint, ownerAddress []byte, stateData []byte) *utxo.UTXO {
	cat := utxo.UTXOCategory_UTXO_CATEGORY_STATE
	return CreateUTXO(outpoint, ownerAddress, &cat)
}

// CreateUTXOSnapshotData 创建测试用的快照数据
func CreateUTXOSnapshotData(snapshotID string, height uint64, stateRoot []byte) *types.UTXOSnapshotData {
	if snapshotID == "" {
		snapshotID = fmt.Sprintf("snapshot-%d", height)
	}
	if stateRoot == nil {
		stateRoot = RandomBytes(32)
	}
	return &types.UTXOSnapshotData{
		SnapshotID: snapshotID,
		Height:     height,
		StateRoot:  stateRoot,
	}
}

