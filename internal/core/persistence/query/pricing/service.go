// Package pricing 实现定价查询服务
package pricing

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// Service 定价查询服务
type Service struct {
	storage     storage.BadgerStore
	txQuery     interfaces.InternalTxQuery
	resourceQuery interfaces.InternalResourceQuery
	logger      log.Logger
}

// NewService 创建定价查询服务
func NewService(
	badgerStore storage.BadgerStore,
	txQuery interfaces.InternalTxQuery,
	resourceQuery interfaces.InternalResourceQuery,
	logger log.Logger,
) (interfaces.InternalPricingQuery, error) {
	if badgerStore == nil {
		return nil, fmt.Errorf("badgerStore 不能为空")
	}
	if txQuery == nil {
		return nil, fmt.Errorf("txQuery 不能为空")
	}
	if resourceQuery == nil {
		return nil, fmt.Errorf("resourceQuery 不能为空")
	}

	s := &Service{
		storage:       badgerStore,
		txQuery:       txQuery,
		resourceQuery: resourceQuery,
		logger:        logger,
	}

	if logger != nil {
		logger.Info("✅ PricingQuery 服务已创建")
	}

	return s, nil
}

// GetPricingState 根据资源哈希查询定价状态
//
// 🎯 **查询流程**：
// 1. 优先查询本地 KV 索引（快速路径）
// 2. 如果索引不存在，回溯部署交易查找 StateOutput（慢速路径）
// 3. 找到后更新索引，加速后续查询
func (s *Service) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	if len(resourceHash) != 32 {
		return nil, fmt.Errorf("资源哈希必须是 32 字节，实际: %d", len(resourceHash))
	}

	// ========== 1. 查询本地 KV 索引（快速路径）==========
	indexKey := s.buildPricingIndexKey(resourceHash)
	pricingStateBytes, err := s.storage.Get(ctx, indexKey)
	if err == nil && len(pricingStateBytes) > 0 {
		// 索引存在，直接解析
		pricingState, err := types.DecodePricingState(pricingStateBytes)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("解析定价状态索引失败，将回溯查询: %v", err)
			}
			// 索引损坏，继续回溯查询
		} else {
			// 验证资源哈希是否匹配
			if len(pricingState.ResourceHash) == 32 {
				match := true
				for i := 0; i < 32; i++ {
					if pricingState.ResourceHash[i] != resourceHash[i] {
						match = false
						break
					}
				}
				if match {
					if s.logger != nil {
						s.logger.Debugf("✅ 从索引获取定价状态，resourceHash=%x", resourceHash)
					}
					return pricingState, nil
				}
			}
		}
	}

	// ========== 2. 回溯查询部署交易（慢速路径）==========
	if s.logger != nil {
		s.logger.Infof("🔍 索引不存在，回溯查询定价状态，resourceHash=%x", resourceHash)
	}

	// 2.1 获取资源关联的交易信息
	txHash, _, _, err := s.resourceQuery.GetResourceTransaction(ctx, resourceHash)
	if err != nil {
		return nil, fmt.Errorf("资源不存在或未找到部署交易: %w", err)
	}

	// 2.2 查询完整交易
	_, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("获取部署交易失败: %w", err)
	}

	// 2.3 遍历交易输出，查找定价状态 StateOutput
	for _, output := range tx.Outputs {
		if output == nil {
			continue
		}

		stateOutput := output.GetState()
		if stateOutput == nil {
			continue
		}

		// 检查是否是定价状态（通过 metadata 中的 pricing_type 判断）
		if stateOutput.Metadata == nil {
			continue
		}

		pricingType, ok := stateOutput.Metadata["pricing_type"]
		if !ok || pricingType != "resource_pricing" {
			continue
		}

		// 验证 resource_hash 是否匹配
		resourceHashHex, ok := stateOutput.Metadata["resource_hash"]
		if !ok {
			continue
		}

		expectedHashHex := hex.EncodeToString(resourceHash)
		if resourceHashHex != expectedHashHex {
			continue
		}

		// 提取定价状态 JSON
		pricingStateJSON, ok := stateOutput.Metadata["pricing_state"]
		if !ok || pricingStateJSON == "" {
			continue
		}

				// 解析定价状态
				pricingState, err := types.DecodePricingState([]byte(pricingStateJSON))
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("解析定价状态失败: %v", err)
			}
			continue
		}

		// 验证定价状态
		if err := pricingState.Validate(); err != nil {
			if s.logger != nil {
				s.logger.Warnf("定价状态验证失败: %v", err)
			}
			continue
		}

		// ========== 3. 更新本地索引 ==========
		pricingStateBytes, err := pricingState.Encode()
		if err == nil {
			// 异步更新索引（不阻塞查询）
			go func() {
				updateCtx := context.Background()
				if err := s.storage.Set(updateCtx, indexKey, pricingStateBytes); err != nil {
					if s.logger != nil {
						s.logger.Warnf("更新定价状态索引失败: %v", err)
					}
				} else if s.logger != nil {
					s.logger.Debugf("✅ 定价状态索引已更新，resourceHash=%x", resourceHash)
				}
			}()
		}

		if s.logger != nil {
			s.logger.Infof("✅ 从部署交易获取定价状态，resourceHash=%x, txHash=%x", resourceHash, txHash)
		}

		return pricingState, nil
	}

	// 未找到定价状态
	return nil, fmt.Errorf("资源 %x 未配置定价状态", resourceHash)
}

// buildPricingIndexKey 构建定价状态索引键
//
// 键格式：indices:pricing:{resourceHash}
func (s *Service) buildPricingIndexKey(resourceHash []byte) []byte {
	return []byte(fmt.Sprintf("indices:pricing:%x", resourceHash))
}

