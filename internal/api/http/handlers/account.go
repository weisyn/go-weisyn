// Package handlers 提供HTTP API处理器
//
// account.go 实现账户管理相关的HTTP API端点

package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/gin-gonic/gin"
)

// AccountHandlers 账户管理API处理器
type AccountHandlers struct {
	accountService blockchain.AccountService
	chainService   blockchain.ChainService
	addressManager crypto.AddressManager
	logger         log.Logger
}

// NewAccountHandlers 创建账户管理API处理器
func NewAccountHandlers(
	accountService blockchain.AccountService,
	chainService blockchain.ChainService,
	addressManager crypto.AddressManager,
	logger log.Logger,
) *AccountHandlers {
	return &AccountHandlers{
		accountService: accountService,
		chainService:   chainService,
		addressManager: addressManager,
		logger:         logger,
	}
}

// validateAndParseAddress 验证并解析地址
func (h *AccountHandlers) validateAndParseAddress(addressStr string) ([]byte, error) {
	// 使用AddressManager验证地址格式
	valid, err := h.addressManager.ValidateAddress(addressStr)
	if err != nil || !valid {
		return nil, err
	}

	// 转换地址为字节
	return h.addressManager.AddressToBytes(addressStr)
}

// validateAndParsePublicKey 验证并解析公钥
func (h *AccountHandlers) validateAndParsePublicKey(publicKeyStr string) ([]byte, error) {
	// 去掉可能的0x前缀
	if len(publicKeyStr) >= 2 && (publicKeyStr[:2] == "0x" || publicKeyStr[:2] == "0X") {
		publicKeyStr = publicKeyStr[2:]
	}

	// 验证公钥长度（64字节 = 128个十六进制字符）
	if len(publicKeyStr) != 128 {
		return nil, fmt.Errorf("公钥长度错误: %d, 期望128个十六进制字符", len(publicKeyStr))
	}

	// 解析十六进制
	publicKeyBytes := make([]byte, 64)
	for i := 0; i < 64; i++ {
		high := hexCharToByte(publicKeyStr[i*2])
		low := hexCharToByte(publicKeyStr[i*2+1])
		if high == 255 || low == 255 {
			return nil, fmt.Errorf("公钥包含无效的十六进制字符: %s", publicKeyStr[i*2:i*2+2])
		}
		publicKeyBytes[i] = (high << 4) | low
	}

	return publicKeyBytes, nil
}

// hexCharToByte 将十六进制字符转换为字节（0-15），无效字符返回255
func hexCharToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	default:
		return 255
	}
}

// publicKeyToAddress 从公钥转换为地址字节
func (h *AccountHandlers) publicKeyToAddress(publicKeyBytes []byte) ([]byte, error) {
	// 使用AddressManager将公钥转换为地址字符串
	addressStr, err := h.addressManager.PublicKeyToAddress(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("公钥转地址失败: %w", err)
	}

	// 将地址字符串转换为字节
	return h.addressManager.AddressToBytes(addressStr)
}

// GetPlatformBalance 获取平台主币余额
//
// 📌 **接口说明**：查询指定地址的平台主币余额
//
// **HTTP Method**: `GET`
// **URL Path**: `/account/{address}/balance`
//
// **路径参数**：
//   - address (string, required):WES标准地址
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "data": {
//	    "address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
//	    "balance": "1500000000000000000",
//	    "balance_formatted": "1.5",
//	    "last_updated": "2024-01-15T10:30:00Z"
//	  },
//	  "message": "余额查询成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "INVALID_ADDRESS",
//	    "message": "地址格式无效",
//	    "details": "地址必须是有效的 Base58Check格式"
//	  }
//	}
//
// 💡 **使用说明**：
// - address参数：标准地址，Base58Check编码格式
// - balance字段：以wei为单位的余额 (1 = 10^18 wei)
// - 支持查询任何有效的地址
//
// 📋 **地址格式要求**：
//
//	有效地址示例：
//	Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn
//	DfA8Bks2QnEUeykiJJgrAtKPNPrAzPdPmT
//
//	无效地址示例：
//	0x1234567890abcdef1234567890abcdef12345678    // 错误的0x格式
//	Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPm            // 长度不足
//	invalid_address_format                      // 非Base58字符
func (h *AccountHandlers) GetPlatformBalance(c *gin.Context) {
	addressStr := c.Param("address")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInvalidAddress,
				Message: "无效的地址格式",
				Details: err.Error(),
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	balance, err := h.accountService.GetPlatformBalance(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInternalError,
				Message: "查询余额失败",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, StandardAPIResponse{
		Success: true,
		Data:    balance,
		Message: "余额查询成功",
	})
}

// GetTokenBalance 查询指定代币余额
func (h *AccountHandlers) GetTokenBalance(c *gin.Context) {
	addressStr := c.Param("address")
	tokenIDStr := c.Param("tokenId")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的地址格式"})
		return
	}

	tokenID, err := hex.DecodeString(tokenIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代币ID格式"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	balance, err := h.accountService.GetTokenBalance(ctx, address, tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询代币余额失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    balance,
	})
}

// GetAllTokenBalances 查询所有代币余额
//
// GET /accounts/:address/balances
//
// 📋 **功能说明**：
// 查询指定地址的所有代币余额，包括主币和所有合约代币。
//
// 🌐 **curl调用示例**：
//
//	curl http://localhost:8080/api/v1/accounts/0x1234567890abcdef1234567890abcdef12345678/balances
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "data": {
//	    "address": "0x1234567890abcdef1234567890abcdef12345678",
//	    "native_balance": {
//	      "token_name": "",
//	      "token_symbol": "",
//	      "balance": "1500000000000000000",
//	      "balance_formatted": "1.5"
//	    },
//	    "token_balances": [
//	      {
//	        "token_id": "0xabcdef123456789abcdef123456789abcdef123456",
//	        "token_name": "Example Token",
//	        "token_symbol": "EXT",
//	        "balance": "1000000000",
//	        "balance_formatted": "1000.0 EXT",
//	        "decimals": 6
//	      }
//	    ],
//	    "total_tokens": 2,
//	    "last_updated": "2024-01-15T10:30:00Z"
//	  },
//	  "message": "代币余额查询成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "INVALID_ADDRESS",
//	    "message": "地址格式无效",
//	    "details": "地址必须是42字符的十六进制字符串，以0x开头"
//	  }
//	}
//
// 💡 **使用说明**：
// - 返回主币和所有合约代币的完整余额信息
// - native_balance：主币余额
// - token_balances：合约代币余额列表
// - 自动获取代币元数据（名称、符号、精度）
//
// 📊 **数据字段说明**：
// - balance：原始余额（最小单位）
// - balance_formatted：格式化余额（易读格式）
// - decimals：代币精度（小数位数）
// - total_tokens：持有的代币种类总数
func (h *AccountHandlers) GetAllTokenBalances(c *gin.Context) {
	addressStr := c.Param("address")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInvalidAddress,
				Message: "无效的地址格式",
				Details: err.Error(),
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	allBalances, err := h.accountService.GetAllTokenBalances(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInternalError,
				Message: "查询所有代币余额失败",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, StandardAPIResponse{
		Success: true,
		Data:    allBalances,
		Message: "所有代币余额查询成功",
	})
}

// GetLockedBalances 查询锁定余额详情
func (h *AccountHandlers) GetLockedBalances(c *gin.Context) {
	addressStr := c.Param("address")
	tokenIDStr := c.Query("tokenId")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的地址格式"})
		return
	}

	var tokenID []byte
	if tokenIDStr != "" {
		tokenID, err = hex.DecodeString(tokenIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代币ID格式"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	lockedEntries, err := h.accountService.GetLockedBalances(ctx, address, tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询锁定余额失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    lockedEntries,
	})
}

// GetPendingBalances 查询待确认余额详情
func (h *AccountHandlers) GetPendingBalances(c *gin.Context) {
	addressStr := c.Param("address")
	tokenIDStr := c.Query("tokenId")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的地址格式"})
		return
	}

	var tokenID []byte
	if tokenIDStr != "" {
		tokenID, err = hex.DecodeString(tokenIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代币ID格式"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	pendingEntries, err := h.accountService.GetPendingBalances(ctx, address, tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询待确认余额失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pendingEntries,
	})
}

// GetEffectiveBalance 获取有效可用余额
//
// 🎯 **新增API接口**：解决审查报告中用户期望的余额实时扣减问题
//
// 📍 **API路径**：GET /api/v1/accounts/:address/effective-balance
//
// 🔄 **查询参数**：
//   - tokenId (可选)：代币ID，十六进制格式，不提供则查询原生币
//   - includeDebug (可选)：是否包含调试信息，默认false
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "data": {
//	    "address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
//	    "token_id": null,
//	    "spendable_amount": "1200000000000000000",
//	    "confirmed_available": "1500000000000000000",
//	    "pending_out": "350000000000000000",
//	    "pending_in": "50000000000000000",
//	    "pending_tx_count": 2,
//	    "calculation_method": "confirmed_available_minus_pending_out_plus_pending_in",
//	    "last_updated": "2024-01-15T10:30:00Z"
//	  },
//	  "message": "有效余额查询成功"
//	}
//
// 💡 **重要说明**：
// - spendable_amount：用户真正可以花费的金额
// - confirmed_available：已确认的可用余额
// - pending_out：待确认的支出金额（会减少可用余额）
// - pending_in：待确认的收入金额（会增加可用余额）
// - 计算公式：spendable_amount = confirmed_available - pending_out + pending_in
func (h *AccountHandlers) GetEffectiveBalance(c *gin.Context) {
	addressStr := c.Param("address")
	tokenIDStr := c.Query("tokenId")
	includeDebugStr := c.Query("includeDebug")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInvalidAddress,
				Message: "无效的地址格式",
				Details: err.Error(),
			},
		})
		return
	}

	var tokenID []byte
	if tokenIDStr != "" {
		tokenID, err = hex.DecodeString(tokenIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, StandardAPIResponse{
				Success: false,
				Error: &APIError{
					Code:    ErrorCodeInvalidTokenID,
					Message: "无效的代币ID格式",
					Details: err.Error(),
				},
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	effectiveBalance, err := h.accountService.GetEffectiveBalance(ctx, address, tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardAPIResponse{
			Success: false,
			Error: &APIError{
				Code:    ErrorCodeInternalError,
				Message: "查询有效余额失败",
				Details: err.Error(),
			},
		})
		return
	}

	// 根据参数决定是否包含调试信息
	includeDebug := includeDebugStr == "true" || includeDebugStr == "1"
	if !includeDebug {
		effectiveBalance.DebugInfo = nil
	}

	c.JSON(http.StatusOK, StandardAPIResponse{
		Success: true,
		Data:    effectiveBalance,
		Message: "有效余额查询成功",
	})
}

// GetAccountInfo 获取账户信息
func (h *AccountHandlers) GetAccountInfo(c *gin.Context) {
	addressStr := c.Param("address")

	address, err := h.validateAndParseAddress(addressStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的地址格式"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	accountInfo, err := h.accountService.GetAccountInfo(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询账户信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    accountInfo,
	})
}

// GetPlatformBalanceByPublicKey 通过公钥查询平台主币余额
func (h *AccountHandlers) GetPlatformBalanceByPublicKey(c *gin.Context) {
	publicKeyStr := c.Param("publicKey")

	// 验证并解析公钥
	publicKeyBytes, err := h.validateAndParsePublicKey(publicKeyStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公钥格式: " + err.Error()})
		return
	}

	// 从公钥转换为地址
	address, err := h.publicKeyToAddress(publicKeyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公钥转地址失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	balance, err := h.accountService.GetPlatformBalance(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询余额失败"})
		return
	}

	derivedAddress, err := h.addressManager.BytesToAddress(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "地址转换失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    balance,
		"meta": gin.H{
			"public_key":      publicKeyStr,
			"derived_address": derivedAddress,
		},
	})
}

// GetAllTokenBalancesByPublicKey 通过公钥查询账户所有代币余额
func (h *AccountHandlers) GetAllTokenBalancesByPublicKey(c *gin.Context) {
	publicKeyStr := c.Param("publicKey")

	// 验证并解析公钥
	publicKeyBytes, err := h.validateAndParsePublicKey(publicKeyStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公钥格式: " + err.Error()})
		return
	}

	// 从公钥转换为地址
	address, err := h.publicKeyToAddress(publicKeyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公钥转地址失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	allBalances, err := h.accountService.GetAllTokenBalances(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询所有代币余额失败"})
		return
	}

	derivedAddress, err := h.addressManager.BytesToAddress(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "地址转换失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    allBalances,
		"meta": gin.H{
			"public_key":      publicKeyStr,
			"derived_address": derivedAddress,
		},
	})
}

// GetAccountInfoByPublicKey 通过公钥获取账户信息
func (h *AccountHandlers) GetAccountInfoByPublicKey(c *gin.Context) {
	publicKeyStr := c.Param("publicKey")

	// 验证并解析公钥
	publicKeyBytes, err := h.validateAndParsePublicKey(publicKeyStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公钥格式: " + err.Error()})
		return
	}

	// 从公钥转换为地址
	address, err := h.publicKeyToAddress(publicKeyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公钥转地址失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	accountInfo, err := h.accountService.GetAccountInfo(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询账户信息失败"})
		return
	}

	derivedAddress, err := h.addressManager.BytesToAddress(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "地址转换失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    accountInfo,
		"meta": gin.H{
			"public_key":      publicKeyStr,
			"derived_address": derivedAddress,
		},
	})
}

// RegisterRoutes 注册账户相关路由
func (h *AccountHandlers) RegisterRoutes(router *gin.RouterGroup) {
	accounts := router.Group("/accounts")
	{
		// 通过地址查询（原有功能）
		accounts.GET("/:address/balance", h.GetPlatformBalance)
		accounts.GET("/:address/balance/:tokenId", h.GetTokenBalance)
		accounts.GET("/:address/balances", h.GetAllTokenBalances)
		accounts.GET("/:address/locked", h.GetLockedBalances)
		accounts.GET("/:address/pending", h.GetPendingBalances)
		accounts.GET("/:address/info", h.GetAccountInfo)

		// 🔥 新增：有效可用余额查询接口（解决审查报告中的用户期望问题）
		accounts.GET("/:address/effective-balance", h.GetEffectiveBalance)

		// 通过公钥查询账户信息（用户友好接口）
		accounts.GET("/by-public-key/:publicKey/balance", h.GetPlatformBalanceByPublicKey)
		accounts.GET("/by-public-key/:publicKey/balances", h.GetAllTokenBalancesByPublicKey)
		accounts.GET("/by-public-key/:publicKey/info", h.GetAccountInfoByPublicKey)
	}
}
