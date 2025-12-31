package repair

import (
	"context"
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// RepairGenesisIndex 修复创世区块索引
//
// 🎯 **创世区块索引修复器**：从 blocks 文件重建创世区块索引
//
// 策略：
// 1. 从 blocks/0000000000/0000000000.bin 读取创世区块文件
// 2. 反序列化并验证区块
// 3. 计算区块哈希
// 4. 重建 indices:height:0 和 indices:hash:<hash> 索引
// 5. 如果 state:chain:tip 高度为0或不存在，一并修复
//
// 参数：
//   - ctx: 上下文
//   - store: BadgerDB存储（用于写入索引）
//   - fileStore: 文件存储（用于读取区块文件）
//   - blockHashClient: 区块哈希计算服务客户端
//   - logger: 日志记录器
//
// 返回：
//   - error: 修复失败时返回错误，nil表示成功
func RepairGenesisIndex(
	ctx context.Context,
	store storage.BadgerStore,
	fileStore storage.FileStore,
	blockHashClient core.BlockHashServiceClient,
	logger log.Logger,
) error {
	if logger != nil {
		logger.Info("🩹 开始修复创世区块索引...")
	}

	// 1. 从文件系统读取创世区块
	//
	// 注意：Writer 侧写入 blocks 文件使用的是 blocks/...（blocks/ 与 files/ 同级）
	// 详见 internal/core/persistence/writer/block.go
	blockFilePath := "blocks/0000000000/0000000000.bin"
	blockBytes, err := fileStore.Load(ctx, blockFilePath)
	if err != nil {
		if logger != nil {
			logger.Errorf("❌ 读取创世区块文件失败: path=%s err=%v", blockFilePath, err)
		}
		return fmt.Errorf("读取创世区块文件失败: %w", err)
	}

	if len(blockBytes) == 0 {
		if logger != nil {
			logger.Errorf("❌ 创世区块文件为空: path=%s", blockFilePath)
		}
		return fmt.Errorf("创世区块文件为空: path=%s", blockFilePath)
	}

	// 2. 反序列化区块
	genesisBlock := &core.Block{}
	if err := proto.Unmarshal(blockBytes, genesisBlock); err != nil {
		if logger != nil {
			logger.Errorf("❌ 反序列化创世区块失败: err=%v", err)
		}
		return fmt.Errorf("反序列化创世区块失败: %w", err)
	}

	// 验证区块高度
	if genesisBlock.Header == nil {
		return fmt.Errorf("创世区块头为空")
	}
	if genesisBlock.Header.Height != 0 {
		if logger != nil {
			logger.Errorf("❌ 区块高度不为0: height=%d", genesisBlock.Header.Height)
		}
		return fmt.Errorf("区块高度不为0: height=%d", genesisBlock.Header.Height)
	}

	// 3. 计算区块哈希
	req := &core.ComputeBlockHashRequest{
		Block: genesisBlock,
	}
	resp, err := blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		if logger != nil {
			logger.Errorf("❌ 计算创世区块哈希失败: err=%v", err)
		}
		return fmt.Errorf("计算创世区块哈希失败: %w", err)
	}
	if !resp.IsValid {
		return fmt.Errorf("创世区块结构无效")
	}
	
	genesisHash := resp.Hash

	if len(genesisHash) != 32 {
		return fmt.Errorf("创世区块哈希长度不正确: len=%d (expected=32)", len(genesisHash))
	}

	if logger != nil {
		logger.Infof("🔍 创世区块信息: height=0 hash=%x path=%s", genesisHash[:8], blockFilePath)
	}

	// 4. 重建索引
	//
	// indices:height:{h} 值格式必须与 Query/Writer 保持一致：
	// blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)
	// 详见：
	// - internal/core/persistence/query/block/service.go
	// - internal/core/persistence/writer/block.go
	heightKey := []byte("indices:height:0")
	pathBytes := []byte(blockFilePath)
	if len(pathBytes) > 255 {
		return fmt.Errorf("创世区块路径过长，无法写入高度索引: pathLen=%d", len(pathBytes))
	}
	heightValue := make([]byte, 32+1+len(pathBytes)+8)
	copy(heightValue[0:32], genesisHash)
	heightValue[32] = byte(len(pathBytes))
	copy(heightValue[33:33+len(pathBytes)], pathBytes)
	binary.BigEndian.PutUint64(heightValue[33+len(pathBytes):41+len(pathBytes)], uint64(len(blockBytes)))

	if err := store.Set(ctx, heightKey, heightValue); err != nil {
		if logger != nil {
			logger.Errorf("❌ 写入高度索引失败: key=%s err=%v", string(heightKey), err)
		}
		return fmt.Errorf("写入高度索引失败: %w", err)
	}

	if logger != nil {
		logger.Infof("✅ 高度索引已重建: key=%s value_len=%d", string(heightKey), len(heightValue))
	}

	// 4.2 indices:hash:<hash> = height(8 bytes)
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", genesisHash))
	hashValue := make([]byte, 8)
	binary.BigEndian.PutUint64(hashValue, 0)

	if err := store.Set(ctx, hashKey, hashValue); err != nil {
		if logger != nil {
			logger.Errorf("❌ 写入哈希索引失败: key=%s err=%v", string(hashKey), err)
		}
		return fmt.Errorf("写入哈希索引失败: %w", err)
	}

	if logger != nil {
		logger.Infof("✅ 哈希索引已重建: key=indices:hash:%x... height=0", genesisHash[:8])
	}

	// 4.3 如果 state:chain:tip 不存在或高度为0，也一并修复
	tipKey := []byte("state:chain:tip")
	tipData, _ := store.Get(ctx, tipKey)

	shouldRepairTip := false
	if len(tipData) < 8 {
		// tip不存在或格式错误
		shouldRepairTip = true
		if logger != nil {
			logger.Infof("🔍 链尖不存在或格式错误: len=%d", len(tipData))
		}
	} else {
		tipHeight := binary.BigEndian.Uint64(tipData[:8])
		if tipHeight == 0 {
			// tip高度为0，可能需要修复哈希部分
			shouldRepairTip = true
			if logger != nil {
				logger.Infof("🔍 链尖高度为0，检查哈希部分")
			}
		}
	}

	if shouldRepairTip {
		tipValue := make([]byte, 40)
		binary.BigEndian.PutUint64(tipValue[0:8], 0)
		copy(tipValue[8:40], genesisHash)

		if err := store.Set(ctx, tipKey, tipValue); err != nil {
			if logger != nil {
				logger.Warnf("⚠️ 修复链尖失败 (非致命): err=%v", err)
			}
			// 不返回错误，因为这不是关键失败
		} else {
			if logger != nil {
				logger.Infof("✅ 链尖已修复: height=0 hash=%x", genesisHash[:8])
			}
		}
	}

	if logger != nil {
		logger.Infof("✅ 创世区块索引修复成功: hash=%x", genesisHash[:8])
	}

	return nil
}

