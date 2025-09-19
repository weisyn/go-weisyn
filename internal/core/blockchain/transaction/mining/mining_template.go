// Package mining 提供挖矿模板生成相关的业务逻辑实现
//
// 🏗️ **挖矿模块架构设计**
//
// 本模块负责为区块链矿工提供完整的交易模板生成服务：
// - **模板生成**：生成包含 Coinbase 交易和待确认交易的完整模板
// - **交易排序**：按照优先级和手续费对交易进行排序
// - **奖励计算**：计算挖矿奖励和手续费收益
// - **模板验证**：确保生成的模板符合区块链规则
//
// 🎯 **业务职责**
// - **Coinbase构建**：生成矿工奖励交易（固定奖励+交易手续费）
// - **交易选择**：从交易池中选择最优的待确认交易
// - **模板组装**：将 Coinbase 和待确认交易组合成完整模板
// - **性能优化**：缓存和批量处理，提高模板生成效率
//
// ⚠️ **架构一致性**
// - 与其他业务模块保持一致的目录结构和设计模式
// - 遵循薄服务层原则，专注于挖矿模板相关业务逻辑
// - 支持依赖注入和模块化测试
package mining

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 费用系统集成
	"github.com/weisyn/v1/internal/core/blockchain/transaction/fee"

	// 协议定义
	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
//                              挖矿模板服务
// ============================================================================

// MiningTemplateService 挖矿模板生成服务
//
// 🎯 **核心职责**：为矿工提供完整的区块模板生成服务
//
// 📋 **主要功能**：
// - 生成包含 Coinbase 交易的完整挖矿模板
// - 从交易池选择最优交易组合
// - 计算矿工奖励和手续费收益
// - 优化模板生成性能
//
// 🏗️ **架构设计**：
// - 专门的业务服务类，遵循模块化架构
// - 支持依赖注入，便于测试和扩展
// - 与其他模块保持一致的设计模式
type MiningTemplateService struct {
	// ========== 基础设施依赖 ==========
	repo                repository.RepositoryManager      // 数据存储访问
	txPool              mempool.TxPool                    // 交易池访问
	utxoManager         repository.UTXOManager            // UTXO管理
	minerService        consensus.MinerService            // 矿工服务
	configManager       config.Provider                   // 配置管理器
	txHashServiceClient pbtx.TransactionHashServiceClient // 交易哈希服务客户端
	hashManager         crypto.HashManager                // 哈希计算服务
	addressManager      crypto.AddressManager             // 地址管理服务
	cacheStore          storage.MemoryStore               // 内存缓存服务
	logger              log.Logger                        // 日志记录器（可选）
}

// NewMiningTemplateService 创建新的挖矿模板服务实例
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数说明：
//   - repo: 仓储管理器，提供底层数据访问能力
//   - txPool: 交易池，提供待确认交易
//   - utxoManager: UTXO管理器，用于验证交易有效性
//   - consensusService: 共识服务，提供矿工地址和共识参数
//   - configManager: 配置管理器，提供链ID等配置信息
//   - txHashServiceClient: 交易哈希服务客户端
//   - hashManager: 哈希管理器，用于计算交易和区块哈希
//   - addressManager: 地址管理器，用于地址相关操作
//   - cacheStore: 内存缓存服务，用于缓存模板数据
//   - logger: 日志记录器，用于记录操作日志（可选）
//
// 返回：
//   - *MiningTemplateService: 挖矿模板服务实例
func NewMiningTemplateService(
	repo repository.RepositoryManager,
	txPool mempool.TxPool,
	utxoManager repository.UTXOManager,
	minerService consensus.MinerService,
	configManager config.Provider,
	txHashServiceClient pbtx.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	addressManager crypto.AddressManager,
	cacheStore storage.MemoryStore,
	logger log.Logger,
) *MiningTemplateService {
	if repo == nil {
		panic("挖矿模板服务初始化失败：仓储管理器不能为空")
	}
	if txPool == nil {
		panic("挖矿模板服务初始化失败：交易池不能为空")
	}
	if utxoManager == nil {
		panic("挖矿模板服务初始化失败：UTXO管理器不能为空")
	}
	// 矿工服务允许为nil，在共识模块启动后再注入
	// if minerService == nil {
	//     panic("挖矿模板服务初始化失败：矿工服务不能为空")
	// }
	if configManager == nil {
		panic("挖矿模板服务初始化失败：配置管理器不能为空")
	}
	if txHashServiceClient == nil {
		panic("挖矿模板服务初始化失败：交易哈希服务客户端不能为空")
	}
	if hashManager == nil {
		panic("挖矿模板服务初始化失败：哈希管理器不能为空")
	}
	if addressManager == nil {
		panic("挖矿模板服务初始化失败：地址管理器不能为空")
	}
	if cacheStore == nil {
		panic("挖矿模板服务初始化失败：内存缓存服务不能为空")
	}

	service := &MiningTemplateService{
		repo:                repo,
		txPool:              txPool,
		utxoManager:         utxoManager,
		minerService:        minerService,
		configManager:       configManager,
		txHashServiceClient: txHashServiceClient,
		hashManager:         hashManager,
		addressManager:      addressManager,
		cacheStore:          cacheStore,
		logger:              logger,
	}

	if logger != nil {
		logger.Info("✅ 挖矿模板服务初始化完成 - component: MiningTemplateService")
	}

	return service
}

// SetMinerService 设置矿工服务（用于延迟注入，解决循环依赖）
func (s *MiningTemplateService) SetMinerService(minerService consensus.MinerService) {
	s.minerService = minerService
	if s.logger != nil {
		s.logger.Info("🔗 挖矿模板服务已注入矿工服务")
	}
}

// ============================================================================
//                              核心业务方法
// ============================================================================

// GetMiningTemplate 获取包含 Coinbase 在首位的完整挖矿交易模板
//
// 🎯 **挖矿模板生成核心逻辑**
//
// 实现逻辑：
// 1) 从内存池获取用于挖矿的优质交易（由内存池配置约束数量和大小）
// 2) 使用费用系统收集所有交易费用并构建 Coinbase 交易
// 3) 组合为 [coinbase, ...transactions]
//
// 💡 **参数说明**：
//   - ctx: 上下文对象，用于超时控制和取消操作
//
// 💡 **返回值说明**：
//   - []*pbtx.Transaction: 挖矿模板交易列表（Coinbase在首位）
//   - error: 生成错误
func (s *MiningTemplateService) GetMiningTemplate(ctx context.Context) ([]*pbtx.Transaction, error) {
	if s.logger != nil {
		s.logger.Debug("🏗️ 开始生成挖矿模板")
	}

	// 1. 从交易池获取候选交易（排序与筛选由交易池内部处理）
	txs, err := s.txPool.GetTransactionsForMining()
	if err != nil {
		return nil, fmt.Errorf("获取挖矿交易失败: %w", err)
	}

	// 2. 从矿工服务获取矿工地址
	if s.minerService == nil {
		return nil, fmt.Errorf("矿工服务尚未初始化，无法生成挖矿模板")
	}

	isRunning, minerAddr, err := s.minerService.GetMiningStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取挖矿状态失败: %w", err)
	}

	// 检查挖矿状态和矿工地址
	if !isRunning {
		return nil, fmt.Errorf("挖矿未启动，无法生成挖矿模板")
	}

	if len(minerAddr) == 0 {
		return nil, fmt.Errorf("矿工地址为空，无法生成挖矿模板")
	}

	// 3. 获取链ID（从配置中获取）
	chainID, err := s.getChainIDFromConfig()
	if err != nil {
		return nil, fmt.Errorf("获取链ID失败: %v", err)
	}

	if len(chainID) == 0 {
		return nil, fmt.Errorf("链ID为空，无法生成挖矿模板")
	}

	// 4. 创建UTXO查询回调函数（费用系统需要）
	fetchUTXO := func(ctx context.Context, outpoint *pbtx.OutPoint) (*pbtx.TxOutput, error) {
		utxo, err := s.utxoManager.GetUTXO(ctx, outpoint)
		if err != nil {
			return nil, fmt.Errorf("获取UTXO失败 [%x:%d]: %v",
				outpoint.TxId, outpoint.OutputIndex, err)
		}

		// 从UTXO中提取TxOutput，根据UTXO的存储策略处理
		return s.extractTxOutputFromUTXO(ctx, utxo)
	}

	// 5. 使用费用系统生成 Coinbase 交易（传入交易池交易和UTXO查询回调）
	// 创建费用管理器实例
	feeManager := fee.NewManager(s.txHashServiceClient)
	if feeManager == nil {
		return nil, fmt.Errorf("创建费用管理器失败")
	}

	// 调用费用系统生成真实的Coinbase交易
	coinbase, err := feeManager.CollectFeesAndBuildCoinbase(ctx, txs, minerAddr, chainID, fetchUTXO)
	if err != nil {
		if s.logger != nil {
			s.logger.Error(fmt.Sprintf("❌ Coinbase构建失败: %v", err))
		}
		return nil, fmt.Errorf("费用系统Coinbase构建失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("💰 Coinbase交易构建完成 - 输出数量: %d", len(coinbase.Outputs)))
	}

	// 6. 组合最终挖矿模板：[coinbase, ...transactions]
	template := make([]*pbtx.Transaction, 0, len(txs)+1)
	template = append(template, coinbase) // Coinbase交易排在第一位
	template = append(template, txs...)   // 添加交易池中的交易

	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("✅ 挖矿模板生成完成 - Coinbase: 1个, 普通交易: %d个, 总计: %d个",
			len(txs), len(template)))
	}

	return template, nil
}

// ============================================================================
//                              内部辅助方法
// ============================================================================

// getChainIDFromConfig 从配置中获取链ID
//
// 🎯 **链ID配置获取工具**
//
// 从配置管理器中获取当前链的ID，用于防重放攻击。
//
// 返回值：
//   - []byte: 链ID字节数组
//   - error: 获取错误
func (s *MiningTemplateService) getChainIDFromConfig() ([]byte, error) {
	if s.configManager == nil {
		return nil, fmt.Errorf("配置管理器为空")
	}

	// 获取链ID配置
	blockchainConfig := s.configManager.GetBlockchain()
	if blockchainConfig == nil {
		if s.logger != nil {
			s.logger.Warn("获取区块链配置失败，使用默认链ID")
		}
		return []byte("weisyn-mainnet"), nil
	}

	// ChainID 是 uint64 类型，直接转换为字符串
	chainID := blockchainConfig.ChainID
	if chainID == 0 {
		return nil, fmt.Errorf("链ID不能为0")
	}

	// 将数字链ID转换为有意义的字符串格式
	chainIDStr := fmt.Sprintf("weisyn-chain-%d", chainID)
	chainIDBytes := []byte(chainIDStr)

	// 添加日志调试
	if s.logger != nil {
		s.logger.Debugf("链ID调试: 原始值=%d, 转换后=%s, 字节长度=%d", chainID, chainIDStr, len(chainIDBytes))
	}

	// 验证链ID长度（至少4字节）
	if len(chainIDBytes) < 4 {
		return nil, fmt.Errorf("链ID长度过短: %d", len(chainIDBytes))
	}

	return chainIDBytes, nil
}

// extractTxOutputFromUTXO 从UTXO中提取TxOutput
//
// 🎯 **UTXO内容提取工具**
//
// 根据UTXO的存储策略提取TxOutput内容：
// - 热数据策略：直接从cached_output获取
// - 冷数据策略：通过区块链回溯获取
//
// 参数：
//   - ctx: 上下文对象
//   - utxoData: UTXO数据对象
//
// 返回值：
//   - *pbtx.TxOutput: 提取的TxOutput对象
//   - error: 提取错误
func (s *MiningTemplateService) extractTxOutputFromUTXO(
	ctx context.Context,
	utxoData *utxo.UTXO,
) (*pbtx.TxOutput, error) {
	if utxoData == nil {
		return nil, fmt.Errorf("UTXO数据为空")
	}

	// 检查UTXO的存储策略
	switch strategy := utxoData.ContentStrategy.(type) {
	case *utxo.UTXO_CachedOutput:
		// 热数据策略：直接从缓存获取
		if strategy.CachedOutput == nil {
			return nil, fmt.Errorf("UTXO缓存输出为空")
		}
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("🔥 使用热数据缓存 - UTXO: %x:%d",
				utxoData.Outpoint.TxId, utxoData.Outpoint.OutputIndex))
		}
		return strategy.CachedOutput, nil

	case *utxo.UTXO_ReferenceOnly:
		// 冷数据策略：需要从区块链回溯获取
		if s.logger != nil {
			s.logger.Debug(fmt.Sprintf("🧊 使用冷数据回溯 - UTXO: %x:%d",
				utxoData.Outpoint.TxId, utxoData.Outpoint.OutputIndex))
		}
		return s.fetchTxOutputFromBlockchain(ctx, utxoData.Outpoint)

	default:
		return nil, fmt.Errorf("未知的UTXO存储策略: %T", strategy)
	}
}

// fetchTxOutputFromBlockchain 从区块链获取TxOutput
//
// 🎯 **区块链数据回溯工具**
//
// 当UTXO使用冷数据策略时，通过区块链回溯获取完整的TxOutput数据。
// 这是存储优化的一部分，用于节省热数据存储空间。
//
// 参数：
//   - ctx: 上下文对象
//   - outpoint: UTXO位置引用
//
// 返回值：
//   - *pbtx.TxOutput: 回溯获取的TxOutput对象
//   - error: 回溯错误
//
// ⚠️ **实现状态**：
// 当前为基础实现，需要repository层支持根据OutPoint获取历史TxOutput
// 实际实现可能需要访问区块存储、交易索引等底层服务
func (s *MiningTemplateService) fetchTxOutputFromBlockchain(
	ctx context.Context,
	outpoint *pbtx.OutPoint,
) (*pbtx.TxOutput, error) {
	if outpoint == nil {
		return nil, fmt.Errorf("输出点为空")
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("🔍 开始区块链回溯 - 交易: %x, 输出索引: %d",
			outpoint.TxId, outpoint.OutputIndex))
	}

	// ✅ 区块链回溯逻辑已实现
	// 通过Repository.GetTransaction接口从区块链获取历史交易
	// 实现完整的边界检查和错误处理

	// 通过Repository接口获取历史交易
	if s.repo == nil {
		return nil, fmt.Errorf("数据仓储接口未初始化")
	}

	// 从区块链获取完整交易
	_, _, historicalTx, err := s.repo.GetTransaction(ctx, outpoint.TxId)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(fmt.Sprintf("获取历史交易失败 - TxId: %x, 错误: %v", outpoint.TxId, err))
		}
		return nil, fmt.Errorf("获取历史交易失败: %w", err)
	}

	if historicalTx == nil {
		return nil, fmt.Errorf("历史交易不存在 - TxId: %x", outpoint.TxId)
	}

	// 检查输出索引边界
	if outpoint.OutputIndex >= uint32(len(historicalTx.Outputs)) {
		return nil, fmt.Errorf("输出索引越界 - 索引: %d, 总输出数: %d",
			outpoint.OutputIndex, len(historicalTx.Outputs))
	}

	// 获取目标输出
	targetOutput := historicalTx.Outputs[outpoint.OutputIndex]
	if targetOutput == nil {
		return nil, fmt.Errorf("目标输出为空 - 索引: %d", outpoint.OutputIndex)
	}

	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("✅ 成功从区块链回溯获取TxOutput - OutPoint: %x:%d",
			outpoint.TxId[:8], outpoint.OutputIndex))
	}

	return targetOutput, nil
}

// ============================================================================
//                              数据转换工具
// ============================================================================

// Uint64ToBytes 将uint64转换为字节数组
//
// 🎯 **数值序列化工具**
//
// 将uint64数值转换为8字节的字节数组，用于区块链数据序列化。
// 使用大端序（Big Endian）确保跨平台兼容性。
//
// 参数：
//   - value: 需要转换的uint64值
//
// 返回值：
//   - []byte: 8字节的大端序字节数组
func Uint64ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, value)
	return bytes
}

// BytesToUint64 将字节数组转换为uint64
//
// 🎯 **数值反序列化工具**
//
// 将8字节的字节数组转换为uint64数值，用于区块链数据反序列化。
// 使用大端序（Big Endian）确保跨平台兼容性。
//
// 参数：
//   - bytes: 8字节的字节数组
//
// 返回值：
//   - uint64: 转换后的数值
//   - error: 转换错误
func BytesToUint64(bytes []byte) (uint64, error) {
	if len(bytes) != 8 {
		return 0, fmt.Errorf("字节数组长度必须为8，实际: %d", len(bytes))
	}
	return binary.BigEndian.Uint64(bytes), nil
}
