package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
)

// ContractService 合约业务服务
// 等价于旧TX的ContractService，提供完整的合约部署和调用业务逻辑
type ContractService struct {
	builder   *builder.ContractBuilder
	transport transport.Client
	signer    *wallet.Signer
}

// NewContractService 创建合约业务服务
func NewContractService(
	client transport.Client,
	signer *wallet.Signer,
) *ContractService {
	return &ContractService{
		builder:   builder.NewContractBuilder(client),
		transport: client,
		signer:    signer,
	}
}

// ========== 合约部署 ==========

// DeployRequest 合约部署请求
type DeployRequest struct {
	WasmPath     string            // WASM文件路径
	Deployer     string            // 部署者地址
	PrivateKey   []byte            // 部署者私钥
	ContractName string            // 合约名称（可选）
	InitArgs     map[string]string // 初始化参数（可选）
	Memo         string            // 备注（可选）
}

// DeployResult 合约部署结果
type DeployResult struct {
	TxHash          string // 交易哈希
	ContractAddress string // 合约地址
	Success         bool   // 是否成功
	Message         string // 结果消息
	Fee             string // 实际手续费
	BlockHeight     uint64 // 区块高度（待确认时为0）
}

// DeployContract 部署合约
//
// 完整流程：
//  1. 读取并验证WASM文件
//  2. 计算合约地址（WASM哈希）
//  3. 构建部署交易Draft
//  4. Seal、Sign、Broadcast
//
// 等价于旧CLI的ContractCommands.DeployContract()
func (cs *ContractService) DeployContract(ctx context.Context, req *DeployRequest) (*DeployResult, error) {
	// 1. 参数验证
	if err := cs.validateDeployRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 构建Draft交易
	draft, err := cs.builder.BuildDeploy(ctx, &builder.DeployRequest{
		WasmPath:     req.WasmPath,
		Deployer:     req.Deployer,
		ContractName: req.ContractName,
		InitArgs:     req.InitArgs,
		Memo:         req.Memo,
	})
	if err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}

	// 3. Seal - 密封交易，计算TxID
	composed, err := draft.Seal()
	if err != nil {
		return nil, fmt.Errorf("seal transaction: %w", err)
	}

	// 4. 添加解锁证明
	proofs := cs.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}

	// 5. 签名交易
	signed, err := cs.signTransaction(ctx, proven, req.Deployer, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 6. 广播交易
	txResult, err := cs.transport.SendRawTransaction(ctx, signed.RawHex())
	if err != nil {
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}

	// 7. 提取合约地址和费用信息
	contractAddress, fee := cs.extractDeployInfo(draft)

	return &DeployResult{
		TxHash:          txResult.TxHash,
		ContractAddress: contractAddress,
		Success:         true,
		Message:         "合约部署交易已提交",
		Fee:             fee,
		BlockHeight:     0, // 待确认
	}, nil
}

// ========== 合约调用 ==========

// CallRequest 合约调用请求
type CallRequest struct {
	ContractAddress string            // 合约地址
	Method          string            // 调用方法
	Args            map[string]string // 方法参数
	Caller          string            // 调用者地址
	PrivateKey      []byte            // 调用者私钥
	Amount          string            // 转账金额（如果需要，WES单位）
	Memo            string            // 备注（可选）
}

// CallResult 合约调用结果
type CallResult struct {
	TxHash      string                 // 交易哈希
	Success     bool                   // 是否成功
	Message     string                 // 结果消息
	ReturnValue map[string]interface{} // 返回值（待确认后查询）
	Fee         string                 // 实际手续费
	BlockHeight uint64                 // 区块高度（待确认时为0）
}

// CallContract 调用合约
//
// 完整流程：
//  1. 验证合约存在性（可选）
//  2. 构建调用交易Draft
//  3. Seal、Sign、Broadcast
//
// 等价于旧CLI的ContractCommands.CallContract()
func (cs *ContractService) CallContract(ctx context.Context, req *CallRequest) (*CallResult, error) {
	// 1. 参数验证
	if err := cs.validateCallRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 解析金额（如果有）
	var amount *builder.Amount
	if req.Amount != "" {
		var err error
		amount, err = builder.NewAmountFromString(req.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount: %w", err)
		}
	}

	// 3. 构建Draft交易
	draft, err := cs.builder.BuildCall(ctx, &builder.CallRequest{
		ContractAddress: req.ContractAddress,
		Method:          req.Method,
		Args:            req.Args,
		Caller:          req.Caller,
		Amount:          amount,
		Memo:            req.Memo,
	})
	if err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}

	// 4. Seal - 密封交易，计算TxID
	composed, err := draft.Seal()
	if err != nil {
		return nil, fmt.Errorf("seal transaction: %w", err)
	}

	// 5. 添加解锁证明
	proofs := cs.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}

	// 6. 签名交易
	signed, err := cs.signTransaction(ctx, proven, req.Caller, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 7. 广播交易
	txResult, err := cs.transport.SendRawTransaction(ctx, signed.RawHex())
	if err != nil {
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}

	// 8. 提取费用信息
	fee := cs.extractCallFee(draft)

	return &CallResult{
		TxHash:      txResult.TxHash,
		Success:     true,
		Message:     "合约调用交易已提交",
		ReturnValue: nil, // 需要等待交易确认后查询
		Fee:         fee,
		BlockHeight: 0, // 待确认
	}, nil
}

// ========== 合约查询（只读调用） ==========

// QueryRequest 合约查询请求（只读调用，不上链）
type QueryRequest struct {
	ContractAddress string            // 合约地址
	Method          string            // 查询方法
	Args            map[string]string // 方法参数
}

// QueryResult 合约查询结果
type QueryResult struct {
	Success     bool                   // 是否成功
	ReturnValue map[string]interface{} // 返回值
	Message     string                 // 结果消息
}

// QueryContract 查询合约（只读调用）
//
// 这是一个只读操作，不会上链，不消耗Gas
// 通过 wes_call 实现合约方法的只读调用
func (cs *ContractService) QueryContract(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	// 参数验证
	if req == nil || req.ContractAddress == "" || req.Method == "" {
		return nil, fmt.Errorf("invalid query request")
	}

	// 清理 ContractAddress（移除 0x 前缀）
	contentHash := strings.TrimPrefix(strings.TrimPrefix(req.ContractAddress, "0x"), "0X")

	// 将 Args map[string]string 转换为 []uint64（简单实现：尝试解析为 u64）
	// 注意：这是一个简化实现，实际场景可能需要更复杂的参数转换逻辑
	var params []uint64
	for _, valStr := range req.Args {
		if val, err := strconv.ParseUint(valStr, 10, 64); err == nil {
			params = append(params, val)
		}
	}

	// 组装 callData 用于 wes_call
	callSpec := map[string]interface{}{
		"method": req.Method,
		"params": params,
	}
	callSpecJSON, err := json.Marshal(callSpec)
	if err != nil {
		return &QueryResult{
			Success: false,
			Message: fmt.Sprintf("组装调用参数失败: %v", err),
		}, fmt.Errorf("组装调用参数失败: %w", err)
	}

	callData := map[string]interface{}{
		"to":   contentHash,
		"data": string(callSpecJSON),
	}

	// 调用 transport.CallRaw("wes_call", ...)
	rawResult, err := cs.transport.CallRaw(ctx, "wes_call", []interface{}{callData})
	if err != nil {
		return &QueryResult{
			Success: false,
			Message: fmt.Sprintf("合约调用失败: %v", err),
		}, fmt.Errorf("合约调用失败: %w", err)
	}

	// 解析返回结果
	resultMap, ok := rawResult.(map[string]interface{})
	if !ok {
		return &QueryResult{
			Success: false,
			Message: fmt.Sprintf("返回格式错误: %T", rawResult),
		}, fmt.Errorf("返回格式错误: %T", rawResult)
	}

	success := true
	if successVal, ok := resultMap["success"].(bool); ok {
		success = successVal
	}

	// 构建返回值映射
	returnValue := make(map[string]interface{})
	for k, v := range resultMap {
		returnValue[k] = v
	}

	message := "查询成功"
	if !success {
		message = "查询失败"
		if msgVal, ok := resultMap["error"].(string); ok {
			message = msgVal
		}
	}

	return &QueryResult{
		Success:     success,
		ReturnValue: returnValue,
		Message:     message,
	}, nil
}

// ========== 辅助方法 ==========

// generateProofs 生成解锁证明
func (cs *ContractService) generateProofs(composed *builder.ComposedTx) []builder.UnlockingProof {
	inputs := composed.Inputs()
	proofs := make([]builder.UnlockingProof, len(inputs))

	for i := range inputs {
		proofs[i] = builder.UnlockingProof{
			InputIndex: i,
			Type:       "signature",
			Data:       []byte{},
		}
	}

	return proofs
}

// signTransaction 签名交易
func (cs *ContractService) signTransaction(
	ctx context.Context,
	proven *builder.ProvenTx,
	fromAddress string,
	privateKey []byte,
) (*builder.SignedTx, error) {
	signers := make(map[string]string)
	txID := proven.TxID()

	signature, err := (*cs.signer).SignHash([]byte(txID), fromAddress)
	if err != nil {
		return nil, fmt.Errorf("sign hash with address %s: %w", fromAddress, err)
	}

	signers[fromAddress] = string(signature)

	signed, err := proven.Sign(cs.transport, signers)
	if err != nil {
		return nil, fmt.Errorf("create signed tx: %w", err)
	}

	return signed, nil
}

// extractDeployInfo 从Draft中提取部署信息
func (cs *ContractService) extractDeployInfo(draft *builder.DraftTx) (contractAddress, fee string) {
	if draft.GetParams().Extra != nil {
		if addr, ok := draft.GetParams().Extra["contract_address"].(string); ok {
			contractAddress = addr
		}
		if feeVal, ok := draft.GetParams().Extra["estimated_fee"].(string); ok {
			fee = feeVal
		}
	}
	return contractAddress, fee
}

// extractCallFee 从Draft中提取调用费用
func (cs *ContractService) extractCallFee(draft *builder.DraftTx) string {
	if draft.GetParams().Extra != nil {
		if feeVal, ok := draft.GetParams().Extra["estimated_fee"].(string); ok {
			return feeVal
		}
	}
	return "0"
}

// validateDeployRequest 验证部署请求
func (cs *ContractService) validateDeployRequest(req *DeployRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.WasmPath == "" {
		return fmt.Errorf("wasm_path is empty")
	}

	if req.Deployer == "" {
		return fmt.Errorf("deployer is empty")
	}

	if len(req.PrivateKey) == 0 {
		return fmt.Errorf("private_key is empty")
	}

	return nil
}

// validateCallRequest 验证调用请求
func (cs *ContractService) validateCallRequest(req *CallRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.ContractAddress == "" {
		return fmt.Errorf("contract_address is empty")
	}

	if req.Method == "" {
		return fmt.Errorf("method is empty")
	}

	if req.Caller == "" {
		return fmt.Errorf("caller is empty")
	}

	if len(req.PrivateKey) == 0 {
		return fmt.Errorf("private_key is empty")
	}

	return nil
}
