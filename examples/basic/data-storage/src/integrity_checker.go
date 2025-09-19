package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

/*
🎯 数据完整性检查模块

这个模块展示如何在应用中确保数据完整性：
1. 计算和验证数据哈希
2. 检测数据篡改
3. 验证数字签名
4. 审计数据访问

💡 实际应用考虑：
- 支持多种哈希算法
- 实现批量验证
- 提供完整性报告
- 集成监控告警
*/

// IntegrityChecker 完整性检查器
type IntegrityChecker struct {
	hashAlgorithm string // 使用的哈希算法
}

// IntegrityResult 完整性检查结果
type IntegrityResult struct {
	RecordID     string    `json:"record_id"`
	IsValid      bool      `json:"is_valid"`
	ExpectedHash string    `json:"expected_hash"`
	ActualHash   string    `json:"actual_hash"`
	CheckTime    time.Time `json:"check_time"`
	ErrorMessage string    `json:"error_message"`
}

// BatchIntegrityResult 批量完整性检查结果
type BatchIntegrityResult struct {
	TotalChecked int               `json:"total_checked"`
	ValidCount   int               `json:"valid_count"`
	InvalidCount int               `json:"invalid_count"`
	Results      []IntegrityResult `json:"results"`
	CheckTime    time.Time         `json:"check_time"`
	Duration     time.Duration     `json:"duration"`
}

// NewIntegrityChecker 创建新的完整性检查器
func NewIntegrityChecker() *IntegrityChecker {
	return &IntegrityChecker{
		hashAlgorithm: "SHA256",
	}
}

// VerifyDataIntegrity 验证单个数据记录的完整性
// 🎯 功能：检查数据是否被篡改
func (ic *IntegrityChecker) VerifyDataIntegrity(record DataRecord) IntegrityResult {
	result := IntegrityResult{
		RecordID:  record.ID,
		CheckTime: time.Now(),
	}

	// 💡 生活化理解：
	// 数据完整性检查就像验证文件的指纹
	// - 原始哈希 = 文件的原始指纹
	// - 当前哈希 = 当前文件的指纹
	// - 比较结果 = 文件是否被修改过

	// 📋 步骤1：计算当前内容的哈希
	actualHash := ic.calculateContentHash(record.Content)
	result.ActualHash = actualHash

	// 📋 步骤2：获取期望的哈希值
	expectedHash := record.Hash
	result.ExpectedHash = expectedHash

	// 📋 步骤3：比较哈希值
	if actualHash == expectedHash {
		result.IsValid = true
	} else {
		result.IsValid = false
		result.ErrorMessage = "数据哈希不匹配，可能已被篡改"
	}

	return result
}

// BatchVerifyIntegrity 批量验证数据完整性
// 🎯 功能：高效地检查多个数据记录的完整性
func (ic *IntegrityChecker) BatchVerifyIntegrity(records []DataRecord) BatchIntegrityResult {
	startTime := time.Now()

	batchResult := BatchIntegrityResult{
		TotalChecked: len(records),
		Results:      make([]IntegrityResult, 0, len(records)),
		CheckTime:    startTime,
	}

	for _, record := range records {
		result := ic.VerifyDataIntegrity(record)
		batchResult.Results = append(batchResult.Results, result)

		if result.IsValid {
			batchResult.ValidCount++
		} else {
			batchResult.InvalidCount++
		}
	}

	batchResult.Duration = time.Since(startTime)
	return batchResult
}

// CalculateRecordHash 计算数据记录的完整哈希
// 🎯 功能：为完整的数据记录生成唯一标识
func (ic *IntegrityChecker) CalculateRecordHash(record DataRecord) string {
	// 构建用于哈希的数据字符串
	// 包含关键字段以确保完整性
	hashData := fmt.Sprintf("%s|%s|%s|%s|%s|%d",
		record.ID,
		record.Title,
		record.Content,
		record.Owner,
		record.DataType,
		record.Timestamp.Unix(),
	)

	return ic.calculateContentHash(hashData)
}

// DetectTampering 检测数据篡改
// 🎯 功能：深度分析数据是否存在篡改迹象
func (ic *IntegrityChecker) DetectTampering(record DataRecord, originalRecord DataRecord) map[string]interface{} {
	analysis := make(map[string]interface{})

	// 检查各个字段的变化
	changes := make(map[string]bool)

	if record.Title != originalRecord.Title {
		changes["title"] = true
	}

	if record.Content != originalRecord.Content {
		changes["content"] = true
	}

	if record.Owner != originalRecord.Owner {
		changes["owner"] = true
	}

	if record.DataType != originalRecord.DataType {
		changes["data_type"] = true
	}

	// 检查标签变化
	if !ic.compareStringSlices(record.Tags, originalRecord.Tags) {
		changes["tags"] = true
	}

	// 检查元数据变化
	if !ic.compareMetadata(record.Metadata, originalRecord.Metadata) {
		changes["metadata"] = true
	}

	analysis["has_changes"] = len(changes) > 0
	analysis["changed_fields"] = changes
	analysis["change_count"] = len(changes)

	// 计算变化程度
	totalFields := 6 // title, content, owner, data_type, tags, metadata
	changePercentage := float64(len(changes)) / float64(totalFields) * 100
	analysis["change_percentage"] = changePercentage

	// 篡改风险评估
	riskLevel := "low"
	if changePercentage > 50 {
		riskLevel = "high"
	} else if changePercentage > 20 {
		riskLevel = "medium"
	}
	analysis["risk_level"] = riskLevel

	return analysis
}

// ValidateRecordStructure 验证记录结构的完整性
// 🎯 功能：检查数据记录的结构是否符合规范
func (ic *IntegrityChecker) ValidateRecordStructure(record DataRecord) map[string]interface{} {
	validation := make(map[string]interface{})
	errors := make([]string, 0)
	warnings := make([]string, 0)

	// 必填字段检查
	if record.ID == "" {
		errors = append(errors, "记录ID不能为空")
	}

	if record.Title == "" {
		warnings = append(warnings, "标题为空")
	}

	if record.Content == "" {
		errors = append(errors, "内容不能为空")
	}

	if record.Owner == "" {
		errors = append(errors, "所有者不能为空")
	}

	// 格式检查
	if len(record.ID) < 8 {
		warnings = append(warnings, "记录ID长度可能不足")
	}

	if record.Hash == "" {
		warnings = append(warnings, "缺少内容哈希")
	} else if len(record.Hash) != 64 { // SHA256哈希长度
		warnings = append(warnings, "哈希格式可能不正确")
	}

	// 时间戳检查
	if record.Timestamp.IsZero() {
		warnings = append(warnings, "时间戳为零值")
	} else if record.Timestamp.After(time.Now().Add(time.Hour)) {
		warnings = append(warnings, "时间戳指向未来")
	}

	// 版本号检查
	if record.Version <= 0 {
		warnings = append(warnings, "版本号无效")
	}

	validation["is_valid"] = len(errors) == 0
	validation["errors"] = errors
	validation["warnings"] = warnings
	validation["error_count"] = len(errors)
	validation["warning_count"] = len(warnings)

	return validation
}

// GenerateIntegrityReport 生成完整性报告
// 🎯 功能：为数据集生成详细的完整性报告
func (ic *IntegrityChecker) GenerateIntegrityReport(records []DataRecord) map[string]interface{} {
	report := make(map[string]interface{})
	startTime := time.Now()

	// 基本统计
	totalRecords := len(records)
	report["total_records"] = totalRecords
	report["check_time"] = startTime
	report["hash_algorithm"] = ic.hashAlgorithm

	// 完整性检查
	batchResult := ic.BatchVerifyIntegrity(records)
	report["integrity_check"] = batchResult

	// 结构验证统计
	validStructures := 0
	totalErrors := 0
	totalWarnings := 0

	for _, record := range records {
		validation := ic.ValidateRecordStructure(record)
		if validation["is_valid"].(bool) {
			validStructures++
		}
		totalErrors += validation["error_count"].(int)
		totalWarnings += validation["warning_count"].(int)
	}

	report["structure_validation"] = map[string]interface{}{
		"valid_structures":   validStructures,
		"invalid_structures": totalRecords - validStructures,
		"total_errors":       totalErrors,
		"total_warnings":     totalWarnings,
	}

	// 数据质量评分
	integrityScore := float64(batchResult.ValidCount) / float64(totalRecords) * 100
	structureScore := float64(validStructures) / float64(totalRecords) * 100
	overallScore := (integrityScore + structureScore) / 2

	report["quality_scores"] = map[string]interface{}{
		"integrity_score": integrityScore,
		"structure_score": structureScore,
		"overall_score":   overallScore,
	}

	// 风险评估
	riskLevel := "low"
	if overallScore < 70 {
		riskLevel = "high"
	} else if overallScore < 90 {
		riskLevel = "medium"
	}
	report["risk_assessment"] = riskLevel

	// 建议
	recommendations := ic.generateRecommendations(batchResult, totalErrors, totalWarnings)
	report["recommendations"] = recommendations

	report["report_duration"] = time.Since(startTime)
	return report
}

// 私有方法：计算内容哈希
func (ic *IntegrityChecker) calculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// 私有方法：比较字符串切片
func (ic *IntegrityChecker) compareStringSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	// 创建计数映射
	count1 := make(map[string]int)
	count2 := make(map[string]int)

	for _, s := range slice1 {
		count1[s]++
	}

	for _, s := range slice2 {
		count2[s]++
	}

	// 比较计数
	for key, count := range count1 {
		if count2[key] != count {
			return false
		}
	}

	return true
}

// 私有方法：比较元数据
func (ic *IntegrityChecker) compareMetadata(meta1, meta2 map[string]interface{}) bool {
	if len(meta1) != len(meta2) {
		return false
	}

	for key, value1 := range meta1 {
		value2, exists := meta2[key]
		if !exists {
			return false
		}

		// 简单的值比较（实际应用中可能需要更复杂的比较）
		if fmt.Sprintf("%v", value1) != fmt.Sprintf("%v", value2) {
			return false
		}
	}

	return true
}

// 私有方法：生成建议
func (ic *IntegrityChecker) generateRecommendations(batchResult BatchIntegrityResult, totalErrors, totalWarnings int) []string {
	var recommendations []string

	if batchResult.InvalidCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("发现 %d 个完整性问题，建议立即调查可能的数据篡改", batchResult.InvalidCount))
	}

	if totalErrors > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("发现 %d 个结构错误，建议修复数据格式问题", totalErrors))
	}

	if totalWarnings > 10 {
		recommendations = append(recommendations, "警告数量较多，建议优化数据质量流程")
	}

	if batchResult.Duration > time.Second {
		recommendations = append(recommendations, "检查耗时较长，建议优化批量处理性能")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "数据完整性良好，建议保持当前的数据管理实践")
	}

	return recommendations
}

// 演示函数：展示完整性检查功能
func DemoIntegrityChecker() {
	fmt.Println("🎮 数据完整性检查演示")
	fmt.Println("===================")

	// 创建完整性检查器
	ic := NewIntegrityChecker()

	// 1. 创建测试数据
	fmt.Println("1. 创建测试数据...")

	originalRecord := DataRecord{
		ID:        "test_record_1",
		Title:     "原始文档",
		Content:   "这是原始内容",
		Owner:     "alice",
		DataType:  "document",
		Tags:      []string{"测试", "原始"},
		Metadata:  map[string]interface{}{"版本": 1, "状态": "正常"},
		Timestamp: time.Now(),
		Version:   1,
	}

	// 计算原始哈希
	originalRecord.Hash = ic.calculateContentHash(originalRecord.Content)

	fmt.Printf("原始记录创建完成，哈希: %s\n", originalRecord.Hash[:16]+"...")

	// 2. 完整性验证演示
	fmt.Println("\n2. 完整性验证演示...")
	result := ic.VerifyDataIntegrity(originalRecord)
	fmt.Printf("完整性检查结果: 有效=%t\n", result.IsValid)

	// 3. 篡改检测演示
	fmt.Println("\n3. 篡改检测演示...")
	tamperedRecord := originalRecord
	tamperedRecord.Content = "这是被篡改的内容" // 模拟篡改

	result = ic.VerifyDataIntegrity(tamperedRecord)
	fmt.Printf("篡改后检查结果: 有效=%t, 错误=%s\n", result.IsValid, result.ErrorMessage)

	// 4. 变化分析演示
	fmt.Println("\n4. 变化分析演示...")
	analysis := ic.DetectTampering(tamperedRecord, originalRecord)
	fmt.Printf("变化分析: 有变化=%t, 风险级别=%s\n",
		analysis["has_changes"], analysis["risk_level"])

	// 5. 结构验证演示
	fmt.Println("\n5. 结构验证演示...")
	validation := ic.ValidateRecordStructure(originalRecord)
	fmt.Printf("结构验证: 有效=%t, 错误数=%d, 警告数=%d\n",
		validation["is_valid"], validation["error_count"], validation["warning_count"])

	// 6. 批量检查演示
	fmt.Println("\n6. 批量检查演示...")
	testRecords := []DataRecord{originalRecord, tamperedRecord}
	batchResult := ic.BatchVerifyIntegrity(testRecords)
	fmt.Printf("批量检查: 总数=%d, 有效=%d, 无效=%d\n",
		batchResult.TotalChecked, batchResult.ValidCount, batchResult.InvalidCount)

	fmt.Println("✅ 完整性检查演示完成")
}
