// Package handlers 提供HTTP API处理器
//
// transaction.go 实现交易管理相关的HTTP API端点
//
// 🎯 **现代化交易API架构**
//
// 本文件严格按照 pkg/types 和 pkg/interfaces 中实际存在的类型和接口实现，
// 提供简洁、类型安全的交易管理API。
//
// 📋 **支持的API端点**
// - POST /transactions/transfer        - 基础转账
// - POST /transactions/batch-transfer  - 批量转账
// - POST /transactions/sign           - 签名交易
// - POST /transactions/submit         - 提交交易
// - GET  /transactions/status/:txHash - 查询交易状态
// - GET  /transactions/:txHash        - 获取交易详情
// - POST /transactions/estimate-fee   - 估算交易费用
// - POST /transactions/validate       - 验证交易
//
// 🔐 **企业级多签工作流API**
// - POST /transactions/multisig/start              - 开始多签会话
// - POST /transactions/multisig/:sessionID/sign    - 添加签名到会话
// - GET  /transactions/multisig/:sessionID/status  - 获取会话状态
// - POST /transactions/multisig/:sessionID/finalize - 完成多签会话

package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 🎯 请求响应结构定义 ====================

// TransferRequest 基础转账请求
type TransferRequest struct {
	SenderPrivateKey string                 `json:"sender_private_key" binding:"required"`
	ToAddress        string                 `json:"to_address" binding:"required"`
	Amount           string                 `json:"amount" binding:"required"`
	TokenID          string                 `json:"token_id,omitempty"` // 空字符串表示原生币
	Memo             string                 `json:"memo,omitempty"`
	Options          *types.TransferOptions `json:"options,omitempty"`
}

// BatchTransferRequest 批量转账请求
type BatchTransferRequest struct {
	SenderPrivateKey string                 `json:"sender_private_key" binding:"required"`
	Transfers        []types.TransferParams `json:"transfers" binding:"required"`
}

// TransferResponse 转账响应
type TransferResponse struct {
	Success         bool   `json:"success"`
	TransactionHash string `json:"transaction_hash"`
	Message         string `json:"message"`
}

// SignTransactionRequest 签名交易请求
type SignTransactionRequest struct {
	TransactionHash string `json:"transaction_hash" binding:"required"`
	PrivateKey      string `json:"private_key" binding:"required"`
}

// SignTransactionResponse 签名交易响应
type SignTransactionResponse struct {
	Success      bool   `json:"success"`
	SignedTxHash string `json:"signed_tx_hash"`
	Message      string `json:"message"`
}

// SubmitTransactionRequest 提交交易请求
type SubmitTransactionRequest struct {
	SignedTxHash string `json:"signed_tx_hash" binding:"required"`
}

// SubmitTransactionResponse 提交交易响应
type SubmitTransactionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// EstimateFeeRequest 费用估算请求
type EstimateFeeRequest struct {
	TransactionHash string `json:"transaction_hash" binding:"required"`
}

// EstimateFeeResponse 费用估算响应
type EstimateFeeResponse struct {
	Success      bool   `json:"success"`
	EstimatedFee uint64 `json:"estimated_fee"`
	Message      string `json:"message"`
}

// ValidateTransactionRequest 交易验证请求
type ValidateTransactionRequest struct {
	TransactionHash string `json:"transaction_hash" binding:"required"`
}

// ValidateTransactionResponse 交易验证响应
type ValidateTransactionResponse struct {
	Success bool   `json:"success"`
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// TransactionStatusResponse 交易状态响应
type TransactionStatusResponse struct {
	Success bool                        `json:"success"`
	Status  types.TransactionStatusEnum `json:"status"`
	Message string                      `json:"message"`
}

// TransactionDetailsResponse 交易详情响应
type TransactionDetailsResponse struct {
	Success     bool                     `json:"success"`
	Transaction *transaction.Transaction `json:"transaction,omitempty"`
	Message     string                   `json:"message"`
}

// StartMultiSigSessionRequest 开始多签会话请求
type StartMultiSigSessionRequest struct {
	RequiredSignatures uint32        `json:"required_signatures" binding:"required"`
	AuthorizedSigners  []string      `json:"authorized_signers" binding:"required"`
	ExpiryDuration     time.Duration `json:"expiry_duration" binding:"required"`
	Description        string        `json:"description,omitempty"`
}

// StartMultiSigSessionResponse 开始多签会话响应
type StartMultiSigSessionResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// AddMultiSigSignatureRequest 添加多签签名请求
type AddMultiSigSignatureRequest struct {
	Signature *types.MultiSigSignature `json:"signature" binding:"required"`
}

// AddMultiSigSignatureResponse 添加多签签名响应
type AddMultiSigSignatureResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// MultiSigSessionStatusResponse 多签会话状态响应
type MultiSigSessionStatusResponse struct {
	Success bool                   `json:"success"`
	Session *types.MultiSigSession `json:"session,omitempty"`
	Message string                 `json:"message"`
}

// FinalizeMultiSigSessionResponse 完成多签会话响应
type FinalizeMultiSigSessionResponse struct {
	Success     bool   `json:"success"`
	FinalTxHash string `json:"final_tx_hash"`
	Message     string `json:"message"`
}

// FetchStaticResourceRequest 获取静态资源文件请求
type FetchStaticResourceRequest struct {
	ContentHash         string `json:"content_hash" binding:"required"`
	RequesterPrivateKey string `json:"requester_private_key" binding:"required"`
	TargetDir           string `json:"target_dir,omitempty"`
}

// FetchStaticResourceResponse 获取静态资源文件响应
type FetchStaticResourceResponse struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path,omitempty"`
	Message  string `json:"message"`
}

// ==================== 🏗️ 交易管理API处理器 ====================

// TransactionHandlers 交易处理器
type TransactionHandlers struct {
	transactionService blockchain.TransactionService
	transactionManager blockchain.TransactionManager
	contractService    blockchain.ContractService
	aiModelService     blockchain.AIModelService
	logger             log.Logger
}

// NewTransactionHandlers 创建交易处理器实例
func NewTransactionHandlers(
	transactionService blockchain.TransactionService,
	transactionManager blockchain.TransactionManager, // 可以为 nil
	contractService blockchain.ContractService, // 可以为 nil
	aiModelService blockchain.AIModelService, // 可以为 nil
	logger log.Logger,
) *TransactionHandlers {
	return &TransactionHandlers{
		transactionService: transactionService,
		transactionManager: transactionManager,
		contractService:    contractService,
		aiModelService:     aiModelService,
		logger:             logger,
	}
}

// ==================== 🎯 核心API方法实现 ====================

// Transfer 基础转账操作
//
// 📌 **接口说明**：执行基础和高级模式的资产转账操作
//
// **HTTP Method**: `POST`
// **URL Path**: `/transactions/transfer`
//
// **请求体参数**：
//   - sender_private_key (string, required): 发送方私钥，十六进制格式
//   - to_address (string, required): 接收方地址
//   - amount (string, required): 转账金额，支持小数
//   - token_id (string, optional): 代币标识，空字符串表示原生币
//   - memo (string, optional): 转账备注
//   - options (object, optional): 高级转账选项
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "transaction_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	  "message": "转账交易已成功创建"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "私钥格式错误，请使用十六进制格式"
//	}
//
// 💡 **使用说明**：
// - 基础模式：只需要基本参数，系统自动处理UTXO选择和费用计算
// - 高级模式：通过options参数支持多签、定时、委托等企业级功能
// - 返回的transaction_hash用于后续的签名和提交操作
func (h *TransactionHandlers) Transfer(c *gin.Context) {
	// 获取客户端标识用于跨终端日志追踪
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	requestID := fmt.Sprintf("API-%d", time.Now().UnixNano())

	h.logger.Infof("🌐 [%s] 开始处理转账请求 - ClientIP: %s, UserAgent: %s", requestID, clientIP, userAgent)

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("🌐 [%s] 解析转账参数失败: %v", requestID, err)
		c.JSON(http.StatusBadRequest, TransferResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 记录详细的转账请求参数（隐藏私钥）
	h.logger.Infof("🌐 [%s] 转账请求详情: ToAddress=%s, Amount=%s, TokenID=%s, Memo=%s",
		requestID, req.ToAddress, req.Amount, req.TokenID, req.Memo)

	// 解析私钥
	privateKey, err := hex.DecodeString(req.SenderPrivateKey)
	if err != nil {
		h.logger.Errorf("🌐 [%s] ❌ 私钥格式错误: %v", requestID, err)
		c.JSON(http.StatusBadRequest, TransferResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	h.logger.Infof("🌐 [%s] ✅ 私钥解析成功，长度: %d字节", requestID, len(privateKey))

	// 调用交易服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	h.logger.Infof("🌐 [%s] 🔄 开始调用交易服务进行转账", requestID)

	txHash, err := h.transactionService.TransferAsset(
		ctx,
		privateKey,
		req.ToAddress,
		req.Amount,
		req.TokenID,
		req.Memo,
		req.Options,
	)
	if err != nil {
		h.logger.Errorf("🌐 [%s] ❌ 转账失败: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, TransferResponse{
			Success: false,
			Message: fmt.Sprintf("转账失败: %v", err),
		})
		return
	}

	h.logger.Infof("🌐 [%s] ✅ 转账成功，交易哈希: %x", requestID, txHash)
	h.logger.Infof("🌐 [%s] 📤 返回响应给客户端", requestID)

	c.JSON(http.StatusOK, TransferResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         "转账交易已成功创建",
	})
}

// BatchTransfer 批量转账操作
//
// 📌 **接口说明**：一次性处理多笔转账，降低手续费
//
// **HTTP Method**: `POST`
// **URL Path**: `/transactions/batch-transfer`
//
// **请求体参数**：
//   - sender_private_key (string, required): 发送方私钥，十六进制格式
//   - transfers (array, required): 转账参数列表，每个包含to_address、amount、token_id、memo
//
// **请求体示例**：
//
//	{
//	  "sender_private_key": "1234567890abcdef...",
//	  "transfers": [
//	    {
//	      "to_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
//	      "amount": "100.0",
//	      "token_id": "",
//	      "memo": "工资发放"
//	    },
//	    {
//	      "to_address": "DfA8Bks2QnEUeykiJJgrAtKPNPrAzPdPmT",
//	      "amount": "200.0",
//	      "token_id": "",
//	      "memo": "奖金发放"
//	    }
//	  ]
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "transaction_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	  "message": "批量转账交易已成功创建，共 2 笔转账"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "批量转账失败: 余额不足"
//	}
//
// 💡 **使用说明**：
// - 适用场景：工资发放、红包分发、空投发放、批量退款
// - 优化特性：UTXO批量选择优化、手续费分摊计算、原子性保证
// - 最多支持1000笔转账
func (h *TransactionHandlers) BatchTransfer(c *gin.Context) {
	h.logger.Info("开始处理批量转账请求")

	var req BatchTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析批量转账参数失败: %v", err)
		c.JSON(http.StatusBadRequest, TransferResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析私钥
	privateKey, err := hex.DecodeString(req.SenderPrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, TransferResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 调用批量转账服务
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	txHash, err := h.transactionService.BatchTransfer(
		ctx,
		privateKey,
		req.Transfers,
	)
	if err != nil {
		h.logger.Errorf("批量转账失败: %v", err)
		c.JSON(http.StatusInternalServerError, TransferResponse{
			Success: false,
			Message: fmt.Sprintf("批量转账失败: %v", err),
		})
		return
	}

	h.logger.Infof("批量转账成功，交易哈希: %x", txHash)
	c.JSON(http.StatusOK, TransferResponse{
		Success:         true,
		TransactionHash: hex.EncodeToString(txHash),
		Message:         fmt.Sprintf("批量转账交易已成功创建，共 %d 笔转账", len(req.Transfers)),
	})
}

// SignTransaction 签名交易
//
// 📌 **接口说明**：对未签名的交易进行数字签名，使其可以提交到网络
//
// **HTTP Method**: `POST`
// **URL Path**: `/transactions/sign`
//
// **请求体参数**：
//   - transaction_hash (string, required): 未签名交易哈希，十六进制格式
//   - private_key (string, required): 用户私钥，十六进制格式
//
// **请求体示例**：
//
//	{
//	  "transaction_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	  "private_key": "1234567890abcdef1234567890abcdef12345678901234567890abcdef1234567890"
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "signed_tx_hash": "b2c3d4e5f6789012345678901234567890abcdef12",
//	  "message": "交易签名成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "交易签名服务暂时不可用"
//	}
//
// 💡 **使用说明**：
// - 交易哈希来自Transfer或BatchTransfer接口的返回值
// - 私钥用于数字签名，确保交易授权
// - 返回的signed_tx_hash用于SubmitTransaction接口
func (h *TransactionHandlers) SignTransaction(c *gin.Context) {
	h.logger.Info("开始处理交易签名请求")

	var req SignTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析签名参数失败: %v", err)
		c.JSON(http.StatusBadRequest, SignTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析交易哈希和私钥
	txHash, err := hex.DecodeString(req.TransactionHash)
	if err != nil {
		h.logger.Errorf("交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, SignTransactionResponse{
			Success: false,
			Message: "交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	privateKey, err := hex.DecodeString(req.PrivateKey)
	if err != nil {
		h.logger.Errorf("私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, SignTransactionResponse{
			Success: false,
			Message: "私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, SignTransactionResponse{
			Success: false,
			Message: "交易签名服务暂时不可用",
		})
		return
	}

	// 调用交易管理器签名
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	signedTxHash, err := h.transactionManager.SignTransaction(ctx, txHash, privateKey)
	if err != nil {
		h.logger.Errorf("交易签名失败: %v", err)
		c.JSON(http.StatusInternalServerError, SignTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("交易签名失败: %v", err),
		})
		return
	}

	h.logger.Infof("交易签名成功，签名交易哈希: %x", signedTxHash)
	c.JSON(http.StatusOK, SignTransactionResponse{
		Success:      true,
		SignedTxHash: hex.EncodeToString(signedTxHash),
		Message:      "交易签名成功",
	})
}

// SubmitTransaction 提交交易
//
// 📌 **接口说明**：将已签名的交易提交到区块链网络
//
// **HTTP Method**: `POST`
// **URL Path**: `/transactions/submit`
//
// **请求体参数**：
//   - signed_tx_hash (string, required): 已签名交易哈希，十六进制格式
//
// **请求体示例**：
//
//	{
//	  "signed_tx_hash": "b2c3d4e5f6789012345678901234567890abcdef12"
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "message": "交易已成功提交到网络"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "交易提交失败: 网络连接超时"
//	}
//
// 💡 **使用说明**：
// - signed_tx_hash来自SignTransaction接口的返回值
// - 提交后交易进入内存池等待矿工打包
// - 使用GetTransactionStatus查询交易确认状态
func (h *TransactionHandlers) SubmitTransaction(c *gin.Context) {
	h.logger.Info("开始处理交易提交请求")

	var req SubmitTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析提交参数失败: %v", err)
		c.JSON(http.StatusBadRequest, SubmitTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析签名交易哈希
	signedTxHash, err := hex.DecodeString(req.SignedTxHash)
	if err != nil {
		h.logger.Errorf("签名交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, SubmitTransactionResponse{
			Success: false,
			Message: "签名交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, SubmitTransactionResponse{
			Success: false,
			Message: "交易提交服务暂时不可用",
		})
		return
	}

	// 调用交易管理器提交
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err = h.transactionManager.SubmitTransaction(ctx, signedTxHash)
	if err != nil {
		h.logger.Errorf("交易提交失败: %v", err)
		c.JSON(http.StatusInternalServerError, SubmitTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("交易提交失败: %v", err),
		})
		return
	}

	h.logger.Infof("交易提交成功，签名交易哈希: %x", signedTxHash)
	c.JSON(http.StatusOK, SubmitTransactionResponse{
		Success: true,
		Message: "交易已成功提交到网络",
	})
}

// GetTransactionStatus 查询交易状态
//
// 📌 **接口说明**：查询交易在区块链中的确认状态
//
// **HTTP Method**: `GET`
// **URL Path**: `/transactions/status/{txHash}`
//
// **路径参数**：
//   - txHash (string, required): 交易哈希，十六进制格式
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "status": "confirmed",
//	  "message": "交易状态: confirmed"
//	}
//
// **状态值说明**：
//   - "pending": 交易在内存池中等待确认
//   - "confirmed": 交易已被打包到区块
//   - "failed": 交易执行失败
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "查询交易状态失败: 交易不存在"
//	}
//
// 💡 **使用说明**：
// - txHash可以是签名前或签名后的交易哈希
// - 用于监控交易确认进度
// - 建议每2-5秒轮询一次直到状态变为confirmed或failed
func (h *TransactionHandlers) GetTransactionStatus(c *gin.Context) {
	txHashStr := c.Param("txHash")
	if txHashStr == "" {
		c.JSON(http.StatusBadRequest, TransactionStatusResponse{
			Success: false,
			Message: "交易哈希参数缺失",
		})
		return
	}

	// 解析交易哈希
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		h.logger.Errorf("交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, TransactionStatusResponse{
			Success: false,
			Message: "交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, TransactionStatusResponse{
			Success: false,
			Message: "交易状态查询服务暂时不可用",
		})
		return
	}

	// 调用交易管理器查询状态
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status, err := h.transactionManager.GetTransactionStatus(ctx, txHash)
	if err != nil {
		h.logger.Errorf("查询交易状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, TransactionStatusResponse{
			Success: false,
			Message: fmt.Sprintf("查询交易状态失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, TransactionStatusResponse{
		Success: true,
		Status:  status,
		Message: fmt.Sprintf("交易状态: %s", status),
	})
}

// GetTransactionDetails 获取交易详情
//
// 📌 **接口说明**：获取交易的完整详细信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/transactions/{txHash}`
//
// **路径参数**：
//   - txHash (string, required): 交易哈希，十六进制格式
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "transaction": {
//	    "hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	    "inputs": [...],
//	    "outputs": [...],
//	    "signatures": [...],
//	    "timestamp": 1640995200
//	  },
//	  "message": "交易详情获取成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "查询交易详情失败: 交易不存在"
//	}
//
// 💡 **使用说明**：
// - 返回完整的protobuf交易结构
// - 包含交易输入输出详情、锁定条件和解锁证明
// - 主要用于调试和详细分析
func (h *TransactionHandlers) GetTransactionDetails(c *gin.Context) {
	txHashStr := c.Param("txHash")
	if txHashStr == "" {
		c.JSON(http.StatusBadRequest, TransactionDetailsResponse{
			Success: false,
			Message: "交易哈希参数缺失",
		})
		return
	}

	// 解析交易哈希
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		h.logger.Errorf("交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, TransactionDetailsResponse{
			Success: false,
			Message: "交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, TransactionDetailsResponse{
			Success: false,
			Message: "交易详情查询服务暂时不可用",
		})
		return
	}

	// 调用交易管理器查询详情
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	tx, err := h.transactionManager.GetTransaction(ctx, txHash)
	if err != nil {
		h.logger.Errorf("查询交易详情失败: %v", err)
		c.JSON(http.StatusInternalServerError, TransactionDetailsResponse{
			Success: false,
			Message: fmt.Sprintf("查询交易详情失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, TransactionDetailsResponse{
		Success:     true,
		Transaction: tx,
		Message:     "交易详情获取成功",
	})
}

// EstimateTransactionFee 估算交易费用
//
// POST /transactions/estimate-fee
func (h *TransactionHandlers) EstimateTransactionFee(c *gin.Context) {
	h.logger.Info("开始处理费用估算请求")

	var req EstimateFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析费用估算参数失败: %v", err)
		c.JSON(http.StatusBadRequest, EstimateFeeResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析交易哈希
	txHash, err := hex.DecodeString(req.TransactionHash)
	if err != nil {
		h.logger.Errorf("交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, EstimateFeeResponse{
			Success: false,
			Message: "交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, EstimateFeeResponse{
			Success: false,
			Message: "费用估算服务暂时不可用",
		})
		return
	}

	// 调用交易管理器估算费用
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	fee, err := h.transactionManager.EstimateTransactionFee(ctx, txHash)
	if err != nil {
		h.logger.Errorf("估算交易费用失败: %v", err)
		c.JSON(http.StatusInternalServerError, EstimateFeeResponse{
			Success: false,
			Message: fmt.Sprintf("估算交易费用失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, EstimateFeeResponse{
		Success:      true,
		EstimatedFee: fee,
		Message:      fmt.Sprintf("预估费用: %d", fee),
	})
}

// ValidateTransaction 验证交易
//
// POST /transactions/validate
func (h *TransactionHandlers) ValidateTransaction(c *gin.Context) {
	h.logger.Info("开始处理交易验证请求")

	var req ValidateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析交易验证参数失败: %v", err)
		c.JSON(http.StatusBadRequest, ValidateTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析交易哈希
	txHash, err := hex.DecodeString(req.TransactionHash)
	if err != nil {
		h.logger.Errorf("交易哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, ValidateTransactionResponse{
			Success: false,
			Message: "交易哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, ValidateTransactionResponse{
			Success: false,
			Message: "交易验证服务暂时不可用",
		})
		return
	}

	// 调用交易管理器验证交易
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	valid, err := h.transactionManager.ValidateTransaction(ctx, txHash)
	if err != nil {
		h.logger.Errorf("验证交易失败: %v", err)
		c.JSON(http.StatusInternalServerError, ValidateTransactionResponse{
			Success: false,
			Message: fmt.Sprintf("验证交易失败: %v", err),
		})
		return
	}

	message := "交易验证通过"
	if !valid {
		message = "交易验证失败"
	}

	c.JSON(http.StatusOK, ValidateTransactionResponse{
		Success: true,
		Valid:   valid,
		Message: message,
	})
}

// ==================== 🔐 多签工作流API ====================

// StartMultiSigSession 开始多签会话
//
// POST /transactions/multisig/start
func (h *TransactionHandlers) StartMultiSigSession(c *gin.Context) {
	h.logger.Info("开始处理多签会话创建请求")

	var req StartMultiSigSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析多签会话参数失败: %v", err)
		c.JSON(http.StatusBadRequest, StartMultiSigSessionResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, StartMultiSigSessionResponse{
			Success: false,
			Message: "多签会话服务暂时不可用",
		})
		return
	}

	// 调用交易管理器创建多签会话
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	sessionID, err := h.transactionManager.StartMultiSigSession(
		ctx,
		req.RequiredSignatures,
		req.AuthorizedSigners,
		req.ExpiryDuration,
		req.Description,
	)
	if err != nil {
		h.logger.Errorf("创建多签会话失败: %v", err)
		c.JSON(http.StatusInternalServerError, StartMultiSigSessionResponse{
			Success: false,
			Message: fmt.Sprintf("创建多签会话失败: %v", err),
		})
		return
	}

	h.logger.Infof("多签会话创建成功，会话ID: %s", sessionID)
	c.JSON(http.StatusOK, StartMultiSigSessionResponse{
		Success:   true,
		SessionID: sessionID,
		Message:   "多签会话创建成功",
	})
}

// AddMultiSigSignature 添加多签签名
//
// POST /transactions/multisig/:sessionID/sign
func (h *TransactionHandlers) AddMultiSigSignature(c *gin.Context) {
	sessionID := c.Param("sessionID")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, AddMultiSigSignatureResponse{
			Success: false,
			Message: "会话ID参数缺失",
		})
		return
	}

	var req AddMultiSigSignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析多签签名参数失败: %v", err)
		c.JSON(http.StatusBadRequest, AddMultiSigSignatureResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, AddMultiSigSignatureResponse{
			Success: false,
			Message: "多签签名服务暂时不可用",
		})
		return
	}

	// 调用交易管理器添加签名
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err := h.transactionManager.AddSignatureToMultiSigSession(ctx, sessionID, req.Signature)
	if err != nil {
		h.logger.Errorf("添加多签签名失败: %v", err)
		c.JSON(http.StatusInternalServerError, AddMultiSigSignatureResponse{
			Success: false,
			Message: fmt.Sprintf("添加多签签名失败: %v", err),
		})
		return
	}

	h.logger.Infof("多签签名添加成功，会话ID: %s", sessionID)
	c.JSON(http.StatusOK, AddMultiSigSignatureResponse{
		Success: true,
		Message: "签名已成功添加到多签会话",
	})
}

// GetMultiSigSessionStatus 获取多签会话状态
//
// GET /transactions/multisig/:sessionID/status
func (h *TransactionHandlers) GetMultiSigSessionStatus(c *gin.Context) {
	sessionID := c.Param("sessionID")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, MultiSigSessionStatusResponse{
			Success: false,
			Message: "会话ID参数缺失",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, MultiSigSessionStatusResponse{
			Success: false,
			Message: "多签会话状态查询服务暂时不可用",
		})
		return
	}

	// 调用交易管理器查询会话状态
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	session, err := h.transactionManager.GetMultiSigSessionStatus(ctx, sessionID)
	if err != nil {
		h.logger.Errorf("查询多签会话状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, MultiSigSessionStatusResponse{
			Success: false,
			Message: fmt.Sprintf("查询多签会话状态失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, MultiSigSessionStatusResponse{
		Success: true,
		Session: session,
		Message: "多签会话状态获取成功",
	})
}

// FinalizeMultiSigSession 完成多签会话
//
// POST /transactions/multisig/:sessionID/finalize
func (h *TransactionHandlers) FinalizeMultiSigSession(c *gin.Context) {
	sessionID := c.Param("sessionID")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, FinalizeMultiSigSessionResponse{
			Success: false,
			Message: "会话ID参数缺失",
		})
		return
	}

	// 检查交易管理器是否可用
	if h.transactionManager == nil {
		h.logger.Error("TransactionManager 服务不可用")
		c.JSON(http.StatusServiceUnavailable, FinalizeMultiSigSessionResponse{
			Success: false,
			Message: "多签会话完成服务暂时不可用",
		})
		return
	}

	// 调用交易管理器完成会话
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	finalTxHash, err := h.transactionManager.FinalizeMultiSigSession(ctx, sessionID)
	if err != nil {
		h.logger.Errorf("完成多签会话失败: %v", err)
		c.JSON(http.StatusInternalServerError, FinalizeMultiSigSessionResponse{
			Success: false,
			Message: fmt.Sprintf("完成多签会话失败: %v", err),
		})
		return
	}

	h.logger.Infof("多签会话完成成功，最终交易哈希: %x", finalTxHash)
	c.JSON(http.StatusOK, FinalizeMultiSigSessionResponse{
		Success:     true,
		FinalTxHash: hex.EncodeToString(finalTxHash),
		Message:     "多签会话已成功完成",
	})
}

// FetchStaticResourceFile 获取静态资源文件
//
// POST /transactions/fetch-resource
func (h *TransactionHandlers) FetchStaticResourceFile(c *gin.Context) {
	h.logger.Info("开始处理静态资源获取请求")

	var req FetchStaticResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析静态资源获取参数失败: %v", err)
		c.JSON(http.StatusBadRequest, FetchStaticResourceResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 解析内容哈希
	contentHash, err := hex.DecodeString(req.ContentHash)
	if err != nil {
		h.logger.Errorf("内容哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, FetchStaticResourceResponse{
			Success: false,
			Message: "内容哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 解析请求者私钥
	requesterPrivateKey, err := hex.DecodeString(req.RequesterPrivateKey)
	if err != nil {
		h.logger.Errorf("请求者私钥格式错误: %v", err)
		c.JSON(http.StatusBadRequest, FetchStaticResourceResponse{
			Success: false,
			Message: "请求者私钥格式错误，请使用十六进制格式",
		})
		return
	}

	// 调用交易服务获取静态资源文件
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	filePath, err := h.transactionService.FetchStaticResourceFile(
		ctx,
		contentHash,
		requesterPrivateKey,
		req.TargetDir,
	)
	if err != nil {
		h.logger.Errorf("获取静态资源文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, FetchStaticResourceResponse{
			Success: false,
			Message: fmt.Sprintf("获取静态资源文件失败: %v", err),
		})
		return
	}

	h.logger.Infof("静态资源文件获取成功，保存路径: %s", filePath)
	c.JSON(http.StatusOK, FetchStaticResourceResponse{
		Success:  true,
		FilePath: filePath,
		Message:  "静态资源文件获取成功",
	})
}
