// Package resource - 索引管理实现
//
// 🎯 **BadgerDB索引管理 (BadgerDB Index Management)**
//
// 本文件实现资源的高性能索引管理：
// - 元数据索引：快速查询资源完整信息
// - 哈希映射：内容哈希到存储路径的映射
// - 分类索引：按资源类型（static/contract/aimodel）索引
// - 创建者索引：按创建者地址索引资源
// - 引用计数：资源生命周期管理
package resource

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//                              索引键定义
// ============================================================================

const (
	// 分类索引前缀: index:category:{category} -> list of content_hash
	indexCategoryPrefix = "index:category:"

	// 创建者索引前缀: index:creator:{address} -> list of content_hash
	indexCreatorPrefix = "index:creator:"

	// 名称搜索前缀: index:name:{name} -> content_hash
	indexNamePrefix = "index:name:"

	// 健康状态前缀: health:file:{path} -> last_verified_timestamp
	healthFilePrefix = "health:file:"

	// ============================================================================
	//                           🚀 新型per-item索引键前缀 (v2版本)
	// ============================================================================

	// 分类索引v2前缀: index:category:v2:{category}:{content_hash} -> 1
	indexCategoryV2Prefix = "index:category:v2:"

	// 创建者索引v2前缀: index:creator:v2:{address}:{content_hash} -> 1
	indexCreatorV2Prefix = "index:creator:v2:"

	// 名称索引v2前缀: index:name:v2:{normalized_name}:{content_hash} -> 1
	indexNameV2Prefix = "index:name:v2:"
)

// ============================================================================
//                           🗂️ 分类索引管理
// ============================================================================

// addToCategoryIndex 将资源添加到分类索引
//
// 📋 **分类索引结构**：
// - 键: index:category:{category}
// - 值: {content_hash1},{content_hash2},...
// - 支持的分类: static, contract, aimodel, unknown
func (m *Manager) addToCategoryIndex(ctx context.Context, tx storage.BadgerTransaction, category string, contentHash []byte) error {
	if category == "" {
		category = "unknown"
	}

	categoryKey := indexCategoryPrefix + category
	contentHashHex := hex.EncodeToString(contentHash)

	// 获取现有的分类索引
	existingData, err := tx.Get([]byte(categoryKey))
	if err != nil && err.Error() != "key not found" {
		return fmt.Errorf("获取分类索引失败: %w", err)
	}

	var hashList []string
	if existingData != nil {
		// 解析现有的哈希列表
		existingList := strings.TrimSpace(string(existingData))
		if existingList != "" {
			hashList = strings.Split(existingList, ",")
		}
	}

	// 检查是否已存在（去重）
	for _, existingHash := range hashList {
		if existingHash == contentHashHex {
			return nil // 已存在，无需添加
		}
	}

	// 添加新哈希
	hashList = append(hashList, contentHashHex)

	// 更新索引
	newData := strings.Join(hashList, ",")
	if err := tx.Set([]byte(categoryKey), []byte(newData)); err != nil {
		return fmt.Errorf("更新分类索引失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到分类索引: %s -> %s", category, contentHashHex)
	}

	return nil
}

// removeFromCategoryIndex 从分类索引中移除资源
func (m *Manager) removeFromCategoryIndex(ctx context.Context, tx storage.BadgerTransaction, category string, contentHash []byte) error {
	if category == "" {
		category = "unknown"
	}

	categoryKey := indexCategoryPrefix + category
	contentHashHex := hex.EncodeToString(contentHash)

	// 获取现有的分类索引
	existingData, err := tx.Get([]byte(categoryKey))
	if err != nil {
		if err.Error() == "key not found" {
			return nil // 索引不存在，无需处理
		}
		return fmt.Errorf("获取分类索引失败: %w", err)
	}

	// 解析现有的哈希列表
	existingList := strings.TrimSpace(string(existingData))
	if existingList == "" {
		return nil
	}

	hashList := strings.Split(existingList, ",")

	// 移除指定哈希
	var newHashList []string
	for _, existingHash := range hashList {
		if existingHash != contentHashHex {
			newHashList = append(newHashList, existingHash)
		}
	}

	// 更新索引
	if len(newHashList) == 0 {
		// 如果列表为空，删除整个索引键
		if err := tx.Delete([]byte(categoryKey)); err != nil {
			return fmt.Errorf("删除分类索引失败: %w", err)
		}
	} else {
		// 更新索引数据
		newData := strings.Join(newHashList, ",")
		if err := tx.Set([]byte(categoryKey), []byte(newData)); err != nil {
			return fmt.Errorf("更新分类索引失败: %w", err)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("从分类索引移除: %s -> %s", category, contentHashHex)
	}

	return nil
}

// getCategoryIndex 获取分类索引中的资源列表
func (m *Manager) getCategoryIndex(ctx context.Context, category string) ([][]byte, error) {
	if category == "" {
		category = "unknown"
	}

	categoryKey := indexCategoryPrefix + category

	// 从BadgerDB获取分类索引
	indexData, err := m.badgerStore.Get(ctx, []byte(categoryKey))
	if err != nil {
		if err.Error() == "key not found" {
			return [][]byte{}, nil // 返回空列表
		}
		return nil, fmt.Errorf("获取分类索引失败: %w", err)
	}

	// 解析哈希列表
	hashListStr := strings.TrimSpace(string(indexData))
	if hashListStr == "" {
		return [][]byte{}, nil
	}

	hashStrList := strings.Split(hashListStr, ",")
	var hashList [][]byte

	for _, hashStr := range hashStrList {
		hash, err := hex.DecodeString(strings.TrimSpace(hashStr))
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("解析哈希失败，跳过: %s, 错误: %v", hashStr, err)
			}
			continue
		}
		hashList = append(hashList, hash)
	}

	return hashList, nil
}

// ============================================================================
//                           👤 创建者索引管理
// ============================================================================

// addToCreatorIndex 将资源添加到创建者索引
//
// 📋 **创建者索引结构**：
// - 键: index:creator:{creator_address}
// - 值: {content_hash1},{content_hash2},...
// - 用于查询特定创建者的所有资源
func (m *Manager) addToCreatorIndex(ctx context.Context, tx storage.BadgerTransaction, creatorAddress string, contentHash []byte) error {
	if creatorAddress == "" {
		return nil // 没有创建者信息，跳过索引
	}

	creatorKey := indexCreatorPrefix + creatorAddress
	contentHashHex := hex.EncodeToString(contentHash)

	// 获取现有的创建者索引
	existingData, err := tx.Get([]byte(creatorKey))
	if err != nil && err.Error() != "key not found" {
		return fmt.Errorf("获取创建者索引失败: %w", err)
	}

	var hashList []string
	if existingData != nil {
		existingList := strings.TrimSpace(string(existingData))
		if existingList != "" {
			hashList = strings.Split(existingList, ",")
		}
	}

	// 检查是否已存在（去重）
	for _, existingHash := range hashList {
		if existingHash == contentHashHex {
			return nil
		}
	}

	// 添加新哈希
	hashList = append(hashList, contentHashHex)

	// 更新索引
	newData := strings.Join(hashList, ",")
	if err := tx.Set([]byte(creatorKey), []byte(newData)); err != nil {
		return fmt.Errorf("更新创建者索引失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到创建者索引: %s -> %s", creatorAddress, contentHashHex)
	}

	return nil
}

// removeFromCreatorIndex 从创建者索引中移除资源
func (m *Manager) removeFromCreatorIndex(ctx context.Context, tx storage.BadgerTransaction, creatorAddress string, contentHash []byte) error {
	if creatorAddress == "" {
		return nil // 没有创建者信息，跳过
	}

	creatorKey := indexCreatorPrefix + creatorAddress
	contentHashHex := hex.EncodeToString(contentHash)

	// 获取现有的创建者索引
	existingData, err := tx.Get([]byte(creatorKey))
	if err != nil {
		if err.Error() == "key not found" {
			return nil // 索引不存在，无需处理
		}
		return fmt.Errorf("获取创建者索引失败: %w", err)
	}

	// 解析现有的哈希列表
	existingList := strings.TrimSpace(string(existingData))
	if existingList == "" {
		return nil
	}

	hashList := strings.Split(existingList, ",")

	// 移除指定哈希
	var newHashList []string
	for _, existingHash := range hashList {
		if existingHash != contentHashHex {
			newHashList = append(newHashList, existingHash)
		}
	}

	// 更新索引
	if len(newHashList) == 0 {
		// 如果列表为空，删除整个索引键
		if err := tx.Delete([]byte(creatorKey)); err != nil {
			return fmt.Errorf("删除创建者索引失败: %w", err)
		}
	} else {
		// 更新索引数据
		newData := strings.Join(newHashList, ",")
		if err := tx.Set([]byte(creatorKey), []byte(newData)); err != nil {
			return fmt.Errorf("更新创建者索引失败: %w", err)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("从创建者索引移除: %s -> %s", creatorAddress, contentHashHex)
	}

	return nil
}

// getCreatorIndex 获取创建者索引中的资源列表
func (m *Manager) getCreatorIndex(ctx context.Context, creatorAddress string) ([][]byte, error) {
	if creatorAddress == "" {
		return [][]byte{}, nil
	}

	creatorKey := indexCreatorPrefix + creatorAddress

	// 从BadgerDB获取创建者索引
	indexData, err := m.badgerStore.Get(ctx, []byte(creatorKey))
	if err != nil {
		if err.Error() == "key not found" {
			return [][]byte{}, nil
		}
		return nil, fmt.Errorf("获取创建者索引失败: %w", err)
	}

	// 解析哈希列表
	hashListStr := strings.TrimSpace(string(indexData))
	if hashListStr == "" {
		return [][]byte{}, nil
	}

	hashStrList := strings.Split(hashListStr, ",")
	var hashList [][]byte

	for _, hashStr := range hashStrList {
		hash, err := hex.DecodeString(strings.TrimSpace(hashStr))
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("解析创建者索引哈希失败，跳过: %s", hashStr)
			}
			continue
		}
		hashList = append(hashList, hash)
	}

	return hashList, nil
}

// ============================================================================
//                           🏷️ 名称搜索索引管理
// ============================================================================

// addToNameIndex 将资源添加到名称搜索索引
//
// 📋 **名称索引结构**：
// - 键: index:name:{resource_name}
// - 值: {content_hash}
// - 用于按名称快速查找资源
func (m *Manager) addToNameIndex(ctx context.Context, tx storage.BadgerTransaction, resourceName string, contentHash []byte) error {
	if resourceName == "" {
		return nil // 没有名称，跳过索引
	}

	// 标准化资源名称（转小写，用于搜索）
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	if normalizedName == "" {
		return nil
	}

	nameKey := indexNamePrefix + normalizedName
	contentHashHex := hex.EncodeToString(contentHash)

	// 直接设置名称到哈希的映射
	if err := tx.Set([]byte(nameKey), []byte(contentHashHex)); err != nil {
		return fmt.Errorf("更新名称索引失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到名称索引: %s -> %s", normalizedName, contentHashHex)
	}

	return nil
}

// removeFromNameIndex 从名称索引中移除资源
func (m *Manager) removeFromNameIndex(ctx context.Context, resourceName string, contentHash []byte) error {
	if resourceName == "" {
		return nil // 没有名称，跳过
	}

	// 标准化资源名称
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	if normalizedName == "" {
		return nil
	}

	nameKey := indexNamePrefix + normalizedName
	contentHashHex := hex.EncodeToString(contentHash)

	// 检查当前名称索引是否指向要删除的哈希
	currentHashData, err := m.badgerStore.Get(ctx, []byte(nameKey))
	if err != nil {
		if err.Error() == "key not found" {
			return nil // 名称索引不存在，无需处理
		}
		return fmt.Errorf("获取名称索引失败: %w", err)
	}

	currentHashHex := strings.TrimSpace(string(currentHashData))
	if currentHashHex == contentHashHex {
		// 当前名称指向要删除的资源，删除名称索引
		if err := m.badgerStore.Delete(ctx, []byte(nameKey)); err != nil {
			return fmt.Errorf("删除名称索引失败: %w", err)
		}

		if m.logger != nil {
			m.logger.Debugf("从名称索引移除: %s -> %s", normalizedName, contentHashHex)
		}
	}

	return nil
}

// getResourceByName 按名称获取资源哈希
func (m *Manager) getResourceByName(ctx context.Context, resourceName string) ([]byte, error) {
	if resourceName == "" {
		return nil, fmt.Errorf("资源名称不能为空")
	}

	// 标准化资源名称
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	nameKey := indexNamePrefix + normalizedName

	// 从BadgerDB获取名称索引
	hashData, err := m.badgerStore.Get(ctx, []byte(nameKey))
	if err != nil {
		if err.Error() == "key not found" {
			return nil, fmt.Errorf("未找到名称为 %s 的资源", resourceName)
		}
		return nil, fmt.Errorf("获取名称索引失败: %w", err)
	}

	// 解析哈希
	contentHashHex := strings.TrimSpace(string(hashData))
	contentHash, err := hex.DecodeString(contentHashHex)
	if err != nil {
		return nil, fmt.Errorf("解析资源哈希失败: %w", err)
	}

	return contentHash, nil
}

// ============================================================================
//                           🩺 健康状态索引管理
// ============================================================================

// updateHealthStatus 更新资源健康状态
//
// 📋 **健康状态结构**：
// - 键: health:file:{storage_path}
// - 值: {last_verified_timestamp}
// - 用于跟踪文件完整性验证状态
func (m *Manager) updateHealthStatus(ctx context.Context, storagePath string, isHealthy bool) error {
	healthKey := healthFilePrefix + storagePath

	currentTime := strconv.FormatInt(getCurrentTimestamp(), 10)

	// 健康状态值格式: timestamp:status
	var statusValue string
	if isHealthy {
		statusValue = currentTime + ":ok"
	} else {
		statusValue = currentTime + ":error"
	}

	if err := m.badgerStore.Set(ctx, []byte(healthKey), []byte(statusValue)); err != nil {
		return fmt.Errorf("更新健康状态失败: %w", err)
	}

	if m.logger != nil {
		status := "healthy"
		if !isHealthy {
			status = "unhealthy"
		}
		m.logger.Debugf("更新资源健康状态: %s -> %s", storagePath, status)
	}

	return nil
}

// getHealthStatus 获取资源健康状态
func (m *Manager) getHealthStatus(ctx context.Context, storagePath string) (bool, int64, error) {
	healthKey := healthFilePrefix + storagePath

	statusData, err := m.badgerStore.Get(ctx, []byte(healthKey))
	if err != nil {
		if err.Error() == "key not found" {
			return false, 0, nil // 未记录健康状态
		}
		return false, 0, fmt.Errorf("获取健康状态失败: %w", err)
	}

	// 解析状态值: timestamp:status
	statusStr := string(statusData)
	parts := strings.SplitN(statusStr, ":", 2)
	if len(parts) != 2 {
		return false, 0, fmt.Errorf("健康状态格式错误: %s", statusStr)
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false, 0, fmt.Errorf("解析时间戳失败: %w", err)
	}

	isHealthy := parts[1] == "ok"

	return isHealthy, timestamp, nil
}

// ============================================================================
//                              🔧 辅助函数
// ============================================================================

// getCurrentTimestamp 获取当前Unix时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// ============================================================================
//                        🚀 新型per-item索引管理 (v2版本)
// ============================================================================

// addToCategoryIndexV2 将资源添加到分类索引 (并发优化版本)
//
// 🎯 **Per-Item键设计的并发优化索引 (Concurrent-Optimized Index with Per-Item Keys)**
//
// 这是v2版本的分类索引，专为高并发环境设计。通过为每个资源分配独立的键，
// 彻底解决了传统逗号分隔列表设计中的读-修改-写竞争问题。
//
// 📋 **键值设计详解**：
//
//	🔑 键格式: index:category:v2:{category}:{content_hash}
//	💾 值内容: "1" (简单标记，表示该资源属于此分类)
//
//	示例：
//	- index:category:v2:static:abcd1234... → "1"
//	- index:category:v2:contract:efgh5678... → "1"
//	- index:category:v2:aimodel:ijkl9012... → "1"
//
// 🚀 **技术优势分析**：
//
//	✅ **无读-修改-写竞争**：
//	   • 每个资源拥有独立的键，不同线程操作不同资源时完全无冲突
//	   • 相比v1版本的"读取→解析→修改→写入"流程，v2版本仅需"直接写入"
//
//	✅ **原子操作简单高效**：
//	   • 添加操作：单次Set操作即可完成
//	   • 删除操作：单次Delete操作即可完成
//	   • 无需复杂的字符串解析和重组逻辑
//
//	✅ **查询性能优异**：
//	   • 使用BadgerDB的PrefixScan功能快速获取所有相关资源
//	   • 避免了字符串分割的CPU开销
//
//	✅ **内存友好**：
//	   • 不需要将完整的资源列表加载到内存
//	   • 流式处理查询结果，支持大量资源的分类
//
// 🔄 **处理流程**：
//  1. 验证并标准化分类名称（空值默认为"unknown"）
//  2. 构建资源的唯一索引键
//  3. 在事务中直接设置键值对
//  4. 记录调试日志（便于问题排查）
//
// 🔧 **兼容性说明**：
//   - 与v1版本并存，支持平滑迁移
//   - 查询时优先使用v2，失败时降级到v1
//   - 清理时同时处理v1和v2索引
func (m *Manager) addToCategoryIndexV2(ctx context.Context, tx storage.BadgerTransaction, category string, contentHash []byte) error {
	if category == "" {
		category = "unknown"
	}

	contentHashHex := hex.EncodeToString(contentHash)
	categoryKey := indexCategoryV2Prefix + category + ":" + contentHashHex

	// 直接设置键值对，无需读取现有数据
	if err := tx.Set([]byte(categoryKey), []byte("1")); err != nil {
		return fmt.Errorf("更新分类索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到分类索引v2: %s -> %s", category, contentHashHex)
	}

	return nil
}

// removeFromCategoryIndexV2 从分类索引中移除资源 (并发优化版本)
func (m *Manager) removeFromCategoryIndexV2(ctx context.Context, tx storage.BadgerTransaction, category string, contentHash []byte) error {
	if category == "" {
		category = "unknown"
	}

	contentHashHex := hex.EncodeToString(contentHash)
	categoryKey := indexCategoryV2Prefix + category + ":" + contentHashHex

	// 直接删除键，无需读取现有数据
	if err := tx.Delete([]byte(categoryKey)); err != nil && err.Error() != "key not found" {
		return fmt.Errorf("删除分类索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("从分类索引v2移除: %s -> %s", category, contentHashHex)
	}

	return nil
}

// getCategoryIndexV2 获取分类索引中的资源列表 (并发优化版本)
//
// 🔍 **Per-Item键的高效查询实现 (Efficient Querying with Per-Item Keys)**
//
// 利用BadgerDB的PrefixScan功能，高效检索指定分类下的所有资源。
// 相比v1版本的字符串分割方式，v2版本在性能和内存使用上都有显著提升。
//
// 🔄 **查询处理流程**：
//
//	1️⃣ **前缀构建**：构建分类查询前缀 "index:category:v2:{category}:"
//	2️⃣ **前缀扫描**：使用BadgerDB的PrefixScan一次性获取所有匹配键
//	3️⃣ **哈希提取**：从每个键中解析出资源哈希值
//	4️⃣ **结果聚合**：将所有哈希值收集到结果列表中
//	5️⃣ **错误处理**：跳过格式异常的键，记录警告日志
//
// 🎯 **性能优势**：
//   - 🚀 **一次性扫描**：PrefixScan比逐键查询效率高
//   - 🧠 **内存高效**：流式处理，不需要预先加载完整列表
//   - ⚡ **CPU友好**：避免了复杂的字符串解析操作
//   - 📊 **可扩展**：支持大量资源的分类查询
//
// 🔧 **容错机制**：
//   - 自动跳过格式错误的键
//   - 记录异常键的警告日志
//   - 确保部分数据损坏不影响整体查询
func (m *Manager) getCategoryIndexV2(ctx context.Context, category string) ([][]byte, error) {
	if category == "" {
		category = "unknown"
	}

	// 使用前缀扫描获取所有相关资源
	categoryPrefix := indexCategoryV2Prefix + category + ":"
	indexData, err := m.badgerStore.PrefixScan(ctx, []byte(categoryPrefix))
	if err != nil {
		return nil, fmt.Errorf("获取分类索引v2失败: %w", err)
	}

	var hashList [][]byte
	for keyStr, _ := range indexData {
		// 从键中提取哈希: index:category:v2:{category}:{content_hash}
		parts := strings.Split(keyStr, ":")
		if len(parts) >= 4 {
			contentHashHex := parts[len(parts)-1] // 最后一部分是哈希
			hash, err := hex.DecodeString(contentHashHex)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("解析分类索引v2哈希失败，跳过: %s", contentHashHex)
				}
				continue
			}
			hashList = append(hashList, hash)
		}
	}

	return hashList, nil
}

// addToCreatorIndexV2 将资源添加到创建者索引 (并发优化版本)
//
// 🎯 **per-item键设计**：
// - 键格式: index:creator:v2:{creator_address}:{content_hash} -> 1
// - 并发安全的创建者资源映射
func (m *Manager) addToCreatorIndexV2(ctx context.Context, tx storage.BadgerTransaction, creatorAddress string, contentHash []byte) error {
	if creatorAddress == "" {
		return nil // 没有创建者信息，跳过索引
	}

	contentHashHex := hex.EncodeToString(contentHash)
	creatorKey := indexCreatorV2Prefix + creatorAddress + ":" + contentHashHex

	// 直接设置键值对
	if err := tx.Set([]byte(creatorKey), []byte("1")); err != nil {
		return fmt.Errorf("更新创建者索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到创建者索引v2: %s -> %s", creatorAddress, contentHashHex)
	}

	return nil
}

// removeFromCreatorIndexV2 从创建者索引中移除资源 (并发优化版本)
func (m *Manager) removeFromCreatorIndexV2(ctx context.Context, tx storage.BadgerTransaction, creatorAddress string, contentHash []byte) error {
	if creatorAddress == "" {
		return nil // 没有创建者信息，跳过
	}

	contentHashHex := hex.EncodeToString(contentHash)
	creatorKey := indexCreatorV2Prefix + creatorAddress + ":" + contentHashHex

	// 直接删除键
	if err := tx.Delete([]byte(creatorKey)); err != nil && err.Error() != "key not found" {
		return fmt.Errorf("删除创建者索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("从创建者索引v2移除: %s -> %s", creatorAddress, contentHashHex)
	}

	return nil
}

// getCreatorIndexV2 获取创建者索引中的资源列表 (并发优化版本)
func (m *Manager) getCreatorIndexV2(ctx context.Context, creatorAddress string) ([][]byte, error) {
	if creatorAddress == "" {
		return [][]byte{}, nil
	}

	// 使用前缀扫描获取所有相关资源
	creatorPrefix := indexCreatorV2Prefix + creatorAddress + ":"
	indexData, err := m.badgerStore.PrefixScan(ctx, []byte(creatorPrefix))
	if err != nil {
		return nil, fmt.Errorf("获取创建者索引v2失败: %w", err)
	}

	var hashList [][]byte
	for keyStr, _ := range indexData {
		// 从键中提取哈希: index:creator:v2:{address}:{content_hash}
		parts := strings.Split(keyStr, ":")
		if len(parts) >= 4 {
			contentHashHex := parts[len(parts)-1] // 最后一部分是哈希
			hash, err := hex.DecodeString(contentHashHex)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("解析创建者索引v2哈希失败，跳过: %s", contentHashHex)
				}
				continue
			}
			hashList = append(hashList, hash)
		}
	}

	return hashList, nil
}

// addToNameIndexV2 将资源添加到名称索引 (并发优化版本)
//
// 🎯 **per-item键设计**：
// - 键格式: index:name:v2:{normalized_name}:{content_hash} -> 1
// - 支持同名资源的多个版本共存
func (m *Manager) addToNameIndexV2(ctx context.Context, tx storage.BadgerTransaction, resourceName string, contentHash []byte) error {
	if resourceName == "" {
		return nil // 没有名称，跳过索引
	}

	// 标准化资源名称（转小写，用于搜索）
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	if normalizedName == "" {
		return nil
	}

	contentHashHex := hex.EncodeToString(contentHash)
	nameKey := indexNameV2Prefix + normalizedName + ":" + contentHashHex

	// 直接设置键值对
	if err := tx.Set([]byte(nameKey), []byte("1")); err != nil {
		return fmt.Errorf("更新名称索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("添加到名称索引v2: %s -> %s", normalizedName, contentHashHex)
	}

	return nil
}

// removeFromNameIndexV2 从名称索引中移除资源 (并发优化版本)
func (m *Manager) removeFromNameIndexV2(ctx context.Context, tx storage.BadgerTransaction, resourceName string, contentHash []byte) error {
	if resourceName == "" {
		return nil // 没有名称，跳过
	}

	// 标准化资源名称
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	if normalizedName == "" {
		return nil
	}

	contentHashHex := hex.EncodeToString(contentHash)
	nameKey := indexNameV2Prefix + normalizedName + ":" + contentHashHex

	// 直接删除键
	if err := tx.Delete([]byte(nameKey)); err != nil && err.Error() != "key not found" {
		return fmt.Errorf("删除名称索引v2失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("从名称索引v2移除: %s -> %s", normalizedName, contentHashHex)
	}

	return nil
}

// getResourcesByNameV2 按名称获取资源哈希列表 (并发优化版本)
func (m *Manager) getResourcesByNameV2(ctx context.Context, resourceName string) ([][]byte, error) {
	if resourceName == "" {
		return [][]byte{}, nil
	}

	// 标准化资源名称
	normalizedName := strings.ToLower(strings.TrimSpace(resourceName))
	namePrefix := indexNameV2Prefix + normalizedName + ":"

	// 使用前缀扫描获取所有相关资源
	indexData, err := m.badgerStore.PrefixScan(ctx, []byte(namePrefix))
	if err != nil {
		return nil, fmt.Errorf("获取名称索引v2失败: %w", err)
	}

	var hashList [][]byte
	for keyStr, _ := range indexData {
		// 从键中提取哈希: index:name:v2:{normalized_name}:{content_hash}
		parts := strings.Split(keyStr, ":")
		if len(parts) >= 4 {
			contentHashHex := parts[len(parts)-1] // 最后一部分是哈希
			hash, err := hex.DecodeString(contentHashHex)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("解析名称索引v2哈希失败，跳过: %s", contentHashHex)
				}
				continue
			}
			hashList = append(hashList, hash)
		}
	}

	return hashList, nil
}
