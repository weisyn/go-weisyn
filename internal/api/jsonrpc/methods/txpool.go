package methods

import (
	"context"
	"encoding/json"
	"fmt"

	cryptoInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"go.uber.org/zap"
)

// TxPoolMethods 交易池相关方法
type TxPoolMethods struct {
	logger         *zap.Logger
	pool           mempool.TxPool
	addressManager cryptoInterface.AddressManager // 地址管理器，用于验证Base58格式地址
}

// NewTxPoolMethods 创建交易池方法处理器
func NewTxPoolMethods(
	logger *zap.Logger,
	pool mempool.TxPool,
	addressManager cryptoInterface.AddressManager,
) *TxPoolMethods {
	return &TxPoolMethods{
		logger:         logger,
		pool:           pool,
		addressManager: addressManager,
	}
}

// TxPoolStatus 查询交易池状态
// Method: wes_txpool_status
// Params: []
func (m *TxPoolMethods) TxPoolStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.pool == nil {
		return nil, NewInternalError("txpool not available", nil)
	}

	// 获取待处理交易
	pendingTxs, err := m.pool.GetPendingTransactions()
	if err != nil {
		m.logger.Error("Failed to get pending transactions", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	return map[string]interface{}{
		"pending": len(pendingTxs),
		"queued":  0, // 可选：后续扩展queued统计
	}, nil
}

// TxPoolContent 查询交易池内容
// Method: wes_txpool_content
// Params: []
func (m *TxPoolMethods) TxPoolContent(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.pool == nil {
		return nil, NewInternalError("txpool not available", nil)
	}

	// 获取待处理交易
	pendingTxs, err := m.pool.GetPendingTransactions()
	if err != nil {
		m.logger.Error("Failed to get pending transactions", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	// 简化：返回交易数量和总结信息
	// 完整的地址分组需要计算交易哈希或从输入推导发送者，暂简化
	pendingList := make([]interface{}, 0, len(pendingTxs))
	for _, tx := range pendingTxs {
		if tx == nil {
			continue
		}
		// 简化信息：只显示输入输出数量
		txInfo := map[string]interface{}{
			"version":    tx.Version,
			"numInputs":  len(tx.Inputs),
			"numOutputs": len(tx.Outputs),
		}
		pendingList = append(pendingList, txInfo)
	}

	return map[string]interface{}{
		"pending": pendingList,
		"queued":  []interface{}{},
	}, nil
}

// TxPoolInspect 查询特定地址的待处理交易
// Method: wes_txpool_inspect
// Params: [address: string]
// address: Base58格式的WES地址（如CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
func (m *TxPoolMethods) TxPoolInspect(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("missing address", nil)
	}

	addressStr := args[0]

	// 验证并转换Base58格式地址
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}

	// 拒绝0x前缀的ETH地址格式
	if len(addressStr) > 2 && (addressStr[:2] == "0x" || addressStr[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}

	// 验证Base58格式地址
	validAddress, err := m.addressManager.StringToAddress(addressStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid address format: %v", err), nil)
	}

	// 转换为字节数组
	address, err := m.addressManager.AddressToBytes(validAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("failed to convert address: %v", err), nil)
	}

	if m.pool == nil {
		return nil, NewInternalError("txpool not available", nil)
	}

	// 获取待处理交易并筛选该地址的交易
	pendingTxs, err := m.pool.GetPendingTransactions()
	if err != nil {
		m.logger.Error("Failed to get pending transactions", zap.Error(err))
		return nil, NewInternalError(err.Error(), nil)
	}

	// 筛选属于该地址的交易
	// 🔍 地址匹配策略（当前为简化实现）：
	//
	// WES 中交易输入使用 LockingCondition，不直接包含发送者地址。
	// 完整的 sender 推导需要：
	// 1. 通过 input.PreviousOutput 查询 UTXO
	// 2. 从 UTXO 的 LockingCondition 中提取地址
	// 3. 或从 UnlockingProof 中恢复公钥并派生地址
	//
	// 🚧 当前实现：粗略匹配解锁证明中的公钥字节
	// 🎯 后续优化方向：
	// - 引入 pkg/interfaces/tx 的 sender 推导接口
	// - 使用 crypto.AddressManager 规范化地址派生
	// - 支持多种解锁证明类型（MultiKey、Delegation、Threshold 等）
	// - 建立 txpool 索引加速地址查询
	matchedTxs := make([]interface{}, 0)
	for _, tx := range pendingTxs {
		if tx == nil || len(tx.Inputs) == 0 {
			continue
		}

		// 检查输入的解锁证明（简化版）
		isMatch := false
		for _, input := range tx.Inputs {
			if input == nil || input.PreviousOutput == nil {
				continue
			}

			// 🔍 策略1：检查单密钥证明
			if singleKey := input.GetSingleKeyProof(); singleKey != nil && singleKey.PublicKey != nil {
				// 粗略匹配：比较公钥字节前缀
				// TODO: 替换为规范化地址派生（PublicKey -> Address）
				pubKeyBytes := singleKey.PublicKey.Value
				if len(pubKeyBytes) >= len(address) &&
					string(pubKeyBytes[:len(address)]) == string(address) {
					isMatch = true
					break
				}
			}

			// 🔍 策略2：检查多密钥证明（扩展点）
			// if multiKey := input.GetMultiKeyProof(); multiKey != nil {
			//     // TODO: 检查多个公钥是否包含目标地址
			// }

			// 🔍 策略3：检查委托证明（扩展点）
			// if delegation := input.GetDelegationProof(); delegation != nil {
			//     // TODO: 检查委托者或被委托者地址
			// }
		}

		if isMatch {
			txInfo := map[string]interface{}{
				"version":    tx.Version,
				"numInputs":  len(tx.Inputs),
				"numOutputs": len(tx.Outputs),
			}
			matchedTxs = append(matchedTxs, txInfo)
		}
	}

	return map[string]interface{}{
		"address":     validAddress, // 返回Base58格式地址
		"pending":     len(matchedTxs),
		"queued":      0,
		"txCount":     len(matchedTxs),
		"totalInPool": len(pendingTxs),
	}, nil
}
