// Package startup 实现区块链启动流程
//
// 🎯 **启动流程包 (Startup Flow Package)**
//
// 本包实现了区块链启动时的初始化逻辑，包括：
// - 创世区块检查和初始化
// - 启动时同步触发
//
// 🏗️ **设计原则**
// - 启动逻辑：属于启动流程，不是长期服务
// - 函数式设计：使用函数而不是服务，避免创建不必要的服务实例
// - 职责清晰：启动逻辑集中在一个地方
package startup

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/persistence/repair"
	"github.com/weisyn/v1/internal/core/tx/builder"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	blockif "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// InitializeGenesisIfNeeded 启动时检查并初始化创世区块
//
// 🎯 **启动流程函数**：负责在启动时检查并创建创世区块
//
// 这是一个启动函数，不是服务方法。
// 在 chain/module.go 的 fx.Invoke 中直接调用。
//
// 注意：此函数不支持创世区块索引完整性检查，建议使用 InitializeGenesisIfNeededWithStore
//
// 参数：
//   - ctx: 上下文对象
//   - queryService: 查询服务（检查链状态）
//   - blockProcessor: 区块处理器（处理创世区块，统一入口）
//   - genesisBuilder: 创世区块构建器（Block模块提供，公共接口）
//   - addressManager: 地址管理器（构建创世交易）
//   - genesisConfig: 创世配置
//   - logger: 日志服务
//
// 返回：
//   - bool: true表示创建了创世区块，false表示跳过
//   - error: 处理过程中的错误
func InitializeGenesisIfNeeded(
	ctx context.Context,
	queryService persistence.QueryService,
	blockProcessor blockif.BlockProcessor,
	genesisBuilder blockif.GenesisBlockBuilder,
	addressManager crypto.AddressManager,
	powEngine crypto.POWEngine,
	genesisConfig *types.GenesisConfig,
	logger log.Logger,
) (bool, error) {
	if logger != nil {
		logger.Debug("检查是否需要初始化创世区块")
	}

	// 1. 检查是否需要创世区块（不传store，跳过索引完整性检查）
	needed, err := needsGenesisBlock(ctx, queryService, nil, logger)
	if err != nil {
		return false, fmt.Errorf("检查创世需求失败: %w", err)
	}

	if !needed {
		if logger != nil {
			logger.Infof("链已初始化，跳过创世区块创建")
		}
		return false, nil
	}

	// 2. 协调构建创世区块（包括PoW挖矿）
	genesisBlock, err := buildGenesisBlock(
		ctx,
		genesisConfig,
		genesisBuilder,
		addressManager,
		powEngine,
		logger,
	)
	if err != nil {
		return false, fmt.Errorf("构建创世区块失败: %w", err)
	}

	// 3. 处理创世区块
	if err := processGenesisBlock(ctx, genesisBlock, blockProcessor, queryService, logger); err != nil {
		return false, fmt.Errorf("处理创世区块失败: %w", err)
	}

	if logger != nil {
		logger.Infof("🎉 创世区块初始化完成")
	}

	return true, nil
}

// InitializeGenesisIfNeededWithStore 带存储的创世区块初始化（用于持久化 genesis_hash）
//
// 🎯 **推荐使用**：支持创世区块索引完整性检查和自动修复
//
// 这是 InitializeGenesisIfNeeded 的扩展版本，额外功能包括：
// 1. 创世区块索引完整性检查
// 2. 创世区块创建成功后持久化 genesis_hash
// 3. 🆕 支持从blocks文件自动修复索引
//
// 参数：
//   - ctx: 上下文
//   - queryService: 查询服务
//   - blockProcessor: 区块处理器
//   - genesisBuilder: 创世区块构建器
//   - addressManager: 地址管理器
//   - powEngine: PoW 引擎
//   - genesisConfig: 创世配置
//   - store: BadgerStore（用于索引完整性检查和持久化 genesis_hash）
//   - fileStore: 文件存储（用于从blocks文件修复索引）
//   - blockHashClient: 区块哈希计算服务（用于修复索引）
//   - logger: 日志记录器
//
// 返回：
//   - bool: 是否创建了创世区块
//   - error: 处理过程中的错误
func InitializeGenesisIfNeededWithStore(
	ctx context.Context,
	queryService persistence.QueryService,
	blockProcessor blockif.BlockProcessor,
	genesisBuilder blockif.GenesisBlockBuilder,
	addressManager crypto.AddressManager,
	powEngine crypto.POWEngine,
	genesisConfig *types.GenesisConfig,
	store storage.BadgerStore,
	fileStore storage.FileStore,
	blockHashClient core.BlockHashServiceClient,
	logger log.Logger,
) (bool, error) {
	if logger != nil {
		logger.Debug("检查是否需要初始化创世区块（带索引完整性检查和自动修复）")
	}

	// 1. 检查是否需要创世区块或修复索引（传store+fileStore，支持索引完整性检查和文件存在性检查）
	checkResult, err := needsGenesisBlockV2(ctx, queryService, store, fileStore, logger)
	if err != nil {
		return false, fmt.Errorf("检查创世需求失败: %w", err)
	}

	// 2. 如果需要修复索引，调用修复器
	if checkResult.NeedsRepair {
		if logger != nil {
			logger.Info("🩹 开始修复创世区块索引...")
		}

		// 导入repair包并调用修复函数
		if err := repair.RepairGenesisIndex(ctx, store, fileStore, blockHashClient, logger); err != nil {
			return false, fmt.Errorf("修复创世区块索引失败: %w", err)
		}

		if logger != nil {
			logger.Info("✅ 创世区块索引修复完成")
		}

		// 修复完成，不需要创建区块
		return false, nil
	}

	// 3. 如果不需要创建区块，直接返回
	if !checkResult.NeedsCreate {
		if logger != nil {
			logger.Infof("链已初始化，跳过创世区块创建")
		}
		return false, nil
	}

	// 3.5 首次启动保护：清理可能被“查询侧修复策略”提前写入的链尖，避免 DataWriter 判定为重复创世
	// 说明：QueryService.GetCurrentHeight 具备“链尖修复兜底（策略3-创世）”，可能在创世流程之前写入 state:chain:tip。
	// 但“创世创建”必须由启动机制主导，不能被错误补偿机制抢先写入链尖。
	if store != nil {
		firstTime, err := isFirstTimeStartup(ctx, store, logger)
		if err != nil {
			return false, fmt.Errorf("检查首次启动状态失败: %w", err)
		}
		if firstTime {
			tipKey := []byte("state:chain:tip")
			if err := store.Delete(ctx, tipKey); err != nil {
				return false, fmt.Errorf("首次启动清理链尖失败: %w", err)
			}
			if logger != nil {
				logger.Infof("🧹 首次启动已清理链尖状态，确保可以写入创世区块: key=%s", string(tipKey))
			}
		}
	}

	// 4. 需要创建创世区块：协调构建创世区块（包括PoW挖矿）
	genesisBlock, err := buildGenesisBlock(
		ctx,
		genesisConfig,
		genesisBuilder,
		addressManager,
		powEngine,
		logger,
	)
	if err != nil {
		return false, fmt.Errorf("构建创世区块失败: %w", err)
	}

	// 5. 处理创世区块
	if err := processGenesisBlock(ctx, genesisBlock, blockProcessor, queryService, logger); err != nil {
		return false, fmt.Errorf("处理创世区块失败: %w", err)
	}

	// 6. 持久化 genesis_hash
	if store != nil {
		if err := PersistGenesisHash(ctx, store, genesisConfig); err != nil {
			if logger != nil {
				logger.Errorf("持久化 genesis hash 失败: %v", err)
			}
			return false, fmt.Errorf("持久化 genesis hash 失败: %w", err)
		}
		if logger != nil {
			logger.Info("✅ Genesis hash 已持久化到 metadata")
		}
	}

	if logger != nil {
		logger.Infof("🎉 创世区块初始化完成")
	}

	return true, nil
}

// GenesisCheckResult 创世区块检查结果
type GenesisCheckResult struct {
	NeedsCreate bool // 需要创建创世区块
	NeedsRepair bool // 需要修复索引
}

// isFirstTimeStartup 判断是否为首次启动（根据 genesis_hash 元数据）
//
// 这是判断“链是否已创建”的唯一权威方法：
// - genesis_hash 不存在/为空：首次启动，应创建创世区块
// - genesis_hash 存在：链已存在（即使索引损坏，也应走修复流程，而不是重新创世）
func isFirstTimeStartup(ctx context.Context, store storage.BadgerStore, logger log.Logger) (bool, error) {
	if store == nil {
		return false, fmt.Errorf("store 不能为空")
	}

	key := []byte(ChainIdentityMetadataKey)
	v, err := store.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("读取 genesis_hash 元数据失败: %w", err)
	}

	// BadgerStore.Get：键不存在时返回 (nil, nil)
	if len(v) == 0 {
		if logger != nil {
			logger.Info("🆕 未检测到 genesis_hash 元数据，判定为首次启动")
		}
		return true, nil
	}

	if logger != nil {
		genesisHash := string(v)
		logger.Infof("✅ 检测到 genesis_hash 元数据，链已存在: %s (前8位: %s)",
			genesisHash, genesisHash[:min(8, len(genesisHash))])
	}
	return false, nil
}

// needsGenesisBlockV2 检查是否需要创建或修复创世区块（新版本）
//
// 判断当前链状态：
// 1. 检查链是否已初始化（通过查询最高区块）
// 2. 检查是否存在高度为0的区块
// 3. 检查链状态的一致性
// 4. 🆕 检查创世区块索引完整性
// 5. 🆕 检查创世区块文件是否存在（区分首次启动与索引损坏）
//
// 返回：
//   - GenesisCheckResult: 检查结果
//   - error: 检查过程中的错误
func needsGenesisBlockV2(ctx context.Context, queryService persistence.QueryService, store storage.BadgerStore, fileStore storage.FileStore, logger log.Logger) (GenesisCheckResult, error) {
	result := GenesisCheckResult{}

	if logger != nil {
		logger.Debug("检查是否需要创建或修复创世区块")
	}

	// 兼容路径：当 store 未注入时，无法使用 genesis_hash 元数据作为权威判断；
	// 此时保持最小行为：仅根据 QueryService 判断是否需要创建创世区块，不做修复（NeedsRepair 恒为 false）。
	if store == nil {
		height, err := queryService.GetCurrentHeight(ctx)
		if err != nil {
			result.NeedsCreate = true
			return result, nil
		}
		if height == 0 {
			hash, herr := queryService.GetBestBlockHash(ctx)
			if herr != nil || len(hash) == 0 {
				result.NeedsCreate = true
				return result, nil
			}
		}
		return result, nil
	}

	// ============================================================
	// 阶段1：首次启动判断（主要机制）
	// ============================================================
	firstTime, err := isFirstTimeStartup(ctx, store, logger)
	if err != nil {
		return result, err
	}
	if firstTime {
		result.NeedsCreate = true
		return result, nil
	}

	// ============================================================
	// 阶段2：错误补偿机制（仅链已存在时才检查/修复索引）
	// ============================================================
	// 1) 获取最佳区块哈希（通常就是链尖哈希；链存在时应可读）
	hash, err := queryService.GetBestBlockHash(ctx)
	if err != nil || len(hash) == 0 {
		// 链已存在（genesis_hash 存在），但链尖哈希不可读：视为严重损坏 → 尝试走修复（如果 blocks 文件存在）
		if logger != nil {
			logger.Warnf("⚠️ 链已存在但无法获取最佳区块哈希，可能需要修复索引: err=%v len=%d", err, len(hash))
		}

		if fileStore != nil && store != nil {
			blockFilePath := "blocks/0000000000/0000000000.bin"
			if _, loadErr := fileStore.Load(ctx, blockFilePath); loadErr != nil {
				return result, fmt.Errorf("数据损坏：链已存在但创世区块文件缺失: %w", loadErr)
			}
			result.NeedsRepair = true
			return result, nil
		}

		return result, fmt.Errorf("链已存在但无法获取最佳区块哈希，且无法执行修复（store/fileStore 未注入）: %w", err)
	}

	// 2) 检查创世区块索引完整性（缺失/损坏则修复）
	if store != nil {
		needsRepair := checkGenesisIndexIntegrity(ctx, store, hash, logger)
		if needsRepair {
			if fileStore != nil {
				blockFilePath := "blocks/0000000000/0000000000.bin"
				if _, loadErr := fileStore.Load(ctx, blockFilePath); loadErr != nil {
					return result, fmt.Errorf("数据损坏：索引损坏且创世区块文件缺失: %w", loadErr)
				}
			}

			if logger != nil {
				logger.Warn("🩹 检测到创世区块索引损坏（链已存在，进入修复流程）")
			}
			result.NeedsRepair = true
			return result, nil
		}
	}

	// 3) 链已存在且索引完整 → 不需要任何操作
	return result, nil
}

// needsGenesisBlock 检查是否需要创建创世区块（兼容版本）
//
// ⚠️ **已废弃**: 建议使用 needsGenesisBlockV2 以支持索引修复
//
// 判断当前链状态是否需要创建创世区块：
// 1. 检查链是否已初始化（通过查询最高区块）
// 2. 检查是否存在高度为0的区块
// 3. 检查链状态的一致性
func needsGenesisBlock(ctx context.Context, queryService persistence.QueryService, store storage.BadgerStore, logger log.Logger) (bool, error) {
	// 兼容版本：传入 nil fileStore（无法检查文件存在性）
	result, err := needsGenesisBlockV2(ctx, queryService, store, nil, logger)
	if err != nil {
		return false, err
	}
	// 旧版本API：只要需要创建或修复，都返回true
	return result.NeedsCreate, nil
}

// checkGenesisIndexIntegrity 检查创世区块索引完整性
//
// 🎯 **启动门闸增强**：在启动时主动检测创世区块索引完整性
//
// 检查项：
// 1. indices:height:0 存在且格式正确（至少32字节hash）
// 2. indices:hash:<genesis_hash> 存在且指向高度0
//
// 返回：
//   - true: 需要修复
//   - false: 索引完整
func checkGenesisIndexIntegrity(ctx context.Context, store storage.BadgerStore, genesisHash []byte, logger log.Logger) bool {
	if store == nil || len(genesisHash) == 0 {
		return false // 无法检查，假设完整
	}

	// 检查 indices:height:0
	heightKey := []byte("indices:height:0")
	heightData, err := store.Get(ctx, heightKey)
	if err != nil {
		if logger != nil {
			logger.Warnf("🔍 创世区块高度索引缺失: key=%s err=%v", string(heightKey), err)
		}
		return true // 需要修复
	}
	if len(heightData) < 32 {
		if logger != nil {
			logger.Warnf("🔍 创世区块高度索引损坏: key=%s len=%d (expected>=32)", string(heightKey), len(heightData))
		}
		return true // 需要修复
	}

	// 检查 indices:hash:<genesis_hash>
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", genesisHash))
	hashData, err := store.Get(ctx, hashKey)
	if err != nil {
		if logger != nil {
			logger.Warnf("🔍 创世区块哈希索引缺失: key=%s err=%v", string(hashKey), err)
		}
		return true // 需要修复
	}
	if len(hashData) != 8 {
		if logger != nil {
			logger.Warnf("🔍 创世区块哈希索引损坏: key=%s len=%d (expected=8)", string(hashKey), len(hashData))
		}
		return true // 需要修复
	}

	// 验证哈希索引指向高度0
	indexedHeight := binary.BigEndian.Uint64(hashData)
	if indexedHeight != 0 {
		if logger != nil {
			logger.Warnf("🔍 创世区块哈希索引高度不匹配: expected=0 actual=%d", indexedHeight)
		}
		return true // 需要修复
	}

	if logger != nil {
		logger.Debug("✅ 创世区块索引完整性检查通过")
	}
	return false // 索引完整
}

// buildGenesisBlock 协调构建创世区块
//
// 🎯 **协调方法**：负责协调完整的创世区块构建流程
//
// 这是一个协调方法，负责协调完整的创世区块构建流程：
// 1. 验证创世配置的有效性
// 2. 创建创世交易（通过TX组件）
// 3. 调用BLOCK的GenesisBlockBuilder构建创世区块
// 4. 对创世区块进行PoW挖矿，找到满足难度要求的Nonce
// 5. 调用BLOCK的GenesisBlockBuilder验证创世区块
//
// ⚠️ **注意**：实际构建由BLOCK.GenesisBlockBuilder.CreateGenesisBlock()完成。
// 本方法负责协调构建流程，并在构建后进行PoW挖矿。
func buildGenesisBlock(
	ctx context.Context,
	genesisConfig *types.GenesisConfig,
	genesisBuilder blockif.GenesisBlockBuilder,
	addressManager crypto.AddressManager,
	powEngine crypto.POWEngine,
	logger log.Logger,
) (*core.Block, error) {
	if logger != nil {
		logger.Infof("开始创建创世区块...")
	}

	// 1. 验证创世配置
	if err := validateGenesisConfig(genesisConfig, logger); err != nil {
		return nil, fmt.Errorf("创世配置验证失败: %w", err)
	}

	// 2. 创建创世交易
	genesisTransactions, err := createGenesisTransactions(ctx, genesisConfig, addressManager, logger)
	if err != nil {
		return nil, fmt.Errorf("创建创世交易失败: %w", err)
	}

	if logger != nil {
		logger.Infof("创世交易创建完成，数量: %d", len(genesisTransactions))
	}

	// 3. 构建创世区块（通过构建器）
	genesisBlock, err := genesisBuilder.CreateGenesisBlock(ctx, genesisTransactions, genesisConfig)
	if err != nil {
		return nil, fmt.Errorf("构建创世区块失败: %w", err)
	}

	// 4. 对创世区块进行PoW挖矿，找到满足难度要求的Nonce
	if powEngine != nil {
		if logger != nil {
			logger.Infof("⛏️  开始对创世区块进行PoW挖矿（难度=%d，可能需要几秒到几分钟，请稍候）...", genesisBlock.Header.Difficulty)
		}
		minedHeader, err := powEngine.MineBlockHeader(ctx, genesisBlock.Header)
		if err != nil {
			return nil, fmt.Errorf("创世区块PoW挖矿失败: %w", err)
		}
		genesisBlock.Header = minedHeader
		if logger != nil {
			logger.Infof("✅ 创世区块PoW挖矿完成，Nonce=%x", minedHeader.Nonce)
		}
	} else {
		if logger != nil {
			logger.Warn("PoW引擎未注入，跳过创世区块挖矿（将无法通过PoW验证）")
		}
	}

	// 5. 验证创世区块（包括PoW验证）
	valid, err := genesisBuilder.ValidateGenesisBlock(ctx, genesisBlock)
	if err != nil {
		return nil, fmt.Errorf("验证创世区块失败: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("创世区块验证失败")
	}

	if logger != nil {
		logger.Infof("✅ 创世区块创建成功，高度: %d, 交易数: %d",
			genesisBlock.Header.Height, len(genesisTransactions))
	}

	return genesisBlock, nil
}

// processGenesisBlock 处理创世区块
//
// 🎯 **创世区块处理核心**
//
// 处理创世区块的完整流程：
// 1. 验证创世区块的有效性
// 2. 通过BlockProcessor处理创世区块（统一入口）
// 3. 验证创世后链状态
func processGenesisBlock(
	ctx context.Context,
	genesisBlock *core.Block,
	blockProcessor blockif.BlockProcessor,
	queryService persistence.QueryService,
	logger log.Logger,
) error {
	if logger != nil {
		logger.Infof("开始处理创世区块...")
	}

	// 1. 最终验证创世区块
	if err := validateCreatedGenesisBlock(genesisBlock); err != nil {
		return fmt.Errorf("创世区块最终验证失败: %w", err)
	}

	// 2. 通过BlockProcessor处理创世区块（统一入口，确保与其他区块一致）
	// BlockProcessor内部会调用DataWriter.WriteBlock()，并会发布BlockProcessed事件，
	// DataWriter会自动更新链尖，因此这里不需要手动更新链尖。
	if err := blockProcessor.ProcessBlock(ctx, genesisBlock); err != nil {
		return fmt.Errorf("处理创世区块失败: %w", err)
	}

	if logger != nil {
		logger.Info("✅ 创世区块已提交处理，等待异步事件处理完成...")
	}

	// 🔧 等待异步事件处理完成
	// 由于 BlockProcessed 事件采用异步订阅，需要给事件处理器一些时间来更新状态
	time.Sleep(200 * time.Millisecond)

	// 3. 验证创世后链状态
	if err := verifyGenesisState(ctx, queryService, logger); err != nil {
		return fmt.Errorf("创世后状态验证失败: %w", err)
	}

	if logger != nil {
		logger.Infof("✅ 创世区块处理完成")
	}

	return nil
}

// ============================================================================
//                              辅助函数
// ============================================================================

// createGenesisTransactions 基于创世配置通过TX Builder构建创世交易列表
//
// 🎯 **架构原则**：
// - CHAIN 调用 TX Builder 来创建交易，而不是直接构造 PROTO
// - 遵循组件边界，各组件各司其职
//
// 规则：
// - 构建单个 coinbase 交易，包含所有创世账户的资产输出
// - 每个输出为 NativeCoin，金额取自 InitialBalance
// - 锁定条件使用 SingleKeyLock，RequiredAddressHash 为账户地址哈希
func createGenesisTransactions(
	ctx context.Context,
	genesisConfig *types.GenesisConfig,
	addressManager crypto.AddressManager,
	logger log.Logger,
) ([]*transaction.Transaction, error) {
	if genesisConfig == nil {
		return nil, fmt.Errorf("创世配置不能为空")
	}

	if len(genesisConfig.GenesisAccounts) == 0 {
		return nil, fmt.Errorf("创世账户列表不能为空")
	}

	if addressManager == nil {
		return nil, fmt.Errorf("地址管理器未初始化，无法构建创世输出")
	}

	// 使用 TX Builder 创建交易，而不是直接构造 PROTO
	// 创建 TxBuilder 实例（创世交易不需要 Draft，传入 nil）
	txBuilder := builder.NewService(nil)

	// 链ID编码（8字节大端）
	chainIDBytes := make([]byte, 8)
	chainId := genesisConfig.ChainID
	chainIDBytes[0] = byte(chainId >> 56)
	chainIDBytes[1] = byte(chainId >> 48)
	chainIDBytes[2] = byte(chainId >> 40)
	chainIDBytes[3] = byte(chainId >> 32)
	chainIDBytes[4] = byte(chainId >> 24)
	chainIDBytes[5] = byte(chainId >> 16)
	chainIDBytes[6] = byte(chainId >> 8)
	chainIDBytes[7] = byte(chainId)

	// 设置链ID
	txBuilder.SetChainID(chainIDBytes)

	// 设置Nonce为0（创世交易）
	txBuilder.SetNonce(0)
	// 使用创世配置时间戳，确保多节点创世交易一致
	txBuilder.SetCreationTimestamp(uint64(genesisConfig.Timestamp))

	// 为每个创世账户添加资产输出
	for i, acc := range genesisConfig.GenesisAccounts {
		if acc.Address == "" || acc.InitialBalance == "" {
			return nil, fmt.Errorf("第%d个创世账户配置不完整", i)
		}

		addrBytes, err := addressManager.AddressToBytes(acc.Address)
		if err != nil || len(addrBytes) != 20 {
			return nil, fmt.Errorf("解析创世账户地址失败[%d]: name=%q address=%q err=%v", i, acc.Name, acc.Address, err)
		}

		// 构建锁定条件（SingleKeyLock）
		lock := &transaction.LockingCondition{
			Condition: &transaction.LockingCondition_SingleKeyLock{
				SingleKeyLock: &transaction.SingleKeyLock{
					KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: addrBytes,
					},
				},
			},
		}

		// 使用 TX Builder 添加资产输出（原生币，nil 表示原生币）
		txBuilder.AddAssetOutput(addrBytes, acc.InitialBalance, nil, lock)
	}

	// 构建交易（返回 ComposedTx）
	composedTx, err := txBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("构建创世交易失败: %w", err)
	}

	// 从 ComposedTx 提取 Transaction（ComposedTx.Tx 是公开字段）
	genesisTx := composedTx.Tx
	if genesisTx == nil {
		return nil, fmt.Errorf("构建的交易为空")
	}

	return []*transaction.Transaction{genesisTx}, nil
}

// validateGenesisConfig 验证创世配置
func validateGenesisConfig(config *types.GenesisConfig, logger log.Logger) error {
	if config == nil {
		return fmt.Errorf("创世配置不能为空")
	}

	if config.ChainID == 0 {
		return fmt.Errorf("链ID不能为0")
	}

	if config.NetworkID == "" {
		return fmt.Errorf("网络ID不能为空")
	}

	if config.Timestamp == 0 {
		return fmt.Errorf("时间戳不能为0")
	}

	// 验证创世账户配置
	if len(config.GenesisAccounts) == 0 {
		if logger != nil {
			logger.Warnf("创世配置中没有预设账户")
		}
	}

	for i, account := range config.GenesisAccounts {
		// 目前创世流程在构建交易时只依赖 Address + InitialBalance，
		// PublicKey / PrivateKey 主要用于配置与后续账户管理，不强制要求在此处全部给出。
		// createGenesisTransactions() 会对 Address 做更严格的校验。
		if account.InitialBalance == "" || account.InitialBalance == "0" {
			return fmt.Errorf("第%d个创世账户的初始余额不能为空或为0", i)
		}
	}

	return nil
}

// validateCreatedGenesisBlock 验证创建的创世区块
func validateCreatedGenesisBlock(block *core.Block) error {
	if block == nil {
		return fmt.Errorf("创世区块不能为空")
	}

	if block.Header == nil {
		return fmt.Errorf("创世区块头不能为空")
	}

	if block.Body == nil {
		return fmt.Errorf("创世区块体不能为空")
	}

	// 验证创世区块的特殊属性
	if block.Header.Height != 0 {
		return fmt.Errorf("创世区块高度必须为0，当前为: %d", block.Header.Height)
	}

	// 验证父区块哈希为全零
	if len(block.Header.PreviousHash) != 32 {
		return fmt.Errorf("创世区块父哈希长度必须为32字节，当前为: %d", len(block.Header.PreviousHash))
	}

	for _, b := range block.Header.PreviousHash {
		if b != 0 {
			return fmt.Errorf("创世区块父哈希必须为全零")
		}
	}

	if block.Header.Timestamp == 0 {
		return fmt.Errorf("创世区块时间戳不能为0")
	}

	return nil
}

// verifyGenesisState 验证创世后的链状态
func verifyGenesisState(ctx context.Context, queryService persistence.QueryService, logger log.Logger) error {
	if logger != nil {
		logger.Info("🔍 开始验证创世后链状态...")
	}

	// 1. 检查链是否已标记为初始化
	if logger != nil {
		logger.Info("🔍 正在获取链高度...")
	}
	height, err := queryService.GetCurrentHeight(ctx)
	if err != nil {
		if logger != nil {
			logger.Errorf("❌ 获取链高度失败: %v", err)
		}
		return fmt.Errorf("获取链高度失败: %w", err)
	}
	if logger != nil {
		logger.Infof("✅ 获取到链高度: %d", height)
	}

	if height != 0 {
		return fmt.Errorf("创世后链高度应该为0，当前为: %d", height)
	}

	if logger != nil {
		logger.Info("🔍 正在获取最佳区块哈希...")
	}
	hash, err := queryService.GetBestBlockHash(ctx)
	if err != nil {
		if logger != nil {
			logger.Errorf("❌ 获取最佳区块哈希失败: %v", err)
		}
		return fmt.Errorf("获取最佳区块哈希失败: %w", err)
	}
	if logger != nil {
		logger.Infof("✅ 获取到区块哈希，长度: %d", len(hash))
	}

	if len(hash) == 0 {
		return fmt.Errorf("创世后链哈希不能为空")
	}

	if logger != nil {
		logger.Infof("✅ 创世后链状态验证通过 - 高度: %d, 哈希: %x", height, hash[:min(8, len(hash))])
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
