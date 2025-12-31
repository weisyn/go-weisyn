// Package hostabi 实现 ISPC HostABI（引擎无关宿主能力接口）
//
// 本文件：账户查询与交易查询
// 提供账户余额查询、交易详情查询等只读接口（账户抽象设计）。
// 委托给 UTXOManager 和 RepositoryManager 实现，隐藏 UTXO 技术细节。
package hostabi

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/utils"
)

// ==================== 账户查询（账户抽象设计）====================

// GetBalance 查询账户余额
//
// 参数：
//   - ctx: 上下文对象
//   - address: 账户地址（20字节）
//   - tokenID: 代币标识（nil=原生币，非nil=指定代币）
//
// 返回值：
//   - uint64: 账户可用余额（单位：最小单位）
//   - error: 查询失败时的错误信息
//
// 说明：
//   - 查询指定地址的可用余额（底层自动聚合资产，开发者无需关心实现）
//   - tokenID=nil：查询原生币余额
//   - tokenID!=nil：查询指定代币余额
//   - 简单直观，符合"我有多少钱？"的自然认知
//   - 性能优化：底层使用高效聚合查询，无需获取完整资产列表
func (h *HostRuntimePorts) GetBalance(ctx context.Context, address []byte, tokenID []byte) (uint64, error) {
	if h.eutxoQuery == nil {
		return 0, fmt.Errorf("eutxoQuery 未初始化")
	}

	// 底层实现：通过 UTXOQuery 聚合资产（技术细节，不暴露给合约开发者）
	utxoList, err := h.eutxoQuery.GetUTXOsByAddress(ctx, address, nil, true)
	if err != nil {
		if h.logger != nil {
			h.logger.Debugf("查询余额失败: address=%x, tokenID=%x, err=%v", address, tokenID, err)
		}
		return 0, fmt.Errorf("查询余额失败: %w", err)
	}

	var totalBalance uint64
	for _, utxoItem := range utxoList {
		if utxoItem == nil {
			continue
		}

		// 只处理Asset类型的UTXO
		if utxoItem.Category != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 获取缓存的TxOutput
		cachedOutput := utxoItem.GetCachedOutput()
		if cachedOutput == nil {
			continue
		}

		// 获取AssetOutput
		assetOutput := cachedOutput.GetAsset()
		if assetOutput == nil {
			continue
		}

		// 根据tokenID匹配逻辑
		var amountStr string
		if len(tokenID) == 0 {
			// 查询原生币
			nativeCoin := assetOutput.GetNativeCoin()
			if nativeCoin == nil {
				continue // 跳过非原生币
			}
			amountStr = nativeCoin.Amount
		} else {
			// 查询合约代币
			contractToken := assetOutput.GetContractToken()
			if contractToken == nil {
				continue // 跳过非合约代币
			}
			// 比较合约地址（tokenID应该是合约地址）
			if len(contractToken.ContractAddress) != len(tokenID) {
				continue
			}
			match := true
			for i := range len(tokenID) {
				if contractToken.ContractAddress[i] != tokenID[i] {
					match = false
					break
				}
			}
			if !match {
				continue // 跳过不匹配的代币
			}
			amountStr = contractToken.Amount
		}

		// 解析金额（使用安全的解析函数）
		if amountStr != "" {
			amount, err := utils.ParseAmountSafely(amountStr)
			if err != nil {
				if h.logger != nil {
					h.logger.Warnf("解析UTXO金额失败: utxo=%x:%d, err=%v", utxoItem.Outpoint.TxId[:8], utxoItem.Outpoint.OutputIndex, err)
				}
				continue // 跳过解析失败的UTXO
			}
			totalBalance += amount
		}
	}

	if h.logger != nil {
		if tokenID == nil {
			h.logger.Debugf("✅ 查询余额成功（原生币）: address=%x, balance=%d", address, totalBalance)
		} else {
			h.logger.Debugf("✅ 查询余额成功（代币）: address=%x, tokenID=%x, balance=%d", address, tokenID, totalBalance)
		}
	}

	return totalBalance, nil
}

// GetTransaction 查询交易详情
//
// 参数：
//   - ctx: 上下文对象
//   - txID: 交易ID（32字节 SHA-256 哈希）
//
// 返回值：
//   - *pb.Transaction: 交易详情（Protobuf 结构）
//   - uint64: 交易所在区块高度（0=未确认）
//   - bool: 交易是否已确认（true=已确认，false=未确认或在交易池）
//   - error: 查询失败时的错误信息
//
// 说明：
//   - 通过 RepositoryManager 查询交易及其确认状态
//   - 用于交易追踪、依赖验证等场景
//   - 实现步骤：
//     1. 调用 repoManager.GetTransaction(txID) 获取交易和区块信息
//     2. 调用 repoManager.GetTxBlockHeight(txID) 获取区块高度
//     3. 判断确认状态：height > 0 表示已确认
//   - 未确认交易（在交易池中）返回 height=0, confirmed=false
func (h *HostRuntimePorts) GetTransaction(ctx context.Context, txID []byte) (*pb.Transaction, uint64, bool, error) {
	if h.txQuery == nil {
		return nil, 0, false, fmt.Errorf("txQuery 未初始化")
	}
	blockHash, txIndex, tx, err := h.txQuery.GetTransaction(ctx, txID)
	if err != nil {
		if h.logger != nil {
			h.logger.Debugf("GetTransaction 失败: txID=%x, err=%v", txID, err)
		}
		return nil, 0, false, fmt.Errorf("查询交易失败: %w", err)
	}
	height, err := h.txQuery.GetTxBlockHeight(ctx, txID)
	if err != nil {
		if h.logger != nil {
			h.logger.Debugf("GetTxBlockHeight 失败: txID=%x, err=%v", txID, err)
		}
		height = 0
		confirmed := false
		if h.logger != nil {
			_ = confirmed
		}
	} else {
		confirmed := height > 0
		if h.logger != nil {
			_ = confirmed
		}
	}
	if h.logger != nil {
		h.logger.Debugf("✅ GetTransaction 成功: txID=%x, blockHash=%x, txIndex=%d, height=%d, confirmed=%v", txID, blockHash, txIndex, height, height > 0)
	}
	return tx, height, height > 0, nil
}
