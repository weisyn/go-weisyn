// Package resource 实现资源查询服务
package resource

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// Service 资源查询服务
type Service struct {
	storage     storage.BadgerStore
	fileStore   storage.FileStore
	txQuery     interfaces.InternalTxQuery
	resourceDir string
	logger      log.Logger
}

// NewService 创建资源查询服务（P3-19：从配置获取资源目录）
func NewService(badgerStore storage.BadgerStore, fileStore storage.FileStore, txQuery interfaces.InternalTxQuery, logger log.Logger) (interfaces.InternalResourceQuery, error) {
	if badgerStore == nil {
		return nil, fmt.Errorf("badgerStore 不能为空")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("fileStore 不能为空")
	}
	if txQuery == nil {
		return nil, fmt.Errorf("txQuery 不能为空")
	}

	// ⚠️ **注意**：resourceDir 已不再使用
	// FileStore 的根目录由 FileStore 配置决定（在节点场景下通常为 {instance_data_dir}/files）
	// BuildFilePath() 现在返回相对路径，与 CASStorage 保持一致
	// 保留 resourceDir 字段以避免破坏现有代码，但不再使用
	resourceDir := "" // 不再使用，保留字段以兼容

	s := &Service{
		storage:     badgerStore,
		fileStore:   fileStore,
		txQuery:     txQuery,
		resourceDir: resourceDir,
		logger:      logger,
	}

	if logger != nil {
		logger.Info("✅ ResourceQuery 服务已创建")
	}

	return s, nil
}

// GetResourceByContentHash 根据内容哈希查询完整资源
func (s *Service) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	// 从区块链存储获取资源元信息
	resource, exists, err := s.GetResourceFromBlockchain(ctx, contentHash)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("资源不存在: %x", contentHash)
	}

	return resource, nil
}

// GetResourceByInstance 根据资源实例标识获取资源
//
// 实现 interfaces.InternalResourceQuery.GetResourceByInstance
func (s *Service) GetResourceByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*pb_resource.Resource, bool, error) {
	if len(txHash) != 32 {
		return nil, false, fmt.Errorf("txHash 必须是 32 字节，实际: %d", len(txHash))
	}

	// 通过 TxQuery 获取交易
	blockHash, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
	if err != nil || tx == nil {
		return nil, false, fmt.Errorf("获取交易失败: %w", err)
	}

	// 边界检查
	if int(outputIndex) >= len(tx.Outputs) {
		return nil, false, nil
	}

	output := tx.Outputs[outputIndex]
	if output == nil {
		return nil, false, nil
	}

	resourceOutput := output.GetResource()
	if resourceOutput == nil || resourceOutput.Resource == nil {
		return nil, false, nil
	}

	if s.logger != nil {
		s.logger.Infof("✅ 通过实例查询资源成功, txHash=%x, blockHash=%x, outputIndex=%d",
			txHash, blockHash, outputIndex)
	}

	return resourceOutput.Resource, true, nil
}

// GetResourceFromBlockchain 从区块链获取资源元信息
//
// 🎯 **正确的查询流程**（遵循 data-architecture.md 规范）：
// 1. 通过 indices:resource:{contentHash} 找到 txHash
// 2. 通过 txHash 查询交易
// 3. 从交易的 ResourceOutput 中提取 Resource
//
// ⚠️ **重要**：资源元数据存储在交易/区块中，不应存储在 BadgerDB 的 resource:meta: 键下
func (s *Service) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	// 1. 获取资源关联的交易信息
	txHash, _, _, err := s.GetResourceTransaction(ctx, contentHash)
	if err != nil {
		// 资源索引不存在，说明资源不存在
		return nil, false, nil
	}

	// 2. 通过交易哈希查询完整交易
	_, _, tx, err := s.txQuery.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, false, fmt.Errorf("获取交易失败: %w", err)
	}

	// 3. 从交易输出中查找匹配的 ResourceOutput
	for _, output := range tx.Outputs {
		if output == nil {
			continue
		}

		// 检查是否是 ResourceOutput
		resourceOutput := output.GetResource()
		if resourceOutput == nil {
			continue
		}

		// 检查 Resource 是否存在
		if resourceOutput.Resource == nil {
			continue
		}

		// 匹配 contentHash
		if len(resourceOutput.Resource.ContentHash) == len(contentHash) {
			match := true
			for i := 0; i < len(contentHash); i++ {
				if resourceOutput.Resource.ContentHash[i] != contentHash[i] {
					match = false
					break
				}
			}
			if match {
				// 🔍 调试日志：检查从交易中提取的 Resource 是否有 ExecutionConfig
				resource := resourceOutput.Resource
				if resource.ExecutionConfig != nil {
					if contract, ok := resource.ExecutionConfig.(*pb_resource.Resource_Contract); ok && contract.Contract != nil {
						if s.logger != nil {
							s.logger.Infof("🔍 [DEBUG] GetResourceFromBlockchain: 找到 Resource，ExecutionConfig 存在 (abi_version=%s, functions=%d)",
								contract.Contract.AbiVersion, len(contract.Contract.ExportedFunctions))
						}
					} else {
						if s.logger != nil {
							s.logger.Warnf("🔍 [DEBUG] GetResourceFromBlockchain: ExecutionConfig 类型不匹配或为空")
						}
					}
				} else {
					if s.logger != nil {
						s.logger.Warnf("🔍 [DEBUG] GetResourceFromBlockchain: Resource.ExecutionConfig 为 nil (contentHash=%x)", contentHash)
					}
				}
				// 找到匹配的 Resource，返回
				return resourceOutput.Resource, true, nil
			}
		}
	}

	// 4. 未找到匹配的 ResourceOutput
	return nil, false, nil
}

// GetResourceTransaction 获取资源关联的交易信息
func (s *Service) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) {
	// ⚠️ Phase 4：使用代码→实例索引 + 交易查询，不再依赖旧的 indices:resource:{contentHash}
	if len(contentHash) != 32 {
		return nil, nil, 0, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 1. 从代码→实例索引获取第一个实例
	codeIndexKey := []byte(fmt.Sprintf("indices:resource-code:%x", contentHash))
	data, err := s.storage.Get(ctx, codeIndexKey)
	if err != nil || len(data) == 0 {
		return nil, nil, 0, fmt.Errorf("资源交易信息不存在: contentHash=%x", contentHash)
	}

	var instanceList []string
	if err := json.Unmarshal(data, &instanceList); err != nil || len(instanceList) == 0 {
		return nil, nil, 0, fmt.Errorf("解析资源实例索引失败: %w", err)
	}

	instanceIDStr := instanceList[0]
	parts := strings.Split(instanceIDStr, ":")
	if len(parts) != 2 {
		return nil, nil, 0, fmt.Errorf("无效的实例ID格式: %s", instanceIDStr)
	}

	txHashBytes, err := hex.DecodeString(parts[0])
	if err != nil || len(txHashBytes) != 32 {
		return nil, nil, 0, fmt.Errorf("无效的实例ID中的 txHash: %s", parts[0])
	}
	txHash = txHashBytes

	// 2. 通过 TxQuery 获取区块哈希和高度
	blockHash, _, _, err = s.txQuery.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("获取交易失败: %w", err)
	}

	blockHeight, err = s.txQuery.GetTxBlockHeight(ctx, txHash)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("获取交易区块高度失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ 资源交易查询成功，txHash=%x, blockHash=%x, blockHeight=%d",
			txHash, blockHash, blockHeight)
	}

	return txHash, blockHash, blockHeight, nil
}

// ListResourceInstancesByCode 列出指定代码的所有实例 OutPoint
//
// 实现 interfaces.InternalResourceQuery.ListResourceInstancesByCode
func (s *Service) ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*transaction.OutPoint, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	codeIndexKey := []byte(fmt.Sprintf("indices:resource-code:%x", contentHash))
	data, err := s.storage.Get(ctx, codeIndexKey)
	if err != nil || len(data) == 0 {
		// 索引不存在则返回空列表
		return []*transaction.OutPoint{}, nil
	}

	var instanceList []string
	if err := json.Unmarshal(data, &instanceList); err != nil {
		return nil, fmt.Errorf("解析资源实例索引失败: %w", err)
	}

	outpoints := make([]*transaction.OutPoint, 0, len(instanceList))
	for _, instanceIDStr := range instanceList {
		parts := strings.Split(instanceIDStr, ":")
		if len(parts) != 2 {
			if s.logger != nil {
				s.logger.Warnf("无效的实例ID格式, 跳过: %s", instanceIDStr)
			}
			continue
		}

		txHashBytes, err := hex.DecodeString(parts[0])
		if err != nil || len(txHashBytes) != 32 {
			if s.logger != nil {
				s.logger.Warnf("解析实例ID中的 txHash 失败, 跳过: %s", parts[0])
			}
			continue
		}

		// outputIndex 当前编码为十进制字符串
		var outputIndex uint32
		_, err = fmt.Sscanf(parts[1], "%d", &outputIndex)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("解析实例ID中的 outputIndex 失败, 跳过: %s", parts[1])
			}
			continue
		}

		outpoints = append(outpoints, &transaction.OutPoint{
			TxId:        txHashBytes,
			OutputIndex: outputIndex,
		})
	}

	return outpoints, nil
}

// CheckFileExists 检查本地文件是否存在
func (s *Service) CheckFileExists(contentHash []byte) bool {
	filePath := s.BuildFilePath(contentHash)
	ctx := context.Background()
	exists, err := s.fileStore.Exists(ctx, filePath)
	if err != nil {
		return false
	}
	return exists
}

// BuildFilePath 构建本地文件路径
//
// 🎯 **路径格式**（遵循 data-architecture.md 规范）：
// FileStore 根目录：./data/files（由 FileStore 配置决定）
// 相对路径：{hash[0:2]}/{hash[2:4]}/{fullHash}
// 完整路径：./data/files/{hash[0:2]}/{hash[2:4]}/{fullHash}
//
// ⚠️ **注意**：
// - FileStore 的根目录已经是 ./data/files
// - 只需要返回相对路径：{hash[0:2]}/{hash[2:4]}/{fullHash}
// - 与 CASStorage.BuildFilePath() 保持一致
func (s *Service) BuildFilePath(contentHash []byte) string {
	hashStr := fmt.Sprintf("%x", contentHash)
	if len(hashStr) < 4 {
		// 哈希长度不足，返回哈希本身（兜底处理）
		return hashStr
	}

	// 分层路径：hash[0:2]/hash[2:4]/fullHash
	// 注意：不包含 "files/" 前缀，因为 FileStore 根目录已经是 ./data/files
	return filepath.Join(
		hashStr[0:2], // 一级目录（256种可能）
		hashStr[2:4], // 二级目录（256种可能）
		hashStr,      // 完整哈希作为文件名
	)
}

// ListResourceHashes 列出所有资源哈希（P3-20：资源哈希列表查询）
//
// 🎯 **实现策略（Phase 4）**：
// 1. 使用前缀扫描 `indices:resource-code:` 获取所有资源索引键
// 2. 从键中提取哈希（键格式：`indices:resource-code:{contentHash}`）
// 3. 实现分页逻辑（offset, limit）
// 4. 返回哈希列表
func (s *Service) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	// 验证参数
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100 // 默认限制100个
	}
	if limit > 1000 {
		limit = 1000 // 最大限制1000个
	}

	// 使用前缀扫描获取所有资源索引键
	// 键格式：indices:resource-code:{contentHash}
	prefix := []byte("indices:resource-code:")
	results, err := s.storage.PrefixScan(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("前缀扫描失败: %w", err)
	}

	// 从键中提取哈希
	hashes := make([][]byte, 0, len(results))
	prefixStr := "indices:resource-code:"

	for keyStr := range results {
		// 提取哈希部分（跳过前缀）
		if len(keyStr) <= len(prefixStr) {
			continue
		}

		hashStr := keyStr[len(prefixStr):]
		// 验证哈希格式（应该是十六进制字符串）
		if len(hashStr) == 0 {
			continue
		}

		// 将十六进制字符串转换为字节数组
		hashBytes, err := hex.DecodeString(hashStr)
		if err != nil {
			// 解码失败，跳过此键
			if s.logger != nil {
				s.logger.Debugf("资源哈希解码失败，跳过: %s, error: %v", hashStr, err)
			}
			continue
		}

		hashes = append(hashes, hashBytes)
	}

	// 应用分页逻辑
	totalCount := len(hashes)
	if offset >= totalCount {
		// offset超出范围，返回空列表
		if s.logger != nil {
			s.logger.Debugf("列出资源哈希: offset=%d >= total=%d, 返回空列表", offset, totalCount)
		}
		return [][]byte{}, nil
	}

	// 计算结束位置
	end := offset + limit
	if end > totalCount {
		end = totalCount
	}

	// 提取分页结果
	pagedHashes := hashes[offset:end]

	if s.logger != nil {
		s.logger.Debugf("列出资源哈希: offset=%d, limit=%d, total=%d, returned=%d",
			offset, limit, totalCount, len(pagedHashes))
	}

	return pagedHashes, nil
}

// bytesToUint64 将字节数组转换为uint64
func bytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// 编译时检查接口实现
var _ interfaces.InternalResourceQuery = (*Service)(nil)
