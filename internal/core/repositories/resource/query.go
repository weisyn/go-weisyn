// Package resource - 查询操作实现
//
// 🎯 **资源查询服务 (Resource Query Service)**
//
// 本文件实现区块链自运行系统的核心资源查询操作：
// - 按哈希查询：内容寻址的精确查询（核心功能）
// - 按类型查询：列出特定类型的资源（简化分页）
package resource

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                          📦 公共接口查询实现
// ============================================================================

// getResourceByHash 基于内容哈希获取资源信息
//
// 🎯 **纯内容寻址查询**：
// - 直接基于内容哈希查询，避免路径查询的复杂性
// - 从BadgerDB索引获取完整元数据
// - 这是内容寻址架构的唯一标准查询方式
func (m *Manager) getResourceByHash(ctx context.Context, contentHash []byte) (*types.ResourceStorageInfo, error) {
	contentHashHex := hex.EncodeToString(contentHash)

	// 从BadgerDB获取资源元数据
	metaKey := resourceMetaPrefix + contentHashHex
	metaData, err := m.badgerStore.Get(ctx, []byte(metaKey))
	if err != nil {
		if err.Error() == "key not found" {
			return nil, fmt.Errorf("资源不存在: %s", contentHashHex)
		}
		return nil, fmt.Errorf("获取资源元数据失败: %w", err)
	}

	// 反序列化资源信息
	resourceInfo, err := m.deserializeResourceInfo(metaData)
	if err != nil {
		return nil, fmt.Errorf("解析资源元数据失败: %w", err)
	}

	// 验证文件是否仍然存在
	targetPath := m.buildHashBasedPath(contentHash)
	fullTargetPath := m.buildResourcePath(targetPath)
	exists, err := m.fileStore.Exists(ctx, fullTargetPath)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("检查文件存在性失败: %s, 错误: %v", targetPath, err)
		}
	} else if !exists {
		resourceInfo.IsAvailable = false
		if m.logger != nil {
			m.logger.Warnf("资源文件不存在: %s", targetPath)
		}
	}

	return resourceInfo, nil
}

// listResourcesByType 按类型列出资源
//
// 🗂️ **简化类型查询**：
// - 基于分类索引查询
// - 简化分页处理，使用固定限制
// - 专注核心业务需求
func (m *Manager) listResourcesByType(ctx context.Context, resourceType string, offset int, limit int) ([]*types.ResourceStorageInfo, error) {
	// 参数验证与简化
	if resourceType == "" {
		return nil, fmt.Errorf("资源类型不能为空")
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 { // 简化：固定最大100个
		limit = 50 // 简化：默认50个
	}

	// 从分类索引获取哈希列表
	hashList, err := m.getCategoryIndex(ctx, resourceType)
	if err != nil {
		return nil, fmt.Errorf("获取分类索引失败: %w", err)
	}

	// 简化分页逻辑
	totalCount := len(hashList)
	if offset >= totalCount {
		return []*types.ResourceStorageInfo{}, nil
	}

	end := offset + limit
	if end > totalCount {
		end = totalCount
	}
	paginatedHashList := hashList[offset:end]

	// 查询资源信息
	var results []*types.ResourceStorageInfo
	for _, contentHash := range paginatedHashList {
		resourceInfo, err := m.getResourceByHash(ctx, contentHash)
		if err != nil {
			continue // 简化：跳过错误，不记录日志
		}
		results = append(results, resourceInfo)
	}

	return results, nil
}

// ============================================================================
//                         🎯 区块链自运行系统查询原则
// ============================================================================
//
// 🔧 **设计理念**：
// - 内容寻址为核心：基于哈希的精确查询是根本
// - 自运行特性：不需要复杂的管理界面查询功能
// - 简单高效：避免过度抽象和统计功能
// - 业务导向：只保留真正业务需要的查询能力
//
// 📋 **保留的核心查询**：
// 1. getResourceByHash - 内容寻址的核心查询
// 2. listResourcesByType - 简化的类型查询
//
// ❌ **移除的管理类查询**：
// - 创建者查询（管理功能）
// - 名称查询（管理界面功能）
// - 批量查询（过度优化）
// - 统计查询（完全不需要）
