package adapter

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/interfaces/ures"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TxReceipt 交易收据（从hostabi复制，避免循环导入）
type TxReceipt struct {
	Mode           string `json:"mode"`
	UnsignedTxHash string `json:"unsigned_tx_hash,omitempty"`
	SignedTxHash   string `json:"signed_tx_hash,omitempty"`
	SerializedTx   string `json:"serialized_tx,omitempty"`
	ProposalID     string `json:"proposal_id,omitempty"`
	Error          string `json:"error,omitempty"`
}

// TxAdapter 接口定义（从hostabi复制，避免循环导入）
type TxAdapter interface {
	BeginTransaction(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error)
	AddTransfer(ctx context.Context, draftHandle int32, from []byte, to []byte, amount string, tokenID []byte) (int32, error)
	AddCustomInput(ctx context.Context, draftHandle int32, outpoint *pb.OutPoint, isReferenceOnly bool) (int32, error)
	AddCustomOutput(ctx context.Context, draftHandle int32, output *pb.TxOutput) (int32, error)
	GetDraft(ctx context.Context, draftHandle int32) (interface{}, error)
	FinalizeTransaction(ctx context.Context, draftHandle int32) (*pb.Transaction, error)
	CleanupDraft(ctx context.Context, draftHandle int32) error
}

// 错误码常量（从hostabi包复制，避免循环导入）
const (
	ErrContextNotFound    = 5003
	ErrMemoryAccessFailed = 5004
	ErrInternalError      = 5001
	ErrServiceUnavailable = 5005
	ErrInvalidParameter   = 1001
	ErrBufferTooSmall     = 1005
	ErrInvalidAddress     = 1010
	ErrInvalidHash        = 1011
	ErrResourceNotFound   = 2003
	ErrEncodingFailed     = 5002
	ErrNotImplemented     = 5006 // 功能未实现
)

// WASMAdapter WASM宿主函数适配器
//
// 🎯 **设计目的**：从HostABI构建WASM引擎兼容的宿主函数映射
// 📋 **职责**：将HostABI的原语方法适配为WASM引擎兼容的闭包函数
//
// 🏗️ **架构位置**：
// - 作为hostabi/adapter的一部分
// - 接收HostFunctionProvider的依赖，构建WASM宿主函数映射
//
// 🔧 **依赖关系**：
// - HostABI：提供原始宿主能力
// - Provider依赖：logger, blockQuery, eutxoQuery等（从Provider传递）
type WASMAdapter struct {
	logger         log.Logger
	chainQuery     persistence.ChainQuery
	blockQuery     persistence.BlockQuery
	eutxoQuery     persistence.UTXOQuery
	uresCAS        ures.CASStorage
	txQuery        persistence.TxQuery
	resourceQuery  persistence.ResourceQuery
	txHashClient   transaction.TransactionHashServiceClient
	addressManager crypto.AddressManager
	hashManager    crypto.HashManager
	txAdapter      interface{} // TxAdapter类型（避免循环依赖）
	draftService   tx.TransactionDraftService
	getExecCtxFunc func(context.Context) ispcInterfaces.ExecutionContext // 从context提取ExecutionContext的函数（避免循环依赖）

	// 函数依赖（避免循环导入）
	// 注意：buildTxFromDraft的第一个参数是interface{}，因为适配函数内部会使用hostabi.TxAdapter而不是adapter.TxAdapter
	buildTxFromDraft func(context.Context, interface{}, transaction.TransactionHashServiceClient, persistence.UTXOQuery, []byte, []byte, []byte, uint64, uint64) (*TxReceipt, error)
	encodeTxReceipt  func(*TxReceipt) ([]byte, error)

	// 内存分配器管理（每个模块一个allocator）
	allocators map[string]*memoryAllocator
	allocMutex sync.RWMutex
}

// NewWASMAdapter 创建WASM适配器
//
// 📋 **参数**：
//   - logger: 日志服务
//   - chainQuery: 链查询服务
//   - blockQuery: 区块查询服务
//   - eutxoQuery: UTXO查询服务
//   - uresCAS: CAS存储服务
//   - txQuery: 交易查询服务
//   - resourceQuery: 资源查询服务
//   - txHashClient: 交易哈希服务客户端
//   - addressManager: 地址管理器
//   - hashManager: 哈希管理器
//   - txAdapter: TX适配器
//   - draftService: 交易草稿服务
//   - getExecCtxFunc: 从context提取ExecutionContext的函数（避免循环依赖）
//
// 🔧 **返回值**：
//   - *WASMAdapter: WASM适配器实例
func NewWASMAdapter(
	logger log.Logger,
	chainQuery persistence.ChainQuery,
	blockQuery persistence.BlockQuery,
	eutxoQuery persistence.UTXOQuery,
	uresCAS ures.CASStorage,
	txQuery persistence.TxQuery,
	resourceQuery persistence.ResourceQuery,
	txHashClient transaction.TransactionHashServiceClient,
	addressManager crypto.AddressManager,
	hashManager crypto.HashManager,
	txAdapter interface{},
	draftService tx.TransactionDraftService,
	getExecCtxFunc func(context.Context) ispcInterfaces.ExecutionContext,
	buildTxFromDraft func(context.Context, interface{}, transaction.TransactionHashServiceClient, persistence.UTXOQuery, []byte, []byte, []byte, uint64, uint64) (*TxReceipt, error),
	encodeTxReceipt func(*TxReceipt) ([]byte, error),
) *WASMAdapter {
	return &WASMAdapter{
		logger:           logger,
		chainQuery:       chainQuery,
		blockQuery:       blockQuery,
		eutxoQuery:       eutxoQuery,
		uresCAS:          uresCAS,
		txQuery:          txQuery,
		resourceQuery:    resourceQuery,
		txHashClient:     txHashClient,
		addressManager:   addressManager,
		hashManager:      hashManager,
		txAdapter:        txAdapter,
		draftService:     draftService,
		getExecCtxFunc:   getExecCtxFunc,
		buildTxFromDraft: buildTxFromDraft,
		encodeTxReceipt:  encodeTxReceipt,
		allocators:       make(map[string]*memoryAllocator),
	}
}

// BuildHostFunctions 构建WASM宿主函数映射
//
// 📋 **参数**：
//   - ctx: 调用上下文（包含ExecutionContext）
//   - hostABI: HostABI实例
//
// 🔧 **返回值**：
//   - map[string]interface{}: WASM宿主函数映射（24个函数）
//
// 🎯 **设计说明**：
// 该方法将HostABI的原语方法适配为WASM引擎兼容的闭包函数。
// 所有宿主函数都从ctx动态提取ExecutionContext，确保状态隔离。
func (a *WASMAdapter) BuildHostFunctions(
	ctx context.Context,
	hostABI publicispc.HostABI,
) map[string]interface{} {
	// 🎯 **完整的 WASM 宿主函数集（28个函数）**
	//
	// ⚠️ **重要设计**：
	// 所有宿主函数都从 ctx 动态提取 ExecutionContext，而不是闭包捕获
	// 原因：env 模块只能实例化一次，闭包捕获会导致第二次调用使用旧的 ExecutionContext
	//
	// 注意：以下函数签名需要与 WASM 合约的 import 声明匹配
	// 包括原有的业务函数 + 新增的合约运行时函数

	return map[string]interface{}{
		// ═══════════════════════════════════════════════
		// 类别 A：ABI 版本查询
		// ═══════════════════════════════════════════════

		// get_abi_version - 获取 Host ABI 版本号
		// 签名: () -> (version: u32)
		// 返回: ABI版本号（格式: (major<<16)|(minor<<8)|patch），例如 v1.0.0 -> 0x00010000
		"get_abi_version": func() uint32 {
			// WES Host ABI v1.0.0
			// 版本编码: (major << 16) | (minor << 8) | patch
			const (
				ABIVersionMajor = 1
				ABIVersionMinor = 0
				ABIVersionPatch = 0
			)
			version := uint32((ABIVersionMajor << 16) | (ABIVersionMinor << 8) | ABIVersionPatch)
			if a.logger != nil {
				a.logger.Debugf("get_abi_version: v%d.%d.%d (0x%08X)", ABIVersionMajor, ABIVersionMinor, ABIVersionPatch, version)
			}
			return version
		},

		// ═══════════════════════════════════════════════
		// 类别 B：链上上下文查询（只读，确定性）
		// ═══════════════════════════════════════════════

		"get_block_height": func() uint64 {
			height, err := hostABI.GetBlockHeight(ctx)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("get_block_height: 获取区块高度失败: %v", err)
				}
				// 🔧 **修复**：使用 math.MaxUint64 表示错误，避免与区块0混淆
				return math.MaxUint64
			}
			return height
		},

		"get_block_timestamp": func() uint64 {
			timestamp, err := hostABI.GetBlockTimestamp(ctx)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("get_block_timestamp: 获取区块时间戳失败: %v", err)
				}
				// 🔧 **修复**：使用 math.MaxUint64 表示错误，避免与Unix纪元混淆
				return math.MaxUint64
			}
			return timestamp
		},

		// get_caller - 获取调用者地址（写入WASM内存）
		// 签名: (addr_ptr: u32) -> (len: u32)
		// 写入20字节地址到addr_ptr，返回字节数或错误码
		// 🔧 **修复**：使用错误码区分不同错误类型
		"get_caller": func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Error("get_caller: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("get_caller: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// ⚠️ **边界检查**：验证内存大小是否有效
			memSize := memory.Size()
			if memSize == 0 || addrPtr+20 > memSize {
				if a.logger != nil {
					a.logger.Errorf("get_caller: 内存大小无效或地址越界 memSize=%d addrPtr=%d", memSize, addrPtr)
				}
				return ErrInvalidParameter
			}

			// 从ExecutionContext获取调用者地址（20字节）
			callerBytes := currentExecCtx.GetCallerAddress()
			if len(callerBytes) != 20 {
				if a.logger != nil {
					a.logger.Errorf("get_caller: 调用者地址长度错误: %d", len(callerBytes))
				}
				return ErrInvalidAddress
			}

			// 写入WASM内存
			if !memory.Write(addrPtr, callerBytes) {
				if a.logger != nil {
					a.logger.Errorf("get_caller: 写入内存失败 addrPtr=%d", addrPtr)
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Infof("🔧 get_caller: %x (20字节)", callerBytes)
			}

			return 20 // 成功时返回20字节
		},

		// get_block_hash - 获取指定高度的区块哈希（写入WASM内存）
		// 签名: (height: u64, hash_ptr: u32) -> (len: u32)
		// 写入32字节区块哈希到hash_ptr，返回字节数（32）
		"get_block_hash": func(ctx context.Context, m api.Module, height uint64, hashPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Error("get_block_hash: ExecutionContext 未找到")
				}
				return 0
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("get_block_hash: 无法获取WASM内存")
				}
				return 0
			}

			// 1. 查询区块
			if a.blockQuery == nil {
				if a.logger != nil {
					a.logger.Error("get_block_hash: BlockQuery 未设置")
				}
				return 0
			}

			block, err := a.blockQuery.GetBlockByHeight(ctx, height)
			if err != nil || block == nil {
				if a.logger != nil {
					a.logger.Errorf("get_block_hash: 获取区块失败 height=%d err=%v", height, err)
				}
				return 0
			}

			// 2. 计算区块哈希（使用DoubleSHA256，与挖矿保持一致）
			if a.hashManager == nil {
				if a.logger != nil {
					a.logger.Error("get_block_hash: HashManager 未设置")
				}
				return 0
			}

			// 序列化区块头
			headerBytes, err := proto.Marshal(block.Header)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("get_block_hash: 序列化区块头失败: %v", err)
				}
				return 0
			}

			// 计算DoubleSHA256哈希（32字节）
			blockHash := a.hashManager.DoubleSHA256(headerBytes)
			if len(blockHash) != 32 {
				if a.logger != nil {
					a.logger.Errorf("get_block_hash: 哈希长度错误: %d", len(blockHash))
				}
				return 0
			}

			// 3. 写入WASM内存
			if !memory.Write(hashPtr, blockHash) {
				if a.logger != nil {
					a.logger.Error("get_block_hash: 写入内存失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Infof("🔧 get_block_hash: height=%d hash=%x (32字节)", height, blockHash[:8])
			}

			return 32 // 区块哈希固定32字节
		},

		// get_merkle_root - 获取指定高度区块的Merkle根（写入WASM内存）
		// 签名: (height: u64, root_ptr: u32) -> (len: u32)
		// 写入32字节Merkle根到root_ptr，返回字节数（32）
		"get_merkle_root": func(ctx context.Context, m api.Module, height uint64, rootPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_merkle_root: ExecutionContext未找到")
				}
				return 0
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_merkle_root: 无法获取WASM内存")
				}
				return 0
			}

			// 查询区块
			if a.blockQuery == nil {
				if a.logger != nil {
					a.logger.Warn("get_merkle_root: BlockQuery未设置")
				}
				return 0
			}

			block, err := a.blockQuery.GetBlockByHeight(ctx, height)
			if err != nil || block == nil || block.Header == nil {
				if a.logger != nil {
					a.logger.Warnf("get_merkle_root: 获取区块失败 height=%d err=%v", height, err)
				}
				return 0
			}

			merkleRoot := block.Header.MerkleRoot
			if len(merkleRoot) != 32 {
				if a.logger != nil {
					a.logger.Warnf("get_merkle_root: Merkle根长度错误 len=%d", len(merkleRoot))
				}
				return 0
			}

			// 写入WASM内存
			if !memory.Write(rootPtr, merkleRoot) {
				if a.logger != nil {
					a.logger.Warn("get_merkle_root: 写入内存失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Debugf("get_merkle_root: height=%d root=%x", height, merkleRoot[:8])
			}

			return 32 // Merkle根固定32字节
		},

		// get_state_root - 获取指定高度区块的状态根（写入WASM内存）
		// 签名: (height: u64, root_ptr: u32) -> (len: u32)
		// 写入32字节状态根到root_ptr，返回字节数（32）
		"get_state_root": func(ctx context.Context, m api.Module, height uint64, rootPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_state_root: ExecutionContext未找到")
				}
				return 0
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_state_root: 无法获取WASM内存")
				}
				return 0
			}

			// 查询区块
			if a.blockQuery == nil {
				if a.logger != nil {
					a.logger.Warn("get_state_root: BlockQuery未设置")
				}
				return 0
			}

			block, err := a.blockQuery.GetBlockByHeight(ctx, height)
			if err != nil || block == nil || block.Header == nil {
				if a.logger != nil {
					a.logger.Warnf("get_state_root: 获取区块失败 height=%d err=%v", height, err)
				}
				return 0
			}

			stateRoot := block.Header.StateRoot
			if len(stateRoot) != 32 {
				if a.logger != nil {
					a.logger.Warnf("get_state_root: 状态根长度错误 len=%d", len(stateRoot))
				}
				return 0
			}

			// 写入WASM内存
			if !memory.Write(rootPtr, stateRoot) {
				if a.logger != nil {
					a.logger.Warn("get_state_root: 写入内存失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Debugf("get_state_root: height=%d root=%x", height, stateRoot[:8])
			}

			return 32 // 状态根固定32字节
		},

		// get_miner_address - 获取指定高度区块的矿工地址（写入WASM内存）
		// 签名: (height: u64, addr_ptr: u32) -> (len: u32)
		// 写入20字节矿工地址到addr_ptr，返回字节数（20）
		"get_miner_address": func(ctx context.Context, m api.Module, height uint64, addrPtr uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_miner_address: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}
			if a.blockQuery == nil {
				if a.logger != nil {
					a.logger.Warn("get_miner_address: BlockQuery未设置")
				}
				return ErrInternalError
			}

			blk, err := a.blockQuery.GetBlockByHeight(ctx, height)
			if err != nil || blk == nil || blk.Body == nil {
				if a.logger != nil {
					a.logger.Warnf("get_miner_address: 获取区块失败 height=%d err=%v", height, err)
				}
				return ErrInternalError
			}

			// 规则：优先选择“0 inputs 的交易”（coinbase/铸造语义）里第一个输出的 owner 作为矿工地址。
			// 这是交易层的正确边界：coinbase 是 0 inputs + AssetOutput 的组合语义。
			var minerAddr []byte
			for _, tx := range blk.Body.Transactions {
				if tx == nil {
					continue
				}
				if len(tx.GetInputs()) != 0 {
					continue
				}
				for _, out := range tx.GetOutputs() {
					if out == nil {
						continue
					}
					owner := out.GetOwner()
					if len(owner) == 20 {
						minerAddr = owner
						break
					}
				}
				if len(minerAddr) == 20 {
					break
				}
			}

			if len(minerAddr) != 20 {
				if a.logger != nil {
					a.logger.Warnf("get_miner_address: 未找到有效矿工地址 height=%d", height)
				}
				return ErrInternalError
			}

			if !memory.Write(addrPtr, minerAddr) {
				if a.logger != nil {
					a.logger.Warn("get_miner_address: 写入内存失败")
				}
				return ErrMemoryAccessFailed
			}

			return 20
		},

		// get_chain_id - 获取链ID（写入WASM内存）
		// 签名: (chain_id_ptr: u32) -> (len: u32)
		// 写入链ID字符串到chain_id_ptr，返回字节数
		"get_chain_id": func(ctx context.Context, m api.Module, chainIDPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_chain_id: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_chain_id: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 从ExecutionContext获取链ID
			chainID := currentExecCtx.GetChainID()
			if len(chainID) == 0 {
				if a.logger != nil {
					a.logger.Warn("get_chain_id: 链ID为空")
				}
				return ErrInternalError
			}

			// 写入WASM内存
			if !memory.Write(chainIDPtr, chainID) {
				if a.logger != nil {
					a.logger.Warnf("get_chain_id: 写入内存失败 ptr=%d", chainIDPtr)
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("get_chain_id: chainID=%s len=%d", string(chainID), len(chainID))
			}

			return uint32(len(chainID))
		},

		// get_contract_address - 获取合约地址（写入WASM内存）
		// 签名: (addr_ptr: u32) -> (len: u32)
		// 写入20字节合约地址到addr_ptr，返回字节数（20）
		"get_contract_address": func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_contract_address: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_contract_address: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 从ExecutionContext获取合约地址（20字节）
			contractAddr := currentExecCtx.GetContractAddress()
			if len(contractAddr) != 20 {
				if a.logger != nil {
					a.logger.Warnf("get_contract_address: 合约地址长度错误 len=%d", len(contractAddr))
				}
				return ErrInvalidAddress
			}

			// 写入WASM内存
			if !memory.Write(addrPtr, contractAddr) {
				if a.logger != nil {
					a.logger.Warnf("get_contract_address: 写入内存失败 ptr=%d", addrPtr)
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("get_contract_address: addr=%x", contractAddr)
			}

			return 20 // 合约地址固定20字节
		},

		// get_transaction_id - 获取交易ID（写入WASM内存）
		// 签名: (tx_id_ptr: u32) -> (len: u32)
		// 写入32字节交易哈希到tx_id_ptr，返回字节数（32）
		"get_transaction_id": func(ctx context.Context, m api.Module, txIDPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 获取DraftID
			draftID := currentExecCtx.GetDraftID()
			if draftID == "" {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: DraftID为空")
				}
				return ErrInternalError
			}

			// 2. 加载Draft
			if a.draftService == nil {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: DraftService未设置")
				}
				return ErrServiceUnavailable
			}

			draft, err := a.draftService.GetDraftByID(ctx, draftID)
			if err != nil || draft == nil {
				if a.logger != nil {
					a.logger.Warnf("get_transaction_id: 加载Draft失败 draftID=%s err=%v", draftID, err)
				}
				return ErrInternalError
			}

			// 3. 获取交易对象
			if draft.Tx == nil {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: Draft.Tx为空")
				}
				return ErrInternalError
			}

			// 4. 计算交易哈希
			if a.txHashClient == nil {
				if a.logger != nil {
					a.logger.Warn("get_transaction_id: TransactionHashServiceClient未设置")
				}
				return ErrServiceUnavailable
			}

			req := &transaction.ComputeHashRequest{
				Transaction:      draft.Tx,
				IncludeDebugInfo: false,
			}

			resp, err := a.txHashClient.ComputeHash(ctx, req)
			if err != nil || resp == nil || len(resp.Hash) != 32 {
				if a.logger != nil {
					a.logger.Warnf("get_transaction_id: 计算交易哈希失败 draftID=%s err=%v", draftID, err)
				}
				return ErrInternalError
			}

			txHash := resp.Hash

			// 5. 写入WASM内存
			if !memory.Write(txIDPtr, txHash) {
				if a.logger != nil {
					a.logger.Warnf("get_transaction_id: 写入内存失败 ptr=%d", txIDPtr)
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("get_transaction_id: txHash=%x", txHash[:8])
			}

			return 32 // 交易哈希固定32字节
		},

		// get_tx_hash - 获取交易哈希（SDK兼容别名，与get_transaction_id相同）
		// 签名: (hash_ptr: u32) -> (len: u32)
		// 写入32字节交易哈希到hash_ptr，返回字节数（32）
		"get_tx_hash": func(ctx context.Context, m api.Module, hashPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("get_tx_hash: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("get_tx_hash: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 从ExecutionContext获取交易ID（32字节哈希）
			txHash := currentExecCtx.GetTransactionID()
			if len(txHash) != 32 {
				if a.logger != nil {
					a.logger.Warnf("get_tx_hash: 交易哈希长度错误 len=%d", len(txHash))
				}
				return ErrInternalError
			}

			// 写入WASM内存
			if !memory.Write(hashPtr, txHash) {
				if a.logger != nil {
					a.logger.Warnf("get_tx_hash: 写入内存失败 ptr=%d", hashPtr)
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("get_tx_hash: txHash=%x", txHash[:8])
			}

			return 32 // 交易哈希固定32字节
		},

		// get_tx_index - 获取交易在区块中的索引
		// 签名: () -> (index: u32)
		// 返回: 交易在区块中的索引（从0开始），如果未确定则返回0xFFFFFFFF
		"get_tx_index": func() uint32 {
			// ⚠️ **注意**：当前实现中，交易索引在执行时可能尚未确定
			// 因为交易还在草稿阶段，尚未打包到区块中
			// 返回0xFFFFFFFF表示索引未确定
			// 如果需要索引，应该在交易打包后通过其他方式获取
			return 0xFFFFFFFF // 表示索引未确定
		},

		// ═══════════════════════════════════════════════
		// 类别 B：UTXO 查询（只读）
		// ═══════════════════════════════════════════════

		// query_utxo_balance - 查询地址余额（framework需要）
		// 签名: (address_ptr: u32, token_id_ptr: u32, token_id_len: u32) -> (balance: u64)
		"query_utxo_balance": func(ctx context.Context, m api.Module, addressPtr uint32, tokenIDPtr uint32, tokenIDLen uint32) uint64 {
			if a.logger != nil {
				a.logger.Infof("🔧 query_utxo_balance 被调用: addressPtr=%d, tokenIDPtr=%d, tokenIDLen=%d", addressPtr, tokenIDPtr, tokenIDLen)
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("query_utxo_balance: 无法获取内存")
				}
				return 0
			}

			// 从WASM内存读取地址（20字节）
			addressBytes, ok := memory.Read(addressPtr, 20)
			if !ok {
				if a.logger != nil {
					a.logger.Error("query_utxo_balance: 读取地址失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Infof("🔧 query_utxo_balance: 读取到地址 address=%x", addressBytes)
			}

			// 读取可选的tokenID
			var tokenID []byte
			if tokenIDPtr != 0 && tokenIDLen > 0 {
				tokenID, ok = memory.Read(tokenIDPtr, tokenIDLen)
				if !ok {
					if a.logger != nil {
						a.logger.Errorf("query_utxo_balance: 读取tokenID失败")
					}
					return 0
				}
				if a.logger != nil {
					a.logger.Infof("🔧 query_utxo_balance: tokenID=%s (len=%d)", string(tokenID), tokenIDLen)
				}
			} else {
				if a.logger != nil {
					a.logger.Info("🔧 query_utxo_balance: tokenID为空，查询所有代币")
				}
			}

			// 查询余额（通过utxoManager）
			// 获取该地址的所有UTXO（只查未花费的）
			utxos, err := a.eutxoQuery.GetUTXOsByAddress(ctx, addressBytes, nil, true)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("query_utxo_balance: 查询UTXO失败: %v", err)
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Infof("🔧 query_utxo_balance: 找到 %d 个UTXO", len(utxos))
			}

			currentExecCtx := a.getExecCtxFunc(ctx)
			var contractAddress []byte
			if currentExecCtx != nil {
				contractAddress = currentExecCtx.GetContractAddress()
			} else if len(tokenID) > 0 && a.logger != nil {
				a.logger.Warn("query_utxo_balance: ExecutionContext未找到，无法匹配合约代币")
			}

			// 累加余额
			var balance uint64
			requestTokenID := string(tokenID)
			for idx, utxo := range utxos {
				output := utxo.GetCachedOutput()
				if output == nil {
					if a.logger != nil {
						a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 没有缓存输出", idx)
					}
					continue
				}

				// 检查是否是Asset输出
				if asset := output.GetAsset(); asset != nil {
					if len(tokenID) == 0 {
						if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
							if amount, err := strconv.ParseUint(nativeCoin.Amount, 10, 64); err == nil {
								balance += amount
								if a.logger != nil {
									a.logger.Infof("🔧 query_utxo_balance: UTXO[%d] amount=%d, 累计=%d", idx, amount, balance)
								}
							} else if a.logger != nil {
								a.logger.Errorf("🔧 query_utxo_balance: UTXO[%d] 解析金额失败: %v", idx, err)
							}
						} else if a.logger != nil {
							a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 不是原生币", idx)
						}
						continue
					}

					// 合约代币路径
					contractToken := asset.GetContractToken()
					if contractToken == nil {
						if a.logger != nil {
							a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 不是合约代币输出", idx)
						}
						continue
					}

					if len(contractAddress) != 20 {
						if a.logger != nil {
							a.logger.Warnf("🔧 query_utxo_balance: 无法比较合约地址（len=%d）", len(contractAddress))
						}
						continue
					}

					if !bytes.Equal(contractToken.GetContractAddress(), contractAddress) {
						if a.logger != nil {
							a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 合约地址不匹配", idx)
						}
						continue
					}

					if requestTokenID == "" {
						if a.logger != nil {
							a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 请求TokenID为空，跳过合约代币", idx)
						}
						continue
					}

					if string(contractToken.GetFungibleClassId()) != requestTokenID {
						if a.logger != nil {
							a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] TokenID不匹配", idx)
						}
						continue
					}

					if amount, err := strconv.ParseUint(contractToken.GetAmount(), 10, 64); err == nil {
						balance += amount
						if a.logger != nil {
							a.logger.Infof("🔧 query_utxo_balance: UTXO[%d] 合约代币 amount=%d, 累计=%d", idx, amount, balance)
						}
					} else if a.logger != nil {
						a.logger.Errorf("🔧 query_utxo_balance: UTXO[%d] 合约代币金额解析失败: %v", idx, err)
					}
				} else {
					if a.logger != nil {
						a.logger.Debugf("🔧 query_utxo_balance: UTXO[%d] 不是Asset输出", idx)
					}
				}
			}

			if a.logger != nil {
				if len(tokenID) == 0 {
					a.logger.Infof("🔧 query_utxo_balance 完成: address=%x, 原生余额=%d", addressBytes, balance)
				} else {
					a.logger.Infof("🔧 query_utxo_balance 完成: address=%x, tokenID=%s, 代币余额=%d", addressBytes, requestTokenID, balance)
				}
			}

			return balance
		},
		"utxo_lookup": func(ctx context.Context, m api.Module, txIDPtr uint32, txIDLen uint32, index uint32, outputPtr uint32, outputSize uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("utxo_lookup: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("utxo_lookup: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 txID
			if txIDLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: txID长度无效 len=%d", txIDLen)
				}
				return ErrInvalidParameter
			}

			txIDBytes, ok := memory.Read(txIDPtr, txIDLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: 读取txID失败 ptr=%d len=%d", txIDPtr, txIDLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 构造 OutPoint
			outpoint := &pb.OutPoint{
				TxId:        txIDBytes,
				OutputIndex: index,
			}

			// 3. 调用 hostABI.UTXOLookup
			txOutput, err := hostABI.UTXOLookup(ctx, outpoint)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: 查询失败 txID=%x index=%d err=%v", txIDBytes[:8], index, err)
				}
				return ErrResourceNotFound
			}

			if txOutput == nil {
				if a.logger != nil {
					a.logger.Debugf("utxo_lookup: UTXO不存在 txID=%x index=%d", txIDBytes[:8], index)
				}
				return ErrResourceNotFound
			}

			// 4. 将 TxOutput 序列化并写入 WASM 内存
			outputBytes, err := proto.Marshal(txOutput)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: 序列化失败 err=%v", err)
				}
				return ErrEncodingFailed
			}

			if uint32(len(outputBytes)) > outputSize {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: 输出缓冲区太小 required=%d provided=%d", len(outputBytes), outputSize)
				}
				return ErrBufferTooSmall
			}

			if !memory.Write(outputPtr, outputBytes) {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup: 写入内存失败 ptr=%d len=%d", outputPtr, len(outputBytes))
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("utxo_lookup: 成功 txID=%x index=%d outputLen=%d", txIDBytes[:8], index, len(outputBytes))
			}

			return uint32(len(outputBytes))
		},

		// utxo_lookup_json - UTXO查询（JSON格式，TinyGo友好）
		// 签名: (tx_id_ptr: u32, tx_id_len: u32, index: u32, output_ptr: u32, output_size: u32) -> (actual_len: u32)
		// 返回: 实际写入的JSON字节数，0表示失败
		"utxo_lookup_json": func(ctx context.Context, m api.Module, txIDPtr uint32, txIDLen uint32, index uint32, outputPtr uint32, outputSize uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("utxo_lookup_json: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("utxo_lookup_json: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 txID
			if txIDLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: txID长度无效 len=%d", txIDLen)
				}
				return ErrInvalidParameter
			}

			txIDBytes, ok := memory.Read(txIDPtr, txIDLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: 读取txID失败 ptr=%d len=%d", txIDPtr, txIDLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 构造 OutPoint
			outpoint := &pb.OutPoint{
				TxId:        txIDBytes,
				OutputIndex: index,
			}

			// 3. 调用 hostABI.UTXOLookup
			txOutput, err := hostABI.UTXOLookup(ctx, outpoint)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: 查询失败 txID=%x index=%d err=%v", txIDBytes[:8], index, err)
				}
				return ErrResourceNotFound
			}

			if txOutput == nil {
				if a.logger != nil {
					a.logger.Debugf("utxo_lookup_json: UTXO不存在 txID=%x index=%d", txIDBytes[:8], index)
				}
				return ErrResourceNotFound
			}

			// 4. 将 TxOutput 序列化为JSON（而非Protobuf）
			outputJSON, err := json.Marshal(txOutput)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: JSON序列化失败 err=%v", err)
				}
				return ErrEncodingFailed
			}

			if uint32(len(outputJSON)) > outputSize {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: 输出缓冲区太小 required=%d provided=%d", len(outputJSON), outputSize)
				}
				return ErrBufferTooSmall
			}

			if !memory.Write(outputPtr, outputJSON) {
				if a.logger != nil {
					a.logger.Warnf("utxo_lookup_json: 写入内存失败 ptr=%d len=%d", outputPtr, len(outputJSON))
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("utxo_lookup_json: 成功 txID=%x index=%d jsonLen=%d", txIDBytes[:8], index, len(outputJSON))
			}

			return uint32(len(outputJSON))
		},

		"utxo_exists": func(ctx context.Context, m api.Module, txIDPtr uint32, txIDLen uint32, index uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("utxo_exists: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// ⚠️ **边界检查**：验证内存大小是否有效
			memSize := memory.Size()
			if memSize == 0 {
				if a.logger != nil {
					a.logger.Warn("utxo_exists: 内存大小无效")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 txID
			if txIDLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("utxo_exists: txID长度无效 len=%d", txIDLen)
				}
				return ErrInvalidParameter
			}

			// ⚠️ **边界检查**：验证地址范围
			if txIDPtr+txIDLen > memSize {
				if a.logger != nil {
					a.logger.Warnf("utxo_exists: 地址越界 ptr=%d len=%d memSize=%d", txIDPtr, txIDLen, memSize)
				}
				return ErrMemoryAccessFailed
			}

			txIDBytes, ok := memory.Read(txIDPtr, txIDLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("utxo_exists: 读取txID失败 ptr=%d len=%d", txIDPtr, txIDLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 构造 OutPoint
			outpoint := &pb.OutPoint{
				TxId:        txIDBytes,
				OutputIndex: index,
			}

			// 3. 调用 hostABI.UTXOExists
			exists, err := hostABI.UTXOExists(ctx, outpoint)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("utxo_exists: 查询失败 txID=%x index=%d err=%v", txIDBytes[:8], index, err)
				}
				return 0 // 查询失败视为不存在
			}

			// 4. 返回 1（存在）或 0（不存在）
			if exists {
				if a.logger != nil {
					a.logger.Debugf("utxo_exists: UTXO存在 txID=%x index=%d", txIDBytes[:8], index)
				}
				return 1
			}

			if a.logger != nil {
				a.logger.Debugf("utxo_exists: UTXO不存在 txID=%x index=%d", txIDBytes[:8], index)
			}
			return 0
		},

		// ═══════════════════════════════════════════════
		// 类别 C：交易草稿操作（副作用）
		// ═══════════════════════════════════════════════

		"append_tx_input": func(ctx context.Context, m api.Module, txIDPtr uint32, txIDLen uint32, index uint32, isRefOnly uint32, proofPtr uint32, proofLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("append_tx_input: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("append_tx_input: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 txID
			if txIDLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("append_tx_input: txID长度无效 len=%d", txIDLen)
				}
				return ErrInvalidParameter
			}

			txIDBytes, ok := memory.Read(txIDPtr, txIDLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_tx_input: 读取txID失败 ptr=%d len=%d", txIDPtr, txIDLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 构造 OutPoint
			outpoint := &pb.OutPoint{
				TxId:        txIDBytes,
				OutputIndex: index,
			}

			// 3. 从 WASM 内存读取 unlockingProof（可选）
			var unlockingProof *pb.UnlockingProof
			if proofPtr != 0 && proofLen > 0 {
				proofBytes, ok := memory.Read(proofPtr, proofLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_tx_input: 读取proof失败 ptr=%d len=%d", proofPtr, proofLen)
					}
					return ErrMemoryAccessFailed
				}

				unlockingProof = &pb.UnlockingProof{}
				if err := proto.Unmarshal(proofBytes, unlockingProof); err != nil {
					if a.logger != nil {
						a.logger.Warnf("append_tx_input: 解析proof失败 err=%v", err)
					}
					return ErrEncodingFailed
				}
			}

			// 4. 调用 hostABI.TxAddInput
			isReferenceOnly := isRefOnly != 0
			inputIndex, err := hostABI.TxAddInput(ctx, outpoint, isReferenceOnly, unlockingProof)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("append_tx_input: 添加输入失败 err=%v", err)
				}
				return ErrInternalError
			}

			if a.logger != nil {
				a.logger.Debugf("append_tx_input: 成功 txID=%x index=%d inputIndex=%d", txIDBytes[:8], index, inputIndex)
			}

			return inputIndex
		},

		"append_asset_output": func(ctx context.Context, m api.Module, ownerPtr uint32, ownerLen uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32, lockPtr uint32, lockLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("append_asset_output: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("append_asset_output: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 owner（20字节）
			if ownerLen != 20 {
				if a.logger != nil {
					a.logger.Warnf("append_asset_output: owner长度无效 len=%d", ownerLen)
				}
				return ErrInvalidAddress
			}

			ownerBytes, ok := memory.Read(ownerPtr, ownerLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_asset_output: 读取owner失败 ptr=%d len=%d", ownerPtr, ownerLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 读取可选的 tokenID
			var tokenID []byte
			if tokenIDPtr != 0 && tokenIDLen > 0 {
				tokenID, ok = memory.Read(tokenIDPtr, tokenIDLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_asset_output: 读取tokenID失败 ptr=%d len=%d", tokenIDPtr, tokenIDLen)
					}
					return ErrMemoryAccessFailed
				}
			}

			// 3. 读取可选的锁定条件
			var lockingConditions []*pb.LockingCondition
			if lockPtr != 0 && lockLen > 0 {
				lockBytes, ok := memory.Read(lockPtr, lockLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_asset_output: 读取lock失败 ptr=%d len=%d", lockPtr, lockLen)
					}
					return ErrMemoryAccessFailed
				}

				// 解析锁定条件（protobuf编码）
				lock := &pb.LockingCondition{}
				if err := proto.Unmarshal(lockBytes, lock); err != nil {
					if a.logger != nil {
						a.logger.Warnf("append_asset_output: 解析lock失败 err=%v", err)
					}
					return ErrEncodingFailed
				}
				lockingConditions = []*pb.LockingCondition{lock}
			}

			// 4. 调用 hostABI.TxAddAssetOutput
			outputIndex, err := hostABI.TxAddAssetOutput(ctx, ownerBytes, amount, tokenID, lockingConditions)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("append_asset_output: 添加输出失败 err=%v", err)
				}
				return ErrInternalError
			}

			if a.logger != nil {
				a.logger.Debugf("append_asset_output: 成功 owner=%x amount=%d outputIndex=%d", ownerBytes[:8], amount, outputIndex)
			}

			return outputIndex
		},

		"append_resource_output": func(ctx context.Context, m api.Module, resourcePtr uint32, resourceLen uint32, ownerPtr uint32, ownerLen uint32, lockPtr uint32, lockLen uint32, timestamp uint64) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("append_resource_output: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("append_resource_output: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 resource（JSON格式）
			if resourceLen == 0 {
				if a.logger != nil {
					a.logger.Warn("append_resource_output: resource长度无效")
				}
				return ErrInvalidParameter
			}

			resourceBytes, ok := memory.Read(resourcePtr, resourceLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: 读取resource失败 ptr=%d len=%d", resourcePtr, resourceLen)
				}
				return ErrMemoryAccessFailed
			}

			// 解析资源数据（JSON格式）：{"content_hash": "hex_string", "category": "wasm", "metadata": "hex_string"}
			var resourceData struct {
				ContentHash string `json:"content_hash"`
				Category    string `json:"category"`
				Metadata    string `json:"metadata,omitempty"`
			}

			if err := json.Unmarshal(resourceBytes, &resourceData); err != nil {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: 解析resource JSON失败 err=%v", err)
				}
				return ErrEncodingFailed
			}

			// 转换 contentHash（hex -> bytes）
			contentHash, err := hex.DecodeString(resourceData.ContentHash)
			if err != nil || len(contentHash) != 32 {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: contentHash格式无效 err=%v len=%d", err, len(contentHash))
				}
				return ErrInvalidHash
			}

			// 转换 metadata（hex -> bytes，可选）
			var metadata []byte
			if resourceData.Metadata != "" {
				metadata, err = hex.DecodeString(resourceData.Metadata)
				if err != nil {
					if a.logger != nil {
						a.logger.Warnf("append_resource_output: metadata格式无效 err=%v", err)
					}
					return ErrEncodingFailed
				}
			}

			// 2. 读取 owner（20字节）
			if ownerLen != 20 {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: owner长度无效 len=%d", ownerLen)
				}
				return ErrInvalidAddress
			}

			ownerBytes, ok := memory.Read(ownerPtr, ownerLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: 读取owner失败 ptr=%d len=%d", ownerPtr, ownerLen)
				}
				return ErrMemoryAccessFailed
			}

			// 3. 读取可选的锁定条件
			var lockingConditions []*pb.LockingCondition
			if lockPtr != 0 && lockLen > 0 {
				lockBytes, ok := memory.Read(lockPtr, lockLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_resource_output: 读取lock失败 ptr=%d len=%d", lockPtr, lockLen)
					}
					return ErrMemoryAccessFailed
				}

				lock := &pb.LockingCondition{}
				if err := proto.Unmarshal(lockBytes, lock); err != nil {
					if a.logger != nil {
						a.logger.Warnf("append_resource_output: 解析lock失败 err=%v", err)
					}
					return ErrEncodingFailed
				}
				lockingConditions = []*pb.LockingCondition{lock}
			}

			// 4. 调用 hostABI.TxAddResourceOutput
			outputIndex, err := hostABI.TxAddResourceOutput(ctx, contentHash, resourceData.Category, ownerBytes, lockingConditions, metadata)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("append_resource_output: 添加输出失败 err=%v", err)
				}
				return ErrInternalError
			}

			if a.logger != nil {
				a.logger.Debugf("append_resource_output: 成功 contentHash=%x category=%s outputIndex=%d", contentHash[:8], resourceData.Category, outputIndex)
			}

			return outputIndex
		},

		// create_utxo_output - 创建UTXO输出（原生币）
		// 签名: (recipient_ptr: u32, recipient_len: u32, amount: u64, token_id_ptr: u32, token_id_len: u32) -> (status: u32)
		// 返回: 0=成功, 非0=失败
		"create_utxo_output": func(ctx context.Context, m api.Module, recipientPtr uint32, recipientLen uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("create_utxo_output: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("create_utxo_output: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 读取接收者地址
			if recipientLen != 20 {
				if a.logger != nil {
					a.logger.Warnf("create_utxo_output: recipient长度无效 len=%d", recipientLen)
				}
				return ErrInvalidAddress
			}

			recipient, ok := memory.Read(recipientPtr, recipientLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("create_utxo_output: 读取recipient失败 ptr=%d len=%d", recipientPtr, recipientLen)
				}
				return ErrMemoryAccessFailed
			}

			// 读取可选的 tokenID
			var tokenID []byte
			if tokenIDPtr != 0 && tokenIDLen > 0 {
				tokenID, ok = memory.Read(tokenIDPtr, tokenIDLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("create_utxo_output: 读取tokenID失败 ptr=%d len=%d", tokenIDPtr, tokenIDLen)
					}
					return ErrMemoryAccessFailed
				}
			}

			// 调用 hostABI.TxAddAssetOutput
			_, err := hostABI.TxAddAssetOutput(ctx, recipient, amount, tokenID, nil)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("create_utxo_output: 创建输出失败 err=%v", err)
				}
				return ErrInternalError
			}

			if a.logger != nil {
				a.logger.Debugf("create_utxo_output: 成功 recipient=%x amount=%d", recipient[:8], amount)
			}

			return 0 // 成功
		},

		// create_asset_output_with_lock - 创建带锁定条件的资产输出
		// 签名: (recipient_ptr: u32, recipient_len: u32, amount: u64, token_id_ptr: u32, token_id_len: u32, locking_ptr: u32, locking_len: u32) -> (output_index: u32)
		// 返回: 输出索引（>=0表示成功，0xFFFFFFFF表示失败）
		"create_asset_output_with_lock": func(ctx context.Context, m api.Module, recipientPtr uint32, recipientLen uint32, amount uint64, tokenIDPtr uint32, tokenIDLen uint32, lockingPtr uint32, lockingLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("create_asset_output_with_lock: ExecutionContext未找到")
				}
				return 0xFFFFFFFF
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("create_asset_output_with_lock: 无法获取WASM内存")
				}
				return 0xFFFFFFFF
			}

			// 读取接收者地址
			if recipientLen != 20 {
				if a.logger != nil {
					a.logger.Warnf("create_asset_output_with_lock: recipient长度无效 len=%d", recipientLen)
				}
				return 0xFFFFFFFF
			}

			recipient, ok := memory.Read(recipientPtr, recipientLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("create_asset_output_with_lock: 读取recipient失败 ptr=%d len=%d", recipientPtr, recipientLen)
				}
				return 0xFFFFFFFF
			}

			// 读取可选的 tokenID
			var tokenID []byte
			if tokenIDPtr != 0 && tokenIDLen > 0 {
				tokenID, ok = memory.Read(tokenIDPtr, tokenIDLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("create_asset_output_with_lock: 读取tokenID失败 ptr=%d len=%d", tokenIDPtr, tokenIDLen)
					}
					return 0xFFFFFFFF
				}
			}

			// 读取可选的锁定条件（JSON数组格式）
			var lockingConditions []*pb.LockingCondition
			if lockingPtr != 0 && lockingLen > 0 {
				lockingBytes, ok := memory.Read(lockingPtr, lockingLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("create_asset_output_with_lock: 读取locking失败 ptr=%d len=%d", lockingPtr, lockingLen)
					}
					return 0xFFFFFFFF
				}

				// 解析JSON数组格式的锁定条件
				var jsonConds []json.RawMessage
				if err := json.Unmarshal(lockingBytes, &jsonConds); err != nil {
					if a.logger != nil {
						a.logger.Warnf("create_asset_output_with_lock: 解析locking JSON失败 err=%v", err)
					}
					return 0xFFFFFFFF
				}

				for _, raw := range jsonConds {
					cond := &pb.LockingCondition{}
					if err := protojson.Unmarshal(raw, cond); err != nil {
						if a.logger != nil {
							a.logger.Warnf("create_asset_output_with_lock: 解析locking条件失败 err=%v", err)
						}
						return 0xFFFFFFFF
					}
					lockingConditions = append(lockingConditions, cond)
				}
			}

			// 调用 hostABI.TxAddAssetOutput
			outputIndex, err := hostABI.TxAddAssetOutput(ctx, recipient, amount, tokenID, lockingConditions)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("create_asset_output_with_lock: 创建输出失败 err=%v", err)
				}
				return 0xFFFFFFFF
			}

			if a.logger != nil {
				a.logger.Debugf("create_asset_output_with_lock: 成功 recipient=%x amount=%d outputIndex=%d", recipient[:8], amount, outputIndex)
			}

			return outputIndex
		},

		// batch_create_outputs - 批量创建资产输出
		// 签名: (batch_ptr: u32, batch_len: u32) -> (created_count: u32)
		// 返回: 成功创建的输出数量，0表示失败
		"batch_create_outputs": func(ctx context.Context, m api.Module, batchPtr uint32, batchLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("batch_create_outputs: ExecutionContext未找到")
				}
				return 0
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("batch_create_outputs: 无法获取WASM内存")
				}
				return 0
			}

			// 读取批量数据（JSON数组格式）
			batchBytes, ok := memory.Read(batchPtr, batchLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("batch_create_outputs: 读取batch失败 ptr=%d len=%d", batchPtr, batchLen)
				}
				return 0
			}

			// ⚠️ **注意**：HostABI 接口中没有 BatchCreateOutputs 方法
			// 当前实现：解析 JSON 数组，逐个调用 TxAddAssetOutput
			var batchItems []struct {
				Recipient         string          `json:"recipient"`
				Amount            uint64          `json:"amount"`
				TokenID           *string         `json:"token_id"`
				LockingConditions json.RawMessage `json:"locking_conditions"`
			}
			if err := json.Unmarshal(batchBytes, &batchItems); err != nil {
				if a.logger != nil {
					a.logger.Warnf("batch_create_outputs: 解析batch JSON失败 err=%v", err)
				}
				return 0
			}

			count := uint32(0)
			for _, item := range batchItems {
				// 解码 recipient（base64）
				recipient, err := hex.DecodeString(item.Recipient)
				if err != nil || len(recipient) != 20 {
					if a.logger != nil {
						a.logger.Warnf("batch_create_outputs: recipient格式无效")
					}
					continue
				}

				// 解码 tokenID（可选）
				var tokenID []byte
				if item.TokenID != nil {
					tokenID, err = hex.DecodeString(*item.TokenID)
					if err != nil {
						if a.logger != nil {
							a.logger.Warnf("batch_create_outputs: tokenID格式无效")
						}
						continue
					}
				}

				// 解析锁定条件
				var lockingConditions []*pb.LockingCondition
				if len(item.LockingConditions) > 0 {
					var jsonConds []json.RawMessage
					if err := json.Unmarshal(item.LockingConditions, &jsonConds); err == nil {
						for _, raw := range jsonConds {
							cond := &pb.LockingCondition{}
							if err := protojson.Unmarshal(raw, cond); err == nil {
								lockingConditions = append(lockingConditions, cond)
							}
						}
					}
				}

				// 调用 TxAddAssetOutput
				_, err = hostABI.TxAddAssetOutput(ctx, recipient, item.Amount, tokenID, lockingConditions)
				if err != nil {
					if a.logger != nil {
						a.logger.Warnf("batch_create_outputs: 创建输出失败 err=%v", err)
					}
					continue
				}
				count++
			}

			if a.logger != nil {
				a.logger.Debugf("batch_create_outputs: 成功创建 %d 个输出", count)
			}

			return count
		},

		"append_state_output": func(ctx context.Context, m api.Module, stateIDPtr uint32, stateIDLen uint32, version uint64, resultHashPtr uint32, publicInputsPtr uint32, publicInputsLen uint32, parentHashPtr uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("append_state_output: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("append_state_output: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 读取 stateID
			if stateIDLen == 0 {
				if a.logger != nil {
					a.logger.Warn("append_state_output: stateID长度无效")
				}
				return ErrInvalidParameter
			}

			stateIDBytes, ok := memory.Read(stateIDPtr, stateIDLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_state_output: 读取stateID失败 ptr=%d len=%d", stateIDPtr, stateIDLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 读取 executionResultHash（32字节）
			if resultHashPtr == 0 {
				if a.logger != nil {
					a.logger.Warn("append_state_output: resultHashPtr无效")
				}
				return ErrInvalidParameter
			}

			resultHashBytes, ok := memory.Read(resultHashPtr, 32)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("append_state_output: 读取resultHash失败 ptr=%d", resultHashPtr)
				}
				return ErrMemoryAccessFailed
			}

			// 3. 读取可选的 publicInputs
			var publicInputs []byte
			if publicInputsPtr != 0 && publicInputsLen > 0 {
				publicInputs, ok = memory.Read(publicInputsPtr, publicInputsLen)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_state_output: 读取publicInputs失败 ptr=%d len=%d", publicInputsPtr, publicInputsLen)
					}
					return ErrMemoryAccessFailed
				}
			}

			// 4. 读取可选的 parentStateHash（32字节）
			var parentStateHash []byte
			if parentHashPtr != 0 {
				parentStateHash, ok = memory.Read(parentHashPtr, 32)
				if !ok {
					if a.logger != nil {
						a.logger.Warnf("append_state_output: 读取parentHash失败 ptr=%d", parentHashPtr)
					}
					return ErrMemoryAccessFailed
				}
			}

			// 5. 调用 hostABI.TxAddStateOutput
			outputIndex, err := hostABI.TxAddStateOutput(ctx, stateIDBytes, version, resultHashBytes, publicInputs, parentStateHash)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("append_state_output: 添加输出失败 err=%v", err)
				}
				return ErrInternalError
			}

			stateIDDisplay := stateIDBytes
			if len(stateIDDisplay) > 8 {
				stateIDDisplay = stateIDDisplay[:8]
			}
			if a.logger != nil {
				a.logger.Debugf("append_state_output: 成功 stateID=%x version=%d outputIndex=%d", stateIDDisplay, version, outputIndex)
			}

			return outputIndex
		},

		// 注意：seal_transaction_draft 已移除
		// 草稿的 Seal 操作由 TX 层在执行完成后统一处理，
		// 而不是在执行期间由宿主函数调用

		// ═══════════════════════════════════════════════
		// 类别 D：资源查询（只读）
		// ═══════════════════════════════════════════════

		"resource_lookup": func(ctx context.Context, m api.Module, contentHashPtr uint32, contentHashLen uint32, resourcePtr uint32, resourceSize uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("resource_lookup: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 contentHash（32字节）
			if contentHashLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: contentHash长度无效 len=%d", contentHashLen)
				}
				return ErrInvalidHash
			}

			contentHashBytes, ok := memory.Read(contentHashPtr, contentHashLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: 读取contentHash失败 ptr=%d len=%d", contentHashPtr, contentHashLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 调用 hostABI.ResourceLookup
			resource, err := hostABI.ResourceLookup(ctx, contentHashBytes)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: 查询失败 contentHash=%x err=%v", contentHashBytes[:8], err)
				}
				return ErrResourceNotFound
			}

			if resource == nil {
				if a.logger != nil {
					a.logger.Debugf("resource_lookup: 资源不存在 contentHash=%x", contentHashBytes[:8])
				}
				return ErrResourceNotFound
			}

			// 3. 将 Resource 序列化并写入 WASM 内存
			resourceBytes, err := proto.Marshal(resource)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: 序列化失败 err=%v", err)
				}
				return ErrEncodingFailed
			}

			if uint32(len(resourceBytes)) > resourceSize {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: 输出缓冲区太小 required=%d provided=%d", len(resourceBytes), resourceSize)
				}
				return ErrBufferTooSmall
			}

			if !memory.Write(resourcePtr, resourceBytes) {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup: 写入内存失败 ptr=%d len=%d", resourcePtr, len(resourceBytes))
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("resource_lookup: 成功 contentHash=%x resourceLen=%d", contentHashBytes[:8], len(resourceBytes))
			}

			return uint32(len(resourceBytes))
		},

		// resource_lookup_json - 资源查询（JSON格式，TinyGo友好）
		// 签名: (content_hash_ptr: u32, content_hash_len: u32, resource_ptr: u32, resource_size: u32) -> (actual_len: u32)
		// 返回: 实际写入的JSON字节数，0表示失败
		"resource_lookup_json": func(ctx context.Context, m api.Module, contentHashPtr uint32, contentHashLen uint32, resourcePtr uint32, resourceSize uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("resource_lookup_json: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 contentHash（32字节）
			if contentHashLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: contentHash长度无效 len=%d", contentHashLen)
				}
				return ErrInvalidHash
			}

			contentHashBytes, ok := memory.Read(contentHashPtr, contentHashLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: 读取contentHash失败 ptr=%d len=%d", contentHashPtr, contentHashLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 调用 hostABI.ResourceLookup
			resource, err := hostABI.ResourceLookup(ctx, contentHashBytes)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: 查询失败 contentHash=%x err=%v", contentHashBytes[:8], err)
				}
				return ErrResourceNotFound
			}

			if resource == nil {
				if a.logger != nil {
					a.logger.Debugf("resource_lookup_json: 资源不存在 contentHash=%x", contentHashBytes[:8])
				}
				return ErrResourceNotFound
			}

			// 3. 将 Resource 序列化为JSON（而非Protobuf）
			resourceJSON, err := json.Marshal(resource)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: JSON序列化失败 err=%v", err)
				}
				return ErrEncodingFailed
			}

			if uint32(len(resourceJSON)) > resourceSize {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: 输出缓冲区太小 required=%d provided=%d", len(resourceJSON), resourceSize)
				}
				return ErrBufferTooSmall
			}

			if !memory.Write(resourcePtr, resourceJSON) {
				if a.logger != nil {
					a.logger.Warnf("resource_lookup_json: 写入内存失败 ptr=%d len=%d", resourcePtr, len(resourceJSON))
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("resource_lookup_json: 成功 contentHash=%x jsonLen=%d", contentHashBytes[:8], len(resourceJSON))
			}

			return uint32(len(resourceJSON))
		},

		"resource_exists": func(ctx context.Context, m api.Module, contentHashPtr uint32, contentHashLen uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("resource_exists: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// ⚠️ **边界检查**：验证内存大小是否有效
			memSize := memory.Size()
			if memSize == 0 {
				if a.logger != nil {
					a.logger.Warn("resource_exists: 内存大小无效")
				}
				return ErrMemoryAccessFailed
			}

			// 1. 从 WASM 内存读取 contentHash（32字节）
			if contentHashLen != 32 {
				if a.logger != nil {
					a.logger.Warnf("resource_exists: contentHash长度无效 len=%d", contentHashLen)
				}
				return ErrInvalidHash
			}

			// ⚠️ **边界检查**：验证地址范围
			if contentHashPtr+contentHashLen > memSize {
				if a.logger != nil {
					a.logger.Warnf("resource_exists: 地址越界 ptr=%d len=%d memSize=%d", contentHashPtr, contentHashLen, memSize)
				}
				return ErrMemoryAccessFailed
			}

			contentHashBytes, ok := memory.Read(contentHashPtr, contentHashLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("resource_exists: 读取contentHash失败 ptr=%d len=%d", contentHashPtr, contentHashLen)
				}
				return ErrMemoryAccessFailed
			}

			// 2. 调用 hostABI.ResourceExists
			exists, err := hostABI.ResourceExists(ctx, contentHashBytes)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("resource_exists: 查询失败 contentHash=%x err=%v", contentHashBytes[:8], err)
				}
				return 0 // 查询失败视为不存在
			}

			// 3. 返回 1（存在）或 0（不存在）
			if exists {
				if a.logger != nil {
					a.logger.Debugf("resource_exists: 资源存在 contentHash=%x", contentHashBytes[:8])
				}
				return 1
			}

			if a.logger != nil {
				a.logger.Debugf("resource_exists: 资源不存在 contentHash=%x", contentHashBytes[:8])
			}
			return 0
		},

		// ═══════════════════════════════════════════════
		// 类别 E：高阶交易构建（host_build_transaction）
		// ═══════════════════════════════════════════════

		"host_build_transaction": func(ctx context.Context, m api.Module, draftJSONPtr uint32, draftJSONLen uint32, receiptPtr uint32, receiptSize uint32) uint32 {
			// 🎯 **核心宿主函数**：批量构建交易并返回 TxReceipt（✅ 完整实现）
			//
			// 📋 **参数**：
			//   - ctx: 执行上下文
			//   - m: WASM 模块实例（用于访问内存）
			//   - draftJSONPtr: Draft JSON 指针（在 WASM 内存中）
			//   - draftJSONLen: Draft JSON 长度
			//   - receiptPtr: TxReceipt 写入指针（在 WASM 内存中）
			//   - receiptSize: TxReceipt 缓冲区大小
			//
			// 🔧 **返回值**：
			//   - 0: 成功
			//   - 1001: ErrInvalidParameter
			//   - 1005: ErrBufferTooSmall
			//   - 5001: ErrInternalError
			//   - 5002: ErrEncodingFailed
			//
			// 🔄 **流程**：
			//   1. 从 WASM 内存读取 Draft JSON
			//   2. 解析并构建交易
			//   3. 编码 TxReceipt 为 JSON
			//   4. 检查缓冲区大小
			//   5. 将 TxReceipt JSON 写入 WASM 内存

			// 1. 检查 TxAdapter 是否已注入
			if a.txAdapter == nil {
				if a.logger != nil {
					a.logger.Warn("host_build_transaction: TxAdapter未注入")
				}
				return ErrServiceUnavailable
			}

			// 2. 从 WASM 内存读取 Draft JSON
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("host_build_transaction: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			draftJSONBytes, ok := memory.Read(draftJSONPtr, draftJSONLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("host_build_transaction: 无法读取Draft JSON ptr=%d len=%d", draftJSONPtr, draftJSONLen)
				}
				return ErrInvalidParameter
			}

			// 从ctx动态提取ExecutionContext
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Warn("host_build_transaction: ExecutionContext未找到")
				}
				return ErrContextNotFound
			}

			// 4. 调用核心业务逻辑构建交易
			// 注意：buildTxFromDraft函数已经在host_function_provider中适配好了，
			// 它内部使用hostabi.TxAdapter（p.txAdapter），不需要我们在适配器中再次类型断言
			if a.buildTxFromDraft == nil {
				if a.logger != nil {
					a.logger.Error("host_build_transaction: buildTxFromDraft函数未设置")
				}
				return ErrServiceUnavailable
			}

			// 注意：buildTxFromDraft的第一个参数（txAdapter）在适配函数中已经处理好了（使用p.txAdapter）
			// 这里的a.txAdapter是interface{}类型，传递给适配函数，适配函数会忽略它并使用实际的hostabi.TxAdapter
			// 为了保持函数签名一致，我们传递a.txAdapter（虽然适配函数内部不会使用它）
			receipt, err := a.buildTxFromDraft(
				ctx,
				a.txAdapter, // 传递interface{}，适配函数内部使用实际的hostabi.TxAdapter
				a.txHashClient,
				a.eutxoQuery,                        // UTXO查询服务（用于paymaster模式）
				currentExecCtx.GetCallerAddress(),   // 调用者地址（用于delegated模式）
				currentExecCtx.GetContractAddress(), // ✅ 合约地址（用于设置合约代币输出的contract_address）
				draftJSONBytes,
				currentExecCtx.GetBlockHeight(),
				currentExecCtx.GetBlockTimestamp(),
			)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("host_build_transaction: 交易构建失败 err=%v", err)
				}
				return ErrInternalError
			}

			// 5. 编码 TxReceipt 为 JSON
			if a.encodeTxReceipt == nil {
				if a.logger != nil {
					a.logger.Error("host_build_transaction: encodeTxReceipt函数未设置")
				}
				return ErrServiceUnavailable
			}

			receiptJSON, encodeErr := a.encodeTxReceipt(receipt)
			if encodeErr != nil {
				if a.logger != nil {
					a.logger.Errorf("host_build_transaction: TxReceipt编码失败 err=%v", encodeErr)
				}
				return ErrEncodingFailed
			}

			// 6. 检查缓冲区大小
			if uint32(len(receiptJSON)) > receiptSize {
				if a.logger != nil {
					a.logger.Warnf("host_build_transaction: 缓冲区太小 required=%d available=%d", len(receiptJSON), receiptSize)
				}
				return ErrBufferTooSmall
			}

			// 7. 将 TxReceipt JSON 写入 WASM 内存
			if !memory.Write(receiptPtr, receiptJSON) {
				if a.logger != nil {
					a.logger.Warnf("host_build_transaction: 写入内存失败 ptr=%d len=%d", receiptPtr, len(receiptJSON))
				}
				return ErrMemoryAccessFailed
			}

			if a.logger != nil {
				a.logger.Debugf("host_build_transaction: 成功 mode=%s receiptLen=%d", receipt.Mode, len(receiptJSON))
			}

			// 8. 成功
			return 0
		},

		// ═══════════════════════════════════════════════
		// 类别 F：ISPC 创新函数（受控外部交互）
		// ═══════════════════════════════════════════════

		// host_declare_external_state - 声明外部状态预期（ISPC创新）
		// 签名: (claim_ptr: u32, claim_len: u32, claim_id_ptr: u32, claim_id_size: u32) -> (status: u32)
		// 返回: 0=成功, 非0=失败
		// ⚠️ **注意**：此功能还在开发中，当前返回错误
		"host_declare_external_state": func(ctx context.Context, m api.Module, claimPtr uint32, claimLen uint32, claimIDPtr uint32, claimIDSize uint32) uint32 {
			if a.logger != nil {
				a.logger.Warn("host_declare_external_state: 功能还在开发中，暂不支持")
			}
			return ErrNotImplemented
		},

		// host_provide_evidence - 提供验证佐证（ISPC创新）
		// 签名: (claim_id_ptr: u32, claim_id_len: u32, evidence_ptr: u32, evidence_len: u32) -> (status: u32)
		// 返回: 0=成功, 非0=失败
		// ⚠️ **注意**：此功能还在开发中，当前返回错误
		"host_provide_evidence": func(ctx context.Context, m api.Module, claimIDPtr uint32, claimIDLen uint32, evidencePtr uint32, evidenceLen uint32) uint32 {
			if a.logger != nil {
				a.logger.Warn("host_provide_evidence: 功能还在开发中，暂不支持")
			}
			return ErrNotImplemented
		},

		// host_query_controlled_state - 查询受控外部状态（ISPC创新）
		// 签名: (claim_id_ptr: u32, claim_id_len: u32, result_ptr: u32, result_size: u32) -> (actual_len: u32)
		// 返回: 实际写入的字节数，0表示失败
		// ⚠️ **注意**：此功能还在开发中，当前返回错误
		"host_query_controlled_state": func(ctx context.Context, m api.Module, claimIDPtr uint32, claimIDLen uint32, resultPtr uint32, resultSize uint32) uint32 {
			if a.logger != nil {
				a.logger.Warn("host_query_controlled_state: 功能还在开发中，暂不支持")
			}
			return 0 // 失败
		},

		// ═══════════════════════════════════════════════
		// 类别 G：合约运行时函数（新增 - 支持TinyGo合约）
		// ═══════════════════════════════════════════════

		// malloc - WASM内存分配
		// 签名: (size: u32) -> (ptr: u32)
		"malloc": func(ctx context.Context, m api.Module, size uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("malloc: 无法获取WASM内存")
				}
				return 0
			}

			// 获取或创建该模块的分配器
			moduleName := m.Name()
			if moduleName == "" {
				moduleName = "default"
			}
			allocator := a.getOrCreateAllocator(moduleName, memory)

			// 执行分配
			ptr, err := allocator.allocate(memory, size)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("malloc: 分配 %d 字节失败: %v", size, err)
				}
				return 0
			}

			if a.logger != nil {
				memSize := uint32(memory.Size())
				a.logger.Debugf("malloc: 分配 %d 字节 -> ptr=%d (内存: %d bytes / %.2f KB)",
					size, ptr, memSize, float64(memSize)/1024)
			}

			return ptr
		},

		// node_add - 简单加法（测试/演示用）
		// 签名: (a: i32, b: i32) -> (result: i32)
		"node_add": func(x, y int32) int32 {
			result := x + y
			if a.logger != nil {
				a.logger.Infof("🔧 node_add: %d + %d = %d", x, y, result)
			}
			return result
		},

		// get_timestamp - 获取区块时间戳（framework需要）
		// 签名: () -> (timestamp: u64)
		// ⚠️ 注意：这个函数没有ctx参数，无法动态获取ExecutionContext
		// 但timestamp是确定性的（从区块高度查询），所以可以接受闭包捕获
		"get_timestamp": func() uint64 {
			// ⚠️ 注意：此函数没有ctx参数，但可以通过hostABI获取timestamp
			timestamp, err := hostABI.GetBlockTimestamp(ctx)
			if err != nil {
				if a.logger != nil {
					a.logger.Warnf("get_timestamp: 获取时间戳失败: %v", err)
				}
				return 0
			}
			if a.logger != nil {
				a.logger.Infof("🔧 get_timestamp: %d", timestamp)
			}
			return timestamp
		},

		// get_contract_init_params - 获取合约初始化参数（framework需要）
		// 签名: (buf_ptr: u32, buf_len: u32) -> (actual_len: u32)
		// 返回实际参数长度，如果buf_len不够则返回所需长度但不写入
		"get_contract_init_params": func(ctx context.Context, m api.Module, bufPtr uint32, bufLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				return 0
			}

			memory := m.Memory()
			if memory == nil {
				return 0
			}

			// 从ExecutionContext获取initParams
			initParams, err := currentExecCtx.GetInitParams()
			if err != nil || len(initParams) == 0 {
				return 0 // 无参数
			}

			actualLen := uint32(len(initParams))

			// 如果缓冲区足够，写入参数
			if bufLen >= actualLen {
				if !memory.Write(bufPtr, initParams) {
					if a.logger != nil {
						a.logger.Error("get_contract_init_params: 写入内存失败")
					}
					return 0
				}
			}

			if a.logger != nil {
				a.logger.Debugf("get_contract_init_params: %d 字节", actualLen)
			}

			return actualLen
		},

		// log_debug - 记录调试日志
		// 签名: (message_ptr: u32, message_len: u32) -> (status: u32)
		// 返回: 0=成功, 非0=失败
		"log_debug": func(ctx context.Context, m api.Module, messagePtr uint32, messageLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Warn("log_debug: 无法获取WASM内存")
				}
				return ErrMemoryAccessFailed
			}

			// 从 WASM 内存读取日志消息
			messageBytes, ok := memory.Read(messagePtr, messageLen)
			if !ok {
				if a.logger != nil {
					a.logger.Warnf("log_debug: 读取消息失败 ptr=%d len=%d", messagePtr, messageLen)
				}
				return ErrMemoryAccessFailed
			}

			message := string(messageBytes)

			// 记录调试日志
			if currentExecCtx != nil && a.logger != nil {
				execID := currentExecCtx.GetExecutionID()
				a.logger.Debugf("[Contract:%s] %s", execID, message)
			} else if a.logger != nil {
				a.logger.Debugf("[Contract] %s", message)
			}

			return 0 // 成功
		},

		// set_return_data - 设置返回数据
		// 签名: (data_ptr: u32, data_len: u32) -> (status: u32)
		// 返回: 0=成功, 1=失败
		"set_return_data": func(ctx context.Context, m api.Module, dataPtr uint32, dataLen uint32) uint32 {
			// ⚠️ **关键修复**：从ctx动态提取ExecutionContext，而不是闭包捕获
			// 原因：宿主函数只注册一次，但每次调用的ExecutionContext不同
			//      如果闭包捕获，第二次调用会使用旧的ExecutionContext
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Error("set_return_data: ExecutionContext 未从 context 中找到")
				}
				return 1 // 失败
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("set_return_data: 无法获取WASM内存")
				}
				return 1 // 失败
			}

			// 从 WASM 内存读取返回数据
			data, ok := memory.Read(dataPtr, dataLen)
			if !ok {
				if a.logger != nil {
					a.logger.Errorf("set_return_data: 无法从WASM内存读取数据 (ptr=%d, len=%d)", dataPtr, dataLen)
				}
				return 1 // 失败
			}

			// 保存到ExecutionContext
			if a.logger != nil {
				a.logger.Infof("🔧 set_return_data: 准备设置到 ExecutionContext (ID=%s)", currentExecCtx.GetExecutionID())
			}

			if err := currentExecCtx.SetReturnData(data); err != nil {
				if a.logger != nil {
					a.logger.Errorf("set_return_data: 保存失败: %v", err)
				}
				return 1 // 失败
			}

			if a.logger != nil {
				a.logger.Infof("🔧 set_return_data: 已设置返回数据到ExecutionContext[%s] (%d 字节): %s", currentExecCtx.GetExecutionID(), len(data), string(data))
			}

			return 0 // 成功
		},

		// emit_event - 发出事件
		// 签名: (event_ptr: u32, event_len: u32) -> (status: u32)
		// 返回: 0=成功, 1=失败
		// 注意: event数据为JSON格式，由SDK序列化
		"emit_event": func(ctx context.Context, m api.Module, eventPtr uint32, eventLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				if a.logger != nil {
					a.logger.Error("emit_event: ExecutionContext 未从 context 中找到")
				}
				return 1 // 失败
			}

			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("emit_event: 无法获取WASM内存")
				}
				return 1 // 失败
			}

			// 从 WASM 内存读取事件JSON
			eventJSON, ok := memory.Read(eventPtr, eventLen)
			if !ok {
				if a.logger != nil {
					a.logger.Errorf("emit_event: 无法从WASM内存读取事件 (ptr=%d, len=%d)", eventPtr, eventLen)
				}
				return 1 // 失败
			}

			// 发出事件（保存到ExecutionContext）
			event := &ispcInterfaces.Event{
				Type:      "contract_event",
				Timestamp: int64(currentExecCtx.GetBlockTimestamp()),
				Data: map[string]interface{}{
					"json_payload": string(eventJSON),
				},
			}
			if err := currentExecCtx.AddEvent(event); err != nil {
				if a.logger != nil {
					a.logger.Errorf("emit_event: 保存事件失败: %v", err)
				}
				return 1 // 失败
			}

			if a.logger != nil {
				a.logger.Debugf("emit_event: 已发出事件 (%d 字节): %s", len(eventJSON), string(eventJSON))
			}

			return 0 // 成功
		},

		// state_get - 状态读取（从ExecutionContext的draft查询）
		// 签名: (key_ptr: u32, key_len: u32, value_ptr: u32, value_len: u32) -> (status: u32)
		// 返回: 0=成功, 1=失败/不存在
		"state_get": func(ctx context.Context, m api.Module, keyPtr uint32, keyLen uint32, valuePtr uint32, valueLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				return 1 // 失败
			}

			memory := m.Memory()
			if memory == nil {
				return 1 // 失败
			}

			// 从 WASM 内存读取 key
			keyBytes, ok := memory.Read(keyPtr, keyLen)
			if !ok {
				return 1 // 失败
			}

			// 从ExecutionContext获取draft（内存中的）
			draft, err := currentExecCtx.GetTransactionDraft()
			if err != nil || draft == nil {
				return 1 // 无draft
			}

			// 遍历draft的outputs，查找匹配的StateOutput
			if draft.Tx == nil || draft.Tx.Outputs == nil {
				return 1 // draft无输出
			}

			outputs := draft.Tx.Outputs
			var foundValue []byte
			for i := len(outputs) - 1; i >= 0; i-- { // 倒序查找，获取最新的
				output := outputs[i]
				if stateOut := output.GetState(); stateOut != nil {
					if string(stateOut.GetStateId()) == string(keyBytes) {
						// 找到了！提取executionResultHash作为value
						foundValue = stateOut.GetExecutionResultHash()
						break
					}
				}
			}

			if foundValue == nil {
				return 1 // 不存在
			}

			// 检查缓冲区大小
			if uint32(len(foundValue)) > valueLen {
				return 1 // 缓冲区太小
			}

			// 写入value到WASM内存
			if !memory.Write(valuePtr, foundValue) {
				return 1 // 写入失败
			}

			if a.logger != nil {
				a.logger.Debugf("state_get: key=%s, value_len=%d", string(keyBytes), len(foundValue))
			}

			return 0 // 成功
		},

		// state_get_from_chain - 从链上查询历史状态
		// 签名: (state_id_ptr: u32, state_id_len: u32, value_ptr: u32, value_len: u32, version_ptr: u32) -> (status: u32)
		// 返回: 0=成功, 1=失败/不存在
		// 说明: 查询链上已确认交易中的StateOutput，返回匹配stateID的最新状态值和版本号
		"state_get_from_chain": func(ctx context.Context, m api.Module, stateIDPtr uint32, stateIDLen uint32, valuePtr uint32, valueLen uint32, versionPtr uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				return 1 // 失败
			}

			// 从 WASM 内存读取 stateID
			stateIDBytes, ok := memory.Read(stateIDPtr, stateIDLen)
			if !ok {
				return 1 // 失败
			}

			// 查询链上历史状态
			foundValue, foundVersion, err := a.queryStateFromChain(ctx, stateIDBytes)
			if err != nil || foundValue == nil {
				if a.logger != nil {
					a.logger.Debugf("state_get_from_chain: stateID=%s, 未找到", string(stateIDBytes))
				}
				return 1 // 不存在
			}

			// 检查缓冲区大小
			if uint32(len(foundValue)) > valueLen {
				return ErrBufferTooSmall // 缓冲区太小
			}

			// 写入value到WASM内存
			if !memory.Write(valuePtr, foundValue) {
				return 1 // 写入失败
			}

			// 写入版本号（8字节uint64）
			versionBytes := make([]byte, 8)
			versionBytes[0] = byte(foundVersion >> 56)
			versionBytes[1] = byte(foundVersion >> 48)
			versionBytes[2] = byte(foundVersion >> 40)
			versionBytes[3] = byte(foundVersion >> 32)
			versionBytes[4] = byte(foundVersion >> 24)
			versionBytes[5] = byte(foundVersion >> 16)
			versionBytes[6] = byte(foundVersion >> 8)
			versionBytes[7] = byte(foundVersion)
			if !memory.Write(versionPtr, versionBytes) {
				return 1 // 写入失败
			}

			if a.logger != nil {
				a.logger.Debugf("state_get_from_chain: stateID=%s, value_len=%d, version=%d", string(stateIDBytes), len(foundValue), foundVersion)
			}

			return 0 // 成功
		},

		// state_set - 状态写入（直接操作ExecutionContext的draft）
		// 签名: (key_ptr: u32, key_len: u32, value_ptr: u32, value_len: u32) -> (status: u32)
		// 返回: 0=成功, 1=失败
		"state_set": func(ctx context.Context, m api.Module, keyPtr uint32, keyLen uint32, valuePtr uint32, valueLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				return 1 // 失败
			}

			memory := m.Memory()
			if memory == nil {
				return 1 // 失败
			}

			// 从 WASM 内存读取 key 和 value
			keyBytes, ok := memory.Read(keyPtr, keyLen)
			if !ok {
				return 1 // 失败
			}

			valueBytes, ok := memory.Read(valuePtr, valueLen)
			if !ok {
				return 1 // 失败
			}

			// 获取ExecutionContext的draft（内存中的）
			draft, err := currentExecCtx.GetTransactionDraft()
			if err != nil || draft == nil {
				if a.logger != nil {
					a.logger.Errorf("state_set: 获取draft失败: %v", err)
				}
				return 1 // 失败
			}

			// 创建StateOutput
			stateOutput := &pb.TxOutput{
				OutputContent: &pb.TxOutput_State{
					State: &pb.StateOutput{
						StateId:             keyBytes,
						StateVersion:        1,
						ExecutionResultHash: valueBytes,
						ZkProof: &pb.ZKStateProof{
							// ⚠️ **占位符说明**：
							// Proof字段在此处设置为空字节数组作为占位符，实际的ZK证明将在以下时机生成：
							// 1. 同步模式：在coordinator.ExecuteContract执行完成后，通过generateZKProof生成
							// 2. 异步模式：通过异步ZK证明工作池生成，完成后通过回调更新StateOutput
							// 3. 替换时机：在交易最终化（FinalizeTransaction）之前，Proof必须被填充
							// 4. 验证要求：如果Proof为空，交易验证将失败，确保占位符必须被替换
							// 参考：internal/core/ispc/coordinator/execute_contract.go (generateZKProof)
							// 参考：internal/core/ispc/coordinator/async_zk_proof.go (submitZKProofTask)
							Proof:        []byte{},
							PublicInputs: nil,
						},
						ParentStateHash: nil,
					},
				},
			}

			// 添加到draft.Tx.Outputs（直接操作内存中的draft）
			if draft.Tx == nil {
				draft.Tx = &pb.Transaction{
					Inputs:  []*pb.TxInput{},
					Outputs: []*pb.TxOutput{},
				}
			}
			draft.Tx.Outputs = append(draft.Tx.Outputs, stateOutput)

			// 更新ExecutionContext的draft
			if err := currentExecCtx.UpdateTransactionDraft(draft); err != nil {
				if a.logger != nil {
					a.logger.Errorf("state_set: 更新draft失败: %v", err)
				}
				return 1 // 失败
			}

			if a.logger != nil {
				a.logger.Debugf("state_set: key=%s, value_len=%d", string(keyBytes), len(valueBytes))
			}

			return 0 // 成功
		},

		// state_exists - 状态存在性检查（从ExecutionContext的draft查询）
		// 签名: (key_ptr: u32, key_len: u32) -> (exists: u32)
		// 返回: 1=存在, 0=不存在
		"state_exists": func(ctx context.Context, m api.Module, keyPtr uint32, keyLen uint32) uint32 {
			currentExecCtx := a.getExecCtxFunc(ctx)
			if currentExecCtx == nil {
				return 0 // 不存在
			}

			memory := m.Memory()
			if memory == nil {
				return 0 // 不存在
			}

			// 从 WASM 内存读取 key
			keyBytes, ok := memory.Read(keyPtr, keyLen)
			if !ok {
				return 0 // 不存在
			}

			// 从ExecutionContext获取draft（内存中的）
			draft, err := currentExecCtx.GetTransactionDraft()
			if err != nil || draft == nil {
				return 0 // 无draft
			}

			// 遍历draft的outputs，查找匹配的StateOutput
			if draft.Tx == nil || draft.Tx.Outputs == nil {
				return 0 // draft无输出
			}

			outputs := draft.Tx.Outputs
			for i := len(outputs) - 1; i >= 0; i-- {
				output := outputs[i]
				if stateOut := output.GetState(); stateOut != nil {
					if string(stateOut.GetStateId()) == string(keyBytes) {
						return 1 // 存在
					}
				}
			}

			return 0 // 不存在
		},

		// ═══════════════════════════════════════════════
		// 类别 E：地址编码转换
		// ═══════════════════════════════════════════════

		// address_bytes_to_base58 - 地址字节转Base58
		// 签名: (addr_ptr: u32, result_ptr: u32, max_len: u32) -> (actual_len: u32)
		// ✅ 使用 btcutil/base58 进行标准 Base58 编码
		"address_bytes_to_base58": func(ctx context.Context, m api.Module, addrPtr uint32, resultPtr uint32, maxLen uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("address_bytes_to_base58: 无法获取内存")
				}
				return 0
			}

			// 读取20字节地址
			addressBytes, ok := memory.Read(addrPtr, 20)
			if !ok {
				if a.logger != nil {
					a.logger.Error("address_bytes_to_base58: 读取地址失败")
				}
				return 0
			}

			// ✅ 使用 AddressManager 进行标准 Base58Check 编码
			// 这样才能得到正确的 WES 地址格式（带版本字节和校验和）
			if a.addressManager == nil {
				if a.logger != nil {
					a.logger.Error("address_bytes_to_base58: AddressManager 未初始化")
				}
				return 0
			}

			base58Str, err := a.addressManager.BytesToAddress(addressBytes)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("address_bytes_to_base58: Base58Check 编码失败: %v", err)
				}
				return 0
			}

			base58Bytes := []byte(base58Str)
			base58Len := uint32(len(base58Bytes))

			// 检查长度
			if base58Len > maxLen {
				if a.logger != nil {
					a.logger.Errorf("address_bytes_to_base58: Base58Check长度 %d 超过最大长度 %d", base58Len, maxLen)
				}
				return 0
			}

			// 写入WASM内存
			if !memory.Write(resultPtr, base58Bytes) {
				if a.logger != nil {
					a.logger.Error("address_bytes_to_base58: 写入失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Infof("🔧 address_bytes_to_base58: %x -> %s", addressBytes, base58Str)
			}

			return base58Len
		},

		// address_base58_to_bytes - Base58转地址字节
		// 签名: (base58_ptr: u32, base58_len: u32, result_ptr: u32) -> (success: u32)
		// ✅ 使用 AddressManager 进行标准 Base58Check 解码
		"address_base58_to_bytes": func(ctx context.Context, m api.Module, base58Ptr uint32, base58Len uint32, resultPtr uint32) uint32 {
			memory := m.Memory()
			if memory == nil {
				if a.logger != nil {
					a.logger.Error("address_base58_to_bytes: 无法获取内存")
				}
				return 0
			}

			// 读取字符串
			strBytes, ok := memory.Read(base58Ptr, base58Len)
			if !ok {
				if a.logger != nil {
					a.logger.Error("address_base58_to_bytes: 读取字符串失败")
				}
				return 0
			}

			str := string(strBytes)

			// ✅ 使用 AddressManager 进行标准 Base58Check 解码
			if a.addressManager == nil {
				if a.logger != nil {
					a.logger.Error("address_base58_to_bytes: AddressManager 未初始化")
				}
				return 0
			}

			addressBytes, err := a.addressManager.AddressToBytes(str)
			if err != nil {
				if a.logger != nil {
					a.logger.Errorf("address_base58_to_bytes: Base58Check 解码失败: %v", err)
				}
				return 0
			}

			if len(addressBytes) != 20 {
				if a.logger != nil {
					a.logger.Errorf("address_base58_to_bytes: 解码后长度错误: %d (期望20)", len(addressBytes))
				}
				return 0
			}

			// 写入20字节地址
			if !memory.Write(resultPtr, addressBytes) {
				if a.logger != nil {
					a.logger.Error("address_base58_to_bytes: 写入失败")
				}
				return 0
			}

			if a.logger != nil {
				a.logger.Infof("🔧 address_base58_to_bytes: %s -> %x", str, addressBytes)
			}

			return 1 // 成功
		},
	}
}

// queryStateFromChain 从链上查询历史状态
//
// 🎯 **用途**：查询链上已确认交易中的StateOutput，返回匹配stateID的最新状态值和版本号
//
// **查询策略**：
// 1. 从链尖开始向后查找（最多查找最近100个区块，避免性能问题）
// 2. 遍历每个区块的交易
// 3. 查找包含匹配stateID的StateOutput
// 4. 返回版本号最高的那个
//
// **参数**：
//   - ctx: 上下文
//   - stateID: 状态ID
//
// **返回**：
//   - value: 状态值（executionResultHash）
//   - version: 状态版本号
//   - error: 错误信息
//
// **性能优化**：
//   - 当前实现从链尖向后查找，效率较低
//   - 后续可以维护状态索引（stateID -> 最新的StateOutput的OutPoint）以提高查询效率
func (a *WASMAdapter) queryStateFromChain(ctx context.Context, stateID []byte) ([]byte, uint64, error) {
	// 1. 获取当前链高度
	currentHeight, err := a.chainQuery.GetCurrentHeight(ctx)
	if err != nil {
		if a.logger != nil {
			a.logger.Debugf("queryStateFromChain: 获取链高度失败: %v", err)
		}
		return nil, 0, err
	}

	// 2. 限制查找范围（最多查找最近100个区块，避免性能问题）
	maxBlocksToSearch := uint64(100)
	startHeight := uint64(0)
	if currentHeight > maxBlocksToSearch {
		startHeight = currentHeight - maxBlocksToSearch
	}

	// 3. 从链尖开始向后查找
	var foundValue []byte
	var foundVersion uint64 = 0

	for height := currentHeight; height >= startHeight && height > 0; height-- {
		// 获取区块
		block, err := a.blockQuery.GetBlockByHeight(ctx, height)
		if err != nil {
			if a.logger != nil {
				a.logger.Debugf("queryStateFromChain: 获取区块失败 height=%d: %v", height, err)
			}
			continue
		}

		// 遍历区块中的交易
		if block.Body == nil || block.Body.Transactions == nil {
			continue
		}

		for _, tx := range block.Body.Transactions {
			if tx == nil || tx.Outputs == nil {
				continue
			}

			// 遍历交易输出，查找StateOutput
			for _, output := range tx.Outputs {
				if output == nil {
					continue
				}

				stateOut := output.GetState()
				if stateOut == nil {
					continue
				}

				// 检查stateID是否匹配
				if len(stateOut.StateId) != len(stateID) {
					continue
				}

				match := true
				for i := 0; i < len(stateID); i++ {
					if stateOut.StateId[i] != stateID[i] {
						match = false
						break
					}
				}

				if match {
					// 找到匹配的StateOutput，检查版本号
					if stateOut.StateVersion > foundVersion {
						foundValue = stateOut.ExecutionResultHash
						foundVersion = stateOut.StateVersion
					}
				}
			}
		}

		// 如果找到了状态，可以提前退出（因为从链尖向后查找，找到的就是最新的）
		if foundValue != nil {
			break
		}
	}

	if foundValue == nil {
		return nil, 0, fmt.Errorf("状态不存在: stateID=%s", string(stateID))
	}

	return foundValue, foundVersion, nil
}
