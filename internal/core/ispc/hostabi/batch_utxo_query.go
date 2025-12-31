package hostabi

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// BatchUTXOQueryResult 批量UTXO查询结果
type BatchUTXOQueryResult struct {
	UTXOs map[string]*pb.TxOutput // key: outpoint字符串表示, value: UTXO对象
	Errors map[string]error       // key: outpoint字符串表示, value: 查询错误
}

// BatchUTXOQuerier 批量UTXO查询器
//
// 🎯 **设计目的**：
// 提供批量UTXO查询功能，减少重复的查询操作。
//
// 🏗️ **实现策略**：
// - 批量查询：一次查询多个UTXO
// - 并发查询：使用goroutine并发查询（可选）
// - 结果聚合：返回成功和失败的查询结果
type BatchUTXOQuerier struct {
	eutxoQuery persistence.UTXOQuery
	logger     log.Logger
}

// NewBatchUTXOQuerier 创建批量UTXO查询器
func NewBatchUTXOQuerier(eutxoQuery persistence.UTXOQuery, logger log.Logger) *BatchUTXOQuerier {
	return &BatchUTXOQuerier{
		eutxoQuery: eutxoQuery,
		logger:     logger,
	}
}

// BatchQueryUTXOs 批量查询UTXO
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - outpoints: OutPoint列表
//
// 🔧 **返回值**：
//   - *BatchUTXOQueryResult: 批量查询结果
//   - error: 批量查询失败时的错误信息（如果所有查询都失败）
//
// 🎯 **性能优化**：
//   - 减少重复的查询操作
//   - 可以并发查询多个UTXO（未来优化）
func (b *BatchUTXOQuerier) BatchQueryUTXOs(
	ctx context.Context,
	outpoints []*pb.OutPoint,
) (*BatchUTXOQueryResult, error) {
	if len(outpoints) == 0 {
		return &BatchUTXOQueryResult{
			UTXOs:  make(map[string]*pb.TxOutput),
			Errors: make(map[string]error),
		}, nil
	}

	result := &BatchUTXOQueryResult{
		UTXOs:  make(map[string]*pb.TxOutput, len(outpoints)),
		Errors: make(map[string]error),
	}

	// 批量查询UTXO
	for _, outpoint := range outpoints {
		if outpoint == nil {
			continue
		}

		// 生成outpoint的字符串表示（用于结果映射）
		outpointKey := fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)

		// 查询UTXO
		utxoObj, err := b.eutxoQuery.GetUTXO(ctx, outpoint)
		if err != nil {
			result.Errors[outpointKey] = err
			if b.logger != nil {
				b.logger.Debugf("批量UTXO查询失败: outpoint=%s, error=%v", outpointKey, err)
			}
			continue
		}

		// 从UTXO提取TxOutput
		if utxoObj != nil {
			// 尝试获取缓存的输出
			if cachedOutput := utxoObj.GetCachedOutput(); cachedOutput != nil {
				result.UTXOs[outpointKey] = cachedOutput
			} else {
				// UTXO存在但没有缓存的输出，需要从交易中获取（这里简化处理）
				// 实际实现可能需要调用txQuery.GetTransaction
				result.Errors[outpointKey] = fmt.Errorf("UTXO存在但无法获取输出")
			}
		}
	}

	// 如果所有查询都失败，返回错误
	if len(result.UTXOs) == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("批量UTXO查询全部失败: 共%d个查询，全部失败", len(outpoints))
	}

	return result, nil
}

// BatchQueryUTXOExists 批量查询UTXO是否存在
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - outpoints: OutPoint列表
//
// 🔧 **返回值**：
//   - map[string]bool: key: outpoint字符串表示, value: 是否存在
//   - error: 批量查询失败时的错误信息
//
// 🎯 **性能优化**：
//   - 减少重复的查询操作
//   - 只查询存在性，不返回完整UTXO对象
func (b *BatchUTXOQuerier) BatchQueryUTXOExists(
	ctx context.Context,
	outpoints []*pb.OutPoint,
) (map[string]bool, error) {
	if len(outpoints) == 0 {
		return make(map[string]bool), nil
	}

	result := make(map[string]bool, len(outpoints))

	// 批量查询UTXO存在性
	for _, outpoint := range outpoints {
		if outpoint == nil {
			continue
		}

		// 生成outpoint的字符串表示
		outpointKey := fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)

		// 查询UTXO（只检查是否存在）
		_, err := b.eutxoQuery.GetUTXO(ctx, outpoint)
		if err != nil {
			result[outpointKey] = false
			if b.logger != nil {
				b.logger.Debugf("批量UTXO存在性查询: outpoint=%s, exists=false", outpointKey)
			}
		} else {
			result[outpointKey] = true
		}
	}

	return result, nil
}

