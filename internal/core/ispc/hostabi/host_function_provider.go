package hostabi

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"

	"github.com/weisyn/v1/internal/core/ispc/hostabi/adapter"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ████████████████████████████████████████████████████████████████████████████████████████████
// HostFunctionProvider - 宿主函数提供者实现（有状态服务+无状态执行）
// ████████████████████████████████████████████████████████████████████████████████████████████
//
// 🎯 **设计目的**：
// 为 WASM/ONNX 引擎提供宿主函数映射（不再依赖外部接口）。
// 采用"有状态服务+无状态执行"设计：保存服务依赖，每次执行创建独立 HostABI。
//
// 🏗️ **实现策略**：
// - 有状态服务：保存底层服务依赖（chainService, utxoManager, repoManager, draftService）
// - 无状态执行：每次 GetWASMHostFunctions 调用时，基于 ExecutionContext 创建独立 HostABI
// - 闭包隔离：返回的函数闭包捕获 HostABI 实例，避免跨执行共享状态
//
// 🔒 **并发安全**：
// - 服务依赖不可变
// - 每次执行创建独立 HostABI 实例
// - HostABI 实例捕获在闭包中，无跨执行竞争
//
// ████████████████████████████████████████████████████████████████████████████████████████████

// HostFunctionProvider 宿主函数提供者实现（导出类型，供 module.go 类型断言使用）
type HostFunctionProvider struct {
	logger         log.Logger
	chainQuery     persistence.ChainQuery // 延迟注入（用于读操作）
	blockQuery     persistence.BlockQuery // 延迟注入（用于区块查询）
	eutxoQuery     persistence.UTXOQuery
	uresCAS        ures.CASStorage
	txQuery        persistence.TxQuery
	resourceQuery  persistence.ResourceQuery
	draftService   tx.TransactionDraftService
	txAdapter      TxAdapter                       // TX 适配器（用于 host_build_transaction）
	txHashClient   pb.TransactionHashServiceClient // 交易哈希服务客户端（用于计算交易哈希）
	addressManager crypto.AddressManager           // 地址管理器（用于 Base58Check 编码）
	hashManager    crypto.HashManager              // 哈希管理器（用于计算区块哈希）

	// 注意：内存分配器已迁移到 adapter/memory_allocator.go，不再在此处管理

	// P1: 原语调用缓存（可选）
	primitiveCache *PrimitiveCallCache
	cacheEnabled   bool
}

// HostFunctionProvider 不再实现外部接口，仅供 ISPC 内部使用

// SetChainQuery 设置链查询服务（延迟注入）
func (p *HostFunctionProvider) SetChainQuery(chainQuery persistence.ChainQuery) {
	p.chainQuery = chainQuery
}

// SetBlockQuery 设置区块查询服务（延迟注入）
func (p *HostFunctionProvider) SetBlockQuery(blockQuery persistence.BlockQuery) {
	p.blockQuery = blockQuery
}

// SetTxQuery 设置交易查询服务（延迟注入）
func (p *HostFunctionProvider) SetTxQuery(txQuery persistence.TxQuery) {
	p.txQuery = txQuery
}

// SetResourceQuery 设置资源查询服务（延迟注入）
func (p *HostFunctionProvider) SetResourceQuery(resourceQuery persistence.ResourceQuery) {
	p.resourceQuery = resourceQuery
}

// SetHashManager 设置哈希管理器（延迟注入）
func (p *HostFunctionProvider) SetHashManager(hashManager crypto.HashManager) {
	p.hashManager = hashManager
}

// SetTxAdapter 设置TX适配器（延迟注入）
func (p *HostFunctionProvider) SetTxAdapter(txAdapter TxAdapter) {
	p.txAdapter = txAdapter
}

// GetCacheStats 获取原语调用缓存统计信息
//
// 🎯 **缓存统计**：
// - 返回原语调用缓存的统计信息
// - 用于性能分析和缓存优化
//
// 📋 **返回值**：
//   - map[string]interface{}: 缓存统计信息（如果缓存未启用则返回nil）
func (p *HostFunctionProvider) GetCacheStats() map[string]interface{} {
	if !p.cacheEnabled || p.primitiveCache == nil {
		return nil
	}
	return p.primitiveCache.GetStats()
}

// ClearCache 清空原语调用缓存
//
// 🎯 **缓存清理**：
// - 清空所有缓存的原语调用结果
// - 重置缓存统计信息
func (p *HostFunctionProvider) ClearCache() {
	if p.primitiveCache != nil {
		p.primitiveCache.Clear()
		if p.logger != nil {
			p.logger.Info("✅ 原语调用缓存已清空")
		}
	}
}

// NewHostFunctionProvider 创建宿主函数提供者
//
// 📋 **参数**：
//   - logger: 日志服务
//   - utxoManager: UTXO 管理器
//   - repoManager: 仓储管理器
//   - draftService: 交易草稿服务
//   - txAdapter: TX 适配器
//   - txHashClient: 交易哈希服务客户端（用于计算交易哈希）
//   - addressManager: 地址管理器
//
// 🔧 **返回值**：
//   - *HostFunctionProvider: 提供者实例
//
// 🎯 **用途**：由 ISPC module 创建并用于内部引擎
func NewHostFunctionProvider(
	logger log.Logger,
	eutxoQuery persistence.UTXOQuery,
	uresCAS ures.CASStorage,
	draftService tx.TransactionDraftService,
	txAdapter TxAdapter,
	txHashClient pb.TransactionHashServiceClient,
	addressManager crypto.AddressManager,
) *HostFunctionProvider {
	return NewHostFunctionProviderWithCache(logger, eutxoQuery, uresCAS, draftService, txAdapter, txHashClient, addressManager, true, 500, 1*time.Minute)
}

// NewHostFunctionProviderWithCache 创建宿主函数提供者（带缓存配置）
//
// 📋 **参数**：
//   - logger: 日志服务
//   - eutxoQuery: UTXO查询服务
//   - uresCAS: 资源存储服务
//   - draftService: 交易草稿服务
//   - txAdapter: TX适配器
//   - txHashClient: 交易哈希服务客户端
//   - addressManager: 地址管理器
//   - enableCache: 是否启用原语调用缓存
//   - cacheSize: 缓存最大条目数
//   - cacheTTL: 缓存生存时间
//
// 🔧 **返回值**：
//   - *HostFunctionProvider: 提供者实例
func NewHostFunctionProviderWithCache(
	logger log.Logger,
	eutxoQuery persistence.UTXOQuery,
	uresCAS ures.CASStorage,
	draftService tx.TransactionDraftService,
	txAdapter TxAdapter,
	txHashClient pb.TransactionHashServiceClient,
	addressManager crypto.AddressManager,
	enableCache bool,
	cacheSize int,
	cacheTTL time.Duration,
) *HostFunctionProvider {
	provider := &HostFunctionProvider{
		logger:         logger,
		eutxoQuery:     eutxoQuery,
		uresCAS:        uresCAS,
		draftService:   draftService,
		txAdapter:      txAdapter,
		txHashClient:   txHashClient,
		addressManager: addressManager,
		// 注意：allocators 字段已删除，内存分配器已迁移到 adapter/memory_allocator.go
		cacheEnabled: enableCache,
	}

	// 初始化原语调用缓存
	if enableCache {
		provider.primitiveCache = NewPrimitiveCallCache(logger, cacheSize, cacheTTL)
		if logger != nil {
			logger.Infof("✅ HostABI原语调用缓存已启用: size=%d, ttl=%v", cacheSize, cacheTTL)
		}
	}

	return provider
}

// SetExecutionContext 设置执行上下文（废弃）
//
// ⚠️ **废弃说明**：
// 该方法已废弃，保留仅为向后兼容。
// 新的设计中，ExecutionContext 通过 context.Context 传递（使用 context.WithValue），
// 而不是预先设置，这样可以确保无状态和并发安全。
//
// 🔧 **返回值**：
// 总是空操作（兼容接口签名，但不执行任何操作）
func (p *HostFunctionProvider) SetExecutionContext(ctx interface{}) {
	// 废弃方法：什么都不做
	// 新的设计中，ExecutionContext 通过 context.WithValue 在调用时传递
}

// contextKey 是用于在 context.Context 中传递 ExecutionContext 的键类型
type contextKey string

const executionContextKey contextKey = "execution_context"

// WithExecutionContext 将 ExecutionContext 注入到 context.Context 中
//
// 📋 **参数**：
//   - ctx: 父 context.Context
//   - execCtx: 执行上下文实例
//
// 🔧 **返回值**：
//   - context.Context: 包含 ExecutionContext 的新 context
//
// 🎯 **用途**：由 ISPC Coordinator 在调用 GetWASMHostFunctions 前调用
func WithExecutionContext(ctx context.Context, execCtx ispcInterfaces.ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey, execCtx)
}

// GetExecutionContext 从 context.Context 中提取 ExecutionContext
//
// ⚠️ **关键函数**：宿主函数应该调用此函数动态获取 ExecutionContext
// 原因：env 模块只能实例化一次，宿主函数闭包捕获会导致第二次调用使用旧的 ExecutionContext
//
// 📋 **参数**：
//   - ctx: context.Context（应该包含 ExecutionContext）
//
// 🔧 **返回值**：
//   - ExecutionContext 实例，如果不存在则返回 nil
func GetExecutionContext(ctx context.Context) ispcInterfaces.ExecutionContext {
	execCtxRaw := ctx.Value(executionContextKey)
	if execCtxRaw == nil {
		return nil
	}
	execCtx, ok := execCtxRaw.(ispcInterfaces.ExecutionContext)
	if !ok {
		return nil
	}
	return execCtx
}

// GetWASMHostFunctions 获取 WASM 宿主函数映射
//
// 📋 **参数**：
//   - ctx: 调用上下文（必须包含 ExecutionContext，通过 WithExecutionContext 注入）
//   - executionID: 执行上下文标识符（用于日志和调试）
//
// 🔧 **返回值**：
//   - map[string]interface{}: 宿主函数映射（17个最小原语）
//   - error: 构造失败时的错误
//
// 🎯 **实现说明**：
// 1. 从 ctx 中提取 ExecutionContext
// 2. 使用 ExecutionContext + 底层服务创建 HostABI 实例
// 3. 基于 HostABI 构建 WASM 兼容的闭包函数映射
//
// ⚠️ **重要**：
// - ctx 必须通过 WithExecutionContext 注入 ExecutionContext
// - 返回的函数映射捕获 HostABI 实例，确保状态隔离
// - 每次调用创建新的 HostABI 实例，保证并发安全
func (p *HostFunctionProvider) GetWASMHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error) {
	// 从 context 中提取 ExecutionContext
	execCtxRaw := ctx.Value(executionContextKey)
	if execCtxRaw == nil {
		return nil, fmt.Errorf("ExecutionContext 未在 context 中设置，请先调用 WithExecutionContext")
	}

	execCtx, ok := execCtxRaw.(ispcInterfaces.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context 中的 ExecutionContext 类型不正确")
	}

	// 创建 HostABI 实例（每次执行创建独立实例）
	// 使用 chainQuery 而不是 chainService（因为 HostABI 只需要读操作）
	if p.chainQuery == nil {
		return nil, fmt.Errorf("chainQuery 未设置，请先调用 SetChainQuery")
	}
	hostABI, err := NewHostRuntimePorts(
		p.logger,
		p.chainQuery,
		p.blockQuery,
		p.eutxoQuery,
		p.uresCAS,
		p.txQuery,
		p.resourceQuery,
		p.draftService,
		p.hashManager,
		execCtx,
	)
	if err != nil {
		return nil, fmt.Errorf("创建 HostABI 实例失败: %w", err)
	}

	// P1: 如果启用缓存，用缓存包装器包装HostABI
	if p.cacheEnabled && p.primitiveCache != nil {
		hostABI = NewHostRuntimePortsWithCache(hostABI, p.primitiveCache, executionID, p.logger, p.hashManager)
		if p.logger != nil {
			p.logger.Debugf("✅ HostABI已启用缓存包装: executionID=%s", executionID)
		}
	}

	if p.logger != nil {
		p.logger.Debugf("✅ 为执行 %s 创建宿主函数映射（28个函数，含 host_build_transaction）", executionID)
	}

	// ✅ 使用适配器构建WASM宿主函数映射
	// 符合架构约束：适配器负责构建宿主函数映射，provider只负责协调
	// 注意：p.txAdapter是hostabi.TxAdapter类型，需要通过适配函数桥接
	wasmAdapter := adapter.NewWASMAdapter(
		p.logger,
		p.chainQuery,
		p.blockQuery,
		p.eutxoQuery,
		p.uresCAS,
		p.txQuery,
		p.resourceQuery,
		p.txHashClient,
		p.addressManager,
		p.hashManager,
		p.txAdapter, // 传递interface{}，适配器内部会处理
		p.draftService,
		GetExecutionContext, // 从context提取ExecutionContext的函数
		// 适配函数：将hostabi的函数适配为适配器需要的签名
		// 注意：函数签名的第一个参数是interface{}（为了匹配适配器的类型定义），
		// 但实际使用p.txAdapter（hostabi.TxAdapter）而非参数中的txAdapter
		func(ctx context.Context, txAdapter interface{}, txHashClient pb.TransactionHashServiceClient, eutxoQuery persistence.UTXOQuery, callerAddr []byte, contractAddr []byte, draftJSONBytes []byte, blockHeight uint64, blockTimestamp uint64) (*adapter.TxReceipt, error) {
			// 直接使用p.txAdapter（hostabi.TxAdapter）而非参数中的txAdapter
			// 因为参数txAdapter是interface{}类型（仅用于匹配函数签名），
			// 而BuildTransactionFromDraft需要hostabi.TxAdapter类型
			if p.logger != nil {
				p.logger.Infof("tx_draft_debug: %s", string(draftJSONBytes))
			}
			receipt, err := BuildTransactionFromDraft(ctx, p.txAdapter, txHashClient, eutxoQuery, callerAddr, contractAddr, draftJSONBytes, blockHeight, blockTimestamp)
			if err != nil {
				return nil, err
			}
			// 类型转换：hostabi.TxReceipt -> adapter.TxReceipt
			return &adapter.TxReceipt{
				Mode:           receipt.Mode,
				UnsignedTxHash: receipt.UnsignedTxHash,
				SignedTxHash:   receipt.SignedTxHash,
				SerializedTx:   receipt.SerializedTx,
				ProposalID:     receipt.ProposalID,
				Error:          receipt.Error,
			}, nil
		},
		// 适配函数：将hostabi.EncodeTxReceipt适配为适配器需要的签名
		func(receipt *adapter.TxReceipt) ([]byte, error) {
			// 类型转换：adapter.TxReceipt -> hostabi.TxReceipt
			hostabiReceipt := &TxReceipt{
				Mode:           receipt.Mode,
				UnsignedTxHash: receipt.UnsignedTxHash,
				SignedTxHash:   receipt.SignedTxHash,
				SerializedTx:   receipt.SerializedTx,
				ProposalID:     receipt.ProposalID,
				Error:          receipt.Error,
			}
			return EncodeTxReceipt(hostabiReceipt)
		},
	)
	return wasmAdapter.BuildHostFunctions(ctx, hostABI), nil
}

// GetONNXHostFunctions 获取 ONNX 宿主函数映射
//
// 📋 **参数**：
//   - ctx: 调用上下文（必须包含 ExecutionContext，通过 WithExecutionContext 注入）
//   - executionID: 执行上下文标识符
//
// 🔧 **返回值**：
//   - map[string]interface{}: ONNX 宿主函数映射（最小只读集合）
//   - error: 构造失败时的错误
//
// 🎯 **实现说明**：
// ONNX 模型推理主要是纯计算任务，但提供一个最小的只读宿主函数集：
//   - 确定性区块视图（get_block_height, get_block_timestamp）
//   - UTXO 查询（utxo_exists - 轻量级）
//   - 资源查询（resource_exists - 用于加载模型依赖）
//
// ⚠️ **设计约束**：
//   - 只提供只读查询，不提供任何写操作
//   - 不提供交易草稿操作（ONNX 模型不构建交易）
//   - 参数和返回值使用 Go 原生类型（float64, int64, bool）
//
// 📋 **ONNX 宿主函数集合（5个最小原语）**：
//  1. get_block_height() -> int64
//  2. get_block_timestamp() -> int64
//  3. utxo_exists(txHash []byte, index uint32) -> bool
//  4. resource_exists(contentHash []byte) -> bool
//  5. get_chain_id() -> []byte
func (p *HostFunctionProvider) GetONNXHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error) {
	// 从 context 中提取 ExecutionContext
	execCtxRaw := ctx.Value(executionContextKey)
	if execCtxRaw == nil {
		// ONNX 宿主函数是可选的，如果没有执行上下文，返回空映射
		if p.logger != nil {
			p.logger.Debugf("⚠️ ONNX 执行 %s 未提供 ExecutionContext，返回空宿主函数集", executionID)
		}
		return make(map[string]interface{}), nil
	}

	execCtx, ok := execCtxRaw.(ispcInterfaces.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context 中的 ExecutionContext 类型不正确")
	}

	// 创建 HostABI 实例（复用 WASM 的 HostABI）
	// 使用 chainQuery 而不是 chainService（因为 HostABI 只需要读操作）
	if p.chainQuery == nil {
		return nil, fmt.Errorf("chainQuery 未设置，请先调用 SetChainQuery")
	}
	hostABI, err := NewHostRuntimePorts(
		p.logger,
		p.chainQuery,
		p.blockQuery,
		p.eutxoQuery,
		p.uresCAS,
		p.txQuery,
		p.resourceQuery,
		p.draftService,
		p.hashManager,
		execCtx,
	)
	if err != nil {
		return nil, fmt.Errorf("创建 HostABI 实例失败: %w", err)
	}

	// P1: 如果启用缓存，用缓存包装器包装HostABI
	if p.cacheEnabled && p.primitiveCache != nil {
		hostABI = NewHostRuntimePortsWithCache(hostABI, p.primitiveCache, executionID, p.logger, p.hashManager)
		if p.logger != nil {
			p.logger.Debugf("✅ HostABI已启用缓存包装: executionID=%s", executionID)
		}
	}

	if p.logger != nil {
		p.logger.Debugf("✅ 为 ONNX 执行 %s 创建宿主函数映射（5个最小只读原语）", executionID)
	}

	// ✅ 使用适配器构建ONNX宿主函数映射
	// 符合架构约束：适配器负责构建宿主函数映射，provider只负责协调
	onnxAdapter := adapter.NewONNXAdapter()
	return onnxAdapter.BuildHostFunctions(ctx, hostABI), nil
}

// 编译时检查：HostFunctionProvider 实现了内部接口层的 HostFunctionProvider 能力接口。
var _ ispcInterfaces.HostFunctionProvider = (*HostFunctionProvider)(nil)

