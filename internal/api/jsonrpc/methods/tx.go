package methods

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ecdsacrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/weisyn/v1/internal/api/format"
	"github.com/weisyn/v1/internal/core/ispc/billing"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/introspect"
	"github.com/weisyn/v1/internal/core/ispc/hostabi"
	"github.com/weisyn/v1/internal/core/tx/selector"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	respb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	cryptoInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	resourcesvciface "github.com/weisyn/v1/pkg/interfaces/resourcesvc"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"
	pkgtypes "github.com/weisyn/v1/pkg/types"
	amountutils "github.com/weisyn/v1/pkg/utils"
	"github.com/weisyn/v1/pkg/utils/timeutil"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/ripemd160"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	defaultMintAmountWES = "100"
)

// TxMethods 交易相关方法
type TxMethods struct {
	logger              *zap.Logger
	txQuery             persistence.TxQuery
	blockQuery          persistence.BlockQuery
	utxoQuery           persistence.UTXOQuery
	resourceQuery       persistence.ResourceQuery
	pricingQuery        persistence.PricingQuery // Phase 2: 定价查询
	accountQuery        persistence.AccountQuery // 账户查询（用于余额查询）
	uresCAS             ures.CASStorage          // 用于存储资源文件
	txVerifier          tx.TxVerifier
	mempool             mempool.TxPool
	txHashCli           txpb.TransactionHashServiceClient
	blkHashCli          core.BlockHashServiceClient
	ispcCoordinator     ispc.ISPCCoordinator           // ISPC执行协调器（用于合约调用）
	addressManager      cryptoInterface.AddressManager // 地址管理器，用于验证Base58格式地址
	draftService        tx.TransactionDraftService     // 交易草稿服务（用于构建交易）
	txAdapter           hostabi.TxAdapter              // Host ABI 交易适配器（用于基于 Draft 构建交易）
	selectorService     *selector.Service              // UTXO选择器（用于构建交易）
	nonceManager        *NonceManager                  // 合约调用身份 nonce 分配器
	resourceViewService resourcesvciface.Service       // 资源视图服务（新增）
}

// TxMethodsParams 封装TxMethods的依赖参数
type TxMethodsParams struct {
	fx.In

	Logger              *zap.Logger
	QueryService        persistence.QueryService `name:"query_service"` // ✅ 匹配 persistence 模块的导出标签
	URESCAS             ures.CASStorage          `name:"cas_storage"`   // ✅ 匹配 ures 模块的导出标签
	TxVerifier          tx.TxVerifier            `name:"tx_verifier"`   // ✅ 匹配 tx 模块的导出标签
	TxPool              mempool.TxPool           `name:"tx_pool"`       // ✅ 匹配 mempool 模块的导出标签
	TxHashCli           txpb.TransactionHashServiceClient
	BlkHashCli          core.BlockHashServiceClient
	ISPCCoordinator     ispc.ISPCCoordinator           `name:"execution_coordinator"` // ISPC执行协调器
	AddressManager      cryptoInterface.AddressManager // 地址管理器，用于验证Base58格式地址
	DraftService        tx.TransactionDraftService     // 交易草稿服务（未命名依赖，从 tx 模块导出）
	SelectorService     *selector.Service              // UTXO选择器（未命名依赖，从 tx 模块导出）
	ResourceViewService resourcesvciface.Service       `optional:"true"` // 资源视图服务（可选，如果未注入则使用旧方式）
}

// NewTxMethods 创建交易方法处理器
func NewTxMethods(params TxMethodsParams) *TxMethods {
	// 打印 ISPC 协调器的状态
	if params.ISPCCoordinator == nil {
		params.Logger.Error("❌ TxMethods: ISPC协调器注入失败（nil）")
	} else {
		params.Logger.Info("✅ TxMethods: ISPC协调器注入成功")
	}

	// 调试日志：记录 TxPool 实例指针，帮助确认 API 层使用的 TxPool 是否与其他模块一致
	if params.Logger != nil && params.TxPool != nil {
		params.Logger.Info("🧩 [Fx] api.NewTxMethods 使用 TxPool 实例",
			zap.String("txpool_ptr", fmt.Sprintf("%p", params.TxPool)),
		)
	}

	// 为 Draft JSON 构建路径创建 TxAdapter（基于 DraftService / TxVerifier / Selector）
	var txAdapter hostabi.TxAdapter
	if params.DraftService != nil && params.TxVerifier != nil && params.SelectorService != nil {
		txAdapter = hostabi.NewTxAdapter(params.DraftService, params.TxVerifier, params.SelectorService)
	}

	return &TxMethods{
		logger:              params.Logger,
		txQuery:             params.QueryService, // TxQuery
		blockQuery:          params.QueryService, // BlockQuery
		utxoQuery:           params.QueryService, // UTXOQuery
		resourceQuery:       params.QueryService, // ResourceQuery
		pricingQuery:        params.QueryService, // PricingQuery (Phase 2)
		accountQuery:        params.QueryService, // AccountQuery
		uresCAS:             params.URESCAS,      // CAS存储
		txVerifier:          params.TxVerifier,
		mempool:             params.TxPool,
		txHashCli:           params.TxHashCli,
		blkHashCli:          params.BlkHashCli,
		ispcCoordinator:     params.ISPCCoordinator,
		addressManager:      params.AddressManager,
		draftService:        params.DraftService,
		txAdapter:           txAdapter,
		selectorService:     params.SelectorService,
		nonceManager:        NewNonceManager(),
		resourceViewService: params.ResourceViewService, // 可选，如果未注入则为 nil，会回退到旧方式
	}
}

// GetTransactionByHash 查询交易详情
// Method: wes_getTransactionByHash
// Params: [hash: string]
// 返回：交易对象（含状态锚点）或null（交易不存在）
func (m *TxMethods) GetTransactionByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("transaction hash required", nil)
	}

	// 解析交易哈希
	txHashStr := args[0]
	if len(txHashStr) > 2 && txHashStr[:2] == "0x" {
		txHashStr = txHashStr[2:]
	}

	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid hash format: %v", err), nil)
	}

	if len(txHash) != 32 {
		return nil, NewInvalidParamsError("hash must be 32 bytes", nil)
	}

	// 从repository查询交易（含位置）
	blockHash, txIndex, transaction, err := m.txQuery.GetTransaction(ctx, txHash)
	if err != nil || transaction == nil {
		// fallback: 查询交易池（返回完整 pending 交易结构，含 inputs/outputs）
		if m.mempool != nil {
			if pendingTx, _ := m.mempool.GetTx(txHash); pendingTx != nil {
				// 使用与已确认交易相同的格式化逻辑，blockHeight=0, txIndex=0
				pendingResp, ferr := m.formatTransactionResponse(ctx, pendingTx, nil, 0, 0)
				if ferr != nil {
					m.logger.Warn("format pending transaction failed", zap.Error(ferr))
					// 回退到最小信息
					return map[string]interface{}{
						"tx_hash": format.HashToHex(txHash),
						"status":  "pending",
					}, nil
				}
				pendingResp["status"] = "pending"
				return pendingResp, nil
			}
		}
		return nil, nil
	}

	// 获取交易所在区块高度
	blockHeight, err := m.txQuery.GetTxBlockHeight(ctx, txHash)
	if err != nil {
		m.logger.Error("Failed to get block height for transaction",
			zap.String("hash", hex.EncodeToString(txHash)),
			zap.Error(err))
		// 继续返回交易信息，高度字段为null
		blockHeight = 0
	}

	// 格式化为JSON-RPC响应
	resp, err := m.formatTransactionResponse(ctx, transaction, blockHash, blockHeight, txIndex)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetTransactionReceipt 查询交易收据
// Method: wes_getTransactionReceipt
// Params: [hash: string]
// 返回：交易收据（含状态锚点和执行结果）或null（交易不存在或未确认）
func (m *TxMethods) GetTransactionReceipt(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("transaction hash required", nil)
	}

	// 解析交易哈希
	txHashStr := args[0]
	if len(txHashStr) > 2 && txHashStr[:2] == "0x" {
		txHashStr = txHashStr[2:]
	}

	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid hash format: %v", err), nil)
	}

	if len(txHash) != 32 {
		return nil, NewInvalidParamsError("hash must be 32 bytes", nil)
	}

	// 从repository查询交易
	blockHash, txIndex, transaction, err := m.txQuery.GetTransaction(ctx, txHash)
	if err != nil || transaction == nil {
		return nil, nil // 交易不存在或未确认
	}

	// 获取交易所在区块高度
	blockHeight, err := m.txQuery.GetTxBlockHeight(ctx, txHash)
	if err != nil {
		m.logger.Error("Failed to get block height for transaction",
			zap.String("hash", hex.EncodeToString(txHash)),
			zap.Error(err))
		blockHeight = 0
	}

	// 格式化为收据响应（含状态锚点和执行结果）
	resp := map[string]interface{}{
		"tx_hash":    format.HashToHex(txHash),
		"tx_index":   txIndex,
		"block_height": blockHeight,
		"block_hash": format.HashToHex(blockHash),
	}

	// 提取交易执行状态（从输出中推断）
	// ✅ 真实（链内可验证）语义：
	// - “进块”仅表示交易通过共识/验证规则，不代表某个合约调用业务语义一定成功；
	// - WES 的 StateOutput 通过 zk_proof + public_inputs 证明执行正确性；
	// - 这里用“execution_result_hash 与 zk_proof.public_inputs 一致性”给出可验证的 status：
	//   - 一致 → 0x1
	//   - 不一致/缺失关键字段 → 0x0，并返回 statusReason
	// - 无 StateOutput（纯转账/资源等）默认视为成功：0x1
	txStatus := "0x1"
	hasStateOutput := false
	var statusReason string
	for _, output := range transaction.Outputs {
		if output != nil && output.GetState() != nil {
			hasStateOutput = true
			stateOut := output.GetState()
			if len(stateOut.ExecutionResultHash) > 0 {
				resp["execution_result_hash"] = format.HashToHex(stateOut.ExecutionResultHash)
			}
			// 强一致性校验：exec hash 必须出现在 zk public inputs 中
			ok, reason := isStateOutputReceiptSuccess(stateOut)
			if !ok {
				txStatus = "0x0"
				statusReason = reason
				break
			}
		}
	}
	resp["status"] = txStatus
	if hasStateOutput && statusReason != "" && txStatus == "0x0" {
		resp["statusReason"] = statusReason
	}

	// 获取状态锚点信息
	if block, err := m.blockQuery.GetBlockByHash(ctx, blockHash); err == nil && block != nil && block.Header != nil {
		resp["state_root"] = format.HashToHex(block.Header.StateRoot)
		resp["timestamp"] = block.Header.Timestamp
	}

	return resp, nil
}

// isStateOutputReceiptSuccess 基于链内数据对 StateOutput 执行结果做“可验证的”成功判定。
// 规则：
// - execution_result_hash 必须为 32 bytes
// - zk_proof 必须存在
// - zk_proof.public_inputs 中必须包含一个 32 bytes 值与 execution_result_hash 相同
func isStateOutputReceiptSuccess(stateOut *txpb.StateOutput) (bool, string) {
	if stateOut == nil {
		return false, "state_output_nil"
	}
	if len(stateOut.ExecutionResultHash) != 32 {
		return false, "invalid_execution_result_hash_length"
	}
	if stateOut.ZkProof == nil {
		return false, "missing_zk_proof"
	}
	for _, pi := range stateOut.ZkProof.PublicInputs {
		if len(pi) == 32 && bytes.Equal(pi, stateOut.ExecutionResultHash) {
			return true, ""
		}
	}
	return false, "execution_result_hash_not_in_public_inputs"
}

// SendRawTransaction 提交已签名交易
// Method: wes_sendRawTransaction
// Params: [signedTx: string (hex)]
// ⚠️ 零信任架构：仅接受已签名交易，不接受私钥！
// 返回：交易哈希（十六进制字符串）或详细的拒绝原因
func (m *TxMethods) SendRawTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 解析参数
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("signed transaction required", nil)
	}

	// 解析已签名交易（十六进制）
	signedTxHex := args[0]
	if len(signedTxHex) > 2 && signedTxHex[:2] == "0x" {
		signedTxHex = signedTxHex[2:]
	}

	signedTxBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid transaction hex: %v", err), nil)
	}

	// 反序列化protobuf交易
	txObj := &txpb.Transaction{}
	if err := proto.Unmarshal(signedTxBytes, txObj); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid transaction format: %v", err), nil)
	}

	// 步骤1: 验证（调用TxVerifier）
	if m.txVerifier != nil {
		if err := m.txVerifier.Verify(ctx, txObj); err != nil {
			// 签名验证失败 - 记录详细错误
			m.logger.Error("交易验证失败",
				zap.String("error", err.Error()),
				zap.Int("inputs", len(txObj.Inputs)),
				zap.Int("outputs", len(txObj.Outputs)))
			return nil, NewTxValidationFailedError(err.Error(), map[string]interface{}{
				"reason": "signature verification failed",
			})
		}
		m.logger.Info("✅ 交易验证通过",
			zap.Int("inputs", len(txObj.Inputs)),
			zap.Int("outputs", len(txObj.Outputs)))
	}

	// 步骤2: 计算交易哈希
	if m.txHashCli == nil {
		return nil, NewInternalError("transaction hash service not available", nil)
	}
	hResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: txObj})
	if err != nil || hResp == nil || len(hResp.Hash) == 0 {
		return nil, NewInternalError("failed to compute transaction hash", nil)
	}
	txHash := hResp.Hash

	// 步骤3: 提交到内存池（细化错误处理）
	if m.mempool == nil {
		return nil, NewInternalError("mempool not available", nil)
	}
	if _, err := m.mempool.SubmitTx(txObj); err != nil {
		// 根据错误类型返回细化的错误码
		errMsg := err.Error()

		// 费率过低
		if strings.Contains(errMsg, "fee too low") ||
			strings.Contains(errMsg, "insufficient fee") {
			return nil, NewTxValidationFailedError("Transaction fee too low", map[string]interface{}{
				"error": errMsg,
				"hint":  "Use wes_estimateFee to get recommended fee rate",
			})
		}

		// 交易已存在
		if strings.Contains(errMsg, "already known") ||
			strings.Contains(errMsg, "duplicate") ||
			strings.Contains(errMsg, "already in pool") {
			return nil, NewTxValidationFailedError("Transaction already known", map[string]interface{}{"tx_hash": format.HashToHex(txHash)})
		}

		// 交易冲突（UTXO 双花）
		if strings.Contains(errMsg, "conflict") ||
			strings.Contains(errMsg, "double spend") ||
			strings.Contains(errMsg, "input already spent") {
			return nil, NewTxValidationFailedError("Transaction conflicts", map[string]interface{}{
				"error": errMsg,
				"hint":  "One or more inputs are already spent by another transaction",
			})
		}

		// 内存池已满
		if strings.Contains(errMsg, "pool is full") ||
			strings.Contains(errMsg, "mempool full") ||
			strings.Contains(errMsg, "capacity exceeded") {
			return nil, NewServiceUnavailableError("Mempool is full", nil)
		}

		// 其他内部错误
		return nil, NewInternalError(errMsg, nil)
	}

	// 返回交易哈希
	return format.HashToHex(txHash), nil
}

// EstimateFee 估算交易费用
// Method: wes_estimateFee
// Params: [tx: object] - 交易草稿对象
// 返回：费用估算（含基础费用、优先级费用和预计确认时间）
func (m *TxMethods) EstimateFee(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 1. 解析参数
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("transaction object required", nil)
	}

	txData, ok := args[0].(map[string]interface{})
	if !ok {
		return nil, NewInvalidParamsError("transaction must be object", nil)
	}

	// 2. 统计输入输出数量（用于返回信息）
	numInputs := 0
	numOutputs := 0
	if inputs, ok := txData["inputs"].([]interface{}); ok {
		numInputs = len(inputs)
	}
	if outputs, ok := txData["outputs"].([]interface{}); ok {
		numOutputs = len(outputs)
	}

	// 3. 按金额比例估算手续费：万分之三（0.03%），与旧CLI一致（无最低）
	var transferAmount uint64
	if amountStr, ok := txData["amount"].(string); ok {
		if amt, ok := new(big.Int).SetString(amountStr, 10); ok && amt.IsUint64() {
			transferAmount = amt.Uint64()
		}
	}

	var estimatedFee uint64
	if transferAmount > 0 {
		feeBig := new(big.Int).Mul(new(big.Int).SetUint64(transferAmount), big.NewInt(3))
		feeBig.Div(feeBig, big.NewInt(10000))
		if feeBig.IsUint64() {
			estimatedFee = feeBig.Uint64()
		}
	}

	return map[string]interface{}{
		"estimated_fee": estimatedFee,
		"fee_rate":      "3 bps (0.03%)",
		"num_inputs":    numInputs,
		"num_outputs":   numOutputs,
	}, nil
}

// formatTransactionResponse 格式化交易响应（含状态锚点）
func (m *TxMethods) formatTransactionResponse(ctx context.Context, transaction *txpb.Transaction, blockHash []byte, blockHeight uint64, txIndex uint32) (map[string]interface{}, error) {
	if m.txHashCli == nil {
		return nil, NewInternalError("transaction hash service not available", nil)
	}
	hResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: transaction})
	if err != nil || hResp == nil || len(hResp.Hash) == 0 {
		return nil, NewInternalError("failed to compute transaction hash", nil)
	}

	// 使用 protojson 将完整的交易转换为 JSON
	// 这样可以包含所有字段，包括 inputs、outputs、state_output 等
	protojsonMarshaler := &protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	txJSON, err := protojsonMarshaler.Marshal(transaction)
	if err != nil {
		m.logger.Warn("序列化完整交易失败，使用精简格式", zap.Error(err))
		// 如果序列化失败，回退到精简格式
		resp := map[string]interface{}{
			"tx_hash":      format.HashToHex(hResp.Hash),
			"block_height": blockHeight,
			"block_hash":   format.HashToHex(blockHash),
			"tx_index":     txIndex,
		}
		return resp, nil
	}

	// 解析 JSON 以便添加额外字段
	var txMap map[string]interface{}
	if err := json.Unmarshal(txJSON, &txMap); err != nil {
		m.logger.Warn("解析交易JSON失败，使用精简格式", zap.Error(err))
		resp := map[string]interface{}{
			"tx_hash":      format.HashToHex(hResp.Hash),
			"block_height": blockHeight,
			"block_hash":   format.HashToHex(blockHash),
			"tx_index":     txIndex,
		}
		return resp, nil
	}

	// 添加区块信息
	txMap["tx_hash"] = format.HashToHex(hResp.Hash)
	txMap["block_height"] = blockHeight
	txMap["block_hash"] = format.HashToHex(blockHash)
	txMap["tx_index"] = txIndex

	return txMap, nil
}

// attachResourceReferenceInput 为交易追加 ResourceInput（引用不消费），显式表达合约/模型调用对资源UTXO的只读引用。
//
// 设计原则（对齐 transaction.proto 与文档约束）：
//   - 通过 TxInput.previous_output 精确定位 ResourceOutput 所在的部署交易UTXO
//   - 使用 is_reference_only = true 表达“引用不消费”
//   - 不改变现有费用/资产逻辑，仅补充资源引用语义
//   - 失败时采用“最佳努力”：记录日志但不影响原有交易流程
func (m *TxMethods) attachResourceReferenceInput(ctx context.Context, tx *txpb.Transaction, resourceHash []byte) {
	// 基本防御性检查
	if tx == nil {
		return
	}
	if len(resourceHash) != 32 {
		// 只接受标准32字节内容哈希
		return
	}
	if m.resourceQuery == nil {
		return
	}

	// 1. 查询资源对应的部署交易
	txHash, _, _, err := m.resourceQuery.GetResourceTransaction(ctx, resourceHash)
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("attachResourceReferenceInput: 查询资源部署交易失败，跳过引用输入追加",
				zap.Error(err))
		}
		return
	}
	if len(txHash) != 32 {
		if m.logger != nil {
			m.logger.Warn("attachResourceReferenceInput: 部署交易哈希长度无效，跳过引用输入追加",
				zap.Int("length", len(txHash)))
		}
		return
	}

	// 2. 如果已经存在对该部署交易的只读引用输入，则不重复追加
	for _, input := range tx.Inputs {
		if input == nil {
			continue
		}
		if !input.IsReferenceOnly {
			continue
		}
		prev := input.GetPreviousOutput()
		if prev == nil {
			continue
		}
		if bytes.Equal(prev.TxId, txHash) {
			// 已存在引用，不再重复追加
			return
		}
	}

	// 3. 追加新的 ResourceInput（引用不消费）
	refInput := &txpb.TxInput{
		PreviousOutput: &txpb.OutPoint{
			TxId:        txHash,
			OutputIndex: 0, // 当前资源部署交易的 ResourceOutput 默认位于索引0
		},
		IsReferenceOnly: true,
		Sequence:        0,
		// UnlockingProof 留空：ExecutionProof / 签名等由后续流程（如 populateExecutionProofIdentities）补全
	}

	tx.Inputs = append(tx.Inputs, refInput)

	if m.logger != nil {
		m.logger.Info("✅ 已为交易追加资源引用输入（引用不消费）",
			zap.String("resource_tx_hash", hex.EncodeToString(txHash)),
			zap.Int("total_inputs", len(tx.Inputs)))
	}
}

// buildExecutionResourceTransaction 统一构建“可执行资源调用”交易（合约/模型/未来执行体）。
//
// 语义：
//   - 接收 ISPC 返回的 StateOutput（包含 ZKStateProof）和可选的 DraftTransaction
//   - 保留 DraftTransaction 中已有的资产/资源输出
//   - 追加或覆盖 StateOutput（以调用者地址作为锁定条件）
//   - 通过 attachResourceReferenceInput 显式添加 ResourceInput（引用不消费部署UTXO）
//
// 该函数是所有可执行资源调用交易的唯一构建入口，避免合约/模型各自散落实现。
func (m *TxMethods) buildExecutionResourceTransaction(
	ctx context.Context,
	draft *txpb.Transaction,
	stateOutput *txpb.StateOutput,
	resourceHash []byte,
	callerAddrBytes []byte,
) (*txpb.Transaction, error) {
	if stateOutput == nil {
		return nil, fmt.Errorf("stateOutput cannot be nil")
	}

	// 1. 构建状态输出 TxOutput（供合并/追加使用）
	stateTxOutput := &txpb.TxOutput{
		OutputContent: &txpb.TxOutput_State{
			State: stateOutput,
		},
		LockingConditions: []*txpb.LockingCondition{
			{
				Condition: &txpb.LockingCondition_SingleKeyLock{
					SingleKeyLock: &txpb.SingleKeyLock{
						KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
							RequiredAddressHash: callerAddrBytes,
						},
					},
				},
			},
		},
	}

	// 2. 基础交易对象：优先使用 DraftTransaction（合约可能在执行过程中构建了草稿）
	tx := draft
	if tx == nil {
		tx = &txpb.Transaction{
			Version: 1,
			Inputs:  []*txpb.TxInput{},
			Outputs: []*txpb.TxOutput{},
		}
	}

	// 3. 规范化基础字段
	if tx.Version == 0 {
		tx.Version = 1
	}
	if tx.CreationTimestamp == 0 {
		tx.CreationTimestamp = uint64(time.Now().Unix())
	}
	if tx.Inputs == nil {
		tx.Inputs = []*txpb.TxInput{}
	}
	if tx.Outputs == nil {
		tx.Outputs = []*txpb.TxOutput{}
	}

	// 4. 合并/追加 StateOutput
	hasStateOutput := false
	for _, out := range tx.Outputs {
		if existingState := out.GetState(); existingState != nil {
			// 如果已有状态输出，且 state_id 为空或等于当前 stateOutput，则直接覆盖
			if len(existingState.StateId) == 0 || bytes.Equal(existingState.StateId, stateOutput.StateId) {
				out.OutputContent = &txpb.TxOutput_State{State: stateOutput}
				out.LockingConditions = stateTxOutput.LockingConditions
				hasStateOutput = true
				break
			}
		}
	}
	if !hasStateOutput {
		tx.Outputs = append(tx.Outputs, stateTxOutput)
	}

	// 5. 为交易追加资源引用输入（引用不消费）
	m.attachResourceReferenceInput(ctx, tx, resourceHash)

	return tx, nil
}

// formatReceiptResponse 格式化收据响应（含状态锚点和执行结果）
// 收据格式将在WES执行结果稳定后补充

// SendTransaction 执行转账（内部三步流程：构建→签名→提交）
// Method: wes_sendTransaction
// Params: [{fromAddress: string, toAddress: string, amount: string, privateKey: string}]
// 返回：{txHash: string, accepted: bool, reason: string}
// 注意：这是一个完整的转账接口，内部会完成构建、签名、验证、提交全流程
func (m *TxMethods) SendTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("wes_sendTransaction: 开始转账流程")

	// 解析参数（数组格式）
	var argsArray []interface{}
	if err := json.Unmarshal(params, &argsArray); err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
	}

	if len(argsArray) == 0 {
		return nil, NewInvalidParamsError("parameters required", nil)
	}

	// 第一个参数应该是包含 fromAddress, toAddress, amount, privateKey 的对象
	argsMap, ok := argsArray[0].(map[string]interface{})
	if !ok {
		return nil, NewInvalidParamsError("first parameter must be an object", nil)
	}

	// 提取参数
	fromAddress, _ := argsMap["fromAddress"].(string)
	toAddress, _ := argsMap["toAddress"].(string)
	amount, _ := argsMap["amount"].(string)
	privateKey, _ := argsMap["privateKey"].(string)

	// 验证参数
	if fromAddress == "" || toAddress == "" || amount == "" || privateKey == "" {
		return nil, NewInvalidParamsError("fromAddress, toAddress, amount, and privateKey are required", nil)
	}

	// 解析地址（WES使用Base58格式，不兼容ETH的0x前缀格式）
	if m.addressManager == nil {
		return nil, NewInternalError("address manager not available", nil)
	}

	// 拒绝0x前缀的ETH地址格式
	if len(fromAddress) > 2 && (fromAddress[:2] == "0x" || fromAddress[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}
	if len(toAddress) > 2 && (toAddress[:2] == "0x" || toAddress[:2] == "0X") {
		return nil, NewInvalidParamsError("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式", nil)
	}

	// 验证并转换Base58格式地址
	validFromAddress, err := m.addressManager.StringToAddress(fromAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid fromAddress format: %v", err), nil)
	}
	validToAddress, err := m.addressManager.StringToAddress(toAddress)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid toAddress format: %v", err), nil)
	}

	// 转换为字节数组
	fromBytes, err := m.addressManager.AddressToBytes(validFromAddress)
	if err != nil || len(fromBytes) != 20 {
		return nil, NewInvalidParamsError("invalid fromAddress format", nil)
	}

	toBytes, err := m.addressManager.AddressToBytes(validToAddress)
	if err != nil || len(toBytes) != 20 {
		return nil, NewInvalidParamsError("invalid toAddress format", nil)
	}

	// 解析金额
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok || amountBig.Sign() <= 0 {
		return nil, NewInvalidParamsError("invalid amount", nil)
	}

	m.logger.Info("🔍 [DEBUG] 接收到的转账参数",
		zap.String("amount_string", amount),
		zap.String("amount_big_int", amountBig.String()),
		zap.Uint64("amount_uint64", amountBig.Uint64()),
	)

	// 解析私钥
	privateKeyBytes, err := hex.DecodeString(strings.TrimPrefix(privateKey, "0x"))
	if err != nil || len(privateKeyBytes) != 32 {
		return nil, NewInvalidParamsError("invalid privateKey format", nil)
	}

	ecdsaPrivateKey, err := ecdsacrypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid private key: %v", err), nil)
	}

	// ========== 步骤1：构建交易 ==========
	m.logger.Info("wes_sendTransaction: 步骤1 - 构建未签名交易")

	tx, err := m.buildTransferTransaction(ctx, fromBytes, toBytes, amountBig.Uint64())
	if err != nil {
		return map[string]interface{}{
			"accepted": false,
			"reason":   fmt.Sprintf("构建交易失败: %v", err),
		}, nil
	}

	// ========== 步骤2：签名交易 ==========
	m.logger.Info("wes_sendTransaction: 步骤2 - 签名交易")

	if err := m.signTransaction(ctx, tx, ecdsaPrivateKey, fromBytes); err != nil {
		return map[string]interface{}{
			"accepted": false,
			"reason":   fmt.Sprintf("签名失败: %v", err),
		}, nil
	}

	// ========== 步骤3：验证并提交交易 ==========
	m.logger.Info("wes_sendTransaction: 步骤3 - 验证并提交交易")

	// 验证交易
	if m.txVerifier != nil {
		if err := m.txVerifier.Verify(ctx, tx); err != nil {
			m.logger.Error("交易验证失败", zap.Error(err))
			return map[string]interface{}{
				"accepted": false,
				"reason":   fmt.Sprintf("验证失败: %v", err),
			}, nil
		}
		m.logger.Info("✅ 交易验证通过")
	}

	// 计算交易哈希
	if m.txHashCli == nil {
		return nil, NewInternalError("transaction hash service not available", nil)
	}
	hResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{Transaction: tx})
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to compute tx hash: %v", err), nil)
	}
	txHashHex := format.HashToHex(hResp.Hash)

	// 提交到mempool
	if m.mempool != nil {
		if _, err := m.mempool.SubmitTx(tx); err != nil {
			m.logger.Error("提交到mempool失败", zap.Error(err))
			return map[string]interface{}{
				"accepted": false,
				"reason":   fmt.Sprintf("提交失败: %v", err),
			}, nil
		}
	}

	m.logger.Info("wes_sendTransaction: 转账成功", zap.String("txHash", txHashHex))

	return map[string]interface{}{
		"txHash":   txHashHex,
		"accepted": true,
	}, nil
}

// buildTransferTransaction 构建转账交易（查UTXO、计算找零）
func (m *TxMethods) buildTransferTransaction(
	ctx context.Context,
	fromAddress []byte,
	toAddress []byte,
	amount uint64,
) (*txpb.Transaction, error) {
	// 查询UTXO
	utxos, err := m.utxoQuery.GetUTXOsByAddress(ctx, fromAddress, nil, true)
	if err != nil {
		return nil, fmt.Errorf("查询UTXO失败: %w", err)
	}
	if len(utxos) == 0 {
		return nil, fmt.Errorf("没有可用的UTXO")
	}

	m.logger.Debug("查询到UTXO", zap.Int("count", len(utxos)))

	// 选择UTXO（简化：使用第一个足够的）
	var selectedUTXO *utxopb.UTXO
	for _, utxo := range utxos {
		// 只选择资产类型的UTXO
		if utxo.Category != utxopb.UTXOCategory_UTXO_CATEGORY_ASSET {
			continue
		}

		// 解析UTXO金额
		output := utxo.GetCachedOutput()
		if output == nil {
			continue // 跳过没有缓存输出的UTXO
		}
		utxoContent := output.GetOutputContent()
		if asset, ok := utxoContent.(*txpb.TxOutput_Asset); ok && asset.Asset != nil {
			if nativeCoin, ok := asset.Asset.GetAssetContent().(*txpb.AssetOutput_NativeCoin); ok && nativeCoin.NativeCoin != nil {
				utxoAmount := new(big.Int)
				utxoAmount.SetString(nativeCoin.NativeCoin.Amount, 10)
				// 内扣模型：所需金额即为用户输入金额（手续费从该金额内扣）
				requiredAmount := new(big.Int).SetUint64(amount)
				if utxoAmount.Cmp(requiredAmount) >= 0 {
					selectedUTXO = utxo
					break
				}
			}
		}
	}

	if selectedUTXO == nil {
		return nil, fmt.Errorf("余额不足")
	}

	// 获取UTXO金额
	outputContent := selectedUTXO.GetCachedOutput().GetOutputContent()
	asset, ok := outputContent.(*txpb.TxOutput_Asset)
	if !ok || asset.Asset == nil {
		return nil, fmt.Errorf("选中的UTXO不是资产类型")
	}
	nativeCoinWrapper, ok := asset.Asset.GetAssetContent().(*txpb.AssetOutput_NativeCoin)
	if !ok || nativeCoinWrapper.NativeCoin == nil {
		return nil, fmt.Errorf("选中的UTXO不是原生币")
	}
	nativeCoin := nativeCoinWrapper.NativeCoin
	utxoAmountBig := new(big.Int)
	utxoAmountBig.SetString(nativeCoin.Amount, 10)

	m.logger.Debug("选中UTXO", zap.String("amount", utxoAmountBig.String()))

	// 计算手续费（万分之三，按金额内扣）
	transferAmountBig := new(big.Int).SetUint64(amount)
	feeBig := new(big.Int).Mul(transferAmountBig, big.NewInt(3)) // amount × 3
	feeBig.Div(feeBig, big.NewInt(10000))                        // ÷ 10000 = 0.03%

	// 计算找零（已内扣手续费，找零不再扣费）
	changeBig := new(big.Int).Sub(utxoAmountBig, transferAmountBig)

	// 构建protobuf交易
	tx := &txpb.Transaction{
		Version: 1,
		Inputs: []*txpb.TxInput{
			{
				PreviousOutput:  selectedUTXO.GetOutpoint(),
				IsReferenceOnly: false,
				Sequence:        0,
			},
		},
		Outputs: []*txpb.TxOutput{
			{
				Owner: toAddress,
				OutputContent: &txpb.TxOutput_Asset{
					Asset: &txpb.AssetOutput{
						AssetContent: &txpb.AssetOutput_NativeCoin{
							NativeCoin: &txpb.NativeCoinAsset{
								// 接收方收到的金额 = 用户输入金额 - 手续费
								Amount: new(big.Int).Sub(transferAmountBig, feeBig).String(),
							},
						},
					},
				},
				LockingConditions: []*txpb.LockingCondition{
					{
						Condition: &txpb.LockingCondition_SingleKeyLock{
							SingleKeyLock: &txpb.SingleKeyLock{
								KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: toAddress,
								},
							},
						},
					},
				},
			},
		},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte("testnet"),
	}

	// 添加找零输出
	if changeBig.Sign() > 0 {
		tx.Outputs = append(tx.Outputs, &txpb.TxOutput{
			Owner: fromAddress,
			OutputContent: &txpb.TxOutput_Asset{
				Asset: &txpb.AssetOutput{
					AssetContent: &txpb.AssetOutput_NativeCoin{
						NativeCoin: &txpb.NativeCoinAsset{
							Amount: changeBig.String(),
						},
					},
				},
			},
			LockingConditions: []*txpb.LockingCondition{
				{
					Condition: &txpb.LockingCondition_SingleKeyLock{
						SingleKeyLock: &txpb.SingleKeyLock{
							KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: fromAddress,
							},
						},
					},
				},
			},
		})
	}

	// 调试：打印交易详情
	m.logger.Info("✅ 交易构建成功",
		zap.Int("inputs", len(tx.Inputs)),
		zap.Int("outputs", len(tx.Outputs)),
		zap.String("utxo_amount", utxoAmountBig.String()),
		zap.String("transfer_amount", transferAmountBig.String()),
		zap.String("fee", feeBig.String()),
		zap.String("receiver_amount", new(big.Int).Sub(transferAmountBig, feeBig).String()),
		zap.String("change_amount", changeBig.String()),
	)
	return tx, nil
}

// signTransaction 签名交易
func (m *TxMethods) signTransaction(
	ctx context.Context,
	tx *txpb.Transaction,
	privateKey *ecdsa.PrivateKey,
	fromAddress []byte,
) error {
	// 查找需要签名的输入（转账交易使用消费型输入 is_reference_only=false）
	// 对于转账交易，应该签名消费型输入（is_reference_only=false）
	// 对于合约/模型调用，可能需要签名引用型输入（is_reference_only=true）
	var inputIndex int = -1
	for idx, input := range tx.Inputs {
		if input != nil {
			// 优先使用消费型输入（转账场景）
			if !input.IsReferenceOnly {
				inputIndex = idx
				break
			}
			// 如果没有消费型输入，使用引用型输入（合约/模型调用场景）
			if inputIndex < 0 && input.IsReferenceOnly {
				inputIndex = idx
			}
		}
	}
	if inputIndex < 0 {
		return fmt.Errorf("未找到需要签名的输入")
	}
	sighashType := txpb.SignatureHashType_SIGHASH_ALL

	if m.txHashCli == nil {
		return fmt.Errorf("transaction hash service not available")
	}

	sigHashResp, err := m.txHashCli.ComputeSignatureHash(ctx, &txpb.ComputeSignatureHashRequest{
		Transaction:      tx,
		InputIndex:       uint32(inputIndex),
		SighashType:      sighashType,
		IncludeDebugInfo: false,
	})
	if err != nil {
		return fmt.Errorf("计算签名哈希失败: %w", err)
	}
	if sigHashResp == nil || !sigHashResp.IsValid || len(sigHashResp.Hash) == 0 {
		return fmt.Errorf("签名哈希响应无效")
	}
	sigHashBytes := sigHashResp.Hash

	// 签名
	signature65, err := ecdsacrypto.Sign(sigHashBytes, privateKey)
	if err != nil {
		return fmt.Errorf("签名失败: %w", err)
	}
	signature := signature65[:64] // 移除recovery ID
	signature = normalizeSignature(signature)

	// 获取压缩公钥
	fromPublicKey := ecdsacrypto.CompressPubkey(&privateKey.PublicKey)

	// 验证地址匹配
	computedAddr := hash160(fromPublicKey)
	if !bytes.Equal(computedAddr, fromAddress) {
		return fmt.Errorf("私钥与地址不匹配")
	}

	// 填充签名
	tx.Inputs[inputIndex].UnlockingProof = &txpb.TxInput_SingleKeyProof{
		SingleKeyProof: &txpb.SingleKeyProof{
			Signature: &txpb.SignatureData{
				Value: signature,
			},
			PublicKey: &txpb.PublicKey{
				Value: fromPublicKey,
			},
			Algorithm:   txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
			SighashType: sighashType,
		},
	}

	m.logger.Info("✅ 交易签名完成", zap.Int("sig_len", len(signature)), zap.Int("pubkey_len", len(fromPublicKey)))
	return nil
}

// hash160 计算 RIPEMD160(SHA256(data))
func hash160(data []byte) []byte {
	h1 := sha256.Sum256(data)
	h2 := ripemd160.New()
	h2.Write(h1[:])
	return h2.Sum(nil)
}

// normalizeSignature 规范化签名为 low-S 格式
func normalizeSignature(sig []byte) []byte {
	if len(sig) != 64 {
		return sig
	}

	// secp256k1 的 N/2
	halfOrder := new(big.Int)
	halfOrder.SetString("7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0", 16)

	s := new(big.Int).SetBytes(sig[32:64])
	if s.Cmp(halfOrder) > 0 {
		// S 太大，计算 N - S
		order := new(big.Int)
		order.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
		s.Sub(order, s)

		// 重新构造签名
		normalizedSig := make([]byte, 64)
		copy(normalizedSig[:32], sig[:32]) // R 不变
		sBytes := s.Bytes()
		copy(normalizedSig[64-len(sBytes):], sBytes) // S 规范化
		return normalizedSig
	}

	return sig
}

// ============================================================================
// 智能合约相关RPC方法
// ============================================================================

// DeployContract 部署智能合约 (wes_deployContract)
//
// 🎯 **功能**：完整的合约部署流程（存储WASM、构建交易、签名、提交）
//
// 📋 **参数**（JSON格式）：
//
//	{
//	  "private_key": "十六进制私钥",
//	  "wasm_content": "Base64编码的WASM文件内容",
//	  "abi_version": "v1",
//	  "name": "合约名称",
//	  "description": "合约描述（可选）"
//	}
//
// 📋 **返回**（JSON格式）：
//
//	{
//	  "content_hash": "合约ID（64位十六进制）",
//	  "tx_hash": "交易哈希（64位十六进制）",
//	  "success": true,
//	  "message": "部署成功"
//	}
func (m *TxMethods) DeployContract(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("📤 [wes_deployContract] 开始处理合约部署请求")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		PrivateKey        string                   `json:"private_key"`
		WasmContent       string                   `json:"wasm_content"` // Base64编码的WASM内容
		AbiVersion        string                   `json:"abi_version"`
		Name              string                   `json:"name"`
		Description       string                   `json:"description"`
		InitArgs          string                   `json:"init_args,omitempty"`          // Base64编码，可选
		LockingConditions []map[string]interface{} `json:"locking_conditions,omitempty"` // ✅ 新增：锁定条件列表
	}

	// 尝试解析数组格式：[{...}]
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		// 成功解析为数组，取第一个元素
		paramsBytes, err := json.Marshal(paramsArray[0])
		if err != nil {
			m.logger.Error("序列化参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("marshal params object: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &req); err != nil {
			m.logger.Error("解析参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params object: %w", err)
		}
	} else {
		// 尝试直接解析为对象：{...}
		if err := json.Unmarshal(params, &req); err != nil {
			m.logger.Error("解析参数失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if req.PrivateKey == "" {
		return nil, fmt.Errorf("private_key is required")
	}
	if req.WasmContent == "" {
		return nil, fmt.Errorf("wasm_content is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.AbiVersion == "" {
		req.AbiVersion = "v1" // 默认ABI版本
	}

	m.logger.Info("🔍 [DEBUG] 收到合约部署参数",
		zap.String("name", req.Name),
		zap.String("abi_version", req.AbiVersion),
		zap.Int("wasm_content_length", len(req.WasmContent)),
	)

	// ========== 1. 解码Base64 WASM内容 ==========
	wasmBytes, err := base64.StdEncoding.DecodeString(req.WasmContent)
	if err != nil {
		m.logger.Error("解码WASM内容失败", zap.Error(err))
		return nil, fmt.Errorf("decode wasm content: %w", err)
	}

	m.logger.Info("✅ WASM内容解码成功", zap.Int("size_bytes", len(wasmBytes)))

	// ========== 2. 验证WASM格式（魔数检查）==========
	if len(wasmBytes) < 4 || wasmBytes[0] != 0x00 || wasmBytes[1] != 0x61 || wasmBytes[2] != 0x73 || wasmBytes[3] != 0x6D {
		m.logger.Error("无效的WASM文件：魔数不匹配")
		return nil, fmt.Errorf("invalid wasm file: magic number mismatch")
	}

	m.logger.Info("✅ WASM格式验证通过")

	// ========== 3. 保存WASM到临时文件 ==========
	tempDir := os.TempDir()
	tempFileName := fmt.Sprintf("contract-%s-%d.wasm", req.Name, time.Now().UnixNano())
	tempFilePath := filepath.Join(tempDir, tempFileName)

	if err := os.WriteFile(tempFilePath, wasmBytes, 0600); err != nil {
		m.logger.Error("保存临时WASM文件失败", zap.Error(err))
		return nil, fmt.Errorf("save temp wasm file: %w", err)
	}
	defer os.Remove(tempFilePath) // 确保清理临时文件

	m.logger.Info("✅ WASM临时文件已创建", zap.String("path", tempFilePath))

	// ========== 4. 存储文件到CAS并获取contentHash ==========
	// 计算文件内容哈希
	hash := sha256.Sum256(wasmBytes)
	contentHash := hash[:]
	// 存储文件到CAS
	if err := m.uresCAS.StoreFile(ctx, contentHash, wasmBytes); err != nil {
		m.logger.Error("存储WASM文件到CAS失败", zap.Error(err))
		return nil, fmt.Errorf("store wasm file: %w", err)
	}

	contentHashHex := hex.EncodeToString(contentHash)
	m.logger.Info("✅ WASM文件已存储", zap.String("content_hash", contentHashHex))

	// ========== 5. 解析WASM导出函数 ==========
	exportedFunctions, err := introspect.ExtractExportedFunctions(tempFilePath)
	if err != nil {
		m.logger.Error("解析WASM导出函数失败", zap.Error(err))
		return nil, fmt.Errorf("extract exported functions: %w", err)
	}

	m.logger.Info("✅ WASM导出函数解析成功", zap.Strings("functions", exportedFunctions))

	// ========== 6. 构建Contract Resource protobuf ==========
	contractResource := &respb.Resource{
		Category:         respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType:   respb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
		Name:             req.Name,
		Version:          "1.0",
		MimeType:         "application/wasm",
		ContentHash:      contentHash,
		Size:             uint64(len(wasmBytes)),
		Description:      req.Description,
		CreatedTimestamp: uint64(time.Now().Unix()),
		OriginalFilename: req.Name + ".wasm",
		FileExtension:    ".wasm",
		ExecutionConfig: &respb.Resource_Contract{
			Contract: &respb.ContractExecutionConfig{
				AbiVersion:        req.AbiVersion,
				ExportedFunctions: exportedFunctions,
			},
		},
	}

	// 🔍 调试日志：检查 ExecutionConfig 是否设置
	if contractResource.ExecutionConfig != nil {
		if contract, ok := contractResource.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
			m.logger.Info("🔍 [DEBUG] DeployContract: ExecutionConfig 已设置",
				zap.String("abi_version", contract.Contract.AbiVersion),
				zap.Int("exported_functions_count", len(contract.Contract.ExportedFunctions)),
				zap.Strings("exported_functions", contract.Contract.ExportedFunctions),
			)
		} else {
			m.logger.Warn("🔍 [DEBUG] DeployContract: ExecutionConfig 类型不匹配或为空")
		}
	} else {
		m.logger.Error("🔍 [DEBUG] DeployContract: ExecutionConfig 为 nil")
	}

	m.logger.Info("✅ Contract Resource protobuf构建完成")

	// 🔍 调试日志：在构建 ResourceOutput 前再次确认 contractResource 的 ExecutionConfig
	if contractResource.ExecutionConfig != nil {
		if contract, ok := contractResource.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
			m.logger.Info("🔍 [DEBUG] DeployContract: contractResource 确认包含 ExecutionConfig",
				zap.String("abi_version", contract.Contract.AbiVersion),
				zap.Int("functions_count", len(contract.Contract.ExportedFunctions)),
			)
		} else {
			m.logger.Error("🔍 [DEBUG] DeployContract: contractResource.ExecutionConfig 类型不匹配",
				zap.String("type", fmt.Sprintf("%T", contractResource.ExecutionConfig)),
			)
		}
	} else {
		m.logger.Error("🔍 [DEBUG] DeployContract: contractResource.ExecutionConfig 为 nil（不应该发生！）")
	}

	// ========== 7. 从私钥推导部署者地址 ==========
	privateKeyBytes, err := hex.DecodeString(strings.TrimPrefix(req.PrivateKey, "0x"))
	if err != nil {
		m.logger.Error("解码私钥失败", zap.Error(err))
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	privateKey, err := ecdsacrypto.ToECDSA(privateKeyBytes)
	if err != nil {
		m.logger.Error("解析私钥失败", zap.Error(err))
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	publicKey := ecdsacrypto.CompressPubkey(&privateKey.PublicKey)
	ownerAddrBytes := hash160(publicKey)

	m.logger.Info("✅ 部署者地址推导完成", zap.String("address_hex", hex.EncodeToString(ownerAddrBytes)))

	// ========== 8. 推导合约地址 ==========
	contractAddrBytes := hash160(contentHash)
	if len(contractAddrBytes) != 20 {
		return nil, fmt.Errorf("invalid contract address length: %d", len(contractAddrBytes))
	}
	m.logger.Info("✅ 合约地址推导完成", zap.String("contract_address_hex", hex.EncodeToString(contractAddrBytes)))

	// ========== 9. 构建ResourceOutput ==========
	resourceOutput := &txpb.ResourceOutput{
		Resource:          contractResource,
		CreationTimestamp: timeutil.NowUnix(),
		StorageStrategy:   txpb.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
		IsImmutable:       true, // 智能合约一旦部署不可变
	}

	// 🔍 调试日志：构建 ResourceOutput 后立即检查
	if resourceOutput.Resource != nil {
		if resourceOutput.Resource.ExecutionConfig != nil {
			if contract, ok := resourceOutput.Resource.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
				m.logger.Info("🔍 [DEBUG] DeployContract: ResourceOutput.Resource 包含 ExecutionConfig",
					zap.String("abi_version", contract.Contract.AbiVersion),
					zap.Int("functions_count", len(contract.Contract.ExportedFunctions)),
				)
			} else {
				m.logger.Warn("🔍 [DEBUG] DeployContract: ResourceOutput.Resource.ExecutionConfig 类型不匹配")
			}
		} else {
			m.logger.Error("🔍 [DEBUG] DeployContract: ResourceOutput.Resource.ExecutionConfig 为 nil（在赋值后立即丢失！）")
		}
	} else {
		m.logger.Error("🔍 [DEBUG] DeployContract: ResourceOutput.Resource 为 nil")
	}

	// ========== 10. 构建锁定条件 ==========
	var lockingConditions []*txpb.LockingCondition
	if len(req.LockingConditions) > 0 {
		// ✅ 解析用户提供的锁定条件
		parsedConditions, err := m.parseLockingConditions(req.LockingConditions, ownerAddrBytes)
		if err != nil {
			m.logger.Error("解析锁定条件失败", zap.Error(err))
			return nil, fmt.Errorf("parse locking conditions: %w", err)
		}
		lockingConditions = parsedConditions
		m.logger.Info("✅ 使用用户指定的锁定条件", zap.Int("count", len(lockingConditions)))
	} else {
		// 默认：单密钥锁（部署者地址）
		lockingConditions = m.createDefaultSingleKeyLock(ownerAddrBytes)
		m.logger.Info("✅ 使用默认单密钥锁（部署者地址）")
	}

	// ========== 11. 构建TxOutput ==========
	txOutput := &txpb.TxOutput{
		Owner: contractAddrBytes,
		OutputContent: &txpb.TxOutput_Resource{
			Resource: resourceOutput,
		},
		LockingConditions: lockingConditions,
	}

	// ========== 12. 构建交易（无输入，只有资源输出）==========
	transaction := &txpb.Transaction{
		Version:           1,
		CreationTimestamp: uint64(time.Now().Unix()),
		Inputs:            []*txpb.TxInput{}, // 合约部署无UTXO输入
		Outputs:           []*txpb.TxOutput{txOutput},
	}

	m.logger.Info("✅ 交易构建完成")

	// 🔍 调试日志：在提交前检查交易中的 Resource 是否包含 ExecutionConfig
	if len(transaction.Outputs) > 0 {
		for i, output := range transaction.Outputs {
			if output != nil {
				if resourceOutput := output.GetResource(); resourceOutput != nil && resourceOutput.Resource != nil {
					resource := resourceOutput.Resource
					// 检查 contractResource 引用是否相同
					if &resource == &contractResource {
						m.logger.Info("🔍 [DEBUG] DeployContract: Resource 引用相同")
					} else {
						m.logger.Warn("🔍 [DEBUG] DeployContract: Resource 引用不同，可能是复制导致")
					}
					if resource.ExecutionConfig != nil {
						if contract, ok := resource.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
							m.logger.Info("🔍 [DEBUG] DeployContract: 交易构建后，Output中的Resource包含ExecutionConfig",
								zap.Int("output_index", i),
								zap.String("abi_version", contract.Contract.AbiVersion),
								zap.Int("functions_count", len(contract.Contract.ExportedFunctions)),
							)
						} else {
							m.logger.Warn("🔍 [DEBUG] DeployContract: 交易构建后，ExecutionConfig类型不匹配",
								zap.Int("output_index", i),
								zap.String("type", fmt.Sprintf("%T", resource.ExecutionConfig)),
							)
						}
					} else {
						m.logger.Error("🔍 [DEBUG] DeployContract: 交易构建后，Output中的Resource.ExecutionConfig为nil",
							zap.Int("output_index", i),
							zap.String("content_hash", hex.EncodeToString(resource.ContentHash)),
						)
						// 检查 contractResource 是否还有 ExecutionConfig
						if contractResource.ExecutionConfig != nil {
							m.logger.Error("🔍 [DEBUG] DeployContract: 但 contractResource 仍有 ExecutionConfig，说明在设置到 ResourceOutput 时丢失了")
						}
					}
				}
			}
		}
	}

	// ========== 12. 计算交易哈希（使用统一的gRPC哈希服务）==========
	// ⚠️ 重要：必须使用 txHashClient，确保与交易池、区块处理的哈希计算一致
	txHashResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{
		Transaction: transaction,
	})
	if err != nil || txHashResp == nil || !txHashResp.IsValid {
		m.logger.Error("计算交易哈希失败", zap.Error(err))
		return nil, fmt.Errorf("compute transaction hash: %w", err)
	}

	txHash := txHashResp.Hash
	m.logger.Info("✅ 交易哈希计算完成（gRPC服务）", zap.String("tx_hash", hex.EncodeToString(txHash)))

	// ========== 13. 签名交易 ==========
	signature, err := ecdsacrypto.Sign(txHash, privateKey)
	if err != nil {
		m.logger.Error("签名交易失败", zap.Error(err))
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 移除recovery ID（最后一个字节），使用64字节签名
	signature64 := signature[:64]
	normalizedSignature := normalizeSignature(signature64)

	m.logger.Info("✅ 交易签名完成", zap.Int("signature_length", len(normalizedSignature)))

	// ========== 14. 提交交易到内存池 ==========
	// 注意：合约部署交易没有输入，所以不需要解锁证明

	// 🔍 调试日志：提交前再次检查
	if len(transaction.Outputs) > 0 {
		for i, output := range transaction.Outputs {
			if output != nil {
				if resourceOutput := output.GetResource(); resourceOutput != nil && resourceOutput.Resource != nil {
					resource := resourceOutput.Resource
					if resource.ExecutionConfig == nil {
						m.logger.Error("🔍 [DEBUG] DeployContract: 提交前检查，Resource.ExecutionConfig为nil",
							zap.Int("output_index", i),
							zap.String("content_hash", hex.EncodeToString(resource.ContentHash)),
						)
					}
				}
			}
		}
	}

	txHash2, err := m.mempool.SubmitTx(transaction)
	if err != nil {
		m.logger.Error("提交交易到内存池失败", zap.Error(err))
		return nil, fmt.Errorf("submit transaction: %w", err)
	}

	// 调试日志：帮助确认 TxPool 实例与区块构建使用的实例是否一致
	if m.logger != nil {
		m.logger.Info("✅ AI模型部署交易已提交到内存池",
			zap.String("tx_hash_hex", hex.EncodeToString(txHash2)),
			zap.String("mempool_ptr", fmt.Sprintf("%p", m.mempool)),
		)
	}

	// 注意：txHash2 是内存池返回的txHash，可用于验证，但当前不使用
	if txHash2 != nil {
		m.logger.Debug("内存池返回的交易哈希", zap.String("tx_hash", hex.EncodeToString(txHash2)))
	}

	m.logger.Info("✅ 交易已提交到内存池")

	// ========== 16. 返回结果 ==========
	txHashHex := hex.EncodeToString(txHash[:])

	m.logger.Info("🎉 智能合约部署完成！",
		zap.String("content_hash", contentHashHex),
		zap.String("tx_hash", txHashHex),
		zap.String("contract_address", hex.EncodeToString(contractAddrBytes)),
	)

	return map[string]interface{}{
		"content_hash":     contentHashHex,
		"contract_address": hex.EncodeToString(contractAddrBytes),
		"tx_hash":          txHashHex,
		"success":          true,
		"message":          "合约部署成功，交易已提交到内存池",
	}, nil
}

// CallContract 调用智能合约 (wes_callContract)
//
// 🎯 **功能**：调用已部署的智能合约方法（链上执行）
//
// 📋 **参数**（JSON格式）：
//
//	{
//	  "private_key": "十六进制私钥",
//	  "content_hash": "合约ID（64位十六进制）",
//	  "method": "方法名",
//	  "params": [100, 200],  // u64数组
//	  "payload": "base64编码的额外数据（可选）"
//	}
//
// 📋 **返回**（JSON格式）：
//
//	{
//	  "tx_hash": "交易哈希",
//	  "results": [300],  // 返回值（u64数组）
//	  "return_data": "base64编码的返回数据",
//	  "events": [...],
//	  "success": true,
//	  "message": "调用成功"
//	}
func (m *TxMethods) CallContract(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("📞 [wes_callContract] 开始处理合约调用请求")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		PrivateKey       string   `json:"private_key"` // 可选：如果 return_unsigned_tx=true 则不需要
		ContentHash      string   `json:"content_hash"`
		Method           string   `json:"method"`
		Params           []uint64 `json:"params"`
		Payload          string   `json:"payload"`            // Base64编码
		ReturnUnsignedTx bool     `json:"return_unsigned_tx"` // 可选：如果为 true，返回未签名交易
	}

	// 尝试解析数组格式：[{...}]
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		// 成功解析为数组，取第一个元素
		paramsBytes, err := json.Marshal(paramsArray[0])
		if err != nil {
			m.logger.Error("序列化参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("marshal params object: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &req); err != nil {
			m.logger.Error("解析参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params object: %w", err)
		}
	} else {
		// 尝试直接解析为对象：{...}
		if err := json.Unmarshal(params, &req); err != nil {
			m.logger.Error("解析参数失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if !req.ReturnUnsignedTx && req.PrivateKey == "" {
		return nil, fmt.Errorf("private_key is required when return_unsigned_tx is false")
	}
	if req.ContentHash == "" {
		return nil, fmt.Errorf("content_hash is required")
	}
	if req.Method == "" {
		return nil, fmt.Errorf("method is required")
	}

	m.logger.Info("🔍 [DEBUG] 收到合约调用参数",
		zap.String("content_hash", req.ContentHash),
		zap.String("method", req.Method),
		zap.Int("params_count", len(req.Params)),
	)

	// ========== 1. 解码contentHash ==========
	contentHash, err := hex.DecodeString(strings.TrimPrefix(req.ContentHash, "0x"))
	if err != nil {
		m.logger.Error("解码contentHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode content hash: %w", err)
	}

	if len(contentHash) != 32 {
		m.logger.Error("无效的contentHash长度", zap.Int("length", len(contentHash)))
		return nil, fmt.Errorf("invalid content hash length: expected 32, got %d", len(contentHash))
	}

	m.logger.Info("✅ contentHash解码成功")

	// ========== 2. 验证合约存在性 ==========
	resource, err := m.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil {
		m.logger.Error("查询合约资源失败", zap.Error(err))
		return nil, fmt.Errorf("query contract resource: %w", err)
	}

	if resource.Category != respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE ||
		resource.ExecutableType != respb.ExecutableType_EXECUTABLE_TYPE_CONTRACT {
		m.logger.Error("资源不是智能合约类型")
		return nil, fmt.Errorf("resource is not a contract")
	}

	m.logger.Info("✅ 合约验证通过", zap.String("name", resource.Name))

	// ========== 3. 从私钥推导调用者地址（如果需要签名）==========
	var privateKey *ecdsa.PrivateKey
	var callerAddrBytes []byte
	var callerAddrHex string
	var publicKey []byte
	var baseNonce []byte

	if !req.ReturnUnsignedTx {
		// 需要签名，必须提供私钥
		if req.PrivateKey == "" {
			return nil, fmt.Errorf("private_key is required when return_unsigned_tx is false")
		}
		privateKeyBytes, err := hex.DecodeString(strings.TrimPrefix(req.PrivateKey, "0x"))
		if err != nil {
			m.logger.Error("解码私钥失败", zap.Error(err))
			return nil, fmt.Errorf("decode private key: %w", err)
		}

		privateKey, err = ecdsacrypto.ToECDSA(privateKeyBytes)
		if err != nil {
			m.logger.Error("解析私钥失败", zap.Error(err))
			return nil, fmt.Errorf("parse private key: %w", err)
		}

		publicKey = ecdsacrypto.CompressPubkey(&privateKey.PublicKey)
		callerAddrBytes = hash160(publicKey)
		callerAddrHex = hex.EncodeToString(callerAddrBytes)
		if m.nonceManager != nil {
			baseNonce = m.nonceManager.Next(callerAddrBytes)
		}

		m.logger.Info("✅ 调用者地址推导完成", zap.String("address", callerAddrHex))
	} else {
		// 返回未签名交易，不需要私钥，但需要调用者地址（可以从参数中获取或使用零地址）
		// 注意：如果返回未签名交易，调用者地址应该在 SDK 层提供
		// 当前简化：使用零地址（SDK 层应该提供正确的调用者地址）
		callerAddrHex = "0000000000000000000000000000000000000000"
		var err error
		callerAddrBytes, err = hex.DecodeString(callerAddrHex)
		if err != nil {
			m.logger.Warn("解码调用者地址失败", zap.Error(err))
			callerAddrBytes = make([]byte, 20) // 使用零地址作为后备
		}
		m.logger.Info("⚠️  返回未签名交易模式，使用零地址作为调用者地址（SDK 层应提供正确地址）")
	}

	// ========== 4. 解码payload（可选）==========
	var payloadBytes []byte
	if req.Payload != "" {
		payloadBytes, err = base64.StdEncoding.DecodeString(req.Payload)
		if err != nil {
			m.logger.Error("解码payload失败", zap.Error(err))
			return nil, fmt.Errorf("decode payload: %w", err)
		}

		payloadBytes, err = m.normalizeContractAmount(req.Method, payloadBytes)
		if err != nil {
			m.logger.Error("规范化合约金额失败", zap.Error(err))
			return nil, fmt.Errorf("normalize amount: %w", err)
		}
	} else if strings.EqualFold(req.Method, "Mint") {
		defaultPayload := map[string]interface{}{
			"amount": defaultMintAmountWES,
		}
		defaultBytes, marshalErr := json.Marshal(defaultPayload)
		if marshalErr != nil {
			m.logger.Warn("默认铸币金额序列化失败", zap.Error(marshalErr))
		} else {
			normalized, normErr := m.normalizeContractAmount(req.Method, defaultBytes)
			if normErr != nil {
				m.logger.Warn("规范化默认铸币金额失败", zap.Error(normErr))
			} else {
				payloadBytes = normalized
				m.logger.Info("⚙️ 自动填充默认铸币金额",
					zap.String("method", req.Method),
					zap.String("amount_wes", defaultMintAmountWES),
				)
			}
		}
	}

	// ========== 5. 调用ISPC执行引擎（同步执行合约）==========
	m.logger.Info("🚀 调用ISPC执行引擎", zap.String("method", req.Method))

	// 检查ISPC协调器是否可用
	if m.ispcCoordinator == nil {
		m.logger.Error("❌ ISPC协调器未初始化")
		return nil, fmt.Errorf("ISPC coordinator is not initialized")
	}

	m.logger.Info("✅ ISPC协调器状态正常")

	// ISPC期望的调用者地址格式（直接使用hex字符串）
	callerAddrStr := callerAddrHex

	m.logger.Info("📞 准备调用ExecuteWASMContract",
		zap.String("contentHash", hex.EncodeToString(contentHash)),
		zap.String("method", req.Method),
		zap.Int("params_count", len(req.Params)),
		zap.Int("payload_size", len(payloadBytes)),
		zap.String("caller", callerAddrHex),
	)

	executionResult, err := m.ispcCoordinator.ExecuteWASMContract(
		ctx,
		contentHash,
		req.Method,
		req.Params,
		payloadBytes,
		callerAddrStr,
	)
	if err != nil {
		m.logger.Error("❌ ISPC执行合约失败（详细）",
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.String("error_msg", err.Error()),
		)
		return nil, fmt.Errorf("execute contract: %w", err)
	}

	m.logger.Info("✅ ISPC执行成功",
		zap.Int("return_values_count", len(executionResult.ReturnValues)),
		zap.Int("return_data_size", len(executionResult.ReturnData)),
		zap.Int("events_count", len(executionResult.Events)),
	)

	// ========== 6. 使用统一执行资源交易构建器（包含StateOutput + ResourceInput）==========
	stateOutput := executionResult.StateOutput
	if stateOutput == nil {
		m.logger.Error("StateOutput为空")
		return nil, fmt.Errorf("state output is nil")
	}
	if stateOutput.ZkProof == nil {
		m.logger.Error("ZK证明为空")
		return nil, fmt.Errorf("zk proof is nil")
	}

	m.logger.Info("✅ StateOutput验证通过，包含ZK证明")

	// 统一构建执行资源调用交易（合约/模型/未来执行体共享）
	transaction, err := m.buildExecutionResourceTransaction(ctx, executionResult.DraftTransaction, stateOutput, contentHash, callerAddrBytes)
	if err != nil {
		m.logger.Error("构建执行资源调用交易失败", zap.Error(err))
		return nil, fmt.Errorf("build execution transaction: %w", err)
	}

	m.logger.Info("✅ 合约调用交易构建完成")

	// ========== 7.0 为引用输入补全 ExecutionProof（如果缺失）==========
	// 检查引用输入是否有 UnlockingProof，如果没有则创建 ExecutionProof
	if !req.ReturnUnsignedTx {
		if err := m.ensureExecutionProofForRefInputs(ctx, transaction, stateOutput, contentHash, req.Method, payloadBytes, callerAddrBytes); err != nil {
			m.logger.Error("为引用输入补全 ExecutionProof 失败", zap.Error(err))
			return nil, fmt.Errorf("ensure execution proof for ref inputs: %w", err)
		}
	}

	// ========== 7.1 补全 ExecutionProof 身份字段 ==========
	if !req.ReturnUnsignedTx {
		if err := m.populateExecutionProofIdentities(transaction, privateKey, publicKey, baseNonce); err != nil {
			m.logger.Error("补全 ExecutionProof 身份信息失败", zap.Error(err))
			return nil, fmt.Errorf("populate execution proof identities: %w", err)
		}
	}

	// ========== 8. 计算交易哈希（使用统一的gRPC哈希服务）==========
	// ⚠️ 重要：必须使用 txHashClient，确保与交易池、区块处理的哈希计算一致
	txHashResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{
		Transaction: transaction,
	})
	if err != nil || txHashResp == nil || !txHashResp.IsValid {
		m.logger.Error("计算交易哈希失败", zap.Error(err))
		return nil, fmt.Errorf("compute transaction hash: %w", err)
	}

	txHash := txHashResp.Hash
	m.logger.Info("✅ 交易哈希计算完成（gRPC服务）", zap.String("tx_hash", hex.EncodeToString(txHash)))

	if validateResp, err := m.txHashCli.ValidateHash(ctx, &txpb.ValidateHashRequest{
		Transaction:  transaction,
		ExpectedHash: txHash,
	}); err != nil {
		m.logger.Warn("交易哈希验证请求失败", zap.Error(err))
	} else if !validateResp.IsValid {
		m.logger.Warn("交易哈希自检失败",
			zap.String("expected", hex.EncodeToString(txHash)),
			zap.String("computed", hex.EncodeToString(validateResp.GetComputedHash())),
		)
	}

	// ========== 9. 如果 return_unsigned_tx=true，返回未签名交易 ==========
	if req.ReturnUnsignedTx {
		// 序列化未签名交易
		txBytes, err := proto.Marshal(transaction)
		if err != nil {
			m.logger.Error("序列化交易失败", zap.Error(err))
			return nil, fmt.Errorf("marshal transaction: %w", err)
		}
		unsignedTxHex := hex.EncodeToString(txBytes)
		txHashHex := format.HashToHex(txHash)

		m.logger.Info("✅ 返回未签名交易", zap.String("tx_hash", txHashHex))

		return map[string]interface{}{
			"unsigned_tx": unsignedTxHex,
			"tx_hash":     txHashHex,
		}, nil
	}

	// ========== 10. 签名交易 ==========
	signature, err := ecdsacrypto.Sign(txHash, privateKey)
	if err != nil {
		m.logger.Error("签名交易失败", zap.Error(err))
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 移除recovery ID，使用64字节签名
	signature64 := signature[:64]
	normalizedSignature := normalizeSignature(signature64)

	// 对于无inputs的交易，签名信息存储在单独的字段中
	// 注意：当前简化实现，实际生产环境中StateOutput交易可能需要特殊处理
	m.logger.Info("✅ 交易签名完成", zap.Int("signature_bytes", len(normalizedSignature)))

	// ========== 11. 提交交易到内存池 ==========
	_, err = m.mempool.SubmitTx(transaction)
	if err != nil {
		m.logger.Error("提交交易到内存池失败", zap.Error(err))
		return nil, fmt.Errorf("submit transaction: %w", err)
	}

	m.logger.Info("✅ 合约调用交易已提交到内存池")

	// ========== 12. 返回完整执行结果（与旧CLI一致）==========
	txHashHex := hex.EncodeToString(txHash[:])

	// 转换事件格式
	events := make([]map[string]interface{}, 0, len(executionResult.Events))
	for _, evt := range executionResult.Events {
		if evt != nil {
			events = append(events, map[string]interface{}{
				"type":      evt.Type,
				"timestamp": evt.Timestamp,
				"data":      evt.Data,
			})
		}
	}

	// 编码ReturnData为Base64
	returnDataBase64 := ""
	if len(executionResult.ReturnData) > 0 {
		returnDataBase64 = base64.StdEncoding.EncodeToString(executionResult.ReturnData)
	}

	m.logger.Info("🎉 智能合约调用完成！",
		zap.String("tx_hash", txHashHex),
		zap.String("method", req.Method),
		zap.Int("results_count", len(executionResult.ReturnValues)),
	)

	return map[string]interface{}{
		"tx_hash":     txHashHex,
		"results":     executionResult.ReturnValues, // WASM函数返回值
		"return_data": returnDataBase64,             // 业务返回数据（Base64编码）
		"events":      events,                       // 事件列表
		"success":     true,
		"message":     fmt.Sprintf("合约调用成功，方法：%s", req.Method),
	}, nil
}

// GetContract 查询合约元数据 (wes_getContract)
//
// 🎯 **功能**：查询已部署合约的元数据（名称、版本、导出函数等）
//
// 📋 **参数**（JSON格式）：
//
//	{
//	  "content_hash": "合约ID（64位十六进制）"
//	}
//
// 📋 **返回**（JSON格式）：
//
//	{
//	  "content_hash": "合约ID",
//	  "name": "合约名称",
//	  "version": "1.0",
//	  "abi_version": "v1",
//	  "exported_functions": ["add", "sub", ...],
//	  "description": "合约描述",
//	  "size": 12345,
//	  "mime_type": "application/wasm",
//	  "creation_time": 1234567890,
//	  "owner": "部署者地址",
//	  "success": true
//	}
func (m *TxMethods) GetContract(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔍 [wes_getContract] 开始处理合约查询请求")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		ContentHash string `json:"content_hash"`
	}

	// 尝试解析数组格式：[{...}]
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		// 成功解析为数组，取第一个元素
		paramsBytes, err := json.Marshal(paramsArray[0])
		if err != nil {
			m.logger.Error("序列化参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("marshal params object: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &req); err != nil {
			m.logger.Error("解析参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params object: %w", err)
		}
	} else {
		// 尝试直接解析为对象：{...}
		if err := json.Unmarshal(params, &req); err != nil {
			m.logger.Error("解析参数失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if req.ContentHash == "" {
		return nil, fmt.Errorf("content_hash is required")
	}

	m.logger.Info("🔍 [DEBUG] 查询合约",
		zap.String("content_hash", req.ContentHash),
	)

	// ========== 1. 解码contentHash ==========
	contentHash, err := hex.DecodeString(strings.TrimPrefix(req.ContentHash, "0x"))
	if err != nil {
		m.logger.Error("解码contentHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode content hash: %w", err)
	}

	if len(contentHash) != 32 {
		m.logger.Error("无效的contentHash长度", zap.Int("length", len(contentHash)))
		return nil, fmt.Errorf("invalid content hash length: expected 32, got %d", len(contentHash))
	}

	m.logger.Info("✅ contentHash解码成功", zap.String("content_hash_hex", hex.EncodeToString(contentHash)))

	// ========== 2. 从区块链查询Resource ==========
	resource, err := m.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil {
		m.logger.Error("查询资源失败", zap.Error(err))
		return nil, fmt.Errorf("query resource: %w", err)
	}

	m.logger.Info("✅ 资源查询成功", zap.String("name", resource.Name))

	// ========== 3. 验证是否为Contract类型 ==========
	if resource.Category != respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE ||
		resource.ExecutableType != respb.ExecutableType_EXECUTABLE_TYPE_CONTRACT {
		m.logger.Error("资源不是智能合约类型",
			zap.String("category", resource.Category.String()),
			zap.String("executable_type", resource.ExecutableType.String()),
		)
		return nil, fmt.Errorf("resource is not a contract: category=%s, type=%s",
			resource.Category.String(), resource.ExecutableType.String())
	}

	// ========== 4. 提取Contract执行配置 ==========
	contractConfig, ok := resource.ExecutionConfig.(*respb.Resource_Contract)
	if !ok || contractConfig.Contract == nil {
		m.logger.Error("资源缺少合约执行配置")
		return nil, fmt.Errorf("resource missing contract execution config")
	}

	m.logger.Info("✅ 合约类型验证通过")

	// ========== 5. 返回完整元数据 ==========
	return map[string]interface{}{
		"content_hash":       hex.EncodeToString(resource.ContentHash),
		"name":               resource.Name,
		"version":            resource.Version,
		"description":        resource.Description,
		"mime_type":          resource.MimeType,
		"size":               resource.Size,
		"abi_version":        contractConfig.Contract.AbiVersion,
		"exported_functions": contractConfig.Contract.ExportedFunctions,
		"created_timestamp":  resource.CreatedTimestamp,
		"creator_address":    resource.CreatorAddress,
		"original_filename":  resource.OriginalFilename,
		"file_extension":     resource.FileExtension,
		"custom_attributes":  resource.CustomAttributes,
		"execution_params":   contractConfig.Contract.ExecutionParams,
		"success":            true,
	}, nil
}

// buildResourceMetadata 将链上 Resource 对象转换为统一的资源元数据映射
//
// ⚠️ 注意：
// - 字段命名需与 `internal/api/docs/jsonrpc_resource_metadata.md` 中描述保持一致
// - 如需为 SDK / 前端提供额外便捷字段，可以在不破坏兼容性的前提下新增键
func (m *TxMethods) buildResourceMetadata(resource *respb.Resource) map[string]interface{} {
	if resource == nil {
		return map[string]interface{}{}
	}

	// 规范化 resourceType（兼容 SDK 设计）
	var resourceType string
	if resource.Category == respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE {
		if resource.ExecutableType == respb.ExecutableType_EXECUTABLE_TYPE_CONTRACT {
			resourceType = "contract"
		} else if resource.ExecutableType == respb.ExecutableType_EXECUTABLE_TYPE_AIMODEL {
			resourceType = "model"
		} else {
			resourceType = "static" // 其他可执行类型暂时归类为 static
		}
	} else {
		resourceType = "static"
	}

	// owner 字段：creator_address 的 hex 别名（无 0x 前缀，便于 SDK 解析）
	// 注意：CreatorAddress 是 string 类型，可能是 Base58 或 hex 格式
	// 这里直接使用原值，如果前端需要 hex 格式，可以在 SDK 层转换
	ownerHex := resource.CreatorAddress
	if len(ownerHex) > 0 && strings.HasPrefix(ownerHex, "0x") {
		ownerHex = strings.TrimPrefix(ownerHex, "0x")
	}

	return map[string]interface{}{
		"content_hash":      hex.EncodeToString(resource.ContentHash),
		"name":              resource.Name,
		"version":           resource.Version,
		"description":       resource.Description,
		"category":          resource.Category.String(),
		"executable_type":   resource.ExecutableType.String(),
		"mime_type":         resource.MimeType,
		"size":              resource.Size,
		"created_timestamp": resource.CreatedTimestamp,
		"creator_address":   resource.CreatorAddress,
		"original_filename": resource.OriginalFilename,
		"file_extension":    resource.FileExtension,
		"custom_attributes": resource.CustomAttributes,
		// ✅ 兼容 SDK 设计的便捷字段（不改变既有字段语义）
		"resourceType": resourceType, // 规范化资源类型 'contract' | 'model' | 'static'
		"owner":        ownerHex,     // 与 creator_address 等价的别名（hex 字符串，无 0x 前缀）
	}
}

// GetResourceByContentHash 通用 Resource 查询 (wes_getResourceByContentHash)
//
// 用途：根据 content_hash 查询任意资源（AI 模型 / 合约 / 其他），返回基础元数据。
//
// 支持的参数格式：
//  1. ["<content_hash_hex>"]
//  2. [{"content_hash": "<content_hash_hex>"}]
//  3. {"content_hash": "<content_hash_hex>"}
func (m *TxMethods) GetResourceByContentHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔍 [wes_getResourceByContentHash] 开始处理资源查询请求")

	// 解析参数：优先按数组处理，再退回到对象格式
	var contentHashHex string

	// 尝试解析为字符串数组：["hash"]
	var strArgs []string
	if err := json.Unmarshal(params, &strArgs); err == nil && len(strArgs) > 0 {
		contentHashHex = strArgs[0]
	} else {
		// 尝试解析为对象或对象数组
		var objArgs []map[string]interface{}
		if err := json.Unmarshal(params, &objArgs); err == nil && len(objArgs) > 0 {
			if v, ok := objArgs[0]["content_hash"].(string); ok {
				contentHashHex = v
			}
		}
		if contentHashHex == "" {
			var obj struct {
				ContentHash string `json:"content_hash"`
			}
			if err := json.Unmarshal(params, &obj); err == nil {
				contentHashHex = obj.ContentHash
			}
		}
	}

	if contentHashHex == "" {
		return nil, fmt.Errorf("content_hash is required")
	}

	m.logger.Info("🔍 [DEBUG] 查询资源",
		zap.String("content_hash", contentHashHex),
	)

	// 解码 content_hash
	rawHash, err := hex.DecodeString(strings.TrimPrefix(contentHashHex, "0x"))
	if err != nil {
		m.logger.Error("解码contentHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode content hash: %w", err)
	}
	if len(rawHash) != 32 {
		m.logger.Error("无效的contentHash长度", zap.Int("length", len(rawHash)))
		return nil, fmt.Errorf("invalid content hash length: expected 32, got %d", len(rawHash))
	}

	resource, err := m.resourceQuery.GetResourceByContentHash(ctx, rawHash)
	if err != nil {
		m.logger.Error("查询资源失败", zap.Error(err))
		return nil, fmt.Errorf("query resource: %w", err)
	}

	m.logger.Info("✅ 资源查询成功",
		zap.String("name", resource.Name),
		zap.String("category", resource.Category.String()),
		zap.String("executable_type", resource.ExecutableType.String()),
	)

	// 返回通用资源元数据（避免直接暴露 protobuf 结构）
	resp := m.buildResourceMetadata(resource)
	resp["success"] = true
	return resp, nil
}

// GetResourceTransaction 查询资源对应的交易与区块信息 (wes_getResourceTransaction)
//
// 用途：根据 content_hash 查询资源首次出现的交易哈希、区块哈希与区块高度。
//
// 支持的参数格式：
//  1. ["<content_hash_hex>"]
//  2. [{"content_hash": "<content_hash_hex>"}]
//  3. {"content_hash": "<content_hash_hex>"}
func (m *TxMethods) GetResourceTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔍 [wes_getResourceTransaction] 开始处理资源交易查询请求")

	// 解析参数
	var contentHashHex string

	// 数组形式
	var strArgs []string
	if err := json.Unmarshal(params, &strArgs); err == nil && len(strArgs) > 0 {
		contentHashHex = strArgs[0]
	} else {
		// 对象数组形式
		var objArgs []map[string]interface{}
		if err := json.Unmarshal(params, &objArgs); err == nil && len(objArgs) > 0 {
			if v, ok := objArgs[0]["content_hash"].(string); ok {
				contentHashHex = v
			}
		}
		if contentHashHex == "" {
			// 单对象形式
			var obj struct {
				ContentHash string `json:"content_hash"`
			}
			if err := json.Unmarshal(params, &obj); err == nil {
				contentHashHex = obj.ContentHash
			}
		}
	}

	if contentHashHex == "" {
		return nil, fmt.Errorf("content_hash is required")
	}

	// 解码 content_hash
	rawHash, err := hex.DecodeString(strings.TrimPrefix(contentHashHex, "0x"))
	if err != nil {
		m.logger.Error("解码contentHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode content hash: %w", err)
	}
	if len(rawHash) != 32 {
		m.logger.Error("无效的contentHash长度", zap.Int("length", len(rawHash)))
		return nil, fmt.Errorf("invalid content hash length: expected 32, got %d", len(rawHash))
	}

	// 查询资源对应交易
	txHash, blockHash, blockHeight, err := m.resourceQuery.GetResourceTransaction(ctx, rawHash)
	if err != nil {
		m.logger.Error("查询资源交易失败", zap.Error(err))
		return nil, fmt.Errorf("query resource transaction: %w", err)
	}

	m.logger.Info("✅ 资源交易查询成功",
		zap.String("tx_hash", hex.EncodeToString(txHash)),
		zap.String("block_hash", hex.EncodeToString(blockHash)),
		zap.Uint64("block_height", blockHeight),
	)

	return map[string]interface{}{
		"content_hash": contentHashHex,
		"tx_hash":      hex.EncodeToString(txHash),
		"block_hash":   hex.EncodeToString(blockHash),
		"block_height": blockHeight,
		"success":      true,
	}, nil
}

// ListResources 列出资源列表（使用 ResourceViewService，基于 UTXO 视图）
//
// Method: wes_listResources
// 基于 ResourceViewService，返回完整的 ResourceView 数组
func (m *TxMethods) ListResources(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 如果 ResourceViewService 不可用，直接返回内部错误（不再回退到旧方法）
	if m.resourceViewService == nil {
		return nil, NewInternalError("ResourceViewService not available", nil)
	}

	// 解析过滤器
	type resourceFilters struct {
		ResourceType string   `json:"resourceType"`
		Owner        string   `json:"owner"`
		Status       string   `json:"status"`
		Tags         []string `json:"tags"`
		Limit        int      `json:"limit"`
		Offset       int      `json:"offset"`
	}
	var filters resourceFilters

	// 尝试解析为数组形式：[{"filters": {...}}]
	var arrayParams []struct {
		Filters resourceFilters `json:"filters"`
	}
	if err := json.Unmarshal(params, &arrayParams); err == nil && len(arrayParams) > 0 {
		filters = arrayParams[0].Filters
	} else {
		// 尝试解析为对象形式：{"filters": {...}}
		var objWithFilters struct {
			Filters resourceFilters `json:"filters"`
		}
		if err := json.Unmarshal(params, &objWithFilters); err == nil {
			filters = objWithFilters.Filters
		} else {
			// 尝试直接解析为 filters 对象
			json.Unmarshal(params, &filters)
		}
	}

	// 设置默认值
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	// 构建过滤条件
	viewFilter := resourcesvciface.ResourceViewFilter{}
	if filters.Owner != "" {
		ownerHex := strings.TrimPrefix(strings.TrimSpace(filters.Owner), "0x")
		ownerBytes, err := hex.DecodeString(ownerHex)
		if err == nil {
			viewFilter.Owner = ownerBytes
		}
	}
	if filters.Status != "" {
		status := filters.Status
		viewFilter.Status = &status
	}
	if filters.ResourceType != "" {
		// 映射 resourceType 到 category/executableType
		if filters.ResourceType == "contract" {
			category := "EXECUTABLE"
			execType := "CONTRACT"
			viewFilter.Category = &category
			viewFilter.ExecutableType = &execType
		} else if filters.ResourceType == "model" {
			category := "EXECUTABLE"
			execType := "AI_MODEL"
			viewFilter.Category = &category
			viewFilter.ExecutableType = &execType
		} else if filters.ResourceType == "static" {
			category := "STATIC"
			viewFilter.Category = &category
		}
	}
	if len(filters.Tags) > 0 {
		viewFilter.Tags = filters.Tags
	}

	// 调用 ResourceViewService
	views, pageResp, err := m.resourceViewService.ListResources(ctx, viewFilter, resourcesvciface.PageRequest{
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		m.logger.Error("ListResources 失败", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("list resources failed: %v", err), nil)
	}

	// 转换为 JSON 格式
	protojsonMarshaler := &protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	results := make([]map[string]interface{}, 0, len(views))
	for _, view := range views {
		// ✅ 严格要求 OutPoint 存在，避免 nil 指针
		if view.OutPoint == nil || len(view.OutPoint.TxId) == 0 {
			m.logger.Error("ResourceView 缺少 OutPoint，跳过该资源",
				zap.Binary("content_hash", view.ContentHash))
			continue
		}

		result := map[string]interface{}{
			"content_hash":    format.HashToHex(view.ContentHash),
			"category":        view.Category,
			"executable_type": view.ExecutableType,
			"mime_type":       view.MimeType,
			"size":            view.Size,
			"out_point": map[string]interface{}{
				"tx_id":        format.HashToHex(view.OutPoint.TxId),
				"output_index": view.OutPoint.OutputIndex,
			},
			"owner":                   format.MustAddressToBase58(view.Owner, m.addressManager),
			"status":                  view.Status,
			"creation_timestamp":      view.CreationTimestamp,
			"is_immutable":            view.IsImmutable,
			"current_reference_count": view.CurrentReferenceCount,
			"total_reference_times":   view.TotalReferenceTimes,
			"deploy_tx_id":            format.HashToHex(view.DeployTxId),
			"deploy_block_height":     view.DeployBlockHeight,
			"deploy_block_hash":       format.HashToHex(view.DeployBlockHash),
		}
		// ✅ 新增：添加可选字段
		if view.DeployTimestamp > 0 {
			result["deploy_timestamp"] = view.DeployTimestamp
		}
		if view.OriginalFilename != "" {
			result["original_filename"] = view.OriginalFilename
		}
		if view.FileExtension != "" {
			result["file_extension"] = view.FileExtension
		}
		if view.CreationContext != "" {
			result["creation_context"] = view.CreationContext
		}
		if view.DeployMemo != "" {
			result["deploy_memo"] = view.DeployMemo
		}
		if len(view.DeployTags) > 0 {
			result["deploy_tags"] = view.DeployTags
		}
		// ✅ 新增：序列化执行配置
		if view.ExecutionConfig != nil {
			var execConfigMap map[string]interface{}

			// 处理 Resource_Contract 类型
			if contract, ok := view.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
				execConfigJSON, err := protojsonMarshaler.Marshal(contract.Contract)
				if err == nil {
					if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
						result["executionConfig"] = map[string]interface{}{
							"contract": execConfigMap,
						}
					}
				}
			} else if aimodel, ok := view.ExecutionConfig.(*respb.Resource_Aimodel); ok && aimodel.Aimodel != nil {
				// 处理 Resource_Aimodel 类型
				execConfigJSON, err := protojsonMarshaler.Marshal(aimodel.Aimodel)
				if err == nil {
					if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
						result["executionConfig"] = map[string]interface{}{
							"aimodel": execConfigMap,
						}
					}
				}
			} else if protoMsg, ok := view.ExecutionConfig.(proto.Message); ok {
				// 其他类型直接序列化
				execConfigJSON, err := protojsonMarshaler.Marshal(protoMsg)
				if err == nil {
					if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
						result["executionConfig"] = execConfigMap
					}
				}
			}
		}
		if view.ExpiryTimestamp != nil {
			result["expiryTimestamp"] = *view.ExpiryTimestamp
		}
		// ✅ 新增：序列化锁定条件
		if len(view.LockingConditions) > 0 {
			lockingConditionsJSON := make([]map[string]interface{}, 0, len(view.LockingConditions))
			for _, lc := range view.LockingConditions {
				lcJSON, err := protojsonMarshaler.Marshal(lc)
				if err != nil {
					m.logger.Warn("序列化锁定条件失败", zap.Error(err))
					continue
				}
				var lcMap map[string]interface{}
				if err := json.Unmarshal(lcJSON, &lcMap); err != nil {
					m.logger.Warn("解析锁定条件 JSON 失败", zap.Error(err))
					continue
				}
				lockingConditionsJSON = append(lockingConditionsJSON, lcMap)
			}
			if len(lockingConditionsJSON) > 0 {
				result["lockingConditions"] = lockingConditionsJSON
			}
		}
		results = append(results, result)
	}

	m.logger.Info("✅ [wes_listResources] 资源列表查询完成",
		zap.Int("total", pageResp.Total),
		zap.Int("returned", len(results)),
	)

	return results, nil
}

// GetResource 获取单个资源（新版本，使用 ResourceViewService）
//
// Method: wes_getResource
// 基于 ResourceViewService.GetResource，返回单个 ResourceView
func (m *TxMethods) GetResource(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 如果 ResourceViewService 不可用，返回错误
	if m.resourceViewService == nil {
		return nil, NewInternalError("ResourceViewService not available", nil)
	}

	// 解析参数
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		// 尝试解析为对象格式
		var req struct {
			ResourceId string `json:"resourceId"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
		args = []string{req.ResourceId}
	}

	if len(args) == 0 {
		return nil, NewInvalidParamsError("resourceId required", nil)
	}

	resourceIdStr := args[0]
	resourceIdHex := strings.TrimPrefix(strings.TrimSpace(resourceIdStr), "0x")
	contentHash, err := hex.DecodeString(resourceIdHex)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid resourceId: %v", err), nil)
	}
	if len(contentHash) != 32 {
		return nil, NewInvalidParamsError("resourceId must be 32 bytes", nil)
	}

	// 调用 ResourceViewService
	view, err := m.resourceViewService.GetResource(ctx, contentHash)
	if err != nil {
		m.logger.Error("GetResource 失败", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("get resource failed: %v", err), nil)
	}

	// 转换为 JSON 格式
	protojsonMarshaler := &protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	result := map[string]interface{}{
		"content_hash":    format.HashToHex(view.ContentHash),
		"category":        view.Category,
		"executable_type": view.ExecutableType,
		"mime_type":       view.MimeType,
		"size":            view.Size,
		"out_point": map[string]interface{}{
			"tx_id":        format.HashToHex(view.OutPoint.TxId),
			"output_index": view.OutPoint.OutputIndex,
		},
		"owner":                   format.MustAddressToBase58(view.Owner, m.addressManager),
		"status":                  view.Status,
		"creation_timestamp":      view.CreationTimestamp,
		"is_immutable":            view.IsImmutable,
		"current_reference_count": view.CurrentReferenceCount,
		"total_reference_times":   view.TotalReferenceTimes,
		"deploy_tx_id":            format.HashToHex(view.DeployTxId),
		"deploy_block_height":     view.DeployBlockHeight,
		"deploy_block_hash":       format.HashToHex(view.DeployBlockHash),
	}
	if view.ExpiryTimestamp != nil {
		result["expiry_timestamp"] = *view.ExpiryTimestamp
	}
	// ✅ 新增：添加可选字段
	if view.DeployTimestamp > 0 {
		result["deploy_timestamp"] = view.DeployTimestamp
	}
	if view.OriginalFilename != "" {
		result["original_filename"] = view.OriginalFilename
	}
	if view.FileExtension != "" {
		result["file_extension"] = view.FileExtension
	}
	if view.CreationContext != "" {
		result["creation_context"] = view.CreationContext
	}
	if view.DeployMemo != "" {
		result["deploy_memo"] = view.DeployMemo
	}
	if len(view.DeployTags) > 0 {
		result["deploy_tags"] = view.DeployTags
	}
	// ✅ 新增：序列化执行配置
	if view.ExecutionConfig != nil {
		var execConfigMap map[string]interface{}

		// 处理 Resource_Contract 类型
		if contract, ok := view.ExecutionConfig.(*respb.Resource_Contract); ok && contract.Contract != nil {
			m.logger.Info("🔍 [DEBUG] GetResource RPC: ExecutionConfig 存在，开始序列化",
				zap.String("abi_version", contract.Contract.AbiVersion),
				zap.Int("functions_count", len(contract.Contract.ExportedFunctions)),
			)
			// Resource_Contract.Contract 是 ContractExecutionConfig，这才是 proto.Message
			execConfigJSON, err := protojsonMarshaler.Marshal(contract.Contract)
			if err == nil {
				if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
					// 包装为 contract 对象，匹配 oneof 结构
					result["executionConfig"] = map[string]interface{}{
						"contract": execConfigMap,
					}
					m.logger.Info("🔍 [DEBUG] GetResource RPC: ExecutionConfig 序列化成功",
						zap.String("abi_version", contract.Contract.AbiVersion),
						zap.Int("functions_count", len(contract.Contract.ExportedFunctions)),
					)
				} else {
					m.logger.Warn("🔍 [DEBUG] GetResource RPC: ExecutionConfig JSON 解析失败", zap.Error(err))
				}
			} else {
				m.logger.Warn("🔍 [DEBUG] GetResource RPC: ExecutionConfig protojson 序列化失败", zap.Error(err))
			}
		} else if aimodel, ok := view.ExecutionConfig.(*respb.Resource_Aimodel); ok && aimodel.Aimodel != nil {
			// 处理 Resource_Aimodel 类型
			execConfigJSON, err := protojsonMarshaler.Marshal(aimodel.Aimodel)
			if err == nil {
				if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
					result["executionConfig"] = map[string]interface{}{
						"aimodel": execConfigMap,
					}
				}
			}
		} else if protoMsg, ok := view.ExecutionConfig.(proto.Message); ok {
			// 其他类型直接序列化
			execConfigJSON, err := protojsonMarshaler.Marshal(protoMsg)
			if err == nil {
				if err := json.Unmarshal(execConfigJSON, &execConfigMap); err == nil {
					result["executionConfig"] = execConfigMap
				}
			}
		} else {
			m.logger.Warn("🔍 [DEBUG] GetResource RPC: ExecutionConfig 类型不支持序列化",
				zap.String("type", fmt.Sprintf("%T", view.ExecutionConfig)),
			)
		}
	} else {
		m.logger.Warn("🔍 [DEBUG] GetResource RPC: view.ExecutionConfig 为 nil",
			zap.String("content_hash", format.HashToHex(view.ContentHash)),
		)
	}
	// ✅ 新增：序列化锁定条件
	if len(view.LockingConditions) > 0 {
		lockingConditionsJSON := make([]map[string]interface{}, 0, len(view.LockingConditions))
		for _, lc := range view.LockingConditions {
			lcJSON, err := protojsonMarshaler.Marshal(lc)
			if err != nil {
				m.logger.Warn("序列化锁定条件失败", zap.Error(err))
				continue
			}
			var lcMap map[string]interface{}
			if err := json.Unmarshal(lcJSON, &lcMap); err != nil {
				m.logger.Warn("解析锁定条件 JSON 失败", zap.Error(err))
				continue
			}
			lockingConditionsJSON = append(lockingConditionsJSON, lcMap)
		}
		if len(lockingConditionsJSON) > 0 {
			result["lockingConditions"] = lockingConditionsJSON
		}
	}

	return result, nil
}

// GetResourceHistory 获取资源历史（新版本，使用 ResourceViewService）
//
// Method: wes_getResourceHistory
// 基于 ResourceViewService.GetResourceHistory，返回资源历史记录
func (m *TxMethods) GetResourceHistory(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 如果 ResourceViewService 不可用，返回错误
	if m.resourceViewService == nil {
		return nil, NewInternalError("ResourceViewService not available", nil)
	}

	// 解析参数
	var req struct {
		ResourceId string `json:"resourceId"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}

	// 尝试解析为数组格式
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		paramsBytes, _ := json.Marshal(paramsArray[0])
		json.Unmarshal(paramsBytes, &req)
	} else {
		// 尝试直接解析为对象
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
	}

	if req.ResourceId == "" {
		return nil, NewInvalidParamsError("resourceId required", nil)
	}

	resourceIdHex := strings.TrimPrefix(strings.TrimSpace(req.ResourceId), "0x")
	contentHash, err := hex.DecodeString(resourceIdHex)
	if err != nil {
		return nil, NewInvalidParamsError(fmt.Sprintf("invalid resourceId: %v", err), nil)
	}
	if len(contentHash) != 32 {
		return nil, NewInvalidParamsError("resourceId must be 32 bytes", nil)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// 调用 ResourceViewService
	history, err := m.resourceViewService.GetResourceHistory(ctx, contentHash, resourcesvciface.PageRequest{
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		m.logger.Error("GetResourceHistory 失败", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("get resource history failed: %v", err), nil)
	}

	// 转换为 JSON 格式
	result := map[string]interface{}{}

	// 部署交易
	if history.DeployTx != nil {
		result["deploy_tx"] = map[string]interface{}{
			"tx_id":        format.HashToHex(history.DeployTx.TxId),
			"block_hash":   format.HashToHex(history.DeployTx.BlockHash),
			"block_height": history.DeployTx.BlockHeight,
			"timestamp":    history.DeployTx.Timestamp,
		}
	}

	// 升级交易
	upgrades := make([]map[string]interface{}, 0, len(history.Upgrades))
	for _, upgrade := range history.Upgrades {
		upgradeMap := map[string]interface{}{
			"tx_id":        format.HashToHex(upgrade.TxId),
			"block_height": upgrade.BlockHeight,
			"timestamp":    upgrade.Timestamp,
		}
		if len(upgrade.BlockHash) > 0 {
			upgradeMap["block_hash"] = format.HashToHex(upgrade.BlockHash)
		}
		upgrades = append(upgrades, upgradeMap)
	}
	result["upgrades"] = upgrades

	// ✅ 新增：引用交易列表
	references := make([]map[string]interface{}, 0, len(history.References))
	for _, ref := range history.References {
		refMap := map[string]interface{}{
			"tx_id":        format.HashToHex(ref.TxId),
			"block_height": ref.BlockHeight,
			"timestamp":    ref.Timestamp,
		}
		if len(ref.BlockHash) > 0 {
			refMap["block_hash"] = format.HashToHex(ref.BlockHash)
		}
		references = append(references, refMap)
	}
	result["references"] = references

	// 引用统计
	if history.ReferencesSummary != nil {
		result["references_summary"] = map[string]interface{}{
			"total_references":    history.ReferencesSummary.TotalReferences,
			"unique_callers":      history.ReferencesSummary.UniqueCallers,
			"last_reference_time": history.ReferencesSummary.LastReferenceTime,
		}
	}

	return result, nil
}

// GetResourceCode 获取资源代码/字节码 (wes_getResourceCode)
//
// 📋 **方法说明**：
// 根据 resource_id (txId:outputIndex) 或 content_hash 获取资源的代码/字节码。
//
// 📥 **请求参数**（支持多种格式）：
//  1. {"resource_id": "txId:outputIndex", "code_type": "wasm"}
//  2. {"content_hash": "0xabc...", "code_type": "wasm"}
//  3. [{"resource_id": "txId:outputIndex", "code_type": "wasm"}]
//
// 📤 **返回结果**：
//
//	{
//	  "code_type": "wasm",
//	  "content": "0x0061736d01000000...",  // 十六进制编码的字节码
//	  "size": 12345,
//	  "success": true
//	}
//
// ⚠️ **注意**：
//   - code_type="wasm": 返回 WASM 字节码（十六进制）
//   - code_type="source": 如果链上存储了源码，返回源码；否则返回错误
func (m *TxMethods) GetResourceCode(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔍 [wes_getResourceCode] 开始处理资源代码查询请求")

	// 解析参数
	var req struct {
		ResourceID  string `json:"resource_id"`
		ContentHash string `json:"content_hash"`
		CodeType    string `json:"code_type"` // "wasm" | "source"
	}

	// 尝试解析数组格式
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		paramsBytes, _ := json.Marshal(paramsArray[0])
		json.Unmarshal(paramsBytes, &req)
	} else {
		// 尝试直接解析为对象
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if req.ResourceID == "" && req.ContentHash == "" {
		return nil, fmt.Errorf("resource_id or content_hash is required")
	}
	if req.CodeType == "" {
		req.CodeType = "wasm" // 默认返回 WASM
	}

	var contentHash []byte
	var err error

	// 如果提供了 resource_id，先查询 UTXO 获取 content_hash
	if req.ResourceID != "" {
		parts := strings.Split(req.ResourceID, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid resource_id format, expected txId:outputIndex")
		}
		txId := parts[0]
		outputIndex, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid output_index: %w", err)
		}

		// 查询 UTXO
		txIdBytes, err := hex.DecodeString(strings.TrimPrefix(txId, "0x"))
		if err != nil {
			return nil, fmt.Errorf("decode tx_id: %w", err)
		}

		outPoint := &txpb.OutPoint{
			TxId:        txIdBytes,
			OutputIndex: uint32(outputIndex),
		}

		utxo, err := m.utxoQuery.GetUTXO(ctx, outPoint)
		if err != nil {
			return nil, fmt.Errorf("get utxo: %w", err)
		}

		if utxo == nil {
			return nil, fmt.Errorf("utxo not found")
		}

		// 从 UTXO 提取 content_hash
		cachedOutput := utxo.GetCachedOutput()
		if cachedOutput == nil {
			return nil, fmt.Errorf("utxo output not cached")
		}

		resourceOutput := cachedOutput.GetResource()
		if resourceOutput == nil || resourceOutput.Resource == nil {
			return nil, fmt.Errorf("utxo does not contain a resource")
		}
		contentHash = resourceOutput.Resource.ContentHash
	} else {
		// 直接使用 content_hash
		contentHash, err = hex.DecodeString(strings.TrimPrefix(req.ContentHash, "0x"))
		if err != nil {
			return nil, fmt.Errorf("decode content_hash: %w", err)
		}
		if len(contentHash) != 32 {
			return nil, fmt.Errorf("invalid content_hash length: expected 32, got %d", len(contentHash))
		}
	}

	// 从 CAS 获取文件内容
	codeBytes, err := m.uresCAS.ReadFile(ctx, contentHash)
	if err != nil {
		m.logger.Error("获取资源代码失败", zap.Error(err))
		return nil, fmt.Errorf("get resource code: %w", err)
	}

	if req.CodeType == "source" {
		// 源码通常不上链，返回错误
		return nil, fmt.Errorf("source code is not stored on-chain, only WASM bytecode is available")
	}

	// 返回十六进制编码的字节码（不带 0x 前缀）
	return map[string]interface{}{
		"code_type": req.CodeType,
		"content":   hex.EncodeToString(codeBytes),
		"size":      len(codeBytes),
		"success":   true,
	}, nil
}

// GetResourceABI 获取资源 ABI (wes_getResourceABI)
//
// 📋 **方法说明**：
// 根据 resource_id (txId:outputIndex) 或 content_hash 获取资源的 ABI（应用二进制接口）。
//
// 📥 **请求参数**（支持多种格式）：
//  1. {"resource_id": "txId:outputIndex"}
//  2. {"content_hash": "0xabc..."}
//  3. [{"resource_id": "txId:outputIndex"}]
//
// 📤 **返回结果**：
//
//	{
//	  "abi_version": "v1",
//	  "methods": [
//	    {
//	      "name": "transfer",
//	      "type": "write",
//	      "parameters": [
//	        {"name": "to", "type": "string"},
//	        {"name": "amount", "type": "uint64"}
//	      ],
//	      "return_type": "void"
//	    }
//	  ],
//	  "success": true
//	}
func (m *TxMethods) GetResourceABI(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔍 [wes_getResourceABI] 开始处理资源 ABI 查询请求")

	// 解析参数
	var req struct {
		ResourceID  string `json:"resource_id"`
		ContentHash string `json:"content_hash"`
	}

	// 尝试解析数组格式
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		paramsBytes, _ := json.Marshal(paramsArray[0])
		json.Unmarshal(paramsBytes, &req)
	} else {
		// 尝试直接解析为对象
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if req.ResourceID == "" && req.ContentHash == "" {
		return nil, fmt.Errorf("resource_id or content_hash is required")
	}

	var contentHash []byte
	var err error

	// 如果提供了 resource_id，先查询 UTXO 获取 content_hash
	if req.ResourceID != "" {
		parts := strings.Split(req.ResourceID, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid resource_id format, expected txId:outputIndex")
		}
		txId := parts[0]
		outputIndex, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid output_index: %w", err)
		}

		// 查询 UTXO
		txIdBytes, err := hex.DecodeString(strings.TrimPrefix(txId, "0x"))
		if err != nil {
			return nil, fmt.Errorf("decode tx_id: %w", err)
		}

		outPoint := &txpb.OutPoint{
			TxId:        txIdBytes,
			OutputIndex: uint32(outputIndex),
		}

		utxo, err := m.utxoQuery.GetUTXO(ctx, outPoint)
		if err != nil {
			return nil, fmt.Errorf("get utxo: %w", err)
		}

		if utxo == nil {
			return nil, fmt.Errorf("utxo not found")
		}

		// 从 UTXO 提取 content_hash
		cachedOutput := utxo.GetCachedOutput()
		if cachedOutput == nil {
			return nil, fmt.Errorf("utxo output not cached")
		}

		resourceOutput := cachedOutput.GetResource()
		if resourceOutput == nil || resourceOutput.Resource == nil {
			return nil, fmt.Errorf("utxo does not contain a resource")
		}
		contentHash = resourceOutput.Resource.ContentHash
	} else {
		// 直接使用 content_hash
		contentHash, err = hex.DecodeString(strings.TrimPrefix(req.ContentHash, "0x"))
		if err != nil {
			return nil, fmt.Errorf("decode content_hash: %w", err)
		}
		if len(contentHash) != 32 {
			return nil, fmt.Errorf("invalid content_hash length: expected 32, got %d", len(contentHash))
		}
	}

	// 查询资源
	resource, err := m.resourceQuery.GetResourceByContentHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("query resource: %w", err)
	}

	// 检查是否为合约类型
	if resource.Category != respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE ||
		resource.ExecutableType != respb.ExecutableType_EXECUTABLE_TYPE_CONTRACT {
		return nil, fmt.Errorf("resource is not a contract")
	}

	// 提取合约执行配置
	contractConfig, ok := resource.ExecutionConfig.(*respb.Resource_Contract)
	if !ok || contractConfig.Contract == nil {
		return nil, fmt.Errorf("resource missing contract execution config")
	}

	// 构建 ABI 响应
	// 注意：当前节点只存储了 exported_functions，完整的 ABI 需要从合约模板或链下获取
	methods := make([]map[string]interface{}, 0)
	for _, funcName := range contractConfig.Contract.ExportedFunctions {
		methods = append(methods, map[string]interface{}{
			"name":        funcName,
			"type":        "write", // 默认类型，实际类型需要从完整 ABI 获取
			"parameters":  []interface{}{},
			"return_type": "void",
		})
	}

	return map[string]interface{}{
		"abi_version": contractConfig.Contract.AbiVersion,
		"methods":     methods,
		"success":     true,
	}, nil
}

// GetPricingState 查询资源定价状态 (wes_getPricingState)
//
// 📋 **方法说明**：
// 根据资源内容哈希查询资源的定价策略（计费模式、支付代币、CU 单价等）。
//
// 📥 **请求参数**（支持多种格式）：
//  1. "resource_hash_hex"（字符串）
//  2. ["resource_hash_hex"]（字符串数组）
//  3. {"resource_hash": "resource_hash_hex"}（对象）
//  4. [{"resource_hash": "resource_hash_hex"}]（对象数组）
//
// 📤 **返回结果**：
//
//	{
//	  "resource_hash": "hex_string",
//	  "owner_address": "hex_string",
//	  "billing_mode": "FREE|FIXED|CU_BASED",
//	  "payment_tokens": [
//	    {
//	      "token_id": "",                         // 为空字符串表示“原生代币”
//	      "cu_price": "1000000000000000"
//	    }
//	  ],
//	  "fixed_fee": "0",  // 仅 FIXED 模式
//	  "free_until": 0,   // 可选
//	  "success": true,
//	  "message": "定价状态查询成功"
//	}
func (m *TxMethods) GetPricingState(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("💰 [wes_getPricingState] 开始处理定价状态查询请求")

	// 解析参数：优先按数组处理，再退回到对象格式
	var resourceHashHex string

	// 尝试解析为字符串数组：["hash"]
	var strArgs []string
	if err := json.Unmarshal(params, &strArgs); err == nil && len(strArgs) > 0 {
		resourceHashHex = strArgs[0]
	} else {
		// 尝试解析为对象或对象数组
		var objArgs []map[string]interface{}
		if err := json.Unmarshal(params, &objArgs); err == nil && len(objArgs) > 0 {
			if v, ok := objArgs[0]["resource_hash"].(string); ok {
				resourceHashHex = v
			}
		}
		if resourceHashHex == "" {
			var obj struct {
				ResourceHash string `json:"resource_hash"`
			}
			if err := json.Unmarshal(params, &obj); err == nil {
				resourceHashHex = obj.ResourceHash
			}
		}
	}

	if resourceHashHex == "" {
		return nil, fmt.Errorf("resource_hash is required")
	}

	m.logger.Info("🔍 [DEBUG] 查询定价状态",
		zap.String("resource_hash", resourceHashHex),
	)

	// 解码 resource_hash
	rawHash, err := hex.DecodeString(strings.TrimPrefix(resourceHashHex, "0x"))
	if err != nil {
		m.logger.Error("解码resourceHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode resource hash: %w", err)
	}
	if len(rawHash) != 32 {
		m.logger.Error("无效的resourceHash长度", zap.Int("length", len(rawHash)))
		return nil, fmt.Errorf("invalid resource hash length: expected 32, got %d", len(rawHash))
	}

	// 查询定价状态
	pricingStateInterface, err := m.pricingQuery.GetPricingState(ctx, rawHash)
	if err != nil {
		m.logger.Error("查询定价状态失败", zap.Error(err))
		return nil, fmt.Errorf("query pricing state: %w", err)
	}

	// pricingState 已经是 *pkgtypes.ResourcePricingState 类型（接口返回具体类型）
	pricingState := pricingStateInterface

	// 构建返回结果
	result := map[string]interface{}{
		"resource_hash": resourceHashHex,
		"owner_address": hex.EncodeToString(pricingState.OwnerAddress),
		"billing_mode":  pricingState.BillingMode.String(),
		"success":       true,
		"message":       "定价状态查询成功",
	}

	// 根据计费模式添加相应字段
	switch pricingState.BillingMode {
	case pkgtypes.BillingModeCUBASED:
		// CU_BASED 模式：返回支付代币列表
		paymentTokens := make([]map[string]interface{}, 0, len(pricingState.PaymentTokens))
		for _, token := range pricingState.PaymentTokens {
			cuPrice, exists := pricingState.GetCUPrice(token.TokenID)
			if !exists {
				continue
			}
			paymentTokens = append(paymentTokens, map[string]interface{}{
				"token_id": string(token.TokenID),
				"cu_price": cuPrice.String(),
			})
		}
		result["payment_tokens"] = paymentTokens

	case pkgtypes.BillingModeFIXED:
		// FIXED 模式：返回固定费用
		// Phase 2: 固定费用字段等待 billing 模块完善，这里先返回 "0" 作为占位
		result["fixed_fee"] = "0"

	case pkgtypes.BillingModeFREE:
		// FREE 模式：无需额外字段
	}

	// Phase 2: 免费期限字段等待 billing 模块暴露，这里暂不返回

	m.logger.Info("✅ 定价状态查询成功",
		zap.String("billing_mode", pricingState.BillingMode.String()),
		zap.Int("payment_tokens", len(pricingState.PaymentTokens)),
	)

	return result, nil
}

// EstimateComputeFee 预估计算费用 (wes_estimateComputeFee)
//
// 📋 **方法说明**：
// 根据资源哈希和输入参数，预估执行所需的 CU 和费用。
//
// 📥 **请求参数**（Token 表示规则与 ResourcePricingState 一致）：
//
//	{
//	  "resource_hash": "hex_string",
//	  "inputs": [...],  // 与 CallAIModel 相同的输入格式
//	  "payment_token": ""           // 可选，指定支付代币：
//	                               //   - ""     表示原生代币（默认）
//	                               //   - 40hex 表示合约代币合约地址
//	}
//
// 📤 **返回结果**：
//
//	{
//	  "resource_hash": "hex_string",
//	  "estimated_cu": 123.45,
//	  "estimated_fee": "1000000000000000",
//	  "payment_token": "",          // 同上："" = 原生代币，40hex = 合约地址
//	  "billing_mode": "CU_BASED",
//	  "success": true
//	}
func (m *TxMethods) EstimateComputeFee(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("💰 [wes_estimateComputeFee] 开始处理费用预估请求")

	// 解析参数
	var req struct {
		ResourceHash string                   `json:"resource_hash"`
		Inputs       []map[string]interface{} `json:"inputs"`
		PaymentToken string                   `json:"payment_token,omitempty"`
	}

	// 尝试解析为数组格式
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		paramsBytes, _ := json.Marshal(paramsArray[0])
		json.Unmarshal(paramsBytes, &req)
	} else {
		json.Unmarshal(params, &req)
	}

	if req.ResourceHash == "" {
		return nil, fmt.Errorf("resource_hash is required")
	}

	// 解码 resource_hash
	modelHash, err := hex.DecodeString(strings.TrimPrefix(req.ResourceHash, "0x"))
	if err != nil || len(modelHash) != 32 {
		return nil, fmt.Errorf("invalid resource_hash")
	}

	// 预估输入大小（基于 inputs）
	estimatedInputSizeBytes := uint64(0)
	for _, inputMap := range req.Inputs {
		if shapeArray, ok := inputMap["shape"].([]interface{}); ok {
			elements := uint64(1)
			for _, val := range shapeArray {
				if sVal, ok := val.(float64); ok {
					elements *= uint64(sVal)
				}
			}
			dataType := "float32"
			if dt, ok := inputMap["data_type"].(string); ok {
				dataType = dt
			}
			bytesPerElement := uint64(4)
			if dataType == "float64" || dataType == "int64" {
				bytesPerElement = 8
			} else if dataType == "uint8" {
				bytesPerElement = 1
			}
			estimatedInputSizeBytes += elements * bytesPerElement
		}
	}

	// 预估 CU：使用与 ComputeMeter 相同的完整公式
	// 公式：base_cu + (input_size_bytes / 1024) * input_factor + (exec_time_ms / 100) * time_factor
	// 预估阶段：使用 base_cu + input_contribution（执行时间未知，使用 0）
	baseCU := 2.0 // AI 模型基础 CU
	inputFactor := 0.1
	inputContribution := (float64(estimatedInputSizeBytes) / 1024.0) * inputFactor
	estimatedCU := baseCU + inputContribution

	// 生成预估计费计划（直接使用 GenerateBillingPlan，它会内部查询定价状态）
	billingOrchestrator := billing.NewDefaultBillingOrchestrator(m.pricingQuery)
	estimatedPlan, err := billingOrchestrator.GenerateBillingPlan(ctx, modelHash, estimatedCU, req.PaymentToken)
	if err != nil {
		return nil, fmt.Errorf("生成预估计费计划失败: %w", err)
	}

	// 构建返回结果
	result := map[string]interface{}{
		"resource_hash": req.ResourceHash,
		"estimated_cu":  estimatedCU,
		"estimated_fee": estimatedPlan.FeeAmount.String(),
		"payment_token": estimatedPlan.PaymentToken,
		"billing_mode":  estimatedPlan.BillingMode.String(),
		"owner_address": hex.EncodeToString(estimatedPlan.OwnerAddress),
		"success":       true,
		"message":       "费用预估成功",
	}

	m.logger.Info("✅ 费用预估完成",
		zap.Float64("estimated_cu", estimatedCU),
		zap.String("estimated_fee", estimatedPlan.FeeAmount.String()),
	)

	return result, nil
}

// BuildTransaction 构建未签名交易（通用交易构建 API）
// Method: wes_buildTransaction
// Params: [draft: object]
// draft: JSON 格式的交易草稿（参考 host_build_transaction 的 DraftJSON 格式）
// 返回：未签名交易（hex编码）和交易哈希
//
// **架构说明**：
// - 这是一个通用的交易构建 API，不包含业务语义
// - SDK 层可以使用此 API 构建 Burn、BatchTransfer 等交易
// - 交易草稿格式与 host_build_transaction 的 DraftJSON 格式一致
func (m *TxMethods) BuildTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔨 [wes_buildTransaction] 开始构建交易")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		Draft json.RawMessage `json:"draft"` // 交易草稿（JSON格式）
	}

	// 尝试解析为数组格式
	var args []interface{}
	if err := json.Unmarshal(params, &args); err == nil && len(args) > 0 {
		// 数组格式：[{draft: {...}}]
		if draftMap, ok := args[0].(map[string]interface{}); ok {
			draftBytes, err := json.Marshal(draftMap)
			if err != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("marshal draft map: %v", err), nil)
			}
			req.Draft = draftBytes
		}
	} else {
		// 对象格式：{draft: {...}}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
	}

	if len(req.Draft) == 0 {
		return nil, NewInvalidParamsError("draft is required", nil)
	}

	// 获取当前区块高度和时间戳（用于交易构建）
	height, blockHash, err := m.blockQuery.GetHighestBlock(ctx)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to get current block: %v", err), nil)
	}

	var blockTimestamp uint64
	if block, err := m.blockQuery.GetBlockByHash(ctx, blockHash); err == nil && block != nil && block.Header != nil {
		blockTimestamp = block.Header.Timestamp
	} else {
		// 如果获取失败，使用当前时间戳
		blockTimestamp = uint64(time.Now().Unix())
	}

	// 检查必要的依赖是否已注入
	if m.draftService == nil || m.txAdapter == nil || m.selectorService == nil {
		return nil, NewInternalError("transaction building services not available: "+
			"draftService, txAdapter, or selectorService is nil", nil)
	}

	// 从 draft 中提取 callerAddress 和 contractAddress（如果存在）
	// 简化：如果没有 callerAddress，使用零地址（SDK 层应该提供正确的调用者地址）
	var callerAddress []byte
	var contractAddress []byte
	var draftMap map[string]interface{}
	if err := json.Unmarshal(req.Draft, &draftMap); err == nil {
		// 尝试从 draft 的 metadata 中提取 callerAddress
		if metadata, ok := draftMap["metadata"].(map[string]interface{}); ok {
			if callerStr, ok := metadata["caller_address"].(string); ok {
				callerBytes, err := hex.DecodeString(strings.TrimPrefix(callerStr, "0x"))
				if err == nil && len(callerBytes) == 20 {
					callerAddress = callerBytes
				}
			}
			// 尝试从 draft 的 metadata 中提取 contractAddress（用于合约代币输出）
			if contractStr, ok := metadata["contract_address"].(string); ok {
				contractBytes, err := hex.DecodeString(strings.TrimPrefix(contractStr, "0x"))
				if err == nil && len(contractBytes) == 20 {
					contractAddress = contractBytes
				}
			}
		}
		// 如果 metadata 中没有 contractAddress，尝试从 outputs 中提取（检查是否有合约代币输出）
		if len(contractAddress) == 0 {
			if outputs, ok := draftMap["outputs"].([]interface{}); ok {
				for _, output := range outputs {
					if outputMap, ok := output.(map[string]interface{}); ok {
						// 检查是否有 token_id（表示可能是合约代币）
						if tokenIDStr, hasTokenID := outputMap["token_id"].(string); hasTokenID && tokenIDStr != "" {
							// 如果有 token_id 但没有 contract_address，说明这是合约代币
							// 对于 wes_buildTransaction API，如果没有提供合约地址，使用零地址
							// 注意：这会导致 buildAssetOutput 返回错误，这是预期的行为
							// SDK 层应该确保在构建包含合约代币的 draft 时提供 contract_address
							contractAddress = make([]byte, 20) // 使用零地址作为占位符
							break
						}
					}
				}
			}
		}
	}
	// 如果没有找到 callerAddress，使用零地址
	if len(callerAddress) == 0 {
		callerAddress = make([]byte, 20)
	}
	// 如果没有找到 contractAddress，使用零地址（如果 draft 中有合约代币输出，buildAssetOutput 会返回错误）
	if len(contractAddress) == 0 {
		contractAddress = make([]byte, 20)
	}

	// 调用 BuildTransactionFromDraft 构建交易
	receipt, err := hostabi.BuildTransactionFromDraft(
		ctx,
		m.txAdapter,
		m.txHashCli,
		m.utxoQuery,
		callerAddress,
		contractAddress,
		req.Draft,
		height,
		blockTimestamp,
	)
	if err != nil {
		m.logger.Error("构建交易失败", zap.Error(err))
		return nil, NewInternalError(fmt.Sprintf("failed to build transaction: %v", err), nil)
	}

	// 检查是否有错误
	if receipt.Error != "" {
		return nil, NewInternalError(receipt.Error, nil)
	}

	// 返回未签名交易和交易哈希
	result := map[string]interface{}{
		"unsigned_tx": receipt.SerializedTx, // Base64 编码的序列化交易
		"tx_hash":     receipt.UnsignedTxHash,
	}

	// 如果 SerializedTx 是 Base64，需要转换为 hex
	// 检查 receipt.SerializedTx 的格式
	if receipt.SerializedTx != "" {
		// 尝试解码 Base64
		if txBytes, err := base64.StdEncoding.DecodeString(receipt.SerializedTx); err == nil {
			// 转换为 hex (不带 0x 前缀)
			result["unsigned_tx"] = hex.EncodeToString(txBytes)
		} else {
			// 如果解码失败，假设已经是 hex 格式
			txHex := receipt.SerializedTx
			// 移除可能存在的 0x 前缀
			if strings.HasPrefix(txHex, "0x") || strings.HasPrefix(txHex, "0X") {
				txHex = txHex[2:]
			}
			result["unsigned_tx"] = txHex
		}
	}

	m.logger.Info("✅ 交易构建成功",
		zap.String("tx_hash", receipt.UnsignedTxHash),
		zap.String("mode", receipt.Mode))

	return result, nil
}

// ComputeSignatureHashFromDraft 计算 Draft 生成的交易在指定输入上的签名哈希
//
// Method: wes_computeSignatureHashFromDraft
// Params:
//   - 对象格式：{draft: {...}, input_index: 0, sighash_type: "SIGHASH_ALL"}
//   - 或数组格式：[ {draft: {...}, input_index: 0, sighash_type: "SIGHASH_ALL"} ]
//
// 返回：
//   - {
//     "hash": "0x...",        // 待签名哈希
//     "unsignedTx": "0x..."   // 对应的未签名交易（protobuf字节，hex编码）
//     }
func (m *TxMethods) ComputeSignatureHashFromDraft(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🔐 [wes_computeSignatureHashFromDraft] 计算签名哈希")

	// 解析参数
	type reqBody struct {
		Draft       json.RawMessage `json:"draft"`
		InputIndex  *uint32         `json:"input_index,omitempty"`
		SighashType string          `json:"sighash_type,omitempty"`
	}

	var req reqBody

	// 兼容数组形式：[{...}]
	var args []map[string]interface{}
	if err := json.Unmarshal(params, &args); err == nil && len(args) > 0 {
		// 重新编码为对象，便于统一解析
		buf, err := json.Marshal(args[0])
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("marshal params failed: %v", err), nil)
		}
		if err := json.Unmarshal(buf, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
	} else {
		// 对象形式：{draft: {...}, ...}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params: %v", err), nil)
		}
	}

	if len(req.Draft) == 0 {
		return nil, NewInvalidParamsError("draft is required", nil)
	}

	// 构造仅包含 draft 的参数，复用 BuildTransaction 的 Draft 解析与构建逻辑
	buildParamsMap := map[string]json.RawMessage{
		"draft": req.Draft,
	}
	buildParamsBytes, err := json.Marshal(buildParamsMap)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("marshal draft params failed: %v", err), nil)
	}

	// 调用内部 BuildTransaction 逻辑构建未签名交易
	buildResult, err := m.BuildTransaction(ctx, buildParamsBytes)
	if err != nil {
		return nil, err
	}

	resultMap, ok := buildResult.(map[string]interface{})
	if !ok {
		return nil, NewInternalError("invalid response format from wes_buildTransaction", nil)
	}

	unsignedTxHex, ok := resultMap["unsignedTx"].(string)
	if !ok || unsignedTxHex == "" {
		return nil, NewInternalError("missing unsignedTx in wes_buildTransaction response", nil)
	}

	// 解码未签名交易
	rawHex := strings.TrimPrefix(unsignedTxHex, "0x")
	txBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("decode unsignedTx failed: %v", err), nil)
	}

	txObj := &txpb.Transaction{}
	if err := proto.Unmarshal(txBytes, txObj); err != nil {
		return nil, NewInternalError(fmt.Sprintf("unmarshal unsignedTx failed: %v", err), nil)
	}

	if m.txHashCli == nil {
		return nil, NewInternalError("transaction hash service not available", nil)
	}

	// 解析输入索引
	inputIndex := uint32(0)
	if req.InputIndex != nil {
		inputIndex = *req.InputIndex
	}
	if inputIndex >= uint32(len(txObj.Inputs)) {
		return nil, NewInvalidParamsError(fmt.Sprintf("input_index out of range: %d (len=%d)", inputIndex, len(txObj.Inputs)), nil)
	}

	// 解析签名类型
	sighashType := txpb.SignatureHashType_SIGHASH_ALL
	if req.SighashType != "" {
		switch strings.ToUpper(req.SighashType) {
		case "SIGHASH_ALL":
			sighashType = txpb.SignatureHashType_SIGHASH_ALL
		case "SIGHASH_SINGLE":
			sighashType = txpb.SignatureHashType_SIGHASH_SINGLE
		case "SIGHASH_NONE":
			sighashType = txpb.SignatureHashType_SIGHASH_NONE
		default:
			return nil, NewInvalidParamsError(fmt.Sprintf("unsupported sighash_type: %s", req.SighashType), nil)
		}
	}

	// 调用 TxHash 服务计算签名哈希
	sigHashResp, err := m.txHashCli.ComputeSignatureHash(ctx, &txpb.ComputeSignatureHashRequest{
		Transaction:      txObj,
		InputIndex:       inputIndex,
		SighashType:      sighashType,
		IncludeDebugInfo: false,
	})
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to compute signature hash: %v", err), nil)
	}
	if sigHashResp == nil || !sigHashResp.IsValid || len(sigHashResp.Hash) == 0 {
		return nil, NewInternalError("signature hash response is invalid", nil)
	}

	hashHex := format.HashToHex(sigHashResp.Hash)

	// 🔍 调试：输出与 TxHashService 一致的前缀，便于对齐
	var hashPrefix string
	if len(sigHashResp.Hash) >= 8 {
		hashPrefix = hex.EncodeToString(sigHashResp.Hash[:8])
	} else {
		hashPrefix = hex.EncodeToString(sigHashResp.Hash)
	}

	m.logger.Info("✅ [wes_computeSignatureHashFromDraft] 签名哈希计算完成",
		zap.Uint32("input_index", inputIndex),
		zap.String("sighash_type", sighashType.String()),
		zap.String("sig_hash_prefix", hashPrefix))

	return map[string]interface{}{
		"sig_hash":   hashHex,
		"unsignedTx": unsignedTxHex,
	}, nil
}

// GetTransactionHistory 查询交易历史 (wes_getTransactionHistory)
//
// 📋 方法说明：
//   - 提供按 txId 或 resourceId 查询相关交易的能力
//   - 当前实现为最小可用版本：
//   - 如果提供 txId：返回该笔交易的详细信息（数组包裹）
//   - 如果提供 resourceId：返回资源首次出现的部署交易
//   - 尚未支持“全网扫描”的无过滤查询
//
// 📥 请求参数（兼容多种格式）：
//  1. [{"filters": {"txId": "0x...", "limit": 1, "offset": 0}}]
//  2. [{"filters": {"resourceId": "0x<content_hash_hex>", "limit": 1, "offset": 0}}]
//  3. {"filters": {...}}
//  4. {"txId": "0x...", "limit": 1, "offset": 0}
//
// 📤 返回结果：
//   - 交易信息数组，每项字段与 `wes_getTransactionByHash` 一致
func (m *TxMethods) GetTransactionHistory(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// 0. 准备解析结构
	type txFilters struct {
		ResourceID string `json:"resourceId"`
		TxID       string `json:"txId"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	var filters txFilters

	// 1. 解析参数（支持多种包装形式）
	// 1.1 数组形式：[{"filters": {...}}]
	var arrayParams []struct {
		Filters txFilters `json:"filters"`
	}
	if err := json.Unmarshal(params, &arrayParams); err == nil && len(arrayParams) > 0 {
		filters = arrayParams[0].Filters
	} else {
		// 1.2 对象形式：{"filters": {...}}
		var objWithFilters struct {
			Filters txFilters `json:"filters"`
		}
		if err := json.Unmarshal(params, &objWithFilters); err == nil && (objWithFilters.Filters.TxID != "" || objWithFilters.Filters.ResourceID != "") {
			filters = objWithFilters.Filters
		} else {
			// 1.3 直接 filters 形式：{"txId": "...", "resourceId": "..."}
			var direct txFilters
			if err := json.Unmarshal(params, &direct); err == nil {
				filters = direct
			}
		}
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 1
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	m.logger.Info("🔍 [wes_getTransactionHistory] 开始处理交易历史查询请求",
		zap.String("txId", filters.TxID),
		zap.String("resourceId", filters.ResourceID),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	// 2. 至少需要 txId 或 resourceId 之一
	if filters.TxID == "" && filters.ResourceID == "" {
		return nil, NewInvalidParamsError("at least one of txId or resourceId is required", nil)
	}

	// 3. 如果提供 txId，则复用 GetTransactionByHash 的逻辑
	if filters.TxID != "" {
		args := []string{filters.TxID}
		argsBytes, _ := json.Marshal(args)

		txResp, err := m.GetTransactionByHash(ctx, argsBytes)
		if err != nil {
			return nil, err
		}
		if txResp == nil {
			// 找不到交易时返回空数组，而不是 null，便于前端处理
			return []interface{}{}, nil
		}

		return []interface{}{txResp}, nil
	}

	// 4. 如果提供 resourceId，则查找资源关联的部署交易
	if filters.ResourceID != "" {
		if m.resourceQuery == nil || m.txQuery == nil {
			return nil, NewInternalError("resource or transaction query not available", nil)
		}

		// 4.1 解析 resourceId（content_hash hex）
		resourceIDHex := strings.TrimSpace(filters.ResourceID)
		resourceIDHex = strings.TrimPrefix(resourceIDHex, "0x")
		rawHash, err := hex.DecodeString(resourceIDHex)
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid resourceId hex: %v", err), nil)
		}
		if len(rawHash) != 32 {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid resourceId length: expected 32 bytes, got %d", len(rawHash)), nil)
		}

		// 4.2 查询资源对应的部署交易
		txHash, blockHash, blockHeight, err := m.resourceQuery.GetResourceTransaction(ctx, rawHash)
		if err != nil {
			m.logger.Error("查询资源部署交易失败", zap.Error(err))
			return nil, NewInternalError(fmt.Sprintf("query resource transaction failed: %v", err), nil)
		}

		// 4.3 查询交易详情
		_, txIndex, transaction, err := m.txQuery.GetTransaction(ctx, txHash)
		if err != nil || transaction == nil {
			m.logger.Error("根据资源部署交易哈希查询交易失败",
				zap.Error(err),
			)
			return nil, NewInternalError("transaction not found for resource", nil)
		}

		// 4.4 格式化为与 wes_getTransactionByHash 一致的响应
		resp, err := m.formatTransactionResponse(ctx, transaction, blockHash, blockHeight, txIndex)
		if err != nil {
			return nil, err
		}

		return []interface{}{resp}, nil
	}

	// 理论上不会走到这里，防御性返回
	return []interface{}{}, nil
}

// FinalizeTransactionFromDraft 使用 SDK 提供的公钥和签名，为 Draft 生成的交易附加 SingleKeyProof 并返回可提交的交易
//
// Method: wes_finalizeTransactionFromDraft
// Params:
//   - 对象格式（单输入签名，向后兼容）：
//     {
//     "draft": {...},              // DraftJSON（可选，和 unsignedTx 至少提供一个）
//     "unsignedTx": "0x...",       // 未签名交易（可选，推荐使用）
//     "input_index": 0,
//     "sighash_type": "SIGHASH_ALL",
//     "pubkey": "0x...",
//     "signature": "0x..."
//     }
//   - 对象格式（多输入签名）：
//     {
//     "draft": {...},              // DraftJSON（可选，和 unsignedTx 至少提供一个）
//     "unsignedTx": "0x...",       // 未签名交易（可选，推荐使用）
//     "signatures": [               // 签名数组（如果提供，优先使用）
//     {
//     "input_index": 0,
//     "sighash_type": "SIGHASH_ALL",
//     "pubkey": "0x...",
//     "signature": "0x..."
//     },
//     ...
//     ]
//     }
//
// 返回：
//   - { "tx": "0x..." } // 可直接传给 wes_sendRawTransaction 的交易字节（十六进制编码）
func (m *TxMethods) FinalizeTransactionFromDraft(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🧩 [wes_finalizeTransactionFromDraft] 开始生成带 SingleKeyProof 的交易")

	type signatureItem struct {
		InputIndex  *uint32 `json:"input_index"`
		SighashType string  `json:"sighash_type,omitempty"`
		PubKeyHex   string  `json:"pubkey"`
		SigHex      string  `json:"signature"`
	}

	type reqBody struct {
		Draft         json.RawMessage `json:"draft"`
		UnsignedTxHex string          `json:"unsignedTx,omitempty"`
		InputIndex    *uint32         `json:"input_index,omitempty"`  // 单输入签名（向后兼容）
		SighashType   string          `json:"sighash_type,omitempty"` // 单输入签名（向后兼容）
		PubKeyHex     string          `json:"pubkey"`                 // 单输入签名（向后兼容）
		SigHex        string          `json:"signature"`              // 单输入签名（向后兼容）
		Signatures    []signatureItem `json:"signatures,omitempty"`   // 多输入签名（优先使用）
	}

	var req reqBody

	// 调试：打印原始参数（使用Debug级别）
	if m.logger != nil {
		m.logger.Debug("🔍 [wes_finalizeTransactionFromDraft] 原始参数",
			zap.String("params", string(params)))
	}

	// 兼容数组形式：[{...}] 和对象形式：{...}
	var args []interface{}
	var parseErr error
	if err := json.Unmarshal(params, &args); err == nil && len(args) > 0 {
		// 数组格式：[{...}]
		if draftMap, ok := args[0].(map[string]interface{}); ok {
			buf, err := json.Marshal(draftMap)
			if err != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("marshal params failed: %v", err), nil)
			}
			parseErr = json.Unmarshal(buf, &req)
			if parseErr != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("invalid params (array format): %v", parseErr), nil)
			}
		} else {
			return nil, NewInvalidParamsError("invalid params format: expected object in array", nil)
		}
	} else {
		// 对象格式：{...}
		parseErr = json.Unmarshal(params, &req)
		if parseErr != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid params (object format): %v", parseErr), nil)
		}
	}

	// 调试：打印解析后的参数（使用Debug级别）
	if m.logger != nil {
		signaturesJSON, _ := json.Marshal(req.Signatures)
		m.logger.Debug("🔍 [wes_finalizeTransactionFromDraft] 解析后的参数",
			zap.Int("signatures_count", len(req.Signatures)),
			zap.String("signatures", string(signaturesJSON)),
			zap.String("pubkey", req.PubKeyHex),
			zap.String("signature", req.SigHex))
	}

	if len(req.Draft) == 0 && req.UnsignedTxHex == "" {
		return nil, NewInvalidParamsError("either draft or unsignedTx is required", nil)
	}

	// 验证签名参数：要么提供 signatures 数组，要么提供单个签名（向后兼容）
	useMultiSig := len(req.Signatures) > 0

	if m.logger != nil {
		m.logger.Debug("🔍 [wes_finalizeTransactionFromDraft] 签名参数检查",
			zap.Bool("useMultiSig", useMultiSig),
			zap.Int("signatures_count", len(req.Signatures)),
			zap.String("pubkey", req.PubKeyHex),
			zap.String("signature", req.SigHex))
	}

	// 如果 signatures 数组为空，检查是否有单个签名参数
	if !useMultiSig {
		if req.PubKeyHex == "" || req.SigHex == "" {
			// 返回更详细的错误信息，帮助调试
			return nil, NewInvalidParamsError(fmt.Sprintf("either signatures array (got %d items) or single pubkey/signature is required. pubkey=%s, signature=%s",
				len(req.Signatures), req.PubKeyHex, req.SigHex), nil)
		}
	}

	var txObj *txpb.Transaction

	if req.UnsignedTxHex != "" {
		// 优先使用客户端提供的 unsignedTx，确保与签名哈希对应
		rawHex := strings.TrimPrefix(req.UnsignedTxHex, "0x")
		txBytes, err := hex.DecodeString(rawHex)
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid unsignedTx hex: %v", err), nil)
		}

		txObj = &txpb.Transaction{}
		if err := proto.Unmarshal(txBytes, txObj); err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("unmarshal unsignedTx failed: %v", err), nil)
		}
	} else {
		// 兼容旧用法：从 draft 重新构建交易
		buildParamsMap := map[string]json.RawMessage{
			"draft": req.Draft,
		}
		buildParamsBytes, err := json.Marshal(buildParamsMap)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("marshal draft params failed: %v", err), nil)
		}

		buildResult, err := m.BuildTransaction(ctx, buildParamsBytes)
		if err != nil {
			return nil, err
		}

		resultMap, ok := buildResult.(map[string]interface{})
		if !ok {
			return nil, NewInternalError("invalid response format from wes_buildTransaction", nil)
		}

		unsignedTxHex, ok := resultMap["unsignedTx"].(string)
		if !ok || unsignedTxHex == "" {
			return nil, NewInternalError("missing unsignedTx in wes_buildTransaction response", nil)
		}

		rawHex := strings.TrimPrefix(unsignedTxHex, "0x")
		txBytes, err := hex.DecodeString(rawHex)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("decode unsignedTx failed: %v", err), nil)
		}

		txObj = &txpb.Transaction{}
		if err := proto.Unmarshal(txBytes, txObj); err != nil {
			return nil, NewInternalError(fmt.Sprintf("unmarshal unsignedTx failed: %v", err), nil)
		}
	}

	// 辅助函数：解析签名类型
	parseSighashType := func(sighashTypeStr string) (txpb.SignatureHashType, error) {
		sighashType := txpb.SignatureHashType_SIGHASH_ALL
		if sighashTypeStr != "" {
			switch strings.ToUpper(sighashTypeStr) {
			case "SIGHASH_ALL":
				sighashType = txpb.SignatureHashType_SIGHASH_ALL
			case "SIGHASH_SINGLE":
				sighashType = txpb.SignatureHashType_SIGHASH_SINGLE
			case "SIGHASH_NONE":
				sighashType = txpb.SignatureHashType_SIGHASH_NONE
			default:
				return 0, fmt.Errorf("unsupported sighash_type: %s", sighashTypeStr)
			}
		}
		return sighashType, nil
	}

	// 辅助函数：为指定输入附加 SingleKeyProof
	attachSingleKeyProof := func(inputIndex uint32, pubKeyBytes []byte, sigBytes []byte, sighashType txpb.SignatureHashType) error {
		if inputIndex >= uint32(len(txObj.Inputs)) {
			return fmt.Errorf("input_index out of range: %d (len=%d)", inputIndex, len(txObj.Inputs))
		}
		if txObj.Inputs[inputIndex] == nil {
			txObj.Inputs[inputIndex] = &txpb.TxInput{}
		}
		txObj.Inputs[inputIndex].UnlockingProof = &txpb.TxInput_SingleKeyProof{
			SingleKeyProof: &txpb.SingleKeyProof{
				Signature: &txpb.SignatureData{
					Value: sigBytes,
				},
				PublicKey: &txpb.PublicKey{
					Value: pubKeyBytes,
				},
				Algorithm:   txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: sighashType,
			},
		}
		return nil
	}

	// 处理签名：多输入签名或单输入签名（向后兼容）
	if useMultiSig {
		// 多输入签名模式
		for _, sigItem := range req.Signatures {
			if sigItem.InputIndex == nil {
				return nil, NewInvalidParamsError("signature item missing input_index", nil)
			}
			inputIndex := *sigItem.InputIndex

			if sigItem.PubKeyHex == "" || sigItem.SigHex == "" {
				return nil, NewInvalidParamsError(fmt.Sprintf("signature for input %d missing pubkey or signature", inputIndex), nil)
			}

			pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(sigItem.PubKeyHex, "0x"))
			if err != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("invalid pubkey hex for input %d: %v", inputIndex, err), nil)
			}
			sigBytes, err := hex.DecodeString(strings.TrimPrefix(sigItem.SigHex, "0x"))
			if err != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("invalid signature hex for input %d: %v", inputIndex, err), nil)
			}

			sighashType, err := parseSighashType(sigItem.SighashType)
			if err != nil {
				return nil, NewInvalidParamsError(fmt.Sprintf("invalid sighash_type for input %d: %v", inputIndex, err), nil)
			}

			if err := attachSingleKeyProof(inputIndex, pubKeyBytes, sigBytes, sighashType); err != nil {
				return nil, NewInvalidParamsError(err.Error(), nil)
			}
		}
		m.logger.Info("✅ [wes_finalizeTransactionFromDraft] 多输入签名模式",
			zap.Int("signature_count", len(req.Signatures)))
	} else {
		// 单输入签名模式（向后兼容）
		inputIndex := uint32(0)
		if req.InputIndex != nil {
			inputIndex = *req.InputIndex
		}

		pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(req.PubKeyHex, "0x"))
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid pubkey hex: %v", err), nil)
		}
		sigBytes, err := hex.DecodeString(strings.TrimPrefix(req.SigHex, "0x"))
		if err != nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("invalid signature hex: %v", err), nil)
		}

		sighashType, err := parseSighashType(req.SighashType)
		if err != nil {
			return nil, NewInvalidParamsError(err.Error(), nil)
		}

		if err := attachSingleKeyProof(inputIndex, pubKeyBytes, sigBytes, sighashType); err != nil {
			return nil, NewInvalidParamsError(err.Error(), nil)
		}
		m.logger.Info("✅ [wes_finalizeTransactionFromDraft] 单输入签名模式",
			zap.Uint32("input_index", inputIndex))
	}

	// 重新序列化交易
	finalBytes, err := proto.Marshal(txObj)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("marshal finalized tx failed: %v", err), nil)
	}

	txHex := hex.EncodeToString(finalBytes)

	if useMultiSig {
		m.logger.Info("✅ [wes_finalizeTransactionFromDraft] 生成带 SingleKeyProof 的交易成功（多输入签名）",
			zap.Int("signature_count", len(req.Signatures)),
			zap.Int("tx_inputs", len(txObj.Inputs)),
			zap.Int("tx_outputs", len(txObj.Outputs)))
	} else {
		m.logger.Info("✅ [wes_finalizeTransactionFromDraft] 生成带 SingleKeyProof 的交易成功（单输入签名）",
			zap.Int("tx_inputs", len(txObj.Inputs)),
			zap.Int("tx_outputs", len(txObj.Outputs)))
	}

	return map[string]interface{}{
		"tx": txHex,
	}, nil
}

// ensureExecutionProofForRefInputs 确保所有引用输入都有 ExecutionProof
// 如果引用输入没有 UnlockingProof，创建一个最小化的 ExecutionProof（后续由 populateExecutionProofIdentities 补全）
func (m *TxMethods) ensureExecutionProofForRefInputs(
	ctx context.Context,
	tx *txpb.Transaction,
	stateOutput *txpb.StateOutput,
	resourceHash []byte,
	methodName string,
	inputParams []byte,
	callerAddrBytes []byte,
) error {
	if tx == nil || stateOutput == nil {
		return nil
	}

	// 推导合约地址（hash160(contentHash)）
	contractAddrBytes := hash160(resourceHash)
	if len(contractAddrBytes) != 20 {
		return fmt.Errorf("invalid contract address length: %d", len(contractAddrBytes))
	}

	// 计算输入数据哈希
	normalizedParams := inputParams
	if len(normalizedParams) == 0 {
		normalizedParams = []byte("[]")
	}
	inputDataHash := sha256.Sum256(normalizedParams)

	// 计算输出数据哈希（使用 execution_result_hash）
	var outputDataHash [32]byte
	if len(stateOutput.ExecutionResultHash) == 32 {
		copy(outputDataHash[:], stateOutput.ExecutionResultHash)
	} else {
		outputDataHash = sha256.Sum256([]byte(""))
	}

	// 从 ZKProof 中提取 state_transition_proof
	var stateTransitionProof []byte
	if stateOutput.ZkProof != nil && len(stateOutput.ZkProof.Proof) > 0 {
		stateTransitionProof = stateOutput.ZkProof.Proof
	} else {
		return fmt.Errorf("state_transition_proof is empty")
	}

	// 尝试从 ZKProof 中获取执行时间（证明生成时间作为参考）
	// 注意：proof_generation_time_ms 是证明生成时间，不是实际执行时间
	// 但可以作为参考值，实际执行时间通常 <= 证明生成时间
	var executionTimeMs uint64 = 1000 // 默认值：1秒（更保守的估计）
	if stateOutput.ZkProof != nil && stateOutput.ZkProof.ProofGenerationTimeMs != nil {
		executionTimeMs = *stateOutput.ZkProof.ProofGenerationTimeMs
		if m.logger != nil {
			m.logger.Debug("使用 ZKProof.ProofGenerationTimeMs 作为 execution_time_ms 参考值",
				zap.Uint64("execution_time_ms", executionTimeMs))
		}
	} else {
		if m.logger != nil {
			m.logger.Warn("无法获取实际执行时间，使用默认值 1000ms",
				zap.Uint64("default_execution_time_ms", executionTimeMs))
		}
	}

	// 遍历所有引用输入，为没有 UnlockingProof 的输入创建 ExecutionProof
	for idx, input := range tx.Inputs {
		if input == nil || !input.IsReferenceOnly {
			continue
		}

		// 如果已经有 UnlockingProof，跳过
		if input.UnlockingProof != nil {
			continue
		}

		// 创建占位符 IdentityProof（后续由 populateExecutionProofIdentities 补全）
		callerIdentity := &txpb.IdentityProof{
			CallerAddress: callerAddrBytes,
			Algorithm:     txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
			SighashType:   txpb.SignatureHashType_SIGHASH_ALL,
			// Signature, PublicKey, Nonce, Timestamp, ContextHash 将在 populateExecutionProofIdentities 中补全
		}

		// 创建 ExecutionContext
		execCtx := &txpb.ExecutionProof_ExecutionContext{
			CallerIdentity:  callerIdentity,
			ResourceAddress: contractAddrBytes,
			ExecutionType:   txpb.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   inputDataHash[:],
			OutputDataHash:  outputDataHash[:],
			Metadata: map[string][]byte{
				"method_name": []byte(methodName),
			},
		}

		// 计算 context_hash（用于后续签名）
		contextHash := m.computeExecutionContextHash(execCtx)
		callerIdentity.ContextHash = contextHash

		// 创建 ExecutionProof
		execProof := &txpb.ExecutionProof{
			ExecutionResultHash:  stateOutput.ExecutionResultHash,
			StateTransitionProof: stateTransitionProof,
			ExecutionTimeMs:      executionTimeMs, // 使用从 ZKProof 获取的时间或默认值
			Context:              execCtx,
		}

		// 设置到输入
		input.UnlockingProof = &txpb.TxInput_ExecutionProof{
			ExecutionProof: execProof,
		}

		if m.logger != nil {
			m.logger.Info("✅ 为引用输入创建 ExecutionProof",
				zap.Int("input_index", idx),
				zap.String("contract_address", hex.EncodeToString(contractAddrBytes)))
		}
	}

	return nil
}

// computeExecutionContextHash 计算 ExecutionContext 的哈希（用于 IdentityProof 签名）
func (m *TxMethods) computeExecutionContextHash(execCtx *txpb.ExecutionProof_ExecutionContext) []byte {
	var buf bytes.Buffer

	// 添加所有非敏感字段
	if len(execCtx.InputDataHash) == 32 {
		buf.Write(execCtx.InputDataHash)
	}
	if len(execCtx.OutputDataHash) == 32 {
		buf.Write(execCtx.OutputDataHash)
	}
	if len(execCtx.ResourceAddress) == 20 {
		buf.Write(execCtx.ResourceAddress)
	}

	// 添加 execution_type（4字节）
	execTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(execTypeBytes, uint32(execCtx.ExecutionType))
	buf.Write(execTypeBytes)

	// 添加 metadata（排序后添加，确保确定性）
	if len(execCtx.Metadata) > 0 {
		keys := make([]string, 0, len(execCtx.Metadata))
		for k := range execCtx.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString(k)
			buf.Write(execCtx.Metadata[k])
		}
	}

	// 计算SHA-256哈希
	hash := sha256.Sum256(buf.Bytes())
	return hash[:]
}

// populateExecutionProofIdentities 使用真实公钥/签名/nonce 补全所有 ExecutionProof
func (m *TxMethods) populateExecutionProofIdentities(
	tx *txpb.Transaction,
	privateKey *ecdsa.PrivateKey,
	publicKey []byte,
	baseNonce []byte,
) error {
	if tx == nil || privateKey == nil || len(publicKey) == 0 {
		return nil
	}
	if len(baseNonce) > 0 && len(baseNonce) != 32 {
		return fmt.Errorf("base nonce must be 32 bytes when provided, got %d", len(baseNonce))
	}

	now := uint64(time.Now().Unix())

	for idx, input := range tx.GetInputs() {
		proof := input.GetExecutionProof()
		if proof == nil || proof.Context == nil || proof.Context.CallerIdentity == nil {
			continue
		}

		identity := proof.Context.CallerIdentity
		if len(identity.ContextHash) != 32 {
			return fmt.Errorf("execution proof #%d missing valid context hash", idx)
		}

		sig, err := ecdsacrypto.Sign(identity.ContextHash, privateKey)
		if err != nil {
			return fmt.Errorf("sign context hash for input %d: %w", idx, err)
		}
		identity.Signature = append([]byte(nil), sig[:64]...)
		identity.PublicKey = append([]byte(nil), publicKey...)
		identity.Timestamp = now

		// Nonce：如果未提供 baseNonce，则使用 (publicKey || contextHash) 的 SHA256 作为 baseNonce（稳定且可复现）。
		// 说明：当前 verifier 侧仅检查 nonce 长度（唯一性校验尚未实现），但这里仍保证结构完整，避免“nonce 为空直接失败”。
		derivedBaseNonce := baseNonce
		if len(derivedBaseNonce) == 0 {
			h := sha256.Sum256(append(append([]byte(nil), publicKey...), identity.ContextHash...))
			derivedBaseNonce = h[:]
		}
		if len(derivedBaseNonce) == 32 {
			if nonce := deriveInputNonce(derivedBaseNonce, idx); len(nonce) == 32 {
				identity.Nonce = nonce
			}
		}
	}

	return nil
}

// CallAIModel 调用AI模型 (wes_callAIModel)
//
// 🎯 **功能**：调用已部署的AI模型进行推理（链上执行）
//
// 📋 **参数**（JSON格式）：
//
//	{
//	  "private_key": "0x...",          // 可选：如果 return_unsigned_tx=true 则不需要
//	  "model_hash": "0x...",           // 模型内容哈希（32字节hex）
//	  "inputs": [                      // 张量输入列表
//	    {
//	      "name": "input",             // 可选：输入名称
//	      "data": [1.0, 2.0, ...],     // float32类型数据（通过float64传递）
//	      "int64_data": [101, 2023],   // 可选：int64类型数据（用于文本模型）
//	      "uint8_data": [255, 128],    // 可选：uint8类型数据（用于图像原始数据）
//	      "shape": [1, 3, 224, 224],   // 形状信息（如 [1, 3, 224, 224]）
//	      "data_type": "float32"       // 可选：数据类型（"float32", "int64", "uint8"）
//	    }
//	  ],
//	  "return_unsigned_tx": false      // 可选：如果为 true，返回未签名交易
//	}
//
// 📋 **返回值**（JSON格式）：
//
//	{
//	  "success": true,
//	  "tx_hash": "0x...",              // 交易哈希
//	  "outputs": [[1.0, 2.0, ...]],   // 推理结果
//	  "message": "调用成功"
//	}
func (m *TxMethods) CallAIModel(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("🤖 [wes_callAIModel] 开始处理AI模型调用请求")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		PrivateKey       string                   `json:"private_key"`             // 可选：如果 return_unsigned_tx=true 则不需要
		ModelHash        string                   `json:"model_hash"`              // 模型内容哈希
		Inputs           []map[string]interface{} `json:"inputs"`                  // 张量输入列表
		ReturnUnsignedTx bool                     `json:"return_unsigned_tx"`      // 可选：如果为 true，返回未签名交易
		PaymentToken     string                   `json:"payment_token,omitempty"` // Phase 3: 指定支付代币（可选）
	}

	// 尝试解析数组格式：[{...}]
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		// 成功解析为数组，取第一个元素
		paramsBytes, err := json.Marshal(paramsArray[0])
		if err != nil {
			m.logger.Error("序列化参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("marshal params object: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &req); err != nil {
			m.logger.Error("解析参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params object: %w", err)
		}
	} else {
		// 尝试直接解析为对象：{...}
		if err := json.Unmarshal(params, &req); err != nil {
			m.logger.Error("解析参数失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if !req.ReturnUnsignedTx && req.PrivateKey == "" {
		return nil, fmt.Errorf("private_key is required when return_unsigned_tx is false")
	}
	if req.ModelHash == "" {
		return nil, fmt.Errorf("model_hash is required")
	}
	if len(req.Inputs) == 0 {
		return nil, fmt.Errorf("inputs is required and cannot be empty")
	}

	m.logger.Info("🔍 [DEBUG] 收到AI模型调用参数",
		zap.String("model_hash", req.ModelHash),
		zap.Int("inputs_count", len(req.Inputs)),
	)

	// ========== 1. 解码modelHash ==========
	modelHash, err := hex.DecodeString(strings.TrimPrefix(req.ModelHash, "0x"))
	if err != nil {
		m.logger.Error("解码modelHash失败", zap.Error(err))
		return nil, fmt.Errorf("decode model hash: %w", err)
	}

	if len(modelHash) != 32 {
		m.logger.Error("无效的modelHash长度", zap.Int("length", len(modelHash)))
		return nil, fmt.Errorf("invalid model hash length: expected 32, got %d", len(modelHash))
	}

	m.logger.Info("✅ modelHash解码成功")

	// ========== 2. 验证模型资源存在性 ==========
	resource, err := m.resourceQuery.GetResourceByContentHash(ctx, modelHash)
	if err != nil {
		m.logger.Error("查询模型资源失败", zap.Error(err))
		return nil, fmt.Errorf("query model resource: %w", err)
	}

	if resource.Category != respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE ||
		resource.ExecutableType != respb.ExecutableType_EXECUTABLE_TYPE_AIMODEL {
		m.logger.Error("资源不是AI模型类型")
		return nil, fmt.Errorf("resource is not an AI model")
	}

	m.logger.Info("✅ 模型验证通过", zap.String("name", resource.Name))

	// ========== 3. 解析张量输入 ==========
	tensorInputs := make([]ispc.TensorInput, 0, len(req.Inputs))
	for i, inputMap := range req.Inputs {
		tensorInput := ispc.TensorInput{}

		// 解析Name（可选）
		if name, ok := inputMap["name"].(string); ok {
			tensorInput.Name = name
		}

		// 解析Data（float64数组，用于float32类型）
		if dataArray, ok := inputMap["data"].([]interface{}); ok {
			tensorInput.Data = make([]float64, len(dataArray))
			for j, val := range dataArray {
				if fVal, ok := val.(float64); ok {
					tensorInput.Data[j] = fVal
				} else {
					return nil, fmt.Errorf("input[%d].data[%d] must be a number", i, j)
				}
			}
		}

		// 解析Int64Data（int64数组，用于int64类型）
		if int64Array, ok := inputMap["int64_data"].([]interface{}); ok {
			tensorInput.Int64Data = make([]int64, len(int64Array))
			for j, val := range int64Array {
				if iVal, ok := val.(float64); ok {
					tensorInput.Int64Data[j] = int64(iVal)
				} else {
					return nil, fmt.Errorf("input[%d].int64_data[%d] must be a number", i, j)
				}
			}
		}

		// 解析Int32Data（int32数组，用于int32类型）
		// 📚 官方实现参考: onnxruntime_test.go:396-397
		//    inputData := []int32{12, 21}
		//    input, e := NewTensor(NewShape(1, 2), inputData)
		//    直接使用 []int32 创建 *Tensor[int32]，无需类型转换
		if int32Array, ok := inputMap["int32_data"].([]interface{}); ok {
			tensorInput.Int32Data = make([]int32, len(int32Array))
			for j, val := range int32Array {
				if iVal, ok := val.(float64); ok {
					tensorInput.Int32Data[j] = int32(iVal)
				} else {
					return nil, fmt.Errorf("input[%d].int32_data[%d] must be a number", i, j)
				}
			}
		}

		// 解析Int16Data（int16数组，用于int16类型）
		// 📚 官方实现参考: onnxruntime_test.go:572
		//    outputA := newTestTensor[int16](t, NewShape(1, 2, 2))
		//    其中 newTestTensor[int16] 内部调用 NewEmptyTensor[int16](shape)
		//    对于输入，使用 NewTensor(shape, []int16{...}) 创建 *Tensor[int16]
		if int16Array, ok := inputMap["int16_data"].([]interface{}); ok {
			tensorInput.Int16Data = make([]int16, len(int16Array))
			for j, val := range int16Array {
				if iVal, ok := val.(float64); ok {
					tensorInput.Int16Data[j] = int16(iVal)
				} else {
					return nil, fmt.Errorf("input[%d].int16_data[%d] must be a number", i, j)
				}
			}
		}

		// 解析Uint8Data（uint8数组，用于uint8类型）
		if uint8Array, ok := inputMap["uint8_data"].([]interface{}); ok {
			tensorInput.Uint8Data = make([]uint8, len(uint8Array))
			for j, val := range uint8Array {
				if uVal, ok := val.(float64); ok {
					tensorInput.Uint8Data[j] = uint8(uVal)
				} else {
					return nil, fmt.Errorf("input[%d].uint8_data[%d] must be a number", i, j)
				}
			}
		}

		// 解析Shape（int64数组）
		if shapeArray, ok := inputMap["shape"].([]interface{}); ok {
			tensorInput.Shape = make([]int64, len(shapeArray))
			for j, val := range shapeArray {
				if sVal, ok := val.(float64); ok {
					tensorInput.Shape[j] = int64(sVal)
				} else {
					return nil, fmt.Errorf("input[%d].shape[%d] must be a number", i, j)
				}
			}
		}

		// 解析DataType（字符串）
		if dataType, ok := inputMap["data_type"].(string); ok {
			tensorInput.DataType = dataType
		}

		// 添加调试日志：检查解析后的数据字段
		m.logger.Debug("🔍 [DEBUG] 解析后的张量输入",
			zap.Int("index", i),
			zap.String("name", tensorInput.Name),
			zap.Int("data_len", len(tensorInput.Data)),
			zap.Int("int64_data_len", len(tensorInput.Int64Data)),
			zap.Int("int32_data_len", len(tensorInput.Int32Data)),
			zap.Int("int16_data_len", len(tensorInput.Int16Data)),
			zap.Int("uint8_data_len", len(tensorInput.Uint8Data)),
			zap.String("data_type", tensorInput.DataType),
		)

		tensorInputs = append(tensorInputs, tensorInput)
	}

	m.logger.Info("✅ 张量输入解析成功", zap.Int("tensor_count", len(tensorInputs)))

	// ========== 4. 从私钥推导调用者地址（如果需要签名）==========
	var privateKey *ecdsa.PrivateKey
	var callerAddrBytes []byte
	var callerAddrHex string

	if !req.ReturnUnsignedTx {
		// 需要签名，必须提供私钥
		if req.PrivateKey == "" {
			return nil, fmt.Errorf("private_key is required when return_unsigned_tx is false")
		}
		privateKeyBytes, err := hex.DecodeString(strings.TrimPrefix(req.PrivateKey, "0x"))
		if err != nil {
			m.logger.Error("解码私钥失败", zap.Error(err))
			return nil, fmt.Errorf("decode private key: %w", err)
		}

		privateKey, err = ecdsacrypto.ToECDSA(privateKeyBytes)
		if err != nil {
			m.logger.Error("解析私钥失败", zap.Error(err))
			return nil, fmt.Errorf("parse private key: %w", err)
		}

		// 从私钥推导公钥和地址（使用压缩公钥，与 signTransaction 保持一致）
		publicKey := privateKey.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			m.logger.Error("公钥类型转换失败")
			return nil, fmt.Errorf("public key type assertion failed")
		}

		// 使用压缩公钥计算地址（与 signTransaction 中的 hash160(CompressPubkey(...)) 一致）
		compressedPubKey := ecdsacrypto.CompressPubkey(publicKeyECDSA)
		callerAddrBytes = hash160(compressedPubKey)
		callerAddrHex = hex.EncodeToString(callerAddrBytes)

		m.logger.Info("✅ 调用者地址推导成功", zap.String("caller", callerAddrHex))
	}

	// ========== 4.5. Phase 3: 费用预估和校验（调用前）==========
	if !req.ReturnUnsignedTx && callerAddrBytes != nil {
		// 预估 CU：使用与 ComputeMeter 相同的完整公式
		estimatedInputSizeBytes := uint64(0)
		for _, ti := range tensorInputs {
			elements := uint64(1)
			for _, dim := range ti.Shape {
				elements *= uint64(dim)
			}
			bytesPerElement := uint64(4) // 默认 float32
			if ti.DataType == "float64" || ti.DataType == "int64" {
				bytesPerElement = 8
			} else if ti.DataType == "uint8" {
				bytesPerElement = 1
			}
			estimatedInputSizeBytes += elements * bytesPerElement
		}

		// 使用 ComputeMeter 的完整公式进行 CU 预估
		// 公式：base_cu + (input_size_bytes / 1024) * input_factor + (exec_time_ms / 100) * time_factor
		// 预估阶段：使用 base_cu + input_contribution（执行时间未知，使用 0）
		baseCU := 2.0 // AI 模型基础 CU
		inputFactor := 0.1
		inputContribution := (float64(estimatedInputSizeBytes) / 1024.0) * inputFactor
		estimatedCU := baseCU + inputContribution

		// 查询定价状态并预估费用
		pricingState, err := m.pricingQuery.GetPricingState(ctx, modelHash)
		if err == nil && pricingState != nil {
			// 定价状态存在，进行费用预估
			if !pricingState.IsFree() {
				// 生成预估计费计划（使用用户指定的支付代币）
				billingOrchestrator := billing.NewDefaultBillingOrchestrator(m.pricingQuery)
				// payment_token 规则：
				// - ""     表示原生代币
				// - 40hex 表示合约代币合约地址
				estimatedPlan, err := billingOrchestrator.GenerateBillingPlan(ctx, modelHash, estimatedCU, req.PaymentToken)
				if err == nil && estimatedPlan.FeeAmount.Sign() > 0 {
					// 检查余额：支持多 Token 余额检查
					var tokenIDBytes []byte
					if estimatedPlan.PaymentToken != "" {
						// 如果支付代币是合约地址格式（40字符十六进制），转换为字节；否则认为协议层无效
						if len(estimatedPlan.PaymentToken) == 40 {
							if tokenIDBytesDecoded, err := hex.DecodeString(estimatedPlan.PaymentToken); err == nil && len(tokenIDBytesDecoded) == 20 {
								tokenIDBytes = tokenIDBytesDecoded
							}
						}
					}

					balance, err := m.accountQuery.GetAccountBalance(ctx, callerAddrBytes, tokenIDBytes)
					if err == nil && balance != nil {
						// 根据支付代币类型检查余额
						// GetAccountBalance 返回的 BalanceInfo.Total 就是指定代币的余额
						balanceBigInt := new(big.Int).SetUint64(balance.Total)

						if balanceBigInt != nil {
							if balanceBigInt.Cmp(estimatedPlan.FeeAmount) < 0 {
								m.logger.Warn("💰 余额不足，预估费用",
									zap.String("estimated_fee", estimatedPlan.FeeAmount.String()),
									zap.String("balance", balanceBigInt.String()),
									zap.String("payment_token", estimatedPlan.PaymentToken),
									zap.Float64("estimated_cu", estimatedCU),
								)
								return nil, fmt.Errorf("余额不足：预估费用 %s %s，当前余额 %s（预估 CU: %.2f）",
									estimatedPlan.FeeAmount.String(), estimatedPlan.PaymentToken, balanceBigInt.String(), estimatedCU)
							}
							m.logger.Info("✅ 费用预估通过",
								zap.String("estimated_fee", estimatedPlan.FeeAmount.String()),
								zap.String("payment_token", estimatedPlan.PaymentToken),
								zap.Float64("estimated_cu", estimatedCU),
							)
						} else {
							m.logger.Warn("💰 无法获取指定代币余额，跳过余额检查",
								zap.String("payment_token", estimatedPlan.PaymentToken),
							)
						}
					}
				}
			}
		} else {
			// 定价状态不存在或查询失败，继续执行（可能是免费资源）
			m.logger.Debug("定价状态查询失败或不存在，跳过费用预估", zap.Error(err))
		}
	}

	// ========== 5. 调用ISPC执行引擎（同步执行AI模型）==========
	m.logger.Info("🚀 调用ISPC执行引擎执行AI模型推理")

	// 检查ISPC协调器是否可用
	if m.ispcCoordinator == nil {
		m.logger.Error("❌ ISPC协调器未初始化")
		return nil, fmt.Errorf("ISPC coordinator is not initialized")
	}

	m.logger.Info("✅ ISPC协调器状态正常")

	m.logger.Info("📞 准备调用ExecuteONNXModel",
		zap.String("modelHash", hex.EncodeToString(modelHash)),
		zap.Int("inputs_count", len(tensorInputs)),
		zap.String("caller", callerAddrHex),
	)

	executionResult, err := m.ispcCoordinator.ExecuteONNXModel(
		ctx,
		modelHash,
		tensorInputs,
	)
	if err != nil {
		m.logger.Error("❌ ISPC执行AI模型失败",
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.String("error_msg", err.Error()),
		)
		return nil, fmt.Errorf("execute AI model: %w", err)
	}

	m.logger.Info("✅ ISPC执行成功",
		zap.Int("outputs_count", len(executionResult.ReturnTensors)),
	)

	// ========== 6. 使用ISPC返回的StateOutput（包含ZK证明）==========
	stateOutput := executionResult.StateOutput
	if stateOutput == nil {
		m.logger.Error("StateOutput为空")
		return nil, fmt.Errorf("state output is nil")
	}
	if stateOutput.ZkProof == nil {
		m.logger.Error("ZK证明为空")
		return nil, fmt.Errorf("zk proof is nil")
	}

	m.logger.Info("✅ StateOutput验证通过，包含ZK证明")

	// ========== 7. 使用统一执行资源交易构建器构建 AI 模型调用交易 ==========
	transaction, err := m.buildExecutionResourceTransaction(ctx, nil, stateOutput, modelHash, callerAddrBytes)
	if err != nil {
		m.logger.Error("构建AI模型调用交易失败", zap.Error(err))
		return nil, fmt.Errorf("build execution transaction: %w", err)
	}

	m.logger.Info("✅ AI模型调用交易构建完成")

	// ========== 8. 为引用型资源输入补充 SingleKeyProof（模型当前采用 SingleKeyLock 作为访问控制）==========
	//
	// 说明：
	//   - DeployAIModel 部署的模型 ResourceOutput 使用 SingleKeyLock（RequiredAddressHash = 部署者地址）
	//   - wes_callAIModel 当前在测试脚本中使用同一私钥作为“部署者 + 调用者”
	//   - 这里复用通用的 signTransaction 辅助函数，为第一个输入追加 SingleKeyProof
	//   - 该输入引用模型的 ResourceOutput 且 is_reference_only = true，确保“引用不消费”语义不变
	//
	// 后续演进（文档中已规划）：
	//   - 模型资源访问将迁移到 ContractLock + ExecutionProof 统一路径
	//   - 届时这里的 SingleKeyProof 将被 ExecutionProof 所取代
	if !req.ReturnUnsignedTx {
		// 仅在需要提交交易的情况下才补签（unsignedTx 模式交由上层处理）
		if err := m.signTransaction(ctx, transaction, privateKey, callerAddrBytes); err != nil {
			m.logger.Error("为AI模型调用交易补充 SingleKeyProof 失败", zap.Error(err))
			return nil, fmt.Errorf("sign execution transaction (ai model): %w", err)
		}
	}

	// ========== 9. 计算交易哈希（使用统一的gRPC哈希服务）==========
	txHashResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{
		Transaction: transaction,
	})
	if err != nil || txHashResp == nil || !txHashResp.IsValid {
		m.logger.Error("计算交易哈希失败", zap.Error(err))
		return nil, fmt.Errorf("compute transaction hash: %w", err)
	}

	txHash := txHashResp.Hash
	m.logger.Info("✅ 交易哈希计算完成（gRPC服务）", zap.String("tx_hash", hex.EncodeToString(txHash)))

	// ========== 10. 如果 return_unsigned_tx=true，返回未签名交易 ==========
	if req.ReturnUnsignedTx {
		// 序列化未签名交易
		txBytes, err := proto.Marshal(transaction)
		if err != nil {
			m.logger.Error("序列化交易失败", zap.Error(err))
			return nil, fmt.Errorf("marshal transaction: %w", err)
		}
		unsignedTxHex := hex.EncodeToString(txBytes)
		txHashHex := format.HashToHex(txHash)

		m.logger.Info("✅ 返回未签名交易", zap.String("tx_hash", txHashHex))

		return map[string]interface{}{
			"success":     true,
			"unsigned_tx": unsignedTxHex,
			"tx_hash":     txHashHex,
			"outputs":    executionResult.ReturnTensors,
			"message":    "AI模型调用成功（未签名交易）",
		}, nil
	}

	// ========== 11. 签名交易（用于交易级签名与追责，而非UTXO级权限验证）==========
	if privateKey == nil {
		return nil, fmt.Errorf("private key is required when return_unsigned_tx is false")
	}

	signature, err := ecdsacrypto.Sign(txHash, privateKey)
	if err != nil {
		m.logger.Error("签名交易失败", zap.Error(err))
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 移除recovery ID，使用64字节签名
	signature64 := signature[:64]
	normalizedSignature := normalizeSignature(signature64)

	m.logger.Info("✅ 交易签名完成", zap.Int("signature_bytes", len(normalizedSignature)))

	// ========== 12. 提交交易到内存池 ==========
	_, err = m.mempool.SubmitTx(transaction)
	if err != nil {
		m.logger.Error("提交交易到内存池失败", zap.Error(err))
		return nil, fmt.Errorf("submit transaction: %w", err)
	}

	m.logger.Info("✅ AI模型调用交易已提交到内存池")

	// ========== 13. 返回完整执行结果 ==========
	txHashHex := hex.EncodeToString(txHash[:])

	result := map[string]interface{}{
		"success": true,
		"tx_hash": txHashHex,
		"outputs": executionResult.ReturnTensors,
		"message": "AI模型调用成功",
	}

	// Phase 4: 添加 CU 和费用信息到返回结果
	if cuVal, ok := executionResult.ExecutionContext["compute_units"].(float64); ok {
		cuInfo := map[string]interface{}{
			"compute_units": cuVal,
		}
		if planVal, ok := executionResult.ExecutionContext["billing_plan"].(map[string]interface{}); ok {
			cuInfo["billing_plan"] = planVal
		}
		result["compute_info"] = cuInfo
	}

	return result, nil
}

// DeployAIModel 部署AI模型 (wes_deployAIModel)
//
// 🎯 **功能**：完整的AI模型部署流程（存储ONNX、构建交易、签名、提交）
//
// 📋 **参数**（JSON格式）：
//
//	{
//	  "private_key": "十六进制私钥",
//	  "onnx_content": "Base64编码的ONNX文件内容",
//	  "name": "模型名称",
//	  "description": "模型描述（可选）"
//	}
//
// 📋 **返回**（JSON格式）：
//
//	{
//	  "content_hash": "模型ID（64位十六进制）",
//	  "tx_hash": "交易哈希（64位十六进制）",
//	  "success": true,
//	  "message": "部署成功"
//	}
func (m *TxMethods) DeployAIModel(ctx context.Context, params json.RawMessage) (interface{}, error) {
	m.logger.Info("📤 [wes_deployAIModel] 开始处理AI模型部署请求")

	// 解析参数（JSON-RPC可能发送数组格式：[{...}]）
	var req struct {
		PrivateKey  string `json:"private_key"`
		OnnxContent string `json:"onnx_content"` // Base64编码的ONNX内容
		Name        string `json:"name"`
		Description string `json:"description"`

		// Phase 2: 定价参数（可选）
		Pricing *struct {
			BillingMode   string `json:"billing_mode"`            // FREE / FIXED / CU_BASED
			OwnerAddress  string `json:"owner_address,omitempty"` // 资源所有者地址（默认使用部署者地址）
			PaymentTokens []struct {
				TokenID string `json:"token_id"` // 代币标识符
				CUPrice string `json:"cu_price"` // CU 单价（字符串格式，如 "1000000000000000"）
			} `json:"payment_tokens,omitempty"` // 仅 CU_BASED 模式需要
			FixedFee  string `json:"fixed_fee,omitempty"`  // 仅 FIXED 模式需要
			FreeUntil uint64 `json:"free_until,omitempty"` // 免费期限（Unix 时间戳）
		} `json:"pricing,omitempty"`
	}

	// 尝试解析数组格式：[{...}]
	var paramsArray []map[string]interface{}
	if err := json.Unmarshal(params, &paramsArray); err == nil && len(paramsArray) > 0 {
		// 成功解析为数组，取第一个元素
		paramsBytes, err := json.Marshal(paramsArray[0])
		if err != nil {
			m.logger.Error("序列化参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("marshal params object: %w", err)
		}
		if err := json.Unmarshal(paramsBytes, &req); err != nil {
			m.logger.Error("解析参数对象失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params object: %w", err)
		}
	} else {
		// 尝试直接解析为对象：{...}
		if err := json.Unmarshal(params, &req); err != nil {
			m.logger.Error("解析参数失败", zap.Error(err))
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// 参数校验
	if req.PrivateKey == "" {
		return nil, fmt.Errorf("private_key is required")
	}
	if req.OnnxContent == "" {
		return nil, fmt.Errorf("onnx_content is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	m.logger.Info("🔍 [DEBUG] 收到AI模型部署参数",
		zap.String("name", req.Name),
		zap.Int("onnx_content_length", len(req.OnnxContent)),
	)

	// ========== 1. 解码Base64 ONNX内容 ==========
	onnxBytes, err := base64.StdEncoding.DecodeString(req.OnnxContent)
	if err != nil {
		m.logger.Error("解码ONNX内容失败", zap.Error(err))
		return nil, fmt.Errorf("decode onnx content: %w", err)
	}

	m.logger.Info("✅ ONNX内容解码成功", zap.Int("size_bytes", len(onnxBytes)))

	// ========== 2. 验证ONNX格式（简单检查：至少要有一定大小）==========
	if len(onnxBytes) < 16 {
		m.logger.Error("无效的ONNX文件：文件太小")
		return nil, fmt.Errorf("invalid onnx file: file too small")
	}

	m.logger.Info("✅ ONNX格式基本验证通过")

	// ========== 3. 存储文件到CAS并获取contentHash ==========
	// 计算文件内容哈希
	hash := sha256.Sum256(onnxBytes)
	contentHash := hash[:]
	// 存储文件到CAS
	if err := m.uresCAS.StoreFile(ctx, contentHash, onnxBytes); err != nil {
		m.logger.Error("存储ONNX文件到CAS失败", zap.Error(err))
		return nil, fmt.Errorf("store onnx file: %w", err)
	}

	contentHashHex := hex.EncodeToString(contentHash)
	m.logger.Info("✅ ONNX文件已存储", zap.String("content_hash", contentHashHex))

	// ========== 4. 提取ONNX模型元数据（输入/输出名称）==========
	// 简化方案：暂时不提取元数据，在调用时由引擎自动加载
	// TODO: 未来可以通过依赖注入ONNX引擎来提取元数据
	inputNames := []string{}  // 空列表，由引擎在调用时自动加载
	outputNames := []string{} // 空列表，由引擎在调用时自动加载

	m.logger.Info("✅ ONNX模型元数据准备完成（输入/输出名称将在调用时自动加载）")

	// ========== 5. 构建AI Model Resource protobuf ==========
	aiModelResource := &respb.Resource{
		Category:         respb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType:   respb.ExecutableType_EXECUTABLE_TYPE_AIMODEL,
		Name:             req.Name,
		Version:          "1.0",
		MimeType:         "application/onnx",
		ContentHash:      contentHash,
		Size:             uint64(len(onnxBytes)),
		Description:      req.Description,
		CreatedTimestamp: uint64(time.Now().Unix()),
		OriginalFilename: req.Name + ".onnx",
		FileExtension:    ".onnx",
		ExecutionConfig: &respb.Resource_Aimodel{
			Aimodel: &respb.AIModelExecutionConfig{
				ModelFormat:     "ONNX",
				InputNames:      inputNames,
				OutputNames:     outputNames,
				ExecutionParams: map[string]string{}, // 可选执行参数
			},
		},
	}

	m.logger.Info("✅ AI Model Resource protobuf构建完成")

	// ========== 6. 从私钥推导部署者地址 ==========
	privateKeyBytes, err := hex.DecodeString(strings.TrimPrefix(req.PrivateKey, "0x"))
	if err != nil {
		m.logger.Error("解码私钥失败", zap.Error(err))
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	privateKey, err := ecdsacrypto.ToECDSA(privateKeyBytes)
	if err != nil {
		m.logger.Error("解析私钥失败", zap.Error(err))
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	publicKey := ecdsacrypto.CompressPubkey(&privateKey.PublicKey)
	ownerAddrBytes := hash160(publicKey)

	m.logger.Info("✅ 部署者地址推导完成", zap.String("address_hex", hex.EncodeToString(ownerAddrBytes)))

	// ========== 7. 构建ResourceOutput ==========
	resourceOutput := &txpb.ResourceOutput{
		Resource:          aiModelResource,
		CreationTimestamp: timeutil.NowUnix(),
		StorageStrategy:   txpb.ResourceOutput_STORAGE_STRATEGY_CONTENT_ADDRESSED,
		IsImmutable:       true, // AI模型一旦部署不可变
	}

	// ========== 8. 构建锁定条件（单密钥锁）==========
	lockingCondition := &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_SingleKeyLock{
			SingleKeyLock: &txpb.SingleKeyLock{
				KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: ownerAddrBytes,
				},
			},
		},
	}

	// ========== 9. 构建TxOutput ==========
	txOutput := &txpb.TxOutput{
		Owner: ownerAddrBytes, // 设置所有者地址（索引/展示用途，权限以locking_conditions为准）
		OutputContent: &txpb.TxOutput_Resource{
			Resource: resourceOutput,
		},
		LockingConditions: []*txpb.LockingCondition{lockingCondition},
	}

	// ========== 9.5. Phase 2: 处理定价参数（如果提供）==========
	outputs := []*txpb.TxOutput{txOutput}

	if req.Pricing != nil {
		m.logger.Info("📊 Phase 2: 检测到定价参数，创建定价状态")

		// 确定资源所有者地址（优先使用定价参数中的地址，否则使用部署者地址）
		pricingOwnerAddr := ownerAddrBytes
		if req.Pricing.OwnerAddress != "" {
			// 解析十六进制地址
			if addrBytes, err := hex.DecodeString(strings.TrimPrefix(req.Pricing.OwnerAddress, "0x")); err == nil && len(addrBytes) == 20 {
				pricingOwnerAddr = addrBytes
			} else {
				m.logger.Warn("无效的 owner_address，使用部署者地址", zap.String("provided", req.Pricing.OwnerAddress))
			}
		}

		// 解析计费模式
		billingMode := pkgtypes.BillingMode(req.Pricing.BillingMode)
		if !billingMode.IsValid() {
			return nil, fmt.Errorf("无效的计费模式: %s，支持的模式: FREE, FIXED, CU_BASED", req.Pricing.BillingMode)
		}

		// 创建 ResourcePricingState
		pricingState := pkgtypes.NewResourcePricingState(
			contentHash,
			pricingOwnerAddr,
			billingMode,
		)

		// 根据计费模式配置定价状态
		switch billingMode {
		case pkgtypes.BillingModeCUBASED:
			// CU_BASED 模式：需要配置支付代币和 CU 单价
			// ⚠️ 当前实现约束：每个资源仅支持 1 个支付代币，简化后续调用与结算路径
			if len(req.Pricing.PaymentTokens) == 0 {
				return nil, fmt.Errorf("CU_BASED 模式必须至少配置 1 个支付代币")
			}
			if len(req.Pricing.PaymentTokens) > 1 {
				return nil, fmt.Errorf("CU_BASED 模式当前仅支持 1 个支付代币，实际: %d", len(req.Pricing.PaymentTokens))
			}

			for _, token := range req.Pricing.PaymentTokens {
				// token.TokenID 允许为空：表示原生代币（与 TokenReference.native_token 语义对齐）
				if token.CUPrice == "" {
					return nil, fmt.Errorf("支付代币 %s 的 cu_price 不能为空", token.TokenID)
				}

				// 解析 CU 单价（字符串转 big.Int）
				cuPrice, ok := new(big.Int).SetString(token.CUPrice, 10)
				if !ok {
					return nil, fmt.Errorf("无效的 CU 单价: %s (代币: %s)", token.CUPrice, token.TokenID)
				}
				if cuPrice.Sign() < 0 {
					return nil, fmt.Errorf("CU 单价必须 >= 0: %s (代币: %s)", token.CUPrice, token.TokenID)
				}

				pricingState.AddPaymentToken(pkgtypes.TokenID(token.TokenID), cuPrice)
			}

		case pkgtypes.BillingModeFIXED:
			// FIXED 模式：需要配置固定费用
			if req.Pricing.FixedFee == "" {
				return nil, fmt.Errorf("FIXED 模式必须设置 fixed_fee")
			}
			fixedFee, ok := new(big.Int).SetString(req.Pricing.FixedFee, 10)
			if !ok {
				return nil, fmt.Errorf("无效的固定费用: %s", req.Pricing.FixedFee)
			}
			if fixedFee.Sign() < 0 {
				return nil, fmt.Errorf("固定费用必须 >= 0: %s", req.Pricing.FixedFee)
			}
			pricingState.SetFixedFee(fixedFee)

		case pkgtypes.BillingModeFREE:
			// FREE 模式：无需额外配置
			m.logger.Info("配置为免费模式")
		}

		// 设置免费期限（如果提供）
		if req.Pricing.FreeUntil > 0 {
			pricingState.SetFreeUntil(req.Pricing.FreeUntil)
		}

		// 验证定价状态
		if err := pricingState.Validate(); err != nil {
			return nil, fmt.Errorf("定价状态验证失败: %w", err)
		}

		// 序列化定价状态
		pricingStateBytes, err := pricingState.Encode()
		if err != nil {
			return nil, fmt.Errorf("序列化定价状态失败: %w", err)
		}

		m.logger.Info("✅ 定价状态创建成功",
			zap.String("billing_mode", billingMode.String()),
			zap.Int("payment_tokens", len(pricingState.PaymentTokens)),
			zap.Int("pricing_state_size", len(pricingStateBytes)),
		)

		// 创建 StateOutput（定价状态）
		// 注意：StateOutput 的 ZkProof 字段在 proto 中定义为可选，定价状态不需要 ZK 证明
		pricingStateID := sha256.Sum256(append(contentHash, []byte("_pricing")...))

		// 计算定价状态的哈希（用于 ExecutionResultHash）
		pricingStateHash := sha256.Sum256(pricingStateBytes)

		pricingStateOutput := &txpb.StateOutput{
			StateId:             pricingStateID[:],
			StateVersion:        1,
			ZkProof:             nil,                 // 定价状态不需要 ZK 证明（配置数据，非执行结果）
			ExecutionResultHash: pricingStateHash[:], // 使用定价状态的哈希
			Metadata: map[string]string{
				"resource_hash": hex.EncodeToString(contentHash),
				"pricing_state": string(pricingStateBytes), // JSON 字符串
				"pricing_type":  "resource_pricing",
			},
		}

		// 创建 StateOutput 的 TxOutput
		pricingStateTxOutput := &txpb.TxOutput{
			Owner: pricingOwnerAddr,
			OutputContent: &txpb.TxOutput_State{
				State: pricingStateOutput,
			},
			LockingConditions: []*txpb.LockingCondition{
				{
					Condition: &txpb.LockingCondition_SingleKeyLock{
						SingleKeyLock: &txpb.SingleKeyLock{
							KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: pricingOwnerAddr,
							},
						},
					},
				},
			},
		}

		// 将定价状态输出添加到交易输出列表
		outputs = append(outputs, pricingStateTxOutput)

		m.logger.Info("✅ 定价状态 StateOutput 已添加到交易")
	}

	// ========== 10. 构建交易（ResourceOutput + 可选的 StateOutput(定价状态)）==========
	transaction := &txpb.Transaction{
		Version:           1,
		CreationTimestamp: uint64(time.Now().Unix()),
		Inputs:            []*txpb.TxInput{}, // AI模型部署无UTXO输入
		Outputs:           outputs,
	}

	m.logger.Info("✅ 交易构建完成")

	// ========== 11. 计算交易哈希（使用统一的gRPC哈希服务）==========
	txHashResp, err := m.txHashCli.ComputeHash(ctx, &txpb.ComputeHashRequest{
		Transaction: transaction,
	})
	if err != nil || txHashResp == nil || !txHashResp.IsValid {
		m.logger.Error("计算交易哈希失败", zap.Error(err))
		return nil, fmt.Errorf("compute transaction hash: %w", err)
	}

	txHash := txHashResp.Hash
	m.logger.Info("✅ 交易哈希计算完成（gRPC服务）", zap.String("tx_hash", hex.EncodeToString(txHash)))

	// ========== 12. 签名交易 ==========
	signature, err := ecdsacrypto.Sign(txHash, privateKey)
	if err != nil {
		m.logger.Error("签名交易失败", zap.Error(err))
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 移除recovery ID（最后一个字节），使用64字节签名
	signature64 := signature[:64]
	normalizedSignature := normalizeSignature(signature64)

	m.logger.Info("✅ 交易签名完成", zap.Int("signature_length", len(normalizedSignature)))

	// ========== 13. 提交交易到内存池 ==========
	txHash2, err := m.mempool.SubmitTx(transaction)
	if err != nil {
		m.logger.Error("提交交易到内存池失败", zap.Error(err))
		return nil, fmt.Errorf("submit transaction: %w", err)
	}

	if txHash2 != nil {
		m.logger.Debug("内存池返回的交易哈希", zap.String("tx_hash", hex.EncodeToString(txHash2)))
	}

	m.logger.Info("✅ 交易已提交到内存池")

	// ========== 14. 返回结果 ==========
	txHashHex := hex.EncodeToString(txHash[:])

	m.logger.Info("🎉 AI模型部署完成！",
		zap.String("content_hash", contentHashHex),
		zap.String("tx_hash", txHashHex),
	)

	return map[string]interface{}{
		"content_hash": contentHashHex,
		"tx_hash":      txHashHex,
		"success":      true,
		"message":      "AI模型部署成功，交易已提交到内存池",
	}, nil
}

func (m *TxMethods) normalizeContractAmount(method string, payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return payload, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()

	var params map[string]interface{}
	if err := decoder.Decode(&params); err != nil {
		return payload, nil
	}

	amountValue, ok := params["amount"]
	if !ok {
		return payload, nil
	}

	unit := "wes"
	if rawUnit, ok := params["amount_unit"]; ok {
		if unitStr, ok := rawUnit.(string); ok && unitStr != "" {
			unit = strings.ToLower(strings.TrimSpace(unitStr))
		}
	}
	if unit == "wei" {
		return payload, nil
	}

	amountStr, ok := normalizeAmountField(amountValue)
	if !ok {
		return nil, fmt.Errorf("amount 字段类型不支持: %T", amountValue)
	}
	if amountStr == "" {
		return nil, fmt.Errorf("amount 不能为空")
	}

	amountWei, err := amountutils.ParseDecimalToWei(amountStr)
	if err != nil {
		return nil, fmt.Errorf("解析 amount 失败: %w", err)
	}

	params["amount"] = strconv.FormatUint(amountWei, 10)
	params["amount_unit"] = "wei"

	normalized, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化规范化金额失败: %w", err)
	}

	m.logger.Info("⚖️ 已自动转换合约金额为最小单位",
		zap.String("method", method),
		zap.String("amount_wes", amountStr),
		zap.Uint64("amount_wei", amountWei),
	)

	return normalized, nil
}

func normalizeAmountField(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), true
	case json.Number:
		return strings.TrimSpace(v.String()), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	default:
		return "", false
	}
}

// ============================================================================
// 锁定条件解析辅助函数（用于 wes_deployContract）
// ============================================================================

// parseLockingConditions 解析锁定条件列表
func (m *TxMethods) parseLockingConditions(
	rawConditions []map[string]interface{},
	deployerAddress []byte,
) ([]*txpb.LockingCondition, error) {
	if len(rawConditions) == 0 {
		// 默认：单密钥锁（部署者地址）
		return m.createDefaultSingleKeyLock(deployerAddress), nil
	}

	var conditions []*txpb.LockingCondition
	contractAddresses := make(map[string]bool) // 用于循环检测

	for _, raw := range rawConditions {
		conditionType, ok := raw["type"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'type' field in locking condition")
		}

		var condition *txpb.LockingCondition
		var err error

		switch conditionType {
		case "singleKey":
			condition, err = m.parseSingleKeyLock(raw, deployerAddress)
		case "multiKey":
			condition, err = m.parseMultiKeyLock(raw)
		case "timeLock":
			condition, err = m.parseTimeLock(raw, deployerAddress)
		case "heightLock":
			condition, err = m.parseHeightLock(raw, deployerAddress)
		case "delegation":
			condition, err = m.parseDelegationLock(raw)
		case "contract":
			condition, err = m.parseContractLock(raw, contractAddresses)
		case "threshold":
			condition, err = m.parseThresholdLock(raw)
		default:
			return nil, fmt.Errorf("unsupported locking condition type: %s", conditionType)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse %s lock: %w", conditionType, err)
		}

		conditions = append(conditions, condition)
	}

	return conditions, nil
}

// createDefaultSingleKeyLock 创建默认单密钥锁（向后兼容）
func (m *TxMethods) createDefaultSingleKeyLock(address []byte) []*txpb.LockingCondition {
	return []*txpb.LockingCondition{
		{
			Condition: &txpb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &txpb.SingleKeyLock{
					KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: address,
					},
					RequiredAlgorithm: txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
					SighashType:       txpb.SignatureHashType_SIGHASH_ALL,
				},
			},
		},
	}
}

// parseSingleKeyLock 解析单密钥锁定条件
func (m *TxMethods) parseSingleKeyLock(raw map[string]interface{}, deployerAddress []byte) (*txpb.LockingCondition, error) {
	singleKeyData, ok := raw["single_key_lock"].(map[string]interface{})
	if !ok {
		// 如果没有 single_key_lock 字段，使用默认地址
		return &txpb.LockingCondition{
			Condition: &txpb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &txpb.SingleKeyLock{
					KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: deployerAddress,
					},
					RequiredAlgorithm: txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
					SighashType:       txpb.SignatureHashType_SIGHASH_ALL,
				},
			},
		}, nil
	}

	addressHashStr, _ := singleKeyData["required_address_hash"].(string)
	algorithmStr, _ := singleKeyData["required_algorithm"].(string)

	var addressHash []byte
	if addressHashStr != "" {
		var err error
		addressHash, err = hex.DecodeString(strings.TrimPrefix(addressHashStr, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid address hash: %w", err)
		}
		if len(addressHash) != 20 {
			return nil, fmt.Errorf("address hash must be 20 bytes")
		}
	} else {
		addressHash = deployerAddress
	}

	algorithm := txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1
	if algorithmStr == "ED25519" {
		algorithm = txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_SingleKeyLock{
			SingleKeyLock: &txpb.SingleKeyLock{
				KeyRequirement: &txpb.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: addressHash,
				},
				RequiredAlgorithm: algorithm,
				SighashType:       txpb.SignatureHashType_SIGHASH_ALL,
			},
		},
	}, nil
}

// parseMultiKeyLock 解析多密钥锁定条件
func (m *TxMethods) parseMultiKeyLock(raw map[string]interface{}) (*txpb.LockingCondition, error) {
	multiKeyData, ok := raw["multi_key_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing multi_key_lock field")
	}

	requiredSignatures, ok := multiKeyData["required_signatures"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid required_signatures")
	}

	authorizedKeysRaw, ok := multiKeyData["authorized_keys"].([]interface{})
	if !ok || len(authorizedKeysRaw) == 0 {
		return nil, fmt.Errorf("missing or empty authorized_keys")
	}

	var authorizedKeys []*txpb.PublicKey
	for i, keyRaw := range authorizedKeysRaw {
		keyMap, ok := keyRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid authorized_key[%d]", i)
		}
		keyValueStr, _ := keyMap["value"].(string)
		if keyValueStr == "" {
			return nil, fmt.Errorf("missing value in authorized_key[%d]", i)
		}
		keyBytes, err := hex.DecodeString(strings.TrimPrefix(keyValueStr, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid key value in authorized_key[%d]: %w", i, err)
		}
		authorizedKeys = append(authorizedKeys, &txpb.PublicKey{
			Value: keyBytes,
		})
	}

	if uint32(requiredSignatures) > uint32(len(authorizedKeys)) {
		return nil, fmt.Errorf("required_signatures cannot exceed authorized_keys count")
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_MultiKeyLock{
			MultiKeyLock: &txpb.MultiKeyLock{
				RequiredSignatures:       uint32(requiredSignatures),
				AuthorizedKeys:           authorizedKeys,
				RequiredAlgorithm:        txpb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				RequireOrderedSignatures: false,
				SighashType:              txpb.SignatureHashType_SIGHASH_ALL,
			},
		},
	}, nil
}

// parseTimeLock 解析时间锁定条件
func (m *TxMethods) parseTimeLock(raw map[string]interface{}, deployerAddress []byte) (*txpb.LockingCondition, error) {
	timeLockData, ok := raw["time_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing time_lock field")
	}

	unlockTimestamp, ok := timeLockData["unlock_timestamp"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid unlock_timestamp")
	}

	baseLockRaw, ok := timeLockData["base_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing base_lock")
	}

	baseLock, err := m.parseSingleLockingCondition(baseLockRaw, deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base_lock: %w", err)
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_TimeLock{
			TimeLock: &txpb.TimeLock{
				UnlockTimestamp: uint64(unlockTimestamp),
				BaseLock:        baseLock,
				TimeSource:      txpb.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}, nil
}

// parseHeightLock 解析高度锁定条件
func (m *TxMethods) parseHeightLock(raw map[string]interface{}, deployerAddress []byte) (*txpb.LockingCondition, error) {
	heightLockData, ok := raw["height_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing height_lock field")
	}

	unlockHeight, ok := heightLockData["unlock_height"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid unlock_height")
	}

	baseLockRaw, ok := heightLockData["base_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing base_lock")
	}

	baseLock, err := m.parseSingleLockingCondition(baseLockRaw, deployerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base_lock: %w", err)
	}

	confirmationBlocks := uint32(6) // 默认值
	if cb, ok := heightLockData["confirmation_blocks"].(float64); ok {
		confirmationBlocks = uint32(cb)
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_HeightLock{
			HeightLock: &txpb.HeightLock{
				UnlockHeight:       uint64(unlockHeight),
				BaseLock:           baseLock,
				ConfirmationBlocks: confirmationBlocks,
			},
		},
	}, nil
}

// parseDelegationLock 解析委托锁定条件
func (m *TxMethods) parseDelegationLock(raw map[string]interface{}) (*txpb.LockingCondition, error) {
	delegationData, ok := raw["delegation_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing delegation_lock field")
	}

	originalOwnerStr, _ := delegationData["original_owner"].(string)
	if originalOwnerStr == "" {
		return nil, fmt.Errorf("missing original_owner")
	}
	originalOwner, err := hex.DecodeString(strings.TrimPrefix(originalOwnerStr, "0x"))
	if err != nil || len(originalOwner) != 20 {
		return nil, fmt.Errorf("invalid original_owner")
	}

	allowedDelegatesRaw, ok := delegationData["allowed_delegates"].([]interface{})
	if !ok || len(allowedDelegatesRaw) == 0 {
		return nil, fmt.Errorf("missing or empty allowed_delegates")
	}

	var allowedDelegates [][]byte
	for i, delegateStr := range allowedDelegatesRaw {
		delegate, ok := delegateStr.(string)
		if !ok {
			return nil, fmt.Errorf("invalid allowed_delegate[%d]", i)
		}
		delegateBytes, err := hex.DecodeString(strings.TrimPrefix(delegate, "0x"))
		if err != nil || len(delegateBytes) != 20 {
			return nil, fmt.Errorf("invalid allowed_delegate[%d]", i)
		}
		allowedDelegates = append(allowedDelegates, delegateBytes)
	}

	authorizedOperationsRaw, _ := delegationData["authorized_operations"].([]interface{})
	var authorizedOperations []string
	for _, op := range authorizedOperationsRaw {
		if opStr, ok := op.(string); ok {
			authorizedOperations = append(authorizedOperations, opStr)
		}
	}

	var expiryDurationBlocks *uint64
	if edb, ok := delegationData["expiry_duration_blocks"].(float64); ok && edb > 0 {
		val := uint64(edb)
		expiryDurationBlocks = &val
	}

	maxValuePerOperation := uint64(0)
	if mvo, ok := delegationData["max_value_per_operation"].(float64); ok {
		maxValuePerOperation = uint64(mvo)
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_DelegationLock{
			DelegationLock: &txpb.DelegationLock{
				OriginalOwner:        originalOwner,
				AllowedDelegates:     allowedDelegates,
				AuthorizedOperations: authorizedOperations,
				ExpiryDurationBlocks: expiryDurationBlocks,
				MaxValuePerOperation: maxValuePerOperation,
			},
		},
	}, nil
}

// parseContractLock 解析合约锁定条件
func (m *TxMethods) parseContractLock(raw map[string]interface{}, contractAddresses map[string]bool) (*txpb.LockingCondition, error) {
	contractData, ok := raw["contract_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing contract_lock field")
	}

	contractAddressStr, _ := contractData["contract_address"].(string)
	if contractAddressStr == "" {
		return nil, fmt.Errorf("missing contract_address")
	}
	contractAddress, err := hex.DecodeString(strings.TrimPrefix(contractAddressStr, "0x"))
	if err != nil || len(contractAddress) != 20 {
		return nil, fmt.Errorf("invalid contract_address")
	}

	// 检查循环依赖
	addrHex := hex.EncodeToString(contractAddress)
	if contractAddresses[addrHex] {
		return nil, fmt.Errorf("duplicate contract lock address: %s", addrHex)
	}
	contractAddresses[addrHex] = true

	requiredMethod, _ := contractData["required_method"].(string)
	if requiredMethod == "" {
		return nil, fmt.Errorf("missing required_method")
	}

	parameterSchema, _ := contractData["parameter_schema"].(string)
	stateRequirementsRaw, _ := contractData["state_requirements"].([]interface{})
	var stateRequirements []string
	for _, req := range stateRequirementsRaw {
		if reqStr, ok := req.(string); ok {
			stateRequirements = append(stateRequirements, reqStr)
		}
	}

	maxExecutionTimeMs := uint64(5000) // 默认5秒
	if met, ok := contractData["max_execution_time_ms"].(float64); ok {
		maxExecutionTimeMs = uint64(met)
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_ContractLock{
			ContractLock: &txpb.ContractLock{
				ContractAddress:    contractAddress,
				RequiredMethod:     requiredMethod,
				ParameterSchema:    parameterSchema,
				StateRequirements:  stateRequirements,
				MaxExecutionTimeMs: maxExecutionTimeMs,
			},
		},
	}, nil
}

// parseThresholdLock 解析门限签名锁定条件
func (m *TxMethods) parseThresholdLock(raw map[string]interface{}) (*txpb.LockingCondition, error) {
	thresholdData, ok := raw["threshold_lock"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing threshold_lock field")
	}

	threshold, ok := thresholdData["threshold"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid threshold")
	}

	totalParties, ok := thresholdData["total_parties"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid total_parties")
	}

	partyKeysRaw, ok := thresholdData["party_verification_keys"].([]interface{})
	if !ok || len(partyKeysRaw) != int(totalParties) {
		return nil, fmt.Errorf("party_verification_keys count must match total_parties")
	}

	var partyKeys [][]byte
	for i, keyStr := range partyKeysRaw {
		key, ok := keyStr.(string)
		if !ok {
			return nil, fmt.Errorf("invalid party_verification_key[%d]", i)
		}
		keyBytes, err := hex.DecodeString(strings.TrimPrefix(key, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid party_verification_key[%d]: %w", i, err)
		}
		partyKeys = append(partyKeys, keyBytes)
	}

	signatureScheme, _ := thresholdData["signature_scheme"].(string)
	if signatureScheme == "" {
		signatureScheme = "BLS_THRESHOLD"
	}

	if uint32(threshold) > uint32(totalParties) {
		return nil, fmt.Errorf("threshold cannot exceed total_parties")
	}

	return &txpb.LockingCondition{
		Condition: &txpb.LockingCondition_ThresholdLock{
			ThresholdLock: &txpb.ThresholdLock{
				Threshold:             uint32(threshold),
				TotalParties:          uint32(totalParties),
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       signatureScheme,
				SecurityLevel:         256,
			},
		},
	}, nil
}

// parseSingleLockingCondition 解析单个锁定条件（用于 TimeLock/HeightLock 的 base_lock）
func (m *TxMethods) parseSingleLockingCondition(raw map[string]interface{}, deployerAddress []byte) (*txpb.LockingCondition, error) {
	// 尝试识别类型
	if _, ok := raw["single_key_lock"]; ok {
		return m.parseSingleKeyLock(raw, deployerAddress)
	}
	if _, ok := raw["multi_key_lock"]; ok {
		return m.parseMultiKeyLock(raw)
	}
	// 默认使用单密钥锁
	return m.parseSingleKeyLock(raw, deployerAddress)
}
