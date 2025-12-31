package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
)

// ResourceService 资源业务服务
// 等价于旧TX的ResourceService，提供完整的资源部署业务逻辑
type ResourceService struct {
	builder   *builder.ResourceBuilder
	transport transport.Client
	signer    *wallet.Signer
}

// NewResourceService 创建资源业务服务
func NewResourceService(
	client transport.Client,
	signer *wallet.Signer,
) *ResourceService {
	return &ResourceService{
		builder:   builder.NewResourceBuilder(client),
		transport: client,
		signer:    signer,
	}
}

// DeployRequest 资源部署请求
type DeployRequest struct {
	FilePath     string // 资源文件路径
	Deployer     string // 部署者地址
	PrivateKey   []byte // 部署者私钥
	ResourceName string // 资源名称（可选）
	ResourceType string // 资源类型（可选）
	Memo         string // 备注（可选）
}

// DeployResult 资源部署结果
type DeployResult struct {
	TxHash          string // 交易哈希
	ResourceAddress string // 资源地址
	Success         bool   // 是否成功
	Message         string // 结果消息
	Fee             string // 实际手续费
	BlockHeight     uint64 // 区块高度（待确认时为0）
}

// DeployResource 部署资源
//
// 完整流程：
//  1. 读取并验证文件
//  2. 计算资源地址（文件哈希）
//  3. 构建部署交易Draft
//  4. Seal、Sign、Broadcast
//
// 等价于旧CLI的ResourceCommands.DeployResource()
func (rs *ResourceService) DeployResource(ctx context.Context, req *DeployRequest) (*DeployResult, error) {
	// 1. 参数验证
	if err := rs.validateDeployRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 构建Draft交易
	draft, err := rs.builder.BuildDeployResource(ctx, &builder.DeployResourceRequest{
		FilePath:     req.FilePath,
		Deployer:     req.Deployer,
		ResourceName: req.ResourceName,
		ResourceType: req.ResourceType,
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
	proofs := rs.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}

	// 5. 签名交易
	signed, err := rs.signTransaction(ctx, proven, req.Deployer, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 6. 广播交易
	txResult, err := rs.transport.SendRawTransaction(ctx, signed.RawHex())
	if err != nil {
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}

	// 7. 提取资源地址和费用信息
	resourceAddress, fee := rs.extractDeployInfo(draft)

	return &DeployResult{
		TxHash:          txResult.TxHash,
		ResourceAddress: resourceAddress,
		Success:         true,
		Message:         "资源部署交易已提交",
		Fee:             fee,
		BlockHeight:     0,
	}, nil
}

// FetchRequest 资源获取请求
type FetchRequest struct {
	ResourceAddress string // 资源地址
}

// FetchResult 资源获取结果
type FetchResult struct {
	Success      bool                   // 是否成功
	Data         []byte                 // 资源数据（元数据查询时为空）
	ResourceName string                 // 资源名称
	ResourceType string                 // 资源类型
	Message      string                 // 结果消息
	Metadata     map[string]interface{} // 资源元数据（新增字段）
}

// FetchResource 获取资源元数据
//
// 从链上查询资源元数据（通过 wes_getResourceByContentHash）
func (rs *ResourceService) FetchResource(ctx context.Context, req *FetchRequest) (*FetchResult, error) {
	if req == nil || req.ResourceAddress == "" {
		return nil, fmt.Errorf("invalid request")
	}

	// 清理 ResourceAddress（移除 0x 前缀）
	contentHashHex := strings.TrimPrefix(strings.TrimPrefix(req.ResourceAddress, "0x"), "0X")

	// 调用节点的 wes_getResourceByContentHash 接口
	result, err := rs.transport.CallRaw(ctx, "wes_getResourceByContentHash", []interface{}{contentHashHex})
	if err != nil {
		return &FetchResult{
			Success: false,
			Message: fmt.Sprintf("查询资源失败: %v", err),
		}, fmt.Errorf("调用 wes_getResourceByContentHash 失败: %w", err)
	}

	// 解析返回结果
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return &FetchResult{
			Success: false,
			Message: fmt.Sprintf("返回格式错误: %T", result),
		}, fmt.Errorf("wes_getResourceByContentHash 返回格式错误: %T", result)
	}

	// 检查 success 字段（如果有）
	if successVal, ok := resultMap["success"].(bool); ok && !successVal {
		message := "查询失败"
		if msgVal, ok := resultMap["message"].(string); ok {
			message = msgVal
		}
		return &FetchResult{
			Success: false,
			Message: message,
		}, fmt.Errorf("资源查询失败: %s", message)
	}

	// 提取资源元数据
	resourceName := ""
	if nameVal, ok := resultMap["name"].(string); ok {
		resourceName = nameVal
	}

	resourceType := ""
	if resourceTypeVal, ok := resultMap["resourceType"].(string); ok {
		resourceType = resourceTypeVal
	} else if categoryVal, ok := resultMap["category"].(string); ok {
		// 兼容 category 字段
		resourceType = categoryVal
	}

	// 构建完整的元数据映射
	metadata := make(map[string]interface{})
	for k, v := range resultMap {
		metadata[k] = v
	}

	return &FetchResult{
		Success:      true,
		Data:         nil, // 元数据查询不返回原始字节
		ResourceName: resourceName,
		ResourceType: resourceType,
		Message:      "资源元数据获取成功",
		Metadata:     metadata,
	}, nil
}

// ========== 辅助方法 ==========

// generateProofs 生成解锁证明
func (rs *ResourceService) generateProofs(composed *builder.ComposedTx) []builder.UnlockingProof {
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
func (rs *ResourceService) signTransaction(
	ctx context.Context,
	proven *builder.ProvenTx,
	fromAddress string,
	privateKey []byte,
) (*builder.SignedTx, error) {
	signers := make(map[string]string)
	txID := proven.TxID()

	signature, err := (*rs.signer).SignHash([]byte(txID), fromAddress)
	if err != nil {
		return nil, fmt.Errorf("sign hash with address %s: %w", fromAddress, err)
	}

	signers[fromAddress] = string(signature)

	signed, err := proven.Sign(rs.transport, signers)
	if err != nil {
		return nil, fmt.Errorf("create signed tx: %w", err)
	}

	return signed, nil
}

// extractDeployInfo 从Draft中提取部署信息
func (rs *ResourceService) extractDeployInfo(draft *builder.DraftTx) (resourceAddress, fee string) {
	if draft.GetParams().Extra != nil {
		if addr, ok := draft.GetParams().Extra["resource_address"].(string); ok {
			resourceAddress = addr
		}
		if feeVal, ok := draft.GetParams().Extra["estimated_fee"].(string); ok {
			fee = feeVal
		}
	}
	return resourceAddress, fee
}

// validateDeployRequest 验证部署请求
func (rs *ResourceService) validateDeployRequest(req *DeployRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.FilePath == "" {
		return fmt.Errorf("file_path is empty")
	}

	if req.Deployer == "" {
		return fmt.Errorf("deployer is empty")
	}

	if len(req.PrivateKey) == 0 {
		return fmt.Errorf("private_key is empty")
	}

	return nil
}
