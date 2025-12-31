// Package builder 提供 Type-state Builder 实现
//
// service.go: TxBuilder Service 实现
package builder

import (
	"context"
	"fmt"
	"sync"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// Service TxBuilder 服务实现
//
// 🎯 **核心职责**：纯装配器，提供流式 API 构建交易
//
// 💡 **设计理念**：
// TxBuilder 是纯装配器，只负责将输入输出装配成 Transaction，不做任何业务逻辑：
// - ❌ 不做 UTXO 选择（由 UTXOSelector 负责，P2 实现）
// - ❌ 不做费用估算（由 FeeEstimator 负责）
// - ❌ 不做验证（由 Verifier 负责）
// - ✅ 只提供装配能力，调用方决定输入输出组合
//
// ⚠️ **P1 MVP 约束**：
// - 只支持 AddAssetOutput（不支持 Resource/State）
// - 不做 UTXO 存在性检查
// - 不做余额检查
//
// 📞 **调用方**：ISPC、BLOCKCHAIN、CLI
type Service struct {
	mu           sync.Mutex                 // 保护 tx 的并发读写（测试中存在并发 AddOutput 场景）
	tx           *transaction.Transaction   // 正在构建的交易
	draftService tx.TransactionDraftService // Draft 服务（P3 新增）
}

// NewService 创建新的 TxBuilder Service
//
// 参数：
//   - draftService: Draft 服务（用于 CreateDraft/LoadDraft）
//
// 返回：
//   - *Service: 新创建的实例
func NewService(draftService tx.TransactionDraftService) *Service {
	return &Service{
		tx: &transaction.Transaction{
			Version: 1,
			Inputs:  make([]*transaction.TxInput, 0),
			Outputs: make([]*transaction.TxOutput, 0),
		},
		draftService: draftService,
	}
}

// AddInput 添加交易输入
//
// 🎯 **P1 MVP 逻辑**：
// - 只做装配，不验证 UTXO 是否存在
// - 不验证余额是否充足
// - 支持消费型和引用型输入
//
// 参数：
//   - outpoint: UTXO 引用（txid + index）
//   - isReferenceOnly: 是否为引用型输入
//
// 返回：
//   - *Service: 返回自身，支持链式调用
func (s *Service) AddInput(
	outpoint *transaction.OutPoint,
	isReferenceOnly bool,
) tx.TxBuilder {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tx.Inputs = append(s.tx.Inputs, &transaction.TxInput{
		PreviousOutput:  outpoint,
		IsReferenceOnly: isReferenceOnly,
		// UnlockingProof 将在 WithProofs() 阶段填充
	})
	return s
}

// SetExecutionProof 为最后一个输入设置 ExecutionProof
//
// 🎯 **用途**：用于铸造场景，为引用型输入设置 ExecutionProof
//
// ⚠️ **约束**：
// - 只能为引用型输入（is_reference_only=true）设置 ExecutionProof
// - 必须在 AddInput 之后调用
// - 如果最后一个输入不是引用型输入，返回错误
//
// 参数：
//   - executionProof: ExecutionProof（ISPC执行证明）
//
// 返回：
//   - *Service: 返回自身，支持链式调用
//   - error: 设置失败的原因
//
// 💡 **使用示例**（铸造场景）：
//
//	builder.
//	    AddInput(contractUTXO, true).  // 引用型输入
//	    SetExecutionProof(executionProof).
//	    AddAssetOutput(recipient, "1000", contractAddr, lock)
func (s *Service) SetExecutionProof(executionProof *transaction.ExecutionProof) (tx.TxBuilder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tx.Inputs) == 0 {
		return s, fmt.Errorf("没有输入，无法设置 ExecutionProof")
	}

	lastInput := s.tx.Inputs[len(s.tx.Inputs)-1]
	if !lastInput.IsReferenceOnly {
		return s, fmt.Errorf("只能为引用型输入设置 ExecutionProof")
	}

	lastInput.UnlockingProof = &transaction.TxInput_ExecutionProof{
		ExecutionProof: executionProof,
	}

	return s, nil
}

// AddAssetOutput 添加资产输出
//
// 🎯 **P1 MVP 逻辑**：
// - 只支持 NativeCoin 和 ContractToken（FungibleToken）
// - 不支持 NFT 和 SFT（后续阶段实现）
//
// 参数：
//   - owner: 输出所有者地址
//   - amount: 资产金额（字符串格式，支持大数）
//   - contractAddress: 合约地址（nil 表示原生币）
//   - lock: 锁定条件
//
// 返回：
//   - *Service: 返回自身，支持链式调用
func (s *Service) AddAssetOutput(
	owner []byte,
	amount string,
	contractAddress []byte,
	lock *transaction.LockingCondition,
) tx.TxBuilder {
	var assetOutput *transaction.AssetOutput

	if contractAddress == nil {
		// 原生币
		assetOutput = &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_NativeCoin{
				NativeCoin: &transaction.NativeCoinAsset{
					Amount: amount,
				},
			},
		}
	} else {
		// 合约代币（P1 只支持 Fungible Token）
		assetOutput = &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_ContractToken{
				ContractToken: &transaction.ContractTokenAsset{
					ContractAddress: contractAddress,
					TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
						FungibleClassId: []byte("default"), // P1 使用默认类别
					},
					Amount: amount,
				},
			},
		}
	}

	output := &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: []*transaction.LockingCondition{lock},
		OutputContent:     &transaction.TxOutput_Asset{Asset: assetOutput},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.Outputs = append(s.tx.Outputs, output)
	return s
}

// AddResourceOutput 添加资源输出
//
// 🎯 **核心逻辑**：
// - 纯装配：将 Resource 包装成 ResourceOutput 并添加到交易输出
// - 不做验证：不检查 content_hash 有效性、资源是否存在等
// - 不做业务逻辑：不做费用估算、存储分配等
//
// ⚠️ **P2 约束**：
// - 只做装配，资源内容由调用方提供
// - 权限控制通过 lock 参数指定
// - 生命周期控制（expiry_timestamp 等）在 Resource 对象中指定
//
// 参数：
//   - owner: 资源所有者地址
//   - resource: 完整的资源定义（from pb.blockchain.resource.Resource）
//   - lock: 锁定条件
//
// 返回：
//   - *Service: 返回自身，支持链式调用
//
// 💡 **使用示例**：
//
//	resource := &resourcepb.Resource{
//	    Category: resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
//	    ExecutableType: resourcepb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
//	    ContentHash: contractHash,
//	    MimeType: "application/wasm",
//	    Size: uint64(len(wasmBytes)),
//	    Contract: &resourcepb.ContractExecutionConfig{...},
//	}
//	builder.AddResourceOutput(ownerAddr, resource, singleKeyLock)
func (s *Service) AddResourceOutput(
	owner []byte,
	resource *resourcepb.Resource, // 使用具体类型确保类型安全
	lock *transaction.LockingCondition,
) *Service {
	// 构建 ResourceOutput
	resourceOutput := &transaction.ResourceOutput{
		Resource:          resource,
		CreationTimestamp: uint64(time.Now().Unix()),
		StorageStrategy:   transaction.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
		IsImmutable:       true, // 默认不可变
	}

	output := &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: []*transaction.LockingCondition{lock},
		OutputContent:     &transaction.TxOutput_Resource{Resource: resourceOutput},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.Outputs = append(s.tx.Outputs, output)
	return s
}

// AddStateOutput 添加状态输出
//
// 🎯 **核心逻辑**：
// - 纯装配：将状态数据包装成 StateOutput 并添加到交易输出
// - 不做验证：不验证 state_id 唯一性、ZK证明有效性等
// - 不做业务逻辑：不计算 execution_result_hash 等
//
// ⚠️ **P2 约束**：
// - 只做装配，状态内容（ZK证明等）由调用方提供
// - 权限控制通过 lock 参数指定
// - TTL 等生命周期参数在 StateOutput 对象中指定
//
// 参数：
//   - owner: 状态所有者地址
//   - stateID: 状态标识符（全局唯一）
//   - stateVersion: 状态版本号
//   - zkProof: 零知识证明（可选，nil 表示无证明）
//   - executionResultHash: 执行结果哈希
//   - lock: 锁定条件
//
// 返回：
//   - *Service: 返回自身，支持链式调用
//
// 💡 **使用示例**：
//
//	zkProof := &transaction.ZKStateProof{
//	    Proof: proofBytes,
//	    PublicInputs: publicInputs,
//	    ProvingScheme: "groth16",
//	    Curve: "bn254",
//	}
//	builder.AddStateOutput(ownerAddr, stateID, version, zkProof, resultHash, singleKeyLock)
func (s *Service) AddStateOutput(
	owner []byte,
	stateID []byte,
	stateVersion uint64,
	zkProof *transaction.ZKStateProof,
	executionResultHash []byte,
	lock *transaction.LockingCondition,
) *Service {
	// 构建 StateOutput
	stateOutput := &transaction.StateOutput{
		StateId:             stateID,
		StateVersion:        stateVersion,
		ZkProof:             zkProof,
		ExecutionResultHash: executionResultHash,
	}

	output := &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: []*transaction.LockingCondition{lock},
		OutputContent:     &transaction.TxOutput_State{State: stateOutput},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.Outputs = append(s.tx.Outputs, output)
	return s
}

// SetNonce 设置交易 nonce
//
// 参数：
//   - nonce: nonce 值
//
// 返回：
//   - *Service: 返回自身，支持链式调用
func (s *Service) SetNonce(nonce uint64) tx.TxBuilder {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.Nonce = nonce
	return s
}

// SetCreationTimestamp 设置交易创建时间戳
//
// 参数：
//   - timestamp: Unix 时间戳（秒）
//
// 返回：
//   - *Service: 返回自身，支持链式调用
func (s *Service) SetCreationTimestamp(timestamp uint64) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.CreationTimestamp = timestamp
	return s
}

// SetChainID 设置链 ID
//
// 参数：
//   - chainID: 链 ID
//
// 返回：
//   - *Service: 返回自身，支持链式调用
func (s *Service) SetChainID(chainID []byte) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx.ChainId = chainID
	return s
}

// Build 构建交易，返回 ComposedTx
//
// 🎯 **核心逻辑**：
// 1. 验证交易不为空
// 2. 设置创建时间戳
// 3. 返回 ComposedTx（进入 Type-state 状态机）
//
// 返回：
//   - *ComposedTx: 已组合的交易（包装类型，支持流式 API）
//   - error: 构建失败
//
// 💡 **使用示例**（流式调用）：
//
//	composed, _ := builder.Build()
//	proven, _ := composed.WithProofs(ctx, proofProvider)
//	signed, _ := proven.Sign(ctx, signer)
//	submitted, _ := signed.Submit(ctx, processor)
func (s *Service) Build() (*types.ComposedTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 验证交易不为空
	if len(s.tx.Inputs) == 0 && len(s.tx.Outputs) == 0 {
		return nil, fmt.Errorf("empty transaction: no inputs and outputs")
	}

	// 2. 设置创建时间戳
	if s.tx.CreationTimestamp == 0 {
		s.tx.CreationTimestamp = uint64(time.Now().Unix())
	}

	// 3. 为避免后续对 Builder 的修改影响已返回的交易，这里对底层交易做一次浅拷贝，
	//    并在拷贝结构中重新切片 Inputs/Outputs，保证调用方拿到的 Tx 与后续 Builder 状态隔离。
	clonedTx := *s.tx
	if len(s.tx.Inputs) > 0 {
		clonedInputs := make([]*transaction.TxInput, len(s.tx.Inputs))
		copy(clonedInputs, s.tx.Inputs)
		clonedTx.Inputs = clonedInputs
	}
	if len(s.tx.Outputs) > 0 {
		clonedOutputs := make([]*transaction.TxOutput, len(s.tx.Outputs))
		copy(clonedOutputs, s.tx.Outputs)
		clonedTx.Outputs = clonedOutputs
	}

	composedTx := &types.ComposedTx{
		Tx:     &clonedTx,
		Sealed: false, // 初始状态未封闭
	}

	// 4. 自动重置内部状态，避免后续构建复用旧的 Inputs/Outputs
	s.resetLocked()

	return composedTx, nil
}

// resetLocked 重置 Builder（调用方需持有 s.mu）
func (s *Service) resetLocked() {
	s.tx = &transaction.Transaction{
		Version: 1,
		Inputs:  make([]*transaction.TxInput, 0),
		Outputs: make([]*transaction.TxOutput, 0),
	}
}

// Reset 重置 Builder
//
// 🎯 **用途**：重置 Builder 状态，准备构建下一个交易
func (s *Service) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

// ════════════════════════════════════════════════════════════════════════════════════════════════
// 📝 草稿模式接口（P3 新增）
// ════════════════════════════════════════════════════════════════════════════════════════════════

// CreateDraft 创建交易草稿
//
// 🎯 **用途**：创建一个可变的交易草稿，支持渐进式构建
//
// 💡 **Draft 定位**：
// - Draft 是 Builder 的辅助工具（Compose/Plan 隐式）
// - **不是正式 Type-state 的一部分**
// - Draft.Seal() → ComposedTx（进入正式状态机）
//
// 🔄 **使用场景**：
//
// **场景 1：ISPC 渐进式构建**
//
//	draft, _ := builder.CreateDraft(ctx)
//	draftService.AddInput(ctx, draft, outpoint, false, nil)   // 第 1 次调用
//	// ... 合约执行 ...
//	draftService.AddAssetOutput(ctx, draft, recipient, "100", nil, []*pb.LockingCondition{lock})  // 第 2 次调用
//	// ... 合约执行 ...
//	composed, _ := draftService.SealDraft(ctx, draft)  // 封闭，进入 Type-state
//
// **场景 2：Off-chain 交互式构建**
//
//	draft, _ := builder.CreateDraft(ctx)
//	draftService.AddInput(ctx, draft, ...)
//	draftService.SaveDraft(ctx, draft)  // 保存草稿
//	draftID := draft.DraftID
//	// ... 用户确认 ...
//	draft, _ = builder.LoadDraft(ctx, draftID)  // 检索草稿
//	draftService.AddAssetOutput(ctx, draft, ...)  // 继续修改
//	composed, _ := draftService.SealDraft(ctx, draft)  // 封闭
//
// 返回：
//   - *types.DraftTx: 可变的交易草稿
//   - error: 创建失败
//
// ⚠️ 注意：
// - Draft 可以多次调用 Add* 方法（通过 DraftService）
// - Draft.Seal() 后不可再修改
// - Draft 存储由 DraftStore 端口负责
func (s *Service) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	if s.draftService == nil {
		return nil, fmt.Errorf("draftService 未初始化")
	}

	return s.draftService.CreateDraft(ctx)
}

// LoadDraft 加载已保存的交易草稿
//
// 🎯 **用途**：通过 draftID 检索之前保存的草稿
//
// 参数：
//   - ctx: 上下文
//   - draftID: 草稿唯一 ID
//
// 返回：
//   - *types.DraftTx: 加载的交易草稿
//   - error: 加载失败（如草稿不存在）
//
// ⚠️ 注意：
// - 加载的草稿可以继续修改（如果未封闭）
// - 草稿存储由 DraftStore 端口负责
func (s *Service) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if s.draftService == nil {
		return nil, fmt.Errorf("draftService 未初始化")
	}

	return s.draftService.LoadDraft(ctx, draftID)
}
