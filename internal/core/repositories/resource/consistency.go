// Package resource - 一致性检查与修复实现
//
// 🎯 **存储一致性管理 (Storage Consistency Management)**
//
// 本文件实现资源存储的一致性检查与自动修复功能：
// - 完整性验证：单个资源的文件与索引一致性检查
// - 批量检查：系统级的存储一致性扫描
// - 自动修复：检测并修复各种不一致状态
// - 故障恢复：系统启动时的自动一致性恢复
// - 自愈能力：区块链自运行系统的重要保障
package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//                              一致性检查状态
// ============================================================================

// ConsistencyIssueType 一致性问题类型
type ConsistencyIssueType string

const (
	IssueOrphanFile       ConsistencyIssueType = "orphan_file"       // 孤儿文件：文件存在但索引缺失
	IssueOrphanIndex      ConsistencyIssueType = "orphan_index"      // 孤儿索引：索引存在但文件缺失
	IssueHashMismatch     ConsistencyIssueType = "hash_mismatch"     // 哈希不匹配：文件内容与索引不符
	IssueSizeInconsistent ConsistencyIssueType = "size_inconsistent" // 大小不一致：文件大小与元数据不符
	IssueCorruptedFile    ConsistencyIssueType = "corrupted_file"    // 文件损坏：无法读取或计算哈希
	IssueCorruptedMeta    ConsistencyIssueType = "corrupted_meta"    // 元数据损坏：无法解析元数据
)

// ConsistencyIssue 一致性问题记录
type ConsistencyIssue struct {
	Type        ConsistencyIssueType `json:"type"`         // 问题类型
	ContentHash []byte               `json:"content_hash"` // 资源哈希
	FilePath    string               `json:"file_path"`    // 文件路径
	Description string               `json:"description"`  // 问题描述
	DetectedAt  time.Time            `json:"detected_at"`  // 检测时间
	Fixed       bool                 `json:"fixed"`        // 是否已修复
	FixError    string               `json:"fix_error"`    // 修复错误信息
}

// ConsistencyReport 一致性检查报告
type ConsistencyReport struct {
	StartTime    time.Time          `json:"start_time"`    // 检查开始时间
	EndTime      time.Time          `json:"end_time"`      // 检查结束时间
	TotalChecked int                `json:"total_checked"` // 总检查数量
	IssuesFound  int                `json:"issues_found"`  // 发现问题数量
	IssuesFixed  int                `json:"issues_fixed"`  // 修复问题数量
	Issues       []ConsistencyIssue `json:"issues"`        // 问题详情
	Summary      map[string]int     `json:"summary"`       // 问题类型统计
}

// ============================================================================
//                         🔍 单资源一致性验证
// ============================================================================

// verifyResourceIntegrity 验证单个资源的存储完整性
//
// 🔍 **资源完整性深度验证 (Deep Resource Integrity Verification)**
//
// 对指定资源执行全面的完整性检查，验证文件存储与索引数据的一致性。
// 这是系统自愈机制的核心组件，确保资源数据的长期可靠性。
//
// 📋 **验证流程详解**：
//
//	1️⃣ **元数据验证阶段**：
//	   • 检查资源元数据是否存在于BadgerDB中
//	   • 尝试反序列化元数据，验证数据结构完整性
//	   • 提取存储路径等关键信息
//
//	2️⃣ **文件存在性检查**：
//	   • 验证物理文件在FileStore中是否存在
//	   • 检查文件是否可访问和可读
//	   • 更新文件不存在时的健康状态
//
//	3️⃣ **文件属性验证**：
//	   • 获取实际文件的大小信息
//	   • 与元数据中记录的大小进行对比
//	   • 检测文件是否被外部修改或损坏
//
//	4️⃣ **内容完整性检查**：
//	   • 重新计算文件的SHA-256哈希值
//	   • 与索引中存储的原始哈希对比
//	   • 使用流式计算支持大文件验证
//
//	5️⃣ **健康状态更新**：
//	   • 验证通过：更新为健康状态，记录验证时间
//	   • 验证失败：标记为不健康，便于后续修复
//
// 🛠️ **验证项目清单**：
//
//	✅ **数据存在性**：元数据索引 + 物理文件
//	✅ **数据可解析性**：元数据格式 + 文件可读性
//	✅ **数据一致性**：文件大小 + 内容哈希
//	✅ **数据完整性**：哈希重计算 + 对比验证
//
// 🔧 **容错处理**：
//   - 任何验证失败都会更新健康状态
//   - 详细的错误信息记录便于问题定位
//   - 部分验证失败不影响其他验证项
//
// 🎯 **健康状态管理**：
//   - 成功验证：记录OK状态和验证时间戳
//   - 失败验证：记录ERROR状态和失败原因
//   - 状态数据用于系统监控和自动修复决策
//
// 💡 **调用场景**：
//   - 系统启动时的自检
//   - 定期健康检查任务
//   - 用户访问资源前的验证
//   - 存储故障后的数据验证
func (m *Manager) verifyResourceIntegrity(ctx context.Context, contentHash []byte) error {
	contentHashHex := hex.EncodeToString(contentHash)

	if m.logger != nil {
		m.logger.Debugf("验证资源完整性: %s", contentHashHex)
	}

	// 1. 检查并获取元数据
	metaKey := resourceMetaPrefix + contentHashHex
	metaData, err := m.badgerStore.Get(ctx, []byte(metaKey))
	if err != nil {
		if err.Error() == "key not found" {
			return fmt.Errorf("资源元数据不存在: %s", contentHashHex)
		}
		return fmt.Errorf("获取资源元数据失败: %w", err)
	}

	// 2. 解析元数据
	resourceInfo, err := m.deserializeResourceInfo(metaData)
	if err != nil {
		// 更新健康状态为不健康
		if storagePath := resourceInfo.Metadata["storage_path"]; storagePath != "" {
			m.updateHealthStatus(ctx, storagePath, false)
		}
		return fmt.Errorf("解析资源元数据失败: %w", err)
	}

	storagePath := resourceInfo.Metadata["storage_path"]
	if storagePath == "" {
		return fmt.Errorf("资源存储路径为空: %s", contentHashHex)
	}

	// 3. 检查文件是否存在
	exists, err := m.fileStore.Exists(ctx, storagePath)
	if err != nil {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("检查文件存在性失败: %w", err)
	}

	if !exists {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("资源文件不存在: %s", storagePath)
	}

	// 4. 检查文件大小
	fileInfo, err := m.fileStore.FileInfo(ctx, storagePath)
	if err != nil {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	if fileInfo.Size != resourceInfo.Size {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("文件大小不匹配: 期望 %d，实际 %d", resourceInfo.Size, fileInfo.Size)
	}

	// 5. 重新计算文件哈希
	actualHash, err := m.computeFileHash(ctx, storagePath)
	if err != nil {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 6. 对比哈希值
	expectedHashHex := hex.EncodeToString(resourceInfo.ContentHash)
	actualHashHex := hex.EncodeToString(actualHash)

	if expectedHashHex != actualHashHex {
		m.updateHealthStatus(ctx, storagePath, false)
		return fmt.Errorf("文件哈希不匹配: 期望 %s，实际 %s", expectedHashHex, actualHashHex)
	}

	// 7. 验证通过，更新健康状态
	if err := m.updateHealthStatus(ctx, storagePath, true); err != nil {
		if m.logger != nil {
			m.logger.Warnf("更新健康状态失败: %s, 错误: %v", storagePath, err)
		}
	}

	if m.logger != nil {
		m.logger.Debugf("✅ 资源完整性验证通过: %s", contentHashHex)
	}

	return nil
}

// computeFileHash 计算指定文件的SHA-256哈希
func (m *Manager) computeFileHash(ctx context.Context, filePath string) ([]byte, error) {
	// 使用流式读取计算哈希，支持大文件
	stream, err := m.fileStore.OpenReadStream(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件流失败: %w", err)
	}
	defer stream.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, stream)
	if err != nil {
		return nil, fmt.Errorf("流式哈希计算失败: %w", err)
	}

	return hasher.Sum(nil), nil
}

// ============================================================================
//                         🔧 系统级一致性修复
// ============================================================================

// repairStorageInconsistency 修复存储不一致状态
//
// 🔧 **系统级一致性修复引擎 (System-Level Consistency Repair Engine)**
//
// 这是WES系统的核心自愈机制，负责检测和修复文件存储与索引之间的各种不一致状态。
// 设计为区块链自运行系统的重要组件，确保数据的长期完整性和可靠性。
//
// 📋 **详细修复流程**：
//
//	1️⃣ **全局扫描阶段**：
//	   • 使用PrefixScan扫描所有资源元数据索引
//	   • 获取系统中所有已注册资源的完整列表
//	   • 统计待检查资源总数，记录调试信息
//
//	2️⃣ **逐一检查阶段**：
//	   • 遍历每个资源的元数据记录
//	   • 解析资源哈希和相关信息
//	   • 调用checkSingleResourceConsistency进行深度检查
//	   • 收集发现的各类一致性问题
//
//	3️⃣ **问题分类统计**：
//	   • 将发现的问题按类型分类统计：
//	     - IssueOrphanFile：孤儿文件（文件存在但索引缺失）
//	     - IssueOrphanIndex：孤儿索引（索引存在但文件缺失）
//	     - IssueHashMismatch：哈希不匹配（文件内容与索引不符）
//	     - IssueSizeInconsistent：大小不一致（文件大小与元数据不符）
//	     - IssueCorruptedFile：文件损坏（无法读取或计算哈希）
//	     - IssueCorruptedMeta：元数据损坏（无法解析元数据）
//
//	4️⃣ **自动修复阶段**：
//	   • 对每个发现的问题尝试自动修复：
//	     - attemptFixConsistencyIssue执行具体修复逻辑
//	     - 成功修复的问题更新统计计数
//	     - 无法修复的问题记录错误信息
//
//	5️⃣ **报告生成阶段**：
//	   • 生成完整的ConsistencyReport：
//	     - 检查开始和结束时间
//	     - 总检查数量、发现问题数量、修复数量
//	     - 详细的问题列表和类型统计
//	   • 记录修复完成的信息日志和性能统计
//
// 🛠️ **修复能力范围**：
//
//	✅ **可自动修复**：
//	   - 孤儿索引：删除无效的索引记录
//	   - 损坏的元数据：删除损坏的记录
//	   - 损坏的文件：删除相关记录和文件
//	   - 不一致记录：删除不匹配的记录和文件
//
//	⚠️ **需要人工干预**：
//	   - 孤儿文件：需要额外的资源信息才能重建索引
//	   - 复杂的数据损坏：可能需要从备份恢复
//
// 🎯 **设计原则**：
//   - 🛡️ **安全优先**：采用保守策略，宁可删除也不创建错误数据
//   - 🔄 **自动化**：无需人工干预的自动检测和修复
//   - 📊 **可观测**：详细的报告和日志记录
//   - ⚡ **性能友好**：批量处理，避免长时间阻塞
//
// 💡 **调用时机**：
//   - 系统启动时的自检
//   - 定期维护任务
//   - 存储异常后的恢复
//   - 升级迁移后的验证
func (m *Manager) repairStorageInconsistency(ctx context.Context) (int, error) {
	if m.logger != nil {
		m.logger.Info("开始系统级存储一致性修复")
	}

	startTime := time.Now()
	report := &ConsistencyReport{
		StartTime: startTime,
		Issues:    []ConsistencyIssue{},
		Summary:   make(map[string]int),
	}

	// 1. 扫描所有资源元数据
	metaPrefix := []byte(resourceMetaPrefix)
	metaData, err := m.badgerStore.PrefixScan(ctx, metaPrefix)
	if err != nil {
		return 0, fmt.Errorf("扫描资源元数据失败: %w", err)
	}

	report.TotalChecked = len(metaData)

	if m.logger != nil {
		m.logger.Debugf("发现 %d 个资源需要检查一致性", report.TotalChecked)
	}

	// 2. 逐一检查每个资源
	for metaKeyStr, metaBytes := range metaData {
		// 提取内容哈希
		contentHashHex := strings.TrimPrefix(metaKeyStr, resourceMetaPrefix)
		contentHash, err := hex.DecodeString(contentHashHex)
		if err != nil {
			issue := ConsistencyIssue{
				Type:        IssueCorruptedMeta,
				ContentHash: nil,
				FilePath:    "",
				Description: fmt.Sprintf("无效的哈希格式: %s", contentHashHex),
				DetectedAt:  time.Now(),
				Fixed:       false,
			}
			report.Issues = append(report.Issues, issue)
			report.IssuesFound++
			report.Summary[string(IssueCorruptedMeta)]++
			continue
		}

		// 检查单个资源的一致性
		issues := m.checkSingleResourceConsistency(ctx, contentHash, metaBytes)
		for _, issue := range issues {
			report.Issues = append(report.Issues, issue)
			report.IssuesFound++
			report.Summary[string(issue.Type)]++

			// 尝试修复问题
			if m.attemptFixConsistencyIssue(ctx, &issue) {
				report.IssuesFixed++
				if m.logger != nil {
					m.logger.Debugf("修复一致性问题: %s - %s", issue.Type, issue.Description)
				}
			} else if issue.FixError != "" {
				if m.logger != nil {
					m.logger.Warnf("修复失败: %s - %s", issue.Type, issue.FixError)
				}
			}
		}
	}

	// 3. 完善报告
	report.EndTime = time.Now()

	if m.logger != nil {
		duration := report.EndTime.Sub(report.StartTime)
		m.logger.Infof("✅ 存储一致性修复完成: 检查 %d 个资源，发现 %d 个问题，修复 %d 个，耗时 %v",
			report.TotalChecked, report.IssuesFound, report.IssuesFixed, duration)

		// 打印问题统计
		for issueType, count := range report.Summary {
			m.logger.Debugf("问题类型 %s: %d 个", issueType, count)
		}
	}

	return report.IssuesFixed, nil
}

// checkSingleResourceConsistency 检查单个资源的一致性
func (m *Manager) checkSingleResourceConsistency(ctx context.Context, contentHash []byte, metaBytes []byte) []ConsistencyIssue {
	var issues []ConsistencyIssue

	// 1. 尝试解析元数据
	resourceInfo, err := m.deserializeResourceInfo(metaBytes)
	if err != nil {
		issue := ConsistencyIssue{
			Type:        IssueCorruptedMeta,
			ContentHash: contentHash,
			FilePath:    "",
			Description: fmt.Sprintf("元数据损坏: %v", err),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues // 元数据损坏时无法继续检查
	}

	storagePath := resourceInfo.Metadata["storage_path"]
	if storagePath == "" {
		issue := ConsistencyIssue{
			Type:        IssueCorruptedMeta,
			ContentHash: contentHash,
			FilePath:    "",
			Description: "存储路径为空",
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues
	}

	// 2. 检查文件是否存在
	exists, err := m.fileStore.Exists(ctx, storagePath)
	if err != nil {
		issue := ConsistencyIssue{
			Type:        IssueOrphanIndex,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: fmt.Sprintf("无法检查文件存在性: %v", err),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues
	}

	if !exists {
		issue := ConsistencyIssue{
			Type:        IssueOrphanIndex,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: "索引存在但文件缺失",
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues // 文件不存在时无法继续检查
	}

	// 3. 检查文件大小
	fileInfo, err := m.fileStore.FileInfo(ctx, storagePath)
	if err != nil {
		issue := ConsistencyIssue{
			Type:        IssueCorruptedFile,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: fmt.Sprintf("无法获取文件信息: %v", err),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues
	}

	if fileInfo.Size != resourceInfo.Size {
		issue := ConsistencyIssue{
			Type:        IssueSizeInconsistent,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: fmt.Sprintf("文件大小不匹配: 期望 %d，实际 %d", resourceInfo.Size, fileInfo.Size),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
	}

	// 4. 检查文件哈希
	actualHash, err := m.computeFileHash(ctx, storagePath)
	if err != nil {
		issue := ConsistencyIssue{
			Type:        IssueCorruptedFile,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: fmt.Sprintf("计算文件哈希失败: %v", err),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
		return issues
	}

	expectedHashHex := hex.EncodeToString(resourceInfo.ContentHash)
	actualHashHex := hex.EncodeToString(actualHash)

	if expectedHashHex != actualHashHex {
		issue := ConsistencyIssue{
			Type:        IssueHashMismatch,
			ContentHash: contentHash,
			FilePath:    storagePath,
			Description: fmt.Sprintf("文件哈希不匹配: 期望 %s，实际 %s", expectedHashHex, actualHashHex),
			DetectedAt:  time.Now(),
			Fixed:       false,
		}
		issues = append(issues, issue)
	}

	return issues
}

// attemptFixConsistencyIssue 尝试修复一致性问题
func (m *Manager) attemptFixConsistencyIssue(ctx context.Context, issue *ConsistencyIssue) bool {
	switch issue.Type {
	case IssueOrphanIndex:
		// 孤儿索引：删除无效的索引记录
		return m.fixOrphanIndex(ctx, issue)

	case IssueOrphanFile:
		// 孤儿文件：为文件重建索引（暂不实现，需要额外信息）
		issue.FixError = "孤儿文件修复需要额外的资源信息，暂不支持自动修复"
		return false

	case IssueCorruptedMeta:
		// 损坏的元数据：删除损坏的记录（保守策略）
		return m.fixCorruptedMetadata(ctx, issue)

	case IssueCorruptedFile:
		// 损坏的文件：删除相关记录
		return m.fixCorruptedFile(ctx, issue)

	case IssueHashMismatch, IssueSizeInconsistent:
		// 哈希或大小不匹配：删除不一致的记录（保守策略）
		return m.fixInconsistentRecord(ctx, issue)

	default:
		issue.FixError = "未知问题类型，无法修复"
		return false
	}
}

// fixOrphanIndex 修复孤儿索引问题
func (m *Manager) fixOrphanIndex(ctx context.Context, issue *ConsistencyIssue) bool {
	contentHashHex := hex.EncodeToString(issue.ContentHash)

	// 在事务中删除所有相关的索引记录
	err := m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 删除主元数据
		metaKey := resourceMetaPrefix + contentHashHex
		if err := tx.Delete([]byte(metaKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除元数据失败: %w", err)
		}

		// 删除路径映射
		pathKey := resourcePathPrefix + contentHashHex
		if err := tx.Delete([]byte(pathKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除路径映射失败: %w", err)
		}

		// 删除引用计数
		refsKey := resourceRefsPrefix + contentHashHex
		if err := tx.Delete([]byte(refsKey)); err != nil && err.Error() != "key not found" {
			return fmt.Errorf("删除引用计数失败: %w", err)
		}

		return nil
	})

	if err != nil {
		issue.FixError = err.Error()
		return false
	}

	issue.Fixed = true
	return true
}

// fixCorruptedMetadata 修复损坏的元数据
func (m *Manager) fixCorruptedMetadata(ctx context.Context, issue *ConsistencyIssue) bool {
	if issue.ContentHash == nil {
		issue.FixError = "内容哈希为空，无法定位记录"
		return false
	}

	return m.fixOrphanIndex(ctx, issue) // 与孤儿索引修复逻辑相同
}

// fixCorruptedFile 修复损坏的文件
func (m *Manager) fixCorruptedFile(ctx context.Context, issue *ConsistencyIssue) bool {
	// 删除损坏的文件和相关索引
	success := m.fixOrphanIndex(ctx, issue)

	// 尝试删除物理文件（如果存在）
	if issue.FilePath != "" {
		if err := m.fileStore.Delete(ctx, issue.FilePath); err != nil {
			if m.logger != nil {
				m.logger.Warnf("删除损坏文件失败: %s, 错误: %v", issue.FilePath, err)
			}
		}
	}

	return success
}

// fixInconsistentRecord 修复不一致的记录
func (m *Manager) fixInconsistentRecord(ctx context.Context, issue *ConsistencyIssue) bool {
	// 保守策略：删除不一致的记录和文件
	success := m.fixOrphanIndex(ctx, issue)

	// 删除不一致的文件
	if issue.FilePath != "" {
		if err := m.fileStore.Delete(ctx, issue.FilePath); err != nil {
			if m.logger != nil {
				m.logger.Warnf("删除不一致文件失败: %s, 错误: %v", issue.FilePath, err)
			}
		}
	}

	return success
}
