package hostabi

import (
	"context"
	"fmt"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"

	// 内部接口
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"

	// Protobuf 定义
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"google.golang.org/protobuf/proto"
)

// ████████████████████████████████████████████████████████████████████████████████████████████
// HostRuntimePorts - HostABI 实现（ISPC 运行期宿主能力端口）
// ████████████████████████████████████████████████████████████████████████████████████████████
//
// 🎯 **设计目的**：
// 实现 publicispc.HostABI 接口，提供 17 个最小原语，无业务语义。
// 委托模式：不直接操作 DraftTx，而是委托给 TransactionDraftService。
//
// 🏗️ **实现策略**：
// - 类别 A（链上上下文）：委托给 chainService + execCtx
// - 类别 B（UTXO 查询）：委托给 utxoManager
// - 类别 C（交易草稿）：委托给 draftService
// - 类别 D（资源查询）：委托给 repoManager
//
// 🔒 **并发安全**：
// - 每次执行创建独立实例
// - 底层服务自身保证并发安全
// - ExecutionContext 操作单次执行的草稿，无跨执行竞争
//
// ████████████████████████████████████████████████████████████████████████████████████████████

// HostRuntimePorts HostABI 实现
type HostRuntimePorts struct {
	logger log.Logger

	// 底层服务
	chainQuery    persistence.ChainQuery
	blockQuery    persistence.BlockQuery
	eutxoQuery    persistence.UTXOQuery
	uresCAS       ures.CASStorage
	txQuery       persistence.TxQuery
	resourceQuery persistence.ResourceQuery
	draftService  tx.TransactionDraftService
	hashManager   crypto.HashManager // 哈希管理器（用于计算区块哈希）

	// 执行上下文（提供确定性区块视图）
	execCtx ispcInterfaces.ExecutionContext
}

// 确保实现接口（公共接口类型）
var _ publicispc.HostABI = (*HostRuntimePorts)(nil)

// NewHostRuntimePorts 创建 HostABI 实现
//
// 📋 **参数**：
//   - logger: 日志服务
//   - chainQuery: 链查询服务
//   - eutxoQuery: UTXO查询服务
//   - uresCAS: 资源存储服务
//   - txQuery: 交易查询服务
//   - resourceQuery: 资源查询服务
//   - draftService: 交易草稿构建服务
//   - hashManager: 哈希管理器（用于计算区块哈希）
//   - execCtx: 当前执行上下文
//
// 🔧 **返回值**：
//   - publicispc.HostABI: HostABI 实例
//   - error: 创建失败时的错误信息
//
// 🎯 **用途**：由 ISPC Coordinator 在每次执行前创建并注入到 ExecutionContext
func NewHostRuntimePorts(
	logger log.Logger,
	chainQuery persistence.ChainQuery,
	blockQuery persistence.BlockQuery,
	eutxoQuery persistence.UTXOQuery,
	uresCAS ures.CASStorage,
	txQuery persistence.TxQuery,
	resourceQuery persistence.ResourceQuery,
	draftService tx.TransactionDraftService,
	hashManager crypto.HashManager,
	execCtx ispcInterfaces.ExecutionContext,
) (publicispc.HostABI, error) {
	if chainQuery == nil {
		return nil, fmt.Errorf("chainQuery 不能为 nil")
	}
	if blockQuery == nil {
		return nil, fmt.Errorf("blockQuery 不能为 nil")
	}
	if eutxoQuery == nil {
		return nil, fmt.Errorf("eutxoQuery 不能为 nil")
	}
	if uresCAS == nil {
		return nil, fmt.Errorf("uresCAS 不能为 nil")
	}
	if txQuery == nil {
		return nil, fmt.Errorf("txQuery 不能为 nil")
	}
	if resourceQuery == nil {
		return nil, fmt.Errorf("resourceQuery 不能为 nil")
	}
	if draftService == nil {
		return nil, fmt.Errorf("draftService 不能为 nil")
	}
	if hashManager == nil {
		return nil, fmt.Errorf("hashManager 不能为 nil")
	}
	if execCtx == nil {
		return nil, fmt.Errorf("执行上下文不能为 nil")
	}

	if logger != nil {
		logger.Debug("✅ HostRuntimePorts 创建成功（17个最小原语，无业务语义）")
	}

	return &HostRuntimePorts{
		logger:        logger,
		chainQuery:    chainQuery,
		blockQuery:    blockQuery,
		eutxoQuery:    eutxoQuery,
		uresCAS:       uresCAS,
		txQuery:       txQuery,
		resourceQuery: resourceQuery,
		draftService:  draftService,
		hashManager:   hashManager,
		execCtx:       execCtx,
	}, nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 类别 A：链上上下文查询（只读）- 7个原语
// ════════════════════════════════════════════════════════════════════════════════════════════════

// GetBlockHeight 获取当前区块高度（确定性快照）
func (h *HostRuntimePorts) GetBlockHeight(ctx context.Context) (uint64, error) {
	// 从 chainService 获取当前区块高度（与旧CLI一致）
	info, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return 0, fmt.Errorf("获取链信息失败: %w", err)
	}

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetBlockHeight height=%d", info.Height)
	}

	return info.Height, nil
}

// GetBlockHash 获取指定高度的区块哈希
func (h *HostRuntimePorts) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	// 获取链信息，检查是否为最新区块
	chainInfo, err := h.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取链信息失败: %w", err)
	}

	// 如果请求的是最新区块，直接返回
	if height == chainInfo.Height {
		if h.logger != nil {
			h.logger.Debugf("HostABI.GetBlockHash height=%d hash=%x", height, chainInfo.BestBlockHash[:8])
		}
		return chainInfo.BestBlockHash, nil
	}

	// 历史区块查询：通过 BlockQuery 查询并计算哈希
	block, err := h.blockQuery.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("查询历史区块失败: height=%d, error=%w", height, err)
	}

	if block == nil || block.Header == nil {
		return nil, fmt.Errorf("区块不存在或区块头为空: height=%d", height)
	}

	// 计算区块哈希：序列化区块头，然后使用HashManager.DoubleSHA256计算
	headerBytes, err := proto.Marshal(block.Header)
	if err != nil {
		return nil, fmt.Errorf("序列化区块头失败: height=%d, error=%w", height, err)
	}

	// 使用HashManager计算DoubleSHA256哈希（与挖矿保持一致）
	blockHash := h.hashManager.DoubleSHA256(headerBytes)

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetBlockHash height=%d hash=%x (历史区块)", height, blockHash[:min(8, len(blockHash))])
	}
	return blockHash, nil
}

// min 返回两个整数中的较小值（辅助函数）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetBlockTimestamp 获取当前区块时间戳（确定性快照）
func (h *HostRuntimePorts) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	// 从 ExecutionContext 获取固定快照
	timestamp := h.execCtx.GetBlockTimestamp()

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetBlockTimestamp timestamp=%d", timestamp)
	}

	return timestamp, nil
}

// GetChainID 获取链标识
func (h *HostRuntimePorts) GetChainID(ctx context.Context) ([]byte, error) {
	// 从 ExecutionContext 获取
	chainID := h.execCtx.GetChainID()

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetChainID chainID=%x", chainID)
	}

	return chainID, nil
}

// GetCaller 获取调用者地址
func (h *HostRuntimePorts) GetCaller(ctx context.Context) ([]byte, error) {
	// 从 ExecutionContext 获取
	caller := h.execCtx.GetCallerAddress()

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetCaller caller=%x", caller)
	}

	return caller, nil
}

// GetContractAddress 获取当前合约地址
func (h *HostRuntimePorts) GetContractAddress(ctx context.Context) ([]byte, error) {
	// 从 ExecutionContext 获取
	contractAddr := h.execCtx.GetContractAddress()

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetContractAddress contract=%x", contractAddr)
	}

	return contractAddr, nil
}

// GetTransactionID 获取当前交易ID
func (h *HostRuntimePorts) GetTransactionID(ctx context.Context) ([]byte, error) {
	// 从 ExecutionContext 获取
	txID := h.execCtx.GetTransactionID()

	if h.logger != nil {
		h.logger.Debugf("HostABI.GetTransactionID txID=%x", txID)
	}

	return txID, nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 类别 B：UTXO 查询（只读）- 2个原语
// ════════════════════════════════════════════════════════════════════════════════════════════════

// UTXOLookup 查询指定 UTXO
func (h *HostRuntimePorts) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	if outpoint == nil {
		return nil, fmt.Errorf("outpoint 不能为 nil")
	}

	// 委托给 eutxoQuery
	utxo, err := h.eutxoQuery.GetUTXO(ctx, outpoint)
	if err != nil {
		return nil, fmt.Errorf("查询 UTXO 失败: %w", err)
	}

	// 将 UTXO 转换为 TxOutput
	if utxo == nil {
		return nil, fmt.Errorf("UTXO 不存在")
	}

	// 从 UTXO 提取 TxOutput
	// 支持两种存储策略：
	// 1. CachedOutput：直接返回缓存的输出（热数据）
	// 2. ReferenceOnly：从区块链回溯获取（冷数据）
	var txOutput *pb.TxOutput

	// 尝试获取缓存的输出（热数据）
	if cachedOutput := utxo.GetCachedOutput(); cachedOutput != nil {
		txOutput = cachedOutput
		if h.logger != nil {
			h.logger.Debugf("HostABI.UTXOLookup 使用热数据缓存 - UTXO: %x:%d", outpoint.TxId[:8], outpoint.OutputIndex)
		}
	} else if utxo.GetReferenceOnly() {
		// 从区块链回溯获取（冷数据）
		if h.txQuery == nil {
			return nil, fmt.Errorf("txQuery 未初始化，无法回溯获取UTXO")
		}
		_, _, transaction, err := h.txQuery.GetTransaction(ctx, outpoint.TxId)
		if err != nil {
			return nil, fmt.Errorf("获取历史交易失败 [%x]: %w", outpoint.TxId, err)
		}
		if transaction == nil {
			return nil, fmt.Errorf("历史交易不存在 [%x]", outpoint.TxId)
		}
		if outpoint.OutputIndex >= uint32(len(transaction.Outputs)) {
			return nil, fmt.Errorf("输出索引越界 - 索引: %d, 总输出数: %d", outpoint.OutputIndex, len(transaction.Outputs))
		}
		txOutput = transaction.Outputs[outpoint.OutputIndex]
		if txOutput == nil {
			return nil, fmt.Errorf("目标输出为空 - 索引: %d", outpoint.OutputIndex)
		}
		if h.logger != nil {
			h.logger.Debugf("HostABI.UTXOLookup 使用冷数据回溯 - UTXO: %x:%d", outpoint.TxId[:8], outpoint.OutputIndex)
		}
	} else {
		return nil, fmt.Errorf("UTXO存储策略无效：既没有缓存输出也不是引用模式")
	}

	if h.logger != nil {
		h.logger.Debugf("HostABI.UTXOLookup 成功: txId=%x index=%d", outpoint.TxId[:8], outpoint.OutputIndex)
	}

	// P0: 更新资源使用统计（UTXO查询）
	if h.execCtx != nil {
		if usage := h.execCtx.GetResourceUsage(); usage != nil {
			usage.UTXOQueries++
		}
	}

	return txOutput, nil
}

// UTXOExists 检查 UTXO 是否存在
func (h *HostRuntimePorts) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	if outpoint == nil {
		return false, fmt.Errorf("outpoint 不能为 nil")
	}

	// 委托给 eutxoQuery - 尝试查询
	utxo, err := h.eutxoQuery.GetUTXO(ctx, outpoint)
	if err != nil {
		// 如果是"不存在"错误，返回 false
		return false, nil
	}

	exists := utxo != nil

	if h.logger != nil {
		h.logger.Debugf("HostABI.UTXOExists txId=%x index=%d exists=%v", outpoint.TxId[:8], outpoint.OutputIndex, exists)
	}

	return exists, nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 类别 C：交易草稿操作（副作用）- 4个原语
// ════════════════════════════════════════════════════════════════════════════════════════════════

// TxAddInput 添加交易输入
func (h *HostRuntimePorts) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	if outpoint == nil {
		return 0, fmt.Errorf("outpoint 不能为 nil")
	}

	// 获取 DraftID 和 Draft
	draftID := h.execCtx.GetDraftID()
	draft, err := h.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return 0, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 委托给 draftService
	index, err := h.draftService.AddInput(ctx, draft, outpoint, isReferenceOnly, unlockingProof)
	if err != nil {
		return 0, fmt.Errorf("追加输入失败: %w", err)
	}

	// 保存草稿
	if err := h.draftService.SaveDraft(ctx, draft); err != nil {
		return 0, fmt.Errorf("保存草稿失败: %w", err)
	}

	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "TxAddInput",
		Parameters: map[string]interface{}{
			"outpoint":        outpoint,
			"isReferenceOnly": isReferenceOnly,
		},
		Result: map[string]interface{}{
			"index": index,
		},
	})

	if h.logger != nil {
		h.logger.Debugf("HostABI.TxAddInput draftID=%s index=%d isRefOnly=%v", draftID, index, isReferenceOnly)
	}

	return index, nil
}

// TxAddAssetOutput 添加资产输出
func (h *HostRuntimePorts) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	if len(owner) != 20 {
		return 0, fmt.Errorf("owner 地址必须是 20 字节")
	}

	// 获取 DraftID 和 Draft
	draftID := h.execCtx.GetDraftID()
	draft, err := h.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return 0, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 委托给 draftService
	index, err := h.draftService.AddAssetOutput(ctx, draft, owner, fmt.Sprintf("%d", amount), tokenID, lockingConditions)
	if err != nil {
		return 0, fmt.Errorf("追加资产输出失败: %w", err)
	}

	// ✅ 如果是合约代币，必须设置 contractAddress 为当前执行合约的地址
	if len(tokenID) > 0 {
		contractAddr := h.execCtx.GetContractAddress()
		if len(contractAddr) == 0 {
			return 0, fmt.Errorf("无法获取合约地址（创建合约代币输出需要）")
		}
		if len(contractAddr) != 20 {
			return 0, fmt.Errorf("合约地址必须是 20 字节，实际: %d", len(contractAddr))
		}
		// 设置 contractAddress 到刚创建的输出
		if index < uint32(len(draft.Tx.Outputs)) {
			output := draft.Tx.Outputs[index]
			if asset := output.GetAsset(); asset != nil {
				if contractToken := asset.GetContractToken(); contractToken != nil {
					contractToken.ContractAddress = contractAddr
				}
			}
		}
	}

	// 保存草稿
	if err := h.draftService.SaveDraft(ctx, draft); err != nil {
		return 0, fmt.Errorf("保存草稿失败: %w", err)
	}

	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "TxAddAssetOutput",
		Parameters: map[string]interface{}{
			"owner":  owner,
			"amount": amount,
		},
		Result: map[string]interface{}{
			"index": index,
		},
	})

	if h.logger != nil {
		h.logger.Debugf("HostABI.TxAddAssetOutput draftID=%s index=%d amount=%d", draftID, index, amount)
	}

	return index, nil
}

// TxAddResourceOutput 添加资源输出
func (h *HostRuntimePorts) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	if len(contentHash) != 32 {
		return 0, fmt.Errorf("contentHash 必须是 32 字节")
	}
	if len(owner) != 20 {
		return 0, fmt.Errorf("owner 地址必须是 20 字节")
	}

	// 获取 DraftID 和 Draft
	draftID := h.execCtx.GetDraftID()
	draft, err := h.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return 0, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 委托给 draftService
	index, err := h.draftService.AddResourceOutput(ctx, draft, contentHash, category, owner, lockingConditions, metadata)
	if err != nil {
		return 0, fmt.Errorf("追加资源输出失败: %w", err)
	}

	// 保存草稿
	if err := h.draftService.SaveDraft(ctx, draft); err != nil {
		return 0, fmt.Errorf("保存草稿失败: %w", err)
	}

	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "TxAddResourceOutput",
		Parameters: map[string]interface{}{
			"contentHash": contentHash,
			"category":    category,
		},
		Result: map[string]interface{}{
			"index": index,
		},
	})

	if h.logger != nil {
		h.logger.Debugf("HostABI.TxAddResourceOutput draftID=%s index=%d contentHash=%x", draftID, index, contentHash[:8])
	}

	return index, nil
}

// TxAddStateOutput 添加状态输出
func (h *HostRuntimePorts) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	if len(stateID) == 0 {
		return 0, fmt.Errorf("stateID 不能为空")
	}
	if len(executionResultHash) != 32 {
		return 0, fmt.Errorf("executionResultHash 必须是 32 字节")
	}

	// 获取 DraftID 和 Draft
	draftID := h.execCtx.GetDraftID()
	draft, err := h.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return 0, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 委托给 draftService
	index, err := h.draftService.AddStateOutput(ctx, draft, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)
	if err != nil {
		return 0, fmt.Errorf("追加状态输出失败: %w", err)
	}

	// 保存草稿
	if err := h.draftService.SaveDraft(ctx, draft); err != nil {
		return 0, fmt.Errorf("保存草稿失败: %w", err)
	}

	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "TxAddStateOutput",
		Parameters: map[string]interface{}{
			"stateID":      stateID,
			"stateVersion": stateVersion,
		},
		Result: map[string]interface{}{
			"index": index,
		},
	})

	if h.logger != nil {
		h.logger.Debugf("HostABI.TxAddStateOutput draftID=%s index=%d stateVersion=%d", draftID, index, stateVersion)
	}

	return index, nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 类别 D：资源查询（只读）- 2个原语
// ════════════════════════════════════════════════════════════════════════════════════════════════

// ResourceLookup 查询资源元数据
func (h *HostRuntimePorts) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节")
	}

	// 使用 ResourceQuery 查询资源元数据
	resource, err := h.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询资源失败: %w", err)
	}

	if h.logger != nil {
		h.logger.Debugf("HostABI.ResourceLookup contentHash=%x", contentHash[:8])
	}

	// P0: 更新资源使用统计（资源查询）
	if h.execCtx != nil {
		if usage := h.execCtx.GetResourceUsage(); usage != nil {
			usage.ResourceQueries++
		}
	}

	return resource, nil
}

// ResourceExists 检查资源是否存在
func (h *HostRuntimePorts) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	if len(contentHash) != 32 {
		return false, fmt.Errorf("contentHash 必须是 32 字节")
	}

	// 委托给 repoManager - 尝试查询
	resource, err := h.uresCAS.ReadFile(ctx, contentHash)
	if err != nil {
		// 如果是"不存在"错误，返回 false
		return false, nil
	}

	exists := resource != nil

	if h.logger != nil {
		h.logger.Debugf("HostABI.ResourceExists contentHash=%x exists=%v", contentHash[:8], exists)
	}

	return exists, nil
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 类别 G：执行追踪（辅助）- 2个原语
// ════════════════════════════════════════════════════════════════════════════════════════════════

// EmitEvent 发射链上事件
func (h *HostRuntimePorts) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "EmitEvent",
		Parameters: map[string]interface{}{
			"eventType": eventType,
			"dataSize":  len(eventData),
		},
		Result: map[string]interface{}{
			"emitted": true,
		},
	})

	// 将事件添加到执行上下文
	// 事件会被包含在执行结果中，供上层（如ZK证明）使用
	event := &ispcInterfaces.Event{
		Type:      eventType,
		Timestamp: 0, // 由AddEvent自动填充
		Data: map[string]interface{}{
			"data": eventData,
		},
	}
	if err := h.execCtx.AddEvent(event); err != nil {
		if h.logger != nil {
			h.logger.Warnf("HostABI.EmitEvent 添加事件失败: %v", err)
		}
		return fmt.Errorf("添加事件失败: %w", err)
	}

	if h.logger != nil {
		h.logger.Debugf("HostABI.EmitEvent eventType=%s dataSize=%d", eventType, len(eventData))
	}

	return nil
}

// LogDebug 记录调试日志（非链上）
func (h *HostRuntimePorts) LogDebug(ctx context.Context, message string) error {
	if h.logger != nil {
		h.logger.Debugf("HostABI.LogDebug [Contract] %s", message)
	}

	// 记录到 ExecutionTrace
	h.execCtx.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		FunctionName: "LogDebug",
		Parameters: map[string]interface{}{
			"message": message,
		},
		Result: map[string]interface{}{
			"logged": true,
		},
	})

	return nil
}
