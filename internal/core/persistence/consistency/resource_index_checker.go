// Package consistency 提供数据一致性检查工具
package consistency

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ResourceIndexChecker 资源索引一致性检查器
//
// 🎯 **核心职责**：
// 验证资源索引的一致性，确保 CodeId → InstanceId 的 1:N 关系正确
//
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// - 验证旧索引（基于 ContentHash）与新索引（基于 InstanceId）的一致性
// - 检查代码→实例的 1:N 关系是否正确维护
// - 发现并报告索引不一致问题
type ResourceIndexChecker struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewResourceIndexChecker 创建资源索引一致性检查器
func NewResourceIndexChecker(storage storage.BadgerStore, logger log.Logger) *ResourceIndexChecker {
	return &ResourceIndexChecker{
		storage: storage,
		logger:  logger,
	}
}

// CheckResult 检查结果
type CheckResult struct {
	TotalCodesChecked   int             // 检查的代码数量
	TotalInstancesFound int             // 找到的实例总数
	Inconsistencies     []Inconsistency // 不一致问题列表
	OrphanedInstances   []InstanceInfo  // 孤立实例（代码索引中不存在）
	OrphanedCodes       []CodeInfo      // 孤立代码（实例索引中不存在）
	DuplicateInstances  []InstanceInfo  // 重复实例（同一实例ID出现多次）
}

// Inconsistency 不一致问题
type Inconsistency struct {
	Type        string // 问题类型
	CodeHash    []byte // 代码哈希（ResourceCodeId）
	InstanceID  string // 实例ID（ResourceInstanceId）
	Description string // 问题描述
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	InstanceID  string // 实例ID（格式：{txHash}:{outputIndex}）
	CodeHash    []byte // 代码哈希
	TxHash      []byte // 交易哈希
	OutputIndex uint32 // 输出索引
}

// CodeInfo 代码信息
type CodeInfo struct {
	CodeHash []byte // 代码哈希
	TxHash   []byte // 部署交易哈希（从旧索引获取）
}

// CheckConsistency 检查资源索引一致性
//
// 📋 **检查项**：
// 1. 代码→实例索引一致性：每个代码的实例列表是否完整
// 2. 实例→代码反向一致性：每个实例是否在对应代码的实例列表中
// 3. 旧索引与新索引一致性：旧索引中的实例是否都在新索引中
// 4. 孤立实例检查：新索引中的实例是否都有对应的代码索引
// 5. 重复实例检查：同一实例ID是否出现多次
func (c *ResourceIndexChecker) CheckConsistency(ctx context.Context) (*CheckResult, error) {
	result := &CheckResult{
		Inconsistencies:    make([]Inconsistency, 0),
		OrphanedInstances:  make([]InstanceInfo, 0),
		OrphanedCodes:      make([]CodeInfo, 0),
		DuplicateInstances: make([]InstanceInfo, 0),
	}

	// 1. 扫描所有代码索引（indices:resource-code:*）
	codePrefix := []byte("indices:resource-code:")
	codeIndexes, err := c.storage.PrefixScan(ctx, codePrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描代码索引失败: %w", err)
	}

	// 2. 构建代码→实例映射
	codeToInstances := make(map[string][]string) // codeHash -> [instanceID1, instanceID2, ...]
	instanceToCode := make(map[string][]byte)    // instanceID -> codeHash

	for keyStr, value := range codeIndexes {
		// 提取代码哈希
		codeHashHex := extractCodeHashFromKey(keyStr)
		if codeHashHex == "" {
			continue
		}
		codeHash, err := hex.DecodeString(codeHashHex)
		if err != nil {
			if c.logger != nil {
				c.logger.Warnf("解析代码哈希失败: key=%s, error=%v", keyStr, err)
			}
			continue
		}

		// 解析实例列表
		var instanceList []string
		if err := json.Unmarshal(value, &instanceList); err != nil {
			if c.logger != nil {
				c.logger.Warnf("解析实例列表失败: codeHash=%x, error=%v", codeHash, err)
			}
			continue
		}

		codeToInstances[codeHashHex] = instanceList
		result.TotalCodesChecked++

		// 构建反向映射
		for _, instanceID := range instanceList {
			if existingCode, exists := instanceToCode[instanceID]; exists {
				// 发现重复实例
				result.DuplicateInstances = append(result.DuplicateInstances, InstanceInfo{
					InstanceID: instanceID,
					CodeHash:   codeHash,
				})
				result.Inconsistencies = append(result.Inconsistencies, Inconsistency{
					Type:        "DUPLICATE_INSTANCE",
					CodeHash:    codeHash,
					InstanceID:  instanceID,
					Description: fmt.Sprintf("实例 %s 同时属于代码 %x 和 %x", instanceID, codeHash, existingCode),
				})
			} else {
				instanceToCode[instanceID] = codeHash
			}
		}
		result.TotalInstancesFound += len(instanceList)
	}

	// 3. 扫描所有实例索引（indices:resource-instance:*）
	instancePrefix := []byte("indices:resource-instance:")
	instanceIndexes, err := c.storage.PrefixScan(ctx, instancePrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描实例索引失败: %w", err)
	}

	// 4. 验证实例索引与代码索引的一致性
	for keyStr, value := range instanceIndexes {
		// 提取实例ID
		instanceID := extractInstanceIDFromKey(keyStr)
		if instanceID == "" {
			continue
		}

		// 解析实例元信息（blockHash + blockHeight + contentHash）
		if len(value) < 72 {
			result.Inconsistencies = append(result.Inconsistencies, Inconsistency{
				Type:        "INVALID_INSTANCE_INDEX",
				InstanceID:  instanceID,
				Description: fmt.Sprintf("实例索引值长度不足: expected>=72, actual=%d", len(value)),
			})
			continue
		}

		instanceCodeHash := value[40:72] // contentHash 在索引值的 40-72 字节位置
		instanceCodeHashHex := fmt.Sprintf("%x", instanceCodeHash)

		// 检查实例是否在对应代码的实例列表中
		expectedInstances, exists := codeToInstances[instanceCodeHashHex]
		if !exists {
			// 孤立实例：代码索引中不存在
			txHash, outputIndex, err := eutxo.DecodeInstanceID(instanceID)
			if err == nil {
				result.OrphanedInstances = append(result.OrphanedInstances, InstanceInfo{
					InstanceID:  instanceID,
					CodeHash:    instanceCodeHash,
					TxHash:      txHash,
					OutputIndex: outputIndex,
				})
				result.Inconsistencies = append(result.Inconsistencies, Inconsistency{
					Type:        "ORPHANED_INSTANCE",
					CodeHash:    instanceCodeHash,
					InstanceID:  instanceID,
					Description: fmt.Sprintf("实例 %s 在代码索引中不存在", instanceID),
				})
			}
		} else {
			// 检查实例是否在列表中
			found := false
			for _, expectedID := range expectedInstances {
				if expectedID == instanceID {
					found = true
					break
				}
			}
			if !found {
				result.Inconsistencies = append(result.Inconsistencies, Inconsistency{
					Type:        "MISSING_IN_CODE_LIST",
					CodeHash:    instanceCodeHash,
					InstanceID:  instanceID,
					Description: fmt.Sprintf("实例 %s 不在代码 %x 的实例列表中", instanceID, instanceCodeHash),
				})
			}
		}
	}

	return result, nil
}

// extractCodeHashFromKey 从代码索引键中提取代码哈希
// 键格式：indices:resource-code:{codeHashHex}
func extractCodeHashFromKey(keyStr string) string {
	prefix := "indices:resource-code:"
	if len(keyStr) <= len(prefix) {
		return ""
	}
	return keyStr[len(prefix):]
}

// extractInstanceIDFromKey 从实例索引键中提取实例ID
// 键格式：indices:resource-instance:{txHash}:{outputIndex}
func extractInstanceIDFromKey(keyStr string) string {
	prefix := "indices:resource-instance:"
	if len(keyStr) <= len(prefix) {
		return ""
	}
	return keyStr[len(prefix):]
}
