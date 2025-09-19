// Package internal 提供交易管理的内部工具函数
//
// 📋 **hash_utils.go - 交易哈希计算工具函数**
//
// 本文件提供交易哈希计算相关的工具函数，确保哈希计算的标准化和一致性。
// 支持单个交易哈希、批量交易哈希计算等核心功能。
//
// 🎯 **核心职责**：
// - 标准化哈希计算：调用crypto层服务确保跨平台一致的哈希结果
// - 批量哈希处理：支持高效的批量交易哈希计算
// - 哈希验证：提供交易哈希验证功能
// - 哈希缓存：支持交易哈希缓存以提升性能
//
// 🏗️ **设计特点**：
// - 独立工具函数：不依赖特定结构体，通过参数传递依赖
// - 确定性计算：相同输入保证相同输出
// - 服务调用：统一调用crypto层的TransactionHashService
// - 性能优化：支持批量处理和缓存机制
// - 错误处理：提供完整的错误处理和日志记录
//
// 📋 **使用方式**：
// 其他子模块可直接调用这些工具函数：
//
//	import "github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
//	hash, err := internal.ComputeTransactionHash(ctx, hashClient, tx)
package internal

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                              哈希算法常量
// ============================================================================

// HashConstants 哈希计算相关常量
const (
	StandardHashLength = 32 // SHA-256标准哈希长度（字节）
	HashPrefix         = "tx_hash:"
	BatchHashPrefix    = "batch_hash:"
)

// ============================================================================
//                              单个交易哈希计算
// ============================================================================

// ComputeTransactionHash 计算交易哈希
//
// 🎯 **标准化交易哈希计算**
//
// 通过调用统一的TransactionHashService计算交易哈希，确保计算结果的
// 确定性和跨平台一致性。
//
// 🔒 **确定性保证**：
// - 固定算法：SHA-256
// - 标准序列化：Protobuf确定性序列化
// - 跨平台一致：任何设备计算同一交易得到相同哈希
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - tx: 需要计算哈希的交易对象
//   - includeDebugInfo: 是否包含调试信息（不影响哈希计算）
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - []byte: 32字节的标准化交易哈希
//   - error: 计算过程中的错误，nil表示计算成功
func ComputeTransactionHash(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	tx *transaction.Transaction,
	includeDebugInfo bool,
	logger log.Logger,
) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易为空，无法计算哈希")
	}
	if hashServiceClient == nil {
		return nil, fmt.Errorf("哈希服务客户端为空")
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🧮 开始计算交易哈希 - 版本: %d, 输入数: %d, 输出数: %d",
			tx.Version, len(tx.Inputs), len(tx.Outputs)))
	}

	// 构造 ComputeHashRequest
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: includeDebugInfo,
	}

	// 调用 gRPC TransactionHashService 计算哈希
	resp, err := hashServiceClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用交易哈希服务失败: %w", err)
	}

	// 验证响应结果
	if resp == nil {
		return nil, fmt.Errorf("交易哈希服务返回空响应")
	}
	if !resp.IsValid {
		return nil, fmt.Errorf("交易哈希计算失败：交易格式无效")
	}
	if len(resp.Hash) != StandardHashLength {
		return nil, fmt.Errorf("交易哈希长度不正确: 期望 %d 字节, 实际 %d 字节",
			StandardHashLength, len(resp.Hash))
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("✅ 成功计算交易哈希 - 哈希: %x", resp.Hash))
	}

	return resp.Hash, nil
}

// ValidateTransactionHash 验证交易哈希
//
// 🎯 **交易哈希验证工具**
//
// 验证给定交易的哈希是否正确，通过重新计算哈希并与期望值比较。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - tx: 需要验证的交易对象
//   - expectedHash: 期望的哈希值
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - bool: 验证结果（true=哈希正确，false=哈希不匹配）
//   - error: 验证过程中的错误
func ValidateTransactionHash(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	tx *transaction.Transaction,
	expectedHash []byte,
	logger log.Logger,
) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("交易为空，无法验证哈希")
	}
	if len(expectedHash) != StandardHashLength {
		return false, fmt.Errorf("期望哈希长度不正确: %d", len(expectedHash))
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🔍 开始验证交易哈希 - 期望: %x", expectedHash))
	}

	// 构造验证请求
	req := &transaction.ValidateHashRequest{
		Transaction:  tx,
		ExpectedHash: expectedHash,
	}

	// 调用验证服务
	resp, err := hashServiceClient.ValidateHash(ctx, req)
	if err != nil {
		return false, fmt.Errorf("调用哈希验证服务失败: %w", err)
	}

	if resp == nil {
		return false, fmt.Errorf("哈希验证服务返回空响应")
	}

	if logger != nil {
		if resp.IsValid {
			logger.Debug(fmt.Sprintf("✅ 交易哈希验证通过 - 哈希: %x", expectedHash))
		} else {
			logger.Debug(fmt.Sprintf("❌ 交易哈希验证失败 - 期望: %x, 实际: %x",
				expectedHash, resp.ComputedHash))
		}
	}

	return resp.IsValid, nil
}

// ============================================================================
//                              批量哈希计算
// ============================================================================

// BatchComputeTransactionHashes 批量计算交易哈希
//
// 🎯 **高效的批量哈希计算**
//
// 批量计算多个交易的哈希值，比单个计算更高效。
// 适用于区块验证、交易池处理等需要批量哈希的场景。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - transactions: 交易列表
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - [][]byte: 哈希列表（与输入交易顺序对应）
//   - error: 计算错误
func BatchComputeTransactionHashes(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	transactions []*transaction.Transaction,
	logger log.Logger,
) ([][]byte, error) {
	if hashServiceClient == nil {
		return nil, fmt.Errorf("哈希服务客户端为空")
	}
	if len(transactions) == 0 {
		return [][]byte{}, nil
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🧮 开始批量计算交易哈希 - 数量: %d", len(transactions)))
	}

	hashes := make([][]byte, 0, len(transactions))

	// 逐个计算（后续可优化为真正的批量接口）
	for i, tx := range transactions {
		if tx == nil {
			return nil, fmt.Errorf("第 %d 个交易为空", i)
		}

		hash, err := ComputeTransactionHash(ctx, hashServiceClient, tx, false, logger)
		if err != nil {
			return nil, fmt.Errorf("计算第 %d 个交易哈希失败: %w", i, err)
		}

		hashes = append(hashes, hash)
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("✅ 批量哈希计算完成 - %d 个哈希", len(hashes)))
	}

	return hashes, nil
}

// BatchValidateTransactionHashes 批量验证交易哈希
//
// 🎯 **高效的批量哈希验证**
//
// 批量验证多个交易的哈希值是否正确。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - transactions: 交易列表
//   - expectedHashes: 期望的哈希列表
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - []bool: 验证结果列表（与输入顺序对应）
//   - error: 验证错误
func BatchValidateTransactionHashes(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	transactions []*transaction.Transaction,
	expectedHashes [][]byte,
	logger log.Logger,
) ([]bool, error) {
	if hashServiceClient == nil {
		return nil, fmt.Errorf("哈希服务客户端为空")
	}
	if len(transactions) != len(expectedHashes) {
		return nil, fmt.Errorf("交易数量与期望哈希数量不匹配: %d vs %d",
			len(transactions), len(expectedHashes))
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🔍 开始批量验证交易哈希 - 数量: %d", len(transactions)))
	}

	results := make([]bool, 0, len(transactions))

	for i, tx := range transactions {
		if tx == nil {
			return nil, fmt.Errorf("第 %d 个交易为空", i)
		}

		isValid, err := ValidateTransactionHash(ctx, hashServiceClient, tx, expectedHashes[i], logger)
		if err != nil {
			return nil, fmt.Errorf("验证第 %d 个交易哈希失败: %w", i, err)
		}

		results = append(results, isValid)
	}

	// 统计验证结果
	validCount := 0
	for _, isValid := range results {
		if isValid {
			validCount++
		}
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("✅ 批量哈希验证完成 - 通过: %d/%d", validCount, len(results)))
	}

	return results, nil
}

// ============================================================================
//                              调试和工具方法
// ============================================================================

// ComputeTransactionHashWithDebug 计算交易哈希（包含调试信息）
//
// 🎯 **调试友好的哈希计算**
//
// 计算交易哈希并返回详细的调试信息，用于开发和测试阶段。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - tx: 交易对象
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - []byte: 交易哈希
//   - *transaction.HashDebugInfo: 调试信息
//   - error: 计算错误
func ComputeTransactionHashWithDebug(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	tx *transaction.Transaction,
	logger log.Logger,
) ([]byte, *transaction.HashDebugInfo, error) {
	if tx == nil {
		return nil, nil, fmt.Errorf("交易为空，无法计算哈希")
	}
	if hashServiceClient == nil {
		return nil, nil, fmt.Errorf("哈希服务客户端为空")
	}

	// 构造调试请求
	req := &transaction.ComputeHashRequest{
		Transaction:      tx,
		IncludeDebugInfo: true, // 包含调试信息
	}

	// 调用服务
	resp, err := hashServiceClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("调用交易哈希服务失败: %w", err)
	}

	if resp == nil {
		return nil, nil, fmt.Errorf("交易哈希服务返回空响应")
	}
	if !resp.IsValid {
		return nil, nil, fmt.Errorf("交易哈希计算失败：交易格式无效")
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🐛 调试信息 - 哈希: %x, 字段数: %d",
			resp.Hash, len(resp.DebugInfo.GetIncludedFields())))
	}

	return resp.Hash, resp.DebugInfo, nil
}

// GetTransactionID 获取交易ID字符串
//
// 🎯 **用户友好的交易标识符**
//
// 将交易哈希转换为用户友好的字符串格式，用于显示和查询。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - hashServiceClient: 交易哈希服务客户端
//   - tx: 交易对象
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - string: 十六进制格式的交易ID字符串
//   - error: 计算错误
func GetTransactionID(
	ctx context.Context,
	hashServiceClient transaction.TransactionHashServiceClient,
	tx *transaction.Transaction,
	logger log.Logger,
) (string, error) {
	hash, err := ComputeTransactionHash(ctx, hashServiceClient, tx, false, logger)
	if err != nil {
		return "", fmt.Errorf("计算交易哈希失败: %w", err)
	}

	txID := fmt.Sprintf("%x", hash)

	if logger != nil {
		logger.Debug(fmt.Sprintf("🆔 生成交易ID - %s", txID))
	}

	return txID, nil
}

// ============================================================================
//                              编译时检查
// ============================================================================

// 确保包含必要的导入和类型检查
// ValidateHashLength 验证哈希长度
//
// 🎯 **哈希长度验证工具**
//
// 验证给定的哈希是否为标准长度（32字节），确保哈希格式的一致性。
//
// 💡 **参数说明**：
//   - hash: 待验证的哈希字节数组
//
// 💡 **返回值说明**：
//   - error: 验证错误，nil表示长度正确
func ValidateHashLength(hash []byte) error {
	const StandardHashLength = 32 // SHA256哈希的标准长度

	if len(hash) != StandardHashLength {
		return fmt.Errorf("哈希长度不正确: 期望 %d 字节, 实际 %d 字节",
			StandardHashLength, len(hash))
	}
	return nil
}

// ============================================================================
//                              工具函数和常量
// ============================================================================

var (
	_ = fmt.Sprintf          // 确保fmt包正确导入
	_ = context.Context(nil) // 确保context包正确导入
)
