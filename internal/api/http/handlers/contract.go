// Package handlers 提供HTTP API处理器
//
// contract.go 实现智能合约相关的HTTP API端点
//
// 🎯 **现代化合约API架构**
//
// 本文件严格按照 pkg/interfaces/blockchain 中实际存在的接口实现，
// 提供简洁、类型安全的合约管理API。

package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 🎯 请求响应结构定义 ====================

// DeployContractRequest 部署合约请求
type DeployContractRequest struct {
	DeployerPrivateKey string                            `json:"deployer_private_key" binding:"required"`
	ContractFilePath   string                            `json:"contract_file_path" binding:"required"`
	Config             *resource.ContractExecutionConfig `json:"config" binding:"required"`
	Name               string                            `json:"name" binding:"required"`
	Description        string                            `json:"description,omitempty"`
	Options            *types.ResourceDeployOptions      `json:"options,omitempty"`
}

// CallContractRequest 调用合约请求
type CallContractRequest struct {
	CallerPrivateKey  string                 `json:"caller_private_key" binding:"required"`
	ContractAddress   string                 `json:"contract_address" binding:"required"`
	MethodName        string                 `json:"method_name" binding:"required"`
	Parameters        map[string]interface{} `json:"parameters,omitempty"`
	ExecutionFeeLimit uint64                 `json:"execution_fee_limit" binding:"required"`
	Value             string                 `json:"value,omitempty"`
	Options           *types.TransferOptions `json:"options,omitempty"`
}

// ContractResponse 合约响应
type ContractResponse struct {
	Success         bool   `json:"success"`
	TransactionHash string `json:"transaction_hash"`
	Message         string `json:"message"`
}

// DeployStaticResourceRequest 部署静态资源请求
type DeployStaticResourceRequest struct {
	DeployerPrivateKey string                       `json:"deployer_private_key" binding:"required"`
	FilePath           string                       `json:"file_path" binding:"required"`
	Name               string                       `json:"name" binding:"required"`
	Description        string                       `json:"description,omitempty"`
	Tags               []string                     `json:"tags,omitempty"`
	Options            *types.ResourceDeployOptions `json:"options,omitempty"`
}

// DeployAIModelRequest 部署AI模型请求
type DeployAIModelRequest struct {
	DeployerPrivateKey string                           `json:"deployer_private_key" binding:"required"`
	ModelFilePath      string                           `json:"model_file_path" binding:"required"`
	Config             *resource.AIModelExecutionConfig `json:"config" binding:"required"`
	Name               string                           `json:"name" binding:"required"`
	Description        string                           `json:"description,omitempty"`
	Options            *types.ResourceDeployOptions     `json:"options,omitempty"`
}

// InferAIModelRequest AI模型推理请求
type InferAIModelRequest struct {
	CallerPrivateKey string                 `json:"caller_private_key" binding:"required"`
	ModelAddress     string                 `json:"model_address" binding:"required"`
	InputData        interface{}            `json:"input_data" binding:"required"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Options          *types.TransferOptions `json:"options,omitempty"`
}

// ==================== 🏗️ 合约API处理器 ====================

// ContractHandler 智能合约HTTP处理器
type ContractHandler struct {
	contractService    blockchain.ContractService
	transactionService blockchain.TransactionService
	transactionManager blockchain.TransactionManager
	aiModelService     blockchain.AIModelService
	logger             log.Logger
}

// NewContractHandler 创建合约处理器实例
func NewContractHandler(
	contractService blockchain.ContractService,
	transactionService blockchain.TransactionService,
	transactionManager blockchain.TransactionManager,
	aiModelService blockchain.AIModelService,
	logger log.Logger,
) *ContractHandler {
	return &ContractHandler{
		contractService:    contractService,
		transactionService: transactionService,
		transactionManager: transactionManager,
		aiModelService:     aiModelService,
		logger:             logger,
	}
}

// ==================== 🎯 核心API方法实现 ====================

// DeployContract 部署智能合约
//
// 📌 **接口说明**：部署WASM智能合约到区块链网络
//
// **HTTP Method**: `POST`
// **URL Path**: `/contracts/deploy`
//
// **请求体参数**：
//   - deployer_private_key (string, required): 部署者私钥，十六进制格式
//   - contract_file_path (string, required): 合约WASM文件路径
//   - config (object, required): 合约执行配置
//   - name (string, required): 合约显示名称
//   - description (string, optional): 合约功能描述
//   - options (object, optional): 高级部署选项
//
// **请求体示例**：
//
//	{
//	  "deployer_private_key": "1234567890abcdef...",
//	  "contract_file_path": "/path/to/contract.wasm",
//	  "config": {
//	    "max_执行费用_limit": 1000000,
//	    "max_memory_pages": 256,
//	    "timeout": 30
//	  },
//	  "name": "去中心化投票系统",
//	  "description": "基于区块链的透明投票合约"
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "transaction_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	  "message": "合约部署交易已成功创建"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "合约部署服务暂时不可用"
//	}
//
// 💡 **使用说明**：
// - 支持基础模式（公开合约）和高级模式（私有合约、付费合约）
// - 返回的transaction_hash需要签名和提交才能完成部署
// - 部署成功后可通过CallContract调用合约方法
func (h *ContractHandler) DeployContract(c *gin.Context) {
	h.logger.Info("开始处理合约部署请求")

	var req DeployContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析合约部署参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.DeployerPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查合约服务是否可用
	if h.contractService == nil {
		h.logger.Error("ContractService 服务不可用")
		c.JSON(http.StatusServiceUnavailable, ContractResponse{
			Success: false,
			Message: "合约部署服务暂时不可用",
		})
		return
	}

	// 调用合约服务部署合约
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	txHash, err := h.contractService.DeployContract(
		ctx,
		privateKey,
		req.ContractFilePath,
		req.Config,
		req.Name,
		req.Description,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("合约部署失败: %v", err)
		c.JSON(http.StatusInternalServerError, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("合约部署失败: %v", err),
		})
		return
	}

	h.logger.Infof("合约部署成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, ContractResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "合约部署交易已成功创建",
	})
}

// CallContract 调用智能合约
//
// 📌 **接口说明**：调用已部署的智能合约方法
//
// **HTTP Method**: `POST`
// **URL Path**: `/contracts/call`
//
// **请求体参数**：
//   - caller_private_key (string, required): 调用者私钥，十六进制格式
//   - contract_address (string, required): 合约地址
//   - method_name (string, required): 要调用的方法名
//   - parameters (object, optional): 方法参数，JSON格式
//   - 执行费用_limit (number, required): 执行费用限制
//   - value (string, optional): 发送的代币数量
//   - options (object, optional): 高级调用选项
//
// **请求体示例**：
//
//	{
//	  "caller_private_key": "1234567890abcdef...",
//	  "contract_address": "0xabcdef123456789abcdef123456789abcdef123456",
//	  "method_name": "transfer",
//	  "parameters": {
//	    "to": "0x123...",
//	    "amount": "100"
//	  },
//	  "执行费用_limit": 500000,
//	  "value": "0"
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "transaction_hash": "b2c3d4e5f6789012345678901234567890abcdef12",
//	  "message": "合约调用交易已成功创建"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "合约调用服务暂时不可用"
//	}
//
// 💡 **使用说明**：
// - 用于执行合约业务逻辑：代币转账、投票、查询等
// - 支持基础模式和企业级模式（委托、多签、定时调用）
// - 返回的transaction_hash需要签名和提交才能执行
func (h *ContractHandler) CallContract(c *gin.Context) {
	h.logger.Info("开始处理合约调用请求")

	var req CallContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析合约调用参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.CallerPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查合约服务是否可用
	if h.contractService == nil {
		h.logger.Error("ContractService 服务不可用")
		c.JSON(http.StatusServiceUnavailable, ContractResponse{
			Success: false,
			Message: "合约调用服务暂时不可用",
		})
		return
	}

	// 调用合约服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	txHash, err := h.contractService.CallContract(
		ctx,
		privateKey,
		req.ContractAddress,
		req.MethodName,
		req.Parameters,
		req.ExecutionFeeLimit,
		req.Value,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("合约调用失败: %v", err)
		c.JSON(http.StatusInternalServerError, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("合约调用失败: %v", err),
		})
		return
	}

	h.logger.Infof("合约调用成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, ContractResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "合约调用交易已成功创建",
	})
}

// DeployStaticResource 部署静态资源
//
// POST /resources/deploy
func (h *ContractHandler) DeployStaticResource(c *gin.Context) {
	h.logger.Info("开始处理静态资源部署请求")

	var req DeployStaticResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析静态资源部署参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.DeployerPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 调用交易服务部署静态资源
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	txHash, err := h.transactionService.DeployStaticResource(
		ctx,
		privateKey,
		req.FilePath,
		req.Name,
		req.Description,
		req.Tags,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("静态资源部署失败: %v", err)
		c.JSON(http.StatusInternalServerError, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("静态资源部署失败: %v", err),
		})
		return
	}

	h.logger.Infof("静态资源部署成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, ContractResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "静态资源部署交易已成功创建",
	})
}

// DeployAIModel 部署AI模型
//
// POST /ai/deploy
func (h *ContractHandler) DeployAIModel(c *gin.Context) {
	h.logger.Info("开始处理AI模型部署请求")

	var req DeployAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析AI模型部署参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.DeployerPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查AI模型服务是否可用
	if h.aiModelService == nil {
		h.logger.Error("AIModelService 服务不可用")
		c.JSON(http.StatusServiceUnavailable, ContractResponse{
			Success: false,
			Message: "AI模型部署服务暂时不可用",
		})
		return
	}

	// 调用AI模型服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second) // AI模型部署需要更长时间
	defer cancel()

	txHash, err := h.aiModelService.DeployAIModel(
		ctx,
		privateKey,
		req.ModelFilePath,
		req.Config,
		req.Name,
		req.Description,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("AI模型部署失败: %v", err)
		c.JSON(http.StatusInternalServerError, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("AI模型部署失败: %v", err),
		})
		return
	}

	h.logger.Infof("AI模型部署成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, ContractResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "AI模型部署交易已成功创建",
	})
}

// InferAIModel AI模型推理
//
// POST /ai/infer
func (h *ContractHandler) InferAIModel(c *gin.Context) {
	h.logger.Info("开始处理AI模型推理请求")

	var req InferAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析AI模型推理参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.CallerPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ContractResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查AI模型服务是否可用
	if h.aiModelService == nil {
		h.logger.Error("AIModelService 服务不可用")
		c.JSON(http.StatusServiceUnavailable, ContractResponse{
			Success: false,
			Message: "AI模型推理服务暂时不可用",
		})
		return
	}

	// 调用AI模型服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	txHash, err := h.aiModelService.InferAIModel(
		ctx,
		privateKey,
		req.ModelAddress,
		req.InputData,
		req.Parameters,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("AI模型推理失败: %v", err)
		c.JSON(http.StatusInternalServerError, ContractResponse{
			Success: false,
			Message: fmt.Sprintf("AI模型推理失败: %v", err),
		})
		return
	}

	h.logger.Infof("AI模型推理成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, ContractResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "AI模型推理交易已成功创建",
	})
}
