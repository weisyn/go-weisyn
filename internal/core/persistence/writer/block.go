// Package writer 实现区块数据写入逻辑
//
// 📦 **区块数据写入 (Block Data Writing)**
//
// 本文件实现区块数据的写入逻辑，包括区块序列化、哈希计算、
// 文件存储和索引更新。
//
// 🎯 **核心职责**：
// - 序列化区块数据
// - 计算区块哈希
// - 写入区块文件（如果使用文件存储）
// - 写入区块索引（高度索引、哈希索引）
package writer

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

// writeBlockData 存储区块数据
//
// 🎯 **核心职责**：
// 将区块数据写入存储，包括：
// - 区块文件存储（文件系统：blocks/{segment}/{height}.bin）
// - 区块索引（BadgerDB：高度索引、哈希索引）
//
// 📋 **处理流程**：
// 1. 序列化区块数据
// 2. 计算区块哈希
// 3. 计算存储路径（按高度段组织：每1000个一段）
// 4. 写入区块文件到文件系统
// 5. 写入区块索引到 BadgerDB（indices:height:{height} → {blockHash, filePath, fileSize}）
// 6. 写入区块哈希索引（indices:hash:{hash} → height）
//
// ⚠️ **注意事项**：
// - ✅ 设计约束（对齐 _dev）：区块原始数据落盘到 blocks/ 目录，Badger 仅存索引与链元数据
// - 索引值格式：blockHash(32字节) + filePath长度(1字节) + filePath(N字节) + fileSize(8字节)
func (s *Service) writeBlockData(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	// 1. 计算区块哈希（使用 gRPC 服务）
	if s.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化")
	}

	req := &core.ComputeBlockHashRequest{
		Block: block,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return fmt.Errorf("区块结构无效")
	}

	blockHash := resp.Hash

	// 2. 序列化区块数据
	blockData, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("序列化区块失败: %w", err)
	}

	// 3. 计算存储路径（按高度段组织，每1000个一段）
	// 格式：blocks/{heightSegment:010d}/{height:010d}.bin
	// 例如：blocks/0000000000/0000000001.bin, blocks/0000001000/0000001000.bin
	heightSegment := (block.Header.Height / 1000) * 1000
	fileName := fmt.Sprintf("%010d.bin", block.Header.Height)

	// 4. 写入区块文件到文件系统
	// 说明：FileStore 根目录是 {instance_data_dir}/files，但允许通过 blocks/... 访问同级的 {instance_data_dir}/blocks。
	blockFilePath := fmt.Sprintf("blocks/%010d/%s", heightSegment, fileName)
	if s.fileStore == nil {
		return fmt.Errorf("fileStore 未初始化")
	}
	blockDirPath := fmt.Sprintf("blocks/%010d", heightSegment)
	if err := s.fileStore.MakeDir(ctx, blockDirPath, true); err != nil {
		return fmt.Errorf("创建区块目录失败: %w", err)
	}
	if err := s.fileStore.Save(ctx, blockFilePath, blockData); err != nil {
		return fmt.Errorf("写入区块文件失败: %w", err)
	}

	// 5. 存储区块索引（高度 -> {blockHash, filePath, fileSize}）
	// 键格式：indices:height:{height}
	// 值格式：blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)
	filePathBytes := []byte(blockFilePath) // 在索引中存储 blocks/...，便于 Query 层直接 Load
	indexValue := make([]byte, 32+1+len(filePathBytes)+8)
	copy(indexValue[0:32], blockHash)
	indexValue[32] = byte(len(filePathBytes))
	copy(indexValue[33:33+len(filePathBytes)], filePathBytes)
	copy(indexValue[33+len(filePathBytes):41+len(filePathBytes)], uint64ToBytes(uint64(len(blockData))))

	heightKey := fmt.Sprintf("indices:height:%d", block.Header.Height)
	if err := tx.Set([]byte(heightKey), indexValue); err != nil {
		return fmt.Errorf("存储区块高度索引失败: %w", err)
	}

	// 6. 存储区块哈希索引（哈希 -> height）
	hashKey := fmt.Sprintf("indices:hash:%x", blockHash)
	heightBytes := uint64ToBytes(block.Header.Height)
	if err := tx.Set([]byte(hashKey), heightBytes); err != nil {
		return fmt.Errorf("存储区块哈希索引失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 区块数据已存储(blocks/): height=%d, hash=%x, size=%d bytes, path=%s",
			block.Header.Height, blockHash[:8], len(blockData), blockFilePath)
	}

	return nil
}
