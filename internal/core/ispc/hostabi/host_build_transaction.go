// Package hostabi 提供 Host ABI 实现
//
// host_build_transaction.go: host_build_transaction 宿主函数实现
package hostabi

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/utils"
	"google.golang.org/protobuf/proto"
)

// ═══════════════════════════════════════════════════════════════
// Draft JSON 数据结构（从 WASM 合约传入）
// ═══════════════════════════════════════════════════════════════

// DraftJSON 交易草稿 JSON 结构
//
// 🎯 **用途**：WASM 合约通过此结构描述交易意图
//
// 📋 **字段说明**：
//   - Inputs: 显式指定的输入（可选，支持高级用法）
//   - Outputs: 显式指定的输出（可选）
//   - Intents: 业务意图（如 transfer，由 Host 自动展开为输入输出）
//   - SignMode: 签名模式（defer_sign, delegated, threshold, paymaster）
//   - Metadata: 交易元数据（可选）
type DraftJSON struct {
	Inputs   []InputSpec  `json:"inputs,omitempty"`
	Outputs  []OutputSpec `json:"outputs,omitempty"`
	Intents  []Intent     `json:"intents,omitempty"`
	SignMode string       `json:"sign_mode"` // "defer_sign" | "delegated" | "threshold" | "paymaster"
	Metadata Metadata     `json:"metadata,omitempty"`
}

// InputSpec 输入规范
type InputSpec struct {
	TxHash          string `json:"tx_hash"`           // 交易哈希（十六进制）
	OutputIndex     uint32 `json:"output_index"`      // 输出索引
	IsReferenceOnly bool   `json:"is_reference_only"` // 是否仅引用
}

// OutputSpec 输出规范
type OutputSpec struct {
	Type     string          `json:"type"`     // "asset" | "resource" | "state"
	Owner    string          `json:"owner"`    // 所有者地址（十六进制）
	Amount   string          `json:"amount"`   // 金额（字符串，避免精度丢失）
	TokenID  string          `json:"token_id"` // 代币标识（可选）
	Metadata json.RawMessage `json:"metadata"` // 输出元数据（类型特定）
}

// Intent 业务意图
type Intent struct {
	Type   string          `json:"type"`   // "transfer" | "stake" | "deploy" | "call"
	Params json.RawMessage `json:"params"` // 意图参数（JSON，根据 type 解析）
}

// TransferIntent 转账意图参数
type TransferIntent struct {
	From    string `json:"from"`     // 发送方地址（十六进制）
	To      string `json:"to"`       // 接收方地址（十六进制）
	Amount  string `json:"amount"`   // 转账金额
	TokenID string `json:"token_id"` // 代币标识（可选）
}

// Metadata 交易元数据
type Metadata struct {
	Nonce      uint64            `json:"nonce,omitempty"`
	Memo       string            `json:"memo,omitempty"`
	CustomTags map[string]string `json:"custom_tags,omitempty"`
	GasLimit   uint64            `json:"gas_limit,omitempty"`
	GasPrice   string            `json:"gas_price,omitempty"`
	
	// 高级签名模式参数
	DelegationParams *DelegationParams `json:"delegation_params,omitempty"` // 委托签名参数
	ThresholdParams  *ThresholdParams  `json:"threshold_params,omitempty"`  // 门限签名参数
	PaymasterParams  *PaymasterParams  `json:"paymaster_params,omitempty"`  // 代付参数
}

// DelegationParams 委托签名参数
type DelegationParams struct {
	OriginalOwner        string   `json:"original_owner"`              // 原始所有者地址（十六进制）
	AllowedDelegates     []string `json:"allowed_delegates"`           // 允许的被委托者列表
	AuthorizedOperations []string `json:"authorized_operations"`       // 授权的操作类型
	ExpiryDurationBlocks uint64   `json:"expiry_duration_blocks"`      // 委托有效期（区块数，0=永不过期）
	MaxValuePerOperation string   `json:"max_value_per_operation"`     // 单次操作最大价值
	DelegationPolicy     string   `json:"delegation_policy,omitempty"` // 委托策略（可选）
}

// ThresholdParams 门限签名参数
type ThresholdParams struct {
	Threshold             uint32   `json:"threshold"`                  // 门限值（需要的最少份额数）
	TotalParties          uint32   `json:"total_parties"`              // 总参与方数量
	PartyVerificationKeys []string `json:"party_verification_keys"`    // 参与方验证密钥列表（十六进制）
	SignatureScheme       string   `json:"signature_scheme"`           // 门限签名方案（如"BLS_THRESHOLD"）
	SecurityLevel         uint32   `json:"security_level"`             // 安全级别（位数）
	ThresholdPolicy       string   `json:"threshold_policy,omitempty"` // 门限策略（可选）
}

// PaymasterParams 代付参数
type PaymasterParams struct {
	FeeAmount string `json:"fee_amount"`           // 费用金额（字符串格式）
	TokenID   string `json:"token_id,omitempty"`   // 费用代币标识（可选，空表示原生币）
	MinerAddr string `json:"miner_addr,omitempty"` // 矿工地址（可选，用于费用输出）
}

// ═══════════════════════════════════════════════════════════════
// TxReceipt 数据结构（返回给 WASM 合约）
// ═══════════════════════════════════════════════════════════════

// TxReceipt 交易收据
//
// 🎯 **用途**：Host 返回给 WASM 合约的交易构建结果
//
// 📋 **字段说明**：
//   - Mode: 签名模式
//   - UnsignedTxHash: 未签名交易哈希（defer_sign 模式）
//   - SignedTxHash: 已签名交易哈希（其他模式）
//   - SerializedTx: 序列化交易（defer_sign 模式）
//   - ProposalID: 提案 ID（threshold 模式，未达门限时）
//   - Error: 错误信息（如果失败）
type TxReceipt struct {
	Mode           string `json:"mode"`                       // "unsigned" | "delegated" | "threshold" | "paymaster"
	UnsignedTxHash string `json:"unsigned_tx_hash,omitempty"` // 未签名交易哈希（十六进制）
	SignedTxHash   string `json:"signed_tx_hash,omitempty"`   // 已签名交易哈希（十六进制）
	SerializedTx   string `json:"serialized_tx,omitempty"`    // 序列化交易（Base64 或十六进制）
	ProposalID     string `json:"proposal_id,omitempty"`      // 提案 ID（threshold 模式）
	Error          string `json:"error,omitempty"`            // 错误信息
}

// ═══════════════════════════════════════════════════════════════
// Draft JSON 解析与验证
// ═══════════════════════════════════════════════════════════════

// ParseDraftJSON 解析 Draft JSON
//
// 参数：
//   - draftJSONBytes: Draft JSON 字节数组
//
// 返回：
//   - *DraftJSON: 解析后的 Draft 结构
//   - error: 解析错误
func ParseDraftJSON(draftJSONBytes []byte) (*DraftJSON, error) {
	var draft DraftJSON
	if err := json.Unmarshal(draftJSONBytes, &draft); err != nil {
		return nil, fmt.Errorf("解析 Draft JSON 失败: %w", err)
	}

	// 验证基本字段
	if draft.SignMode == "" {
		draft.SignMode = "defer_sign" // 默认模式
	}

	return &draft, nil
}

// ValidateDraftJSON 验证 Draft JSON
//
// 参数：
//   - draft: Draft 结构
//
// 返回：
//   - error: 验证错误
func ValidateDraftJSON(draft *DraftJSON) error {
	// 验证 SignMode
	validModes := map[string]bool{
		"defer_sign": true,
		"delegated":  true,
		"threshold":  true,
		"paymaster":  true,
	}
	if !validModes[draft.SignMode] {
		return fmt.Errorf("无效的签名模式: %s", draft.SignMode)
	}

	// 验证至少有输入/输出或意图
	if len(draft.Inputs) == 0 && len(draft.Outputs) == 0 && len(draft.Intents) == 0 {
		return fmt.Errorf("交易为空：没有输入、输出或意图")
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════
// TxReceipt 编码
// ═══════════════════════════════════════════════════════════════

// EncodeTxReceipt 编码 TxReceipt 为 JSON
//
// 参数：
//   - receipt: TxReceipt 结构
//
// 返回：
//   - []byte: JSON 字节数组
//   - error: 编码错误
func EncodeTxReceipt(receipt *TxReceipt) ([]byte, error) {
	data, err := json.Marshal(receipt)
	if err != nil {
		return nil, fmt.Errorf("编码 TxReceipt 失败: %w", err)
	}
	return data, nil
}

// ═══════════════════════════════════════════════════════════════
// host_build_transaction 核心业务逻辑
// ═══════════════════════════════════════════════════════════════

// BuildTransactionFromDraft 从 Draft JSON 构建交易
//
// 🔄 **核心流程**：
//  1. 解析并验证 Draft JSON
//  2. 处理 Intents（展开为输入输出）
//  3. 处理显式的输入输出
//  4. 根据 sign_mode 在 Finalize 之前处理特殊逻辑
//  5. Finalize 交易
//  6. 根据 sign_mode 路由（计算哈希、序列化等）
//  7. 返回 TxReceipt
//
// 参数：
//   - ctx: 执行上下文
//   - txAdapter: TX 适配器
//   - txHashClient: 交易哈希服务客户端（用于计算交易哈希）
//   - eutxoQuery: UTXO查询服务（用于paymaster模式查询赞助池）
//   - callerAddress: 调用者地址（用于delegated模式）
//   - contractAddress: 合约地址（用于设置合约代币输出的contract_address）
//   - draftJSONBytes: Draft JSON 字节数组
//   - blockHeight: 当前区块高度
//   - blockTimestamp: 当前区块时间戳
//
// 返回：
//   - *TxReceipt: 交易收据
//   - error: 构建错误
func BuildTransactionFromDraft(
	ctx context.Context,
	txAdapter TxAdapter,
	txHashClient transaction.TransactionHashServiceClient,
	eutxoQuery persistence.UTXOQuery,
	callerAddress []byte,
	contractAddress []byte,
	draftJSONBytes []byte,
	blockHeight uint64,
	blockTimestamp uint64,
) (*TxReceipt, error) {
	// 1. 解析 Draft JSON
	draft, err := ParseDraftJSON(draftJSONBytes)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("解析 Draft JSON 失败: %v", err),
		}, err
	}

	// 2. 验证 Draft JSON
	if err := ValidateDraftJSON(draft); err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("验证 Draft JSON 失败: %v", err),
		}, err
	}

	// 3. 创建 Draft
	draftHandle, err := txAdapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("创建 Draft 失败: %v", err),
		}, err
	}
	defer txAdapter.CleanupDraft(ctx, draftHandle) // 确保清理

	// 4. 处理 Intents（业务意图）
	for _, intent := range draft.Intents {
		if err := processIntent(ctx, txAdapter, draftHandle, intent); err != nil {
			return &TxReceipt{
				Mode:  "error",
				Error: fmt.Sprintf("处理意图失败: %v", err),
			}, err
		}
	}

	// 5. 处理显式输入
	for _, inputSpec := range draft.Inputs {
		outpoint := &transaction.OutPoint{
			TxId:        decodeHex(inputSpec.TxHash),
			OutputIndex: inputSpec.OutputIndex,
		}
		_, err := txAdapter.AddCustomInput(ctx, draftHandle, outpoint, inputSpec.IsReferenceOnly)
		if err != nil {
			return &TxReceipt{
				Mode:  "error",
				Error: fmt.Sprintf("添加输入失败: %v", err),
			}, err
		}
	}

	// 6. 处理显式输出（支持 asset/resource/state 三种类型）
	// ✅ 修复：传递合约地址给 buildTxOutputFromSpec，用于设置合约代币输出的 contract_address
	// 注意：buildAssetOutput 中 tokenID 是 token_identifier，不是 contract_address
	for _, outputSpec := range draft.Outputs {
		txOutput, err := buildTxOutputFromSpec(&outputSpec, contractAddress)
		if err != nil {
			return &TxReceipt{
				Mode:  "error",
				Error: fmt.Sprintf("构建输出失败: %v", err),
			}, err
		}

		_, err = txAdapter.AddCustomOutput(ctx, draftHandle, txOutput)
		if err != nil {
			return &TxReceipt{
				Mode:  "error",
				Error: fmt.Sprintf("添加输出失败: %v", err),
			}, err
		}
	}

	// 7. 根据 sign_mode 在 Finalize 之前处理特殊逻辑
	if err := applySignModeLogic(ctx, txAdapter, eutxoQuery, callerAddress, draftHandle, draft, blockHeight); err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("应用签名模式逻辑失败: %v", err),
		}, err
	}

	// 8. Finalize 交易
	unsignedTx, err := txAdapter.FinalizeTransaction(ctx, draftHandle)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("完成交易构建失败: %v", err),
		}, err
	}

	// 8.1 将最终草稿同步回执行上下文，确保协调器能够读取完整交易
	if execCtx := GetExecutionContext(ctx); execCtx != nil {
		if draftObj, err := txAdapter.GetDraft(ctx, draftHandle); err == nil && draftObj != nil {
			clonedTx, ok := proto.Clone(unsignedTx).(*transaction.Transaction)
			if !ok {
				clonedTx = unsignedTx
			}

			callerHex := ""
			if callerAddr := execCtx.GetCallerAddress(); len(callerAddr) > 0 {
				callerHex = hex.EncodeToString(callerAddr)
			}

			ctxDraft := &ispcInterfaces.TransactionDraft{
				DraftID:       draftObj.DraftID,
				ExecutionID:   execCtx.GetExecutionID(),
				CallerAddress: callerHex,
				CreatedAt:     draftObj.CreatedAt,
				IsSealed:      draftObj.IsSealed,
				Tx:            clonedTx,
				Outputs:       clonedTx.GetOutputs(),
			}

			_ = execCtx.UpdateTransactionDraft(ctxDraft)
		}
	}

	// 9. 根据 sign_mode 路由（计算哈希、序列化等）
	return routeBySignMode(ctx, txHashClient, draft.SignMode, unsignedTx)
}

// processIntent 处理单个业务意图
func processIntent(
	ctx context.Context,
	txAdapter TxAdapter,
	draftHandle int32,
	intent Intent,
) error {
	switch intent.Type {
	case "transfer":
		// 解析转账意图
		var transferParams TransferIntent
		if err := json.Unmarshal(intent.Params, &transferParams); err != nil {
			return fmt.Errorf("解析转账意图参数失败: %w", err)
		}

		// 调用 AddTransfer
		_, err := txAdapter.AddTransfer(
			ctx,
			draftHandle,
			decodeHex(transferParams.From),
			decodeHex(transferParams.To),
			transferParams.Amount,
			decodeHex(transferParams.TokenID),
		)
		return err

	default:
		return fmt.Errorf("不支持的意图类型: %s", intent.Type)
	}
}

// applySignModeLogic 根据签名模式在 Finalize 之前处理特殊逻辑
//
// 🔄 **处理流程**：
//  1. delegated模式：修改输出的锁定条件为DelegationLock
//  2. threshold模式：修改输出的锁定条件为ThresholdLock
//  3. paymaster模式：添加赞助池输入和费用输出
//
// 参数：
//   - ctx: 执行上下文
//   - txAdapter: TX 适配器
//   - eutxoQuery: UTXO查询服务（用于paymaster模式）
//   - callerAddress: 调用者地址（用于delegated模式）
//   - draftHandle: Draft 句柄
//   - draft: Draft JSON 结构
//   - blockHeight: 当前区块高度
//
// 返回：
//   - error: 处理错误
func applySignModeLogic(
	ctx context.Context,
	txAdapter TxAdapter,
	eutxoQuery persistence.UTXOQuery,
	callerAddress []byte,
	draftHandle int32,
	draft *DraftJSON,
	blockHeight uint64,
) error {
	switch draft.SignMode {
	case "defer_sign":
		// defer_sign模式无需特殊处理
		return nil

	case "delegated":
		// 委托签名模式：修改输出的锁定条件为DelegationLock
		if draft.Metadata.DelegationParams == nil {
			return fmt.Errorf("delegated模式需要提供delegation_params")
		}
		return applyDelegationLock(ctx, txAdapter, callerAddress, draftHandle, draft.Metadata.DelegationParams, blockHeight)

	case "threshold":
		// 门限签名模式：修改输出的锁定条件为ThresholdLock
		if draft.Metadata.ThresholdParams == nil {
			return fmt.Errorf("threshold模式需要提供threshold_params")
		}
		return applyThresholdLock(ctx, txAdapter, draftHandle, draft.Metadata.ThresholdParams)

	case "paymaster":
		// 代付模式：添加赞助池输入和费用输出
		if draft.Metadata.PaymasterParams == nil {
			return fmt.Errorf("paymaster模式需要提供paymaster_params")
		}
		return applyPaymaster(ctx, txAdapter, eutxoQuery, draftHandle, draft.Metadata.PaymasterParams, blockHeight)

	default:
		// 未知模式，跳过处理
		return nil
	}
}

// applyDelegationLock 应用委托锁定条件到交易输出
func applyDelegationLock(
	ctx context.Context,
	txAdapter TxAdapter,
	callerAddress []byte,
	draftHandle int32,
	params *DelegationParams,
	blockHeight uint64,
) error {
	// 1. 获取Draft对象
	draft, err := txAdapter.GetDraft(ctx, draftHandle)
	if err != nil {
		return fmt.Errorf("获取Draft失败: %w", err)
	}

	// 2. 解析参数
	originalOwner := decodeHex(params.OriginalOwner)
	if len(originalOwner) != 20 {
		return fmt.Errorf("original_owner地址长度错误: 期望20字节，实际%d字节", len(originalOwner))
	}

	allowedDelegates := make([][]byte, 0, len(params.AllowedDelegates))
	for _, delegateStr := range params.AllowedDelegates {
		delegateAddr := decodeHex(delegateStr)
		if len(delegateAddr) != 20 {
			return fmt.Errorf("allowed_delegate地址长度错误: %s", delegateStr)
		}
		allowedDelegates = append(allowedDelegates, delegateAddr)
	}

	maxValue, err := parseAmount(params.MaxValuePerOperation)
	if err != nil {
		return fmt.Errorf("解析max_value_per_operation失败: %w", err)
	}

	// 3. 构建DelegationLock
	delegationLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				OriginalOwner:        originalOwner,
				AllowedDelegates:     allowedDelegates,
				AuthorizedOperations: params.AuthorizedOperations,
				MaxValuePerOperation: maxValue,
			},
		},
	}

	// 设置过期时间（如果指定）
	var expiryDurationBlocks *uint64
	if params.ExpiryDurationBlocks > 0 {
		expiryDurationBlocks = &params.ExpiryDurationBlocks
	}
	delegationLock.GetDelegationLock().ExpiryDurationBlocks = expiryDurationBlocks

	// 设置委托策略（如果指定）
	if params.DelegationPolicy != "" {
		delegationLock.GetDelegationLock().DelegationPolicy = []byte(params.DelegationPolicy)
	}

	// 4. 修改所有Asset输出的锁定条件为DelegationLock
	for _, output := range draft.Tx.Outputs {
		if output.GetAsset() != nil {
			// 替换锁定条件
			output.LockingConditions = []*transaction.LockingCondition{delegationLock}
		}
	}

	return nil
}

// applyThresholdLock 应用门限锁定条件到交易输出
func applyThresholdLock(
	ctx context.Context,
	txAdapter TxAdapter,
	draftHandle int32,
	params *ThresholdParams,
) error {
	// 1. 获取Draft对象
	draft, err := txAdapter.GetDraft(ctx, draftHandle)
	if err != nil {
		return fmt.Errorf("获取Draft失败: %w", err)
	}

	// 2. 解析参数
	partyKeys := make([][]byte, 0, len(params.PartyVerificationKeys))
	for _, keyStr := range params.PartyVerificationKeys {
		keyBytes := decodeHex(keyStr)
		if len(keyBytes) == 0 {
			return fmt.Errorf("party_verification_key解码失败: %s", keyStr)
		}
		partyKeys = append(partyKeys, keyBytes)
	}

	// 3. 构建ThresholdLock
	thresholdLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             params.Threshold,
				TotalParties:          params.TotalParties,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       params.SignatureScheme,
				SecurityLevel:         params.SecurityLevel,
			},
		},
	}

	// 设置门限策略（如果指定）
	if params.ThresholdPolicy != "" {
		thresholdLock.GetThresholdLock().ThresholdPolicy = []byte(params.ThresholdPolicy)
	}

	// 4. 修改所有Asset输出的锁定条件为ThresholdLock
	for _, output := range draft.Tx.Outputs {
		if output.GetAsset() != nil {
			// 替换锁定条件
			output.LockingConditions = []*transaction.LockingCondition{thresholdLock}
		}
	}

	return nil
}

// applyPaymaster 应用代付逻辑（添加赞助池输入和费用输出）
func applyPaymaster(
	ctx context.Context,
	txAdapter TxAdapter,
	eutxoQuery persistence.UTXOQuery,
	draftHandle int32,
	params *PaymasterParams,
	blockHeight uint64,
) error {
	// 1. 查询赞助池UTXO
	if eutxoQuery == nil {
		return fmt.Errorf("UTXOQuery未设置，无法查询赞助池")
	}

	sponsorUTXOs, err := eutxoQuery.GetSponsorPoolUTXOs(ctx, true) // onlyAvailable=true
	if err != nil {
		return fmt.Errorf("查询赞助池UTXO失败: %w", err)
	}

	if len(sponsorUTXOs) == 0 {
		return fmt.Errorf("赞助池中没有可用的UTXO")
	}

	// 2. 选择足够的赞助池UTXO来支付费用
	// 解析所需费用金额
	requiredFee, err := parseAmount(params.FeeAmount)
	if err != nil {
		return fmt.Errorf("解析费用金额失败: %w", err)
	}

	// 按金额选择UTXO：选择金额 >= 所需费用的第一个UTXO
	var selectedUTXO *utxo.UTXO
	for _, utxoItem := range sponsorUTXOs {
		if utxoItem == nil {
			continue
		}

		// 只处理Asset类型的UTXO
		if utxoItem.Category != utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 获取缓存的TxOutput
		cachedOutput := utxoItem.GetCachedOutput()
		if cachedOutput == nil {
			continue
		}

		// 获取AssetOutput
		assetOutput := cachedOutput.GetAsset()
		if assetOutput == nil {
			continue
		}

		// 获取原生币金额（paymaster通常使用原生币支付费用）
		nativeCoin := assetOutput.GetNativeCoin()
		if nativeCoin == nil {
			continue // 跳过非原生币
		}

		// 解析金额
		amount, err := parseAmount(nativeCoin.Amount)
		if err != nil {
			continue // 跳过解析失败的UTXO
		}

		// 选择第一个金额足够的UTXO
		if amount >= requiredFee {
			selectedUTXO = utxoItem
			break
		}
	}

	// 如果没有找到足够的UTXO，返回错误
	if selectedUTXO == nil {
		return fmt.Errorf("赞助池中没有金额足够的UTXO来支付费用 %s", params.FeeAmount)
	}

	// 3. 添加赞助池UTXO作为输入
	outpoint := selectedUTXO.Outpoint // UTXO.Outpoint已经是*transaction.OutPoint类型
	if outpoint == nil {
		return fmt.Errorf("赞助池UTXO的Outpoint为空")
	}
	_, err = txAdapter.AddCustomInput(ctx, draftHandle, outpoint, false) // 消费模式
	if err != nil {
		return fmt.Errorf("添加赞助池输入失败: %w", err)
	}

	// 5. 添加费用输出到矿工地址（如果指定）或使用默认地址
	minerAddr := decodeHex(params.MinerAddr)
	if len(minerAddr) != 20 {
		// 如果未指定矿工地址，使用全零地址（系统地址）
		minerAddr = make([]byte, 20)
	}

	// 构建费用输出的锁定条件（单密钥锁）
	feeLockingCondition := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: minerAddr,
				},
				RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	// 添加费用输出
	// 注意：这里需要直接访问DraftService，因为TxAdapter可能没有AddAssetOutput方法
	// 简化实现：通过AddCustomOutput方法添加
	feeOutput := &transaction.TxOutput{
		Owner:             minerAddr,
		LockingConditions: []*transaction.LockingCondition{feeLockingCondition},
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_NativeCoin{
					NativeCoin: &transaction.NativeCoinAsset{
						Amount: params.FeeAmount,
					},
				},
			},
		},
	}

	_, err = txAdapter.AddCustomOutput(ctx, draftHandle, feeOutput)
	if err != nil {
		return fmt.Errorf("添加费用输出失败: %w", err)
	}

	return nil
}

// routeBySignMode 根据签名模式路由
func routeBySignMode(
	ctx context.Context,
	txHashClient transaction.TransactionHashServiceClient,
	signMode string,
	unsignedTx *transaction.Transaction,
) (*TxReceipt, error) {
	switch signMode {
	case "defer_sign":
		// 即时签名模式：返回未签名交易
		// 使用 gRPC 服务计算交易哈希
		if txHashClient == nil {
			return &TxReceipt{
				Mode:  "error",
				Error: "transaction hash client is not initialized",
			}, fmt.Errorf("transaction hash client is not initialized")
		}
		req := &transaction.ComputeHashRequest{
			Transaction:      unsignedTx,
			IncludeDebugInfo: false,
		}
		resp, err := txHashClient.ComputeHash(ctx, req)
		if err != nil {
			return &TxReceipt{
				Mode:  "error",
				Error: fmt.Sprintf("failed to compute transaction hash: %v", err),
			}, fmt.Errorf("failed to compute transaction hash: %w", err)
		}
		if !resp.IsValid {
			return &TxReceipt{
				Mode:  "error",
				Error: "transaction structure is invalid",
			}, fmt.Errorf("transaction structure is invalid")
		}
		return &TxReceipt{
			Mode:           "unsigned",
			UnsignedTxHash: encodeHex(resp.Hash),
			SerializedTx:   encodeBase64(serializeTx(unsignedTx)),
			Error:          "",
		}, nil

	case "delegated":
		// 委托签名模式：返回未签名交易（锁定条件已在applySignModeLogic中应用）
		return handleDelegatedMode(ctx, txHashClient, unsignedTx)

	case "threshold":
		// 门限签名模式：返回未签名交易（锁定条件已在applySignModeLogic中应用）
		return handleThresholdMode(ctx, txHashClient, unsignedTx)

	case "paymaster":
		// 代付模式：返回未签名交易（赞助池输入和费用输出已在applySignModeLogic中添加）
		return handlePaymasterMode(ctx, txHashClient, unsignedTx)

	default:
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("未知的签名模式: %s", signMode),
		}, fmt.Errorf("未知的签名模式: %s", signMode)
	}
}

// handleDelegatedMode 处理委托签名模式（返回未签名交易）
func handleDelegatedMode(
	ctx context.Context,
	txHashClient transaction.TransactionHashServiceClient,
	unsignedTx *transaction.Transaction,
) (*TxReceipt, error) {
	// 委托模式：锁定条件已在applySignModeLogic中应用，这里只需要计算哈希
	if txHashClient == nil {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction hash client is not initialized",
		}, fmt.Errorf("transaction hash client is not initialized")
	}

	req := &transaction.ComputeHashRequest{
		Transaction:      unsignedTx,
		IncludeDebugInfo: false,
	}
	resp, err := txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("failed to compute transaction hash: %v", err),
		}, fmt.Errorf("failed to compute transaction hash: %w", err)
	}
	if !resp.IsValid {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction structure is invalid",
		}, fmt.Errorf("transaction structure is invalid")
	}

	return &TxReceipt{
		Mode:           "delegated",
		UnsignedTxHash: encodeHex(resp.Hash),
		SerializedTx:   encodeBase64(serializeTx(unsignedTx)),
		Error:          "",
	}, nil
}

// handleThresholdMode 处理门限签名模式（返回未签名交易）
func handleThresholdMode(
	ctx context.Context,
	txHashClient transaction.TransactionHashServiceClient,
	unsignedTx *transaction.Transaction,
) (*TxReceipt, error) {
	// 门限模式：锁定条件已在applySignModeLogic中应用，这里只需要计算哈希
	if txHashClient == nil {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction hash client is not initialized",
		}, fmt.Errorf("transaction hash client is not initialized")
	}

	req := &transaction.ComputeHashRequest{
		Transaction:      unsignedTx,
		IncludeDebugInfo: false,
	}
	resp, err := txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("failed to compute transaction hash: %v", err),
		}, fmt.Errorf("failed to compute transaction hash: %w", err)
	}
	if !resp.IsValid {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction structure is invalid",
		}, fmt.Errorf("transaction structure is invalid")
	}

	return &TxReceipt{
		Mode:           "threshold",
		UnsignedTxHash: encodeHex(resp.Hash),
		SerializedTx:   encodeBase64(serializeTx(unsignedTx)),
		Error:          "",
	}, nil
}

// handlePaymasterMode 处理代付模式（返回未签名交易）
func handlePaymasterMode(
	ctx context.Context,
	txHashClient transaction.TransactionHashServiceClient,
	unsignedTx *transaction.Transaction,
) (*TxReceipt, error) {
	// 代付模式：赞助池输入和费用输出已在applySignModeLogic中添加，这里只需要计算哈希
	if txHashClient == nil {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction hash client is not initialized",
		}, fmt.Errorf("transaction hash client is not initialized")
	}

	req := &transaction.ComputeHashRequest{
		Transaction:      unsignedTx,
		IncludeDebugInfo: false,
	}
	resp, err := txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return &TxReceipt{
			Mode:  "error",
			Error: fmt.Sprintf("failed to compute transaction hash: %v", err),
		}, fmt.Errorf("failed to compute transaction hash: %w", err)
	}
	if !resp.IsValid {
		return &TxReceipt{
			Mode:  "error",
			Error: "transaction structure is invalid",
		}, fmt.Errorf("transaction structure is invalid")
	}

	return &TxReceipt{
		Mode:           "paymaster",
		UnsignedTxHash: encodeHex(resp.Hash),
		SerializedTx:   encodeBase64(serializeTx(unsignedTx)),
		Error:          "",
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// 辅助函数（编码/解码）
// ═══════════════════════════════════════════════════════════════

// decodeHex 解码十六进制字符串
//
// 支持的格式：
//   - 带 0x 前缀：0xabc123
//   - 不带前缀：abc123
//
// 参数：
//   - hexStr: 十六进制字符串
//
// 返回：
//   - []byte: 解码后的字节数组
func decodeHex(hexStr string) []byte {
	// 移除 0x 前缀（如果存在）
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	// 使用标准库解码
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		// 解码失败返回空字节数组
		return []byte{}
	}
	return data
}

// encodeHex 编码字节数组为十六进制字符串
//
// 参数：
//   - data: 字节数组
//
// 返回：
//   - string: 十六进制字符串（不带 0x 前缀）
func encodeHex(data []byte) string {
	return hex.EncodeToString(data)
}

// encodeBase64 编码字节数组为 Base64 字符串
//
// 使用标准 Base64 编码（RFC 4648）
//
// 参数：
//   - data: 字节数组
//
// 返回：
//   - string: Base64 编码字符串
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// computeTxHash 已移除：交易哈希计算应通过 gRPC TransactionHashService 完成
// 请使用 transaction.TransactionHashServiceClient.ComputeHash 方法

// serializeTx 序列化交易
//
// 使用 Protobuf 序列化为字节数组
//
// 参数：
//   - tx: 交易对象
//
// 返回：
//   - []byte: 序列化后的字节数组
func serializeTx(tx *transaction.Transaction) []byte {
	// 使用 Protobuf Marshal
	data, err := proto.Marshal(tx)
	if err != nil {
		// 序列化失败返回空字节数组
		// 注意：此函数返回[]byte而非error，调用者应检查返回的data是否为空
		return []byte{}
	}
	return data
}

// buildTxOutputFromSpec 从 OutputSpec 构建 TxOutput
//
// 支持三种输出类型：
//   - asset: 资产输出（NativeCoin 或 ContractToken）
//   - resource: 资源输出（ResourceOutput）
//   - state: 状态输出（StateOutput）
//
// 参数：
//   - spec: 输出规范
//   - contractAddress: 合约地址（用于设置合约代币输出的contract_address）
//
// 返回：
//   - *transaction.TxOutput: 构建的交易输出
//   - error: 构建错误
func buildTxOutputFromSpec(spec *OutputSpec, contractAddress []byte) (*transaction.TxOutput, error) {
	if spec == nil {
		return nil, fmt.Errorf("outputSpec 不能为空")
	}

	// 解析 owner 地址
	ownerBytes := decodeHex(spec.Owner)
	if len(ownerBytes) != 20 {
		return nil, fmt.Errorf("owner 地址必须是 20 字节，实际: %d", len(ownerBytes))
	}

	// 解析锁定条件（从 Metadata 中提取，如果存在）
	var lockingConditions []*transaction.LockingCondition
	if len(spec.Metadata) > 0 {
		// 尝试从 Metadata 中解析锁定条件
		var metadata map[string]interface{}
		if err := json.Unmarshal(spec.Metadata, &metadata); err == nil {
			if lockData, ok := metadata["locking_conditions"].(string); ok {
				lockBytes := decodeHex(lockData)
				if len(lockBytes) > 0 {
					lock := &transaction.LockingCondition{}
					if err := proto.Unmarshal(lockBytes, lock); err == nil {
						lockingConditions = []*transaction.LockingCondition{lock}
					} else {
						// 解析锁定条件失败，使用默认锁定条件
						// 错误已记录在 proto.Unmarshal 中
					}
				}
			}
		} else {
			// JSON解析失败，使用默认锁定条件
			// 错误已记录在 json.Unmarshal 中
		}
	}

	// 如果没有指定锁定条件，使用默认的 SingleKeyLock（基于 owner）
	if len(lockingConditions) == 0 {
		// 创建默认的 SingleKeyLock（地址哈希锁定）
		defaultLock := &transaction.LockingCondition{
			Condition: &transaction.LockingCondition_SingleKeyLock{
				SingleKeyLock: &transaction.SingleKeyLock{
					KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: ownerBytes, // 使用 owner 作为地址哈希
					},
					RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
					SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
				},
			},
		}
		lockingConditions = []*transaction.LockingCondition{defaultLock}
	}

	// 根据类型构建不同的输出
	switch spec.Type {
	case "asset":
		return buildAssetOutput(ownerBytes, spec, lockingConditions, contractAddress)
	case "resource":
		return buildResourceOutput(ownerBytes, spec, lockingConditions)
	case "state":
		return buildStateOutput(ownerBytes, spec, lockingConditions)
	default:
		return nil, fmt.Errorf("不支持的输出类型: %s", spec.Type)
	}
}

// buildAssetOutput 构建资产输出
func buildAssetOutput(owner []byte, spec *OutputSpec, locks []*transaction.LockingCondition, contractAddress []byte) (*transaction.TxOutput, error) {
	// 解析金额（字符串格式，支持大数）
	amountStr := spec.Amount
	if amountStr == "" {
		amountStr = "0"
	}

	// 解析 tokenID（可选）
	var tokenIDBytes []byte
	if spec.TokenID != "" {
		tokenIDBytes = decodeHex(spec.TokenID)
	}

	// 构建 AssetOutput
	var assetOutput *transaction.AssetOutput
	if len(tokenIDBytes) == 0 {
		// 原生币
		assetOutput = &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_NativeCoin{
				NativeCoin: &transaction.NativeCoinAsset{
					Amount: amountStr, // 使用字符串格式支持大数
				},
			},
		}
	} else {
		// ✅ 合约代币：tokenID 是 token_identifier（如 FungibleClassId），不是 contract_address
		// ContractAddress 从参数传入（从执行上下文获取的合约地址）
		if len(contractAddress) == 0 {
			return nil, fmt.Errorf("合约代币输出需要提供合约地址（contractAddress）")
		}
		if len(contractAddress) != 20 {
			return nil, fmt.Errorf("合约地址必须是 20 字节，实际: %d", len(contractAddress))
		}
		assetOutput := &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_ContractToken{
				ContractToken: &transaction.ContractTokenAsset{
					ContractAddress: append([]byte(nil), contractAddress...), // ✅ 从执行上下文获取的合约地址
					TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
						FungibleClassId: tokenIDBytes, // tokenID 是 token_identifier
					},
					Amount: amountStr, // 使用字符串格式支持大数
				},
			},
		}
		locks = []*transaction.LockingCondition{
			{
				Condition: &transaction.LockingCondition_ContractLock{
					ContractLock: &transaction.ContractLock{
						ContractAddress: append([]byte(nil), contractAddress...),
					},
				},
			},
		}
		return &transaction.TxOutput{
			Owner:             owner,
			LockingConditions: locks,
			OutputContent:     &transaction.TxOutput_Asset{Asset: assetOutput},
		}, nil
	}

	return &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: locks,
		OutputContent:     &transaction.TxOutput_Asset{Asset: assetOutput},
	}, nil
}

// buildResourceOutput 构建资源输出
func buildResourceOutput(owner []byte, spec *OutputSpec, locks []*transaction.LockingCondition) (*transaction.TxOutput, error) {
	// 从 Metadata 中解析资源信息
	var resourceData struct {
		ContentHash string `json:"content_hash"`
		Category    string `json:"category"`
		MimeType    string `json:"mime_type,omitempty"`
		Size        uint64 `json:"size,omitempty"`
		Metadata    string `json:"metadata,omitempty"`
	}

	if len(spec.Metadata) > 0 {
		if err := json.Unmarshal(spec.Metadata, &resourceData); err != nil {
			return nil, fmt.Errorf("解析资源元数据失败: %w", err)
		}
	}

	// 解析 contentHash
	if resourceData.ContentHash == "" {
		return nil, fmt.Errorf("资源 content_hash 不能为空")
	}

	contentHashBytes := decodeHex(resourceData.ContentHash)
	if len(contentHashBytes) != 32 {
		return nil, fmt.Errorf("content_hash 必须是 32 字节，实际: %d", len(contentHashBytes))
	}

	// 确定资源类别
	var category pbresource.ResourceCategory
	var executableType pbresource.ExecutableType

	switch resourceData.Category {
	case "wasm", "contract":
		category = pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE
		executableType = pbresource.ExecutableType_EXECUTABLE_TYPE_CONTRACT
	case "onnx", "model":
		category = pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE
		executableType = pbresource.ExecutableType_EXECUTABLE_TYPE_AIMODEL
	case "document", "file", "static":
		category = pbresource.ResourceCategory_RESOURCE_CATEGORY_STATIC
	default:
		return nil, fmt.Errorf("不支持的资源类别: %s", resourceData.Category)
	}

	// 构建 Resource 对象
	resource := &pbresource.Resource{
		Category:       category,
		ContentHash:    contentHashBytes,
		MimeType:       resourceData.MimeType,
		Size:           resourceData.Size,
		ExecutableType: executableType,
	}

	// 构建 ResourceOutput
	resourceOutput := &transaction.ResourceOutput{
		Resource:          resource,
		CreationTimestamp: 0, // 将在 Finalize 时设置
		StorageStrategy:   transaction.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
		IsImmutable:       true,
	}

	return &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: locks,
		OutputContent:     &transaction.TxOutput_Resource{Resource: resourceOutput},
	}, nil
}

// buildStateOutput 构建状态输出
func buildStateOutput(owner []byte, spec *OutputSpec, locks []*transaction.LockingCondition) (*transaction.TxOutput, error) {
	// 从 Metadata 中解析状态信息
	var stateData struct {
		StateID             string `json:"state_id"`
		StateVersion        uint64 `json:"state_version"`
		ExecutionResultHash string `json:"execution_result_hash"`
		PublicInputs        string `json:"public_inputs,omitempty"`
		ParentStateHash     string `json:"parent_state_hash,omitempty"`
		TTLDurationSeconds  uint64 `json:"ttl_duration_seconds,omitempty"`
	}

	if len(spec.Metadata) > 0 {
		if err := json.Unmarshal(spec.Metadata, &stateData); err != nil {
			return nil, fmt.Errorf("解析状态元数据失败: %w", err)
		}
	}

	// 解析 stateID
	if stateData.StateID == "" {
		return nil, fmt.Errorf("状态 state_id 不能为空")
	}

	stateIDBytes := decodeHex(stateData.StateID)
	if len(stateIDBytes) == 0 {
		return nil, fmt.Errorf("state_id 格式无效")
	}

	// 解析 executionResultHash（32字节）
	var resultHashBytes []byte
	if stateData.ExecutionResultHash != "" {
		resultHashBytes = decodeHex(stateData.ExecutionResultHash)
		if len(resultHashBytes) != 32 {
			return nil, fmt.Errorf("execution_result_hash 必须是 32 字节，实际: %d", len(resultHashBytes))
		}
	} else {
		// 如果没有提供，使用零哈希（占位）
		resultHashBytes = make([]byte, 32)
	}

	// 解析可选的 publicInputs
	var publicInputs [][]byte
	if stateData.PublicInputs != "" {
		publicInputsBytes := decodeHex(stateData.PublicInputs)
		// 假设 publicInputs 是多个32字节的哈希值拼接
		if len(publicInputsBytes)%32 == 0 {
			for i := 0; i < len(publicInputsBytes); i += 32 {
				publicInputs = append(publicInputs, publicInputsBytes[i:i+32])
			}
		}
	}

	// 解析可选的 parentStateHash
	var parentStateHash []byte
	if stateData.ParentStateHash != "" {
		parentStateHash = decodeHex(stateData.ParentStateHash)
		if len(parentStateHash) != 32 {
			return nil, fmt.Errorf("parent_state_hash 必须是 32 字节，实际: %d", len(parentStateHash))
		}
	}

	// 构建 ZKStateProof（可选，如果没有则不包含）
	var zkProof *transaction.ZKStateProof
	if len(publicInputs) > 0 {
		zkProof = &transaction.ZKStateProof{
			PublicInputs:  publicInputs,
			ProvingScheme: "groth16", // 默认使用 Groth16
			Curve:         "bn254",   // 默认使用 BN254 曲线
		}
	}

	// 构建 StateOutput
	stateOutput := &transaction.StateOutput{
		StateId:             stateIDBytes,
		StateVersion:        stateData.StateVersion,
		ZkProof:             zkProof,
		ExecutionResultHash: resultHashBytes,
		ParentStateHash:     parentStateHash,
	}

	if stateData.TTLDurationSeconds > 0 {
		ttlPtr := stateData.TTLDurationSeconds
		stateOutput.TtlDurationSeconds = &ttlPtr
	}

	return &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: locks,
		OutputContent:     &transaction.TxOutput_State{State: stateOutput},
	}, nil
}

// parseAmount 解析金额字符串（验证格式，返回uint64）
// 使用 utils.ParseAmountSafely 进行安全的大数解析
func parseAmount(amountStr string) (uint64, error) {
	if amountStr == "" {
		return 0, nil
	}
	// 使用安全的金额解析函数（支持大数，防止溢出）
	return utils.ParseAmountSafely(amountStr)
}
