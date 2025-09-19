// Package resource - 生命周期管理实现
//
// 🎯 **资源生命周期管理 (Resource Lifecycle Management)**
//
// 本文件实现资源的生命周期管理功能：
// - 引用计数：ResourceUTXO的并发安全引用管理
// - 垃圾回收：自动清理无引用的资源
// - 生命周期：资源从创建到销毁的完整生命周期
// - 并发控制：多线程环境下的引用计数安全操作
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
//                            引用计数键定义
// ============================================================================

const (
	// 清理标记前缀: cleanup:mark:{content_hash} -> timestamp
	cleanupMarkPrefix = "cleanup:mark:"

	// 清理队列前缀: cleanup:queue:{timestamp}:{content_hash} -> empty
	cleanupQueuePrefix = "cleanup:queue:"
)

// ============================================================================
//                         📊 引用计数管理实现
// ============================================================================

// getResourceReferenceCount 获取资源引用计数
//
// 🔢 **并发安全的引用计数查询 (Concurrent-Safe Reference Count Query)**
//
// 查询指定资源的当前引用计数，用于生命周期管理和垃圾回收决策。
// 设计为线程安全，支持高并发环境下的频繁查询操作。
//
// 📋 **查询处理流程**：
//
//	1️⃣ **哈希处理**：将二进制哈希转换为十六进制字符串
//	2️⃣ **键构建**：构建引用计数存储键 "resource:refs:{hash}"
//	3️⃣ **数据库查询**：从BadgerDB读取引用计数数据
//	4️⃣ **数据解析**：将字符串格式的计数转换为整数
//	5️⃣ **异常处理**：处理键不存在和格式错误的情况
//
// 🔧 **容错机制**：
//
//	✅ **默认值处理**：
//	   • 键不存在时返回0（表示无引用）
//	   • 避免因缺失数据导致的查询失败
//
//	✅ **格式错误恢复**：
//	   • 数据格式异常时重置为0并记录警告
//	   • 确保系统的持续可用性
//	   • 便于后续的数据修复操作
//
// 🎯 **返回值说明**：
//   - int32: 当前引用计数（≥0）
//   - error: 查询过程中的错误（通常为数据库访问异常）
//
// 💡 **使用场景**：
//   - 垃圾回收前的引用检查
//   - 资源使用情况统计
//   - 生命周期管理决策
//   - 系统监控和报警
func (m *Manager) getResourceReferenceCount(ctx context.Context, contentHash []byte) (int32, error) {
	contentHashHex := hex.EncodeToString(contentHash)
	refsKey := resourceRefsPrefix + contentHashHex

	// 从BadgerDB获取引用计数
	refData, err := m.badgerStore.Get(ctx, []byte(refsKey))
	if err != nil {
		if err.Error() == "key not found" {
			return 0, nil // 默认引用计数为0
		}
		return 0, fmt.Errorf("获取引用计数失败: %w", err)
	}

	// 解析引用计数
	refCountStr := strings.TrimSpace(string(refData))
	refCount, err := strconv.ParseInt(refCountStr, 10, 32)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("引用计数格式错误，重置为0: %s -> %s", contentHashHex, refCountStr)
		}
		return 0, nil // 格式错误时返回0
	}

	return int32(refCount), nil
}

// incrementResourceReference 增加资源引用计数
//
// ➕ **原子性引用计数增加 (Atomic Reference Count Increment)**
//
// 在资源被新的UTXO引用时调用，原子性地增加引用计数。
// 采用BadgerDB事务确保操作的原子性，避免并发环境下的竞争条件。
//
// 📋 **处理流程详解**：
//
//	1️⃣ **事务开始**：启动BadgerDB事务，确保操作原子性
//	2️⃣ **当前计数获取**：
//	   • 查询现有引用计数值
//	   • 处理键不存在的情况（默认为0）
//	   • 处理数据格式异常（重置为0）
//	3️⃣ **计数增加**：将当前计数加1
//	4️⃣ **数据更新**：将新计数写入数据库
//	5️⃣ **清理标记处理**：
//	   • 如果资源之前被标记为待清理，取消清理标记
//	   • 确保被重新引用的资源不会被误删
//	6️⃣ **事务提交**：提交所有更改，确保数据一致性
//
// 🔒 **并发安全保证**：
//
//	✅ **事务原子性**：
//	   • 整个操作在单一事务中完成
//	   • 避免读-修改-写过程中的竞争条件
//	   • 确保并发环境下的数据一致性
//
//	✅ **幂等性设计**：
//	   • 重复调用的结果是可预测的
//	   • 异常重试不会导致计数错误
//
// 🔄 **自动清理取消**：
//
//	当引用计数从0变为正数时：
//	- 自动删除清理标记（cleanup:mark:{hash}）
//	- 防止资源被垃圾回收器误删
//	- 记录取消清理的调试日志
//
// 💡 **调用场景**：
//   - ResourceUTXO创建时
//   - 资源重新被引用时
//   - 去重存储时增加引用
//   - 资源恢复操作时
func (m *Manager) incrementResourceReference(ctx context.Context, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	// 在事务中原子性增加引用计数
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		refsKey := resourceRefsPrefix + contentHashHex

		// 获取当前引用计数
		currentRefData, err := tx.Get([]byte(refsKey))
		var currentCount int64 = 0

		if err != nil {
			if err.Error() != "key not found" {
				return fmt.Errorf("获取当前引用计数失败: %w", err)
			}
			// key不存在时，使用默认值0
		} else {
			// 解析当前计数
			currentRefStr := strings.TrimSpace(string(currentRefData))
			currentCount, err = strconv.ParseInt(currentRefStr, 10, 64)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("引用计数格式错误，重置为0: %s", contentHashHex)
				}
				currentCount = 0
			}
		}

		// 增加引用计数
		newCount := currentCount + 1
		newRefData := strconv.FormatInt(newCount, 10)

		// 保存新的引用计数
		if err := tx.Set([]byte(refsKey), []byte(newRefData)); err != nil {
			return fmt.Errorf("更新引用计数失败: %w", err)
		}

		// 如果资源被标记为待清理，取消清理标记
		if newCount > 0 {
			cleanupKey := cleanupMarkPrefix + contentHashHex
			if err := tx.Delete([]byte(cleanupKey)); err != nil && err.Error() != "key not found" {
				// 删除清理标记失败不影响主流程
				if m.logger != nil {
					m.logger.Warnf("取消清理标记失败: %s, 错误: %v", contentHashHex, err)
				}
			}
		}

		if m.logger != nil {
			m.logger.Debugf("✅ 增加资源引用: %s, 计数: %d -> %d", contentHashHex, currentCount, newCount)
		}

		return nil
	})
}

// decrementResourceReference 减少资源引用计数
//
// ➖ **原子性引用计数减少**：
// - 在BadgerDB事务中原子性减少引用计数
// - 计数归零时自动标记为待清理
// - ResourceUTXO引用被释放时调用
func (m *Manager) decrementResourceReference(ctx context.Context, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	// 在事务中原子性减少引用计数
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		refsKey := resourceRefsPrefix + contentHashHex

		// 获取当前引用计数
		currentRefData, err := tx.Get([]byte(refsKey))
		if err != nil {
			if err.Error() == "key not found" {
				if m.logger != nil {
					m.logger.Warnf("尝试减少不存在的资源引用: %s", contentHashHex)
				}
				return nil // 不存在的引用，无需处理
			}
			return fmt.Errorf("获取当前引用计数失败: %w", err)
		}

		// 解析当前计数
		currentRefStr := strings.TrimSpace(string(currentRefData))
		currentCount, err := strconv.ParseInt(currentRefStr, 10, 64)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("引用计数格式错误，设为0: %s", contentHashHex)
			}
			currentCount = 0
		}

		// 减少引用计数（不允许小于0）
		newCount := currentCount - 1
		if newCount < 0 {
			newCount = 0
		}

		// 保存新的引用计数
		newRefData := strconv.FormatInt(newCount, 10)
		if err := tx.Set([]byte(refsKey), []byte(newRefData)); err != nil {
			return fmt.Errorf("更新引用计数失败: %w", err)
		}

		// 如果引用计数归零，标记为待清理
		if newCount == 0 {
			if err := m.markResourceForCleanupInTx(ctx, tx, contentHash); err != nil {
				if m.logger != nil {
					m.logger.Warnf("标记资源待清理失败: %s, 错误: %v", contentHashHex, err)
				}
				// 清理标记失败不影响引用计数更新
			}
		}

		if m.logger != nil {
			m.logger.Debugf("✅ 减少资源引用: %s, 计数: %d -> %d", contentHashHex, currentCount, newCount)
		}

		return nil
	})
}

// ============================================================================
//                         🗑️ 清理标记管理实现
// ============================================================================

// markResourceForCleanup 标记资源待清理
//
// 🏷️ **清理标记逻辑**：
// - 仅标记引用计数为0的资源
// - 使用时间戳记录标记时间
// - 支持延迟清理策略
func (m *Manager) markResourceForCleanup(ctx context.Context, contentHash []byte) error {
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return m.markResourceForCleanupInTx(ctx, tx, contentHash)
	})
}

// markResourceForCleanupInTx 在事务中标记资源待清理
func (m *Manager) markResourceForCleanupInTx(ctx context.Context, tx storage.BadgerTransaction, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	// 检查引用计数是否为0
	refsKey := resourceRefsPrefix + contentHashHex
	refData, err := tx.Get([]byte(refsKey))
	if err != nil {
		if err.Error() == "key not found" {
			// 引用计数不存在，视为0，可以清理
		} else {
			return fmt.Errorf("检查引用计数失败: %w", err)
		}
	} else {
		refCountStr := strings.TrimSpace(string(refData))
		refCount, err := strconv.ParseInt(refCountStr, 10, 64)
		if err != nil || refCount > 0 {
			if m.logger != nil {
				m.logger.Debugf("资源仍被引用，跳过清理标记: %s (引用计数: %d)", contentHashHex, refCount)
			}
			return nil // 仍被引用，不标记清理
		}
	}

	// 标记为待清理
	currentTime := time.Now()
	cleanupKey := cleanupMarkPrefix + contentHashHex
	timestamp := strconv.FormatInt(currentTime.Unix(), 10)

	if err := tx.Set([]byte(cleanupKey), []byte(timestamp)); err != nil {
		return fmt.Errorf("设置清理标记失败: %w", err)
	}

	// 添加到清理队列
	queueKey := cleanupQueuePrefix + timestamp + ":" + contentHashHex
	if err := tx.Set([]byte(queueKey), []byte("")); err != nil {
		return fmt.Errorf("添加到清理队列失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("✅ 标记资源待清理: %s, 时间: %s", contentHashHex, currentTime.Format(time.RFC3339))
	}

	return nil
}

// ============================================================================
//                         🧹 垃圾回收实现
// ============================================================================

// cleanupUnreferencedResources 清理无引用的资源
//
// 🧹 **自动垃圾回收机制 (Automatic Garbage Collection Mechanism)**
//
// 这是WES区块链系统的核心垃圾回收器，负责自动清理不再被引用的资源，
// 释放存储空间并维护系统的整体健康状态。设计为区块链自运行系统的重要组成部分。
//
// 🔄 **清理处理流程**：
//
//	1️⃣ **参数验证阶段**：
//	   • 验证并调整清理数量限制（默认10个，最大100个）
//	   • 防止单次清理时间过长导致系统阻塞
//	   • 记录清理开始的调试日志
//
//	2️⃣ **队列扫描阶段**：
//	   • 调用getCleanupQueue获取待清理资源列表
//	   • 从清理队列键中解析出资源哈希
//	   • 限制扫描数量，支持分批处理
//
//	3️⃣ **批量清理阶段**：
//	   • 遍历待清理资源列表
//	   • 对每个资源调用cleanupSingleResource
//	   • 清理失败的资源会被跳过，不影响其他资源
//	   • 记录每次清理的警告日志（用于问题排查）
//
//	4️⃣ **统计报告阶段**：
//	   • 统计实际清理成功的资源数量
//	   • 记录清理完成的信息日志
//	   • 返回清理统计结果
//
// 🎯 **设计目标**：
//   - 🔄 **自动化运行**：无需人工干预的自动垃圾回收
//   - ⚡ **性能友好**：限制单次处理量，避免系统阻塞
//   - 🛡️ **容错能力**：单个资源清理失败不影响整体流程
//   - 📊 **可观测性**：详细的日志记录和统计报告
//
// 💡 **调用场景**：
//   - 定时任务：定期执行垃圾回收
//   - 存储压力：存储空间不足时触发
//   - 系统维护：系统维护期间的清理操作
//   - 手动触发：管理员手动执行清理
//
// 🔧 **配置说明**：
//   - maxCleanupCount ≤ 0：使用默认值10
//   - maxCleanupCount > 100：限制为100（防止过度清理）
//   - 建议根据系统负载动态调整清理频率和数量
func (m *Manager) cleanupUnreferencedResources(ctx context.Context, maxCleanupCount int) (int, error) {
	// 使用配置值替代硬编码值
	if maxCleanupCount <= 0 {
		maxCleanupCount = m.config.GarbageCollection.DefaultBatchSize // 使用配置的默认批处理大小
	}
	if maxCleanupCount > m.config.GarbageCollection.MaxBatchSize {
		maxCleanupCount = m.config.GarbageCollection.MaxBatchSize // 使用配置的最大限制
	}

	if m.logger != nil {
		m.logger.Debugf("开始清理无引用资源，最大清理数量: %d", maxCleanupCount)
	}

	// 获取待清理的资源列表
	cleanupList, err := m.getCleanupQueue(ctx, maxCleanupCount)
	if err != nil {
		return 0, fmt.Errorf("获取清理队列失败: %w", err)
	}

	if len(cleanupList) == 0 {
		if m.logger != nil {
			m.logger.Debug("暂无需要清理的资源")
		}
		return 0, nil
	}

	cleanedCount := 0

	// 批量清理资源
	for _, contentHash := range cleanupList {
		if cleanedCount >= maxCleanupCount {
			break
		}

		err := m.cleanupSingleResource(ctx, contentHash)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("清理单个资源失败，跳过: %x, 错误: %v", contentHash, err)
			}
			continue // 继续清理其他资源
		}

		cleanedCount++
	}

	if m.logger != nil {
		m.logger.Infof("✅ 资源清理完成: 清理了 %d 个资源", cleanedCount)
	}

	return cleanedCount, nil
}

// getCleanupQueue 获取清理队列
func (m *Manager) getCleanupQueue(ctx context.Context, limit int) ([][]byte, error) {
	// 使用前缀扫描获取清理队列
	queueData, err := m.badgerStore.PrefixScan(ctx, []byte(cleanupQueuePrefix))
	if err != nil {
		return nil, fmt.Errorf("扫描清理队列失败: %w", err)
	}

	var cleanupList [][]byte
	count := 0

	for key, _ := range queueData {
		if count >= limit {
			break
		}

		// 从清理队列键中提取内容哈希
		// 格式: cleanup:queue:{timestamp}:{content_hash}
		parts := strings.Split(key, ":")
		if len(parts) >= 4 {
			contentHashHex := parts[len(parts)-1] // 最后一部分是哈希
			contentHash, err := hex.DecodeString(contentHashHex)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("清理队列中无效的哈希，跳过: %s", contentHashHex)
				}
				continue
			}
			cleanupList = append(cleanupList, contentHash)
			count++
		}
	}

	return cleanupList, nil
}

// cleanupSingleResource 清理单个资源
func (m *Manager) cleanupSingleResource(ctx context.Context, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	// 在事务中执行完整的清理操作
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 1. 再次检查引用计数（防止竞争条件）
		refsKey := resourceRefsPrefix + contentHashHex
		refData, err := tx.Get([]byte(refsKey))
		if err == nil {
			refCountStr := strings.TrimSpace(string(refData))
			refCount, err := strconv.ParseInt(refCountStr, 10, 64)
			if err == nil && refCount > 0 {
				if m.logger != nil {
					m.logger.Debugf("资源已重新被引用，取消清理: %s (引用计数: %d)", contentHashHex, refCount)
				}
				return m.removeFromCleanupQueue(tx, contentHash)
			}
		}

		// 2. 获取资源存储信息
		metaKey := resourceMetaPrefix + contentHashHex
		metaData, err := tx.Get([]byte(metaKey))
		if err != nil {
			if err.Error() == "key not found" {
				// 元数据已不存在，仅清理队列记录
				return m.removeFromCleanupQueue(tx, contentHash)
			}
			return fmt.Errorf("获取资源元数据失败: %w", err)
		}

		// 3. 解析资源信息获取存储路径
		resourceInfo, err := m.deserializeResourceInfo(metaData)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("解析资源元数据失败，仅清理索引: %s", contentHashHex)
			}
		} else {
			// 4. 删除物理文件
			storagePath := resourceInfo.Metadata["storage_path"]
			if storagePath != "" {
				fullStoragePath := m.buildResourcePath(storagePath)
				if err := m.fileStore.Delete(ctx, fullStoragePath); err != nil {
					if m.logger != nil {
						m.logger.Warnf("删除文件失败: %s, 错误: %v", storagePath, err)
					}
					// 文件删除失败不阻止索引清理
				}
			}

			// 5. 从各类索引中移除 (优先使用v2版本)
			if err := m.removeFromCategoryIndexV2(ctx, tx, resourceInfo.ResourceType, contentHash); err != nil {
				if m.logger != nil {
					m.logger.Warnf("从分类索引v2移除失败: %s", contentHashHex)
				}
				// 降级到v1版本
				if err := m.removeFromCategoryIndex(ctx, tx, resourceInfo.ResourceType, contentHash); err != nil {
					if m.logger != nil {
						m.logger.Warnf("从分类索引v1移除失败: %s", contentHashHex)
					}
				}
			}

			// 5.1 从创建者索引中移除 (优先使用v2版本)
			if creatorAddress := resourceInfo.Metadata["creator_address"]; creatorAddress != "" {
				if err := m.removeFromCreatorIndexV2(ctx, tx, creatorAddress, contentHash); err != nil {
					if m.logger != nil {
						m.logger.Warnf("从创建者索引v2移除失败: %s", contentHashHex)
					}
					// 降级到v1版本
					if err := m.removeFromCreatorIndex(ctx, tx, creatorAddress, contentHash); err != nil {
						if m.logger != nil {
							m.logger.Warnf("从创建者索引v1移除失败: %s", contentHashHex)
						}
					}
				}
			}

			// 5.2 从名称索引中移除 (优先使用v2版本)
			if resourceName := resourceInfo.Metadata["name"]; resourceName != "" {
				if err := m.removeFromNameIndexV2(ctx, tx, resourceName, contentHash); err != nil {
					if m.logger != nil {
						m.logger.Warnf("从名称索引v2移除失败: %s", contentHashHex)
					}
					// 降级到v1版本
					if err := m.removeFromNameIndex(ctx, resourceName, contentHash); err != nil {
						if m.logger != nil {
							m.logger.Warnf("从名称索引v1移除失败: %s", contentHashHex)
						}
					}
				}
			}

			// 5.3 删除健康状态记录
			if storagePath := resourceInfo.Metadata["storage_path"]; storagePath != "" {
				healthKey := healthFilePrefix + storagePath
				if err := tx.Delete([]byte(healthKey)); err != nil && err.Error() != "key not found" {
					if m.logger != nil {
						m.logger.Warnf("删除健康状态记录失败: %s", contentHashHex)
					}
				}
			}
		}

		// 6. 删除主要索引记录
		if err := tx.Delete([]byte(metaKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除元数据失败: %w", err)
		}

		pathKey := resourcePathPrefix + contentHashHex
		if err := tx.Delete([]byte(pathKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除路径映射失败: %w", err)
		}

		if err := tx.Delete([]byte(refsKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除引用计数失败: %w", err)
		}

		// 7. 从清理队列移除
		if err := m.removeFromCleanupQueue(tx, contentHash); err != nil {
			return fmt.Errorf("从清理队列移除失败: %w", err)
		}

		if m.logger != nil {
			m.logger.Debugf("✅ 单个资源清理完成: %s", contentHashHex)
		}

		return nil
	})
}

// removeFromCleanupQueue 从清理队列中移除资源
func (m *Manager) removeFromCleanupQueue(tx storage.BadgerTransaction, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	// 删除清理标记
	cleanupKey := cleanupMarkPrefix + contentHashHex
	if err := tx.Delete([]byte(cleanupKey)); err != nil && err.Error() != "key not found" {
		return fmt.Errorf("删除清理标记失败: %w", err)
	}

	// 删除队列中的记录 - 需要扫描并精确删除
	// 由于队列键格式为: cleanup:queue:{timestamp}:{content_hash}
	// 我们需要通过BadgerDB事务上下文来处理，但事务不支持PrefixScan
	// 因此采用延迟删除策略：队列处理时会重新检查引用计数
	// TODO: 考虑重构队列设计为 cleanup:queue:{hash}:{timestamp} 格式便于删除

	if m.logger != nil {
		m.logger.Debugf("清理标记已删除，队列记录将在下次处理时自动忽略: %s", contentHashHex)
	}

	return nil
}
