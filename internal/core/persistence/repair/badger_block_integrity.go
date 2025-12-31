package repair

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

type BlockStoreIntegrityReport struct {
	TipHeight uint64
	Checked   int

	// TipHash 来自 state:chain:tip（若存在）
	TipHash []byte

	// FailReason 用于日志观测（不用于程序逻辑）
	FailReason string
}

// CheckBadgerBlockStoreIntegrity 是“blocks/ + Badger 索引”模式下的启动一致性门闸（历史函数名保留）。
//
// 现实约束（对齐当前实现）：
// - 区块原始数据落盘在 FileStore（`blocks/<segment>/<height>.bin`）
// - Badger 仅存索引与链状态（`state:chain:tip` / `indices:*`）
// - 跨存储无法做到单事务原子提交，因此本门闸的目标是：阻止节点在“索引声称存在但 blocks/ 缺失/损坏”时继续运行。
//
// 核心校验：
// - state:chain:tip 若存在：tipHeight/tipHash 必须可解析
// - indices:height:tipHeight 必须存在（值中包含 blockHash + filePath + fileSize），且 blockHash 等于 tipHash
// - indices:hash:<tipHash> 必须存在且等于 tipHeight
// - tip 对应的 blocks/ 文件必须存在、可反序列化（可选：计算 hash 与 tipHash 一致）
// - 对最近 sample 个高度做“存在性 + 索引一致性 + 文件可读”快速校验（不计算 hash）
func CheckBadgerBlockStoreIntegrity(
	ctx context.Context,
	store storeiface.BadgerStore,
	fileStore storeiface.FileStore,
	blockHashClient core.BlockHashServiceClient,
	logger logiface.Logger,
	sample uint64,
) (BlockStoreIntegrityReport, error) {
	rep := BlockStoreIntegrityReport{}
	if store == nil {
		return rep, fmt.Errorf("store is nil")
	}
	if fileStore == nil {
		return rep, fmt.Errorf("fileStore is nil")
	}

	// 1) 解析 tip（若不存在则认为空链，不做校验）
	tipData, err := store.Get(ctx, []byte("state:chain:tip"))
	if err != nil || len(tipData) == 0 {
		return rep, nil
	}
	if len(tipData) < 40 {
		rep.FailReason = fmt.Sprintf("invalid tip len=%d", len(tipData))
		return rep, fmt.Errorf("invalid state:chain:tip (len=%d)", len(tipData))
	}
	rep.TipHeight = bytesToUint64(tipData[:8])
	rep.TipHash = append([]byte(nil), tipData[8:40]...)

	if rep.TipHeight == 0 && len(rep.TipHash) == 0 {
		return rep, nil
	}

	// 2) 校验 indices:height:tipHeight（必须可解析，并且其 blockHash == tipHash）
	heightIndexKey := []byte(fmt.Sprintf("indices:height:%d", rep.TipHeight))
	heightIndexVal, err := store.Get(ctx, heightIndexKey)
	if err != nil || len(heightIndexVal) == 0 {
		rep.FailReason = "tip_height_index_missing"
		return rep, fmt.Errorf("missing indices:height for tip (height=%d err=%v)", rep.TipHeight, err)
	}
	_, tipFilePath, tipFileSize, err := parseHeightIndexValue(heightIndexVal)
	if err != nil {
		rep.FailReason = "tip_height_index_invalid"
		return rep, fmt.Errorf("invalid indices:height for tip (height=%d): %w", rep.TipHeight, err)
	}
	if len(heightIndexVal) < 32 || string(heightIndexVal[:32]) != string(rep.TipHash) {
		rep.FailReason = "tip_height_index_hash_mismatch"
		return rep, fmt.Errorf("indices:height hash mismatch for tip (height=%d)", rep.TipHeight)
	}

	// 3) tip 对应 blocks/ 文件必须存在 & 可解码
	blockBytes, err := fileStore.Load(ctx, tipFilePath)
	if err != nil || len(blockBytes) == 0 {
		rep.FailReason = "tip_block_file_missing"
		if err == nil {
			err = fmt.Errorf("empty block file")
		}
		return rep, fmt.Errorf("missing tip block file (height=%d path=%s err=%v)", rep.TipHeight, tipFilePath, err)
	}
	if tipFileSize > 0 && uint64(len(blockBytes)) != tipFileSize {
		rep.FailReason = "tip_block_file_size_mismatch"
		return rep, fmt.Errorf("tip block file size mismatch (height=%d expected=%d got=%d path=%s)", rep.TipHeight, tipFileSize, len(blockBytes), tipFilePath)
	}
	blk := &core.Block{}
	if err := proto.Unmarshal(blockBytes, blk); err != nil {
		rep.FailReason = "tip_block_unmarshal_failed"
		return rep, fmt.Errorf("unmarshal tip block failed (height=%d): %w", rep.TipHeight, err)
	}
	if blk.Header == nil || blk.Header.Height != rep.TipHeight {
		rep.FailReason = "tip_block_height_mismatch"
		return rep, fmt.Errorf("tip block height mismatch: expected=%d got=%v", rep.TipHeight, func() interface{} {
			if blk.Header == nil {
				return nil
			}
			return blk.Header.Height
		}())
	}

	// 4) 校验 indices:hash:<tipHash> == tipHeight
	hashIndexKey := []byte(fmt.Sprintf("indices:hash:%x", rep.TipHash))
	hashIndexVal, err := store.Get(ctx, hashIndexKey)
	if err != nil || len(hashIndexVal) != 8 {
		rep.FailReason = "tip_hash_index_missing_or_invalid"
		return rep, fmt.Errorf("invalid indices:hash for tip (height=%d len=%d err=%v)", rep.TipHeight, len(hashIndexVal), err)
	}
	if bytesToUint64(hashIndexVal) != rep.TipHeight {
		rep.FailReason = "tip_hash_index_height_mismatch"
		return rep, fmt.Errorf("indices:hash height mismatch for tip (expected=%d got=%d)", rep.TipHeight, bytesToUint64(hashIndexVal))
	}

	// 5) （强校验）计算 tip block hash，与 tipHash 一致
	if blockHashClient != nil {
		hashResp, herr := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk})
		if herr != nil || hashResp == nil || !hashResp.IsValid || len(hashResp.Hash) == 0 {
			rep.FailReason = "tip_compute_hash_failed"
			if herr == nil {
				herr = fmt.Errorf("invalid hash response")
			}
			return rep, fmt.Errorf("compute tip block hash failed: %w", herr)
		}
		if string(hashResp.Hash) != string(rep.TipHash) {
			rep.FailReason = "tip_computed_hash_mismatch"
			return rep, fmt.Errorf("tip computed hash mismatch (height=%d)", rep.TipHeight)
		}
	}

	// 6) 快速抽样校验最近 sample 个高度（存在性 + 索引一致性，不计算 hash）
	if sample == 0 {
		sample = 16
	}
	var checked uint64
	for h := rep.TipHeight; ; h-- {
		if checked >= sample {
			break
		}
		checked++

		hiKey := []byte(fmt.Sprintf("indices:height:%d", h))
		hiVal, e := store.Get(ctx, hiKey)
		if e != nil || len(hiVal) == 0 {
			rep.FailReason = "sample_height_index_missing_or_invalid"
			return rep, fmt.Errorf("invalid indices:height in sample window (height=%d)", h)
		}
		_, filePath, fileSize, pe := parseHeightIndexValue(hiVal)
		if pe != nil {
			rep.FailReason = "sample_height_index_parse_failed"
			return rep, fmt.Errorf("invalid indices:height format in sample window (height=%d): %w", h, pe)
		}

		// hash->height 必须可回查
		hsKey := []byte(fmt.Sprintf("indices:hash:%x", hiVal[:32]))
		hsVal, e := store.Get(ctx, hsKey)
		if e != nil || len(hsVal) != 8 || bytesToUint64(hsVal) != h {
			rep.FailReason = "sample_hash_index_mismatch"
			return rep, fmt.Errorf("indices:hash mismatch in sample window (height=%d)", h)
		}

		// blocks/ 文件必须可读取（不反序列化，不计算 hash）
		bb, fe := fileStore.Load(ctx, filePath)
		if fe != nil || len(bb) == 0 {
			rep.FailReason = "sample_block_file_missing"
			if fe == nil {
				fe = fmt.Errorf("empty block file")
			}
			return rep, fmt.Errorf("missing block file in sample window (height=%d path=%s err=%v)", h, filePath, fe)
		}
		if fileSize > 0 && uint64(len(bb)) != fileSize {
			rep.FailReason = "sample_block_file_size_mismatch"
			return rep, fmt.Errorf("block file size mismatch in sample window (height=%d expected=%d got=%d path=%s)", h, fileSize, len(bb), filePath)
		}

		if h == 0 {
			break
		}
	}
	rep.Checked = int(checked)

	if logger != nil {
		logger.Infof("✅ Badger 区块存储一致性检查通过: tip=%d sample=%d", rep.TipHeight, rep.Checked)
	}
	return rep, nil
}

// parseHeightIndexValue 解析 `indices:height:{height}` 的 value：
// `blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)`
func parseHeightIndexValue(val []byte) (blockHash []byte, filePath string, fileSize uint64, err error) {
	if len(val) < 32+1+8 {
		return nil, "", 0, fmt.Errorf("too short len=%d", len(val))
	}
	blockHash = append([]byte(nil), val[:32]...)
	plen := int(val[32])
	if plen <= 0 {
		return nil, "", 0, fmt.Errorf("invalid filePathLen=%d", plen)
	}
	if len(val) < 33+plen+8 {
		return nil, "", 0, fmt.Errorf("invalid len=%d for filePathLen=%d", len(val), plen)
	}
	filePath = string(val[33 : 33+plen])
	fileSize = bytesToUint64(val[33+plen : 33+plen+8])
	return blockHash, filePath, fileSize, nil
}
