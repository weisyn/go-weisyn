// Package writer 实现 DataWriter 接口
//
// ✍️ **数据写入服务 (Data Writer Service)**
//
// 本包实现 WES 系统的统一数据写入接口，提供区块写入入口，
// 协调所有数据写入操作，确保原子性和一致性。
//
// 🎯 **核心职责**：
// - 实现 persistence.DataWriter 接口
// - 协调所有数据写入操作（区块、交易索引、UTXO、链状态、资源索引）
// - 确保所有写操作在单一事务中完成
// - 严格验证高度顺序
//
// 🏗️ **设计原则**：
// - 统一入口：区块是唯一数据写入点
// - 有序写入：严格按高度顺序写入
// - 原子性：所有操作在单一事务中完成
// - 避免循环依赖：直接读存储，不依赖 QueryService
package writer

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/internal/core/persistence/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// ✅ **架构修复**：
// 本文件已移除对 eutxo.UTXOWriter 的依赖，符合架构原则。
// - UTXO 的创建和删除直接操作存储，不依赖业务层组件
// - 引用计数管理和状态根更新应由调用方（BlockProcessor）通过 eutxo.UTXOWriter 处理
// - Persistence 只负责持久化操作，不处理业务逻辑

// Service DataWriter 实现
//
// 🎯 **核心职责**：
// 实现统一数据写入接口，协调所有数据写入操作。
//
// 💡 **设计理念**：
// - 实现内部接口 interfaces.InternalDataWriter（遵循代码组织规范）
// - 直接操作存储，不依赖 QueryService（避免循环依赖）
// - 协调各领域 Writer，但不暴露给调用方
// - 所有操作在事务中原子性完成
//
// ⚠️ **实现约束**：
// - 必须实现 interfaces.InternalDataWriter（内部接口）
// - 通过 module.go 绑定到 persistence.DataWriter（公共接口）
type Service struct {
	// 存储服务
	storage storage.BadgerStore

	// fileStore 文件存储服务（用于区块 blocks/、资源/附件等文件类数据）
	fileStore storage.FileStore

	// blockHashClient 区块哈希服务客户端（用于计算区块哈希）
	blockHashClient core.BlockHashServiceClient

	// txHashClient 交易哈希服务客户端（用于计算交易哈希）
	txHashClient transaction.TransactionHashServiceClient

	// ✅ **架构修复**：已移除 utxoWriter 和 utxoQuery 依赖
	// - 引用计数管理和状态根更新应由调用方（BlockProcessor）通过 eutxo.UTXOWriter 处理
	// - Persistence 只负责持久化操作，不处理业务逻辑

	// 辅助服务
	logger log.Logger
}

// 编译期检查：确保 Service 实现了内部接口
var _ interfaces.InternalDataWriter = (*Service)(nil)

// NewService 创建新的 DataWriter 服务
//
// 🏗️ **构造器模式**：
// 通过依赖注入方式创建服务实例，遵循代码组织规范。
//
// ⚙️ **参数说明**：
// - storage: BadgerDB 存储服务（必需）
// - fileStore: 文件存储服务（必需，用于区块 blocks/ 以及资源/附件等文件类数据）
// - blockHashClient: 区块哈希服务客户端（必需）
// - txHashClient: 交易哈希服务客户端（必需）
// - logger: 日志记录器（可选）
//
// ✅ **架构修复**：
// - UTXO 的创建和删除直接操作存储，不依赖业务层组件
// - 引用计数管理和状态根更新应由调用方（BlockProcessor）通过 eutxo.UTXOWriter 处理
// - Persistence 只负责持久化操作，不处理业务逻辑
//
// 📋 **返回类型**：
// - 返回 interfaces.InternalDataWriter（内部接口）
// - 通过 module.go 绑定到 persistence.DataWriter（公共接口）
func NewService(
	storage storage.BadgerStore,
	fileStore storage.FileStore,
	blockHashClient core.BlockHashServiceClient,
	txHashClient transaction.TransactionHashServiceClient,
	logger log.Logger,
) interfaces.InternalDataWriter {
	return &Service{
		storage:         storage,
		fileStore:       fileStore,
		blockHashClient: blockHashClient,
		txHashClient:    txHashClient,
		logger:          logger,
	}
}

// WriteBlock 实现 DataWriter 接口
//
// 🎯 **核心方法**：
// 这是数据层的唯一写入入口，所有数据（区块、交易索引、UTXO、状态）
// 都通过此方法写入。
//
// 📋 **处理流程**：
// 1. 验证高度顺序（必须 = currentHeight + 1）
// 2. 在事务中原子性完成所有写操作
//   - 存储区块数据
//   - 更新交易索引
//   - 处理 UTXO 变更
//   - 更新链状态
//   - 更新资源索引
//
// 3. 提交事务（全部成功或全部失败）
func (s *Service) WriteBlock(ctx context.Context, block *core.Block) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "persistence.DataWriter.WriteBlock"); err != nil {
		return err
	}
	// 1. 验证高度顺序（严格有序写入原则）
	// ⚠️ 关键设计：直接读存储，不依赖 QueryService，避免循环依赖
	currentHeight, err := s.getCurrentHeight(ctx)
	if err != nil {
		return fmt.Errorf("获取当前链高度失败: %w", err)
	}

	// 🔧 **高度验证逻辑（严格有序写入原则）**：
	//
	// 1. 创世区块（高度 0）：
	//    - 如果 currentHeight == 0 且链尖不存在（空链），允许写入
	//    - 如果 currentHeight == 0 且链尖存在（已有创世区块），拒绝重复写入
	//    - 如果 currentHeight > 0，说明链已初始化，创世区块不应再次写入
	//
	// 2. 非创世区块（高度 > 0）：
	//    - 必须严格有序：expectedHeight = currentHeight + 1
	//    - 如果 currentHeight == 0 且链尖存在，说明创世区块已存在，下一个应为高度 1
	//    - 如果 currentHeight == 0 且链尖不存在，必须先写入创世区块
	//
	// ⚠️ **架构原则**：
	// - 不破坏读写分离：Writer 只负责写入，不依赖 QueryService
	// - 直接读存储获取状态，避免循环依赖
	var expectedHeight uint64
	if block.Header.Height == 0 {
		// 创世区块验证：链尖必须不存在（空链状态）
		if currentHeight > 0 {
			// 链已有区块，不应该再写入创世区块
			return fmt.Errorf("%w: 链已初始化（当前高度=%d），不允许再次写入创世区块",
				persistence.ErrInvalidHeight, currentHeight)
		}
		// currentHeight == 0，但需要确认链尖是否真的不存在
		// 如果链尖存在但高度为0，说明创世区块已存在
		tipKey := []byte("state:chain:tip")
		// ⚠️ 不使用 Get+len 判断存在性：因为“键存在但值为空”会被误判为不存在
		exists, tipErr := s.storage.Exists(ctx, tipKey)
		if tipErr != nil {
			return fmt.Errorf("检查链尖状态失败: %w", tipErr)
		}
		if exists {
			// 链尖已存在，说明创世区块已存在
			return fmt.Errorf("%w: 创世区块已存在，不允许重复写入",
				persistence.ErrInvalidHeight)
		}
		// 链尖不存在，允许写入创世区块
		expectedHeight = 0
	} else {
		// 非创世区块验证：必须严格有序
		if currentHeight == 0 {
			// 当前高度为0，需要检查链尖是否存在
			// 如果链尖不存在，必须先写入创世区块
			tipKey := []byte("state:chain:tip")
			exists, tipErr := s.storage.Exists(ctx, tipKey)
			if tipErr != nil {
				return fmt.Errorf("检查链尖状态失败: %w", tipErr)
			}
			if !exists {
				// 链尖不存在，必须先写入创世区块
				return fmt.Errorf("%w: 链未初始化，必须先写入创世区块（高度0），当前尝试写入高度%d",
					persistence.ErrInvalidHeight, block.Header.Height)
			}
		}
		// 正常情况：期望高度 = 当前高度 + 1
		expectedHeight = currentHeight + 1
	}

	if block.Header.Height != expectedHeight {
		// 🆕 2025-12-18: 区分"区块已处理"和"区块高度异常"两种情况
		if block.Header.Height < expectedHeight {
			// 区块高度低于期望，说明已被其他流程处理（如聚合器/挖矿）
			// 返回 ErrBlockAlreadyProcessed，允许调用方优雅跳过
			return fmt.Errorf("%w: 期望 %d, 实际 %d（该区块可能已被其他流程处理）",
				persistence.ErrBlockAlreadyProcessed, expectedHeight, block.Header.Height)
		}
		// 区块高度高于期望，说明缺失中间区块
		return fmt.Errorf("%w: 期望 %d, 实际 %d（DataWriter 只接受有序写入，分叉处理应由 BLOCK/CHAIN 层完成）",
			persistence.ErrInvalidHeight, expectedHeight, block.Header.Height)
	}

	// 3. 在事务中原子性完成所有写操作
	err = s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 3.1 存储区块数据
		if err := s.writeBlockData(ctx, tx, block); err != nil {
			return fmt.Errorf("存储区块数据失败: %w", err)
		}

		// 3.2 更新交易索引（从区块中提取交易，只存储索引）
		if err := s.writeTransactionIndices(ctx, tx, block); err != nil {
			return fmt.Errorf("更新交易索引失败: %w", err)
		}

		// 3.3 处理 UTXO 变更（从交易中提取）
		// 彻底迭代：引用型输入的“计数/统计”在事务内更新（如 ResourceUsageCounters），不再依赖事务后回调
		if err := s.writeUTXOChanges(ctx, tx, block); err != nil {
			return fmt.Errorf("处理UTXO变更失败: %w", err)
		}

		// 3.4 更新链状态
		if err := s.writeChainState(ctx, tx, block); err != nil {
			return fmt.Errorf("更新链状态失败: %w", err)
		}

		// 3.5 更新资源索引（如果有资源相关交易）
		if err := s.writeResourceIndices(ctx, tx, block); err != nil {
			return fmt.Errorf("更新资源索引失败: %w", err)
		}

		return nil // 事务提交
	})

	if err != nil {
		return err
	}

	return nil
}

// WriteBlocks 实现 DataWriter 接口（批量写入）
//
// 🎯 **批量优化**：
// 用于同步场景，批量写入多个连续区块，提升性能。
//
// ⚠️ **严格有序约束**：
// - 区块列表必须连续（高度 n, n+1, n+2, ...）
// - 第一个区块高度必须 = currentHeight + 1
func (s *Service) WriteBlocks(ctx context.Context, blocks []*core.Block) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "persistence.DataWriter.WriteBlocks"); err != nil {
		return err
	}
	if len(blocks) == 0 {
		return fmt.Errorf("区块列表为空")
	}

	// 1. 验证高度顺序和连续性
	currentHeight, err := s.getCurrentHeight(ctx)
	if err != nil {
		return fmt.Errorf("获取当前链高度失败: %w", err)
	}

	expectedHeight := currentHeight + 1
	if blocks[0].Header.Height != expectedHeight {
		return fmt.Errorf("%w: 第一个区块高度不匹配，期望 %d, 实际 %d",
			persistence.ErrInvalidHeight, expectedHeight, blocks[0].Header.Height)
	}

	// 验证连续性
	for i := 1; i < len(blocks); i++ {
		if blocks[i].Header.Height != blocks[i-1].Header.Height+1 {
			return fmt.Errorf("%w: 区块不连续，位置 %d 的高度 %d 不等于前一个高度 %d + 1",
				persistence.ErrInvalidHeight, i, blocks[i].Header.Height, blocks[i-1].Header.Height)
		}
	}

	// 🆕 3. 分批写入以避免 "Txn is too big" 错误
	// 配置：每批最多写入多少个区块（默认5个，可根据实际区块大小调整）
	batchSize := 5
	// TODO: 从配置中读取 batchSize

	// 分批写入循环
	for i := 0; i < len(blocks); i += batchSize {
		end := i + batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]

		// 在事务中原子性完成当前批次的所有写操作
		err = s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
			for _, block := range batch {
				// 3.1 存储区块数据
				if err := s.writeBlockData(ctx, tx, block); err != nil {
					return fmt.Errorf("存储区块数据失败（高度 %d）: %w", block.Header.Height, err)
				}

				// 3.2 更新交易索引
				if err := s.writeTransactionIndices(ctx, tx, block); err != nil {
					return fmt.Errorf("更新交易索引失败（高度 %d）: %w", block.Header.Height, err)
				}

				// 3.2.5 ✅ 新增：更新历史交易索引（必须在writeUTXOChanges之前，因为需要从UTXO中提取资源信息）
				// 注意：必须在writeUTXOChanges之前调用，因为消费型输入会删除UTXO
				if err := s.writeResourceHistoryIndices(ctx, tx, block); err != nil {
					return fmt.Errorf("更新资源历史索引失败（高度 %d）: %w", block.Header.Height, err)
				}
				if err := s.writeUTXOHistoryIndices(ctx, tx, block); err != nil {
					return fmt.Errorf("更新UTXO历史索引失败（高度 %d）: %w", block.Header.Height, err)
				}

				// 3.3 处理 UTXO 变更（事务内完成）
				if err := s.writeUTXOChanges(ctx, tx, block); err != nil {
					return fmt.Errorf("处理UTXO变更失败（高度 %d）: %w", block.Header.Height, err)
				}

				// 3.4 更新链状态（只更新批次中最后一个区块的状态）
				if block == batch[len(batch)-1] {
					if err := s.writeChainState(ctx, tx, block); err != nil {
						return fmt.Errorf("更新链状态失败（高度 %d）: %w", block.Header.Height, err)
					}
				}

				// 3.5 更新资源索引
				if err := s.writeResourceIndices(ctx, tx, block); err != nil {
					return fmt.Errorf("更新资源索引失败（高度 %d）: %w", block.Header.Height, err)
				}
			}

			return nil // 事务提交
		})

		if err != nil {
			return fmt.Errorf("批次写入失败（区块 %d-%d）: %w", batch[0].Header.Height, batch[len(batch)-1].Header.Height, err)
		}

		// 记录批次写入进度（仅对大批量操作）
		if s.logger != nil && len(blocks) > batchSize {
			s.logger.Infof("📦 批次写入成功: 区块 %d-%d (%d/%d)",
				batch[0].Header.Height, batch[len(batch)-1].Header.Height, end, len(blocks))
		}
	}

	return nil
}

// ✅ **架构修复**：已移除 updateStateRootAfterUTXOChanges 方法
// 状态根更新应由调用方（BlockProcessor）通过 eutxo.UTXOWriter 处理
// Persistence 只负责持久化操作，不处理业务逻辑

// getCurrentHeight 获取当前链高度（直接从存储读取，避免循环依赖）
//
// ⚠️ **关键设计**：
// 直接读存储，不依赖 QueryService，避免循环依赖。
//
// 📋 **实现方式**：
// - 读取键：`state:chain:tip`
// - 值格式：height(8字节) + blockHash(32字节)
// - 解析前8字节作为高度
//
// 🔧 **空链和创世区块处理**：
// - BadgerDB.Get 在键不存在时返回 (nil, nil)，不是错误
// - 如果 tipData == nil 或 len(tipData) == 0，表示链尖不存在（空链）
// - 空链状态返回高度 0，允许写入创世区块
// - 如果数据存在但格式不完整（长度 < 8），表示数据损坏，返回错误
// - 正常情况下解析并返回高度
func (s *Service) getCurrentHeight(ctx context.Context) (uint64, error) {
	tipKey := []byte("state:chain:tip")
	tipData, err := s.storage.Get(ctx, tipKey)
	if err != nil {
		// 存储读取错误（非键不存在错误），返回错误
		return 0, fmt.Errorf("读取链尖状态失败: %w", err)
	}

	// BadgerDB 在键不存在时返回 (nil, nil)，不是错误
	// 如果 tipData 为空，表示链尖不存在（空链状态）
	if len(tipData) == 0 {
		// 空链状态：返回高度 0，允许写入创世区块
		return 0, nil
	}

	// 数据存在但格式不完整，表示数据损坏
	if len(tipData) < 8 {
		return 0, fmt.Errorf("链尖数据格式错误：长度不足8字节（实际长度=%d）", len(tipData))
	}

	// 正常情况：解析高度（前8字节，BigEndian）
	return bytesToUint64(tipData[:8]), nil
}

// 注意：writeBlockData、writeTransactionIndices、writeUTXOChanges、
// writeChainState、writeResourceIndices 的实现都在对应的单独文件中
// (block.go, transaction.go, utxo.go, chain.go, resource.go)

// ============================================================================
//                              辅助函数
// ============================================================================

// uint64ToBytes 将 uint64 转换为字节数组（BigEndian）
func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// bytesToUint64 将字节数组转换为 uint64（BigEndian）
func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
