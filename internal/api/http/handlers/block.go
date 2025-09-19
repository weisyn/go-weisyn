// Package handlers 提供HTTP API处理器
//
// block.go 实现区块查询相关的HTTP API端点
//
// 🎯 **区块查询API架构**
//
// 本文件严格按照 pkg/interfaces 中实际存在的接口实现，
// 使用 repository.RepositoryManager 进行区块查询，
// 使用 blockchain.ChainService 获取链状态信息。

package handlers

import (
	"context"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 🎯 请求响应结构定义 ====================

// BlockResponse 区块响应
type BlockResponse struct {
	Success bool        `json:"success"`
	Block   *core.Block `json:"block,omitempty"`
	Message string      `json:"message"`
}

// ChainInfoResponse 链信息响应
type ChainInfoResponse struct {
	Success   bool             `json:"success"`
	ChainInfo *types.ChainInfo `json:"chain_info,omitempty"`
	Message   string           `json:"message"`
}

// ==================== 🏗️ 区块查询API处理器 ====================

// BlockHandlers 区块查询API处理器
type BlockHandlers struct {
	repositoryManager repository.RepositoryManager
	chainService      blockchain.ChainService
	logger            log.Logger
}

// NewBlockHandlers 创建区块查询API处理器
func NewBlockHandlers(
	repositoryManager repository.RepositoryManager,
	chainService blockchain.ChainService,
	logger log.Logger,
) *BlockHandlers {
	return &BlockHandlers{
		repositoryManager: repositoryManager,
		chainService:      chainService,
		logger:            logger,
	}
}

// ==================== 🎯 核心API方法实现 ====================

// GetChainInfo 获取链信息
//
// 📌 **接口说明**：获取区块链的基础状态信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/blocks/chain-info`
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "chain_info": {
//	    "height": 12345,
//	    "best_block_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	    "is_ready": true,
//	    "status": "normal",
//	    "network_height": 12345,
//	    "peer_count": 8,
//	    "last_block_time": 1640995200,
//	    "uptime": 86400,
//	    "node_mode": "full"
//	  },
//	  "message": "链信息获取成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "获取链信息失败: 服务暂时不可用"
//	}
//
// 💡 **使用说明**：
// - 返回链的核心状态：当前高度、最佳区块哈希、同步状态等
// - 用于监控系统状态和确定链是否就绪
// - node_mode显示节点类型：light（轻节点）或full（全节点）
func (h *BlockHandlers) GetChainInfo(c *gin.Context) {
	h.logger.Info("开始处理链信息查询请求")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	chainInfo, err := h.chainService.GetChainInfo(ctx)
	if err != nil {
		h.logger.Errorf("获取链信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, ChainInfoResponse{
			Success: false,
			Message: "获取链信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ChainInfoResponse{
		Success:   true,
		ChainInfo: chainInfo,
		Message:   "链信息获取成功",
	})
}

// GetBlockByHeight 根据高度获取区块
//
// 📌 **接口说明**：根据区块高度获取完整区块信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/blocks/height/{height}`
//
// **路径参数**：
//   - height (number, required): 区块高度，从0开始的递增整数
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "block": {
//	    "header": {
//	      "height": 12345,
//	      "hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	      "previous_hash": "b2c3d4e5f6789012345678901234567890abcdef12",
//	      "timestamp": 1640995200,
//	      "nonce": 12345678
//	    },
//	    "transactions": [...]
//	  },
//	  "message": "区块 12345 获取成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "获取区块失败: 区块高度不存在"
//	}
//
// 💡 **使用说明**：
// - 返回指定高度的完整区块数据
// - 包含区块头和所有交易信息
// - 用于区块链浏览器和历史数据查询
func (h *BlockHandlers) GetBlockByHeight(c *gin.Context) {
	heightStr := c.Param("height")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, BlockResponse{
			Success: false,
			Message: "缺少高度参数",
		})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		h.logger.Errorf("高度格式错误: %v", err)
		c.JSON(http.StatusBadRequest, BlockResponse{
			Success: false,
			Message: "高度格式错误，请使用数字",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	block, err := h.repositoryManager.GetBlockByHeight(ctx, height)
	if err != nil {
		h.logger.Errorf("获取区块失败: %v", err)
		c.JSON(http.StatusNotFound, BlockResponse{
			Success: false,
			Message: "获取区块失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, BlockResponse{
		Success: true,
		Block:   block,
		Message: "区块获取成功",
	})
}

// GetBlockByHash 根据哈希获取区块
//
// GET /blocks/hash/:hash
func (h *BlockHandlers) GetBlockByHash(c *gin.Context) {
	hashStr := c.Param("hash")
	if hashStr == "" {
		c.JSON(http.StatusBadRequest, BlockResponse{
			Success: false,
			Message: "缺少哈希参数",
		})
		return
	}

	blockHash, err := hex.DecodeString(hashStr)
	if err != nil {
		h.logger.Errorf("哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, BlockResponse{
			Success: false,
			Message: "哈希格式错误，请使用十六进制格式",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	block, err := h.repositoryManager.GetBlock(ctx, blockHash)
	if err != nil {
		h.logger.Errorf("获取区块失败: %v", err)
		c.JSON(http.StatusNotFound, BlockResponse{
			Success: false,
			Message: "获取区块失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, BlockResponse{
		Success: true,
		Block:   block,
		Message: "区块获取成功",
	})
}

// GetLatestBlock 获取最新区块
//
// GET /blocks/latest
func (h *BlockHandlers) GetLatestBlock(c *gin.Context) {
	h.logger.Info("开始处理最新区块查询请求")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 先获取链信息得到最新高度
	chainInfo, err := h.chainService.GetChainInfo(ctx)
	if err != nil {
		h.logger.Errorf("获取链信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, BlockResponse{
			Success: false,
			Message: "获取链信息失败: " + err.Error(),
		})
		return
	}

	// 根据最新高度获取区块
	block, err := h.repositoryManager.GetBlockByHeight(ctx, chainInfo.Height)
	if err != nil {
		h.logger.Errorf("获取最新区块失败: %v", err)
		c.JSON(http.StatusNotFound, BlockResponse{
			Success: false,
			Message: "获取最新区块失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, BlockResponse{
		Success: true,
		Block:   block,
		Message: "最新区块获取成功",
	})
}
