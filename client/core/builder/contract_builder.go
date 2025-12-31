package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/weisyn/v1/client/core/transport"
)

// ContractBuilder 合约Draft构建器
// 负责构建合约部署和调用交易
type ContractBuilder struct {
	client transport.Client
}

// NewContractBuilder 创建合约构建器
func NewContractBuilder(client transport.Client) *ContractBuilder {
	return &ContractBuilder{
		client: client,
	}
}

// ========== 合约部署 ==========

// DeployRequest 合约部署请求
type DeployRequest struct {
	WasmPath     string            // WASM文件路径
	Deployer     string            // 部署者地址
	ContractName string            // 合约名称（可选）
	InitArgs     map[string]string // 初始化参数（可选）
	Memo         string            // 备注（可选）
}

// BuildDeploy 构建合约部署交易Draft
//
// 流程：
//  1. 读取WASM文件
//  2. 计算文件哈希
//  3. 查询部署者的UTXO（用于支付费用）
//  4. 构建部署输出
//  5. 返回Draft
func (cb *ContractBuilder) BuildDeploy(ctx context.Context, req *DeployRequest) (*DraftTx, error) {
	// 1. 参数验证
	if err := cb.validateDeployRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 读取WASM文件
	wasmData, err := os.ReadFile(req.WasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file: %w", err)
	}

	// 3. 验证WASM格式（简单验证magic number）
	if err := validateWasmFormat(wasmData); err != nil {
		return nil, fmt.Errorf("invalid wasm format: %w", err)
	}

	// 4. 计算WASM哈希（用作合约地址）
	wasmHash := computeContentHash(wasmData)

	// 5. 查询部署者的UTXO（用于支付费用）
	utxos, err := cb.client.GetUTXOs(ctx, req.Deployer, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no available UTXOs for deployer %s", req.Deployer)
	}

	// 6. 估算部署费用（基于WASM大小）
	estimatedFee := estimateDeployFee(len(wasmData))

	// 7. 选择UTXO
	selector := NewFirstFitSelector()
	convertedUTXOs := convertTransportUTXOs(utxos)
	selectedUTXOs, totalSelected, err := selector.Select(convertedUTXOs, estimatedFee)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w", err)
	}

	// 8. 计算找零
	change, err := totalSelected.Sub(estimatedFee)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	// 9. 创建Draft
	builder := NewTxBuilder(cb.client).(*DefaultTxBuilder)
	draft := builder.CreateDraft()

	// 10. 添加输入（支付费用）
	for _, utxo := range selectedUTXOs {
		draft.AddInput(Input{
			TxHash:      utxo.TxHash,
			OutputIndex: utxo.Vout,
			Amount:      utxo.Amount.StringUnits(),
			Address:     utxo.Address,
			LockScript:  string(utxo.ScriptPub),
		})
	}

	// 11. 添加合约部署输出
	draft.AddOutput(Output{
		Address:    wasmHash, // 合约地址 = WASM哈希
		Amount:     "0",      // 部署不需要转账金额
		Type:       OutputTypeContract,
		LockScript: generateContractLockScript(wasmHash),
		Data: map[string]interface{}{
			"type":          "deploy",
			"wasm_hash":     wasmHash,
			"wasm_size":     len(wasmData),
			"contract_name": req.ContractName,
			"init_args":     req.InitArgs,
			"wasm_data":     hex.EncodeToString(wasmData), // 完整WASM数据
		},
	})

	// 12. 添加找零输出
	if change.IsPositive() {
		draft.AddOutput(Output{
			Address:    req.Deployer,
			Amount:     change.StringUnits(),
			Type:       OutputTypeTransfer,
			LockScript: generateContractAddressLockScript(req.Deployer),
		})
	}

	// 13. 设置参数
	if req.Memo != "" {
		draft.SetMemo(req.Memo)
	}

	draft.params.Extra = map[string]interface{}{
		"contract_address": wasmHash,
		"estimated_fee":    estimatedFee.StringUnits(),
	}

	return draft, nil
}

// ========== 合约调用 ==========

// CallRequest 合约调用请求
type CallRequest struct {
	ContractAddress string            // 合约地址
	Method          string            // 调用方法
	Args            map[string]string // 方法参数
	Caller          string            // 调用者地址
	Amount          *Amount           // 转账金额（如果需要）
	Memo            string            // 备注（可选）
}

// BuildCall 构建合约调用交易Draft
//
// 流程：
//  1. 查询合约是否存在
//  2. 查询调用者的UTXO
//  3. 估算调用费用
//  4. 构建调用输出
//  5. 返回Draft
func (cb *ContractBuilder) BuildCall(ctx context.Context, req *CallRequest) (*DraftTx, error) {
	// 1. 参数验证
	if err := cb.validateCallRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 查询调用者的UTXO
	utxos, err := cb.client.GetUTXOs(ctx, req.Caller, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no available UTXOs for caller %s", req.Caller)
	}

	// 3. 估算调用费用
	estimatedFee := estimateCallFee(len(req.Args))

	// 4. 计算需要的总金额（转账金额 + 费用）
	var targetAmount *Amount
	if req.Amount != nil && req.Amount.IsPositive() {
		targetAmount = req.Amount.Add(estimatedFee)
	} else {
		targetAmount = estimatedFee
	}

	// 5. 选择UTXO
	selector := NewFirstFitSelector()
	convertedUTXOs := convertTransportUTXOs(utxos)
	selectedUTXOs, totalSelected, err := selector.Select(convertedUTXOs, targetAmount)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w", err)
	}

	// 6. 计算找零
	change, err := totalSelected.Sub(targetAmount)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	// 7. 创建Draft
	builder := NewTxBuilder(cb.client).(*DefaultTxBuilder)
	draft := builder.CreateDraft()

	// 8. 添加输入
	for _, utxo := range selectedUTXOs {
		draft.AddInput(Input{
			TxHash:      utxo.TxHash,
			OutputIndex: utxo.Vout,
			Amount:      utxo.Amount.StringUnits(),
			Address:     utxo.Address,
			LockScript:  string(utxo.ScriptPub),
		})
	}

	// 9. 添加合约调用输出
	callAmount := "0"
	if req.Amount != nil {
		callAmount = req.Amount.StringUnits()
	}

	draft.AddOutput(Output{
		Address:    req.ContractAddress,
		Amount:     callAmount,
		Type:       OutputTypeContract,
		LockScript: generateContractLockScript(req.ContractAddress),
		Data: map[string]interface{}{
			"type":   "call",
			"method": req.Method,
			"args":   req.Args,
		},
	})

	// 10. 添加找零输出
	if change.IsPositive() {
		draft.AddOutput(Output{
			Address:    req.Caller,
			Amount:     change.StringUnits(),
			Type:       OutputTypeTransfer,
			LockScript: generateContractAddressLockScript(req.Caller),
		})
	}

	// 11. 设置参数
	if req.Memo != "" {
		draft.SetMemo(req.Memo)
	}

	draft.params.Extra = map[string]interface{}{
		"contract_address": req.ContractAddress,
		"method":           req.Method,
		"estimated_fee":    estimatedFee.StringUnits(),
	}

	return draft, nil
}

// ========== 辅助函数 ==========

// validateWasmFormat 验证WASM格式
func validateWasmFormat(data []byte) error {
	// WASM文件的magic number: 0x00 0x61 0x73 0x6d (\0asm)
	if len(data) < 4 {
		return fmt.Errorf("file too small to be valid WASM")
	}

	magic := []byte{0x00, 0x61, 0x73, 0x6d}
	for i := 0; i < 4; i++ {
		if data[i] != magic[i] {
			return fmt.Errorf("invalid WASM magic number")
		}
	}

	return nil
}

// computeContentHash 计算内容哈希
func computeContentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return "0x" + hex.EncodeToString(hash[:])
}

// estimateDeployFee 估算部署费用
// 基于WASM文件大小计算
func estimateDeployFee(wasmSize int) *Amount {
	// 基础费用 + 存储费用
	baseFee := uint64(100000)           // 0.001 WES
	storageFee := uint64(wasmSize * 10) // 每字节10 sat
	totalFee := baseFee + storageFee

	return NewAmountFromUnits(totalFee)
}

// estimateCallFee 估算调用费用
// 基于参数数量简单估算
func estimateCallFee(numArgs int) *Amount {
	baseFee := uint64(50000)          // 0.0005 WES
	argsFee := uint64(numArgs * 1000) // 每个参数1000 sat
	totalFee := baseFee + argsFee

	return NewAmountFromUnits(totalFee)
}

// generateContractLockScript 生成合约锁定脚本
func generateContractLockScript(contractAddress string) string {
	return fmt.Sprintf("OP_CONTRACT_LOCK %s", contractAddress)
}

// generateContractAddressLockScript 生成普通地址锁定脚本
func generateContractAddressLockScript(address string) string {
	return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address)
}

// convertTransportUTXOs 转换transport.UTXO为builder.UTXO
func convertTransportUTXOs(rawUTXOs []*transport.UTXO) []UTXO {
	utxos := make([]UTXO, 0, len(rawUTXOs))

	for _, raw := range rawUTXOs {
		amount, err := NewAmountFromString(raw.Amount)
		if err != nil {
			continue
		}

		utxos = append(utxos, UTXO{
			TxHash:    raw.TxHash,
			Vout:      raw.OutputIndex,
			Amount:    amount,
			Address:   raw.Address,
			ScriptPub: []byte(raw.LockScript),
		})
	}

	return utxos
}

// validateDeployRequest 验证部署请求
func (cb *ContractBuilder) validateDeployRequest(req *DeployRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.WasmPath == "" {
		return fmt.Errorf("wasm_path is empty")
	}

	if req.Deployer == "" {
		return fmt.Errorf("deployer is empty")
	}

	return nil
}

// validateCallRequest 验证调用请求
func (cb *ContractBuilder) validateCallRequest(req *CallRequest) error {
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

	return nil
}
