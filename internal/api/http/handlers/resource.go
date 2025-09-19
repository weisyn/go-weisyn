// Package handlers 提供HTTP API处理器
//
// resource.go 实现资源管理相关的HTTP API端点
//
// 🎯 **资源API架构**
//
// 本文件严格按照 pkg/interfaces 中实际存在的接口实现，
// 使用 repository.ResourceManager 进行资源管理。

package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 🎯 请求响应结构定义 ====================

// StoreResourceRequest 存储资源请求
type StoreResourceRequest struct {
	SourceFilePath string            `json:"source_file_path" binding:"required"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// StoreResourceResponse 存储资源响应
type StoreResourceResponse struct {
	Success     bool   `json:"success"`
	ContentHash string `json:"content_hash"`
	Message     string `json:"message"`
}

// GetResourceResponse 获取资源响应
type GetResourceResponse struct {
	Success  bool                       `json:"success"`
	Resource *types.ResourceStorageInfo `json:"resource,omitempty"`
	Message  string                     `json:"message"`
}

// ListResourcesResponse 列出资源响应
type ListResourcesResponse struct {
	Success   bool                         `json:"success"`
	Resources []*types.ResourceStorageInfo `json:"resources,omitempty"`
	Message   string                       `json:"message"`
}

// ==================== 🏗️ 资源API处理器 ====================

// ResourceHandler 资源处理器
type ResourceHandler struct {
	resourceManager repository.ResourceManager
	logger          log.Logger
}

// NewResourceHandler 创建资源处理器
func NewResourceHandler(
	resourceManager repository.ResourceManager,
	logger log.Logger,
) *ResourceHandler {
	return &ResourceHandler{
		resourceManager: resourceManager,
		logger:          logger,
	}
}

// ==================== 🎯 核心API方法实现 ====================

// StoreResource 存储资源
//
// 📌 **接口说明**：将文件存储到区块链资源系统
//
// **HTTP Method**: `POST`
// **URL Path**: `/resources/store`
//
// **请求体参数**：
//   - source_file_path (string, required): 源文件的完整路径
//   - metadata (object, optional): 资源元数据信息
//
// **请求体示例**：
//
//	{
//	  "source_file_path": "/path/to/document.pdf",
//	  "metadata": {
//	    "type": "document",
//	    "author": "张三",
//	    "description": "重要合同文件"
//	  }
//	}
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "content_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	  "message": "资源存储成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "资源存储失败: 文件不存在"
//	}
//
// 💡 **使用说明**：
// - 支持任意大小的文件
// - 返回的content_hash是文件的SHA-256哈希，用于后续查询
// - 相同内容的文件只存储一次（自动去重）
func (h *ResourceHandler) StoreResource(c *gin.Context) {
	h.logger.Info("开始处理资源存储请求")

	var req StoreResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorf("解析资源存储参数失败: %v", err)
		c.JSON(http.StatusBadRequest, StoreResourceResponse{
			Success: false,
			Message: fmt.Sprintf("参数格式错误: %v", err),
		})
		return
	}

	// 调用资源管理器存储文件
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	contentHash, err := h.resourceManager.StoreResourceFile(
		ctx,
		req.SourceFilePath,
		req.Metadata,
	)
	if err != nil {
		h.logger.Errorf("资源存储失败: %v", err)
		c.JSON(http.StatusInternalServerError, StoreResourceResponse{
			Success: false,
			Message: fmt.Sprintf("资源存储失败: %v", err),
		})
		return
	}

	h.logger.Infof("资源存储成功，内容哈希: %x", contentHash)
	c.JSON(http.StatusOK, StoreResourceResponse{
		Success:     true,
		ContentHash: hex.EncodeToString(contentHash),
		Message:     "资源存储成功",
	})
}

// GetResource 获取资源信息
//
// 📌 **接口说明**：根据内容哈希获取资源的元数据信息
//
// **HTTP Method**: `GET`
// **URL Path**: `/resources/{hash}`
//
// **路径参数**：
//   - hash (string, required): 资源内容哈希，十六进制格式
//
// ✅ **成功响应示例**：
//
//	{
//	  "success": true,
//	  "resource": {
//	    "resource_path": "/contracts/token.wasm",
//	    "resource_type": "contract",
//	    "content_hash": "a1b2c3d4e5f6789012345678901234567890abcdef",
//	    "size": 1024,
//	    "stored_at": 1640995200,
//	    "metadata": {
//	      "author": "张三",
//	      "version": "1.0"
//	    },
//	    "is_available": true
//	  },
//	  "message": "资源信息获取成功"
//	}
//
// ❌ **错误响应示例**：
//
//	{
//	  "success": false,
//	  "message": "获取资源失败: 资源不存在"
//	}
//
// 💡 **使用说明**：
// - hash参数来自StoreResource接口的返回值
// - 返回完整的资源元数据信息
// - 用于验证资源存在性和获取资源属性
func (h *ResourceHandler) GetResource(c *gin.Context) {
	hashParam := c.Param("hash")
	if hashParam == "" {
		c.JSON(http.StatusBadRequest, GetResourceResponse{
			Success: false,
			Message: "缺少哈希参数",
		})
		return
	}

	// 解析哈希
	contentHash, err := hex.DecodeString(hashParam)
	if err != nil {
		h.logger.Errorf("哈希格式错误: %v", err)
		c.JSON(http.StatusBadRequest, GetResourceResponse{
			Success: false,
			Message: "哈希格式错误，请使用十六进制格式",
		})
		return
	}

	// 调用资源管理器查询资源
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resource, err := h.resourceManager.GetResourceByHash(ctx, contentHash)
	if err != nil {
		h.logger.Errorf("获取资源失败: %v", err)
		c.JSON(http.StatusNotFound, GetResourceResponse{
			Success: false,
			Message: fmt.Sprintf("获取资源失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, GetResourceResponse{
		Success:  true,
		Resource: resource,
		Message:  "资源信息获取成功",
	})
}

// ListResources 列出指定类型的资源
//
// GET /resources/list/:type?offset=0&limit=50
func (h *ResourceHandler) ListResources(c *gin.Context) {
	resourceType := c.Param("type")
	if resourceType == "" {
		c.JSON(http.StatusBadRequest, ListResourcesResponse{
			Success: false,
			Message: "缺少资源类型参数",
		})
		return
	}

	// 解析分页参数
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "50")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ListResourcesResponse{
			Success: false,
			Message: "offset 参数格式错误",
		})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ListResourcesResponse{
			Success: false,
			Message: "limit 参数格式错误",
		})
		return
	}

	// 调用资源管理器列出资源
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	resources, err := h.resourceManager.ListResourcesByType(ctx, resourceType, offset, limit)
	if err != nil {
		h.logger.Errorf("列出资源失败: %v", err)
		c.JSON(http.StatusInternalServerError, ListResourcesResponse{
			Success: false,
			Message: fmt.Sprintf("列出资源失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, ListResourcesResponse{
		Success:   true,
		Resources: resources,
		Message:   fmt.Sprintf("成功获取 %d 个资源", len(resources)),
	})
}
