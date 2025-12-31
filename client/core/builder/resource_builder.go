package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/weisyn/v1/client/core/transport"
)

// ResourceBuilder 资源Draft构建器
// 负责构建资源部署交易
type ResourceBuilder struct {
	client transport.Client
}

// NewResourceBuilder 创建资源构建器
func NewResourceBuilder(client transport.Client) *ResourceBuilder {
	return &ResourceBuilder{
		client: client,
	}
}

// DeployResourceRequest 资源部署请求
type DeployResourceRequest struct {
	FilePath     string // 资源文件路径
	Deployer     string // 部署者地址
	ResourceName string // 资源名称（可选）
	ResourceType string // 资源类型（image/video/document等）
	Memo         string // 备注（可选）
}

// BuildDeployResource 构建资源部署交易Draft
//
// 流程：
//  1. 读取资源文件
//  2. 计算文件哈希
//  3. 查询部署者的UTXO
//  4. 构建部署输出
//  5. 返回Draft
func (rb *ResourceBuilder) BuildDeployResource(ctx context.Context, req *DeployResourceRequest) (*DraftTx, error) {
	// 1. 参数验证
	if err := rb.validateDeployRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 读取资源文件
	fileData, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// 3. 计算文件哈希（用作资源地址）
	fileHash := computeFileHash(fileData)

	// 4. 查询部署者的UTXO（用于支付费用）
	utxos, err := rb.client.GetUTXOs(ctx, req.Deployer, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no available UTXOs for deployer %s", req.Deployer)
	}

	// 5. 估算部署费用（基于文件大小）
	estimatedFee := estimateResourceDeployFee(len(fileData))

	// 6. 选择UTXO
	selector := NewFirstFitSelector()
	convertedUTXOs := convertResourceUTXOs(utxos)
	selectedUTXOs, totalSelected, err := selector.Select(convertedUTXOs, estimatedFee)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w", err)
	}

	// 7. 计算找零
	change, err := totalSelected.Sub(estimatedFee)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	// 8. 创建Draft
	builder := NewTxBuilder(rb.client).(*DefaultTxBuilder)
	draft := builder.CreateDraft()

	// 9. 添加输入（支付费用）
	for _, utxo := range selectedUTXOs {
		draft.AddInput(Input{
			TxHash:      utxo.TxHash,
			OutputIndex: utxo.Vout,
			Amount:      utxo.Amount.StringUnits(),
			Address:     utxo.Address,
			LockScript:  string(utxo.ScriptPub),
		})
	}

	// 10. 添加资源部署输出
	draft.AddOutput(Output{
		Address:    fileHash, // 资源地址 = 文件哈希
		Amount:     "0",      // 资源部署不需要转账金额
		Type:       OutputTypeResource,
		LockScript: generateResourceLockScript(fileHash),
		Data: map[string]interface{}{
			"type":          "resource_deploy",
			"file_hash":     fileHash,
			"file_size":     len(fileData),
			"resource_name": req.ResourceName,
			"resource_type": req.ResourceType,
			"file_data":     hex.EncodeToString(fileData), // 完整文件数据
		},
	})

	// 11. 添加找零输出
	if change.IsPositive() {
		draft.AddOutput(Output{
			Address:    req.Deployer,
			Amount:     change.StringUnits(),
			Type:       OutputTypeTransfer,
			LockScript: generateResourceAddressLockScript(req.Deployer),
		})
	}

	// 12. 设置参数
	if req.Memo != "" {
		draft.SetMemo(req.Memo)
	}

	draft.params.Extra = map[string]interface{}{
		"resource_address": fileHash,
		"estimated_fee":    estimatedFee.StringUnits(),
	}

	return draft, nil
}

// ========== 辅助函数 ==========

// computeFileHash 计算文件哈希
func computeFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return "0x" + hex.EncodeToString(hash[:])
}

// estimateResourceDeployFee 估算资源部署费用
func estimateResourceDeployFee(fileSize int) *Amount {
	baseFee := uint64(50000)           // 0.0005 WES
	storageFee := uint64(fileSize * 5) // 每字节5 sat
	totalFee := baseFee + storageFee

	return NewAmountFromUnits(totalFee)
}

// generateResourceLockScript 生成资源锁定脚本
func generateResourceLockScript(resourceAddress string) string {
	return fmt.Sprintf("OP_RESOURCE_LOCK %s", resourceAddress)
}

// generateResourceAddressLockScript 生成普通地址锁定脚本
func generateResourceAddressLockScript(address string) string {
	return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address)
}

// convertResourceUTXOs 转换transport.UTXO为builder.UTXO
func convertResourceUTXOs(rawUTXOs []*transport.UTXO) []UTXO {
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
func (rb *ResourceBuilder) validateDeployRequest(req *DeployResourceRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.FilePath == "" {
		return fmt.Errorf("file_path is empty")
	}

	if req.Deployer == "" {
		return fmt.Errorf("deployer is empty")
	}

	return nil
}
