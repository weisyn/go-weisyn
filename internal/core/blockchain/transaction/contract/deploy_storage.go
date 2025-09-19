// Package contract 合约部署存储管理器
//
// 🎯 **模块职责**：
// 专门负责智能合约部署过程中的存储管理工作。
// 从主服务文件中分离出来，实现单一职责原则。
//
// 🔧 **核心功能**：
// - 合约内容预存储管理
// - 资源文件存储接口
// - 内容寻址网络集成
// - 分布式存储位置管理
// - 存储策略优化
//
// 📋 **主要组件**：
// - DeployStorageManager: 核心存储管理器
// - ContentAddressedStorage: 内容寻址存储
// - StorageLocationManager: 存储位置管理
//
// 🎯 **设计特点**：
// - 多位置存储：提高数据可靠性和可用性
// - 内容寻址：通过哈希实现去重和验证
// - 异构友好：解决节点间的内容同步问题
package contract

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//
//	存储管理器数据结构定义
//
// ============================================================================

// DeployStorageManager 合约部署存储管理器
//
// 🎯 **存储职责**：
// 负责智能合约部署过程中所有存储相关的操作，包括内容预存储、
// 资源文件管理和分布式存储位置管理。
//
// 🔧 **存储能力**：
// - 内容预存储：在交易确认前预先存储合约内容
// - 多位置存储：提高数据可靠性和网络可用性
// - 内容寻址：基于哈希的去重和验证机制
// - 资源管理：与资源存储接口的集成
// - 异构支持：支持不同节点间的内容同步
//
// 💡 **设计特点**：
// - 高可用：多个存储位置提高容错能力
// - 高效率：内容寻址避免重复存储
// - 高一致性：哈希验证确保内容完整性
type DeployStorageManager struct {
	resourceManager repository.ResourceManager // 资源存储管理器
	logger          log.Logger                 // 日志记录器
}

// NewDeployStorageManager 创建部署存储管理器
//
// 🎯 **工厂方法**：
// 创建一个新的合约部署存储管理器实例。
//
// 参数：
//   - resourceManager: 资源存储管理器
//   - logger: 日志记录器
//
// 返回：
//   - *DeployStorageManager: 配置好的存储管理器实例
func NewDeployStorageManager(
	resourceManager repository.ResourceManager,
	logger log.Logger,
) *DeployStorageManager {
	return &DeployStorageManager{
		resourceManager: resourceManager,
		logger:          logger,
	}
}

// ============================================================================
//
//	内容预存储方法
//
// ============================================================================

// PreStoreContractContent 预存储合约内容到项目资源存储系统
//
// 🎯 **异构部署问题解决方案**：
// 在交易构建阶段就将合约内容存储到项目的资源管理系统中，确保：
// 1. 其他节点可以通过content_hash获取合约内容
// 2. 即使部署者离线，合约仍可被其他节点验证和执行
// 3. 支持异构节点的协同工作
//
// 📋 **存储策略**：
// - 使用项目统一的 ResourceManager 接口
// - 基于 SHA-256 的内容寻址存储
// - 自动去重：相同哈希的内容只存储一份
// - 元数据管理：包含完整的合约部署信息
//
// 🔧 **实现机制**：
// - 创建工作文件用于存储操作
// - 调用 ResourceManager.StoreResourceFile
// - 返回存储后的实际位置信息
// - 清理工作文件资源
//
// 参数：
//   - ctx: 上下文对象
//   - wasmCode: 合约WASM字节码
//   - contractFilePath: 原始合约文件路径
//
// 返回：
//   - []byte: 内容哈希（SHA-256）
//   - [][]byte: 存储位置列表（项目内部存储位置）
//   - error: 存储失败时的错误信息
func (dsm *DeployStorageManager) PreStoreContractContent(
	ctx context.Context,
	wasmCode []byte,
	contractFilePath string,
) ([]byte, [][]byte, error) {
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("🏗️ 开始预存储合约内容 - 大小: %d bytes", len(wasmCode)))
	}

	// ========== 使用 ResourceManager 进行存储 ==========
	metadata := dsm.buildPreStoreMetadata(contractFilePath, wasmCode)

	// 调用项目统一的资源存储接口
	storedHash, err := dsm.resourceManager.StoreResourceFile(ctx, contractFilePath, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("合约内容预存储失败: %v", err)
	}

	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("✅ 合约内容存储完成 - 哈希: %x", storedHash))
	}

	// ========== 生成存储位置信息 ==========
	storageLocations := [][]byte{
		// 项目内部的内容寻址位置
		dsm.generateInternalStorageLocation(storedHash),
	}

	if dsm.logger != nil {
		dsm.logger.Info(fmt.Sprintf("✅ 合约内容预存储完成 - 哈希: %x", storedHash))
	}

	return storedHash, storageLocations, nil
}

// ============================================================================
//
//	资源文件存储方法
//
// ============================================================================

// StoreContractResource 存储合约资源文件
//
// 🎯 **资源存储管理**：
// 通过资源管理器接口存储合约文件，建立文件路径与内容哈希的映射关系。
//
// 📋 **存储内容**：
// - 文件元数据：类型、大小、创建时间等基础信息
// - 存储路径：文件在存储系统中的位置
// - 内容哈希：用于内容验证和去重的标识
//
// 🔧 **集成特性**：
// - 统一接口：通过ResourceManager进行统一管理
// - 元数据丰富：提供完整的文件描述信息
// - 错误处理：存储失败时提供详细错误信息
//
// 参数：
//   - ctx: 上下文对象
//   - filePath: 合约文件路径
//   - wasmCode: WASM合约字节码
//
// 返回：
//   - []byte: 存储操作的结果哈希或标识
//   - error: 存储过程中的错误
func (dsm *DeployStorageManager) StoreContractResource(
	ctx context.Context,
	filePath string,
	wasmCode []byte,
) ([]byte, error) {
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("🗄️ 开始存储合约资源 - 文件: %s, 大小: %d bytes",
			filePath, len(wasmCode)))
	}

	// ========== 构建资源元数据 ==========
	metadata := dsm.buildResourceMetadata(filePath, wasmCode)

	// ========== 执行存储操作 ==========
	result, err := dsm.resourceManager.StoreResourceFile(ctx, filePath, metadata)
	if err != nil {
		return nil, fmt.Errorf("资源文件存储失败: %v", err)
	}

	if dsm.logger != nil {
		dsm.logger.Info(fmt.Sprintf("✅ 合约资源存储完成 - 结果: %x", result))
	}

	return result, nil
}

// ============================================================================
//
//	内部存储位置管理方法
//
// ============================================================================

// generateInternalStorageLocation 生成项目内部存储位置
//
// 🎯 **内部位置生成**：
// 基于内容哈希生成项目内部资源存储系统的位置标识。
//
// 📋 **位置格式**：
// 使用项目统一的内容寻址格式：
// - resource://[hash] - 项目内部资源存储位置
//
// 🔧 **设计优势**：
// - 统一管理：使用项目现有的 ResourceManager 接口
// - 内容寻址：基于 SHA-256 哈希的确定性定位
// - 去重优化：相同内容自动去重，节省存储空间
// - 高可用：依赖项目成熟的资源存储架构
//
// 参数：
//   - contentHash: 内容哈希字节数组
//
// 返回：
//   - []byte: 项目内部存储位置标识
func (dsm *DeployStorageManager) generateInternalStorageLocation(contentHash []byte) []byte {
	// 生成项目内部资源存储位置格式
	location := append([]byte("resource://"), contentHash...)

	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("📍 生成内部存储位置: resource://%x", contentHash))
	}

	return location
}

// ============================================================================
//
//	存储验证方法
//
// ============================================================================

// VerifyStoredContract 验证已存储的合约内容
//
// 🎯 **存储验证**：
// 通过 ResourceManager 接口验证合约是否已正确存储，确保内容完整性。
//
// 📋 **验证项目**：
// - 存储可达性：资源是否存在于存储系统中
// - 内容完整性：存储的内容是否完整
// - 哈希一致性：内容哈希是否匹配
//
// 🔧 **验证策略**：
// - 使用 ResourceManager.GetResourceByHash 进行查询
// - 检查返回的资源信息是否完整
// - 验证元数据的一致性
//
// 参数：
//   - ctx: 上下文对象
//   - contentHash: 预期的内容哈希
//
// 返回：
//   - bool: 验证是否成功
//   - error: 验证过程中的错误
func (dsm *DeployStorageManager) VerifyStoredContract(
	ctx context.Context,
	contentHash []byte,
) (bool, error) {
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("🔍 开始验证已存储合约 - 哈希: %x", contentHash))
	}

	// ========== 使用 ResourceManager 查询资源 ==========
	resourceInfo, err := dsm.resourceManager.GetResourceByHash(ctx, contentHash)
	if err != nil {
		if dsm.logger != nil {
			dsm.logger.Debug(fmt.Sprintf("❌ 合约存储验证失败: %v", err))
		}
		return false, fmt.Errorf("查询存储的合约失败: %v", err)
	}

	// ========== 验证资源信息完整性 ==========
	if resourceInfo == nil {
		if dsm.logger != nil {
			dsm.logger.Debug("❌ 合约资源信息为空")
		}
		return false, fmt.Errorf("合约资源信息为空")
	}

	// ========== 验证元数据合理性 ==========
	isValid := dsm.validateResourceMetadata(resourceInfo, contentHash)
	if !isValid {
		if dsm.logger != nil {
			dsm.logger.Debug("❌ 合约资源元数据验证失败")
		}
		return false, fmt.Errorf("合约资源元数据验证失败")
	}

	if dsm.logger != nil {
		dsm.logger.Info(fmt.Sprintf("✅ 合约存储验证成功 - 哈希: %x", contentHash))
	}

	return true, nil
}

// ============================================================================
//
//	工具方法
//
// ============================================================================

// buildResourceMetadata 构建资源元数据
//
// 🎯 **元数据构建**：
// 为资源文件生成完整的元数据信息，用于资源管理器存储。
func (dsm *DeployStorageManager) buildResourceMetadata(filePath string, wasmCode []byte) map[string]string {
	return map[string]string{
		"type":       "contract",                           // 资源类型
		"mime_type":  "application/wasm",                   // MIME类型
		"size":       fmt.Sprintf("%d", len(wasmCode)),     // 文件大小
		"created_at": fmt.Sprintf("%d", time.Now().Unix()), // 创建时间
		"file_path":  filePath,                             // 文件路径
		"format":     "wasm",                               // 格式标识
		"category":   "executable",                         // 资源分类
		"deployment": "contract_deploy",                    // 部署来源
	}
}

// buildPreStoreMetadata 构建预存储元数据
//
// 🎯 **预存储元数据**：
// 为合约预存储生成详细的元数据信息，包含部署上下文和哈希验证信息。
func (dsm *DeployStorageManager) buildPreStoreMetadata(filePath string, wasmCode []byte) map[string]string {
	return map[string]string{
		"type":            "contract",                           // 资源类型
		"mime_type":       "application/wasm",                   // MIME类型
		"size":            fmt.Sprintf("%d", len(wasmCode)),     // 文件大小
		"created_at":      fmt.Sprintf("%d", time.Now().Unix()), // 创建时间
		"file_path":       filePath,                             // 原始文件路径
		"format":          "wasm",                               // 格式标识
		"category":        "executable",                         // 资源分类
		"deployment":      "contract_deploy",                    // 部署来源
		"stage":           "pre_store",                          // 存储阶段
		"hash_algorithm":  "sha256",                             // 哈希算法
		"storage_purpose": "heterogeneous_deployment",           // 存储目的
	}
}

// validateResourceMetadata 验证资源元数据
//
// 🎯 **完整实现**：
// 严格校验ResourceStorageInfo中的关键字段，确保资源存储的正确性。
//
// 📋 **验证项目**：
// 1. 基础字段：资源信息不为空
// 2. 哈希一致性：使用bytes.Equal安全比较内容哈希
// 3. 资源类型：验证为contract类型
// 4. MIME类型：验证为application/wasm格式
// 5. 文件大小：检查文件大小的合理性
// 6. 可用性状态：确认资源处于可用状态
//
// 参数：
//   - resourceInfo: 资源存储信息（必须为*types.ResourceStorageInfo类型）
//   - expectedHash: 期望的内容哈希
//
// 返回：
//   - bool: 验证通过返回true，否则返回false
func (dsm *DeployStorageManager) validateResourceMetadata(resourceInfo *types.ResourceStorageInfo, expectedHash []byte) bool {
	// ========== 基础有效性检查 ==========
	if resourceInfo == nil {
		if dsm.logger != nil {
			dsm.logger.Warn("资源信息为空")
		}
		return false
	}

	if len(expectedHash) != 32 {
		if dsm.logger != nil {
			dsm.logger.Warn(fmt.Sprintf("期望哈希长度无效: %d (应为32字节)", len(expectedHash)))
		}
		return false
	}

	// ========== 内容哈希验证（使用bytes.Equal安全比较）==========
	if !bytes.Equal(resourceInfo.ContentHash, expectedHash) {
		if dsm.logger != nil {
			dsm.logger.Warn(fmt.Sprintf("内容哈希不匹配 - 期望: %x, 实际: %x",
				expectedHash, resourceInfo.ContentHash))
		}
		return false
	}

	// ========== 资源类型验证 ==========
	if resourceInfo.ResourceType != "contract" {
		if dsm.logger != nil {
			dsm.logger.Warn(fmt.Sprintf("资源类型不匹配 - 期望: contract, 实际: %s",
				resourceInfo.ResourceType))
		}
		return false
	}

	// ========== MIME类型验证 ==========
	if mimeType, exists := resourceInfo.Metadata["mime_type"]; exists {
		if mimeType != "application/wasm" {
			if dsm.logger != nil {
				dsm.logger.Warn(fmt.Sprintf("MIME类型不匹配 - 期望: application/wasm, 实际: %s", mimeType))
			}
			return false
		}
	} else {
		if dsm.logger != nil {
			dsm.logger.Warn("缺少MIME类型元数据")
		}
		return false
	}

	// ========== 文件大小合理性检查 ==========
	if resourceInfo.Size <= 0 {
		if dsm.logger != nil {
			dsm.logger.Warn(fmt.Sprintf("文件大小无效: %d", resourceInfo.Size))
		}
		return false
	}

	if resourceInfo.Size > 100*1024*1024 { // 100MB限制
		if dsm.logger != nil {
			dsm.logger.Warn(fmt.Sprintf("文件过大: %d bytes (超过100MB限制)", resourceInfo.Size))
		}
		return false
	}

	// ========== 可用性状态检查 ==========
	if !resourceInfo.IsAvailable {
		if dsm.logger != nil {
			dsm.logger.Warn("资源标记为不可用")
		}
		return false
	}

	// ========== 验证通过 ==========
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("✅ 资源元数据验证通过 - 哈希: %x, 大小: %d bytes",
			resourceInfo.ContentHash, resourceInfo.Size))
	}
	return true
}

// ============================================================================
//
//	查询和检索方法
//
// ============================================================================

// GetStoredContractInfo 获取已存储的合约信息
//
// 🎯 **合约查询**：
// 通过内容哈希查询已存储的合约详细信息。
func (dsm *DeployStorageManager) GetStoredContractInfo(
	ctx context.Context,
	contentHash []byte,
) (interface{}, error) {
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("🔍 查询已存储合约信息 - 哈希: %x", contentHash))
	}

	// 使用 ResourceManager 查询资源信息
	resourceInfo, err := dsm.resourceManager.GetResourceByHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("查询合约信息失败: %v", err)
	}

	if dsm.logger != nil {
		dsm.logger.Info(fmt.Sprintf("✅ 合约信息查询成功 - 哈希: %x", contentHash))
	}

	return resourceInfo, nil
}

// ListStoredContracts 列出已存储的合约
//
// 🎯 **合约列表**：
// 获取项目中所有已存储的合约资源列表。
func (dsm *DeployStorageManager) ListStoredContracts(
	ctx context.Context,
	offset int,
	limit int,
) ([]interface{}, error) {
	if dsm.logger != nil {
		dsm.logger.Debug(fmt.Sprintf("📋 列出已存储合约 - 偏移: %d, 限制: %d", offset, limit))
	}

	// 使用 ResourceManager 按类型查询合约资源
	contracts, err := dsm.resourceManager.ListResourcesByType(ctx, "contract", offset, limit)
	if err != nil {
		return nil, fmt.Errorf("列出合约失败: %v", err)
	}

	// 转换为通用接口类型
	result := make([]interface{}, len(contracts))
	for i, contract := range contracts {
		result[i] = contract
	}

	if dsm.logger != nil {
		dsm.logger.Info(fmt.Sprintf("✅ 合约列表查询成功 - 数量: %d", len(result)))
	}

	return result, nil
}
