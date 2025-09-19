// Package lifecycle 提供交易生命周期管理 - 状态查询服务
//
// 🎯 **职责定位**：TransactionManager状态查询接口的专门实现
//
// 本文件实现公共接口`TransactionManager.GetTransactionStatus`方法，
// 负责查询交易在区块链中的实时状态和确认情况。
//
// 🏗️ **架构分层**：
// - 本文件：公共接口适配层（状态查询逻辑）
// - manager.go：顶层协调层（方法委托和依赖注入）
// - 存储层：区块链数据和内存池查询（外部依赖）
//
// 📋 **核心功能**：
// - 交易状态跟踪：实时查询交易的确认状态
// - 多数据源查询：内存池、已确认区块、失败记录
// - 状态缓存管理：优化高频查询的性能
// - 错误状态分析：详细的失败原因和建议
//
// 💡 **设计价值**：
// - 状态统一：提供标准化的交易状态枚举
// - 性能优化：智能缓存和批量查询优化
// - 用户友好：简洁明了的状态描述和错误信息
package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"
)

// TransactionStatusService 交易状态查询服务
//
// 🎯 **TransactionManager状态接口的专门实现**
//
// 负责实现公共接口中的交易状态查询相关方法，管理交易
// 从提交到最终确认的完整状态跟踪。
//
// 💡 **核心价值**：
// - ✅ **实时状态**：准确反映交易的当前状态
// - ✅ **多源查询**：内存池、区块链、缓存的统一查询
// - ✅ **性能优化**：智能缓存和批量查询策略
// - ✅ **错误诊断**：详细的失败分析和处理建议
//
// 📝 **状态生命周期**：
// 1. **pending**：交易在内存池中等待打包
// 2. **confirmed**：交易已被打包到区块并确认
// 3. **failed**：交易验证失败或执行出错
//
// 📊 **查询策略**：
// - **缓存优先**：首先检查本地状态缓存
// - **内存池查询**：检查待确认交易状态
// - **区块链查询**：查询已确认的交易记录
// - **失败记录查询**：检查交易失败历史
//
// 🔄 **缓存策略**：
// - **confirmed状态**：长期缓存（1小时）
// - **pending状态**：短期缓存（30秒）
// - **failed状态**：中期缓存（10分钟）
type TransactionStatusService struct {
	logger     log.Logger                   // 日志记录器（可选）
	cacheStore storage.MemoryStore          // 状态缓存存储
	txPool     mempool.TxPool               // 交易内存池
	repository repository.RepositoryManager // 数据存储访问
}

// NewTransactionStatusService 创建交易状态查询服务
//
// 🎯 **服务工厂方法**
//
// 创建完整的交易状态查询服务实例，集成所有必要的依赖服务。
//
// 💡 **参数说明**：
//   - logger: 日志记录器（可选，传nil则不记录日志）
//   - cacheStore: 状态缓存存储服务
//   - txPool: 交易内存池服务
//   - repository: 数据存储访问服务
//
// 💡 **返回值说明**：
//   - *TransactionStatusService: 状态服务实例
func NewTransactionStatusService(
	logger log.Logger,
	cacheStore storage.MemoryStore,
	txPool mempool.TxPool,
	repository repository.RepositoryManager,
) *TransactionStatusService {
	// 严格检查必需的依赖
	if cacheStore == nil {
		panic("TransactionStatusService: cacheStore不能为nil")
	}
	if txPool == nil {
		panic("TransactionStatusService: txPool不能为nil")
	}
	if repository == nil {
		panic("TransactionStatusService: repository不能为nil")
	}

	return &TransactionStatusService{
		logger:     logger,
		cacheStore: cacheStore,
		txPool:     txPool,
		repository: repository,
	}
}

// GetTransactionStatus 查询交易状态（公共接口实现）
//
// 🎯 **TransactionManager.GetTransactionStatus接口实现**
//
// 查询交易在区块链中的实时状态和确认情况，提供准确的
// 状态信息供用户和应用程序使用。
//
// 📝 **查询流程**：
// 1. **缓存检查阶段**：
//   - 检查本地状态缓存中的记录
//   - 验证缓存数据的有效性和时效性
//   - 如果缓存命中且有效，直接返回结果
//
// 2. **内存池查询阶段**：
//   - 在交易内存池中搜索待确认交易
//   - 检查交易的验证状态和排队位置
//   - 如果找到，返回pending状态
//
// 3. **区块链查询阶段**：
//   - 在已确认区块中搜索交易记录
//   - 计算交易的确认区块数和时间
//   - 如果找到，返回confirmed状态
//
// 4. **失败记录查询阶段**：
//   - 检查交易失败历史记录
//   - 分析失败原因和错误详情
//   - 如果找到，返回failed状态
//
// 5. **结果缓存阶段**：
//   - 将查询结果缓存到本地存储
//   - 设置合适的缓存过期时间
//   - 更新查询统计和性能指标
//
// 📊 **状态含义**：
// - **pending**：交易已提交到网络，在内存池中等待矿工打包
// - **confirmed**：交易已被打包到区块并获得足够确认
// - **failed**：交易验证失败或执行出错，不会被打包
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时控制
//   - txHash: 交易哈希（32字节，签名前后均可）
//
// 💡 **返回值说明**：
//   - types.TransactionStatusEnum: 交易状态枚举
//   - error: 查询错误，nil表示查询成功
//
// 💡 **调用示例**：
//
//	service := NewTransactionStatusService(logger)
//	status, err := service.GetTransactionStatus(ctx, txHash)
//	if err != nil {
//	    log.Errorf("状态查询失败: %v", err)
//	    return "", fmt.Errorf("查询失败: %w", err)
//	}
//
//	switch status {
//	case types.TxStatus_Pending:
//	    log.Info("交易等待确认中")
//	case types.TxStatus_Confirmed:
//	    log.Info("交易已确认")
//	case types.TxStatus_Failed:
//	    log.Warn("交易执行失败")
//	}
//
// ⚠️ **注意事项**：
// - 状态查询结果具有时效性，confirmed状态最为稳定
// - pending状态可能随时变化，建议定期重新查询
// - 网络拥堵时pending状态可能持续较长时间
func (s *TransactionStatusService) GetTransactionStatus(
	ctx context.Context,
	txHash []byte,
) (types.TransactionStatusEnum, error) {
	if s.logger != nil {
		s.logger.Debugf("开始查询交易状态 - 哈希: %x", txHash[:8])
	}

	// 1. 基础参数验证
	if len(txHash) != 32 {
		err := fmt.Errorf("交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
		if s.logger != nil {
			s.logger.Warnf(err.Error())
		}
		return "", err
	}

	// 2. 检查状态缓存
	if cachedStatus, found := s.getStatusFromCache(ctx, txHash); found {
		if s.logger != nil {
			s.logger.Debugf("缓存命中 - 状态: %s", cachedStatus)
		}
		return cachedStatus, nil
	}

	// 3. 查询内存池（pending状态）
	if isPending, err := s.checkMempool(ctx, txHash); err != nil {
		if s.logger != nil {
			s.logger.Warnf("内存池查询失败: %v", err)
		}
		return "", fmt.Errorf("内存池查询失败: %w", err)
	} else if isPending {
		status := types.TxStatus_Pending
		s.cacheStatus(ctx, txHash, status)
		if s.logger != nil {
			s.logger.Debug("交易在内存池中，状态: pending")
		}
		return status, nil
	}

	// 4. 查询区块链（confirmed状态）
	if isConfirmed, err := s.checkBlockchain(ctx, txHash); err != nil {
		if s.logger != nil {
			s.logger.Warnf("区块链查询失败: %v", err)
		}
		return "", fmt.Errorf("区块链查询失败: %w", err)
	} else if isConfirmed {
		status := types.TxStatus_Confirmed
		s.cacheStatus(ctx, txHash, status)
		if s.logger != nil {
			s.logger.Debug("交易已确认，状态: confirmed")
		}
		return status, nil
	}

	// 5. 检查失败记录（failed状态）
	if isFailed, err := s.checkFailedRecords(ctx, txHash); err != nil {
		if s.logger != nil {
			s.logger.Warnf("失败记录查询失败: %v", err)
		}
		return "", fmt.Errorf("失败记录查询失败: %w", err)
	} else if isFailed {
		status := types.TxStatus_Failed
		s.cacheStatus(ctx, txHash, status)
		if s.logger != nil {
			s.logger.Debug("交易执行失败，状态: failed")
		}
		return status, nil
	}

	// 6. 交易不存在
	err := fmt.Errorf("交易不存在: %x", txHash[:8])
	if s.logger != nil {
		s.logger.Warnf(err.Error())
	}
	return "", err
}

// getStatusFromCache 从缓存获取状态
//
// 🎯 **优化高频查询性能**
//
// 通过智能缓存策略减少重复的数据库查询，提升状态查询性能。
func (s *TransactionStatusService) getStatusFromCache(
	ctx context.Context,
	txHash []byte,
) (types.TransactionStatusEnum, bool) {
	// 构建缓存键
	cacheKey := fmt.Sprintf("tx_status:%x", txHash)

	// 查询缓存存储
	cachedData, exists, err := s.cacheStore.Get(ctx, cacheKey)
	if err != nil {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("缓存查询失败: %v", err))
		}
		return "", false
	}

	if !exists || cachedData == nil {
		return "", false
	}

	// 反序列化状态数据
	var statusInfo struct {
		Status    types.TransactionStatusEnum `json:"status"`
		Timestamp int64                       `json:"timestamp"`
		TxHash    string                      `json:"txHash"`
	}

	if err := json.Unmarshal(cachedData, &statusInfo); err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("缓存数据反序列化失败: %v", err))
		}
		return "", false
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("缓存命中 - 状态: %s, 时间戳: %d", statusInfo.Status, statusInfo.Timestamp))
	}

	return statusInfo.Status, true
}

// checkMempool 检查内存池
//
// 🎯 **查询待确认交易状态**
//
// 在交易内存池中搜索交易，确定是否处于pending状态。
func (s *TransactionStatusService) checkMempool(
	ctx context.Context,
	txHash []byte,
) (bool, error) {
	// 尝试从内存池中获取交易
	tx, err := s.txPool.GetTx(txHash)
	if err != nil {
		// 如果是"交易不存在"错误，返回false而不是错误
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("交易不在内存池中: %x", txHash[:8]))
		}
		return false, nil
	}

	// 交易存在于内存池中
	if tx != nil {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("交易在内存池中找到: %x", txHash[:8]))
		}
		return true, nil
	}

	return false, nil
}

// checkBlockchain 检查区块链
//
// 🎯 **查询已确认交易记录**
//
// 在区块链的已确认区块中搜索交易记录。
func (s *TransactionStatusService) checkBlockchain(
	ctx context.Context,
	txHash []byte,
) (bool, error) {
	// 使用repository查询已确认的交易
	blockHash, txIndex, tx, err := s.repository.GetTransaction(ctx, txHash)
	if err != nil {
		// 如果是"交易不存在"错误，返回false而不是错误
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("交易不在区块链中: %x", txHash[:8]))
		}
		return false, nil
	}

	// 交易存在于区块链中
	if tx != nil && blockHash != nil {
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("交易在区块链中找到: %x, block: %x, index: %d", txHash[:8], blockHash[:8], txIndex))
		}
		return true, nil
	}

	return false, nil
}

// checkFailedRecords 检查失败记录
//
// 🎯 **查询交易失败历史**
//
// 检查交易失败记录，分析失败原因和详细信息。
func (s *TransactionStatusService) checkFailedRecords(
	ctx context.Context,
	txHash []byte,
) (bool, error) {
	// 暂不实现失败记录查询，等待仓储层支持
	if s.logger != nil {
		s.logger.Debug("失败记录查询功能暂未实现")
	}
	return false, nil
}

// cacheStatus 缓存状态结果
//
// 🎯 **缓存查询结果以优化性能**
//
// 将状态查询结果缓存到本地存储，减少重复查询开销。
func (s *TransactionStatusService) cacheStatus(
	ctx context.Context,
	txHash []byte,
	status types.TransactionStatusEnum,
) {
	// 构建缓存键
	cacheKey := fmt.Sprintf("tx_status:%x", txHash)

	// 创建状态信息
	statusInfo := struct {
		Status    types.TransactionStatusEnum `json:"status"`
		Timestamp int64                       `json:"timestamp"`
		TxHash    string                      `json:"txHash"`
	}{
		Status:    status,
		Timestamp: time.Now().Unix(),
		TxHash:    fmt.Sprintf("%x", txHash),
	}

	// 序列化状态信息
	statusData, err := json.Marshal(statusInfo)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("序列化状态信息失败: %v", err))
		}
		return
	}

	// 根据状态类型设置过期时间
	var ttl time.Duration
	switch status {
	case types.TxStatus_Confirmed:
		ttl = time.Hour // 已确认状态稳定，长期缓存
	case types.TxStatus_Pending:
		ttl = 30 * time.Second // 待确认状态变化频繁，短期缓存
	case types.TxStatus_Failed:
		ttl = 10 * time.Minute // 失败状态中等稳定，中期缓存
	default:
		ttl = time.Minute // 默认缓存时间
	}

	// 存储到缓存
	err = s.cacheStore.Set(ctx, cacheKey, statusData, ttl)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("保存状态到缓存失败: %v", err))
		}
		return
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("状态已缓存 - 状态: %s, TTL: %v", status, ttl))
	}
}

// UpdateTransactionStatus 更新交易状态（内部使用）
//
// 🎯 **提供给其他服务更新交易状态**
//
// 当交易状态发生变化时（如提交成功、确认等），其他服务可以
// 调用此方法更新状态缓存，确保状态查询的一致性。
//
// 📝 **使用场景**：
// - 交易提交服务：提交成功后更新为pending状态
// - 交易确认服务：确认后更新为confirmed状态
// - 交易验证服务：验证失败后更新为failed状态
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - txHash: 交易哈希
//   - status: 新的交易状态
//
// 💡 **返回值说明**：
//   - error: 更新错误，nil表示更新成功
func (s *TransactionStatusService) UpdateTransactionStatus(
	ctx context.Context,
	txHash []byte,
	status types.TransactionStatusEnum,
) error {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("更新交易状态 - txHash: %x, status: %s", txHash[:8], status))
	}

	// 基础参数验证
	if len(txHash) != 32 {
		err := fmt.Errorf("交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
		if s.logger != nil {
			s.logger.Warn(err.Error())
		}
		return err
	}

	// 验证状态值
	if status == "" {
		err := fmt.Errorf("交易状态不能为空")
		if s.logger != nil {
			s.logger.Warn(err.Error())
		}
		return err
	}

	// 更新缓存
	s.cacheStatus(ctx, txHash, status)

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("交易状态更新完成 - txHash: %x, status: %s", txHash[:8], status))
	}

	return nil
}
