// Package hostabi 提供 Host ABI 实现
//
// tx_adapter_impl.go: TxAdapter 接口实现
package hostabi

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/tx/selector"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// txAdapterImpl TxAdapter 接口实现
//
// 🎯 **职责**:
//   - 封装 TX 模块能力，提供链上交易构建原语
//   - 确保确定性执行（固定区块视图、确定性 UTXO 选择）
//   - 管理链上 Draft 生命周期（绑定执行上下文）
//
// 💡 **依赖注入**：
//   - draftService: 交易草稿服务
//   - builder: 交易构建器
//   - verifier: 交易验证器
//   - selector: UTXO 选择器（确定性）
//   - draftManager: 链上 Draft 管理器
type txAdapterImpl struct {
	draftService tx.TransactionDraftService
	verifier     tx.TxVerifier
	selector     *selector.Service
	draftManager chainDraftManager
}

// NewTxAdapter 创建 TxAdapter 实例（导出函数，供 module.go 使用）
//
// 参数：
//   - draftService: 交易草稿服务
//   - verifier: 交易验证器
//   - selector: UTXO 选择器
//
// 返回：
//   - TxAdapter: TxAdapter 实例
func NewTxAdapter(
	draftService tx.TransactionDraftService,
	verifier tx.TxVerifier,
	selector *selector.Service,
) TxAdapter {
	return &txAdapterImpl{
		draftService: draftService,
		verifier:     verifier,
		selector:     selector,
		draftManager: newChainDraftManager(draftService),
	}
}

// BeginTransaction 开始构建交易
//
// 🔄 流程：
//  1. 创建链上 Draft（内存）
//  2. 绑定到当前执行上下文
//  3. 返回 draftHandle（用于后续调用）
//
// 参数：
//   - ctx: 执行上下文
//   - blockHeight: 当前区块高度（固定区块视图）
//   - blockTimestamp: 当前区块时间戳
//
// 返回：
//   - draftHandle: Draft 句柄（>0 成功，0 失败）
//   - error: 错误信息
func (a *txAdapterImpl) BeginTransaction(
	ctx context.Context,
	blockHeight uint64,
	blockTimestamp uint64,
) (int32, error) {
	// 调用 draftManager 创建 Draft
	handle, err := a.draftManager.CreateDraft(ctx, blockHeight, blockTimestamp)
	if err != nil {
		return 0, fmt.Errorf("开始交易失败: %w", err)
	}

	return handle, nil
}

// AddTransfer 添加转账意图
//
// 🔄 流程：
//  1. 根据 draftHandle 获取 Draft
//  2. 使用确定性 UTXO 选择器选择输入
//  3. 添加转账输出
//  4. 计算找零并添加找零输出
//
// 参数：
//   - ctx: 执行上下文
//   - draftHandle: Draft 句柄
//   - from: 发送方地址
//   - to: 接收方地址
//   - amount: 转账金额（字符串格式，如 "100"）
//   - tokenID: 代币标识（空表示原生币）
//
// 返回：
//   - outputIndex: 转账输出索引（成功返回 >= 0，失败返回 -1）
//   - error: 错误信息
func (a *txAdapterImpl) AddTransfer(
	ctx context.Context,
	draftHandle int32,
	from []byte,
	to []byte,
	amount string,
	tokenID []byte,
) (int32, error) {
	// 1. 获取 Draft
	draft, err := a.draftManager.GetDraft(ctx, draftHandle)
	if err != nil {
		return -1, fmt.Errorf("获取 Draft 失败: %w", err)
	}

	execCtx := GetExecutionContext(ctx)

	// 2. 构造 TokenID Key 及资产请求
	var (
		tokenIDKey   string
		contractAddr []byte
	)

	if len(tokenID) == 0 {
		tokenIDKey = "native"
	} else {
		if execCtx == nil {
			return -1, fmt.Errorf("执行上下文缺失，无法处理合约代币转账")
		}
		contractAddr = execCtx.GetContractAddress()
		if len(contractAddr) != 20 {
			return -1, fmt.Errorf("合约代币转账需要有效的20字节合约地址，实际: %d", len(contractAddr))
		}
		tokenIDKey = fmt.Sprintf("%x:%x", contractAddr, tokenID)
	}

	assetRequest := &selector.AssetRequest{
		TokenID: tokenIDKey,
		Amount:  amount,
	}
	if len(contractAddr) > 0 {
		assetRequest.ContractAddress = append([]byte(nil), contractAddr...)
		assetRequest.ClassID = append([]byte(nil), tokenID...)
	}

	// 3. 使用 Selector 选择 UTXO
	assetRequests := []*selector.AssetRequest{assetRequest}

	selectionResult, err := a.selector.SelectUTXOs(ctx, from, assetRequests)
	if err != nil {
		return -1, fmt.Errorf("UTXO 选择失败: %w", err)
	}

	// 4. 添加选中的 UTXO 作为输入
	for _, utxo := range selectionResult.SelectedUTXOs {
		_, err := a.draftService.AddInput(ctx, draft, utxo.Outpoint, false, nil)
		if err != nil {
			return -1, fmt.Errorf("添加输入失败: %w", err)
		}
	}

	// 5. 添加转账输出
	var toLockingCondition *transaction.LockingCondition
	if len(contractAddr) > 0 {
		toLockingCondition = buildContractLock(contractAddr)
	} else {
		toLockingCondition = buildSingleKeyLock(to)
	}

	outputIndex, err := a.draftService.AddAssetOutput(ctx, draft, to, amount, tokenID, []*transaction.LockingCondition{toLockingCondition})
	if err != nil {
		return -1, fmt.Errorf("添加转账输出失败: %w", err)
	}
	if len(contractAddr) > 0 {
		patchContractTokenOutput(draft, int(outputIndex), contractAddr)
	}

	// 6. 添加找零输出（如果有）
	if changeAmount, ok := selectionResult.ChangeAmounts[tokenIDKey]; ok {
		// 找零金额已经是字符串类型，直接使用
		changeStr := changeAmount

		// 找零锁定条件（回发送方，使用相同的单密钥锁）
		var changeLockingCondition *transaction.LockingCondition
		if len(contractAddr) > 0 {
			changeLockingCondition = buildContractLock(contractAddr)
		} else {
			changeLockingCondition = buildSingleKeyLock(from)
		}

		changeIndex, err := a.draftService.AddAssetOutput(ctx, draft, from, changeStr, tokenID, []*transaction.LockingCondition{changeLockingCondition})
		if err != nil {
			return -1, fmt.Errorf("添加找零输出失败: %w", err)
		}
		if len(contractAddr) > 0 {
			patchContractTokenOutput(draft, int(changeIndex), contractAddr)
		}
	}

	// 返回转账输出索引
	return int32(outputIndex), nil
}

func patchContractTokenOutput(draft *types.DraftTx, index int, contractAddr []byte) {
	if draft == nil || draft.Tx == nil || len(contractAddr) == 0 {
		return
	}
	if index < 0 || index >= len(draft.Tx.Outputs) {
		return
	}
	output := draft.Tx.Outputs[index]
	if output == nil {
		return
	}
	asset := output.GetAsset()
	if asset == nil {
		return
	}
	contractToken := asset.GetContractToken()
	if contractToken == nil {
		return
	}
	if len(contractToken.ContractAddress) == 0 {
		contractToken.ContractAddress = append([]byte(nil), contractAddr...)
	}
}

func buildSingleKeyLock(address []byte) *transaction.LockingCondition {
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: append([]byte(nil), address...),
				},
			},
		},
	}
}

func buildContractLock(contractAddr []byte) *transaction.LockingCondition {
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: append([]byte(nil), contractAddr...),
			},
		},
	}
}

// AddCustomInput 添加自定义输入（高级用法）
//
// 🎯 用途：合约显式指定输入 UTXO（绕过自动选择）
//
// 参数：
//   - ctx: 执行上下文
//   - draftHandle: Draft 句柄
//   - outpoint: UTXO 引用
//   - isReferenceOnly: 是否仅引用（true=不消费）
//
// 返回：
//   - inputIndex: 输入索引（从 0 开始）
//   - error: 错误信息
func (a *txAdapterImpl) AddCustomInput(
	ctx context.Context,
	draftHandle int32,
	outpoint *transaction.OutPoint,
	isReferenceOnly bool,
) (int32, error) {
	// 1. 获取 Draft
	draft, err := a.draftManager.GetDraft(ctx, draftHandle)
	if err != nil {
		return 0, fmt.Errorf("获取 Draft 失败: %w", err)
	}

	// 2. 调用 DraftService 添加输入
	inputIndex, err := a.draftService.AddInput(ctx, draft, outpoint, isReferenceOnly, nil)
	if err != nil {
		return 0, fmt.Errorf("添加输入失败: %w", err)
	}

	// 返回输入索引
	return int32(inputIndex), nil
}

// AddCustomOutput 添加自定义输出（高级用法）
//
// 🎯 用途：合约显式构建输出（支持复杂锁定条件）
//
// 参数：
//   - ctx: 执行上下文
//   - draftHandle: Draft 句柄
//   - output: 交易输出
//
// 返回：
//   - outputIndex: 输出索引（从 0 开始）
//   - error: 错误信息
func (a *txAdapterImpl) AddCustomOutput(
	ctx context.Context,
	draftHandle int32,
	output *transaction.TxOutput,
) (int32, error) {
	// 1. 获取 Draft
	draft, err := a.draftManager.GetDraft(ctx, draftHandle)
	if err != nil {
		return 0, fmt.Errorf("获取 Draft 失败: %w", err)
	}

	// 2. 添加输出到 Draft（直接添加到底层交易对象）
	draft.Tx.Outputs = append(draft.Tx.Outputs, output)

	// 返回输出索引
	return int32(len(draft.Tx.Outputs) - 1), nil
}

// GetDraft 获取Draft对象（高级用法）
//
// 🎯 用途：用于修改输出的锁定条件（delegated/threshold模式）
func (a *txAdapterImpl) GetDraft(
	ctx context.Context,
	draftHandle int32,
) (*types.DraftTx, error) {
	return a.draftManager.GetDraft(ctx, draftHandle)
}

// FinalizeTransaction 完成交易构建
//
// 🔄 流程：
//  1. Seal Draft → ComposedTx
//  2. 调用 Verifier 验证（AuthZ + Conservation + Condition）
//  3. 验证失败返回错误（触发合约回滚）
//  4. 验证通过返回未签名交易
//
// 参数：
//   - ctx: 执行上下文
//   - draftHandle: Draft 句柄
//
// 返回：
//   - tx: 未签名的交易（需外部签名）
//   - error: 错误信息
func (a *txAdapterImpl) FinalizeTransaction(
	ctx context.Context,
	draftHandle int32,
) (*transaction.Transaction, error) {
	// 1. 获取 Draft
	draft, err := a.draftManager.GetDraft(ctx, draftHandle)
	if err != nil {
		return nil, fmt.Errorf("获取 Draft 失败: %w", err)
	}

	// 2. 验证 Draft 非空
	if len(draft.Tx.Inputs) == 0 && len(draft.Tx.Outputs) == 0 {
		return nil, fmt.Errorf("交易为空：没有输入和输出")
	}

	// 3. Seal Draft（标记为不可修改）
	draft.IsSealed = true

	// 4. 返回未签名交易（验证在签名后进行）
	return draft.Tx, nil
}

// CleanupDraft 清理 Draft（可选，执行结束自动调用）
//
// 参数：
//   - ctx: 执行上下文
//   - draftHandle: Draft 句柄
//
// 返回：
//   - error: 错误信息
func (a *txAdapterImpl) CleanupDraft(
	ctx context.Context,
	draftHandle int32,
) error {
	return a.draftManager.RemoveDraft(ctx, draftHandle)
}
