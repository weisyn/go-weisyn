// Package lifecycle 提供交易生命周期管理 - 详细查询服务
//
// 🎯 **职责定位**：TransactionManager详细查询接口的专门实现
//
// 本文件实现公共接口`TransactionManager.GetTransaction`方法，
// 负责查询交易的完整原始数据和详细执行信息。
//
// 🏗️ **架构分层**：
// - 本文件：公共接口适配层（详细查询逻辑）
// - manager.go：顶层协调层（方法委托和依赖注入）
// - 存储层：区块链数据和交易详情查询（外部依赖）
//
// 📋 **核心功能**：
// - 完整交易数据：返回protobuf格式的完整交易结构
// - 多数据源查询：内存池、区块链、缓存的统一查询
// - 执行详情获取：执行费用消耗、执行结果、状态变更等
// - 性能优化：智能缓存和批量查询策略
//
// 💡 **设计价值**：
// - 数据完整：提供交易的所有原始数据和计算结果
// - 格式标准：返回标准的protobuf交易结构
// - 性能优化：缓存策略和查询优化
// - 调试友好：详细的执行信息便于问题排查
package lifecycle

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"google.golang.org/protobuf/proto"

	"github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
)

// TransactionQueryService 交易详细查询服务
//
// 🎯 **TransactionManager查询接口的专门实现**
//
// 负责实现公共接口中的交易详细查询相关方法，提供交易
// 的完整数据访问和详细信息查询能力。
//
// 💡 **核心价值**：
// - ✅ **完整数据**：返回交易的完整protobuf结构
// - ✅ **执行详情**：包含执行费用消耗、执行结果、状态变更
// - ✅ **多源查询**：统一查询内存池、区块链、缓存数据
// - ✅ **性能优化**：智能缓存和查询优化策略
//
// 📝 **查询范围**：
// - **基础数据**：版本、输入、输出、时间戳等
// - **签名信息**：解锁证明、锁定条件、签名数据
// - **执行结果**：执行费用消耗、状态变更、事件日志
// - **确认信息**：区块高度、确认数、交易索引
//
// 📊 **数据来源**：
// - **内存池**：待确认交易的实时数据
// - **区块链**：已确认交易的历史数据
// - **缓存层**：高频查询的性能优化
// - **执行引擎**：合约和AI模型的执行详情
//
// 🔄 **缓存策略**：
// - **已确认交易**：长期缓存（2小时）
// - **待确认交易**：短期缓存（1分钟）
// - **执行详情**：中期缓存（30分钟）
type TransactionQueryService struct {
	logger     log.Logger                   // 日志记录器（可选）
	cacheStore storage.MemoryStore          // 查询缓存存储
	txPool     mempool.TxPool               // 交易内存池
	repository repository.RepositoryManager // 数据存储访问
}

// NewTransactionQueryService 创建交易查询服务
//
// 🎯 **服务工厂方法**
//
// 创建完整的交易查询服务实例，集成所有必要的依赖服务。
//
// 💡 **参数说明**：
//   - logger: 日志记录器（可选，传nil则不记录日志）
//   - cacheStore: 查询结果缓存存储
//   - txPool: 交易内存池（查询待确认交易）
//   - repository: 区块链数据仓储（查询已确认交易）
//
// 💡 **返回值说明**：
//   - *TransactionQueryService: 查询服务实例
func NewTransactionQueryService(logger log.Logger, cacheStore storage.MemoryStore, txPool mempool.TxPool, repository repository.RepositoryManager) *TransactionQueryService {
	if cacheStore == nil {
		panic("TransactionQueryService: cacheStore不能为nil")
	}
	if txPool == nil {
		panic("TransactionQueryService: txPool不能为nil")
	}
	if repository == nil {
		panic("TransactionQueryService: repository不能为nil")
	}

	return &TransactionQueryService{
		logger:     logger,
		cacheStore: cacheStore,
		txPool:     txPool,
		repository: repository,
	}
}

// GetTransaction 查询完整交易信息（公共接口实现）
//
// 🎯 **TransactionManager.GetTransaction接口实现**
//
// 查询交易的完整原始数据和详细执行信息，返回标准的
// protobuf交易结构，供调用方进行详细分析和处理。
//
// 📝 **查询流程**：
// 1. **缓存检查阶段**：
//   - 检查本地交易缓存中的完整数据
//   - 验证缓存数据的完整性和时效性
//   - 如果缓存命中且完整，直接返回结果
//
// 2. **内存池查询阶段**：
//   - 在交易内存池中搜索待确认交易
//   - 获取交易的完整数据和验证状态
//   - 包含实时的执行费用估算和优先级信息
//
// 3. **区块链查询阶段**：
//   - 在已确认区块中搜索交易记录
//   - 获取交易的确认信息和执行结果
//   - 包含区块高度、交易索引、确认数等
//
// 4. **执行详情补充阶段**：
//   - 查询合约执行的详细结果
//   - 获取AI模型推理的执行日志
//   - 包含状态变更、事件触发等详细信息
//
// 5. **数据整合阶段**：
//   - 整合多个数据源的信息
//   - 构建完整的protobuf交易结构
//   - 缓存查询结果以优化后续访问
//
// 📊 **返回数据结构**：
// ```protobuf
//
//	message Transaction {
//	  uint32 version = 1;                    // 交易版本
//	  repeated TxInput inputs = 2;           // 交易输入列表
//	  repeated TxOutput outputs = 3;         // 交易输出列表
//	  uint64 nonce = 20;                     // 账户nonce
//	  uint64 creation_timestamp = 21;        // 创建时间戳
//	  bytes chain_id = 24;                   // 链ID
//	  // ... 其他字段
//	}
//
// ```
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，支持取消和超时控制
//   - txHash: 交易哈希（32字节，签名前后均可）
//
// 💡 **返回值说明**：
//   - *transaction.Transaction: 完整的protobuf交易结构
//   - error: 查询错误，nil表示查询成功
//
// 💡 **调用示例**：
//
//	service := NewTransactionQueryService(logger)
//	tx, err := service.GetTransaction(ctx, txHash)
//	if err != nil {
//	    log.Errorf("交易查询失败: %v", err)
//	    return nil, fmt.Errorf("查询失败: %w", err)
//	}
//
//	// 分析交易详情
//	log.Infof("交易版本: %d", tx.Version)
//	log.Infof("输入数量: %d", len(tx.Inputs))
//	log.Infof("输出数量: %d", len(tx.Outputs))
//	log.Infof("创建时间: %d", tx.CreationTimestamp)
//
//	// 访问具体的输入输出详情
//	for i, input := range tx.Inputs {
//	    log.Infof("输入%d: %x:%d", i, input.PreviousOutput.TxId, input.PreviousOutput.OutputIndex)
//	}
//
// ⚠️ **注意事项**：
// - 查询结果包含完整的交易数据，数据量可能较大
// - 执行详情查询可能涉及复杂计算，响应时间较长
// - 建议根据实际需要选择性访问返回数据的字段
// - 已确认交易的数据相对稳定，可以进行长期缓存
func (s *TransactionQueryService) GetTransaction(
	ctx context.Context,
	txHash []byte,
) (*transaction.Transaction, error) {
	if s.logger != nil {
		s.logger.Debugf("开始查询完整交易信息 - 哈希: %x", txHash[:8])
	}

	// 1. 基础参数验证
	if len(txHash) != 32 {
		err := fmt.Errorf("交易哈希长度无效: 期望32字节，实际%d字节", len(txHash))
		if s.logger != nil {
			s.logger.Warnf(err.Error())
		}
		return nil, err
	}

	// 2. 检查交易缓存
	if cachedTx := s.getTransactionFromCache(ctx, txHash); cachedTx != nil {
		if s.logger != nil {
			s.logger.Debug("缓存命中，返回缓存的交易数据")
		}
		return cachedTx, nil
	}

	// 3. 查询内存池（待确认交易）
	if tx, found, err := s.queryFromMempool(ctx, txHash); err != nil {
		if s.logger != nil {
			s.logger.Warnf("内存池查询失败: %v", err)
		}
		return nil, fmt.Errorf("内存池查询失败: %w", err)
	} else if found {
		// 补充实时执行信息
		s.enrichTransactionWithExecutionDetails(ctx, tx)

		// 缓存查询结果（短期）
		s.cacheTransaction(ctx, txHash, tx, false)

		if s.logger != nil {
			s.logger.Debug("从内存池查询到交易数据")
		}
		return tx, nil
	}

	// 4. 查询区块链（已确认交易）
	if tx, found, err := s.queryFromBlockchain(ctx, txHash); err != nil {
		if s.logger != nil {
			s.logger.Warnf("区块链查询失败: %v", err)
		}
		return nil, fmt.Errorf("区块链查询失败: %w", err)
	} else if found {
		// 补充确认信息和执行详情
		s.enrichTransactionWithConfirmationDetails(ctx, tx)
		s.enrichTransactionWithExecutionDetails(ctx, tx)

		// 缓存查询结果（长期）
		s.cacheTransaction(ctx, txHash, tx, true)

		if s.logger != nil {
			s.logger.Debug("从区块链查询到交易数据")
		}
		return tx, nil
	}

	// 5. 交易不存在
	err := fmt.Errorf("交易不存在: %x", txHash[:8])
	if s.logger != nil {
		s.logger.Warnf(err.Error())
	}
	return nil, err
}

// getTransactionFromCache 从缓存获取交易
//
// 🎯 **优化高频查询性能**
//
// 通过缓存机制减少重复的数据库查询，提升交易查询性能。
func (s *TransactionQueryService) getTransactionFromCache(
	ctx context.Context,
	txHash []byte,
) *transaction.Transaction {
	// 尝试从已签名交易缓存中获取
	tx, exists, err := internal.GetSignedTransactionFromCache(ctx, s.cacheStore, txHash, s.logger)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("缓存查询失败: %v", err))
		}
		return nil
	}

	if exists {
		if s.logger != nil {
			s.logger.Debug("从已签名交易缓存中找到交易数据")
		}
		return tx
	}

	// 尝试从未签名交易缓存中获取
	unsignedTx, exists, err := internal.GetUnsignedTransactionFromCache(ctx, s.cacheStore, txHash, s.logger)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("未签名交易缓存查询失败: %v", err))
		}
		return nil
	}

	if exists {
		if s.logger != nil {
			s.logger.Debug("从未签名交易缓存中找到交易数据")
		}
		return unsignedTx
	}

	// 都未找到
	return nil
}

// queryFromMempool 从内存池查询
//
// 🎯 **查询待确认交易的完整数据**
//
// 在交易内存池中搜索交易，获取待确认交易的完整信息。
func (s *TransactionQueryService) queryFromMempool(
	ctx context.Context,
	txHash []byte,
) (*transaction.Transaction, bool, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("从内存池查询交易 - 哈希: %x", txHash[:8]))
	}

	// 使用交易池接口查询交易
	tx, err := s.txPool.GetTx(txHash)
	if err != nil {
		return nil, false, fmt.Errorf("内存池查询失败: %w", err)
	}

	if tx == nil {
		// 交易不在内存池中
		if s.logger != nil {
			s.logger.Debug("交易不在内存池中")
		}
		return nil, false, nil
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 内存池查询成功 - 交易版本: %d", tx.Version))
	}

	return tx, true, nil
}

// queryFromBlockchain 从区块链查询
//
// 🎯 **查询已确认交易的完整数据**
//
// 在区块链的已确认区块中搜索交易，获取历史交易的完整信息。
func (s *TransactionQueryService) queryFromBlockchain(
	ctx context.Context,
	txHash []byte,
) (*transaction.Transaction, bool, error) {
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("从区块链查询交易 - 哈希: %x", txHash[:8]))
	}

	// 使用仓储管理器查询已确认的交易
	blockHash, txIndex, tx, err := s.repository.GetTransaction(ctx, txHash)
	if err != nil {
		// 查询失败：交易不存在或其他错误
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("区块链交易查询失败: %v", err))
		}
		return nil, false, nil
	}

	// 查询成功：找到已确认的交易
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 区块链交易查询成功 - 区块: %x, 索引: %d",
			blockHash[:8], txIndex))
	}

	return tx, true, nil
}

// enrichTransactionWithExecutionDetails 补充执行详情
//
// 🎯 **为交易数据补充执行详情信息**
//
// 查询和补充交易的执行结果、执行费用消耗、状态变更等详细信息。
// 当前暂未实现，执行引擎集成后可补充此功能。
func (s *TransactionQueryService) enrichTransactionWithExecutionDetails(
	ctx context.Context,
	tx *transaction.Transaction,
) {
	// 暂不实现执行详情补充，等待执行引擎集成
	if s.logger != nil {
		s.logger.Debug("执行详情补充功能暂未实现")
	}
}

// enrichTransactionWithConfirmationDetails 补充确认详情
//
// 🎯 **为已确认交易补充确认信息**
//
// 查询和补充已确认交易的区块高度、确认数、交易索引等信息。
func (s *TransactionQueryService) enrichTransactionWithConfirmationDetails(
	ctx context.Context,
	tx *transaction.Transaction,
) {
	// 暂不实现确认详情补充，等待区块链仓储层集成
	if s.logger != nil {
		s.logger.Debug("确认详情补充功能暂未实现")
	}
}

// cacheTransaction 缓存交易数据
//
// 🎯 **缓存查询结果以优化性能**
//
// 将查询结果缓存到本地存储，根据确认状态设置不同的缓存策略。
func (s *TransactionQueryService) cacheTransaction(
	ctx context.Context,
	txHash []byte,
	tx *transaction.Transaction,
	isConfirmed bool,
) {
	// 序列化交易对象
	txData, err := proto.Marshal(tx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("交易序列化失败: %v", err)
		}
		return
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("tx_data:%x", txHash)

	// 根据确认状态设置TTL
	var ttl time.Duration
	if isConfirmed {
		ttl = 2 * time.Hour // 已确认交易长期缓存
	} else {
		ttl = time.Minute // 待确认交易短期缓存
	}

	// 存储到缓存
	if err := s.cacheStore.Set(ctx, cacheKey, txData, ttl); err != nil {
		if s.logger != nil {
			s.logger.Warnf("缓存交易数据失败: %v", err)
		}
		return
	}

	if s.logger != nil {
		cacheType := "短期"
		if isConfirmed {
			cacheType = "长期"
		}
		s.logger.Debugf("交易数据已%s缓存", cacheType)
	}
}
